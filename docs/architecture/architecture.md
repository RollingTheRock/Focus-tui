# Focus-tui 完整架构设计

基于 [ADR-0004: Protocol-Driven Multi-Agent Orchestration with External Terminal Execution](../adr/0004-protocol-driven-multi-agent-orchestration.md)。

---

## 目录

1. [概述](#概述)
2. [架构原则](#架构原则)
3. [系统全景图](#系统全景图)
4. [组件架构](#组件架构)
5. [数据流](#数据流)
6. [状态机](#状态机)
7. [数据库设计](#数据库设计)
8. [接口契约](#接口契约)
9. [安全与隔离](#安全与隔离)
10. [部署模式](#部署模式)
11. [演进路径](#演进路径)

---

## 概述

Focus-tui 是一个**人类主权的 Agent 原生开发者工作台**。它不试图成为 Agent 的运行容器，而是成为 Agent 的**指挥调度中心**。

**核心洞察**：现代 Agent CLI（Claude Code、OpenCode、Kimi CLI、Codex CLI）已经提供了足够强大的 Agent 运行环境。Focus-tui 不需要重新发明 Agent 运行时，它只需要：
1. 统一调度多个 Agent 的执行
2. 让 Agent 之间自动共享上下文
3. 让人类在任何时候都能直接干预

### 架构定位

```
┌─────────────────────────────────────────────────────────────────┐
│  传统 AI IDE（如 Cursor）                                         │
│  └── Agent 是 IDE 的插件，受限于 IDE 的 UI                       │
├─────────────────────────────────────────────────────────────────┤
│  Global Agent 架构（ADR-0002/0003）                               │
│  └── 自建 Agent Loop，试图替代现有 Agent CLI                      │
├─────────────────────────────────────────────────────────────────┤
│  本架构（ADR-0004）                                               │
│  └── Focus-tui 是指挥中心，Agent CLI 是独立执行单元               │
│  └── Agent 在外部终端自由运行，通过协议与 Focus-tui 协作          │
└─────────────────────────────────────────────────────────────────┘
```

---

## 架构原则

### P1. Agent 优先于封装

Agent 应该在最原生的环境中运行。Claude Code 在 kitty 中运行，比在 TUI 的 Pane 里运行体验更好。Focus-tui 不封装 Agent，而是协调 Agent。

### P2. 协议优先于接口

Agent 与 Focus-tui 的通信通过标准协议（MCP + A2A），不是函数调用。这意味着：
- Agent 可以用任何语言实现
- Agent 可以运行在任何机器上
- Focus-tui 和 Agent 可以独立升级

### P3. 人类直接操作优先于代理中转

人类通过 TUI 直接读写 MCP 共享状态，不通过任何 Agent 中转。这是"人类主权"的技术保证。

### P4. 调度器必须愚蠢

调度器是规则引擎，不是 AI。它只检测状态变化并触发下游，不做智能决策。智能应该留在 Agent 内部。

### P5. 上下文自动流动

Agent 产出自动写入 Knowledge Graph，下游 Agent 自动读取。不需要人类做"搬运工"。

---

## 系统全景图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Human（人类操作者）                              │
│                                                                             │
│  ┌──────────────────────┐              ┌────────────────────────────────┐  │
│  │ Focus-tui Dashboard  │◄────────────►│ 外部终端窗口（多个 Agent）      │  │
│  │ （指挥中心）          │   Alt-Tab    │                                │  │
│  └──────────────────────┘              │  Terminal 1: Claude            │  │
│         │                              │  Terminal 2: OpenCode          │  │
│         │ MCP / A2A                    │  Terminal 3: Kimi              │  │
│         ▼                              └────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         Focus-tui 进程                              │  │
│  │                                                                      │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │  │
│  │  │ TUI Layer    │  │ Orchestrator │  │ Agent Registry           │  │  │
│  │  │ （Bubble Tea）│  │ （规则引擎）  │  │ （发现 + 生命周期）       │  │  │
│  │  └──────────────┘  └──────────────┘  └──────────────────────────┘  │  │
│  │         │                   │                        │              │  │
│  │         └───────────────────┼────────────────────────┘              │  │
│  │                             │                                       │  │
│  │  ┌──────────────────────────┴──────────────────────────┐           │  │
│  │  │              Communication Layer                       │           │  │
│  │  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐  │           │  │
│  │  │  │ MCP Server │  │ A2A Router │  │ Event Bus      │  │           │  │
│  │  │  │ (JSON-RPC) │  │ (Signals)  │  │ (SQLite trigger│  │           │  │
│  │  │  └────────────┘  └────────────┘  │  + file watch) │  │           │  │
│  │  └─────────────────────────────────────────────────────┘           │  │
│  │                             │                                       │  │
│  │  ┌──────────────────────────┴──────────────────────────┐           │  │
│  │  │              Persistence Layer                         │           │  │
│  │  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐  │           │  │
│  │  │  │ SQLite     │  │ WAL Mode   │  │ Migration      │  │           │  │
│  │  │  │ (15+ 表)   │  │ (并发读写)  │  │ Manager        │  │           │  │
│  │  │  └────────────┘  └────────────┘  └────────────────┘  │           │  │
│  │  └─────────────────────────────────────────────────────┘           │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 组件架构

### 1. Focus-tui（指挥中心）

**职责**：提供人类操作界面，显示调度状态，发送控制指令。

**不承担的职责**：
- ❌ 不渲染 Agent 输出
- ❌ 不运行 Agent 进程
- ❌ 不做智能决策

**Pane 类型**：

| Pane | 内容 | 数据来源 |
|------|------|---------|
| **Dashboard** | 当前 Plan 进度、整体状态 | `task_contexts` + `task_plans` |
| **Task Board** | 任务列表、DAG 依赖图 | `task_contexts` + `task_dependencies` |
| **Agent Grid** | Agent 状态卡片（名称、状态、当前任务） | `agent_sessions` |
| **ADR Browser** | ADR 列表、约束查询 | `adrs` + `adr_constraints` |
| **Knowledge Graph** | 知识事实浏览、搜索 | `knowledge_facts` |
| **Human Shell** | 人类自己的工作区 Shell | `creack/pty` |
| **Message Log** | A2A 消息历史 | `agent_messages` |

**Agent 卡片 UI**：

```
┌─────────────────────────────────┐
│ 🤖 Claude              [运行中]  │
├─────────────────────────────────┤
│ Plan: API 重构                    │
│ Step 3: 实现 handler             │
│ Worktree: /wt/plan-123           │
│                                  │
│ 进度: 分析接口 ▶ 生成代码          │
│ 最后心跳: 2s 前                   │
│                                  │
│ [聚焦窗口] [停止] [接管] [日志]    │
└─────────────────────────────────┘
```

### 2. MCP Server（共享状态）

**职责**：将 SQLite Store 暴露为 MCP（Model Context Protocol）接口，供 Agent 和人类操作。

**设计**：
- 基于 JSON-RPC 2.0 over Unix Socket
- 每个 Agent 启动时通过环境变量 `FOCUS_MCP_SOCKET` 连接
- Focus-tui TUI 层也通过 MCP Client 读写（人类和 Agent 同一接口）

**MCP Resources**：

| Resource | URI | 内容 |
|----------|-----|------|
| ADR Registry | `mcp://adr-registry` | 所有 Accepted ADR |
| ADR Detail | `mcp://adr-registry/{id}` | 单条 ADR 全文 |
| Task Board | `mcp://task-board/{plan_id}` | Plan 下所有 Task |
| Task Detail | `mcp://task-board/{task_id}` | 单条 Task 详情 |
| Knowledge Graph | `mcp://kg/{plan_id}` | Plan 相关知识 |
| Session State | `mcp://session/{session_id}` | Agent Session 状态 |

**MCP Tools**：

| Tool | 权限 | 说明 |
|------|------|------|
| `adr.list` | 只读 | 列出 ADR |
| `adr.get` | 只读 | 获取单条 ADR |
| `adr.get_constraints` | 只读 | 获取 ADR 约束列表 |
| `task.list` | 只读 | 列出 Task |
| `task.get` | 只读 | 获取 Task 详情 |
| `task.update_status` | 读写 | 更新 Task 状态（Agent 可标记完成） |
| `task.create_output` | 读写 | 写入 Task 产出 |
| `kg.query` | 只读 | 查询 Knowledge Graph |
| `kg.add_fact` | 读写 | 添加知识事实 |
| `session.heartbeat` | 读写 | Session 心跳 |
| `session.request_intervention` | 读写 | 请求人类干预 |
| `context.get_for_task` | 只读 | 获取任务所需的全部上下文 |

### 3. A2A Router（信号层）

**职责**：轻量信号路由，不传输数据，只传递引用和信号。

**设计**：
- 基于 Unix Socket 或本地消息队列
- 消息体极小（< 1KB），只包含信号类型和 ID 引用
- 所有消息持久化到 `agent_messages` 表

**A2A 消息格式**：

```json
{
  "id": "msg-uuid",
  "from": "agent-claude-abc123",
  "to": "orchestrator",
  "type": "status_update",
  "payload": {
    "task_id": "task-456",
    "status": "completed",
    "output_ref": "mcp://task-board/task-456/output"
  },
  "timestamp": "2026-04-28T12:00:00Z"
}
```

**A2A 消息类型**：

| 类型 | 方向 | 说明 |
|------|------|------|
| `task.delegation` | Orchestrator → Agent | 分配新任务 |
| `status.update` | Agent → Orchestrator | 报告状态变化 |
| `intervention.request` | Agent → Human | 请求人类干预 |
| `intervention.response` | Human → Agent | 人类干预响应 |
| `artifact.reference` | Agent → Agent | 产物引用（通过 MCP URI） |
| `session.heartbeat` | Agent → Orchestrator | 周期性心跳 |

### 4. Orchestrator（调度器）

**职责**：纯粹的规则引擎，检测 Task Board 状态变化并触发下游。

**核心规则**：

```go
type Orchestrator struct {
    store     *store.Store
    registry  *agents.Registry
    launcher  *agents.Launcher
}

// 规则 1：任务完成 → 启动下游
func (o *Orchestrator) OnTaskCompleted(taskID string) {
    downstream := o.store.GetDownstreamTasks(taskID)
    for _, next := range downstream {
        if o.allPrerequisitesMet(next.ID) {
            o.launchAgent(next)
        }
    }
}

// 规则 2：任务失败 → 通知人类，阻塞下游
func (o *Orchestrator) OnTaskFailed(taskID string, reason string) {
    o.store.BlockDownstreamTasks(taskID)
    o.notifyHuman("任务失败，需要干预", taskID, reason)
}

// 规则 3：心跳超时 → 标记 Agent 失联
func (o *Orchestrator) OnHeartbeatTimeout(sessionID string) {
    o.store.UpdateSessionState(sessionID, "disconnected")
    o.notifyHuman("Agent 失联", sessionID, "")
}

// 规则 4：Plan 所有 Task 完成 → 归档 Plan
func (o *Orchestrator) OnPlanCompleted(planID string) {
    if o.allTasksDone(planID) {
        o.store.UpdatePlanStatus(planID, "completed")
    }
}
```

**调度器不做的事**：
- ❌ 不解析 Agent 输出内容
- ❌ 不决定"下一步做什么"（这是 Plan 层的职责）
- ❌ 不持有 Agent 上下文
- ❌ 不直接操作 Agent 进程（只通过 Launcher 启动，通过 A2A 发信号）

### 5. Agent Registry + Launcher

**Registry 职责**：
- 维护当前所有活跃 Agent Session 的内存索引
- 通过 `pgrep` + 环境变量发现外部运行的 Agent
- 提供按 Provider / Worktree / State 的过滤查询

**Launcher 职责**：
- 根据配置调用终端模拟器 CLI 启动 Agent
- 设置环境变量（`FOCUS_SESSION_ID`, `FOCUS_MCP_SOCKET` 等）
- 支持自动检测终端模拟器（kitty → alacritty → wezterm → gnome-terminal）

```go
type Launcher struct {
    terminalEmulator string  // 如 "kitty"
    mcpSocket        string  // /tmp/focus-mcp.sock
    a2aSocket        string  // /tmp/focus-a2a.sock
}

func (l *Launcher) Launch(task Task, provider string) (*Session, error) {
    sessionID := generateID()
    
    envVars := []string{
        fmt.Sprintf("FOCUS_SESSION_ID=%s", sessionID),
        fmt.Sprintf("FOCUS_TASK_ID=%s", task.ID),
        fmt.Sprintf("FOCUS_PLAN_ID=%s", task.PlanID),
        fmt.Sprintf("FOCUS_MCP_SOCKET=%s", l.mcpSocket),
        fmt.Sprintf("FOCUS_A2A_SOCKET=%s", l.a2aSocket),
    }
    
    cmd := l.buildTerminalCommand(
        title: fmt.Sprintf("Focus:%s:%s", task.PlanID, task.Name),
        directory: task.WorktreeID,
        env: envVars,
        command: ProviderCommand(provider),
    )
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    return &Session{
        ID:        sessionID,
        Provider:  provider,
        TaskID:    task.ID,
        PlanID:    task.PlanID,
        PID:       cmd.Process.Pid,
        State:     "starting",
    }, nil
}
```

### 6. Compliance Checker（合规检查器）

**职责**：验证 Agent 产出是否违反 ADR 约束。

**检查类型**：

| 类型 | 时机 | 说明 |
|------|------|------|
| **静态检查** | Agent 启动前 | 检查 worktree 结构、文件命名等 |
| **语义检查** | Agent 产出写入后 | 检查代码是否违反 ADR 约束（如"必须使用接口"） |
| **漂移检测** | 周期性 | 检测已有代码是否偏离已接受的 ADR |

**实现**：
- 静态检查：基于 AST / 正则的规则引擎
- 语义检查：调用 LLM 判断（ lightweight，只传入 ADR constraints + 产出片段）
- 漂移检测：Git diff + 规则匹配

### 7. Human Interface（TUI）

**职责**：人类与系统的交互界面。

**核心交互**：

| 操作 | TUI 行为 | 底层调用 |
|------|---------|---------|
| 审批 ADR | 点击"接受"按钮 | `adr.update_status(id, "accepted", "human-zhang")` |
| 修改 Task | 编辑 Task 详情面板 | `task.update(taskID, updates, "human-zhang")` |
| 启动 Agent | 点击 Task 上的"启动"按钮 | Orchestrator.launchAgent(task) |
| 停止 Agent | 点击 Agent 卡片上的"停止" | `kill -TERM {pid}` |
| 聚焦 Agent 窗口 | 点击"聚焦窗口" | `xdotool` / `swaymsg` / AppleScript |
| 接管 Agent | 点击"接管" | 切换到 Agent 终端窗口直接操作 |
| 查看产出 | 点击 Task 上的"查看产出" | 从 MCP 读取并显示在面板 |

---

## 数据流

### 流 1：Agent 启动流程

```
人类在 TUI 点击"启动 Agent"
    │
    ▼
TUI 调用 Orchestrator.launchAgent(taskID)
    │
    ▼
Orchestrator 查询 Task 详情 + 关联 ADR constraints
    │
    ▼
Launcher 构建终端启动命令
    │
    ▼
终端模拟器打开新窗口，运行 Agent CLI
    │
    ▼
Agent 进程启动，读取环境变量
    │
    ├─► 连接 MCP Server（注册自己）
    ├─► 连接 A2A Router（开始监听信号）
    └─► 调用 context.get_for_task() 获取上下文
    │
    ▼
Agent 开始执行任务
    │
    ▼
Agent 定期发送心跳（A2A session.heartbeat）
```

### 流 2：任务完成 → 启动下游

```
Agent 完成任务
    │
    ▼
Agent 调用 task.update_status(taskID, "completed")
    │
    ▼
MCP Server 更新 SQLite
    │
    ▼
SQLite trigger 发布变更事件
    │
    ▼
Orchestrator 收到 OnTaskCompleted 事件
    │
    ▼
Orchestrator 查询 task_dependencies 表
    │
    ▼
发现 Task-B 依赖 Task-A（刚完成）
    │
    ▼
检查 Task-B 的所有前置依赖是否都已完成
    │
    ▼
是 → Launcher 启动 Task-B 的 Agent
    │
    ▼
Task-B Agent 通过 MCP 读取 Task-A 的产出（自动上下文共享）
```

### 流 3：人类干预流程

```
Agent 遇到无法处理的情况
    │
    ▼
Agent 调用 session.request_intervention("需要人类确认接口设计")
    │
    ▼
A2A Router 转发给 Orchestrator
    │
    ▼
Orchestrator 标记 Task 状态为 "blocked"
    │
    ▼
TUI Dashboard 显示 "需要干预" 警告
    │
    ▼
人类看到警告，点击"查看详情"
    │
    ▼
人类切换到 Agent 终端窗口，直接输入指令
    │
    ▼
Agent 收到人类输入，继续执行
    │
    ▼
Agent 调用 task.update_status(taskID, "in_progress")
```

### 流 4：知识沉淀与复用

```
Agent-1（架构设计）分析出关键决策
    │
    ▼
Agent-1 调用 kg.add_fact(
    subject="handler层",
    predicate="应该使用",
    object="接口而非具体实现",
    source="agent-claude-abc123"
)
    │
    ▼
MCP Server 写入 knowledge_facts 表
    │
    ▼
Agent-2（代码实现）启动时调用 context.get_for_task()
    │
    ▼
MCP Server 自动查询 knowledge_facts，返回相关事实
    │
    ▼
Agent-2 的 prompt 自动包含："根据架构设计，handler 层应该使用接口而非具体实现"
```

---

## 状态机

### Task 状态机

```
                    ┌─────────────┐
         ┌─────────►│   created   │◄────────┐
         │          └──────┬──────┘         │
         │                 │ start          │ human edits
         │                 ▼                │
         │          ┌─────────────┐         │
         │    ┌────►│  in_progress│◄────────┘
         │    │     └──────┬──────┘
         │    │            │
         │    │    ┌───────┼───────┐
         │    │    │       │       │
         │    │    ▼       ▼       ▼
         │    │ ┌──────┐ ┌─────┐ ┌──────┐
         │    │ │paused│ │blocked│ │completed│
         │    │ └──┬───┘ └──┬──┘ └──┬───┘
         │    │    │ resume │       │
         │    └────┘        │       │
         │                  │       │
         │                  ▼       ▼
         │               ┌─────────────┐
         │               │   archived  │
         │               └─────────────┘
         │
         └────────────────────────────────┘
              (Orchestrator auto-starts downstream)
```

| 状态 | 说明 | 触发条件 |
|------|------|---------|
| `created` | 已创建，等待启动 | Plan 被批准 |
| `in_progress` | 正在执行 | Agent 启动 |
| `paused` | 暂停（人类主动） | 人类点击暂停 |
| `blocked` | 阻塞（需要干预） | Agent 请求干预 / 前置依赖失败 |
| `completed` | 已完成 | Agent 标记完成 |
| `archived` | 已归档 | Plan 完成后自动归档 |

### Agent Session 状态机

```
┌─────────┐    start     ┌──────────┐   heartbeat   ┌─────────┐
│ starting│─────────────►│ running  │◄─────────────►│ healthy │
└─────────┘              └────┬─────┘               └─────────┘
                              │
                    ┌─────────┼─────────┐
                    │         │         │
                    ▼         ▼         ▼
              ┌────────┐ ┌────────┐ ┌────────┐
              │paused  │ │blocked │ │completed│
              └───┬────┘ └───┬────┘ └────┬───┘
                  │          │           │
                  ▼          ▼           ▼
              ┌─────────────────────────────┐
              │         stopped             │
              └─────────────────────────────┘
```

| 状态 | 说明 |
|------|------|
| `starting` | Launcher 已启动进程，Agent 尚未注册 |
| `running` | Agent 正常运行中 |
| `healthy` | Agent 心跳正常（running 的子状态） |
| `paused` | Agent 被人类暂停 |
| `blocked` | Agent 等待人类干预 |
| `completed` | Agent 已完成任务 |
| `stopped` | Agent 进程已退出 |
| `disconnected` | 心跳超时，Agent 可能崩溃 |

### Plan 状态机

```
┌────────┐   approve   ┌────────┐  start   ┌────────┐
│ draft  │────────────►│approved│─────────►│ active │
└────────┘             └────────┘          └───┬────┘
                                               │
                    ┌──────────────────────────┼──────┐
                    │                          │      │
                    ▼                          ▼      ▼
              ┌─────────┐              ┌──────────┐ ┌──────────┐
              │discarded│              │ completed│ │ archived │
              └─────────┘              └──────────┘ └──────────┘
```

| 状态 | 说明 |
|------|------|
| `draft` | 草稿，人类编辑中 |
| `approved` | 已批准，等待执行 |
| `active` | 正在执行中 |
| `completed` | 所有 Task 完成 |
| `discarded` | 被丢弃 |
| `archived` | 已归档 |

---

## 数据库设计

### 完整 Schema

```sql
-- =====================================================
-- 1. ADR 层（宪法约束）
-- =====================================================

CREATE TABLE adrs (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    status          TEXT NOT NULL CHECK(status IN ('proposed', 'accepted', 'deprecated', 'superseded')),
    version         INTEGER NOT NULL DEFAULT 1,
    context         TEXT NOT NULL,
    decision        TEXT NOT NULL,
    consequences    TEXT,
    superseded_by   TEXT REFERENCES adrs(id),
    created_by      TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accepted_at     DATETIME,
    accepted_by     TEXT,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE adr_constraints (
    id          TEXT PRIMARY KEY,
    adr_id      TEXT NOT NULL REFERENCES adrs(id) ON DELETE CASCADE,
    category    TEXT NOT NULL CHECK(category IN ('must', 'must_not', 'should', 'should_not')),
    rule        TEXT NOT NULL,
    rationale   TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================
-- 2. Plan 层（任务规划）
-- =====================================================

CREATE TABLE task_plans (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'draft' 
                        CHECK(status IN ('draft', 'approved', 'active', 'completed', 'discarded', 'archived')),
    adr_id          TEXT REFERENCES adrs(id),
    created_by      TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME,
    completed_at    DATETIME,
    version         INTEGER NOT NULL DEFAULT 1
);

-- =====================================================
-- 3. Task 层（执行单元）
-- =====================================================

CREATE TABLE task_contexts (
    id              TEXT PRIMARY KEY,
    plan_id         TEXT NOT NULL REFERENCES task_plans(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    state           TEXT NOT NULL DEFAULT 'created' 
                        CHECK(state IN ('created', 'in_progress', 'paused', 'blocked', 'completed', 'archived')),
    assigned_to     TEXT,  -- agent provider or 'human-zhang'
    worktree_id     TEXT REFERENCES worktree_contexts(id),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME,
    completed_at    DATETIME,
    output_summary  TEXT,
    version         INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE task_dependencies (
    from_task_id    TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    to_task_id      TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    dependency_type TEXT NOT NULL DEFAULT 'hard' CHECK(dependency_type IN ('hard', 'soft')),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (from_task_id, to_task_id)
);

-- =====================================================
-- 4. Step 层（线性执行）
-- =====================================================

CREATE TABLE plan_steps (
    id              TEXT PRIMARY KEY,
    plan_id         TEXT NOT NULL REFERENCES task_plans(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    state           TEXT NOT NULL DEFAULT 'pending' 
                        CHECK(state IN ('pending', 'in_progress', 'blocked', 'done', 'invalidated')),
    order_index     INTEGER NOT NULL,
    assigned_to     TEXT,
    output_summary  TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME,
    completed_at    DATETIME
);

-- =====================================================
-- 5. Session 层（Agent 实例）
-- =====================================================

CREATE TABLE agent_sessions (
    id              TEXT PRIMARY KEY,
    provider        TEXT NOT NULL,
    worktree_id     TEXT REFERENCES worktree_contexts(id),
    task_id         TEXT REFERENCES task_contexts(id),
    plan_id         TEXT REFERENCES task_plans(id),
    step_id         TEXT REFERENCES plan_steps(id),
    state           TEXT NOT NULL DEFAULT 'starting' 
                        CHECK(state IN ('starting', 'running', 'paused', 'blocked', 'completed', 'stopped', 'disconnected')),
    pid             INTEGER,
    env_snapshot    TEXT,  -- JSON: {FOCUS_SESSION_ID, FOCUS_TASK_ID, ...}
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_heartbeat  DATETIME,
    stopped_at      DATETIME,
    stop_reason     TEXT
);

-- =====================================================
-- 6. Worktree 层（执行环境）
-- =====================================================

CREATE TABLE worktree_contexts (
    id              TEXT PRIMARY KEY,
    path            TEXT NOT NULL UNIQUE,
    branch          TEXT,
    primary_task_id TEXT REFERENCES task_contexts(id),
    task_mode       TEXT NOT NULL DEFAULT 'plan' CHECK(task_mode IN ('plan', 'session')),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_accessed   DATETIME
);

-- =====================================================
-- 7. Handoff 层（会话交接）
-- =====================================================

CREATE TABLE session_handoffs (
    id              TEXT PRIMARY KEY,
    from_session_id TEXT NOT NULL REFERENCES agent_sessions(id),
    to_session_id   TEXT REFERENCES agent_sessions(id),
    handoff_type    TEXT NOT NULL CHECK(handoff_type IN ('pause_resume', 'checkpoint', 'agent_switch')),
    context_payload TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================
-- 8. Agent Mesh 扩展表
-- =====================================================

CREATE TABLE agent_messages (
    id          TEXT PRIMARY KEY,
    from_agent  TEXT NOT NULL,
    to_agent    TEXT NOT NULL,
    msg_type    TEXT NOT NULL CHECK(msg_type IN ('task_delegation', 'artifact_reference', 'status_update', 'intervention_request', 'intervention_response')),
    payload     TEXT NOT NULL,
    read_at     DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE knowledge_facts (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT REFERENCES task_plans(id),
    subject     TEXT NOT NULL,
    predicate   TEXT NOT NULL,
    object      TEXT NOT NULL,
    source      TEXT NOT NULL,
    confidence  REAL DEFAULT 1.0 CHECK(confidence >= 0 AND confidence <= 1),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_knowledge_plan ON knowledge_facts(plan_id);
CREATE INDEX idx_knowledge_subject ON knowledge_facts(subject);

CREATE TABLE compliance_checks (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES agent_sessions(id),
    adr_id      TEXT NOT NULL REFERENCES adrs(id),
    check_type  TEXT NOT NULL CHECK(check_type IN ('static', 'semantic', 'drift')),
    passed      BOOLEAN NOT NULL,
    issues      TEXT,
    checked_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================
-- 9. 索引优化
-- =====================================================

CREATE INDEX idx_task_plan ON task_contexts(plan_id);
CREATE INDEX idx_task_state ON task_contexts(state);
CREATE INDEX idx_task_assigned ON task_contexts(assigned_to);
CREATE INDEX idx_session_task ON agent_sessions(task_id);
CREATE INDEX idx_session_plan ON agent_sessions(plan_id);
CREATE INDEX idx_session_state ON agent_sessions(state);
CREATE INDEX idx_adr_status ON adrs(status);
CREATE INDEX idx_messages_from ON agent_messages(from_agent);
CREATE INDEX idx_messages_to ON agent_messages(to_agent);
CREATE INDEX idx_messages_type ON agent_messages(msg_type);
CREATE INDEX idx_deps_from ON task_dependencies(from_task_id);
CREATE INDEX idx_deps_to ON task_dependencies(to_task_id);
```

---

## 接口契约

### MCP Server API

#### Resources（只读，URI 访问）

```
GET mcp://adr-registry
→ 返回所有 Accepted ADR 列表

GET mcp://adr-registry/{adr_id}
→ 返回单条 ADR 完整内容（含 constraints）

GET mcp://task-board/{plan_id}
→ 返回 Plan 下所有 Task 列表（含状态）

GET mcp://task-board/{task_id}
→ 返回单条 Task 详情（含依赖关系）

GET mcp://kg/{plan_id}?subject={s}&predicate={p}
→ 返回 Knowledge Graph 查询结果

GET mcp://session/{session_id}
→ 返回 Session 状态、心跳时间
```

#### Tools（读写，JSON-RPC 调用）

```json
// adr.list
{"method": "adr.list", "params": {"status": "accepted"}}
→ {"adrs": [{"id": "0004", "title": "...", "status": "accepted"}]}

// adr.get_constraints
{"method": "adr.get_constraints", "params": {"adr_id": "0004"}}
→ {"constraints": [{"category": "must", "rule": "..."}]}

// task.update_status
{"method": "task.update_status", "params": {"task_id": "t1", "state": "completed", "actor": "agent-claude"}}
→ {"success": true, "version": 3}

// task.create_output
{"method": "task.create_output", "params": {"task_id": "t1", "output": "...", "actor": "agent-claude"}}
→ {"success": true, "output_id": "o1"}

// kg.add_fact
{"method": "kg.add_fact", "params": {"plan_id": "p1", "subject": "handler", "predicate": "uses", "object": "interface", "source": "agent-claude"}}
→ {"success": true, "fact_id": "f1"}

// context.get_for_task
{"method": "context.get_for_task", "params": {"task_id": "t1"}}
→ {
    "adr_constraints": [...],
    "upstream_outputs": [...],
    "knowledge_facts": [...],
    "worktree_info": {...}
  }

// session.heartbeat
{"method": "session.heartbeat", "params": {"session_id": "s1", "status": "running"}}
→ {"success": true}

// session.request_intervention
{"method": "session.request_intervention", "params": {"session_id": "s1", "reason": "需要确认接口设计"}}
→ {"success": true, "intervention_id": "i1"}
```

### A2A 消息协议

```typescript
interface A2AMessage {
    id: string;           // UUID
    from: string;         // sender agent_id or "orchestrator" or "human"
    to: string;           // recipient agent_id or "broadcast"
    type: A2AMessageType;
    payload: object;
    timestamp: string;    // ISO 8601
}

type A2AMessageType =
    | "task.delegation"        // 分配任务
    | "task.revocation"        // 撤销任务
    | "status.update"          // 状态更新
    | "status.heartbeat"       // 心跳
    | "intervention.request"   // 请求干预
    | "intervention.response"  // 干预响应
    | "artifact.reference"     // 产物引用
    | "system.shutdown";       // 系统关闭通知

// 示例：任务委托
{
    "id": "a2a-001",
    "from": "orchestrator",
    "to": "agent-claude-abc123",
    "type": "task.delegation",
    "payload": {
        "task_id": "task-456",
        "task_ref": "mcp://task-board/task-456",
        "priority": "normal",
        "deadline": "2026-04-29T00:00:00Z"
    },
    "timestamp": "2026-04-28T12:00:00Z"
}

// 示例：状态更新
{
    "id": "a2a-002",
    "from": "agent-claude-abc123",
    "to": "orchestrator",
    "type": "status.update",
    "payload": {
        "task_id": "task-456",
        "session_state": "completed",
        "task_state": "completed",
        "output_ref": "mcp://task-board/task-456/output",
        "summary": "完成了 handler 层的接口设计"
    },
    "timestamp": "2026-04-28T12:30:00Z"
}
```

### 环境变量规范

| 变量名 | 必填 | 说明 |
|--------|------|------|
| `FOCUS_SESSION_ID` | 是 | Agent Session 唯一标识，由 Launcher 生成 |
| `FOCUS_TASK_ID` | 是 | 当前绑定任务的 ID |
| `FOCUS_PLAN_ID` | 是 | 当前绑定 Plan 的 ID |
| `FOCUS_MCP_SOCKET` | 是 | MCP Server Unix socket 路径 |
| `FOCUS_A2A_SOCKET` | 是 | A2A Router Unix socket 路径 |
| `FOCUS_ADR_CONSTRAINTS` | 否 | 当前 Plan 关联的 ADR 约束 JSON（可选，Agent 可通过 MCP 查询） |

---

## 安全与隔离

### 进程隔离

- 每个 Agent 是独立 OS 进程，由操作系统调度
- Agent 进程以启动 Focus-tui 的同一用户身份运行
- Agent 崩溃不影响 Focus-tui 或其他 Agent

### Worktree 隔离

- 每个 Plan/Step 有独立的 Git worktree
- Agent 只能访问自己被分配的 worktree
- 通过 Git worktree 机制实现文件系统隔离（不是容器，但足够安全）

### 数据隔离

- MCP 权限模型：Agent 只能读写自己被授权的 Task / Plan
- `context.get_for_task()` 自动过滤，只返回必要信息
- Agent 不能直接修改 ADR（只读约束）

### 通信安全

- MCP 和 A2A 都通过本地 Unix Socket 通信，不暴露网络端口
- 未来支持远程 Agent 时，MCP over TCP 需添加 TLS + Token 认证

---

## 部署模式

### 模式 1：本地单用户（默认）

```
┌─────────────────────────────────────┐
│  单台开发机器                        │
│                                     │
│  ┌──────────────┐                   │
│  │ Focus-tui    │                   │
│  │  + SQLite    │                   │
│  │  + MCP/A2A   │                   │
│  └──────┬───────┘                   │
│         │ Unix Socket               │
│  ┌──────┴───────┐                   │
│  │ 多个 Agent    │                   │
│  │ 终端窗口      │                   │
│  └──────────────┘                   │
└─────────────────────────────────────┘
```

- SQLite 本地文件
- Unix Socket 本地通信
- Agent 在同一机器上运行

### 模式 2：本地多用户（共享）

```
┌─────────────────────────────────────┐
│  共享开发服务器                      │
│                                     │
│  ┌──────────────┐                   │
│  │ MCP Server   │◄──── 用户 A       │
│  │ (TCP 端口)   │◄──── 用户 B       │
│  │  + SQLite    │                   │
│  └──────────────┘                   │
│         │                           │
│  每个用户有自己的 Focus-tui 实例     │
│  和独立的 Agent 终端窗口             │
└─────────────────────────────────────┘
```

- MCP Server 暴露 TCP 端口
- 每个用户有独立 SQLite 数据库（或按用户分区）
- 未来扩展

### 模式 3：远程 Agent（未来）

```
┌──────────────┐      TLS      ┌─────────────────┐
│  Focus-tui   │◄─────────────►│  Remote Agent   │
│  (本地)       │   MCP/A2A    │  (云服务器)      │
└──────────────┘              └─────────────────┘
```

- MCP over TCP + TLS
- A2A over WebSocket + TLS
- Agent 运行在远程机器（如 GPU 服务器）

---

## 演进路径

### Phase 1：MVP（当前目标）

- [ ] MCP Server 骨架（SQLite Store 包装）
- [ ] A2A Router（本地 Socket）
- [ ] Orchestrator（规则引擎）
- [ ] Agent Launcher（外部终端启动）
- [ ] Dashboard Pane（Agent 状态卡片）
- [ ] Task Board Pane（任务列表）
- [ ] 数据库 Schema 扩展（task_dependencies, knowledge_facts, agent_messages, adr_constraints）

### Phase 2：完整调度

- [ ] DAG 调度（并行执行独立 Task）
- [ ] Compliance Checker（静态 + 语义检查）
- [ ] Knowledge Graph 自动沉淀
- [ ] 人类审批门（TUI 交互）
- [ ] Session Handoff（Agent 间交接上下文）

### Phase 3：高级功能

- [ ] 窗口聚焦 API（跨平台）
- [ ] 远程 Agent 支持（MCP over TCP）
- [ ] Agent 性能指标收集
- [ ] Plan 模板系统
- [ ] 可视化依赖图（DAG 图形渲染）

### Phase 4：生态

- [ ] 自定义 Agent Provider（Python/TypeScript SDK）
- [ ] Agent 市场（可复用的 Agent 模板）
- [ ] 团队协作（多用户共享 Plan）
- [ ] CI/CD 集成（Agent 执行作为流水线步骤）

---

## 附录

### A. 术语表

| 术语 | 定义 |
|------|------|
| **ADR** | Architecture Decision Record，架构决策记录 |
| **Plan** | 任务计划，由 ADR 分解而来，包含多个 Task |
| **Task** | 执行单元，可被分配给 Agent 或人类 |
| **Session** | Agent 进程实例，对应一个运行中的 Agent |
| **Worktree** | Git worktree，独立的代码工作区 |
| **MCP** | Model Context Protocol，共享状态协议 |
| **A2A** | Agent-to-Agent，轻量信号协议 |
| **Orchestrator** | 调度器，纯规则引擎 |
| **Compliance** | 合规检查，验证 Agent 产出是否违反 ADR |

### B. 与现有代码的映射

| 新模块 | 现有代码基础 | 改动说明 |
|--------|------------|---------|
| `internal/mcp/server.go` | `internal/store/*.go` | 新增：包装 Store 为 MCP Server |
| `internal/a2a/router.go` | 无 | 全新模块 |
| `internal/orchestrator/orchestrator.go` | 无 | 全新模块，替换 Global Agent 概念 |
| `internal/agents/launcher.go` | `internal/agents/launcher.go` | 修改：从 `tea.ExecProcess` 改为外部终端 CLI |
| `internal/agents/discovery.go` | `internal/agents/discovery.go` | 扩展：通过环境变量发现外部 Agent |
| `internal/agents/registry.go` | `internal/agents/registry.go` | 扩展：增加心跳超时检测 |
| `internal/ui/dashboard.go` | `internal/ui/` | 新增：Agent 状态卡片、任务板 |
| `internal/compliance/checker.go` | 无 | 全新模块 |
| `internal/store/` | `internal/store/*.go` | 扩展：新增 task_dependencies 等表 |

### C. 参考资料

- [ADR-0000](../adr/0000-constitution-for-adr-driven-execution.md)
- [ADR-0001](../adr/0001-product-positioning-human-sovereign-agent-native-workbench.md)
- [ADR-0004](../adr/0004-protocol-driven-multi-agent-orchestration.md)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [Agent-to-Agent Protocol](https://github.com/google/A2A)
- [Git Worktrees](https://git-scm.com/docs/git-worktree)
