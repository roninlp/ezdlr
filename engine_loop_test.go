package main

import (
	"sync"
	"testing"
	"time"
)

type countingStateStore struct {
	mu    sync.Mutex
	saves int
}

func (s *countingStateStore) Load() (persistedState, error) {
	return persistedState{Version: persistenceVersion, RetryAt: map[string]string{}}, nil
}

func (s *countingStateStore) Save(persistedState) error {
	s.mu.Lock()
	s.saves++
	s.mu.Unlock()
	return nil
}

func (s *countingStateStore) saveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saves
}

type snapshotTrackingEngine struct {
	*FakeEngine
	statusCalls int
}

func (e *snapshotTrackingEngine) Status(string) (EngineStatus, error) {
	e.statusCalls++
	return EngineStatus{Status: StateActive}, nil
}

func TestSnapshotIsPure(t *testing.T) {
	engine := &snapshotTrackingEngine{FakeEngine: NewFakeEngine()}
	service := NewDownloadService(engine)
	if _, err := service.AddURL("https://example.com/file"); err != nil {
		t.Fatal(err)
	}
	service.Snapshot()
	if engine.statusCalls != 0 {
		t.Fatalf("Snapshot() status calls = %d, want 0", engine.statusCalls)
	}
}

func TestEngineLoopBatchesDirtySignals(t *testing.T) {
	store := &countingStateStore{}
	service := NewDownloadServiceWithStore(NewFakeEngine(), store)
	for _, rawURL := range []string{
		"https://example.com/one",
		"https://example.com/two",
		"https://example.com/three",
	} {
		if _, err := service.AddURL(rawURL); err != nil {
			t.Fatal(err)
		}
	}
	service.Start()
	defer service.Shutdown()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && store.saveCount() == 0 {
		time.Sleep(time.Millisecond)
	}
	if got := store.saveCount(); got != 1 {
		t.Fatalf("batched saves = %d, want 1", got)
	}
}
