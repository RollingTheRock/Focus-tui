package layout

import (
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

func TestComputeFramesHorizontalSplit(t *testing.T) {
	root := Split(SplitHorizontal, 70, Leaf("left"), Leaf("right"))
	frames := ComputeFrames(root, models.PaneFrame{X: 0, Y: 0, W: 100, H: 20})

	left := frames["left"]
	right := frames["right"]

	if left.W != 70 || right.W != 30 {
		t.Fatalf("expected widths 70/30, got %d/%d", left.W, right.W)
	}
	if left.X != 0 || right.X != 70 {
		t.Fatalf("expected x positions 0/70, got %d/%d", left.X, right.X)
	}
	if left.H != 20 || right.H != 20 {
		t.Fatalf("expected full height 20, got %d/%d", left.H, right.H)
	}
}

func TestComputeFramesVerticalSplitHonorsMinimumHeight(t *testing.T) {
	root := Split(SplitVertical, 50, Leaf("top"), Leaf("bottom"))
	frames := ComputeFrames(root, models.PaneFrame{X: 0, Y: 0, W: 40, H: 5})

	top := frames["top"]
	bottom := frames["bottom"]

	if top.H != 2 || bottom.H != 3 {
		t.Fatalf("expected heights 2/3 after clamp, got %d/%d", top.H, bottom.H)
	}
}

func TestComputeFramesSmallWidthStaysNonNegative(t *testing.T) {
	root := Split(SplitHorizontal, 50, Leaf("left"), Leaf("right"))
	frames := ComputeFrames(root, models.PaneFrame{X: 0, Y: 0, W: 10, H: 4})

	left := frames["left"]
	right := frames["right"]

	if left.W < 0 || right.W < 0 {
		t.Fatalf("expected non-negative widths, got %d/%d", left.W, right.W)
	}
	if left.W+right.W != 10 {
		t.Fatalf("expected total width 10, got %d", left.W+right.W)
	}
}

func TestComputeFramesHorizontalSplitUsesPaneMinimumWidths(t *testing.T) {
	root := Split(SplitHorizontal, 50, Leaf("shell-main"), Leaf("todo-main"))
	frames := ComputeFrames(root, models.PaneFrame{X: 0, Y: 0, W: 35, H: 12})

	if frames["shell-main"].W != 20 || frames["todo-main"].W != 15 {
		t.Fatalf("expected widths 20/15, got %d/%d", frames["shell-main"].W, frames["todo-main"].W)
	}
}

func TestComputeFramesVerticalSplitUsesPaneMinimumHeights(t *testing.T) {
	root := Split(SplitVertical, 50, Leaf("todo-main"), Leaf("pomodoro-main"))
	frames := ComputeFrames(root, models.PaneFrame{X: 0, Y: 0, W: 40, H: 8})

	if frames["todo-main"].H != 4 || frames["pomodoro-main"].H != 4 {
		t.Fatalf("expected heights 4/4, got %d/%d", frames["todo-main"].H, frames["pomodoro-main"].H)
	}
}

func TestLeafOrderReturnsTraversalOrder(t *testing.T) {
	root := Split(
		SplitHorizontal,
		50,
		Leaf("shell-main"),
		Split(SplitVertical, 50, Leaf("todo-main"), Leaf("pomodoro-main")),
	)

	got := LeafOrder(root)
	want := []models.PaneID{"shell-main", "todo-main", "pomodoro-main"}

	if len(got) != len(want) {
		t.Fatalf("expected %d leaves, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected leaf %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestSplitLeafReplacesTargetLeaf(t *testing.T) {
	root := SplitLeaf(Leaf("shell-main"), "shell-main", "shell-2", SplitHorizontal, true)

	if root == nil || root.Direction != SplitHorizontal {
		t.Fatalf("expected horizontal split root")
	}
	if root.First == nil || root.First.PaneID != "shell-main" {
		t.Fatalf("expected original pane as first child")
	}
	if root.Second == nil || root.Second.PaneID != "shell-2" {
		t.Fatalf("expected new pane as second child")
	}
}

func TestRemoveLeafCollapsesParent(t *testing.T) {
	root := Split(SplitHorizontal, 50, Leaf("left"), Leaf("right"))
	root = RemoveLeaf(root, "left")

	if root == nil {
		t.Fatalf("expected remaining leaf root")
	}
	if root.PaneID != "right" {
		t.Fatalf("expected collapsed root to be right, got %q", root.PaneID)
	}
}

func TestMoveFocusSelectsNearestPaneInDirection(t *testing.T) {
	frames := map[models.PaneID]models.PaneFrame{
		"center": {X: 20, Y: 10, W: 10, H: 6},
		"left":   {X: 0, Y: 10, W: 10, H: 6},
		"right":  {X: 40, Y: 10, W: 10, H: 6},
		"up":     {X: 20, Y: 0, W: 10, H: 6},
		"down":   {X: 20, Y: 25, W: 10, H: 6},
	}

	if got := MoveFocus("center", frames, FocusLeft); got != "left" {
		t.Fatalf("expected left focus, got %q", got)
	}
	if got := MoveFocus("center", frames, FocusRight); got != "right" {
		t.Fatalf("expected right focus, got %q", got)
	}
	if got := MoveFocus("center", frames, FocusUp); got != "up" {
		t.Fatalf("expected up focus, got %q", got)
	}
	if got := MoveFocus("center", frames, FocusDown); got != "down" {
		t.Fatalf("expected down focus, got %q", got)
	}
}

func TestAdjustSplitRatioUsesNearestMatchingAncestor(t *testing.T) {
	root := Split(
		SplitHorizontal,
		70,
		Leaf("shell-main"),
		Split(SplitVertical, 50, Leaf("todo-main"), Leaf("pomodoro-main")),
	)

	changed := AdjustSplitRatio(root, models.PaneFrame{X: 0, Y: 0, W: 100, H: 40}, "todo-main", SplitVertical, 10)
	if !changed {
		t.Fatalf("expected vertical ratio adjustment to succeed")
	}
	if root.Ratio != 70 {
		t.Fatalf("expected outer horizontal ratio to stay 70, got %d", root.Ratio)
	}
	if root.Second == nil || root.Second.Ratio <= 50 {
		t.Fatalf("expected inner vertical ratio to increase, got %d", root.Second.Ratio)
	}
}

func TestAdjustSplitRatioClampsToMinimumPaneSize(t *testing.T) {
	root := Split(SplitHorizontal, 50, Leaf("left"), Leaf("right"))

	changed := AdjustSplitRatio(root, models.PaneFrame{X: 0, Y: 0, W: 24, H: 10}, "left", SplitHorizontal, -20)
	if changed {
		t.Fatalf("expected ratio adjustment to no-op at minimum width")
	}
	if root.Ratio != 50 {
		t.Fatalf("expected ratio to remain 50, got %d", root.Ratio)
	}
}

func TestAdjustSplitRatioHonorsPaneSpecificMinimumWidth(t *testing.T) {
	root := Split(SplitHorizontal, 50, Leaf("shell-main"), Leaf("todo-main"))

	changed := AdjustSplitRatio(root, models.PaneFrame{X: 0, Y: 0, W: 35, H: 10}, "shell-main", SplitHorizontal, -20)
	if changed {
		t.Fatalf("expected ratio adjustment to no-op at shell minimum width")
	}
	if root.Ratio != 50 {
		t.Fatalf("expected ratio to remain 50, got %d", root.Ratio)
	}
}

func TestCloseFocusFallbackPrefersAdjacentPane(t *testing.T) {
	frames := map[models.PaneID]models.PaneFrame{
		"left":    {X: 0, Y: 0, W: 20, H: 20},
		"closing": {X: 20, Y: 0, W: 20, H: 20},
		"right":   {X: 40, Y: 0, W: 20, H: 20},
	}

	if got := CloseFocusFallback("closing", frames); got != "left" {
		t.Fatalf("expected left neighbor fallback, got %q", got)
	}
}

func TestCloseFocusFallbackChoosesClosestPaneAcrossDirections(t *testing.T) {
	frames := map[models.PaneID]models.PaneFrame{
		"closing": {X: 20, Y: 10, W: 20, H: 20},
		"right":   {X: 40, Y: 10, W: 20, H: 20},
		"down":    {X: 20, Y: 40, W: 20, H: 20},
	}

	if got := CloseFocusFallback("closing", frames); got != "right" {
		t.Fatalf("expected closest right neighbor, got %q", got)
	}
}

func TestComputeFramesFourPaneGridStaysUsable(t *testing.T) {
	root := Split(
		SplitHorizontal,
		50,
		Split(SplitVertical, 50, Leaf("a"), Leaf("b")),
		Split(SplitVertical, 50, Leaf("c"), Leaf("d")),
	)

	frames := ComputeFrames(root, models.PaneFrame{X: 0, Y: 0, W: 120, H: 32})
	for _, id := range []models.PaneID{"a", "b", "c", "d"} {
		frame := frames[id]
		if frame.W < 24 {
			t.Fatalf("expected pane %q width >= 24, got %d", id, frame.W)
		}
		if frame.H < 6 {
			t.Fatalf("expected pane %q height >= 6, got %d", id, frame.H)
		}
	}
}
