// Package sessiondigest builds a post-session digest by reading external agent
// session transcripts (Claude Code, Codex, etc.) and combining them with the
// git diff produced during the session.
package sessiondigest

import (
	"fmt"
	"strings"
	"time"
)

// FileChange captures a single file change from git diff --stat.
type FileChange struct {
	Path      string
	Status    string // added, modified, deleted, renamed
	Additions int
	Deletions int
}

// CommandRun captures a shell command executed by the agent.
type CommandRun struct {
	Command string
	Output  string
}

// Digest is the complete沉淀report for a finished agent session.
type Digest struct {
	SessionProvider     string
	SessionID           string
	WorktreePath        string
	StartedAt           time.Time
	EndedAt             time.Time
	Summary             string
	FilesChanged        []FileChange
	CommandsRun         []CommandRun
	KeyDecisions        []string
	Blockers            []string
	ConversationTurns   int
	RawExcerpt          string
	GitDiffStat         string
	GitDiffFull         string
}

// ToMarkdown renders the digest as a Markdown document suitable for
// persistence in task_outputs or agent_sessions.summary.
func (d *Digest) ToMarkdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Session Digest: %s\n\n", d.SessionProvider)
	fmt.Fprintf(&b, "- **Session ID**: %s\n", d.SessionID)
	fmt.Fprintf(&b, "- **Worktree**: %s\n", d.WorktreePath)
	fmt.Fprintf(&b, "- **Duration**: %s → %s\n",
		d.StartedAt.Format(time.RFC3339),
		d.EndedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Conversation Turns**: %d\n\n", d.ConversationTurns)

	if d.Summary != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", d.Summary)
	}

	if len(d.KeyDecisions) > 0 {
		fmt.Fprintf(&b, "## Key Decisions\n\n")
		for _, dec := range d.KeyDecisions {
			fmt.Fprintf(&b, "- %s\n", dec)
		}
		b.WriteString("\n")
	}

	if len(d.Blockers) > 0 {
		fmt.Fprintf(&b, "## Blockers / Risks\n\n")
		for _, blk := range d.Blockers {
			fmt.Fprintf(&b, "- %s\n", blk)
		}
		b.WriteString("\n")
	}

	if len(d.FilesChanged) > 0 {
		fmt.Fprintf(&b, "## Files Changed\n\n")
		for _, fc := range d.FilesChanged {
			fmt.Fprintf(&b, "- `%s` (%s, +%d / -%d)\n",
				fc.Path, fc.Status, fc.Additions, fc.Deletions)
		}
		b.WriteString("\n")
	}

	if d.GitDiffStat != "" {
		fmt.Fprintf(&b, "## Git Diff Stat\n\n```\n%s\n```\n\n", d.GitDiffStat)
	}

	if len(d.CommandsRun) > 0 {
		fmt.Fprintf(&b, "## Commands Executed\n\n")
		for _, cr := range d.CommandsRun {
			fmt.Fprintf(&b, "### `%s`\n\n", cr.Command)
			if cr.Output != "" {
				fmt.Fprintf(&b, "```\n%s\n```\n\n", truncate(cr.Output, 2000))
			}
		}
	}

	if d.RawExcerpt != "" {
		fmt.Fprintf(&b, "## Conversation Excerpt\n\n%s\n\n", truncate(d.RawExcerpt, 3000))
	}

	if d.GitDiffFull != "" {
		fmt.Fprintf(&b, "## Full Git Diff\n\n```diff\n%s\n```\n", truncate(d.GitDiffFull, 5000))
	}

	return b.String()
}

// OneLineSummary returns a compact single-line summary for UI display.
func (d *Digest) OneLineSummary() string {
	files := len(d.FilesChanged)
	cmds := len(d.CommandsRun)
	turns := d.ConversationTurns

	parts := []string{}
	if d.Summary != "" {
		parts = append(parts, d.Summary)
	}
	if files > 0 {
		parts = append(parts, fmt.Sprintf("%d files changed", files))
	}
	if cmds > 0 {
		parts = append(parts, fmt.Sprintf("%d commands run", cmds))
	}
	if turns > 0 {
		parts = append(parts, fmt.Sprintf("%d turns", turns))
	}
	if len(parts) == 0 {
		return "Session completed."
	}
	return strings.Join(parts, " | ")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n\n... (truncated)"
}
