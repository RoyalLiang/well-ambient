# 领域模型与 Schema

## 1. Capability

稳定能力身份。

| 字段 | 说明 |
| --- | --- |
| `id` | 内部 ID |
| `capability_key` | 稳定字符串 ID |
| `kind` | skill/mcp/plugin/policy/model_profile/context_provider |
| `status` | active/disabled/retired |
| `owner` | 责任团队 |
| `sensitivity` | public/internal/restricted |

## 2. CapabilityVersion

Append-only 版本。

| 字段 | 说明 |
| --- | --- |
| `capability_id` | 稳定能力 |
| `version` | 作用域内递增版本 |
| `scope_type` | global/tenant/project/repository |
| `scope_id` | 权威作用域 ID |
| `manifest_json` | canonical manifest |
| `content_digest` | manifest 与资源摘要 |
| `signature` | 发布签名 |
| `validation_status` | untested/passed/failed |
| `status` | draft/active/retired |

唯一约束：

```text
capability_id + scope_type + scope_id + version
```

同一 capability/scope 只能有一个 active version。

## 3. CapabilityResource

能力正文和资源块。

| 字段 | 说明 |
| --- | --- |
| `resource_key` | review-method/evidence-contract/tool-schema 等 |
| `content_kind` | text/json/schema/wasm/binary/ref |
| `content_hash` | 内容地址 |
| `blob_ref` | 大内容外部引用 |
| `token_estimate` | 模型呈现 token 估算 |
| `load_level` | L0/L1/L2/L3 |

## 4. CapabilityDependency

有向依赖：

```text
merge-review@3
  requires gitlab.snapshot >=2
  requires knowledge.search >=1
  optional repository.callgraph >=1
```

依赖解析必须检测：

- 缺失版本；
- 循环依赖；
- scope 冲突；
- 权限不足；
- 预算超限；
- 不兼容 runtime。

## 5. AgentRun

通用 Agent 运行根。

| 字段 | 说明 |
| --- | --- |
| `run_key` | 幂等 key |
| `agent_kind` | code_review/deconstructor/... |
| `kernel_version` | 微内核版本 |
| `model_profile_ref` | 模型锁定 |
| `state` | queued/resolving/loading/running/verifying/... |
| `context_pack_id` | 冻结上下文 |
| `permission_grant_id` | 权限快照 |
| `budget_json` | token/time/tool/cost |
| `lockfile_hash` | 运行锁文件摘要 |

## 6. RunCapabilityBinding

每次运行实际加载的能力。

| 字段 | 说明 |
| --- | --- |
| `run_id` | AgentRun |
| `capability_version_id` | 精确版本 |
| `digest` | 内容摘要 |
| `selection_reason` | resolver 选择理由 |
| `load_level` | 实际加载层级 |
| `permission_grant` | 允许能力 |
| `loaded_at` | 加载时间 |
| `invocation_count` | 调用次数 |
| `token_cost` | 呈现 token |

## 7. AgentRunEvent

Append-only trace：

- intent_compiled
- capability_selected
- capability_rejected
- resource_loaded
- mcp_connected
- plugin_invoked
- context_expanded
- model_called
- validation_failed
- human_approved
- canary_assigned
- completed
- rolled_back

## 8. CapabilityEvaluation

Replay/canary 评估结果：

| 字段 | 说明 |
| --- | --- |
| `candidate_version_id` | 候选版本 |
| `baseline_version_id` | 基线版本 |
| `dataset_ref` | 样本集 |
| `quality_score` | 质量 |
| `evidence_score` | 证据 |
| `safety_score` | 安全 |
| `token_cost` | token |
| `latency_ms` | 延迟 |
| `regressions_json` | 回归 |
| `verdict` | pass/reject/manual_review |

## 9. CapabilityProposal

最强大脑只生成 proposal：

- 修改 skill resource；
- 调整 resolver 条件；
- 调整 budget；
- 调整工具顺序；
- 切换 model profile；
- 启用/禁用 optional capability。

Proposal 不具有激活权限。

## 10. 核心接口

```go
type Registry interface {
    Resolve(ctx context.Context, query ResolveQuery) (CapabilityPlan, error)
    GetVersion(ctx context.Context, ref VersionRef) (CapabilityVersion, error)
}

type Loader interface {
    Load(ctx context.Context, plan CapabilityPlan, budget Budget) (LoadedCapabilities, error)
}

type Renderer interface {
    Render(ctx context.Context, pack ContextPack, view ViewSpec) (ModelContext, error)
}

type Launcher interface {
    Start(ctx context.Context, command StartRunCommand) (AgentRun, error)
    Cancel(ctx context.Context, runID uint, reason string) error
}

type Replay interface {
    Evaluate(ctx context.Context, command ReplayCommand) (CapabilityEvaluation, error)
}
```

## 11. 建议 API

```text
GET    /api/agent-runtime/capabilities
POST   /api/agent-runtime/capabilities
POST   /api/agent-runtime/capabilities/{id}/validate
POST   /api/agent-runtime/capabilities/{id}/activate
POST   /api/agent-runtime/resolve/preview
GET    /api/agent-runtime/runs
GET    /api/agent-runtime/runs/{id}
GET    /api/agent-runtime/runs/{id}/lockfile
GET    /api/agent-runtime/runs/{id}/trace
POST   /api/agent-runtime/replays
GET    /api/strongest-brain/capability-intelligence
POST   /api/strongest-brain/capability-proposals/{id}/review
```

## 12. 权限

- `capability:read`
- `capability:manage`
- `capability:validate`
- `capability:activate`
- `agent_run:read`
- `agent_run:start`
- `agent_run:cancel`
- `agent_trace:read`
- `agent_replay:execute`
- `capability_proposal:review`

激活、权限扩展、插件安装和 MCP 凭证绑定继续要求全局超级管理员或独立高风险审批。
