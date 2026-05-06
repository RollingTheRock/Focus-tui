package commands

import (
	"context"
	"fmt"

	"focus/internal/models"
	"focus/internal/store"
)

// AddTaskDependency creates a directed dependency between two tasks.
type AddTaskDependency struct {
	FromTaskID, ToTaskID string
	DependencyType       string
}

// Validate checks that both task IDs are present.
func (c *AddTaskDependency) Validate() error {
	if c.FromTaskID == "" {
		return fmt.Errorf("from_task_id required")
	}
	if c.ToTaskID == "" {
		return fmt.Errorf("to_task_id required")
	}
	if c.DependencyType == "" {
		c.DependencyType = "hard"
	}
	return nil
}

// Execute persists the dependency via the store.
func (c *AddTaskDependency) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID:     c.FromTaskID,
		ToTaskID:       c.ToTaskID,
		DependencyType: c.DependencyType,
	})
}
