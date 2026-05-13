# Focus TUI - 探索总结

> 本文档记录对lazygit、sidecar、neovim等项目的深度源码分析
> 日期: 2026-04-10
> 状态: 当前有效

---

## 快速导航

- **总体方向**: [DIRECTION.md](./DIRECTION.md)
- **Phase 2.4计划**: [PLAN-2.4.md](./PLAN-2.4.md)
- **历史探索**: [LEARNINGS-2026-04-03.md](./LEARNINGS-2026-04-03.md)（dashboard/Todo/Pomodoro阶段）

---

## 本次探索范围（2026-04-10）

| 项目 | 路径 | 核心关注点 |
|------|------|-----------|
| **lazygit** | `/mnt/d/dev/dev-learn/lazygit` | TUI Git客户端架构、刷新管道、文件树 |
| **LunarVim** | `/mnt/d/dev/dev-learn/LunarVim` | UI布局模式、插件系统、命令发现 |
| **Neovim** | `/mnt/d/dev/dev-learn/neovim` | 窗口管理、事件系统、diff引擎 |
| **Glow** | `/mnt/d/dev/dev-learn/glow` | Bubble Tea TUI模式、双模式设计 |
| **Sidecar** | `/mnt/d/dev/dev-learn/sidecar` | Plugin+Adapter架构、工作台模式 |
| **Amux** | `/mnt/d/dev/dev-learn/amux` | 布局管理、面板分割 |

---

## 核心发现

### 1. Lazygit - 架构模式

**关键模式**: Context + Controller分离

```
Context:  负责渲染、选择、视图状态
Controller: 负责快捷键、动作、主面板渲染
```

**关键文件**:
- `pkg/gui/gui.go` - Gui对象组织
- `pkg/gui/controllers/helpers/refresh_helper.go` - 刷新管道
- `pkg/gui/filetree/file_tree_view_model.go` - 文件树视图模型

**核心洞察**:
- 扁平化可见列表，不递归渲染
- 选择状态在刷新间保持（包括重命名处理）
- UI线程 vs 工作线程分离

### 2. Sidecar - 工作台架构 ⭐ 最重要参考

**定位**: 不是终端复用器，而是**终端原生的AI开发者工作台**

**核心架构**:
```
App (Bubble Tea shell)
├── Plugin Layer (GitStatus, FileBrowser, Conversations)
├── Adapter Layer (ClaudeAdapter, AiderAdapter)
└── Tmux Execution Layer
```

**关键文件**:
- `internal/plugin/plugin.go` - Plugin契约
- `internal/adapter/adapter.go` - Adapter抽象
- `internal/modal/modal.go` - 可重用模态框架
- `internal/plugins/gitstatus/tree.go` - Git状态实现

**可重用模式**:
- 跨工作树状态聚合
- 骨架屏加载
- 集中主题系统

### 3. Git文件树算法（最佳实践）

**来自gitui、lf、yazi**:

1. **扁平化树** - 不递归渲染，保持flat array
2. **缓存视觉选择** - absolute→visual selection映射
3. **异步目录加载** - channel异步加载，元数据变化时才reload

### 4. Git解析最佳实践

**首选**: 结构化库/API（gix/git2）

**备选**: porcelain/raw + NUL分隔符
```bash
git status --porcelain=v2 -z
git diff-index ... -z
git log --pretty=raw
```

### 5. TUI框架选择

**Focus TUI选择**: **Bubble Tea**（Go）
- MVU模式适合状态化应用
- 子模型模式适合多面板
- Charm生态成熟

---

## 应用于Phase 2.4

### 架构决策

1. **Plugin + Adapter**（来自Sidecar）
2. **Context + Controller**（来自Lazygit）
3. **扁平化文件树**（来自gitui）
4. **异步Adapter刷新**（来自Sidecar）

### 关键代码参考

**Lazygit**:
```
pkg/gui/gui.go
pkg/gui/controllers/helpers/refresh_helper.go
pkg/gui/filetree/file_tree_view_model.go
```

**Sidecar**:
```
internal/plugin/plugin.go
internal/adapter/adapter.go
internal/plugins/gitstatus/tree.go
```

**Glow**:
```
ui/ui.go
ui/stash.go
ui/pager.go
```

---

## 详细探索记录

完整探索包含9个背景任务：
1. lazygit架构分析
2. LunarVim UI分析
3. Neovim底层分析
4. Glow TUI模式
5. Sidecar深度分析
6. 跨项目模式搜索
7. Neovim Git深入
8. 外部最佳实践研究
9. 终端IDE模式研究

---

## 结论

Phase 2.4将采用：
- **架构**: Plugin + Adapter（Sidecar模式）
- **框架**: Bubble Tea + Lip Gloss
- **Git解析**: porcelain=v2 + 异步刷新
- **文件树**: 扁平化列表 + 视图模型分离
- **布局**: 树形布局 + 配置驱动

详见 [PLAN-2.4.md](./PLAN-2.4.md)

---

## 历史文档

- [LEARNINGS-2026-04-03.md](./LEARNINGS-2026-04-03.md) - dashboard/Todo/Pomodoro阶段探索
