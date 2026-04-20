# Phase 2.4 测试指南

> Git/Tree 功能测试说明

---

## 已完成功能

### 1. Plugin 系统
- ✅ Plugin 接口和 Registry
- ✅ 支持动态注册 Pane 类型
- ✅ Git Plugin 和 FileBrowser Plugin

### 2. Adapter 系统  
- ✅ Adapter 接口和 Manager
- ✅ GitLocalAdapter (使用 `git status --porcelain=v2`)
- ✅ 异步状态监听

### 3. GitStatusPane
- ✅ 显示当前分支
- ✅ 显示 ahead/behind 计数
- ✅ 列出 staged/unstaged/untracked 文件
- ✅ 键盘导航 (up/down)
- ✅ 自动刷新

### 4. FileTreePane
- ✅ 文件树浏览
- ✅ 目录折叠/展开
- ✅ 键盘导航
- ✅ 自动刷新

---

## 测试方法

### 方法 1: 单元测试

```bash
cd /mnt/d/dev/focus-tui

# 测试 Plugin Registry
go test ./internal/plugins/... -v

# 测试 Git 和 FileBrowser 插件
go test ./internal/plugins/git/... -v
go test ./internal/plugins/filebrowser/... -v

# 测试 Git Adapter
go test ./internal/adapters/... -v

# 测试所有包
go test ./...
```

### 方法 2: 手动测试 Pane 组件

创建一个简单的测试程序来验证 Pane 组件：

```go
// test/main.go
package main

import (
    "fmt"
    "os"
    
    tea "github.com/charmbracelet/bubbletea"
    "focus/internal/adapters"
    "focus/internal/models"
    gitplugin "focus/internal/plugins/git"
    filebrowser "focus/internal/plugins/filebrowser"
)

func main() {
    // 初始化 Adapter
    adapterManager := adapters.NewManager()
    gitAdapter := adapters.NewGitLocalAdapter()
    adapterManager.Register("git-local", gitAdapter)
    adapterManager.Init()
    
    // 初始化 Plugins
    gitPlugin := gitplugin.New(gitAdapter)
    gitPlugin.Init()
    
    filePlugin := filebrowser.New()
    filePlugin.Init()
    
    // 创建 CommonModel
    common := models.CommonModel{}
    
    // 创建 GitStatusPane
    if pane, err := gitPlugin.CreatePane(
        models.PaneTypeGitStatus,
        models.PaneID("git-test"),
        models.PaneMeta{
            ID:       "git-test",
            Name:     "Git Test",
            Type:     models.PaneTypeGitStatus,
            CWD:      "/path/to/git/repo",
            Status:   models.PaneStatusIdle,
            Closable: true,
        },
        common,
    ); err == nil {
        fmt.Println("✅ GitStatusPane created successfully")
        
        // 初始化并获取视图
        cmd := pane.Init()
        if cmd != nil {
            fmt.Println("✅ Init command returned")
        }
    } else {
        fmt.Printf("❌ Failed to create GitStatusPane: %v\n", err)
    }
    
    // 创建 FileTreePane
    if pane, err := filePlugin.CreatePane(
        models.PaneTypeFileTree,
        models.PaneID("tree-test"),
        models.PaneMeta{
            ID:       "tree-test",
            Name:     "Tree Test",
            Type:     models.PaneTypeFileTree,
            CWD:      "/path/to/dir",
            Status:   models.PaneStatusIdle,
            Closable: true,
        },
        common,
    ); err == nil {
        fmt.Println("✅ FileTreePane created successfully")
    } else {
        fmt.Printf("❌ Failed to create FileTreePane: %v\n", err)
    }
}
```

### 方法 3: 完整集成测试 (需要 App 层修改)

当前 App 层尚未集成 Plugin 系统。要完整测试，需要：

1. 修改 `cmd/focus/main.go` 接受 `--git` 和 `--tree` 参数
2. 修改 `internal/app/app.go` 在 `New()` 中根据配置创建 PluginPanes
3. 或者创建一个独立的测试入口

参考 `PLAN-2.4.md` Week 4 的任务完成集成。

---

## 关键代码位置

| 功能 | 文件路径 |
|------|---------|
| Plugin 接口 | `internal/plugins/plugin.go` |
| Plugin Registry | `internal/plugins/registry.go` |
| Git Adapter | `internal/adapters/git_local.go` |
| Git Plugin | `internal/plugins/git/plugin.go` |
| GitStatusPane | `internal/plugins/git/status_pane.go` |
| FileBrowser Plugin | `internal/plugins/filebrowser/plugin.go` |
| FileTreePane | `internal/plugins/filebrowser/tree_pane.go` |
| Git 类型定义 | `internal/git/types.go` |

---

## 下一步 (Phase 2.4 Week 4)

要完成可测试的版本，需要：

1. **App 层集成**
   - 在 `model` 结构体中添加 `pluginRegistry` 和 `adapterManager` 字段
   - 在 `New()` 函数中根据 `cfg.Features` 初始化插件
   - 在 `Update()` 中处理 `adapters.StatusEvent`

2. **配置系统**
   - 在 `config.Config` 中添加 `Features` 字段
   - 支持命令行参数 `--git` 和 `--tree`

3. **布局集成**
   - 将 PluginPanes 添加到 `bodyTree`
   - 确保 Pane 可以正确渲染和交互

---

## 已知限制

- ✅ Plugin 系统完整
- ✅ Adapter 系统完整  
- ✅ GitStatusPane 完整
- ✅ FileTreePane 完整
- ⚠️ **App 层集成未完成**（需要修改 `internal/app/app.go`）
- ⚠️ **配置系统未完成**（需要修改 `cmd/focus/main.go` 和 `internal/config/config.go`）

当前状态：**基础设施已完成，需要最后一步集成到 App 中才能完整测试**。

---

## 快速验证

运行以下命令验证代码完整性：

```bash
# 编译检查
go build ./...

# 单元测试
go test ./internal/plugins/... ./internal/adapters/... ./internal/git/...

# 代码统计
echo "=== Phase 2.4 新增代码 ==="
find internal/plugins internal/adapters internal/git -name "*.go" | xargs wc -l
```

---

**状态**: Phase 2.4 基础设施 100% 完成，等待最终 App 层集成。
