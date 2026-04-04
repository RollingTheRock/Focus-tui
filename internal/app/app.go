package app

import (
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/styles"
	"focus/internal/ui/header"
	"focus/internal/ui/todo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	panelHeader = iota
	panelTodo
	numPanels
)

// model is the top-level Bubbletea model.
type model struct {
	common   *models.CommonModel
	state    AppState
	mode     AppMode
	children []models.Panel
}

// New creates and returns the initial application model.
func New(cfg config.Config, store models.Store) tea.Model {
	cm := &models.CommonModel{
		Theme: styles.DefaultTheme(),
		Cfg:   cfg,
		Store: store,
	}
	m := model{
		common: cm,
		state:  StateDashboard,
		mode:   ModeNormal,
		children: []models.Panel{
			header.New(cfg, cm.Theme), // panelHeader
			todo.New(cm),              // panelTodo
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
		} else if m.mode == ModeInput {
			// In input mode, only allow ctrl+c to quit.
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}

	case todo.ModeChangeMsg:
		if msg.InputActive {
			m.mode = ModeInput
		} else {
			m.mode = ModeNormal
		}

	case tea.WindowSizeMsg:
		m.common.Width = msg.Width
		m.common.Height = msg.Height
		// Header gets full width; todo gets left-half width.
		m.children[panelHeader].SetSize(msg.Width, msg.Height)
		halfW := msg.Width / 2
		m.children[panelTodo].SetSize(halfW, msg.Height)
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
	if len(m.children) < numPanels {
		return ""
	}

	w := m.common.Width
	if w <= 0 {
		w = 80
	}

	// Header — full width.
	headerView := m.children[panelHeader].View()
	headerHeight := lipgloss.Height(headerView)

	// Body area: Todo (left) + Pomodoro placeholder (right).
	bodyHeight := m.common.Height - headerHeight - 1 // -1 for separator
	if bodyHeight < 0 {
		bodyHeight = 0
	}

	halfW := w / 2

	// Todo panel (left).
	todoView := m.children[panelTodo].View()
	todoBox := lipgloss.NewStyle().
		Width(halfW).
		Height(bodyHeight).
		Render(todoView)

	// Pomodoro placeholder (right).
	pomodoroPlaceholder := lipgloss.Place(
		w-halfW,
		bodyHeight,
		lipgloss.Center,
		lipgloss.Center,
		m.common.Theme.SubtleStyle.Render("[ FOCUS ]\n🍅 --:--"),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top, todoBox, pomodoroPlaceholder)

	// Separator line.
	sep := lipgloss.NewStyle().
		Foreground(styles.Subtle).
		Render(repeatChar('─', w))

	return lipgloss.JoinVertical(lipgloss.Left, headerView, sep, body)
}

func repeatChar(ch rune, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]rune, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}
