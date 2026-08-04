package main

import (
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

type downloadEngineLoop struct {
	mu              sync.Mutex
	service         *DownloadService
	dirty           chan struct{}
	wake            chan struct{}
	stop            chan struct{}
	done            chan struct{}
	exited          <-chan error
	recovery        []EngineDownload
	recoveryPending bool
	exitPending     bool
}

func newDownloadEngineLoop(service *DownloadService) *downloadEngineLoop {
	return &downloadEngineLoop{
		service: service,
		dirty:   make(chan struct{}, 1),
		wake:    make(chan struct{}, 1),
		exited:  service.engine.Exited(),
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
			_ = l.tick(true)
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
		l.service.reconcile(recovery)
		reconciled = true
	}

	if l.exitPending {
		l.service.markEngineExit()
		l.exitPending = false
		l.exited = nil
	} else if l.exited != nil {
		select {
		case <-l.exited:
			l.service.markEngineExit()
			l.exited = nil
		default:
		}
	}

	changed := l.service.refreshStatusesAndSchedule()
	if changed || reconciled {
		l.SignalDirty()
	}
	shouldSave := forceSave
	select {
	case <-l.dirty:
		shouldSave = true
	default:
	}
	if shouldSave && l.service.store != nil {
		for {
			select {
			case <-l.dirty:
			default:
				return l.service.saveState()
			}
		}
	}
	return nil
}

func (s *DownloadService) markEngineExit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.items {
		if s.items[index].State == StateActive {
			s.items[index].State = StateFailed
			s.items[index].DownloadSpeed = 0
		}
	}
	s.signalDirty()
}

func (s *DownloadService) refreshStatusesAndSchedule() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for index := range s.items {
		if s.items[index].State == StatePaused || s.items[index].State == StateFailed || s.items[index].State == StateComplete || s.items[index].GID == "" {
			continue
		}
		status, err := s.engine.Status(s.items[index].GID)
		if err != nil {
			continue
		}
		previousState := s.items[index].State
		if !(previousState == StateQueued && status.Status == StatePaused) {
			s.items[index].State = status.Status
		}
		s.items[index].TotalBytes = status.TotalBytes
		s.items[index].CompletedBytes = status.CompletedBytes
		s.items[index].DownloadSpeed = status.DownloadSpeed
		if previousState != status.Status {
			s.handleStatusLocked(index)
			changed = true
		}
	}
	scheduled := s.scheduleLocked()
	return changed || scheduled
}
