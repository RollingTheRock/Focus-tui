package sessiondigest

import (
	"fmt"
	"strings"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/agents"
)

// Builder assembles a Digest from an external agent session file and git diff.
type Builder struct {
	gitCollector *GitDiffCollector
}

// NewBuilder creates a new digest builder.
func NewBuilder() *Builder {
	return &Builder{
		gitCollector: NewGitDiffCollector(),
	}
}

// Build creates a Digest for the given agent session. It attempts to read
// the external agent's transcript (Claude Code, Codex, etc.), collects the
// git diff from the worktree, and merges both into a unified report.
func (b *Builder) Build(
	provider agents.Provider,
	worktreePath string,
	branchSnapshot string,
	startedAt, endedAt time.Time,
) (*Digest, error) {
	digest := &Digest{
		SessionProvider: string(provider),
		WorktreePath:    worktreePath,
		StartedAt:       startedAt,
		EndedAt:         endedAt,
	}

	// 1. Read external agent session transcript.
	events, err := b.readEvents(provider, worktreePath, startedAt, endedAt)
	if err == nil {
		digest = b.applyEvents(digest, events)
	}
	// If reading fails we continue with just the git diff.

	// 2. Collect git diff.
	changes, stat, full, err := b.gitCollector.Collect(worktreePath, branchSnapshot)
	if err == nil {
		digest.FilesChanged = changes
		digest.GitDiffStat = stat
		digest.GitDiffFull = full
	}

	// 3. Generate a one-line summary if we have enough data.
	if digest.Summary == "" {
		digest.Summary = b.inferSummary(digest)
	}

	return digest, nil
}

func (b *Builder) readEvents(provider agents.Provider, worktreePath string, startedAt, endedAt time.Time) ([]Event, error) {
	switch provider {
	case agents.ProviderClaude:
		return NewClaudeReader().ReadSession(worktreePath, startedAt, endedAt)
	case agents.ProviderCodex:
		return NewCodexReader().ReadSession(worktreePath, startedAt, endedAt)
	case agents.ProviderKimi:
		return NewKimiReader().ReadSession(worktreePath, startedAt, endedAt)
	default:
		return nil, fmt.Errorf("no session reader for provider %q", provider)
	}
}

func (b *Builder) applyEvents(digest *Digest, events []Event) *Digest {
	var userTurns, assistantTurns int
	var lastUserMessage string
	var excerpts []string

	for _, ev := range events {
		switch ev.Type {
		case EventUser:
			userTurns++
			lastUserMessage = ev.Content
		case EventAssistant:
			assistantTurns++
			if len(excerpts) < 5 && ev.Content != "" {
				excerpts = append(excerpts, ev.Content)
			}
		case EventCommand:
			digest.CommandsRun = append(digest.CommandsRun, CommandRun{
				Command: ev.Content,
			})
		case EventToolUse:
			// Track file-related tool usage.
			name := strings.ToLower(ev.Content)
			if strings.Contains(name, "write") || strings.Contains(name, "edit") {
				if path, ok := ev.Metadata["file_path"]; ok && path != "" {
					digest.FilesChanged = appendIfMissingFile(digest.FilesChanged, path)
				}
				if path, ok := ev.Metadata["path"]; ok && path != "" {
					digest.FilesChanged = appendIfMissingFile(digest.FilesChanged, path)
				}
			}
		case EventFileChange:
			if ev.Content != "" {
				digest.FilesChanged = appendIfMissingFile(digest.FilesChanged, ev.Content)
			}
		}
	}

	digest.ConversationTurns = userTurns + assistantTurns
	if len(excerpts) > 0 {
		digest.RawExcerpt = strings.Join(excerpts, "\n\n---\n\n")
	}

	// Generate a summary from the last user request + outcome.
	if lastUserMessage != "" {
		digest.Summary = fmt.Sprintf("Task: %s", truncateSentence(lastUserMessage, 120))
	}

	return digest
}

func (b *Builder) inferSummary(digest *Digest) string {
	parts := []string{}
	if len(digest.FilesChanged) > 0 {
		parts = append(parts, fmt.Sprintf("%d files changed", len(digest.FilesChanged)))
	}
	if len(digest.CommandsRun) > 0 {
		parts = append(parts, fmt.Sprintf("%d commands executed", len(digest.CommandsRun)))
	}
	if len(parts) == 0 {
		return "Session completed (no transcript or diff captured)."
	}
	return "Session completed: " + strings.Join(parts, ", ") + "."
}

func appendIfMissingFile(list []FileChange, path string) []FileChange {
	for _, fc := range list {
		if fc.Path == path {
			return list
		}
	}
	return append(list, FileChange{Path: path, Status: "modified"})
}

func truncateSentence(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Try to cut at the last space before max.
	if idx := strings.LastIndex(s[:max], " "); idx > 0 {
		return s[:idx] + "..."
	}
	return s[:max] + "..."
}
