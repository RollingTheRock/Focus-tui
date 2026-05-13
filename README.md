# focus

> Stop juggling terminals.  
> Run parallel agent development across multiple worktrees without losing control.

`focus` is a terminal-native execution workbench designed for one hard problem:
**how to run multiple coding agents across multiple worktrees in parallel without losing control of context, dependencies, state, and handoff.**

---

## The Real Problem

Parallel development does not fail because people cannot open enough terminals.
It fails because execution becomes invisible and brittle:

- multiple agents run at the same time, but ownership and progress are unclear
- multiple worktrees evolve in parallel, but task-to-branch mapping drifts
- upstream/downstream task dependencies break silently
- session outputs exist, but handoff is not structured for reliable takeover
- humans become manual schedulers across window chaos

`focus` is not built to replace coding agents.
It is built to make parallel execution **observable, controllable, and auditable**.

---

## Agent Neutral, Operator Sovereign

One core principle of `focus` is:
**developers should not be locked into a single agent coding tool.**

In practice, delivery stability is usually decided less by headline model differences and more by:

- runtime stability in your real workflow
- response speed and predictability
- provider-native integration quality

That is why advanced users often run multiple tools side by side, for example:

- `Kimi CLI`
- `Codex`
- `Claude Code`
- `OpenCode` (for models without mature first-party agent tooling)

`focus` does not ask you to switch preferences.
It unifies your preferred agent stack into one execution system.

- plug in the agents you already trust
- manage them under one workbench
- coordinate multi-agent execution against shared task/worktree structure
- use built-in `cc-switch` to launch and switch provider-specific `Claude Code` and `Codex`

You choose the best agent for the moment.
`focus` keeps the system coherent.

---

## How focus Handles Parallel Multi-Agent Work

`focus` uses a structured execution model:

- **Worktree as execution container**
  each worktree acts as an isolated execution unit for branch/task/session activity
- **Task DAG as source of truth**
  dependency flow is explicit rather than informal
- **Session-to-worktree mapping**
  agent sessions are anchored to worktree/task context
- **Protocol-driven coordination (MCP)**
  shared state is managed through explicit tools/resources
- **Structured handoff**
  session completion is captured as takeover-ready context

This is not “chat-first coding.”
This is execution-system design for real parallel delivery.

---

## Philosophy

1. **Human Sovereign**  
   humans own decisions and arbitration; agents execute
2. **Agent Native**  
   multi-agent parallelism is a default, not an afterthought
3. **Structure First**  
   `ADR -> Plan -> Task -> Session` is operational scaffolding, not decoration
4. **Terminal Realism**  
   real engineering happens in shell, git, worktree, and scripts
5. **Auditable Execution**  
   progress must be inspectable, reproducible, and reversible

---

## Quickstart

### Prerequisites

- Go `1.25+`
- Unix-like terminal environment recommended

### Build

```bash
go build ./cmd/focus
```

### Run

```bash
./focus
```

### Test

```bash
go test ./...
```

Note: in restricted environments, some MCP HTTP/socket tests may fail due to OS permission constraints.

---

## Internal Release Policy (Current)

`focus` currently uses an **internal testing release strategy**.
It is not positioned as a public stable release pipeline yet.

### Versioning

- `v0.x.y-rc.N`
- `v0.x.y-internal.N`

### Release Gates

1. `go build ./cmd/focus` passes
2. core tests pass in a non-restricted environment
3. parallel-critical smoke paths pass:
   - worktree list/switch workflow
   - task DAG transitions
   - agent session mapping
   - MCP baseline tool/resource interactions

### Artifacts

- multi-platform binaries
- `checksums.txt`
- internal release notes (changes, risks, rollback guidance)

---

## Repository Map

```text
cmd/                   # application entrypoints
internal/app/          # top-level TUI runtime and page orchestration
internal/plugins/git/  # worktree/git panes and interactions
internal/agents/       # agent session model and drivers
internal/mcp/          # MCP server and protocol layer
internal/orchestrator/ # task-state orchestration
internal/store/        # persistence for task/plan/session/worktree context
docs/adr/              # architecture decision records
docs/architecture/     # architecture specifications
docs/plans/            # implementation and cleanup plans
docs/research/         # analysis and audit artifacts
docs/releases/         # release process docs
docs/archive/          # historical archived documents
```

---

## Documentation

- `docs/architecture/`
- `docs/adr/`
- `docs/plans/`
- `docs/research/`
- `docs/releases/`
- `docs/archive/`

---

## Who focus Is For

If you only need a single-agent coding chat surface, `focus` may feel heavy.
If you are running **multi-worktree, multi-agent, parallel software execution** and need control instead of terminal chaos, this is what `focus` is built for.
