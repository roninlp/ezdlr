package main

import "testing"

func TestCannedClipboardReaderUsesClipboardTextAdapter(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	reader := NewCannedClipboardReader(service, "https://example.com/file")
	app := NewAppWithClipboardReader(service, reader)

	text, err := app.ReadClipboard()
	if err != nil {
		t.Fatal(err)
	}
	if text != "https://example.com/file" {
		t.Fatalf("clipboard text = %q", text)
	}
	review := app.ReviewClipboard(text)
	if review.AcceptedCount() != 1 {
		t.Fatalf("accepted count = %d, want 1", review.AcceptedCount())
	}
	result, err := app.ConfirmClipboard(review, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.AcceptedCount() != 1 {
		t.Fatalf("accepted result count = %d, want 1", result.AcceptedCount())
	}
}

func TestAppCancelClipboardReviewRemainsExplicit(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	reader := NewCannedClipboardReader(service, "https://example.com/file")
	app := NewAppWithClipboardReader(service, reader)

	text, err := app.ReadClipboard()
	if err != nil {
		t.Fatal(err)
	}
	app.ReviewClipboard(text)
	app.CancelClipboardReview()

	if !reader.cancelled {
		t.Fatal("cancellation was not forwarded to clipboard reader")
	}
	if len(service.Snapshot().Items) != 0 {
		t.Fatal("cancelling review changed the queue")
	}
}
