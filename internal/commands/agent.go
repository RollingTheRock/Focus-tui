package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"focus/internal/models"
	"focus/internal/store"
)

// RequestIntervention records a human-intervention request.
type RequestIntervention struct {
	ID, SessionID, Reason, Actor string
}

// Validate checks required fields.
func (c *RequestIntervention) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("intervention id required")
	}
	if c.SessionID == "" {
		return fmt.Errorf("session_id required")
	}
	if c.Reason == "" {
		return fmt.Errorf("reason required")
	}
	return nil
}

// Execute persists the intervention request as an agent message.
func (c *RequestIntervention) Execute(ctx context.Context, s *store.Store) error {
	payloadJSON := toJSONString(map[string]any{
		"session_id": c.SessionID,
		"reason":     c.Reason,
		"actor":      c.Actor,
	})
	return s.SaveAgentMessage(models.AgentMessageRecord{
		ID:        c.ID,
		FromAgent: c.SessionID,
		ToAgent:   "human",
		MsgType:   "intervention_request",
		Payload:   payloadJSON,
	})
}

// HeartbeatSession updates an agent session heartbeat.
type HeartbeatSession struct {
	SessionID string
	At        time.Time
	State     string
}

// Validate checks the session ID.
func (c *HeartbeatSession) Validate() error {
	if c.SessionID == "" {
		return fmt.Errorf("session_id required")
	}
	return nil
}

// Execute updates the session heartbeat via the store.
func (c *HeartbeatSession) Execute(ctx context.Context, s *store.Store) error {
	return s.UpdateAgentSessionHeartbeat(c.SessionID, c.At, c.State)
}

// toJSONString is a tiny helper for JSON encoding.
func toJSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
