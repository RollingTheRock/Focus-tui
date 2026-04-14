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

func TestWorktreePaneRemoveCleanWorktreeRequiresConfirmation(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}, {Path: "/repo/feature-a", Branch: "feature-a"}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	pane = updated.(*WorktreePane)
	if pane.confirm == nil || pane.confirm.kind != "remove" || pane.confirm.force {
		t.Fatalf("expected non-force remove confirmation, got %+v", pane.confirm)
	}

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected remove request command")
	}
	msg := runCmd(t, cmd)
	removeMsg, ok := msg.(RequestRemoveWorktreeMsg)
	if !ok {
		t.Fatalf("expected RequestRemoveWorktreeMsg, got %T", msg)
	}
	if removeMsg.Worktree.Path != "/repo/feature-a" || removeMsg.Force {
		t.Fatalf("unexpected remove request %+v", removeMsg)
	}
}

func TestWorktreePaneForceRemoveDirtyWorktree(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}, {Path: "/repo/feature-a", Branch: "feature-a", DirtySummary: gitmodel.DirtySummary{Unstaged: 1}}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	pane = updated.(*WorktreePane)
	if pane.confirm != nil {
		t.Fatalf("expected dirty worktree to require explicit force path first")
	}
	if pane.err == nil || !strings.Contains(pane.err.Error(), "Shift+X") {
		t.Fatalf("expected dirty warning, got %v", pane.err)
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	pane = updated.(*WorktreePane)
	if pane.confirm == nil || !pane.confirm.force {
		t.Fatalf("expected force confirmation, got %+v", pane.confirm)
	}
}

func TestWorktreePanePruneConfirmationEmitsRequest(t *testing.T) {
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, &fakeGitAdapter{})

	updated, _ := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	pane = updated.(*WorktreePane)
	if pane.confirm == nil || pane.confirm.kind != "prune" {
		t.Fatalf("expected prune confirmation, got %+v", pane.confirm)
	}

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected prune command")
	}
	msg := runCmd(t, cmd)
	pruneMsg, ok := msg.(RequestPruneWorktreesMsg)
	if !ok {
		t.Fatalf("expected RequestPruneWorktreesMsg, got %T", msg)
	}
	if pruneMsg.RepoPath != "/repo/main" {
		t.Fatalf("expected repo path /repo/main, got %q", pruneMsg.RepoPath)
	}
}
