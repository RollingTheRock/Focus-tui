package app

import (
	"fmt"
	"strings"

	"focus/internal/adapters/ccswitch"
	"focus/internal/agents"
	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AgentSelectedMsg is emitted when the user chooses an agent provider.
type AgentSelectedMsg struct {
	PaneID           models.PaneID
	WorktreeID       string
	Provider         agents.Provider
	ProviderConfigID string // cc-switch provider ID for one-off override
	Resume           bool   // true = resume previous session with --continue
}

// OpenProviderSelectMsg is emitted when the user picks Claude or Codex and
// we need a second overlay to choose the cc-switch provider.
type OpenProviderSelectMsg struct {
	PaneID     models.PaneID
	WorktreeID string
	Provider   agents.Provider
	Resume     bool
}

// CloseAgentSelectMsg closes the agent selection overlay.
type CloseAgentSelectMsg struct {
	ID models.PaneID
}

type agentSelectPane struct {
	id              models.PaneID
	meta            models.PaneMeta
	common          models.CommonModel
	worktree        string
	cursor          int
	options         []agentOption
	width           int
	height          int
	showResume    bool // true = showing resume/new sub-options
	resumeOptions []agentOption
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
			if p.showResume {
				p.showResume = false
				return p, nil
			}
			return p, func() tea.Msg {
				return CloseAgentSelectMsg{ID: p.id}
			}
		case "j", "down":
			max := len(p.options) - 1
			if p.showResume {
				max = len(p.resumeOptions) - 1
			}
			if p.cursor < max {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "enter":
			if p.showResume {
				if p.cursor >= 0 && p.cursor < len(p.resumeOptions) {
					// Claude was chosen and resume/start-fresh is confirmed;
					// open the provider selection overlay if cc-switch is installed,
					// otherwise fall back to direct launch.
					if ccswitch.IsInstalled() {
						return p, func() tea.Msg {
							return OpenProviderSelectMsg{
								PaneID:     p.id,
								WorktreeID: p.worktree,
								Provider:   agents.ProviderClaude,
								Resume:     p.cursor == 0,
							}
						}
					}
					return p, func() tea.Msg {
						return AgentSelectedMsg{
							PaneID:     p.id,
							WorktreeID: p.worktree,
							Provider:   agents.ProviderClaude,
							Resume:     p.cursor == 0,
						}
					}
				}
			} else if p.cursor >= 0 && p.cursor < len(p.options) {
				opt := p.options[p.cursor]
				if opt.provider == agents.ProviderClaude && agents.HasResumableClaudeSession(p.worktree) {
					p.showResume = true
					p.cursor = 0
					p.resumeOptions = []agentOption{
						{provider: agents.ProviderClaude, name: "Resume session", desc: "continue previous conversation"},
						{provider: agents.ProviderClaude, name: "Start fresh", desc: "begin a new session"},
					}
					return p, nil
				}
				// For Claude or Codex, open provider selection if cc-switch is installed.
				if (opt.provider == agents.ProviderClaude || opt.provider == agents.ProviderCodex) && ccswitch.IsInstalled() {
					return p, func() tea.Msg {
						return OpenProviderSelectMsg{
							PaneID:     p.id,
							WorktreeID: p.worktree,
							Provider:   opt.provider,
						}
					}
				}
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

	if p.showResume {
		b.WriteString(agentSelectHeaderStyle.Render("Claude Code — Resume?"))
		b.WriteByte('\n')
		b.WriteString(agentSelectHintStyle.Render("A previous session exists in this worktree."))
		b.WriteByte('\n')
		b.WriteByte('\n')

		for i, opt := range p.resumeOptions {
			cursor := "  "
			if i == p.cursor {
				cursor = "▸ "
			}
			nameStyle := lipgloss.NewStyle().Foreground(appstyles.Text).Bold(true)
			if i == p.cursor {
				nameStyle = nameStyle.Background(appstyles.Highlight)
			}
			line := fmt.Sprintf("%s%s  %s", cursor, nameStyle.Render(opt.name), agentSelectHintStyle.Render("("+opt.desc+")"))
			b.WriteString(appstyles.StyleCache.MaxWidth(width).Render(line))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
		b.WriteString(agentSelectHintStyle.Render("↑↓ move · Enter select · Esc back"))
	} else {
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
	}

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
