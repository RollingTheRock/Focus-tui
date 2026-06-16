package commands

import (
	"context"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

func TestCreateTaskOutput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreateTaskOutput
		wantErr string
	}{
		{"missing task id", CreateTaskOutput{ID: "o", Content: "c"}, "task id required"},
		{"missing id", CreateTaskOutput{TaskID: "t", Content: "c"}, "output id required"},
		{"missing content", CreateTaskOutput{TaskID: "t", ID: "o"}, "content required"},
		{"valid", CreateTaskOutput{TaskID: "t", ID: "o", Content: "c"}, ""},
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

func TestCreateTaskOutput_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed task
	if err := s.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "r", Title: "T"}); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	cmd := &CreateTaskOutput{ID: "out-1", TaskID: "task-1", Content: "hello", Actor: "agent"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	outputs, err := s.ListTaskOutputs("task-1")
	if err != nil {
		t.Fatalf("list outputs: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(outputs))
	}
	if outputs[0].Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", outputs[0].Content)
	}
}
