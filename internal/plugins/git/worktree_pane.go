package git

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"focus/internal/adapters"
	"focus/internal/agents"
	gitmodel "focus/internal/git"
	"focus/internal/models"
	"focus/internal/render"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	_ models.Panel    = (*WorktreePane)(nil)
	_ render.Renderer = (*WorktreePane)(nil)
)

var _ models.Panel = (*WorktreePane)(nil)

type WorktreePane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoPath      string
	worktrees     []gitmodel.Worktree
	activities    map[string]gitmodel.WorktreeActivity
	summaries     map[string]gitmodel.WorktreeResumeSummary
	agentSessions map[string][]agents.Session
	activeTab     worktreeTab
	cursor        int
	confirm       *worktreeConfirmState
	loading       bool
	width         int
	height        int
	err           error
	notice        string
}

type WorktreeContextView struct {
	Worktree gitmodel.Worktree
	Summary  gitmodel.WorktreeResumeSummary
	Activity gitmodel.WorktreeActivity
}

type worktreesLoadedMsg struct {
	worktrees []gitmodel.Worktree
	err       error
}

type worktreeTab string

const (
	worktreeTabAll    worktreeTab = "all"
	worktreeTabActive worktreeTab = "active"
	worktreeTabFocus  worktreeTab = "focus"
	worktreeTabQueued worktreeTab = "queued"
)

type OpenWorktreeShellMsg struct {
	Worktree gitmodel.Worktree
}

type ResumeWorktreeMsg struct {
	Worktree gitmodel.Worktree
}

type OpenTaskEditMsg struct {
	TaskID       string
	WorktreeID   string
	RepoID       string
	RelationType string
	ParentTaskID string
}

type OpenPlanEditMsg struct {
	TaskID     string
	WorktreeID string
	RepoID     string
}

type CycleTaskStateMsg struct {
	TaskID       string
	WorktreeID   string
	CurrentState string
}

type RequestRemoveWorktreeMsg struct {
	Worktree gitmodel.Worktree
	Force    bool
}

type RequestPruneWorktreesMsg struct {
	RepoPath string
}

type OpenWorktreeHistoryMsg struct {
	RepoPath string
}

type WorktreeRemovedMsg struct {
	Path  string
	Force bool
}

type WorktreesPrunedMsg struct{}

type WorktreeActionFailedMsg struct {
	Action string
	Err    error
}

type worktreeConfirmState struct {
	kind     string
	worktree gitmodel.Worktree
	force    bool
}

func NewWorktreePane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter) *WorktreePane {
	repoPath := meta.CWD
	if repoPath == "" {
		repoPath = "."
	}
	return &WorktreePane{
		id:         id,
		meta:       meta,
		common:     common,
		adapter:    adapter,
		repoPath:   repoPath,
		activities: make(map[string]gitmodel.WorktreeActivity),
		summaries:  make(map[string]gitmodel.WorktreeResumeSummary),
		activeTab:  worktreeTabAll,
		loading:    true,
	}
}

func (p *WorktreePane) Init() tea.Cmd {
	p.loading = true
	return p.loadWorktreesCmd()
}

func (p *WorktreePane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil {
			p.worktrees = msg.worktrees
			if p.cursor >= len(p.worktrees) {
				p.cursor = max(0, len(p.worktrees)-1)
			}
			p.notice = fmt.Sprintf("Loaded %d worktrees", len(p.worktrees))
		}
		return p, nil
	case WorktreeRemovedMsg:
		p.confirm = nil
		p.loading = true
		p.err = nil
		p.notice = "Removed worktree " + shortenWorktreePath(msg.Path)
		return p, p.loadWorktreesCmd()
	case WorktreesPrunedMsg:
		p.confirm = nil
		p.loading = true
		p.err = nil
		p.notice = "Pruned stale worktree metadata"
		return p, p.loadWorktreesCmd()
	case WorktreeActionFailedMsg:
		p.loading = false
		p.err = msg.Err
		return p, nil
	case RefreshWorktreesMsg:
		p.loading = true
		p.notice = ""
		return p, p.loadWorktreesCmd()
	case tea.KeyMsg:
		if p.confirm != nil {
			switch msg.String() {
			case "y":
				return p.confirmAction()
			case "n", "esc":
				p.confirm = nil
				p.notice = ""
				return p, nil
			default:
				return p, nil
			}
		}
		switch msg.String() {
		case "j", "down":
			if p.cursor < len(p.visibleWorktrees())-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "1":
			p.setActiveTab(worktreeTabAll)
		case "2":
			p.setActiveTab(worktreeTabActive)
		case "3":
			p.setActiveTab(worktreeTabFocus)
		case "4":
			p.setActiveTab(worktreeTabQueued)
		case "[":
			p.cycleTab(-1)
		case "]":
			p.cycleTab(1)
		case "r":
			p.loading = true
			p.notice = ""
			return p, p.loadWorktreesCmd()
		case "n":
			baseRef := "HEAD"
			if wt, ok := p.selectedWorktree(); ok && wt.Branch != "" {
				baseRef = wt.Branch
			}
			return p, func() tea.Msg {
				return OpenCreateWorktreeMsg{RepoPath: p.repoPath, BaseRef: baseRef}
			}
		case "enter":
			if wt, ok := p.selectedWorktree(); ok {
				p.notice = "Resuming " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return ResumeWorktreeMsg{Worktree: wt}
				}
			}
		case "o":
			if wt, ok := p.selectedWorktree(); ok {
				p.notice = "Opening shell in " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return OpenWorktreeShellMsg{Worktree: wt}
				}
			}
		case "a":
			if wt, ok := p.selectedWorktree(); ok {
				provider := agents.DefaultProvider()
				p.notice = "Launching " + string(provider) + " in " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return agents.LaunchAgentMsg{WorktreeID: wt.Path, Provider: provider}
				}
			}
		case "e":
			if wt, ok := p.selectedWorktree(); ok {
				summary := p.summaries[wt.Path]
				p.notice = "Editing task for " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return OpenTaskEditMsg{TaskID: summary.TaskID, WorktreeID: wt.Path, RepoID: p.repoPath, RelationType: "primary"}
				}
			}
		case "f":
			if wt, ok := p.selectedWorktree(); ok {
				summary := p.summaries[wt.Path]
				p.notice = "Adding follow-up for " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return OpenTaskEditMsg{WorktreeID: wt.Path, RepoID: p.repoPath, RelationType: "queued", ParentTaskID: summary.TaskID}
				}
			}
		case "p":
			if wt, ok := p.selectedWorktree(); ok {
				summary := p.summaries[wt.Path]
				p.notice = "Drafting plan for " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return OpenPlanEditMsg{TaskID: summary.TaskID, WorktreeID: wt.Path, RepoID: p.repoPath}
				}
			}
		case "s":
			if wt, ok := p.selectedWorktree(); ok {
				summary := p.summaries[wt.Path]
				if summary.TaskID == "" {
					p.err = fmt.Errorf("create or attach a primary task before cycling state")
					return p, nil
				}
				p.notice = "Cycling task state for " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return CycleTaskStateMsg{TaskID: summary.TaskID, WorktreeID: wt.Path, CurrentState: summary.TaskState}
				}
			}
		case "d", "x":
			if wt, ok := p.selectedWorktree(); ok {
				if wt.IsMain {
					p.err = fmt.Errorf("cannot remove the main worktree")
					return p, nil
				}
				if wt.DirtySummary.IsDirty() || wt.IsLocked {
					p.err = fmt.Errorf("worktree is dirty or locked; press Shift+X to force remove")
					return p, nil
				}
				p.confirm = &worktreeConfirmState{kind: "remove", worktree: wt}
				p.err = nil
				return p, nil
			}
		case "X":
			if wt, ok := p.selectedWorktree(); ok {
				if wt.IsMain {
					p.err = fmt.Errorf("cannot remove the main worktree")
					return p, nil
				}
				p.confirm = &worktreeConfirmState{kind: "remove", worktree: wt, force: true}
				p.err = nil
				return p, nil
			}
		case "P":
			p.confirm = &worktreeConfirmState{kind: "prune"}
			p.err = nil
			return p, nil
		case "H":
			return p, func() tea.Msg {
				return OpenWorktreeHistoryMsg{RepoPath: p.repoPath}
			}
		}
	}
	return p, nil
}

func (p *WorktreePane) View() string {
	width := p.width
	if width <= 0 {
		width = 40
	}
	height := p.height
	if height <= 0 {
		height = 24
	}
	canvas := render.NewCanvas(width, height)
	p.Render(canvas, width, height)
	return canvas.Render()
}

func (p *WorktreePane) Render(canvas render.Surface, width, height int) {
	if p.loading && len(p.worktrees) == 0 && p.err == nil {
		canvas.SetString(0, 0, loadingStyle.Render("Loading worktrees…"), nil)
		return
	}
	if p.err != nil && len(p.worktrees) == 0 {
		canvas.SetString(0, 0, errorStyle.Render("Unable to load worktrees: "+p.err.Error()), nil)
		return
	}

	var lines []renderedLine
	lines = append(lines, renderedLine{content: "Worktrees", style: &sectionStyle})
	if p.repoPath != "" {
		lines = append(lines, renderedLine{content: shortenWorktreePath(p.repoPath), style: &upstreamStyle})
	}
	lines = append(lines, renderedLine{content: p.renderTabs(), style: nil})
	lines = append(lines, renderedLine{content: "", style: nil})

	visible := p.visibleWorktrees()
	if len(visible) == 0 {
		lines = append(lines, renderedLine{content: "No worktrees found.", style: &emptyStyle})
	} else {
		for i, wt := range visible {
			line := p.renderWorktreeRow(wt)
			style := (*lipgloss.Style)(nil)
			if i == p.cursor {
				style = &selectedRowStyle
			}
			lines = append(lines, renderedLine{content: line, style: style})
		}
	}

	if p.notice != "" {
		lines = append(lines, renderedLine{content: "", style: nil})
		lines = append(lines, renderedLine{content: p.notice, style: &upstreamStyle})
	}
	if selected, ok := p.selectedWorktree(); ok {
		for _, detail := range p.renderSelectedDetails(selected) {
			lines = append(lines, renderedLine{content: detail, style: &upstreamStyle})
		}
	}
	if p.confirm != nil {
		lines = append(lines, renderedLine{content: "", style: nil})
		lines = append(lines, renderedLine{content: p.renderConfirmPrompt(), style: nil})
	}
	if p.err != nil {
		lines = append(lines, renderedLine{content: "", style: nil})
		lines = append(lines, renderedLine{content: p.err.Error(), style: &errorStyle})
	}

	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	maxWidthStyle := styles.StyleCache.MaxWidth(width)
	for y, line := range lines {
		if y >= height {
			break
		}
		s := line.style
		if s == nil {
			canvas.SetString(0, y, maxWidthStyle.Render(line.content), nil)
		} else {
			canvas.SetString(0, y, maxWidthStyle.Render(s.Render(line.content)), nil)
		}
	}
}

type renderedLine struct {
	content string
	style   *lipgloss.Style
}

func (p *WorktreePane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *WorktreePane) SetActivity(worktreeID string, activity gitmodel.WorktreeActivity) {
	if p.activities == nil {
		p.activities = make(map[string]gitmodel.WorktreeActivity)
	}
	p.activities[worktreeID] = activity
}

func (p *WorktreePane) SetAgentSessions(sessions map[string][]agents.Session) {
	p.agentSessions = sessions
}

func (p *WorktreePane) SetResumeSummaries(summaries map[string]gitmodel.WorktreeResumeSummary) {
	p.summaries = summaries
}

func (p *WorktreePane) loadWorktreesCmd() tea.Cmd {
	return func() tea.Msg {
		if p.adapter == nil {
			return worktreesLoadedMsg{err: fmt.Errorf("git adapter unavailable")}
		}
		worktrees, err := p.adapter.ListWorktrees(p.repoPath)
		return worktreesLoadedMsg{worktrees: worktrees, err: err}
	}
}

func (p *WorktreePane) renderWorktreeRow(wt gitmodel.Worktree) string {
	var titleTags []string
	var statusParts []string
	if wt.IsMain {
		titleTags = append(titleTags, "main")
	}
	if wt.IsDetached {
		titleTags = append(titleTags, "detached")
	}
	if wt.IsLocked {
		titleTags = append(titleTags, "locked")
	}
	if wt.IsPrunable {
		titleTags = append(titleTags, "prunable")
	}
	ds := wt.DirtySummary
	if ds.IsDirty() {
		var parts []string
		if ds.Staged > 0 {
			parts = append(parts, fmt.Sprintf("+%d", ds.Staged))
		}
		if ds.Unstaged > 0 {
			parts = append(parts, fmt.Sprintf("~%d", ds.Unstaged))
		}
		if ds.Untracked > 0 {
			parts = append(parts, fmt.Sprintf("?%d", ds.Untracked))
		}
		if ds.Conflicted > 0 {
			parts = append(parts, fmt.Sprintf("!%d", ds.Conflicted))
		}
		statusParts = append(statusParts, "git "+strings.Join(parts, " "))
	}
	activity := p.activities[wt.Path]
	summary := p.summaries[wt.Path]
	if summary.TaskState != "" {
		statusParts = append(statusParts, summary.TaskState)
	}
	if summary.TaskPriority == "high" {
		statusParts = append(statusParts, "high")
	}
	if summary.TaskMode != "" && summary.TaskMode != "single" {
		statusParts = append(statusParts, summary.TaskMode)
	}
	if summary.GitPressure != "" && summary.GitPressure != "clean" {
		statusParts = append(statusParts, summary.GitPressure)
	}
	if activity.AgentCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("agent %d", activity.AgentCount))
	}
	if summary.QueuedTaskCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("queued %d", summary.QueuedTaskCount))
	}
	if wt.AheadBehind.Ahead > 0 {
		statusParts = append(statusParts, fmt.Sprintf("↑%d", wt.AheadBehind.Ahead))
	}
	if wt.AheadBehind.Behind > 0 {
		statusParts = append(statusParts, fmt.Sprintf("↓%d", wt.AheadBehind.Behind))
	}

	title := wt.DisplayName()
	if summary.TaskTitle != "" {
		title = summary.TaskTitle
	}
	branchText := wt.Branch
	if wt.Branch != "" {
		branchText = wt.Branch
	} else if wt.HeadOID != "" {
		branchText = wt.HeadOID[:7]
	}
	titleLine := title
	if branchText != "" {
		titleLine += "  " + branchStyle.Render(branchText)
	}
	if len(titleTags) > 0 {
		titleLine += "  " + upstreamStyle.Render("["+strings.Join(titleTags, ", ")+"]")
	}

	statusLineParts := []string{}
	if len(statusParts) > 0 {
		statusLineParts = append(statusLineParts, strings.Join(statusParts, " · "))
	}
	if summary.LastActiveLabel != "" {
		statusLineParts = append(statusLineParts, summary.LastActiveLabel)
	} else if activity.LastActive != "" {
		statusLineParts = append(statusLineParts, "active "+activity.LastActive)
	}
	if summary.LastAgentLabel != "" {
		statusLineParts = append(statusLineParts, summary.LastAgentLabel)
	}
	if wt.Upstream != "" {
		statusLineParts = append(statusLineParts, "-> "+wt.Upstream)
	}
	statusLine := emptyStyle.Render(strings.Join(statusLineParts, "  "))

	pathLine := emptyStyle.Render(shortenWorktreePath(wt.Path))
	cue := primaryQueueCue(summary)
	if cue != "" {
		pathLine += "  " + upstreamStyle.Render(cue)
	}
	if strings.TrimSpace(statusLine) == "" {
		return titleLine + "\n" + pathLine
	}
	return titleLine + "\n" + statusLine + "\n" + pathLine
}

func (p *WorktreePane) renderSelectedDetails(wt gitmodel.Worktree) []string {
	summary := p.summaries[wt.Path]
	activity := p.activities[wt.Path]
	var lines []string
	if summary.LastResumeHint == "" && summary.TaskGoal == "" && summary.ResumeReason == "" && !wt.DirtySummary.IsDirty() && summary.LastAgentSummary == "" && summary.QueuedTaskTitle == "" {
		return lines
	}
	lines = append(lines, "")
	lines = append(lines, "Selected: "+wt.DisplayName())
	if summary.TaskGoal != "" {
		lines = append(lines, "Goal: "+summary.TaskGoal)
	}
	if summary.LastResumeHint != "" {
		lines = append(lines, "Next: "+summary.LastResumeHint)
	}
	if summary.ResumeReason != "" {
		lines = append(lines, "Why now: "+summary.ResumeReason)
	}
	if summary.LastAgentSummary != "" {
		lines = append(lines, "Agent: "+summary.LastAgentSummary)
	}
	if summary.QueuedTaskTitle != "" {
		queued := "Queued: " + summary.QueuedTaskTitle
		if summary.QueuedTaskCount > 1 {
			queued += fmt.Sprintf(" (+%d more)", summary.QueuedTaskCount-1)
		}
		lines = append(lines, queued)
	}
	if wt.DirtySummary.IsDirty() {
		lines = append(lines, "Git: "+formatDirtySummary(wt.DirtySummary))
	}
	if wt.AheadBehind.Ahead > 0 || wt.AheadBehind.Behind > 0 {
		lines = append(lines, fmt.Sprintf("Upstream: ↑%d ↓%d", wt.AheadBehind.Ahead, wt.AheadBehind.Behind))
	}
	if activity.HasShell || activity.OpenEditors > 0 || activity.AgentCount > 0 {
		lines = append(lines, fmt.Sprintf("Runtime: shell=%t edits=%d agents=%d", activity.HasShell, activity.OpenEditors, activity.AgentCount))
	}
	return lines
}

func formatDirtySummary(ds gitmodel.DirtySummary) string {
	parts := []string{}
	if ds.Staged > 0 {
		parts = append(parts, fmt.Sprintf("+%d staged", ds.Staged))
	}
	if ds.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("~%d unstaged", ds.Unstaged))
	}
	if ds.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("?%d untracked", ds.Untracked))
	}
	if ds.Conflicted > 0 {
		parts = append(parts, fmt.Sprintf("!%d conflicted", ds.Conflicted))
	}
	return strings.Join(parts, " · ")
}

func primaryQueueCue(summary gitmodel.WorktreeResumeSummary) string {
	switch {
	case summary.NextStep != "":
		return "next: " + summary.NextStep
	case summary.BlockerNote != "":
		return "blocked: " + summary.BlockerNote
	case summary.HandoffNote != "":
		return "handoff: " + summary.HandoffNote
	case summary.AttentionAnchor != "":
		return summary.AttentionAnchor
	case summary.QueuedTaskTitle != "":
		return "queued: " + summary.QueuedTaskTitle
	case summary.ResumeReason != "":
		return summary.ResumeReason
	default:
		return ""
	}
}

func FormatDirtySummaryForUI(ds gitmodel.DirtySummary) string {
	return formatDirtySummary(ds)
}

func (p *WorktreePane) SelectedContext() (gitmodel.Worktree, gitmodel.WorktreeResumeSummary, gitmodel.WorktreeActivity, bool) {
	wt, ok := p.selectedWorktree()
	if !ok {
		return gitmodel.Worktree{}, gitmodel.WorktreeResumeSummary{}, gitmodel.WorktreeActivity{}, false
	}
	return wt, p.summaries[wt.Path], p.activities[wt.Path], true
}

func (p *WorktreePane) OrderedContexts() []WorktreeContextView {
	ordered := p.orderedWorktrees()
	items := make([]WorktreeContextView, 0, len(ordered))
	for _, wt := range ordered {
		items = append(items, WorktreeContextView{
			Worktree: wt,
			Summary:  p.summaries[wt.Path],
			Activity: p.activities[wt.Path],
		})
	}
	return items
}

func (p *WorktreePane) selectedWorktree() (gitmodel.Worktree, bool) {
	ordered := p.orderedWorktrees()
	if p.cursor < 0 || p.cursor >= len(ordered) {
		return gitmodel.Worktree{}, false
	}
	visible := p.visibleWorktrees()
	if p.cursor < 0 || p.cursor >= len(visible) {
		return gitmodel.Worktree{}, false
	}
	return visible[p.cursor], true
}

func (p *WorktreePane) orderedWorktrees() []gitmodel.Worktree {
	ordered := append([]gitmodel.Worktree(nil), p.worktrees...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := p.summaries[ordered[i].Path]
		right := p.summaries[ordered[j].Path]
		if left.ResumeScore != right.ResumeScore {
			return left.ResumeScore > right.ResumeScore
		}
		leftActivity := p.activities[ordered[i].Path]
		rightActivity := p.activities[ordered[j].Path]
		if leftActivity.LastActive != rightActivity.LastActive {
			return leftActivity.LastActive > rightActivity.LastActive
		}
		return false
	})
	return ordered
}

func (p *WorktreePane) visibleWorktrees() []gitmodel.Worktree {
	ordered := p.orderedWorktrees()
	visible := make([]gitmodel.Worktree, 0, len(ordered))
	for _, wt := range ordered {
		summary := p.summaries[wt.Path]
		switch p.activeTab {
		case worktreeTabActive:
			if summary.TaskState != "active" {
				continue
			}
		case worktreeTabFocus:
			if !wt.DirtySummary.IsDirty() && summary.LastAgentSummary == "" && summary.ResumeScore < 60 {
				continue
			}
		case worktreeTabQueued:
			if summary.QueuedTaskCount == 0 {
				continue
			}
		}
		visible = append(visible, wt)
	}
	if p.cursor >= len(visible) {
		p.cursor = max(0, len(visible)-1)
	}
	return visible
}

func (p *WorktreePane) setActiveTab(tab worktreeTab) {
	p.activeTab = tab
	p.cursor = 0
	p.notice = "Tab: " + strings.ToUpper(string(tab))
}

func (p *WorktreePane) cycleTab(delta int) {
	tabs := []worktreeTab{worktreeTabAll, worktreeTabActive, worktreeTabFocus, worktreeTabQueued}
	idx := 0
	for i, tab := range tabs {
		if tab == p.activeTab {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(tabs)) % len(tabs)
	p.setActiveTab(tabs[idx])
}

func (p *WorktreePane) renderTabs() string {
	visibleCounts := map[worktreeTab]int{
		worktreeTabAll:    len(p.orderedWorktrees()),
		worktreeTabActive: 0,
		worktreeTabFocus:  0,
		worktreeTabQueued: 0,
	}
	for _, wt := range p.orderedWorktrees() {
		summary := p.summaries[wt.Path]
		if summary.TaskState == "active" {
			visibleCounts[worktreeTabActive]++
		}
		if wt.DirtySummary.IsDirty() || summary.LastAgentSummary != "" || summary.ResumeScore >= 60 {
			visibleCounts[worktreeTabFocus]++
		}
		if summary.QueuedTaskCount > 0 {
			visibleCounts[worktreeTabQueued]++
		}
	}
	tabs := []struct {
		key   string
		tab   worktreeTab
		label string
	}{
		{"1", worktreeTabAll, "ALL"},
		{"2", worktreeTabActive, "ACTIVE"},
		{"3", worktreeTabFocus, "FOCUS"},
		{"4", worktreeTabQueued, "QUEUED"},
	}
	parts := make([]string, 0, len(tabs))
	for _, item := range tabs {
		text := fmt.Sprintf("%s %s(%d)", item.key, item.label, visibleCounts[item.tab])
		if item.tab == p.activeTab {
			parts = append(parts, sectionStyle.Render("["+text+"]"))
		} else {
			parts = append(parts, upstreamStyle.Render(text))
		}
	}
	return strings.Join(parts, "  ")
}

func (p *WorktreePane) confirmAction() (models.Panel, tea.Cmd) {
	if p.confirm == nil {
		return p, nil
	}
	confirm := *p.confirm
	p.confirm = nil
	p.loading = true
	p.err = nil
	switch confirm.kind {
	case "remove":
		return p, func() tea.Msg {
			return RequestRemoveWorktreeMsg{Worktree: confirm.worktree, Force: confirm.force}
		}
	case "prune":
		return p, func() tea.Msg {
			return RequestPruneWorktreesMsg{RepoPath: p.repoPath}
		}
	default:
		p.loading = false
		return p, nil
	}
}

func (p *WorktreePane) renderConfirmPrompt() string {
	if p.confirm == nil {
		return ""
	}
	switch p.confirm.kind {
	case "remove":
		verb := "Remove"
		if p.confirm.force {
			verb = "Force remove"
		}
		return commitHintWarningStyle.Render(fmt.Sprintf("%s %s? [y/n]", verb, shortenWorktreePath(p.confirm.worktree.Path)))
	case "prune":
		return commitHintWarningStyle.Render("Prune stale worktree metadata? [y/n]")
	default:
		return ""
	}
}

func shortenWorktreePath(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == "." || parent == string(filepath.Separator) || parent == "" {
		return base
	}
	return filepath.Join(parent, base)
}
