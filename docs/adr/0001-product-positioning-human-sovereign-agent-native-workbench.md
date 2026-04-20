# ADR-0001: Product Positioning — Human-Sovereign, Agent-Native Workbench

- **Date:** 2026-04-19
- **Status:** Accepted

## Context

Focus-tui did not start with a fully coherent product definition. It accumulated shell, git, worktree, and agent-facing capabilities while searching for the right center of gravity.

At the same time, AI-assisted development revealed a deeper problem: speed is not the primary failure mode. Drift is. Session-by-session execution without durable upper-layer constraints produces fluent output, but unstable direction.

The product therefore needs a positioning decision that clarifies what Focus-tui is, what problem it exists to solve, and what it refuses to become.

## Decision

Focus-tui is an **ADR-driven developer workbench for human-sovereign, agent-native software execution**.

It is not a chat-first coding surface, not a generic agent dashboard, and not a terminal clone of a GUI IDE. Its purpose is to let humans use agents aggressively without surrendering direction.

Focus-tui does this by organizing development through explicit layers:

**ADR → Plan → Task → Session**

Agents are first-class participants in exploration and execution, but humans retain final convergence authority over upper-layer decisions.

## Core Product Principles

### 1. Human-sovereign, not agent-sovereign

Agents may assist, propose, explore, and execute. Humans own final convergence on architectural and planning boundaries.

### 2. Agent-native, not agent-incidental

Agents are not treated as add-on plugins. The workbench is designed with agent participation as a native execution reality.

### 3. Structure over conversational drift

The product does not rely on each session reconstructing context from memory or prompts. Durable upper layers must exist before bounded execution begins.

### 4. Layered execution over flat orchestration

Focus-tui is not primarily a tool for “managing many agents.” It is a system for separating architectural constraint, planning, execution units, and atomic execution into distinct layers.

### 5. Terminal-native does not mean terminal-limited

The terminal is the current primary workbench form because it supports dense, scriptable, agent-compatible workflows. The deeper product value is the execution model, not terminal aesthetics.

### 6. Planning and constitutional logic should remain separable from presentation

The ADR/planning core should be able to evolve as a distinct module or product surface, while Focus-tui acts as a workbench host for that logic.

## Non-Goals

Focus-tui is not trying to be:

- a generic “AI coding chat” experience
- a GUI IDE recreation inside a terminal
- a pure session launcher without durable upper-layer constraints
- a system where sessions can rewrite plans or architecture on the fly

## Considered Alternatives

### Alternative A — Agent dashboard

Rejected because “seeing and launching agents” does not solve drift, ownership, or execution-boundary problems.

### Alternative B — Chat-first coding surface

Rejected because single-session prompt quality does not solve cross-session alignment or durable intent.

### Alternative C — Generic terminal workspace

Rejected because a neutral workspace does not explain why Focus-tui needs ADRs, plans, tasks, and bounded execution.

## Consequences

### Positive

- The product gains a clear center: bounded agent execution under human-owned constraints.
- Future planning and session features can be judged against a stable positioning decision.
- The repo can evolve toward a separable planning/ADR core without losing the workbench identity.

### Negative

- The product becomes more opinionated and less generic.
- Session freedom is intentionally constrained.
- Some previously tolerated “just start a session and figure it out” workflows become second-class.

## Relationship to ADR-0000

ADR-0000 defines the constitutional hierarchy and authority model. This ADR defines the product identity that lives inside that constitutional model.
