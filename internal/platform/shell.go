package platform

import (
	"os"
	"runtime"
)

// DefaultShell returns the user's preferred shell for the current platform.
// On Unix, it reads $SHELL and falls back to /bin/sh.
// On Windows, it reads %ComSpec% and falls back to cmd.exe.
func DefaultShell() string {
	if runtime.GOOS == "windows" {
		if sh := os.Getenv("ComSpec"); sh != "" {
			return sh
		}
		return "cmd.exe"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}
