package gitfiletree

import (
	"fmt"
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/adapters"
	"github.com/RollingTheRock/Focus-tui/internal/git"
	"github.com/RollingTheRock/Focus-tui/internal/models"

	tea "charm.land/bubbletea/v2"
)

// mockGitAdapter 模拟 git 操作，验证 stage 流程
type mockGitAdapter struct {
	status      *git.Status
	stageCalls    []string
	unstageCalls  []string
	discardCalls  []string
	commitCalls   []string
}

func (m *mockGitAdapter) Name() string { return "mock" }
func (m *mockGitAdapter) Init() error  { return nil }
func (m *mockGitAdapter) Destroy() error { return nil }
func (m *mockGitAdapter) GetStatus(repoPath string) (*git.Status, error) { return m.status, nil }
func (m *mockGitAdapter) GetWorktreeStatus(worktreePath string) (*git.Status, error) { return m.status, nil }
func (m *mockGitAdapter) GetBranches(repoPath string) ([]git.Branch, error) { return nil, nil }
func (m *mockGitAdapter) ListWorktrees(repoPath string) ([]git.Worktree, error) { return nil, nil }
func (m *mockGitAdapter) CreateWorktree(repoPath string, req git.CreateWorktreeRequest) (*git.Worktree, error) { return nil, nil }
func (m *mockGitAdapter) RemoveWorktree(repoPath, worktreePath string, opts git.RemoveWorktreeOptions) error { return nil }
func (m *mockGitAdapter) PruneWorktrees(repoPath string) error { return nil }
func (m *mockGitAdapter) GetDiff(repoPath string, path string, staged bool) (string, error) { return "", nil }
func (m *mockGitAdapter) Commit(repoPath, message string) error {
	m.commitCalls = append(m.commitCalls, message)
	return nil
}
func (m *mockGitAdapter) Fetch(repoPath string) error { return nil }
func (m *mockGitAdapter) Pull(repoPath string) error { return nil }
func (m *mockGitAdapter) Push(repoPath string) error { return nil }
func (m *mockGitAdapter) WatchStatus(repoPath string) (<-chan adapters.StatusEvent, error) { return nil, nil }
func (m *mockGitAdapter) StopWatch(repoPath string) {}
func (m *mockGitAdapter) StageFile(repoPath string, path string) error {
	m.stageCalls = append(m.stageCalls, path)
	// 模拟：从 unstaged/untracked 移到 staged，并设置 StagedStatus
	for i, f := range m.status.UnstagedFiles {
		if f.Path == path {
			f.StagedStatus = git.Modified
			m.status.StagedFiles = append(m.status.StagedFiles, f)
			m.status.UnstagedFiles = append(m.status.UnstagedFiles[:i], m.status.UnstagedFiles[i+1:]...)
			break
		}
	}
	for i, f := range m.status.UntrackedFiles {
		if f.Path == path {
			f.StagedStatus = git.Added
			m.status.StagedFiles = append(m.status.StagedFiles, f)
			m.status.UntrackedFiles = append(m.status.UntrackedFiles[:i], m.status.UntrackedFiles[i+1:]...)
			break
		}
	}
	return nil
}
func (m *mockGitAdapter) StageAll(repoPath string) error { return nil }
func (m *mockGitAdapter) UnstageFile(repoPath string, path string) error {
	m.unstageCalls = append(m.unstageCalls, path)
	return nil
}
func (m *mockGitAdapter) UnstageAll(repoPath string) error { return nil }
func (m *mockGitAdapter) DiscardChanges(repoPath string, path string) error {
	m.discardCalls = append(m.discardCalls, path)
	return nil
}
func (m *mockGitAdapter) GetFileContent(repoPath string, path string, ref string) (string, error) { return "", nil }
func (m *mockGitAdapter) GetStashList(repoPath string) ([]adapters.StashEntry, error) { return nil, nil }
func (m *mockGitAdapter) StashApply(repoPath string, index int) error { return nil }
func (m *mockGitAdapter) StashPop(repoPath string, index int) error { return nil }
func (m *mockGitAdapter) StashDrop(repoPath string, index int) error { return nil }
func (m *mockGitAdapter) CheckoutBranch(repoPath string, branch string) error { return nil }

// TestFileGitStatusPathMatching 验证 fileGitStatus 路径匹配
func TestFileGitStatusPathMatching(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/tmp/test-repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{
			{Path: "internal/app/page.go"},
			{Path: "README.md"},
		},
		StagedFiles: []git.File{
			{Path: "internal/plugins/gitfiletree/overlay.go"},
		},
	}
	overlay.rebuildTree()

	// 打印树结构
	fmt.Println("Tree structure:")
	for i, n := range overlay.treeFlatList {
		fmt.Printf("  [%d] name=%q path=%q isDir=%v status=%d\n", i, n.Name, n.Path, n.IsDir, n.Status)
	}

	// 验证每个文件节点的 fileGitStatus
	for _, n := range overlay.treeFlatList {
		if n.IsDir {
			continue
		}
		status := overlay.fileGitStatus(n.Path)
		fmt.Printf("fileGitStatus(%q) = %q\n", n.Path, status)
		if status == "" {
			t.Fatalf("fileGitStatus(%q) returned empty - path mismatch!", n.Path)
		}
	}
}

// TestStageWithMockAdapter 验证完整 stage 流程
func TestStageWithMockAdapter(t *testing.T) {
	mock := &mockGitAdapter{
		status: &git.Status{
			Branch: "main",
			UnstagedFiles: []git.File{
				{Path: "internal/app/page.go"},
			},
		},
	}
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, mock, "/tmp/test-repo")
	overlay.status = mock.status
	overlay.rebuildTree()

	fmt.Println("Before stage:")
	for i, n := range overlay.treeFlatList {
		fmt.Printf("  [%d] name=%q path=%q isDir=%v\n", i, n.Name, n.Path, n.IsDir)
	}
	fmt.Printf("Staged files: %d\n", len(overlay.status.StagedFiles))

	// 选中文件节点（page.go）
	for i, n := range overlay.treeFlatList {
		if n.Name == "page.go" {
			overlay.treeCursor = i
			break
		}
	}

	// 按 space (bubbletea v2 中 Keystroke() 返回 "space")
	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if updated == nil {
		t.Fatal("Update returned nil")
	}
	if cmd == nil {
		t.Fatal("space should return refresh cmd")
	}

	fmt.Printf("StageFile called with: %v\n", mock.stageCalls)
	if len(mock.stageCalls) == 0 {
		t.Fatal("StageFile was not called")
	}
	if mock.stageCalls[0] != "internal/app/page.go" {
		t.Fatalf("StageFile called with wrong path: %q", mock.stageCalls[0])
	}

	// 模拟 refresh：执行 cmd
	msg := cmd()
	if msg == nil {
		t.Fatal("refresh cmd returned nil msg")
	}

	// 更新 overlay 状态（模拟 bubbletea 的行为）
	updated2, _ := updated.Update(msg)
	if updated2 == nil {
		t.Fatal("Update with refresh msg returned nil")
	}
	o2 := updated2.(*GitFileTreeOverlay)

	fmt.Printf("After refresh: Staged=%d Unstaged=%d\n",
		len(o2.status.StagedFiles), len(o2.status.UnstagedFiles))

	if len(o2.status.StagedFiles) != 1 {
		t.Fatalf("expected 1 staged file after refresh, got %d", len(o2.status.StagedFiles))
	}
	if len(o2.status.UnstagedFiles) != 0 {
		t.Fatalf("expected 0 unstaged files after refresh, got %d", len(o2.status.UnstagedFiles))
	}
}

// TestCommitInputShowsWhenStagedExists 验证有 staged 文件时 commit 输入显示
func TestCommitInputShowsWhenStagedExists(t *testing.T) {
	mock := &mockGitAdapter{
		status: &git.Status{
			Branch:      "main",
			StagedFiles: []git.File{{Path: "a.go", StagedStatus: git.Added}},
		},
	}
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, mock, "/tmp/test-repo")
	overlay.status = mock.status
	overlay.rebuildTree()
	overlay.SetSize(100, 10)

	// 按 c
	updated, _ := overlay.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	o2 := updated.(*GitFileTreeOverlay)

	if o2.commitMode != commitInputNormal {
		t.Fatalf("expected commitMode=commitInputNormal after 'c', got %d", o2.commitMode)
	}

	view := o2.View().Content
	if !contains(view, "Message:") {
		t.Fatalf("commit input not shown in view. View:\n%s", view)
	}
	fmt.Printf("Commit input shown: OK\n%s\n", view)
}

// TestStageThenCommitFlow 验证完整的 stage → commit 端到端流程
func TestStageThenCommitFlow(t *testing.T) {
	mock := &mockGitAdapter{
		status: &git.Status{
			Branch: "main",
			UnstagedFiles: []git.File{
				{Path: "main.go"},
			},
		},
	}
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, mock, "/tmp/test-repo")
	overlay.status = mock.status
	overlay.rebuildTree()
	overlay.SetSize(100, 10)

	// 1. 选中文件并按 space stage
	for i, n := range overlay.treeFlatList {
		if n.Name == "main.go" {
			overlay.treeCursor = i
			break
		}
	}
	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	// 2. 模拟 refresh cmd 执行（adapter.StageFile 已在 handleStageToggle 中调用）
	msg := cmd()
	updated2, _ := updated.Update(msg)
	o2 := updated2.(*GitFileTreeOverlay)

	if len(o2.status.StagedFiles) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(o2.status.StagedFiles))
	}
	fmt.Printf("Stage OK: StagedFiles=%d\n", len(o2.status.StagedFiles))

	// 3. 按 c 进入 commit 模式
	updated3, _ := o2.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	o3 := updated3.(*GitFileTreeOverlay)
	if o3.commitMode != commitInputNormal {
		t.Fatalf("expected commit mode, got %d", o3.commitMode)
	}
	fmt.Printf("Commit mode entered: OK\n")

	// 4. 输入 commit message "feat: init"
	for _, ch := range "feat: init" {
		updated3, _ = o3.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		o3 = updated3.(*GitFileTreeOverlay)
	}
	if o3.commitInput != "feat: init" {
		t.Fatalf("expected commitInput='feat: init', got %q", o3.commitInput)
	}
	fmt.Printf("Commit message typed: %q\n", o3.commitInput)

	// 5. 按 enter 确认 commit
	updated4, cmd4 := o3.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	o4 := updated4.(*GitFileTreeOverlay)

	// commit 应该已提交，mode 重置
	if o4.commitMode != commitInputNone {
		t.Fatalf("expected commitMode reset to None, got %d", o4.commitMode)
	}

	// 执行 commit cmd
	cmd4()

	if len(mock.commitCalls) != 1 {
		t.Fatalf("expected 1 commit call, got %d", len(mock.commitCalls))
	}
	if mock.commitCalls[0] != "feat: init" {
		t.Fatalf("expected commit message 'feat: init', got %q", mock.commitCalls[0])
	}
	fmt.Printf("Commit submitted: %q — OK\n", mock.commitCalls[0])
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}
func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
