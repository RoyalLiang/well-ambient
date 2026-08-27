# Linux + PostgreSQL 部署手册

## 适用范围

当前交付面面向 Linux 主机，使用 Docker Engine 与 Docker Compose。Compose 只运行 `migrate`、`server` 和 `web`，不创建、启动、停止或备份 PostgreSQL。生产 PostgreSQL 由服务器现有实例、其他容器栈或托管服务提供；SQLite 只保留给本地开发、测试和首次历史数据迁移。

首次安装页可以迁移一个由服务器固定路径指定的只读 SQLite 快照。该能力不搬附件，也没有在本开发机完成真实 PostgreSQL 验证；需要保留旧数据时，先完整执行 [SQLite → PostgreSQL 上线迁移指南](./sqlite-to-postgresql-migration-guide.md)，并在 Linux staging 用生产快照演练。

## 首次准备

主机需要：

- Linux AMD64 或 ARM64、Docker Engine、Docker Compose v2 和 `curl`；
- 已部署并已备份的 PostgreSQL 14+，应用容器能通过受控网络访问；
- 可从镜像仓库拉取 server/web 镜像，或已通过 `docker load` 导入镜像；
- 由宿主机或上游负载均衡器终止 TLS。Compose 只暴露应用 HTTP 端口。

## 自动发布元数据与一条命令

默认发布不再要求输入版本、日期或批次内容。`scripts/release-metadata.sh` 一次生成 `deploy/generated/release.env` 和 `release-notes.txt`，server/web 镜像、Compose 包、离线镜像包与部署状态共用这份元数据：

- 干净工作树使用 Git 提交日期和短 SHA，例如 `2026.08.27-58e6467`；
- 未提交工作树自动追加 `dirty` 和 UTC 构建时刻，避免不同内容共用同一标签；
- 批次说明取最近 Git 标签之后的提交；无标签时取最近 20 个提交；
- 两个镜像写入相同的 OCI version、revision 和 created 标签；
- 离线镜像包使用 gzip 压缩，避免 `docker save` 的未压缩 tar 被误认为镜像本身超过 1 GB。

完整校验、构建镜像并生成 Compose/离线镜像两个交付包：

```bash
make release
```

在已配置 `deploy/.env.production` 的同一台主机上构建并部署：

```bash
make deploy
```

特殊发布仍可用 `VERSION`、`BUILD_TIME` 或 `WELL_AMBIENT_RELEASE_BATCH` 覆盖自动值，但日常发布无需填写。

服务器建议使用以下目录：

```text
/opt/well-ambient/
├── compose.yaml
└── deploy/
    ├── .env.production
    ├── config.production.example.yaml
    ├── deploy.sh
    ├── rollback.sh
    └── runtime/
        ├── config.yaml
        └── data/
            ├── attachments/
            └── legacy/
```

推荐先在仓库根目录生成不含源码、数据库和秘密的 Compose 部署包：

```bash
make compose-bundle
. deploy/generated/release.env
scp "deploy/bundles/well-ambient-compose-${WELL_AMBIENT_VERSION}.tar.gz" \
  deploy@your-server:/tmp/well-ambient-compose.tar.gz
```

服务器上预先创建由部署用户持有的目录，然后解压：

```bash
sudo install -d -o deploy -g deploy -m 0750 /opt/well-ambient
sudo -u deploy tar -xzf /tmp/well-ambient-compose.tar.gz \
  -C /opt/well-ambient
cd /opt/well-ambient
```

若不使用部署包，也可按相同相对路径搬运 `compose.yaml`、`deploy/.env.production.example`、`deploy/config.production.example.yaml`、`deploy/deploy.sh` 和 `deploy/rollback.sh`。随后执行：

```bash
cp deploy/.env.production.example deploy/.env.production
mkdir -p deploy/runtime/data/legacy
cp deploy/config.production.example.yaml deploy/runtime/config.yaml
chmod 600 deploy/.env.production deploy/runtime/config.yaml
```

编辑 `deploy/.env.production`：

- `WELL_AMBIENT_SERVER_IMAGE` 和 `WELL_AMBIENT_WEB_IMAGE` 指向服务器能取得的镜像仓库；离线导入时填写 `well-ambient-server` 和 `well-ambient-web`；
- 不再填写 `WELL_AMBIENT_VERSION`；版本、UTC 构建日期和批次说明由 Git 自动生成并随部署包写入 `deploy/generated/`；
- `WELL_AMBIENT_SETUP_TOKEN` 可留空，让程序在首次安装模式生成一次性令牌；也可填写与数据库密码不同且至少 32 个字符的高熵随机值。显式令牌不会被程序回显或写入临时文件；
- `APP_UID`、`APP_GID` 应与服务器上 `deploy/runtime` 的所有者一致；
- `HTTP_BIND` 默认是 `127.0.0.1`，供同机 TLS 反向代理使用；只有防火墙和 TLS 边界明确时才改为外部地址；
- `HTTP_PORT` 是 Linux 主机暴露端口；
- 不把这个文件提交到 Git。

首次部署保持 `deploy/runtime/config.yaml` 中的 `database.driver: setup`。若需要迁移历史数据，把停写后的 SQLite 快照安装为 `deploy/runtime/data/legacy/well-ambient.db`，不要复制仍在写入的工作文件。数据库连接由浏览器安装页写入 bootstrap 配置；它不进入运行时配置 API、配置版本档案或镜像。GitLab、Jira、飞书和 AI 仍在登录后的配置中心按需开启。

## 把镜像交付到服务器

Compose 不再包含 `build:`，所以服务器不需要项目源码，但必须能取得相同版本的两个镜像。

使用镜像仓库时，在构建机执行：

```bash
make images PLATFORM=linux/amd64 \
  SERVER_IMAGE=registry.example.com/your-team/well-ambient-server \
  WEB_IMAGE=registry.example.com/your-team/well-ambient-web
. deploy/generated/release.env
docker push "registry.example.com/your-team/well-ambient-server:${WELL_AMBIENT_VERSION}"
docker push "registry.example.com/your-team/well-ambient-web:${WELL_AMBIENT_VERSION}"
```

将真实仓库地址写入服务器的 `.env.production`；版本由部署包携带，不再重复填写。使用离线交付时，在与服务器架构一致的构建机执行：

```bash
make image-bundle PLATFORM=linux/amd64
. deploy/generated/release.env
scp "deploy/bundles/well-ambient-images-${WELL_AMBIENT_VERSION}.tar.gz" \
  deploy@your-server:/opt/well-ambient/well-ambient-images.tar.gz
```

然后在服务器导入：

```bash
docker load -i well-ambient-images.tar.gz
```

ARM64 服务器将 `PLATFORM` 改为 `linux/arm64`。不要把 AMD64 镜像搬到 ARM64 主机后再声明部署完成。

## 首次用 Compose 启动

先验证最终配置，不会启动容器：

```bash
set -a
. deploy/generated/release.env
set +a
docker compose --env-file deploy/.env.production config
```

首次启动只需要 server 和 web：

```bash
docker compose --env-file deploy/.env.production up -d --wait server web
```

也可以把 `deploy/deploy.sh` 一并搬到相同目录结构后执行一键部署：

脚本默认读取部署包中的自动版本、构建日期和批次说明，不需要位置参数：

```bash
./deploy/deploy.sh
```

### 获取自动生成的安装令牌

若 `WELL_AMBIENT_SETUP_TOKEN` 留空，程序会在首次安装模式生成令牌，将它写入容器 `/tmp` 下权限为 `0600` 的临时文件，并在服务终端输出令牌和路径。`deploy/deploy.sh` 会把这两行服务日志显示在当前终端。安装完成或安装进程正常退出后，临时文件会被删除。

也可再次查看服务日志：

```bash
docker compose --env-file deploy/.env.production logs --tail 100 server
```

自动生成的完整令牌会进入 Docker 日志；当前 Compose 将日志限制为最多 5 个、每个 20 MB。完成安装后应限制主机日志读取权限。显式配置的令牌不会被程序回显或写入临时文件。

脚本依次执行：

1. 读取并显示自动生成的版本、UTC 构建日期和批次说明，再校验工具、镜像地址、安装令牌和端口；
2. 验证 Compose 渲染结果；
3. 启动只提供健康检查和 `/api/setup/*` 的受限 server，以及同源 Web；
4. `/ready` 返回 `SETUP` 后记录版本并退出，等待管理员完成页面配置。

通过 HTTPS 管理入口或 SSH 隧道打开页面。普通登录页不会先闪现，页面会先检查数据库状态。第一步填写 PostgreSQL 主机、端口、目标库、维护库（通常为 `postgres`）、用户、密码、SSL 和 `WELL_AMBIENT_SETUP_TOKEN`：

- 新环境的目标数据库应不存在。连接测试先连维护库，明确显示“目标数据库不存在”，同时检查当前角色是否具有 `CREATEDB`；
- 若目标数据库已存在，服务只接受完全空的 schema 或当前核心表/列、读模型 generation 与 PostgreSQL trigger 均完整的 Well Ambient schema；
- 必须先用当前表单完成连接测试，信息变化后要重新测试；
- PostgreSQL 位于同一台 Linux 主机时，页面主机填写 `host.docker.internal`，不能填写 `127.0.0.1`；Compose 已将该名称映射到 host gateway。PostgreSQL 仍需监听容器可达地址，并在 `pg_hba.conf` 中只允许所需 Docker 网段和角色。
- PostgreSQL 位于其他服务器或共享容器网络时，填写其受控 DNS/IP；生产环境建议 `verify-full`。CA 文件可放在 `deploy/runtime/ca.pem`，页面填写容器路径 `/etc/well-ambient/ca.pem`。

第二步仅在服务器发现配置路径下的 SQLite 快照时询问一次“迁移本地数据”或“跳过本地数据”。选择会先持久化，刷新页面或迁移失败后不会重复询问。迁移任务在服务端异步执行，页面显示阶段、当前表、表数和行数；关闭页面不会取消任务。

提交成功后，连接串原子写入 `deploy/runtime/config.yaml`，文件权限保持 `0600`。setup server 退出，由 Compose 重启为正常 server；安装 API 随即消失，页面刷新到登录入口。数据库短暂断连只会让 readiness 失败，不会重新开放安装页。

后续版本部署前，必须先在现有 PostgreSQL 平台创建并验证快照或备份。应用脚本不会接管外部数据库备份；没有备份引用时会失败关闭。完成备份后按本次备份 ID 显式运行：

```bash
WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE=provider-snapshot-20260826-001 \
  ./deploy/deploy.sh
```

脚本只记录外部备份引用，然后执行一次 `--migrate-only` 和 server/web 切换，不会伪装成自己已经备份 PostgreSQL。

部署状态、当前/上一版发布元数据、运行配置和备份分别位于 `deploy/.state/`、`deploy/runtime/`、`deploy/backups/`，都已加入 `.gitignore`。应用回滚时，版本、release env 和批次说明会一起切换。容器日志默认轮转为单文件 20MiB、最多五份，避免长期运行耗尽系统盘。

## 健康检查与验收

```bash
curl -fsS http://127.0.0.1:8080/live
curl -fsS http://127.0.0.1:8080/ready
curl -fsS http://127.0.0.1:8080/api/status
docker compose --env-file deploy/.env.production ps
docker compose --env-file deploy/.env.production logs --tail=200 migrate server web
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

本开发机没有 Docker 和 PostgreSQL 服务，当前只能完成 Go/前端、脚本、YAML 与静态容器合同验证。首次投产前仍需在目标 Linux 主机完成镜像拉取或导入、Compose 启停、容器到现有 PostgreSQL 的网络连接、真实迁移、备份恢复、SIGTERM 优雅退出、磁盘满/数据库断连和回滚演练。
