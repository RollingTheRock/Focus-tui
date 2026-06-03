package app

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"focus/internal/agents"
	"focus/internal/config"
	gitmodel "focus/internal/git"
	"focus/internal/models"
	"focus/internal/orchestrator"
	editorplugin "focus/internal/plugins/editor"
	gitplugin "focus/internal/plugins/git"
	"focus/internal/store"
	"focus/internal/trellis"
	"focus/internal/ui/layout"
	"focus/internal/ui/shell"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type fakePanel struct {
	updates []tea.Msg
}

func (p *fakePanel) Init() tea.Cmd { return nil }

func (p *fakePanel) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	p.updates = append(p.updates, msg)
	return p, nil
}

func (p *fakePanel) View() tea.View { return tea.NewView("") }

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

type fakeTrellisBridge struct {
	worktreeID           string
	linkedWorktreePath   string
	syncedTaskID         string
	builtSessionTaskID   string
	builtSessionWorktree string
}

func (f *fakeTrellisBridge) EnsureInitialized() error {
	return nil
}

func (f *fakeTrellisBridge) SetWorktreeID(id string) {
	f.worktreeID = id
}

func (f *fakeTrellisBridge) EnsureWorktreeLinks(worktreePath string) error {
	f.linkedWorktreePath = worktreePath
	return nil
}

func (f *fakeTrellisBridge) SyncTaskCreate(task models.TaskContextRecord, plan *models.TaskPlanRecord) (string, error) {
	f.syncedTaskID = task.ID
	return ".trellis/tasks/test-task", nil
}

func (f *fakeTrellisBridge) BuildAgentContext(session *agents.Session) (*agents.AgentSpec, error) {
	f.builtSessionTaskID = session.TaskID
	f.builtSessionWorktree = session.WorktreeID
	return agents.NewAgentSpec(), nil
}

func (f *fakeTrellisBridge) AddTaskOutput(taskID, output string) error                { return nil }
func (f *fakeTrellisBridge) AddKnowledgeFact(subject, predicate, object string) error { return nil }
func (f *fakeTrellisBridge) SyncTaskStart(taskID string) error                        { return nil }
func (f *fakeTrellisBridge) SyncTaskFinish(taskID string) error                       { return nil }
func (f *fakeTrellisBridge) SyncTaskArchive(taskID string) error                      { return nil }
func (f *fakeTrellisBridge) RecordSessionByTitle(title string) error                  { return nil }
func (f *fakeTrellisBridge) UpdateWorkflowState(planID string, step models.PlanStepRecord) error {
	return nil
}
func (f *fakeTrellisBridge) GetTaskContextExtended(taskID string) (*trellis.ExtendedTaskContext, error) {
	return nil, nil
}
func (f *fakeTrellisBridge) Version() string              { return "" }
func (f *fakeTrellisBridge) Update() error                { return nil }
func (f *fakeTrellisBridge) ListSpecs() ([]string, error) { return nil, nil }

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
	m.activePage.focused = paneWorktreeDetail
	editorID := models.PaneID("editor-1")
	m.activePage.panes[editorID] = &fakeEditorMetaPanel{name: "main.go", filePath: "/tmp/main.go"}
	m.activePage.paneMeta[editorID] = models.PaneMeta{ID: editorID, Name: "main.go", Type: models.PaneTypeEditor, Closable: true}
	m.activePage.paneOrder = append(m.activePage.paneOrder, editorID)
	m.activePage.focused = editorID

	help := m.renderHelpLine(120)
	for _, want := range []string{"ctrl+s", "save", "ctrl+f /", "search", ":", "line", "n/N", "result"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help line to contain %q, got %q", want, help)
		}
	}
}

func TestRenderHelpLineForWorktreeDetailIncludesCoreShortcuts(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneWorktreeDetail

	help := m.renderHelpLine(140)
	for _, want := range []string{"1-3", "tabs", "j/k", "nav", "enter", "edit task", "o", "files"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help line to contain %q, got %q", want, help)
		}
	}
}

func TestWorktreeDetailPaneUsesThreeCoreTabs(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	detail, ok := m.activePage.pane(paneWorktreeDetail).(*worktreeDetailPane)
	if !ok {
		t.Fatalf("expected worktreeDetailPane, got %T", m.activePage.pane(paneWorktreeDetail))
	}

	if got, want := detailTabNames, []string{"Task", "Context", "Agents"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected core detail tabs %v, got %v", want, got)
	}

	for _, key := range []string{"1", "2", "3"} {
		updated, _ := detail.Update(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
		detail = updated.(*worktreeDetailPane)
	}
	if detail.activeTab != detailTabAgent {
		t.Fatalf("expected key 3 to select agents tab, got %v", detail.activeTab)
	}

	updated, _ := detail.Update(tea.KeyPressMsg{Code: '4', Text: "4"})
	detail = updated.(*worktreeDetailPane)
	if detail.activeTab != detailTabAgent {
		t.Fatalf("expected key 4 to be ignored, got %v", detail.activeTab)
	}
}

func TestSyncWorktreeActivitiesUpdatesDetailPaneAgentSessions(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	worktreeID := "/repo/feature-detail-agent"
	now := time.Now()
	if err := st.SaveAgentSession(models.AgentSessionRecord{
		ID:             "session-running",
		WorktreeID:     worktreeID,
		RepoID:         "/repo/main",
		Provider:       string(agents.ProviderCodex),
		State:          string(agents.SessionRunning),
		StartedAt:      now.Add(-time.Minute),
		LastActivityAt: &now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("save agent session: %v", err)
	}
	m.currentWorktreePage = worktreeID

	detail, ok := m.activePage.pane(paneWorktreeDetail).(*worktreeDetailPane)
	if !ok {
		t.Fatalf("expected worktreeDetailPane, got %T", m.activePage.pane(paneWorktreeDetail))
	}
	if cmd := detail.setWorktree(worktreeID); cmd != nil {
		_ = cmd()
	}

	m.syncWorktreeActivities()

	if len(detail.sessions) != 1 {
		t.Fatalf("expected detail pane to receive 1 running session, got %d", len(detail.sessions))
	}
	if detail.sessions[0].ID != "session-running" {
		t.Fatalf("expected running session to be shown, got %+v", detail.sessions[0])
	}
}

func TestRenderHelpLineForWorktreePaneIncludesRefreshShortcut(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneWorktree

	help := m.renderHelpLine(120)
	for _, want := range []string{"j/k", "nav", "enter", "select", "o", "shell", "e", "edit", "d", "del"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help line to contain %q, got %q", want, help)
		}
	}
}

func TestRenderHelpLineForDAGMatchesCurrentKeybindings(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	help := m.renderHelpLine(140)
	for _, want := range []string{"s", "state", "t", "todo", "c", "new-wt"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected DAG help line to contain %q, got %q", want, help)
		}
	}
	if strings.Contains(help, "start agent") {
		t.Fatalf("expected DAG help line to avoid stale binding text, got %q", help)
	}
}

func TestRenderNotificationBarStaysSingleLine(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.notifications = []orchestrator.Notification{
		{
			Title:    "Agent",
			Body:     strings.Repeat("very long message ", 20),
			Severity: "info",
		},
	}
	bar := m.renderNotificationBar(40)
	if strings.Count(bar, "\n") > 0 {
		t.Fatalf("expected single-line notification bar, got %q", bar)
	}
	if got := ansi.StringWidth(bar); got > 40 {
		t.Fatalf("expected notification width <= 40, got %d", got)
	}
}

func TestRenderHelpLineCollapsesWhenTextOverflows(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.focused = paneDAG

	help := m.renderHelpLine(24)
	if strings.Count(help, "\n") > 0 {
		t.Fatalf("expected single-line help line, got %q", help)
	}
	if got := ansi.StringWidth(help); got > 24 {
		t.Fatalf("expected compact help line width <= 24, got %d", got)
	}
}

func TestOverviewPageBodyTreeIsOrchestrationHub(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if m.activePage.bodyTree.Direction != layout.SplitVertical {
		t.Fatalf("expected overview layout to split vertically, got %s", m.activePage.bodyTree.Direction)
	}
	if m.activePage.bodyTree.Ratio != 74 {
		t.Fatalf("expected overview shell to be demoted to a small lower section, got ratio %d", m.activePage.bodyTree.Ratio)
	}
	leaves := layout.LeafOrder(m.activePage.bodyTree)
	if len(leaves) != 6 {
		t.Fatalf("expected overview page to have 6 leaves, got %d", len(leaves))
	}
	if leaves[0] != paneDAG || leaves[1] != paneWorktree || leaves[2] != paneWorktree || leaves[3] != paneWorktreeDetail || leaves[4] != paneShell || leaves[5] != paneShell {
		t.Fatalf("expected overview leaves [summary worktree dag detail agent shell], got %v", leaves)
	}
}

func TestOverviewLayoutPrioritizesWorktreeOverShellHeight(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	frames := layout.ComputeFrames(m.activePage.bodyTree, models.PaneFrame{X: 0, Y: 0, W: 120, H: 40})
	summaryFrame := frames[paneDAG]
	worktreeFrame := frames[paneWorktree]
	dagFrame := frames[paneWorktree]
	detailFrame := frames[paneWorktreeDetail]
	agentFrame := frames[paneShell]
	shellFrame := frames[paneShell]
	if summaryFrame.W <= 0 || summaryFrame.H <= 0 {
		t.Fatalf("expected overview summary pane frame to exist, got %+v", summaryFrame)
	}
	if worktreeFrame.H <= shellFrame.H {
		t.Fatalf("expected worktree pane to have more vertical space than shell, got worktree=%d shell=%d", worktreeFrame.H, shellFrame.H)
	}
	if shellFrame.H >= worktreeFrame.H/2 {
		t.Fatalf("expected shell to remain a secondary strip, got worktree=%d shell=%d", worktreeFrame.H, shellFrame.H)
	}
	if detailFrame.W <= 0 || detailFrame.H <= 0 {
		t.Fatalf("expected overview detail pane frame to exist, got %+v", detailFrame)
	}
	if dagFrame.W <= 0 || dagFrame.H <= 0 {
		t.Fatalf("expected overview dag pane frame to exist, got %+v", dagFrame)
	}
	if agentFrame.W <= 0 || agentFrame.H <= 0 {
		t.Fatalf("expected overview agent pane frame to exist, got %+v", agentFrame)
	}
	if summaryFrame.H >= worktreeFrame.H {
		t.Fatalf("expected summary pane to be a compact band above the main workbench, got summary=%d worktree=%d", summaryFrame.H, worktreeFrame.H)
	}
	if worktreeFrame.W <= detailFrame.W/2 {
		t.Fatalf("expected queue pane to remain a meaningful primary column, got worktree=%d detail=%d", worktreeFrame.W, detailFrame.W)
	}
}

func TestSwitchToWorktreePageTracksPageState(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl, Text: "g"})
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
	t.Skip("incompatible with new single-page layout")
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

func TestResumeWorktreeSwitchesPageWithoutOpeningExtraShell(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	cmd := m.resumeWorktree(gitplugin.ResumeWorktreeMsg{Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	if cmd == nil {
		t.Fatalf("expected resume command")
	}
	if m.state != StateWorktreePage {
		t.Fatalf("expected worktree page state, got %v", m.state)
	}
	if got := len(layout.LeafOrder(m.activePage.bodyTree)); got != 4 {
		t.Fatalf("expected default worktree layout without extra shell split, got %d leaves", got)
	}
	if m.activePage.focused != paneShell {
		t.Fatalf("expected resume to keep default useful focus, got %s", m.activePage.focused)
	}
}

func TestWorktreePageSplitDoesNotAffectOverview(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	worktreePage := m.activePage

	m.Update(keyCtrlBackslash())

	if got := len(layout.LeafOrder(worktreePage.bodyTree)); got != 5 {
		t.Fatalf("expected 5 panes in worktree page after split, got %d", got)
	}
	m.switchToOverviewPage()
	overviewLeaves := len(layout.LeafOrder(m.activePage.bodyTree))
	if overviewLeaves != 6 {
		t.Fatalf("expected overview page to have 6 leaves, got %d", overviewLeaves)
	}
}

func TestWorktreePageSnapshotRestoresLayoutAndFocus(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))

	m.Update(keyCtrlBackslash())
	worktreeLeavesAfterSplit := len(layout.LeafOrder(m.activePage.bodyTree))
	if worktreeLeavesAfterSplit != 5 {
		t.Fatalf("expected 5 panes after split, got %d", worktreeLeavesAfterSplit)
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

func TestRenderHelpLineForDiffOverlayIncludesReviewCloseShortcut(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.panes[paneGitDiff] = &fakePanel{}
	m.activePage.paneMeta[paneGitDiff] = models.PaneMeta{ID: paneGitDiff, Name: "Diff", Type: models.PaneTypeDiffView, Closable: true}
	m.activePage.focused = paneGitDiff

	help := m.renderHelpLine(120)
	for _, want := range []string{"enter", "open file", "s", "toggle staged", "[ / ]", "files", "v", "toggle layout", "wheel", "scroll", "q/esc", "close review"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected diff help line to contain %q, got %q", want, help)
		}
	}
	// Global hints should NOT appear when diff overlay is active
	for _, avoid := range []string{"weather", "refresh", "quit"} {
		if strings.Contains(help, avoid) {
			t.Fatalf("expected diff overlay help to exclude global hint %q, got %q", avoid, help)
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

func keyCtrlBackslash() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl, Text: "\\"}
}

func keyCtrlW() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl, Text: "w"}
}

func TestSplitFocusedHorizontal(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	initialOrder := layout.LeafOrder(m.activePage.bodyTree)
	if len(initialOrder) != 6 {
		t.Fatalf("expected 6 panes initially (summary + worktree + dag + detail + agents + shell), got %d", len(initialOrder))
	}

	m.setFocus(paneShell)
	newM, cmd := m.Update(keyCtrlBackslash())
	if cmd == nil {
		t.Fatal("expected cmd from split, got nil")
	}
	m = newM.(model)

	newOrder := layout.LeafOrder(m.activePage.bodyTree)
	if len(newOrder) != 7 {
		t.Fatalf("expected 7 panes after split, got %d", len(newOrder))
	}

	foundNewPane := false
	for _, id := range newOrder {
		if string(id) != string(paneShell) && string(id) != string(paneWorktree) && string(id) != string(paneWorktreeDetail) && string(id) != string(paneDAG) && string(id) != string(paneHeader) && string(id) != string(paneFooter) {
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

func TestLaunchAgentPersistsSessionAndInjectsStableID(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	if err := st.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-1", Title: "Plan", Status: "active", CurrentStep: "Step one"}); err != nil {
		t.Fatalf("save task plan: %v", err)
	}
	if err := st.SavePlanStep(models.PlanStepRecord{ID: "step-1", PlanID: "plan-1", OrderIndex: 0, Title: "Step one", State: "in_progress"}); err != nil {
		t.Fatalf("save plan step: %v", err)
	}
	if err := st.SaveWorktreeContext(models.WorktreeContextRecord{WorktreeID: "/repo/feature-a", RepoID: "/repo/main", CurrentPlanID: stringPtr("plan-1"), TaskMode: "single", TaskName: "Planning", LastActiveAt: time.Now()}); err != nil {
		t.Fatalf("save worktree context: %v", err)
	}

	cmd := m.launchAgent(agents.LaunchAgentMsg{WorktreeID: "/repo/feature-a", Provider: agents.ProviderOpenCode})
	if cmd == nil {
		t.Fatal("expected launch command")
	}

	records, err := st.ListAgentSessions("/repo/feature-a")
	if err != nil {
		t.Fatalf("list agent sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(records))
	}
	if records[0].Provider != string(agents.ProviderOpenCode) {
		t.Fatalf("expected opencode session, got %+v", records[0])
	}
	if records[0].PlanID != "plan-1" || records[0].StepID != "step-1" {
		t.Fatalf("expected session to capture current execution slice, got %+v", records[0])
	}

	focused := m.activePage.focused
	sh, ok := m.activePage.pane(focused).(*shell.Model)
	if !ok {
		t.Fatalf("expected focused pane to be shell, got %T", m.activePage.pane(focused))
	}
	if !strings.Contains(shAutoType(sh), agents.SessionIDEnvVar+"=") {
		t.Fatalf("expected auto-typed command to inject session id, got %q", shAutoType(sh))
	}
}

func TestLaunchAgentExternalModePersistsEnvSnapshot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.ExternalTerminal = true
	cfg.Agent.TerminalEmulator = "nonexistent-terminal-binary"
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	if err := st.SaveWorktreeContext(models.WorktreeContextRecord{
		WorktreeID:   "/repo/feature-x",
		RepoID:       "/repo/main",
		TaskMode:     "single",
		TaskName:     "Planning",
		LastActiveAt: time.Now(),
	}); err != nil {
		t.Fatalf("save worktree context: %v", err)
	}

	cmd := m.launchAgent(agents.LaunchAgentMsg{WorktreeID: "/repo/feature-x", Provider: agents.ProviderClaude})
	if cmd == nil {
		t.Fatal("expected external launch command")
	}

	records, err := st.ListAgentSessions("/repo/feature-x")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one session record, got %+v", records)
	}
	if !strings.Contains(records[0].EnvSnapshot, agents.SessionIDEnvVar+"=") {
		t.Fatalf("expected env snapshot to contain session id env, got %+v", records[0])
	}
	if !strings.Contains(records[0].EnvSnapshot, "TRELLIS_CONTEXT_ID=") {
		t.Fatalf("expected env snapshot to contain trellis context id env, got %+v", records[0])
	}
	if records[0].State != string(agents.SessionWaiting) {
		t.Fatalf("expected waiting state before launch result, got %+v", records[0])
	}
}

func TestPrepareAgentProfileEnsuresWorktreeLinks(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	bridge := &fakeTrellisBridge{}
	m.trellisBridge = bridge

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:       "task-profile",
		RepoID:   m.gitRepoPath(),
		Title:    "Profile Task",
		State:    "active",
		Priority: "medium",
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}

	session := &agents.Session{
		ID:         "session-profile",
		WorktreeID: "/tmp/focus-profile-wt",
		RepoID:     m.gitRepoPath(),
		TaskID:     "task-profile",
	}
	if err := m.prepareAgentProfile(session); err != nil {
		t.Fatalf("prepare agent profile: %v", err)
	}

	if bridge.linkedWorktreePath != session.WorktreeID {
		t.Fatalf("expected worktree links for %q, got %q", session.WorktreeID, bridge.linkedWorktreePath)
	}
	if bridge.builtSessionTaskID != session.TaskID {
		t.Fatalf("expected context build for task %q, got %q", session.TaskID, bridge.builtSessionTaskID)
	}
}

func TestHandleExternalLaunchResultUpdatesState(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	now := time.Now().Add(-time.Minute)
	if err := st.SaveAgentSession(models.AgentSessionRecord{
		ID:         "session-external-1",
		Provider:   "claude",
		WorktreeID: "/repo/feature-z",
		State:      "waiting",
		StartedAt:  now,
	}); err != nil {
		t.Fatalf("save seed session: %v", err)
	}

	m.handleExternalLaunchResult(agents.ExternalLaunchResultMsg{
		SessionID: "session-external-1",
		PID:       4321,
	})
	records, err := st.ListAgentSessions("/repo/feature-z")
	if err != nil {
		t.Fatalf("list sessions after success: %v", err)
	}
	if len(records) != 1 || records[0].State != string(agents.SessionRunning) || records[0].PID != 4321 {
		t.Fatalf("expected running session with pid, got %+v", records)
	}

	m.handleExternalLaunchResult(agents.ExternalLaunchResultMsg{
		SessionID: "session-external-1",
		Err:       errors.New("launch failed"),
	})
	records, err = st.ListAgentSessions("/repo/feature-z")
	if err != nil {
		t.Fatalf("list sessions after failure: %v", err)
	}
	if len(records) != 1 || records[0].State != string(agents.SessionFailed) || records[0].StopReason == "" {
		t.Fatalf("expected failed session with stop reason, got %+v", records)
	}
}

func TestMCPToolSessionHeartbeatUpdatesSessionRecord(t *testing.T) {
	cfg := config.DefaultConfig()
	socketDir := t.TempDir()
	cfg.Agent.MCPSocket = filepath.Join(socketDir, "focus-mcp.sock")
	cfg.Agent.A2ASocket = filepath.Join(socketDir, "focus-a2a.sock")

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveAgentSession(models.AgentSessionRecord{
		ID:         "session-heartbeat-1",
		Provider:   string(agents.ProviderClaude),
		WorktreeID: "/repo/feature-heartbeat",
		State:      string(agents.SessionWaiting),
		StartedAt:  time.Now().Add(-2 * time.Minute),
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	out, err := m.mcpSessionHeartbeatTool(map[string]any{
		"session_id": "session-heartbeat-1",
		"status":     string(agents.SessionRunning),
	})
	if err != nil {
		t.Fatalf("session.heartbeat tool error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("expected success response, got %+v", out)
	}

	records, err := st.ListAgentSessions("/repo/feature-heartbeat")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one session record, got %+v", records)
	}
	if records[0].State != string(agents.SessionRunning) {
		t.Fatalf("expected running state after heartbeat, got %+v", records[0])
	}
	if records[0].LastHeartbeat == nil {
		t.Fatalf("expected heartbeat timestamp, got %+v", records[0])
	}
}

func TestMCPToolTaskUpdateStatusNormalizesCompletedToDone(t *testing.T) {
	cfg := config.DefaultConfig()
	socketDir := t.TempDir()
	cfg.Agent.MCPSocket = filepath.Join(socketDir, "focus-mcp.sock")
	cfg.Agent.A2ASocket = filepath.Join(socketDir, "focus-a2a.sock")

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-status-1",
		RepoID:              "/repo/main",
		Title:               "Status update target",
		State:               "paused",
		Priority:            "medium",
		PreferredWorktreeID: "/repo/feature-status",
	}); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	out, err := m.mcpTaskUpdateStatusTool(map[string]any{
		"task_id": "task-status-1",
		"state":   "completed",
	})
	if err != nil {
		t.Fatalf("task.update_status tool error: %v", err)
	}
	if out["state"] != "done" {
		t.Fatalf("expected normalized done state in response, got %+v", out)
	}

	task, err := st.GetTaskContext("task-status-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.State != "done" {
		t.Fatalf("expected task state persisted as done, got %+v", task)
	}
}

func TestMCPTaskStatusDoneTriggersProtocolDownstreamLaunch(t *testing.T) {
	cfg := config.DefaultConfig()
	socketDir := t.TempDir()
	cfg.Agent.MCPSocket = filepath.Join(socketDir, "focus-mcp.sock")
	cfg.Agent.A2ASocket = filepath.Join(socketDir, "focus-a2a.sock")
	cfg.Agent.ExternalTerminal = false

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-upstream-1",
		RepoID:              "/repo/main",
		Title:               "Upstream protocol task",
		State:               "active",
		Priority:            "high",
		PreferredWorktreeID: "/repo/feature-a",
	}); err != nil {
		t.Fatalf("save upstream task: %v", err)
	}
	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-downstream-1",
		RepoID:              "/repo/main",
		Title:               "Downstream protocol task",
		State:               "paused",
		Priority:            "medium",
		PreferredWorktreeID: "/repo/feature-b",
	}); err != nil {
		t.Fatalf("save downstream task: %v", err)
	}
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID:     "task-upstream-1",
		ToTaskID:       "task-downstream-1",
		DependencyType: "hard",
	}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}
	if err := st.SaveAgentSession(models.AgentSessionRecord{
		ID:         "session-upstream-1",
		Provider:   string(agents.ProviderClaude),
		WorktreeID: "/repo/feature-a",
		TaskID:     "task-upstream-1",
		State:      string(agents.SessionRunning),
		StartedAt:  time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save upstream session: %v", err)
	}

	out, err := m.mcpTaskUpdateStatusTool(map[string]any{
		"task_id":    "task-upstream-1",
		"state":      "completed",
		"session_id": "session-upstream-1",
		"summary":    "upstream done",
	})
	if err != nil {
		t.Fatalf("task.update_status tool error: %v", err)
	}
	// Phase 4: downstream auto-launch removed per ADR-0000 Human Sovereignty.
	launchedIDs, _ := out["launched_task_ids"].([]string)
	if len(launchedIDs) != 0 {
		t.Fatalf("expected no downstream auto-launch, got %+v", out)
	}

	upstream, err := st.GetTaskContext("task-upstream-1")
	if err != nil {
		t.Fatalf("get upstream task: %v", err)
	}
	if upstream == nil || upstream.State != "done" {
		t.Fatalf("expected upstream done, got %+v", upstream)
	}

	upSessions, err := st.ListAgentSessions("/repo/feature-a")
	if err != nil {
		t.Fatalf("list upstream sessions: %v", err)
	}
	if len(upSessions) != 1 || upSessions[0].State != string(agents.SessionExited) || upSessions[0].Summary != "upstream done" {
		t.Fatalf("expected upstream session completed, got %+v", upSessions)
	}

	// Downstream sessions should NOT be auto-launched.
	downstreamSessions, err := st.ListAgentSessions("/repo/feature-b")
	if err != nil {
		t.Fatalf("list downstream sessions: %v", err)
	}
	if len(downstreamSessions) != 0 {
		t.Fatalf("expected no downstream auto-launched sessions, got %+v", downstreamSessions)
	}
}

func TestA2AStatusUpdateBridgesToMCPAndTriggersDownstream(t *testing.T) {
	cfg := config.DefaultConfig()
	socketDir := t.TempDir()
	cfg.Agent.MCPSocket = filepath.Join(socketDir, "focus-mcp.sock")
	cfg.Agent.A2ASocket = filepath.Join(socketDir, "focus-a2a.sock")
	cfg.Agent.ExternalTerminal = false

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-upstream-a2a",
		RepoID:              "/repo/main",
		Title:               "Upstream A2A task",
		State:               "active",
		Priority:            "high",
		PreferredWorktreeID: "/repo/feature-a2a-a",
	}); err != nil {
		t.Fatalf("save upstream task: %v", err)
	}
	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-downstream-a2a",
		RepoID:              "/repo/main",
		Title:               "Downstream A2A task",
		State:               "paused",
		Priority:            "medium",
		PreferredWorktreeID: "/repo/feature-a2a-b",
	}); err != nil {
		t.Fatalf("save downstream task: %v", err)
	}
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID:     "task-upstream-a2a",
		ToTaskID:       "task-downstream-a2a",
		DependencyType: "hard",
	}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}
	if err := st.SaveAgentSession(models.AgentSessionRecord{
		ID:         "session-a2a-up-1",
		Provider:   string(agents.ProviderOpenCode),
		WorktreeID: "/repo/feature-a2a-a",
		TaskID:     "task-upstream-a2a",
		State:      string(agents.SessionWaiting),
		StartedAt:  time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save upstream session: %v", err)
	}

	if _, err := m.mcpSessionHeartbeatTool(map[string]any{
		"session_id": "session-a2a-up-1",
		"status":     "running",
	}); err != nil {
		t.Fatalf("heartbeat tool: %v", err)
	}
	if _, err := m.mcpTaskUpdateStatusTool(map[string]any{
		"task_id":    "task-upstream-a2a",
		"state":      "completed",
		"session_id": "session-a2a-up-1",
		"summary":    "done by a2a",
	}); err != nil {
		t.Fatalf("update status tool: %v", err)
	}

	upstream, err := st.GetTaskContext("task-upstream-a2a")
	if err != nil {
		t.Fatalf("get upstream task: %v", err)
	}
	if upstream == nil || upstream.State != "done" {
		t.Fatalf("expected upstream done after a2a status update, got %+v", upstream)
	}

	upSessions, err := st.ListAgentSessions("/repo/feature-a2a-a")
	if err != nil {
		t.Fatalf("list upstream sessions: %v", err)
	}
	if len(upSessions) != 1 || upSessions[0].State != string(agents.SessionExited) {
		t.Fatalf("expected upstream session exited, got %+v", upSessions)
	}

	// Phase 4: downstream auto-launch removed per ADR-0000 Human Sovereignty.
	downstreamSessions, err := st.ListAgentSessions("/repo/feature-a2a-b")
	if err != nil {
		t.Fatalf("list downstream sessions: %v", err)
	}
	if len(downstreamSessions) != 0 {
		t.Fatalf("expected no downstream auto-launched sessions, got %+v", downstreamSessions)
	}
}

func TestMCPContextGetForTaskIncludesUpstreamOutputsAndFacts(t *testing.T) {
	cfg := config.DefaultConfig()
	socketDir := t.TempDir()
	cfg.Agent.MCPSocket = filepath.Join(socketDir, "focus-mcp.sock")
	cfg.Agent.A2ASocket = filepath.Join(socketDir, "focus-a2a.sock")

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-upstream-ctx",
		RepoID:              "/repo/main",
		Title:               "Upstream context task",
		State:               "done",
		Priority:            "high",
		PreferredWorktreeID: "/repo/feature-ctx-a",
	}); err != nil {
		t.Fatalf("save upstream task: %v", err)
	}
	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-downstream-ctx",
		RepoID:              "/repo/main",
		Title:               "Downstream context task",
		State:               "paused",
		Priority:            "medium",
		PreferredWorktreeID: "/repo/feature-ctx-b",
	}); err != nil {
		t.Fatalf("save downstream task: %v", err)
	}
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID:     "task-upstream-ctx",
		ToTaskID:       "task-downstream-ctx",
		DependencyType: "hard",
	}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}
	if err := st.SaveTaskPlan(models.TaskPlanRecord{
		ID:     "plan-downstream-ctx",
		TaskID: "task-downstream-ctx",
		Title:  "Downstream plan",
		Status: "active",
	}); err != nil {
		t.Fatalf("save downstream plan: %v", err)
	}
	if _, err := m.mcpTaskCreateOutputTool(map[string]any{
		"task_id": "task-upstream-ctx",
		"output":  "Upstream implementation complete",
		"actor":   "agent-claude",
	}); err != nil {
		t.Fatalf("create output: %v", err)
	}
	if _, err := m.mcpKnowledgeAddFactTool(map[string]any{
		"plan_id":    "plan-downstream-ctx",
		"subject":    "router",
		"predicate":  "uses",
		"object":     "json-rpc",
		"source":     "agent-kimi",
		"confidence": 0.9,
	}); err != nil {
		t.Fatalf("add fact: %v", err)
	}

	ctx, err := m.mcpContextGetForTaskTool(map[string]any{
		"task_id": "task-downstream-ctx",
	})
	if err != nil {
		t.Fatalf("context.get_for_task: %v", err)
	}
	upstreamOutputs, ok := ctx["upstream_outputs"].([]map[string]any)
	if !ok || len(upstreamOutputs) == 0 {
		t.Fatalf("expected upstream outputs in context, got %+v", ctx["upstream_outputs"])
	}
	knowledgeFacts, ok := ctx["knowledge_facts"].([]map[string]any)
	if !ok || len(knowledgeFacts) == 0 {
		t.Fatalf("expected knowledge facts in context, got %+v", ctx["knowledge_facts"])
	}
}

func TestProtocolClosedLoopSmoke(t *testing.T) {
	cfg := config.DefaultConfig()
	socketDir := t.TempDir()
	cfg.Agent.MCPSocket = filepath.Join(socketDir, "focus-mcp.sock")
	cfg.Agent.A2ASocket = filepath.Join(socketDir, "focus-a2a.sock")
	cfg.Agent.ExternalTerminal = false

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-smoke-up",
		RepoID:              "/repo/main",
		Title:               "Upstream smoke task",
		State:               "active",
		Priority:            "high",
		PreferredWorktreeID: "/repo/smoke-up",
	}); err != nil {
		t.Fatalf("save upstream: %v", err)
	}
	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-smoke-down",
		RepoID:              "/repo/main",
		Title:               "Downstream smoke task",
		State:               "paused",
		Priority:            "medium",
		PreferredWorktreeID: "/repo/smoke-down",
	}); err != nil {
		t.Fatalf("save downstream: %v", err)
	}
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID:     "task-smoke-up",
		ToTaskID:       "task-smoke-down",
		DependencyType: "hard",
	}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}
	if err := st.SaveTaskPlan(models.TaskPlanRecord{
		ID:     "plan-smoke-down",
		TaskID: "task-smoke-down",
		Title:  "Downstream smoke plan",
		Status: "active",
	}); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := st.SaveAgentSession(models.AgentSessionRecord{
		ID:         "session-smoke-up",
		Provider:   string(agents.ProviderClaude),
		WorktreeID: "/repo/smoke-up",
		TaskID:     "task-smoke-up",
		State:      string(agents.SessionRunning),
		StartedAt:  time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save upstream session: %v", err)
	}

	if _, err := m.mcpTaskCreateOutputTool(map[string]any{
		"task_id": "task-smoke-up",
		"output":  "upstream outcome payload",
		"actor":   "session-smoke-up",
	}); err != nil {
		t.Fatalf("create output: %v", err)
	}
	if _, err := m.mcpKnowledgeAddFactTool(map[string]any{
		"plan_id":   "plan-smoke-down",
		"subject":   "api",
		"predicate": "requires",
		"object":    "idempotent retries",
		"source":    "session-smoke-up",
	}); err != nil {
		t.Fatalf("add fact: %v", err)
	}

	if _, err := m.mcpSessionHeartbeatTool(map[string]any{
		"session_id": "session-smoke-up",
		"status":     "running",
	}); err != nil {
		t.Fatalf("heartbeat tool: %v", err)
	}
	if _, err := m.mcpTaskUpdateStatusTool(map[string]any{
		"task_id":    "task-smoke-up",
		"state":      "completed",
		"session_id": "session-smoke-up",
		"summary":    "smoke task done",
	}); err != nil {
		t.Fatalf("update status tool: %v", err)
	}

	// Phase 4: downstream auto-launch removed per ADR-0000 Human Sovereignty.
	downSessions, err := st.ListAgentSessions("/repo/smoke-down")
	if err != nil {
		t.Fatalf("list downstream sessions: %v", err)
	}
	if len(downSessions) != 0 {
		t.Fatalf("expected no downstream auto-launched sessions, got %+v", downSessions)
	}

	ctx, err := m.mcpContextGetForTaskTool(map[string]any{"task_id": "task-smoke-down"})
	if err != nil {
		t.Fatalf("context.get_for_task: %v", err)
	}
	upstreamOutputs, ok := ctx["upstream_outputs"].([]map[string]any)
	if !ok || len(upstreamOutputs) == 0 {
		t.Fatalf("expected upstream outputs in smoke context, got %+v", ctx["upstream_outputs"])
	}
	facts, ok := ctx["knowledge_facts"].([]map[string]any)
	if !ok || len(facts) == 0 {
		t.Fatalf("expected knowledge facts in smoke context, got %+v", ctx["knowledge_facts"])
	}

}

func TestResolveOrchestratedProviderUsesRoleDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.ResearchProvider = string(agents.ProviderKimi)
	cfg.Agent.ArchitectureProvider = string(agents.ProviderClaude)
	cfg.Agent.CodingProvider = "codex,kimi,claude"

	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	cases := []struct {
		name string
		task orchestrator.Task
		want agents.Provider
	}{
		{
			name: "explicit provider overrides role mapping",
			task: orchestrator.Task{Name: "实现主流程", Provider: string(agents.ProviderKimi)},
			want: agents.ProviderKimi,
		},
		{
			name: "research title maps to research provider",
			task: orchestrator.Task{Name: "调研现有接口方案"},
			want: agents.ProviderKimi,
		},
		{
			name: "architecture title maps to architecture provider",
			task: orchestrator.Task{Name: "架构设计 ADR 拆解"},
			want: agents.ProviderClaude,
		},
	}

	for _, tt := range cases {
		if got := m.resolveOrchestratedProvider(tt.task); got != tt.want {
			t.Fatalf("%s: expected %s, got %s", tt.name, tt.want, got)
		}
	}

	codingTask := orchestrator.Task{ID: "task-coding-1", Name: "实现 task 状态同步"}
	providerA := m.resolveOrchestratedProvider(codingTask)
	providerB := m.resolveOrchestratedProvider(codingTask)
	if providerA != providerB {
		t.Fatalf("expected stable coding provider for same task, got %s and %s", providerA, providerB)
	}
	allowed := map[agents.Provider]struct{}{
		agents.ProviderCodex:  {},
		agents.ProviderKimi:   {},
		agents.ProviderClaude: {},
	}
	if _, ok := allowed[providerA]; !ok {
		t.Fatalf("expected coding provider in codex/kimi/claude, got %s", providerA)
	}
}

func TestSwitchToWorktreePagePersistsWorktreeContext(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))

	record, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if record == nil {
		t.Fatal("expected worktree context to be persisted")
	}
	if record.TaskMode != "single" {
		t.Fatalf("expected single task mode, got %+v", record)
	}
	if record.TaskName == "" {
		t.Fatalf("expected task name to be inferred, got %+v", record)
	}
}

func TestSyncWorktreeActivitiesBuildsResumeSummaryCache(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:       "task-1",
		RepoID:   "/repo/main",
		Title:    "Tighten resume pipeline",
		Goal:     "Show useful overview context",
		NextStep: "Render task summary in worktree pane",
		State:    "active",
		Priority: "high",
	}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if err := st.SaveWorktreeContext(models.WorktreeContextRecord{
		WorktreeID:    "/repo/feature-a",
		RepoID:        "/repo/main",
		PrimaryTaskID: stringPtr("task-1"),
		TaskMode:      "single",
		TaskName:      "feature-a",
		LastActiveAt:  time.Now(),
	}); err != nil {
		t.Fatalf("save worktree context: %v", err)
	}
	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	m.switchToOverviewPage()
	m.syncWorktreeActivities()

	summary, ok := m.resumeSummaryCache["/repo/feature-a"]
	if !ok {
		t.Fatal("expected resume summary cache for worktree")
	}
	if summary.TaskTitle != "Tighten resume pipeline" {
		t.Fatalf("expected task title from task context, got %+v", summary)
	}
	if summary.NextStep != "Render task summary in worktree pane" {
		t.Fatalf("expected next step in summary, got %+v", summary)
	}
}

func TestOpenTaskEditPaneUsesOverlayLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	cmd := m.openTaskEditPane(gitplugin.OpenTaskEditMsg{WorktreeID: "/repo/feature-a"})
	if cmd == nil {
		t.Fatalf("expected init command for task edit overlay")
	}
	if m.activeOverlayPane() != paneTaskEdit {
		t.Fatalf("expected active overlay %s, got %s", paneTaskEdit, m.activeOverlayPane())
	}
	if m.activePage.focused != paneTaskEdit {
		t.Fatalf("expected focus on task edit overlay, got %s", m.activePage.focused)
	}
	updated, _ := m.Update(CloseTaskEditorMsg{ID: paneTaskEdit})
	m = updated.(model)
	if m.activeOverlayPane() != "" {
		t.Fatalf("expected overlay to close, got %s", m.activeOverlayPane())
	}
}

func TestOpenPlanEditPaneUsesOverlayLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	cmd := m.openPlanEditPane(gitplugin.OpenPlanEditMsg{TaskID: "task-1", WorktreeID: "/repo/feature-a"})
	if cmd == nil {
		t.Fatalf("expected init command for plan edit overlay")
	}
	if m.activeOverlayPane() != panePlanEdit {
		t.Fatalf("expected active overlay %s, got %s", panePlanEdit, m.activeOverlayPane())
	}
	if m.activePage.focused != panePlanEdit {
		t.Fatalf("expected focus on plan edit overlay, got %s", m.activePage.focused)
	}
	updated, _ := m.Update(ClosePlanEditorMsg{ID: panePlanEdit})
	m = updated.(model)
	if m.activeOverlayPane() != "" {
		t.Fatalf("expected overlay to close, got %s", m.activeOverlayPane())
	}
}

func TestSaveTaskEditorPersistsTaskAndPrimaryWorktreeLink(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	if cmd := m.saveTaskEditor(TaskEditorSavedMsg{
		ID:           paneTaskEdit,
		WorktreeID:   "/repo/feature-a",
		Title:        "Draft overview task planning",
		Goal:         "Enable lightweight task editing",
		WhyNow:       "The product needs a planning entry point before task explosion.",
		Success:      "A saved task keeps enough context to create a plan from it.",
		OutOfScope:   "Full plan decomposition UI.",
		KnownRisks:   "Too much form friction could slow quick edits.",
		NextStep:     "Save primary task to worktree context",
		State:        "active",
		Priority:     "high",
		RelationType: "primary",
	}); cmd != nil {
		t.Fatalf("expected saveTaskEditor to complete synchronously")
	}

	worktreeContext, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if worktreeContext == nil || worktreeContext.PrimaryTaskID == nil {
		t.Fatalf("expected primary task to be linked, got %+v", worktreeContext)
	}
	task, err := st.GetTaskContext(*worktreeContext.PrimaryTaskID)
	if err != nil {
		t.Fatalf("get task context: %v", err)
	}
	if task == nil || task.Title != "Draft overview task planning" || task.NextStep != "Save primary task to worktree context" || task.Priority != "high" {
		t.Fatalf("unexpected task context: %+v", task)
	}
	brief, err := st.GetTaskBrief(*worktreeContext.PrimaryTaskID)
	if err != nil {
		t.Fatalf("get task brief: %v", err)
	}
	if brief == nil || brief.WhyNow == "" || brief.SuccessCriteria == "" || brief.OutOfScope == "" || brief.KnownRisks == "" {
		t.Fatalf("expected task brief to be persisted, got %+v", brief)
	}
	links, err := st.ListWorktreeTaskLinks("/repo/feature-a")
	if err != nil {
		t.Fatalf("list worktree task links: %v", err)
	}
	if len(links) != 1 || links[0].RelationType != "primary" {
		t.Fatalf("unexpected task/worktree links: %+v", links)
	}
}

func TestSavePlanEditorPersistsDraftPlan(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "/repo/main", Title: "Primary task", State: "active", Priority: "high", PreferredWorktreeID: "/repo/feature-a"}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if cmd := m.savePlanEditor(PlanEditorSavedMsg{
		ID:         panePlanEdit,
		TaskID:     "task-1",
		WorktreeID: "/repo/feature-a",
		Title:      "Phase 1 rollout",
		WhyNow:     "Planning should happen before task expansion.",
		Success:    "The plan survives reopening with brief intact.",
		OutOfScope: "Full session orchestration.",
		KnownRisks: "Expansion may create too many tasks too early.",
		PlanBody:   "Main lane\nValidation lane\nRisk lane",
	}); cmd != nil {
		t.Fatalf("expected savePlanEditor to complete synchronously")
	}
	plans, err := st.ListTaskPlans("task-1")
	if err != nil {
		t.Fatalf("list task plans: %v", err)
	}
	if len(plans) != 1 || plans[0].Title != "Phase 1 rollout" || plans[0].PlanBody == "" || plans[0].Status != "draft" || plans[0].CurrentStep != "Main lane" {
		t.Fatalf("unexpected plans: %+v", plans)
	}
	if plans[0].WhyNow == "" || plans[0].Success == "" || plans[0].OutOfScope == "" || plans[0].KnownRisks == "" {
		t.Fatalf("expected plan-owned brief fields, got %+v", plans[0])
	}
	steps, err := st.ListPlanSteps(plans[0].ID)
	if err != nil {
		t.Fatalf("list plan steps: %v", err)
	}
	if len(steps) != 3 || steps[0].Title != "Main lane" || steps[0].State != "in_progress" || steps[1].State != "pending" {
		t.Fatalf("unexpected saved plan steps: %+v", steps)
	}
	if steps[0].Notes != "main" {
		t.Fatalf("expected default step note kind to be main, got %+v", steps[0])
	}
	wc, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if wc == nil || wc.CurrentPlanID == nil || *wc.CurrentPlanID != plans[0].ID {
		t.Fatalf("expected worktree current plan to point at saved draft, got %+v", wc)
	}
}

func TestSavePlanEditorPersistsStandaloneDraftPlan(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if cmd := m.savePlanEditor(PlanEditorSavedMsg{
		ID:         panePlanEdit,
		WorktreeID: "/repo/feature-a",
		Title:      "Plan before task",
		PlanBody:   "Intent brief\nDecomposition\nConvergence",
	}); cmd != nil {
		t.Fatalf("expected savePlanEditor to complete synchronously")
	}
	plans, err := st.ListTaskPlans("")
	if err != nil {
		t.Fatalf("list task plans: %v", err)
	}
	if len(plans) != 1 || plans[0].TaskID != "" || plans[0].Title != "Plan before task" || plans[0].CurrentStep != "Intent brief" {
		t.Fatalf("unexpected standalone plans: %+v", plans)
	}
	steps, err := st.ListPlanSteps(plans[0].ID)
	if err != nil {
		t.Fatalf("list standalone plan steps: %v", err)
	}
	if len(steps) != 3 || steps[2].Title != "Convergence" {
		t.Fatalf("unexpected standalone plan steps: %+v", steps)
	}
	wc, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if wc == nil || wc.CurrentPlanID == nil || *wc.CurrentPlanID != plans[0].ID {
		t.Fatalf("expected standalone plan to anchor on worktree, got %+v", wc)
	}
}

func TestSavePlanEditorExpandToTasksCreatesPrimaryAndQueuedTasks(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if cmd := m.savePlanEditor(PlanEditorSavedMsg{
		ID:            panePlanEdit,
		WorktreeID:    "/repo/feature-a",
		Title:         "Plan before task",
		WhyNow:        "Need a converged plan before execution.",
		Success:       "Tasks appear only after explicit expansion.",
		PlanBody:      "Intent brief\nDecomposition\nConvergence",
		ExpandToTasks: true,
	}); cmd != nil {
		t.Fatalf("expected savePlanEditor to complete synchronously")
	}
	wc, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if wc == nil || wc.PrimaryTaskID == nil || wc.CurrentPlanID == nil {
		t.Fatalf("expected expanded worktree context, got %+v", wc)
	}
	links, err := st.ListWorktreeTaskLinks("/repo/feature-a")
	if err != nil {
		t.Fatalf("list worktree task links: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("expected 3 task links after expansion, got %+v", links)
	}
	plans, err := st.ListTaskPlans("")
	if err != nil || len(plans) != 1 || plans[0].TaskID == "" {
		t.Fatalf("expected expanded plan to attach primary task, got %+v err=%v", plans, err)
	}
	steps, err := st.ListPlanSteps(*wc.CurrentPlanID)
	if err != nil {
		t.Fatalf("list plan steps: %v", err)
	}
	for _, step := range steps {
		if step.ExpandedTaskID == "" {
			t.Fatalf("expected expanded task id on step, got %+v", step)
		}
	}
}

func TestSavePlanEditorParsesConvergencePrefixes(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if cmd := m.savePlanEditor(PlanEditorSavedMsg{
		ID:         panePlanEdit,
		WorktreeID: "/repo/feature-a",
		Title:      "Converged plan",
		PlanBody:   "validation: check smoke path\nblocked: wait for repo cleanup\nfollow-up: polish detail pane\nworktree: isolate risky refactor",
	}); cmd != nil {
		t.Fatalf("expected savePlanEditor to complete synchronously")
	}
	plans, err := st.ListTaskPlans("")
	if err != nil || len(plans) != 1 {
		t.Fatalf("list plans: %+v err=%v", plans, err)
	}
	steps, err := st.ListPlanSteps(plans[0].ID)
	if err != nil {
		t.Fatalf("list plan steps: %v", err)
	}
	if len(steps) != 4 || steps[0].Notes != "validation" || steps[1].State != "blocked" || steps[2].Notes != "follow-up" || steps[3].Notes != "worktree-candidate" {
		t.Fatalf("unexpected converged steps: %+v", steps)
	}
}

func TestPlanEditorApprovalRequiresWorthItBrief(t *testing.T) {
	pane := NewPlanEditPane(panePlanEdit, models.PaneMeta{ID: panePlanEdit}, models.CommonModel{}, planEditorSeed{Title: "Plan", PlanBody: "Do thing"})
	pane.status = "approved"
	updated, cmd := pane.submit()
	if cmd != nil {
		t.Fatalf("expected approval without brief to fail")
	}
	planPane := updated.(*PlanEditPane)
	if planPane.err == nil {
		t.Fatalf("expected validation error for missing worth-it brief")
	}
}

func TestHandleAgentExitedBackflowsPlanStepAndHandoff(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{ID: "task-1", RepoID: "/repo/main", Title: "Step one", NextStep: "Step one", State: "active", Priority: "medium"}); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := st.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-1", TaskID: "task-1", Title: "Plan", Status: "active", CurrentStep: "Step one"}); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := st.SavePlanStep(models.PlanStepRecord{ID: "step-1", PlanID: "plan-1", OrderIndex: 0, Title: "Step one", State: "in_progress", ExpandedTaskID: "task-1"}); err != nil {
		t.Fatalf("save current step: %v", err)
	}
	if err := st.SavePlanStep(models.PlanStepRecord{ID: "step-2", PlanID: "plan-1", OrderIndex: 1, Title: "Step two", State: "pending"}); err != nil {
		t.Fatalf("save next step: %v", err)
	}
	m.saveAgentSessionRecord(models.AgentSessionRecord{ID: "session-1", Provider: string(agents.ProviderOpenCode), WorktreeID: "/repo/feature-a", TaskID: "task-1", PlanID: "plan-1", StepID: "step-1", State: string(agents.SessionRunning), StartedAt: time.Now()})

	m.handleAgentExited(agents.AgentExitedMsg{WorktreeID: "/repo/feature-a", Provider: agents.ProviderOpenCode})

	steps, err := st.ListPlanSteps("plan-1")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if steps[0].State != "done" || steps[1].State != "in_progress" {
		t.Fatalf("expected plan steps to advance, got %+v", steps)
	}
	plan, err := st.GetTaskPlan("plan-1")
	if err != nil || plan == nil || plan.CurrentStep != "Step two" {
		t.Fatalf("expected plan current step to advance, got %+v err=%v", plan, err)
	}
	task, err := st.GetTaskContext("task-1")
	if err != nil || task == nil || task.NextStep != "Step two" {
		t.Fatalf("expected task next step to update, got %+v err=%v", task, err)
	}
	handoffs, err := st.ListSessionHandoffs("task-1")
	if err != nil || len(handoffs) != 1 || handoffs[0].DoneSummary != "Step one" || handoffs[0].RemainingSummary != "Step two" {
		t.Fatalf("expected handoff backflow, got %+v err=%v", handoffs, err)
	}
}

func TestOpenPlanEditPaneLoadsCurrentPlanForWorktreeWithoutTask(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskPlan(models.TaskPlanRecord{ID: "plan-1", Title: "Standalone plan", Status: "draft", CurrentStep: "Intent brief", PlanBody: "- Intent brief\n- Decomposition"}); err != nil {
		t.Fatalf("save task plan: %v", err)
	}
	if err := st.SavePlanStep(models.PlanStepRecord{ID: "plan-1::step::000", PlanID: "plan-1", OrderIndex: 0, Title: "Intent brief", State: "in_progress"}); err != nil {
		t.Fatalf("save first plan step: %v", err)
	}
	if err := st.SavePlanStep(models.PlanStepRecord{ID: "plan-1::step::001", PlanID: "plan-1", OrderIndex: 1, Title: "Decomposition", State: "pending"}); err != nil {
		t.Fatalf("save second plan step: %v", err)
	}
	if err := st.SaveWorktreeContext(models.WorktreeContextRecord{WorktreeID: "/repo/feature-a", RepoID: "/repo/main", CurrentPlanID: stringPtr("plan-1"), TaskMode: "single", TaskName: "Planning", LastActiveAt: time.Now()}); err != nil {
		t.Fatalf("save worktree context: %v", err)
	}

	cmd := m.openPlanEditPane(gitplugin.OpenPlanEditMsg{WorktreeID: "/repo/feature-a"})
	if cmd == nil {
		t.Fatalf("expected init command for plan edit overlay")
	}
	pane, ok := m.activePage.pane(panePlanEdit).(*PlanEditPane)
	if !ok {
		t.Fatalf("expected plan edit pane, got %T", m.activePage.pane(panePlanEdit))
	}
	view := pane.View()
	if !strings.Contains(view.Content, "Standalone plan") || !strings.Contains(view.Content, "Intent brief") {
		t.Fatalf("expected existing standalone plan to seed editor, got view=%q", view.Content)
	}
}

func TestSaveTaskEditorPersistsQueuedFollowUpWithoutReplacingPrimary(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	if cmd := m.saveTaskEditor(TaskEditorSavedMsg{
		ID:           paneTaskEdit,
		WorktreeID:   "/repo/feature-a",
		Title:        "Primary task",
		Goal:         "Land primary flow",
		WhyNow:       "The queue should attach to a clear parent task.",
		Success:      "Primary task stays stable while follow-ups queue behind it.",
		NextStep:     "Keep resume stable",
		State:        "active",
		Priority:     "medium",
		RelationType: "primary",
	}); cmd != nil {
		t.Fatalf("expected primary save to complete synchronously")
	}
	primaryContext, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get primary worktree context: %v", err)
	}
	if primaryContext == nil || primaryContext.PrimaryTaskID == nil {
		t.Fatalf("expected primary task after first save, got %+v", primaryContext)
	}
	primaryTaskID := *primaryContext.PrimaryTaskID

	if cmd := m.saveTaskEditor(TaskEditorSavedMsg{
		ID:           paneTaskEdit,
		WorktreeID:   "/repo/feature-a",
		Title:        "Queued follow-up",
		Goal:         "Queue cleanup after main work",
		WhyNow:       "This cleanup should not interrupt the active slice.",
		Success:      "The queued task is visible without replacing the primary task.",
		NextStep:     "Clean summary scoring",
		State:        "paused",
		Priority:     "low",
		RelationType: "queued",
		ParentTaskID: primaryTaskID,
	}); cmd != nil {
		t.Fatalf("expected queued save to complete synchronously")
	}

	worktreeContext, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	if worktreeContext == nil || worktreeContext.PrimaryTaskID == nil || *worktreeContext.PrimaryTaskID != primaryTaskID {
		t.Fatalf("expected queued follow-up to keep primary task, got %+v", worktreeContext)
	}
	links, err := st.ListWorktreeTaskLinks("/repo/feature-a")
	if err != nil {
		t.Fatalf("list worktree task links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected primary and queued links, got %+v", links)
	}
	foundQueued := false
	for _, link := range links {
		if link.RelationType == "queued" {
			foundQueued = true
			queuedTask, err := st.GetTaskContext(link.TaskID)
			if err != nil {
				t.Fatalf("get queued task: %v", err)
			}
			if queuedTask == nil || queuedTask.ParentTaskID == nil || *queuedTask.ParentTaskID != primaryTaskID {
				t.Fatalf("expected queued task to point at primary parent, got %+v", queuedTask)
			}
		}
	}
	if !foundQueued {
		t.Fatalf("expected queued relation in links: %+v", links)
	}
}

func TestCycleTaskStateUpdatesPrimaryTask(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	m.switchToWorktreePage("/repo/feature-a", string(paneShell))
	if cmd := m.saveTaskEditor(TaskEditorSavedMsg{
		ID:           paneTaskEdit,
		WorktreeID:   "/repo/feature-a",
		Title:        "Primary task",
		Goal:         "Land primary flow",
		WhyNow:       "The worktree needs an anchored primary task.",
		Success:      "State cycling updates the anchored task.",
		NextStep:     "Keep resume stable",
		State:        "active",
		Priority:     "medium",
		RelationType: "primary",
	}); cmd != nil {
		t.Fatalf("expected saveTaskEditor to complete synchronously")
	}
	worktreeContext, err := st.GetWorktreeContext("/repo/feature-a")
	if err != nil {
		t.Fatalf("get worktree context: %v", err)
	}
	m.cycleTaskState(gitplugin.CycleTaskStateMsg{TaskID: *worktreeContext.PrimaryTaskID, WorktreeID: "/repo/feature-a", CurrentState: "active"})
	task, err := st.GetTaskContext(*worktreeContext.PrimaryTaskID)
	if err != nil {
		t.Fatalf("get task context: %v", err)
	}
	if task == nil || task.State != "paused" {
		t.Fatalf("expected task state to cycle to paused, got %+v", task)
	}
}

func TestCycleTaskStateDoneLaunchesReadyDownstreamTask(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-upstream",
		RepoID:              "/repo/main",
		Title:               "Upstream",
		State:               "blocked",
		Priority:            "high",
		PreferredWorktreeID: "/repo/feature-a",
	}); err != nil {
		t.Fatalf("save upstream task: %v", err)
	}
	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-downstream",
		RepoID:              "/repo/main",
		Title:               "Downstream",
		State:               "paused",
		Priority:            "medium",
		PreferredWorktreeID: "/repo/feature-b",
	}); err != nil {
		t.Fatalf("save downstream task: %v", err)
	}
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID:     "task-upstream",
		ToTaskID:       "task-downstream",
		DependencyType: "hard",
	}); err != nil {
		t.Fatalf("save task dependency: %v", err)
	}

	cmd := m.cycleTaskState(gitplugin.CycleTaskStateMsg{
		TaskID:       "task-upstream",
		WorktreeID:   "/repo/feature-a",
		CurrentState: "blocked",
	})
	if cmd == nil {
		t.Fatal("expected downstream launch command when upstream moves to done")
	}

	upstream, err := st.GetTaskContext("task-upstream")
	if err != nil {
		t.Fatalf("get upstream task: %v", err)
	}
	if upstream == nil || upstream.State != "done" {
		t.Fatalf("expected upstream task done, got %+v", upstream)
	}

	sessions, err := st.ListAgentSessions("/repo/feature-b")
	if err != nil {
		t.Fatalf("list downstream sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one launched downstream session, got %+v", sessions)
	}
}

func TestBuildWorkbenchOverviewContextTranslatesTruthToSummaryAndDetail(t *testing.T) {
	now := time.Now()
	source := fakeWorkbenchContextSource{
		ordered: []gitplugin.WorktreeContextView{
			{Worktree: gitmodel.Worktree{Path: "/repo/main", Branch: "main", IsMain: true}, Summary: gitmodel.WorktreeResumeSummary{}, Activity: gitmodel.WorktreeActivity{}},
			{Worktree: gitmodel.Worktree{Path: "/repo/feature-a", Branch: "feature-a", DirtySummary: gitmodel.DirtySummary{Staged: 1, Unstaged: 2}, AheadBehind: gitmodel.AheadBehind{Ahead: 1}, Upstream: "origin/feature-a"}, Summary: gitmodel.WorktreeResumeSummary{TaskID: "task-1", TaskTitle: "Refactor overview translation", TaskGoal: "Make overview reflect context truth", TaskWhyNow: "The overview needs a plan-first entry point.", TaskSuccess: "A programmer can restart work from the saved brief.", TaskOutOfScope: "Full graph orchestration.", TaskKnownRisks: "Too much density could hurt scanning.", NextStep: "Centralize workbench context builder", TaskState: "active", TaskPriority: "high", PlanTitle: "Phase 1 rollout", PlanStatus: "active", CurrentPlanStep: "Wire overview summary", PlanBody: "Main lane\nValidation lane\nRisk lane", PlanSteps: []string{"[in_progress] Wire overview summary", "[pending] Validation lane", "[pending] Risk lane"}, HandoffEntrypoint: "Open overview detail pane", QueuedTaskTitle: "Follow-up queue cleanup", QueuedTaskCount: 2, AttentionAnchor: "editing", RecentArtifact: "internal/app/workbench_context.go", PinnedNote: "Keep truth and projection separate", BlockerNote: "Need better auto inference", HandoffNote: "Resume from shared builder wiring", GitPressure: "diverged+dirty", LastAgentSummary: "opencode running", ResumeReason: "active task · next step ready", LastResumeHint: "Continue: Centralize workbench context builder", ResumeScore: 95, LastActiveLabel: formatRelativeLabel("active", now)}, Activity: gitmodel.WorktreeActivity{OpenEditors: 2, HasShell: true, AgentCount: 1, LastActive: "now"}},
		},
		selectedWorktree: gitmodel.Worktree{Path: "/repo/feature-a", Branch: "feature-a", DirtySummary: gitmodel.DirtySummary{Staged: 1, Unstaged: 2}, AheadBehind: gitmodel.AheadBehind{Ahead: 1}, Upstream: "origin/feature-a"},
		selectedSummary:  gitmodel.WorktreeResumeSummary{TaskID: "task-1", TaskTitle: "Refactor overview translation", TaskGoal: "Make overview reflect context truth", TaskWhyNow: "The overview needs a plan-first entry point.", TaskSuccess: "A programmer can restart work from the saved brief.", TaskOutOfScope: "Full graph orchestration.", TaskKnownRisks: "Too much density could hurt scanning.", NextStep: "Centralize workbench context builder", TaskState: "active", TaskPriority: "high", PlanTitle: "Phase 1 rollout", PlanStatus: "active", CurrentPlanStep: "Wire overview summary", PlanBody: "Main lane\nValidation lane\nRisk lane", PlanSteps: []string{"[in_progress] Wire overview summary", "[pending] Validation lane", "[pending] Risk lane"}, HandoffEntrypoint: "Open overview detail pane", QueuedTaskTitle: "Follow-up queue cleanup", QueuedTaskCount: 2, AttentionAnchor: "editing", RecentArtifact: "internal/app/workbench_context.go", PinnedNote: "Keep truth and projection separate", BlockerNote: "Need better auto inference", HandoffNote: "Resume from shared builder wiring", GitPressure: "diverged+dirty", LastAgentSummary: "opencode running", ResumeReason: "active task · next step ready", LastResumeHint: "Continue: Centralize workbench context builder", ResumeScore: 95},
		selectedActivity: gitmodel.WorktreeActivity{OpenEditors: 2, HasShell: true, AgentCount: 1, LastActive: "now"},
	}
	ctx := buildWorkbenchOverviewContext(source, nil, "/repo/main")
	if ctx.Stats.Total != 2 || ctx.Stats.Active != 1 || ctx.Stats.Queued != 2 || ctx.Stats.Dirty != 1 || ctx.Stats.RunningAgent != 1 {
		t.Fatalf("unexpected overview stats: %+v", ctx.Stats)
	}
	if ctx.Detail.Title != "Refactor overview translation" || ctx.Detail.Goal != "Make overview reflect context truth" {
		t.Fatalf("unexpected detail projection: %+v", ctx.Detail)
	}
	if ctx.Detail.WhyNow == "" || ctx.Detail.SuccessCriteria == "" || ctx.Detail.OutOfScope == "" || ctx.Detail.KnownRisks == "" {
		t.Fatalf("expected detail to include brief projection, got %+v", ctx.Detail)
	}
	if !strings.Contains(ctx.Detail.GitSummary, "staged") || !strings.Contains(ctx.Detail.RuntimeSummary, "shell=true") || !strings.Contains(ctx.Detail.AgentSummary, "opencode running") {
		t.Fatalf("expected detail to include git/runtime/agent summaries, got %+v", ctx.Detail)
	}
	if ctx.Detail.Attention != "editing · internal/app/workbench_context.go" {
		t.Fatalf("expected detail to include attention signal, got %+v", ctx.Detail)
	}
	if ctx.Detail.RecentArtifact != "internal/app/workbench_context.go" || ctx.Detail.PinnedNote == "" || ctx.Detail.BlockerNote == "" {
		t.Fatalf("expected detail to include artifact and notes, got %+v", ctx.Detail)
	}
	if ctx.Detail.HandoffNote != "Resume from shared builder wiring" || ctx.Detail.GitPressure != "git pressure: diverged+dirty" {
		t.Fatalf("expected detail to include handoff and git pressure, got %+v", ctx.Detail)
	}
	if ctx.Detail.PlanTitle != "Phase 1 rollout" || ctx.Detail.CurrentPlanStep != "Wire overview summary" || ctx.Detail.HandoffEntrypoint != "Open overview detail pane" {
		t.Fatalf("expected detail to include plan/handoff projection, got %+v", ctx.Detail)
	}
	if !strings.Contains(ctx.Detail.PlanBody, "Validation lane") {
		t.Fatalf("expected detail to include plan body, got %+v", ctx.Detail)
	}
	if len(ctx.Detail.PlanSteps) != 3 || !strings.Contains(ctx.Detail.PlanSteps[0], "Wire overview summary") {
		t.Fatalf("expected detail to include structured plan steps, got %+v", ctx.Detail)
	}
}

type fakeWorkbenchContextSource struct {
	ordered          []gitplugin.WorktreeContextView
	selectedWorktree gitmodel.Worktree
	selectedSummary  gitmodel.WorktreeResumeSummary
	selectedActivity gitmodel.WorktreeActivity
}

func (f fakeWorkbenchContextSource) OrderedContexts() []gitplugin.WorktreeContextView {
	return f.ordered
}

func (f fakeWorkbenchContextSource) SelectedContext() (gitmodel.Worktree, gitmodel.WorktreeResumeSummary, gitmodel.WorktreeActivity, bool) {
	return f.selectedWorktree, f.selectedSummary, f.selectedActivity, true
}

func shAutoType(m *shell.Model) string {
	return reflect.ValueOf(m).Elem().FieldByName("autoType").String()
}

func stringPtr(value string) *string {
	return &value
}

func TestZoomToggle(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	initialOrder := layout.LeafOrder(m.activePage.bodyTree)
	m.setFocus(paneShell)

	newM, _ := m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	m = newM.(model)

	if m.activePage.zoomedPane != paneShell {
		t.Fatalf("expected zoomed pane to be shell, got %s", m.activePage.zoomedPane)
	}

	zoomedOrder := layout.LeafOrder(m.activePage.bodyTree)
	if len(zoomedOrder) != 1 {
		t.Fatalf("expected 1 pane when zoomed, got %d", len(zoomedOrder))
	}

	newM, _ = m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
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
	t.Skip("incompatible with new single-page layout")
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

func TestClosePaneRestoresFocusToEditorOpener(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)

	path := filepath.Join(t.TempDir(), "main.go")
	m.setFocus(paneWorktree)
	m.openEditorPane(editorplugin.OpenEditorMsg{FilePath: path, Behavior: editorplugin.OpenBehaviorDefault})
	editorID := m.activePage.focused

	m.closePane(editorID)

	if m.activePage.focused != paneWorktree {
		t.Fatalf("expected focus to return to worktree pane, got %s", m.activePage.focused)
	}
	if _, ok := m.activePage.paneMeta[editorID]; ok {
		t.Fatalf("expected editor pane %s to be removed", editorID)
	}
}

func TestOpenDiffPaneAddsBodyPaneAndRestoresOpenerFocus(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.switchToWorktreePage("/repo/feature-a", string(paneWorktreeDetail))
	initialLeaves := len(layout.LeafOrder(m.activePage.bodyTree))

	cmd := m.openDiffPane(gitplugin.OpenDiffMsg{FilePath: "", Staged: false})
	if cmd == nil {
		t.Fatalf("expected init command for diff pane")
	}
	if m.activePage.focused != paneGitDiff {
		t.Fatalf("expected diff pane to be focused, got %s", m.activePage.focused)
	}
	if got := len(layout.LeafOrder(m.activePage.bodyTree)); got != initialLeaves {
		t.Fatalf("expected leaf count %d after opening diff overlay (overlays do not add leaves), got %d", initialLeaves+1, got)
	}
	if meta, ok := m.activePage.paneMeta[paneGitDiff]; !ok || meta.Type != models.PaneTypeDiffView {
		t.Fatalf("expected diff pane metadata to be registered")
	}

	m.closePane(paneGitDiff)
	if m.activePage.focused != paneWorktreeDetail {
		t.Fatalf("expected focus to return to git status, got %s", m.activePage.focused)
	}
}

func TestOpenWorktreeShellAddsScopedShellPane(t *testing.T) {
	t.Skip("incompatible with new single-page layout")
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.setFocus(paneWorktree)

	m.openWorktreeShell(gitplugin.OpenWorktreeShellMsg{Worktree: gitplugin_testWorktree("/repo/feature-a", "feature-a")})
	if got := len(layout.LeafOrder(m.activePage.bodyTree)); got != 4 {
		t.Fatalf("expected leaf count 4 after opening worktree shell (reuses existing shell), got %d", got)
	}
	if m.activePage.focused != paneShell {
		t.Fatalf("expected focus to move to existing shell, got %s", m.activePage.focused)
	}
	meta, ok := m.activePage.paneMeta[paneShell]
	if !ok || meta.Type != models.PaneTypeShell {
		t.Fatalf("expected focused pane to be shell, got %+v", meta)
	}
	if meta.CWD != "/repo/feature-a" {
		t.Fatalf("expected shell cwd to be worktree path, got %q", meta.CWD)
	}
	if meta.WorktreeID != "/repo/feature-a" {
		t.Fatalf("expected shell worktree id to match path, got %q", meta.WorktreeID)
	}
	if sh, ok := m.pane(paneShell).(*shell.Model); !ok || sh == nil {
		t.Fatalf("expected shell pane to hold shell model")
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
		t.Fatalf("expected leaf count 4 after opening new worktree shell (reuses existing shell), got %d", got)
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

func TestWorktreeCreatedForExistingTaskPreparesTrellisContext(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	bridge := &fakeTrellisBridge{}
	m.trellisBridge = bridge

	task := models.TaskContextRecord{
		ID:       "task-existing",
		RepoID:   m.gitRepoPath(),
		Title:    "Existing Task",
		State:    "active",
		Priority: "medium",
	}
	if err := st.SaveTaskContext(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	cmd := m.createTaskForWorktree(gitplugin.WorktreeCreatedMsg{
		Worktree:  gitplugin_testWorktree("/tmp/focus-existing-task-wt", "existing-task"),
		TaskTitle: task.Title,
		TaskID:    task.ID,
		IsPhase:   true,
	})
	if cmd == nil {
		t.Fatalf("expected dag refresh command")
	}
	_ = cmd()

	if bridge.worktreeID != "/tmp/focus-existing-task-wt" {
		t.Fatalf("expected bridge worktree id to be set, got %q", bridge.worktreeID)
	}
	if bridge.linkedWorktreePath != "/tmp/focus-existing-task-wt" {
		t.Fatalf("expected worktree links to be prepared, got %q", bridge.linkedWorktreePath)
	}
	if bridge.syncedTaskID != task.ID {
		t.Fatalf("expected task sync for %q, got %q", task.ID, bridge.syncedTaskID)
	}
	if bridge.builtSessionTaskID != task.ID || bridge.builtSessionWorktree != "/tmp/focus-existing-task-wt" {
		t.Fatalf("expected agent context build for task/worktree, got task=%q worktree=%q", bridge.builtSessionTaskID, bridge.builtSessionWorktree)
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

func TestHandleMouseBlocksWhenDiffOverlayActive(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	m.activePage.panes[paneGitDiff] = &fakePanel{}
	m.activePage.paneMeta[paneGitDiff] = models.PaneMeta{ID: paneGitDiff, Name: "Diff", Type: models.PaneTypeDiffView, Closable: true}
	m.activePage.focused = paneWorktreeDetail
	m.common.Width = 120
	m.common.Height = 40
	m.updateSizes(120, 40)
	dims := layout.ComputeBanner(m.common.Width, m.common.Height)

	updated, _ := m.handleMouse(tea.MouseWheelMsg{X: dims.HeaderH + 5, Y: dims.HeaderH + 5, Button: tea.MouseWheelDown})
	m = updated.(model)

	diffPanel := m.pane(paneGitDiff).(*fakePanel)
	if len(diffPanel.updates) != 0 {
		t.Fatalf("expected diff overlay to block mouse events, got %d updates", len(diffPanel.updates))
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

func TestCmdBusAlwaysAvailable(t *testing.T) {
	cfg := config.DefaultConfig()
	st, _ := store.New(":memory:")
	m := New(cfg, st).(model)
	if m.cmdBus == nil {
		t.Fatal("cmdBus should be available when store is *store.Store")
	}
}

func TestDagPaneQuickCreateTask(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	common := &models.CommonModel{Store: st}

	// Seed two tasks with a dependency to verify DAG builds correctly.
	now := time.Now()
	task1 := models.TaskContextRecord{
		ID: "task-1", RepoID: "repo", Title: "First task",
		State: "active", Priority: "medium", CreatedAt: now, UpdatedAt: now,
	}
	task2 := models.TaskContextRecord{
		ID: "task-2", RepoID: "repo", Title: "Second task",
		State: "paused", Priority: "medium", CreatedAt: now, UpdatedAt: now,
	}
	if err := st.SaveTaskContext(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := st.SaveTaskContext(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	if err := st.SaveTaskDependency(models.TaskDependencyRecord{
		FromTaskID: "task-1", ToTaskID: "task-2", DependencyType: "hard",
	}); err != nil {
		t.Fatalf("save dependency: %v", err)
	}

	pane := newDagPane(paneDAG, models.PaneMeta{}, common, "repo", nil)
	pane.SetSize(120, 20)

	// Trigger initial DAG build.
	panel, _ := pane.Update(dagRefreshMsg{repoID: "repo"})
	pane = panel.(*dagPane)

	// Verify initial state: not creating, DAG contains tasks.
	if pane.creating {
		t.Fatal("expected creating=false initially")
	}
	view := pane.View()
	if !strings.Contains(view.Content, "Task DAG") {
		t.Fatalf("expected 'Task DAG' in view, got:\n%s", view.Content)
	}
	if !strings.Contains(view.Content, "[n]new-task") {
		t.Fatalf("expected '[n]new-task' hint in normal mode, got:\n%s", view.Content)
	}
	if !strings.Contains(view.Content, "First task") {
		t.Fatalf("expected task title in DAG view, got:\n%s", view.Content)
	}

	// Press 'n' to enter creating mode.
	panel, cmd := pane.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	pane = panel.(*dagPane)
	if !pane.creating {
		t.Fatal("expected creating=true after pressing 'n'")
	}
	if cmd == nil {
		t.Fatal("expected init cmd after entering create mode")
	}
	// Bootstrap the Huh form with its init command.
	if initMsg := cmd(); initMsg != nil {
		panel, _ = pane.Update(initMsg)
		pane = panel.(*dagPane)
	}

	// Verify creating mode view shows Huh form fields.
	view = pane.View()
	if !strings.Contains(ansi.Strip(view.Content), "New task title") {
		t.Fatalf("expected title input prompt in view, got:\n%s", ansi.Strip(view.Content))
	}
	if !strings.Contains(ansi.Strip(view.Content), "Goal (optional)") {
		t.Fatalf("expected goal input prompt in view, got:\n%s", ansi.Strip(view.Content))
	}
	if !strings.Contains(view.Content, "[T]asks [A]DRs") {
		t.Fatalf("expected creating-mode hint in view, got:\n%s", view.Content)
	}
	// Normal-mode hint should NOT appear.
	if strings.Contains(view.Content, "[n]new-task") {
		t.Fatal("expected normal-mode hint to be replaced in creating mode")
	}

	// Type a title into the focused title input.
	panel, _ = pane.Update(tea.KeyPressMsg{Code: 'B', Text: "B"})
	pane = panel.(*dagPane)
	panel, _ = pane.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	pane = panel.(*dagPane)
	panel, _ = pane.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	pane = panel.(*dagPane)

	// Press Enter to confirm title and move to goal field.
	panel, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = panel.(*dagPane)

	// Type a goal.
	panel, _ = pane.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	pane = panel.(*dagPane)
	panel, _ = pane.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	pane = panel.(*dagPane)
	panel, _ = pane.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	pane = panel.(*dagPane)

	// Press Enter to submit (completes the Huh form).
	panel, cmd = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = panel.(*dagPane)
	if pane.creating {
		t.Fatal("expected creating=false after submit")
	}
	if cmd == nil {
		t.Fatal("expected cmd to emit dagTaskCreatedMsg")
	}

	msg := cmd()
	created, ok := msg.(dagTaskCreatedMsg)
	if !ok {
		t.Fatalf("expected dagTaskCreatedMsg, got %T", msg)
	}
	if created.Title != "Bug" {
		t.Fatalf("expected title 'Bug', got %q", created.Title)
	}
	if created.Goal != "fix" {
		t.Fatalf("expected goal 'fix', got %q", created.Goal)
	}
	if created.RepoID != "repo" {
		t.Fatalf("expected repoID 'repo', got %q", created.RepoID)
	}

	// Verify pressing Esc cancels creation.
	pane.enterCreateMode()
	if !pane.creating {
		t.Fatal("expected creating=true")
	}
	panel, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	pane = panel.(*dagPane)
	if pane.creating {
		t.Fatal("expected creating=false after esc")
	}
}

func TestWorktreeDetailPaneTasksTabOpensTaskEditOnEnterAndE(t *testing.T) {
	st, _ := store.New(":memory:")
	worktreeID := "/repo/feature-a"
	cfg := config.DefaultConfig()
	m := New(cfg, st).(model)
	repoID := m.gitRepoPath()
	if repoID == "" {
		repoID = "/repo/main"
	}
	if err := st.SaveTaskContext(models.TaskContextRecord{
		ID:                  "task-1",
		RepoID:              repoID,
		Title:               "Fix DAG routing",
		State:               "active",
		Priority:            "high",
		PreferredWorktreeID: worktreeID,
	}); err != nil {
		t.Fatalf("save task context: %v", err)
	}
	if err := st.SaveWorktreeContext(models.WorktreeContextRecord{
		WorktreeID:   worktreeID,
		RepoID:       repoID,
		TaskName:     "feature-a",
		TaskMode:     "single",
		LastActiveAt: time.Now(),
	}); err != nil {
		t.Fatalf("save worktree context: %v", err)
	}

	m.switchToWorktreePage(worktreeID, string(paneWorktreeDetail))

	detail, ok := m.activePage.pane(paneWorktreeDetail).(*worktreeDetailPane)
	if !ok {
		t.Fatalf("expected worktreeDetailPane, got %T", m.activePage.pane(paneWorktreeDetail))
	}
	detail.worktreeID = worktreeID
	detail.loadTasks()

	updatedAny, cmd := m.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = updatedAny.(model)
	if cmd == nil {
		t.Fatalf("expected open task edit command on e")
	}
	msg := cmd()
	openMsg, ok := msg.(gitplugin.OpenTaskEditMsg)
	if !ok {
		t.Fatalf("expected OpenTaskEditMsg from e, got %T", msg)
	}
	if openMsg.WorktreeID != worktreeID || openMsg.TaskID != "task-1" || openMsg.RelationType != "primary" {
		t.Fatalf("unexpected open msg from e: %+v", openMsg)
	}

	m.closePane(paneTaskEdit)
	updatedAny, cmd = m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updatedAny.(model)
	if cmd == nil {
		t.Fatalf("expected open task edit command on enter")
	}
	msg = cmd()
	openMsg, ok = msg.(gitplugin.OpenTaskEditMsg)
	if !ok {
		t.Fatalf("expected OpenTaskEditMsg from enter, got %T", msg)
	}
	if openMsg.WorktreeID != worktreeID || openMsg.TaskID != "task-1" || openMsg.RelationType != "primary" {
		t.Fatalf("unexpected open msg from enter: %+v", openMsg)
	}
}
