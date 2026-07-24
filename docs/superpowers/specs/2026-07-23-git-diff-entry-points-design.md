# Git Diff Entry Points Design

## 1. Background & Goal

`focus` has a fully functional independent `DiffPane` (`internal/plugins/git/diff_pane.go`) that can render both single-file diffs and full-worktree review diffs. However, after the Git File Tree overlay was rewritten as a lazygit-style dual-pane layout (`c90ce9c`), the previous diff-preview entry point was removed and no replacement was added. The standalone `StatusPane` can still open diffs, but it is not part of the default layout and has no user-facing shortcut.

This leaves users with no way to open the diff panel from the normal TUI flow.

**Goal:** Reintroduce clear, discoverable keyboard entry points for the existing `DiffPane` without disrupting the current panel architecture or the lazygit-inspired Git File Tree overlay.

## 2. Current State

- `DiffPane` exists and handles `OpenDiffMsg` correctly.
- `OpenDiffMsg` is dispatched only from `internal/plugins/git/status_pane.go` (`enter` / `d`).
- `StatusPane` is not created in the default layout and has no shortcut to open it.
- Git File Tree overlay left pane:
  - `enter` on a file opens the editor.
  - `o` on a file opens the editor.
  - `d` discards changes.
  - `space` stages / unstages.
- Worktree pane:
  - `enter` resumes / selects a worktree.
  - `d` deletes a worktree.
  - `o` opens a shell in the worktree.
  - `a` starts an agent in the worktree.

## 3. Design Decisions

1. **Reuse the existing `DiffPane` rather than reintroducing an embedded preview.** The embedded preview was removed to make room for stash/branch panes; restoring it would fight the new lazygit-style layout. An independent panel is more flexible and already supports split/unified views, staged/unstaged toggling, and file-to-file navigation.
2. **Align Git File Tree file selection with lazygit conventions.** In lazygit, `enter` on a changed file opens the diff; the editor is a secondary action. We adopt the same convention.
3. **Keep destructive keys unchanged.** `d` remains discard in Git File Tree and delete in Worktree pane. This avoids breaking existing muscle memory.
4. **Use `v` for worktree-level diff.** It is unbound in the Worktree pane, visually mnemonic for "view diff / review", and does not conflict with any existing action.
5. **Make `OpenDiffMsg` worktree-aware.** `OpenDiffMsg` carries an explicit `RepoPath` field. Senders that target a specific worktree set it; senders that rely on the current page context leave it empty and `page.openDiffPane()` falls back to `p.gitRepoPath()`.

## 4. Keybinding Changes

### 4.1 Git File Tree Overlay — Left Pane

| Key | Current | New |
|---|---|---|
| `enter` on file | Open editor | **Open `DiffPane` for selected file** |
| `enter` on directory | Toggle collapse | Toggle collapse (unchanged) |
| `o` on file | Open editor | Open editor (unchanged) |
| `e` on file | — | **Open editor (alias)** |
| `d` | Discard changes | Discard changes (unchanged) |
| `space` | Stage / unstage | Stage / unstage (unchanged) |

### 4.2 Worktree Pane

| Key | Current | New |
|---|---|---|
| `v` | — | **Open `DiffPane` for full worktree review diff** |
| `enter` | Resume / select worktree | Unchanged |
| `d` | Delete worktree | Unchanged |
| `o` | Open shell | Unchanged |
| `a` | Start agent | Unchanged |

### 4.3 DiffPane (unchanged)

| Key | Action |
|---|---|
| `q` / `esc` | Close panel |
| `s` | Toggle staged / unstaged |
| `v` | Toggle split / unified layout |
| `]` / `[` | Next / previous file |
| `enter` | Open current file in editor |

## 5. Code Changes

### 5.1 `internal/plugins/gitfiletree/overlay.go`

1. Import the git plugin package:
   ```go
   gitplugin "github.com/RollingTheRock/Focus-tui/internal/plugins/git"
   ```
2. In `updateKeyLeftPane()`, change `enter` handling on files to call a new `handleDiff()` method. Directories keep the existing toggle behavior.
3. Add `handleDiff() tea.Cmd`:
   - Resolve the selected file node.
   - Determine whether it is currently staged using the existing `fileGitStatus(node.Path)` helper (status `"staged"` maps to `Staged: true`).
   - Return a command that sends:
     ```go
     gitplugin.OpenDiffMsg{FilePath: path, Staged: staged}
     ```
   - `RepoPath` is left empty so `page.openDiffPane()` uses the overlay's current repo context.
4. Add `e` as an alias for `o` to open the editor.

### 5.2 `internal/plugins/git/worktree_pane.go`

1. In `Update()` key handling, add `case "v"`:
   - Use the selected worktree's path as `RepoPath` so the diff targets that worktree:
     ```go
     gitplugin.OpenDiffMsg{RepoPath: wt.Path, FilePath: "", Staged: false}
     ```
2. In `KeyBindings()`, add `{Keys: []string{"v"}, Help: "diff"}`.

### 5.3 `internal/plugins/git/diff_pane.go`

- Add `RepoPath string` to `OpenDiffMsg`.
- Verify that `loadDiffCmd()` handles `FilePath == ""` by requesting the full worktree diff from `adapter.GetDiff(repoPath, "", staged)`.
- No behavioral change is expected; this is a validation step.

### 5.4 `internal/app/page.go`

- In `openDiffPane()`, use `msg.RepoPath` when it is non-empty; otherwise fall back to `p.gitRepoPath()`:
  ```go
  repoPath := msg.RepoPath
  if repoPath == "" {
      repoPath = p.gitRepoPath()
  }
  ```
- Set `meta.CWD` to the resolved `repoPath`.

## 6. Data Flow

```text
User presses key
    │
    ▼
┌─────────────────────┐     ┌──────────────────────┐
│ GitFileTree overlay │────▶│ OpenDiffMsg          │
│   enter on file     │     │ FilePath, Staged     │
└─────────────────────┘     │ RepoPath: ""         │
                            └──────────────────────┘
                                     │
┌─────────────────────┐              │
│ Worktree pane       │──────────────┘
│   v                 │     OpenDiffMsg{RepoPath: wt.Path, FilePath:""}
└─────────────────────┘
                                     │
                                     ▼
                          ┌─────────────────────┐
                          │ internal/app/app.go │
                          │ OpenDiffMsg handler │
                          └─────────────────────┘
                                     │
                                     ▼
                          ┌─────────────────────┐
                          │ page.openDiffPane() │
                          │ use msg.RepoPath or │
                          │ fall back to page   │
                          │ repo context        │
                          └─────────────────────┘
                                     │
                                     ▼
                          ┌─────────────────────┐
                          │ gitplugin.DiffPane  │
                          └─────────────────────┘
```

## 7. Testing Plan

1. **Unit tests for `overlay.go`:**
   - `enter` on a staged file sends `OpenDiffMsg` with `Staged: true`.
   - `enter` on an unstaged file sends `OpenDiffMsg` with `Staged: false`.
   - `enter` on a directory toggles collapse and does not send `OpenDiffMsg`.
   - `o` and `e` on a file send `OpenEditorMsg`.
   - `d` still triggers discard.

2. **Unit tests for `worktree_pane.go`:**
   - `v` sends `OpenDiffMsg{RepoPath: selectedWorktree.Path, FilePath: "", Staged: false}`.
   - `enter`, `d`, `o`, `a` retain existing behavior.

3. **DiffPane verification:**
   - Empty `FilePath` loads full worktree diff.
   - Non-empty `FilePath` loads single-file diff.

## 8. Edge Cases

- **Untracked files:** `adapter.GetDiff` returns an empty diff for untracked files. `DiffPane` should display a friendly "No diff for untracked file" message instead of a blank viewport.
- **Directories in Git File Tree:** `enter` on a directory must continue to toggle collapse, never open a diff.
- **Clean worktree:** Pressing `v` on a worktree with no changes opens `DiffPane` with an empty diff. This is acceptable; the pane can show "No changes".
- **DiffPane `enter` key:** When inside `DiffPane`, `enter` opens the current file in the editor. This behavior is unchanged.
- **Worktree list from a different repo context:** The `RepoPath` field ensures `v` always diffs the selected worktree, even when the Worktree pane is showing worktrees from a repo other than the page's default repo context.

## 9. Out of Scope

- Reintroducing the embedded diff preview in the Git File Tree right pane.
- Adding the standalone `StatusPane` back to the default layout.
- New diff rendering features (syntax highlighting improvements, hunk staging, etc.).
- Mouse support for opening diffs.

## 10. Success Criteria

- A user can open a single-file diff from the Git File Tree overlay by selecting a changed file and pressing `enter`.
- A user can open a full-worktree review diff from the Worktree pane by selecting a worktree and pressing `v`, and the diff uses that worktree's path rather than the page's default repo context.
- Existing keys (`d` for discard/delete, `enter` for resume, `o` for shell/editor) continue to work as before.
- All new behavior is covered by unit tests.
