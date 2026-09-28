package main

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

type DownloadState string

const (
	StateQueued   DownloadState = "queued"
	StateActive   DownloadState = "active"
	StatePaused   DownloadState = "paused"
	StateFailed   DownloadState = "failed"
	StateComplete DownloadState = "complete"
)

type DownloadItem struct {
	ID             string        `json:"id"`
	URL            string        `json:"url"`
	GID            string        `json:"gid,omitempty"`
	State          DownloadState `json:"state"`
	QueueID        string        `json:"queueId"`
	QueuePaused    bool          `json:"queuePaused,omitempty"`
	Destination    string        `json:"destination"`
	Path           string        `json:"path,omitempty"`
	AddedAt        string        `json:"addedAt"`
	TotalBytes     int64         `json:"totalBytes"`
	CompletedBytes int64         `json:"completedBytes"`
	DownloadSpeed  int64         `json:"downloadSpeed"`
	Attempts       int           `json:"attempts"`
}

// DownloadQueue groups downloads behind one start/stop control. Downloads are
// only scheduled while their queue is running, so a queue that is stopped
// holds its downloads — including ones that were already running when it was
// stopped, which are paused and marked QueuePaused so starting the queue
// resumes them instead of losing them.
type DownloadQueue struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Running bool   `json:"running"`
	BuiltIn bool   `json:"builtIn"`
}

const (
	// MainQueueID is the built-in queue that single links from the intake
	// field join by default. It is the only queue that starts out running.
	MainQueueID = "main"
	// ClipboardQueueID is the queue clipboard batches land in by default.
	// It starts stopped so a pasted batch never begins downloading on its own.
	ClipboardQueueID = "clipboard"
)

func defaultQueues() []DownloadQueue {
	return []DownloadQueue{
		{ID: MainQueueID, Name: "Main", Running: true, BuiltIn: true},
		{ID: ClipboardQueueID, Name: "Clipboard", Running: false},
	}
}

func validateQueueName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("enter a name for the queue")
	}
	if len([]rune(trimmed)) > 48 {
		return "", errors.New("queue names are limited to 48 characters")
	}
	return trimmed, nil
}

type Configuration struct {
	DownloadDirectory string `json:"downloadDirectory"`
	ActiveLimit       int    `json:"activeLimit"`
	Connections       int    `json:"connections"`
	MaxRetries        int    `json:"maxRetries"`
}

func applyConfigurationDefaults(config Configuration) Configuration {
	if config.DownloadDirectory == "" {
		config.DownloadDirectory = "Downloads"
	}
	if config.ActiveLimit == 0 {
		config.ActiveLimit = 3
	}
	if config.Connections == 0 {
		config.Connections = 4
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	return config
}

func defaultConfiguration() Configuration {
	return applyConfigurationDefaults(Configuration{})
}

type ServiceSnapshot struct {
	Items         []DownloadItem  `json:"items"`
	Queues        []DownloadQueue `json:"queues"`
	Configuration Configuration   `json:"configuration"`
}

// ItemProgress is the counter-only half of the queue update. It is what the
// engine loop emits while downloads are moving: a snapshot event tells the UI
// the shape of the queue changed, a progress event only refreshes bytes,
// speed, and state for the rows that moved.
type ItemProgress struct {
	ID             string        `json:"id"`
	State          DownloadState `json:"state"`
	TotalBytes     int64         `json:"totalBytes"`
	CompletedBytes int64         `json:"completedBytes"`
	DownloadSpeed  int64         `json:"downloadSpeed"`
	Attempts       int           `json:"attempts"`
	Path           string        `json:"path,omitempty"`
}

// The engine loop pushes these to the frontend so the UI never has to poll for
// fresh transfer counters.
const (
	EventQueueSnapshot = "ezdlr:queue:snapshot"
	EventQueueProgress = "ezdlr:queue:progress"
)

// BatchFailure reports one item of a bulk action that did not apply.
type BatchFailure struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// BatchResult is the outcome of a bulk action over a selection of downloads.
type BatchResult struct {
	Succeeded []string       `json:"succeeded"`
	Failures  []BatchFailure `json:"failures"`
}

type DownloadEngine interface {
	Add(url string, destination string) (string, error)
	Status(gid string) (EngineStatus, error)
	Pause(gid string) error
	Resume(gid string) error
	Cancel(gid string) error
	Recover() ([]EngineDownload, error)
	Exited() <-chan error
	Shutdown() error
}

type EngineStatus struct {
	GID            string
	Status         DownloadState
	TotalBytes     int64
	CompletedBytes int64
	DownloadSpeed  int64
	Path           string
}

type EngineDownload struct {
	GID         string
	URL         string
	Destination string
	Status      EngineStatus
}

// DownloadService is the app-facing interface over the queue. The queue
// state and its background passes live in the engine loop module.
type DownloadService struct {
	loop *downloadEngineLoop
}

func NewDownloadService(engine DownloadEngine) *DownloadService {
	return newDownloadService(engine, nil)
}

func NewDownloadServiceWithStore(engine DownloadEngine, store StateStore) *DownloadService {
	return newDownloadService(engine, store)
}

func newDownloadService(engine DownloadEngine, store StateStore) *DownloadService {
	return &DownloadService{loop: newDownloadEngineLoop(engine, store)}
}

func (s *DownloadService) Start() { s.loop.Start() }

func (s *DownloadService) Restore() error { return s.loop.restore() }

// AddURL validates a single link and enqueues it in queueID. An empty
// queueID means the main queue, which is where the intake field sends links
// by default; links only wait when the caller picks a queue that is stopped.
func (s *DownloadService) AddURL(rawURL string, queueID string) (DownloadItem, error) {
	cleanURL, err := validateURL(rawURL)
	if err != nil {
		return DownloadItem{}, err
	}
	if queueID == "" {
		queueID = MainQueueID
	}
	return s.loop.addURL(cleanURL, queueID)
}

func (s *DownloadService) Snapshot() ServiceSnapshot { return s.loop.snapshot() }

func (s *DownloadService) CreateQueue(name string) (DownloadQueue, error) {
	return s.loop.createQueue(name)
}

func (s *DownloadService) RenameQueue(id string, name string) error {
	return s.loop.renameQueue(id, name)
}

func (s *DownloadService) DeleteQueue(id string) error { return s.loop.deleteQueue(id) }

func (s *DownloadService) StartQueue(id string) error { return s.loop.setQueueRunning(id, true) }

func (s *DownloadService) StopQueue(id string) error { return s.loop.setQueueRunning(id, false) }

// MoveToQueue repoints every listed download at queueID in one operation, so
// a context-menu selection does not turn into one IPC call per row.
func (s *DownloadService) MoveToQueue(ids []string, queueID string) BatchResult {
	if err := s.loop.queueExists(queueID); err != nil {
		return failedBatch(ids, err.Error())
	}
	return s.batch(ids, func(id string) error { return s.loop.setItemQueue(id, queueID) })
}

func (s *DownloadService) PauseBatch(ids []string) BatchResult {
	return s.batch(ids, func(id string) error { return s.loop.pauseItem(id) })
}

func (s *DownloadService) ResumeBatch(ids []string) BatchResult {
	return s.batch(ids, func(id string) error { return s.loop.resumeItem(id) })
}

func (s *DownloadService) CancelBatch(ids []string) BatchResult {
	return s.batch(ids, func(id string) error { return s.loop.cancelItem(id) })
}

func (s *DownloadService) RetryBatch(ids []string) BatchResult {
	return s.batch(ids, func(id string) error { return s.loop.retryItem(id) })
}

func (s *DownloadService) RemoveBatch(ids []string) BatchResult {
	return s.batch(ids, func(id string) error { return s.loop.removeItem(id) })
}

func (s *DownloadService) DeleteBatch(ids []string) BatchResult {
	return s.batch(ids, func(id string) error { return s.loop.deleteItem(id) })
}

func (s *DownloadService) batch(ids []string, operation func(string) error) BatchResult {
	result := BatchResult{Succeeded: []string{}, Failures: []BatchFailure{}}
	for _, id := range ids {
		if err := operation(id); err != nil {
			result.Failures = append(result.Failures, BatchFailure{ID: id, Reason: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
	}
	return result
}

// failedBatch reports every listed download as failed for one reason, used
// when a bulk action cannot apply at all — for instance when the queue it
// targets no longer exists.
func failedBatch(ids []string, reason string) BatchResult {
	result := BatchResult{Succeeded: []string{}, Failures: []BatchFailure{}}
	for _, id := range ids {
		result.Failures = append(result.Failures, BatchFailure{ID: id, Reason: reason})
	}
	return result
}

// setNotify installs the backend-to-frontend push channel. It is unexported on
// purpose: it takes a Go function, so it must never become a Wails binding.
func (s *DownloadService) setNotify(notify func(event string, data any)) {
	s.loop.setNotify(notify)
}

func (s *DownloadService) MoveUp(id string) error { return s.loop.move(id, -1) }

func (s *DownloadService) MoveDown(id string) error { return s.loop.move(id, 1) }

func (s *DownloadService) Pause(id string) error { return s.loop.pauseItem(id) }

func (s *DownloadService) Resume(id string) error { return s.loop.resumeItem(id) }

func (s *DownloadService) Cancel(id string) error { return s.loop.cancelItem(id) }

func (s *DownloadService) Retry(id string) error { return s.loop.retryItem(id) }

func (s *DownloadService) Remove(id string) error { return s.loop.removeItem(id) }

func (s *DownloadService) Delete(id string) error { return s.loop.deleteItem(id) }

func (s *DownloadService) ClearCompleted() error { return s.loop.clearCompleted() }

func (s *DownloadService) OpenFile(id string) error {
	path, _, err := s.loop.itemLocation(id)
	if err != nil {
		return err
	}
	if path == "" {
		return errors.New("the download has no file on disk")
	}
	return defaultOpener(path)
}

func (s *DownloadService) OpenDirectory(id string) error {
	path, destination, err := s.loop.itemLocation(id)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if directory == "." {
		directory = destination
	}
	if directory == "" {
		directory = "."
	}
	return defaultOpener(directory)
}

func (s *DownloadService) Configuration() Configuration { return s.loop.configuration() }

func (s *DownloadService) Shutdown() error { return s.loop.shutdown() }

func (s *DownloadService) setDownloadDirectory(directory string) {
	s.loop.setDownloadDirectory(directory)
}

func validateURL(rawURL string) (string, error) {
	cleanURL := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(cleanURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("enter a complete HTTP or HTTPS URL")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", errors.New("only HTTP and HTTPS links are supported")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.User != nil {
		return "", errors.New("URLs containing credentials are not supported")
	}
	return parsed.String(), nil
}

func formatID(number int) string {
	return fmt.Sprintf("download-%d", number+1)
}

type FakeEngine struct {
	mu       sync.Mutex
	adds     []string
	items    map[string]EngineStatus
	next     int
	shutdown bool
}

func NewFakeEngine() *FakeEngine {
	return &FakeEngine{items: make(map[string]EngineStatus)}
}

func (e *FakeEngine) Add(url string, destination string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.adds = append(e.adds, url+"|"+destination)
	e.next++
	gid := formatID(e.next)
	e.items[gid] = EngineStatus{GID: gid, Status: StatePaused}
	return gid, nil
}

func (e *FakeEngine) Shutdown() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdown = true
	return nil
}

func (e *FakeEngine) Status(gid string) (EngineStatus, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.items[gid], nil
}
func (e *FakeEngine) Pause(gid string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.items[gid] = EngineStatus{GID: gid, Status: StatePaused}
	return nil
}
func (e *FakeEngine) Resume(gid string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.items[gid] = EngineStatus{GID: gid, Status: StateActive}
	return nil
}
func (e *FakeEngine) Cancel(gid string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.items, gid)
	return nil
}
func (e *FakeEngine) Recover() ([]EngineDownload, error) { return []EngineDownload{}, nil }
func (e *FakeEngine) Exited() <-chan error               { return nil }
