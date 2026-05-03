# ADR-0004: Protocol-Driven Multi-Agent Orchestration with External Terminal Execution

- **Date:** 2026-04-28
- **Status:** Accepted
- **Supersedes:** ADR-0002, ADR-0003

---

## Context

### 1. ADR-0002/0003 的问题

ADR-0002 提出了 **Global Agent** 架构：一个 in-process Go Agent 辅助人类完成 ADR→Plan→Task→Session 分解。ADR-0003 进一步钉定了技术选型（自建 Go Loop、Context 四层模型、MCP 集成）。

但经过对以下开源项目的深度调研，我们认识到 Global Agent 是一个**过渡性设计**：

- **Open Multi-Agent** (`JackChen-me/open-multi-agent`)：证明了 Coordinator Agent 分解 + DAG 并行调度是可行的，但 Coordinator 是**临时 Agent**（执行完即退出），不是持久化的 Global Agent
- **Composio Agent Orchestrator** (`ComposioHQ/agent-orchestrator`)：用**生命周期状态机**管理并行 Agent Session，每个 Agent 独立 worktree，没有中心大脑
- **MetaGPT** (`FoundationAgents/MetaGPT`)：多角色协作（产品经理→架构师→工程师），但每个角色是**独立 Agent**，通过标准化文档（事件驱动）协作，没有 Global Agent
- **OpenHands** (`All-Hands-AI/OpenHands`)：Claude Code 的开源替代，SDK/CLI/GUI/Cloud 分层，同一引擎驱动多种形态——**引擎-客户端分离**模式
- **Sema Code** (论文)：核心洞察是"完全解耦 agent engine 与 client layers"，同一引擎同时驱动 VSCode 扩展和多通道消息网关

这些项目的共同趋势是：**去中心化协议驱动**，而非中心化 Global Agent。

### 2. Focus-tui 的架构优势

Focus-tui 已经拥有以下天然优势：

- **内嵌 Shell + PTY** (`creack/pty`)：Pane 系统支持独立 Shell Session
- **Agent Registry + Launcher**：支持 OpenCode/Claude/Kimi/Codex 五种 Provider
- **SQLite 持久化 + WAL 模式**：15+ 张表，支持并发读写
- **Plugin 架构**：`Plugin` 接口 + `Registry`，Pane 类型可插拔
- **ADR→Plan→Task→Session 层级**：宪法层级已确立

**核心洞察**：Focus-tui 的**内嵌 Shell 是 Agent 容器的一种形态**，但不是唯一形态。Agent 应该在最原生的环境中运行——外部独立终端。

### 3. 关键需求

- **Agent 拥有完整的终端原生体验**，不受 TUI 尺寸和渲染限制
- **Agent 间上下文自动共享**，无需人为搬运
- **人类可直接参与任何环节**，不通过中间层
- **多 Agent 并行执行**，各自在独立 worktree/环境中
- **统一视图**：人类可以在指挥中心看到所有 Agent 状态和调度情况
- **Agent 可被完整调度**：编排器能启动、停止、监控所有 Agent

---

## Decision

### D1. 废除 Global Agent，采用去中心化 Agent Mesh

**ADR-0002/0003 的 Global Agent 被正式废除。**

取代方案：
- 每个工作流步骤是**独立 Agent 进程**（需求翻译、调研、架构设计、代码实现、检测）
- Agent 之间通过 **A2A 协议**（轻量信号）通信
- Agent 通过 **MCP 协议** 读写共享状态
- **无中心大脑**，无持久化 Agent 进程

### D2. Agent 在外部独立终端运行，Focus-tui 作为纯指挥中心

Agent 是**完全独立的进程**，运行在用户的外部终端中，不是 Focus-tui 的内嵌组件。

**启动方式 A：Focus-tui 自动启动（默认）**

```
Focus-tui 编排器决定启动 Agent
    │
    ▼
通过配置的终端模拟器 CLI 启动外部终端窗口
    │
    ▼
终端窗口中运行 Agent CLI（claude / opencode / kimi / codex）
    │
    ▼
Agent 通过环境变量自动接入 Focus-tui 的 MCP + A2A
```

示例命令（kitty）：
```bash
kitty --title "Focus:[plan-name]:[step-name]" \
      --directory /worktrees/plan-123 \
      env FOCUS_SESSION_ID=abc123 \
          FOCUS_MCP_SOCKET=/tmp/focus-mcp.sock \
          FOCUS_A2A_SOCKET=/tmp/focus-a2a.sock \
          claude
```

**启动方式 B：用户手动启动（高级用户）**

```bash
# 用户在自己的终端中手动启动，自动注册到 Focus-tui
FOCUS_SESSION_ID=abc123 \
FOCUS_MCP_SOCKET=/tmp/focus-mcp.sock \
FOCUS_A2A_SOCKET=/tmp/focus-a2a.sock \
claude
```

**Focus-tui 的角色**：

| 层面 | Focus-tui 职责 | Agent 职责 |
|------|---------------|-----------|
| **进程管理** | 通过终端模拟器 CLI 启动 / 通过信号停止 | 自主运行，自行处理内部状态 |
| **显示** | 指挥中心 Dashboard（状态卡片、任务板、依赖图） | 在外部终端中显示自己的完整 ANSI 输出 |
| **交互** | 人类在 Dashboard 点击"分配任务"/"停止"/"聚焦窗口" | 人类直接在 Agent 终端窗口与 Agent CLI 交互 |
| **状态同步** | 通过 MCP 读取 Agent 写入的共享状态 | 通过 MCP 写入任务结果、知识图谱 |
| **调度** | 检测任务完成，启动下游 Agent | 执行被分配的任务 |

**关键设计**：
- Agent 进程**完全独立于 Focus-tui**，Focus-tui 崩溃不影响 Agent 运行
- Agent 拥有**完整的终端尺寸**（由终端模拟器决定，不受 TUI 限制）
- Agent 输出由**终端模拟器原生渲染**（ANSI、颜色、光标、滚动历史），Focus-tui 不处理 Agent 输出
- Focus-tui 通过 **A2A 信号 + MCP 共享状态** 与 Agent 通信，不依赖进程间管道
- Focus-tui 可通过操作系统窗口管理 API 聚焦到 Agent 窗口（可选）

### D3. 通信协议：A2A + MCP

| 协议 | 用途 | 数据量 |
|------|------|--------|
| **A2A** (Agent-to-Agent) | Agent 间信号传递：任务委托、状态通知、干预请求 | 极小（只传引用和信号） |
| **MCP** (Model Context Protocol) | Agent 与共享状态通信：读写 ADR、Task、Knowledge Graph | 按需加载 |

**A2A 消息类型**：
- `task.delegation`：任务委托（只传 Task ID，不传内容）
- `status.update`：状态更新
- `intervention.request`：人类/其他 Agent 请求干预
- `artifact.reference`：产物引用（`mcp://adr-registry/001`）

**MCP 工具集**：
- `adr.*`：ADR CRUD + 约束查询
- `task.*`：Task Board 操作
- `kg.*`：Knowledge Graph 读写
- `context.*`：上下文摘要（按需加载前置任务产出）
- `session.*`：Session 状态查询 + 干预

### D4. 调度器：极简规则引擎

调度器**不是 Agent**，是纯粹的规则引擎：

```go
type Orchestrator struct {
    store *store.Store
}

// 唯一职责：检测 Task Board 状态变化，启动下游 Agent
func (o *Orchestrator) OnTaskCompleted(taskID string) {
    // 1. 谁依赖这个刚完成的任务？
    downstream := o.store.GetDownstreamTasks(taskID)
    
    // 2. 检查每个下游任务的前置依赖是否全部满足
    for _, next := range downstream {
        if o.allPrerequisitesMet(next.ID) {
            // 3. 启动 Agent，只传 Task ID 和 MCP 地址
            o.launchAgent(next)
        }
    }
}

func (o *Orchestrator) launchAgent(task Task) {
    cmd := exec.Command("kitty",
        "--title", fmt.Sprintf("Focus:%s:%s", task.PlanID, task.Name),
        "--directory", task.WorktreeID,
        "env",
        "FOCUS_TASK_ID="+task.ID,
        "FOCUS_MCP_SOCKET=/tmp/focus-mcp.sock",
        "FOCUS_A2A_SOCKET=/tmp/focus-a2a.sock",
        ProviderCommand(task.Provider),
    )
    cmd.Start()
    
    // Agent 通过环境变量自动注册，无需管道绑定
}
```

**调度器不持有上下文，不做出智能决策，只执行规则。**

### D5. 人类通过 TUI 直接读写 MCP 共享状态

人类和 Agent 使用**同一套 MCP 接口**：

```
人类操作 TUI
    │
    ▼
TUI 调用 MCP Client
    │
    ▼
MCP Server 操作 SQLite
    │
    ▼
发布变更事件
    │
    ▼
调度器收到事件 → 触发下游
    │
    ▼
Agent 通过 MCP 读取更新
```

人类操作示例：
- 审批 ADR → `adr.update_status(id, "accepted", "human-zhang")`
- 修改 Task → `task.update(taskID, updates, "human-zhang")`
- 接管 Agent → `session.intervene(sessionID, "takeover", "human-zhang")`

人类与 Agent 的交互方式：
- **查看 Agent 输出**：直接看 Agent 的外部终端窗口（完整尺寸、自由滚动）
- **给 Agent 发指令**：在 Agent 终端窗口直接输入（原生 CLI 交互）
- **从 Focus-tui 调度**：Dashboard 点击"分配新任务"，Agent 收到 A2A 信号
- **接管 Agent**：Dashboard 点击"接管"，人类切换到 Agent 终端直接操作

### D6. 数据库 Schema 扩展

```sql
-- Agent Mesh 核心表

-- 1. ADR 结构化存储（从 markdown 升级）
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

-- 2. ADR 约束项（可执行的设计契约）
CREATE TABLE adr_constraints (
    id          TEXT PRIMARY KEY,
    adr_id      TEXT NOT NULL REFERENCES adrs(id) ON DELETE CASCADE,
    category    TEXT NOT NULL CHECK(category IN ('must', 'must_not', 'should', 'should_not')),
    rule        TEXT NOT NULL,
    rationale   TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. Task 依赖图（DAG）
CREATE TABLE task_dependencies (
    from_task_id    TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    to_task_id      TEXT NOT NULL REFERENCES task_contexts(id) ON DELETE CASCADE,
    dependency_type TEXT NOT NULL DEFAULT 'hard' CHECK(dependency_type IN ('hard', 'soft')),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (from_task_id, to_task_id)
);

-- 4. Agent 间消息日志（A2A 通信记录）
CREATE TABLE agent_messages (
    id          TEXT PRIMARY KEY,
    from_agent  TEXT NOT NULL,
    to_agent    TEXT NOT NULL,
    msg_type    TEXT NOT NULL CHECK(msg_type IN ('task_delegation', 'artifact_reference', 'status_update', 'intervention')),
    payload     TEXT NOT NULL,
    read_at     DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. Knowledge Graph（交互沉淀的上下文）
CREATE TABLE knowledge_facts (
    id          TEXT PRIMARY KEY,
    subject     TEXT NOT NULL,
    predicate   TEXT NOT NULL,
    object      TEXT NOT NULL,
    source      TEXT NOT NULL,  -- 'agent-X' 或 'human-zhang'
    confidence  REAL DEFAULT 1.0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 6. 架构合规检查记录
CREATE TABLE compliance_checks (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES agent_sessions(id),
    adr_id      TEXT NOT NULL REFERENCES adrs(id),
    check_type  TEXT NOT NULL CHECK(check_type IN ('static', 'semantic', 'drift')),
    passed      BOOLEAN NOT NULL,
    issues      TEXT,
    checked_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### D7. 终端模拟器配置

Focus-tui 支持通过配置指定终端模拟器：

```toml
[agent]
# 终端模拟器命令，Focus-tui 通过此命令启动 Agent 外部终端
# 支持占位符：{title}, {directory}, {env}, {command}
terminal_emulator = "kitty --title {title} --directory {directory} {env} {command}"

# 备选方案（自动检测）
# terminal_emulator = "alacritty --title {title} --working-directory {directory} -e {command}"
# terminal_emulator = "wezterm cli spawn --cwd {directory} -- {command}"
# terminal_emulator = "gnome-terminal --title {title} --working-directory {directory} -- {command}"
```

**环境变量约定**：

| 环境变量 | 说明 |
|---------|------|
| `FOCUS_SESSION_ID` | Agent Session 唯一标识 |
| `FOCUS_TASK_ID` | 当前绑定任务的 ID |
| `FOCUS_PLAN_ID` | 当前绑定 Plan 的 ID |
| `FOCUS_MCP_SOCKET` | MCP Server 的 Unix socket 路径 |
| `FOCUS_A2A_SOCKET` | A2A Router 的 Unix socket 路径 |
| `FOCUS_ADR_CONSTRAINTS` | 当前 Plan 关联的 ADR 约束 JSON |

---

## Trade-offs

### T1. 去中心化 vs 中心化 Global Agent

| | 去中心化 (本 ADR) | 中心化 (ADR-0002) |
|---|---|---|
| **上下文窗口** | 每个 Agent 只持有自己的任务上下文 | Global Agent 需要持有全部上下文，窗口爆炸 |
| **故障隔离** | 单个 Agent 失败不影响其他 Agent | Global Agent 挂了，整个系统瘫痪 |
| **调试** | 每个 Agent 独立日志，易于追踪 | Global Agent 内部状态复杂，难以调试 |
| **灵活性** | 可随时替换某个步骤的 Agent 实现 | 修改 Global Agent 影响全部流程 |
| **协调成本** | 需要 A2A/MCP 协议层 | 内部函数调用，延迟更低 |
| **一致性** | 依赖共享状态的一致性保证 | Global Agent 单点保证一致性 |

**取舍理由**：一致性可以通过 MCP + SQLite WAL + 乐观锁保证，而中心化带来的上下文窗口和故障传播问题是根本性的。

### T2. 外部终端运行 vs TUI 内嵌 Pane vs PTY 镜像

| | 外部终端 (本 ADR) | TUI 内嵌 Pane (原有设计) | PTY 镜像 (早期草案) |
|---|---|---|---|
| **Agent 原生感** | ✅ 完整终端自由体验 | ❌ 像被关在 Pane 里 | ⚠️ 后台运行但 TUI 内显示 |
| **显示空间** | ✅ 终端模拟器原生尺寸 | ❌ 受 Pane 大小限制 | ⚠️ 逻辑尺寸 120x40，Pane 内适配 |
| **多 Agent 查看** | ✅ 用户自由排列多个窗口 | ❌ 挤在一个 TUI 里 | ⚠️ Grid 模式但仍受 TUI 限制 |
| **ANSI 渲染** | ✅ 终端模拟器原生支持 | ❌ 需 TUI 自行解析 | ⚠️ 需 TUI 解析并适配尺寸 |
| **滚动历史** | ✅ 终端自己的缓冲区 | ❌ 受 Pane 高度限制 | ⚠️ 受配置的循环缓冲区限制 |
| **统一视图** | ✅ Dashboard 卡片 + 外部窗口 | ✅ 天然统一 | ✅ 监控面板 + 镜像 |
| **多显示器** | ✅ Agent 窗口可分布在多屏 | ❌ 单窗口 | ⚠️ 单窗口 |
| **窗口管理** | ⚠️ 需 Alt-Tab / 窗口管理器 | ✅ TUI 内切换 | ✅ TUI 内切换 |
| **实现复杂度** | 低（只需启动外部进程） | 最低 | 高（需 PTY 镜像层） |
| **与现有工作流融合** | ✅ 用户继续用自己习惯的终端 | ❌ 必须进入 TUI | ❌ 必须进入 TUI |

**取舍理由**：外部终端让 Agent 获得真正的原生体验，同时 Focus-tui 通过协议层保持调度统一性。唯一的"损失"是窗口切换需要 Alt-Tab 而非 TUI 内按键，但这是操作系统原生的窗口管理，用户更熟悉。

### T3. 协议开销 vs 紧耦合

| | A2A + MCP (本 ADR) | 直接函数调用 (ADR-0003) |
|---|---|---|
| **通信延迟** | ~1ms（本地 Unix socket） | ~0ms（同进程） |
| **解耦程度** | Agent 可用任何语言实现 | 必须是 Go |
| **扩展性** | 任何符合协议的 Agent 可接入 | 需要修改 focus-tui 代码 |
| **协议维护** | 需要维护 A2A/MCP 层 | 无额外协议层 |

**取舍理由**：1ms 延迟在 Agent 工作流中完全可忽略（Agent 执行通常以秒为单位），而解耦带来的扩展性（Claude Code / Codex / 自定义 Agent 无需修改 focus-tui）是关键优势。

### T4. 人类直接操作 vs 代理中转

| | 直接操作 MCP (本 ADR) | 通过 Global Agent 中转 (ADR-0002) |
|---|---|---|
| **响应速度** | 直接写 SQLite，无延迟 | 需要 Global Agent 处理 |
| **认知负担** | 人类需要理解共享状态模型 | Global Agent 包装成自然语言 |
| **可靠性** | Agent 失灵时人类仍可操作系统 | Global Agent 失灵时系统部分瘫痪 |
| **精细控制** | 可直接修改任何字段 | 受 Global Agent 能力限制 |

**取舍理由**：ADR-0000 的"人类主权"原则要求人类在任何情况下都能直接操作系统。直接操作 MCP 是这一原则的技术保证。

---

## Performance Assessment

### 已识别的性能风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **SQLite 并发写竞争** | 多个 Agent 同时写 MCP | WAL 模式 + 乐观锁 + 批量写入 |
| **A2A 消息堆积** | 高频通信导致数据库膨胀 | 消息 TTL（7 天），已读消息自动归档 |
| **Knowledge Graph 查询** | 大量事实导致查询变慢 | 按 task_id 分区索引，全文搜索延后实现 |
| **外部终端进程数** | 大量 Agent 同时运行 | Agent 生命周期由调度器管理，完成后自动退出 |

### 基准预期

- **Agent 启动延迟**：< 200ms（启动终端模拟器 + fork Agent 进程）
- **MCP 调用延迟**：< 1ms（本地 Unix socket）
- **Dashboard 渲染**：< 10ms（纯数据查询，无 ANSI 解析）
- **内存**：Focus-tui 本身无 Agent 输出缓冲区负担，仅 SQLite 存储

---

## Consequences

### Positive

1. **架构极简**：去掉 Global Agent 和 PTY 镜像层后，系统组件减少 60%+
2. **Agent 原生体验**：Agent 在完整终端中运行，有真正的 CLI 自由感
3. **多窗口自由**：用户可以用操作系统原生方式管理 Agent 窗口（平铺、标签、多显示器）
4. **语言无关**：任何符合 A2A/MCP 的 Agent 可接入（Python/TypeScript/Go 均可）
5. **故障隔离**：单个 Agent 失败不影响全局；Focus-tui 崩溃不影响 Agent
6. **水平扩展**：Agent 可以运行在远程机器上（MCP over TCP）
7. **人类主权**：人类直接操作共享状态，不依赖任何 Agent 中转
8. **与现有工作流融合**：用户继续使用自己习惯的终端模拟器

### Negative

1. **窗口管理**：多个 Agent 窗口需要用户自行管理（或依赖窗口管理器）
2. **调试复杂度**：分布式 Agent 的调试比单进程 Global Agent 更难
3. **一致性挑战**：需要 SQLite WAL + 乐观锁保证共享状态一致性
4. **协议维护**：需要维护 A2A/MCP 协议层
5. **上下文传递**：Agent 需要通过 MCP 主动读取上下文，而不是被动接收

### Mitigation

- 窗口管理：提供可选的窗口聚焦 API（`xdotool`/`swaymsg`/AppleScript），但不强制
- 调试：A2A 消息全部记录到 `agent_messages` 表，可回放
- 一致性：SQLite WAL 模式已启用，乐观锁通过 `version` 字段实现
- 协议：MCP 协议面简单（JSON-RPC），最坏情况下自实现可控
- 上下文：`context.get_for_task()` MCP 工具自动聚合所有必要上下文

---

## Relationship to ADR-0000

Per ADR-0000 Clause 1 (ADR is the highest decision layer):
- 本 ADR 作为 Accepted ADR，定义了 Agent 架构的宪法级约束

Per ADR-0000 Clause 2 (Lower layers may not redefine upper layers):
- Agent 进程（Session 层）**不可修改** ADR / Plan / Task 边界
- Agent 进程**只读** ADR constraints，通过 MCP 写入产出后由调度器触发状态变更

Per ADR-0000 Clause 3 (Sessions must execute with constitutional context):
- 每个 Agent 启动时通过 MCP 读取相关 ADR constraints
- Agent 产出必须通过 Compliance Check 验证是否违反 ADR

---

## Relationship to ADR-0001

Per ADR-0001 (Human-Sovereign, Agent-Native Workbench):
- **Human-sovereign**：人类通过 TUI 直接读写 MCP，不通过 Agent 中转
- **Agent-native**：Agent 是一等公民，在外部终端自由运行，不依赖 Global Agent
- **No vendor lock-in**：任何符合 A2A/MCP 的 Agent 可接入

---

## Supersedes

- **ADR-0002**: Global Agent Architecture — Human-Augmented Development Workflow
  - 状态更新为：`Superseded by ADR-0004`
  - 原因：Global Agent 模式被去中心化 Agent Mesh 取代

- **ADR-0003**: Global Agent 架构技术选型
  - 状态更新为：`Superseded by ADR-0004`
  - 原因：In-process Go Agent 技术选型不再适用

---

## Out of Scope / Deferred

本 ADR 不决策以下内容，由后续 ADR 处理：

- **ADR-future-A**：MCP 接口完整清单（resources / tools / schema）
- **ADR-future-B**：A2A 协议轻量实现细节（Agent Cards / Task 委托 / Artifact 引用）
- **ADR-future-C**：终端模拟器自动检测与配置格式
- **ADR-future-D**：Compliance Checker 实现（静态检查 / 语义检查 / 漂移检测）
- **ADR-future-E**：窗口聚焦 API 实现（跨平台窗口管理）
- **ADR-future-F**：人类审批门的动作分类规则与 TUI 交互形态
- **ADR-future-G**：远程 Agent 支持（MCP over TCP / Agent 运行在远程机器）

---

## References

- ADR-0000: Constitution for ADR-Driven Execution
- ADR-0001: Product Positioning — Human-Sovereign, Agent-Native Workbench
- ADR-0002: ~~Global Agent Architecture~~ (Superseded)
- ADR-0003: ~~Global Agent 架构技术选型~~ (Superseded)
- Open Multi-Agent (`JackChen-me/open-multi-agent`)：Coordinator + DAG 调度
- Composio Agent Orchestrator (`ComposioHQ/agent-orchestrator`)：生命周期状态机 + 独立 worktree
- MetaGPT (`FoundationAgents/MetaGPT`)：多角色协作 + 标准化文档接口
- OpenHands (`All-Hands-AI/OpenHands`)：SDK/CLI/GUI/Cloud 分层 + MCP 集成
- Sema Code (论文)：引擎-客户端分离 + 多租户隔离
