package layout

import (
	"strings"

	"focus/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// Layout breakpoints.
const (
	TwoColMinWidth = 90
	CrampedHeight  = 15

	panelBorderV = 2 // top + bottom border of each panel
)

// Dimensions holds computed sizes for all layout zones.
type Dimensions struct {
	Width, Height int

	ContentH int // height available for panel content (inside borders)

	// Two-column mode.
	TwoCol bool
	LeftW  int // left panel content width (inside border+padding)
	RightW int // right panel content width (inside border+padding)

	// Single-column mode.
	TopH    int // todo panel height (single-col stacked)
	BottomH int // pomo panel height (single-col stacked)

	ShowQuote bool
}

// Compute calculates layout dimensions from terminal size.
// Layout (no outer frame):
//
//	header (1-2 lines)
//	panels (bordered, side-by-side or stacked)
//	footer (1 line)
//	help   (1 line)
func Compute(w, h int) Dimensions {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	d := Dimensions{
		Width:     w,
		Height:    h,
		ShowQuote: h >= CrampedHeight,
	}

	// Vertical space: header + footer + help + panel borders.
	headerH := 1
	if d.ShowQuote {
		headerH = 2
	}
	usedH := headerH + 1 + 1 + panelBorderV // header + footer + help + panel top/bottom border
	d.ContentH = h - usedH
	if d.ContentH < 3 {
		d.ContentH = 3
	}

	d.TwoCol = w >= TwoColMinWidth

	if d.TwoCol {
		// Each panel: border(1) + padding(1) + content + padding(1) + border(1) = content + 4
		// Two panels side by side: leftTotal + rightTotal = w
		// leftTotal = LeftW + 4, rightTotal = RightW + 4
		// LeftW + RightW = w - 8
		available := w - 8
		if available < 10 {
			available = 10
		}
		d.LeftW = int(float64(available) * 0.55)
		d.RightW = available - d.LeftW
	} else {
		// Single column: panel takes full width.
		// border(1) + padding(1) + content + padding(1) + border(1) = w
		d.LeftW = w - 4
		d.RightW = d.LeftW
		if d.LeftW < 10 {
			d.LeftW = 10
			d.RightW = 10
		}

		// Split content height between two stacked panels.
		// Each panel has its own panelBorderV, so we need extra vertical space.
		d.ContentH -= panelBorderV // account for the second panel's borders
		d.TopH = d.ContentH * 60 / 100
		d.BottomH = d.ContentH - d.TopH
		if d.TopH < 3 {
			d.TopH = 3
		}
		if d.BottomH < 3 {
			d.BottomH = 3
		}
	}

	return d
}

// RenderPanel renders content inside a bordered panel with a title in the top border.
// w is the content width (not including borders/padding). The total rendered width
// will be w + 4 (1 border + 1 padding + content + 1 padding + 1 border).
func RenderPanel(title, content string, w, h int, active bool) string {
	borderColor := styles.DimBorder
	if active {
		borderColor = styles.ActiveBorder
	}

	bc := lipgloss.NewStyle().Foreground(borderColor)
	tc := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)

	totalW := w + 4 // border + padding on each side

	// Build top border with embedded title.
	// ╭─ TITLE ──...──╮  total = totalW chars
	titleRendered := tc.Render(" " + title + " ")
	titleWidth := lipgloss.Width(titleRendered)

	topLeft := bc.Render("╭─")
	rightDashes := totalW - 3 - titleWidth // 3 = len("╭─") + len("╮")
	if rightDashes < 1 {
		rightDashes = 1
	}
	topRight := bc.Render(strings.Repeat("─", rightDashes) + "╮")
	topLine := topLeft + titleRendered + topRight

	// Pad content lines to fill height.
	contentLines := strings.Split(content, "\n")
	for len(contentLines) < h {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > h {
		contentLines = contentLines[:h]
	}

	// Render body with side borders + 1 char padding.
	leftBorder := bc.Render("│")
	rightBorder := bc.Render("│")

	var body strings.Builder
	for _, line := range contentLines {
		lineW := lipgloss.Width(line)
		pad := w - lineW
		if pad < 0 {
			pad = 0
		}
		body.WriteString(leftBorder + " " + line + strings.Repeat(" ", pad) + " " + rightBorder + "\n")
	}

	// Bottom border.
	bottomLine := bc.Render("╰" + strings.Repeat("─", totalW-2) + "╯")

	return topLine + "\n" + body.String() + bottomLine
}
