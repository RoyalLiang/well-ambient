# 本地发布版本直接拥有 Jira 事项范围

版本计划以本地 `Release Version` 为发布承诺主体，并直接关联一个或多个同项目的 Jira 交付项；不再要求或创建 Jira 项目版本，也不把本地关联写回 Jira `fixVersion`。这样可以让发布范围独立于 Jira 版本目录，同时用发布版本的项目归属唯一确定候选 Jira 范围，避免重复的 Jira 项目选择和双向版本状态冲突。

## Consequences

- Jira 事项只能有一个有效的主目标发布版本；批量关联必须先完成同项目与冲突校验，并在一个事务中提交。
- `WorkItemReleaseLink` 是关联事实的唯一写模型；历史 `ReleaseJiraLink` 暂时保留用于兼容旧数据与旧接口，但不再被版本计划界面读写。
- 只有关联到真实 Jira 外部版本的变更才进入 Jira 版本同步 outbox；本地发布版本关系不会清空或覆盖 Jira `fixVersion`。
