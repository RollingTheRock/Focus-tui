# Next Steps

> 本文档已整合到新的文档体系中
> 更新日期: 2026-04-10

---

## 文档导航

| 文档 | 用途 | 优先级 |
|------|------|--------|
| **[DIRECTION.md](./DIRECTION.md)** | 总体技术方向与架构规划 | ⭐⭐⭐ 必读 |
| **[PLAN-2.4.md](./PLAN-2.4.md)** | Phase 2.4详细开发计划 | ⭐⭐⭐ 必读 |
| **[LEARNINGS.md](./LEARNINGS.md)** | 源码探索总结 | ⭐⭐ 参考 |
| **[LEARNINGS-2026-04-03.md](./LEARNINGS-2026-04-03.md)** | 历史探索（早期dashboard阶段） | 归档 |

---

## 当前状态

- **阶段**: Phase 2.4 - Git/Tree集成准备
- **目标**: 建立Plugin+Adapter架构，实现首个GitStatus Plugin Pane
- **周期**: 4周（详见PLAN-2.4.md）

---

## 立即开始

1. 阅读 **[DIRECTION.md](./DIRECTION.md)** 理解总体方向
2. 阅读 **[PLAN-2.4.md](./PLAN-2.4.md)** 了解详细任务分解
3. 从 **Week 1** 任务开始：创建Plugin系统骨架

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

**下一步**: 开始Phase 2.4 Week 1任务 - 创建Plugin系统骨架
