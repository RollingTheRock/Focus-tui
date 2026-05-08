package adr

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// RenderMarkdown parses markdown source and returns styled terminal lines
// suitable for display in a TUI. width is the available text width for wrapping.
func RenderMarkdown(source []byte, width int) []string {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	r := &mdRenderer{
		source: source,
		width:  width,
	}

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		r.renderBlock(child)
	}

	return r.lines
}

type mdRenderer struct {
	source []byte
	width  int
	lines  []string
}

func (r *mdRenderer) renderBlock(node ast.Node) {
	switch n := node.(type) {
	case *ast.Heading:
		text := r.renderInline(node)
		switch n.Level {
		case 1:
			r.lines = append(r.lines, mdH1Style.Render(text))
		case 2:
			r.lines = append(r.lines, "")
			r.lines = append(r.lines, mdH2Style.Render("── "+text))
		default:
			r.lines = append(r.lines, "")
			r.lines = append(r.lines, mdH3Style.Render(text))
		}

	case *ast.Paragraph, *ast.TextBlock:
		text := r.renderInline(node)
		if text == "" {
			return
		}
		r.lines = append(r.lines, "")
		for _, ln := range r.wrapLine(text) {
			r.lines = append(r.lines, "  "+mdTextStyle.Render(ln))
		}

	case *ast.List:
		r.lines = append(r.lines, "")
		for li := n.FirstChild(); li != nil; li = li.NextSibling() {
			if item, ok := li.(*ast.ListItem); ok {
				text := r.renderInline(item)
				if text == "" {
					continue
				}
				for i, ln := range r.wrapLine(text) {
					if i == 0 {
						r.lines = append(r.lines, mdBulletStyle.Render(" •")+" "+mdTextStyle.Render(ln))
					} else {
						r.lines = append(r.lines, "   "+mdTextStyle.Render(ln))
					}
				}
			}
		}

	case *ast.FencedCodeBlock, *ast.CodeBlock:
		r.lines = append(r.lines, "")
		for i := 0; i < node.Lines().Len(); i++ {
			line := node.Lines().At(i)
			content := strings.TrimRight(string(line.Value(r.source)), "\n\r")
			r.lines = append(r.lines, mdCodeStyle.Render("  │ "+content))
		}

	case *ast.ThematicBreak:
		sep := mdMutedStyle.Render(strings.Repeat("─", r.width-4))
		r.lines = append(r.lines, "")
		r.lines = append(r.lines, "  "+sep)

	case *ast.Blockquote:
		r.lines = append(r.lines, "")
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			text := r.renderInline(child)
			if text == "" {
				continue
			}
			for _, ln := range r.wrapLine(text) {
				r.lines = append(r.lines, mdQuoteStyle.Render("▎ ")+mdTextStyle.Render(ln))
			}
		}

	default:
		if node.HasChildren() {
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				r.renderBlock(child)
			}
		}
	}
}

// renderInline collects plain text from inline children.
func (r *mdRenderer) renderInline(node ast.Node) string {
	var sb strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		r.writeText(child, &sb)
	}
	return strings.TrimSpace(sb.String())
}

func (r *mdRenderer) writeText(node ast.Node, sb *strings.Builder) {
	switch n := node.(type) {
	case *ast.Text:
		sb.Write(n.Segment.Value(r.source))
	case *ast.String:
		sb.Write(n.Value)
	case *ast.Emphasis:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			r.writeText(child, sb)
		}
	case *ast.Link:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			r.writeText(child, sb)
		}
	case *ast.CodeSpan:
		sb.WriteByte('`')
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			r.writeText(child, sb)
		}
		sb.WriteByte('`')
	case *ast.RawHTML, *ast.Image:
		// skip
	}
}

func (r *mdRenderer) wrapLine(text string) []string {
	if r.width <= 4 {
		return []string{text}
	}
	width := r.width - 6 // account for indent
	if width <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	var lines []string
	var current string
	for _, word := range words {
		candidate := current
		if candidate != "" {
			candidate += " "
		}
		candidate += word
		if lipgloss.Width(candidate) > width && current != "" {
			lines = append(lines, current)
			current = word
		} else {
			current = candidate
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = append(lines, text)
	}
	return lines
}

var (
	mdH1Style     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Padding(0, 1)
	mdH2Style     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#87CEEB"))
	mdH3Style     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B0C4DE"))
	mdCodeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B8B"))
	mdMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	mdTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D0D0D0"))
	mdBulletStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#87CEEB"))
	mdQuoteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6A5ACD"))
)
