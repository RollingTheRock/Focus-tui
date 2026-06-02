package diffview

import (
	"charm.land/lipgloss/v2"
	appstyles "focus/internal/styles"
)

// LineStyle defines the styles for a given line type in the diff view.
type LineStyle struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

// Style defines the overall style for the diff view.
type Style struct {
	DividerLine LineStyle
	MissingLine LineStyle
	EqualLine   LineStyle
	InsertLine  LineStyle
	DeleteLine  LineStyle
	Filename    LineStyle
}

// Focus-native diff backgrounds (pure-black base).
var (
	bgPureBlack   = lipgloss.Color("#000000") // base canvas
	bgEqualNum    = lipgloss.Color("#0A0A0A") // line-number strip, one step above pure black
	bgFilename    = lipgloss.Color("#2a1a42") // dark purple tint for file-name title bar, lifted for visibility
	bgInsertNum   = lipgloss.Color("#0d1f14") // dark green tint (slightly more vivid)
	bgInsertCode  = lipgloss.Color("#10251a") // one step lighter
	bgDeleteNum   = lipgloss.Color("#1f0d0d") // dark red tint (slightly more vivid)
	bgDeleteCode  = lipgloss.Color("#251111") // one step lighter
)

// DefaultStyle returns the Focus-native pure-black diff style.
// Hierarchy:
//   - Filename: most prominent (full-width dark-purple bar + Accent text)
//   - DividerLine (@@): subdued (pure-black + Subtle text)
//   - Equal/Insert/Delete: semantic tints on pure-black canvas
func DefaultStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(appstyles.Subtle).Background(bgPureBlack),
			Code: lipgloss.NewStyle().
				Foreground(appstyles.Subtle).Background(bgPureBlack),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(bgEqualNum),
			Code: lipgloss.NewStyle().
				Background(bgEqualNum),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(appstyles.Subtle).Background(bgEqualNum),
			Code: lipgloss.NewStyle().
				Foreground(appstyles.Text).Background(bgPureBlack),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(appstyles.Success).Background(bgInsertNum),
			Symbol: lipgloss.NewStyle().
				Foreground(appstyles.Success).Background(bgInsertCode),
			Code: lipgloss.NewStyle().
				Foreground(appstyles.Text).Background(bgInsertCode),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(appstyles.Overdue).Background(bgDeleteNum),
			Symbol: lipgloss.NewStyle().
				Foreground(appstyles.Overdue).Background(bgDeleteCode),
			Code: lipgloss.NewStyle().
				Foreground(appstyles.Text).Background(bgDeleteCode),
		},
		Filename: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(appstyles.Accent).Background(bgFilename).Bold(true).Underline(true),
			Code: lipgloss.NewStyle().
				Foreground(appstyles.Accent).Background(bgFilename).Bold(true).Underline(true),
		},
	}
}
