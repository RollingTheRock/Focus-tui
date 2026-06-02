package git

import (
	"errors"
	"strings"
	"testing"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "charm.land/bubbletea/v2"
)

func TestPluginCreatePaneReturnsStatusPane(t *testing.T) {
	adapter := &fakeGitAdapter{watchCh: make(chan adapters.StatusEvent, 2)}
	pl := New(adapter)

	panel, err := pl.CreatePane(models.PaneTypeGitStatus, "git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{})
	if err != nil {
		t.Fatalf("create pane: %v", err)
	}

	if _, ok := panel.(*StatusPane); !ok {
		t.Fatalf("expected *StatusPane, got %T", panel)
	}
}

func TestStatusPaneInitLoadsStatusAndRendersSections(t *testing.T) {
	status := &gitmodel.Status{
		Branch:         "main",
		Upstream:       "origin/main",
		Ahead:          2,
		Behind:         1,
		StagedFiles:    []gitmodel.File{{Path: "staged.go", StagedStatus: gitmodel.Added}},
		UnstagedFiles:  []gitmodel.File{{Path: "modified.go", WorktreeStatus: gitmodel.Modified}},
		UntrackedFiles: []gitmodel.File{{Path: "new.txt", Status: gitmodel.Untracked}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	if pane.Init() == nil {
		t.Fatalf("expected init command")
	}

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	view := pane.View()
	for _, want := range []string{"main", "origin/main", "↑2", "↓1", "Staged", "Unstaged", "Untracked", "staged.go", "modified.go", "new.txt"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view.Content)
		}
	}
	if pane.loading {
		t.Fatalf("expected loading to be false after initial status")
	}
}

func TestStatusPaneNavigationClampsCursor(t *testing.T) {
	pane := &StatusPane{
		status: &gitmodel.Status{
			StagedFiles:    []gitmodel.File{{Path: "a.go", StagedStatus: gitmodel.Modified}},
			UntrackedFiles: []gitmodel.File{{Path: "b.go", Status: gitmodel.Untracked}},
		},
	}

	updated, _ := pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*StatusPane)
	if pane.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", pane.cursor)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*StatusPane)
	if pane.cursor != 1 {
		t.Fatalf("expected cursor to clamp at 1, got %d", pane.cursor)
	}

	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	pane = updated.(*StatusPane)
	if pane.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", pane.cursor)
	}
}

func TestStatusPaneStatusEventSchedulesWatchAndStatsRefresh(t *testing.T) {
	adapter := &fakeGitAdapter{watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	updated, cmd := pane.Update(adapters.StatusEvent{
		RepoPath: "/repo",
		Status: &gitmodel.Status{
			Branch:      "feature/test",
			StagedFiles: []gitmodel.File{{Path: "file.go", StagedStatus: gitmodel.Modified}},
		},
	})
	pane = updated.(*StatusPane)
	if cmd == nil {
		t.Fatalf("expected batch command after status event")
	}

	adapter.watchCh <- adapters.StatusEvent{RepoPath: "/repo", Status: &gitmodel.Status{Branch: "feature/test"}}
	msgs := runBatchMsg(t, cmd)

	var sawRefresh bool
	var sawWatch bool
	for _, subCmd := range msgs {
		msg := runCmd(t, subCmd)
		switch msg.(type) {
		case models.StatsRefreshMsg:
			sawRefresh = true
		case adapters.StatusEvent:
			sawWatch = true
		}
	}

	if !sawRefresh {
		t.Fatalf("expected stats refresh command")
	}
	if !sawWatch {
		t.Fatalf("expected watch command")
	}
	if pane.status == nil || pane.status.Branch != "feature/test" {
		t.Fatalf("expected pane status to update")
	}
}

func TestStatusPaneShowsErrorWhenLoadFails(t *testing.T) {
	adapter := &fakeGitAdapter{
		getStatusErr: errors.New("not a git repository"),
		watchCh:      make(chan adapters.StatusEvent, 1),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 10)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	if !strings.Contains(pane.View().Content, "not a git repository") {
		t.Fatalf("expected error view, got %q", pane.View().Content)
	}
}

func TestStatusPaneStageFile(t *testing.T) {
	status := &gitmodel.Status{
		Branch:        "main",
		UnstagedFiles: []gitmodel.File{{Path: "modified.go", WorktreeStatus: gitmodel.Modified}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after staging")
	}

	if !adapter.stageCalled {
		t.Fatalf("expected StageFile to be called")
	}

	if adapter.stagedPath != "modified.go" {
		t.Fatalf("expected to stage modified.go, got %s", adapter.stagedPath)
	}
}

func TestStatusPaneUnstageFile(t *testing.T) {
	status := &gitmodel.Status{
		Branch:      "main",
		StagedFiles: []gitmodel.File{{Path: "staged.go", StagedStatus: gitmodel.Added}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after unstaging")
	}

	if !adapter.unstageCalled {
		t.Fatalf("expected UnstageFile to be called")
	}

	if adapter.unstagedPath != "staged.go" {
		t.Fatalf("expected to unstage staged.go, got %s", adapter.unstagedPath)
	}
}

func TestStatusPaneStageUntrackedFile(t *testing.T) {
	status := &gitmodel.Status{
		Branch:         "main",
		UntrackedFiles: []gitmodel.File{{Path: "new.txt", Status: gitmodel.Untracked}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after staging")
	}

	if !adapter.stageCalled {
		t.Fatalf("expected StageFile to be called for untracked file")
	}

	if adapter.stagedPath != "new.txt" {
		t.Fatalf("expected to stage new.txt, got %s", adapter.stagedPath)
	}
}

func TestStatusPaneStageError(t *testing.T) {
	status := &gitmodel.Status{
		Branch:        "main",
		UnstagedFiles: []gitmodel.File{{Path: "modified.go", WorktreeStatus: gitmodel.Modified}},
	}
	adapter := &fakeGitAdapter{
		status:   status,
		stageErr: errors.New("permission denied"),
		watchCh:  make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command when staging fails")
	}
}

func TestStatusPaneDiscardStagedFile(t *testing.T) {
	status := &gitmodel.Status{
		Branch:      "main",
		StagedFiles: []gitmodel.File{{Path: "staged.go", StagedStatus: gitmodel.Added}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl, Text: "d"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command before discard confirmation")
	}
	if !pane.confirmDiscard {
		t.Fatalf("expected discard confirmation to be active")
	}

	updated, cmd = pane.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after discarding staged file")
	}
	if pane.confirmDiscard {
		t.Fatalf("expected discard confirmation to close")
	}
	if !adapter.discardCalled {
		t.Fatalf("expected DiscardChanges to be called")
	}
	if adapter.discardedPath != "staged.go" {
		t.Fatalf("expected to discard staged.go, got %s", adapter.discardedPath)
	}
}

func TestStatusPaneDiscardUnstagedFile(t *testing.T) {
	status := &gitmodel.Status{
		Branch:        "main",
		UnstagedFiles: []gitmodel.File{{Path: "modified.go", WorktreeStatus: gitmodel.Modified}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl, Text: "d"})
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after discarding unstaged file")
	}
	if !adapter.discardCalled {
		t.Fatalf("expected DiscardChanges to be called")
	}
	if adapter.discardedPath != "modified.go" {
		t.Fatalf("expected to discard modified.go, got %s", adapter.discardedPath)
	}
}

func TestStatusPaneDiscardUntrackedFile(t *testing.T) {
	status := &gitmodel.Status{
		Branch:         "main",
		UntrackedFiles: []gitmodel.File{{Path: "new.txt", Status: gitmodel.Untracked}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl, Text: "d"})
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after discarding untracked file")
	}
	if !adapter.discardCalled {
		t.Fatalf("expected DiscardChanges to be called")
	}
	if adapter.discardedPath != "new.txt" {
		t.Fatalf("expected to discard new.txt, got %s", adapter.discardedPath)
	}
}

func TestStatusPaneDiscardConfirmationCancel(t *testing.T) {
	status := &gitmodel.Status{
		Branch:        "main",
		UnstagedFiles: []gitmodel.File{{Path: "modified.go", WorktreeStatus: gitmodel.Modified}},
	}
	adapter := &fakeGitAdapter{
		status:  status,
		watchCh: make(chan adapters.StatusEvent, 2),
	}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)
	pane.SetSize(80, 20)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl, Text: "d"})
	pane = updated.(*StatusPane)

	if !strings.Contains(pane.View().Content, "Discard selected changes for modified.go? [y/n]") {
		t.Fatalf("expected discard confirmation prompt, got:\n%s", pane.View().Content)
	}

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command when discard confirmation is cancelled")
	}
	if pane.confirmDiscard {
		t.Fatalf("expected discard confirmation to close after cancel")
	}
	if adapter.discardCalled {
		t.Fatalf("expected DiscardChanges not to be called when cancelled")
	}
}

type fakeGitAdapter struct {
	status       *gitmodel.Status
	getStatusErr error
	watchErr     error
	watchCh      chan adapters.StatusEvent
	diff         string
	diffErr      error
	fileContent  string
	beforeContent string
	fileContentErr error
	fetchErr     error
	pullErr      error
	pushErr      error

	stageCalled       bool
	stagedPath        string
	stageErr          error
	stageAllCalled    bool
	stageAllErr       error
	unstageCalled     bool
	unstagedPath      string
	unstageErr        error
	unstageAllCalled  bool
	unstageAllErr     error
	fetchCalled       bool
	pullCalled        bool
	pushCalled        bool
	discardCalled     bool
	discardedPath     string
	discardErr        error
	worktrees         []gitmodel.Worktree
	listWorktreesErr  error
	createWorktreeReq *gitmodel.CreateWorktreeRequest
	createWorktreeRes *gitmodel.Worktree
	createWorktreeErr error
}

func (f *fakeGitAdapter) Name() string { return "fake-git" }

func (f *fakeGitAdapter) Init() error { return nil }

func (f *fakeGitAdapter) Destroy() error { return nil }

func (f *fakeGitAdapter) GetStatus(repoPath string) (*gitmodel.Status, error) {
	return f.status, f.getStatusErr
}

func (f *fakeGitAdapter) GetBranches(repoPath string) ([]gitmodel.Branch, error) { return nil, nil }

func (f *fakeGitAdapter) GetWorktreeStatus(worktreePath string) (*gitmodel.Status, error) {
	return f.status, f.getStatusErr
}

func (f *fakeGitAdapter) ListWorktrees(repoPath string) ([]gitmodel.Worktree, error) {
	return f.worktrees, f.listWorktreesErr
}

func (f *fakeGitAdapter) CreateWorktree(repoPath string, req gitmodel.CreateWorktreeRequest) (*gitmodel.Worktree, error) {
	copyReq := req
	f.createWorktreeReq = &copyReq
	return f.createWorktreeRes, f.createWorktreeErr
}

func (f *fakeGitAdapter) RemoveWorktree(repoPath, worktreePath string, opts gitmodel.RemoveWorktreeOptions) error {
	return nil
}

func (f *fakeGitAdapter) PruneWorktrees(repoPath string) error { return nil }

func (f *fakeGitAdapter) GetDiff(repoPath string, path string, staged bool) (string, error) {
	return f.diff, f.diffErr
}

func (f *fakeGitAdapter) GetFileContent(repoPath string, path string, ref string) (string, error) {
	if ref == "HEAD" && f.beforeContent != "" {
		return f.beforeContent, f.fileContentErr
	}
	return f.fileContent, f.fileContentErr
}

func (f *fakeGitAdapter) Fetch(repoPath string) error {
	f.fetchCalled = true
	return f.fetchErr
}

func (f *fakeGitAdapter) Pull(repoPath string) error {
	f.pullCalled = true
	return f.pullErr
}

func (f *fakeGitAdapter) Push(repoPath string) error {
	f.pushCalled = true
	return f.pushErr
}

func (f *fakeGitAdapter) WatchStatus(repoPath string) (<-chan adapters.StatusEvent, error) {
	if f.watchErr != nil {
		return nil, f.watchErr
	}
	return f.watchCh, nil
}

func (f *fakeGitAdapter) StopWatch(repoPath string) {}

func (f *fakeGitAdapter) StageFile(repoPath string, path string) error {
	f.stageCalled = true
	f.stagedPath = path
	return f.stageErr
}

func (f *fakeGitAdapter) StageAll(repoPath string) error {
	f.stageAllCalled = true
	return f.stageAllErr
}

func (f *fakeGitAdapter) UnstageFile(repoPath string, path string) error {
	f.unstageCalled = true
	f.unstagedPath = path
	return f.unstageErr
}

func (f *fakeGitAdapter) UnstageAll(repoPath string) error {
	f.unstageAllCalled = true
	return f.unstageAllErr
}

func (f *fakeGitAdapter) DiscardChanges(repoPath string, path string) error {
	f.discardCalled = true
	f.discardedPath = path
	return f.discardErr
}

func TestStatusPaneStageAll(t *testing.T) {
	status := &gitmodel.Status{
		Branch:        "main",
		UnstagedFiles: []gitmodel.File{{Path: "modified.go", WorktreeStatus: gitmodel.Modified}},
	}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after stage all")
	}
	if !adapter.stageAllCalled {
		t.Fatalf("expected StageAll to be called")
	}
	if adapter.unstageAllCalled {
		t.Fatalf("expected UnstageAll not to be called")
	}
}

func TestStatusPaneUnstageAll(t *testing.T) {
	status := &gitmodel.Status{
		Branch:      "main",
		StagedFiles: []gitmodel.File{{Path: "staged.go", StagedStatus: gitmodel.Added}},
	}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after unstage all")
	}
	if !adapter.unstageAllCalled {
		t.Fatalf("expected UnstageAll to be called")
	}
	if adapter.stageAllCalled {
		t.Fatalf("expected StageAll not to be called")
	}
}

func TestStatusPanePush(t *testing.T) {
	status := &gitmodel.Status{
		Branch:   "main",
		Upstream: "origin/main",
		Ahead:    2,
	}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'P', Text: "P"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after push")
	}
	if !adapter.pushCalled {
		t.Fatalf("expected Push to be called")
	}
}

func TestStatusPanePushRequiresUpstream(t *testing.T) {
	status := &gitmodel.Status{Branch: "main", Ahead: 1}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'P', Text: "P"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command when upstream is missing")
	}
	if adapter.pushCalled {
		t.Fatalf("expected Push not to be called without upstream")
	}
	if pane.err == nil || !strings.Contains(pane.err.Error(), "no upstream") {
		t.Fatalf("expected missing upstream error, got %v", pane.err)
	}
}

func TestStatusPanePushRequiresPullBeforePushing(t *testing.T) {
	status := &gitmodel.Status{Branch: "main", Upstream: "origin/main", Ahead: 1, Behind: 1}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'P', Text: "P"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command when branch is diverged")
	}
	if adapter.pushCalled {
		t.Fatalf("expected Push not to be called for diverged branch")
	}
	if pane.err == nil || !strings.Contains(pane.err.Error(), "diverged") {
		t.Fatalf("expected diverged branch error, got %v", pane.err)
	}
}

func TestStatusPanePushShowsNoticeWhenNothingToPush(t *testing.T) {
	status := &gitmodel.Status{Branch: "main", Upstream: "origin/main"}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'P', Text: "P"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no refresh when there is nothing to push")
	}
	if adapter.pushCalled {
		t.Fatalf("expected Push not to be called when ahead is zero")
	}
	if pane.notice != "no local commits to push" {
		t.Fatalf("expected no-push notice, got %q", pane.notice)
	}
}

func TestStatusPaneFetch(t *testing.T) {
	adapter := &fakeGitAdapter{status: &gitmodel.Status{Branch: "main"}, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after fetch")
	}
	if !adapter.fetchCalled {
		t.Fatalf("expected Fetch to be called")
	}
}

func TestStatusPanePull(t *testing.T) {
	status := &gitmodel.Status{Branch: "main", Upstream: "origin/main", Behind: 1}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after pull")
	}
	if !adapter.pullCalled {
		t.Fatalf("expected Pull to be called")
	}
}

func TestStatusPanePullRequiresUpstream(t *testing.T) {
	adapter := &fakeGitAdapter{status: &gitmodel.Status{Branch: "main"}, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command when pull upstream is missing")
	}
	if adapter.pullCalled {
		t.Fatalf("expected Pull not to be called without upstream")
	}
	if pane.err == nil || !strings.Contains(pane.err.Error(), "no upstream") {
		t.Fatalf("expected missing upstream error, got %v", pane.err)
	}
}

func TestStatusPanePullRejectsDivergedBranch(t *testing.T) {
	status := &gitmodel.Status{Branch: "main", Upstream: "origin/main", Ahead: 2, Behind: 1}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no command when pull is blocked by divergence")
	}
	if adapter.pullCalled {
		t.Fatalf("expected Pull not to be called for diverged branch")
	}
	if pane.err == nil || !strings.Contains(pane.err.Error(), "diverged") {
		t.Fatalf("expected diverged pull error, got %v", pane.err)
	}
}

func TestStatusPanePullShowsUpToDateNotice(t *testing.T) {
	status := &gitmodel.Status{Branch: "main", Upstream: "origin/main"}
	adapter := &fakeGitAdapter{status: status, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	pane = updated.(*StatusPane)

	if cmd != nil {
		t.Fatalf("expected no refresh when branch is already up to date")
	}
	if adapter.pullCalled {
		t.Fatalf("expected Pull not to be called when behind is zero")
	}
	if pane.notice != "branch is already up to date with upstream" {
		t.Fatalf("expected up-to-date notice, got %q", pane.notice)
	}
}

func TestStatusPaneFetchShowsNoticeOnSuccess(t *testing.T) {
	adapter := &fakeGitAdapter{status: &gitmodel.Status{Branch: "main"}, watchCh: make(chan adapters.StatusEvent, 2)}
	pane := NewStatusPane("git-1", models.PaneMeta{ID: "git-1", Type: models.PaneTypeGitStatus, CWD: "/repo"}, models.CommonModel{}, adapter)

	msg := runCmd(t, pane.loadStatusCmd())
	updated, _ := pane.Update(msg)
	pane = updated.(*StatusPane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	pane = updated.(*StatusPane)

	if cmd == nil {
		t.Fatalf("expected refresh command after fetch")
	}
	if pane.notice != "fetched latest remote state" {
		t.Fatalf("expected fetch notice, got %q", pane.notice)
	}
}

func runBatchMsg(t *testing.T, cmd tea.Cmd) tea.BatchMsg {
	t.Helper()
	msg := runCmd(t, cmd)
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	return batch
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected command")
	}
	return cmd()
}
