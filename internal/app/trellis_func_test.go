//go:build !short

package app

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"focus/internal/config"
	gitmodel "focus/internal/git"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/store"
)

// TestTrellisWorktreeCreateEndToEnd runs the real focus program against a
// temporary git repository, sends it the WorktreeCreatedMsg that the TUI would
// produce after creating a new worktree with a task name, and verifies that
// .trellis is correctly linked in the new worktree.
func TestTrellisWorktreeCreateEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("trellis"); err != nil {
		t.Skip("trellis CLI not installed")
	}

	// Isolate HOME so ensureKimiAdapter doesn't touch the user's ~/.kimi.
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	// Create a temp git repo and an initial commit.
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir("/")
	}()

	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Tester")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")

	// Create focus store for this repo.
	projectRoot, err := store.ResolveProjectRoot(repo)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}
	dbPath, err := store.ProjectDBPath(projectRoot)
	if err != nil {
		t.Fatalf("ProjectDBPath: %v", err)
	}
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer st.Close()

	cfg := config.Load()
	cfg.Agent.MCPSocket = filepath.Join(projectRoot, ".focus", "mcp.sock")
	cfg.Agent.MCPPort = "127.0.0.1:0"

	m := New(cfg, st)

	// Run the real Bubble Tea program with discarded output and no input, then
	// inject the WorktreeCreatedMsg that the create-worktree pane would send.
	p := tea.NewProgram(m,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignals(),
	)

	wtPath := filepath.Join(repo, ".worktrees", "func-test")
	runGit(t, repo, "worktree", "add", wtPath)

	go func() {
		// Give the program a moment to start and initialize the trellis bridge.
		time.Sleep(2 * time.Second)
		p.Send(gitplugin.WorktreeCreatedMsg{
			TaskTitle: "func test task",
			Worktree:  gitmodel.Worktree{Path: wtPath},
		})
		// Give the async trellis setup time to run.
		time.Sleep(4 * time.Second)
		p.Send(tea.Quit())
	}()

	if _, err := p.Run(); err != nil {
		t.Fatalf("focus program: %v", err)
	}

	// Verify the worktree got a task context record linked to it.
	ctx, err := st.GetWorktreeContext(wtPath)
	if err != nil {
		t.Fatalf("GetWorktreeContext: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected worktree context to be created")
	}
	if ctx.TaskName != "func test task" {
		t.Errorf("expected TaskName %q, got %q", "func test task", ctx.TaskName)
	}

	// Verify the new worktree has a valid .trellis symlink pointing to the
	// main repo's .trellis.
	trellisLink := filepath.Join(wtPath, ".trellis")
	target, err := os.Readlink(trellisLink)
	if err != nil {
		t.Fatalf("expected .trellis symlink in new worktree: %v", err)
	}
	expectedTarget := filepath.Join(repo, ".trellis")
	if target != expectedTarget {
		t.Errorf("expected .trellis -> %s, got %s", expectedTarget, target)
	}
	if _, err := os.Stat(trellisLink); err != nil {
		t.Errorf(".trellis symlink points to missing target: %v", err)
	}
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
