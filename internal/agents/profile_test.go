package agents

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"focus/internal/models"
)

func TestProfilePaths(t *testing.T) {
	p := NewProfilePaths("/tmp/wt")
	if p.BaseDir != "/tmp/wt/.focus" {
		t.Errorf("BaseDir = %q, want /tmp/wt/.focus", p.BaseDir)
	}
	if p.SpecDir != "/tmp/wt/.focus/spec" {
		t.Errorf("SpecDir = %q", p.SpecDir)
	}
}

func TestProfileManagerEnsure(t *testing.T) {
	tmpDir := t.TempDir()
	wt := filepath.Join(tmpDir, "worktree")
	pm := NewProfileManager(wt)

	if err := pm.Prepare(); err != nil {
		t.Fatalf("Prepare error: %v", err)
	}

	for _, dir := range []string{pm.Paths.SpecDir, pm.Paths.HandoffDir, pm.Paths.JournalDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("directory %s should exist: %v", dir, err)
		}
	}
}

func TestProfileManagerWriteAGENTSMD(t *testing.T) {
	tmpDir := t.TempDir()
	wt := filepath.Join(tmpDir, "worktree")
	pm := NewProfileManager(wt)

	spec := NewAgentSpec()
	spec.AddLayer(SpecLayer{Title: "General", Content: "Be nice.", Source: "repo"})
	spec.AddLayer(SpecLayer{Title: "Task", Content: "Fix bug.", Source: "worktree"})

	if err := pm.WriteAGENTSMD(spec); err != nil {
		t.Fatalf("WriteAGENTSMD error: %v", err)
	}

	data, err := os.ReadFile(pm.Paths.AGENTSMDPath())
	if err != nil {
		t.Fatalf("read AGENTS.md error: %v", err)
	}
	content := string(data)
	if !contains(content, "Focus Agent Context") {
		t.Errorf("AGENTS.md missing header")
	}
	if !contains(content, "Be nice.") {
		t.Errorf("AGENTS.md missing layer content")
	}
	if !contains(content, "Fix bug.") {
		t.Errorf("AGENTS.md missing task content")
	}
}

func TestProfileManagerWriteJournal(t *testing.T) {
	tmpDir := t.TempDir()
	wt := filepath.Join(tmpDir, "worktree")
	pm := NewProfileManager(wt)

	entry := JournalEntry{
		SessionID:    "sess-1",
		TaskTitle:    "Auth",
		StepTitle:    "Parser",
		Duration:     30 * time.Minute,
		Completed:    "Token parsing",
		NextSteps:    "Integration",
		KeyDecisions: []string{"Use jwx"},
		Timestamp:    time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC),
	}

	if err := pm.WriteJournal("dev1", entry); err != nil {
		t.Fatalf("WriteJournal error: %v", err)
	}

	data, err := os.ReadFile(pm.Paths.JournalPath("dev1"))
	if err != nil {
		t.Fatalf("read journal error: %v", err)
	}
	content := string(data)
	if !contains(content, "Auth") {
		t.Errorf("journal missing task title")
	}
	if !contains(content, "Use jwx") {
		t.Errorf("journal missing decision")
	}
}

func TestProfileManagerWriteHandoff(t *testing.T) {
	tmpDir := t.TempDir()
	wt := filepath.Join(tmpDir, "worktree")
	pm := NewProfileManager(wt)

	if err := pm.WriteHandoff("sess-1", "Completed auth parsing."); err != nil {
		t.Fatalf("WriteHandoff error: %v", err)
	}

	data, err := os.ReadFile(pm.Paths.LatestHandoffPath())
	if err != nil {
		t.Fatalf("read latest handoff error: %v", err)
	}
	if string(data) != "Completed auth parsing." {
		t.Errorf("handoff content = %q, want %q", string(data), "Completed auth parsing.")
	}
}

func TestSpecLoaderLoad(t *testing.T) {
	tmpDir := t.TempDir()
	repoSpec := filepath.Join(tmpDir, "repo", ".focus", "spec")
	os.MkdirAll(repoSpec, 0755)
	os.WriteFile(filepath.Join(repoSpec, "general.md"), []byte("Always write tests."), 0644)
	os.WriteFile(filepath.Join(repoSpec, "backend.md"), []byte("Use REST."), 0644)

	loader := NewSpecLoader(repoSpec, "")

	ctx := SpecLoadContext{
		Task: &models.TaskContextRecord{
			Title: "Implement API",
			Goal:  "Create REST endpoints",
		},
		Step: &models.PlanStepRecord{
			Title: "Write handler",
			State: "in_progress",
		},
	}

	spec, err := loader.Load(ctx)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(spec.Layers) < 3 {
		t.Fatalf("expected at least 3 layers, got %d", len(spec.Layers))
	}

	md := spec.ToMarkdown()
	if !contains(md, "Always write tests.") {
		t.Errorf("missing general spec")
	}
	if !contains(md, "Use REST.") {
		t.Errorf("missing backend spec")
	}
	if !contains(md, "Implement API") {
		t.Errorf("missing task context")
	}
	if !contains(md, "Write handler") {
		t.Errorf("missing step context")
	}
}

func TestInferDomain(t *testing.T) {
	tests := []struct {
		goal  string
		step  string
		want  string
	}{
		{"Create REST API", "Write handler", "backend"},
		{"Build React UI", "Add component", "frontend"},
		{"Add DB migration", "Create table", "database"},
		{"Setup CI pipeline", "Write yaml", "devops"},
		{"Write unit tests", "Add coverage", "testing"},
		{"Add JWT auth", "Verify token", "security"},
		{"Build CLI tool", "Parse args", "cli"},
		{"", "", ""},
	}

	for _, tt := range tests {
		ctx := SpecLoadContext{
			Task: &models.TaskContextRecord{Goal: tt.goal},
			Step: &models.PlanStepRecord{Title: tt.step},
		}
		got := inferDomain(ctx)
		if got != tt.want {
			t.Errorf("inferDomain(%q, %q) = %q, want %q", tt.goal, tt.step, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || len(s) > 0 && containsInternal(s, substr))
}

func containsInternal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
