package app

import (
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseTaskArchiveConfirmMsg closes the task archive confirmation overlay.
type CloseTaskArchiveConfirmMsg struct {
	ID models.PaneID
}

type taskArchiveConfirmPane struct {
	id        models.PaneID
	taskID    string
	taskTitle string
	state     string
	width     int
	height    int
}

func newTaskArchiveConfirmPane(id models.PaneID, taskID, taskTitle, state string) *taskArchiveConfirmPane {
	return &taskArchiveConfirmPane{
		id:        id,
		taskID:    taskID,
		taskTitle: taskTitle,
		state:     state,
	}
}

func (p *taskArchiveConfirmPane) Init() tea.Cmd {
	return nil
}

func (p *taskArchiveConfirmPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			return p, func() tea.Msg {
				return requestArchiveTaskMsg{TaskID: p.taskID}
			}
		case "n", "N", "q", "esc", "ctrl+c":
			return p, func() tea.Msg {
				return CloseTaskArchiveConfirmMsg{ID: p.id}
			}
		}
	}
	return p, nil
}

func (p *taskArchiveConfirmPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 64
	}

	var b strings.Builder

	title := p.taskTitle
	if title == "" {
		title = p.taskID
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Warning).Render("Archive Task?"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString("Task: ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(title))
	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("Current state: " + p.state))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("y confirm · n cancel"))

	return tea.NewView(lipgloss.NewStyle().MaxWidth(width).Render(b.String()))
}

func (p *taskArchiveConfirmPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

type requestArchiveTaskMsg struct {
	TaskID string
}
