package adapters

import "focus/internal/git"

type GitAdapter interface {
	Adapter
	GetStatus(repoPath string) (*git.Status, error)
	GetWorktreeStatus(worktreePath string) (*git.Status, error)
	GetBranches(repoPath string) ([]git.Branch, error)
	ListWorktrees(repoPath string) ([]git.Worktree, error)
	CreateWorktree(repoPath string, req git.CreateWorktreeRequest) (*git.Worktree, error)
	RemoveWorktree(repoPath, worktreePath string, opts git.RemoveWorktreeOptions) error
	PruneWorktrees(repoPath string) error
	GetDiff(repoPath string, path string, staged bool) (string, error)
	Commit(repoPath, message string) error
	Fetch(repoPath string) error
	Pull(repoPath string) error
	Push(repoPath string) error
	WatchStatus(repoPath string) (<-chan StatusEvent, error)
	StopWatch(repoPath string)

	StageFile(repoPath string, path string) error
	StageAll(repoPath string) error
	UnstageFile(repoPath string, path string) error
	UnstageAll(repoPath string) error
	DiscardChanges(repoPath string, path string) error
}

type StatusEvent struct {
	RepoPath string
	Status   *git.Status
	Error    error
}
