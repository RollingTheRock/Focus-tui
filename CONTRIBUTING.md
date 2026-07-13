# Contributing to Focus

感谢你对 `focus` 感兴趣！本文件面向人类贡献者。

## 项目现状

`focus` 的**单机 MVP 已经由项目开发者完成**。当前版本已经能够在开发者的日常环境中稳定运行：

- 多 Git worktree 并行管理
- 多 Agent（Claude Code、Codex、Kimi CLI、OpenCode、Gemini CLI 等）的自动发现与会话启动
- Phase-driven DAG 编排
- 基于 MCP / A2A 的共享上下文协议
- 终端原生 TUI

然而，**受限于作者个人的设备与环境**，当前代码无法覆盖所有操作系统、终端模拟器、Shell、Agent 安装方式以及数据库后端组合。因此，我们非常欢迎你针对自己的环境做适配，并通过 Pull Request 把经验共享给社区。

## 当前最需要帮助的领域

如果你不确定从哪里开始，以下是优先级较高的方向：

| 方向 | 说明 |
|---|---|
| **跨平台适配** | Linux 不同发行版、macOS 不同版本、Windows（含 WSL）下的路径、PTY、信号处理差异 |
| **终端模拟器适配** | 外部终端启动（`external_terminal`）对 kitty、alacritty、wezterm、ghostty、iTerm2、Windows Terminal 等的支持 |
| **Agent 二进制发现** | 各平台 / 各安装方式（Homebrew、pipx、cargo、手动安装、Windows `.exe` 等）下 Agent 的查找与调用 |
| **数据库后端** | SQLite 是默认；对 PostgreSQL 后端在生产环境、容器环境、CI 环境中的验证与优化 |
| **构建与发布** | 更完善的 release matrix、包管理器分发（Homebrew、AUR、Scoop 等）、codesign / notarization |
| **文档与翻译** | 安装指南、架构文档、ADR 的英文完善与其他语言翻译 |
| **测试覆盖** | 在你能访问的平台上运行测试并修复失败用例 |

## 如何贡献

1. **Fork 仓库** 并从 `master` 切出特性分支。
2. **先读文档**：`docs/architecture/` 和 `docs/adr/` 记录了设计决策，`docs/releases/` 记录了发布流程。
3. **配置环境**：参考 `CONTRIBUTING.agents.md` 中的前置检查清单，确认你的 Agent、终端、MCP 等已就绪。
4. **做改动**，保持改动最小且聚焦。
5. **本地验证**：
   ```bash
   go build ./cmd/focus
   go test -short ./...
   ```
6. **提交 PR**：目标分支为 `master`。
   - PR 标题建议使用 conventional commits 风格，例如 `fix(adapter): handle Windows agent path with spaces`。
   - 描述里说明你的环境（OS、Shell、终端、Agent 版本）以及改动的动机。
7. **等待 review**：当前 `master` 分支要求至少 1 个 approving review 且 `test` status check 通过。

## 开发环境要求

- Go 1.25+
- 一个可用的 Git 仓库（用于实际体验 worktree 功能）
- 至少安装了一种支持的 Agent 二进制（用于测试 Agent 发现与会话启动）
- Unix-like 环境体验最佳；Windows 贡献者请优先确保 WSL 路径能跑通

## 提交规范

- 使用 [Conventional Commits](https://www.conventionalcommits.org/) 风格。
- 常见类型：`feat`、`fix`、`docs`、`refactor`、`test`、`ci`。
- 提交信息使用英文，保持简洁。

## 沟通方式

- 有问题先查 [Issues](https://github.com/RollingTheRock/Focus-tui/issues)。
- 如果是针对你所在环境的适配问题，开 Issue 时请附上：
  - 操作系统及版本
  - Shell 及版本
  - 终端模拟器
  - 安装的 Agent 及版本
  - 复现步骤与完整错误日志
- 中文或英文 Issue 均可接受。

## 行为准则

- 尊重不同环境、不同工作流的贡献者。
- 保持讨论围绕技术与用户体验。
- 对新手友好，review 意见请说明阻塞与非阻塞。

## 许可证

通过提交 PR，你同意你的贡献将在 [Apache-2.0](LICENSE) 许可证下发布。
