package models

import "time"

// AgentCapabilities declares what an agent can do.
type AgentCapabilities struct {
	SupportsMCP      bool
	SupportsA2A      bool
	SupportsResume   bool
	SupportsCCSwitch bool
	HeadlessDriver   bool
}

// AgentDefinition represents a registered agent that can be launched from Focus.
type AgentDefinition struct {
	ID           string
	Name         string
	Description  string // e.g. "Anthropic", "OpenAI", "Google" — one phrase
	Binary       string
	Args         []string
	EnvVars      []string
	ProviderType string
	Tags         []string
	Category     string // built-in | registered | recommended
	InstallHint  string // shown in install-hint overlay
	Capabilities AgentCapabilities
	IsInstalled  bool
	IsEnabled    bool
	LastUsedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
