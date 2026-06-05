package app

import (
	"strings"

	"focus/internal/models"
	appstyles "focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	bubblesKey "charm.land/bubbles/v2/key"
)

type CloseHelpOverlayMsg struct {
	ID models.PaneID
}

// Quick launch messages emitted from Help overlay.
type OpenGitFileTreeFromHelpMsg struct{}
type ToggleTodoFromHelpMsg struct{}
type OpenWeatherFromHelpMsg struct{}
type OpenAgentStoreFromHelpMsg struct{}

type helpOverlayPane struct {
	id           models.PaneID
	common       models.CommonModel
	width        int
	height       int
	scrollOffset int
	sections     []helpSection
	totalLines   int
}

type helpSection struct {
	title    string
	bindings []bubblesKey.Binding
}

func newHelpOverlayPane(id models.PaneID, common models.CommonModel, page *page) *helpOverlayPane {
	p := &helpOverlayPane{
		id:     id,
		common: common,
	}
	p.buildSections(page)
	return p
}

func (p *helpOverlayPane) buildSections(page *page) {
	var sections []helpSection

	// Quick Launch section.
	sections = append(sections, helpSection{
		title: "Quick Launch",
		bindings: []bubblesKey.Binding{
			bubblesKey.NewBinding(bubblesKey.WithKeys("g"), bubblesKey.WithHelp("g", "Open Git File Tree")),
			bubblesKey.NewBinding(bubblesKey.WithKeys("t"), bubblesKey.WithHelp("t", "Toggle Todo")),
			bubblesKey.NewBinding(bubblesKey.WithKeys("w"), bubblesKey.WithHelp("w", "Open Weather")),
			bubblesKey.NewBinding(bubblesKey.WithKeys("s"), bubblesKey.WithHelp("s", "Open Agent Store")),
		},
	})

	// Global bindings.
	sections = append(sections, helpSection{
		title:    "Global",
		bindings: globalHelpBindings,
	})

	// Collect bindings from all KeyBindingProvider panes.
	for _, id := range page.paneOrder {
		panel := page.panes[id]
		if kp, ok := panel.(models.KeyBindingProvider); ok {
			meta := page.paneMeta[id]
			mbs := kp.KeyBindings(false)
			if len(mbs) == 0 {
				continue
			}
			bbs := toBubblesBindings(mbs)
			sections = append(sections, helpSection{
				title:    meta.Name,
				bindings: bbs,
			})
		}
	}

	p.sections = sections
	p.totalLines = p.computeTotalLines()
}

func (p *helpOverlayPane) computeTotalLines() int {
	lines := 0
	for _, sec := range p.sections {
		lines += 2 // title + separator underline
		lines += len(sec.bindings)
		lines++ // blank line after section
	}
	return lines
}

func (p *helpOverlayPane) Init() tea.Cmd {
	return nil
}

func (p *helpOverlayPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "?", "esc", "q":
			return p, func() tea.Msg { return CloseHelpOverlayMsg{ID: p.id} }

		case "j", "down":
			p.scrollDown()
			return p, nil
		case "k", "up":
			p.scrollUp()
			return p, nil

		// Quick launch: close Help and open target panel.
		case "g":
			return p, func() tea.Msg { return CloseHelpOverlayMsg{ID: p.id} }
		case "t":
			return p, tea.Sequence(
				func() tea.Msg { return CloseHelpOverlayMsg{ID: p.id} },
				func() tea.Msg { return ToggleTodoFromHelpMsg{} },
			)
		case "w":
			return p, tea.Sequence(
				func() tea.Msg { return CloseHelpOverlayMsg{ID: p.id} },
				func() tea.Msg { return OpenWeatherFromHelpMsg{} },
			)
		case "s":
			return p, tea.Sequence(
				func() tea.Msg { return CloseHelpOverlayMsg{ID: p.id} },
				func() tea.Msg { return OpenAgentStoreFromHelpMsg{} },
			)

		// All other keys: just close Help.
		default:
			return p, func() tea.Msg { return CloseHelpOverlayMsg{ID: p.id} }
		}
	}
	return p, nil
}

func (p *helpOverlayPane) scrollDown() {
	visibleLines := p.height - 3
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := p.totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scrollOffset < maxScroll {
		p.scrollOffset++
	}
}

func (p *helpOverlayPane) scrollUp() {
	if p.scrollOffset > 0 {
		p.scrollOffset--
	}
}

func (p *helpOverlayPane) View() tea.View {
	w := p.width
	if w <= 0 {
		w = 80
	}
	h := p.height
	if h <= 0 {
		h = 24
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)
	sectionTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(appstyles.Highlight)
	keyStyle := lipgloss.NewStyle().Foreground(appstyles.Accent)
	descStyle := lipgloss.NewStyle().Foreground(appstyles.Text)
	subtleStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)
	hintStyle := lipgloss.NewStyle().Foreground(appstyles.Subtle)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Help"))
	b.WriteString(subtleStyle.Render("  (press ?/Esc/q to close, j/k to scroll)"))
	b.WriteByte('\n')
	b.WriteString(subtleStyle.Render(strings.Repeat("─", w-1)))
	b.WriteByte('\n')

	// Compute visible range.
	visibleLines := h - 3
	if visibleLines < 1 {
		visibleLines = 1
	}

	lineIdx := 0
	skipped := 0
	rendered := 0

	for _, sec := range p.sections {
		// Section title line.
		if lineIdx >= p.scrollOffset && rendered < visibleLines {
			b.WriteByte('\n')
			b.WriteString(sectionTitleStyle.Render(sec.title))
			b.WriteByte('\n')
			titleW := len(sec.title) + 2
			if titleW > w-1 {
				titleW = w - 1
			}
			b.WriteString(subtleStyle.Render(strings.Repeat("─", titleW)))
			b.WriteByte('\n')
			rendered++
		}
		lineIdx++

		// Separator line (under title).
		if lineIdx >= p.scrollOffset && rendered < visibleLines {
			rendered++
		}
		lineIdx++

		for _, binding := range sec.bindings {
			if lineIdx >= p.scrollOffset && rendered < visibleLines {
				h := binding.Help()
				keyStr := keyStyle.Render(h.Key)
				b.WriteString("  ")
				b.WriteString(keyStr)
				b.WriteString("  ")
				b.WriteString(descStyle.Render(h.Desc))
				b.WriteByte('\n')
				rendered++
			} else if lineIdx < p.scrollOffset {
				skipped++
			}
			lineIdx++
		}

		// Blank line between sections.
		if lineIdx >= p.scrollOffset && rendered < visibleLines {
			b.WriteByte('\n')
			rendered++
		}
		lineIdx++
	}

	// Footer hint.
	for rendered < visibleLines {
		b.WriteByte('\n')
		rendered++
	}
	footer := "Quick Launch: g/t/w/s open panels. Other keys close."
	if skipped > 0 {
		footer = "▲ scroll up for more  ·  " + footer
	}
	b.WriteString(hintStyle.Render(footer))

	return tea.NewView(lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(b.String()))
}

func (p *helpOverlayPane) KeyBindings(compact bool) []models.KeyBinding {
	if compact {
		return []models.KeyBinding{
			{Keys: []string{"j", "k"}, Help: "scroll"},
			{Keys: []string{"g", "t", "w", "s"}, Help: "launch"},
			{Keys: []string{"?", "esc", "q"}, Help: "close"},
		}
	}
	return []models.KeyBinding{
		{Keys: []string{"j", "k", "up", "down"}, Help: "scroll"},
		{Keys: []string{"g"}, Help: "Git File Tree"},
		{Keys: []string{"t"}, Help: "Toggle Todo"},
		{Keys: []string{"w"}, Help: "Weather"},
		{Keys: []string{"s"}, Help: "Agent Store"},
		{Keys: []string{"?", "esc", "q"}, Help: "close"},
	}
}

func (p *helpOverlayPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}
