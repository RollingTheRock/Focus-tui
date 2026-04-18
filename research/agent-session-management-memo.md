# MEMO: Agent/Session Management Surfaces in AI Coding Workbenches
## Research for Focus-tui Agent Pane Guidance

**Date:** 2026-04-17
**Sources:** Public docs, GitHub repos, product blogs, release notes (cited inline)
**Scope:** Focus on what a human sees and acts on — not chat-only UIs, not made-up behavior.

---

## 1. DEVIN (Cognition AI)

### What a human sees
- **Sidebar session list** with inline rename, optimistic updates, and nested parent/child grouping. Child sessions stay visually indented under their parent regardless of time.
- **Session Manager** (dedicated page) for filtering by PR status, users, playbooks, tags, and time range. Supports custom tags.
- **Progress Tab** inside a session: unified view of shell commands, code edits, and browser activity logged in one timeline. Click any step to see details.
- **Session Insights UI** (redesigned) — on-demand analysis button generates a timeline of what happened, actionable feedback, and an improved prompt for reuse.
- **Status dot in browser tab favicon** (green = running, etc.).

### Grouping model
- Sessions are the top-level unit. Parent/child session grouping is visual (sidebar indentation).
- Batch sessions appear indented under their parent.
- Sessions can be tagged and filtered by category/subcategory.
- Playbooks and schedules are first-class organizing concepts.

### Visible state model
- Session status (running, waiting, completed, failed, archived).
- Progress steps within a session (clickable, with tool-level detail).
- Confidence scores (🟢🟡🔴) on tasks.
- Consumption analytics (ACU usage, line items for sessions, indexing, Devin Review).
- Org-level metrics API.

### Interaction model
- **List-like at the top level** (sidebar session list, Session Manager table).
- **Timeline-like inside a session** (Progress Tab shows chronological steps).
- **No kanban/board view** for tasks. The closest to board-like is the categorization/filtering in Session Manager.
- Human acts by: starting sessions, tagging, filtering, clicking into progress steps, taking over IDE/shell/browser, generating insights on demand.

**Sources:**
- docs.devin.ai (release notes 2025–2026, "Your First Session", "Session Tools", "Advanced Capabilities")
- cognition.ai/blog (Dec '24 product update)

---

## 2. CURSOR (Anysphere)

### What a human sees
- **Agents Window** (Cursor 3, full-screen workspace) — replaces the old Composer pane. Shows all local and cloud agents in a sidebar. Agent Tabs can be arranged side-by-side, stacked, or in a grid.
- **Agent status dashboard** (Cursor 2.0+) — progress, context pills, approvals in real time.
- **Diffs view** for editing and reviewing changes across files.
- **Cloud agent handoff** — move an agent session from local to cloud (keep running while offline) or cloud to local (edit and test on desktop). Cloud agents produce demos and screenshots.
- **Design Mode** — live browser preview inside the Agents Window; click UI elements to send context to the agent.
- **Task queue / drag-and-drop** (Cursor 2.0) — tasks can be assigned to agents.

### Grouping model
- **Agent Tabs** are the primary grouping unit. Each tab = one independent agent session with its own context, model, and environment.
- Tabs can be grouped visually (side-by-side, grid, stack).
- Background/cloud agents appear alongside local agents in the same sidebar.
- Up to 8 parallel agents (Cursor 2.0) / many more in Cursor 3.
- Git worktrees or remote sandboxes isolate each agent run.

### Visible state model
- Agent status (running, paused, waiting for approval, completed).
- Token usage per Composer session shown in the session footer.
- Real-time tool-call stream (🔍 Reading, ✏️ Writing, 💻 Running, etc.).
- Mode state: Ask (read-only), Agent (full autonomous), Plan (human approves plan first), Debug.

### Interaction model
- **Board-like / canvas-like by default** — the Agents Window is a spatial workspace. You arrange tabs in grids, view multiple agents simultaneously.
- **List-like fallback** — sidebar lists all agents across all environments (local, cloud, web, mobile, Slack, GitHub, Linear).
- Human acts by: dragging tabs, pausing/resuming, redirecting tasks mid-execution, merging/discarding changes, using `/best-of-n` to compare parallel agent outcomes, moving sessions between local and cloud.

**Sources:**
- cursor.com/blog/cursor-3 (Apr 2026)
- cursor.com/docs/cookbook/agent-workflows
- sdd.sh/2026/04/cursor-3-agent-first-branding-ide-last-architecture
- aibreaking.org/blog/cursor-2-0-composer-multi-agent-launch

---

## 3. CLAUDE CODE (Anthropic)

### What a human sees
- **Terminal TUI** built with React + Ink. The UI is entirely in the terminal.
- **Expanded view modes** in AppState: `'none' | 'tasks' | 'teammates'`.
- **Tasks panel** — shows active tasks when `expandedView === 'tasks'`.
- **Session history search** (`useHistorySearch`) and session resume (`claude -c`, `claude -r "<id>"`).
- **Worktree support** (`claude --worktree`) for isolated sessions.
- **`/batch` skill** spawns background agents in worktrees.
- **TodoWrite tool** writes tasks to a structured list, making progress visible and recoverable.

### Grouping model
- Sessions are flat in the CLI (resume by ID/name). No built-in sidebar or board.
- **Third-party dashboards** fill this gap:
  - `claude-code-dashboard` (Stargx): auto-detects all running sessions, shows token/cost, status, context window usage, active subagents, files, git branch.
  - `claude-control` (sverrirsig): native macOS app, auto-discovers sessions, shows live status (Working/Idle/Waiting/Errored/Finished), git changes, PR status, conversation preview, approve/reject from dashboard.
  - `AgentHub` (jamesrochabrun): native macOS app, hub panel with list/2-column/3-column grid layouts, real-time monitoring, embedded PTY terminal per card, plan view, usage stats.
  - `agentdock` (vishalnarkhede): web dashboard, split-panel (session list + terminal/plan/changes/files), mobile-friendly.

### Visible state model
- `AppState` carries: `tasks: { [taskId: string]: TaskState }`, `remoteBackgroundTaskCount`, `mcp` state, `toolPermissionContext`.
- Speculation state for pre-emptive response generation.
- Session status: thinking, waiting, idle, stale (community dashboards add these).

### Interaction model
- **List-like natively** — Claude Code is a single-session-in-view CLI. You switch sessions by resuming IDs.
- **Board-like only via third-party tools** — AgentHub has a hub panel with grid layouts; agentdock has a split-panel dashboard; claude-control has a dashboard with status cards.
- Human acts by: typing prompts in TUI, approving/denying tool calls, resuming sessions by ID, using worktrees for isolation. Third-party tools add approve/reject from dashboard, launch new sessions with worktree support, view plan tabs.

**Sources:**
- mintlify.com/nirholas/claude-code/architecture/state-and-ui (community reverse-engineering of AppState)
- github.com/Stargx/claude-code-dashboard
- github.com/sverrirsig/claude-control
- github.com/jamesrochabrun/AgentHub
- github.com/vishalnarkhede/agentdock
- nimbalyst.com/blog/best-session-managers-for-claude-code-and-codex

---

## 4. WINDSURF CASCADE (Codeium)

### What a human sees
- **Cascade panel** (Cmd/Ctrl+L) — side panel in the IDE with two modes: Code (write) and Chat (read-only).
- **Action Log panel** (View → Cascade Actions) — chronological list of agent steps: `[READ] file.ts`, `[EDIT] file.ts — renamed 4 occurrences`, `[RUN] pnpm typecheck`.
- **Todo list** within the conversation for complex tasks — Cascade creates and updates it automatically.
- **Named checkpoints / reverts** — hover over original prompt, click revert arrow. Named snapshots can be created and navigated to.
- **Plan mode** (`/plan`) — specialized planning agent creates a detailed markdown plan file stored in `~/.windsurf/plans/`. Click "Implement" to switch to Code mode.
- **Simultaneous Cascades** — dropdown menu in top-left of Cascade panel to switch between multiple running Cascade sessions.
- **Skills, Rules, Workflows, Memories** — customization layers with progressive disclosure.

### Grouping model
- **Conversation/thread-centric** — each Cascade session is a conversation thread.
- Multiple Cascades can run simultaneously; switched via dropdown.
- Git worktrees recommended for isolation when running multiple Cascades.
- Plans are external markdown files, cross-session referenceable via `@mention`.

### Visible state model
- Mode toggle: Code / Plan / Ask.
- Action Log shows real-time step stream.
- Todo list tracks progress on complex tasks.
- Checkpoint history in conversation (hover prompt → revert).
- Auto-execution levels: Off / Auto / Turbo.
- Context window usage indicator.

### Interaction model
- **List-like / timeline-like** — Action Log is a vertical chronological feed. Todo list is a checklist inside the conversation.
- **No board view** for multiple sessions. Simultaneous Cascades are managed via a dropdown, not a spatial canvas.
- Human acts by: watching the Action Log, pausing (⏸) when drift is detected, reverting to checkpoints, switching modes, queuing messages while Cascade works, clicking "Implement" on plan files.

**Sources:**
- docs.windsurf.com/windsurf/cascade
- docs.windsurf.com/windsurf/cascade/modes
- docs.windsurf.com/windsurf/cascade/skills
- markaicode.com/windsurf-cascade-agent-autonomous-refactoring
- localskills.sh/blog/windsurf-cascade-workflows

---

## 5. REPLIT AGENT 4

### What a human sees
- **Workspace** — home base with prompt box, editor, and live app preview.
- **Task board** — a column-based kanban board showing all tasks organized by status: Drafts, Active, Ready, and Done. Tasks move left to right.
- **Thread view** — each task runs in its own thread with a live status indicator.
- **Design Canvas** — visual editor for mockups, separate from the code editor.
- **Plan mode** — brainstorm and map out projects; Agent creates an ordered task list.
- **Checkpoints and rollbacks** — complete snapshot of workspace, AI conversation context, environment config, and agent memory.
- **Live preview** of the app in the workspace.

### Grouping model
- **Board-like at the task level** — Task board with columns (Drafts, Active, Ready, Done).
- **Thread-centric at the conversation level** — main thread + background task threads.
- Parallel tasks run simultaneously in isolated copies of the project. Main version stays untouched until changes are applied.
- Up to 10 concurrent tasks (Pro), up to 2 (Core).
- Agent handles conflict resolution when applying changes from multiple tasks.

### Visible state model
- Task states: Draft → Active → Ready → Done, with icons for each stage.
- Agent status in threads (live indicator).
- Checkpoints with AI-generated descriptions.
- Plan mode task lists (ordered, reviewable).
- Live browser preview of the app.

### Interaction model
- **Board-like by default** — the Task board is the primary operational view for tracking parallel work.
- **List-like in Plan mode** — ordered task list before building.
- Human acts by: reviewing tasks on the board, applying or dismissing finished tasks, dragging/interacting with the Design Canvas, rolling back to checkpoints, approving plans.

**Sources:**
- blog.replit.com/introducing-agent-4-built-for-creativity
- docs.replit.com/core-concepts/agent/task-system
- docs.replit.com/core-concepts/agent/plan-mode
- docs.replit.com/replitai/checkpoints-and-rollbacks
- replit.com/agent
- mindstudio.ai/blog/what-is-replit-agent-4

---

## 6. WARP (Warp Terminal)

### What a human sees
- **Terminal mode** (default) — clean terminal input. Agent controls hidden until needed.
- **Oz agent conversation view** — dedicated multi-turn conversation space with model select, voice input, image attachments, richer controls.
- **Conversation Panel** (left side) — split into Active and Past dropdowns. Shows conversation title, status, and recent activity.
- **Task Lists** — automatic task lists that update progress in real time during complex workflows.
- **Blocks** — terminal commands and agent outputs are rendered as discrete blocks. Blocks belong to either the terminal view or a specific agent conversation.
- **Cloud agent management view** — see all cloud agent runs, filter by status, and click into any run.
- **Full Terminal Use** — agent can attach to interactive terminal apps (psql, vim, npm run dev) and the user can take over or hand off control.

### Grouping model
- **Conversation-centric** — each agent interaction is a conversation tied to a session.
- Multiple Agent Mode conversations can run simultaneously in different windows, tabs, or panes.
- Terminal blocks vs agent conversation blocks are visually separated.
- Cloud agents have a separate management view.

### Visible state model
- Conversation status (active, past, cloud-run status).
- Task list progress (auto-updating checklist).
- Context window usage indicator.
- Block-level attribution (terminal block vs agent block).
- Session-level approvals (Allow, Refine, Takeover).

### Interaction model
- **List-like** — Conversation Panel is a collapsible list (Active / Past). Task lists are vertical checklists.
- **No board view** for parallel conversations. Parallelism is achieved through multiple windows/tabs/panes, not a unified board.
- Human acts by: attaching blocks as context, toggling between terminal and agent modes, using Takeover/Hand-off controls, filtering cloud agent runs, approving session-level actions.

**Sources:**
- docs.warp.dev/agent-platform/warps-agent/interacting-with-agents/terminal-and-agent-modes
- docs.warp.dev/agent-platform/local-agents/interacting-with-agents
- docs.warp.dev/agent-platform/warps-agent/capabilities-overview/full-terminal-use
- warp.dev/blog/december-drop

---

## 7. GITHUB COPILOT / VS CODE AGENT SESSIONS

### What a human sees
- **Agent Sessions view** (VS Code) — unified list of local, background, and cloud agent sessions. Supports multi-select, bulk operations, resizing, filters, and a stacked view.
- **Agents tab / Agents panel** (GitHub web) — list of running and past agent sessions across repositories. Each session shows status. Click to open session log and overview (progress, token usage, session count, length).
- **Mission Control** (GitHub, late 2025) — dashboard for assigning, steering, and tracking multiple concurrent Coding Agent tasks. "Engineering manager interface for your AI teammates."
- **Session logs** — internal monologue and tools used, viewable in GitHub, VS Code, or Raycast.
- **Agent status indicator** in VS Code.
- **Steering** — prompt Copilot mid-run without stopping the session.

### Grouping model
- **List-like in VS Code** — Agent Sessions view is a list with filters and multi-select.
- **List-like on GitHub web** — Agents tab is a table/list of sessions.
- **Dashboard-like in Mission Control** — designed for overseeing multiple agents ("which agent tasks are running, review progress, intervene when they stall").
- Sessions can be local, background, or cloud. Handoff between types is supported.
- Forked sessions supported (fork a chat session from current history).

### Visible state model
- Agent phase: STARTING, RUNNING, WAITING, COMPLETED, FAILED, DELETED.
- PR status tracking (open, merged, closed, draft).
- Suggestion status (commit suggestion workflow).
- Token usage, session count, session length.
- Background task count.

### Interaction model
- **List-like primary** — Agent Sessions view is a list.
- **Dashboard-like in Mission Control** — higher-level orchestration view.
- Human acts by: switching between sessions, archiving in bulk, steering mid-run, opening sessions in VS Code from GitHub, reviewing session logs, comparing parallel agent approaches.

**Sources:**
- docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/track-copilot-sessions
- docs.github.com/en/copilot/how-tos/use-copilot-agents/manage-agents
- javacodegeeks.com/2026/02/github-copilot-workspace-the-agentic-era.html
- code.visualstudio.com/updates/v1_109 (Jan 2026)
- youtube.com/watch?v=0CsKOO7d35I (VS Code Learn: Agent Sessions)

---

## 8. CONTINUE.DEV

### What a human sees
- **IDE side panel** (VS Code / JetBrains) — chat session with Agent/Chat/Plan modes.
- **`cn` CLI TUI** — terminal UI with slash commands (`/resume`, `/fork`, `/jobs`, `/info` for token usage).
- **Session selector with preview panel** (`cn ls` or `/resume`) — two-column layout: session list on left, chat history preview on right (first 10 messages, truncated).
- **Background Agents UI** — list background agents in the GUI with status badges, 10s polling, and links to agent detail pages.
- **`/jobs` slash command** — list background jobs.

### Grouping model
- **List-like** — session list in TUI, background agents list in GUI.
- **Preview-enhanced list** — session selector shows history preview, inspired by fzf.
- Background agents are tracked separately with polling.

### Visible state model
- `AgentPhase` enum: STARTING, RUNNING, COMPLETED, FAILED, etc.
- `pullRequestStatus`, `suggestionStatus`.
- Background agent status badges.
- Token usage and cost (`/info`).

### Interaction model
- **List-like** — no board view. Sessions are a list with preview.
- Human acts by: resuming/forking sessions, listing background jobs, polling agent status, compacting chat history.

**Sources:**
- docs.continue.dev/cli/tui-mode
- github.com/continuedev/continue/pull/8231 (session preview panel)
- github.com/continuedev/continue/issues/10254 (session lifecycle docs)
- github.com/continuedev/continue/pull/8191 (background agents UI)

---

## 9. OH-MY-OPENCODE / OPENCODE DASHBOARDS (Community)

### What a human sees
- **`oh-my-opencode-dashboard`** (WilliamJudge94) — web dashboard showing: main session status, plan progress tracking, background task monitoring, tool call metadata, token usage, time-series activity visualization. Browser notifications on task completion or when input needed.
- **`ohmydashboard`** (radenadri) — web dashboard with: 5 summary cards (total sessions, messages, tokens, cost, active agents), active agents live view, agent leaderboard, cost history chart, model distribution donut, activity heatmap, session table with TanStack Table (sorting, search, filter, pagination, expandable rows).
- **Built-in TUI** — OpenCode TUI shows tool call lines (e.g., `delegate_task [prompt=..., run_in_background=true, subagent_type=explore]`) but limited metadata (no model info visible on background tasks — known UX gap, github issue #941).

### Grouping model
- **Dashboard / analytics view** — summary cards + tables + charts.
- **Background task table** — lists background tasks with status, tool calls, last tool used.
- Session table with expandable rows.

### Visible state model
- Main session: agent name, current model, current tool, status pill.
- Background tasks: status (queued, running, completed, error, unknown), tool calls count, last tool, last model.
- Plan progress: completed steps count, step descriptions.
- Time-series activity data.

### Interaction model
- **Analytics-dashboard-like** — not a board for acting, but a board for observing. Primary actions are viewing, not dragging or moving tasks.
- Human acts by: viewing dashboard, receiving browser notifications, inspecting session tables. Limited direct action from the dashboard itself.

**Sources:**
- github.com/WilliamJudge94/oh-my-opencode-dashboard
- github.com/radenadri/ohmydashboard
- github.com/code-yeongyu/oh-my-opencode/issues/941
- context7.com/williamjudge94/oh-my-opencode-dashboard/llms.txt

---

## COMPARATIVE SUMMARY: BOARD-LIKE vs LIST-LIKE

| Product | Primary Grouping | Board-Like? | List-Like? | Human Action Surface |
|---|---|---|---|---|
| **Devin** | Session list + nested parent/child | ❌ | ✅ (sidebar list, Session Manager table) | Filter, tag, click into progress timeline, take over IDE/shell/browser |
| **Cursor 3** | Agent Tabs (grid/stack/side-by-side) | ✅✅ (spatial Agents Window) | ✅ (sidebar agent list) | Drag tabs, move local↔cloud, pause/resume, merge/discard, Design Mode annotate |
| **Claude Code** | Single session TUI | ❌ (native) | ✅✅ (native CLI list resume) | Approve/deny tools, resume by ID, worktree isolation |
| **Claude Code + 3P** | Dashboard cards / grid panels | ✅ (AgentHub grid, agentdock split-panel, claude-control cards) | ✅ | Approve/reject from dashboard, launch sessions, view plans, monitor git/PR |
| **Windsurf Cascade** | Conversation threads | ❌ | ✅✅ (Action Log timeline, todo checklist, dropdown switch) | Pause, revert checkpoint, switch modes, queue messages, click Implement on plan |
| **Replit Agent 4** | Task board columns | ✅✅ (Drafts/Active/Ready/Done kanban) | ✅ (Plan mode ordered list) | Move tasks through columns, apply/dismiss changes, rollback checkpoints, Design Canvas |
| **Warp** | Conversation panel (Active/Past) | ❌ | ✅✅ (collapsible list, task checklist) | Attach blocks, takeover/handoff, filter cloud runs |
| **GitHub Copilot / VS Code** | Agent Sessions list | ✅ (Mission Control dashboard) | ✅✅ (Agent Sessions view list) | Multi-select bulk archive, steer mid-run, open in VS Code, compare approaches |
| **Continue.dev** | Session list + preview | ❌ | ✅✅ (list with fzf-style preview) | Resume, fork, list background jobs, poll status |
| **OhMyOpenCode dashboards** | Summary cards + tables | ✅ (analytics dashboard) | ✅ (session table) | View-only primarily; notifications for completion/input-needed |

---

## KEY PATTERNS FOR FOCUS-TUI

### What makes a surface feel "board-like" (not just a list)
1. **Spatial arrangement** — Cursor's Agents Window lets you arrange tabs in grids. Replit's Task board has columns. AgentHub has grid layouts. This gives situational awareness at a glance.
2. **State as position** — Replit tasks *move* left-to-right through columns. Cursor agents occupy visual "slots." Position encodes status.
3. **Parallel visibility** — Multiple sessions/tasks visible simultaneously without clicking through a list. Cursor 3's grid, Replit's board, and AgentHub's multi-column layout all do this.
4. **Human acts by moving / promoting** — Replit: apply/dismiss tasks (promote out of Ready). Cursor: drag tabs, move local↔cloud. AgentHub: click cards to open embedded terminals. The UI supports direct manipulation of work items.

### What mature products do that Focus-tui should consider
- **Nested grouping** — Devin's parent/child session indentation, Replit's main thread + task threads.
- **Status badges + real-time polling** — Continue's background agents (10s polling), OhMyDashboard's live view, claude-control's Working/Idle/Waiting classification.
- **Action from the overview** — Approve/reject from claude-control dashboard; apply/dismiss from Replit board; merge/discard from Cursor dashboard.
- **Handoff between environments** — Cursor's local↔cloud movement. VS Code's local/background/cloud session types.
- **On-demand insights** — Devin's Session Insights (generate analysis on demand, not auto). This keeps the UI clean until the human asks for depth.
- **Preview without opening** — Continue's session selector with chat history preview. AgentHub's conversation preview in cards.
- **Token/cost visibility** — Almost every product surfaces this. Devin, Cursor, Claude Code dashboards, OhMyDashboard all show token usage and/or cost.

### What to avoid
- **Flat history lists** — Claude Code native, Continue native, and Warp's Past dropdown are all limited by being flat chronological lists. Third-party tools immediately add dashboards to fix this.
- **Chat-only UIs** — Windsurf Cascade's Action Log is powerful but it's still a vertical feed inside a conversation. Without the Task board (Replit) or Agents Window (Cursor), long-running parallel work is hard to track.
- **Over-automation of insights** — Devin moved Session Insights from auto-generated to on-demand. This suggests humans prefer control over when analysis appears.

---

## CONCLUSION

The strongest board-like models are:
- **Replit Agent 4's Task board** — true kanban with columns, parallel tasks, and apply/dismiss actions.
- **Cursor 3's Agents Window** — spatial canvas with draggable tabs, grid layouts, and environment handoff.
- **Third-party Claude Code dashboards** (AgentHub, claude-control, agentdock) — they add grid/list hybrid views with embedded terminals and action buttons because the native CLI lacks them.

For Focus-tui, the gap between "running/recent list" and "board-like orchestration" is exactly what these products have solved in different ways. The most transferable pattern is a **hybrid: a status-driven list that can expand into a spatial or columnar view** (like Cursor's sidebar → Agents Window, or VS Code's Agent Sessions list → Mission Control).

**Sources cited:** 20+ public docs, GitHub repos, product blogs, release notes, and reverse-engineering docs. No product behavior was fabricated.
