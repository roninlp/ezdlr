package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeEngineLock(t *testing.T, directory, contents string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(directory, engineLockName)
	if err := os.WriteFile(lockPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return lockPath
}

// deadPID returns a PID that has already run and been reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	command := exec.Command("/bin/true")
	if err := command.Run(); err != nil {
		t.Fatalf("run probe process: %v", err)
	}
	return command.Process.Pid
}

func readinessFailure(t *testing.T, root string) error {
	t.Helper()
	_, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "/bin/false",
		DataDirectory:     filepath.Join(root, "runtime"),
		DownloadDirectory: filepath.Join(root, "downloads"),
		ReadinessTimeout:  time.Second,
	})
	if err == nil {
		t.Fatal("NewManagedAria2() error = nil, want the fake binary to fail readiness")
	}
	return err
}

func TestEngineLockIsTakenOverAfterAKilledRun(t *testing.T) {
	root := t.TempDir()
	lockPath := writeEngineLock(t, filepath.Join(root, "runtime"), fmt.Sprintf("%d\n", deadPID(t)))

	err := readinessFailure(t, root)
	if strings.Contains(err.Error(), "already running") {
		t.Fatalf("stale lock blocked startup: %v", err)
	}
	if _, statErr := os.Stat(lockPath); !os.IsNotExist(statErr) {
		t.Fatalf("lock left behind by the failed startup: %v", statErr)
	}
}

func TestEngineLockTakesOverALockFromAnOlderBuild(t *testing.T) {
	root := t.TempDir()
	// Older builds created the lock and never wrote to it, so an empty lock
	// names nobody. Treating it as held would brick the profile forever.
	writeEngineLock(t, filepath.Join(root, "runtime"), "")

	err := readinessFailure(t, root)
	if strings.Contains(err.Error(), "already running") {
		t.Fatalf("ownerless lock blocked startup: %v", err)
	}
}

func TestEngineLockRefusesASecondLiveInstance(t *testing.T) {
	root := t.TempDir()
	writeEngineLock(t, filepath.Join(root, "runtime"), fmt.Sprintf("%d\n", os.Getpid()))

	_, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "/bin/false",
		DataDirectory:     filepath.Join(root, "runtime"),
		DownloadDirectory: filepath.Join(root, "downloads"),
		ReadinessTimeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("live lock error = %v, want already running", err)
	}
}

func TestAcquireEngineLockWritesTheOwningPID(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), engineLockName)
	lock, err := acquireEngineLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	pid, ok := lockOwner(lockPath)
	if !ok || pid != os.Getpid() {
		t.Fatalf("lock owner = %d (%v), want pid %d", pid, ok, os.Getpid())
	}
	if !processAlive(pid) {
		t.Fatal("processAlive() reported this process as gone")
	}
	if staleEngineLock(lockPath) {
		t.Fatal("a lock owned by this process was treated as stale")
	}
}
