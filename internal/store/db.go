package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/events"
	"github.com/RollingTheRock/Focus-tui/internal/platform"
	"github.com/RollingTheRock/Focus-tui/internal/store/pgconn"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"
)

// Store wraps database connections. In sqlite mode it holds a single SQLite DB.
// In postgresql mode it holds an embedded PostgreSQL instance, its connection
// pool, the Event Store, and the in-memory Event Bus.
type Store struct {
	db         *sql.DB
	pgPool     *pgxpool.Pool
	pgEmbedded *pgconn.EmbeddedPostgres // only set in postgresql mode
	events     *EventStore
	bus        *events.EventBus
	mode       string // "sqlite" or "postgresql"
}

// New opens (or creates) the database and runs migrations.
// By default it opens SQLite. If FOCUS_STORE=postgresql is set, it starts an
// embedded PostgreSQL instance instead.
func New(dbPath string) (*Store, error) {
	mode := os.Getenv("FOCUS_STORE")
	if mode == "" {
		mode = "sqlite"
	}

	switch mode {
	case "postgresql":
		return newPostgresStore()
	default:
		return newSQLiteStore(dbPath)
	}
}

func newSQLiteStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// In-memory SQLite databases are private to each connection. Force a single
	// connection so that all operations share the same database instance.
	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
		db.SetConnMaxLifetime(0)
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
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	s := &Store{db: db, mode: "sqlite", bus: events.NewEventBus()}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func newPostgresStore() (*Store, error) {
	ctx := context.Background()

	dataDir, err := pgconn.DefaultDataDir()
	if err != nil {
		return nil, fmt.Errorf("pg data dir: %w", err)
	}

	emb, err := pgconn.Start(ctx, dataDir)
	if err != nil {
		return nil, fmt.Errorf("start embedded postgres: %w", err)
	}

	pool := emb.Pool()

	bus := events.NewEventBus()
	es := NewEventStore(pool)
	es.SetBus(bus)

	s := &Store{
		pgEmbedded: emb,
		pgPool:     pool,
		events:     es,
		bus:        bus,
		mode:       "postgresql",
	}

	// Migrate event store and projection schemas.
	if err := s.events.MigrateEventSchema(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate event schema: %w", err)
	}
	if err := MigrateProjectionSchema(ctx, pool); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate projection schema: %w", err)
	}

	return s, nil
}

// Close closes all database connections and stops the embedded PostgreSQL
// instance when running in postgresql mode.
func (s *Store) Close() error {
	var errs []error
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.pgEmbedded != nil {
		if err := s.pgEmbedded.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.pgPool != nil {
		s.pgPool.Close()
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// PGPool returns the PostgreSQL connection pool (nil in sqlite mode).
func (s *Store) PGPool() *pgxpool.Pool {
	return s.pgPool
}

// Mode returns the current storage mode ("sqlite" or "postgresql").
func (s *Store) Mode() string {
	return s.mode
}

// SetMaxOpenConns sets the maximum number of open connections to the database.
// Useful in tests with in-memory SQLite to force single-connection mode.
func (s *Store) SetMaxOpenConns(n int) {
	if s.db != nil {
		s.db.SetMaxOpenConns(n)
	}
}

// EventStore returns the event store (nil in sqlite mode).
func (s *Store) EventStore() any {
	if s.events == nil {
		return nil
	}
	return s.events
}

// EventBus returns the in-memory event bus. Available in both sqlite and
// postgresql modes.
func (s *Store) EventBus() *events.EventBus {
	return s.bus
}

// DefaultDBPath returns the platform-appropriate database path.
func DefaultDBPath() (string, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "focus.db"), nil
}

// ResolveProjectRoot returns the project root for the given cwd.
// It uses git to detect repository root; if unavailable or not in a repo,
// it falls back to cwd.
func ResolveProjectRoot(cwd string) (string, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = abs
	out, err := cmd.Output()
	if err != nil {
		return abs, nil
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return abs, nil
	}
	return root, nil
}

// ProjectDBPath returns the project-local database path:
// <project_root>/.focus/focus.db.
func ProjectDBPath(projectRoot string) (string, error) {
	if projectRoot == "" {
		return "", fmt.Errorf("project root required")
	}
	return filepath.Join(projectRoot, ".focus", "focus.db"), nil
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
    deleted_at            DATETIME,
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

CREATE TABLE IF NOT EXISTS worktree_history (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL,
    branch          TEXT,
    path            TEXT,
    created_at      DATETIME,
    removed_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    task_id         TEXT,
    plan_id         TEXT,
    provider        TEXT,
    summary         TEXT,
    duration_minutes INTEGER
);

CREATE INDEX IF NOT EXISTS idx_worktree_history_repo
    ON worktree_history(repo_id, removed_at DESC);

CREATE TABLE IF NOT EXISTS agent_definitions (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    binary          TEXT NOT NULL,
    args            TEXT DEFAULT '[]',
    env_vars        TEXT DEFAULT '[]',
    provider_type   TEXT NOT NULL DEFAULT 'generic',
    tags            TEXT DEFAULT '[]',
    category        TEXT NOT NULL DEFAULT 'registered'
        CHECK(category IN ('built-in', 'registered', 'recommended')),
    install_hint    TEXT,
    capabilities    TEXT DEFAULT '{}',
    is_installed    BOOLEAN NOT NULL DEFAULT 0,
    is_enabled      BOOLEAN NOT NULL DEFAULT 1,
    last_used_at    DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_definitions_category
    ON agent_definitions(category);
CREATE INDEX IF NOT EXISTS idx_agent_definitions_enabled
    ON agent_definitions(is_enabled);
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
	if err := s.migrateAgentSessionsWorkflowColumns(); err != nil {
		return err
	}
	if err := s.migrateTodosTaskID(); err != nil {
		return err
	}
	if err := s.migrateAgentDefinitionsInstalledColumn(); err != nil {
		return err
	}
	if err := s.migrateTaskContextsDeletedAtColumn(); err != nil {
		return err
	}
	return s.migrateFixTaskPlansForeignKeys()
}

func (s *Store) migrateAgentDefinitionsInstalledColumn() error {
	return ensureColumns(s.db, "agent_definitions", map[string]string{
		"is_installed": "BOOLEAN NOT NULL DEFAULT 0",
	})
}

func (s *Store) migrateTodosTaskID() error {
	return ensureColumns(s.db, "todos", map[string]string{
		"task_id": "TEXT",
	})
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
		`DROP TABLE IF EXISTS task_plans_legacy`,
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

func (s *Store) migrateTaskContextsDeletedAtColumn() error {
	return ensureColumns(s.db, "task_contexts", map[string]string{
		"deleted_at": "DATETIME",
	})
}

// migrateFixTaskPlansForeignKeys detects and repairs foreign keys in plan_steps
// and session_handoffs that were left pointing to task_plans_legacy after a prior
// migration (task_plans → task_plans_legacy rename). SQLite auto-updates FK references
// on rename, so dependent tables end up referencing the legacy table. This migration
// rebuilds those tables to point to the correct task_plans table.
func (s *Store) migrateFixTaskPlansForeignKeys() error {
	tables := []struct {
		name       string
		fkCol      string
		refTable   string
		onDelete   string
		recreateSQL string
		indexSQL   string
	}{
		{
			name:     "plan_steps",
			fkCol:    "plan_id",
			refTable: "task_plans",
			onDelete: "CASCADE",
			recreateSQL: `CREATE TABLE plan_steps_new (
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
			)`,
			indexSQL: `CREATE INDEX IF NOT EXISTS idx_plan_steps_plan_order ON plan_steps(plan_id, order_index ASC)`,
		},
		{
			name:     "session_handoffs",
			fkCol:    "plan_id",
			refTable: "task_plans",
			onDelete: "SET NULL",
			recreateSQL: `CREATE TABLE session_handoffs_new (
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
			)`,
			indexSQL: `CREATE INDEX IF NOT EXISTS idx_session_handoffs_task_created ON session_handoffs(task_id, created_at DESC)`,
		},
	}

	for _, tb := range tables {
		needsFix, err := s.hasForeignKeyTo(s.legacyTableName(tb.name), tb.fkCol)
		if err != nil {
			return err
		}
		if !needsFix {
			continue
		}

		// Rebuild the table with correct FK reference
		if _, err := s.db.Exec(tb.recreateSQL); err != nil {
			return err
		}
		if _, err := s.db.Exec(fmt.Sprintf(
			`INSERT OR IGNORE INTO %s_new SELECT * FROM %s`, tb.name, tb.name,
		)); err != nil {
			return err
		}
		if _, err := s.db.Exec(fmt.Sprintf(`DROP TABLE %s`, tb.name)); err != nil {
			return err
		}
		if _, err := s.db.Exec(fmt.Sprintf(
			`ALTER TABLE %s_new RENAME TO %s`, tb.name, tb.name,
		)); err != nil {
			return err
		}
		if _, err := s.db.Exec(tb.indexSQL); err != nil {
			return err
		}
	}
	return nil
}

// legacyTableName returns the legacy table name for known task_plans references.
func (s *Store) legacyTableName(tableName string) string {
	return "task_plans_legacy"
}

// hasForeignKeyTo checks if the given table has a foreign key referencing the target table.
func (s *Store) hasForeignKeyTo(tableName, fkCol string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA foreign_key_list(%s)`, tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, seq int
		var refTable, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return false, err
		}
		if refTable == "task_plans_legacy" {
			return true, nil
		}
	}
	return false, rows.Err()
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
