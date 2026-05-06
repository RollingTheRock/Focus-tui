package store

import (
	"fmt"
	"time"

	"focus/internal/events"
	"focus/internal/models"
)

type TaskOutputRecord = models.TaskOutputRecord

func (s *Store) SaveTaskOutput(record TaskOutputRecord) error {
	if record.ID == "" {
		return fmt.Errorf("task output id required")
	}
	if record.TaskID == "" {
		return fmt.Errorf("task output task id required")
	}
	if record.Content == "" {
		return fmt.Errorf("task output content required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}

	s.tryAppendEvent(events.AggregateTask, record.TaskID, events.TaskOutputAdded,
		events.TaskOutputAddedPayload{
			TaskID:  record.TaskID,
			Content: record.Content,
			Actor:   record.Actor,
		}, events.AggregateTask, record.TaskID)

	const q = `
		INSERT INTO task_outputs (id, task_id, content, actor, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			content = excluded.content,
			actor = excluded.actor,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(
		q,
		record.ID,
		record.TaskID,
		record.Content,
		nullIfEmpty(record.Actor),
		record.CreatedAt,
	)
	return err
}

func (s *Store) ListTaskOutputs(taskID string) ([]TaskOutputRecord, error) {
	const base = `
		SELECT id, task_id, content, COALESCE(actor, ''), created_at, updated_at
		FROM task_outputs
	`
	q := base
	args := []any{}
	if taskID != "" {
		q += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TaskOutputRecord, 0)
	for rows.Next() {
		var record TaskOutputRecord
		if err := rows.Scan(
			&record.ID,
			&record.TaskID,
			&record.Content,
			&record.Actor,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
