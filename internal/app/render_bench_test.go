package app

import (
	"fmt"
	"testing"
)

// makeBenchDAG constructs a synthetic DAG with n nodes arranged in a chain
// with some skip-level edges for realism.
func makeBenchDAG(n int) (map[string]dagNode, []dagEdge, map[string]int, map[int][]string, int) {
	nodes := make(map[string]dagNode, n)
	edges := make([]dagEdge, 0, n)
	adjacency := make(map[string][]dagEdge)
	indegree := make(map[string]int)

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("task-%04d", i)
		nodes[id] = dagNode{
			ID:       id,
			Title:    fmt.Sprintf("Task %d", i),
			State:    "active",
			Priority: "medium",
		}
	}

	for i := 0; i < n; i++ {
		from := fmt.Sprintf("task-%04d", i)
		// chain edge to next node
		if i+1 < n {
			to := fmt.Sprintf("task-%04d", i+1)
			e := dagEdge{From: from, To: to, Type: "hard"}
			edges = append(edges, e)
			adjacency[from] = append(adjacency[from], e)
			indegree[to]++
		}
		// occasional skip-level edge
		if i+3 < n && i%5 == 0 {
			to := fmt.Sprintf("task-%04d", i+3)
			e := dagEdge{From: from, To: to, Type: "hard"}
			edges = append(edges, e)
			adjacency[from] = append(adjacency[from], e)
			indegree[to]++
		}
	}

	levels := computeDAGLevels(nodes, adjacency, indegree)
	maxLevel := 0
	for _, lv := range levels {
		if lv > maxLevel {
			maxLevel = lv
		}
	}
	layerIDs := make(map[int][]string, maxLevel+1)
	for id, lv := range levels {
		layerIDs[lv] = append(layerIDs[lv], id)
	}
	return nodes, edges, levels, layerIDs, maxLevel
}

func BenchmarkRenderDAG10(b *testing.B) {
	nodes, edges, levels, layerIDs, maxLevel := makeBenchDAG(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderHorizontalDAG(nodes, edges, levels, layerIDs, maxLevel, "task-0000", 120, 40)
	}
}

func BenchmarkRenderDAG50(b *testing.B) {
	nodes, edges, levels, layerIDs, maxLevel := makeBenchDAG(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderHorizontalDAG(nodes, edges, levels, layerIDs, maxLevel, "task-0000", 120, 40)
	}
}

func BenchmarkRenderDAG100(b *testing.B) {
	nodes, edges, levels, layerIDs, maxLevel := makeBenchDAG(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderHorizontalDAG(nodes, edges, levels, layerIDs, maxLevel, "task-0000", 120, 40)
	}
}

func BenchmarkRenderDAG200(b *testing.B) {
	nodes, edges, levels, layerIDs, maxLevel := makeBenchDAG(200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderHorizontalDAG(nodes, edges, levels, layerIDs, maxLevel, "task-0000", 120, 40)
	}
}
