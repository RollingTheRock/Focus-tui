package commands

import (
	"context"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	return s
}

func TestCreateTask_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreateTask
		wantErr string
	}{
		{"missing id", CreateTask{RepoID: "r", Title: "t"}, "task id required"},
		{"missing repo", CreateTask{ID: "1", Title: "t"}, "repo id required"},
		{"missing title", CreateTask{ID: "1", RepoID: "r"}, "title required"},
		{"valid", CreateTask{ID: "1", RepoID: "r", Title: "t"}, ""},
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

func TestCreateTask_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &CreateTask{
		ID:     "task-1",
		RepoID: "repo-1",
		Title:  "Test Task",
		Goal:   "do something",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute create: %v", err)
	}

	record, err := s.GetTaskContext("task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if record == nil {
		t.Fatal("task not found")
	}
	if record.State != "active" {
		t.Fatalf("expected state active, got %q", record.State)
	}
	if record.Priority != "medium" {
		t.Fatalf("expected priority medium, got %q", record.Priority)
	}
	if record.Goal != "do something" {
		t.Fatalf("expected goal %q, got %q", "do something", record.Goal)
	}
}

func TestCreateTask_Execute_WithExplicitState(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &CreateTask{
		ID:       "task-2",
		RepoID:   "repo-1",
		Title:    "Paused Task",
		State:    "paused",
		Priority: "high",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute create: %v", err)
	}

	record, err := s.GetTaskContext("task-2")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if record.State != "paused" {
		t.Fatalf("expected state paused, got %q", record.State)
	}
	if record.Priority != "high" {
		t.Fatalf("expected priority high, got %q", record.Priority)
	}
}

func TestUpdateTaskState_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     UpdateTaskState
		wantErr string
	}{
		{"missing task id", UpdateTaskState{NewState: "done"}, "task id required"},
		{"invalid state", UpdateTaskState{TaskID: "1", NewState: "frobnicate"}, `invalid state "frobnicate"`},
		{"valid", UpdateTaskState{TaskID: "1", NewState: "done"}, ""},
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

func TestUpdateTaskState_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed task
	if err := s.SaveTaskContext(models.TaskContextRecord{
		ID:     "task-1",
		RepoID: "repo-1",
		Title:  "Task",
		State:  "active",
	}); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	cmd := &UpdateTaskState{TaskID: "task-1", NewState: "done"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute update: %v", err)
	}

	record, err := s.GetTaskContext("task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if record.State != "done" {
		t.Fatalf("expected state done, got %q", record.State)
	}
}

func TestUpdateTaskState_Execute_NotFound(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &UpdateTaskState{TaskID: "missing", NewState: "done"}
	err := cmd.Execute(ctx, s)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
}

func TestUpdateTaskGoal_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	if err := s.SaveTaskContext(models.TaskContextRecord{
		ID:     "task-1",
		RepoID: "repo-1",
		Title:  "Task",
		Goal:   "old goal",
	}); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	cmd := &UpdateTaskGoal{TaskID: "task-1", Goal: "new goal", NextStep: "step 1"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute update goal: %v", err)
	}

	record, err := s.GetTaskContext("task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if record.Goal != "new goal" {
		t.Fatalf("expected goal %q, got %q", "new goal", record.Goal)
	}
	if record.NextStep != "step 1" {
		t.Fatalf("expected next_step %q, got %q", "step 1", record.NextStep)
	}
}

func TestBus_Send_ValidationFailure(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	bus := NewBus(s)
	cmd := &CreateTask{ID: "", RepoID: "r", Title: "t"} // invalid
	err := bus.Send(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "task id required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBus_Send_Success(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	bus := NewBus(s)
	cmd := &CreateTask{ID: "bus-1", RepoID: "r", Title: "t"}
	if err := bus.Send(context.Background(), cmd); err != nil {
		t.Fatalf("send command: %v", err)
	}

	record, err := s.GetTaskContext("bus-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if record == nil {
		t.Fatal("task not found after bus send")
	}
}

func TestDeleteTask_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     DeleteTask
		wantErr string
	}{
		{"missing task id", DeleteTask{TaskID: ""}, "task id required"},
		{"valid", DeleteTask{TaskID: "task-1"}, ""},
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

func TestDeleteTask_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	if err := s.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "repo-1", Title: "Task", State: "active"}); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	cmd := &DeleteTask{TaskID: "task-1"}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute delete: %v", err)
	}

	record, err := s.GetTaskContext("task-1")
	if err != nil {
		t.Fatalf("get task after delete: %v", err)
	}
	if record != nil {
		t.Fatalf("expected task to be deleted, got %+v", record)
	}
}
