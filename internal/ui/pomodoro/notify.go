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
	case "windows":
		// Use PowerShell to show a Windows toast notification
		script := `[System.Reflection.Assembly]::LoadWithPartialName("System.Windows.Forms") | Out-Null; $n = New-Object System.Windows.Forms.NotifyIcon; $n.Icon = [System.Drawing.SystemIcons]::Information; $n.Visible = $true; $n.ShowBalloonTip(5000, "Focus", "` + message + `", [System.Windows.Forms.ToolTipIcon]::Info)`
		_ = exec.Command("powershell", "-Command", script).Run()
	}
}
