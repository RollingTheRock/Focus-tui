package app

import (
	"strings"
	"testing"
)

func TestMainRepoRootFromWorktree(t *testing.T) {
	wtRoot, ok := gitRepoRoot(".")
	if !ok {
		t.Fatal("gitRepoRoot failed")
	}
	mainRoot, ok := mainRepoRoot(".")
	if !ok {
		t.Fatal("mainRepoRoot failed")
	}

	if wtRoot == mainRoot {
		t.Logf("running from main repo root: %s", mainRoot)
		return
	}

	if !strings.HasPrefix(wtRoot, mainRoot) || !strings.Contains(wtRoot, "/.worktrees/") {
		t.Errorf("expected worktree root under main repo root, got wt=%q main=%q", wtRoot, mainRoot)
	}
}
