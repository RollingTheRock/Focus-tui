package main

import (
	"fmt"

	"focus/internal/ui/layout"
)

func main() {
	dims := layout.Compute(100, 24)
	fmt.Printf("TwoCol=%v LeftW=%d RightW=%d ContentH=%d\n", dims.TwoCol, dims.LeftW, dims.RightW, dims.ContentH)

	todoContent := "▸ ○ Write documentation\n  ○ Fix login bug\n  ✓ Review PR #42\n  ! Pay electric bill"
	pomoContent := "🍅 --:--\n\nIdle\n\n○ ○ ○ ○   Cycle 0/4"

	todoPanel := layout.RenderPanel("TODO [today]", todoContent, dims.LeftW, dims.ContentH, true)
	pomoPanel := layout.RenderPanel("POMODORO", pomoContent, dims.RightW, dims.ContentH, false)

	fmt.Println("\n=== Todo Panel ===")
	fmt.Println(todoPanel)
	fmt.Println("\n=== Pomo Panel ===")
	fmt.Println(pomoPanel)

	dims2 := layout.Compute(80, 24)
	fmt.Printf("\n80col: TwoCol=%v LeftW=%d TopH=%d BottomH=%d\n", dims2.TwoCol, dims2.LeftW, dims2.TopH, dims2.BottomH)
}
