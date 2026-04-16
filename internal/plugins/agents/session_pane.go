package agents

import (
	"fmt"
	"path/filepath"
	"time"

	"focus/internal/agents"
	"focus/internal/models"
	"focus/internal/render"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	_ models.Panel    = (*SessionPane)(nil)
	_ render.Renderer = (*SessionPane)(nil)
)

type SessionPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	common   models.CommonModel
	cursor   int
	sessions []*agents.Session
	width    int
	height   int
	err      error
	notice   string
}

type KillSessionMsg struct {
	SessionID string
	PID       int
}

type FocusAgentSessionMsg struct {
	WorktreeID string
	Provider   agents.Provider
}

func NewSessionPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel) *SessionPane {
	return &SessionPane{
		id:     id,
		meta:   meta,
		common: common,
	}
}

func (p *SessionPane) Init() tea.Cmd {
	return nil
}

func (p *SessionPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if p.cursor < len(p.sessions)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "r":
			p.notice = "Refreshing agent sessions..."
			return p, nil
		case "enter":
			if s, ok := p.selectedSession(); ok {
				return p, func() tea.Msg {
					return FocusAgentSessionMsg{WorktreeID: s.WorktreeID, Provider: s.Provider}
				}
			}
		case "x":
			if s, ok := p.selectedSession(); ok {
				p.notice = fmt.Sprintf("Killing %s (%d)...", s.DisplayName(), s.PID)
				return p, func() tea.Msg {
					return KillSessionMsg{SessionID: s.ID, PID: s.PID}
				}
			}
		case "a":
			if s, ok := p.selectedSession(); ok {
				p.notice = fmt.Sprintf("Launching %s in %s...", s.DisplayName(), shortenPath(s.WorktreeID))
				return p, func() tea.Msg {
					return agents.LaunchAgentMsg{WorktreeID: s.WorktreeID, Provider: s.Provider}
				}
			}
		}
	}
	return p, nil
}

func (p *SessionPane) View() string {
	width := p.width
	if width <= 0 {
		width = 40
	}
	height := p.height
	if height <= 0 {
		height = 24
	}
	canvas := render.NewCanvas(width, height)
	p.Render(canvas, width, height)
	return canvas.Render()
}

func (p *SessionPane) Render(canvas render.Surface, width, height int) {
	var lines []renderedLine
	lines = append(lines, renderedLine{content: "Agent Sessions", style: &sectionStyle})
	lines = append(lines, renderedLine{content: "", style: nil})

	if len(p.sessions) == 0 {
		lines = append(lines, renderedLine{content: "No running agents.", style: &emptyStyle})
	} else {
		for i, s := range p.sessions {
			line := fmt.Sprintf("%s  pid:%d  %s  %s", s.DisplayName(), s.PID, shortenPath(s.WorktreeID), formatDuration(time.Since(s.StartedAt)))
			style := (*lipgloss.Style)(nil)
			if i == p.cursor {
				style = &selectedRowStyle
			}
			lines = append(lines, renderedLine{content: line, style: style})
		}
	}

	lines = append(lines, renderedLine{content: "", style: nil})
	lines = append(lines, renderedLine{content: "enter:focus  a:launch  x:kill  r:refresh", style: &hintStyle})

	if p.notice != "" {
		lines = append(lines, renderedLine{content: "", style: nil})
		lines = append(lines, renderedLine{content: p.notice, style: &noticeStyle})
	}
	if p.err != nil {
		lines = append(lines, renderedLine{content: "", style: nil})
		lines = append(lines, renderedLine{content: p.err.Error(), style: &errorStyle})
	}

	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	maxWidthStyle := styles.StyleCache.MaxWidth(width)
	for y, line := range lines {
		if y >= height {
			break
		}
		s := line.style
		if s == nil {
			canvas.SetString(0, y, maxWidthStyle.Render(line.content), nil)
		} else {
			canvas.SetString(0, y, maxWidthStyle.Render(s.Render(line.content)), nil)
		}
	}
}

func (p *SessionPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *SessionPane) SetSessions(sessions []*agents.Session) {
	p.sessions = sessions
	if p.cursor >= len(p.sessions) {
		p.cursor = max(0, len(p.sessions)-1)
	}
}

func (p *SessionPane) selectedSession() (*agents.Session, bool) {
	if p.cursor < 0 || p.cursor >= len(p.sessions) {
		return nil, false
	}
	return p.sessions[p.cursor], true
}

type renderedLine struct {
	content string
	style   *lipgloss.Style
}

func shortenPath(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == "." || parent == string(filepath.Separator) || parent == "" {
		return base
	}
	return filepath.Join(parent, base)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

var (
	sectionStyle     = lipgloss.NewStyle().Bold(true).Underline(true)
	selectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#3a3a3a"))
	emptyStyle       = lipgloss.NewStyle().Faint(true)
	hintStyle        = lipgloss.NewStyle().Faint(true)
	noticeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#f0f0f0"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))
)
