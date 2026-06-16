package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/RollingTheRock/Focus-tui/internal/models"
)

// TestTabContainerSurvivesBroadcast verifies that tabContainer.Update
// always returns *tabContainer (not a sub-pane) so that app.go's broadcast
// loop at the end of Update() does not replace paneDAG with *dagPane or *adrPane.
func TestTabContainerSurvivesBroadcast(t *testing.T) {
	common := &models.CommonModel{}
	dag := newDagPane(paneDAG, models.PaneMeta{}, common, "", nil)
	adr := newAdrPane(paneDAG+"-adr", models.PaneMeta{}, common, "")
	tc := newTabContainer(paneDAG, models.PaneMeta{}, common, dag, adr)

	// dagRefreshMsg must return *tabContainer, not *dagPane
	newPanel, _ := tc.Update(dagRefreshMsg{repoID: ""})
	if _, ok := newPanel.(*tabContainer); !ok {
		t.Fatalf("dagRefreshMsg: expected *tabContainer, got %T", newPanel)
	}

	// adrRefreshMsg must return *tabContainer, not *adrPane
	newPanel2, _ := tc.Update(adrRefreshMsg{})
	if _, ok := newPanel2.(*tabContainer); !ok {
		t.Fatalf("adrRefreshMsg: expected *tabContainer, got %T", newPanel2)
	}

	// KeyMsg must also return *tabContainer
	newPanel3, _ := tc.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	if _, ok := newPanel3.(*tabContainer); !ok {
		t.Fatalf("KeyMsg A: expected *tabContainer, got %T", newPanel3)
	}
	if tc.activeTab != 1 {
		t.Fatalf("expected activeTab=1, got %d", tc.activeTab)
	}
}
