package app

import (
	"path/filepath"
	"strings"
	"testing"

	"focus/internal/config"
	gitmodel "focus/internal/git"
	"focus/internal/models"
	editorplugin "focus/internal/plugins/editor"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/store"
	"focus/internal/ui/layout"
	"focus/internal/ui/shell"

	tea "github.com/charmbracelet/bubbletea"
)

type fakePanel struct {
	updates []tea.Msg
}

func (p *fakePanel) Init() tea.Cmd { return nil }

func (p *fakePanel) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	p.updates = append(p.updates, msg)
	return p, nil
}

func (p *fakePanel) View() string { return "" }

func (p *fakePanel) SetSize(width, height int) {}

type fakeEditorMetaPanel struct {
	fakePanel
	filePath string
	dirty    bool
	name     string
}

func (p *fakeEditorMetaPanel) FilePath() string { return p.filePath }

func (p *fakeEditorMetaPanel) Dirty() bool { return p.dirty }

func (p *fakeEditorMetaPanel) DisplayName() string { return p.name }

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		home string
		want string
	}{
		{name: "home path collapsed", path: "/home/dev/projects/focus-tui", home: "/home/dev", want: "~/projects/focus-tui"},
		{name: "long absolute path shortened", path: "/mnt/d/dev/focus-tui/internal", home: "", want: "/.../focus-tui/internal"},
		{name: "short relative path kept", path: "focus/internal", home: "", want: "focus/internal"},
	}

	for _, tt := range tests {
		if got := shortenPath(tt.path, tt.home); got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestFormatPaneTitleIncludesShellStatusAndFocus(t *testing.T) {
	meta := models.PaneMeta{
		Name:   "Shell 2",
		Type:   models.PaneTypeShell,
		CWD:    "/home/dev/projects/focus-tui",
		Status: models.PaneStatusRunning,
	}

	got := formatPaneTitle(meta, true, false, 56)
	want := "SHELL 2 [/.../projects/focus-tui] [running] [focus]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleUsesFixedBadgeForNativePane(t *testing.T) {
	meta := models.PaneMeta{Name: "Todo", Type: models.PaneTypeTodo, Closable: false}

	got := formatPaneTitle(meta, false, false, 20)
	want := "TODO [fixed]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleShowsModifiedBadgeForEditor(t *testing.T) {
	meta := models.PaneMeta{Name: "*main.go", Type: models.PaneTypeEditor, Closable: true}

	got := formatPaneTitle(meta, false, false, 28)
	want := "*MAIN.GO [modified]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatPaneTitleUsesActiveForShellMode(t *testing.T) {
	meta := models.PaneMeta{Name: "Shell", Type: models.PaneTypeShell, Status: models.PaneStatusRunning}

	got := formatPaneTitle(meta, true, true, 28)
	want := "SHELL [running] [active]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderHelpLineForEditorIncludesSearchShortcuts(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.focused = paneFileTree
	editorID := models.PaneID("editor-1")
	m.panes[editorID] = &fakeEditorMetaPanel{name: "main.go", filePath: "/tmp/main.go"}
	m.paneMeta[editorID] = models.PaneMeta{ID: editorID, Name: "main.go", Type: models.PaneTypeEditor, Closable: true}
	m.paneOrder = append(m.paneOrder, editorID)
	m.focused = editorID

	help := m.renderHelpLine(120)
	for _, want := range []string{"[ctrl+s]save", "[ctrl+f /]search", "[:]line", "[n/N]result"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help line to contain %q, got %q", want, help)
		}
	}
}

func TestRenderHelpLineForGitStatusIncludesReviewShortcuts(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.focused = paneGitStatus

	help := m.renderHelpLine(140)
	for _, want := range []string{"[enter]review", "[d]iff file", "[space]stage"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help line to contain %q, got %q", want, help)
		}
	}
}

func TestRenderHelpLineForWorktreePaneIncludesRefreshShortcut(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.focused = paneWorktree

	help := m.renderHelpLine(120)
	for _, want := range []string{"[j/k]move", "[r]efresh"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help line to contain %q, got %q", want, help)
		}
	}
}

func TestSwitchToWorktreePageTracksPageState(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	if m.state != StateWorktreePage {
		t.Fatalf("expected worktree page state, got %v", m.state)
	}
	if m.currentWorktreePage != "/repo/feature-a" {
		t.Fatalf("expected current worktree page to be tracked, got %q", m.currentWorktreePage)
	}
}

func TestCtrlGReturnsToOverviewPage(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = updated.(model)
	if m.state != StateOverviewPage {
		t.Fatalf("expected overview page state, got %v", m.state)
	}
	if m.currentWorktreePage != "" {
		t.Fatalf("expected overview to clear active worktree page, got %q", m.currentWorktreePage)
	}
	if m.focused != paneWorktree {
		t.Fatalf("expected focus to return to worktree overview pane, got %s", m.focused)
	}
}

func TestRenderHelpLineForDiffPaneIncludesReviewCloseShortcut(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.panes[paneGitDiff] = &fakePanel{}
	m.paneMeta[paneGitDiff] = models.PaneMeta{ID: paneGitDiff, Name: "Diff", Type: models.PaneTypeDiffView, Closable: true}
	m.bodyTree = layout.SplitLeaf(m.bodyTree, paneShell, paneGitDiff, layout.SplitHorizontal, true)
	m.focused = paneGitDiff

	help := m.renderHelpLine(120)
	for _, want := range []string{"[enter]open file", "[s]toggle staged", "[[]/[]]files", "[wheel]scroll", "[q/esc]close review"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected diff help line to contain %q, got %q", want, help)
		}
	}
}

func TestFormatPaneTitleDropsCWDWhenPaneIsNarrow(t *testing.T) {
	meta := models.PaneMeta{
		Name:   "Shell 2",
		Type:   models.PaneTypeShell,
		CWD:    "/home/dev/projects/focus-tui",
		Status: models.PaneStatusRunning,
	}

	got := formatPaneTitle(meta, true, false, 18)
	want := "SHELL 2 [focus]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestShellRefreshMsgRoutesOnlyToTargetPane(t *testing.T) {
	target := &fakePanel{}
	other := &fakePanel{}
	m := model{
		panes: map[models.PaneID]models.Panel{
			"shell-1": target,
			"shell-2": other,
		},
		paneMeta: map[models.PaneID]models.PaneMeta{
			"shell-1": {ID: "shell-1", Type: models.PaneTypeShell},
			"shell-2": {ID: "shell-2", Type: models.PaneTypeShell},
		},
		paneOrder: []models.PaneID{"shell-1", "shell-2"},
	}

	updated, _ := m.Update(shell.RefreshMsg{PaneID: "shell-1"})
	m = updated.(model)

	targetPanel := m.panes["shell-1"].(*fakePanel)
	otherPanel := m.panes["shell-2"].(*fakePanel)
	if len(targetPanel.updates) != 1 {
		t.Fatalf("expected target pane to receive 1 update, got %d", len(targetPanel.updates))
	}
	if len(otherPanel.updates) != 0 {
		t.Fatalf("expected other pane to receive 0 updates, got %d", len(otherPanel.updates))
	}
}

func keyCtrlBackslash() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlBackslash}
}

func keyCtrlW() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlW}
}

func TestSplitFocusedHorizontal(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	initialOrder := layout.LeafOrder(m.bodyTree)
	if len(initialOrder) != 4 {
		t.Fatalf("expected 4 panes initially (shell + worktree + git + file-tree), got %d", len(initialOrder))
	}

	newM, cmd := m.Update(keyCtrlBackslash())
	if cmd == nil {
		t.Fatal("expected cmd from split, got nil")
	}
	m = newM.(model)

	newOrder := layout.LeafOrder(m.bodyTree)
	if len(newOrder) != 5 {
		t.Fatalf("expected 5 panes after split, got %d", len(newOrder))
	}

	foundNewPane := false
	for _, id := range newOrder {
		if string(id) != string(paneShell) && string(id) != string(paneWorktree) && string(id) != string(paneGitStatus) && string(id) != string(paneFileTree) {
			foundNewPane = true
			if m.focused != id {
				t.Fatalf("expected focus on new pane %s, got %s", id, m.focused)
			}
			if meta, ok := m.paneMeta[id]; !ok || meta.Type != models.PaneTypeShell {
				t.Fatalf("expected new pane to be shell type, got %v", meta.Type)
			}
			break
		}
	}
	if !foundNewPane {
		t.Fatal("new pane not found after split")
	}
}

func TestZoomToggle(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	initialOrder := layout.LeafOrder(m.bodyTree)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = newM.(model)

	if m.zoomedPane != paneShell {
		t.Fatalf("expected zoomed pane to be shell, got %s", m.zoomedPane)
	}

	zoomedOrder := layout.LeafOrder(m.bodyTree)
	if len(zoomedOrder) != 1 {
		t.Fatalf("expected 1 pane when zoomed, got %d", len(zoomedOrder))
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = newM.(model)

	if m.zoomedPane != "" {
		t.Fatalf("expected zoom to be restored, got %s", m.zoomedPane)
	}

	restoredOrder := layout.LeafOrder(m.bodyTree)
	if len(restoredOrder) != len(initialOrder) {
		t.Fatalf("expected %d panes after restore, got %d", len(initialOrder), len(restoredOrder))
	}
}

func TestOpenEditorPaneReusesExistingEditorForSameFile(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	path := filepath.Join(t.TempDir(), "main.go")
	cmd := m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	if cmd == nil {
		t.Fatalf("expected init command for first editor open")
	}
	firstFocused := m.focused
	if meta, ok := m.paneMeta[firstFocused]; !ok || meta.Type != models.PaneTypeEditor {
		t.Fatalf("expected focused pane to be editor, got %v", meta.Type)
	}
	leafCount := len(layout.LeafOrder(m.bodyTree))

	cmd = m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	if cmd != nil {
		t.Fatalf("expected no init command when reusing existing editor")
	}
	if len(layout.LeafOrder(m.bodyTree)) != leafCount {
		t.Fatalf("expected leaf count to remain %d when reusing editor", leafCount)
	}
	if m.focused != firstFocused {
		t.Fatalf("expected focus to stay on reused editor %s, got %s", firstFocused, m.focused)
	}
}

func TestEditorHostPaneTargetPrefersLastEditorFromFileTree(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	path := filepath.Join(t.TempDir(), "main.go")
	m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	firstEditor := m.focused
	m.setFocus(paneFileTree)

	target := m.editorHostPaneTarget(m.focused, editorplugin.OpenBehaviorDefault)
	if target != firstEditor {
		t.Fatalf("expected default tree open to target existing editor %s, got %s", firstEditor, target)
	}

	vsplitTarget := m.editorHostPaneTarget(m.focused, editorplugin.OpenBehaviorVSplit)
	if vsplitTarget != firstEditor {
		t.Fatalf("expected vsplit tree open to target existing editor %s, got %s", firstEditor, vsplitTarget)
	}
}

func TestEditorSplitDirectionDistinguishesDefaultAndVSplit(t *testing.T) {
	m := model{
		paneMeta: map[models.PaneID]models.PaneMeta{
			"editor-1": {ID: "editor-1", Type: models.PaneTypeEditor},
			paneShell:  {ID: paneShell, Type: models.PaneTypeShell},
		},
	}

	if got := m.editorSplitDirection("editor-1", editorplugin.OpenBehaviorDefault); got != layout.SplitVertical {
		t.Fatalf("expected default open against editor to split vertically, got %s", got)
	}
	if got := m.editorSplitDirection("editor-1", editorplugin.OpenBehaviorVSplit); got != layout.SplitHorizontal {
		t.Fatalf("expected vsplit open against editor to split horizontally, got %s", got)
	}
	if got := m.editorSplitDirection(paneShell, editorplugin.OpenBehaviorDefault); got != layout.SplitHorizontal {
		t.Fatalf("expected default open against shell to split horizontally, got %s", got)
	}
}

func TestClosePaneRestoresFocusToEditorOpener(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	path := filepath.Join(t.TempDir(), "main.go")
	m.setFocus(paneFileTree)
	m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	editorID := m.focused

	m.closePane(editorID)

	if m.focused != paneFileTree {
		t.Fatalf("expected focus to return to file tree, got %s", m.focused)
	}
	if _, ok := m.paneMeta[editorID]; ok {
		t.Fatalf("expected editor pane %s to be removed", editorID)
	}
}

func TestOpenDiffPaneAddsBodyPaneAndRestoresOpenerFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneGitStatus)
	initialLeaves := len(layout.LeafOrder(m.bodyTree))

	cmd := m.openDiffPane(gitplugin.OpenDiffMsg{FilePath: "", Staged: false})
	if cmd == nil {
		t.Fatalf("expected init command for diff pane")
	}
	if m.focused != paneGitDiff {
		t.Fatalf("expected diff pane to be focused, got %s", m.focused)
	}
	if got := len(layout.LeafOrder(m.bodyTree)); got != initialLeaves+1 {
		t.Fatalf("expected leaf count %d after opening diff pane, got %d", initialLeaves+1, got)
	}
	if meta, ok := m.paneMeta[paneGitDiff]; !ok || meta.Type != models.PaneTypeDiffView {
		t.Fatalf("expected diff pane metadata to be registered")
	}

	m.closePane(paneGitDiff)
	if m.focused != paneGitStatus {
		t.Fatalf("expected focus to return to git status, got %s", m.focused)
	}
}

func TestOpenWorktreeShellAddsScopedShellPane(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneWorktree)
	initialLeaves := len(layout.LeafOrder(m.bodyTree))

	cmd := m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	if cmd == nil {
		t.Fatalf("expected init command for worktree shell")
	}
	if got := len(layout.LeafOrder(m.bodyTree)); got != initialLeaves+1 {
		t.Fatalf("expected leaf count %d after opening worktree shell, got %d", initialLeaves+1, got)
	}
	if m.focused == paneWorktree {
		t.Fatalf("expected focus to move to new shell")
	}
	meta, ok := m.paneMeta[m.focused]
	if !ok || meta.Type != models.PaneTypeShell {
		t.Fatalf("expected focused pane to be shell, got %+v", meta)
	}
	if meta.CWD != "/repo/feature-a" {
		t.Fatalf("expected shell cwd to be worktree path, got %q", meta.CWD)
	}
	if meta.WorktreeID != "/repo/feature-a" {
		t.Fatalf("expected shell worktree id to match path, got %q", meta.WorktreeID)
	}
	if meta.BranchSnapshot != "feature-a" {
		t.Fatalf("expected branch snapshot feature-a, got %q", meta.BranchSnapshot)
	}
	if sh, ok := m.pane(m.focused).(*shell.Model); !ok || sh == nil {
		t.Fatalf("expected focused pane to hold shell model")
	}
}

func gitplugin_testWorktree(path, branch string) gitmodel.Worktree {
	return gitmodel.Worktree{Path: path, Branch: branch}
}

func TestOpenCreateWorktreePaneUsesOverlayLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneWorktree)

	cmd := m.openCreateWorktreePane(gitplugin.OpenCreateWorktreeMsg{RepoPath: "/repo/focus-tui", BaseRef: "main"})
	if cmd == nil {
		t.Fatalf("expected init command for create overlay")
	}
	if m.activeOverlayPane() != paneWorktreeCreate {
		t.Fatalf("expected active overlay %s, got %s", paneWorktreeCreate, m.activeOverlayPane())
	}
	if m.focused != paneWorktreeCreate {
		t.Fatalf("expected focus on create overlay, got %s", m.focused)
	}

	m.closePane(paneWorktreeCreate)
	if m.focused != paneWorktree {
		t.Fatalf("expected focus to restore to worktree pane, got %s", m.focused)
	}
}

func TestWorktreeCreatedRefreshesAndOpensShell(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	worktreePanel, ok := m.pane(paneWorktree).(*gitplugin.WorktreePane)
	if !ok || worktreePanel == nil {
		t.Fatalf("expected worktree pane to be registered")
	}
	initialLeaves := len(layout.LeafOrder(m.bodyTree))

	updatedModel, cmd := m.Update(gitplugin.WorktreeCreatedMsg{ID: paneWorktreeCreate, Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	m = updatedModel.(model)
	if cmd == nil {
		t.Fatalf("expected batched commands after worktree creation")
	}
	if got := len(layout.LeafOrder(m.bodyTree)); got != initialLeaves+1 {
		t.Fatalf("expected leaf count %d after opening new worktree shell, got %d", initialLeaves+1, got)
	}
	if m.paneMeta[m.focused].Type != models.PaneTypeShell {
		t.Fatalf("expected focus on shell after worktree creation, got %v", m.paneMeta[m.focused].Type)
	}
	if m.paneMeta[m.focused].CWD != "/repo/feature-a" {
		t.Fatalf("expected created shell cwd /repo/feature-a, got %q", m.paneMeta[m.focused].CWD)
	}
	if worktreePanel == nil {
		t.Fatalf("expected worktree pane to remain present")
	}
}

func TestWorktreeRemovedClosesScopedPanes(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneWorktree)
	m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	removedShell := m.focused
	if m.paneMeta[removedShell].Type != models.PaneTypeShell {
		t.Fatalf("expected focused pane to be shell")
	}

	updatedModel, _ := m.Update(gitplugin.WorktreeRemovedMsg{Path: "/repo/feature-a"})
	m = updatedModel.(model)
	if _, ok := m.paneMeta[removedShell]; ok {
		t.Fatalf("expected removed worktree shell pane to be closed")
	}
	if m.focused == removedShell {
		t.Fatalf("expected focus to move away from removed shell")
	}
}

func TestHandleMouseRoutesWheelToDiffPaneWithoutStealingFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.panes[paneGitDiff] = &fakePanel{}
	m.paneMeta[paneGitDiff] = models.PaneMeta{ID: paneGitDiff, Name: "Diff", Type: models.PaneTypeDiffView, Closable: true}
	m.bodyTree = layout.SplitLeaf(m.bodyTree, paneShell, paneGitDiff, layout.SplitHorizontal, true)
	m.focused = paneGitStatus
	m.common.Width = 120
	m.common.Height = 40
	m.updateSizes(120, 40)
	frame := m.frames[paneGitDiff]
	dims := layout.ComputeBanner(m.common.Width, m.common.Height)

	updated, _ := m.handleMouse(tea.MouseMsg{X: frame.X + 1, Y: dims.HeaderH + frame.Y + 1, Button: tea.MouseButtonWheelDown})
	m = updated.(model)

	diffPanel := m.pane(paneGitDiff).(*fakePanel)
	if len(diffPanel.updates) == 0 {
		t.Fatalf("expected diff pane to receive mouse wheel message")
	}
	if _, ok := diffPanel.updates[0].(tea.MouseMsg); !ok {
		t.Fatalf("expected forwarded message to be tea.MouseMsg, got %T", diffPanel.updates[0])
	}
	if m.focused != paneGitStatus {
		t.Fatalf("expected wheel scroll not to steal focus, got %s", m.focused)
	}
}

func TestSyncPaneMetaMarksDirtyEditorName(t *testing.T) {
	m := model{
		panes: map[models.PaneID]models.Panel{
			"editor-1": &fakeEditorMetaPanel{filePath: "/tmp/main.go", dirty: true, name: "main.go"},
		},
		paneMeta: map[models.PaneID]models.PaneMeta{
			"editor-1": {ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor, Closable: true},
		},
	}

	m.syncPaneMeta("editor-1")
	if got := m.paneMeta["editor-1"].Name; got != "*main.go" {
		t.Fatalf("expected dirty editor title '*main.go', got %q", got)
	}
}
