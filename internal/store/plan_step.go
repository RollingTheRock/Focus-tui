package store

import (
	"database/sql"
	"fmt"

	"focus/internal/models"
)

type PlanStepRecord = models.PlanStepRecord

func (s *Store) SavePlanStep(record PlanStepRecord) error {
	if record.ID == "" {
		return fmt.Errorf("plan step id required")
	}
	if record.PlanID == "" {
		return fmt.Errorf("plan step plan_id required")
	}
	if record.Title == "" {
		return fmt.Errorf("plan step title required")
	}
	if record.State == "" {
		record.State = "pending"
	}
	const q = `
		INSERT INTO plan_steps (
			id, plan_id, order_index, title, state, expanded_task_id, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			plan_id = excluded.plan_id,
			order_index = excluded.order_index,
			title = excluded.title,
			state = excluded.state,
			expanded_task_id = excluded.expanded_task_id,
			notes = excluded.notes,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
		record.ID,
		record.PlanID,
		record.OrderIndex,
		record.Title,
		record.State,
		nullIfEmpty(record.ExpandedTaskID),
		nullIfEmpty(record.Notes),
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

func (s *Store) DeletePlanSteps(planID string) error {
	if planID == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM plan_steps WHERE plan_id = ?`, planID)
	return err
}

func (s *Store) ListPlanSteps(planID string) ([]PlanStepRecord, error) {
	const q = `
		SELECT id, plan_id, order_index, title, state, expanded_task_id, notes, created_at, updated_at
		FROM plan_steps
		WHERE plan_id = ?
		ORDER BY order_index ASC, created_at ASC
	`
	rows, err := s.db.Query(q, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []PlanStepRecord
	for rows.Next() {
		var record PlanStepRecord
		var expandedTaskID sql.NullString
		var notes sql.NullString
		if err := rows.Scan(&record.ID, &record.PlanID, &record.OrderIndex, &record.Title, &record.State, &expandedTaskID, &notes, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		if expandedTaskID.Valid {
			record.ExpandedTaskID = expandedTaskID.String
		}
		if notes.Valid {
			record.Notes = notes.String
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
