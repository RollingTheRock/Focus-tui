package store

import (
	"fmt"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/events"
)

// GetSessionTimeByTodoToday returns the total completed session duration for a
// todo item today.
func (s *Store) GetSessionTimeByTodoToday(todoID int) (time.Duration, error) {
	tbl := s.tbl("pomodoro_sessions", "proj_pomodoro_sessions")
	today := time.Now().Format("2006-01-02")

	// Use different duration computation for SQLite vs PostgreSQL.
	if s.mode == "postgresql" {
		var seconds float64
		err := s.qRow(
			fmt.Sprintf(`SELECT COALESCE(SUM(EXTRACT(EPOCH FROM (end_time - start_time))), 0) FROM %s WHERE linked_todo_id = ? AND date = ? AND status = 'completed'`, tbl),
			todoID, today,
		).Scan(&seconds)
		if err != nil {
			return 0, err
		}
		return time.Duration(seconds) * time.Second, nil
	}

	var seconds float64
	err := s.qRow(
		fmt.Sprintf(`SELECT COALESCE(SUM(CAST((julianday(end_time) - julianday(start_time)) * 86400 AS REAL)), 0) FROM %s WHERE linked_todo_id = ? AND date = ? AND status = 'completed'`, tbl),
		todoID, today,
	).Scan(&seconds)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

// StartSession creates a new pomodoro session, optionally linked to a todo.
func (s *Store) StartSession(linkedTodoID *int) (int64, error) {
	now := time.Now()
	q := fmt.Sprintf(`INSERT INTO %s (date, start_time, linked_todo_id) VALUES (?, ?, ?) RETURNING id`, s.tbl("pomodoro_sessions", "proj_pomodoro_sessions"))
	id, err := s.insertReturningID(q, now.Format("2006-01-02"), now, linkedTodoID)
	if err != nil {
		return 0, err
	}

	s.tryAppendEvent(events.AggregateTask, fmt.Sprintf("pomodoro-%d", id), events.PomodoroStarted,
		events.PomodoroStartedPayload{
			Date:         now.Format("2006-01-02"),
			StartTime:    now.Format(time.RFC3339),
			LinkedTodoID: linkedTodoID,
		}, events.AggregateTask, fmt.Sprintf("pomodoro-%d", id))

	return id, nil
}

// CompleteSession marks a session as completed with the current time.
func (s *Store) CompleteSession(id int64) error {
	now := time.Now()
	_, err := s.exec(
		fmt.Sprintf(`UPDATE %s SET end_time = ?, status = 'completed' WHERE id = ?`, s.tbl("pomodoro_sessions", "proj_pomodoro_sessions")),
		now, id,
	)
	if err != nil {
		return err
	}
	// Also record in streaks table.
	_, err = s.exec(
		fmt.Sprintf(`INSERT INTO %s (date, has_pomodoro) VALUES (?, 1) ON CONFLICT(date) DO UPDATE SET has_pomodoro = 1`, s.tbl("streaks", "proj_streaks")),
		now.Format("2006-01-02"),
	)
	if err != nil {
		return err
	}

	dateStr := now.Format("2006-01-02")
	s.tryAppendEvent(events.AggregateTask, fmt.Sprintf("pomodoro-%d", id), events.PomodoroCompleted,
		events.PomodoroCompletedPayload{
			ID:           id,
			Date:         dateStr,
			EndTime:      now.Format(time.RFC3339),
		}, events.AggregateTask, fmt.Sprintf("pomodoro-%d", id))

	s.tryAppendEvent(events.AggregateTask, "streak-"+dateStr, events.StreakUpdated,
		events.StreakUpdatedPayload{
			Date:        dateStr,
			HasPomodoro: true,
		}, events.AggregateTask, "streak-"+dateStr)

	return nil
}

// CancelSession marks a session as cancelled.
func (s *Store) CancelSession(id int64) error {
	now := time.Now()
	_, err := s.exec(
		fmt.Sprintf(`UPDATE %s SET end_time = ?, status = 'cancelled' WHERE id = ?`, s.tbl("pomodoro_sessions", "proj_pomodoro_sessions")),
		now, id,
	)
	if err != nil {
		return err
	}

	s.tryAppendEvent(events.AggregateTask, fmt.Sprintf("pomodoro-%d", id), events.PomodoroCancelled,
		events.PomodoroCancelledPayload{
			ID:      id,
			Date:    now.Format("2006-01-02"),
			EndTime: now.Format(time.RFC3339),
		}, events.AggregateTask, fmt.Sprintf("pomodoro-%d", id))

	return nil
}

// TodaySessionCount returns the number of completed sessions today.
func (s *Store) TodaySessionCount() (int, error) {
	var count int
	err := s.qRow(
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE date = ? AND status = 'completed'`, s.tbl("pomodoro_sessions", "proj_pomodoro_sessions")),
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
		err := s.qRow(
			fmt.Sprintf(`SELECT has_pomodoro FROM %s WHERE date = ?`, s.tbl("streaks", "proj_streaks")),
			d.Format("2006-01-02"),
		).Scan(&has)
		if isNoRows(err) || !has {
			break
		}
		if err != nil {
			return 0, err
		}
		streak++
	}
	return streak, nil
}
