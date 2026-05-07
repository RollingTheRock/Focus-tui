package commands

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"focus/internal/store"
)

func setupStressStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	// Force single connection for in-memory DB so concurrent goroutines
	// don't each get their own empty connection.
	s.SetMaxOpenConns(1)
	return s
}

// TestBus_StressConcurrentCreateTask sends 100 concurrent CreateTask commands.
func TestBus_StressConcurrentCreateTask(t *testing.T) {
	s := setupStressStore(t)
	defer s.Close()

	bus := NewBus(s)
	ctx := context.Background()

	const numGoroutines = 100
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cmd := &CreateTask{
				ID:     fmt.Sprintf("task-%d", id),
				RepoID: "repo-1",
				Title:  fmt.Sprintf("Task %d", id),
				Goal:   "stress test",
			}
			if err := bus.Send(ctx, cmd); err != nil {
				t.Errorf("create task %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	took := time.Since(start)

	// Verify all tasks exist
	for i := 0; i < numGoroutines; i++ {
		record, err := s.GetTaskContext(fmt.Sprintf("task-%d", i))
		if err != nil {
			t.Fatalf("get task %d: %v", i, err)
		}
		if record == nil {
			t.Fatalf("task-%d not found", i)
		}
	}

	t.Logf("Command stress: %d concurrent CreateTask in %v (%.0f cmds/sec)",
		numGoroutines, took, float64(numGoroutines)/took.Seconds())
}

// TestBus_StressMixedCommands interleaves different command types.
func TestBus_StressMixedCommands(t *testing.T) {
	s := setupStressStore(t)
	defer s.Close()

	bus := NewBus(s)
	ctx := context.Background()

	// Seed tasks
	for i := 0; i < 20; i++ {
		cmd := &CreateTask{ID: fmt.Sprintf("task-%d", i), RepoID: "repo-1", Title: fmt.Sprintf("Task %d", i)}
		if err := bus.Send(ctx, cmd); err != nil {
			t.Fatalf("seed task %d: %v", i, err)
		}
	}

	const numOps = 200
	var wg sync.WaitGroup
	start := time.Now()

	// Half updates, half goal changes
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			taskID := fmt.Sprintf("task-%d", id%20)
			if id%2 == 0 {
				cmd := &UpdateTaskState{TaskID: taskID, NewState: "done"}
				if err := bus.Send(ctx, cmd); err != nil {
					t.Errorf("update state %s: %v", taskID, err)
				}
			} else {
				cmd := &UpdateTaskGoal{TaskID: taskID, Goal: fmt.Sprintf("goal-%d", id)}
				if err := bus.Send(ctx, cmd); err != nil {
					t.Errorf("update goal %s: %v", taskID, err)
				}
			}
		}(i)
	}
	wg.Wait()
	took := time.Since(start)

	t.Logf("Mixed command stress: %d concurrent ops in %v (%.0f ops/sec)",
		numOps, took, float64(numOps)/took.Seconds())
}

// TestBus_StressValidationFailure ensures validation errors don't leak or deadlock.
func TestBus_StressValidationFailure(t *testing.T) {
	s := setupStressStore(t)
	defer s.Close()

	bus := NewBus(s)
	ctx := context.Background()

	const numInvalid = 1000
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numInvalid; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := &CreateTask{ID: "", RepoID: "r", Title: "t"} // invalid
			err := bus.Send(ctx, cmd)
			if err == nil || err.Error() != "task id required" {
				t.Errorf("expected validation error, got %v", err)
			}
		}()
	}
	wg.Wait()
	took := time.Since(start)

	t.Logf("Validation stress: %d invalid commands rejected in %v (%.0f ops/sec)",
		numInvalid, took, float64(numInvalid)/took.Seconds())
}
