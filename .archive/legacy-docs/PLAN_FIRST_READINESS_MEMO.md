# MEMO: Focus-tui Plan-First Readiness Audit

> [过时归档说明]
> 本 memo 的价值在于记录 **旧系统为何仍是 task-first**。
> 自 2026-04-19 起，它不再描述目标状态，而是 ADR-first 转型前的审计证据。

**Date:** 2026-04-18  
**Scope:** Schema, models, panes, builder logic, and interaction flows  
**Constraint:** Evidence only — no edits, no builds, no speculation.

---

## 1. Executive Summary

The codebase has **plan-as-child-of-task** infrastructure, not **plan-as-parent-of-task** infrastructure. Every schema FK, store query signature, workbench builder, and overview pane assumes the user lands on a **worktree** or **task** first, then optionally attaches a plan. A true plan-first hierarchy (plan > task > session) would require a new top-level entity, inverted FKs, and a redesigned overview rendering pipeline.

**Verdict:** The current substrate supports "task with optional plan" well. It does **not** support "plan containing tasks" without schema redesign.

---

## 2. Schema Analysis: FK Directions Encode Task-First Semantics

### 2.1 Current Foreign-Key Topology

| Child Table | Parent FK | Direction | File / Line |
|-------------|-----------|-----------|-------------|
| `task_plans` | `task_id` → `task_contexts(id)` | Plan belongs to Task | `internal/store/db.go:150` |
| `plan_steps` | `plan_id` → `task_plans(id)` | Step belongs to Plan | `internal/store/db.go:166` |
| `session_handoffs` | `task_id` → `task_contexts(id)` | Handoff belongs to Task | `internal/store/db.go:181` |
| `session_handoffs` | `plan_id` → `task_plans(id)` (nullable) | Handoff optionally refs Plan | `internal/store/db.go:182` |
| `worktree_contexts` | `primary_task_id` → `task_contexts(id)` | Worktree belongs to Task | `internal/store/db.go:116` |
| `task_worktree_links` | `task_id` → `task_contexts(id)` | Link belongs to Task | `internal/store/db.go:134` |
| `context_notes` | `task_id` → `task_contexts(id)` (nullable) | Note optionally refs Task | `internal/store/db.go:198` |
| `agent_sessions` | `worktree_id` (text, no FK) | Session refs Worktree | `internal/store/db.go:216` |

**Evidence of task-first encoding:**
- `task_plans.task_id` is `NOT NULL` and has `ON DELETE CASCADE`. A plan cannot exist without a task.
- `worktree_contexts.primary_task_id` FK means the worktree is annotated by a task, not the reverse.
- `session_handoffs.task_id` is `NOT NULL` while `plan_id` is nullable. Handoff is anchored to task first.

### 2.2 What Plan-First Would Require

A plan-first schema would need at minimum:
- `plans` table as the root entity (no `task_id` FK)
- `plan_tasks` junction or `tasks.plan_id` FK (task belongs to plan)
- `plan_sessions` or `task_sessions` with plan ancestry discoverable
- Overview queries that `GROUP BY plan_id` instead of `GROUP BY worktree_id`

None of these exist today.

---

## 3. Model & Store Interface: Plan Methods Are Task-Scoped

### 3.1 Store Interface Signatures

File: `internal/models/ui.go` lines 41–51

```go
SaveTaskPlan(record TaskPlanRecord) error
GetTaskPlan(id string) (*TaskPlanRecord, error)
ListTaskPlans(taskID string) ([]TaskPlanRecord, error)   // <-- task-scoped

SavePlanStep(record PlanStepRecord) error
ListPlanSteps(planID string) ([]PlanStepRecord, error)

SaveSessionHandoff(record SessionHandoffRecord) error
ListSessionHandoffs(taskID string) ([]SessionHandoffRecord, error)  // <-- task-scoped
```

**Evidence:** `ListTaskPlans` takes a `taskID` parameter. There is no `ListTasksByPlan(planID)` method.

### 3.2 Record Structs

File: `internal/models/ui.go` lines 111–147

```go
type TaskPlanRecord struct {
    ID          string
    TaskID      string        // mandatory parent
    Title       string
    Status      string
    CurrentStep string
    PlanBody    string
    ...
}

type SessionHandoffRecord struct {
    ID        string
    TaskID    string        // mandatory
    PlanID    *string       // optional
    ...
}
```

**Evidence:** Both structs carry the task as the mandatory anchor. The plan is an optional attribute, not the organizing container.

---

## 4. Workbench Context Builder: Worktree-Centric, Not Plan-Centric

### 4.1 Data Flow

File: `internal/app/workbench_context.go` lines 32–106

```go
type workbenchOverviewContext struct {
    Items    []workbenchQueueItem   // one per worktree
    Selected *workbenchQueueItem
    Stats    overviewSummaryStats
    Detail   overviewDetailSelection
}

func buildWorkbenchOverviewContext(source workbenchContextSource) workbenchOverviewContext {
    for _, item := range source.OrderedContexts() {   // iterates worktrees
        queueItem := buildWorkbenchQueueItem(item.Worktree, item.Summary, item.Activity)
        ctx.Items = append(ctx.Items, queueItem)
        ...
    }
}
```

**Evidence:** The builder iterates `OrderedContexts()` which returns a slice of `WorktreeContextView` (one per git worktree). There is no `OrderedPlans()` or plan-level grouping.

### 4.2 Resume Summary Cache

File: `internal/app/app.go` lines 1572–1667 (`refreshResumeSummaryCache`)

```go
for _, wc := range worktreeContexts {
    summary := gitmodel.WorktreeResumeSummary{...}
    if wc.PrimaryTaskID != nil {
        if task, ok := tasksByID[*wc.PrimaryTaskID]; ok {
            summary.TaskTitle = task.Title
            summary.TaskGoal = task.Goal
            ...
        }
    }
    if summary.TaskID != "" {
        plans := m.listTaskPlans(summary.TaskID)   // plans fetched per-task
        ...
    }
    m.resumeSummaryCache[wc.WorktreeID] = summary
}
```

**Evidence:** The outer loop is over `worktreeContexts`. Plans are fetched inside the loop using the task ID derived from the worktree's `primary_task_id`. The cache key is `worktreeID`, not `planID`.

---

## 5. Overview Panes: Stats and Details Are Task/Worktree Dimensions

### 5.1 Summary Pane

File: `internal/app/overview_summary_pane.go` lines 13–22, 56–64

```go
type overviewSummaryStats struct {
    Total        int
    Active       int   // task state == "active"
    Blocked      int   // task state == "blocked"
    Queued       int   // queued task links
    Dirty        int   // git dirty
    RunningAgent int   // agent sessions
    Focus        int   // resume score heuristic
    Recent       string
}
```

Rendered cards: `WORKTREES`, `ACTIVE`, `BLOCKED`, `QUEUED`, `DIRTY`, `AGENTS`, `FOCUS`.

**Evidence:** There is no `PLANS` or `PLANNED` counter. The stats vocabulary is entirely worktree/task/git/runtime.

### 5.2 Detail Pane

File: `internal/app/overview_detail_pane.go` lines 12–38, 91–105

```go
type overviewDetailSelection struct {
    Title             string
    Branch            string
    WorktreePath      string
    State             string   // task state
    Priority          string   // task priority
    Goal              string   // task goal
    NextStep          string   // task next_step
    PlanTitle         string   // plan attribute
    PlanStatus        string   // plan attribute
    CurrentPlanStep   string   // plan attribute
    ...
}
```

View renders sections: `Now`, `Plan`, `Context`, `Signals`, `Notes`, `Path`.

**Evidence:** Even though a `Plan` section exists, it is rendered **inside** a worktree/task detail view. The selection struct is scoped to one worktree. There is no plan-level detail pane that lists tasks underneath it.

---

## 6. Interaction Flows: Tasks Are Created First, Plans Are Optional Attachments

### 6.1 Task Edit Pane

File: `internal/app/task_edit_pane.go` lines 42–52, 71–77

```go
type taskEditorSeed struct {
    TaskID       string
    WorktreeID   string
    Title        string
    Goal         string
    NextStep     string
    State        string
    Priority     string
    RelationType string
    ParentTaskID string
}

const (
    taskEditFieldTitle = iota
    taskEditFieldGoal
    taskEditFieldNextStep
    taskEditFieldState
    taskEditFieldPriority
)
```

**Evidence:** The task creation overlay has no `PlanID` field, no plan selector, and no plan step editor. The user creates a task (title, goal, next step, state, priority) first. Plan attachment is not part of the creation flow.

### 6.2 Worktree Pane Keybindings

File: `internal/plugins/git/worktree_pane.go` lines 241–268

```go
case "e":
    // "Editing task for <worktree>"
    return OpenTaskEditMsg{TaskID: summary.TaskID, WorktreeID: wt.Path, RelationType: "primary"}
case "f":
    // "Adding follow-up for <worktree>"
    return OpenTaskEditMsg{WorktreeID: wt.Path, RelationType: "queued", ParentTaskID: summary.TaskID}
case "s":
    // "Cycling task state for <worktree>"
    return CycleTaskStateMsg{TaskID: summary.TaskID, WorktreeID: wt.Path}
```

**Evidence:** The primary actions on a worktree are "edit task", "add follow-up task", and "cycle task state". There is no "attach plan", "create plan", or "view plan" keybinding. Plans are invisible in the worktree interaction model.

---

## 7. Resume Summary Struct: Plan Fields Are Decorations

File: `internal/git/worktree.go` lines 30–56

```go
type WorktreeResumeSummary struct {
    TaskID            string
    TaskTitle         string
    TaskGoal          string
    NextStep          string
    TaskState         string
    TaskPriority      string
    TaskMode          string
    ...
    PlanTitle         string   // decoration on task
    PlanStatus        string   // decoration on task
    CurrentPlanStep   string   // decoration on task
    HandoffEntrypoint string
    ...
}
```

**Evidence:** `PlanTitle`, `PlanStatus`, and `CurrentPlanStep` are string fields on a task-scoped summary struct. They carry display text, not foreign keys or child collections. There is no `PlanID` field, no `Tasks []TaskSummary`, and no nested hierarchy.

---

## 8. What Is Compatible vs. What Assumes Task-First

### 8.1 Compatible with Plan-First (minimal change)

| Component | Compatibility | Notes |
|-----------|--------------|-------|
| `task_contexts.parent_task_id` | Compatible | Already supports task hierarchies; could be reused for plan-task nesting with semantic reinterpretation. |
| `plan_steps` table | Compatible | Clean child-of-plan schema; steps naturally belong to a plan. |
| `session_handoffs.plan_id` | Compatible | Nullable FK already allows linking a handoff to a plan. |
| `context_notes` | Compatible | Free-form notes can attach to plans if `task_id` semantics are relaxed or a `plan_id` column is added. |
| `worktree_contexts.task_mode` | Compatible | `"single"` / `"mixed"` / `"staging"` is agnostic to whether the task came from a plan. |

### 8.2 Encodes Task-First Assumptions (requires redesign)

| Component | Assumption | Redesign Needed |
|-----------|-----------|-----------------|
| `task_plans.task_id` NOT NULL FK | Plan cannot exist without pre-created task | Invert: add `tasks.plan_id` FK or `plan_tasks` junction. |
| `ListTaskPlans(taskID string)` | Query plans by task | Add `ListTasksByPlan(planID string)`. |
| `worktree_contexts.primary_task_id` | Worktree anchored to task | Add `worktree_contexts.primary_plan_id` or indirect via task. |
| `refreshResumeSummaryCache()` outer loop | Iterates worktrees | Add `refreshPlanSummaryCache()` that iterates plans. |
| `overviewSummaryStats` | Counters are worktree/task/git dimensions | Add `Planned`, `PlanActive`, `PlanCompleted` counters. |
| `overviewDetailSelection` | Scoped to one worktree | Add `planDetailSelection` scoped to one plan with task list. |
| `TaskEditPane` | Creates task without plan context | Add plan picker / plan creation fields. |
| `WorktreePane` keybindings | `e` = edit task, `f` = follow-up task | Add `p` = create/attach plan, `P` = view plan. |
| `buildWorkbenchOverviewContext()` | Source is `workbenchContextSource` (worktrees) | Add `planContextSource` (plans) and toggle between views. |

---

## 9. Concrete File Inventory for Redesign Path

If the goal is a true plan-first hierarchy, these files **must** change:

| File | Why | Lines of Impact |
|------|-----|-----------------|
| `internal/store/db.go` | Schema FK inversion or new junction tables | ~30–50 |
| `internal/models/ui.go` | New `PlanRecord` root struct; inverted store methods | ~40–60 |
| `internal/store/task_plan.go` | `task_id` must become nullable or move to junction | ~20–40 |
| `internal/app/app.go` | `refreshResumeSummaryCache` needs plan-first pipeline | ~100–150 |
| `internal/app/workbench_context.go` | Builder needs plan-level grouping | ~50–80 |
| `internal/app/overview_summary_pane.go` | New plan-centric stat cards | ~20–30 |
| `internal/app/overview_detail_pane.go` | Plan detail view with task list | ~40–60 |
| `internal/app/task_edit_pane.go` | Plan selection / creation UI | ~30–50 |
| `internal/plugins/git/worktree_pane.go` | New keybindings for plan actions | ~20–30 |
| `internal/git/worktree.go` | `WorktreeResumeSummary` may need `PlanID` | ~5–10 |

---

## 10. Conclusion

The current codebase is well-architected for **"task with optional plan and steps"**. The schema, store interface, workbench builder, and overview panes all treat the plan as an enrichment layer attached to an existing task.

Mapping an external **plan-first** pattern (plan created first, tasks derived from it, sessions attached to tasks) onto the current repo would require:
1. **Schema inversion:** Make `task_plans` a root table or add `tasks.plan_id`.
2. **Store inversion:** Add plan-scoped queries.
3. **UI pipeline inversion:** Add a plan-centric overview builder and detail pane.
4. **Interaction flow inversion:** Allow plan creation before task creation.

This is feasible — the existing patterns (task context, worktree context, resume summary cache, overview panes) can be cloned and retargeted to plans — but it is **not** a thin layer on top of current code. It is a parallel hierarchy that shares some tables but reverses the navigation topology.

*End of memo.*
