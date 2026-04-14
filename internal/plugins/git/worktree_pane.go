package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ models.Panel = (*WorktreePane)(nil)

type WorktreePane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoPath  string
	worktrees []gitmodel.Worktree
	cursor    int
	loading   bool
	width     int
	height    int
	err       error
	notice    string
}

type worktreesLoadedMsg struct {
	worktrees []gitmodel.Worktree
	err       error
}

func NewWorktreePane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter) *WorktreePane {
	repoPath := meta.CWD
	if repoPath == "" {
		repoPath = "."
	}
	return &WorktreePane{
		id:       id,
		meta:     meta,
		common:   common,
		adapter:  adapter,
		repoPath: repoPath,
		loading:  true,
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
	case tea.KeyMsg:
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
		}
	}
	return p, nil
}

func (p *WorktreePane) View() string {
	width := p.width
	if width <= 0 {
		width = 40
	}

	if p.loading && len(p.worktrees) == 0 && p.err == nil {
		return loadingStyle.MaxWidth(width).Render("Loading worktrees…")
	}
	if p.err != nil && len(p.worktrees) == 0 {
		return errorStyle.MaxWidth(width).Render("Unable to load worktrees: " + p.err.Error())
	}

	lines := []string{sectionStyle.Render("Worktrees")}
	if p.repoPath != "" {
		lines = append(lines, upstreamStyle.Render(shortenWorktreePath(p.repoPath)))
	}
	lines = append(lines, "")

	if len(p.worktrees) == 0 {
		lines = append(lines, emptyStyle.Render("No worktrees found."))
	} else {
		for i, wt := range p.worktrees {
			line := p.renderWorktreeRow(wt)
			if i == p.cursor {
				line = selectedRowStyle.Render(line)
			}
			lines = append(lines, line)
		}
	}

	if p.notice != "" {
		lines = append(lines, "", upstreamStyle.Render(p.notice))
	}
	if p.err != nil {
		lines = append(lines, "", errorStyle.Render(p.err.Error()))
	}

	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	for i := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (p *WorktreePane) SetSize(width, height int) {
	p.width = width
	p.height = height
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
	if wt.DirtySummary.IsDirty() {
		tags = append(tags, fmt.Sprintf("dirty %d", wt.DirtySummary.Staged+wt.DirtySummary.Unstaged+wt.DirtySummary.Untracked+wt.DirtySummary.Conflicted))
	}

	label := wt.DisplayName()
	if wt.Branch != "" {
		label = branchStyle.Render(wt.Branch)
	}
	pathLine := emptyStyle.Render(shortenWorktreePath(wt.Path))
	if len(tags) == 0 {
		return label + "\n" + pathLine
	}
	return label + "  " + upstreamStyle.Render("["+strings.Join(tags, ", ")+"]") + "\n" + pathLine
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
