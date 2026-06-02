package git

import (
	appstyles "focus/internal/styles"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	branchStyle = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)

	upstreamStyle = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	aheadStyle    = lipgloss.NewStyle().Foreground(appstyles.Success)
	behindStyle   = lipgloss.NewStyle().Foreground(appstyles.Warning)

	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Text)
	pathStyle    = lipgloss.NewStyle().Foreground(appstyles.Text)
	emptyStyle   = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	errorStyle   = lipgloss.NewStyle().Foreground(appstyles.Overdue)

	diffHeaderStyle  = lipgloss.NewStyle().Foreground(appstyles.Subtle).Background(lipgloss.Color("#000000"))
	diffFileStyle    = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Text).Background(appstyles.Highlight).Padding(0, 1)
	hunkHeaderStyle  = lipgloss.NewStyle().Foreground(appstyles.Accent).Background(appstyles.Highlight).Padding(0, 1)
	binaryMetaStyle  = lipgloss.NewStyle().Foreground(appstyles.Warning).Bold(true)
	addedLineStyle   = lipgloss.NewStyle().Foreground(appstyles.Success)
	removedLineStyle = lipgloss.NewStyle().Foreground(appstyles.Overdue)

	commitHeaderStyle      = lipgloss.NewStyle().Bold(true).Foreground(appstyles.Accent)
	commitFileStyle        = lipgloss.NewStyle().Foreground(appstyles.Text)
	commitInputStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(appstyles.Highlight).Padding(0, 1)
	commitHintStyle        = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	commitHintWarningStyle = lipgloss.NewStyle().Foreground(appstyles.Warning)

	selectedRowStyle = lipgloss.NewStyle().
				Background(compat.AdaptiveColor{Light: lipgloss.Color("#E8F0FE"), Dark: lipgloss.Color("#2B364B")}).
				Foreground(appstyles.Text).
				Bold(true)
	selectedSecondaryRowStyle = lipgloss.NewStyle().
					Background(compat.AdaptiveColor{Light: lipgloss.Color("#E8F0FE"), Dark: lipgloss.Color("#2B364B")}).
					Foreground(appstyles.Subtle)
	rowRailStyle          = lipgloss.NewStyle().Foreground(appstyles.Accent)
	rowRailSecondaryStyle = lipgloss.NewStyle().Foreground(appstyles.AccentDim)
	rowStripeRailStyle    = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	loadingStyle          = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	cleanupHintStyle      = lipgloss.NewStyle().Foreground(appstyles.Subtle)

	// Task state indicators — colors match DAG node state colors
	taskStateActiveStyle  = lipgloss.NewStyle().Bold(true).Foreground(appstyles.StateActive)
	taskStatePausedStyle  = lipgloss.NewStyle().Bold(true).Foreground(appstyles.StatePaused)
	taskStateBlockedStyle = lipgloss.NewStyle().Bold(true).Foreground(appstyles.StateBlocked)
	taskStateDoneStyle    = lipgloss.NewStyle().Bold(true).Foreground(appstyles.StateDone)
	taskStateNoneStyle    = lipgloss.NewStyle().Foreground(appstyles.StateIdle)

	modifiedIconStyle   = lipgloss.NewStyle().Foreground(appstyles.Warning)
	addedIconStyle      = lipgloss.NewStyle().Foreground(appstyles.Success)
	deletedIconStyle    = lipgloss.NewStyle().Foreground(appstyles.Overdue)
	renamedIconStyle    = lipgloss.NewStyle().Foreground(appstyles.Accent)
	untrackedIconStyle  = lipgloss.NewStyle().Foreground(appstyles.Subtle)
	conflictedIconStyle = lipgloss.NewStyle().Foreground(appstyles.Overdue).Bold(true)
)

func renderStatusIcon(icon string) string {
	switch icon {
	case "M":
		return modifiedIconStyle.Render(icon)
	case "A":
		return addedIconStyle.Render(icon)
	case "D":
		return deletedIconStyle.Render(icon)
	case "R":
		return renamedIconStyle.Render(icon)
	case "?":
		return untrackedIconStyle.Render(icon)
	case "!":
		return conflictedIconStyle.Render(icon)
	default:
		return emptyStyle.Render(icon)
	}
}


