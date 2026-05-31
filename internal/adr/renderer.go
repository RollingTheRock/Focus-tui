package adr

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// RenderMarkdown parses markdown source and returns styled terminal lines
// suitable for display in a TUI. width is the available text width for wrapping.
func RenderMarkdown(source []byte, width int) []string {
	if width <= 0 {
		width = 80
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback: return source as plain text lines
		return strings.Split(string(source), "\n")
	}

	out, err := r.Render(string(source))
	if err != nil {
		return strings.Split(string(source), "\n")
	}

	// Split glamour output into lines, trimming trailing whitespace
	var lines []string
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimRight(ln, " \t\r")
		lines = append(lines, ln)
	}
	return lines
}
