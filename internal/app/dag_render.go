package app

import (
	"sort"
	"strings"

	"focus/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// ────────────────────────────────────────────────────────────
// Horizontal DAG renderer (left-to-right)
//
//   Levels → columns       Nodes → stacked vertically within columns
//   Edges  → box-drawing chars in "edge gutter" columns
//   Focus  → upstream/downstream edges highlighted in accent color
// ────────────────────────────────────────────────────────────

const (
	minColW    = 12
	minRows    = 6
	leftMargin = 1
	rowSpacing = 1 // rows between nodes in same column
)

// ── layout ──────────────────────────────────────────────────

type hLayout struct {
	colX         []int          // pixel x of each node column
	colW         []int          // width of each node column
	edgeX        []int          // pixel x of each edge gutter start (len = numCols-1)
	edgeW        []int          // width of each edge gutter (len = numCols-1)
	nodeRow      map[string]int // y position of each node
	nodeCol      map[string]int // column index of each node
	gutterSlot   map[string]int // edge key → assigned slot index within source gutter
	skipChannels map[string]int // edge key → channel Y for skip-level edges
	rows         int
	cols         int
}

func buildLayout(
	nodes map[string]dagNode,
	edges []dagEdge,
	levels map[string]int,
	layerIDs map[int][]string,
	maxLevel int,
	maxW int,
	maxH int,
) *hLayout {
	l := &hLayout{
		nodeRow:      map[string]int{},
		nodeCol:      map[string]int{},
		gutterSlot:   map[string]int{},
		skipChannels: map[string]int{},
	}

	nCols := maxLevel + 1

	// measure column widths from compact labels
	l.colW = make([]int, nCols)
	for lv := 0; lv <= maxLevel; lv++ {
		w := minColW
		for _, id := range layerIDs[lv] {
			n := nodes[id]
			lw := len(chipLabel(n)) + 4 // [ title ] = title + 2 brackets + 2 spaces
			if lw > w {
				w = lw
			}
		}
		l.colW[lv] = w
	}

	// Compute gutter widths: each gutter needs at least 2 chars,
	// plus one slot per unique source node that has outgoing edges through it.
	l.edgeW = make([]int, nCols-1)
	for lv := 0; lv < nCols-1; lv++ {
		srcSet := map[string]bool{}
		for _, e := range edges {
			srcLv := levels[e.From]
			dstLv := levels[e.To]
			if srcLv == lv && dstLv == lv+1 {
				srcSet[e.From] = true
			}
		}
		w := 2
		if nSrc := len(srcSet); nSrc > 1 {
			w = nSrc
		}
		l.edgeW[lv] = w
	}

	// Scale down columns proportionally to fit available width
	total := leftMargin
	for _, w := range l.colW {
		total += w
	}
	for _, w := range l.edgeW {
		total += w
	}

	if total > maxW && maxW > 40 {
		available := maxW - leftMargin
		for i := range l.edgeW {
			available -= l.edgeW[i]
		}
		if available < nCols*8 {
			available = nCols * 8
		}
		totalColW := 0
		for _, w := range l.colW {
			totalColW += w
		}
		if totalColW > 0 {
			scale := float64(available) / float64(totalColW)
			for i := range l.colW {
				l.colW[i] = int(float64(l.colW[i]) * scale)
				if l.colW[i] < 8 {
					l.colW[i] = 8
				}
			}
		}
	}

	// x offsets
	l.colX = make([]int, nCols)
	l.edgeX = make([]int, nCols-1)
	x := leftMargin
	for lv := 0; lv <= maxLevel; lv++ {
		l.colX[lv] = x
		x += l.colW[lv]
		if lv < maxLevel {
			l.edgeX[lv] = x
			x += l.edgeW[lv]
		}
	}
	l.cols = x

	// row assignment with fixed spacing
	maxNodeRow := 1
	for lv := 0; lv <= maxLevel; lv++ {
		ids := layerIDs[lv]
		if len(ids) == 0 {
			continue
		}
		for i, id := range ids {
			l.nodeCol[id] = lv
			row := 1 + i*(rowSpacing+1)
			l.nodeRow[id] = row
			if row > maxNodeRow {
				maxNodeRow = row
			}
		}
	}

	// Assign gutter slots for adjacent-level edges.
	// Each unique source node in a gutter gets its own vertical slot,
	// so all outgoing edges from that source share the same gutter X.
	for lv := 0; lv < nCols-1; lv++ {
		sources := []string{}
		seen := map[string]bool{}
		for _, e := range edges {
			srcLv := levels[e.From]
			dstLv := levels[e.To]
			if srcLv == lv && dstLv == lv+1 && !seen[e.From] {
				sources = append(sources, e.From)
				seen[e.From] = true
			}
		}
		// Sort by row for stable, deterministic ordering
		sort.Slice(sources, func(i, j int) bool {
			return l.nodeRow[sources[i]] < l.nodeRow[sources[j]]
		})
		slotMap := map[string]int{}
		for i, src := range sources {
			slotMap[src] = i
		}
		for _, e := range edges {
			srcLv := levels[e.From]
			dstLv := levels[e.To]
			if srcLv == lv && dstLv == lv+1 {
				if slot, ok := slotMap[e.From]; ok {
					l.gutterSlot[key(e)] = slot
				}
			}
		}
	}

	// Pre-allocate skip-level channel Ys at the bottom of the grid
	// so they don't pierce intermediate nodes.
	skipCount := 0
	for _, e := range edges {
		if levels[e.To] > levels[e.From]+1 {
			skipCount++
		}
	}
	l.rows = maxNodeRow + 2 + skipCount
	if l.rows < minRows {
		l.rows = minRows
	}
	if maxH > 0 && l.rows > maxH {
		l.rows = maxH
	}

	// Build a set of rows occupied by real nodes
	occupiedRows := map[int]bool{}
	for _, row := range l.nodeRow {
		occupiedRows[row] = true
	}

	channelY := l.rows - 2
	for _, e := range edges {
		srcLv := levels[e.From]
		dstLv := levels[e.To]
		if dstLv > srcLv+1 {
			for channelY >= 0 && occupiedRows[channelY] {
				channelY--
			}
			if channelY < 0 {
				channelY = l.rows - 2
			}
			l.skipChannels[key(e)] = channelY
			channelY--
			if channelY < 0 {
				channelY = 0
			}
		}
	}

	return l
}

// chipLabel returns the node title only (no state prefix).
func chipLabel(n dagNode) string {
	title := n.Title
	if title == "" {
		title = shortTaskID(n.ID)
	}
	maxTitle := 45
	runes := []rune(title)
	if len(runes) > maxTitle {
		title = string(runes[:maxTitle-3]) + "..."
	}
	return title
}

// ── grid ─────────────────────────────────────────────────────

type dagGrid struct {
	cells  [][]rune // character cell (0 = space)
	focus  [][]bool // true if this cell is part of a focused edge
	width  int
	height int
}

func newGrid(w, h int) *dagGrid {
	g := &dagGrid{
		cells:  make([][]rune, h),
		focus:  make([][]bool, h),
		width:  w,
		height: h,
	}
	for i := 0; i < h; i++ {
		g.cells[i] = make([]rune, w)
		g.focus[i] = make([]bool, w)
	}
	return g
}

func (g *dagGrid) put(x, y int, ch rune) {
	if y >= 0 && y < g.height && x >= 0 && x < g.width {
		g.cells[y][x] = ch
	}
}

func (g *dagGrid) mergePut(x, y int, ch rune) {
	if y >= 0 && y < g.height && x >= 0 && x < g.width {
		existing := g.cells[y][x]
		if existing == 0 {
			g.cells[y][x] = ch
		} else {
			g.cells[y][x] = mergeBoxDraw(existing, ch)
		}
	}
}

func (g *dagGrid) putStr(x, y int, s string) {
	runeIdx := 0
	for _, ch := range s {
		g.put(x+runeIdx, y, ch)
		runeIdx++
	}
}

func (g *dagGrid) get(x, y int) rune {
	if y >= 0 && y < g.height && x >= 0 && x < g.width {
		return g.cells[y][x]
	}
	return 0
}

func (g *dagGrid) markFocus(x, y int) {
	if y >= 0 && y < g.height && x >= 0 && x < g.width {
		g.focus[y][x] = true
	}
}

// mergeBoxDraw merges two box-drawing characters at the same cell.
func mergeBoxDraw(a, b rune) rune {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a == b {
		return a
	}

	type dir uint8
	const (
		Up dir = 1 << iota
		Down
		Left
		Right
	)

	maskOf := func(ch rune) dir {
		switch ch {
		case '─':
			return Left | Right
		case '│':
			return Up | Down
		case '┌':
			return Right | Down
		case '┐':
			return Left | Down
		case '└':
			return Right | Up
		case '┘':
			return Left | Up
		case '├':
			return Up | Down | Right
		case '┤':
			return Up | Down | Left
		case '┬':
			return Left | Right | Down
		case '┴':
			return Left | Right | Up
		case '┼':
			return Up | Down | Left | Right
		case '▶':
			return Right
		}
		return 0
	}

	charOf := func(m dir) rune {
		switch m {
		case Left | Right:
			return '─'
		case Up | Down:
			return '│'
		case Right | Down:
			return '┌'
		case Left | Down:
			return '┐'
		case Right | Up:
			return '└'
		case Left | Up:
			return '┘'
		case Up | Down | Right:
			return '├'
		case Up | Down | Left:
			return '┤'
		case Left | Right | Down:
			return '┬'
		case Left | Right | Up:
			return '┴'
		case Up | Down | Left | Right:
			return '┼'
		}
		return '?'
	}

	m := maskOf(a) | maskOf(b)
	if m == 0 {
		return b
	}
	return charOf(m)
}

func (g *dagGrid) hline(x1, x2, y int, ch rune) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		g.mergePut(x, y, ch)
	}
}

func (g *dagGrid) hlineF(x1, x2, y int, ch rune) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		g.mergePut(x, y, ch)
		g.markFocus(x, y)
	}
}

func (g *dagGrid) vline(x, y1, y2 int, ch rune) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		g.mergePut(x, y, ch)
	}
}

func (g *dagGrid) vlineF(x, y1, y2 int, ch rune) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		g.mergePut(x, y, ch)
		g.markFocus(x, y)
	}
}

// ── edge routing ─────────────────────────────────────────────

func paintEdges(g *dagGrid, l *hLayout, edges []dagEdge, focusID string) {
	focusSet := map[string]bool{}
	if focusID != "" {
		for _, e := range edges {
			if e.From == focusID || e.To == focusID {
				focusSet[key(e)] = true
			}
		}
	}

	for _, e := range edges {
		f := focusSet[key(e)]
		routeEdge(g, l, e, f)
	}
}

func key(e dagEdge) string { return e.From + "→" + e.To }

func routeEdge(g *dagGrid, l *hLayout, e dagEdge, focus bool) {
	srcCol := l.nodeCol[e.From]
	dstCol := l.nodeCol[e.To]
	if dstCol <= srcCol {
		return
	}

	srcRow := l.nodeRow[e.From]
	dstRow := l.nodeRow[e.To]

	// Edge connection points:
	//  → exits the source column at its right edge
	//  → enters the target column at its left edge (arrow placed there)
	srcX := l.colX[srcCol] + l.colW[srcCol] - 1
	dstX := l.colX[dstCol]

	// Skip-level edge: route through an outer channel
	if dstCol > srcCol+1 {
		routeSkipLevel(g, l, e, srcCol, dstCol, srcRow, dstRow, srcX, dstX, focus)
		return
	}

	// Adjacent-level edge: corner routing through assigned gutter slot
	egX := l.edgeX[srcCol]
	slot := 0
	if s, ok := l.gutterSlot[key(e)]; ok {
		slot = s
	}
	gutterX := egX + slot

	// Horizontal from source to gutter
	if focus {
		g.hlineF(srcX, gutterX-1, srcRow, '─')
	} else {
		g.hline(srcX, gutterX-1, srcRow, '─')
	}

	if srcRow == dstRow {
		// Direct: same row, continue through gutter to target
		if focus {
			g.hlineF(gutterX, dstX-1, srcRow, '─')
		} else {
			g.hline(gutterX, dstX-1, srcRow, '─')
		}
		g.put(dstX, dstRow, '▶')
		if focus {
			g.markFocus(dstX, dstRow)
		}
		return
	}

	// Corner: vertical in gutter, but do NOT draw through the endpoints
	if srcRow < dstRow {
		if srcRow+1 <= dstRow-1 {
			if focus {
				g.vlineF(gutterX, srcRow+1, dstRow-1, '│')
			} else {
				g.vline(gutterX, srcRow+1, dstRow-1, '│')
			}
		}
		g.mergePut(gutterX, srcRow, '┐')
		g.mergePut(gutterX, dstRow, '└')
	} else {
		if dstRow+1 <= srcRow-1 {
			if focus {
				g.vlineF(gutterX, dstRow+1, srcRow-1, '│')
			} else {
				g.vline(gutterX, dstRow+1, srcRow-1, '│')
			}
		}
		g.mergePut(gutterX, srcRow, '┘')
		g.mergePut(gutterX, dstRow, '┌')
	}
	if focus {
		g.markFocus(gutterX, srcRow)
		g.markFocus(gutterX, dstRow)
	}

	// Horizontal from gutter to target (arrow at dstX)
	if focus {
		g.hlineF(gutterX+1, dstX-1, dstRow, '─')
	} else {
		g.hline(gutterX+1, dstX-1, dstRow, '─')
	}
	g.put(dstX, dstRow, '▶')
	if focus {
		g.markFocus(dstX, dstRow)
	}
}

func routeSkipLevel(g *dagGrid, l *hLayout, e dagEdge, srcCol, dstCol, srcRow, dstRow, srcX, dstX int, focus bool) {
	channelY := l.skipChannels[key(e)]
	if channelY == 0 {
		channelY = l.rows - 2
	}

	egX0 := l.edgeX[srcCol]
	slot0 := 0
	if s, ok := l.gutterSlot[key(e)]; ok {
		slot0 = s
	}
	gutter0 := egX0 + slot0

	lastEgX := l.edgeX[dstCol-1]
	lastEgW := l.edgeW[dstCol-1]
	lastGutter := lastEgX + lastEgW - 1

	// Source → gutter0 (horizontal)
	if focus {
		g.hlineF(srcX, gutter0-1, srcRow, '─')
	} else {
		g.hline(srcX, gutter0-1, srcRow, '─')
	}

	// gutter0: vertical from srcRow to channelY
	if srcRow < channelY {
		if srcRow+1 <= channelY-1 {
			if focus {
				g.vlineF(gutter0, srcRow+1, channelY-1, '│')
			} else {
				g.vline(gutter0, srcRow+1, channelY-1, '│')
			}
		}
		g.mergePut(gutter0, srcRow, '┐')
		g.mergePut(gutter0, channelY, '├')
	} else {
		if channelY+1 <= srcRow-1 {
			if focus {
				g.vlineF(gutter0, channelY+1, srcRow-1, '│')
			} else {
				g.vline(gutter0, channelY+1, srcRow-1, '│')
			}
		}
		g.mergePut(gutter0, srcRow, '┘')
		g.mergePut(gutter0, channelY, '├')
	}
	if focus {
		g.markFocus(gutter0, srcRow)
		g.markFocus(gutter0, channelY)
	}

	// Long horizontal bus across intermediate gutters
	if focus {
		g.hlineF(gutter0+1, lastGutter-1, channelY, '─')
	} else {
		g.hline(gutter0+1, lastGutter-1, channelY, '─')
	}

	// lastGutter: vertical from channelY to dstRow
	if channelY < dstRow {
		if channelY+1 <= dstRow-1 {
			if focus {
				g.vlineF(lastGutter, channelY+1, dstRow-1, '│')
			} else {
				g.vline(lastGutter, channelY+1, dstRow-1, '│')
			}
		}
		g.mergePut(lastGutter, channelY, '┌')
		g.mergePut(lastGutter, dstRow, '┘')
	} else {
		if dstRow+1 <= channelY-1 {
			if focus {
				g.vlineF(lastGutter, dstRow+1, channelY-1, '│')
			} else {
				g.vline(lastGutter, dstRow+1, channelY-1, '│')
			}
		}
		g.mergePut(lastGutter, channelY, '└')
		g.mergePut(lastGutter, dstRow, '┐')
	}
	if focus {
		g.markFocus(lastGutter, channelY)
		g.markFocus(lastGutter, dstRow)
	}

	// lastGutter → target (horizontal, arrow at dstX)
	if focus {
		g.hlineF(lastGutter+1, dstX-1, dstRow, '─')
	} else {
		g.hline(lastGutter+1, dstX-1, dstRow, '─')
	}
	g.put(dstX, dstRow, '▶')
	if focus {
		g.markFocus(dstX, dstRow)
	}
}

// ── node painting ────────────────────────────────────────────

type nodeBox struct {
	id    string
	x, y  int
	w     int
	state string
}

func paintNodes(g *dagGrid, l *hLayout, nodes map[string]dagNode, focusID string) []nodeBox {
	boxes := make([]nodeBox, 0, len(nodes))
	for id, n := range nodes {
		col := l.nodeCol[id]
		row := l.nodeRow[id]
		colX := l.colX[col]
		colW := l.colW[col]

		maxChipW := colW - 2 // leave 1 cell on each side for arrow / margin
		if maxChipW < 6 {
			maxChipW = 6
		}

		var chip string
		if id == focusID {
			// Focus node: ▸ title (no brackets, use ▸ as left indicator)
			title := strings.TrimSpace(n.Title)
			if title == "" {
				title = shortTaskID(n.ID)
			}
			runes := []rune(title)
			if len(runes) > maxChipW-2 {
				title = string(runes[:maxChipW-5]) + "..."
			}
			chip = "▸ " + title
			// Pad to maxChipW so background is consistent
			pad := maxChipW - len([]rune(chip))
			if pad > 0 {
				chip += strings.Repeat(" ", pad)
			}
		} else {
			// Normal node: [ title    ] with brackets
			title := chipLabel(n)
			innerW := maxChipW - 2 // space inside [ ]
			trunes := []rune(title)
			if len(trunes) > innerW {
				title = string(trunes[:innerW-3]) + "..."
			}
			pad := innerW - len([]rune(title))
			if pad < 0 {
				pad = 0
			}
			chip = "[" + title + strings.Repeat(" ", pad) + "]"
		}

		// Draw chip starting at colX+1 (arrow will be at colX if any)
		g.putStr(colX+1, row, chip)

		boxes = append(boxes, nodeBox{id: id, x: colX + 1, y: row, w: len([]rune(chip)), state: n.State})
	}
	return boxes
}

// ── styled output ────────────────────────────────────────────

// nodeStyle returns a lipgloss style for a node based on its state and focus.
func nodeStyle(state string, focus bool) lipgloss.Style {
	if focus {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5a5080"))
	}
	switch state {
	case "active":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5ea3f4"))
	case "paused":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5a623"))
	case "blocked":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f44747"))
	case "done":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4ec94e"))
	case "ready":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#56d8d8"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	}
}

var (
	stEdge   = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	stFocusE = lipgloss.NewStyle().Foreground(styles.Accent)
	stPrefix = lipgloss.NewStyle().Bold(true).Foreground(styles.Accent)
)

type cellKind int

const (
	kindSpace cellKind = iota
	kindEdge
	kindFocusEdge
	kindNode
	kindFocusNode
	kindPrefix
)

func styledString(g *dagGrid, focusID string, boxes []nodeBox, nodes map[string]dagNode, maxW int) string {
	nodeArea := map[int]map[int]string{}
	for _, b := range boxes {
		if nodeArea[b.y] == nil {
			nodeArea[b.y] = map[int]string{}
		}
		for dx := 0; dx < b.w; dx++ {
			nodeArea[b.y][b.x+dx] = b.id
		}
	}

	nodeStyles := map[string]lipgloss.Style{}
	for id, n := range nodes {
		nodeStyles[id] = nodeStyle(n.State, id == focusID)
	}

	var sb strings.Builder
	for y := 0; y < g.height; y++ {
		kinds := make([]cellKind, g.width)
		for x := 0; x < g.width; x++ {
			ch := g.cells[y][x]
			kinds[x] = classifyCell(x, y, ch, g, nodeArea, focusID)
		}

		var lineSB strings.Builder
		for x := 0; x < g.width; {
			k := kinds[x]

			end := x + 1
			for end < g.width && kinds[end] == k && g.cells[y][end] != 0 {
				end++
			}

			if k == kindSpace {
				for i := x; i < end; i++ {
					lineSB.WriteByte(' ')
				}
				x = end
				continue
			}

			run := string(g.cells[y][x:end])

			switch k {
			case kindEdge:
				lineSB.WriteString(stEdge.Render(run))
			case kindFocusEdge:
				lineSB.WriteString(stFocusE.Render(run))
			case kindNode, kindFocusNode:
				nid := ""
				for xi := x; xi < end; xi++ {
					if ids, ok := nodeArea[y]; ok {
						if id, ok2 := ids[xi]; ok2 {
							nid = id
							break
						}
					}
				}
				if nid != "" {
					lineSB.WriteString(nodeStyles[nid].Render(run))
				} else {
					lineSB.WriteString(run)
				}
			case kindPrefix:
				lineSB.WriteString(stPrefix.Render(run))
			default:
				lineSB.WriteString(run)
			}
			x = end
		}

		line := lineSB.String()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if maxW > 0 && lipgloss.Width(line) > maxW {
			line = lipgloss.NewStyle().MaxWidth(maxW).Render(line)
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return strings.TrimRight(sb.String(), "\n")
}

func classifyCell(x, y int, ch rune, g *dagGrid, nodeArea map[int]map[int]string, focusID string) cellKind {
	if ch == 0 {
		return kindSpace
	}
	if ch == '▸' {
		return kindPrefix
	}
	if ids, ok := nodeArea[y]; ok {
		if nid, ok2 := ids[x]; ok2 {
			if nid == focusID {
				return kindFocusNode
			}
			return kindNode
		}
	}
	if isEdgeChar(ch) {
		if g.focus[y][x] {
			return kindFocusEdge
		}
		return kindEdge
	}
	return kindSpace
}

func isEdgeChar(ch rune) bool {
	switch ch {
	case '─', '│', '┐', '┘', '└', '┌', '├', '┤', '┬', '┴', '┼', '▶':
		return true
	}
	return false
}

// ── public entry point ────────────────────────────────────────

func renderHorizontalDAG(
	nodes map[string]dagNode,
	edges []dagEdge,
	levels map[string]int,
	layerIDs map[int][]string,
	maxLevel int,
	focusID string,
	maxW int,
	maxH int,
) string {
	if len(nodes) == 0 {
		return ""
	}

	l := buildLayout(nodes, edges, levels, layerIDs, maxLevel, maxW, maxH)
	g := newGrid(l.cols, l.rows)

	paintEdges(g, l, edges, focusID)
	boxes := paintNodes(g, l, nodes, focusID)

	return styledString(g, focusID, boxes, nodes, maxW)
}
