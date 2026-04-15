# Focus TUI 开发实现计划（完整版）

> 更新日期: 2026-04-16
> 当前阶段: Phase 3B - Page-based Worktree Containers（已完成结构迁移，进入实例隔离阶段）
> 设计参考: [DESIGN-WORKTREE-CONTAINERS-2026-04.md](./DESIGN-WORKTREE-CONTAINERS-2026-04.md)

---

## 已完成里程碑

### Phase A - Domain and Adapter Foundation ✅
- `internal/git/worktree.go` - Worktree 领域模型
- `internal/adapters/git.go` / `git_local.go` - Git adapter 扩展（List/Create/Remove/Prune）
- `internal/worktree/registry.go` - Worktree registry 服务

### Phase B - Worktree UI and App Integration ✅
- `PaneTypeWorktree` 注册与 `WorktreePane` 实现
- Pane meta 扩展 `RepoID` / `WorktreeID` / `BranchSnapshot`
- Editor reuse 已按 `WorktreeID` 隔离
- `openWorktreeShell` 已支持按 worktree 路径 spawn shell

### Phase C - Worktree Lifecycle Operations ✅
- Create worktree overlay flow（`WorktreeCreatePane`）
- Remove / prune worktree 流程（含确认与 pane 清理）
- `WorktreeCreatedMsg` 自动打开新 worktree 的 shell

### Phase 3B 前半 - Page Model Skeleton ✅（已提交 `9c70557`）
- `StateOverviewPage` / `StateWorktreePage` 状态定义
- `page` struct 落地，持有 `panes`、`bodyTree`、`frames`、`focused`、`zoom` 等全部 page-local 状态
- `model` 改为 `activePage *page` + `pages map[string]*page`
- `newOverviewPage()` 从 `New()` 中独立出来
- 所有测试更新并通过 `go test ./...`

---

## 当前状态

**代码层面**：page 结构已存在，但当前始终只有一个 `activePage`（overview page）。`switchToWorktreePage` 只是修改了状态字符串，没有真正切换到独立的 page 实例。

**核心差距**：
1. 没有 `newWorktreePage(worktreeID)` 工厂函数
2. `pages` map 只有一个 `""`（overview）entry
3. 所有 editor/diff/shell 仍然创建在 overview page 上
4. `Ctrl+G` 返回 overview 时，没有保存/恢复 worktree page 的状态

---

## 剩余实现计划

### Phase 3C - Per-Worktree Page Instance Isolation

**目标**：让 `switchToWorktreePage` 真正创建并切换到一个独立的 worktree workspace page。

#### C1. 创建 `newWorktreePage` 工厂函数
- **文件**: `internal/app/page.go`
- **任务**:
  - 实现 `newWorktreePage(common, pluginRegistry, adapterManager, worktreeID, repoRoot) *page`
  - Body tree 默认布局：左侧 `git-status` + `file-tree` 上下分栏，右侧 `shell`（或全屏 `shell`）
  - 自动注册一个绑定到该 worktree 的初始 shell pane
  - 注册 `git-status` pane，CWD 指向 worktree 路径
  - 注册 `file-tree` pane，CWD 指向 worktree 路径
  - Focus 默认落在 shell
- **验收**:
  - `newWorktreePage` 返回的 page 有独立的 pane 集合和 bodyTree
  - 该 page 的 paneMeta 全部带有正确的 `WorktreeID`

#### C2. 让 `openWorktreeShell` 自动进入 worktree page
- **文件**: `internal/app/app.go`, `internal/app/page.go`
- **任务**:
  - 修改 `model.openWorktreeShell`：如果当前不在目标 worktree 的 page，先 `switchToWorktreePage(worktreeID, "")`
  - 然后在对应的 worktree page 上创建 shell pane（而不是在 overview page 上 split）
  - 更新 `WorktreeCreatedMsg` 的处理逻辑：创建成功后自动切到 worktree page 并打开 shell
- **验收**:
  - 从 worktree pane 按 Enter 进入的是全屏 worktree workspace page
  - 新 shell 出现在 worktree page 中，不出现在 overview page

#### C3. 实现真正的 page 切换（overview ↔ worktree）
- **文件**: `internal/app/app.go`, `internal/app/page.go`
- **任务**:
  - 修改 `switchToWorktreePage(worktreeID, preferredPane)`：
    - 如果 `pages[worktreeID]` 不存在，调用 `newWorktreePage` 创建
    - 设置 `m.activePage = pages[worktreeID]`
    - 设置 `m.state = StateWorktreePage`
    - 恢复该 page 的 focus（或设置为 preferredPane）
    - 触发 `updateSizes`
  - 修改 `switchToOverviewPage()`：
    - 设置 `m.activePage = pages[""]`
    - 设置 `m.state = StateOverviewPage`
    - 恢复 overview 的 focus（如 `paneWorktree`）
    - 触发 `updateSizes`
- **验收**:
  - 连续切换 overview 和多个 worktree page 时，各自的 layout 和 focus 保持独立

#### C4. 更新测试覆盖 page 切换行为
- **文件**: `internal/app/app_test.go`
- **任务**:
  - 测试 `switchToWorktreePage` 会创建新的 page 实例
  - 测试在 worktree page 上 split shell 不会影响 overview page 的 bodyTree
  - 测试 `Ctrl+G` 返回 overview 后，overview 的原始 layout 不变
  - 测试 `WorktreeCreatedMsg` 后自动进入 worktree page
- **验收**:
  - `go test ./...` 全绿

---

### Phase 3D - Worktree Page Content Completeness

**目标**：让 worktree page 具备与 overview 同等的核心生产力 pane。

#### D1. Worktree page 支持 editor / diff / commit overlay
- **文件**: `internal/app/page.go`, `internal/app/app.go`
- **任务**:
  - 确保 `openEditorPane`、`openDiffPane`、`openCommitPane` 在当前 `activePage` 上执行
  - 验证 editor reuse key（`filepath + WorktreeID`）在 page 隔离下仍然正确
  - diff / commit overlay 的 `CWD` 和 `WorktreeID` 指向当前 active worktree
- **验收**:
  - 在 worktree page 中打开 editor、diff、commit overlay 正常工作
  - 同一文件在不同 worktree page 中打开产生独立 editor pane

#### D2. Worktree page 的 git status 与 file tree 联动
- **文件**: `internal/plugins/git/worktree_pane.go`, `internal/plugins/git/git_status_pane.go`
- **任务**:
  - 确保 worktree page 的 `git-status` pane 监听的是当前 worktree 路径的状态
  - file tree 的根目录绑定到当前 worktree 路径
  - 从 file tree 打开文件时，editor 创建在正确的 worktree page 上
- **验收**:
  - 在 worktree A 的 page 中看到的 git status 是 worktree A 的
  - 在 worktree B 的 page 中不会看到 worktree A 的改动

---

### Phase 3E - Layout Snapshot and Resume

**目标**：保存和恢复每个 worktree page 的 layout 与最近打开的文件。

#### E1. Page layout snapshot
- **文件**: `internal/app/page.go`, `internal/app/state.go`
- **任务**:
  - 定义 `PageSnapshot` struct：包含 `BodyTree` 序列化表示、`Focused`、`OpenEditors`（文件路径列表）
  - 在 `switchToOverviewPage` 离开 worktree page 前，调用 `snapshot := m.activePage.snapshot()` 保存到 `m.pages[worktreeID]` 或持久化层
  - 在 `switchToWorktreePage` 时，如果存在 snapshot，恢复 `bodyTree` 和 focus
- **验收**:
  - 离开并返回 worktree page 时，layout 和 focus 与离开时一致

#### E2. 持久化 worktree page metadata
- **文件**: `internal/store/*` 或 `internal/worktree/registry.go`
- **任务**:
  - 在 SQLite 或 JSON 文件中存储每个 worktree 的：
    - 最后活跃时间
    - layout snapshot
    - 最近打开的文件列表
  - App 启动时从持久化层加载 snapshot，预热 `pages` map
- **验收**:
  - 重启应用后，进入 worktree page 能恢复上次的 layout 和打开的文件

---

### Phase 3F - Overview Page Orchestration UX

**目标**：让 overview page 成为真正的 orchestration hub，而不仅仅是旧布局的别名。

#### F1. Overview page 的 bodyTree 简化
- **文件**: `internal/app/page.go`
- **任务**:
  - Overview page 的 bodyTree 以 `WorktreePane` 为主，右侧或下方可保留一个全局 shell（可选）
  - 移除 overview page 中绑定到特定 worktree 的 git-status / file-tree（这些属于 worktree page）
- **验收**:
  - Overview page 只显示 worktree 列表和可能的系统级 pane（todo / pomodoro）

#### F2. Worktree pane 增强导航
- **文件**: `internal/plugins/git/worktree_pane.go`
- **任务**:
  - 在 worktree list 中显示每个 worktree 的最近活跃时间、打开的文件数、是否有运行中的 shell
  - 支持 `Enter` 打开 worktree page，`d` 删除，`n` 新建，`r` 刷新
- **验收**:
  - Worktree pane 提供足够信息帮助用户选择要恢复的任务

---

## 开发顺序建议

按以下顺序执行，风险最低：

1. **C1** → `newWorktreePage` 工厂函数
2. **C2** → `openWorktreeShell` 绑定到 worktree page
3. **C3** → 真正的 `switchToWorktreePage` / `switchToOverviewPage`
4. **C4** → 测试覆盖
5. **D1** → editor/diff/commit 在 worktree page 中正常工作
6. **D2** → git status / file tree 按 worktree 隔离
7. **E1** → layout snapshot（内存级别）
8. **E2** → 持久化
9. **F1/F2** → overview page UX 优化

---

## 关键约束

- **每次提交前必须 `go test ./...` 全绿**
- **渐进式演进，不做大规模重构**
- **所有新增代码需有测试覆盖**
- **保持现有无 worktree 场景的行为不变（向后兼容）**

---

## 下一步行动

如果你确认这个计划，我将立即开始实现 **Phase 3C**（C1: `newWorktreePage` 工厂函数）。
