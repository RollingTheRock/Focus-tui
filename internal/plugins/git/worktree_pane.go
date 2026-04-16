package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"focus/internal/adapters"
	"focus/internal/agents"
	gitmodel "focus/internal/git"
	"focus/internal/models"
	"focus/internal/render"

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
	agentSessions map[string][]agents.Session
	cursor        int
	confirm       *worktreeConfirmState
	loading       bool
	width         int
	height        int
	err           error
	notice        string
}

type worktreesLoadedMsg struct {
	worktrees []gitmodel.Worktree
	err       error
}

type OpenWorktreeShellMsg struct {
	Worktree gitmodel.Worktree
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
			if p.cursor < len(p.worktrees)-1 {
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
		case "enter", "o":
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
		case "x":
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
		case "p":
			p.confirm = &worktreeConfirmState{kind: "prune"}
			p.err = nil
			return p, nil
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
	lines = append(lines, renderedLine{content: "", style: nil})

	if len(p.worktrees) == 0 {
		lines = append(lines, renderedLine{content: "No worktrees found.", style: &emptyStyle})
	} else {
		for i, wt := range p.worktrees {
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
	maxWidthStyle := lipgloss.NewStyle().MaxWidth(width)
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
	var tags []string
	if wt.IsMain {
		tags = append(tags, "main")
	}
	if wt.IsDetached {
		tags = append(tags, "detached")
	}
	if wt.IsLocked {
		tags = append(tags, "locked")
	}
	if wt.IsPrunable {
		tags = append(tags, "prunable")
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
		tags = append(tags, strings.Join(parts, " "))
	}
	activity := p.activities[wt.Path]
	if activity.HasShell {
		tags = append(tags, "shell")
	}
	if activity.OpenEditors > 0 {
		tags = append(tags, fmt.Sprintf("edits %d", activity.OpenEditors))
	}
	if sessions := p.agentSessions[wt.Path]; len(sessions) > 0 {
		for _, s := range sessions {
			tags = append(tags, s.DisplayName())
		}
	}
	if wt.AheadBehind.Ahead > 0 {
		tags = append(tags, aheadStyle.Render(fmt.Sprintf("↑%d", wt.AheadBehind.Ahead)))
	}
	if wt.AheadBehind.Behind > 0 {
		tags = append(tags, behindStyle.Render(fmt.Sprintf("↓%d", wt.AheadBehind.Behind)))
	}

	label := wt.DisplayName()
	if wt.Branch != "" {
		label = branchStyle.Render(wt.Branch)
	} else if wt.HeadOID != "" {
		label = upstreamStyle.Render(wt.HeadOID[:7])
	}
	pathLine := emptyStyle.Render(shortenWorktreePath(wt.Path))
	activityLine := ""
	if activity.LastActive != "" {
		activityLine = upstreamStyle.Render("active " + activity.LastActive)
	}
	if wt.Upstream != "" {
		if activityLine != "" {
			activityLine += "  "
		}
		activityLine += upstreamStyle.Render("-> " + wt.Upstream)
	}
	if len(tags) == 0 {
		if activityLine != "" {
			return label + "\n" + pathLine + "  " + activityLine
		}
		return label + "\n" + pathLine
	}
	row := label + "  " + upstreamStyle.Render("["+strings.Join(tags, ", ")+"]") + "\n" + pathLine
	if activityLine != "" {
		row += "  " + activityLine
	}
	return row
}

func (p *WorktreePane) selectedWorktree() (gitmodel.Worktree, bool) {
	if p.cursor < 0 || p.cursor >= len(p.worktrees) {
		return gitmodel.Worktree{}, false
	}
	return p.worktrees[p.cursor], true
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
