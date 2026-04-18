package app

import (
	"errors"
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ClosePlanEditorMsg struct {
	ID models.PaneID
}

type PlanEditorSavedMsg struct {
	ID          models.PaneID
	PlanID      string
	TaskID      string
	WorktreeID  string
	Title       string
	PlanBody    string
	Status      string
	CurrentStep string
}

type planEditorSeed struct {
	PlanID      string
	TaskID      string
	WorktreeID  string
	Title       string
	PlanBody    string
	Status      string
	CurrentStep string
}

type PlanEditPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	planID      string
	taskID      string
	worktreeID  string
	titleInput  textinput.Model
	bodyInput   textarea.Model
	focus       int
	width       int
	height      int
	err         error
	saving      bool
	status      string
	currentStep string
}

const (
	planEditFieldTitle = iota
	planEditFieldBody
)

func NewPlanEditPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, seed planEditorSeed) *PlanEditPane {
	titleInput := textinput.New()
	titleInput.Prompt = "Plan: "
	titleInput.Placeholder = "Overview recovery rollout"
	titleInput.SetValue(seed.Title)
	titleInput.PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)
	titleInput.TextStyle = lipgloss.NewStyle().Foreground(appstyles.Text)
	titleInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	titleInput.Focus()

	bodyInput := textarea.New()
	bodyInput.Placeholder = "Phase A\nValidation lane\nRisk lane"
	bodyInput.Prompt = "│ "
	bodyInput.ShowLineNumbers = false
	bodyInput.SetValue(seed.PlanBody)
	focusedStyle, blurredStyle := textarea.DefaultStyles()
	focusedStyle.Base = lipgloss.NewStyle().Foreground(appstyles.Text)
	focusedStyle.CursorLine = lipgloss.NewStyle()
	focusedStyle.Prompt = lipgloss.NewStyle().Foreground(appstyles.Accent)
	focusedStyle.Placeholder = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	focusedStyle.Text = lipgloss.NewStyle().Foreground(appstyles.Text)
	blurredStyle = focusedStyle
	blurredStyle.Prompt = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	bodyInput.FocusedStyle = focusedStyle
	bodyInput.BlurredStyle = blurredStyle
	bodyInput.Blur()
	bodyInput.SetWidth(56)
	bodyInput.SetHeight(8)

	return &PlanEditPane{
		id:          id,
		meta:        meta,
		common:      common,
		planID:      seed.PlanID,
		taskID:      seed.TaskID,
		worktreeID:  seed.WorktreeID,
		titleInput:  titleInput,
		bodyInput:   bodyInput,
		status:      seed.Status,
		currentStep: seed.CurrentStep,
	}
}

func (p *PlanEditPane) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, textarea.Blink)
}

func (p *PlanEditPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if p.saving {
				return p, nil
			}
			return p, closePlanEditorCmd(p.id)
		case tea.KeyCtrlS:
			return p.submit()
		case tea.KeyTab:
			p.moveFocus(1)
			return p, nil
		case tea.KeyShiftTab:
			p.moveFocus(-1)
			return p, nil
		case tea.KeyEnter:
			if p.focus == planEditFieldTitle {
				p.moveFocus(1)
				return p, nil
			}
		}
	}

	var cmd tea.Cmd
	if p.focus == planEditFieldTitle {
		p.titleInput, cmd = p.titleInput.Update(msg)
		return p, cmd
	}
	p.bodyInput, cmd = p.bodyInput.Update(msg)
	return p, cmd
}

func (p *PlanEditPane) View() string {
	width := p.width
	if width <= 0 {
		width = 72
	}
	lines := []string{
		taskEditHeaderStyle.Render("Plan Draft"),
		taskEditHintStyle.Render("Create the plan first, then break it into tasks and sessions once the structure is right. Each non-empty line becomes a decomposition step."),
		"",
		p.titleInput.View(),
		"",
		p.bodyInput.View(),
		"",
		taskEditHintStyle.Render("Tab switch fields · Enter move into body · Ctrl+S save · Esc cancel"),
	}
	if p.saving {
		lines = append(lines, taskEditHintStyle.Render("Saving plan draft..."))
	}
	if p.err != nil {
		lines = append(lines, taskEditErrorStyle.Render("Save failed: "+p.err.Error()))
	}
	for i := range lines {
		lines[i] = appstyles.StyleCache.MaxWidth(width).Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (p *PlanEditPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	inputWidth := width - 8
	if inputWidth < 24 {
		inputWidth = 24
	}
	p.titleInput.Width = inputWidth
	p.bodyInput.SetWidth(inputWidth)
	bodyHeight := 8
	if height > 0 {
		bodyHeight = height / 2
		if bodyHeight < 5 {
			bodyHeight = 5
		}
		if bodyHeight > 12 {
			bodyHeight = 12
		}
	}
	p.bodyInput.SetHeight(bodyHeight)
}

func (p *PlanEditPane) moveFocus(delta int) {
	p.focus = (p.focus + delta + 2) % 2
	if p.focus == planEditFieldTitle {
		p.titleInput.Focus()
		p.bodyInput.Blur()
	} else {
		p.titleInput.Blur()
		_ = p.bodyInput.Focus()
	}
}

func (p *PlanEditPane) submit() (models.Panel, tea.Cmd) {
	if p.saving {
		return p, nil
	}
	title := strings.TrimSpace(p.titleInput.Value())
	if title == "" {
		p.err = errors.New("plan title cannot be empty")
		return p, nil
	}
	p.saving = true
	p.err = nil
	return p, func() tea.Msg {
		return PlanEditorSavedMsg{
			ID:          p.id,
			PlanID:      p.planID,
			TaskID:      p.taskID,
			WorktreeID:  p.worktreeID,
			Title:       title,
			PlanBody:    strings.TrimSpace(p.bodyInput.Value()),
			Status:      p.status,
			CurrentStep: p.currentStep,
		}
	}
}

func closePlanEditorCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return ClosePlanEditorMsg{ID: id} }
}
