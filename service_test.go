package main

import (
	"testing"
)

func TestAddURLCreatesQueuedItemAndUsesConfiguredDestination(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)

	item, err := service.AddURL("  https://example.com/archive.zip  ")
	if err != nil {
		t.Fatalf("AddURL() error = %v", err)
	}
	if item.URL != "https://example.com/archive.zip" {
		t.Fatalf("URL = %q", item.URL)
	}
	if item.State != StateQueued {
		t.Fatalf("state = %q, want %q", item.State, StateQueued)
	}
	if item.Destination != "Downloads" {
		t.Fatalf("destination = %q, want Downloads", item.Destination)
	}
	if len(engine.adds) != 1 || engine.adds[0] != "https://example.com/archive.zip|Downloads" {
		t.Fatalf("engine adds = %#v", engine.adds)
	}
}

func TestAddURLRejectsUnsupportedAndCredentialBearingURLs(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	for _, test := range []struct {
		name string
		url  string
	}{
		{name: "unsupported scheme", url: "ftp://example.com/file"},
		{name: "credentials", url: "https://user:password@example.com/file"},
		{name: "missing host", url: "https:///file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.AddURL(test.url); err == nil {
				t.Fatal("AddURL() error = nil, want validation error")
			}
		})
	}
}

func TestAddURLRejectsDuplicatesWithoutCallingEngine(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	if _, err := service.AddURL("https://example.com/file"); err != nil {
		t.Fatal(err)
	}
	_, err := service.AddURL(" https://example.com/file ")
	if err == nil {
		t.Fatal("duplicate AddURL() should fail")
	}
	if len(engine.adds) != 1 {
		t.Fatalf("engine add count = %d, want 1", len(engine.adds))
	}
}

func TestShutdownClosesEngine(t *testing.T) {
	engine := NewFakeEngine()
	service := NewDownloadService(engine)
	if err := service.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if !engine.shutdown {
		t.Fatal("engine was not shut down")
	}
}
