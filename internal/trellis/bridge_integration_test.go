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
		wfPath := filepath.Join(repoRoot, ".trellis", "workflow.md")
		before, err := os.ReadFile(wfPath)
		if err != nil {
			t.Fatalf("read workflow.md before update: %v", err)
		}
		step := models.PlanStepRecord{
			ID:         "step-test-001",
			Title:      "Verify Bridge Integration",
			OrderIndex: 1,
			State:      "active",
			Notes:      "Running integration tests",
		}
		err = bridge.UpdateWorkflowState("plan-test-001", step)
		if err != nil {
			t.Fatalf("UpdateWorkflowState: %v", err)
		}
		after, err := os.ReadFile(wfPath)
		if err != nil {
			t.Fatalf("read workflow.md after update: %v", err)
		}
		if string(after) != string(before) {
			t.Fatal("workflow.md should remain source-only and not receive runtime step state")
		}

		statePath := filepath.Join(repoRoot, ".trellis", ".runtime", "workflow-state", "plan-test-001.jsonl")
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("read runtime workflow state: %v", err)
		}
		if !strings.Contains(string(data), `"title":"Verify Bridge Integration"`) {
			t.Fatal("runtime workflow state missing step title")
		}
		t.Logf("workflow runtime state tail:\n%s", string(data)[max(0, len(data)-500):])
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

// TestEnsureWorktreeLinksIntegration verifies that EnsureWorktreeLinks
// correctly creates symlinks for .trellis/ and platform config directories
// in a git worktree subdirectory, and handles pre-existing directories.
func TestEnsureWorktreeLinksIntegration(t *testing.T) {
	// Use the real repo root which already has .trellis/ initialized.
	repoRoot, _ := filepath.Abs("../..")
	bridge := NewBridge(repoRoot, "test-wt-links", nil)

	// Create a temp worktree directory under /tmp.
	worktreeDir := t.TempDir()

	t.Run("CreatesTrellisSymlink", func(t *testing.T) {
		if err := bridge.EnsureWorktreeLinks(worktreeDir); err != nil {
			t.Fatalf("EnsureWorktreeLinks: %v", err)
		}
		link := filepath.Join(worktreeDir, ".trellis")
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf(".trellis symlink not created: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf(".trellis is not a symlink")
		}
		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("readlink .trellis: %v", err)
		}
		want := filepath.Join(repoRoot, ".trellis")
		if target != want {
			t.Fatalf(".trellis symlink target = %q, want %q", target, want)
		}
		t.Logf(".trellis symlink -> %s", target)
	})

	t.Run("ReplacesExistingDirectoryWithSymlink", func(t *testing.T) {
		// Simulate trellis init creating a standalone .trellis/ inside a worktree.
		wt2 := t.TempDir()
		staleTrellis := filepath.Join(wt2, ".trellis")
		if err := os.MkdirAll(filepath.Join(staleTrellis, "scripts"), 0755); err != nil {
			t.Fatalf("mkdir stale .trellis: %v", err)
		}
		if err := os.WriteFile(filepath.Join(staleTrellis, "scripts", "dummy.py"), []byte("pass"), 0644); err != nil {
			t.Fatalf("write dummy file: %v", err)
		}

		bridge2 := NewBridge(repoRoot, "test-wt-stale", nil)
		if err := bridge2.EnsureWorktreeLinks(wt2); err != nil {
			t.Fatalf("EnsureWorktreeLinks: %v", err)
		}

		// .trellis should now be a symlink, not a directory.
		info, err := os.Lstat(staleTrellis)
		if err != nil {
			t.Fatalf("stale .trellis lstat: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("stale .trellis was not replaced with a symlink")
		}
		target, _ := os.Readlink(staleTrellis)
		if target != filepath.Join(repoRoot, ".trellis") {
			t.Fatalf("stale .trellis symlink target wrong: %s", target)
		}
		t.Log("stale directory correctly replaced with symlink")
	})

	t.Run("UpdatesWrongSymlink", func(t *testing.T) {
		// Create a worktree with a .trellis symlink pointing to a wrong location.
		wt3 := t.TempDir()
		wrongLink := filepath.Join(wt3, ".trellis")
		wrongTarget := "/dev/null"
		if err := os.Symlink(wrongTarget, wrongLink); err != nil {
			t.Fatalf("create wrong symlink: %v", err)
		}

		bridge3 := NewBridge(repoRoot, "test-wt-wrong", nil)
		if err := bridge3.EnsureWorktreeLinks(wt3); err != nil {
			t.Fatalf("EnsureWorktreeLinks: %v", err)
		}

		// The symlink should now point to the correct target.
		info, err := os.Lstat(wrongLink)
		if err != nil {
			t.Fatalf("wrongLink lstat: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("wrongLink is not a symlink after update")
		}
		target, _ := os.Readlink(wrongLink)
		if target != filepath.Join(repoRoot, ".trellis") {
			t.Fatalf("wrongLink target = %q, want repoRoot/.trellis", target)
		}
		t.Log("wrong symlink correctly updated")
	})

	t.Run("PlatformConfigSymlinks", func(t *testing.T) {
		// Verify that at least one platform config symlink is created
		// if the corresponding directory exists in the repo root.
		for _, dir := range []string{".kimi", ".claude", ".codex", ".gemini", ".opencode"} {
			target := filepath.Join(repoRoot, dir)
			if _, err := os.Stat(target); os.IsNotExist(err) {
				continue // platform not initialized, skip
			}
			link := filepath.Join(worktreeDir, dir)
			info, err := os.Lstat(link)
			if err != nil {
				t.Fatalf("%s symlink not created: %v", dir, err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s is not a symlink", dir)
			}
			gotTarget, _ := os.Readlink(link)
			if gotTarget != target {
				t.Fatalf("%s symlink target = %q, want %q", dir, gotTarget, target)
			}
			t.Logf("%s symlink -> %s", dir, gotTarget)
		}
	})
}
