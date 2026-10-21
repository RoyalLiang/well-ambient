# Jira → Issue 自动同步

在配置中心的 Jira → Issue 同步页面填写本功能独立的 GitLab 群组地址、GitLab API Token、Jira Server/Data Center 地址和个人访问令牌（Bearer PAT），保存后测试并读取仓库。此功能不借用通用 GitLab 或 Jira 凭据。

服务端需要 `WELL_JIRA_ISSUE_SYNC_KEY`：32 个随机字节的 base64 编码。将其放在部署主机权限为 0600 的环境文件中，通过 Compose 传入 server；密钥与数据库备份分别保护，升级和回滚均保留原密钥。不要将真实 Token 或加密密钥写入 Git。Token 在数据库用 AES-256-GCM 加密，查询接口只返回是否已配置；留空保留已有 Token，变更连接地址必须重新填写 Token。

每条映射关联一个 Jira 项目和多个代码仓库。任务编号留空时同步该项目未完成任务；填写编号时仅同步这些任务。多个映射匹配同一任务时，所有关联仓库共同组成关闭条件。启用开关控制每日北京时间定时执行，右上角启动同步支持手动执行。默认关闭，空配置不会写远端。

AI 分析可选择 Jira 项目和仓库，读取各仓库默认分支的固定提交；每次最多 20 个仓库、每仓库前 300 个目录项中的最多 10 个源码/说明文件（超过 5 仓库时最多 4 个）、每文件最多 6000 字符且全批源码预算 40000 字符，按头中尾抽样，Jira 候选为最近更新的 100 个未完成任务且摘要总预算 12000 字符。它会调用系统已配置的 AI 模型。建议包含来源文件和提交；这属于抽样建议，采纳并保存后才成为正式映射。来源文档中的指令不会获得执行权限，返回的任务、项目、仓库和文件证据都须通过本地范围校验。

同步在每个关联仓库创建或更新一个带稳定标识的 Issue。已确认远端 Issue 身份持久化，正常改标题不会重复创建；创建请求结果不明时下次按标识对账。用户移除标识、移动或删除已关联 Issue 会阻断自动替代创建。Jira 描述只更新受管理区域，保留其他正文。

自动关闭 Jira 需要每个关联仓库都有且仅有一个明确关闭该 Issue 的已合并 MR，目标为当前默认分支；合并/压缩提交 SHA 必须对应默认分支最近一次 `source=push`、同 SHA 的成功流水线。CI 成功是本功能的验证定义，不代表额外执行了人工验收或业务测试。多 MR、缺失流水线、不同 SHA、失败或其他来源的流水线均保持待处理。Jira 重新打开后，不能复用上次关闭时的代码提交集合再次自动关闭。

证据评论包含仓库、Issue、MR、完整提交 SHA、流水线及采集时间。评论回读确认完整后，才执行唯一的 Done 分类流转，再回读 Jira 状态。多个 Done 流转或工作流必填字段无法满足时会保留“证据已评论，关闭未完成”，需调整工作流。请求超时不等于远端未执行；待确认关联会在下次运行对账，即使 Jira 已进入 Done。重试不盲目重复创建 Issue 或评论。

群组发现包含子群组，排除共享到群组但实际位于其他命名空间的仓库、归档仓库、无默认分支或无 Issue 功能的仓库。每群组最多 100 个可枚举仓库，Jira 同步每次最多 1000 个匹配任务，评论对账最多 2000 条；超限和分页不完整会明确失败。任务运行最长 8 分钟，跨进程租约 10 分钟；停止请求会取消正在等待的调用，但已发出的写入必须留待下次核对。

发布先备份数据库，再应用 `deploy/migrations/20261021_jira_issue_workspace.sql`，确认新增三表和索引，更新 server/web 镜像并检查 ready 与认证页面。迁移只添加表；回退应用镜像时保留这些表、关联记录和原加密密钥。旧 preview 接口保留，定时执行已改用独立 workspace。

接口位于 `/api/jira-issue-sync/workspace`，GET/PUT 读取与保存；POST 子路径 `discover`、`analyze`、`sync`、`cancel` 运行操作。GET 的 `run_id` 和 `offset` 参数按每页 20 项读取历史证据，最近列表最多 20 次运行。所有接口同时要求全局超级管理员身份及相应 config 权限。

API 依据：[GitLab 群组](https://docs.gitlab.com/api/groups/)、[Issue](https://docs.gitlab.com/api/issues/)、[MR](https://docs.gitlab.com/api/merge_requests/)、[流水线](https://docs.gitlab.com/api/pipelines/)、[仓库](https://docs.gitlab.com/api/repositories/)、[Jira Server REST 示例](https://developer.atlassian.com/server/jira/platform/jira-rest-api-examples/)。
