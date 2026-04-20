# ADR Index

This directory is the new top-level decision layer for Focus-tui.

Focus-tui now follows this hierarchy:

1. **ADR** — constitutional constraints and durable product decisions
2. **Plan** — bounded decomposition inside ADR constraints
3. **Task** — executable work units
4. **Session** — atomic execution contexts

Accepted ADRs are immutable. If a decision changes, it must be replaced by a new ADR that supersedes the old one.

## Current Records

- [ADR-0000 — Constitution for ADR-Driven Execution](./0000-constitution-for-adr-driven-execution.md)
- [ADR-0001 — Product Positioning: Human-Sovereign, Agent-Native Workbench](./0001-product-positioning-human-sovereign-agent-native-workbench.md)
