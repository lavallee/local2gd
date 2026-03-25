package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lavallee/local2gd/internal/gdrive"
)

// LocalFile represents a markdown file found on the local filesystem.
type LocalFile struct {
	RelPath string
	AbsPath string
	Hash    string
	ModTime time.Time
}

// RemoteFile represents a Google Doc found on Google Drive.
type RemoteFile struct {
	RelPath string
	DriveID string
	Hash    string
	ModTime time.Time
}

// ScanLocal walks a local directory tree and returns all .md files with their hashes.
// The .local2gd/ directory is excluded from scanning.
func ScanLocal(rootDir string) ([]LocalFile, error) {
	var files []LocalFile

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root dir: %w", err)
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .local2gd directory
		if d.IsDir() && d.Name() == ".local2gd" {
			return filepath.SkipDir
		}

		// Skip non-markdown files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		hash := hashContent(content)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", relPath, err)
		}

		files = append(files, LocalFile{
			RelPath: relPath,
			AbsPath: path,
			Hash:    hash,
			ModTime: info.ModTime(),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan %s: %w", rootDir, err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})

	slog.Debug("Scanned local", "root", rootDir, "fileCount", len(files))
	return files, nil
}

// ScanRemote lists all Google Docs in a Drive folder and exports them for hashing.
func ScanRemote(client *gdrive.Client, folderID string) ([]RemoteFile, error) {
	entries, err := client.ListFolderRecursive(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote folder: %w", err)
	}

	var files []RemoteFile
	for _, entry := range entries {
		// Only process Google Docs
		if entry.MimeType != "application/vnd.google-apps.document" {
			continue
		}

		// Export as markdown to compute content hash
		content, err := client.ExportMarkdown(entry.ID)
		if err != nil {
			slog.Warn("Failed to export doc for hashing", "name", entry.Name, "error", err)
			continue
		}

		hash := hashContent(content)

		// Add .md extension to remote path for matching with local files
		relPath := entry.RelPath
		if !strings.HasSuffix(relPath, ".md") {
			relPath += ".md"
		}

		files = append(files, RemoteFile{
			RelPath: relPath,
			DriveID: entry.ID,
			Hash:    hash,
			ModTime: entry.ModifiedTime,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})

	slog.Debug("Scanned remote", "folderID", folderID, "fileCount", len(files))
	return files, nil
}

func hashContent(content []byte) string {
	h := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(h[:])
}
