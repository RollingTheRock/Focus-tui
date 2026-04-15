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
	Activity       WorktreeActivity
}

type WorktreeActivity struct {
	OpenEditors int
	HasShell    bool
	LastActive  string
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
