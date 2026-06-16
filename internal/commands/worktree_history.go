package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// RecordWorktreeHistory saves a worktree history entry.
type RecordWorktreeHistory struct {
	ID, RepoID, Branch, Path, Provider, Summary string
	CreatedAt                                   time.Time
	RemovedAt                                   time.Time
	TaskID, PlanID                              *string
	DurationMinutes                             int
}

// Validate checks required fields.
func (c *RecordWorktreeHistory) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("history id required")
	}
	if c.RepoID == "" {
		return fmt.Errorf("repo id required")
	}
	return nil
}

// Execute persists the history record via the store.
func (c *RecordWorktreeHistory) Execute(ctx context.Context, s *store.Store) error {
	return s.SaveWorktreeHistory(models.WorktreeHistoryRecord{
		ID:              c.ID,
		RepoID:          c.RepoID,
		Branch:          c.Branch,
		Path:            c.Path,
		CreatedAt:       c.CreatedAt,
		RemovedAt:       c.RemovedAt,
		TaskID:          c.TaskID,
		PlanID:          c.PlanID,
		Provider:        c.Provider,
		Summary:         c.Summary,
		DurationMinutes: c.DurationMinutes,
	})
}
