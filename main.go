package main

import (
	"embed"
	"log"
	"os"
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
	engine, err := NewManagedAria2(ManagedAria2Config{
		BinaryPath:        "aria2c",
		DataDirectory:     filepath.Join(dataDirectory, "ezdlr", "aria2"),
		DownloadDirectory: "Downloads",
	})
	if err != nil {
		log.Fatal(err)
	}
	app := NewApp(NewDownloadService(engine))

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
