package git

import (
	"path/filepath"
	"strings"
	"testing"

	gitmodel "github.com/RollingTheRock/Focus-tui/internal/git"
	"github.com/RollingTheRock/Focus-tui/internal/models"

	tea "charm.land/bubbletea/v2"
)

func TestWorktreePaneNewKeyOpensCreateOverlay(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
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

	pane.formValues.branch = "feature-a"
	pane.formValues.path = "/repo/focus-tui-feature-a"
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

func TestCreateWorktreePaneUpdatesDefaultPathWhenBranchChanges(t *testing.T) {
	adapter := &fakeGitAdapter{createWorktreeRes: &gitmodel.Worktree{Path: "/repo/focus-tui/.worktrees/feature-a", Branch: "feature/a"}}
	pane := NewWorktreeCreatePane("create-1", models.PaneMeta{ID: "create-1", Type: "worktree-create", CWD: "/repo/focus-tui"}, models.CommonModel{}, adapter, OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	pane.formValues.branch = "feature/a"

	updated, cmd := pane.submit()
	pane = updated.(*WorktreeCreatePane)
	if cmd == nil {
		t.Fatalf("expected submit command")
	}
	wantPath := filepath.Join("/repo/focus-tui", ".worktrees", "feature-a")
	if adapter.createWorktreeReq == nil {
		_ = runCmd(t, cmd)
	}
	if adapter.createWorktreeReq == nil || adapter.createWorktreeReq.Path != wantPath {
		t.Fatalf("expected default path %q, got req=%+v", wantPath, adapter.createWorktreeReq)
	}
}

func TestCreateWorktreePaneAutoUpdatesDisplayedDefaultPath(t *testing.T) {
	pane := NewWorktreeCreatePane("create-1", models.PaneMeta{ID: "create-1", Type: "worktree-create", CWD: "/repo/focus-tui"}, models.CommonModel{}, &fakeGitAdapter{}, OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	pane.SetSize(120, 20)
	runCmd(t, pane.Init())

	pane.formValues.branch = "feature/a"
	pane.syncAutoPathWithBranch()

	wantPath := filepath.Join("/repo/focus-tui", ".worktrees", "feature-a")
	if pane.formValues.path != wantPath {
		t.Fatalf("expected displayed path %q, got %q", wantPath, pane.formValues.path)
	}
	if view := pane.View().Content; !strings.Contains(view, wantPath) {
		t.Fatalf("expected rendered form to contain %q, got:\n%s", wantPath, view)
	}

	pane.formValues.branch = "bug/fix"
	pane.syncAutoPathWithBranch()

	wantPath = filepath.Join("/repo/focus-tui", ".worktrees", "bug-fix")
	if pane.formValues.path != wantPath {
		t.Fatalf("expected displayed path %q after branch change, got %q", wantPath, pane.formValues.path)
	}
	if view := pane.View().Content; !strings.Contains(view, wantPath) {
		t.Fatalf("expected rendered form to contain %q after branch change, got:\n%s", wantPath, view)
	}
}

func TestCreateWorktreePaneDoesNotOverwriteCustomPath(t *testing.T) {
	pane := NewWorktreeCreatePane("create-1", models.PaneMeta{ID: "create-1", Type: "worktree-create", CWD: "/repo/focus-tui"}, models.CommonModel{}, &fakeGitAdapter{}, OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	pane.formValues.path = "/custom/worktree"

	pane.formValues.branch = "feature/a"
	pane.syncAutoPathWithBranch()

	if pane.formValues.path != "/custom/worktree" {
		t.Fatalf("expected custom path to remain unchanged, got %q", pane.formValues.path)
	}
}

func TestCreateWorktreePaneViewShowsHints(t *testing.T) {
	pane := NewWorktreeCreatePane("create-1", models.PaneMeta{ID: "create-1", Type: "worktree-create", CWD: "/repo/focus-tui"}, models.CommonModel{}, &fakeGitAdapter{}, OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	pane.SetSize(80, 12)
	runCmd(t, pane.Init())
	view := pane.View()
	for _, want := range []string{"Create Worktree", "Task (optional)", "Branch", "Base ref", "Path", "Ctrl+S create"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view.Content)
		}
	}
}
