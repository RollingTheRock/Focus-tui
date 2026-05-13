# Focus TUI Development Archive & Phase 4 Proposal

> Generated: 2026-04-16
> Session: Phase 3 stabilization + Phase 4 Agent Orchestration kickoff
> Status: Canvas compositor has feature flag; Agent Session Pane implemented; awaiting Phase 4 direction decision

---

## Part 1: Historical Development Timeline

### Phase 1: Embedded Shell (Complete)
- PTY-backed shell execution
- Alternate screen handling
- Terminal capability forwarding
- Mouse and scrollback behavior
- Resize synchronization

### Phase 2: Multi-Pane Layout System (Complete)
- Horizontal/vertical splits
- Pane focus routing
- Pane metadata (name, type, cwd, status)
- Multi-instance shell panes
- Todo and Pomodoro migrated to pane system

### Phase 3: Native Git and Worktree Workflow (Mostly Complete)

**Completed:**
- Repository status summary
- Worktree list and state display
- Worktree create / switch / remove
- Worktree as primary task container
- Overview page + full-screen worktree workspace pages
- Diff views and staging workflow
- Commit flow inside workspace
- Layout snapshot persistence (save/restore per worktree)
- Editor pane integration

**Known Issues Fixed:**
- `gitRepoPath()` on worktree pages returned worktree path instead of repo root — **fixed**
- `bodyTree` fallback missing when `CreatePane(PaneTypeGitStatus)` failed — **fixed**

**Performance Optimizations (Complete & Pushed):**
1. Shell tick reduced: `33ms → 100ms`
2. `invalidateView()` now conditional on actual pane state changes
3. `syncWorktreeActivities()` moved out of `buildView()` into state handlers
4. Lipgloss style cache: `internal/styles/cache.go`
5. Agent discovery throttled to max 1/sec
6. Parallelized `ListWorktrees` with `sync.WaitGroup`
7. GitLocalAdapter TTL cache (1s) for `GetStatus` and `ListWorktrees`
8. `fsnotify` watcher replacing 2s ticker for git status polling

---

## Part 2: Canvas Abstraction Layer

### What Exists
- `internal/render/canvas.go` — Cell-grid Canvas with ANSI-aware `SetString`
- `internal/render/adapter.go` — `StringAdapter`, `RenderPane`, `PanelRenderer`
- `Renderer` interface for panes to render directly to Canvas
- `WorktreePane` and `AgentSessionPane` both implement `Renderer`

### Current Status
- **Production path**: `page.renderBody()` uses stable string-based compositor (`blankCanvas` + `layout.RenderPanel` + `layout.OverlayOnBase`)
- **Experimental path**: `page.renderBodyCanvas()` accessible via config flag
- **Feature flag**: `config.Config.Experimental.UseCanvasCompositor bool`

### Why the Split?
Canvas compositor was enabled but had rendering issues (pane misalignment, border errors, overflow). After two fix rounds (ANSI byte-width handling, border padding), it was still unstable in real terminal use. Rather than deleting the abstraction, we:
1. Kept all Canvas code and `Renderer` implementations
2. Reverted the production compositor to string-based
3. Added a feature flag so Canvas can be debugged and hardened incrementally

### Next Steps for Canvas
- Fix `RenderPane` dimension handling when passed full frame sizes vs content sizes
- Add visual regression tests (render known layouts to strings and assert alignment)
- Validate all pane types under Canvas before flipping the default

---

## Part 3: Phase 4 Agent Orchestration — Three Direction Proposal

### Current State (MVP Exists)
- `internal/agents/types.go` — `Session`, `Provider`, `SessionState`
- `internal/agents/discovery.go` — `DiscoverRunningAgents()` via `pgrep` + `/proc/{pid}/cwd`
- `internal/agents/launcher.go` — `LaunchCommand`, `AutoTypeCommand`, `LaunchAgentMsg`, `AgentExitedMsg`
- `internal/agents/registry.go` — in-memory `Registry`
- `internal/app/app.go` — `launchAgent()` handler, throttled discovery sync
- `internal/plugins/git/worktree_pane.go` — `a` key launches default agent; displays agent tags in worktree list
- `internal/plugins/agents/session_pane.go` — **NEW** dedicated Agent Session Pane

### What Was Just Implemented
**Agent Session Pane (`PaneTypeAgentSession`):**
- Lists all running agents from `agentRegistry`
- Shows provider, worktree (shortened), PID, runtime
- Actions:
  - `enter` → focus agent's worktree/shell
  - `x` → kill agent process
  - `a` → launch new agent in selected worktree
  - `r` → refresh
- Added to both overview page and worktree page (right side, split vertically with shell)

---

## Three Proposed Directions for Phase 4

### Direction A: Agent Session Management & History
**Goal:** Make agents fully observable and actionable from within Focus.

**Scope:**
1. **Session History Pane** — show not just running agents, but recently exited sessions with exit code, duration, last command/context
2. **Persistent Registry** — store agent sessions in SQLite (start time, end time, provider, worktree, branch, exit code, session log path if available)
3. **Agent-to-Branch Mapping** — discovery currently maps by CWD only; enhance to capture current git branch at session start
4. **Session Replay/Log View** — if agent tools write to known log paths (e.g. `~/.local/share/opencode/sessions/`), surface recent output in a read-only pane

**Effort:** Medium (2–3 weeks)
**Value:** High immediate utility; transforms agents from "tags in a list" to first-class workspace citizens

---

### Direction B: Multi-Agent Orchestration & Workflows
**Goal:** Enable running and coordinating multiple agents simultaneously within a single worktree or across worktrees.

**Scope:**
1. **Remove Single-Agent-per-Worktree Limit** — `HasRunning(worktreeID, provider)` currently blocks launching a second agent of the same provider; relax this and track multiple PIDs per (worktree, provider)
2. **Agent Queue / Scheduler** — simple queue for agent tasks: "run opencode in worktree A, then claude in worktree B"
3. **Agent Workspace Presets** — save and restore sets of agents for a given worktree (e.g. "opencode + claude" for backend work)
4. **Cross-Agent Messaging** — lightweight protocol for agents to signal completion or hand off to another agent (could start with file-system signals or simple socket)

**Effort:** High (3–5 weeks)
**Value:** High strategic value; this is the "multi-agent operating system" vision
**Risk:** Requires understanding how each agent tool handles concurrent instances in the same directory

---

### Direction C: Agent-Native Pane Integration
**Goal:** Deepen the integration so that agent output becomes a native pane type, not just a shell running an agent.

**Scope:**
1. **Agent Output Pane (`PaneTypeAgentOutput`)** — a pane that tails an agent's session log or captures its stdout in a structured way (separating user input from agent response)
2. **Agent Command Injection** — from Focus, send pre-canned commands to an agent's stdin (e.g. "/commit", "/test", "/explain")
3. **Agent State Icons in Pane Borders** — show spinner when agent is "thinking", checkmark when idle, warning on error
4. **Agent-Aware Shell** — the embedded shell detects when an agent is running and adjusts its title/status accordingly

**Effort:** High (4–6 weeks)
**Value:** Very high; this is the most "native" integration and hardest to replicate in a generic terminal
**Risk:** Each agent tool has different I/O behavior; may require per-provider adapters

---

## Recommendation

**Short term (next 1–2 sprints):** Pursue **Direction A** (Session Management & History). It builds on the Agent Session Pane we just created, has clear deliverables, and provides immediate user value without requiring deep per-agent reverse engineering.

**Medium term:** Use Direction A's persistent registry as the foundation for **Direction B** (you can't orchestrate what you can't track).

**Long term:** Direction C is the endgame, but should be attempted only after A and B are solid and after Canvas compositor is production-ready (since rich agent output panes will benefit heavily from cell-level rendering).

---

## Files of Interest for Next Session

### Core Agent Architecture
- `/home/rollingtherock/dev/Focus-tui/internal/agents/types.go`
- `/home/rollingtherock/dev/Focus-tui/internal/agents/discovery.go`
- `/home/rollingtherock/dev/Focus-tui/internal/agents/launcher.go`
- `/home/rollingtherock/dev/Focus-tui/internal/agents/registry.go`

### Agent UI / Pane
- `/home/rollingtherock/dev/Focus-tui/internal/plugins/agents/session_pane.go`
- `/home/rollingtherock/dev/Focus-tui/internal/plugins/agents/plugin.go`

### App Integration
- `/home/rollingtherock/dev/Focus-tui/internal/app/app.go` (lines ~230–680, ~1270–1330)
- `/home/rollingtherock/dev/Focus-tui/internal/app/page.go` (lines ~100–180, ~620–720)

### Config / Feature Flags
- `/home/rollingtherock/dev/Focus-tui/internal/config/config.go`

### Canvas (for future virtualization)
- `/home/rollingtherock/dev/Focus-tui/internal/render/canvas.go`
- `/home/rollingtherock/dev/Focus-tui/internal/render/adapter.go`

---

## Test Status
- `go test ./...` — **PASSING** (all packages)
- Agent pane tests: `internal/plugins/agents/session_pane_test.go`, `plugin_test.go`
