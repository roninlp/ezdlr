package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSupervisedProcess struct {
	endpoint   string
	secret     string
	exited     chan error
	shutdown   int
	shutdownFn func()
}

func (p *fakeSupervisedProcess) Endpoint() string               { return p.endpoint }
func (p *fakeSupervisedProcess) Secret() string                 { return p.secret }
func (p *fakeSupervisedProcess) ShutdownTimeout() time.Duration { return 5 * time.Second }
func (p *fakeSupervisedProcess) Exited() <-chan error           { return p.exited }
func (p *fakeSupervisedProcess) Shutdown() error {
	p.shutdown++
	if p.shutdownFn != nil {
		p.shutdownFn()
	}
	return nil
}

var _ SupervisedProcess = (*fakeSupervisedProcess)(nil)

func TestAria2EngineSerializesRPCAndMapsStatus(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
		method := request["method"].(string)
		result := any("gid-1")
		if method == "aria2.tellStatus" {
			result = map[string]string{"gid": "gid-1", "status": "active", "totalLength": "1000", "completedLength": "250", "downloadSpeed": "80"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request["id"], "result": result})
	}))
	defer server.Close()

	engine := NewAria2Engine(server.URL, "private-secret")
	gid, err := engine.Add("https://example.com/file", "Downloads")
	if err != nil {
		t.Fatal(err)
	}
	if gid != "gid-1" {
		t.Fatalf("Add() gid = %q", gid)
	}
	status, err := engine.Status("gid-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != StateActive || status.CompletedBytes != 250 || status.DownloadSpeed != 80 {
		t.Fatalf("status = %#v", status)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	for _, request := range requests {
		params := request["params"].([]any)
		if params[0] != "token:private-secret" {
			t.Fatalf("secret was not sent in params: %#v", params)
		}
	}
	addParams := requests[0]["params"].([]any)
	if addParams[1].([]any)[0] != "https://example.com/file" {
		t.Fatalf("add params = %#v", addParams)
	}
}

func TestAria2EngineShutdownDelegatesToSupervisedProcess(t *testing.T) {
	shutdown := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["method"] != "aria2.saveSession" && request["method"] != "aria2.shutdown" {
			t.Fatalf("unexpected method %v", request["method"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request["id"], "result": nil})
	}))
	defer server.Close()
	process := &fakeSupervisedProcess{endpoint: server.URL, secret: "secret", exited: make(chan error, 1), shutdownFn: func() { close(shutdown) }}
	engine := newAria2Engine(process)
	if err := engine.Shutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-shutdown:
	default:
		t.Fatal("supervised process was not shut down")
	}
	if process.shutdown != 1 {
		t.Fatalf("Shutdown() calls = %d", process.shutdown)
	}
}

func TestSupervisedAria2ProcessExitBeforeReadinessReleasesLock(t *testing.T) {
	root := t.TempDir()
	_, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "/bin/false",
		DataDirectory:     filepath.Join(root, "runtime"),
		DownloadDirectory: filepath.Join(root, "downloads"),
		ReadinessTimeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "exited before RPC readiness") {
		t.Fatalf("NewManagedAria2() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime", "engine.lock")); !os.IsNotExist(err) {
		t.Fatalf("engine lock still exists: %v", err)
	}
}

func TestSupervisedAria2ProcessSetupIsDeterministicWithoutAria2(t *testing.T) {
	root := t.TempDir()
	runtime := filepath.Join(root, "runtime")
	if err := os.MkdirAll(runtime, 0700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(runtime, engineLockName)
	// A lock written by a live instance still guards the profile.
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "/bin/false",
		DataDirectory:     runtime,
		DownloadDirectory: filepath.Join(root, "downloads"),
	}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("lock contention error = %v", err)
	}
	_ = os.Remove(lockPath)

	binary := filepath.Join(root, "fake-aria2")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec /bin/sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        binary,
		DataDirectory:     runtime,
		DownloadDirectory: filepath.Join(root, "downloads"),
		ReadinessTimeout:  50 * time.Millisecond,
		ShutdownTimeout:   50 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "RPC did not become ready") {
		t.Fatalf("readiness timeout error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("readiness timeout took %s", elapsed)
	}
	config, err := os.ReadFile(filepath.Join(runtime, "aria2.conf"))
	if err != nil {
		t.Fatal(err)
	}
	contents := string(config)
	if !strings.Contains(contents, "rpc-listen-port=") || !strings.Contains(contents, "rpc-secret=") {
		t.Fatalf("configuration did not contain allocated RPC settings: %q", contents)
	}
	if secret := strings.Split(strings.Split(contents, "rpc-secret=")[1], "\n")[0]; len(secret) != 64 {
		t.Fatalf("generated secret was not 32 bytes: %q", secret)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("engine lock still exists after readiness timeout: %v", err)
	}
}

func TestAria2EngineExitedUsesSupervisedProcessChannel(t *testing.T) {
	exited := make(chan error, 1)
	engine := newAria2Engine(&fakeSupervisedProcess{exited: exited})
	if got := engine.Exited(); got != exited {
		t.Fatal("Exited() did not return the supervised process channel")
	}
}

func TestSupervisedAria2ProcessShutdownIsBounded(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	process := &supervisedAria2Process{
		cmd:     cmd,
		timeout: 20 * time.Millisecond,
		lock:    nil,
		done:    make(chan struct{}),
		exit:    make(chan error, 1),
	}
	go process.wait()
	started := time.Now()
	if err := process.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Shutdown() took %s", elapsed)
	}
}

func TestNewManagedAria2WaitsForReadinessAndShutsDown(t *testing.T) {
	if _, err := os.Stat("/usr/bin/aria2c"); err != nil {
		t.Skip("aria2c is not installed")
	}
	root := t.TempDir()
	engine, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "/usr/bin/aria2c",
		DataDirectory:     filepath.Join(root, "runtime"),
		DownloadDirectory: filepath.Join(root, "downloads"),
		ShutdownTimeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(engine.client.url, "http://127.0.0.1:") {
		t.Fatalf("RPC endpoint = %q", engine.client.url)
	}
	if err := engine.Shutdown(); err != nil && !strings.Contains(err.Error(), "process already finished") {
		t.Fatal(err)
	}
}

func TestNewManagedAria2ReportsReadinessProcessExit(t *testing.T) {
	root := t.TempDir()
	_, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "/bin/false",
		DataDirectory:     filepath.Join(root, "runtime"),
		DownloadDirectory: filepath.Join(root, "downloads"),
	})
	if err == nil || !strings.Contains(err.Error(), "exited before RPC readiness") {
		t.Fatalf("NewManagedAria2() error = %v", err)
	}
}

func TestNewManagedAria2PreventsConcurrentProfileInstances(t *testing.T) {
	if _, err := os.Stat("/usr/bin/aria2c"); err != nil {
		t.Skip("aria2c is not installed")
	}
	root := t.TempDir()
	config := ManagedAria2Config{
		BinaryPath:        "/usr/bin/aria2c",
		DataDirectory:     filepath.Join(root, "runtime"),
		DownloadDirectory: filepath.Join(root, "downloads"),
	}
	first, err := NewManagedAria2(config)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Shutdown()
	if _, err := NewManagedAria2(config); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second managed instance error = %v", err)
	}
}
