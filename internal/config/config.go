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
		WorkMinutes       int  `yaml:"work_minutes"`
		ShortBreakMinutes int  `yaml:"short_break_minutes"`
		LongBreakMinutes  int  `yaml:"long_break_minutes"`
		LongBreakInterval int  `yaml:"long_break_interval"`
		Notify            bool `yaml:"notify"`
	} `yaml:"pomodoro"`
	TimeFormat string `yaml:"time_format"`
	Agent      struct {
		ExternalTerminal bool   `yaml:"external_terminal"`
		TerminalEmulator string `yaml:"terminal_emulator"`
		MCPSocket        string `yaml:"mcp_socket"`
		A2ASocket        string `yaml:"a2a_socket"`
	} `yaml:"agent"`
	Experimental struct {
		UseCanvasCompositor bool `yaml:"use_canvas_compositor"`
	} `yaml:"experimental"`
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
	cfg.TimeFormat = "Mon Jan 2 · 15:04"
	cfg.Agent.ExternalTerminal = false
	cfg.Agent.TerminalEmulator = "kitty"
	cfg.Agent.MCPSocket = "/tmp/focus-mcp.sock"
	cfg.Agent.A2ASocket = "/tmp/focus-a2a.sock"
	return cfg
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
