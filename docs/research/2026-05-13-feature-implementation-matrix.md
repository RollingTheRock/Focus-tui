# Feature Implementation Matrix (Safety Baseline)

Date: 2026-05-13
Branch: feat-clean-code
Purpose: Define feature-to-code mapping before cleanup, so repository cleanup does not break active functionality.

## Scope

This matrix separates:
1. Product feature status (in use vs not in use)
2. Package-level reachability (actively wired vs currently not wired)

A package being unreachable does not automatically mean the product feature is unused.

## Summary

| Feature | Product Status | Primary Implementation Path | Cleanup Risk |
|---|---|---|---|
| App startup and TUI runtime | Active | `cmd/focus/main.go`, `internal/app/app.go` | High |
| Embedded shell pane | Active | `internal/ui/shell`, `internal/app/page.go` | High |
| Worktree management workflow | Active | `internal/plugins/git`, `internal/git`, `internal/app/worktree_detail_pane.go`, `internal/commands/worktree*.go` | High |
| Task/Plan/DAG workflow | Active | `internal/app/dag_pane.go`, `internal/store/task_*`, `internal/store/plan_*`, `internal/orchestrator` | High |
| Agent session management | Active | `internal/agents`, `internal/plugins/agents`, `internal/app/app.go` | High |
| MCP tool/resource server | Active | `internal/mcp`, `internal/app/app.go` | High |
| Todo overlay | Active | `internal/ui/todo`, `internal/app/page.go` | Medium |
| Pomodoro domain/event pipeline | Partially active | `internal/config/config.go`, `internal/store/session.go`, `internal/events/types.go`, `internal/projections/builder.go` | Medium |
| Pomodoro panel (`internal/ui/pomodoro`) | Not wired to current page | `internal/ui/pomodoro` (no active page registration found) | Low/Medium |
| A2A router package (`internal/a2a`) | Not wired in current runtime path | `internal/a2a` | Low |
| Session digest package (`internal/sessiondigest`) | Disabled integration point | `internal/sessiondigest`, disabled call in `internal/app/app.go` | Low |
| Legacy worktree registry package (`internal/worktree`) | Not wired in current runtime path | `internal/worktree/registry.go` | Low |
| Layout test command (`cmd/layouttest`) | Dev-only helper | `cmd/layouttest/main.go` | Low |

## Evidence Map

## 1) Runtime Entry and Core Wiring

- App entry initializes store and app model:
  - `cmd/focus/main.go:44` (`MarkOverdue`)
  - `cmd/focus/main.go:58` (`app.New(cfg, st)`)
- App runtime starts MCP endpoints and registers plugins:
  - `internal/app/app.go:174` (`mcpServer.Start`)
  - `internal/app/app.go:177` (`mcpServer.StartHTTP`)
  - `internal/app/app.go:190-193` (register git/filebrowser/editor/agent plugins)

## 2) Worktree Feature (Active)

The feature is active and used, but not through `internal/worktree` package.

- Worktree plugin provides pane types and UI behavior:
  - `internal/plugins/git/plugin.go:26` (PaneTypes includes `Worktree`, `GitStatus`, `DiffView`)
- Overview page creates worktree panes:
  - `internal/app/page.go:133-134` (`CreatePane(PaneTypeWorktree, ...)`)
  - `internal/app/page.go:138` (`newWorktreeDetailPane`)
- App uses Git adapter-based worktree operations:
  - `internal/app/app.go:3989` (`worktreeList` usage)
  - `internal/app/app.go:4044` (`ListWorktrees`)
  - `internal/app/app.go:3833` (`RemoveWorktree`)
- Command/store paths for worktree metadata are active:
  - `internal/commands/worktree.go`
  - `internal/commands/worktree_cleanup.go`

Package-level nuance:
- `internal/worktree/registry.go` currently has no import/use in runtime path; treat as candidate legacy package, not as worktree feature removal target.

## 3) Pomodoro (Split Status)

- Pomodoro concept/config/events are present in active code paths:
  - `internal/config/config.go:58-63` (pomodoro defaults)
  - `internal/store/session.go:49-107` (append pomodoro events)
  - `internal/events/types.go:69-71` (pomodoro event types)
  - `internal/projections/builder.go:119-124` and `:441+` (apply pomodoro events)
- But current overview page does not register pomodoro panel:
  - `internal/app/page.go:150` registers `todo.New(common)`
  - no active `pomodoro.New(...)` registration found in `internal/app/page.go` or `internal/app/app.go`

Conclusion:
- Do not claim "pomodoro feature fully removed".
- Do not remove pomodoro domain/event code blindly.
- `internal/ui/pomodoro` is a package-level cleanup candidate only after explicit product decision.

## 4) MCP/Task/Plan/Agent Flows (Active)

- MCP protocol/server and HTTP transport are implemented and tested:
  - `internal/mcp/protocol.go`
  - `internal/mcp/server.go`
- Orchestrator listens to event bus and reacts to task state changes:
  - `internal/orchestrator/orchestrator.go`
- Task/Plan/Worktree/Agent records are persisted in store package:
  - `internal/store/task_context.go`
  - `internal/store/task_plan.go`
  - `internal/store/plan_step.go`
  - `internal/store/agent_session.go`

## 5) Packages currently not wired in main runtime path

- `internal/a2a`
- `internal/sessiondigest`
- `internal/worktree`
- `internal/ui/pomodoro` (panel package only)
- `cmd/layouttest`

These should be handled via explicit "freeze/migrate/remove" decisions instead of immediate deletion.

## Cleanup Guardrails (must satisfy before and after each cleanup batch)

1. Build and tests (where environment permits):
   - `GOCACHE=/tmp/go-build go test ./...`
2. Runtime smoke checks:
   - App starts without panic
   - Worktree pane loads
   - Task/DAG pane interaction works
   - MCP server startup path remains intact
3. No cleanup PR may delete code from any "Active" feature row unless replacement is merged in same batch.

## Initial Decision Recommendations

1. Keep: all active feature paths above.
2. Mark as `candidate-legacy`: `internal/worktree`, `internal/ui/pomodoro`.
3. Mark as `freeze-candidate`: `internal/a2a`, `internal/sessiondigest`.
4. Mark as `safe-remove`: `cmd/layouttest`, tracked binary artifact under `bin/`.

