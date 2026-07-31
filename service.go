package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
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
	AddedAt        string        `json:"addedAt"`
	TotalBytes     int64         `json:"totalBytes"`
	CompletedBytes int64         `json:"completedBytes"`
	DownloadSpeed  int64         `json:"downloadSpeed"`
}

type Configuration struct {
	DownloadDirectory string `json:"downloadDirectory"`
	ActiveLimit       int    `json:"activeLimit"`
	Connections       int    `json:"connections"`
	MaxRetries        int    `json:"maxRetries"`
}

type ServiceSnapshot struct {
	Items         []DownloadItem `json:"items"`
	Configuration Configuration  `json:"configuration"`
}

type DownloadEngine interface {
	Add(url string, destination string) error
	Shutdown() error
}

type EngineStatus struct {
	GID            string
	Status         DownloadState
	TotalBytes     int64
	CompletedBytes int64
	DownloadSpeed  int64
}

type DownloadStatusProvider interface {
	Status(gid string) (EngineStatus, error)
}

type DownloadGIDProvider interface {
	GID(url string) string
}

type DownloadService struct {
	mu     sync.RWMutex
	engine DownloadEngine
	config Configuration
	items  []DownloadItem
	nextID int
}

func NewDownloadService(engine DownloadEngine) *DownloadService {
	return &DownloadService{
		engine: engine,
		config: Configuration{
			DownloadDirectory: "Downloads",
			ActiveLimit:       3,
			Connections:       4,
			MaxRetries:        3,
		},
	}
}

func (s *DownloadService) AddURL(rawURL string) (DownloadItem, error) {
	cleanURL, err := validateURL(rawURL)
	if err != nil {
		return DownloadItem{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.URL == cleanURL {
			return DownloadItem{}, errors.New("that URL is already in the queue")
		}
	}

	item := DownloadItem{
		ID:          formatID(s.nextID),
		URL:         cleanURL,
		State:       StateQueued,
		Destination: s.config.DownloadDirectory,
		AddedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := s.engine.Add(item.URL, item.Destination); err != nil {
		return DownloadItem{}, err
	}
	if provider, ok := s.engine.(DownloadGIDProvider); ok {
		item.GID = provider.GID(item.URL)
	}
	s.nextID++
	s.items = append(s.items, item)
	return item, nil
}

func (s *DownloadService) Snapshot() ServiceSnapshot {
	s.mu.RLock()
	items := append([]DownloadItem(nil), s.items...)
	config := s.config
	s.mu.RUnlock()

	if provider, ok := s.engine.(DownloadStatusProvider); ok {
		for index := range items {
			if items[index].GID == "" {
				continue
			}
			if status, err := provider.Status(items[index].GID); err == nil {
				items[index].State = status.Status
				items[index].TotalBytes = status.TotalBytes
				items[index].CompletedBytes = status.CompletedBytes
				items[index].DownloadSpeed = status.DownloadSpeed
			}
		}
	}
	return ServiceSnapshot{Items: items, Configuration: config}
}

func (s *DownloadService) Configuration() Configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *DownloadService) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.engine.Shutdown()
}

func validateURL(rawURL string) (string, error) {
	cleanURL := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(cleanURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("enter a complete HTTP or HTTPS URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("only HTTP and HTTPS links are supported")
	}
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
	shutdown bool
}

func NewFakeEngine() *FakeEngine {
	return &FakeEngine{}
}

func (e *FakeEngine) Add(url string, destination string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.adds = append(e.adds, url+"|"+destination)
	return nil
}

func (e *FakeEngine) Shutdown() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdown = true
	return nil
}
