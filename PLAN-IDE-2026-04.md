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
- [ ] `push` 完全缺失
- [ ] `stage all / unstage all` 缺失
- [ ] `fetch / pull / sync` 缺失
- [ ] hunk 级操作缺失

### 编辑器现状

- [x] File Tree pane 已可浏览目录
- [x] 目录展开/折叠已可用
- [ ] 文件节点 `enter` 打开文件能力缺失
- [ ] 通用 Editor pane type 缺失
- [ ] 文件 buffer / dirty 状态 / save 流程缺失
- [ ] TUI 内编辑能力缺失

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

- [ ] 在 `GitAdapter` 增加 `Push(repoPath string) error`
- [ ] 在 `GitLocalAdapter` 实现 `git push`
- [ ] Git Status pane 增加 `P` 快捷键
- [ ] Push 完成后自动刷新 status
- [ ] Push 失败时在 pane 中显示错误

涉及文件：

- `internal/adapters/git.go`
- `internal/adapters/git_local.go`
- `internal/plugins/git/status_pane.go`

### A2. Stage All / Unstage All

- [ ] 在 `GitAdapter` 增加批量暂存接口
- [ ] 在 `GitLocalAdapter` 实现 `git add --all` / `git reset HEAD -- .`
- [ ] Git Status pane 增加 `a` 快捷键
- [ ] 动作后自动刷新 status

涉及文件：

- `internal/adapters/git.go`
- `internal/adapters/git_local.go`
- `internal/plugins/git/status_pane.go`

### A3. Commit 流程增强

- [ ] 保留现有 commit overlay
- [ ] 明确 help line 中的 commit / push / stage-all 快捷键
- [ ] commit 完成后仍保持 status refresh 闭环

涉及文件：

- `internal/plugins/git/commit_pane.go`
- `internal/app/app.go`

### A4. 验证

- [ ] 补充 `status_pane_test.go` 中的 push / batch stage 测试
- [ ] 跑通 Git pane 相关测试
- [ ] `go test ./...`

---

## Phase B：File Tree → TUI Editor MVP

目标：让 File Tree 成为终端 IDE 的文件入口，而不是只读浏览器。

### B1. 打开文件

- [ ] File Tree 在文件节点上触发 `OpenEditorMsg`
- [ ] app 层接收消息并创建 editor pane

### B2. Editor MVP

- [ ] 新增 `PaneTypeEditor`
- [ ] 用 `textarea.Model` 实现多行编辑
- [ ] 支持文件加载、编辑、保存、dirty 状态
- [ ] 支持 `ctrl+s` 保存、`esc` 关闭/确认放弃

### B2.1. Editor 设计约束（本轮确认）

- [ ] **Editor 是正常 pane，不是 overlay**
- [ ] **File Tree 只负责发 `OpenEditorMsg`，不承担编辑逻辑**
- [ ] **MVP 先做单文件/单 pane 编辑，不做完整 buffer 管理器**
- [ ] **先支持 UTF-8 文本文件，不处理二进制与超大文件**
- [ ] **保留后续演进空间：Buffer/Window 分离、Split、搜索、只读预览**

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

1. File Tree 文件节点按 `enter/o` 可以打开 editor pane
2. `v` 可以以 split 方式打开 editor pane
3. editor 能加载文件内容并编辑
4. `ctrl+s` 可以保存到磁盘
5. 未保存时 `esc` 不会直接关闭，而是要求确认
6. `go test ./...` 全绿

### B3. 打开方式

- [ ] `enter/o`：当前方式打开
- [ ] `v`：右侧 split 打开

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

- 文件树打开文件
- editor pane 骨架
- 保存 / dirty 状态

### 第 3 周

完成 **Editor 交互增强**：

- split 打开完善
- 常规 pane 生命周期打磨
- 焦点与关闭回退

### 第 4 周

增强层：

- hunk staging
- fetch / pull / branch actions
- editor 搜索 / 跳行 / 外部编辑器 fallback

---

## 当前立即执行项

本次从 **Phase A / Git workflow MVP** 开始，首批落地范围：

- [ ] Push 快捷键 `P`
- [ ] Stage all / Unstage all 快捷键 `a`
- [ ] Git adapter 扩展与测试
- [ ] Git pane help line 更新

验收标准：

1. Git Status pane 中可使用 `space` 单文件暂存/取消暂存
2. Git Status pane 中可使用 `a` 批量暂存/取消暂存
3. Git Status pane 中可使用 `c` commit
4. Git Status pane 中可使用 `P` push
5. 所有动作后状态自动刷新
6. 相关测试通过，且 `go test ./...` 全绿
