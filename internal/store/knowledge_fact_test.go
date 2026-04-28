package store

import "testing"

func TestSaveAndListKnowledgeFacts(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.SaveTaskPlan(TaskPlanRecord{
		ID:     "plan-1",
		Title:  "Knowledge Plan",
		Status: "active",
	}); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	if err := s.SaveKnowledgeFact(KnowledgeFactRecord{
		ID:        "fact-1",
		PlanID:    "plan-1",
		Subject:   "router",
		Predicate: "depends_on",
		Object:    "mcp",
		Source:    "agent-kimi",
	}); err != nil {
		t.Fatalf("save fact: %v", err)
	}
	if err := s.SaveKnowledgeFact(KnowledgeFactRecord{
		ID:         "fact-2",
		PlanID:     "plan-1",
		Subject:    "orchestrator",
		Predicate:  "launches",
		Object:     "downstream-task",
		Source:     "agent-claude",
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("save fact2: %v", err)
	}

	facts, err := s.ListKnowledgeFacts("plan-1")
	if err != nil {
		t.Fatalf("list facts: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %+v", facts)
	}
	if facts[0].ID != "fact-2" || facts[1].ID != "fact-1" {
		t.Fatalf("expected latest fact first, got %+v", facts)
	}
}
