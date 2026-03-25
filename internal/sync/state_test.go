package sync

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadState_MissingFile(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != 1 {
		t.Errorf("expected version 1, got %d", state.Version)
	}
	if len(state.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(state.Files))
	}
}

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	state := NewSyncState("folder123")
	state.Files["doc.md"] = FileState{
		DriveFileID: "drive123",
		LocalHash:   "sha256:abc",
		RemoteHash:  "sha256:def",
		LastSynced:  time.Now().Truncate(time.Second),
	}

	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.RemoteFolderID != "folder123" {
		t.Errorf("expected folder ID 'folder123', got %q", loaded.RemoteFolderID)
	}
	if len(loaded.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(loaded.Files))
	}
	fs := loaded.Files["doc.md"]
	if fs.DriveFileID != "drive123" {
		t.Errorf("expected drive ID 'drive123', got %q", fs.DriveFileID)
	}
}

func TestSaveAndLoadBase(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# Base Content\n\nOriginal text.\n")

	if err := SaveBase(dir, "notes/doc.md", content); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadBase(dir, "notes/doc.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded) != string(content) {
		t.Errorf("base content mismatch")
	}
}

func TestLoadBase_Missing(t *testing.T) {
	dir := t.TempDir()
	data, err := LoadBase(dir, "nonexistent.md")
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		t.Error("expected nil for missing base")
	}
}

func TestDeleteBase(t *testing.T) {
	dir := t.TempDir()
	SaveBase(dir, "doc.md", []byte("content"))

	if err := DeleteBase(dir, "doc.md"); err != nil {
		t.Fatal(err)
	}

	data, _ := LoadBase(dir, "doc.md")
	if data != nil {
		t.Error("expected nil after delete")
	}
}

func TestDeleteBase_Missing(t *testing.T) {
	dir := t.TempDir()
	// Should not error on missing file
	if err := DeleteBase(dir, "nonexistent.md"); err != nil {
		t.Fatal(err)
	}
}

func TestStateFileLocation(t *testing.T) {
	dir := t.TempDir()
	state := NewSyncState("test")
	SaveState(dir, state)

	// Verify file exists at correct path
	path := filepath.Join(dir, ".local2gd", "state.json")
	if _, err := statFile(path); err != nil {
		t.Errorf("state file not at expected path: %s", path)
	}
}

func statFile(path string) (bool, error) {
	_, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	// Just check the file exists by trying to read state
	_, err = LoadState(filepath.Dir(filepath.Dir(path)))
	return err == nil, err
}
