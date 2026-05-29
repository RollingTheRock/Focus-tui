package app

import (
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CloseDAGMiniOverlayMsg closes the DAG mini overlay.
type CloseDAGMiniOverlayMsg struct {
	ID models.PaneID
}

type dagMiniOverlayPane struct {
	id         models.PaneID
	phaseID    string
	phaseTitle string
	steps      []models.TaskContextRecord
	width      int
	height     int
}

func newDAGMiniOverlayPane(id models.PaneID, phaseID, phaseTitle string, steps []models.TaskContextRecord) *dagMiniOverlayPane {
	return &dagMiniOverlayPane{
		id:         id,
		phaseID:    phaseID,
		phaseTitle: phaseTitle,
		steps:      steps,
	}
}

func (p *dagMiniOverlayPane) Init() tea.Cmd {
	return nil
}

func (p *dagMiniOverlayPane) ID() models.PaneID {
	return p.id
}

func (p *dagMiniOverlayPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return p, func() tea.Msg {
				return CloseDAGMiniOverlayMsg{ID: p.id}
			}
		}
	}
	return p, nil
}

func (p *dagMiniOverlayPane) View() string {
	width := p.width
	if width <= 0 {
		width = 64
	}

	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent).Render("Phase: " + p.phaseTitle))
	b.WriteByte('\n')
	b.WriteByte('\n')

	if len(p.steps) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("  No steps yet."))
	} else {
		for i, step := range p.steps {
			prefix := "├── "
			if i == len(p.steps)-1 {
				prefix = "└── "
			}
			stateStr := step.State
			if stateStr == "" {
				stateStr = "active"
			}
			line := prefix + step.Title + "   [" + stateStr + "]"
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("[esc] or [q] close"))

	return appstyles.StyleCache.MaxWidth(width).Render(b.String())
}

func (p *dagMiniOverlayPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}
