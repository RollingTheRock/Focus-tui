package layout

import (
	"encoding/json"
	"fmt"

	"focus/internal/models"
)

type SerializedNode struct {
	PaneID    string          `json:"paneID,omitempty"`
	Direction SplitDirection  `json:"direction,omitempty"`
	Ratio     int             `json:"ratio,omitempty"`
	First     *SerializedNode `json:"first,omitempty"`
	Second    *SerializedNode `json:"second,omitempty"`
}

func SerializeTree(root *TreeNode) ([]byte, error) {
	if root == nil {
		return []byte("null"), nil
	}
	node := serializeNode(root)
	return json.Marshal(node)
}

func serializeNode(n *TreeNode) *SerializedNode {
	if n == nil {
		return nil
	}
	node := &SerializedNode{
		PaneID:    string(n.PaneID),
		Direction: n.Direction,
		Ratio:     n.Ratio,
	}
	if n.First != nil {
		node.First = serializeNode(n.First)
	}
	if n.Second != nil {
		node.Second = serializeNode(n.Second)
	}
	return node
}

func DeserializeTree(data []byte) (*TreeNode, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if string(data) == "null" {
		return nil, nil
	}
	var node SerializedNode
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("deserialize tree: %w", err)
	}
	return deserializeNode(&node), nil
}

func deserializeNode(n *SerializedNode) *TreeNode {
	if n == nil {
		return nil
	}
	node := &TreeNode{
		PaneID:    models.PaneID(n.PaneID),
		Direction: n.Direction,
		Ratio:     n.Ratio,
	}
	if n.First != nil {
		node.First = deserializeNode(n.First)
	}
	if n.Second != nil {
		node.Second = deserializeNode(n.Second)
	}
	return node
}
