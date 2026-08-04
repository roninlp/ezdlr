package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	service *DownloadService
	context context.Context
}

func NewApp(service *DownloadService) *App {
	return &App{service: service}
}

func (a *App) startup(ctx context.Context) { a.context = ctx }

func (a *App) shutdown(context.Context) {
	_ = a.service.Shutdown()
}

func (a *App) AddURL(url string) (DownloadItem, error) {
	return a.service.AddURL(url)
}

func (a *App) ReviewClipboard(text string) ClipboardReview {
	return a.service.ReviewClipboard(text)
}

func (a *App) ReadClipboard() (string, error) {
	return runtime.ClipboardGetText(a.context)
}

func (a *App) ConfirmClipboard(review ClipboardReview) ClipboardBatchResult {
	return a.service.ConfirmClipboard(review)
}

func (a *App) CancelClipboardReview() {
	a.service.CancelClipboardReview()
}

func (a *App) Snapshot() ServiceSnapshot {
	return a.service.Snapshot()
}

func (a *App) Configuration() Configuration {
	return a.service.Configuration()
}

func (a *App) MoveUp(id string) error { return a.service.MoveUp(id) }

func (a *App) MoveDown(id string) error { return a.service.MoveDown(id) }

func (a *App) Pause(id string) error { return a.service.Pause(id) }

func (a *App) Resume(id string) error { return a.service.Resume(id) }

func (a *App) Cancel(id string) error { return a.service.Cancel(id) }

func (a *App) Retry(id string) error { return a.service.Retry(id) }
