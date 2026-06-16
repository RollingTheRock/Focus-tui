package app

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/events"
)

func TestExtractScopes(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		want   []scopeKey
	}{
		{
			name:   "empty params",
			params: map[string]any{},
			want:   nil,
		},
		{
			name:   "task_id only",
			params: map[string]any{"task_id": "task-1"},
			want:   []scopeKey{{events.AggregateTask, "task-1"}},
		},
		{
			name:   "multiple scopes",
			params: map[string]any{"task_id": "task-1", "worktree_id": "wt-1", "plan_id": "plan-1"},
			want: []scopeKey{
				{events.AggregateTask, "task-1"},
				{events.AggregateWorktree, "wt-1"},
				{events.AggregatePlan, "plan-1"},
			},
		},
		{
			name:   "repo_id and session_id",
			params: map[string]any{"repo_id": "repo-1", "session_id": "sess-1"},
			want: []scopeKey{
				{"repo", "repo-1"},
				{events.AggregateAgentSession, "sess-1"},
			},
		},
		{
			name:   "ignored keys",
			params: map[string]any{"foo": "bar", "task_id": "task-2"},
			want:   []scopeKey{{events.AggregateTask, "task-2"}},
		},
		{
			name:   "empty string values are ignored",
			params: map[string]any{"task_id": ""},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractScopes(tt.params)
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("extractScopes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatPiggyback(t *testing.T) {
	now := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		evs   []events.Event
		want  string
		empty bool
	}{
		{
			name:  "no events",
			evs:   []events.Event{},
			empty: true,
		},
		{
			name: "task created",
			evs: []events.Event{
				{EventID: 1, EventType: events.TaskCreated, OccurredAt: now, ActorID: "system"},
			},
			want: "\n---\n📬 Context Updates (recent activity):\n• Task created by system\n---",
		},
		{
			name: "multiple events",
			evs: []events.Event{
				{EventID: 3, EventType: events.TaskStateChanged, OccurredAt: now.Add(2 * time.Minute)},
				{EventID: 2, EventType: events.TaskGoalUpdated, OccurredAt: now.Add(time.Minute)},
				{EventID: 1, EventType: events.TaskCreated, OccurredAt: now, ActorID: "system"},
			},
			want: "\n---\n📬 Context Updates (recent activity):\n• State changed at 14:32\n• Goal/next-step updated at 14:31\n• Task created by system\n---",
		},
		{
			name: "unknown event type falls back to generic",
			evs: []events.Event{
				{EventID: 1, EventType: "CustomEvent", OccurredAt: now},
			},
			want: "\n---\n📬 Context Updates (recent activity):\n• CustomEvent at 14:30\n---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPiggyback(tt.evs)
			if tt.empty {
				if got != "" {
					t.Errorf("formatPiggyback() = %q, want empty string", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("formatPiggyback() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithPiggyback(t *testing.T) {
	t.Run("injects _context_updates on success", func(t *testing.T) {
		m := &model{disablePiggyback: false}
		handler := func(params map[string]any) (map[string]any, error) {
			return map[string]any{"task_id": "t1"}, nil
		}
		wrapped := m.withPiggyback(handler)
		// piggybackForParams will return empty because there's no EventStore,
		// but the decorator should still run and not error.
		result, err := wrapped(map[string]any{"task_id": "t1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["task_id"] != "t1" {
			t.Errorf("original result lost")
		}
	})

	t.Run("passes errors through unchanged", func(t *testing.T) {
		m := &model{disablePiggyback: false}
		handler := func(params map[string]any) (map[string]any, error) {
			return nil, fmt.Errorf("boom")
		}
		wrapped := m.withPiggyback(handler)
		_, err := wrapped(nil)
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom error, got: %v", err)
		}
	})

	t.Run("disabled via model flag skips injection", func(t *testing.T) {
		m := &model{disablePiggyback: true}
		handler := func(params map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		}
		wrapped := m.withPiggyback(handler)
		result, err := wrapped(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := result["_context_updates"]; exists {
			t.Error("_context_updates should not be injected when disabled")
		}
	})
}
