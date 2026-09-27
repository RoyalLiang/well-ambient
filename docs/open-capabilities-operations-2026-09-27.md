# 运行开放能力

日期：2026-09-27。状态：本地实现与自动化验证完成，尚未签发生产凭证、迁移生产数据库、连接生产 Jira，或完成目标客户端验收。

## 先理解运行边界

HTTP、MCP 和内部最强大脑共享 `openaccess`、`jiraquery`、`decisioncommands` 和 `reviewread` Module。

- Integration Source 只表示来源和配额，不代表 Jira 用户。
- 所有有效 Key 使用同一 active Integration Access Policy。
- Jira 写操作使用系统配置的 Jira Execution Binding。
- `succeeded` 只表示写后读取 Jira 时确认了期望值。
- `unknown` 不会自动重试。

## 迁移数据库

先运行现有迁移入口：

```bash
go run ./cmd/server -config config.yaml -migrate-only
```

PostgreSQL 默认不在普通启动时自动迁移。不要在未备份和未评审 schema 的生产环境临时打开自动迁移。

## 创建来源和凭证

创建来源：

```bash
go run ./cmd/open-access-admin -config config.yaml source-create \
  -id target-agent \
  -name "Target Agent" \
  -owner "AI Platform" \
  -quota standard
```

激活统一策略：

```bash
go run ./cmd/open-access-admin -config config.yaml policy-activate \
  -file capabilities/open/policy.example.yaml \
  -actor admin@example.com
```

`data_rules.jira_issue_visibility: verified_cache` 是显式 P0 门禁。只有在确认本地同步范围已经排除不可发布的 Issue Security 记录后才能设置；缺少该值时，Jira schema、search、get 和 aggregate 全部拒绝。

`allowed_repositories` 使用权威 GitLab Project ID，不使用可重名的显示名称。

配置单事项写入绑定：

```bash
go run ./cmd/open-access-admin -config config.yaml binding-upsert \
  -project WA \
  -action reassign \
  -connector jira-primary \
  -executor jira-service

go run ./cmd/open-access-admin -config config.yaml binding-upsert \
  -project WA \
  -action reschedule \
  -connector jira-primary \
  -executor jira-service
```

签发 Key：

```bash
go run ./cmd/open-access-admin -config config.yaml credential-issue \
  -source target-agent \
  -ttl 720h
```

命令只显示一次完整 Key。系统只保存 Key ID、前缀和验证摘要。

`-ttl` 必须是正时长；首版不签发无期限 Key。

查看安全元数据或临时禁用 Key：

```bash
go run ./cmd/open-access-admin -config config.yaml credential-list \
  -source target-agent

go run ./cmd/open-access-admin -config config.yaml credential-show \
  -key-id <key-id>

go run ./cmd/open-access-admin -config config.yaml credential-status \
  -key-id <key-id> \
  -status disabled
```

list/show 只返回 Key ID、前缀、来源、状态、过期时间和使用时间，不返回 secret 或 verifier。

轮换时先签发第二个 Key，更新客户端，再撤销旧 Key：

```bash
go run ./cmd/open-access-admin -config config.yaml credential-revoke \
  -key-id <old-key-id>
```

多个轮换 Key 共享来源配额。

请求配额与并发槽保存在数据库中，不按 server 进程分别计数。每个活动请求持有可续期的来源 lease；正常结束时删除，进程崩溃后最多约 90 秒自动过期。多副本部署仍按 Integration Source 统一执行分钟窗口与并发上限。

## 分阶段打开能力

所有开放能力默认关闭。

先打开只读：

```bash
export WELL_AMBIENT_OPEN_READ_ENABLED=1
```

验证 Jira 与 Review 后打开 prepare：

```bash
export WELL_AMBIENT_OPEN_PREPARE_ENABLED=1
```

完成超时、冲突、撤销和恢复验收后打开 execute：

```bash
export WELL_AMBIENT_OPEN_EXECUTE_ENABLED=1
```

关闭 execute 会拒绝新执行请求。已经受理的 Operation 仍由 durable worker 继续核验。

打开 execute 后，旧 strongest-brain 转派/改期、Daily Jira 转派和排期页 Jira 转派/改期会返回 `409 legacy_sender_disabled`。这保证同一部署中只有 Decision Operation worker 发送这些 Jira 字段变更。

## 调用 HTTP

所有请求使用：

```text
Authorization: Bearer <integration credential>
```

契约入口：

- `GET /open/v1/jira/schema`
- `POST /open/v1/jira/issues/search`
- `GET /open/v1/jira/issues/{key}`
- `POST /open/v1/jira/aggregations`
- `POST /open/v1/decisions/context`
- `POST /open/v1/decisions/plans`
- `POST /open/v1/decisions/executions`
- `GET /open/v1/operations/{id}`
- `POST /open/v1/reviews/search`
- `GET /open/v1/reviews/{id}`

OpenAPI 位于 `capabilities/open/contracts/openapi.yaml`。

## 连接 MCP

支持自定义认证头的客户端使用：

```text
<well-ambient-origin>/mcp
```

MCP 使用无状态 Streamable HTTP。客户端应同时接受 `application/json` 和 `text/event-stream`。

不支持自定义认证头的客户端使用 stdio：

```bash
go build -o open-mcp-stdio ./cmd/open-mcp-stdio

export WELL_AMBIENT_BASE_URL="https://well-ambient.example"
export WELL_AMBIENT_API_KEY="<integration credential>"
./open-mcp-stdio
```

stdout 只承载 MCP JSON-RPC。日志写入 stderr。

## 安装 Skill

三个 Skill 位于：

- `.agents/skills/jira-analysis`
- `.agents/skills/board-decision`
- `.agents/skills/code-review-reader`

DSH 会从项目 `.agents/skills` 自动发现。其他宿主可安装生成的 Skill 压缩包，并连接同一个 MCP server。

## 查看最强大脑证据

内部管理员可读取：

```text
GET /api/strongest-brain/open-capability-intelligence
```

报告聚合工具成功率、错误码、延迟、Operation 远端确认率、unknown 数量和持续时间。建议是只读候选，不会自动修改 Key、策略、执行绑定或 Jira。

## 完成生产 P0 验收

上线前仍需提供外部证据：

1. 记录生产 Jira Data Center 版本、字段权限、Issue Security 和服务执行身份。
2. 验证允许的项目、仓库和项目到仓库的权威映射。
3. 对权威样本核对状态、负责人、分类、优先级、截止日、重开和历史完整性。
4. 选择两个目标 MCP 客户端，分别验证远程认证或 stdio。
5. 冻结 Key 有效期、来源配额、审计保留期和快照保留期。
6. 在测试环境执行转派、改期、前置冲突、远端超时、worker 崩溃、来源停用和策略收紧测试。
7. 确认旧 Jira 发送路径不会处理开放 Operation。

没有这些证据时，不要宣称生产 P2/P3 验收完成。

## 回退

1. 先关闭 `WELL_AMBIENT_OPEN_EXECUTE_ENABLED`。
2. 保持服务和 worker 运行，直到已受理 Operation 完成确认或进入 `unknown`。
3. 关闭 prepare 和 read。
4. 禁用来源或撤销 Key。
5. 不删除 Operation、Action 或 Outbox。

已确认的 Jira 变更只能通过新的补偿方案处理。
