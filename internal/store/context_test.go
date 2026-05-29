package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskContextRoundTrip(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskContext(TaskContextRecord{
		ID:                  "task-1",
		RepoID:              "/repo/main",
		Title:               "Stabilize phase 4 schema",
		Goal:                "Land schema and store contracts",
		NextStep:            "Add worktree context table",
		State:               "active",
		Priority:            "high",
		PreferredWorktreeID: "/repo/feature-a",
	}); err != nil {
		t.Fatalf("save task context: %v", err)
	}

	record, err := s.GetTaskContext("task-1")
	if err != nil {
		t.Fatalf("get task context: %v", err)
	}
	if record == nil || record.Title != "Stabilize phase 4 schema" || record.PreferredWorktreeID != "/repo/feature-a" {
		t.Fatalf("unexpected task context: %+v", record)
	}
}

func TestTaskBriefRoundTrip(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskContext(TaskContextRecord{
		ID:       "task-1",
		RepoID:   "/repo/main",
		Title:    "Stabilize phase 4 schema",
		State:    "active",
		Priority: "high",
	}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if err := s.SaveTaskBrief(TaskBriefRecord{
		TaskID:          "task-1",
		WhyNow:          "The current overview has no planning entry point.",
		SuccessCriteria: "The overview shows enough context to start from a brief.",
		OutOfScope:      "Full task graph editing.",
		KnownRisks:      "Might overfit the UI before plan convergence exists.",
	}); err != nil {
		t.Fatalf("save task brief: %v", err)
	}

	record, err := s.GetTaskBrief("task-1")
	if err != nil {
		t.Fatalf("get task brief: %v", err)
	}
	if record == nil || record.WhyNow == "" || record.SuccessCriteria == "" || record.OutOfScope == "" || record.KnownRisks == "" {
		t.Fatalf("unexpected task brief: %+v", record)
	}
}

func TestWorktreeContextRoundTrip(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	now := time.Now().UTC().Round(time.Second)
	if err := s.SaveTaskContext(TaskContextRecord{
		ID:       "task-1",
		RepoID:   "/repo/main",
		Title:    "Primary task",
		State:    "active",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if err := s.SaveTaskPlan(TaskPlanRecord{ID: "plan-1", Title: "Planning", Status: "draft"}); err != nil {
		t.Fatalf("save task plan: %v", err)
	}
	if err := s.SaveWorktreeContext(WorktreeContextRecord{
		WorktreeID:     "/repo/feature-a",
		RepoID:         "/repo/main",
		PrimaryTaskID:  stringPtr("task-1"),
		CurrentPlanID:  stringPtr("plan-1"),
		TaskMode:       "single",
		TaskName:       "Feature A",
		BranchSnapshot: "feature-a",
		LastActiveAt:   now,
		LastOpenedAt:   &now,
	}); err != nil {
		t.Fatalf("save worktree context: %v", err)
	}

	record, err := s.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if record == nil || record.TaskName != "Feature A" || record.PrimaryTaskID == nil || *record.PrimaryTaskID != "task-1" || record.CurrentPlanID == nil || *record.CurrentPlanID != "plan-1" {
		t.Fatalf("unexpected worktree context: %+v", record)
	}
}

func TestTaskWorktreeLinksAndNotes(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskContext(TaskContextRecord{
		ID:       "task-1",
		RepoID:   "/repo/main",
		Title:    "Primary task",
		State:    "active",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if err := s.SaveTaskWorktreeLink(TaskWorktreeLinkRecord{
		ID:           "link-1",
		TaskID:       "task-1",
		WorktreeID:   "/repo/feature-a",
		RelationType: "primary",
	}); err != nil {
		t.Fatalf("save task worktree link: %v", err)
	}
	if err := s.SaveContextNote(ContextNoteRecord{
		ID:         "note-1",
		TaskID:     stringPtr("task-1"),
		WorktreeID: "/repo/feature-a",
		NoteType:   "next_step",
		Body:       "Implement overview summary pipeline",
		Pinned:     true,
	}); err != nil {
		t.Fatalf("save context note: %v", err)
	}

	links, err := s.ListTaskWorktreeLinks("task-1")
	if err != nil {
		t.Fatalf("list task worktree links: %v", err)
	}
	if len(links) != 1 || links[0].RelationType != "primary" {
		t.Fatalf("unexpected task worktree links: %+v", links)
	}

	notes, err := s.ListContextNotes("task-1", "/repo/feature-a")
	if err != nil {
		t.Fatalf("list context notes: %v", err)
	}
	if len(notes) != 1 || !notes[0].Pinned || notes[0].Body != "Implement overview summary pipeline" {
		t.Fatalf("unexpected context notes: %+v", notes)
	}
}

func TestTaskPlansAndSessionHandoffsRoundTrip(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()
	if err := s.SaveTaskContext(TaskContextRecord{ID: "task-1", RepoID: "/repo/main", Title: "Primary task", State: "active", Priority: "medium"}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if err := s.SaveTaskPlan(TaskPlanRecord{ID: "plan-1", TaskID: "task-1", Title: "Phase 1 rollout", Status: "active", CurrentStep: "Wire overview summary", PlanBody: "1. add schema\n2. wire panes"}); err != nil {
		t.Fatalf("save task plan: %v", err)
	}
	if err := s.SavePlanStep(PlanStepRecord{ID: "step-1", PlanID: "plan-1", OrderIndex: 0, Title: "Wire overview summary", State: "in_progress", ExpandedTaskID: "task-1", Notes: "Do this before pane cleanup"}); err != nil {
		t.Fatalf("save plan step: %v", err)
	}
	if err := s.SaveSessionHandoff(SessionHandoffRecord{ID: "handoff-1", TaskID: "task-1", PlanID: stringPtr("plan-1"), SessionID: "session-1", DoneSummary: "Added schema", RemainingSummary: "Wire pane rendering", DecisionSummary: "Keep builder centralized", BlockerSummary: "Need UX pass", Entrypoint: "Open overview detail pane"}); err != nil {
		t.Fatalf("save session handoff: %v", err)
	}
	plans, err := s.ListTaskPlans("task-1")
	if err != nil || len(plans) != 1 || plans[0].CurrentStep != "Wire overview summary" {
		t.Fatalf("unexpected task plans: %+v err=%v", plans, err)
	}
	if plans[0].WhyNow != "" {
		t.Fatalf("expected empty optional plan brief by default, got %+v", plans[0])
	}
	steps, err := s.ListPlanSteps("plan-1")
	if err != nil || len(steps) != 1 || steps[0].State != "in_progress" || steps[0].ExpandedTaskID != "task-1" {
		t.Fatalf("unexpected plan steps: %+v err=%v", steps, err)
	}
	handoffs, err := s.ListSessionHandoffs("task-1")
	if err != nil || len(handoffs) != 1 || handoffs[0].Entrypoint != "Open overview detail pane" {
		t.Fatalf("unexpected session handoffs: %+v err=%v", handoffs, err)
	}
}

func TestTaskPlanCanExistWithoutTask(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskPlan(TaskPlanRecord{ID: "plan-orphan", Title: "Plan first", Status: "draft", PlanBody: "Brief\nDecompose\nConverge"}); err != nil {
		t.Fatalf("save standalone plan: %v", err)
	}
	plan, err := s.GetTaskPlan("plan-orphan")
	if err != nil {
		t.Fatalf("get standalone plan: %v", err)
	}
	if plan == nil || plan.TaskID != "" || plan.Title != "Plan first" {
		t.Fatalf("unexpected standalone plan: %+v", plan)
	}
	if err := s.SavePlanStep(PlanStepRecord{ID: "plan-orphan::step::000", PlanID: "plan-orphan", OrderIndex: 0, Title: "Intent brief", State: "in_progress", ExpandedTaskID: ""}); err != nil {
		t.Fatalf("save standalone plan step: %v", err)
	}
	steps, err := s.ListPlanSteps("plan-orphan")
	if err != nil || len(steps) != 1 || steps[0].Title != "Intent brief" {
		t.Fatalf("unexpected standalone plan steps: %+v err=%v", steps, err)
	}
	plans, err := s.ListTaskPlans("")
	if err != nil {
		t.Fatalf("list all plans: %v", err)
	}
	if len(plans) != 1 || plans[0].ID != "plan-orphan" {
		t.Fatalf("unexpected standalone plan list: %+v", plans)
	}
}

func TestMigrateAddsCurrentPlanIDToExistingWorktreeContexts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "focus.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	legacySchema := `
CREATE TABLE worktree_contexts (
    worktree_id      TEXT PRIMARY KEY,
    repo_id          TEXT NOT NULL,
    primary_task_id  TEXT,
    task_mode        TEXT NOT NULL DEFAULT 'single',
    task_name        TEXT,
    branch_snapshot  TEXT,
    last_active_at   DATETIME NOT NULL,
    last_opened_at   DATETIME,
    last_agent_at    DATETIME,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_worktree_contexts_repo_last_active ON worktree_contexts(repo_id, last_active_at DESC);
CREATE INDEX idx_worktree_contexts_primary_task ON worktree_contexts(primary_task_id);
INSERT INTO worktree_contexts (worktree_id, repo_id, task_mode, task_name, last_active_at)
VALUES ('/repo/feature-a', '/repo/main', 'single', 'Planning', CURRENT_TIMESTAMP);
`
	if _, err := raw.Exec(legacySchema); err != nil {
		raw.Close()
		t.Fatalf("seed legacy schema: %v", err)
	}
	raw.Close()

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskPlan(TaskPlanRecord{ID: "plan-1", Title: "Plan", Status: "draft"}); err != nil {
		t.Fatalf("save standalone plan: %v", err)
	}
	wc, err := s.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if wc == nil {
		t.Fatalf("expected migrated worktree context")
	}
	wc.CurrentPlanID = stringPtr("plan-1")
	if err := s.SaveWorktreeContext(*wc); err != nil {
		t.Fatalf("save migrated worktree context with current plan: %v", err)
	}
	reloaded, err := s.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("reload worktree context: %v", err)
	}
	if reloaded == nil || reloaded.CurrentPlanID == nil || *reloaded.CurrentPlanID != "plan-1" {
		t.Fatalf("expected current_plan_id after migration, got %+v", reloaded)
	}
}

func TestDeleteTaskContext(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	// Create a Phase with two Steps
	parentID := "phase-1"
	if err := s.SaveTaskContext(TaskContextRecord{ID: parentID, RepoID: "/repo/main", Title: "Phase", State: "active", Priority: "medium"}); err != nil {
		t.Fatalf("save phase: %v", err)
	}
	if err := s.SaveTaskContext(TaskContextRecord{ID: "step-1", RepoID: "/repo/main", Title: "Step 1", State: "blocked", Priority: "medium", ParentTaskID: &parentID}); err != nil {
		t.Fatalf("save step 1: %v", err)
	}
	if err := s.SaveTaskContext(TaskContextRecord{ID: "step-2", RepoID: "/repo/main", Title: "Step 2", State: "blocked", Priority: "medium", ParentTaskID: &parentID}); err != nil {
		t.Fatalf("save step 2: %v", err)
	}
	// Add a dependency between steps
	if err := s.SaveTaskDependency(TaskDependencyRecord{FromTaskID: "step-1", ToTaskID: "step-2", DependencyType: "hard"}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}

	// Delete the Phase — should cascade to Steps
	if err := s.DeleteTaskContext(parentID); err != nil {
		t.Fatalf("delete phase: %v", err)
	}

	// Verify Phase is gone
	if _, err := s.GetTaskContext(parentID); err != nil {
		t.Fatalf("get phase after delete: %v", err)
	}
	// Verify Steps are gone
	for _, id := range []string{"step-1", "step-2"} {
		record, err := s.GetTaskContext(id)
		if err != nil {
			t.Fatalf("get step after delete: %v", err)
		}
		if record != nil {
			t.Fatalf("expected step %s to be deleted, got %+v", id, record)
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
