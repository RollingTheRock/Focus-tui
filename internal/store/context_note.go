package store

import (
	"database/sql"
	"fmt"

	"focus/internal/models"
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
	const q = `
		INSERT INTO context_notes (
			id, task_id, worktree_id, note_type, body, pinned, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			worktree_id = excluded.worktree_id,
			note_type = excluded.note_type,
			body = excluded.body,
			pinned = excluded.pinned,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
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
	q := `
		SELECT id, task_id, worktree_id, note_type, body, pinned, created_at, updated_at
		FROM context_notes
		WHERE 1 = 1
	`
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

	rows, err := s.db.Query(q, args...)
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
	_, err := s.db.Exec(`DELETE FROM context_notes WHERE worktree_id = ?`, worktreeID)
	return err
}
