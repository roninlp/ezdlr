package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type exitFakeEngine struct {
	*FakeEngine
	exit chan error
}

func newExitFakeEngine() *exitFakeEngine {
	return &exitFakeEngine{FakeEngine: NewFakeEngine(), exit: make(chan error, 1)}
}

func (e *exitFakeEngine) Exited() <-chan error { return e.exit }

func TestJSONStateStoreRoundTripProtectsStateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "state.json")
	store := NewJSONStateStore(path)
	state := persistedState{
		Items:         []DownloadItem{{ID: "download-7", URL: "https://example.com/file", State: StatePaused}},
		Configuration: Configuration{DownloadDirectory: "Downloads", ActiveLimit: 3, Connections: 4, MaxRetries: 3},
		NextID:        7,
		RetryAt:       map[string]string{"download-7": time.Now().UTC().Format(time.RFC3339Nano)},
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.NextID != 7 || len(loaded.Items) != 1 || loaded.Items[0].State != StatePaused {
		t.Fatalf("loaded state = %#v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("state permissions = %o, want 600", info.Mode().Perm())
	}
}

type recoveryFakeEngine struct {
	items     map[string]EngineStatus
	gids      map[string]string
	next      int
	recovered []EngineDownload
	addCount  int
}

func newRecoveryFakeEngine(recovered []EngineDownload) *recoveryFakeEngine {
	return &recoveryFakeEngine{items: make(map[string]EngineStatus), gids: make(map[string]string), recovered: recovered}
}

func (e *recoveryFakeEngine) Add(url, destination string) error {
	e.addCount++
	e.next++
	gid := formatID(e.next)
	e.items[gid] = EngineStatus{GID: gid, Status: StatePaused}
	e.gids[url] = gid
	return nil
}

func (e *recoveryFakeEngine) Recover() ([]EngineDownload, error)      { return e.recovered, nil }
func (e *recoveryFakeEngine) Shutdown() error                         { return nil }
func (e *recoveryFakeEngine) Status(gid string) (EngineStatus, error) { return e.items[gid], nil }
func (e *recoveryFakeEngine) Pause(gid string) error {
	e.items[gid] = EngineStatus{GID: gid, Status: StatePaused}
	return nil
}
func (e *recoveryFakeEngine) Resume(gid string) error {
	e.items[gid] = EngineStatus{GID: gid, Status: StateActive}
	return nil
}
func (e *recoveryFakeEngine) Cancel(gid string) error { delete(e.items, gid); return nil }
func (e *recoveryFakeEngine) Exited() <-chan error    { return nil }
func (e *recoveryFakeEngine) GID(url string) string   { return e.gids[url] }

func TestRestoreReconcilesEngineStateWithoutDuplicatingTransfers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewJSONStateStore(path)
	state := persistedState{
		Items: []DownloadItem{
			{ID: "download-1", URL: "https://example.com/active", GID: "old-gid", State: StateActive, Destination: "Downloads"},
			{ID: "download-2", URL: "https://example.com/paused", GID: "paused-gid", State: StatePaused, Destination: "Downloads"},
			{ID: "download-3", URL: "https://example.com/failed", GID: "failed-gid", State: StateFailed, Destination: "Downloads", Attempts: 3},
			{ID: "download-4", URL: "https://example.com/complete", GID: "complete-gid", State: StateComplete, Destination: "Downloads"},
			{ID: "download-9", URL: "https://example.com/missing", State: StateQueued, Destination: "Downloads"},
		},
		Configuration: Configuration{DownloadDirectory: "Downloads", ActiveLimit: 3, Connections: 4, MaxRetries: 3},
		NextID:        9,
		RetryAt:       map[string]string{},
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	engine := newRecoveryFakeEngine([]EngineDownload{{
		GID: "new-gid", URL: "https://example.com/active",
		Status: EngineStatus{GID: "new-gid", Status: StateActive, TotalBytes: 100, CompletedBytes: 40},
	}})
	engine.items["new-gid"] = EngineStatus{GID: "new-gid", Status: StateActive, TotalBytes: 100, CompletedBytes: 40}
	service := NewDownloadServiceWithStore(engine, store)
	if err := service.Restore(); err != nil {
		t.Fatal(err)
	}
	items := service.Snapshot().Items
	if len(items) != 5 {
		t.Fatalf("item count = %d", len(items))
	}
	if items[0].GID != "new-gid" || items[0].CompletedBytes != 40 || items[0].State != StateActive {
		t.Fatalf("active item was not reconciled: %#v", items[0])
	}
	if items[1].State != StatePaused || items[2].State != StateFailed || items[3].State != StateComplete {
		t.Fatalf("terminal semantics changed: %#v", items)
	}
	if items[4].State != StateActive || engine.addCount != 1 {
		t.Fatalf("missing item recovery = %#v, add count = %d", items[4], engine.addCount)
	}
}

func TestRestoreReturnsEngineRecoveryFailure(t *testing.T) {
	store := NewJSONStateStore(filepath.Join(t.TempDir(), "state.json"))
	engine := newRecoveryFakeEngine(nil)
	engine.recovered = nil
	service := NewDownloadServiceWithStore(engine, store)
	if err := store.Save(persistedState{Version: persistenceVersion, Configuration: service.Configuration(), RetryAt: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if err := service.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
}

func TestServiceMarksActiveItemsFailedAfterUnexpectedEngineExit(t *testing.T) {
	engine := newExitFakeEngine()
	store := NewJSONStateStore(filepath.Join(t.TempDir(), "state.json"))
	service := NewDownloadServiceWithStore(engine, store)
	item, err := service.AddURL("https://example.com/file")
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.items[0].State = StateActive
	service.mu.Unlock()
	service.Start()
	defer service.Shutdown()
	engine.exit <- errors.New("engine crashed")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := service.Snapshot().Items[0].State; got == StateFailed {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("item %s did not become failed after engine exit", item.ID)
}
