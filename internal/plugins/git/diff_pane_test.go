package git

import (
	"strings"
	"testing"

	gitmodel "focus/internal/git"
	"focus/internal/models"
	editorplugin "focus/internal/plugins/editor"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPluginCreatePaneReturnsDiffPane(t *testing.T) {
	adapter := &fakeGitAdapter{}
	pl := New(adapter)

	panel, err := pl.CreatePane(models.PaneTypeDiffView, "diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{})
	if err != nil {
		t.Fatalf("create pane: %v", err)
	}

	if _, ok := panel.(*DiffPane); !ok {
		t.Fatalf("expected *DiffPane, got %T", panel)
	}
}

func TestDiffPaneInitLoadsAndRendersDiff(t *testing.T) {
	adapter := &fakeGitAdapter{
		diff: strings.Join([]string{
			"diff --git a/main.go b/main.go",
			"index 1111111..2222222 100644",
			"--- a/main.go",
			"+++ b/main.go",
			"@@ -1,2 +1,2 @@",
			"-old line",
			"+new line",
			" context line",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "main.go", true)
	pane.SetSize(80, 10)

	cmd := pane.Init()
	if cmd == nil {
		t.Fatalf("expected init command")
	}

	updated, _ := pane.Update(runCmd(t, cmd))
	pane = updated.(*DiffPane)

	if pane.loading {
		t.Fatalf("expected loading to be false after init")
	}

	view := pane.View()
	for _, want := range []string{"main.go", "staged", "File · main.go", "-old line", "+new line", "context line"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}

	if pane.diff == "" {
		t.Fatalf("expected diff content to be loaded")
	}
}

func TestDiffPaneKeyboardHandling(t *testing.T) {
	adapter := &fakeGitAdapter{
		diff: strings.Join([]string{
			"diff --git a/main.go b/main.go",
			"@@ -1,2 +1,4 @@",
			" line 1",
			"+line 2",
			"+line 3",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "main.go", false)
	pane.SetSize(80, 3)

	updated, _ := pane.Update(runCmd(t, pane.Init()))
	pane = updated.(*DiffPane)

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*DiffPane)
	if pane.scroll != 1 {
		t.Fatalf("expected scroll 1, got %d", pane.scroll)
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*DiffPane)
	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*DiffPane)
	if pane.scroll != 3 {
		t.Fatalf("expected scroll to clamp at 3, got %d", pane.scroll)
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyUp})
	pane = updated.(*DiffPane)
	if pane.scroll != 2 {
		t.Fatalf("expected scroll 2, got %d", pane.scroll)
	}

	_, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	msg := runCmd(t, cmd)
	closeMsg, ok := msg.(CloseDiffMsg)
	if !ok {
		t.Fatalf("expected CloseDiffMsg, got %T", msg)
	}
	if closeMsg.ID != "diff-1" {
		t.Fatalf("expected pane id diff-1, got %q", closeMsg.ID)
	}
}

func TestDiffPaneEnterOpensCurrentReviewFileInEditor(t *testing.T) {
	adapter := &fakeGitAdapter{
		diff: strings.Join([]string{
			"diff --git a/a.go b/a.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
			"diff --git a/b.go b/b.go",
			"@@ -2 +2 @@",
			"-before",
			"+after",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)
	pane.SetSize(90, 12)

	updated, _ := pane.Update(runCmd(t, pane.Init()))
	pane = updated.(*DiffPane)
	pane.scroll = 5

	_, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(editorplugin.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if openMsg.FilePath != "b.go" {
		t.Fatalf("expected b.go from current review section, got %q", openMsg.FilePath)
	}
	if openMsg.Behavior != editorplugin.OpenBehaviorDefault {
		t.Fatalf("expected default editor open behavior, got %q", openMsg.Behavior)
	}
}

func TestDiffPaneEnterOpensSingleFileDiffTarget(t *testing.T) {
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, &fakeGitAdapter{}, "pkg/main.go", false)

	_, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(editorplugin.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if openMsg.FilePath != "pkg/main.go" {
		t.Fatalf("expected single-file diff path pkg/main.go, got %q", openMsg.FilePath)
	}
}

func TestStatusPaneEnterOpensDiff(t *testing.T) {
	pane := &StatusPane{
		status: &gitmodel.Status{
			StagedFiles: []gitmodel.File{{Path: "staged.go", StagedStatus: gitmodel.Modified}},
		},
	}

	_, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenDiffMsg)
	if !ok {
		t.Fatalf("expected OpenDiffMsg, got %T", msg)
	}
	if openMsg.FilePath != "" {
		t.Fatalf("expected review diff with empty file path, got %q", openMsg.FilePath)
	}
	if openMsg.Staged {
		t.Fatalf("expected review diff to default to unstaged view")
	}
	if _, cmd := pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}); cmd == nil {
		t.Fatalf("expected single-file diff command on d")
	} else {
		msg = runCmd(t, cmd)
		openMsg, ok = msg.(OpenDiffMsg)
		if !ok {
			t.Fatalf("expected OpenDiffMsg from d, got %T", msg)
		}
		if openMsg.FilePath != "staged.go" {
			t.Fatalf("expected staged.go from single-file diff, got %q", openMsg.FilePath)
		}
		if !openMsg.Staged {
			t.Fatalf("expected staged single-file diff to preserve staged flag")
		}
	}
}

func TestDiffPaneReviewModeLoadsFullWorktreeDiff(t *testing.T) {
	adapter := &fakeGitAdapter{
		diff: strings.Join([]string{
			"diff --git a/a.go b/a.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
			"diff --git a/b.go b/b.go",
			"@@ -2 +2 @@",
			"-before",
			"+after",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)
	pane.SetSize(90, 12)

	updated, _ := pane.Update(runCmd(t, pane.Init()))
	pane = updated.(*DiffPane)

	view := pane.View()
	for _, want := range []string{"Review · unstaged", "File · a.go", "File · b.go", "+after"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected review view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestParseDiffFilePathUsesRightHandPath(t *testing.T) {
	path, ok := parseDiffFilePath("diff --git a/internal/old.go b/internal/new.go")
	if !ok {
		t.Fatalf("expected diff file path to parse")
	}
	if path != "internal/new.go" {
		t.Fatalf("expected right-hand path internal/new.go, got %q", path)
	}
}

func TestDiffPaneRendersFileSectionsAndHunks(t *testing.T) {
	adapter := &fakeGitAdapter{
		diff: strings.Join([]string{
			"diff --git a/main.go b/main.go",
			"index 1111111..2222222 100644",
			"@@ -1 +1 @@",
			"-old",
			"+new",
			"diff --git a/pkg/util.go b/pkg/util.go",
			"@@ -2 +2 @@",
			"-before",
			"+after",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)
	pane.SetSize(100, 14)

	updated, _ := pane.Update(runCmd(t, pane.Init()))
	pane = updated.(*DiffPane)

	rendered := pane.renderedDiffLines()
	joined := strings.Join(rendered, "\n")
	for _, want := range []string{"File · main.go", "File · pkg/util.go", "@@ -1 +1 @@", "@@ -2 +2 @@"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected rendered review lines to contain %q, got:\n%s", want, joined)
		}
	}
	if len(rendered) < 6 || rendered[5] != "" {
		t.Fatalf("expected blank separator line before second file section, got %#v", rendered)
	}
}
