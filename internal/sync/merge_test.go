package sync

import (
	"strings"
	"testing"
)

func TestThreeWayMerge_CleanMerge(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	base := []byte("line 1\nline 2\nline 3\n")
	local := []byte("line 1 modified\nline 2\nline 3\n")
	remote := []byte("line 1\nline 2\nline 3 modified\n")

	merged, conflicts, err := ThreeWayMerge(base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if conflicts {
		t.Error("expected clean merge, got conflicts")
	}

	result := string(merged)
	if !strings.Contains(result, "line 1 modified") {
		t.Error("expected local change in merged output")
	}
	if !strings.Contains(result, "line 3 modified") {
		t.Error("expected remote change in merged output")
	}
}

func TestThreeWayMerge_Conflict(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	base := []byte("line 1\nline 2\nline 3\n")
	local := []byte("line 1 local\nline 2\nline 3\n")
	remote := []byte("line 1 remote\nline 2\nline 3\n")

	merged, conflicts, err := ThreeWayMerge(base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if !conflicts {
		t.Error("expected conflicts")
	}
	if !strings.Contains(string(merged), "<<<<<<<") {
		t.Error("expected conflict markers in output")
	}
}

func TestThreeWayMerge_OneSideOnly(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	base := []byte("original\n")
	local := []byte("modified\n")
	remote := []byte("original\n")

	merged, conflicts, err := ThreeWayMerge(base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if conflicts {
		t.Error("expected clean merge for single-side change")
	}
	if string(merged) != "modified\n" {
		t.Errorf("expected 'modified\\n', got %q", string(merged))
	}
}

func TestThreeWayMerge_EmptyBase(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	base := []byte("")
	local := []byte("local content\n")
	remote := []byte("remote content\n")

	_, conflicts, err := ThreeWayMerge(base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	// Both added content to empty base — should conflict
	if !conflicts {
		t.Error("expected conflict when both add to empty base")
	}
}
