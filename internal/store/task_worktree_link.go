package store

import (
	"fmt"

	"focus/internal/models"
)

type TaskWorktreeLinkRecord = models.TaskWorktreeLinkRecord

func (s *Store) SaveTaskWorktreeLink(record TaskWorktreeLinkRecord) error {
	if record.ID == "" {
		return fmt.Errorf("task worktree link id required")
	}
	if record.TaskID == "" {
		return fmt.Errorf("task worktree link task required")
	}
	if record.WorktreeID == "" {
		return fmt.Errorf("task worktree link worktree required")
	}
	if record.RelationType == "" {
		return fmt.Errorf("task worktree link relation required")
	}
	const q = `
		INSERT INTO task_worktree_links (
			id, task_id, worktree_id, relation_type, created_at, updated_at
		) VALUES (?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			worktree_id = excluded.worktree_id,
			relation_type = excluded.relation_type,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
		record.ID,
		record.TaskID,
		record.WorktreeID,
		record.RelationType,
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

func (s *Store) ListTaskWorktreeLinks(taskID string) ([]TaskWorktreeLinkRecord, error) {
	return s.listTaskWorktreeLinks(`WHERE task_id = ?`, taskID)
}

func (s *Store) ListWorktreeTaskLinks(worktreeID string) ([]TaskWorktreeLinkRecord, error) {
	return s.listTaskWorktreeLinks(`WHERE worktree_id = ?`, worktreeID)
}

func (s *Store) listTaskWorktreeLinks(where string, arg string) ([]TaskWorktreeLinkRecord, error) {
	q := `
		SELECT id, task_id, worktree_id, relation_type, created_at, updated_at
		FROM task_worktree_links ` + where + `
		ORDER BY updated_at DESC, created_at DESC
	`
	rows, err := s.db.Query(q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TaskWorktreeLinkRecord
	for rows.Next() {
		var record TaskWorktreeLinkRecord
		if err := rows.Scan(
			&record.ID,
			&record.TaskID,
			&record.WorktreeID,
			&record.RelationType,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) DeleteTaskWorktreeLinksByWorktreeID(worktreeID string) error {
	_, err := s.db.Exec(`DELETE FROM task_worktree_links WHERE worktree_id = ?`, worktreeID)
	return err
}
