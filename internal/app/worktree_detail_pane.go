package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/models"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// worktreeDetailPane is the right-side pane showing the current worktree's
// tasks, git status, file tree, agent cards, and a human shell below.
type worktreeDetailPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoID     string
	worktreeID string

	activeTab detailTab

	// Tasks for this worktree
	tasks      []models.TaskContextRecord
	taskCursor int

	// Agent sessions attached to this worktree
	sessions []*agents.Session

	width  int
	height int
}

type detailTab int

const (
	tabTasks detailTab = iota
)

func newWorktreeDetailPane(id models.PaneID, meta models.PaneMeta, common *models.CommonModel, adapter adapters.GitAdapter, repoID string) *worktreeDetailPane {
	p := &worktreeDetailPane{
		id:      id,
		meta:    meta,
		common:  *common,
		adapter: adapter,
		repoID:  repoID,
	}
	return p
}

func (p *worktreeDetailPane) setWorktree(worktreeID string) tea.Cmd {
	if p.worktreeID == worktreeID {
		return nil
	}
	p.worktreeID = worktreeID
	p.taskCursor = 0
	p.loadTasks()
	return nil
}

func (p *worktreeDetailPane) initSubPanes() {
	// Git and Files moved to GitFileTree overlay; this pane now only shows Tasks.
}

func (p *worktreeDetailPane) loadTasks() {
	p.tasks = nil
	if p.common.Store == nil {
		return
	}
	// Load tasks whose preferred_worktree_id matches this worktree.
	all, err := p.common.Store.ListTaskContexts(p.repoID)
	if err != nil {
		return
	}
	for _, t := range all {
		if t.PreferredWorktreeID == p.worktreeID {
			p.tasks = append(p.tasks, t)
		}
	}
}

func (p *worktreeDetailPane) Init() tea.Cmd {
	p.loadTasks()
	return nil
}

func (p *worktreeDetailPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreeSelectedMsg:
		cmd := p.setWorktree(msg.WorktreeID)
		return p, cmd
	case agents.LaunchAgentMsg:
		// An agent was launched for this worktree – refresh sessions.
		if msg.WorktreeID == p.worktreeID {
			p.refreshSessions()
		}
		return p, nil
	case refreshWorktreeDetailMsg:
		p.loadTasks()
		return p, nil
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "j", "down":
			if p.taskCursor < len(p.tasks)-1 {
				p.taskCursor++
			}
			return p, nil
		case "k", "up":
			if p.taskCursor > 0 {
				p.taskCursor--
			}
			return p, nil
		case "s":
			return p, p.openAgentSelectCmd()
		case "e", "enter":
			return p, p.openTaskEditCmd()
		case "tab":
			return p, nil // let app route to next pane
		}
	}

	return p, nil
}

type worktreeSelectedMsg struct {
	WorktreeID string
}

type OpenAgentSelectMsg struct {
	WorktreeID string
}

type refreshWorktreeDetailMsg struct{}

func (p *worktreeDetailPane) openAgentSelectCmd() tea.Cmd {
	if p.worktreeID == "" {
		return nil
	}
	return func() tea.Msg {
		return OpenAgentSelectMsg{WorktreeID: p.worktreeID}
	}
}

func (p *worktreeDetailPane) openTaskEditCmd() tea.Cmd {
	if p.worktreeID == "" || len(p.tasks) == 0 {
		return nil
	}
	if p.taskCursor < 0 {
		p.taskCursor = 0
	}
	if p.taskCursor >= len(p.tasks) {
		p.taskCursor = len(p.tasks) - 1
	}
	task := p.tasks[p.taskCursor]
	return func() tea.Msg {
		return gitplugin.OpenTaskEditMsg{
			TaskID:       task.ID,
			WorktreeID:   p.worktreeID,
			RepoID:       p.repoID,
			RelationType: "primary",
		}
	}
}

func (p *worktreeDetailPane) refreshSessions() {
	// Sessions will be refreshed by the app layer pushing them in.
}

func (p *worktreeDetailPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func contentHeight(total int) int {
	ch := total - 3 - agentCardsHeight() // tab bar + agent cards
	if ch < 3 {
		ch = 3
	}
	return ch
}

func agentCardsHeight() int {
	return 3
}

func (p *worktreeDetailPane) helpText() (wide, compact string) {
	wide = "[j/k]nav  [enter/e]edit task  [s]tart agent  [tab]cycle focus"
	compact = "[j/k]nav  [enter/e]edit  [s]agent"
	return
}

func (p *worktreeDetailPane) View() tea.View {
	w := p.width
	h := p.height
	if w <= 0 || h <= 0 {
		return tea.NewView("")
	}

	var parts []string
	ch := h - agentCardsHeight()
	if ch < 3 {
		ch = 3
	}
	parts = append(parts, p.renderTasks(w, ch))
	parts = append(parts, p.renderAgentCards(w))
	return tea.NewView(strings.Join(parts, "\n"))
}

func (p *worktreeDetailPane) renderTabs(w int) string {
	return ""
}

func (p *worktreeDetailPane) renderTasks(w, h int) string {
	if len(p.tasks) == 0 {
		return lipgloss.NewStyle().MaxWidth(w).Render("  No tasks assigned to this worktree.")
	}
	if p.taskCursor >= len(p.tasks) {
		p.taskCursor = len(p.tasks) - 1
	}
	if p.taskCursor < 0 {
		p.taskCursor = 0
	}

	header := detailMetaStyle.Render(fmt.Sprintf("  tasks %d  position %d/%d", len(p.tasks), p.taskCursor+1, len(p.tasks)))
	var lines []string
	lines = append(lines, header, "")

	rowBudget := h - len(lines)
	if rowBudget < 2 {
		rowBudget = 2
	}
	start, end, showUp, showDown := taskWindow(len(p.tasks), p.taskCursor, rowBudget)
	if showUp {
		lines = append(lines, detailMetaStyle.Render(fmt.Sprintf("  ▲ %d hidden", start)))
	}
	for i := start; i < end; i++ {
		t := p.tasks[i]
		state := t.State
		if state == "" {
			state = "ready"
		}
		cursor := "  "
		if i == p.taskCursor {
			cursor = "▸ "
		}
		stateBadge := detailStateBadge(state)
		priorityBadge := detailPriorityBadge(t.Priority)
		title := clipDisplayText(t.Title, max(10, w-32))
		line := fmt.Sprintf("%s%s %s %s", cursor, stateBadge, priorityBadge, title)
		if i == p.taskCursor {
			line = detailSelectedRowStyle.Render(ansi.Truncate(line, w, "…"))
		} else {
			line = ansi.Truncate(line, w, "…")
		}
		lines = append(lines, line)
		if i == p.taskCursor && strings.TrimSpace(t.NextStep) != "" {
			next := detailMetaStyle.Render("   next: " + ansi.Truncate(strings.TrimSpace(strings.ReplaceAll(t.NextStep, "\n", " ")), max(8, w-9), "…"))
			lines = append(lines, next)
		}
	}
	if showDown {
		lines = append(lines, detailMetaStyle.Render(fmt.Sprintf("  ▼ %d hidden", len(p.tasks)-end)))
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(strings.Join(lines, "\n"))
}

func (p *worktreeDetailPane) renderAgentCards(w int) string {
	if p.worktreeID == "" {
		return lipgloss.NewStyle().MaxWidth(w).Foreground(styles.Subtle).Render("  Agents: —  (select a worktree first)")
	}
	if len(p.sessions) == 0 {
		return lipgloss.NewStyle().MaxWidth(w).Foreground(styles.Subtle).Render("  Agents: none  [s] start")
	}
	sessions := append([]*agents.Session(nil), p.sessions...)
	sort.SliceStable(sessions, func(i, j int) bool {
		li, lj := sessions[i], sessions[j]
		if li.State == agents.SessionRunning && lj.State != agents.SessionRunning {
			return true
		}
		if li.State != agents.SessionRunning && lj.State == agents.SessionRunning {
			return false
		}
		lti := sessionActivityAt(li)
		ltj := sessionActivityAt(lj)
		return lti.After(ltj)
	})

	var parts []string
	for _, s := range sessions {
		icon := "○"
		if s.State == agents.SessionRunning {
			icon = "●"
		}
		stateStyle := detailSessionStateStyle(s.State)
		card := fmt.Sprintf("%s %s | %s | %s", icon, s.Provider, s.State, sessionRecencyLabel(s))
		parts = append(parts, stateStyle.Render("["+card+"]"))
	}
	return lipgloss.NewStyle().MaxWidth(w).Render("  Agents: " + strings.Join(parts, " "))
}

func (p *worktreeDetailPane) SetAgentSessions(sessions []*agents.Session) {
	p.sessions = sessions
}

func emptyLine(w, h int) string {
	return lipgloss.NewStyle().Width(w).Height(h).Render("")
}

var (
	detailTabStyle         = lipgloss.NewStyle().Foreground(styles.Subtle)
	detailTabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	detailMetaStyle        = lipgloss.NewStyle().Foreground(styles.Subtle)
	detailSelectedRowStyle = lipgloss.NewStyle().Background(styles.Highlight)
)

func clipDisplayText(value string, maxW int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if value == "" || maxW <= 0 {
		return ""
	}
	return ansi.Truncate(value, maxW, "…")
}

func detailStateBadge(state string) string {
	label := strings.ToUpper(state)
	if label == "" {
		label = "READY"
	}
	switch state {
	case "active":
		return lipgloss.NewStyle().Foreground(styles.StateActive).Bold(true).Render("[" + label + "]")
	case "paused":
		return lipgloss.NewStyle().Foreground(styles.StatePaused).Bold(true).Render("[" + label + "]")
	case "blocked":
		return lipgloss.NewStyle().Foreground(styles.StateBlocked).Bold(true).Render("[" + label + "]")
	case "done":
		return lipgloss.NewStyle().Foreground(styles.StateDone).Bold(true).Render("[" + label + "]")
	default:
		return lipgloss.NewStyle().Foreground(styles.StateReady).Bold(true).Render("[" + label + "]")
	}
}

func detailPriorityBadge(priority string) string {
	label := strings.ToUpper(strings.TrimSpace(priority))
	if label == "" {
		label = "MEDIUM"
	}
	switch strings.ToLower(priority) {
	case "critical":
		return lipgloss.NewStyle().Foreground(styles.PriorityCritical).Bold(true).Render("{" + label + "}")
	case "high":
		return lipgloss.NewStyle().Foreground(styles.PriorityHigh).Bold(true).Render("{" + label + "}")
	case "low":
		return lipgloss.NewStyle().Foreground(styles.PriorityLow).Render("{" + label + "}")
	default:
		return lipgloss.NewStyle().Foreground(styles.PriorityMedium).Render("{" + label + "}")
	}
}

func taskWindow(total, cursor, rowBudget int) (start, end int, showUp, showDown bool) {
	if total <= 0 {
		return 0, 0, false, false
	}
	// Reserve one line for selected "next step" detail when possible.
	visible := rowBudget
	if visible > 3 {
		visible--
	}
	if visible < 1 {
		visible = 1
	}
	if total <= visible {
		return 0, total, false, false
	}
	start = cursor - visible/2
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > total {
		end = total
		start = end - visible
	}
	return start, end, start > 0, end < total
}

func detailSessionStateStyle(state agents.SessionState) lipgloss.Style {
	switch state {
	case agents.SessionRunning:
		return lipgloss.NewStyle().Foreground(styles.StateActive)
	case agents.SessionFailed, agents.SessionDisconnected:
		return lipgloss.NewStyle().Foreground(styles.StateBlocked)
	case agents.SessionExited:
		return lipgloss.NewStyle().Foreground(styles.StateIdle)
	default:
		return lipgloss.NewStyle().Foreground(styles.StatePaused)
	}
}

func sessionActivityAt(s *agents.Session) time.Time {
	if s == nil {
		return time.Time{}
	}
	if s.LastActivityAt != nil {
		return *s.LastActivityAt
	}
	return s.StartedAt
}

func sessionRecencyLabel(s *agents.Session) string {
	at := sessionActivityAt(s)
	if at.IsZero() {
		return "idle"
	}
	d := time.Since(at)
	if d < time.Minute {
		return "active now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}
