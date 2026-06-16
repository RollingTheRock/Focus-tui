package commands

import (
	"context"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

func TestAddKnowledgeFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     AddKnowledgeFact
		wantErr string
	}{
		{"missing id", AddKnowledgeFact{Subject: "s", Predicate: "p", Object: "o"}, "fact id required"},
		{"missing subject", AddKnowledgeFact{ID: "1", Predicate: "p", Object: "o"}, "subject/predicate/object required"},
		{"missing predicate", AddKnowledgeFact{ID: "1", Subject: "s", Object: "o"}, "subject/predicate/object required"},
		{"missing object", AddKnowledgeFact{ID: "1", Subject: "s", Predicate: "p"}, "subject/predicate/object required"},
		{"valid", AddKnowledgeFact{ID: "1", Subject: "s", Predicate: "p", Object: "o"}, ""},
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

func TestAddKnowledgeFact_Execute(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	// seed plan
	if err := s.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-1", Title: "P", Status: "draft"}); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	cmd := &AddKnowledgeFact{
		ID:         "fact-1",
		PlanID:     "plan-1",
		Subject:    "Go",
		Predicate:  "is",
		Object:     "great",
		Source:     "agent",
		Confidence: 0.95,
	}
	if err := cmd.Execute(ctx, s); err != nil {
		t.Fatalf("execute: %v", err)
	}

	facts, err := s.ListKnowledgeFacts("plan-1")
	if err != nil {
		t.Fatalf("list facts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Object != "great" {
		t.Fatalf("expected object %q, got %q", "great", facts[0].Object)
	}
}
