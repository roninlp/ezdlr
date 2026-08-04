package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type aria2RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type aria2Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *aria2RPCError  `json:"error"`
}

type aria2RPCClient struct {
	url, secret string
	httpClient  *http.Client
	mu          sync.Mutex
	nextID      int
}

func (c *aria2RPCClient) call(ctx context.Context, method string, args ...any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	params := append([]any{"token:" + c.secret}, args...)
	body, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  []any  `json:"params"`
	}{"2.0", id, method, params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("aria2 RPC returned HTTP %d: %s", resp.StatusCode, message)
	}
	var decoded aria2Response
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode aria2 RPC response: %w", err)
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != id {
		return nil, errors.New("invalid aria2 RPC response")
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("aria2 RPC error %d: %s", decoded.Error.Code, decoded.Error.Message)
	}
	return decoded.Result, nil
}

type aria2StatusResult struct {
	GID             string            `json:"gid"`
	Status          string            `json:"status"`
	TotalLength     string            `json:"totalLength"`
	CompletedLength string            `json:"completedLength"`
	DownloadSpeed   string            `json:"downloadSpeed"`
	Files           []aria2FileResult `json:"files"`
}

type aria2FileResult struct {
	Path string `json:"path"`
	URIs []struct {
		URI string `json:"uri"`
	} `json:"uris"`
}

func (s aria2StatusResult) engineStatus() EngineStatus {
	return EngineStatus{GID: s.GID, Status: mapAria2State(s.Status), TotalBytes: parseCounter(s.TotalLength), CompletedBytes: parseCounter(s.CompletedLength), DownloadSpeed: parseCounter(s.DownloadSpeed)}
}

func (s aria2StatusResult) download() EngineDownload {
	if len(s.Files) == 0 || len(s.Files[0].URIs) == 0 {
		return EngineDownload{GID: s.GID, Status: s.engineStatus()}
	}
	return EngineDownload{GID: s.GID, URL: s.Files[0].URIs[0].URI, Destination: filepath.Dir(s.Files[0].Path), Status: s.engineStatus()}
}
func parseCounter(value string) int64 { number, _ := strconv.ParseInt(value, 10, 64); return number }
func mapAria2State(value string) DownloadState {
	switch value {
	case "active":
		return StateActive
	case "paused":
		return StatePaused
	case "complete":
		return StateComplete
	case "error", "removed":
		return StateFailed
	default:
		return StateQueued
	}
}

type Aria2Engine struct {
	client          *aria2RPCClient
	cmd             *exec.Cmd
	shutdownTimeout time.Duration
	mu              sync.Mutex
	gids            map[string]string
	lock            *os.File
	exit            chan error
	processDone     chan struct{}
	processErr      error
}

var _ DownloadEngine = (*Aria2Engine)(nil)

func NewAria2Engine(endpoint, secret string) *Aria2Engine {
	return &Aria2Engine{client: &aria2RPCClient{url: endpoint, secret: secret, httpClient: &http.Client{Timeout: 5 * time.Second}}, shutdownTimeout: 5 * time.Second, gids: make(map[string]string)}
}

func (e *Aria2Engine) watchProcess() {
	if e.cmd == nil || e.exit != nil {
		return
	}
	e.exit = make(chan error, 1)
	e.processDone = make(chan struct{})
	go func() {
		err := e.cmd.Wait()
		e.mu.Lock()
		e.processErr = err
		e.mu.Unlock()
		close(e.processDone)
		e.exit <- err
	}()
}

func (e *Aria2Engine) Exited() <-chan error { return e.exit }

func (e *Aria2Engine) Add(rawURL, destination string) error {
	result, err := e.client.call(context.Background(), "aria2.addUri", []any{rawURL}, map[string]string{"dir": destination, "check-certificate": "true", "pause": "true", "split": "4", "max-connection-per-server": "4"})
	if err != nil {
		return err
	}
	var gid string
	if err := json.Unmarshal(result, &gid); err != nil || gid == "" {
		return errors.New("aria2 returned an invalid download identifier")
	}
	e.mu.Lock()
	e.gids[rawURL] = gid
	e.mu.Unlock()
	return nil
}

func (e *Aria2Engine) GID(rawURL string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.gids[rawURL]
}

func (e *Aria2Engine) Status(gid string) (EngineStatus, error) {
	result, err := e.client.call(context.Background(), "aria2.tellStatus", gid, []string{"gid", "status", "totalLength", "completedLength", "downloadSpeed"})
	if err != nil {
		return EngineStatus{}, err
	}
	var status aria2StatusResult
	if err := json.Unmarshal(result, &status); err != nil || status.GID == "" {
		return EngineStatus{}, errors.New("aria2 returned an invalid status")
	}
	return status.engineStatus(), nil
}

func (e *Aria2Engine) Recover() ([]EngineDownload, error) {
	var recovered []EngineDownload
	for _, request := range []struct {
		method string
		args   []any
	}{
		{method: "aria2.tellActive"},
		{method: "aria2.tellWaiting", args: []any{0, 1000}},
		{method: "aria2.tellStopped", args: []any{0, 1000}},
	} {
		result, err := e.client.call(context.Background(), request.method, request.args...)
		if err != nil {
			return nil, err
		}
		var statuses []aria2StatusResult
		if err := json.Unmarshal(result, &statuses); err != nil {
			return nil, fmt.Errorf("decode aria2 recovery response: %w", err)
		}
		for _, status := range statuses {
			recovered = append(recovered, status.download())
		}
	}
	return recovered, nil
}

func (e *Aria2Engine) Pause(gid string) error {
	_, err := e.client.call(context.Background(), "aria2.pause", gid)
	return err
}

func (e *Aria2Engine) Resume(gid string) error {
	_, err := e.client.call(context.Background(), "aria2.unpause", gid)
	return err
}

func (e *Aria2Engine) Cancel(gid string) error {
	_, err := e.client.call(context.Background(), "aria2.remove", gid)
	return err
}

func (e *Aria2Engine) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), e.shutdownTimeout)
	defer cancel()
	_, saveErr := e.client.call(ctx, "aria2.saveSession")
	_, shutdownErr := e.client.call(ctx, "aria2.shutdown")
	if e.cmd == nil {
		e.releaseLock()
		return errors.Join(saveErr, shutdownErr)
	}
	if e.exit == nil {
		e.watchProcess()
	}
	select {
	case err := <-e.exit:
		e.releaseLock()
		return errors.Join(saveErr, shutdownErr, err)
	case <-e.processDone:
		e.mu.Lock()
		err := e.processErr
		e.mu.Unlock()
		e.releaseLock()
		return errors.Join(saveErr, shutdownErr, err)
	case <-ctx.Done():
		_ = e.cmd.Process.Kill()
		<-e.processDone
		e.releaseLock()
		return errors.Join(saveErr, shutdownErr)
	}
}

func (e *Aria2Engine) releaseLock() {
	if e.lock == nil {
		return
	}
	_ = e.lock.Close()
	_ = os.Remove(e.lock.Name())
	e.lock = nil
}

type ManagedAria2Config struct {
	BinaryPath, DataDirectory, DownloadDirectory string
	Port                                         int
	ShutdownTimeout                              time.Duration
}

func NewManagedAria2(config ManagedAria2Config) (*Aria2Engine, error) {
	if config.BinaryPath == "" {
		return nil, errors.New("aria2 binary path is required")
	}
	if config.DataDirectory == "" {
		return nil, errors.New("aria2 data directory is required")
	}
	if config.DownloadDirectory == "" {
		return nil, errors.New("download directory is required")
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 5 * time.Second
	}
	if err := os.MkdirAll(config.DataDirectory, 0700); err != nil {
		return nil, fmt.Errorf("create aria2 data directory: %w", err)
	}
	if err := os.Chmod(config.DataDirectory, 0700); err != nil {
		return nil, fmt.Errorf("protect aria2 data directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(config.DataDirectory, "engine.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("aria2 engine is already running or its lock is unavailable: %w", err)
	}
	lockClosed := false
	defer func() {
		if !lockClosed {
			_ = lock.Close()
			_ = os.Remove(lock.Name())
		}
	}()
	if err := os.MkdirAll(config.DownloadDirectory, 0700); err != nil {
		return nil, fmt.Errorf("create download directory: %w", err)
	}
	port := config.Port
	if port == 0 {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		port = listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(secretBytes)
	session := filepath.Join(config.DataDirectory, "session.txt")
	configPath := filepath.Join(config.DataDirectory, "aria2.conf")
	if file, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY, 0600); err != nil {
		return nil, fmt.Errorf("create aria2 configuration: %w", err)
	} else {
		_ = file.Close()
	}
	if file, err := os.OpenFile(session, os.O_CREATE|os.O_WRONLY, 0600); err != nil {
		return nil, fmt.Errorf("create aria2 session: %w", err)
	} else {
		_ = file.Close()
	}
	args := []string{"--enable-rpc=true", "--rpc-listen-all=false", "--rpc-listen-port=" + strconv.Itoa(port), "--rpc-secret=" + secret, "--check-certificate=true", "--dir=" + config.DownloadDirectory, "--save-session=" + session, "--save-session-interval=30", "--input-file=" + session, "--conf-path=" + filepath.Join(config.DataDirectory, "aria2.conf")}
	cmd := exec.Command(config.BinaryPath, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start aria2: %w", err)
	}
	engine := NewAria2Engine(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port), secret)
	engine.cmd = cmd
	engine.shutdownTimeout = config.ShutdownTimeout
	engine.lock = lock
	engine.watchProcess()
	lockClosed = true
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case err := <-engine.exit:
			engine.releaseLock()
			return nil, fmt.Errorf("aria2 exited before RPC readiness: %w", err)
		default:
		}
		if _, err := engine.client.call(context.Background(), "aria2.getVersion"); err == nil {
			return engine, nil
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	<-engine.exit
	engine.releaseLock()
	return nil, fmt.Errorf("aria2 RPC did not become ready: %w", lastErr)
}
