# SQLite 历史数据迁移到 PostgreSQL

## 适用范围与边界

首次安装页支持把一个由服务器配置指定的、已经停写的 SQLite 快照迁移到 PostgreSQL。浏览器不能提交文件路径；服务只读取 `database.legacy_sqlite_path`，生产示例默认是：

```yaml
database:
  driver: setup
  legacy_sqlite_path: /var/lib/well-ambient/legacy/well-ambient.db
```

Compose 对应宿主机路径为 `deploy/runtime/data/legacy/well-ambient.db`。

自动迁移覆盖当前版本 `db.RequiredSchemaModels()` 声明的应用事实表，不复制 SQLite 专用派生表、未知旧表、`sqlite_sequence`、`daily_jira_audit_*` 或 `read_model_generations`。附件文件不在 SQLite 内，必须单独复制并验收。开发机没有真实 PostgreSQL 服务，因此上线前仍须在 Linux staging 用生产快照完整演练。

## 迁移前门禁

必须同时满足：

- 旧服务及后台 worker 已停止写入；
- 已用 SQLite 备份接口生成独立快照，不能直接搬运仍在 WAL 写入的工作文件；
- 快照通过 `PRAGMA quick_check` 与 `PRAGMA foreign_key_check`；
- 目标 PostgreSQL 版本不低于 14；
- 目标数据库不存在，或已经存在但当前 schema 完全为空；
- 当前 PostgreSQL 角色能连接维护库；若目标数据库不存在，还必须具有 `CREATEDB` 或超级用户权限；
- PostgreSQL、SQLite 快照、附件和迁移报告均有独立备份与回退责任人；
- setup token 与数据库密码不同，部署入口未对普通用户开放。

`CREATE DATABASE` 不能在事务块中执行，且需要相应权限；安装服务先连接维护库查询 `pg_database` 和当前角色权限，再用 `TEMPLATE template0` 创建干净目标库。参见 PostgreSQL 官方 [`CREATE DATABASE`](https://www.postgresql.org/docs/current/sql-createdatabase.html)。

## 1. 冻结 SQLite 与附件

在旧系统停写后执行：

```bash
migration_dir=/srv/well-ambient-migration/2026-08-26
source_db=/srv/well-ambient-old/well-ambient.db
attachment_dir=/srv/well-ambient-old/data/demand-attachments

install -d -m 0700 "$migration_dir"
sqlite3 "$source_db" ".backup '$migration_dir/source.snapshot.db'"
sqlite3 -readonly "$migration_dir/source.snapshot.db" "PRAGMA quick_check;"
sqlite3 -readonly "$migration_dir/source.snapshot.db" "PRAGMA foreign_key_check;"
sha256sum "$migration_dir/source.snapshot.db" > "$migration_dir/source.snapshot.db.sha256"
find "$attachment_dir" -type f -print0 | sort -z | xargs -0 sha256sum > "$migration_dir/attachments.sha256"
```

SQLite CLI 的 `.backup` 使用在线备份接口生成一致快照；也可以使用 `VACUUM INTO`，但目标文件必须不存在或为空。参见 [SQLite CLI](https://www.sqlite.org/cli.html)、[Backup API](https://www.sqlite.org/backup.html) 和 [`VACUUM INTO`](https://www.sqlite.org/lang_vacuum.html#vacuuminto)。

复制到部署目录并设为只读：

```bash
install -d -m 0700 deploy/runtime/data/legacy
install -m 0400 "$migration_dir/source.snapshot.db" \
  deploy/runtime/data/legacy/well-ambient.db
```

容器运行用户必须能读取该文件。不要把快照提交 Git，也不要挂载旧系统仍在使用的原文件。

## 2. 打开首次安装页并测试连接

保持 `database.driver: setup`，启动首次部署。第一步填写：

- PostgreSQL 主机与端口；
- 目标数据库名，例如 `well_ambient`；
- 维护数据库名，通常为 `postgres`；
- 用户名、密码、SSL 与安装令牌。

点击“测试连接”后，页面必须明确显示以下状态之一：

- **目标数据库不存在**：同时显示当前角色是否允许创建；
- **目标数据库存在且 schema 为空**：允许初始化；
- **已识别完整的 Well Ambient 结构**：只接入，不重复迁移；
- **目标 schema 已有未知表**：安装被阻止。

连接测试不会创建数据库或写运行配置。输入变化后，原测试结果立即失效。

## 3. 只确认一次是否迁移

进入第二步后，只有服务器找到并校验通过 SQLite 快照，且目标需要创建/初始化时，页面才显示：

- **迁移本地数据**；
- **跳过本地数据**。

选择通过受 setup token 保护的接口原子写入运行 YAML，只能记录一次。页面刷新或任务失败后会显示已记录结果，不再次询问。若确需改变，必须停止安装任务，由管理员人工编辑运行配置中的 `legacy_migration_decision`，并重新承担变更审批。

完整 PostgreSQL 已存在时不会重复迁移；SQLite 文件不存在时不会显示问题。

## 4. 自动迁移事务

选择“迁移本地数据”后，服务按以下顺序执行：

1. 以 `mode=ro&immutable=1` 打开服务器固定路径；
2. 再次运行 SQLite `quick_check` 与外键检查；
3. 若目标数据库不存在，在事务外创建数据库；创建成功但后续失败时不会自动删除该数据库；
4. 在一个 PostgreSQL 事务中创建当前表、约束与索引，但暂不插入默认 seed；
5. 按应用模型白名单和依赖顺序，以 500 行为一批复制存在的源表；
6. 对每张复制表核对源/目标行数；任一不一致则回滚整个数据与 schema 事务；
7. 重置应用拥有的单列自增主键 sequence；
8. 插入幂等 RBAC/方案模板 seed，重建并验证当前读模型，清洗旧配置版本中的明文凭据；
9. 提交前在同一事务中执行 `ANALYZE`，让 PostgreSQL 采集新数据统计信息；
10. 原子保存最终 PostgreSQL DSN，清除 setup-only 的 SQLite 路径与选择，退出安装模式。

页面轮询服务端任务状态，显示阶段、当前表、完成表数和累计行数。关闭页面不会取消服务端任务。任务失败后 SQLite 保持只读，PostgreSQL 事务回滚；若数据库已在事务前创建，它会保留为空库供安全重试。

## 5. 附件与业务验收

自动迁移不搬附件。保持 `demand_attachments.storage_path` 所对应的相对布局，把旧附件复制到新的持久卷，再核对：

- 附件清单文件数、总大小、逐文件 SHA-256；
- 每张迁移事实表的源/目标行数；
- 登录、用户和权限；
- 配置读取、Jira 日审、排期/任务、方案、KPI；
- BLOB/压缩 payload 抽样哈希；
- 时间字段按 UTC 解释后的边界样本；
- 一条可回滚测试写入不会触发 duplicate key，证明 sequence 与读模型正常；
- `config_versions` 不含已知 token、密码或 DSN 明文。

验收前生成 PostgreSQL custom-format 备份，并在 staging 实际恢复一次：

```bash
pg_dump --format=custom --file="$migration_dir/target-accepted.dump" \
  "service=well_ambient_target"
pg_restore --list "$migration_dir/target-accepted.dump" \
  > "$migration_dir/target-accepted.list"
sha256sum "$migration_dir/target-accepted.dump" \
  > "$migration_dir/target-accepted.dump.sha256"
```

参见 PostgreSQL 官方 [`pg_dump`](https://www.postgresql.org/docs/current/app-pgdump.html) 与 [`pg_restore`](https://www.postgresql.org/docs/current/app-pgrestore.html)。

## 6. PostgreSQL 索引验收

当前版本增加了与实际读查询对应的 PostgreSQL 专用部分、表达式、排序和 `INCLUDE` 覆盖索引。索引只能在真实数据分布与真实 planner 上验收；不要仅以 DDL 创建成功作为结论。

迁移后先检查统计信息和索引使用情况：

```sql
ANALYZE VERBOSE task_telemetries;
ANALYZE VERBOSE solution_catalog_entries;

SELECT relname, indexrelname, idx_scan, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
ORDER BY relname, indexrelname;
```

对 Jira 日审、方案目录分页、方案 revision/source/job/comparison 查询保存迁移前后：

```sql
EXPLAIN (ANALYZE, BUFFERS, SETTINGS) <真实参数化查询>;
```

重点检查实际耗时、shared hit/read、扫描行数与返回行数、是否发生额外排序，以及部分索引谓词是否被 planner 证明。多列索引最依赖前导列约束；覆盖索引会增加存储与写放大，因此本轮只包含窄字段。参见 PostgreSQL 官方 [多列索引](https://www.postgresql.org/docs/current/indexes-multicolumn.html)、[Index-only scans 与覆盖索引](https://www.postgresql.org/docs/current/indexes-index-only-scans.html)、[统计视图](https://www.postgresql.org/docs/current/monitoring-stats.html) 和 [`ANALYZE`](https://www.postgresql.org/docs/current/sql-analyze.html)。

## 7. 切流与回退

切流前保持旧系统停写且 SQLite 快照只读。安装页完成并进入登录页后再开放外部流量；轮换 setup token，保留迁移报告、源快照和目标备份至审批保留期结束。

回退时：

1. 关闭新系统入口并停止 PostgreSQL 写入；
2. 保留失败现场并生成目标库快照；
3. 校验旧 SQLite 快照与附件 SHA-256；
4. 恢复旧版本和附件挂载，先只读冒烟；
5. 获批后恢复旧系统写入；
6. 人工制定 PostgreSQL 切流后新增数据的回放方案，禁止双写。

应用版本回滚不会自动回滚数据库。不要用 `deploy/rollback.sh` 代替数据回退，也不要对未确认目标执行 `pg_restore --clean`。

## 迁移交付物

上线审批至少归档：应用 commit、源 SQLite 版本/大小/SHA-256、目标 PostgreSQL 版本、逐表行数、任务阶段日志摘要、附件校验、sequence 写入验证、索引 `EXPLAIN` 前后证据、staging 恢复记录、业务冒烟、目标备份引用、切流/回退时间线和批准人。
