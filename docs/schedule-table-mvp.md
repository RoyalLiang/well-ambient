# 排期表 MVP 落地说明

## 目标

排期表 MVP 用来把“需求、排期、影子任务推进、延期风险”收敛到一个可扫描工作台，服务 PM、研发负责人和需求负责人快速判断：

- 哪些需求还没有进入可追踪排期。
- 哪些需求已经逾期、临期或排期后长期无推进。
- AI 解构出的影子任务是否已经和需求形成同一任务组。
- 需要调整分支、截止日或任务组时，是否能回到同一套排期弹窗完成操作。

## 接口契约

### `GET /api/schedule`

权限：`demands:read`

来源：基于 `task_telemetries` 一次查询聚合，不新增表结构。`issue_type = demand` 的记录作为排期主行，其它任务按 `task_group_id` 聚合为影子任务进度。

响应：

```json
{
  "generated_at": "2026-06-22 10:30",
  "summary": {
    "total": 12,
    "scheduled": 8,
    "unscheduled": 4,
    "in_progress": 3,
    "review": 1,
    "done": 2,
    "overdue": 1,
    "due_soon": 2,
    "stale": 1
  },
  "items": [
    {
      "demand_id": "DEMAND-001",
      "title": "用户部门刷新",
      "description": "登录后部门信息自动同步",
      "assignee": "Bob",
      "department": "Engineering",
      "repo": "platform-core",
      "branch": "feat/department-sync",
      "status": "progress",
      "task_group_id": "brain-demand-001",
      "due_date": "2026-06-30",
      "created_at": "2026-06-20 09:20",
      "last_update": "2026-06-22 10:20",
      "estimate_days": 2.5,
      "estimate_hours": 20,
      "difficulty": "Medium",
      "risk_level": "due_soon",
      "risk_label": "临期",
      "risk_reason": "2 天后到期，需要关注交付确定性",
      "risk_rank": 82,
      "days_remaining": 2,
      "subtask_total": 4,
      "subtask_done": 2,
      "subtask_active": 2,
      "subtask_review": 1
    }
  ]
}
```

## 风险规则

- `done`：需求状态为 `done`，不进入风险关注。
- `unscheduled`：未绑定开发分支，或已绑定分支但没有截止日。
- `overdue`：未完成需求的截止日早于今天。
- `due_soon`：未完成需求距离截止日不超过 3 天。
- `stale`：已有排期且未完成，最近更新时间超过 72 小时，并且影子任务未全部完成。
- `safe`：分支、截止日、推进状态完整，且不满足以上风险条件。

排序默认按 `risk_rank` 降序，再按截止日升序，保证逾期和临期需求优先出现在表格顶部。

## 前端实现

入口位于需求看板顶部，提供“流转看板 / 排期表”分段切换。排期表由三层组成：

- 指标带：需求总量、已排期、逾期、临期、推进滞后。
- 控制区：关键词搜索、风险筛选、负责人筛选、排序、刷新。
- 表格区：需求、负责人、排期、交付窗口、影子任务、风险、操作。

性能策略：

- 表格消费 `/api/schedule` 的紧凑 DTO，不再从全量任务记录临时推断风险。
- 筛选和排序在已加载数组上完成，避免频繁请求后端。
- 页面轮询保持 15 秒；仅当用户停留在排期表时同步刷新排期接口。
- 表格使用固定列宽和内部滚动，避免需求量增长后撑破页面布局。

## 后续扩展

- 延期预警通知可以直接复用 `risk_level in (overdue, due_soon, stale)` 的语义，接入通知中心或飞书机器人。
- 容量视图可在当前接口上增加 `capacity` 字段，按负责人/部门聚合工时与排期冲突。
- 页面增多后，排期表仍只依赖需求域接口，不需要耦合具体页面路由；新增需求来源只要写入 `task_telemetries` 即可展示。
