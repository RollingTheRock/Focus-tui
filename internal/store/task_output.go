package store

import (
	"fmt"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/events"
	"github.com/RollingTheRock/Focus-tui/internal/models"
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

	q := fmt.Sprintf(`
		INSERT INTO %s (id, task_id, content, actor, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			content = excluded.content,
			actor = excluded.actor,
			updated_at = CURRENT_TIMESTAMP
	`, s.tbl("task_outputs", "proj_task_outputs"))
	_, err := s.exec(
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
	base := fmt.Sprintf(`
		SELECT id, task_id, content, COALESCE(actor, ''), created_at, updated_at
		FROM %s
	`, s.tbl("task_outputs", "proj_task_outputs"))
	q := base
	args := []any{}
	if taskID != "" {
		q += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.qRows(q, args...)
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
