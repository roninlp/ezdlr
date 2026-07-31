package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	if err := engine.Add("https://example.com/file", "Downloads"); err != nil {
		t.Fatal(err)
	}
	if got := engine.GID("https://example.com/file"); got != "gid-1" {
		t.Fatalf("GID() = %q", got)
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
