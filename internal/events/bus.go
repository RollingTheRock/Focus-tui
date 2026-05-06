package events

import (
	"sync"
)

// EventBus is a lightweight in-memory pub/sub system for domain events.
// It is used to decouple event producers (EventStore) from consumers
// (ProjectionBuilder, Orchestrator, TUI, Piggyback cache).
type EventBus struct {
	subscribers map[string][]chan Event
	mu          sync.RWMutex
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan Event),
	}
}

// Subscribe returns a channel that receives all published events.
// The subscriberID is used only for grouping; multiple calls with the
// same ID will create independent channels. Close the channel when done.
func (eb *EventBus) Subscribe(subscriberID string) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, 64)
	eb.subscribers[subscriberID] = append(eb.subscribers[subscriberID], ch)
	return ch
}

// Unsubscribe closes and removes all channels for the given subscriberID.
func (eb *EventBus) Unsubscribe(subscriberID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for _, ch := range eb.subscribers[subscriberID] {
		close(ch)
	}
	delete(eb.subscribers, subscriberID)
}

// Publish sends an event to all subscribers. Non-blocking: if a subscriber's
// channel is full, the event is dropped for that subscriber (they should
// consume faster or use a larger buffer).
func (eb *EventBus) Publish(e Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, chs := range eb.subscribers {
		for _, ch := range chs {
			select {
			case ch <- e:
			default:
				// channel full, drop event for this subscriber
			}
		}
	}
}
