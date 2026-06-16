package app

import (
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	appstyles "github.com/RollingTheRock/Focus-tui/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseTaskDeleteConfirmMsg closes the task delete confirmation overlay.
type CloseTaskDeleteConfirmMsg struct {
	ID models.PaneID
}

type taskDeleteConfirmPane struct {
	id        models.PaneID
	taskID    string
	taskTitle string
	clearAll  bool
	repoID    string
	width     int
	height    int
}

func newTaskDeleteConfirmPane(id models.PaneID, taskID, taskTitle, repoID string, clearAll bool) *taskDeleteConfirmPane {
	return &taskDeleteConfirmPane{
		id:        id,
		taskID:    taskID,
		taskTitle: taskTitle,
		repoID:    repoID,
		clearAll:  clearAll,
	}
}

func (p *taskDeleteConfirmPane) Init() tea.Cmd {
	return nil
}

func (p *taskDeleteConfirmPane) KeyBindings(compact bool) []models.KeyBinding {
	return []models.KeyBinding{
		{Keys: []string{"y"}, Help: "confirm"},
		{Keys: []string{"n", "esc"}, Help: "cancel"},
	}
}

func (p *taskDeleteConfirmPane) ID() models.PaneID {
	return p.id
}

func (p *taskDeleteConfirmPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			if p.clearAll {
				return p, func() tea.Msg {
					return requestClearAllTasksMsg{RepoID: p.repoID}
				}
			}
			return p, func() tea.Msg {
				return requestDeleteTaskMsg{TaskID: p.taskID}
			}
		case "n", "N", "q", "esc", "ctrl+c":
			return p, func() tea.Msg {
				return CloseTaskDeleteConfirmMsg{ID: p.id}
			}
		}
	}
	return p, nil
}

func (p *taskDeleteConfirmPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 64
	}

	var b strings.Builder

	if p.clearAll {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Warning).Render("Clear All Tasks?"))
		b.WriteByte('\n')
		b.WriteByte('\n')
		b.WriteString("This will permanently delete ")
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("all tasks"))
		b.WriteString(" in the current repository.")
	} else {
		title := p.taskTitle
		if title == "" {
			title = p.taskID
		}
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Warning).Render("Delete Task?"))
		b.WriteByte('\n')
		b.WriteByte('\n')
		b.WriteString("Task: ")
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(title))
		b.WriteByte('\n')
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("Child steps will also be deleted."))
	}
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("y confirm · n cancel"))

	return tea.NewView(appstyles.StyleCache.MaxWidth(width).Render(b.String()))
}

func (p *taskDeleteConfirmPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

type requestDeleteTaskMsg struct {
	TaskID string
}

type requestClearAllTasksMsg struct {
	RepoID string
}
