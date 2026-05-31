package styles

import (
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

// TextareaStyles returns a theme-colored textarea.Styles for Focused and
// Blurred states (no line numbers).
func TextareaStyles() textarea.Styles {
	return textarea.Styles{
		Focused: textarea.StyleState{
			Base:        lipgloss.NewStyle().Foreground(Text),
			CursorLine:  lipgloss.NewStyle(),
			Prompt:      lipgloss.NewStyle().Foreground(Accent),
			Placeholder: lipgloss.NewStyle().Foreground(Subtle),
			Text:        lipgloss.NewStyle().Foreground(Text),
		},
		Blurred: textarea.StyleState{
			Base:        lipgloss.NewStyle().Foreground(Text),
			CursorLine:  lipgloss.NewStyle(),
			Prompt:      lipgloss.NewStyle().Foreground(Subtle),
			Placeholder: lipgloss.NewStyle().Foreground(Subtle),
			Text:        lipgloss.NewStyle().Foreground(Text),
		},
	}
}

// TextareaEditorStyles returns a theme-colored textarea.Styles with line
// numbers and cursor-line highlighting (suitable for a code editor).
func TextareaEditorStyles() textarea.Styles {
	return textarea.Styles{
		Focused: textarea.StyleState{
			Base:             lipgloss.NewStyle().Foreground(Text),
			CursorLine:       lipgloss.NewStyle().Background(Highlight),
			LineNumber:       lipgloss.NewStyle().Foreground(Subtle),
			CursorLineNumber: lipgloss.NewStyle().Foreground(Accent),
			Prompt:           lipgloss.NewStyle().Foreground(Accent),
			Placeholder:      lipgloss.NewStyle().Foreground(Subtle),
			Text:             lipgloss.NewStyle().Foreground(Text),
		},
		Blurred: textarea.StyleState{
			Base:             lipgloss.NewStyle().Foreground(Text),
			CursorLine:       lipgloss.NewStyle().Background(Highlight),
			LineNumber:       lipgloss.NewStyle().Foreground(Subtle),
			CursorLineNumber: lipgloss.NewStyle().Foreground(Accent),
			Prompt:           lipgloss.NewStyle().Foreground(Subtle),
			Placeholder:      lipgloss.NewStyle().Foreground(Subtle),
			Text:             lipgloss.NewStyle().Foreground(Text),
		},
	}
}
