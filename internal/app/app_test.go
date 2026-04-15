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
	m.activePage.focused = paneFileTree
	editorID := models.PaneID("editor-1")
	m.activePage.panes[editorID] = &fakeEditorMetaPanel{name: "main.go", filePath: "/tmp/main.go"}
	m.activePage.paneMeta[editorID] = models.PaneMeta{ID: editorID, Name: "main.go", Type: models.PaneTypeEditor, Closable: true}
	m.activePage.paneOrder = append(m.activePage.paneOrder, editorID)
	m.activePage.focused = editorID

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
	m.activePage.focused = paneGitStatus

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
	m.activePage.focused = paneWorktree

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
	if m.activePage == nil {
		t.Fatal("expected active page to be set")
	}
	if m.activePage.paneMeta[paneShell].CWD != "/repo/feature-a" {
		t.Fatalf("expected worktree page shell cwd to be worktree path, got %q", m.activePage.paneMeta[paneShell].CWD)
	}
}

func TestCtrlGReturnsToOverviewPage(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	overviewPage := m.activePage
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = updated.(model)
	if m.state != StateOverviewPage {
		t.Fatalf("expected overview page state, got %v", m.state)
	}
	if m.currentWorktreePage != "" {
		t.Fatalf("expected overview to clear active worktree page, got %q", m.currentWorktreePage)
	}
	if m.activePage != overviewPage {
		t.Fatal("expected active page to return to overview instance")
	}
	if m.activePage.focused != paneWorktree {
		t.Fatalf("expected focus to return to worktree overview pane, got %s", m.activePage.focused)
	}
}

func TestOpenWorktreeShellCreatesNewPageInstance(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	overviewPage := m.activePage

	cmd := m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	if cmd == nil {
		t.Fatalf("expected init command for worktree shell")
	}
	if m.activePage == overviewPage {
		t.Fatal("expected active page to switch away from overview")
	}
	if m.state != StateWorktreePage {
		t.Fatalf("expected worktree page state, got %v", m.state)
	}
	if m.activePage.paneMeta[m.activePage.focused].CWD != "/repo/feature-a" {
		t.Fatalf("expected shell cwd to be worktree path, got %q", m.activePage.paneMeta[m.activePage.focused].CWD)
	}
}

func TestWorktreePageSplitDoesNotAffectOverview(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	worktreePage := m.activePage

	m.Update(keyCtrlBackslash())

	if got := len(layout.LeafOrder(worktreePage.bodyTree)); got != 4 {
		t.Fatalf("expected 4 panes in worktree page after split, got %d", got)
	}
	m.switchToOverviewPage()
	overviewLeaves := len(layout.LeafOrder(m.activePage.bodyTree))
	if overviewLeaves != 4 {
		t.Fatalf("expected overview page to have 4 leaves, got %d", overviewLeaves)
	}
}

func TestWorktreePageSnapshotRestoresLayoutAndFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))

	m.Update(keyCtrlBackslash())
	worktreeLeavesAfterSplit := len(layout.LeafOrder(m.activePage.bodyTree))
	if worktreeLeavesAfterSplit != 4 {
		t.Fatalf("expected 4 panes after split, got %d", worktreeLeavesAfterSplit)
	}

	m.switchToOverviewPage()
	m.switchToWorktreePage("/repo/feature-a", "")

	restoredLeaves := len(layout.LeafOrder(m.activePage.bodyTree))
	if restoredLeaves != worktreeLeavesAfterSplit {
		t.Fatalf("expected restored layout to have %d panes, got %d", worktreeLeavesAfterSplit, restoredLeaves)
	}
	if m.activePage.focused != paneShell && m.activePage.paneMeta[m.activePage.focused].Type != models.PaneTypeShell {
		t.Fatalf("expected focus to restore to a shell pane, got %s", m.activePage.focused)
	}
}

func TestRenderHelpLineForDiffPaneIncludesReviewCloseShortcut(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.panes[paneGitDiff] = &fakePanel{}
	m.activePage.paneMeta[paneGitDiff] = models.PaneMeta{ID: paneGitDiff, Name: "Diff", Type: models.PaneTypeDiffView, Closable: true}
	m.activePage.bodyTree = layout.SplitLeaf(m.activePage.bodyTree, paneShell, paneGitDiff, layout.SplitHorizontal, true)
	m.activePage.focused = paneGitDiff

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
		activePage: &page{
			panes: map[models.PaneID]models.Panel{
				"shell-1": target,
				"shell-2": other,
			},
			paneMeta: map[models.PaneID]models.PaneMeta{
				"shell-1": {ID: "shell-1", Type: models.PaneTypeShell},
				"shell-2": {ID: "shell-2", Type: models.PaneTypeShell},
			},
			paneOrder: []models.PaneID{"shell-1", "shell-2"},
		},
	}

	updated, _ := m.Update(shell.RefreshMsg{PaneID: "shell-1"})
	m = updated.(model)

	targetPanel := m.activePage.panes["shell-1"].(*fakePanel)
	otherPanel := m.activePage.panes["shell-2"].(*fakePanel)
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

	initialOrder := layout.LeafOrder(m.activePage.bodyTree)
	if len(initialOrder) != 4 {
		t.Fatalf("expected 4 panes initially (shell + worktree + git + file-tree), got %d", len(initialOrder))
	}

	newM, cmd := m.Update(keyCtrlBackslash())
	if cmd == nil {
		t.Fatal("expected cmd from split, got nil")
	}
	m = newM.(model)

	newOrder := layout.LeafOrder(m.activePage.bodyTree)
	if len(newOrder) != 5 {
		t.Fatalf("expected 5 panes after split, got %d", len(newOrder))
	}

	foundNewPane := false
	for _, id := range newOrder {
		if string(id) != string(paneShell) && string(id) != string(paneWorktree) && string(id) != string(paneGitStatus) && string(id) != string(paneFileTree) {
			foundNewPane = true
			if m.activePage.focused != id {
				t.Fatalf("expected focus on new pane %s, got %s", id, m.activePage.focused)
			}
			if meta, ok := m.activePage.paneMeta[id]; !ok || meta.Type != models.PaneTypeShell {
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

	initialOrder := layout.LeafOrder(m.activePage.bodyTree)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = newM.(model)

	if m.activePage.zoomedPane != paneShell {
		t.Fatalf("expected zoomed pane to be shell, got %s", m.activePage.zoomedPane)
	}

	zoomedOrder := layout.LeafOrder(m.activePage.bodyTree)
	if len(zoomedOrder) != 1 {
		t.Fatalf("expected 1 pane when zoomed, got %d", len(zoomedOrder))
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = newM.(model)

	if m.activePage.zoomedPane != "" {
		t.Fatalf("expected zoom to be restored, got %s", m.activePage.zoomedPane)
	}

	restoredOrder := layout.LeafOrder(m.activePage.bodyTree)
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
	firstFocused := m.activePage.focused
	if meta, ok := m.activePage.paneMeta[firstFocused]; !ok || meta.Type != models.PaneTypeEditor {
		t.Fatalf("expected focused pane to be editor, got %v", meta.Type)
	}
	leafCount := len(layout.LeafOrder(m.activePage.bodyTree))

	cmd = m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	if cmd != nil {
		t.Fatalf("expected no init command when reusing existing editor")
	}
	if len(layout.LeafOrder(m.activePage.bodyTree)) != leafCount {
		t.Fatalf("expected leaf count to remain %d when reusing editor", leafCount)
	}
	if m.activePage.focused != firstFocused {
		t.Fatalf("expected focus to stay on reused editor %s, got %s", firstFocused, m.activePage.focused)
	}
}

func TestEditorIsolationAcrossWorktreePages(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	path := filepath.Join(t.TempDir(), "main.go")

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	cmd := m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	if cmd == nil {
		t.Fatalf("expected init command for first editor open")
	}
	pageA := m.pages["/repo/feature-a"]
	editorA := pageA.focused

	m.switchToWorktreePage("/repo/feature-b", string(paneShell))
	cmd = m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	if cmd == nil {
		t.Fatalf("expected init command for second editor open in different worktree")
	}
	pageB := m.pages["/repo/feature-b"]
	editorB := pageB.focused

	if pageA.panes[editorA] == pageB.panes[editorB] {
		t.Fatalf("expected different editor pane instances for same file in different worktrees")
	}

	m.switchToWorktreePage("/repo/feature-a", "")
	if _, ok := m.activePage.paneMeta[editorA]; !ok {
		t.Fatalf("expected editor A to remain in worktree A page")
	}
}

func TestEditorHostPaneTargetPrefersLastEditorFromFileTree(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	path := filepath.Join(t.TempDir(), "main.go")
	m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	firstEditor := m.activePage.focused
	m.setFocus(paneFileTree)

	target := m.editorHostPaneTarget(m.activePage.focused, editorplugin.OpenBehaviorDefault)
	if target != firstEditor {
		t.Fatalf("expected default tree open to target existing editor %s, got %s", firstEditor, target)
	}

	vsplitTarget := m.editorHostPaneTarget(m.activePage.focused, editorplugin.OpenBehaviorVSplit)
	if vsplitTarget != firstEditor {
		t.Fatalf("expected vsplit tree open to target existing editor %s, got %s", firstEditor, vsplitTarget)
	}
}

func TestEditorSplitDirectionDistinguishesDefaultAndVSplit(t *testing.T) {
	m := model{
		activePage: &page{
			paneMeta: map[models.PaneID]models.PaneMeta{
				"editor-1": {ID: "editor-1", Type: models.PaneTypeEditor},
				paneShell:  {ID: paneShell, Type: models.PaneTypeShell},
			},
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
	editorID := m.activePage.focused

	m.closePane(editorID)

	if m.activePage.focused != paneFileTree {
		t.Fatalf("expected focus to return to file tree, got %s", m.activePage.focused)
	}
	if _, ok := m.activePage.paneMeta[editorID]; ok {
		t.Fatalf("expected editor pane %s to be removed", editorID)
	}
}

func TestOpenDiffPaneAddsBodyPaneAndRestoresOpenerFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneGitStatus)
	initialLeaves := len(layout.LeafOrder(m.activePage.bodyTree))

	cmd := m.openDiffPane(gitplugin.OpenDiffMsg{FilePath: "", Staged: false})
	if cmd == nil {
		t.Fatalf("expected init command for diff pane")
	}
	if m.activePage.focused != paneGitDiff {
		t.Fatalf("expected diff pane to be focused, got %s", m.activePage.focused)
	}
	if got := len(layout.LeafOrder(m.activePage.bodyTree)); got != initialLeaves+1 {
		t.Fatalf("expected leaf count %d after opening diff pane, got %d", initialLeaves+1, got)
	}
	if meta, ok := m.activePage.paneMeta[paneGitDiff]; !ok || meta.Type != models.PaneTypeDiffView {
		t.Fatalf("expected diff pane metadata to be registered")
	}

	m.closePane(paneGitDiff)
	if m.activePage.focused != paneGitStatus {
		t.Fatalf("expected focus to return to git status, got %s", m.activePage.focused)
	}
}

func TestOpenWorktreeShellAddsScopedShellPane(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneWorktree)

	cmd := m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	if cmd == nil {
		t.Fatalf("expected init command for worktree shell")
	}
	if got := len(layout.LeafOrder(m.activePage.bodyTree)); got != 4 {
		t.Fatalf("expected leaf count 4 after opening worktree shell, got %d", got)
	}
	if m.activePage.focused == paneWorktree {
		t.Fatalf("expected focus to move to new shell")
	}
	meta, ok := m.activePage.paneMeta[m.activePage.focused]
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
	if sh, ok := m.pane(m.activePage.focused).(*shell.Model); !ok || sh == nil {
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
	if m.activePage.focused != paneWorktreeCreate {
		t.Fatalf("expected focus on create overlay, got %s", m.activePage.focused)
	}

	m.closePane(paneWorktreeCreate)
	if m.activePage.focused != paneWorktree {
		t.Fatalf("expected focus to restore to worktree pane, got %s", m.activePage.focused)
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

	updatedModel, cmd := m.Update(gitplugin.WorktreeCreatedMsg{ID: paneWorktreeCreate, Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	m = updatedModel.(model)
	if cmd == nil {
		t.Fatalf("expected batched commands after worktree creation")
	}
	if got := len(layout.LeafOrder(m.activePage.bodyTree)); got != 4 {
		t.Fatalf("expected leaf count 4 after opening new worktree shell, got %d", got)
	}
	if m.activePage.paneMeta[m.activePage.focused].Type != models.PaneTypeShell {
		t.Fatalf("expected focus on shell after worktree creation, got %v", m.activePage.paneMeta[m.activePage.focused].Type)
	}
	if m.activePage.paneMeta[m.activePage.focused].CWD != "/repo/feature-a" {
		t.Fatalf("expected created shell cwd /repo/feature-a, got %q", m.activePage.paneMeta[m.activePage.focused].CWD)
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
	removedShell := m.activePage.focused
	if m.activePage.paneMeta[removedShell].Type != models.PaneTypeShell {
		t.Fatalf("expected focused pane to be shell")
	}

	updatedModel, _ := m.Update(gitplugin.WorktreeRemovedMsg{Path: "/repo/feature-a"})
	m = updatedModel.(model)
	if _, ok := m.activePage.paneMeta[removedShell]; ok {
		t.Fatalf("expected removed worktree shell pane to be closed")
	}
	if m.activePage.focused == removedShell {
		t.Fatalf("expected focus to move away from removed shell")
	}
}

func TestHandleMouseRoutesWheelToDiffPaneWithoutStealingFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.panes[paneGitDiff] = &fakePanel{}
	m.activePage.paneMeta[paneGitDiff] = models.PaneMeta{ID: paneGitDiff, Name: "Diff", Type: models.PaneTypeDiffView, Closable: true}
	m.activePage.bodyTree = layout.SplitLeaf(m.activePage.bodyTree, paneShell, paneGitDiff, layout.SplitHorizontal, true)
	m.activePage.focused = paneGitStatus
	m.common.Width = 120
	m.common.Height = 40
	m.updateSizes(120, 40)
	frame := m.activePage.frames[paneGitDiff]
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
	if m.activePage.focused != paneGitStatus {
		t.Fatalf("expected wheel scroll not to steal focus, got %s", m.activePage.focused)
	}
}

func TestSyncPaneMetaMarksDirtyEditorName(t *testing.T) {
	m := model{
		activePage: &page{
			panes: map[models.PaneID]models.Panel{
				"editor-1": &fakeEditorMetaPanel{filePath: "/tmp/main.go", dirty: true, name: "main.go"},
			},
			paneMeta: map[models.PaneID]models.PaneMeta{
				"editor-1": {ID: "editor-1", Name: "main.go", Type: models.PaneTypeEditor, Closable: true},
			},
		},
	}

	m.syncPaneMeta("editor-1")
	if got := m.activePage.paneMeta["editor-1"].Name; got != "*main.go" {
		t.Fatalf("expected dirty editor title '*main.go', got %q", got)
	}
}
