package app

import (
	"fmt"
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CloseAgentInstallHintMsg closes the install hint overlay.
type CloseAgentInstallHintMsg struct{}

// AgentInstallShellMsg requests running the install command in the embedded shell.
type AgentInstallShellMsg struct{ AgentID string }

// AgentInstallExternalMsg requests opening an external terminal to run the install.
type AgentInstallExternalMsg struct{ AgentID string }

// AgentInstallCopiedMsg signals the install command was copied to clipboard.
type AgentInstallCopiedMsg struct{ AgentID string }

type agentInstallHintPane struct {
	id      models.PaneID
	meta    models.PaneMeta
	common  models.CommonModel
	agentID string
	width   int
	height  int
}

func newAgentInstallHintPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel, agentID string) *agentInstallHintPane {
	return &agentInstallHintPane{
		id:      id,
		meta:    meta,
		common:  common,
		agentID: agentID,
	}
}

func (p *agentInstallHintPane) Init() tea.Cmd { return nil }

func (p *agentInstallHintPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return p, func() tea.Msg { return CloseAgentInstallHintMsg{} }
		case "x":
			return p, func() tea.Msg { return AgentInstallShellMsg{AgentID: p.agentID} }
		case "o":
			return p, func() tea.Msg { return AgentInstallExternalMsg{AgentID: p.agentID} }
		case "c":
			return p, func() tea.Msg { return AgentInstallCopiedMsg{AgentID: p.agentID} }
		}
	}
	return p, nil
}

func (p *agentInstallHintPane) View() string {
	w := p.width
	if w <= 0 {
		w = 64
	}

	def, err := p.common.Store.GetAgentDefinition(p.agentID)
	if err != nil || def == nil {
		return lipgloss.NewStyle().Foreground(appstyles.Warning).Render("Agent not found.")
	}

	// Inner content width (accounting for box padding).
	innerW := w - 6
	if innerW < 40 {
		innerW = 40
	}

	var b strings.Builder

	// ── Title ──
	title := fmt.Sprintf("📦  %s", def.Name)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent).Render(title))
	b.WriteByte('\n')

	// ── Separator ──
	sepStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	b.WriteString(sepStyle.Render(strings.Repeat("━", innerW)))
	b.WriteByte('\n')

	// ── Description ──
	if def.Description != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Text).Render(def.Description))
		b.WriteByte('\n')
	}

	// ── Metadata (compact inline) ──
	metaStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	var metaParts []string
	if len(def.Tags) > 0 {
		metaParts = append(metaParts, "🏷️ "+strings.Join(def.Tags, ", "))
	}
	metaParts = append(metaParts, fmt.Sprintf("📂 %s", def.Category))
	metaParts = append(metaParts, fmt.Sprintf("⚙️  %s", def.ProviderType))
	b.WriteString(metaStyle.Render(strings.Join(metaParts, "  ·  ")))
	b.WriteByte('\n')
	b.WriteByte('\n')

	// ── Install command box ──
	cmdBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(appstyles.AccentDim).
		Padding(0, 1).
		Width(innerW)

	var cmd string
	if def.InstallHint != "" {
		cmd = def.InstallHint
	} else {
		cmd = "No install command available."
	}
	b.WriteString(cmdBoxStyle.Render(lipgloss.NewStyle().Foreground(appstyles.Text).Render(cmd)))
	b.WriteByte('\n')
	b.WriteByte('\n')

	// ── Actions (prominent, always visible) ──
	actStyle := lipgloss.NewStyle().Foreground(appstyles.Accent).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	b.WriteString(actStyle.Render("[x]"))
	b.WriteString(dimStyle.Render(" Shell  "))
	b.WriteString(actStyle.Render("[o]"))
	b.WriteString(dimStyle.Render(" Terminal  "))
	b.WriteString(actStyle.Render("[c]"))
	b.WriteString(dimStyle.Render(" Copy  "))
	b.WriteString(actStyle.Render("[q]"))
	b.WriteString(dimStyle.Render(" close"))
	b.WriteByte('\n')

	// ── Compact post-install hint ──
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("💡  After install, source your shell config, then press r in Store."))

	// Wrap everything in a box.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(appstyles.Accent).
		Padding(1, 2).
		Width(w)

	return box.Render(b.String())
}

func (p *agentInstallHintPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// wrapAndStyle wraps text to the given width and applies the style.
func wrapAndStyle(text string, width int, style lipgloss.Style) string {
	lines := hintWrapText(text, width)
	return style.Render(strings.Join(lines, "\n"))
}

// hintWrapText splits a string into lines no longer than maxWidth.
func hintWrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	var lines []string
	words := strings.Fields(text)
	var current strings.Builder
	for _, word := range words {
		if current.Len()+len(word)+1 > maxWidth && current.Len() > 0 {
			lines = append(lines, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		lines = append(lines, strings.TrimSpace(current.String()))
	}
	if len(lines) == 0 {
		lines = append(lines, text)
	}
	return lines
}
