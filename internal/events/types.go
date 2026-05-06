package events

import (
	"encoding/json"
	"time"
)

// Event is the canonical representation of a domain change.
// It maps 1:1 to the events table in PostgreSQL.
type Event struct {
	EventID       int64           `json:"event_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Version       int64           `json:"version"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	ActorType     string          `json:"actor_type,omitempty"`
	ActorID       string          `json:"actor_id,omitempty"`
	CausationID   *int64          `json:"causation_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	ScopeType     string          `json:"scope_type,omitempty"`
	ScopeID       string          `json:"scope_id,omitempty"`
}

// Aggregate types.
const (
	AggregateTask          = "task"
	AggregateWorktree      = "worktree"
	AggregatePlan          = "plan"
	AggregateAgentSession  = "agent_session"
	AggregateContextNote   = "context_note"
	AggregateSessionHandoff = "session_handoff"
	AggregateKnowledgeFact = "knowledge_fact"
)

// Event types.
const (
	TaskCreated       = "TaskCreated"
	TaskStateChanged  = "TaskStateChanged"
	TaskGoalUpdated   = "TaskGoalUpdated"
	TaskNextStepSet   = "TaskNextStepSet"

	WorktreeContextUpdated = "WorktreeContextUpdated"
	WorktreeActivated      = "WorktreeActivated"

	PlanStepStateChanged = "PlanStepStateChanged"
	PlanApproved         = "PlanApproved"
	PlanArchived         = "PlanArchived"

	AgentSessionCreated     = "AgentSessionCreated"
	AgentSessionHeartbeat   = "AgentSessionHeartbeat"
	AgentSessionDisconnected = "AgentSessionDisconnected"
	AgentSessionSummarySet  = "AgentSessionSummarySet"

	ContextNoteAdded   = "ContextNoteAdded"
	ContextNotePinned  = "ContextNotePinned"

	SessionHandoffCreated = "SessionHandoffCreated"

	KnowledgeFactAdded = "KnowledgeFactAdded"

	TaskDependencyAdded    = "TaskDependencyAdded"
	TaskDependencyRemoved  = "TaskDependencyRemoved"

	TodoCreated        = "TodoCreated"
	TaskPlanCreated    = "TaskPlanCreated"
	TaskOutputAdded    = "TaskOutputAdded"
	TaskBriefUpdated   = "TaskBriefUpdated"
)

// Actor types.
const (
	ActorHuman        = "human"
	ActorAgent        = "agent"
	ActorOrchestrator = "orchestrator"
	ActorSystem       = "system"
)

// --- Payload types ---

// TaskCreatedPayload is emitted when a new task is created.
type TaskCreatedPayload struct {
	RepoID        string  `json:"repo_id"`
	Title         string  `json:"title"`
	Goal          string  `json:"goal,omitempty"`
	Priority      string  `json:"priority"`
	ParentTaskID  *string `json:"parent_task_id,omitempty"`
}

// TaskStateChangedPayload is emitted when a task transitions state.
type TaskStateChangedPayload struct {
	PreviousState string `json:"previous_state"`
	NewState      string `json:"new_state"`
}

// TaskGoalUpdatedPayload is emitted when a task's goal or next_step is modified.
type TaskGoalUpdatedPayload struct {
	Goal     string `json:"goal,omitempty"`
	NextStep string `json:"next_step,omitempty"`
}

// WorktreeContextUpdatedPayload is emitted when a worktree's context changes.
type WorktreeContextUpdatedPayload struct {
	RepoID        string  `json:"repo_id"`
	TaskMode      string  `json:"task_mode"`
	TaskName      string  `json:"task_name,omitempty"`
	BranchSnapshot  string  `json:"branch_snapshot,omitempty"`
	PrimaryTaskID *string `json:"primary_task_id,omitempty"`
}

// AgentSessionCreatedPayload is emitted when an agent session starts.
type AgentSessionCreatedPayload struct {
	Provider       string `json:"provider"`
	WorktreeID     string `json:"worktree_id"`
	RepoID         string `json:"repo_id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	PlanID         string `json:"plan_id,omitempty"`
	StepID         string `json:"step_id,omitempty"`
	BranchSnapshot string `json:"branch_snapshot,omitempty"`
	PID            int    `json:"pid,omitempty"`
	LaunchSource   string `json:"launch_source,omitempty"`
	Summary        string `json:"summary,omitempty"`
	EnvSnapshot    string `json:"env_snapshot,omitempty"`
}

// AgentSessionHeartbeatPayload is emitted on each heartbeat.
type AgentSessionHeartbeatPayload struct {
	State string `json:"state,omitempty"`
}

// AgentSessionDisconnectedPayload is emitted when a session ends.
type AgentSessionDisconnectedPayload struct {
	Reason string `json:"reason,omitempty"`
}

// ContextNoteAddedPayload is emitted when a note is added.
type ContextNoteAddedPayload struct {
	TaskID    string `json:"task_id,omitempty"`
	WorktreeID string `json:"worktree_id,omitempty"`
	NoteType  string `json:"note_type"`
	Body      string `json:"body"`
	Pinned    bool   `json:"pinned"`
}

// SessionHandoffCreatedPayload is emitted when a handoff is recorded.
type SessionHandoffCreatedPayload struct {
	TaskID              string `json:"task_id"`
	PlanID              string `json:"plan_id,omitempty"`
	SessionID           string `json:"session_id,omitempty"`
	DoneSummary         string `json:"done_summary,omitempty"`
	RemainingSummary    string `json:"remaining_summary,omitempty"`
	DecisionSummary     string `json:"decision_summary,omitempty"`
	UncertaintySummary  string `json:"uncertainty_summary,omitempty"`
	BlockerSummary      string `json:"blocker_summary,omitempty"`
	Entrypoint          string `json:"entrypoint,omitempty"`
}

// KnowledgeFactAddedPayload is emitted when a knowledge fact is added.
type KnowledgeFactAddedPayload struct {
	PlanID     string  `json:"plan_id,omitempty"`
	Subject    string  `json:"subject"`
	Predicate  string  `json:"predicate"`
	Object     string  `json:"object"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// PlanStepStateChangedPayload is emitted when a plan step changes state.
type PlanStepStateChangedPayload struct {
	PlanID        string  `json:"plan_id"`
	PreviousState string  `json:"previous_state"`
	NewState      string  `json:"new_state"`
	ExpandedTaskID *string `json:"expanded_task_id,omitempty"`
}

// TaskDependencyAddedPayload is emitted when a dependency is created.
type TaskDependencyAddedPayload struct {
	FromTaskID      string `json:"from_task_id"`
	ToTaskID        string `json:"to_task_id"`
	DependencyType  string `json:"dependency_type"`
}

// TodoCreatedPayload is emitted when a todo is created.
type TodoCreatedPayload struct {
	Text string `json:"text"`
	List string `json:"list"`
}

// TaskPlanCreatedPayload is emitted when a task plan is created.
type TaskPlanCreatedPayload struct {
	TaskID     string `json:"task_id,omitempty"`
	Title      string `json:"title"`
	WhyNow     string `json:"why_now,omitempty"`
	Success    string `json:"success,omitempty"`
	OutOfScope string `json:"out_of_scope,omitempty"`
	KnownRisks string `json:"known_risks,omitempty"`
}

// TaskOutputAddedPayload is emitted when a task output is recorded.
type TaskOutputAddedPayload struct {
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
	Actor   string `json:"actor,omitempty"`
}

// TaskBriefUpdatedPayload is emitted when a task brief is updated.
type TaskBriefUpdatedPayload struct {
	TaskID           string `json:"task_id"`
	WhyNow           string `json:"why_now,omitempty"`
	SuccessCriteria  string `json:"success_criteria,omitempty"`
	OutOfScope       string `json:"out_of_scope,omitempty"`
	KnownRisks       string `json:"known_risks,omitempty"`
}

// Serialize returns the JSON payload for an event payload struct.
func Serialize(v any) ([]byte, error) {
	return json.Marshal(v)
}
