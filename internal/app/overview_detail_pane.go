package app

import (
	"strings"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overviewDetailSelection struct {
	Title          string
	Branch         string
	WorktreePath   string
	State          string
	Priority       string
	ResumeReason   string
	ResumeHint     string
	Goal           string
	NextStep       string
	Attention      string
	RecentArtifact string
	AgentSummary   string
	QueuedSummary  string
	GitSummary     string
	GitPressure    string
	RuntimeSummary string
	Upstream       string
	PinnedNote     string
	BlockerNote    string
	HandoffNote    string
	HasSelection   bool
}

type overviewDetailProvider func() workbenchOverviewContext

type overviewDetailPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	provider overviewDetailProvider
	width    int
	height   int
}

func newOverviewDetailPane(id models.PaneID, meta models.PaneMeta, provider overviewDetailProvider) *overviewDetailPane {
	return &overviewDetailPane{id: id, meta: meta, provider: provider}
}

func (p *overviewDetailPane) Init() tea.Cmd { return nil }

func (p *overviewDetailPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) { return p, nil }

func (p *overviewDetailPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *overviewDetailPane) View() string {
	selection := overviewDetailSelection{}
	if p.provider != nil {
		selection = p.provider().Detail
	}
	width := p.width
	if width <= 0 {
		width = 48
	}
	lines := []string{overviewDetailHeaderStyle.Render("Context Detail")}
	if !selection.HasSelection {
		lines = append(lines, overviewDetailMutedStyle.Render("Select a task/worktree from the queue to inspect context."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines,
		overviewDetailTitleStyle.Render(selection.Title),
		overviewDetailMutedStyle.Render(compactJoin("branch: "+selection.Branch, "state: "+selection.State, "priority: "+selection.Priority)),
		"",
		overviewDetailSectionStyle.Render("Now"),
		overviewDetailBodyStyle.Render(compactJoin(selection.ResumeReason, selection.ResumeHint)),
	)
	if selection.NextStep != "" {
		lines = append(lines, overviewDetailBodyStyle.Render("next: "+selection.NextStep))
	}
	if selection.Goal != "" {
		lines = append(lines, overviewDetailBodyStyle.Render("goal: "+selection.Goal))
	}
	if selection.Attention != "" || selection.RecentArtifact != "" {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("Context"),
		)
		if selection.Attention != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("attention: "+selection.Attention))
		}
		if selection.RecentArtifact != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("artifact: "+selection.RecentArtifact))
		}
	}
	lines = append(lines,
		"",
		overviewDetailSectionStyle.Render("Signals"),
	)
	for _, item := range []string{selection.GitSummary, selection.GitPressure, selection.RuntimeSummary, selection.AgentSummary, selection.QueuedSummary, selection.Upstream} {
		if strings.TrimSpace(item) != "" {
			lines = append(lines, overviewDetailBodyStyle.Render(item))
		}
	}
	if selection.PinnedNote != "" || selection.BlockerNote != "" || selection.HandoffNote != "" {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("Notes"),
		)
		if selection.PinnedNote != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("note: "+selection.PinnedNote))
		}
		if selection.BlockerNote != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("blocker: "+selection.BlockerNote))
		}
		if selection.HandoffNote != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("handoff: "+selection.HandoffNote))
		}
	}
	lines = append(lines,
		"",
		overviewDetailSectionStyle.Render("Path"),
		overviewDetailMutedStyle.Render(selection.WorktreePath),
	)

	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(line)
	}
	return strings.Join(lines, "\n")
}

func compactJoin(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " · ")
}

var (
	overviewDetailHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	overviewDetailTitleStyle   = lipgloss.NewStyle().Bold(true)
	overviewDetailSectionStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	overviewDetailMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	overviewDetailBodyStyle    = lipgloss.NewStyle()
)
