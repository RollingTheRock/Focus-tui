# ADR-0006: DAG Pane Phase-Step 分层与 Agent 粒度约束

- **Date:** 2026-05-29
- **Status:** Accepted

---

## Context

### 1. 当前 DAG Pane 的核心问题

在高强度开发实践中，DAG pane 的任务设计暴露出两个结构性矛盾：

**问题 1：任务粒度与人类认知能力不匹配**

Agent 制定架构计划时，倾向于输出大量细粒度执行步骤（如"改函数签名"、"写单元测试"、"更新配置"）。当前 `plan.expand_to_tasks` 将这些步骤全部展开为平级 `task_contexts`，人类在 DAG pane 中同时面对 20-30 个节点。由于人类无法在任务制定途中理解每一个技术决策的具体位置，只能被动接受（点 yes），人的作用被严重削弱。

**问题 2：任务与工作树几乎 1:1 映射**

`task_contexts.preferred_worktree_id` 的设计使得每个任务都可能触发工作树创建。30 个任务意味着 30 个工作树，即使可以并行开发，管理成本也远超收益。既然人类不清楚每个任务具体怎么做，人就无法判断"这个任务是否值得一个独立工作树"。

**问题 3：DAG pane 缺乏任务清理能力**

当前 DAG pane 没有任何删除单个任务或清空面板的快捷键。任务只能通过 `s` 键循环状态（active→paused→blocked→done），即使标记为 done 的任务仍然占据视觉空间。这导致 DAG 只能增长不能收缩，长期累积后成为噪音。

**问题 4：Agent 缺乏粒度约束机制**

Agent 通过 MCP 工具与系统交互，但当前协议没有区分"架构级任务"和"执行级任务"。Agent 的 `task.create` 和 `dag.get_status` 默认暴露完整平级视图，Agent 没有结构性动机去收敛到架构师可理解的粒度。

### 2. 已有但未使用的数据模型伏笔

`task_contexts` 表已经包含 `parent_task_id` 字段（`ON DELETE SET NULL`），但 `dagPane.buildDAG()` 完全未处理父子关系。`worktree_contexts` 表的 `task_mode` 字段已预留 `single` / `mixed` / `staging` 三种值，但 DAG pane 交互层从未使用。这意味着两层结构的存储基础已经存在，只需要在交互层和 MCP 协议层激活。

### 3. 关键洞察

> **Agent 能看到全图，但人类只能看轮廓。问题不是让 Agent 变笨，而是让 Agent 知道"人类能看到什么"。**
>
> 当 Agent 意识到它创建的每一个没有 parent 的任务都会直接出现在人类的 DAG 面板上时，它会自觉收敛到架构粒度。

---

## Decision

### D1. 两层任务模型：Phase（架构层）+ Step（执行层）

复用已有的 `parent_task_id` 字段，不做任何表结构迁移：

- **Phase（粗任务）**：`parent_task_id IS NULL`，代表架构师可理解的大方向阶段（如"引入 Redis 缓存层"、"拆分 Order 服务"）。Phase 对人类可见，一个 Phase 绑定一个工作树。
- **Step（细任务）**：`parent_task_id IS NOT NULL`，代表具体执行动作（如"初始化 Redis 连接池"、"封装 CacheRepository 接口"）。Step 对人类透明，Agent 在 worktree 内根据 Step DAG 自调度。

Phase 之间的依赖关系**自动推导**：若存在 Step-a（属于 Phase-A）依赖 Step-b（属于 Phase-B），则 Phase-A 依赖 Phase-B。人类不需要手动维护 Phase 级别的边。

### D2. DAG pane 主视图只渲染 Phase

`dagPane.buildDAG()` 过滤掉 `parent_task_id != ""` 的任务，主视图只显示 Phase 节点。Phase 状态用现有颜色编码（active / paused / blocked / done），**不显示 Step 完成进度**——Agent 完成 Step 后不会自动回填状态，进度数字是幻觉，人类也不关心这个层次的进度。

### D3. MCP 协议分层暴露

通过上下文设计约束 Agent 行为，最小改动 MCP 协议：

- **`task.create`**：增加可选参数 `parent_task_id`。不传 = Phase，传了 = Step。
- **`task.list`**：默认只返回 `parent_task_id IS NULL` 的 Phase。增加 `include_subtasks: true` 参数，Agent 深入规划时显式传参获取完整列表。
- **`dag.get_status`**：默认返回 Phase 级别 DAG。增加 `detail: "full"` 参数返回完整 DAG（含 Step）。

**不做标题硬校验，不增加 `granularity` 字段。** Agent 的约束主要来自上下文（见 D5），而非服务端规则。

### D4. Plan Expansion 只到 Phase 层

当前 `plan.expand_to_tasks` 将 plan steps 全部展开为平级 task（问题的根源）。修改后：

1. `plan.expand_to_tasks` 每个 plan step 创建一个 **Phase**（`parent_task_id = NULL`）。
2. Agent 拿到 Phase 列表后，用 `task.create(parent_task_id=phase_id)` 填充 Step。
3. 人类在 DAG pane 看到 5-10 个 Phase，审查顺序和依赖是否合理。
4. 人类选中 Phase 按 `Enter` 进入 worktree，Agent 在 worktree 内根据 Step DAG 自行调度。

**分界清晰**：Plan → Phase（人审）→ Step（Agent 填）。

### D5. Agent 上下文约束

Agent 的 AGENTS.md / handoff context 中固定写入分层约定：

```
Focus 的 DAG 面板是架构师的规划视图，只显示 parent_task_id 为空的任务。

你制定计划时：
1. 先创建 5-10 个 Phase（不传 parent_task_id），代表架构级阶段
2. Phase 的标题是架构师能判断的大方向，不是执行指令
3. 需要细化时，创建 Step 并指定 parent_task_id，Step 对人类不可见
4. 你创建的任何没有 parent 的任务都会直接出现在人类的 DAG 面板上
```

这不是提示词建议，而是 Agent 对自身输出可见性的认知约束。Agent 知道裸任务会出现在人的面板上，它会自觉收敛到架构粒度。

### D6. DAG pane 交互补全

在**不改动现有快捷键**的前提下，增加三个操作：

| 按键 | 行为 |
|---|---|
| `d` | 删除当前选中的 Phase（连带删除其所有 Step）。弹出确认 overlay。 |
| `D` | 清空整个 DAG 面板（弹出确认，可选按状态过滤清除）。 |
| `z` | 弹出 mini-dag overlay，显示当前 Phase 内部的 Step 结构。只读，按 `esc` / `q` 关闭。 |

mini-dag overlay 用缩进树形展示 Step 及其状态（如 `├── 初始化连接池 [done]`），**纯展示，不做任何交互**。人类看一眼了解 Agent 在这个 Phase 内的细化思路即可。

### D7. Worktree 绑定规则

- Phase 通过 `preferred_worktree_id` 绑定工作树，Phase 创建/进入时 `task_mode = 'mixed'`。
- Step 不直接绑定工作树，通过 `task_worktree_links(relation_type='secondary')` 继承所属 Phase 的 worktree。
- 一个 Phase = 一个工作树，解决 "30 任务 = 30 工作树" 的问题。

---

## Considered Alternatives

### Alternative A — 硬编码标题规则约束 Agent

在 `task.create` 服务端对标题做启发式校验（如包含"写"、"改"、"测"等动词时强制要求 `parent_task_id`）。

**Rejected。** 规则难以覆盖所有情况，误判成本高，维护负担重。Agent 的架构能力足以通过上下文自觉收敛，不需要服务端 babysitting。

### Alternative B — 在 Phase 节点显示 Step 完成进度

在 DAG pane 的 Phase 节点上显示 `x/y` 完成度（如 `2/5`）。

**Rejected。** Agent 完成 Step 后不会自动回填状态，进度数字是不可靠的幻觉。人类在 Phase 层面也不关心细粒度进度，Phase 的状态（active / paused / blocked / done）已足够表达宏观进展。

### Alternative C — 取消 DAG 改为列表视图

将 DAG pane 改为简单的任务列表，降低认知负担。

**Rejected。** DAG 的依赖可视化是 Focus-tui 的核心价值之一，问题在于粒度而非形式。保留 DAG 的结构表达力，只过滤掉不应在这个层面出现的节点。

### Alternative D — 新增独立的 `subtask` 表

为 Step 创建独立的 `subtasks` 表，与 `task_contexts` 分离。

**Rejected。** 已有的 `parent_task_id` 字段和 `ON DELETE CASCADE` 完全满足需求。新表增加复杂度，且 `task_dependencies` 表已经支持跨表边（实际上在同一张表内）。

---

## Consequences

### Positive

- 人类从"审批 30 个不懂的任务"变为"理解 5-10 个 Phase 的架构并决策优先级"，人的作用从被动点 yes 恢复为主动架构审查。
- 工作树数量从"约等于任务数"降至"约等于 Phase 数"，管理成本大幅下降。
- DAG pane 视觉噪音减少，架构依赖关系更清晰。
- Agent 获得明确的粒度边界：Phase 是人与 Agent 的契约，Step 是 Agent 的内部调度单元。
- 复用已有数据模型（`parent_task_id`、`task_mode`），零迁移成本。

### Negative

- Agent 需要两轮调用才能建立完整计划（先创建 Phase，再创建 Step），plan expansion 的交互变长。
- Phase 依赖的自动推导逻辑增加渲染层复杂度。
- 如果 Agent 不遵守上下文约束，仍可能创建无 parent 的细粒度 Phase 任务，需要人类手动删除或合并。
- mini-dag overlay 的 Step 状态可能因 Agent 不回填而长期停留在 `blocked`，overlay 的信息价值有限。

---

## Relationship to ADR-0001

ADR-0001 确立 "Human-sovereign, Agent-native" 原则——Agent 可以 proposing 和 executing，但人类拥有 final convergence authority。本 ADR 是这一原则在 DAG pane 任务粒度层面的具体落实：Agent 拥有完整 DAG 的可见性和 Step 的调度权，但人类只在 Phase 层面做收敛决策。

## Relationship to ADR-0004

ADR-0004 确立 Protocol-Driven Multi-Agent Orchestration，Agent 通过 MCP 工具读写共享状态。本 ADR 扩展了 MCP 协议的任务暴露策略（`task.list` 默认过滤、`dag.get_status` 分层），使协议本身成为粒度约束的载体。
