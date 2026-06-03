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
	// task.py adds the date prefix; we just need a clean slug.
	slug := strings.ToLower(title)
	var b strings.Builder
	for _, r := range slug {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
		// skip all other special characters
	}
	slug = b.String()
	// collapse multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	// limit length
	const maxLen = 50
	if len(slug) > maxLen {
		slug = slug[:maxLen]
		slug = strings.TrimRight(slug, "-")
	}
	if slug == "" {
		slug = "task"
	}
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
