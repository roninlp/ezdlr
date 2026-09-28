package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	*DownloadService
	clipboard ClipboardReader
}

func NewApp(wailsApp *application.App, service *DownloadService) *App {
	return NewAppWithClipboardReader(service, NewWailsClipboardReader(wailsApp, service))
}

func NewAppWithClipboardReader(service *DownloadService, clipboard ClipboardReader) *App {
	return &App{DownloadService: service, clipboard: clipboard}
}

func (a *App) ServiceShutdown() error {
	return a.DownloadService.Shutdown()
}

func (a *App) ReviewClipboard(text string) ClipboardReview {
	return a.clipboard.ReviewClipboard(text)
}

func (a *App) ReadClipboard() (string, error) {
	return a.clipboard.ReadClipboard()
}

func (a *App) ConfirmClipboard(review ClipboardReview, queueID string) (ClipboardBatchResult, error) {
	return a.clipboard.ConfirmClipboard(review, queueID)
}

func (a *App) CancelClipboardReview() {
	a.clipboard.CancelClipboardReview()
}
