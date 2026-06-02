package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration.
type Config struct {
	Weather struct {
		City string `yaml:"city"`
	} `yaml:"weather"`
	Quote struct {
		Source     string `yaml:"source"`
		CustomFile string `yaml:"custom_file"`
	} `yaml:"quote"`
	Avatar struct {
		Image string `yaml:"image"` // path to image file (jpg/png), rendered via chafa
		Width int    `yaml:"width"` // avatar width in columns (default 32)
	} `yaml:"avatar"`
	Shell struct {
		Mouse bool `yaml:"mouse"` // enable mouse forwarding to embedded shell
	} `yaml:"shell"`
	Pomodoro struct {
		WorkMinutes          int  `yaml:"work_minutes"`
		ShortBreakMinutes    int  `yaml:"short_break_minutes"`
		LongBreakMinutes     int  `yaml:"long_break_minutes"`
		LongBreakInterval    int  `yaml:"long_break_interval"`
		Notify               bool `yaml:"notify"`
		FocusReminderMinutes int  `yaml:"focus_reminder_minutes"`
	} `yaml:"pomodoro"`
	AdrDir     string `yaml:"adr_dir"`   // optional override for ADR directory (default: repoRoot/docs/adr)
	TimeFormat string `yaml:"time_format"`
	Agent      struct {
		ExternalTerminal     bool   `yaml:"external_terminal"`
		TerminalEmulator     string `yaml:"terminal_emulator"`
		MCPSocket            string `yaml:"mcp_socket"`
		MCPPort              string `yaml:"mcp_port"`
		A2ASocket            string `yaml:"a2a_socket"`
		ResearchProvider     string `yaml:"research_provider"`
		ArchitectureProvider string `yaml:"architecture_provider"`
		CodingProvider       string `yaml:"coding_provider"`
	} `yaml:"agent"`
	Experimental struct {
		UseCanvasCompositor bool `yaml:"use_canvas_compositor"`
	} `yaml:"experimental"`
	Editor struct {
		Command string `yaml:"command"` // external editor command (e.g. "nvim", "vim", "code --wait"). empty defaults to "nvim".
	} `yaml:"editor"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	var cfg Config
	cfg.Weather.City = "Shanghai"
	cfg.Quote.Source = "builtin"
	cfg.Avatar.Width = 32
	cfg.Pomodoro.WorkMinutes = 25
	cfg.Pomodoro.ShortBreakMinutes = 5
	cfg.Pomodoro.LongBreakMinutes = 15
	cfg.Pomodoro.LongBreakInterval = 4
	cfg.Pomodoro.Notify = true
	cfg.Pomodoro.FocusReminderMinutes = 90
	cfg.TimeFormat = "Mon Jan 2 · 15:04"
	cfg.Agent.ExternalTerminal = false
	cfg.Agent.TerminalEmulator = ""
	cfg.Agent.MCPSocket = ""
	cfg.Agent.MCPPort = "127.0.0.1:18766"
	cfg.Agent.A2ASocket = ""
	cfg.Agent.ResearchProvider = "kimi"
	cfg.Agent.ArchitectureProvider = "claude"
	cfg.Agent.CodingProvider = "codex,kimi,claude"
	return cfg
}

// Save writes configuration to ~/.config/focus/config.yaml.
func Save(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".config", "focus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.yaml")
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// Load reads configuration from ~/.config/focus/config.yaml.
// If the file does not exist or is malformed, it returns defaults.
func Load() Config {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".config", "focus", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "focus: warning: could not read config: %v\n", err)
		}
		return cfg
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "focus: warning: could not parse config: %v\n", err)
		return DefaultConfig()
	}

	return cfg
}
