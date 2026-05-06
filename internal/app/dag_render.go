package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// Layout Engine: Sugiyama-style hierarchical DAG layout adapted for horizontal
// rendering (levels = columns, left-to-right).
//
// Based on ascii-dag (https://github.com/AshutoshMahala/ascii-dag) but
// transposed for horizontal flow and rewritten in Go.
// ============================================================================

// layoutNode is a positioned node in the layout grid.
type layoutNode struct {
	id         string
	label      string
	state      string
	x, y       int // top-left corner in character cells
	w, h       int // width and height in cells
	cx, cy     int // center coordinates
	level      int // column index (0 = leftmost)
	levelPos   int // position within the column (row index)
	isDummy    bool
	isFocus    bool
}

// layoutEdge is a routed edge between two nodes.
type layoutEdge struct {
	fromID string
	toID   string
	fromX  int // source center x
	fromY  int // source bottom y
	toX    int // target center x
	toY    int // target top y
	path   edgePath
}

// edgePath describes how an edge is routed.
type edgePath interface{}

type edgePathDirect struct{}

type edgePathCorner struct {
	gutterX int // x coordinate in gutter where the vertical segment runs
}

type edgePathMultiSegment struct {
	waypoints      []point
	startXOffset   int
}

type point struct{ x, y int }

// dagLayout holds the complete computed layout.
type dagLayout struct {
	nodes      []*layoutNode
	edges      []*layoutEdge
	nodeByID   map[string]*layoutNode
	levels     [][]int // level -> node indices
	width      int
	height     int
	levelCount int
}

// ============================================================================
// Constants
// ============================================================================

const (
	nodePadX       = 2  // horizontal spacing between nodes in same column
	nodePadY       = 1  // vertical spacing between nodes in same column
	gutterMinW     = 3  // minimum gutter width between columns
	gutterSlotH    = 1  // vertical space per edge slot in gutter
	maxSlots       = 8  // max edge slots per gutter to prevent unbounded growth
	dummyWidth     = 3  // width of dummy node markers in gutter
	nodeHeight     = 1  // node chip is single-line
	crossingPasses = 4  // median crossing reduction passes
)

// ============================================================================
// Public Entry Point
// ============================================================================

// renderHorizontalDAG renders a DAG in horizontal (left-to-right) layout.
func renderHorizontalDAG(
	nodes map[string]dagNode,
	edges []dagEdge,
	levels map[string]int,
	layerIDs map[int][]string,
	maxLevel int,
	focusID string,
	maxW, maxH int,
) string {
	if len(nodes) == 0 {
		return ""
	}

	// Build layout IR.
	lyt := buildLayout(nodes, edges, levels, layerIDs, maxLevel, focusID)

	// Render to grid.
	grid := newRenderGrid(lyt.width, lyt.height)
	paintEdges(grid, lyt)
	paintNodes(grid, lyt)

	// Convert grid to styled string.
	return styledString(grid, lyt, maxW, maxH)
}

// ============================================================================
// Phase 1: Layout Computation
// ============================================================================

func buildLayout(
	nodes map[string]dagNode,
	edges []dagEdge,
	levels map[string]int,
	layerIDs map[int][]string,
	maxLevel int,
	focusID string,
) *dagLayout {
	lyt := &dagLayout{
		nodeByID:   make(map[string]*layoutNode),
		levelCount: maxLevel + 1,
		levels:     make([][]int, maxLevel+1),
	}

	// --- Step 1: Create virtual levels with dummy nodes for skip-level edges ---
	virtualLevels := buildVirtualLevels(nodes, edges, levels, maxLevel)

	// Ensure deterministic initial order within each level.
	for lv := range virtualLevels {
		sort.Slice(virtualLevels[lv], func(i, j int) bool {
			ai, aj := virtualLevels[lv][i].id, virtualLevels[lv][j].id
			if ai != aj {
				return ai < aj
			}
			return virtualLevels[lv][i].edgeIdx < virtualLevels[lv][j].edgeIdx
		})
	}

	// --- Step 2: Crossing reduction (Median heuristic) ---
	reduceCrossings(virtualLevels, edges, nodes, maxLevel)

	// --- Step 3: Compute node dimensions and build label map ---
	nodeLabel := make(map[string]string)
	nodeWidth := make(map[string]int)
	for id, n := range nodes {
		lbl := chipLabel(n)
		nodeLabel[id] = lbl
		nodeWidth[id] = runeWidth(lbl)
	}

	// --- Step 4: Assign Y coordinates (rows) within each column ---
	// Y = vertical position; within a column nodes stack top-to-bottom.
	nodeY := make(map[string]int)
	dummyY := make(map[int]map[string]int) // level -> dummyKey -> y
	for lv := 0; lv <= maxLevel; lv++ {
		vlevel := virtualLevels[lv]
		y := 0
		for i, vnode := range vlevel {
			if i > 0 {
				y += nodePadY
			}
			if vnode.isDummy {
				key := fmt.Sprintf("%d-%d", vnode.edgeIdx, lv)
				if dummyY[lv] == nil {
					dummyY[lv] = make(map[string]int)
				}
				dummyY[lv][key] = y
				y += dummyWidth
			} else {
				nodeY[vnode.id] = y
				y += nodeHeight
			}
		}
	}

	// --- Step 5: Assign X coordinates (columns) ---
	// X = horizontal position; columns flow left-to-right.
	colX := make([]int, maxLevel+1)
	colW := make([]int, maxLevel+1)
	x := 0
	for lv := 0; lv <= maxLevel; lv++ {
		colX[lv] = x
		maxNodeW := 0
		for _, vnode := range virtualLevels[lv] {
			if !vnode.isDummy {
				w := nodeWidth[vnode.id]
				if w > maxNodeW {
					maxNodeW = w
				}
			}
		}
		if maxNodeW == 0 {
			maxNodeW = dummyWidth
		}
		colW[lv] = maxNodeW
		x += maxNodeW + gutterMinW
	}

	// Total width before gutter trimming.
	lyt.width = x

	// --- Step 6: Build layout nodes ---
	for lv := 0; lv <= maxLevel; lv++ {
		for pos, vnode := range virtualLevels[lv] {
			if vnode.isDummy {
				continue // dummies are routing waypoints, not rendered
			}
			id := vnode.id
			n := nodes[id]
			w := nodeWidth[id]
			y := nodeY[id]
			ln := &layoutNode{
				id:       id,
				label:    nodeLabel[id],
				state:    n.State,
				x:        colX[lv],
				y:        y,
				w:        w,
				h:        nodeHeight,
				cx:       colX[lv] + w/2,
				cy:       y + nodeHeight/2,
				level:    lv,
				levelPos: pos,
				isFocus:  id == focusID,
			}
			idx := len(lyt.nodes)
			lyt.nodes = append(lyt.nodes, ln)
			lyt.nodeByID[id] = ln
			lyt.levels[lv] = append(lyt.levels[lv], idx)
		}
	}

	// --- Step 7: Slot allocation for edge routing ---
	// Assign each edge a vertical slot in each gutter it passes through.
	edgeSlots := allocateSlots(edges, levels, colX, colW, nodeY, dummyY, virtualLevels, maxLevel)

	// --- Step 8: Build layout edges with routing paths ---
	for ei, e := range edges {
		fromNode := lyt.nodeByID[e.From]
		toNode := lyt.nodeByID[e.To]
		if fromNode == nil || toNode == nil {
			continue
		}
		fromLevel := levels[e.From]
		toLevel := levels[e.To]

		fromX := fromNode.cx // center of source
		fromY := fromNode.cy // center-y of source
		toX := toNode.x - 1 // one cell left of target (arrow position)
		if toX <= fromX {
			toX = fromX + 2
		}
		toY := toNode.cy // center-y of target

		var path edgePath
		if toLevel == fromLevel+1 {
			// Adjacent columns.
			if fromY == toY {
				path = edgePathDirect{}
			} else {
				slot := edgeSlots[ei]
				gutterStart := colX[fromLevel] + colW[fromLevel]
				gutterEnd := colX[toLevel]
				gutterWidth := gutterEnd - gutterStart
				if gutterWidth < 2 {
					gutterWidth = 2
				}
				// Distribute slots evenly, ensuring gutterX > fromX (source right edge).
				gutterX := gutterStart + 1 + slot
				if gutterX >= gutterEnd {
					gutterX = gutterEnd - 1
				}
				path = edgePathCorner{gutterX: gutterX}
			}
		} else {
			// Skip-level: build waypoints through dummy nodes.
			waypoints := buildWaypoints(ei, e, fromLevel, toLevel, colX, colW, dummyY, edgeSlots)
			slot := edgeSlots[ei]
			path = edgePathMultiSegment{
				waypoints:    waypoints,
				startXOffset: slot,
			}
		}

		lyt.edges = append(lyt.edges, &layoutEdge{
			fromID: e.From,
			toID:   e.To,
			fromX:  fromX,
			fromY:  fromY,
			toX:    toX,
			toY:    toY,
			path:   path,
		})
	}

	// --- Step 9: Compute total height ---
	maxY := 0
	for _, n := range lyt.nodes {
		if n.y+n.h > maxY {
			maxY = n.y + n.h
		}
	}
	for ei, e := range edges {
		fromNode := lyt.nodeByID[e.From]
		toNode := lyt.nodeByID[e.To]
		if fromNode == nil || toNode == nil {
			continue
		}
		// Account for routing space.
		fromLevel := levels[e.From]
		toLevel := levels[e.To]
		if toLevel > fromLevel {
			lastGutterBottom := toNode.y
			if len(virtualLevels[toLevel-1]) > 0 {
				// estimate bottom of previous column
				lastNodeInPrev := virtualLevels[toLevel-1][len(virtualLevels[toLevel-1])-1]
				var prevBottom int
				if lastNodeInPrev.isDummy {
					key := fmt.Sprintf("%d-%d", lastNodeInPrev.edgeIdx, toLevel-1)
					prevBottom = dummyY[toLevel-1][key] + dummyWidth
				} else {
					prevBottom = nodeY[lastNodeInPrev.id] + nodeHeight
				}
				if prevBottom > lastGutterBottom {
					lastGutterBottom = prevBottom
				}
			}
			slot := edgeSlots[ei]
			routeBottom := lastGutterBottom + 2 + slot*gutterSlotH
			if routeBottom > maxY {
				maxY = routeBottom
			}
		}
	}
	lyt.height = maxY + 1

	return lyt
}

// ============================================================================
// Step 1: Virtual Levels (with dummy nodes for skip-level edges)
// ============================================================================

type virtualNode struct {
	id      string
	isDummy bool
	edgeIdx int // for dummy: which edge this dummy belongs to
}

func buildVirtualLevels(
	nodes map[string]dagNode,
	edges []dagEdge,
	levels map[string]int,
	maxLevel int,
) [][]virtualNode {
	virtualLevels := make([][]virtualNode, maxLevel+1)

	// Add real nodes.
	for id, lv := range levels {
		virtualLevels[lv] = append(virtualLevels[lv], virtualNode{id: id})
	}

	// Add dummy nodes for skip-level edges.
	for ei, e := range edges {
		fromLevel := levels[e.From]
		toLevel := levels[e.To]
		if toLevel > fromLevel+1 {
			for lv := fromLevel + 1; lv < toLevel; lv++ {
				virtualLevels[lv] = append(virtualLevels[lv], virtualNode{
					isDummy: true,
					edgeIdx: ei,
				})
			}
		}
	}

	return virtualLevels
}

// ============================================================================
// Step 2: Crossing Reduction (Median Heuristic)
// ============================================================================

func reduceCrossings(
	virtualLevels [][]virtualNode,
	edges []dagEdge,
	nodes map[string]dagNode,
	maxLevel int,
) {
	// Build adjacency for quick lookup.
	children := make(map[string][]string)
	parents := make(map[string][]string)
	for _, e := range edges {
		children[e.From] = append(children[e.From], e.To)
		parents[e.To] = append(parents[e.To], e.From)
	}

	for pass := 0; pass < crossingPasses; pass++ {
		// Top-down: order by median of parents.
		for lv := 1; lv <= maxLevel; lv++ {
			orderByMedian(virtualLevels[lv], virtualLevels[lv-1], parents, true)
		}
		// Bottom-up: order by median of children.
		for lv := maxLevel - 1; lv >= 0; lv-- {
			orderByMedian(virtualLevels[lv], virtualLevels[lv+1], children, false)
		}
	}
}

func orderByMedian(
	level []virtualNode,
	adjLevel []virtualNode,
	adjacency map[string][]string,
	useParents bool,
) {
	if len(level) < 2 {
		return
	}

	// Build position map for adjacent level.
	posMap := make(map[string]int)
	for i, v := range adjLevel {
		if !v.isDummy {
			posMap[v.id] = i
		}
	}

	type medianRec struct {
		vnode  virtualNode
		median float64
	}
	recs := make([]medianRec, 0, len(level))

	for i, vnode := range level {
		if vnode.isDummy {
			recs = append(recs, medianRec{vnode: vnode, median: float64(i)})
			continue
		}
		var positions []int
		for _, adj := range adjacency[vnode.id] {
			if p, ok := posMap[adj]; ok {
				positions = append(positions, p)
			}
		}
		if len(positions) == 0 {
			recs = append(recs, medianRec{vnode: vnode, median: float64(i)})
			continue
		}
		sort.Ints(positions)
		var median float64
		if len(positions)%2 == 1 {
			median = float64(positions[len(positions)/2])
		} else {
			mid := len(positions) / 2
			median = float64(positions[mid-1]+positions[mid]) / 2.0
		}
		recs = append(recs, medianRec{vnode: vnode, median: median})
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].median < recs[j].median
	})

	for i := range level {
		level[i] = recs[i].vnode
	}
}

// ============================================================================
// Step 7: Slot Allocation
// ============================================================================

func allocateSlots(
	edges []dagEdge,
	levels map[string]int,
	colX, colW []int,
	nodeY map[string]int,
	dummyY map[int]map[string]int,
	virtualLevels [][]virtualNode,
	maxLevel int,
) []int {
	edgeSlots := make([]int, len(edges))
	for i := range edgeSlots {
		edgeSlots[i] = -1
	}

	// For each gutter (between level L and L+1), track occupied slots.
	// slotOccupied[level] = list of (minY, maxY) for each slot at that gutter.
	slotOccupied := make(map[int][][][2]int) // level -> slot -> intervals

	for ei, e := range edges {
		fromLevel := levels[e.From]
		toLevel := levels[e.To]
		if toLevel <= fromLevel {
			continue
		}

		fromNodeY := nodeY[e.From]
		toNodeY := nodeY[e.To]
		if fromNodeY > toNodeY {
			fromNodeY, toNodeY = toNodeY, fromNodeY
		}

		gutter := fromLevel
		if slotOccupied[gutter] == nil {
			slotOccupied[gutter] = make([][][2]int, 0)
		}

		// Try to find a non-conflicting slot.
		chosenSlot := -1
		for s := 0; s < len(slotOccupied[gutter]) && s < maxSlots; s++ {
			conflict := false
			for _, interval := range slotOccupied[gutter][s] {
				if fromNodeY < interval[1] && toNodeY > interval[0] {
					conflict = true
					break
				}
			}
			if !conflict {
				chosenSlot = s
				slotOccupied[gutter][s] = append(slotOccupied[gutter][s], [2]int{fromNodeY, toNodeY})
				break
			}
		}
		if chosenSlot < 0 && len(slotOccupied[gutter]) < maxSlots {
			chosenSlot = len(slotOccupied[gutter])
			slotOccupied[gutter] = append(slotOccupied[gutter], [][2]int{{fromNodeY, toNodeY}})
		}
		if chosenSlot < 0 {
			chosenSlot = 0 // fallback: overflow to slot 0
		}
		edgeSlots[ei] = chosenSlot
	}

	return edgeSlots
}

// ============================================================================
// Step 8: Waypoint Building for Skip-Level Edges
// ============================================================================

func buildWaypoints(
	edgeIdx int,
	e dagEdge,
	fromLevel, toLevel int,
	colX, colW []int,
	dummyY map[int]map[string]int,
	edgeSlots []int,
) []point {
	var waypoints []point
	for lv := fromLevel + 1; lv < toLevel; lv++ {
		key := fmt.Sprintf("%d-%d", edgeIdx, lv)
		y, ok := dummyY[lv][key]
		if !ok {
			continue
		}
		// X is centered in the gutter between lv-1 and lv, offset by slot.
		gutterX := colX[lv-1] + colW[lv-1]
		slot := edgeSlots[edgeIdx]
		if slot < 0 {
			slot = 0
		}
		x := gutterX + 1 + slot
		if x >= colX[lv] {
			x = colX[lv] - 1
		}
		waypoints = append(waypoints, point{x: x, y: y})
	}
	return waypoints
}

// ============================================================================
// Phase 4: Rendering Engine
// ============================================================================

// renderGrid is a 2D rune grid for painting the DAG.
type renderGrid struct {
	cells [][]rune
	w, h  int
}

func newRenderGrid(w, h int) *renderGrid {
	cells := make([][]rune, h)
	for i := range cells {
		cells[i] = make([]rune, w)
		for j := range cells[i] {
			cells[i][j] = ' '
		}
	}
	return &renderGrid{cells: cells, w: w, h: h}
}

func (g *renderGrid) set(x, y int, ch rune) {
	if x < 0 || x >= g.w || y < 0 || y >= g.h {
		return
	}
	existing := g.cells[y][x]
	if existing == ' ' {
		g.cells[y][x] = ch
		return
	}
	if existing == ch {
		return
	}
	g.cells[y][x] = mergeBoxDraw(existing, ch)
}

func (g *renderGrid) hline(y, x1, x2 int, ch rune) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		g.set(x, y, ch)
	}
}

func (g *renderGrid) vline(x, y1, y2 int, ch rune) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		g.set(x, y, ch)
	}
}

// Box-drawing character merging (direction bitmask approach from ascii-dag).
const (
	dirUp    = 1
	dirDown  = 2
	dirLeft  = 4
	dirRight = 8
)

func charMask(ch rune) int {
	switch ch {
	case '│', '┊':
		return dirUp | dirDown
	case '─', '┈':
		return dirLeft | dirRight
	case '└':
		return dirUp | dirRight
	case '┘':
		return dirUp | dirLeft
	case '┌':
		return dirDown | dirRight
	case '┐':
		return dirDown | dirLeft
	case '┴':
		return dirUp | dirLeft | dirRight
	case '┬':
		return dirDown | dirLeft | dirRight
	case '├':
		return dirUp | dirDown | dirRight
	case '┤':
		return dirUp | dirDown | dirLeft
	case '┼':
		return dirUp | dirDown | dirLeft | dirRight
	case '↓', '⇣':
		return dirUp
	case '→':
		return dirLeft
	}
	return 0
}

func maskToChar(mask int) rune {
	switch mask {
	case dirUp | dirDown:
		return '│'
	case dirLeft | dirRight:
		return '─'
	case dirUp | dirRight:
		return '└'
	case dirUp | dirLeft:
		return '┘'
	case dirDown | dirRight:
		return '┌'
	case dirDown | dirLeft:
		return '┐'
	case dirUp | dirLeft | dirRight:
		return '┴'
	case dirDown | dirLeft | dirRight:
		return '┬'
	case dirUp | dirDown | dirRight:
		return '├'
	case dirUp | dirDown | dirLeft:
		return '┤'
	case dirUp | dirDown | dirLeft | dirRight:
		return '┼'
	}
	return ' '
}

func mergeBoxDraw(a, b rune) rune {
	if a == ' ' {
		return b
	}
	if b == ' ' {
		return a
	}
	if a == b {
		return a
	}
	// Arrow precedence.
	if a == '↓' || b == '↓' {
		return '↓'
	}
	if a == '→' || b == '→' {
		return '→'
	}
	m1 := charMask(a)
	m2 := charMask(b)
	if m1 == 0 {
		return b
	}
	if m2 == 0 {
		return a
	}
	merged := maskToChar(m1 | m2)
	if merged == ' ' {
		return a
	}
	return merged
}

// ============================================================================
// Edge Painting
// ============================================================================

func paintEdges(grid *renderGrid, lyt *dagLayout) {
	for _, edge := range lyt.edges {
		paintEdge(grid, edge)
	}
}

func paintEdge(grid *renderGrid, edge *layoutEdge) {
	switch p := edge.path.(type) {
	case edgePathDirect:
		paintDirectEdge(grid, edge)
	case edgePathCorner:
		paintCornerEdge(grid, edge, p.gutterX)
	case edgePathMultiSegment:
		paintMultiSegmentEdge(grid, edge, p.waypoints, p.startXOffset)
	}
}

func paintDirectEdge(grid *renderGrid, edge *layoutEdge) {
	// Horizontal line from source right edge to target left edge.
	x1 := edge.fromX
	x2 := edge.toX
	y := edge.fromY
	if y != edge.toY {
		midX := (x1 + x2) / 2
		paintCornerEdge(grid, edge, midX)
		return
	}
	if y >= grid.h || y < 0 {
		return
	}
	if x2 > x1 {
		for x := x1; x < x2; x++ {
			if x >= 0 && x < grid.w {
				grid.set(x, y, '─')
			}
		}
		if x2 >= 0 && x2 < grid.w {
			grid.set(x2, y, '→')
		}
	} else if x2 < x1 {
		for x := x1; x > x2; x-- {
			if x >= 0 && x < grid.w {
				grid.set(x, y, '─')
			}
		}
		if x2 >= 0 && x2 < grid.w {
			grid.set(x2, y, '→')
		}
	} else {
		if y+1 < grid.h && x1+1 < grid.w {
			grid.set(x1, y, '└')
			grid.hline(y+1, x1, x1+1, '─')
			grid.set(x1+1, y+1, '┘')
			if y+2 < edge.toY && y+2 < grid.h {
				grid.vline(x1+1, y+2, edge.toY-1, '│')
			}
			if edge.toY < grid.h {
				grid.set(x1+1, edge.toY, '┌')
				grid.hline(edge.toY, x1, x1+1, '─')
				grid.set(x1, edge.toY, '→')
			}
		}
	}
}

func paintCornerEdge(grid *renderGrid, edge *layoutEdge, gutterX int) {
	fromX := edge.fromX
	fromY := edge.fromY
	toX := edge.toX
	toY := edge.toY

	goesDown := toY > fromY

	// 1. Horizontal from source right edge to gutter.
	if fromX < gutterX {
		grid.hline(fromY, fromX, gutterX, '─')
	}

	// 2. Corner at (gutterX, fromY).
	if goesDown {
		grid.set(gutterX, fromY, '┐')
	} else {
		grid.set(gutterX, fromY, '┘')
	}

	// 3. Vertical segment in gutter.
	if goesDown {
		if fromY+1 <= toY-1 {
			grid.vline(gutterX, fromY+1, toY-1, '│')
		}
	} else {
		if toY+1 <= fromY-1 {
			grid.vline(gutterX, toY+1, fromY-1, '│')
		}
	}

	// 4. Corner at (gutterX, toY).
	if goesDown {
		grid.set(gutterX, toY, '└')
	} else {
		grid.set(gutterX, toY, '┌')
	}

	// 5. Horizontal from gutter to target left edge.
	if gutterX < toX {
		grid.hline(toY, gutterX, toX, '─')
	}

	// 6. Arrow at target left edge.
	if toX >= 0 && toX < grid.w && toY >= 0 && toY < grid.h {
		grid.set(toX, toY, '→')
	}
}

func paintMultiSegmentEdge(grid *renderGrid, edge *layoutEdge, waypoints []point, startXOffset int) {
	if len(waypoints) == 0 {
		// Fallback to corner.
		gutterX := edge.fromX + 2 + startXOffset
		paintCornerEdge(grid, edge, gutterX)
		return
	}

	// Build full path: source -> waypoints -> target.
	fullPath := make([]point, 0, len(waypoints)+2)
	fullPath = append(fullPath, point{x: edge.fromX, y: edge.fromY})
	fullPath = append(fullPath, waypoints...)
	fullPath = append(fullPath, point{x: edge.toX, y: edge.toY})

	for segIdx := 0; segIdx < len(fullPath)-1; segIdx++ {
		x1, y1 := fullPath[segIdx].x, fullPath[segIdx].y
		x2, y2 := fullPath[segIdx+1].x, fullPath[segIdx+1].y
		isLast := segIdx == len(fullPath)-2
		isFirst := segIdx == 0

		if y1 == y2 {
			// Horizontal segment (same Y).
			minX, maxX := x1, x2
			if minX > maxX {
				minX, maxX = maxX, minX
			}
			startX := minX
			if !isFirst {
				startX = minX
			} else {
				startX = minX + 1 // skip source point
			}
			for x := startX; x <= maxX; x++ {
				if x >= 0 && x < grid.w && y1 >= 0 && y1 < grid.h {
					if isLast && x == maxX {
						grid.set(x, y1, '→')
					} else {
						grid.set(x, y1, '─')
					}
				}
			}
		} else if x1 == x2 {
			// Vertical segment (same X).
			minY, maxY := y1, y2
			if minY > maxY {
				minY, maxY = maxY, minY
			}
			startY := minY
			if !isFirst {
				startY = minY
			} else {
				startY = minY + 1
			}
			for y := startY; y <= maxY; y++ {
				if x1 >= 0 && x1 < grid.w && y >= 0 && y < grid.h {
					grid.set(x1, y, '│')
				}
			}
		} else {
			// L-shaped: horizontal then vertical.
			cornerX := x2
			cornerY := y1
			if isFirst && startXOffset > 0 {
				cornerX = x1 + startXOffset
			}

			// Horizontal from x1 to cornerX at y1.
			minX, maxX := x1, cornerX
			if minX > maxX {
				minX, maxX = maxX, minX
			}
			for x := minX; x <= maxX; x++ {
				if x >= 0 && x < grid.w && y1 >= 0 && y1 < grid.h {
					if x == x1 {
						if x1 < cornerX {
							if y1 < y2 {
								grid.set(x, y1, '┌')
							} else {
								grid.set(x, y1, '└')
							}
						} else {
							if y1 < y2 {
								grid.set(x, y1, '┐')
							} else {
								grid.set(x, y1, '┘')
							}
						}
					} else if x == cornerX {
						if x1 < cornerX {
							if y1 < y2 {
								grid.set(x, y1, '┐')
							} else {
								grid.set(x, y1, '┘')
							}
						} else {
							if y1 < y2 {
								grid.set(x, y1, '┌')
							} else {
								grid.set(x, y1, '└')
							}
						}
					} else {
						grid.set(x, y1, '─')
					}
				}
			}

			// Vertical from cornerY to y2 at cornerX.
			minY, maxY := cornerY, y2
			if minY > maxY {
				minY, maxY = maxY, minY
			}
			for y := minY + 1; y <= maxY; y++ {
				if cornerX >= 0 && cornerX < grid.w && y >= 0 && y < grid.h {
					grid.set(cornerX, y, '│')
				}
			}
		}
	}

	// Arrow at target.
	if edge.toX >= 0 && edge.toX < grid.w && edge.toY >= 0 && edge.toY < grid.h {
		grid.set(edge.toX, edge.toY, '→')
	}
}

// ============================================================================
// Node Painting
// ============================================================================

func paintNodes(grid *renderGrid, lyt *dagLayout) {
	for _, node := range lyt.nodes {
		paintNode(grid, node)
	}
}

func paintNode(grid *renderGrid, node *layoutNode) {
	label := []rune(node.label)
	x := node.x
	y := node.y
	if y < 0 || y >= grid.h {
		return
	}

	// Draw label runes.
	for i, r := range label {
		px := x + i
		if px >= 0 && px < grid.w {
			grid.cells[y][px] = r
		}
	}
}

// ============================================================================
// Styled Output
// ============================================================================

type cellKind int

const (
	ckSpace cellKind = iota
	ckEdge
	ckNode
	ckFocusNode
	ckFocusEdge
)

func styledString(grid *renderGrid, lyt *dagLayout, maxW, maxH int) string {
	// Compute bounding box of actual content.
	maxX := 0
	for y := 0; y < grid.h; y++ {
		for x := grid.w - 1; x >= 0; x-- {
			if grid.cells[y][x] != ' ' {
				if x > maxX {
					maxX = x
				}
				break
			}
		}
	}
	maxX++

	// Build node bounding boxes for classification.
	nodeBoxes := make(map[int]map[int]bool) // y -> x -> isFocus
	for _, node := range lyt.nodes {
		if node.y < 0 || node.y >= grid.h {
			continue
		}
		if nodeBoxes[node.y] == nil {
			nodeBoxes[node.y] = make(map[int]bool)
		}
		for i := 0; i < node.w && node.x+i < grid.w; i++ {
			nodeBoxes[node.y][node.x+i] = node.isFocus
		}
	}

	var lines []string
	for y := 0; y < grid.h && y < maxH; y++ {
		var parts []string
		var lastKind cellKind = ckSpace
		var lastFocus bool
		var buf strings.Builder

		endX := maxX
		if endX > maxW {
			endX = maxW
		}

		for x := 0; x < endX; x++ {
			ch := grid.cells[y][x]
			kind := ckSpace
			isFocus := false
			if ch != ' ' {
				if nodeBoxes[y] != nil && nodeBoxes[y][x] {
					kind = ckFocusNode
					isFocus = true
				} else if nodeBoxes[y] != nil && nodeBoxes[y][x] == false {
					kind = ckNode
				} else {
					// Check if this edge cell is adjacent to focus node.
					isFocusEdge := false
					for _, n := range lyt.nodes {
						if !n.isFocus {
							continue
						}
						if abs(x-n.cx) <= 2 && abs(y-n.cy) <= 1 {
							isFocusEdge = true
							break
						}
					}
					if isFocusEdge {
						kind = ckFocusEdge
					} else {
						kind = ckEdge
					}
				}
			}

			if kind != lastKind || isFocus != lastFocus {
				if buf.Len() > 0 {
					parts = append(parts, styleForKind(lastKind, lastFocus).Render(buf.String()))
					buf.Reset()
				}
			}
			buf.WriteRune(ch)
			lastKind = kind
			lastFocus = isFocus
		}
		if buf.Len() > 0 {
			parts = append(parts, styleForKind(lastKind, lastFocus).Render(buf.String()))
		}

		line := strings.Join(parts, "")
		// Trim trailing spaces while preserving styling.
		line = strings.TrimRight(line, " ")
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func styleForKind(kind cellKind, focus bool) lipgloss.Style {
	switch kind {
	case ckFocusNode:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#333333"))
	case ckNode:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#cccccc"))
	case ckFocusEdge:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa44"))
	case ckEdge:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	default:
		return lipgloss.NewStyle()
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// ============================================================================
// Helpers
// ============================================================================

func chipLabel(n dagNode) string {
	state := n.State
	if state == "" {
		state = "pending"
	}
	abbr := stateAbbr(state)
	title := strings.TrimSpace(n.Title)
	if title == "" {
		title = n.ID
	}
	if len(title) > 20 {
		title = title[:20]
	}
	return fmt.Sprintf("[%s] %s", abbr, title)
}

func stateAbbr(s string) string {
	switch s {
	case "active":
		return "act"
	case "paused":
		return "pau"
	case "completed":
		return "done"
	case "ready":
		return "rdy"
	case "blocked":
		return "blk"
	case "pending":
		return "pen"
	default:
		if len(s) > 3 {
			return s[:3]
		}
		return s
	}
}

func runeWidth(s string) int {
	return len([]rune(s))
}
