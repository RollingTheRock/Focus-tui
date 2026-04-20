# Focus TUI - 需求规格文档

> [归档说明]
> 本文档对应项目早期的“生产力 TUI / Todo + Pomodoro 工作台”阶段。
> 自 2026-04-07 起，项目主方向已经切换为“终端里的开发者操作系统 / terminal-native developer workspace”。
> 本文档不再作为当前开发计划依据，保留仅供历史参考。

## 项目概览

**项目名：** `focus`

**定位：** 一个运行在终端的全屏 TUI 工作台，集成每日仪表盘、Todo 管理、Pomodoro 专注计时，面向开发者设计，开箱即用，可选作默认 Shell 启动。

**技术栈：** Go + Bubbletea + Lipgloss + SQLite

**目标平台：** macOS / Linux

---

## 核心设计原则

- **键盘优先**：所有操作均可通过键盘完成，不依赖鼠标
- **本地优先**：数据存本地 SQLite，不依赖任何云服务
- **零干扰**：信息密度合理，不堆砌功能，保持终端的简洁气质
- **可退出**：随时按 `q` 或 `Ctrl+C` 退出回到正常 shell，不锁死用户

---

## 界面布局

```
┌─────────────────────────────────────────────────┐
│  🌤 26°C · Shanghai   Fri Apr 3 · 21:17         │
│  "Do one thing every day that scares you."      │
├────────────────────────┬────────────────────────┤
│  TODAY                 │  FOCUS                 │
│                        │                        │
│  ○ 拆 KV Store         │     🍅 24:37           │
│  ○ 写 README           │                        │
│  ● 推 formatter        │   Working: KV Store    │
│  ✓ 复现 B-Tree 论文    │                        │
│                        │  [S] Start  [P] Pause  │
│  [a]dd [d]el [space]✓  │  [N] Next   [R] Reset  │
├────────────────────────┴────────────────────────┤
│  🔥 streak 7d  |  🍅 today 3  |  ✓ done 1/4    │
└─────────────────────────────────────────────────┘
```

---

## 数据存储

**SQLite 数据库路径：** `~/.local/share/focus/focus.db`

### 表结构

```sql
-- Todo 表
CREATE TABLE todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    text TEXT NOT NULL,
    status TEXT CHECK(status IN ('todo', 'done', 'overdue')) DEFAULT 'todo',
    list TEXT CHECK(list IN ('today', 'someday')) DEFAULT 'today',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 番茄会话表
CREATE TABLE pomodoro_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    linked_todo_id INTEGER REFERENCES todos(id),
    status TEXT CHECK(status IN ('completed', 'cancelled'))
);

-- 连续使用记录表
CREATE TABLE streaks (
    date DATE PRIMARY KEY,
    has_pomodoro BOOLEAN DEFAULT 0
);
```

---

## 模块详细需求

### 模块 1：顶栏 Header

**天气**
- 调用 **Open-Meteo** API，无需 Key，稳定性优于 wttr.in
- 内置常见城市经纬度映射表（前100大城市）
- 显示：城市名、温度、天气图标
- 城市在配置文件中设定，默认读系统时区推断
- 请求失败时静默降级，不显示天气区域

**时间与日期**
- 实时刷新，精确到分钟
- 格式：`Mon Jan 2 · 15:04`，可在配置中自定义

**每日一句**
- 默认从内置语录库随机读取（英文为主，开发者/创作者风格）
- 支持用户在配置文件中指定自定义语录文件路径
- 每日固定一句（同一天内不变），次日自动换

---

### 模块 2：Todo 管理

**数据结构**
- 每条 Todo：`id`, `text`, `status` (todo/done/overdue), `list` (today/someday), `created_at`, `updated_at`
- 存储：SQLite `~/.local/share/focus/focus.db`

**交互操作（浏览模式）**

| 按键 | 操作 |
|------|------|
| `a` | 进入输入模式，添加新 Todo |
| `d` | 删除选中 Todo（需确认） |
| `Space` | 切换完成状态 |
| `e` | 进入输入模式，编辑选中 Todo |
| `Tab` | 在 Today / Someday 列表间切换 |
| `↑/↓` | 上下移动选中项 |
| `k/j` | Vim 风格上下移动 |
| `m` | 将 Someday 任务移入 Today |

**显示规则**
- 已完成的 Todo 置底显示，带删除线样式
- Today 列表只显示当天创建或手动移入的任务
- 每日 00:00 自动将未完成的 Today 任务标记为 overdue（保留显示，颜色变灰）

---

### 模块 3：Pomodoro 专注计时

**基本流程**
- 标准番茄：25 分钟工作 → 5 分钟休息（短休息）
- 每 4 个番茄后：15 分钟长休息
- 时长均可在配置文件中修改

**交互操作（浏览模式）**

| 按键 | 操作 |
|------|------|
| `s` | 开始计时（弹出 Todo 选择器，Esc 跳过） |
| `p` | 暂停 / 继续 |
| `n` | 跳过当前阶段（进入下一个） |
| `r` | 重置当前番茄 |

**关联 Todo**
- 开始计时前，可选择绑定一个 Today 的 Todo
- 计时区显示当前绑定的任务名
- 按 `Esc` 跳过选择直接开始

**完成通知**
- 番茄结束时发送系统通知（macOS 用 `osascript`，Linux 用 `notify-send`）
- 可在配置中关闭

**数据记录**
- 每完成一个番茄，写入 `pomodoro_sessions` 表
- 字段：`date`, `start_time`, `end_time`, `linked_todo_id`, `status`

---

### 模块 4：底栏 Footer

实时显示三个指标：

| 指标 | 计算方式 |
|------|----------|
| 🔥 **streak** | 连续有番茄完成的天数 |
| 🍅 **today** | 今日完成番茄数（查 sessions 表） |
| ✓ **done** | 今日 Todo 完成数 / 总数 |

---

### 模块 5：配置系统

**配置文件路径：** `~/.config/focus/config.yaml`

```yaml
# 天气
weather:
  city: "Shanghai"
  unit: "celsius"  # celsius / fahrenheit

# 每日一句
quote:
  source: "builtin"  # builtin / custom
  custom_file: "~/.config/focus/quotes.txt"

# Pomodoro
pomodoro:
  work_minutes: 25
  short_break_minutes: 5
  long_break_minutes: 15
  long_break_interval: 4
  notify: true

# 时间格式
time_format: "Mon Jan 2 · 15:04"

# 主题色（基于 Lipgloss 颜色系统）
theme:
  accent: "#7C3AED"
  done: "#6B7280"
  overdue: "#EF4444"
```

---

### 模块 6：模式系统

**浏览模式（默认）**
- 所有快捷键可用
- 焦点在 Todo 列表或 Pomodoro 区域

**输入模式**
- 按 `a` 或 `e` 进入
- 底部弹出输入框，输入新 Todo 文本或编辑现有文本
- `Enter` 确认，`Esc` 取消
- 输入模式下其他快捷键禁用

---

## 项目结构

```
focus-tui/
├── cmd/
│   └── focus/
│       └── main.go          # 入口
├── internal/
│   ├── app/
│   │   └── app.go           # Bubbletea App 主逻辑
│   ├── models/
│   │   ├── todo.go          # Todo 模型
│   │   └── session.go       # Pomodoro 会话模型
│   ├── storage/
│   │   └── db.go            # SQLite 操作
│   ├── config/
│   │   └── config.go        # 配置读取
│   ├── ui/
│   │   ├── styles.go        # Lipgloss 样式
│   │   ├── header.go        # 顶栏组件
│   │   ├── todo_list.go     # Todo 列表组件
│   │   ├── pomodoro.go      # 计时器组件
│   │   └── footer.go        # 底栏组件
│   ├── weather/
│   │   └── weather.go       # Open-Meteo API
│   └── quotes/
│       └── quotes.go        # 每日一句
├── go.mod
├── go.sum
└── SPEC.md                  # 本文档
```

---

## 开发里程碑

| 阶段 | 目标 |
|------|------|
| Phase 1 | 基础 TUI 框架 + SQLite 初始化 + 配置系统 |
| Phase 2 | Header 模块（时间、天气 API、每日一句） |
| Phase 3 | Todo 模块完整实现（CRUD + 模式系统） |
| Phase 4 | Pomodoro 计时器 + 系统通知 + 数据记录 |
| Phase 5 | Footer 数据统计 + UI 打磨 + README |

---

## 后续迭代方向

- Shell 模式（内嵌终端，v2）
- 周报统计视图：过去 7 天番茄数折线图（纯字符渲染）
- 插件系统：允许用户写 Go 插件挂载自定义 widget
- Sync 模式：可选把数据同步到 git repo
- `focus export`：导出今日完成情况为 Markdown 日报
