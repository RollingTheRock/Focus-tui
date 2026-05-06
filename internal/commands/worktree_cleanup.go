package commands

import (
	"context"
	"fmt"

	"focus/internal/store"
)

// DeleteWorktree cleans up all records associated with a worktree.
type DeleteWorktree struct {
	WorktreePath string
}

// Validate checks the worktree path.
func (c *DeleteWorktree) Validate() error {
	if c.WorktreePath == "" {
		return fmt.Errorf("worktree path required")
	}
	return nil
}

// Execute deletes all associated records via the store.
func (c *DeleteWorktree) Execute(ctx context.Context, s *store.Store) error {
	_ = s.DeletePageSnapshot(c.WorktreePath)
	_ = s.DeleteWorktreeContext(c.WorktreePath)
	_ = s.DeleteAgentSessionsByWorktreeID(c.WorktreePath)
	_ = s.DeleteContextNotesByWorktreeID(c.WorktreePath)
	_ = s.DeleteTaskWorktreeLinksByWorktreeID(c.WorktreePath)
	return nil
}
