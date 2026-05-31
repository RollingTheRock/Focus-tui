package app

import (
	"errors"
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	"charm.land/huh/v2"
	tea "charm.land/bubbletea/v2"
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

	planID      string
	taskID      string
	worktreeID  string
	editForm    *huh.Form
	width       int
	height      int
	err         error
	saving      bool
	status      string
	currentStep string
}

func NewPlanEditPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, seed planEditorSeed) *PlanEditPane {
	p := &PlanEditPane{
		id:          id,
		meta:        meta,
		common:      common,
		planID:      seed.PlanID,
		taskID:      seed.TaskID,
		worktreeID:  seed.WorktreeID,
		status:      seed.Status,
		currentStep: seed.CurrentStep,
	}

	var values struct {
		title, whyNow, success, outOfScope, knownRisks, body string
	}
	values.title = seed.Title
	values.whyNow = seed.WhyNow
	values.success = seed.Success
	values.outOfScope = seed.OutOfScope
	values.knownRisks = seed.KnownRisks
	values.body = seed.PlanBody

	km := huh.NewDefaultKeyMap()
	km.Quit.SetEnabled(false)

	p.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("title").
				Title("Plan").
				Placeholder("重构认证模块 v2").
				Value(&values.title).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Key("whyNow").
				Title("Why now").
				Placeholder("为什么现在值得执行这个计划？").
				Value(&values.whyNow),
			huh.NewInput().
				Key("success").
				Title("Success").
				Placeholder("什么能证明计划成功了？").
				Value(&values.success),
			huh.NewInput().
				Key("outOfScope").
				Title("Out of scope").
				Placeholder("明确排除不做的事情有哪些？").
				Value(&values.outOfScope),
			huh.NewInput().
				Key("knownRisks").
				Title("Risks").
				Placeholder("还可能出现什么问题？").
				Value(&values.knownRisks),
			huh.NewText().
				Key("body").
				Title("Body").
				Placeholder("阶段 A\n验证通道\n风险通道").
				Value(&values.body).
				Lines(8),
		),
	).WithKeyMap(km)

	return p
}

func (p *PlanEditPane) Init() tea.Cmd {
	return p.editForm.Init()
}

func (p *PlanEditPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "esc":
			if p.saving {
				return p, nil
			}
			return p, closePlanEditorCmd(p.id)
		case "ctrl+s":
			return p.submit()
		case "ctrl+a":
			if p.status == "approved" {
				p.status = "draft"
			} else {
				p.status = "approved"
			}
			return p, nil
		case "ctrl+t":
			return p.submitExpanded()
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

func (p *PlanEditPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 72
	}
	lines := []string{
		taskEditHeaderStyle.Render("Plan Draft"),
		taskEditHintStyle.Render("Create the plan first, then break it into tasks and sessions once the structure is right. Prefix lines with blocked:, follow-up:, worktree:, validation:, or risk: to converge the plan before expansion."),
		"",
	}
	if p.editForm != nil {
		lines = append(lines, p.editForm.View())
	}
	lines = append(lines, "")
	lines = append(lines, taskEditHintStyle.Render("Tab switch fields · Enter move into body · Ctrl+S save · Ctrl+A approve/draft · Ctrl+T save+expand · Esc cancel"))
	lines = append(lines, taskEditHintStyle.Render("status: "+p.status))
	if p.saving {
		lines = append(lines, taskEditHintStyle.Render("Saving plan draft..."))
	}
	if p.err != nil {
		lines = append(lines, taskEditErrorStyle.Render("Save failed: "+p.err.Error()))
	}
	for i := range lines {
		lines[i] = appstyles.StyleCache.MaxWidth(width).Render(lines[i])
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (p *PlanEditPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	if p.editForm != nil {
		p.editForm.WithWidth(width)
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
	if p.editForm == nil {
		return PlanEditorSavedMsg{}, errors.New("edit form is nil")
	}
	title := strings.TrimSpace(p.editForm.GetString("title"))
	if title == "" {
		return PlanEditorSavedMsg{}, errors.New("plan title cannot be empty")
	}
	if expand || p.status == "approved" {
		if strings.TrimSpace(p.editForm.GetString("whyNow")) == "" {
			return PlanEditorSavedMsg{}, errors.New("approved plans require a why-now brief")
		}
		if strings.TrimSpace(p.editForm.GetString("success")) == "" {
			return PlanEditorSavedMsg{}, errors.New("approved plans require success criteria")
		}
		if strings.TrimSpace(p.editForm.GetString("outOfScope")) == "" {
			return PlanEditorSavedMsg{}, errors.New("approved plans require an out-of-scope boundary")
		}
		if len(strings.TrimSpace(p.editForm.GetString("body"))) == 0 {
			return PlanEditorSavedMsg{}, errors.New("approved plans require at least one decomposition step")
		}
	}
	return PlanEditorSavedMsg{
		ID:            p.id,
		PlanID:        p.planID,
		TaskID:        p.taskID,
		WorktreeID:    p.worktreeID,
		Title:         title,
		WhyNow:        strings.TrimSpace(p.editForm.GetString("whyNow")),
		Success:       strings.TrimSpace(p.editForm.GetString("success")),
		OutOfScope:    strings.TrimSpace(p.editForm.GetString("outOfScope")),
		KnownRisks:    strings.TrimSpace(p.editForm.GetString("knownRisks")),
		PlanBody:      strings.TrimSpace(p.editForm.GetString("body")),
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
