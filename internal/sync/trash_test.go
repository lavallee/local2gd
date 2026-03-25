package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMoveToTrash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "doc.md", "content to trash")

	if err := MoveToTrash(dir, "doc.md"); err != nil {
		t.Fatal(err)
	}

	// Original should be gone
	if _, err := os.Stat(filepath.Join(dir, "doc.md")); !os.IsNotExist(err) {
		t.Error("expected original file to be removed")
	}

	// Trash should have the file
	entries, err := ListTrash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 trash entry, got %d", len(entries))
	}
	if entries[0].RelPath != "doc.md" {
		t.Errorf("expected relpath 'doc.md', got %q", entries[0].RelPath)
	}
}

func TestCleanTrash(t *testing.T) {
	dir := t.TempDir()
	trashRoot := filepath.Join(dir, ".local2gd", "trash")
	os.MkdirAll(trashRoot, 0755)

	// Create an old file
	oldPath := filepath.Join(trashRoot, "old.md.2020-01-01T120000")
	os.WriteFile(oldPath, []byte("old"), 0644)
	// Set modification time to the past
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)

	// Create a recent file
	recentPath := filepath.Join(trashRoot, "recent.md.2026-03-25T120000")
	os.WriteFile(recentPath, []byte("recent"), 0644)

	if err := CleanTrash(dir, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}

	// Old file should be gone
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("expected old file to be cleaned")
	}

	// Recent file should remain
	if _, err := os.Stat(recentPath); err != nil {
		t.Error("expected recent file to remain")
	}
}

func TestListTrash_Empty(t *testing.T) {
	dir := t.TempDir()
	entries, err := ListTrash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
