package git

import (
	"strings"
	"testing"

	gitmodel "focus/internal/git"
	"focus/internal/models"
	editorplugin "focus/internal/plugins/editor"

	tea "charm.land/bubbletea/v2"
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

	pane = initPane(t, pane).(*DiffPane)

	if pane.loading {
		t.Fatalf("expected loading to be false after init")
	}

	view := pane.View()
	for _, want := range []string{"main.go", "staged", "File · main.go", "-old line", "+new line", "context line"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view.Content)
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

	pane = initPane(t, pane).(*DiffPane)

	updated, _ := pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*DiffPane)
	if pane.scroll != 1 {
		t.Fatalf("expected scroll 1, got %d", pane.scroll)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*DiffPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*DiffPane)
	if pane.scroll != 3 {
		t.Fatalf("expected scroll to clamp at 3, got %d", pane.scroll)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	pane = updated.(*DiffPane)
	if pane.scroll != 2 {
		t.Fatalf("expected scroll 2, got %d", pane.scroll)
	}

	_, cmd := pane.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
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
			"@@ -10 +10 @@",
			"-old",
			"+new",
			"diff --git a/b.go b/b.go",
			"@@ -22 +22 @@",
			"-before",
			"+after",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)
	pane.SetSize(90, 4)

	pane = initPane(t, pane).(*DiffPane)
	pane.scroll = 5

	_, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(editorplugin.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if openMsg.FilePath != "b.go" {
		t.Fatalf("expected b.go from current review section, got %q", openMsg.FilePath)
	}
	if openMsg.LineNumber != 22 {
		t.Fatalf("expected hunk target line 22, got %d", openMsg.LineNumber)
	}
	if openMsg.Behavior != editorplugin.OpenBehaviorDefault {
		t.Fatalf("expected default editor open behavior, got %q", openMsg.Behavior)
	}
}

func TestDiffPaneBracketNavigationMovesBetweenFileSections(t *testing.T) {
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
	pane.SetSize(90, 4)

	pane = initPane(t, pane).(*DiffPane)

	updated, _ := pane.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	pane = updated.(*DiffPane)
	if pane.currentFilePath() != "b.go" {
		t.Fatalf("expected to jump to b.go, got %q", pane.currentFilePath())
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	pane = updated.(*DiffPane)
	if pane.currentFilePath() != "a.go" {
		t.Fatalf("expected to jump back to a.go, got %q", pane.currentFilePath())
	}
}

func TestDiffPaneMouseWheelScrollsReview(t *testing.T) {
	adapter := &fakeGitAdapter{
		diff: strings.Join([]string{
			"diff --git a/a.go b/a.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
			" context",
			" context2",
			" context3",
		}, "\n"),
	}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)
	pane.SetSize(80, 4)

	pane = initPane(t, pane).(*DiffPane)

	updated, _ := pane.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	pane = updated.(*DiffPane)
	if pane.scroll == 0 {
		t.Fatalf("expected wheel down to increase scroll")
	}
	prev := pane.scroll
	updated, _ = pane.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	pane = updated.(*DiffPane)
	if pane.scroll >= prev {
		t.Fatalf("expected wheel up to reduce scroll, got %d from %d", pane.scroll, prev)
	}
}

func TestDiffPaneEnterOpensSingleFileDiffTarget(t *testing.T) {
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, &fakeGitAdapter{}, "pkg/main.go", false)

	_, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(editorplugin.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if openMsg.FilePath != "pkg/main.go" {
		t.Fatalf("expected single-file diff path pkg/main.go, got %q", openMsg.FilePath)
	}
	if openMsg.LineNumber != 1 {
		t.Fatalf("expected fallback line number 1, got %d", openMsg.LineNumber)
	}
}

func TestDiffPaneToggleStagedReloadsDiff(t *testing.T) {
	adapter := &fakeGitAdapter{diff: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new"}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	pane = updated.(*DiffPane)
	if !pane.staged {
		t.Fatalf("expected staged flag to toggle on")
	}
	if cmd == nil {
		t.Fatalf("expected reload command after staged toggle")
	}
}

func TestStatusPaneEnterOpensDiff(t *testing.T) {
	pane := &StatusPane{
		status: &gitmodel.Status{
			StagedFiles: []gitmodel.File{{Path: "staged.go", StagedStatus: gitmodel.Modified}},
		},
	}

	_, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	if _, cmd := pane.Update(tea.KeyPressMsg{Code: 'd', Text: "d"}); cmd == nil {
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

	pane = initPane(t, pane).(*DiffPane)

	view := pane.View()
	for _, want := range []string{"Review · unstaged", "File · a.go", "File · b.go", "+after"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("expected review view to contain %q, got:\n%s", want, view.Content)
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

	pane = initPane(t, pane).(*DiffPane)

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

func initPane(t *testing.T, pane models.Panel) models.Panel {
	t.Helper()
	cmd := pane.Init()
	if cmd == nil {
		t.Fatalf("expected init command")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		updated, _ := pane.Update(msg)
		return updated
	}
	var final models.Panel = pane
	for _, sub := range batch {
		subMsg := sub()
		updated, _ := final.Update(subMsg)
		final = updated
	}
	return final
}

func TestParseNewHunkLineUsesTargetSide(t *testing.T) {
	lineNumber, ok := parseNewHunkLine("@@ -10,2 +42,7 @@")
	if !ok {
		t.Fatalf("expected hunk line parse to succeed")
	}
	if lineNumber != 42 {
		t.Fatalf("expected target line 42, got %d", lineNumber)
	}
}

func TestDiffPaneRendersSpecialDiffMetadata(t *testing.T) {
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, &fakeGitAdapter{}, "", false)
	for input, want := range map[string]string{
		"rename from old.go":                      "↪ old.go",
		"rename to new.go":                        "→ new.go",
		"new file mode 100644":                    "+ new file",
		"deleted file mode 100644":                "- deleted file",
		"Binary files a/a.png and b/a.png differ": "Binary files",
	} {
		rendered := pane.renderDiffLine(input)
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q to render %q, got %q", input, want, rendered)
		}
	}
}
