package agents

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"focus/internal/models"
)

// detectBinary checks whether a binary is available in PATH.
// It first tries exec.LookPath (fast, uses the current process environment).
// If that fails, it falls back to launching the user's login shell with
// "command -v" so that PATH modifications in ~/.zshrc / ~/.bashrc are
// picked up (e.g. after npm/pip global installs).
func detectBinary(binary string) (string, error) {
	// Fast path: current process PATH.
	if path, err := exec.LookPath(binary); err == nil {
		return path, nil
	}

	// Fallback: start a login shell to get the user's full environment.
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-lc", fmt.Sprintf("command -v %s", binary))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not found in PATH or login shell: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("command -v returned empty for %s", binary)
	}
	return path, nil
}

// AgentStore defines the storage interface needed by DiscoveryRegistry.
type AgentStore interface {
	ListAgentDefinitions() ([]models.AgentDefinition, error)
	GetAgentDefinition(id string) (*models.AgentDefinition, error)
	SaveAgentDefinition(def models.AgentDefinition) error
	DeleteAgentDefinition(id string) error
}

// DiscoveryRegistry scans the system for installed agents and maintains
// the agent definition registry.
type DiscoveryRegistry struct {
	store AgentStore
}

// NewDiscoveryRegistry creates a new discovery registry.
func NewDiscoveryRegistry(store AgentStore) *DiscoveryRegistry {
	return &DiscoveryRegistry{store: store}
}

// BootstrapIfEmpty initializes the registry with built-in and recommended
// agents if the table is empty.
func (d *DiscoveryRegistry) BootstrapIfEmpty() error {
	defs, err := d.store.ListAgentDefinitions()
	if err != nil {
		return fmt.Errorf("check existing definitions: %w", err)
	}
	if len(defs) > 0 {
		return nil
	}

	now := time.Now()

	// Built-in agents.
	builtins := []models.AgentDefinition{
		{
			ID: "claude", Name: "Claude Code", Description: "Anthropic",
			Binary: "claude", ProviderType: string(ProviderClaude),
			Tags: []string{"coding", "architecture"}, Category: "built-in",
			IsEnabled: true, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "kimi", Name: "Kimi", Description: "Moonshot AI",
			Binary: "kimi", ProviderType: string(ProviderKimi),
			Tags: []string{"coding", "research"}, Category: "built-in",
			IsEnabled: true, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "codex", Name: "Codex", Description: "OpenAI",
			Binary: "codex", ProviderType: string(ProviderCodex),
			Tags: []string{"coding"}, Category: "built-in",
			IsEnabled: true, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "opencode", Name: "OpenCode", Description: "Community",
			Binary: "opencode", ProviderType: string(ProviderOpenCode),
			Tags: []string{"coding", "open-source"}, Category: "built-in",
			IsEnabled: true, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "gemini", Name: "Gemini CLI", Description: "Google",
			Binary: "gemini", ProviderType: string(ProviderGemini),
			Tags: []string{"coding", "research"}, Category: "built-in",
			IsEnabled: true, CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, def := range builtins {
		if err := d.store.SaveAgentDefinition(def); err != nil {
			return fmt.Errorf("save built-in %s: %w", def.ID, err)
		}
	}

	// Recommended agents.
	recommended := []models.AgentDefinition{
		{
			ID: "aider", Name: "Aider", Description: "Paul Gauthier",
			Binary: "aider", ProviderType: string(ProviderGeneric),
			Tags: []string{"pair-programming"}, Category: "recommended",
			InstallHint: "pip install aider-chat  # or: brew install aider",
			IsEnabled: false, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "goose", Name: "Goose", Description: "Block (AAIF)",
			Binary: "goose", ProviderType: string(ProviderGeneric),
			Tags: []string{"mcp-native", "local"}, Category: "recommended",
			InstallHint: "brew install block-goose-cli  # or: curl -fsSL https://github.com/aaif-goose/goose/releases/download/stable/download_cli.sh | bash",
			IsEnabled: false, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "open-interpreter", Name: "Open Interpreter", Description: "Killian",
			Binary: "interpreter", ProviderType: string(ProviderGeneric),
			Tags: []string{"automation"}, Category: "recommended",
			InstallHint: "pip install open-interpreter",
			IsEnabled: false, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "crush", Name: "Crush", Description: "Charmbracelet",
			Binary: "crush", ProviderType: string(ProviderGeneric),
			Tags: []string{"tui", "beautiful"}, Category: "recommended",
			InstallHint: "brew install charmbracelet/tap/crush  # or: go install github.com/charmbracelet/crush@latest",
			IsEnabled: false, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "qwen-code", Name: "Qwen Code", Description: "Alibaba",
			Binary: "qwen", ProviderType: string(ProviderGeneric),
			Tags: []string{"open-weights"}, Category: "recommended",
			InstallHint: "npm install -g @qwen-code/qwen-code@latest  # or: brew install qwen-code",
			IsEnabled: false, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "openhands", Name: "OpenHands", Description: "AllHands",
			Binary: "openhands", ProviderType: string(ProviderGeneric),
			Tags: []string{"automation", "sandbox"}, Category: "recommended",
			InstallHint: "pip install openhands  # or: uv tool install openhands --python 3.12",
			IsEnabled: false, CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, def := range recommended {
		if err := d.store.SaveAgentDefinition(def); err != nil {
			return fmt.Errorf("save recommended %s: %w", def.ID, err)
		}
	}

	return d.Scan()
}

// Scan checks PATH for every registered agent binary and updates IsInstalled.
func (d *DiscoveryRegistry) Scan() error {
	defs, err := d.store.ListAgentDefinitions()
	if err != nil {
		return fmt.Errorf("list definitions for scan: %w", err)
	}

	for _, def := range defs {
		_, lookErr := detectBinary(def.Binary)
		installed := lookErr == nil

		// Only update if status changed.
		if def.IsInstalled != installed {
			def.IsInstalled = installed
			// Auto-disable if no longer installed.
			if !installed && def.IsEnabled {
				def.IsEnabled = false
			}
			def.UpdatedAt = time.Now()
			if err := d.store.SaveAgentDefinition(def); err != nil {
				return fmt.Errorf("update scan result for %s: %w", def.ID, err)
			}
		}
	}
	return nil
}

// ListEnabled returns agents that are both installed and enabled.
// These are the agents shown in the launch panel.
func (d *DiscoveryRegistry) ListEnabled() ([]models.AgentDefinition, error) {
	defs, err := d.store.ListAgentDefinitions()
	if err != nil {
		return nil, err
	}
	var out []models.AgentDefinition
	for _, def := range defs {
		if def.IsInstalled && def.IsEnabled {
			out = append(out, def)
		}
	}
	return out, nil
}

// ListAll returns all agent definitions for the Store view.
func (d *DiscoveryRegistry) ListAll() ([]models.AgentDefinition, error) {
	return d.store.ListAgentDefinitions()
}

// ToggleEnabled flips the enabled state of an agent.
func (d *DiscoveryRegistry) ToggleEnabled(id string) error {
	def, err := d.store.GetAgentDefinition(id)
	if err != nil || def == nil {
		return fmt.Errorf("agent not found: %s", id)
	}
	if !def.IsInstalled {
		return fmt.Errorf("agent %s is not installed", id)
	}
	def.IsEnabled = !def.IsEnabled
	def.UpdatedAt = time.Now()
	return d.store.SaveAgentDefinition(*def)
}

// RegisterCustom adds a user-defined agent.
func (d *DiscoveryRegistry) RegisterCustom(def models.AgentDefinition) error {
	def.Category = "registered"
	def.IsEnabled = true
	def.CreatedAt = time.Now()
	def.UpdatedAt = time.Now()

	// Check if binary exists.
	_, err := detectBinary(def.Binary)
	def.IsInstalled = err == nil

	return d.store.SaveAgentDefinition(def)
}

// DeleteCustom removes a user-defined agent.
func (d *DiscoveryRegistry) DeleteCustom(id string) error {
	def, err := d.store.GetAgentDefinition(id)
	if err != nil || def == nil {
		return fmt.Errorf("agent not found: %s", id)
	}
	if def.Category != "registered" {
		return fmt.Errorf("cannot delete built-in or recommended agent: %s", id)
	}
	return d.store.DeleteAgentDefinition(id)
}
