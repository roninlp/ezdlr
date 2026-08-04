package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// defaultOpener reveals a file or directory with the platform default
// application. Tests replace it to avoid spawning real viewers.
var defaultOpener = openWithDefault

func openWithDefault(path string) error {
	if path == "" {
		return errors.New("the download has no file on disk")
	}
	command, args := openCommand(path)
	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	return nil
}

func openCommand(path string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{path}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", path}
	default:
		return "xdg-open", []string{path}
	}
}

func deleteDownloadFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete downloaded file: %w", err)
	}
	return nil
}
