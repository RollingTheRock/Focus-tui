package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"focus/internal/models"
)

func TestTabContainerViaRouteToPane(t *testing.T) {
	common := &models.CommonModel{}
	dag := newDagPane(paneDAG, models.PaneMeta{}, common, "", nil)
	adr := newAdrPane(paneDAG+"-adr", models.PaneMeta{}, common)
	tc := newTabContainer(paneDAG, models.PaneMeta{}, common, dag, adr)

	p := newPage(common, nil, nil)
	p.registerPane(paneDAG, tc, models.PaneMeta{ID: paneDAG, Type: models.PaneTypeWorktree})
	p.focused = paneDAG

	if tc.activeTab != 0 {
		t.Fatalf("expected activeTab=0 initially, got %d", tc.activeTab)
	}

	// Simulate the full routeToPane flow for "A"
	cmd := p.routeToPane(paneDAG, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd from routeToPane, got %v", cmd)
	}

	retrieved := p.pane(paneDAG).(*tabContainer)
	if retrieved.activeTab != 1 {
		t.Fatalf("expected activeTab=1 after routing A, got %d", retrieved.activeTab)
	}

	// Simulate the full routeToPane flow for "T"
	cmd2 := p.routeToPane(paneDAG, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if cmd2 != nil {
		t.Fatalf("expected nil cmd from routeToPane, got %v", cmd2)
	}

	retrieved2 := p.pane(paneDAG).(*tabContainer)
	if retrieved2.activeTab != 0 {
		t.Fatalf("expected activeTab=0 after routing T, got %d", retrieved2.activeTab)
	}
}
