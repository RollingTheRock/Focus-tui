package app

import (
	"fmt"
	"strings"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overviewDetailSelection struct {
	TaskID            string
	Title             string
	Branch            string
	WorktreePath      string
	State             string
	Priority          string
	ResumeReason      string
	ResumeHint        string
	Goal              string
	WhyNow            string
	SuccessCriteria   string
	OutOfScope        string
	KnownRisks        string
	NextStep          string
	PlanTitle         string
	PlanStatus        string
	CurrentPlanStep   string
	PlanBody          string
	PlanSteps         []string
	HandoffEntrypoint string
	Attention         string
	RecentArtifact    string
	AgentSummary      string
	QueuedSummary     string
	GitSummary        string
	GitPressure       string
	RuntimeSummary    string
	Upstream          string
	DAGSummary        string
	DAGLanes          []string
	DAGEdges          []string
	UpstreamTasks     []string
	DownstreamTasks   []string
	SharedContext     []string
	PinnedNote        string
	BlockerNote       string
	HandoffNote       string
	HasSelection      bool
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
	if selection.WhyNow != "" || selection.SuccessCriteria != "" || selection.OutOfScope != "" || selection.KnownRisks != "" {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("Brief"),
		)
		if selection.WhyNow != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("why now: "+selection.WhyNow))
		}
		if selection.SuccessCriteria != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("success: "+selection.SuccessCriteria))
		}
		if selection.OutOfScope != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("out of scope: "+selection.OutOfScope))
		}
		if selection.KnownRisks != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("risks: "+selection.KnownRisks))
		}
	}
	if selection.PlanTitle != "" || selection.PlanStatus != "" || selection.CurrentPlanStep != "" || selection.PlanBody != "" || len(selection.PlanSteps) > 0 {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("Plan"),
		)
		if selection.PlanTitle != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("plan: "+selection.PlanTitle))
		}
		if selection.PlanStatus != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("status: "+selection.PlanStatus))
		}
		if selection.CurrentPlanStep != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("step: "+selection.CurrentPlanStep))
		}
		if selection.PlanBody != "" {
			lines = append(lines, overviewDetailBodyStyle.Render(selection.PlanBody))
		}
		if len(selection.PlanSteps) > 0 {
			lines = append(lines, overviewDetailBodyStyle.Render("decomposition:"))
			for _, step := range selection.PlanSteps {
				lines = append(lines, overviewDetailBodyStyle.Render("  "+step))
			}
		}
	}
	if selection.DAGSummary != "" || len(selection.DAGLanes) > 0 || len(selection.DAGEdges) > 0 {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("DAG"),
		)
		if selection.DAGSummary != "" {
			lines = append(lines, overviewDetailBodyStyle.Render(selection.DAGSummary))
		}
		for i, lane := range selection.DAGLanes {
			if i >= 4 {
				remaining := len(selection.DAGLanes) - i
				lines = append(lines, overviewDetailMutedStyle.Render(fmt.Sprintf("...+%d more lanes", remaining)))
				break
			}
			lines = append(lines, overviewDetailBodyStyle.Render(lane))
		}
		for i, edge := range selection.DAGEdges {
			if i >= 4 {
				remaining := len(selection.DAGEdges) - i
				lines = append(lines, overviewDetailMutedStyle.Render(fmt.Sprintf("...+%d more edges", remaining)))
				break
			}
			lines = append(lines, overviewDetailMutedStyle.Render(edge))
		}
	}
	if len(selection.UpstreamTasks) > 0 || len(selection.DownstreamTasks) > 0 {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("Dependencies"),
		)
		if len(selection.UpstreamTasks) > 0 {
			lines = append(lines, overviewDetailBodyStyle.Render("upstream:"))
			for _, row := range selection.UpstreamTasks {
				lines = append(lines, overviewDetailBodyStyle.Render("  - "+row))
			}
		}
		if len(selection.DownstreamTasks) > 0 {
			lines = append(lines, overviewDetailBodyStyle.Render("downstream:"))
			for _, row := range selection.DownstreamTasks {
				lines = append(lines, overviewDetailBodyStyle.Render("  - "+row))
			}
		}
	}
	if len(selection.SharedContext) > 0 {
		lines = append(lines,
			"",
			overviewDetailSectionStyle.Render("Shared Context"),
		)
		for _, row := range selection.SharedContext {
			lines = append(lines, overviewDetailBodyStyle.Render(row))
		}
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
		if selection.HandoffEntrypoint != "" {
			lines = append(lines, overviewDetailBodyStyle.Render("entrypoint: "+selection.HandoffEntrypoint))
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
