package header

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/quotes"
	"focus/internal/styles"
	"focus/internal/weather"
)

// Model is the Header sub-model.
type Model struct {
	cfg     config.Config
	theme   styles.Theme
	width   int
	weather string // e.g. "🌤 26°C · Shanghai"
	timeStr string
	quote   string
}

// New creates a new Header model.
func New(cfg config.Config, theme styles.Theme) models.Panel {
	return &Model{
		cfg:     cfg,
		theme:   theme,
		weather: "",
		timeStr: time.Now().Format(cfg.TimeFormat),
		quote:   quotes.Get(cfg.Quote.Source, cfg.Quote.CustomFile),
	}
}

// Init starts the tick and weather fetch commands.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		fetchWeatherCmd(m.cfg.Weather.City),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.timeStr = msg.t.Format(m.cfg.TimeFormat)
		return m, tickCmd()

	case weatherMsg:
		if msg.err != nil || msg.info == nil {
			m.weather = ""
		} else {
			m.weather = fmt.Sprintf("%s %d°C · %s", msg.info.Icon, msg.info.Temp, msg.info.City)
		}
	}
	return m, nil
}

// View renders the header.
func (m *Model) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	// Right-aligned time.
	timeBlock := m.theme.NormalStyle.Render(m.timeStr)
	// Left-aligned weather (or empty).
	weatherBlock := m.theme.NormalStyle.Render(m.weather)

	// Use a style that forces the time to the right edge.
	timeStyle := lipgloss.NewStyle().Width(w - lipgloss.Width(weatherBlock)).Align(lipgloss.Right)
	timeLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		weatherBlock,
		timeStyle.Render(timeBlock),
	)

	quoteLine := m.theme.SubtleStyle.Render(m.quote)

	return lipgloss.JoinVertical(lipgloss.Left, timeLine, quoteLine)
}

// SetSize updates the width.
func (m *Model) SetSize(width, height int) {
	m.width = width
}

// Messages ------------------------------------------------------------

type tickMsg struct {
	t time.Time
}

func tickCmd() tea.Cmd {
	return tea.Every(time.Minute, func(t time.Time) tea.Msg {
		return tickMsg{t: t}
	})
}

type weatherMsg struct {
	info *weather.Info
	err  error
}

func fetchWeatherCmd(city string) tea.Cmd {
	return func() tea.Msg {
		info, err := weather.Get(city)
		return weatherMsg{info: info, err: err}
	}
}
