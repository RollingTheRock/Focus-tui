package sessiondigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// findRecentJSONL scans dir for .jsonl files whose ModTime falls inside
// [startedAt, endedAt] and returns the most recently modified match.
func findRecentJSONL(dir string, startedAt, endedAt time.Time) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var best string
	var bestTime time.Time

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		mtime := info.ModTime()
		// Allow a 5-minute grace window before startedAt because the file may
		// have been created just before Focus registered the session.
		if mtime.Before(startedAt.Add(-5*time.Minute)) || mtime.After(endedAt.Add(5*time.Minute)) {
			continue
		}
		if best == "" || mtime.After(bestTime) {
			best = filepath.Join(dir, e.Name())
			bestTime = mtime
		}
	}

	if best == "" {
		return "", fmt.Errorf("no .jsonl session file found in %s for time window %s - %s",
			dir, startedAt.Format(time.RFC3339), endedAt.Format(time.RFC3339))
	}
	return best, nil
}

// encodeClaudeProjectPath converts an absolute filesystem path into the
// encoding used by Claude Code under ~/.claude/projects/.
// Rules (observed): replace '/', '\', ' ', and '~' with '-'.
func encodeClaudeProjectPath(absPath string) string {
	s := absPath
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "~", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

// claudeProjectsDir returns the Claude Code projects data directory.
func claudeProjectsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// codexSessionsDir returns the Codex CLI sessions directory.
func codexSessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "sessions")
}
