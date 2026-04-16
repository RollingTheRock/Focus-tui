package agents

import (
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
	return SessionIDEnvVar + "=" + sessionID + " " + cmd
}

type LaunchAgentMsg struct {
	WorktreeID string
	Provider   Provider
}

type FocusAgentMsg struct {
	WorktreeID string
	Provider   Provider
}
