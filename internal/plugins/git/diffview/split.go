package diffview

import (
	"slices"

	"github.com/aymanbagabas/go-udiff"
)

type splitHunk struct {
	fromLine int
	toLine   int
	lines    []*splitLine
}

type splitLine struct {
	before *udiff.Line
	after  *udiff.Line
}

func shift[T any](s []T) (T, []T, bool) {
	var zero T
	if len(s) == 0 {
		return zero, s, false
	}
	return s[0], s[1:], true
}

func deleteAt[T any](s []T, i int) (T, []T, bool) {
	var zero T
	if i < 0 || i >= len(s) {
		return zero, s, false
	}
	return s[i], append(s[:i], s[i+1:]...), true
}

func hunkToSplit(h *udiff.Hunk) (sh splitHunk) {
	lines := slices.Clone(h.Lines)
	sh = splitHunk{
		fromLine: h.FromLine,
		toLine:   h.ToLine,
		lines:    make([]*splitLine, 0, len(lines)),
	}

	for {
		var ul udiff.Line
		var ok bool
		ul, lines, ok = shift(lines)
		if !ok {
			break
		}

		var sl splitLine

		switch ul.Kind {
		// For equal lines, add as is
		case udiff.Equal:
			sl.before = &ul
			sl.after = &ul

		// For inserted lines, set after and keep before as nil
		case udiff.Insert:
			sl.before = nil
			sl.after = &ul

		// For deleted lines, set before and loop over the next lines
		// searching for the equivalent after line.
		case udiff.Delete:
			sl.before = &ul

		inner:
			for i, l := range lines {
				switch l.Kind {
				case udiff.Insert:
					var ll udiff.Line
					ll, lines, _ = deleteAt(lines, i)
					sl.after = &ll
					break inner
				case udiff.Equal:
					break inner
				}
			}
		}

		sh.lines = append(sh.lines, &sl)
	}

	return sh
}
