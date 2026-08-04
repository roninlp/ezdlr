package main

import (
	"errors"
	"testing"
	"time"
)

type batchFailingEngine struct {
	*FakeEngine
	failURL string
}

func (e *batchFailingEngine) Add(url, destination string) error {
	if url == e.failURL {
		return errors.New("engine rejected URL")
	}
	return e.FakeEngine.Add(url, destination)
}

type queueFakeEngine struct {
	items map[string]EngineStatus
	gids  map[string]string
	next  int
}

func newQueueFakeEngine() *queueFakeEngine {
	return &queueFakeEngine{items: make(map[string]EngineStatus), gids: make(map[string]string)}
}

func (e *queueFakeEngine) Add(url, destination string) error {
	e.next++
	gid := formatID(e.next)
	e.items[gid] = EngineStatus{GID: gid, Status: StatePaused}
	e.gids[url] = gid
	return nil
}
func (e *queueFakeEngine) Shutdown() error { return nil }
func (e *queueFakeEngine) GID(url string) string {
	return e.gids[url]
}
func (e *queueFakeEngine) Status(gid string) (EngineStatus, error) { return e.items[gid], nil }
func (e *queueFakeEngine) Pause(gid string) error {
	e.items[gid] = EngineStatus{GID: gid, Status: StatePaused}
	return nil
}
func (e *queueFakeEngine) Resume(gid string) error {
	e.items[gid] = EngineStatus{GID: gid, Status: StateActive}
	return nil
}
func (e *queueFakeEngine) Cancel(gid string) error            { delete(e.items, gid); return nil }
func (e *queueFakeEngine) Recover() ([]EngineDownload, error) { return []EngineDownload{}, nil }
func (e *queueFakeEngine) Exited() <-chan error               { return nil }

func TestAddURLCreatesQueuedItemAndUsesConfiguredDestination(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)

	item, err := service.AddURL("  https://example.com/archive.zip  ")
	if err != nil {
		t.Fatalf("AddURL() error = %v", err)
	}
	if item.URL != "https://example.com/archive.zip" {
		t.Fatalf("URL = %q", item.URL)
	}
	if item.State != StateQueued {
		t.Fatalf("state = %q, want %q", item.State, StateQueued)
	}
	if item.Destination != "Downloads" {
		t.Fatalf("destination = %q, want Downloads", item.Destination)
	}
	if len(engine.adds) != 1 || engine.adds[0] != "https://example.com/archive.zip|Downloads" {
		t.Fatalf("engine adds = %#v", engine.adds)
	}
}

func TestAddURLRejectsUnsupportedAndCredentialBearingURLs(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	for _, test := range []struct {
		name string
		url  string
	}{
		{name: "unsupported scheme", url: "ftp://example.com/file"},
		{name: "credentials", url: "https://user:password@example.com/file"},
		{name: "missing host", url: "https:///file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.AddURL(test.url); err == nil {
				t.Fatal("AddURL() error = nil, want validation error")
			}
		})
	}
}

func TestAddURLRejectsDuplicatesWithoutCallingEngine(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	if _, err := service.AddURL("https://example.com/file"); err != nil {
		t.Fatal(err)
	}
	_, err := service.AddURL(" https://example.com/file ")
	if err == nil {
		t.Fatal("duplicate AddURL() should fail")
	}
	if len(engine.adds) != 1 {
		t.Fatalf("engine add count = %d, want 1", len(engine.adds))
	}
}

func TestReviewClipboardExtractsAndClassifiesURLsWithoutEnqueueing(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	app := NewAppWithClipboardReader(service, NewCannedClipboardReader(service, ""))
	if _, err := service.AddURL("https://example.com/existing"); err != nil {
		t.Fatal(err)
	}

	review := app.ReviewClipboard("notes https://example.com/one\nhttps://example.com/one ftp://example.com/file https://user:pass@example.com/private https:///broken https://example.com/existing")
	if review.AcceptedCount() != 1 || review.Accepted[0].URL != "https://example.com/one" {
		t.Fatalf("accepted = %#v", review.Accepted)
	}
	if review.DuplicateCount() != 2 {
		t.Fatalf("duplicate count = %d, want 2", review.DuplicateCount())
	}
	if review.RejectedCount() != 3 {
		t.Fatalf("rejected count = %d, want 3", review.RejectedCount())
	}
	if len(engine.adds) != 1 {
		t.Fatalf("review enqueued %d URLs", len(engine.adds))
	}
}

func TestConfirmClipboardUsesConfiguredDestinationAndReturnsItemLinks(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	app := NewAppWithClipboardReader(service, NewCannedClipboardReader(service, ""))
	review := app.ReviewClipboard("https://example.com/one https://example.com/two")

	result := app.ConfirmClipboard(review)
	if result.AcceptedCount() != 2 {
		t.Fatalf("accepted count = %d", result.AcceptedCount())
	}
	if len(result.Results) != 2 || result.Results[0].Item == nil || result.Results[0].Item.ID == "" {
		t.Fatalf("result item links = %#v", result.Results)
	}
	if len(engine.adds) != 2 || engine.adds[0] != "https://example.com/one|Downloads" || engine.adds[1] != "https://example.com/two|Downloads" {
		t.Fatalf("engine adds = %#v", engine.adds)
	}
}

func TestConfirmClipboardContinuesAfterPartialEnqueueFailure(t *testing.T) {
	engine := &batchFailingEngine{FakeEngine: NewFakeEngine(), failURL: "https://example.com/fail"}
	service := NewDownloadService(engine)
	app := NewAppWithClipboardReader(service, NewCannedClipboardReader(service, ""))
	review := app.ReviewClipboard("https://example.com/ok https://example.com/fail https://example.com/later")

	result := app.ConfirmClipboard(review)
	if result.AcceptedCount() != 2 || result.EnqueueFailureCount() != 1 {
		t.Fatalf("batch result = %#v", result.Results)
	}
	if len(service.Snapshot().Items) != 2 {
		t.Fatalf("queue items = %#v", service.Snapshot().Items)
	}
}

func TestClipboardReviewCancellationDoesNotEnqueue(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	app := NewAppWithClipboardReader(service, NewCannedClipboardReader(service, ""))
	app.ReviewClipboard("https://example.com/file")
	app.CancelClipboardReview()
	if len(engine.adds) != 0 || len(service.Snapshot().Items) != 0 {
		t.Fatal("cancelling review changed the queue")
	}
}

func TestShutdownClosesEngine(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	if err := service.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if !engine.shutdown {
		t.Fatal("engine was not shut down")
	}
}

func TestQueueSchedulesThreeAndProgressesFIFO(t *testing.T) {
	engine := newQueueFakeEngine()
	service := NewDownloadService(engine)
	first, _ := service.AddURL("https://example.com/1")
	second, _ := service.AddURL("https://example.com/2")
	third, _ := service.AddURL("https://example.com/3")
	fourth, _ := service.AddURL("https://example.com/4")

	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	snapshot := service.Snapshot()
	for _, item := range snapshot.Items[:3] {
		if item.State != StateActive {
			t.Fatalf("item %s state = %q, want active", item.ID, item.State)
		}
	}
	if snapshot.Items[3].State != StateQueued {
		t.Fatalf("fourth state = %q, want queued", snapshot.Items[3].State)
	}
	engine.items[first.GID] = EngineStatus{GID: first.GID, Status: StateComplete, TotalBytes: 10, CompletedBytes: 10}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	snapshot = service.Snapshot()
	if snapshot.Items[3].ID != fourth.ID || snapshot.Items[3].State != StateActive {
		t.Fatalf("queue did not progress: %#v", snapshot.Items)
	}
	if snapshot.Items[1].ID != second.ID || snapshot.Items[2].ID != third.ID {
		t.Fatalf("active order changed: %#v", snapshot.Items)
	}
}

func TestQueueLifecycleControlsPreserveItemSemantics(t *testing.T) {
	engine := newQueueFakeEngine()
	service := NewDownloadService(engine)
	item, _ := service.AddURL("https://example.com/file")
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	if err := service.Pause(item.ID); err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot().Items[0].State; got != StatePaused {
		t.Fatalf("paused state = %q", got)
	}
	if err := service.Resume(item.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Cancel(item.ID); err != nil {
		t.Fatal(err)
	}
	if len(service.Snapshot().Items) != 0 {
		t.Fatal("cancel should remove only the queue item")
	}
}

func TestQueuedItemsMoveWithoutReorderingActiveItems(t *testing.T) {
	engine := newQueueFakeEngine()
	service := NewDownloadService(engine)
	first, _ := service.AddURL("https://example.com/1")
	second, _ := service.AddURL("https://example.com/2")
	third, _ := service.AddURL("https://example.com/3")
	fourth, _ := service.AddURL("https://example.com/4")
	fifth, _ := service.AddURL("https://example.com/5")
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	if err := service.MoveDown(fourth.ID); err != nil {
		t.Fatal(err)
	}
	items := service.Snapshot().Items
	if items[0].ID != first.ID || items[1].ID != second.ID || items[2].ID != third.ID || items[3].ID != fifth.ID || items[4].ID != fourth.ID {
		t.Fatalf("queue order = %#v", items)
	}
	if err := service.MoveUp(fourth.ID); err != nil {
		t.Fatal(err)
	}
	if service.Snapshot().Items[3].ID != fourth.ID {
		t.Fatal("move up did not restore FIFO order")
	}
}

func TestQueueRetriesTransientFailuresAndStopsAtLimit(t *testing.T) {
	engine := newQueueFakeEngine()
	service := NewDownloadService(engine)
	_, _ = service.AddURL("https://example.com/file")
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		current := service.Snapshot().Items[0]
		engine.items[current.GID] = EngineStatus{GID: current.GID, Status: StateFailed}
		if err := service.loop.Tick(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(retryDelay(attempt) + 10*time.Millisecond)
		if err := service.loop.Tick(); err != nil {
			t.Fatal(err)
		}
		if attempt < 3 && service.Snapshot().Items[0].State != StateActive {
			t.Fatalf("attempt %d did not retry", attempt)
		}
	}
	if got := service.Snapshot().Items[0].State; got != StateFailed {
		t.Fatalf("exhausted state = %q, want failed", got)
	}
	if service.Snapshot().Items[0].Attempts != 3 {
		t.Fatalf("attempts = %d, want 3", service.Snapshot().Items[0].Attempts)
	}
}

func TestFailedDownloadDoesNotBlockLaterWorkAndCanBeRetried(t *testing.T) {
	engine := newQueueFakeEngine()
	service := NewDownloadService(engine)
	failed, _ := service.AddURL("https://example.com/failed")
	later, _ := service.AddURL("https://example.com/later")
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	engine.items[failed.GID] = EngineStatus{GID: failed.GID, Status: StateFailed}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	snapshot := service.Snapshot()
	if snapshot.Items[1].ID != later.ID || snapshot.Items[1].State != StateActive {
		t.Fatalf("later item did not progress: %#v", snapshot.Items)
	}
	if err := service.Retry(failed.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot().Items[0].State; got != StateActive {
		t.Fatalf("manual retry state = %q, want active", got)
	}
}
