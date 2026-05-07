package app

import (
	"sort"
	"strings"

	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/models"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// dagPane shows the full Task DAG at the top of the screen.
// Humans navigate nodes here and decide which task to create a worktree for.
type dagPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel
	repoID string
	adapter adapters.GitAdapter

	tasks     []models.TaskContextRecord
	nodes     map[string]dagNode
	edges     []dagEdge
	levels    map[string]int
	layerIDs  map[int][]string
	maxLevel  int
	cursorNode string

	width  int
	height int
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

func (p *dagPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case dagRefreshMsg:
		p.buildDAG()
		return p, nil

	case tea.KeyMsg:
		switch msg.String() {
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

	p.nodes = make(map[string]dagNode, len(tasks))
	for _, t := range tasks {
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

	for _, t := range tasks {
		downstream, err := store.ListDownstreamTaskContexts(t.ID)
		if err != nil {
			continue
		}
		for _, d := range downstream {
			if _, ok := p.nodes[d.ID]; !ok {
				continue
			}
			key := t.ID + "->" + d.ID
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			e := dagEdge{From: t.ID, To: d.ID, Type: "hard"}
			p.edges = append(p.edges, e)
			adjacency[t.ID] = append(adjacency[t.ID], e)
			indegree[d.ID]++
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

	// default cursor
	if p.cursorNode == "" || p.nodes[p.cursorNode].ID == "" {
		for _, t := range tasks {
			if t.State == "ready" || t.State == "" {
				p.cursorNode = t.ID
				break
			}
		}
		if p.cursorNode == "" && len(tasks) > 0 {
			p.cursorNode = tasks[0].ID
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
		return
	}

	if dIndex != 0 {
		ids := p.layerIDs[curLevel]
		target := curIdx + dIndex
		if target < 0 || target >= len(ids) {
			return
		}
		p.cursorNode = ids[target]
	}
}

func (p *dagPane) resetCursor() {
	for _, t := range p.tasks {
		if t.State == "ready" || t.State == "" {
			p.cursorNode = t.ID
			return
		}
	}
	if len(p.tasks) > 0 {
		p.cursorNode = p.tasks[0].ID
	} else {
		p.cursorNode = ""
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
	return func() tea.Msg {
		return dagNodeSelectedMsg{
			TaskID:    task.ID,
			TaskTitle: task.Title,
			RepoID:    p.repoID,
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

type dagNodeSelectedMsg struct {
	TaskID    string
	TaskTitle string
	RepoID    string
}

type dagCreateWorktreeMsg struct {
	TaskID    string
	TaskTitle string
	RepoID    string
}

type dagLaunchAgentMsg struct {
	TaskID      string
	TaskTitle   string
	RepoID      string
	WorktreeID  string
	Provider    agents.Provider
	ExtraArgs   []string
}

func (p *dagPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *dagPane) View() string {
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
	bodyH := h - headerRows
	if bodyH < minRows {
		bodyH = minRows
	}

	var lines []string
	lines = append(lines,
		dagHeaderStyle.Render(" Task DAG ")+
			dagHintStyle.Render("  [j/k]↑↓  [h/l]←→  [enter]select  [c]wt  [r]research  [a]arch  [/]tab  [R]refresh"),
	)
	lines = append(lines, "")

	if !p.hasDAG() {
		lines = append(lines, dagMutedStyle.Render("  No tasks yet. Press [r] to refresh."))
		return p.clampAndJoin(lines, h, w)
	}


	dagStr := renderHorizontalDAG(p.nodes, p.edges, p.levels, p.layerIDs, p.maxLevel, p.cursorNode, w, bodyH)
	lines = append(lines, dagStr)

	return p.clampAndJoin(lines, h, w)
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
)
