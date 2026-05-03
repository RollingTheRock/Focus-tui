package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProfilePaths holds the on-disk locations for a worktree's agent profile.
type ProfilePaths struct {
	WorktreeID string
	BaseDir    string // {worktree}/.focus/
	SpecDir    string // {worktree}/.focus/spec/
	HandoffDir string // {worktree}/.focus/handoff/
	JournalDir string // {worktree}/.focus/journal/
}

// NewProfilePaths creates paths for the given worktree.
func NewProfilePaths(worktreeID string) ProfilePaths {
	base := filepath.Join(worktreeID, ".focus")
	return ProfilePaths{
		WorktreeID: worktreeID,
		BaseDir:    base,
		SpecDir:    filepath.Join(base, "spec"),
		HandoffDir: filepath.Join(base, "handoff"),
		JournalDir: filepath.Join(base, "journal"),
	}
}

// Ensure creates the .focus/ directory tree if it does not exist.
func (p ProfilePaths) Ensure() error {
	for _, dir := range []string{p.SpecDir, p.HandoffDir, p.JournalDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// AGENTSMDPath returns the path to the generated AGENTS.md.
func (p ProfilePaths) AGENTSMDPath() string {
	return filepath.Join(p.SpecDir, "AGENTS.md")
}

// ContextMDPath returns the path to the standalone context file.
func (p ProfilePaths) ContextMDPath() string {
	return filepath.Join(p.SpecDir, "context.md")
}

// ConstraintsMDPath returns the path to the constraints file.
func (p ProfilePaths) ConstraintsMDPath() string {
	return filepath.Join(p.SpecDir, "constraints.md")
}

// HandoffPath returns the path for a specific session handoff.
func (p ProfilePaths) HandoffPath(sessionID string) string {
	return filepath.Join(p.HandoffDir, fmt.Sprintf("prev-%s.md", sessionID))
}

// LatestHandoffPath returns the path for the latest handoff symlink/copy.
func (p ProfilePaths) LatestHandoffPath() string {
	return filepath.Join(p.HandoffDir, "latest.md")
}

// JournalPath returns the path for a developer's journal.
func (p ProfilePaths) JournalPath(developerName string) string {
	if developerName == "" {
		developerName = "default"
	}
	return filepath.Join(p.JournalDir, fmt.Sprintf("%s.md", developerName))
}

// ProfileManager writes and maintains the per-worktree agent profile.
type ProfileManager struct {
	Paths ProfilePaths
}

// NewProfileManager creates a manager for the given worktree.
func NewProfileManager(worktreeID string) *ProfileManager {
	return &ProfileManager{Paths: NewProfilePaths(worktreeID)}
}

// Prepare ensures the directory tree exists.
func (pm *ProfileManager) Prepare() error {
	return pm.Paths.Ensure()
}

// WriteAGENTSMD writes the assembled spec to AGENTS.md.
func (pm *ProfileManager) WriteAGENTSMD(spec *AgentSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}
	if err := pm.Prepare(); err != nil {
		return err
	}
	data := []byte(spec.ToMarkdown())
	return os.WriteFile(pm.Paths.AGENTSMDPath(), data, 0644)
}

// WriteContextMD writes a standalone context file (useful for agents that
// read multiple files rather than a single AGENTS.md).
func (pm *ProfileManager) WriteContextMD(content string) error {
	if err := pm.Prepare(); err != nil {
		return err
	}
	return os.WriteFile(pm.Paths.ContextMDPath(), []byte(content), 0644)
}

// WriteConstraintsMD writes the constraints file.
func (pm *ProfileManager) WriteConstraintsMD(content string) error {
	if err := pm.Prepare(); err != nil {
		return err
	}
	return os.WriteFile(pm.Paths.ConstraintsMDPath(), []byte(content), 0644)
}

// WriteHandoff persists a session handoff note.
func (pm *ProfileManager) WriteHandoff(sessionID string, content string) error {
	if err := pm.Prepare(); err != nil {
		return err
	}
	path := pm.Paths.HandoffPath(sessionID)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	// Also write a "latest" copy for easy discovery.
	latest := pm.Paths.LatestHandoffPath()
	return os.WriteFile(latest, []byte(content), 0644)
}

// ReadLatestHandoff returns the content of the latest handoff if it exists.
func (pm *ProfileManager) ReadLatestHandoff() (string, error) {
	path := pm.Paths.LatestHandoffPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JournalEntry is a single entry in the workspace journal.
type JournalEntry struct {
	SessionID     string
	TaskTitle     string
	StepTitle     string
	Duration      time.Duration
	Completed     string
	Blockers      string
	NextSteps     string
	KeyDecisions  []string
	Timestamp     time.Time
}

// WriteJournal appends an entry to the developer's journal.
func (pm *ProfileManager) WriteJournal(developerName string, entry JournalEntry) error {
	if err := pm.Prepare(); err != nil {
		return err
	}
	path := pm.Paths.JournalPath(developerName)

	var b []byte
	if existing, err := os.ReadFile(path); err == nil {
		b = existing
	}

	section := formatJournalEntry(entry)
	b = append(b, []byte(section)...)

	return os.WriteFile(path, b, 0644)
}

func formatJournalEntry(e JournalEntry) string {
	ts := e.Timestamp.Format("2006-01-02 15:04")
	var parts []string
	parts = append(parts, fmt.Sprintf("\n## %s Session\n", ts))
	if e.TaskTitle != "" {
		parts = append(parts, fmt.Sprintf("- **Task:** %s", e.TaskTitle))
	}
	if e.StepTitle != "" {
		parts = append(parts, fmt.Sprintf("- **Step:** %s", e.StepTitle))
	}
	if e.Duration > 0 {
		parts = append(parts, fmt.Sprintf("- **Duration:** %s", e.Duration.Round(time.Second)))
	}
	if e.Completed != "" {
		parts = append(parts, fmt.Sprintf("- **Completed:** %s", e.Completed))
	}
	if e.Blockers != "" {
		parts = append(parts, fmt.Sprintf("- **Blockers:** %s", e.Blockers))
	}
	if e.NextSteps != "" {
		parts = append(parts, fmt.Sprintf("- **Next:** %s", e.NextSteps))
	}
	for _, d := range e.KeyDecisions {
		parts = append(parts, fmt.Sprintf("- **Decision:** %s", d))
	}
	parts = append(parts, "")
	return "\n" + joinLines(parts) + "\n"
}

func joinLines(lines []string) string {
	var out string
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
