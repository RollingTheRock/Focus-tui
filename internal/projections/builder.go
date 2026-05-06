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
	case events.TodoCreated:
		return b.applyTodoCreated(ctx, ev)
	case events.AgentSessionCreated:
		return b.applyAgentSessionCreated(ctx, ev)
	case events.AgentSessionHeartbeat:
		return b.applyAgentSessionHeartbeat(ctx, ev)
	case events.AgentSessionDisconnected:
		return b.applyAgentSessionDisconnected(ctx, ev)
	case events.WorktreeContextUpdated:
		return b.applyWorktreeContextUpdated(ctx, ev)
	case events.TaskPlanCreated:
		return b.applyTaskPlanCreated(ctx, ev)
	case events.PlanStepStateChanged:
		return b.applyPlanStepStateChanged(ctx, ev)
	case events.ContextNoteAdded:
		return b.applyContextNoteAdded(ctx, ev)
	case events.TaskDependencyAdded:
		return b.applyTaskDependencyAdded(ctx, ev)
	case events.TaskOutputAdded:
		return b.applyTaskOutputAdded(ctx, ev)
	case events.KnowledgeFactAdded:
		return b.applyKnowledgeFactAdded(ctx, ev)
	case events.SessionHandoffCreated:
		return b.applySessionHandoffCreated(ctx, ev)
	case events.TaskBriefUpdated:
		return b.applyTaskBriefUpdated(ctx, ev)
	default:
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
			parent_task_id, event_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, NOW(), NOW())
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
	var p events.TodoCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_todos (id, text, list, status, event_version)
		VALUES ($1, $2, $3, 'todo', $4)
		ON CONFLICT (id) DO NOTHING
	`, ev.AggregateID, p.Text, p.List, ev.EventID)
	return err
}

func (b *Builder) applyAgentSessionCreated(ctx context.Context, ev events.Event) error {
	var p events.AgentSessionCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_agent_sessions (
			id, provider, worktree_id, repo_id, task_id, plan_id, step_id,
			branch_snapshot, pid, state, launch_source, summary, env_snapshot,
			started_at, event_version, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active', $10, $11, $12, NOW(), $13, NOW())
		ON CONFLICT (id) DO UPDATE SET
			provider = EXCLUDED.provider,
			worktree_id = EXCLUDED.worktree_id,
			repo_id = EXCLUDED.repo_id,
			task_id = EXCLUDED.task_id,
			plan_id = EXCLUDED.plan_id,
			step_id = EXCLUDED.step_id,
			branch_snapshot = EXCLUDED.branch_snapshot,
			pid = EXCLUDED.pid,
			launch_source = EXCLUDED.launch_source,
			summary = EXCLUDED.summary,
			env_snapshot = EXCLUDED.env_snapshot,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.Provider, p.WorktreeID, p.RepoID, p.TaskID, p.PlanID, p.StepID,
		p.BranchSnapshot, p.PID, p.LaunchSource, p.Summary, p.EnvSnapshot, ev.EventID)
	return err
}

func (b *Builder) applyAgentSessionHeartbeat(ctx context.Context, ev events.Event) error {
	var p events.AgentSessionHeartbeatPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	state := p.State
	if state == "" {
		state = "active"
	}
	_, err := b.pool.Exec(ctx, `
		UPDATE proj_agent_sessions
		SET state = $1, last_heartbeat = NOW(), last_activity_at = NOW(),
		    event_version = $2, updated_at = NOW()
		WHERE id = $3
	`, state, ev.EventID, ev.AggregateID)
	return err
}

func (b *Builder) applyAgentSessionDisconnected(ctx context.Context, ev events.Event) error {
	var p events.AgentSessionDisconnectedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		UPDATE proj_agent_sessions
		SET state = 'disconnected', stop_reason = $1, ended_at = COALESCE(ended_at, NOW()),
		    event_version = $2, updated_at = NOW()
		WHERE id = $3
	`, p.Reason, ev.EventID, ev.AggregateID)
	return err
}

func (b *Builder) applyWorktreeContextUpdated(ctx context.Context, ev events.Event) error {
	var p events.WorktreeContextUpdatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_worktree_contexts (
			worktree_id, repo_id, primary_task_id, current_plan_id, task_mode, task_name,
			branch_snapshot, last_active_at, event_version, updated_at
		) VALUES ($1, $2, $3, NULL, $4, $5, $6, NOW(), $7, NOW())
		ON CONFLICT (worktree_id) DO UPDATE SET
			repo_id = EXCLUDED.repo_id,
			primary_task_id = EXCLUDED.primary_task_id,
			task_mode = EXCLUDED.task_mode,
			task_name = EXCLUDED.task_name,
			branch_snapshot = EXCLUDED.branch_snapshot,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.RepoID, p.PrimaryTaskID, p.TaskMode, p.TaskName, p.BranchSnapshot, ev.EventID)
	return err
}

func (b *Builder) applyTaskPlanCreated(ctx context.Context, ev events.Event) error {
	var p events.TaskPlanCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_task_plans (
			id, task_id, title, why_now, success, out_of_scope, known_risks,
			status, event_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', $8, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			title = EXCLUDED.title,
			why_now = EXCLUDED.why_now,
			success = EXCLUDED.success,
			out_of_scope = EXCLUDED.out_of_scope,
			known_risks = EXCLUDED.known_risks,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.TaskID, p.Title, p.WhyNow, p.Success, p.OutOfScope, p.KnownRisks, ev.EventID)
	return err
}

func (b *Builder) applyPlanStepStateChanged(ctx context.Context, ev events.Event) error {
	var p events.PlanStepStateChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_plan_steps (id, plan_id, state, expanded_task_id, event_version, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET
			plan_id = EXCLUDED.plan_id,
			state = EXCLUDED.state,
			expanded_task_id = EXCLUDED.expanded_task_id,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.PlanID, p.NewState, p.ExpandedTaskID, ev.EventID)
	return err
}

func (b *Builder) applyContextNoteAdded(ctx context.Context, ev events.Event) error {
	var p events.ContextNoteAddedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_context_notes (id, task_id, worktree_id, note_type, body, pinned, event_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			worktree_id = EXCLUDED.worktree_id,
			note_type = EXCLUDED.note_type,
			body = EXCLUDED.body,
			pinned = EXCLUDED.pinned,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.TaskID, p.WorktreeID, p.NoteType, p.Body, p.Pinned, ev.EventID)
	return err
}

func (b *Builder) applyTaskDependencyAdded(ctx context.Context, ev events.Event) error {
	var p events.TaskDependencyAddedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_task_dependencies (from_task_id, to_task_id, dependency_type, event_version, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (from_task_id, to_task_id) DO UPDATE SET
			dependency_type = EXCLUDED.dependency_type,
			event_version = EXCLUDED.event_version
	`, p.FromTaskID, p.ToTaskID, p.DependencyType, ev.EventID)
	return err
}

func (b *Builder) applyTaskOutputAdded(ctx context.Context, ev events.Event) error {
	var p events.TaskOutputAddedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_task_outputs (id, task_id, content, actor, event_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			content = EXCLUDED.content,
			actor = EXCLUDED.actor,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, ev.AggregateID, p.TaskID, p.Content, p.Actor, ev.EventID)
	return err
}

func (b *Builder) applyKnowledgeFactAdded(ctx context.Context, ev events.Event) error {
	var p events.KnowledgeFactAddedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_knowledge_facts (id, plan_id, subject, predicate, object, source, confidence, event_version, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (id) DO UPDATE SET
			plan_id = EXCLUDED.plan_id,
			subject = EXCLUDED.subject,
			predicate = EXCLUDED.predicate,
			object = EXCLUDED.object,
			source = EXCLUDED.source,
			confidence = EXCLUDED.confidence,
			event_version = EXCLUDED.event_version
	`, ev.AggregateID, p.PlanID, p.Subject, p.Predicate, p.Object, p.Source, p.Confidence, ev.EventID)
	return err
}

func (b *Builder) applySessionHandoffCreated(ctx context.Context, ev events.Event) error {
	var p events.SessionHandoffCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_session_handoffs (
			id, task_id, plan_id, session_id, done_summary, remaining_summary,
			decision_summary, uncertainty_summary, blocker_summary, entrypoint, event_version, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			plan_id = EXCLUDED.plan_id,
			session_id = EXCLUDED.session_id,
			done_summary = EXCLUDED.done_summary,
			remaining_summary = EXCLUDED.remaining_summary,
			decision_summary = EXCLUDED.decision_summary,
			uncertainty_summary = EXCLUDED.uncertainty_summary,
			blocker_summary = EXCLUDED.blocker_summary,
			entrypoint = EXCLUDED.entrypoint,
			event_version = EXCLUDED.event_version
	`, ev.AggregateID, p.TaskID, p.PlanID, p.SessionID, p.DoneSummary, p.RemainingSummary,
		p.DecisionSummary, p.UncertaintySummary, p.BlockerSummary, p.Entrypoint, ev.EventID)
	return err
}

func (b *Builder) applyTaskBriefUpdated(ctx context.Context, ev events.Event) error {
	var p events.TaskBriefUpdatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := b.pool.Exec(ctx, `
		INSERT INTO proj_task_briefs (task_id, why_now, success_criteria, out_of_scope, known_risks, event_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (task_id) DO UPDATE SET
			why_now = EXCLUDED.why_now,
			success_criteria = EXCLUDED.success_criteria,
			out_of_scope = EXCLUDED.out_of_scope,
			known_risks = EXCLUDED.known_risks,
			event_version = EXCLUDED.event_version,
			updated_at = NOW()
	`, p.TaskID, p.WhyNow, p.SuccessCriteria, p.OutOfScope, p.KnownRisks, ev.EventID)
	return err
}
