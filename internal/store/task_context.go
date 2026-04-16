package store

import (
	"database/sql"
	"fmt"

	"focus/internal/models"
)

type TaskContextRecord = models.TaskContextRecord

func (s *Store) SaveTaskContext(record TaskContextRecord) error {
	if record.ID == "" {
		return fmt.Errorf("task context id required")
	}
	if record.RepoID == "" {
		return fmt.Errorf("task context repo required")
	}
	if record.Title == "" {
		return fmt.Errorf("task context title required")
	}
	if record.State == "" {
		record.State = "active"
	}
	if record.Priority == "" {
		record.Priority = "medium"
	}
	const q = `
		INSERT INTO task_contexts (
			id, repo_id, title, goal, next_step, state, priority,
			parent_task_id, preferred_worktree_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			repo_id = excluded.repo_id,
			title = excluded.title,
			goal = excluded.goal,
			next_step = excluded.next_step,
			state = excluded.state,
			priority = excluded.priority,
			parent_task_id = excluded.parent_task_id,
			preferred_worktree_id = excluded.preferred_worktree_id,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
		record.ID,
		record.RepoID,
		record.Title,
		nullIfEmpty(record.Goal),
		nullIfEmpty(record.NextStep),
		record.State,
		record.Priority,
		record.ParentTaskID,
		nullIfEmpty(record.PreferredWorktreeID),
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

func (s *Store) GetTaskContext(id string) (*TaskContextRecord, error) {
	const q = `
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, created_at, updated_at
		FROM task_contexts WHERE id = ?
	`
	row := s.db.QueryRow(q, id)
	record, err := scanTaskContext(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return record, err
}

func (s *Store) ListTaskContexts(repoID string) ([]TaskContextRecord, error) {
	const base = `
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, created_at, updated_at
		FROM task_contexts
	`
	q := base
	args := []any{}
	if repoID != "" {
		q += ` WHERE repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY updated_at DESC, created_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TaskContextRecord
	for rows.Next() {
		record, err := scanTaskContextRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func scanTaskContext(row *sql.Row) (*TaskContextRecord, error) {
	var record TaskContextRecord
	var goal sql.NullString
	var nextStep sql.NullString
	var parentTaskID sql.NullString
	var preferredWorktreeID sql.NullString
	if err := row.Scan(
		&record.ID,
		&record.RepoID,
		&record.Title,
		&goal,
		&nextStep,
		&record.State,
		&record.Priority,
		&parentTaskID,
		&preferredWorktreeID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if goal.Valid {
		record.Goal = goal.String
	}
	if nextStep.Valid {
		record.NextStep = nextStep.String
	}
	if parentTaskID.Valid {
		record.ParentTaskID = &parentTaskID.String
	}
	if preferredWorktreeID.Valid {
		record.PreferredWorktreeID = preferredWorktreeID.String
	}
	return &record, nil
}

func scanTaskContextRows(rows *sql.Rows) (*TaskContextRecord, error) {
	var record TaskContextRecord
	var goal sql.NullString
	var nextStep sql.NullString
	var parentTaskID sql.NullString
	var preferredWorktreeID sql.NullString
	if err := rows.Scan(
		&record.ID,
		&record.RepoID,
		&record.Title,
		&goal,
		&nextStep,
		&record.State,
		&record.Priority,
		&parentTaskID,
		&preferredWorktreeID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if goal.Valid {
		record.Goal = goal.String
	}
	if nextStep.Valid {
		record.NextStep = nextStep.String
	}
	if parentTaskID.Valid {
		record.ParentTaskID = &parentTaskID.String
	}
	if preferredWorktreeID.Valid {
		record.PreferredWorktreeID = preferredWorktreeID.String
	}
	return &record, nil
}
