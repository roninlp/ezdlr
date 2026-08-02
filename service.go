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
	Attempts       int           `json:"attempts"`
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

type EngineDownload struct {
	GID         string
	URL         string
	Destination string
	Status      EngineStatus
}

type DownloadRecoveryEngine interface {
	Recover() ([]EngineDownload, error)
}

type DownloadEngineExitProvider interface {
	Exited() <-chan error
}

type DownloadLifecycleEngine interface {
	Pause(gid string) error
	Resume(gid string) error
	Cancel(gid string) error
}

type DownloadGIDProvider interface {
	GID(url string) string
}

type DownloadService struct {
	mu         sync.RWMutex
	engine     DownloadEngine
	config     Configuration
	items      []DownloadItem
	nextID     int
	retryAt    map[string]time.Time
	store      StateStore
	saveMu     sync.Mutex
	stop       chan struct{}
	done       chan struct{}
	engineDone chan struct{}
}

func NewDownloadService(engine DownloadEngine) *DownloadService {
	return newDownloadService(engine, nil)
}

func NewDownloadServiceWithStore(engine DownloadEngine, store StateStore) *DownloadService {
	return newDownloadService(engine, store)
}

func newDownloadService(engine DownloadEngine, store StateStore) *DownloadService {
	return &DownloadService{
		engine: engine,
		config: Configuration{
			DownloadDirectory: "Downloads",
			ActiveLimit:       3,
			Connections:       4,
			MaxRetries:        3,
		},
		retryAt: make(map[string]time.Time),
		store:   store,
	}
}

func (s *DownloadService) Start() {
	if s.store == nil || s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer close(s.done)
		for {
			select {
			case <-ticker.C:
				_ = s.saveState()
			case <-s.stop:
				return
			}
		}
	}()
	if observer, ok := s.engine.(DownloadEngineExitProvider); ok && observer.Exited() != nil {
		s.engineDone = make(chan struct{})
		go func() {
			defer close(s.engineDone)
			select {
			case <-observer.Exited():
				s.handleEngineExit()
			case <-s.stop:
			}
		}()
	}
}

func (s *DownloadService) handleEngineExit() {
	s.mu.Lock()
	for index := range s.items {
		if s.items[index].State == StateActive {
			s.items[index].State = StateFailed
			s.items[index].DownloadSpeed = 0
		}
	}
	s.mu.Unlock()
	_ = s.saveState()
}

func (s *DownloadService) Restore() error {
	if s.store == nil {
		return nil
	}
	state, err := s.store.Load()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.items = append([]DownloadItem(nil), state.Items...)
	s.config = state.Configuration
	if s.config.DownloadDirectory == "" {
		s.config.DownloadDirectory = "Downloads"
	}
	if s.config.ActiveLimit == 0 {
		s.config.ActiveLimit = 3
	}
	if s.config.Connections == 0 {
		s.config.Connections = 4
	}
	if s.config.MaxRetries == 0 {
		s.config.MaxRetries = 3
	}
	s.nextID = state.NextID
	if s.nextID < nextIDAfterItems(s.items) {
		s.nextID = nextIDAfterItems(s.items)
	}
	s.retryAt = make(map[string]time.Time)
	for id, value := range state.RetryAt {
		if deadline, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil {
			s.retryAt[id] = deadline
		}
	}
	s.mu.Unlock()
	if recovery, ok := s.engine.(DownloadRecoveryEngine); ok {
		if err := s.reconcile(recovery); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.scheduleLocked()
	s.mu.Unlock()
	return nil
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

func (s *DownloadService) reconcile(recovery DownloadRecoveryEngine) error {
	engineItems, err := recovery.Recover()
	if err != nil {
		return fmt.Errorf("recover engine state: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	byURL := make(map[string]EngineDownload, len(engineItems))
	for _, item := range engineItems {
		if _, exists := byURL[item.URL]; !exists {
			byURL[item.URL] = item
		}
	}
	for index := range s.items {
		item := &s.items[index]
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
		if err := s.engine.Add(item.URL, item.Destination); err != nil {
			item.State = StateFailed
			continue
		}
		if provider, ok := s.engine.(DownloadGIDProvider); ok {
			item.GID = provider.GID(item.URL)
		}
	}
	return nil
}

func (s *DownloadService) saveState() error {
	if s.store == nil {
		return nil
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	retryAt := make(map[string]string, len(s.retryAt))
	for id, deadline := range s.retryAt {
		retryAt[id] = deadline.UTC().Format(time.RFC3339Nano)
	}
	state := persistedState{Items: append([]DownloadItem(nil), s.items...), Configuration: s.config, NextID: s.nextID, RetryAt: retryAt}
	s.mu.Unlock()
	return s.store.Save(state)
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
	s.scheduleLocked()
	go s.saveState()
	return s.items[len(s.items)-1], nil
}

func (s *DownloadService) Snapshot() ServiceSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if provider, ok := s.engine.(DownloadStatusProvider); ok {
		for index := range s.items {
			if s.items[index].State == StatePaused || s.items[index].State == StateFailed || s.items[index].State == StateComplete {
				continue
			}
			if s.items[index].GID == "" {
				continue
			}
			if status, err := provider.Status(s.items[index].GID); err == nil {
				previousState := s.items[index].State
				if !(previousState == StateQueued && status.Status == StatePaused) {
					s.items[index].State = status.Status
				}
				s.items[index].TotalBytes = status.TotalBytes
				s.items[index].CompletedBytes = status.CompletedBytes
				s.items[index].DownloadSpeed = status.DownloadSpeed
				if previousState != status.Status {
					s.handleStatusLocked(index)
				}
			}
		}
	}
	s.scheduleLocked()
	result := ServiceSnapshot{Items: append([]DownloadItem(nil), s.items...), Configuration: s.config}
	go s.saveState()
	return result
}

func (s *DownloadService) MoveUp(id string) error { return s.move(id, -1) }

func (s *DownloadService) MoveDown(id string) error { return s.move(id, 1) }

func (s *DownloadService) move(id string, direction int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.itemIndex(id)
	if index < 0 || s.items[index].State != StateQueued {
		return errors.New("only queued downloads can be moved")
	}
	for adjacent := index + direction; adjacent >= 0 && adjacent < len(s.items); adjacent += direction {
		if s.items[adjacent].State == StateQueued {
			s.items[index], s.items[adjacent] = s.items[adjacent], s.items[index]
			go s.saveState()
			return nil
		}
	}
	return nil
}

func (s *DownloadService) Pause(id string) error {
	return s.control(id, StateActive, StatePaused, func(engine DownloadLifecycleEngine, gid string) error { return engine.Pause(gid) })
}

func (s *DownloadService) Resume(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.itemIndex(id)
	if index < 0 {
		return errors.New("download was not found")
	}
	if s.items[index].State == StatePaused && s.activeCountLocked() >= s.config.ActiveLimit {
		return errors.New("active download limit reached")
	}
	return s.controlLocked(index, StatePaused, StateActive, func(engine DownloadLifecycleEngine, gid string) error { return engine.Resume(gid) })
}

func (s *DownloadService) control(id string, from, state DownloadState, action func(DownloadLifecycleEngine, string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.itemIndex(id)
	if index < 0 {
		return errors.New("download was not found")
	}
	return s.controlLocked(index, from, state, action)
}

func (s *DownloadService) controlLocked(index int, from, state DownloadState, action func(DownloadLifecycleEngine, string) error) error {
	if s.items[index].State != from {
		return fmt.Errorf("download is %s, want %s", s.items[index].State, from)
	}
	engine, ok := s.engine.(DownloadLifecycleEngine)
	if !ok || s.items[index].GID == "" {
		return errors.New("download engine does not support lifecycle controls")
	}
	if err := action(engine, s.items[index].GID); err != nil {
		return err
	}
	s.items[index].State = state
	if state == StatePaused {
		delete(s.retryAt, s.items[index].ID)
	}
	s.scheduleLocked()
	go s.saveState()
	return nil
}

func (s *DownloadService) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.itemIndex(id)
	if index < 0 {
		return errors.New("download was not found")
	}
	if s.items[index].GID != "" {
		engine, ok := s.engine.(DownloadLifecycleEngine)
		if !ok {
			return errors.New("download engine does not support lifecycle controls")
		}
		if err := engine.Cancel(s.items[index].GID); err != nil {
			return err
		}
	}
	delete(s.retryAt, id)
	s.items = append(s.items[:index], s.items[index+1:]...)
	s.scheduleLocked()
	go s.saveState()
	return nil
}

func (s *DownloadService) Retry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.itemIndex(id)
	if index < 0 || s.items[index].State != StateFailed {
		return errors.New("only failed downloads can be retried")
	}
	s.items[index].Attempts = 0
	delete(s.retryAt, id)
	if err := s.retryLocked(index); err != nil {
		return err
	}
	s.scheduleLocked()
	go s.saveState()
	return nil
}

func (s *DownloadService) Configuration() Configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *DownloadService) itemIndex(id string) int {
	for index := range s.items {
		if s.items[index].ID == id {
			return index
		}
	}
	return -1
}

func (s *DownloadService) resumeLocked(index int) error {
	engine, ok := s.engine.(DownloadLifecycleEngine)
	if !ok {
		return errors.New("download engine does not support lifecycle controls")
	}
	if err := engine.Resume(s.items[index].GID); err != nil {
		return err
	}
	s.items[index].State = StateActive
	return nil
}

func (s *DownloadService) retryLocked(index int) error {
	item := &s.items[index]
	if engine, ok := s.engine.(DownloadLifecycleEngine); ok && item.GID != "" {
		if err := engine.Cancel(item.GID); err != nil {
			s.retryFailureLocked(index)
			return err
		}
	}
	if err := s.engine.Add(item.URL, item.Destination); err != nil {
		s.retryFailureLocked(index)
		return err
	}
	if provider, ok := s.engine.(DownloadGIDProvider); ok {
		item.GID = provider.GID(item.URL)
	}
	if err := s.resumeLocked(index); err != nil {
		s.retryFailureLocked(index)
		return err
	}
	return nil
}

func (s *DownloadService) retryFailureLocked(index int) {
	item := &s.items[index]
	item.State = StateFailed
	if item.Attempts < s.config.MaxRetries {
		item.Attempts++
		s.retryAt[item.ID] = time.Now().Add(retryDelay(item.Attempts))
	}
}

func (s *DownloadService) handleStatusLocked(index int) {
	item := &s.items[index]
	if item.State == StateFailed && item.Attempts < s.config.MaxRetries {
		item.Attempts++
		s.retryAt[item.ID] = time.Now().Add(retryDelay(item.Attempts))
	}
	if item.State == StateComplete || item.State == StatePaused {
		delete(s.retryAt, item.ID)
	}
}

func retryDelay(attempt int) time.Duration {
	return time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
}

func (s *DownloadService) scheduleLocked() {
	active := s.activeCountLocked()
	now := time.Now()
	for index := range s.items {
		item := &s.items[index]
		if item.State == StateFailed {
			if item.Attempts > 0 && item.Attempts < s.config.MaxRetries && !now.Before(s.retryAt[item.ID]) && active < s.config.ActiveLimit {
				if s.retryLocked(index) == nil {
					active++
					delete(s.retryAt, item.ID)
				}
			}
			continue
		}
		if item.State != StateQueued || active >= s.config.ActiveLimit {
			continue
		}
		if s.resumeLocked(index) == nil {
			active++
		}
	}
}

func (s *DownloadService) activeCountLocked() int {
	active := 0
	for index := range s.items {
		if s.items[index].State == StateActive {
			active++
		}
	}
	return active
}

func (s *DownloadService) Shutdown() error {
	if s.stop != nil {
		close(s.stop)
		<-s.done
		if s.engineDone != nil {
			<-s.engineDone
			s.engineDone = nil
		}
		s.stop = nil
	}
	saveErr := s.saveState()
	s.mu.Lock()
	defer s.mu.Unlock()
	return errors.Join(saveErr, s.engine.Shutdown())
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
