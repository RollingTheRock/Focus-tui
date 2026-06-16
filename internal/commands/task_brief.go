package commands

import (
	"context"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// UpdateTaskBrief updates the brief metadata for a task.
type UpdateTaskBrief struct {
	TaskID          string
	WhyNow          string
	SuccessCriteria string
	OutOfScope      string
	KnownRisks      string
}

// Validate checks the task ID.
func (c *UpdateTaskBrief) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	return nil
}

// Execute persists the task brief via the store.
func (c *UpdateTaskBrief) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskBrief(models.TaskBriefRecord{
		TaskID:          c.TaskID,
		WhyNow:          c.WhyNow,
		SuccessCriteria: c.SuccessCriteria,
		OutOfScope:      c.OutOfScope,
		KnownRisks:      c.KnownRisks,
	})
}
