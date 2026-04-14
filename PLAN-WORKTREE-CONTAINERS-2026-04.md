# Worktree Container Implementation Plan (2026-04)

> Document version: 2026-04-14
> Expected duration: 4-6 weeks
> Goal: Make worktree the primary development container for parallel shell, git, editor, and agent workflows
> Design reference: [DESIGN-WORKTREE-CONTAINERS-2026-04.md](./DESIGN-WORKTREE-CONTAINERS-2026-04.md)

---

## 1. Goal

Shift Focus TUI from a pane-first Git workspace into a worktree-container workspace where each active task line has:

- a stable worktree identity,
- isolated shell/editor/review state,
- resumable layout and task context,
- a future-ready attachment point for agent sessions.

---

## 2. Success Criteria

- [ ] Focus TUI can discover and render all worktrees for the active repo
- [ ] A new worktree can be created from the UI with safe defaults
- [ ] Each shell/editor/review/git pane is scoped to exactly one worktree
- [ ] Same-path files from different worktrees never reuse the same editor pane
- [ ] Removing a worktree closes dependent panes and respects dirty safeguards
- [ ] Per-worktree workspace state can be resumed
- [ ] Agent/session integration has a clear worktree identity hook
- [ ] `go test ./...` remains green throughout rollout

---

## 3. Scope Boundaries

### In scope

- worktree registry and domain model
- Git adapter worktree methods
- worktree list pane
- worktree-scoped pane identity
- worktree-local shell and git flows
- resume metadata and layout snapshots

### Out of scope for this plan

- full agent orchestration UI
- restoring live subprocesses after restart
- advanced merge/rebase/conflict UI
- vendor-specific agent optimizations

---

## 4. Delivery Phases

## Phase A - Domain and Adapter Foundation

### A1. Introduce worktree domain types

**Files**:

- `internal/git/worktree.go` (new)
- `internal/git/types.go`

**Tasks**:

- add `Worktree`, `DirtySummary`, `AheadBehind`, create/remove option structs
- define worktree status enums (`active`, `idle`, `dirty`, `blocked`, `missing`, `prunable`)
- separate persistent metadata from runtime-only metadata where needed

**Acceptance**:

- model supports main worktree, linked worktree, detached HEAD, locked, and prunable states

### A2. Extend adapter layer

**Files**:

- `internal/adapters/git.go`
- `internal/adapters/git_local.go`
- `internal/adapters/git_local_test.go`

**Tasks**:

- add `ListWorktrees`, `CreateWorktree`, `RemoveWorktree`, `PruneWorktrees`, `GetWorktreeStatus`
- parse `git worktree list --porcelain`
- reuse `git status --porcelain=v2 --branch -z` for per-worktree state
- define safety errors for dirty main/locked/missing worktrees

**Acceptance**:

- parser handles bare minimum Git formats without human-readable assumptions
- tests cover porcelain parsing and safety guards

### A3. Add worktree registry service

**Files**:

- `internal/worktree/registry.go` (new)
- `internal/worktree/registry_test.go` (new)
- `internal/adapters/manager.go`

**Tasks**:

- track repo root → worktree records
- enforce default branch-to-active-worktree uniqueness
- expose refresh/discover methods for app layer

**Acceptance**:

- app can ask for current repo worktrees from one place
- duplicate active branch mapping is detected and surfaced

---

## Phase B - Worktree UI and App Integration

### B1. Add pane type and plugin surface

**Files**:

- `internal/models/pane.go`
- `internal/plugins/git/plugin.go`
- `internal/plugins/git/worktree_pane.go` (new)
- `internal/plugins/git/worktree_pane_test.go` (new)

**Tasks**:

- add `PaneTypeWorktreeList`
- register worktree pane in git plugin
- render task name, branch, dirty state, path, activity counts

**Acceptance**:

- worktree pane can load, refresh, and render predictable rows

### B2. Make pane identity worktree-aware

**Files**:

- `internal/models/pane.go`
- `internal/app/app.go`
- `internal/plugins/editor/editor_pane.go`
- `internal/plugins/editor/editor_pane_test.go`

**Tasks**:

- extend pane meta with `RepoID` / `WorktreeID` / optional `BranchSnapshot`
- update editor reuse key to include `WorktreeID`
- ensure review/diff/editor panes inherit worktree identity from opener

**Acceptance**:

- opening `same/path/file.go` from two worktrees creates two independent editor panes

### B3. Add worktree-local shell spawn rules

**Files**:

- `internal/app/app.go`
- `internal/ui/shell/*`
- `internal/app/app_test.go`

**Tasks**:

- opening a shell from a worktree uses that worktree path
- detect shell cwd drift and mark shell/worktree as detached or degraded
- route new panes to the correct worktree-local layout context

**Acceptance**:

- shells created from different worktrees never share implicit cwd state

---

## Phase C - Worktree Lifecycle Operations

### C1. Create worktree flow

**Files**:

- `internal/plugins/git/worktree_create_modal.go` (new or equivalent)
- `internal/app/app.go`
- `internal/plugins/git/messages.go`

**Tasks**:

- create worktree from branch/task name
- default path naming scheme based on repo + task slug
- open new workspace after creation

**Acceptance**:

- user can create a worktree without touching CLI

### C2. Remove/prune flow

**Files**:

- `internal/plugins/git/worktree_pane.go`
- `internal/app/app.go`
- `internal/plugins/git/messages.go`

**Tasks**:

- add remove confirmation with dirty/main/locked guards
- close all dependent panes before final removal
- support prune for stale metadata entries

**Acceptance**:

- destructive flow is safe and explicit

---

## Phase D - Context Restoration

### D1. Persist worktree metadata

**Files**:

- `internal/store/*` or equivalent persistence layer
- `internal/worktree/registry.go`
- `internal/config/*` if snapshot config is needed

**Tasks**:

- persist task name, last active time, layout snapshot, recent files
- load registry on startup

**Acceptance**:

- closed and reopened app can restore useful worktree context without guessing

### D2. Resume-first UX

**Files**:

- `internal/plugins/git/worktree_pane.go`
- `internal/app/app.go`
- header/footer surfaces as needed

**Tasks**:

- show most recent worktrees first
- show recent activity summary
- support "resume this worktree" as a first-class action

**Acceptance**:

- users can re-enter yesterday's task with minimal context rebuilding

---

## Phase E - Agent Attachment Foundation

### E1. Define worktree-bound session model

**Files**:

- `internal/agents/types.go` (new)
- `internal/adapters/agent.go` (new)
- `internal/worktree/registry.go`

**Tasks**:

- define `AgentSession` with `WorktreeID`, `cwd`, `branchSnapshot`, provider metadata
- classify sessions by worktree path first

**Acceptance**:

- session data can attach to worktree even before a full agent-history pane exists

---

## 5. Sequence Recommendation

Recommended build order:

1. Phase A (types + adapter + registry)
2. Phase B (worktree pane + pane identity + shell rules)
3. Phase C (create/remove/prune)
4. Phase D (resume and persistence)
5. Phase E (agent attachment foundation)

This order is important. Phase E depends on stable worktree identity from A-D.

---

## 6. Risks and Controls

### Risk: scope explosion into a full IDE rewrite

**Control**: keep first milestones limited to identity, isolation, and resume.

### Risk: pane/app architecture is currently pane-first

**Control**: introduce worktree identity incrementally through metadata and registry before attempting large tree-layout rewrites.

### Risk: editor and shell reuse leaks across worktrees

**Control**: make `WorktreeID` mandatory in reuse keys before adding more worktree UI.

### Risk: docs and implementation drift again

**Control**: treat this plan as the active Phase 3 reference and keep `NEXT_STEPS.md` pointed here.

---

## 7. Immediate Next Tasks

1. Finalize worktree domain model and registry interfaces.
2. Extend `GitAdapter` with worktree methods.
3. Add `PaneTypeWorktreeList` and skeleton pane.
4. Update pane metadata with worktree identity.
5. Add tests for same-path editor isolation across worktrees.

---

## 8. Done Definition for First Milestone

The first milestone is complete when:

- Focus TUI can discover all repo worktrees,
- the worktree list is visible in the UI,
- users can open a shell and git status in a selected worktree,
- pane identity no longer leaks across worktrees,
- tests cover the new worktree-aware identity rules.
