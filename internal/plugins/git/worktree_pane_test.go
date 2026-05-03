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
	for _, want := range []string{"Worktrees", "main", "feature-a", "*"} {
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


func TestWorktreePaneResumeMessageUsesSelectedWorktree(t *testing.T) {
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
		t.Fatalf("expected resume command")
	}
	msg := runCmd(t, cmd)
	resumeMsg, ok := msg.(ResumeWorktreeMsg)
	if !ok {
		t.Fatalf("expected ResumeWorktreeMsg, got %T", msg)
	}
	if resumeMsg.Worktree.Path != "/repo/feature-a" {
		t.Fatalf("expected selected worktree path, got %q", resumeMsg.Worktree.Path)
	}
	if !strings.Contains(pane.notice, "Resuming") {
		t.Fatalf("expected resume notice, got %q", pane.notice)
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

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
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
}

func TestWorktreePaneEditTaskMessageUsesSelectedWorktree(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a"},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	pane.SetResumeSummaries(map[string]gitmodel.WorktreeResumeSummary{
		"/repo/feature-a": {TaskID: "task-1", TaskTitle: "Fix resume pipeline", ResumeScore: 90},
	})

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected edit-task command")
	}
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenTaskEditMsg)
	if !ok {
		t.Fatalf("expected OpenTaskEditMsg, got %T", msg)
	}
	if openMsg.WorktreeID != "/repo/feature-a" || openMsg.TaskID != "task-1" || openMsg.RelationType != "primary" {
		t.Fatalf("unexpected task edit message %+v", openMsg)
	}
}






func TestWorktreePaneRemoveCleanWorktreeRequiresConfirmation(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}, {Path: "/repo/feature-a", Branch: "feature-a"}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected delete confirm command")
	}
	msg := runCmd(t, cmd)
	confirmMsg, ok := msg.(OpenWorktreeDeleteConfirmMsg)
	if !ok {
		t.Fatalf("expected OpenWorktreeDeleteConfirmMsg, got %T", msg)
	}
	if confirmMsg.Worktree.Path != "/repo/feature-a" || confirmMsg.Force {
		t.Fatalf("unexpected delete confirm %+v", confirmMsg)
	}
}

func TestWorktreePaneDeleteKeyUsesD(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}, {Path: "/repo/feature-a", Branch: "feature-a"}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected delete confirm command")
	}
	msg := runCmd(t, cmd)
	confirmMsg, ok := msg.(OpenWorktreeDeleteConfirmMsg)
	if !ok {
		t.Fatalf("expected OpenWorktreeDeleteConfirmMsg, got %T", msg)
	}
	if confirmMsg.Worktree.Path != "/repo/feature-a" || confirmMsg.Force {
		t.Fatalf("unexpected delete confirm %+v", confirmMsg)
	}
}

func TestWorktreePaneRendersResumeSummaryAndOrdersByScore(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a"},
			{Path: "/repo/feature-b", Branch: "feature-b"},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	pane.SetSize(320, 24)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	pane.SetResumeSummaries(map[string]gitmodel.WorktreeResumeSummary{
		"/repo/feature-a": {TaskTitle: "Fix resume pipeline", TaskGoal: "Make overview dense and useful", NextStep: "Wire overview summaries", ResumeScore: 90, ResumeReason: "active task", LastResumeHint: "Continue: Wire overview summaries", TaskState: "active", TaskPriority: "high", QueuedTaskTitle: "Follow-up cleanup", QueuedTaskCount: 2, LastAgentSummary: "opencode running"},
		"/repo/feature-b": {TaskTitle: "Later task", NextStep: "Leave for tomorrow", ResumeScore: 10},
	})
	pane.SetActivity("/repo/feature-a", gitmodel.WorktreeActivity{OpenEditors: 2, HasShell: true, AgentCount: 1, LastActive: "2m ago"})
	pane.SetActivity("/repo/feature-b", gitmodel.WorktreeActivity{LastActive: "10m ago"})

	view := pane.View()
	for _, want := range []string{
		"Fix resume pipeline",
		"feature-a",
		"agent:1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}

	featureA := strings.Index(view, "Fix resume pipeline")
	featureB := strings.Index(view, "Later task")
	if featureA == -1 || featureB == -1 || featureA > featureB {
		t.Fatalf("expected higher resume score task to render first, got:\n%s", view)
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
	if pane.err == nil || !strings.Contains(pane.err.Error(), "Shift+X") {
		t.Fatalf("expected dirty warning, got %v", pane.err)
	}

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected force delete confirm command")
	}
	msg := runCmd(t, cmd)
	confirmMsg, ok := msg.(OpenWorktreeDeleteConfirmMsg)
	if !ok {
		t.Fatalf("expected OpenWorktreeDeleteConfirmMsg, got %T", msg)
	}
	if confirmMsg.Worktree.Path != "/repo/feature-a" || !confirmMsg.Force {
		t.Fatalf("unexpected force delete confirm %+v", confirmMsg)
	}
}

