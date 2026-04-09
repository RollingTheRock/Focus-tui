package layout

import (
	"math"
	"sort"
	"strings"

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
		firstH, secondH := splitSpan(bounds.H, desiredFirst, minSubtreeHeight(node.First), minSubtreeHeight(node.Second))
		return models.PaneFrame{X: bounds.X, Y: bounds.Y, W: bounds.W, H: firstH},
			models.PaneFrame{X: bounds.X, Y: bounds.Y + firstH, W: bounds.W, H: secondH}
	default:
		desiredFirst := int(math.Round(float64(bounds.W) * ratio))
		firstW, secondW := splitSpan(bounds.W, desiredFirst, minSubtreeWidth(node.First), minSubtreeWidth(node.Second))
		return models.PaneFrame{X: bounds.X, Y: bounds.Y, W: firstW, H: bounds.H},
			models.PaneFrame{X: bounds.X + firstW, Y: bounds.Y, W: secondW, H: bounds.H}
	}
}

func splitSpan(total, desiredFirst, minFirst, minSecond int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if total == 1 {
		return 1, 0
	}
	if total < minFirst+minSecond {
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
	if desiredFirst < minFirst {
		desiredFirst = minFirst
	}
	if desiredFirst > total-minSecond {
		desiredFirst = total - minSecond
	}
	return desiredFirst, total - desiredFirst
}

func minSubtreeWidth(node *TreeNode) int {
	if node == nil {
		return 1
	}
	if node.PaneID != "" {
		return minPaneWidth(node.PaneID)
	}
	if node.Direction == SplitHorizontal {
		return minSubtreeWidth(node.First) + minSubtreeWidth(node.Second)
	}
	return maxInt(minSubtreeWidth(node.First), minSubtreeWidth(node.Second))
}

func minSubtreeHeight(node *TreeNode) int {
	if node == nil {
		return 1
	}
	if node.PaneID != "" {
		return minPaneHeight(node.PaneID)
	}
	if node.Direction == SplitVertical {
		return minSubtreeHeight(node.First) + minSubtreeHeight(node.Second)
	}
	return maxInt(minSubtreeHeight(node.First), minSubtreeHeight(node.Second))
}

func minPaneWidth(id models.PaneID) int {
	switch paneTypeForID(id) {
	case models.PaneTypeShell:
		return 20
	case models.PaneTypeTodo, models.PaneTypePomodoro:
		return 15
	default:
		return 12
	}
}

func minPaneHeight(id models.PaneID) int {
	switch paneTypeForID(id) {
	case models.PaneTypeShell:
		return 5
	case models.PaneTypeTodo, models.PaneTypePomodoro:
		return 4
	default:
		return 3
	}
}

func paneTypeForID(id models.PaneID) models.PaneType {
	name := string(id)
	switch {
	case strings.HasPrefix(name, string(models.PaneTypeShell)):
		return models.PaneTypeShell
	case strings.HasPrefix(name, string(models.PaneTypeTodo)):
		return models.PaneTypeTodo
	case strings.HasPrefix(name, string(models.PaneTypePomodoro)):
		return models.PaneTypePomodoro
	case strings.HasPrefix(name, string(models.PaneTypeHeader)):
		return models.PaneTypeHeader
	case strings.HasPrefix(name, string(models.PaneTypeFooter)):
		return models.PaneTypeFooter
	default:
		return ""
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

// CloseFocusFallback picks the closest neighboring pane for focus after a pane closes.
func CloseFocusFallback(current models.PaneID, frames map[models.PaneID]models.PaneFrame) models.PaneID {
	currentFrame, ok := frames[current]
	if !ok {
		return current
	}

	directions := []FocusDirection{FocusLeft, FocusRight, FocusUp, FocusDown}
	bestID := current
	bestScore := 0
	found := false

	for _, direction := range directions {
		candidate := MoveFocus(current, frames, direction)
		if candidate == current {
			continue
		}
		score := closeFallbackScore(currentFrame, frames[candidate], direction)
		if !found || score < bestScore {
			bestID = candidate
			bestScore = score
			found = true
		}
	}

	return bestID
}

func closeFallbackScore(current, candidate models.PaneFrame, direction FocusDirection) int {
	switch direction {
	case FocusLeft:
		return axisGap(current.X, candidate.X+candidate.W)*1000 + rangeGap(current.Y, current.Y+current.H, candidate.Y, candidate.Y+candidate.H)
	case FocusRight:
		return axisGap(candidate.X, current.X+current.W)*1000 + rangeGap(current.Y, current.Y+current.H, candidate.Y, candidate.Y+candidate.H)
	case FocusUp:
		return axisGap(current.Y, candidate.Y+candidate.H)*1000 + rangeGap(current.X, current.X+current.W, candidate.X, candidate.X+candidate.W)
	default:
		return axisGap(candidate.Y, current.Y+current.H)*1000 + rangeGap(current.X, current.X+current.W, candidate.X, candidate.X+candidate.W)
	}
}

func axisGap(start, end int) int {
	if start <= end {
		return 0
	}
	return start - end
}

func rangeGap(startA, endA, startB, endB int) int {
	if endA <= startB {
		return startB - endA
	}
	if endB <= startA {
		return startA - endB
	}
	return 0
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
	minFirst := minSubtreeWidth(node.First)
	minSecond := minSubtreeWidth(node.Second)
	currentFirst := firstBounds.W
	if direction == SplitVertical {
		span = bounds.H
		minFirst = minSubtreeHeight(node.First)
		minSecond = minSubtreeHeight(node.Second)
		currentFirst = firstBounds.H
	}
	if span <= 1 {
		return true, false
	}

	currentRatio := int(math.Round(float64(currentFirst) * 100 / float64(span)))
	desiredFirst := int(math.Round(float64(span) * float64(currentRatio+delta) / 100))
	first, _ := splitSpan(span, desiredFirst, minFirst, minSecond)
	if first == currentFirst {
		return true, false
	}
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
