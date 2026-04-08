# focus-tui 开发方向

> 本文档沉淀了 2026-04-07 讨论的核心结论，供后续开发参考。

---

## 定位：运行在终端里的开发者操作系统

focus-tui 的最终形态不是一个 TUI 工具，而是**运行在终端里的开发者操作系统**。

### 与现有工具的本质区别

| 工具 | 局限 |
|------|------|
| 传统终端 | 只负责渲染，不理解用户操作 |
| IDE | 困于 GUI 交互范式，agent 是插件，体验被产品经理管控 |
| lazygit | 懂 git，但不懂 shell 上下文，不知道哪个 worktree 有 agent 在跑 |
| sidecar | 旁观者，只能读，无法编排多 agent |
| **focus-tui** | 同一进程内，shell 状态 + agent 状态 + 任务状态互相可见 |

### 三层架构（长期）

```
层级    功能                                        对应阶段
底层    完全掌控的 shell（主终端）                   阶段一（当前）
中层    与 shell 状态深度绑定的信息面板               阶段二
        （git diff、worktree 状态、Claude 对话历史）
顶层    多 agent 并行工作的编排视图                   阶段三
        （worktree 管理、agent 状态监控）
```

### 核心用户判断

未来的开发是 agent 的天下。终端开发者是最能掌握 agent 的那批人——他们直接操控 worktree、直接编排并行 agent，不被 GUI 产品的交互范式限制。focus-tui 的目标是让"操控 agent"这件事有结构，而不是在 tmux 里开一堆窗口自己心算状态。

---

## 架构原则

**原生优于集成**：git/worktree 工作流必须原生实现，而不是 pane 里开别人的工具。原生集成才能让 git 状态、worktree 状态、agent 状态三者互相感知。

借鉴开源项目（lazygit、sidecar）的源码和思路是合法手段，但目标是把能力内化为 focus-tui 自己的组件，而不是做成其他工具的启动器。

---

## 短期开发优先级

按依赖关系排序，必须顺序推进：

### Phase 1：加固内置 Shell（前提）

**核心指标**：Claude Code 在内置 shell 里跑起来和原生终端一样流畅。

当前实现：PTY + `charmbracelet/x/vt` SafeEmulator，已有基本 key/mouse forwarding。

需要验证和修复的场景：
- Claude Code 进入 alternate screen 后的渲染正确性
- 终端能力查询（DA/DSR/CPR）的响应是否完整（当前有 `io.Copy(p, vtm)` 转发机制）
- 鼠标捕获模式的切换（Claude Code 会启用 SGR 鼠标模式）
- `TIOCGWINSZ` 窗口大小正确传递
- resize 时 vterm + PTY 同步

测试方法：直接在当前 shell pane 跑 `claude`，逐一记录异常。

---

### Phase 2：Multi-Pane Layout（基础设施）

git 面板、claude 历史面板、文件树——所有信息面板都需要在 shell 旁边有空间。没有 layout 系统，后续功能没有容器。

架构变化：
- `shell.Model` 从单例改为可多实例
- 新增 layout engine：支持水平/垂直分割、pane 大小拖拽
- focus 路由：键盘焦点在 pane 间切换
- 每个 pane 有 metadata：`name`、`cwd`、`type`（shell/git/claude-history/filetree）、`status`

参考 lazygit 的 panel 管理思路，但不照搬——focus-tui 的 pane 需要承载非交互式的状态展示面板，不只是 shell。

---

### Phase 3：Git & Diff 工作流（原生）

目标：不离开 focus-tui 完成日常 95% 的 git 操作。

**原生实现范围**（focus-tui 直接构建）：
- worktree 状态面板：列出所有 worktree、各自的 branch、dirty 状态、是否有 agent 在跑
- 当前 repo 状态：branch、ahead/behind、staged/unstaged 文件数
- worktree 创建、切换、删除

**参考 lazygit 源码实现**：
- staged/unstaged 文件列表
- hunk 级 diff 查看
- 交互式 staging
- commit message 编辑

重点：worktree 面板必须与 Phase 4 的 Claude 会话状态联动——同一个面板里能看到"worktree-b 里有 claude agent 正在运行，当前在处理 feature/login"。

---

### Phase 4：Claude Code 对话历史 & Worktree 状态

**数据来源（参考 sidecar 的做法）**：

Claude Code 会把每次会话写入 `~/.claude/projects/<project-slug>/*.jsonl`，每行一条结构化消息：

```go
// 核心字段（来自 sidecar/internal/adapter/claudecode/types.go）
type RawMessage struct {
    Type      string         // "summary" | "user" | "assistant"
    UUID      string
    SessionID string
    Timestamp time.Time
    Message   *MessageContent
    CWD       string
    GitBranch string
    Slug      string
}
```

实现方案：
1. `fsnotify` 监听 `~/.claude/projects/<slug>/` 目录
2. 解析新增的 `.jsonl` 行，提取对话结构
3. 渲染为对话历史面板：显示 user prompt、assistant 回复摘要、tool 调用情况
4. 与 worktree 状态联动：通过 `CWD` + `GitBranch` 字段把会话绑定到对应 worktree

这套方案完全不需要解析 PTY 输出，数据源是 Claude Code 自己落盘的结构化文件，稳定可靠。

---

## 长期天花板（暂不实现）

基于开源终端框架（如 WezTerm、Ghostty）改造出一个新型终端开发者 OS。这是性能和稳定性的上限，当前 `charmbracelet/x/vt` 方案的 vterm 渲染保真度有天花板。

这个方向在 focus-tui 的核心功能跑通之后再评估是否值得投入。

---

## 技术参考

| 功能 | 参考项目 | 参考内容 |
|------|----------|----------|
| Git 工作流 UI | lazygit (`/mnt/d/dev/dev-learn/lazygit`) | panel 管理、diff 渲染、staging 交互 |
| Claude 对话历史 | sidecar (`/mnt/d/dev/dev-learn/sidecar`) | `internal/adapter/claudecode/`：JSONL 解析、fsnotify watcher |
| 文件树 | sidecar / 任意 Go filetree 库 | 轻量实现，导航 + cwd 感知 |
