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
	ID            models.PaneID
	PlanID        string
	TaskID        string
	WorktreeID    string
	Title         string
	WhyNow        string
	Success       string
	OutOfScope    string
	KnownRisks    string
	PlanBody      string
	Status        string
	CurrentStep   string
	ExpandToTasks bool
}

type planEditorSeed struct {
	PlanID      string
	TaskID      string
	WorktreeID  string
	Title       string
	WhyNow      string
	Success     string
	OutOfScope  string
	KnownRisks  string
	PlanBody    string
	Status      string
	CurrentStep string
}

type PlanEditPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	planID          string
	taskID          string
	worktreeID      string
	titleInput      textinput.Model
	whyNowInput     textinput.Model
	successInput    textinput.Model
	outOfScopeInput textinput.Model
	knownRisksInput textinput.Model
	bodyInput       textarea.Model
	focus           int
	width           int
	height          int
	err             error
	saving          bool
	status          string
	currentStep     string
}

const (
	planEditFieldTitle = iota
	planEditFieldWhyNow
	planEditFieldSuccess
	planEditFieldOutOfScope
	planEditFieldKnownRisks
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

	whyNowInput := textinput.New()
	whyNowInput.Prompt = "Why now: "
	whyNowInput.Placeholder = "Why is this plan worth doing now?"
	whyNowInput.SetValue(seed.WhyNow)
	whyNowInput.PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)

	successInput := textinput.New()
	successInput.Prompt = "Success: "
	successInput.Placeholder = "What proves the plan succeeded?"
	successInput.SetValue(seed.Success)
	successInput.PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)

	outOfScopeInput := textinput.New()
	outOfScopeInput.Prompt = "Out of scope: "
	outOfScopeInput.Placeholder = "What are we explicitly not doing?"
	outOfScopeInput.SetValue(seed.OutOfScope)
	outOfScopeInput.PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)

	knownRisksInput := textinput.New()
	knownRisksInput.Prompt = "Risks: "
	knownRisksInput.Placeholder = "What can still go wrong?"
	knownRisksInput.SetValue(seed.KnownRisks)
	knownRisksInput.PromptStyle = lipgloss.NewStyle().Foreground(appstyles.Accent)

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
		id:              id,
		meta:            meta,
		common:          common,
		planID:          seed.PlanID,
		taskID:          seed.TaskID,
		worktreeID:      seed.WorktreeID,
		titleInput:      titleInput,
		whyNowInput:     whyNowInput,
		successInput:    successInput,
		outOfScopeInput: outOfScopeInput,
		knownRisksInput: knownRisksInput,
		bodyInput:       bodyInput,
		status:          seed.Status,
		currentStep:     seed.CurrentStep,
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
		case tea.KeyCtrlA:
			if p.status == "approved" {
				p.status = "draft"
			} else {
				p.status = "approved"
			}
			return p, nil
		case tea.KeyCtrlT:
			return p.submitExpanded()
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
	switch p.focus {
	case planEditFieldTitle:
		p.titleInput, cmd = p.titleInput.Update(msg)
	case planEditFieldWhyNow:
		p.whyNowInput, cmd = p.whyNowInput.Update(msg)
	case planEditFieldSuccess:
		p.successInput, cmd = p.successInput.Update(msg)
	case planEditFieldOutOfScope:
		p.outOfScopeInput, cmd = p.outOfScopeInput.Update(msg)
	case planEditFieldKnownRisks:
		p.knownRisksInput, cmd = p.knownRisksInput.Update(msg)
	default:
		p.bodyInput, cmd = p.bodyInput.Update(msg)
	}
	return p, cmd
}

func (p *PlanEditPane) View() string {
	width := p.width
	if width <= 0 {
		width = 72
	}
	lines := []string{
		taskEditHeaderStyle.Render("Plan Draft"),
		taskEditHintStyle.Render("Create the plan first, then break it into tasks and sessions once the structure is right. Prefix lines with blocked:, follow-up:, worktree:, validation:, or risk: to converge the plan before expansion."),
		"",
		p.titleInput.View(),
		p.whyNowInput.View(),
		p.successInput.View(),
		p.outOfScopeInput.View(),
		p.knownRisksInput.View(),
		"",
		p.bodyInput.View(),
		"",
		taskEditHintStyle.Render("Tab switch fields · Enter move into body · Ctrl+S save · Ctrl+A approve/draft · Ctrl+T save+expand · Esc cancel"),
		taskEditHintStyle.Render("status: " + p.status),
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
	p.whyNowInput.Width = inputWidth
	p.successInput.Width = inputWidth
	p.outOfScopeInput.Width = inputWidth
	p.knownRisksInput.Width = inputWidth
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
	p.focus = (p.focus + delta + 5) % 5
	if p.focus == planEditFieldTitle {
		p.titleInput.Focus()
		p.whyNowInput.Blur()
		p.successInput.Blur()
		p.outOfScopeInput.Blur()
		p.knownRisksInput.Blur()
		p.bodyInput.Blur()
	} else if p.focus == planEditFieldWhyNow {
		p.titleInput.Blur()
		p.whyNowInput.Focus()
		p.successInput.Blur()
		p.outOfScopeInput.Blur()
		p.knownRisksInput.Blur()
		p.bodyInput.Blur()
	} else if p.focus == planEditFieldSuccess {
		p.titleInput.Blur()
		p.whyNowInput.Blur()
		p.successInput.Focus()
		p.outOfScopeInput.Blur()
		p.knownRisksInput.Blur()
		p.bodyInput.Blur()
	} else if p.focus == planEditFieldOutOfScope {
		p.titleInput.Blur()
		p.whyNowInput.Blur()
		p.successInput.Blur()
		p.outOfScopeInput.Focus()
		p.knownRisksInput.Blur()
		p.bodyInput.Blur()
	} else if p.focus == planEditFieldKnownRisks {
		p.titleInput.Blur()
		p.whyNowInput.Blur()
		p.successInput.Blur()
		p.outOfScopeInput.Blur()
		p.knownRisksInput.Focus()
		p.bodyInput.Blur()
	} else {
		p.titleInput.Blur()
		p.whyNowInput.Blur()
		p.successInput.Blur()
		p.outOfScopeInput.Blur()
		p.knownRisksInput.Blur()
		_ = p.bodyInput.Focus()
	}
}

func (p *PlanEditPane) submit() (models.Panel, tea.Cmd) {
	msg, err := p.buildSavedMsg(false)
	if err != nil {
		p.err = err
		return p, nil
	}
	p.saving = true
	p.err = nil
	return p, func() tea.Msg { return msg }
}

func (p *PlanEditPane) buildSavedMsg(expand bool) (PlanEditorSavedMsg, error) {
	if p.saving {
		return PlanEditorSavedMsg{}, errors.New("plan is already saving")
	}
	title := strings.TrimSpace(p.titleInput.Value())
	if title == "" {
		return PlanEditorSavedMsg{}, errors.New("plan title cannot be empty")
	}
	if expand || p.status == "approved" {
		if strings.TrimSpace(p.whyNowInput.Value()) == "" {
			return PlanEditorSavedMsg{}, errors.New("approved plans require a why-now brief")
		}
		if strings.TrimSpace(p.successInput.Value()) == "" {
			return PlanEditorSavedMsg{}, errors.New("approved plans require success criteria")
		}
		if strings.TrimSpace(p.outOfScopeInput.Value()) == "" {
			return PlanEditorSavedMsg{}, errors.New("approved plans require an out-of-scope boundary")
		}
		if len(strings.TrimSpace(p.bodyInput.Value())) == 0 {
			return PlanEditorSavedMsg{}, errors.New("approved plans require at least one decomposition step")
		}
	}
	return PlanEditorSavedMsg{
		ID:            p.id,
		PlanID:        p.planID,
		TaskID:        p.taskID,
		WorktreeID:    p.worktreeID,
		Title:         title,
		WhyNow:        strings.TrimSpace(p.whyNowInput.Value()),
		Success:       strings.TrimSpace(p.successInput.Value()),
		OutOfScope:    strings.TrimSpace(p.outOfScopeInput.Value()),
		KnownRisks:    strings.TrimSpace(p.knownRisksInput.Value()),
		PlanBody:      strings.TrimSpace(p.bodyInput.Value()),
		Status:        p.status,
		CurrentStep:   p.currentStep,
		ExpandToTasks: expand,
	}, nil
}

func (p *PlanEditPane) submitExpanded() (models.Panel, tea.Cmd) {
	p.status = "approved"
	msg, err := p.buildSavedMsg(true)
	if err != nil {
		p.err = err
		return p, nil
	}
	p.saving = true
	p.err = nil
	return p, func() tea.Msg { return msg }
}

func closePlanEditorCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return ClosePlanEditorMsg{ID: id} }
}
