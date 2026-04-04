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
		weather: "⏳ loading weather...",
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
		if msg.err != nil {
			m.weather = fmt.Sprintf("⚠ weather: %v", msg.err)
		} else if msg.info == nil {
			m.weather = "⚠ weather: no data"
		} else {
			m.weather = fmt.Sprintf("%s %d°C · %s", msg.info.Icon, msg.info.Temp, msg.info.City)
		}
	}
	return m, nil
}

// View renders the header as two compact lines: weather+time, then quote.
func (m *Model) View() string {
	return m.ViewCompact(m.width, true)
}

// ViewCompact renders the header in the given width. If showQuote is false, only
// the weather+time line is returned.
func (m *Model) ViewCompact(w int, showQuote bool) string {
	if w <= 0 {
		w = 78
	}

	weatherBlock := m.theme.NormalStyle.Render(" " + m.weather)
	timeBlock := m.theme.NormalStyle.Render(m.timeStr + " ")

	timeStyle := lipgloss.NewStyle().Width(w - lipgloss.Width(weatherBlock)).Align(lipgloss.Right)
	timeLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		weatherBlock,
		timeStyle.Render(timeBlock),
	)

	if !showQuote || m.quote == "" {
		return timeLine
	}

	quoteStyle := lipgloss.NewStyle().Foreground(styles.Subtle).Italic(true)
	quoteLine := quoteStyle.Render(" \"" + m.quote + "\"")

	return timeLine + "\n" + quoteLine
}

// SetSize updates the width.
func (m *Model) SetSize(width, height int) {
	m.width = width
}

// --- Exported getters for app-level layout ---

// WeatherStr returns the current weather display string.
func (m *Model) WeatherStr() string {
	return m.weather
}

// TimeStr returns the current time display string.
func (m *Model) TimeStr() string {
	return m.timeStr
}

// QuoteStr returns the daily quote.
func (m *Model) QuoteStr() string {
	return m.quote
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
