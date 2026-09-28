package main

import (
	"testing"
	"time"
)

type recordedUpdate struct {
	event string
	data  any
}

func waitForUpdate(t *testing.T, updates <-chan recordedUpdate) recordedUpdate {
	t.Helper()
	select {
	case update := <-updates:
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a queue update")
		return recordedUpdate{}
	}
}

func TestQueueUpdatesArePushedInsteadOfPolled(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	item, err := service.AddURL("https://example.com/file")
	if err != nil {
		t.Fatal(err)
	}

	updates := make(chan recordedUpdate, 8)
	service.setNotify(func(event string, data any) {
		updates <- recordedUpdate{event: event, data: data}
	})

	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	first := waitForUpdate(t, updates)
	if first.event != EventQueueSnapshot {
		t.Fatalf("first update = %q, want %q", first.event, EventQueueSnapshot)
	}
	snapshot, ok := first.data.(ServiceSnapshot)
	if !ok || len(snapshot.Items) != 1 {
		t.Fatalf("snapshot payload = %#v", first.data)
	}

	// Counters moving is a progress-only update: the shape of the queue did
	// not change, so the payload is one row rather than the whole snapshot.
	engine.items[item.GID] = EngineStatus{
		GID:            item.GID,
		Status:         StateActive,
		TotalBytes:     100,
		CompletedBytes: 40,
		DownloadSpeed:  10,
	}
	service.loop.refreshPass()

	second := waitForUpdate(t, updates)
	if second.event != EventQueueProgress {
		t.Fatalf("second update = %q, want %q", second.event, EventQueueProgress)
	}
	progress, ok := second.data.([]ItemProgress)
	if !ok || len(progress) != 1 {
		t.Fatalf("progress payload = %#v", second.data)
	}
	if progress[0].ID != item.ID || progress[0].CompletedBytes != 40 || progress[0].DownloadSpeed != 10 {
		t.Fatalf("progress row = %#v", progress[0])
	}

	// An unchanged queue stays silent.
	service.loop.refreshPass()
	select {
	case update := <-updates:
		t.Fatalf("idle pass still emitted %q", update.event)
	case <-time.After(50 * time.Millisecond):
	}
}
