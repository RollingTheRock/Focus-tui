# Git Diff Entry Points Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reintroduce keyboard entry points for the existing independent `DiffPane` from the Git File Tree overlay (`enter` on a file) and the Worktree pane (`v`).

**Architecture:** Reuse the existing `OpenDiffMsg` → `page.openDiffPane()` → `gitplugin.DiffPane` pipeline. Add an optional `RepoPath` field to `OpenDiffMsg` so the Worktree pane can diff a specific worktree; when `RepoPath` is empty, `page.openDiffPane()` falls back to the page's current repo context (used by Git File Tree overlay and StatusPane).

**Tech Stack:** Go 1.25+, Bubble Tea v2, existing `internal/adapters.GitAdapter`.

---

## File Map

| File | Responsibility |
|---|---|
| `internal/plugins/gitfiletree/overlay.go` | Git File Tree overlay. Receives `enter` and dispatches single-file `OpenDiffMsg{FilePath, Staged}`. Also adds `e` as an editor alias. |
| `internal/plugins/git/worktree_pane.go` | Worktree list. Receives `v` and dispatches full-worktree `OpenDiffMsg{RepoPath: wt.Path, FilePath: "", Staged: false}`. Adds `v` to key bindings. |
| `internal/plugins/git/diff_pane.go` | Message type `OpenDiffMsg` with optional `RepoPath`. Independent diff viewer. |
| `internal/app/page.go` | `openDiffPane()` resolves repo context from `msg.RepoPath`, falling back to `p.gitRepoPath()`. |
| `internal/plugins/gitfiletree/overlay_test.go` | Tests for Git File Tree `enter` → diff and `e` → editor. |
| `internal/plugins/git/worktree_pane_test.go` | Tests for Worktree pane `v` → full diff. |
| `internal/plugins/git/diff_pane_test.go` | Regression test for empty `FilePath` → full worktree diff. |

---

### Task 1: Git File Tree `enter` opens single-file diff

**Files:**
- Modify: `internal/plugins/gitfiletree/overlay.go`
- Test: `internal/plugins/gitfiletree/overlay_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plugins/gitfiletree/overlay_test.go`:

```go
func TestOverlayEnterOnFileOpensDiff(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'enter'")
	}
	if cmd == nil {
		t.Fatal("expected diff command")
	}

	msg := cmd()
	diffMsg, ok := msg.(gitplugin.OpenDiffMsg)
	if !ok {
		t.Fatalf("expected OpenDiffMsg, got %T", msg)
	}
	if diffMsg.FilePath != "a.go" {
		t.Fatalf("expected FilePath a.go, got %q", diffMsg.FilePath)
	}
	if diffMsg.Staged {
		t.Fatalf("expected unstaged diff, got staged")
	}
}

func TestOverlayEnterOnStagedFileOpensStagedDiff(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/repo")
	overlay.status = &git.Status{
		StagedFiles: []git.File{{Path: "b.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'enter'")
	}
	if cmd == nil {
		t.Fatal("expected diff command")
	}

	msg := cmd()
	diffMsg, ok := msg.(gitplugin.OpenDiffMsg)
	if !ok {
		t.Fatalf("expected OpenDiffMsg, got %T", msg)
	}
	if diffMsg.FilePath != "b.go" {
		t.Fatalf("expected FilePath b.go, got %q", diffMsg.FilePath)
	}
	if !diffMsg.Staged {
		t.Fatalf("expected staged diff, got unstaged")
	}
}
```

Add the import alias at the top of the test file:

```go
import (
	"testing"

	"github.com/RollingTheRock/Focus-tui/internal/git"
	"github.com/RollingTheRock/Focus-tui/internal/models"
	gitplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/git"

	tea "charm.land/bubbletea/v2"
)
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/plugins/gitfiletree -run TestOverlayEnterOnFile -v
```

Expected: FAIL — `undefined: gitplugin` or `OpenDiffMsg` type mismatch.

- [ ] **Step 3: Import git plugin and add `handleDiff()`**

In `internal/plugins/gitfiletree/overlay.go`, add the import alias:

```go
gitplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/git"
```

Change `updateKeyLeftPane()` so `enter` on a file calls `handleDiff()`:

```go
func (o *GitFileTreeOverlay) updateKeyLeftPane(msg tea.KeyPressMsg) (models.Panel, tea.Cmd) {
	switch msg.Keystroke() {
	case "j", "down":
		if o.treeCursor < len(o.treeFlatList)-1 {
			o.treeCursor++
		}
		return o, nil
	case "k", "up":
		if o.treeCursor > 0 {
			o.treeCursor--
		}
		return o, nil
	case "enter":
		return o.handleEnter()
	case "left", "right":
		return o.handleTreeToggle(msg.Keystroke())
	case "space":
		return o.handleStageToggle()
	case "o":
		return o, o.handleOpenFile()
	case "e":
		return o, o.handleOpenFile()
	case "a":
		return o, o.handleStageAllToggle()
	case "c":
		return o.handleCommit()
	case "d":
		return o, o.handleDiscard()
	}
	return o, nil
}
```

Update `handleEnter()` to keep directory toggle but send files to editor:

```go
func (o *GitFileTreeOverlay) handleEnter() (models.Panel, tea.Cmd) {
	node := o.selectedTreeNode()
	if node == nil {
		return o, nil
	}
	if !node.IsDir {
		return o, o.handleDiff()
	}
	// Directory: toggle collapse
	node.Collapsed = !node.Collapsed
	selectedPath := node.Path
	o.treeFlatList = flattenVisibleGitNodes(o.treeRoot, true)
	o.selectTreeNodeByPath(selectedPath)
	return o, nil
}
```

Add `handleDiff()` in the `// --- Actions ---` section:

```go
func (o *GitFileTreeOverlay) handleDiff() tea.Cmd {
	node := o.selectedTreeNode()
	if node == nil || node.IsDir || o.adapter == nil {
		return nil
	}
	path := node.Path
	staged := o.fileGitStatus(path) == "staged"
	return func() tea.Msg {
		return gitplugin.OpenDiffMsg{FilePath: path, Staged: staged}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/plugins/gitfiletree -run TestOverlayEnterOnFile -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/gitfiletree/overlay.go internal/plugins/gitfiletree/overlay_test.go
git commit -m "feat(gitfiletree): open diff pane on enter for changed files"
```

---

### Task 2: Git File Tree `e` opens editor as alias

**Files:**
- Modify: `internal/plugins/gitfiletree/overlay.go` (already done in Task 1)
- Test: `internal/plugins/gitfiletree/overlay_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plugins/gitfiletree/overlay_test.go`:

```go
func TestOverlayEOpensEditor(t *testing.T) {
	overlay := NewOverlay("test", models.PaneMeta{ID: "test"}, models.CommonModel{}, nil, "/repo")
	overlay.status = &git.Status{
		UnstagedFiles: []git.File{{Path: "a.go"}},
	}
	overlay.rebuildTree()

	updated, cmd := overlay.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if updated == nil {
		t.Fatal("Update returned nil panel for 'e'")
	}
	if cmd == nil {
		t.Fatal("expected editor command")
	}

	msg := cmd()
	editorMsg, ok := msg.(editor.OpenEditorMsg)
	if !ok {
		t.Fatalf("expected OpenEditorMsg, got %T", msg)
	}
	if editorMsg.FilePath != "a.go" {
		t.Fatalf("expected FilePath a.go, got %q", editorMsg.FilePath)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/plugins/gitfiletree -run TestOverlayEOpensEditor -v
```

Expected: FAIL — `e` key not handled yet.

- [ ] **Step 3: Add `e` key handling**

Already added in Task 1. If not present, add `case "e":` to `updateKeyLeftPane()` returning `o.handleOpenFile()`.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/plugins/gitfiletree -run TestOverlayEOpensEditor -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/gitfiletree/overlay.go internal/plugins/gitfiletree/overlay_test.go
git commit -m "feat(gitfiletree): add e key alias to open editor"
```

---

### Task 3: Worktree pane `v` opens full-worktree diff for the selected worktree

**Files:**
- Modify: `internal/plugins/git/worktree_pane.go`
- Modify: `internal/plugins/git/diff_pane.go` (add `RepoPath` to `OpenDiffMsg`)
- Modify: `internal/app/page.go` (use `msg.RepoPath` in `openDiffPane()`)
- Test: `internal/plugins/git/worktree_pane_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plugins/git/worktree_pane_test.go`:

```go
func TestWorktreePaneVOpensFullDiff(t *testing.T) {
	adapter := &fakeGitAdapter{
		worktrees: []gitmodel.Worktree{
			{Path: "/repo/main", Branch: "main", IsMain: true},
			{Path: "/repo/feature-a", Branch: "feature-a"},
		},
	}
	pane := NewWorktreePane("worktree-1", models.PaneMeta{ID: "worktree-1", Type: models.PaneTypeWorktree, CWD: "/repo/main"}, models.CommonModel{}, adapter)
	updated, _ := pane.Update(worktreesLoadedMsg{worktrees: adapter.worktrees})
	pane = updated.(*WorktreePane)
	updated, _ = pane.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	pane = updated.(*WorktreePane)

	updated, cmd := pane.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	pane = updated.(*WorktreePane)
	if cmd == nil {
		t.Fatalf("expected diff command")
	}
	msg := runCmd(t, cmd)
	diffMsg, ok := msg.(OpenDiffMsg)
	if !ok {
		t.Fatalf("expected OpenDiffMsg, got %T", msg)
	}
	if diffMsg.RepoPath != "/repo/feature-a" {
		t.Fatalf("expected RepoPath /repo/feature-a, got %q", diffMsg.RepoPath)
	}
	if diffMsg.FilePath != "" {
		t.Fatalf("expected empty FilePath for full worktree diff, got %q", diffMsg.FilePath)
	}
	if diffMsg.Staged {
		t.Fatalf("expected unstaged full worktree diff")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/plugins/git -run TestWorktreePaneVOpensFullDiff -v
```

Expected: FAIL — `v` key not handled yet.

- [ ] **Step 3: Add `v` key handling and `RepoPath` support**

In `internal/plugins/git/diff_pane.go`, add `RepoPath` to `OpenDiffMsg`:

```go
type OpenDiffMsg struct {
	RepoPath string
	FilePath string
	Staged   bool
}
```

In `internal/app/page.go`, update `openDiffPane()` to use `msg.RepoPath` when provided:

```go
repoPath := msg.RepoPath
if repoPath == "" {
	repoPath = p.gitRepoPath()
}
meta := models.PaneMeta{
	...
	CWD: repoPath,
	...
}
```

In `internal/plugins/git/worktree_pane.go`, add `case "v"` to the `tea.KeyPressMsg` switch in `Update()`:

```go
		case "v":
			if wt, ok := p.selectedWorktree(); ok {
				p.notice = "Opening diff for " + shortenWorktreePath(wt.Path)
				return p, func() tea.Msg {
					return OpenDiffMsg{RepoPath: wt.Path, FilePath: "", Staged: false}
				}
			}
```

Add `v` to `KeyBindings()`:

```go
func (p *WorktreePane) KeyBindings(compact bool) []models.KeyBinding {
	if compact {
		return []models.KeyBinding{
			{Keys: []string{"j", "k"}, Help: "nav"},
			{Keys: []string{"enter"}, Help: "select"},
			{Keys: []string{"n"}, Help: "new"},
			{Keys: []string{"e"}, Help: "edit"},
			{Keys: []string{"d"}, Help: "del"},
			{Keys: []string{"v"}, Help: "diff"},
		}
	}
	return []models.KeyBinding{
		{Keys: []string{"j", "k"}, Help: "nav"},
		{Keys: []string{"enter"}, Help: "select"},
		{Keys: []string{"n"}, Help: "new"},
		{Keys: []string{"e"}, Help: "edit"},
		{Keys: []string{"d"}, Help: "del"},
		{Keys: []string{"o"}, Help: "shell"},
		{Keys: []string{"v"}, Help: "diff"},
		{Keys: []string{"tab"}, Help: "cycle focus"},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/plugins/git -run TestWorktreePaneVOpensFullDiff -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/git/worktree_pane.go internal/plugins/git/worktree_pane_test.go internal/plugins/git/diff_pane.go internal/app/page.go
git commit -m "feat(git): open full worktree diff from worktree pane with v"
```

---

### Task 4: Validate DiffPane handles empty `FilePath` as full worktree diff

**Files:**
- Modify: `internal/plugins/git/diff_pane.go` (only if test reveals a bug)
- Test: `internal/plugins/git/diff_pane_test.go`

- [ ] **Step 1: Inspect current behavior**

Read `internal/plugins/git/diff_pane.go` around `loadDiffCmd()` (lines ~251-283) and confirm it calls:

```go
p.adapter.GetDiff(p.repoPath, p.filePath, p.staged)
```

- [ ] **Step 2: Write the test**

Append to `internal/plugins/git/diff_pane_test.go`:

```go
func TestDiffPaneEmptyFilePathLoadsFullWorktreeDiff(t *testing.T) {
	adapter := &fakeGitAdapter{}
	pane := NewDiffPane("diff-1", models.PaneMeta{ID: "diff-1", Type: models.PaneTypeDiffView, CWD: "/repo"}, models.CommonModel{}, adapter, "", false)
	pane = initPane(t, pane).(*DiffPane)

	if pane.filePath != "" {
		t.Fatalf("expected empty file path, got %q", pane.filePath)
	}
	if pane.staged {
		t.Fatalf("expected unstaged")
	}
}
```

- [ ] **Step 3: Run test**

```bash
go test ./internal/plugins/git -run TestDiffPaneEmptyFilePathLoadsFullWorktreeDiff -v
```

Expected: PASS. If it fails, adjust `loadDiffCmd()` to handle `p.filePath == ""` by passing an empty path to `GetDiff`.

- [ ] **Step 4: Commit**

```bash
git add internal/plugins/git/diff_pane.go internal/plugins/git/diff_pane_test.go
git commit -m "test(git): verify empty diff path loads full worktree diff"
```

---

### Task 5: Regression test existing Git File Tree behavior

**Files:**
- Test: `internal/plugins/gitfiletree/overlay_test.go`

- [ ] **Step 1: Ensure existing tests still pass**

```bash
go test ./internal/plugins/gitfiletree -v
```

Expected: all PASS, including:
- `TestOverlayEscClosesOverlay`
- `TestOverlayJMovesCursor`
- `TestOverlaySpaceStagesFile`
- `TestOverlayEnterTogglesDir`

- [ ] **Step 2: If `TestOverlayEnterTogglesDir` fails, fix directory enter handling**

The updated `handleEnter()` keeps directory toggle, so this should pass. If not, verify that `handleEnter()` distinguishes files from directories.

- [ ] **Step 3: Commit if any fixes were needed**

```bash
git add internal/plugins/gitfiletree/overlay.go
git commit -m "fix(gitfiletree): preserve directory toggle on enter"
```

If no fixes needed, skip this commit.

---

### Task 6: Run full targeted test suite

**Files:** all modified

- [ ] **Step 1: Run all git and gitfiletree tests**

```bash
go test ./internal/plugins/git/... ./internal/plugins/gitfiletree/... -v
```

Expected: all PASS.

- [ ] **Step 2: Run the full Go test suite**

```bash
go test ./...
```

Expected: all PASS. If not, fix regressions.

- [ ] **Step 3: Final commit**

```bash
git commit -m "test(git): add diff entry point tests and validate full suite" --allow-empty
```

---

## Spec Coverage Check

| Spec Requirement | Implementing Task |
|---|---|
| Git File Tree `enter` on file opens `DiffPane` | Task 1 |
| Git File Tree `e` opens editor alias | Task 2 |
| Git File Tree `enter` on directory still toggles | Task 1 + Task 5 regression |
| Worktree pane `v` opens full-worktree diff | Task 3 |
| Worktree pane `d` still deletes | Task 3 (no change) |
| DiffPane handles empty `FilePath` | Task 4 |
| Tests cover new behavior | Tasks 1-4 |

## Open Risks

1. **Import cycle:** `overlay.go` will import `internal/plugins/git`. Verify that `internal/plugins/git` does not import `internal/plugins/gitfiletree` (it currently does not).
2. **Key collision:** `v` is already used inside `DiffPane` to toggle split/unified. This is fine because `DiffPane` is a separate panel with focus; the Worktree pane's `v` only applies when the Worktree pane is focused.
3. **Adapter nil check:** `handleDiff()` guards against `nil` adapter. If the overlay is opened without an adapter, `enter` on a file does nothing.
