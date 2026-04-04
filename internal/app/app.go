package app

import (
	"strings"

	"focus/internal/avatar"
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/styles"
	"focus/internal/ui/footer"
	"focus/internal/ui/header"
	"focus/internal/ui/layout"
	"focus/internal/ui/pomodoro"
	"focus/internal/ui/todo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	panelHeader = iota
	panelTodo
	panelPomodoro
	panelFooter
	numPanels
)

// avatarRenderedMsg carries the pre-rendered avatar string from chafa.
type avatarRenderedMsg struct {
	art string
}

// model is the top-level Bubbletea model.
type model struct {
	common         *models.CommonModel
	state          AppState
	mode           AppMode
	focused        FocusedPanel
	children       []models.Panel
	avatarRendered string // cached chafa output
}

// New creates and returns the initial application model.
func New(cfg config.Config, store models.Store) tea.Model {
	cm := &models.CommonModel{
		Theme: styles.DefaultTheme(),
		Cfg:   cfg,
		Store: store,
	}
	m := model{
		common:  cm,
		state:   StateDashboard,
		mode:    ModeNormal,
		focused: FocusTodo,
		children: []models.Panel{
			header.New(cfg, cm.Theme), // panelHeader
			todo.New(cm),              // panelTodo
			pomodoro.New(cm),          // panelPomodoro
			footer.New(cm),            // panelFooter
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
	// Render avatar in background.
	cfg := m.common.Cfg
	if cfg.Avatar.Image != "" {
		cmds = append(cmds, func() tea.Msg {
			art := avatar.Render(cfg.Avatar.Image, cfg.Avatar.Width)
			return avatarRenderedMsg{art: art}
		})
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case avatarRenderedMsg:
		m.avatarRendered = msg.art
		return m, nil

	case tea.KeyMsg:
		pomo := m.children[panelPomodoro].(*pomodoro.Model)

		// When picker is active, all keys go to pomodoro.
		if pomo.IsPickerActive() {
			newChild, cmd := m.children[panelPomodoro].Update(msg)
			m.children[panelPomodoro] = newChild
			return m, cmd
		}

		if m.mode == ModeNormal {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "tab":
				// Toggle panel focus.
				if m.focused == FocusTodo {
					m.focused = FocusPomodoro
				} else {
					m.focused = FocusTodo
				}
				return m, nil
			case "shift+tab":
				// Forward to todo for list switching.
				newChild, cmd := m.children[panelTodo].Update(msg)
				m.children[panelTodo] = newChild
				return m, cmd
			default:
				// Route to focused panel.
				if m.focused == FocusTodo {
					newChild, cmd := m.children[panelTodo].Update(msg)
					m.children[panelTodo] = newChild
					return m, cmd
				}
				newChild, cmd := m.children[panelPomodoro].Update(msg)
				m.children[panelPomodoro] = newChild
				return m, cmd
			}
		} else if m.mode == ModeInput {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			// Input mode: keys go to todo (for textinput).
			newChild, cmd := m.children[panelTodo].Update(msg)
			m.children[panelTodo] = newChild
			return m, cmd
		}

	case todo.ModeChangeMsg:
		if msg.InputActive {
			m.mode = ModeInput
		} else {
			m.mode = ModeNormal
		}

	case pomodoro.SessionCompleteMsg, models.StatsRefreshMsg:
		if ft, ok := m.children[panelFooter].(*footer.Model); ok {
			return m, ft.Refresh()
		}

	case tea.WindowSizeMsg:
		m.common.Width = msg.Width
		m.common.Height = msg.Height
		m.updateSizes(msg.Width, msg.Height)
	}

	// Non-key messages go to all panels.
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

func (m *model) updateSizes(w, h int) {
	dims := layout.Compute(w, h)

	m.children[panelHeader].SetSize(w, headerLines(dims))

	if dims.TwoCol {
		m.children[panelTodo].SetSize(dims.LeftW, dims.ContentH)
		m.children[panelPomodoro].SetSize(dims.RightW, dims.ContentH)
	} else {
		m.children[panelTodo].SetSize(dims.LeftW, dims.TopH)
		m.children[panelPomodoro].SetSize(dims.RightW, dims.BottomH)
	}
	m.children[panelFooter].SetSize(w, 1)
}

func headerLines(d layout.Dimensions) int {
	if d.ShowQuote {
		return 2
	}
	return 1
}

// View implements tea.Model.
func (m model) View() string {
	if len(m.children) < numPanels {
		return ""
	}

	w := m.common.Width
	h := m.common.Height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	dims := layout.Compute(w, h)

	hdr := m.children[panelHeader].(*header.Model)
	pomo := m.children[panelPomodoro].(*pomodoro.Model)
	todoModel := m.children[panelTodo].(*todo.Model)

	// 1. Header (weather + time, optional quote).
	headerView := hdr.ViewCompact(w, dims.ShowQuote)

	// 2. Todo panel content and title.
	todoContent := m.children[panelTodo].View()
	todoTitle := "TODO [today]"
	if todoModel.ActiveList() == models.ListSomeday {
		todoTitle = "TODO [someday]"
	}

	// 3. Pomodoro panel content and title.
	pomoContent := m.children[panelPomodoro].View()
	pomoTitle := "POMODORO"
	if pomo.IsPickerActive() {
		pomoTitle = "POMODORO - Select"
	}

	// 4. Build panel area.
	var panelArea string
	if dims.TwoCol {
		todoPanel := layout.RenderPanel(todoTitle, todoContent, dims.LeftW, dims.ContentH, m.focused == FocusTodo)
		pomoPanel := layout.RenderPanel(pomoTitle, pomoContent, dims.RightW, dims.ContentH, m.focused == FocusPomodoro)
		panelArea = lipgloss.JoinHorizontal(lipgloss.Top, todoPanel, pomoPanel)
	} else {
		todoPanel := layout.RenderPanel(todoTitle, todoContent, dims.LeftW, dims.TopH, m.focused == FocusTodo)
		pomoPanel := layout.RenderPanel(pomoTitle, pomoContent, dims.RightW, dims.BottomH, m.focused == FocusPomodoro)
		panelArea = lipgloss.JoinVertical(lipgloss.Left, todoPanel, pomoPanel)
	}

	// 5. Footer stats.
	statsView := m.children[panelFooter].View()

	// 6. Context-aware help bar.
	helpLine := m.renderHelpLine(w)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		panelArea,
		statsView,
		helpLine,
	)
}

// renderHelpLine creates a context-aware help bar.
func (m model) renderHelpLine(w int) string {
	helpStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	pomo := m.children[panelPomodoro].(*pomodoro.Model)
	todoModel := m.children[panelTodo].(*todo.Model)

	// Input mode.
	if m.mode == ModeInput {
		return helpStyle.Render("  [enter]confirm  [esc]cancel")
	}

	var left string

	// Left side: focused panel commands.
	switch m.focused {
	case FocusTodo:
		if todoModel.IsConfirmingDelete() {
			left = "[y]es  [n]o — Delete?"
		} else {
			left = "[a]dd [e]dit [d]el [space]done [shift+tab]list"
		}
	case FocusPomodoro:
		if pomo.IsPickerActive() {
			left = "[enter]select  [esc]skip"
		} else if pomo.CurrentPhase() == pomodoro.PhaseIdle {
			left = "[s]tart"
		} else if pomo.IsPaused() {
			left = "[p]resume [r]eset"
		} else {
			left = "[p]ause [n]ext [r]eset"
		}
	}

	// Right side: global commands.
	right := "[tab]switch  [q]uit"

	leftRendered := helpStyle.Render("  " + left)
	rightRendered := helpStyle.Render(right + "  ")

	gap := w - lipgloss.Width(leftRendered) - lipgloss.Width(rightRendered)
	if gap < 1 {
		gap = 1
	}

	return leftRendered + strings.Repeat(" ", gap) + rightRendered
}
