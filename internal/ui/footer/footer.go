package footer

import (
	"fmt"

	"focus/internal/models"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the Footer panel sub-model.
type Model struct {
	common *models.CommonModel
	width  int
	height int

	streak     int
	todayPomo  int
	todayDone  int
	todayTotal int
}

// New creates a new Footer panel.
func New(common *models.CommonModel) models.Panel {
	return &Model{common: common}
}

func (m *Model) Init() tea.Cmd {
	return m.loadStats
}

func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg.(type) {
	case statsLoadedMsg:
		s := msg.(statsLoadedMsg)
		m.streak = s.streak
		m.todayPomo = s.todayPomo
		m.todayDone = s.todayDone
		m.todayTotal = s.todayTotal
	}
	return m, nil
}

// Refresh reloads stats from the store. Called by the app on StatsRefreshMsg.
func (m *Model) Refresh() tea.Cmd {
	return m.loadStats
}

func (m *Model) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	sep := lipgloss.NewStyle().Foreground(styles.Subtle).Render(" · ")

	streakStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	pomoStyle := lipgloss.NewStyle().Foreground(styles.Accent)
	doneStyle := lipgloss.NewStyle().Foreground(styles.Success)

	streakStr := streakStyle.Render(fmt.Sprintf("🔥 streak %dd", m.streak))
	pomoStr := pomoStyle.Render(fmt.Sprintf("🍅 today %d", m.todayPomo))
	doneStr := doneStyle.Render(fmt.Sprintf("✓ done %d/%d", m.todayDone, m.todayTotal))

	content := streakStr + sep + pomoStr + sep + doneStr

	bar := lipgloss.NewStyle().
		Width(w).
		Align(lipgloss.Center).
		Render(content)

	return bar
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// --- Stats loading ---

type statsLoadedMsg struct {
	streak     int
	todayPomo  int
	todayDone  int
	todayTotal int
}

func (m *Model) loadStats() tea.Msg {
	streak, _ := m.common.Store.GetStreak()
	pomo, _ := m.common.Store.TodaySessionCount()
	done, total, _ := m.common.Store.TodayDoneCount()
	return statsLoadedMsg{
		streak:     streak,
		todayPomo:  pomo,
		todayDone:  done,
		todayTotal: total,
	}
}
