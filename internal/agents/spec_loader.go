package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"focus/internal/models"
)

// SpecLoadContext carries everything needed to assemble a context-aware
// agent spec.
type SpecLoadContext struct {
	Task    *models.TaskContextRecord
	Plan    *models.TaskPlanRecord
	Step    *models.PlanStepRecord
	Handoff *models.SessionHandoffRecord
}

// SpecLoader progressively assembles an AgentSpec from layered sources.
type SpecLoader struct {
	// RepoSpecDir is the project-level spec directory.
	// Defaults to repo-root/.focus/spec/ if left empty.
	RepoSpecDir string

	// WorktreeSpecDir is the worktree-level spec directory.
	// Usually {worktree}/.focus/spec/.
	WorktreeSpecDir string
}

// NewSpecLoader creates a loader with the given directories.
func NewSpecLoader(repoSpecDir, worktreeSpecDir string) *SpecLoader {
	return &SpecLoader{
		RepoSpecDir:     repoSpecDir,
		WorktreeSpecDir: worktreeSpecDir,
	}
}

// Load builds an AgentSpec by layering context from general to specific.
//
// Layer order:
//   1. Repo general spec      (coding standards, architecture)
//   2. Repo domain spec       (backend/frontend/database rules)
//   3. Worktree context       (task goal, plan body)
//   4. Step-specific context  (current step objective)
//   5. Handoff context        (previous session summary)
func (l *SpecLoader) Load(ctx SpecLoadContext) (*AgentSpec, error) {
	spec := NewAgentSpec()

	// Layer 1: Project-wide standards.
	if layer := l.loadRepoLayer("general"); layer != nil {
		spec.AddLayer(*layer)
	}

	// Layer 2: Domain-specific guidelines.
	if ctx.Task != nil || ctx.Step != nil {
		domain := inferDomain(ctx)
		if domain != "" {
			if layer := l.loadRepoLayer(domain); layer != nil {
				spec.AddLayer(*layer)
			}
		}
	}

	// Layer 3: Worktree task context.
	if layer := l.buildWorktreeContextLayer(ctx); layer != nil {
		spec.AddLayer(*layer)
	}

	// Layer 4: Current step context.
	if layer := l.buildStepLayer(ctx); layer != nil {
		spec.AddLayer(*layer)
	}

	// Layer 5: Handoff from previous session.
	if ctx.Handoff != nil {
		if layer := l.buildHandoffLayer(ctx.Handoff); layer != nil {
			spec.AddLayer(*layer)
		}
	}

	return spec, nil
}

// loadRepoLayer reads a Markdown file from RepoSpecDir named {name}.md.
// If the file does not exist it returns nil (soft failure — specs are
// optional).
func (l *SpecLoader) loadRepoLayer(name string) *SpecLayer {
	if l.RepoSpecDir == "" {
		return nil
	}
	path := filepath.Join(l.RepoSpecDir, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}
	return &SpecLayer{
		Title:   titleForLayer(name),
		Content: content,
		Source:  "repo",
	}
}

// buildWorktreeContextLayer creates a layer from the task and plan metadata.
func (l *SpecLoader) buildWorktreeContextLayer(ctx SpecLoadContext) *SpecLayer {
	if ctx.Task == nil {
		return nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("**Task:** %s", ctx.Task.Title))
	if ctx.Task.Goal != "" {
		parts = append(parts, fmt.Sprintf("**Goal:** %s", ctx.Task.Goal))
	}
	if ctx.Task.NextStep != "" {
		parts = append(parts, fmt.Sprintf("**Next Step:** %s", ctx.Task.NextStep))
	}

	if ctx.Plan != nil {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("**Plan:** %s", ctx.Plan.Title))
		if ctx.Plan.PlanBody != "" {
			parts = append(parts, ctx.Plan.PlanBody)
		}
	}

	content := strings.Join(parts, "\n")
	if content == "" {
		return nil
	}

	return &SpecLayer{
		Title:   "Task Context",
		Content: content,
		Source:  "worktree",
	}
}

// buildStepLayer creates a layer for the current plan step.
func (l *SpecLoader) buildStepLayer(ctx SpecLoadContext) *SpecLayer {
	if ctx.Step == nil {
		return nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("**Current Step:** %s", ctx.Step.Title))
	if ctx.Step.State != "" {
		parts = append(parts, fmt.Sprintf("**Status:** %s", ctx.Step.State))
	}
	if ctx.Step.Notes != "" {
		parts = append(parts, fmt.Sprintf("**Notes:** %s", ctx.Step.Notes))
	}

	content := strings.Join(parts, "\n")
	return &SpecLayer{
		Title:   fmt.Sprintf("Step: %s", ctx.Step.Title),
		Content: content,
		Source:  "step",
	}
}

// buildHandoffLayer creates a layer from a previous session handoff.
func (l *SpecLoader) buildHandoffLayer(h *models.SessionHandoffRecord) *SpecLayer {
	var parts []string
	parts = append(parts, "Previous session summary:")
	if h.DoneSummary != "" {
		parts = append(parts, fmt.Sprintf("- **Completed:** %s", h.DoneSummary))
	}
	if h.RemainingSummary != "" {
		parts = append(parts, fmt.Sprintf("- **Remaining:** %s", h.RemainingSummary))
	}
	if h.DecisionSummary != "" {
		parts = append(parts, fmt.Sprintf("- **Decisions:** %s", h.DecisionSummary))
	}
	if h.BlockerSummary != "" {
		parts = append(parts, fmt.Sprintf("- **Blockers:** %s", h.BlockerSummary))
	}
	if h.Entrypoint != "" {
		parts = append(parts, fmt.Sprintf("- **Entrypoint:** %s", h.Entrypoint))
	}

	content := strings.Join(parts, "\n")
	if content == "" {
		return nil
	}

	return &SpecLayer{
		Title:   "Previous Session Handoff",
		Content: content,
		Source:  "handoff",
	}
}

// inferDomain guesses the technical domain from task/step metadata so that
// the loader can pull the most relevant spec layer.
func inferDomain(ctx SpecLoadContext) string {
	text := ""
	if ctx.Task != nil {
		text += strings.ToLower(ctx.Task.Goal + " " + ctx.Task.Title)
	}
	if ctx.Step != nil {
		text += " " + strings.ToLower(ctx.Step.Title)
	}
	if text == "" {
		return ""
	}

	keywords := map[string][]string{
		"backend":   {"api", "server", "backend", "database", "db", "sql", "postgres", "redis", "grpc", "rest"},
		"frontend":  {"ui", "frontend", "react", "vue", "component", "html", "css", "dom", "browser"},
		"database":  {"database", "db", "schema", "migration", "sql", "postgres", "sqlite", "index"},
		"devops":    {"docker", "kubernetes", "k8s", "ci", "cd", "deploy", "infra", "terraform"},
		"testing":   {"test", "testing", "spec", "jest", "pytest", "coverage", "e2e"},
		"security":  {"auth", "oauth", "jwt", "security", "encrypt", "permission", "rbac"},
		"cli":       {"cli", "command", "terminal", "tui", "shell", "script"},
	}

	scores := make(map[string]int)
	for domain, words := range keywords {
		for _, w := range words {
			if strings.Contains(text, w) {
				scores[domain]++
			}
		}
	}

	best := ""
	bestScore := 0
	for domain, score := range scores {
		if score > bestScore {
			best = domain
			bestScore = score
		}
	}
	return best
}

func titleForLayer(name string) string {
	switch name {
	case "general":
		return "Project Standards"
	case "backend":
		return "Backend Guidelines"
	case "frontend":
		return "Frontend Guidelines"
	case "database":
		return "Database Guidelines"
	case "devops":
		return "DevOps Guidelines"
	case "testing":
		return "Testing Guidelines"
	case "security":
		return "Security Guidelines"
	case "cli":
		return "CLI Guidelines"
	default:
		return strings.Title(name) + " Guidelines"
	}
}
