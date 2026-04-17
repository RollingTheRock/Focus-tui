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
		running, attention, recent := p.partitionSessions()
		if len(running) > 0 {
			lines = append(lines, renderedLine{content: "Running", style: &sectionStyle})
			for _, group := range p.groupSessionsByWorktree(running) {
				lines = append(lines, renderedLine{content: group.label, style: &hintStyle})
				for _, s := range group.sessions {
					line := p.renderSessionLine(s)
					style := (*lipgloss.Style)(nil)
					if p.sessionIndex(s.ID) == p.cursor {
						style = &selectedRowStyle
					}
					lines = append(lines, renderedLine{content: line, style: style})
				}
			}
		}
		if len(attention) > 0 {
			lines = append(lines, renderedLine{content: "", style: nil})
			lines = append(lines, renderedLine{content: "Attention", style: &sectionStyle})
			for _, group := range p.groupSessionsByWorktree(attention) {
				lines = append(lines, renderedLine{content: group.label, style: &hintStyle})
				for _, s := range group.sessions {
					line := p.renderSessionLine(s)
					style := (*lipgloss.Style)(nil)
					if p.sessionIndex(s.ID) == p.cursor {
						style = &selectedRowStyle
					}
					lines = append(lines, renderedLine{content: line, style: style})
				}
			}
		}
		if len(recent) > 0 {
			lines = append(lines, renderedLine{content: "", style: nil})
			lines = append(lines, renderedLine{content: "Recent", style: &sectionStyle})
			for _, group := range p.groupSessionsByWorktree(recent) {
				lines = append(lines, renderedLine{content: group.label, style: &hintStyle})
				for _, s := range group.sessions {
					line := p.renderSessionLine(s)
					style := (*lipgloss.Style)(nil)
					if p.sessionIndex(s.ID) == p.cursor {
						style = &selectedRowStyle
					}
					lines = append(lines, renderedLine{content: line, style: style})
				}
			}
		}
		if len(running) == 0 && len(recent) == 0 {
			lines = append(lines, renderedLine{content: "No running agents.", style: &emptyStyle})
		}
		/* old flat rendering retained below for reference
		for i, s := range p.sessions {
			state := string(s.State)
			if state == "" {
				state = string(agents.SessionUnknown)
			}
			line := fmt.Sprintf("%s  [%s]  %s", s.DisplayName(), state, shortenPath(s.WorktreeID))
			if s.PID > 0 {
				line += fmt.Sprintf("  pid:%d", s.PID)
			}
			if !s.StartedAt.IsZero() {
				line += "  " + formatDuration(time.Since(s.StartedAt))
			}
			style := (*lipgloss.Style)(nil)
			if i == p.cursor {
				style = &selectedRowStyle
			}
			lines = append(lines, renderedLine{content: line, style: style})
		}
		*/
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

func (p *SessionPane) partitionSessions() (running []*agents.Session, attention []*agents.Session, recent []*agents.Session) {
	for _, s := range p.sessions {
		if s == nil {
			continue
		}
		if s.State == agents.SessionRunning {
			running = append(running, s)
		} else if s.State == agents.SessionWaiting || s.State == agents.SessionFailed {
			attention = append(attention, s)
		} else {
			recent = append(recent, s)
		}
	}
	return running, attention, recent
}

func (p *SessionPane) renderSessionLine(s *agents.Session) string {
	state := string(s.State)
	if state == "" {
		state = string(agents.SessionUnknown)
	}
	line := fmt.Sprintf("%s  [%s]  %s", s.DisplayName(), state, shortenPath(s.WorktreeID))
	if s.BranchSnapshot != "" {
		line += "  " + s.BranchSnapshot
	}
	if s.PID > 0 {
		line += fmt.Sprintf("  pid:%d", s.PID)
	}
	if s.LaunchSource != "" {
		line += "  src:" + s.LaunchSource
	}
	if !s.StartedAt.IsZero() {
		line += "  " + formatDuration(time.Since(s.StartedAt))
	}
	if s.LastActivityAt != nil && !s.LastActivityAt.IsZero() {
		line += "  last:" + formatSince(*s.LastActivityAt)
	}
	if s.Summary != "" {
		line += "  — " + s.Summary
	}
	return line
}

func (p *SessionPane) sessionIndex(id string) int {
	for i, s := range p.sessions {
		if s != nil && s.ID == id {
			return i
		}
	}
	return -1
}

type sessionGroup struct {
	label    string
	sessions []*agents.Session
}

func (p *SessionPane) groupSessionsByWorktree(sessions []*agents.Session) []sessionGroup {
	groups := []sessionGroup{}
	seen := map[string]int{}
	for _, s := range sessions {
		label := shortenPath(s.WorktreeID)
		if idx, ok := seen[label]; ok {
			groups[idx].sessions = append(groups[idx].sessions, s)
			continue
		}
		seen[label] = len(groups)
		groups = append(groups, sessionGroup{label: label, sessions: []*agents.Session{s}})
	}
	return groups
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

func formatSince(ts time.Time) string {
	d := time.Since(ts)
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
