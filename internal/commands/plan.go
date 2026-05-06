package commands

import (
	"context"
	"fmt"

	"focus/internal/models"
	"focus/internal/store"
)

// CreatePlan creates a new task plan.
type CreatePlan struct {
	ID, TaskID, Title, WhyNow, Success, OutOfScope, KnownRisks, PlanBody string
}

// Validate checks that the title is present.
func (c *CreatePlan) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("plan id required")
	}
	if c.Title == "" {
		return fmt.Errorf("title required")
	}
	return nil
}

// Execute persists the new plan via the store.
func (c *CreatePlan) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskPlan(models.TaskPlanRecord{
		ID:         c.ID,
		TaskID:     c.TaskID,
		Title:      c.Title,
		WhyNow:     c.WhyNow,
		Success:    c.Success,
		OutOfScope: c.OutOfScope,
		KnownRisks: c.KnownRisks,
		PlanBody:   c.PlanBody,
		Status:     "draft",
	})
}

// AddPlanStep adds a step to an existing plan.
type AddPlanStep struct {
	ID, PlanID, Title, Notes string
	OrderIndex               int
}

// Validate checks that plan ID and title are present.
func (c *AddPlanStep) Validate() error {
	if c.PlanID == "" {
		return fmt.Errorf("plan id required")
	}
	if c.Title == "" {
		return fmt.Errorf("title required")
	}
	if c.ID == "" {
		return fmt.Errorf("step id required")
	}
	return nil
}

// Execute persists the new plan step via the store.
func (c *AddPlanStep) Execute(ctx context.Context, s *store.Store) error {
	return s.SavePlanStep(models.PlanStepRecord{
		ID:         c.ID,
		PlanID:     c.PlanID,
		Title:      c.Title,
		Notes:      c.Notes,
		OrderIndex: c.OrderIndex,
		State:      "pending",
	})
}

// UpdatePlan performs a full update of a task plan record.
type UpdatePlan struct {
	Record models.TaskPlanRecord
}

// Validate checks required fields.
func (c *UpdatePlan) Validate() error {
	if c.Record.ID == "" {
		return fmt.Errorf("plan id required")
	}
	if c.Record.Title == "" {
		return fmt.Errorf("title required")
	}
	return nil
}

// Execute persists the complete plan record via the store.
func (c *UpdatePlan) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveTaskPlan(c.Record)
}

// UpdatePlanStep performs a full update of a plan step record.
type UpdatePlanStep struct {
	Record models.PlanStepRecord
}

// Validate checks required fields.
func (c *UpdatePlanStep) Validate() error {
	if c.Record.ID == "" {
		return fmt.Errorf("step id required")
	}
	if c.Record.PlanID == "" {
		return fmt.Errorf("plan id required")
	}
	return nil
}

// Execute persists the complete step record via the store.
func (c *UpdatePlanStep) Execute(ctx context.Context, s *store.Store) error {
	return s.SavePlanStep(c.Record)
}


