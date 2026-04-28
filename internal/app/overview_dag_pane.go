package app

import (
	"fmt"
	"strings"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overviewDAGProvider func() workbenchOverviewContext

type overviewDAGPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	provider overviewDAGProvider
	width    int
	height   int
}

func newOverviewDAGPane(id models.PaneID, meta models.PaneMeta, provider overviewDAGProvider) *overviewDAGPane {
	return &overviewDAGPane{id: id, meta: meta, provider: provider}
}

func (p *overviewDAGPane) Init() tea.Cmd { return nil }

func (p *overviewDAGPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) { return p, nil }

func (p *overviewDAGPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *overviewDAGPane) View() string {
	ctx := workbenchOverviewContext{}
	if p.provider != nil {
		ctx = p.provider()
	}
	dag := ctx.DAG
	width := p.width
	if width <= 0 {
		width = 64
	}
	height := p.height
	if height <= 0 {
		height = 18
	}

	lines := []string{overviewDAGHeaderStyle.Render("Task DAG")}
	if dag.NodeCount == 0 {
		lines = append(lines, overviewDAGMutedStyle.Render("No task graph available yet."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, overviewDAGBodyStyle.Render(dag.SummaryLine()))
	if dag.FocusTaskID != "" {
		focusLabel := dag.FocusTaskID
		if dag.FocusTaskTitle != "" {
			focusLabel = fmt.Sprintf("%s (%s)", dag.FocusTaskTitle, dag.FocusTaskID)
		}
		lines = append(lines, overviewDAGMutedStyle.Render("focus: "+focusLabel))
	}
	lines = append(lines, "", overviewDAGSectionStyle.Render("Lanes"))
	for _, lane := range dag.LaneLines {
		lines = append(lines, overviewDAGBodyStyle.Render(lane))
	}
	lines = append(lines, "", overviewDAGSectionStyle.Render("Edges"))
	for i, edge := range dag.EdgeLines {
		if i >= 8 {
			remaining := len(dag.EdgeLines) - i
			lines = append(lines, overviewDAGMutedStyle.Render(fmt.Sprintf("...+%d more edges", remaining)))
			break
		}
		lines = append(lines, overviewDAGMutedStyle.Render(edge))
	}

	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(line)
	}
	return strings.Join(lines, "\n")
}

var (
	overviewDAGHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	overviewDAGSectionStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	overviewDAGMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	overviewDAGBodyStyle    = lipgloss.NewStyle()
)
