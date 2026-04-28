package orchestrator

import (
	"errors"
	"testing"
)

type fakeStore struct {
	downstream   map[string][]Task
	ready        map[string]bool
	disconnected []string
}

func (f *fakeStore) GetDownstreamTasks(taskID string) ([]Task, error) {
	return f.downstream[taskID], nil
}

func (f *fakeStore) AllPrerequisitesMet(taskID string) (bool, error) {
	ok, exists := f.ready[taskID]
	if !exists {
		return false, nil
	}
	return ok, nil
}

func (f *fakeStore) MarkSessionDisconnected(sessionID string) error {
	if sessionID == "" {
		return errors.New("session id required")
	}
	f.disconnected = append(f.disconnected, sessionID)
	return nil
}

type fakeLauncher struct {
	launched []Task
	err      error
}

func (l *fakeLauncher) LaunchTask(task Task) error {
	if l.err != nil {
		return l.err
	}
	l.launched = append(l.launched, task)
	return nil
}

func TestOnTaskCompletedLaunchesOnlyReadyDownstream(t *testing.T) {
	store := &fakeStore{
		downstream: map[string][]Task{
			"task-A": {
				{ID: "task-B", Name: "B"},
				{ID: "task-C", Name: "C"},
			},
		},
		ready: map[string]bool{
			"task-B": true,
			"task-C": false,
		},
	}
	launcher := &fakeLauncher{}
	o := New(store, launcher)

	launched, err := o.OnTaskCompleted("task-A")
	if err != nil {
		t.Fatalf("OnTaskCompleted err: %v", err)
	}
	if len(launched) != 1 || launched[0] != "task-B" {
		t.Fatalf("expected task-B launched, got %#v", launched)
	}
	if len(launcher.launched) != 1 || launcher.launched[0].ID != "task-B" {
		t.Fatalf("unexpected launched tasks: %#v", launcher.launched)
	}
}

func TestOnHeartbeatTimeoutMarksDisconnected(t *testing.T) {
	store := &fakeStore{ready: map[string]bool{}}
	launcher := &fakeLauncher{}
	o := New(store, launcher)

	if err := o.OnHeartbeatTimeout("session-1"); err != nil {
		t.Fatalf("OnHeartbeatTimeout err: %v", err)
	}
	if len(store.disconnected) != 1 || store.disconnected[0] != "session-1" {
		t.Fatalf("expected session-1 disconnected, got %#v", store.disconnected)
	}
}
