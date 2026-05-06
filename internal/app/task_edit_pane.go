package app

import (
	"errors"
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	inputs       []textinput.Model
	focus        int
	width        int
	height       int
	err          error
	saving       bool
}

const (
	taskEditFieldTitle = iota
	taskEditFieldGoal
	taskEditFieldWhyNow
	taskEditFieldSuccess
	taskEditFieldOutOfScope
	taskEditFieldKnownRisks
	taskEditFieldNextStep
	taskEditFieldState
	taskEditFieldPriority
)

func NewTaskEditPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, seed taskEditorSeed) *TaskEditPane {
	titleInput := textinput.New()
	titleInput.Prompt = "Title: "
	titleInput.Placeholder = "修复用户登录接口缓存问题"
	titleInput.SetValue(seed.Title)

	goalInput := textinput.New()
	goalInput.Prompt = "Goal: "
	goalInput.Placeholder = "这个任务应该产出什么结果？"
	goalInput.SetValue(seed.Goal)

	whyNowInput := textinput.New()
	whyNowInput.Prompt = "Why now: "
	whyNowInput.Placeholder = "为什么值得在编码前先做这件事？"
	whyNowInput.SetValue(seed.WhyNow)

	successInput := textinput.New()
	successInput.Prompt = "Success: "
	successInput.Placeholder = "什么结果能证明这件事做成功了？"
	successInput.SetValue(seed.Success)

	outOfScopeInput := textinput.New()
	outOfScopeInput.Prompt = "Out of scope: "
	outOfScopeInput.Placeholder = "明确排除不做的事情有哪些？"
	outOfScopeInput.SetValue(seed.OutOfScope)

	knownRisksInput := textinput.New()
	knownRisksInput.Prompt = "Known risks: "
	knownRisksInput.Placeholder = "后续可能出现什么风险？"
	knownRisksInput.SetValue(seed.KnownRisks)

	nextStepInput := textinput.New()
	nextStepInput.Prompt = "Next: "
	nextStepInput.Placeholder = "在概览中渲染摘要"
	nextStepInput.SetValue(seed.NextStep)

	stateInput := textinput.New()
	stateInput.Prompt = "State: "
	stateInput.Placeholder = "active"
	stateInput.SetValue(seed.State)

	priorityInput := textinput.New()
	priorityInput.Prompt = "Priority: "
	priorityInput.Placeholder = "medium"
	priorityInput.SetValue(seed.Priority)

	inputs := []textinput.Model{titleInput, goalInput, whyNowInput, successInput, outOfScopeInput, knownRisksInput, nextStepInput, stateInput, priorityInput}
	for i := range inputs {
		inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)
		inputs[i].TextStyle = lipgloss.NewStyle().Foreground(appstyles.Text)
		inputs[i].PlaceholderStyle = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	}
	inputs[0].Focus()

	return &TaskEditPane{
		id:           id,
		meta:         meta,
		common:       common,
		worktreeID:   seed.WorktreeID,
		taskID:       seed.TaskID,
		relationType: seed.RelationType,
		parentTaskID: seed.ParentTaskID,
		inputs:       inputs,
	}
}

func (p *TaskEditPane) Init() tea.Cmd {
	return textinput.Blink
}

func (p *TaskEditPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if p.saving {
				return p, nil
			}
			return p, closeTaskEditorCmd(p.id)
		case "tab":
			p.moveFocus(1)
			return p, nil
		case "shift+tab":
			p.moveFocus(-1)
			return p, nil
		case "enter":
			if p.focus == taskEditFieldState {
				p.moveFocus(1)
				return p, nil
			}
			if p.focus == taskEditFieldPriority {
				return p.submit()
			}
			p.moveFocus(1)
			return p, nil
		case "ctrl+s":
			return p.submit()
		}
	}

	var cmd tea.Cmd
	for i := range p.inputs {
		p.inputs[i], cmd = p.inputs[i].Update(msg)
	}
	return p, cmd
}

func (p *TaskEditPane) View() string {
	width := p.width
	if width <= 0 {
		width = 72
	}
	lines := []string{
		taskEditHeaderStyle.Render("Task Context"),
		taskEditHintStyle.Render("Define the task, its brief constraints, and the next step for this worktree."),
		"",
		p.inputs[taskEditFieldTitle].View(),
		p.inputs[taskEditFieldGoal].View(),
		p.inputs[taskEditFieldWhyNow].View(),
		p.inputs[taskEditFieldSuccess].View(),
		p.inputs[taskEditFieldOutOfScope].View(),
		p.inputs[taskEditFieldKnownRisks].View(),
		p.inputs[taskEditFieldNextStep].View(),
		p.inputs[taskEditFieldState].View(),
		p.inputs[taskEditFieldPriority].View(),
		"",
		taskEditHintStyle.Render("Tab move · Enter next/save · Ctrl+S save · Esc cancel"),
	}
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
	return strings.Join(lines, "\n")
}

func (p *TaskEditPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	inputWidth := width - 8
	if inputWidth < 24 {
		inputWidth = 24
	}
	for i := range p.inputs {
		p.inputs[i].Width = inputWidth
	}
}

func (p *TaskEditPane) moveFocus(delta int) {
	p.focus = (p.focus + delta + len(p.inputs)) % len(p.inputs)
	for i := range p.inputs {
		if i == p.focus {
			p.inputs[i].Focus()
		} else {
			p.inputs[i].Blur()
		}
	}
}

func (p *TaskEditPane) submit() (models.Panel, tea.Cmd) {
	if p.saving {
		return p, nil
	}
	title := strings.TrimSpace(p.inputs[taskEditFieldTitle].Value())
	if title == "" {
		p.err = errors.New("task title cannot be empty")
		return p, nil
	}
	state := strings.TrimSpace(p.inputs[taskEditFieldState].Value())
	if state == "" {
		state = "active"
	}
	priority := strings.TrimSpace(p.inputs[taskEditFieldPriority].Value())
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
			Goal:         strings.TrimSpace(p.inputs[taskEditFieldGoal].Value()),
			WhyNow:       strings.TrimSpace(p.inputs[taskEditFieldWhyNow].Value()),
			Success:      strings.TrimSpace(p.inputs[taskEditFieldSuccess].Value()),
			OutOfScope:   strings.TrimSpace(p.inputs[taskEditFieldOutOfScope].Value()),
			KnownRisks:   strings.TrimSpace(p.inputs[taskEditFieldKnownRisks].Value()),
			NextStep:     strings.TrimSpace(p.inputs[taskEditFieldNextStep].Value()),
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
