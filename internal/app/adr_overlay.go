package app

import (
	"fmt"
	"os"
	"strings"

	"focus/internal/adr"
	"focus/internal/models"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type adrDetailOverlay struct {
	id       models.PaneID
	meta     models.PaneMeta
	common   models.CommonModel
	filePath string

	source      []byte // cached raw markdown
	lines       []string
	scroll      int
	searchHits  []int
	searchIdx   int
	searching   bool
	searchInput string

	width  int
	height int
}

func newADRDetailOverlay(id models.PaneID, meta models.PaneMeta, common models.CommonModel, filePath string) *adrDetailOverlay {
	return &adrDetailOverlay{
		id:       id,
		meta:     meta,
		common:   common,
		filePath: filePath,
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
		p.scroll = 0
		return p, nil

	case tea.KeyMsg:
		if p.searching {
			return p.handleSearchKey(msg)
		}
		return p.handleNormalKey(msg)
	}
	return p, nil
}

func (p *adrDetailOverlay) handleNormalKey(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "enter":
		return p, closeADRDetailCmd()

	case "j", "down":
		if p.scroll < len(p.lines)-1 {
			p.scroll++
		}

	case "k", "up":
		if p.scroll > 0 {
			p.scroll--
		}

	case "g":
		p.scroll = 0

	case "G":
		p.scroll = len(p.lines) - 1
		if p.scroll < 0 {
			p.scroll = 0
		}

	case "pgdown":
		p.scroll += p.height - 2
		if p.scroll >= len(p.lines) {
			p.scroll = len(p.lines) - 1
		}
		if p.scroll < 0 {
			p.scroll = 0
		}

	case "pgup":
		p.scroll -= p.height - 2
		if p.scroll < 0 {
			p.scroll = 0
		}

	case "home":
		p.scroll = 0

	case "end":
		p.scroll = len(p.lines) - 1
		if p.scroll < 0 {
			p.scroll = 0
		}

	case "ctrl+d":
		p.scroll += p.height / 2
		if p.scroll >= len(p.lines) {
			p.scroll = len(p.lines) - 1
		}
		if p.scroll < 0 {
			p.scroll = 0
		}

	case "ctrl+u":
		p.scroll -= p.height / 2
		if p.scroll < 0 {
			p.scroll = 0
		}

	case "/":
		p.searching = true
		p.searchInput = ""
		p.searchHits = nil
		p.searchIdx = 0
	}

	return p, nil
}

func (p *adrDetailOverlay) handleSearchKey(msg tea.KeyMsg) (models.Panel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.searching = false
		p.searchInput = ""
		p.searchHits = nil

	case "enter":
		p.searching = false
		if len(p.searchInput) > 0 {
			p.searchHits = p.findHits(p.searchInput)
			if len(p.searchHits) > 0 {
				p.searchIdx = 0
				p.scroll = p.searchHits[0]
			}
		}

	case "n":
		if len(p.searchHits) > 0 {
			p.searchIdx = (p.searchIdx + 1) % len(p.searchHits)
			p.scroll = p.searchHits[p.searchIdx]
		}

	case "N":
		if len(p.searchHits) > 0 {
			p.searchIdx--
			if p.searchIdx < 0 {
				p.searchIdx = len(p.searchHits) - 1
			}
			p.scroll = p.searchHits[p.searchIdx]
		}

	case "backspace":
		if len(p.searchInput) > 0 {
			p.searchInput = p.searchInput[:len(p.searchInput)-1]
		}

	default:
		if len(msg.Runes) > 0 {
			p.searchInput += string(msg.Runes)
		}
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
	}
}

func (p *adrDetailOverlay) View() string {
	w := p.width
	if w <= 0 {
		w = 80
	}
	h := p.height
	if h <= 0 {
		h = 20
	}

	header := adrOverlayTitleStyle.Render(" ADR Detail ") + adrHintStyle.Render("  [j/k]scroll  [/]search  [enter/esc/q]close")
	var content []string
	content = append(content, header)

	bodyH := h - 1
	if bodyH < 1 {
		bodyH = 1
	}

	if p.searching {
		searchPrompt := adrConstraintStyle.Render("/") + p.searchInput + adrDimStyle.Render("█")
		content = append(content, "  "+searchPrompt)
		bodyH--
	}

	if bodyH < 1 {
		return clampOverlayLines(content, h, w)
	}

	if len(p.lines) == 0 {
		content = append(content, adrMutedStyle.Render("  Loading..."))
		return clampOverlayLines(content, h, w)
	}

	// Calculate visible range
	start := p.scroll
	end := start + bodyH
	if end > len(p.lines) {
		end = len(p.lines)
		start = end - bodyH
		if start < 0 {
			start = 0
		}
	}

	if start > 0 {
		content = append(content, adrDimStyle.Render(fmt.Sprintf("  ↑ %d more lines above", start)))
	}

	for i := start; i < end; i++ {
		line := p.lines[i]
		// Highlight current search match
		if len(p.searchHits) > 0 && p.searchIdx >= 0 && p.searchIdx < len(p.searchHits) {
			if p.searchHits[p.searchIdx] == i {
				line = adrFocusedStyle.Background(lipgloss.Color("#444444")).Render(line)
			}
		}
		content = append(content, lipgloss.NewStyle().MaxWidth(w).Render(line))
	}

	if end < len(p.lines) {
		remaining := len(p.lines) - end
		content = append(content, adrDimStyle.Render(fmt.Sprintf("  ↓ %d more lines below", remaining)))
	}

	if len(p.lines) > 0 {
		pct := (p.scroll * 100) / len(p.lines)
		content = append(content, adrDimStyle.Render(fmt.Sprintf("  %d%%", pct)))
	}

	return clampOverlayLines(content, h, w)
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

func clampOverlayLines(lines []string, h, w int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(w).Render(line)
	}
	return strings.Join(lines, "\n")
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
