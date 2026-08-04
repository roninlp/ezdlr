package main

import (
	"context"
)

type App struct {
	*DownloadService
	clipboard ClipboardReader
}

func NewApp(service *DownloadService) *App {
	return NewAppWithClipboardReader(service, NewWailsClipboardReader(service))
}

func NewAppWithClipboardReader(service *DownloadService, clipboard ClipboardReader) *App {
	return &App{DownloadService: service, clipboard: clipboard}
}

func (a *App) startup(ctx context.Context) { a.clipboard.Startup(ctx) }

func (a *App) shutdown(context.Context) {
	_ = a.DownloadService.Shutdown()
}

func (a *App) ReviewClipboard(text string) ClipboardReview {
	return a.clipboard.ReviewClipboard(text)
}

func (a *App) ReadClipboard() (string, error) {
	return a.clipboard.ReadClipboard()
}

func (a *App) ConfirmClipboard(review ClipboardReview) ClipboardBatchResult {
	return a.clipboard.ConfirmClipboard(review)
}

func (a *App) CancelClipboardReview() {
	a.clipboard.CancelClipboardReview()
}
