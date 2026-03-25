package sync

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// gitAvailable checks if git is installed.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// ThreeWayMerge performs a three-way merge using git merge-file.
// Returns the merged content and whether there were conflicts.
// If git is not available, returns an error.
func ThreeWayMerge(base, local, remote []byte) (merged []byte, hasConflicts bool, err error) {
	if !gitAvailable() {
		return nil, false, fmt.Errorf("git not found — three-way merge requires git")
	}

	// Write to temp files
	dir, err := os.MkdirTemp("", "local2gd-merge-*")
	if err != nil {
		return nil, false, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	basePath := filepath.Join(dir, "base")
	localPath := filepath.Join(dir, "local")
	remotePath := filepath.Join(dir, "remote")

	if err := os.WriteFile(basePath, base, 0600); err != nil {
		return nil, false, fmt.Errorf("failed to write base: %w", err)
	}
	if err := os.WriteFile(localPath, local, 0600); err != nil {
		return nil, false, fmt.Errorf("failed to write local: %w", err)
	}
	if err := os.WriteFile(remotePath, remote, 0600); err != nil {
		return nil, false, fmt.Errorf("failed to write remote: %w", err)
	}

	// Run git merge-file -p --diff3 local base remote
	// -p: write to stdout instead of modifying the file
	// --diff3: include the base version in conflict markers
	cmd := exec.Command("git", "merge-file", "-p", "--diff3",
		"-L", "LOCAL",
		"-L", "BASE",
		"-L", "REMOTE",
		localPath, basePath, remotePath)

	output, err := cmd.Output()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, false, fmt.Errorf("git merge-file failed: %w", err)
		}
	}

	switch {
	case exitCode == 0:
		slog.Debug("Three-way merge: clean")
		return output, false, nil
	case exitCode > 0:
		slog.Debug("Three-way merge: conflicts detected")
		return output, true, nil
	default:
		return nil, false, fmt.Errorf("git merge-file failed with exit code %d", exitCode)
	}
}
