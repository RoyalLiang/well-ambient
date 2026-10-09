# 开放能力方案核对与 AI 治理优化

核对日期：2026-10-03（Asia/Shanghai）。方案：`docs/open-capabilities-implementation-plan-2026-09-26.md`。审计基线 HEAD：`b2bd8d8c984398152cc7bdd5dc54df072f6daa16`（2026-09-27）。以下区分基线发现与本次工作区修复，修复尚未部署。

初始工作区仅 `task_status.md` 有用户修改，本次保留。没有访问生产 Jira、执行真实 Jira 写入、签发 Key、安装外部客户端或部署。

## 方案实现结论

主要本地链路已实现，但不能认定 P0–P5 全部验收完成。

| 方案项 | 基线实现与证据 | 验收边界 |
|---|---|---|
| 统一身份、来源溯源、Key 生命周期 | `internal/openaccess/openaccess.go` 提供来源、摘要认证、签发、禁用、撤销和轮换基础 | 生产来源与凭证配置未核验 |
| 版本化策略、项目范围、字段、执行绑定、来源配额 | `internal/openaccess`、`internal/jiraquery`、`internal/reviewread`、`internal/server/open_jira_adapter.go` | 生产策略、服务身份及实际敏感数据边界未验收 |
| HTTP/MCP 共用 Module，10 工具 | `internal/server/open_capability_handlers.go`、`open_mcp_adapter.go`、`internal/openmcp/server.go`、`capabilities/open/contracts/` | MCP 自动测试通过；两个目标客户端真实安装/认证验收缺少证据 |
| Jira 四工具及历史聚合 | `internal/jiraquery/module.go` | 完成状态名称仍有硬编码；当前维度与历史事件维度的口径需要业务确认 |
| Review 两工具及发布过滤 | `internal/reviewread/module.go` | 实际 GitLab 授权与报告覆盖未在生产核验 |
| 单 Issue 决策四工具、幂等、持久化 Operation/Action/Outbox、写后核验 | `internal/decisioncommands/module.go`、`internal/db/open_capabilities.go` | fake Jira/SQLite 验证，不代表真实 Jira 或 PostgreSQL 多实例验收 |
| 三个外部 Skill | `.agents/skills/jira-analysis/`、`board-decision/`、`code-review-reader/`，服务端动态归档分发 | 外部安装未验收；与治理页内部 `code_review` 的组件层级分别管理 |
| 治理开关、凭证、绑定、统计建议 | `internal/server/ai_governance_open_capability_handlers.go`、`internal/openaccess/intelligence.go`、`OpenCapabilitiesConfig.svelte` | 本次修复统计完整性与显示口径；配置关闭仍可能被环境变量启用 |
| P4 批量与多事项能力 | 当前执行仅 reassign/reschedule 单 Issue | 未实现 |
| P5 闭环演进 | 开放调用统计及建议已具备；内部 strongestbrain 有独立引擎 | 开放调用样本重评、候选版本发布闭环未完整实现 |

P0 实际基线、P2 目标客户端验收和 P3 生产迁移仍缺证据。运维文档也明确其验证限于本地实现与自动测试。

## 基线隐患及本次修复

### 1. 安装 URL 和脚本携带完整 Key（P1，已修复）

基线前端将签发结果写入 `selectedToken`，生成 `install.sh?token=...` 和带 Authorization 的 Cursor 深链；后端读取查询参数并将秘密嵌入脚本。Key 可能进入访问日志、复制历史或协议 URL。

现在脚本拒绝全部查询参数，返回无密钥脚本；所有客户端导出、深链和安装命令使用 `<YOUR_API_KEY>` 占位符，真实 Key 仅在签发弹窗显示一次，关闭即清除。安装后用户在本地填写 Authorization。公开 URL 清理用户信息、query/hash，限制 HTTP(S)，命令使用 shell 引号。

代码：`internal/server/open_skills_distribution_handlers.go`、`web/src/lib/openCapabilityInstall.ts`、`web/src/components/config/OpenCapabilitiesConfig.svelte`。Cursor 的 `name`/Base64 `config` 链接结构依据 [Cursor 官方安装链接文档](https://cursor.com/docs/mcp/install-links)。未实际唤起或安装客户端。

兼容性：旧 `install.sh?token=...` 命令返回 400；应复制新命令并在本地填 Key。

### 2. 权限收紧后仍能读取旧操作完整结果（P1，已修复）

基线 GetOperation 仅检查来源和 decision.read，没有重核当前项目范围与证据可见性。隔离实验已复现：成功后将 active policy 改为仅允许 OTHER 项目，仍可读到原项目的负责人及远端证据。

现在读取、Execute 返回及幂等重放统一复核当前 policy、项目和历史证据，使用当前 active binding 做最多 5 秒的只读 Jira 核验。允许 restricted 也必须核验当前项目及事项身份，避免事项变为 restricted 或移项后旧快照仍披露。绑定身份变化、读取失败、缺少可信证据时保守裁剪。worker 不使用该返回门禁，内部恢复与写后确认仍可继续。

项目不再发布或来源不匹配时返回 not_found。可追踪但不可完整发布时返回 `publication_status: redacted` 和固定 `missing_reason`，只保留 operation/source ID、状态、时间及 action ID/类型/状态/时间；省略 plan_id、desired/confirmed 值、remote_evidence 和错误码。完整结果标为 published。客户端不得把缺失证据理解为未执行。旧证据若未记录可信事项身份，也会保守裁剪；状态与内部存储并未被删除。

代码：`internal/decisioncommands/module.go`。公开行为补充到 `capabilities/open/contracts/openapi.yaml`。实时核验仅证明读取时的可见性，不能保证 Jira 在响应之后不再变化。

### 3. 统计保留最旧 10,000 条，隐藏最新风险（P2，已修复）

基线 Invocation 与 Operation 都 ASC 排序后 LIMIT 10000，再用返回行数作总数。已复现最新第 10,001 次失败及 unknown 被遗漏，而确认率仍为 100%。

现在数据库完整 GROUP BY 聚合，保留 30 天口径；另提供不受窗口限制的 `outstanding_unknown_count` 和 `oldest_outstanding_unknown_updated_at`。时间字段是未确认操作中最早的最后更新时间，不是进入 unknown 的精确时点。审计未终结记录有 per-tool `started_count` 和建议，成功率及平均耗时仅按已结束调用计算。started 不证明请求仍在运行，也可能是进程退出或完成审计失败后留下的待核查记录。

代码：`internal/openaccess/intelligence.go`。回归覆盖超 10,000 行、窗口外 unresolved unknown 及 started 调用。

### 4. 业务发生后审计错误被忽略（P2，已修复）

基线 HTTP/MCP 使用请求 context 在调用后写 Invocation 并忽略错误；断连/取消或数据库失败可造成业务已发生但无审计。

现在执行前持久化 started，审计准入失败则拒绝业务；完成后更新同一记录。审计使用独立于取消的 context 与 5 秒上限。同 request_id 只有身份/工具/策略匹配且仍为 started 的记录可完成，已终结记录不能覆盖，重复准入不能重新执行业务。完成审计失败明确返回 audit_unavailable，并保留 started 供核查；这类失败不能证明业务未执行，不能盲目重试写入。

代码：`internal/openaccess/openaccess.go`、`internal/server/open_capability_handlers.go`、`open_mcp_adapter.go`。这是本地持久化保证，未新增独立审计重放服务。MCP panic 记录 failed/internal_error 并继续传播异常，避免误记成功。

MCP 每个工具调用仍有独立 ID，使用传输层 X-Request-ID 加下划线和随机后缀，正常长度 58 字符。在已取得 header 或记录日志的情况下，运营可用精确前缀关联该 HTTP 请求的所有审计行；应转义 SQL LIKE 的下划线，或用 substr 精确比较前缀。断连客户端未必收到 header，不能承诺仅凭客户端现场一定能定位。

### 5. 开关及 Key 指标误导（P2，已缓解）

上方状态读取实际运行阶段 actual_features，说明配置关闭时环境变量仍可能启用；后端回退机制没有改变。Key 指标改为“标记启用的 Key”，说明有效性还取决于到期时间和来源状态，没有把 status=active 计数冒充实际认证可用数。

代码：`web/src/components/config/OpenCapabilitiesConfig.svelte`。

### 6. 交互式提示词测试可能无限等待（P2，已修复）

基线示例测试无总 deadline、无取消；浏览器一次测试等待超过 2 分钟没有结果，具体上游阻塞原因未确认。

现在方案提示词预览单次服务端 deadline 为 3 分钟，超时 504、取消 408；前端 185 秒兜底，提供“取消测试”及提示，销毁组件中止本地请求。测试期间冻结系统指令和 Markdown，防止旧输出对应新输入。取消后恢复按钮和输入。

代码：`internal/server/solution_handlers.go`、`web/src/lib/promptPreview.ts`、`web/src/components/config/SolutionPromptConfig.svelte`。其他后台模型任务及 code_review 保存版本校验不属于此交互式预览边界，未统一加总时限；全局 LLM HTTP Timeout=0 保留。

### 7. 安装脚本部分失败仍宣称全部完成（P2，已修复）

脚本现在分别跟踪实际下载/解压成功与失败的 Skill，部分失败时列出结果并非零退出。回归使用 stub curl 和临时目录，不下载或改动用户的技能。

### 8. Jira 旧事项键移项后绕过项目核验（P1，已修复）

Jira 的旧事项键仍可能解析到移项后的实际事项。基线 adapter 丢弃响应 Key/Project，仅用请求或计划中的旧项目授权。现在 JiraState 保留并核验真实 IssueKey/ProjectRef；缺失或与请求/计划不符即保守拒绝。context/prepare 和派发前拒绝错误身份；派发后及 lease 恢复无法核验原目标则保持 unknown；读取和幂等返回裁剪完整结果（issue_scope_unverified）。即使允许 restricted，也不能跳过项目身份核验。

代码：`internal/server/open_jira_adapter.go`、`internal/decisioncommands/module.go`。新增 fake 回归覆盖移项、缺失身份、写前/写后、恢复、读取、Execute 与幂等重放。

## AI 治理页面优化

六组件复用共享材质、间距、字色和 AdminListFilterBar：外层结构透明，主列表/编辑区使用单层毛玻璃，内部用间距和分隔线区分。长表格、代码局部滚动，检查移动操作触达、弹窗焦点限制及 Escape。移除了无当前证据的沙箱/DAG 指标；规则缺名称时显示仓库 ID。

`gitlab.snapshot` 与 `knowledge.search` 是 code_review 内置组件，不再占独立列表行，不提供独立启停/升级/卸载。归属由 API parent_skill_key/included_components 解析，不硬编码名称。加载完整 registry、解析层级后再过滤；搜索组件名称、描述或工具返回父技能。独立能力保留，孤儿/循环关系另提示且不误给生命周期动作。

当前示例为 1 顶层技能、2 内置组件。计数分别呈现顶层记录、内置组件和完整注册表磁盘占用。详情保留两组件信息；桌面显示“详情/升级/卸载”，小屏局部横滚；1023 断点标题左对齐。

组件：AIGovernanceCenter、SolutionPromptConfig、AIConfig、CorpusSourceLibrary、CorpusCandidateReview、OpenCapabilitiesConfig。层级适配由 `web/src/lib/governance-capability-list.ts` 负责。

## 验证

基线：45 项后端既有测试通过；3 隔离实验成功复现范围泄露、10,000 行截断和环境开关回退。实验 PASS 表示问题被复现。

本次修复验证：

- 完整 `GOCACHE=/tmp/well-ambient-audit-go-cache go test -json ./...` 通过：28 个包、723 个顶层测试，无失败。decisioncommands/openaccess 两包现在合计 37 个测试；这两包完整 race 和相关 server race 通过。缺陷回归先确认失败再修复，Jira 全部使用 fake。httptest 仅绑定隔离 localhost，未启动业务服务。
- 服务端提示词预览 3 个回归通过，使用隔离 RoundTripper，没有向模型发送请求。
- 前端 39 项相关测试通过，覆盖归属、搜索、过滤、异常关系、占位链接、shell 引号、取消、超时与已有治理契约。
- npm run check：0 errors / 155 既有 warnings（12 files）；npm run build 通过。
- 六组件 Impeccable 无发现；git diff --check 通过。
- 认证管理员浏览器：首轮五分区已检查；本轮技能归属/组件搜索/空状态/详情、开放能力及提示词复查 1440、1023、759、480 CSS 宽度，根无横向溢出。签发弹窗只打开/取消，未签发、卸载或安装。提示词测试一次主动取消，显示“已取消模型测试”并恢复输入。未声称模型输出成功。

截图在 `.agent-runtime/tasks/2026-10-03-ai-governance/screenshots/`，本轮为 `*-fixed-*.jpg`。前端日志 `/tmp/well-ambient-fixes-{check,build,web-tests}.log`；完整 Go 日志 `/tmp/well-ambient-fixes-go-tests.jsonl`。

## 剩余边界

浏览器使用原运行服务验证当前前端；修改后的 Go 行为由隔离测试验证，未重启/部署业务服务。生产 PostgreSQL schema、多实例租约、真实 Jira、目标 MCP 客户端及只读角色的新视觉验收仍缺证据。降级材质与动效通过源代码/检测检查，未强制模拟系统设置。

P4/P5 的缺失范围没有自动补齐。历史统计业务口径和永久 Key 管理需按部署需求确认；运维文档已澄清 CLI 必须正 TTL、治理 API 支持永久 Key 的差异。
