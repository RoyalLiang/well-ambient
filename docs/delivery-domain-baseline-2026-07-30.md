# 交付领域只读数据基线（2026-07-30）

## 生成边界

- 数据源：工作区当前 `well-ambient.db`
- 读取方式：SQLite `mode=ro` 与 query-only 连接；仅执行 `PRAGMA table_info` 和 `SELECT`
- 数据库 SHA-256：`4ae729ece61bc3d0e57ba47a4afe66782d1286a612ae024609a720b0a9e8c64d`
- 未执行 AutoMigrate、回填、Jira 写回或业务数据更新
- 可重复命令：`go run ./cmd/delivery-audit --database well-ambient.db`

## 数量基线

| 指标 | 数量 |
|---|---:|
| `task_telemetries` 总数 | 655 |
| 需求 `demand` | 157 |
| 缺陷 `bug` | 486 |
| 执行任务 `task` | 12 |
| 已配置项目 | 36 |
| ID 前缀可唯一匹配项目 | 648 |
| 项目无法确定 | 7 |
| 有 `task_group_id` | 6 |
| 无 `task_group_id` | 649 |

当前 schema 尚无 `project_key`，且 36 个项目配置的 `git_repos_json` 都为空数组，因此本基线不能通过仓库映射补充项目归属。6 条带任务组记录全部是需求；486 条 Bug 和 12 条执行任务都没有任务组关系。

## 状态基线

| 状态 | 数量 |
|---|---:|
| `backlog` | 351 |
| `done` | 266 |
| `progress` | 36 |
| `review` | 2 |

## 无法自动归属项目的记录

| Task ID | 类型 | 当前 `Repo` | 原因 |
|---|---|---|---|
| `DEMAND-001` | demand | `PRJ23096` | `DEMAND` 不是项目目录 Key，且无仓库映射 |
| `YBET-114` | demand | `PRJ26100-宜宾港E-truck项目 (YBET)` | `YBET` 尚未进入项目目录，且无仓库映射 |
| `YBET-121` | demand | `PRJ26100-宜宾港E-truck项目 (YBET)` | 同上 |
| `YBET-127` | bug | `PRJ26100-宜宾港E-truck项目 (YBET)` | 同上 |
| `YBET-137` | demand | `PRJ26100-宜宾港E-truck项目 (YBET)` | 同上 |
| `actual-4` | task | `tos_interface_general` | `ACTUAL` 不是项目目录 Key，且无仓库映射 |
| `e-03` | task | `task_executor` | `E` 不是项目目录 Key，且无仓库映射 |

这些记录必须保留在人工处理清单中；迁移器不得猜测或删除它们。`YBET` 是否补入项目目录、`DEMAND-001` 是否归属 `PRJ23096`，以及两个本地执行任务的父交付项，均需要人工确认。

## Dirty worktree 边界

当前工作区包含大量既有未提交的后端、前端、文档、运行时数据库和验证截图。领域收敛实施只在计划列出的文件中做增量修改，不回退或覆盖其他改动；`well-ambient.db`、`task_status.md`、`internal/server/task_status.md` 和既有 `output/` 文件不进入本次源代码提交边界。
