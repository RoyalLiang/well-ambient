# 最强大脑 Capability 优化闭环

## 1. 角色

最强大脑是治理建议和评估系统，不是自动改写生产 Agent 的控制器。

它可以：

- 发现 capability 选择和加载问题；
- 聚类失败、partial、重评和高成本运行；
- 生成候选版本、resolver 或预算调整；
- 执行隔离 replay；
- 推荐 canary。

它不能：

- 自动激活 Skill；
- 自动安装插件；
- 自动扩大权限；
- 自动绑定 MCP 凭证；
- 自动发布外部评论；
- 在 replay 中执行生产写操作。

## 2. 输入

### Run Lockfile

- kernel、模型和 capability 版本；
- context pack；
- permission grant；
- budgets。

### Run Trace

- resolver 候选和选择理由；
- 资源加载层级；
- 工具调用；
- token、延迟和错误；
- 验证与人工反馈。

### 领域结果

- completed/partial/failed；
- evidence completeness；
- accepted/rejected findings；
- retry、cancel、override；
- 外部写入结果。

## 3. 特征

| 类别 | 特征 |
| --- | --- |
| 选择 | capability 漏选、过载、冲突 |
| 上下文 | token、重复率、late-load、cache hit |
| 工具 | MCP 失败、插件超时、无效调用 |
| 质量 | evidence completeness、误报、漏报、人工重评 |
| 成本 | token/有效发现、延迟、模型成本 |
| 安全 | 权限拒绝、注入、敏感数据、未签名能力 |
| 稳定 | failed/partial/cancel/unknown |

## 4. 候选类型

- 新 Skill 草稿；
- Skill 资源拆分；
- resolver 条件调整；
- optional capability 启停；
- context budget 调整；
- tool ordering；
- model profile 切换；
- MCP 熔断和 fallback；
- plugin 权限缩减；
- context provider 检索策略。

## 5. Replay

Replay 数据集至少包含：

- 正常成功样本；
- partial 和缺上下文样本；
- 模型格式失败；
- MCP/plugin 故障；
- 高风险和安全样本；
- 人工纠正和重评样本；
- 长上下文和预算边界；
- 多项目/多租户隔离样本。

Replay 必须：

- 使用冻结事实；
- 禁止生产写工具；
- 记录候选与基线结果；
- 保留随机种子、模型和 capability 版本；
- 支持重复运行。

## 6. 评分

建议总分：

```text
score =
  0.40 * quality
+ 0.20 * evidence
+ 0.15 * safety
+ 0.10 * stability
+ 0.10 * token_efficiency
+ 0.05 * latency
```

硬门禁：

- safety 不得下降；
- 权限不得扩大；
- evidence completeness 不得下降；
- replay parse failure 不得上升；
- P0 回归为 0；
- 关键业务样本必须人工复核。

## 7. Proposal

每个建议必须包含：

- 问题聚类；
- 证据 Run IDs；
- 基线指标；
- 候选变更；
- replay 数据集；
- 质量/成本/安全差异；
- 影响 capability；
- canary 范围；
- 回滚条件。

## 8. Canary

推荐阶段：

```text
shadow
→ 1% tenant/project
→ 10%
→ 50%
→ one-agent full rollout
```

自动回滚条件：

- evidence completeness 下降；
- failed/partial 明显上升；
- P0/P1 回归；
- token 成本超过阈值；
- MCP/plugin 错误扩散；
- 权限拒绝或敏感数据事件。

## 9. 对外接口

```text
GET  /api/strongest-brain/capability-intelligence
GET  /api/strongest-brain/capability-proposals
POST /api/strongest-brain/capability-proposals/{id}/review
POST /api/agent-runtime/replays
GET  /api/agent-runtime/replays/{id}
```

## 10. 当前 review intelligence 的迁移

当前 `/api/strongest-brain/review-intelligence` 继续保留，作为 Phase 0 adapter：

- skill version metrics → capability version metrics；
- retry rate → stability feature；
- evidence gaps → context feature；
- high-risk findings → quality feature；
- recommendations → capability proposal draft。

当 Capability Registry 上线后，该端点可以内部读取统一 AgentRun/Lockfile/Trace，而不再直接解析 CodeReviewRun。
