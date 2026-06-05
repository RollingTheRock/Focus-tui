package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/models"
	agentsplugin "focus/internal/plugins/agents"
	gitplugin "focus/internal/plugins/git"
	gitfiletree "focus/internal/plugins/gitfiletree"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// detailTab enumerates the tabs inside the worktree detail pane.
type detailTab int

const (
	detailTabTasks detailTab = iota
	detailTabContext
	detailTabAgent
)

var detailTabNames = []string{"Task", "Context", "Agents"}

// worktreeDetailPane is the right-side pane showing tabbed content for the
// current worktree: task state, Trellis context, and agent sessions.
type worktreeDetailPane struct {
	id            models.PaneID
	meta          models.PaneMeta
	common        models.CommonModel
	adapter       adapters.GitAdapter
	trellisBridge trellisBridge

	repoID     string
	worktreeID string

	activeTab detailTab

	// Tasks tab
	tasks      []models.TaskContextRecord
	taskCursor int

	// Agent tab
	sessions    []*agents.Session
	agentCursor int

	width  int
	height int
}

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

func (p *worktreeDetailPane) SetTrellisBridge(bridge trellisBridge) {
	p.trellisBridge = bridge
}

func (p *worktreeDetailPane) setWorktree(worktreeID string) tea.Cmd {
	if p.worktreeID == worktreeID {
		return nil
	}
	p.worktreeID = worktreeID
	p.taskCursor = 0
	p.agentCursor = 0
	p.loadTasks()
	return nil
}

func (p *worktreeDetailPane) loadTasks() {
	p.tasks = nil
	if p.common.Store == nil {
		return
	}
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
		if msg.WorktreeID == p.worktreeID {
			p.refreshSessions()
		}
		return p, nil
	case refreshWorktreeDetailMsg:
		p.loadTasks()
		return p, nil
	case tea.KeyPressMsg:
		// Global tab switching (1-3)
		switch msg.Keystroke() {
		case "1":
			p.activeTab = detailTabTasks
			return p, nil
		case "2":
			p.activeTab = detailTabContext
			return p, nil
		case "3":
			p.activeTab = detailTabAgent
			return p, nil
		case "tab":
			return p, nil // let app route to next pane
		}

		// Route keys to the active tab
		switch p.activeTab {
		case detailTabTasks:
			return p.handleTasksKey(msg)
		case detailTabContext:
			return p.handleContextKey(msg)
		case detailTabAgent:
			return p.handleAgentKey(msg)
		}
	}

	return p, nil
}

// --- Tasks tab key handling ---

func (p *worktreeDetailPane) handleTasksKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
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
	case "o":
		return p, p.openGitFileTreeCmd()
	}
	return p, nil
}

// --- Context tab key handling ---

func (p *worktreeDetailPane) handleContextKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "e":
		return p, p.openTaskEditCmd()
	case "u":
		return p, p.trellisUpdateCmd()
	}
	return p, nil
}

// --- Agent tab key handling ---

func (p *worktreeDetailPane) handleAgentKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if p.agentCursor < len(p.sessions)-1 {
			p.agentCursor++
		}
		return p, nil
	case "k", "up":
		if p.agentCursor > 0 {
			p.agentCursor--
		}
		return p, nil
	case "s":
		return p, p.openAgentSelectCmd()
	case "x":
		return p, p.stopSelectedAgentCmd()
	}
	return p, nil
}

// --- commands ---

type worktreeSelectedMsg struct {
	WorktreeID string
}

type OpenAgentSelectMsg struct {
	WorktreeID string
}

type refreshWorktreeDetailMsg struct{}

// TrellisUpdateMsg signals a request to run trellis update.
type TrellisUpdateMsg struct{}

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

func (p *worktreeDetailPane) openGitFileTreeCmd() tea.Cmd {
	if p.worktreeID == "" {
		return nil
	}
	return func() tea.Msg {
		return gitfiletree.OpenGitFileTreeMsg{RepoPath: p.worktreeID}
	}
}

func (p *worktreeDetailPane) trellisUpdateCmd() tea.Cmd {
	if p.trellisBridge == nil {
		return nil
	}
	go func() {
		_ = p.trellisBridge.Update()
	}()
	return func() tea.Msg {
		return TrellisUpdateMsg{}
	}
}

func (p *worktreeDetailPane) stopSelectedAgentCmd() tea.Cmd {
	sessions := p.sortedSessions()
	if p.agentCursor < 0 || p.agentCursor >= len(sessions) {
		return nil
	}
	session := sessions[p.agentCursor]
	if session.State != agents.SessionRunning {
		return nil
	}
	return func() tea.Msg {
		return agentsplugin.KillSessionMsg{SessionID: session.ID, PID: session.PID}
	}
}

func (p *worktreeDetailPane) refreshSessions() {
	// Sessions will be refreshed by the app layer pushing them in.
}

func (p *worktreeDetailPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *worktreeDetailPane) KeyBindings(compact bool) []models.KeyBinding {
	bindings := []models.KeyBinding{
		{Keys: []string{"1", "2", "3"}, Help: "tabs"},
	}
	switch p.activeTab {
	case detailTabTasks:
		bindings = append(bindings,
			models.KeyBinding{Keys: []string{"j", "k"}, Help: "nav"},
			models.KeyBinding{Keys: []string{"enter", "e"}, Help: "edit task"},
			models.KeyBinding{Keys: []string{"s"}, Help: "start agent"},
			models.KeyBinding{Keys: []string{"o"}, Help: "files"},
		)
	case detailTabContext:
		bindings = append(bindings,
			models.KeyBinding{Keys: []string{"e"}, Help: "edit PRD"},
			models.KeyBinding{Keys: []string{"u"}, Help: "trellis update"},
		)
	case detailTabAgent:
		bindings = append(bindings,
			models.KeyBinding{Keys: []string{"j", "k"}, Help: "nav"},
			models.KeyBinding{Keys: []string{"s"}, Help: "start"},
			models.KeyBinding{Keys: []string{"x"}, Help: "stop"},
		)
	}
	if !compact {
		bindings = append(bindings, models.KeyBinding{Keys: []string{"tab"}, Help: "cycle focus"})
	}
	return bindings
}

func (p *worktreeDetailPane) helpText() (wide, compact string) {
	return "", ""
}

func (p *worktreeDetailPane) View() tea.View {
	w := p.width
	h := p.height
	if w <= 0 || h <= 0 {
		return tea.NewView("")
	}

	tabBar := p.renderTabs(w)
	contentH := h - lipgloss.Height(tabBar)
	if contentH < 2 {
		contentH = 2
	}

	var content string
	switch p.activeTab {
	case detailTabTasks:
		content = p.renderTasks(w, contentH)
	case detailTabContext:
		content = p.renderContextTab(w, contentH)
	case detailTabAgent:
		content = p.renderAgentTab(w, contentH)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, tabBar, content))
}

func (p *worktreeDetailPane) renderTabs(w int) string {
	var parts []string
	for i, name := range detailTabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if detailTab(i) == p.activeTab {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	padding := w - lipgloss.Width(bar)
	if padding < 0 {
		padding = 0
	}
	return bar + strings.Repeat(" ", padding)
}

// --- Tasks tab ---

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

	header := detailMetaStyle.Render(fmt.Sprintf("  tasks %d  position %d/%d  %s", len(p.tasks), p.taskCursor+1, len(p.tasks), p.gitSummary()))
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

// --- Context tab ---

func (p *worktreeDetailPane) renderContextTab(w, h int) string {
	if p.worktreeID == "" {
		return lipgloss.NewStyle().MaxWidth(w).Render("  No worktree selected.")
	}

	var lines []string
	lines = append(lines, detailMetaStyle.Render("  Context (Trellis)"))
	if p.trellisBridge != nil {
		if v := p.trellisBridge.Version(); v != "" {
			lines = append(lines, detailMetaStyle.Render(fmt.Sprintf("  Trellis v%s", v)))
		}
	}
	lines = append(lines, "")

	// Show current task info if available
	if len(p.tasks) > 0 && p.taskCursor >= 0 && p.taskCursor < len(p.tasks) {
		t := p.tasks[p.taskCursor]
		lines = append(lines, fmt.Sprintf("  Task: %s", clipDisplayText(t.Title, w-8)))
		lines = append(lines, fmt.Sprintf("  State: %s  Priority: %s", t.State, t.Priority))
		if t.Goal != "" {
			goal := clipDisplayText(t.Goal, w-8)
			lines = append(lines, fmt.Sprintf("  Goal: %s", goal))
		}
		lines = append(lines, "")

		// Trellis extended context
		if p.trellisBridge != nil {
			extCtx, err := p.trellisBridge.GetTaskContextExtended(t.ID)
			if err == nil && extCtx != nil {
				// Status comparison: Focus vs Trellis
				if extCtx.Task != nil && extCtx.Task.Status != "" {
					trellisState := trellisToFocusState(extCtx.Task.Status)
					if trellisState != t.State {
						warn := fmt.Sprintf("  ⚠ Status mismatch: Focus=%s | Trellis=%s", t.State, extCtx.Task.Status)
						lines = append(lines, lipgloss.NewStyle().Foreground(styles.StateBlocked).Render(warn))
						lines = append(lines, "")
					}
				}
				if extCtx.PRD != "" {
					lines = append(lines, lipgloss.NewStyle().Foreground(styles.Accent).Render("  PRD"))
					prdPreview := firstNLines(extCtx.PRD, 10)
					for _, prdLine := range strings.Split(prdPreview, "\n") {
						lines = append(lines, "  "+ansi.Truncate(prdLine, w-4, "…"))
					}
					lines = append(lines, "")
				}
				// List specs from .trellis/spec/
				if specs, err := p.trellisBridge.ListSpecs(); err == nil && len(specs) > 0 {
					lines = append(lines, lipgloss.NewStyle().Foreground(styles.Accent).Render(fmt.Sprintf("  Specs (%d)", len(specs))))
					for _, spec := range specs {
						marker := "○"
						if extCtx.Task != nil && containsString(extCtx.Task.RelatedFiles, spec) {
							marker = "●"
						}
						lines = append(lines, fmt.Sprintf("  %s %s", marker, ansi.Truncate(spec, w-6, "…")))
					}
					lines = append(lines, "")
				}
				if extCtx.WorkflowState != "" {
					lines = append(lines, fmt.Sprintf("  Workflow: %s", extCtx.WorkflowState))
				}
				if extCtx.ImplementJSONL != "" {
					lines = append(lines, lipgloss.NewStyle().Foreground(styles.Accent).Render("  Curated Context"))
					for _, entry := range firstJSONLContextEntries(extCtx.ImplementJSONL, 5) {
						lines = append(lines, "  "+ansi.Truncate(entry, w-4, "…"))
					}
					lines = append(lines, "")
				}
				if extCtx.Handoff != "" {
					lines = append(lines, lipgloss.NewStyle().Foreground(styles.Accent).Render("  Handoff"))
					handoffPreview := firstNLines(extCtx.Handoff, 5)
					for _, line := range strings.Split(handoffPreview, "\n") {
						lines = append(lines, "  "+ansi.Truncate(line, w-4, "…"))
					}
					lines = append(lines, "")
				}
				if extCtx.Journal != "" {
					lines = append(lines, lipgloss.NewStyle().Foreground(styles.Accent).Render("  Journal"))
					journalPreview := firstNLines(extCtx.Journal, 5)
					for _, line := range strings.Split(journalPreview, "\n") {
						lines = append(lines, "  "+ansi.Truncate(line, w-4, "…"))
					}
					lines = append(lines, "")
				}
			} else {
				lines = append(lines, detailMetaStyle.Render("  Trellis context not yet synced."))
			}
		} else {
			lines = append(lines, detailMetaStyle.Render("  Trellis bridge not available."))
		}
	} else {
		lines = append(lines, "  No task selected. Select a task in the Tasks tab.")
	}

	lines = append(lines, "")
	lines = append(lines, detailMetaStyle.Render("  [e]edit PRD  [u]update trellis"))
	return lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(strings.Join(lines, "\n"))
}

// --- Agent tab ---

func (p *worktreeDetailPane) renderAgentTab(w, h int) string {
	if p.worktreeID == "" {
		return lipgloss.NewStyle().MaxWidth(w).Render("  No worktree selected.")
	}

	var lines []string
	lines = append(lines, detailMetaStyle.Render("  Agent Sessions"))
	lines = append(lines, "")

	if len(p.sessions) == 0 {
		lines = append(lines, "  No active sessions. Press [s] to start an agent.")
		return lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(strings.Join(lines, "\n"))
	}

	sessions := p.sortedSessions()

	if p.agentCursor >= len(sessions) {
		p.agentCursor = len(sessions) - 1
	}
	if p.agentCursor < 0 {
		p.agentCursor = 0
	}

	for i, s := range sessions {
		icon := "○"
		if s.State == agents.SessionRunning {
			icon = "●"
		}
		cursor := "  "
		if i == p.agentCursor {
			cursor = "▸ "
		}
		stateStyle := detailSessionStateStyle(s.State)
		card := fmt.Sprintf("%s%s %s | %s | %s", cursor, icon, s.Provider, s.State, sessionRecencyLabel(s))
		if i == p.agentCursor {
			card = stateStyle.Render(ansi.Truncate(card, w, "…"))
		} else {
			card = ansi.Truncate(card, w, "…")
		}
		lines = append(lines, card)
		if i == p.agentCursor && s.Summary != "" {
			lines = append(lines, detailMetaStyle.Render("   "+clipDisplayText(s.Summary, w-5)))
		}
	}

	lines = append(lines, "")

	// Token budget placeholder (agent SDK does not expose token usage yet).
	lines = append(lines, "")
	lines = append(lines, detailMetaStyle.Render("  Tokens: —  (not tracked by agent SDK)"))
	return lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(strings.Join(lines, "\n"))
}

func (p *worktreeDetailPane) SetAgentSessions(sessions []*agents.Session) {
	p.sessions = sessions
}

func (p *worktreeDetailPane) sortedSessions() []*agents.Session {
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
	return sessions
}

// --- helpers ---

func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}

func emptyLine(w, h int) string {
	return lipgloss.NewStyle().Width(w).Height(h).Render("")
}

var (
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
	case "archived":
		return lipgloss.NewStyle().Foreground(styles.StateIdle).Bold(true).Render("[" + label + "]")
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

// trellisToFocusState maps Trellis task statuses to Focus task states.
func trellisToFocusState(trellisStatus string) string {
	switch trellisStatus {
	case "in_progress", "reviewing":
		return "active"
	case "completed", "done":
		return "done"
	case "archived":
		return "archived"
	case "planning":
		return "blocked"
	default:
		return trellisStatus
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (p *worktreeDetailPane) gitSummary() string {
	if p.worktreeID == "" || p.adapter == nil {
		return "git: -"
	}
	status, err := p.adapter.GetWorktreeStatus(p.worktreeID)
	if err != nil || status == nil {
		return "git: unavailable"
	}
	changes := len(status.StagedFiles) + len(status.UnstagedFiles) + len(status.UntrackedFiles) + len(status.ConflictedFiles)
	if changes == 0 {
		return fmt.Sprintf("git: %s clean", status.Branch)
	}
	return fmt.Sprintf("git: %s %d changes", status.Branch, changes)
}

func firstJSONLContextEntries(value string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	var out []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err == nil {
			if file, ok := obj["file"].(string); ok && file != "" {
				action, _ := obj["action"].(string)
				reason, _ := obj["reason"].(string)
				parts := []string{file}
				if action != "" {
					parts = append(parts, "("+action+")")
				}
				if reason != "" {
					parts = append(parts, "- "+reason)
				}
				out = append(out, strings.Join(parts, " "))
			} else {
				out = append(out, line)
			}
		} else {
			out = append(out, line)
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}
