package agents

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func DefaultProvider() Provider {
	return ProviderOpenCode
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
	default:
		return string(provider), nil
	}
}

func IsInstalled(provider Provider) bool {
	bin, _ := ProviderCommand(provider)
	_, err := exec.LookPath(bin)
	return err == nil
}

func LaunchCommand(provider Provider, worktreeID string) tea.Cmd {
	bin, args := ProviderCommand(provider)
	if _, err := exec.LookPath(bin); err != nil {
		return nil
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = worktreeID
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return AgentExitedMsg{Provider: provider, WorktreeID: worktreeID, Err: err}
	})
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

	// Fallback: check PATH for known binaries.
	for _, name := range []string{"ptyxis", "kitty", "alacritty", "wezterm", "gnome-terminal"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func BuildExternalTerminalCommand(emulator, title, directory string, envVars []string, provider Provider, extraArgs []string, zoom float64) (string, []string) {
	bin, providerArgs := ProviderCommand(provider)
	if emulator == "" {
		emulator = DetectTerminalEmulator()
		if emulator == "" {
			emulator = "kitty"
		}
	}

	// Inject zoom scale into environment for terminals that respect it.
	if zoom != 1.0 && zoom > 0 {
		envVars = append([]string{"FOCUS_TERMINAL_ZOOM=" + fmt.Sprintf("%.2f", zoom)}, envVars...)
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
		name, args := BuildExternalTerminalCommand(
			req.TerminalEmulator,
			req.Title,
			req.WorktreeID,
			req.EnvVars,
			req.Provider,
			req.ExtraArgs,
			1.2,
		)
		cmd := exec.Command(name, args...)
			f, _ := os.OpenFile("/tmp/focus_launch.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
