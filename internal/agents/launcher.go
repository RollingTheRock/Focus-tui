package agents

import (
	"fmt"
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

func BuildExternalTerminalCommand(emulator, title, directory string, envVars []string, provider Provider) (string, []string) {
	bin, providerArgs := ProviderCommand(provider)
	if emulator == "" {
		emulator = "kitty"
	}

	switch emulator {
	case "kitty":
		args := []string{"--title", title, "--directory", directory, "env"}
		args = append(args, envVars...)
		args = append(args, bin)
		args = append(args, providerArgs...)
		return "kitty", args
	case "alacritty":
		cmdArgs := append([]string{bin}, providerArgs...)
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
		return "wezterm", args
	case "gnome-terminal":
		cmdArgs := append([]string{bin}, providerArgs...)
		cmd := strings.Join(cmdArgs, " ")
		args := []string{"--title", title, "--working-directory", directory, "--", "env"}
		args = append(args, envVars...)
		args = append(args, "sh", "-lc", cmd)
		return "gnome-terminal", args
	default:
		// Fallback to shell execution so unknown terminal wrappers can still be attempted.
		args := []string{"-lc", strings.Join(append([]string{bin}, providerArgs...), " ")}
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
		)
		cmd := exec.Command(name, args...)
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
