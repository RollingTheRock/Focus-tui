package commands

import (
	"context"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// UpdateWorktreeContext updates the context record for a worktree.
type UpdateWorktreeContext struct {
	Record models.WorktreeContextRecord
}

// Validate checks required fields.
func (c *UpdateWorktreeContext) Validate() error {
	if c.Record.WorktreeID == "" {
		return fmt.Errorf("worktree id required")
	}
	if c.Record.RepoID == "" {
		return fmt.Errorf("repo id required")
	}
	return nil
}

// Execute persists the worktree context via the store.
func (c *UpdateWorktreeContext) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveWorktreeContext(c.Record)
}
