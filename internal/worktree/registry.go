package worktree

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gitmodel "focus/internal/git"
)

// Record stores one worktree's persistent identity and lightweight metadata.
type Record struct {
	ID           string
	RepoID       string
	RepoRoot     string
	TaskName     string
	Worktree     gitmodel.Worktree
	LastActiveAt time.Time
}

// RepoRegistry groups worktree records for one repository root.
type RepoRegistry struct {
	RepoID         string
	RepoRoot       string
	MainWorktreeID string
	Worktrees      map[string]Record
}

// Registry tracks repositories and their discovered worktrees.
type Registry struct {
	repos map[string]*RepoRegistry
}

func NewRegistry() *Registry {
	return &Registry{repos: make(map[string]*RepoRegistry)}
}

func (r *Registry) Repo(repoRoot string) (*RepoRegistry, bool) {
	repo, ok := r.repos[filepath.Clean(repoRoot)]
	return repo, ok
}

func (r *Registry) RefreshRepo(repoRoot string, worktrees []gitmodel.Worktree) (*RepoRegistry, error) {
	repoRoot = filepath.Clean(repoRoot)
	if repoRoot == "" || repoRoot == "." {
		return nil, fmt.Errorf("repo root cannot be empty")
	}

	repoID := repoIdentity(repoRoot)
	repo := &RepoRegistry{
		RepoID:    repoID,
		RepoRoot:  repoRoot,
		Worktrees: make(map[string]Record, len(worktrees)),
	}

	branchOwners := make(map[string]string)
	now := time.Now()
	for _, wt := range worktrees {
		id := worktreeIdentity(repoID, wt.Path)
		record := Record{
			ID:           id,
			RepoID:       repoID,
			RepoRoot:     repoRoot,
			TaskName:     wt.DisplayName(),
			Worktree:     wt,
			LastActiveAt: now,
		}
		repo.Worktrees[id] = record
		if wt.IsMain {
			repo.MainWorktreeID = id
		}

		if wt.Branch != "" && !wt.IsDetached && !wt.IsPrunable {
			if owner, exists := branchOwners[wt.Branch]; exists {
				return nil, fmt.Errorf("branch %q is active in multiple worktrees: %s and %s", wt.Branch, owner, wt.Path)
			}
			branchOwners[wt.Branch] = wt.Path
		}
	}

	r.repos[repoRoot] = repo
	return repo, nil
}

func (r *Registry) Repos() []*RepoRegistry {
	items := make([]*RepoRegistry, 0, len(r.repos))
	for _, repo := range r.repos {
		items = append(items, repo)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].RepoRoot < items[j].RepoRoot
	})
	return items
}

func repoIdentity(repoRoot string) string {
	return strings.ReplaceAll(filepath.Clean(repoRoot), string(filepath.Separator), "__")
}

func worktreeIdentity(repoID, path string) string {
	cleanPath := strings.ReplaceAll(filepath.Clean(path), string(filepath.Separator), "__")
	return repoID + "::" + cleanPath
}
