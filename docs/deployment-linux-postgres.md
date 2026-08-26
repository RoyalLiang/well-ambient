# Linux + PostgreSQL 部署手册

## 适用范围

当前交付面面向单台 Linux 主机，使用 Docker Engine 与 Docker Compose。生产数据存储使用 PostgreSQL；SQLite 只保留给本地开发和测试。BuildKit 会按目标主机架构构建不依赖 CGO 的生产 server 二进制；首个正式环境仍应按实际 AMD64/ARM64 架构单独验收。

首次安装页可以迁移一个由服务器固定路径指定的只读 SQLite 快照。该能力不搬附件，也没有在本开发机完成真实 PostgreSQL 验证；需要保留旧数据时，先完整执行 [SQLite → PostgreSQL 上线迁移指南](./sqlite-to-postgresql-migration-guide.md)，并在 Linux staging 用生产快照演练。

## 首次准备

主机需要：

- Linux AMD64 或 ARM64、Docker Engine、Docker Compose v2、`curl`、`git`；
- 至少为 PostgreSQL 数据卷、应用附件和备份预留三倍当前有效数据量；
- 由宿主机或上游负载均衡器终止 TLS。Compose 只暴露应用 HTTP 端口，不暴露 PostgreSQL 端口。

在项目根目录执行：

```bash
cp deploy/.env.production.example deploy/.env.production
mkdir -p deploy/runtime/data/legacy
cp deploy/config.production.example.yaml deploy/runtime/config.yaml
chmod 600 deploy/.env.production deploy/runtime/config.yaml
```

编辑 `deploy/.env.production`：

- `POSTGRES_PASSWORD` 必须改成足够长的随机值；为避免 DSN 转义歧义，只使用 `A-Z a-z 0-9 . _ ~ -`；
- `WELL_AMBIENT_SETUP_TOKEN` 使用与数据库密码不同的高熵随机值，至少 32 个字符；它只授权首次数据库安装写操作；
- `HTTP_BIND` 默认是 `127.0.0.1`，供同机 TLS 反向代理使用；只有防火墙和 TLS 边界明确时才改为外部地址；
- `HTTP_PORT` 是 Linux 主机暴露端口；
- 不把这个文件提交到 Git。

首次部署保持 `deploy/runtime/config.yaml` 中的 `database.driver: setup`。若需要迁移历史数据，把停写后的 SQLite 快照安装为 `deploy/runtime/data/legacy/well-ambient.db`，不要复制仍在写入的工作文件。数据库连接由浏览器安装页写入 bootstrap 配置；它不进入运行时配置 API、配置版本档案或镜像。GitLab、Jira、飞书和 AI 仍在登录后的配置中心按需开启。

## 一键部署

版本必须是不可变标识，例如 Git commit SHA 或发布号，不能使用 `latest`：

```bash
./deploy/deploy.sh 2026.08.26-1
```

首次运行时，脚本依次执行：

1. 校验工具、密钥占位符、端口和密码字符；
2. 构建固定版本的 server/web 镜像；
3. 启动并等待 PostgreSQL 维护库 `postgres` ready；内置容器不会预先创建应用目标库；
4. 启动只提供健康检查和 `/api/setup/*` 的受限 server，以及同源 Web；
5. `/ready` 返回 `SETUP` 后记录版本并退出脚本，等待管理员完成页面配置。

通过 HTTPS 管理入口或 SSH 隧道打开页面。普通登录页不会先闪现，页面会先检查数据库状态。第一步填写 PostgreSQL 主机、端口、目标库、维护库（通常为 `postgres`）、用户、密码、SSL 和 `WELL_AMBIENT_SETUP_TOKEN`：

- 新环境的目标数据库应不存在。连接测试先连维护库，明确显示“目标数据库不存在”，同时检查当前角色是否具有 `CREATEDB`；
- 若目标数据库已存在，服务只接受完全空的 schema 或当前核心表/列、读模型 generation 与 PostgreSQL trigger 均完整的 Well Ambient schema；
- 必须先用当前表单完成连接测试，信息变化后要重新测试；
- Compose 内置数据库默认主机为 `postgres`、端口为 `5432`；外部数据库建议 `verify-full`。CA 文件可放在 `deploy/runtime/ca.pem`，页面填写容器路径 `/etc/well-ambient/ca.pem`。

第二步仅在服务器发现配置路径下的 SQLite 快照时询问一次“迁移本地数据”或“跳过本地数据”。选择会先持久化，刷新页面或迁移失败后不会重复询问。迁移任务在服务端异步执行，页面显示阶段、当前表、表数和行数；关闭页面不会取消任务。

提交成功后，连接串原子写入 `deploy/runtime/config.yaml`，文件权限保持 `0600`。setup server 退出，由 Compose 重启为正常 server；安装 API 随即消失，页面刷新到登录入口。数据库短暂断连只会让 readiness 失败，不会重新开放安装页。

后续版本部署时，脚本执行迁移前备份、一次性 `--migrate-only`、server/web 切换和 readiness 验收。内置 PostgreSQL 自动生成并检查 custom-format dump。若页面配置的是外部 PostgreSQL，脚本会先失败关闭；完成云厂商快照或上游备份并验证后，按本次备份 ID 显式重跑：

```bash
WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE=provider-snapshot-20260826-001 \
  ./deploy/deploy.sh 2026.08.26-2
```

脚本只记录外部备份引用，不会伪装成自己已备份外部数据库。

部署状态、运行配置和备份分别位于 `deploy/.state/`、`deploy/runtime/`、`deploy/backups/`，都已加入 `.gitignore`。容器日志默认轮转为单文件 20MiB、最多五份，避免长期运行耗尽系统盘。

## 健康检查与验收

```bash
curl -fsS http://127.0.0.1:8080/live
curl -fsS http://127.0.0.1:8080/ready
curl -fsS http://127.0.0.1:8080/api/status
docker compose --env-file deploy/.env.production ps
docker compose --env-file deploy/.env.production logs --tail=200 migrate server web postgres
```

- `/live` 只证明 HTTP 进程存活；
- `/ready` 与兼容路径 `/health` 会在两秒超时内 ping 数据库，失败返回 503；
- `/api/status` 暴露 version、commit、build time，便于确认实际运行镜像。

业务验收至少覆盖：登录、配置读取、Jira 日审分页、排期/任务读取、一次可回滚的配置保存、附件读写和后台 worker 日志。

## 回滚

```bash
./deploy/rollback.sh
```

回滚只把 server/web 切回上一个已构建版本，不自动回退数据库。数据库迁移必须保持向前兼容、以新增为主；需要恢复数据库时，先停写并使用明确选定的备份人工恢复，不能由应用回滚脚本自动覆盖数据。

## SQLite 历史数据迁移门禁

正式切换已有 SQLite 数据前，执行 [独立迁移指南](./sqlite-to-postgresql-migration-guide.md)。仓库已经包含白名单、分批、事务回滚、逐表行数校验、sequence 重置和进度轮询的迁移器，但当前开发机没有真实 PostgreSQL；因此不能把模拟测试或 Compose 建库成功等同于旧数据已安全迁移。

## 密钥处置

- `config.example.yaml` 和生产示例只能出现不可用占位符；
- 运行配置和环境文件权限保持 `0600`，备份目录由脚本使用私有 umask 创建；
- 数据库密码和 setup token 只存在服务器 bootstrap 文件/请求内存，不进入浏览器缓存；首次安装成功后应轮换 setup token，但当前 Compose 仍要求保留一个有效的高熵值用于容器配置解析；
- 配置 API 只返回 `__configured__` 标记，配置版本只保存标记/摘要哈希；显式迁移会清洗旧版本中的明文秘密；
- 从仓库示例中移除值不等于撤销已经泄露的凭据。所有曾出现过的真实 token/secret 必须在 GitLab、Jira、飞书和模型供应商侧轮换；是否重写 Git 历史应另行批准并协调所有克隆。

## 尚需在真实 Linux 环境完成的验收

本开发机没有 Docker 和 PostgreSQL 服务，当前只能完成 Go/前端、脚本、YAML 与静态容器合同验证。首次投产前仍需在与生产同架构的 Linux staging 上完成镜像构建、Compose 启停、真实 PostgreSQL 迁移、备份恢复、SIGTERM 优雅退出、磁盘满/数据库断连和回滚演练。
