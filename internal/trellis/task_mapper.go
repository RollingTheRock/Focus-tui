package trellis

import (
	"strings"

	"focus/internal/models"
)

// focusStateToTrellis maps Focus task states to Trellis task statuses.
func focusStateToTrellis(state string) string {
	switch state {
	case "active":
		return "in_progress"
	case "paused":
		return "in_progress" // paused is a Focus UI concept
	case "blocked":
		return "planning"
	case "done":
		return "completed"
	case "archived":
		return "archived"
	default:
		return "planning"
	}
}

// trellisStatusToFocus maps Trellis task statuses back to Focus task states.
func trellisStatusToFocus(status string) string {
	switch status {
	case "in_progress":
		return "active"
	case "planning":
		return "blocked"
	case "completed":
		return "done"
	case "archived":
		return "archived"
	default:
		return "active"
	}
}

// focusPriorityToTrellis maps Focus priorities to Trellis priorities.
func focusPriorityToTrellis(p string) string {
	switch p {
	case "critical":
		return "P0"
	case "high":
		return "P1"
	case "medium":
		return "P2"
	case "low":
		return "P3"
	default:
		return "P2"
	}
}

// taskSlugFromTitle generates a Trellis-compatible task slug from a title.
func taskSlugFromTitle(title string) string {
	// Trellis uses {MM-DD}-{slugified-title} format.
	// For now, return a simple slug; date prefix is added by task.py.
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// TODO: remove special characters, limit length.
	return slug
}

// taskRecordToTrellisTask converts a Focus TaskContextRecord to a TrellisTask.
func taskRecordToTrellisTask(task models.TaskContextRecord) TrellisTask {
	return TrellisTask{
		ID:       task.ID,
		Name:     taskSlugFromTitle(task.Title),
		Title:    task.Title,
		Status:   focusStateToTrellis(task.State),
		Priority: focusPriorityToTrellis(task.Priority),
	}
}
