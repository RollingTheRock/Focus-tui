package layout

import (
	"strings"

	"focus/internal/styles"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Layout breakpoints.
const (
	BannerMinWidth = 60 // below this, fall back to compact header
	CrampedHeight  = 15
	BannerHeight   = 6

	panelBorderV = 2 // top + bottom border of each panel
)

// Dimensions holds computed sizes for all layout zones.
type Dimensions struct {
	Width, Height int

	// Header.
	HeaderH   int  // 6 for banner, 1-2 for compact
	UseBanner bool // true when terminal is wide enough for FIGlet
	ShowQuote bool // show quote in compact mode or banner info column

	// Shell (full-width).
	ShellContentW int // content width inside borders
	ShellContentH int // content height inside borders

	// Overlay.
	OverlayW int // overlay content width (inside borders)
	OverlayH int // overlay content height (inside borders)
	OverlayX int // left offset for splicing onto base
	OverlayY int // top offset relative to shell panel start
}

// ComputeBanner calculates the layout for the banner + full-width shell design.
func ComputeBanner(w, h int) Dimensions {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	d := Dimensions{
		Width:  w,
		Height: h,
	}

	d.UseBanner = w >= BannerMinWidth
	d.ShowQuote = h >= CrampedHeight

	// Header height.
	if d.UseBanner {
		d.HeaderH = BannerHeight
	} else if d.ShowQuote {
		d.HeaderH = 2
	} else {
		d.HeaderH = 1
	}

	// Vertical: header + shellBorder(2) + footer(1) + help(1).
	usedH := d.HeaderH + panelBorderV + 1 + 1
	d.ShellContentH = h - usedH
	if d.ShellContentH < 3 {
		d.ShellContentH = 3
	}

	// Shell full width: border(1) + pad(1) + content + pad(1) + border(1) = w.
	d.ShellContentW = w - 4
	if d.ShellContentW < 10 {
		d.ShellContentW = 10
	}

	// Overlay: centered within the shell area.
	d.OverlayW = 50
	if d.OverlayW > w-8 {
		d.OverlayW = w - 8
	}
	if d.OverlayW < 20 {
		d.OverlayW = 20
	}
	d.OverlayH = d.ShellContentH - 2
	if d.OverlayH > 15 {
		d.OverlayH = 15
	}
	if d.OverlayH < 3 {
		d.OverlayH = 3
	}

	// Center overlay over the shell panel.
	overlayTotalW := d.OverlayW + 4 // with borders
	overlayTotalH := d.OverlayH + panelBorderV
	d.OverlayX = (w - overlayTotalW) / 2
	if d.OverlayX < 0 {
		d.OverlayX = 0
	}
	d.OverlayY = (d.ShellContentH + panelBorderV - overlayTotalH) / 2
	if d.OverlayY < 0 {
		d.OverlayY = 0
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
	maxTitleWidth := totalW - 6
	if maxTitleWidth < 1 {
		maxTitleWidth = 1
	}
	title = ansi.Truncate(title, maxTitleWidth, "…")

	// Build top border with embedded title.
	titleRendered := tc.Render(" " + title + " ")
	titleWidth := lipgloss.Width(titleRendered)

	topLeft := bc.Render("\u256d\u2500")
	rightDashes := totalW - 3 - titleWidth
	if rightDashes < 1 {
		rightDashes = 1
	}
	topRight := bc.Render(strings.Repeat("\u2500", rightDashes) + "\u256e")
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
	leftBorder := bc.Render("\u2502")
	rightBorder := bc.Render("\u2502")

	var body strings.Builder
	for _, line := range contentLines {
		lineW := lipgloss.Width(line)
		// Truncate lines that exceed the panel width to prevent layout overflow.
		if lineW > w {
			line = ansi.Truncate(line, w, "")
			lineW = w
		}
		pad := w - lineW
		body.WriteString(leftBorder + " " + line + strings.Repeat(" ", pad) + " " + rightBorder + "\n")
	}

	// Bottom border.
	bottomLine := bc.Render("\u2570" + strings.Repeat("\u2500", totalW-2) + "\u256f")

	return topLine + "\n" + body.String() + bottomLine
}

// OverlayOnBase composites an overlay string onto a base string at position (x, y).
// Both base and overlay are newline-separated rendered strings. The overlay
// replaces characters in the base at the given position. Uses ANSI-safe
// string truncation to preserve colors in the base.
func OverlayOnBase(base, overlay string, x, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, oLine := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		bLine := baseLines[row]
		oWidth := ansi.StringWidth(oLine)

		// Expand base line to ensure it's wide enough.
		bWidth := ansi.StringWidth(bLine)
		if bWidth < x+oWidth {
			bLine += strings.Repeat(" ", x+oWidth-bWidth)
		}

		// ANSI-safe splice: left part of base + overlay + right part of base.
		leftPart := ansi.Truncate(bLine, x, "")
		// Pad left part if it's shorter than x.
		leftW := ansi.StringWidth(leftPart)
		if leftW < x {
			leftPart += strings.Repeat(" ", x-leftW)
		}

		rightStart := x + oWidth
		rightPart := cutLeft(bLine, rightStart)

		baseLines[row] = leftPart + oLine + rightPart
	}

	return strings.Join(baseLines, "\n")
}

// cutLeft removes the first n visual columns from an ANSI string.
func cutLeft(s string, n int) string {
	w := ansi.StringWidth(s)
	if n >= w {
		return ""
	}
	return ansi.Cut(s, n, w)
}
