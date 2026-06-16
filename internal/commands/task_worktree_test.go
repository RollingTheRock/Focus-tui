package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

func TestLinkTaskToWorktree_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     LinkTaskToWorktree
		wantErr string
	}{
		{"missing id", LinkTaskToWorktree{TaskID: "t", WorktreeID: "w"}, "link id required"},
		{"missing task", LinkTaskToWorktree{ID: "1", WorktreeID: "w"}, "task id required"},
		{"missing worktree", LinkTaskToWorktree{ID: "1", TaskID: "t"}, "worktree id required"},
		{"valid", LinkTaskToWorktree{ID: "1", TaskID: "t", WorktreeID: "w"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLinkTaskToWorktree_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed task and worktree
	if err := s.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "r", Title: "T"}); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := s.SaveWorktreeContext(models.WorktreeContextRecord{WorktreeID: "/repo/wt1", RepoID: "r"}); err != nil {
		t.Fatalf("seed worktree: %v", err)
	}

	cmd := &LinkTaskToWorktree{
		ID:           "link-1",
		TaskID:       "task-1",
		WorktreeID:   "/repo/wt1",
		RelationType: "primary",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	links, err := s.ListTaskWorktreeLinks("task-1")
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].RelationType != "primary" {
		t.Fatalf("expected relation %q, got %q", "primary", links[0].RelationType)
	}
}

func TestDeletePlanSteps_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     DeletePlanSteps
		wantErr string
	}{
		{"missing plan id", DeletePlanSteps{}, "plan id required"},
		{"valid", DeletePlanSteps{PlanID: "p"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDeletePlanSteps_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed plan and steps
	if err := s.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-1", Title: "P", Status: "draft"}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := s.SavePlanStep(models.PlanStepRecord{ID: fmt.Sprintf("s%d", i), PlanID: "plan-1", OrderIndex: i, Title: "S", State: "pending"}); err != nil {
			t.Fatalf("seed step: %v", err)
		}
	}

	cmd := &DeletePlanSteps{PlanID: "plan-1"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	steps, err := s.ListPlanSteps("plan-1")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(steps))
	}
}
