package worktree

import (
	"path/filepath"
	"testing"

	gitmodel "focus/internal/git"
)

func TestRefreshRepoStoresMainWorktree(t *testing.T) {
	registry := NewRegistry()
	repoRoot := filepath.Join(string(filepath.Separator), "repo", "main")
	worktrees := []gitmodel.Worktree{
		{Path: repoRoot, Branch: "main", IsMain: true},
		{Path: filepath.Join(string(filepath.Separator), "repo", "feature-a"), Branch: "feature-a"},
	}

	repo, err := registry.RefreshRepo(repoRoot, worktrees)
	if err != nil {
		t.Fatalf("refresh repo: %v", err)
	}
	if repo.MainWorktreeID == "" {
		t.Fatalf("expected main worktree id to be set")
	}
	if len(repo.Worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(repo.Worktrees))
	}
}

func TestRefreshRepoRejectsDuplicateActiveBranch(t *testing.T) {
	registry := NewRegistry()
	repoRoot := filepath.Join(string(filepath.Separator), "repo", "main")
	worktrees := []gitmodel.Worktree{
		{Path: repoRoot, Branch: "main", IsMain: true},
		{Path: filepath.Join(string(filepath.Separator), "repo", "feature-a"), Branch: "feature-a"},
		{Path: filepath.Join(string(filepath.Separator), "repo", "feature-a-copy"), Branch: "feature-a"},
	}

	if _, err := registry.RefreshRepo(repoRoot, worktrees); err == nil {
		t.Fatalf("expected duplicate branch mapping error")
	}
}
