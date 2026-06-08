package store

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"focus/internal/models"
)

// TestStore_SoakTaskLifecycle loops 100k Create/Update/Delete cycles and
// monitors goroutine and memory growth.
func TestStore_SoakTaskLifecycle(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	const iterations = 10000

	// Baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	startG := runtime.NumGoroutine()
	var startM runtime.MemStats
	runtime.ReadMemStats(&startM)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		id := fmt.Sprintf("soak-task-%d", i)
		// Create
		record := models.TaskContextRecord{
			ID:     id,
			RepoID: "soak-repo",
			Title:  fmt.Sprintf("Soak Task %d", i),
			State:  "active",
		}
		if err := s.SaveTaskContext(record); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}

		// Update
		record.State = "done"
		record.Goal = fmt.Sprintf("updated goal %d", i)
		if err := s.SaveTaskContext(record); err != nil {
			t.Fatalf("update task %d: %v", i, err)
		}

		// Delete (soft)
		if err := s.DeleteTaskContext(id); err != nil {
			t.Fatalf("delete task %d: %v", i, err)
		}
	}
	took := time.Since(start)

	// Post-soak measurements
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	endG := runtime.NumGoroutine()
	var endM runtime.MemStats
	runtime.ReadMemStats(&endM)

	t.Logf("Store soak: %d iterations in %v (%.0f ops/sec)", iterations, took, float64(iterations*3)/took.Seconds())
	t.Logf("Goroutines: before=%d after=%d delta=%d", startG, endG, endG-startG)
	t.Logf("HeapAlloc: before=%d after=%d delta=%d bytes", startM.HeapAlloc, endM.HeapAlloc, int64(endM.HeapAlloc)-int64(startM.HeapAlloc))
	t.Logf("HeapObjects: before=%d after=%d delta=%d", startM.HeapObjects, endM.HeapObjects, int64(endM.HeapObjects)-int64(startM.HeapObjects))

	if endG > startG+10 {
		t.Errorf("possible goroutine leak: started with %d, ended with %d", startG, endG)
	}
}
