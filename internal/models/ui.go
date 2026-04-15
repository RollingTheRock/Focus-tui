package models

import (
	"focus/internal/config"
	"focus/internal/styles"

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
}

type PageSnapshotRecord struct {
	WorktreeID   string
	SnapshotJSON string
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
