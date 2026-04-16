package agents

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var knownProviders = map[Provider]string{
	ProviderOpenCode: "opencode",
	ProviderClaude:   "claude",
	ProviderKimi:     "kimi",
	ProviderCodex:    "codex",
}

func DiscoverRunningAgents() []Session {
	var sessions []Session
	for provider, binName := range knownProviders {
		pids := findPIDs(binName)
		for _, pid := range pids {
			cwd := getProcessCWD(pid)
			if cwd == "" {
				continue
			}
			cwd = filepath.Clean(cwd)
			sessions = append(sessions, Session{
				ID:         fmt.Sprintf("%s-%d", provider, pid),
				Provider:   provider,
				WorktreeID: cwd,
				PID:        pid,
				State:      SessionRunning,
				StartedAt:  time.Now(),
			})
		}
	}
	return sessions
}

func findPIDs(name string) []int {
	out, err := exec.Command("pgrep", "-f", "^"+name+"($| )").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pid, err := strconv.Atoi(line); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func getProcessCWD(pid int) string {
	if cwd, err := filepath.EvalSymlinks(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
		return cwd
	}
	out, err := exec.Command("lsof", "-p", strconv.Itoa(pid), "-a", "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(line, "n")
		}
	}
	return ""
}
