// Package ccswitch interacts with the local cc-switch CLI so that Focus can
// optionally launch Claude Code or Codex with a one-off provider override.
//
// It does NOT read cc-switch internal files directly; all data is obtained
// through the cc-switch CLI interface.
package ccswitch

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/agents"
)

// Provider holds a single cc-switch provider entry.
type Provider struct {
	ID             string
	Name           string
	AppType        string
	Model          string // best-effort model name extracted from settings
	IsCurrent      bool
	SettingsConfig map[string]any
}

// IsInstalled reports whether the cc-switch binary is available on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("cc-switch")
	return err == nil
}

// toCCSwitchAppType maps Focus provider names to cc-switch --app values.
func toCCSwitchAppType(appType string) string {
	switch appType {
	case "opencode":
		return "open-code"
	case "openclaw":
		return "open-claw"
	default:
		return appType
	}
}

// ListProviders returns all providers for the given app type (e.g. "claude" or "codex").
func ListProviders(appType string) ([]Provider, error) {
	ccsApp := toCCSwitchAppType(appType)
	out, err := exec.Command("cc-switch", "config", "show", "-a", ccsApp).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("cc-switch config show: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("cc-switch config show: %w", err)
	}

	// The output starts with a human-readable header ("Current Configuration\n===...")
	// followed by the JSON payload.  We locate the first '{' and parse from there.
	text := string(out)
	idx := strings.Index(text, "{")
	if idx == -1 {
		return nil, fmt.Errorf("cc-switch config show: no JSON found in output")
	}

	var cfg rawConfig
	if err := json.Unmarshal([]byte(text[idx:]), &cfg); err != nil {
		return nil, fmt.Errorf("cc-switch config show: parse JSON: %w", err)
	}

	var app appConfig
	switch appType {
	case "claude":
		app = cfg.Claude
	case "codex":
		app = cfg.Codex
	case "opencode":
		app = cfg.OpenCode
	case "gemini":
		app = cfg.Gemini
	default:
		return nil, fmt.Errorf("unsupported app type: %s", appType)
	}

	var result []Provider
	for id, rp := range app.Providers {
		p := Provider{
			ID:             id,
			Name:           rp.Name,
			AppType:        appType,
			IsCurrent:      app.Current == id,
			SettingsConfig: rp.SettingsConfig,
		}
		if p.Name == "" {
			p.Name = id
		}
		p.Model = extractModel(rp.SettingsConfig)
		result = append(result, p)
	}
	return result, nil
}

// rawConfig mirrors the top-level JSON shape emitted by `cc-switch config show`.
type rawConfig struct {
	Claude   appConfig `json:"claude"`
	Codex    appConfig `json:"codex"`
	OpenCode appConfig `json:"opencode"`
	Gemini   appConfig `json:"gemini"`
}

type appConfig struct {
	Providers map[string]rawProvider `json:"providers"`
	Current   string                 `json:"current"`
}

type rawProvider struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	SettingsConfig map[string]any `json:"settingsConfig"`
	Meta           struct {
		CommonConfigEnabled bool `json:"commonConfigEnabled"`
	} `json:"meta"`
	InFailoverQueue bool `json:"inFailoverQueue"`
}

// LaunchCommand returns the binary and arguments for launching the given agent
// with a one-off cc-switch provider override.  When configID is empty or the
// provider is not supported by cc-switch start, it returns empty values so the
// caller can fall back to the default provider binary.
func LaunchCommand(provider agents.Provider, configID string) (string, []string) {
	if configID == "" {
		return "", nil
	}
	switch provider {
	case agents.ProviderClaude:
		return "cc-switch", []string{"start", "claude", configID}
	case agents.ProviderCodex:
		return "cc-switch", []string{"start", "codex", configID}
	default:
		// Gemini, OpenCode, and unsupported providers: cc-switch start cannot
		// launch these directly.  The caller should use the default binary and
		// inject provider env vars via GetProviderEnvVars.
		return "", nil
	}
}

// GetProviderEnvVars returns the env vars for a specific cc-switch provider
// as KEY=VALUE strings, suitable for passing to an external command.
func GetProviderEnvVars(appType string, configID string) []string {
	providers, err := ListProviders(appType)
	if err != nil {
		return nil
	}
	for _, p := range providers {
		if p.ID == configID {
			return p.envVars()
		}
	}
	return nil
}

// envVars extracts KEY=VALUE environment variables from the provider's SettingsConfig.
//
// Two config shapes are supported:
//   - env (used by Claude / Gemini): keys are already env-var names.
//   - options (used by OpenCode): keys are camelCase option names that need
//     mapping to standard env vars (apiKey → OPENAI_API_KEY, baseURL →
//     OPENAI_BASE_URL).
func (p Provider) envVars() []string {
	// Prefer explicit env block (Claude, Gemini).
	if env, ok := p.SettingsConfig["env"].(map[string]any); ok && len(env) > 0 {
		var result []string
		for k, v := range env {
			if s, ok := v.(string); ok && s != "" {
				result = append(result, k+"="+s)
			}
		}
		return result
	}

	// Fall back to options block (OpenCode).
	if opts, ok := p.SettingsConfig["options"].(map[string]any); ok && len(opts) > 0 {
		var result []string
		for k, v := range opts {
			s, ok := v.(string)
			if !ok || s == "" {
				continue
			}
			switch k {
			case "apiKey":
				result = append(result, "OPENAI_API_KEY="+s)
			case "baseURL":
				result = append(result, "OPENAI_BASE_URL="+s)
			}
		}
		return result
	}

	return nil
}

// extractModel tries to pull a human-readable model name from the provider
// settings_config.  The exact schema differs per app, so we probe a few keys.
func extractModel(cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	// Claude Code stores the model under "model" or inside env vars.
	if v, ok := cfg["model"].(string); ok && v != "" {
		return v
	}
	if env, ok := cfg["env"].(map[string]any); ok {
		for _, key := range []string{
			"ANTHROPIC_MODEL",
			"ANTHROPIC_DEFAULT_OPUS_MODEL",
			"ANTHROPIC_DEFAULT_SONNET_MODEL",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL",
			"ANTHROPIC_REASONING_MODEL",
		} {
			if v, ok := env[key].(string); ok && v != "" {
				return v
			}
		}
	}
	// Codex / opencode store models under "models" map.
	if models, ok := cfg["models"].(map[string]any); ok {
		for _, m := range models {
			if mm, ok := m.(map[string]any); ok {
				if name, ok := mm["name"].(string); ok && name != "" {
					return name
				}
			}
		}
		// Fallback: use the first model id itself.
		for id := range models {
			return id
		}
	}
	return ""
}
