package todo

import (
	"fmt"
	"strings"
	"time"

	"focus/internal/models"
	"focus/internal/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Messages sent to the parent app.
type (
	ModeChangeMsg     struct{ InputActive bool }
	OverlayVisibleMsg struct{ Visible bool }
	RefreshTodosMsg   struct{}
)

// Model is the Todo panel sub-model.
type Model struct {
	common     *models.CommonModel
	items      []models.Todo
	cursor     int
	activeList string // "today" or "someday"
	width      int
	height     int

	// Overlay visibility.
	visible bool

	// Timer state.
	runningTimers map[int]*runningTimer // keyed by todo ID
	sessionTimes  map[int]time.Duration // completed session time today per todo

	// Focus tracking (wall-clock based).
	focusSince             *time.Time
	totalWallClock         time.Duration // accumulated wall-clock focus time today
	focusReminderLevel     reminderLevel
	focusReminderDismissed bool
	lastTimerStopped       time.Time
	ticking                bool

	// Linked task context data.
	linkedTasks map[string]models.TaskContextRecord // keyed by task_id

	// Input mode state.
	input         textinput.Model
	inputMode     inputAction
	editID        int // ID of todo being edited
	confirmDelete bool

	// Bulk mode state.
	bulkMode bool
	marked   map[int]bool // keyed by todo ID
}

type inputAction int

const (
	inputNone inputAction = iota
	inputAdd
	inputEdit
)

type runningTimer struct {
	SessionID int64
	StartedAt time.Time
	Elapsed   time.Duration
}

type reminderLevel int

const (
	reminderNone reminderLevel = iota
	reminderSoft
	reminderHard
)

// Internal messages.
type tickMsg time.Time

type timerStartedMsg struct {
	TodoID    int
	SessionID int64
	At        time.Time
}

type timerStoppedMsg struct {
	TodoID    int
	SessionID int64
	Elapsed   time.Duration
}

type todosLoadedMsg struct {
	items []models.Todo
}

type linkedTasksLoadedMsg struct {
	tasks map[string]models.TaskContextRecord
}

type sessionTimesLoadedMsg struct {
	times map[int]time.Duration // todo ID -> session duration
}

// New creates a new Todo panel.
func New(common *models.CommonModel) models.Panel {
	ti := textinput.New()
	ti.Placeholder = "What needs to be done?"
	ti.CharLimit = 120

	m := &Model{
		common:        common,
		activeList:    models.ListToday,
		input:         ti,
		runningTimers: make(map[int]*runningTimer),
		sessionTimes:  make(map[int]time.Duration),
		linkedTasks:   make(map[string]models.TaskContextRecord),
		marked:        make(map[int]bool),
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return m.loadTodos
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	// Input mode first.
	if m.inputMode != inputNone {
		return m.updateInput(msg)
	}

	// Delete confirmation.
	if m.confirmDelete {
		return m.updateConfirm(msg)
	}

	// Focus reminder consumes break/continue keys.
	if m.focusReminderLevel != reminderNone {
		return m.updateReminder(msg)
	}

	switch msg := msg.(type) {
	case tickMsg:
		return m.updateTick()

	case timerStartedMsg:
		m.runningTimers[msg.TodoID] = &runningTimer{
			SessionID: msg.SessionID,
			StartedAt: msg.At,
			Elapsed:   0,
		}
		if m.focusSince == nil {
			now := time.Now()
			m.focusSince = &now
		}
		m.focusReminderLevel = reminderNone
		m.focusReminderDismissed = false
		m.lastTimerStopped = time.Time{}
		if !m.ticking {
			m.ticking = true
			return m, m.startTicking()
		}

	case timerStoppedMsg:
		delete(m.runningTimers, msg.TodoID)
		m.sessionTimes[msg.TodoID] += msg.Elapsed
		if len(m.runningTimers) == 0 {
			// Accumulate wall-clock time for this focus session.
			if m.focusSince != nil {
				m.totalWallClock += time.Since(*m.focusSince)
				m.focusSince = nil
			}
			m.lastTimerStopped = time.Now()
			m.ticking = false
		}
		return m, sendStatsRefresh

	case todosLoadedMsg:
		m.items = msg.items
		m.clampCursor()
		if len(m.marked) > 0 {
			valid := make(map[int]bool, len(m.items))
			for _, item := range m.items {
				valid[item.ID] = true
			}
			for id := range m.marked {
				if !valid[id] {
					delete(m.marked, id)
				}
			}
		}
		return m, m.loadLinkedTasks()

	case linkedTasksLoadedMsg:
		m.linkedTasks = msg.tasks
		return m, m.loadSessionTimes()

	case sessionTimesLoadedMsg:
		m.sessionTimes = msg.times

	case tea.KeyMsg:
		return m.updateNormal(msg)

	case RefreshTodosMsg:
		return m, m.loadTodos
	}
	return m, nil
}

// --- Tick ---

func (m *Model) startTicking() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) updateTick() (models.Panel, tea.Cmd) {
	if !m.ticking {
		return m, nil
	}
	anyRunning := false
	for _, rt := range m.runningTimers {
		rt.Elapsed += time.Second
		anyRunning = true
	}
	m.ticking = anyRunning

	// Focus reminder check (soft then hard).
	reminderMin := m.common.Cfg.Pomodoro.FocusReminderMinutes
	if reminderMin > 0 && m.focusSince != nil && !m.focusReminderDismissed {
		elapsed := time.Since(*m.focusSince)
		softAfter := time.Duration(reminderMin) * time.Minute
		hardAfter := softAfter + 30*time.Minute
		if elapsed >= hardAfter {
			m.focusReminderLevel = reminderHard
		} else if elapsed >= softAfter && m.focusReminderLevel == reminderNone {
			m.focusReminderLevel = reminderSoft
		}
	}

	// Auto-reset: all stopped >5 min.
	if !anyRunning && !m.lastTimerStopped.IsZero() && time.Since(m.lastTimerStopped) > 5*time.Minute {
		if m.focusSince != nil {
			m.totalWallClock += time.Since(*m.focusSince)
			m.focusSince = nil
		}
		m.focusReminderLevel = reminderNone
		m.focusReminderDismissed = false
		m.lastTimerStopped = time.Time{}
	}

	if m.ticking {
		return m, m.startTicking()
	}
	return m, nil
}

// --- Reminder ---

func (m *Model) updateReminder(msg tea.Msg) (models.Panel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "b":
			// Stop all timers, reset focus.
			var cmds []tea.Cmd
			for _, rt := range m.runningTimers {
				sid := rt.SessionID
				elapsed := rt.Elapsed
				cmds = append(cmds, func() tea.Msg {
					_ = m.common.Store.CompleteSession(sid)
					return timerStoppedMsg{TodoID: 0, SessionID: sid, Elapsed: elapsed}
				})
			}
			// We need to stop each running timer with proper info.
			// Build closures by iterating over items.
			cmds = nil
			for _, item := range m.items {
				if rt, ok := m.runningTimers[item.ID]; ok {
					todoID := item.ID
					sid := rt.SessionID
					elapsed := rt.Elapsed
					cmds = append(cmds, func() tea.Msg {
						_ = m.common.Store.CompleteSession(sid)
						return timerStoppedMsg{TodoID: todoID, SessionID: sid, Elapsed: elapsed}
					})
				}
			}
			m.focusReminderLevel = reminderNone
			m.focusReminderDismissed = true
			if m.focusSince != nil {
				m.totalWallClock += time.Since(*m.focusSince)
			}
			m.focusSince = nil
			m.ticking = false
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
			return m, nil

		case "c", "esc":
			// Dismiss reminder, reset tracking.
			m.focusReminderLevel = reminderNone
			m.focusReminderDismissed = true
			if m.focusSince != nil {
				m.totalWallClock += time.Since(*m.focusSince)
			}
			now := time.Now()
			m.focusSince = &now
			return m, nil
		}
	}
	return m, nil
}

// --- Normal mode keys ---

func (m *Model) updateNormal(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	if m.bulkMode {
		return m.updateBulkMode(msg)
	}

	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		return m, m.toggleCurrentTimer()
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
	case "g":
		return m, m.startCurrentTimer()
	case "s":
		return m, m.stopCurrentTimer()
	case "v":
		m.bulkMode = true
		m.marked = make(map[int]bool)
		return m, nil
	case "esc":
		m.visible = false
		return m, func() tea.Msg {
			return OverlayVisibleMsg{Visible: false}
		}
	}
	return m, nil
}

func (m *Model) updateBulkMode(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
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
		if cur := m.currentItem(); cur != nil {
			m.marked[cur.ID] = !m.marked[cur.ID]
			if !m.marked[cur.ID] {
				delete(m.marked, cur.ID)
			}
		}
	case "enter":
		return m, m.bulkToggleDoneCmd()
	case "m":
		return m, m.bulkMoveToTodayCmd()
	case "d", "backspace", "delete":
		return m, m.bulkDeleteCmd()
	case "v", "esc":
		m.bulkMode = false
		m.marked = make(map[int]bool)
	}
	return m, nil
}

// --- Timer controls ---

func (m *Model) startCurrentTimer() tea.Cmd {
	cur := m.currentItem()
	if cur == nil {
		return nil
	}
	if _, running := m.runningTimers[cur.ID]; running {
		return nil // already running
	}
	todoID := cur.ID
	return func() tea.Msg {
		linkedID := todoID
		id, err := m.common.Store.StartSession(&linkedID)
		if err != nil {
			return nil
		}
		return timerStartedMsg{TodoID: todoID, SessionID: id, At: time.Now()}
	}
}

func (m *Model) stopCurrentTimer() tea.Cmd {
	cur := m.currentItem()
	if cur == nil {
		return nil
	}
	rt, running := m.runningTimers[cur.ID]
	if !running {
		return nil
	}
	sessionID := rt.SessionID
	todoID := cur.ID
	elapsed := rt.Elapsed
	return func() tea.Msg {
		_ = m.common.Store.CompleteSession(sessionID)
		return timerStoppedMsg{TodoID: todoID, SessionID: sessionID, Elapsed: elapsed}
	}
}

func (m *Model) toggleCurrentTimer() tea.Cmd {
	cur := m.currentItem()
	if cur == nil {
		return nil
	}
	if _, running := m.runningTimers[cur.ID]; running {
		return m.stopCurrentTimer()
	}
	return m.startCurrentTimer()
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

// --- View ---

func (m *Model) View() string {
	w := m.width
	if w <= 0 {
		w = 50
	}
	contentW := w - 4
	if contentW < 30 {
		contentW = 30
	}

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	hintStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	sepStyle := lipgloss.NewStyle().Foreground(styles.DimBorder)
	metaStyle := lipgloss.NewStyle().Foreground(styles.Subtle)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)

	title := "Today's Focus"
	listHint := "[shift+tab] Someday"
	if m.activeList == models.ListSomeday {
		title = "Someday"
		listHint = "[shift+tab] Today"
	}

	totalDur := m.totalFocusTime()
	runningCount := len(m.runningTimers)
	progress := fmt.Sprintf("● %d  ✓ %d/%d  %s", runningCount, doneCount(m.items), len(m.items), formatDuration(totalDur))
	progress = ansi.Truncate(progress, max(8, contentW/2), "…")

	titleLeft := headerStyle.Render(title)
	titleRight := hintStyle.Render(progress)
	pad := contentW - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(titleLeft + strings.Repeat(" ", pad) + titleRight + "\n")
	b.WriteString(hintStyle.Render(listHint+"  ·  [enter]start/stop  [v]bulk mode  [a]quick add") + "\n")
	b.WriteString(sepStyle.Render(strings.Repeat("─", contentW)) + "\n")

	if m.bulkMode {
		b.WriteString(warnStyle.Render(fmt.Sprintf("SELECT MODE (%d marked)", len(m.marked))) + "\n")
	}

	// List area with viewport feedback.
	listRows := 6
	if m.height > 0 {
		usedRows := 8 // header, hint, separators, action bar/footer baseline
		if m.bulkMode {
			usedRows++
		}
		if m.focusReminderLevel != reminderNone {
			usedRows++
		}
		if m.confirmDelete {
			usedRows++
		}
		listRows = m.height - usedRows
		if listRows < 3 {
			listRows = 3
		}
	}

	if len(m.items) == 0 {
		empty := lipgloss.NewStyle().Foreground(styles.Subtle).Render("No items yet. [a] to add, or [t] in DAG to link a task.")
		b.WriteString(empty + "\n")
	} else {
		start, end := m.listViewport(listRows)
		pos := metaStyle.Render(fmt.Sprintf("position %d/%d", m.cursor+1, len(m.items)))
		b.WriteString(pos + "\n")
		if start > 0 {
			b.WriteString(metaStyle.Render(fmt.Sprintf("▲ %d hidden", start)) + "\n")
		}
		for i := start; i < end; i++ {
			item := m.items[i]
			b.WriteString(m.renderItem(i, item, contentW) + "\n")
		}
		if end < len(m.items) {
			b.WriteString(metaStyle.Render(fmt.Sprintf("▼ %d hidden", len(m.items)-end)) + "\n")
		}
	}

	if m.confirmDelete {
		if cur := m.currentItem(); cur != nil {
			b.WriteString(lipgloss.NewStyle().Foreground(styles.Overdue).Bold(true).Render(
				fmt.Sprintf("Delete \"%s\"? [y/n]", cur.Text),
			))
			b.WriteByte('\n')
		}
	}

	b.WriteString(sepStyle.Render(strings.Repeat("─", contentW)) + "\n")

	if m.focusReminderLevel != reminderNone {
		msg := "⚡ Focused for long time — [b]reak  [c]ontinue"
		if m.focusReminderLevel == reminderHard {
			msg = "⚠ Hard reminder: sustained focus — [b]reak now  [c]ontinue"
		}
		b.WriteString(warnStyle.Render(ansi.Truncate(msg, contentW, "…")) + "\n")
	}

	// Action bar (persistent quick input).
	actionTitle := "Quick Add"
	if m.inputMode == inputEdit {
		actionTitle = "Edit Item"
	}
	b.WriteString(headerStyle.Render(actionTitle) + "\n")
	if m.inputMode == inputAdd || m.inputMode == inputEdit {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.Accent).Render("[enter]save  [esc]cancel  ") + m.input.View() + "\n")
	} else {
		placeholder := metaStyle.Render("Press [a] to type a new todo...")
		b.WriteString(placeholder + "\n")
	}

	var footer string
	if m.bulkMode {
		footer = "[j/k]nav [space]mark [enter]done-toggle [m]move [d]delete [v/esc]exit bulk"
	} else {
		footer = "[j/k]nav [enter]timer [space]done [e]edit [d]del [v]bulk [a]add [esc]close"
	}
	b.WriteString(hintStyle.Render(ansi.Truncate(footer, contentW, "…")) + "\n")

	return b.String()
}

// --- Render helpers ---

func (m *Model) renderItem(idx int, item models.Todo, maxW int) string {
	isSelected := idx == m.cursor
	isRunning := false
	var elapsed time.Duration
	if rt, ok := m.runningTimers[item.ID]; ok {
		isRunning = true
		elapsed = rt.Elapsed
	}

	// Status icon.
	var icon string
	var iconStyle lipgloss.Style
	if isRunning {
		icon = "●"
		iconStyle = lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
	} else {
		switch item.Status {
		case models.StatusDone:
			icon = "✓"
			iconStyle = lipgloss.NewStyle().Foreground(styles.Done)
		case models.StatusOverdue:
			icon = "!"
			iconStyle = lipgloss.NewStyle().Foreground(styles.Overdue)
		default:
			icon = "○"
			iconStyle = lipgloss.NewStyle().Foreground(styles.Subtle)
		}
	}

	// Cursor / selection marker.
	cursor := "  "
	if m.bulkMode {
		marked := " "
		if m.marked[item.ID] {
			marked = "x"
		}
		cursor = "[" + marked + "]"
		if isSelected {
			cursor = "▸" + cursor
		} else {
			cursor = " " + cursor
		}
	} else if isSelected {
		cursor = "▸ "
	}

	// Task info (right side).
	taskInfo := "—"
	if item.TaskID != nil {
		if tc, ok := m.linkedTasks[*item.TaskID]; ok {
			taskInfo = tc.State + "/" + tc.Priority
		}
	}

	// Time display.
	total := m.sessionTimes[item.ID]
	if isRunning {
		total += elapsed
	}
	timeStr := "—"
	if total > 0 {
		timeStr = formatDuration(total)
	}

	// Text style.
	var textStyle lipgloss.Style
	if isRunning {
		textStyle = lipgloss.NewStyle().Foreground(styles.Text).Bold(true)
	} else if item.Status == models.StatusDone {
		textStyle = lipgloss.NewStyle().Foreground(styles.Done).Strikethrough(true)
	} else {
		textStyle = lipgloss.NewStyle().Foreground(styles.Text)
	}

	// Calculate available width for the text.
	leftFixed := lipgloss.Width(cursor) + lipgloss.Width(iconStyle.Render(icon)) + 1 // +1 for space after icon
	rightFixed := lipgloss.Width(lipgloss.NewStyle().Foreground(styles.Subtle).Render(taskInfo)) + 2 + lipgloss.Width(lipgloss.NewStyle().Foreground(styles.Subtle).Render(timeStr))
	textMaxW := maxW - leftFixed - rightFixed
	if textMaxW < 8 {
		textMaxW = 8
	}

	// Truncate item text if needed.
	displayText := item.Text
	if ansi.StringWidth(displayText) > textMaxW {
		displayText = ansiTruncate(displayText, textMaxW-1) + "…"
	}

	// Assemble the full line.
	line := cursor + iconStyle.Render(icon) + " " + textStyle.Render(displayText)
	// Pad to fill remaining space before right-aligned info.
	rightSide := lipgloss.NewStyle().Foreground(styles.Subtle).Render(taskInfo) + "  " + lipgloss.NewStyle().Foreground(styles.Subtle).Render(timeStr)
	pad := maxW - lipgloss.Width(line) - lipgloss.Width(rightSide)
	if pad < 0 {
		pad = 0
	}
	line += strings.Repeat(" ", pad) + rightSide

	// Apply highlight background to entire line if selected.
	if isSelected {
		line = lipgloss.NewStyle().Background(styles.Highlight).Render(line)
	} else if m.bulkMode && m.marked[item.ID] {
		line = lipgloss.NewStyle().Background(styles.Highlight).Foreground(styles.Text).Render(line)
	}

	return line
}

func ansiTruncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxW {
		return s
	}
	return ansi.Truncate(s, maxW, "")
}

// --- Store commands ---

func (m *Model) loadTodos() tea.Msg {
	items, err := m.common.Store.ListTodos(m.activeList)
	if err != nil {
		return todosLoadedMsg{}
	}
	return todosLoadedMsg{items: items}
}

func (m *Model) loadLinkedTasks() tea.Cmd {
	return func() tea.Msg {
		tasks := make(map[string]models.TaskContextRecord)
		for _, item := range m.items {
			if item.TaskID == nil {
				continue
			}
			tc, err := m.common.Store.GetTaskContext(*item.TaskID)
			if err != nil || tc == nil {
				continue
			}
			tasks[*item.TaskID] = *tc
		}
		return linkedTasksLoadedMsg{tasks: tasks}
	}
}

func (m *Model) loadSessionTimes() tea.Cmd {
	return func() tea.Msg {
		times := make(map[int]time.Duration)
		for _, item := range m.items {
			dur, err := m.common.Store.GetSessionTimeByTodoToday(item.ID)
			if err == nil && dur > 0 {
				times[item.ID] = dur
			}
		}
		return sessionTimesLoadedMsg{times: times}
	}
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
			_, _ = m.common.Store.CreateTodo(text, list, nil)
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

func (m *Model) markedIDs() []int {
	ids := make([]int, 0, len(m.marked))
	for id, on := range m.marked {
		if on {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m *Model) bulkToggleDoneCmd() tea.Cmd {
	ids := m.markedIDs()
	if len(ids) == 0 {
		if cur := m.currentItem(); cur != nil {
			ids = append(ids, cur.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return tea.Batch(
		func() tea.Msg {
			for _, id := range ids {
				_ = m.common.Store.ToggleTodo(id)
			}
			return m.loadTodos()
		},
		sendStatsRefresh,
	)
}

func (m *Model) bulkDeleteCmd() tea.Cmd {
	ids := m.markedIDs()
	if len(ids) == 0 {
		if cur := m.currentItem(); cur != nil {
			ids = append(ids, cur.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return tea.Batch(
		func() tea.Msg {
			for _, id := range ids {
				_ = m.common.Store.DeleteTodo(id)
			}
			return m.loadTodos()
		},
		sendStatsRefresh,
	)
}

func (m *Model) bulkMoveToTodayCmd() tea.Cmd {
	if m.activeList != models.ListSomeday {
		return nil
	}
	ids := m.markedIDs()
	if len(ids) == 0 {
		if cur := m.currentItem(); cur != nil {
			ids = append(ids, cur.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return tea.Batch(
		func() tea.Msg {
			for _, id := range ids {
				_ = m.common.Store.MoveToToday(id)
			}
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

func (m *Model) listViewport(rows int) (start, end int) {
	if rows <= 0 || len(m.items) <= rows {
		return 0, len(m.items)
	}
	start = m.cursor - rows/2
	if start < 0 {
		start = 0
	}
	end = start + rows
	if end > len(m.items) {
		end = len(m.items)
		start = end - rows
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func doneCount(items []models.Todo) int {
	n := 0
	for _, it := range items {
		if it.Status == models.StatusDone {
			n++
		}
	}
	return n
}

func (m *Model) totalFocusTime() time.Duration {
	total := m.totalWallClock
	if m.focusSince != nil {
		total += time.Since(*m.focusSince)
	}
	return total
}

// --- Getters for app layer ---

func (m *Model) Visible() bool { return m.visible }

// SetVisible toggles overlay visibility. When showing, reloads todo data.
func (m *Model) SetVisible(v bool) tea.Cmd {
	m.visible = v
	if v {
		return m.loadTodos
	}
	m.bulkMode = false
	m.marked = make(map[int]bool)
	m.inputMode = inputNone
	m.input.Blur()
	m.focusReminderLevel = reminderNone
	return nil
}

// TimerSummary returns a compact status for the header, e.g. "● 2 running".
func (m *Model) TimerSummary() string {
	n := len(m.runningTimers)
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("● %d running", n)
}

func sendModeChange(inputActive bool) tea.Cmd {
	return func() tea.Msg {
		return ModeChangeMsg{InputActive: inputActive}
	}
}

func sendStatsRefresh() tea.Msg {
	return models.StatsRefreshMsg{}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}
