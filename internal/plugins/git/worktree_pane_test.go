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
	for _, want := range []string{"Worktrees", "main", "feature-a", "~2"} {
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

func TestWorktreePaneTabFilteringAndCycling(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a", DirtySummary: gitmodel.DirtySummary{Unstaged: 1}},
			{Path: "/repo/feature-b", Branch: "feature-b"},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	pane.SetSize(220, 24)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	pane.SetResumeSummaries(map[string]gitmodel.WorktreeResumeSummary{
		"/repo/feature-a": {TaskTitle: "Active task", TaskState: "active", ResumeScore: 80},
		"/repo/feature-b": {TaskTitle: "Queued task", TaskState: "paused", ResumeScore: 30, QueuedTaskTitle: "Follow-up cleanup", QueuedTaskCount: 1},
	})

	view := pane.View()
	for _, want := range []string{"1 ALL(3)", "2 ACTIVE(1)", "3 FOCUS(1)", "4 QUEUED(1)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected tabs to contain %q, got:\n%s", want, view)
		}
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	pane = updated.(*WorktreePane)
	view = pane.View()
	if !strings.Contains(view, "Active task") || strings.Contains(view, "Queued task") {
		t.Fatalf("expected ACTIVE tab to filter rows, got:\n%s", view)
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	pane = updated.(*WorktreePane)
	view = pane.View()
	if !strings.Contains(view, "4 QUEUED(1)") || !strings.Contains(view, "Queued task") {
		t.Fatalf("expected cycling tabs to reach queued view, got:\n%s", view)
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

func TestWorktreePaneFollowUpTaskMessageUsesSelectedWorktree(t *testing.T) {
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

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected follow-up command")
	}
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenTaskEditMsg)
	if !ok {
		t.Fatalf("expected OpenTaskEditMsg, got %T", msg)
	}
	if openMsg.WorktreeID != "/repo/feature-a" || openMsg.RelationType != "queued" || openMsg.ParentTaskID != "task-1" {
		t.Fatalf("unexpected follow-up task edit message %+v", openMsg)
	}
}

func TestWorktreePanePlanDraftMessageUsesSelectedWorktree(t *testing.T) {
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

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected plan-draft command")
	}
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenPlanEditMsg)
	if !ok {
		t.Fatalf("expected OpenPlanEditMsg, got %T", msg)
	}
	if openMsg.WorktreeID != "/repo/feature-a" || openMsg.TaskID != "task-1" {
		t.Fatalf("unexpected plan edit message %+v", openMsg)
	}
}

func TestWorktreePanePlanDraftAllowsMissingTask(t *testing.T) {
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
		"/repo/feature-a": {TaskTitle: "No task yet", ResumeScore: 40},
	})

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected standalone plan-draft command")
	}
	if pane.err != nil {
		t.Fatalf("expected no error for standalone plan draft, got %v", pane.err)
	}
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenPlanEditMsg)
	if !ok {
		t.Fatalf("expected OpenPlanEditMsg, got %T", msg)
	}
	if openMsg.WorktreeID != "/repo/feature-a" || openMsg.TaskID != "" {
		t.Fatalf("unexpected standalone plan edit message %+v", openMsg)
	}
}

func TestWorktreePaneUppercasePStartsPruneConfirmation(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}, {Path: "/repo/feature-a", Branch: "feature-a"}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	pane = updated.(*WorktreePane)
	if cmd != nil {
		t.Fatalf("expected prune confirmation without immediate command")
	}
	if pane.confirm == nil || pane.confirm.kind != "prune" {
		t.Fatalf("expected uppercase P to start prune confirmation, got %+v", pane.confirm)
	}
}

func TestWorktreePaneCycleTaskStateMessageUsesSelectedWorktree(t *testing.T) {
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
		"/repo/feature-a": {TaskID: "task-1", TaskTitle: "Fix resume pipeline", TaskState: "active", ResumeScore: 90},
	})

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected cycle-state command")
	}
	msg := runCmd(t, cmd)
	cycleMsg, ok := msg.(CycleTaskStateMsg)
	if !ok {
		t.Fatalf("expected CycleTaskStateMsg, got %T", msg)
	}
	if cycleMsg.TaskID != "task-1" || cycleMsg.WorktreeID != "/repo/feature-a" || cycleMsg.CurrentState != "active" {
		t.Fatalf("unexpected cycle task state message %+v", cycleMsg)
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

func TestWorktreePaneDeleteKeyUsesD(t *testing.T) {
	adapter := &fakeGitAdapter{worktrees: []gitmodel.Worktree{{Path: "/repo/main", Branch: "main", IsMain: true}, {Path: "/repo/feature-a", Branch: "feature-a"}}}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	pane = updated.(*WorktreePane)
	if pane.confirm == nil || pane.confirm.kind != "remove" || pane.confirm.force {
		t.Fatalf("expected non-force remove confirmation, got %+v", pane.confirm)
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
		"active · high · agent 1 · queued 2",
		"next: Wire overview summaries",
		"Selected: feature-a",
		"Goal: Make overview dense and useful",
		"Next: Continue: Wire overview summaries",
		"Why now: active task",
		"Agent: opencode running",
		"Runtime: shell=true edits=2 agents=1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"shell · edits 2 · agents 1  active", "queued: Follow-up cleanup (+1)  active task"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("expected queue row to avoid noisy duplicate context %q, got:\n%s", notWant, view)
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

	updated, _ := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
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
