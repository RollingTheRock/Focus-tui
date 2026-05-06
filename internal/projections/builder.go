package projections

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"focus/internal/events"
	"focus/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Builder consumes events from the Event Store and materializes projection tables.
// It runs as a background goroutine.
type Builder struct {
	pool       *pgxpool.Pool
	events     *store.EventStore
	consumerID string
	stopCh     chan struct{}
}

// NewBuilder creates a projection builder.
func NewBuilder(pool *pgxpool.Pool, events *store.EventStore) *Builder {
	return &Builder{
		pool:       pool,
		events:     events,
		consumerID: "projection-main",
		stopCh:     make(chan struct{}),
	}
}

// Run starts the background event consumption loop.
// It should be launched as a goroutine.
func (b *Builder) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-ticker.C:
			if err := b.processBatch(ctx); err != nil {
				log.Printf("[projection-builder] batch failed: %v", err)
			}
		}
	}
}

// Stop signals the builder to stop.
func (b *Builder) Stop() {
	close(b.stopCh)
}

func (b *Builder) processBatch(ctx context.Context) error {
	const batchSize = 100
	evs, err := b.events.ReadEventsAfterForConsumer(ctx, b.consumerID, batchSize)
	if err != nil {
		return fmt.Errorf("read events: %w", err)
	}
	if len(evs) == 0 {
		return nil
	}

	for _, ev := range evs {
		if err := b.applyEvent(ctx, ev); err != nil {
			return fmt.Errorf("apply event %d (%s): %w", ev.EventID, ev.EventType, err)
		}
		if err := b.events.SaveOffset(ctx, b.consumerID, ev.EventID); err != nil {
			return fmt.Errorf("save offset %d: %w", ev.EventID, err)
		}
	}

	log.Printf("[projection-builder] applied %d events (up to #%d)", len(evs), evs[len(evs)-1].EventID)
	return nil
}

func (b *Builder) applyEvent(ctx context.Context, ev events.Event) error {
	switch ev.EventType {
	case events.TaskCreated:
		return b.applyTaskCreated(ctx, ev)
	case events.TaskStateChanged:
		return b.applyTaskStateChanged(ctx, ev)
	case events.TaskGoalUpdated:
		return b.applyTaskGoalUpdated(ctx, ev)
	case "TodoCreated":
		return b.applyTodoCreated(ctx, ev)
	default:
		// Unknown event types are silently skipped during Phase 1.
		return nil
	}
}

func (b *Builder) applyTaskCreated(ctx context.Context, ev events.Event) error {
	var p events.TaskCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_tasks (
			id, repo_id, title, goal, state, priority,
			parent_task_id, event_version, updated_at
		) VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, NOW())
		ON CONFLICT (id) DO UPDATE SET
			repo_id = EXCLUDED.repo_id,
			title = EXCLUDED.title,
			goal = EXCLUDED.goal,
			priority = EXCLUDED.priority,
			parent_task_id = EXCLUDED.parent_task_id,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.RepoID, p.Title, p.Goal, p.Priority, p.ParentTaskID, ev.EventID)
	return err
}

func (b *Builder) applyTaskStateChanged(ctx context.Context, ev events.Event) error {
	var p events.TaskStateChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		UPDATE proj_tasks
		SET state = $1, event_version = $2, updated_at = NOW()
		WHERE id = $3
	`, p.NewState, ev.EventID, ev.AggregateID)
	return err
}

func (b *Builder) applyTaskGoalUpdated(ctx context.Context, ev events.Event) error {
	var p events.TaskGoalUpdatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		UPDATE proj_tasks
		SET goal = $1, next_step = $2, event_version = $3, updated_at = NOW()
		WHERE id = $4
	`, p.Goal, p.NextStep, ev.EventID, ev.AggregateID)
	return err
}

func (b *Builder) applyTodoCreated(ctx context.Context, ev events.Event) error {
	var p map[string]any
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	text, _ := p["text"].(string)
	list, _ := p["list"].(string)
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_todos (id, text, list, status, event_version)
		VALUES ($1, $2, $3, 'todo', $4)
		ON CONFLICT (id) DO NOTHING
	`, ev.AggregateID, text, list, ev.EventID)
	return err
}
