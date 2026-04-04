package styles

import "github.com/charmbracelet/lipgloss"

var (
	Accent    = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	AccentDim = lipgloss.AdaptiveColor{Light: "#A78BFA", Dark: "#7C3AED"}
	Banner    = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}
	Done      = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"}
	Overdue   = lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#F87171"}
	Text      = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"}
	Subtle    = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	Highlight = lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#374151"}
	Border    = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#4B5563"}
	InfoLabel = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}

	// Panel focus borders.
	ActiveBorder = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	DimBorder    = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}

	// Semantic colors.
	Success = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	Warning = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}
)
