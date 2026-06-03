package trellis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		output := strings.TrimSpace(strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
		return "", fmt.Errorf("task.py create: %w: %s", err, output)
	}
	return strings.TrimSpace(stdout.String()), nil
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

// TaskList scans .trellis/tasks/ and returns active tasks by reading task.json.
func (c *Client) TaskList() ([]TrellisTask, error) {
	tasksDir := filepath.Join(c.repoRoot, ".trellis", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}
	var tasks []TrellisTask
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		taskJSON := filepath.Join(tasksDir, e.Name(), "task.json")
		data, err := os.ReadFile(taskJSON)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		var t TrellisTask
		if id, ok := raw["id"].(string); ok {
			t.ID = id
		}
		if name, ok := raw["name"].(string); ok {
			t.Name = name
		}
		if title, ok := raw["title"].(string); ok {
			t.Title = title
		}
		if status, ok := raw["status"].(string); ok {
			t.Status = status
		}
		if priority, ok := raw["priority"].(string); ok {
			t.Priority = priority
		}
		// Check meta.focus_task_id for Focus → Trellis mapping.
		if meta, ok := raw["meta"].(map[string]any); ok {
			if focusID, ok := meta["focus_task_id"].(string); ok && focusID != "" {
				t.ID = focusID
			}
		}
		// Backfill Name from directory if missing.
		if t.Name == "" {
			t.Name = e.Name()
		}
		tasks = append(tasks, t)
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
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Priority     string   `json:"priority"`
	RelatedFiles []string `json:"relatedFiles"`
}

// runWithTimeout executes a command with a timeout.
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	// Go 1.25 supports context.WithTimeout on exec.CommandContext.
	// For simplicity we use the default behavior; timeouts can be added later.
	return cmd.CombinedOutput()
}

// copyDir recursively copies a directory tree, overwriting existing files.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func appendToFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
