package app

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/commands"
	"github.com/RollingTheRock/Focus-tui/internal/events"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// TestE2E_CommandStorm runs a full-stack command storm: create, update,
// render DAG, and delete tasks via the command bus and event bus.
func TestE2E_CommandStorm(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer s.Close()
	s.SetMaxOpenConns(1)

	bus := commands.NewBus(s)
	eb := s.EventBus()
	if eb == nil {
		eb = events.NewEventBus()
	}

	// Collect events
	var eventCount int64
	ch := eb.Subscribe("e2e-collector", nil)
	go func() {
		for range ch {
			eventCount++
		}
	}()
	defer eb.Unsubscribe("e2e-collector")

	const numTasks = 200
	const numUpdates = 500
	const numRenders = 100
	ctx := context.Background()

	// Phase 1: Create tasks
	t.Log("Phase 1: Create tasks")
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cmd := &commands.CreateTask{
				ID:     fmt.Sprintf("e2e-task-%d", id),
				RepoID: "e2e-repo",
				Title:  fmt.Sprintf("E2E Task %d", id),
				State:  "active",
			}
			if err := bus.Send(ctx, cmd); err != nil {
				t.Errorf("create task %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	createTook := time.Since(start)

	// Phase 2: Update states and goals
	t.Log("Phase 2: Mixed updates")
	start = time.Now()
	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			taskID := fmt.Sprintf("e2e-task-%d", id%numTasks)
			if id%3 == 0 {
				cmd := &commands.UpdateTaskState{TaskID: taskID, NewState: "done"}
				_ = bus.Send(ctx, cmd)
			} else if id%3 == 1 {
				cmd := &commands.UpdateTaskGoal{TaskID: taskID, Goal: fmt.Sprintf("updated-goal-%d", id)}
				_ = bus.Send(ctx, cmd)
			} else {
				cmd := &commands.ArchiveTask{TaskID: taskID}
				_ = bus.Send(ctx, cmd)
			}
		}(i)
	}
	wg.Wait()
	updateTook := time.Since(start)

	// Phase 3: Render DAG repeatedly
	t.Log("Phase 3: Render DAG")
	tasks, _ := s.ListTaskContexts("e2e-repo")
	nodes := make(map[string]dagNode, len(tasks))
	for _, task := range tasks {
		if task.ParentTaskID != nil && *task.ParentTaskID != "" {
			continue
		}
		nodes[task.ID] = dagNode{
			ID:       task.ID,
			Title:    task.Title,
			State:    task.State,
			Priority: task.Priority,
		}
	}
	edges := make([]dagEdge, 0, len(tasks))
	adjacency := make(map[string][]dagEdge)
	indegree := make(map[string]int)
	for i := 0; i < len(tasks)-1; i++ {
		from := tasks[i].ID
		to := tasks[i+1].ID
		if _, ok := nodes[from]; ok {
			if _, ok2 := nodes[to]; ok2 {
				e := dagEdge{From: from, To: to, Type: "hard"}
				edges = append(edges, e)
				adjacency[from] = append(adjacency[from], e)
				indegree[to]++
			}
		}
	}
	levels := computeDAGLevels(nodes, adjacency, indegree)
	maxLevel := 0
	for _, lv := range levels {
		if lv > maxLevel {
			maxLevel = lv
		}
	}
	layerIDs := make(map[int][]string, maxLevel+1)
	for id, lv := range levels {
		layerIDs[lv] = append(layerIDs[lv], id)
	}

	start = time.Now()
	for i := 0; i < numRenders; i++ {
		_ = renderHorizontalDAG(nodes, edges, levels, layerIDs, maxLevel, "", 120, 40)
	}
	renderTook := time.Since(start)

	// Phase 4: Delete all tasks
	t.Log("Phase 4: Delete tasks")
	start = time.Now()
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cmd := &commands.DeleteTask{TaskID: fmt.Sprintf("e2e-task-%d", id)}
			_ = bus.Send(ctx, cmd)
		}(i)
	}
	wg.Wait()
	deleteTook := time.Since(start)

	// Memory stats
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	t.Logf("E2E Command Storm Results:")
	t.Logf("  Create %d tasks: %v (%.0f tasks/sec)", numTasks, createTook, float64(numTasks)/createTook.Seconds())
	t.Logf("  Update %d ops:   %v (%.0f ops/sec)", numUpdates, updateTook, float64(numUpdates)/updateTook.Seconds())
	t.Logf("  Render DAG %dx:  %v (%.0f renders/sec)", numRenders, renderTook, float64(numRenders)/renderTook.Seconds())
	t.Logf("  Delete %d tasks: %v (%.0f tasks/sec)", numTasks, deleteTook, float64(numTasks)/deleteTook.Seconds())
	t.Logf("  Events collected: %d", eventCount)
	t.Logf("  HeapAlloc: %d bytes, Goroutines: %d", m.HeapAlloc, runtime.NumGoroutine())
}
