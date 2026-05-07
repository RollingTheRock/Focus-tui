package commands

import (
	"context"
	"fmt"

	"focus/internal/models"
	"focus/internal/store"
)

// CreateTaskOutput records an artifact or output for a task.
type CreateTaskOutput struct {
	ID, TaskID, Content, Actor string
}

// Validate checks that required fields are present.
func (c *CreateTaskOutput) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	if c.ID == "" {
		return fmt.Errorf("output id required")
	}
	if c.Content == "" {
		return fmt.Errorf("content required")
	}
	return nil
}

// Execute persists the task output via the store.
func (c *CreateTaskOutput) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskOutput(models.TaskOutputRecord{
		ID:      c.ID,
		TaskID:  c.TaskID,
		Content: c.Content,
		Actor:   c.Actor,
	})
}
