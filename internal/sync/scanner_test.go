package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanLocal_FindsMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "doc1.md", "# Hello")
	writeFile(t, dir, "doc2.md", "# World")
	writeFile(t, dir, "readme.txt", "not markdown")

	files, err := ScanLocal(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].RelPath != "doc1.md" {
		t.Errorf("expected 'doc1.md', got %q", files[0].RelPath)
	}
	if files[1].RelPath != "doc2.md" {
		t.Errorf("expected 'doc2.md', got %q", files[1].RelPath)
	}
}

func TestScanLocal_NestedDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "root.md", "root")
	os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0755)
	writeFile(t, dir, "sub/nested.md", "nested")
	writeFile(t, dir, "sub/deep/deeper.md", "deeper")

	files, err := ScanLocal(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	paths := []string{files[0].RelPath, files[1].RelPath, files[2].RelPath}
	expected := []string{"root.md", "sub/deep/deeper.md", "sub/nested.md"}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("file %d: expected %q, got %q", i, expected[i], p)
		}
	}
}

func TestScanLocal_SkipsLocal2gdDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "doc.md", "content")
	os.MkdirAll(filepath.Join(dir, ".local2gd"), 0755)
	writeFile(t, dir, ".local2gd/state.json", "{}")
	writeFile(t, dir, ".local2gd/base/doc.md", "old content")

	files, err := ScanLocal(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file (should skip .local2gd), got %d", len(files))
	}
}

func TestScanLocal_DeterministicHash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "doc.md", "same content")

	files1, _ := ScanLocal(dir)
	files2, _ := ScanLocal(dir)

	if files1[0].Hash != files2[0].Hash {
		t.Error("hashes should be deterministic for same content")
	}
}

func TestScanLocal_DifferentContentDifferentHash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", "content A")
	writeFile(t, dir, "b.md", "content B")

	files, _ := ScanLocal(dir)
	if files[0].Hash == files[1].Hash {
		t.Error("different content should produce different hashes")
	}
}

func TestScanLocal_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	files, err := ScanLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	absPath := filepath.Join(dir, relPath)
	os.MkdirAll(filepath.Dir(absPath), 0755)
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
