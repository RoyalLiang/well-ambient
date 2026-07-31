# 发布版本与 Jira 事项关联数据设计

版本计划使用现有发布领域表，不增加语义重复的“版本-Jira 链接表”。本地发布版本是主实体，Jira 事项先作为交付项进入本地目录，再通过发布关系表关联。

## 表与约束

| 表 | 关键字段 | 作用与约束 |
| --- | --- | --- |
| `release_versions` | `id`, `project_key`, `source`, `external_id`, `name`, `status`, `release_date` | 本地发布版本。`(project_key, source, external_id)` 唯一，版本只属于一个项目。 |
| `task_telemetries` | `task_id`, `source`, `external_key`, `project_key`, `issue_type`, `revision` | 交付项目录。Jira 事项以 `source = 'jira'` 标识，`revision` 用于 CAS 并发控制。 |
| `work_item_release_links` | `work_item_id`, `release_version_id`, `relation`, `is_primary`, `active` | 唯一关联事实。版本计划使用 `relation = 'target_fix' AND is_primary = true AND active = true`。 |
| `work_item_events` | `work_item_id`, `event_type`, `actor`, `before_json`, `after_json`, `revision` | 不可变审计流，记录批量关联中的每条关系变化。 |
| `work_item_sync_operations` | `idempotency_key`, `work_item_id`, `operation`, `status` | 仅真实 Jira 外部版本变化进入 outbox；本地版本关联不产生 `fixVersion` 写回。 |

历史 `release_jira_links` 表保留兼容，但不再是版本计划的读写来源。删除旧表属于单独的数据迁移，不与本次可逆功能修复绑定。

## 索引

| 索引 | 字段顺序 | 主要查询 |
| --- | --- | --- |
| `idx_release_identity` | `project_key, source, external_id`（唯一） | 版本身份幂等写入。 |
| `idx_release_catalog` | `project_key, status, release_date, id` | 版本计划按项目、状态和发布日期读取。 |
| `idx_task_jira_project_type` | `source, project_key, issue_type` | 按版本所属项目筛选 Jira 需求与缺陷候选。 |
| `idx_work_item_active_relation_primary` | `work_item_id, active, relation, is_primary` | 查询某 Jira 事项当前主目标版本、执行冲突校验。 |
| `idx_release_active_relation_primary` | `release_version_id, active, relation, is_primary` | 统计和列出某版本已关联的 Jira 事项。 |

批量关联最多 100 条，先一次读取事项与现有主目标关系完成全量校验，再在单事务内使用事项 `revision` 做 CAS 更新；任一跨项目、重复归属或版本冲突都会回滚整批。版本列表用一次 `GROUP BY release_version_id` 聚合事项数，项目看板用一次批量联表投影版本名称，避免逐卡查询。
