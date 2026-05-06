package app

import (
	"fmt"
	"testing"
)

func TestRenderHorizontalDAG_Diamond(t *testing.T) {
	nodes := map[string]dagNode{
		"a": {ID: "a", Title: "Build", State: "ready"},
		"b": {ID: "b", Title: "Test", State: "active"},
		"c": {ID: "c", Title: "Deploy", State: "pending"},
		"d": {ID: "d", Title: "Monitor", State: "pending"},
	}
	edges := []dagEdge{
		{From: "a", To: "b"},
		{From: "a", To: "c"},
		{From: "b", To: "d"},
		{From: "c", To: "d"},
	}
	levels := map[string]int{
		"a": 0,
		"b": 1,
		"c": 1,
		"d": 2,
	}
	layerIDs := map[int][]string{
		0: {"a"},
		1: {"b", "c"},
		2: {"d"},
	}

	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 2, "", 120, 20)
	if out == "" {
		t.Error("expected non-empty output")
	}
	fmt.Println("=== Diamond DAG ===")
	fmt.Println(out)
}

func TestRenderHorizontalDAG_Chain(t *testing.T) {
	nodes := map[string]dagNode{
		"a": {ID: "a", Title: "A", State: ""},
		"b": {ID: "b", Title: "B", State: ""},
		"c": {ID: "c", Title: "C", State: ""},
	}
	edges := []dagEdge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
	}
	levels := map[string]int{
		"a": 0,
		"b": 1,
		"c": 2,
	}
	layerIDs := map[int][]string{
		0: {"a"},
		1: {"b"},
		2: {"c"},
	}

	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 2, "b", 120, 20)
	if out == "" {
		t.Error("expected non-empty output")
	}
	fmt.Println("=== Chain DAG ===")
	fmt.Println(out)
}

func TestRenderHorizontalDAG_SkipLevel(t *testing.T) {
	nodes := map[string]dagNode{
		"a": {ID: "a", Title: "Root", State: ""},
		"b": {ID: "b", Title: "Mid1", State: ""},
		"c": {ID: "c", Title: "Mid2", State: ""},
		"d": {ID: "d", Title: "End", State: ""},
	}
	edges := []dagEdge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "a", To: "d"}, // skip-level edge
		{From: "c", To: "d"},
	}
	levels := map[string]int{
		"a": 0,
		"b": 1,
		"c": 2,
		"d": 3,
	}
	layerIDs := map[int][]string{
		0: {"a"},
		1: {"b"},
		2: {"c"},
		3: {"d"},
	}

	out := renderHorizontalDAG(nodes, edges, levels, layerIDs, 3, "", 120, 20)
	if out == "" {
		t.Error("expected non-empty output")
	}
	fmt.Println("=== Skip-Level DAG ===")
	fmt.Println(out)
}
