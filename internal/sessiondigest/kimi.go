package sessiondigest

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KimiReader reads Kimi CLI session transcripts from ~/.kimi/sessions/.
type KimiReader struct{}

// NewKimiReader creates a new reader.
func NewKimiReader() *KimiReader {
	return &KimiReader{}
}

// ReadSession locates the most recent Kimi CLI context.jsonl file for the
// given worktree and time window, then extracts events from it.
func (r *KimiReader) ReadSession(worktreePath string, startedAt, endedAt time.Time) ([]Event, error) {
	baseDir := kimiSessionsDir()
	if baseDir == "" {
		return nil, fmt.Errorf("cannot determine Kimi sessions directory")
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		absPath = worktreePath
	}
	encoded := encodeKimiWorktreePath(absPath)
	sessionsDir := filepath.Join(baseDir, encoded)

	sessionFile, err := r.findSessionFile(sessionsDir, startedAt, endedAt)
	if err != nil {
		return nil, err
	}

	return r.parseJSONL(sessionFile)
}

// findSessionFile scans the worktree-specific sessions directory for
// context.jsonl files whose parent directory ModTime falls inside
// [startedAt, endedAt].
func (r *KimiReader) findSessionFile(sessionsDir string, startedAt, endedAt time.Time) (string, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", err
	}

	var bestFile string
	var bestTime time.Time

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		mtime := info.ModTime()
		// Allow a 5-minute grace window.
		if mtime.Before(startedAt.Add(-5*time.Minute)) || mtime.After(endedAt.Add(5*time.Minute)) {
			continue
		}
		candidate := filepath.Join(sessionsDir, e.Name(), "context.jsonl")
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		if bestFile == "" || mtime.After(bestTime) {
			bestFile = candidate
			bestTime = mtime
		}
	}

	if bestFile == "" {
		return "", fmt.Errorf("no Kimi session file found in %s for time window %s - %s",
			sessionsDir, startedAt.Format(time.RFC3339), endedAt.Format(time.RFC3339))
	}
	return bestFile, nil
}

// parseJSONL reads a Kimi context.jsonl transcript and converts each line
// into a generic Event.
func (r *KimiReader) parseJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
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
			continue
		}

		role, _ := raw["role"].(string)
		// Skip internal markers.
		if role == "_system_prompt" || role == "_checkpoint" || role == "_usage" {
			continue
		}

		ev := Event{Source: "kimi", Raw: raw}

		switch role {
		case "user":
			ev.Type = EventUser
			if content, ok := raw["content"].(string); ok {
				ev.Content = content
			}
		case "assistant":
			ev.Type = EventAssistant
			ev.Content = extractKimiAssistantText(raw)
		default:
			// Unknown role; try to extract any text content as assistant.
			if content, ok := raw["content"].(string); ok && content != "" {
				ev.Type = EventAssistant
				ev.Content = content
			} else {
				continue
			}
		}

		events = append(events, ev)
	}

	return events, scanner.Err()
}

// extractKimiAssistantText extracts the visible text from a Kimi assistant
// message. Kimi stores assistant content as an array of parts (think + text).
func extractKimiAssistantText(raw map[string]any) string {
	content, ok := raw["content"]
	if !ok {
		return ""
	}

	// If content is a plain string, return it directly.
	if s, ok := content.(string); ok {
		return s
	}

	// If content is an array of parts, extract text parts.
	parts, ok := content.([]any)
	if !ok {
		return ""
	}

	var texts []string
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			continue
		}
		partType, _ := partMap["type"].(string)
		if partType != "text" {
			continue
		}
		if text, ok := partMap["text"].(string); ok {
			texts = append(texts, text)
		}
	}

	return strings.Join(texts, "")
}

// encodeKimiWorktreePath converts an absolute filesystem path into the MD5
// hash used by Kimi CLI under ~/.kimi/sessions/.
func encodeKimiWorktreePath(absPath string) string {
	h := md5.New()
	h.Write([]byte(absPath))
	return hex.EncodeToString(h.Sum(nil))
}

// kimiSessionsDir returns the Kimi CLI sessions data directory.
func kimiSessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kimi", "sessions")
}
