package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/models"
)

// TestStore_StressBatchTaskCreation creates 1000 tasks sequentially.
func TestStore_StressBatchTaskCreation(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	const numTasks = 1000
	start := time.Now()
	for i := 0; i < numTasks; i++ {
		record := models.TaskContextRecord{
			ID:     fmt.Sprintf("task-%d", i),
			RepoID: "repo-1",
			Title:  fmt.Sprintf("Task %d", i),
			State:  "active",
		}
		if err := s.SaveTaskContext(record); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}
	}
	took := time.Since(start)

	// Verify count
	records, err := s.ListTaskContexts("repo-1")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(records) != numTasks {
		t.Fatalf("expected %d tasks, got %d", numTasks, len(records))
	}

	t.Logf("Store batch stress: %d tasks created in %v (%.0f tasks/sec, %.3f ms/task)",
		numTasks, took, float64(numTasks)/took.Seconds(), float64(took.Milliseconds())/float64(numTasks))
}

// TestStore_StressConcurrentTaskCreation creates tasks from multiple goroutines.
func TestStore_StressConcurrentTaskCreation(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	const numGoroutines = 50
	const tasksPerGoroutine = 100
	var wg sync.WaitGroup
	start := time.Now()

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < tasksPerGoroutine; i++ {
				record := models.TaskContextRecord{
					ID:     fmt.Sprintf("g%d-task-%d", gid, i),
					RepoID: "repo-1",
					Title:  fmt.Sprintf("Task %d", i),
					State:  "active",
				}
				if err := s.SaveTaskContext(record); err != nil {
					t.Errorf("save task g%d-%d: %v", gid, i, err)
				}
			}
		}(g)
	}
	wg.Wait()
	took := time.Since(start)

	total := numGoroutines * tasksPerGoroutine
	records, err := s.ListTaskContexts("repo-1")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(records) != total {
		t.Fatalf("expected %d tasks, got %d", total, len(records))
	}

	t.Logf("Store concurrent stress: %d tasks from %d goroutines in %v (%.0f tasks/sec)",
		total, numGoroutines, took, float64(total)/took.Seconds())
}

// TestStore_StressMixedOperations interleaves CRUD operations.
func TestStore_StressMixedOperations(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	const numTasks = 100
	// Seed
	for i := 0; i < numTasks; i++ {
		s.SaveTaskContext(models.TaskContextRecord{
			ID:     fmt.Sprintf("task-%d", i),
			RepoID: "repo-1",
			Title:  fmt.Sprintf("Task %d", i),
			State:  "active",
		})
	}

	var wg sync.WaitGroup
	start := time.Now()

	// Concurrent updates
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			taskID := fmt.Sprintf("task-%d", id)
			record, _ := s.GetTaskContext(taskID)
			if record == nil {
				return
			}
			record.State = "done"
			record.Goal = fmt.Sprintf("updated goal %d", id)
			s.SaveTaskContext(*record)
		}(i)
	}
	wg.Wait()
	took := time.Since(start)

	// Verify
	doneCount := 0
	records, _ := s.ListTaskContexts("repo-1")
	for _, r := range records {
		if r.State == "done" {
			doneCount++
		}
	}
	if doneCount != numTasks {
		t.Fatalf("expected %d done tasks, got %d", numTasks, doneCount)
	}

	t.Logf("Store mixed stress: %d concurrent updates in %v (%.0f ops/sec)",
		numTasks, took, float64(numTasks)/took.Seconds())
}
