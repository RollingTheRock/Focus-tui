package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database connection.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database and runs migrations.
// dbPath is the full path to the database file; its parent directory
// will be created automatically.
func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DefaultDBPath returns ~/.local/share/focus/focus.db.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "focus", "focus.db"), nil
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS todos (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    text       TEXT    NOT NULL,
    status     TEXT    CHECK(status IN ('todo', 'done', 'overdue')) DEFAULT 'todo',
    list       TEXT    CHECK(list IN ('today', 'someday')) DEFAULT 'today',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pomodoro_sessions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    date           DATE     NOT NULL,
    start_time     DATETIME NOT NULL,
    end_time       DATETIME,
    linked_todo_id INTEGER  REFERENCES todos(id),
    status         TEXT     CHECK(status IN ('completed', 'cancelled'))
);

CREATE TABLE IF NOT EXISTS streaks (
    date          DATE    PRIMARY KEY,
    has_pomodoro  BOOLEAN DEFAULT 0
);

CREATE TABLE IF NOT EXISTS page_snapshots (
    worktree_id   TEXT    PRIMARY KEY,
    snapshot_json TEXT    NOT NULL,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS adrs (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    status        TEXT NOT NULL CHECK(status IN ('proposed', 'accepted', 'deprecated', 'superseded')),
    version       INTEGER NOT NULL DEFAULT 1,
    context       TEXT NOT NULL,
    decision      TEXT NOT NULL,
    consequences  TEXT,
    superseded_by TEXT REFERENCES adrs(id),
    created_by    TEXT NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accepted_at   DATETIME,
    accepted_by   TEXT,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_adrs_status
    ON adrs(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS adr_constraints (
    id          TEXT PRIMARY KEY,
    adr_id      TEXT NOT NULL REFERENCES adrs(id) ON DELETE CASCADE,
    category    TEXT NOT NULL CHECK(category IN ('must', 'must_not', 'should', 'should_not')),
    rule        TEXT NOT NULL,
    rationale   TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_adr_constraints_adr
    ON adr_constraints(adr_id);

CREATE TABLE IF NOT EXISTS task_contexts (
    id                    TEXT PRIMARY KEY,
    repo_id               TEXT NOT NULL,
    title                 TEXT NOT NULL,
    goal                  TEXT,
    next_step             TEXT,
    state                 TEXT NOT NULL CHECK(state IN ('active', 'paused', 'blocked', 'done', 'archived')),
    priority              TEXT NOT NULL DEFAULT 'medium' CHECK(priority IN ('low', 'medium', 'high')),
    parent_task_id        TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
    preferred_worktree_id TEXT,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_contexts_repo_state
    ON task_contexts(repo_id, state, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_contexts_preferred_worktree
    ON task_contexts(preferred_worktree_id);

CREATE TABLE IF NOT EXISTS task_dependencies (
    from_task_id    TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    to_task_id      TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    dependency_type TEXT NOT NULL DEFAULT 'hard' CHECK(dependency_type IN ('hard', 'soft')),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (from_task_id, to_task_id)
);

CREATE INDEX IF NOT EXISTS idx_task_dependencies_from
    ON task_dependencies(from_task_id);

CREATE INDEX IF NOT EXISTS idx_task_dependencies_to
    ON task_dependencies(to_task_id);

CREATE TABLE IF NOT EXISTS task_briefs (
    task_id            TEXT PRIMARY KEY REFERENCES task_contexts(id) ON DELETE CASCADE,
    why_now            TEXT,
    success_criteria   TEXT,
    out_of_scope       TEXT,
    known_risks        TEXT,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_briefs_updated
    ON task_briefs(updated_at DESC);

CREATE TABLE IF NOT EXISTS task_outputs (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    actor       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_outputs_task
    ON task_outputs(task_id, created_at DESC);

CREATE TABLE IF NOT EXISTS worktree_contexts (
    worktree_id      TEXT PRIMARY KEY,
    repo_id          TEXT NOT NULL,
    primary_task_id  TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
    task_mode        TEXT NOT NULL DEFAULT 'single' CHECK(task_mode IN ('single', 'mixed', 'staging')),
    task_name        TEXT,
    branch_snapshot  TEXT,
    last_active_at   DATETIME NOT NULL,
    last_opened_at   DATETIME,
    last_agent_at    DATETIME,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_worktree_contexts_repo_last_active
    ON worktree_contexts(repo_id, last_active_at DESC);

CREATE INDEX IF NOT EXISTS idx_worktree_contexts_primary_task
    ON worktree_contexts(primary_task_id);

CREATE TABLE IF NOT EXISTS task_worktree_links (
    id             TEXT PRIMARY KEY,
    task_id        TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    worktree_id    TEXT NOT NULL,
    relation_type  TEXT NOT NULL CHECK(relation_type IN ('primary', 'secondary', 'queued', 'historical')),
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(task_id, worktree_id, relation_type)
);

CREATE INDEX IF NOT EXISTS idx_task_worktree_links_task
    ON task_worktree_links(task_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_worktree_links_worktree
    ON task_worktree_links(worktree_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS task_plans (
    id            TEXT PRIMARY KEY,
    task_id       TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
    title         TEXT NOT NULL,
    why_now       TEXT,
    success       TEXT,
    out_of_scope  TEXT,
    known_risks   TEXT,
    status        TEXT NOT NULL CHECK(status IN ('draft', 'approved', 'active', 'blocked', 'completed', 'discarded', 'archived')),
    current_step  TEXT,
    plan_body     TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    archived_at   DATETIME,
    done_at       DATETIME
);

CREATE INDEX IF NOT EXISTS idx_task_plans_task_updated
    ON task_plans(task_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS plan_steps (
    id           TEXT PRIMARY KEY,
    plan_id      TEXT NOT NULL REFERENCES task_plans(id) ON DELETE CASCADE,
    order_index  INTEGER NOT NULL,
    title        TEXT NOT NULL,
    state        TEXT NOT NULL CHECK(state IN ('pending', 'in_progress', 'blocked', 'done', 'invalidated')),
    expanded_task_id TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
    notes        TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plan_id, order_index)
);

CREATE INDEX IF NOT EXISTS idx_plan_steps_plan_order
    ON plan_steps(plan_id, order_index ASC);

CREATE TABLE IF NOT EXISTS session_handoffs (
    id                   TEXT PRIMARY KEY,
    task_id              TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    plan_id              TEXT REFERENCES task_plans(id) ON DELETE SET NULL,
    session_id           TEXT,
    done_summary         TEXT,
    remaining_summary    TEXT,
    decision_summary     TEXT,
    uncertainty_summary  TEXT,
    blocker_summary      TEXT,
    entrypoint           TEXT,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_session_handoffs_task_created
    ON session_handoffs(task_id, created_at DESC);

CREATE TABLE IF NOT EXISTS context_notes (
    id          TEXT PRIMARY KEY,
    task_id     TEXT REFERENCES task_contexts(id) ON DELETE CASCADE,
    worktree_id TEXT,
    note_type   TEXT NOT NULL CHECK(note_type IN ('goal', 'next_step', 'blocker', 'insight', 'handoff')),
    body        TEXT NOT NULL,
    pinned      BOOLEAN NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_context_notes_task
    ON context_notes(task_id, pinned DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_context_notes_worktree
    ON context_notes(worktree_id, pinned DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS agent_sessions (
    id              TEXT PRIMARY KEY,
    provider        TEXT NOT NULL,
    worktree_id     TEXT NOT NULL,
    repo_id         TEXT,
    task_id         TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
    plan_id         TEXT REFERENCES task_plans(id) ON DELETE SET NULL,
    step_id         TEXT REFERENCES plan_steps(id) ON DELETE SET NULL,
    branch_snapshot TEXT,
    pid             INTEGER,
    state           TEXT NOT NULL,
    launch_source   TEXT,
    summary         TEXT,
    env_snapshot    TEXT,
    started_at      DATETIME NOT NULL,
    ended_at        DATETIME,
    last_activity_at DATETIME,
    last_heartbeat  DATETIME,
    stop_reason     TEXT,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_worktree ON agent_sessions(worktree_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_state ON agent_sessions(state);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_repo_updated ON agent_sessions(repo_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS agent_messages (
    id          TEXT PRIMARY KEY,
    from_agent  TEXT NOT NULL,
    to_agent    TEXT NOT NULL,
    msg_type    TEXT NOT NULL CHECK(msg_type IN (
        'task_delegation',
        'artifact_reference',
        'status_update',
        'intervention_request',
        'intervention_response'
    )),
    payload     TEXT NOT NULL,
    read_at     DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_to
    ON agent_messages(to_agent, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_messages_type
    ON agent_messages(msg_type, created_at DESC);

CREATE TABLE IF NOT EXISTS knowledge_facts (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT REFERENCES task_plans(id) ON DELETE CASCADE,
    subject     TEXT NOT NULL,
    predicate   TEXT NOT NULL,
    object      TEXT NOT NULL,
    source      TEXT NOT NULL,
    confidence  REAL NOT NULL DEFAULT 1.0 CHECK(confidence >= 0 AND confidence <= 1),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_knowledge_facts_plan
    ON knowledge_facts(plan_id, created_at DESC);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}
	if err := s.migrateTaskPlansForPlanFirst(); err != nil {
		return err
	}
	if err := s.migrateWorktreeContextsForPlanFirst(); err != nil {
		return err
	}
	if err := s.migrateTaskPlansBriefColumns(); err != nil {
		return err
	}
	if err := s.migratePlanStepsExpansionColumns(); err != nil {
		return err
	}
	return s.migrateAgentSessionsWorkflowColumns()
}

func (s *Store) migrateTaskPlansForPlanFirst() error {
	rows, err := s.db.Query(`PRAGMA table_info(task_plans)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var needsRebuild bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "task_id" && notnull == 1 {
			needsRebuild = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !needsRebuild {
		return nil
	}

	if _, err := s.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer s.db.Exec(`PRAGMA foreign_keys=ON`)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	steps := []string{
		`ALTER TABLE task_plans RENAME TO task_plans_legacy`,
		`CREATE TABLE task_plans (
			id            TEXT PRIMARY KEY,
			task_id       TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
			title         TEXT NOT NULL,
			status        TEXT NOT NULL CHECK(status IN ('draft', 'approved', 'active', 'blocked', 'completed', 'discarded', 'archived')),
			current_step  TEXT,
			plan_body     TEXT,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			archived_at   DATETIME,
			done_at       DATETIME
		)`,
		`INSERT INTO task_plans (
			id, task_id, title, status, current_step, plan_body,
			created_at, updated_at, archived_at, done_at
		) SELECT
			id, NULLIF(task_id, ''), title, status, current_step, plan_body,
			created_at, updated_at, archived_at, done_at
		FROM task_plans_legacy`,
		`DROP TABLE task_plans_legacy`,
		`CREATE INDEX IF NOT EXISTS idx_task_plans_task_updated
			ON task_plans(task_id, updated_at DESC)`,
	}
	for _, step := range steps {
		if _, err := tx.Exec(step); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) migrateWorktreeContextsForPlanFirst() error {
	rows, err := s.db.Query(`PRAGMA table_info(worktree_contexts)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var hasCurrentPlanID bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "current_plan_id" {
			hasCurrentPlanID = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasCurrentPlanID {
		return nil
	}
	steps := []string{
		`ALTER TABLE worktree_contexts ADD COLUMN current_plan_id TEXT REFERENCES task_plans(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_worktree_contexts_current_plan ON worktree_contexts(current_plan_id)`,
	}
	for _, step := range steps {
		if _, err := s.db.Exec(step); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateTaskPlansBriefColumns() error {
	return ensureColumns(s.db, "task_plans", map[string]string{
		"why_now":      "TEXT",
		"success":      "TEXT",
		"out_of_scope": "TEXT",
		"known_risks":  "TEXT",
	})
}

func (s *Store) migratePlanStepsExpansionColumns() error {
	return ensureColumns(s.db, "plan_steps", map[string]string{
		"expanded_task_id": "TEXT REFERENCES task_contexts(id) ON DELETE SET NULL",
	})
}

func (s *Store) migrateAgentSessionsWorkflowColumns() error {
	return ensureColumns(s.db, "agent_sessions", map[string]string{
		"task_id":        "TEXT REFERENCES task_contexts(id) ON DELETE SET NULL",
		"plan_id":        "TEXT REFERENCES task_plans(id) ON DELETE SET NULL",
		"step_id":        "TEXT REFERENCES plan_steps(id) ON DELETE SET NULL",
		"env_snapshot":   "TEXT",
		"last_heartbeat": "DATETIME",
		"stop_reason":    "TEXT",
	})
}

func ensureColumns(db *sql.DB, table string, columns map[string]string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for name, decl := range columns {
		if existing[name] {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, decl)); err != nil {
			return err
		}
	}
	return nil
}
