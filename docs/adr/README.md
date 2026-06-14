# ADR Index

This directory is the new top-level decision layer for Focus-tui.

Focus-tui now follows this hierarchy:

1. **ADR** — constitutional constraints and durable product decisions
2. **Plan** — bounded decomposition inside ADR constraints
3. **Task** — executable work units
4. **Session** — atomic execution contexts

Accepted ADRs are immutable. If a decision changes, it must be replaced by a new ADR that supersedes the old one.

## Current Records

### Accepted

- [ADR-0000 — Constitution for ADR-Driven Execution](./0000-constitution-for-adr-driven-execution.md)
- [ADR-0001 — Product Positioning: Human-Sovereign, Agent-Native Workbench](./0001-product-positioning-human-sovereign-agent-native-workbench.md)
- [ADR-0004 — Protocol-Driven Multi-Agent Orchestration with External Terminal Execution](./0004-protocol-driven-multi-agent-orchestration.md)
- [ADR-0006 — DAG Pane Phase-Step 分层与 Agent 粒度约束](./0006-dag-pane-phase-step-hierarchy.md)
- [ADR-0007 — Current Implementation Consolidation: Protocol, Storage, and Orchestration as of Today](./0007-current-implementation-consolidation.md)

### Proposed / In Exploration

- [ADR-0005 — Event-Sourced Shared Context Store](./0005-event-sourced-shared-context-store.md)
- [Dual-Mode Agent Architecture](./dual-mode-agent-architecture.md)

### Superseded

- [ADR-0002 — Global Agent Architecture: Human-Augmented Development](./0002-global-agent-architecture-human-augmented-development.md) — superseded by ADR-0004
- [ADR-0003 — Global Agent Architecture: Technical Selection](./0003-global-agent-architecture-technical-selection.md) — superseded by ADR-0004
