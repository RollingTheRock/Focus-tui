# MEMO: Focus-tui Context/Plan/Archival Model — Current State & Gaps

**Date:** 2026-04-17  
**Scope:** SQLite schema, in-memory runtime model, and planning docs as they exist in the repo today. No speculation beyond file contents.

---

## 1. What the System Currently Stores (Persistent)

All persistent state lives in a single SQLite DB (`~/.local/share/focus/focus.db`). Schema is defined in:

- **`internal/store/db.go`** — `migrate()` creates the following tables.

### 1.1 Personal Workflow (legacy, still active)
| Table | What it stores | File |
|---|---|---|
| `todos` | Text items with status (`todo`/`done`/`overdue`) and list (`today`/`someday`) | `internal/store/db.go:64-71` |
| `pomodoro_sessions` | Start/end times, linked todo ID, completion status | `internal/store/db.go:73-80` |
| `streaks` | Daily boolean for habit tracking | `internal/store/db.go:82-85` |

### 1.2 Phase 4 Context Schema (new, partially wired)
| Table | What it stores | Key Fields |
|---|---|---|
| `task_contexts` | Lightweight intent object per task | `id, repo_id, title, goal, next_step, state, priority, parent_task_id, preferred_worktree_id` |
| `worktree_contexts` | Per-worktree metadata | `worktree_id, repo_id, primary_task_id, task_mode, task_name, branch_snapshot, last_active_at, last_opened_at, last_agent_at` |
| `task_worktree_links` | Many-to-many with relation type | `task_id, worktree_id, relation_type` (`primary`/`secondary`/`queued`/`historical`) |
| `context_notes` | Free-form notes pinned to task or worktree | `task_id, worktree_id, note_type` (`goal`/`next_step`/`blocker`/`insight`/`handoff`), `body`, `pinned` |
| `agent_sessions` | Agent lifecycle summary | `id, provider, worktree_id, repo_id, branch_snapshot, pid, state, launch_source, summary, started_at, ended_at, last_activity_at` |
| `page_snapshots` | Layout blob per worktree | `worktree_id, snapshot_json` (body tree, focused pane, open editors, zoom state) |

Store implementations: `internal/store/task_context.go`, `worktree_context.go`, `task_worktree_link.go`, `context_note.go`, `agent_session.go`, `snapshot.go`.

Models: `internal/models/ui.go:47-113`.

---

## 2. What the System Treats as Ephemeral (In-Memory Only)

These are computed or held at runtime and lost on process exit:

| Entity | Where it lives | Why it disappears |
|---|---|---|
| **Running agent registry** | `internal/agents/registry.go` — `map[string]*Session` | Cleared on restart; reconstructed from `pgrep` + `/proc/{pid}/cwd` discovery (`discovery.go`) |
| **Resume summary cache** | `internal/app/app.go:100` — `resumeSummaryCache map[string]WorktreeResumeSummary` | Built on every `syncWorktreeActivities()` call by fusing DB records + live pane state + git status |
| **Pane runtime state** | `internal/app/page.go:29-48` — `panes`, `paneMeta`, `bodyTree`, `focused`, `zoomedPane`, `preZoomTree` | Only `page_snapshots` is persisted; everything else is reconstructed from snapshot or defaults |
| **Shell PTY state** | `internal/ui/shell/shell.go` — scrollback, env vars, partial input | Not captured at all |
| **Editor dirty state / cursor** | `internal/plugins/editor/` — transient buffer state | Not persisted beyond the file path list in `page_snapshots.OpenEditors` |
| **Agent stdout/stderr / conversation** | None — agents write to their own directories (`~/.claude`, `~/.opencode`) | Focus-tui stores only a one-line `summary` in `agent_sessions` |
| **Git status** | Polled live via `fsnotify` + adapters | No cache across restarts |

---

## 3. Where Structured Context/Plan Archival Is Absent

### 3.1 Session Plans — MISSING ENTIRELY
- **Gap:** There is no table or model for a *session plan* (a structured artifact that an agent or human produces before executing).
- **Evidence:** `task_contexts.goal` and `.next_step` are free-text strings. `context_notes` stores flat text. There is no `plans` table, no plan snapshot, no plan versioning.
- **File paths:** `internal/models/ui.go:68-80` (TaskContextRecord has no plan fields); `internal/store/db.go:93-105` (no plan table).

### 3.2 Plan Snapshots / Versioning — MISSING ENTIRELY
- **Gap:** No record of how a task's plan or goal evolved over time.
- **Evidence:** `task_contexts` is upsert-only. `context_notes` is upsert-only. No `plan_versions`, `goal_history`, or `note_history` tables.

### 3.3 Context Bundles — MISSING ENTIRELY
- **Gap:** No way to capture a "bundle" of related state (task + notes + open editors + agent session + git branch) as a reusable or exportable unit.
- **Evidence:** The `resumeSummaryCache` (`internal/app/app.go:1572-1667`) computes a *transient* fusion of these sources, but nothing persists the bundle. There is no `context_bundles` or `session_exports` table.

### 3.4 Archival Boundaries — UNDER-MODELED
- **Gap:** `task_contexts.state` includes `archived`, but there is no `archived_at` timestamp, no archival reason, no retention policy, and no compaction logic.
- **Gap:** `agent_sessions` stores `ended_at`, but there is no "archive old sessions" boundary. Exited sessions remain in the same table forever.
- **Gap:** `page_snapshots` accumulates one row per worktree forever. There is no cleanup when a worktree is pruned or when a snapshot becomes stale.
- **File paths:** `internal/store/db.go:93-179` (no archival metadata on any table).

### 3.5 Stale Context Handling — MISSING ENTIRELY
- **Gap:** No TTL, staleness detection, or auto-expiry for:
  - `context_notes` (a note from 6 months ago is treated the same as today's)
  - `worktree_contexts` (deleted worktrees leave orphaned rows unless manually cleaned)
  - `page_snapshots` (old layout blobs may reference pane types that no longer exist)
- **Evidence:** No `stale_at`, `expires_at`, or `deleted_at` columns. No background GC.

### 3.6 Cross-Session Reuse — UNDER-MODELED
- **Gap:** Tasks are scoped to `repo_id`, but there is no mechanism to:
  - Clone or template a task context into a new worktree/session
  - Inherit a plan from a parent task beyond the single `parent_task_id` FK
  - Share notes across tasks or repos
- **Gap:** `task_worktree_links.relation_type` has `historical`, but there is no query surface that uses it for "show me what I did in this worktree last month."
- **File paths:** `internal/store/task_worktree_link.go:52-57` (historical link type exists but is unused in UI); `internal/app/workbench_context.go` (builds ephemeral view, no historical queries).

### 3.7 Agent Session Context — SHALLOW
- **Gap:** `agent_sessions.summary` is a single text field. There is no structured storage of:
  - Tool calls made
  - Files touched
  - Conversation turns
  - Context window usage
  - Exit code or failure reason
- **Gap:** No link between an agent session and the *plan* it was executing.
- **Evidence:** `internal/store/db.go:165-179` (agent_sessions schema); `internal/agents/types.go:31-45` (Session struct has no plan/context fields).

---

## 4. Summary Matrix

| Concern | Stored? | Where | Ephemeral? | Gaps |
|---|---|---|---|---|
| Personal todos | Yes | `todos` table | No | No plan linkage |
| Pomodoro sessions | Yes | `pomodoro_sessions` | No | No task context linkage |
| Worktree layout | Yes | `page_snapshots` | No | No versioning, no stale cleanup |
| Task intent | Yes | `task_contexts` | No | No plan artifact, no version history |
| Worktree metadata | Yes | `worktree_contexts` | No | No stale detection, no archive boundary |
| Task-worktree relation | Yes | `task_worktree_links` | No | `historical` type unused |
| Notes | Yes | `context_notes` | No | No versioning, no TTL |
| Agent lifecycle | Yes | `agent_sessions` | Partial (running state in `agents.Registry`) | No transcripts, no tool logs, no plan link |
| Agent runtime state | No | — | Yes (reconstructed via `pgrep`) | No persistent registry of running processes |
| Resume summary | No | — | Yes (`resumeSummaryCache`) | Not stored; recomputed every second |
| Shell state | No | — | Yes | Not captured |
| Editor state | Partial | `page_snapshots.OpenEditors` | Yes (cursor, dirty, scroll) | Only file paths stored |
| Session plans | No | — | — | **Missing entity** |
| Plan snapshots | No | — | — | **Missing entity** |
| Context bundles | No | — | — | **Missing entity** |
| Archival policy | No | — | — | **Missing system** |
| Stale context GC | No | — | — | **Missing system** |

---

## 5. Files to Reference in Discussion

| File | Relevance |
|---|---|
| `internal/store/db.go` | SQLite schema — ground truth for what is persisted |
| `internal/models/ui.go` | Go structs for all persisted records |
| `internal/app/app.go` | Runtime model: `resumeSummaryCache`, `agentRegistry`, `pages`, `syncWorktreeActivities()` |
| `internal/app/page.go` | Page runtime state: `bodyTree`, `focused`, `snapshot`, `captureSnapshot()`, `restoreSnapshot()` |
| `internal/agents/types.go` | Agent session model (shallow) |
| `internal/agents/registry.go` | In-memory only; no persistence |
| `internal/agents/discovery.go` | Runtime process discovery via `pgrep` |
| `internal/store/snapshot.go` | Layout persistence (blob only) |
| `internal/app/workbench_context.go` | Ephemeral fusion of task + worktree + git + agent state |
| `RESEARCH_MEMO_context_restoration.md` | External research on what other tools restore vs. not |
| `IMPLEMENTATION_PLAN.md` | Phase 4 roadmap — confirms intent to avoid "event log, transcript store, process restore" |

---

## 6. Key Design Tension

The `IMPLEMENTATION_PLAN.md` (Section V, Constraints) explicitly states:

> - 只持久化 **resume summary / lifecycle summary**，不持久化事件流  
> - `page_snapshots` 保持为 layout blob，不增加 shell transcript / pane telemetry  
> - task 只做轻量 intent object，不进入复杂项目管理语义

This means the system currently **chooses** to keep agent transcripts, shell state, and event streams ephemeral. The engineering question is not "why wasn't this stored?" but rather:

> **If the user wants engineered context/plan archival, which of the current constraints should be relaxed, and what new entities (`plans`, `plan_snapshots`, `context_bundles`, `archival_rules`) would need to be added without violating the "no event sink" principle?**

---

*End of memo.*
