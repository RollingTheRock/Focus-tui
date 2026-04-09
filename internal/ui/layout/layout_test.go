package layout

import (
	"strings"
	"testing"
)

func TestRenderPanelTruncatesLongTitle(t *testing.T) {
	rendered := RenderPanel("THIS IS A VERY LONG PANEL TITLE", "body", 10, 3, false)
	firstLine := strings.Split(rendered, "\n")[0]
	if strings.Contains(firstLine, "THIS IS A VERY LONG PANEL TITLE") {
		t.Fatalf("expected title to be truncated in top border")
	}
	if !strings.Contains(firstLine, "…") {
		t.Fatalf("expected truncated title to contain ellipsis")
	}
}

func TestComputeBannerDisablesBannerWhenHeightIsTooSmall(t *testing.T) {
	dims := ComputeBanner(80, 7)

	if dims.UseBanner {
		t.Fatal("expected compact header when height is below 8")
	}
	if dims.HeaderH != 1 {
		t.Fatalf("expected compact header height 1, got %d", dims.HeaderH)
	}
}
