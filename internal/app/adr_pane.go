package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"focus/internal/adr"
	"focus/internal/models"
	"focus/internal/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type adrPane struct {
	id     models.PaneID
	meta   models.PaneMeta
	common models.CommonModel

	adrDir      string
	watchCancel context.CancelFunc
	watchCh     <-chan struct{}

	adrs        []models.ADRRecord
	constraints []models.ADRConstraintRecord
	cursor      int
	detailMode  bool

	width  int
	height int
}

func newAdrPane(id models.PaneID, meta models.PaneMeta, common *models.CommonModel, repoRoot string) *adrPane {
	adrDir := filepath.Join(repoRoot, "docs", "adr")
	if cfg := common.Cfg; cfg.AdrDir != "" {
		adrDir = cfg.AdrDir
	}
	return &adrPane{
		id:     id,
		meta:   meta,
		common: *common,
		adrDir: adrDir,
		cursor: -1,
	}
}

func (p *adrPane) Init() tea.Cmd {
	watchCh, cancel, err := adr.Watch(p.adrDir)
	if err != nil {
		return p.refreshCmd()
	}
	p.watchCh = watchCh
	p.watchCancel = cancel
	return tea.Batch(
		p.refreshCmd(),
		p.waitForChangeCmd(),
	)
}

func (p *adrPane) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		return adrRefreshMsg{}
	}
}

type adrRefreshMsg struct{}

func (p *adrPane) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case adrRefreshMsg:
		p.loadADRs()
		if p.watchCh != nil {
			return p, p.waitForChangeCmd()
		}
		return p, nil

	case adrLoadConstraintsMsg:
		p.loadConstraintsFor(msg.adrID)
		p.detailMode = !p.detailMode
		return p, nil

	case tea.KeyPressMsg:
		switch msg.Keystroke() {
		case "R":
			return p, p.refreshCmd()
		case "j", "down":
			p.moveCursor(1)
		case "k", "up":
			p.moveCursor(-1)
		case "e":
			return p, p.openDetailCmd()
		case "enter":
			return p, p.loadConstraintsCmd()}
	}
	return p, nil
}

func (p *adrPane) waitForChangeCmd() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-p.watchCh
		if !ok {
			return nil
		}
		return adrRefreshMsg{}
	}
}

func (p *adrPane) loadADRs() {
	recs, err := adr.LoadAll(p.adrDir)
	if err != nil {
		return
	}
	p.adrs = recs
	if p.cursor < 0 || p.cursor >= len(p.adrs) {
		p.cursor = 0
	}
	if p.cursor < len(p.adrs) {
		p.loadConstraintsFor(p.adrs[p.cursor].ID)
	}
}

func (p *adrPane) openDetailCmd() tea.Cmd {
	if p.cursor < 0 || p.cursor >= len(p.adrs) {
		return nil
	}
	fp := p.adrs[p.cursor].FilePath
	if fp == "" {
		return nil
	}
	return openADRDetailCmd(fp)
}

func (p *adrPane) loadConstraintsCmd() tea.Cmd {
	if p.cursor < 0 || p.cursor >= len(p.adrs) {
		return nil
	}
	adrID := p.adrs[p.cursor].ID
	return func() tea.Msg {
		return adrLoadConstraintsMsg{adrID: adrID}
	}
}

type adrLoadConstraintsMsg struct {
	adrID string
}

func (p *adrPane) loadConstraintsFor(adrID string) {
	constraints, err := adr.LoadConstraints(p.adrDir, adrID)
	if err != nil {
		p.constraints = nil
		return
	}
	p.constraints = constraints
}

func (p *adrPane) moveCursor(delta int) {
	if len(p.adrs) == 0 {
		return
	}
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.adrs) {
		p.cursor = len(p.adrs) - 1
	}
	if p.cursor < len(p.adrs) {
		p.loadConstraintsFor(p.adrs[p.cursor].ID)
	}
}

func (p *adrPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *adrPane) View() tea.View {
	w := p.width
	if w <= 0 {
		w = 80
	}
	h := p.height
	if h <= 0 {
		h = 10
	}

	var lines []string

	if p.detailMode && p.cursor >= 0 && p.cursor < len(p.adrs) {
		lines = append(lines, adrHeaderStyle.Render("  ADR Detail ")+adrHintStyle.Render("[enter]back  [T]asks [A]DRs"))
		lines = append(lines, p.renderDetail(p.adrs[p.cursor], h-1, w)...)
		return tea.NewView(clampLines(lines, h, w))
	}

	lines = append(lines, adrHeaderStyle.Render("  ADRs  ")+adrHintStyle.Render("[j/k]move  [enter]preview  [e]open  [T]asks [A]DRs  [R]refresh"))

	if len(p.adrs) == 0 {
		lines = append(lines, adrMutedStyle.Render("  No ADRs found."))
		return tea.NewView(clampLines(lines, h, w))
	}

	listH := h - 1
	if listH < 3 {
		listH = 3
	}
	previewH := 0
	if p.cursor >= 0 && p.cursor < len(p.adrs) && h > 6 {
		previewH = h - listH
		if previewH > 0 {
			listH = h - previewH
		}
	}

	for i, adr := range p.adrs {
		if i >= listH {
			break
		}
		line := p.renderAdrRow(adr, i == p.cursor)
		lines = append(lines, lipgloss.NewStyle().MaxWidth(w).Render(line))
	}

	if previewH > 1 && p.cursor >= 0 && p.cursor < len(p.adrs) {
		lines = append(lines, "")
		lines = append(lines, p.renderPreview(p.adrs[p.cursor], previewH, w)...)
	}

	return tea.NewView(clampLines(lines, h, w))
}

func (p *adrPane) renderAdrRow(adr models.ADRRecord, focused bool) string {
	statusChip := p.statusChip(adr.Status)
	shortID := adr.ID
	title := clipText(adr.Title, 40)
	verOrDate := adrDimStyle.Render(fmt.Sprintf("v%d", adr.Version))
	if adr.Date != "" {
		verOrDate = adrDimStyle.Render(adr.Date)
	}
	row := fmt.Sprintf("  %-8s %s  %s  %s", shortID, statusChip, title, verOrDate)
	if focused {
		return adrFocusedStyle.Render("▸ " + row + " ")
	}
	return adrRowStyle.Render("  " + row + " ")
}

func (p *adrPane) renderDetail(adr models.ADRRecord, maxH int, w int) []string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Highlight)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)

	var lines []string
	width := w - 4

	// Title
	lines = append(lines, titleStyle.Render("  "+adr.Title))

	// Metadata line
	meta := fmt.Sprintf("  %s | %s | v%d", adr.ID, adr.Status, adr.Version)
	if adr.Date != "" {
		meta += " | " + adr.Date
	}
	if adr.SupersededBy != nil && *adr.SupersededBy != "" {
		meta += " | supersedes " + *adr.SupersededBy
	}
	lines = append(lines, adrDimStyle.Render(meta))
	lines = append(lines, "")

	// Helper to add a section
	addSection := func(label, content string) {
		if content == "" {
			return
		}
		lines = append(lines, sectionStyle.Render("  "+label))
		for _, ln := range wrapText(content, width) {
			lines = append(lines, "  "+lipgloss.NewStyle().MaxWidth(width).Render(ln))
		}
		lines = append(lines, "")
	}

	addSection("Context", adr.Context)
	addSection("Decision", adr.Decision)
	addSection("Consequences", adr.Consequences)

	// Constraints
	if len(p.constraints) > 0 {
		lines = append(lines, sectionStyle.Render(fmt.Sprintf("  Constraints (%d)", len(p.constraints))))
		for i, c := range p.constraints {
			if i >= 12 {
				lines = append(lines, adrDimStyle.Render("    ..."))
				break
			}
			lines = append(lines, fmt.Sprintf("    [%s] %s",
				adrConstraintStyle.Render(c.Category), clipText(c.Rule, width-8)))
			if c.Rationale != "" {
				lines = append(lines, fmt.Sprintf("         %s",
					adrDimStyle.Render(clipText(c.Rationale, width-10))))
			}
		}
	}

	// Remove trailing blank
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > maxH {
		lines = lines[:maxH]
		lines[maxH-1] = adrDimStyle.Render("  ... (scroll with j/k)")
	}
	return lines
}

func (p *adrPane) statusChip(status string) string {
	switch status {
	case "accepted":
		return adrAcceptedStyle.Render("◆ Accepted ")
	case "proposed":
		return adrProposedStyle.Render("◇ Proposed ")
	case "superseded":
		return adrSupersededStyle.Render("◇ Superseded")
	case "deprecated":
		return adrDeprecatedStyle.Render("◇ Deprecated")
	default:
		return adrDimStyle.Render("  " + status)
	}
}

func (p *adrPane) renderPreview(adr models.ADRRecord, maxH int, w int) []string {
	sep := adrDimStyle.Render("  ----------- " + adr.ID + " Preview -----------")
	lines := []string{sep}

	lines = append(lines, fmt.Sprintf("  Status: %s  |  Version: %d",
		p.statusChip(adr.Status), adr.Version))
	if adr.SupersededBy != nil && *adr.SupersededBy != "" {
		lines = append(lines, fmt.Sprintf("  Superseded by: %s", *adr.SupersededBy))
	}

	ctxLabel := adrLabelStyle.Render("Context:")
	lines = append(lines, "  "+ctxLabel)
	for _, ln := range wrapText(adr.Context, w-4) {
		lines = append(lines, "    "+lipgloss.NewStyle().MaxWidth(w-4).Render(ln))
	}

	decLabel := adrLabelStyle.Render("Decision:")
	lines = append(lines, "  "+decLabel)
	for _, ln := range wrapText(adr.Decision, w-4) {
		lines = append(lines, "    "+lipgloss.NewStyle().MaxWidth(w-4).Render(ln))
	}

	conLabel := adrLabelStyle.Render("Consequences:")
	lines = append(lines, "  "+conLabel)
	for _, ln := range wrapText(adr.Consequences, w-4) {
		lines = append(lines, "    "+lipgloss.NewStyle().MaxWidth(w-4).Render(ln))
	}

	if len(p.constraints) > 0 {
		lines = append(lines, "  "+adrLabelStyle.Render(fmt.Sprintf("Constraints (%d):", len(p.constraints))))
		for i, c := range p.constraints {
			if i >= 8 {
				lines = append(lines, "    ...")
				break
			}
			lines = append(lines, fmt.Sprintf("    [%s] %s",
				adrConstraintStyle.Render(c.Category), clipText(c.Rule, w-10)))
		}
	}

	if len(lines) > maxH {
		lines = lines[:maxH]
	}
	return lines
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 40
	}
	words := strings.Fields(text)
	var lines []string
	var current string
	for _, word := range words {
		candidate := current
		if candidate != "" {
			candidate += " "
		}
		candidate += word
		if lipgloss.Width(candidate) > width && current != "" {
			lines = append(lines, current)
			current = word
		} else {
			current = candidate
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func clampLines(lines []string, h, w int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(w).Render(line)
	}
	return strings.Join(lines, "\n")
}

var (
	adrHeaderStyle     = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
	adrHintStyle       = lipgloss.NewStyle().Foreground(styles.Subtle)
	adrRowStyle        = lipgloss.NewStyle().Foreground(styles.Text)
	adrFocusedStyle    = lipgloss.NewStyle().Bold(true).Foreground(styles.Highlight).Background(lipgloss.Color("#333333"))
	adrMutedStyle      = lipgloss.NewStyle().Foreground(styles.Subtle)
	adrDimStyle        = lipgloss.NewStyle().Foreground(styles.Subtle)
	adrLabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(styles.Text)
	adrConstraintStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	adrAcceptedStyle   = lipgloss.NewStyle().Foreground(styles.Success).Bold(true)
	adrProposedStyle   = lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
	adrSupersededStyle = lipgloss.NewStyle().Foreground(styles.Subtle)
	adrDeprecatedStyle = lipgloss.NewStyle().Foreground(styles.Overdue).Bold(true)
)
