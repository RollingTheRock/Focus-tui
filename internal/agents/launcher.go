package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/RollingTheRock/Focus-tui/internal/platform"
)

func DefaultProvider() Provider {
	return ProviderOpenCode
}

// binaryPathCacheTTL bounds how long a cached PATH lookup is trusted.
// Binary install state changes rarely, so a short TTL turns repeated scans
// into cheap map lookups while still picking up newly installed agents.
const binaryPathCacheTTL = 2 * time.Minute

type binaryPathCacheEntry struct {
	path  string    // resolved path, "" when not found
	found bool      // whether the binary was found
	at    time.Time // when the lookup was performed
}

// binaryPathCache caches the results of exec.LookPath to avoid repeated
// expensive lookups, especially on Windows where each lookup is slow.
var (
	binaryPathCache   = make(map[string]binaryPathCacheEntry)
	binaryPathCacheMu sync.RWMutex
)

// lookupBinary finds a binary in PATH, using cache to avoid repeated lookups.
// Negative results (binary not found) are also cached so absence is cheap.
func lookupBinary(name string) string {
	// Fast path: return a recent cached result if present.
	binaryPathCacheMu.RLock()
	if e, ok := binaryPathCache[name]; ok && time.Since(e.at) < binaryPathCacheTTL {
		binaryPathCacheMu.RUnlock()
		if e.found {
			return e.path
		}
		return ""
	}
	binaryPathCacheMu.RUnlock()

	binaryPathCacheMu.Lock()
	defer binaryPathCacheMu.Unlock()
	// Double-check after acquiring write lock.
	if e, ok := binaryPathCache[name]; ok && time.Since(e.at) < binaryPathCacheTTL {
		if e.found {
			return e.path
		}
		return ""
	}

	path, err := exec.LookPath(name)
	found := err == nil
	binaryPathCache[name] = binaryPathCacheEntry{path: path, found: found, at: time.Now()}
	if found {
		return path
	}
	return ""
}

func ProviderCommand(provider Provider) (string, []string) {
	switch provider {
	case ProviderOpenCode:
		return "opencode", nil
	case ProviderClaude:
		return "claude", nil
	case ProviderKimi:
		return "kimi", nil
	case ProviderCodex:
		return "codex", nil
	case ProviderGemini:
		return "gemini", nil
	default:
		return "", nil
	}
}

func IsInstalled(provider Provider) bool {
	bin, _ := ProviderCommand(provider)
	if bin == "" {
		return false
	}
	return lookupBinary(bin) != ""
}

func LaunchCommand(provider Provider, worktreeID string) tea.Cmd {
	bin, args := ProviderCommand(provider)
	if lookupBinary(bin) == "" {
		return nil
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = worktreeID
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return AgentExitedMsg{Provider: provider, WorktreeID: worktreeID, Err: err}
	})
}

// HasResumableClaudeSession checks whether the given worktree directory has at
// least one prior Claude Code session that can be resumed.
func HasResumableClaudeSession(worktreePath string) bool {
	if worktreePath == "" {
		return false
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	projectHash := strings.ReplaceAll(filepath.Clean(worktreePath), string(filepath.Separator), "-")
	projectDir := filepath.Join(homeDir, ".claude", "projects", projectHash)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			return true
		}
	}
	return false
}

type AgentExitedMsg struct {
	Provider   Provider
	WorktreeID string
	Err        error
}

func AutoTypeCommand(provider Provider) string {
	bin, args := ProviderCommand(provider)
	if bin == "" {
		return ""
	}
	cmd := bin
	if len(args) > 0 {
		cmd += " " + strings.Join(args, " ")
	}
	return cmd
}

func AutoTypeCommandWithSession(provider Provider, sessionID string) string {
	cmd := AutoTypeCommand(provider)
	if cmd == "" || sessionID == "" {
		return cmd
	}
	return strings.Join([]string{
		SessionIDEnvVar + "=" + sessionID,
		LegacySessionIDEnvVar + "=" + sessionID,
		TrellisContextIDEnvVar + "=" + sessionID,
		cmd,
	}, " ")
}

// DetectTerminalEmulator inspects the runtime environment to guess which
// terminal emulator is currently in use.
func DetectTerminalEmulator() string {
	// Environment-variable based detection (fast, reliable).
	if os.Getenv("PTYXIS_VERSION") != "" {
		return "ptyxis"
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return "kitty"
	}
	if os.Getenv("WEZTERM_EXECUTABLE") != "" {
		return "wezterm"
	}
	if os.Getenv("GNOME_TERMINAL_SCREEN") != "" {
		return "gnome-terminal"
	}
	if os.Getenv("ALACRITTY_SOCKET") != "" || os.Getenv("ALACRITTY_LOG") != "" {
		return "alacritty"
	}
	// Windows Terminal detection
	if os.Getenv("WT_SESSION") != "" {
		return "wt"
	}

	// Fallback: check PATH for known binaries.
	for _, name := range []string{"ptyxis", "kitty", "alacritty", "wezterm", "gnome-terminal"} {
		if lookupBinary(name) != "" {
			return name
		}
	}
	// Check for Windows Terminal
	if lookupBinary("wt.exe") != "" {
		return "wt"
	}
	return ""
}

// powershellQuote returns s wrapped in single quotes, with embedded single
// quotes escaped by doubling them. This makes s safe to embed in a PowerShell
// single-quoted string literal.
func powershellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// buildWindowsTerminalPowerShellScript builds a PowerShell script that sets
// environment variables, changes directory, and invokes the agent binary with
// arguments. All user-controlled values are single-quoted and escaped so the
// resulting script can safely be passed via -EncodedCommand.
func buildWindowsTerminalPowerShellScript(directory, bin string, envVars, providerArgs, extraArgs []string) string {
	var b strings.Builder

	// Set environment variables at the process level.
	for _, v := range envVars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			continue
		}
		b.WriteString("[Environment]::SetEnvironmentVariable(")
		b.WriteString(powershellQuote(parts[0]))
		b.WriteString(", ")
		b.WriteString(powershellQuote(parts[1]))
		b.WriteString(", 'Process'); ")
	}

	// Change to the worktree directory.
	b.WriteString("Set-Location ")
	b.WriteString(powershellQuote(directory))
	b.WriteString("; ")

	// Invoke the agent binary with arguments.
	b.WriteString("& ")
	b.WriteString(powershellQuote(bin))
	for _, arg := range append(providerArgs, extraArgs...) {
		b.WriteString(" ")
		b.WriteString(powershellQuote(arg))
	}

	return b.String()
}

func BuildExternalTerminalCommand(emulator, title, directory string, envVars []string, bin string, providerArgs []string, extraArgs []string, zoom float64) (string, []string) {
	if emulator == "" {
		emulator = DetectTerminalEmulator()
		if emulator == "" {
			if runtime.GOOS == "windows" {
				emulator = "wt"
			} else {
				emulator = "kitty"
			}
		}
	}

	// Inject zoom scale into environment for terminals that respect it.
	if zoom != 1.0 && zoom > 0 {
		envVars = append([]string{"FOCUS_TERMINAL_ZOOM=" + fmt.Sprintf("%.2f", zoom)}, envVars...)
	}

	// Windows Terminal (wt.exe) - Windows-specific handling
	if emulator == "wt" && runtime.GOOS == "windows" {
		// Build a PowerShell script that sets environment variables, changes
		// directory, and invokes the agent binary. All user-controlled values are
		// single-quoted and escaped, then the script is passed via
		// -EncodedCommand so no shell metacharacter parsing occurs.
		script := buildWindowsTerminalPowerShellScript(directory, bin, envVars, providerArgs, extraArgs)
		encoded, err := platform.EncodePowerShellCommand(script)
		if err != nil {
			// Encoding should never fail for valid strings; fall back to plain
			// -Command, which is still safe because the script itself contains
			// only quoted literals.
			args := []string{"new-tab", "--title", title, "--startingDirectory", directory, "powershell", "-NoProfile", "-NoExit", "-Command", script}
			return "wt.exe", args
		}
		args := []string{"new-tab", "--title", title, "--startingDirectory", directory, "powershell", "-NoProfile", "-EncodedCommand", encoded}
		return "wt.exe", args
	}

	switch emulator {
	case "kitty":
		args := []string{"--title", title, "--directory", directory, "env"}
		args = append(args, envVars...)
		args = append(args, bin)
		args = append(args, providerArgs...)
		args = append(args, extraArgs...)
		return "kitty", args
	case "alacritty":
		cmdArgs := append([]string{bin}, providerArgs...)
		cmdArgs = append(cmdArgs, extraArgs...)
		cmd := strings.Join(cmdArgs, " ")
		args := []string{"--title", title, "--working-directory", directory, "-e", "env"}
		args = append(args, envVars...)
		args = append(args, "sh", "-lc", cmd)
		return "alacritty", args
	case "wezterm":
		args := []string{"cli", "spawn", "--cwd", directory, "--", "env"}
		args = append(args, envVars...)
		args = append(args, bin)
		args = append(args, providerArgs...)
		args = append(args, extraArgs...)
		return "wezterm", args
	case "gnome-terminal":
		cmdArgs := append([]string{bin}, providerArgs...)
		cmdArgs = append(cmdArgs, extraArgs...)
		cmd := strings.Join(cmdArgs, " ")
		args := []string{"--window", "--title", title, "--working-directory", directory}
		if zoom != 1.0 && zoom > 0 {
			args = append(args, "--zoom", fmt.Sprintf("%.2f", zoom))
		}
		args = append(args, "--", "env")
		args = append(args, envVars...)
		args = append(args, "sh", "-lc", cmd)
		return "gnome-terminal", args
	case "ptyxis":
		cmdArgs := append([]string{bin}, providerArgs...)
		cmdArgs = append(cmdArgs, extraArgs...)
		cmd := strings.Join(cmdArgs, " ")
		// Use bash -lc so that ~/.bashrc is sourced and PATH is complete.
		// ptyxis spawns commands directly without a shell, so agent binaries
		// installed via user package managers (homebrew, nvm, etc.) would not
		// be found unless we explicitly launch through bash.
		exports := make([]string, len(envVars))
		for i, v := range envVars {
			exports[i] = "export " + v
		}
		inner := fmt.Sprintf("bash -lc '%s; cd %s; exec %s'",
			strings.Join(exports, "; "),
			directory,
			cmd,
		)
		args := []string{"--new-window", "-d", directory, "-x", inner}
		if title != "" {
			args = append([]string{"-T", title}, args...)
		}
		return "ptyxis", args
	default:
		// Fallback to shell execution so unknown terminal wrappers can still be attempted.
		args := []string{"-lc", strings.Join(append(append([]string{bin}, providerArgs...), extraArgs...), " ")}
		return emulator, args
	}
}

type LaunchAgentMsg struct {
	WorktreeID string
	Provider   Provider
}

type FocusAgentMsg struct {
	WorktreeID string
	Provider   Provider
}

type ExternalLaunchRequest struct {
	SessionID        string
	Title            string
	WorktreeID       string
	Provider         Provider
	TerminalEmulator string
	EnvVars          []string
	ExtraArgs        []string
	// OverrideBinary and OverrideArgs allow the caller to replace the default
	// provider binary (e.g. "claude") with a custom command (e.g.
	// "cc-switch start claude <provider>").  When OverrideBinary is empty the
	// default provider binary is used.
	OverrideBinary string
	OverrideArgs   []string
}

type ExternalLaunchResultMsg struct {
	SessionID  string
	WorktreeID string
	Provider   Provider
	PID        int
	Err        error
}

func LaunchExternalCommand(req ExternalLaunchRequest) tea.Cmd {
	return func() tea.Msg {
		if req.SessionID == "" {
			return ExternalLaunchResultMsg{Err: fmt.Errorf("session id required")}
		}
		if req.WorktreeID == "" {
			return ExternalLaunchResultMsg{
				SessionID: req.SessionID,
				Provider:  req.Provider,
				Err:       fmt.Errorf("worktree id required"),
			}
		}
		bin, providerArgs := ProviderCommand(req.Provider)
		if req.OverrideBinary != "" {
			bin = req.OverrideBinary
			providerArgs = req.OverrideArgs
		}
		name, args := BuildExternalTerminalCommand(
			req.TerminalEmulator,
			req.Title,
			req.WorktreeID,
			req.EnvVars,
			bin,
			providerArgs,
			req.ExtraArgs,
			1.2,
		)
		cmd := exec.Command(name, args...)
		f, _ := os.OpenFile(filepath.Join(os.TempDir(), "focus_launch.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		f.WriteString(fmt.Sprintf("Launch: %s %v\n", name, args))
		defer f.Close()
		if err := cmd.Start(); err != nil {
			return ExternalLaunchResultMsg{
				SessionID:  req.SessionID,
				WorktreeID: req.WorktreeID,
				Provider:   req.Provider,
				Err:        err,
			}
		}
		return ExternalLaunchResultMsg{
			SessionID:  req.SessionID,
			WorktreeID: req.WorktreeID,
			Provider:   req.Provider,
			PID:        cmd.Process.Pid,
		}
	}
}

// BuildExternalShellCommand builds a command to open an interactive shell in an
// external terminal emulator for the given directory. It reuses the emulator
// detection logic but launches $SHELL instead of an agent provider.
func BuildExternalShellCommand(emulator, title, directory string, zoom float64) (string, []string) {
	if emulator == "" {
		emulator = DetectTerminalEmulator()
		if emulator == "" {
			emulator = "kitty"
		}
	}

	shell := platform.DefaultShell()

	switch emulator {
	case "kitty":
		return "kitty", []string{"--title", title, "--directory", directory, shell}
	case "alacritty":
		return "alacritty", []string{"--title", title, "--working-directory", directory, "-e", shell}
	case "wezterm":
		return "wezterm", []string{"cli", "spawn", "--cwd", directory, "--", shell}
	case "gnome-terminal":
		args := []string{"--window", "--title", title, "--working-directory", directory}
		if zoom != 1.0 && zoom > 0 {
			args = append(args, "--zoom", fmt.Sprintf("%.2f", zoom))
		}
		args = append(args, "--", shell)
		return "gnome-terminal", args
	case "ptyxis":
		args := []string{"--new-window", "-d", directory, "-x", shell}
		if title != "" {
			args = append([]string{"-T", title}, args...)
		}
		return "ptyxis", args
	default:
		// Fallback: attempt to open a shell through the unknown terminal wrapper.
		return emulator, []string{"-e", shell}
	}
}

type ExternalShellLaunchedMsg struct {
	PID int
	CWD string
	Err error
}

// LaunchExternalShell opens an external terminal with an interactive shell at
// the given working directory. The caller should handle ExternalShellLaunchedMsg
// to report success or failure.
func LaunchExternalShell(cwd, title, terminalEmulator string, zoom float64) tea.Cmd {
	return func() tea.Msg {
		if cwd == "" {
			return ExternalShellLaunchedMsg{Err: fmt.Errorf("cwd required")}
		}
		name, args := BuildExternalShellCommand(terminalEmulator, title, cwd, zoom)
		cmd := exec.Command(name, args...)
		if err := cmd.Start(); err != nil {
			return ExternalShellLaunchedMsg{CWD: cwd, Err: err}
		}
		return ExternalShellLaunchedMsg{PID: cmd.Process.Pid, CWD: cwd}
	}
}
