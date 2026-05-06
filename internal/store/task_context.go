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

	// Phase 1: dual-write to Event Store (best-effort).
	if s.events != nil {
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

	_, err := s.events.AppendEvent(ctx, events.Event{
		OccurredAt:    time.Now(),
		AggregateType: events.AggregateTask,
		AggregateID:   record.ID,
		EventType:     evType,
		Payload:       payload,
		ActorType:     events.ActorSystem,
		ScopeType:     events.AggregateTask,
		ScopeID:       record.ID,
	})
	if err != nil {
		log.Printf("[event-store] append task event failed (non-critical): %v", err)
	}
}

func (s *Store) GetTaskContext(id string) (*TaskContextRecord, error) {
	q := fmt.Sprintf(`
		SELECT id, repo_id, title, goal, next_step, state, priority,
		       parent_task_id, preferred_worktree_id, created_at, updated_at
		FROM %s WHERE id = ?
	`, s.tbl("task_contexts", "proj_tasks"))
	record, err := scanTaskContext(s.qRow(q, id))
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
	`, s.tbl("task_contexts", "proj_tasks"))
	q := base
	args := []any{}
	if repoID != "" {
		q += ` WHERE repo_id = ?`
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
