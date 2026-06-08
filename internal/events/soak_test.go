package events

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestEventBus_SoakPublishSubscribe starts 100 subscribers and publishes
// 1M events, monitoring goroutine and memory growth.
func TestEventBus_SoakPublishSubscribe(t *testing.T) {
	eb := NewEventBus()

	const numSubs = 100
	const numEvents = 1000000

	// Channels to count deliveries
	counters := make([]*atomic.Int64, numSubs)
	for i := 0; i < numSubs; i++ {
		counters[i] = &atomic.Int64{}
	}

	// Baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	startG := runtime.NumGoroutine()
	var startM runtime.MemStats
	runtime.ReadMemStats(&startM)

	// Start subscribers
	for i := 0; i < numSubs; i++ {
		idx := i
		ch := eb.Subscribe(fmt.Sprintf("sub-%d", idx), nil)
		go func() {
			for range ch {
				counters[idx].Add(1)
			}
		}()
	}

	start := time.Now()
	for i := 0; i < numEvents; i++ {
		eb.Publish(Event{
			EventType:     "TestEvent",
			AggregateType: AggregateTask,
			AggregateID:   fmt.Sprintf("task-%d", i),
		})
	}
	took := time.Since(start)

	// Allow subscribers to drain
	time.Sleep(500 * time.Millisecond)

	// Post-soak measurements
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	endG := runtime.NumGoroutine()
	var endM runtime.MemStats
	runtime.ReadMemStats(&endM)

	totalDelivered := int64(0)
	for i := 0; i < numSubs; i++ {
		totalDelivered += counters[i].Load()
	}

	t.Logf("EventBus soak: %d events x %d subs = %d deliveries in %v (%.0f events/sec)",
		numEvents, numSubs, totalDelivered, took, float64(numEvents)/took.Seconds())
	t.Logf("Goroutines: before=%d after=%d delta=%d", startG, endG, endG-startG)
	t.Logf("HeapAlloc: before=%d after=%d delta=%d bytes", startM.HeapAlloc, endM.HeapAlloc, int64(endM.HeapAlloc)-int64(startM.HeapAlloc))
	t.Logf("HeapObjects: before=%d after=%d delta=%d", startM.HeapObjects, endM.HeapObjects, int64(endM.HeapObjects)-int64(startM.HeapObjects))

	if endG > startG+numSubs+10 {
		t.Errorf("possible goroutine leak: started with %d, ended with %d", startG, endG)
	}

	// Cleanup
	for i := 0; i < numSubs; i++ {
		eb.Unsubscribe(fmt.Sprintf("sub-%d", i))
	}
}
