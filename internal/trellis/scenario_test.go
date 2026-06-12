package trellis

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupRepo(t *testing.T) string {
	t.Helper()

	// Isolate ~/.kimi so tests don't mutate the user's real Kimi config.
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Tester")
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// installFakeTrellis puts a non-interactive stub trellis on PATH so the tests
// focus on Focus's own logic rather than real Trellis's network/template prompts.
func installFakeTrellis(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(bin, "trellis")
	content := `#!/usr/bin/env bash
set -e
cmd="${1:-}"
shift || true
repo="$PWD"
force=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    -u|--user) shift 2 ;;
    --force|-f) force=true; shift ;;
    -y|--yes) shift ;;
    --claude|--cursor|--opencode|--codex|--gemini|--kimi|--kilo|--kiro|--antigravity|--windsurf|--qoder|--codebuddy|--copilot|--droid|--pi) shift ;;
    -t|--template|--registry) shift 2 ;;
    --monorepo|--no-monorepo) shift ;;
    *) shift ;;
  esac
done

case "$cmd" in
  init)
    if [[ ( -e "$repo/.trellis" || -L "$repo/.trellis" ) && "$force" == "false" ]]; then
      echo "fake-trellis: .trellis already exists (including broken symlink); use --force" >&2
      exit 1
    fi
    rm -rf "$repo/.trellis"
    mkdir -p "$repo/.trellis/scripts" "$repo/.trellis/spec" "$repo/.trellis/tasks"
    echo "version: 1" > "$repo/.trellis/config.yaml"
    cat > "$repo/.trellis/scripts/task.py" <<'PY'
#!/usr/bin/env python3
import sys, os, json, argparse
def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest='sub')
    create = sub.add_parser('create')
    create.add_argument('title')
    create.add_argument('--slug')
    create.add_argument('--assignee')
    create.add_argument('--priority')
    create.add_argument('--description')
    args = parser.parse_args()
    if args.sub == 'create':
        slug = args.slug or args.title.replace(' ', '-').lower()
        d = os.path.join('.trellis', 'tasks', slug)
        os.makedirs(d, exist_ok=True)
        meta = {'id': slug, 'title': args.title, 'status': 'in_progress'}
        with open(os.path.join(d, 'task.json'), 'w') as f:
            json.dump(meta, f)
        print(d)
if __name__ == '__main__':
    main()
PY
    chmod +x "$repo/.trellis/scripts/task.py"
    ;;
  update|uninstall)
    ;;
  *)
    echo "fake-trellis: unknown command $cmd" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+path)
	return bin
}

func assertValidSymlink(t *testing.T, linkPath string) {
	t.Helper()
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Errorf("expected symlink at %s: %v", linkPath, err)
		return
	}
	if _, err := os.Stat(linkPath); err != nil {
		t.Errorf("symlink %s -> %s points to missing target: %v", linkPath, target, err)
	}
}

// TestScenarioA_BrokenRepoRootTrellis: repo root .trellis is a dangling symlink.
// Expected behavior: Focus should detect the broken symlink, re-initialize trellis,
// and successfully link the new worktree.
func TestScenarioA_BrokenRepoRootTrellis(t *testing.T) {
	repo := setupRepo(t)
	installFakeTrellis(t)

	// Create a dangling .trellis symlink like the one observed in the real repo.
	missing := filepath.Join(repo, "missing")
	link := filepath.Join(repo, ".trellis")
	if err := os.Symlink(missing, link); err != nil {
		t.Fatal(err)
	}

	wt := filepath.Join(repo, ".worktrees", "new")
	runGit(t, repo, "worktree", "add", wt)

	bridge := NewBridge(repo, "", nil)

	initErr := bridge.EnsureInitialized()
	linkErr := bridge.EnsureWorktreeLinks(wt)

	t.Logf("EnsureInitialized err=%v", initErr)
	t.Logf("EnsureWorktreeLinks err=%v", linkErr)

	// After the fix, EnsureInitialized should detect the broken symlink, remove
	// it, run trellis init, and then EnsureWorktreeLinks should succeed.
	if initErr != nil || linkErr != nil {
		t.Errorf("expected broken-symlink recovery, got initErr=%v linkErr=%v", initErr, linkErr)
	}
	assertValidSymlink(t, filepath.Join(wt, ".trellis"))
}

// TestScenarioB_NeverInitialized: fresh repo, no .trellis, no .focus.
// Expected behavior: EnsureInitialized runs trellis init, then EnsureWorktreeLinks
// creates the symlink in the new worktree.
func TestScenarioB_NeverInitialized(t *testing.T) {
	repo := setupRepo(t)
	installFakeTrellis(t)

	wt := filepath.Join(repo, ".worktrees", "new")
	runGit(t, repo, "worktree", "add", wt)

	bridge := NewBridge(repo, "", nil)

	initErr := bridge.EnsureInitialized()
	linkErr := bridge.EnsureWorktreeLinks(wt)

	t.Logf("EnsureInitialized err=%v", initErr)
	t.Logf("EnsureWorktreeLinks err=%v", linkErr)

	if initErr != nil || linkErr != nil {
		t.Errorf("expected success for fresh repo, got initErr=%v linkErr=%v", initErr, linkErr)
	}
	assertValidSymlink(t, filepath.Join(wt, ".trellis"))
}

// TestScenarioC_FocusInitializedNoTrellis: .focus exists but .trellis does not.
// Expected behavior is the same as scenario B: trellis should be initialized and
// the worktree should get a valid .trellis symlink.
func TestScenarioC_FocusInitializedNoTrellis(t *testing.T) {
	repo := setupRepo(t)
	installFakeTrellis(t)

	// Simulate "focus has been initialized": create a .focus directory.
	if err := os.MkdirAll(filepath.Join(repo, ".focus"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, ".focus", "config.toml"), "# placeholder\n")

	wt := filepath.Join(repo, ".worktrees", "new")
	runGit(t, repo, "worktree", "add", wt)

	bridge := NewBridge(repo, "", nil)

	initErr := bridge.EnsureInitialized()
	linkErr := bridge.EnsureWorktreeLinks(wt)

	t.Logf("EnsureInitialized err=%v", initErr)
	t.Logf("EnsureWorktreeLinks err=%v", linkErr)

	if initErr != nil || linkErr != nil {
		t.Errorf("expected success when only .focus exists, got initErr=%v linkErr=%v", initErr, linkErr)
	}
	assertValidSymlink(t, filepath.Join(wt, ".trellis"))
}

// TestCleanupWorktreeLinks verifies that when a worktree is deleted, symlinks
// in the main repo and in remaining worktrees that point into the deleted
// worktree are removed.
func TestCleanupWorktreeLinks(t *testing.T) {
	repo := setupRepo(t)

	a := filepath.Join(repo, ".worktrees", "a")
	b := filepath.Join(repo, ".worktrees", "b")
	runGit(t, repo, "worktree", "add", a)
	runGit(t, repo, "worktree", "add", b)

	// Simulate trellis having been initialized in worktree A and linked from
	// the main repo and worktree B (the old fragile behavior).
	if err := os.MkdirAll(filepath.Join(a, ".trellis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(a, ".kimi", "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(a, ".trellis", "config.yaml"), "version: 1\n")
	writeFile(t, filepath.Join(a, ".kimi", "hooks", "session-start.py"), "# noop\n")

	mustSymlink := func(link, target string) {
		t.Helper()
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink %s -> %s: %v", link, target, err)
		}
	}
	mustSymlink(filepath.Join(repo, ".trellis"), filepath.Join(a, ".trellis"))
	mustSymlink(filepath.Join(repo, ".kimi"), filepath.Join(a, ".kimi"))
	mustSymlink(filepath.Join(b, ".trellis"), filepath.Join(a, ".trellis"))
	mustSymlink(filepath.Join(b, ".kimi"), filepath.Join(a, ".kimi"))

	// Delete worktree A, leaving the symlinks dangling.
	if err := os.RemoveAll(a); err != nil {
		t.Fatal(err)
	}

	bridge := NewBridge(repo, "", nil)
	if err := bridge.CleanupWorktreeLinks(a); err != nil {
		t.Fatalf("CleanupWorktreeLinks: %v", err)
	}

	for _, dir := range []string{repo, b} {
		for _, name := range []string{".trellis", ".kimi"} {
			link := filepath.Join(dir, name)
			if _, err := os.Lstat(link); err == nil {
				t.Errorf("expected %s to be removed, but it still exists", link)
			}
		}
	}
}
