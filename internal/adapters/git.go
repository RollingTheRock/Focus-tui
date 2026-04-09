package adapters

import "focus/internal/git"

type GitAdapter interface {
	Adapter
	GetStatus(repoPath string) (*git.Status, error)
	GetBranches(repoPath string) ([]git.Branch, error)
	GetDiff(repoPath string, path string, staged bool) (string, error)
	WatchStatus(repoPath string) (<-chan StatusEvent, error)
}

type StatusEvent struct {
	RepoPath string
	Status   *git.Status
	Error    error
}
