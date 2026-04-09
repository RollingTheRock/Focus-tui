package git

import (
	"errors"
	"strings"
	"testing"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "github.com/charmbracelet/bubbletea"
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
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
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

	updated, _ := pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*StatusPane)
	if pane.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", pane.cursor)
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane = updated.(*StatusPane)
	if pane.cursor != 1 {
		t.Fatalf("expected cursor to clamp at 1, got %d", pane.cursor)
	}

	updated, _ = pane.Update(tea.KeyMsg{Type: tea.KeyUp})
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

	if !strings.Contains(pane.View(), "not a git repository") {
		t.Fatalf("expected error view, got %q", pane.View())
	}
}

type fakeGitAdapter struct {
	status       *gitmodel.Status
	getStatusErr error
	watchErr     error
	watchCh      chan adapters.StatusEvent
}

func (f *fakeGitAdapter) Name() string { return "fake-git" }

func (f *fakeGitAdapter) Init() error { return nil }

func (f *fakeGitAdapter) Destroy() error { return nil }

func (f *fakeGitAdapter) GetStatus(repoPath string) (*gitmodel.Status, error) {
	return f.status, f.getStatusErr
}

func (f *fakeGitAdapter) GetBranches(repoPath string) ([]gitmodel.Branch, error) { return nil, nil }

func (f *fakeGitAdapter) GetDiff(repoPath string, path string, staged bool) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) WatchStatus(repoPath string) (<-chan adapters.StatusEvent, error) {
	if f.watchErr != nil {
		return nil, f.watchErr
	}
	return f.watchCh, nil
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
