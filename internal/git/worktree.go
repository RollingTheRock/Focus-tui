package git

import "path/filepath"

// Worktree describes one git worktree attached to a repository.
type Worktree struct {
	Path           string
	Branch         string
	HeadOID        string
	IsMain         bool
	IsDetached     bool
	IsLocked       bool
	LockReason     string
	IsPrunable     bool
	PrunableReason string
	IsBare         bool
	DirtySummary   DirtySummary
	AheadBehind    AheadBehind
	Upstream       string
	Activity       WorktreeActivity
}

type WorktreeActivity struct {
	OpenEditors int
	HasShell    bool
	AgentCount  int
	LastActive  string
}

type WorktreeResumeSummary struct {
	TaskID            string
	TaskTitle         string
	TaskGoal          string
	TaskWhyNow        string
	TaskSuccess       string
	TaskOutOfScope    string
	TaskKnownRisks    string
	NextStep          string
	TaskState         string
	TaskPriority      string
	TaskMode          string
	QueuedTaskTitle   string
	QueuedTaskCount   int
	AttentionAnchor   string
	RecentArtifact    string
	PinnedNote        string
	BlockerNote       string
	HandoffNote       string
	GitPressure       string
	PlanTitle         string
	PlanStatus        string
	CurrentPlanStep   string
	PlanBody          string
	PlanSteps         []string
	HandoffEntrypoint string
	LastResumeHint    string
	LastActiveLabel   string
	LastAgentLabel    string
	LastAgentSummary  string
	ResumeReason      string
	ResumeScore       int
}

// DisplayName returns a short label suitable for UI display.
func (w Worktree) DisplayName() string {
	base := filepath.Base(w.Path)
	if w.Branch != "" {
		return w.Branch
	}
	if base != "." && base != string(filepath.Separator) {
		return base
	}
	return w.Path
}

// DirtySummary captures a compact summary of working state.
type DirtySummary struct {
	Staged     int
	Unstaged   int
	Untracked  int
	Conflicted int
}

// IsDirty reports whether the worktree has any tracked or untracked changes.
func (d DirtySummary) IsDirty() bool {
	return d.Staged > 0 || d.Unstaged > 0 || d.Untracked > 0 || d.Conflicted > 0
}

// AheadBehind captures upstream divergence data.
type AheadBehind struct {
	Ahead  int
	Behind int
}

// CreateWorktreeRequest describes how to create a worktree.
type CreateWorktreeRequest struct {
	Path    string
	Branch  string
	BaseRef string
	Detach  bool
	Force   bool
}

// RemoveWorktreeOptions controls worktree removal behavior.
type RemoveWorktreeOptions struct {
	Force bool
}
