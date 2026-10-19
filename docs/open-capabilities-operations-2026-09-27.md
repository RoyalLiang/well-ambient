# 运行开放能力

日期：2026-09-27。状态：本地实现与自动化验证完成，尚未签发生产凭证、迁移生产数据库、连接生产 Jira，或完成目标客户端验收。

## 先理解运行边界

HTTP、MCP 和内部最强大脑共享 `openaccess`、`jiraquery`、`decisioncommands` 和 `reviewread` Module。

- Integration Source 只表示来源和配额，不代表 Jira 用户。
- 所有有效 Key 使用同一 active Integration Access Policy。
- Jira 写操作使用系统配置的 Jira Execution Binding。
- `succeeded` 只表示写后读取 Jira 时确认了期望值。
- `unknown` 不会自动重试。

## 统一 Skill 与调用观测

对外只安装 `well-ambient`。其中包含 `jira-analysis`、`board-decision`、`code-review-reader`、`runtime-knowledge` 四个子技能，按任务加载 `modules/<子技能>/WORKFLOW.md`，不会注册为四个顶级 Skill。

入口为「AI 治理 → Skill 中心」（`?tab=ai_governance&section=skill_registry`）。页面提供统一安装包、文档、子技能发布开关、近 24 小时调用统计与最近 20 条审计记录。全局超级管理员可查看观测；普通有读取权限的用户可查看目录、文档和安装说明。发布修改仍需 `config:write`。

MCP 连接名固定为 `well-ambient`，13 个工具统一为 `well-ambient-<原工具名>`，例如 `well-ambient-knowledge_get_context`。不再支持自定义前缀；旧配置中的 `well` 可在保存时迁移，其他旧前缀应改为 `well-ambient`。HTTP 路径、权限 action 与数据库审计的 canonical tool ID 保持现有协议；工具目录的 `public_name` 明确给出客户端名称。

客户端升级时重新运行统一安装器，并重新加载 Skill 和 MCP 工具列表。旧技能安装 URL 与 `--skill jira-analysis` 等选择器会转向父包。安装器先校验和暂存，再替换 `well-ambient`，避免撤下子技能残留；旧包与可识别的旧独立目录移到 `<客户端根目录>/skill-backups/`。不识别的同名目录会保留并提示人工核对。备份可用于恢复本地修改，不要把备份复制回 skills 目录后同时启用新旧入口。

发布开关只控制分发，不撤销已发凭证或工具权限。已安装的离线副本需要重新安装才同步撤下内容。调用统计按审计中的工具归类，客户端直接调用工具也会计入；它不能证明某一客户端已经装载或实际触发某份 Skill。待回写记录不代表仍在运行。知识整理显示服务已同步数据的最近检查、快照、下一次到期与失败情况，默认每 6 小时运行。

分发接口：`GET /open/v1/skills` 返回一个父包及四个 children；主文档、manifest 和 archive 位于 `/open/v1/skills/well-ambient/`；子文档位于 `/open/v1/skills/well-ambient/subskills/<子技能>/skill.md`。内部观测位于 `GET /api/ai-governance/open-skills/overview`，不随公共目录公开。

Docker 镜像需包含统一父包、四份子技能来源和 MCP manifest。本次变更只在本地验证；发布应用后才会在线上显示入口与新工具名。知识初始化与 schema 部署仍遵循下方迁移流程。

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

CLI 的 `-ttl` 必须是正时长。治理页面与管理 API 另支持永久 Key（`ttl_hours <= 0`）；不能把 CLI 限制理解为系统禁止永久 Key。签发时按集成来源的实际需要选择有效期。

安装脚本和客户端安装链接仅分发占位配置，不携带真实 Key：

```bash
curl -fsSL 'https://ambient.example.com/open/v1/install.sh' | bash
```

将示例地址替换为部署的公开地址。安装后在客户端本地把 `Authorization: Bearer <YOUR_API_KEY>` 替换为真实 Key。`install.sh` 拒绝所有查询参数，旧的 `?token=...` 安装命令会返回 HTTP 400。完整 Key 仅在签发结果中显示一次；关闭弹窗即清除页面中的结果，请先保存。

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


### Skill 分发启用状态

Skill 仓库分别展示用途、包含工具、启用状态、版本与操作。具备 `config:write` 权限的管理员可以切换官方 Skill 的启用状态；其他账号只能查看。

启用状态与统一名称前缀一起保存在数据库的版本化运行配置中，停用列表采用稳定的官方 Skill ID，修改前缀不会重置状态。未设置过的 Skill 默认启用。停用后，列表与文档仍可查看，但安装包下载返回 403，默认批量安装脚本跳过该 Skill；显式指定停用 Skill 时脚本会提示“技能已停用”。已安装的本地文件不会自动删除，MCP 工具继续使用独立的服务开关与数据库授权策略。
