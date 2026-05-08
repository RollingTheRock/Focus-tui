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
	if err := s.DeletePageSnapshot(c.WorktreePath); err != nil {
		return fmt.Errorf("delete page snapshot: %w", err)
	}
	if err := s.DeleteWorktreeContext(c.WorktreePath); err != nil {
		return fmt.Errorf("delete worktree context: %w", err)
	}
	if err := s.DeleteAgentSessionsByWorktreeID(c.WorktreePath); err != nil {
		return fmt.Errorf("delete agent sessions: %w", err)
	}
	if err := s.DeleteContextNotesByWorktreeID(c.WorktreePath); err != nil {
		return fmt.Errorf("delete context notes: %w", err)
	}
	if err := s.DeleteTaskWorktreeLinksByWorktreeID(c.WorktreePath); err != nil {
		return fmt.Errorf("delete task worktree links: %w", err)
	}
	return nil
}
