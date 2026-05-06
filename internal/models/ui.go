package models

import (
	"focus/internal/config"
	"focus/internal/events"
	"focus/internal/styles"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Store defines the storage interface used by UI components.
type Store interface {
	CreateTodo(text, list string) (*Todo, error)
	GetTodo(id int) (*Todo, error)
	ListTodos(list string) ([]Todo, error)
	ToggleTodo(id int) error
	UpdateTodoText(id int, text string) error
	DeleteTodo(id int) error
	MoveToToday(id int) error
	MarkOverdue() error
	TodayDoneCount() (done int, total int, err error)
	StartSession(linkedTodoID *int) (int64, error)
	CompleteSession(id int64) error
	CancelSession(id int64) error
	TodaySessionCount() (int, error)
	GetStreak() (int, error)
	SavePageSnapshot(worktreeID string, snapshotJSON []byte) error
	LoadPageSnapshot(worktreeID string) ([]byte, error)
	ListPageSnapshots() ([]PageSnapshotRecord, error)
	DeletePageSnapshot(worktreeID string) error
	SaveTaskContext(record TaskContextRecord) error
	GetTaskContext(id string) (*TaskContextRecord, error)
	ListTaskContexts(repoID string) ([]TaskContextRecord, error)
	SaveTaskBrief(record TaskBriefRecord) error
	GetTaskBrief(taskID string) (*TaskBriefRecord, error)
	SaveWorktreeContext(record WorktreeContextRecord) error
	GetWorktreeContext(worktreeID string) (*WorktreeContextRecord, error)
	ListWorktreeContexts(repoID string) ([]WorktreeContextRecord, error)
	DeleteWorktreeContext(worktreeID string) error
	SaveTaskWorktreeLink(record TaskWorktreeLinkRecord) error
	ListTaskWorktreeLinks(taskID string) ([]TaskWorktreeLinkRecord, error)
	ListWorktreeTaskLinks(worktreeID string) ([]TaskWorktreeLinkRecord, error)
	SaveTaskPlan(record TaskPlanRecord) error
	GetTaskPlan(id string) (*TaskPlanRecord, error)
	ListTaskPlans(taskID string) ([]TaskPlanRecord, error)
	SavePlanStep(record PlanStepRecord) error
	DeletePlanSteps(planID string) error
	ListPlanSteps(planID string) ([]PlanStepRecord, error)
	SaveSessionHandoff(record SessionHandoffRecord) error
	ListSessionHandoffs(taskID string) ([]SessionHandoffRecord, error)
	SaveContextNote(record ContextNoteRecord) error
	ListContextNotes(taskID string, worktreeID string) ([]ContextNoteRecord, error)
	SaveAgentSession(record AgentSessionRecord) error
	ListAgentSessions(worktreeID string) ([]AgentSessionRecord, error)
	SaveTaskDependency(record TaskDependencyRecord) error
	DeleteTaskDependency(fromTaskID, toTaskID string) error
	ListDownstreamTaskContexts(taskID string) ([]TaskContextRecord, error)
	ListUpstreamTaskContexts(taskID string) ([]TaskContextRecord, error)
	AreTaskPrerequisitesMet(taskID string) (bool, error)
	MarkAgentSessionDisconnected(sessionID string, reason string) error
	UpdateAgentSessionHeartbeat(sessionID string, at time.Time, state string) error
	SaveTaskOutput(record TaskOutputRecord) error
	ListTaskOutputs(taskID string) ([]TaskOutputRecord, error)
	SaveKnowledgeFact(record KnowledgeFactRecord) error
	ListKnowledgeFacts(planID string) ([]KnowledgeFactRecord, error)
	SaveAgentMessage(record AgentMessageRecord) error
	ListAgentMessages(target string, messageType string, limit int) ([]AgentMessageRecord, error)
	SaveWorktreeHistory(record WorktreeHistoryRecord) error
	ListWorktreeHistory(repoID string) ([]WorktreeHistoryRecord, error)
	DeleteWorktreeHistory(id string) error
	DeleteAgentSessionsByWorktreeID(worktreeID string) error
	DeleteContextNotesByWorktreeID(worktreeID string) error
	DeleteTaskWorktreeLinksByWorktreeID(worktreeID string) error
	// EventStore returns the event store (nil when not using PostgreSQL).
	EventStore() any
	// EventBus returns the in-memory event bus (nil when not using PostgreSQL).
	EventBus() *events.EventBus
}

type PageSnapshotRecord struct {
	WorktreeID   string
	SnapshotJSON string
}

type AgentSessionRecord struct {
	ID             string
	Provider       string
	WorktreeID     string
	RepoID         string
	TaskID         string
	PlanID         string
	StepID         string
	BranchSnapshot string
	PID            int
	State          string
	LaunchSource   string
	Summary        string
	EnvSnapshot    string
	StartedAt      time.Time
	EndedAt        *time.Time
	LastActivityAt *time.Time
	LastHeartbeat  *time.Time
	StopReason     string
	UpdatedAt      time.Time
}

type TaskDependencyRecord struct {
	FromTaskID     string
	ToTaskID       string
	DependencyType string
	CreatedAt      time.Time
}

type TaskOutputRecord struct {
	ID        string
	TaskID    string
	Content   string
	Actor     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type KnowledgeFactRecord struct {
	ID         string
	PlanID     string
	Subject    string
	Predicate  string
	Object     string
	Source     string
	Confidence float64
	CreatedAt  time.Time
}

type AgentMessageRecord struct {
	ID        string
	FromAgent string
	ToAgent   string
	MsgType   string
	Payload   string
	ReadAt    *time.Time
	CreatedAt time.Time
}

type TaskContextRecord struct {
	ID                  string
	RepoID              string
	Title               string
	Goal                string
	NextStep            string
	State               string
	Priority            string
	ParentTaskID        *string
	PreferredWorktreeID string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type TaskBriefRecord struct {
	TaskID          string
	WhyNow          string
	SuccessCriteria string
	OutOfScope      string
	KnownRisks      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WorktreeContextRecord struct {
	WorktreeID     string
	RepoID         string
	PrimaryTaskID  *string
	CurrentPlanID  *string
	TaskMode       string
	TaskName       string
	BranchSnapshot string
	LastActiveAt   time.Time
	LastOpenedAt   *time.Time
	LastAgentAt    *time.Time
	UpdatedAt      time.Time
}

type TaskWorktreeLinkRecord struct {
	ID           string
	TaskID       string
	WorktreeID   string
	RelationType string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type WorktreeHistoryRecord struct {
	ID             string
	RepoID         string
	Branch         string
	Path           string
	CreatedAt      time.Time
	RemovedAt      time.Time
	TaskID         *string
	PlanID         *string
	Provider       string
	Summary        string
	DurationMinutes int
}

type TaskPlanRecord struct {
	ID          string
	TaskID      string
	Title       string
	WhyNow      string
	Success     string
	OutOfScope  string
	KnownRisks  string
	Status      string
	CurrentStep string
	PlanBody    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  *time.Time
	DoneAt      *time.Time
}

type PlanStepRecord struct {
	ID             string
	PlanID         string
	OrderIndex     int
	Title          string
	State          string
	ExpandedTaskID string
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SessionHandoffRecord struct {
	ID                 string
	TaskID             string
	PlanID             *string
	SessionID          string
	DoneSummary        string
	RemainingSummary   string
	DecisionSummary    string
	UncertaintySummary string
	BlockerSummary     string
	Entrypoint         string
	CreatedAt          time.Time
}

type ContextNoteRecord struct {
	ID         string
	TaskID     *string
	WorktreeID string
	NoteType   string
	Body       string
	Pinned     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CommonModel holds shared state across all panels.
type CommonModel struct {
	Width  int
	Height int
	Theme  styles.Theme
	Cfg    config.Config
	Store  Store
}

// Panel is the interface implemented by every UI sub-model.
type Panel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Panel, tea.Cmd)
	View() string
	SetSize(width, height int)
}

// StatsRefreshMsg tells the footer to reload statistics.
// Shared across packages to avoid circular imports.
type StatsRefreshMsg struct{}
