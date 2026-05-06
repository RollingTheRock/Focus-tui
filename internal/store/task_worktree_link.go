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
	q := fmt.Sprintf(`
		INSERT INTO %s (
			id, task_id, worktree_id, relation_type, created_at, updated_at
		) VALUES (?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			worktree_id = excluded.worktree_id,
			relation_type = excluded.relation_type,
			updated_at = CURRENT_TIMESTAMP
	`, s.tbl("task_worktree_links", "proj_task_worktree_links"))
	_, err := s.exec(q,
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
	q := fmt.Sprintf(`
		SELECT id, task_id, worktree_id, relation_type, created_at, updated_at
		FROM %s `, s.tbl("task_worktree_links", "proj_task_worktree_links")) + where + `
		ORDER BY updated_at DESC, created_at DESC
	`
	rows, err := s.qRows(q, arg)
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
	_, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE worktree_id = ?`, s.tbl("task_worktree_links", "proj_task_worktree_links")), worktreeID)
	return err
}
