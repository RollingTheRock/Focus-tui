package agents

import (
	"strings"
	"testing"
)

func TestAutoTypeCommandWithSessionInjectsNewAndLegacyEnv(t *testing.T) {
	cmd := AutoTypeCommandWithSession(ProviderOpenCode, "session-123")
	if !strings.Contains(cmd, SessionIDEnvVar+"=session-123") {
		t.Fatalf("expected new session env var, got %q", cmd)
	}
	if !strings.Contains(cmd, LegacySessionIDEnvVar+"=session-123") {
		t.Fatalf("expected legacy session env var for compatibility, got %q", cmd)
	}
}

func TestBuildExternalTerminalCommand(t *testing.T) {
	name, args := BuildExternalTerminalCommand(
		"kitty",
		"Focus:plan-1:task-1",
		"/tmp/worktree-a",
		[]string{"FOCUS_SESSION_ID=session-1", "FOCUS_MCP_SOCKET=/tmp/focus-mcp.sock"},
		"claude",
		nil,
		nil,
		1.0,
	)

	if name != "kitty" {
		t.Fatalf("expected kitty launcher, got %q", name)
	}
	joined := strings.Join(args, " ")
	for _, token := range []string{"--title", "Focus:plan-1:task-1", "--directory", "/tmp/worktree-a", "env", "FOCUS_SESSION_ID=session-1", "claude"} {
		if !strings.Contains(joined, token) {
			t.Fatalf("expected %q in args %q", token, joined)
		}
	}
}

func TestLaunchExternalCommandValidatesSessionID(t *testing.T) {
	cmd := LaunchExternalCommand(ExternalLaunchRequest{
		SessionID:  "",
		WorktreeID: "/tmp/worktree-a",
		Provider:   ProviderClaude,
	})
	msg := cmd().(ExternalLaunchResultMsg)
	if msg.Err == nil {
		t.Fatal("expected validation error when session id missing")
	}
}
