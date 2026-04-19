package store

import (
	"database/sql"
	"fmt"
	"time"

	"focus/internal/models"
)

type AgentSessionRecord = models.AgentSessionRecord

func (s *Store) SaveAgentSession(record AgentSessionRecord) error {
	if record.ID == "" {
		return fmt.Errorf("agent session id required")
	}
	if record.WorktreeID == "" {
		return fmt.Errorf("agent session worktree required")
	}
	if record.Provider == "" {
		return fmt.Errorf("agent session provider required")
	}
	if record.State == "" {
		record.State = "unknown"
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = time.Now()
	}
	const q = `
		INSERT INTO agent_sessions (
			id, provider, worktree_id, repo_id, task_id, plan_id, step_id, branch_snapshot,
			pid, state, launch_source, summary, started_at, ended_at, last_activity_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			worktree_id = excluded.worktree_id,
			repo_id = excluded.repo_id,
			task_id = excluded.task_id,
			plan_id = excluded.plan_id,
			step_id = excluded.step_id,
			branch_snapshot = excluded.branch_snapshot,
			pid = excluded.pid,
			state = excluded.state,
			launch_source = excluded.launch_source,
			summary = excluded.summary,
			started_at = excluded.started_at,
			ended_at = excluded.ended_at,
			last_activity_at = excluded.last_activity_at,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(q,
		record.ID,
		record.Provider,
		record.WorktreeID,
		record.RepoID,
		nullIfEmpty(record.TaskID),
		nullIfEmpty(record.PlanID),
		nullIfEmpty(record.StepID),
		record.BranchSnapshot,
		record.PID,
		record.State,
		nullIfEmpty(record.LaunchSource),
		nullIfEmpty(record.Summary),
		record.StartedAt,
		record.EndedAt,
		nullableTimePtr(record.LastActivityAt),
	)
	return err
}

func (s *Store) ListAgentSessions(worktreeID string) ([]AgentSessionRecord, error) {
	const base = `
		SELECT id, provider, worktree_id, repo_id, task_id, plan_id, step_id, branch_snapshot,
		       pid, state, launch_source, summary, started_at, ended_at, last_activity_at, updated_at
		FROM agent_sessions
	`
	q := base
	args := []any{}
	if worktreeID != "" {
		q += ` WHERE worktree_id = ?`
		args = append(args, worktreeID)
	}
	q += ` ORDER BY updated_at DESC, started_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AgentSessionRecord
	for rows.Next() {
		var record AgentSessionRecord
		var taskID, planID, stepID sql.NullString
		var endedAt sql.NullTime
		var lastActivityAt sql.NullTime
		var launchSource sql.NullString
		var summary sql.NullString
		if err := rows.Scan(
			&record.ID,
			&record.Provider,
			&record.WorktreeID,
			&record.RepoID,
			&taskID,
			&planID,
			&stepID,
			&record.BranchSnapshot,
			&record.PID,
			&record.State,
			&launchSource,
			&summary,
			&record.StartedAt,
			&endedAt,
			&lastActivityAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if endedAt.Valid {
			t := endedAt.Time
			record.EndedAt = &t
		}
		if lastActivityAt.Valid {
			t := lastActivityAt.Time
			record.LastActivityAt = &t
		}
		if launchSource.Valid {
			record.LaunchSource = launchSource.String
		}
		if taskID.Valid {
			record.TaskID = taskID.String
		}
		if planID.Valid {
			record.PlanID = planID.String
		}
		if stepID.Valid {
			record.StepID = stepID.String
		}
		if summary.Valid {
			record.Summary = summary.String
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
