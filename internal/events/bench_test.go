package events

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func benchmarkEventBus(b *testing.B, numSubs, numEvents int) {
	eb := NewEventBus()

	counters := make([]*atomic.Int64, numSubs)
	for i := 0; i < numSubs; i++ {
		counters[i] = &atomic.Int64{}
	}

	for i := 0; i < numSubs; i++ {
		idx := i
		ch := eb.Subscribe(fmt.Sprintf("sub-%d", idx), nil)
		go func() {
			for range ch {
				counters[idx].Add(1)
			}
		}()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		for j := 0; j < numEvents; j++ {
			eb.Publish(Event{
				EventType:     "BenchEvent",
				AggregateType: AggregateTask,
				AggregateID:   fmt.Sprintf("task-%d", j),
			})
		}
		_ = time.Since(start)
	}

	totalEvents := int64(numEvents * b.N)
	b.ReportMetric(float64(totalEvents)/b.Elapsed().Seconds(), "events/sec")

	for i := 0; i < numSubs; i++ {
		eb.Unsubscribe(fmt.Sprintf("sub-%d", i))
	}
}

func BenchmarkEventBus100Subs10KEvents(b *testing.B)  { benchmarkEventBus(b, 100, 10000) }
func BenchmarkEventBus500Subs10KEvents(b *testing.B)  { benchmarkEventBus(b, 500, 10000) }
func BenchmarkEventBus1000Subs100KEvents(b *testing.B) { benchmarkEventBus(b, 1000, 100000) }

// BenchmarkEventBusContention measures raw throughput under high contention.
func BenchmarkEventBusContention(b *testing.B) {
	eb := NewEventBus()
	const numPublishers = 50
	const eventsPerPublisher = 10000

	ch := eb.Subscribe("collector", nil)
	var delivered atomic.Int64
	go func() {
		for range ch {
			delivered.Add(1)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for p := 0; p < numPublishers; p++ {
			wg.Add(1)
			go func(pid int) {
				defer wg.Done()
				for e := 0; e < eventsPerPublisher; e++ {
					eb.Publish(Event{
						EventType:     "ContentionEvent",
						AggregateType: AggregateTask,
						AggregateID:   fmt.Sprintf("p%d-e%d", pid, e),
					})
				}
			}(p)
		}
		wg.Wait()
	}
	total := int64(numPublishers * eventsPerPublisher * b.N)
	b.ReportMetric(float64(total)/b.Elapsed().Seconds(), "events/sec")
	eb.Unsubscribe("collector")
}
