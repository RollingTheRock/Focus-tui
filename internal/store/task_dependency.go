package store

import (
	"database/sql"
	"fmt"

	"focus/internal/models"
)

type TaskDependencyRecord = models.TaskDependencyRecord

func (s *Store) SaveTaskDependency(record TaskDependencyRecord) error {
	if record.FromTaskID == "" || record.ToTaskID == "" {
		return fmt.Errorf("task dependency endpoints required")
	}
	if record.DependencyType == "" {
		record.DependencyType = "hard"
	}
	const q = `
		INSERT INTO task_dependencies (from_task_id, to_task_id, dependency_type, created_at)
		VALUES (?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
		ON CONFLICT(from_task_id, to_task_id) DO UPDATE SET
			dependency_type = excluded.dependency_type
	`
	_, err := s.db.Exec(q, record.FromTaskID, record.ToTaskID, record.DependencyType, nullableTimeValue(record.CreatedAt))
	return err
}

func (s *Store) DeleteTaskDependency(fromTaskID, toTaskID string) error {
	if fromTaskID == "" || toTaskID == "" {
		return fmt.Errorf("task dependency endpoints required")
	}
	_, err := s.db.Exec(`DELETE FROM task_dependencies WHERE from_task_id = ? AND to_task_id = ?`, fromTaskID, toTaskID)
	return err
}

func (s *Store) ListDownstreamTaskContexts(taskID string) ([]TaskContextRecord, error) {
	if taskID == "" {
		return nil, nil
	}
	const q = `
		SELECT tc.id, tc.repo_id, tc.title, tc.goal, tc.next_step, tc.state, tc.priority,
		       tc.parent_task_id, tc.preferred_worktree_id, tc.created_at, tc.updated_at
		FROM task_dependencies d
		JOIN task_contexts tc ON tc.id = d.to_task_id
		WHERE d.from_task_id = ?
		ORDER BY tc.updated_at DESC, tc.created_at DESC
	`
	rows, err := s.db.Query(q, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]TaskContextRecord, 0)
	for rows.Next() {
		record, err := scanTaskContextRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (s *Store) AreTaskPrerequisitesMet(taskID string) (bool, error) {
	if taskID == "" {
		return false, fmt.Errorf("task id required")
	}
	const q = `
		SELECT COUNT(1)
		FROM task_dependencies d
		JOIN task_contexts pre ON pre.id = d.from_task_id
		WHERE d.to_task_id = ?
		  AND d.dependency_type = 'hard'
		  AND pre.state NOT IN ('done', 'completed', 'archived')
	`
	var missing int
	if err := s.db.QueryRow(q, taskID).Scan(&missing); err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, err
	}
	return missing == 0, nil
}
