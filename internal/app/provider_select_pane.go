package app

import (
	"fmt"
	"strings"

	"focus/internal/adapters/ccswitch"
	"focus/internal/agents"
	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseProviderSelectMsg closes the provider selection overlay.
type CloseProviderSelectMsg struct {
	ID models.PaneID
}

type providerSelectPane struct {
	id              models.PaneID
	meta            models.PaneMeta
	common          models.CommonModel
	worktree        string
	provider        agents.Provider
	cursor          int
	options         []ccswitch.Provider
	width           int
	height          int
	resume          bool
	ccswitchMissing bool
}

var (
	providerSelectHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)
	providerSelectHintStyle   = lipgloss.NewStyle().Foreground(appstyles.Subtle)
)

func newProviderSelectPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, worktree string, provider agents.Provider, resume bool) *providerSelectPane {
	p := &providerSelectPane{
		id:       id,
		meta:     meta,
		common:   common,
		worktree: worktree,
		provider: provider,
		resume:   resume,
	}
	if !ccswitch.IsInstalled() {
		p.ccswitchMissing = true
	} else {
		opts, _ := ccswitch.ListProviders(string(provider))
		p.options = opts
		for i, opt := range p.options {
			if opt.IsCurrent {
				p.cursor = i
				break
			}
		}
	}
	return p
}

func (p *providerSelectPane) Init() tea.Cmd {
	return nil
}

func (p *providerSelectPane) KeyBindings(compact bool) []models.KeyBinding {
	return []models.KeyBinding{
		{Keys: []string{"j", "k"}, Help: "move"},
		{Keys: []string{"enter"}, Help: "select"},
		{Keys: []string{"esc"}, Help: "use default"},
	}
}

func (p *providerSelectPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "esc":
			return p, func() tea.Msg {
				return CloseProviderSelectMsg{ID: p.id}}
		case "j", "down":
			if p.cursor < len(p.options)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "enter":
			var configID string
			if p.cursor >= 0 && p.cursor < len(p.options) {
				configID = p.options[p.cursor].ID
			}
			return p, func() tea.Msg {
				return AgentSelectedMsg{
					PaneID:           p.id,
					WorktreeID:       p.worktree,
					Provider:         p.provider,
					ProviderConfigID: configID,
					Resume:           p.resume,
				}
			}
		}
	}
	return p, nil
}

func (p *providerSelectPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 56
	}

	var b strings.Builder

	providerName := string(p.provider)
	switch p.provider {
	case agents.ProviderClaude:
		providerName = "Claude Code"
	case agents.ProviderCodex:
		providerName = "Codex"
	case agents.ProviderOpenCode:
		providerName = "OpenCode"
	case agents.ProviderGemini:
		providerName = "Gemini CLI"
	}

	b.WriteString(providerSelectHeaderStyle.Render(fmt.Sprintf("%s — Select Provider", providerName)))
	b.WriteByte('\n')
	if p.ccswitchMissing {
		b.WriteString(providerSelectHintStyle.Render("cc-switch is not installed. Press Esc to continue with default."))
	} else if len(p.options) == 0 {
		b.WriteString(providerSelectHintStyle.Render("No providers configured in cc-switch. Press Esc to continue with default."))
	} else {
		b.WriteString(providerSelectHintStyle.Render("Choose a provider for this session only."))
	}
	b.WriteByte('\n')
	b.WriteByte('\n')

	for i, opt := range p.options {
		cursor := "  "
		if i == p.cursor {
			cursor = "▸ "
		}

		nameStyle := lipgloss.NewStyle().Foreground(appstyles.Text).Bold(true)
		if i == p.cursor {
			nameStyle = nameStyle.Background(appstyles.Highlight)
		}

		var parts []string
		parts = append(parts, nameStyle.Render(opt.Name))
		if opt.Model != "" {
			parts = append(parts, providerSelectHintStyle.Render("("+opt.Model+")"))
		}
		if opt.IsCurrent {
			parts = append(parts, lipgloss.NewStyle().Foreground(appstyles.Success).Render("● default"))
		}

		line := cursor + strings.Join(parts, "  ")
		b.WriteString(appstyles.StyleCache.MaxWidth(width).Render(line))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	if len(p.options) > 0 {
		b.WriteString(providerSelectHintStyle.Render("↑↓ move · Enter select · Esc use default"))
	} else if p.ccswitchMissing {
		b.WriteString(providerSelectHintStyle.Render("Press Esc to use default"))
	} else {
		b.WriteString(providerSelectHintStyle.Render("Press Enter to use default · Esc to cancel"))
	}

	lines := strings.Split(b.String(), "\n")
	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (p *providerSelectPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}
