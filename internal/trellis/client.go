package trellis

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Client wraps calls to the trellis CLI and .trellis/scripts/*.py.
type Client struct {
	repoRoot  string
	pythonCmd string
}

// NewClient creates a new Trellis client.
func NewClient(repoRoot, pythonCmd string) *Client {
	return &Client{
		repoRoot:  repoRoot,
		pythonCmd: pythonCmd,
	}
}

// Init runs `trellis init` with the given developer name and platform flags.
func (c *Client) Init(developerName string, platformFlags []string) error {
	args := []string{"init", "-u", developerName}
	args = append(args, platformFlags...)
	cmd := exec.Command("trellis", args...)
	cmd.Dir = c.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTrellisInitFailed, string(out))
	}
	return nil
}

// TaskCreate runs `task.py create` and returns the created task directory.
func (c *Client) TaskCreate(title string, opts TaskCreateOpts) (string, error) {
	args := []string{
		filepath.Join(".trellis", "scripts", "task.py"),
		"create", title,
	}
	if opts.Slug != "" {
		args = append(args, "--slug", opts.Slug)
	}
	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}
	if opts.Priority != "" {
		args = append(args, "--priority", opts.Priority)
	}
	if opts.Description != "" {
		args = append(args, "--description", opts.Description)
	}

	cmd := exec.Command(c.pythonCmd, args...)
	cmd.Dir = c.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("task.py create: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// TaskStart runs `task.py start` to set the current active task.
func (c *Client) TaskStart(taskDir string) error {
	cmd := exec.Command(c.pythonCmd,
		filepath.Join(".trellis", "scripts", "task.py"),
		"start", taskDir,
	)
	cmd.Dir = c.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("task.py start: %w: %s", err, string(out))
	}
	return nil
}

// TaskFinish runs `task.py finish` to clear the active task pointer.
func (c *Client) TaskFinish() error {
	cmd := exec.Command(c.pythonCmd,
		filepath.Join(".trellis", "scripts", "task.py"),
		"finish",
	)
	cmd.Dir = c.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("task.py finish: %w: %s", err, string(out))
	}
	return nil
}

// TaskArchive runs `task.py archive` to archive a completed task.
func (c *Client) TaskArchive(taskName string) error {
	cmd := exec.Command(c.pythonCmd,
		filepath.Join(".trellis", "scripts", "task.py"),
		"archive", taskName,
	)
	cmd.Dir = c.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("task.py archive: %w: %s", err, string(out))
	}
	return nil
}

// TaskList runs `task.py list` and returns the JSON array of tasks.
func (c *Client) TaskList() ([]TrellisTask, error) {
	cmd := exec.Command(c.pythonCmd,
		filepath.Join(".trellis", "scripts", "task.py"),
		"list",
	)
	cmd.Dir = c.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("task.py list: %w", err)
	}

	var tasks []TrellisTask
	if err := json.Unmarshal(out, &tasks); err != nil {
		return nil, fmt.Errorf("parse task list: %w", err)
	}
	return tasks, nil
}

// GetContext runs `get_context.py` with optional mode/task flags.
func (c *Client) GetContext(extraArgs ...string) (string, error) {
	args := []string{
		filepath.Join(".trellis", "scripts", "get_context.py"),
	}
	args = append(args, extraArgs...)

	cmd := exec.Command(c.pythonCmd, args...)
	cmd.Dir = c.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get_context.py: %w", err)
	}
	return string(out), nil
}

// AddSession runs `add_session.py` to record a session to the journal.
func (c *Client) AddSession(title string, commits []string) error {
	cmd := exec.Command(c.pythonCmd,
		filepath.Join(".trellis", "scripts", "add_session.py"),
		"--title", title,
		"--commit", strings.Join(commits, ","),
	)
	cmd.Dir = c.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add_session.py: %w: %s", err, string(out))
	}
	return nil
}

// Version runs `trellis --version` and returns the version string.
func (c *Client) Version() (string, error) {
	cmd := exec.Command("trellis", "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// TaskCreateOpts holds optional parameters for TaskCreate.
type TaskCreateOpts struct {
	Slug        string
	Assignee    string
	Priority    string
	Description string
}

// TrellisTask represents a task as returned by task.py list.
type TrellisTask struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// runWithTimeout executes a command with a timeout.
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	// Go 1.25 supports context.WithTimeout on exec.CommandContext.
	// For simplicity we use the default behavior; timeouts can be added later.
	return cmd.CombinedOutput()
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	// Implementation using filepath.Walk or os.ReadDir.
	// Placeholder: in practice use a helper from internal/util or stdlib.
	return nil
}

// appendToFile appends content to a file, creating it if necessary.
func appendToFile(path, content string) error {
	// Implementation: open with O_APPEND|O_CREATE|O_WRONLY.
	return nil
}

var _ = runWithTimeout // silence unused warning until fully implemented
var _ = copyDir
var _ = appendToFile
