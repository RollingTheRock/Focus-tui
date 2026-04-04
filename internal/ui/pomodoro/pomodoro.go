package pomodoro

import (
	"fmt"
	"strings"
	"time"

	"focus/internal/models"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Phase represents the current pomodoro phase.
type Phase int

const (
	PhaseIdle Phase = iota
	PhaseWork
	PhaseShortBreak
	PhaseLongBreak
)

func (p Phase) String() string {
	switch p {
	case PhaseWork:
		return "Working"
	case PhaseShortBreak:
		return "Short Break"
	case PhaseLongBreak:
		return "Long Break"
	default:
		return "Idle"
	}
}

// Messages sent to parent app.
type (
	// SessionCompleteMsg is emitted when a pomodoro work phase completes.
	SessionCompleteMsg struct{}
)

// TodoSelectedMsg is sent by the app when a todo is picked (or skipped).
type TodoSelectedMsg struct {
	TodoID   *int
	TodoText string
}

// tickMsg is a per-second timer tick.
type tickMsg time.Time

// Model is the Pomodoro panel sub-model.
type Model struct {
	common *models.CommonModel
	width  int
	height int

	phase     Phase
	remaining time.Duration
	paused    bool
	running   bool // true when timer is active (not idle)

	pomodoroCount int // completed work phases in current cycle
	sessionID     int64

	linkedTodoID   *int
	linkedTodoText string

	// Picker state.
	pickerActive bool
	pickerItems  []models.Todo
	pickerCursor int
}

// New creates a new Pomodoro panel.
func New(common *models.CommonModel) models.Panel {
	return &Model{
		common: common,
		phase:  PhaseIdle,
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	if m.pickerActive {
		return m.updatePicker(msg)
	}

	switch msg := msg.(type) {
	case pickerLoadedMsg:
		// Picker data arrived — delegate to picker handler.
		m.pickerActive = true
		return m.updatePicker(msg)

	case tickMsg:
		if !m.running || m.paused {
			return m, nil
		}
		m.remaining -= time.Second
		if m.remaining <= 0 {
			return m, m.phaseComplete()
		}
		return m, tickCmd()

	case TodoSelectedMsg:
		m.pickerActive = false
		m.linkedTodoID = msg.TodoID
		m.linkedTodoText = msg.TodoText
		return m, m.startTimer()

	case tea.KeyMsg:
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m *Model) View() string {
	w := m.width
	if w <= 0 {
		w = 40
	}
	h := m.height
	if h <= 0 {
		h = 20
	}

	if m.pickerActive {
		return m.viewPicker(w, h)
	}

	var lines []string

	// Timer display.
	mins := int(m.remaining.Minutes())
	secs := int(m.remaining.Seconds()) % 60
	timerStr := fmt.Sprintf("🍅 %02d:%02d", mins, secs)
	if m.phase == PhaseIdle {
		timerStr = "🍅 --:--"
	}
	timerStyle := lipgloss.NewStyle().Bold(true)
	switch {
	case m.paused:
		timerStyle = timerStyle.Foreground(styles.Warning)
	case m.phase == PhaseShortBreak || m.phase == PhaseLongBreak:
		timerStyle = timerStyle.Foreground(styles.Success)
	default:
		timerStyle = timerStyle.Foreground(styles.Accent)
	}
	lines = append(lines, timerStyle.Render(timerStr))
	lines = append(lines, "")

	// Phase label.
	phaseLabel := m.phase.String()
	if m.paused {
		phaseLabel += " (paused)"
	}
	phaseStyle := lipgloss.NewStyle().Foreground(styles.Text)
	lines = append(lines, phaseStyle.Render(phaseLabel))

	// Linked todo.
	if m.linkedTodoText != "" {
		todoStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
		lines = append(lines, todoStyle.Render("→ "+m.linkedTodoText))
	}

	lines = append(lines, "")

	// Pomodoro count dots.
	if m.pomodoroCount > 0 || m.running {
		interval := m.common.Cfg.Pomodoro.LongBreakInterval
		if interval <= 0 {
			interval = 4
		}
		dots := ""
		for i := 0; i < interval; i++ {
			if i < m.pomodoroCount%interval {
				dots += "● "
			} else {
				dots += "○ "
			}
		}
		dotStyle := lipgloss.NewStyle().Foreground(styles.Accent)
		cycleLabel := lipgloss.NewStyle().Foreground(styles.Subtle).Render("  " + m.CycleLabel())
		lines = append(lines, dotStyle.Render(dots)+cycleLabel)
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// --- Exported getters for app-level layout ---

// TimerDisplay returns the formatted timer string, e.g. "25:00" or "--:--".
func (m *Model) TimerDisplay() string {
	if m.phase == PhaseIdle {
		return "--:--"
	}
	mins := int(m.remaining.Minutes())
	secs := int(m.remaining.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

// PhaseLabel returns the current phase as a display string.
func (m *Model) PhaseLabel() string {
	label := m.phase.String()
	if m.paused {
		label += " (paused)"
	}
	return label
}

// PomoDots returns the pomodoro cycle progress as filled/empty dots.
func (m *Model) PomoDots() string {
	interval := m.common.Cfg.Pomodoro.LongBreakInterval
	if interval <= 0 {
		interval = 4
	}
	var dots string
	for i := 0; i < interval; i++ {
		if i > 0 {
			dots += " "
		}
		if i < m.pomodoroCount%interval {
			dots += "●"
		} else {
			dots += "○"
		}
	}
	return dots
}

// CycleLabel returns a label like "Cycle 2/4".
func (m *Model) CycleLabel() string {
	interval := m.common.Cfg.Pomodoro.LongBreakInterval
	if interval <= 0 {
		interval = 4
	}
	current := m.pomodoroCount%interval + 1
	if m.phase == PhaseIdle && m.pomodoroCount == 0 {
		current = 0
	}
	return fmt.Sprintf("Cycle %d/%d", current, interval)
}

// IsPickerActive returns true when the todo picker overlay is showing.
func (m *Model) IsPickerActive() bool {
	return m.pickerActive
}

// PickerView returns the rendered picker view.
func (m *Model) PickerView() string {
	return m.viewPicker(m.width, m.height)
}

// LinkedTodoText returns the currently linked todo text.
func (m *Model) LinkedTodoText() string {
	return m.linkedTodoText
}

// IsRunning returns true when the timer is active.
func (m *Model) IsRunning() bool {
	return m.running
}

// IsPaused returns true when the timer is paused.
func (m *Model) IsPaused() bool {
	return m.paused
}

// CurrentPhase returns the current phase.
func (m *Model) CurrentPhase() Phase {
	return m.phase
}

// --- Normal mode keys ---

func (m *Model) updateNormal(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.String() {
	case "s":
		if m.phase == PhaseIdle {
			// Start: show todo picker.
			return m, m.loadPickerTodos()
		}
	case "p":
		if m.running {
			m.paused = !m.paused
			if !m.paused {
				return m, tickCmd()
			}
		}
	case "n":
		if m.running && !m.paused {
			return m, m.skipPhase()
		}
	case "r":
		if m.running {
			m.reset()
		}
	}
	return m, nil
}

// --- Todo Picker ---

type pickerLoadedMsg struct {
	items []models.Todo
}

func (m *Model) loadPickerTodos() tea.Cmd {
	return func() tea.Msg {
		items, err := m.common.Store.ListTodos(models.ListToday)
		if err != nil {
			return pickerLoadedMsg{}
		}
		// Filter out done/overdue.
		var active []models.Todo
		for _, t := range items {
			if t.Status == models.StatusTodo {
				active = append(active, t)
			}
		}
		return pickerLoadedMsg{items: active}
	}
}

func (m *Model) updatePicker(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case pickerLoadedMsg:
		m.pickerItems = msg.items
		m.pickerCursor = 0
		m.pickerActive = true
		if len(m.pickerItems) == 0 {
			// No active todos — start without linking.
			m.pickerActive = false
			return m, m.startTimer()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.pickerCursor < len(m.pickerItems)-1 {
				m.pickerCursor++
			}
		case "k", "up":
			if m.pickerCursor > 0 {
				m.pickerCursor--
			}
		case "enter":
			if len(m.pickerItems) > 0 {
				t := m.pickerItems[m.pickerCursor]
				m.pickerActive = false
				m.linkedTodoID = &t.ID
				m.linkedTodoText = t.Text
				return m, m.startTimer()
			}
		case "esc":
			m.pickerActive = false
			m.linkedTodoID = nil
			m.linkedTodoText = ""
			return m, m.startTimer()
		}
	}
	return m, nil
}

func (m *Model) viewPicker(w, h int) string {
	var b strings.Builder

	if len(m.pickerItems) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.Subtle).Render("Loading..."))
	} else {
		for i, item := range m.pickerItems {
			cursor := "  "
			if i == m.pickerCursor {
				cursor = "▸ "
			}
			st := lipgloss.NewStyle().Foreground(styles.Text)
			if i == m.pickerCursor {
				st = st.Background(styles.Highlight)
			}
			b.WriteString(cursor + st.Render(item.Text))
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(styles.Subtle).Render("[enter] select  [esc] skip"))

	return b.String()
}

// --- Timer logic ---

func (m *Model) startTimer() tea.Cmd {
	cfg := m.common.Cfg.Pomodoro
	m.phase = PhaseWork
	m.remaining = time.Duration(cfg.WorkMinutes) * time.Minute
	m.paused = false
	m.running = true

	// Record session start.
	return func() tea.Msg {
		id, _ := m.common.Store.StartSession(m.linkedTodoID)
		m.sessionID = id
		return tickMsg(time.Now())
	}
}

func (m *Model) phaseComplete() tea.Cmd {
	cfg := m.common.Cfg.Pomodoro
	interval := cfg.LongBreakInterval
	if interval <= 0 {
		interval = 4
	}

	var cmds []tea.Cmd

	switch m.phase {
	case PhaseWork:
		m.pomodoroCount++
		// Complete the session in DB.
		sessionID := m.sessionID
		cmds = append(cmds, func() tea.Msg {
			_ = m.common.Store.CompleteSession(sessionID)
			return SessionCompleteMsg{}
		})
		cmds = append(cmds, sendStatsRefresh())
		cmds = append(cmds, m.sendNotification("Pomodoro complete! Take a break."))

		if m.pomodoroCount%interval == 0 {
			m.phase = PhaseLongBreak
			m.remaining = time.Duration(cfg.LongBreakMinutes) * time.Minute
		} else {
			m.phase = PhaseShortBreak
			m.remaining = time.Duration(cfg.ShortBreakMinutes) * time.Minute
		}
		cmds = append(cmds, tickCmd())

	case PhaseShortBreak, PhaseLongBreak:
		cmds = append(cmds, m.sendNotification("Break over! Ready to focus?"))
		// Go back to idle — user presses 's' to start next pomodoro.
		m.reset()
	}

	return tea.Batch(cmds...)
}

func (m *Model) skipPhase() tea.Cmd {
	m.remaining = 0
	return m.phaseComplete()
}

func (m *Model) reset() {
	m.phase = PhaseIdle
	m.remaining = 0
	m.paused = false
	m.running = false
	m.linkedTodoID = nil
	m.linkedTodoText = ""
}

func tickCmd() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func sendStatsRefresh() tea.Cmd {
	return func() tea.Msg {
		return models.StatsRefreshMsg{}
	}
}

func (m *Model) sendNotification(message string) tea.Cmd {
	if !m.common.Cfg.Pomodoro.Notify {
		return nil
	}
	return func() tea.Msg {
		notify(message)
		return nil
	}
}
