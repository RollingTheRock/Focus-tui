package store

import (
	"database/sql"
	"fmt"

	"focus/internal/models"
)

type SessionHandoffRecord = models.SessionHandoffRecord

func (s *Store) SaveSessionHandoff(record SessionHandoffRecord) error {
	if record.ID == "" {
		return fmt.Errorf("session handoff id required")
	}
	if record.TaskID == "" {
		return fmt.Errorf("session handoff task_id required")
	}
	const q = `
		INSERT INTO session_handoffs (
			id, task_id, plan_id, session_id,
			done_summary, remaining_summary, decision_summary,
			uncertainty_summary, blocker_summary, entrypoint, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			plan_id = excluded.plan_id,
			session_id = excluded.session_id,
			done_summary = excluded.done_summary,
			remaining_summary = excluded.remaining_summary,
			decision_summary = excluded.decision_summary,
			uncertainty_summary = excluded.uncertainty_summary,
			blocker_summary = excluded.blocker_summary,
			entrypoint = excluded.entrypoint
	`
	_, err := s.db.Exec(q,
		record.ID,
		record.TaskID,
		record.PlanID,
		nullIfEmpty(record.SessionID),
		nullIfEmpty(record.DoneSummary),
		nullIfEmpty(record.RemainingSummary),
		nullIfEmpty(record.DecisionSummary),
		nullIfEmpty(record.UncertaintySummary),
		nullIfEmpty(record.BlockerSummary),
		nullIfEmpty(record.Entrypoint),
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

func (s *Store) ListSessionHandoffs(taskID string) ([]SessionHandoffRecord, error) {
	const q = `
		SELECT id, task_id, plan_id, session_id,
		       done_summary, remaining_summary, decision_summary,
		       uncertainty_summary, blocker_summary, entrypoint, created_at
		FROM session_handoffs
		WHERE task_id = ?
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(q, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []SessionHandoffRecord
	for rows.Next() {
		var record SessionHandoffRecord
		var planID sql.NullString
		var sessionID sql.NullString
		var doneSummary, remainingSummary, decisionSummary, uncertaintySummary, blockerSummary, entrypoint sql.NullString
		if err := rows.Scan(&record.ID, &record.TaskID, &planID, &sessionID, &doneSummary, &remainingSummary, &decisionSummary, &uncertaintySummary, &blockerSummary, &entrypoint, &record.CreatedAt); err != nil {
			return nil, err
		}
		if planID.Valid {
			record.PlanID = &planID.String
		}
		if sessionID.Valid {
			record.SessionID = sessionID.String
		}
		if doneSummary.Valid {
			record.DoneSummary = doneSummary.String
		}
		if remainingSummary.Valid {
			record.RemainingSummary = remainingSummary.String
		}
		if decisionSummary.Valid {
			record.DecisionSummary = decisionSummary.String
		}
		if uncertaintySummary.Valid {
			record.UncertaintySummary = uncertaintySummary.String
		}
		if blockerSummary.Valid {
			record.BlockerSummary = blockerSummary.String
		}
		if entrypoint.Valid {
			record.Entrypoint = entrypoint.String
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
