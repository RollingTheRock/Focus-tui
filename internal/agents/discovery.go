package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		return discoverRunningAgentsWindows()
	}
	return discoverRunningAgentsUnix()
}

func discoverRunningAgentsWindows() []Session {
	sessions := make([]Session, 0)

	// Single tasklist call for all providers - much more efficient
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return sessions
	}

	// Build reverse map: binary name -> provider
	binToProvider := make(map[string]Provider, len(knownProviders))
	for provider, binName := range knownProviders {
		binToProvider[strings.ToLower(binName+".exe")] = provider
	}

	// Parse all lines once
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "INFO:") || strings.HasPrefix(line, "信息:") {
			continue
		}

		// CSV format: "name.exe","PID","Session","Session#","Mem"
		parts := strings.Split(line, "\",\"")
		if len(parts) < 2 {
			continue
		}

		imageName := strings.ToLower(strings.Trim(parts[0], "\""))
		provider, ok := binToProvider[imageName]
		if !ok {
			continue
		}

		pidStr := strings.Trim(parts[1], "\"")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		sessionID := fmt.Sprintf("%s-%d", provider, pid)
		sessions = append(sessions, Session{
			ID:         sessionID,
			Provider:   provider,
			WorktreeID: "", // Windows can't get CWD without Win32 API
			PID:        pid,
			State:      SessionRunning,
			StartedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
	}

	return sessions
}

func discoverRunningAgentsUnix() []Session {
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
