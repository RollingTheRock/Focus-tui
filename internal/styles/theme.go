package styles

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	TitleStyle     lipgloss.Style
	NormalStyle    lipgloss.Style
	SubtleStyle    lipgloss.Style
	AccentStyle    lipgloss.Style
	BoxStyle       lipgloss.Style
	BannerStyle    lipgloss.Style
	SeparatorStyle lipgloss.Style
	HelpStyle      lipgloss.Style
	InfoLabelStyle lipgloss.Style

	// Panel styles for focus system.
	ActivePanelStyle   lipgloss.Style
	InactivePanelStyle lipgloss.Style
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
		BannerStyle:    lipgloss.NewStyle().Foreground(Banner).Bold(true),
		SeparatorStyle: lipgloss.NewStyle().Foreground(Subtle),
		HelpStyle:      lipgloss.NewStyle().Foreground(Subtle),
		InfoLabelStyle: lipgloss.NewStyle().Foreground(InfoLabel),

		ActivePanelStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ActiveBorder).
			Padding(0, 1),
		InactivePanelStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DimBorder).
			Padding(0, 1),
	}
}
