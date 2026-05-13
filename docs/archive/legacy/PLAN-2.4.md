# Phase 2.4 开发计划: Git/Tree集成准备

> [过时归档说明]
> 自 2026-04-19 起，本文档已被 ADR-first 方向 supersede。
> 保留仅作历史阶段计划参考，不再作为当前实现依据。

> 文档版本: 2026-04-10
> 预计周期: 4周
> 目标: 完成Plugin+Adapter架构基础设施，实现首个Git Plugin Pane

---

## 一、Phase 2.4 总体目标

基于对lazygit、sidecar、neovim等项目的源码探索，建立Plugin+Adapter扩展架构，实现首个PluginPane（GitStatus），并确保与现有CorePanes（shell/todo/pomodoro）无缝共存。

### 成功标准

- [ ] Plugin系统可动态注册新的Pane类型
- [ ] GitAdapter能稳定解析git状态（porcelain=v2）
- [ ] GitStatusPane作为首个PluginPane正常工作
- [ ] 现有CorePanes（shell/todo/pomodoro）不受影响
- [ ] 所有测试通过 `go test ./...`
- [ ] 支持配置化布局（混合CorePane和PluginPane）

---

## 二、架构设计详情

### 2.1 Plugin系统架构

```go
// internal/plugins/plugin.go
package plugins

type Plugin interface {
    // 元数据
    Name() string
    Version() string
    
    // 声明支持的Pane类型
    PaneTypes() []models.PaneType
    
    // 工厂方法：创建指定类型的Pane实例
    CreatePane(
        paneType models.PaneType,
        id models.PaneID,
        meta models.PaneMeta,
        common models.CommonModel,
    ) (models.Panel, error)
    
    // 生命周期
    Init() error
    Destroy() error
}

type Registry struct {
    plugins map[string]Plugin          // name -> plugin
    paneTypes map[models.PaneType]Plugin  // paneType -> plugin
}

func (r *Registry) Register(p Plugin) error
func (r *Registry) CreatePane(
    paneType models.PaneType,
    ...
) (models.Panel, error)
```

### 2.2 Adapter层架构

```go
// internal/adapters/adapter.go
package adapters

// 基础Adapter接口
type Adapter interface {
    Name() string
    Init() error
    Destroy() error
}

// GitAdapter接口
type GitAdapter interface {
    Adapter
    GetStatus(repoPath string) (*git.Status, error)
    GetBranches(repoPath string) ([]git.Branch, error)
    GetDiff(repoPath string, path string, staged bool) (string, error)
    WatchStatus(repoPath string) (<-chan StatusEvent, error)
}

// AdapterManager管理所有Adapter生命周期
type Manager struct {
    adapters map[string]Adapter
    git GitAdapter
}
```

### 2.3 与现有架构的集成点

**1. App初始化流程增强**

```go
// internal/app/app.go
func New(cfg Config, store store.Store) *model {
    // ... 现有初始化 ...
    
    // 1. 创建PluginRegistry
    m.pluginRegistry = plugins.NewRegistry()
    
    // 2. 加载内置Plugins
    m.loadBuiltinPlugins()
    
    // 3. 创建AdapterManager
    m.adapterManager = adapters.NewManager(cfg.Adapters)
    
    // 4. 根据配置创建PluginPanes
    m.createPluginPanes(cfg.Layout.PluginPanes)
    
    return m
}
```

**2. bodyTree支持混合布局**

```yaml
# config.yaml 示例
layout:
  body:
    type: split
    direction: horizontal
    ratio: 60
    left:
      type: pane
      id: shell-main          # CorePane
      pane_type: shell
    right:
      type: split
      direction: vertical
      ratio: 50
      top:
        type: pane
        id: git-status        # PluginPane
        pane_type: git-status
        plugin: git
        adapter: git-local
      bottom:
        type: pane
        id: file-tree         # PluginPane
        pane_type: file-tree
        plugin: filebrowser
        adapter: fs
```

---

## 三、任务分解

### Week 1: 基础设施（第1-7天）

#### Day 1-2: Plugin系统骨架

**任务 1.1**: 创建Plugin接口和Registry
- **文件**: `internal/plugins/plugin.go`, `internal/plugins/registry.go`
- **要求**:
  - 定义Plugin接口（Name, PaneTypes, CreatePane, Init, Destroy）
  - 实现Registry（Register, CreatePane, GetPluginForType）
  - 添加错误处理（重复注册、未知PaneType等）
- **测试**: `internal/plugins/registry_test.go`
  - 测试Register和重复注册检测
  - 测试CreatePane工厂方法

**任务 1.2**: 扩展PaneType枚举
- **文件**: `internal/models/pane.go`
- **修改**:
  ```go
  const (
      // Core panes (保留)
      PaneTypeHeader   PaneType = "header"
      PaneTypeShell    PaneType = "shell"
      PaneTypeTodo     PaneType = "todo"
      PaneTypePomodoro PaneType = "pomodoro"
      PaneTypeFooter   PaneType = "footer"
      
      // Plugin panes (新增)
      PaneTypeGitStatus PaneType = "git-status"
      PaneTypeFileTree  PaneType = "file-tree"
      PaneTypeDiffView  PaneType = "diff-view"
  )
  ```

#### Day 3-4: Adapter系统骨架

**任务 1.3**: 创建Adapter接口和Manager
- **文件**: `internal/adapters/adapter.go`, `internal/adapters/manager.go`
- **要求**:
  - 定义Adapter基础接口
  - 定义GitAdapter接口（参考Sidecar实现）
  - 实现Manager（生命周期管理、依赖注入）

**任务 1.4**: 实现GitAdapter（基础版）
- **文件**: `internal/adapters/git.go`, `internal/adapters/git_local.go`
- **要求**:
  - 使用`git status --porcelain=v2 -z`解析
  - 支持Status、Branch基础查询
  - 错误处理（非git目录、git未安装等）
- **参考**: `/mnt/d/dev/dev-learn/sidecar/internal/plugins/gitstatus/tree.go`

#### Day 5-7: App集成

**任务 1.5**: 修改App初始化流程
- **文件**: `internal/app/app.go`
- **要求**:
  - 在New()中初始化PluginRegistry和AdapterManager
  - 实现loadBuiltinPlugins()
  - 实现createPluginPanes()
  - 保持现有CorePanes初始化不变

**任务 1.6**: 配置系统支持Plugin
- **文件**: `internal/config/config.go`
- **要求**:
  - 扩展Config结构支持plugins和adapters配置
  - 支持layout配置中的plugin pane定义

---

### Week 2: GitStatus Plugin（第8-14天）

#### Day 8-10: GitStatusPane实现

**任务 2.1**: 实现GitStatusPlugin
- **文件**: `internal/plugins/git/plugin.go`
- **要求**:
  - 实现Plugin接口
  - 声明支持的PaneTypes（git-status）
  - 实现CreatePane工厂方法

**任务 2.2**: 实现GitStatusPane（Panel接口）
- **文件**: `internal/plugins/git/status_pane.go`
- **功能**:
  - 显示当前分支、ahead/behind
  - staged/unstaged文件列表
  - 状态图标（参考Lazygit）
  - 键盘导航（↑/↓选择）
- **消息处理**:
  - 接收StatusChangedMsg更新状态
  - 发送StatsRefreshMsg刷新footer

**任务 2.3**: 样式和渲染
- **文件**: `internal/plugins/git/styles.go`
- **要求**:
  - 使用Lip Gloss定义样式
  - 支持主题色
  - 状态图标映射（M/A/D/R/?）

#### Day 11-12: 状态同步机制

**任务 2.4**: Adapter异步刷新
- **文件**: `internal/adapters/git_local.go`
- **要求**:
  - 实现WatchStatus()返回channel
  - 后台goroutine定期刷新（2秒间隔）
  - TTL缓存避免重复请求

**任务 2.5**: App层消息路由
- **文件**: `internal/app/app.go`
- **要求**:
  - 在Update中处理StatusChangedMsg
  - 将Adapter消息路由到对应Pane
  - 保持现有消息路由不变

#### Day 13-14: 集成测试

**任务 2.6**: 单元测试
- **文件**: `internal/plugins/git/*_test.go`
- **覆盖**:
  - GitStatusPane Update/View
  - 状态解析正确性
  - 键盘事件处理

**任务 2.7**: 集成验证
- **验证**:
  - GitStatusPane能在bodyTree中正确渲染
  - 与shell pane并排工作
  - 焦点切换正常
  - 关闭/重建pane正常

---

### Week 3: 布局配置化（第15-21天）

#### Day 15-17: 配置驱动布局

**任务 3.1**: 布局配置解析
- **文件**: `internal/config/layout.go`
- **要求**:
  - 支持从YAML解析bodyTree配置
  - 支持混合CorePane和PluginPane
  - 验证配置（pane_type必须已注册）

**任务 3.2**: 动态构建bodyTree
- **文件**: `internal/app/layout_builder.go`
- **要求**:
  - 根据配置动态构建TreeNode
  - 创建对应的Pane实例
  - 错误处理（配置错误优雅降级）

**任务 3.3**: 默认配置
- **文件**: `internal/config/defaults.go`
- **要求**:
  - 提供向后兼容的默认布局
  - 默认只有CorePanes（与现有行为一致）
  - 示例配置展示GitStatusPane用法

#### Day 18-19: 消息广播机制

**任务 3.4**: StatsRefreshMsg广播
- **文件**: `internal/app/msg_router.go`
- **要求**:
  - PluginPane能发送StatsRefreshMsg
  - App广播到所有关心此消息的Pane
  - Footer Pane响应并刷新统计

**任务 3.5**: Pane间通信
- **示例**: GitStatusPane选择文件 → DiffPane显示diff
- **要求**:
  - 定义FileSelectedMsg
  - App路由到对应Pane
  - 松耦合设计

#### Day 20-21: 测试和文档

**任务 3.6**: 行为测试
- **文件**: `internal/app/*_test.go`
- **覆盖**:
  - PluginPane注册和创建
  - 配置解析和布局构建
  - 消息路由正确性

**任务 3.7**: 开发者文档
- **文件**: `docs/plugin-development.md`
- **内容**:
  - 如何创建新Plugin
  - Plugin接口详解
  - 示例代码

---

### Week 4: 打磨和扩展（第22-28天）

#### Day 22-24: 用户体验打磨

**任务 4.1**: 骨架屏加载
- **文件**: `internal/ui/skeleton.go`（参考Sidecar）
- **要求**:
  - PluginPane异步加载时显示骨架屏
  - 适配不同尺寸

**任务 4.2**: 错误处理
- **要求**:
  - GitAdapter错误友好显示（非git目录、网络错误等）
  - Plugin加载失败不影响其他Pane
  - 优雅降级（显示错误信息而非崩溃）

**任务 4.3**: 性能优化
- **任务**:
  - Git状态刷新节流（避免快速目录切换时频繁请求）
  - 大仓库优化（分页、虚拟化）

#### Day 25-26: 准备工作（Phase 3）

**任务 4.4**: DiffPane容器准备
- **文件**: `internal/plugins/git/diff_pane.go`（骨架）
- **要求**:
  - 实现基础Panel接口
  - 预留diff渲染接口
  - 与GitStatusPane联动

**任务 4.5**: 快捷键系统增强
- **文件**: `internal/app/keybindings.go`
- **要求**:
  - PluginPane能注册自己的快捷键
  - 上下文感知（不同Pane不同快捷键）
  - 帮助面板动态生成

#### Day 27-28: 最终验证

**任务 4.6**: 端到端测试
- **场景**:
  1. 启动focus-tui（配置包含GitStatusPane）
  2. 在shell pane中执行git操作
  3. 观察GitStatusPane实时更新
  4. 切换focus，操作不同pane
  5. 关闭/重建GitStatusPane

**任务 4.7**: 代码审查和合并准备
- **检查清单**:
  - [ ] 所有测试通过
  - [ ] 代码注释完整
  - [ ] 无循环依赖
  - [ ] 向后兼容（无配置时行为不变）
  - [ ] 文档更新

---

## 四、文件结构（新增）

```
focus-tui/
├── internal/
│   ├── plugins/                    # ⭐ 新增：Plugin系统
│   │   ├── plugin.go               # Plugin接口
│   │   ├── registry.go             # Plugin注册表
│   │   ├── registry_test.go        # 注册表测试
│   │   └── git/                    # Git Plugin
│   │       ├── plugin.go           # GitPlugin实现
│   │       ├── status_pane.go      # GitStatusPane
│   │       ├── status_pane_test.go
│   │       ├── styles.go           # 样式定义
│   │       └── diff_pane.go        # DiffPane（Week 4）
│   ├── adapters/                   # ⭐ 新增：Adapter层
│   │   ├── adapter.go              # Adapter接口
│   │   ├── manager.go              # AdapterManager
│   │   ├── git.go                  # GitAdapter接口
│   │   └── git_local.go            # GitLocalAdapter实现
│   ├── app/
│   │   ├── app.go                  # 修改：集成Plugin/Adapter
│   │   ├── layout_builder.go       # ⭐ 新增：配置驱动布局构建
│   │   ├── msg_router.go           # ⭐ 新增：消息路由增强
│   │   └── keybindings.go          # ⭐ 新增：快捷键系统（Week 4）
│   ├── ui/
│   │   └── skeleton.go             # ⭐ 新增：骨架屏（Week 4）
│   └── config/
│       ├── config.go               # 修改：支持Plugin配置
│       ├── layout.go               # ⭐ 新增：布局配置
│       └── defaults.go             # ⭐ 新增：默认配置
├── docs/
│   └── plugin-development.md       # ⭐ 新增：开发者文档（Week 3）
└── PLAN-2.4.md                     # 本文件
```

---

## 五、测试策略

### 5.1 单元测试

| 模块 | 测试文件 | 覆盖率目标 |
|------|---------|-----------|
| Plugin Registry | `plugins/registry_test.go` | 90% |
| GitAdapter | `adapters/git_local_test.go` | 80% |
| GitStatusPane | `plugins/git/status_pane_test.go` | 75% |
| Layout Builder | `app/layout_builder_test.go` | 80% |

### 5.2 集成测试

- **场景1**: Plugin加载 → Pane创建 → 渲染 → 销毁
- **场景2**: GitAdapter刷新 → 消息路由 → Pane更新
- **场景3**: 混合布局（CorePane + PluginPane）
- **场景4**: 配置解析 → 布局构建 → 运行验证

### 5.3 手工回归清单

- [ ] 启动时间无明显变慢
- [ ] 现有CorePanes功能完整
- [ ] GitStatusPane能正确显示git状态
- [ ] 焦点切换流畅
- [ ] 窗口resize后布局正确
- [ ] 无配置时向后兼容（默认只有CorePanes）

---

## 六、风险与缓解

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|---------|
| Plugin系统与现有架构冲突 | 中 | 高 | 渐进式集成，保持现有测试通过 |
| GitAdapter性能问题（大仓库） | 中 | 中 | 异步刷新、TTL缓存、分页 |
| 配置格式不兼容 | 低 | 高 | 默认配置向后兼容，新版本配置独立 |
| 内存泄漏（后台goroutine） | 低 | 高 | 完善的Destroy生命周期，pprof监控 |

---

## 七、参考资源

### 关键源码（本地）

```
/mnt/d/dev/dev-learn/lazygit/pkg/gui/gui.go
/mnt/d/dev/dev-learn/lazygit/pkg/gui/controllers/helpers/refresh_helper.go
/mnt/d/dev/dev-learn/lazygit/pkg/gui/filetree/file_tree_view_model.go
/mnt/d/dev/dev-learn/sidecar/internal/plugin/plugin.go
/mnt/d/dev/dev-learn/sidecar/internal/adapter/adapter.go
/mnt/d/dev/dev-learn/sidecar/internal/plugins/gitstatus/tree.go
/mnt/d/dev/dev-learn/sidecar/internal/modal/modal.go
```

### 设计文档

- `DIRECTION.md` - 总体技术方向
- `LEARNINGS.md` - 源码探索总结

---

## 八、完成定义（Definition of Done）

Phase 2.4 完成时，必须满足：

- [x] **功能完整**
  - [ ] GitStatusPane能正确显示git工作区状态
  - [ ] 支持分支、ahead/behind、staged/unstaged文件
  - [ ] 键盘导航和选择正常
  - [ ] 状态变化实时更新

- [x] **架构达标**
  - [ ] Plugin系统能注册和创建Pane
  - [ ] Adapter层能异步获取git数据
  - [ ] 配置驱动布局正常工作
  - [ ] CorePanes和PluginPane能混合布局

- [x] **质量达标**
  - [ ] 单元测试覆盖率≥75%（新增代码）
  - [ ] 集成测试覆盖主要场景
  - [ ] 手工回归清单全部通过
  - [ ] 无内存泄漏（pprof验证）

- [x] **文档完整**
  - [ ] 开发者文档（如何创建Plugin）
  - [ ] 配置示例
  - [ ] 代码注释和文档字符串

- [x] **向后兼容**
  - [ ] 无配置时行为与Phase 2.3一致
  - [ ] 现有快捷键和交互不变
  - [ ] 数据格式兼容

---

**下一步**: 开始Week 1任务，创建Plugin系统骨架。
