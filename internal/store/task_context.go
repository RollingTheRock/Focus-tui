package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"focus/internal/events"
	"focus/internal/models"
)

type TaskContextRecord = models.TaskContextRecord

func (s *Store) SaveTaskContext(record TaskContextRecord) error {
	if record.ID == "" {
		return fmt.Errorf("task context id required")
	}
	if record.RepoID == "" {
		return fmt.Errorf("task context repo required")
	}
	if record.Title == "" {
		return fmt.Errorf("task context title required")
	}
	if record.State == "" {
		record.State = "active"
	}
	if record.Priority == "" {
		record.Priority = "medium"
	}

	// Phase 1: dual-write to Event Store (best-effort), or publish directly
	// to the in-memory bus in SQLite mode.
	if s.events != nil || s.bus != nil {
		s.tryAppendTaskEvent(record)
	}

	q := fmt.Sprintf(`
		INSERT INTO %s (
			id, repo_id, title, goal, next_step, state, priority,
			parent_task_id, preferred_worktree_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			repo_id = excluded.repo_id,
			title = excluded.title,
			goal = excluded.goal,
			next_step = excluded.next_step,
			state = excluded.state,
			priority = excluded.priority,
			parent_task_id = excluded.parent_task_id,
			preferred_worktree_id = excluded.preferred_worktree_id,
			deleted_at = NULL,
			updated_at = CURRENT_TIMESTAMP
	`, s.tbl("task_contexts", "proj_tasks"))
	_, err := s.exec(q,
		record.ID,
		record.RepoID,
		record.Title,
		nullIfEmpty(record.Goal),
		nullIfEmpty(record.NextStep),
		record.State,
		record.Priority,
		record.ParentTaskID,
		nullIfEmpty(record.PreferredWorktreeID),
		nullableTimeValue(record.CreatedAt),
	)
	return err
}

// tryAppendTaskEvent attempts to write an event to the Event Store.
// Failures are logged but never block the SQLite write.
func (s *Store) tryAppendTaskEvent(record TaskContextRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	old, _ := s.GetTaskContext(record.ID)

	var evType string
	var payload []byte

	if old == nil {
		evType = events.TaskCreated
		p := events.TaskCreatedPayload{
			RepoID:       record.RepoID,
			Title:        record.Title,
			Goal:         record.Goal,
			Priority:     record.Priority,
			ParentTaskID: record.ParentTaskID,
		}
		payload, _ = events.Serialize(p)
	} else if old.State != record.State {
		evType = events.TaskStateChanged
		p := events.TaskStateChangedPayload{
			PreviousState: old.State,
			NewState:      record.State,
		}
		payload, _ = events.Serialize(p)
	} else if old.Goal != record.Goal || old.NextStep != record.NextStep {
		evType = events.TaskGoalUpdated
		p := events.TaskGoalUpdatedPayload{
			Goal:     record.Goal,
			NextStep: record.NextStep,
		}
		payload, _ = events.Serialize(p)
	} else {
		// No meaningful change; skip event emission.
		return
	}

	ev := events.Event{
		OccurredAt:    time.Now(),
		AggregateType: events.AggregateTask,
		AggregateID:   record.ID,
		EventType:     evType,
		Payload:       payload,
		ActorType:     events.ActorSystem,
		ScopeType:     events.AggregateTask,
		ScopeID:       record.ID,
	}

	if s.events != nil {
		_, err := s.events.AppendEvent(ctx, ev)
		if err != nil {
			log.Printf("[event-store] append task event failed (non-critical): %v", err)
		}
	} else if s.bus != nil {
		s.bus.Publish(ev)
	}
}

func (s *Store) GetTaskContext(id string) (*TaskContextRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, created_at, updated_at
		FROM %s WHERE id = ? AND deleted_at IS NULL AND state != 'archived'
	`, s.tbl("task_contexts", "proj_tasks"))
	record, err := scanTaskContext(s.qRow(q, id))
	if isNoRows(err) {
		return nil, nil
	}
	return record, err
}

func (s *Store) GetTaskContextIncludeDeleted(id string) (*TaskContextRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, deleted_at, created_at, updated_at
		FROM %s WHERE id = ?
	`, s.tbl("task_contexts", "proj_tasks"))
	record, err := scanTaskContextWithDeleted(s.qRow(q, id))
	if isNoRows(err) {
		return nil, nil
	}
	return record, err
}

func (s *Store) ListTaskContexts(repoID string) ([]TaskContextRecord, error) {
	base := fmt.Sprintf(`
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, created_at, updated_at
		FROM %s
		WHERE deleted_at IS NULL AND state != 'archived'
	`, s.tbl("task_contexts", "proj_tasks"))
	q := base
	args := []any{}
	if repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY updated_at DESC, created_at DESC`
	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TaskContextRecord
	for rows.Next() {
		record, err := scanTaskContextRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (s *Store) ListArchivedTaskContexts(repoID string) ([]TaskContextRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, created_at, updated_at
		FROM %s
		WHERE deleted_at IS NULL AND state = 'archived'
	`, s.tbl("task_contexts", "proj_tasks"))
	args := []any{}
	if repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY updated_at DESC`
	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TaskContextRecord
	for rows.Next() {
		record, err := scanTaskContextRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (s *Store) ListDeletedTaskContexts(repoID string) ([]TaskContextRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, deleted_at, created_at, updated_at
		FROM %s
		WHERE deleted_at IS NOT NULL
	`, s.tbl("task_contexts", "proj_tasks"))
	args := []any{}
	if repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY deleted_at DESC`
	rows, err := s.qRows(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TaskContextRecord
	for rows.Next() {
		record, err := scanTaskContextRowsWithDeleted(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func scanTaskContext(row rowScanner) (*TaskContextRecord, error) {
	var record TaskContextRecord
	var goal sql.NullString
	var nextStep sql.NullString
	var parentTaskID sql.NullString
	var preferredWorktreeID sql.NullString
	if err := row.Scan(
		&record.ID,
		&record.RepoID,
		&record.Title,
		&goal,
		&nextStep,
		&record.State,
		&record.Priority,
		&parentTaskID,
		&preferredWorktreeID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if goal.Valid {
		record.Goal = goal.String
	}
	if nextStep.Valid {
		record.NextStep = nextStep.String
	}
	if parentTaskID.Valid {
		record.ParentTaskID = &parentTaskID.String
	}
	if preferredWorktreeID.Valid {
		record.PreferredWorktreeID = preferredWorktreeID.String
	}
	return &record, nil
}

func scanTaskContextWithDeleted(row rowScanner) (*TaskContextRecord, error) {
	var record TaskContextRecord
	var goal sql.NullString
	var nextStep sql.NullString
	var parentTaskID sql.NullString
	var preferredWorktreeID sql.NullString
	var deletedAt sql.NullTime
	if err := row.Scan(
		&record.ID,
		&record.RepoID,
		&record.Title,
		&goal,
		&nextStep,
		&record.State,
		&record.Priority,
		&parentTaskID,
		&preferredWorktreeID,
		&deletedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if goal.Valid {
		record.Goal = goal.String
	}
	if nextStep.Valid {
		record.NextStep = nextStep.String
	}
	if parentTaskID.Valid {
		record.ParentTaskID = &parentTaskID.String
	}
	if preferredWorktreeID.Valid {
		record.PreferredWorktreeID = preferredWorktreeID.String
	}
	if deletedAt.Valid {
		record.DeletedAt = &deletedAt.Time
	}
	return &record, nil
}

func (s *Store) DeleteTaskContext(id string) error {
	if id == "" {
		return fmt.Errorf("task context id required")
	}

	// Recursively soft-delete child tasks first.
	q := fmt.Sprintf(`SELECT id FROM %s WHERE parent_task_id = ? AND deleted_at IS NULL`, s.tbl("task_contexts", "proj_tasks"))
	rows, err := s.qRows(q, id)
	if err != nil {
		return err
	}
	var children []string
	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			rows.Close()
			return err
		}
		children = append(children, childID)
	}
	rows.Close()

	for _, childID := range children {
		if err := s.DeleteTaskContext(childID); err != nil {
			return err
		}
	}

	// Soft-delete self.
	dq := fmt.Sprintf(`UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, s.tbl("task_contexts", "proj_tasks"))
	_, err = s.exec(dq, id)
	return err
}

func (s *Store) RestoreTaskContext(id string) error {
	if id == "" {
		return fmt.Errorf("task context id required")
	}
	q := fmt.Sprintf(`UPDATE %s SET deleted_at = NULL, state = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, s.tbl("task_contexts", "proj_tasks"))
	_, err := s.exec(q, id)
	return err
}

func (s *Store) ArchiveTaskContext(id string) error {
	if id == "" {
		return fmt.Errorf("task context id required")
	}
	q := fmt.Sprintf(`UPDATE %s SET state = 'archived', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, s.tbl("task_contexts", "proj_tasks"))
	_, err := s.exec(q, id)
	return err
}

func scanTaskContextRows(rows rowIter) (*TaskContextRecord, error) {
	var record TaskContextRecord
	var goal sql.NullString
	var nextStep sql.NullString
	var parentTaskID sql.NullString
	var preferredWorktreeID sql.NullString
	if err := rows.Scan(
		&record.ID,
		&record.RepoID,
		&record.Title,
		&goal,
		&nextStep,
		&record.State,
		&record.Priority,
		&parentTaskID,
		&preferredWorktreeID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if goal.Valid {
		record.Goal = goal.String
	}
	if nextStep.Valid {
		record.NextStep = nextStep.String
	}
	if parentTaskID.Valid {
		record.ParentTaskID = &parentTaskID.String
	}
	if preferredWorktreeID.Valid {
		record.PreferredWorktreeID = preferredWorktreeID.String
	}
	return &record, nil
}

func scanTaskContextRowsWithDeleted(rows rowIter) (*TaskContextRecord, error) {
	var record TaskContextRecord
	var goal sql.NullString
	var nextStep sql.NullString
	var parentTaskID sql.NullString
	var preferredWorktreeID sql.NullString
	var deletedAt sql.NullTime
	if err := rows.Scan(
		&record.ID,
		&record.RepoID,
		&record.Title,
		&goal,
		&nextStep,
		&record.State,
		&record.Priority,
		&parentTaskID,
		&preferredWorktreeID,
		&deletedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if goal.Valid {
		record.Goal = goal.String
	}
	if nextStep.Valid {
		record.NextStep = nextStep.String
	}
	if parentTaskID.Valid {
		record.ParentTaskID = &parentTaskID.String
	}
	if preferredWorktreeID.Valid {
		record.PreferredWorktreeID = preferredWorktreeID.String
	}
	if deletedAt.Valid {
		record.DeletedAt = &deletedAt.Time
	}
	return &record, nil
}
