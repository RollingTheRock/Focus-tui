package sessiondigest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeReader reads Claude Code session transcripts from ~/.claude/projects/.
type ClaudeReader struct{}

// NewClaudeReader creates a new reader.
func NewClaudeReader() *ClaudeReader {
	return &ClaudeReader{}
}

// ReadSession locates the most recent Claude Code jsonl file for the given
// worktree and time window, then extracts events from it.
func (r *ClaudeReader) ReadSession(worktreePath string, startedAt, endedAt time.Time) ([]Event, error) {
	projectsDir := claudeProjectsDir()
	if projectsDir == "" {
		return nil, fmt.Errorf("cannot determine Claude projects directory")
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		absPath = worktreePath
	}
	encoded := encodeClaudeProjectPath(absPath)
	projectDir := filepath.Join(projectsDir, encoded)

	// Claude may also store sessions under the parent repo path when the
	// worktree is inside the repo (e.g. .worktrees/feat-x). Try the worktree
	// path first, then fall back to the parent directory.
	candidates := []string{projectDir}
	parent := filepath.Dir(absPath)
	if parent != "" && parent != absPath {
		candidates = append(candidates, filepath.Join(projectsDir, encodeClaudeProjectPath(parent)))
	}

	var sessionFile string
	for _, dir := range candidates {
		f, err := findRecentJSONL(dir, startedAt, endedAt)
		if err == nil {
			sessionFile = f
			break
		}
	}
	if sessionFile == "" {
		return nil, fmt.Errorf("no Claude Code session file found for %s", worktreePath)
	}

	return r.parseJSONL(sessionFile)
}

// parseJSONL reads a Claude Code .jsonl transcript and converts each line
// into a generic Event. Because Claude Code is closed-source we do a
// best-effort parse: we look for fields that are stable across versions.
func (r *ClaudeReader) parseJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	// Claude transcripts can be large; increase buffer.
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue // skip unparseable lines
		}

		ev := Event{Source: "claude", Raw: raw}

		// Detect event type from common keys.
		typeVal, _ := raw["type"].(string)
		switch typeVal {
		case "user":
			ev.Type = EventUser
			ev.Content, _ = raw["content"].(string)
		case "assistant":
			ev.Type = EventAssistant
			ev.Content, _ = raw["content"].(string)
		case "tool_use":
			ev.Type = EventToolUse
			ev.Content, _ = raw["name"].(string) // tool name
			if input, ok := raw["input"].(map[string]any); ok {
				// Flatten input keys for metadata.
				for k, v := range input {
					ev.Metadata[k] = fmt.Sprintf("%v", v)
				}
			}
		case "tool_result":
			ev.Type = EventToolResult
			ev.Content, _ = raw["content"].(string)
		case "file_history_snapshot":
			ev.Type = EventFileChange
			ev.Content, _ = raw["path"].(string)
		default:
			// Fallback: inspect other common keys.
			if _, ok := raw["content"]; ok {
				ev.Type = EventAssistant
				ev.Content, _ = raw["content"].(string)
			} else if _, ok := raw["name"]; ok {
				ev.Type = EventToolUse
				ev.Content, _ = raw["name"].(string)
			}
		}

		if ts, ok := raw["timestamp"].(string); ok {
			ev.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		}

		events = append(events, ev)
	}

	return events, scanner.Err()
}
