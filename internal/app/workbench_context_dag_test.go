package app

import (
	"strings"
	"testing"
	"time"

	gitmodel "focus/internal/git"
	"focus/internal/models"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/store"
)

type dagTestContextSource struct {
	ordered          []gitplugin.WorktreeContextView
	selectedWorktree gitmodel.Worktree
	selectedSummary  gitmodel.WorktreeResumeSummary
	selectedActivity gitmodel.WorktreeActivity
}

func (s dagTestContextSource) OrderedContexts() []gitplugin.WorktreeContextView {
	return s.ordered
}

func (s dagTestContextSource) SelectedContext() (gitmodel.Worktree, gitmodel.WorktreeResumeSummary, gitmodel.WorktreeActivity, bool) {
	return s.selectedWorktree, s.selectedSummary, s.selectedActivity, true
}

func TestBuildWorkbenchOverviewContextIncludesDAGAndSharedContext(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	repoID := "/repo/main"

	tasks := []models.TaskContextRecord{
		{ID: "task-a", RepoID: repoID, Title: "Ingest spec", State: "done", Priority: "medium", NextStep: "handoff"},
		{ID: "task-b", RepoID: repoID, Title: "Implement runtime", State: "active", Priority: "high", NextStep: "wire dag"},
		{ID: "task-c", RepoID: repoID, Title: "GUI integration", State: "paused", Priority: "medium", NextStep: "wait upstream"},
	}
	for _, task := range tasks {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}
	for _, dep := range []models.TaskDependencyRecord{
		{FromTaskID: "task-a", ToTaskID: "task-b", DependencyType: "hard"},
		{FromTaskID: "task-b", ToTaskID: "task-c", DependencyType: "hard"},
	} {
		if err := st.SaveTaskDependency(dep); err != nil {
			t.Fatalf("save dependency %+v: %v", dep, err)
		}
	}
	if err := st.SaveTaskOutput(models.TaskOutputRecord{ID: "out-a", TaskID: "task-a", Content: "spec summary", Actor: "claude", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("save output out-a: %v", err)
	}
	if err := st.SaveTaskOutput(models.TaskOutputRecord{ID: "out-b", TaskID: "task-b", Content: "runtime wiring", Actor: "codex", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("save output out-b: %v", err)
	}
	if err := st.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-b", TaskID: "task-b", Title: "Runtime plan", Status: "active", CurrentStep: "Wire DAG"}); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := st.SaveKnowledgeFact(models.KnowledgeFactRecord{ID: "fact-b", PlanID: "plan-b", Subject: "scheduler", Predicate: "depends_on", Object: "task DAG", Source: "unit-test"}); err != nil {
		t.Fatalf("save fact: %v", err)
	}
	if err := st.SaveAgentMessage(models.AgentMessageRecord{ID: "msg-b", FromAgent: "agent-1", ToAgent: "agent-2", MsgType: "status_update", Payload: `{"task_id":"task-b","note":"handoff"}`}); err != nil {
		t.Fatalf("save message: %v", err)
	}

	source := dagTestContextSource{
		ordered: []gitplugin.WorktreeContextView{
			{Worktree: gitmodel.Worktree{Path: "/repo/feature-runtime", Branch: "feature/runtime"}, Summary: gitmodel.WorktreeResumeSummary{TaskID: "task-b", TaskTitle: "Implement runtime", TaskState: "active", TaskPriority: "high", ResumeReason: "active task", LastResumeHint: "Wire DAG", NextStep: "wire dag"}, Activity: gitmodel.WorktreeActivity{HasShell: true, AgentCount: 1}},
		},
		selectedWorktree: gitmodel.Worktree{Path: "/repo/feature-runtime", Branch: "feature/runtime"},
		selectedSummary:  gitmodel.WorktreeResumeSummary{TaskID: "task-b", TaskTitle: "Implement runtime", TaskState: "active", TaskPriority: "high", ResumeReason: "active task", LastResumeHint: "Wire DAG", NextStep: "wire dag"},
		selectedActivity: gitmodel.WorktreeActivity{HasShell: true, AgentCount: 1},
	}

	ctx := buildWorkbenchOverviewContext(source, st, repoID)
	if ctx.DAG.NodeCount != 3 || ctx.DAG.EdgeCount != 2 {
		t.Fatalf("unexpected dag counts: %+v", ctx.DAG)
	}
	if !strings.Contains(ctx.Detail.DAGSummary, "3 tasks") {
		t.Fatalf("expected dag summary with task count, got %q", ctx.Detail.DAGSummary)
	}
	if len(ctx.Detail.UpstreamTasks) == 0 || len(ctx.Detail.DownstreamTasks) == 0 {
		t.Fatalf("expected dependency lines, got upstream=%v downstream=%v", ctx.Detail.UpstreamTasks, ctx.Detail.DownstreamTasks)
	}
	if len(ctx.Detail.SharedContext) == 0 {
		t.Fatalf("expected shared context lines, got none")
	}
	joined := strings.Join(ctx.Detail.SharedContext, "\n")
	for _, want := range []string{"output/codex", "upstream/Ingest spec", "fact:", "msg/status_update"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected shared context to contain %q, got:\n%s", want, joined)
		}
	}
}
