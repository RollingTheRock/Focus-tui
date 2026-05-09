package git

import (
	"strings"
	"testing"

	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWorktreePaneNewKeyOpensCreateOverlay(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected create-worktree command")
	}
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenCreateWorktreeMsg)
	if !ok {
		t.Fatalf("expected OpenCreateWorktreeMsg, got %T", msg)
	}
	if openMsg.RepoPath != "/repo/main" {
		t.Fatalf("expected repo path /repo/main, got %q", openMsg.RepoPath)
	}
	if openMsg.BaseRef != "main" {
		t.Fatalf("expected base ref main, got %q", openMsg.BaseRef)
	}
}

func TestCreateWorktreePaneSubmitCreatesWorktree(t *testing.T) {
	adapter := &fakeGitAdapter{createWorktreeRes: &gitmodel.Worktree{Path: "/repo/focus-tui-feature-a", Branch: "feature-a"}}
	pane := NewWorktreeCreatePane("create-1", models.PaneMeta{ID: "create-1", Type: "worktree-create", CWD: "/repo/focus-tui"}, models.CommonModel{}, adapter, OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	pane.SetSize(80, 12)

	pane.inputs[createWorktreeFieldBranch].SetValue("feature-a")
	pane.inputs[createWorktreeFieldPath].SetValue("/repo/focus-tui-feature-a")
	updated, cmd := pane.submit()
	pane = updated.(*WorktreeCreatePane)
	if cmd == nil {
		t.Fatalf("expected submit command")
	}
	msg := runCmd(t, cmd)
	updated, cmd = pane.Update(msg)
	pane = updated.(*WorktreeCreatePane)
	if cmd == nil {
		t.Fatalf("expected created message command")
	}
	createdMsg, ok := runCmd(t, cmd).(WorktreeCreatedMsg)
	if !ok {
		t.Fatalf("expected WorktreeCreatedMsg")
	}
	if createdMsg.Worktree.Branch != "feature-a" {
		t.Fatalf("expected created branch feature-a, got %q", createdMsg.Worktree.Branch)
	}
	if adapter.createWorktreeReq == nil || adapter.createWorktreeReq.Branch != "feature-a" {
		t.Fatalf("expected adapter create request to capture branch")
	}
	if adapter.createWorktreeReq.BaseRef != "main" {
		t.Fatalf("expected base ref main, got %q", adapter.createWorktreeReq.BaseRef)
	}
	if adapter.createWorktreeReq.Path != "/repo/focus-tui-feature-a" {
		t.Fatalf("expected path to be passed through, got %q", adapter.createWorktreeReq.Path)
	}
}

func TestCreateWorktreePaneViewShowsHints(t *testing.T) {
	pane := NewWorktreeCreatePane("create-1", models.PaneMeta{ID: "create-1", Type: "worktree-create", CWD: "/repo/focus-tui"}, models.CommonModel{}, &fakeGitAdapter{}, OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	pane.SetSize(80, 12)
	view := pane.View()
	for _, want := range []string{"Create Worktree", "Task (optional):", "Branch:", "Base ref:", "Path:", "Ctrl+S create"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}
