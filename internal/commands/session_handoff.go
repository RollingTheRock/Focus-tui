package commands

import (
	"context"
	"fmt"

	"focus/internal/models"
	"focus/internal/store"
)

// CreateSessionHandoff records a handoff between sessions.
type CreateSessionHandoff struct {
	ID, TaskID, SessionID, DoneSummary, RemainingSummary, DecisionSummary, Entrypoint string
	PlanID                                                                           *string
}

// Validate checks required fields.
func (c *CreateSessionHandoff) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("handoff id required")
	}
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	if c.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	return nil
}

// Execute persists the handoff via the store.
func (c *CreateSessionHandoff) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveSessionHandoff(models.SessionHandoffRecord{
		ID:               c.ID,
		TaskID:           c.TaskID,
		PlanID:           c.PlanID,
		SessionID:        c.SessionID,
		DoneSummary:      c.DoneSummary,
		RemainingSummary: c.RemainingSummary,
		DecisionSummary:  c.DecisionSummary,
		Entrypoint:       c.Entrypoint,
	})
}
