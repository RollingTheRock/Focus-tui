# ADR-0007: Current Implementation Consolidation — Protocol, Storage, and Orchestration as of Today

- **Date:** 2026-06-14
- **Status:** Accepted
- **Consolidates:** ADR-0000, ADR-0001, ADR-0004, ADR-0005, ADR-0006
- **Clarifies:** ADR-0002, ADR-0003, `dual-mode-agent-architecture.md`

---

## Context

Focus-tui 的架构经历了多轮快速迭代。每一轮迭代都留下了对应的 ADR，这些 ADR 共同构成了项目的设计历史：

- **ADR-0000** 确立了人类主权与 `ADR → Plan → Task → Session` 四层执行层级。
- **ADR-0001** 确立了产品定位：Human-Sovereign、Agent-Native 的开发者工作台。
- **ADR-0002 / ADR-0003** 提出了 in-process **Global Agent**，后被 ADR-0004 废除。
- **ADR-0004** 转向去中心化的 **Agent Mesh**：Agent 作为外部独立进程运行，Focus-tui 作为指挥中心，使用 **A2A + MCP** 双协议通信。
- **ADR-0005** 提出将存储从 CRUD 迁移到 **Event Sourcing**，使用 PostgreSQL 作为事件存储，并引入 **Piggyback** 机制。
- **ADR-0006** 确立了 **Phase-Step 两层任务模型**，DAG pane 只显示 Phase，Step 对人类透明。
- **`dual-mode-agent-architecture.md`** 提出了 Interactive + Headless 双模式 Agent 执行框架，目前仍处于探索阶段。

经过半年的工程实践，代码已经沉淀出一套相对稳定的实现。但旧 ADR 之间存在以下需要澄清的地方：

1. ADR-0004 设计了 A2A + MCP 双协议，但当前主流程已完全使用标准 MCP。
2. ADR-0004 中 Orchestrator 被设计为可以自动启动下游 Agent，但当前实现中已移除自动启动。
3. ADR-0005 提出 PostgreSQL + Event Sourcing 为唯一真相来源，但当前默认生产模式仍是 SQLite。
4. ADR-0006 奠定了 Phase-Step 分层，但 Phase 与 Worktree 的绑定、`plan.expand_to_tasks` 的边界仍需明确。
5. `dual-mode-agent-architecture.md` 中的 Headless 模式部分代码存在，但未接入主调度流程。

本 ADR **不废除任何历史 ADR**，而是对当前实现进行一次集中澄清，作为项目发版前的架构快照。后续若架构再次发生重大变化，应通过新的 ADR 来 supersede 本 ADR 或相关旧 ADR。

---

## Decision

### D1. 协议层：标准 MCP 是唯一活跃协议，A2A 已废弃

当前实现采用 **标准 Model Context Protocol (MCP)** 作为 Agent 与 Focus-tui 之间的唯一活跃通信协议。

- 已实现完整的 MCP server，支持 `initialize`、`tools/list`、`tools/call`、`resources/list`、`resources/read`、`resources/subscribe`、`ping` 等方法。
- 传输层默认使用 **HTTP (Streamable HTTP)**，Agent 通过环境变量 `FOCUS_MCP_URL` 接入。
- Unix Domain Socket 传输在代码中保留（默认路径 `.focus/mcp.sock`），用于兼容和本地测试，但不再是主路径。
- **A2A 协议已废弃**。`internal/a2a/` 目录中的自定义 Pub/Sub 实现不再被主流程实例化或启动，保留代码仅作为清理前的历史遗留。

这一决策继承 ADR-0004 中 "MCP 作为共享状态协议" 的核心思想，但废弃了其 "A2A 作为信号协议" 的部分。

### D2. 存储层：SQLite 为默认生产模式，PostgreSQL + Event Sourcing 为可选进阶模式

当前实现同时支持两种存储模式：

| 模式 | 触发条件 | 真相来源 | 适用场景 |
|------|---------|---------|---------|
| **SQLite** | 默认；`FOCUS_STORE=sqlite` 或未设置 | SQLite 表直接作为真相来源 | 本地开发、单仓库、快速上手 |
| **PostgreSQL** | `FOCUS_STORE=postgresql` | `events` 表为追加式事件日志，`proj_*` 表为投影 | 需要完整事件溯源、Piggyback、跨会话重建 |

- SQLite 模式下，写操作直接更新当前状态表，同时通过内存 `EventBus` 发布事件（不持久化到 Event Store）。
- PostgreSQL 模式下，写操作采用双写：先更新 `proj_*` 投影表，再向 `events` 表追加事件。
- **Piggyback（夹带增量）** 当前仅在 PostgreSQL 模式下可用，因为它依赖持久化的 Event Store。
- 这不是最终架构，而是迁移期的双模式并存。SQLite 模式降低了新用户的接入门槛；PostgreSQL 模式验证 Event Sourcing 的完整路径。

这一决策继承 ADR-0005 的方向，但承认当前尚未完成从 SQLite 到 Event Sourcing 的完全切换。

### D3. Orchestrator：不自动启动 Agent，只通知人类并自动转换任务状态

当前 `orchestrator.Orchestrator` 的职责边界如下：

- 订阅 `TaskStateChanged` 和 `PlanStepStateChanged` 事件。
- 当下游依赖满足时，自动将 `blocked` 任务转为 `active`。
- 当上游任务从完成状态回退时，自动将下游任务重新置为 `blocked`。
- 向 TUI 发送 `downstream_ready`、`task_blocked`、`plan_completed` 等通知，由人类决定是否启动 Agent。
- 每 30 秒检测 Agent 心跳，超时时标记为 `disconnected` 并通知。
- **Orchestrator 不自动启动 Agent**。自动启动 Agent 的能力已在 `launchDownstreamTasks` 中移除，注释明确说明这是为了遵守 ADR-0000 的人类主权原则。

这一决策修正了 ADR-0004 中 "Orchestrator 自动启动下游 Agent" 的设计，使其与 ADR-0000 的人类主权约束保持一致。

### D4. Agent 执行：外部终端为主，内嵌 shell 为辅

Agent 以**外部独立进程**的形式运行，Focus-tui 通过以下方式启动 Agent：

1. 创建 `agents.Session`，分配 `FOCUS_SESSION_ID`。
2. 通过 Trellis Bridge 生成 per-worktree 的上下文文件（`AGENTS.md` / `CLAUDE.md`）。
3. 根据配置选择启动方式：
   - `agent.external_terminal = true`：通过外部终端模拟器（kitty / alacritty / wezterm / gnome-terminal / ptyxis）启动 Agent。
   - `agent.external_terminal = false`：在 TUI 内嵌 shell pane 中启动 Agent。
4. 向 Agent 注入环境变量：`FOCUS_MCP_URL`、`FOCUS_SESSION_ID`、`FOCUS_TASK_ID`、`FOCUS_PLAN_ID`、`TRELLIS_CONTEXT_ID`。

Agent 发现通过 `pgrep` 扫描已知 Provider 进程实现。当前支持的 Provider 包括 OpenCode、Claude、Kimi、Codex、Gemini 和 generic。

Headless Driver（`internal/agents/driver_kimi.go`）基于 Kimi Agent SDK 实现，目前处于实验性状态，尚未接入主调度流程。

### D5. Phase-Step-Worktree：一个 Phase 绑定一个 Worktree，Step 继承绑定

继承 ADR-0006 的两层任务模型，当前实现明确以下细节：

- **Phase**：`task_contexts.parent_task_id IS NULL`，对人类可见，代表架构级阶段。
- **Step**：`task_contexts.parent_task_id IS NOT NULL`，对人类透明，Agent 在 worktree 内自调度。
- **Phase-Worktree 绑定**：Phase 通过 `task_contexts.preferred_worktree_id` 绑定到一个 worktree。
- **Step-Worktree 继承**：Step 创建时若指定 `parent_task_id`，系统会自动通过 `task_worktree_links(relation_type='secondary')` 将 Step 关联到父 Phase 的 worktree。
- **DAG 暴露**：`dag.get_status` 默认只返回 Phase 级 DAG；`detail=full` 返回完整 Phase+Step DAG。
- **Plan Expansion**：`plan.expand_to_tasks` 将每个 plan step 创建为 Phase（无 parent），第一个 Phase 为 `active`，后续为 `blocked`，并建立顺序依赖。

Phase 是 worktree 的入口，Step 是 worktree 内的执行细节。人类通过 Phase 控制执行节奏，Agent 在 Phase 内部完成 Step 的分解与执行。

### D6. Trellis 集成：上下文桥梁，不是执行层

Trellis 与 Focus-tui 的集成通过 `internal/trellis/bridge.go` 实现，定位如下：

- Trellis 负责管理 per-worktree 的 spec、PRD、handoff journal、workflow state。
- Focus-tui 在创建任务 / 启动 Agent 时，通过 Trellis Bridge 读取和写入这些上下文。
- Trellis 不提供 Agent 执行能力，也不替代 Focus-tui 的 Orchestrator。
- 当 MCP `context.get_for_task` 被调用时，Focus-tui 通过 Trellis Bridge 组装任务上下文返回给 Agent。

Trellis 是 Focus-tui 的**上下文增强器**，而非执行引擎。

### D7. Dual-Mode：Headless 模式为实验性路线图

`dual-mode-agent-architecture.md` 中提出的 Interactive + Headless 双模式当前状态如下：

- **Interactive 模式**：已落地，即当前 TUI + 外部终端 Agent 的执行方式。
- **Headless 模式**：部分基础设施存在（`AgentDriver` 接口、`KimiDriver`），但未接入主调度器，不支持从 TUI 直接触发。
- Headless 模式被视为未来路线图，而非当前发布版本的一部分。

本 ADR 明确：当前发布版本只承诺 Interactive 模式的能力，Headless 模式的完整实现将通过后续 ADR 跟进。

---

## Consequences

### 保留的好处

- **人类主权得到保障**：Orchestrator 不自动启动 Agent，所有 Agent 启动最终由人类触发。
- **接入门槛低**：SQLite 默认模式让新用户无需 PostgreSQL 即可运行 Focus-tui。
- **协议简单**：单一 MCP 协议降低了 Agent Provider 的接入成本。
- **历史决策得以保留**：旧 ADR 继续存在，项目演进脉络清晰。

### 需要接受的代价

- **双存储模式维护成本**：SQLite 和 PostgreSQL 两条路径需要同时维护，直到 Event Sourcing 完全成熟。
- **A2A 代码需要清理**：`internal/a2a/` 目录已废弃但尚未删除，后续应通过专项重构移除。
- **Piggyback 在 SQLite 下不可用**：默认用户无法享受夹带增量能力，这是迁移期的临时限制。
- **Headless 模式尚未可用**：对于期望无人值守执行的用户，当前版本无法满足。

---

## References

- [ADR-0000 — Constitution for ADR-Driven Execution](./0000-constitution-for-adr-driven-execution.md)
- [ADR-0001 — Product Positioning: Human-Sovereign, Agent-Native Workbench](./0001-product-positioning-human-sovereign-agent-native-workbench.md)
- [ADR-0002 — Global Agent Architecture: Human-Augmented Development](./0002-global-agent-architecture-human-augmented-development.md)
- [ADR-0003 — Global Agent Architecture: Technical Selection](./0003-global-agent-architecture-technical-selection.md)
- [ADR-0004 — Protocol-Driven Multi-Agent Orchestration with External Terminal Execution](./0004-protocol-driven-multi-agent-orchestration.md)
- [ADR-0005 — Event-Sourced Shared Context Store](./0005-event-sourced-shared-context-store.md)
- [ADR-0006 — DAG Pane Phase-Step 分层与 Agent 粒度约束](./0006-dag-pane-phase-step-hierarchy.md)
- [dual-mode-agent-architecture.md](./dual-mode-agent-architecture.md)
- [MCP over A2A Refactor Plan](../architecture/mcp-a2a-refactor-plan.md)
