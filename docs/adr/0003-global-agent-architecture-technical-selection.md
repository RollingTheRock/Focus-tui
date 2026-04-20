# ADR-0003: Global Agent 架构技术选型

## Status
Proposed — 2026-04-20 (v2)

## Context

ADR-0002 确立了 Global Agent 的产品角色（辅助 ADR→Plan→Task→Session 分层、
按需唤起、执行层可外包），但未落地到技术形态。本 ADR 在 ADR-0000（宪法约束）
与 ADR-0001（产品定位）的前提下，钉死 Global Agent 的运行时形态、Context
模型、人类/Agent 写权边界、外部 Agent 集成协议与模型依赖策略。

**回顾约束**：
- 人类主权：所有决定性动作须可被人类拦截与回滚
- 层级不可逾越：Session 不得修改 Task/Plan/ADR
- ADR 不可变：本 ADR Accepted 后不再修改，只能被 Supersede
- 不绑定模型提供商，优先支持 Kimi

**候选底座结论**：Kimi CLI / Claude Code / OpenCode 均非 Go 实现，无法
in-process 嵌入 focus-tui。三者作为**架构蓝本**参考，不作为 Global Agent
的运行时依赖；保留为 Session 层可选执行进程。

---

## Decision

### D1. 运行时形态：In-process Go Agent

Global Agent 作为 focus-tui 的内部组件运行（新增
`internal/plugins/globalagent/`），与 TUI、Store、现有 Plugins 同进程，不引入
独立 daemon。

**理由**：低延迟访问 SQLite、统一崩溃域、最小化用户心智负担。Daemon 化留待
未来 ADR 在出现"TUI 关闭后仍需后台运行"强需求时评估。

### D2. Agent Loop：自建 Go 实现，参考多底座但不嵌入

Global Agent 的 Loop 由本项目自行实现，采用双层设计：
- **Outer Loop**：Plan 级规划与任务分解，参考 XAgent
- **Inner Loop**：ReAct 式工具调用，参考 Claude Code 的协调机制与工具范式
- **Context 管理**：参考 Kimi CLI 的 Checkpoint + Revert 模型

**自建 vs 嵌入权衡**：

| 维度 | 嵌入 Kimi CLI | 自建 Go Loop |
|---|---|---|
| 构建复杂度 | 引入 Python runtime + uv/pip 依赖 | 单一 Go 二进制 |
| Context 四层控制 | L2/L3 隔离需反向改造 Soul/Context | 从零设计，契合 ADR-0000 |
| 崩溃域 | 跨语言跨进程 | 同进程 goroutine |
| 工程量 | 小（复用底座实现） | 大（需自实现 loop/compaction/checkpoint/心跳） |

结论：以 2–3 个月自研工程量换取可移植性与架构纯度。三个候选底座的角色是
**算法蓝本**而非运行时依赖。

### D3. Context 四层模型与所有权隔离

| Layer | 内容 | 归属 | 可变性 |
|---|---|---|---|
| **L1 Constitutional** | ADR / 产品定位 / 全局配置 | Global | 不可变 |
| **L2 Global Working** | Task / Plan / plan_steps / context_notes / handoff / worktree_contexts | **Global 独占** | Global 读写，Session 只读投影 |
| **L3 Session** | 对话历史 / tool_call 轨迹 / 未提交 diff / 临时产物 | Session | Session 读写，随 Session 生命周期释放 |
| **L4 Model** | LLM token 窗口 | — | 每次请求动态构造 |

**所有权不变量**：
- Session 不持有 L2，只通过 MCP 接收"任务信封（Task Envelope）"与按需投影
- Session 不可直接写 L2；状态变更通过 handoff 由 Global 审阅后改写
- 该隔离是 ADR-0000"层级不可逾越"约束的技术实现

**"Global 独占"的精确含义**：在所有**自动化代理**中，仅 Global Agent 可写
L2。此约束不限制人类主权。人类修改 L2 有两条合法路径：

- **路径 A（直接写）**：人类通过 TUI 直接修改 SQLite 中的 Task/Plan 状态
- **路径 B（代理写）**：人类向 Global Agent 发起自然语言请求，Global Agent
  执行并回填（并可做一致性检查，如拒绝将含未完成子 Task 的父 Task 标为 done）

两条路径**必须都支持**。路径 A 保证 Agent 失灵时系统仍可用；路径 B 保证
Agent 能做决策增强。

### D3a. Store 变更通知机制（显式决策）

Store 层必须提供变更通知能力，使 Global Agent 能感知人类路径 A 的直接写入：

- `internal/store` 新增订阅接口（pub/sub 或 channel-based）
- Global Agent 在启动时订阅关键表（task_contexts / task_plans / plan_steps /
  context_notes）的变更事件
- 收到事件后，Global Agent 刷新自身工作集缓存；若当前正在运行 Outer/Inner
  Loop，将变更注入下一轮决策的 L4 Model Context
- 具体事件 schema、批处理策略、订阅粒度由未来 ADR 细化

**此决策影响 Store 层接口，必须在本 ADR 中显式钉死**，以避免后续实现时发现
Store 不支持订阅而引发架构回退。

### D4. Worktree 与 Session 绑定：主从 Session 模型

**Session 的精确定义**：由 LLM 驱动的 Agent 执行单元，持有 L3 并占用模型
token 窗口。`tmux` 中手动跑的测试、shell、日志查看器 **不是 Session**，不受
本约束限制。

**一个 worktree 可同时绑定**：
- 至多一个 **Primary Session**：持有 worktree 的**写锁**，可执行文件修改工具
- 任意数量的 **Readonly Session**：仅可用只读工具（read/grep/glob/LSP 查询），
  不占用写锁

**写锁语义**：由 SQLite 的 `worktree_contexts.primary_session_id` 字段表达，
行级事务保证互斥。Primary Session 退出或显式释放后，下一个 Dispatch 可申请
写锁。

Readonly Session 用于"边修改边分析"场景（如主 Session 改代码，副 Session
审查 diff 并给建议）。

### D5. Global Agent 写入语义：三类路径

Global Agent 的写入能力按"操作范围 × 审批强度"分为三类：

| 路径 | 触发方式 | 机制 | 适用场景 |
|---|---|---|---|
| **探索读** | Agent 自主 | 只读工具子集（read/grep/glob/dry-run bash） | 起草 ADR、分解 Plan 时读代码 |
| **Staging 写** | Agent 自主 + 事后审批 | 写入 `.focus/staging/<session_id>/`（独立 git worktree 或目录），人类审查后 `promote` 到目标 worktree | 样板代码生成、批量重构草稿 |
| **Session 写** | 显式 dispatch | 派生 Primary Session 获取 worktree 写锁，按 D4 执行 | 需进入实际 worktree 的代码改动 |

**不变量**：**没有任何 Agent 能在未经人类审批的前提下直接写入活跃 worktree**。
Staging 写对 Global Agent 是"自主的"，但对 worktree 仍是"间接的"——promote
是显式人类动作。

Staging 目录的 GC 策略、promote 的冲突解决、和 git 的交互细节由未来 ADR 定义。

### D6. 外部 Agent 集成协议：MCP

- **MCP Server** 作为 goroutine 运行于 focus-tui 进程内，暴露 L2 只读投影与
  受控写通道（handoff 接收、审批请求）
- **外部 Session Agent**（Claude Code / OpenCode / Kimi CLI）作为 MCP Client，
  通过子进程 + Unix socket 接入
- 取代现有 `FOCUS_AGENT_SESSION_ID` 环境变量 + `backflowAgentSession` 猜测
  状态方案

**实施策略（三段式）**：
1. **P0（本期）**：基于 `github.com/mark3labs/mcp-go` 实现 MCP Server
2. **P1（若遇阻）**：若社区库关键特性缺失或 bug 过多，降级为 focus-tui 自
   维护的 minimal MCP 子集实现
3. **备选协议**：不引入 gRPC（会破坏"外部 Agent 无改造接入"目标）。若 MCP
   整体不可用，退回环境变量 + stdout line-protocol 过渡方案，**仅作为应急
   降级**，不作为长期决策

**外部端**：Claude Code / OpenCode / Kimi CLI 均原生支持 MCP Client，无需
本项目适配。

MCP 资源/工具清单、心跳协议、审批通道 schema 延后至未来 ADR。

### D7. 模型策略：Adapter 分层，Kimi 优先

模型访问通过 `internal/agents/model/` 下的 Adapter 抽象，Loop 层模型无关。

**接口定义**：
```go
type ModelAdapter interface {
    // 核心推理调用。prompt_cache / session key / header 等模型特定优化
    // 由 Adapter 内部处理，对 Loop 层透明。
    Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)

    // 声明 Adapter 支持的特性，Loop 层按能力降级。
    Capabilities() ModelCapabilities

    // 通知 Adapter 一个会话边界（L1 变更、Checkpoint 切换），
    // Adapter 据此决定是否轮换 prompt_cache_key。
    InvalidateCache(sessionKey string)
}

type GenerateRequest struct {
    SessionKey   string           // 稳定标识，用于 prompt_cache
    SystemPrompt string           // L1 + 静态指令（应命中 cache）
    Tools        []ToolDef        // 工具定义（应命中 cache）
    Messages     []Message        // 对话历史 + L2 投影
    Options      GenerateOptions  // thinking / temperature / max_tokens
}

type ModelCapabilities struct {
    PromptCache      bool
    Thinking         bool
    MaxContextTokens int
    ToolCallFormat   string  // "openai" | "anthropic" | "kimi"
}
```

**Kimi Adapter 具体行为**：
- `SessionKey` 映射为 Kimi API 的 cache key
- `SystemPrompt + Tools` 放在请求前缀以最大化 cache 命中
- L1 Constitutional Context 注入 SystemPrompt，L2 投影注入 Messages 末尾
- `Options.Thinking=true` 时启用 Kimi thinking 模式，Adapter 负责解析
  `<thinking>` 段与 tool_call 段的分离
- Checkpoint Revert 时调用 `InvalidateCache` 防止脏缓存

**其他模型**（Claude / GPT / DeepSeek 等）通过 OpenAI 兼容协议 Adapter 接入。
**不引入 Kimi CLI 的 Python runtime 作为依赖**。

### D8. Global Agent 工具集边界

Global Agent 的工具集是 Claude Code 工具集的**超集**，包括：
- **底座继承工具**：read / write / edit / bash / grep / glob / web_fetch /
  todo / launch_subagent 等
- **L2 元级工具**：ADR / Plan / Task / Session 的 CRUD 与调度、Staging promote、
  审批通道、Checkpoint 管理

**关键约束**：Global Agent 的文件写入类工具受 D5 约束——直接写 worktree
的能力不开放，写操作必须走 Staging 或 Session 路径。

工具集完整清单与 schema 延后至未来 ADR。

### D9. 数据模型扩展

- `agents.Provider` 枚举新增 `ProviderNative`
- SQLite 新增表（schema 细节延后）：
  - `agent_checkpoints`：L2 与 Session 状态的快照
  - `mcp_sessions`：MCP 连接与外包 Session 的映射
- `worktree_contexts` 表新增 `primary_session_id` 字段（支持 D4 写锁）
- 现有 `agent_sessions` / `session_handoffs` 的 state 枚举扩充以覆盖心跳与
  审批态，迁移由未来 ADR 规划

---

## Consequences

### 正面
- Global Agent 与 Store 同进程，性能与一致性最优
- Context 四层隔离使 ADR-0000 的层级约束有可验证的技术落点
- 人类主权通过路径 A + D3a Store 通知机制得到技术保证
- Staging 机制在"Agent 辅助效率"与"worktree diff 纯净"之间取得平衡
- 模型可替换性完整保留，Kimi 优势通过 Adapter 获得而非底座绑定
- MCP 作为统一协议替换环境变量方案，为未来外部 Agent 扩展提供稳定契约

### 负面
- 自建 Agent Loop 初期工程量显著
- Staging 机制引入新的目录管理与 promote 冲突处理复杂度
- Go MCP 生态成熟度低于 TS 生态，P1 降级路径存在实施风险
- Store 变更通知机制（D3a）对现有 `internal/store` 接口有侵入式改造

### 缓解
- 通过参考 Claude Code / XAgent / Kimi CLI 成熟算法降低自研风险
- Staging 目录复用 git worktree 机制，避免另造管理层
- MCP 协议面简单（JSON-RPC over stdio），最坏情况下自实现可控
- Store 订阅接口可用 Go channel 最小化实现，不引入外部 pub/sub 依赖

---

## Out of Scope / Deferred

本 ADR 不决策以下内容，由后续 ADR 处理：

- **ADR-future-A**：MCP 接口清单（resources / tools / 心跳 / 审批 schema）
- **ADR-future-B**：Global Agent 工具集完整目录与权限模型
- **ADR-future-C**：Agent 状态机、心跳协议与失败恢复语义
- **ADR-future-D**：Context Manager 的 Checkpoint 存储格式、压缩策略、
  Revert 粒度
- **ADR-future-E**：Kimi Adapter 的 prompt_cache 命中策略细节与 thinking
  模式工具调用协议
- **ADR-future-F**：人类审批门的动作分类规则与 UI 交互形态
- **ADR-future-G**：Staging 目录的 GC 策略、promote 冲突解决、与 git 交互
- **ADR-future-H**：Store 变更通知事件 schema 与批处理策略
- **ADR-future-I**：Daemon 化迁移路径（若未来有需求触发）

---

## References

- ADR-0000 宪法约束
- ADR-0001 产品定位
- ADR-0002 Global Agent 架构
- 参考项目：XAgent、Claude Code、Kimi CLI、OpenCode、
  `github.com/madebyaris/agent-orchestration`（待调研）
- Go MCP 实现：`github.com/mark3labs/mcp-go`、
  `github.com/modelcontextprotocol/go-sdk`
