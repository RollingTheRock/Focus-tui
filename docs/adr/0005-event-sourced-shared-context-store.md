# ADR-0005: Event-Sourced Shared Context Store with Piggyback Synchronization

- **Date:** 2026-05-06
- **Status:** Proposed

---

## Context

### 1. ADR-0004 的存储层现状

ADR-0004 确立了 **Protocol-Driven Multi-Agent Orchestration**，Agent 通过 MCP 读写共享状态，调度器通过检测状态变化触发下游 Agent。当时的存储层设计为：

- **SQLite** (`modernc.org/sqlite`) 作为持久化后端
- **WAL 模式** 缓解读阻塞
- **15+ 张表** 直接存储当前状态（CRUD 模型）

### 2. 生产环境中暴露的三个致命问题

#### 问题 1：SQLite 锁导致操作失败

SQLite 在 WAL 模式下 writer 仍然是**全局串行**的。当多个 Agent 并发写入（尤其是心跳更新、状态变更），频繁出现 `database is locked` 错误，后续正常操作无法完成。`modernc.org/sqlite` 的 busy timeout 只能缓解，无法根除。

#### 问题 2：Agent 无法实时感知上下文变化

MCP 是 request-response 协议（pull 模型），Agent 不发起 tool call 就无法获取信息。更关键的是，**标准 CLI Agent（Claude Code、Kimi CLI 等）作为第三方进程，其 MCP 客户端实现不处理 server-to-client notifications**。即使 MCP 协议原生支持 `resources/subscribe` 和 `notifications/resources/updated`，主流 CLI 工具也不响应这些通知。这意味着任何依赖"push"到 Agent 的架构对标准 CLI 都不成立。

#### 问题 3：多 Agent 同时读写是刚需

Focus-tui 的 Agent Mesh 架构天然要求多个本地 Agent 进程并行读写共享上下文。SQLite 的连接模型（文件锁）和并发写能力已触及架构上限。

### 3. 调研后的关键发现

#### 发现 1：MCP Resource Subscription 对 CLI Agent 是"死路"

| 客户端 | Notification 支持 | 证据 |
|--------|------------------|------|
| Claude Code | ❌ 不支持 | GitHub issue #4094 明确：不响应 `notifications/prompts/list_changed`，不刷新 tool/resource list |
| Kimi CLI | ❓ 未提及 | 官方文档只涉及 `kimi mcp add/list/remove/auth`，零 notification 相关内容 |
| Claude Desktop | ⚠️ 部分 | 支持 sampling，但 notifications 不稳定 |

Reddit 社区讨论 *"Do MCP clients support Push Notifications?"* 的共识：**基本不支持**。任何依赖 push 的 Agent 接入方案只能覆盖少数 IDE/桌面客户端，不能覆盖作为主力执行者的 CLI Agent。

#### 发现 2：业界务实共识是 Delta/Piggyback 模式

- **`shared-memory-mcp`**（开源项目）核心 API 为 `get_context_delta(since_version)`，实现增量状态共享，达成 6x token 效率提升
- **memX** 使用 Redis + WebSocket Pub/Sub，但要求 Agent 端集成 SDK 或维护 WebSocket 连接
- **AutoGen v0.4** 完全重构为 Actor 模型 + 异步消息，但前提是 Agent 在框架内运行，不适用第三方 CLI

#### 发现 3：Blackboard 架构被学术重新验证

LbMAS 论文证明：Agent 通过共享黑板通信（无直接联系）、Control Unit 基于黑板内容选择下一 Agent，在数学和推理任务上达到 SOTA 性能。这与 Focus-tui 的 Orchestrator + 共享上下文架构方向一致。

### 4. 技术约束

- **Agent 是本地进程**（Kimi CLI、Claude Code、Codex 等第三方工具）
- **Orchestrator 与 Focus-tui 是同一进程**
- **短期内存储层以进程内嵌入为主**，独立引擎是未来方向
- **Agent 代码不可修改**——它们是标准 CLI 工具，不是框架内组件

### 5. 核心洞察

> **不要试图"推送"给标准 CLI Agent。改为让 Agent 每次"拉取"时，自动拿到增量。**
>
> Agent 不需要知道"事件"的存在——它只需要知道"我查询的上下文总是最新的"。

---

## Decision

### D1. 存储范式迁移：从 CRUD 到 Event Sourcing

**当前状态表（`task_contexts`、`agent_sessions` 等）不再是真相来源。** 唯一的真相来源是**不可变的全局事件日志**。

所有状态变更遵循：

```
Command → Validate → Append Event → Publish to Bus → Projection Builder 更新物化视图
```

**事件是 append-only 的**，利用 PostgreSQL 的 MVCC 保证：
- 写只有 `INSERT`，无锁竞争
- 读（查询物化视图）不阻塞写（追加事件）
- 长事务不阻塞其他写入

### D2. PostgreSQL 作为 Event Store（短期嵌入进程内）

短期内 Event Store 以**进程内嵌入 PostgreSQL** 方式运行，未来可平滑切换为独立 PostgreSQL 实例或分布式引擎。

**Event Store Schema：**

```sql
-- 全局事件日志：唯一需要严格 ACID 的表
CREATE TABLE events (
    event_id       BIGSERIAL PRIMARY KEY,  -- 全局严格递增逻辑时钟
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 聚合根（借鉴 DDD + Temporal 的 entity 概念）
    aggregate_type TEXT NOT NULL,           -- 'task', 'worktree', 'plan', 'agent_session'
    aggregate_id   TEXT NOT NULL,
    version        BIGINT NOT NULL,         -- 聚合内版本，乐观并发控制

    -- 事件内容
    event_type     TEXT NOT NULL,           -- 'TaskCreated', 'TaskStateChanged', ...
    payload        JSONB NOT NULL,

    -- 元数据：因果追踪 + Agent 路由
    actor_type     TEXT,                    -- 'human', 'agent', 'orchestrator', 'system'
    actor_id       TEXT,
    causation_id   BIGINT REFERENCES events(event_id),  -- 哪个事件导致了当前事件
    correlation_id TEXT,                    -- 业务追踪 ID（workflow / trace）

    -- 订阅路由（用于 Piggyback 过滤）
    scope_type     TEXT,                    -- 'task', 'worktree', 'plan', 'global'
    scope_id       TEXT,

    UNIQUE(aggregate_type, aggregate_id, version)
);

-- 按聚合查询（replay 时用）
CREATE INDEX idx_events_aggregate
    ON events(aggregate_type, aggregate_id, version);

-- 按全局顺序消费（projection builder 用）
CREATE INDEX idx_events_order
    ON events(event_id);

-- 按 scope 查询（piggyback 过滤用）
CREATE INDEX idx_events_scope
    ON events(scope_type, scope_id, event_id);

-- 业务追踪
CREATE INDEX idx_events_correlation
    ON events(correlation_id, event_id);

-- 每个 Agent/消费者的消费位点
CREATE TABLE consumer_offsets (
    consumer_id    TEXT PRIMARY KEY,        -- 'orchestrator', 'projection-task', 'agent-kimi-abc'
    last_event_id  BIGINT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ DEFAULT NOW()
);
```

**关键特性：**
- `event_id` 全局递增：提供精确的逻辑时钟
- `version` + `UNIQUE` 约束：同一聚合的并发写被 PostgreSQL 拒绝，天然防止竞争写
- `scope_type` / `scope_id`：支持按任务/工作树/计划维度快速过滤事件（Piggyback 核心依赖）
- `causation_id` + `correlation_id`：可画出完整因果图

### D3. Projections 作为物化查询视图

当前所有状态表变为**可丢弃、可重建**的 projections。

```sql
CREATE TABLE proj_tasks (
    id             TEXT PRIMARY KEY,
    repo_id        TEXT NOT NULL,
    title          TEXT NOT NULL,
    goal           TEXT,
    next_step      TEXT,
    state          TEXT,
    priority       TEXT,
    parent_task_id TEXT,
    preferred_worktree_id TEXT,
    event_version  BIGINT NOT NULL,        -- 投影到哪个版本了
    updated_at     TIMESTAMPTZ
);
```

**Projection Builder**（同进程内的一个 goroutine）消费事件流并更新物化视图。Projections 可随时重建：`TRUNCATE proj_tasks; Replay all Task events`。

### D4. Piggyback（夹带增量）——标准 CLI Agent 的唯一路径

**核心机制**：Focus MCP Server 为每个 connected client 维护 `last_seen_event_id`。当 client 调用任何 `context.*` tool 时，自动计算并返回该 client 未消费的相关事件。

```
Agent 调用 context.get_task("task-123")
    │
    ▼
MCP Server 执行 tool 逻辑
    │
    ▼
查询 events WHERE event_id > agent_last_seen_event_id
        AND scope 与当前 tool 相关
    │
    ▼
将事件摘要附加到 tool result
    │
    ▼
更新 agent_last_seen_event_id = 最新 event_id
```

**Tool Result 示例：**

```json
{
  "content": [
    {
      "type": "text",
      "text": "{\"id\":\"task-123\",\"title\":\"Fix auth bug\",\"state\":\"active\"...}"
    },
    {
      "type": "text",
      "text": "\n---\n📬 Context Updates since your last interaction:\n• 2 min ago: Agent-Kimi completed subtask 'auth-module-refactor'\n• 1 min ago: New blocker: 'Database migration pending review'\n• 30 sec ago: Plan step 3 status → 'in_progress' by Agent-Codex\n---\n"
    }
  ]
}
```

**为什么这是正确的默认路径：**

| 特性 | 说明 |
|------|------|
| **零 Agent 适配** | 标准 CLI Agent 完全无感知，每次 tool call 自动带回增量 |
| **零额外 API 调用** | 不需要专门的 `sync_events()` 轮询 tool |
| **LLM 自然理解** | 事件摘要是自然语言，Agent LLM 自动根据信息调整行为 |
| **精准过滤** | 只返回与当前 tool 调用 scope 相关的事件，避免噪音 |
| **无代码路径分叉** | 一种机制覆盖所有标准 CLI Agent，没有 UDS vs Long Polling 的复杂度 |

**边界控制：**
- 事件过多时：返回最近 N 条 + "还有 X 条未读事件"
- Agent 长时间不调用 tool（如编译 5 分钟）：**不推送**——Agent 不应在执行中途被打断，这是正确行为
- 紧急事件需要立即响应：见 D6（Orchestrator 职责）

### D5. MCP Resource Subscription —— 支持 Notification 客户端的 Bonus

把共享上下文暴露为 MCP Resources，给**支持 notifications 的客户端**（如未来的 Claude Desktop、VS Code 扩展）提供真正的实时推送：

```
context://task/{task_id}
context://worktree/{worktree_id}
context://plan/{plan_id}
context://workspace/current
```

**Resource 能力声明：**
```json
{
  "capabilities": {
    "resources": {
      "subscribe": true,
      "listChanged": true
    }
  }
}
```

支持 `resources/subscribe` 的客户端会自动收到 `notifications/resources/updated`。这是**bonus 路径，不是主要路径**——标准 CLI Agent 走 D4 的 Piggyback。

### D6. Orchestrator 基于内存 Event Bus 的状态通知引擎

Orchestrator、TUI、Projection Builder 作为同一进程内的 goroutine，通过**内存通道（Go channel）**直接订阅 Event Bus，零序列化延迟。

```go
type EventBus struct {
    inProcSubs []InProcSubscription  // orchestrator, projection builder, TUI
}

func (bus *EventBus) Publish(e Event) {
    for _, sub := range bus.inProcSubs {
        if sub.Matches(e) {
            sub.Ch <- e
        }
    }
}
```

**Orchestrator 的职责是"检测状态变化并通知人类"，不是"自动调度执行"。** 人类保留创建工作树和启动 Agent 的最终决策权。

| 事件 | Orchestrator 行为 |
|------|------------------|
| 任务 A 完成 → 下游任务 B 依赖满足 | TUI 通知栏显示："下游任务 B 已就绪，等待创建工作树并启动 Agent" |
| 新 blocker 添加且影响正在运行的 Agent | TUI 通知栏显示警告："Agent-X 当前任务出现新 blocker：..." |
| Agent 心跳超时 | 标记 session disconnected，TUI 通知栏显示："Agent-X 已断开，建议检查" |
| Agent 写入新的 session handoff | 更新 resume summary cache，TUI 实时刷新 worktree 状态 |
| 计划所有步骤完成 | TUI 通知栏显示："计划已完成，建议审阅产出" |

**Agent 的启动由人类通过 TUI 触发，不是由 Orchestrator 自动触发。** Orchestrator 只负责在 TUI 中呈现"可以做什么"的提示，人类决定"何时何地用什么 provider 去做"。

```
Event Bus
    │
    ▼
┌─────────────────────────────────────┐
│        Orchestrator Engine          │
│  ┌─────────┐  ┌──────────────┐     │
│  │ Event   │  │ Rule Engine  │     │
│  │ Consumer│→ │ (switch/map) │     │
│  │ (1 goroutine)│             │     │
│  └─────────┘  └──────┬───────┘     │
│                      │              │
│         ┌────────────┼────────────┐│
│         ▼            ▼            ▼│
│    ┌────────┐  ┌──────────┐  ┌────────┐
│    │ Notify │  │  Update  │  │  Track │
│    │ TUI    │  │  State   │  │ Agents │
│    │(channel)│  │(memory)  │  │(memory)│
│    └────────┘  └──────────┘  └────────┘
│                      │              │
│    ┌─────────────────┘              │
│    ▼                                │
│ TUI Dashboard 显示通知              │
│ 人类点击"创建工作树并启动 Agent"      │
│    │                                │
│    ▼                                │
│ 调用 app.launchAgent(...) ──────────┘
└─────────────────────────────────────┘
```

### D7. 渐进式迁移策略（四阶段）

**Phase 1：Event Store 双写（1 周）**
- 保留现有所有 SQLite 表和 SQL 不变
- 新增 PostgreSQL `events` 表
- 每个写操作**先 append event，再写 SQLite**
- Event 表此时仅用于审计和验证

**Phase 2：Projection 并行 + Piggyback 实验（1 周）**
- 新增 projection builder goroutine
- 新增 `consumer_offsets` 表和 per-client tracking
- 在 1-2 个 `context.*` tool 中实验 Piggyback（如 `context.get_task`）
- 读操作逐步切换到 `proj_*`，保留旧表对照
- 每天运行一致性校验

**Phase 3：Command 化写操作（2 周）**
- 新增 Command Handler 层
- 写操作改为 `SendCommand → Validate → Append Event → Projection Builder 更新`
- 旧表的直接 UPDATE 逐步下线
- 所有 `context.*` tools 启用 Piggyback

**Phase 4：SQLite 下线 + Resource Subscription（1 周）**
- 删除 SQLite 旧表，projection 表成为主表
- 添加 MCP Resource 层
- TUI / Orchestrator 全面接入内存 Event Bus

---

## Consequences

### Positive

- **根除锁问题**：PostgreSQL MVCC + append-only 事件写入，无全局写锁，多 Agent 并发读写不再阻塞
- **标准 CLI Agent 零适配**：Piggyback 模式下 Agent 完全无感知，每次 tool call 自动带回增量，不需要任何特殊工具或协议扩展
- **单一机制覆盖所有 Agent**：没有 UDS vs Long Polling 的代码路径分叉，一种 Piggyback 机制覆盖 Claude Code、Kimi CLI、Codex 等所有标准 CLI
- **Orchestrator 真正实时**：内存 Event Bus 让引擎毫秒级感知状态变化，可立即在 TUI 通知人类（下游就绪、blocker 出现、Agent 断开等）
- **完整的因果追踪**：`causation_id` 和 `correlation_id` 可精确还原"谁在什么时候因为什么改了什么"
- **可重建的历史**：Projections 可随时从事件日志重建，调试时可 replay 任意时间点的状态
- **未来兼容**：支持 notifications 的 MCP 客户端（IDE/桌面应用）还能获得真正的实时 Resource Subscription

### Negative

- **迁移成本高**：现有 ~60 个 Store 方法需要从 CRUD 改写为 Command/Event 模式，SQL 方言从 SQLite 迁移到 PostgreSQL
- **读写最终一致性**：事件写入后，Projection Builder 异步更新物化视图，存在毫秒~秒级的读取延迟
- **存储膨胀**：事件日志只增不减，需要未来引入快照（snapshot）机制来截断历史
- **部署复杂度上升**：从 SQLite 的零配置到需要嵌入/管理 PostgreSQL 实例
- **Piggyback 增加 token 消耗**：每个 tool result 附加事件摘要会增加少量 token（但远小于 `shared-memory-mcp` 之前 48K→8K 要解决的问题）
- **Orchestrator 不自动启动 Agent**：需要人类手动确认才能启动下游 Agent，在高度并发的场景下可能引入人类响应延迟——但这符合 Human Sovereignty 的宪法原则

### Neutral / Trade-offs

- **Agent 执行期间不接收更新**：如果 Agent 正在编译代码 5 分钟且期间不调用 MCP tool，它不会知道上下文变化——但这**是正确行为**，Agent 不应在执行中途被打断
- **紧急事件的响应依赖人类**：Orchestrator 通过 TUI 通知人类，由人类决定何时干预。不存在 Orchestrator 绕过人类自动执行的情况
- **短期内进程内嵌入 PostgreSQL**：不追求独立引擎，降低部署成本，但未来切换为独立实例时接口不变
- **Event Schema 演进**：事件格式变更需要版本管理，旧事件需有兼容的 deserialization 路径

---

## Relation to ADR-0004

ADR-0004 的 D6（数据库 Schema 扩展）中基于 SQLite 的表设计，在本 ADR 生效后将被逐步替换为：
- `events` 表取代直接状态表成为唯一真相来源
- 原 `task_contexts`、`agent_sessions` 等表变为 `proj_tasks`、`proj_agent_sessions` 等物化投影
- MCP 工具集（`task.*`、`session.*` 等）的读写路径从直接 SQL 改为 Command → Event → Projection 链路

ADR-0004 的 D4（调度器）中的 `OnTaskCompleted` 轮询/查表检测机制，将升级为"直接订阅内存 Event Bus 中的事件"。调度器从**被动轮询**变为**事件驱动**；同时调度器的职责从"自动启动下游 Agent"收缩为"通知人类下游任务已就绪"，人类保留创建工作树和启动 Agent 的最终决策权。

ADR-0004 的 D3（A2A + MCP 通信协议）中 A2A 的用途被重新聚焦：A2A 不再承载"上下文变化通知"（这由 Piggyback 覆盖），而是专用于**任务委托信号**和**人类干预请求**。
