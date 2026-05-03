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
	cursor        int
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

type OpenWorktreeHistoryMsg struct {
	RepoPath string
}

type OpenWorktreeDeleteConfirmMsg struct {
	Worktree gitmodel.Worktree
	Force    bool
}

type RequestRemoveWorktreeMsg struct {
	Worktree gitmodel.Worktree
	Force    bool
}

type RequestPruneWorktreesMsg struct {
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
		p.loading = true
		p.err = nil
		p.notice = "Removed worktree " + shortenWorktreePath(msg.Path)
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
		switch msg.String() {
		case "j", "down":
			if p.cursor < len(p.visibleWorktrees())-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
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
				p.err = nil
				return p, func() tea.Msg {
					return OpenWorktreeDeleteConfirmMsg{Worktree: wt, Force: false}
				}
			}
		case "X":
			if wt, ok := p.selectedWorktree(); ok {
				if wt.IsMain {
					p.err = fmt.Errorf("cannot remove the main worktree")
					return p, nil
				}
				p.err = nil
				return p, func() tea.Msg {
					return OpenWorktreeDeleteConfirmMsg{Worktree: wt, Force: true}
				}
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
	summary := p.summaries[wt.Path]
	title := wt.DisplayName()
	if summary.TaskTitle != "" {
		title = summary.TaskTitle
	}
	branchText := wt.Branch
	if branchText == "" && wt.HeadOID != "" {
		branchText = wt.HeadOID[:7]
	}
	if branchText != "" {
		title += "  " + branchStyle.Render(branchText)
	}

	var marks []string
	if wt.IsMain {
		marks = append(marks, "main")
	}
	if wt.DirtySummary.IsDirty() {
		marks = append(marks, "*")
	}
	activity := p.activities[wt.Path]
	if activity.AgentCount > 0 {
		marks = append(marks, fmt.Sprintf("agent:%d", activity.AgentCount))
	}
	if len(marks) > 0 {
		title += "  " + upstreamStyle.Render(strings.Join(marks, " "))
	}
	return title
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
	if p.cursor >= len(ordered) {
		p.cursor = max(0, len(ordered)-1)
	}
	return ordered
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

func FormatDirtySummaryForUI(ds gitmodel.DirtySummary) string {
	return formatDirtySummary(ds)
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
