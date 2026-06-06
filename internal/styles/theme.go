package styles

import "charm.land/lipgloss/v2"

type Theme struct {
	// --- Base Typography ---
	TitleStyle       lipgloss.Style // e.g. Bold + Accent
	DescriptionStyle lipgloss.Style // e.g. Regular + Subtle/Gray
	NoteStyle        lipgloss.Style // e.g. Dimmed + Italic
	NormalStyle      lipgloss.Style // Regular text
	AccentStyle      lipgloss.Style // Highlighted text
	SecondaryAccent  lipgloss.Style // Pink/Magenta for life & interaction

	// --- Selection & Focus (The Hub Soul) ---
	FocusedStyle      lipgloss.Style // Style for a focused item/field
	BlurredStyle      lipgloss.Style // Style for a blurred item/field
	SelectedIndicator lipgloss.Style // The vertical bar "|" or pointer "▸"
	ActiveIndicator   lipgloss.Style // Accent indicator for the active pane

	// --- Layout & Containers ---
	BoxStyle           lipgloss.Style // General pane container
	ActivePanelStyle   lipgloss.Style // Border for the active pane
	InactivePanelStyle lipgloss.Style // Border for inactive panes
	SeparatorStyle     lipgloss.Style // Vertical or horizontal dividers
	DarkroomStyle      lipgloss.Style // Extremely subtle text for backgrounds during overlay

	// --- Components ---
	BannerStyle    lipgloss.Style
	HelpStyle      lipgloss.Style
	InfoLabelStyle lipgloss.Style
	ErrorStyle     lipgloss.Style
	SuccessStyle   lipgloss.Style
}

func DefaultTheme() Theme {
	// Selection indicators
	focusedIndicator := lipgloss.NewStyle().Foreground(Accent).Bold(true).SetString("▎")
	activeIndicator := lipgloss.NewStyle().Foreground(Accent).SetString("▎")
	pink := lipgloss.Color("#f472b6") // Neon Pink

	return Theme{
		TitleStyle:       lipgloss.NewStyle().Foreground(Accent).Bold(true),
		DescriptionStyle: lipgloss.NewStyle().Foreground(Subtle),
		NoteStyle:        lipgloss.NewStyle().Foreground(Subtle).Italic(true),
		NormalStyle:      lipgloss.NewStyle().Foreground(Text),
		AccentStyle:      lipgloss.NewStyle().Foreground(Accent),
		SecondaryAccent:  lipgloss.NewStyle().Foreground(pink),

		FocusedStyle:      lipgloss.NewStyle().Foreground(Text).Bold(true),
		BlurredStyle:      lipgloss.NewStyle().Foreground(Subtle),
		SelectedIndicator: focusedIndicator,
		ActiveIndicator:   activeIndicator,

		BoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Accent).
			Padding(1, 2),
		ActivePanelStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ActiveBorder).
			Padding(0, 1),
		InactivePanelStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DimBorder).
			Padding(0, 1),
		SeparatorStyle:     lipgloss.NewStyle().Foreground(DimBorder),
		DarkroomStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")),

		BannerStyle:    lipgloss.NewStyle().Foreground(Banner).Bold(true),

		HelpStyle:      lipgloss.NewStyle().Foreground(Subtle),
		InfoLabelStyle: lipgloss.NewStyle().Foreground(InfoLabel),
		ErrorStyle:     lipgloss.NewStyle().Foreground(Overdue),
		SuccessStyle:   lipgloss.NewStyle().Foreground(Success),
	}
}
