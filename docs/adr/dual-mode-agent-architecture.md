# ADR: Dual-Mode Agent Architecture — Interactive (Trellis-style) + Headless (Symphony-style)

**Status:** Proposed  
**Date:** 2026-04-29  
**Branch:** `feat-long-session`  
**Context:** Agent session digest MVP completed; exploring evolution from "post-hoc file reading" to "structured communication"

---

## 1. 背景与问题陈述

### 1.1 当前状态

Focus-tui 已实现：

- **Interactive 模式**：通过 `tea.ExecProcess` 或外部终端（kitty/alacritty/wezterm/gnome-terminal/ptyxis）启动 Agent CLI（Claude/Codex/Kimi/OpenCode），Agent 在独立终端中自主运行，Focus-tui 仅管理生命周期。Session 结束后通过 `sessiondigest` 包异步读取 `.jsonl` 文件 + `git diff` 生成沉淀。
- **基础设施**：MCP Server（Unix Socket JSON-RPC）、A2A Router（Unix Socket pub/sub）、SQLite 存储（`agent_sessions`, `task_outputs`, `session_handoffs`, `plan_steps` 等）、Provider picker overlay。

### 1.2 核心瓶颈

| 瓶颈 | 说明 |
|------|------|
| **事后读文件** | Agent 运行期间 Focus-tui 完全盲视，只能等退出后读 `.jsonl`，实时干预 impossible |
| **Monolithic 配置** | AGENTS.md 是单一大文件，无法按 task/step 渐进加载 |
| **无 per-worktree 配置空间** | 每个 worktree 的 task 上下文、规范、约束无法结构化存储 |
| **无 Skill/Command 系统** | Agent 没有标准化的 workflow commands（/start, /check, /finish-work）|
| **Headless 缺失** | 无法让 Agent 自主运行并在 Focus-tui 中实时监控和审批 |
| **无 Orchestrator Control Plane** | Task Board 只是静态看板，不是 Agent 调度的 Control Plane |

### 1.3 参考系统

- **Trellis** (`mindfold-ai/trellis`)：以 `.trellis/` 为配置空间的多平台 Agent Harness，核心能力是渐进式 Spec 加载、Task PRD、Workspace Journal、Skill/Command 系统、Hook 自动注入。
- **Symphony** (`openai/symphony`)：以 Issue Tracker 为 Control Plane 的 Agent Orchestrator，核心能力是 Objective 驱动、DAG 自动执行、Guardrails（CI 监控、自动 rebase、PR shepherding）、Codex App Server Headless 驱动。

---

## 2. 设计原则

1. **Worktree 是一等公民**：所有配置、上下文、Session 都以 worktree 为锚点。
2. **渐进式配置，非 Monolithic**：Spec 按领域/任务/步骤分层组合，按需加载。
3. **同一套 Task/Plan/Step 模型，两种执行界面**：Interactive 和 Headless 共享数据模型，差异仅在执行方式。
4. **Focus-tui 是 Control Plane**：Task Board 是 Agent 调度的控制中心，不是静态看板。
5. **Session 沉淀统一**：无论哪种模式，结束后统一走 `SessionDigest` → SQLite。
6. **No Takeover**：同一个 session 不能从 Headless 切换到 Interactive（技术不可行），而是同一个 task 的不同 session。

---

## 3. 双层架构全景

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Focus-tui Control Plane                              │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Task Board (Orchestrator)                     │   │
│   │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────────────┐  │   │
│   │  │ Open     │  │ In       │  │ Blocked  │  │ Done                │  │   │
│   │  │ Tasks    │  │ Progress │  │          │  │                     │  │   │
│   │  │          │  │          │  │          │  │                     │  │   │
│   │  │ #123     │  │ #124     │  │ #125     │  │ #120, #121, #122    │  │   │
│   │  │ #126     │  │          │  │          │  │                     │  │   │
│   │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └─────────────────────┘  │   │
│   │       │             │             │                                  │   │
│   │       └─────────────┴─────────────┘                                  │   │
│   │                    │                                                 │   │
│   │                    ▼                                                 │   │
│   │         ┌─────────────────────┐                                    │   │
│   │         │   Dispatch Router   │                                    │   │
│   │         │  (DAG + Mode + Wt)  │                                    │   │
│   │         └─────────────────────┘                                    │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                              │                                               │
│          ┌───────────────────┴───────────────────┐                           │
│          ▼                                       ▼                           │
│   ┌─────────────────────────┐          ┌─────────────────────────┐          │
│   │    Interactive Mode     │          │     Headless Mode       │          │
│   │    (Trellis-style)      │          │    (Symphony-style)     │          │
│   │                         │          │                         │          │
│   │  Human-in-the-Center    │          │  Human-at-the-Gate      │          │
│   │  Agent is external tool │          │  Agent is autonomous    │          │
│   │  Post-hoc + real-time   │          │  Real-time stream +     │          │
│   │  observation (future)   │          │  approval gates         │          │
│   │                         │          │                         │          │
│   │  Launch: user "A" key   │          │  Launch: Orchestrator   │          │
│   │  Exit: user Ctrl+D      │          │  Exit: done/error/abort │          │
│   │                         │          │                         │          │
│   │  ┌───────────────────┐  │          │  ┌───────────────────┐  │          │
│   │  │ External Terminal │  │          │  │ Agent SDK Driver  │  │          │
│   │  │ (kitty/alacritty) │  │          │  │ (subprocess/      │  │          │
│   │  │                   │  │          │  │  JSON-RPC)        │  │          │
│   │  │ Agent CLI TUI     │  │          │  │                   │  │          │
│   │  │ (claude/codex)    │  │          │  │ Stream channel    │  │          │
│   │  └───────────────────┘  │          │  │ Events → Pane     │  │          │
│   │                         │          │  └───────────────────┘  │          │
│   └─────────────────────────┘          └─────────────────────────┘          │
│                              │                                               │
│          ┌───────────────────┴───────────────────┐                           │
│          ▼                                       ▼                           │
│   ┌─────────────────────────┐          ┌─────────────────────────┐          │
│   │   Per-Worktree Config   │          │   Per-Worktree Config   │          │
│   │   Space (.focus/wt/)    │          │   Space (.focus/wt/)    │          │
│   │                         │          │                         │          │
│   │  spec/AGENTS.md         │          │  spec/AGENTS.md         │          │
│   │  spec/context.md        │          │  spec/context.md        │          │
│   │  spec/constraints.md    │          │  spec/constraints.md    │          │
│   │  handoff/*.md           │          │  handoff/*.md           │          │
│   │  journal/*.md           │          │  journal/*.md           │          │
│   └─────────────────────────┘          └─────────────────────────┘          │
│                              │                                               │
│          ┌───────────────────┴───────────────────┐                           │
│          ▼                                       ▼                           │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    Unified Infrastructure                            │   │
│   │                                                                      │   │
│   │  MCP Server (Unix Socket)      A2A Router (Unix Socket)             │   │
│   │  ├── session.heartbeat         ├── status.heartbeat                 │   │
│   │  ├── session.request_approval  ├── status.update                    │   │
│   │  ├── task.get                  ├── task.delegation                  │   │
│   │  ├── task.create_output        └── artifact.reference               │   │
│   │  ├── task.update_status                                              │   │
│   │  ├── kg.add_fact                                                     │   │
│   │  └── context.get_for_task                                            │   │
│   │                                                                      │   │
│   │  SessionDigest (统一沉淀)                                            │   │
│   │  ├── digest.go (Markdown 渲染)                                       │   │
│   │  ├── builder.go (事件+GitDiff 组装)                                  │   │
│   │  └── store.go (Persist → SQLite)                                     │   │
│   │                                                                      │   │
│   │  Guardrails (轻量版)                                                 │   │
│   │  ├── heartbeat timeout detection                                     │   │
│   │  ├── step stall detection                                            │   │
│   │  └── git state validation                                            │   │
│   │                                                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                              │                                               │
│                              ▼                                               │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Unified Storage                               │   │
│   │  (SQLite: agent_sessions, task_outputs, session_handoffs,            │   │
│   │   plan_steps, agent_events, agent_approvals, task_dependencies)      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Interactive 模式：借鉴 Trellis 的完整形态

### 4.1 Trellis 核心形态回顾

Trellis 不是"终端对"那么简单。它的核心是一个**以 `.trellis/` 为配置空间的多平台 Agent Harness**，包含六个层次：

| 层次 | 位置 | 作用 |
|------|------|------|
| **Spec** | `.trellis/spec/` | 渐进式编码规范，按领域/模块组织 |
| **Task** | `.trellis/tasks/{task}/` | 自包含的任务空间：PRD + context + status |
| **Workspace** | `.trellis/workspace/{developer}/` | 开发者级别的 session journal |
| **Workflow** | `.trellis/workflow.md` | 共享生命周期规则 |
| **Skill** | platform-specific | 自动触发的工作流模块 |
| **Hook** | `.claude/hooks/` 等 | Session 启动时自动注入上下文 |

Trellis 的核心工作流命令：

| Command | 触发时机 | 作用 |
|---------|---------|------|
| `/start` | 每个 session 开始 | 加载项目上下文 |
| `/brainstorm` | 新功能/scope 不清 | 生成 PRD |
| `/before-dev` | 编码前 | 读取相关 spec |
| `/check` | 实现后 | 对照 spec 检查 |
| `/finish-work` | 提交前 | Pre-commit checklist |
| `/parallel` | 大任务拆分 | 多 worktree 并行 |
| `/record-session` | Session 结束 | 保存到 workspace journal |
| `/update-spec` | 发现新模式 | 保存到 spec |

### 4.2 Focus-tui Interactive 的映射

Focus-tui 的优势是**每个 worktree 天然对应一个 Task/Plan**，因此可以比 Trellis 更细粒度。

#### 4.2.1 Per-Worktree Agent Configuration Space

当用户为 worktree 分配 Task/Plan 时，Focus-tui 自动生成/维护：

```
{worktree-path}/
└── .focus/                              # Focus-tui 管理（可 gitignore）
    ├── spec/
    │   ├── AGENTS.md                    # 主入口：任务目标 + 计划步骤
    │   ├── context.md                   # Task Goal + Plan Body + Step Notes
    │   ├── constraints.md               # 代码规范、检查规则
    │   └── step-{order}.md              # 每个 step 的专项上下文（按需生成）
    ├── handoff/
    │   └── prev-{session-id}.md         # 上一个 session 的交接摘要
    ├── journal/
    │   └── {developer-name}.md          # 该开发者在该 worktree 的 session 日志
    └── workflow.md                      # 该 worktree 的工作流规则（继承项目级）
```

**与 Trellis 的关键差异**：

- Trellis 的 `.trellis/` 是**项目级**的，所有 worktree 共享。
- Focus-tui 的 `.focus/` 是**worktree 级**的，每个 worktree 的 spec 是该 task 的专用配置。
- 项目级通用规范放在 repo root 的 `.focus/spec/` 或 `AGENTS.md`，worktree 级配置自动继承并覆盖。

#### 4.2.2 渐进式 Spec 加载（核心）

Trellis 的核心创新是**不是把所有 spec 塞进一个 prompt，而是按需渐进加载**。

Focus-tui 的实现：

```go
// internal/agents/spec_loader.go

type SpecLoader struct {
    repoSpecDir     string        // ~/.focus/spec/ 或 repo/.focus/spec/
    worktreeSpecDir string        // {worktree}/.focus/spec/
}

func (l *SpecLoader) LoadForStep(step PlanStepRecord, task TaskContextRecord) (*AgentSpec, error) {
    spec := &AgentSpec{}
    
    // Layer 1: 项目级通用规范（coding standards, architecture）
    spec.AddLayer(l.loadRepoSpec("general"))
    
    // Layer 2: 领域级规范（backend/frontend/database）
    domain := inferDomain(task.Goal, step.Title)
    spec.AddLayer(l.loadRepoSpec(domain))
    
    // Layer 3: Worktree 级任务上下文
    spec.AddLayer(l.loadWorktreeSpec("context"))
    
    // Layer 4: 当前 Step 专项上下文
    spec.AddLayer(l.loadWorktreeSpec(fmt.Sprintf("step-%d", step.OrderIndex)))
    
    // Layer 5: Handoff 上下文（如果有）
    if handoff := l.loadLatestHandoff(); handoff != nil {
        spec.AddLayer(handoff)
    }
    
    return spec, nil
}
```

**效果**：
- Step 1（Research）加载：general + research 领域规范 + task context
- Step 2（Implement）加载：general + backend 领域规范 + task context + implement step 规范
- 不是 monolithic，而是**分层、按需、可组合**。

#### 4.2.3 Hook 机制（自动注入）

Trellis 的 Hook 在支持的平台（Claude Code, Codex, iFlow, OpenCode）上自动在 session start 时注入上下文。

Focus-tui 的实现：

```go
// Agent 启动时，自动将 AGENTS.md 注入到 worktree

func (m *model) prepareWorktreeAgentProfile(worktreeID, taskID, planID, stepID string) error {
    // 1. 查询 Task/Plan/Step
    task, _ := m.common.Store.GetTaskContext(taskID)
    plan, _ := m.common.Store.GetTaskPlan(planID)
    steps, _ := m.common.Store.ListPlanSteps(planID)
    
    // 2. 生成 AGENTS.md
    loader := agents.NewSpecLoader(m.repoSpecDir(), worktreeSpecDir(worktreeID))
    spec, _ := loader.LoadForStep(currentStep, *task)
    
    // 3. 写入 worktree/.focus/spec/AGENTS.md
    agentsMdPath := filepath.Join(worktreeID, ".focus", "spec", "AGENTS.md")
    os.WriteFile(agentsMdPath, []byte(spec.ToMarkdown()), 0644)
    
    // 4. 生成 handoff 上下文（如果有上一个 session）
    if handoff := m.loadLatestHandoff(taskID); handoff != nil {
        handoffPath := filepath.Join(worktreeID, ".focus", "handoff", "latest.md")
        os.WriteFile(handoffPath, []byte(handoff.ToMarkdown()), 0644)
    }
    
    return nil
}
```

**用户无感**：按 "A" 启动 Agent 时，Focus-tui 自动准备好所有上下文，Agent 启动后立即可见相关规范。

#### 4.2.4 Skill/Command 系统

Trellis 通过 `/start`, `/check`, `/finish-work` 等命令标准化 Agent 工作流。

Focus-tui 将这些命令**映射为 MCP Tools**，Agent 可以通过 MCP 调用：

```go
// 新增 MCP Tools
_ = m.mcpServer.RegisterTool("workflow.start", m.mcpWorkflowStartTool)       // /start
_ = m.mcpServer.RegisterTool("workflow.check", m.mcpWorkflowCheckTool)       // /check
_ = m.mcpServer.RegisterTool("workflow.finish", m.mcpWorkflowFinishTool)     // /finish-work
_ = m.mcpServer.RegisterTool("workflow.record", m.mcpWorkflowRecordTool)     // /record-session
_ = m.mcpServer.RegisterTool("workflow.brainstorm", m.mcpWorkflowBrainstormTool) // /brainstorm
```

**Agent 使用方式**：
```
Agent: "I need to check my work against the project specs"
→ MCP call: workflow.check({session_id: "..."})
→ Focus-tui 返回：当前 step 的验收标准 + 代码规范检查清单
→ Agent 根据返回调整实现
```

#### 4.2.5 Workspace Journal（Session 连续性）

Trellis 的 `/record-session` 将 session summary 保存到 `.trellis/workspace/{name}/journal.md`。

Focus-tui 的映射：

```
{worktree}/.focus/journal/{developer-name}.md

## 2026-04-29 Session
- Task: Implement JWT auth middleware
- Step: 2/5 Write token parser
- Duration: 45min
- Completed: Token parsing logic, basic validation
- Blockers: None
- Next: Integrate with user system
- Key Decisions: Used jwx library instead of jwt-go
```

**Hook 自动加载**：下一个 session 启动时，Focus-tui 自动将最新 journal 注入 AGENTS.md 的 "Previous Session Context" 部分。

#### 4.2.6 Sub-agent 分工（扩展）

Trellis 有 `trellis-research`, `trellis-implement`, `trellis-check` 等 sub-agent。

Focus-tui 的映射：通过 **Plan Step 的 `agent_role` 字段** 实现：

```go
type PlanStepRecord struct {
    // ... existing fields ...
    AgentRole string  // "researcher", "implementer", "checker", "reviewer"
}
```

不同 role 加载不同的 spec 层：
- `researcher` → 加载 research 方法论 spec
- `implementer` → 加载 coding standards + testing requirements
- `checker` → 加载 review checklist + acceptance criteria

### 4.3 Interactive 启动流程（完整版）

```
用户按 "A" 选择 Provider
         │
         ▼
┌─────────────────────────────┐
│ 1. Focus-tui 检查 worktree  │
│    是否已有 running agent   │
│    (HasRunning → 防止重复)  │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 2. 查询 Task/Plan/Step 上下文│
│    - task_contexts.goal     │
│    - task_plans.plan_body   │
│    - plan_steps (current)   │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 3. SpecLoader 渐进加载      │
│    - repo spec (general)    │
│    - repo spec (domain)     │
│    - worktree context       │
│    - step-specific spec     │
│    - latest handoff         │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 4. 写入 worktree/.focus/    │
│    - spec/AGENTS.md         │
│    - spec/context.md        │
│    - spec/constraints.md    │
│    - spec/step-N.md         │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 5. 构造 Env Vars            │
│    FOCUS_SESSION_ID=xxx     │
│    FOCUS_TASK_ID=xxx        │
│    FOCUS_PLAN_ID=xxx        │
│    FOCUS_STEP_ID=xxx        │
│    FOCUS_MCP_SOCKET=...     │
│    FOCUS_A2A_SOCKET=...     │
│    FOCUS_MODE=interactive   │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 6. 启动外部终端              │
│    kitty --title "Focus·wt·provider"
│    --directory {worktree}   │
│    env ... claude           │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 7. Agent 在终端中自主运行    │
│    - 自动看到 AGENTS.md     │
│    - 可通过 MCP 调用 Focus  │
│    - A2A heartbeat 每 30s   │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 8. 用户退出 Agent (Ctrl+D)  │
│    → AgentExitedMsg         │
│    → handleAgentExited      │
│    → archiveAgentSession    │
│    → SessionDigest 异步沉淀  │
│    → 写入 journal/*.md      │
└─────────────────────────────┘
```

---

## 5. Headless 模式：借鉴 Symphony 的完整形态

### 5.1 Symphony 核心形态回顾

Symphony 不是"Task 驱动"那么简单。它的核心是一个**以 Issue Tracker 为 Control Plane 的、Objective 驱动的、自主运行的 Agent Orchestrator**，包含五个层次：

| 层次 | 机制 | 作用 |
|------|------|------|
| **Control Plane** | Issue Tracker (Linear/GitHub) | 任何 open task 都应该被 agent pickup |
| **Objective 驱动** | 给 agent 目标而非 strict transitions | Agent 自主决定如何达成 |
| **DAG 执行** | 任务依赖自动拓扑排序 | 自然并行，阻塞自动等待 |
| **WORKFLOW.md** | 显式化人类隐式遵循的流程 | Agent 遵循的标准化流程 |
| **Guardrails** | CI 监控、自动 rebase、PR shepherding | 减少人类 babysitting |

Symphony 的关键洞察：

> "Treating agents as rigid nodes in a state machine doesn't work well. Models get smarter and can solve bigger problems than the box we try to fit them in. We eventually moved toward giving agents **objectives** instead of strict transitions, much like a good manager would assign a goal to a direct report."

> "The power of models comes from their ability to reason, so give them tools and context and let them cook."

### 5.2 Focus-tui Headless 的映射

#### 5.2.1 Task Board = Control Plane

Focus-tui 的 Task Board 就是 Symphony 的 Issue Tracker：

```
Task Board（Focus-tui TUI 内）
├── Open Tasks      ← Agent 自动 pickup
│   ├── #124: Refactor DB layer
│   └── #125: Add caching layer
├── In Progress     ← Agent 正在执行
│   └── #124: 🟢 Running (Codex, Turn 5/20)
├── Blocked         ← 依赖未满足
│   └── #126: React upgrade (blocked on Vite migration)
└── Done            ← 已完成
    └── #123: Implement JWT auth
```

**Orchestrator 调度逻辑**：

```go
// internal/orchestrator/scheduler.go

type Scheduler struct {
    store models.Store
    drivers map[agents.Provider]agents.AgentDriver
}

func (s *Scheduler) Tick(ctx context.Context) error {
    // 1. 获取所有 open tasks
    openTasks, _ := s.store.ListTaskContextsByState("active")
    
    for _, task := range openTasks {
        // 2. 检查是否已有 running agent
        if s.hasRunningAgent(task.ID) {
            continue
        }
        
        // 3. 检查依赖是否满足
        met, _ := s.store.AreTaskPrerequisitesMet(task.ID)
        if !met {
            continue
        }
        
        // 4. 获取当前 plan 和待执行 step
        plan, _ := s.store.GetActivePlanForTask(task.ID)
        step, _ := s.store.GetNextPendingStep(plan.ID)
        
        // 5. 确定 mode（headless vs interactive）
        mode := InferStepMode(step, task)
        
        // 6. 如果是 headless，启动 Agent
        if mode == "headless" {
            s.dispatchHeadless(ctx, task, plan, step)
        }
    }
    return nil
}
```

**调度周期**：每分钟一次（通过 Bubble Tea 的 `Tick` message 实现）。

#### 5.2.2 Objective 驱动（核心差异）

Symphony 的关键创新是**给 agent objectives 而非 strict transitions**。

Focus-tui 的实现：Plan Step 的标题不是命令，而是 objective。

```yaml
# 之前的写法（命令式）
- Step 2: Write token parser function

# Symphony 风格的写法（目标式）
- Step 2: Implement JWT token parsing that handles RS256/HS256, 
          validates claims (exp, iss, aud), and integrates with 
          the existing UserService interface.
          Acceptance: unit tests pass, benchmark < 1ms/token.
```

**Agent 自主权**：
- Agent 可以决定先写测试还是先写实现
- Agent 可以决定使用哪个库
- Agent 可以在实现过程中发现需要额外的 sub-step 并自行处理
- 只有 acceptance criteria 是强制的

#### 5.2.3 DAG 自动执行

Focus-tui 已有 `task_dependencies` 表，Symphony 风格扩展：

```sql
-- 已有表扩展：plan_steps 增加依赖关系
ALTER TABLE plan_steps ADD COLUMN depends_on TEXT;  -- JSON array of step IDs
ALTER TABLE plan_steps ADD COLUMN agent_role TEXT CHECK(agent_role IN (
    'researcher', 'implementer', 'checker', 'reviewer', 'integrator'
));
```

```go
// 拓扑排序执行
func (s *Scheduler) buildStepDAG(planID string) (*DAG, error) {
    steps, _ := s.store.ListPlanSteps(planID)
    dag := NewDAG()
    for _, step := range steps {
        dag.AddNode(step.ID, step)
        for _, depID := range step.DependsOn {
            dag.AddEdge(depID, step.ID)
        }
    }
    return dag, dag.Validate()
}

func (s *Scheduler) nextExecutableSteps(dag *DAG) []PlanStepRecord {
    // 返回所有依赖已完成的 steps
    return dag.NodesWhere(func(n *Node) bool {
        return n.Data.State == "pending" && 
               dag.AllDependenciesDone(n.ID)
    })
}
```

**并行执行**：多个无依赖关系的 step 可以同时分配给多个 headless agents（不同 worktrees）。

#### 5.2.4 WORKFLOW.md（项目级流程定义）

Symphony 将人类隐式遵循的流程显式化到 `WORKFLOW.md`。

Focus-tui 的映射：

```markdown
# repo-root/.focus/WORKFLOW.md

## Development Lifecycle

### 1. Pickup
- Read the task objective from `.focus/spec/context.md`
- Read the current plan step from `.focus/spec/step-N.md`
- Check `.focus/handoff/latest.md` for previous session context

### 2. Implement
- Run `/before-dev` to load relevant specs
- Implement the step objective
- Write tests alongside implementation
- Run `/check` to validate against project standards

### 3. Verify
- Run the test suite
- If tests fail, fix and retry (max 3 retries)
- If blocked, report blocker via MCP and pause

### 4. Handoff
- Run `/finish-work` pre-commit checklist
- Commit with descriptive message
- Update step status to "done"
- Record session summary to journal

### 5. Review
- If acceptance criteria met, mark step done
- If not, create follow-up task
```

**Agent 加载**：Headless agent 启动时，WORKFLOW.md 作为 spec 的一层自动注入。

#### 5.2.5 Guardrails（轻量版）

Symphony 的 Guardrails 包括 CI 监控、自动 rebase、flaky test retry、PR shepherding。

Focus-tui 的轻量实现：

```go
// internal/orchestrator/guardrails.go

type Guardrail struct {
    Name        string
    Check       func(session AgentSessionRecord) (bool, string)
    Action      func(session AgentSessionRecord) error
}

var DefaultGuardrails = []Guardrail{
    {
        Name: "heartbeat_timeout",
        Check: func(s AgentSessionRecord) (bool, string) {
            if s.LastHeartbeat == nil {
                return false, ""
            }
            return time.Since(*s.LastHeartbeat) > 2*time.Minute,
                "No heartbeat for 2 minutes"
        },
        Action: func(s AgentSessionRecord) error {
            // Mark session disconnected, release step
            return nil
        },
    },
    {
        Name: "step_stall",
        Check: func(s AgentSessionRecord) (bool, string) {
            if s.LastActivityAt == nil {
                return false, ""
            }
            return time.Since(*s.LastActivityAt) > 10*time.Minute,
                "No activity for 10 minutes"
        },
        Action: func(s AgentSessionRecord) error {
            // Send intervention request via MCP
            return nil
        },
    },
    {
        Name: "git_dirty_check",
        Check: func(s AgentSessionRecord) (bool, string) {
            // Check if worktree has uncommitted changes after session
            return false, ""
        },
        Action: func(s AgentSessionRecord) error {
            // Auto-commit or flag for review
            return nil
        },
    },
}
```

**扩展 Guardrails**（未来）：
- CI 状态监控（通过 git adapter 查询 CI status）
- 自动 rebase（当 base branch 有新提交时）
- PR shepherding（监控 PR review 状态，自动回复 reviewer）

#### 5.2.6 Agent 自主创建 Task

Symphony 的关键能力：Agent 发现超出当前 scope 的改进时，自动 filing 新 issue。

Focus-tui 的映射：新增 MCP Tool `task.create_followup`。

```go
_ = m.mcpServer.RegisterTool("task.create_followup", m.mcpTaskCreateFollowupTool)

func (m *model) mcpTaskCreateFollowupTool(params map[string]any) (map[string]any, error) {
    // Agent 调用此 tool 创建后续任务
    // 例如："发现性能问题，建议重构缓存层"
    // Focus-tui 在 Task Board 中创建新 task，标记为 "suggested_by_agent"
    // 人类可以评估并 schedule
}
```

### 5.3 Agent Driver 接口（完整版）

```go
// internal/agents/driver.go

// AgentDriver 是程序化驱动 Headless Agent 的抽象。
// 每个 Provider 有自己的实现（KimiDriver, CodexDriver, ClaudeDriver）。
type AgentDriver interface {
    Provider() Provider
    Start(ctx context.Context, req DriverStartRequest) (DriverSession, error)
    Capabilities() DriverCapabilities
}

type DriverStartRequest struct {
    SessionID      string
    TaskID         string
    PlanID         string
    StepID         string
    WorktreeID     string
    Objective      string           // Symphony-style: 目标而非命令
    Acceptance     string           // 验收标准
    SpecPaths      []string         // 要加载的 spec 文件路径
    WorkflowPath   string           // WORKFLOW.md 路径
    EnvVars        []string
    ApprovalMode   ApprovalMode     // auto, interactive, conservative
    MaxTurns       int              // 最大 turn 数（防止无限运行）
}

type DriverSession interface {
    Stream() <-chan AgentEvent
    Send(ctx context.Context, message string) error
    Approve(ctx context.Context, requestID string, decision ApprovalDecision) error
    Pause(ctx context.Context) error
    Stop(ctx context.Context) error
    State() SessionState
}

type AgentEvent struct {
    Type      AgentEventType
    Turn      int
    Timestamp time.Time
    Payload   any
}

type AgentEventType string

const (
    EventMessageDelta     AgentEventType = "message_delta"      // 文本流式输出
    EventToolCall         AgentEventType = "tool_call"          // 工具调用
    EventToolResult       AgentEventType = "tool_result"        // 工具结果
    EventApprovalRequest  AgentEventType = "approval_request"   // 需要审批
    EventTurnComplete     AgentEventType = "turn_complete"      // 当前 turn 完成
    EventTurnStarted      AgentEventType = "turn_started"       // 新 turn 开始
    EventStatus           AgentEventType = "status"             // 状态更新
    EventError            AgentEventType = "error"              // 错误
    EventCompact          AgentEventType = "compact"            // 上下文压缩
)
```

### 5.4 Headless 启动流程（完整版）

```
Orchestrator Scheduler Tick
         │
         ▼
┌─────────────────────────────┐
│ 1. 扫描 Open Tasks           │
│    - 过滤掉已有 running agent│
│    - 过滤掉依赖未满足的       │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 2. 获取 Task/Plan/Step       │
│    - task_contexts           │
│    - task_plans              │
│    - plan_steps (next pending)│
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 3. 确定 Mode                 │
│    - step.preferred_mode     │
│    - auto → InferStepMode()  │
│    - 如果 interactive，跳过   │
│      （等用户手动启动）        │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 4. 准备 Worktree             │
│    - SpecLoader 渐进加载     │
│    - 写入 .focus/spec/       │
│    - 加载 WORKFLOW.md        │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 5. 调用 AgentDriver.Start()  │
│    - 传入 Objective + Spec   │
│    - 返回 Stream channel     │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 6. Agent Monitor Pane 显示   │
│    - 实时流式输出            │
│    - 审批请求弹窗            │
│    - 干预输入框              │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 7. Guardrails 监控           │
│    - heartbeat timeout       │
│    - step stall              │
│    - git dirty check         │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 8. Agent 完成 / 出错 / 停止  │
│    - Stream closed           │
│    - 更新 agent_sessions     │
│    - 推进 plan step 状态     │
│    - archiveAgentSession     │
│    - 写入 journal            │
│    - 调度下一个 step         │
└─────────────────────────────┘
```

---

## 6. 两种模式的配合

### 6.1 核心配合原则

**同一 worktree 同一时间只能有一种模式。** 这不是技术限制，而是用户体验设计——避免用户困惑。

### 6.2 Mode 决策矩阵

| 场景 | Interactive | Headless | 理由 |
|------|-------------|----------|------|
| **Task 创建/Scope 定义** | ✅ | ❌ | 需要人类判断需求 |
| **Brainstorm/Research** | ⚠️ | ✅ | Headless 可以自主搜索 |
| **Implementation** | ✅ | ✅ | 根据复杂度选择 |
| **Bug Fix/Debug** | ✅ | ❌ | 需要人类判断 |
| **Refactor** | ⚠️ | ✅ | 如果测试覆盖好，可 headless |
| **Code Review** | ❌ | ✅ | 纯分析任务 |
| **Pre-commit Check** | ❌ | ✅ | 自动化流程 |

### 6.3 运行时配合流程

```
Task 创建（人类在 Focus-tui 中）
    │
    ▼
Plan 创建（人类或 Agent brainstorm）
    │
    ▼
Step 1: Define scope ──────────────▶ Interactive（人类确认）
    │                                    │
    ▼                                    ▼
Step 2: Research libs ──────────────▶ Headless（Agent 自主搜索）
    │                                    │
    ▼                                    ▼
Step 3: Implement core logic ───────▶ Headless（Agent 编码）
    │                                    │
    ▼                                    ▼
Step 4: Handle edge cases ──────────▶ Interactive（复杂判断）
    │                                    │
    ▼                                    ▼
Step 5: Write tests ────────────────▶ Headless（Agent 写测试）
    │                                    │
    ▼                                    ▼
Step 6: Run checks ─────────────────▶ Headless（自动化检查）
    │                                    │
    ▼                                    ▼
All steps done ─────────────────────▶ SessionDigest 沉淀
    │
    ▼
Human Review → Merge
```

### 6.4 模式切换（有限）

不是同一个 session 切换，而是**同一个 task 的不同 session**：

```
Interactive Session 结束
    │
    ▼
Focus-tui 提示："还有 3 个 steps 待执行，建议启动 Headless 模式继续？"
    │
    ├── [Yes] → 保存当前上下文 → 为下一个 pending step 启动 Headless
    │
    └── [No] → 保持当前状态，用户手动继续

Headless Session 遇到复杂场景
    │
    ▼
连续 3 次 approval_request 或 stall detected
    │
    ▼
Focus-tui 提示："Agent 多次需要干预，建议切换到 Interactive 模式？"
    │
    ├── [Switch] → Stop headless → 标记 step 为 blocked → 提示用户启动 Interactive
    │
    └── [Continue] → 保持 headless，持续审批弹窗
```

### 6.5 共享基础设施详细设计

#### MCP Server（两种模式共用）

| Tool | Interactive | Headless | 说明 |
|------|-------------|----------|------|
| `session.heartbeat` | ✅ | ✅ | 统一心跳（已有） |
| `session.request_intervention` | ✅ | ❌ | Interactive 专用（已有） |
| `session.request_approval` | ❌ | ✅ | Headless 审批请求（新增） |
| `session.report_turn` | ❌ | ✅ | Headless turn 完成报告（新增） |
| `task.get` | ✅ | ✅ | 查询 task 上下文（已有） |
| `task.create_output` | ✅ | ✅ | 创建输出（已有） |
| `task.update_status` | ✅ | ✅ | 更新状态（已有） |
| `task.create_followup` | ✅ | ✅ | 创建后续任务（新增） |
| `kg.add_fact` | ✅ | ✅ | 知识图谱（已有） |
| `context.get_for_task` | ✅ | ✅ | 获取上下文（已有） |
| `workflow.start` | ✅ | ✅ | /start 命令（新增） |
| `workflow.check` | ✅ | ✅ | /check 命令（新增） |
| `workflow.finish` | ✅ | ✅ | /finish-work 命令（新增） |
| `workflow.record` | ✅ | ✅ | /record-session 命令（新增） |

#### A2A Router（两种模式共用）

Headless Agent 通过 A2A 发送实时事件（补充 SDK channel）：

```go
// Headless Agent SDK 内部
a2aRouter.Publish(a2a.Message{
    Type: "agent.event",
    From: sessionID,
    To:   "orchestrator",
    Payload: map[string]any{
        "event_type": "message_delta",
        "turn":       turnNum,
        "delta":      text,
    },
})
```

这样即使 SDK channel 出问题，A2A 作为 fallback 保证事件可达。

### 6.6 Worktree 资源隔离

```
Worktree 状态机:

                    ┌──────────────┐
                    │   空闲       │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌─────────┐  ┌─────────┐  ┌─────────┐
        │Interactive│  │ Headless│  │ 冲突    │
        │ 运行中   │  │ 运行中  │  │ (禁止)  │
        └────┬────┘  └────┬────┘  └─────────┘
             │            │
             └────────────┘
                    │
                    ▼
            ┌──────────────┐
            │   沉淀中     │
            │ (SessionDigest)│
            └──────┬───────┘
                   │
                   ▼
            ┌──────────────┐
            │   空闲       │
            └──────────────┘

规则:
1. 同一个 worktree 不能同时运行 Interactive 和 Headless
2. Headless 启动前检查 worktree 是否有 running interactive session
3. Interactive 启动前检查 worktree 是否有 running headless session
4. 冲突时提示："该 worktree 已有 running agent，请先停止"
```

---

## 7. 数据模型变更

### 7.1 Schema 变更

```sql
-- agent_sessions 新增 mode 字段
ALTER TABLE agent_sessions ADD COLUMN mode TEXT 
    CHECK(mode IN ('interactive', 'headless')) DEFAULT 'interactive';

-- agent_sessions 新增 objective 字段（Symphony-style）
ALTER TABLE agent_sessions ADD COLUMN objective TEXT;
ALTER TABLE agent_sessions ADD COLUMN acceptance TEXT;

-- plan_steps 新增 preferred_mode 字段
ALTER TABLE plan_steps ADD COLUMN preferred_mode TEXT 
    CHECK(preferred_mode IN ('interactive', 'headless', 'auto')) DEFAULT 'auto';

-- plan_steps 新增 agent_role 字段
ALTER TABLE plan_steps ADD COLUMN agent_role TEXT 
    CHECK(agent_role IN ('researcher', 'implementer', 'checker', 'reviewer', 'integrator'));

-- plan_steps 新增 depends_on 字段（JSON array）
ALTER TABLE plan_steps ADD COLUMN depends_on TEXT;  -- JSON: ["step-id-1", "step-id-2"]

-- agent_events（NEW）- Headless 事件流缓存
CREATE TABLE IF NOT EXISTS agent_events (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    turn        INTEGER NOT NULL DEFAULT 0,
    event_type  TEXT NOT NULL CHECK(event_type IN (
        'message_delta', 'tool_call', 'tool_result', 'approval_request',
        'turn_complete', 'turn_started', 'status', 'error', 'compact'
    )),
    payload     TEXT NOT NULL,  -- JSON
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_agent_events_session ON agent_events(session_id, turn, created_at);

-- agent_approvals（NEW）- 审批请求队列
CREATE TABLE IF NOT EXISTS agent_approvals (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    request_id  TEXT NOT NULL,  -- provider-specific ID
    tool_name   TEXT,
    description TEXT NOT NULL,
    decision    TEXT CHECK(decision IN ('pending', 'allowed', 'denied', 'allowed_session')),
    decided_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_agent_approvals_session ON agent_approvals(session_id, decision);

-- task_contexts 新增 orchestration 字段
ALTER TABLE task_contexts ADD COLUMN orchestration_mode TEXT 
    CHECK(orchestration_mode IN ('manual', 'auto')) DEFAULT 'manual';
```

### 7.2 Go 类型变更

```go
// internal/agents/mode.go

type SessionMode string

const (
    ModeInteractive SessionMode = "interactive"
    ModeHeadless    SessionMode = "headless"
)

// internal/agents/session.go（扩展）
type Session struct {
    ID             string
    Provider       Provider
    Mode           SessionMode       // NEW
    WorktreeID     string
    RepoID         string
    TaskID         string
    PlanID         string
    StepID         string
    BranchSnapshot string
    PID            int
    State          SessionState
    LaunchSource   string
    Summary        string
    Objective      string            // NEW: Symphony-style objective
    Acceptance     string            // NEW: acceptance criteria
    EnvSnapshot    string
    StartedAt      time.Time
    EndedAt        *time.Time
    LastActivityAt *time.Time
    UpdatedAt      time.Time
    
    // Headless-only fields（不在 DB 中持久化）
    DriverSession  DriverSession     // nil for interactive
    EventBuffer    []AgentEvent      // headless event cache
}

// internal/models/plan_step.go（扩展）
type PlanStepRecord struct {
    ID             string
    PlanID         string
    OrderIndex     int
    Title          string
    State          string
    ExpandedTaskID string
    Notes          string
    PreferredMode  string   // NEW: "interactive", "headless", "auto"
    AgentRole      string   // NEW: "researcher", "implementer", "checker", "reviewer", "integrator"
    DependsOn      string   // NEW: JSON array of step IDs
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// internal/models/task_context.go（扩展）
type TaskContextRecord struct {
    // ... existing fields ...
    OrchestrationMode string  // NEW: "manual", "auto"
}
```

---

## 8. UI 设计

### 8.1 Worktree Page（Interactive 模式）

```
┌────────────────────────────────────────────────────────────┐
│ repo/main  │  Task #123: Implement auth  │  🍅 25:00       │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌──────────────────────────┐  ┌────────────────────────┐ │
│  │ 🤖 Claude · 🟢 Running    │  │ Shell · main           │ │
│  │ Turn 3/20 · 14:32-now    │  │ $ git status            │ │
│  │ Mode: Interactive        │  │ M  auth.go              │ │
│  │                          │  │ ?? jwt_parser.go        │ │
│  │ [查看外部终端]            │  │ $                       │ │
│  │ [聚焦会话]                │  │                         │ │
│  │                          │  │                         │ │
│  └──────────────────────────┘  └────────────────────────┘ │
│                                                            │
│  Step: 2/5 Write token parser  │  [⏭ Next Step]           │
│  Plan: Implement JWT auth middleware                       │
│                                                            │
│  Context: .focus/spec/AGENTS.md (auto-injected)            │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### 8.2 Agent Monitor Pane（Headless 模式）

```
┌────────────────────────────────────────────────────────────┐
│ 🤖 Agent Monitor  │  Task #124  │  ⏸  │  🛑  │  📋       │
├────────────────────────────────────────────────────────────┤
│                                                            │
│ Codex · 🟢 Running · Turn 5/20 · 14:32-now                 │
│ Mode: Headless  │  Step: 3/5 Migrate user table schema      │
│ Objective: Implement zero-downtime migration...            │
│                                                            │
│ ┌────────────────────────────────────────────────────────┐ │
│ │ [14:32] 💭 Analyzing schema changes...                 │ │
│ │ [14:33] 🔧 readFile("models/user.go")                  │ │
│ │ [14:33] 🔧 writeFile("migrations/003.sql")             │ │
│ │ [14:34] 🔧 exec("go test ./models") → ✅ 12/12         │ │
│ │ [14:35] ✍️  Writing migration script...                │ │
│ │                                                        │ │
│ │ ⚠️  Pending Approval:                                  │ │
│ │ Command: git push origin feat-db-migration             │ │
│ │ [Allow] [Allow Session] [Deny] [View]                  │ │
│ │                                                        │ │
│ │ > Use interface for DB abstraction                     │ │
│ │ [Send]                                                 │ │
│ └────────────────────────────────────────────────────────┘ │
│                                                            │
│ Tokens: 12.3K/20K  Cost: ~$0.42  Speed: 45 tok/s          │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### 8.3 Task Board（Orchestrator 视图）

```
┌────────────────────────────────────────────────────────────┐
│ Task Board                              [+ New Task]       │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Open                        In Progress                   │
│  ─────                       ───────────                   │
│  #123 Impl JWT auth          #124 Refactor DB 🟢 Codex     │
│  #126 Add caching            #125 Fix race  🟡 Kimi       │
│                                                              │
│  Blocked                     Done                          │
│  ────────                    ────                          │
│  #127 React upgrade          #120 Auth tests              │
│     (on #125)                #121 DB index                │
│                                                              │
│  [Auto-dispatch: ON]  [🔄 Refresh]  [⚙️ Settings]          │
│                                                            │
│  Mode: int=interactive  hd=headless  auto=auto-select      │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

---

## 9. 实施路线图

> **核心判断：Interactive 是主力开发方式，Headless 是架构补充。**
> 优先把 Interactive 的 Trellis 化做深做透，Headless 保留架构设计但优先级降低。

### Phase 1：基础设施完善（当前）

| 任务 | 状态 | 说明 |
|------|------|------|
| SessionDigest MVP | ✅ 完成 | digest + event + finder + builder + store |
| Kimi reader | 🔄 待实现 | `~/.kimi/sessions/<md5>/<id>/context.jsonl` |
| Compact summary | 🔄 待实现 | 从 session 文件提取 compact |
| Merge latest master | ✅ 完成 | 已合并 provider picker + external terminal |

### Phase 2：Interactive 模式 Trellis 化（核心，最高优先级）

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Per-worktree `.focus/` 配置空间 | **P0** | `{worktree}/.focus/spec/`, `handoff/`, `journal/` |
| SpecLoader 渐进加载 | **P0** | 分层：repo → domain → worktree → step → handoff |
| AGENTS.md 自动生成 | **P0** | 启动 agent 前自动写入 |
| Hook 自动注入 | **P0** | Agent 启动时自动加载上下文（零用户操作） |
| Workspace Journal | **P1** | `{worktree}/.focus/journal/{dev}.md` |
| MCP workflow tools | **P1** | `workflow.start`, `check`, `finish`, `record` |
| Agent Role 系统 | **P2** | `plan_steps.agent_role` |
| 外部终端标题统一 | **P2** | `Focus · {worktree} · {provider}` |

### Phase 3：Headless 模式 Symphony 化（保留架构，按需实现）

| 任务 | 优先级 | 说明 |
|------|--------|------|
| AgentDriver 接口 | P1 | `Start` / `Stream` / `Send` / `Approve` / `Stop` |
| KimiDriver | P1 | 官方 SDK 实现（验证架构可行性） |
| Agent Monitor Pane | P1 | 实时流式 + 审批 UI |
| CodexDriver | P2 | `codex app-server` JSON-RPC |
| ClaudeDriver | P3 | 社区 SDK（需 ANTHROPIC_API_KEY）|
| Orchestrator Scheduler | P2 | Tick-based task dispatch |
| DAG 执行 | P2 | `plan_steps.depends_on` + 拓扑排序 |
| Objective 驱动 | P2 | Step title → Objective + Acceptance |
| WORKFLOW.md | P3 | 项目级流程定义 |
| Guardrails | P3 | heartbeat timeout, stall detection |
| Agent 自主创建 task | P3 | `task.create_followup` MCP tool |

### Phase 4：Orchestrator 与配合

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Step-Mode 绑定 | P2 | `plan_steps.preferred_mode` |
| Mode Router | P2 | Orchestrator 根据 mode 路由 |
| 运行时模式建议 | P3 | interactive 结束建议 headless 继续 |
| Multi-agent Teams | P3 | 多 worktree 并行 headless |

### Phase 5：沉淀与闭环

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Headless event 沉淀 | P2 | 将 headless 事件流纳入 SessionDigest |
| 统一报告生成 | P1 | Interactive + Headless 统一 Markdown |
| Cost tracking | P2 | Token 使用统计 |
| Performance metrics | P3 | 各 provider 成功率、耗时对比 |

---

## 10. 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| **Takeover** | ❌ 不实现 | 同一个 session 不能从 headless 切 interactive（技术不可行） |
| **同一 session 切换** | ❌ 不支持 | SDK 和 CLI 不能同时持有 session |
| **Headless 审批** | ✅ Focus-tui UI | 替代 CLI TUI 弹窗，统一体验 |
| **首选 Headless Provider** | 🥇 Kimi | 官方 Go SDK，最成熟 |
| **Auto Mode 默认** | 🥇 Interactive | 更安全，用户逐步信任后切 headless |
| **Event 流持久化** | ✅ SQLite | 便于事后分析和审计 |
| **A2A 用于 Headless** | ✅ Fallback | SDK channel 为主，A2A 为 backup |
| **Per-worktree 配置** | ✅ `.focus/` | Trellis 是项目级，Focus-tui 可以更细粒度 |
| **Objective vs Command** | ✅ Objective | Symphony 洞察：给目标而非命令 |
| **Orchestration** | ✅ Focus-tui 内置 | 不需要 Linear，Task Board 就是 Control Plane |

---

## 11. 与现有代码的兼容性

### 11.1 向后兼容

- `agent_sessions` 新增 `mode` 字段，默认 `'interactive'` → 现有记录不受影响
- `plan_steps` 新增 `preferred_mode`/`agent_role`/`depends_on`，默认 `NULL` → 现有记录不受影响
- Interactive 模式的启动流程完全保留，新增 SpecLoader 是**增强**而非**替换**

### 11.2 最小侵入原则

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/agents/types.go` | 扩展 | 新增 `SessionMode`, `Objective`, `Acceptance` |
| `internal/agents/launcher.go` | 增强 | 启动前调用 `prepareWorktreeAgentProfile` |
| `internal/agents/` | 新增 | `driver.go`, `spec_loader.go`, `drivers/` |
| `internal/app/app.go` | 增强 | 新增 `AgentMonitorPane`, `OrchestratorTick` |
| `internal/app/` | 新增 | `agent_monitor_pane.go` |
| `internal/orchestrator/` | 新增 | `scheduler.go`, `guardrails.go` |
| `internal/store/db.go` | 扩展 | 新增表 + 字段 |
| `internal/models/` | 扩展 | 新增字段 |
| `internal/mcp/server.go` | 增强 | 新增 workflow tools |
| `internal/sessiondigest/` | 增强 | 支持 headless event 沉淀 |

---

## 12. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Kimi SDK 不稳定 | Headless 无法运行 | 先实现 KimiDriver，同时调研 CodexDriver 作为 backup |
| SpecLoader 过度复杂 | 配置难以维护 | 从简单两层（repo + worktree）开始，逐步增加层级 |
| Headless 审批延迟 | Agent 等待人类响应 | 设置审批超时（5分钟），超时自动 deny + 标记 blocked |
| Worktree 配置污染 | `.focus/` 污染 git | `.focus/` 默认加入 `.gitignore`，可选提交 |
| Multi-agent 资源竞争 | 多个 headless agent 抢资源 | 限制同时运行的 headless agents 数量（默认 2）|
| Objective 模糊导致 Agent 偏离 | 实现不符合预期 | 保留 Acceptance Criteria 作为硬性检查 |

---

*End of ADR*
