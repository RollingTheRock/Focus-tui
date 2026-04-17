package app

import (
	"fmt"
	"strings"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overviewSummaryStats struct {
	Total        int
	Active       int
	Blocked      int
	Queued       int
	Dirty        int
	RunningAgent int
	Focus        int
	Recent       string
}

type overviewSummaryProvider func() workbenchOverviewContext

type overviewSummaryPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	provider overviewSummaryProvider
	width    int
	height   int
}

func newOverviewSummaryPane(id models.PaneID, meta models.PaneMeta, provider overviewSummaryProvider) *overviewSummaryPane {
	return &overviewSummaryPane{id: id, meta: meta, provider: provider}
}

func (p *overviewSummaryPane) Init() tea.Cmd { return nil }

func (p *overviewSummaryPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) { return p, nil }

func (p *overviewSummaryPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *overviewSummaryPane) View() string {
	stats := overviewSummaryStats{}
	if p.provider != nil {
		stats = p.provider().Stats
	}
	width := p.width
	if width <= 0 {
		width = 80
	}
	parts := []string{
		overviewSummaryCard("WORKTREES", fmt.Sprintf("%d", stats.Total)),
		overviewSummaryCard("ACTIVE", fmt.Sprintf("%d", stats.Active)),
		overviewSummaryCard("BLOCKED", fmt.Sprintf("%d", stats.Blocked)),
		overviewSummaryCard("QUEUED", fmt.Sprintf("%d", stats.Queued)),
		overviewSummaryCard("DIRTY", fmt.Sprintf("%d", stats.Dirty)),
		overviewSummaryCard("AGENTS", fmt.Sprintf("%d", stats.RunningAgent)),
		overviewSummaryCard("FOCUS", fmt.Sprintf("%d", stats.Focus)),
	}
	line := strings.Join(parts, "  ")
	if stats.Recent != "" {
		line += "\n" + overviewSummaryHintStyle.Render("recent: "+stats.Recent)
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func overviewSummaryCard(label, value string) string {
	return overviewSummaryCardStyle.Render(overviewSummaryLabelStyle.Render(label) + " " + overviewSummaryValueStyle.Render(value))
}

var (
	overviewSummaryCardStyle  = lipgloss.NewStyle().Padding(0, 1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	overviewSummaryLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	overviewSummaryValueStyle = lipgloss.NewStyle().Bold(true)
	overviewSummaryHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
