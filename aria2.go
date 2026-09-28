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
	path := ""
	if len(s.Files) > 0 {
		path = s.Files[0].Path
	}
	return EngineStatus{GID: s.GID, Status: mapAria2State(s.Status), TotalBytes: parseCounter(s.TotalLength), CompletedBytes: parseCounter(s.CompletedLength), DownloadSpeed: parseCounter(s.DownloadSpeed), Path: path}
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
	client  *aria2RPCClient
	process SupervisedProcess
}

var _ DownloadEngine = (*Aria2Engine)(nil)

func NewAria2Engine(endpoint, secret string) *Aria2Engine {
	return &Aria2Engine{client: newAria2RPCClient(endpoint, secret)}
}

func newAria2Engine(process SupervisedProcess) *Aria2Engine {
	return &Aria2Engine{client: newAria2RPCClient(process.Endpoint(), process.Secret()), process: process}
}

func newAria2RPCClient(endpoint, secret string) *aria2RPCClient {
	return &aria2RPCClient{url: endpoint, secret: secret, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (e *Aria2Engine) Exited() <-chan error {
	if e.process == nil {
		return nil
	}
	return e.process.Exited()
}

func (e *Aria2Engine) Add(rawURL, destination string) (string, error) {
	result, err := e.client.call(context.Background(), "aria2.addUri", []any{rawURL}, map[string]string{"dir": destination, "check-certificate": "true", "pause": "true", "split": "4", "max-connection-per-server": "4"})
	if err != nil {
		return "", err
	}
	var gid string
	if err := json.Unmarshal(result, &gid); err != nil || gid == "" {
		return "", errors.New("aria2 returned an invalid download identifier")
	}
	return gid, nil
}

func (e *Aria2Engine) Status(gid string) (EngineStatus, error) {
	result, err := e.client.call(context.Background(), "aria2.tellStatus", gid, []string{"gid", "status", "totalLength", "completedLength", "downloadSpeed", "files"})
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
	timeout := 5 * time.Second
	if e.process != nil {
		timeout = e.process.ShutdownTimeout()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, saveErr := e.client.call(ctx, "aria2.saveSession")
	_, shutdownErr := e.client.call(ctx, "aria2.shutdown")
	if e.process == nil {
		return errors.Join(saveErr, shutdownErr)
	}
	return errors.Join(saveErr, shutdownErr, e.process.Shutdown())
}

// SupervisedProcess owns the lifecycle of the local aria2 child. The RPC
// adapter only needs its endpoint, secret, exit notification, and stop hook.
type SupervisedProcess interface {
	Endpoint() string
	Secret() string
	ShutdownTimeout() time.Duration
	Exited() <-chan error
	Shutdown() error
}

type ManagedAria2Config struct {
	BinaryPath, DataDirectory, DownloadDirectory string
	Port                                         int
	ShutdownTimeout, ReadinessTimeout            time.Duration
}

func NewManagedAria2(config ManagedAria2Config) (*Aria2Engine, error) {
	process, err := newSupervisedAria2Process(config)
	if err != nil {
		return nil, err
	}
	return newAria2Engine(process), nil
}

type supervisedAria2Process struct {
	endpoint, secret string
	cmd              *exec.Cmd
	lock             *os.File
	timeout          time.Duration
	exit             chan error
	done             chan struct{}
	mu               sync.Mutex
	err              error
	shutdownOnce     sync.Once
	shutdownErr      error
}

func newSupervisedAria2Process(config ManagedAria2Config) (*supervisedAria2Process, error) {
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
	if config.ReadinessTimeout == 0 {
		config.ReadinessTimeout = 5 * time.Second
	}
	if err := os.MkdirAll(config.DataDirectory, 0700); err != nil {
		return nil, fmt.Errorf("create aria2 data directory: %w", err)
	}
	if err := os.Chmod(config.DataDirectory, 0700); err != nil {
		return nil, fmt.Errorf("protect aria2 data directory: %w", err)
	}
	lock, err := acquireEngineLock(filepath.Join(config.DataDirectory, engineLockName))
	if err != nil {
		return nil, fmt.Errorf("aria2 engine is already running or its lock is unavailable: %w", err)
	}
	keepLock := false
	defer func() {
		if !keepLock {
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
			return nil, fmt.Errorf("allocate aria2 RPC port: %w", err)
		}
		port = listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("generate aria2 RPC secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)
	session := filepath.Join(config.DataDirectory, "session.txt")
	configPath := filepath.Join(config.DataDirectory, "aria2.conf")
	configContents := fmt.Sprintf("enable-rpc=true\nrpc-listen-all=false\nrpc-listen-port=%d\nrpc-secret=%s\ncheck-certificate=true\ndir=%s\nsave-session=%s\nsave-session-interval=30\ninput-file=%s\n", port, secret, config.DownloadDirectory, session, session)
	if err := os.WriteFile(configPath, []byte(configContents), 0600); err != nil {
		return nil, fmt.Errorf("materialize aria2 configuration: %w", err)
	}
	if file, err := os.OpenFile(session, os.O_CREATE|os.O_WRONLY, 0600); err != nil {
		return nil, fmt.Errorf("create aria2 session: %w", err)
	} else {
		_ = file.Close()
	}
	cmd := exec.Command(config.BinaryPath, "--conf-path="+configPath)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start aria2: %w", err)
	}
	process := &supervisedAria2Process{endpoint: fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port), secret: secret, cmd: cmd, lock: lock, timeout: config.ShutdownTimeout, exit: make(chan error, 1), done: make(chan struct{})}
	go process.wait()
	keepLock = true
	client := newAria2RPCClient(process.endpoint, secret)
	deadline := time.Now().Add(config.ReadinessTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case err := <-process.exit:
			process.releaseLock()
			return nil, fmt.Errorf("aria2 exited before RPC readiness: %w", err)
		default:
		}
		probeContext, cancel := context.WithDeadline(context.Background(), deadline)
		_, err = client.call(probeContext, "aria2.getVersion")
		cancel()
		if err == nil {
			return process, nil
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = process.Shutdown()
	return nil, fmt.Errorf("aria2 RPC did not become ready: %w", lastErr)
}

func (p *supervisedAria2Process) Endpoint() string               { return p.endpoint }
func (p *supervisedAria2Process) Secret() string                 { return p.secret }
func (p *supervisedAria2Process) ShutdownTimeout() time.Duration { return p.timeout }
func (p *supervisedAria2Process) Exited() <-chan error           { return p.exit }
func (p *supervisedAria2Process) wait() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
	close(p.done)
	p.exit <- err
}
func (p *supervisedAria2Process) releaseLock() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lock != nil {
		_ = p.lock.Close()
		_ = os.Remove(p.lock.Name())
		p.lock = nil
	}
}
func (p *supervisedAria2Process) Shutdown() error {
	p.shutdownOnce.Do(func() {
		select {
		case <-p.done:
			p.mu.Lock()
			p.shutdownErr = p.err
			p.mu.Unlock()
		case <-time.After(p.timeout):
			_ = p.cmd.Process.Kill()
			<-p.done
		}
		p.releaseLock()
	})
	return p.shutdownErr
}
