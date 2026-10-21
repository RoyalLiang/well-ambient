# 2026-10-21 Knowledge, member roles and Jira Issue preview

## 中文

- Skill中心新增知识快照正文浏览、来源筛选与展开核验。
- MCP知识上下文支持紧凑格式和同一会话背景复用；来源、授权与时效仍重新校验。
- 成员角色目录增加搜索、筛选、分页与清晰的操作反馈；分配、移除和转让使用统一浅色弹窗、可读字段和响应式按钮，恢复缺失的移除确认。
- 代码评审列表移除重复仓库策略与规则配置，集中治理入口保留。
- 独立Jira到GitLab Issue每日预览：选择项目、分组及承载仓库，默认北京时间09:00且停用，不创建、修改或关闭远端Issue。
- GitLab API Token可在配置页重新录入；未新增SSH私钥托管。

## English

- Browse runtime knowledge snapshots, filter sources and revalidate expanded evidence in the Skill center.
- Retrieve dense MCP knowledge context and reuse unchanged session background after fresh authorization and source checks.
- Search, filter and page through members. Assignment, removal and admin-transfer dialogs share readable light-theme fields and responsive actions; the missing removal confirmation is restored.
- Remove duplicate repository policy controls from the code-review list while retaining centralized governance.
- Configure an independent Jira-to-GitLab-Issue daily preview for one target repository within a group. The default is09:00 Asia/Shanghai, disabled. Preview never writes remote Issues.
- Re-enter the GitLab API Token in configuration. SSH private-key storage is outside this release.

The memory receipt is not cross-session recovery. Synthetic wire-size savings do not establish real model-token savings. Remote Issue reconciliation and write policies remain future work.
