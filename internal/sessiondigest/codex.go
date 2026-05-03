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

// CodexReader reads Codex CLI session transcripts from ~/.codex/sessions/.
type CodexReader struct{}

// NewCodexReader creates a new reader.
func NewCodexReader() *CodexReader {
	return &CodexReader{}
}

// ReadSession locates the most recent Codex rollout jsonl file for the given
// time window and extracts events.
func (r *CodexReader) ReadSession(_ string, startedAt, endedAt time.Time) ([]Event, error) {
	baseDir := codexSessionsDir()
	if baseDir == "" {
		return nil, fmt.Errorf("cannot determine Codex sessions directory")
	}

	// Codex stores files under YYYY/MM/DD/.  Scan the date-range directories
	// that overlap with [startedAt, endedAt].
	var dirs []string
	for d := startedAt.Truncate(24 * time.Hour); !d.After(endedAt); d = d.Add(24 * time.Hour) {
		dirs = append(dirs, filepath.Join(baseDir,
			d.Format("2006"), d.Format("01"), d.Format("02")))
	}

	var sessionFile string
	var bestTime time.Time
	for _, dir := range dirs {
		f, err := findRecentJSONL(dir, startedAt, endedAt)
		if err == nil {
			info, _ := os.Stat(f)
			if info != nil && (sessionFile == "" || info.ModTime().After(bestTime)) {
				sessionFile = f
				bestTime = info.ModTime()
			}
		}
	}
	if sessionFile == "" {
		return nil, fmt.Errorf("no Codex session file found for time window %s - %s",
			startedAt.Format(time.RFC3339), endedAt.Format(time.RFC3339))
	}

	return r.parseJSONL(sessionFile)
}

// parseJSONL reads a Codex rollout .jsonl file and converts lines into
// generic Events. Codex event types (from public issues/docs):
//   thread.started, turn.started, item.started, item.completed, turn.completed
func (r *CodexReader) parseJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	const maxCapacity = 1024 * 1024
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

		ev := Event{Source: "codex", Raw: raw}

		typeVal, _ := raw["type"].(string)
		switch typeVal {
		case "thread.started":
			// skip metadata-only lines
			continue
		case "turn.started", "turn.completed":
			continue
		case "item.started":
			item, _ := raw["item"].(map[string]any)
			if item == nil {
				continue
			}
			itemType, _ := item["type"].(string)
			switch itemType {
			case "command_execution":
				ev.Type = EventCommand
				ev.Content, _ = item["command"].(string)
			case "file_edit":
				ev.Type = EventFileChange
				ev.Content, _ = item["file_path"].(string)
			case "agent_message":
				ev.Type = EventAssistant
				ev.Content, _ = item["text"].(string)
			default:
				ev.Type = EventToolUse
				ev.Content = itemType
			}
		case "item.completed":
			item, _ := raw["item"].(map[string]any)
			if item == nil {
				continue
			}
			itemType, _ := item["type"].(string)
			if itemType == "agent_message" {
				ev.Type = EventAssistant
				ev.Content, _ = item["text"].(string)
			} else if itemType == "command_execution" {
				ev.Type = EventCommand
				// Try to capture output if present.
				if out, ok := item["output"].(string); ok {
					ev.Content = out
				} else {
					ev.Content, _ = item["command"].(string)
				}
			} else {
				continue
			}
		default:
			continue
		}

		if ts, ok := raw["timestamp"].(string); ok {
			ev.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		}

		events = append(events, ev)
	}

	return events, scanner.Err()
}
