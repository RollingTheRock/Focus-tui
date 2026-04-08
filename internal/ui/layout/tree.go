package layout

import (
	"math"
	"sort"

	"focus/internal/models"
)

// SplitDirection controls how a branch node divides space.
type SplitDirection string

const (
	SplitHorizontal SplitDirection = "horizontal" // left / right
	SplitVertical   SplitDirection = "vertical"   // top / bottom
)

// TreeNode is a pane leaf or a split branch.
type TreeNode struct {
	PaneID models.PaneID

	Direction SplitDirection
	Ratio     int
	First     *TreeNode
	Second    *TreeNode
}

const (
	minHorizontalPaneWidth = 12
	minVerticalPaneHeight  = 3
)

// Leaf creates a leaf node for a pane.
func Leaf(id models.PaneID) *TreeNode {
	return &TreeNode{PaneID: id}
}

// Split creates a branch node.
func Split(direction SplitDirection, ratio int, first, second *TreeNode) *TreeNode {
	if ratio <= 0 || ratio >= 100 {
		ratio = 50
	}
	return &TreeNode{
		Direction: direction,
		Ratio:     ratio,
		First:     first,
		Second:    second,
	}
}

// ComputeFrames allocates absolute frames for every leaf in the tree.
func ComputeFrames(root *TreeNode, bounds models.PaneFrame) map[models.PaneID]models.PaneFrame {
	frames := make(map[models.PaneID]models.PaneFrame)
	computeFrames(root, bounds, frames)
	return frames
}

func computeFrames(node *TreeNode, bounds models.PaneFrame, frames map[models.PaneID]models.PaneFrame) {
	if node == nil || bounds.W <= 0 || bounds.H <= 0 {
		return
	}
	if node.PaneID != "" {
		frames[node.PaneID] = bounds
		return
	}

	firstBounds, secondBounds := splitBounds(node, bounds)
	switch node.Direction {
	case SplitVertical:
		computeFrames(node.First, firstBounds, frames)
		computeFrames(node.Second, secondBounds, frames)
	default:
		computeFrames(node.First, firstBounds, frames)
		computeFrames(node.Second, secondBounds, frames)
	}
}

func splitBounds(node *TreeNode, bounds models.PaneFrame) (models.PaneFrame, models.PaneFrame) {
	ratio := float64(node.Ratio) / 100
	switch node.Direction {
	case SplitVertical:
		desiredFirst := int(math.Round(float64(bounds.H) * ratio))
		firstH, secondH := splitSpan(bounds.H, desiredFirst, minVerticalPaneHeight)
		return models.PaneFrame{X: bounds.X, Y: bounds.Y, W: bounds.W, H: firstH},
			models.PaneFrame{X: bounds.X, Y: bounds.Y + firstH, W: bounds.W, H: secondH}
	default:
		desiredFirst := int(math.Round(float64(bounds.W) * ratio))
		firstW, secondW := splitSpan(bounds.W, desiredFirst, minHorizontalPaneWidth)
		return models.PaneFrame{X: bounds.X, Y: bounds.Y, W: firstW, H: bounds.H},
			models.PaneFrame{X: bounds.X + firstW, Y: bounds.Y, W: secondW, H: bounds.H}
	}
}

func splitSpan(total, desiredFirst, minSize int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if total == 1 {
		return 1, 0
	}
	if total < minSize*2 {
		first := total / 2
		if first < 1 {
			first = 1
		}
		second := total - first
		if second < 1 {
			second = 1
			first = total - second
		}
		return first, second
	}
	if desiredFirst < minSize {
		desiredFirst = minSize
	}
	if desiredFirst > total-minSize {
		desiredFirst = total - minSize
	}
	return desiredFirst, total - desiredFirst
}

// LeafOrder returns the visible leaves in traversal order.
func LeafOrder(root *TreeNode) []models.PaneID {
	var ids []models.PaneID
	var walk func(node *TreeNode)
	walk = func(node *TreeNode) {
		if node == nil {
			return
		}
		if node.PaneID != "" {
			ids = append(ids, node.PaneID)
			return
		}
		walk(node.First)
		walk(node.Second)
	}
	walk(root)
	return ids
}

// FocusDirection is used for directional focus movement.
type FocusDirection string

const (
	FocusLeft  FocusDirection = "left"
	FocusRight FocusDirection = "right"
	FocusUp    FocusDirection = "up"
	FocusDown  FocusDirection = "down"
)

// MoveFocus picks the nearest pane in the requested direction.
func MoveFocus(current models.PaneID, frames map[models.PaneID]models.PaneFrame, direction FocusDirection) models.PaneID {
	currentFrame, ok := frames[current]
	if !ok {
		return current
	}
	type candidate struct {
		id    models.PaneID
		score int
	}
	var options []candidate
	cx := currentFrame.X + currentFrame.W/2
	cy := currentFrame.Y + currentFrame.H/2
	for id, frame := range frames {
		if id == current {
			continue
		}
		px := frame.X + frame.W/2
		py := frame.Y + frame.H/2

		switch direction {
		case FocusLeft:
			if px >= cx {
				continue
			}
			options = append(options, candidate{id: id, score: (cx-px)*100 + abs(cy-py)})
		case FocusRight:
			if px <= cx {
				continue
			}
			options = append(options, candidate{id: id, score: (px-cx)*100 + abs(cy-py)})
		case FocusUp:
			if py >= cy {
				continue
			}
			options = append(options, candidate{id: id, score: (cy-py)*100 + abs(cx-px)})
		case FocusDown:
			if py <= cy {
				continue
			}
			options = append(options, candidate{id: id, score: (py-cy)*100 + abs(cx-px)})
		}
	}
	if len(options) == 0 {
		return current
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].score < options[j].score
	})
	return options[0].id
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// SplitLeaf replaces a leaf with a branch containing the old leaf and a new one.
func SplitLeaf(root *TreeNode, target, newID models.PaneID, direction SplitDirection, placeNewAfter bool) *TreeNode {
	if root == nil {
		return Leaf(newID)
	}
	if root.PaneID == target {
		oldLeaf := Leaf(target)
		newLeaf := Leaf(newID)
		if placeNewAfter {
			return Split(direction, 50, oldLeaf, newLeaf)
		}
		return Split(direction, 50, newLeaf, oldLeaf)
	}
	root.First = SplitLeaf(root.First, target, newID, direction, placeNewAfter)
	root.Second = SplitLeaf(root.Second, target, newID, direction, placeNewAfter)
	return root
}

// RemoveLeaf removes a leaf from the tree, collapsing parents with one child.
func RemoveLeaf(root *TreeNode, target models.PaneID) *TreeNode {
	if root == nil {
		return nil
	}
	if root.PaneID == target {
		return nil
	}
	if root.PaneID != "" {
		return root
	}
	root.First = RemoveLeaf(root.First, target)
	root.Second = RemoveLeaf(root.Second, target)
	if root.First == nil {
		return root.Second
	}
	if root.Second == nil {
		return root.First
	}
	return root
}

// AdjustSplitRatio updates the nearest matching split ancestor around target.
// Delta is expressed in ratio points and is clamped against split size bounds.
func AdjustSplitRatio(root *TreeNode, bounds models.PaneFrame, target models.PaneID, direction SplitDirection, delta int) bool {
	if root == nil || target == "" || delta == 0 {
		return false
	}
	_, changed := adjustSplitRatio(root, bounds, target, direction, delta)
	return changed
}

func adjustSplitRatio(node *TreeNode, bounds models.PaneFrame, target models.PaneID, direction SplitDirection, delta int) (bool, bool) {
	if node == nil {
		return false, false
	}
	if node.PaneID != "" {
		return node.PaneID == target, false
	}

	firstBounds, secondBounds := splitBounds(node, bounds)
	containsFirst, changed := adjustSplitRatio(node.First, firstBounds, target, direction, delta)
	if changed {
		return true, true
	}
	containsSecond, changed := adjustSplitRatio(node.Second, secondBounds, target, direction, delta)
	if changed {
		return true, true
	}

	containsTarget := containsFirst || containsSecond
	if !containsTarget || node.Direction != direction {
		return containsTarget, false
	}

	span := bounds.W
	minSize := minHorizontalPaneWidth
	if direction == SplitVertical {
		span = bounds.H
		minSize = minVerticalPaneHeight
	}
	if span <= 1 {
		return true, false
	}

	desiredFirst := int(math.Round(float64(span) * float64(node.Ratio+delta) / 100))
	first, _ := splitSpan(span, desiredFirst, minSize)
	newRatio := int(math.Round(float64(first) * 100 / float64(span)))
	if newRatio <= 0 {
		newRatio = 1
	}
	if newRatio >= 100 {
		newRatio = 99
	}
	if newRatio == node.Ratio {
		return true, false
	}
	node.Ratio = newRatio
	return true, true
}
