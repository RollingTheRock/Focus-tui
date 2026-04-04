package pomodoro

import (
	"os/exec"
	"runtime"
)

// notify sends a desktop notification.
func notify(message string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("osascript", "-e",
			`display notification "`+message+`" with title "Focus"`,
		).Run()
	case "linux":
		_ = exec.Command("notify-send", "Focus", message).Run()
	}
}
