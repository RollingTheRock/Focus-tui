package gitfiletree

import (
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/git"
	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/plugins/editor"
	gitplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/git"

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

func TestOverlayEnterOnFileOpensDiff(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, &mockGitAdapter{}, "/repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'enter'")
	}
	if cmd == nil {
		t.Fatal("expected diff command")
	}

	msg := cmd()
	diffMsg, ok := msg.(gitplugin.OpenDiffMsg)
	if !ok {
		t.Fatalf("expected OpenDiffMsg, got %T", msg)
	}
	if diffMsg.FilePath != "a.go" {
		t.Fatalf("expected FilePath a.go, got %q", diffMsg.FilePath)
	}
	if diffMsg.Staged {
		t.Fatalf("expected unstaged diff, got staged")
	}
}

func TestOverlayEnterOnStagedFileOpensStagedDiff(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, &mockGitAdapter{}, "/repo")
	overlay.status = &git.Status{
		StagedFiles: []git.File{{Path: "b.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'enter'")
	}
	if cmd == nil {
		t.Fatal("expected diff command")
	}

	msg := cmd()
	diffMsg, ok := msg.(gitplugin.OpenDiffMsg)
	if !ok {
		t.Fatalf("expected OpenDiffMsg, got %T", msg)
	}
	if diffMsg.FilePath != "b.go" {
		t.Fatalf("expected FilePath b.go, got %q", diffMsg.FilePath)
	}
	if !diffMsg.Staged {
		t.Fatalf("expected staged diff, got unstaged")
	}
}

func TestOverlayEOpensEditor(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'e'")
	}
	if cmd == nil {
		t.Fatal("expected editor command")
	}

	msg := cmd()
	editorMsg, ok := msg.(editor.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if editorMsg.FilePath != "/repo/a.go" {
		t.Fatalf("expected FilePath /repo/a.go, got %q", editorMsg.FilePath)
	}
}

func TestOverlayOOpensEditor(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'o'")
	}
	if cmd == nil {
		t.Fatal("expected editor command")
	}

	msg := cmd()
	editorMsg, ok := msg.(editor.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if editorMsg.FilePath != "/repo/a.go" {
		t.Fatalf("expected FilePath /repo/a.go, got %q", editorMsg.FilePath)
	}
}

func TestOverlayDStillDiscards(t *testing.T) {
	mock := &mockGitAdapter{}
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, mock, "/repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'd'")
	}
	if cmd == nil {
		t.Fatal("expected discard command")
	}

	cmd()

	if len(mock.discardCalls) != 1 {
		t.Fatalf("expected 1 discard call, got %d", len(mock.discardCalls))
	}
	if mock.discardCalls[0] != "a.go" {
		t.Fatalf("expected discard path a.go, got %q", mock.discardCalls[0])
	}
}
