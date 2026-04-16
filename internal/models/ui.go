package models

import (
	"focus/internal/config"
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
	SaveWorktreeContext(record WorktreeContextRecord) error
	GetWorktreeContext(worktreeID string) (*WorktreeContextRecord, error)
	ListWorktreeContexts(repoID string) ([]WorktreeContextRecord, error)
	DeleteWorktreeContext(worktreeID string) error
	SaveTaskWorktreeLink(record TaskWorktreeLinkRecord) error
	ListTaskWorktreeLinks(taskID string) ([]TaskWorktreeLinkRecord, error)
	ListWorktreeTaskLinks(worktreeID string) ([]TaskWorktreeLinkRecord, error)
	SaveContextNote(record ContextNoteRecord) error
	ListContextNotes(taskID string, worktreeID string) ([]ContextNoteRecord, error)
	SaveAgentSession(record AgentSessionRecord) error
	ListAgentSessions(worktreeID string) ([]AgentSessionRecord, error)
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
	BranchSnapshot string
	PID            int
	State          string
	LaunchSource   string
	Summary        string
	StartedAt      time.Time
	EndedAt        *time.Time
	LastActivityAt *time.Time
	UpdatedAt      time.Time
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

type WorktreeContextRecord struct {
	WorktreeID     string
	RepoID         string
	PrimaryTaskID  *string
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
