# 布局重构与交互收敛计划（2026-04）

> [过时归档说明]
> 自 2026-04-19 起，本文档已被 ADR-first 方向 supersede。
> 保留仅作历史阶段计划参考，不再作为当前实现依据。

## 目标

在不破坏现有视觉风格（尤其是 Header 的 `focus` ASCII art）的前提下，完成默认工作区布局重构：

- Header 保持当前样式，只新增简要状态标识：`🍅xN`、`✓M/K`
- 默认 Body 改为：左侧堆叠 `Git Status + File Tree`，右侧 `Shell`
- Todo/Pomodoro 不再作为默认独立工作区 Pane
- 后续支持 YAML（方案 B）自定义布局，且用户配置优先生效

---

## 用户明确约束

1. 不破坏当前 Header 的整体布局和 ASCII art
2. Todo/Pomodoro 仅需状态标识，不需要常驻独立 Pane
3. 需要复用原有快捷键，在无特定 focus 状态时以**居中弹窗**方式进行 Todo/Pomodoro 操作
4. 左侧 Git 区采用上下堆叠（参考 sidecar 风格）
5. 保留现有 resize / toggle / split 机制，不做破坏性改动
6. 可以增加快速收起左侧 Git 区快捷键（计划使用 `Ctrl+B`，需避免冲突）
7. 用户提供 YAML 时严格按配置布局渲染

---

## 本次迭代已完成

- [x] Header 已新增状态字段（todoDone/todoTotal/pomoCount）
- [x] Header 展示已接入 `🍅xN` 与 `✓M/K`
- [x] Header 在初始化时立即拉取统计数据，并进行周期刷新
- [x] 默认 Body 布局已改为：
  - 左侧：`Git Status`（上） + `File Tree`（下）
  - 右侧：`Shell`（主工作区）
- [x] `internal/app/app_test.go` 已同步更新默认布局断言
- [x] 构建通过：`go build ./...`
- [x] 全量测试通过：`go test ./...`

---

## 下一步实现清单

### A. 居中弹窗交互（Todo/Pomodoro）

目标：保留现有快捷键语义，但改为中心弹层工作流。

- [ ] 统一 Overlay 类型（Todo Modal / Pomodoro Modal）
- [ ] Todo 快捷键映射到居中弹窗（新增、编辑、删除确认）
- [ ] Pomodoro 快捷键映射到居中弹窗（开始/暂停/重置）
- [ ] 弹窗关闭后焦点回退到原 Pane

涉及文件（预期）：

- `internal/app/app.go`
- `internal/ui/todo/*`
- `internal/ui/pomodoro/*`
- `internal/ui/layout/layout.go`（如需通用弹层尺寸）

### B. 左侧栏快速收起（Ctrl+B）

目标：一键折叠/恢复左侧 Git 栏，且不影响现有 split/resize。

- [ ] 校验 `Ctrl+B` 无现有冲突
- [ ] 增加侧栏折叠状态（保存前宽度，恢复时回填）
- [ ] 极窄窗口下行为降级策略
- [ ] Help Line 同步提示快捷键

涉及文件（预期）：

- `internal/app/app.go`
- `internal/app/app_test.go`

### C. YAML 布局（方案 B）

目标：默认布局可配置化，且用户 YAML 优先。

- [ ] 定义布局配置结构（split/pane 树）
- [ ] 实现 LayoutBuilder（配置 -> TreeNode）
- [ ] 默认配置写入新布局（左 Git+Tree / 右 Shell）
- [ ] 用户配置存在时覆盖默认

涉及文件（预期）：

- `internal/config/config.go`
- `internal/config/layout.go`（新增）
- `internal/config/defaults.go`（新增）
- `internal/app/layout_builder.go`（新增）
- `internal/app/app.go`

---

## 风险与回归检查

1. **焦点路由风险**：移除默认 Todo/Pomodoro Pane 后，快捷键路由可能悬空
   - 对策：统一走 overlay 入口，显式定义 focus fallback

2. **快捷键冲突风险**：新增 `Ctrl+B` 与现有绑定冲突
   - 对策：先扫描 key map，再加入并测试

3. **布局破坏风险**：收起侧栏可能影响 split 比例与恢复
   - 对策：保存折叠前 ratio，恢复时回填；增加行为测试

4. **兼容性风险**：自定义 YAML 与默认布局并存
   - 对策：无配置走默认；有配置严格按用户定义

---

## 验收标准

1. Header 保持现有观感，仅增加状态标识，不破坏 ASCII art
2. 默认启动即呈现左 Git+Tree、右 Shell
3. Todo/Pomodoro 操作可通过居中弹窗完成
4. `Ctrl+B` 可折叠/恢复左栏且不影响已有交互
5. YAML（方案 B）可覆盖默认布局
6. `go build ./...` 与 `go test ./...` 全绿
