package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"focus/internal/models"

	"github.com/charmbracelet/x/ansi"
	tea "charm.land/bubbletea/v2"
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

	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, path, 0)
	pane.SetSize(80, 16)
	updated, _ := pane.Update(editorLoadedMsg{content: "package main\n"})
	pane = updated.(*EditorPane)

	pane.input.SetValue("package main\n\nfunc main() {}\n")
	pane.dirty = true
	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl, Text: "s"})
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
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, "/tmp/main.go", 0)
	pane.originalContent = "package main\n"
	pane.input.SetValue("package main\n\nfunc main() {}\n")
	pane.dirty = true

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	pane = updated.(*EditorPane)
	if cmd != nil {
		t.Fatalf("expected no close command before confirmation")
	}
	if !pane.confirmClose {
		t.Fatalf("expected confirmClose to be enabled")
	}

	updated, cmd = pane.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	pane = updated.(*EditorPane)
	if cmd != nil {
		t.Fatalf("expected no close command when declining close")
	}
	if pane.confirmClose {
		t.Fatalf("expected confirmClose to be cleared after decline")
	}
}

func TestEditorPaneViewShowsShortcutHint(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, "", 0)
	pane.SetSize(80, 16)
	view := pane.View()
	for _, want := range []string{"main.go"} {
		if !strings.Contains(ansi.Strip(view.Content), want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view.Content)
		}
	}
}

func TestEditorPaneReloadsCleanBufferAfterExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, path, 0)
	updated, _ := pane.Update(runCmd(t, pane.loadFileCmd(fileOpenedNotice(path))))
	pane = updated.(*EditorPane)

	newModTime := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	if err := os.Chtimes(path, newModTime, newModTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	updated, cmd := pane.Update(runCmd(t, pane.checkExternalFileCmd()))
	pane = updated.(*EditorPane)
	if cmd == nil {
		t.Fatalf("expected reload command after clean external change")
	}

	updated, _ = pane.Update(runCmd(t, cmd))
	pane = updated.(*EditorPane)
	if got := pane.input.Value(); got != "package main\n\nfunc main() {}\n" {
		t.Fatalf("expected reloaded content, got %q", got)
	}
	if pane.dirty {
		t.Fatalf("expected pane to stay clean after reload")
	}
	if !strings.Contains(pane.notice, "reloaded") {
		t.Fatalf("expected reload notice, got %q", pane.notice)
	}
}

func TestEditorPaneWarnsWhenDirtyBufferHasExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, path, 0)
	updated, _ := pane.Update(runCmd(t, pane.loadFileCmd(fileOpenedNotice(path))))
	pane = updated.(*EditorPane)
	pane.input.SetValue("package main\n\nfunc local() {}\n")
	pane.dirty = true

	newModTime := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(path, []byte("package main\n\nfunc remote() {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	if err := os.Chtimes(path, newModTime, newModTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	updated, cmd := pane.Update(runCmd(t, pane.checkExternalFileCmd()))
	pane = updated.(*EditorPane)
	if cmd != nil {
		t.Fatalf("expected no reload command when buffer is dirty")
	}
	if !pane.externalChange {
		t.Fatalf("expected externalChange warning to be set")
	}
	if !strings.Contains(pane.notice, "save will overwrite") {
		t.Fatalf("expected overwrite warning, got %q", pane.notice)
	}
	if got := pane.input.Value(); got != "package main\n\nfunc local() {}\n" {
		t.Fatalf("expected local buffer to stay intact, got %q", got)
	}
}

func TestEditorPaneUsesReadOnlyPreviewForLargeFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.txt")
	largeContent := strings.Repeat("abcdefg\n", (maxEditableBytes/8)+32)
	if err := os.WriteFile(path, []byte(largeContent), 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "large.txt", Type: models.PaneTypeEditor}, models.CommonModel{}, path, 0)
	updated, _ := pane.Update(runCmd(t, pane.loadFileCmd(fileOpenedNotice(path))))
	pane = updated.(*EditorPane)

	if !pane.readOnly || !pane.previewMode {
		t.Fatalf("expected large file to load as read-only preview")
	}
	if !strings.Contains(pane.previewReason, "Large file preview") {
		t.Fatalf("expected large file preview reason, got %q", pane.previewReason)
	}
	if !strings.Contains(pane.previewContent, "[preview truncated]") {
		t.Fatalf("expected preview truncation marker, got %q", pane.previewContent)
	}
	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl, Text: "s"})
	pane = updated.(*EditorPane)
	if cmd != nil {
		t.Fatalf("expected no save command for read-only preview")
	}
	if pane.notice != "preview mode is read-only" {
		t.Fatalf("expected read-only notice, got %q", pane.notice)
	}
	if view := pane.View(); !strings.Contains(ansi.Strip(view.Content), "read-only preview") {
		t.Fatalf("expected view to show read-only status, got:\n%s", view.Content)
	}
}

func TestEditorPaneUsesBinaryPreviewForBinaryFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.bin")
	content := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01, 0x02, 0x03}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "image.bin", Type: models.PaneTypeEditor}, models.CommonModel{}, path, 0)
	updated, _ := pane.Update(runCmd(t, pane.loadFileCmd(fileOpenedNotice(path))))
	pane = updated.(*EditorPane)

	if !pane.readOnly || !pane.previewMode {
		t.Fatalf("expected binary file to load as read-only preview")
	}
	if !strings.Contains(pane.previewReason, "Binary file preview") {
		t.Fatalf("expected binary preview reason, got %q", pane.previewReason)
	}
	if !strings.Contains(pane.previewContent, "0000:") {
		t.Fatalf("expected hex preview output, got %q", pane.previewContent)
	}
	if view := pane.View(); !strings.Contains(ansi.Strip(view.Content), "Binary file preview") {
		t.Fatalf("expected binary preview text in view, got:\n%s", view.Content)
	}
}

func TestEditorPanePreviewScrollsWithJK(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "preview.txt", Type: models.PaneTypeEditor}, models.CommonModel{}, "", 0)
	pane.previewMode = true
	pane.readOnly = true
	pane.previewReason = "Large file preview"
	pane.previewContent = strings.Join([]string{"line1", "line2", "line3", "line4", "line5", "line6"}, "\n")
	pane.previewLines = splitPreviewLines(pane.previewContent)
	pane.SetSize(80, 8)

	updated, _ := pane.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	pane = updated.(*EditorPane)
	if pane.previewScroll != 1 {
		t.Fatalf("expected preview scroll 1, got %d", pane.previewScroll)
	}
	if view := pane.View(); !strings.Contains(ansi.Strip(view.Content), "line2") {
		t.Fatalf("expected scrolled view to contain line2, got:\n%s", view.Content)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	pane = updated.(*EditorPane)
	if pane.previewScroll != 0 {
		t.Fatalf("expected preview scroll 0 after scrolling back, got %d", pane.previewScroll)
	}
}

func TestEditorPaneSearchFindsNextMatch(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, "", 0)
	pane.SetSize(80, 16)
	updated, _ := pane.Update(editorLoadedMsg{content: "alpha\nbeta\nalpha again\n"})
	pane = updated.(*EditorPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	pane = updated.(*EditorPane)
	if cmd == nil {
		t.Fatalf("expected focus command for search mode")
	}
	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'a', Text: "alpha"})
	pane = updated.(*EditorPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*EditorPane)

	if !strings.Contains(pane.notice, "match 1/2") {
		t.Fatalf("expected first match notice, got %q", pane.notice)
	}
	if pane.input.Line() != 0 {
		t.Fatalf("expected first match on line 0, got %d", pane.input.Line())
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	pane = updated.(*EditorPane)
	if !strings.Contains(pane.notice, "match 2/2") {
		t.Fatalf("expected second match notice, got %q", pane.notice)
	}
	if pane.input.Line() != 2 {
		t.Fatalf("expected second match on line 2, got %d", pane.input.Line())
	}
	if view := pane.View(); !strings.Contains(ansi.Strip(view.Content), "search \"alpha\" · 2/2") {
		t.Fatalf("expected persistent search status in view, got:\n%s", view.Content)
	}
}

func TestEditorPaneJumpToLineMovesCursor(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor}, models.CommonModel{}, "", 0)
	pane.SetSize(80, 16)
	updated, _ := pane.Update(editorLoadedMsg{content: "one\ntwo\nthree\nfour\n"})
	pane = updated.(*EditorPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	pane = updated.(*EditorPane)
	if cmd == nil {
		t.Fatalf("expected focus command for jump mode")
	}
	updated, _ = pane.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	pane = updated.(*EditorPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*EditorPane)

	if pane.input.Line() != 2 {
		t.Fatalf("expected cursor on line 2 after jump, got %d", pane.input.Line())
	}
	if !strings.Contains(pane.notice, "jumped to line 3") {
		t.Fatalf("expected jump notice, got %q", pane.notice)
	}
}

func TestEditorPanePreviewSearchMovesScrollToMatch(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "preview.txt", Type: models.PaneTypeEditor}, models.CommonModel{}, "", 0)
	pane.previewMode = true
	pane.readOnly = true
	pane.previewReason = "Large file preview"
	pane.previewContent = strings.Join([]string{"zero", "one", "needle here", "three", "needle again", "five"}, "\n")
	pane.previewLines = splitPreviewLines(pane.previewContent)
	pane.SetSize(80, 8)

	updated, _ := pane.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	pane = updated.(*EditorPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'n', Text: "needle"})
	pane = updated.(*EditorPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*EditorPane)

	if !strings.Contains(pane.notice, "match 1/2") {
		t.Fatalf("expected preview search notice, got %q", pane.notice)
	}
	if pane.previewScroll != 2 {
		t.Fatalf("expected preview scroll to first match line, got %d", pane.previewScroll)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	pane = updated.(*EditorPane)
	if pane.previewScroll != 4 {
		t.Fatalf("expected preview scroll to advance to later match, got %d", pane.previewScroll)
	}
	if view := pane.View(); !strings.Contains(ansi.Strip(view.Content), "search \"needle\" · 2/2") || !strings.Contains(ansi.Strip(view.Content), "› needle again") {
		t.Fatalf("expected preview view to show active search status and marker, got:\n%s", view.Content)
	}
}

func TestPreviewLineMatchesTracksActiveSearchHit(t *testing.T) {
	pane := NewEditorPane("editor-1", models.PaneMeta{ID: "editor-1", Name: "preview.txt", Type: models.PaneTypeEditor}, models.CommonModel{}, "", 0)
	pane.previewMode = true
	pane.readOnly = true
	pane.previewLines = []string{"zero needle middle needle end"}
	pane.searchQuery = "needle"
	pane.searchHits = []cursorTarget{{line: 0, column: 5}, {line: 0, column: 19}}
	pane.searchIndex = 1

	matches := pane.previewLineMatches(0)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].active {
		t.Fatalf("expected first match to be inactive")
	}
	if !matches[1].active {
		t.Fatalf("expected second match to be active")
	}
	rendered := pane.renderPreviewLine(0)
	if !strings.Contains(rendered, "needle") {
		t.Fatalf("expected rendered line to keep visible search text, got %q", rendered)
	}
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected command")
	}
	return cmd()
}
