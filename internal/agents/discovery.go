package agents

import (
	"fmt"
	"os"
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
	ProviderGemini:   "gemini",
}

func DiscoverRunningAgents() []Session {
	sessions := make([]Session, 0)
	for provider, binName := range knownProviders {
		pids := findPIDs(binName)
		for _, pid := range pids {
			cwd := getProcessCWD(pid)
			if cwd == "" {
				continue
			}
			cwd = filepath.Clean(cwd)
			sessionID := getProcessEnvValue(pid, SessionIDEnvVar)
			if sessionID == "" {
				sessionID = getProcessEnvValue(pid, LegacySessionIDEnvVar)
			}
			if sessionID == "" {
				sessionID = fmt.Sprintf("%s-%d", provider, pid)
			}
			sessions = append(sessions, Session{
				ID:         sessionID,
				Provider:   provider,
				WorktreeID: cwd,
				PID:        pid,
				State:      SessionRunning,
				StartedAt:  time.Now(),
				UpdatedAt:  time.Now(),
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

func getProcessEnvValue(pid int, key string) string {
	if key == "" {
		return ""
	}
	path := fmt.Sprintf("/proc/%d/environ", pid)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, part := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}
