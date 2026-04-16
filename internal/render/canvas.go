package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Cell struct {
	Content string
	Style   lipgloss.Style
}

type Canvas struct {
	width  int
	height int
	lines  []string
}

func NewCanvas(width, height int) *Canvas {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &Canvas{
		width:  width,
		height: height,
		lines:  make([]string, height),
	}
}

func (c *Canvas) Width() int {
	return c.width
}

func (c *Canvas) Height() int {
	return c.height
}

func (c *Canvas) SetCell(x, y int, ch rune, style *lipgloss.Style) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	content := string(ch)
	if style != nil {
		content = style.Render(content)
	}
	c.setCellContent(x, y, content)
}

func (c *Canvas) SetString(x, y int, s string, style *lipgloss.Style) {
	if y < 0 || y >= c.height {
		return
	}
	if x >= c.width {
		return
	}
	if x < 0 {
		if -x >= len(s) {
			return
		}
		s = s[-x:]
		x = 0
	}
	if x+len(s) > c.width {
		s = s[:c.width-x]
	}
	if style != nil {
		s = style.Render(s)
	}
	c.setCellContent(x, y, s)
}

func (c *Canvas) setCellContent(x, y int, content string) {
	line := c.lines[y]
	if len(line) < x {
		line += strings.Repeat(" ", x-len(line))
	}
	before := line[:x]
	after := ""
	if x+len(content) < len(line) {
		after = line[x+len(content):]
	}
	c.lines[y] = before + content + after
}

func (c *Canvas) Clear() {
	emptyLine := strings.Repeat(" ", c.width)
	for i := range c.lines {
		c.lines[i] = emptyLine
	}
}

func (c *Canvas) Render() string {
	return strings.Join(c.lines, "\n")
}

func (c *Canvas) SubCanvas(x, y, w, h int) *SubCanvas {
	return &SubCanvas{
		parent:  c,
		offsetX: x,
		offsetY: y,
		width:   w,
		height:  h,
	}
}

type SubCanvas struct {
	parent  *Canvas
	offsetX int
	offsetY int
	width   int
	height  int
}

func (s *SubCanvas) Width() int {
	return s.width
}

func (s *SubCanvas) Height() int {
	return s.height
}

func (s *SubCanvas) SetCell(x, y int, ch rune, style *lipgloss.Style) {
	if x < 0 || x >= s.width || y < 0 || y >= s.height {
		return
	}
	s.parent.SetCell(s.offsetX+x, s.offsetY+y, ch, style)
}

func (s *SubCanvas) SetString(x, y int, str string, style *lipgloss.Style) {
	if y < 0 || y >= s.height {
		return
	}
	if x >= s.width {
		return
	}
	if x < 0 {
		if -x >= len(str) {
			return
		}
		str = str[-x:]
		x = 0
	}
	if x+len(str) > s.width {
		str = str[:s.width-x]
	}
	s.parent.SetString(s.offsetX+x, s.offsetY+y, str, style)
}

func (s *SubCanvas) SubCanvas(x, y, w, h int) *SubCanvas {
	return &SubCanvas{
		parent:  s.parent,
		offsetX: s.offsetX + x,
		offsetY: s.offsetY + y,
		width:   w,
		height:  h,
	}
}

type Renderer interface {
	Render(canvas Surface, width, height int)
}

type Surface interface {
	Width() int
	Height() int
	SetCell(x, y int, ch rune, style *lipgloss.Style)
	SetString(x, y int, s string, style *lipgloss.Style)
	SubCanvas(x, y, w, h int) *SubCanvas
}

var _ Surface = (*Canvas)(nil)
var _ Surface = (*SubCanvas)(nil)
