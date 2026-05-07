package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"focus/internal/models"
)

func TestTabContainerKeySwitching(t *testing.T) {
	common := &models.CommonModel{}
	dag := newDagPane(paneDAG, models.PaneMeta{}, common, "", nil)
	adr := newAdrPane(paneDAG+"-adr", models.PaneMeta{}, common)
	tc := newTabContainer(paneDAG, models.PaneMeta{}, common, dag, adr)

	if tc.activeTab != 0 {
		t.Fatalf("expected activeTab=0, got %d", tc.activeTab)
	}

	// Press A to switch to ADRs tab
	newPanel, cmd := tc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	newTc, ok := newPanel.(*tabContainer)
	if !ok {
		t.Fatalf("expected *tabContainer, got %T", newPanel)
	}
	if newTc.activeTab != 1 {
		t.Fatalf("expected activeTab=1 after A, got %d", newTc.activeTab)
	}

	// Press T to switch back to Tasks tab
	newPanel2, cmd2 := newTc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if cmd2 != nil {
		t.Fatalf("expected nil cmd, got %v", cmd2)
	}
	newTc2, ok2 := newPanel2.(*tabContainer)
	if !ok2 {
		t.Fatalf("expected *tabContainer, got %T", newPanel2)
	}
	if newTc2.activeTab != 0 {
		t.Fatalf("expected activeTab=0 after T, got %d", newTc2.activeTab)
	}
}
