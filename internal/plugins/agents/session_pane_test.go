package agents

import (
	"strings"
	"testing"
	"time"

	"focus/internal/agents"
	"focus/internal/models"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSessionPaneNavigation(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)

	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderOpenCode, PID: 1001, WorktreeID: "/tmp/wt1", StartedAt: time.Now()},
		{ID: "s2", Provider: agents.ProviderClaude, PID: 1002, WorktreeID: "/tmp/wt2", StartedAt: time.Now()},
		{ID: "s3", Provider: agents.ProviderKimi, PID: 1003, WorktreeID: "/tmp/wt3", StartedAt: time.Now()},
	})

	if pane.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", pane.cursor)
	}

	updated, _ := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p := updated.(*SessionPane)
	if p.cursor != 1 {
		t.Fatalf("expected cursor 1 after j, got %d", p.cursor)
	}

	updated, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = updated.(*SessionPane)
	if p.cursor != 2 {
		t.Fatalf("expected cursor 2 after second j, got %d", p.cursor)
	}

	updated, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = updated.(*SessionPane)
	if p.cursor != 2 {
		t.Fatalf("expected cursor to stay at 2 at boundary, got %d", p.cursor)
	}

	updated, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	p = updated.(*SessionPane)
	if p.cursor != 1 {
		t.Fatalf("expected cursor 1 after k, got %d", p.cursor)
	}
}

func TestSessionPaneFocusAction(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderOpenCode, PID: 1001, WorktreeID: "/tmp/wt1", StartedAt: time.Now()},
	})

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from enter action")
	}
	msg := cmd()
	focusMsg, ok := msg.(FocusAgentSessionMsg)
	if !ok {
		t.Fatalf("expected FocusAgentSessionMsg, got %T", msg)
	}
	if focusMsg.WorktreeID != "/tmp/wt1" {
		t.Errorf("expected worktree /tmp/wt1, got %q", focusMsg.WorktreeID)
	}
	if focusMsg.Provider != agents.ProviderOpenCode {
		t.Errorf("expected provider opencode, got %q", focusMsg.Provider)
	}
	_ = updated
}

func TestSessionPaneKillAction(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderClaude, PID: 2002, WorktreeID: "/tmp/wt2", StartedAt: time.Now()},
	})

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("expected cmd from kill action")
	}
	msg := cmd()
	killMsg, ok := msg.(KillSessionMsg)
	if !ok {
		t.Fatalf("expected KillSessionMsg, got %T", msg)
	}
	if killMsg.SessionID != "s1" {
		t.Errorf("expected session id s1, got %q", killMsg.SessionID)
	}
	if killMsg.PID != 2002 {
		t.Errorf("expected pid 2002, got %d", killMsg.PID)
	}
	_ = updated
}

func TestSessionPaneLaunchAction(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderKimi, PID: 3003, WorktreeID: "/tmp/wt3", StartedAt: time.Now()},
	})

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected cmd from launch action")
	}
	msg := cmd()
	launchMsg, ok := msg.(agents.LaunchAgentMsg)
	if !ok {
		t.Fatalf("expected LaunchAgentMsg, got %T", msg)
	}
	if launchMsg.WorktreeID != "/tmp/wt3" {
		t.Errorf("expected worktree /tmp/wt3, got %q", launchMsg.WorktreeID)
	}
	if launchMsg.Provider != agents.ProviderKimi {
		t.Errorf("expected provider kimi, got %q", launchMsg.Provider)
	}
	_ = updated
}

func TestSessionPaneSetSessionsUpdatesCursor(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderOpenCode, PID: 1, WorktreeID: "/tmp/wt1", StartedAt: time.Now()},
		{ID: "s2", Provider: agents.ProviderClaude, PID: 2, WorktreeID: "/tmp/wt2", StartedAt: time.Now()},
	})
	pane.cursor = 1

	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderOpenCode, PID: 1, WorktreeID: "/tmp/wt1", StartedAt: time.Now()},
	})

	if pane.cursor != 0 {
		t.Fatalf("expected cursor reset to 0, got %d", pane.cursor)
	}
}

func TestSessionPaneViewNotEmpty(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSize(40, 10)
	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderCodex, PID: 4004, WorktreeID: "/tmp/wt4", StartedAt: time.Now().Add(-5 * time.Minute)},
	})

	view := pane.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestSessionPaneEmptyState(t *testing.T) {
	common := models.CommonModel{Width: 80, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSize(40, 10)
	pane.SetSessions(nil)

	view := pane.View()
	if view == "" {
		t.Fatal("expected non-empty view for empty state")
	}
	if !strings.Contains(view, "No running agents.") {
		t.Fatalf("expected empty state message, got:\n%s", view)
	}
}

func TestSessionPaneShowsRunningAndRecentSections(t *testing.T) {
	common := models.CommonModel{Width: 100, Height: 24, Theme: styles.DefaultTheme()}
	pane := NewSessionPane("agent-session", models.PaneMeta{ID: "agent-session", Name: "Agents", Type: models.PaneTypeAgentSession}, common)
	pane.SetSize(120, 18)
	now := time.Now()
	pane.SetSessions([]*agents.Session{
		{ID: "s1", Provider: agents.ProviderOpenCode, PID: 1, WorktreeID: "/tmp/wt1", State: agents.SessionRunning, StartedAt: now.Add(-2 * time.Minute), LastActivityAt: timePtr(now.Add(-30 * time.Second)), Summary: "syncing context"},
		{ID: "s2", Provider: agents.ProviderClaude, PID: 0, WorktreeID: "/tmp/wt2", State: agents.SessionFailed, StartedAt: now.Add(-5 * time.Minute), LastActivityAt: timePtr(now.Add(-time.Minute)), Summary: "tool call failed"},
		{ID: "s3", Provider: agents.ProviderKimi, PID: 0, WorktreeID: "/tmp/wt3", State: agents.SessionExited, StartedAt: now.Add(-10 * time.Minute), LastActivityAt: timePtr(now.Add(-2 * time.Minute)), Summary: "completed review"},
	})
	view := pane.View()
	for _, want := range []string{"Running", "Attention", "Recent", "opencode  [running]", "claude  [failed]", "kimi  [exited]", "last:", "tool call failed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
	for _, want := range []string{"tmp/wt1", "tmp/wt2", "tmp/wt3"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected grouped worktree label %q, got:\n%s", want, view)
		}
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func TestShortenPath(t *testing.T) {
	if got := shortenPath("/home/user/project"); got != "user/project" {
		t.Errorf("shortenPath = %q, want user/project", got)
	}
	if got := shortenPath("/project"); got != "project" {
		t.Errorf("shortenPath = %q, want project", got)
	}
	if got := shortenPath(""); got != "" {
		t.Errorf("shortenPath empty = %q, want empty", got)
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(30 * time.Second); got != "30s" {
		t.Errorf("formatDuration = %q, want 30s", got)
	}
	if got := formatDuration(5 * time.Minute); got != "5m" {
		t.Errorf("formatDuration = %q, want 5m", got)
	}
	if got := formatDuration(2 * time.Hour); got != "2h" {
		t.Errorf("formatDuration = %q, want 2h", got)
	}
}
