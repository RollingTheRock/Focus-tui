package agents

import (
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(r.All()) != 0 {
		t.Fatalf("expected empty registry, got %d", len(r.All()))
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	s := &Session{ID: "s1", Provider: ProviderOpenCode, WorktreeID: "/tmp/wt1"}
	r.Register(s)

	all := r.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 session, got %d", len(all))
	}
	if all[0].ID != "s1" {
		t.Errorf("expected ID s1, got %s", all[0].ID)
	}

	r.Register(nil)
	if len(r.All()) != 1 {
		t.Errorf("expected still 1 session after nil register, got %d", len(r.All()))
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	r.Register(&Session{ID: "s1", Provider: ProviderOpenCode, WorktreeID: "/tmp/wt1"})
	r.Register(&Session{ID: "s2", Provider: ProviderClaude, WorktreeID: "/tmp/wt2"})

	r.Remove("s1")
	if len(r.All()) != 1 {
		t.Fatalf("expected 1 session after remove, got %d", len(r.All()))
	}
	if r.All()[0].ID != "s2" {
		t.Errorf("expected remaining s2, got %s", r.All()[0].ID)
	}
}

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()
	r.Register(&Session{ID: "s1", Provider: ProviderOpenCode, WorktreeID: "/tmp/wt1"})
	r.Clear()
	if len(r.All()) != 0 {
		t.Fatalf("expected empty registry after clear, got %d", len(r.All()))
	}
}

func TestRegistryByWorktree(t *testing.T) {
	r := NewRegistry()
	r.Register(&Session{ID: "s1", Provider: ProviderOpenCode, WorktreeID: "/tmp/wt1"})
	r.Register(&Session{ID: "s2", Provider: ProviderClaude, WorktreeID: "/tmp/wt1"})
	r.Register(&Session{ID: "s3", Provider: ProviderKimi, WorktreeID: "/tmp/wt2"})

	wt1 := r.ByWorktree("/tmp/wt1")
	if len(wt1) != 2 {
		t.Fatalf("expected 2 sessions for wt1, got %d", len(wt1))
	}

	wt2 := r.ByWorktree("/tmp/wt2")
	if len(wt2) != 1 {
		t.Fatalf("expected 1 session for wt2, got %d", len(wt2))
	}

	wt3 := r.ByWorktree("/tmp/wt3")
	if len(wt3) != 0 {
		t.Fatalf("expected 0 sessions for wt3, got %d", len(wt3))
	}
}

func TestRegistryHasRunning(t *testing.T) {
	r := NewRegistry()
	r.Register(&Session{ID: "s1", Provider: ProviderOpenCode, WorktreeID: "/tmp/wt1", State: SessionRunning})
	r.Register(&Session{ID: "s2", Provider: ProviderOpenCode, WorktreeID: "/tmp/wt1", State: SessionExited})
	r.Register(&Session{ID: "s3", Provider: ProviderClaude, WorktreeID: "/tmp/wt1", State: SessionRunning})

	if !r.HasRunning("/tmp/wt1", ProviderOpenCode) {
		t.Error("expected HasRunning true for opencode on wt1")
	}
	if r.HasRunning("/tmp/wt1", ProviderKimi) {
		t.Error("expected HasRunning false for kimi on wt1")
	}
	if !r.HasRunning("/tmp/wt1", ProviderClaude) {
		t.Error("expected HasRunning true for claude on wt1")
	}
	if r.HasRunning("/tmp/wt2", ProviderOpenCode) {
		t.Error("expected HasRunning false for opencode on wt2")
	}
}

func TestSessionDisplayName(t *testing.T) {
	tests := []struct {
		provider Provider
		want     string
	}{
		{ProviderOpenCode, "opencode"},
		{ProviderClaude, "claude"},
		{ProviderKimi, "kimi"},
		{ProviderCodex, "codex"},
		{ProviderGeneric, "agent"},
		{Provider("unknown"), "agent"},
	}
	for _, tt := range tests {
		s := Session{Provider: tt.provider}
		if got := s.DisplayName(); got != tt.want {
			t.Errorf("DisplayName() for %s = %q, want %q", tt.provider, got, tt.want)
		}
	}
}

func TestProviderCommand(t *testing.T) {
	tests := []struct {
		provider Provider
		wantBin  string
	}{
		{ProviderOpenCode, "opencode"},
		{ProviderClaude, "claude"},
		{ProviderKimi, "kimi"},
		{ProviderCodex, "codex"},
		{ProviderGeneric, ""},
		{Provider("custom"), ""},
	}
	for _, tt := range tests {
		bin, args := ProviderCommand(tt.provider)
		if bin != tt.wantBin {
			t.Errorf("ProviderCommand(%s) bin = %q, want %q", tt.provider, bin, tt.wantBin)
		}
		if args != nil {
			t.Errorf("ProviderCommand(%s) args expected nil, got %v", tt.provider, args)
		}
	}
}

func TestAutoTypeCommand(t *testing.T) {
	if got := AutoTypeCommand(ProviderOpenCode); got != "opencode" {
		t.Errorf("AutoTypeCommand(opencode) = %q, want opencode", got)
	}
	if got := AutoTypeCommand(Provider("")); got != "" {
		t.Errorf("AutoTypeCommand(empty) = %q, want empty", got)
	}
}

func TestDefaultProvider(t *testing.T) {
	if got := DefaultProvider(); got != ProviderOpenCode {
		t.Errorf("DefaultProvider() = %q, want %q", got, ProviderOpenCode)
	}
}

func TestDiscoverRunningAgents(t *testing.T) {
	// This test just ensures the function doesn't panic and returns a slice.
	// Actual discovery depends on the host environment.
	sessions := DiscoverRunningAgents()
	if sessions == nil {
		t.Fatal("expected non-nil slice from DiscoverRunningAgents")
	}
}

func TestFindPIDs(t *testing.T) {
	// Searching for a process that should never exist.
	pids := findPIDs("this-binary-definitely-does-not-exist-12345")
	if pids == nil {
		// nil is acceptable when pgrep errors
		return
	}
	if len(pids) != 0 {
		t.Fatalf("expected no PIDs for fake binary, got %v", pids)
	}
}

func TestGetProcessCWD(t *testing.T) {
	// PID 1 should exist on Unix systems; we just check it doesn't panic.
	cwd := getProcessCWD(1)
	_ = cwd // may be empty or a path depending on OS/permissions
}

func TestLaunchCommand(t *testing.T) {
	// Use a provider that is unlikely to be installed.
	cmd := LaunchCommand(Provider("nonexistent-binary-xyz"), "/tmp")
	if cmd != nil {
		t.Error("expected nil command for nonexistent binary")
	}
}

func TestLaunchAgentMsg(t *testing.T) {
	msg := LaunchAgentMsg{WorktreeID: "/tmp/wt", Provider: ProviderOpenCode}
	if msg.WorktreeID != "/tmp/wt" {
		t.Errorf("WorktreeID = %q, want /tmp/wt", msg.WorktreeID)
	}
	if msg.Provider != ProviderOpenCode {
		t.Errorf("Provider = %q, want opencode", msg.Provider)
	}
}

func TestFocusAgentMsg(t *testing.T) {
	msg := FocusAgentMsg{WorktreeID: "/tmp/wt", Provider: ProviderClaude}
	if msg.WorktreeID != "/tmp/wt" {
		t.Errorf("WorktreeID = %q, want /tmp/wt", msg.WorktreeID)
	}
	if msg.Provider != ProviderClaude {
		t.Errorf("Provider = %q, want claude", msg.Provider)
	}
}

func TestAgentExitedMsg(t *testing.T) {
	msg := AgentExitedMsg{Provider: ProviderKimi, WorktreeID: "/tmp/wt", Err: nil}
	if msg.Provider != ProviderKimi {
		t.Errorf("Provider = %q, want kimi", msg.Provider)
	}
	if msg.WorktreeID != "/tmp/wt" {
		t.Errorf("WorktreeID = %q, want /tmp/wt", msg.WorktreeID)
	}
}

func TestSessionStruct(t *testing.T) {
	s := Session{
		ID:         "test-id",
		Provider:   ProviderCodex,
		WorktreeID: "/tmp/wt",
		PID:        1234,
		State:      SessionRunning,
		StartedAt:  time.Now(),
	}
	if s.ID != "test-id" {
		t.Errorf("ID = %q, want test-id", s.ID)
	}
	if s.Provider != ProviderCodex {
		t.Errorf("Provider = %q, want codex", s.Provider)
	}
	if s.WorktreeID != "/tmp/wt" {
		t.Errorf("WorktreeID = %q, want /tmp/wt", s.WorktreeID)
	}
	if s.PID != 1234 {
		t.Errorf("PID = %d, want 1234", s.PID)
	}
	if s.State != SessionRunning {
		t.Errorf("State = %q, want running", s.State)
	}
}
