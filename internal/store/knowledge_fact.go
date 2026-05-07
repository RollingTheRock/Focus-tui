package store

import (
	"fmt"
	"time"

	"focus/internal/events"
	"focus/internal/models"
)

type KnowledgeFactRecord = models.KnowledgeFactRecord

func (s *Store) SaveKnowledgeFact(record KnowledgeFactRecord) error {
	if record.ID == "" {
		return fmt.Errorf("knowledge fact id required")
	}
	if record.Subject == "" {
		return fmt.Errorf("knowledge fact subject required")
	}
	if record.Predicate == "" {
		return fmt.Errorf("knowledge fact predicate required")
	}
	if record.Object == "" {
		return fmt.Errorf("knowledge fact object required")
	}
	if record.Source == "" {
		return fmt.Errorf("knowledge fact source required")
	}
	if record.Confidence <= 0 {
		record.Confidence = 1
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}

	scopeID := record.PlanID
	if scopeID == "" {
		scopeID = record.ID
	}
	s.tryAppendEvent(events.AggregateKnowledgeFact, record.ID, events.KnowledgeFactAdded,
		events.KnowledgeFactAddedPayload{
			PlanID:     record.PlanID,
			Subject:    record.Subject,
			Predicate:  record.Predicate,
			Object:     record.Object,
			Source:     record.Source,
			Confidence: record.Confidence,
		}, events.AggregateKnowledgeFact, scopeID)

	q := fmt.Sprintf(`
		INSERT INTO %s (id, plan_id, subject, predicate, object, source, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			plan_id = excluded.plan_id,
			subject = excluded.subject,
			predicate = excluded.predicate,
			object = excluded.object,
			source = excluded.source,
			confidence = excluded.confidence
	`, s.tbl("knowledge_facts", "proj_knowledge_facts"))
	_, err := s.exec(
		q,
		record.ID,
		nullIfEmpty(record.PlanID),
		record.Subject,
		record.Predicate,
		record.Object,
		record.Source,
		record.Confidence,
		record.CreatedAt,
	)
	return err
}

func (s *Store) ListKnowledgeFacts(planID string) ([]KnowledgeFactRecord, error) {
	base := fmt.Sprintf(`
		SELECT id, COALESCE(plan_id, ''), subject, predicate, object, source, confidence, created_at
		FROM %s
	`, s.tbl("knowledge_facts", "proj_knowledge_facts"))
	q := base
	args := []any{}
	if planID != "" {
		q += ` WHERE plan_id = ?`
		args = append(args, planID)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]KnowledgeFactRecord, 0)
	for rows.Next() {
		var record KnowledgeFactRecord
		if err := rows.Scan(
			&record.ID,
			&record.PlanID,
			&record.Subject,
			&record.Predicate,
			&record.Object,
			&record.Source,
			&record.Confidence,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
