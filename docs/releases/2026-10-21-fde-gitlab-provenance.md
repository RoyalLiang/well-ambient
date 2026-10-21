# FDE GitLab callbacks and repository provenance

## 中文

GitLab 设置按实例管理连接、仓库清单与 Push / Merge Request 回调。默认 devgit 保留原连接与数字项目 ID；FDE 使用独立 API Token、回调密钥和 `fde:<远端项目 ID>`，未知或误选的实例标识会被拒绝。更换服务器地址不会继承另一地址的令牌，禁用实例会阻止排队评审继续读取源码。

仓库列表以完整分组路径展示同名仓库，提供分组筛选、搜索、分页和分批安装进度。连接检查验证当前实例的 API 身份，配置错误和局部失败可见。

事件、提交、评审和自动交付保存来源键及冻结来源 JSON，包括实例、远端 ID、名称、完整路径、分组和采集时间。最强大脑的证据链、摘要及快照携带这些来源；仓库改名不会改写历史名称，跨实例同名或相同 SHA 不再合并计数。MCP 新来源字段仅在发布策略授权后输出，原授权范围不会自动扩大。

发布迁移仅新增六个来源字段和两个索引。已有历史记录缺乏证据时保持来源未知，不凭短名补猜 FDE。默认实例旧评审的去重锚点绑定部署前地址，保留已发布、发送中和结果未知的评论幂等边界；切换服务器后使用新来源身份，旧任务不能跨目标执行或发布。

操作边界：本次生产范围为清单中 `project-code` 及全部子分组的 1,106 个仓库；未来新增仓库需要重新拉取并安装。安装读回与真实事件送达分别留证。未自动回填历史仓库代码、未修改 Jira 或远端源代码。

## English

GitLab settings now separate instance credentials, callback secrets and repository registries. FDE uses qualified project IDs and an instance-specific Push/MR receiver. Full namespace paths, filtering, pagination and batch progress distinguish repositories with identical names.

Immutable source metadata follows webhook, commit, review and delivery evidence into Strongest Brain summaries and snapshots. Cross-instance IDs and SHAs stay separate. Optional MCP source fields require explicit field publication. Legacy records retain unknown provenance unless supported by evidence.

The migration adds six provenance columns and two indexes. The default instance retains its original deduplication boundary only while its endpoint matches the release anchor. Rebound endpoints cannot reuse or publish historical evidence. The current inventory covers 1,106 FDE repositories; future repositories require another discovery/install run. Hook registration and event-delivery evidence are reported separately.
