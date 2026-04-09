// Package git defines core git domain types used by adapters and panes.
package git

// FileStatus represents the git status of a file.
type FileStatus byte

const (
	Unmodified FileStatus = ' '
	Modified   FileStatus = 'M'
	Added      FileStatus = 'A'
	Deleted    FileStatus = 'D'
	Renamed    FileStatus = 'R'
	Copied     FileStatus = 'C'
	Updated    FileStatus = 'U' // Updated but unmerged
	Untracked  FileStatus = '?'
	Ignored    FileStatus = '!'
)

// File represents a file in the working tree with its git status.
type File struct {
	Path           string
	Status         FileStatus
	StagedStatus   FileStatus
	WorktreeStatus FileStatus
	OriginalPath   string // For renamed files
}

// Status represents the overall git working tree status.
type Status struct {
	Branch          string
	Upstream        string
	Ahead           int
	Behind          int
	StagedFiles     []File
	UnstagedFiles   []File
	UntrackedFiles  []File
	ConflictedFiles []File
	IsMerge         bool
	IsRebase        bool
}

// Branch represents a git branch.
type Branch struct {
	Name     string
	Current  bool
	Upstream string
	Ahead    int
	Behind   int
	IsRemote bool
}

// ShortBranch returns a short branch name for display.
func (b *Branch) ShortName() string {
	if len(b.Name) > 20 {
		return b.Name[:17] + "..."
	}
	return b.Name
}

// HasChanges returns true if there are any changes in the working tree.
func (s *Status) HasChanges() bool {
	return len(s.StagedFiles) > 0 ||
		len(s.UnstagedFiles) > 0 ||
		len(s.UntrackedFiles) > 0 ||
		len(s.ConflictedFiles) > 0
}

// StagedCount returns the number of staged files.
func (s *Status) StagedCount() int {
	return len(s.StagedFiles)
}

// UnstagedCount returns the number of unstaged files.
func (s *Status) UnstagedCount() int {
	return len(s.UnstagedFiles)
}

// UntrackedCount returns the number of untracked files.
func (s *Status) UntrackedCount() int {
	return len(s.UntrackedFiles)
}
