package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"focus/internal/events"
)

// TestOrchestrator_StressEventThroughput verifies the orchestrator handles many events.
func TestOrchestrator_StressEventThroughput(t *testing.T) {
	bus := events.NewEventBus()
	store := &fakeStore{
		downstream: map[string][]Task{},
		ready:      map[string]bool{},
	}
	o := New(store, bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Start(ctx)
	defer o.Stop()

	// Wait for subscription
	time.Sleep(100 * time.Millisecond)

	const numEvents = 5000
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			payload, _ := events.Serialize(events.TaskStateChangedPayload{PreviousState: "active", NewState: "done"})
			bus.Publish(events.Event{
				EventType:     events.TaskStateChanged,
				AggregateID:   fmt.Sprintf("task-%d", id%100),
				AggregateType: events.AggregateTask,
				Payload:       payload,
			})
		}(i)
	}
	wg.Wait()
	took := time.Since(start)

	// Give orchestrator time to process
	time.Sleep(200 * time.Millisecond)

	t.Logf("Orchestrator stress: %d events published in %v (%.0f events/sec)",
		numEvents, took, float64(numEvents)/took.Seconds())
}

// TestOrchestrator_StressFilteredSubscription verifies filtered events don't overwhelm.
func TestOrchestrator_StressFilteredSubscription(t *testing.T) {
	bus := events.NewEventBus()
	store := &fakeStore{
		downstream: map[string][]Task{},
		ready:      map[string]bool{},
	}
	o := New(store, bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Start(ctx)
	defer o.Stop()

	time.Sleep(100 * time.Millisecond)

	const numTaskEvents = 2000
	const numOtherEvents = 3000

	start := time.Now()
	for i := 0; i < numTaskEvents; i++ {
		payload, _ := events.Serialize(events.TaskStateChangedPayload{PreviousState: "active", NewState: "done"})
		bus.Publish(events.Event{
			EventType:     events.TaskStateChanged,
			AggregateID:   fmt.Sprintf("task-%d", i),
			AggregateType: events.AggregateTask,
			Payload:       payload,
		})
	}
	for i := 0; i < numOtherEvents; i++ {
		payload, _ := events.Serialize(events.AgentSessionHeartbeatPayload{State: "running"})
		bus.Publish(events.Event{
			EventType:     events.AgentSessionHeartbeat,
			AggregateID:   fmt.Sprintf("session-%d", i),
			AggregateType: events.AggregateAgentSession,
			Payload:       payload,
		})
	}
	took := time.Since(start)

	time.Sleep(200 * time.Millisecond)

	t.Logf("Orchestrator filter stress: %d task + %d other events in %v (%.0f events/sec)",
		numTaskEvents, numOtherEvents, took, float64(numTaskEvents+numOtherEvents)/took.Seconds())
}
