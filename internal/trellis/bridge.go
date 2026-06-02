package trellis

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"focus/internal/agents"
	"focus/internal/models"
)

// Bridge is the primary adapter between Focus and Trellis.
type Bridge struct {
	repoRoot     string
	worktreeID   string
	pythonCmd    string
	trellisReady bool
	client       *Client
}

// NewBridge creates a new Trellis Bridge for the given repository and worktree.
func NewBridge(repoRoot, worktreeID string) *Bridge {
	return &Bridge{
		repoRoot:   repoRoot,
		worktreeID: worktreeID,
	}
}

// RepoRoot returns the repository root directory.
func (b *Bridge) RepoRoot() string {
	return b.repoRoot
}

// SetWorktreeID updates the worktree ID for this bridge.
func (b *Bridge) SetWorktreeID(id string) {
	b.worktreeID = id
}

// EnsureInitialized checks that Trellis is installed and the .trellis/ directory
// exists, running `trellis init` if necessary. It also generates Kimi-specific
// adapters since Trellis does not natively support Kimi as a platform.
func (b *Bridge) EnsureInitialized() error {
	if b.trellisReady {
		return nil
	}

	// 1. Detect trellis CLI.
	if _, err := exec.LookPath("trellis"); err != nil {
		return ErrTrellisNotInstalled
	}

	// 2. Detect python3.
	python, err := resolvePythonCommand()
	if err != nil {
		return err
	}
	b.pythonCmd = python
	b.client = NewClient(b.repoRoot, b.pythonCmd)

	// 3. Ensure .trellis/ exists.
	trellisDir := filepath.Join(b.repoRoot, ".trellis")
	if _, err := os.Stat(trellisDir); os.IsNotExist(err) {
		platforms := b.detectInstalledPlatforms()
		flags := b.buildTrellisInitFlags(platforms)
		devName := b.gitUserName()
		if err := b.client.Init(devName, flags); err != nil {
			return fmt.Errorf("%w: %v", ErrTrellisInitFailed, err)
		}
	}

	// 4. Kimi adapter: generate .kimi/ configuration.
	if err := b.ensureKimiAdapter(); err != nil {
		return fmt.Errorf("kimi adapter: %w", err)
	}

	b.trellisReady = true
	return nil
}

// BuildAgentContext assembles the full agent context from Focus metadata (Layer A)
// and Trellis context (Layer B), then writes the entry files (Layer C).
func (b *Bridge) BuildAgentContext(session *agents.Session) (*agents.AgentSpec, error) {
	if err := b.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Sync task if needed.
	taskSlug, err := b.syncTaskIfNeeded(session.TaskID)
	if err != nil {
		return nil, err
	}

	// Layer A: Focus metadata.
	focusMeta, err := b.buildFocusMetadataLayer(session)
	if err != nil {
		return nil, err
	}

	// Layer B: Trellis context via get_context.py.
	var trellisCtx string
	if taskSlug != "" {
		trellisCtx, err = b.client.GetContext("--task", taskSlug)
		if err != nil {
			return nil, err
		}
	}

	// Assemble spec.
	spec := agents.NewAgentSpec()
	spec.AddLayer(focusMeta)
	if trellisCtx != "" {
		spec.AddLayerFromMarkdown(trellisCtx)
	}

	// Layer C: write entry files.
	pm := agents.NewProfileManager(b.worktreeID)
	pm.WriteRootFiles(spec)
	pm.WriteAGENTSMD(spec)

	return spec, nil
}

// SyncTaskCreate creates a Trellis task from a Focus task record.
func (b *Bridge) SyncTaskCreate(task models.TaskContextRecord, plan *models.TaskPlanRecord) (string, error) {
	if err := b.EnsureInitialized(); err != nil {
		return "", err
	}
	opts := TaskCreateOpts{
		Slug:     taskSlugFromTitle(task.Title),
		Priority: focusPriorityToTrellis(task.Priority),
	}
	if plan != nil {
		opts.Description = plan.PlanBody
	}
	dir, err := b.client.TaskCreate(task.Title, opts)
	if err != nil {
		return "", err
	}

	// Write PRD if plan exists.
	if plan != nil {
		b.WritePRD(dir, task, plan)
	}

	return dir, nil
}

// SyncTaskStart marks a task as in_progress in Trellis.
func (b *Bridge) SyncTaskStart(taskID string) error {
	if err := b.EnsureInitialized(); err != nil {
		return err
	}
	slug, err := b.resolveTaskSlug(taskID)
	if err != nil {
		return err
	}
	return b.client.TaskStart(slug)
}

// SyncTaskFinish marks a task as completed in Trellis.
func (b *Bridge) SyncTaskFinish(taskID string) error {
	if err := b.EnsureInitialized(); err != nil {
		return err
	}
	_, err := b.resolveTaskSlug(taskID)
	if err != nil {
		return err
	}
	return b.client.TaskFinish()
}

// SyncTaskArchive archives a task in Trellis.
func (b *Bridge) SyncTaskArchive(taskID string) error {
	if err := b.EnsureInitialized(); err != nil {
		return err
	}
	slug, err := b.resolveTaskSlug(taskID)
	if err != nil {
		return err
	}
	return b.client.TaskArchive(slug)
}

// RecordSession appends a session entry to the Trellis journal.
func (b *Bridge) RecordSession(session *agents.Session, handoff *models.SessionHandoffRecord) error {
	if err := b.EnsureInitialized(); err != nil {
		return err
	}
	return b.client.AddSession(session.DisplayTitle, []string{})
}

// WriteSpecFact writes a knowledge fact into the auto-discovered spec file.
func (b *Bridge) WriteSpecFact(domain, subject, predicate, object string) error {
	if domain == "" {
		domain = "general"
	}
	specPath := filepath.Join(b.repoRoot, ".trellis", "spec", domain, "auto-discovered.md")
	entry := fmt.Sprintf("- **%s %s %s** (discovered by agent)\n", subject, predicate, object)
	return appendToFile(specPath, entry)
}

// WritePRD generates or updates the PRD for a task directory.
func (b *Bridge) WritePRD(taskDir string, task models.TaskContextRecord, plan *models.TaskPlanRecord) error {
	prdPath := filepath.Join(taskDir, "prd.md")
	content := b.renderPRD(task, plan)
	return os.WriteFile(prdPath, []byte(content), 0644)
}

// UpdateWorkflowState updates the workflow.md with the current step state.
func (b *Bridge) UpdateWorkflowState(planID string, step models.PlanStepRecord) error {
	// TODO: read workflow.md, update [workflow-state:...] blocks.
	return nil
}

// GetTaskContextExtended returns extended task context from Trellis.
func (b *Bridge) GetTaskContextExtended(taskID string) (*ExtendedTaskContext, error) {
	// TODO: read task.json, prd.md, implement.jsonl from .trellis/tasks/{slug}/
	return nil, nil
}

// ExtendedTaskContext holds the full Trellis context for a task.
type ExtendedTaskContext struct {
	Task          *TrellisTask
	PRD           string
	Specs         []string
	Handoff       string
	Journal       string
	WorkflowState string
}

// --- helpers ---

func (b *Bridge) syncTaskIfNeeded(taskID string) (string, error) {
	// TODO: maintain an in-memory or DB mapping from Focus task ID -> Trellis slug.
	// For now, return empty to let get_context.py use the default active task.
	return "", nil
}

func (b *Bridge) resolveTaskSlug(taskID string) (string, error) {
	// TODO: lookup mapping.
	return "", nil
}

func (b *Bridge) detectInstalledPlatforms() []agents.Provider {
	var platforms []agents.Provider
	for _, id := range []string{"claude", "kimi", "codex", "gemini", "opencode"} {
		if _, err := exec.LookPath(id); err == nil {
			switch id {
			case "claude":
				platforms = append(platforms, agents.ProviderClaude)
			case "kimi":
				platforms = append(platforms, agents.ProviderKimi)
			case "codex":
				platforms = append(platforms, agents.ProviderCodex)
			case "gemini":
				platforms = append(platforms, agents.ProviderGemini)
			case "opencode":
				platforms = append(platforms, agents.ProviderOpenCode)
			}
		}
	}
	return platforms
}

func (b *Bridge) buildTrellisInitFlags(providers []agents.Provider) []string {
	var flags []string
	for _, p := range providers {
		switch p {
		case agents.ProviderClaude:
			flags = append(flags, "--claude")
		case agents.ProviderCodex:
			flags = append(flags, "--codex")
		case agents.ProviderOpenCode:
			flags = append(flags, "--opencode")
		case agents.ProviderGemini:
			flags = append(flags, "--gemini")
		case agents.ProviderKimi:
			// Trellis has no --kimi flag; .agents/skills/ is written automatically.
		}
	}
	return flags
}

func (b *Bridge) gitUserName() string {
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = b.repoRoot
	out, _ := cmd.Output()
	name := string(out)
	if name == "" {
		name = "developer"
	}
	return name
}

func resolvePythonCommand() (string, error) {
	for _, cmd := range []string{"python3", "python", "py"} {
		if path, err := exec.LookPath(cmd); err == nil {
			// Verify version >= 3.9.
			verCmd := exec.Command(path, "--version")
			out, _ := verCmd.Output()
			_ = out // TODO: parse version.
			return path, nil
		}
	}
	return "", ErrPythonNotFound
}


