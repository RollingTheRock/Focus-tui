package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"focus/internal/events"
	"focus/internal/models"
)

// CreateTodo inserts a new todo item.
func (s *Store) CreateTodo(text, list string) (*models.Todo, error) {
	res, err := s.db.Exec(
		`INSERT INTO todos (text, list, status) VALUES (?, ?, 'todo')`,
		text, list,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	// Phase 1: best-effort event append.
	if s.events != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		payload, _ := events.Serialize(map[string]any{
			"text": text,
			"list": list,
		})
		todoID := fmt.Sprintf("todo-%d", id)
		_, evErr := s.events.AppendEvent(ctx, events.Event{
			OccurredAt:    time.Now(),
			AggregateType: "todo",
			AggregateID:   todoID,
			EventType:     events.TodoCreated,
			Payload:       payload,
			ActorType:     events.ActorSystem,
			ScopeType:     "todo",
			ScopeID:       todoID,
		})
		if evErr != nil {
			log.Printf("[event-store] append todo event failed (non-critical): %v", evErr)
		}
	}

	return s.GetTodo(int(id))
}

// GetTodo returns a single todo by ID.
func (s *Store) GetTodo(id int) (*models.Todo, error) {
	row := s.db.QueryRow(
		`SELECT id, text, status, list, created_at, updated_at FROM todos WHERE id = ?`, id,
	)
	return scanTodo(row)
}

// ListTodos returns todos for a given list, with done/overdue items sorted to the bottom.
func (s *Store) ListTodos(list string) ([]models.Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, text, status, list, created_at, updated_at
		 FROM todos WHERE list = ?
		 ORDER BY CASE status
		     WHEN 'todo' THEN 0
		     WHEN 'overdue' THEN 1
		     WHEN 'done' THEN 2
		 END, id ASC`, list,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []models.Todo
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(&t.ID, &t.Text, &t.Status, &t.List, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// ToggleTodo switches a todo between todo and done.
func (s *Store) ToggleTodo(id int) error {
	_, err := s.db.Exec(
		`UPDATE todos SET
		    status = CASE WHEN status = 'done' THEN 'todo' ELSE 'done' END,
		    updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`, id,
	)
	return err
}

// UpdateTodoText updates the text of a todo.
func (s *Store) UpdateTodoText(id int, text string) error {
	_, err := s.db.Exec(
		`UPDATE todos SET text = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		text, id,
	)
	return err
}

// DeleteTodo removes a todo by ID.
func (s *Store) DeleteTodo(id int) error {
	_, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	return err
}

// MoveToToday moves a someday todo to the today list.
func (s *Store) MoveToToday(id int) error {
	_, err := s.db.Exec(
		`UPDATE todos SET list = 'today', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND list = 'someday'`, id,
	)
	return err
}

// MarkOverdue marks yesterday's unfinished today items as overdue.
// Call this once at app startup or at midnight.
func (s *Store) MarkOverdue() error {
	today := time.Now().Format("2006-01-02")
	_, err := s.db.Exec(
		`UPDATE todos SET status = 'overdue', updated_at = CURRENT_TIMESTAMP
		 WHERE list = 'today' AND status = 'todo'
		 AND DATE(created_at) < ?`, today,
	)
	return err
}

// TodayDoneCount returns (done_count, total_count) for today's list.
func (s *Store) TodayDoneCount() (done int, total int, err error) {
	err = s.db.QueryRow(
		`SELECT COUNT(*) FROM todos WHERE list = 'today'`,
	).Scan(&total)
	if err != nil {
		return
	}
	err = s.db.QueryRow(
		`SELECT COUNT(*) FROM todos WHERE list = 'today' AND status = 'done'`,
	).Scan(&done)
	return
}

func scanTodo(row *sql.Row) (*models.Todo, error) {
	var t models.Todo
	if err := row.Scan(&t.ID, &t.Text, &t.Status, &t.List, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}
