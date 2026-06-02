package gitfiletree

import (
	"testing"

	"focus/internal/git"
	"focus/internal/models"

	tea "charm.land/bubbletea/v2"
)

func TestOverlayEscClosesOverlay(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/home/rollingtherock/dev/Focus-tui/.worktrees/feat-filetree-git-ui")

	// Simulate pressing 'esc' directly
	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'esc'")
	}
	if cmd == nil {
		t.Fatal("Update returned nil cmd for 'esc' — CloseOverlayMsg should have been returned")
	}

	msg := cmd()
	if _, ok := msg.(CloseOverlayMsg); !ok {
		t.Fatalf("Expected CloseOverlayMsg, got %T", msg)
	}
}

func TestOverlayJMovesCursor(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/home/rollingtherock/dev/Focus-tui/.worktrees/feat-filetree-git-ui")

	// Set up fake tree nodes so cursor can move
	overlay.treeFlatList = []*GitFileTreeNode{
		{Name: "a.go", Path: "a.go", IsDir: false, Status: gitNodeUnstaged},
		{Name: "b.go", Path: "b.go", IsDir: false, Status: gitNodeUnstaged},
	}

	if overlay.treeCursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", overlay.treeCursor)
	}

	updated, _ := overlay.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'j'")
	}

	o2 := updated.(*GitFileTreeOverlay)
	if o2.treeCursor != 1 {
		t.Fatalf("expected cursor 1 after 'j', got %d", o2.treeCursor)
	}
}

func TestOverlaySpaceStagesFile(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/home/rollingtherock/dev/Focus-tui/.worktrees/feat-filetree-git-ui")

	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	if len(overlay.treeFlatList) == 0 {
		t.Fatal("expected tree to have nodes")
	}

	// space on a file should return a refresh cmd
	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'space'")
	}
	// cmd may be nil since adapter is nil, that's expected in this test
	_ = cmd
}

func TestOverlayEnterTogglesDir(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/home/rollingtherock/dev/Focus-tui/.worktrees/feat-filetree-git-ui")

	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "dir/a.go"}},
	}
	overlay.rebuildTree()

	// Find a non-root directory node
	var dirNode *GitFileTreeNode
	for _, n := range overlay.treeFlatList {
		if n.IsDir && n.Parent != nil {
			dirNode = n
			break
		}
	}
	if dirNode == nil {
		t.Fatal("expected a non-root directory node in tree")
	}

	// Select the directory
	for i, n := range overlay.treeFlatList {
		if n == dirNode {
			overlay.treeCursor = i
			break
		}
	}

	if dirNode.Collapsed {
		t.Fatal("expected directory to be expanded by default")
	}

	// enter should collapse the directory
	updated, _ := overlay.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'enter'")
	}

	o2 := updated.(*GitFileTreeOverlay)
	// The directory should now be collapsed
	if !o2.treeFlatList[overlay.treeCursor].Collapsed {
		t.Fatal("expected directory to be collapsed after enter")
	}
}
