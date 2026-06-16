package sessiondigest

import (
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/models"
	"github.com/RollingTheRock/Focus-tui/internal/store"
)

// Persist saves the digest into Focus-tui's SQLite store:
//   - agent_sessions.summary (one-line summary)
//   - task_outputs (full markdown report)
//   - session_handoffs (enriched with uncertainty/blocker summaries)
func Persist(
	st *store.Store,
	record models.AgentSessionRecord,
	digest *Digest,
) error {
	now := time.Now()

	// 1. Update agent session summary.
	record.Summary = digest.OneLineSummary()
	if err := st.SaveAgentSession(record); err != nil {
		return err
	}

	// 2. Save full report to task_outputs.
	if record.TaskID != "" {
		report := digest.ToMarkdown()
		if report != "" {
			if err := st.SaveTaskOutput(models.TaskOutputRecord{
				ID:        record.ID + "::digest",
				TaskID:    record.TaskID,
				Content:   report,
				Actor:     record.Provider,
				CreatedAt: now,
			}); err != nil {
				return err
			}
		}
	}

	// 3. Enrich session handoff if one exists.
	if record.TaskID != "" {
		handoff, _ := st.ListSessionHandoffs(record.TaskID)
		if len(handoff) > 0 {
			// Update the most recent handoff with richer data.
			h := handoff[0]
			h.DoneSummary = digest.Summary
			if len(digest.Blockers) > 0 {
				h.BlockerSummary = "Blockers: " + joinLimited(digest.Blockers, 3)
			}
			if len(digest.KeyDecisions) > 0 {
				h.DecisionSummary = "Decisions: " + joinLimited(digest.KeyDecisions, 3)
			}
			if len(digest.FilesChanged) > 0 {
				paths := make([]string, 0, len(digest.FilesChanged))
				for _, fc := range digest.FilesChanged {
					paths = append(paths, fc.Path)
				}
				h.UncertaintySummary = "Files: " + joinLimited(paths, 5)
			}
			_ = st.SaveSessionHandoff(h)
		}
	}

	return nil
}

func joinLimited(items []string, limit int) string {
	if len(items) > limit {
		items = append(items[:limit], "...")
	}
	result := ""
	for i, it := range items {
		if i > 0 {
			result += "; "
		}
		result += it
	}
	return result
}
