package app

import (
	"strings"

	"focus/internal/models"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type tabContainer struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	tabs      []string
	activeTab int
	dagPane   *dagPane
	adrPane   *adrPane

	width  int
	height int
}

func newTabContainer(id models.PaneID, meta models.PaneMeta, common *models.CommonModel, dag *dagPane, adr *adrPane) *tabContainer {
	return &tabContainer{
		id:        id,
		meta:      meta,
		common:    *common,
		tabs:      []string{"Tasks", "ADRs"},
		activeTab: 0,
		dagPane:   dag,
		adrPane:   adr,
	}
}

func (tc *tabContainer) HandleTab() bool {
	return false // tab now cycles pane focus globally; use [/] for tab switching
}

func (tc *tabContainer) activePane() models.Panel {
	if tc.activeTab == 0 {
		return tc.dagPane
	}
	return tc.adrPane
}

func (tc *tabContainer) Init() tea.Cmd {
	return tea.Batch(tc.dagPane.Init(), tc.adrPane.Init())
}

func (tc *tabContainer) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case dagRefreshMsg:
		newPane, cmd := tc.dagPane.Update(msg)
		if dp, ok := newPane.(*dagPane); ok {
			tc.dagPane = dp
		}
		return tc, cmd
	case adrRefreshMsg, adrLoadConstraintsMsg:
		newPane, cmd := tc.adrPane.Update(msg)
		if ap, ok := newPane.(*adrPane); ok {
			tc.adrPane = ap
		}
		return tc, cmd
	case tea.KeyMsg:
		key := msg.String()
		if key == "T" {
			tc.activeTab = 0
			return tc, nil
		}
		if key == "A" {
			tc.activeTab = 1
			return tc, nil
		}
		newPane, cmd := tc.activePane().Update(msg)
		if tc.activeTab == 0 {
			if dp, ok := newPane.(*dagPane); ok {
				tc.dagPane = dp
			}
		} else {
			if ap, ok := newPane.(*adrPane); ok {
				tc.adrPane = ap
			}
		}
		return tc, cmd
	default:
		newPane, cmd := tc.activePane().Update(msg)
		if tc.activeTab == 0 {
			if dp, ok := newPane.(*dagPane); ok {
				tc.dagPane = dp
			}
		} else {
			if ap, ok := newPane.(*adrPane); ok {
				tc.adrPane = ap
			}
		}
		return tc, cmd
	}
}

func (tc *tabContainer) setActivePane(updated models.Panel) {
	if tc.activeTab == 0 {
		if dp, ok := updated.(*dagPane); ok {
			tc.dagPane = dp
		}
	} else {
		if ap, ok := updated.(*adrPane); ok {
			tc.adrPane = ap
		}
	}
}

func (tc *tabContainer) SetSize(width, height int) {
	tc.width = width
	tc.height = height
	contentH := height - 2
	if contentH < 3 {
		contentH = 3
	}
	tc.dagPane.SetSize(width, contentH)
	tc.adrPane.SetSize(width, contentH)
}

func (tc *tabContainer) View() tea.View {
	w := tc.width
	if w <= 0 {
		w = 80
	}
	h := tc.height
	if h <= 0 {
		h = 10
	}

	var lines []string
	lines = append(lines, tc.renderTabBar(w))
	contentH := h - 1
	if contentH < 2 {
		contentH = 2
	}
	tc.activePane().SetSize(w, contentH)
	content := tc.activePane().View()
	lines = append(lines, content.Content)
	return tea.NewView(lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	))
}

func (tc *tabContainer) renderTabBar(w int) string {
	var parts []string
	for i, name := range tc.tabs {
		if i == tc.activeTab {
			parts = append(parts, tabActiveStyle.Render(" ●"+name+" "))
		} else {
			parts = append(parts, tabInactiveStyle.Render("  "+name+" "))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	padding := w - lipgloss.Width(bar) - 2
	if padding < 0 {
		padding = 0
	}
	return " " + bar + strings.Repeat(" ", padding)
}

func (tc *tabContainer) refreshDAG() {
	tc.dagPane.buildDAG()
}

func (tc *tabContainer) selectedTask() (models.TaskContextRecord, bool) {
	return tc.dagPane.selectedTask()
}

var (
	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent).Underline(true)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(styles.Subtle)
)
