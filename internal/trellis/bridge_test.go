package trellis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"focus/internal/agents"
	"focus/internal/models"
)

func TestEnsureWorktreeLinksReturnsErrorWhenTrellisTargetMissing(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeDir := t.TempDir()
	bridge := NewBridge(repoRoot, "wt", nil)

	if err := bridge.EnsureWorktreeLinks(worktreeDir); err == nil {
		t.Fatalf("expected missing .trellis target to return an error")
	}
	if _, err := os.Lstat(filepath.Join(worktreeDir, ".trellis")); !os.IsNotExist(err) {
		t.Fatalf("expected no .trellis link to be created, got err=%v", err)
	}
}

func TestBuildAgentContextReturnsRootFileWriteError(t *testing.T) {
	repoRoot := t.TempDir()
	scriptsDir := filepath.Join(repoRoot, ".trellis", "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("mkdir trellis scripts: %v", err)
	}
	getContext := filepath.Join(scriptsDir, "get_context.py")
	if err := os.WriteFile(getContext, []byte("#!/usr/bin/env python3\nprint('trellis context')\n"), 0755); err != nil {
		t.Fatalf("write get_context.py: %v", err)
	}
	worktreeFile := filepath.Join(repoRoot, "not-a-worktree")
	if err := os.WriteFile(worktreeFile, []byte("file"), 0644); err != nil {
		t.Fatalf("write worktree file: %v", err)
	}

	bridge := NewBridge(repoRoot, worktreeFile, nil)
	bridge.trellisReady = true
	bridge.pythonCmd = "python3"
	bridge.client = NewClient(repoRoot, bridge.pythonCmd)
	bridge.taskSlugMap["task-1"] = ".trellis/tasks/task-1"

	if _, err := bridge.BuildAgentContext(&agents.Session{WorktreeID: worktreeFile, TaskID: "task-1"}); err == nil {
		t.Fatalf("expected root file write failure to be returned")
	}
}

func TestSyncTaskCreateUsesFocusTaskIDMetadataBeforeSlugMatch(t *testing.T) {
	repoRoot := t.TempDir()
	tasksDir := filepath.Join(repoRoot, ".trellis", "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("mkdir tasks: %v", err)
	}
	writeTaskJSON := func(dir, title, focusID string) {
		taskDir := filepath.Join(tasksDir, dir)
		if err := os.MkdirAll(taskDir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		meta := ""
		if focusID != "" {
			meta = fmt.Sprintf(`, "meta": {"focus_task_id": %q}`, focusID)
		}
		data := fmt.Sprintf(`{"id": %q, "name": %q, "title": %q, "status": "planning"%s}`, dir, dir, title, meta)
		if err := os.WriteFile(filepath.Join(taskDir, "task.json"), []byte(data), 0644); err != nil {
			t.Fatalf("write task json: %v", err)
		}
	}
	writeTaskJSON("06-03-existing-task-a", "Existing Task", "other-task")
	writeTaskJSON("06-03-existing-task-b", "Existing Task", "focus-task")

	bridge := NewBridge(repoRoot, "wt", nil)
	bridge.trellisReady = true
	bridge.pythonCmd = "python3"
	bridge.client = NewClient(repoRoot, bridge.pythonCmd)

	dir, err := bridge.SyncTaskCreate(models.TaskContextRecord{
		ID:       "focus-task",
		Title:    "Existing Task",
		Priority: "medium",
	}, nil)
	if err != nil {
		t.Fatalf("SyncTaskCreate: %v", err)
	}
	if dir != "06-03-existing-task-b" {
		t.Fatalf("expected metadata-matched task, got %q", dir)
	}
}

func TestBridgePreparesRealWorktreeContextFiles(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeDir := t.TempDir()
	scriptsDir := filepath.Join(repoRoot, ".trellis", "scripts")
	taskDir := filepath.Join(repoRoot, ".trellis", "tasks", "06-03-real")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("mkdir task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "get_context.py"), []byte("#!/usr/bin/env python3\nprint('REAL TRELLIS CONTEXT')\n"), 0755); err != nil {
		t.Fatalf("write get_context.py: %v", err)
	}
	taskJSON := `{"id":"trellis-real","name":"06-03-real","title":"Real Task","status":"planning","meta":{"focus_task_id":"focus-real"}}`
	if err := os.WriteFile(filepath.Join(taskDir, "task.json"), []byte(taskJSON), 0644); err != nil {
		t.Fatalf("write task json: %v", err)
	}

	bridge := NewBridge(repoRoot, worktreeDir, nil)
	bridge.trellisReady = true
	bridge.pythonCmd = "python3"
	bridge.client = NewClient(repoRoot, bridge.pythonCmd)

	if err := bridge.EnsureWorktreeLinks(worktreeDir); err != nil {
		t.Fatalf("EnsureWorktreeLinks: %v", err)
	}
	if _, err := bridge.BuildAgentContext(&agents.Session{WorktreeID: worktreeDir, TaskID: "focus-real"}); err != nil {
		t.Fatalf("BuildAgentContext: %v", err)
	}

	linkTarget, err := os.Readlink(filepath.Join(worktreeDir, ".trellis"))
	if err != nil {
		t.Fatalf("read .trellis link: %v", err)
	}
	if linkTarget != filepath.Join(repoRoot, ".trellis") {
		t.Fatalf(".trellis link target = %q, want repo .trellis", linkTarget)
	}
	for _, rel := range []string{"AGENTS.md", "CLAUDE.md", filepath.Join(".focus", "spec", "AGENTS.md")} {
		data, err := os.ReadFile(filepath.Join(worktreeDir, rel))
		if err != nil {
			t.Fatalf("expected %s to be written: %v", rel, err)
		}
		if !strings.Contains(string(data), "REAL TRELLIS CONTEXT") {
			t.Fatalf("expected %s to include trellis context, got %q", rel, string(data))
		}
	}
}
