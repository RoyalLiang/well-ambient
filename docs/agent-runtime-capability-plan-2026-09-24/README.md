# Agent Runtime 与最强大脑落地方案

**状态**：待评审、可拆任务  
**日期**：2026-09-24  
**范围**：Well Ambient 在线 Agent Runtime、Skill、MCP、插件、上下文协议与最强大脑优化闭环

## 一句话目标

把当前以版本化 Prompt 为核心的在线 Agent，演进为：

> 微内核运行时 + 受治理 Capability Registry + 懒加载 Context Pack + Run Lockfile + Replay/Canary 优化闭环。

## 当前基线

Phase 0 已具备：

- 在线 `code_review` skill 版本、草稿、验证、激活和退役；
- Review Run 入队冻结 skill ID、版本和 hash；
- 固定安全、证据、结构化输出与两轮复核边界；
- 最强大脑 review intelligence 只读质量投影；
- 本地开发 Skill 与生产在线 Skill 的边界。

当前实现仍然是 Prompt-centric。它是通用 Agent Runtime 的迁移起点，不是终态。

## 目标结果

完成本方案后，系统应满足：

1. Skill、MCP、插件、策略、模型和上下文提供者使用统一 capability manifest。
2. Agent 启动只加载 manifest，正文、工具连接和原始证据按需懒加载。
3. 每次运行冻结 kernel、模型、capability、权限、context pack 和预算版本。
4. 存储、服务传输和模型呈现使用不同的数据编码。
5. 所有运行可以重放，并能解释“为什么加载这些能力和证据”。
6. 最强大脑优化 resolver、预算、工具顺序和 capability 版本，而不只优化 Prompt。
7. 候选优化必须经过 replay、人工审批和 canary，不能自动扩大权限或激活。

## 输出目录

| 文件 | 用途 |
| --- | --- |
| `README.md` | 总览、范围、目录和决策点 |
| `01-execution-roadmap.md` | 分阶段实施路线、依赖、工期与退出门禁 |
| `02-domain-and-schema.md` | 领域对象、数据库模型、服务接口和 API |
| `03-agent-context-protocol.md` | 存储/传输/LLM 呈现三层协议与 token 策略 |
| `04-strongest-brain-optimization-loop.md` | Trace、Replay、候选、Canary 和回滚闭环 |
| `05-migration-validation-rollout.md` | 兼容迁移、灰度、验证矩阵、风险与回滚 |
| `capability-manifest.example.yaml` | Capability Manifest 示例 |
| `run-lockfile.example.json` | 单次运行能力锁文件示例 |

## 关键决策

### 采用

- 微内核保留身份、权限、安全、预算、状态机、结构化输出、证据校验和审计。
- 业务能力进入 Capability Registry。
- Context 使用 L0-L3 分层懒加载。
- 每次运行生成不可变 lockfile。
- 最强大脑只生成候选和建议，激活必须有人类门禁。

### 不采用

- 不继续把所有能力塞进一个全局 Prompt。
- 不让 Agent 任意下载并执行未签名 Skill/插件。
- 不把 MCP 输出视为可信指令。
- 不使用二进制格式直接喂给模型。
- 不一次性替换现有 `SolutionPromptTemplate`、ContextPack 和 CodeReviewRun。
- 不在没有 replay/canary 前自动优化线上 resolver。

## 建议实施顺序

```text
Phase 1 Capability Registry
  → Phase 2 Run Lockfile / Trace
  → Phase 3 Context Protocol / Lazy Loader
  → Phase 4 MCP / Plugin Runtime
  → Phase 5 Strongest-Brain Replay / Canary
  → Phase 6 Migration / Rollout
```

## 总体工期

初步估算：`41–61 engineer-days`。

这是架构估算，不是承诺日期。Phase 1 完成后，必须用真实 capability 数量、运行 trace 和 token 数据重新估算后续阶段。

## 审阅决策点

1. 是否同意微内核保留不可动态覆盖的安全和审计边界？
2. 是否同意先建设 Registry/Lockfile，再接 MCP 和插件？
3. 是否同意模型输入使用确定性 compact renderer，而不是直接发送 canonical JSON？
4. 是否同意最强大脑只能生成候选，不能自动激活或扩大权限？
5. 是否同意 `code_review` 作为第一个迁移样板，其他 Agent 暂不并行迁移？
