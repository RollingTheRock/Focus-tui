package shell

import (
	"testing"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSessionStatus(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want models.PaneStatus
	}{
		{name: "default is starting", m: Model{}, want: models.PaneStatusStarting},
		{name: "running shell", m: Model{running: true}, want: models.PaneStatusRunning},
		{name: "exited shell", m: Model{exited: true}, want: models.PaneStatusExited},
	}

	for _, tt := range tests {
		if got := tt.m.SessionStatus(); got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestScopedShellMessagesIgnoreOtherPanes(t *testing.T) {
	m := &Model{id: "shell-1", running: true}

	updated, cmd := m.Update(RefreshMsg{PaneID: "shell-2"})
	if cmd != nil {
		t.Fatalf("expected no command for foreign pane refresh")
	}
	if updated.(*Model).running != true {
		t.Fatalf("expected model state to remain unchanged")
	}
}

func TestExitedShellRestartsOnKeyPress(t *testing.T) {
	m := &Model{id: "shell-1", exited: true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected restart command after key press")
	}
	model := updated.(*Model)
	if model.exited {
		t.Fatalf("expected exited flag to clear before restart")
	}
}
