package trellis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureKimiAdapter generates .kimi/ configuration for Kimi Code CLI.
// It creates YAML agent definitions, system prompt files, a SessionStart
// hook script, and registers the hook in ~/.kimi/config.toml.
func (b *Bridge) ensureKimiAdapter() error {
	kimiDir := filepath.Join(b.repoRoot, ".kimi")
	agentsDir := filepath.Join(kimiDir, "agents")
	hooksDir := filepath.Join(kimiDir, "hooks")

	for _, dir := range []string{kimiDir, agentsDir, hooksDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// 1. Generate Kimi agent YAML definitions + system prompt files.
	agentDefs := []struct {
		name        string
		description string
	}{
		{"trellis-implement", "Coding sub-agent. Writes code from the PRD with curated context. No git commit."},
		{"trellis-check", "Code quality check expert. Reviews diffs against specs, runs lint/typecheck/test, self-fixes."},
		{"trellis-research", "Codebase / doc search sub-agent. Read-only."},
	}
	for _, def := range agentDefs {
		yamlPath := filepath.Join(agentsDir, def.name+".yaml")
		promptPath := filepath.Join(agentsDir, def.name+".md")

		yamlContent := renderKimiAgentYAML(def.name, "./"+def.name+".md")
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			return fmt.Errorf("write agent yaml %s: %w", def.name, err)
		}

		promptContent := renderKimiAgentPrompt(def.name, def.description)
		if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
			return fmt.Errorf("write agent prompt %s: %w", def.name, err)
		}
	}

	// 2. Generate SessionStart hook script.
	hookPath := filepath.Join(hooksDir, "session-start.py")
	hookContent := renderKimiSessionStartHook()
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("write session-start hook: %w", err)
	}

	// 3. Register hook in ~/.kimi/config.toml.
	if err := b.updateKimiConfigHooks(); err != nil {
		return fmt.Errorf("update kimi config: %w", err)
	}

	return nil
}

func renderKimiAgentYAML(name, promptRelPath string) string {
	var b strings.Builder
	b.WriteString("version: 1\n")
	b.WriteString("agent:\n")
	fmt.Fprintf(&b, "  name: %s\n", name)
	b.WriteString("  extend: default\n")
	fmt.Fprintf(&b, "  system_prompt_path: %s\n", promptRelPath)
	return b.String()
}

func renderKimiAgentPrompt(name, description string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)
	fmt.Fprintf(&b, "**Role:** %s\n\n", description)
	b.WriteString("## Context Sources\n\n")
	b.WriteString("The following variables are automatically injected by Kimi Code CLI:\n")
	b.WriteString("- `${KIMI_AGENTS_MD}` — merged AGENTS.md from project root\n")
	b.WriteString("- `${KIMI_SKILLS}` — loaded skills list\n")
	b.WriteString("- `${KIMI_WORK_DIR}` — current working directory\n\n")
	b.WriteString("## Trellis Task Context\n\n")
	b.WriteString("Before you begin, load the active task context:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("cat .trellis/tasks/*/implement.jsonl 2>/dev/null || echo \"No active task\"\n")
	b.WriteString("```\n\n")
	b.WriteString("Then read each file referenced in that JSONL. After loading all context, proceed.\n")
	return b.String()
}

func renderKimiSessionStartHook() string {
	return `#!/usr/bin/env python3
"""Kimi SessionStart hook — injects Trellis context on session start.

Kimi Code CLI passes a JSON object via stdin with fields like:
  {"session_id": "...", "cwd": "/path/to/project",
   "hook_event_name": "SessionStart", "source": "startup"}

The script reads the current Trellis state and prints context to stdout,
which Kimi CLI injects into the session context.
"""

import json
import os
import subprocess
import sys


def _find_trellis_dir(start_path):
    """Walk up the directory tree to find .trellis/ (supports git worktrees)."""
    current = os.path.abspath(start_path or os.getcwd())
    while current != os.path.dirname(current):
        trellis_dir = os.path.join(current, ".trellis")
        if os.path.isdir(trellis_dir):
            return trellis_dir
        current = os.path.dirname(current)
    return None


def main():
    # Read hook context from stdin.
    try:
        hook_ctx = json.load(sys.stdin)
    except json.JSONDecodeError:
        hook_ctx = {}

    # Determine repo root: prefer cwd from hook context, then env, then os.getcwd().
    repo_root = hook_ctx.get("cwd", "")
    if not repo_root:
        repo_root = os.environ.get("KIMI_WORKSPACE", "")
    if not repo_root:
        repo_root = os.getcwd()

    # Walk up the tree to find .trellis/ (handles git worktrees where
    # .trellis lives in the main repo and the agent runs in a worktree).
    trellis_dir = _find_trellis_dir(repo_root)
    if not trellis_dir:
        return

    # Run get_context.py to get the full Trellis context.
    get_ctx = os.path.join(trellis_dir, "scripts", "get_context.py")
    if os.path.exists(get_ctx):
        result = subprocess.run(
            [sys.executable, get_ctx],
            capture_output=True,
            text=True,
            cwd=os.path.dirname(trellis_dir),
        )
        if result.returncode == 0 and result.stdout:
            print(result.stdout)


if __name__ == "__main__":
    main()
`
}

// updateKimiConfigHooks reads ~/.kimi/config.toml and idempotently adds
// the SessionStart hook that points to .kimi/hooks/session-start.py.
func (b *Bridge) updateKimiConfigHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".kimi", "config.toml")

	// Ensure ~/.kimi/ exists.
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Read existing config or start empty.
	var content string
	if data, err := os.ReadFile(configPath); err == nil {
		content = string(data)
	}

	// Check if our hook is already registered.
	if strings.Contains(content, ".kimi/hooks/session-start.py") {
		return nil
	}

	// Append hook block.
	hookBlock := `
# Focus-trellis SessionStart hook
[[hooks]]
event = "SessionStart"
matcher = "startup"
command = ".kimi/hooks/session-start.py"
timeout = 30
`

	// Ensure there is a blank line before the hook block.
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	content += strings.TrimLeft(hookBlock, "\n")
	content += "\n"

	return os.WriteFile(configPath, []byte(content), 0644)
}
