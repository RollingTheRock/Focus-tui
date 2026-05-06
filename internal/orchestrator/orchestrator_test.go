package orchestrator

import (
	"context"
	"testing"
	"time"

	"focus/internal/events"
)

type fakeStore struct {
	downstream          map[string][]Task
	ready               map[string]bool
	disconnected        []string
	activeAgentSessions []AgentSession
	taskContexts        map[string]*TaskContext
}

func (f *fakeStore) GetDownstreamTasks(taskID string) ([]Task, error) {
	return f.downstream[taskID], nil
}

func (f *fakeStore) AllPrerequisitesMet(taskID string) (bool, error) {
	ok, exists := f.ready[taskID]
	if !exists {
		return false, nil
	}
	return ok, nil
}

func (f *fakeStore) MarkSessionDisconnected(sessionID string, reason string) error {
	f.disconnected = append(f.disconnected, sessionID)
	return nil
}

func (f *fakeStore) ListActiveAgentSessions() ([]AgentSession, error) {
	return f.activeAgentSessions, nil
}

func (f *fakeStore) GetTaskContext(taskID string) (*TaskContext, error) {
	return f.taskContexts[taskID], nil
}

func TestOrchestratorDownstreamReadyNotification(t *testing.T) {
	bus := events.NewEventBus()
	store := &fakeStore{
		downstream: map[string][]Task{
			"task-A": {
				{ID: "task-B", Name: "B"},
				{ID: "task-C", Name: "C"},
			},
		},
		ready: map[string]bool{
			"task-B": true,
			"task-C": false,
		},
	}
	o := New(store, bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Start(ctx)
	defer o.Stop()

	// Give eventLoop time to subscribe.
	time.Sleep(100 * time.Millisecond)

	payload, _ := events.Serialize(events.TaskStateChangedPayload{PreviousState: "active", NewState: "done"})
	bus.Publish(events.Event{
		EventType:     events.TaskStateChanged,
		AggregateID:   "task-A",
		AggregateType: events.AggregateTask,
		Payload:       payload,
	})

	select {
	case n := <-o.Notifications():
		if n.Type != "downstream_ready" {
			t.Fatalf("expected downstream_ready, got %s", n.Type)
		}
		if n.TaskID != "task-B" {
			t.Fatalf("expected task-B, got %s", n.TaskID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for downstream_ready notification")
	}
}

func TestOrchestratorTaskBlockedNotification(t *testing.T) {
	bus := events.NewEventBus()
	store := &fakeStore{
		taskContexts: map[string]*TaskContext{
			"task-X": {ID: "task-X", Title: "X", State: "blocked"},
		},
	}
	o := New(store, bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Start(ctx)
	defer o.Stop()

	// Give eventLoop time to subscribe.
	time.Sleep(100 * time.Millisecond)

	payload, _ := events.Serialize(events.TaskStateChangedPayload{PreviousState: "active", NewState: "blocked"})
	bus.Publish(events.Event{
		EventType:     events.TaskStateChanged,
		AggregateID:   "task-X",
		AggregateType: events.AggregateTask,
		Payload:       payload,
	})

	select {
	case n := <-o.Notifications():
		if n.Type != "task_blocked" {
			t.Fatalf("expected task_blocked, got %s", n.Type)
		}
		if n.TaskID != "task-X" {
			t.Fatalf("expected task-X, got %s", n.TaskID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task_blocked notification")
	}
}

func TestOrchestratorHeartbeatTimeout(t *testing.T) {
	// Heartbeat loop uses a 10s initial + 30s periodic timer.
	// Full integration testing is deferred to avoid long test runs.
	t.Skip("heartbeat timeout tested via integration")
}
