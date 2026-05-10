package git

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type OpenCreateWorktreeMsg struct {
	RepoPath  string
	BaseRef   string
	TaskTitle string
	TaskID    string
}

type WorktreeCreatedMsg struct {
	ID           models.PaneID
	Worktree     gitmodel.Worktree
	OpenExternal bool
	TaskTitle    string
	TaskID       string
}

type CloseCreateWorktreeMsg struct {
	ID models.PaneID
}

type RefreshWorktreesMsg struct{}

type createWorktreeFinishedMsg struct {
	worktree *gitmodel.Worktree
	err      error
}

type WorktreeCreatePane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	adapter adapters.GitAdapter

	repoPath     string
	width        int
	height       int
	inputs       []textinput.Model
	focus        int
	err          error
	creating     bool
	pathAuto     bool
	openExternal bool
	taskName     string
	taskID       string
}

const (
	createWorktreeFieldTask = iota
	createWorktreeFieldBranch
	createWorktreeFieldBaseRef
	createWorktreeFieldPath
)

var worktreeSlugRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func NewWorktreeCreatePane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, adapter adapters.GitAdapter, msg OpenCreateWorktreeMsg) *WorktreeCreatePane {
	repoPath := msg.RepoPath
	if repoPath == "" {
		repoPath = meta.CWD
	}
	if repoPath == "" {
		repoPath = "."
	}
	baseRef := strings.TrimSpace(msg.BaseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}

	taskInput := textinput.New()
	taskInput.Prompt = "Task (optional): "
	taskInput.Placeholder = "What are you working on?"
	if msg.TaskTitle != "" {
		taskInput.SetValue(msg.TaskTitle)
	}

	branchInput := textinput.New()
	branchInput.Prompt = "Branch: "
	branchInput.Placeholder = "feature/worktree-pane"

	baseRefInput := textinput.New()
	baseRefInput.Prompt = "Base ref: "
	baseRefInput.Placeholder = "HEAD"
	baseRefInput.SetValue(baseRef)

	pathInput := textinput.New()
	pathInput.Prompt = "Path: "
	defaultPath := defaultWorktreePath(repoPath, branchInput.Value())
	pathInput.Placeholder = defaultPath
	pathInput.SetValue(defaultPath)

	inputs := []textinput.Model{taskInput, branchInput, baseRefInput, pathInput}
	applyCreateInputStyles(inputs)
	inputs[0].Focus()

	return &WorktreeCreatePane{
		id:           id,
		meta:         meta,
		common:       common,
		adapter:      adapter,
		repoPath:     repoPath,
		inputs:       inputs,
		pathAuto:     true,
		openExternal: true,
		taskID:       msg.TaskID,
	}
}

func (p *WorktreeCreatePane) Init() tea.Cmd {
	return textinput.Blink
}

func (p *WorktreeCreatePane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case createWorktreeFinishedMsg:
		p.creating = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		if msg.worktree == nil {
			p.err = errors.New("worktree creation returned no result")
			return p, nil
		}
		p.err = nil
		return p, func() tea.Msg {
			return WorktreeCreatedMsg{ID: p.id, Worktree: *msg.worktree, OpenExternal: p.openExternal, TaskTitle: p.taskName, TaskID: p.taskID}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if p.creating {
				return p, nil
			}
			return p, closeCreateWorktreeCmd(p.id)
		case "tab":
			p.moveFocus(1)
			return p, nil
		case "shift+tab":
			p.moveFocus(-1)
			return p, nil
		case "enter":
			if p.focus == createWorktreeFieldPath {
				return p.submit()
			}
			p.moveFocus(1)
			return p, nil
		case "ctrl+s":
			return p.submit()
		case "ctrl+t":
			p.openExternal = !p.openExternal
			return p, nil
		}
	}

	var cmd tea.Cmd
	for i := range p.inputs {
		p.inputs[i], cmd = p.inputs[i].Update(msg)
	}
	if p.pathAuto {
		defaultPath := defaultWorktreePath(p.repoPath, p.inputs[createWorktreeFieldBranch].Value())
		p.inputs[createWorktreeFieldPath].SetValue(defaultPath)
	}
	pathValue := strings.TrimSpace(p.inputs[createWorktreeFieldPath].Value())
	defaultPath := defaultWorktreePath(p.repoPath, p.inputs[createWorktreeFieldBranch].Value())
	p.pathAuto = pathValue == "" || pathValue == defaultPath
	return p, cmd
}

func (p *WorktreeCreatePane) View() string {
	width := p.width
	if width <= 0 {
		width = 72
	}

	modeHint := "shell"
	if p.openExternal {
		modeHint = "external terminal"
	}
	lines := []string{
		commitHeaderStyle.Render("Create Worktree"),
		upstreamStyle.Render("Create a branch-backed worktree from the current repository."),
		"",
		p.inputs[createWorktreeFieldTask].View(),
		p.inputs[createWorktreeFieldBranch].View(),
		p.inputs[createWorktreeFieldBaseRef].View(),
		p.inputs[createWorktreeFieldPath].View(),
		"",
		commitHintStyle.Render("Tab move · Enter next/submit · Ctrl+S create · Ctrl+T " + modeHint + " · Esc cancel"),
	}
	if p.creating {
		lines = append(lines, upstreamStyle.Render("Creating worktree..."))
	}
	if p.err != nil {
		lines = append(lines, errorStyle.Render("Create failed: "+p.err.Error()))
	}

	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	for i := range lines {
		lines[i] = appstyles.StyleCache.MaxWidth(width).Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (p *WorktreeCreatePane) SetSize(width, height int) {
	p.width = width
	p.height = height
	inputWidth := width - 6
	if inputWidth < 24 {
		inputWidth = 24
	}
	for i := range p.inputs {
		p.inputs[i].Width = inputWidth
	}
}

func (p *WorktreeCreatePane) moveFocus(delta int) {
	p.focus = (p.focus + delta + len(p.inputs)) % len(p.inputs)
	for i := range p.inputs {
		if i == p.focus {
			p.inputs[i].Focus()
		} else {
			p.inputs[i].Blur()
		}
	}
}

func (p *WorktreeCreatePane) submit() (models.Panel, tea.Cmd) {
	if p.creating {
		return p, nil
	}
	p.taskName = strings.TrimSpace(p.inputs[createWorktreeFieldTask].Value())
	branch := strings.TrimSpace(p.inputs[createWorktreeFieldBranch].Value())
	baseRef := strings.TrimSpace(p.inputs[createWorktreeFieldBaseRef].Value())
	path := strings.TrimSpace(p.inputs[createWorktreeFieldPath].Value())
	if branch == "" {
		p.err = errors.New("branch name cannot be empty")
		return p, nil
	}
	if path == "" {
		p.err = errors.New("worktree path cannot be empty")
		return p, nil
	}
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if p.adapter == nil {
		p.err = errors.New("git adapter is not configured")
		return p, nil
	}
	request := gitmodel.CreateWorktreeRequest{Path: path, Branch: branch, BaseRef: baseRef}
	p.creating = true
	p.err = nil
	return p, func() tea.Msg {
		worktree, err := p.adapter.CreateWorktree(p.repoPath, request)
		return createWorktreeFinishedMsg{worktree: worktree, err: err}
	}
}

func closeCreateWorktreeCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return CloseCreateWorktreeMsg{ID: id} }
}

func defaultWorktreePath(repoPath, branch string) string {
	slug := slugifyWorktreeBranch(branch)
	if slug == "" {
		slug = "new-worktree"
	}
	return filepath.Join(repoPath, ".worktrees", slug)
}

func slugifyWorktreeBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.ReplaceAll(branch, "/", "-")
	branch = worktreeSlugRE.ReplaceAllString(branch, "-")
	branch = strings.Trim(branch, "-._")
	return branch
}

func applyCreateInputStyles(inputs []textinput.Model) {
	for i := range inputs {
		inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)
		inputs[i].TextStyle = lipgloss.NewStyle().Foreground(appstyles.Text)
		inputs[i].PlaceholderStyle = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	}
}
