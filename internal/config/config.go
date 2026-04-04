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
	TimeFormat string `yaml:"time_format"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	var cfg Config
	cfg.Weather.City = "Shanghai"
	cfg.Quote.Source = "builtin"
	cfg.TimeFormat = "Mon Jan 2 · 15:04"
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
