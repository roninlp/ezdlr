package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func completeItem(t *testing.T, service *DownloadService, rawURL, path string) DownloadItem {
	t.Helper()
	engine := service.loop.engine.(*FakeEngine)
	item, err := service.AddURL(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	engine.items[item.GID] = EngineStatus{GID: item.GID, Status: StateComplete, Path: path}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	return service.Snapshot().Items[0]
}

func TestStatusPathPropagatesToItem(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	item := completeItem(t, service, "https://example.com/archive.zip", "/home/user/Downloads/archive.zip")
	if item.Path != "/home/user/Downloads/archive.zip" {
		t.Fatalf("item path = %q", item.Path)
	}
}

func TestRemoveItemKeepsFileOnDisk(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	file := filepath.Join(t.TempDir(), "archive.zip")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	item := completeItem(t, service, "https://example.com/archive.zip", file)

	if err := service.Remove(item.ID); err != nil {
		t.Fatal(err)
	}
	if len(service.Snapshot().Items) != 0 {
		t.Fatal("item was not removed from the list")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("file was deleted on disk: %v", err)
	}
}

func TestDeleteItemRemovesFileAndListEntry(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	file := filepath.Join(t.TempDir(), "archive.zip")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	item := completeItem(t, service, "https://example.com/archive.zip", file)

	if err := service.Delete(item.ID); err != nil {
		t.Fatal(err)
	}
	if len(service.Snapshot().Items) != 0 {
		t.Fatal("item was not removed from the list")
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestDeleteItemToleratesMissingFile(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	item := completeItem(t, service, "https://example.com/archive.zip", filepath.Join(t.TempDir(), "ghost.zip"))
	if err := service.Delete(item.ID); err != nil {
		t.Fatalf("Delete() error = %v, want success for missing file", err)
	}
}

func TestClearCompletedKeepsActiveAndFailed(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	engine := service.loop.engine.(*FakeEngine)
	first, _ := service.AddURL("https://example.com/one")
	second, _ := service.AddURL("https://example.com/two")
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}
	engine.items[first.GID] = EngineStatus{GID: first.GID, Status: StateComplete}
	engine.items[second.GID] = EngineStatus{GID: second.GID, Status: StateFailed}
	if err := service.loop.Tick(); err != nil {
		t.Fatal(err)
	}

	if err := service.ClearCompleted(); err != nil {
		t.Fatal(err)
	}
	items := service.Snapshot().Items
	if len(items) != 1 || items[0].ID != second.ID {
		t.Fatalf("items after clear = %#v", items)
	}
}

func TestOpenFileOpensItemPath(t *testing.T) {
	previous := defaultOpener
	var opened string
	defaultOpener = func(path string) error { opened = path; return nil }
	defer func() { defaultOpener = previous }()

	service := NewDownloadService(NewFakeEngine())
	item := completeItem(t, service, "https://example.com/archive.zip", "/home/user/Downloads/archive.zip")
	if err := service.OpenFile(item.ID); err != nil {
		t.Fatal(err)
	}
	if opened != "/home/user/Downloads/archive.zip" {
		t.Fatalf("opened path = %q", opened)
	}
}

func TestOpenDirectoryFallsBackToDestination(t *testing.T) {
	previous := defaultOpener
	var opened string
	defaultOpener = func(path string) error { opened = path; return nil }
	defer func() { defaultOpener = previous }()

	service := NewDownloadService(NewFakeEngine())
	item := completeItem(t, service, "https://example.com/archive.zip", "")
	if err := service.OpenDirectory(item.ID); err != nil {
		t.Fatal(err)
	}
	if opened != "Downloads" {
		t.Fatalf("opened directory = %q", opened)
	}
}

func TestOpenFileRejectsItemsWithoutFiles(t *testing.T) {
	previous := defaultOpener
	defaultOpener = func(string) error { t.Fatal("opener called"); return nil }
	defer func() { defaultOpener = previous }()

	service := NewDownloadService(NewFakeEngine())
	item := completeItem(t, service, "https://example.com/archive.zip", "")
	if err := service.OpenFile(item.ID); err == nil {
		t.Fatal("OpenFile() error = nil, want error for missing file")
	}
}

func TestRemoveDeleteClearReportMissingItem(t *testing.T) {
	service := NewDownloadService(NewFakeEngine())
	for name, task := range map[string]func(string) error{
		"Remove":        service.Remove,
		"Delete":        service.Delete,
		"OpenFile":      service.OpenFile,
		"OpenDirectory": service.OpenDirectory,
	} {
		if err := task("download-42"); err == nil {
			t.Fatalf("%s() error = nil, want not-found error", name)
		}
	}
	if err := service.ClearCompleted(); err != nil {
		t.Fatalf("ClearCompleted() error = %v", err)
	}
}
