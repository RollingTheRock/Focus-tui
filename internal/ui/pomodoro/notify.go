package pomodoro

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/RollingTheRock/Focus-tui/internal/platform"
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
	case "windows":
		// PowerShell single-quoted strings only interpret '' as an escaped
		// single quote, so doubling every literal quote prevents injection.
		escaped := strings.ReplaceAll(message, "'", "''")
		script := fmt.Sprintf(
			`[System.Reflection.Assembly]::LoadWithPartialName("System.Windows.Forms") | Out-Null; `+
				`$n = New-Object System.Windows.Forms.NotifyIcon; `+
				`$n.Icon = [System.Drawing.SystemIcons]::Information; `+
				`$n.Visible = $true; `+
				`$n.ShowBalloonTip(5000, "Focus", '%s', [System.Windows.Forms.ToolTipIcon]::Info)`,
			escaped,
		)
		if encoded, err := platform.EncodePowerShellCommand(script); err == nil {
			_ = exec.Command("powershell", "-NoProfile", "-EncodedCommand", encoded).Run()
		}
	}
}
