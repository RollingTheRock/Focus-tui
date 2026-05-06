# ADR-0005 Implementation Plan

## 前置条件

- [ ] 当前分支代码干净（已提交或 stash 未完成的 DAG 相关修改）
- [ ] 创建 feature branch：`git checkout -b feat/event-sourced-context-store`
- [ ] 确认 go build / go test 基线通过

---

## Phase 0: 基础设施与 Schema（第 1-2 天）

**目标**：引入 PostgreSQL，建立 Event Store 和 Projection 的 schema，保持 SQLite 完全不变。

### 0.1 依赖引入
- [ ] `go get github.com/jackc/pgx/v5`
- [ ] 验证 `go mod tidy` 后构建仍然通过

### 0.2 PostgreSQL 连接管理
- [ ] 新建 `internal/store/pgconn/`（或直接在 `internal/store/` 中）
- [ ] `pgconn.New(dbURL string) (*pgxpool.Pool, error)` — 连接池初始化
- [ ] `pgconn.EmbeddedPostgresPath()` — 进程内嵌入路径（默认 `~/.local/share/focus/postgres`）
- [ ] 通过环境变量切换：`FOCUS_STORE=sqlite`（默认）或 `FOCUS_STORE=postgresql`
- [ ] `Store` 结构体支持双模式：`type Store struct { sqlite *sql.DB; pg *pgxpool.Pool; mode string }`

### 0.3 Event Store Schema
```sql
CREATE TABLE events (...)
CREATE TABLE consumer_offsets (...)
CREATE INDEX idx_events_aggregate (...)
CREATE INDEX idx_events_order (...)
CREATE INDEX idx_events_scope (...)
CREATE INDEX idx_events_correlation (...)
```
- [ ] 实现 `internal/store/event_store.go`
  - `AppendEvent(ctx, Event) (eventID int64, error)`
  - `ReadEventsAfter(ctx, consumerID string, batchSize int) ([]Event, error)`
  - `SaveOffset(ctx, consumerID string, eventID int64) error`
  - `GetOffset(ctx, consumerID string) (int64, error)`

### 0.4 Projection Schema（与现有 SQLite 表结构一致，加 `event_version` 字段）
- [ ] `proj_tasks`
- [ ] `proj_worktree_contexts`
- [ ] `proj_agent_sessions`
- [ ] `proj_task_plans`
- [ ] `proj_plan_steps`
- [ ] `proj_context_notes`
- [ ] `proj_knowledge_facts`
- [ ] `proj_task_outputs`
- [ ] `proj_session_handoffs`
- [ ] `proj_worktree_history`
- [ ] `proj_task_worktree_links`
- [ ] `proj_task_dependencies`
- [ ] `proj_agent_messages`

### 0.5 Event 类型定义
- [ ] 新建 `internal/events/types.go`
- [ ] 定义 `Event` struct（对应 schema 字段）
- [ ] 定义各领域 Event 类型：
  - `TaskCreated`, `TaskStateChanged`, `TaskGoalUpdated`
  - `WorktreeContextUpdated`, `WorktreeActivated`
  - `AgentSessionCreated`, `AgentSessionHeartbeat`, `AgentSessionDisconnected`
  - `ContextNoteAdded`, `ContextNoteUpdated`
  - `PlanStepStateChanged`, `PlanApproved`
  - `SessionHandoffCreated`
  - `KnowledgeFactAdded`

### 验收标准
```bash
go build ./cmd/focus/
go test ./internal/store/...
# 新测试：Event Store 追加和读取
# 新测试：Offset 保存和恢复
```

---

## Phase 1: 双写模式（第 3-5 天）

**目标**：所有写操作同时写 SQLite 和 Event Store，SQLite 仍是唯一真相，Event Store 用于验证。

### 1.1 改造核心写方法（先 Todo + TaskContext 两个领域试点）
- [ ] `SaveTaskContext`：先 `AppendEvent(TaskCreated/TaskStateChanged)`，再写 SQLite
- [ ] `SaveTodo` / `CreateTodo`：同上
- [ ] 双写失败时：**SQLite 优先**，Event Store 写失败只打日志不中断

### 1.2 Projection Builder（只读验证，不用于生产查询）
- [ ] 新建 `internal/projections/builder.go`
- [ ] `ProjectionBuilder` goroutine：消费 `events` 表，更新 `proj_*` 表
- [ ] 每天运行一致性校验：`SELECT COUNT(*)` 对比 SQLite 表 vs Projection 表
- [ ] 不一致时报警（日志 ERROR）

### 1.3 事件 payload 序列化
- [ ] `Event.Payload` 使用 `json.Marshal`/`Unmarshal`
- [ ] 每个领域定义 payload struct（如 `TaskCreatedPayload`）

### 验收标准
```bash
go test ./internal/store/...
go test ./internal/projections/...
# 手动测试：运行 focus，创建任务，检查 events 表有记录
# 一致性校验脚本通过
```

---

## Phase 2: Piggyback + Projection 并行（第 6-10 天）

**目标**：Projection 表成为实际查询目标；Piggyback 模式在 `context.*` tools 中实验。

### 2.1 读操作迁移到 Projection（逐步）
- [ ] `GetTaskContext` → 读 `proj_tasks`
- [ ] `ListTaskContexts` → 读 `proj_tasks`
- [ ] `GetWorktreeContext` → 读 `proj_worktree_contexts`
- [ ] 每个迁移后，运行一致性校验确保结果一致

### 2.2 Consumer Offsets 机制
- [ ] MCP Server 维护 `clientLastSeenEventID map[string]int64`
- [ ] key 使用 MCP session ID（或连接标识）

### 2.3 Piggyback 实验（2-3 个 tools）
- [ ] `context.get_task`：result 中附加 `EventsSinceLastCall` 摘要
- [ ] `context.list_tasks`：同上
- [ ] `context.get_worktree`：同上
- [ ] 事件摘要格式（自然语言）：
  ```
  ---
  📬 Context Updates since your last interaction:
  • 2 min ago: Agent-Kimi completed subtask 'auth-module-refactor'
  • 1 min ago: New blocker added: 'Database migration pending review'
  ---
  ```

### 2.4 事件过滤逻辑
- [ ] 根据 tool 的 scope（task_id / worktree_id）过滤相关事件
- [ ] 最多返回 N 条（默认 5 条），超限提示"还有 X 条未读"

### 2.5 回滚能力
- [ ] 保留 `FOCUS_DISABLE_PIGGYBACK=1` 环境变量开关
- [ ] Piggyback 内容作为独立 content block，不影响原有 tool result 解析

### 验收标准
```bash
# MCP Inspector 测试：连续调用 context.get_task，第二次应收到增量事件
# 单元测试：Piggyback 过滤逻辑正确
# 一致性校验：SQLite vs Projection 结果完全一致
```

---

## Phase 3: Command 化写操作（第 11-18 天）

**目标**：写操作不再直接 UPDATE SQLite，而是走 Command → Event → Projection 链路。

### 3.1 Command 层设计
- [ ] 新建 `internal/commands/`
- [ ] `Command` interface：`Validate(ctx) error` + `Execute(ctx) ([]Event, error)`
- [ ] `CommandBus`：`Send(ctx, Command) error`

### 3.2 按领域逐个 Command 化
优先级顺序：
1. [ ] `TaskContext`（`CreateTask`, `UpdateTaskState`, `UpdateTaskGoal`）
2. [ ] `WorktreeContext`（`ActivateWorktree`, `UpdateWorktreeTask`）
3. [ ] `AgentSession`（`RegisterSession`, `RecordHeartbeat`, `DisconnectSession`）
4. [ ] `ContextNote`（`AddNote`, `PinNote`）
5. [ ] `PlanStep`（`UpdateStepState`, `ExpandStepToTask`）
6. [ ] `SessionHandoff`（`CreateHandoff`）
7. [ ] `TaskDependency`（`AddDependency`, `RemoveDependency`）
8. [ ] 其余领域（Todo, Pomodoro 等低优先级）

### 3.3 每个 Command 的改造流程
```
1. 定义 Command struct + Validate + Execute
2. 修改 Store 方法：
   旧：直接 SQLite UPDATE
   新：commandBus.Send(ctx, cmd) → AppendEvent → Projection Builder 更新
3. 更新调用方（app.go 中的 handler）
4. 运行该领域的 store 测试确保通过
5. 运行一致性校验
```

### 3.4 乐观并发控制
- [ ] `SaveTaskContext` 等更新操作自动附加 `expected_version`
- [ ] `AppendEvent` 时检查 `UNIQUE(aggregate_type, aggregate_id, version)`
- [ ] 冲突时返回 `ErrConcurrentModification`，由调用方决定重试或报错

### 验收标准
```bash
go test ./internal/commands/...
go test ./internal/store/...
# 全量一致性校验通过
# 手动测试：focus 正常运行，创建/更新/删除任务无异常
```

---

## Phase 4: Event Bus + Orchestrator 重构（第 19-24 天）

**目标**：In-Memory Event Bus；Orchestrator 变成常驻状态通知引擎。

### 4.1 In-Memory Event Bus
- [ ] 新建 `internal/eventbus/`
- [ ] `EventBus` struct：管理 subscribers（map[string]chan Event）
- [ ] `Publish(ctx, Event)`：广播给所有匹配 subscriber
- [ ] `Subscribe(topic string, filter EventFilter) (chan Event, cancelFunc)`
- [ ] Event Bus 在 `Store.AppendEvent` 成功后自动触发（同进程内）

### 4.2 Orchestrator 常驻化
- [ ] `app.go` 初始化时创建 `orchestrator.New(...)`，作为 `model` 字段保留
- [ ] `orchestrator.Run()` 启动常驻 goroutine 消费 Event Bus
- [ ] 移除所有现场 `orchestrator.New(...)` 的调用

### 4.3 Orchestrator 规则引擎
- [ ] `Rule` 类型：`func(ctx, Event) []Action`
- [ ] 初始规则集：
  - `OnTaskCompleted`：TUI 通知"下游任务已就绪"
  - `OnBlockerAdded`：TUI 通知"Agent-X 当前任务出现 blocker"
  - `OnAgentDisconnected`：TUI 通知"Agent-X 已断开"
  - `OnPlanCompleted`：TUI 通知"计划已完成"

### 4.4 TUI 通知机制
- [ ] `model` 新增 `notifications []UINotification`
- [ ] 通知栏 UI 组件（footer 或独立 pane）
- [ ] 通知可点击展开详情
- [ ] 通知有过期时间（如 30 分钟后自动淡化）
- [ ] 键盘快捷键查看/清除通知

### 4.5 心跳检查 goroutine
- [ ] Orchestrator 维护 `runningAgents map[string]RunningAgent`
- [ ] `time.Ticker(30s)`：检查最后心跳时间
- [ ] 超 2 分钟未心跳 → 发布 `AgentSessionDisconnected` 事件 → TUI 通知

### 4.6 移除旧机制
- [ ] 移除 `launchDownstreamTasks` 中的自动 Agent 启动逻辑
- [ ] `OnTaskCompleted` 不再启动 Agent，改为 enqueue 通知
- [ ] 保留 `resolveOrchestratedProvider`（人类手动启动时仍需要 provider 建议）

### 验收标准
```bash
go test ./internal/eventbus/...
go test ./internal/orchestrator/...
# 手动测试：
#   1. 启动 focus
#   2. 完成任务 A
#   3. TUI 应立即显示"下游任务 B 已就绪"通知
#   4. 杀死一个 Agent 进程
#   5. 30 秒内 TUI 显示"Agent 已断开"通知
```

---

## Phase 5: SQLite 下线 + 清理（第 25-28 天）

**目标**：SQLite 完全下线，Projection 表成为唯一查询目标。

### 5.1 删除 SQLite 写路径
- [ ] 所有 Store 方法不再写 SQLite
- [ ] `Store.sqlite` 字段可标记为 deprecated

### 5.2 MCP Resource 层（Bonus）
- [ ] 新建 `internal/mcp/resources.go`
- [ ] 暴露 `context://task/{id}`, `context://worktree/{id}`, `context://plan/{id}`
- [ ] 声明 `resources: { subscribe: true, listChanged: true }`
- [ ] Resource 内容从 Projection 表实时读取

### 5.3 清理遗留代码
- [ ] 删除 SQLite schema 中的旧表（或保留为空，留作回滚）
- [ ] 删除 `internal/store/db.go` 中的 SQLite 初始化逻辑（如果不再支持 sqlite 模式）
- [ ] 删除双写代码中遗留的 SQLite 分支
- [ ] 删除 `FOCUS_STORE=sqlite` 环境变量支持（如果决定完全下线）

### 5.4 性能基准
- [ ] 测量 Event Store 写入吞吐量（目标：> 100 events/sec）
- [ ] 测量 Projection 查询延迟（目标：P99 < 10ms）
- [ ] 测量 Piggyback 事件过滤延迟（目标：P99 < 5ms）

### 验收标准
```bash
go test ./...
go build ./cmd/focus/
# 手动回归测试：所有 TUI 操作正常
# 性能基准通过
```

---

## 风险与回滚策略

| 风险 | 缓解措施 |
|------|---------|
| PostgreSQL 引入增加部署复杂度 | Phase 0-4 保留 SQLite 作为 fallback；Phase 5 才完全下线 |
| Event Store 写入失败导致数据丢失 | 双写期间 SQLite 优先；Command 化后乐观锁保证一致性 |
| Projection 延迟导致读不一致 | 双写期间每天运行一致性校验；不一致时自动重建 projection |
| Piggyback 增加 token 消耗 | 提供 `FOCUS_DISABLE_PIGGYBACK=1` 开关；限制最多 5 条事件 |
| Orchestrator 常驻 goroutine 泄漏 | 所有 goroutine 有 context 取消；app 退出时优雅关闭 |
| 迁移周期过长导致分支冲突 | 每 Phase 结束时合并到主分支（保持可工作的代码） |

---

## 里程碑与合并节点

| 节点 | 时间 | 可合并？ |
|------|------|---------|
| Phase 0 完成 | Day 2 | ✅ SQLite 不变，只新增代码 |
| Phase 1 完成 | Day 5 | ✅ SQLite 仍是真相，Event Store 只是审计 |
| Phase 2 完成 | Day 10 | ✅ Projection 并行运行，SQLite 仍是 fallback |
| Phase 3 完成 | Day 18 | ⚠️ 需要充分测试后才能合并 |
| Phase 4 完成 | Day 24 | ⚠️ Orchestrator 行为改变，需要人工验收 |
| Phase 5 完成 | Day 28 | ✅ 完整功能，可合并 |

---

## 关键文件清单

### 新建文件
```
internal/store/event_store.go
internal/store/pgconn/
internal/events/types.go
internal/commands/
internal/projections/builder.go
internal/eventbus/
internal/orchestrator/engine.go        # 替换原有 orchestrator.go
internal/mcp/resources.go              # MCP Resource 层
internal/ui/notification.go            # TUI 通知组件
```

### 重大修改文件
```
internal/store/db.go                   # 双模式连接管理
internal/store/*.go                    # 所有写方法 Command 化
internal/app/app.go                    # Orchestrator 常驻化 + 通知消费
internal/models/ui.go                  # Store 接口可能扩展
```

### 最终删除文件
```
internal/store/db.go 中的 SQLite schema（Phase 5）
```
