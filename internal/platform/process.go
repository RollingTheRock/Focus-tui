package platform

import (
	"os/exec"
	"runtime"
	"strconv"
)

// KillProcess terminates a process by PID.
// On Unix, it sends SIGTERM; on Windows, it first asks the process tree to
// shut down gracefully via taskkill, and only falls back to /F if that fails.
func KillProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	pidStr := strconv.Itoa(pid)
	if runtime.GOOS != "windows" {
		return exec.Command("kill", "-TERM", pidStr).Run()
	}
	// Windows: try graceful termination first (no /F).
	if err := exec.Command("taskkill", "/PID", pidStr, "/T").Run(); err == nil {
		return nil
	}
	// Graceful termination failed or process did not exit; force kill.
	return exec.Command("taskkill", "/PID", pidStr, "/T", "/F").Run()
}

// KillProcessForce forcefully terminates a process by PID.
// On Unix, it sends SIGKILL; on Windows, it uses taskkill /F.
func KillProcessForce(pid int) error {
	if pid <= 0 {
		return nil
	}
	pidStr := strconv.Itoa(pid)
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", pidStr, "/T", "/F").Run()
	}
	return exec.Command("kill", "-KILL", pidStr).Run()
}
