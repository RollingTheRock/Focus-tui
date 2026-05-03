package sessiondigest

import "time"

// EventType categorises a generic session event.
type EventType string

const (
	EventUser       EventType = "user"
	EventAssistant  EventType = "assistant"
	EventToolUse    EventType = "tool_use"
	EventToolResult EventType = "tool_result"
	EventCommand    EventType = "command"
	EventFileChange EventType = "file_change"
	EventUnknown    EventType = "unknown"
)

// Event is a provider-agnostic record extracted from an external agent
// session transcript.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Content   string
	Metadata  map[string]string
	Source    string // "claude", "codex", etc.
	Raw       map[string]any
}
