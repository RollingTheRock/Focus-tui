package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPluginCreatePaneReturnsEditorPane(t *testing.T) {
	pl := New()
	panel, err := pl.CreatePane(models.PaneTypeEditor, "editor-1", models.PaneMeta{ID: "editor-1", Type: models.PaneTypeEditor}, models.CommonModel{})
	if err != nil {
		t.Fatalf("create pane: %v", err)
	}
	if _, ok := panel.(*EditorPane); !ok {
		t.Fatalf("expected *EditorPane, got %T", panel)
	}
}

func TestEditorPaneLoadsAndSavesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, path)
	pane.SetSize(80, 16)
	updated, _ := pane.Update(editorLoadedMsg{content: "package main\n"})
	pane = updated.(*EditorPane)

	pane.input.SetValue("package main\n\nfunc main() {}\n")
	pane.dirty = true
	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	pane = updated.(*EditorPane)
	if cmd == nil {
		t.Fatalf("expected save command")
	}
	updated, saveCmd := pane.Update(runCmd(t, cmd))
	pane = updated.(*EditorPane)
	if saveCmd == nil {
		t.Fatalf("expected save completed command")
	}
	if pane.dirty {
		t.Fatalf("expected pane to be clean after save")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(content) != "package main\n\nfunc main() {}\n" {
		t.Fatalf("unexpected saved content: %q", string(content))
	}
}

func TestEditorPaneEscRequiresConfirmationWhenDirty(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, "/tmp/main.go")
	pane.originalContent = "package main\n"
	pane.input.SetValue("package main\n\nfunc main() {}\n")
	pane.dirty = true

	updated, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pane = updated.(*EditorPane)
	if cmd != nil {
		t.Fatalf("expected no close command before confirmation")
	}
	if !pane.confirmClose {
		t.Fatalf("expected confirmClose to be enabled")
	}

	updated, cmd = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	pane = updated.(*EditorPane)
	if cmd != nil {
		t.Fatalf("expected no close command when declining close")
	}
	if pane.confirmClose {
		t.Fatalf("expected confirmClose to be cleared after decline")
	}
}

func TestEditorPaneViewShowsShortcutHint(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, "")
	pane.SetSize(80, 16)
	view := pane.View()
	for _, want := range []string{"main.go", "Ctrl+S save", "Esc close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected command")
	}
	return cmd()
}
