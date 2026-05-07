package events

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── Smoke Tests ──

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("sub-1", nil)
	defer bus.Unsubscribe("sub-1")

	ev := Event{EventType: TaskCreated, AggregateID: "t1"}
	bus.Publish(ev)

	select {
	case got := <-ch:
		if got.EventType != TaskCreated || got.AggregateID != "t1" {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_FilteredSubscription(t *testing.T) {
	bus := NewEventBus()

	// Subscriber A: only TaskCreated events
	chA := bus.Subscribe("sub-a", func(e Event) bool {
		return e.EventType == TaskCreated
	})
	defer bus.Unsubscribe("sub-a")

	// Subscriber B: only events for aggregate t2
	chB := bus.Subscribe("sub-b", func(e Event) bool {
		return e.AggregateID == "t2"
	})
	defer bus.Unsubscribe("sub-b")

	bus.Publish(Event{EventType: TaskCreated, AggregateID: "t1"})
	bus.Publish(Event{EventType: TaskStateChanged, AggregateID: "t2"})
	bus.Publish(Event{EventType: TaskCreated, AggregateID: "t2"})

	// A should receive 2 TaskCreated events
	countA := 0
	doneA := make(chan struct{})
	go func() {
		for {
			select {
			case <-chA:
				countA++
				if countA == 2 {
					close(doneA)
					return
				}
			case <-time.After(200 * time.Millisecond):
				close(doneA)
				return
			}
		}
	}()

	// B should receive 2 events for t2
	countB := 0
	doneB := make(chan struct{})
	go func() {
		for {
			select {
			case <-chB:
				countB++
				if countB == 2 {
					close(doneB)
					return
				}
			case <-time.After(200 * time.Millisecond):
				close(doneB)
				return
			}
		}
	}()

	<-doneA
	<-doneB

	if countA != 2 {
		t.Fatalf("expected 2 events for sub-a, got %d", countA)
	}
	if countB != 2 {
		t.Fatalf("expected 2 events for sub-b, got %d", countB)
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("sub-1", nil)

	// Publish one event before unsubscribe
	bus.Publish(Event{EventType: TaskCreated, AggregateID: "pre"})

	bus.Unsubscribe("sub-1")

	// Drain any pre-unsubscribe events
	select {
	case ev := <-ch:
		if ev.AggregateID != "pre" {
			t.Fatalf("unexpected pre-unsubscribe event: %+v", ev)
		}
	case <-time.After(100 * time.Millisecond):
		// no pre-unsubscribe event
	}

	// After unsubscribe, Publish should not send to this channel
	bus.Publish(Event{EventType: TaskCreated})

	// Channel is closed; reading should return zero value immediately
	select {
	case ev := <-ch:
		if ev.EventType != "" || ev.AggregateID != "" {
			t.Fatalf("should not receive real event after unsubscribe, got: %+v", ev)
		}
		// zero value from closed channel is expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("closed channel should return immediately")
	}
}

func TestEventBus_MultipleChannelsPerSubscriber(t *testing.T) {
	bus := NewEventBus()
	ch1 := bus.Subscribe("sub-1", nil)
	ch2 := bus.Subscribe("sub-1", nil)
	defer bus.Unsubscribe("sub-1")

	bus.Publish(Event{EventType: TaskCreated})

	// Both channels should receive the event
	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("timeout ch1")
	}
	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("timeout ch2")
	}
}

// ── Stress Tests ──

func TestEventBus_StressManySubscribers(t *testing.T) {
	bus := NewEventBus()
	const numSubs = 25
	const numEvents = 1000

	var counters []atomic.Int64
	var chans []<-chan Event

	for i := 0; i < numSubs; i++ {
		ch := bus.Subscribe("sub-"+string(rune('a'+i%26)), nil)
		chans = append(chans, ch)
		counters = append(counters, atomic.Int64{})
		go func(idx int, c <-chan Event) {
			for range c {
				counters[idx].Add(1)
			}
		}(i, ch)
	}
	defer func() {
		for i := 0; i < numSubs; i++ {
			bus.Unsubscribe("sub-" + string(rune('a'+i%26)))
		}
	}()

	// Give goroutines time to start before the burst
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	for i := 0; i < numEvents; i++ {
		bus.Publish(Event{EventType: TaskCreated, AggregateID: "t" + string(rune('0'+i%10))})
	}
	took := time.Since(start)

	// Wait for all subscribers to process
	time.Sleep(200 * time.Millisecond)

	for i := 0; i < numSubs; i++ {
		if got := counters[i].Load(); got != int64(numEvents) {
			t.Fatalf("subscriber %d expected %d events, got %d", i, numEvents, got)
		}
	}

	t.Logf("Stress: %d events × %d subscribers = %d deliveries in %v (%.0f events/sec)",
		numEvents, numSubs, numEvents*numSubs, took, float64(numEvents)/took.Seconds())
}

func TestEventBus_StressFilteredSubscribers(t *testing.T) {
	bus := NewEventBus()
	const numEvents = 5000
	const numTaskSubs = 20
	const numWorktreeSubs = 20

	var taskCounter atomic.Int64
	var worktreeCounter atomic.Int64

	for i := 0; i < numTaskSubs; i++ {
		ch := bus.Subscribe("task-sub-"+string(rune('a'+i%26)), func(e Event) bool {
			return e.AggregateType == AggregateTask
		})
		go func(c <-chan Event) {
			for range c {
				taskCounter.Add(1)
			}
		}(ch)
	}
	for i := 0; i < numWorktreeSubs; i++ {
		ch := bus.Subscribe("wt-sub-"+string(rune('a'+i%26)), func(e Event) bool {
			return e.AggregateType == AggregateWorktree
		})
		go func(c <-chan Event) {
			for range c {
				worktreeCounter.Add(1)
			}
		}(ch)
	}
	defer func() {
		for i := 0; i < numTaskSubs; i++ {
			bus.Unsubscribe("task-sub-" + string(rune('a'+i%26)))
		}
		for i := 0; i < numWorktreeSubs; i++ {
			bus.Unsubscribe("wt-sub-" + string(rune('a'+i%26)))
		}
	}()

	start := time.Now()
	for i := 0; i < numEvents; i++ {
		if i%2 == 0 {
			bus.Publish(Event{EventType: TaskCreated, AggregateType: AggregateTask, AggregateID: "t1"})
		} else {
			bus.Publish(Event{EventType: WorktreeContextUpdated, AggregateType: AggregateWorktree, AggregateID: "wt1"})
		}
	}
	took := time.Since(start)

	time.Sleep(200 * time.Millisecond)

	expectedTask := int64(numEvents/2) * int64(numTaskSubs)
	expectedWorktree := int64(numEvents/2) * int64(numWorktreeSubs)

	if got := taskCounter.Load(); got != expectedTask {
		t.Fatalf("task subscribers expected %d events, got %d", expectedTask, got)
	}
	if got := worktreeCounter.Load(); got != expectedWorktree {
		t.Fatalf("worktree subscribers expected %d events, got %d", expectedWorktree, got)
	}

	t.Logf("Filtered stress: %d events in %v (%.0f events/sec)", numEvents, took, float64(numEvents)/took.Seconds())
}

func TestEventBus_StressConcurrentPublish(t *testing.T) {
	bus := NewEventBus()
	const numPublishers = 5
	const eventsPerPublisher = 20

	ch := bus.Subscribe("collector", nil)
	defer bus.Unsubscribe("collector")

	// Start consumer before publishers to minimize drops
	var received atomic.Int64
	var consumerDone sync.WaitGroup
	consumerDone.Add(1)
	go func() {
		defer consumerDone.Done()
		for range ch {
			received.Add(1)
		}
	}()

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerPublisher; j++ {
				bus.Publish(Event{EventType: TaskCreated, AggregateID: "pub-" + string(rune('0'+id%10))})
			}
		}(i)
	}
	wg.Wait()
	took := time.Since(start)

	// Wait for consumer to drain buffered events
	time.Sleep(200 * time.Millisecond)

	total := numPublishers * eventsPerPublisher
	got := received.Load()
	// With concurrent publish and a 64-buffer, some drops are possible.
	// We assert at least 80% delivery as a reasonable threshold.
	minExpected := int64(total * 80 / 100)
	if got < minExpected {
		t.Fatalf("expected at least %d events, got %d (%.1f%% delivery)", minExpected, got, float64(got)*100/float64(total))
	}

	t.Logf("Concurrent publish stress: %d events from %d publishers in %v (%.0f events/sec, %d/%d delivered = %.1f%%)",
		total, numPublishers, took, float64(total)/took.Seconds(), got, total, float64(got)*100/float64(total))
}
