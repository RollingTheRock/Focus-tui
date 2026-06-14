# Focus-tui Architecture

This directory describes the current system architecture of Focus-tui. The authoritative consolidated decision is [ADR-0007: Current Implementation Consolidation](../adr/0007-current-implementation-consolidation.md).

## Document Navigation

| Document | Content |
|----------|---------|
| [architecture.md](./architecture.md) | Current architecture: components, data model, protocols, state machines, flows |
| [mcp-a2a-refactor-plan.md](./mcp-a2a-refactor-plan.md) | Historical refactor plan that moved MCP to HTTP and deprecated A2A |
| [ADR-0000](../adr/0000-constitution-for-adr-driven-execution.md) | Constitution: ADR → Plan → Task → Session hierarchy |
| [ADR-0001](../adr/0001-product-positioning-human-sovereign-agent-native-workbench.md) | Product positioning: human sovereign, agent native |
| [ADR-0004](../adr/0004-protocol-driven-multi-agent-orchestration.md) | Decision to move to decentralized Agent Mesh and external terminals |
| [ADR-0006](../adr/0006-dag-pane-phase-step-hierarchy.md) | Phase-Step task layering |
| [ADR-0007](../adr/0007-current-implementation-consolidation.md) | Consolidated snapshot of the current implementation |

## System at a Glance

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

## Key Characteristics

- **Agents run in external terminals.** Each agent is an independent process with its own native terminal experience.
- **Focus-tui is a command center.** It shows scheduling state, worktrees, and task context, not agent output.
- **Protocol-driven.** The UI and agents share the same MCP interface.
- **Human sovereign.** The orchestrator notifies but never auto-launches agents.
- **Decentralized.** No Global Agent, no central brain.
- **Worktree-native.** Each Phase binds to one git worktree; agents execute inside it.

## Quick Links

- [Core Components](./architecture.md#3-core-components)
- [Data Model](./architecture.md#4-data-model)
- [Communication Protocol](./architecture.md#5-communication-protocol)
- [State Machines](./architecture.md#6-state-machines)
- [Key Flows](./architecture.md#7-key-flows)
- [Configuration](./architecture.md#8-configuration)
