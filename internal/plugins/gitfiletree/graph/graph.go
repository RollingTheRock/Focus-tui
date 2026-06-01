package graph

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Commit represents a commit with its parent hashes.
type Commit struct {
	Hash    string
	Subject string
	Author  string
	Date    string
	Parents []string
}

func (c *Commit) HashPtr() *string   { return &c.Hash }
func (c *Commit) ParentPtrs() []*string {
	out := make([]*string, len(c.Parents))
	for i := range c.Parents {
		out[i] = &c.Parents[i]
	}
	return out
}
func (c *Commit) IsMerge() bool      { return len(c.Parents) > 1 }
func (c *Commit) IsFirstCommit() bool { return len(c.Parents) == 0 }

// pipeKind describes what a pipe does at the current row.
type pipeKind int

const (
	pipeTerminates pipeKind = iota
	pipeStarts
	pipeContinues
)

// pipe represents a branch line segment on one row.
type pipe struct {
	fromPos  int
	toPos    int
	fromHash string
	toHash   string
	kind     pipeKind
}

func (p pipe) left() int  { return min(p.fromPos, p.toPos) }
func (p pipe) right() int { return max(p.fromPos, p.toPos) }

// Renderer renders a lazygit-style commit graph.
type Renderer struct {
	commits           []Commit
	selectedHash      string
	branchColors      map[string]lipgloss.Style
	defaultLineStyle  lipgloss.Style
	commitNodeStyle   lipgloss.Style
	mergeNodeStyle    lipgloss.Style
	highlightStyle    lipgloss.Style
}

// NewRenderer creates a graph renderer.
func NewRenderer(commits []Commit, selectedHash string) *Renderer {
	// Assign a stable color to each branch head (first commit of each branch)
	colors := []string{
		"#A78BFA", "#34D399", "#FBBF24", "#F472B6",
		"#60A5FA", "#FB923C", "#2DD4BF", "#F87171",
	}
	branchColors := make(map[string]lipgloss.Style)
	colorIdx := 0
	for _, c := range commits {
		if len(c.Parents) <= 1 && colorIdx < len(colors) {
			branchColors[c.Hash] = lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIdx]))
			colorIdx++
		}
	}

	return &Renderer{
		commits:      commits,
		selectedHash: selectedHash,
		branchColors: branchColors,
		defaultLineStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")),
		commitNodeStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB")),
		mergeNodeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")),
		highlightStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")),
	}
}

// Render returns the rendered graph lines.
func (r *Renderer) Render(width int) []string {
	pipeSets := getPipeSets(r.commits)
	if len(pipeSets) == 0 {
		return nil
	}

	lines := make([]string, len(pipeSets))
	for i, pipes := range pipeSets {
		var prevCommit *Commit
		if i > 0 {
			prevCommit = &r.commits[i-1]
		}
		lines[i] = r.renderPipeSet(pipes, &r.commits[i], prevCommit, width)
	}
	return lines
}

// getPipeSets computes the pipe layout for each commit row.
func getPipeSets(commits []Commit) [][]pipe {
	if len(commits) == 0 {
		return nil
	}
	startHash := "START"
	pipes := []pipe{{fromPos: 0, toPos: 0, fromHash: startHash, toHash: commits[0].Hash, kind: pipeStarts}}

	pipeSets := make([][]pipe, len(commits))
	for i, c := range commits {
		pipes = getNextPipes(pipes, c)
		pipeSets[i] = pipes
	}
	return pipeSets
}

func getNextPipes(prevPipes []pipe, commit Commit) []pipe {
	maxPos := 0
	for _, p := range prevPipes {
		if p.toPos > maxPos {
			maxPos = p.toPos
		}
	}

	// Filter out pipes that terminated on the previous row
	currentPipes := make([]pipe, 0, len(prevPipes))
	for _, p := range prevPipes {
		if p.kind != pipeTerminates {
			currentPipes = append(currentPipes, p)
		}
	}

	newPipes := make([]pipe, 0, len(currentPipes)+len(commit.Parents))
	// Default pos: far right (for `git log --all` detached commits)
	pos := maxPos + 1
	for _, p := range currentPipes {
		if p.toHash == commit.Hash {
			pos = p.toPos
			break
		}
	}

	takenSpots := make(map[int]struct{})
	traversedSpots := make(map[int]struct{})

	toHash := ""
	if commit.IsFirstCommit() {
		toHash = "EMPTY_TREE"
	} else {
		toHash = commit.Parents[0]
	}
	newPipes = append(newPipes, pipe{
		fromPos:  pos,
		toPos:    pos,
		fromHash: commit.Hash,
		toHash:   toHash,
		kind:     pipeStarts,
	})

	traversedSpotsForContinuing := make(map[int]struct{})
	for _, p := range currentPipes {
		if p.toHash != commit.Hash {
			traversedSpotsForContinuing[p.toPos] = struct{}{}
		}
	}

	nextAvailableForContinuing := func() int {
		i := 0
		for {
			if _, ok := traversedSpots[i]; !ok {
				return i
			}
			i++
		}
	}
	nextAvailableForNew := func() int {
		i := 0
		for {
			_, inTaken := takenSpots[i]
			_, inTraversed := traversedSpotsForContinuing[i]
			if !inTaken && !inTraversed {
				return i
			}
			i++
		}
	}
	traverse := func(from, to int) {
		left, right := from, to
		if left > right {
			left, right = right, left
		}
		for i := left; i <= right; i++ {
			traversedSpots[i] = struct{}{}
		}
		takenSpots[to] = struct{}{}
	}

	for _, p := range currentPipes {
		if p.toHash == commit.Hash {
			// Terminates here
			newPipes = append(newPipes, pipe{
				fromPos:  p.toPos,
				toPos:    pos,
				fromHash: p.fromHash,
				toHash:   p.toHash,
				kind:     pipeTerminates,
			})
			traverse(p.toPos, pos)
		} else if p.toPos < pos {
			// Continuing on the left
			available := nextAvailableForContinuing()
			newPipes = append(newPipes, pipe{
				fromPos:  p.toPos,
				toPos:    available,
				fromHash: p.fromHash,
				toHash:   p.toHash,
				kind:     pipeContinues,
			})
			traverse(p.toPos, available)
		}
	}

	if commit.IsMerge() {
		for _, parent := range commit.Parents[1:] {
			available := nextAvailableForNew()
			newPipes = append(newPipes, pipe{
				fromPos:  pos,
				toPos:    available,
				fromHash: commit.Hash,
				toHash:   parent,
				kind:     pipeStarts,
			})
			takenSpots[available] = struct{}{}
		}
	}

	for _, p := range currentPipes {
		if p.toHash != commit.Hash && p.toPos > pos {
			// Continuing on the right, shift left to fill gaps
			last := p.toPos
			for i := p.toPos; i > pos; i-- {
				_, taken := takenSpots[i]
				_, traversed := traversedSpots[i]
				if taken || traversed {
					break
				}
				last = i
			}
			newPipes = append(newPipes, pipe{
				fromPos:  p.toPos,
				toPos:    last,
				fromHash: p.fromHash,
				toHash:   p.toHash,
				kind:     pipeContinues,
			})
			traverse(p.toPos, last)
		}
	}

	// Sort by toPos, then by kind
	sortPipes(newPipes)
	return newPipes
}

func sortPipes(pipes []pipe) {
	for i := 0; i < len(pipes); i++ {
		for j := i + 1; j < len(pipes); j++ {
			if pipes[j].toPos < pipes[i].toPos ||
				(pipes[j].toPos == pipes[i].toPos && pipes[j].kind < pipes[i].kind) {
				pipes[i], pipes[j] = pipes[j], pipes[i]
			}
		}
	}
}

// renderPipeSet renders one row of the graph.
func (r *Renderer) renderPipeSet(pipes []pipe, commit *Commit, prevCommit *Commit, width int) string {
	maxPos := 0
	commitPos := 0
	startCount := 0
	for _, p := range pipes {
		if p.kind == pipeStarts {
			startCount++
			commitPos = p.fromPos
		} else if p.kind == pipeTerminates {
			commitPos = p.toPos
		}
		if p.right() > maxPos {
			maxPos = p.right()
		}
	}
	isMerge := startCount > 1

	cells := make([]*cell, maxPos+1)
	for i := range cells {
		cells[i] = &cell{cellType: connectionCell}
	}

	renderPipe := func(p *pipe, st lipgloss.Style, overrideRight bool) {
		left := p.left()
		right := p.right()
		if left != right {
			for i := left + 1; i < right; i++ {
				cells[i].setLeft(st).setRight(st, overrideRight)
			}
			cells[left].setRight(st, overrideRight)
			cells[right].setLeft(st)
		}
		if p.kind == pipeStarts || p.kind == pipeContinues {
			cells[p.toPos].setDown(st)
		}
		if p.kind == pipeTerminates || p.kind == pipeContinues {
			cells[p.fromPos].setUp(st)
		}
	}

	// Determine if we should highlight the selected commit's path
	highlight := true
	if prevCommit != nil && prevCommit.Hash == r.selectedHash {
		highlight = false
		for _, p := range pipes {
			if p.fromHash == r.selectedHash && (p.kind != pipeTerminates || p.fromPos != p.toPos) {
				highlight = true
			}
		}
	}

	// Partition into selected / non-selected pipes
	selectedPipes := make([]pipe, 0)
	nonSelectedPipes := make([]pipe, 0)
	for _, p := range pipes {
		if highlight && p.fromHash == r.selectedHash {
			selectedPipes = append(selectedPipes, p)
		} else {
			nonSelectedPipes = append(nonSelectedPipes, p)
		}
	}

	// Render non-selected STARTS pipes first
	for _, p := range nonSelectedPipes {
		if p.kind == pipeStarts {
			style := r.lineStyleForPipe(&p, commit)
			renderPipe(&p, style, true)
		}
	}
	// Render non-selected other pipes
	for _, p := range nonSelectedPipes {
		if p.kind != pipeStarts && !(p.kind == pipeTerminates && p.fromPos == commitPos && p.toPos == commitPos) {
			style := r.lineStyleForPipe(&p, commit)
			renderPipe(&p, style, false)
		}
	}

	// Render selected pipes (clear cells first, then re-render with highlight)
	for _, p := range selectedPipes {
		for i := p.left(); i <= p.right(); i++ {
			cells[i].reset()
		}
	}
	for _, p := range selectedPipes {
		renderPipe(&p, r.highlightStyle, true)
		if p.toPos == commitPos {
			cells[p.toPos].setStyle(r.highlightStyle)
		}
	}

	// Set commit node
	cType := commitCell
	if isMerge {
		cType = mergeCell
	}
	cells[commitPos].setType(cType)

	// Build string
	var sb strings.Builder
	for _, c := range cells {
		c.render(&sb)
	}

	// Truncate/pad to width (each cell is 2 chars wide)
	result := sb.String()
	graphWidth := len(cells) * 2
	if graphWidth > width {
		result = result[:width]
	} else if graphWidth < width {
		result += strings.Repeat(" ", width-graphWidth)
	}
	return result
}

func (r *Renderer) lineStyleForPipe(p *pipe, commit *Commit) lipgloss.Style {
	if style, ok := r.branchColors[p.fromHash]; ok {
		return style
	}
	return r.defaultLineStyle
}

// --- Cell rendering (simplified from lazygit) ---

type cellType int

const (
	connectionCell cellType = iota
	commitCell
	mergeCell
)

type cell struct {
	up, down, left, right bool
	cellType              cellType
	rightStyle            *lipgloss.Style
	style                 lipgloss.Style
}

func (c *cell) reset() {
	c.up = false
	c.down = false
	c.left = false
	c.right = false
}

func (c *cell) setUp(st lipgloss.Style) *cell {
	c.up = true
	c.style = st
	return c
}
func (c *cell) setDown(st lipgloss.Style) *cell {
	c.down = true
	c.style = st
	return c
}
func (c *cell) setLeft(st lipgloss.Style) *cell {
	c.left = true
	if !c.up && !c.down {
		c.style = st
	}
	return c
}
func (c *cell) setRight(st lipgloss.Style, override bool) *cell {
	c.right = true
	if c.rightStyle == nil || override {
		c.rightStyle = &st
	}
	return c
}
func (c *cell) setStyle(st lipgloss.Style) *cell {
	c.style = st
	return c
}
func (c *cell) setType(t cellType) *cell {
	c.cellType = t
	return c
}

func (c *cell) render(w *strings.Builder) {
	first, second := getBoxDrawingChars(c.up, c.down, c.left, c.right)
	adjustedFirst := first
	switch c.cellType {
	case commitCell:
		adjustedFirst = "○"
	case mergeCell:
		adjustedFirst = "◎"
	}

	rs := c.rightStyle
	if rs == nil {
		rs = &c.style
	}

	if second == " " {
		w.WriteString(c.style.Render(adjustedFirst))
		w.WriteString(" ")
	} else {
		w.WriteString(c.style.Render(adjustedFirst))
		w.WriteString(rs.Render(second))
	}
}

func getBoxDrawingChars(up, down, left, right bool) (string, string) {
	switch {
	case up && down && left && right:
		return "│", "─"
	case up && down && left && !right:
		return "│", " "
	case up && down && !left && right:
		return "│", "─"
	case up && down && !left && !right:
		return "│", " "
	case up && !down && left && right:
		return "┴", "─"
	case up && !down && left && !right:
		return "╯", " "
	case up && !down && !left && right:
		return "╰", "─"
	case up && !down && !left && !right:
		return "╵", " "
	case !up && down && left && right:
		return "┬", "─"
	case !up && down && left && !right:
		return "╮", " "
	case !up && down && !left && right:
		return "╭", "─"
	case !up && down && !left && !right:
		return "╷", " "
	case !up && !down && left && right:
		return "─", "─"
	case !up && !down && left && !right:
		return "─", " "
	case !up && !down && !left && right:
		return "╶", "─"
	case !up && !down && !left && !right:
		return " ", " "
	}
	return " ", " "
}

// Truncate safely truncates a string for display.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
