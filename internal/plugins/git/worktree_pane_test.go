package git

import (
	"strings"
	"testing"

	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPluginCreatePaneReturnsWorktreePane(t *testing.T) {
	adapter := &fakeGitAdapter{}
	pl := New(adapter)

	panel, err := pl.CreatePane(models.PaneTypeWorktree, "worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo"}, models.CommonModel{})
	if err != nil {
		t.Fatalf("create pane: %v", err)
	}
	if _, ok := panel.(*WorktreePane); !ok {
		t.Fatalf("expected *WorktreePane, got %T", panel)
	}
}

func TestWorktreePaneInitLoadsAndRendersWorktrees(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a", DirtySummary: gitmodel.DirtySummary{Unstaged: 2}},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 10)

	cmd := pane.Init()
	if cmd == nil {
		t.Fatalf("expected init command")
	}
	updated, _ := pane.Update(runCmd(t, cmd))
	pane = updated.(*WorktreePane)

	view := pane.View()
	for _, want := range []string{"Worktrees", "main", "feature-a", "dirty 2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestWorktreePaneKeyboardHandling(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a"},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)
	if pane.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", pane.cursor)
	}

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected refresh command")
	}
}

func TestWorktreePaneOpenShellMessageUsesSelectedWorktree(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a"},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected open-shell command")
	}
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenWorktreeShellMsg)
	if !ok {
		t.Fatalf("expected OpenWorktreeShellMsg, got %T", msg)
	}
	if openMsg.Worktree.Path != "/repo/feature-a" {
		t.Fatalf("expected selected worktree path, got %q", openMsg.Worktree.Path)
	}
	if !strings.Contains(pane.notice, "feature-a") {
		t.Fatalf("expected notice to mention selected worktree, got %q", pane.notice)
	}
}
