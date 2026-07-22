package platform

import (
	"os/exec"
	"runtime"
	"strconv"
)

// KillProcess terminates a process by PID.
// On Unix, it sends SIGTERM; on Windows, it uses taskkill.
func KillProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	pidStr := strconv.Itoa(pid)
	if runtime.GOOS == "windows" {
		// Use taskkill to terminate the process tree gracefully
		return exec.Command("taskkill", "/PID", pidStr, "/T", "/F").Run()
	}
	return exec.Command("kill", "-TERM", pidStr).Run()
}

// KillProcessForce forcefully terminates a process by PID.
// On Unix, it sends SIGKILL; on Windows, same as KillProcess (taskkill /F).
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
