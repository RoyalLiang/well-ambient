# 迁移、验证与发布

## 1. 迁移策略

不做大爆炸替换。

### Step 1：新增表，不改权威路径

- Capability
- CapabilityVersion
- CapabilityResource
- CapabilityDependency
- AgentRun
- RunCapabilityBinding
- AgentRunEvent
- CapabilityEvaluation
- CapabilityProposal

当前 `SolutionPromptTemplate` 和 CodeReviewRun 继续工作。

### Step 2：兼容适配

- 将 active `code_review` 映射为 capability manifest。
- 新 Run 同时写现有 skill 字段和 RunCapabilityBinding。
- 对比两侧 digest、version 和 selection reason。

### Step 3：Shadow Resolver

- 新 resolver 产生计划，但执行仍使用现有路径。
- 记录差异：
  - capability 漏选；
  - 多选；
  - 权限冲突；
  - 预算差异；
  - scope 选择差异。

### Step 4：Canary

- 只对测试仓库启用 runtime launcher。
- 保留旧执行路径作为 fallback。
- 外部评论默认关闭。

### Step 5：Authority 切换

- 新 runtime 成为 Code Review 唯一入口。
- 旧字段只作为兼容投影。
- 稳定后再迁移下一个 Agent。

## 2. 数据回填

- 历史 `code_review` prompt → CapabilityVersion。
- CodeReviewRun.skill_version_id → RunCapabilityBinding。
- PromptVersion/model/policy/context → AgentRun lockfile。
- 无版本历史 Run 标记为 `legacy_unresolved`，不伪造 digest。
- 历史结果不重新执行、不自动评论。

## 3. 安全门禁

- capability digest 必须校验。
- 生产 capability 必须有签名或受信任发布来源。
- MCP/plugin 必须声明权限。
- 运行 permission grant 不得宽于调用者。
- untrusted data 不能改变 capability plan。
- replay 禁止生产写工具。
- canary 不得自动扩大 scope。

## 4. 测试矩阵

### Registry

- 创建、版本递增、激活、退役、回滚；
- scope 覆盖和 fallback；
- 依赖缺失、冲突、循环；
- digest/signature 失败；
- 并发激活；
- 权限。

### Resolver

- 单 Skill；
- 多 Skill 组合；
- optional dependency；
- 权限不足；
- 预算不足；
- MCP 不可用；
- plugin 不兼容；
- 无能力 fail closed。

### Lockfile / Replay

- 激活新版不改变历史运行；
- hash 一致；
- replay 输入一致；
- legacy run；
- cancellation；
- failed run；
- 并发运行；
- 跨租户隔离。

### Context Protocol

- L0-L3；
- delta；
- blob/ref；
- 长路径、长代码和 Unicode；
- 恶意输入；
- token 预算；
- cache；
- evidence line mapping。

### MCP / Plugin

- 连接、首次懒加载；
- timeout、熔断、恢复；
- 凭证隔离；
- 未签名插件；
- 沙箱文件/网络边界；
- 输出注入；
- 资源耗尽。

### Strongest Brain

- proposal 不可直接激活；
- replay 无副作用；
- canary 分流；
- 自动回滚；
- 指标项目隔离；
- 敏感字段不泄露。

## 5. 性能目标

首个 Code Review 迁移目标：

- 输入 token 下降 ≥30%；
- p95 延迟增加 ≤10%；
- evidence completeness 不下降；
- retained finding precision 不下降；
- Run replay 一致率 ≥99%；
- capability cache hit ≥70%；
- MCP/plugin 故障不影响无关能力。

## 6. 观测

每次运行记录：

- resolver latency；
- selected/rejected capabilities；
- manifest/resource cache hit；
- tokens by layer；
- tool calls and failures；
- MCP connect latency；
- plugin sandbox exits；
- context expansion；
- model calls；
- verification results；
- human feedback；
- total cost。

## 7. 回滚

每个阶段必须具备：

- 关闭新 resolver；
- 回到旧 CodeReview Service；
- 保留新表但停止写入；
- 关闭 MCP/plugin capability；
- 恢复上一 active capability version；
- 终止 canary；
- 保留 lockfile/trace 供分析。

禁止通过删除历史版本或运行记录回滚。

## 8. 发布清单

- [ ] 数据库备份。
- [ ] migration dry-run。
- [ ] Capability Registry schema。
- [ ] 默认能力 seed。
- [ ] 兼容适配校验。
- [ ] 全量测试和 race。
- [ ] 安全审计。
- [ ] Replay 基线。
- [ ] Shadow resolver。
- [ ] 测试仓库 canary。
- [ ] 1% / 10% / 50% 门禁。
- [ ] 回滚演练。
- [ ] 运行手册和告警。

## 9. 生产批准条件

只有同时满足以下条件才允许把新 runtime 设为 authority：

1. 核心质量指标无回归。
2. 安全和权限测试通过。
3. lockfile/replay 可用。
4. MCP/plugin 故障隔离通过。
5. canary 达到约定样本数。
6. 回滚演练成功。
7. 最强大脑 proposal 仍需人工批准。
