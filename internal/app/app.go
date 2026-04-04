package app

import (
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/styles"
	"focus/internal/ui/header"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// model is the top-level Bubbletea model.
type model struct {
	common   *models.CommonModel
	state    AppState
	mode     AppMode
	children []models.Panel
}

// New creates and returns the initial application model.
func New(cfg config.Config) tea.Model {
	cm := &models.CommonModel{
		Theme: styles.DefaultTheme(),
		Cfg:   cfg,
	}
	m := model{
		common: cm,
		state:  StateDashboard,
		mode:   ModeNormal,
		children: []models.Panel{
			header.New(cfg, cm.Theme),
		},
	}
	return m
}

// Init implements tea.Model.
func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, child := range m.children {
		if cmd := child.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode == ModeNormal {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.common.Width = msg.Width
		m.common.Height = msg.Height
		for _, child := range m.children {
			child.SetSize(msg.Width, msg.Height)
		}
	}

	var cmds []tea.Cmd
	for i, child := range m.children {
		newChild, cmd := child.Update(msg)
		m.children[i] = newChild
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m model) View() string {
	if len(m.children) == 0 {
		return ""
	}

	headerView := m.children[0].View()
	headerHeight := lipgloss.Height(headerView)
	bodyHeight := m.common.Height - headerHeight
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	bodyView := lipgloss.Place(
		m.common.Width,
		bodyHeight,
		lipgloss.Center,
		lipgloss.Center,
		m.common.Theme.SubtleStyle.Render("[Dashboard Placeholder]"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, headerView, bodyView)
}
