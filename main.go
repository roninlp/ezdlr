package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed frontend/dist/*
var assets embed.FS

func main() {
	dataDirectory, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	downloadDirectory, err := defaultDownloadDirectory()
	if err != nil {
		log.Fatal(err)
	}
	var downloadEngine DownloadEngine
	if wailsBindings {
		downloadEngine = NewFakeEngine()
	} else {
		binaryPath, err := resolveAria2Binary(executable)
		if err != nil {
			log.Fatal(err)
		}
		downloadEngine, err = NewManagedAria2(ManagedAria2Config{
			BinaryPath:        binaryPath,
			DataDirectory:     filepath.Join(dataDirectory, "ezdlr", "aria2"),
			DownloadDirectory: downloadDirectory,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	store := NewJSONStateStore(filepath.Join(dataDirectory, "ezdlr", "state.json"))
	service := NewDownloadServiceWithStore(downloadEngine, store)
	if err := service.Restore(); err != nil {
		log.Fatal(err)
	}
	service.setDownloadDirectory(downloadDirectory)
	service.Start()
	app := NewApp(service)

	err = wails.Run(&options.App{
		Title:     "ezdlr",
		Width:     980,
		Height:    680,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 14, G: 18, B: 24, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
			// WebKitGTK's Wayland DMABUF renderer is not reliable on all
			// supported Mesa/driver combinations. Software compositing keeps
			// the app usable without requiring a launcher environment override.
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func resolveAria2Binary(executable string) (string, error) {
	bundledPaths := []string{
		filepath.Join(filepath.Dir(executable), "aria2c"),
		filepath.Join(filepath.Dir(executable), "..", "lib", "ezdlr", "aria2c"),
	}
	for _, bundled := range bundledPaths {
		if info, err := os.Stat(bundled); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return bundled, nil
		}
	}
	if strings.Contains(filepath.Base(executable), "-dev-") {
		if system, err := exec.LookPath("aria2c"); err == nil {
			return system, nil
		}
		return "", fmt.Errorf("aria2c is required on PATH for Wails development")
	}
	return "", fmt.Errorf("bundled aria2c not found for %s", executable)
}

func defaultDownloadDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, "Downloads"), nil
}
