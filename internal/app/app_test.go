package app

import (
	"testing"

	"focus/internal/models"
	"focus/internal/ui/shell"

	tea "github.com/charmbracelet/bubbletea"
)

type fakePanel struct {
	updates []tea.Msg
}

func (p *fakePanel) Init() tea.Cmd { return nil }

func (p *fakePanel) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	p.updates = append(p.updates, msg)
	return p, nil
}

func (p *fakePanel) View() string { return "" }

func (p *fakePanel) SetSize(width, height int) {}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		home string
		want string
	}{
		{name: "home path collapsed", path: "/home/dev/projects/focus-tui", home: "/home/dev", want: "~/projects/focus-tui"},
		{name: "long absolute path shortened", path: "/mnt/d/dev/focus-tui/internal", home: "", want: "/.../focus-tui/internal"},
		{name: "short relative path kept", path: "focus/internal", home: "", want: "focus/internal"},
	}

	for _, tt := range tests {
		if got := shortenPath(tt.path, tt.home); got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestFormatPaneTitleIncludesShellStatusAndFocus(t *testing.T) {
	meta := models.PaneMeta{
		Name:   "Shell 2",
		Type:   models.PaneTypeShell,
		CWD:    "/home/dev/projects/focus-tui",
		Status: models.PaneStatusRunning,
	}

	got := formatPaneTitle(meta, true, false)
	want := "SHELL 2 [/.../projects/focus-tui] [running] [focus]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleUsesFixedBadgeForNativePane(t *testing.T) {
	meta := models.PaneMeta{Name: "Todo", Type: models.PaneTypeTodo, Closable: false}

	got := formatPaneTitle(meta, false, false)
	want := "TODO [fixed]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleUsesActiveForShellMode(t *testing.T) {
	meta := models.PaneMeta{Name: "Shell", Type: models.PaneTypeShell, Status: models.PaneStatusRunning}

	got := formatPaneTitle(meta, true, true)
	want := "SHELL [running] [active]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestShellRefreshMsgRoutesOnlyToTargetPane(t *testing.T) {
	target := &fakePanel{}
	other := &fakePanel{}
	m := model{
		panes: map[models.PaneID]models.Panel{
			"shell-1": target,
			"shell-2": other,
		},
		paneMeta: map[models.PaneID]models.PaneMeta{
			"shell-1": {ID: "shell-1", Type: models.PaneTypeShell},
			"shell-2": {ID: "shell-2", Type: models.PaneTypeShell},
		},
		paneOrder: []models.PaneID{"shell-1", "shell-2"},
	}

	updated, _ := m.Update(shell.RefreshMsg{PaneID: "shell-1"})
	m = updated.(model)

	targetPanel := m.panes["shell-1"].(*fakePanel)
	otherPanel := m.panes["shell-2"].(*fakePanel)
	if len(targetPanel.updates) != 1 {
		t.Fatalf("expected target pane to receive 1 update, got %d", len(targetPanel.updates))
	}
	if len(otherPanel.updates) != 0 {
		t.Fatalf("expected other pane to receive 0 updates, got %d", len(otherPanel.updates))
	}
}
