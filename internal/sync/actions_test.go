package sync

import (
	"testing"
	"time"
)

func TestClassifyActions_NewLocalFile(t *testing.T) {
	local := []LocalFile{{RelPath: "new.md", Hash: "sha256:abc"}}
	remote := []RemoteFile{}
	state := NewSyncState("folder")

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "new.md", ActionCreateRemote)
}

func TestClassifyActions_NewRemoteFile(t *testing.T) {
	local := []LocalFile{}
	remote := []RemoteFile{{RelPath: "new.md", DriveID: "d1", Hash: "sha256:abc"}}
	state := NewSyncState("folder")

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "new.md", ActionCreateLocal)
}

func TestClassifyActions_Unchanged(t *testing.T) {
	local := []LocalFile{{RelPath: "doc.md", Hash: "sha256:abc"}}
	remote := []RemoteFile{{RelPath: "doc.md", DriveID: "d1", Hash: "sha256:def"}}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{
		DriveFileID: "d1",
		LocalHash:   "sha256:abc",
		RemoteHash:  "sha256:def",
	}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionUnchanged)
}

func TestClassifyActions_Push(t *testing.T) {
	local := []LocalFile{{RelPath: "doc.md", Hash: "sha256:new-local"}}
	remote := []RemoteFile{{RelPath: "doc.md", DriveID: "d1", Hash: "sha256:old-remote"}}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{
		DriveFileID: "d1",
		LocalHash:   "sha256:old-local",
		RemoteHash:  "sha256:old-remote",
	}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionPush)
}

func TestClassifyActions_Pull(t *testing.T) {
	local := []LocalFile{{RelPath: "doc.md", Hash: "sha256:old-local"}}
	remote := []RemoteFile{{RelPath: "doc.md", DriveID: "d1", Hash: "sha256:new-remote"}}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{
		DriveFileID: "d1",
		LocalHash:   "sha256:old-local",
		RemoteHash:  "sha256:old-remote",
	}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionPull)
}

func TestClassifyActions_Conflict(t *testing.T) {
	local := []LocalFile{{RelPath: "doc.md", Hash: "sha256:new-local"}}
	remote := []RemoteFile{{RelPath: "doc.md", DriveID: "d1", Hash: "sha256:new-remote"}}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{
		DriveFileID: "d1",
		LocalHash:   "sha256:old-local",
		RemoteHash:  "sha256:old-remote",
	}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionConflict)
}

func TestClassifyActions_DeletedRemotely(t *testing.T) {
	local := []LocalFile{{RelPath: "doc.md", Hash: "sha256:abc"}}
	remote := []RemoteFile{}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{
		DriveFileID: "d1",
		LocalHash:   "sha256:abc",
		RemoteHash:  "sha256:def",
	}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionDeleteLocal)
}

func TestClassifyActions_DeletedLocally(t *testing.T) {
	local := []LocalFile{}
	remote := []RemoteFile{{RelPath: "doc.md", DriveID: "d1", Hash: "sha256:def"}}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{
		DriveFileID: "d1",
		LocalHash:   "sha256:abc",
		RemoteHash:  "sha256:def",
	}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionDeleteRemote)
}

func TestClassifyActions_BothExistNoState(t *testing.T) {
	local := []LocalFile{{RelPath: "doc.md", Hash: "sha256:abc"}}
	remote := []RemoteFile{{RelPath: "doc.md", DriveID: "d1", Hash: "sha256:def"}}
	state := NewSyncState("folder")

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionConflict)
}

func TestClassifyActions_BothDeletedCleanup(t *testing.T) {
	local := []LocalFile{}
	remote := []RemoteFile{}
	state := NewSyncState("folder")
	state.Files["doc.md"] = FileState{DriveFileID: "d1"}

	actions := ClassifyActions(local, remote, state)
	assertAction(t, actions, "doc.md", ActionUnchanged)
}

func TestClassifyActions_MultipleFiles(t *testing.T) {
	now := time.Now()
	local := []LocalFile{
		{RelPath: "new.md", Hash: "sha256:a"},
		{RelPath: "changed.md", Hash: "sha256:new"},
		{RelPath: "same.md", Hash: "sha256:same"},
	}
	remote := []RemoteFile{
		{RelPath: "remote-new.md", DriveID: "d2", Hash: "sha256:b"},
		{RelPath: "changed.md", DriveID: "d3", Hash: "sha256:old-r"},
		{RelPath: "same.md", DriveID: "d4", Hash: "sha256:same-r"},
	}
	state := NewSyncState("folder")
	state.Files["changed.md"] = FileState{DriveFileID: "d3", LocalHash: "sha256:old", RemoteHash: "sha256:old-r", LastSynced: now}
	state.Files["same.md"] = FileState{DriveFileID: "d4", LocalHash: "sha256:same", RemoteHash: "sha256:same-r", LastSynced: now}

	actions := ClassifyActions(local, remote, state)

	if len(actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(actions))
	}

	// Verify creates come before pushes, pushes before unchanged
	actionMap := make(map[string]ActionType)
	for _, a := range actions {
		actionMap[a.RelPath] = a.Type
	}

	if actionMap["new.md"] != ActionCreateRemote {
		t.Errorf("new.md: expected create-remote, got %s", actionMap["new.md"])
	}
	if actionMap["remote-new.md"] != ActionCreateLocal {
		t.Errorf("remote-new.md: expected create-local, got %s", actionMap["remote-new.md"])
	}
	if actionMap["changed.md"] != ActionPush {
		t.Errorf("changed.md: expected push, got %s", actionMap["changed.md"])
	}
	if actionMap["same.md"] != ActionUnchanged {
		t.Errorf("same.md: expected unchanged, got %s", actionMap["same.md"])
	}
}

func assertAction(t *testing.T, actions []SyncAction, path string, expected ActionType) {
	t.Helper()
	for _, a := range actions {
		if a.RelPath == path {
			if a.Type != expected {
				t.Errorf("%s: expected %s, got %s", path, expected, a.Type)
			}
			return
		}
	}
	t.Errorf("%s: not found in actions", path)
}
