package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ClipboardReader isolates the Wails runtime from clipboard intake behavior.
type ClipboardReader interface {
	Startup(context.Context)
	ReadClipboard() (string, error)
	ReviewClipboard(string) ClipboardReview
	ConfirmClipboard(ClipboardReview) ClipboardBatchResult
	CancelClipboardReview()
}

type WailsClipboardReader struct {
	service *DownloadService
	context context.Context
}

func NewWailsClipboardReader(service *DownloadService) *WailsClipboardReader {
	return &WailsClipboardReader{service: service}
}

func (r *WailsClipboardReader) Startup(ctx context.Context) { r.context = ctx }

func (r *WailsClipboardReader) ReadClipboard() (string, error) {
	return runtime.ClipboardGetText(r.context)
}

func (r *WailsClipboardReader) ReviewClipboard(text string) ClipboardReview {
	return r.service.ReviewClipboard(text)
}

func (r *WailsClipboardReader) ConfirmClipboard(review ClipboardReview) ClipboardBatchResult {
	return r.service.ConfirmClipboard(review)
}

func (r *WailsClipboardReader) CancelClipboardReview() { r.service.CancelClipboardReview() }

// CannedClipboardReader is the runtime substitute used by clipboard tests.
type CannedClipboardReader struct {
	service *DownloadService
	text    string
}

func NewCannedClipboardReader(service *DownloadService, text string) *CannedClipboardReader {
	return &CannedClipboardReader{service: service, text: text}
}

func (r *CannedClipboardReader) Startup(context.Context) {}

func (r *CannedClipboardReader) ReadClipboard() (string, error) { return r.text, nil }

func (r *CannedClipboardReader) ReviewClipboard(text string) ClipboardReview {
	return r.service.ReviewClipboard(text)
}

func (r *CannedClipboardReader) ConfirmClipboard(review ClipboardReview) ClipboardBatchResult {
	return r.service.ConfirmClipboard(review)
}

func (r *CannedClipboardReader) CancelClipboardReview() { r.service.CancelClipboardReview() }
