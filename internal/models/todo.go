package models

import "time"

// Todo status constants.
const (
	StatusTodo    = "todo"
	StatusDone    = "done"
	StatusOverdue = "overdue"
)

// Todo list constants.
const (
	ListToday   = "today"
	ListSomeday = "someday"
)

// Todo represents a single task item.
type Todo struct {
	ID        int
	Text      string
	Status    string // todo, done, overdue
	List      string // today, someday
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsDone returns true if the todo is completed.
func (t *Todo) IsDone() bool { return t.Status == StatusDone }

// IsOverdue returns true if the todo is overdue.
func (t *Todo) IsOverdue() bool { return t.Status == StatusOverdue }
