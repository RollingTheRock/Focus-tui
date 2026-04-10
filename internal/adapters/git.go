package adapters

import "focus/internal/git"

type GitAdapter interface {
	Adapter
	GetStatus(repoPath string) (*git.Status, error)
	GetBranches(repoPath string) ([]git.Branch, error)
	GetDiff(repoPath string, path string, staged bool) (string, error)
	Commit(repoPath, message string) error
	WatchStatus(repoPath string) (<-chan StatusEvent, error)

	StageFile(repoPath string, path string) error
	UnstageFile(repoPath string, path string) error
	DiscardChanges(repoPath string, path string) error
}

type StatusEvent struct {
	RepoPath string
	Status   *git.Status
	Error    error
}
