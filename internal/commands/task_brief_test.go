package commands

import (
	"context"
	"testing"

	"focus/internal/models"
)

func TestUpdateTaskBrief_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     UpdateTaskBrief
		wantErr string
	}{
		{"missing task id", UpdateTaskBrief{}, "task id required"},
		{"valid", UpdateTaskBrief{TaskID: "t"}, ""},
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

func TestUpdateTaskBrief_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	if err := s.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "r", Title: "T"}); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	cmd := &UpdateTaskBrief{
		TaskID:          "task-1",
		WhyNow:          "because",
		SuccessCriteria: "it works",
		OutOfScope:      "edge cases",
		KnownRisks:      "none",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	brief, err := s.GetTaskBrief("task-1")
	if err != nil {
		t.Fatalf("get brief: %v", err)
	}
	if brief == nil {
		t.Fatal("brief not found")
	}
	if brief.WhyNow != "because" {
		t.Fatalf("expected why_now %q, got %q", "because", brief.WhyNow)
	}
}
