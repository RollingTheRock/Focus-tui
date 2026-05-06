package store

import (
	"database/sql"
	"fmt"
	"time"

	"focus/internal/events"
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

	s.tryAppendEvent(events.AggregateAgentSession, record.ID, events.AgentSessionCreated,
		events.AgentSessionCreatedPayload{
			Provider:       record.Provider,
			WorktreeID:     record.WorktreeID,
			RepoID:         record.RepoID,
			TaskID:         record.TaskID,
			PlanID:         record.PlanID,
			StepID:         record.StepID,
			BranchSnapshot: record.BranchSnapshot,
			PID:            record.PID,
			LaunchSource:   record.LaunchSource,
			Summary:        record.Summary,
			EnvSnapshot:    record.EnvSnapshot,
		}, events.AggregateAgentSession, record.ID)

	const q = `
		INSERT INTO agent_sessions (
			id, provider, worktree_id, repo_id, task_id, plan_id, step_id, branch_snapshot,
			pid, state, launch_source, summary, env_snapshot,
			started_at, ended_at, last_activity_at, last_heartbeat, stop_reason, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
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
			env_snapshot = excluded.env_snapshot,
			started_at = excluded.started_at,
			ended_at = excluded.ended_at,
			last_activity_at = excluded.last_activity_at,
			last_heartbeat = excluded.last_heartbeat,
			stop_reason = excluded.stop_reason,
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
		nullIfEmpty(record.EnvSnapshot),
		record.StartedAt,
		record.EndedAt,
		nullableTimePtr(record.LastActivityAt),
		nullableTimePtr(record.LastHeartbeat),
		nullIfEmpty(record.StopReason),
	)
	return err
}

func (s *Store) ListAgentSessions(worktreeID string) ([]AgentSessionRecord, error) {
	const base = `
		SELECT id, provider, worktree_id, repo_id, task_id, plan_id, step_id, branch_snapshot,
		       pid, state, launch_source, summary, env_snapshot,
		       started_at, ended_at, last_activity_at, last_heartbeat, stop_reason, updated_at
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
		var lastHeartbeat sql.NullTime
		var launchSource sql.NullString
		var summary sql.NullString
		var envSnapshot sql.NullString
		var stopReason sql.NullString
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
			&envSnapshot,
			&record.StartedAt,
			&endedAt,
			&lastActivityAt,
			&lastHeartbeat,
			&stopReason,
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
		if lastHeartbeat.Valid {
			t := lastHeartbeat.Time
			record.LastHeartbeat = &t
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
		if envSnapshot.Valid {
			record.EnvSnapshot = envSnapshot.String
		}
		if stopReason.Valid {
			record.StopReason = stopReason.String
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) MarkAgentSessionDisconnected(sessionID string, reason string) error {
	if sessionID == "" {
		return fmt.Errorf("agent session id required")
	}
	now := time.Now()

	s.tryAppendEvent(events.AggregateAgentSession, sessionID, events.AgentSessionDisconnected,
		events.AgentSessionDisconnectedPayload{Reason: reason},
		events.AggregateAgentSession, sessionID)

	_, err := s.db.Exec(`
		UPDATE agent_sessions
		SET state = 'disconnected',
		    stop_reason = ?,
		    ended_at = COALESCE(ended_at, ?),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nullIfEmpty(reason), now, sessionID)
	return err
}

func (s *Store) DeleteAgentSessionsByWorktreeID(worktreeID string) error {
	_, err := s.db.Exec(`DELETE FROM agent_sessions WHERE worktree_id = ?`, worktreeID)
	return err
}

func (s *Store) UpdateAgentSessionHeartbeat(sessionID string, at time.Time, state string) error {
	if sessionID == "" {
		return fmt.Errorf("agent session id required")
	}
	if at.IsZero() {
		at = time.Now()
	}

	s.tryAppendEvent(events.AggregateAgentSession, sessionID, events.AgentSessionHeartbeat,
		events.AgentSessionHeartbeatPayload{State: state},
		events.AggregateAgentSession, sessionID)

	if state == "" {
		_, err := s.db.Exec(`
			UPDATE agent_sessions
			SET last_heartbeat = ?,
			    last_activity_at = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, at, at, sessionID)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE agent_sessions
		SET state = ?,
		    last_heartbeat = ?,
		    last_activity_at = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, state, at, at, sessionID)
	return err
}
