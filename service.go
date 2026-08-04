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
	Destination    string        `json:"destination"`
	Path           string        `json:"path,omitempty"`
	AddedAt        string        `json:"addedAt"`
	TotalBytes     int64         `json:"totalBytes"`
	CompletedBytes int64         `json:"completedBytes"`
	DownloadSpeed  int64         `json:"downloadSpeed"`
	Attempts       int           `json:"attempts"`
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
	Items         []DownloadItem `json:"items"`
	Configuration Configuration  `json:"configuration"`
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

func (s *DownloadService) AddURL(rawURL string) (DownloadItem, error) {
	cleanURL, err := validateURL(rawURL)
	if err != nil {
		return DownloadItem{}, err
	}
	return s.loop.addURL(cleanURL)
}

func (s *DownloadService) Snapshot() ServiceSnapshot { return s.loop.snapshot() }

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
