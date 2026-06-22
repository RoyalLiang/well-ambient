# Jira Task 执行观测 MVP

## 目标

Jira `Task` 和 `Bug` 不应该自动进入需求池排期。它们更像需求落地过程中的执行证据：谁在做、有没有分支、有没有 commit、MR 是否合并、Jira 状态是否回写、最终结果能不能回流到父级需求。

执行观测 MVP 用来把这些执行任务放进独立视图，帮助 PM、研发负责人和需求负责人判断：

- Jira Task 是否绑定到一个真实需求。
- 任务是否已经产生代码证据。
- MR 合并、Jira 状态、需求排期之间是否出现断层。
- 哪些完成任务缺少证据，哪些活跃任务已经停滞。

这套链路的边界是：需求池和排期表只承载 `issue_type = demand` 的主需求；`task` 和 `bug` 进入执行追踪，不直接污染需求排期。

## 数据边界

执行观测复用现有 `task_telemetries` 和 `git_commit_logs`。

- `task_telemetries.issue_type = demand`：作为父级需求，只用于通过 `task_group_id` 建立绑定关系。
- `task_telemetries.issue_type in (task, bug)`：作为执行任务，进入执行追踪视图。
- `git_commit_logs`：作为执行证据，按 `task_id` 汇总 commit、MR、merge 事件。
- 用户表：用于补充负责人部门，让问题能按组织维度排查。

Jira 同步器继续把无法识别为需求的普通 Jira 任务同步为 `task`，缺陷同步为 `bug`。是否能排期不由 Jira 类型决定，而由是否被建模为需求决定。

## 接口契约

### `GET /api/execution/tasks`

权限：`dashboard:read`

响应：

```json
{
  "generated_at": "2026-06-22 18:30",
  "summary": {
    "total": 5,
    "active": 3,
    "done": 2,
    "bound": 1,
    "orphan": 4,
    "with_evidence": 3,
    "missing_evidence": 2,
    "stale": 1,
    "mismatch": 1,
    "high_risk": 2
  },
  "items": [
    {
      "task_id": "JIRA-101",
      "title": "Implement department refresh",
      "issue_type": "task",
      "assignee": "Eddie",
      "department": "Engineering",
      "repo": "well-ambient",
      "branch": "feat/dept-refresh",
      "status": "progress",
      "task_group_id": "demand-001",
      "parent_demand_id": "DEMAND-001",
      "parent_demand": "部门刷新",
      "created_at": "2026-06-21 09:00",
      "last_update": "2026-06-22 17:50",
      "last_evidence_at": "2026-06-22 17:48",
      "last_commit": "abc1234",
      "mr_url": "https://gitlab.example.com/mr/12",
      "mr_iid": 12,
      "commit_count": 2,
      "mr_count": 1,
      "merged_mr_count": 0,
      "evidence_score": 90,
      "risk_level": "safe",
      "risk_label": "推进中",
      "risk_reason": "任务有执行证据，当前未命中异常规则",
      "risk_rank": 16,
      "result_state": "mr_active",
      "result_label": "MR 处理中",
      "active_days": 1,
      "evidence_age_hours": 0,
      "risk_tags": []
    }
  ]
}
```

## 风险规则

执行任务按风险优先级排序，异常任务排在前面：

- `完成无证据`：Jira 已完成，但没有分支、commit 或 MR 证据。高风险。
- `状态不一致`：MR 已合并，但 Jira/任务状态还没有完成。高风险。
- `未启动`：任务创建超过 1 天，但没有任何执行证据。中风险。
- `推进停滞`：超过 72 小时没有新的任务活动或代码证据。中风险。
- `未绑定需求`：执行任务没有父级需求，交付结果难以回流排期。中风险。
- `已闭环`：任务完成，并具备执行证据或明确绑定关系。
- `推进中`：任务有执行证据，且没有命中异常规则。

证据分数由分支、最后 commit、commit 日志、MR、merge 事件共同组成，最高 100 分。

## 前端实现

入口位于 **Git 协同看板** 的第三个 tab：**执行追踪**。

视图由三层组成：

- 指标带：执行任务总量、高风险、未绑定、有代码证据、状态不一致。
- 控制区：关键词搜索、风险筛选、负责人筛选、刷新。
- 表格区：任务、归属需求、执行证据、结果状态、风险判断、最近活动。

表格优先展示“证据链”和“回流关系”，而不是只展示 Jira 状态。这样可以看见 Task 的开发结果，也能避免 Task 被误当成需求排进需求池。

## 性能策略

- 后端一次查询 `task_telemetries`、`git_commit_logs` 和用户表，在内存中完成分组聚合。
- 前端消费紧凑 DTO，搜索、筛选和排序在已加载数组上完成。
- 执行追踪接口只在用户进入该 tab 后主动拉取；轮询时也只刷新当前 tab，避免额外负载。
- 表格使用固定最小宽度和内部滚动，任务量增长时不挤压整体布局。

## 后续扩展

- 把 `risk_level in (high, medium)` 接入通知中心，形成延期预警和状态不一致提醒。
- 增加 Jira Epic、Parent、Link 解析，自动补齐 `task_group_id` 绑定。
- 为 Jira Issue Type 增加配置映射，让团队自行定义哪些类型是需求、哪些类型是执行任务。
- 对接 GitLab pipeline、reviewer、deployment 结果，形成从需求到上线的完整证据链。
- 在需求排期表中增加“执行异常数”，从需求视角反查相关 Task/Bug 的风险。
