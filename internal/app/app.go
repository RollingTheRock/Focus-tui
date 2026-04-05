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
	"focus/internal/ui/shell"
	"focus/internal/ui/todo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	panelHeader = iota
	panelTodo
	panelPomodoro
	panelFooter
	panelShell
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
	overlay        OverlayKind
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
		mode:    ModeShell,
		focused: FocusShell,
		overlay: OverlayNone,
		children: []models.Panel{
			header.New(cfg, cm.Theme), // panelHeader
			todo.New(cm),              // panelTodo
			pomodoro.New(cm),          // panelPomodoro
			footer.New(cm),            // panelFooter
			shell.New(cm),             // panelShell
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
		return m.handleKey(msg)

	case pomodoro.PickerLoadedMsg:
		m.overlay = OverlayPicker
		// Forward to pomodoro panel.
		newChild, cmd := m.children[panelPomodoro].Update(msg)
		m.children[panelPomodoro] = newChild
		return m, cmd

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

// handleKey routes keyboard input based on overlay and mode state.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pomo := m.children[panelPomodoro].(*pomodoro.Model)

	// 1. Pomodoro picker overlay — all keys to pomodoro.
	if m.overlay == OverlayPicker {
		if pomo.IsPickerActive() {
			newChild, cmd := m.children[panelPomodoro].Update(msg)
			m.children[panelPomodoro] = newChild
			// Check if picker closed itself.
			if !m.children[panelPomodoro].(*pomodoro.Model).IsPickerActive() {
				m.overlay = OverlayNone
			}
			return m, cmd
		}
		// Picker was closed by a non-key message; dismiss overlay.
		m.overlay = OverlayNone
		return m, nil
	}

	// 2. Todo overlay — keys to todo panel.
	if m.overlay == OverlayTodo {
		if m.mode == ModeInput {
			// Text input mode: all keys to todo.
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			newChild, cmd := m.children[panelTodo].Update(msg)
			m.children[panelTodo] = newChild
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+t", "esc":
			m.overlay = OverlayNone
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			newChild, cmd := m.children[panelTodo].Update(msg)
			m.children[panelTodo] = newChild
			return m, cmd
		}
	}

	// 3. Shell mode — keys to shell, except escape and ctrl+t.
	if m.mode == ModeShell {
		switch msg.String() {
		case "esc":
			m.mode = ModeNormal
			return m, nil
		case "ctrl+t":
			m.overlay = OverlayTodo
			m.mode = ModeNormal
			return m, nil
		default:
			newChild, cmd := m.children[panelShell].Update(msg)
			m.children[panelShell] = newChild
			return m, cmd
		}
	}

	// 4. Normal mode — global shortcuts.
	switch msg.String() {
	case "q", "ctrl+c":
		if sh, ok := m.children[panelShell].(*shell.Model); ok {
			sh.Close()
		}
		return m, tea.Quit
	case "ctrl+t":
		m.overlay = OverlayTodo
		return m, nil
	case "enter":
		m.mode = ModeShell
		return m, nil
	case "s", "p", "n", "r":
		// Pomodoro controls.
		newChild, cmd := m.children[panelPomodoro].Update(msg)
		m.children[panelPomodoro] = newChild
		return m, cmd
	}

	return m, nil
}

func (m *model) updateSizes(w, h int) {
	dims := layout.ComputeBanner(w, h)

	m.children[panelHeader].SetSize(w, dims.HeaderH)
	m.children[panelShell].SetSize(dims.ShellContentW, dims.ShellContentH)
	m.children[panelFooter].SetSize(w, 1)

	// Todo and pomodoro get overlay dimensions.
	m.children[panelTodo].SetSize(dims.OverlayW, dims.OverlayH)
	m.children[panelPomodoro].SetSize(dims.OverlayW, dims.OverlayH)
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

	dims := layout.ComputeBanner(w, h)

	hdr := m.children[panelHeader].(*header.Model)
	pomo := m.children[panelPomodoro].(*pomodoro.Model)

	// 1. Header.
	var headerView string
	if dims.UseBanner {
		pomoTimer := ""
		pomoPhase := ""
		linkedTodo := ""
		if pomo.IsRunning() {
			pomoTimer = pomo.TimerDisplay()
			pomoPhase = pomo.PhaseLabel()
			linkedTodo = pomo.LinkedTodoText()
			if linkedTodo != "" {
				linkedTodo = "-> " + linkedTodo
			}
		}
		headerView = hdr.ViewBanner(w, pomoTimer, pomoPhase, linkedTodo)
	} else {
		headerView = hdr.ViewCompact(w, dims.ShowQuote)
	}

	// 2. Shell panel (full width).
	shellContent := m.children[panelShell].View()
	shellTitle := "SHELL"
	if m.mode == ModeShell {
		shellTitle = "SHELL [active]"
	}
	shellPanel := layout.RenderPanel(shellTitle, shellContent,
		dims.ShellContentW, dims.ShellContentH, m.mode == ModeShell)

	// 3. Overlay compositing.
	panelArea := shellPanel
	if m.overlay == OverlayTodo {
		todoContent := m.children[panelTodo].View()
		todoModel := m.children[panelTodo].(*todo.Model)
		todoTitle := "TODO [today]"
		if todoModel.ActiveList() == models.ListSomeday {
			todoTitle = "TODO [someday]"
		}
		overlayPanel := layout.RenderPanel(todoTitle, todoContent,
			dims.OverlayW, dims.OverlayH, true)
		panelArea = layout.OverlayOnBase(shellPanel, overlayPanel, dims.OverlayX, dims.OverlayY)
	} else if m.overlay == OverlayPicker {
		pickerContent := pomo.PickerView()
		overlayPanel := layout.RenderPanel("SELECT TASK", pickerContent,
			dims.OverlayW, dims.OverlayH, true)
		panelArea = layout.OverlayOnBase(shellPanel, overlayPanel, dims.OverlayX, dims.OverlayY)
	}

	// 4. Footer stats.
	statsView := m.children[panelFooter].View()

	// 5. Context-aware help bar.
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

	// Overlay: picker.
	if m.overlay == OverlayPicker {
		return helpStyle.Render("  [enter]select  [esc]skip")
	}

	// Overlay: todo.
	if m.overlay == OverlayTodo {
		if m.mode == ModeInput {
			return helpStyle.Render("  [enter]confirm  [esc]cancel")
		}
		var left string
		if todoModel.IsConfirmingDelete() {
			left = "[y]es  [n]o -- Delete?"
		} else {
			left = "[a]dd [e]dit [d]el [space]done [shift+tab]list"
		}
		right := "[ctrl+t/esc]close"
		return renderHelpBar(helpStyle, left, right, w)
	}

	// Shell active mode.
	if m.mode == ModeShell {
		return helpStyle.Render("  [esc]normal  [ctrl+t]todo")
	}

	// Normal mode.
	var left string
	if pomo.CurrentPhase() == pomodoro.PhaseIdle {
		left = "[s]tart pomo"
	} else if pomo.IsPaused() {
		left = "[p]resume [r]eset"
	} else if pomo.IsRunning() {
		left = "[p]ause [n]ext [r]eset"
	}

	right := "[ctrl+t]todo  [enter]shell  [q]uit"
	return renderHelpBar(helpStyle, left, right, w)
}

func renderHelpBar(helpStyle lipgloss.Style, left, right string, w int) string {
	leftRendered := helpStyle.Render("  " + left)
	rightRendered := helpStyle.Render(right + "  ")

	gap := w - lipgloss.Width(leftRendered) - lipgloss.Width(rightRendered)
	if gap < 1 {
		gap = 1
	}

	return leftRendered + strings.Repeat(" ", gap) + rightRendered
}
