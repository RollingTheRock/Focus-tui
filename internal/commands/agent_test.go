package commands

import (
	"context"
	"testing"
	"time"
)

func TestRequestIntervention_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     RequestIntervention
		wantErr string
	}{
		{"missing id", RequestIntervention{SessionID: "s", Reason: "r"}, "intervention id required"},
		{"missing session", RequestIntervention{ID: "1", Reason: "r"}, "session_id required"},
		{"missing reason", RequestIntervention{ID: "1", SessionID: "s"}, "reason required"},
		{"valid", RequestIntervention{ID: "1", SessionID: "s", Reason: "r"}, ""},
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

func TestRequestIntervention_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &RequestIntervention{
		ID:        "int-1",
		SessionID: "sess-1",
		Reason:    "stuck",
		Actor:     "agent-a",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	msgs, err := s.ListAgentMessages("human", "intervention_request", 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].FromAgent != "sess-1" {
		t.Fatalf("expected from_agent %q, got %q", "sess-1", msgs[0].FromAgent)
	}
}

func TestHeartbeatSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     HeartbeatSession
		wantErr string
	}{
		{"missing session", HeartbeatSession{At: time.Now()}, "session_id required"},
		{"valid", HeartbeatSession{SessionID: "s", At: time.Now()}, ""},
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
