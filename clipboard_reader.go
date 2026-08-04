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
	serviceClipboardReader
	context context.Context
}

func NewWailsClipboardReader(service *DownloadService) *WailsClipboardReader {
	return &WailsClipboardReader{serviceClipboardReader: serviceClipboardReader{service: service}}
}

func (r *WailsClipboardReader) Startup(ctx context.Context) { r.context = ctx }

func (r *WailsClipboardReader) ReadClipboard() (string, error) {
	return runtime.ClipboardGetText(r.context)
}

type serviceClipboardReader struct{ service *DownloadService }

func (r serviceClipboardReader) ReviewClipboard(text string) ClipboardReview {
	return r.service.ReviewClipboard(text)
}

func (r serviceClipboardReader) ConfirmClipboard(review ClipboardReview) ClipboardBatchResult {
	return r.service.ConfirmClipboard(review)
}

func (r serviceClipboardReader) CancelClipboardReview() { r.service.CancelClipboardReview() }

// CannedClipboardReader is the runtime substitute used by clipboard tests.
type CannedClipboardReader struct {
	serviceClipboardReader
	text      string
	cancelled bool
}

func NewCannedClipboardReader(service *DownloadService, text string) *CannedClipboardReader {
	return &CannedClipboardReader{serviceClipboardReader: serviceClipboardReader{service: service}, text: text}
}

func (r *CannedClipboardReader) Startup(context.Context) {}

func (r *CannedClipboardReader) ReadClipboard() (string, error) { return r.text, nil }

func (r *CannedClipboardReader) CancelClipboardReview() {
	r.cancelled = true
	r.serviceClipboardReader.CancelClipboardReview()
}
