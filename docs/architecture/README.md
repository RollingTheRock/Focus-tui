# Focus-tui 架构设计

本文档描述 Focus-tui 的完整系统架构，基于 [ADR-0004: Protocol-Driven Multi-Agent Orchestration](../adr/0004-protocol-driven-multi-agent-orchestration.md)。

## 文档导航

| 文档 | 内容 |
|------|------|
| [architecture.md](./architecture.md) | 完整架构设计：组件、数据流、状态机、接口契约 |
| [ADR-0000](../adr/0000-constitution-for-adr-driven-execution.md) | 宪法：ADR → Plan → Task → Session 层级 |
| [ADR-0001](../adr/0001-product-positioning-human-sovereign-agent-native-workbench.md) | 产品定位：人类主权、Agent 原生 |
| [ADR-0004](../adr/0004-protocol-driven-multi-agent-orchestration.md) | 架构决策：去中心化 Agent Mesh、外部终端执行 |

## 架构核心

```
┌─────────────────────────────────────────────────────────────────┐
│                     Focus-tui（指挥中心）                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │ Dashboard   │  │ Task Board  │  │ Agent Grid              │ │
│  │（调度状态）   │  │（任务依赖图） │  │（Agent 状态卡片）        │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │ ADR Browser │  │ Knowledge   │  │ Human Shell             │ │
│  │（约束查询）   │  │ Graph       │  │（人类工作区）            │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Orchestrator（调度器）─ 纯规则引擎                         │   │
│  │ MCP Server（共享状态）─ SQLite 包装                        │   │
│  │ A2A Router（信号层）─ 本地 Socket                          │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          │ MCP               │ A2A               │ 进程管理
          ▼                   ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Terminal 1      │  │  Terminal 2      │  │  Terminal 3      │
│  $ claude        │  │  $ opencode      │  │  $ kimi          │
│  Agent-1         │  │  Agent-2         │  │  Agent-3         │
│  （自由运行）     │  │  （自由运行）     │  │  （自由运行）     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## 关键特征

- **Agent 在外部终端自由运行**：每个 Agent 是独立进程，拥有完整的终端体验
- **Focus-tui 纯指挥中心**：不显示 Agent 输出，只显示调度状态和数据视图
- **协议驱动**：MCP（共享状态）+ A2A（轻量信号）
- **人类主权**：人类直接操作 MCP 共享状态，不通过 Agent 中转
- **去中心化**：无 Global Agent，无中心大脑

## 快速链接

- [组件架构](./architecture.md#组件架构)
- [数据流](./architecture.md#数据流)
- [状态机](./architecture.md#状态机)
- [接口契约](./architecture.md#接口契约)
- [数据库 Schema](./architecture.md#数据库设计)
