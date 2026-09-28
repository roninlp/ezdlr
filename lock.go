package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The engine lock keeps one aria2 child per user profile. It records the PID of
// the process that took it, so a lock left behind by a force-stopped or
// crashed run can be recognised as leftovers and taken over instead of
// blocking every launch after that.
const (
	engineLockName   = "engine.lock"
	lockWriteGrace   = 50 * time.Millisecond
	lockAcquireTries = 3
)

// acquireEngineLock takes the profile lock, waiting out locks that no longer
// belong to a running process. The file is created empty and the PID written
// immediately after, so a lock that has not been written yet is only treated
// as stale after lockWriteGrace has passed — that closes the window where a
// concurrent startup would otherwise steal a lock that is being taken.
func acquireEngineLock(path string) (*os.File, error) {
	var lastErr error
	for attempt := 0; attempt < lockAcquireTries; attempt++ {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			if _, writeErr := fmt.Fprintf(file, "%d\n", os.Getpid()); writeErr != nil {
				// The lock itself still stands; only its provenance is lost,
				// which the next launch treats as an empty lock.
				_ = writeErr
			}
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		lastErr = err
		if !staleEngineLock(path) {
			return nil, err
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, removeErr
		}
	}
	return nil, fmt.Errorf("engine lock could not be taken over: %w", lastErr)
}

// staleEngineLock reports whether the lock at path belongs to nobody. A lock
// naming a live process belongs to a running instance; every other shape — a
// dead PID, an empty file, an unreadable file — is a run that never released
// it.
func staleEngineLock(path string) bool {
	if pid, ok := lockOwner(path); ok {
		return !processAlive(pid)
	}
	// The owner may have created the file but not written its PID yet. Give it
	// a moment, then decide on what is actually there.
	time.Sleep(lockWriteGrace)
	if pid, ok := lockOwner(path); ok {
		return !processAlive(pid)
	}
	return true
}

func lockOwner(path string) (int, bool) {
	contents, err := os.ReadFile(path)
	if err != nil {
		// Gone or unreadable: nothing is holding it.
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// processAlive reports whether pid still belongs to a running process. Unix
// needs a zero-signal probe because FindProcess succeeds for dead processes
// there; on Windows FindProcess itself fails once the process is gone, and the
// probe is reported as unsupported, which is not "no such process".
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone) {
		return false
	}
	return true
}
