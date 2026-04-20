# MEMO: AI Productivity Workbenches — Context Organization Patterns for Terminal-First Developer Overviews

**Date:** 2026-04-17  
**Scope:** Publicly documented features of mature AI developer workbenches. No speculation on unreleased functionality.  
**Focus:** How products organize *tasks, context, agent state, code/workspace state,* and *next actions* on a human-facing overview/dashboard/home surface.

---

## 1. Executive Summary

Across the market, AI workbenches are converging on a small set of information-architecture patterns for their "home" surfaces:

1. **Session/Task as the primary object** — not files, not chats. Humans scan a list of active or recent tasks first.
2. **Status as a first-class visual primitive** — running, paused, ready-for-review, failed. Color + iconography + live indicators are universal.
3. **Context scoping via containment** — projects, workspaces, or worktrees isolate context and prevent cross-contamination.
4. **Split-pane or pane-based layout** — the home surface is rarely a single chat; it pairs an activity list with a preview/terminal/diff pane.
5. **Action prompts embedded in empty or low-density states** — "What do you want to build?" as the zero-state CTA.
6. **Explicit human steering points** — plan approval, checkpoint review, inline comments on diffs. The human is presented with decision moments, not just output.

The sections below map these patterns to concrete products with source citations.

---

## 2. Product Precedents by Category

### 2.1 Chat/Project Workspaces — OpenAI & Anthropic

#### OpenAI ChatGPT Projects
- **Source:** [OpenAI Academy — Using projects in ChatGPT](https://openai.com/academy/projects/), [OpenAI Help Center — Projects in ChatGPT](https://help.openai.com/en/articles/10169521-using-projects-in-chatgpt)
- **Home surface:** Left-hand sidebar lists Projects; each Project is a dedicated space grouping chats, files, and custom instructions.
- **Context organization:** Files and instructions live in the project knowledge base. "Project-only memory" isolates context so chats reference only material inside the project.
- **Task presentation:** Chats are the task units. No explicit status beyond conversation history.
- **Next actions:** Suggested entry points: ongoing research, writing/editing, planning. Recent update adds "shared projects" with real-time collaboration.
- **Pattern:** **Containment-first.** The home view is a file-tree-like list of projects; drilling in reveals chats and uploaded files.

#### Anthropic Claude Console / Workspaces / Projects
- **Source:** [Claude Platform — Workspaces docs](https://console.anthropic.com/docs/en/build-with-claude/workspaces), [Anthropic blog — Workspaces in the API Console](https://www.anthropic.com/news/workspaces), [Claude Help Center — What are projects?](https://support.anthropic.com/en/articles/9517075-what-are-projects)
- **Console Workspaces:** Top-left workspace selector switches environments (dev/staging/prod). Each workspace has its own API keys, rate limits, spend limits, and member permissions.
- **Claude.ai Projects:** Self-contained workspaces with chat histories and knowledge bases. Project instructions and RAG-enhanced knowledge sit on the right side of the project main page.
- **Workbench:** A prompt-testing surface inside the Console with model/temperature controls and saved prompt lists.
- **Pattern:** **Environment separation + scoping controls.** The home surface is a settings-heavy workspace list; project view splits conversation (center) from knowledge base (right).

#### Claude Cowork / Claude Code Desktop
- **Source:** [Claude Code Desktop docs](https://code.claude.com/docs/en/desktop.md)
- **Home surface:** Sidebar of sessions, filterable by status, project, or environment (Local / Remote / Cowork). Grouping by project is supported.
- **Task/agent state:** Each session tracks its own context and changes independently. Live status indicators in the sidebar.
- **Context organization:** Drag-and-drop pane layout: chat, diff, preview, terminal, file, plan, tasks, subagent. Terminal shares the session environment.
- **Next actions:** Usage ring next to the model picker shows context-window usage and plan usage, prompting `/compact` or context management.
- **Pattern:** **Pane-based session dashboard.** The human arranges the workspace; agent output is rendered in specialized panes rather than a chat stream.

---

### 2.2 IDE-Centric Agent Workbenches — Cursor & Windsurf

#### Cursor 3 (Agents Window)
- **Source:** [Cursor blog — Meet the new Cursor](https://cursor.com/en/blog/cursor-3), [Cursor Changelog — New Cursor Interface](https://cursor.com/changelog/04-02-26), [Digital Applied — Cursor 3 Guide](https://www.digitalapplied.com/blog/cursor-3-agents-window-design-mode-complete-guide)
- **Home surface shift:** Cursor 3 rebuilt the IDE around the **Agents Window** — a full-screen workspace for running and monitoring multiple agents. The traditional editor is available but no longer the center of gravity.
- **Task presentation:** **Agent Tabs** show independent agent sessions, arrangeable side-by-side or in a grid. Each tab has its own context, model selection, and execution environment (local, git worktree, cloud, remote SSH).
- **Agent state:** Progress shown inside each tab with live output, tool calls, and file changes. Cloud agents produce demos/screenshots for verification.
- **Context organization:** `@` mentions for files/definitions; context tags appear as pills at the top of the chat. Composer 2 auto-gathers recommended context.
- **Next actions:** `/worktree` isolates experimental changes; `/best-of-n` runs the same task across multiple models in parallel for comparison.
- **Pattern:** **Agent-first tab grid.** The overview is a fleet of concurrent tasks, not a single conversation. Design Mode adds a browser annotation layer for visual feedback.

#### Windsurf (Cascade)
- **Source:** [Windsurf docs — Cascade](https://docs.windsurf.com/windsurf/cascade), [Windsurf docs — Workspace](https://docs.repl.it/core-concepts/workspace), [Markaicode — Cascade Agent Guide](https://markaicode.com/windsurf-cascade-agent-autonomous-refactoring/)
- **Home surface:** The **Workspace** is the home base. A prompt box sits at the bottom; the main area shows the editor + live preview + Cascade panel (right side by default).
- **Task presentation:** Cascade creates an internal **Todo list** within the conversation to track progress on complex tasks. **Simultaneous Cascades** allow multiple agents; a dropdown in the top-left of the Cascade panel switches between them.
- **Agent state:** **Action Log panel** (`View → Cascade Actions`) shows each step: `[READ]`, `[EDIT]`, `[SEARCH]`, `[TERMINAL]`. Pause button for human intervention.
- **Context organization:** Multi-layer context: global rules → project rules (`.windsurfrules`) → memories → workspace index → active editor state → flow context (recent edits, terminal output, navigation). `contextPaths` in `.windsurf/cascade.config` scoping narrows the file tree.
- **Next actions:** Problems panel has a **Send to Cascade** button that @-mentions the error directly into the agent panel.
- **Pattern:** **Flow-aware sidebar + scoped config files.** The workspace remembers your real-time actions; the agent panel is both chat and action log.

---

### 2.3 Cloud Agent Workbenches — GitHub Copilot, Devin, Replit

#### GitHub Copilot Workspace (sunset) & Copilot Coding Agent / Agents Panel
- **Source:** [GitHub Blog — Copilot Workspace](https://github.blog/news-insights/product-news/github-copilot-workspace/), [GitHub Next — Copilot Workspace](https://next.github.com/projects/copilot-workspace/), [GitHub Blog — Agents panel](https://github.blog/news-insights/product-news/agents-panel-launch-copilot-coding-agent-tasks-anywhere-on-github/), [GitHub Docs — Managing cloud agents](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/manage-agents)
- **Copilot Workspace (historical):** Task-oriented dev environment starting from a GitHub Issue or repo. A homepage dashboard listed recently assigned issues and quota counters. The workflow was: specification → editable plan → implementation → validation → PR. Everything was editable in-place; integrated terminal and port forwarding for verification.
- **Agents Panel / Mission Control (current):** A lightweight overlay on every github.com page. Humans can assign background tasks, monitor running tasks with real-time status, and jump into resulting PRs. Full-screen "Mission Control" centralizes assignment, oversight, and review across repos.
- **Task presentation:** Task list with status badges. Session logs show the agent's thought process and tool usage.
- **Agent state:** Live status updates in the session log. Steering input allowed mid-run without stopping.
- **Pattern:** **Issue-native task list + session log as reasoning artifact.** The home surface is a filtered queue of tasks attached to repo context; humans review logs before reviewing code.

#### Devin (Cognition)
- **Source:** [Devin Docs — Session Tools](https://docs.devin.ai/work-with-devin/devin-session-tools), [Cognition blog — Devin 2.0](https://www.cognition.ai/blog/devin-2), [Cognition blog — Devin can now Manage Devins](https://cognition.ai/blog/devin-can-now-manage-devins)
- **Home surface:** **Progress tab** brings Shell, IDE, and Browser into one unified view. All shell commands, code edits, and browser activity are logged in one timeline.
- **Task presentation:** Sessions are the primary object. Sidebar shows active and archived sessions. Scheduled sessions get a visual pill indicator.
- **Agent state:** Real-time IDE view (VSCode embedded) lets you watch Devin edit live. Interactive Browser for manual takeover on auth/CAPTCHA. Machine utilization (CPU/RAM) appears in the top-right of the session page.
- **Context organization:** **Devin's Workspace** is a saved machine state that resets at the start of every session. **Repo Knowledge** auto-generates and auto-updates notes on repo structure. **Pinned Knowledge** for facts Devin should always remember.
- **Managed Devins:** Parent Devin acts as coordinator; child Devins each get isolated VMs with their own terminals, browsers, and session links.
- **Pattern:** **Unified progress timeline + embedded IDE takeover.** The human monitors a single chronological feed and can drop into any of three tools (shell, IDE, browser) without leaving the webapp.

#### Replit Agent 4
- **Source:** [Replit Docs — Workspace](https://docs.repl.it/core-concepts/workspace), [Replit Docs — Task System](https://docs.repl.it/core-concepts/agent/task-system), [Replit Agent 4 landing page](https://replit.com/agent), [GLN-7.5 — Replit Agent 4 Features](https://gln75.com/en/blog/replit-agent-hands-four-new-features-explained)
- **Home surface:** The **Workspace** is the home base — prompt box at the bottom, editor + live preview + conversation on the side.
- **Task presentation:** **Task board** (Kanban: Drafts, Active, Ready, Done). Each task runs in its own thread. Up to 10 concurrent background tasks on Pro. Dependencies auto-detected.
- **Agent state:** Tasks show live status icons. Thread view shows per-task conversation and status. Board view shows fleet state at a glance.
- **Context organization:** **Design Canvas** — an infinite zoomable board for mockups, app previews, and user flows. Existing pages can be pulled onto the canvas as copies without touching the live app.
- **Next actions:** Plan mode lets humans brainstorm and map tasks before any code changes. Once happy, "Make this real" converts a design frame into a working app.
- **Pattern:** **Kanban task board + infinite canvas.** The home surface blends project management (task columns) with creative exploration (canvas frames).

---

### 2.4 Productivity & Orchestration — Linear & Raycast

#### Linear
- **Source:** [Linear Changelog — Linear Agent](https://linear.app/changelog/2026-03-24-introducing-linear-agent), [Linear Docs — Linear Agent](https://linear.app/docs/linear-agent), [Linear — AI workflows](https://linear.app/ai)
- **Home surface:** Highly customizable bottom toolbar + Inbox + command menu. Default home view can be set to Linear Agent, Inbox, or custom views.
- **Task presentation:** Issues are the atomic unit. Triage Intelligence auto-suggests assignees, labels, and related issues. AI Agents can be assigned to issues; they appear as delegates in the issue list.
- **Agent state:** Agent sessions are visible on issue pages and in custom views filtered by Delegate. Session state updates automatically based on emitted activities.
- **Context organization:** Linear Agent chat (shortcut `Cmd/Ctrl + J`) is grounded in workspace data: roadmap, issues, projects, documents, customer requests. Skills save reusable conversation workflows.
- **Next actions:** One-click launchers open any issue directly in Claude Code, Cursor, Codex CLI, Devin, Warp, etc., preloaded with context.
- **Pattern:** **Issue-centric command center.** The overview is a filtered, sortable stream of work items with AI status woven into the same list.

#### Raycast
- **Source:** [Raycast Manual — AI Extensions](https://manual.raycast.com/ai-extensions), [Raycast API — AI Extensions](https://developers.raycast.com/ai/learn-core-concepts-of-ai-extensions), [Raycast API — Create an AI Extension](https://developers.raycast.com/ai/create-an-ai-extension)
- **Home surface:** Root search + AI Chat + AI Commands. The launcher is the universal entry point.
- **Task presentation:** No persistent task board; instead, **conversations** and **commands** are the units. AI Extensions add tools that the AI can call via `@mention`.
- **Agent state:** Tool calls are shown inline in the chat as execution steps. No long-running session dashboard.
- **Context organization:** Custom Instructions per extension. AI Chat Presets bundle context + extensions + prompts. Clipboard, window, and file selections can be fed into Quick AI.
- **Pattern:** **Launcher-as-workbench.** The home surface is a command palette that doubles as an agent orchestrator. Best for micro-tasks; weakest for monitoring long-running agent fleets.

---

### 2.5 Terminal-Native Tools — Claude Code Ecosystem, Warp, Community Dashboards

#### Claude Code (Terminal + Desktop + Community Dashboards)
- **Source:** [Claude Code Desktop docs](https://code.claude.com/docs/en/desktop.md), [Claude Lab — Session Management Guide](https://claudelab.net/en/articles/claude-code/claude-code-session-management-resume-guide), [Pooya Golchian — Claude Code Workspace Structure](https://pooya.blog/blog/claude-code-workspace-folder-structure-2026/)
- **Home surface (CLI):** `claude --list` to see sessions; `claude --resume <name>` to continue. Session data stored locally in `~/.claude/sessions/`.
- **Home surface (Desktop):** Sidebar of sessions with filters and grouping by project. Drag-and-drop panes for chat, diff, terminal, preview.
- **Context organization:** `.claude/` folder in project root maintains a semantic index, conversation threads, context snapshots, and cache. `CLAUDE.md` (project) and `~/.claude/CLAUDE.md` (global) inject persistent rules.
- **Community dashboards:**
  - **ccboard** ([Docs.rs](https://docs.rs/ccboard)) — Rust TUI/Web dashboard with 13 tabs: Dashboard (KPIs + forecast), Sessions (3-pane + live status icons), Analytics (8 sub-views), Brain (cross-session knowledge base), MCP Servers, Hooks, Agents, Commands.
  - **Canopy** ([GitHub](https://github.com/The-Banana-Standard/canopy)) — Tauri desktop workspace manager and terminal multiplexer for Claude Code. Workspace cards, session history, daily planner, GitHub dashboard.
  - **ELVES** ([GitHub](https://github.com/mvmcode/elves)) — Tauri desktop app for orchestrating agent teams in isolated git worktrees. Workspaces view, split terminal pane, Insights dashboard with 9 KPIs + timeline + analysis.
- **Pattern:** **Session-list + pane layout + local-first context store.** The terminal is the engine; the dashboard is an optional visual layer on top of session metadata.

#### Warp Terminal (Oz Agent)
- **Source:** [Warp docs — Terminal and Agent Modes](https://docs.warp.dev/agent-platform/warps-agent/interacting-with-agents/terminal-and-agent-modes), [Warp docs — Blocks as Context](https://docs.warp.dev/agent-platform/warps-agent/agent-context/blocks-as-context), [Warp — Agent Mode](https://www.warp.dev/ai), [Warp — Agents](https://www.warp.dev/agents)
- **Home surface:** Block-based terminal by default. Agent conversations open in a dedicated **Agent Modality** view. Conversation Panel browses and manages agent conversations.
- **Task presentation:** Terminal blocks are atomic command+output units. Agent conversation blocks are scoped to the conversation and do not clutter the terminal blocklist.
- **Agent state:** Agent Management Panel lets you run multiple agents at once and track them centrally. Status indicators on conversations.
- **Context organization:** **Blocks as Context** — terminal output blocks can be attached to agent queries (`CMD-UP` on macOS). Commands run inside an agent conversation are automatically included as context for the next query.
- **Knowledge store:** Warp Drive saves notebooks, workflows, and decisions for agent accuracy and team velocity.
- **Pattern:** **Block-atomic terminal + modality switch.** The default view is a clean terminal; agent work lives in a distinct conversation pane with explicit context attachment mechanics.

---

## 3. Distilled UI / Information Architecture Patterns

### 3.1 Tasks & Next Actions

| Pattern | Products | Description |
|---------|----------|-------------|
| **Task Board / Kanban** | Replit, Linear | Columns (Drafts → Active → Ready → Done) give humans a project-management mental model for agent work. |
| **Session List + Status Icons** | Cursor, Devin, Claude Code Desktop, ccboard | A sidebar of named sessions with live indicators (● running, ◐ paused, ✓ done). |
| **Issue-Native Queue** | GitHub Copilot Agents Panel, Linear | Tasks are born from issues/PRs; the dashboard shows assigned items ready for agent delegation. |
| **Todo List Inside Chat** | Windsurf Cascade | Agent maintains an internal checklist of steps visible in the conversation pane. |
| **Embedded Action Prompts** | Replit Workspace, Claude Code Desktop | Empty-state prompt box: "Describe what you want to build." Reduces decision paralysis. |

### 3.2 Context Presentation

| Pattern | Products | Description |
|---------|----------|-------------|
| **Project/Workspace Containment** | OpenAI Projects, Anthropic Workspaces, Claude Projects | Files, instructions, and memory are scoped to a container. Cross-contamination is opt-in or impossible. |
| **Context Pills / Tags** | Cursor Composer, Windsurf | Active files, @mentions, and rules appear as removable chips at the top of the chat/agent panel. |
| **Knowledge Base Sidebar** | Claude Projects (right rail), Devin Pinned Knowledge | Persistent docs/instructions visible alongside the main workspace. |
| **Canvas / Infinite Board** | Replit Design Canvas, Windsurf (acquired Devin team) | Visual context (mockups, flows, app previews) is laid out spatially rather than linearly. |
| **Block Attachment** | Warp | Terminal output blocks are explicitly attached to agent queries, making context lineage visible. |

### 3.3 Agent State & Orchestration

| Pattern | Products | Description |
|---------|----------|-------------|
| **Agent Tabs / Grid** | Cursor 3 Agents Window | Multiple agents shown side-by-side or in a grid, each with independent context and model. |
| **Progress Timeline / Unified Log** | Devin Progress tab, Windsurf Action Log | Chronological feed of all agent actions (shell, file, browser) in one scannable stream. |
| **Mission Control / Fleet View** | GitHub Copilot Agents Panel, Warp Agent Management Panel | Lightweight overlay or full-screen page listing all running/completed agent tasks across repos. |
| **Parent-Child Agent Orchestration** | Devin Managed Devins | Coordinator agent delegates to isolated child agents; each child gets its own VM and session link. |
| **Plan → Execute Checkpoint** | GitHub Copilot Workspace, Replit Plan Mode, Devin Agency toggle | Human must approve or edit a plan before the agent writes code. Plan is a first-class editable artifact. |

### 3.4 Workspace / Code State

| Pattern | Products | Description |
|---------|----------|-------------|
| **Pane-Based Layout** | Cursor 3, Claude Code Desktop, Devin | Chat, diff, terminal, preview, and file editor are draggable panes. Humans arrange density. |
| **Live Preview Embedded** | Replit, Cursor Design Mode, Devin, Claude Code Desktop | Running app or browser preview sits next to the code/agent panel. |
| **Git Worktree Isolation** | Cursor `/worktree`, ELVES, Claude Code Desktop | Every task/session can run in an isolated branch/directory, preventing file conflicts. |
| **Semantic Codebase Index** | Cursor, Windsurf, Claude Code `.claude/context/` | Background indexing of file structure, signatures, and patterns for fast context retrieval. |
| **Usage / Budget Gauge** | Claude Code Desktop (usage ring), ccboard Dashboard, OpenAI/Anthropic Console | Visual indicator of context-window fill or API spend, prompting compaction or review. |

### 3.5 Dashboard / Home Surface Patterns (Summary Matrix)

| Product | Primary Home Object | Layout Paradigm | Human Steering Moment | Terminal Integration |
|---------|---------------------|-----------------|----------------------|----------------------|
| **OpenAI ChatGPT Projects** | Project → Chat list | Sidebar + chat thread | Edit instructions / upload files | None |
| **Anthropic Claude Console** | Workspace → API keys/usage | Settings tabs + Workbench | Rate/spend limit sliders | None |
| **Claude Code Desktop** | Session sidebar + panes | Drag-and-drop panes | Plan approval, diff review, `/compact` | Embedded terminal |
| **Cursor 3** | Agent Tabs (fleet) | Grid / side-by-side tabs | Checkpoint checkout, `/best-of-n` compare | Built-in IDE terminal |
| **Windsurf** | Workspace + Cascade panel | Editor + right-side agent panel | Pause button, Action Log review | Dedicated zsh shell |
| **GitHub Copilot Agents Panel** | Task list (issue-native) | Overlay / Mission Control full-screen | Mid-run steering input | Cloud agent terminal logs |
| **Devin** | Session + Progress timeline | Unified timeline + embedded IDE/Browser | Stop / take over IDE or shell | Full shell + IDE |
| **Replit Agent 4** | Task board + Canvas | Kanban + infinite design board | Plan mode, task approval | Built-in cloud IDE |
| **Linear** | Issue list / Inbox | Stream + command palette | Triage suggestions, agent assignment | One-click launch to CLI tools |
| **Raycast** | Command / Chat | Launcher + chat pane | Tool-call approval inline | Can launch terminal commands |
| **Warp** | Terminal blocks + Agent conversations | Block list + conversation pane | Block attachment, command approval | Native terminal |
| **ccboard / Canopy / ELVES** | Session dashboard | TUI/Desktop tabs + terminal multiplexer | Session resume, hook review | Central |

---

## 4. Implications for Focus-tui

### 4.1 What "Terminal-First Developer Overview" Means in Precedent Terms

None of the major commercial products are *pure* terminal dashboards, but the **community tools around Claude Code** (ccboard, Canopy, ELVES) explicitly fill this gap. They prove that developers want:

1. **A TUI or lightweight desktop shell** that wraps terminal sessions with metadata (status, cost, context usage).
2. **Session as the primary navigable object** — not files, not repos.
3. **Live status indicators** in a dense list (running, paused, done, failed).
4. **Quick resume / context injection** without re-explaining the project.
5. **Worktree or project isolation** visible in the overview.

### 4.2 Design Patterns to Borrow or Avoid

| Borrow | Avoid |
|--------|-------|
| **Session list with status icons** (Cursor, Devin, ccboard) | Generic chat-thread as the only home view |
| **Explicit context scoping** (workspaces, worktrees, projects) | Implicit, unbounded context that bleeds across tasks |
| **Pane or tab layout** for chat + terminal + diff (Claude Desktop, Devin) | Forcing everything into a single scrollback stream |
| **Task board or todo list** for multi-step work (Replit, Windsurf) | Assuming the human remembers every sub-goal |
| **Usage/budget gauge** prompting compaction (Claude Desktop, ccboard) | Hiding context-window or cost constraints until failure |
| **One-click "open in terminal tool"** launchers (Linear) | Locking the user into a single agent runtime |

### 4.3 Unique Positioning Opportunities

- **Density-first TUI:** Most commercial products (Cursor, Replit, Devin) use GUI layouts with generous whitespace. A terminal-native overview can surface more sessions, more metrics, and more context in the same screen real estate.
- **Local-first session metadata:** Like ccboard, Focus-tui can read local agent directories (`~/.claude`, `.focus/`) to build a dashboard without cloud APIs.
- **Unified fleet view across runtimes:** Unlike Warp or Cursor, which center their own agent, Focus-tui could act as a neutral overview for Claude Code, Codex, OpenCode, or custom agents running in local PTYs.
- **Git-native isolation:** Following ELVES and Cursor `/worktree`, the home surface can visually map each session to a branch/worktree, making parallel work explicit and conflict-free.

---

## 5. Sources Index

| Source | URL |
|--------|-----|
| OpenAI Academy — Using projects in ChatGPT | https://openai.com/academy/projects/ |
| OpenAI Help Center — Projects in ChatGPT | https://help.openai.com/en/articles/10169521-using-projects-in-chatgpt |
| Anthropic — Workspaces docs | https://console.anthropic.com/docs/en/build-with-claude/workspaces |
| Anthropic blog — Workspaces in the API Console | https://www.anthropic.com/news/workspaces |
| Claude Help Center — What are projects? | https://support.anthropic.com/en/articles/9517075-what-are-projects |
| Claude Code Desktop docs | https://code.claude.com/docs/en/desktop.md |
| Cursor blog — Meet the new Cursor | https://cursor.com/en/blog/cursor-3 |
| Cursor Changelog — New Cursor Interface | https://cursor.com/changelog/04-02-26 |
| Digital Applied — Cursor 3 Guide | https://www.digitalapplied.com/blog/cursor-3-agents-window-design-mode-complete-guide |
| Windsurf docs — Cascade | https://docs.windsurf.com/windsurf/cascade |
| Windsurf docs — Workspace | https://docs.repl.it/core-concepts/workspace |
| Markaicode — Cascade Agent Guide | https://markaicode.com/windsurf-cascade-agent-autonomous-refactoring/ |
| GitHub Blog — Copilot Workspace | https://github.blog/news-insights/product-news/github-copilot-workspace/ |
| GitHub Next — Copilot Workspace | https://next.github.com/projects/copilot-workspace/ |
| GitHub Blog — Agents panel | https://github.blog/news-insights/product-news/agents-panel-launch-copilot-coding-agent-tasks-anywhere-on-github/ |
| GitHub Docs — Managing cloud agents | https://docs.github.com/en/copilot/how-tos/use-copilot-agents/manage-agents |
| Devin Docs — Session Tools | https://docs.devin.ai/work-with-devin/devin-session-tools |
| Cognition blog — Devin 2.0 | https://www.cognition.ai/blog/devin-2 |
| Cognition blog — Devin can now Manage Devins | https://cognition.ai/blog/devin-can-now-manage-devins |
| Replit Docs — Workspace | https://docs.repl.it/core-concepts/workspace |
| Replit Docs — Task System | https://docs.repl.it/core-concepts/agent/task-system |
| Replit Agent 4 landing page | https://replit.com/agent |
| Linear Changelog — Linear Agent | https://linear.app/changelog/2026-03-24-introducing-linear-agent |
| Linear Docs — Linear Agent | https://linear.app/docs/linear-agent |
| Linear — AI workflows | https://linear.app/ai |
| Raycast Manual — AI Extensions | https://manual.raycast.com/ai-extensions |
| Raycast API — AI Extensions | https://developers.raycast.com/ai/learn-core-concepts-of-ai-extensions |
| Claude Lab — Session Management Guide | https://claudelab.net/en/articles/claude-code/claude-code-session-management-resume-guide |
| Pooya Golchian — Claude Code Workspace Structure | https://pooya.blog/blog/claude-code-workspace-folder-structure-2026/ |
| ccboard (Rust TUI/Web dashboard) | https://docs.rs/ccboard |
| Canopy (Claude Code workspace manager) | https://github.com/The-Banana-Standard/canopy |
| ELVES (agent orchestration desktop app) | https://github.com/mvmcode/elves |
| Warp docs — Terminal and Agent Modes | https://docs.warp.dev/agent-platform/warps-agent/interacting-with-agents/terminal-and-agent-modes |
| Warp docs — Blocks as Context | https://docs.warp.dev/agent-platform/warps-agent/agent-context/blocks-as-context |
| Warp — Agent Mode | https://www.warp.dev/ai |
| Warp — Agents | https://www.warp.dev/agents |

---

**End of Memo**
