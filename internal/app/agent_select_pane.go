package app

import (
	"fmt"
	"strings"

	"focus/internal/agents"
	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AgentSelectedMsg is emitted when the user chooses an agent provider.
type AgentSelectedMsg struct {
	PaneID     models.PaneID
	WorktreeID string
	Provider   agents.Provider
}

// CloseAgentSelectMsg closes the agent selection overlay.
type CloseAgentSelectMsg struct {
	ID models.PaneID
}

type agentSelectPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	common   models.CommonModel
	worktree string
	cursor   int
	options  []agentOption
	width    int
	height   int
}

type agentOption struct {
	provider agents.Provider
	name     string
	desc     string
}

var (
	agentSelectHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)
	agentSelectHintStyle   = lipgloss.NewStyle().Foreground(appstyles.Subtle)
)

func newAgentSelectPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, worktree string) *agentSelectPane {
	opts := []agentOption{
		{provider: agents.ProviderKimi, name: "Kimi", desc: "kimi"},
		{provider: agents.ProviderCodex, name: "Codex", desc: "codex"},
		{provider: agents.ProviderClaude, name: "Claude Code", desc: "claude"},
		{provider: agents.ProviderOpenCode, name: "OpenCode", desc: "opencode"},
	}

	// Move the first installed option to the top as a sensible default.
	for i, opt := range opts {
		if agents.IsInstalled(opt.provider) {
			opts[0], opts[i] = opts[i], opts[0]
			break
		}
	}

	return &agentSelectPane{
		id:       id,
		meta:     meta,
		common:   common,
		worktree: worktree,
		options:  opts,
	}
}

func (p *agentSelectPane) Init() tea.Cmd {
	return nil
}

func (p *agentSelectPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg {
				return CloseAgentSelectMsg{ID: p.id}
			}
		case "j", "down":
			if p.cursor < len(p.options)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "enter":
			if p.cursor >= 0 && p.cursor < len(p.options) {
				opt := p.options[p.cursor]
				return p, func() tea.Msg {
					return AgentSelectedMsg{
						PaneID:     p.id,
						WorktreeID: p.worktree,
						Provider:   opt.provider,
					}
				}
			}
		}
	}
	return p, nil
}

func (p *agentSelectPane) View() string {
	width := p.width
	if width <= 0 {
		width = 56
	}

	var b strings.Builder
	b.WriteString(agentSelectHeaderStyle.Render("Select Terminal Agent"))
	b.WriteByte('\n')
	b.WriteString(agentSelectHintStyle.Render("Choose which agent to launch in the external terminal."))
	b.WriteByte('\n')
	b.WriteByte('\n')

	for i, opt := range p.options {
		cursor := "  "
		if i == p.cursor {
			cursor = "▸ "
		}

		installed := agents.IsInstalled(opt.provider)
		status := lipgloss.NewStyle().Foreground(appstyles.Success).Render("● installed")
		if !installed {
			status = lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("○ not found")
		}

		nameStyle := lipgloss.NewStyle().Foreground(appstyles.Text).Bold(true)
		if i == p.cursor {
			nameStyle = nameStyle.Background(appstyles.Highlight)
		}

		line := fmt.Sprintf("%s%s  %s  %s", cursor, nameStyle.Render(opt.name), agentSelectHintStyle.Render("("+opt.desc+")"), status)
		b.WriteString(appstyles.StyleCache.MaxWidth(width).Render(line))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(agentSelectHintStyle.Render("↑↓ move · Enter select · Esc cancel"))

	lines := strings.Split(b.String(), "\n")
	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	return strings.Join(lines, "\n")
}

func (p *agentSelectPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func closeAgentSelectCmd(id models.PaneID) tea.Cmd {
	return func() tea.Msg { return CloseAgentSelectMsg{ID: id} }
}
