//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestPostgresqlModeSmoke starts an embedded PostgreSQL instance, writes a task
// and a todo, and verifies they can be read back from projection tables.
//
// Run with: go test -tags=integration ./internal/store/... -run TestPostgresqlModeSmoke -timeout=120s
func TestPostgresqlModeSmoke(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping integration test in CI")
	}

	t.Setenv("FOCUS_STORE", "postgresql")

	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("open postgresql store: %v", err)
	}
	defer st.Close()

	if st.Mode() != "postgresql" {
		t.Fatalf("expected postgresql mode, got %s", st.Mode())
	}

	ctx := context.Background()

	// Write a task.
	taskID := "task-smoke-1"
	if err := st.SaveTaskContext(TaskContextRecord{
		ID:     taskID,
		RepoID: "repo-smoke",
		Title:  "Smoke Test Task",
		State:  "active",
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Write a todo.
	todo, err := st.CreateTodo("smoke todo", "today")
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}

	// Allow projection builder a moment to catch up.
	time.Sleep(6 * time.Second)

	// Read task back via projection table.
	task, err := st.GetTaskContext(taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil {
		t.Fatal("task not found in projections")
	}
	if task.Title != "Smoke Test Task" {
		t.Fatalf("unexpected task title: %s", task.Title)
	}

	// Read todo back via projection table.
	readTodo, err := st.GetTodo(todo.ID)
	if err != nil {
		t.Fatalf("get todo: %v", err)
	}
	if readTodo == nil {
		t.Fatal("todo not found in projections")
	}
	if readTodo.Text != "smoke todo" {
		t.Fatalf("unexpected todo text: %s", readTodo.Text)
	}

	// Verify event store has events.
	raw := st.EventStore()
	if raw == nil {
		t.Fatal("event store is nil")
	}
	evStore := raw.(*EventStore)
	events, err := evStore.ReadEventsAfterForConsumer(ctx, "test-consumer", 100)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events in event store")
	}
}
