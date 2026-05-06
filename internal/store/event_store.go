package store

import (
	"context"
	"fmt"

	"focus/internal/events"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventStore handles append-only event persistence on PostgreSQL.
type EventStore struct {
	pool *pgxpool.Pool
	bus  *events.EventBus
}

// NewEventStore creates an EventStore backed by the given pool.
func NewEventStore(pool *pgxpool.Pool) *EventStore {
	return &EventStore{pool: pool}
}

// SetBus attaches an in-memory event bus for real-time pub/sub.
func (es *EventStore) SetBus(bus *events.EventBus) {
	es.bus = bus
}

// MigrateEventSchema creates the events table and indexes if they don't exist.
func (es *EventStore) MigrateEventSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS events (
    event_id       BIGSERIAL PRIMARY KEY,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    aggregate_type TEXT NOT NULL,
    aggregate_id   TEXT NOT NULL,
    version        BIGINT NOT NULL,
    event_type     TEXT NOT NULL,
    payload        JSONB NOT NULL,
    actor_type     TEXT,
    actor_id       TEXT,
    causation_id   BIGINT REFERENCES events(event_id),
    correlation_id TEXT,
    scope_type     TEXT,
    scope_id       TEXT,
    UNIQUE(aggregate_type, aggregate_id, version)
);

CREATE INDEX IF NOT EXISTS idx_events_aggregate
    ON events(aggregate_type, aggregate_id, version);

CREATE INDEX IF NOT EXISTS idx_events_order
    ON events(event_id);

CREATE INDEX IF NOT EXISTS idx_events_scope
    ON events(scope_type, scope_id, event_id);

CREATE INDEX IF NOT EXISTS idx_events_correlation
    ON events(correlation_id, event_id);

CREATE TABLE IF NOT EXISTS consumer_offsets (
    consumer_id    TEXT PRIMARY KEY,
    last_event_id  BIGINT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ DEFAULT NOW()
);
`
	_, err := es.pool.Exec(ctx, schema)
	return err
}

// AppendEvent inserts a single event into the store.
// Version is auto-incremented per aggregate to enforce optimistic concurrency.
// It returns the generated event_id or an error (including version conflicts).
func (es *EventStore) AppendEvent(ctx context.Context, ev events.Event) (int64, error) {
	var causationID *int64
	if ev.CausationID != nil {
		v := *ev.CausationID
		causationID = &v
	}

	var eventID int64
	err := es.pool.QueryRow(ctx, `
		INSERT INTO events (
			occurred_at, aggregate_type, aggregate_id, version,
			event_type, payload, actor_type, actor_id,
			causation_id, correlation_id, scope_type, scope_id
		) VALUES (
			$1, $2, $3,
			COALESCE((SELECT MAX(version) FROM events WHERE aggregate_type = $2 AND aggregate_id = $3), 0) + 1,
			$4, $5, $6, $7, $8, $9, $10, $11
		)
		RETURNING event_id
	`,
		ev.OccurredAt, ev.AggregateType, ev.AggregateID,
		ev.EventType, ev.Payload, ev.ActorType, ev.ActorID,
		causationID, ev.CorrelationID, ev.ScopeType, ev.ScopeID,
	).Scan(&eventID)

	if err != nil {
		return 0, fmt.Errorf("append event: %w", err)
	}
	// Publish to in-memory bus for real-time consumers (ProjectionBuilder, Orchestrator, TUI).
	if es.bus != nil {
		ev.EventID = eventID
		es.bus.Publish(ev)
	}
	return eventID, nil
}

// ReadEventsAfter fetches events strictly after the given event_id, up to batchSize.
func (es *EventStore) ReadEventsAfter(ctx context.Context, afterEventID int64, batchSize int) ([]events.Event, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchSize > 1000 {
		batchSize = 1000
	}

	rows, err := es.pool.Query(ctx, `
		SELECT event_id, occurred_at, aggregate_type, aggregate_id, version,
		       event_type, payload, actor_type, actor_id,
		       causation_id, correlation_id, scope_type, scope_id
		FROM events
		WHERE event_id > $1
		ORDER BY event_id ASC
		LIMIT $2
	`, afterEventID, batchSize)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[events.Event])
}

// ReadEventsAfterForConsumer is a convenience wrapper that reads from a consumer's last offset.
func (es *EventStore) ReadEventsAfterForConsumer(ctx context.Context, consumerID string, batchSize int) ([]events.Event, error) {
	offset, err := es.GetOffset(ctx, consumerID)
	if err != nil {
		return nil, err
	}
	return es.ReadEventsAfter(ctx, offset, batchSize)
}

// SaveOffset persists a consumer's last processed event_id.
func (es *EventStore) SaveOffset(ctx context.Context, consumerID string, eventID int64) error {
	_, err := es.pool.Exec(ctx, `
		INSERT INTO consumer_offsets (consumer_id, last_event_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (consumer_id)
		DO UPDATE SET last_event_id = EXCLUDED.last_event_id, updated_at = NOW()
	`, consumerID, eventID)
	if err != nil {
		return fmt.Errorf("save offset: %w", err)
	}
	return nil
}

// GetOffset retrieves a consumer's last processed event_id (0 if never set).
func (es *EventStore) GetOffset(ctx context.Context, consumerID string) (int64, error) {
	var offset int64
	err := es.pool.QueryRow(ctx, `
		SELECT last_event_id FROM consumer_offsets WHERE consumer_id = $1
	`, consumerID).Scan(&offset)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get offset: %w", err)
	}
	return offset, nil
}

// GetLatestEventID returns the highest event_id currently in the store.
func (es *EventStore) GetLatestEventID(ctx context.Context) (int64, error) {
	var id *int64
	err := es.pool.QueryRow(ctx, `SELECT MAX(event_id) FROM events`).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get latest event id: %w", err)
	}
	if id == nil {
		return 0, nil
	}
	return *id, nil
}

// GetRecentEventsForScope returns the most recent events matching a scope,
// ordered newest first.
func (es *EventStore) GetRecentEventsForScope(ctx context.Context, scopeType, scopeID string, limit int) ([]events.Event, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := es.pool.Query(ctx, `
		SELECT event_id, occurred_at, aggregate_type, aggregate_id, version,
		       event_type, payload, actor_type, actor_id,
		       causation_id, correlation_id, scope_type, scope_id
		FROM events
		WHERE scope_type = $1 AND scope_id = $2
		ORDER BY event_id DESC
		LIMIT $3
	`, scopeType, scopeID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent events: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[events.Event])
}

// ReadEventsInRange fetches events in a closed range [from, to] ordered by event_id.
func (es *EventStore) ReadEventsInRange(ctx context.Context, from, to int64) ([]events.Event, error) {
	rows, err := es.pool.Query(ctx, `
		SELECT event_id, occurred_at, aggregate_type, aggregate_id, version,
		       event_type, payload, actor_type, actor_id,
		       causation_id, correlation_id, scope_type, scope_id
		FROM events
		WHERE event_id >= $1 AND event_id <= $2
		ORDER BY event_id ASC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("read events range: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[events.Event])
}
