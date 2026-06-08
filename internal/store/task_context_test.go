package store

import (
	"testing"
	"time"

	"focus/internal/events"
)

func TestSaveTaskContextPublishesTaskEventsInSQLiteMode(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()

	ch := st.EventBus().Subscribe("test-task-events", func(e events.Event) bool {
		return e.AggregateID == "task-1"
	})
	defer st.EventBus().Unsubscribe("test-task-events")

	if err := st.SaveTaskContext(TaskContextRecord{
		ID:       "task-1",
		RepoID:   "/repo/main",
		Title:    "Task 1",
		State:    "active",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}

	assertEventType(t, ch, events.TaskCreated)

	if err := st.SaveTaskContext(TaskContextRecord{
		ID:       "task-1",
		RepoID:   "/repo/main",
		Title:    "Task 1",
		State:    "done",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("update task: %v", err)
	}

	assertEventType(t, ch, events.TaskStateChanged)
}

func assertEventType(t *testing.T, ch <-chan events.Event, want string) {
	t.Helper()
	select {
	case got := <-ch:
		if got.EventType != want {
			t.Fatalf("event type = %s, want %s", got.EventType, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event %s", want)
	}
}
