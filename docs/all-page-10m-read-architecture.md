# 全站千万量级有界读架构

## 结论与口径

本架构覆盖全部用户可达页面和全部 77 条 GET `/api` 路由，Daily Jira 只是第一个完整迁移样本。目标不是让任意 SQL、任意模糊搜索或任意全量导出在一千万行上都返回毫秒级，而是把交互式读取约束成可证明的有限问题：

- 10M 隔离数据上，warm 有界数据库读取 p95 `< 20ms`；
- 单机本地 HTTP handler（含查询、DTO 和 JSON）p95 `< 50ms`；
- 默认页 50 行，硬上限 100 行；时间线硬上限 200 行；详情嵌套集合必须单独设上限；
- 深页使用 opaque keyset/generation cursor，禁止 OFFSET 成本随页深增长；
- 聚合读取完成的投影，不在请求中扫描千万行写模型；
- 前端保留当前快照，刷新 single-flight，取消或抑制过期响应，完整新页就绪后原子替换。

冷缓存、并发排队、网络 RTT、浏览器长任务、外部 Jira/GitLab 延迟、索引创建和投影回填是独立指标，不能用本地 warm benchmark 代替。

## 根因层级

1. **缺少全站强制读 seam。** 少数页面已经有分页或投影，但新接口可以直接 `Find`、OFFSET 或返回全量 JSON，性能规则没有可执行门禁。
2. **聚合页重算写模型。** 返回几十个结果不代表只读几十行；日程、KPI、绩效和最强大脑接口会在请求内扫描、分组和关联大量事实。
3. **刷新所有权分散。** 页面 interval、SSE、显式刷新和路由切换可能同时发请求；旧请求晚到后覆盖新范围，造成渲染卡顿和页面抖动。
4. **ORM 无界/N+1。** 无 `Limit` 的 `Find`、详情嵌套全部子项、循环内关联查询，使 SQL 数、解码行数和 JSON 字节随数据增长。
5. **索引缺失是放大器。** 索引能消除扫描和临时排序，但无法修复全量响应、深 OFFSET、低选择性 contains 或请求内全量 GROUP BY。

## 公共深模块

```mermaid
flowchart LR
    UI[页面业务查询意图] --> PR[PagedResource\nsingle-flight / abort / stable snapshot]
    PR --> HC[HTTP Read Contract\nclass / hard limit / SLO / maturity]
    HC --> DA[领域 Adapter\n权限 / scope / filter / order]
    DA --> RP{读策略}
    RP --> KS[Generation Keyset]
    RP --> TL[Indexed Top-K Timeline]
    RP --> PJ[Completed Projection]
    RP --> DR[Bounded Detail/Directory]
    KS --> DB[(SQLite/GORM)]
    TL --> DB
    PJ --> DB
    DR --> DB
    DB --> QM[请求级 SQL 观测\nquery count / rows / p95]
    HC --> HM[/api/status\nhandler / bytes / breaches]
    QM --> HM
```

公共层隐藏游标编码、scope 指纹、checksum、数据集 generation、409 stale cursor、请求合并、旧响应抑制和指标窗口。领域 adapter 继续拥有权限、业务过滤、排序、聚合口径和具体索引；不引入可以任意拼列或任意 SQL 的万能 repository。

### 后端契约

每条 GET 路由声明：

- `class`: singleton、directory、detail、page、timeline、aggregate 或 stream；
- `target_strategy`: current、cached directory、bounded detail、keyset、generation keyset、projection、indexed top-k 或 bounded stream；
- `default_rows`、`max_rows`、`max_nested_rows`、`max_response_bytes`；
- query p95 和 handler p95 目标；
- snapshot/generation 规则；
- `maturity`: `verified`、`bounded` 或 `migration_pending`。

生产 wrapper 返回 `X-Well-Ambient-Read-*` 响应头，并在最近 2,048 次请求窗口中记录 handler p50/p95/p99、最大响应字节、SQL 数、SQL 行数、SQL p95/max、错误和预算越界。GORM `Query`、`Row/Scan`、`Raw` 都接入同一 request context；未使用 `WithContext(r.Context())` 的旧 handler 会显示为 0 条归属 SQL，作为迁移审计信号。

`GET /api/status` 只返回成熟度汇总，`GET /api/status?read_contracts=full` 才返回 77 条声明，避免常规状态轮询自身膨胀。

### 前端资源层

`web/src/lib/paged-resource.ts` 统一负责：

- 同 scope refresh single-flight；
- scope 改变时中止旧请求，并禁止旧 payload 发布；
- 首屏完整返回后原子替换；续页去重追加；
- 加载和错误期间保留已有快照，避免清空 DOM 引发布局位移；
- generation cursor 409 时重新读取完整首屏；
- 页面销毁时释放请求 owner。

页面只负责构造业务 scope、消费状态和提供“加载更多”。首波已经覆盖发布计划、发布 Jira 候选/已关联、上下文事实、语料文档和语料候选；Daily Jira 使用同一稳定快照原则及专用虚拟列表。

## 页面迁移矩阵

状态定义：`verified` 表示策略、索引/投影、边界和回归均已证明；`bounded` 表示已消除无界读取，但目标游标/投影或大规模基准仍需补齐；`pending` 表示 contract 已强制登记，领域查询仍可能随数据增长。

| 页面状态 | 主要数据路径 | 目标策略 | 当前状态与下一步 |
|---|---|---|---|
| shell | `/api/me`、config、status、preferences、SSE、work-items | singleton + bounded stream + page | 关键 singleton/stream verified；work-items bounded，补 generation 游标 |
| decision.agenda | agenda、decision queue、cockpit、exceptions、weekly、release facts | completed projections | release facts bounded；其余聚合 pending，拆分议程/决策/异常投影 |
| decision.daily_jira | `/api/decision/daily-jira`、SSE | generation keyset + stable snapshot | verified；自动 worker 是主刷新，手动同步仅辅助 |
| schedule.schedule | schedule、risk calendar、directory、users、projects、task activity | projection + top-k | directory/users/activity bounded；schedule/risk pending，建立日程投影 |
| schedule.releases | releases、project releases、Jira issues、snapshot/search | generation keyset + projection | 列表/详情已 verified 或 bounded；外部 Jira search pending |
| schedule.board | work-items、demand specs、execution runs、directory/users | generation keyset + bounded detail | 多条已 bounded/verified；legacy tasks 与 workspace 聚合待迁移 |
| schedule.projects | project scores/config/preferences | completed score projection | config bounded；scores pending |
| solutions.catalog | catalog、standards、workspace、demand specs | keyset + search projection | demand specs/standard detail bounded；catalog/search/workspace pending |
| evidence.health | data assets、strongest-brain evidence/quality/cockpit | keyset + completed projections | data assets verified；跨域聚合 pending |
| tasks.status | work-items、assignees、users、legacy tasks | generation keyset | work-items/目录 bounded；legacy `/api/tasks` pending |
| tasks.execution | execution runs/tasks、task activity、evidence chain | generation keyset + projection + top-k | runs/activity bounded；execution aggregate/evidence pending |
| kpi.overview | KPI、project scores、users、latest assets | completed KPI projection | latest asset verified、users bounded；KPI/scores pending |
| kpi.calculation | performance explanation/snapshots | completed score projection | pending；禁止请求内重放全部 evidence/events |
| settings.gitlab | config、status、webhook/logs、external projects | singleton + top-k/cache | 本地状态/logs bounded；外部目录 pending 且使用独立外部 SLO |
| settings.feishu | config/status | singleton | verified |
| settings.jira | config/status/link、release Jira search | singleton + bounded external search | 本地 singleton verified；外部 search pending |
| settings.performance | config、performance explanation/snapshot | singleton + score projection | config verified；绩效投影 pending |
| settings.projects | config、project config/scores | directory + score projection | config/project directory bounded；scores pending |
| settings.ai | config/status | singleton | verified |
| settings.solution_prompts | solution prompts | keyset | pending |
| settings.ai_context | context facts/docs/candidates、traces、replay | generation keyset + bounded detail | 主列表 verified，详情/trace bounded |
| settings.users | users/groups | cached directory + batch memberships | bounded；用户最多 5,000，membership 固定两条查询 |
| settings.matrix | users/groups/permissions | cached directory | bounded |
| settings.policies | policies/authz audit | cached directory + top-k | audit bounded；policy directory pending |
| settings.audit | audit/config versions/logs | indexed top-k | bounded |

## 路由成熟度清单

当前可执行 inventory：77 条 GET 路由中 `verified=17`、`bounded=32`、`migration_pending=28`。测试设有单调 ratchet：verified 不得低于 17，bounded-or-verified 不得低于 49，pending 不得高于 28。

### Verified（17）

`/api/config`、`/api/context/documents`、`/api/context/facts`、`/api/corpus-candidates`、`/api/data-assets/events`、`/api/data-assets/events/{id}`、`/api/data-assets/snapshots/latest`、`/api/data-assets/snapshots/{id}`、`/api/decision/daily-jira`、`/api/jira/link-config`、`/api/me`、`/api/me/decision-table-columns`、`/api/me/project-preferences`、`/api/notifications/sse`、`/api/projects/{project_key}/releases`、`/api/releases/{id}`、`/api/status`。

### Bounded（32）

`/api/ai/output-trace`、`/api/ai/requirement-clarification`、`/api/ai/traces`、`/api/audit-logs`、`/api/authz/audit-logs`、`/api/config/versions`、`/api/context/documents/{id}`、`/api/context/pack/replay`、`/api/context/packs/{id}/replay`、`/api/corpus-candidates/{id}/impact`、`/api/delivery/directory`、`/api/demand-specs`、`/api/demands/options`、`/api/execution/runs`、`/api/gitlab/webhooks/status`、`/api/groups`、`/api/logs`、`/api/permissions`、`/api/projects/config`、`/api/releases`、`/api/releases/{id}/jira-issues`、`/api/releases/{id}/snapshot`、`/api/requirements/clarification`、`/api/review-contracts`、`/api/solution-standards/{id}`、`/api/strongest-brain/override-audit`、`/api/strongest-brain/releases`、`/api/task-tracking/assignees`、`/api/tasks/commits`、`/api/users`、`/api/work-items`、`/api/work-items/{id}`。

### Migration pending（28）

`/api/agenda/summary`、`/api/authz/policies`、`/api/delivery/exceptions`、`/api/delivery/quality`、`/api/execution/tasks`、`/api/gitlab/projects`、`/api/kpi/performance`、`/api/kpi/report-preview`、`/api/performance/explanation`、`/api/performance/snapshots/{id}`、`/api/projects/scores`、`/api/releases/jira-search`、`/api/schedule`、`/api/schedule/risk-calendar`、`/api/solution-catalog`、`/api/solution-catalog/projects`、`/api/solution-catalog/{id}`、`/api/solution-prompts`、`/api/solution-standards`、`/api/solutions/workspace`、`/api/strongest-brain/ai-traces`、`/api/strongest-brain/decision-queue`、`/api/strongest-brain/delivery-cockpit`、`/api/strongest-brain/demand-readiness`、`/api/strongest-brain/evidence-chain`、`/api/strongest-brain/exceptions`、`/api/strongest-brain/weekly-decisions`、`/api/tasks`。

## 已落地索引与查询边界

首波索引由 `internal/db/read_indexes.go` 统一迁移并以 `EXPLAIN QUERY PLAN` 回归：

- context facts/documents：`status, updated_at DESC, id DESC` 及全局游标；
- corpus candidates：状态/全局/文档局部 keyset 与文档状态统计；
- execution runs/actions：demand 及 run 局部 keyset；
- release：全局和 project/status 的 release-date expression page；
- release Jira issue：project/last-update partial index；
- task activity：`LOWER(task_id)` 的 Git/Jira 时间线索引，避免大小写历史变体触发临时排序；
- identity：用户创建顺序索引，membership 由 `(user_id, ...)` 唯一索引批量关联；无配置边界时的历史负责人兜底使用 partial covering index，且用户和负责人目录都硬限 5,000。

首波领域边界还包括：详情文档最多 100 个候选并给出 continuation；execution run 每项最多 20 个 action；task activity 合并后最多 100 项；demand spec 历史最多 100 版；用户目录最多 5,000 人且 membership 固定两条 SQL；发布、发布 Jira 和 context/corpus 列表最大 100 行。

## 10M 隔离基准

命令：

```bash
GOCACHE=/tmp/well-ambient-all-page-gocache go run ./cmd/read-path-bench --rows 10000000 --samples 500 --limit 100
```

2026-08-21 本机结果（数据库约 1.01GB，seed 3.453s，建索引+ANALYZE 12.785s）：

| 场景 | p50 | p95 | p99 | max |
|---|---:|---:|---:|---:|
| entity first page | 0.093ms | 0.103ms | 0.122ms | 0.305ms |
| deep keyset page | 0.094ms | 0.110ms | 0.174ms | 0.338ms |
| entity timeline | 0.095ms | 0.116ms | 0.181ms | 0.445ms |
| aggregate projection lookup | 0.005ms | 0.006ms | 0.006ms | 0.012ms |
| local handler + JSON | 0.106ms | 0.117ms | 0.201ms | 0.276ms |

四条查询计划分别命中 `idx_read_entity_page`、`idx_read_timeline` 和 aggregate primary key，无 table scan 或临时排序。基准数据库创建在临时目录，命令结束自动删除，不写主库。

## 隔离认证浏览器证据

- 发布列表首屏 50 行、续页后 60 行；选中项保持不变。候选 Jira 从 100 续到 125，已关联 Jira 从 100 续到 105，均真实发送 cursor 请求并在末页移除续页入口。
- 搜索 `Release 59` 只返回 1 条，服务端收到 `q=release+59&limit=50`；使用真实键盘清空后恢复 50 行首屏与续页入口。
- 人为增加 180ms 响应延迟时，刷新期间继续保留 51 个已渲染行，列表 `x/y/width/height` 和刷新后均为零变化，旧快照没有先被清空。
- 1440/1024/760/390px 均无文档级横向滚动；760/390px 搜索控件为 44px，列表和检查器按既有断点堆叠。
- 验证只连接临时 fixture/Vite，不访问主数据库和外部系统。验收后页签转为 `about:blank`，两个服务及临时 fixture 均已清理。关闭隔离服务时 Vite 仅记录预期的 SSE 重连 warning，没有可见业务错误。

## 发布和回滚

1. 在生产数据库副本上先执行 migrations，记录每个索引的耗时、峰值临时空间、最终磁盘增量和写阻塞；SQLite 需要预留足够磁盘并安排受控维护窗口。
2. 对每个迁移 adapter 保存 `EXPLAIN QUERY PLAN`、100/1M/10M warm/cold p95、响应字节、SQL 数和并发 1/8/32 的结果；未通过不得从 pending 升级。
3. 先部署 contract/observability，再部署 adapter，最后部署前端消费者；观察 `/api/status?read_contracts=full` 的 query/handler/bytes breaches。
4. 投影采用双写或增量 worker + generation watermark；只发布完成 generation，构建中的部分结果不得覆盖稳定快照。
5. 回滚时先切回旧 reader，再停止新投影 worker；新增索引保留到确认无回滚需要，避免在故障窗口执行破坏性 DROP。
6. 页面验收必须覆盖 1440/1024/760/390、初载/续页/刷新/错误/空态/路由切换，并采集请求数、响应字节、CLS、INP/长任务与滚动位置。

## 后续迁移顺序

1. `schedule`、`risk-calendar`、`execution/tasks`、`kpi/*`、`performance/*`：先建 completed projection，禁止继续优化请求内全表 GROUP BY。
2. `solution-catalog`、`solution-standards`、`solution-prompts`、`tasks`：建立 scope-aware keyset/search projection，移除 OFFSET 和全量列表兼容路径。
3. strongest-brain/decision 聚合：复用同一 task/performance/release projection，不重复读取写模型。
4. 外部 Jira/GitLab 搜索和目录：独立缓存、过期策略、熔断与外部 SLO，不纳入本地 50ms handler 承诺。

完成标准不是“77 条路由都有一行配置”，而是 pending 降到 0，且每一条 route/surface 都有领域边界、查询计划、大规模基准、响应预算和浏览器稳定性证据。
