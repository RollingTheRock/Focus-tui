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
