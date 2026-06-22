# 执行任务与缺陷观测 MVP

## 目标

在当前团队的 Jira 使用方式里，Jira `Task` 实际承载的是需求，因此应该进入需求池和排期表。执行观测 MVP 的目标随之调整为：追踪需求拆解后的影子开发任务、缺陷修复任务，以及它们产生的代码证据。

执行观测 MVP 用来把这些执行任务放进独立视图，帮助 PM、研发负责人和需求负责人判断：

- 影子开发任务或缺陷是否绑定到一个真实需求。
- 任务是否已经产生代码证据。
- MR 合并、Jira 状态、需求排期之间是否出现断层。
- 哪些完成任务缺少证据，哪些活跃任务已经停滞。

这套链路的边界是：需求池和排期表承载 `issue_type = demand` 的主需求，其中 Jira `Task`、`Story`、`Feature`、`Epic` 和本地口头需求都会归入需求；`task` 用于 AI 解构后的影子开发任务；`bug` 用于缺陷流转和缺陷修复证据。

## 数据边界

执行观测复用现有 `task_telemetries` 和 `git_commit_logs`。

- `task_telemetries.issue_type = demand`：作为父级需求，进入需求池、排期表和迭代治理。Jira `Task` 在本部署中会同步为 `demand`。
- `task_telemetries.issue_type = task`：作为 AI 解构或人工拆解后的执行任务，进入执行追踪视图。
- `task_telemetries.issue_type = bug`：作为缺陷，保留缺陷属性，进入执行追踪和缺陷治理，不和需求吞并。
- `git_commit_logs`：作为执行证据，按 `task_id` 汇总 commit、MR、merge 事件。
- 用户表：用于补充负责人部门，让问题能按组织维度排查。

Jira 同步器的类型映射如下：

- Jira `Task`、`Story`、`Feature`、`Epic`、`Requirement`、`需求`、`任务` -> `demand`
- Jira `Bug`、`Defect`、`缺陷`、`故障` -> `bug`
- AI 解构导入的子任务 -> `task`

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

表格优先展示“证据链”和“回流关系”，而不是只展示 Jira 状态。这样可以看见影子任务与缺陷的开发结果，同时让 Jira Task 作为需求进入排期治理。

## 性能策略

- 后端一次查询 `task_telemetries`、`git_commit_logs` 和用户表，在内存中完成分组聚合。
- 前端消费紧凑 DTO，搜索、筛选和排序在已加载数组上完成。
- 执行追踪接口只在用户进入该 tab 后主动拉取；轮询时也只刷新当前 tab，避免额外负载。
- 表格使用固定最小宽度和内部滚动，任务量增长时不挤压整体布局。

## 后续扩展

- 把 `risk_level in (high, medium)` 接入通知中心，形成延期预警和状态不一致提醒。
- 增加 Jira Epic、Parent、Link 解析，自动补齐 `task_group_id` 绑定。
- 为 Jira Issue Type 增加配置映射，让团队自行定义哪些类型是需求、哪些类型是执行任务或缺陷。
- 对接 GitLab pipeline、reviewer、deployment 结果，形成从需求到上线的完整证据链。
- 在需求排期表中增加“执行异常数”，从需求视角反查相关 Task/Bug 的风险。
