# 终端 IDE 核心功能优先计划（2026-04）

## 背景

在完成默认布局重构后，当前 `Focus-tui` 已具备基础工作台形态，但距离“可日常使用的终端 IDE”还差两个关键闭环：

1. **Git 工作流不完整**：已有状态列表、单文件暂存、Diff、Commit，但缺少 Push、批量暂存/取消暂存，以及更明确的日常快捷键闭环。
2. **文件树不可编辑**：当前 File Tree 只能浏览目录和文件，不能从树中直接打开文件并在 TUI 内编辑。

因此，本轮开发优先级调整为：

1. **Git workflow MVP**
2. **File Tree → TUI Editor MVP**
3. 布局增强（Overlay 收敛 / 侧栏折叠 / YAML 布局）

---

## 已确认现状

### Git 现状

- [x] Git Status pane 已接入默认布局
- [x] 单文件 `stage / unstage` 已可用
- [x] `enter` 查看 diff 已可用
- [x] `c` 打开 commit overlay 已可用
- [x] commit 成功后自动 refresh 已可用
- [x] `push` 已可用
- [x] `stage all / unstage all` 已可用
- [x] `fetch / pull / sync` 已可用
- [ ] hunk 级操作缺失
- [x] Git pane 已具备基础状态保护与同步反馈
- [x] review diff 已是正常 body pane，不再是临时 overlay
- [x] review diff 支持多文件 stacked view、file section、hunk header
- [x] review diff 支持 review -> editor 打开
- [x] review diff 支持 review 内文件跳转与鼠标滚轮滚动
- [x] review diff 支持 staged / unstaged 视角切换
- [x] review diff 支持按当前 hunk 打开到 editor 目标行
- [x] rename / binary / new / delete diff 已有基础可读展示

### 编辑器现状

- [x] File Tree pane 已可浏览目录
- [x] 目录展开/折叠已可用
- [x] 文件节点 `enter/o` 可打开文件
- [x] 文件节点 `v` 可按 split 语义打开 editor
- [x] 通用 Editor pane type 已接入
- [x] 文件加载、编辑、保存已可用
- [x] dirty 状态与关闭确认已可用
- [x] 同一路径 editor pane 复用已可用
- [ ] pane title / dirty / split / 焦点恢复仍需进一步打磨

### 当前阶段总结

截至当前阶段，`Focus-tui` 已完成：

- Git workflow MVP：stage / stage-all / diff / commit / fetch / pull / push
- Git pane 的 upstream / ahead / behind / diverged 基础保护
- Editor MVP 第一阶段：从 File Tree 打开 editor、编辑、`ctrl+s` 保存、`esc` 未保存确认、dirty title 标记、同文件复用
- Git review workflow 第一阶段：review pane 正式并入工作区布局，支持 stacked diff、section/hunk 层次、文件跳转、鼠标滚轮、staged/unstaged 切换、review → editor 定位

当前真正未完成的主线，已经从“有没有 Git/editor 工作流”转为“review 与 editor 的联动是否足够顺手，以及更高阶的 Git 操作是否要继续深入”。

---

## 参考来源

### LazyGit

用于参考 Git 工作流与快捷键收敛方式：

- `space`：stage / unstage
- `a`：stage / unstage all
- `c`：commit
- `P`：push

关键参考方向：

- Files controller 的快捷键绑定与动作分层
- Push 前的 upstream / ahead / behind 防护逻辑
- Commit 面板和刷新流程

### LunarVim

用于参考“文件树负责选择，编辑器负责打开”的职责划分：

- `<CR> / o / l`：打开文件
- `v`：vertical split 打开
- 文件树不承担编辑逻辑本身，而是触发 editor/buffer 打开流程

### Neovim

用于参考编辑器内核的状态分层，而不是直接照搬复杂度：

- Buffer 与 Window 分离：文件内容与显示视图分离
- `modified` / `changedtick`：dirty 状态必须是一等公民
- 打开 / 编辑 / 保存 / 关闭是明确生命周期
- split 属于布局系统，不属于编辑器控件本身

本项目只借鉴其架构思路，不引入 swapfile、undo tree、脚本系统、syntax engine 等重型能力。

---

## 开发顺序

## Phase A：Git workflow MVP（最高优先级）

目标：让 `Focus-tui` 首次具备可日常使用的 Git 闭环。

### A1. Push 能力

- [x] 在 `GitAdapter` 增加 `Push(repoPath string) error`
- [x] 在 `GitLocalAdapter` 实现 `git push`
- [x] Git Status pane 增加 `P` 快捷键
- [x] Push 完成后自动刷新 status
- [x] Push 失败时在 pane 中显示错误

涉及文件：

- `internal/adapters/git.go`
- `internal/adapters/git_local.go`
- `internal/plugins/git/status_pane.go`

### A2. Stage All / Unstage All

- [x] 在 `GitAdapter` 增加批量暂存接口
- [x] 在 `GitLocalAdapter` 实现 `git add --all` / `git reset HEAD -- .`
- [x] Git Status pane 增加 `a` 快捷键
- [x] 动作后自动刷新 status

涉及文件：

- `internal/adapters/git.go`
- `internal/adapters/git_local.go`
- `internal/plugins/git/status_pane.go`

### A3. Commit 流程增强

- [x] 保留现有 commit overlay
- [x] 明确 help line 中的 commit / push / stage-all 快捷键
- [x] commit 完成后仍保持 status refresh 闭环
- [x] commit 快捷键改为 `ctrl+s`，`ctrl+j` 保留 fallback

涉及文件：

- `internal/plugins/git/commit_pane.go`
- `internal/app/app.go`

### A4. 验证

- [x] 补充 `status_pane_test.go` 中的 push / batch stage 测试
- [x] 跑通 Git pane 相关测试
- [x] `go test ./...`

### A5. Sync Guard / Feedback（已完成）

- [x] `push` 对 no-upstream / behind / diverged 做显式保护
- [x] `pull` 对 no-upstream / diverged / already-up-to-date 做显式保护
- [x] `fetch / pull / push` 成功后显示 notice
- [x] 补充对应回归测试

---

## Phase B：File Tree → TUI Editor MVP

目标：让 File Tree 成为终端 IDE 的文件入口，而不是只读浏览器。

### B1. 打开文件

- [x] File Tree 在文件节点上触发 `OpenEditorMsg`
- [x] app 层接收消息并创建 editor pane

### B2. Editor MVP

- [x] 新增 `PaneTypeEditor`
- [x] 用 `textarea.Model` 实现多行编辑
- [x] 支持文件加载、编辑、保存、dirty 状态
- [x] 支持 `ctrl+s` 保存、`esc` 关闭/确认放弃

### B2.1. Editor 设计约束（本轮确认）

- [x] **Editor 是正常 pane，不是 overlay**
- [x] **File Tree 只负责发 `OpenEditorMsg`，不承担编辑逻辑**
- [x] **MVP 先做单文件/单 pane 编辑，不做完整 buffer 管理器**
- [x] **先支持 UTF-8 文本文件，不处理二进制与超大文件**
- [x] **保留后续演进空间：Buffer/Window 分离、Split、搜索、只读预览**

### B2.2. Editor 状态模型（MVP）

首批 editor pane 采用轻量状态模型：

- `filePath string`
- `originalContent string`
- `dirty bool`
- `changeTick int64`
- `confirmClose bool`

说明：

- `dirty` 用于 pane title、保存按钮和关闭确认
- `changeTick` 用于后续做增量刷新和更细粒度状态同步
- 这一版暂不实现多 window 共享同一 buffer，但设计上不阻塞后续升级

### B2.3. 打开与分屏策略（MVP）

- `enter / o`：从 File Tree 打开 editor
- `v`：右侧 split 打开 editor
- 默认优先在主工作区（shell/editor 区）分裂，不在左侧导航列内打开

说明：File Tree 继续作为导航列存在，editor 在主工作区中打开，符合 terminal IDE 的工作流预期。

### B2.4. 首批文件设计

- `internal/plugins/editor/plugin.go`
- `internal/plugins/editor/editor_pane.go`
- `internal/plugins/editor/messages.go`
- `internal/plugins/editor/editor_pane_test.go`

涉及修改：

- `internal/models/pane.go`
- `internal/plugins/filebrowser/tree_pane.go`
- `internal/plugins/filebrowser/tree_pane_test.go`
- `internal/app/app.go`

### B2.5. MVP 验收标准

1. [x] File Tree 文件节点按 `enter/o` 可以打开 editor pane
2. [x] `v` 可以以 split 方式打开 editor pane
3. [x] editor 能加载文件内容并编辑
4. [x] `ctrl+s` 可以保存到磁盘
5. [x] 未保存时 `esc` 不会直接关闭，而是要求确认
6. [x] `go test ./...` 全绿
7. [x] 再次打开同一文件时复用已有 editor pane
8. [x] dirty 状态同步到 pane title

### B3. 打开方式

- [x] `enter/o`：当前方式打开
- [x] `v`：右侧 split 打开

涉及文件（预期）：

- `internal/plugins/filebrowser/tree_pane.go`
- `internal/app/app.go`
- `internal/models/pane.go`
- `internal/plugins/editor/plugin.go`（新增）
- `internal/plugins/editor/editor_pane.go`（新增）

---

## 布局线的处理策略

以下工作不取消，但下调优先级：

- Todo / Pomodoro Overlay 收敛
- 左侧栏折叠（`Ctrl+B`）
- YAML 布局配置

原因：这些工作提升的是结构一致性与可配置性，但当前阻塞日常开发效率的是 Git 闭环和文件编辑能力。

---

## 预计排期

### 第 1 周

完成 **Git workflow MVP**：

- Push
- Stage all / Unstage all
- Git pane help 更新
- 测试与回归

### 第 2 周

完成 **File Tree → Editor pane MVP**：

- [x] 文件树打开文件
- [x] editor pane 骨架
- [x] 保存 / dirty 状态

### 第 3 周

完成 **Editor 交互增强**：

- [ ] split 打开完善
- [~] 常规 pane 生命周期打磨（已完成同文件复用与 dirty title）
- [ ] 焦点与关闭回退

### 第 4 周

增强层：

- hunk staging
- fetch / pull / branch actions
- editor 搜索 / 跳行 / 外部编辑器 fallback
- review pane staged/unstaged 切换与 review → editor 行定位（已完成）
- review pane 文件级导航与鼠标滚轮（已完成）

---

## 当前立即执行项

当前 Git / worktree / review 工作流已经**基本可日常使用**。下一步推荐执行项：

- [ ] hunk 级 stage / discard / partial review 操作
- [ ] review pane refresh / auto-refresh 策略
- [ ] review 当前文件 / 当前 hunk 的高亮与上下文强化
- [ ] branch / checkout / branch-aware actions
- [ ] 继续打磨 editor 与 review 的上下文回退

---

## Handoff（供新 Session 直接续接）

### 当前代码状态

- `master` 已包含 Git workflow MVP、Editor MVP，以及 review workflow 第一阶段全部提交
- Editor 相关代码已落地到：
  - `internal/plugins/editor/`
  - `internal/plugins/filebrowser/tree_pane.go`
  - `internal/app/app.go`
  - `internal/models/pane.go`
- 全量测试在最新提交时为绿色：`go test ./...`

### Editor 已实现能力

- File Tree 文件节点 `enter/o` 打开 editor
- `v` 以 split 语义打开 editor
- `textarea.Model` 多行编辑
- `ctrl+s` 保存

### Review 已实现能力

- Git Status 中 `enter` 打开 review pane，`d` 打开单文件 diff
- review pane 进入正常工作区布局，不再是 diff overlay
- 多文件 stacked diff、file section、hunk header
- `enter` 从 review 打开 editor，并尽量定位到当前 hunk 行
- `[` / `]` 在文件 section 之间跳转
- 鼠标滚轮滚动 review pane
- `s` 切换 staged / unstaged review
- rename / binary / new / delete 的基础可读渲染
- `esc` 未保存确认
- 同一路径 editor pane 复用
- dirty 状态同步到 pane title（如 `*main.go [modified]`）

### 下一步最自然的开发入口

1. `internal/app/app.go`
   - 继续完善 `openEditorPane()`
   - 打磨 editor 关闭后的 focus fallback
2. `internal/plugins/editor/editor_pane.go`
   - 加入 mtime / 外部变更检测
   - 视需要加入只读/preview 模式
3. `internal/app/app_test.go`
   - 为 editor close/focus 恢复补 app 级测试

### 不建议在下一 Session 重做的事

- 不需要再重新研究 Neovim / LunarVim 参考结论
- 不需要再重新设计 editor 是 overlay 还是 pane —— 该决策已经确定：**editor 是正常 pane**
- 不需要再重新验证 Git workflow 是否可用 —— 当前阶段应把主要精力放在 editor 交互打磨上
