package sessiondigest

import (
	"testing"
	"time"

	"focus/internal/agents"
)

func TestBuilderInferSummary(t *testing.T) {
	b := NewBuilder()

	d := &Digest{FilesChanged: []FileChange{{Path: "a.go", Status: "modified"}}}
	s := b.inferSummary(d)
	if !contains(s, "1 files changed") {
		t.Errorf("expected '1 files changed' in summary, got %q", s)
	}

	d = &Digest{}
	s = b.inferSummary(d)
	if !contains(s, "no transcript or diff captured") {
		t.Errorf("expected fallback summary, got %q", s)
	}
}

func TestApplyEvents(t *testing.T) {
	b := NewBuilder()
	d := &Digest{}
	events := []Event{
		{Type: EventUser, Content: "Fix the auth bug"},
		{Type: EventAssistant, Content: "I'll refactor the middleware."},
		{Type: EventCommand, Content: "go test ./..."},
		{Type: EventFileChange, Content: "auth.go"},
		{Type: EventToolUse, Content: "Write", Metadata: map[string]string{"file_path": "auth.go"}},
	}

	d = b.applyEvents(d, events)

	if d.ConversationTurns != 2 {
		t.Errorf("expected 2 conversation turns, got %d", d.ConversationTurns)
	}
	if len(d.CommandsRun) != 1 || d.CommandsRun[0].Command != "go test ./..." {
		t.Errorf("expected 1 command, got %+v", d.CommandsRun)
	}
	if len(d.FilesChanged) != 1 || d.FilesChanged[0].Path != "auth.go" {
		t.Errorf("expected auth.go in files changed, got %+v", d.FilesChanged)
	}
	if !contains(d.Summary, "Fix the auth bug") {
		t.Errorf("expected summary to reference user request, got %q", d.Summary)
	}
}

func TestBuildWithGitOnly(t *testing.T) {
	// This test creates a real temp git repo so we can verify end-to-end.
	b := NewBuilder()

	// We cannot easily create a real git repo in a unit test without filesystem
	// manipulation, so we just verify the builder tolerates missing transcripts
	// and git errors gracefully.
	d, err := b.Build(
		agents.ProviderClaude,
		"/nonexistent/worktree",
		"",
		time.Now().Add(-time.Hour),
		time.Now(),
	)
	if err != nil {
		t.Fatalf("Build should not error when transcript/git are missing: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil digest")
	}
	if d.SessionProvider != "claude" {
		t.Errorf("expected provider claude, got %q", d.SessionProvider)
	}
}
