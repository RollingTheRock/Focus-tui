package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the platform-appropriate configuration directory.
// On Unix: ~/.config/focus
// On Windows: %APPDATA%\focus
func ConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "focus"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "focus"), nil
}

// DataDir returns the platform-appropriate data directory.
// On Unix: ~/.local/share/focus
// On Windows: %LOCALAPPDATA%\focus
func DataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "focus"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "focus"), nil
}
