package store

import (
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
	if err := s.SaveWorktreeContext(WorktreeContextRecord{
		WorktreeID:     "/repo/feature-a",
		RepoID:         "/repo/main",
		PrimaryTaskID:  stringPtr("task-1"),
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
	if record == nil || record.TaskName != "Feature A" || record.PrimaryTaskID == nil || *record.PrimaryTaskID != "task-1" {
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

func stringPtr(value string) *string {
	return &value
}
