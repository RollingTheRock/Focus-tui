# Go TUI UX 优化方案调研

> 调研目标：在保持 Bubble Tea 技术栈的前提下，借鉴 Ink、Textual 等框架的设计范式，提升 Focus-tui 的终端用户体验。

---

## 一、当前技术栈诊断

Focus-tui 当前使用 Charm 生态的 v1 版本：

```
bubbletea  v1.3.10
lipgloss   v1.1.0
bubbles    v1.0.0
```

已有的 UX 基础：
- `styles/colors.go`：使用 `lipgloss.AdaptiveColor` 做亮色/暗色适配
- `styles/theme.go`：定义了 TitleStyle、BoxStyle、ActivePanelStyle 等主题 token
- 多 pane 布局系统 + overlay 弹层
- 语义化颜色（Success/Warning/StateActive/StateDone 等）

**核心瓶颈**：Bubble Tea v1 的 `View() string` 模型决定了布局基本靠字符串拼接 + `lipgloss.JoinHorizontal/Vertical`，没有真正的 flexbox 布局引擎，复杂响应式布局代码冗长。

---

## 二、标杆框架分析

### 2.1 Ink（JavaScript/React）

Ink 用 React 组件模型构建 TUI，核心优势：

| 特性 | Ink 做法 | Bubble Tea 现状 |
|------|---------|----------------|
| **布局** | `<Box flexDirection="row" justifyContent="center">` | `lipgloss.JoinHorizontal(lipgloss.Top, ...)` |
| **样式** | 组件 props：`borderStyle="round" borderColor="white"` | `lipgloss.NewStyle().Border(...).BorderForeground(...)` |
| **文本** | `<Text color="green" bold italic>` | `.Foreground(c).Bold(true).Italic(true)` |
| **弹性空间** | `<Spacer />` | 手动计算 padding |
| **截断/换行** | `<Text wrap="truncate">` | 无内置，需自己算 width |

**可借鉴的设计模式**：
- **组件化布局**：用组合式的 Box/Text/Spacer 替代字符串拼接
- **声明式样式**：将样式从 imperative 的 Go API 改为更接近 CSS 的声明

### 2.2 Textual（Python）

Textual 是目前 TUI UX 的天花板，核心优势：

| 特性 | Textual 做法 | 价值 |
|------|-------------|------|
| **CSS 文件** | `.tcss` 样式表分离结构与表现 | 设计师可独立调整样式 |
| **DOM 查询** | `query_one("#sidebar")` | 组件间通信更灵活 |
| **响应式属性** | `@reactive` 装饰器，自动触发重绘 | 状态驱动UI |
| **命令面板** | 内置 Cmd+Shift+P 式命令面板 | 快速导航 |
| **无障碍** | 屏幕阅读器模式 + 高对比度主题 | 可访问性 |
| **主题系统** | 内置 Dracula/Catppuccin 等 | 用户可切换 |

**可借鉴的设计模式**：
- **CSS-like 主题配置**：将颜色/间距/border 抽取为 token 文件
- **命令面板**：全局快速跳转
- **响应式状态**：减少手动消息传递

### 2.3 go-tui（新兴 Go 框架，2025-2026）

这是一个值得密切关注的项目——**在 Go 生态中最接近 Ink/Textual 体验**：

```go
// .gsx 模板文件，编译为类型安全的 Go
templ (c *counter) Render() {
    <div class="flex-col items-center justify-center h-full gap-1">
        <span class="font-bold text-cyan">{fmt.Sprintf("Count: %d", c.count.Get())}</span>
        <span class="font-dim">+/- to change, q to quit</span>
    </div>
}
```

特性：
- **Tailwind 风格类名**：`flex-col`, `gap-1`, `text-cyan`, `border-rounded`, `p-2`
- **Flexbox 布局引擎**：纯 Go 实现，无 CGO
- **Reactive State[T]**：`State.Bind()` 自动触发重绘
- **LSP + Tree-sitter**：有 VS Code 插件和 Neovim 插件

**风险**：Pre-1.0，API 可能变化。但思路非常值得参考。

---

## 三、Charm 生态的升级机会

### 3.1 Bubble Tea v2 + Lipgloss v2（已发布，2026.02）

**Breaking Changes**：
- 导入路径改为 `charm.land/bubbletea/v2`、`charm.land/lipgloss/v2`
- `View() string` → `View() tea.View`
- `lipgloss.AdaptiveColor{...}` → `lipgloss.LightDark(isDark)(light, dark)`

**UX 收益**：
- `tea.View` 是结构化视图，可设置 `AltScreen`, `MouseMode`, `ReportFocus`, `ProgressBar` 等
- 更好的终端能力检测（键盘增强、焦点事件）
- 颜色 profile 选择：`colorprofile.Detect()` + `lipgloss.Complete()`
- 自动 downsampling，无需手动处理

### 3.2 Huh（表单库）

Huh 已内置 **5 套主题**：Charm / Dracula / Catppuccin / Base16 / Default。

其 Theme 抽象可以**独立提取出来作为全局 Design System**：

```go
type Theme struct {
    Form           *FormStyles
    FieldSeparator *lipgloss.Style
    Blurred        *FieldStyles
    Focused        *FieldStyles
    ErrorMessage   *lipgloss.Style
    Help           *lipgloss.Style
}
```

Focus-tui 可以：
1. 将当前 `styles/theme.go` 扩展为与 Huh 兼容的 Theme 结构
2. 引入 Huh 做表单 overlay（当前 task-edit、plan-edit 是手工实现的）
3. 复用 Huh 的主题切换能力

### 3.3 Harmonica（动画库）

物理弹簧动画库，可用于：
- 进度条平滑过渡
- Pane 切换动画
- 滚动惯性

### 3.4 社区组件（Charm & Friends）

| 库 | 用途 |
|----|------|
| `evertras/bubble-table` | 比 bubbles/table 更强大的表格 |
| `rmhubbert/bubbletea-overlay` | Modal / Overlay 组件 |
| `kevm/bubbleo` | 导航栈、面包屑、菜单 |
| `ntcharts` | 终端图表（Sparkline、BarChart）|

---

## 四、推荐优化路线

### 路线 A：渐进式改良（推荐）

不更换框架，在 Bubble Tea v1/v2 基础上引入设计系统：

1. **升级到 v2**（如果稳定）：获得 `tea.View` 和更好的颜色管理
2. **建立 Design Token 系统**：
   ```
   colors/     # 语义化颜色（primary, success, warning, error）
   spacing/    # 间距 scale（xs, sm, md, lg, xl）
   typography/ # 字体样式（heading, body, caption, code）
   borders/    # 边框风格（radius, width, style）
   ```
3. **引入 Huh 主题引擎**：复用其 Theme/Styles 抽象
4. **封装布局组件**：模仿 Ink 的 Box/Spacer 模式
   ```go
   // 用 Go 代码模拟声明式布局
   layout.Column(
       layout.Row(layout.Flex1, header, layout.Flex2, body),
       layout.Row(layout.Fixed(3), footer),
   )
   ```
5. **添加命令面板**：全局 `Ctrl+Shift+P` 快速跳转 pane/overlay

### 路线 B：布局引擎增强

如果布局复杂度继续增长，可考虑：

1. **引入 `bubblezone`**：让 mouse 事件可以精准命中 pane
2. **参考 go-tui 的 flexbox 算法**：在 Bubble Tea 之上封装一个轻量布局层
3. **响应式断点**：根据 terminal width/height 自动切换布局

### 路线 C：长期激进（观望）

关注 `go-tui` 的成熟度。如果它达到 1.0 且稳定，可以考虑：
- 核心界面用 `.gsx` 重写
- 保留 Bubble Tea 的底层事件循环
- 但迁移成本较高，现阶段不建议

---

## 五、具体行动建议

### 立即可做的（低风险）

1. **扩展 Theme 系统**
   - 将 `styles/theme.go` 从 `DefaultTheme()` 改为支持多主题（Dark/Light/HighContrast）
   - 引入 `lipgloss.LightDark()` 替代 `AdaptiveColor`（兼容 v2 迁移）

2. **封装布局 helper**
   - 在 `internal/ui/layout/` 中加入 `FlexRow`, `FlexColumn`, `Spacer` 等 helper
   - 减少直接调用 `lipgloss.JoinHorizontal/Vertical`

3. **引入 Huh 做表单**
   - task-edit、plan-edit 等 overlay 用 Huh 的 Form 重写
   - 自动获得主题切换、无障碍、验证等功能

### 中期可做的（中等风险）

4. **升级到 Bubble Tea v2**
   - 评估 `View() tea.View` 的迁移成本
   - 利用新能力：ProgressBar、BackgroundColorMsg

5. **构建命令面板**
   - 一个全局 overlay：列出所有 pane、命令、快捷键
   - 类似 VS Code 的 Command Palette

6. **动画增强**
   - 引入 Harmonica 做平滑过渡
   - pane focus 切换时加 subtle 动画

### 长期观望的

7. **go-tui 评估**
   - 每季度检查其 release 状态
   - 用小型 PoC 验证与现有架构的兼容性

---

## 六、附录：对比总表

| 维度 | Bubble Tea v1 | Bubble Tea v2 | Ink | Textual | go-tui |
|------|--------------|---------------|-----|---------|--------|
| 架构 | Elm (M-U-V) | Elm (M-U-V) | React | OOP+Reactive | Reactive Component |
| 布局 | 字符串拼接 | 结构化 View | Flexbox | Flex/Grid | Flexbox |
| 样式 | Go API (lipgloss) | Go API | JSX props | CSS 文件 | Tailwind classes |
| 主题 | 自建 | 自建 | 自建 | 内置5套 | 编译时生成 |
| 组件库 | Bubbles | Bubbles | 自建 | 丰富内置 | HTML primitives |
| 动画 | Harmonica | Harmonica | 无 | 内置 | 有 |
| 命令面板 | 无 | 无 | 无 | 有 | 无 |
| 无障碍 | 无 | 无 | 无 | 有 | 无 |
| 成熟度 | 生产级 | 生产级 | 生产级 | 生产级 | Pre-1.0 |
| Go 原生 | ✅ | ✅ | ❌ | ❌ | ✅ |

---

*调研时间：2026-05-30*
*建议下一步：与团队确认是走「路线 A 渐进式改良」还是考虑更大范围的 v2 升级。*
