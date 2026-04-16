# MEMO: Real Product Patterns for Restoring Developer Work Context

**Date:** 2026-04-17  
**Scope:** IDE, terminal multiplexer, git/worktree, session-manager, and developer-environment products that credibly save/restore human developer context across sessions.  
**Goal:** Provide concrete precedents with source names, URLs, and a granular inventory of *what is restored* vs. *what is not* so Focus-tui can compare its direction against proven primitives.

---

## 1. IDEs

### 1.1 Visual Studio Code
*Sources:*
- [What is a VS Code workspace?](https://code.visualstudio.com/docs/editor/workspaces)
- [Allow to open a workspace without restoring any state · Issue #22613](https://github.com/microsoft/vscode/issues/22613)
- [VS Code API docs](https://code.visualstudio.com/api/references/vscode-api)

**Restoration primitives**
- Open editors (tabs), editor group layout, and active file.
- UI state associated with the workspace (e.g., Explorer tree expansion, side-bar visibility).
- Breakpoints per workspace (stored in `workspaceStorage` SQLite DB).
- Task definitions and debugger launch configurations scoped to `.vscode/` or `.code-workspace`.
- Extension enable/disable state per workspace.
- Terminal reconnection in Remote Development (reconnects to remote terminals, not local).
- Auxiliary windows (floating windows) restore since v1.86: editors, size, location.

**What is NOT restored**
- Running local terminal processes or their runtime state (scrollback, partial input, env vars).
- Arbitrary extension runtime state unless the extension explicitly persists it via the Memento API.
- Exact scroll position within a file in all cases (some extensions handle this, core does not guarantee it).
- There is no full “reset workspace” CLI; `--no-state` was requested but closed as out-of-scope.

### 1.2 JetBrains IDEs (IntelliJ IDEA, CLion, WebStorm, Rider)
*Sources:*
- [Manage tasks and contexts](https://www.jetbrains.com/help/idea/managing-tasks-and-context.html)
- [Tasks and contexts | CLion](https://www.jetbrains.com/help/clion/managing-tasks-and-context.html)

**Restoration primitives**
- **Tasks & Contexts:** A *context* is explicitly defined as “a set of bookmarks, breakpoints, and tabs opened in the editor.”
- Contexts can be saved/loaded manually (`Tools | Tasks & Contexts | Save/Load Context`).
- **Save context on commit:** Creates a closed local task that keeps files, bookmarks, and breakpoints.
- Switching tasks can optionally create/switch changelists and shelve current changes.
- Time tracking per task.

**What is NOT restored**
- Running processes or terminal sessions.
- Tool-window layouts outside the editor tab set (e.g., run-config windows, database tool windows).
- Arbitrary plugin UI state unless the plugin implements its own persistence.

---

## 2. Terminal / Session Multiplexers

### 2.1 tmux — `tmux-resurrect` + `tmux-continuum`
*Sources:*
- [tmux-plugins/tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect)
- [restoring_programs.md](https://github.com/tmux-plugins/tmux-resurrect/blob/master/docs/restoring_programs.md)
- [tmux-plugins/tmux-continuum](https://github.com/tmux-plugins/tmux-continuum)

**Restoration primitives**
- All sessions, windows, panes, and their order.
- Current working directory (CWD) for each pane.
- Exact pane layouts within windows (even zoomed state).
- Active and alternative session.
- Grouped sessions (multi-monitor setups).
- Programs running within panes (default whitelist: `vi vim nvim emacs man less more tail top htop irssi weechat mutt`).
- Optional: vim/neovim sessions (via `vim-obsession` / `auto-session`), pane scrollback contents, bash history.
- Continuous auto-save every N minutes (default 15) via `tmux-continuum`.

**What is NOT restored**
- Process *state* (e.g., a running `rails server` loses in-memory state; only the command string is restarted).
- Environment variables inside panes.
- Arbitrary programs unless explicitly added to `@resurrect-processes`.
- SSH connection state or remote shells.

### 2.2 Zellij
*Sources:*
- [Session Resurrection - Zellij User Guide](https://zellij.dev/documentation/session-resurrection.html)
- [Session Management with Zellij](https://zellij.dev/tutorials/session-management/)
- [zellij-org/zellij PR #2801](https://github.com/zellij-org/zellij/pull/2801)

**Restoration primitives**
- Built-in serialization (no plugins required). Session layout (tabs and panes) is written to cache every 1 s as a human-readable KDL layout.
- Command running in each pane is recorded and re-run on resurrection with a `Press ENTER to run...` safety banner (`start_suspended`).
- Optional: pane viewport (on-screen text) and scrollback lines serialization.
- Exited sessions appear as `EXITED` in `zellij ls` and can be resurrected via `zellij attach` or the built-in `session-manager`.
- `dump-layout` and `save-session` CLI actions for manual snapshots.

**What is NOT restored**
- Actual running process state (commands are replayed, not resumed).
- Shell environment variables or history.
- Plugin runtime state (only layout is serialized).

### 2.3 GNU Screen
*Sources:*
- [GNU Screen Manual — Layout](https://www.gnu.org/software/screen/manual/html_node/Layout.html)
- [skoneka/screen-session](https://github.com/skoneka/screen-session)
- [Session Save and Restore with Bash and GNU Screen](https://blog.jasonantman.com/2014/07/session-save-and-restore-with-bash-and-gnu-screen/)

**Restoration primitives (native)**
- `layout save [title]` remembers region arrangements across detach/reattach *within the same server process*.
- `layout dump [filename]` writes split order to a file; can be sourced via `.screenrc` to recreate splits.

**What is NOT restored (native)**
- No built-in persistence across reboots or server death.
- Window-to-region mapping, region sizes, and running commands are not saved by `layout dump`.

**Third-party augmentation: `screen-session`**
- Saves layouts, scrollbacks, titles, filters; can restart programs via a primer executable.
- Saves Vim sessions via `:mksession`.
- Still does not restore process state or environment variables.

### 2.4 abduco + dvtm
*Sources:*
- [martanne/abduco](https://github.com/martanne/abduco)
- [Abduco + DVTM a Lightweight Alternative to Tmux and Screen](https://www.brain-dump.org/blog/abduco-dvtm-a-lightweight-alternative-to-tmux-and-screen/)
- [abduco & dvtm talk slides (PDF)](https://www.brain-dump.org/talks/abduco-dvtm-cosin18.pdf)

**Restoration primitives**
- abduco provides session *attach/detach* over a Unix domain socket (like `dtach`).
- dvtm provides tiling window management; abduco keeps the session alive when the terminal closes.

**What is NOT restored**
- No serialization across reboots or server crash.
- No save/restore of pane layouts, CWDs, running commands, or terminal contents after the server dies.
- Explicit design goal: “no session support (see abduco)” in dvtm; separation of concerns means no cross-session context restoration.

---

## 3. Git / Worktree-Oriented Tools

### 3.1 lazyworktree
*Source:* [chmouel/lazyworktree](https://github.com/chmouel/lazyworktree)

**Restoration primitives**
- Keyboard-first TUI for creating, switching, and pruning git worktrees.
- Can open a worktree in a new tmux window/pane or Zellij tab (ties filesystem isolation to multiplexer context).
- Per-worktree notes, taskboards, CI/PR status, and hooks (`.wt` files).
- Shell helper `cd "$(lazyworktree)"` to jump back to a selected worktree.

**What is NOT restored**
- Does not persist or restore editor state, open files, or terminal scrollback.
- Multiplexer integration is launch-time only; runtime layout changes are not saved.

### 3.2 twig
*Source:* [andersonkrs/twig](https://github.com/andersonkrs/twig)

**Restoration primitives**
- Ties each git worktree to a named tmux session (`project__branch`).
- YAML project configs define windows/panes (e.g., `editor: nvim`, `git: lazygit`).
- Can copy/symlink files (`.env`, credentials) into the worktree on creation.

**What is NOT restored**
- No dynamic snapshot of a running session; if you rearrange panes or open new files, those changes are lost on next `twig start` unless the YAML is edited.
- No process state or terminal history restoration.

### 3.3 mxt
*Source:* [gkarolyi/mxt](https://github.com/gkarolyi/mxt)

**Restoration primitives**
- CLI to spin up an isolated git worktree + tmux session.
- Configurable `copy_files` (e.g., `.env`, `CLAUDE.md`) into the new worktree.
- Pre-session command (e.g., `bundle install`).

**What is NOT restored**
- No session save/restore; purely a bootstrap tool.

### 3.4 grove
*Source:* [thisguymartin/grove](https://github.com/thisguymartin/grove)

**Restoration primitives**
- One-command launcher that creates a Zellij session with one color-coded tab per git worktree.
- Each tab pre-loads LazyGit and an AI agent shell.
- Auto-kills stale sessions on re-launch.

**What is NOT restored**
- No persistence of open files, editor state, or process context across launches.

### 3.5 lazygit
*Source:* [jesseduffield/lazygit](https://github.com/jesseduffield/lazygit)

**Restoration primitives**
- Built-in worktree creation/switching from the branches view (`w`).

**What is NOT restored**
- No session or editor state management; it is purely a Git TUI.

---

## 4. Developer Environment Products

### 4.1 GitHub Codespaces
*Sources:*
- [Rebuilding the container in a codespace](https://docs.github.com/en/codespaces/developing-in-a-codespace/rebuilding-the-container-in-a-codespace)
- [Understanding the codespace lifecycle](https://docs.github.com/en/codespaces/developing-in-codespaces/codespaces-lifecycle)

**Restoration primitives**
- `/workspaces` directory is persistent across stop/start *and* rebuild.
- Uncommitted changes are preserved when stopping.
- Terminal history is preserved (but visible viewport text is not).
- Dotfiles repo can restore shell configuration on creation.

**What is NOT restored**
- Changes outside `/workspaces` (including home directory) are cleared on rebuild.
- Running processes are killed on stop.
- `/tmp` is cleared on stop.
- No snapshot of IDE window layout or open files beyond VS Code’s own workspace state.

### 4.2 Gitpod
*Sources:*
- [Workspace Lifecycle - Gitpod Classic](https://www.gitpod.io/docs/configure/workspaces/workspace-lifecycle)
- [Prebuilds - Gitpod Classic](https://www.gitpod.io/docs/configure/repositories/prebuilds)
- [Gitpod Workspace CLI](https://www.gitpod.io/docs/configure/workspaces/gitpod-cli)

**Restoration primitives**
- Only `/workspace` persists across stop/start.
- **Snapshots** (`gp snapshot`): create a complete clone of a workspace (filesystem in `/workspace` + task definitions) that can be shared via URL.
- **Prebuilds**: execute `init`/`before` tasks ahead of time and cache the `/workspace` filesystem snapshot.
- Pinned workspaces are never auto-deleted.

**What is NOT restored**
- Home directory (`/home/gitpod`) is not persisted across stops or snapshots.
- Running processes are terminated on stop.
- Terminal layout or editor open-file state is not part of the snapshot (depends on the IDE inside the workspace).

### 4.3 Coder (code-server)
*Sources:*
- [Write a Template from Scratch | Coder Docs](https://coder.com/docs/tutorials/template-from-scratch)
- [Workspace Scheduling | Coder Docs](https://www.coder.com/docs/admin/templates/managing-templates/schedule)
- [Coder API: Workspaces Management](https://coder.com/docs/reference/api/workspaces)

**Restoration primitives**
- Templates are Terraform configurations; admins can attach **persistent volumes** (e.g., Docker volumes, Kubernetes PVCs) that survive start/stop cycles.
- Common pattern: mount a persistent volume to `$HOME` so dotfiles, shell history, and installed tools survive.
- `startup_script` runs on every start, allowing re-installation or re-configuration.
- Workspace schedules support auto-stop, dormancy, and autostart.

**What is NOT restored**
- Ephemeral container/Pod resources are destroyed on stop; any state not on a persistent volume is lost.
- No built-in mechanism to restore running processes, terminal sessions, or IDE layout.
- Users must explicitly template persistence; default behavior is ephemeral.

### 4.4 VS Code Dev Containers
*Source:* [How to Backup DevContainer State](https://github.com/devcontainers/images/issues/1225)

**Restoration primitives**
- Workspace mount (`/workspaces/<repo>`) persists across container rebuilds.
- `devcontainer.json`, Dockerfile, and dotfiles repo provide declarative restoration of tools and config.

**What is NOT restored**
- Container layers outside the workspace mount are rebuilt from scratch.
- No automatic save/restore of running services, terminal sessions, or editor state beyond VS Code’s normal workspace storage.

---

## 5. Editor & Shell Context Tools

### 5.1 Neovim Session Managers
#### auto-session (rmagatti)
*Source:* [rmagatti/auto-session](https://github.com/rmagatti/auto-session)

**Restoration primitives**
- Auto-save/restore per CWD using `:mksession`.
- Restores buffers, windows, tabs, folds, terminal buffers, `localoptions`, and cursor positions.
- Supports git-branch-scoped session names.
- CWD-change handling: save old session, clear buffers/jumps, restore new session.

**What is NOT restored**
- LSP/DAP runtime state, running debug sessions, or external processes.
- Arbitrary plugin UI state unless captured by `sessionoptions`.

#### possession.nvim (jedrzejboczar)
*Source:* [jedrzejboczar/possession.nvim](https://github.com/jedrzejboczar/possession.nvim)

**Restoration primitives**
- JSON-based session files using `:mksession` under the hood.
- Saves/restore nvim-dap breakpoints and DAP UI layouts.
- Telescope integration for picking sessions.

**What is NOT restored**
- Active debug sessions or process state.

### 5.2 Vim `:mksession`
*Source:* Vim help (`:help 'sessionoptions'`)

**Restoration primitives**
- Buffers, windows, tabs, folds, current directory, win sizes/positions, and (optionally) terminal buffers.

**What is NOT restored**
- Running external processes, plugin runtime state, or LSP servers.

### 5.3 Emacs `desktop-save-mode`
*Sources:*
- [Saving Emacs Sessions (GNU Emacs Manual)](https://www.gnu.org/software/emacs/manual/html_node/emacs/Saving-Emacs-Sessions.html)
- [Desktop Save Mode (GNU Emacs Lisp Reference Manual)](https://www.gnu.org/software/emacs/manual/html_node/elisp/Desktop-Save-Mode.html)

**Restoration primitives**
- Buffers, file names, major modes, buffer positions, window and frame configurations.
- Optional frame sizes/locations (`desktop-restore-frames`).
- Lazy restoration of buffers (`desktop-restore-eager`).
- Minibuffer history via `savehist` (separate library).

**What is NOT restored**
- Running processes or arbitrary elisp runtime state.
- Window registers saved via `window-configuration-to-register` are reported to corrupt across sessions in some cases.

### 5.4 direnv
*Source:* [direnv.net](https://direnv.net/)

**Restoration primitives**
- Automatically loads/unloads environment variables from `.envrc` per directory.
- PATH modifications, secret injection, and per-project isolated environments.

**What is NOT restored**
- Any session layout, open files, running processes, or terminal state.
- Purely environment-variable scoping.

### 5.5 autoenv
*Source:* [hyperupcall/autoenv](https://github.com/kennethreitz/autoenv)

**Restoration primitives**
- Executes `.env` (and optionally `.env.leave`) scripts on `cd` enter/leave.

**What is NOT restored**
- No persistence of session, layout, or process context beyond the side effects of the sourced scripts.

---

## 6. Cross-Category Summary: Restoration Primitives Matrix

| Primitive | VS Code | JetBrains | tmux-resurrect | Zellij | Screen (native) | lazyworktree | Gitpod | Coder | Neovim auto-session | Emacs desktop-save |
|---|---|---|---|---|---|---|---|---|---|---|
| **Open files / buffers** | Yes | Yes (tabs) | N/A | N/A | N/A | No | No* | No* | Yes | Yes |
| **Window / pane layout** | Yes (editor groups) | Partial (tabs only) | Yes | Yes | Partial (regions only) | No | No* | No* | Yes (splits) | Yes (frames) |
| **CWD per pane/editor** | Yes | Yes | Yes | Yes | No | Yes (worktree) | Yes (`/workspace`) | Configurable | Yes | No |
| **Breakpoints / bookmarks** | Yes | Yes | N/A | N/A | N/A | No | No* | No* | Partial (DAP via plugins) | No |
| **Running commands** | No | No | Yes (whitelist) | Yes (replay) | No | No | No | No | No | No |
| **Process *state*** | No | No | No | No | No | No | No | No | No | No |
| **Scrollback / viewport** | No | No | Optional | Optional | No | No | No | No | No | No |
| **Environment variables** | No | No | No | No | No | No | No | Only via dotfiles/.envrc | No | No |
| **Terminal sessions** | Remote only | No | Yes | Yes | Yes (live only) | No | No | No | Terminal buffers only | No |
| **Cross-reboot durability** | Yes | Yes | Yes | Yes | No (native) | No | Yes (`/workspace` only) | Yes (persistent vols) | Yes | Yes |

`*` These products do not natively restore IDE/editor state; they rely on the IDE inside the workspace to do so.

---

## 7. Implications for Focus-tui

1. **The “gold standard” primitives that are consistently restored across successful tools are:**
   - *Filesystem location* (CWD / worktree / workspace mount).
   - *Open documents / buffers*.
   - *Window / pane layout*.
   - *Breakpoints or bookmarks* (in IDEs).
   - *Command replay* (not process state resurrection) in terminal multiplexers.

2. **No mainstream tool attempts to restore true process state** (e.g., a running REPL’s in-memory variables, a partially edited `git rebase`, or a live SSH connection). The closest approximation is *command-string replay* with a safety prompt (Zellij) or a conservative whitelist (tmux-resurrect).

3. **Environment variable restoration is handled orthogonally** by tools like `direnv` and `autoenv`. Session managers generally do not capture shell env diffs.

4. **Worktree-based tools** (lazyworktree, twig, grove) solve *context isolation* by using git worktrees + named tmux/Zellij sessions, but they do not solve *state save/restore*. They rely on the underlying multiplexer or IDE for that.

5. **Cloud dev environments** (Codespaces, Gitpod, Coder) persist *files* and *declarative config*, but treat running processes and UI layout as ephemeral. This suggests that for Focus-tui, prioritizing **fast re-creation of a known layout** may be more practical than true hibernation of process state.

---

*End of memo*
