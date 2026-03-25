package sync

// ActionType represents what should happen to a file during sync.
type ActionType int

const (
	ActionUnchanged ActionType = iota
	ActionPush                 // Local changed, push to remote
	ActionPull                 // Remote changed, pull to local
	ActionCreateRemote         // New local file, create on remote
	ActionCreateLocal          // New remote file, create locally
	ActionDeleteLocal          // Deleted on remote, delete locally
	ActionDeleteRemote         // Deleted locally, delete on remote
	ActionConflict             // Both sides changed
)

func (a ActionType) String() string {
	switch a {
	case ActionUnchanged:
		return "unchanged"
	case ActionPush:
		return "push"
	case ActionPull:
		return "pull"
	case ActionCreateRemote:
		return "create-remote"
	case ActionCreateLocal:
		return "create-local"
	case ActionDeleteLocal:
		return "delete-local"
	case ActionDeleteRemote:
		return "delete-remote"
	case ActionConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// SyncAction describes what should happen to a single file.
type SyncAction struct {
	Type       ActionType
	RelPath    string
	LocalFile  *LocalFile
	RemoteFile *RemoteFile
	State      *FileState
}

// ClassifyActions determines the sync action for each file based on
// local files, remote files, and stored state.
func ClassifyActions(local []LocalFile, remote []RemoteFile, state *SyncState) []SyncAction {
	localMap := make(map[string]*LocalFile, len(local))
	for i := range local {
		localMap[local[i].RelPath] = &local[i]
	}

	remoteMap := make(map[string]*RemoteFile, len(remote))
	for i := range remote {
		remoteMap[remote[i].RelPath] = &remote[i]
	}

	// Collect all known paths
	allPaths := make(map[string]bool)
	for _, f := range local {
		allPaths[f.RelPath] = true
	}
	for _, f := range remote {
		allPaths[f.RelPath] = true
	}
	for path := range state.Files {
		allPaths[path] = true
	}

	var actions []SyncAction
	for path := range allPaths {
		lf := localMap[path]
		rf := remoteMap[path]
		fs, inState := state.Files[path]

		action := classifyFile(path, lf, rf, inState, fs)
		actions = append(actions, action)
	}

	// Sort: creates first, then pushes/pulls, then deletes, then conflicts, then unchanged
	sortActions(actions)
	return actions
}

func classifyFile(path string, lf *LocalFile, rf *RemoteFile, inState bool, fs FileState) SyncAction {
	hasLocal := lf != nil
	hasRemote := rf != nil

	action := SyncAction{
		RelPath:    path,
		LocalFile:  lf,
		RemoteFile: rf,
	}
	if inState {
		action.State = &fs
	}

	switch {
	// New local file, not in state, not on remote
	case hasLocal && !hasRemote && !inState:
		action.Type = ActionCreateRemote

	// New remote file, not in state, not local
	case !hasLocal && hasRemote && !inState:
		action.Type = ActionCreateLocal

	// Both exist but no state — first sync for this file, treat as conflict
	case hasLocal && hasRemote && !inState:
		action.Type = ActionConflict

	// In state but gone from both sides — cleanup
	case !hasLocal && !hasRemote && inState:
		action.Type = ActionUnchanged // Both deleted, just clean up state

	// Deleted remotely, still local and in state
	case hasLocal && !hasRemote && inState:
		action.Type = ActionDeleteLocal

	// Deleted locally, still remote and in state
	case !hasLocal && hasRemote && inState:
		action.Type = ActionDeleteRemote

	// Both exist and in state — compare hashes
	case hasLocal && hasRemote && inState:
		localChanged := lf.Hash != fs.LocalHash
		remoteChanged := rf.Hash != fs.RemoteHash

		switch {
		case !localChanged && !remoteChanged:
			action.Type = ActionUnchanged
		case localChanged && !remoteChanged:
			action.Type = ActionPush
		case !localChanged && remoteChanged:
			action.Type = ActionPull
		case localChanged && remoteChanged:
			action.Type = ActionConflict
		}

	default:
		action.Type = ActionUnchanged
	}

	return action
}

func sortActions(actions []SyncAction) {
	priority := func(a ActionType) int {
		switch a {
		case ActionCreateRemote, ActionCreateLocal:
			return 0
		case ActionPush, ActionPull:
			return 1
		case ActionDeleteLocal, ActionDeleteRemote:
			return 2
		case ActionConflict:
			return 3
		case ActionUnchanged:
			return 4
		default:
			return 5
		}
	}

	for i := 0; i < len(actions); i++ {
		for j := i + 1; j < len(actions); j++ {
			pi, pj := priority(actions[i].Type), priority(actions[j].Type)
			if pi > pj || (pi == pj && actions[i].RelPath > actions[j].RelPath) {
				actions[i], actions[j] = actions[j], actions[i]
			}
		}
	}
}
