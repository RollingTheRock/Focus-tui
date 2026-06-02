package git

import (
	"strings"
	"testing"

	"focus/internal/adapters"
	gitmodel "focus/internal/git"
	"focus/internal/models"

	tea "charm.land/bubbletea/v2"
)

func TestCommitPaneInitRendersStagedFiles(t *testing.T) {
	adapter := &fakeCommitAdapter{}
	pane := NewCommitPane("commit-1", models.PaneMeta{ID: "commit-1", CWD: "/repo"}, models.CommonModel{}, adapter, []gitmodel.File{
		{Path: "main.go", StagedStatus: gitmodel.Modified},
		{Path: "README.md", StagedStatus: gitmodel.Added},
	})
	pane.SetSize(80, 20)

	if pane.Init() == nil {
		t.Fatalf("expected init command")
	}
	if !pane.input.Focused() {
		t.Fatalf("expected commit input to be focused after init")
	}

	view := pane.View()
	for _, want := range []string{"Commit", "main.go", "README.md", "Subject 0/50 chars", "Ctrl+S commit", "Ctrl+J fallback"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view.Content)
		}
	}
}

func TestCommitPaneInputHandlingSupportsMultilineMessage(t *testing.T) {
	adapter := &fakeCommitAdapter{}
	pane := NewCommitPane("commit-1", models.PaneMeta{ID: "commit-1", CWD: "/repo"}, models.CommonModel{}, adapter, []gitmodel.File{{Path: "main.go", StagedStatus: gitmodel.Modified}})
	pane.SetSize(80, 20)
	pane.Init()

	updated, _ := pane.Update(tea.KeyPressMsg{Code: 'f', Text: "feat"})
	pane = updated.(*CommitPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pane = updated.(*CommitPane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: 'b', Text: "body"})
	pane = updated.(*CommitPane)

	if got := pane.input.Value(); got != "feat\nbody" {
		t.Fatalf("expected multiline message, got %q", got)
	}
	if !strings.Contains(pane.View().Content, "Subject 4/50 chars") {
		t.Fatalf("expected subject length hint, got:\n%s", pane.View().Content)
	}
}

func TestStatusPaneCKeyOpensCommitPane(t *testing.T) {
	pane := &StatusPane{
		repoPath: "/repo",
		status: &gitmodel.Status{
			StagedFiles: []gitmodel.File{{Path: "main.go", StagedStatus: gitmodel.Modified}},
		},
	}

	_, cmd := pane.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	msg := runCmd(t, cmd)
	openMsg, ok := msg.(OpenCommitMsg)
	if !ok {
		t.Fatalf("expected OpenCommitMsg, got %T", msg)
	}
	if openMsg.RepoPath != "/repo" {
		t.Fatalf("expected repo path /repo, got %q", openMsg.RepoPath)
	}
	if len(openMsg.StagedFiles) != 1 || openMsg.StagedFiles[0].Path != "main.go" {
		t.Fatalf("expected staged file main.go, got %#v", openMsg.StagedFiles)
	}
}

func TestCommitPaneCommitExecutionSendsCompletionMessageWithCtrlS(t *testing.T) {
	adapter := &fakeCommitAdapter{}
	pane := NewCommitPane("commit-1", models.PaneMeta{ID: "commit-1", CWD: "/repo"}, models.CommonModel{}, adapter, []gitmodel.File{{Path: "main.go", StagedStatus: gitmodel.Modified}})
	pane.SetSize(80, 20)
	pane.Init()
	pane.input.SetValue("feat: add commit pane\n\ninclude commit workflow")

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl, Text: "s"})
	pane = updated.(*CommitPane)
	if cmd == nil {
		t.Fatalf("expected commit command")
	}
	if adapter.commitCalled {
		t.Fatalf("expected commit not to run until the command executes")
	}

	updated, cmd = pane.Update(runCmd(t, cmd))
	pane = updated.(*CommitPane)
	if !adapter.commitCalled {
		t.Fatalf("expected adapter commit to be called")
	}
	if adapter.commitRepoPath != "/repo" {
		t.Fatalf("expected commit repo path /repo, got %q", adapter.commitRepoPath)
	}
	if adapter.commitMessage != "feat: add commit pane\n\ninclude commit workflow" {
		t.Fatalf("unexpected commit message %q", adapter.commitMessage)
	}
	if cmd == nil {
		t.Fatalf("expected completion command")
	}

	msg := runCmd(t, cmd)
	completedMsg, ok := msg.(CommitCompletedMsg)
	if !ok {
		t.Fatalf("expected CommitCompletedMsg, got %T", msg)
	}
	if completedMsg.ID != "commit-1" {
		t.Fatalf("expected pane id commit-1, got %q", completedMsg.ID)
	}
	if completedMsg.RepoPath != "/repo" {
		t.Fatalf("expected repo path /repo, got %q", completedMsg.RepoPath)
	}
}

func TestCommitPaneCommitExecutionStillSupportsCtrlJ(t *testing.T) {
	adapter := &fakeCommitAdapter{}
	pane := NewCommitPane("commit-1", models.PaneMeta{ID: "commit-1", CWD: "/repo"}, models.CommonModel{}, adapter, []gitmodel.File{{Path: "main.go", StagedStatus: gitmodel.Modified}})
	pane.SetSize(80, 20)
	pane.Init()
	pane.input.SetValue("feat: keep ctrl+j support")

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl, Text: "j"})
	pane = updated.(*CommitPane)
	if cmd == nil {
		t.Fatalf("expected commit command for ctrl+j fallback")
	}

	updated, _ = pane.Update(runCmd(t, cmd))
	pane = updated.(*CommitPane)
	if !adapter.commitCalled {
		t.Fatalf("expected adapter commit to be called for ctrl+j fallback")
	}
	if adapter.commitMessage != "feat: keep ctrl+j support" {
		t.Fatalf("unexpected commit message %q", adapter.commitMessage)
	}
}

func TestCommitPaneCancelOperation(t *testing.T) {
	adapter := &fakeCommitAdapter{}
	pane := NewCommitPane("commit-1", models.PaneMeta{ID: "commit-1", CWD: "/repo"}, models.CommonModel{}, adapter, []gitmodel.File{{Path: "main.go", StagedStatus: gitmodel.Modified}})
	pane.SetSize(80, 20)
	pane.Init()

	_, cmd := pane.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	msg := runCmd(t, cmd)
	closeMsg, ok := msg.(CloseCommitMsg)
	if !ok {
		t.Fatalf("expected CloseCommitMsg, got %T", msg)
	}
	if closeMsg.ID != "commit-1" {
		t.Fatalf("expected pane id commit-1, got %q", closeMsg.ID)
	}
	if adapter.commitCalled {
		t.Fatalf("expected commit not to run on cancel")
	}
}

func (f *fakeGitAdapter) Commit(repoPath, message string) error {
	return nil
}

type fakeCommitAdapter struct {
	commitCalled   bool
	commitRepoPath string
	commitMessage  string
	commitErr      error
	watchCh        chan adapters.StatusEvent
}

func (f *fakeCommitAdapter) Name() string { return "fake-commit-git" }

func (f *fakeCommitAdapter) Init() error { return nil }

func (f *fakeCommitAdapter) Destroy() error { return nil }

func (f *fakeCommitAdapter) GetStatus(repoPath string) (*gitmodel.Status, error) { return nil, nil }

func (f *fakeCommitAdapter) GetWorktreeStatus(worktreePath string) (*gitmodel.Status, error) {
	return nil, nil
}

func (f *fakeCommitAdapter) GetBranches(repoPath string) ([]gitmodel.Branch, error) { return nil, nil }

func (f *fakeCommitAdapter) ListWorktrees(repoPath string) ([]gitmodel.Worktree, error) {
	return nil, nil
}

func (f *fakeCommitAdapter) CreateWorktree(repoPath string, req gitmodel.CreateWorktreeRequest) (*gitmodel.Worktree, error) {
	return nil, nil
}

func (f *fakeCommitAdapter) RemoveWorktree(repoPath, worktreePath string, opts gitmodel.RemoveWorktreeOptions) error {
	return nil
}

func (f *fakeCommitAdapter) PruneWorktrees(repoPath string) error { return nil }

func (f *fakeCommitAdapter) GetDiff(repoPath string, path string, staged bool) (string, error) {
	return "", nil
}

func (f *fakeCommitAdapter) GetFileContent(repoPath string, path string, ref string) (string, error) {
	return "", nil
}

func (f *fakeCommitAdapter) Commit(repoPath, message string) error {
	f.commitCalled = true
	f.commitRepoPath = repoPath
	f.commitMessage = message
	return f.commitErr
}

func (f *fakeCommitAdapter) Fetch(repoPath string) error { return nil }

func (f *fakeCommitAdapter) Pull(repoPath string) error { return nil }

func (f *fakeCommitAdapter) Push(repoPath string) error { return nil }

func (f *fakeCommitAdapter) WatchStatus(repoPath string) (<-chan adapters.StatusEvent, error) {
	if f.watchCh == nil {
		f.watchCh = make(chan adapters.StatusEvent)
	}
	return f.watchCh, nil
}

func (f *fakeCommitAdapter) StopWatch(repoPath string) {}

func (f *fakeCommitAdapter) StageFile(repoPath string, path string) error { return nil }

func (f *fakeCommitAdapter) StageAll(repoPath string) error { return nil }

func (f *fakeCommitAdapter) UnstageFile(repoPath string, path string) error { return nil }

func (f *fakeCommitAdapter) UnstageAll(repoPath string) error { return nil }

func (f *fakeCommitAdapter) DiscardChanges(repoPath string, path string) error { return nil }

func (f *fakeCommitAdapter) GetStashList(repoPath string) ([]adapters.StashEntry, error) { return nil, nil }
func (f *fakeCommitAdapter) StashApply(repoPath string, index int) error { return nil }
func (f *fakeCommitAdapter) StashPop(repoPath string, index int) error { return nil }
func (f *fakeCommitAdapter) StashDrop(repoPath string, index int) error { return nil }
func (f *fakeCommitAdapter) CheckoutBranch(repoPath string, branch string) error { return nil }
