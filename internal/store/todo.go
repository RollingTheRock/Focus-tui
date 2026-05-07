package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"focus/internal/events"
	"focus/internal/models"
)

// CreateTodo inserts a new todo item.
func (s *Store) CreateTodo(text, list string) (*models.Todo, error) {
	q := fmt.Sprintf(`INSERT INTO %s (text, list, status) VALUES (?, ?, 'todo') RETURNING id`, s.tbl("todos", "proj_todos"))
	id, err := s.insertReturningID(q, text, list)
	if err != nil {
		return nil, err
	}

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
	q := fmt.Sprintf(
		`SELECT id, text, status, list, created_at, updated_at FROM %s WHERE id = ?`,
		s.tbl("todos", "proj_todos"),
	)
	return scanTodo(s.qRow(q, id))
}

// ListTodos returns todos for a given list, with done/overdue items sorted to the bottom.
func (s *Store) ListTodos(list string) ([]models.Todo, error) {
	q := fmt.Sprintf(`
		SELECT id, text, status, list, created_at, updated_at
		FROM %s WHERE list = ?
		ORDER BY CASE status
			WHEN 'todo' THEN 0
			WHEN 'overdue' THEN 1
			WHEN 'done' THEN 2
		END, id ASC`, s.tbl("todos", "proj_todos"))
	rows, err := s.qRows(q, list)
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
	q := fmt.Sprintf(`UPDATE %s SET
	    status = CASE WHEN status = 'done' THEN 'todo' ELSE 'done' END,
	    updated_at = CURRENT_TIMESTAMP
	 WHERE id = ?`, s.tbl("todos", "proj_todos"))
	_, err := s.exec(q, id)
	return err
}

// UpdateTodoText updates the text of a todo.
func (s *Store) UpdateTodoText(id int, text string) error {
	q := fmt.Sprintf(`UPDATE %s SET text = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, s.tbl("todos", "proj_todos"))
	_, err := s.exec(q, text, id)
	return err
}

// DeleteTodo removes a todo by ID.
func (s *Store) DeleteTodo(id int) error {
	q := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, s.tbl("todos", "proj_todos"))
	_, err := s.exec(q, id)
	return err
}

// MoveToToday moves a someday todo to the today list.
func (s *Store) MoveToToday(id int) error {
	q := fmt.Sprintf(`UPDATE %s SET list = 'today', updated_at = CURRENT_TIMESTAMP
	 WHERE id = ? AND list = 'someday'`, s.tbl("todos", "proj_todos"))
	_, err := s.exec(q, id)
	return err
}

// MarkOverdue marks yesterday's unfinished today items as overdue.
// Call this once at app startup or at midnight.
func (s *Store) MarkOverdue() error {
	today := time.Now().Format("2006-01-02")
	q := fmt.Sprintf(`UPDATE %s SET status = 'overdue', updated_at = CURRENT_TIMESTAMP
	 WHERE list = 'today' AND status = 'todo'
	 AND DATE(created_at) < ?`, s.tbl("todos", "proj_todos"))
	_, err := s.exec(q, today)
	return err
}

// TodayDoneCount returns (done_count, total_count) for today's list.
func (s *Store) TodayDoneCount() (done int, total int, err error) {
	tbl := s.tbl("todos", "proj_todos")
	err = s.qRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE list = 'today'`, tbl)).Scan(&total)
	if err != nil {
		return
	}
	err = s.qRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE list = 'today' AND status = 'done'`, tbl)).Scan(&done)
	return
}

func scanTodo(row rowScanner) (*models.Todo, error) {
	var t models.Todo
	if err := row.Scan(&t.ID, &t.Text, &t.Status, &t.List, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}
