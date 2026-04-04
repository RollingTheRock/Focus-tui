package styles

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	TitleStyle  lipgloss.Style
	NormalStyle lipgloss.Style
	SubtleStyle lipgloss.Style
	AccentStyle lipgloss.Style
	BoxStyle    lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		TitleStyle:  lipgloss.NewStyle().Foreground(Accent).Bold(true),
		NormalStyle: lipgloss.NewStyle().Foreground(Text),
		SubtleStyle: lipgloss.NewStyle().Foreground(Subtle),
		AccentStyle: lipgloss.NewStyle().Foreground(Accent),
		BoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Accent).
			Padding(1, 2),
	}
}
