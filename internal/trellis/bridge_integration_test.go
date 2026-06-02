package trellis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"focus/internal/models"
)

// TestBridgeIntegration exercises the real Bridge against the actual
// .trellis/ directory in the repository.  It requires:
//   - trellis CLI installed
//   - python3 >= 3.9
//   - .trellis/ initialized (run `trellis init -u dev --claude --gemini -y`)
func TestBridgeIntegration(t *testing.T) {
	repoRoot, _ := filepath.Abs("../..")
	bridge := NewBridge(repoRoot, "test-wt", nil)

	// Use a unique task ID per run so repeated tests don't collide with archived tasks.
	uniqueSuffix := fmt.Sprintf("%d", time.Now().Unix())
	testTaskID := "focus-task-test-" + uniqueSuffix

	t.Run("EnsureInitialized", func(t *testing.T) {
		if err := bridge.EnsureInitialized(); err != nil {
			t.Fatalf("EnsureInitialized: %v", err)
		}
		if !bridge.trellisReady {
			t.Fatal("expected trellisReady = true")
		}
	})

	t.Run("SyncTaskCreate", func(t *testing.T) {
		task := models.TaskContextRecord{
			ID:       testTaskID,
			RepoID:   repoRoot,
			Title:    "Integration Test Task " + uniqueSuffix,
			Goal:     "Verify Trellis sync works end-to-end",
			State:    "active",
			Priority: "high",
		}
		plan := &models.TaskPlanRecord{
			ID:       "plan-test-" + uniqueSuffix,
			Title:    "Test Plan",
			PlanBody: "1. Create task\n2. Verify files\n3. Clean up",
		}
		dir, err := bridge.SyncTaskCreate(task, plan)
		if err != nil {
			t.Fatalf("SyncTaskCreate: %v", err)
		}
		t.Logf("created trellis task dir: %s", dir)
		absDir := filepath.Join(repoRoot, dir)

		// Verify directory exists.
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			t.Fatalf("task dir does not exist: %s", absDir)
		}

		// Verify PRD was written.
		prdPath := filepath.Join(absDir, "prd.md")
		if data, err := os.ReadFile(prdPath); err != nil {
			t.Fatalf("read prd.md: %v", err)
		} else {
			if !strings.Contains(string(data), "Integration Test Task") {
				t.Fatalf("prd.md missing task title")
			}
			t.Logf("prd.md content length: %d", len(data))
		}

		// Verify mapping is registered.
		if bridge.taskSlugMap[testTaskID] == "" {
			t.Fatal("taskSlugMap not populated after SyncTaskCreate")
		}
	})

	t.Run("ResolveTaskSlug", func(t *testing.T) {
		slug, err := bridge.resolveTaskSlug(testTaskID)
		if err != nil {
			t.Fatalf("resolveTaskSlug: %v", err)
		}
		if slug == "" {
			t.Fatal("expected non-empty slug")
		}
		t.Logf("resolved slug: %s", slug)
	})

	t.Run("AddTaskOutput", func(t *testing.T) {
		err := bridge.AddTaskOutput(testTaskID, "This is a test output from Focus MCP tool.")
		if err != nil {
			t.Fatalf("AddTaskOutput: %v", err)
		}
		slug := bridge.taskSlugMap[testTaskID]
		notesPath := filepath.Join(repoRoot, slug, "notes.md")
		data, err := os.ReadFile(notesPath)
		if err != nil {
			t.Fatalf("read notes.md: %v", err)
		}
		if !strings.Contains(string(data), "test output") {
			t.Fatal("notes.md missing expected content")
		}
		t.Logf("notes.md content:\n%s", string(data))
	})

	t.Run("AddKnowledgeFact", func(t *testing.T) {
		err := bridge.AddKnowledgeFact("Focus", "integrates with", "Trellis")
		if err != nil {
			t.Fatalf("AddKnowledgeFact: %v", err)
		}
		specPath := filepath.Join(repoRoot, ".trellis", "spec", "general", "auto-discovered.md")
		data, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read auto-discovered.md: %v", err)
		}
		if !strings.Contains(string(data), "Focus integrates with Trellis") {
			t.Fatal("auto-discovered.md missing expected fact")
		}
		t.Logf("auto-discovered.md content:\n%s", string(data))
	})

	t.Run("RecordSessionByTitle", func(t *testing.T) {
		err := bridge.RecordSessionByTitle("Integration test session")
		if err != nil {
			t.Fatalf("RecordSessionByTitle: %v", err)
		}
		// Journal is written under .trellis/workspace/journal/YYYY-MM.md
		// Just verify no error for now.
		t.Log("session recorded to journal")
	})

	t.Run("UpdateWorkflowState", func(t *testing.T) {
		step := models.PlanStepRecord{
			ID:         "step-test-001",
			Title:      "Verify Bridge Integration",
			OrderIndex: 1,
			State:      "active",
			Notes:      "Running integration tests",
		}
		err := bridge.UpdateWorkflowState("plan-test-001", step)
		if err != nil {
			t.Fatalf("UpdateWorkflowState: %v", err)
		}
		wfPath := filepath.Join(repoRoot, ".trellis", "workflow.md")
		data, err := os.ReadFile(wfPath)
		if err != nil {
			t.Fatalf("read workflow.md: %v", err)
		}
		if !strings.Contains(string(data), "Verify Bridge Integration") {
			t.Fatal("workflow.md missing step title")
		}
		t.Logf("workflow.md tail:\n%s", string(data)[max(0, len(data)-500):])
	})

	t.Run("GetTaskContextExtended", func(t *testing.T) {
		ext, err := bridge.GetTaskContextExtended(testTaskID)
		if err != nil {
			t.Fatalf("GetTaskContextExtended: %v", err)
		}
		if ext == nil {
			t.Fatal("expected non-nil extended context")
		}
		t.Logf("extended context: PRD len=%d, Task=%+v", len(ext.PRD), ext.Task)
	})

	t.Run("SyncTaskStartFinishArchive", func(t *testing.T) {
		// Start
		if err := bridge.SyncTaskStart(testTaskID); err != nil {
			t.Fatalf("SyncTaskStart: %v", err)
		}
		t.Log("task started")

		// Finish
		if err := bridge.SyncTaskFinish(testTaskID); err != nil {
			t.Fatalf("SyncTaskFinish: %v", err)
		}
		t.Log("task finished")

		// Archive
		if err := bridge.SyncTaskArchive(testTaskID); err != nil {
			t.Fatalf("SyncTaskArchive: %v", err)
		}
		t.Log("task archived")
	})

	t.Run("KimiAdapterGenerated", func(t *testing.T) {
		// EnsureInitialized already ran ensureKimiAdapter.
		kimiDir := filepath.Join(repoRoot, ".kimi")
		if _, err := os.Stat(kimiDir); os.IsNotExist(err) {
			t.Fatal(".kimi/ directory not created")
		}
		hookPath := filepath.Join(kimiDir, "hooks", "session-start.py")
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Fatal("Kimi session-start hook not created")
		}
		data, _ := os.ReadFile(hookPath)
		if !strings.Contains(string(data), "get_context.py") {
			t.Fatal("Kimi hook missing get_context.py reference")
		}
		t.Logf("Kimi hook ok, length=%d", len(data))
	})
}

func TestTaskSlugFromTitle(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Fix: Login Bug!", "fix-login-bug"},
		{"  Spaces  Everywhere  ", "spaces-everywhere"},
		{"---leading-dashes", "leading-dashes"},
		{"special@#chars$$$", "specialchars"},
		{"", "task"},
		{"a", "a"},
		{strings.Repeat("a", 100), strings.Repeat("a", 50)},
	}
	for _, c := range cases {
		got := taskSlugFromTitle(c.input)
		if got != c.want {
			t.Errorf("taskSlugFromTitle(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestIsPythonVersionOK(t *testing.T) {
	cases := []struct {
		ver  string
		want bool
	}{
		{"Python 3.11.4", true},
		{"Python 3.9.0", true},
		{"Python 3.8.10", false},
		{"Python 2.7.18", false},
		{"Python 4.0.0", true},
		{"", false},
		{"3.10", false}, // missing "Python" prefix
	}
	for _, c := range cases {
		got := isPythonVersionOK(c.ver)
		if got != c.want {
			t.Errorf("isPythonVersionOK(%q) = %v, want %v", c.ver, got, c.want)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
