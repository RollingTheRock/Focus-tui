# focus-tui Testing

## 目标

当前阶段的测试目标不是把整个 TUI 做成重型端到端测试，而是先为最容易退化的核心逻辑建立自动保护。

优先级：

1. `layout` 纯逻辑单元测试
2. `app` 关键交互行为测试
3. `shell` 最小可稳定验证的测试
4. shell / PTY / resize / mouse 的人工回归

## 日常开发流程

每次改动默认执行：

```bash
go test ./...
```

当改动涉及以下区域时，额外关注：

1. `internal/ui/layout/`
   - 检查 frame 分配、split、remove、focus 是否仍符合预期
2. `internal/app/`
   - 检查 split / close / focus / mode / help bar 是否一致
3. `internal/ui/shell/`
   - 手工验证 shell 启动、退出、resize、scrollback、mouse

## 自动化测试分层

### 1. Unit Tests

适用于不依赖 PTY、Bubble Tea 事件循环或真实终端的纯逻辑。

当前已建立的重点：

- `internal/ui/layout/tree_test.go`

覆盖目标：

- `ComputeFrames`
- `LeafOrder`
- `SplitLeaf`
- `RemoveLeaf`
- `MoveFocus`

### 2. Behavior Tests

适用于 app 层状态流转，不追求渲染像素级断言。

建议后续补充：

- 初始 pane 树正确
- `splitFocused()` 创建新 shell pane
- `closeFocusedPane()` 不关闭固定 pane
- close 后 focus fallback 合理
- shell mode / normal mode 状态一致

### 3. Manual Regression

以下场景目前仍以手工回归为主：

1. shell pane 内运行交互式 TUI
2. alternate screen 切换
3. 终端 resize 后 redraw
4. scrollback 与 mouse reporting 切换
5. 多 shell pane 并存下的输入与焦点体验

## CI

仓库已增加最小 GitHub Actions 工作流：在 `push` 和 `pull_request` 时执行：

```bash
go test ./...
```

后续可逐步加入：

1. `go test -race ./...`
2. `golangci-lint`
3. 平台矩阵验证
