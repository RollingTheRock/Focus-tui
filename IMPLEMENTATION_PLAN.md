# Focus TUI 开发实现计划（当前执行版）

> 更新日期: 2026-04-17
> 当前阶段: Phase 4 - Human Context Recovery + Lightweight Task Orchestration
> 当前执行点: Phase 4 Step 1 - Context Schema & Store Foundation
> 设计参考: [DESIGN-WORKTREE-CONTAINERS-2026-04.md](./DESIGN-WORKTREE-CONTAINERS-2026-04.md)

---

## 一、当前结论

本文件以**当前代码事实**为准，而不是沿用旧的阶段描述。

截至 2026-04-17，以下能力已经在代码中落地并有测试覆盖：

- overview page / per-worktree page 双层 page model
- `newWorktreePage(...)` 工厂函数
- `switchToWorktreePage(...)` / `switchToOverviewPage(...)` 真正切换 page 实例
- worktree-scoped shell / editor / diff / commit pane 作用域隔离
- editor reuse 按 `WorktreeID` 隔离
- page snapshot 内存恢复与 SQLite 持久化
- worktree create / remove / prune 主流程
- Agent Session pane 基础版（运行中 agent 可见、focus、launch、kill）

因此，旧计划中关于“page 结构只有 skeleton、worktree page 尚未真正切换”的描述已过期，不再作为执行依据。

---

## 二、已完成里程碑（按真实进度重述）

### Phase A - Domain / Adapter / Registry Foundation ✅

- `internal/git/worktree.go` - Worktree 领域模型
- `internal/adapters/git.go` / `git_local.go` - Worktree 相关 Git adapter 能力
- `internal/worktree/registry.go` - Worktree registry 基础服务

### Phase B - Worktree UI / Identity / Local Shell ✅

- `PaneTypeWorktree` 与 `WorktreePane` 已落地
- pane meta 已扩展 `RepoID` / `WorktreeID` / `BranchSnapshot`
- editor reuse 已按 worktree 隔离
- `openWorktreeShell` 已按目标 worktree 打开 shell

### Phase C - Worktree Lifecycle Operations ✅

- Create worktree flow 已落地
- Remove / prune flow 已落地
- worktree 删除时已清理依赖 pane

### Phase D - Page Model / Snapshot / Resume Core ✅

- `page` struct 已承载 page-local pane / layout / focus / zoom 状态
- `model` 已使用 `activePage *page` + `pages map[string]*page`
- `newOverviewPage()` / `newWorktreePage()` 已落地
- `PageSnapshot`、`captureSnapshot()`、`restoreSnapshot()` 已落地
- SQLite page snapshot 持久化已落地
- `go test ./...` 当前全绿

### Agent Session Visibility MVP ✅

- `internal/plugins/agents/session_pane.go` 已提供 Agent Session pane
- 能显示运行中 agent、provider、PID、worktree、运行时长
- 支持 focus / launch / kill 基础动作

---

## 三、当前执行重点

当前真正未完成、且优先级最高的工作已经切换为 **Phase 4**：

1. **Human Context Recovery**
2. **Lightweight Task Orchestration**

以下内容先记录，但**暂不执行**：

- hunk-level Git 操作深化
- branch-aware Git actions
- editor / review 更深一层交互打磨
- canvas compositor 转正
- YAML layout 配置化
- 多 agent orchestration

---

## 四、Phase 4 - Human Context Recovery + Lightweight Task Orchestration

### 目标

让 overview 从“状态确认页”升级为“恢复 + 决策 + 分配”的 orchestration hub。

### 需要达成的结果

#### 4.1 Step 1 - Context Schema & Store Foundation（当前执行点）

**文件**:

- `internal/store/db.go`
- `internal/models/ui.go`
- `internal/store/*.go`

**任务**:

- 新增 `task_contexts`
- 新增 `worktree_contexts`
- 新增 `task_worktree_links`
- 新增 `context_notes`
- 扩展 `agent_sessions` 但保持为轻量 lifecycle summary
- 将新的 record / store interface 固化到 `models.Store`

**验收**:

- 新 schema 可在现有 SQLite migration 风格下创建成功
- Store 接口能读写新的 Phase 4 核心实体
- 不引入 event log、transcript store、process restore 等超范围能力

#### 4.2 Step 2 - Resume Summary Pipeline

**文件**:

- `internal/app/app.go`
- `internal/worktree/registry.go`
- `internal/plugins/git/worktree_pane.go`

**任务**:

- 生成 `task + worktree + snapshot + agent` 的 resume summary
- overview 排序转为 resume-first
- task/worktree 摘要成为 UI 直接消费的数据源

**验收**:

- overview 能判断“该恢复哪个任务 / worktree”
- render path 不依赖频繁 DB 查询

#### 4.3 Step 3 - Resume-First Overview UX

**文件**:

- `internal/app/app_test.go`
- `internal/plugins/git/worktree_pane_test.go`
- `internal/plugins/agents/session_pane.go`

**任务**:

- overview 主对象从裸 worktree 升级为 task-in-worktree summary
- `Enter = resume`
- UI 显示 `task title / next step / recent signal / recent agent activity`

**验收**:

- 用户能快速决定“继续哪个任务”
- 恢复路径明确，不再只是打开某个 pane

#### 4.4 Step 4 - Lightweight Task Planning

**任务**:

- 支持创建 / 编辑 task
- 支持 `title / goal / next_step / state`
- 支持 follow-up task 派生

**验收**:

- overview 可直接制定下一步任务
- task 先存在于 context 模型中，不强制立即独立成 worktree

#### 4.5 Step 5 - Task-to-Worktree Allocation

**任务**:

- 支持 task 绑定现有 worktree
- 支持 task promote 成新 worktree
- 支持 `primary / secondary / queued / historical` 关系

**验收**:

- 用户能决定“复用当前容器”还是“升格为新 worktree”

---

## 五、Phase 4 约束（必须保持）

### 目标

在扩展 human context 能力的同时，避免系统逐渐变成 project manager、process manager 或 telemetry sink。

### 约束

- 只持久化 **resume summary / lifecycle summary**，不持久化事件流
- 只在 **launch / exit / reconciliation** 写 agent session，不做 heartbeat 写库
- render path **零 DB 查询**
- `dirty / aheadBehind / shellCount / agentCount` 等运行态信息优先派生，不作为持久化真相
- `page_snapshots` 保持为 layout blob，不增加 shell transcript / pane telemetry
- task 只做轻量 intent object，不进入复杂项目管理语义

## 六、Backlog（记录但暂不做）

- hunk stage / discard / partial review
- review auto-refresh 策略继续深化
- branch / checkout / branch-aware actions
- editor close/focus fallback 深度打磨
- canvas compositor 稳定化与默认启用
- YAML layout 配置化
- agent output pane
- multi-agent orchestration / queue / presets
- transcript / timeline / event sourcing

---

## 七、执行约束

- 每次提交前必须 `go test ./...` 全绿
- 渐进式演进，不做无关大重构
- 新增行为必须补测试
- 文档必须与代码事实同步
- 新持久化字段必须有明确 UI 消费方，否则不入库

---

## 八、下一步动作

立即开始：

1. 完成 **Context Schema & Store Foundation**
2. 进入 **Resume Summary Pipeline**
3. 然后继续到 **Resume-First Overview UX**
