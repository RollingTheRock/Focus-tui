package store

import (
	"database/sql"
	"fmt"
	"time"

	"focus/internal/models"
)

type WorktreeHistoryRecord = models.WorktreeHistoryRecord

func (s *Store) SaveWorktreeHistory(record WorktreeHistoryRecord) error {
	if record.ID == "" {
		return fmt.Errorf("worktree history id required")
	}
	if record.RepoID == "" {
		return fmt.Errorf("worktree history repo required")
	}
	if record.RemovedAt.IsZero() {
		record.RemovedAt = time.Now()
	}
	const q = `
		INSERT INTO worktree_history (
			id, repo_id, branch, path, created_at, removed_at, task_id, plan_id, provider, summary, duration_minutes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			repo_id = excluded.repo_id,
			branch = excluded.branch,
			path = excluded.path,
			created_at = excluded.created_at,
			removed_at = excluded.removed_at,
			task_id = excluded.task_id,
			plan_id = excluded.plan_id,
			provider = excluded.provider,
			summary = excluded.summary,
			duration_minutes = excluded.duration_minutes
	`
	_, err := s.db.Exec(q,
		record.ID,
		record.RepoID,
		nullIfEmpty(record.Branch),
		nullIfEmpty(record.Path),
		nullableTimeValue(record.CreatedAt),
		record.RemovedAt,
		record.TaskID,
		record.PlanID,
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.Summary),
		record.DurationMinutes,
	)
	return err
}

func (s *Store) ListWorktreeHistory(repoID string) ([]WorktreeHistoryRecord, error) {
	base := fmt.Sprintf(`
		SELECT id, repo_id, branch, path, created_at, removed_at, task_id, plan_id, provider, summary, duration_minutes
		FROM %s
	`, s.tbl("worktree_history", "proj_worktree_history"))
	q := base
	args := []any{}
	if repoID != "" {
		q += ` WHERE repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY removed_at DESC`
	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []WorktreeHistoryRecord
	for rows.Next() {
		var record WorktreeHistoryRecord
		var branch, path, taskID, planID, provider, summary sql.NullString
		var createdAt sql.NullTime
		var durationMinutes sql.NullInt64
		if err := rows.Scan(
			&record.ID,
			&record.RepoID,
			&branch,
			&path,
			&createdAt,
			&record.RemovedAt,
			&taskID,
			&planID,
			&provider,
			&summary,
			&durationMinutes,
		); err != nil {
			return nil, err
		}
		if branch.Valid {
			record.Branch = branch.String
		}
		if path.Valid {
			record.Path = path.String
		}
		if createdAt.Valid {
			record.CreatedAt = createdAt.Time
		}
		if taskID.Valid {
			record.TaskID = &taskID.String
		}
		if planID.Valid {
			record.PlanID = &planID.String
		}
		if provider.Valid {
			record.Provider = provider.String
		}
		if summary.Valid {
			record.Summary = summary.String
		}
		if durationMinutes.Valid {
			record.DurationMinutes = int(durationMinutes.Int64)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) DeleteWorktreeHistory(id string) error {
	_, err := s.db.Exec(`DELETE FROM worktree_history WHERE id = ?`, id)
	return err
}
