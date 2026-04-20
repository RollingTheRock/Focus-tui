# ADR-0000: Constitution for ADR-Driven Execution

- **Date:** 2026-04-19
- **Status:** Accepted

## Context

Focus-tui is shifting away from plan documents that drift independently from the codebase and from session-driven execution that repeatedly rebuilds context from scratch.

The project needs a durable top-level layer that survives individual sessions, constrains execution, and records why architectural and workflow boundaries exist. Without this layer, plans become disposable, tasks redefine their own intent, and sessions silently drift the system.

## Decision

Focus-tui will operate under an **ADR-driven execution hierarchy**:

**ADR → Plan → Task → Session**

This hierarchy is constitutional, not optional.

## Constitutional Clauses

### 1. ADR is the highest decision layer

ADRs define durable constraints, boundaries, and architectural intent. They do not change with normal execution.

### 2. Lower layers may not redefine upper layers

- Sessions may not modify Tasks, Plans, or ADRs by authority.
- Tasks may not redefine Plans or ADRs.
- Plans may not silently rewrite ADR constraints.

Execution may surface evidence upward, but it may not unilaterally change the layer above it.

### 3. Human sovereignty is mandatory

Agents may extract reality, propose decompositions, pressure-test decisions, and execute bounded work. Humans retain final convergence authority over all ADRs and all upper-layer changes.

### 4. Accepted ADRs are immutable

Once accepted, an ADR is not rewritten as a living document. If the decision changes, a new ADR must supersede it.

### 5. Supersession must preserve history

Old ADRs are not deleted. They remain readable and are marked as superseded. The new ADR must explain what changed and why the old decision is no longer sufficient.

### 6. Sessions must execute with constitutional context

Every execution session must start with the relevant ADR context and the current bounded execution slice. Running without constitutional context is invalid execution.

### 7. Outdated planning documents cannot act as current truth

Historical plans, design memos, and implementation documents may remain as evidence, but once marked outdated or superseded they are no longer valid execution entry points.

## ADR Trigger Rules

An ADR is required when a decision:

- changes object hierarchy or ownership boundaries
- changes execution authority or injection rules
- affects long-lived architecture or workflow invariants
- is costly to reverse
- will likely require future readers to understand why it was chosen

An ADR is not required for minor UI adjustments, low-cost reversible implementation details, or temporary experiments that do not change durable boundaries.

## Enforcement

This constitution is immediately enforced by workflow and will later be enforced in product seams.

### Immediate workflow enforcement

- New plans should be created under this ADR hierarchy.
- Superseded docs must be explicitly marked and not reused as active truth.
- Humans must approve upper-layer changes.

### Product enforcement targets

The codebase already contains obvious seams where constitutional checks can attach:

- agent launch
- plan save / approve / expand
- task save
- worktree resume / entry
- session exit backflow

These product-level checks are expected to evolve over time, but the constitutional rules above are already binding.

## Consequences

### Positive

- Execution becomes bounded and less prone to drift.
- Architectural reasoning becomes durable and reviewable.
- Plans, tasks, and sessions gain clearer authority boundaries.

### Negative

- More work moves upward into ADR and plan formulation.
- Sessions lose freedom to improvise beyond their scope.
- The system becomes stricter about what counts as valid execution.

## Supersession Rule

This ADR is the first constitutional record. Future constitutional changes must supersede this ADR rather than editing it in place.
