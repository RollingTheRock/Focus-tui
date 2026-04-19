package store

import (
	"database/sql"
	"fmt"

	"focus/internal/models"
)

type TaskPlanRecord = models.TaskPlanRecord

func (s *Store) SaveTaskPlan(record TaskPlanRecord) error {
	if record.ID == "" {
		return fmt.Errorf("task plan id required")
	}
	if record.Title == "" {
		return fmt.Errorf("task plan title required")
	}
	if record.Status == "" {
		record.Status = "draft"
	}
	const q = `
		INSERT INTO task_plans (
			id, task_id, title, why_now, success, out_of_scope, known_risks, status, current_step, plan_body,
			created_at, updated_at, archived_at, done_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			title = excluded.title,
			why_now = excluded.why_now,
			success = excluded.success,
			out_of_scope = excluded.out_of_scope,
			known_risks = excluded.known_risks,
			status = excluded.status,
			current_step = excluded.current_step,
			plan_body = excluded.plan_body,
			archived_at = excluded.archived_at,
			done_at = excluded.done_at,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
		record.ID,
		nullIfEmpty(record.TaskID),
		record.Title,
		nullIfEmpty(record.WhyNow),
		nullIfEmpty(record.Success),
		nullIfEmpty(record.OutOfScope),
		nullIfEmpty(record.KnownRisks),
		record.Status,
		nullIfEmpty(record.CurrentStep),
		nullIfEmpty(record.PlanBody),
		nullableTimeValue(record.CreatedAt),
		nullableTimePtr(record.ArchivedAt),
		nullableTimePtr(record.DoneAt),
	)
	return err
}

func (s *Store) GetTaskPlan(id string) (*TaskPlanRecord, error) {
	const q = `
		SELECT id, task_id, title, why_now, success, out_of_scope, known_risks, status, current_step, plan_body,
		       created_at, updated_at, archived_at, done_at
		FROM task_plans WHERE id = ?
	`
	row := s.db.QueryRow(q, id)
	return scanTaskPlan(row)
}

func (s *Store) ListTaskPlans(taskID string) ([]TaskPlanRecord, error) {
	const base = `
		SELECT id, task_id, title, why_now, success, out_of_scope, known_risks, status, current_step, plan_body,
		       created_at, updated_at, archived_at, done_at
		FROM task_plans
	`
	q := base
	args := []any{}
	if taskID != "" {
		q += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	q += ` ORDER BY updated_at DESC, created_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []TaskPlanRecord
	for rows.Next() {
		record, err := scanTaskPlanRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func scanTaskPlan(row *sql.Row) (*TaskPlanRecord, error) {
	var record TaskPlanRecord
	var taskID sql.NullString
	var whyNow, success, outOfScope, knownRisks sql.NullString
	var currentStep, planBody sql.NullString
	var archivedAt, doneAt sql.NullTime
	if err := row.Scan(&record.ID, &taskID, &record.Title, &whyNow, &success, &outOfScope, &knownRisks, &record.Status, &currentStep, &planBody, &record.CreatedAt, &record.UpdatedAt, &archivedAt, &doneAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if taskID.Valid {
		record.TaskID = taskID.String
	}
	if whyNow.Valid {
		record.WhyNow = whyNow.String
	}
	if success.Valid {
		record.Success = success.String
	}
	if outOfScope.Valid {
		record.OutOfScope = outOfScope.String
	}
	if knownRisks.Valid {
		record.KnownRisks = knownRisks.String
	}
	if currentStep.Valid {
		record.CurrentStep = currentStep.String
	}
	if planBody.Valid {
		record.PlanBody = planBody.String
	}
	if archivedAt.Valid {
		t := archivedAt.Time
		record.ArchivedAt = &t
	}
	if doneAt.Valid {
		t := doneAt.Time
		record.DoneAt = &t
	}
	return &record, nil
}

func scanTaskPlanRows(rows *sql.Rows) (*TaskPlanRecord, error) {
	var record TaskPlanRecord
	var taskID sql.NullString
	var whyNow, success, outOfScope, knownRisks sql.NullString
	var currentStep, planBody sql.NullString
	var archivedAt, doneAt sql.NullTime
	if err := rows.Scan(&record.ID, &taskID, &record.Title, &whyNow, &success, &outOfScope, &knownRisks, &record.Status, &currentStep, &planBody, &record.CreatedAt, &record.UpdatedAt, &archivedAt, &doneAt); err != nil {
		return nil, err
	}
	if taskID.Valid {
		record.TaskID = taskID.String
	}
	if whyNow.Valid {
		record.WhyNow = whyNow.String
	}
	if success.Valid {
		record.Success = success.String
	}
	if outOfScope.Valid {
		record.OutOfScope = outOfScope.String
	}
	if knownRisks.Valid {
		record.KnownRisks = knownRisks.String
	}
	if currentStep.Valid {
		record.CurrentStep = currentStep.String
	}
	if planBody.Valid {
		record.PlanBody = planBody.String
	}
	if archivedAt.Valid {
		t := archivedAt.Time
		record.ArchivedAt = &t
	}
	if doneAt.Valid {
		t := doneAt.Time
		record.DoneAt = &t
	}
	return &record, nil
}
