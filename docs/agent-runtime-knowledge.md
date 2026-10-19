# 按需求获取系统知识背景

Agent 可以先获取需求相关的背景、能力边界与场景证据，再评估需求。知识来自本系统已经同步或导入的数据，不会调用模型重新编造系统现状，也不会改变 Jira、方案或评审原记录。

## 让服务定时刷新知识

新版本服务启动后，内置 worker 默认每 6 小时重新整理一次知识投影，并在启动时补建尚不存在的快照。worker 每分钟检查是否到期；关闭服务后停止。不需要依赖某个 Codex 对话保持在线。

部署时，按项目现有数据库迁移流程创建 `runtime_knowledge_states`、`runtime_knowledge_generations`、`runtime_knowledge_items` 三张新增表；本次没有对运行环境执行迁移。已保存的自定义间隔会保留，6 小时是首次初始化的默认值。

刷新覆盖以下来源：

| 来源 | 纳入条件 | 可以支持的结论 |
| --- | --- | --- |
| Jira | 已同步到本地、具有项目范围的事项 | 记录过什么需求、事项当前同步状态 |
| 方案 | 方案资产当前指向的已发布版本，关联事项有项目范围 | 已发布的设计及其约束 |
| 事实卡 | active；关联文档仍 active 且导入完成 | 人工或审核确认的知识陈述 |
| 文档 | active 且导入 completed | 文档所描述的架构、场景与边界 |
| 代码评审 | completed 或 partial | 指定 head/base SHA 下的评审摘要与场景 |
| Agent 能力 | 能力及其版本均 active | 已注册的能力、类型与验证状态 |

来源按游标分页提取，没有每类 2000 条的总量截断。长正文按最多 16000 UTF-8 字节分段，相邻段保留重叠，避免丢失分段边界上的关键词。检索先筛选关键词候选，再轮流读取各类来源，减少 Jira 数量对其他知识的影响。当前使用关键词召回，不包含向量检索或同义词语义召回。

一次刷新在数据库事务中发布，使用版本比较防止并发构建覆盖新状态。失败时保留上次成功快照，记录 `refresh_failed`；相同内容复用已发布代号。仅保留当前及前一代。每条证据保留来源版本、完整投影正文的哈希、源更新时间、范围和陈述类型。

全局超级管理员可使用现有登录会话调用：

| 方法和路径 | 用途 |
| --- | --- |
| `GET /api/agent-runtime/knowledge/settings` | 查询开关、间隔、配置版本、快照代号与刷新状态 |
| `PUT /api/agent-runtime/knowledge/settings` | 修改 `enabled`、`interval_minutes`、`expected_version` |
| `POST /api/agent-runtime/knowledge/refresh` | 立即重建本地知识快照；即使定时开关关闭也可手动调用 |

间隔可设置为 1 至 10080 分钟。保存时必须带查询所得的版本，冲突后重新读取。`enabled=false` 停止后续定时刷新，已有证据仍须通过读取时的来源核验。禁用开放读取能力则会拒绝外部知识请求。

## 给外部 Agent 发布范围

知识接口复用已有集成凭证、开放读取开关、调用审计和配额。旧策略不会自动获得新知识权限。

在受审的开放策略中添加 `knowledge.read`，并通过 `field_rules.knowledge_get_context` 显式发布允许的来源类型，例如：

```yaml
actions:
  - knowledge.read
  - jira.read
  - review.read
field_rules:
  knowledge_get_context:
    - jira
    - solution
    - context_fact
    - context_document
    - review
    - capability
```

这段配置应合并到现有策略，保留原有项目、仓库和字段约束。Jira 正文还要求已有 `jira_get_issue` 发布 `key`、`project`、`summary`、`description`、`status`、`issue_type`、`updated_at`。Jira 和方案都要求 `jira_issue_visibility: verified_cache`；评审摘要还要求 `review.read` 和对应 `review_get` 字段。参考 [完整策略示例](../capabilities/open/policy.example.yaml)。

项目范围使用 `allowed_projects`，仓库范围使用 `allowed_repositories` 中的 GitLab Project ID。已有资料使用仓库名称或路径时，只通过当前 `gitlab.repos` 中唯一的 `Name/Path → ProjectID` 映射归一化。歧义和无法映射的名称会被排除，数字项目 ID 不会被同名仓库覆盖。

全局知识默认不对外发布。确认适合该连接后，可显式设置 `data_rules.knowledge_global_context: allow`。`module`、`demand_type` 等没有项目或仓库映射的范围暂不发布，也不会被自动提升为全局。

## 通过 HTTP 或 MCP 取上下文

| HTTP | MCP 工具名称（未加实例前缀） | 用途 |
| --- | --- | --- |
| `GET /open/v1/knowledge` | `knowledge_describe` | 查看当前连接的来源类型、范围、刷新状态与限制 |
| `POST /open/v1/knowledge/context` | `knowledge_get_context` | 按问题和预算取相关背景 |
| `POST /open/v1/knowledge/evidence` | `knowledge_get_evidence` | 展开指定快照中的一段证据 |

HTTP 使用 `Authorization: Bearer <集成凭证>`。远程 MCP 与 stdio 适配器提供相同能力；工具公开名称固定以 `well-ambient-` 开头，例如 `well-ambient-knowledge_get_context`。OpenAPI 见 [接口契约](../capabilities/open/contracts/openapi.yaml)。

上下文请求示例：

```json
{
  "text": "评估车辆与场位原子锁需求，检查既有设计、接管场景与能力边界",
  "projects": ["WA"],
  "repositories": ["42"],
  "token_budget": 1800
}
```

可用 `kinds` 限定来源。不指定项目或仓库时，在本连接发布的相应范围内检索。请求未授权范围会失败。

结果中的 `compact` 是带信任边界声明的压缩背景，来源标题和正文片段使用 JSON 转义，不能伪造上下文结构。`evidence` 附 `source_ref`、`content_hash`、`source_updated_at`、`claim` 和 `chunk_index/chunk_count`。`token_count` 使用 UTF-8 字节数作为保守 token 上界，预算范围为 512 至 16000，**只覆盖 compact 文本**，响应元数据不计入该预算；没有宣称特定模型的实测 token 节省比例。

核对关键结论时，将返回的 `snapshot_id`、`source_ref` 传给证据接口。证据返回所索引段的正文及来源信息；长文档可以按段引用。读取时重新校验来源状态、发布指针、父文档、范围映射和指纹。变化或撤回后的旧证据不可用，应重新获取上下文。快照超过保留期也会返回不可用。

## 让 Agent 在评审时先取背景

知识流程通过统一 `well-ambient` Skill 的 `runtime-knowledge` 子技能对外提供。在「AI 治理 → Skill 中心」安装主技能并连接 MCP 后，Agent 可在评审需求、评估方案、查询系统能力或场景边界时自动选择这套检索流程。能力注册表同时增加 `knowledge.runtime-context` 上下文提供者。

典型调用顺序是：查看可用范围，按需求取压缩上下文，针对关键结论或冲突展开证据，再进行评估。仅把工具接入客户端，不保证每轮对话都会调用它；是否调用仍取决于客户端和技能路由。知识不会训练进模型参数。

## 把未验证部分留在结果里

本地知识刷新不能代表上游 Jira 或文档服务刚刚完成同步。结果会明确标记上游同步时效未知；本地快照过旧、刷新失败、扫描或候选数量受限、证据变化和无匹配结果也有独立缺口标记。`complete=false` 表示不能据此证明全系统和现场环境的完整覆盖。

需求完成、设计发布、评审通过或 Agent 能力注册，都不能单独证明代码已经实现、上线或现场验收通过。冲突陈述保留为待核对证据，检索器不自动裁决。外部 Confluence、GitLab 源码、运行日志等只有在已同步到上述来源时才会纳入；本次没有新增外部采集凭证或改变现有采集周期。

停用定时刷新和开放读取即可停止本能力的后台更新与外部使用。来源记录不会被修改或删除。当前验证使用隔离 SQLite 和本机模拟服务；生产 PostgreSQL 的并发、多实例竞争及真实存量负载仍需发布后的独立验证。
