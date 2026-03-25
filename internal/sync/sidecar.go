package sync

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/lavallee/local2gd/internal/gdrive"
)

const metaDir = "meta"

// DocMetadata stores Google Doc metadata that can't be represented in markdown.
type DocMetadata struct {
	DriveFileID    string    `json:"drive_file_id"`
	Title          string    `json:"title"`
	LastModifiedBy string    `json:"last_modified_by,omitempty"`
	CreatedTime    time.Time `json:"created_time,omitempty"`
	ModifiedTime   time.Time `json:"modified_time,omitempty"`
	WebViewLink    string    `json:"web_view_link,omitempty"`
}

// FetchMetadata retrieves metadata for a Google Doc.
func FetchMetadata(client *gdrive.Client, fileID string) (*DocMetadata, error) {
	// Use the Drive API files.get for metadata
	info, err := client.GetFileInfo(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &DocMetadata{
		DriveFileID:  info.ID,
		Title:        info.Name,
		ModifiedTime: info.ModifiedTime,
	}, nil
}

// SaveSidecar writes metadata to .local2gd/meta/{relPath}.json
func SaveSidecar(localRoot, relPath string, meta *DocMetadata) error {
	path := filepath.Join(localRoot, stateDir, metaDir, relPath+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create meta dir: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write sidecar: %w", err)
	}

	slog.Debug("Saved sidecar", "path", path)
	return nil
}

// LoadSidecar reads metadata from .local2gd/meta/{relPath}.json
func LoadSidecar(localRoot, relPath string) (*DocMetadata, error) {
	path := filepath.Join(localRoot, stateDir, metaDir, relPath+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sidecar: %w", err)
	}

	var meta DocMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse sidecar: %w", err)
	}

	return &meta, nil
}

// DeleteSidecar removes the sidecar file for a given path.
func DeleteSidecar(localRoot, relPath string) error {
	path := filepath.Join(localRoot, stateDir, metaDir, relPath+".json")
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete sidecar: %w", err)
	}
	return nil
}
