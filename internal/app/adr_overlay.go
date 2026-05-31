package app

import (
	"fmt"
	"os"
	"strings"

	"focus/internal/adr"
	"focus/internal/models"
	"focus/internal/styles"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type adrDetailOverlay struct {
	id       models.PaneID
	meta     models.PaneMeta
	common   models.CommonModel
	filePath string

	source      []byte // cached raw markdown
	lines       []string
	vp          viewport.Model
	searchHits  []int
	searchIdx   int
	searching   bool
	searchInput textinput.Model

	width  int
	height int
}

func newADRDetailOverlay(id models.PaneID, meta models.PaneMeta, common models.CommonModel, filePath string) *adrDetailOverlay {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "search..."
	ti.CharLimit = 120

	return &adrDetailOverlay{
		id:          id,
		meta:        meta,
		common:      common,
		filePath:    filePath,
		searchInput: ti,
		vp:          viewport.New(),
	}
}

func (p *adrDetailOverlay) Init() tea.Cmd {
	return p.loadCmd()
}

func (p *adrDetailOverlay) loadCmd() tea.Cmd {
	fp := p.filePath
	return func() tea.Msg {
		source, err := os.ReadFile(fp)
		if err != nil {
			return nil
		}
		// Cache source and render at default width
		return adrContentLoadedMsg{source: source, lines: adr.RenderMarkdown(source, 80)}
	}
}

type adrContentLoadedMsg struct {
	source []byte
	lines  []string
}

func (p *adrDetailOverlay) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case adrContentLoadedMsg:
		p.source = msg.source
		p.lines = msg.lines
		p.syncViewportSize()
		p.refreshViewportContent()
		return p, nil

	case tea.KeyPressMsg:
		if p.searching {
			panel, cmd := p.handleSearchKey(msg)
			p.syncViewportSize()
			return panel, cmd
		}
		return p.handleNormalKey(msg)
	}
	return p, nil
}

func (p *adrDetailOverlay) handleNormalKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "q", "esc", "enter":
		return p, closeADRDetailCmd()

	case "/":
		p.searching = true
		p.searchInput.SetValue("")
		p.searchInput.Focus()
		p.searchHits = nil
		p.searchIdx = 0
		p.syncViewportSize()
		return p, textinput.Blink

	case "n":
		if len(p.searchHits) > 0 {
			p.searchIdx = (p.searchIdx + 1) % len(p.searchHits)
			p.refreshViewportContent()
			p.vp.SetYOffset(p.searchHits[p.searchIdx])
		}
		return p, nil

	case "N":
		if len(p.searchHits) > 0 {
			p.searchIdx--
			if p.searchIdx < 0 {
				p.searchIdx = len(p.searchHits) - 1
			}
			p.refreshViewportContent()
			p.vp.SetYOffset(p.searchHits[p.searchIdx])
		}
		return p, nil

	case "g", "home":
		p.vp.GotoTop()
		return p, nil

	case "G", "end":
		p.vp.GotoBottom()
		return p, nil
	}

	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *adrDetailOverlay) handleSearchKey(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "esc":
		p.searching = false
		p.searchInput.SetValue("")
		p.searchInput.Blur()
		p.searchHits = nil
		p.searchIdx = 0
		p.refreshViewportContent()

	case "enter":
		p.searching = false
		p.searchInput.Blur()
		query := strings.TrimSpace(p.searchInput.Value())
		p.searchHits = nil
		p.searchIdx = 0
		if len(query) > 0 {
			p.searchHits = p.findHits(query)
			if len(p.searchHits) > 0 {
				p.searchIdx = 0
				p.vp.SetYOffset(p.searchHits[0])
			}
		}
		p.refreshViewportContent()

	case "n":
		if len(p.searchHits) > 0 {
			p.searchIdx = (p.searchIdx + 1) % len(p.searchHits)
			p.refreshViewportContent()
			p.vp.SetYOffset(p.searchHits[p.searchIdx])
		}

	case "N":
		if len(p.searchHits) > 0 {
			p.searchIdx--
			if p.searchIdx < 0 {
				p.searchIdx = len(p.searchHits) - 1
			}
			p.refreshViewportContent()
			p.vp.SetYOffset(p.searchHits[p.searchIdx])
		}

	default:
		var cmd tea.Cmd
		p.searchInput, cmd = p.searchInput.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p *adrDetailOverlay) findHits(query string) []int {
	if query == "" {
		return nil
	}
	lowerQuery := strings.ToLower(query)
	var hits []int
	for i, line := range p.lines {
		plain := stripANSISequences(line)
		if strings.Contains(strings.ToLower(plain), lowerQuery) {
			hits = append(hits, i)
		}
	}
	return hits
}

func (p *adrDetailOverlay) refreshViewportContent() {
	if len(p.lines) == 0 {
		p.vp.SetContent("")
		return
	}
	highlighted := make([]string, len(p.lines))
	for i, line := range p.lines {
		if len(p.searchHits) > 0 && p.searchIdx >= 0 && p.searchIdx < len(p.searchHits) && p.searchHits[p.searchIdx] == i {
			highlighted[i] = adrFocusedStyle.Background(lipgloss.Color("#444444")).Render(line)
		} else {
			highlighted[i] = line
		}
	}
	p.vp.SetContent(strings.Join(highlighted, "\n"))
}

func (p *adrDetailOverlay) syncViewportSize() {
	if p.width <= 0 || p.height <= 0 {
		return
	}
	p.vp.SetWidth(p.width)
	h := p.height - 2 // header + percentage indicator
	if p.searching {
		h--
	}
	if h < 1 {
		h = 1
	}
	p.vp.SetHeight(h)
}

func (p *adrDetailOverlay) SetSize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	prevWidth := p.width
	p.width = width
	p.height = height

	// Re-render at new width if it changed and we have cached source
	if width != prevWidth && len(p.source) > 0 {
		p.lines = adr.RenderMarkdown(p.source, width)
		p.refreshViewportContent()
	}

	p.syncViewportSize()

	inputWidth := width - 6
	if inputWidth < 20 {
		inputWidth = 20
	}
	p.searchInput.SetWidth(inputWidth)
}

func (p *adrDetailOverlay) View() tea.View {
	w := p.width
	if w <= 0 {
		w = 80
	}
	h := p.height
	if h <= 0 {
		h = 20
	}

	header := adrOverlayTitleStyle.Render(" ADR Detail ") + adrHintStyle.Render("  [j/k]scroll  [/]search  [enter/esc/q]close")
	var b strings.Builder
	b.WriteString(header)

	if p.searching {
		b.WriteByte('\n')
		b.WriteString(lipgloss.NewStyle().MaxWidth(w).Render("  " + p.searchInput.View()))
	}

	if len(p.lines) == 0 {
		b.WriteByte('\n')
		b.WriteString(lipgloss.NewStyle().MaxWidth(w).Render(adrMutedStyle.Render("  Loading...")))
		return tea.NewView(b.String())
	}

	b.WriteByte('\n')
	b.WriteString(p.vp.View())

	b.WriteByte('\n')
	pct := int(p.vp.ScrollPercent() * 100)
	b.WriteString(lipgloss.NewStyle().MaxWidth(w).Render(adrDimStyle.Render(fmt.Sprintf("  %d%%", pct))))

	return tea.NewView(b.String())
}

func stripANSISequences(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

type closeADRDetailMsg struct{}

func closeADRDetailCmd() tea.Cmd {
	return func() tea.Msg {
		return closeADRDetailMsg{}
	}
}

type openADRDetailMsg struct {
	FilePath string
}

func openADRDetailCmd(filePath string) tea.Cmd {
	return func() tea.Msg {
		return openADRDetailMsg{FilePath: filePath}
	}
}

var (
	adrOverlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
)
