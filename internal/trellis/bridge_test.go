package trellis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"focus/internal/agents"
	"focus/internal/models"
	"focus/internal/store"
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

func TestSyncTaskFinishMarksMappedTaskCompleted(t *testing.T) {
	repoRoot := t.TempDir()
	taskDir := filepath.Join(repoRoot, ".trellis", "tasks", "06-03-finish-me")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("mkdir task: %v", err)
	}
	taskJSON := `{"id":"trellis-finish","name":"06-03-finish-me","title":"Finish Me","status":"in_progress","meta":{"focus_task_id":"focus-finish"}}`
	if err := os.WriteFile(filepath.Join(taskDir, "task.json"), []byte(taskJSON), 0644); err != nil {
		t.Fatalf("write task json: %v", err)
	}

	bridge := NewBridge(repoRoot, "wt", nil)
	bridge.trellisReady = true
	bridge.pythonCmd = "python3"
	bridge.client = NewClient(repoRoot, bridge.pythonCmd)

	if err := bridge.SyncTaskFinish("focus-finish"); err != nil {
		t.Fatalf("SyncTaskFinish: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(taskDir, "task.json"))
	if err != nil {
		t.Fatalf("read task json: %v", err)
	}
	if !strings.Contains(string(data), `"status": "completed"`) {
		t.Fatalf("expected status to be completed, got %s", string(data))
	}
	if !strings.Contains(string(data), `"completedAt"`) {
		t.Fatalf("expected completedAt to be set, got %s", string(data))
	}
}

func TestUpdateWorkflowStateWritesRuntimeStateNotWorkflowSource(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".trellis"), 0755); err != nil {
		t.Fatalf("mkdir trellis: %v", err)
	}
	workflowPath := filepath.Join(repoRoot, ".trellis", "workflow.md")
	workflowSource := "# Development Workflow\n\nsource of truth\n"
	if err := os.WriteFile(workflowPath, []byte(workflowSource), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	bridge := NewBridge(repoRoot, "wt", nil)
	step := models.PlanStepRecord{
		ID:         "step-1",
		PlanID:     "plan-1",
		Title:      "Implement detail pane",
		OrderIndex: 2,
		State:      "active",
		Notes:      "Runtime state only",
	}

	if err := bridge.UpdateWorkflowState("plan-1", step); err != nil {
		t.Fatalf("UpdateWorkflowState: %v", err)
	}
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if string(data) != workflowSource {
		t.Fatalf("workflow source should not be mutated, got %q", string(data))
	}

	statePath := filepath.Join(repoRoot, ".trellis", ".runtime", "workflow-state", "plan-1.jsonl")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read runtime state: %v", err)
	}
	if !strings.Contains(string(stateData), `"title":"Implement detail pane"`) {
		t.Fatalf("runtime state missing step title: %s", string(stateData))
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
		if !strings.Contains(string(data), "Trellis Task") || !strings.Contains(string(data), "Real Task") {
			t.Fatalf("expected %s to include trellis context, got %q", rel, string(data))
		}
	}
}

func TestBridgeFullTaskWorktreeContextAndCompletionLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}
	sourceRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("source root: %v", err)
	}
	repoRoot := t.TempDir()
	worktreeDir := filepath.Join(t.TempDir(), "feature-worktree")

	setupTrellisFixture(t, sourceRoot, repoRoot)

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	task := models.TaskContextRecord{
		ID:       "focus-loop-task",
		RepoID:   repoRoot,
		Title:    "完整闭环任务",
		Goal:     "验证创建任务、工作树、上下文注入和完成回填",
		NextStep: "生成 agent context",
		State:    "active",
		Priority: "high",
	}
	if err := st.SaveTaskContext(task); err != nil {
		t.Fatalf("save task: %v", err)
	}
	plan := models.TaskPlanRecord{
		ID:       "plan-loop",
		TaskID:   task.ID,
		Title:    "闭环计划",
		Status:   "active",
		PlanBody: "1. 创建 Trellis task\n2. 注入 worktree context\n3. 完成任务",
	}
	if err := st.SaveTaskPlan(plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	bridge := NewBridge(repoRoot, worktreeDir, st)
	bridge.trellisReady = true
	bridge.pythonCmd = "python3"
	bridge.client = NewClient(repoRoot, bridge.pythonCmd)

	if err := bridge.EnsureWorktreeLinks(worktreeDir); err != nil {
		t.Fatalf("EnsureWorktreeLinks: %v", err)
	}
	dir, err := bridge.SyncTaskCreate(task, &plan)
	if err != nil {
		t.Fatalf("SyncTaskCreate: %v", err)
	}
	if _, err := bridge.BuildAgentContext(&agents.Session{
		WorktreeID: worktreeDir,
		RepoID:     repoRoot,
		TaskID:     task.ID,
		PlanID:     plan.ID,
	}); err != nil {
		t.Fatalf("BuildAgentContext: %v", err)
	}

	taskDir := bridge.taskDirPath(dir)
	taskJSONPath := filepath.Join(taskDir, "task.json")
	taskJSON := readJSONFile(t, taskJSONPath)
	if meta, _ := taskJSON["meta"].(map[string]any); meta["focus_task_id"] != task.ID {
		t.Fatalf("task.json missing focus_task_id mapping: %#v", taskJSON["meta"])
	}
	if status, _ := taskJSON["status"].(string); status != "planning" {
		t.Fatalf("expected new Trellis task status planning, got %q", status)
	}

	prdData, err := os.ReadFile(filepath.Join(taskDir, "prd.md"))
	if err != nil {
		t.Fatalf("read prd: %v", err)
	}
	for _, want := range []string{task.Title, task.Goal, plan.Title, "创建 Trellis task"} {
		if !strings.Contains(string(prdData), want) {
			t.Fatalf("prd.md missing %q:\n%s", want, string(prdData))
		}
	}

	linkTarget, err := os.Readlink(filepath.Join(worktreeDir, ".trellis"))
	if err != nil {
		t.Fatalf("read worktree .trellis symlink: %v", err)
	}
	if linkTarget != filepath.Join(repoRoot, ".trellis") {
		t.Fatalf("worktree .trellis link target = %q", linkTarget)
	}

	agentsMD, err := os.ReadFile(filepath.Join(worktreeDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read worktree AGENTS.md: %v", err)
	}
	for _, want := range []string{
		"Focus Task Metadata",
		task.Title,
		task.Goal,
		"Trellis Task",
		"PRD",
		"创建 Trellis task",
	} {
		if !strings.Contains(string(agentsMD), want) {
			t.Fatalf("AGENTS.md missing %q:\n%s", want, string(agentsMD))
		}
	}

	if err := bridge.SyncTaskFinish(task.ID); err != nil {
		t.Fatalf("SyncTaskFinish: %v", err)
	}
	taskJSON = readJSONFile(t, taskJSONPath)
	if status, _ := taskJSON["status"].(string); status != "completed" {
		t.Fatalf("expected completed task status, got %q", status)
	}
	if completedAt, _ := taskJSON["completedAt"].(string); completedAt == "" {
		t.Fatalf("expected completedAt to be set: %#v", taskJSON)
	}

	ext, err := bridge.GetTaskContextExtended(task.ID)
	if err != nil {
		t.Fatalf("GetTaskContextExtended: %v", err)
	}
	if ext.Task == nil || ext.Task.ID != task.ID {
		t.Fatalf("extended context task mapping lost: %#v", ext.Task)
	}
	if !strings.Contains(ext.PRD, plan.Title) {
		t.Fatalf("extended context missing PRD: %q", ext.PRD)
	}
}

func setupTrellisFixture(t *testing.T, sourceRoot, repoRoot string) {
	t.Helper()
	for _, rel := range []string{
		filepath.Join(".trellis", "scripts"),
		filepath.Join(".trellis", "spec"),
	} {
		if err := copyDir(filepath.Join(sourceRoot, rel), filepath.Join(repoRoot, rel)); err != nil {
			t.Fatalf("copy %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		filepath.Join(".trellis", "config.yaml"),
		filepath.Join(".trellis", "workflow.md"),
	} {
		data, err := os.ReadFile(filepath.Join(sourceRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		dst := filepath.Join(repoRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".trellis", ".developer"), []byte("name=tester\n"), 0644); err != nil {
		t.Fatalf("write developer: %v", err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json %s: %v", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse json %s: %v\n%s", path, err, string(data))
	}
	return obj
}
