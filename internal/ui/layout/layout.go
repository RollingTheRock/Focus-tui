package layout

import (
	"math/rand"
	"strings"
	"time"

	"focus/internal/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Braille frames for the streaming gutter effect.
var brailleFrames = []string{"⡿", "⣟", "⣯", "⣷", "⣾", "⣽", "⣻", "⢿"}

// cipherChars used for the glitch/cipher effect.
const cipherChars = "0123456789abcdef!@#$%^&*()_+~|{}:?><"

// CipherText generates a random string of a fixed length for visual feedback.
func CipherText(length int) string {
	if length <= 0 {
		return ""
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = cipherChars[rand.Intn(len(cipherChars))]
	}
	return string(b)
}

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

	d.UseBanner = w >= BannerMinWidth && h >= 8
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

// RenderHubPanel renders a pane in the "Exquisite HUD" style:
// - Dynamic L-shaped gutter with Braille animation for active panes.
// - Symmetric HUD header: ━┫ TITLE ┣━━━━━━━━
// - Cipher status for running states: [ .f1)6_D! ]
func RenderHubPanel(theme styles.Theme, title, description, content string, w, h int, active, isRunning bool) string {
	var b strings.Builder

	// --- 1. State Configuration ---
	accentColor := theme.AccentStyle.GetForeground()
	dimColor := theme.SeparatorStyle.GetForeground()
	
	borderColor := dimColor
	titleStyle := theme.DescriptionStyle
	indicator := " "
	
	cornerChar := "┌"
	horizChar := "─"
	vertChar := "│"

	if active {
		borderColor = accentColor
		titleStyle = theme.TitleStyle
		indicator = theme.SelectedIndicator.String()
		if indicator == "" {
			indicator = "▎"
		}
		cornerChar = "┏"
		horizChar = "━"
		vertChar = "┃"
	}

	bc := lipgloss.NewStyle().Foreground(borderColor)
	
	// --- 2. Cipher & Metadata Processing ---
	descPart := ""
	if description != "" {
		text := description
		style := theme.NoteStyle
		if isRunning {
			// Glitch effect: replace status word with cipher text in Neon Pink
			text = "[" + CipherText(8) + "]"
			style = theme.SecondaryAccent.Bold(true)
		}
		descPart = " " + style.Render(text)
	}

	// --- 3. HUD Header Construction ---
	// ┏━━┫ TITLE ┣━━━━━━━━━━━━━━━━ status
	titleText := strings.ToUpper(title)
	titlePart := bc.Render("┫") + " " + indicator + " " + titleStyle.Render(titleText) + " " + bc.Render("┣")
	titleW := lipgloss.Width(titlePart)
	
	descW := lipgloss.Width(descPart)
	
	dashW := w + 4 - 2 - titleW - descW
	if dashW < 1 {
		dashW = 1
	}

	header := bc.Render(cornerChar+horizChar) + titlePart + bc.Render(strings.Repeat(horizChar, dashW)) + descPart
	b.WriteString(header + "\n")

	// --- 4. Body with Streaming Braille Gutter ---
	contentLines := strings.Split(content, "\n")
	bodyH := h - 1
	
	// Determine streaming gutter frame
	streamChar := vertChar
	gutterStyle := bc
	if active && isRunning {
		frameIdx := (time.Now().UnixNano() / int64(time.Millisecond*100)) % int64(len(brailleFrames))
		streamChar = brailleFrames[frameIdx]
		gutterStyle = theme.SecondaryAccent.Bold(true)
	}
	gutter := gutterStyle.Render(streamChar)

	for i := 0; i < bodyH; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		
		lineW := lipgloss.Width(line)
		if lineW > w + 2 {
			line = ansi.Truncate(line, w + 2, "…")
			lineW = w + 2
		}
		pad := w + 2 - lineW
		b.WriteString(gutter + " " + line + strings.Repeat(" ", pad) + "\n")
	}

	return b.String()
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
	var tc lipgloss.Style
	if active {
		tc = lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
	} else {
		tc = lipgloss.NewStyle().Foreground(styles.Subtle)
	}

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
