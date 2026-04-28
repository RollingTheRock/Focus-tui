package store

import (
	"testing"
	"time"
)

func TestSaveAndListTaskOutputs(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskContext(TaskContextRecord{
		ID:       "task-output-1",
		RepoID:   "/repo/main",
		Title:    "Task output",
		State:    "active",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := s.SaveTaskOutput(TaskOutputRecord{
		ID:      "out-1",
		TaskID:  "task-output-1",
		Content: "Implemented protocol bridge",
		Actor:   "agent-claude",
	}); err != nil {
		t.Fatalf("save output: %v", err)
	}
	if err := s.SaveTaskOutput(TaskOutputRecord{
		ID:        "out-2",
		TaskID:    "task-output-1",
		Content:   "Added tests",
		Actor:     "agent-codex",
		CreatedAt: time.Now().Add(time.Second),
	}); err != nil {
		t.Fatalf("save output2: %v", err)
	}

	outputs, err := s.ListTaskOutputs("task-output-1")
	if err != nil {
		t.Fatalf("list outputs: %v", err)
	}
	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %+v", outputs)
	}
	if outputs[0].ID != "out-2" || outputs[1].ID != "out-1" {
		t.Fatalf("expected reverse created order, got %+v", outputs)
	}
}
