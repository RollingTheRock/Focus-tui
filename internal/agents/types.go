package agents

import "time"

type Provider string

const (
	ProviderOpenCode Provider = "opencode"
	ProviderClaude   Provider = "claude"
	ProviderKimi     Provider = "kimi"
	ProviderCodex    Provider = "codex"
	ProviderGeneric  Provider = "generic"
)

type SessionState string

const (
	SessionRunning SessionState = "running"
	SessionExited  SessionState = "exited"
	SessionUnknown SessionState = "unknown"
)

type Session struct {
	ID         string
	Provider   Provider
	WorktreeID string
	PID        int
	State      SessionState
	StartedAt  time.Time
}

func (s Session) DisplayName() string {
	switch s.Provider {
	case ProviderOpenCode:
		return "opencode"
	case ProviderClaude:
		return "claude"
	case ProviderKimi:
		return "kimi"
	case ProviderCodex:
		return "codex"
	default:
		return "agent"
	}
}
