package commands

import (
	"context"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// LinkTaskToWorktree creates or updates a task-worktree association.
type LinkTaskToWorktree struct {
	ID, TaskID, WorktreeID, RelationType string
}

// Validate checks required fields.
func (c *LinkTaskToWorktree) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("link id required")
	}
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	if c.WorktreeID == "" {
		return fmt.Errorf("worktree id required")
	}
	if c.RelationType == "" {
		c.RelationType = "primary"
	}
	return nil
}

// Execute persists the link via the store.
func (c *LinkTaskToWorktree) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskWorktreeLink(models.TaskWorktreeLinkRecord{
		ID:           c.ID,
		TaskID:       c.TaskID,
		WorktreeID:   c.WorktreeID,
		RelationType: c.RelationType,
	})
}

// DeletePlanSteps removes all steps belonging to a plan.
type DeletePlanSteps struct {
	PlanID string
}

// Validate checks the plan ID.
func (c *DeletePlanSteps) Validate() error {
	if c.PlanID == "" {
		return fmt.Errorf("plan id required")
	}
	return nil
}

// Execute deletes the plan steps via the store.
func (c *DeletePlanSteps) Execute(ctx context.Context, s *store.Store) error {
	return s.DeletePlanSteps(c.PlanID)
}
