package trellis

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	store        models.Store
	taskSlugMap  map[string]string // Focus taskID → Trellis task dir
}

// NewBridge creates a new Trellis Bridge for the given repository and worktree.
func NewBridge(repoRoot, worktreeID string, store models.Store) *Bridge {
	return &Bridge{
		repoRoot:    repoRoot,
		worktreeID:  worktreeID,
		store:       store,
		taskSlugMap: make(map[string]string),
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

// SetRepoRoot switches the bridge to a different repository / worktree root.
// This resets trellisReady so EnsureInitialized re-runs for the new path.
func (b *Bridge) SetRepoRoot(repoRoot string) {
	if b.repoRoot == repoRoot {
		return
	}
	b.repoRoot = repoRoot
	b.trellisReady = false
	b.client = nil
}

// EnsureWorktreeLinks creates symlinks in the given worktree directory
// pointing to the main repo's .trellis/ and platform config directories
// (.kimi, .claude, .codex, .gemini, .opencode).
// This allows agent CLI tools launched inside a git worktree to discover
// Trellis and platform configurations even though the configs live in the
// main repository root.
func (b *Bridge) EnsureWorktreeLinks(worktreePath string) error {
	if b.repoRoot == "" || b.repoRoot == worktreePath {
		return nil // same directory, nothing to link
	}

	// Ensure worktree directory exists.
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		return fmt.Errorf("create worktree dir: %w", err)
	}

	// Link .trellis — critical for Claude SessionStart hook which does
	// NOT walk up the directory tree; it uses project_dir / ".trellis" directly.
	trellisLink := filepath.Join(worktreePath, ".trellis")
	trellisTarget := filepath.Join(b.repoRoot, ".trellis")
	if err := ensureSymlink(trellisLink, trellisTarget); err != nil {
		return fmt.Errorf("trellis link %s -> %s: %w", trellisLink, trellisTarget, err)
	}

	// Link platform config directories.
	for _, dir := range []string{".kimi", ".claude", ".codex", ".gemini", ".opencode"} {
		link := filepath.Join(worktreePath, dir)
		target := filepath.Join(b.repoRoot, dir)
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			if err := ensureSymlink(link, target); err != nil {
				log.Printf("platform link %s -> %s: %v", link, target, err)
			}
		}
	}
	return nil
}

// ensureSymlink creates a symlink at linkPath pointing to targetPath.
// If linkPath already exists as a directory (e.g. trellis init created it),
// it is removed and replaced with a symlink. If linkPath already exists as
// a symlink pointing to a different target, it is updated.
func ensureSymlink(linkPath, targetPath string) error {
	if linkPath == "" || targetPath == "" {
		return nil
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("target does not exist: %w", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Symlink(targetPath, linkPath)
		}
		return err
	}

	// Already a symlink — check if it points to the right target.
	if info.Mode()&os.ModeSymlink != 0 {
		currentTarget, err := os.Readlink(linkPath)
		if err == nil && currentTarget == targetPath {
			return nil // already correct
		}
		// Wrong target — remove and recreate.
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove stale symlink: %w", err)
		}
		return os.Symlink(targetPath, linkPath)
	}

	// Exists but is a regular file or directory — remove and replace with symlink.
	// This handles the case where trellis init (or another tool) created a
	// standalone config directory inside the worktree before we linked it.
	if err := os.RemoveAll(linkPath); err != nil {
		return fmt.Errorf("remove existing path for symlink: %w", err)
	}
	return os.Symlink(targetPath, linkPath)
}

// Version returns the installed trellis CLI version, or an empty string if
// trellis is not installed.
func (b *Bridge) Version() string {
	if b.client == nil {
		b.client = NewClient(b.repoRoot, "")
	}
	v, _ := b.client.Version()
	return v
}

// Update runs `trellis update -f` to update trellis configuration to the latest version.
func (b *Bridge) Update() error {
	if b.client == nil {
		b.client = NewClient(b.repoRoot, "")
	}
	cmd := exec.Command("trellis", "update", "-f")
	cmd.Dir = b.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trellis update: %w\n%s", err, string(out))
	}
	return nil
}

// MigrateLegacyFocus copies legacy .focus/ files into .trellis/ directories.
// It migrates: .focus/spec/ → .trellis/spec/ and .focus/handoff/ + .focus/journal/
// → .trellis/workspace/{developer}/journal/. Returns the number of files migrated.
func (b *Bridge) MigrateLegacyFocus() (int, error) {
	migrated := 0
	devName := b.gitUserName()

	// Migrate .focus/spec/*.md → .trellis/spec/
	legacySpecDir := filepath.Join(b.repoRoot, ".focus", "spec")
	if entries, err := os.ReadDir(legacySpecDir); err == nil {
		targetDir := filepath.Join(b.repoRoot, ".trellis", "spec")
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			src := filepath.Join(legacySpecDir, e.Name())
			dst := filepath.Join(targetDir, e.Name())
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				data, _ := os.ReadFile(src)
				if len(data) > 0 {
					_ = os.MkdirAll(targetDir, 0755)
					_ = os.WriteFile(dst, data, 0644)
					migrated++
				}
			}
		}
	}

	// Migrate .focus/handoff/*.md → .trellis/workspace/{dev}/journal/
	legacyHandoffDir := filepath.Join(b.repoRoot, ".focus", "handoff")
	if entries, err := os.ReadDir(legacyHandoffDir); err == nil {
		targetDir := filepath.Join(b.repoRoot, ".trellis", "workspace", devName, "journal")
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			src := filepath.Join(legacyHandoffDir, e.Name())
			dst := filepath.Join(targetDir, e.Name())
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				data, _ := os.ReadFile(src)
				if len(data) > 0 {
					_ = os.MkdirAll(targetDir, 0755)
					_ = os.WriteFile(dst, data, 0644)
					migrated++
				}
			}
		}
	}

	// Migrate .focus/journal/*.md → .trellis/workspace/{dev}/journal/
	legacyJournalDir := filepath.Join(b.repoRoot, ".focus", "journal")
	if entries, err := os.ReadDir(legacyJournalDir); err == nil {
		targetDir := filepath.Join(b.repoRoot, ".trellis", "workspace", devName, "journal")
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			src := filepath.Join(legacyJournalDir, e.Name())
			dst := filepath.Join(targetDir, e.Name())
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				data, _ := os.ReadFile(src)
				if len(data) > 0 {
					_ = os.MkdirAll(targetDir, 0755)
					_ = os.WriteFile(dst, data, 0644)
					migrated++
				}
			}
		}
	}

	return migrated, nil
}

// ListSpecs returns the paths of all Markdown spec files under .trellis/spec/.
func (b *Bridge) ListSpecs() ([]string, error) {
	specDir := filepath.Join(b.repoRoot, ".trellis", "spec")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil, err
	}
	var specs []string
	for _, e := range entries {
		if e.IsDir() {
			// Read sub-directory for domain specs.
			subDir := filepath.Join(specDir, e.Name())
			subEntries, err := os.ReadDir(subDir)
			if err != nil {
				continue
			}
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(se.Name(), ".md") {
					specs = append(specs, filepath.Join(".trellis/spec", e.Name(), se.Name()))
				}
			}
		} else if strings.HasSuffix(e.Name(), ".md") {
			specs = append(specs, filepath.Join(".trellis/spec", e.Name()))
		}
	}
	return specs, nil
}

// LegacyFocusMigrationNeeded reports whether legacy .focus/ files exist that
// should be migrated to .trellis/.
func (b *Bridge) LegacyFocusMigrationNeeded() bool {
	focusDir := filepath.Join(b.repoRoot, ".focus")
	if _, err := os.Stat(focusDir); os.IsNotExist(err) {
		return false
	}
	// Check for spec or handoff subdirectories.
	for _, sub := range []string{"spec", "handoff", "journal"} {
		if _, err := os.Stat(filepath.Join(focusDir, sub)); err == nil {
			return true
		}
	}
	return false
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
		log.Printf("trellis not found in PATH: %v", err)
		return ErrTrellisNotInstalled
	}

	// 2. Detect python3.
	python, err := resolvePythonCommand()
	if err != nil {
		log.Printf("python3 not found: %v", err)
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
			log.Printf("trellis init failed: %v", err)
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

	// Layer B: task-scoped Trellis context.
	var trellisCtx string
	if taskSlug != "" {
		trellisCtx, err = b.buildTrellisTaskContext(taskSlug)
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
	if err := pm.WriteRootFiles(spec); err != nil {
		return nil, fmt.Errorf("write root agent files: %w", err)
	}
	if err := pm.WriteAGENTSMD(spec); err != nil {
		return nil, fmt.Errorf("write focus agent spec: %w", err)
	}

	return spec, nil
}

func (b *Bridge) buildTrellisTaskContext(taskSlug string) (string, error) {
	taskDir := b.taskDirPath(taskSlug)
	taskJSONPath := filepath.Join(taskDir, "task.json")
	data, err := os.ReadFile(taskJSONPath)
	if err != nil {
		return "", err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}

	var lines []string
	lines = append(lines, "## Trellis Task")
	if title, _ := raw["title"].(string); title != "" {
		lines = append(lines, fmt.Sprintf("Title: %s", title))
	}
	if name, _ := raw["name"].(string); name != "" {
		lines = append(lines, fmt.Sprintf("Path: %s", taskSlug))
	}
	if status, _ := raw["status"].(string); status != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", status))
	}
	if description, _ := raw["description"].(string); description != "" {
		lines = append(lines, fmt.Sprintf("Description: %s", description))
	}
	if priority, _ := raw["priority"].(string); priority != "" {
		lines = append(lines, fmt.Sprintf("Priority: %s", priority))
	}

	for _, item := range []struct {
		title string
		file  string
	}{
		{title: "PRD", file: "prd.md"},
		{title: "Implementation Context", file: "implement.jsonl"},
		{title: "Check Context", file: "check.jsonl"},
		{title: "Notes", file: "notes.md"},
	} {
		path := filepath.Join(taskDir, item.file)
		content, err := os.ReadFile(path)
		if err != nil || len(strings.TrimSpace(string(content))) == 0 {
			continue
		}
		lines = append(lines, "", "## "+item.title, strings.TrimSpace(string(content)))
	}

	return strings.Join(lines, "\n"), nil
}

// SyncTaskCreate creates a Trellis task from a Focus task record.
// If a Trellis task already has meta.focus_task_id for this Focus task, it
// returns that existing directory.
func (b *Bridge) SyncTaskCreate(task models.TaskContextRecord, plan *models.TaskPlanRecord) (string, error) {
	if err := b.EnsureInitialized(); err != nil {
		return "", err
	}

	// Check if already mapped.
	if existing := b.taskSlugMap[task.ID]; existing != "" {
		return existing, nil
	}

	// Check if task already exists in Trellis (by scanning task list).
	existingTasks, _ := b.client.TaskList()
	expectedSlug := taskSlugFromTitle(task.Title)
	for _, et := range existingTasks {
		if et.ID == task.ID {
			b.taskSlugMap[task.ID] = et.Name
			return et.Name, nil
		}
	}

	opts := TaskCreateOpts{
		Slug:     expectedSlug,
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

	// Write Focus task ID into task.json meta so resolveTaskSlug can find it later.
	_ = b.writeFocusTaskIDMeta(dir, task.ID)

	b.taskSlugMap[task.ID] = dir
	return dir, nil
}

func (b *Bridge) writeFocusTaskIDMeta(taskDir, focusTaskID string) error {
	taskJSONPath := b.taskFilePath(taskDir, "task.json")
	data, err := os.ReadFile(taskJSONPath)
	if err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	meta, ok := obj["meta"].(map[string]any)
	if !ok {
		meta = make(map[string]any)
		obj["meta"] = meta
	}
	meta["focus_task_id"] = focusTaskID
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(taskJSONPath, out, 0644)
}

func (b *Bridge) markTaskCompleted(taskDir string) error {
	taskJSONPath := b.taskFilePath(taskDir, "task.json")
	data, err := os.ReadFile(taskJSONPath)
	if err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	obj["status"] = "completed"
	if _, ok := obj["completedAt"]; !ok || obj["completedAt"] == nil || obj["completedAt"] == "" {
		obj["completedAt"] = time.Now().Format(time.RFC3339)
	}
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(taskJSONPath, out, 0644)
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
	slug, err := b.resolveTaskSlug(taskID)
	if err != nil {
		return err
	}
	return b.markTaskCompleted(slug)
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

// RecordSessionByTitle appends a session entry using just a title.
func (b *Bridge) RecordSessionByTitle(title string) error {
	if err := b.EnsureInitialized(); err != nil {
		return err
	}
	return b.client.AddSession(title, []string{})
}

// AddTaskOutput appends an output note to a Trellis task directory.
func (b *Bridge) AddTaskOutput(taskID, output string) error {
	if err := b.EnsureInitialized(); err != nil {
		return err
	}
	slug, err := b.resolveTaskSlug(taskID)
	if err != nil {
		return err
	}
	notesPath := b.taskFilePath(slug, "notes.md")
	entry := fmt.Sprintf("\n## Output (%s)\n\n%s\n", time.Now().Format(time.RFC3339), output)
	return appendToFile(notesPath, entry)
}

// AddKnowledgeFact writes a knowledge fact into the auto-discovered spec file.
func (b *Bridge) AddKnowledgeFact(subject, predicate, object string) error {
	return b.WriteSpecFact("general", subject, predicate, object)
}

// WriteSpecFact writes a knowledge fact into the auto-discovered spec file.
func (b *Bridge) WriteSpecFact(domain, subject, predicate, object string) error {
	if domain == "" {
		domain = "general"
	}
	specPath := filepath.Join(b.repoRoot, ".trellis", "spec", domain, "auto-discovered.md")
	entry := fmt.Sprintf("- **%s %s %s** (discovered by agent)\n", subject, predicate, object)
	factKey := fmt.Sprintf("**%s %s %s**", subject, predicate, object)
	if data, err := os.ReadFile(specPath); err == nil {
		content := string(data)
		if strings.Contains(content, factKey) || strings.Contains(content, strings.TrimSpace(entry)) {
			return nil
		}
	}
	return appendToFile(specPath, entry)
}

// WritePRD generates or updates the PRD for a task directory.
func (b *Bridge) WritePRD(taskDir string, task models.TaskContextRecord, plan *models.TaskPlanRecord) error {
	prdPath := b.taskFilePath(taskDir, "prd.md")
	content := b.renderPRD(task, plan)
	return os.WriteFile(prdPath, []byte(content), 0644)
}

// UpdateWorkflowState records the latest Focus plan-step state in Trellis runtime
// storage. workflow.md is the workflow source of truth and must not be mutated
// by runtime plan updates.
func (b *Bridge) UpdateWorkflowState(planID string, step models.PlanStepRecord) error {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		planID = strings.TrimSpace(step.PlanID)
	}
	if planID == "" {
		return fmt.Errorf("planID required")
	}
	stateDir := filepath.Join(b.repoRoot, ".trellis", ".runtime", "workflow-state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	entry := map[string]any{
		"planID":     planID,
		"stepID":     step.ID,
		"title":      step.Title,
		"orderIndex": step.OrderIndex,
		"state":      step.State,
		"notes":      step.Notes,
		"updatedAt":  time.Now().Format(time.RFC3339),
	}
	out, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	out = append(out, '\n')
	statePath := filepath.Join(stateDir, planID+".jsonl")
	return appendToFile(statePath, string(out))
}

func (b *Bridge) workflowStateForTask(taskID string) string {
	if b.store == nil || taskID == "" {
		return ""
	}
	planID := ""
	if plans, err := b.store.ListTaskPlans(taskID); err == nil {
		for _, plan := range plans {
			if plan.TaskID == taskID {
				planID = plan.ID
				break
			}
		}
	}
	if planID == "" {
		return ""
	}
	statePath := filepath.Join(b.repoRoot, ".trellis", ".runtime", "workflow-state", planID+".jsonl")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry struct {
			Title string `json:"title"`
			State string `json:"state"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Title == "" && entry.State == "" {
			continue
		}
		if entry.State == "" {
			return entry.Title
		}
		if entry.Title == "" {
			return entry.State
		}
		return fmt.Sprintf("%s: %s", entry.Title, entry.State)
	}
	return ""
}

// GetTaskContextExtended returns extended task context from Trellis.
func (b *Bridge) GetTaskContextExtended(taskID string) (*ExtendedTaskContext, error) {
	if err := b.EnsureInitialized(); err != nil {
		return nil, err
	}
	slug, err := b.resolveTaskSlug(taskID)
	if err != nil {
		return nil, err
	}
	taskDir := b.taskDirPath(slug)

	ext := &ExtendedTaskContext{}

	// task.json
	taskJSONPath := filepath.Join(taskDir, "task.json")
	if data, err := os.ReadFile(taskJSONPath); err == nil {
		_ = json.Unmarshal(data, &ext.Task)
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err == nil {
			if meta, ok := raw["meta"].(map[string]any); ok {
				if focusID, ok := meta["focus_task_id"].(string); ok && focusID != "" && ext.Task != nil {
					ext.Task.ID = focusID
				}
			}
		}
	}

	// prd.md
	prdPath := filepath.Join(taskDir, "prd.md")
	if data, err := os.ReadFile(prdPath); err == nil {
		ext.PRD = string(data)
	}

	// implement.jsonl
	implPath := filepath.Join(taskDir, "implement.jsonl")
	if data, err := os.ReadFile(implPath); err == nil {
		ext.ImplementJSONL = string(data)
	}

	// journal.md if present
	journalPath := filepath.Join(taskDir, "journal.md")
	if data, err := os.ReadFile(journalPath); err == nil {
		ext.Journal = string(data)
	}

	ext.WorkflowState = b.workflowStateForTask(taskID)

	return ext, nil
}

// ExtendedTaskContext holds the full Trellis context for a task.
type ExtendedTaskContext struct {
	Task           *TrellisTask
	PRD            string
	Specs          []string
	Handoff        string
	Journal        string
	WorkflowState  string
	ImplementJSONL string
}

// --- helpers ---

func (b *Bridge) syncTaskIfNeeded(taskID string) (string, error) {
	// 1. Fast path: in-memory map.
	if dir, ok := b.taskSlugMap[taskID]; ok && dir != "" {
		return dir, nil
	}
	// 2. Fallback: scan Trellis task list.
	tasks, err := b.client.TaskList()
	if err == nil {
		for _, t := range tasks {
			if t.ID == taskID {
				b.taskSlugMap[taskID] = t.Name
				return t.Name, nil
			}
		}
	}
	// 3. No mapping found: create Trellis task from Focus record.
	if b.store != nil {
		task, err := b.store.GetTaskContext(taskID)
		if err != nil || task == nil {
			return "", fmt.Errorf("focus task %s not found: %w", taskID, err)
		}
		var plan *models.TaskPlanRecord
		if plans, _ := b.store.ListTaskPlans(taskID); len(plans) > 0 {
			plan = &plans[0]
		}
		dir, err := b.SyncTaskCreate(*task, plan)
		if err != nil {
			return "", fmt.Errorf("sync task create: %w", err)
		}
		return dir, nil
	}
	return "", fmt.Errorf("no store available to sync task %s", taskID)
}

func (b *Bridge) resolveTaskSlug(taskID string) (string, error) {
	if dir, ok := b.taskSlugMap[taskID]; ok && dir != "" {
		return dir, nil
	}
	// Fallback: scan Trellis task list for a task whose directory name
	// or metadata contains the Focus task ID.
	tasks, err := b.client.TaskList()
	if err != nil {
		return "", fmt.Errorf("list trellis tasks: %w", err)
	}
	for _, t := range tasks {
		if t.ID == taskID {
			b.taskSlugMap[taskID] = t.Name
			return t.Name, nil
		}
	}
	return "", fmt.Errorf("no trellis task found for focus task %s", taskID)
}

func (b *Bridge) taskDirPath(taskDir string) string {
	cleaned := filepath.Clean(taskDir)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	if strings.HasPrefix(cleaned, ".trellis"+string(os.PathSeparator)) || strings.HasPrefix(cleaned, ".trellis/") {
		return filepath.Join(b.repoRoot, cleaned)
	}
	return filepath.Join(b.repoRoot, ".trellis", "tasks", cleaned)
}

func (b *Bridge) taskFilePath(taskDir, name string) string {
	return filepath.Join(b.taskDirPath(taskDir), name)
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
			ver := strings.TrimSpace(string(out))
			if isPythonVersionOK(ver) {
				return path, nil
			}
		}
	}
	return "", ErrPythonNotFound
}

func isPythonVersionOK(ver string) bool {
	// Expected format: "Python 3.11.4" or "Python 3.9.0"
	parts := strings.Fields(ver)
	if len(parts) < 2 {
		return false
	}
	var major, minor int
	if _, err := fmt.Sscanf(parts[1], "%d.%d", &major, &minor); err != nil {
		return false
	}
	return major > 3 || (major == 3 && minor >= 9)
}
