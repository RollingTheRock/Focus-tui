package header

import (
	"fmt"
	"strings"
	"time"

	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/quotes"
	"focus/internal/styles"
	"focus/internal/ui/banner"
	"focus/internal/weather"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the Header sub-model.
type Model struct {
	cfg     config.Config
	theme   styles.Theme
	width   int
	weather string // e.g. "26C . Shanghai"
	timeStr string
	quote   string
}

// New creates a new Header model.
func New(cfg config.Config, theme styles.Theme) models.Panel {
	return &Model{
		cfg:     cfg,
		theme:   theme,
		weather: "loading weather...",
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
			m.weather = fmt.Sprintf("weather: %v", msg.err)
		} else if msg.info == nil {
			m.weather = "weather: no data"
		} else {
			m.weather = fmt.Sprintf("%s %dC . %s", msg.info.Icon, msg.info.Temp, msg.info.City)
		}
	}
	return m, nil
}

// View renders the header as two compact lines: weather+time, then quote.
func (m *Model) View() string {
	return m.ViewCompact(m.width, true)
}

// ViewBanner renders the big FOCUS FIGlet banner on the left with an info
// column (weather, time, pomodoro, quote) on the right. Returns 6 lines.
func (m *Model) ViewBanner(w int, pomoTimer, pomoPhase, linkedTodo string) string {
	if w <= 0 {
		w = 80
	}

	bannerLines := strings.Split(banner.Art, "\n")
	// Measure banner visual width.
	bannerW := 0
	for _, line := range bannerLines {
		lw := lipgloss.Width(line)
		if lw > bannerW {
			bannerW = lw
		}
	}

	// Ensure we have exactly 6 banner lines.
	for len(bannerLines) < 6 {
		bannerLines = append(bannerLines, "")
	}

	gutter := 3
	infoW := w - bannerW - gutter
	if infoW < 10 {
		// Not enough space for info column — just show banner.
		bannerStyle := lipgloss.NewStyle().Foreground(styles.Banner)
		return bannerStyle.Render(banner.Art)
	}

	// Build right-column info lines (one per banner row).
	infoLines := make([]string, 6)
	infoLines[0] = ""
	infoLines[1] = m.weather
	infoLines[2] = m.timeStr
	if pomoTimer != "" && pomoPhase != "" {
		infoLines[3] = pomoPhase + " " + pomoTimer
		if linkedTodo != "" {
			infoLines[4] = linkedTodo
		}
	}
	if m.quote != "" {
		// Put quote on the first empty slot from the bottom.
		if infoLines[4] == "" {
			infoLines[4] = "\"" + m.quote + "\""
		} else if infoLines[5] == "" {
			infoLines[5] = "\"" + m.quote + "\""
		}
	}

	// Style and compose each line.
	bannerStyle := lipgloss.NewStyle().Foreground(styles.Banner)
	infoStyle := lipgloss.NewStyle().Foreground(styles.Text)
	subtleStyle := lipgloss.NewStyle().Foreground(styles.Subtle).Italic(true)
	accentStyle := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)

	var result []string
	for i := 0; i < 6; i++ {
		left := bannerStyle.Render(bannerLines[i])
		// Pad left to fixed banner width.
		leftW := lipgloss.Width(left)
		if leftW < bannerW {
			left += strings.Repeat(" ", bannerW-leftW)
		}

		// Style the info line.
		var right string
		info := infoLines[i]
		switch {
		case i == 3 && info != "":
			right = accentStyle.Render(info)
		case strings.HasPrefix(info, "\""):
			right = subtleStyle.Render(info)
		case info != "":
			right = infoStyle.Render(info)
		}

		// Right-align info within infoW.
		rightW := lipgloss.Width(right)
		pad := infoW - rightW
		if pad < 0 {
			pad = 0
		}

		result = append(result, left+strings.Repeat(" ", gutter+pad)+right)
	}

	return strings.Join(result, "\n")
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
