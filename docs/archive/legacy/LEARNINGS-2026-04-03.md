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



---

## 一、四大项目核心借鉴点

### 1. neofetch (Bash) - 信息展示与样式

**核心亮点：**
- **单文件架构**：11K 行 Bash 自包含，便于分发
- **6色角色系统**：title/at/underline/subtitle/colon/info 明确定义
- **统一输出包装**：`prin()` 函数统一处理所有输出，自动应用配色和缩进
- **进度条设计**：空格填充再替换字符，简洁高效
- **跨平台 case 处理**：使用 `case $os in` 优雅处理系统差异

**直接借鉴：**
```go
// 6色角色定义
type HeaderColors struct {
    Title     lipgloss.Color  // 用户名/主标题
    At        lipgloss.Color  // @ 符号/分隔符
    Hostname  lipgloss.Color  // 主机名/副标题
    Underline lipgloss.Color  // 下划线
    Subtitle  lipgloss.Color  // 小标题
    Colon     lipgloss.Color  // 冒号分隔符
    Info      lipgloss.Color  // 信息值
}

// 统一信息项组件
type InfoItem struct {
    Label     string
    Value     string
    Colors    InfoColors
    Separator string  // 默认 ": "
}
```

---

### 2. taskwarrior (C++) - 数据模型与状态机

**核心亮点：**
- **灵活数据模型**：键值对存储，便于扩展字段
- **虚拟标签系统**：OVERDUE、READY、BLOCKED 等通过计算得出，不冗余存储
- **多状态设计**：pending/completed/deleted/recurring/waiting
- **紧急度算法**：多因子评分（优先级 + 截止日期 + 阻塞状态 + 年龄）
- **UUID + 短 ID**：内部 UUID，界面短 ID (1,2,3...)

**直接借鉴：**
```go
// Todo 状态设计
type TodoStatus string
const (
    StatusTodo    TodoStatus = "todo"
    StatusDone    TodoStatus = "done"
    StatusOverdue TodoStatus = "overdue"
)

type TodoList string
const (
    ListToday   TodoList = "today"
    ListSomeday TodoList = "someday"
)

// 虚拟标签（计算属性）
func (t *Todo) IsOverdue() bool {
    return t.Status == StatusTodo && t.DueDate.Before(time.Now())
}

func (t *Todo) IsReady() bool {
    return t.Status == StatusTodo && 
           t.List == ListToday && 
           !t.IsBlocked()
}
```

---

### 3. glow (Go + Bubbletea) - 架构模式

**核心亮点：**
- **三层状态管理**：顶层 state + 子模型状态 + commonModel 共享
- **子模型设计**：每个面板独立 Model，通过顶层分发消息
- **消息系统**：异步命令通过 Channel + tea.Cmd 实现
- **分层快捷键**：全局绑定 + 上下文绑定 + guards 条件禁用
- **AdaptiveColor**：自动适应终端主题（light/dark）

**架构核心（直接复用）：**
```go
// 1. 顶层状态 - 页面切换
type appState int
const (
    stateDashboard appState = iota
    stateTaskList
    stateTaskDetail
)

// 2. 共享 commonModel
type commonModel struct {
    cfg    Config
    width  int
    height int
    theme  Theme
}

// 3. 主 Model 结构
type model struct {
    common *commonModel
    state  appState
    
    // 子模型
    taskList  tasklist.Model
    pomodoro  pomodoro.Model
    header    header.Model
    footer    footer.Model
}

// 4. 消息分发
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.state {
    case stateTaskList:
        newModel, cmd := m.taskList.Update(msg)
        m.taskList = newModel
        return m, cmd
    // ...
    }
}
```

---

### 4. lazygit (Go + gocui) - 复杂交互与模式系统

**核心亮点：**
- **Context 模式**：每个面板有独立 Context 管理状态和交互
- **模式系统**：Filtering/CherryPicking 等有统一接口（IsActive/Enter/Exit）
- **分层键盘配置**：Universal/Context-specific 分层定义
- **Helper 模式**：业务逻辑封装在 Helpers，Controllers 只处理输入
- **刷新系统**：细粒度刷新范围（Scope）+ 并发控制

**直接借鉴：**
```go
// 模式系统
type AppMode int
const (
    ModeNormal AppMode = iota  // 浏览模式
    ModeInput                  // 输入模式
)

type ModeStatus struct {
    IsActive    func() bool
    InfoLabel   func() string
    CancelLabel func() string
    Reset       func() error
}

// 键盘配置分层
type KeybindingConfig struct {
    Universal KeybindingUniversalConfig  // 全局：q, ctrl+c, ?
    TaskList  KeybindingTaskListConfig   // 任务列表
    Pomodoro  KeybindingPomodoroConfig   // 计时器
}
```

---

## 二、确定的技术方案

### 技术栈
- **语言**: Go 1.22+
- **TUI 框架**: Bubbletea (charm.sh)
- **样式**: Lipgloss + Bubbles (charm.sh)
- **数据库**: SQLite (modernc.org/sqlite，纯 Go，无 CGO)
- **配置**: YAML (gopkg.in/yaml.v3)
- **HTTP**: 标准库 net/http (天气 API)

### 关键设计决策

| 决策 | 方案 | 理由 |
|------|------|------|
| 架构模式 | 三层状态 (glow) | 清晰、可维护、子模型可独立测试 |
| 数据存储 | SQLite | 统计查询方便、事务支持、单文件 |
| 配色方案 | AdaptiveColor | 自动适应用户终端主题 |
| 模式系统 | Normal/Input 两种 | 简洁，覆盖所有场景 |
| 键盘映射 | 分层配置 | 可扩展，支持用户自定义 |
| 天气 API | Open-Meteo | 免费、无需 Key、比 wttr.in 稳定 |

### 数据库表结构

```sql
-- Todo 表
CREATE TABLE todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    short_id INTEGER,
    text TEXT NOT NULL,
    status TEXT CHECK(status IN ('todo', 'done', 'overdue')) DEFAULT 'todo',
    list TEXT CHECK(list IN ('today', 'someday')) DEFAULT 'today',
    priority TEXT CHECK(priority IN ('H', 'M', 'L')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    due_date DATETIME,
    completed_at DATETIME
);

-- 番茄会话表
CREATE TABLE pomodoro_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    date DATE NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_minutes INTEGER,
    linked_todo_uuid TEXT REFERENCES todos(uuid),
    status TEXT CHECK(status IN ('completed', 'cancelled'))
);

-- 连续使用记录表
CREATE TABLE streaks (
    date DATE PRIMARY KEY,
    has_pomodoro BOOLEAN DEFAULT 0
);

-- 配置表（用于缓存每日一句、天气等）
CREATE TABLE cache (
    key TEXT PRIMARY KEY,
    value TEXT,
    expires_at DATETIME
);
```

---

## 三、项目结构

```
focus-tui/
├── cmd/focus/
│   └── main.go                    # 程序入口
├── internal/
│   ├── app/
│   │   ├── app.go                 # 主 Model，消息路由
│   │   ├── state.go               # 状态定义
│   │   └── update.go              # Update 逻辑分发
│   ├── components/                # 可复用 UI 组件
│   │   ├── list.go                # 通用列表组件
│   │   ├── input.go               # 输入框组件
│   │   ├── helpbar.go             # 底部帮助栏
│   │   └── statusbar.go           # 状态栏
│   ├── models/                    # 领域模型
│   │   ├── todo.go                # Todo 模型
│   │   └── session.go             # Pomodoro 会话模型
│   ├── storage/                   # 数据层
│   │   ├── db.go                  # SQLite 初始化
│   │   ├── todo_store.go          # Todo CRUD
│   │   └── session_store.go       # 会话记录
│   ├── config/                    # 配置系统
│   │   └── config.go              # YAML 配置读取
│   ├── ui/                        # 子模型（页面）
│   │   ├── dashboard/             # 主仪表盘
│   │   │   └── dashboard.go
│   │   ├── tasklist/              # 任务列表
│   │   │   └── tasklist.go
│   │   ├── pomodoro/              # 计时器
│   │   │   └── pomodoro.go
│   │   ├── header/                # 顶栏
│   │   │   └── header.go
│   │   └── footer/                # 底栏
│   │       └── footer.go
│   ├── styles/                    # 主题系统
│   │   ├── theme.go               # 主题定义
│   │   └── colors.go              # 颜色常量
│   ├── weather/                   # 天气 API
│   │   └── openmeteo.go
│   └── quotes/                    # 每日一句
│       └── quotes.go
├── pkg/                           # 可复用公共库
│   └── utils/
│       └── utils.go
├── go.mod
├── go.sum
├── SPEC.md                        # 需求规格
└── LEARNINGS.md                   # 本文档
```

---

## 四、开发计划

### Phase 1: 基础框架
**目标**: 搭建项目结构，实现基础 TUI 框架

**任务:**
- [ ] 初始化 Go 模块，安装依赖 (bubbletea, lipgloss, sqlite)
- [ ] 创建项目目录结构
- [ ] 实现基础 App Model（状态管理、消息路由）
- [ ] 实现 commonModel（配置、尺寸传递）
- [ ] 实现基础主题系统（AdaptiveColor）
- [ ] 实现 Mode 系统（Normal/Input 切换）

**交付**: 能运行的空框架，显示欢迎界面，q/ctrl+c 退出

---

### Phase 2: Header 模块
**目标**: 顶栏（时间、天气、每日一句）

**任务:**
- [ ] 实现时间显示组件（实时刷新）
- [ ] 集成 Open-Meteo API（内置城市经纬度表）
- [ ] 实现天气显示（带缓存）
- [ ] 实现每日一句（内置语录库 + 自定义文件）
- [ ] Header 布局组合

**交付**: 显示完整的 Header，天气正常获取，语录每日固定

---

### Phase 3: 数据层与配置
**目标**: SQLite 数据库和配置系统

**任务:**
- [ ] 实现数据库初始化（自动建表）
- [ ] 实现 Todo Store（CRUD）
- [ ] 实现 Session Store
- [ ] 实现配置读取（YAML + 默认值）
- [ ] 实现数据目录创建（~/.local/share/focus/）

**交付**: 能读写数据库，配置可加载

---

### Phase 4: Todo 模块
**目标**: 完整的任务管理功能

**任务:**
- [ ] 实现 Todo List 组件（带光标、分页）
- [ ] 实现添加功能（输入模式）
- [ ] 实现删除功能（带确认）
- [ ] 实现编辑功能
- [ ] 实现状态切换（Space）
- [ ] 实现 Today/Someday 切换（Tab）
- [ ] 实现移动功能（m: Someday → Today）
- [ ] 实现虚拟标签（OVERDUE 自动标记）

**交付**: 完整的 Todo 管理，所有快捷键可用

---

### Phase 5: Pomodoro 模块
**目标**: 番茄钟计时器

**任务:**
- [ ] 实现计时器逻辑（工作/短休息/长休息）
- [ ] 实现计时显示（大字体）
- [ ] 实现开始/暂停/跳过/重置
- [ ] 实现 Todo 选择器（开始前的绑定）
- [ ] 实现系统通知（macOS/Linux）
- [ ] 实现会话记录（完成时写入数据库）

**交付**: 能完整跑一个番茄钟，通知正常，数据记录

---

### Phase 6: Footer 与统计
**目标**: 底栏数据显示

**任务:**
- [ ] 实现 streak 计算（连续天数）
- [ ] 实现今日番茄数统计
- [ ] 实现 Todo 完成率
- [ ] Footer 布局组合

**交付**: 底栏显示正确数据

---

### Phase 7: 整合与打磨
**目标**: 完整功能，发布准备

**任务:**
- [ ] 整合所有模块到主界面
- [ ] 实现快捷键帮助（? 键）
- [ ] 错误处理和降级（API 失败、数据库错误）
- [ ] UI 细节打磨（间距、颜色、对齐）
- [ ] 编写 README（安装、使用、配置）
- [ ] 编写 Makefile（build/install）

**交付**: v0.1.0 可用版本

---

## 五、关键代码模式备忘

### Bubbletea 基础模板

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"
)

type model struct {
    // 状态
}

func (m model) Init() tea.Cmd {
    return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    }
    return m, nil
}

func (m model) View() string {
    return "Hello, World!"
}

func main() {
    p := tea.NewProgram(model{}, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

### 子模型接口

```go
// 所有子模型实现此接口
type SubModel interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (SubModel, tea.Cmd)
    View() string
    SetSize(width, height int)
}
```

### 异步命令模式

```go
// 定义消息类型
type weatherMsg struct {
    temp int
    city string
    err  error
}

// 异步获取天气
func fetchWeather(city string) tea.Cmd {
    return func() tea.Msg {
        temp, err := weather.Get(city)
        return weatherMsg{temp: temp, city: city, err: err}
    }
}

// 在 Update 中处理
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case weatherMsg:
        if msg.err != nil {
            m.weather = "--"  // 静默降级
        } else {
            m.weather = fmt.Sprintf("%d°C", msg.temp)
        }
    }
    return m, nil
}
```

### AdaptiveColor 使用

```go
var (
    // 自动适应终端主题
    accent = lipgloss.AdaptiveColor{
        Light: "#7C3AED",
        Dark:  "#7C3AED",
    }
    
    // 创建样式
    titleStyle = lipgloss.NewStyle().
        Foreground(accent).
        Bold(true)
)
```

---

## 六、待决策事项

| 事项 | 选项 | 建议 |
|------|------|------|
| 列表组件 | 自研 vs bubbles/list | 先用自研，更灵活 |
| 输入框 | bubbles/textinput vs 自研 | 直接用 bubbles/textinput |
| 分页器 | bubbles/paginator vs 自研 | 直接用 bubbles/paginator |
| 帮助系统 | 动态生成 vs 静态 | 动态生成，根据当前模式显示 |
| 热重载配置 | 支持 vs v2 | v2 再支持 |
| 插件系统 | 支持 vs v2 | v2 再考虑 |

---

## 七、参考资源

### 项目源码
- `/mnt/d/dev/dev-learn/neofetch/` - 样式系统参考
- `/mnt/d/dev/dev-learn/taskwarrior/` - 数据模型参考
- `/mnt/d/dev/dev-learn/glow/` - 架构直接复用
- `/mnt/d/dev/dev-learn/lazygit/` - 复杂交互参考

### 文档
- [Bubbletea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Lipgloss Docs](https://github.com/charmbracelet/lipgloss)
- [Open-Meteo API](https://open-meteo.com/en/docs)

---

## 八、开发原则

1. **键盘优先**：所有操作必须有键盘快捷键
2. **本地优先**：数据只存本地 SQLite，不依赖云服务
3. **静默降级**：API 失败、配置错误时优雅降级，不中断使用
4. **可退出**：q/Ctrl+C 随时退出，不锁死用户
5. **零干扰**：信息密度合理，保持终端简洁气质

---

*准备就绪，开始 Phase 1 开发*
