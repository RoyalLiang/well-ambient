# 修复 MCP / Skill 远程分发并明确最强大脑评估边界

审查日期：2026-10-03。范围：当前工作树的开发代理、部署代理、官方 Skill 镜像资源与最强大脑评估。没有访问生产、发布镜像、签发密钥或执行 Jira 写操作。

当前代码存在三类可证实的问题：代理漏转发开放路径、镜像漏带官方 Skill、静态检查冒充回放通过。本次修复这三类问题。实际域名、外层 TLS、运行中的镜像及远程 MCP 客户端仍需部署环境验证。

## 先让入口和镜像包含真实分发能力

| 已修复问题 | 修复后的行为 | 代码位置 |
| --- | --- | --- |
| Vite 只代理 `/api`，开放路径落入前端响应；改写 Host 后生成内部地址 | 转发 `/open/` 与精确 `/mcp`，保留入口 Host、端口、路径和查询参数 | [vite.config.ts](/Users/eddie/Workspace/well-ambient/web/vite.config.ts:21) |
| Nginx 没有开放路径转发，Host 丢失非默认端口 | 增加开放路径，保留 `$http_host`，MCP 关闭缓冲及缓存 | [nginx.conf](/Users/eddie/Workspace/well-ambient/deploy/nginx.conf:28) |
| server 镜像只有二进制，构建上下文还排除了 `.agents` | 仅打包 `jira-analysis`、`board-decision`、`code-review-reader` 的公开资源，保留原工作目录 | [Dockerfile](/Users/eddie/Workspace/well-ambient/Dockerfile:45)、[.dockerignore](/Users/eddie/Workspace/well-ambient/.dockerignore:31) |

公开资源使用逐文件允许列表，当前包含 24 个文件，均受 Git 追踪并包含在 HEAD 的归档中。私有规则、凭据、运行态及官方目录内额外文件继续排除。COPY 使用相对目标，不移动原有配置、SQLite 或附件的相对路径。以后增加官方资源时，需要同步更新允许列表；打包契约测试会检验文件和引用完整性。

服务器优先使用显式 `server.public_url` 生成公开链接；未配置时使用请求协议与 Host。代理修复保留了这个回退来源，但不会自动替部署环境选择域名。远程客户端不能使用服务器自己的 `localhost` 或 `127.0.0.1` 作为仓库地址。

## 区分在线阅读、下载和 MCP 协议连接

后端已经有这些分发入口，`{name}` 仅允许上述三项官方 Skill：

| 路径 | 返回内容 |
| --- | --- |
| `/open/v1/skills` | JSON 技能目录 |
| `/open/v1/skills/{name}` | JSON 技能详情及链接 |
| `/open/v1/skills/{name}/skill.md` | Markdown 正文 |
| `/open/v1/skills/{name}/manifest` | YAML 清单 |
| `/open/v1/skills/{name}/archive` | gzip 下载附件 |
| `/open/v1/install.sh` | 安装脚本 |

当前 AI 治理卡片只有下载归档和复制安装命令，尚缺少在线阅读入口。建议在现有卡片增加“查看文档”和版本信息；MCP 旁增加工具目录及只读连接诊断。它们改善可发现性，不能代替路径转发与镜像资源修复。本次没有改变页面布局或添加这些按钮。

`/mcp` 是客户端协议入口，不是 HTML 仓库页；需要有效 Bearer。普通浏览器请求得到 401，或 GET 不提供 SSE 时得到 405，都不能单独证明 MCP 服务故障。连接应验证初始化和工具目录调用，并保留会话、协议头及流式响应。[MCP Streamable HTTP 规范](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)

| 观察到的响应 | 首先排查 |
| --- | --- |
| Skill URL 返回 HTML | 是否落入 SPA 回退，而没有到达后端 |
| 目录返回 JSON，文档或归档 404 | 运行镜像是否包含官方 Skill 资源 |
| MCP 401 | Bearer 是否有效、权限是否适用；不要将密钥放进 URL |
| MCP 403，提示 invalid Host | 是否是域名/LAN 入口代理到 loopback 后端 |
| 页面生成回环地址或错误协议 | `server.public_url` 与实际外部入口是否一致 |

MCP Go SDK 保留默认 DNS 重绑定保护：后端实际连接地址为 loopback，而请求 Host 不是 loopback 时会拒绝。本机 loopback 开发入口与当前 Docker 私网代理拓扑不触发这个条件。域名/LAN 前端连接 loopback 后端需重新核实拓扑；不要关闭保护来掩盖问题。公开 URL 配置只改变生成链接，不能改变该 SDK 校验。

如果 TLS 在更外层终止，内层 Nginx 的 `$scheme` 可能是 HTTP，应配置明确的公开 HTTPS 基址，或在可信入口正确处理转发协议。没有对未经确认的转发头扩大信任。

## 静态检查不能作为真实回放通过

原回放代码没有执行 `SampleRunIDs`，质量、证据、稳定、延迟等分数为固定值；基线读取和解析失败被忽略，空权限基线也跳过了扩权检查。结果可能返回 `pass`。未发现该结果自动触发实际版本推广，但它已经被 API 和基准工具当作评估结果读取。

本次 [ReplayEngine](/Users/eddie/Workspace/well-ambient/internal/strongestbrain/replay.go:37) 修复：

- 候选与基线必须存在、清单有效，并属于同一能力；基线缺失或损坏明确报错。
- 空权限基线仍比较扩权；扩权返回 `reject`，其余返回 `manual_review`。
- 明确 `evaluation_kind=manifest_checks_only`、`metrics_available=false`、`evaluated_sample_count=0`；保留 DTO 数字字段，但归零且不作为实测成绩。
- [GetReplay](/Users/eddie/Workspace/well-ambient/internal/strongestbrain/service.go:74) 在读取旧静态结果时清除展示分数，将旧 `pass` 标为人工审查，并记录旧判定。不迁移或覆盖历史数据库。
- [基准工具](/Users/eddie/Workspace/well-ambient/cmd/agent-runtime-bench/main.go:304) 使用已注册的真实基线，明确测量静态检查及存储吞吐，报告真实回放样本数为 0。

兼容边界：旧 API 调用现在必须提供有效基线。消费者应读取 `metrics_available`，不能把数字零解释成实测零分，也不能绕过 `GetReplay` 使用旧数据库 `pass` 作为质量背书。完整历史回放仍未实现。

## 按证据质量推进最强大脑优化

| 优先级 | 当前状态与下一步 | 收益与成本 |
| --- | --- | --- |
| 1 | 虚假回放通过已封闭。后续保存输入、工具结果与版本，进行同一固定样本的候选/基线对照 | 建立可信质量依据；完整引擎成本较高 |
| 2 | [治理统计](/Users/eddie/Workspace/well-ambient/internal/strongestbrain/intelligence.go:153) 仍有最近各取 limit、全历史拒绝事件、混合分母及完成率当质量的问题。统一时间窗口和可计算样本分母，未测质量标未知 | 避免错误建议；成本中，需混合历史记录和窗口边界回归 |
| 3 | [开放能力健康统计](/Users/eddie/Workspace/well-ambient/internal/openaccess/intelligence.go:169) 已区分完整窗口、started、confirmed、unknown。把字符串建议变成有原因、证据、责任人与 Operation 链接的排查项 | 让建议可处理、可关闭；成本中，不自动重发未知写操作 |
| 4 | [内部旧决策入口](/Users/eddie/Workspace/well-ambient/internal/server/strongest_brain_handlers.go:2207) 仍存在异步写后返回 success 的语义缺口。复用受控 prepare / execute / Operation / confirmation | 统一幂等、恢复及远端生效证据；成本中 |
| 5 | [交付反馈](/Users/eddie/Workspace/well-ambient/internal/server/strongest_brain_delivery_handlers.go:829) 仍恒为 pending。记录采用、修改、拒绝与原因，关联上下文、运行和版本，形成回归样本 | 形成改进闭环；成本中，主观反馈不能覆盖权威业务事实 |

这些后续项尚未在本次实现。建议下一轮先做第 2 项，再按[原方案](/Users/eddie/Workspace/well-ambient/docs/open-capabilities-implementation-plan-2026-09-26.md:214)执行样本评估、人工选择、限定试用及发布或撤回。

## 验证结果和部署边界

- 真实 Vite 隔离代理测试先失败后通过，8 项覆盖分发类型、查询参数、入口地址、MCP GET/POST 认证及协议头、流式首帧、SPA 与近似路径。后端使用 fixture，不代表实际 SDK 连接或真实归档内容已经验证。
- Docker 打包测试使用 Docker 自身 Moby matcher 读取 `.dockerignore`，2 项契约先失败后通过，覆盖全部公开资源、引用和额外私有文件排除。独立测试模块不属于主模块 `go test ./...`。
- strongestbrain、agentruntime 包回归、server 分发/安装器及最强大脑 handler 定向回归通过；基准工具编译及内存 SQLite 执行通过。
- 前端 check 为 0 errors、155 个既有 warnings；build 通过。Impeccable 对改动配置检测无发现，`git diff --check` 通过。
- 没有运行 Nginx 实例，没有构建或部署 Docker 镜像。原本地 5173/8080 服务不可达，原浏览器验证标签页已不存在，未完成本轮实时已登录 UI 或实际远程域名验证。

可复现命令：

```bash
cd web
node --test tests/open-distribution-proxy.test.mjs
npm run check
npm run build
```

在仓库根目录运行：

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/tmp/well-ambient-audit-go-cache go -C tests/open-skill-image test -mod=readonly -count=1
GOCACHE=/tmp/well-ambient-go-cache go test ./internal/strongestbrain ./internal/agentruntime -count=1
GOCACHE=/tmp/well-ambient-go-cache go test ./internal/server -run 'Test(AIGovernanceOpenCapabilitiesOverviewDefaultDisabled|OpenSkillsPublicDistributionEndpoints|OpenInstallScript|StrongestBrainCapabilityHandlers_)' -count=1
GOCACHE=/tmp/well-ambient-go-cache go test ./cmd/agent-runtime-bench
GOCACHE=/tmp/well-ambient-go-cache go run ./cmd/agent-runtime-bench
```

独立打包模块依赖已缓存时可使用上述离线命令；首次运行需按环境准备依赖。本轮 server 测试改用临时 Go 缓存后通过，未修改用户诊断文件或工具链。
