package app

import (
	"strings"

	gitmodel "focus/internal/git"
	"focus/internal/models"
	gitplugin "focus/internal/plugins/git"
	appstyles "focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseDeleteConfirmMsg closes the delete confirmation overlay.
type CloseDeleteConfirmMsg struct {
	ID models.PaneID
}

type deleteConfirmPane struct {
	id       models.PaneID
	worktree gitmodel.Worktree
	force    bool
	width    int
	height   int
}

func newDeleteConfirmPane(id models.PaneID, wt gitmodel.Worktree, force bool) *deleteConfirmPane {
	return &deleteConfirmPane{
		id:       id,
		worktree: wt,
		force:    force,
	}
}

func (p *deleteConfirmPane) Init() tea.Cmd {
	return nil
}

func (p *deleteConfirmPane) KeyBindings(compact bool) []models.KeyBinding {
	return []models.KeyBinding{
		{Keys: []string{"y"}, Help: "confirm"},
		{Keys: []string{"n", "esc"}, Help: "cancel"},
	}
}

func (p *deleteConfirmPane) ID() models.PaneID {
	return p.id
}

func (p *deleteConfirmPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "y", "Y":
			return p, func() tea.Msg {
				return gitplugin.RequestRemoveWorktreeMsg{
					Worktree: p.worktree,
					Force:    p.force,}
			}
		case "n", "N", "q", "esc", "ctrl+c":
			return p, func() tea.Msg {
				return CloseDeleteConfirmMsg{ID: p.id}
			}
		}
	}
	return p, nil
}

func (p *deleteConfirmPane) View() tea.View {
	width := p.width
	if width <= 0 {
		width = 64
	}

	name := p.worktree.DisplayName()

	var b strings.Builder

	title := "Delete Worktree?"
	if p.force {
		title = "Force Delete Worktree?"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(appstyles.Warning).Render(title))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString("Worktree: ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(name))
	b.WriteByte('\n')
	b.WriteString("Path:     ")
	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render(p.worktree.Path))
	b.WriteByte('\n')
	b.WriteByte('\n')

	if p.force {
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Overdue).Render("This will force-remove the worktree even if it has uncommitted changes."))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("This will permanently remove the worktree."))
	}
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(lipgloss.NewStyle().Foreground(appstyles.Subtle).Render("y confirm · n cancel"))

	return tea.NewView(appstyles.StyleCache.MaxWidth(width).Render(b.String()))
}

func (p *deleteConfirmPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}
