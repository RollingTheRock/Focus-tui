package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveProjectRootFallsBackToCWD(t *testing.T) {
	cwd := t.TempDir()
	root, err := ResolveProjectRoot(cwd)
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	if root != cwd {
		t.Fatalf("expected cwd root %q, got %q", cwd, root)
	}
}

func TestResolveProjectRootUsesGitTopLevel(t *testing.T) {
	repo := t.TempDir()
	if err := exec.Command("git", "init", repo).Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	sub := filepath.Join(repo, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	root, err := ResolveProjectRoot(sub)
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	if root != repo {
		t.Fatalf("expected git root %q, got %q", repo, root)
	}
}

func TestProjectDBPath(t *testing.T) {
	root := "/tmp/repo"
	path, err := ProjectDBPath(root)
	if err != nil {
		t.Fatalf("project db path: %v", err)
	}
	want := filepath.Join(root, ".focus", "focus.db")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

