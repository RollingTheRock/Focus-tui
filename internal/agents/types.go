package agents

import (
	"time"

	"github.com/google/uuid"
)

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

const SessionIDEnvVar = "FOCUS_AGENT_SESSION_ID"

type Session struct {
	ID             string
	Provider       Provider
	WorktreeID     string
	RepoID         string
	BranchSnapshot string
	PID            int
	State          SessionState
	StartedAt      time.Time
	EndedAt        *time.Time
	UpdatedAt      time.Time
}

func NewSessionID() string {
	return uuid.NewString()
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
