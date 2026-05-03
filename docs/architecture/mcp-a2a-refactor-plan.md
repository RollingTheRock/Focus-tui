# MCP/A2A 协议层重构方案

## 背景

当前 Focus-tui 的 MCP 实现是自定义 JSON-RPC over Unix socket，A2A 是自定义 Pub/Sub over Unix socket。外部 agent CLI（Kimi、Codex、OpenCode、Claude Code）虽然都原生支持标准 MCP，但无法连接到 TUI 的自定义协议服务器。

## 目标

1. 将 MCP 升级为标准 **Model Context Protocol**（initialize、tools/list、tools/call）
2. 暴露 **HTTP 端口**供 agent CLI 直接连接
3. 重新审视 A2A 定位，简化架构
4. 修复生命周期管理缺陷

---

## 1. 标准 MCP Server 重构

### 当前问题

```go
// 当前 server.go：直接把所有 JSON-RPC method 映射为 tool name
func (s *Server) handleRequest(req rpcRequest) rpcResponse {
    result, err := s.CallTool(req.Method, req.Params)  // ← 缺少 initialize、tools/list
    ...
}
```

### 标准 MCP 协议流程

```
Client                              Server
  | ──POST initialize──────────────> |
  | <────────initialize result────── |
  | ──POST notifications/initialized>|
  | ──POST tools/list─────────────> |
  | <────────tools/list result────── |
  | ──POST tools/call─────────────> |
  | <────────tools/call result────── |
```

### 需要实现的方法

| 方法 | 方向 | 说明 |
|------|------|------|
| `initialize` | C→S | 握手，交换 capabilities |
| `notifications/initialized` | C→S | 客户端确认初始化完成 |
| `tools/list` | C→S | 获取可用工具列表 |
| `tools/call` | C→S | 调用指定工具 |
| `ping` | C→S | 保活（可选） |

### 新增类型（protocol.go）

```go
// Initialize
 type InitializeRequest struct {
     ProtocolVersion string                 `json:"protocolVersion"`
     Capabilities    ClientCapabilities     `json:"capabilities"`
     ClientInfo      Implementation         `json:"clientInfo"`
 }
 type InitializeResult struct {
     ProtocolVersion string                 `json:"protocolVersion"`
     Capabilities    ServerCapabilities     `json:"capabilities"`
     ServerInfo      Implementation         `json:"serverInfo"`
 }

// Tools
 type Tool struct {
     Name        string                 `json:"name"`
     Description string                 `json:"description"`
     InputSchema map[string]any         `json:"inputSchema"`
 }
 type ListToolsResult struct {
     Tools []Tool `json:"tools"`
 }
 type CallToolRequest struct {
     Name      string         `json:"name"`
     Arguments map[string]any `json:"arguments"`
 }
 type TextContent struct {
     Type string `json:"type"`
     Text string `json:"text"`
 }
 type CallToolResult struct {
     Content []any  `json:"content"`
     IsError bool   `json:"isError,omitempty"`
 }
```

---

## 2. HTTP Transport（Streamable HTTP）

采用 MCP 2025-03-26 规范中的 **Streamable HTTP**：

- **Endpoint**: `POST /mcp`
- **Request**: JSON-RPC 2.0 request body
- **Response**: JSON-RPC 2.0 response body（常规 HTTP 响应）
- **SSE**: 如需 server→client push，返回 `text/event-stream`

### 架构

```
┌─────────────┐     POST /mcp      ┌─────────────────┐     tool handler    ┌─────────────┐
│ Agent CLI   │ ────────────────> │  MCP HTTP Server│ ──────────────────> │   TUI       │
│ (kimi etc)  │ <──────────────── │  (127.0.0.1:PORT│ <────────────────── │  (app.go)   │
└─────────────┘   JSON-RPC 2.0    └─────────────────┘   tool result       └─────────────┘
```

Agent 配置示例：

```bash
# Kimi
kimi mcp add focus --transport http http://localhost:18765/mcp

# Codex
codex mcp add focus --url http://localhost:18765/mcp

# Claude Code
# settings.json: { "mcpServers": { "focus": { "url": "http://localhost:18765/mcp" } } }
```

---

## 3. 端口选择策略

### 优先级

```
1. 配置文件: cfg.Agent.MCPPort  （如 "18765"）
2. 环境变量: FOCUS_MCP_PORT    （如 "18765"）
3. 自动选择: 127.0.0.1:0       （操作系统分配随机端口）
```

### 启动后行为

```go
func (s *Server) StartHTTP() (string, error) {
    addr := s.httpAddr
    if addr == "" {
        addr = "127.0.0.1:0"  // 自动选择
    }
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return "", err
    }
    s.httpListener = listener
    s.httpURL = "http://" + listener.Addr().String() + "/mcp"
    go s.httpServe(listener)

    // 持久化端口信息
    _ = s.writePortFile(s.httpURL)
    return s.httpURL, nil
}
```

### 多实例处理

| 场景 | 行为 |
|------|------|
| 用户指定端口 | 使用该端口；冲突时报错 |
| 自动选择端口 | 每个实例独立端口 |
| 端口信息存储 | `~/.focus/mcp.port`（最新实例覆盖） |
| TUI 显示 | Footer 显示当前 MCP URL |

### 安全性

- 仅绑定 `127.0.0.1`，拒绝外部连接
- 未来可添加 Bearer Token 验证（通过 `Authorization` header）

---

## 4. A2A 重新定位：建议废弃

### 当前 A2A 功能 vs MCP 替代

| A2A 功能 | 当前实现 | MCP 替代方案 |
|---------|---------|------------|
| `session.heartbeat` | A2A message → `mcpSessionHeartbeatTool` | Agent 直接调用 MCP tool `session.heartbeat` |
| `status.update` | A2A message → `mcpTaskUpdateStatusTool` | Agent 直接调用 MCP tool `task.update_status` |
| `task.delegation` | TUI 通过 A2A 向 agent 推送 | 环境变量 `FOCUS_TASK_ID` + agent 调用 `context.get_for_task` |

### 废弃理由

1. **生态兼容性**：主流 agent CLI（Kimi、Codex、Claude、OpenCode）广泛支持 MCP，但 **A2A 支持还不普及**
2. **外部终端模式的限制**：agent 运行在外部终端中（ADR-0004），TUI 无法干预外部终端的 UI。即使 A2A 推送消息到 agent，agent CLI 也不一定有机制将其展示给用户
3. **架构简化**：单协议（MCP）比双协议（MCP + A2A）更易维护
4. **功能无损失**：A2A 的所有功能都可以通过 MCP tool 调用实现

### TUI→Agent 推送的替代方案

如果未来需要 TUI 主动向 agent 发送通知（如"人类要求干预"）：

**方案 A（推荐）**：Agent 轮询
- Agent 在每次 turn 开始时调用 `session.check_intervention` MCP tool
- TUI 返回是否有待处理的干预请求
- 优点：无需额外协议，agent 端零适配

**方案 B**：MCP SSE stream
- 如果 agent CLI 的 MCP HTTP client 支持长连接 SSE
- TUI 可以通过 SSE stream 主动推送通知
- 优点：实时推送
- 风险：不确定各 agent CLI 的 SSE client 实现是否支持 server→client push

### 决策

**废弃 `internal/a2a/` 自定义 A2A，全部功能迁移到 MCP。**

若未来标准 A2A 生态成熟且 agent CLI 广泛支持，可再评估是否引入标准 A2A（基于 HTTP）。

---

## 5. 生命周期修复

### 当前问题

```go
// app.go
_ = m.mcpServer.Start()      // 错误被忽略
_ = m.a2aRouter.Start()      // 错误被忽略
// tea.Quit 时没有任何 Stop() 调用
```

### 修复方案

```go
// 1. 错误处理
if err := m.mcpServer.Start(); err != nil {
    log.Printf("mcp server start failed: %v", err)
}
if err := m.mcpServer.StartHTTP(); err != nil {
    log.Printf("mcp http start failed: %v", err)
}

// 2. Quit 时清理
func (m *model) Close() tea.Cmd {
    return func() tea.Msg {
        if m.mcpServer != nil {
            _ = m.mcpServer.Stop()
        }
        return nil
    }
}

// 3. 使用 context.Context 管理 goroutine
// HTTP server 使用 http.Server{BaseContext: ...}
// Unix socket server 使用 select { case <-ctx.Done(): }
```

---

## 6. Agent 环境变量更新

启动外部 agent 时，替换旧的环境变量：

| 旧变量 | 新变量 | 说明 |
|--------|--------|------|
| `FOCUS_MCP_SOCKET=/tmp/focus-mcp.sock` | `FOCUS_MCP_URL=http://localhost:PORT/mcp` | MCP HTTP endpoint |
| `FOCUS_A2A_SOCKET=/tmp/focus-a2a.sock` | **移除** | A2A 废弃 |
| `FOCUS_SESSION_ID` | `FOCUS_SESSION_ID` | 保留 |
| `FOCUS_TASK_ID` | `FOCUS_TASK_ID` | 保留 |
| `FOCUS_PLAN_ID` | `FOCUS_PLAN_ID` | 保留 |

---

## 7. 文件结构

```
internal/mcp/
  protocol.go        # 新增：标准 MCP JSON-RPC 类型
  server.go          # 重构：标准 MCP 核心逻辑 + HTTP transport
  stdio.go           # 新增：stdio transport（供 future use / focus mcp 子命令）
  server_test.go     # 更新：标准协议测试 + HTTP 测试
  helpers.go         # 新增：端口文件读写、工具元数据生成

internal/a2a/        # 废弃（整个目录删除）
  router.go
  router_test.go

internal/agents/
  types.go           # 更新：移除 A2A 环境变量，更新 MCP 环境变量
  launcher.go        # 更新：传递 FOCUS_MCP_URL

internal/app/app.go  # 更新：
  - 移除 a2aRouter 相关代码
  - mcpServer 使用标准协议
  - 启动 HTTP transport
  - Quit 时 Stop()
  - handleA2AMessage 相关代码移除

internal/config/
  config.go          # 新增：Agent.MCPPort
```

---

## 8. 实施顺序

| 阶段 | 任务 | 估计工作量 |
|------|------|---------|
| **Phase 1** | 定义标准 MCP 类型（protocol.go） | 小 |
| **Phase 2** | 重构 server.go：标准协议 + HTTP transport | 中 |
| **Phase 3** | 更新 app.go：移除 A2A，集成新 MCP，生命周期修复 | 中 |
| **Phase 4** | 更新 agent 启动流程：新环境变量 | 小 |
| **Phase 5** | 更新/添加测试 | 中 |
| **Phase 6** | 验证：用 kimi/codex 实际连接测试 | 小 |

---

## 9. 风险与回退

| 风险 | 缓解措施 |
|------|---------|
| HTTP 端口冲突 | 自动回退到随机端口 |
| Agent CLI 的 MCP HTTP 实现差异 | 先测试 Kimi（已知支持 HTTP），再测试 Codex/OpenCode |
| 移除 A2A 后失去 TUI→Agent 推送 | 通过 MCP tool 轮询替代；如不满足，后期可加 SSE push |
| 标准 MCP 协议版本演进 | 实现时声明支持 `2024-11-05`，后续升级 |
