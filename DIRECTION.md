# Focus TUI - 总体技术方向与架构规划

> 文档版本: 2026-04-10
> 状态: 取代 DIRECTION.md 和 NEXT_STEPS.md 成为唯一方向文档

---

## 一、产品定位

**Focus TUI 是一个运行在终端里的开发者操作系统。**

它不是TUI工具，不是IDE替代品，也不是其他工具的启动器。它是原生集成shell、git工作流、agent会话的终端原生工作空间。

### 核心差异

| 工具 | 局限 | Focus TUI 的解决 |
|------|------|-----------------|
| 传统终端 | 只负责渲染，不理解用户操作 | 深度理解shell状态、git状态、agent状态 |
| IDE | 困于GUI交互范式，agent是插件 | 终端原生，agent作为一等公民 |
| lazygit | 懂git，但不懂shell上下文 | 同一进程内，shell+git+agent互相可见 |
| sidecar | 旁观者，只能读，无法编排 | 完全掌控shell，可编排多agent并行 |

### 用户场景

终端开发者直接操控worktree、直接编排并行agent，不被GUI产品交互范式限制。Focus TUI让"操控agent"这件事有结构，而不是在tmux里开一堆窗口自己心算状态。

---

## 二、分层架构（已验证）

基于对lazygit、sidecar、neovim等项目的深度源码分析，我们采用**四层渐进架构**：

```
┌─────────────────────────────────────────────────────────┐
│ Layer 4: 应用与编排层 (App Orchestrator)                 │
│ - Pane生命周期管理（注册、注销、布局）                     │
│ - 消息路由（定向+广播）                                   │
│ - Focus管理                                              │
│ - 全局状态协调                                           │
├─────────────────────────────────────────────────────────┤
│ Layer 3: Pane类型层 (Pane Types)                         │
│ ┌─────────────────────────────────────────────────┐    │
│ │ CorePanes (硬编码)                               │    │
│ │ - shell: 多实例PTY会话                           │    │
│ │ - todo: 任务管理（保留）                          │    │
│ │ - pomodoro: 专注计时（保留）                      │    │
│ │ - header/footer: 系统UI                          │    │
│ └─────────────────────────────────────────────────┘    │
│ ┌─────────────────────────────────────────────────┐    │
│ │ PluginPanes (动态)                               │    │
│ │ - git-status: 工作区状态                          │    │
│ │ - file-tree: 文件树                               │    │
│ │ - diff-view: 差异查看                             │    │
│ │ - log-view: 提交历史                              │    │
│ │ - agent-history: agent会话历史                    │    │
│ └─────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────┤
│ Layer 2: 插件系统 (Plugin System) ⭐ 核心扩展机制        │
│ - Plugin接口定义                                        │
│ - PluginRegistry (类型→工厂映射)                        │
│ - 生命周期管理 (Init/Update/Destroy)                    │
│ - 与Layout Tree集成                                     │
├─────────────────────────────────────────────────────────┤
│ Layer 1: 适配器层 (Adapter Layer) ⭐ 数据基础设施        │
│ - GitAdapter: git操作封装                               │
│ - GitHubAdapter: gh CLI集成                             │
│ - AgentAdapter: agent会话监听                           │
│ - FileSystemAdapter: 文件系统监听                       │
│ - 异步数据获取 + TTL缓存 + 后台刷新                      │
└─────────────────────────────────────────────────────────┘
```

### 架构设计原则

1. **渐进式演进，非推倒重来**
   - 保留现有CorePanes和Tree Layout系统
   - 通过Plugin机制逐步引入新Pane类型
   - 现有测试和稳定行为保持不变

2. **原生优于集成**
   - git/worktree/agent工作流必须原生实现
   - 不是pane里开别人的工具，而是把能力内化为组件
   - 借鉴开源项目源码，但目标是内化能力

3. **Adapter解耦**
   - Adapter只负责数据获取，不操作UI
   - Pane通过消息机制接收Adapter更新
   - 同一Adapter可服务多个Pane类型

4. **配置驱动布局**
   - bodyTree支持配置化定义
   - CorePane和PluginPane在布局中平等对待
   - 支持混合布局（左侧shell，右侧git+filetree）

---

## 三、技术栈与参考实现

### 核心技术栈

| 组件 | 选择 | 理由 |
|------|------|------|
| TUI框架 | Bubble Tea | MVU模式，生态成熟，与Charm套件配合 |
| 样式 | Lip Gloss | 声明式样式，主题系统 |
| 渲染 | Glamour | Markdown渲染（用于help、doc） |
| 终端仿真 | charmbracelet/x/vt | SafeEmulator，支持alternate screen |
| 数据存储 | SQLite | 本地优先，零配置 |
| 配置 | YAML/Viper | 用户友好，支持热重载 |

### 关键参考项目

基于对以下项目的深度源码分析（详见LEARNINGS.md）：

| 项目 | 借鉴重点 | 关键文件参考 |
|------|---------|-------------|
| **lazygit** | Context+Controller架构、刷新管道、文件树 | `pkg/gui/gui.go`, `pkg/gui/controllers/helpers/refresh_helper.go`, `pkg/gui/filetree/file_tree_view_model.go` |
| **sidecar** | Plugin+Adapter架构、跨工作树聚合、模态系统 | `internal/plugin/plugin.go`, `internal/adapter/adapter.go`, `internal/modal/modal.go`, `internal/plugins/gitstatus/tree.go` |
| **LunarVim** | UI布局模式、事件驱动加载、命令发现 | `lua/lvim/core/nvimtree.lua`, `lua/lvim/core/which-key.lua` |
| **Glow** | Bubble Tea双模式、子模型切换 | `ui/ui.go`, `ui/stash.go`, `ui/pager.go` |
| **gitui** | 文件树算法、扁平化渲染、异步加载 | `src/filetreelist.rs` |

---

## 四、Phase规划（修订版）

### Phase 1: 嵌入式Shell ✅ 已完成

**目标**: Claude Code在内置shell里跑起来和原生终端一样流畅。

**已完成**:
- PTY + `charmbracelet/x/vt` SafeEmulator
- key/mouse forwarding
- alternate screen处理
- 窗口大小同步

### Phase 2: Multi-Pane Layout 🔄 进行中

**目标**: 构建支持多Pane并排工作的布局基础设施。

**已完成 (Phase 2.1-2.3)**:
- ✅ Tree-based layout engine (split/leaf)
- ✅ Pane注册表和生命周期
- ✅ Focus路由（方向键、Tab切换）
- ✅ Shell pane多实例（创建/分割/关闭）
- ✅ Split ratio调整（Ctrl+方向键）
- ✅ 关闭pane后focus fallback
- ✅ Pane标题/状态展示
- ✅ 极窄布局下降级规则

**当前**: Phase 2.4 - Git集成准备

### Phase 3: 原生Git工作流 📋 计划中

**目标**: 不离开focus-tui完成日常95%的git操作。

**范围**:
- worktree状态面板（列表、branch、dirty状态）
- 当前repo状态（branch、ahead/behind、staged/unstaged）
- staged/unstaged文件列表
- hunk级diff查看
- 交互式staging
- commit message编辑

**关键**: worktree面板必须与agent会话状态联动。

### Phase 4: Agent会话可见性 📋 计划中

**目标**: 结构化展示agent活动，不绑定单一vendor。

**数据来源**（参考sidecar）:
- Claude Code: `~/.claude/projects/<slug>/*.jsonl`
- OpenCode: 类似结构化日志
- 其他agent: Adapter接口扩展

**功能**:
- Session历史面板
- 通过CWD+Branch绑定到worktree
- 多agent并行状态展示

### Phase 5: 高级功能 📋 远期

- 周报统计视图
- 插件市场
- 数据同步到git repo
- 导出Markdown日报

---

## 五、开发原则

1. **不绑定单一agent**
   - 所有面向agent的设计保持agent-agnostic
   - 通过Adapter接口支持多agent

2. **保留有用工具**
   - Todo/Pomodoro不删除，作为原生工作台组件保留
   - 个人工作流组件与开发工作流共存

3. **最小正确改动**
   - 不做大规模无关重构
   - 每个改动有明确测试覆盖
   - `go test ./...`是每次提交前的固定动作

4. **测试分层**
   - 单元测试: 优先覆盖layout/tree.go等纯逻辑
   - 行为测试: 覆盖app.go的split/close/focus路由
   - 手工回归: shell/PTY/resize/mouse等复杂交互

---

## 六、下一步行动

详见 [PLAN-2.4.md](./PLAN-2.4.md) - Phase 2.4详细开发计划。

---

## 附录：文档关系

- **本文件 (DIRECTION.md)**: 唯一方向文档，取代旧DIRECTION.md和NEXT_STEPS.md
- **PLAN-2.4.md**: Phase 2.4详细任务分解
- **LEARNINGS.md**: 源码探索总结（lazygit、sidecar等分析）
- **SPEC.md**: 已归档，仅供历史参考
- **README.md**: 项目介绍和快速开始
