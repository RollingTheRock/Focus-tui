package app

import (
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseTaskArchivePaneMsg closes the archive/deleted task overlay.
type CloseTaskArchivePaneMsg struct{}

type taskArchivePane struct {
	common    models.CommonModel
	repoID    string
	archived  []models.TaskContextRecord
	deleted   []models.TaskContextRecord
	activeTab int // 0 = archived, 1 = deleted
	cursor    int
	loading   bool
	width     int
	height    int

	tabTitles []string
}

func newTaskArchivePane(common models.CommonModel, repoID string) *taskArchivePane {
	return &taskArchivePane{
		common:    common,
		repoID:    repoID,
		loading:   true,
		tabTitles: []string{"Archived Tasks", "Deleted Tasks"},
	}
}

func (p *taskArchivePane) Init() tea.Cmd {
	return p.loadCmd()
}

func (p *taskArchivePane) KeyBindings(compact bool) []models.KeyBinding {
	return []models.KeyBinding{
		{Keys: []string{"j", "k"}, Help: "move"},
		{Keys: []string{"enter"}, Help: "restore"},
		{Keys: []string{"tab"}, Help: "switch"},
		{Keys: []string{"R"}, Help: "refresh"},
		{Keys: []string{"q", "esc"}, Help: "close"},
	}
}

func (p *taskArchivePane) loadCmd() tea.Cmd {
	return func() tea.Msg {
		var archived, deleted []models.TaskContextRecord
		if p.common.Store != nil {
			archived, _ = p.common.Store.ListArchivedTaskContexts(p.repoID)
			deleted, _ = p.common.Store.ListDeletedTaskContexts(p.repoID)
		}
		return taskArchiveLoadedMsg{archived: archived, deleted: deleted}
	}
}

func (p *taskArchivePane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case taskArchiveLoadedMsg:
		p.archived = msg.archived
		p.deleted = msg.deleted
		p.loading = false
		p.cursor = 0
		return p, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return p, func() tea.Msg { return CloseTaskArchivePaneMsg{} }
		case "tab":
			p.activeTab = 1 - p.activeTab
			p.cursor = 0
			return p, nil
		case "j", "down":
			p.moveCursor(1)
			return p, nil
		case "k", "up":
			p.moveCursor(-1)
			return p, nil
		case "enter":
			if task := p.selectedTask(); task != nil {
				return p, func() tea.Msg { return requestRestoreTaskMsg{TaskID: task.ID} }
			}
			return p, nil
		case "R":
			p.loading = true
			return p, p.loadCmd()
		}
	}
	return p, nil
}

func (p *taskArchivePane) selectedTask() *models.TaskContextRecord {
	items := p.currentItems()
	if p.cursor < 0 || p.cursor >= len(items) {
		return nil
	}
	return &items[p.cursor]
}

func (p *taskArchivePane) currentItems() []models.TaskContextRecord {
	if p.activeTab == 0 {
		return p.archived
	}
	return p.deleted
}

func (p *taskArchivePane) moveCursor(delta int) {
	items := p.currentItems()
	if len(items) == 0 {
		p.cursor = 0
		return
	}
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(items) {
		p.cursor = len(items) - 1
	}
}

func (p *taskArchivePane) View() tea.View {
	w := p.width
	if w <= 0 {
		w = 80
	}
	h := p.height
	if h <= 0 {
		h = 20
	}

	var b strings.Builder

	// Header
	b.WriteString(p.renderTabBar(w))
	b.WriteByte('\n')

	if p.loading {
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("  Loading..."))
		return tea.NewView(lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(b.String()))
	}

	items := p.currentItems()
	if len(items) == 0 {
		if p.activeTab == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("  No archived tasks yet."))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("  No deleted tasks yet."))
		}
		b.WriteByte('\n')
		return tea.NewView(lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(b.String()))
	}

	// List items
	for i, task := range items {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(appstyles.Text)
		if i == p.cursor {
			prefix = "▸ "
			style = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Highlight)
		}
		line := prefix + task.Title
		if task.State != "" {
			line += " (" + task.State + ")"
		}
		b.WriteString(style.Render(line))
		b.WriteByte('\n')
	}

	return tea.NewView(lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(b.String()))
}

func (p *taskArchivePane) renderTabBar(w int) string {
	var parts []string
	for i, name := range p.tabTitles {
		if i == p.activeTab {
			parts = append(parts, tabActiveStyle.Render(" ●"+name+" "))
		} else {
			parts = append(parts, tabInactiveStyle.Render("  "+name+" "))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	padding := w - lipgloss.Width(bar) - 2
	if padding < 0 {
		padding = 0
	}
	return " " + bar + strings.Repeat(" ", padding)
}

func (p *taskArchivePane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

type taskArchiveLoadedMsg struct {
	archived []models.TaskContextRecord
	deleted  []models.TaskContextRecord
}

type requestRestoreTaskMsg struct {
	TaskID string
}
