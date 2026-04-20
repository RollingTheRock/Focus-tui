# MEMO: Plan & Handoff Layer — Safe Seams & Minimal-Diff Path

> [过时归档说明]
> 本 memo 记录的是 ADR-first 转型前的 seam 分析。
> 自 2026-04-19 起保留作历史参考，不再作为当前执行依据。

**Date:** 2026-04-17  
**Scope:** Schema, store, models, app workbench context builder, overview panes, task/worktree links, notes, agent sessions, tests.  
**Constraint:** No broad redesign. Identify exact extension points and likely conflicts.

---

## 1. Executive Summary

The codebase already has **~80% of the foundational schema and store contracts** for Phase 4 (Human Context Recovery + Lightweight Task Orchestration). The `task_contexts`, `worktree_contexts`, `task_worktree_links`, `context_notes`, `agent_sessions`, and `page_snapshots` tables are live, migrated, and tested. The `archived` task state and `handoff` note type already exist in schema.

**What is genuinely missing** for a full Plan & Handoff layer:
- `task_plans` / `plan_steps` tables and stores
- `session_handoffs` table (structured handoff record, not just a text note)
- Archival/staleness **hooks** (the `archived` state exists but no automation drives it)
- UI rendering paths for plan steps and structured handoffs

**Good news:** Every missing piece can attach to an existing seam without redesign. The worst conflict risk is schema ordering in `db.go` and Store interface bloat in `models/ui.go`.

---

## 2. Existing Foundation (Evidence)

### 2.1 Schema Already Migrated

File: `internal/store/db.go` (lines 62–187)

| Table | Status | Key Fields |
|-------|--------|------------|
| `task_contexts` | **LIVE** | `id, repo_id, title, goal, next_step, state, priority, parent_task_id, preferred_worktree_id` |
| `worktree_contexts` | **LIVE** | `worktree_id, repo_id, primary_task_id, task_mode, task_name, branch_snapshot, last_active_at, last_opened_at, last_agent_at` |
| `task_worktree_links` | **LIVE** | `id, task_id, worktree_id, relation_type` (`primary`/`secondary`/`queued`/`historical`) |
| `context_notes` | **LIVE** | `id, task_id, worktree_id, note_type, body, pinned` — note_type includes **`handoff`** |
| `agent_sessions` | **LIVE** | `id, provider, worktree_id, repo_id, branch_snapshot, pid, state, launch_source, summary, started_at, ended_at, last_activity_at` |
| `page_snapshots` | **LIVE** | `worktree_id, snapshot_json, updated_at` |

**Critical detail:** `task_contexts.state` CHECK includes **`'archived'`** (line 99). The archival concept is already in the database; only the automation hook is missing.

**Critical detail:** `context_notes.note_type` CHECK includes **`'handoff'`** (line 152). Handoff notes already exist as free text; structured handoff records do not.

### 2.2 Store Interface Already Exposed

File: `internal/models/ui.go` (lines 12–45)

The `models.Store` interface already exposes:
- `SaveTaskContext` / `GetTaskContext` / `ListTaskContexts`
- `SaveWorktreeContext` / `GetWorktreeContext` / `ListWorktreeContexts` / `DeleteWorktreeContext`
- `SaveTaskWorktreeLink` / `ListTaskWorktreeLinks` / `ListWorktreeTaskLinks`
- `SaveContextNote` / `ListContextNotes`
- `SaveAgentSession` / `ListAgentSessions`
- `SavePageSnapshot` / `LoadPageSnapshot` / `ListPageSnapshots` / `DeletePageSnapshot`

All record structs (`TaskContextRecord`, `WorktreeContextRecord`, etc.) are defined in the same file (lines 47–113).

### 2.3 Resume Summary Pipeline Already Built

File: `internal/app/app.go` (lines 1505–1570, 1572–1667)

- `syncWorktreeActivities()` — reconciles agent sessions, computes `WorktreeActivity`, feeds `WorktreePane`
- `refreshResumeSummaryCache()` — reads `task_contexts` + `worktree_contexts` + `task_worktree_links` + `context_notes` + `agent_sessions` into `WorktreeResumeSummary` per worktree
- `deriveWorktreeNotes(taskID, worktreeID)` — extracts `pinned`, `blocker`, `handoff` from `context_notes`
- `computeResumeReason()` + `buildResumeHint()` — score-based resume ranking already operational

### 2.4 Overview UI Already Rendering Context

Files:
- `internal/app/workbench_context.go` — `buildWorkbenchOverviewContext()` assembles `workbenchQueueItem` + `workbenchOverviewContext`
- `internal/app/overview_summary_pane.go` — renders stats cards (WORKTREES, ACTIVE, BLOCKED, QUEUED, DIRTY, AGENTS, FOCUS)
- `internal/app/overview_detail_pane.go` — renders `overviewDetailSelection` with `ResumeReason`, `ResumeHint`, `NextStep`, `Goal`, `BlockerNote`, `HandoffNote`
- `internal/app/task_edit_pane.go` — `TaskEditPane` overlay for creating/editing tasks (title, goal, next_step, state, priority)

### 2.5 Tests Already Covering Context Round-Trip

File: `internal/store/context_test.go`
- `TestTaskContextRoundTrip`
- `TestWorktreeContextRoundTrip`
- `TestTaskWorktreeLinksAndNotes`

Pattern: `New(":memory:")` → save → load → assert fields.

---

## 3. Where Plan & Handoff State Should Attach

### 3.1 `task_plans` — Attach to `task_contexts` (1:1 or 1:N)

**Seam:** `task_contexts.id` is already the stable task identifier. A plan is an extension of a task's intent.

**Schema location:** `internal/store/db.go`, after `task_contexts` block, before `worktree_contexts`.

```sql
CREATE TABLE IF NOT EXISTS task_plans (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    state       TEXT NOT NULL DEFAULT 'draft' CHECK(state IN ('draft', 'active', 'paused', 'done', 'abandoned')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_task_plans_task ON task_plans(task_id, updated_at DESC);
```

**Store file:** `internal/store/task_plan.go` (new, following `task_context.go` pattern)

**Model struct:** Add to `internal/models/ui.go` after `TaskContextRecord`:
```go
type TaskPlanRecord struct {
    ID        string
    TaskID    string
    Title     string
    State     string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**Store interface:** Add to `models.Store`:
```go
SaveTaskPlan(record TaskPlanRecord) error
GetTaskPlan(id string) (*TaskPlanRecord, error)
ListTaskPlans(taskID string) ([]TaskPlanRecord, error)
```

**Conflict risk:** LOW. No existing table uses `task_plans` name. FK is safe because `task_contexts` is created first in migration order.

### 3.2 `plan_steps` — Attach to `task_plans` (N:1)

**Seam:** Each plan has ordered steps. Natural child table.

**Schema location:** `internal/store/db.go`, immediately after `task_plans`.

```sql
CREATE TABLE IF NOT EXISTS plan_steps (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT NOT NULL REFERENCES task_plans(id) ON DELETE CASCADE,
    step_number INTEGER NOT NULL,
    title       TEXT NOT NULL,
    state       TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending', 'in_progress', 'done', 'blocked', 'skipped')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plan_id, step_number)
);
CREATE INDEX IF NOT EXISTS idx_plan_steps_plan ON plan_steps(plan_id, step_number);
```

**Store file:** `internal/store/plan_step.go` (new)

**Model struct:** `PlanStepRecord` in `internal/models/ui.go`

**Store interface:** `SavePlanStep`, `ListPlanSteps(planID string)`, `GetPlanStep(id string)`

**Conflict risk:** LOW. No collisions.

### 3.3 `session_handoffs` — Structured handoff between sessions

**Seam:** `agent_sessions` has lifecycle events (`started_at`, `ended_at`, `state`). A handoff is a bridge between an outgoing session and an incoming one, optionally with a task context. This is **different** from `context_notes` (which is free text) because it needs FKs to sessions and a structured `handoff_type`.

**Schema location:** `internal/store/db.go`, after `agent_sessions`.

```sql
CREATE TABLE IF NOT EXISTS session_handoffs (
    id              TEXT PRIMARY KEY,
    from_session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    to_session_id   TEXT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    task_id         TEXT REFERENCES task_contexts(id) ON DELETE SET NULL,
    worktree_id     TEXT NOT NULL,
    handoff_type    TEXT NOT NULL CHECK(handoff_type IN ('human_resume', 'agent_continue', 'blocked', 'completed')),
    summary         TEXT,
    next_step_hint  TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_session_handoffs_worktree ON session_handoffs(worktree_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_handoffs_task ON session_handoffs(task_id, created_at DESC);
```

**Store file:** `internal/store/session_handoff.go` (new)

**Model struct:** `SessionHandoffRecord` in `internal/models/ui.go`

**Store interface:** `SaveSessionHandoff`, `ListSessionHandoffs(worktreeID string)`, `GetSessionHandoff(id string)`

**Conflict risk:** LOW. `context_notes` already has a `handoff` type, but that is a **text note**; this is a **structured record**. They serve different purposes and can coexist. If you want to unify, you could drop `handoff` from `context_notes` CHECK, but that requires a migration and is unnecessary for minimal diff.

### 3.4 Archival/Staleness Hooks — Leverage `syncWorktreeActivities()`

**Seam:** `internal/app/app.go` `syncWorktreeActivities()` (line 1505) is the **only place** where the app regularly reconciles runtime state with persisted state. It already:
- Reconciles discovered agent sessions
- Refreshes resume summary cache
- Sets `WorktreeActivity` per page

**Where to add staleness signals:**

1. **Task-level staleness:** After `refreshResumeSummaryCache()` (line 1519), iterate `taskContexts` and check `updated_at` age. If `state = 'active'` and `updated_at > N days`, optionally emit a staleness signal or auto-transition to `paused`. **Do not auto-archive** without user confirmation (violates Phase 4 constraint).

2. **Worktree-level staleness:** In `touchWorktreeContext()` (line 1964), `worktree_contexts.last_active_at` is already touched on every page switch. A simple comparison in `refreshResumeSummaryCache()` can set a `Stale` flag on `WorktreeResumeSummary`.

3. **Agent session cleanup:** `reconcileDiscoveredAgentSessions()` (line 2040) already marks missing PIDs as `exited`. Extend to write a `session_handoffs` record when a running session disappears and a task was active.

**Conflict risk:** MEDIUM. `syncWorktreeActivities()` is hot path (called on every relevant message in `Update`). Keep staleness checks **cheap** (time.Since comparisons, no DB writes). Any DB write must be guarded by a threshold to avoid hammering SQLite.

---

## 4. UI Extension Points

### 4.1 Workbench Context Builder

File: `internal/app/workbench_context.go`

- `workbenchQueueItem` (line 16) already has `HandoffSummary`. Add `PlanSummary string` and `Stale bool`.
- `buildWorkbenchQueueItem()` (line 103) is where plan step summaries and staleness labels would be assembled.
- `overviewSummaryStats` (line 13) can gain `Planned int` and `Stale int` counters.

### 4.2 Overview Detail Pane

File: `internal/app/overview_detail_pane.go`

- `overviewDetailSelection` (line 12) can gain `PlanTitle`, `PlanState`, `CurrentStep`, `StepCount`, `IsStale`.
- `View()` (line 59) already has a "Signals" section and "Notes" section. Add a "Plan" section between "Now" and "Signals".

### 4.3 Resume Summary Cache

File: `internal/git/worktree.go`

- `WorktreeResumeSummary` (line 30) can gain `PlanTitle`, `PlanState`, `CurrentStepNumber`, `TotalSteps`, `IsStale`.
- This is the **source of truth** for overview rendering; all new UI data should flow through here.

### 4.4 Task Edit Pane

File: `internal/app/task_edit_pane.go`

- `taskEditorSeed` (line 42) and `TaskEditorSavedMsg` (line 25) are the contracts for task creation/editing.
- To attach a plan to task creation, extend `TaskEditorSavedMsg` with `PlanTitle string` and `PlanSteps []string`, then in `saveTaskEditor()` (line 1219) also call `SaveTaskPlan` + `SavePlanStep`.

---

## 5. Minimal-Diff Implementation Path

### Step 1: Schema + Store (no UI changes)

1. `internal/store/db.go` — add `task_plans`, `plan_steps`, `session_handoffs` CREATE TABLE blocks to `migrate()`.
2. `internal/models/ui.go` — add `TaskPlanRecord`, `PlanStepRecord`, `SessionHandoffRecord` structs; add methods to `Store` interface.
3. Create `internal/store/task_plan.go`, `plan_step.go`, `session_handoff.go` following exact patterns from `task_context.go` / `agent_session.go`.
4. `internal/store/context_test.go` — add round-trip tests for each new table.
5. Run `go test ./internal/store/...` → green.

**Estimated diff:** ~400 lines, 4 new files, 1 modified file (`db.go`), 1 modified file (`ui.go`).

### Step 2: Resume Summary Pipeline (no new panes)

1. `internal/git/worktree.go` — extend `WorktreeResumeSummary` with plan/staleness fields.
2. `internal/app/app.go` — in `refreshResumeSummaryCache()`, query new tables and populate new fields.
3. `internal/app/workbench_context.go` — extend `workbenchQueueItem` and `buildWorkbenchQueueItem()` to surface plan/staleness data.
4. Run `go test ./internal/app/...` → green.

**Estimated diff:** ~80 lines, 3 modified files.

### Step 3: Overview Pane Rendering (no new panes)

1. `internal/app/overview_detail_pane.go` — extend `overviewDetailSelection` and `View()` to show plan and staleness state.
2. `internal/app/overview_summary_pane.go` — extend `overviewSummaryStats` and `View()` to show planned/stale counts.
3. Run `go test ./...` → green.

**Estimated diff:** ~60 lines, 2 modified files.

### Step 4: Staleness Hooks (optional, can defer)

1. `internal/app/app.go` — in `syncWorktreeActivities()`, add time-based staleness detection to `refreshResumeSummaryCache()`. Keep it read-only (no auto-state-change) to stay within Phase 4 constraints.
2. If desired, add a manual "Archive" keybinding in `handleKey()` or `WorktreePane` that sets `task_contexts.state = 'archived'`.

**Estimated diff:** ~30 lines, 1 modified file.

### Step 5: Task Editor Plan Attachment (optional, can defer)

1. `internal/app/task_edit_pane.go` — add plan title + steps inputs.
2. `internal/app/app.go` — in `saveTaskEditor()`, save plan + steps alongside task.

**Estimated diff:** ~50 lines, 2 modified files.

---

## 6. Likely Conflicts & Mitigations

| Risk | Location | Mitigation |
|------|----------|------------|
| **Store interface bloat** | `models/ui.go` line 12 | The interface is already large (~30 methods). Adding 6–9 more is acceptable; if it grows past 40, consider splitting into `TaskStore`, `AgentStore`, `PlanStore` interfaces composed into `Store`. |
| **Migration ordering** | `db.go` migrate() | SQLite executes statements sequentially. Place new tables after their FK parents (`task_plans` after `task_contexts`, `plan_steps` after `task_plans`, `session_handoffs` after `agent_sessions`). |
| **Hot-path DB writes** | `app.go` `syncWorktreeActivities()` | Phase 4 constraint: "render path zero DB query." Staleness checks must be **read-only** in the render path. Any write (e.g., auto-archive) must be async or triggered by explicit user action. |
| **Handoff concept duality** | `context_notes.note_type = 'handoff'` vs `session_handoffs` | Keep both. `context_notes.handoff` is a user-written free-text note. `session_handoffs` is a machine-generated structured bridge between sessions. If unifying later, migrate `context_notes` rows into `session_handoffs.summary`. |
| **Task state machine collision** | `task_contexts.state` has `archived` | `nextTaskState()` in `app.go` (line 1309) cycles `active→paused→blocked→done→active`. It does **not** include `archived`. This is correct — archiving should be a deliberate action, not a cycle step. |
| **Test data coupling** | `context_test.go` uses `:memory:` | New tests should follow same pattern. No conflict. |

---

## 7. Concrete File Inventory for First Pass

| File | Action | Lines (est.) |
|------|--------|--------------|
| `internal/store/db.go` | Append 3 CREATE TABLE + INDEX blocks to `migrate()` | +50 |
| `internal/models/ui.go` | Append 3 record structs; append 6–9 methods to `Store` interface | +60 |
| `internal/store/task_plan.go` | **New** — `SaveTaskPlan`, `GetTaskPlan`, `ListTaskPlans` + scanners | +90 |
| `internal/store/plan_step.go` | **New** — `SavePlanStep`, `GetPlanStep`, `ListPlanSteps` + scanners | +90 |
| `internal/store/session_handoff.go` | **New** — `SaveSessionHandoff`, `GetSessionHandoff`, `ListSessionHandoffs` + scanners | +90 |
| `internal/store/context_test.go` | Append round-trip tests for new tables | +80 |
| `internal/git/worktree.go` | Extend `WorktreeResumeSummary` | +15 |
| `internal/app/app.go` | Extend `refreshResumeSummaryCache()` to query new tables | +40 |
| `internal/app/workbench_context.go` | Extend `workbenchQueueItem` + `buildWorkbenchQueueItem()` | +20 |
| `internal/app/overview_detail_pane.go` | Extend `overviewDetailSelection` + `View()` | +25 |
| `internal/app/overview_summary_pane.go` | Extend `overviewSummaryStats` + `View()` | +15 |

**Total first-pass diff: ~575 lines across 11 files, 3 new files.**

---

## 8. Extension Points Summary

| New Concept | Attaches To | Schema File | Store File | Model File | UI Consumer |
|-------------|-------------|-------------|------------|------------|-------------|
| `task_plans` | `task_contexts.id` | `db.go` | `task_plan.go` | `ui.go` | `WorktreeResumeSummary`, `overviewDetailSelection` |
| `plan_steps` | `task_plans.id` | `db.go` | `plan_step.go` | `ui.go` | `workbenchQueueItem`, `overviewDetailSelection` |
| `session_handoffs` | `agent_sessions.id` + `task_contexts.id` | `db.go` | `session_handoff.go` | `ui.go` | `WorktreeResumeSummary`, `overviewDetailSelection` |
| Staleness signal | `worktree_contexts.last_active_at` / `task_contexts.updated_at` | *(existing)* | *(existing)* | `WorktreeResumeSummary` | `overviewSummaryStats`, `overviewDetailSelection` |
| Archive action | `task_contexts.state = 'archived'` | *(existing)* | `task_context.go` | *(existing)* | `handleKey` or `WorktreePane` keybinding |

---

## 9. What NOT to Touch (Phase 4 Constraints)

Per `IMPLEMENTATION_PLAN.md` Section V:

- **Do not** add event log / transcript store tables.
- **Do not** add heartbeat writes to `agent_sessions`.
- **Do not** perform DB queries inside `View()` or `renderBody()`.
- **Do not** expand `page_snapshots` beyond layout blob.
- **Do not** turn tasks into a full project-management graph.

All recommended extensions above respect these constraints.

---

## 10. Recommended Next Action

Start with **Step 1** (Schema + Store). The schema blocks can be copied verbatim from this memo into `db.go`. The store files can be templated directly from `internal/store/task_context.go` (for `task_plans`) and `internal/store/agent_session.go` (for `session_handoffs`). Tests follow `context_test.go` exactly. This is a ~1-hour mechanical pass with zero architectural risk.

Once Step 1 is green, Steps 2–3 are purely additive data plumbing into the existing resume summary pipeline and overview panes. No pane registration, no layout tree changes, no new Bubble Tea model types required.
