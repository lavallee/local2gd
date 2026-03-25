package sync

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const stateDir = ".local2gd"
const stateFile = "state.json"
const baseDir = "base"

// SyncState tracks the state of a sync pairing.
type SyncState struct {
	Version        int                  `json:"version"`
	RemoteFolderID string               `json:"remote_folder_id"`
	LastSync       time.Time            `json:"last_sync"`
	Files          map[string]FileState `json:"files"`
}

// FileState tracks the state of a single synced file.
type FileState struct {
	DriveFileID string    `json:"drive_file_id"`
	LocalHash   string    `json:"local_hash"`
	RemoteHash  string    `json:"remote_hash"`
	LastSynced  time.Time `json:"last_synced"`
	Frontmatter []byte    `json:"frontmatter,omitempty"`
}

// NewSyncState creates an empty sync state.
func NewSyncState(remoteFolderID string) *SyncState {
	return &SyncState{
		Version:        1,
		RemoteFolderID: remoteFolderID,
		Files:          make(map[string]FileState),
	}
}

// LoadState reads the sync state from .local2gd/state.json.
// Returns a new empty state if the file doesn't exist.
func LoadState(localRoot string) (*SyncState, error) {
	path := filepath.Join(localRoot, stateDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("No state file found, starting fresh", "path", path)
			return &SyncState{
				Version: 1,
				Files:   make(map[string]FileState),
			}, nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state (may be corrupted): %w", err)
	}

	if state.Files == nil {
		state.Files = make(map[string]FileState)
	}

	return &state, nil
}

// SaveState writes the sync state to .local2gd/state.json.
func SaveState(localRoot string, state *SyncState) error {
	dir := filepath.Join(localRoot, stateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}

	state.LastSync = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	path := filepath.Join(dir, stateFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	slog.Debug("State saved", "path", path, "files", len(state.Files))
	return nil
}

// SaveBase writes a base copy of a file for three-way merge.
func SaveBase(localRoot, relPath string, content []byte) error {
	path := filepath.Join(localRoot, stateDir, baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create base dir: %w", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write base copy: %w", err)
	}
	return nil
}

// LoadBase reads the base copy of a file.
func LoadBase(localRoot, relPath string) ([]byte, error) {
	path := filepath.Join(localRoot, stateDir, baseDir, relPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No base copy is not an error
		}
		return nil, fmt.Errorf("failed to read base copy: %w", err)
	}
	return data, nil
}

// DeleteBase removes the base copy of a file.
func DeleteBase(localRoot, relPath string) error {
	path := filepath.Join(localRoot, stateDir, baseDir, relPath)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete base copy: %w", err)
	}
	return nil
}
