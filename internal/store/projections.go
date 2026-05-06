package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrateProjectionSchema creates projection tables that mirror the current
// SQLite schema but with an additional event_version column for rebuild tracking.
func MigrateProjectionSchema(ctx context.Context, pool *pgxpool.Pool) error {
	schema := `
CREATE TABLE IF NOT EXISTS proj_tasks (
    id                    TEXT PRIMARY KEY,
    repo_id               TEXT NOT NULL,
    title                 TEXT NOT NULL,
    goal                  TEXT,
    next_step             TEXT,
    state                 TEXT NOT NULL,
    priority              TEXT NOT NULL DEFAULT 'medium',
    parent_task_id        TEXT,
    preferred_worktree_id TEXT,
    event_version         BIGINT NOT NULL DEFAULT 0,
    updated_at            TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_tasks_repo_state
    ON proj_tasks(repo_id, state, updated_at DESC);

CREATE TABLE IF NOT EXISTS proj_worktree_contexts (
    worktree_id      TEXT PRIMARY KEY,
    repo_id          TEXT NOT NULL,
    primary_task_id  TEXT,
    task_mode        TEXT NOT NULL DEFAULT 'single',
    task_name        TEXT,
    branch_snapshot  TEXT,
    last_active_at   TIMESTAMPTZ NOT NULL,
    last_opened_at   TIMESTAMPTZ,
    last_agent_at    TIMESTAMPTZ,
    event_version    BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_worktree_repo_active
    ON proj_worktree_contexts(repo_id, last_active_at DESC);

CREATE TABLE IF NOT EXISTS proj_agent_sessions (
    id               TEXT PRIMARY KEY,
    provider         TEXT NOT NULL,
    worktree_id      TEXT NOT NULL,
    repo_id          TEXT,
    task_id          TEXT,
    plan_id          TEXT,
    step_id          TEXT,
    branch_snapshot  TEXT,
    pid              INTEGER,
    state            TEXT NOT NULL,
    launch_source    TEXT,
    summary          TEXT,
    env_snapshot     TEXT,
    started_at       TIMESTAMPTZ NOT NULL,
    ended_at         TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,
    last_heartbeat   TIMESTAMPTZ,
    stop_reason      TEXT,
    event_version    BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_agent_sessions_worktree
    ON proj_agent_sessions(worktree_id);
CREATE INDEX IF NOT EXISTS idx_proj_agent_sessions_state
    ON proj_agent_sessions(state);

CREATE TABLE IF NOT EXISTS proj_task_plans (
    id            TEXT PRIMARY KEY,
    task_id       TEXT,
    title         TEXT NOT NULL,
    why_now       TEXT,
    success       TEXT,
    out_of_scope  TEXT,
    known_risks   TEXT,
    status        TEXT NOT NULL,
    current_step  TEXT,
    plan_body     TEXT,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW(),
    archived_at   TIMESTAMPTZ,
    done_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_proj_task_plans_task_updated
    ON proj_task_plans(task_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS proj_plan_steps (
    id               TEXT PRIMARY KEY,
    plan_id          TEXT NOT NULL,
    order_index      INTEGER NOT NULL,
    title            TEXT NOT NULL,
    state            TEXT NOT NULL,
    expanded_task_id TEXT,
    notes            TEXT,
    event_version    BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(plan_id, order_index)
);

CREATE INDEX IF NOT EXISTS idx_proj_plan_steps_plan_order
    ON proj_plan_steps(plan_id, order_index ASC);

CREATE TABLE IF NOT EXISTS proj_context_notes (
    id           TEXT PRIMARY KEY,
    task_id      TEXT,
    worktree_id  TEXT,
    note_type    TEXT NOT NULL,
    body         TEXT NOT NULL,
    pinned       BOOLEAN NOT NULL DEFAULT FALSE,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_context_notes_task
    ON proj_context_notes(task_id, pinned DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS proj_session_handoffs (
    id                  TEXT PRIMARY KEY,
    task_id             TEXT NOT NULL,
    plan_id             TEXT,
    session_id          TEXT,
    done_summary        TEXT,
    remaining_summary   TEXT,
    decision_summary    TEXT,
    uncertainty_summary TEXT,
    blocker_summary     TEXT,
    entrypoint          TEXT,
    event_version       BIGINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_session_handoffs_task
    ON proj_session_handoffs(task_id, created_at DESC);

CREATE TABLE IF NOT EXISTS proj_knowledge_facts (
    id           TEXT PRIMARY KEY,
    plan_id      TEXT,
    subject      TEXT NOT NULL,
    predicate    TEXT NOT NULL,
    object       TEXT NOT NULL,
    source       TEXT NOT NULL,
    confidence   REAL NOT NULL DEFAULT 1.0,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_knowledge_facts_plan
    ON proj_knowledge_facts(plan_id, created_at DESC);

CREATE TABLE IF NOT EXISTS proj_task_outputs (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL,
    content      TEXT NOT NULL,
    actor        TEXT,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_task_outputs_task
    ON proj_task_outputs(task_id, created_at DESC);

CREATE TABLE IF NOT EXISTS proj_task_dependencies (
    from_task_id     TEXT NOT NULL,
    to_task_id       TEXT NOT NULL,
    dependency_type  TEXT NOT NULL DEFAULT 'hard',
    event_version    BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (from_task_id, to_task_id)
);

CREATE INDEX IF NOT EXISTS idx_proj_task_deps_from
    ON proj_task_dependencies(from_task_id);
CREATE INDEX IF NOT EXISTS idx_proj_task_deps_to
    ON proj_task_dependencies(to_task_id);

CREATE TABLE IF NOT EXISTS proj_task_worktree_links (
    id            TEXT PRIMARY KEY,
    task_id       TEXT NOT NULL,
    worktree_id   TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(task_id, worktree_id, relation_type)
);

CREATE INDEX IF NOT EXISTS idx_proj_task_wt_links_task
    ON proj_task_worktree_links(task_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS proj_worktree_history (
    id               TEXT PRIMARY KEY,
    repo_id          TEXT NOT NULL,
    branch           TEXT,
    path             TEXT,
    created_at       TIMESTAMPTZ,
    removed_at       TIMESTAMPTZ DEFAULT NOW(),
    task_id          TEXT,
    plan_id          TEXT,
    provider         TEXT,
    summary          TEXT,
    duration_minutes INTEGER,
    event_version    BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_proj_worktree_history_repo
    ON proj_worktree_history(repo_id, removed_at DESC);

CREATE TABLE IF NOT EXISTS proj_agent_messages (
    id          TEXT PRIMARY KEY,
    from_agent  TEXT NOT NULL,
    to_agent    TEXT NOT NULL,
    msg_type    TEXT NOT NULL,
    payload     TEXT NOT NULL,
    read_at     TIMESTAMPTZ,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_agent_msg_to
    ON proj_agent_messages(to_agent, created_at DESC);

CREATE TABLE IF NOT EXISTS proj_todos (
    id          SERIAL PRIMARY KEY,
    text        TEXT NOT NULL,
    status      TEXT DEFAULT 'todo',
    list        TEXT DEFAULT 'today',
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS proj_pomodoro_sessions (
    id             SERIAL PRIMARY KEY,
    date           DATE NOT NULL,
    start_time     TIMESTAMPTZ NOT NULL,
    end_time       TIMESTAMPTZ,
    linked_todo_id INTEGER,
    status         TEXT,
    event_version  BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS proj_streaks (
    date         DATE PRIMARY KEY,
    has_pomodoro BOOLEAN DEFAULT FALSE,
    event_version BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS proj_page_snapshots (
    worktree_id   TEXT PRIMARY KEY,
    snapshot_json TEXT NOT NULL,
    event_version BIGINT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS proj_adrs (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    status        TEXT NOT NULL,
    version       INTEGER NOT NULL DEFAULT 1,
    context       TEXT NOT NULL,
    decision      TEXT NOT NULL,
    consequences  TEXT,
    superseded_by TEXT,
    created_by    TEXT NOT NULL,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    accepted_at   TIMESTAMPTZ,
    accepted_by   TEXT,
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_adrs_status
    ON proj_adrs(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS proj_adr_constraints (
    id          TEXT PRIMARY KEY,
    adr_id      TEXT NOT NULL,
    category    TEXT NOT NULL,
    rule        TEXT NOT NULL,
    rationale   TEXT,
    event_version BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proj_adr_constraints_adr
    ON proj_adr_constraints(adr_id);

CREATE TABLE IF NOT EXISTS proj_task_briefs (
    task_id           TEXT PRIMARY KEY,
    why_now           TEXT,
    success_criteria  TEXT,
    out_of_scope      TEXT,
    known_risks       TEXT,
    event_version     BIGINT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);
`
	_, err := pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("migrate projection schema: %w", err)
	}
	return nil
}
