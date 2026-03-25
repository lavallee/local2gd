package sync

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const trashDir = "trash"
const defaultMaxAge = 30 * 24 * time.Hour // 30 days

// TrashEntry represents a file in the trash buffer.
type TrashEntry struct {
	RelPath    string
	TrashedAt  time.Time
	AbsPath    string
}

// MoveToTrash moves a file to .local2gd/trash/ with a timestamp suffix.
func MoveToTrash(localRoot, relPath string) error {
	srcPath := filepath.Join(localRoot, relPath)
	timestamp := time.Now().Format("2006-01-02T150405")
	trashPath := filepath.Join(localRoot, stateDir, trashDir, relPath+"."+timestamp)

	if err := os.MkdirAll(filepath.Dir(trashPath), 0755); err != nil {
		return fmt.Errorf("failed to create trash dir: %w", err)
	}

	// Copy file to trash (don't move, in case we need it)
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read file for trash: %w", err)
	}
	if err := os.WriteFile(trashPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write to trash: %w", err)
	}

	// Remove original
	if err := os.Remove(srcPath); err != nil {
		return fmt.Errorf("failed to remove original: %w", err)
	}

	slog.Debug("Moved to trash", "file", relPath, "trash", trashPath)
	return nil
}

// CleanTrash removes trashed files older than maxAge.
func CleanTrash(localRoot string, maxAge time.Duration) error {
	trashRoot := filepath.Join(localRoot, stateDir, trashDir)
	if _, err := os.Stat(trashRoot); os.IsNotExist(err) {
		return nil // no trash directory
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	err := filepath.WalkDir(trashRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return nil // skip files we can't stat
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		return nil
	})

	if removed > 0 {
		slog.Debug("Cleaned trash", "removed", removed)
	}
	return err
}

// ListTrash returns all files in the trash buffer.
func ListTrash(localRoot string) ([]TrashEntry, error) {
	trashRoot := filepath.Join(localRoot, stateDir, trashDir)
	if _, err := os.Stat(trashRoot); os.IsNotExist(err) {
		return nil, nil
	}

	var entries []TrashEntry
	err := filepath.WalkDir(trashRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel(trashRoot, path)

		// Parse timestamp from suffix
		var trashedAt time.Time
		if idx := strings.LastIndex(relPath, "."); idx > 0 {
			if t, err := time.Parse("2006-01-02T150405", relPath[idx+1:]); err == nil {
				trashedAt = t
				relPath = relPath[:idx]
			}
		}

		entries = append(entries, TrashEntry{
			RelPath:   relPath,
			TrashedAt: trashedAt,
			AbsPath:   path,
		})
		return nil
	})

	return entries, err
}
