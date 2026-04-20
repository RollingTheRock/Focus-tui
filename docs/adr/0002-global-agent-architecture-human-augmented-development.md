# ADR-0002: Global Agent Architecture — Human-Augmented Development Workflow

- **Date:** 2026-04-20
- **Status:** Proposed

## Context

1. Current codebase has `internal/plugins/agents/` but only supports external agent process launching with no native agent capabilities
2. User feedback reveals external agents (Claude Code, OpenCode, Kimi CLI) suffer from session-based amnesia and inability to share context across sessions
3. Previous exploration concluded multi-agent session scheduling is not viable; a "single-agent multi-context" architecture is required
4. Market validation shows Notion AI's "human-first, agent-augmented" pattern is widely accepted by users
5. The product needs a clear agent architecture that maintains human sovereignty while leveraging agent capabilities for efficiency

## Decision

We will adopt a **global agent auxiliary + layered execution** architecture:

1. **Global Agent**: A persistent agent runtime that assists humans with ADR→Plan→Task→Session decomposition work
2. **Auxiliary Mode**: Humans invoke the agent on-demand; the agent returns suggestions for human review, with humans making final decisions
3. **Execution Layer**: Session execution can be outsourced to external agent processes (Claude Code, OpenCode, Kimi CLI), sharing key context via MCP
4. **Automation**: The agent can assist with testing, commits, and other repetitive tasks, subject to human confirmation

## Core Architecture Principles

### 1. Human-first, not agent-first

The human developer is always the entry point and final decision maker. The agent is an accelerator, not a replacement.

### 2. Agent as tool, not actor

The agent proposes but does not execute without confirmation. The agent remembers but does not decide priorities.

### 3. Context alignment over context reconstruction

The global agent maintains persistent context across the ADR→Plan→Task→Session hierarchy, eliminating the need to reconstruct context from scratch each session.

### 4. Layered assistance over flat orchestration

The global agent provides different assistance capabilities at each layer:
- **ADR layer**: Research, exploration, and drafting assistance
- **Plan layer**: Decomposition, dependency analysis, and validation
- **Task layer**: Splitting, estimation, and risk assessment
- **Session layer**: Execution, testing, and commit assistance

## Typical Usage Flow

1. Human creates ADR draft
2. *[Optional]* Invoke global agent for research → Agent returns research report → Human reviews
3. Human approves ADR
4. *[Optional]* Invoke global agent for Plan decomposition → Agent returns Plan draft → Human reviews
5. Human approves Plan
6. *[Optional]* Invoke global agent for Task splitting → Agent returns Task breakdown → Human reviews
7. Human assigns Tasks to Worktrees
8. *[Optional]* Invoke agent for Session execution, or launch external agent process
9. Agent/external process returns results
10. Human reviews results and decides next action

## Relationship to ADR-0000

Per ADR-0000 Clause 2 (Lower layers may not redefine upper layers):
- The global agent **may assist** in drafting ADRs but **cannot** modify accepted ADRs
- The global agent **may propose** Plan changes but **cannot** activate them without human approval
- The global agent **may execute** Tasks but **cannot** redefine Task boundaries

Per ADR-0000 Clause 3 (Sessions must execute with constitutional context):
- The global agent ensures Sessions receive proper context from upper layers
- The global agent validates Session outputs against Plan constraints

## Considered Alternatives

### Alternative A — Multi-agent orchestration

Rejected because multi-agent session scheduling introduces context pollution, token waste, and coordination complexity without proportional benefit.

### Alternative B — Full agent automation

Rejected because fully automated agent execution violates the human-sovereign principle and introduces unbounded drift risk.

### Alternative C — External agent only

Rejected because relying solely on external agents (Claude Code, OpenCode, etc.) perpetuates session-based amnesia and prevents native context sharing.

## Consequences

### Positive

- True human-native development experience with agent-native auxiliary support
- Persistent context across the entire ADR→Plan→Task→Session hierarchy
- Flexible execution: internal agent, external agent, or human-only
- Reduced cognitive load for repetitive decomposition and validation tasks

### Negative

- **Architecture complexity increases**: Must maintain agent runtime, context injection protocol, and MCP gateway
- **Debugging difficulty**: Human-agent collaboration flows are harder to trace than purely manual or purely agent-driven workflows
- **Context window pressure**: Global agent's context may grow with project size, requiring compaction strategies

### Neutral / Requires Further Design

- Global agent's specific runtime architecture and底座 selection
- Task scheduling mechanism for internal vs external agent dispatch
- MCP protocol implementation and external agent integration testing
- Context compaction and memory management strategies

## Non-Goals

- The global agent is not an autonomous developer
- The global agent does not replace human judgment on architectural decisions
- The global agent does not bypass ADR-0000's constitutional hierarchy

## References

- ADR-0000: Constitution for ADR-Driven Execution
- ADR-0001: Product Positioning — Human-Sovereign, Agent-Native Workbench
- Kimi CLI architecture analysis (context persistence via JSONL)
- Claude Code architecture analysis (tool use and compaction)
- OpenCode architecture analysis (session pool and MCP support)
