package store

import (
	"database/sql"
	"fmt"

	"github.com/RollingTheRock/Focus-tui/internal/events"
	"github.com/RollingTheRock/Focus-tui/internal/models"
)

type ContextNoteRecord = models.ContextNoteRecord

func (s *Store) SaveContextNote(record ContextNoteRecord) error {
	if record.ID == "" {
		return fmt.Errorf("context note id required")
	}
	if record.TaskID == nil && record.WorktreeID == "" {
		return fmt.Errorf("context note task or worktree required")
	}
	if record.NoteType == "" {
		return fmt.Errorf("context note type required")
	}
	if record.Body == "" {
		return fmt.Errorf("context note body required")
	}

	scopeID := ""
	if record.TaskID != nil {
		scopeID = *record.TaskID
	} else {
		scopeID = record.WorktreeID
	}
	var taskID string
	if record.TaskID != nil {
		taskID = *record.TaskID
	}
	s.tryAppendEvent(events.AggregateContextNote, record.ID, events.ContextNoteAdded,
		events.ContextNoteAddedPayload{
			TaskID:     taskID,
			WorktreeID: record.WorktreeID,
			NoteType:   record.NoteType,
			Body:       record.Body,
			Pinned:     record.Pinned,
		}, events.AggregateContextNote, scopeID)

	q := fmt.Sprintf(`
		INSERT INTO %s (
			id, task_id, worktree_id, note_type, body, pinned, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			worktree_id = excluded.worktree_id,
			note_type = excluded.note_type,
			body = excluded.body,
			pinned = excluded.pinned,
			updated_at = CURRENT_TIMESTAMP
	`, s.tbl("context_notes", "proj_context_notes"))
	_, err := s.exec(q,
		record.ID,
		record.TaskID,
		nullIfEmpty(record.WorktreeID),
		record.NoteType,
		record.Body,
		record.Pinned,
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

func (s *Store) ListContextNotes(taskID string, worktreeID string) ([]ContextNoteRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, task_id, worktree_id, note_type, body, pinned, created_at, updated_at
		FROM %s
		WHERE 1 = 1
	`, s.tbl("context_notes", "proj_context_notes"))
	args := []any{}
	if taskID != "" {
		q += ` AND task_id = ?`
		args = append(args, taskID)
	}
	if worktreeID != "" {
		q += ` AND worktree_id = ?`
		args = append(args, worktreeID)
	}
	q += ` ORDER BY pinned DESC, updated_at DESC, created_at DESC`

	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ContextNoteRecord
	for rows.Next() {
		var record ContextNoteRecord
		var taskIDValue sql.NullString
		var worktreeIDValue sql.NullString
		if err := rows.Scan(
			&record.ID,
			&taskIDValue,
			&worktreeIDValue,
			&record.NoteType,
			&record.Body,
			&record.Pinned,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if taskIDValue.Valid {
			record.TaskID = &taskIDValue.String
		}
		if worktreeIDValue.Valid {
			record.WorktreeID = worktreeIDValue.String
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) DeleteContextNotesByWorktreeID(worktreeID string) error {
	_, err := s.exec(fmt.Sprintf(`DELETE FROM %s WHERE worktree_id = ?`, s.tbl("context_notes", "proj_context_notes")), worktreeID)
	return err
}
