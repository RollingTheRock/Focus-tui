package diffview

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
)

// chromaFormatter returns a custom Chroma formatter that uses Lipgloss for
// foreground styling while keeping a forced background color.
func chromaFormatter(bgColor color.Color, processValue func(string) string) chroma.Formatter {
	return chroma.FormatterFunc(func(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
		for token := it(); token != chroma.EOF; token = it() {
			value := token.Value
			if processValue != nil {
				value = processValue(value)
			}

			entry := style.Get(token.Type)
			if entry.IsZero() {
				if _, err := fmt.Fprint(w, value); err != nil {
					return err
				}
				continue
			}

			s := lipgloss.NewStyle().
				Background(bgColor)

			if entry.Bold == chroma.Yes {
				s = s.Bold(true)
			}
			if entry.Underline == chroma.Yes {
				s = s.Underline(true)
			}
			if entry.Italic == chroma.Yes {
				s = s.Italic(true)
			}
			if entry.Colour.IsSet() {
				s = s.Foreground(lipgloss.Color(entry.Colour.String()))
			}

			if _, err := fmt.Fprint(w, s.Render(value)); err != nil {
				return err
			}
		}
		return nil
	})
}

// escapeControlChars replaces control characters with their Unicode Control
// Picture representations so they are displayed correctly in the terminal.
func escapeControlChars(content string) string {
	var sb strings.Builder
	sb.Grow(len(content))
	for _, r := range content {
		switch {
		case r >= 0 && r <= 0x1f:
			sb.WriteRune('\u2400' + r)
		case r == 0x7f: // DEL
			sb.WriteRune('\u2421')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
