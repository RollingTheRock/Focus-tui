package app

import (
	"testing"

	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/store"
	"focus/internal/ui/layout"
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

	got := formatPaneTitle(meta, true, false, 56)
	want := "SHELL 2 [/.../projects/focus-tui] [running] [focus]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleUsesFixedBadgeForNativePane(t *testing.T) {
	meta := models.PaneMeta{Name: "Todo", Type: models.PaneTypeTodo, Closable: false}

	got := formatPaneTitle(meta, false, false, 20)
	want := "TODO [fixed]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleUsesActiveForShellMode(t *testing.T) {
	meta := models.PaneMeta{Name: "Shell", Type: models.PaneTypeShell, Status: models.PaneStatusRunning}

	got := formatPaneTitle(meta, true, true, 28)
	want := "SHELL [running] [active]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleDropsCWDWhenPaneIsNarrow(t *testing.T) {
	meta := models.PaneMeta{
		Name:   "Shell 2",
		Type:   models.PaneTypeShell,
		CWD:    "/home/dev/projects/focus-tui",
		Status: models.PaneStatusRunning,
	}

	got := formatPaneTitle(meta, true, false, 18)
	want := "SHELL 2 [focus]"
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

func keyCtrlBackslash() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlBackslash}
}

func keyCtrlW() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlW}
}

func TestSplitFocusedHorizontal(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	initialOrder := layout.LeafOrder(m.bodyTree)
	if len(initialOrder) != 5 {
		t.Fatalf("expected 5 panes initially (shell + git + todo + file-tree + pomodoro), got %d", len(initialOrder))
	}

	newM, cmd := m.Update(keyCtrlBackslash())
	if cmd == nil {
		t.Fatal("expected cmd from split, got nil")
	}
	m = newM.(model)

	newOrder := layout.LeafOrder(m.bodyTree)
	if len(newOrder) != 6 {
		t.Fatalf("expected 6 panes after split, got %d", len(newOrder))
	}

	foundNewPane := false
	for _, id := range newOrder {
		if string(id) != string(paneShell) && string(id) != string(paneGitStatus) && string(id) != string(paneTodo) && string(id) != string(paneFileTree) && string(id) != string(panePomodoro) {
			foundNewPane = true
			if m.focused != id {
				t.Fatalf("expected focus on new pane %s, got %s", id, m.focused)
			}
			if meta, ok := m.paneMeta[id]; !ok || meta.Type != models.PaneTypeShell {
				t.Fatalf("expected new pane to be shell type, got %v", meta.Type)
			}
			break
		}
	}
	if !foundNewPane {
		t.Fatal("new pane not found after split")
	}
}

func TestZoomToggle(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	initialOrder := layout.LeafOrder(m.bodyTree)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = newM.(model)

	if m.zoomedPane != paneShell {
		t.Fatalf("expected zoomed pane to be shell, got %s", m.zoomedPane)
	}

	zoomedOrder := layout.LeafOrder(m.bodyTree)
	if len(zoomedOrder) != 1 {
		t.Fatalf("expected 1 pane when zoomed, got %d", len(zoomedOrder))
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = newM.(model)

	if m.zoomedPane != "" {
		t.Fatalf("expected zoom to be restored, got %s", m.zoomedPane)
	}

	restoredOrder := layout.LeafOrder(m.bodyTree)
	if len(restoredOrder) != len(initialOrder) {
		t.Fatalf("expected %d panes after restore, got %d", len(initialOrder), len(restoredOrder))
	}
}
