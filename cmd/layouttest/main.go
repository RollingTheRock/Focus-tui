package main

import (
	"fmt"

	"focus/internal/ui/layout"
)

func main() {
	dims := layout.ComputeBanner(100, 24)
	fmt.Printf("UseBanner=%v ShellW=%d ShellH=%d HeaderH=%d\n",
		dims.UseBanner, dims.ShellContentW, dims.ShellContentH, dims.HeaderH)

	shellContent := "$ ls -la\ntotal 48\ndrwxr-xr-x  8 user staff  256 Apr  5 22:00 .\n$ _"
	shellPanel := layout.RenderPanel("SHELL", shellContent, dims.ShellContentW, dims.ShellContentH, true)

	fmt.Println("\n=== Shell Panel ===")
	fmt.Println(shellPanel)

	// Test overlay compositing.
	todoContent := "▸ ○ Write documentation\n  ○ Fix login bug\n  ✓ Review PR #42"
	overlayPanel := layout.RenderPanel("TODO [today]", todoContent, dims.OverlayW, dims.OverlayH, true)
	combined := layout.OverlayOnBase(shellPanel, overlayPanel, dims.OverlayX, dims.OverlayY)

	fmt.Println("\n=== Shell + Todo Overlay ===")
	fmt.Println(combined)

	dims2 := layout.ComputeBanner(50, 20)
	fmt.Printf("\n50col: UseBanner=%v ShellW=%d ShellH=%d\n",
		dims2.UseBanner, dims2.ShellContentW, dims2.ShellContentH)
}
