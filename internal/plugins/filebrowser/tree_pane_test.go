package filebrowser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"focus/internal/models"
	editorplugin "focus/internal/plugins/editor"

	tea "charm.land/bubbletea/v2"
)

func TestPluginCreatePaneReturnsTreePane(t *testing.T) {
	pl := New()

	panel, err := pl.CreatePane(models.PaneTypeFileTree, "tree-1", models.PaneMeta{ID: "tree-1", Type: models.PaneTypeFileTree, CWD: "/tmp"}, models.CommonModel{})
	if err != nil {
		t.Fatalf("create pane: %v", err)
	}

	if _, ok := panel.(*TreePane); !ok {
		t.Fatalf("expected *TreePane, got %T", panel)
	}
}

func TestFlattenVisibleNodesRespectsCollapsedDirectories(t *testing.T) {
	root := &FileNode{Name: "root", Path: "/root", IsDir: true}
	dir := &FileNode{Name: "dir", Path: "/root/dir", IsDir: true, Parent: root, Depth: 1, Collapsed: true}
	file := &FileNode{Name: "file.txt", Path: "/root/dir/file.txt", Parent: dir, Depth: 2}
	other := &FileNode{Name: "other.txt", Path: "/root/other.txt", Parent: root, Depth: 1}
	dir.Children = []*FileNode{file}
	root.Children = []*FileNode{dir, other}

	flat := flattenVisibleNodes(root)
	if len(flat) != 3 {
		t.Fatalf("expected 3 visible nodes, got %d", len(flat))
	}
	for _, node := range flat {
		if node.Path == file.Path {
			t.Fatalf("expected collapsed child to be hidden")
		}
	}

	dir.Collapsed = false
	flat = flattenVisibleNodes(root)
	if len(flat) != 4 {
		t.Fatalf("expected 4 visible nodes after expand, got %d", len(flat))
	}
}

func TestTreePaneRefreshLoadsDirectoryAndRendersIcons(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "main.go"))
	mustMkdir(t, filepath.Join(dir, "pkg"))
	mustWriteFile(t, filepath.Join(dir, "pkg", "util.go"))

	pane := NewTreePane("tree-1", models.PaneMeta{ID: "tree-1", Type: models.PaneTypeFileTree, CWD: dir}, models.CommonModel{})
	pane.SetSize(80, 10)

	msg := runCmd(t, pane.refreshTreeCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*TreePane)

	view := pane.View()
	for _, want := range []string{filepath.Base(dir), "main.go", "pkg", "\uf07b", "\ue627"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view.Content)
		}
	}
	if pane.loading {
		t.Fatalf("expected loading to be false after refresh")
	}
	if len(pane.flatList) < 3 {
		t.Fatalf("expected populated flat list, got %d", len(pane.flatList))
	}
}

func TestTreePaneNavigationAndToggleDirectory(t *testing.T) {
	root := &FileNode{Name: "root", Path: "/root", IsDir: true}
	dir := &FileNode{Name: "dir", Path: "/root/dir", IsDir: true, Parent: root, Depth: 1}
	file := &FileNode{Name: "file.txt", Path: "/root/dir/file.txt", Parent: dir, Depth: 2}
	other := &FileNode{Name: "other.txt", Path: "/root/other.txt", Parent: root, Depth: 1}
	dir.Children = []*FileNode{file}
	root.Children = []*FileNode{dir, other}

	pane := &TreePane{root: root, flatList: flattenVisibleNodes(root)}

	updated, _ := pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*TreePane)
	if pane.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", pane.cursor)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*TreePane)
	if !dir.Collapsed {
		t.Fatalf("expected directory to collapse")
	}
	if len(pane.flatList) != 3 {
		t.Fatalf("expected collapsed list to hide nested file, got %d nodes", len(pane.flatList))
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*TreePane)
	if dir.Collapsed {
		t.Fatalf("expected directory to expand")
	}
	if len(pane.flatList) != 4 {
		t.Fatalf("expected expanded list to restore nested file, got %d nodes", len(pane.flatList))
	}
}

func TestTreePaneRefreshPreservesCollapseStateAndFindsNewFiles(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "pkg"))
	mustWriteFile(t, filepath.Join(dir, "pkg", "one.go"))

	pane := NewTreePane("tree-1", models.PaneMeta{ID: "tree-1", Type: models.PaneTypeFileTree, CWD: dir}, models.CommonModel{})
	updated, _ := pane.Update(runCmd(t, pane.refreshTreeCmd()))
	pane = updated.(*TreePane)

	pane.cursor = 1
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*TreePane)
	if !pane.flatList[1].Collapsed {
		t.Fatalf("expected pkg directory to be collapsed")
	}

	mustWriteFile(t, filepath.Join(dir, "pkg", "two.go"))
	updated, _ = pane.Update(runCmd(t, pane.refreshTreeCmd()))
	pane = updated.(*TreePane)

	if !pane.flatList[1].Collapsed {
		t.Fatalf("expected collapse state to persist across refresh")
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*TreePane)
	if pane.flatList[1].Collapsed {
		t.Fatalf("expected directory to expand after toggle")
	}

	view := pane.View()
	if !strings.Contains(view.Content, "two.go") {
		t.Fatalf("expected refreshed view to contain new file, got:\n%s", view.Content)
	}
}

func TestTreePaneEnterOnFileEmitsOpenEditorMsg(t *testing.T) {
	root := &FileNode{Name: "root", Path: "/root", IsDir: true}
	file := &FileNode{Name: "main.go", Path: "/root/main.go", Parent: root, Depth: 1}
	root.Children = []*FileNode{file}
	pane := &TreePane{root: root, flatList: flattenVisibleNodes(root), cursor: 1}

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*TreePane)
	msg, ok := runCmd(t, cmd).(editorplugin.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", runCmd(t, cmd))
	}
	if msg.FilePath != "/root/main.go" {
		t.Fatalf("unexpected file path %q", msg.FilePath)
	}
	if msg.Behavior != editorplugin.OpenBehaviorDefault {
		t.Fatalf("unexpected behavior %q", msg.Behavior)
	}
	_ = pane
}

func TestTreePaneVOnFileEmitsSplitEditorMsg(t *testing.T) {
	root := &FileNode{Name: "root", Path: "/root", IsDir: true}
	file := &FileNode{Name: "main.go", Path: "/root/main.go", Parent: root, Depth: 1}
	root.Children = []*FileNode{file}
	pane := &TreePane{root: root, flatList: flattenVisibleNodes(root), cursor: 1}

	_, cmd := pane.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	msg, ok := runCmd(t, cmd).(editorplugin.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", runCmd(t, cmd))
	}
	if msg.Behavior != editorplugin.OpenBehaviorVSplit {
		t.Fatalf("unexpected behavior %q", msg.Behavior)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("package test\n"), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected command")
	}
	return cmd()
}
