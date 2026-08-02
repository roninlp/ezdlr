package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

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
			DownloadDirectory: "Downloads",
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
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func resolveAria2Binary(executable string) (string, error) {
	bundled := filepath.Join(filepath.Dir(executable), "aria2c")
	if _, err := os.Stat(bundled); err == nil {
		return bundled, nil
	}
	if system, err := exec.LookPath("aria2c"); err == nil {
		return system, nil
	}
	return "", fmt.Errorf("aria2c not found beside %s or on PATH", executable)
}
