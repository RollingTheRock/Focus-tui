# Worktree Container Architecture Design (2026-04)

> Document version: 2026-04-14
> Status: Proposed engineering design
> Scope: Establish worktree as the primary development container for shell, git, editor, and agent workflows

---

## 1. Problem Statement

Agent-assisted development increases the amount of useful parallel work that can happen at once, but the new bottleneck is human context management.

Today, developers often parallelize by:

- opening multiple terminal panes,
- copying repositories into separate directories,
- manually remembering which branch, shell, and agent belong to which task.

This does not scale. The limiting factor becomes the human context window rather than the coding throughput of the agent.

Focus TUI should solve that problem by treating each active development line as a first-class container.

---

## 2. Core Design Decision

**A worktree is not just a Git checkout path. It is the primary container for a development task.**

In this model:

- one active branch usually maps to one active worktree,
- one active worktree usually maps to one task line,
- shell/editor/review/agent state is grouped under that worktree,
- users switch between worktrees rather than repeatedly checking out branches in one directory.

This is the default operating model, not an absolute Git rule. Detached HEAD flows, temporary inspection, and short-lived commands remain valid exceptions.

---

## 3. Product Goals

### Goals

1. Make parallel development lines visible and resumable.
2. Prevent cross-task contamination between shells, editors, diffs, and agents.
3. Make worktree the stable identity used by Git panes, shell panes, and future agent panes.
4. Reduce the amount of branch/task/session state that users must keep in their head.
5. Create a clean foundation for Phase 4 agent-session visibility.

### Non-goals

1. Replacing all Git CLI functionality.
2. Building a full tmux-style process manager.
3. Perfectly restoring every subprocess after restart in the first milestone.
4. Supporting every exotic Git worktree edge case before the main task-oriented flow is solid.

---

## 4. Design Principles

1. **Worktree-first, not branch-first**
   - branch is an attribute;
   - worktree is the navigation and container unit.

2. **Container identity everywhere**
   - every pane that touches repository state must be traceable to exactly one worktree.

3. **Safe defaults over flexible ambiguity**
   - creating a new task should default to a new worktree;
   - dangerous actions must always reveal the targeted worktree path and branch.

4. **Human-readable task identity**
   - branch names are useful, but task names are better for resuming context.

5. **Progressive rollout**
   - first establish worktree identity and visibility;
   - then deepen Git workflow;
   - then attach agent sessions.

---

## 5. Conceptual Model

```text
App
└── RepoRegistry
    └── WorktreeContainer
        ├── Worktree metadata
        ├── Worktree-local layout state
        ├── Shell panes
        ├── Editor panes
        ├── Git status / review panes
        └── Agent sessions
```

The key shift is that panes are no longer the only top-level working object. They become views that belong to a worktree container.

---

## 6. Domain Model

### 6.1 RepoRegistry

Tracks all worktrees belonging to one repository root.

Suggested fields:

- `ID`
- `RepoRoot`
- `MainWorktreeID`
- `WorktreeIDs[]`
- `LastScannedAt`

### 6.2 WorktreeRecord

Persistent metadata describing one worktree container.

Suggested fields:

- `ID`
- `RepoID`
- `Path`
- `Branch`
- `HeadOID`
- `IsMain`
- `IsDetached`
- `IsLocked`
- `IsPrunable`
- `TaskName`
- `CreatedAt`
- `LastActiveAt`
- `LastOpenedAt`
- `Status`
- `DirtySummary`
- `AheadBehind`

Suggested status enum:

- `active`
- `idle`
- `dirty`
- `blocked`
- `missing`
- `prunable`

### 6.3 WorktreeRuntimeState

Ephemeral state tied to the running TUI session.

Suggested fields:

- `FocusedPaneID`
- `BodyTree`
- `ShellPaneIDs[]`
- `EditorPaneIDs[]`
- `GitPaneIDs[]`
- `AgentSessionIDs[]`
- `RecentCommands[]`
- `OpenFiles[]`
- `Notices[]`

### 6.4 WorktreeScoped Identity Rules

Every runtime object that touches repository state must carry `WorktreeID`.

Examples:

- shell pane meta: `WorktreeID`, `RepoID`, `BranchSnapshot`
- editor pane identity: `WorktreeID + absoluteFilePath`
- review pane identity: `WorktreeID + diff mode`
- agent session identity: `provider + sessionID + WorktreeID`

---

## 7. Isolation Strategy

The system must protect against five contamination classes.

### 7.1 Git contamination

Risks:

- working on the wrong branch/worktree,
- deleting a dirty worktree,
- running commit/push against the wrong task line.

Mitigations:

- maintain a branch-to-active-worktree registry,
- scope all Git operations by explicit `worktreePath`,
- show worktree path and branch in dangerous actions,
- require confirmation for remove/prune when dirty or locked.

### 7.2 Shell contamination

Risks:

- dev servers and test watchers from different tasks becoming indistinguishable,
- users losing track of which shell belongs to which task line.

Mitigations:

- each shell pane belongs to one `WorktreeID`,
- opening a shell from a worktree uses that worktree path by default,
- if a shell leaves its assigned subtree, mark it as detached.

### 7.3 Editor/review contamination

Risks:

- reusing an editor for the same relative path across different worktrees,
- opening review in one branch and editor in another.

Mitigations:

- pane reuse keys include `WorktreeID`,
- diff/review panes are always worktree-scoped,
- pane title includes task/branch/worktree identity.

### 7.4 Agent contamination

Risks:

- multiple agents editing the same directory without clear ownership,
- sessions becoming impossible to map back to the task they belong to.

Mitigations:

- sessions attach to `WorktreeID` rather than loose cwd only,
- starting an agent should be a worktree action,
- session lists are grouped by worktree container.

### 7.5 Human context contamination

Risks:

- users cannot remember which task is where,
- returning to yesterday's work requires reconstructing context manually.

Mitigations:

- worktree dashboard with task name, branch, dirty state, shell count, and agent count,
- explicit task naming separate from branch,
- recent activity timeline,
- resume-first flows when reopening Focus TUI.

---

## 8. Adapter-Layer Architecture

### 8.1 Git worktree capabilities

Extend the adapter layer with worktree-native operations.

Suggested interface:

```go
type WorktreeAdapter interface {
    ListWorktrees(repoPath string) ([]git.Worktree, error)
    CreateWorktree(repoPath string, req git.CreateWorktreeRequest) (*git.Worktree, error)
    RemoveWorktree(repoPath string, worktreePath string, opts git.RemoveWorktreeOptions) error
    PruneWorktrees(repoPath string) error
    RepairWorktrees(repoPath string) error
    GetWorktreeStatus(worktreePath string) (*git.Status, error)
}
```

Recommended parsing contracts:

- `git worktree list --porcelain`
- `git status --porcelain=v2 --branch -z`
- `git rev-parse --show-toplevel`
- `git branch --show-current`

### 8.2 File system monitoring

Need a file-system adapter or watcher path to detect:

- worktree deletion outside Focus TUI,
- mtime drift,
- external file changes relevant to open editor panes,
- stale/prunable worktree records.

### 8.3 Agent adapter preparation

Future session adapters should classify sessions by:

1. exact worktree path match,
2. descendant path match,
3. branch snapshot fallback,
4. unresolved session bucket.

---

## 9. UI Architecture

### 9.1 Worktree List Pane

New pane type: `worktree-list`

Purpose:

- list active worktrees,
- show task identity and health,
- serve as the main navigation surface for parallel development.

Suggested row fields:

- task name
- branch
- path
- dirty / ahead / behind
- shell count
- agent count
- last active time

Suggested actions:

- `enter`: focus/open worktree workspace
- `n`: create worktree
- `s`: open shell in worktree
- `g`: open git status for worktree
- `a`: attach/start agent
- `r`: rename task
- `x`: remove worktree

### 9.2 Worktree-local workspace

Each worktree should own its own layout state.

That means:

- pane layout is resumable per worktree,
- editors belong to one worktree,
- review and git panes reflect one worktree only,
- shell spawning is local to the active worktree.

### 9.3 Header visibility

Header should expose the active container at all times:

- repo
- task name
- branch
- dirty count
- active agent count

---

## 10. Lifecycle Flows

### 10.1 Create worktree

1. user chooses base repo and branch strategy,
2. system creates branch if needed,
3. system creates worktree path,
4. system registers `WorktreeRecord`,
5. system opens worktree workspace with default shell + git status.

### 10.2 Resume worktree

1. user selects worktree from dashboard,
2. system restores layout state,
3. system refreshes git status,
4. system reconnects session/agent metadata,
5. system focuses last useful pane.

### 10.3 Remove worktree

1. system checks dirty/locked/main state,
2. system requires confirmation when destructive,
3. system closes associated panes,
4. system removes worktree via adapter,
5. system prunes registry/runtime state.

### 10.4 Detached shell or external drift

If Focus TUI detects a shell or filesystem state that no longer matches its registered worktree container, mark the worktree or pane as degraded instead of silently guessing.

---

## 11. Persistence Model

Recommended split:

- persistent registry for repo/worktree/task metadata,
- runtime session state for live pane and process data.

Possible persistent fields:

- repo root,
- worktree path,
- branch,
- task name,
- last active time,
- last opened files,
- last layout snapshot.

Do not make initial milestones depend on restoring shell processes. Persist identity and layout first; process restoration can be deferred.

---

## 12. Phased Delivery Strategy

### Phase 3A: Worktree container foundation

- worktree adapter methods,
- worktree domain model,
- worktree list pane,
- shell binding to worktree,
- worktree-scoped pane identity.

### Phase 3B: Worktree-local Git workflow deepening

- branch-aware actions,
- hunk-level stage/discard,
- review pane refresh strategy,
- safer remove/prune flows.

### Phase 3C: Context restoration

- per-worktree layout snapshots,
- recent activity and task metadata,
- resume worktree flow.

### Phase 4: Agent visibility on top of worktree identity

- worktree-bound agent sessions,
- activity summaries,
- session history pane,
- branch/cwd/worktree mapping.

---

## 13. Testing Strategy

### Unit tests

- porcelain parser for `git worktree list --porcelain`,
- worktree registry state transitions,
- branch-to-worktree uniqueness logic,
- editor reuse keyed by `WorktreeID`.

### App-level behavior tests

- opening shell from worktree uses correct cwd,
- switching worktrees restores correct layout,
- removing worktree closes dependent panes,
- worktree header state updates correctly.

### Manual regression

- create/remove/prune worktrees,
- dirty worktree guard rails,
- shell detach detection,
- diff/editor/review isolation across same file path in different worktrees.

---

## 14. Immediate Engineering Implications

Before significant UI expansion, the following contracts should be finalized first:

1. `WorktreeRecord` data model
2. worktree lifecycle state machine
3. worktree-scoped pane identity rules
4. adapter boundaries for git/worktree/filesystem/session discovery

These contracts are the foundation for every later worktree, shell, git, and agent feature.
