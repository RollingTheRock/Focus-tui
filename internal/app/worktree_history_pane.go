package app

import (
	"fmt"
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseWorktreeHistoryMsg closes the history overlay.
type CloseWorktreeHistoryMsg struct {
	ID models.PaneID
}

type worktreeHistoryPane struct {
	id       models.PaneID
	meta     models.PaneMeta
	common   models.CommonModel
	records  []models.WorktreeHistoryRecord
	cursor   int
	width    int
	height   int
	err      error
}

func newWorktreeHistoryPane(id models.PaneID, meta models.PaneMeta, common models.CommonModel) *worktreeHistoryPane {
	var records []models.WorktreeHistoryRecord
	if common.Store != nil {
		records, _ = common.Store.ListWorktreeHistory("")
	}
	return &worktreeHistoryPane{
		id:      id,
		meta:    meta,
		common:  common,
		records: records,
	}
}

func (p *worktreeHistoryPane) Init() tea.Cmd {
	return nil
}

func (p *worktreeHistoryPane) KeyBindings(compact bool) []models.KeyBinding {
	return []models.KeyBinding{
		{Keys: []string{"j", "k"}, Help: "nav"},
		{Keys: []string{"esc", "q"}, Help: "close"},
	}
}

func (p *worktreeHistoryPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "esc", "q":
			return p, func() tea.Msg {
				return CloseWorktreeHistoryMsg{ID: p.id}}
		case "j", "down":
			if p.cursor < len(p.records)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		}
	}
	return p, nil
}

func (p *worktreeHistoryPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 64
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent).Render("Worktree History"))
	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("Previously removed worktrees"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	if len(p.records) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("No archived worktrees yet."))
	} else {
		for i, r := range p.records {
			cursor := "  "
			if i == p.cursor {
				cursor = "▸ "
			}

			branch := r.Branch
			if branch == "" {
				branch = "unknown"
			}

			duration := ""
			if r.DurationMinutes > 0 {
				duration = fmt.Sprintf(" · %dm", r.DurationMinutes)
			}

			provider := ""
			if r.Provider != "" {
				provider = fmt.Sprintf(" · %s", r.Provider)
			}

			summary := r.Summary
			if summary == "" {
				summary = "no summary"
			}

			removedAt := r.RemovedAt.Format("Jan 2 15:04")

			nameStyle := lipgloss.NewStyle().Foreground(appstyles.Text)
			if i == p.cursor {
				nameStyle = nameStyle.Background(appstyles.Highlight)
			}

			line := fmt.Sprintf("%s%s  %s%s%s", cursor, nameStyle.Render(branch), removedAt, provider, duration)
			b.WriteString(appstyles.StyleCache.MaxWidth(width).Render(line))
			b.WriteByte('\n')
			b.WriteString(appstyles.StyleCache.MaxWidth(width).Render(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("  "+summary)))
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("↑↓ move · q/Esc close"))

	lines := strings.Split(b.String(), "\n")
	if p.height > 0 && len(lines) > p.height {
		lines = lines[:p.height]
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (p *worktreeHistoryPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}
