package commands

import (
	"context"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

func TestCreateSessionHandoff_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreateSessionHandoff
		wantErr string
	}{
		{"missing id", CreateSessionHandoff{TaskID: "t", SessionID: "s"}, "handoff id required"},
		{"missing task", CreateSessionHandoff{ID: "1", SessionID: "s"}, "task id required"},
		{"missing session", CreateSessionHandoff{ID: "1", TaskID: "t"}, "session id required"},
		{"valid", CreateSessionHandoff{ID: "1", TaskID: "t", SessionID: "s"}, ""},
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

func TestCreateSessionHandoff_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed task and plan
	if err := s.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "r", Title: "T"}); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := s.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-1", Title: "P", Status: "draft"}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	planID := "plan-1"
	cmd := &CreateSessionHandoff{
		ID:               "handoff-1",
		TaskID:           "task-1",
		PlanID:           &planID,
		SessionID:        "sess-1",
		DoneSummary:      "step 1",
		RemainingSummary: "step 2",
		DecisionSummary:  "continue",
		Entrypoint:       "open plan",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	handoffs, err := s.ListSessionHandoffs("task-1")
	if err != nil {
		t.Fatalf("list handoffs: %v", err)
	}
	if len(handoffs) != 1 {
		t.Fatalf("expected 1 handoff, got %d", len(handoffs))
	}
	if handoffs[0].DoneSummary != "step 1" {
		t.Fatalf("expected done_summary %q, got %q", "step 1", handoffs[0].DoneSummary)
	}
}
