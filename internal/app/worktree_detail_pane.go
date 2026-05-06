package app

import (
	"fmt"
	"strings"

	"focus/internal/adapters"
	"focus/internal/agents"
	"focus/internal/models"
	filebrowser "focus/internal/plugins/filebrowser"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// worktreeDetailPane is the right-side pane showing the current worktree's
// tasks, git status, file tree, agent cards, and a human shell below.
type worktreeDetailPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel
	adapter adapters.GitAdapter

	repoID     string
	worktreeID string

	activeTab detailTab

	// Sub-panes (re-use original component shapes)
	gitPane   *gitplugin.StatusPane
	filesPane *filebrowser.TreePane

	// Tasks for this worktree
	tasks  []models.TaskContextRecord
	taskCursor int

	// Agent sessions attached to this worktree
	sessions []*agents.Session

	width  int
	height int
}

type detailTab int

const (
	tabTasks detailTab = iota
	tabGit
	tabFiles
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
	p.gitPane = nil
	p.filesPane = nil
	p.initSubPanes()
	p.loadTasks()

	var cmds []tea.Cmd
	if p.gitPane != nil {
		if cmd := p.gitPane.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if p.filesPane != nil {
		if cmd := p.filesPane.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (p *worktreeDetailPane) initSubPanes() {
	if p.worktreeID == "" {
		return
	}
	gitMeta := models.PaneMeta{
		ID:         "detail-git",
		Name:       "Git",
		Type:       models.PaneTypeGitStatus,
		CWD:        p.worktreeID,
		RepoID:     p.repoID,
		WorktreeID: p.worktreeID,
		Status:     models.PaneStatusIdle,
		Closable:   false,
	}
	p.gitPane = gitplugin.NewStatusPane(gitMeta.ID, gitMeta, p.common, p.adapter)

	filesMeta := models.PaneMeta{
		ID:         "detail-files",
		Name:       "Files",
		Type:       models.PaneTypeFileTree,
		CWD:        p.worktreeID,
		RepoID:     p.repoID,
		WorktreeID: p.worktreeID,
		Status:     models.PaneStatusIdle,
		Closable:   false,
	}
	p.filesPane = filebrowser.NewTreePane(filesMeta.ID, filesMeta, p.common)
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
	p.initSubPanes()
	p.loadTasks()

	var cmds []tea.Cmd
	if p.gitPane != nil {
		if cmd := p.gitPane.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if p.filesPane != nil {
		if cmd := p.filesPane.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
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
	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			p.activeTab = tabTasks
			return p, nil
		case "2":
			p.activeTab = tabGit
			return p, nil
		case "3":
			p.activeTab = tabFiles
			return p, nil
		case "j", "down":
			if p.activeTab == tabTasks && p.taskCursor < len(p.tasks)-1 {
				p.taskCursor++
			}
			return p, nil
		case "k", "up":
			if p.activeTab == tabTasks && p.taskCursor > 0 {
				p.taskCursor--
			}
			return p, nil
		case "s":
			return p, p.launchAgentCmd()
		case "tab":
			return p, nil // let app route to next pane
		}
	}

	// Route to active sub-pane.
	switch p.activeTab {
	case tabGit:
		if p.gitPane != nil {
			newPane, cmd := p.gitPane.Update(msg)
			if gp, ok := newPane.(*gitplugin.StatusPane); ok {
				p.gitPane = gp
			}
			return p, cmd
		}
	case tabFiles:
		if p.filesPane != nil {
			newPane, cmd := p.filesPane.Update(msg)
			if fp, ok := newPane.(*filebrowser.TreePane); ok {
				p.filesPane = fp
			}
			return p, cmd
		}
	}
	return p, nil
}

type worktreeSelectedMsg struct {
	WorktreeID string
}

func (p *worktreeDetailPane) launchAgentCmd() tea.Cmd {
	if p.worktreeID == "" {
		return nil
	}
	return func() tea.Msg {
		return agents.LaunchAgentMsg{
			WorktreeID: p.worktreeID,
			Provider:   agents.DefaultProvider(),
		}
	}
}

func (p *worktreeDetailPane) refreshSessions() {
	// Sessions will be refreshed by the app layer pushing them in.
}

func (p *worktreeDetailPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	if p.gitPane != nil {
		p.gitPane.SetSize(width, contentHeight(height))
	}
	if p.filesPane != nil {
		p.filesPane.SetSize(width, contentHeight(height))
	}
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

func (p *worktreeDetailPane) View() string {
	w := p.width
	h := p.height
	if w <= 0 || h <= 0 {
		return ""
	}

	var parts []string
	parts = append(parts, p.renderTabs(w))

	ch := contentHeight(h)
	switch p.activeTab {
	case tabGit:
		if p.gitPane != nil {
			p.gitPane.SetSize(w, ch)
			parts = append(parts, p.gitPane.View())
		} else {
			parts = append(parts, emptyLine(w, ch))
		}
	case tabFiles:
		if p.filesPane != nil {
			p.filesPane.SetSize(w, ch)
			parts = append(parts, p.filesPane.View())
		} else {
			parts = append(parts, emptyLine(w, ch))
		}
	default:
		parts = append(parts, p.renderTasks(w, ch))
	}

	parts = append(parts, p.renderAgentCards(w))
	return strings.Join(parts, "\n")
}

func (p *worktreeDetailPane) renderTabs(w int) string {
	tabs := []struct {
		key   string
		label string
		tab   detailTab
	}{
		{"1", "Tasks", tabTasks},
		{"2", "Git", tabGit},
		{"3", "Files", tabFiles},
	}
	var parts []string
	for _, t := range tabs {
		label := t.key + " " + t.label
		if t.tab == p.activeTab {
			parts = append(parts, detailTabActiveStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, detailTabStyle.Render(label))
		}
	}
	return lipgloss.NewStyle().MaxWidth(w).Render("  " + strings.Join(parts, "  "))
}

func (p *worktreeDetailPane) renderTasks(w, h int) string {
	if len(p.tasks) == 0 {
		return lipgloss.NewStyle().MaxWidth(w).Render("  No tasks assigned to this worktree.")
	}
	var lines []string
	for i, t := range p.tasks {
		state := t.State
		if state == "" {
			state = "pending"
		}
		cursor := "  ·"
		if i == p.taskCursor {
			cursor = "▸"
		}
		line := fmt.Sprintf("%s [%s] %s", cursor, state, clipText(t.Title, w-20))
		lines = append(lines, line)
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(strings.Join(lines, "\n"))
}

func (p *worktreeDetailPane) renderAgentCards(w int) string {
	if len(p.sessions) == 0 {
		return lipgloss.NewStyle().MaxWidth(w).Foreground(styles.Subtle).Render("  Agents: none  [s] start")
	}
	var parts []string
	for _, s := range p.sessions {
		icon := "○"
		if s.State == agents.SessionRunning {
			icon = "●"
		}
		parts = append(parts, fmt.Sprintf("%s %s(%s)", icon, s.Provider, s.State))
	}
	return lipgloss.NewStyle().MaxWidth(w).Render("  Agents: " + strings.Join(parts, "  "))
}

func (p *worktreeDetailPane) SetAgentSessions(sessions []*agents.Session) {
	p.sessions = sessions
}

func emptyLine(w, h int) string {
	return lipgloss.NewStyle().Width(w).Height(h).Render("")
}

var (
	detailTabStyle      = lipgloss.NewStyle().Foreground(styles.Subtle)
	detailTabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
)
