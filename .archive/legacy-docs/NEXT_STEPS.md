# Next Steps

> [过时归档说明]
> 自 2026-04-19 起，本文档不再作为当前 session 的执行入口。
> 项目已切换到 **ADR-first** 工作流；后续的执行入口应来自 ADR 宪法与其下位 plan，而不是本文件中的 phase/step 描述。

> 更新日期: 2026-04-17
> 本页只保留“当前真实状态 + 当前执行入口”

---

## 当前状态

- **当前阶段**: Phase 4 - Human Context Recovery + Lightweight Task Orchestration
- **当前执行点**: Step 1 - Context Schema & Store Foundation
- **说明**: Phase 3 的 worktree/page/snapshot/agent 基础已落地，当前主线切换为 task/worktree context 建模与 resume-first overview

---

## 当前应读文档

| 文档 | 用途 | 优先级 |
|------|------|--------|
| **[docs/adr/0000-constitution-for-adr-driven-execution.md](./docs/adr/0000-constitution-for-adr-driven-execution.md)** | 当前最高层宪法约束 | ⭐⭐⭐ 首读 |
| **[docs/adr/0001-product-positioning-human-sovereign-agent-native-workbench.md](./docs/adr/0001-product-positioning-human-sovereign-agent-native-workbench.md)** | 当前产品定位 | ⭐⭐⭐ 必读 |
| **[IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md)** | 历史实现计划参考 | ⭐ 背景 |
| **[DIRECTION.md](./DIRECTION.md)** | ADR-first 转型前方向历史 | ⭐ 背景 |
| **[DESIGN-WORKTREE-CONTAINERS-2026-04.md](./DESIGN-WORKTREE-CONTAINERS-2026-04.md)** | Worktree 容器工程设计 | ⭐⭐ 设计参考 |
| **[PLAN-2.4.md](./PLAN-2.4.md)** | 历史基础设施计划 | 历史参考 |
| **[LEARNINGS.md](./LEARNINGS.md)** | 历史探索笔记 | 参考 |

---

## 现在就做什么

### 1. 先做 Context Schema & Store Foundation

- `task_contexts`
- `worktree_contexts`
- `task_worktree_links`
- `context_notes`
- 扩展 `agent_sessions`

### 2. 再做 Resume Summary Pipeline

- 构建 overview resume summary
- 引入 task/worktree 摘要排序
- 准备 resume-first UI 数据源

### 3. 然后继续 Resume-First Overview UX

- `Enter = resume`
- task-in-worktree summary
- next step / recent signal / agent signal

---

## 暂不做，但已记录

- hunk 级 Git 操作深化
- branch-aware Git actions
- editor / review 进一步打磨
- canvas compositor 稳定化
- YAML layout 配置化
- multi-agent orchestration
- transcript / event sourcing / timeline

---

## 开发原则

- `go test ./...` 是每次提交前的固定动作
- 保持向后兼容（无配置时行为不变）
- 渐进式演进，不做大规模重构
- 所有新增代码需有测试覆盖
- 文档必须定期清理，避免再次漂移

---

## 当前执行入口

如果你是新 session，从这里开始：

1. 先读 **[IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md)**
2. 再读 **[PLAN-WORKTREE-CONTAINERS-2026-04.md](./PLAN-WORKTREE-CONTAINERS-2026-04.md)**
3. 然后直接从 **Phase 4 Step 1** 开始
