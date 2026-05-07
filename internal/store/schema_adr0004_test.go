package store

import (
	"database/sql"
	"testing"
)

func TestADR0004SchemaTablesExist(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	tables := []string{
		"task_dependencies",
		"task_outputs",
		"agent_messages",
		"knowledge_facts",
	}
	for _, table := range tables {
		if !tableExists(t, s.db, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}

func TestADR0004AgentSessionColumnsExist(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	cols := map[string]bool{}
	rows, err := s.db.Query(`PRAGMA table_info(agent_sessions)`)
	if err != nil {
		t.Fatalf("query table info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}

	for _, col := range []string{"env_snapshot", "last_heartbeat", "stop_reason"} {
		if !cols[col] {
			t.Fatalf("expected column %s to exist in agent_sessions", col)
		}
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	row := db.QueryRow(`SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, name)
	var one int
	if err := row.Scan(&one); err != nil {
		return false
	}
	return one == 1
}
