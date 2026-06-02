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
	ProviderGemini   Provider = "gemini"
	ProviderGeneric  Provider = "generic"
)

type SessionState string

const (
	SessionRunning      SessionState = "running"
	SessionExited       SessionState = "exited"
	SessionFailed       SessionState = "failed"
	SessionWaiting      SessionState = "waiting"
	SessionDisconnected SessionState = "disconnected"
	SessionUnknown      SessionState = "unknown"
)

const (
	SessionIDEnvVar       = "FOCUS_SESSION_ID"
	LegacySessionIDEnvVar = "FOCUS_AGENT_SESSION_ID"
	TaskIDEnvVar          = "FOCUS_TASK_ID"
	PlanIDEnvVar          = "FOCUS_PLAN_ID"
	MCPURLEnvVar          = "FOCUS_MCP_URL"
)

type Session struct {
ID               string
	Provider         Provider
	ProviderConfigID string // cc-switch provider ID for one-off override
	WorktreeID       string
	RepoID           string
	TaskID           string
	PlanID           string
	StepID           string
	DisplayTitle     string
	BranchSnapshot   string
	PID              int
	State            SessionState
	LaunchSource     string
	Summary          string
	EnvSnapshot      string
	ExtraArgs        []string
	Binary           string // custom binary for generic providers (e.g. "gemini")
	StartedAt        time.Time
	EndedAt          *time.Time
	LastActivityAt   *time.Time
	UpdatedAt        time.Time
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
	case ProviderGemini:
		return "gemini"
	default:
		return "agent"
	}
}
