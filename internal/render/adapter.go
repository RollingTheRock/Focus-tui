package render

import (
	"focus/internal/models"
	"focus/internal/styles"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type StringAdapter struct {
	panel models.Panel
}

func NewStringAdapter(panel models.Panel) *StringAdapter {
	return &StringAdapter{panel: panel}
}

func (a *StringAdapter) Render(canvas Surface, width, height int) {
	a.panel.SetSize(width, height)
	content := a.panel.View()
	lines := splitLines(content)
	for y, line := range lines {
		if y >= height {
			break
		}
		canvas.SetString(0, y, line, nil)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

type PanelRenderer interface {
	models.Panel
	Renderer
}

func RenderPane(canvas Surface, title, content string, active bool) {
	w := canvas.Width()
	h := canvas.Height()
	if w < 4 || h < 3 {
		return
	}

	contentW := w - 4
	contentH := h - 2

	borderColor := styles.DimBorder
	if active {
		borderColor = styles.ActiveBorder
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)
	tc := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)

	renderTopBorder(canvas, title, w, bc, tc)

	lines := splitLines(content)
	for i := 0; i < contentH && i < len(lines); i++ {
		line := ansi.Truncate(lines[i], contentW, "")
		pad := contentW - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		canvas.SetString(0, i+1, "│", &bc)
		canvas.SetString(2, i+1, line, nil)
		canvas.SetString(2+lipgloss.Width(line), i+1, repeatSpace(pad), nil)
		canvas.SetString(w-1, i+1, "│", &bc)
	}

	for i := len(lines); i < contentH; i++ {
		canvas.SetString(0, i+1, "│", &bc)
		canvas.SetString(2, i+1, repeatSpace(contentW), nil)
		canvas.SetString(w-1, i+1, "│", &bc)
	}

	bottom := "└" + repeatString("─", w-2) + "┘"
	canvas.SetString(0, h-1, bottom, &bc)
}

func renderTopBorder(canvas Surface, title string, width int, bc, tc lipgloss.Style) {
	maxTitleWidth := width - 6
	if maxTitleWidth < 1 {
		maxTitleWidth = 1
	}
	if lipgloss.Width(title) > maxTitleWidth {
		title = ansi.Truncate(title, maxTitleWidth-1, "") + "…"
	}

	titleRendered := tc.Render(" " + title + " ")
	titleWidth := lipgloss.Width(titleRendered)

	left := "┌─"
	rightDashes := width - 3 - titleWidth
	if rightDashes < 1 {
		rightDashes = 1
	}
	right := repeatString("─", rightDashes) + "┐"

	canvas.SetString(0, 0, bc.Render(left), nil)
	canvas.SetString(2, 0, titleRendered, nil)
	canvas.SetString(2+titleWidth, 0, bc.Render(right), nil)
}

func repeatString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func repeatSpace(n int) string {
	return repeatString(" ", n)
}
