# 分阶段实施路线

## 0. 执行原则

- 一次只迁移一条真实 Agent 链路，首条为代码评审。
- 每个 Phase 必须可以单独上线和回滚。
- 新 runtime 先 shadow，再成为 authority。
- 先冻结运行依赖，再做动态加载。
- 先建立 trace/replay，再允许最强大脑优化。

## Phase 1：Capability Registry

**目标**：把 Prompt、Skill、MCP、插件和策略统一为可治理能力。

**预计投入**：5–7 engineer-days

### 主要任务

1. 新增 `internal/agentruntime/registry`。
2. 定义 capability kind：
   - `skill`
   - `mcp`
   - `plugin`
   - `policy`
   - `model_profile`
   - `context_provider`
3. 增加 manifest validation：
   - ID、版本、digest；
   - provides/requires；
   - permissions；
   - budgets；
   - resources/tools；
   - validation suite。
4. 创建 `SolutionPromptTemplate -> CapabilityVersion` 兼容适配器。
5. 将 `code_review` 注册为首个 `skill`。
6. 将 GitLab snapshot、证据校验和评论发布登记为 `plugin`。
7. 将系统知识检索登记为 `context_provider`。

### 触及模块

- `internal/agentruntime/registry`
- `internal/db`
- `internal/codereview`
- `internal/solutions`
- `internal/server`

### 验收

- 同一 capability/scope 只有一个 active version。
- digest、签名、依赖、权限和预算校验失败时不能激活。
- 现有在线 `code_review` v1 可以通过适配器解析为 manifest。
- 现有 Review Run 行为不改变。

### 退出门禁

- Registry API 和迁移测试通过。
- 兼容适配器覆盖历史 Run。
- 没有任何运行时动态加载行为进入生产。

## Phase 2：Run Lockfile 与 Trace

**目标**：完整记录每次 Agent 实际依赖。

**预计投入**：5–7 engineer-days

### 主要任务

1. 新增 AgentRun、RunCapabilityBinding、RunEvent。
2. 冻结：
   - kernel version；
   - model profile；
   - capability ID/version/digest；
   - permission grant；
   - context pack；
   - budget；
   - resolver decision。
3. CodeReviewRun 继续作为领域运行记录，并关联 AgentRun。
4. 保存 capability resolver 输入与选择理由。
5. 提供 lockfile 查询和 replay 输入导出。

### 验收

- 任一运行可回答加载了什么、为什么加载、谁授权、消耗多少。
- capability 激活变更不影响已创建 lockfile。
- lockfile 内容地址可校验。
- 运行失败也必须留下完整选择与加载轨迹。

## Phase 3：Context Protocol 与 Lazy Loader

**目标**：减少无关上下文和重复 token。

**预计投入**：8–12 engineer-days

### 主要任务

1. 定义 canonical Context Graph。
2. 建立 L0-L3 context 层级。
3. 新增内容寻址 blob/ref。
4. 新增 schema-aware compact renderer。
5. 增加 context delta 和稳定前缀缓存。
6. 将 Code Review snapshot 拆为：
   - manifest/index；
   - finding candidates；
   - diff hunks；
   - source slices；
   - knowledge refs；
   - raw blobs。
7. 第二轮复核复用相同 immutable context pack。

### 验收

- 评审质量不低于当前基线。
- 输入 token 至少下降 30%。
- evidence completeness 不下降。
- context pack 可以重放。
- raw blob 未被加载时不会进入模型输入。

## Phase 4：MCP 与 Plugin Runtime

**目标**：让工具和执行能力动态化但保持可控。

**预计投入**：10–15 engineer-days

### 主要任务

1. MCP manifest 和连接管理。
2. 首次调用时建立连接。
3. 健康检查、熔断、超时和降级。
4. Plugin 签名、依赖和权限校验。
5. Plugin sandbox 与资源预算。
6. 工具输出统一标记为非可信数据。
7. 提供 mock MCP/plugin 测试运行时。

### 验收

- 未授权 capability 无法加载或调用。
- MCP 故障不会阻塞无关能力。
- 插件无法越过文件、网络、凭证和工具 grant。
- 运行 lockfile 能准确记录 MCP/plugin 版本。

## Phase 5：最强大脑 Replay / Canary

**目标**：让最强大脑优化能力组合与预算。

**预计投入**：8–12 engineer-days

### 主要任务

1. 运行 trace 特征提取。
2. 异常和成本聚类。
3. 生成 resolver、budget、skill 或模型候选。
4. 建立历史样本 replay。
5. 计算质量、成本、安全和稳定性分数。
6. 人工审批。
7. Canary 分流和自动回滚门禁。

### 验收

- 候选不能直接激活。
- replay 与生产写操作完全隔离。
- canary 可以按 tenant/project/capability 精确回滚。
- 最强大脑建议包含证据、预期收益和风险。

## Phase 6：迁移与生产切换

**目标**：把新 runtime 从 shadow 提升为 authority。

**预计投入**：5–8 engineer-days

### 顺序

1. 双写 lockfile/trace。
2. Shadow resolver，只记录不影响执行。
3. 1% canary。
4. 10% canary。
5. 50% canary。
6. 单 Agent 全量。
7. 下一 Agent 迁移。

### 退出门禁

- 质量无回归。
- token、延迟达到目标。
- 权限和审计无缺口。
- 回滚演练通过。
- 旧 runtime 可以安全关闭。

## 任务拆分建议

| Epic | 角色 | 前置 |
| --- | --- | --- |
| Registry 与 schema | Backend Runtime | 无 |
| Lockfile 与 trace | Backend Runtime | Registry |
| Compact Context Protocol | AI Context | Lockfile |
| Resolver 与 Loader | Agent Runtime | Registry、Context |
| MCP Adapter | Integration | Loader |
| Plugin Sandbox | Security Runtime | Loader |
| Replay Harness | Evaluation | Lockfile、Context |
| Strongest-Brain Optimizer | Intelligence | Replay |
| Governance UI | Frontend Settings | Registry |
| Migration / Canary | Platform | 全部 |
