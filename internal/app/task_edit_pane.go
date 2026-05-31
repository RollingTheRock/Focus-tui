package app

import (
	"errors"
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	"charm.land/huh/v2"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	taskEditHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)
	taskEditHintStyle   = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	taskEditErrorStyle  = lipgloss.NewStyle().Foreground(appstyles.Warning)
)

type OpenTaskEditorMsg struct {
	WorktreeID string
}

type TaskEditorSavedMsg struct {
	ID           models.PaneID
	TaskID       string
	WorktreeID   string
	Title        string
	Goal         string
	WhyNow       string
	Success      string
	OutOfScope   string
	KnownRisks   string
	NextStep     string
	State        string
	Priority     string
	RelationType string
	ParentTaskID string
}

type CloseTaskEditorMsg struct {
	ID models.PaneID
}

type taskEditorSeed struct {
	TaskID       string
	WorktreeID   string
	Title        string
	Goal         string
	WhyNow       string
	Success      string
	OutOfScope   string
	KnownRisks   string
	NextStep     string
	State        string
	Priority     string
	RelationType string
	ParentTaskID string
}

type TaskEditPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	worktreeID   string
	taskID       string
	relationType string
	parentTaskID string
	editForm     *huh.Form
	width        int
	height       int
	err          error
	saving       bool
}

func NewTaskEditPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, seed taskEditorSeed) *TaskEditPane {
	p := &TaskEditPane{
		id:           id,
		meta:         meta,
		common:       common,
		worktreeID:   seed.WorktreeID,
		taskID:       seed.TaskID,
		relationType: seed.RelationType,
		parentTaskID: seed.ParentTaskID,
	}

	// Use Huh's embedded accessor so values are updated in real-time.
	var values struct {
		title, goal, whyNow, success, outOfScope, knownRisks, nextStep, state, priority string
	}
	values.title = seed.Title
	values.goal = seed.Goal
	values.whyNow = seed.WhyNow
	values.success = seed.Success
	values.outOfScope = seed.OutOfScope
	values.knownRisks = seed.KnownRisks
	values.nextStep = seed.NextStep
	values.state = seed.State
	values.priority = seed.Priority

	// Disable Huh's default ctrl+c quit so the app handles it.
	km := huh.NewDefaultKeyMap()
	km.Quit.SetEnabled(false)

	p.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("title").
				Title("Title").
				Placeholder("修复用户登录接口缓存问题").
				Value(&values.title).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Key("goal").
				Title("Goal").
				Placeholder("这个任务应该产出什么结果？").
				Value(&values.goal),
			huh.NewInput().
				Key("whyNow").
				Title("Why now").
				Placeholder("为什么值得在编码前先做这件事？").
				Value(&values.whyNow),
			huh.NewInput().
				Key("success").
				Title("Success").
				Placeholder("什么结果能证明这件事做成功了？").
				Value(&values.success),
			huh.NewInput().
				Key("outOfScope").
				Title("Out of scope").
				Placeholder("明确排除不做的事情有哪些？").
				Value(&values.outOfScope),
			huh.NewInput().
				Key("knownRisks").
				Title("Known risks").
				Placeholder("后续可能出现什么风险？").
				Value(&values.knownRisks),
			huh.NewInput().
				Key("nextStep").
				Title("Next").
				Placeholder("在概览中渲染摘要").
				Value(&values.nextStep),
			huh.NewInput().
				Key("state").
				Title("State").
				Placeholder("active").
				Value(&values.state),
			huh.NewInput().
				Key("priority").
				Title("Priority").
				Placeholder("medium").
				Value(&values.priority),
		),
	).WithKeyMap(km)

	return p
}

func (p *TaskEditPane) Init() tea.Cmd {
	return p.editForm.Init()
}

func (p *TaskEditPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "esc":
			if p.saving {
				return p, nil
			}
			return p, closeTaskEditorCmd(p.id)
		case "ctrl+s":
			return p.submit()
		}
	}

	if p.editForm == nil {
		return p, nil
	}

	m, cmd := p.editForm.Update(msg)
	if f, ok := m.(*huh.Form); ok {
		p.editForm = f
	}
	if p.editForm.State == huh.StateCompleted {
		return p.submit()
	}
	return p, cmd
}

func (p *TaskEditPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 72
	}
	lines := []string{
		taskEditHeaderStyle.Render("Task Context"),
		taskEditHintStyle.Render("Define the task, its brief constraints, and the next step for this worktree."),
		"",
	}
	if p.editForm != nil {
		lines = append(lines, p.editForm.View())
	}
	lines = append(lines, "")
	lines = append(lines, taskEditHintStyle.Render("Tab move · Enter next/save · Ctrl+S save · Esc cancel"))
	if p.saving {
		lines = append(lines, taskEditHintStyle.Render("Saving task context..."))
	}
	if p.err != nil {
		lines = append(lines, taskEditErrorStyle.Render("Save failed: "+p.err.Error()))
	}
	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	for i := range lines {
		lines[i] = appstyles.StyleCache.MaxWidth(width).Render(lines[i])
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (p *TaskEditPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	if p.editForm != nil {
		p.editForm.WithWidth(width)
	}
}

func (p *TaskEditPane) submit() (models.Panel, tea.Cmd) {
	if p.saving {
		return p, nil
	}
	if p.editForm == nil {
		return p, nil
	}

	title := strings.TrimSpace(p.editForm.GetString("title"))
	if title == "" {
		p.err = errors.New("task title cannot be empty")
		return p, nil
	}
	state := strings.TrimSpace(p.editForm.GetString("state"))
	if state == "" {
		state = "active"
	}
	priority := strings.TrimSpace(p.editForm.GetString("priority"))
	if priority == "" {
		priority = "medium"
	}
	p.saving = true
	p.err = nil
	return p, func() tea.Msg {
		return TaskEditorSavedMsg{
			ID:           p.id,
			TaskID:       p.taskID,
			WorktreeID:   p.worktreeID,
			Title:        title,
			Goal:         strings.TrimSpace(p.editForm.GetString("goal")),
			WhyNow:       strings.TrimSpace(p.editForm.GetString("whyNow")),
			Success:      strings.TrimSpace(p.editForm.GetString("success")),
			OutOfScope:   strings.TrimSpace(p.editForm.GetString("outOfScope")),
			KnownRisks:   strings.TrimSpace(p.editForm.GetString("knownRisks")),
			NextStep:     strings.TrimSpace(p.editForm.GetString("nextStep")),
			State:        state,
			Priority:     priority,
			RelationType: p.relationType,
			ParentTaskID: p.parentTaskID,
		}
	}
}

func closeTaskEditorCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return CloseTaskEditorMsg{ID: id} }
}
