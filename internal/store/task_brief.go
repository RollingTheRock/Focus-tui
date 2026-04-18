package store

import (
	"database/sql"

	"focus/internal/models"
)

type TaskBriefRecord = models.TaskBriefRecord

func (s *Store) SaveTaskBrief(record TaskBriefRecord) error {
	const q = `
		INSERT INTO task_briefs (
			task_id, why_now, success_criteria, out_of_scope, known_risks, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(task_id) DO UPDATE SET
			why_now = excluded.why_now,
			success_criteria = excluded.success_criteria,
			out_of_scope = excluded.out_of_scope,
			known_risks = excluded.known_risks,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
		record.TaskID,
		nullIfEmpty(record.WhyNow),
		nullIfEmpty(record.SuccessCriteria),
		nullIfEmpty(record.OutOfScope),
		nullIfEmpty(record.KnownRisks),
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

func (s *Store) GetTaskBrief(taskID string) (*TaskBriefRecord, error) {
	const q = `
		SELECT task_id, why_now, success_criteria, out_of_scope, known_risks, created_at, updated_at
		FROM task_briefs WHERE task_id = ?
	`
	row := s.db.QueryRow(q, taskID)
	var record TaskBriefRecord
	var whyNow, successCriteria, outOfScope, knownRisks sql.NullString
	if err := row.Scan(&record.TaskID, &whyNow, &successCriteria, &outOfScope, &knownRisks, &record.CreatedAt, &record.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if whyNow.Valid {
		record.WhyNow = whyNow.String
	}
	if successCriteria.Valid {
		record.SuccessCriteria = successCriteria.String
	}
	if outOfScope.Valid {
		record.OutOfScope = outOfScope.String
	}
	if knownRisks.Valid {
		record.KnownRisks = knownRisks.String
	}
	return &record, nil
}
