package main

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ClipboardReader isolates the Wails runtime from clipboard intake behavior.
type ClipboardReader interface {
	ReadClipboard() (string, error)
	ReviewClipboard(string) ClipboardReview
	ConfirmClipboard(ClipboardReview) ClipboardBatchResult
	CancelClipboardReview()
}

type WailsClipboardReader struct {
	serviceClipboardReader
	app *application.App
}

func NewWailsClipboardReader(app *application.App, service *DownloadService) *WailsClipboardReader {
	return &WailsClipboardReader{serviceClipboardReader: serviceClipboardReader{service: service}, app: app}
}

func (r *WailsClipboardReader) ReadClipboard() (string, error) {
	text, ok := r.app.Clipboard.Text()
	if !ok {
		return "", errors.New("the clipboard does not contain text")
	}
	return text, nil
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

func (r *CannedClipboardReader) ReadClipboard() (string, error) { return r.text, nil }

func (r *CannedClipboardReader) CancelClipboardReview() {
	r.cancelled = true
	r.serviceClipboardReader.CancelClipboardReview()
}
