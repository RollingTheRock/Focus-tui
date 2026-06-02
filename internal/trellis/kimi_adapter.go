package trellis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureKimiAdapter generates .kimi/ configuration so that Kimi Code CLI
// receives the same Trellis experience as Claude Code. Trellis does not
// natively support Kimi as a platform, so Focus generates the adapter files.
func (b *Bridge) ensureKimiAdapter() error {
	kimiDir := filepath.Join(b.repoRoot, ".kimi")
	agentsDir := filepath.Join(kimiDir, "agents")
	skillsDir := filepath.Join(kimiDir, "skills")
	hooksDir := filepath.Join(kimiDir, "hooks")

	// Create directories.
	for _, dir := range []string{kimiDir, agentsDir, skillsDir, hooksDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// 1. Copy .agents/skills/ to .kimi/skills/.
	srcSkills := filepath.Join(b.repoRoot, ".agents", "skills")
	if stat, err := os.Stat(srcSkills); err == nil && stat.IsDir() {
		if err := copyDir(srcSkills, skillsDir); err != nil {
			return fmt.Errorf("copy skills: %w", err)
		}
	}

	// 2. Generate Kimi sub-agent definitions.
	agentDefs := []struct {
		name        string
		description string
	}{
		{"trellis-implement", "Coding sub-agent. Writes code from the PRD with curated context. No git commit."},
		{"trellis-check", "Code quality check expert. Reviews diffs against specs, runs lint/typecheck/test, self-fixes."},
		{"trellis-research", "Codebase / doc search sub-agent. Read-only."},
	}
	for _, def := range agentDefs {
		agentPath := filepath.Join(agentsDir, def.name+".md")
		content := renderKimiAgentDefinition(def.name, def.description)
		if err := os.WriteFile(agentPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write agent %s: %w", def.name, err)
		}
	}

	// 3. Generate Kimi hooks.
	// Kimi supports hooks via /hooks command and configuration files.
	// The exact format depends on Kimi CLI version; we generate a Python hook
	// that mirrors Claude Code's session-start.py behavior.
	hookPath := filepath.Join(hooksDir, "session-start.py")
	hookContent := renderKimiSessionStartHook()
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("write session-start hook: %w", err)
	}

	return nil
}

func renderKimiAgentDefinition(name, description string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: |\n  %s\ntools: Read, Write, Edit, Bash, Glob, Grep\n---\n\n", name, description)
	fmt.Fprintf(&b, "# %s\n\n", name)
	b.WriteString("Instructions for the sub-agent go here. Treat it as the sub-agent's system prompt.\n\n")
	b.WriteString("Before you begin, read your context file:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("cat .trellis/tasks/*/implement.jsonl 2>/dev/null || echo \"No active task\"\n")
	b.WriteString("```\n\n")
	b.WriteString("Then read each file referenced in that JSONL. After loading all context, proceed.\n")
	return b.String()
}

func renderKimiSessionStartHook() string {
	return `#!/usr/bin/env python3
"""Kimi SessionStart hook — injects Trellis context on session start.

This hook mirrors Claude Code's session-start.py for Kimi Code CLI.
It reads .trellis/ state and outputs context for the Kimi session.
"""

import json
import os
import subprocess
import sys


def main():
    repo_root = os.environ.get("KIMI_WORKSPACE", os.getcwd())
    trellis_dir = os.path.join(repo_root, ".trellis")

    if not os.path.isdir(trellis_dir):
        return

    # Run get_context.py to get the full Trellis context.
    get_ctx = os.path.join(trellis_dir, "scripts", "get_context.py")
    if os.path.exists(get_ctx):
        result = subprocess.run(
            [sys.executable, get_ctx],
            capture_output=True,
            text=True,
            cwd=repo_root,
        )
        if result.returncode == 0 and result.stdout:
            # Kimi hooks can output plain text that gets injected.
            print(result.stdout)


if __name__ == "__main__":
    main()
`
}
