package store

import (
	"database/sql"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/events"
	"github.com/RollingTheRock/Focus-tui/internal/models"
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

	var expandedTaskID *string
	if record.ExpandedTaskID != "" {
		expandedTaskID = &record.ExpandedTaskID
	}
	s.tryAppendEvent(events.AggregatePlan, record.ID, events.PlanStepStateChanged,
		events.PlanStepStateChangedPayload{
			PlanID:         record.PlanID,
			NewState:       record.State,
			OrderIndex:     record.OrderIndex,
			Title:          record.Title,
			Notes:          record.Notes,
			ExpandedTaskID: expandedTaskID,
		}, events.AggregatePlan, record.PlanID)

	q := fmt.Sprintf(`
		INSERT INTO %s (
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
	`, s.tbl("plan_steps", "proj_plan_steps"))
	_, err := s.exec(q,
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
	_, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE plan_id = ?`, s.tbl("plan_steps", "proj_plan_steps")), planID)
	return err
}

func (s *Store) ListPlanSteps(planID string) ([]PlanStepRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, plan_id, order_index, title, state, expanded_task_id, notes, created_at, updated_at
		FROM %s
		WHERE plan_id = ?
		ORDER BY order_index ASC, created_at ASC
	`, s.tbl("plan_steps", "proj_plan_steps"))
	rows, err := s.qRows(q, planID)
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
