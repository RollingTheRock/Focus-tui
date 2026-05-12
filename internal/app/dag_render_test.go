package app

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, ch := range s {
		if ch == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func TestRenderHorizontalDAG(t *testing.T) {
	// Build a realistic 10-node DAG (the one from our dev plan)
	nodes := map[string]dagNode{
		"T1":  {ID: "T1", Title: "Fix stale/broken tests in app_test.go", State: "active"},
		"T2":  {ID: "T2", Title: "Align MCP dag.get_status level", State: "paused"},
		"T3":  {ID: "T3", Title: "Fix cursor staleness on task deletion", State: "paused"},
		"T4":  {ID: "T4", Title: "Fix cross-repo dependency filtering", State: "paused"},
		"T5":  {ID: "T5", Title: "Unit tests: keyboard navigation", State: "paused"},
		"T6":  {ID: "T6", Title: "Unit tests: cursor boundary", State: "paused"},
		"T7":  {ID: "T7", Title: "Unit tests: worktree/agent actions", State: "paused"},
		"T8":  {ID: "T8", Title: "Integration test: full DAG pipeline", State: "paused"},
		"T9":  {ID: "T9", Title: "Clean up orphaned overview components", State: "paused"},
		"T10": {ID: "T10", Title: "Final integration verification", State: "paused"},
	}

	edges := []dagEdge{
		{From: "T1", To: "T2", Type: "hard"},
		{From: "T1", To: "T3", Type: "hard"},
		{From: "T1", To: "T4", Type: "hard"},
		{From: "T1", To: "T9", Type: "hard"},
		{From: "T2", To: "T8", Type: "hard"},
		{From: "T3", To: "T5", Type: "hard"},
		{From: "T3", To: "T7", Type: "hard"},
		{From: "T4", To: "T8", Type: "hard"},
		{From: "T5", To: "T6", Type: "hard"},
		{From: "T5", To: "T8", Type: "hard"},
		{From: "T7", To: "T8", Type: "hard"},
		{From: "T6", To: "T10", Type: "hard"},
		{From: "T8", To: "T10", Type: "hard"},
		{From: "T9", To: "T10", Type: "hard"},
	}

	adj := map[string][]dagEdge{}
	indeg := map[string]int{}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e)
		indeg[e.To]++
	}
	for id := range nodes {
		if _, ok := indeg[id]; !ok {
			indeg[id] = 0
		}
	}

	levels := computeDAGLevels(nodes, adj, indeg)
	maxLevel := 0
	for _, lv := range levels {
		if lv > maxLevel {
			maxLevel = lv
		}
	}
	layerIDs := map[int][]string{}
	for id, lv := range levels {
		layerIDs[lv] = append(layerIDs[lv], id)
	}
	// Stabilise layer order so tests are deterministic regardless of map iteration
	for lv := 0; lv <= maxLevel; lv++ {
		sort.Strings(layerIDs[lv])
	}

	t.Logf("Levels: %v", levels)
	for lv := 0; lv <= maxLevel; lv++ {
		t.Logf("  L%d: %v", lv, layerIDs[lv])
	}

	// Render with different widths
	for _, w := range []int{120, 160, 200} {
		output := renderHorizontalDAG(nodes, edges, levels, layerIDs, maxLevel, "T3", w, 12)
		plain := stripANSI(output)
		t.Logf("\n=== Width=%d ===\n%s\n", w, plain)

		// Basic assertions
		if !strings.Contains(output, "Fix stale") {
			t.Errorf("Width=%d: missing T1 node", w)
		}
		if !strings.Contains(output, "Fix cursor") {
			t.Errorf("Width=%d: missing T3 node (focus)", w)
		}

		// Fix 1: gutter separation — three-or-more adjacent '│' means edges
		// are piling up in the same slot. Two adjacent '│' can happen when two
		// different sources occupy neighbouring slots in the same gutter.
		if strings.Contains(plain, "│││") {
			t.Errorf("Width=%d: found triple '│││' — edges severely overlapping", w)
		}

		// Fix 2: skip-level edges must NOT draw long horizontal lines
		// through the *same* Y as intermediate nodes. We verify by checking
		// that no row has a horizontal line segment (─) inside a node column.
		// Since we can't easily detect node columns in plain text, we use a
		// proxy: rows that contain only edge chars and spaces should not have
		// node text interleaved.
		lines := strings.Split(plain, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// If a line contains node text, it shouldn't have a long horizontal
			// line run (>10 dashes) that isn't part of a node's label
			hasNodeText := strings.Contains(line, "[") || strings.Contains(line, "▸")
			if hasNodeText && strings.Contains(line, strings.Repeat("─", 10)) {
				// Allow if the dashes are part of a node label (unlikely)
				t.Errorf("Width=%d line %d: long horizontal line in node row: %s", w, i, trimmed)
			}
		}

		// Fix 3: labels must be recognisable (at least 8 visible chars of title)
		for _, id := range []string{"T1", "T3", "T8"} {
			title := nodes[id].Title
			if len(title) > 8 {
				title = title[:8]
			}
			if !strings.Contains(output, title) {
				t.Errorf("Width=%d: node %s title prefix '%s' not visible", w, id, title)
			}
		}
	}

	fmt.Println("DONE")
}

// TestRenderHorizontalDAG_Diamond verifies a classic 4-node diamond.
func TestRenderHorizontalDAG_Diamond(t *testing.T) {
	nodes := map[string]dagNode{
		"A": {ID: "A", Title: "Start", State: "done"},
		"B": {ID: "B", Title: "Left", State: "active"},
		"C": {ID: "C", Title: "Right", State: "paused"},
		"D": {ID: "D", Title: "End", State: ""},
	}
	edges := []dagEdge{
		{From: "A", To: "B", Type: "hard"},
		{From: "A", To: "C", Type: "hard"},
		{From: "B", To: "D", Type: "hard"},
		{From: "C", To: "D", Type: "hard"},
	}
	levels := map[string]int{"A": 0, "B": 1, "C": 1, "D": 2}
	layerIDs := map[int][]string{0: {"A"}, 1: {"B", "C"}, 2: {"D"}}
	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 2, "B", 120, 8)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Start") {
		t.Error("missing Start node")
	}
	if !strings.Contains(plain, "Left") {
		t.Error("missing Left node")
	}
	if strings.Contains(plain, "││") {
		t.Error("adjacent vertical bars — gutters not separated")
	}
	t.Logf("\nDiamond:\n%s\n", plain)
}

// TestRenderHorizontalDAG_Chain verifies a simple 3-node chain.
func TestRenderHorizontalDAG_Chain(t *testing.T) {
	nodes := map[string]dagNode{
		"A": {ID: "A", Title: "Alpha", State: "done"},
		"B": {ID: "B", Title: "Beta", State: "active"},
		"C": {ID: "C", Title: "Gamma", State: ""},
	}
	edges := []dagEdge{
		{From: "A", To: "B", Type: "hard"},
		{From: "B", To: "C", Type: "hard"},
	}
	levels := map[string]int{"A": 0, "B": 1, "C": 2}
	layerIDs := map[int][]string{0: {"A"}, 1: {"B"}, 2: {"C"}}
	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 2, "", 60, 6)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Alpha") || !strings.Contains(plain, "Beta") || !strings.Contains(plain, "Gamma") {
		t.Error("missing nodes")
	}
	t.Logf("\nChain:\n%s\n", plain)
}

// TestRenderHorizontalDAG_SkipLevel verifies skip-level routing.
func TestRenderHorizontalDAG_SkipLevel(t *testing.T) {
	nodes := map[string]dagNode{
		"A": {ID: "A", Title: "A-root", State: "done"},
		"B": {ID: "B", Title: "B-mid", State: "active"},
		"C": {ID: "C", Title: "C-leaf", State: ""},
	}
	edges := []dagEdge{
		{From: "A", To: "B", Type: "hard"},
		{From: "A", To: "C", Type: "hard"},
	}
	// Force A at L0, B at L1, C at L2 (A→C is skip-level)
	levels := map[string]int{"A": 0, "B": 1, "C": 2}
	layerIDs := map[int][]string{0: {"A"}, 1: {"B"}, 2: {"C"}}
	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 2, "", 80, 8)
	plain := stripANSI(out)
	if !strings.Contains(plain, "A-root") || !strings.Contains(plain, "C-leaf") {
		t.Error("missing nodes")
	}
	// Skip-level edge should NOT draw a long horizontal segment (>5 dashes)
	// on the same row as an intermediate node.
	lines := strings.Split(plain, "\n")
	for i, line := range lines {
		if strings.Contains(line, "B-mid") {
			// Allow short horizontal connectors (1-3 dashes) adjacent to the node,
			// but flag long runs that would indicate a piercing skip-level line.
			if strings.Contains(line, strings.Repeat("─", 6)) {
				t.Errorf("skip-level edge pierces B node row at line %d: %s", i, line)
			}
		}
	}
	t.Logf("\nSkipLevel:\n%s\n", plain)
}

func TestBuildLayout_LimitsSkipLevelChannels(t *testing.T) {
	nodes := map[string]dagNode{
		"A": {ID: "A", Title: "A-root"},
		"B": {ID: "B", Title: "B-mid"},
		"C": {ID: "C", Title: "C-leaf"},
		"D": {ID: "D", Title: "D-leaf"},
		"E": {ID: "E", Title: "E-leaf"},
		"F": {ID: "F", Title: "F-leaf"},
	}
	levels := map[string]int{"A": 0, "B": 1, "C": 2, "D": 2, "E": 2, "F": 2}
	layerIDs := map[int][]string{0: {"A"}, 1: {"B"}, 2: {"C", "D", "E", "F"}}
	edges := []dagEdge{
		{From: "A", To: "B", Type: "hard"},
		{From: "A", To: "C", Type: "hard"},
		{From: "A", To: "D", Type: "hard"},
		{From: "A", To: "E", Type: "hard"},
		{From: "A", To: "F", Type: "hard"},
	}

	l := buildLayout(nodes, edges, levels, layerIDs, 2, 120, 0)

	channels := map[int]struct{}{}
	for _, e := range edges {
		if levels[e.To] > levels[e.From]+1 {
			if y, ok := l.skipChannels[key(e)]; ok {
				channels[y] = struct{}{}
			}
		}
	}
	if len(channels) > 3 {
		t.Fatalf("expected pooled skip channels (<=3), got %d channels: %+v", len(channels), channels)
	}
}

func TestRenderHorizontalDAG_CJKWidthAlignment(t *testing.T) {
	nodes := map[string]dagNode{
		"A": {ID: "A", Title: "设计协作流程"},
		"B": {ID: "B", Title: "实现渲染修复"},
		"C": {ID: "C", Title: "验证发布"},
	}
	edges := []dagEdge{
		{From: "A", To: "B", Type: "hard"},
		{From: "B", To: "C", Type: "hard"},
	}
	levels := map[string]int{"A": 0, "B": 1, "C": 2}
	layerIDs := map[int][]string{0: {"A"}, 1: {"B"}, 2: {"C"}}

	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 2, "", 80, 8)
	plain := stripANSI(out)
	plainText := strings.ReplaceAll(plain, string(wideRuneCont), "")

	for _, title := range []string{"设计协作流程", "实现渲染修复", "验证发布"} {
		if !strings.Contains(plainText, title) {
			t.Fatalf("expected CJK title %q in output:\n%s", title, plain)
		}
	}

	for _, line := range strings.Split(plain, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.ContainsRune(line, '\x00') {
			t.Fatalf("unexpected NUL rune in rendered line: %q", line)
		}
		if ansi.StringWidth(line) > 80 {
			t.Fatalf("rendered line exceeds width 80: width=%d line=%q", ansi.StringWidth(line), line)
		}
	}
}

func TestDagGridPutStr_WideRuneOccupiesCells(t *testing.T) {
	g := newGrid(8, 1)
	g.putStr(0, 0, "设计")

	// Each Han rune is 2 cells wide, so two runes should occupy 4 grid cells.
	for x := 0; x < 4; x++ {
		if g.get(x, 0) == 0 {
			t.Fatalf("expected cell %d to be occupied for wide runes, grid=%v", x, g.cells[0])
		}
	}
}
