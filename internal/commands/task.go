package commands

import (
	"context"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// validTaskStates is the canonical set of task states.
// Must stay in sync with the SQLite CHECK constraint on task_contexts.state.
var validTaskStates = map[string]bool{
	"active":   true,
	"paused":   true,
	"blocked":  true,
	"done":     true,
	"archived": true,
}

func isValidTaskState(state string) bool {
	return validTaskStates[state]
}

// CreateTask creates a new task context.
type CreateTask struct {
	ID, RepoID, Title, Goal, NextStep, Priority, State string
	ParentTaskID                                       *string
	PreferredWorktreeID                                string
}

// Validate checks that required fields are present.
func (c *CreateTask) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("task id required")
	}
	if c.RepoID == "" {
		return fmt.Errorf("repo id required")
	}
	if c.Title == "" {
		return fmt.Errorf("title required")
	}
	return nil
}

// Execute persists the new task via the store.
func (c *CreateTask) Execute(ctx context.Context, s *store.Store) error {
	state := c.State
	if state == "" {
		state = "active"
	}
	priority := c.Priority
	if priority == "" {
		priority = "medium"
	}
	return s.SaveTaskContext(models.TaskContextRecord{
		ID:                  c.ID,
		RepoID:              c.RepoID,
		Title:               c.Title,
		Goal:                c.Goal,
		NextStep:            c.NextStep,
		Priority:            priority,
		ParentTaskID:        c.ParentTaskID,
		PreferredWorktreeID: c.PreferredWorktreeID,
		State:               state,
	})
}

// UpdateTaskState transitions a task to a new state.
type UpdateTaskState struct {
	TaskID   string
	NewState string
}

// Validate checks IDs and state validity.
func (c *UpdateTaskState) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	if !isValidTaskState(c.NewState) {
		return fmt.Errorf("invalid state %q", c.NewState)
	}
	return nil
}

// Execute reads the existing task, mutates its state, and persists.
func (c *UpdateTaskState) Execute(ctx context.Context, s *store.Store) error {
	task, err := s.GetTaskContext(c.TaskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", c.TaskID)
	}
	task.State = c.NewState
	return s.SaveTaskContext(*task)
}

// UpdateTaskGoal modifies a task's goal and next_step.
type UpdateTaskGoal struct {
	TaskID   string
	Goal     string
	NextStep string
}

// Validate checks the task ID.
func (c *UpdateTaskGoal) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	return nil
}

// Execute reads the existing task, mutates goal/next_step, and persists.
func (c *UpdateTaskGoal) Execute(ctx context.Context, s *store.Store) error {
	task, err := s.GetTaskContext(c.TaskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", c.TaskID)
	}
	task.Goal = c.Goal
	task.NextStep = c.NextStep
	return s.SaveTaskContext(*task)
}

// UpdateTask performs a full UPSERT of a task context record.
// It is used by editors and orchestrators that already have the complete record.
type UpdateTask struct {
	Record models.TaskContextRecord
}

// Validate checks that required fields are present.
func (c *UpdateTask) Validate() error {
	if c.Record.ID == "" {
		return fmt.Errorf("task id required")
	}
	if c.Record.RepoID == "" {
		return fmt.Errorf("repo id required")
	}
	if c.Record.Title == "" {
		return fmt.Errorf("title required")
	}
	return nil
}

// Execute persists the complete record via the store.
func (c *UpdateTask) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskContext(c.Record)
}

// DeleteTask removes a task and all its children (Steps) from the store.
type DeleteTask struct {
	TaskID string
}

// Validate checks the task ID.
func (c *DeleteTask) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	return nil
}

// Execute deletes the task via the store (children are handled recursively).
func (c *DeleteTask) Execute(ctx context.Context, s *store.Store) error {
	return s.DeleteTaskContext(c.TaskID)
}

// ArchiveTask archives a task (sets state to archived).
type ArchiveTask struct {
	TaskID string
}

// Validate checks the task ID.
func (c *ArchiveTask) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	return nil
}

// Execute archives the task via the store.
func (c *ArchiveTask) Execute(ctx context.Context, s *store.Store) error {
	return s.ArchiveTaskContext(c.TaskID)
}

// RestoreTask restores a deleted or archived task back to active.
type RestoreTask struct {
	TaskID string
}

// Validate checks the task ID.
func (c *RestoreTask) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	return nil
}

// Execute restores the task via the store.
func (c *RestoreTask) Execute(ctx context.Context, s *store.Store) error {
	return s.RestoreTaskContext(c.TaskID)
}
