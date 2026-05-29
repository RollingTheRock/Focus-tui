package app

import (
	"testing"

	"focus/internal/config"
	"focus/internal/store"
)

func TestMCPPlanCreateAndGet(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = ""
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	// Create plan
	createResult, err := m.mcpPlanCreateTool(map[string]any{
		"title":        "Build API",
		"why_now":      "User demand",
		"success":      "API deployed",
		"out_of_scope": "Frontend",
		"known_risks":  "Latency",
		"plan_body":    "1. Design schema\n2. Implement handlers",
	})
	if err != nil {
		t.Fatalf("plan.create failed: %v", err)
	}
	if !createResult["success"].(bool) {
		t.Fatal("expected success")
	}
	planID := createResult["plan_id"].(string)
	if planID == "" {
		t.Fatal("expected plan_id")
	}

	// Get plan
	getResult, err := m.mcpPlanGetTool(map[string]any{"plan_id": planID})
	if err != nil {
		t.Fatalf("plan.get failed: %v", err)
	}
	plan := getResult["plan"].(map[string]any)
	if plan["title"] != "Build API" {
		t.Fatalf("expected title 'Build API', got %q", plan["title"])
	}
	if plan["status"] != "draft" {
		t.Fatalf("expected status 'draft', got %q", plan["status"])
	}
}

func TestMCPPlanAddStepAndExpandToTasks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = ""
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	// Create plan
	createResult, _ := m.mcpPlanCreateTool(map[string]any{"title": "Two-step plan"})
	planID := createResult["plan_id"].(string)

	// Add step 0
	_, err := m.mcpPlanAddStepTool(map[string]any{
		"plan_id":     planID,
		"title":       "Step A",
		"order_index": 0,
		"notes":       "First step",
	})
	if err != nil {
		t.Fatalf("plan.add_step 0 failed: %v", err)
	}

	// Add step 1
	_, err = m.mcpPlanAddStepTool(map[string]any{
		"plan_id":     planID,
		"title":       "Step B",
		"order_index": 1,
		"notes":       "Second step",
	})
	if err != nil {
		t.Fatalf("plan.add_step 1 failed: %v", err)
	}

	// Verify steps in get
	getResult, _ := m.mcpPlanGetTool(map[string]any{"plan_id": planID})
	steps := getResult["steps"].([]map[string]any)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	// Expand to tasks
	expandResult, err := m.mcpPlanExpandToTasksTool(map[string]any{
		"plan_id": planID,
		"repo_id": "/tmp/test-repo",
	})
	if err != nil {
		t.Fatalf("plan.expand_to_tasks failed: %v", err)
	}
	if !expandResult["success"].(bool) {
		t.Fatal("expected expand success")
	}
	taskIDs := expandResult["task_ids"].([]string)
	if len(taskIDs) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(taskIDs))
	}
	deps := expandResult["dependencies"].([]map[string]any)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
}

func TestMCPDagGetStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = ""
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	// Create tasks
	rootResult, _ := m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/dag-repo", "title": "Root", "state": "done"})
	rootID := rootResult["task_id"].(string)
	childResult, _ := m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/dag-repo", "title": "Child", "state": "active"})
	childID := childResult["task_id"].(string)

	// Add dependency
	_, _ = m.mcpTaskAddDependencyTool(map[string]any{
		"from_task_id": rootID,
		"to_task_id":   childID,
	})

	// Get DAG status
	result, err := m.mcpDagGetStatusTool(map[string]any{"repo_id": "/tmp/dag-repo"})
	if err != nil {
		t.Fatalf("dag.get_status failed: %v", err)
	}
	if !result["success"].(bool) {
		t.Fatal("expected success")
	}
	nodes := result["nodes"].([]map[string]any)
	if len(nodes) < 1 {
		t.Fatalf("expected at least 1 node, got %d", len(nodes))
	}
	if result["total_count"].(int) < 1 {
		t.Fatal("expected total_count >= 1")
	}
}

func TestMCPTaskCreateWithParentTaskID(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = ""
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	// Create Phase
	phaseResult, err := m.mcpTaskCreateTool(map[string]any{
		"repo_id": "/tmp/parent-repo",
		"title":   "Phase A",
		"state":   "active",
	})
	if err != nil {
		t.Fatalf("task.create phase failed: %v", err)
	}
	phaseID := phaseResult["task_id"].(string)

	// Create Step under Phase
	stepResult, err := m.mcpTaskCreateTool(map[string]any{
		"repo_id":        "/tmp/parent-repo",
		"title":          "Step A1",
		"parent_task_id": phaseID,
	})
	if err != nil {
		t.Fatalf("task.create step failed: %v", err)
	}
	if !stepResult["success"].(bool) {
		t.Fatal("expected step create success")
	}

	// Verify step is in store with correct parent
	records, _ := st.ListTaskContexts("/tmp/parent-repo")
	var foundStep bool
	for _, r := range records {
		if r.Title == "Step A1" {
			foundStep = true
			if r.ParentTaskID == nil || *r.ParentTaskID != phaseID {
				t.Fatalf("expected parent_task_id=%q, got %v", phaseID, r.ParentTaskID)
			}
		}
	}
	if !foundStep {
		t.Fatal("step not found in store")
	}
}

func TestMCPTaskListFiltersSubtasksByDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = ""
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	// Create Phase and Step
	phaseResult, _ := m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/list-repo", "title": "Phase"})
	phaseID := phaseResult["task_id"].(string)
	_, _ = m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/list-repo", "title": "Step", "parent_task_id": phaseID})

	// Default list should only return Phase
	listResult, err := m.mcpTaskListTool(map[string]any{"repo_id": "/tmp/list-repo"})
	if err != nil {
		t.Fatalf("task.list failed: %v", err)
	}
	tasks := listResult["tasks"].([]map[string]any)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 phase task by default, got %d", len(tasks))
	}
	if tasks[0]["title"] != "Phase" {
		t.Fatalf("expected Phase, got %q", tasks[0]["title"])
	}
	if tasks[0]["parent_task_id"] != nil {
		t.Fatalf("expected nil parent_task_id for phase")
	}

	// With include_subtasks=true, should return both
	listResult2, err := m.mcpTaskListTool(map[string]any{"repo_id": "/tmp/list-repo", "include_subtasks": true})
	if err != nil {
		t.Fatalf("task.list with include_subtasks failed: %v", err)
	}
	tasks2 := listResult2["tasks"].([]map[string]any)
	if len(tasks2) != 2 {
		t.Fatalf("expected 2 tasks with include_subtasks, got %d", len(tasks2))
	}

	var foundStep bool
	for _, task := range tasks2 {
		if task["title"] == "Step" {
			foundStep = true
			if task["parent_task_id"] != phaseID {
				t.Fatalf("expected parent_task_id=%q for step, got %v", phaseID, task["parent_task_id"])
			}
		}
	}
	if !foundStep {
		t.Fatal("expected Step in full list")
	}
}

func TestMCPDagGetStatusPhaseLevelDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.MCPPort = ""
	st, _ := store.New(":memory:")
	defer st.Close()

	m := New(cfg, st).(model)
	defer m.closeShellPanes()

	// Create two Phases
	phaseA, _ := m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/dag-phase", "title": "Phase A"})
	phaseAID := phaseA["task_id"].(string)
	phaseB, _ := m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/dag-phase", "title": "Phase B"})
	phaseBID := phaseB["task_id"].(string)

	// Create a Step under Phase A with dependency to Phase B
	stepA1, _ := m.mcpTaskCreateTool(map[string]any{"repo_id": "/tmp/dag-phase", "title": "Step A1", "parent_task_id": phaseAID})
	stepA1ID := stepA1["task_id"].(string)

	// Dependency: Step A1 -> Phase B (should collapse to Phase A -> Phase B)
	_, _ = m.mcpTaskAddDependencyTool(map[string]any{
		"from_task_id": stepA1ID,
		"to_task_id":   phaseBID,
	})

	// Default DAG should show only 2 phase nodes and 1 phase edge
	result, err := m.mcpDagGetStatusTool(map[string]any{"repo_id": "/tmp/dag-phase"})
	if err != nil {
		t.Fatalf("dag.get_status failed: %v", err)
	}
	nodes := result["nodes"].([]map[string]any)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 phase nodes, got %d", len(nodes))
	}
	edges := result["edges"].([]map[string]any)
	if len(edges) != 1 {
		t.Fatalf("expected 1 phase edge, got %d", len(edges))
	}
	if edges[0]["from"] != phaseAID || edges[0]["to"] != phaseBID {
		t.Fatalf("expected edge %s->%s, got %s->%s", phaseAID, phaseBID, edges[0]["from"], edges[0]["to"])
	}
	if result["total_count"].(int) != 2 {
		t.Fatalf("expected total_count=2, got %d", result["total_count"].(int))
	}

	// Full DAG should show all 3 nodes and 1 edge
	resultFull, err := m.mcpDagGetStatusTool(map[string]any{"repo_id": "/tmp/dag-phase", "detail": "full"})
	if err != nil {
		t.Fatalf("dag.get_status full failed: %v", err)
	}
	nodesFull := resultFull["nodes"].([]map[string]any)
	if len(nodesFull) != 3 {
		t.Fatalf("expected 3 nodes in full DAG, got %d", len(nodesFull))
	}
	edgesFull := resultFull["edges"].([]map[string]any)
	if len(edgesFull) != 1 {
		t.Fatalf("expected 1 edge in full DAG, got %d", len(edgesFull))
	}
	if edgesFull[0]["from"] != stepA1ID || edgesFull[0]["to"] != phaseBID {
		t.Fatalf("expected full edge %s->%s, got %s->%s", stepA1ID, phaseBID, edgesFull[0]["from"], edgesFull[0]["to"])
	}
	if resultFull["total_count"].(int) != 3 {
		t.Fatalf("expected total_count=3 in full mode, got %d", resultFull["total_count"].(int))
	}
}
