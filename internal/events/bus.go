package events

import (
	"sync"
)

// EventFilter is a predicate that decides whether a subscriber receives an event.
// A nil filter means "receive all events".
type EventFilter func(Event) bool

// subscription pairs a channel with its filter.
type subscription struct {
	ch     chan Event
	filter EventFilter
}

// EventBus is a lightweight in-memory pub/sub system for domain events.
// It is used to decouple event producers (EventStore) from consumers
// (ProjectionBuilder, Orchestrator, TUI, Piggyback cache).
type EventBus struct {
	subscribers map[string][]subscription
	mu          sync.RWMutex
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]subscription),
	}
}

// Subscribe returns a channel that receives events matching the optional filter.
// If filter is nil, the subscriber receives every published event.
// The subscriberID is used only for grouping; multiple calls with the
// same ID will create independent channels. Close the channel when done.
func (eb *EventBus) Subscribe(subscriberID string, filter EventFilter) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, 64)
	eb.subscribers[subscriberID] = append(eb.subscribers[subscriberID], subscription{ch: ch, filter: filter})
	return ch
}

// Unsubscribe closes and removes all channels for the given subscriberID.
func (eb *EventBus) Unsubscribe(subscriberID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for _, sub := range eb.subscribers[subscriberID] {
		close(sub.ch)
	}
	delete(eb.subscribers, subscriberID)
}

// Publish sends an event to all subscribers whose filter accepts it.
// Non-blocking: if a subscriber's channel is full, the event is dropped
// for that subscriber (they should consume faster or use a larger buffer).
func (eb *EventBus) Publish(e Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, subs := range eb.subscribers {
		for _, sub := range subs {
			if sub.filter != nil && !sub.filter(e) {
				continue
			}
			select {
			case sub.ch <- e:
			default:
				// channel full, drop event for this subscriber
			}
		}
	}
}
