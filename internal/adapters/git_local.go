// Package adapters provides adapter implementations for external tools.
package adapters

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"focus/internal/git"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

const defaultCacheTTL = time.Second

type cachedStatus struct {
	status    *git.Status
	timestamp time.Time
}

type cachedWorktrees struct {
	worktrees []git.Worktree
	timestamp time.Time
}

type watchEntry struct {
	ch     chan StatusEvent
	cancel context.CancelFunc
	refs   int
}

// GitLocalAdapter implements GitAdapter using local git CLI.
type GitLocalAdapter struct {
	name          string
	watches       map[string]*watchEntry
	watchMu       sync.Mutex
	cacheTTL      time.Duration
	statusCache   map[string]cachedStatus
	worktreeCache map[string]cachedWorktrees
	cacheMu       sync.RWMutex
}

// NewGitLocalAdapter creates a new GitLocalAdapter.
func NewGitLocalAdapter() *GitLocalAdapter {
	return &GitLocalAdapter{
		name:          "git-local",
		watches:       make(map[string]*watchEntry),
		cacheTTL:      defaultCacheTTL,
		statusCache:   make(map[string]cachedStatus),
		worktreeCache: make(map[string]cachedWorktrees),
	}
}

// Name returns the adapter name.
func (g *GitLocalAdapter) Name() string {
	return g.name
}

// Init initializes the adapter.
func (g *GitLocalAdapter) Init() error {
	// Verify git is available
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	return nil
}

// Destroy cleans up the adapter.
func (g *GitLocalAdapter) Destroy() error {
	g.watchMu.Lock()
	for path, entry := range g.watches {
		if entry.cancel != nil {
			entry.cancel()
		}
		close(entry.ch)
		delete(g.watches, path)
	}
	g.watchMu.Unlock()
	return nil
}

func (g *GitLocalAdapter) cachedStatus(repoPath string) (*git.Status, bool) {
	g.cacheMu.RLock()
	c, ok := g.statusCache[repoPath]
	g.cacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(c.timestamp) > g.cacheTTL {
		return nil, false
	}
	return c.status, true
}

func (g *GitLocalAdapter) setCachedStatus(repoPath string, status *git.Status) {
	g.cacheMu.Lock()
	g.statusCache[repoPath] = cachedStatus{status: status, timestamp: time.Now()}
	g.cacheMu.Unlock()
}

func (g *GitLocalAdapter) invalidateStatusCache(repoPath string) {
	g.cacheMu.Lock()
	delete(g.statusCache, repoPath)
	g.cacheMu.Unlock()
}

func (g *GitLocalAdapter) invalidateWorktreeCache(repoPath string) {
	g.cacheMu.Lock()
	delete(g.worktreeCache, repoPath)
	g.cacheMu.Unlock()
}

// GetStatus retrieves the git status for a repository.
// Uses porcelain=v2 format for reliable parsing.
func (g *GitLocalAdapter) GetStatus(repoPath string) (*git.Status, error) {
	if cached, ok := g.cachedStatus(repoPath); ok {
		return cached, nil
	}

	// Get branch info
	branch, upstream, ahead, behind, err := g.getBranchInfo(repoPath)
	if err != nil {
		return nil, err
	}

	status := &git.Status{
		Branch:   branch,
		Upstream: upstream,
		Ahead:    ahead,
		Behind:   behind,
	}

	// Get file status using porcelain=v2
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain=v2", "-z")
	output, err := cmd.Output()
	if err != nil {
		// Check if this is a git repository
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return nil, fmt.Errorf("not a git repository: %s", repoPath)
		}
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	if err := g.parsePorcelainV2(output, status); err != nil {
		return nil, fmt.Errorf("failed to parse status: %w", err)
	}

	g.setCachedStatus(repoPath, status)
	return status, nil
}

// GetWorktreeStatus retrieves status for a specific worktree path.
func (g *GitLocalAdapter) GetWorktreeStatus(worktreePath string) (*git.Status, error) {
	return g.GetStatus(worktreePath)
}

// getBranchInfo retrieves branch and upstream information.
func (g *GitLocalAdapter) getBranchInfo(repoPath string) (branch, upstream string, ahead, behind int, err error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	branchBytes, err := cmd.Output()
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("failed to get branch: %w", err)
	}
	branch = strings.TrimSpace(string(branchBytes))

	// Check for upstream
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "@{upstream}")
	upstreamBytes, err := cmd.Output()
	if err == nil {
		upstream = strings.TrimSpace(string(upstreamBytes))

		// Get ahead/behind count
		cmd = exec.Command("git", "-C", repoPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
		countBytes, err := cmd.Output()
		if err == nil {
			parts := strings.Fields(string(countBytes))
			if len(parts) == 2 {
				ahead, _ = strconv.Atoi(parts[0])
				behind, _ = strconv.Atoi(parts[1])
			}
		}
	}

	return branch, upstream, ahead, behind, nil
}

// parsePorcelainV2 parses git status --porcelain=v2 -z output.
// Reference: /mnt/d/dev/dev-learn/sidecar/internal/plugins/gitstatus/tree.go
func (g *GitLocalAdapter) parsePorcelainV2(output []byte, status *git.Status) error {
	if len(output) == 0 {
		return nil
	}

	// -z flag uses NUL as delimiter instead of newline
	entries := bytes.Split(output, []byte{0})

	for i := 0; i < len(entries); i++ {
		entry := string(entries[i])
		if entry == "" {
			continue
		}

		parts := strings.Fields(entry)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "1": // Ordinary changed entries
			if len(parts) >= 9 {
				file := git.File{
					Path:           parts[8],
					StagedStatus:   git.FileStatus(parts[1][0]),
					WorktreeStatus: git.FileStatus(parts[1][1]),
				}
				g.categorizeFile(file, status)
			}
		case "2": // Renamed or copied entries
			if len(parts) >= 10 {
				file := git.File{
					Path:           parts[9],
					OriginalPath:   parts[8],
					StagedStatus:   git.FileStatus(parts[1][0]),
					WorktreeStatus: git.FileStatus(parts[1][1]),
				}
				g.categorizeFile(file, status)
			}
		case "u": // Unmerged entries
			if len(parts) >= 11 {
				file := git.File{
					Path:           parts[10],
					StagedStatus:   git.FileStatus(parts[1][0]),
					WorktreeStatus: git.FileStatus(parts[1][1]),
				}
				status.ConflictedFiles = append(status.ConflictedFiles, file)
			}
		case "?": // Untracked files
			if len(parts) >= 2 {
				file := git.File{
					Path:   parts[1],
					Status: git.Untracked,
				}
				status.UntrackedFiles = append(status.UntrackedFiles, file)
			}
		case "!": // Ignored files
			// Skip ignored files for now
		}
	}

	return nil
}

// categorizeFile adds a file to the appropriate status list.
func (g *GitLocalAdapter) categorizeFile(file git.File, status *git.Status) {
	hasStaged := !isPorcelainUnmodified(file.StagedStatus)
	hasUnstaged := !isPorcelainUnmodified(file.WorktreeStatus)

	if hasStaged {
		status.StagedFiles = append(status.StagedFiles, file)
	}
	if hasUnstaged {
		status.UnstagedFiles = append(status.UnstagedFiles, file)
	}
}

func isPorcelainUnmodified(status git.FileStatus) bool {
	return status == git.Unmodified || status == ' ' || status == '.'
}

// GetBranches retrieves all branches for a repository.
func (g *GitLocalAdapter) GetBranches(repoPath string) ([]git.Branch, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "-vv", "--format=%(refname:short) %(upstream:short) %(upstream:track)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	var branches []git.Branch
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}

		branch := git.Branch{
			Name: parts[0],
		}

		// Check if current branch (starts with *)
		if strings.HasPrefix(parts[0], "*") {
			branch.Name = strings.TrimPrefix(parts[0], "*")
			branch.Current = true
		}

		// Parse upstream and tracking info
		if len(parts) >= 2 {
			branch.Upstream = parts[1]
		}
		if len(parts) >= 3 {
			// Parse [ahead N, behind M]
			tracking := parts[2]
			if strings.Contains(tracking, "ahead") {
				fmt.Sscanf(tracking, "[ahead %d]", &branch.Ahead)
			}
			if strings.Contains(tracking, "behind") {
				fmt.Sscanf(tracking, "[behind %d]", &branch.Behind)
			}
		}

		branches = append(branches, branch)
	}

	return branches, scanner.Err()
}

// ListWorktrees retrieves all worktrees for a repository.
func (g *GitLocalAdapter) cachedWorktrees(repoPath string) ([]git.Worktree, bool) {
	g.cacheMu.RLock()
	c, ok := g.worktreeCache[repoPath]
	g.cacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(c.timestamp) > g.cacheTTL {
		return nil, false
	}
	return c.worktrees, true
}

func (g *GitLocalAdapter) setCachedWorktrees(repoPath string, worktrees []git.Worktree) {
	g.cacheMu.Lock()
	g.worktreeCache[repoPath] = cachedWorktrees{worktrees: worktrees, timestamp: time.Now()}
	g.cacheMu.Unlock()
}

func (g *GitLocalAdapter) ListWorktrees(repoPath string) ([]git.Worktree, error) {
	if cached, ok := g.cachedWorktrees(repoPath); ok {
		return cached, nil
	}

	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list failed: %w", err)
	}

	mainRoot, err := g.mainWorktreeRoot(repoPath)
	if err != nil {
		return nil, err
	}

	worktrees, err := g.parseWorktreeListPorcelain(mainRoot, output)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	statuses := make([]*git.Status, len(worktrees))
	for i := range worktrees {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			status, err := g.GetWorktreeStatus(worktrees[idx].Path)
			if err != nil {
				return
			}
			statuses[idx] = status
		}(i)
	}
	wg.Wait()

	for i := range worktrees {
		status := statuses[i]
		if status == nil {
			continue
		}
		worktrees[i].DirtySummary = git.DirtySummary{
			Staged:     len(status.StagedFiles),
			Unstaged:   len(status.UnstagedFiles),
			Untracked:  len(status.UntrackedFiles),
			Conflicted: len(status.ConflictedFiles),
		}
		worktrees[i].AheadBehind = git.AheadBehind{
			Ahead:  status.Ahead,
			Behind: status.Behind,
		}
		worktrees[i].Upstream = status.Upstream
		if status.Branch != "" && worktrees[i].Branch == "" {
			worktrees[i].Branch = status.Branch
		}
	}

	g.setCachedWorktrees(repoPath, worktrees)
	return worktrees, nil
}

// ensureWorktreesGitignored checks whether .worktrees/ is ignored by git.
// If not, it appends the entry to .gitignore (creating the file if needed).
func ensureWorktreesGitignored(repoPath string) error {
	// First ask git whether .worktrees is already ignored.
	checkCmd := exec.Command("git", "-C", repoPath, "check-ignore", "-q", ".worktrees")
	if err := checkCmd.Run(); err == nil {
		return nil
	}

	gitignorePath := filepath.Join(repoPath, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == ".worktrees/" || line == ".worktrees" {
			return nil
		}
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	prefix := ""
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		prefix = "\n"
	}
	if _, err := f.WriteString(prefix + ".worktrees/\n"); err != nil {
		return err
	}
	return nil
}

// CreateWorktree creates a new worktree and returns the resulting record.
func (g *GitLocalAdapter) CreateWorktree(repoPath string, req git.CreateWorktreeRequest) (*git.Worktree, error) {
	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		return nil, fmt.Errorf("worktree path cannot be empty")
	}

	// If the worktree lives under .worktrees/, ensure it is gitignored.
	if strings.Contains(req.Path, ".worktrees") {
		_ = ensureWorktreesGitignored(repoPath)
	}

	args := []string{"-C", repoPath, "worktree", "add"}
	if req.Force {
		args = append(args, "--force")
	}
	if req.Detach {
		args = append(args, "--detach")
	}
	if req.Branch != "" && req.BaseRef != "" && !req.Detach {
		args = append(args, "-b", req.Branch)
	}
	args = append(args, req.Path)
	if req.Detach {
		if req.BaseRef != "" {
			args = append(args, req.BaseRef)
		}
	} else if req.Branch != "" && req.BaseRef == "" {
		args = append(args, req.Branch)
	} else if req.BaseRef != "" {
		args = append(args, req.BaseRef)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return nil, fmt.Errorf("git worktree add failed: %w", err)
		}
		return nil, fmt.Errorf("git worktree add failed: %s", detail)
	}

	g.invalidateWorktreeCache(repoPath)

	worktrees, err := g.ListWorktrees(repoPath)
	if err != nil {
		return nil, err
	}
	createdPath, err := filepath.Abs(req.Path)
	if err != nil {
		createdPath = req.Path
	}
	for i := range worktrees {
		if samePath(worktrees[i].Path, createdPath) {
			return &worktrees[i], nil
		}
	}

	return nil, fmt.Errorf("created worktree %q not found after creation", createdPath)
}

// RemoveWorktree removes a worktree path from the repository.
func (g *GitLocalAdapter) RemoveWorktree(repoPath, worktreePath string, opts git.RemoveWorktreeOptions) error {
	args := []string{"-C", repoPath, "worktree", "remove"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("git worktree remove failed: %w", err)
		}
		return fmt.Errorf("git worktree remove failed: %s", detail)
	}

	g.invalidateWorktreeCache(repoPath)
	g.invalidateStatusCache(worktreePath)

	// Clean up residual filesystem entries that git worktree remove may leave
	// behind when the directory contains untracked files.
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		_ = os.RemoveAll(worktreePath)
		// If the parent directory (e.g. .worktrees/) is now empty, remove it too.
		parent := filepath.Dir(worktreePath)
		if entries, err := os.ReadDir(parent); err == nil && len(entries) == 0 {
			_ = os.Remove(parent)
		}
	}

	return nil
}

// PruneWorktrees prunes stale worktree metadata.
func (g *GitLocalAdapter) PruneWorktrees(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "prune")
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("git worktree prune failed: %w", err)
		}
		return fmt.Errorf("git worktree prune failed: %s", detail)
	}

	g.invalidateWorktreeCache(repoPath)
	return nil
}

// GetDiff retrieves the diff for a specific file.
func (g *GitLocalAdapter) GetDiff(repoPath string, path string, staged bool) (string, error) {
	args := []string{"-C", repoPath, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	if path != "" {
		args = append(args, "--", path)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	return string(output), nil
}

// Commit creates a git commit with the provided message.
func (g *GitLocalAdapter) Commit(repoPath, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	cmd := exec.Command("git", "-C", repoPath, "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("git commit failed: %w", err)
		}
		return fmt.Errorf("git commit failed: %s", detail)
	}

	return nil
}

// Fetch updates remote tracking refs without changing the working tree.
func (g *GitLocalAdapter) Fetch(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch")
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("git fetch failed: %w", err)
		}
		return fmt.Errorf("git fetch failed: %s", detail)
	}

	return nil
}

// Pull updates the current branch from its configured upstream.
func (g *GitLocalAdapter) Pull(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "pull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("git pull failed: %w", err)
		}
		return fmt.Errorf("git pull failed: %s", detail)
	}

	return nil
}

// Push sends the current branch to its configured upstream.
func (g *GitLocalAdapter) Push(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "push")
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("git push failed: %w", err)
		}
		return fmt.Errorf("git push failed: %s", detail)
	}

	return nil
}

// StageFile adds a file to the staging area.
func (g *GitLocalAdapter) StageFile(repoPath string, path string) error {
	cmd := exec.Command("git", "-C", repoPath, "add", "--", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	return nil
}

// StageAll adds all tracked and untracked changes to the staging area.
func (g *GitLocalAdapter) StageAll(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "add", "--all")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add --all failed: %w", err)
	}
	return nil
}

// UnstageFile removes a file from the staging area.
func (g *GitLocalAdapter) UnstageFile(repoPath string, path string) error {
	cmd := exec.Command("git", "-C", repoPath, "reset", "HEAD", "--", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}
	return nil
}

// UnstageAll removes all staged changes from the index.
func (g *GitLocalAdapter) UnstageAll(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "reset", "HEAD", "--", ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git reset HEAD -- . failed: %w", err)
	}
	return nil
}

// DiscardChanges discards tracked or untracked changes for a file path.
func (g *GitLocalAdapter) DiscardChanges(repoPath string, path string) error {
	statusLine, err := g.statusLine(repoPath, path)
	if err != nil {
		return err
	}
	if statusLine == "" {
		return fmt.Errorf("no changes to discard for %s", path)
	}

	if strings.HasPrefix(statusLine, "??") {
		cleanCmd := exec.Command("git", "-C", repoPath, "clean", "-f", "--", path)
		if err := cleanCmd.Run(); err != nil {
			return fmt.Errorf("git clean failed: %w", err)
		}
		return nil
	}

	if len(statusLine) >= 1 && statusLine[0] != ' ' {
		resetCmd := exec.Command("git", "-C", repoPath, "reset", "HEAD", "--", path)
		if err := resetCmd.Run(); err != nil {
			return fmt.Errorf("git reset failed: %w", err)
		}

		statusLine, err = g.statusLine(repoPath, path)
		if err != nil {
			return err
		}
		if statusLine == "" {
			return nil
		}
		if strings.HasPrefix(statusLine, "??") {
			cleanCmd := exec.Command("git", "-C", repoPath, "clean", "-f", "--", path)
			if err := cleanCmd.Run(); err != nil {
				return fmt.Errorf("git clean failed: %w", err)
			}
			return nil
		}
	}

	checkoutCmd := exec.Command("git", "-C", repoPath, "checkout", "--", path)
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	return nil
}

func (g *GitLocalAdapter) statusLine(repoPath string, path string) (string, error) {
	statusCmd := exec.Command("git", "-C", repoPath, "status", "--porcelain", "--", path)
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(string(statusOutput)), nil
}

// WatchStatus starts watching a repository for status changes.
// Returns a channel that receives status updates.
func (g *GitLocalAdapter) WatchStatus(repoPath string) (<-chan StatusEvent, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}

	g.watchMu.Lock()
	defer g.watchMu.Unlock()

	if entry, exists := g.watches[absPath]; exists {
		entry.refs++
		return entry.ch, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan StatusEvent, 1)
	entry := &watchEntry{ch: ch, cancel: cancel, refs: 1}
	g.watches[absPath] = entry

	go g.watchLoop(ctx, absPath, ch)

	return ch, nil
}

func (g *GitLocalAdapter) StopWatch(repoPath string) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return
	}

	g.watchMu.Lock()
	defer g.watchMu.Unlock()

	entry, exists := g.watches[absPath]
	if !exists {
		return
	}

	entry.refs--
	if entry.refs <= 0 {
		if entry.cancel != nil {
			entry.cancel()
		}
		close(entry.ch)
		delete(g.watches, absPath)
	}
}

func (g *GitLocalAdapter) gitDir(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}
	return filepath.Abs(gitDir)
}

func (g *GitLocalAdapter) watchLoop(ctx context.Context, repoPath string, ch chan StatusEvent) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		select {
		case ch <- StatusEvent{RepoPath: repoPath, Error: err}:
		default:
		}
		return
	}
	defer watcher.Close()

	gitDir, err := g.gitDir(repoPath)
	if err == nil {
		_ = watcher.Add(filepath.Join(gitDir, "index"))
		_ = watcher.Add(filepath.Join(gitDir, "HEAD"))
		_ = watcher.Add(filepath.Join(gitDir, "refs", "heads"))
		_ = watcher.Add(filepath.Join(gitDir, "refs", "remotes"))
	}

	fallbackTicker := time.NewTicker(10 * time.Second)
	defer fallbackTicker.Stop()

	var lastStatus *git.Status
	sendStatus := func() {
		status, err := g.GetStatus(repoPath)
		if err != nil {
			select {
			case ch <- StatusEvent{RepoPath: repoPath, Error: err}:
			default:
			}
			return
		}
		if !g.statusEqual(lastStatus, status) {
			select {
			case ch <- StatusEvent{RepoPath: repoPath, Status: status}:
			default:
			}
			lastStatus = status
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-watcher.Events:
			sendStatus()
		case err := <-watcher.Errors:
			_ = err
		case <-fallbackTicker.C:
			sendStatus()
		}
	}
}

// statusEqual compares two status objects for equality.
func (g *GitLocalAdapter) statusEqual(a, b *git.Status) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Branch == b.Branch &&
		a.Ahead == b.Ahead &&
		a.Behind == b.Behind &&
		len(a.StagedFiles) == len(b.StagedFiles) &&
		len(a.UnstagedFiles) == len(b.UnstagedFiles) &&
		len(a.UntrackedFiles) == len(b.UntrackedFiles)
}

func (g *GitLocalAdapter) parseWorktreeListPorcelain(mainRoot string, output []byte) ([]git.Worktree, error) {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	var worktrees []git.Worktree
	var current *git.Worktree

	flush := func() {
		if current == nil || current.Path == "" {
			return
		}
		current.IsMain = samePath(current.Path, mainRoot)
		worktrees = append(worktrees, *current)
		current = nil
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}

		key, value, hasValue := strings.Cut(line, " ")
		if key == "worktree" {
			flush()
			current = &git.Worktree{Path: value}
			continue
		}
		if current == nil {
			continue
		}

		switch key {
		case "HEAD":
			current.HeadOID = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			current.IsDetached = true
		case "locked":
			current.IsLocked = true
			if hasValue {
				current.LockReason = value
			}
		case "prunable":
			current.IsPrunable = true
			if hasValue {
				current.PrunableReason = value
			}
		case "bare":
			current.IsBare = true
		}
	}
	flush()

	return worktrees, nil
}

func (g *GitLocalAdapter) mainWorktreeRoot(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve git common dir: %w", err)
	}
	commonDir := strings.TrimSpace(string(output))
	if commonDir == "" {
		return "", fmt.Errorf("git common dir is empty")
	}
	return filepath.Clean(filepath.Dir(commonDir)), nil
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// RefreshStatus returns a Bubble Tea command that refreshes the status.
func (g *GitLocalAdapter) RefreshStatus(repoPath string) tea.Cmd {
	return func() tea.Msg {
		status, err := g.GetStatus(repoPath)
		return StatusEvent{
			RepoPath: repoPath,
			Status:   status,
			Error:    err,
		}
	}
}

// Ensure GitLocalAdapter implements GitAdapter
var _ GitAdapter = (*GitLocalAdapter)(nil)
