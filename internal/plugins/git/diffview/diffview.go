package diffview

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/x/ansi"
)

const (
	leadingSymbolsSize = 2
	lineNumPadding     = 1
)

type file struct {
	path    string
	content string
}

type layout int

const (
	layoutUnified layout = iota + 1
	layoutSplit
)

// DiffView represents a view for displaying differences between two files.
type DiffView struct {
	layout          layout
	before          file
	after           file
	fileName        string
	contextLines    int
	lineNumbers     bool
	height          int
	width           int
	xOffset         int
	yOffset         int
	infiniteYScroll bool
	style           Style
	tabWidth        int

	isComputed bool
	err        error
	unified    udiff.UnifiedDiff
	edits      []udiff.Edit

	splitHunks []splitHunk

	totalLines      int
	codeWidth       int
	fullCodeWidth   int  // with leading symbols
	extraColOnAfter bool // add extra column on after panel
	beforeNumDigits int
	afterNumDigits  int

	chromaStyle *chroma.Style

	// Cache lexer to avoid expensive file pattern matching on every line
	cachedLexer chroma.Lexer
	// Cache highlighted lines to avoid re-highlighting the same content
	// Key: hash of (content + background color string)
	syntaxCache map[string]string
}

// New creates a new DiffView with default settings.
func New() *DiffView {
	dv := &DiffView{
		layout:       layoutUnified,
		contextLines: udiff.DefaultContextLines,
		lineNumbers:  true,
		tabWidth:     8,
		syntaxCache:  make(map[string]string),
	}
	dv.style = DefaultStyle()
	return dv
}

// Unified sets the layout of the DiffView to unified.
func (dv *DiffView) Unified() *DiffView {
	dv.layout = layoutUnified
	return dv
}

// Split sets the layout of the DiffView to split (side-by-side).
func (dv *DiffView) Split() *DiffView {
	dv.layout = layoutSplit
	return dv
}

// Before sets the "before" file for the DiffView.
func (dv *DiffView) Before(path, content string) *DiffView {
	dv.before = file{path: path, content: content}
	dv.clearCaches()
	return dv
}

// After sets the "after" file for the DiffView.
func (dv *DiffView) After(path, content string) *DiffView {
	dv.after = file{path: path, content: content}
	dv.clearCaches()
	return dv
}

// ChromaStyle sets the Chroma style for syntax highlighting.
// If nil, no syntax highlighting will be applied.
func (dv *DiffView) ChromaStyle(style *chroma.Style) *DiffView {
	dv.chromaStyle = style
	dv.clearSyntaxCache()
	return dv
}

// FileName sets the file name header to display above the diff.
func (dv *DiffView) FileName(name string) *DiffView {
	dv.fileName = name
	return dv
}

// ContextLines sets the number of context lines for the DiffView.
func (dv *DiffView) ContextLines(contextLines int) *DiffView {
	dv.contextLines = contextLines
	dv.isComputed = false
	return dv
}

// Style sets the style for the DiffView.
func (dv *DiffView) Style(style Style) *DiffView {
	dv.style = style
	return dv
}

// LineNumbers sets whether to display line numbers in the DiffView.
func (dv *DiffView) LineNumbers(lineNumbers bool) *DiffView {
	dv.lineNumbers = lineNumbers
	return dv
}

// Height sets the height of the DiffView.
func (dv *DiffView) Height(height int) *DiffView {
	dv.height = height
	return dv
}

// Width sets the width of the DiffView.
func (dv *DiffView) Width(width int) *DiffView {
	dv.width = width
	return dv
}

// XOffset sets the horizontal offset for the DiffView.
func (dv *DiffView) XOffset(xOffset int) *DiffView {
	dv.xOffset = xOffset
	return dv
}

// YOffset sets the vertical offset for the DiffView.
func (dv *DiffView) YOffset(yOffset int) *DiffView {
	dv.yOffset = yOffset
	return dv
}

// InfiniteYScroll allows the YOffset to scroll beyond the last line.
func (dv *DiffView) InfiniteYScroll(infiniteYScroll bool) *DiffView {
	dv.infiniteYScroll = infiniteYScroll
	return dv
}

// TabWidth sets the tab width. Only relevant for code that contains tabs.
func (dv *DiffView) TabWidth(tabWidth int) *DiffView {
	dv.tabWidth = tabWidth
	return dv
}

// String returns the string representation of the DiffView.
func (dv *DiffView) String() string {
	dv.normalizeLineEndings()
	dv.replaceTabs()
	if err := dv.computeDiff(); err != nil {
		return err.Error()
	}
	// Auto-infer fileName from before/after path if not explicitly set
	if dv.fileName == "" && dv.before.path != "" {
		dv.fileName = dv.before.path
	}
	dv.convertDiffToSplit()
	dv.adjustStyles()
	dv.detectNumDigits()
	dv.detectTotalLines()
	dv.preventInfiniteYScroll()

	if dv.width <= 0 {
		dv.detectCodeWidth()
	} else {
		dv.resizeCodeWidth()
	}

	style := lipgloss.NewStyle()
	if dv.width > 0 {
		style = style.MaxWidth(dv.width)
	}
	if dv.height > 0 {
		style = style.MaxHeight(dv.height)
	}

	switch dv.layout {
	case layoutUnified:
		return style.Render(strings.TrimSuffix(dv.renderUnified(), "\n"))
	case layoutSplit:
		return style.Render(strings.TrimSuffix(dv.renderSplit(), "\n"))
	default:
		panic("unknown diffview layout")
	}
}

// normalizeLineEndings ensures the file contents use Unix-style line endings.
func (dv *DiffView) normalizeLineEndings() {
	dv.before.content = strings.ReplaceAll(dv.before.content, "\r\n", "\n")
	dv.after.content = strings.ReplaceAll(dv.after.content, "\r\n", "\n")
}

// replaceTabs replaces tabs in the before and after file contents with spaces.
func (dv *DiffView) replaceTabs() {
	spaces := strings.Repeat(" ", dv.tabWidth)
	dv.before.content = strings.ReplaceAll(dv.before.content, "\t", spaces)
	dv.after.content = strings.ReplaceAll(dv.after.content, "\t", spaces)
}

// computeDiff computes the differences between the "before" and "after" files.
func (dv *DiffView) computeDiff() error {
	if dv.isComputed {
		return dv.err
	}
	dv.isComputed = true
	dv.edits = udiff.Lines(
		dv.before.content,
		dv.after.content,
	)
	dv.unified, dv.err = udiff.ToUnifiedDiff(
		dv.before.path,
		dv.after.path,
		dv.before.content,
		dv.edits,
		dv.contextLines,
	)
	return dv.err
}

// convertDiffToSplit converts the unified diff to a split diff if the layout is split.
func (dv *DiffView) convertDiffToSplit() {
	if dv.layout != layoutSplit {
		return
	}

	dv.splitHunks = make([]splitHunk, len(dv.unified.Hunks))
	for i, h := range dv.unified.Hunks {
		dv.splitHunks[i] = hunkToSplit(h)
	}
}

// adjustStyles adds padding and alignment to the styles.
func (dv *DiffView) adjustStyles() {
	setPadding := func(s lipgloss.Style) lipgloss.Style {
		return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
	}
	dv.style.MissingLine.LineNumber = setPadding(dv.style.MissingLine.LineNumber)
	dv.style.DividerLine.LineNumber = setPadding(dv.style.DividerLine.LineNumber)
	dv.style.EqualLine.LineNumber = setPadding(dv.style.EqualLine.LineNumber)
	dv.style.InsertLine.LineNumber = setPadding(dv.style.InsertLine.LineNumber)
	dv.style.DeleteLine.LineNumber = setPadding(dv.style.DeleteLine.LineNumber)
	dv.style.Filename.LineNumber = setPadding(dv.style.Filename.LineNumber)
}

// detectNumDigits calculates the maximum number of digits needed for line numbers.
func (dv *DiffView) detectNumDigits() {
	dv.beforeNumDigits = 0
	dv.afterNumDigits = 0

	for _, h := range dv.unified.Hunks {
		dv.beforeNumDigits = max(dv.beforeNumDigits, len(strconv.Itoa(h.FromLine+len(h.Lines))))
		dv.afterNumDigits = max(dv.afterNumDigits, len(strconv.Itoa(h.ToLine+len(h.Lines))))
	}
}

func (dv *DiffView) detectTotalLines() {
	dv.totalLines = 0

	if dv.fileName != "" {
		dv.totalLines++
	}

	switch dv.layout {
	case layoutUnified:
		for _, h := range dv.unified.Hunks {
			dv.totalLines += 1 + len(h.Lines)
		}
	case layoutSplit:
		for _, h := range dv.splitHunks {
			dv.totalLines += 1 + len(h.lines)
		}
	}
}

func (dv *DiffView) preventInfiniteYScroll() {
	if dv.infiniteYScroll {
		return
	}

	if dv.height > 0 {
		maxYOffset := max(0, dv.totalLines-dv.height)
		dv.yOffset = min(dv.yOffset, maxYOffset)
	} else {
		dv.yOffset = min(dv.yOffset, max(0, dv.totalLines-1))
	}
	dv.yOffset = max(0, dv.yOffset)
}

// detectCodeWidth calculates the maximum width of code lines in the diff view.
func (dv *DiffView) detectCodeWidth() {
	switch dv.layout {
	case layoutUnified:
		dv.detectUnifiedCodeWidth()
	case layoutSplit:
		dv.detectSplitCodeWidth()
	}
	dv.fullCodeWidth = dv.codeWidth + leadingSymbolsSize
}

func (dv *DiffView) detectUnifiedCodeWidth() {
	dv.codeWidth = 0

	for _, h := range dv.unified.Hunks {
		shownLines := ansi.StringWidth(dv.hunkLineFor(h))

		for _, l := range h.Lines {
			lineWidth := ansi.StringWidth(strings.TrimSuffix(l.Content, "\n")) + 1
			dv.codeWidth = max(dv.codeWidth, lineWidth, shownLines)
		}
	}
}

func (dv *DiffView) detectSplitCodeWidth() {
	dv.codeWidth = 0

	for i, h := range dv.splitHunks {
		shownLines := ansi.StringWidth(dv.hunkLineFor(dv.unified.Hunks[i]))

		for _, l := range h.lines {
			if l.before != nil {
				codeWidth := ansi.StringWidth(strings.TrimSuffix(l.before.Content, "\n")) + 1
				dv.codeWidth = max(dv.codeWidth, codeWidth, shownLines)
			}
			if l.after != nil {
				codeWidth := ansi.StringWidth(strings.TrimSuffix(l.after.Content, "\n")) + 1
				dv.codeWidth = max(dv.codeWidth, codeWidth, shownLines)
			}
		}
	}
}

// resizeCodeWidth resizes the code width to fit within the specified width.
func (dv *DiffView) resizeCodeWidth() {
	fullNumWidth := dv.beforeNumDigits + dv.afterNumDigits
	fullNumWidth += lineNumPadding * 4 // left and right padding for both line numbers

	switch dv.layout {
	case layoutUnified:
		dv.codeWidth = dv.width - fullNumWidth - leadingSymbolsSize
	case layoutSplit:
		remainingWidth := dv.width - fullNumWidth - leadingSymbolsSize*2
		dv.codeWidth = remainingWidth / 2
		dv.extraColOnAfter = (remainingWidth % 2) != 0
	}

	dv.fullCodeWidth = dv.codeWidth + leadingSymbolsSize
}

// renderUnified renders the unified diff view as a string.
func (dv *DiffView) renderUnified() string {
	var b strings.Builder

	fullContentStyle := lipgloss.NewStyle().MaxWidth(dv.fullCodeWidth)
	printedLines := -dv.yOffset
	shouldWrite := func() bool { return printedLines >= 0 }

	getContent := func(in string, ls LineStyle) (content string, leadingEllipsis bool) {
		content = strings.TrimSuffix(in, "\n")
		content = dv.hightlightCode(content, ls.Code.GetBackground())
		content = ansi.GraphemeWidth.Cut(content, dv.xOffset, len(content))
		content = ansi.Truncate(content, dv.codeWidth, "…")
		leadingEllipsis = dv.xOffset > 0 && strings.TrimSpace(content) != ""
		return content, leadingEllipsis
	}

outer:
	for i, h := range dv.unified.Hunks {
		// Render file name header before the first hunk.
		if i == 0 && dv.fileName != "" {
			if shouldWrite() {
				ls := dv.style.Filename
				if dv.lineNumbers {
					b.WriteString(ls.LineNumber.Render(pad("…", dv.beforeNumDigits)))
					b.WriteString(ls.LineNumber.Render(pad("…", dv.afterNumDigits)))
				}
				content := ansi.Truncate("  "+dv.fileName, dv.fullCodeWidth, "…")
				b.WriteString(ls.Code.Width(dv.fullCodeWidth).Render(content))
				b.WriteString("\n")
			}
			printedLines++
		}
		if shouldWrite() {
			ls := dv.style.DividerLine
			if dv.lineNumbers {
				b.WriteString(ls.LineNumber.Render(pad("…", dv.beforeNumDigits)))
				b.WriteString(ls.LineNumber.Render(pad("…", dv.afterNumDigits)))
			}
			content := ansi.Truncate(dv.hunkLineFor(h), dv.fullCodeWidth, "…")
			b.WriteString(ls.Code.Width(dv.fullCodeWidth).Render(content))
			b.WriteString("\n")
		}
		printedLines++

		beforeLine := h.FromLine
		afterLine := h.ToLine

		for j, l := range h.Lines {
			// Print ellipsis if we don't have enough space to print the rest of the diff
			hasReachedHeight := dv.height > 0 && printedLines+1 == dv.height
			isLastHunk := i+1 == len(dv.unified.Hunks)
			isLastLine := j+1 == len(h.Lines)
			if hasReachedHeight && (!isLastHunk || !isLastLine) {
				if shouldWrite() {
					ls := dv.lineStyleForType(l.Kind)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad("…", dv.beforeNumDigits)))
						b.WriteString(ls.LineNumber.Render(pad("…", dv.afterNumDigits)))
					}
					b.WriteString(fullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth).Render("  …"),
					))
					b.WriteRune('\n')
				}
				break outer
			}

			switch l.Kind {
			case udiff.Equal:
				if shouldWrite() {
					ls := dv.style.EqualLine
					content, leadingEllipsis := getContent(l.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
						b.WriteString(ls.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
					}
					b.WriteString(fullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth).Render(ternary(leadingEllipsis, " …", "  ") + content),
					))
				}
				beforeLine++
				afterLine++
			case udiff.Insert:
				if shouldWrite() {
					ls := dv.style.InsertLine
					content, leadingEllipsis := getContent(l.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
						b.WriteString(ls.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
					}
					b.WriteString(fullContentStyle.Render(
						ls.Symbol.Render(ternary(leadingEllipsis, "+…", "+ ")) +
							ls.Code.Width(dv.codeWidth).Render(content),
					))
				}
				afterLine++
			case udiff.Delete:
				if shouldWrite() {
					ls := dv.style.DeleteLine
					content, leadingEllipsis := getContent(l.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
						b.WriteString(ls.LineNumber.Render(pad(" ", dv.afterNumDigits)))
					}
					b.WriteString(fullContentStyle.Render(
						ls.Symbol.Render(ternary(leadingEllipsis, "-…", "- ")) +
							ls.Code.Width(dv.codeWidth).Render(content),
					))
				}
				beforeLine++
			}
			if shouldWrite() {
				b.WriteRune('\n')
			}

			printedLines++
		}
	}

	return b.String()
}

// renderSplit renders the split (side-by-side) diff view as a string.
func (dv *DiffView) renderSplit() string {
	var b strings.Builder

	beforeFullContentStyle := lipgloss.NewStyle().MaxWidth(dv.fullCodeWidth)
	afterFullContentStyle := lipgloss.NewStyle().MaxWidth(dv.fullCodeWidth + btoi(dv.extraColOnAfter))
	printedLines := -dv.yOffset
	shouldWrite := func() bool { return printedLines >= 0 }

	getContent := func(in string, ls LineStyle) (content string, leadingEllipsis bool) {
		content = strings.TrimSuffix(in, "\n")
		content = dv.hightlightCode(content, ls.Code.GetBackground())
		content = ansi.GraphemeWidth.Cut(content, dv.xOffset, len(content))
		content = ansi.Truncate(content, dv.codeWidth, "…")
		leadingEllipsis = dv.xOffset > 0 && strings.TrimSpace(content) != ""
		return content, leadingEllipsis
	}

outer:
	for i, h := range dv.splitHunks {
		// Render file name header before the first hunk.
		if i == 0 && dv.fileName != "" {
			if shouldWrite() {
				ls := dv.style.Filename
				if dv.lineNumbers {
					b.WriteString(ls.LineNumber.Render(pad("…", dv.beforeNumDigits)))
				}
				content := ansi.Truncate("  "+dv.fileName, dv.fullCodeWidth, "…")
				b.WriteString(ls.Code.Width(dv.fullCodeWidth).Render(content))
				if dv.lineNumbers {
					b.WriteString(ls.LineNumber.Render(pad("…", dv.afterNumDigits)))
				}
				b.WriteString(ls.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render(" "))
				b.WriteRune('\n')
			}
			printedLines++
		}
		if shouldWrite() {
			ls := dv.style.DividerLine
			if dv.lineNumbers {
				b.WriteString(ls.LineNumber.Render(pad("…", dv.beforeNumDigits)))
			}
			content := ansi.Truncate(dv.hunkLineFor(dv.unified.Hunks[i]), dv.fullCodeWidth, "…")
			b.WriteString(ls.Code.Width(dv.fullCodeWidth).Render(content))
			if dv.lineNumbers {
				b.WriteString(ls.LineNumber.Render(pad("…", dv.afterNumDigits)))
			}
			b.WriteString(ls.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render(" "))
			b.WriteRune('\n')
		}
		printedLines++

		beforeLine := h.fromLine
		afterLine := h.toLine

		for j, l := range h.lines {
			// Print ellipsis if we don't have enough space
			hasReachedHeight := dv.height > 0 && printedLines+1 == dv.height
			isLastHunk := i+1 == len(dv.unified.Hunks)
			isLastLine := j+1 == len(h.lines)
			if hasReachedHeight && (!isLastHunk || !isLastLine) {
				if shouldWrite() {
					ls := dv.style.MissingLine
					if l.before != nil {
						ls = dv.lineStyleForType(l.before.Kind)
					}
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad("…", dv.beforeNumDigits)))
					}
					b.WriteString(beforeFullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth).Render("  …"),
					))
					ls = dv.style.MissingLine
					if l.after != nil {
						ls = dv.lineStyleForType(l.after.Kind)
					}
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad("…", dv.afterNumDigits)))
					}
					b.WriteString(afterFullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth).Render("  …"),
					))
					b.WriteRune('\n')
				}
				break outer
			}

			switch {
			case l.before == nil:
				if shouldWrite() {
					ls := dv.style.MissingLine
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
					}
					b.WriteString(beforeFullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth).Render("  "),
					))
				}
			case l.before.Kind == udiff.Equal:
				if shouldWrite() {
					ls := dv.style.EqualLine
					content, leadingEllipsis := getContent(l.before.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
					}
					b.WriteString(beforeFullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth).Render(ternary(leadingEllipsis, " …", "  ")+content),
					))
				}
				beforeLine++
			case l.before.Kind == udiff.Delete:
				if shouldWrite() {
					ls := dv.style.DeleteLine
					content, leadingEllipsis := getContent(l.before.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
					}
					b.WriteString(beforeFullContentStyle.Render(
						ls.Symbol.Render(ternary(leadingEllipsis, "-…", "- ")) +
							ls.Code.Width(dv.codeWidth).Render(content),
					))
				}
				beforeLine++
			}

			switch {
			case l.after == nil:
				if shouldWrite() {
					ls := dv.style.MissingLine
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(" ", dv.afterNumDigits)))
					}
					b.WriteString(afterFullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render("  "),
					))
				}
			case l.after.Kind == udiff.Equal:
				if shouldWrite() {
					ls := dv.style.EqualLine
					content, leadingEllipsis := getContent(l.after.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
					}
					b.WriteString(afterFullContentStyle.Render(
						ls.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render(ternary(leadingEllipsis, " …", "  ")+content),
					))
				}
				afterLine++
			case l.after.Kind == udiff.Insert:
				if shouldWrite() {
					ls := dv.style.InsertLine
					content, leadingEllipsis := getContent(l.after.Content, ls)
					if dv.lineNumbers {
						b.WriteString(ls.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
					}
					b.WriteString(afterFullContentStyle.Render(
						ls.Symbol.Render(ternary(leadingEllipsis, "+…", "+ ")) +
							ls.Code.Width(dv.codeWidth+btoi(dv.extraColOnAfter)).Render(content),
					))
				}
				afterLine++
			}

			if shouldWrite() {
				b.WriteRune('\n')
			}

			printedLines++
		}
	}

	return b.String()
}

// hunkLineFor formats the header line for a hunk in the unified diff view.
func (dv *DiffView) hunkLineFor(h *udiff.Hunk) string {
	beforeShownLines, afterShownLines := dv.hunkShownLines(h)

	return fmt.Sprintf(
		"  @@ -%d,%d +%d,%d @@ ",
		h.FromLine,
		beforeShownLines,
		h.ToLine,
		afterShownLines,
	)
}

// hunkShownLines calculates the number of lines shown in a hunk for both versions.
func (dv *DiffView) hunkShownLines(h *udiff.Hunk) (before, after int) {
	for _, l := range h.Lines {
		switch l.Kind {
		case udiff.Equal:
			before++
			after++
		case udiff.Insert:
			after++
		case udiff.Delete:
			before++
		}
	}
	return before, after
}

func (dv *DiffView) lineStyleForType(t udiff.OpKind) LineStyle {
	switch t {
	case udiff.Equal:
		return dv.style.EqualLine
	case udiff.Insert:
		return dv.style.InsertLine
	case udiff.Delete:
		return dv.style.DeleteLine
	default:
		return dv.style.MissingLine
	}
}

// clearCaches clears all caches when content or major settings change.
func (dv *DiffView) clearCaches() {
	dv.cachedLexer = nil
	dv.clearSyntaxCache()
	dv.isComputed = false
}

// clearSyntaxCache clears the syntax highlighting cache.
func (dv *DiffView) clearSyntaxCache() {
	if dv.syntaxCache != nil {
		for k := range dv.syntaxCache {
			delete(dv.syntaxCache, k)
		}
	}
}

func (dv *DiffView) hightlightCode(source string, bgColor color.Color) string {
	if dv.chromaStyle == nil {
		return source
	}

	cacheKey := dv.createSyntaxCacheKey(source, bgColor)
	if cached, exists := dv.syntaxCache[cacheKey]; exists {
		return cached
	}

	l := dv.getChromaLexer()
	f := dv.getChromaFormatter(bgColor)

	it, err := l.Tokenise(nil, source)
	if err != nil {
		return source
	}

	var b strings.Builder
	if err := f.Format(&b, dv.chromaStyle, it); err != nil {
		return source
	}

	result := b.String()
	dv.syntaxCache[cacheKey] = result
	return result
}

func (dv *DiffView) createSyntaxCacheKey(source string, bgColor color.Color) string {
	r, g, b, a := bgColor.RGBA()
	return fmt.Sprintf("%d,%d,%d,%d|%s", r, g, b, a, source)
}

func (dv *DiffView) getChromaLexer() chroma.Lexer {
	if dv.cachedLexer != nil {
		return dv.cachedLexer
	}

	l := lexers.Match(dv.before.path)
	if l == nil {
		l = lexers.Analyse(dv.before.content)
	}
	if l == nil {
		l = lexers.Fallback
	}
	dv.cachedLexer = chroma.Coalesce(l)
	return dv.cachedLexer
}

func (dv *DiffView) getChromaFormatter(bgColor color.Color) chroma.Formatter {
	return chromaFormatter(bgColor, func(value string) string {
		value = strings.TrimRight(value, "\n")
		value = escapeControlChars(value)
		return value
	})
}
