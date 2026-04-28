package store

import "testing"

func TestTaskDependenciesAndPrerequisites(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer s.Close()

	mustTask := func(id, state string) {
		t.Helper()
		if err := s.SaveTaskContext(TaskContextRecord{
			ID:       id,
			RepoID:   "/repo/main",
			Title:    id,
			State:    state,
			Priority: "medium",
		}); err != nil {
			t.Fatalf("save task %s: %v", id, err)
		}
	}
	mustTask("task-a", "done")
	mustTask("task-b", "active")
	mustTask("task-c", "paused")

	if err := s.SaveTaskDependency(TaskDependencyRecord{FromTaskID: "task-a", ToTaskID: "task-b", DependencyType: "hard"}); err != nil {
		t.Fatalf("save dep a->b: %v", err)
	}
	if err := s.SaveTaskDependency(TaskDependencyRecord{FromTaskID: "task-b", ToTaskID: "task-c", DependencyType: "hard"}); err != nil {
		t.Fatalf("save dep b->c: %v", err)
	}

	downstream, err := s.ListDownstreamTaskContexts("task-a")
	if err != nil {
		t.Fatalf("list downstream: %v", err)
	}
	if len(downstream) != 1 || downstream[0].ID != "task-b" {
		t.Fatalf("expected task-b downstream of task-a, got %+v", downstream)
	}
	upstream, err := s.ListUpstreamTaskContexts("task-b")
	if err != nil {
		t.Fatalf("list upstream: %v", err)
	}
	if len(upstream) != 1 || upstream[0].ID != "task-a" {
		t.Fatalf("expected task-a upstream of task-b, got %+v", upstream)
	}

	readyB, err := s.AreTaskPrerequisitesMet("task-b")
	if err != nil {
		t.Fatalf("check prereq task-b: %v", err)
	}
	if !readyB {
		t.Fatal("expected task-b prerequisites met")
	}

	readyC, err := s.AreTaskPrerequisitesMet("task-c")
	if err != nil {
		t.Fatalf("check prereq task-c: %v", err)
	}
	if readyC {
		t.Fatal("expected task-c prerequisites not met while task-b is active")
	}

	if err := s.SaveTaskContext(TaskContextRecord{
		ID:       "task-b",
		RepoID:   "/repo/main",
		Title:    "task-b",
		State:    "done",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("set task-b done: %v", err)
	}
	readyC, err = s.AreTaskPrerequisitesMet("task-c")
	if err != nil {
		t.Fatalf("recheck prereq task-c: %v", err)
	}
	if !readyC {
		t.Fatal("expected task-c prerequisites met after task-b done")
	}
}
