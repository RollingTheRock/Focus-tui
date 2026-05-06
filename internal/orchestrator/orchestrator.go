package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"focus/internal/events"
)

// Task represents a downstream task that may be ready to start.
type Task struct {
	ID         string
	Name       string
	PlanID     string
	WorktreeID string
	Provider   string
}

// AgentSession represents an active agent session for heartbeat tracking.
type AgentSession struct {
	ID            string
	TaskID        string
	WorktreeID    string
	Provider      string
	LastHeartbeat *time.Time
	State         string
}

// TaskContext is a lightweight view of a task for the orchestrator.
type TaskContext struct {
	ID    string
	Title string
	State string
}

// Notification is an actionable alert produced by the orchestrator.
type Notification struct {
	Type     string // e.g. "downstream_ready", "heartbeat_timeout", "plan_completed", "task_blocked"
	Title    string
	Body     string
	TaskID   string
	Severity string // "info", "warning", "critical"
}

// Store is the subset of the data layer the orchestrator needs.
type Store interface {
	GetDownstreamTasks(taskID string) ([]Task, error)
	AllPrerequisitesMet(taskID string) (bool, error)
	MarkSessionDisconnected(sessionID string, reason string) error
	ListActiveAgentSessions() ([]AgentSession, error)
	GetTaskContext(taskID string) (*TaskContext, error)
}

// Orchestrator is a resident goroutine that consumes domain events and
// enforces workflow rules. It never auto-launches agents; it only
// notifies humans via the notification channel.
type Orchestrator struct {
	store    Store
	bus      *events.EventBus
	notifCh  chan Notification
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// New creates an orchestrator. Call Start to begin background processing.
func New(store Store, bus *events.EventBus) *Orchestrator {
	return &Orchestrator{
		store:   store,
		bus:     bus,
		notifCh: make(chan Notification, 64),
		stopCh:  make(chan struct{}),
	}
}

// Start launches the event and heartbeat loops.
func (o *Orchestrator) Start(ctx context.Context) {
	o.wg.Add(2)
	go o.eventLoop(ctx)
	go o.heartbeatLoop(ctx)
}

// Stop signals all loops to exit and waits for them.
func (o *Orchestrator) Stop() {
	close(o.stopCh)
	o.wg.Wait()
}

// Notifications returns a channel of orchestrator-generated alerts.
func (o *Orchestrator) Notifications() <-chan Notification {
	return o.notifCh
}

// --- event loop ---

func (o *Orchestrator) eventLoop(ctx context.Context) {
	defer o.wg.Done()
	if o.bus == nil {
		return
	}
	ch := o.bus.Subscribe("orchestrator")
	defer o.bus.Unsubscribe("orchestrator")

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.stopCh:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			o.handleEvent(ev)
		}
	}
}

func (o *Orchestrator) handleEvent(ev events.Event) {
	switch ev.EventType {
	case events.TaskStateChanged:
		var p events.TaskStateChangedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return
		}
		if p.NewState == "done" || p.NewState == "archived" {
			o.checkDownstream(ev.AggregateID)
		}
		if p.NewState == "blocked" {
			o.notifyBlocked(ev.AggregateID)
		}

	case events.PlanStepStateChanged:
		var p events.PlanStepStateChangedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return
		}
		if p.NewState == "done" {
			o.checkPlanCompletion(ev.AggregateID)
		}
	}
}

func (o *Orchestrator) checkDownstream(taskID string) {
	downstream, err := o.store.GetDownstreamTasks(taskID)
	if err != nil {
		return
	}
	for _, next := range downstream {
		ready, err := o.store.AllPrerequisitesMet(next.ID)
		if err != nil || !ready {
			continue
		}
		o.send(Notification{
			Type:     "downstream_ready",
			Title:    "下游任务就绪",
			Body:     fmt.Sprintf("任务 %s 的所有前置条件已满足，可以开始工作", next.Name),
			TaskID:   next.ID,
			Severity: "info",
		})
	}
}

func (o *Orchestrator) notifyBlocked(taskID string) {
	tc, err := o.store.GetTaskContext(taskID)
	if err != nil || tc == nil {
		return
	}
	o.send(Notification{
		Type:     "task_blocked",
		Title:    "任务被阻塞",
		Body:     fmt.Sprintf("任务 %s 已标记为阻塞，需要人工干预", tc.Title),
		TaskID:   taskID,
		Severity: "warning",
	})
}

func (o *Orchestrator) checkPlanCompletion(stepID string) {
	// Phase 4 stub: plan completion checks require plan store integration.
	// For now we emit a generic info notification when a step completes.
	o.send(Notification{
		Type:     "plan_step_done",
		Title:    "计划步骤完成",
		Body:     fmt.Sprintf("步骤 %s 已完成", stepID),
		TaskID:   "",
		Severity: "info",
	})
}

// --- heartbeat loop ---

func (o *Orchestrator) heartbeatLoop(ctx context.Context) {
	defer o.wg.Done()
	// First check after 10s, then every 30s.
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.stopCh:
			return
		case <-timer.C:
			o.checkHeartbeats()
			timer.Reset(30 * time.Second)
		}
	}
}

func (o *Orchestrator) checkHeartbeats() {
	sessions, err := o.store.ListActiveAgentSessions()
	if err != nil {
		return
	}
	now := time.Now()
	const timeout = 2 * time.Minute
	for _, s := range sessions {
		if s.LastHeartbeat == nil {
			continue
		}
		if now.Sub(*s.LastHeartbeat) > timeout {
			_ = o.store.MarkSessionDisconnected(s.ID, "heartbeat timeout")
			o.send(Notification{
				Type:     "heartbeat_timeout",
				Title:    "Agent 心跳超时",
				Body:     fmt.Sprintf("Session %s (%s) 超过2分钟未报告心跳", s.ID, s.Provider),
				TaskID:   s.TaskID,
				Severity: "warning",
			})
		}
	}
}

// --- helpers ---

func (o *Orchestrator) send(n Notification) {
	select {
	case o.notifCh <- n:
	default:
		// Channel full; drop oldest by consuming one, then retry.
		select {
		case <-o.notifCh:
		default:
		}
		select {
		case o.notifCh <- n:
		default:
		}
	}
}
