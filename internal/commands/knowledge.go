package commands

import (
	"context"
	"fmt"

	"focus/internal/models"
	"focus/internal/store"
)

// AddKnowledgeFact adds a fact to the knowledge graph.
type AddKnowledgeFact struct {
	ID, PlanID, Subject, Predicate, Object, Source string
	Confidence                                     float64
}

// Validate checks required fields.
func (c *AddKnowledgeFact) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("fact id required")
	}
	if c.Subject == "" || c.Predicate == "" || c.Object == "" {
		return fmt.Errorf("subject/predicate/object required")
	}
	return nil
}

// Execute persists the knowledge fact via the store.
func (c *AddKnowledgeFact) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveKnowledgeFact(models.KnowledgeFactRecord{
		ID:         c.ID,
		PlanID:     c.PlanID,
		Subject:    c.Subject,
		Predicate:  c.Predicate,
		Object:     c.Object,
		Source:     c.Source,
		Confidence: c.Confidence,
	})
}
