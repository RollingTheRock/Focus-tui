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

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

type OpenCreateWorktreeMsg struct {
	RepoPath  string
	BaseRef   string
	TaskTitle string
	TaskID    string
	IsPhase   bool
}

type WorktreeCreatedMsg struct {
	ID           models.PaneID
	Worktree     gitmodel.Worktree
	OpenExternal bool
	TaskTitle    string
	TaskID       string
	IsPhase      bool
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

	repoPath   string
	width      int
	height     int
	editForm   *huh.Form
	pathInput  *huh.Input
	formValues struct {
		task, branch, baseRef, path string
	}
	err          error
	creating     bool
	openExternal bool
	taskName     string
	taskID       string
	isPhase      bool
	lastAutoPath string
}

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

	p := &WorktreeCreatePane{
		id:           id,
		meta:         meta,
		common:       common,
		adapter:      adapter,
		repoPath:     repoPath,
		openExternal: true,
		taskID:       msg.TaskID,
		isPhase:      msg.IsPhase,
	}
	p.formValues.task = msg.TaskTitle
	p.formValues.branch = ""
	p.formValues.baseRef = baseRef
	p.formValues.path = defaultWorktreePath(repoPath, "")
	p.lastAutoPath = p.formValues.path

	km := huh.NewDefaultKeyMap()
	km.Quit.SetEnabled(false)

	p.pathInput = huh.NewInput().
		Key("path").
		Title("Path").
		Placeholder(defaultWorktreePath(repoPath, "")).
		Value(&p.formValues.path).
		Validate(huh.ValidateNotEmpty())

	p.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("task").
				Title("Task (optional)").
				Placeholder("What are you working on?").
				Value(&p.formValues.task),
			huh.NewInput().
				Key("branch").
				Title("Branch").
				Placeholder("feature/worktree-pane").
				Value(&p.formValues.branch).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Key("baseRef").
				Title("Base ref").
				Placeholder("HEAD").
				Value(&p.formValues.baseRef),
			p.pathInput,
		),
	).WithKeyMap(km)

	return p
}

func (p *WorktreeCreatePane) Init() tea.Cmd {
	return p.editForm.Init()
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
			return WorktreeCreatedMsg{ID: p.id, Worktree: *msg.worktree, OpenExternal: p.openExternal, TaskTitle: p.taskName, TaskID: p.taskID, IsPhase: p.isPhase}
		}
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "esc":
			if p.creating {
				return p, nil
			}
			return p, closeCreateWorktreeCmd(p.id)
		case "ctrl+s":
			return p.submit()
		case "ctrl+t":
			p.openExternal = !p.openExternal
			return p, nil
		}
	}

	if p.editForm == nil {
		return p, nil
	}

	m, cmd := p.editForm.Update(msg)
	if f, ok := m.(*huh.Form); ok {
		p.editForm = f
	}
	p.syncAutoPathWithBranch()
	if p.editForm.State == huh.StateCompleted {
		return p.submit()
	}
	return p, cmd
}

func (p *WorktreeCreatePane) View() tea.View {
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
	}
	if p.editForm != nil {
		lines = append(lines, p.editForm.View())
	}
	lines = append(lines, "")
	lines = append(lines, commitHintStyle.Render("Tab move · Enter next/submit · Ctrl+S create · Ctrl+T "+modeHint+" · Esc cancel"))
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
	return tea.NewView(strings.Join(lines, "\n"))
}

func (p *WorktreeCreatePane) SetSize(width, height int) {
	p.width = width
	p.height = height
	if p.editForm != nil {
		p.editForm.WithWidth(width)
	}
}

func (p *WorktreeCreatePane) submit() (models.Panel, tea.Cmd) {
	if p.creating {
		return p, nil
	}
	p.taskName = strings.TrimSpace(p.formValues.task)
	branch := strings.TrimSpace(p.formValues.branch)
	baseRef := strings.TrimSpace(p.formValues.baseRef)
	path := strings.TrimSpace(p.formValues.path)
	if branch == "" {
		p.err = errors.New("branch name cannot be empty")
		return p, nil
	}
	if path == "" {
		path = defaultWorktreePath(p.repoPath, branch)
	} else if path == defaultWorktreePath(p.repoPath, "") {
		path = defaultWorktreePath(p.repoPath, branch)
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

func (p *WorktreeCreatePane) syncAutoPathWithBranch() {
	if p.lastAutoPath == "" {
		p.lastAutoPath = defaultWorktreePath(p.repoPath, "")
	}
	if strings.TrimSpace(p.formValues.path) != p.lastAutoPath {
		return
	}
	nextAutoPath := defaultWorktreePath(p.repoPath, p.formValues.branch)
	p.formValues.path = nextAutoPath
	p.lastAutoPath = nextAutoPath
	if p.pathInput != nil {
		p.pathInput.Value(&p.formValues.path)
	}
	if p.editForm != nil {
		width := p.width
		if width <= 0 {
			width = 72
		}
		height := p.height
		if height <= 0 {
			height = 24
		}
		if m, _ := p.editForm.Update(tea.WindowSizeMsg{Width: width, Height: height}); m != nil {
			if f, ok := m.(*huh.Form); ok {
				p.editForm = f
			}
		}
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
