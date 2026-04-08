# focus-tui 下一步开发接力

> 本文档用于下次直接续开发。
> 现行方向以 `DIRECTION.md` 为准；本文档只记录当前阶段、约束、切入点和首要任务。

## 当前阶段

- 当前处于 `Phase 2`
- `Phase 1` 已基本完成：内置 shell 已达到可用于真实开发工作流的可信程度
- `Phase 2.3` 已基本完成：pane 容器已从骨架推进到可长期使用的第一版
- `Phase 2` 已完成的部分：
  - pane metadata 基础模型
  - layout tree 基础结构
  - pane focus 路由骨架
  - shell pane 的分割 / 新建 / 关闭
  - 固定 pane 与可关闭 pane 的语义约束
  - split ratio 调整
  - 关闭 pane 后相邻 focus fallback
  - shell pane 的 `starting / running / exited` 状态展示
  - shell pane 短 `cwd` 标题
  - 多 shell 场景下的消息风暴修复
  - 多 shell 窄布局下的标题降级与 resize 易用性改进

## 当前约束

- `Todo` 和 `Pomodoro` 目前仍是固定 pane，不允许关闭
- 当前只有 `Shell` 是完整支持创建 / 分割 / 关闭的会话型 pane
- `Header` 和 `Footer` 是系统 pane，不参与普通 pane 生命周期
- 所有新增功能都应建立在当前 pane 语义之上，不要重新引入“所有 pane 默认都可关闭”的假设

## 当前进度快照

- 当前代码已经可以稳定支持多 shell pane 并存，且不会再因 shell 内部消息广播导致整机卡死
- `ctrl+\\` / `ctrl+-` 可分割 shell pane，`ctrl+w` 可关闭当前 shell pane
- `ctrl+arrows` 与 `ctrl+shift+arrows` 都可以调整当前 pane 所在 split 的比例
- pane 标题会根据宽度自动降级，优先丢弃 `cwd` 与状态标签，避免窄 pane 直接把边框撑坏
- 自动化测试已覆盖 `layout/tree.go` 核心逻辑、shell scoped 消息、部分 app 标题与路由行为
- 当前仍未彻底解决的问题主要是“空间不够时 shell 内容本身会拥挤”，这是后续布局策略问题，不是当前的卡死类 bug

## 代码现状

- pane 元数据定义：`internal/models/pane.go`
- layout tree 与 frame/focus 工具：`internal/ui/layout/tree.go`
- 顶层 pane registry、focus、split/close 路由：`internal/app/app.go`
- shell pane 实现：`internal/ui/shell/shell.go`

## 下一步主线

### Phase 2.4

目标：继续把 pane 系统从“能长期使用”推进到“多 shell 下明显顺手”。

优先顺序：

1. 小窗口 / 极端尺寸下的布局约束与降级继续细化
2. app 层核心交互测试补强
3. 多 shell 临时放大 / zoom pane 能力
4. 进入 git/worktree pane 的容器准备

### 具体任务

1. 为 `app.go` 增加 split / close / focus / mode 的行为测试
2. 继续打磨极窄布局下的 pane 最小宽高与标题/帮助栏降级规则
3. 评估并实现当前 focused pane 的临时放大能力（toggle zoom）
4. 为后续 `git/worktree/history pane` 进入容器层做接口准备
5. 保持 `go test ./...` 为每轮开发后的固定动作

## 建议快捷键

- `ctrl+\\`：左右分割当前 pane
- `ctrl+-`：上下分割当前 pane
- `ctrl+w`：关闭当前 shell pane
- `tab` / `shift+tab`：循环切换 pane
- `ctrl+h/j/k/l`：方向切换 pane
- 当前已支持：
  - `ctrl+left/right/up/down`：调整当前 split 比例
  - `ctrl+shift+left/right/up/down`：同上（兼容保留）
- 候选新增：
  - `z` / `ctrl+enter`：临时放大当前 pane（待实现）

## 开发原则

- 不绑定单一 agent，所有面向 agent 的设计都应保持 agent-agnostic
- `Todo` / `Pomodoro` 不删除，作为原生工作台组件继续保留
- 尽量延续当前最小正确改动风格，不做大规模无关重构
- 在 `Phase 2` 内，优先把 pane 容器打稳，再接 git/worktree/history pane

## 下次开工建议顺序

1. 先读 `DIRECTION.md`
2. 再读本文件
3. 优先从 `internal/app/app_test.go` 开始，补 app 层 split / close / focus / mode 的行为测试
4. 完成后运行 `go test ./...`

## Phase 2.3 执行计划

目标：把 pane 系统从“可演示、可轻用”推进到“可长期使用”。

### 工作包 A：布局编辑能力

范围：`internal/ui/layout/tree.go`、`internal/app/app.go`

1. 为 split node 增加可编辑操作，而不只是渲染时读取 `Ratio`
2. 支持根据 focused pane 找到可调整的父 split
3. 接入 `ctrl+shift+h/l/j/k` 快捷键调整比例
4. 保持最小宽高约束，避免 pane 被挤坏
5. 明确极小窗口下的 ratio clamp 规则

完成标准：

- 可以稳定调节左右 / 上下分割比例
- 连续调整不会产生负尺寸、重叠或不可见 pane

### 工作包 B：焦点与关闭行为

范围：`internal/ui/layout/tree.go`、`internal/app/app.go`

1. 关闭 pane 后优先将焦点落到相邻 pane
2. 没有明确相邻 pane 时，再回退到遍历顺序 fallback
3. 检查 split / close 后 `focused`、`mode`、`status` 是否仍一致

完成标准：

- `ctrl+w` 后焦点行为符合直觉
- 不出现焦点丢失或落到已删除 pane 的情况

### 工作包 C：shell pane 元数据可信化

范围：`internal/ui/shell/shell.go`、`internal/app/app.go`

1. shell 标题显示短 `cwd`
2. 区分 pane 的 UI focus 状态与 shell 会话运行状态
3. 为 shell 展示 `starting / running / exited / active` 等可见状态
4. 更新 help bar，让当前 pane 的可操作提示更清晰

完成标准：

- 多 shell 并存时能快速识别各自状态与上下文
- pane 标题信息可信，不依赖用户猜测

### 工作包 D：测试与回归流程

范围：`internal/ui/layout/tree_test.go`、`internal/app/*`、项目文档、CI

1. 为 layout 纯逻辑补单元测试
2. 逐步为 app 层补关键行为测试
3. 把 `go test ./...` 固化为每次改动后的默认检查
4. 增加最小 CI，至少保证仓库提交后自动跑测试

完成标准：

- `layout` 不再处于无测试保护状态
- 每次功能改动后都有固定验证动作

## 推荐实现顺序

1. 先补 `internal/ui/layout/tree_test.go`
2. 实现 ratio 调整 API，并让测试覆盖新增逻辑
3. 在 `internal/app/app.go` 接入调整快捷键
4. 改造 close pane 的 focus fallback
5. 补 shell 标题 / 状态展示
6. 继续为 app 层交互补测试

## 测试策略

当前阶段不追求重型端到端测试，按收益分层推进：

1. 单元测试：优先覆盖 `layout/tree.go` 这类纯逻辑
2. 行为测试：逐步覆盖 `app.go` 的 split / close / focus / mode 路由
3. 手工回归：shell、PTY、resize、mouse 等复杂交互维持人工验证清单

最先应补的自动化测试项：

1. `ComputeFrames`
2. `LeafOrder`
3. `SplitLeaf`
4. `RemoveLeaf`
5. `MoveFocus`
6. 后续的 ratio 调整函数

## 已知风险

- app 层仍存在对固定 pane 的直接引用，这在当前语义下是允许的，但新增可关闭 pane 类型时要重新检查
- shell pane 的 `cwd` 目前是 metadata 级继承，不是实时会话状态同步
- 极窄终端下虽然边框/标题不会先坏掉，但 shell 内容区本身仍会显著拥挤
- 当前还没有 focused pane 的临时最大化能力，多 shell 下快速“看清一个 pane”仍不够顺手
- app 层关键交互还没有足够完整的行为测试，后续重构仍有回归风险

## 完成定义

`Phase 2.4` 完成时，至少应满足：

- 多 shell pane 长时间使用不混乱
- 可调比例，不会轻易把 pane 挤坏
- 关闭 pane 后焦点合理
- pane 标题和状态信息比当前更可信
- 多 shell 下需要快速看清单个 pane 时，有明显更顺手的操作手段
