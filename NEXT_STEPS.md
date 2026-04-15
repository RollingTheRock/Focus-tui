# Next Steps

> 本文档已整合到新的文档体系中
> 更新日期: 2026-04-15

---

## 文档导航

| 文档 | 用途 | 优先级 |
|------|------|--------|
| **[DIRECTION.md](./DIRECTION.md)** | 总体技术方向与架构规划 | ⭐⭐⭐ 必读 |
| **[DESIGN-WORKTREE-CONTAINERS-2026-04.md](./DESIGN-WORKTREE-CONTAINERS-2026-04.md)** | Worktree 容器工程设计 | ⭐⭐⭐ 必读 |
| **[PLAN-WORKTREE-CONTAINERS-2026-04.md](./PLAN-WORKTREE-CONTAINERS-2026-04.md)** | 当前阶段实施计划 | ⭐⭐⭐ 必读 |
| **[PLAN-2.4.md](./PLAN-2.4.md)** | Phase 2.4历史基础设施计划 | ⭐⭐ 历史参考 |
| **[LEARNINGS.md](./LEARNINGS.md)** | 源码探索总结 | ⭐⭐ 参考 |
| **[LEARNINGS-2026-04-03.md](./LEARNINGS-2026-04-03.md)** | 历史探索（早期dashboard阶段） | 归档 |

---

## 当前状态

- **阶段**: Phase 3B - Page-based Worktree Containers
- **目标**: 将顶层 UI 升级为 overview page + per-worktree workspace page
- **周期**: 4-6周（详见 PLAN-WORKTREE-CONTAINERS-2026-04.md）

---

## 立即开始

1. 阅读 **[DIRECTION.md](./DIRECTION.md)** 理解总体方向
2. 阅读 **[DESIGN-WORKTREE-CONTAINERS-2026-04.md](./DESIGN-WORKTREE-CONTAINERS-2026-04.md)** 理解 worktree 容器设计
3. 阅读 **[PLAN-WORKTREE-CONTAINERS-2026-04.md](./PLAN-WORKTREE-CONTAINERS-2026-04.md)** 了解详细任务分解
4. 从 **Phase B4** 任务开始：为 overview/worktree page 引入顶层 page model skeleton

---

## 开发原则

- `go test ./...` 是每次提交前的固定动作
- 保持向后兼容（无配置时行为不变）
- 渐进式演进，不做大规模重构
- 所有新增代码需有测试覆盖

---

## 关键参考代码

```
/mnt/d/dev/dev-learn/lazygit/pkg/gui/gui.go
/mnt/d/dev/dev-learn/lazygit/pkg/gui/controllers/helpers/refresh_helper.go
/mnt/d/dev/dev-learn/sidecar/internal/plugin/plugin.go
/mnt/d/dev/dev-learn/sidecar/internal/adapter/adapter.go
/mnt/d/dev/dev-learn/sidecar/internal/plugins/gitstatus/tree.go
```

---

**下一步**: 开始 Phase B4 - 引入 overview page / worktree page 骨架，并逐步把全局 bodyTree 迁移为 page-local state
