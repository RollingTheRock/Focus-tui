# MEMO: Focus-tui Plan & Handoff Layer — Architecture Risk Review

**Date:** 2026-04-17  
**Scope:** Concrete first-implementation guidance for a durable Plan & Handoff layer, grounded in the existing SQLite store, task/worktree/session structures, and `workbench_context.go` fusion pipeline.  
**Constraint:** Must respect the Phase 4 "no event sink, no transcript store, no project manager bloat" boundary (`IMPLEMENTATION_PLAN.md` Section V).

---

## 1. Executive Summary

The repo already has 80% of the substrate needed for a minimal Plan & Handoff layer. The risk is not missing tables — it is **over-persisting** state that should stay derived, and **under-defining** archival boundaries that will turn the SQLite file into an unbounded junk drawer.

**Verdict:** The first implementation needs **one new table** (`task_plans`) and **three columns** added to existing tables. Everything else should be derived from the existing schema at runtime. Handoff is a `note_type` + a query pattern, not a new entity.

---

## 2. Current Schema Boundaries (Ground Truth)

Source of truth: `internal/store/db.go` migrate block (`db.go:62-187`).

### 2.1 What Already Exists

| Table | Role | Relevance to Plan/Handoff |
|---|---|---|
| `task_contexts` | Intent object | `goal`, `next_step` are free-text plan fragments. No versioning. |
| `worktree_contexts` | Container metadata | `primary_task_id`, `task_mode`, `last_active_at` anchor tasks to workspaces. |
| `task_worktree_links` | Many-to-many binding | `relation_type` (`primary`/`secondary`/`queued`/`historical`) already models task lineage. |
| `context_notes` | Free-form annotations | `note_type` includes `handoff`. Pinned flag exists. |
| `agent_sessions` | Agent lifecycle summary | One-line `summary`. No plan link. No transcript. |
| `page_snapshots` | Layout blob | `snapshot_json` stores pane tree, open editors, zoom state per worktree. |

### 2.2 What Is Explicitly Absent (and must stay absent in v1)

- No `plans` table → plan fragments live in `task_contexts.goal/next_step` and `context_notes`.
- No `plan_versions` table → versioning is deferred; use `context_notes` append-only pattern if history is needed.
- No `context_bundles` table → bundles are runtime fusions in `workbench_context.go`.
- No `archived_at` / `stale_at` columns → archival is a state transition + timestamp, not a new table.

---

## 3. Lifecycle States & State Machine

### 3.1 Task State Machine (`task_contexts.state`)

Current states (`db.go:99`): `active`, `paused`, `blocked`, `done`, `archived`.

This is sufficient for v1. **Do not add states.** The semantics are:

```
active   → user is working on this now (appears in resume-first overview)
paused   → intentionally parked (still appears, lower score)
blocked  → has a blocker note (appears with alert indicator)
done     → completed this session (moves to "recently done" bucket for 7d, then invisible)
archived → historical record (excluded from overview unless explicitly queried)
```

**Critical rule:** `archived` is the **only** terminal state that persists long-term. `done` is a soft terminal — it auto-transitions to `archived` after a TTL (see §5.3). This prevents the overview from accumulating finished tasks forever.

### 3.2 Agent Session State Machine (`agent_sessions.state`)

Current states (`agents/types.go:21-27`): `running`, `exited`, `failed`, `waiting`, `unknown`.

Sufficient for v1. The only needed addition is a **plan link** (see §8).

### 3.3 Worktree Lifecycle (Derived, Not Persisted)

Worktree state (`internal/git/worktree.go:66`) is discovered at runtime via `git worktree list --porcelain` and `git status`. **Do not persist** worktree presence/absence. The `worktree_contexts` table holds *metadata* for worktrees that have been opened in Focus-tui; if git reports the worktree is gone, the row becomes orphaned and is subject to GC (§5.3).

---

## 4. Stale / Archive / Orphan Handling

This is the highest-risk area. Without explicit boundaries, `context_notes` and `agent_sessions` grow without limit.

### 4.1 Orphan Detection (Derived at Runtime)

| Orphan Type | Detection Query | Action |
|---|---|---|
| Worktree gone | `worktree_contexts.worktree_id` not in `git worktree list` output | Mark `last_seen_missing_at` (new column), do not auto-delete |
| Task with no links | `task_contexts` row with zero `task_worktree_links` after 30d | Auto-transition state → `archived` |
| Note on archived task | `context_notes.task_id` points to `task_contexts.state = 'archived'` | Keep; notes are cheap. Query with `pinned DESC` to surface important ones |
| Agent session on pruned worktree | `agent_sessions.worktree_id` not in `worktree_contexts` | Keep; agent history is intentionally append-only |

### 4.2 Archival Boundary (New Column)

Add `archived_at DATETIME` to `task_contexts`. When state transitions to `archived`, set this timestamp. This is the **single source of truth** for "is this task historical?"

Rationale: The existing `state` column is mutable. A timestamp makes archival queries O(index) instead of O(table scan).

### 4.3 Stale Context TTL (New Column)

Add `stale_at DATETIME` to `context_notes`. Default NULL. When a note is created, optionally set `stale_at` (e.g., blocker note expires in 7 days). The UI filters `stale_at IS NULL OR stale_at > now()`.

Rationale: `context_notes` is append-only. Without TTL, old handoff notes from 6 months ago pollute the overview. The `pinned` flag overrides staleness — a pinned handoff note is always visible.

### 4.4 Agent Session Retention (Derived)

`agent_sessions` is already append-only. Do not add TTL. The table is expected to grow linearly with session count (low volume). If it becomes a problem later, partition by `started_at` month.

---

## 5. Minimal First Implementation — MUST INCLUDE

### 5.1 Schema Changes (Exact)

**`task_contexts` table:**
- Add `archived_at DATETIME` (nullable, default NULL)
- Add `done_at DATETIME` (nullable, default NULL)

**`context_notes` table:**
- Add `stale_at DATETIME` (nullable, default NULL)

**New table: `task_plans`**
```sql
CREATE TABLE IF NOT EXISTS task_plans (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    plan_body   TEXT NOT NULL,        -- markdown or plain text plan
    plan_state  TEXT NOT NULL CHECK(plan_state IN ('draft', 'approved', 'active', 'completed', 'discarded')) DEFAULT 'draft',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_task_plans_task ON task_plans(task_id, updated_at DESC);
```

Rationale: `task_contexts.goal` and `next_step` are too small for a structured plan. A separate `task_plans` table gives the user a place to write a multi-step plan without bloating the intent object. One plan per task is sufficient for v1.

**`agent_sessions` table:**
- Add `plan_id TEXT REFERENCES task_plans(id) ON DELETE SET NULL` (nullable)

This is the only link between agent execution and planning. No other agent schema changes.

### 5.2 Store Interface Additions

```go
// internal/models/ui.go — add to Store interface
SaveTaskPlan(record TaskPlanRecord) error
GetTaskPlan(id string) (*TaskPlanRecord, error)
ListTaskPlans(taskID string) ([]TaskPlanRecord, error)  // v1: returns 0 or 1 row

// Soft archival
ArchiveTaskContext(id string) error  // sets state='archived', archived_at=now
ListArchivedTaskContexts(repoID string, since time.Time) ([]TaskContextRecord, error)

// Note staleness
ListActiveContextNotes(taskID string, worktreeID string) ([]ContextNoteRecord, error)  // filters stale_at
```

### 5.3 Workbench Context Integration

Modify `internal/app/workbench_context.go` to:

1. **Read plan state:** If `task_plans.plan_state = 'active'` for the primary task, add `"plan: active"` to `HandoffSummary` or a new `PlanSummary` field.
2. **Filter stale notes:** Use `ListActiveContextNotes` instead of `ListContextNotes` in the resume pipeline.
3. **Surface archival:** Do not render `archived` tasks in the default queue. Add a separate "Recent Archive" section if needed.

### 5.4 Handoff Pattern (Derived, Not a New Entity)

Handoff is **not** a table. It is a query pattern over existing tables:

```sql
-- "What do I need to know to resume this worktree?"
SELECT
    tc.title, tc.goal, tc.next_step, tc.state,
    tp.plan_body, tp.plan_state,
    cn.body AS handoff_note
FROM worktree_contexts wc
LEFT JOIN task_contexts tc ON tc.id = wc.primary_task_id
LEFT JOIN task_plans tp ON tp.task_id = tc.id AND tp.plan_state IN ('active', 'approved')
LEFT JOIN context_notes cn ON cn.worktree_id = wc.worktree_id AND cn.note_type = 'handoff' AND (cn.stale_at IS NULL OR cn.stale_at > datetime('now'))
WHERE wc.worktree_id = ?
ORDER BY cn.pinned DESC, cn.updated_at DESC
LIMIT 1;
```

This query is run by `buildWorkbenchQueueItem` and cached in `resumeSummaryCache`. It is **purely derived** — no handoff table, no handoff state machine.

---

## 6. Minimal First Implementation — MUST NOT INCLUDE

| Temptation | Why Not | What To Do Instead |
|---|---|---|
| `plan_versions` table | Event log in disguise. Violates Phase 4 constraint. | If history is needed, append `context_notes` with `note_type = 'insight'` and timestamp. |
| `context_bundles` table | Bundles are runtime fusions. Persisting them duplicates source of truth. | `buildWorkbenchOverviewContext` already fuses task + worktree + notes + agent + git. Cache in `resumeSummaryCache`. |
| `handoffs` table with lifecycle | Over-engineering. Handoff is a note with query semantics. | Use `context_notes(note_type='handoff', pinned, stale_at)`. |
| Auto-archival background job | Adds process complexity. SQLite has no cron. | Run GC on app startup and on worktree pane refresh. Low frequency is fine. |
| Plan approval workflow with state machine hooks | Project manager bloat. | `task_plans.plan_state` is a text field. UI changes the string. No hooks, no side effects. |
| Transcript / tool-call / file-touch storage | Explicitly out of scope per `IMPLEMENTATION_PLAN.md` Section V. | Agent sessions stay shallow. One-line summary only. |
| Cross-repo task cloning / templates | Under-modeled today; parent_task_id FK exists but no inheritance logic. | Defer. `parent_task_id` is sufficient for visual grouping. |
| Staleness for worktree_contexts rows | Worktree presence is derived from git. Don't add `stale_at` to metadata. | Orphan detection via runtime git query (§4.1). |

---

## 7. Derived vs Persisted — Explicit Map

| Concept | Persisted? | Where | Rationale |
|---|---|---|---|
| Task intent (title, goal, next_step) | **Yes** | `task_contexts` | Human-written, stable across restarts. |
| Task state (active/paused/blocked/done/archived) | **Yes** | `task_contexts.state` + `archived_at` / `done_at` | Must survive restart. |
| Task priority | **Yes** | `task_contexts.priority` | Human-set, stable. |
| Plan artifact | **Yes** | `task_plans.plan_body` | Human-written, multi-line, too large for `task_contexts`. |
| Plan state (draft/approved/active/completed/discarded) | **Yes** | `task_plans.plan_state` | Simple enum, UI-mutable. |
| Handoff note | **Yes** | `context_notes` with `note_type='handoff'` | Human-written text. |
| Pinned / stale flags on notes | **Yes** | `context_notes.pinned`, `context_notes.stale_at` | Control visibility. |
| Worktree metadata (task_mode, task_name) | **Yes** | `worktree_contexts` | Human-assigned labels for containers. |
| Worktree presence (is the path still valid?) | **Derived** | `git worktree list` at runtime | Git is the source of truth. |
| Worktree dirty state | **Derived** | `git status --porcelain` at runtime | Changes every keystroke. |
| Resume score / resume reason | **Derived** | `resumeSummaryCache` in `app.go` | Fused from DB + git + agent registry + pane state. Recomputed every sync. |
| Agent running state | **Derived** | `agents.Registry` + `pgrep` discovery | PID validity is runtime-only. |
| Agent summary | **Yes** | `agent_sessions.summary` | One-line human-readable status, written at exit. |
| Agent session → plan link | **Yes** | `agent_sessions.plan_id` | Static FK, low cardinality. |
| Shell scrollback / env / cwd | **No** | — | Explicitly out of scope (Phase 4 constraints). |
| Pane layout | **Yes** (blob) | `page_snapshots.snapshot_json` | Already implemented. Keep as opaque JSON. |
| Open editor file list | **Yes** (in blob) | `page_snapshots.OpenEditors` | Part of snapshot blob. |
| Editor cursor / dirty state | **No** | — | Too volatile; reconstruct on open. |
| Git ahead/behind | **Derived** | `git status --branch` at runtime | Changes on every fetch. |
| Queued task summary | **Derived** | `task_worktree_links` WHERE `relation_type='queued'` + `task_contexts` | Join at query time. |

---

## 8. Risk Mitigation Checklist

Before writing any migration:

- [ ] `task_plans` has only one plan per task in v1. No versioning. No `plan_steps` sub-table.
- [ ] `agent_sessions.plan_id` is nullable. Existing sessions stay valid.
- [ ] `context_notes.stale_at` is nullable. Existing notes are treated as never-stale.
- [ ] `task_contexts.archived_at` and `done_at` are nullable. Existing rows are valid.
- [ ] All new queries have `LIMIT` clauses to prevent unbounded result sets.
- [ ] No background goroutine for archival GC. Run it synchronously during worktree pane refresh or app init.
- [ ] `page_snapshots` remains an opaque blob. No new fields added to the JSON schema for planning.

---

## 9. Recommended Build Order

1. **Migration:** Add `archived_at`, `done_at` to `task_contexts`; add `stale_at` to `context_notes`; create `task_plans`; add `plan_id` to `agent_sessions`.
2. **Models:** Add `TaskPlanRecord` to `internal/models/ui.go`; extend `Store` interface.
3. **Store:** Implement `SaveTaskPlan`, `GetTaskPlan`, `ListTaskPlans`, `ArchiveTaskContext`, `ListArchivedTaskContexts`, `ListActiveContextNotes`.
4. **App layer:** Wire plan read into `buildWorkbenchQueueItem`. Filter stale notes. Hide archived tasks from default overview.
5. **UI layer:** Add plan create/edit/approve UI in task modal. Add handoff note pin/stale UI in note list.
6. **GC:** Add startup-time orphan detection for missing worktrees and unlinked tasks.
7. **Tests:** Every new store method needs a test. `go test ./...` must stay green.

---

*End of memo.*
