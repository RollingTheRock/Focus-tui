package commands

import (
	"context"
	"testing"

	"focus/internal/models"
)

func TestAddTaskDependency_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     AddTaskDependency
		wantErr string
	}{
		{"missing from", AddTaskDependency{ToTaskID: "b"}, "from_task_id required"},
		{"missing to", AddTaskDependency{FromTaskID: "a"}, "to_task_id required"},
		{"valid", AddTaskDependency{FromTaskID: "a", ToTaskID: "b"}, ""},
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

func TestAddTaskDependency_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed tasks
	for _, id := range []string{"a", "b"} {
		if err := s.SaveTaskContext(models.TaskContextRecord{ID: id, RepoID: "r", Title: id}); err != nil {
			t.Fatalf("seed task %s: %v", id, err)
		}
	}

	cmd := &AddTaskDependency{FromTaskID: "a", ToTaskID: "b", DependencyType: "soft"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify via downstream query
	downstream, err := s.ListDownstreamTaskContexts("a")
	if err != nil {
		t.Fatalf("list downstream: %v", err)
	}
	if len(downstream) != 1 || downstream[0].ID != "b" {
		t.Fatalf("expected downstream [b], got %+v", downstream)
	}
}
