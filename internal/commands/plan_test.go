package commands

import (
	"context"
	"testing"

	"focus/internal/models"
)

func TestCreatePlan_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreatePlan
		wantErr string
	}{
		{"missing id", CreatePlan{Title: "t"}, "plan id required"},
		{"missing title", CreatePlan{ID: "1"}, "title required"},
		{"valid", CreatePlan{ID: "1", Title: "t"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCreatePlan_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	cmd := &CreatePlan{
		ID:       "plan-1",
		Title:    "My Plan",
		WhyNow:   "because",
		PlanBody: "steps...",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	plan, err := s.GetTaskPlan("plan-1")
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if plan == nil {
		t.Fatal("plan not found")
	}
	if plan.Status != "draft" {
		t.Fatalf("expected status draft, got %q", plan.Status)
	}
	if plan.Title != "My Plan" {
		t.Fatalf("expected title %q, got %q", "My Plan", plan.Title)
	}
}

func TestAddPlanStep_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     AddPlanStep
		wantErr string
	}{
		{"missing plan id", AddPlanStep{ID: "s", Title: "t"}, "plan id required"},
		{"missing title", AddPlanStep{PlanID: "p", ID: "s"}, "title required"},
		{"missing id", AddPlanStep{PlanID: "p", Title: "t"}, "step id required"},
		{"valid", AddPlanStep{PlanID: "p", ID: "s", Title: "t"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAddPlanStep_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed plan
	if err := s.SaveTaskPlan(models.TaskPlanRecord{
		ID:     "plan-1",
		Title:  "P",
		Status: "draft",
	}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	cmd := &AddPlanStep{
		ID:         "step-1",
		PlanID:     "plan-1",
		Title:      "Step One",
		OrderIndex: 0,
		Notes:      "note",
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	steps, err := s.ListPlanSteps("plan-1")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Title != "Step One" {
		t.Fatalf("expected title %q, got %q", "Step One", steps[0].Title)
	}
	if steps[0].State != "pending" {
		t.Fatalf("expected state pending, got %q", steps[0].State)
	}
}
