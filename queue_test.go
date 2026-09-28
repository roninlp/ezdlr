package main

import (
	"path/filepath"
	"testing"
	"time"
)

func findQueue(queues []DownloadQueue, id string) (DownloadQueue, bool) {
	for _, queue := range queues {
		if queue.ID == id {
			return queue, true
		}
	}
	return DownloadQueue{}, false
}

func stateByID(t *testing.T, items []DownloadItem) map[string]DownloadItem {
	t.Helper()
	byID := make(map[string]DownloadItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	return byID
}

func TestQueuesStartWithRunningMainAndStoppedClipboard(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	snapshot := service.Snapshot()

	main, found := findQueue(snapshot.Queues, MainQueueID)
	if !found || !main.Running || !main.BuiltIn {
		t.Fatalf("main queue = %#v, want running and built in", main)
	}
	clipboard, found := findQueue(snapshot.Queues, ClipboardQueueID)
	if !found || clipboard.Running {
		t.Fatalf("clipboard queue = %#v, want a stopped default", clipboard)
	}

	item, err := service.AddURL("https://example.com/file", "")
	if err != nil {
		t.Fatal(err)
	}
	if item.QueueID != MainQueueID {
		t.Fatalf("single link queue = %q, want %q", item.QueueID, MainQueueID)
	}
}

func TestClipboardBatchLandsInStoppedQueueAndDoesNotStart(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)

	review := service.ReviewClipboard("https://example.com/one https://example.com/two")
	result, err := service.ConfirmClipboard(review, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.AcceptedCount() != 2 {
		t.Fatalf("accepted count = %d", result.AcceptedCount())
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	for _, item := range service.Snapshot().Items {
		if item.QueueID != ClipboardQueueID {
			t.Fatalf("batch item %s queue = %q, want %q", item.ID, item.QueueID, ClipboardQueueID)
		}
		if item.State != StateQueued {
			t.Fatalf("batch item %s state = %q, want queued while its queue is stopped", item.ID, item.State)
		}
	}
}

func TestConfirmClipboardReportsUnknownQueue(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	review := service.ReviewClipboard("https://example.com/file")
	if _, err := service.ConfirmClipboard(review, "queue-404"); err == nil {
		t.Fatal("ConfirmClipboard() error = nil, want unknown queue error")
	}
	if len(service.Snapshot().Items) != 0 {
		t.Fatal("unknown queue should not enqueue anything")
	}
}

func TestStoppedQueueHoldsItsDownloadsUntilStarted(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	queue, err := service.CreateQueue("Later")
	if err != nil {
		t.Fatal(err)
	}
	if queue.Running {
		t.Fatal("new queue started on creation")
	}

	mainItem, err := service.AddURL("https://example.com/main", "")
	if err != nil {
		t.Fatal(err)
	}
	waiting, err := service.AddURL("https://example.com/waiting", queue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}

	items := stateByID(t, service.Snapshot().Items)
	if items[mainItem.ID].State != StateActive {
		t.Fatalf("main queue item state = %q, want active", items[mainItem.ID].State)
	}
	if items[waiting.ID].State != StateQueued {
		t.Fatalf("stopped queue item state = %q, want queued", items[waiting.ID].State)
	}

	if err := service.StartQueue(queue.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	items = stateByID(t, service.Snapshot().Items)
	if items[waiting.ID].State != StateActive {
		t.Fatalf("started queue item state = %q, want active", items[waiting.ID].State)
	}
}

func TestStoppingQueuePausesRunningDownloadsAndStartingResumesThem(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	item, err := service.AddURL("https://example.com/file", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot().Items[0].State; got != StateActive {
		t.Fatalf("state = %q, want active", got)
	}

	if err := service.StopQueue(MainQueueID); err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	stopped := service.Snapshot().Items[0]
	if stopped.State != StatePaused || !stopped.QueuePaused {
		t.Fatalf("stopped queue item = %#v, want paused by its queue", stopped)
	}
	if engine.items[stopped.GID].Status != StatePaused {
		t.Fatalf("engine status = %q, want paused", engine.items[stopped.GID].Status)
	}
	if err := service.Resume(item.ID); err == nil {
		t.Fatal("resume succeeded while the queue is stopped")
	}

	if err := service.StartQueue(MainQueueID); err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	resumed := service.Snapshot().Items[0]
	if resumed.State != StateActive || resumed.QueuePaused {
		t.Fatalf("resumed item = %#v, want active and released", resumed)
	}
}

func TestQueueManagementRefusesUnsafeChanges(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())

	if _, err := service.CreateQueue("   "); err == nil {
		t.Fatal("CreateQueue() accepted a blank name")
	}
	if _, err := service.CreateQueue("Games"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateQueue("games"); err == nil {
		t.Fatal("CreateQueue() accepted a duplicate name")
	}
	if err := service.RenameQueue(MainQueueID, "  "); err == nil {
		t.Fatal("RenameQueue() accepted a blank name")
	}
	if err := service.DeleteQueue(MainQueueID); err == nil {
		t.Fatal("DeleteQueue() removed the built-in main queue")
	}

	queue, err := service.CreateQueue("Archive")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddURL("https://example.com/file", queue.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteQueue(queue.ID); err == nil {
		t.Fatal("DeleteQueue() removed a queue that still holds downloads")
	}

	if err := service.Remove(service.Snapshot().Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteQueue(queue.ID); err != nil {
		t.Fatalf("DeleteQueue() on an empty queue = %v", err)
	}
	if _, found := findQueue(service.Snapshot().Queues, queue.ID); found {
		t.Fatal("deleted queue is still in the snapshot")
	}
}

func TestMoveToQueueMovesOnlyTheSelectedDownloads(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	first, err := service.AddURL("https://example.com/1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.AddURL("https://example.com/2", "")
	if err != nil {
		t.Fatal(err)
	}
	queue, err := service.CreateQueue("Archive")
	if err != nil {
		t.Fatal(err)
	}

	result := service.MoveToQueue([]string{first.ID, second.ID}, queue.ID)
	if len(result.Succeeded) != 2 || len(result.Failures) != 0 {
		t.Fatalf("batch result = %#v", result)
	}
	items := stateByID(t, service.Snapshot().Items)
	if items[first.ID].QueueID != queue.ID || items[second.ID].QueueID != queue.ID {
		t.Fatalf("items were not moved: %#v", items)
	}

	missing := service.MoveToQueue([]string{first.ID}, "queue-404")
	if len(missing.Succeeded) != 0 || len(missing.Failures) != 1 {
		t.Fatalf("unknown queue result = %#v", missing)
	}
	if err := service.DeleteQueue(queue.ID); err == nil {
		t.Fatal("DeleteQueue() removed a queue that still holds downloads")
	}
}

func TestBatchActionsReportPerItemFailures(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	first, err := service.AddURL("https://example.com/1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.AddURL("https://example.com/2", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}

	paused := service.PauseBatch([]string{first.ID, "download-404", second.ID})
	if len(paused.Succeeded) != 2 || len(paused.Failures) != 1 {
		t.Fatalf("pause batch = %#v", paused)
	}
	if paused.Failures[0].ID != "download-404" || paused.Failures[0].Reason == "" {
		t.Fatalf("failure detail = %#v", paused.Failures[0])
	}
	for _, item := range service.Snapshot().Items {
		if item.State != StatePaused {
			t.Fatalf("item %s state = %q, want paused", item.ID, item.State)
		}
	}
}

func TestRestoreAddsQueuesToStateWrittenBeforeQueuesExisted(t *testing.T) {
	store := NewJSONStateStore(filepath.Join(t.TempDir(), "state.json"))
	state := persistedState{
		Items: []DownloadItem{{
			ID:          "download-1",
			URL:         "https://example.com/file",
			State:       StateQueued,
			Destination: "Downloads",
		}},
		Configuration: Configuration{DownloadDirectory: "Downloads", ActiveLimit: 3, Connections: 4, MaxRetries: 3},
		NextID:        1,
		RetryAt:       map[string]string{},
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	service := NewDownloadServiceWithStore(NewFakeEngine(), store)
	if err := service.Restore(); err != nil {
		t.Fatal(err)
	}

	snapshot := service.Snapshot()
	if _, found := findQueue(snapshot.Queues, MainQueueID); !found {
		t.Fatalf("queues = %#v, want a main queue", snapshot.Queues)
	}
	if _, found := findQueue(snapshot.Queues, ClipboardQueueID); !found {
		t.Fatalf("queues = %#v, want a clipboard queue", snapshot.Queues)
	}
	if snapshot.Items[0].QueueID != MainQueueID {
		t.Fatalf("restored item queue = %q, want %q", snapshot.Items[0].QueueID, MainQueueID)
	}
}

func TestJSONStateStoreRoundTripsQueues(t *testing.T) {
	store := NewJSONStateStore(filepath.Join(t.TempDir(), "profile", "state.json"))
	queues := []DownloadQueue{
		{ID: MainQueueID, Name: "Main", Running: true, BuiltIn: true},
		{ID: "queue-3", Name: "Archive", Running: false},
	}
	if err := store.Save(persistedState{
		Items:       []DownloadItem{{ID: "download-1", URL: "https://example.com/file", State: StateQueued, QueueID: "queue-3"}},
		Queues:      queues,
		NextQueueID: 4,
		NextID:      1,
		RetryAt:     map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Queues) != 2 || loaded.Queues[1].Name != "Archive" || loaded.NextQueueID != 4 {
		t.Fatalf("loaded queues = %#v, next = %d", loaded.Queues, loaded.NextQueueID)
	}
	if loaded.Items[0].QueueID != "queue-3" {
		t.Fatalf("loaded item queue = %q", loaded.Items[0].QueueID)
	}
}

func TestRestoreRaisesTheQueueCounterPastExistingIDs(t *testing.T) {
	store := NewJSONStateStore(filepath.Join(t.TempDir(), "state.json"))
	if err := store.Save(persistedState{
		Queues: []DownloadQueue{
			{ID: MainQueueID, Name: "Main", Running: true, BuiltIn: true},
			{ID: "queue-3", Name: "Archive", Running: false},
		},
		NextID:  1,
		RetryAt: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}

	service := NewDownloadServiceWithStore(NewFakeEngine(), store)
	if err := service.Restore(); err != nil {
		t.Fatal(err)
	}
	queue, err := service.CreateQueue("Games")
	if err != nil {
		t.Fatal(err)
	}
	if queue.ID != "queue-4" {
		t.Fatalf("new queue ID = %q, want %q", queue.ID, "queue-4")
	}
}

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
	item, err := service.AddURL("https://example.com/file", "")
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
	if !ok || len(snapshot.Items) != 1 || len(snapshot.Queues) == 0 {
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
