package layout

import (
	"testing"
)

func TestSerializeDeserializeRoundTrip(t *testing.T) {
	tree := Split(
		SplitHorizontal,
		30,
		Split(SplitVertical, 50, Leaf("git-status"), Leaf("file-tree")),
		Leaf("shell"),
	)

	data, err := SerializeTree(tree)
	if err != nil {
		t.Fatalf("serialize tree: %v", err)
	}

	restored, err := DeserializeTree(data)
	if err != nil {
		t.Fatalf("deserialize tree: %v", err)
	}

	originalLeaves := LeafOrder(tree)
	restoredLeaves := LeafOrder(restored)
	if len(originalLeaves) != len(restoredLeaves) {
		t.Fatalf("leaf count mismatch: %d vs %d", len(originalLeaves), len(restoredLeaves))
	}
	for i := range originalLeaves {
		if originalLeaves[i] != restoredLeaves[i] {
			t.Fatalf("leaf %d mismatch: %s vs %s", i, originalLeaves[i], restoredLeaves[i])
		}
	}

	if restored.Direction != SplitHorizontal {
		t.Fatalf("expected horizontal split, got %s", restored.Direction)
	}
	if restored.Ratio != 30 {
		t.Fatalf("expected ratio 30, got %d", restored.Ratio)
	}
	if restored.First.Ratio != 50 {
		t.Fatalf("expected first child ratio 50, got %d", restored.First.Ratio)
	}
}

func TestSerializeNilTree(t *testing.T) {
	data, err := SerializeTree(nil)
	if err != nil {
		t.Fatalf("serialize nil: %v", err)
	}
	restored, err := DeserializeTree(data)
	if err != nil {
		t.Fatalf("deserialize nil: %v", err)
	}
	if restored != nil {
		t.Fatalf("expected nil tree, got %+v", restored)
	}
}
