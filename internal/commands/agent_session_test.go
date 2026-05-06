package commands

import (
	"context"
	"testing"

	"focus/internal/models"
)

func TestCreateAgentSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreateAgentSession
		wantErr string
	}{
		{"missing id", CreateAgentSession{Record: models.AgentSessionRecord{WorktreeID: "w", Provider: "p"}}, "session id required"},
		{"valid", CreateAgentSession{Record: models.AgentSessionRecord{ID: "s", WorktreeID: "w", Provider: "p"}}, ""},
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

func TestCreateAgentSession_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &CreateAgentSession{
		Record: models.AgentSessionRecord{
			ID:         "sess-1",
			WorktreeID: "/repo/wt1",
			Provider:   "test",
			State:      "running",
		},
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	sessions, err := s.ListAgentSessions("/repo/wt1")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Provider != "test" {
		t.Fatalf("expected provider %q, got %q", "test", sessions[0].Provider)
	}
}
