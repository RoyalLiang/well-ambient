# ADR 0004：开放能力共享领域 Module 与统一集成身份

- 状态：Accepted
- 日期：2026-09-27

外部 HTTP 调用、MCP 工具和内部最强大脑共享 Jira Query、Decision Command 与 Review Read 的应用 Module；HTTP 和 MCP 只作为 Adapter，不复制授权、统计口径、幂等或远端确认规则。外部身份解析为系统签发的 Integration Source 与 Credential，首版所有有效来源使用同一 Integration Access Policy，来源名称不产生不同业务权限；Jira 写入继续使用系统管理的 Jira Execution Binding。

这一决策避免了协议入口之间的事实漂移，也防止外部 actor/source/Jira 用户名绕过系统策略。代价是开放入口不能直接复用内部 JWT handler，必须维护独立的凭证生命周期、审计和配额 Adapter；未来若需要按来源差异授权，必须显式修改领域模型和迁移策略，而不能把来源元数据悄然解释成角色。
