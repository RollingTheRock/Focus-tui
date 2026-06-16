package trellis

import (
	"fmt"
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/agents"
	"github.com/RollingTheRock/Focus-tui/internal/models"
)

// buildFocusMetadataLayer assembles Layer A: Focus-specific metadata that
// Trellis does not have (plan/step/DAG/MCP guide).
func (b *Bridge) buildFocusMetadataLayer(session *agents.Session) (agents.SpecLayer, error) {
	var parts []string

	// Task metadata.
	if session.TaskID != "" {
		parts = append(parts, b.renderTaskSection(session.TaskID))
	}

	// Step metadata.
	if session.StepID != "" {
		parts = append(parts, b.renderStepSection(session.StepID))
	}

	// DAG dependencies.
	if session.TaskID != "" {
		parts = append(parts, b.renderDAGSection(session.TaskID))
	}

	// MCP tools reference.
	parts = append(parts, renderMCPToolsSection())

	content := strings.Join(parts, "\n\n")
	return agents.SpecLayer{
		Title:   "Focus Task Metadata",
		Content: content,
		Source:  "focus",
	}, nil
}

func (b *Bridge) renderTaskSection(taskID string) string {
	var parts []string
	parts = append(parts, "## Task")
	if b.store != nil {
		task, err := b.store.GetTaskContext(taskID)
		if err == nil && task != nil {
			parts = append(parts, fmt.Sprintf("**Title:** %s", task.Title))
			if task.Goal != "" {
				parts = append(parts, fmt.Sprintf("**Goal:** %s", task.Goal))
			}
			if task.NextStep != "" {
				parts = append(parts, fmt.Sprintf("**Next Step:** %s", task.NextStep))
			}
			parts = append(parts, fmt.Sprintf("**State:** %s | **Priority:** %s", task.State, task.Priority))
		} else {
			parts = append(parts, fmt.Sprintf("**TaskID:** %s", taskID))
		}
	} else {
		parts = append(parts, fmt.Sprintf("**TaskID:** %s", taskID))
	}
	return strings.Join(parts, "\n")
}

func (b *Bridge) renderStepSection(stepID string) string {
	var parts []string
	parts = append(parts, "## Current Step")
	if b.store != nil {
		// Steps are PlanStepRecords; look up via all plans.
		stepFound := false
		// We don't have a direct GetPlanStep API, so search through plans.
		// This is best-effort; if the store does not expose list-all-plan-steps,
		// we fall back to just the ID.
		if step, err := b.lookupPlanStep(stepID); err == nil && step != nil {
			parts = append(parts, fmt.Sprintf("**Title:** %s", step.Title))
			if step.Notes != "" {
				parts = append(parts, fmt.Sprintf("**Notes:** %s", step.Notes))
			}
			parts = append(parts, fmt.Sprintf("**Order:** %d | **State:** %s", step.OrderIndex, step.State))
			stepFound = true
		}
		if !stepFound {
			parts = append(parts, fmt.Sprintf("**StepID:** %s", stepID))
		}
	} else {
		parts = append(parts, fmt.Sprintf("**StepID:** %s", stepID))
	}
	return strings.Join(parts, "\n")
}

func (b *Bridge) lookupPlanStep(stepID string) (*models.PlanStepRecord, error) {
	// Brute-force: list all task contexts, then list plans for each, then list steps.
	if b.store == nil {
		return nil, fmt.Errorf("no store")
	}
	ctxs, err := b.store.ListTaskContexts("")
	if err != nil {
		return nil, err
	}
	for _, ctx := range ctxs {
		plans, err := b.store.ListTaskPlans(ctx.ID)
		if err != nil {
			continue
		}
		for _, plan := range plans {
			steps, err := b.store.ListPlanSteps(plan.ID)
			if err != nil {
				continue
			}
			for _, step := range steps {
				if step.ID == stepID {
					return &step, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("step %s not found", stepID)
}

func (b *Bridge) renderDAGSection(taskID string) string {
	var parts []string
	parts = append(parts, "## DAG Dependencies")
	if b.store != nil {
		upstream, _ := b.store.ListUpstreamTaskContexts(taskID)
		downstream, _ := b.store.ListDownstreamTaskContexts(taskID)
		if len(upstream) > 0 {
			parts = append(parts, "**Upstream:**")
			for _, t := range upstream {
				parts = append(parts, fmt.Sprintf("- [%s] %s", t.State, t.Title))
			}
		}
		if len(downstream) > 0 {
			parts = append(parts, "**Downstream:**")
			for _, t := range downstream {
				parts = append(parts, fmt.Sprintf("- [%s] %s", t.State, t.Title))
			}
		}
		if len(upstream) == 0 && len(downstream) == 0 {
			parts = append(parts, "No dependencies.")
		}
	} else {
		parts = append(parts, fmt.Sprintf("**TaskID:** %s", taskID))
	}
	return strings.Join(parts, "\n")
}

func renderMCPToolsSection() string {
	tools := []string{
		"## Focus MCP Tools Reference",
		"",
		"- `task.get` — task details (title, goal, next_step, state)",
		"- `task.list` — all tasks in the repo",
		"- `task.create` — create a new task (title must be Chinese)",
		"- `task.create_output` — record an output/artifact for this task",
		"- `task.update_status` — update task state (active/paused/blocked/done/archived)",
		"- `plan.get` — plan details with steps",
		"- `plan.list` — all task plans",
		"- `plan.add_step` — add a step to a plan",
		"- `dag.get_status` — full DAG topology and dependencies",
		"- `context.get_for_task` — full context for a task",
		"- `kg.add_fact` — add a knowledge graph fact",
		"",
		"## DAG Hierarchy Convention",
		"",
		"Focus uses a two-layer task model:",
		"- **Phase** (parent_task_id = null) = architectural phase, visible to humans in the DAG pane",
		"- **Step** (parent_task_id set) = execution detail, invisible to humans",
		"",
		"When you create tasks:",
		"1. Create 5-10 Phases first (no parent_task_id) representing architectural stages",
		"2. Then create Steps under each Phase (set parent_task_id)",
		"3. Any task without parent_task_id appears directly in the human's DAG pane",
	}
	return strings.Join(tools, "\n")
}

func (b *Bridge) renderPRD(task models.TaskContextRecord, plan *models.TaskPlanRecord) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("# %s", task.Title))
	parts = append(parts, "")
	if task.Goal != "" {
		parts = append(parts, fmt.Sprintf("**Goal:** %s", task.Goal))
	}
	if task.NextStep != "" {
		parts = append(parts, fmt.Sprintf("**Next Step:** %s", task.NextStep))
	}
	if plan != nil {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("**Plan:** %s", plan.Title))
		if plan.PlanBody != "" {
			parts = append(parts, plan.PlanBody)
		}
	}
	return strings.Join(parts, "\n")
}
