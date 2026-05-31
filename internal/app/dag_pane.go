package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/models"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// dagPane shows the full Task DAG at the top of the screen.
// Humans navigate nodes here and decide which task to create a worktree for.
type dagPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	repoID  string
	adapter adapters.GitAdapter

	tasks      []models.TaskContextRecord
	nodes      map[string]dagNode
	edges      []dagEdge
	levels     map[string]int
	layerIDs   map[int][]string
	maxLevel   int
	cursorNode string

	width  int
	height int

	// vertical scroll offset for DAG body (in lines)
	scrollOffset int

	// quick-create state
	creating   bool
	createForm *huh.Form
	createErr  error
}

func newDagPane(id models.PaneID, meta models.PaneMeta, common *models.CommonModel, repoID string, adapter adapters.GitAdapter) *dagPane {
	return &dagPane{
		id:      id,
		meta:    meta,
		common:  *common,
		repoID:  repoID,
		adapter: adapter,
	}
}

func (p *dagPane) Init() tea.Cmd {
	return p.refreshCmd()
}

func (p *dagPane) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		return dagRefreshMsg{repoID: p.repoID}
	}
}

type dagRefreshMsg struct {
	repoID string
}

type dagExpandPhaseMsg struct {
	PhaseID    string
	PhaseTitle string
}

type dagTaskCreatedMsg struct {
	Title  string
	Goal   string
	RepoID string
}

func (p *dagPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case dagRefreshMsg:
		p.buildDAG()
		return p, nil

		case tea.KeyPressMsg:
			if p.creating && p.createForm != nil {
				if msg.Keystroke() == "esc" {
					p.exitCreateMode()
					return p, nil
				}
				m, cmd := p.createForm.Update(msg)
				if f, ok := m.(*huh.Form); ok {
					p.createForm = f
				}
				// Only flush on Enter to avoid per-keystroke overhead in production.
				if msg.Keystroke() == "enter" {
					p.flushFormCmds(cmd)
				}
				if p.createForm.State == huh.StateCompleted {
					title := strings.TrimSpace(p.createForm.GetString("title"))
					goal := strings.TrimSpace(p.createForm.GetString("goal"))
					p.exitCreateMode()
					return p, p.submitCreate(title, goal)
				}
				return p, cmd
			}

			switch msg.Keystroke() {
		case "R":
			return p, p.refreshCmd()
		case "j", "down":
			p.moveCursor(0, 1)
		case "k", "up":
			p.moveCursor(0, -1)
		case "h", "left":
			p.moveCursor(-1, 0)
		case "l", "right":
			p.moveCursor(1, 0)
		case "enter":
			return p, p.selectNodeCmd()
		case "c":
			return p, p.createWorktreeCmd()
		case "a":
			return p, p.launchArchitectureAgentCmd()
		case "r":
			return p, p.launchResearchAgentCmd()
		case "s":
			return p, p.cycleStateCmd()
		case "t":
			return p, p.addToTodayTodosCmd()
		case "n":
			return p, p.enterCreateMode()
		case "d":
			return p, p.deleteTaskCmd()
		case "D":
			return p, p.clearAllTasksCmd()
		case "z":
			return p, p.expandPhaseCmd()
		}
	}
	return p, nil
}

func (p *dagPane) hasDAG() bool {
	return len(p.nodes) > 0
}

func (p *dagPane) buildDAG() {
	store := p.common.Store
	if store == nil {
		return
	}

	tasks, err := store.ListTaskContexts(p.repoID)
	if err != nil {
		return
	}
	p.tasks = tasks

	// Build map from task ID → parent Phase ID (empty string if it's a Phase itself)
	taskToPhase := make(map[string]string, len(tasks))
	for _, t := range tasks {
		if t.ParentTaskID == nil || *t.ParentTaskID == "" {
			taskToPhase[t.ID] = t.ID
		} else {
			taskToPhase[t.ID] = *t.ParentTaskID
		}
	}

	// Only include Phases (tasks with no parent) in the DAG nodes
	p.nodes = make(map[string]dagNode, len(tasks))
	for _, t := range tasks {
		if t.ParentTaskID != nil && *t.ParentTaskID != "" {
			continue // Skip steps
		}
		p.nodes[t.ID] = dagNode{
			ID:       t.ID,
			Title:    t.Title,
			State:    t.State,
			Priority: t.Priority,
		}
	}

	p.edges = nil
	adjacency := make(map[string][]dagEdge)
	indegree := make(map[string]int)
	seen := make(map[string]struct{})

	// Iterate over ALL tasks (including steps) to derive phase-level dependencies
	for _, t := range tasks {
		downstream, err := store.ListDownstreamTaskContexts(t.ID)
		if err != nil {
			continue
		}
		for _, d := range downstream {
			fromPhase := taskToPhase[t.ID]
			toPhase := taskToPhase[d.ID]
			if fromPhase == "" || toPhase == "" || fromPhase == toPhase {
				continue
			}
			if _, ok := p.nodes[fromPhase]; !ok {
				continue
			}
			if _, ok := p.nodes[toPhase]; !ok {
				continue
			}
			key := fromPhase + "->" + toPhase
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			e := dagEdge{From: fromPhase, To: toPhase, Type: "hard"}
			p.edges = append(p.edges, e)
			adjacency[fromPhase] = append(adjacency[fromPhase], e)
			indegree[toPhase]++
		}
	}

	sort.Slice(p.edges, func(i, j int) bool {
		if p.edges[i].From == p.edges[j].From {
			return p.edges[i].To < p.edges[j].To
		}
		return p.edges[i].From < p.edges[j].From
	})

	p.levels = computeDAGLevels(p.nodes, adjacency, indegree)
	p.maxLevel = 0
	for _, lv := range p.levels {
		if lv > p.maxLevel {
			p.maxLevel = lv
		}
	}
	p.layerIDs = make(map[int][]string, p.maxLevel+1)
	for id, lv := range p.levels {
		p.layerIDs[lv] = append(p.layerIDs[lv], id)
	}
	for lv := 0; lv <= p.maxLevel; lv++ {
		ids := p.layerIDs[lv]
		sort.Slice(ids, func(i, j int) bool {
			li, lj := p.nodes[ids[i]], p.nodes[ids[j]]
			if li.Title == lj.Title {
				return li.ID < lj.ID
			}
			return li.Title < lj.Title
		})
	}

	// default cursor: only consider phases
	if p.cursorNode == "" || p.nodes[p.cursorNode].ID == "" {
		for _, t := range tasks {
			if t.ParentTaskID != nil && *t.ParentTaskID != "" {
				continue
			}
			if t.State == "ready" || t.State == "" {
				p.cursorNode = t.ID
				break
			}
		}
		if p.cursorNode == "" && len(tasks) > 0 {
			for _, t := range tasks {
				if t.ParentTaskID != nil && *t.ParentTaskID != "" {
					continue
				}
				p.cursorNode = t.ID
				break
			}
		}
	}
}

func (p *dagPane) moveCursor(dLevel, dIndex int) {
	if p.cursorNode == "" {
		return
	}
	// Defensive: if cursorNode is stale (not in current levels/layers), reset it
	curLevel, ok := p.levels[p.cursorNode]
	if !ok {
		p.resetCursor()
		return
	}
	curIdx := -1
	for i, id := range p.layerIDs[curLevel] {
		if id == p.cursorNode {
			curIdx = i
			break
		}
	}
	if curIdx < 0 {
		p.resetCursor()
		return
	}

	if dLevel != 0 {
		targetLevel := curLevel + dLevel
		if targetLevel < 0 || targetLevel > p.maxLevel {
			return
		}
		ids := p.layerIDs[targetLevel]
		if len(ids) == 0 {
			return
		}
		idx := curIdx
		if idx >= len(ids) {
			idx = len(ids) - 1
		}
		p.cursorNode = ids[idx]
		p.ensureCursorVisible()
		return
	}

	if dIndex != 0 {
		ids := p.layerIDs[curLevel]
		target := curIdx + dIndex
		if target < 0 || target >= len(ids) {
			return
		}
		p.cursorNode = ids[target]
		p.ensureCursorVisible()
	}
}

func (p *dagPane) resetCursor() {
	for _, t := range p.tasks {
		if t.ParentTaskID != nil && *t.ParentTaskID != "" {
			continue
		}
		if t.State == "ready" || t.State == "" {
			p.cursorNode = t.ID
			p.scrollOffset = 0
			return
		}
	}
	for _, t := range p.tasks {
		if t.ParentTaskID != nil && *t.ParentTaskID != "" {
			continue
		}
		p.cursorNode = t.ID
		p.scrollOffset = 0
		return
	}
	p.cursorNode = ""
	p.scrollOffset = 0
}

// ensureCursorVisible adjusts scrollOffset so the cursor node is within the visible area.
// It uses an approximate row calculation (each node occupies ~2 lines with spacing).
func (p *dagPane) ensureCursorVisible() {
	if p.cursorNode == "" {
		return
	}
	curLevel, ok := p.levels[p.cursorNode]
	if !ok {
		return
	}
	curIdx := 0
	for i, id := range p.layerIDs[curLevel] {
		if id == p.cursorNode {
			curIdx = i
			break
		}
	}

	// Approximate row: each node takes 2 lines (1 node + 1 spacing).
	// Add a small buffer for edges drawn above/below nodes.
	approxRow := curIdx * 2

	headerRows := 2
	visibleRows := p.height - headerRows
	if visibleRows < 3 {
		visibleRows = 3
	}

	if approxRow < p.scrollOffset {
		p.scrollOffset = approxRow
	}
	if approxRow >= p.scrollOffset+visibleRows {
		p.scrollOffset = approxRow - visibleRows + 1
	}
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}
}

func (p *dagPane) selectedTask() (models.TaskContextRecord, bool) {
	if p.cursorNode == "" {
		return models.TaskContextRecord{}, false
	}
	store := p.common.Store
	if store == nil {
		return models.TaskContextRecord{}, false
	}
	t, err := store.GetTaskContext(p.cursorNode)
	if err != nil || t == nil {
		return models.TaskContextRecord{}, false
	}
	return *t, true
}

func (p *dagPane) selectNodeCmd() tea.Cmd {
	task, ok := p.selectedTask()
	if !ok {
		return nil
	}
	preferredWT := task.PreferredWorktreeID
	return func() tea.Msg {
		return dagNodeSelectedMsg{
			TaskID:              task.ID,
			TaskTitle:           task.Title,
			RepoID:              p.repoID,
			PreferredWorktreeID: preferredWT,
			IsPhase:             true,
		}
	}
}

func (p *dagPane) createWorktreeCmd() tea.Cmd {
	task, ok := p.selectedTask()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		return dagCreateWorktreeMsg{
			TaskID:    task.ID,
			TaskTitle: task.Title,
			RepoID:    p.repoID,
			IsPhase:   true,
		}
	}
}

func (p *dagPane) launchArchitectureAgentCmd() tea.Cmd {
	task, _ := p.selectedTask()
	return func() tea.Msg {
		wtID := p.repoID
		if task.PreferredWorktreeID != "" {
			wtID = task.PreferredWorktreeID
		}
		return dagLaunchAgentMsg{
			TaskID:     task.ID,
			TaskTitle:  task.Title,
			RepoID:     p.repoID,
			WorktreeID: wtID,
			Provider:   agents.ProviderClaude,
			ExtraArgs:  []string{"--agent", "arch"},
		}
	}
}

func (p *dagPane) cycleStateCmd() tea.Cmd {
	node, ok := p.nodes[p.cursorNode]
	if !ok || node.ID == "" {
		return nil
	}
	return func() tea.Msg {
		return gitplugin.CycleTaskStateMsg{
			TaskID:       node.ID,
			CurrentState: node.State,
		}
	}
}

func (p *dagPane) launchResearchAgentCmd() tea.Cmd {
	task, _ := p.selectedTask()
	return func() tea.Msg {
		wtID := p.repoID
		if task.PreferredWorktreeID != "" {
			wtID = task.PreferredWorktreeID
		}
		return dagLaunchAgentMsg{
			TaskID:     task.ID,
			TaskTitle:  task.Title,
			RepoID:     p.repoID,
			WorktreeID: wtID,
			Provider:   agents.ProviderKimi,
		}
	}
}

func (p *dagPane) addToTodayTodosCmd() tea.Cmd {
	task, ok := p.selectedTask()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		return dagAddTaskToTodoMsg{
			TaskID:    task.ID,
			TaskTitle: task.Title,
		}
	}
}

func (p *dagPane) deleteTaskCmd() tea.Cmd {
	task, ok := p.selectedTask()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		return dagDeleteTaskMsg{
			TaskID:    task.ID,
			TaskTitle: task.Title,
		}
	}
}

func (p *dagPane) clearAllTasksCmd() tea.Cmd {
	return func() tea.Msg {
		return dagClearAllTasksMsg{
			RepoID: p.repoID,
		}
	}
}

func (p *dagPane) expandPhaseCmd() tea.Cmd {
	task, ok := p.selectedTask()
	if !ok {
		return nil
	}
	// Only phases can be expanded
	if task.ParentTaskID != nil && *task.ParentTaskID != "" {
		return nil
	}
	return func() tea.Msg {
		return dagExpandPhaseMsg{
			PhaseID:    task.ID,
			PhaseTitle: task.Title,
		}
	}
}

type dagNodeSelectedMsg struct {
	TaskID              string
	TaskTitle           string
	RepoID              string
	PreferredWorktreeID string
	IsPhase             bool
}

type dagAddTaskToTodoMsg struct {
	TaskID    string
	TaskTitle string
}

type dagCreateWorktreeMsg struct {
	TaskID    string
	TaskTitle string
	RepoID    string
	IsPhase   bool
}

type dagLaunchAgentMsg struct {
	TaskID     string
	TaskTitle  string
	RepoID     string
	WorktreeID string
	Provider   agents.Provider
	ExtraArgs  []string
}

type dagDeleteTaskMsg struct {
	TaskID    string
	TaskTitle string
}

type dagClearAllTasksMsg struct {
	RepoID string
}

type dagTaskDeletedMsg struct {
	RepoID string
}

type dagTasksClearedMsg struct {
	RepoID string
}

func (p *dagPane) enterCreateMode() tea.Cmd {
	p.creating = true
	p.createErr = nil
	p.createForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("title").
				Title("New task title").
				Placeholder("e.g. Bug: crash on empty input").
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Key("goal").
				Title("Goal (optional)").
				Placeholder("What should this task achieve?"),
		),
	).WithWidth(p.width).WithHeight(10)
	return p.createForm.Init()
}

// flushFormCmds synchronously processes Huh form commands (e.g. NextField,
// nextGroup) so that field transitions and form completion happen immediately.
// This is needed because Huh uses async tea.Cmd for navigation, and tests do
// not run a full Bubble Tea runtime.
func (p *dagPane) flushFormCmds(cmd tea.Cmd) {
	if p.createForm == nil || cmd == nil {
		return
	}
	for i := 0; i < 30 && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		// Skip cursor blink messages to avoid infinite loops.
		if strings.Contains(fmt.Sprintf("%T", msg), "BlinkMsg") {
			break
		}
		// Handle BatchMsg: execute each sub-command in order.
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				m := c()
				if m == nil {
					continue
				}
				if strings.Contains(fmt.Sprintf("%T", m), "BlinkMsg") {
					continue
				}
				m2, _ := p.createForm.Update(m)
				if f, ok := m2.(*huh.Form); ok {
					p.createForm = f
				}
			}
			break
		}
		// Pass other messages (including sequenceMsg) through form.Update.
		m, c := p.createForm.Update(msg)
		if f, ok := m.(*huh.Form); ok {
			p.createForm = f
		}
		cmd = c
	}
}

func (p *dagPane) exitCreateMode() {
	p.creating = false
	p.createErr = nil
	p.createForm = nil
}

func (p *dagPane) submitCreate(title, goal string) tea.Cmd {
	if title == "" {
		p.createErr = errors.New("task title cannot be empty")
		return nil
	}
	p.exitCreateMode()
	return func() tea.Msg {
		return dagTaskCreatedMsg{
			Title:  title,
			Goal:   goal,
			RepoID: p.repoID,
		}
	}
}

func (p *dagPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *dagPane) View() tea.View {
	w := p.width
	if w <= 0 {
		w = 80
	}
	h := p.height
	if h <= 0 {
		h = 10
	}

	// header: 2 rows (title + hints), body: rest
	headerRows := 2

	var lines []string
	lines = append(lines,
		dagHeaderStyle.Render(" Task DAG ")+dagHintStyle.Render(dagHelpHint(w, p.creating)),
	)
	lines = append(lines, "")

	if !p.hasDAG() && !p.creating {
		lines = append(lines, dagMutedStyle.Render("  No tasks yet. Press [r] to refresh."))
		return tea.NewView(p.clampAndJoin(lines, h, w))
	}

	bodyH := h - headerRows
	if p.creating {
		bodyH = h - headerRows - 4 // reserve space for inputs + hint
	}
	if bodyH < minRows {
		bodyH = minRows
	}

	// Render full DAG without height limit, then apply vertical scrolling.
	dagStr := renderHorizontalDAG(p.nodes, p.edges, p.levels, p.layerIDs, p.maxLevel, p.cursorNode, w, 0)
	dagLines := strings.Split(dagStr, "\n")

	// Clamp scrollOffset in case DAG shrank (e.g. after refresh).
	if p.scrollOffset >= len(dagLines) {
		p.scrollOffset = max(0, len(dagLines)-bodyH)
	}

	end := p.scrollOffset + bodyH
	if end > len(dagLines) {
		end = len(dagLines)
	}

	if p.scrollOffset > 0 {
		lines = append(lines, dagMutedStyle.Render("  ▲ ..."))
	}
	lines = append(lines, dagLines[p.scrollOffset:end]...)
	if end < len(dagLines) {
		lines = append(lines, dagMutedStyle.Render("  ▼ ..."))
	}

	if p.creating && p.createForm != nil {
		lines = append(lines, "")
		lines = append(lines, p.createForm.View())
		if p.createErr != nil {
			lines = append(lines, dagErrorStyle.Render("Error: "+p.createErr.Error()))
		}
	}

	return tea.NewView(p.clampAndJoin(lines, h, w))
}

func dagHelpHint(width int, creating bool) string {
	if creating {
		if width < 72 {
			return "  [enter]save  [tab]switch  [esc]cancel"
		}
		return "  [T]asks [A]DRs  [Enter/Ctrl+S]save  [Tab]switch field  [Esc]cancel"
	}
	if width < 100 {
		return "  [j/k]move [h/l]level [enter/c]open/wt [s]state [t]todo [n]new [d]del [D]clear [z]expand [R]refresh"
	}
	return "  [j/k]move  [h/l]level  [enter]open/create  [c]new-wt  [s]state  [t]todo  [n]new-task  [d]del-task  [D]clear-all  [z]expand  [r]research  [a]arch  [T]asks [A]DRs  [R]refresh"
}

func (p *dagPane) clampAndJoin(lines []string, h, w int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(w).Render(line)
	}
	return strings.Join(lines, "\n")
}

var (
	dagHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	dagHintStyle    = lipgloss.NewStyle().Foreground(styles.Subtle)
	dagNodeStyle    = lipgloss.NewStyle().Foreground(styles.Text)
	dagFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Highlight).Background(lipgloss.Color("#333333"))
	dagMutedStyle   = lipgloss.NewStyle().Foreground(styles.Subtle)
	dagErrorStyle   = lipgloss.NewStyle().Foreground(styles.Warning)
)
