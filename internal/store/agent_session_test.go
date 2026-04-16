package store

import (
	"testing"
	"time"
)

func TestSaveAndListAgentSessions(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	startedAt := time.Now().Add(-2 * time.Minute).UTC().Round(time.Second)
	record := AgentSessionRecord{
		ID:             "session-1",
		Provider:       "opencode",
		WorktreeID:     "/repo/feature-a",
		RepoID:         "/repo/main",
		BranchSnapshot: "feature-a",
		PID:            1234,
		State:          "running",
		StartedAt:      startedAt,
	}
	if err := s.SaveAgentSession(record); err != nil {
		t.Fatalf("save session: %v", err)
	}

	records, err := s.ListAgentSessions("/repo/feature-a")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 session, got %d", len(records))
	}
	if records[0].ID != record.ID || records[0].Provider != record.Provider {
		t.Fatalf("unexpected record %+v", records[0])
	}
	if records[0].BranchSnapshot != "feature-a" {
		t.Fatalf("expected branch snapshot feature-a, got %q", records[0].BranchSnapshot)
	}
}

func TestSaveAgentSessionUpsertsState(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	startedAt := time.Now().Add(-time.Minute)
	if err := s.SaveAgentSession(AgentSessionRecord{
		ID:         "session-1",
		Provider:   "claude",
		WorktreeID: "/repo/feature-b",
		State:      "running",
		PID:        2222,
		StartedAt:  startedAt,
	}); err != nil {
		t.Fatalf("save running session: %v", err)
	}

	endedAt := time.Now().UTC().Round(time.Second)
	if err := s.SaveAgentSession(AgentSessionRecord{
		ID:         "session-1",
		Provider:   "claude",
		WorktreeID: "/repo/feature-b",
		State:      "exited",
		PID:        0,
		StartedAt:  startedAt,
		EndedAt:    &endedAt,
	}); err != nil {
		t.Fatalf("save exited session: %v", err)
	}

	records, err := s.ListAgentSessions("/repo/feature-b")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 session, got %d", len(records))
	}
	if records[0].State != "exited" || records[0].PID != 0 {
		t.Fatalf("expected exited session with cleared pid, got %+v", records[0])
	}
	if records[0].EndedAt == nil {
		t.Fatal("expected ended_at to be persisted")
	}
}
