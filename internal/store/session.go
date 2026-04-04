package store

import (
	"time"
)

// StartSession creates a new pomodoro session, optionally linked to a todo.
func (s *Store) StartSession(linkedTodoID *int) (int64, error) {
	now := time.Now()
	res, err := s.db.Exec(
		`INSERT INTO pomodoro_sessions (date, start_time, linked_todo_id)
		 VALUES (?, ?, ?)`,
		now.Format("2006-01-02"), now, linkedTodoID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CompleteSession marks a session as completed with the current time.
func (s *Store) CompleteSession(id int64) error {
	now := time.Now()
	_, err := s.db.Exec(
		`UPDATE pomodoro_sessions SET end_time = ?, status = 'completed' WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return err
	}
	// Also record in streaks table.
	_, err = s.db.Exec(
		`INSERT INTO streaks (date, has_pomodoro) VALUES (?, 1)
		 ON CONFLICT(date) DO UPDATE SET has_pomodoro = 1`,
		now.Format("2006-01-02"),
	)
	return err
}

// CancelSession marks a session as cancelled.
func (s *Store) CancelSession(id int64) error {
	_, err := s.db.Exec(
		`UPDATE pomodoro_sessions SET end_time = ?, status = 'cancelled' WHERE id = ?`,
		time.Now(), id,
	)
	return err
}

// TodaySessionCount returns the number of completed sessions today.
func (s *Store) TodaySessionCount() (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM pomodoro_sessions
		 WHERE date = ? AND status = 'completed'`,
		time.Now().Format("2006-01-02"),
	).Scan(&count)
	return count, err
}

// GetStreak returns the current consecutive-day streak (days with at least one completed pomodoro).
func (s *Store) GetStreak() (int, error) {
	today := time.Now().Truncate(24 * time.Hour)
	streak := 0

	for d := today; ; d = d.AddDate(0, 0, -1) {
		var has bool
		err := s.db.QueryRow(
			`SELECT has_pomodoro FROM streaks WHERE date = ?`,
			d.Format("2006-01-02"),
		).Scan(&has)
		if err != nil || !has {
			break
		}
		streak++
	}
	return streak, nil
}
