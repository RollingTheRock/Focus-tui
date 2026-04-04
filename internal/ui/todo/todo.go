package todo

import (
	"strings"

	"focus/internal/models"
	"focus/internal/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages sent to the parent app.
type (
	// ModeChangeMsg requests a mode switch (Normal ↔ Input).
	ModeChangeMsg struct{ InputActive bool }
)

// Model is the Todo panel sub-model.
type Model struct {
	common     *models.CommonModel
	items      []models.Todo
	cursor     int
	activeList string // "today" or "someday"
	width      int
	height     int

	// Input mode state.
	input      textinput.Model
	inputMode  inputAction
	editID     int // ID of todo being edited

	// Delete confirmation.
	confirmDelete bool
}

type inputAction int

const (
	inputNone inputAction = iota
	inputAdd
	inputEdit
)

// New creates a new Todo panel.
func New(common *models.CommonModel) models.Panel {
	ti := textinput.New()
	ti.Placeholder = "What needs to be done?"
	ti.CharLimit = 120

	m := &Model{
		common:     common,
		activeList: models.ListToday,
		input:      ti,
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return m.loadTodos
}

func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	// Handle input mode first — all keys routed to textinput.
	if m.inputMode != inputNone {
		return m.updateInput(msg)
	}

	// Handle delete confirmation.
	if m.confirmDelete {
		return m.updateConfirm(msg)
	}

	switch msg := msg.(type) {
	case todosLoadedMsg:
		m.items = msg.items
		m.clampCursor()

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

	var b strings.Builder

	// Items.
	if len(m.items) == 0 {
		empty := lipgloss.NewStyle().Foreground(styles.Subtle).Render("No tasks yet. Press [a] to add one.")
		b.WriteString(empty)
	} else {
		for i, item := range m.items {
			b.WriteString(m.renderItem(i, item, w))
			if i < len(m.items)-1 {
				b.WriteByte('\n')
			}
		}
	}

	// Input / confirm bar.
	if m.inputMode != inputNone {
		b.WriteByte('\n')
		b.WriteString(m.input.View())
	} else if m.confirmDelete {
		b.WriteByte('\n')
		b.WriteString(lipgloss.NewStyle().Foreground(styles.Overdue).Render("Delete? [y/n]"))
	}

	return b.String()
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// ActiveList returns the currently active list name ("today" or "someday").
func (m *Model) ActiveList() string {
	return m.activeList
}

// IsConfirmingDelete returns true when a delete confirmation prompt is active.
func (m *Model) IsConfirmingDelete() bool {
	return m.confirmDelete
}

// --- Normal mode key handling ---

func (m *Model) updateNormal(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case " ":
		return m, m.toggleCurrent()
	case "a":
		m.inputMode = inputAdd
		m.input.SetValue("")
		m.input.Focus()
		return m, tea.Batch(textinput.Blink, sendModeChange(true))
	case "e":
		if cur := m.currentItem(); cur != nil {
			m.inputMode = inputEdit
			m.editID = cur.ID
			m.input.SetValue(cur.Text)
			m.input.Focus()
			return m, tea.Batch(textinput.Blink, sendModeChange(true))
		}
	case "d":
		if m.currentItem() != nil {
			m.confirmDelete = true
		}
	case "shift+tab":
		if m.activeList == models.ListToday {
			m.activeList = models.ListSomeday
		} else {
			m.activeList = models.ListToday
		}
		m.cursor = 0
		return m, m.loadTodos
	case "m":
		return m, m.moveCurrentToToday()
	}
	return m, nil
}

// --- Input mode ---

func (m *Model) updateInput(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			if text != "" {
				var cmd tea.Cmd
				if m.inputMode == inputAdd {
					cmd = m.addTodo(text)
				} else {
					cmd = m.editTodo(m.editID, text)
				}
				m.inputMode = inputNone
				m.input.Blur()
				return m, tea.Batch(cmd, sendModeChange(false))
			}
			m.inputMode = inputNone
			m.input.Blur()
			return m, sendModeChange(false)
		case "esc":
			m.inputMode = inputNone
			m.input.Blur()
			return m, sendModeChange(false)
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// --- Delete confirmation ---

func (m *Model) updateConfirm(msg tea.Msg) (models.Panel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "y":
			m.confirmDelete = false
			return m, m.deleteCurrent()
		default:
			m.confirmDelete = false
		}
	}
	return m, nil
}

// --- Rendering ---

func (m *Model) renderItem(idx int, item models.Todo, maxWidth int) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = "▸ "
	}

	var icon string
	var textStyle lipgloss.Style
	switch item.Status {
	case models.StatusDone:
		icon = "✓ "
		textStyle = lipgloss.NewStyle().Foreground(styles.Done).Strikethrough(true)
	case models.StatusOverdue:
		icon = "! "
		textStyle = lipgloss.NewStyle().Foreground(styles.Overdue)
	default:
		icon = "○ "
		textStyle = lipgloss.NewStyle().Foreground(styles.Text)
	}

	if idx == m.cursor {
		// Highlight row background for selected item.
		textStyle = textStyle.Background(styles.Highlight)
	}

	return cursor + icon + textStyle.Render(item.Text)
}

// --- Store commands ---

type todosLoadedMsg struct {
	items []models.Todo
}

func (m *Model) loadTodos() tea.Msg {
	items, err := m.common.Store.ListTodos(m.activeList)
	if err != nil {
		return todosLoadedMsg{}
	}
	return todosLoadedMsg{items: items}
}

func (m *Model) toggleCurrent() tea.Cmd {
	cur := m.currentItem()
	if cur == nil {
		return nil
	}
	id := cur.ID
	return tea.Batch(
		func() tea.Msg {
			_ = m.common.Store.ToggleTodo(id)
			return m.loadTodos()
		},
		sendStatsRefresh,
	)
}

func (m *Model) addTodo(text string) tea.Cmd {
	list := m.activeList
	return tea.Batch(
		func() tea.Msg {
			_, _ = m.common.Store.CreateTodo(text, list)
			return m.loadTodos()
		},
		sendStatsRefresh,
	)
}

func (m *Model) editTodo(id int, text string) tea.Cmd {
	return func() tea.Msg {
		_ = m.common.Store.UpdateTodoText(id, text)
		return m.loadTodos()
	}
}

func (m *Model) deleteCurrent() tea.Cmd {
	cur := m.currentItem()
	if cur == nil {
		return nil
	}
	id := cur.ID
	return tea.Batch(
		func() tea.Msg {
			_ = m.common.Store.DeleteTodo(id)
			return m.loadTodos()
		},
		sendStatsRefresh,
	)
}

func (m *Model) moveCurrentToToday() tea.Cmd {
	if m.activeList != models.ListSomeday {
		return nil
	}
	cur := m.currentItem()
	if cur == nil {
		return nil
	}
	id := cur.ID
	return tea.Batch(
		func() tea.Msg {
			_ = m.common.Store.MoveToToday(id)
			return m.loadTodos()
		},
		sendStatsRefresh,
	)
}

// --- Helpers ---

func (m *Model) currentItem() *models.Todo {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func sendModeChange(inputActive bool) tea.Cmd {
	return func() tea.Msg {
		return ModeChangeMsg{InputActive: inputActive}
	}
}

func sendStatsRefresh() tea.Msg {
	return models.StatsRefreshMsg{}
}
