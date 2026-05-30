package app

import (
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CloseAgentRegisterMsg closes the register overlay.
type CloseAgentRegisterMsg struct{}

// AgentRegisteredMsg signals a new custom agent was registered.
type AgentRegisteredMsg struct{}

type agentRegisterPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	common   models.CommonModel
	width    int
	height   int
	fields   []field
	cursor   int
	errMsg   string
}

type field struct {
	label string
	value string
}

func newAgentRegisterPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel) *agentRegisterPane {
	return &agentRegisterPane{
		id:     id,
		meta:   meta,
		common: common,
		fields: []field{
			{label: "Name", value: ""},
			{label: "Description", value: ""},
			{label: "Command", value: ""},
			{label: "Tags (comma-separated)", value: ""},
		},
	}
}

func (p *agentRegisterPane) Init() tea.Cmd { return nil }

func (p *agentRegisterPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return p, func() tea.Msg { return CloseAgentRegisterMsg{} }
		case tea.KeyTab:
			p.cursor = (p.cursor + 1) % len(p.fields)
		case tea.KeyEnter:
			if p.cursor == len(p.fields)-1 {
				return p, p.save()
			}
			p.cursor++
		case tea.KeyBackspace:
			if len(p.fields[p.cursor].value) > 0 {
				p.fields[p.cursor].value = p.fields[p.cursor].value[:len(p.fields[p.cursor].value)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				p.fields[p.cursor].value += string(msg.Runes)
			}
		}
	}
	return p, nil
}

func (p *agentRegisterPane) save() tea.Cmd {
	name := strings.TrimSpace(p.fields[0].value)
	desc := strings.TrimSpace(p.fields[1].value)
	binary := strings.TrimSpace(p.fields[2].value)
	tagsStr := strings.TrimSpace(p.fields[3].value)

	if name == "" || binary == "" {
		p.errMsg = "Name and Command are required."
		return nil
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			tags = append(tags, strings.TrimSpace(t))
		}
	}

	def := models.AgentDefinition{
		ID:          id,
		Name:        name,
		Description: desc,
		Binary:      binary,
		Tags:        tags,
		ProviderType: "generic",
	}

	if err := p.common.Store.SaveAgentDefinition(def); err != nil {
		p.errMsg = err.Error()
		return nil
	}

	return func() tea.Msg { return AgentRegisteredMsg{} }
}

func (p *agentRegisterPane) View() string {
	w := p.width
	if w <= 0 {
		w = 56
	}

	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent).Render("Register Custom Agent"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	for i, f := range p.fields {
		label := f.label
		if i == p.cursor {
			label = "> " + label
		} else {
			label = "  " + label
		}
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render(label))
		b.WriteByte('\n')

		valStyle := lipgloss.NewStyle().Foreground(appstyles.Text)
		if i == p.cursor {
			valStyle = valStyle.Background(appstyles.Highlight)
		}
		b.WriteString("  " + valStyle.Render(f.value+"_"))
		b.WriteByte('\n')
	}

	if p.errMsg != "" {
		b.WriteByte('\n')
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Overdue).Render(p.errMsg))
	}

	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("Tab move · Enter next/save · Esc cancel"))

	return appstyles.StyleCache.MaxWidth(w).Render(b.String())
}

func (p *agentRegisterPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}
