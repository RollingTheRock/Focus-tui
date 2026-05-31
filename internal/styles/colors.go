package styles

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	Accent    = compat.AdaptiveColor{Light: lipgloss.Color("#7C3AED"), Dark: lipgloss.Color("#A78BFA")}
	AccentDim = compat.AdaptiveColor{Light: lipgloss.Color("#A78BFA"), Dark: lipgloss.Color("#7C3AED")}
	Banner    = compat.AdaptiveColor{Light: lipgloss.Color("#7C3AED"), Dark: lipgloss.Color("#C4B5FD")}
	Done      = compat.AdaptiveColor{Light: lipgloss.Color("#6B7280"), Dark: lipgloss.Color("#6B7280")}
	Overdue   = compat.AdaptiveColor{Light: lipgloss.Color("#EF4444"), Dark: lipgloss.Color("#F87171")}
	Text      = compat.AdaptiveColor{Light: lipgloss.Color("#1F2937"), Dark: lipgloss.Color("#E5E7EB")}
	Subtle    = compat.AdaptiveColor{Light: lipgloss.Color("#9CA3AF"), Dark: lipgloss.Color("#6B7280")}
	Highlight = compat.AdaptiveColor{Light: lipgloss.Color("#F3F4F6"), Dark: lipgloss.Color("#374151")}
	Border    = compat.AdaptiveColor{Light: lipgloss.Color("#D1D5DB"), Dark: lipgloss.Color("#4B5563")}
	InfoLabel = compat.AdaptiveColor{Light: lipgloss.Color("#6B7280"), Dark: lipgloss.Color("#9CA3AF")}

	// Panel focus borders.
	ActiveBorder = compat.AdaptiveColor{Light: lipgloss.Color("#7C3AED"), Dark: lipgloss.Color("#A78BFA")}
	DimBorder    = compat.AdaptiveColor{Light: lipgloss.Color("#E5E7EB"), Dark: lipgloss.Color("#374151")}

	// Semantic colors.
	Success = compat.AdaptiveColor{Light: lipgloss.Color("#059669"), Dark: lipgloss.Color("#34D399")}
	Warning = compat.AdaptiveColor{Light: lipgloss.Color("#D97706"), Dark: lipgloss.Color("#FBBF24")}

	// Shared task state colors (used across DAG, worktree, and detail panes).
	StateActive  = compat.AdaptiveColor{Light: lipgloss.Color("#2563EB"), Dark: lipgloss.Color("#60A5FA")}
	StatePaused  = compat.AdaptiveColor{Light: lipgloss.Color("#D97706"), Dark: lipgloss.Color("#FBBF24")}
	StateBlocked = compat.AdaptiveColor{Light: lipgloss.Color("#DC2626"), Dark: lipgloss.Color("#F87171")}
	StateDone    = compat.AdaptiveColor{Light: lipgloss.Color("#059669"), Dark: lipgloss.Color("#34D399")}
	StateReady   = compat.AdaptiveColor{Light: lipgloss.Color("#0891B2"), Dark: lipgloss.Color("#22D3EE")}
	StateIdle    = compat.AdaptiveColor{Light: lipgloss.Color("#9CA3AF"), Dark: lipgloss.Color("#6B7280")}

	PriorityCritical = compat.AdaptiveColor{Light: lipgloss.Color("#DC2626"), Dark: lipgloss.Color("#F87171")}
	PriorityHigh     = compat.AdaptiveColor{Light: lipgloss.Color("#D97706"), Dark: lipgloss.Color("#FBBF24")}
	PriorityMedium   = compat.AdaptiveColor{Light: lipgloss.Color("#2563EB"), Dark: lipgloss.Color("#60A5FA")}
	PriorityLow      = compat.AdaptiveColor{Light: lipgloss.Color("#6B7280"), Dark: lipgloss.Color("#9CA3AF")}
)
