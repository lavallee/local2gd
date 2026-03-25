package sync

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lavallee/local2gd/internal/convert"
	"github.com/lavallee/local2gd/internal/gdrive"
)

// PairingConfig defines a local↔remote sync pairing.
type PairingConfig struct {
	Name       string
	LocalDir   string
	RemotePath string
}

// Report summarizes the results of a sync operation.
type Report struct {
	Pushed    int
	Pulled    int
	Created   int
	Deleted   int
	Conflicts int
	Skipped   int
	Errors    []error
}

func (r *Report) String() string {
	var parts []string
	if r.Created > 0 {
		parts = append(parts, fmt.Sprintf("%d created", r.Created))
	}
	if r.Pushed > 0 {
		parts = append(parts, fmt.Sprintf("%d pushed", r.Pushed))
	}
	if r.Pulled > 0 {
		parts = append(parts, fmt.Sprintf("%d pulled", r.Pulled))
	}
	if r.Deleted > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", r.Deleted))
	}
	if r.Conflicts > 0 {
		parts = append(parts, fmt.Sprintf("%d conflicts", r.Conflicts))
	}
	if r.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", r.Skipped))
	}
	if len(r.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", len(r.Errors)))
	}
	if len(parts) == 0 {
		return "Everything up to date."
	}
	return strings.Join(parts, ", ")
}

// Engine orchestrates the sync operation.
type Engine struct {
	client   *gdrive.Client
	config   PairingConfig
	noDelete bool
}

// NewEngine creates a new sync engine.
func NewEngine(client *gdrive.Client, config PairingConfig) *Engine {
	return &Engine{
		client: client,
		config: config,
	}
}

// SetNoDelete disables deletion propagation.
func (e *Engine) SetNoDelete(noDelete bool) {
	e.noDelete = noDelete
}

// Run executes the sync operation.
func (e *Engine) Run(dryRun bool) (*Report, error) {
	report := &Report{}

	// Resolve local dir
	localDir, err := expandPath(e.config.LocalDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve local dir: %w", err)
	}

	// Ensure local dir exists
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create local dir: %w", err)
	}

	// Clean old trash entries
	if err := CleanTrash(localDir, defaultMaxAge); err != nil {
		slog.Warn("Failed to clean trash", "error", err)
	}

	// Load state
	state, err := LoadState(localDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Resolve remote folder
	folderID := state.RemoteFolderID
	if folderID == "" {
		folderID, err = e.client.ResolveOrCreatePath(e.config.RemotePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve remote path '%s': %w", e.config.RemotePath, err)
		}
		state.RemoteFolderID = folderID
	}

	// Scan both sides
	fmt.Println("Scanning local files...")
	localFiles, err := ScanLocal(localDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan local: %w", err)
	}

	fmt.Println("Scanning remote files...")
	remoteFiles, err := ScanRemote(e.client, folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to scan remote: %w", err)
	}

	// Classify actions
	actions := ClassifyActions(localFiles, remoteFiles, state)

	// Count actionable items
	actionable := 0
	for _, a := range actions {
		if a.Type != ActionUnchanged {
			actionable++
		}
	}

	if actionable == 0 {
		fmt.Println("Everything up to date.")
		return report, nil
	}

	// Print plan
	fmt.Printf("\n%d file(s) to sync:\n", actionable)
	for _, a := range actions {
		if a.Type == ActionUnchanged {
			continue
		}
		icon := actionIcon(a.Type)
		fmt.Printf("  %s %s\n", icon, a.RelPath)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("Dry run — no changes made.")
		return report, nil
	}

	// Execute actions
	total := actionable
	current := 0
	for _, a := range actions {
		if a.Type == ActionUnchanged {
			// Clean up state for files deleted on both sides
			if a.State != nil && a.LocalFile == nil && a.RemoteFile == nil {
				delete(state.Files, a.RelPath)
				DeleteBase(localDir, a.RelPath)
			}
			continue
		}

		current++
		fmt.Printf("[%d/%d] %s %s\n", current, total, actionIcon(a.Type), a.RelPath)

		err := e.executeAction(localDir, folderID, state, a, report)
		if err != nil {
			slog.Error("Failed to sync file", "path", a.RelPath, "error", err)
			report.Errors = append(report.Errors, fmt.Errorf("%s: %w", a.RelPath, err))
			continue
		}

		// Save state after each successful action
		if err := SaveState(localDir, state); err != nil {
			slog.Error("Failed to save state", "error", err)
		}
	}

	fmt.Printf("\nSync complete: %s\n", report)
	return report, nil
}

func (e *Engine) executeAction(localDir, folderID string, state *SyncState, action SyncAction, report *Report) error {
	switch action.Type {
	case ActionCreateRemote:
		return e.createRemote(localDir, folderID, state, action, report)
	case ActionCreateLocal:
		return e.createLocal(localDir, state, action, report)
	case ActionPush:
		return e.push(localDir, state, action, report)
	case ActionPull:
		return e.pull(localDir, state, action, report)
	case ActionConflict:
		return e.resolveConflict(localDir, folderID, state, action, report)
	case ActionDeleteLocal:
		return e.deleteLocal(localDir, state, action, report)
	case ActionDeleteRemote:
		return e.deleteRemote(localDir, state, action, report)
	default:
		return nil
	}
}

func (e *Engine) createRemote(localDir, folderID string, state *SyncState, action SyncAction, report *Report) error {
	content, err := os.ReadFile(action.LocalFile.AbsPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Determine the target folder for nested files
	targetFolderID := folderID
	dir := filepath.Dir(action.RelPath)
	if dir != "." && dir != "" {
		targetFolderID, err = e.client.ResolveOrCreatePath(e.config.RemotePath + "/" + dir)
		if err != nil {
			return fmt.Errorf("failed to resolve subfolder: %w", err)
		}
	}

	info, frontmatter, err := convert.CreateDocFromMarkdown(e.client, targetFolderID, action.RelPath, content)
	if err != nil {
		return err
	}

	// Update state
	state.Files[action.RelPath] = FileState{
		DriveFileID: info.ID,
		LocalHash:   action.LocalFile.Hash,
		RemoteHash:  action.LocalFile.Hash, // Initially same as local
		LastSynced:  time.Now(),
		Frontmatter: frontmatter,
	}
	SaveBase(localDir, action.RelPath, content)
	report.Created++
	return nil
}

func (e *Engine) createLocal(localDir string, state *SyncState, action SyncAction, report *Report) error {
	content, err := convert.ExportDocAsMarkdown(e.client, action.RemoteFile.DriveID, nil)
	if err != nil {
		return err
	}

	absPath := filepath.Join(localDir, action.RelPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	hash := hashContent(content)
	state.Files[action.RelPath] = FileState{
		DriveFileID: action.RemoteFile.DriveID,
		LocalHash:   hash,
		RemoteHash:  action.RemoteFile.Hash,
		LastSynced:  time.Now(),
	}
	SaveBase(localDir, action.RelPath, content)
	report.Created++
	return nil
}

func (e *Engine) push(localDir string, state *SyncState, action SyncAction, report *Report) error {
	content, err := os.ReadFile(action.LocalFile.AbsPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	fileID := action.State.DriveFileID
	frontmatter, err := convert.UpdateDocFromMarkdown(e.client, fileID, action.RelPath, content)
	if err != nil {
		return err
	}

	// Re-export to get the actual remote hash after conversion
	exported, err := e.client.ExportMarkdown(fileID)
	if err != nil {
		slog.Warn("Failed to re-export after push", "error", err)
		exported = content // fallback
	}

	state.Files[action.RelPath] = FileState{
		DriveFileID: fileID,
		LocalHash:   action.LocalFile.Hash,
		RemoteHash:  hashContent(exported),
		LastSynced:  time.Now(),
		Frontmatter: frontmatter,
	}
	SaveBase(localDir, action.RelPath, content)
	report.Pushed++
	return nil
}

func (e *Engine) pull(localDir string, state *SyncState, action SyncAction, report *Report) error {
	// Re-attach frontmatter if we have it stored
	var frontmatter []byte
	if action.State != nil {
		frontmatter = action.State.Frontmatter
	}
	content, err := convert.ExportDocAsMarkdown(e.client, action.State.DriveFileID, frontmatter)
	if err != nil {
		return err
	}

	absPath := filepath.Join(localDir, action.RelPath)
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	hash := hashContent(content)
	state.Files[action.RelPath] = FileState{
		DriveFileID: action.State.DriveFileID,
		LocalHash:   hash,
		RemoteHash:  action.RemoteFile.Hash,
		LastSynced:  time.Now(),
	}
	SaveBase(localDir, action.RelPath, content)
	report.Pulled++
	return nil
}

func (e *Engine) resolveConflict(localDir, folderID string, state *SyncState, action SyncAction, report *Report) error {
	fmt.Printf("\n  CONFLICT: %s\n", action.RelPath)

	// Attempt three-way merge if we have a base copy and git is available
	if action.State != nil && gitAvailable() {
		baseContent, err := LoadBase(localDir, action.RelPath)
		if err == nil && baseContent != nil {
			localContent, err := os.ReadFile(action.LocalFile.AbsPath)
			if err != nil {
				return fmt.Errorf("failed to read local file: %w", err)
			}

			remoteContent, err := convert.ExportDocAsMarkdown(e.client, action.State.DriveFileID, nil)
			if err != nil {
				return fmt.Errorf("failed to export remote: %w", err)
			}

			merged, hasConflicts, err := ThreeWayMerge(baseContent, localContent, remoteContent)
			if err != nil {
				slog.Warn("Three-way merge failed, falling back to pick-one", "error", err)
			} else if !hasConflicts {
				fmt.Printf("  Auto-merged (non-overlapping changes)\n")
				// Write merged content locally
				absPath := filepath.Join(localDir, action.RelPath)
				if err := os.WriteFile(absPath, merged, 0644); err != nil {
					return fmt.Errorf("failed to write merged file: %w", err)
				}
				// Push merged content to remote
				if _, err := convert.UpdateDocFromMarkdown(e.client, action.State.DriveFileID, action.RelPath, merged); err != nil {
					return fmt.Errorf("failed to push merged content: %w", err)
				}
				hash := hashContent(merged)
				state.Files[action.RelPath] = FileState{
					DriveFileID: action.State.DriveFileID,
					LocalHash:   hash,
					RemoteHash:  hash,
					LastSynced:  time.Now(),
				}
				SaveBase(localDir, action.RelPath, merged)
				report.Pushed++
				return nil
			} else {
				fmt.Printf("  Three-way merge has conflicts.\n")
			}
		}
	}

	// Fall back to pick-one
	fmt.Printf("  Both local and remote have changed.\n")
	fmt.Printf("  [l]ocal / [r]emote / [s]kip? ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "l", "local":
		fmt.Printf("  → Keeping local version\n")
		if action.State != nil {
			return e.push(localDir, state, action, report)
		}
		return e.createRemote(localDir, folderID, state, action, report)
	case "r", "remote":
		fmt.Printf("  → Keeping remote version\n")
		if action.State != nil {
			return e.pull(localDir, state, action, report)
		}
		return e.createLocal(localDir, state, action, report)
	default:
		fmt.Printf("  → Skipping\n")
		report.Skipped++
		return nil
	}
}

func (e *Engine) deleteLocal(localDir string, state *SyncState, action SyncAction, report *Report) error {
	if e.noDelete {
		fmt.Printf("  (skipped — --no-delete)\n")
		report.Skipped++
		return nil
	}

	// Move to trash buffer instead of permanent delete
	if err := MoveToTrash(localDir, action.RelPath); err != nil {
		return fmt.Errorf("failed to trash local file: %w", err)
	}

	// Clean up state and base
	delete(state.Files, action.RelPath)
	DeleteBase(localDir, action.RelPath)
	report.Deleted++
	return nil
}

func (e *Engine) deleteRemote(localDir string, state *SyncState, action SyncAction, report *Report) error {
	if e.noDelete {
		fmt.Printf("  (skipped — --no-delete)\n")
		report.Skipped++
		return nil
	}

	// Move to Drive trash
	if err := e.client.DeleteFile(action.State.DriveFileID); err != nil {
		return fmt.Errorf("failed to trash remote file: %w", err)
	}

	// Clean up state and base
	delete(state.Files, action.RelPath)
	DeleteBase(localDir, action.RelPath)
	report.Deleted++
	return nil
}

// ExpandPath resolves ~ and returns an absolute path.
func ExpandPath(path string) (string, error) {
	return expandPath(path)
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}

func actionIcon(t ActionType) string {
	switch t {
	case ActionCreateRemote:
		return "[NEW->]"
	case ActionCreateLocal:
		return "[<-NEW]"
	case ActionPush:
		return "[PUSH>]"
	case ActionPull:
		return "[<PULL]"
	case ActionDeleteLocal:
		return "[DEL<-]"
	case ActionDeleteRemote:
		return "[->DEL]"
	case ActionConflict:
		return "[!!!!!]"
	default:
		return "[     ]"
	}
}
