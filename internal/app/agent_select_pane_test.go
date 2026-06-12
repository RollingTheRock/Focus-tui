package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"focus/internal/agents"
	"focus/internal/models"
	"focus/internal/store"
)

// TestAgentSelectPaneDetectsInstalledAgentOverridesStaleStore verifies that the
// agent selection pane performs a real-time PATH lookup instead of trusting the
// possibly-stale IsInstalled flag stored in the database.
func TestAgentSelectPaneDetectsInstalledAgentOverridesStaleStore(t *testing.T) {
	// Create a fake kimi binary on a temporary PATH entry so the test does not
	// depend on the host having the real agent installed.
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "kimi")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho fake"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Store marks the agent as not-installed even though it is on PATH.
	if err := st.SaveAgentDefinition(models.AgentDefinition{
		ID:           "kimi",
		Name:         "Kimi",
		Description:  "Moonshot AI",
		Binary:       "kimi",
		ProviderType: string(agents.ProviderKimi),
		Category:     "built-in",
		IsInstalled:  false,
		IsEnabled:    true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("save agent definition: %v", err)
	}

	common := models.CommonModel{Store: st}
	pane := newAgentSelectPane(paneAgentSelect, models.PaneMeta{}, common, "/tmp/wt")

	if len(pane.options) == 0 {
		t.Fatal("expected at least one agent option, got none")
	}
	found := false
	for _, opt := range pane.options {
		if opt.agentID == "kimi" {
			found = true
			if !opt.installed {
				t.Errorf("expected kimi option to be marked installed when it is on PATH")
			}
		}
	}
	if !found {
		t.Errorf("expected kimi option to be present in the agent list")
	}
}

// TestAgentSelectPaneFallbackUsesRealTimeDetection verifies that the built-in
// fallback list also reflects the current PATH rather than defaulting to
// not-found.
func TestAgentSelectPaneFallbackUsesRealTimeDetection(t *testing.T) {
	// Only kimi is faked on PATH; the other built-ins are not required.
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "kimi")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho fake"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	common := models.CommonModel{Store: st}
	pane := newAgentSelectPane(paneAgentSelect, models.PaneMeta{}, common, "/tmp/wt")

	var kimiOpt *agentOption
	for i := range pane.options {
		if pane.options[i].agentID == "kimi" {
			kimiOpt = &pane.options[i]
			break
		}
	}
	if kimiOpt == nil {
		t.Fatal("expected fallback list to contain kimi")
	}
	if !kimiOpt.installed {
		t.Errorf("expected fallback kimi option to be marked installed when it is on PATH")
	}
}
