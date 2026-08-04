package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// EngineLoop is the scheduling seam used by service tests.
type EngineLoop interface {
	Start()
	Stop()
	Tick() error
	SetRecovery([]EngineDownload)
	SignalDirty()
}

var _ EngineLoop = (*downloadEngineLoop)(nil)

// downloadEngineLoop is the deep module behind the queue: it owns the queue
// state — items, configuration, retry deadlines — and the passes that
// reconcile, refresh, schedule, and persist it. The download service is the
// app-facing interface over this state; the loop never calls back into it.
type downloadEngineLoop struct {
	engine DownloadEngine
	store  StateStore

	mu      sync.Mutex
	items   []DownloadItem
	config  Configuration
	nextID  int
	retryAt map[string]time.Time

	saveMu sync.Mutex

	dirty           chan struct{}
	wake            chan struct{}
	stop            chan struct{}
	done            chan struct{}
	exited          <-chan error
	recovery        []EngineDownload
	recoveryPending bool
	exitPending     bool
	exitHandled     bool
}

func newDownloadEngineLoop(engine DownloadEngine, store StateStore) *downloadEngineLoop {
	return &downloadEngineLoop{
		engine:  engine,
		store:   store,
		config:  defaultConfiguration(),
		retryAt: make(map[string]time.Time),
		dirty:   make(chan struct{}, 1),
		wake:    make(chan struct{}, 1),
		exited:  engine.Exited(),
	}
}

func (l *downloadEngineLoop) SetRecovery(recovery []EngineDownload) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recovery = append([]EngineDownload(nil), recovery...)
	l.recoveryPending = true
}

func (l *downloadEngineLoop) SignalDirty() {
	select {
	case l.dirty <- struct{}{}:
	default:
	}
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *downloadEngineLoop) Start() {
	if l.stop != nil {
		return
	}
	l.stop = make(chan struct{})
	l.done = make(chan struct{})
	go l.run()
}

func (l *downloadEngineLoop) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer close(l.done)
	_ = l.Tick()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			_ = l.tick(true)
			l.mu.Unlock()
		case <-l.wake:
			timer := time.NewTimer(time.Millisecond)
			<-timer.C
			for {
				select {
				case <-l.wake:
				default:
					_ = l.Tick()
					goto next
				}
			}
		case <-l.exited:
			l.mu.Lock()
			l.exitPending = true
			l.mu.Unlock()
			_ = l.Tick()
		case <-l.stop:
			return
		}
	next:
	}
}

func (l *downloadEngineLoop) Stop() {
	if l.stop == nil {
		return
	}
	close(l.stop)
	<-l.done
	l.stop = nil
	l.done = nil
}

func (l *downloadEngineLoop) Tick() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tick(false)
}

func (l *downloadEngineLoop) tick(forceSave bool) error {
	reconciled := false
	if l.recoveryPending {
		recovery := l.recovery
		l.recovery = nil
		l.recoveryPending = false
		l.reconcileLocked(recovery)
		reconciled = true
	}

	if l.exitPending {
		l.exitPending = false
		l.exitHandled = true
		l.markEngineExitLocked()
	} else if !l.exitHandled {
		select {
		case <-l.exited:
			l.exitHandled = true
			l.markEngineExitLocked()
		default:
		}
	}

	changed := l.refreshStatusesAndScheduleLocked()
	if changed || reconciled {
		l.SignalDirty()
	}

	shouldSave := forceSave
	select {
	case <-l.dirty:
		shouldSave = true
	default:
	}
	if shouldSave && l.store != nil {
	drain:
		for {
			select {
			case <-l.dirty:
			default:
				break drain
			}
		}
		return l.saveStateLocked()
	}
	return nil
}

func (l *downloadEngineLoop) shutdown() error {
	l.Stop()
	saveErr := l.saveState()
	return errors.Join(saveErr, l.engine.Shutdown())
}

func (l *downloadEngineLoop) restore() error {
	if l.store == nil {
		return nil
	}
	state, err := l.store.Load()
	if err != nil {
		return err
	}
	l.restoreState(state)
	recovered, err := l.engine.Recover()
	if err != nil {
		return err
	}
	l.SetRecovery(recovered)
	return nil
}

func (l *downloadEngineLoop) restoreState(state persistedState) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = append([]DownloadItem(nil), state.Items...)
	l.config = applyConfigurationDefaults(state.Configuration)
	l.nextID = state.NextID
	if l.nextID < nextIDAfterItems(l.items) {
		l.nextID = nextIDAfterItems(l.items)
	}
	l.retryAt = make(map[string]time.Time)
	for id, value := range state.RetryAt {
		if deadline, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil {
			l.retryAt[id] = deadline
		}
	}
}

func (l *downloadEngineLoop) setDownloadDirectory(directory string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.DownloadDirectory = directory
}

func (l *downloadEngineLoop) snapshot() ServiceSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return ServiceSnapshot{Items: append([]DownloadItem(nil), l.items...), Configuration: l.config}
}

func (l *downloadEngineLoop) itemsSnapshot() []DownloadItem {
	return l.snapshot().Items
}

func (l *downloadEngineLoop) configuration() Configuration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.config
}

func (l *downloadEngineLoop) addURL(cleanURL string) (DownloadItem, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hasURLLocked(cleanURL) {
		return DownloadItem{}, errors.New("that URL is already in the queue")
	}
	return l.enqueueLocked(cleanURL)
}

func (l *downloadEngineLoop) enqueueLocked(cleanURL string) (DownloadItem, error) {
	item := DownloadItem{
		ID:          formatID(l.nextID),
		URL:         cleanURL,
		State:       StateQueued,
		Destination: l.config.DownloadDirectory,
		AddedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	gid, err := l.engine.Add(item.URL, item.Destination)
	if err != nil {
		return DownloadItem{}, err
	}
	item.GID = gid
	l.nextID++
	l.items = append(l.items, item)
	l.SignalDirty()
	return item, nil
}

func (l *downloadEngineLoop) move(id string, direction int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	index := l.itemIndex(id)
	if index < 0 || l.items[index].State != StateQueued {
		return errors.New("only queued downloads can be moved")
	}
	for adjacent := index + direction; adjacent >= 0 && adjacent < len(l.items); adjacent += direction {
		if l.items[adjacent].State == StateQueued {
			l.items[index], l.items[adjacent] = l.items[adjacent], l.items[index]
			l.SignalDirty()
			return nil
		}
	}
	return nil
}

func (l *downloadEngineLoop) pauseItem(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	index := l.itemIndex(id)
	if index < 0 {
		return errors.New("download was not found")
	}
	return l.controlLocked(index, StateActive, StatePaused, func(engine DownloadEngine, gid string) error { return engine.Pause(gid) })
}

func (l *downloadEngineLoop) controlLocked(index int, from, state DownloadState, action func(DownloadEngine, string) error) error {
	if l.items[index].State != from {
		return fmt.Errorf("download is %s, want %s", l.items[index].State, from)
	}
	if l.items[index].GID == "" {
		return errors.New("download engine does not support lifecycle controls")
	}
	if err := action(l.engine, l.items[index].GID); err != nil {
		return err
	}
	l.items[index].State = state
	if state == StatePaused {
		delete(l.retryAt, l.items[index].ID)
	}
	l.SignalDirty()
	return nil
}

func (l *downloadEngineLoop) resumeItem(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	index := l.itemIndex(id)
	if index < 0 {
		return errors.New("download was not found")
	}
	if l.items[index].State == StatePaused && l.activeCountLocked() >= l.config.ActiveLimit {
		return errors.New("active download limit reached")
	}
	return l.controlLocked(index, StatePaused, StateActive, func(engine DownloadEngine, gid string) error { return engine.Resume(gid) })
}

func (l *downloadEngineLoop) cancelItem(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	index := l.itemIndex(id)
	if index < 0 {
		return errors.New("download was not found")
	}
	if l.items[index].GID != "" {
		if err := l.engine.Cancel(l.items[index].GID); err != nil {
			return err
		}
	}
	delete(l.retryAt, id)
	l.items = append(l.items[:index], l.items[index+1:]...)
	l.SignalDirty()
	return nil
}

func (l *downloadEngineLoop) retryItem(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	index := l.itemIndex(id)
	if index < 0 || l.items[index].State != StateFailed {
		return errors.New("only failed downloads can be retried")
	}
	l.items[index].Attempts = 0
	delete(l.retryAt, id)
	if err := l.retryLocked(index); err != nil {
		return err
	}
	l.SignalDirty()
	return nil
}

func (l *downloadEngineLoop) reconcileLocked(engineItems []EngineDownload) {
	byURL := make(map[string]EngineDownload, len(engineItems))
	for _, item := range engineItems {
		if _, exists := byURL[item.URL]; !exists {
			byURL[item.URL] = item
		}
	}
	for index := range l.items {
		item := &l.items[index]
		if item.State == StateFailed || item.State == StateComplete || item.State == StatePaused {
			continue
		}
		engineItem, found := byURL[item.URL]
		if found {
			item.GID = engineItem.GID
			item.State = engineItem.Status.Status
			item.TotalBytes = engineItem.Status.TotalBytes
			item.CompletedBytes = engineItem.Status.CompletedBytes
			item.DownloadSpeed = engineItem.Status.DownloadSpeed
			delete(byURL, item.URL)
			continue
		}
		item.State = StateQueued
		item.GID = ""
		gid, err := l.engine.Add(item.URL, item.Destination)
		if err != nil {
			item.State = StateFailed
			continue
		}
		item.GID = gid
	}
}

func (l *downloadEngineLoop) markEngineExitLocked() {
	for index := range l.items {
		if l.items[index].State == StateActive {
			l.items[index].State = StateFailed
			l.items[index].DownloadSpeed = 0
		}
	}
	l.SignalDirty()
}

func (l *downloadEngineLoop) refreshStatusesAndScheduleLocked() bool {
	changed := false
	for index := range l.items {
		if l.items[index].State == StatePaused || l.items[index].State == StateFailed || l.items[index].State == StateComplete || l.items[index].GID == "" {
			continue
		}
		status, err := l.engine.Status(l.items[index].GID)
		if err != nil {
			continue
		}
		previousState := l.items[index].State
		if !(previousState == StateQueued && status.Status == StatePaused) {
			l.items[index].State = status.Status
		}
		l.items[index].TotalBytes = status.TotalBytes
		l.items[index].CompletedBytes = status.CompletedBytes
		l.items[index].DownloadSpeed = status.DownloadSpeed
		if previousState != status.Status {
			l.handleStatusLocked(index)
			changed = true
		}
	}
	scheduled := l.scheduleLocked()
	return changed || scheduled
}

func (l *downloadEngineLoop) scheduleLocked() bool {
	active := l.activeCountLocked()
	now := time.Now()
	changed := false
	for index := range l.items {
		item := &l.items[index]
		if item.State == StateFailed {
			if item.Attempts > 0 && item.Attempts < l.config.MaxRetries && !now.Before(l.retryAt[item.ID]) && active < l.config.ActiveLimit {
				if l.retryLocked(index) == nil {
					active++
					delete(l.retryAt, item.ID)
					changed = true
				}
			}
			continue
		}
		if item.State != StateQueued || active >= l.config.ActiveLimit {
			continue
		}
		if l.resumeLocked(index) == nil {
			active++
			changed = true
		}
	}
	return changed
}

func (l *downloadEngineLoop) resumeLocked(index int) error {
	if l.items[index].GID == "" {
		return errors.New("download engine does not support lifecycle controls")
	}
	if err := l.engine.Resume(l.items[index].GID); err != nil {
		return err
	}
	l.items[index].State = StateActive
	return nil
}

func (l *downloadEngineLoop) retryLocked(index int) error {
	item := &l.items[index]
	if item.GID != "" {
		if err := l.engine.Cancel(item.GID); err != nil {
			l.retryFailureLocked(index)
			return err
		}
	}
	gid, err := l.engine.Add(item.URL, item.Destination)
	if err != nil {
		l.retryFailureLocked(index)
		return err
	}
	item.GID = gid
	if err := l.resumeLocked(index); err != nil {
		l.retryFailureLocked(index)
		return err
	}
	return nil
}

func (l *downloadEngineLoop) retryFailureLocked(index int) {
	item := &l.items[index]
	item.State = StateFailed
	l.armRetryLocked(item)
}

func (l *downloadEngineLoop) handleStatusLocked(index int) {
	item := &l.items[index]
	if item.State == StateFailed {
		l.armRetryLocked(item)
	}
	if item.State == StateComplete || item.State == StatePaused {
		delete(l.retryAt, item.ID)
	}
}

// armRetryLocked schedules the next automatic retry for a failed item while
// attempts remain. Callers must hold l.mu.
func (l *downloadEngineLoop) armRetryLocked(item *DownloadItem) {
	if item.Attempts < l.config.MaxRetries {
		item.Attempts++
		l.retryAt[item.ID] = time.Now().Add(retryDelay(item.Attempts))
	}
}

func (l *downloadEngineLoop) activeCountLocked() int {
	active := 0
	for index := range l.items {
		if l.items[index].State == StateActive {
			active++
		}
	}
	return active
}

func (l *downloadEngineLoop) itemIndex(id string) int {
	for index := range l.items {
		if l.items[index].ID == id {
			return index
		}
	}
	return -1
}

func (l *downloadEngineLoop) hasURLLocked(cleanURL string) bool {
	for _, item := range l.items {
		if item.URL == cleanURL {
			return true
		}
	}
	return false
}

func (l *downloadEngineLoop) persistedStateLocked() persistedState {
	retryAt := make(map[string]string, len(l.retryAt))
	for id, deadline := range l.retryAt {
		retryAt[id] = deadline.UTC().Format(time.RFC3339Nano)
	}
	return persistedState{Items: append([]DownloadItem(nil), l.items...), Configuration: l.config, NextID: l.nextID, RetryAt: retryAt}
}

// saveStateLocked writes the store outside the queue lock so disk I/O does
// not block queue operations. Callers must hold l.mu on entry and get it
// back held on return; l.saveMu serializes the writes themselves so a newer
// snapshot never lands beneath an older one.
func (l *downloadEngineLoop) saveStateLocked() error {
	if l.store == nil {
		return nil
	}
	state := l.persistedStateLocked()
	l.saveMu.Lock()
	defer l.saveMu.Unlock()
	l.mu.Unlock()
	err := l.store.Save(state)
	l.mu.Lock()
	return err
}

func (l *downloadEngineLoop) saveState() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.saveStateLocked()
}

func retryDelay(attempt int) time.Duration {
	return time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
}

func nextIDAfterItems(items []DownloadItem) int {
	next := 0
	for _, item := range items {
		var number int
		if _, err := fmt.Sscanf(item.ID, "download-%d", &number); err == nil && number > next {
			next = number
		}
	}
	return next
}
