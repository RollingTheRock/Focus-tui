package commands

import (
	"context"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// CreateAgentSession persists an agent session record.
type CreateAgentSession struct {
	Record models.AgentSessionRecord
}

// Validate checks the session ID.
func (c *CreateAgentSession) Validate() error {
	if c.Record.ID == "" {
		return fmt.Errorf("session id required")
	}
	return nil
}

// Execute persists the agent session via the store.
func (c *CreateAgentSession) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveAgentSession(c.Record)
}
