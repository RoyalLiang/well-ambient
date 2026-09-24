# Agent Runtime 能力架构决策

**状态**：方向确认，分阶段实施  
**日期**：2026-09-24  
**适用范围**：Well Ambient 在线 Agent、代码评审、最强大脑、Skill、MCP 和插件运行时

## 决策

Well Ambient 采用以下长期方向：

> 微内核 Agent Runtime + 受治理 Capability Registry + 懒加载 Context Pack + Run Lockfile + Replay/Canary 优化闭环。

当前上线的 `code_review` 版本治理是 Phase 0：它解决生产加载、版本冻结、测试、激活、审计和回滚，但仍以系统指令为主要可编辑资产。它不是最终的通用 Agent Runtime。

## 为什么不继续扩大全局 Prompt

全局 Prompt 适合固定安全规则和输出契约，不适合承载持续增长的领域能力：

- 每次请求重复发送无关规则，token 成本持续增加。
- 不同能力互相污染，冲突难以定位。
- 工具、MCP、权限和数据依赖无法通过纯文本可靠表达。
- Prompt 更新的影响范围过大，难以做单能力 canary 和回滚。
- 无法解释某次运行具体加载了哪些能力和数据。

## 为什么不采用完全动态、无内核约束的 Agent OS

完全动态加载会引入新的生产风险：

- Skill 漏选会让 Agent 缺失必要规则。
- MCP、插件和远程内容可能引入越权与提示词注入。
- 版本漂移会破坏运行重放和审计。
- 工具过多会增加选择错误、延迟和上下文占用。
- 插件依赖、网络故障和热更新可能形成连锁失败。

因此，以下能力必须留在不可动态覆盖的微内核：

- 身份、租户和权限；
- 非可信输入隔离；
- 能力签名和来源验证；
- token、时间、成本和工具预算；
- 状态机、取消、超时和幂等；
- 结构化输出与证据校验；
- 运行 trace、lockfile、replay 和审计。

## 目标组件

### Agent Microkernel

只保存运行不变量，不保存具体业务方法：

```text
Identity / Authorization
Safety boundary
Capability resolver
Budget allocator
Tool sandbox
Run state machine
Trace and replay
```

### Capability Registry

统一登记 Skill、MCP、插件、策略、模型配置和上下文提供者：

```yaml
id: merge-review
kind: skill
version: 3
digest: sha256:...
triggers: [code-review, merge-review]
provides: [diff-review, evidence-validation]
requires: [gitlab.snapshot, knowledge.search]
permissions: [source.read, knowledge.read]
resources:
  - review-method
  - evidence-contract
tools:
  - repository.read
  - gitlab.snapshot
budgets:
  instructions: 1800
  evidence: 12000
validation:
  suite: merge-review-v3
```

### Runtime Launcher

启动器只先读取 capability manifest：

```text
Request
→ Intent / condition compiler
→ Capability plan
→ Permission check
→ Load required manifests
→ Lazy load instruction chunks and tools
→ Execute
→ Verify
→ Persist lockfile and trace
```

MCP 先加载工具描述，首次调用时才建立连接。插件先完成权限、签名和依赖检查，再进入隔离执行环境。

### Run Lockfile

每次运行必须冻结实际依赖：

```json
{
  "kernel": "agent-runtime-v1",
  "model": "review-high@4",
  "capabilities": [
    {"id": "merge-review", "version": 3, "digest": "sha256:..."},
    {"id": "gitlab", "kind": "mcp", "version": 2}
  ],
  "context_pack": 812,
  "permission_grant": 95
}
```

激活新版本不能修改已经接受的运行。

## Agent 数据协议

一种格式不能同时优化数据库、网络和 LLM token。协议拆成三层。

### 存储层

使用完整、可版本化的 canonical JSON/graph：

- 类型、来源、时间和有效期；
- 内容 hash；
- 事实、声明、证据和关系；
- capability 与 context 引用；
- 原始 blob 的内容地址。

### 传输层

Runtime、插件和 MCP 之间可以使用 Protobuf、CBOR 或 MessagePack，降低网络与解析成本。二进制协议不直接发送给模型。

### 模型呈现层

由确定性 renderer 生成 schema-aware compact view：

```text
@run R82 goal=review-change
@caps merge-review@3 gitlab@2
@scope repo:10 mr:42 head:a91c2e

@facts
F1|requirement|empty feedback != success|K12
F2|constraint|no duplicate release|K18

@changes
C1|dispatch/completion.go|L38-L51|modified

@evidence
E1|C1:L44|return feedback == ""

@open
Q1|vehicle-service contract not loaded

@refs
K12|knowledge:182@v3
B1|blob:sha256:...|full diff
```

原则：

- 重复实体使用 ID，不重复完整字段。
- 大文件和长日志使用 hash/ref，按需读取。
- 首轮只发送索引、摘要和高相关证据。
- 第二轮复核复用同一 immutable context pack。
- 模型输出继续使用严格 JSON schema，不能使用自定义文本协议替代机器校验。

## 分层懒加载

- **L0 Manifest**：能力、触发条件、权限、预算。
- **L1 Guidance**：当前任务需要的 Skill 章节。
- **L2 Evidence**：相关 diff、知识、规格和历史事实。
- **L3 Raw Blob**：完整文件、日志和历史，通过工具按需读取。

加载器根据任务条件、模型不确定性和剩余预算升级层级，而不是启动时填满上下文。

## 最强大脑的职责

最强大脑优化整个执行策略，不只优化 Prompt：

- capability 选择准确率；
- 漏加载和过度加载；
- Skill/MCP/插件调用顺序；
- 上下文命中率和 token 分配；
- 模型路由；
- MCP 可用率和降级行为；
- Skill 版本的 partial、failed、误报和重评率；
- 每个有效发现的 token、延迟和工具成本；
- 权限越界、提示词注入和组合冲突。

优化闭环：

```text
Run trace
→ 异常与成本聚类
→ 生成候选 capability / resolver 调整
→ 历史样本 replay
→ 质量、成本和安全评分
→ 人工审批
→ Canary
→ 激活或回滚
```

最强大脑不能自动激活 Skill、扩大权限或发布外部评论。

## 当前实现与目标架构的关系

当前 `code_review` 在线能力已经提供：

- 数据库版本；
- 草稿、真实 dry-run、激活和退役；
- 入队版本冻结和 hash 校验；
- 不可变运行时安全与证据契约；
- 运行质量的最强大脑只读投影。

下一阶段不删除这些能力，而是迁移到通用抽象：

1. `SolutionPromptTemplate` 逐步抽象为 `CapabilityVersion`。
2. `code_review` 当前内容迁移为 `kind=skill`。
3. GitLab snapshot、证据校验和发布器登记为 `kind=plugin`。
4. GitLab 与知识库登记为 MCP/context provider。
5. `CodeReviewRun.skill_version_id` 扩展为 `run_capability_bindings`。
6. 引入 Capability Manifest、Run Lockfile 和分层 Context Pack。
7. 最强大脑从版本统计升级为 resolver、预算和加载粒度优化。

## 分阶段实施

### Phase 0：在线版本治理

当前已完成。验收标准：

- 生产从数据库加载 active skill；
- Run 冻结版本和 hash；
- 新版不影响历史 Run；
- 真实 dry-run 后才能激活；
- 最强大脑提供只读版本质量指标。

### Phase 1：Capability Registry

- 统一 Skill、MCP、插件和策略 manifest；
- 内容寻址、签名和权限声明；
- 通用版本、激活和回滚；
- Run capability lockfile。

### Phase 2：Runtime Launcher

- Intent/condition compiler；
- capability resolver；
- 懒加载 L0-L3；
- 工具沙箱、预算和取消；
- MCP 健康、熔断和降级。

### Phase 3：Replay 与最强大脑优化

- 历史运行 replay；
- 质量/成本/安全基准；
- 候选 resolver 和 Skill 版本；
- 人工审批与 canary；
- 自动回滚门禁。

## 关键指标

- token / accepted finding；
- context cache hit rate；
- late-load 次数和命中率；
- capability 漏选率、过载率；
- evidence completeness；
- partial、failed、retry 和 false-positive rate；
- MCP/plugin 故障隔离率；
- run replay 一致率；
- capability canary 回滚率；
- 权限拒绝和注入拦截次数。

## 不在当前阶段实施

- 不自动生成并激活 Skill。
- 不允许运行时下载未签名插件。
- 不把 MCP 输出视为可信指令。
- 不移除当前固定安全、证据和发布边界。
- 不在未定义仓库/项目权威映射前开放任意项目级 `code_review` skill。
