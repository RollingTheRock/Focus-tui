package app

import (
	"strings"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

func TestBuildDAG_FiltersOutSteps(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	repoID := "/repo/main"

	phaseA := models.TaskContextRecord{ID: "phase-a", RepoID: repoID, Title: "Phase A", State: "active"}
	stepA1 := models.TaskContextRecord{ID: "step-a1", RepoID: repoID, Title: "Step A1", State: "active", ParentTaskID: stringPtr("phase-a")}
	phaseB := models.TaskContextRecord{ID: "phase-b", RepoID: repoID, Title: "Phase B", State: "active"}

	for _, task := range []models.TaskContextRecord{phaseA, stepA1, phaseB} {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, repoID, nil)
	p.buildDAG()

	if len(p.nodes) != 2 {
		t.Fatalf("expected 2 phase nodes, got %d", len(p.nodes))
	}
	if _, ok := p.nodes["phase-a"]; !ok {
		t.Error("expected phase-a in nodes")
	}
	if _, ok := p.nodes["phase-b"]; !ok {
		t.Error("expected phase-b in nodes")
	}
	if _, ok := p.nodes["step-a1"]; ok {
		t.Error("expected step-a1 to be filtered out")
	}
}

func TestBuildDAG_DerivesPhaseDependenciesFromSteps(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	repoID := "/repo/main"

	phaseA := models.TaskContextRecord{ID: "phase-a", RepoID: repoID, Title: "Phase A", State: "done"}
	stepA1 := models.TaskContextRecord{ID: "step-a1", RepoID: repoID, Title: "Step A1", State: "done", ParentTaskID: stringPtr("phase-a")}
	phaseB := models.TaskContextRecord{ID: "phase-b", RepoID: repoID, Title: "Phase B", State: "active"}
	stepB1 := models.TaskContextRecord{ID: "step-b1", RepoID: repoID, Title: "Step B1", State: "active", ParentTaskID: stringPtr("phase-b")}

	for _, task := range []models.TaskContextRecord{phaseA, stepA1, phaseB, stepB1} {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	// Step A1 depends on Step B1 => Phase A depends on Phase B
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{FromTaskID: "step-a1", ToTaskID: "step-b1", DependencyType: "hard"}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, repoID, nil)
	p.buildDAG()

	if len(p.nodes) != 2 {
		t.Fatalf("expected 2 phase nodes, got %d", len(p.nodes))
	}
	if len(p.edges) != 1 {
		t.Fatalf("expected 1 phase edge, got %d", len(p.edges))
	}
	if p.edges[0].From != "phase-a" || p.edges[0].To != "phase-b" {
		t.Fatalf("expected edge phase-a -> phase-b, got %s -> %s", p.edges[0].From, p.edges[0].To)
	}
}

func TestBuildDAG_DerivesPhaseDependenciesFromPhaseToStep(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	repoID := "/repo/main"

	phaseA := models.TaskContextRecord{ID: "phase-a", RepoID: repoID, Title: "Phase A", State: "active"}
	phaseB := models.TaskContextRecord{ID: "phase-b", RepoID: repoID, Title: "Phase B", State: "active"}
	stepB1 := models.TaskContextRecord{ID: "step-b1", RepoID: repoID, Title: "Step B1", State: "active", ParentTaskID: stringPtr("phase-b")}

	for _, task := range []models.TaskContextRecord{phaseA, phaseB, stepB1} {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	// Phase A depends on Step B1 => Phase A depends on Phase B
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{FromTaskID: "phase-a", ToTaskID: "step-b1", DependencyType: "hard"}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, repoID, nil)
	p.buildDAG()

	if len(p.edges) != 1 {
		t.Fatalf("expected 1 phase edge, got %d", len(p.edges))
	}
	if p.edges[0].From != "phase-a" || p.edges[0].To != "phase-b" {
		t.Fatalf("expected edge phase-a -> phase-b, got %s -> %s", p.edges[0].From, p.edges[0].To)
	}
}

func TestBuildDAG_DeduplicatesPhaseEdges(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	repoID := "/repo/main"

	phaseA := models.TaskContextRecord{ID: "phase-a", RepoID: repoID, Title: "Phase A", State: "active"}
	stepA1 := models.TaskContextRecord{ID: "step-a1", RepoID: repoID, Title: "Step A1", State: "active", ParentTaskID: stringPtr("phase-a")}
	stepA2 := models.TaskContextRecord{ID: "step-a2", RepoID: repoID, Title: "Step A2", State: "active", ParentTaskID: stringPtr("phase-a")}
	phaseB := models.TaskContextRecord{ID: "phase-b", RepoID: repoID, Title: "Phase B", State: "active"}
	stepB1 := models.TaskContextRecord{ID: "step-b1", RepoID: repoID, Title: "Step B1", State: "active", ParentTaskID: stringPtr("phase-b")}
	stepB2 := models.TaskContextRecord{ID: "step-b2", RepoID: repoID, Title: "Step B2", State: "active", ParentTaskID: stringPtr("phase-b")}

	for _, task := range []models.TaskContextRecord{phaseA, stepA1, stepA2, phaseB, stepB1, stepB2} {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	// Multiple steps in Phase A depend on multiple steps in Phase B
	for _, from := range []string{"step-a1", "step-a2"} {
		for _, to := range []string{"step-b1", "step-b2"} {
			if err := st.SaveTaskDependency(models.TaskDependencyRecord{FromTaskID: from, ToTaskID: to, DependencyType: "hard"}); err != nil {
				t.Fatalf("save dependency %s->%s: %v", from, to, err)
			}
		}
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, repoID, nil)
	p.buildDAG()

	if len(p.edges) != 1 {
		t.Fatalf("expected 1 deduplicated phase edge, got %d", len(p.edges))
	}
	if p.edges[0].From != "phase-a" || p.edges[0].To != "phase-b" {
		t.Fatalf("expected edge phase-a -> phase-b, got %s -> %s", p.edges[0].From, p.edges[0].To)
	}
}

func TestBuildDAG_CursorOnlyPhases(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	repoID := "/repo/main"

	// Create phase first, then step (FK constraint requires parent to exist)
	phaseA := models.TaskContextRecord{ID: "phase-a", RepoID: repoID, Title: "Phase A", State: "active"}
	stepA1 := models.TaskContextRecord{ID: "step-a1", RepoID: repoID, Title: "Step A1", State: "active", ParentTaskID: stringPtr("phase-a")}

	for _, task := range []models.TaskContextRecord{phaseA, stepA1} {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, repoID, nil)
	p.buildDAG()

	if p.cursorNode != "phase-a" {
		t.Fatalf("expected cursor on phase-a, got %s", p.cursorNode)
	}
}

func TestDAGPaneViewRefreshesWhenNodeContentChanges(t *testing.T) {
	cm := &models.CommonModel{}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, "/repo/main", nil)
	p.SetSize(100, 12)
	p.nodes = map[string]dagNode{
		"task-1": {ID: "task-1", Title: "Alpha", State: "active", Priority: "medium"},
	}
	p.levels = map[string]int{"task-1": 0}
	p.layerIDs = map[int][]string{0: []string{"task-1"}}
	p.cursorNode = "task-1"

	first := p.View().Content
	if !strings.Contains(first, "Alpha") {
		t.Fatalf("expected first render to contain Alpha, got:\n%s", first)
	}

	p.nodes["task-1"] = dagNode{ID: "task-1", Title: "Beta", State: "active", Priority: "medium"}
	second := p.View().Content
	if !strings.Contains(second, "Beta") {
		t.Fatalf("expected second render to contain updated title Beta, got:\n%s", second)
	}
	if strings.Contains(second, "Alpha") {
		t.Fatalf("expected stale title Alpha to disappear, got:\n%s", second)
	}
}
