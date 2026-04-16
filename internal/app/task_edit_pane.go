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
	ID         models.PaneID
	WorktreeID string
	Title      string
	Goal       string
	NextStep   string
	State      string
}

type CloseTaskEditorMsg struct {
	ID models.PaneID
}

type taskEditorSeed struct {
	WorktreeID string
	Title      string
	Goal       string
	NextStep   string
	State      string
}

type TaskEditPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	worktreeID string
	inputs     []textinput.Model
	focus      int
	width      int
	height     int
	err        error
	saving     bool
}

const (
	taskEditFieldTitle = iota
	taskEditFieldGoal
	taskEditFieldNextStep
	taskEditFieldState
)

func NewTaskEditPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, seed taskEditorSeed) *TaskEditPane {
	titleInput := textinput.New()
	titleInput.Prompt = "Title: "
	titleInput.Placeholder = "Tighten resume pipeline"
	titleInput.SetValue(seed.Title)

	goalInput := textinput.New()
	goalInput.Prompt = "Goal: "
	goalInput.Placeholder = "What outcome should this task produce?"
	goalInput.SetValue(seed.Goal)

	nextStepInput := textinput.New()
	nextStepInput.Prompt = "Next: "
	nextStepInput.Placeholder = "Render summary in overview"
	nextStepInput.SetValue(seed.NextStep)

	stateInput := textinput.New()
	stateInput.Prompt = "State: "
	stateInput.Placeholder = "active"
	stateInput.SetValue(seed.State)

	inputs := []textinput.Model{titleInput, goalInput, nextStepInput, stateInput}
	for i := range inputs {
		inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)
		inputs[i].TextStyle = lipgloss.NewStyle().Foreground(appstyles.Text)
		inputs[i].PlaceholderStyle = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	}
	inputs[0].Focus()

	return &TaskEditPane{
		id:         id,
		meta:       meta,
		common:     common,
		worktreeID: seed.WorktreeID,
		inputs:     inputs,
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
		taskEditHintStyle.Render("Define the task, its goal, and the next step for this worktree."),
		"",
		p.inputs[taskEditFieldTitle].View(),
		p.inputs[taskEditFieldGoal].View(),
		p.inputs[taskEditFieldNextStep].View(),
		p.inputs[taskEditFieldState].View(),
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
	p.saving = true
	p.err = nil
	return p, func() tea.Msg {
		return TaskEditorSavedMsg{
			ID:         p.id,
			WorktreeID: p.worktreeID,
			Title:      title,
			Goal:       strings.TrimSpace(p.inputs[taskEditFieldGoal].Value()),
			NextStep:   strings.TrimSpace(p.inputs[taskEditFieldNextStep].Value()),
			State:      state,
		}
	}
}

func closeTaskEditorCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return CloseTaskEditorMsg{ID: id} }
}
