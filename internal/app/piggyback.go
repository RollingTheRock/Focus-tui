package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/events"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// scopeKey identifies an event scope for querying.
type scopeKey struct {
	Type string
	ID   string
}

// extractScopes inspects tool parameters for common scope identifiers.
func extractScopes(params map[string]any) []scopeKey {
	var scopes []scopeKey
	if v := toolStringParam(params, "task_id"); v != "" {
		scopes = append(scopes, scopeKey{events.AggregateTask, v})
	}
	if v := toolStringParam(params, "worktree_id"); v != "" {
		scopes = append(scopes, scopeKey{events.AggregateWorktree, v})
	}
	if v := toolStringParam(params, "plan_id"); v != "" {
		scopes = append(scopes, scopeKey{events.AggregatePlan, v})
	}
	if v := toolStringParam(params, "repo_id"); v != "" {
		scopes = append(scopes, scopeKey{"repo", v})
	}
	if v := toolStringParam(params, "session_id"); v != "" {
		scopes = append(scopes, scopeKey{events.AggregateAgentSession, v})
	}
	return scopes
}

// piggybackForParams queries recent events for all scopes inferred from params
// and returns a formatted markdown summary.
func (m *model) piggybackForParams(params map[string]any) string {
	if m.disablePiggyback {
		return ""
	}
	if m.common == nil || m.common.Store == nil {
		return ""
	}
	scopes := extractScopes(params)
	if len(scopes) == 0 {
		return ""
	}
	raw := m.common.Store.EventStore()
	if raw == nil {
		return ""
	}
	evStore, ok := raw.(*store.EventStore)
	if !ok {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var allEvents []events.Event
	seen := make(map[int64]bool)
	for _, sc := range scopes {
		evs, _ := evStore.GetRecentEventsForScope(ctx, sc.Type, sc.ID, 5)
		for _, ev := range evs {
			if !seen[ev.EventID] {
				seen[ev.EventID] = true
				allEvents = append(allEvents, ev)
			}
		}
	}
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].EventID > allEvents[j].EventID
	})
	if len(allEvents) > 5 {
		allEvents = allEvents[:5]
	}
	return formatPiggyback(allEvents)
}

// formatPiggyback renders a slice of events into a human-readable markdown summary.
func formatPiggyback(evs []events.Event) string {
	if len(evs) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "\n---\n📬 Context Updates (recent activity):")
	for _, ev := range evs {
		switch ev.EventType {
		case events.TaskCreated:
			lines = append(lines, fmt.Sprintf("• Task created by %s", ev.ActorID))
		case events.TaskStateChanged:
			lines = append(lines, fmt.Sprintf("• State changed at %s", ev.OccurredAt.Format("15:04")))
		case events.TaskGoalUpdated:
			lines = append(lines, fmt.Sprintf("• Goal/next-step updated at %s", ev.OccurredAt.Format("15:04")))
		case events.WorktreeContextUpdated:
			lines = append(lines, fmt.Sprintf("• Worktree context updated at %s", ev.OccurredAt.Format("15:04")))
		case events.AgentSessionCreated:
			lines = append(lines, fmt.Sprintf("• Agent session started at %s", ev.OccurredAt.Format("15:04")))
		case events.AgentSessionHeartbeat:
			lines = append(lines, fmt.Sprintf("• Agent heartbeat at %s", ev.OccurredAt.Format("15:04")))
		case events.AgentSessionDisconnected:
			lines = append(lines, fmt.Sprintf("• Agent disconnected at %s", ev.OccurredAt.Format("15:04")))
		case events.TodoCreated:
			lines = append(lines, fmt.Sprintf("• Todo created at %s", ev.OccurredAt.Format("15:04")))
		case events.PlanStepStateChanged:
			lines = append(lines, fmt.Sprintf("• Plan step updated at %s", ev.OccurredAt.Format("15:04")))
		case events.ContextNoteAdded:
			lines = append(lines, fmt.Sprintf("• Context note added at %s", ev.OccurredAt.Format("15:04")))
		case events.TaskDependencyAdded:
			lines = append(lines, fmt.Sprintf("• Task dependency added at %s", ev.OccurredAt.Format("15:04")))
		case events.TaskDependencyRemoved:
			lines = append(lines, fmt.Sprintf("• Task dependency removed at %s", ev.OccurredAt.Format("15:04")))
		case events.KnowledgeFactAdded:
			lines = append(lines, fmt.Sprintf("• Knowledge fact added at %s", ev.OccurredAt.Format("15:04")))
		case events.SessionHandoffCreated:
			lines = append(lines, fmt.Sprintf("• Session handoff created at %s", ev.OccurredAt.Format("15:04")))
		case events.TaskBriefUpdated:
			lines = append(lines, fmt.Sprintf("• Task brief updated at %s", ev.OccurredAt.Format("15:04")))
		case events.TaskOutputAdded:
			lines = append(lines, fmt.Sprintf("• Task output added at %s", ev.OccurredAt.Format("15:04")))
		case events.TaskPlanCreated:
			lines = append(lines, fmt.Sprintf("• Task plan created at %s", ev.OccurredAt.Format("15:04")))
		case events.PomodoroStarted:
			lines = append(lines, fmt.Sprintf("• Pomodoro started at %s", ev.OccurredAt.Format("15:04")))
		case events.PomodoroCompleted:
			lines = append(lines, fmt.Sprintf("• Pomodoro completed at %s", ev.OccurredAt.Format("15:04")))
		case events.PomodoroCancelled:
			lines = append(lines, fmt.Sprintf("• Pomodoro cancelled at %s", ev.OccurredAt.Format("15:04")))
		case events.StreakUpdated:
			lines = append(lines, fmt.Sprintf("• Streak updated at %s", ev.OccurredAt.Format("15:04")))
		case events.WorktreeHistoryRecorded:
			lines = append(lines, fmt.Sprintf("• Worktree history recorded at %s", ev.OccurredAt.Format("15:04")))
		default:
			lines = append(lines, fmt.Sprintf("• %s at %s", ev.EventType, ev.OccurredAt.Format("15:04")))
		}
	}
	lines = append(lines, "---")
	return strings.Join(lines, "\n")
}

// withPiggyback wraps an MCP tool handler to inject _context_updates into
// successful responses. Errors are passed through unchanged.
func (m *model) withPiggyback(handler func(map[string]any) (map[string]any, error)) func(map[string]any) (map[string]any, error) {
	return func(params map[string]any) (map[string]any, error) {
		result, err := handler(params)
		if err != nil {
			return result, err
		}
		if m.disablePiggyback {
			return result, nil
		}
		updates := m.piggybackForParams(params)
		if updates != "" {
			result["_context_updates"] = updates
		}
		return result, nil
	}
}
