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

	// Set up some fake files so cursor can move
	overlay.files = []fileEntry{
		{file: git.File{Path: "a.go"}, status: "unstaged"},
		{file: git.File{Path: "b.go"}, status: "unstaged"},
	}

	if overlay.fileCursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", overlay.fileCursor)
	}

	updated, _ := overlay.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'j'")
	}

	o2 := updated.(*GitFileTreeOverlay)
	if o2.fileCursor != 1 {
		t.Fatalf("expected cursor 1 after 'j', got %d", o2.fileCursor)
	}
}
