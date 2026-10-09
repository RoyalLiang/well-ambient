# Daily Jira PostgreSQL 性能改造与验证

第二批性能改造已完成本地实现和相关测试。它减少稳定数据版本下重复翻页的汇总成本，并补齐日期、事务回滚和全表重建的一致性约束。没有部署，也没有连接业务数据库。

## 改动与契约

- [Reader](../internal/dailyjira/read_model.go) 继续在 PostgreSQL repeatable-read 事务中读取版本、汇总和列表；每次请求仍校验游标与当前授权范围。
- [汇总缓存](../internal/dailyjira/summary_cache.go) 由 Reader 持有，按标准化项目、负责人、负责人过滤开关、generation、数据库日期和时区区分，LRU 最多保存 128 份纯计数。不同数据库和 Server 不共享缓存。
- summary 维持原有口径：包含整个授权范围，不随搜索词或当前 bucket 缩小。列表内容和精确汇总没有截断。
- [HTTP 处理](../internal/server/daily_jira_handlers.go) 复用 Server 所属的 Reader；数据库连接更换后重新创建 Reader。
- PostgreSQL 游标保存数据库日期与时区。过期日期、时区变化和升级前缺少该字段的 PostgreSQL 游标均返回 `ErrStaleCursor`，HTTP 沿用 409。SQLite 原有游标和投影读取保持兼容。
- 成功结束只读事务后才发布缓存；调用者已有事务时绕过缓存，避免 savepoint 回滚后相同 generation 对应不同未提交内容。

新增缓存要求所有受跟踪的修改都推进 generation。测试发现原有触发器未覆盖 TRUNCATE，能够复现“汇总非零、列表为空”。[读模型迁移](../internal/readmodel/page.go) 已添加 TRUNCATE 事件，并在只读启动检查中校验事件集合和启用状态。读取先取得 `ACCESS SHARE` 锁，再建立快照，避免并发 TRUNCATE 在版本查询和列表查询之间替换表；该锁兼容普通行写入。PostgreSQL 的 TRUNCATE 触发器仅支持语句级，见 [官方 CREATE TRIGGER 文档](https://www.postgresql.org/docs/16/sql-createtrigger.html)。

## 可复现环境

本机 Apple M4 Pro / macOS arm64 / Go 1.25.0。PostgreSQL 16.13 从[官方源码](https://www.postgresql.org/ftp/source/v16.13/)下载并校验 SHA-256，按照[官方构建说明](https://www.postgresql.org/docs/16/install-make.html)安装到临时目录，没有修改系统服务。

[测试工具](../internal/testutil/pgtest/postgres.go) 只接受 PostgreSQL bin 目录，不接受 DSN。每个测试创建独立临时 cluster 和 mode-0700 Unix socket 目录，关闭 TCP，结束后关闭连接、停止进程并移除自有数据。初始化使用 C locale / UTF-8，服务端时区 UTC，测试禁用 fsync，因此这些结果不能用作写入持久性或生产 IO 吞吐指标。

夹具使用真实 `TaskTelemetry` 模型、仓库 `MigratePostgresReadIndexes` 和 generation 触发器；不是单独为基准设计的索引。种入 1 万或 10 万事项，10 个项目、40 天创建分布、约 1/11 已完成、每条 512 字节合成 decision_logs，然后执行 ANALYZE。测量 P1 项目、alice 或未指派、seven_day bucket、20 条一页。

基线取提交 `b2bd8d8` 的 `internal/dailyjira/read_model.go` 与 `internal/readmodel/page.go`，通过 Go `-overlay` 加载；其他夹具和索引与优化后相同，没有切换或覆盖脏工作树。每组预热后连续读取第一页 100 次，单客户端、一个进程，每组报告一次；这是缓存命中基准，不是并发压力测试。

## 结果

| 数据量 | 平均修改前 → 后 | p95 修改前 → 后 | p99 修改前 → 后 | Go 分配修改前 → 后 |
| --- | --- | --- | --- | --- |
| 10,000 | 6.490 → 3.132 ms | 7.647 → 3.325 ms | 7.725 → 3.521 ms | 118,279 → 93,833 B/op |
| 100,000 | 29.606 → 17.663 ms | 33.52 → 18.49 ms | 33.84 → 18.71 ms | 120,569 → 93,421 B/op |

真实前翻／后翻三页的回归测试在修改前失败：汇总执行 3 次，要求 1 次；修改后通过。结果顺序、总数、完整授权范围和前后页边界同时校验。

原始数据保存在：

- [基线基准](performance/daily-jira-postgres-2026-10-04/benchmark-before.txt) / [优化后基准](performance/daily-jira-postgres-2026-10-04/benchmark-after.txt)
- 基线 [summary](performance/daily-jira-postgres-2026-10-04/before/summary.json) / [items](performance/daily-jira-postgres-2026-10-04/before/items.json)
- 优化后冷读 [summary](performance/daily-jira-postgres-2026-10-04/after/summary.json) / [items](performance/daily-jira-postgres-2026-10-04/after/items.json)

计划包含实际 SQL 与绑定值展开，以及 `EXPLAIN (ANALYZE, BUFFERS, SETTINGS, FORMAT JSON)`。10 万数据下，summary 单次计划仍约 12–13 ms、8,334 shared-hit blocks；items 仍约 14–15 ms、8,438 shared-hit blocks，均有并行扫描。优化消除了命中缓存时的 summary 重算，没有使单次查询或动态排序变为恒定成本。

## 测试与复现

使用 Go 1.25 或更高版本。将环境变量设置为本机 PostgreSQL 的 bin 绝对路径；这里仅指定测试二进制，不是数据库地址。

```bash
export WELL_AMBIENT_TEST_PG_BIN=/absolute/path/to/postgresql/bin
go test ./internal/dailyjira ./internal/readmodel ./internal/db ./internal/solutioncatalog ./internal/strongestbrain -count=1
go test -race ./internal/dailyjira ./internal/readmodel -count=1
go test -race ./internal/server -run 'Test.*DailyJira|TestAllPageRead' -count=1
go test ./internal/dailyjira -run '^$' -bench '^BenchmarkPostgresDailyJiraReadPage$' -benchtime=100x -count=1 -benchmem
WELL_AMBIENT_PG_PLAN_DIR=/tmp/daily-jira-pg-plans go test ./internal/dailyjira -run '^TestPostgresDailyJiraExplain$' -count=1
```

没有设置 `WELL_AMBIENT_TEST_PG_BIN` 时，普通 Go 测试会显式跳过 PostgreSQL 场景；CI 必须提供该变量，才能声称已运行 PostgreSQL 验证。测试不会自动下载数据库或使用业务配置。源码构建不是测试工具的职责。

上述模块回归、race 和受影响 HTTP 回归均已通过。macOS race 构建有 `LC_DYSYMTAB` 链接器警告，退出码为零，无 race 报告。

[PostgreSQL 集成测试](../internal/dailyjira/read_model_postgres_test.go) 覆盖：

- 重复翻页、后退、标准化 scope、搜索／bucket 切换的计数口径。
- 项目、负责人、过滤开关和未指派边界；插入、负责人变化、删除后的缓存和游标失效。
- 数据库日期与时区切换、历史游标；并发请求和已取消请求。
- 写事务未提交与 savepoint 回滚；generation 已读取后并发提交，确保 summary/items 保持一个快照。
- TRUNCATE 后清空计数、读快照阻止并发 TRUNCATE、旧触发器被启动检查拒绝以及重新迁移成功。

[缓存界限测试](../internal/dailyjira/summary_cache_test.go) 校验旧 scope／版本淘汰和时间隔离；[Server 生命周期测试](../internal/server/daily_jira_reader_test.go) 校验并发请求共用 Reader、不同 Server 隔离及数据库替换。

## 部署前置条件与剩余工作

本轮没有新增业务字段，但改变了读模型触发器契约。部署前按现有 [server 迁移入口](../cmd/server/main.go) 执行 `--migrate-only`；关闭自动迁移的服务会拒绝旧事件集合或禁用的触发器。此记录没有执行该业务环境迁移。回滚代码时可以保留新增 TRUNCATE 事件，与旧读取代码兼容。

性能边界仍包括：新 scope／新 generation 的冷读、首次并发请求的重复聚合、频繁写入导致的缓存失效、列表扫描与排序、粗粒度 generation 行锁。当前结果不覆盖百万规模、读写混合负载、连接池等待、进程 RSS 和生产 p95/p99；PostgreSQL 部署版本也未核实。

跨日检查在后端游标层完成，本轮没有修改或验收前端保留窗口在自然日变化时的刷新行为。浏览器端若仅按 generation 保留已有页，仍需要单独检查该交互。

下一切片应测量真实命中率和混合读写下的连接池等待，再依据计划优化窄读投影／排序索引。随后推进 worker 资源隔离和租约／fencing，最后把代码审查接入受治理运行时；不在这次缓存改造中扩展业务状态机。
