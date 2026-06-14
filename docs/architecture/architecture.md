# Focus-tui Architecture

- **Version:** 0.2 (current)
- **Last Updated:** 2026-06-14
- **Status:** Current
- **Authoritative ADR:** [ADR-0007 — Current Implementation Consolidation](../adr/0007-current-implementation-consolidation.md)

This document describes the current implementation architecture of Focus-tui. For historical decisions and design evolution, see the [ADR index](../adr/README.md).

---

## 1. Architectural Principles

1. **Human sovereign.** Humans own architectural decisions; agents execute. The orchestrator never auto-launches agents.
2. **Agent native.** Multi-agent parallelism and external-terminal execution are first-class, not afterthoughts.
3. **Structure first.** Execution follows `ADR → Plan → Phase → Step → Session`.
4. **Terminal realism.** Work happens in shell, git, worktree, and external terminals.
5. **Protocol-driven.** Agents and the TUI interact through the same MCP interface.
6. **Worktree as container.** Each Phase binds to one git worktree; agents run inside that worktree.

---

## 2. System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         User (TUI)                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────────┐  ┌───────────┐  │
│  │ DAG     │  │Worktree │  │ Worktree    │  │  Shell    │  │
│  │ Pane    │  │ List    │  │ Detail      │  │  Pane     │  │
│  └────┬────┘  └────┬────┘  └──────┬──────┘  └─────┬─────┘  │
│       └─────────────┴──────────────┴───────────────┘        │
│                         │                                   │
│                  Bubble Tea Model                           │
│                         │                                   │
└─────────────────────────┼───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      MCP Server (HTTP)                      │
│  tools: task.*, plan.*, dag.*, session.*, kg.*, context.*   │
│  resources: context://tasks, plans, worktrees               │
└─────────────────────────┬───────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
          ▼               ▼               ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │  Command   │  │   Store    │  │Orchestrator│
   │    Bus     │  │(SQLite/PG) │  │            │
   └────────────┘  └────────────┘  └────────────┘
          │               │               │
          └───────────────┼───────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Agent (external)                       │
│  Claude / Kimi / Codex / OpenCode / Gemini / generic        │
│  Runs in its own terminal, connected via FOCUS_MCP_URL      │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Core Components

### 3.1 TUI / App (`internal/app`)

The TUI is a Bubble Tea application. The main model (`model` in `app.go`) holds:

- Common state, active page, overlay stack.
- `cmdBus` for structured domain writes.
- `mcpServer` exposing tools/resources to agents.
- `orch` consuming events and emitting notifications.
- `trellisBridge` for per-worktree context.
- `agentRegistry` / `discoveryRegistry` for session tracking.

Main panes:

| Pane | ID | Purpose |
|------|-----|---------|
| Header | `header` | Logo, weather, status |
| DAG | `dag-main` | Phase-level dependency graph |
| Worktree List | `worktree-main` | Git worktrees in the repo |
| Worktree Detail | `worktree-detail-main` | Tasks/context for selected worktree |
| Shell | `shell-main` | Embedded PTY shell |
| Footer | `footer` | Keybindings and streaks |

Overlays include task/plan editors, agent select/store, git file tree, help, todo list, etc.

### 3.2 MCP Server (`internal/mcp`)

A standard MCP server implementing protocol version `2024-11-05`:

- `initialize` / `notifications/initialized`
- `tools/list` / `tools/call`
- `resources/list` / `resources/read` / `resources/subscribe`
- `ping`

Transport:

- **Primary:** HTTP (Streamable HTTP), default `127.0.0.1:18766`; falls back to `:0` if the port is taken.
- **Legacy:** Unix domain socket at `.focus/mcp.sock` — kept for compatibility but no longer the main path.

Tool names are exposed with `_` instead of `.` (e.g. `task_create` externally, `task.create` internally).

Current tools:

| Namespace | Tools |
|-----------|-------|
| Session | `session.heartbeat`, `session.request_intervention` |
| Task | `task.get`, `task.create`, `task.list`, `task.update_status`, `task.add_dependency`, `task.create_output` |
| Plan | `plan.create`, `plan.get`, `plan.list`, `plan.add_step`, `plan.expand_to_tasks` |
| DAG | `dag.get_status` |
| Knowledge | `kg.add_fact` |
| Context | `context.get_for_task` |

Resources:

- `context://tasks`
- `context://plans`
- `context://worktrees`

### 3.3 Store (`internal/store`)

Dual storage modes selected by `FOCUS_STORE`:

| Mode | Trigger | Truth source | Event sourcing |
|------|---------|--------------|----------------|
| SQLite | default / `FOCUS_STORE=sqlite` | current-state tables | memory `EventBus` only |
| PostgreSQL | `FOCUS_STORE=postgresql` | `events` table | full, with `proj_*` projections |

Key tables (SQLite):

- `task_contexts`, `task_dependencies`, `task_outputs`, `task_briefs`, `task_worktree_links`
- `task_plans`, `plan_steps`
- `worktree_contexts`, `worktree_history`
- `agent_sessions`, `agent_messages`, `agent_definitions`
- `knowledge_facts`, `session_handoffs`, `context_notes`
- `todos`, `pomodoro_sessions`, `streaks`

SQLite is configured with WAL mode, foreign keys, and `busy_timeout = 5000`.

### 3.4 Orchestrator (`internal/orchestrator`)

A lightweight goroutine that subscribes to the `EventBus`:

- On `TaskStateChanged` → done/archived: check downstream dependencies, unblock ready tasks, notify human.
- On `TaskStateChanged` → un-done: re-block downstream tasks and notify.
- On `PlanStepStateChanged` → done: check if the whole plan is complete.
- Every 30s: detect agent sessions without heartbeat for 2 minutes, mark `disconnected`, notify.

**The orchestrator does not auto-launch agents.** Launching remains a human action.

### 3.5 Agents (`internal/agents`)

Providers: `opencode` (default), `claude`, `kimi`, `codex`, `gemini`, `generic`.

Lifecycle:

1. `agents.Session` created with UUID and task/plan/worktree bindings.
2. `agents.Registry` tracks in-memory sessions.
3. `agents.DiscoverRunningAgents()` uses `pgrep` to find already-running agents by provider binary.
4. Launch either in the embedded shell pane or in an external terminal:
   - Supported terminals: kitty, alacritty, wezterm, gnome-terminal, ptyxis.
5. Environment variables injected into the agent:
   - `FOCUS_MCP_URL`
   - `FOCUS_SESSION_ID` (legacy `FOCUS_AGENT_SESSION_ID` also set)
   - `FOCUS_TASK_ID`, `FOCUS_PLAN_ID`
   - `TRELLIS_CONTEXT_ID`

`AgentDriver` interface + `KimiDriver` exist for headless execution but are not wired into the main scheduler yet.

### 3.6 Trellis Bridge (`internal/trellis`)

Trellis is a **context bridge**, not an execution engine:

- Ensures `.trellis/` and per-platform config dirs exist.
- Creates symlinks from worktrees to Trellis context.
- Syncs task create/start/finish/archive to Trellis.
- Builds per-agent context (`AGENTS.md` / `CLAUDE.md`) before launch.
- Serves `context.get_for_task` via `GetTaskContextExtended()` (PRD, specs, handoff, journal, workflow state).

### 3.7 Command Bus & Event Bus (`internal/commands`, `internal/events`)

- `commands.Bus` validates and executes domain commands (`CreateTask`, `UpdateTaskState`, `AddTaskDependency`, etc.).
- `events.EventBus` is an in-memory pub/sub used by the store, orchestrator, and TUI.
- In PostgreSQL mode the store also appends events to the `events` table and projections refresh every 5 seconds.

---

## 4. Data Model

### 4.1 Phase-Step Task Model

- **Phase** (`task_contexts.parent_task_id IS NULL`): architecture-level stage, visible in DAG pane, binds to one worktree.
- **Step** (`task_contexts.parent_task_id IS NOT NULL`): execution-level action, transparent to humans, scheduled by the agent inside the worktree.

A Phase has:

- `preferred_worktree_id` → binds to a worktree.
- `state` ∈ {`active`, `paused`, `blocked`, `done`, `archived`}.
- Optional `task_brief` for title/goal/success/out-of-scope.

A Step inherits its Phase's worktree via `task_worktree_links(relation_type='secondary')`.

Phase-level DAG edges are derived from Step dependencies: if Step A (in Phase X) depends on Step B (in Phase Y), then Phase X depends on Phase Y.

### 4.2 Worktree

`worktree_contexts` records git worktrees:

- `worktree_id` (path or symbolic name)
- `branch`, `base_ref`, `task_mode` (`single` / `mixed` / `staging`)
- `state`
- `last_resume_summary`

Focus discovers worktrees via git and keeps them in sync.

### 4.3 Plan

`task_plans` + `plan_steps`:

- Plan status ∈ {`draft`, `approved`, `active`, `blocked`, `completed`, `discarded`, `archived`}.
- `plan.expand_to_tasks` creates one Phase per plan step, with sequential hard dependencies.
- `plan_steps.expanded_task_id` links back to the created Phase.

### 4.4 Agent Session

`agent_sessions` tracks running or historical sessions:

- `state` ∈ {`running`, `exited`, `failed`, `waiting`, `disconnected`, `unknown`}.
- `provider`, `worktree_id`, `task_id`, `plan_id`, `pid`.
- `last_heartbeat` for timeout detection.

---

## 5. Communication Protocol

### 5.1 MCP over HTTP (primary)

1. `focus` starts `mcp.Server.StartHTTP()` on a loopback address.
2. Actual URL is written to `cfg.Agent.MCPSocket`.
3. When launching an agent, Focus sets `FOCUS_MCP_URL` to that URL.
4. Agent initializes the MCP session, lists tools, and calls them to read/write shared state.

### 5.2 A2A (deprecated)

The custom A2A router in `internal/a2a/` is no longer instantiated by the main flow. It remains in the tree only because it has not been removed yet. All signaling is now done through MCP tools and the `EventBus`.

---

## 6. State Machines

### 6.1 Task / Phase State

```
        ┌─────────┐
        │ blocked │◄────────────────────────────┐
        └────┬────┘                             │
             │ human unblocks / dependency ready│
             ▼                                  │
        ┌─────────┐   human pauses   ┌────────┐│
        │ active  │◄────────────────►│ paused ││
        └────┬────┘                  └────────┘│
             │ agent completes                  │
             ▼                                  │
        ┌─────────┐   human archives           │
        │  done   │────────────────────────────►┘
        └────┬────┘
             │ human archives
             ▼
        ┌───────────┐
        │  archived │
        └───────────┘
```

### 6.2 Agent Session State

- `running` — agent process is alive and heartbeating.
- `waiting` — agent is waiting for human approval/intervention.
- `exited` — agent exited cleanly.
- `failed` — agent exited with error.
- `disconnected` — heartbeat timeout.
- `unknown` — state could not be determined.

### 6.3 Plan State

- `draft` → `approved` → `active` → `completed` / `discarded`
- Can also be `blocked` or `archived`.

---

## 7. Key Flows

### 7.1 Starting Focus

1. Resolve project root via git.
2. Load or create `~/.config/focus/config.yaml`.
3. Open Store (SQLite by default).
4. Initialize Trellis bridge.
5. Start MCP server (HTTP + optional Unix socket).
6. Start Orchestrator.
7. Render TUI with DAG / Worktree / Detail / Shell panes.

### 7.2 Creating a Phase

1. Human or agent calls `task.create` without `parent_task_id`.
2. Command bus inserts `task_contexts` row.
3. If `preferred_worktree_id` is provided, the Phase binds to that worktree.
4. Event published; TUI refreshes DAG pane.

### 7.3 Expanding a Plan

1. Human or agent calls `plan.expand_to_tasks(plan_id)`.
2. For each `plan_steps` row, create a Phase (`parent_task_id = NULL`).
3. First Phase becomes `active`; subsequent Phases become `blocked`.
4. Add sequential hard dependencies between Phases.
5. Update `plan_steps.expanded_task_id`.

### 7.4 Launching an Agent

1. Human selects a Phase / worktree and triggers agent launch.
2. `app.launchAgent()` creates `agents.Session` with UUID.
3. Trellis bridge builds agent context for the worktree.
4. Agent is launched in external terminal or embedded shell.
5. Agent reads `FOCUS_MCP_URL` and connects to Focus.
6. Agent calls `task.get`, `context.get_for_task`, etc.

### 7.5 Orchestrator Notification

1. Agent marks a Step `done` via `task.update_status`.
2. Command bus updates state and publishes `TaskStateChanged`.
3. Orchestrator sees the event, checks downstream dependencies.
4. If a blocked downstream Phase is now ready, Orchestrator changes it to `active` and emits `downstream_ready` notification.
5. Human sees notification and decides whether to launch the next Agent.

---

## 8. Configuration

User config: `~/.config/focus/config.yaml`.

```yaml
agent:
  external_terminal: false          # true = launch in external terminal
  terminal_emulator: ""             # auto-detect if empty
  mcp_port: "127.0.0.1:18766"       # MCP HTTP listen address
  research_provider: "kimi"
  architecture_provider: "claude"
  coding_provider: "codex,kimi,claude"
  heartbeat_notifications: false

editor:
  command: "nvim"
```

Environment variables:

| Variable | Purpose |
|----------|---------|
| `FOCUS_STORE` | `sqlite` (default) or `postgresql` |
| `FOCUS_DB_PATH` | Override SQLite path |
| `FOCUS_MCP_URL` | URL passed to agents |
| `FOCUS_DISABLE_PIGGYBACK` | `1` to disable context piggyback |
| `FOCUS_SESSION_ID` / `FOCUS_AGENT_SESSION_ID` | Agent session identity |
| `FOCUS_TASK_ID` / `FOCUS_PLAN_ID` | Bound task/plan |
| `TRELLIS_CONTEXT_ID` | Trellis context identity |

---

## 9. Evolution Roadmap

The current architecture is a consolidation point, not the final destination. Known next steps:

1. **Complete A2A removal.** Delete `internal/a2a/` and the `A2ASocket` config field.
2. **Event sourcing maturity.** Stabilize PostgreSQL mode and decide whether SQLite remains the default.
3. **Piggyback for SQLite.** Enable context piggyback without requiring PostgreSQL.
4. **Headless mode.** Wire `AgentDriver` / `KimiDriver` into the main scheduler for unattended execution.
5. **Session digest.** Re-enable structured handoff archiving, likely backed by Trellis journals.
6. **ADR structured storage.** Decide whether ADRs remain Markdown-only or move into a structured table.

See [ADR-0007](../adr/0007-current-implementation-consolidation.md) for the decision context behind the current state.

---

## 10. References

- [ADR Index](../adr/README.md)
- [ADR-0007 — Current Implementation Consolidation](../adr/0007-current-implementation-consolidation.md)
- [Release Process](../releases/internal-release-process.md)
- [MCP over A2A Refactor Plan](./mcp-a2a-refactor-plan.md)
