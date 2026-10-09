# 在 dev 开启 Jira 邮件，并构建与部署 Docker 镜像

核对日期：2026-10-03。本文对应当前工作树。没有发送真实邮件、连接外部 SMTP、构建或发布镜像、启动容器、执行数据库迁移。

## SQLite dev 需要显式开启自动邮件调度

当前已实现的 Jira 邮件是每日早报。没有独立的 dev 邮件模式，dev 不会自动把正式收件人改成测试邮箱。

1. 在邮件配置页核对 SMTP、发件人及早报收件人。开发测试时先保存期望的测试收件人。
2. 启用并保存邮件服务 `smtp.enabled`，以及每天定时发送 `daily_jira_email.enabled`。核对 `timezone` 和 `send_time`。
3. 停止原 dev 进程，在仓库根目录重新启动：

```bash
WELL_AMBIENT_ALLOW_SQLITE_EMAIL_SCHEDULER=1 make dev
```

[调度策略](/Users/eddie/Workspace/well-ambient/internal/server/email_scheduler.go:16)对 SQLite 默认禁用自动发信；环境开关接受 `1`、`true`、`yes`、`on`。PostgreSQL 默认允许调度，仍需要启用邮件服务和定时发送。这个环境开关在 worker 启动时判断，修改后应重启后端；`make dev` 继承该环境变量。

定时调度每 10 秒检查一次，只在配置时区下的发送分钟内触发。例如设为 `09:30` 时，`09:31` 及之后启动或保存配置都不会自动补发当天早报；可按需要手动发送，下一天仍正常调度。已领取的日期不会因重启而自动重复发送，也不补发过去日期。在发送分钟内已开始的任务会继续完成。不要把开启开关理解为只演练、不发送。

### 配置保存态优先于 YAML

[数据库运行态配置](/Users/eddie/Workspace/well-ambient/internal/server/config_version_handlers.go:140)覆盖 YAML 的 SMTP 和早报配置；页面保存后立即更新配置并唤醒已经启动的 worker。已有保存态时，只改 `config.yaml` 不会覆盖邮件配置。

只读检查的本地数据库快照中，SMTP 和早报已配置，快照更新时间为 2026-09-27；这不能证明当前 dev 进程使用同一个数据库。以当前进程加载的数据库及配置页为准。

| 操作 | 配置来源与限制 |
| --- | --- |
| 每日自动发送 | 已保存 SMTP、收件人和模板；需要邮件服务与定时开关，SQLite 还需要上述环境变量 |
| 手动发送 `/api/daily-jira-email/send` | 需要 `config:write`，使用已保存配置；不检查定时开关或 SQLite 自动调度开关 |
| SMTP 测试 `/api/config/test`，`type: smtp` | 使用输入草稿和单独测试收件人，真实发信，不保存草稿 |

准确配置键是 `smtp.enabled/host/port/username/password/from/tls_mode`，以及 `daily_jira_email.enabled/recipients/timezone/send_time/include_commits/template/project_groups/confluence`。远程 SMTP 必须使用 TLS。

每日早报与其他通知不同：延迟告警的 `EmailNotificationSender` 仍是日志 stub；GitLab 缺 Jira 编号的 MR 自动邮件链路当前被关闭。开启每日邮件调度不会把这两条链路变成可用发信功能。

## 一条 make 命令构建两项镜像

在仓库根目录运行：

```bash
make docker-images
```

`make docker-image` 是同一命令的别名。它复用现有 Dockerfile 的 server、web target，不推送镜像。

构建主机需要 Docker、Python 3.9+ 和 IANA 时区数据；精简主机需提供 `tzdata`，供 `Asia/Shanghai` 日期转换使用。参见 [Python 时区支持文档](https://docs.python.org/3/library/zoneinfo.html)。

完整版本名用作两个镜像的共享 tag：

```text
<名称>.<分支>.<commit号>.<yymmdd>.b<buildcount>
```

默认名称为 `well-ambient`，commit 使用实际 HEAD 前 12 位，日期按上海时间（`Asia/Shanghai`），构建次数按名称、分支、完整 HEAD 和上海日期分别计数。例如当前 `main` 分支的一次构建可以命名为：

```text
well-ambient-server:well-ambient.main.b2bd8d8c9843.261003.b1
well-ambient-web:well-ambient.main.b2bd8d8c9843.261003.b1
```

上面是命名示例，不代表镜像已经构建。Git 分支中的特殊字符逐 UTF-8 字节编码为 `_xx`，编码标记本身也转义，避免不同分支撞名；例如 `feature/a` 变为 `feature_2fa`。超过 Docker tag 的长度限制会明确失败。

| 参数 | 默认或用途 |
| --- | --- |
| `IMAGE_NAME` | `well-ambient`，版本名第一段 |
| `SERVER_IMAGE` / `WEB_IMAGE` | 两个独立镜像仓库；默认沿用现有 Makefile 配置，无生产配置时为 `well-ambient-server` / `well-ambient-web` |
| `PLATFORM` | 默认 `linux/amd64`；可指定 `linux/arm64` |
| `BUILD_COUNT` | 默认本 checkout 自动分配；CI 应传自身唯一构建号 |
| `BUILD_BRANCH` | detached CI checkout 的实际分支声明 |
| `BUILD_TIME` / `SOURCE_DATE_EPOCH` | 固定 UTC 构建时刻；命名日期转换为上海时间，用于可重现命名 |

例如，`BUILD_TIME=2026-10-03T16:00:00Z` 对应上海时间 10 月 4 日 00:00，版本日期为 `261004`，计数也进入 10 月 4 日的分组。镜像构建元数据中的 `BUILD_TIME` 仍保存同一时刻的 UTC 表达。

例如：

```bash
make docker-images IMAGE_NAME=well-ambient PLATFORM=linux/arm64
```

Dockerfile 的 `TARGETOS`、`TARGETARCH` 已改为无默认值声明，继承 BuildKit 自动目标平台参数，避免固定 amd64 与镜像目标平台不一致。[Docker 自动平台参数规则](https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope)

### 构建次数和失败记录不会互相覆盖

[构建脚本](/Users/eddie/Workspace/well-ambient/scripts/docker-images.py)使用 Python 3 标准库和 Linux/macOS 文件锁，并发分配计数。失败也保留占号；CI 重复使用已有构建号会拒绝。跨独立 runner 的唯一计数由 CI 保证，不能依赖本 checkout 的本地记录。

每次构建的独立记录位于 `deploy/generated/builds/<完整tag>/`。两项镜像都成功后，最后完成的成功构建才更新公共 `deploy/generated/release.env`。如果 web 构建失败，server 镜像可能已经存在，但公共成功记录不会更新。

仓库引用中已有的 tag 或 digest 会被新版本替换，registry 端口保留；server、web 不能使用同一仓库。构建包含当前工作树，commit 段仍是 HEAD；未提交状态会记录为 `worktree_dirty`，不能把 HEAD 当作未提交源码内容的完整标识。原 `images`、`release` 接口保持兼容。

## Compose 只填写两处 image

使用 [生产 Compose 模板](/Users/eddie/Workspace/well-ambient/deploy/compose.production.example.yaml)。填写 `server.image` 和 `web.image` 为本次构建输出的完整镜像引用；迁移服务复用 server 镜像锚点，无需再填。

模板的其他参数已给出默认值：非 root `1000:1000`、只读根文件系统、权限收敛、健康依赖、持久化挂载、日志轮转、资源限制及显式维护迁移任务。它不覆盖镜像原工作目录。健康依赖使用 `service_healthy`，符合 [Compose 启动顺序规则](https://docs.docker.com/compose/how-tos/startup-order/)；完整字段按 [Compose 服务配置](https://docs.docker.com/reference/compose-file/services/)核对。

“只填 image”指 Compose 内的留白。运行前仍需要现有 `deploy/runtime/config.yaml`、`deploy/runtime/data`、应用可达的 PostgreSQL，以及正确的宿主目录权限；配置和秘密不硬编码在模板中。目录应由 `1000:1000` 持有，模板禁止静默创建缺失目录。配置目录需要可写，以支持应用的原子配置更新。

模板只把 web 发布到 `127.0.0.1:8080`；对外访问由宿主机已有 TLS 反代或入口转发。网络保留访问 PostgreSQL、SMTP、Jira 等外部依赖的出站能力。开放能力三个开关默认关闭，按实际授权策略开启。

填写 image、准备配置与目录后，先验证：

```bash
docker compose -f deploy/compose.production.example.yaml config --quiet
```

按已有部署流程完成必要数据库引导或迁移，再启动常规服务：

```bash
docker compose -f deploy/compose.production.example.yaml up -d
```

迁移服务位于 `maintenance` profile，常规 `up` 不会自动运行。确需迁移并完成数据库变更检查后，可显式执行：

```bash
docker compose -f deploy/compose.production.example.yaml --profile maintenance run --rm migrate
```

server 使用应用自带 `/ready` 检查。web 镜像未承诺内置 curl/wget，因此探针只验证 Nginx 配置及 master 存活；端到端 HTTP/TLS 检查应由实际入口验证。CPU/内存默认值是单机部署起点，需按真实负载调整。

## 已执行的验证

- 15 项真实 Make / fake Docker 测试通过，覆盖上海午夜切换日期、UTC 午夜保持计数、固定构建时刻、并发、失败占号、分支编码、真实 HEAD、旧 tag/digest、registry 端口、dry-run、不同镜像仓库与 ARM 构建参数。
- Dockerfile 平台参数契约先失败后通过，官方 Skill 打包回归通过。
- Compose YAML、安全与结构约束通过；临时替换 image 的校验数据通过 Compose 官方 JSON Schema，源模板 image 仍留白。
- 邮件相关 74 项顶层测试、45 项子测试通过，包括过时不补发、分钟边界、次日发送、配置变更和 DST。12 项调度相关 race 测试通过；SMTP 验证仅使用本地模拟服务器，未发送真实邮件。`git diff --check` 通过。
- 本机没有 Docker CLI，未执行真实镜像构建、容器启动、原生 `docker compose config`、ARM 二进制运行或真实邮件发送。Schema 校验不能代替这些运行验证。
