package store

import (
	"context"
	"log"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/events"
)

// tryAppendEvent is a best-effort helper that appends an event to the Event Store.
// Failures are logged but never block the caller. It is used by all Store write
// methods during the dual-write migration phase.
func (s *Store) tryAppendEvent(aggregateType, aggregateID, eventType string, payload any, scopeType, scopeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	payloadBytes, err := events.Serialize(payload)
	if err != nil {
		log.Printf("[event-store] serialize %s payload failed: %v", eventType, err)
		return
	}

	ev := events.Event{
		OccurredAt:    time.Now(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
		ActorType:     events.ActorSystem,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
	}

	if s.events != nil {
		// PostgreSQL mode: EventStore persists and then publishes internally.
		_, err = s.events.AppendEvent(ctx, ev)
		if err != nil {
			log.Printf("[event-store] append %s event failed (non-critical): %v", eventType, err)
		}
	} else if s.bus != nil {
		// SQLite mode: no EventStore, publish directly to the bus so that
		// the Orchestrator and other real-time consumers still receive events.
		s.bus.Publish(ev)
	}
}
