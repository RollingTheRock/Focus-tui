package store

import (
	"database/sql"
	"fmt"
	"time"

	"focus/internal/events"
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
	q := fmt.Sprintf(`
		INSERT INTO %s (
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
	`, s.tbl("worktree_history", "proj_worktree_history"))
	_, err := s.exec(q,
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
	if err != nil {
		return err
	}

	var taskID, planID string
	if record.TaskID != nil {
		taskID = *record.TaskID
	}
	if record.PlanID != nil {
		planID = *record.PlanID
	}
	s.tryAppendEvent(events.AggregateWorktree, record.ID, events.WorktreeHistoryRecorded,
		events.WorktreeHistoryRecordedPayload{
			ID:              record.ID,
			RepoID:          record.RepoID,
			Branch:          record.Branch,
			Path:            record.Path,
			CreatedAt:       record.CreatedAt.Format(time.RFC3339),
			RemovedAt:       record.RemovedAt.Format(time.RFC3339),
			TaskID:          taskID,
			PlanID:          planID,
			Provider:        record.Provider,
			Summary:         record.Summary,
			DurationMinutes: record.DurationMinutes,
		}, events.AggregateWorktree, record.ID)

	return nil
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
	_, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, s.tbl("worktree_history", "proj_worktree_history")), id)
	return err
}
