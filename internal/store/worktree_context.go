package store

import (
	"database/sql"
	"fmt"

	"focus/internal/events"
	"focus/internal/models"
)

type WorktreeContextRecord = models.WorktreeContextRecord

func (s *Store) SaveWorktreeContext(record WorktreeContextRecord) error {
	if record.WorktreeID == "" {
		return fmt.Errorf("worktree context id required")
	}
	if record.RepoID == "" {
		return fmt.Errorf("worktree context repo required")
	}
	if record.TaskMode == "" {
		record.TaskMode = "single"
	}

	s.tryAppendEvent(events.AggregateWorktree, record.WorktreeID, events.WorktreeContextUpdated,
		events.WorktreeContextUpdatedPayload{
			RepoID:         record.RepoID,
			TaskMode:       record.TaskMode,
			TaskName:       record.TaskName,
			BranchSnapshot: record.BranchSnapshot,
			PrimaryTaskID:  record.PrimaryTaskID,
		}, events.AggregateWorktree, record.WorktreeID)

	q := fmt.Sprintf(`
		INSERT INTO %s (
			worktree_id, repo_id, primary_task_id, current_plan_id, task_mode, task_name,
			branch_snapshot, last_active_at, last_opened_at, last_agent_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(worktree_id) DO UPDATE SET
			repo_id = excluded.repo_id,
			primary_task_id = excluded.primary_task_id,
			current_plan_id = excluded.current_plan_id,
			task_mode = excluded.task_mode,
			task_name = excluded.task_name,
			branch_snapshot = excluded.branch_snapshot,
			last_active_at = excluded.last_active_at,
			last_opened_at = excluded.last_opened_at,
			last_agent_at = excluded.last_agent_at,
			updated_at = CURRENT_TIMESTAMP
	`, s.tbl("worktree_contexts", "proj_worktree_contexts"))
	_, err := s.exec(q,
		record.WorktreeID,
		record.RepoID,
		record.PrimaryTaskID,
		record.CurrentPlanID,
		record.TaskMode,
		nullIfEmpty(record.TaskName),
		nullIfEmpty(record.BranchSnapshot),
		nullableTimeValue(record.LastActiveAt),
		nullableTimePtr(record.LastOpenedAt),
		nullableTimePtr(record.LastAgentAt),
	)
	return err
}

func (s *Store) GetWorktreeContext(worktreeID string) (*WorktreeContextRecord, error) {
	q := fmt.Sprintf(`
		SELECT worktree_id, repo_id, primary_task_id, current_plan_id, task_mode, task_name,
		       branch_snapshot, last_active_at, last_opened_at, last_agent_at, updated_at
		FROM %s WHERE worktree_id = ?
	`, s.tbl("worktree_contexts", "proj_worktree_contexts"))
	record, err := scanWorktreeContext(s.qRow(q, worktreeID))
	if isNoRows(err) {
		return nil, nil
	}
	return record, err
}

func (s *Store) ListWorktreeContexts(repoID string) ([]WorktreeContextRecord, error) {
	base := fmt.Sprintf(`
		SELECT worktree_id, repo_id, primary_task_id, current_plan_id, task_mode, task_name,
		       branch_snapshot, last_active_at, last_opened_at, last_agent_at, updated_at
		FROM %s
	`, s.tbl("worktree_contexts", "proj_worktree_contexts"))
	q := base
	args := []any{}
	if repoID != "" {
		q += ` WHERE repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY last_active_at DESC, updated_at DESC`
	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []WorktreeContextRecord
	for rows.Next() {
		record, err := scanWorktreeContextRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (s *Store) DeleteWorktreeContext(worktreeID string) error {
	q := fmt.Sprintf(`DELETE FROM %s WHERE worktree_id = ?`, s.tbl("worktree_contexts", "proj_worktree_contexts"))
	_, err := s.exec(q, worktreeID)
	return err
}

func scanWorktreeContext(row rowScanner) (*WorktreeContextRecord, error) {
	var record WorktreeContextRecord
	var primaryTaskID sql.NullString
	var currentPlanID sql.NullString
	var taskName sql.NullString
	var branchSnapshot sql.NullString
	var lastOpenedAt sql.NullTime
	var lastAgentAt sql.NullTime
	if err := row.Scan(
		&record.WorktreeID,
		&record.RepoID,
		&primaryTaskID,
		&currentPlanID,
		&record.TaskMode,
		&taskName,
		&branchSnapshot,
		&record.LastActiveAt,
		&lastOpenedAt,
		&lastAgentAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if primaryTaskID.Valid {
		record.PrimaryTaskID = &primaryTaskID.String
	}
	if currentPlanID.Valid {
		record.CurrentPlanID = &currentPlanID.String
	}
	if taskName.Valid {
		record.TaskName = taskName.String
	}
	if branchSnapshot.Valid {
		record.BranchSnapshot = branchSnapshot.String
	}
	if lastOpenedAt.Valid {
		t := lastOpenedAt.Time
		record.LastOpenedAt = &t
	}
	if lastAgentAt.Valid {
		t := lastAgentAt.Time
		record.LastAgentAt = &t
	}
	return &record, nil
}

func scanWorktreeContextRows(rows rowIter) (*WorktreeContextRecord, error) {
	var record WorktreeContextRecord
	var primaryTaskID sql.NullString
	var currentPlanID sql.NullString
	var taskName sql.NullString
	var branchSnapshot sql.NullString
	var lastOpenedAt sql.NullTime
	var lastAgentAt sql.NullTime
	if err := rows.Scan(
		&record.WorktreeID,
		&record.RepoID,
		&primaryTaskID,
		&currentPlanID,
		&record.TaskMode,
		&taskName,
		&branchSnapshot,
		&record.LastActiveAt,
		&lastOpenedAt,
		&lastAgentAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if primaryTaskID.Valid {
		record.PrimaryTaskID = &primaryTaskID.String
	}
	if currentPlanID.Valid {
		record.CurrentPlanID = &currentPlanID.String
	}
	if taskName.Valid {
		record.TaskName = taskName.String
	}
	if branchSnapshot.Valid {
		record.BranchSnapshot = branchSnapshot.String
	}
	if lastOpenedAt.Valid {
		t := lastOpenedAt.Time
		record.LastOpenedAt = &t
	}
	if lastAgentAt.Valid {
		t := lastAgentAt.Time
		record.LastAgentAt = &t
	}
	return &record, nil
}
