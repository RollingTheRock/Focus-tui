package editor

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEditorCommand(t *testing.T) {
	t.Run("configured takes priority", func(t *testing.T) {
		got := ResolveEditorCommand("vim")
		if got != "vim" {
			t.Fatalf("expected configured 'vim', got %q", got)
		}
	})

	t.Run("defaults to nvim", func(t *testing.T) {
		got := ResolveEditorCommand("")
		if got != "nvim" {
			t.Fatalf("expected default 'nvim', got %q", got)
		}
	})
}

func TestBuildEditorCmd(t *testing.T) {
	t.Run("vim-like with line number", func(t *testing.T) {
		cmd, err := BuildEditorCmd("nvim", "/tmp/main.go", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(cmd.Path) != "nvim" {
			t.Fatalf("expected nvim, got %q", cmd.Path)
		}
		want := []string{"nvim", "+10", "/tmp/main.go"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("expected args %v, got %v", want, cmd.Args)
		}
	})

	t.Run("vim-like without line number", func(t *testing.T) {
		cmd, err := BuildEditorCmd("vim", "/tmp/main.go", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"vim", "/tmp/main.go"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("expected args %v, got %v", want, cmd.Args)
		}
	})

	t.Run("preserves extra args", func(t *testing.T) {
		cmd, err := BuildEditorCmd("code --wait", "/tmp/main.go", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"code", "--wait", "/tmp/main.go"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("expected args %v, got %v", want, cmd.Args)
		}
	})

	t.Run("non-vim editor ignores line number", func(t *testing.T) {
		cmd, err := BuildEditorCmd("emacs", "/tmp/main.go", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"emacs", "/tmp/main.go"}
		if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Fatalf("expected args %v, got %v", want, cmd.Args)
		}
	})

	t.Run("empty command errors", func(t *testing.T) {
		_, err := BuildEditorCmd("", "/tmp/main.go", 0)
		if err == nil {
			t.Fatalf("expected error for empty command")
		}
	})
}

func TestBuildEditorCmdUsesAbsolutePath(t *testing.T) {
	// The cmd.Path should resolve to an absolute path if LookPath succeeds,
	// but we only assert that the file argument is passed through unchanged.
	cmd, err := BuildEditorCmd("nvim", "main.go", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lastArg := cmd.Args[len(cmd.Args)-1]
	if lastArg != "main.go" {
		t.Fatalf("expected last arg 'main.go', got %q", lastArg)
	}
}

func TestLaunchExternalEditorNilForEmptyCommand(t *testing.T) {
	cmd := LaunchExternalEditor("", "/tmp/main.go", 0)
	msg := cmd()
	exitMsg, ok := msg.(ExternalEditorExitedMsg)
	if !ok {
		t.Fatalf("expected ExternalEditorExitedMsg, got %T", msg)
	}
	if exitMsg.Err == nil {
		t.Fatalf("expected error for empty command")
	}
}

func TestIsVimLike(t *testing.T) {
	for _, name := range []string{"vi", "vim", "nvim", "gvim", "mvim"} {
		if !isVimLike(name) {
			t.Fatalf("expected %q to be vim-like", name)
		}
	}
	for _, name := range []string{"emacs", "code", "nano", "subl"} {
		if isVimLike(name) {
			t.Fatalf("expected %q not to be vim-like", name)
		}
	}
}
