package editor

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ExternalEditorExitedMsg is sent when the external editor process finishes.
type ExternalEditorExitedMsg struct {
	FilePath string
	Err      error
}

// ResolveEditorCommand returns the editor command to use.
// If a command is explicitly configured, it is used; otherwise defaults to "nvim".
func ResolveEditorCommand(configured string) string {
	if configured != "" {
		return configured
	}
	return "nvim"
}

// BuildEditorCmd constructs an exec.Cmd for opening filePath in the given editor.
// Line number jumping is supported for vim-like editors (vi, vim, nvim, gvim, mvim)
// using the +line convention. Other editors will open the file without line positioning.
func BuildEditorCmd(command, filePath string, lineNumber int) (*exec.Cmd, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, errors.New("empty editor command")
	}

	bin := parts[0]
	baseArgs := parts[1:]

	args := make([]string, 0, len(baseArgs)+2)
	args = append(args, baseArgs...)

	if lineNumber > 0 && isVimLike(filepath.Base(bin)) {
		args = append(args, "+"+strconv.Itoa(lineNumber))
	}

	args = append(args, filePath)

	cmd := exec.Command(bin, args...)
	return cmd, nil
}

// LaunchExternalEditor returns a tea.Cmd that suspends the TUI, runs the external
// editor, and resumes the TUI when the editor exits.
func LaunchExternalEditor(command, filePath string, lineNumber int) tea.Cmd {
	cmd, err := BuildEditorCmd(command, filePath, lineNumber)
	if err != nil {
		return func() tea.Msg {
			return ExternalEditorExitedMsg{FilePath: filePath, Err: err}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return ExternalEditorExitedMsg{FilePath: filePath, Err: err}
	})
}

func isVimLike(name string) bool {
	switch name {
	case "vi", "vim", "nvim", "gvim", "mvim":
		return true
	}
	return false
}
