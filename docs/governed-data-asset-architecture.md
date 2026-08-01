# 可治理、可追溯数据资产架构

## 1. 目标

该架构为需求、缺陷、执行任务、代码证据、评论、决策、报表和模型分析建立统一的追溯骨架，同时保留各领域写模型的业务所有权。

必须满足：

- 原始事实可定位来源、时间、操作者和完整载荷。
- 历史记录不可覆盖；纠错、撤销和删除声明通过追加事件表达。
- 来源重放幂等，同一幂等键的内容漂移显式失败。
- 报告和模型结论绑定输入水位、证据、生产者及版本。
- 高频时间线查询不读取大 JSON/BLOB，数据增长后复杂度保持为索引定位加一页结果。
- 数据分级、保留等级和到期时间可查询，但未经过审批的清理任务不得删除资产。

本阶段不做：

- 不把数据资产账本变成需求、版本或任务的第二个当前状态写模型。
- 不默认回填所有历史大载荷，不执行生产数据迁移或清理。
- 不引入未经当前 SQLite 运行环境验证的分布式事件总线或 PostgreSQL 专属分区。
- 不让模型输出直接成为事实或绩效裁决。

## 2. 事实层次

1. **源事实**：Jira 更新、Git push、评论、人工排期等来源事实。
2. **领域事件**：交付领域完成校验后产生的业务变化，例如 `planning_state_changed`。
3. **资产事件**：对源事实或领域事件的规范化、不可变、跨来源索引记录。
4. **投影快照**：在 `as_of` 与输入高水位下生成的指标、日报、周报或月报。
5. **分析运行**：规则或模型针对一个确定快照产生的有版本推断。
6. **决策结果**：人或受控自动流程采纳分析后产生的新领域事件。

这些层次不能合表解释：资产事件不是当前状态，快照不是源事实，分析结论不是最终事实。

## 3. 深模块接口

`internal/dataassets.Module` 是调用方和测试共同使用的接口面：

- `Append(ctx, command)`：规范化、校验、哈希、可选压缩、幂等检查并原子追加事件与载荷。
- `Timeline(ctx, query)`：在固定高水位下按 `(occurred_at, id)` 反向游标读取热元数据。
- `Load(ctx, id)`：按 ID 读取冷载荷、解压并校验哈希。
- `SealSnapshot(ctx, command)`：保存投影/报告/分析快照及其证据关系。
- `LoadSnapshot(ctx, id)`：读取一个历史快照及证据身份。
- `LatestSnapshot(ctx, query)`：在明确种类和范围内读取最新快照。

模块内部使用真实 SQLite/GORM 适配器；当前没有第二种数据库适配器，因此不暴露假想 repository port。

## 4. 存储布局

### 4.1 热事件元数据 `data_asset_events`

保存：

- 幂等键、完整指纹和载荷哈希
- 项目、主体类型/身份、事件类型
- 来源系统、来源记录、来源事件
- 操作者与角色、关联/因果/被更正事件
- 发生时间、观察时间、记录时间
- 数据分级、保留等级、到期时间、Schema 版本
- 载荷编码、原始大小和存储大小

该表不保存原始 JSON/BLOB。

### 4.2 冷事件载荷 `data_asset_event_payloads`

以事件 ID 一对一保存载荷。超过阈值的 JSON 使用快速 gzip；哈希始终针对解压后的规范 JSON，读取时重新校验。

### 4.3 快照元数据与载荷

`data_asset_snapshots` 保存种类、范围、`as_of`、输入高水位、生产者/版本、证据数量/哈希、分级和保留策略；`data_asset_snapshot_payloads` 保存冷载荷。

`data_asset_snapshot_evidence` 保存快照到资产事件的有序证据关系。模型或报告中的每项结论应通过该关系回到不可变事件。

## 5. 时间语义

- `occurred_at`：事实在来源系统实际发生的业务时间。
- `observed_at`：本系统第一次获得该事实的时间。
- `recorded_at`：资产账本成功持久化的时间。
- `as_of`：投影快照允许纳入事实的截止时间。
- `input_high_watermark`：快照读取时已纳入的最大资产事件 ID。

迟到事件可以具有较早的 `occurred_at` 和较新的 ID。正在翻页的查询固定高水位，不会中途插入迟到事件；新一轮查询会看到它。

## 6. 幂等与完整性

- 幂等键由来源语义决定，例如 `work_item_event:<id>` 或 `jira:<issue>:<update-id>`。
- `payload_hash` 只证明载荷内容；`fingerprint` 还覆盖规范化元数据、时间、关系、分级和保留策略。
- 同一幂等键与同一指纹返回既有事件并标记 replay。
- 同一幂等键与不同指纹返回冲突，不静默覆盖。
- 调用方省略 `observed_at` 时，首次写入仍保存真实观察时间，但幂等指纹使用稳定的“系统补时”标记，跨时重试不会产生伪冲突。
- SQLite 更新/删除/重复插入触发器保护事件、快照、载荷和证据关系，阻断 `INSERT OR REPLACE` 绕过；应用模块本身不暴露修改或删除方法。

## 7. 大量数据查询策略

### 7.1 热冷垂直分离

时间线只查询窄元数据表，不 JOIN 冷载荷。原始评论、Webhook、before/after、报告正文和模型输出仅在详情取证时读取。

### 7.2 稳定 keyset cursor

- 排序固定为 `occurred_at DESC, id DESC`。
- 首屏获取过滤范围内的 ID 高水位。
- 后续页使用 `id <= high_watermark` 与 `(occurred_at < ? OR (occurred_at = ? AND id < ?))`。
- cursor 同时携带过滤指纹，禁止跨查询复用。
- 页大小默认 100、最大 200；读取 `limit + 1` 判断下一页，不执行全量 count。

这使常规查询复杂度接近 `O(log N + page_size)`，避免 `OFFSET N` 随页数线性变慢。

### 7.3 复合索引

热路径索引按左前缀覆盖：

- 主体时间线：`subject_type, subject_id, occurred_at, id`
- 纯时间范围：`occurred_at, id`
- 项目时间线：`project_key, occurred_at, id`
- 来源系统时间线：`source_system, occurred_at, id`
- 来源记录时间线：`source_system, source_record_id, occurred_at, id`
- 事件类型时间线：`event_type, occurred_at, id`
- 关联链：`correlation_id, occurred_at, id`
- 高水位与回放：`recorded_at, id`
- 保留扫描：`retention_class, expires_at, id`
- 快照版本：`kind, scope_type, scope_id, as_of, id`

### 7.4 载荷和批处理

- 大载荷在写入时按阈值压缩，列表不解压。
- 快照证据批量写入并限制单次最大数量。
- 历史回填必须按主键游标分批，并使用幂等键重复运行；禁止一次性把全库载入内存。

### 7.5 未来分区

当单机 SQLite 的写并发、文件大小或保留清理超过运行预算时，可按 `recorded_at` 月份迁移到原生分区数据库。因为模块接口、时间字段、范围字段和 cursor 不变，调用方不需要感知分区。

## 8. 首个真实接入

`deliveryplanning.ApplyPlanningChange` 与 Jira 版本 reconcile 已在数据库事务中写入 `WorkItemEvent`。本阶段在同一事务内追加对应资产事件：

- 主体：`work_item/<task_id>`
- 来源：原领域事件的 `source`
- 来源事件：`work_item_event/<id>`
- 载荷：before、after、reason、revision、sync state
- 幂等键：`work_item_event:<id>`

任何一个写入失败都会回滚当前状态、领域审计、outbox 和资产事件，不产生双写裂缝。

## 9. 报告与模型分析

日报、周报、月报和成员成长报告在正式保存时必须调用 `SealSnapshot`，记录：

- `as_of` 与输入高水位
- 统计口径/Prompt/模型版本
- 生产者类型与版本
- 证据事件集合和证据哈希
- 数据分级、保留等级、生成者
- 完整输出载荷

确定性指标先生成快照，大模型只基于该快照和权限过滤后的证据生成分析；人工修订产生新快照或后续决策事件，不改写旧输出。

封存时会拒绝高于账本真实最大 ID 的输入水位，也会拒绝 `occurred_at > as_of` 的证据，避免周报/月报引用未来事实。当前阶段已经提供封存模块和读取接口，但尚未把现有周报/月报生产路径切换到 `SealSnapshot`；该切换以及 Jira 评论、GitLab 原始证据接入属于后续增量阶段，不能把基础能力误报为完整分析产品。

## 10. 迁移顺序

1. 新表与索引 additive migration。
2. 新规划事件同事务双记领域审计与资产账本。
3. 以游标和幂等键分批回填历史 `WorkItemEvent`，先 dry-run 后 apply。
4. 对比领域事件数量、哈希和时间范围，输出缺失/冲突清单。
5. 报告保存切换到不可变快照；实时预览仍可临时计算但不得冒充历史报告。
6. 稳定观测后逐步接入 Jira 评论、GitLab 证据和其他来源。

历史规划事件回填命令：

```bash
# 只读、流式 dry-run；不会创建表或写数据库
go run ./cmd/data-assets-migrate --database well-ambient.db --batch-size 200

# apply 必须先生成一个不存在的新备份文件
go run ./cmd/data-assets-migrate \
  --database well-ambient.db \
  --backup well-ambient.before-data-assets.db \
  --batch-size 200 \
  --apply
```

`--after-id` 用于按 `WorkItemEvent.id` 恢复；批大小最大 500。命令不会一次性读取全部历史事件。

apply 前的备份使用 SQLite `VACUUM INTO` 生成事务一致快照并执行 `PRAGMA quick_check`，能够包含已提交但尚未 checkpoint 的 WAL 内容；不会对在线数据库做无锁字节复制。dry-run 使用与正式追加完全相同的规范化和 fingerprint 规则，分别报告 `existing`、`would_append` 与 `conflicts`，冲突样本最多输出 100 个幂等键。损坏的历史 JSON 会作为原字符串连同 `*_json_valid=false` 保存，不会静默替换为 `null`。

## 11. 验收

- 正确性：重放、冲突、不可变、迟到事件、同时间戳、事务回滚和载荷哈希测试通过。
- 查询：`EXPLAIN QUERY PLAN` 命中目标复合索引，不扫描 payload 表，不使用 OFFSET。
- 容量：大量元数据下读取固定页大小；测试与 benchmark 记录吞吐和分配，不用脆弱的墙钟阈值作为唯一门禁。
- 安全：只有受权角色可读取资产详情；列表默认只暴露元数据，敏感载荷不进入日志。
- 运维：本阶段只创建 additive 表/索引/触发器，不删除或改写历史业务数据。
