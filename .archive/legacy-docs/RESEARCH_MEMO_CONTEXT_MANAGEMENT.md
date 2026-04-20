# MEMO: Engineering-First Context Management Systems for AI Coding Agents
## Research Findings — Public Sources & Concrete Patterns

**Date:** 2026-04-17
**Scope:** Structured plans, issues, sessions, handoffs, archives, and what is persisted vs. regenerated across Sidecar/TD, Claude Code, Cursor/Windsurf/Devin, and open-source systems.

---

## 1. EXECUTIVE SUMMARY

Across the AI coding agent landscape, a consistent engineering consensus has emerged: **context windows are a scarce, degradable resource, not an infinite memory bank.** The most robust systems treat context management as a first-class architectural concern—not an afterthought. This memo documents how public tools and documented patterns handle:

- **Context granularity** (what gets loaded, when, and at what fidelity)
- **Task/session separation** (durable state vs. ephemeral conversation)
- **Plan persistence** (filesystem-backed DAGs, progress files, structured tasks)
- **Handoff protocols** (structured state transfer between sessions/agents)
- **Archival** (cold storage, session history, stale context detection)
- **What is persisted vs. regenerated** (the critical design boundary)

---

## 2. SIDECAR / TD — EXTERNAL TASK MEMORY FOR AI AGENTS

### Source
- **TD GitHub:** `marcus/td` (MIT, 49 releases, Go)
- **Sidecar GitHub:** `marcus/sidecar` (MIT, Go, TUI dashboard)
- **Docs:** `sidecar.haplab.com/docs/td`, `marcus.github.io/td`

### Architecture
TD is a **local-first, SQLite-backed task system** explicitly built to solve the "context window ends, memory ends" problem. It operates as external memory that agents invoke via CLI.

```
.todos/
├── db.sqlite          # All issues, logs, handoffs, sessions
└── sessions/          # Per-branch session state
```

### Core Concepts
| Concept | Description |
|---------|-------------|
| **Sessions** | Every agent/terminal gets a unique session ID scoped by `git branch + agent type` (Claude Code, Cursor, Copilot, etc.) |
| **Issues** | Typed work items with status, priority, labels |
| **Structured Handoffs** | `td handoff` captures: done, remaining, decisions, uncertain |
| **Epics & Dependencies** | Multi-issue workflows with blocker tracking |
| **TDQ Query Language** | SQL-like filtering for boards and queries |

### What Is Persisted
- **Task state** (SQLite): issues, status, handoffs, activity logs, session metadata
- **Session isolation**: different sessions cannot approve their own work (prevents "works on my context" bugs)
- **Git snapshots**: optional integration for automatic state capture

### What Is Regenerated
- **Conversation history**: TD does not persist raw LLM turns; it persists *derived state* (handoffs, decisions)
- **Context reconstruction**: next session runs `td usage --new-session` to see open work, then reconstructs context from the task DB

### Handoff Pattern
```bash
td handoff td-a1b2 \
  --done "OAuth flow, token storage" \
  --remaining "Refresh token rotation" \
  --decisions "Using JWT not sessions" \
  --uncertain "Rate limit values TBD"
```

### Key Engineering Decision
**Passive context beats active tools.** When knowledge is injected at session start (via `td usage`), the agent reads it before processing the user's request. TD's design philosophy: *local-first, minimal, CLI-native, agent-optimized.*

---

## 3. CLAUDE CODE — NATIVE CONTEXT ENGINEERING

### Sources
- **Anthropic docs / changelogs:** Claude Code v2.1.16+ (Tasks), Session Memory (beta), `/compact`, `/clear`
- **GitHub issues:** `anthropics/claude-code#6207` (Persistent Plan Storage — closed as completed)
- **Community analysis:** VentureBeat (Tasks update), Agent Factory (Tasks system), Blake Crosley (Context Engineering is Architecture)
- **Filesystem inspection:** `~/.claude/` directory structure (documented by Diljit PR)

### 3.1 Filesystem Architecture
```
~/.claude/
├── projects/<project-hash>/<session-id>/session-memory/summary.md  # Auto-summaries
├── tasks/<uuid>/                    # Filesystem-backed task DAGs
│   ├── .lock
│   ├── .highwatermark
│   ├── 1.json, 2.json, ...         # Individual tasks (status, deps, owner)
├── plans/                           # Plan mode output (markdown)
├── handoffs/                        # User/community handoff docs
├── configs/                         # JSON configs, thresholds, rules
├── state/                           # Runtime state (recursion depth, agent lineage)
├── docs/                            # System documentation files
├── transitions/                     # Session boundary docs (community plugins)
├── MEMORY.md                        # Persistent memory (user-managed)
└── CLAUDE.md                        # Project context (loaded every session)
```

### 3.2 Session Memory (Automatic Cross-Session Context)
- **Storage:** `~/.claude/projects/<project-hash>/<session-id>/session-memory/summary.md`
- **Mechanism:** Background process writes structured markdown summaries continuously during sessions
- **Cross-session recall:** New sessions inject relevant past summaries with a **decay model** (older summaries weighted lower)
- **Instant compaction:** `/compact` loads the pre-written summary instead of re-analyzing conversation history
- **Bridge to permanent knowledge:** `/remember` promotes recurring patterns from session memory into `CLAUDE.md`

### 3.3 Tasks System (v2.1.16+)
Tasks represent a paradigm shift from ephemeral "to-dos" to **filesystem-backed persistent state**.

| Aspect | Old Todos | New Tasks |
|--------|-----------|-----------|
| Storage | In conversation | `~/.claude/tasks/` (JSON files) |
| Survives `/clear` | No | Yes |
| Dependencies | Flat list | DAG (blockedBy / blocks) |
| Cross-session | No | Via `CLAUDE_CODE_TASK_LIST_ID` env var |
| Crash recovery | No | Yes |

**Pattern: Plan → Clear → Execute**
1. Create task DAG when context is fresh
2. `/clear` aggressively when context fills (60-80%)
3. Tasks persist on disk; agent rehydrates plan after clear
4. Multiple sessions can share the same task list via environment variable

### 3.4 Plan Mode & Plan Persistence
- Plan mode (`/plan` or Shift+Tab in agent input) creates read-only exploration before editing
- Plans are saved to `.cursor/plans/` (Cursor) or `~/.claude/plans/` (Claude Code) as markdown files
- **GitHub issue #6207** explicitly requested plan persistence; community workarounds included `PLAN.md` files and `/memory` command
- Plans open as editable markdown files you can modify before approval

### 3.5 Context Zones & Management
| Zone | Action |
|------|--------|
| 0-50% | Work freely |
| 50-75% | Be selective |
| 75-90% | `/compact` now |
| 90%+ | `/clear` required |

### 3.6 Community Handoff Patterns
Multiple public plugins/skills extend Claude Code with structured handoffs:
- **`robynsmith/claude-code-skills-demo`**: `/handoff` and `/pickup` commands with automatic timestamps, git checks, remote backup
- **`applied-artificial-intelligence/claude-code-toolkit`**: `/transition:handoff` creates timestamped docs in `.claude/transitions/YYYY-MM-DD/HHMMSS.md`
- **`arevlo/claude-code-workflows`**: Multi-source context storage (local, Notion, GitHub Issues, docs)
- **`OthmanAdi/planning-with-files`**: Manus-style 3-file pattern (`task_plan.md`, `findings.md`, `progress.md`)

### What Claude Code Persists vs. Regenerates
| Persisted | Regenerated |
|-----------|-------------|
| Tasks (filesystem JSON DAGs) | Conversation turns (summarized, not stored verbatim) |
| Session Memory summaries (markdown) | Full file contents (re-read on demand) |
| `CLAUDE.md` / `MEMORY.md` (user-managed) | Tool outputs (re-executed if needed) |
| Plans (markdown files) | Context window state (reconstructed each session) |
| Git history | Agent reasoning trace (re-derived) |

---

## 4. CURSOR — PROJECT INDEXING & RULES-BASED CONTEXT

### Sources
- **Cursor blog:** "Agent Best Practices" (Lee Robinson, 2026-01-09)
- **Cursor docs:** `.cursor/rules/`, Plan Mode, MCP tools, Notepads
- **Community:** `grapeot/devin.cursorrules`, `blog.balakumar.dev`

### Architecture
Cursor takes a **manual-context + automatic-discovery** hybrid approach:

| Feature | Implementation |
|---------|----------------|
| **Context selection** | `@file`, `@folder`, `@codebase` (semantic search), `@docs`, `@web`, `@past chats` |
| **Rules** | `.cursor/rules/*.mdc` files with YAML frontmatter (`alwaysApply`, `intelligent`, `manual`) |
| **Notepads** | Reusable context bundles you `@mention` |
| **Plan Mode** | Shift+Tab toggles plan-before-code; plans saved to `.cursor/plans/` |
| **Project indexing** | Semantic index for `@codebase` context retrieval |
| **Context compaction** | Automatic on long sessions (can silently drop rules) |

### Plan Persistence
- Plans open as Markdown files you can edit directly
- "Save to workspace" stores plans in `.cursor/plans/` for team documentation and future agent context
- Plans include file paths and code references

### Context Handling Philosophy
- **Let the agent find context:** Cursor's agent has search tools and pulls context on demand via `grep` and semantic search
- **Manual override:** If you know the exact file, tag it; including irrelevant files confuses the agent

### What Cursor Persists vs. Regenerates
| Persisted | Regenerated |
|-----------|-------------|
| `.cursor/rules/` (project conventions) | Conversation history (compacted) |
| `.cursor/plans/` (saved plans) | File contents (re-read via search) |
| Notepads (reusable context bundles) | Semantic index (rebuilt) |
| `.cursorignore` (exclusion rules) | Tool outputs (re-executed) |

---

## 5. WINDSURF — ADAPTIVE CONTEXT & MEMORIES

### Sources
- **Awesome Agents review** (Elena Marchetti, 2026-02-27)
- **AIPromptsX comparison** (2026)
- **Codegen blog** (2026-03-26)

### Architecture
Windsurf (now Cognition/Devin) emphasizes **automatic, adaptive context** over manual management:

| Feature | Implementation |
|---------|----------------|
| **Cascade agent** | Autonomous multi-step agent with "Flow awareness" |
| **Fast Context** | RAG-based automatic context selection (~200K effective tokens) |
| **Memories** | Automatically generated/stored codebase conventions, architectural patterns, naming conventions |
| **Codemaps** | AI-generated visual maps of code structure |
| **Deep Context** | Indexes entire project; retrieves relevant snippets automatically |

### Memories System
- **Between conversations:** Cascade autonomously generates and stores "memories" about your codebase
- **On new sessions:** Memories are loaded automatically, reducing ramp-up time
- **Learning curve:** By ~48 hours of active use, Cascade consistently matches user's coding style
- **Staleness risk:** Memories can become stale or inaccurate as codebase evolves; no explicit staleness detection documented

### Parallel Agents (Wave 13)
- Up to 5 parallel Cascade agents
- Git worktree integration per agent
- Each agent maintains its own context stream

### What Windsurf Persists vs. Regenerates
| Persisted | Regenerated |
|-----------|-------------|
| Memories (auto-generated conventions) | Conversation history (not stored long-term) |
| Fast Context index (project structure) | File contents (re-read from index) |
| Plan state (during execution) | Agent reasoning trace (re-derived) |

---

## 6. DEVIN — AUTONOMOUS CONTEXT DISCOVERY & PERSISTENCE

### Sources
- **Devin docs:** `docs.devin.ai`, release notes (2024-2026)
- **Cognition blog:** September '24 product update, Devin 2.2 launch
- **Medium analysis:** "How Devin AI Actually Thinks" (Nitin Matani, 2026-04-09)
- **ContextArch.ai comparison:** Devin vs Cursor context comparison

### Architecture
Devin operates as a **fully autonomous agent** with its own context discovery and persistence model:

| Feature | Implementation |
|---------|----------------|
| **Autonomous context discovery** | Analyzes entire project structure first; reads README, docs, schema, migrations |
| **Planning** | Creates DAG-based plans (not linear lists) with dependency tracking |
| **Persistent working memory** | Maintains context across hundreds of steps in a single session |
| **Knowledge** | User-provided tips/docs/instructions persisted across all future sessions |
| **Snapshots** | "Save states" for Devin's workspace environment |
| **Playbooks** | Reusable prompt templates |
| **MultiDevin** | Manager Devin + up to 10 worker Devins (each with isolated VM) |
| **Session lifecycle** | Sessions "sleep" instead of ending; can be "woken up" |
| **Parent/Child sessions** | Grouped in sidebar; child sessions for subtasks |

### Context Discovery Tradeoffs
| Metric | Devin | Cursor |
|--------|-------|--------|
| Time to first useful output | 30-60 min | 2-5 min |
| Context completeness | Systematic, exhaustive | Human-guided, iterative |
| Computational overhead | High initial, lower ongoing | Low, distributed across interactions |

### Plan Execution Model
Devin uses a **continuous ReAct loop** (Reason + Act):
1. Reason about what to do
2. Select tool (shell, browser, editor)
3. Execute
4. Observe result
5. Re-plan if needed

Plans are **dynamic DAGs** that change during execution based on observations—this is treated as normal operation, not failure.

### What Devin Persists vs. Regenerates
| Persisted | Regenerated |
|-----------|-------------|
| Knowledge (user-provided docs/tips) | Full codebase analysis (re-run per session) |
| Snapshots (workspace save states) | Plan DAG (re-derived during execution) |
| Session history (sidebar) | Tool outputs (re-executed) |
| Playbooks (reusable prompts) | Context model (rebuilt per session) |
| Child session results | Reasoning trace (re-derived) |

---

## 7. OPEN-SOURCE SYSTEMS

### 7.1 OpenHands
**Source:** `github.com/All-Hands-AI/OpenHands` (MIT, 65K+ stars)

| Feature | Implementation |
|---------|----------------|
| **Persistence** | `ConversationState` class serializes to disk |
| **Auto-save** | Custom `__setattr__` detects field changes, serializes `base_state.json` |
| **Events** | Incremental append (`events/event-*.json`) |
| **Base state** | Agent config, execution status, statistics, secrets, agent_state |
| **Condenser** | Pluggable memory compression (summarize, keep last N, preserve task-related) |
| **Pause/Resume** | Full conversation lifecycle management |

**Directory structure:**
```
workspace/conversations/
└── <conversation-id>/
    ├── base_state.json
    └── events/
        ├── event-000001.json
        ├── event-000002.json
        └── ...
```

**What persists:** Message history, agent config, execution state, tool outputs, statistics, workspace context, activated skills, secrets.

### 7.2 OpenCode (anomalyco/opencode)
**Source:** `github.com/anomalyco/opencode`

| Feature | Implementation |
|---------|----------------|
| **Session persistence** | `--continue --session` CLI flags; sessions stored in `~/.local/share/opencode/storage/` |
| **Subagent delegation** | PR #7756: persistent sessions, task budgets, depth limits |
| **Agent Teams** | Issue #12711: flat teams with named messaging, file-based JSON storage under `.opencode/teams/` |
| **Magic Context plugin** | `cortexkit/opencode-magic-context`: background historian, cross-session memory, dreamer agent |
| **Handoff plugins** | `joshuadavidthomas/opencode-handoff`, `bristena-op/opencode-session-handoff` |

**Context construction pipeline** (documented in issue #11680):
1. Message collection + filtering
2. Last message analysis
3. Reminder injection
4. Tool resolution
5. System prompt assembly (file tree, env context)
6. Plugin transformations
7. Context overflow management (`SessionCompaction.isOverflow()`)

### 7.3 Aider
**Source:** `github.com/Aider-AI/aider` (Apache-2.0, 43K stars)

| Feature | Implementation |
|---------|----------------|
| **Architect mode** | Two-model pipeline: architect proposes changes, editor model generates file edits |
| **Repository map** | Entire codebase mapping for large project context |
| **Chat modes** | `code`, `architect`, `ask`, `help` (sticky via `/chat-mode`) |
| **Context management** | Manual (`/drop` to remove files); no automatic compaction |
| **Git integration** | Auto-commits with sensible messages |

**Limitation:** No native session persistence or handoff system. Community workarounds include `.coding-aider-plans/` directories with plan/checklist/context files.

### 7.4 Continue.dev
**Source:** `github.com/continuedev/continue`

| Feature | Implementation |
|---------|----------------|
| **Agent mode** | Chat, Plan, Agent modes with tool availability |
| **MCP tools** | External tool integration via Model Context Protocol |
| **Context providers** | `@` mentions for files, folders, docs |
| **No native persistence** | Session-scoped; relies on IDE state |

---

## 8. ENGINEERING PATTERNS FOR CONTEXT MANAGEMENT

### 8.1 The Four Context Engineering Strategies
**Source:** Eric Gerl (gerl.dev), Tian Pan (tianpan.co), Zylos Research, Agent Factory

| Strategy | Description | Failure Mode Solved |
|----------|-------------|---------------------|
| **Write** | Externalize state to files/DB instead of carrying in context | Context rot, window exhaustion |
| **Select** | Retrieve only relevant context just-in-time | Distraction, noise, stale index |
| **Compress** | Summarize or truncate accumulated history | Token cost, degradation |
| **Isolate** | Split tasks across agents with scoped windows | Context explosion, cross-contamination |

### 8.2 Tiered Context Model
**Source:** Blake Crosley (blakecrosley.com), Borghei/Claude-Skills

```
Layer 1: Working Memory (Context Window) — Active reasoning
Layer 2: Session Memory (Persistent Store) — Cross-session summaries
Layer 3: Semantic Memory (Vector DB / Structured Files) — Long-term facts
Layer 4: Archive (Cold Storage) — Historical sessions, rarely accessed
```

**Memory Promotion Protocol:** Knowledge flows upward based on recurrence and value. Session facts → persistent memory → canonical facts.

### 8.3 Context Budget Allocation
**Source:** Blake Crosley, Borghei/Claude-Skills

| Segment | Budget % | Purpose | Priority |
|---------|----------|---------|----------|
| System Instructions | 5-10% | Agent identity, rules, constraints | Fixed |
| Task Context | 20-30% | Current task, requirements | High |
| Conversation History | 10-20% | Prior turns, decisions | Sliding window |
| Retrieved Context | 30-40% | Relevant code, docs | Dynamic |
| Working Buffer | 10-15% | Tool outputs, scratch space | Ephemeral |

### 8.4 Staleness Detection
**Source:** Borghei/Claude-Skills, ContextArch.ai

```
Freshness Score = f(last_verified, change_frequency, confidence)

Fresh (< 7 days, file unchanged):     Use directly
Aging (7-30 days, file changed):      Re-verify before using
Stale (> 30 days):                    Flag, re-retrieve, or discard
Unknown (never verified):             Treat as low-confidence
```

**Rule:** Stale context is worse than no context. Missing context produces a question; stale context produces a confident wrong answer.

### 8.5 Handoff Protocol (Five Layers)
**Source:** DEV Community persistence patterns, Zylos Research

An effective handoff includes:
1. **State snapshot** — typed, validated current values
2. **Narrative context** — 3-5 sentences explaining why state looks as it does
3. **Decision log** — what was chosen and why (prevents re-litigation)
4. **Blocker & risk register** — what's blocking, what to watch
5. **Next-action queue** — ordered list of immediate next steps

### 8.6 Session Lifecycle Pattern: Plan → Execute → Reset
**Source:** Harness.io blog, Agent Factory

```
Plan → Execute → Reset (with handoff) → Plan → ...
```

- **Plan:** Break down task, identify dependencies, surface uncertainties
- **Execute:** Incremental steps, catch errors early
- **Reset:** Fresh session restores clarity, removes noise, re-establishes prioritization
- **Handoff:** Structured document bridges the gap

### 8.7 Progressive Context Strategy (Three Levels)
**Source:** BSWEN blog (docs.bswen.com)

| Level | Stack | Use Case |
|-------|-------|----------|
| **Level 1: Minimal** | `CLAUDE.md` only | Single session, bug fixes, quick prototypes |
| **Level 2: Handoff** | `CLAUDE.md` + `plan.md` + `todo.md` + `session_handoff.md` | Multi-session features, ongoing maintenance |
| **Level 3: Automated** | Level 2 + hooks (auto-load, capture decisions) + MCP servers | Long projects, team workflows |

### 8.8 Three Persistence Layers
**Source:** DEV Community (persistence patterns)

| Layer | Volatility | Storage Pattern | Example |
|-------|-----------|-----------------|---------|
| **Working State** | Volatile | Overwritten each session | Current task, active context, runtime flags |
| **Short-term Memory** | Medium | Timestamped, append-only | Session logs, handoffs, checkpoints |
| **Identity/Config** | Slow-changing | Rarely updated | Core parameters, behavioral policies, long-term goals |

---

## 9. COMPARATIVE MATRIX: WHAT IS PERSISTED VS. REGENERATED

| System | Persisted State | Regenerated State | Storage Medium |
|--------|----------------|-------------------|----------------|
| **TD / Sidecar** | Task DAG, handoffs (done/remaining/decisions), session IDs, activity logs | Conversation history, file contents, tool outputs | SQLite + files |
| **Claude Code** | Tasks (JSON DAG), Session Memory summaries, `CLAUDE.md`, plans (markdown), git history | Conversation turns (summarized), file contents (re-read), tool outputs | Filesystem (JSON + markdown) |
| **Cursor** | `.cursor/rules/`, `.cursor/plans/`, Notepads, project index | Conversation history (compacted), file contents (searched) | Filesystem + semantic index |
| **Windsurf** | Memories (conventions), Fast Context index, plan state | Conversation history, file contents (indexed retrieval) | Proprietary (cloud) |
| **Devin** | Knowledge (user docs), Snapshots, Playbooks, session history, child session results | Full codebase analysis, plan DAG (dynamic), tool outputs, reasoning trace | Proprietary (cloud) |
| **OpenHands** | ConversationState (base_state.json), events (incremental JSON), agent config, secrets | Context window state (reconstructed), tool outputs (re-executed) | Filesystem (JSON) |
| **OpenCode** | Session messages (JSON), team state (JSON), subagent budgets | Context assembly (rebuilt per turn), system prompt (regenerated) | Filesystem (JSON) |
| **Aider** | Git history, repository map | Chat history, file contents (added manually), tool outputs | Git + in-memory |

---

## 10. KEY ENGINEERING PRINCIPLES (CROSS-CUTTING)

1. **Filesystem > Conversation:** Durable state belongs on disk, not in the context window. TD, Claude Code Tasks, and OpenHands all use filesystem/SQLite persistence.

2. **DAGs > Lists:** Dependency tracking prevents hallucinated progress. Claude Code Tasks, Devin plans, and TD epics all use directed acyclic graphs.

3. **Structured Handoffs > Raw History:** The next session needs decisions and state, not a transcript. TD's `handoff` command and community Claude Code plugins enforce this.

4. **Decay Models > Static Recall:** Older context should be weighted lower or summarized. Claude Code Session Memory uses decay; Windsurf Memories lack explicit staleness detection.

5. **Session Isolation:** The session that writes code cannot approve its own work. TD enforces this; Claude Code community plugins emulate it.

6. **Context Budget Discipline:** Treat the context window as a scarce resource with explicit allocation. Blake Crosley's seven-layer system allocates percentages.

7. **Plan on Disk Enables Context Freedom:** When plans live in files, you can `/clear` aggressively without losing roadmap. This is the core insight of Claude Code Tasks.

8. **Passive Context Beats Active Tools:** Information injected at session start is consumed 100% of the time; MCP tools are invoked only when the agent remembers to call them (~44% usage rate observed).

---

## 11. RECOMMENDED SOURCES FOR DEEP DIVES

| Topic | Source | URL |
|-------|--------|-----|
| TD docs | Sidecar / Haplab | `sidecar.haplab.com/docs/td` |
| Claude Code Tasks | VentureBeat | `venturebeat.com/orchestration/claude-codes-tasks-update` |
| Context Engineering | Blake Crosley | `blakecrosley.com/blog/context-is-architecture` |
| Claude Code filesystem | Diljit PR | `diljitpr.net/blog-post-2026-02-24-inside-dot-claude-filesystem-architecture.html` |
| Session Memory | ClaudeFast | `claudefa.st/blog/guide/mechanics/session-memory` |
| Devin context | ContextArch.ai | `contextarch.ai/blog/ai-coding-agents-devin-vs-cursor-context-comparison` |
| OpenHands persistence | OpenHands docs | `docs.openhands.dev/sdk/guides/convo-persistence` |
| OpenCode subagents | GitHub PR #7756 | `github.com/anomalyco/opencode/pull/7756` |
| Cursor best practices | Cursor blog | `cursor.com/blog/agent-best-practices` |
| Windsurf review | Awesome Agents | `awesomeagents.ai/reviews/review-windsurf` |
| Context engineering patterns | Eric Gerl | `gerl.dev/blog/context-engineering-ai-performance` |
| Progress files architecture | Agent Factory | `agentfactory.panaversity.org/docs/General-Agents-Foundations/context-engineering/progress-files` |
| Handoff/pickup system | Robyn Smith | `github.com/robynsmith/claude-code-skills-demo` |
| Planning with files | Othman Adi | `github.com/OthmanAdi/planning-with-files` |

---

*End of Memo*
