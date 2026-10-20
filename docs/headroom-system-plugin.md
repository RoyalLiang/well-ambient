# Skill 安装记录与 Headroom 系统插件

“技能治理中心”默认显示已安装 Skill：必须同时存在服务端安装回执、匹配的注册版本摘要，以及通过 SHA256 核验的 SKILL.md 文件。清单导入、远程清单安装和升级会写入这些证据。默认目录为 `/var/lib/well-ambient/skills`，可用 `WELL_AMBIENT_SKILLS_DIR` 指定持久化目录。

系统内置能力单独查看。开放 MCP、四个开放子技能与运行时知识提供方归入 `well-ambient` 详情；这种展示归属不增加父技能的工具或权限。注册、下载及启用状态均不能证明 Codex / Claude 客户端安装，客户端状态显示“未上报”。旧注册记录没有回执时保留在“注册记录”范围，不推断为已安装。

Headroom 是随服务镜像提供的系统插件，注册键为 `system.headroom`，不会计入已安装 Skill 数量。镜像固定安装 [headroom-ai 0.41.0](https://pypi.org/project/headroom-ai/0.41.0/)，上游为 [Headroom](https://github.com/headroomlabs-ai/headroom)，许可证 Apache-2.0。版本及根包 Linux wheel 摘要见 `capabilities/plugins/headroom/UPSTREAM.json`；依赖版本见 `requirements.lock`。后者固定版本，不是全部依赖的哈希锁。

AI 启用后，默认在共享 LLM 客户端发送请求前处理用户上下文。它覆盖 Responses、Chat Completions、Messages 及流式调用；系统指令、附件和文件引用保持原样。使用上游 `densify` 的 agent 无损模式，不启用 CCR 删除或 ML 有损压缩。压缩由本地 Python 进程执行，不新增模型服务或业务数据传输目的地；原本配置的模型仍按原有请求接收上下文。

可以在系统内置列表禁用插件，也可在 AI YAML 中设置 `headroom_enabled: false`。重启会保留显式禁用状态。新 Agent 能力解析只有在业务能力匹配后才附加该系统插件，不会用插件满足未知业务意图，也不增加业务授权。

输入不足 4 KiB 或超过 8 MiB 时直接保留原文。一个服务进程最多运行一个压缩任务；忙碌、错误、超时、无损标记缺失或输出增长时保留原文。单次默认上限 8 秒，错误后冷却 30 秒。运行指标只计进入压缩桥接的请求；阈值跳过、忙碌和冷却不增加处理计数。

“系统内置 → system.headroom → 详情”显示实际进程状态、处理/压缩/回退次数、节省 Token 和最近原因。统计来源为当前服务进程，重启后归零，合成检查也计入。只记录长度、摘要和统计，不保存压缩输入正文。管理员的“运行本地检查”使用合成数据，不调用外部模型。注册表启用状态与实际运行健康分别展示。

服务镜像内 Python 路径为 `/opt/well-ambient/headroom/bin/python`，桥接脚本为 `/opt/well-ambient/plugins/headroom/worker.py`，常用 tokenizer 已在构建时缓存。运行时使用离线模型设置。`WELL_AMBIENT_HEADROOM_PYTHON` 和 `WELL_AMBIENT_HEADROOM_WORKER` 只用于受控部署路径配置。

生产以 `--skip-migrate` 启动时，先执行 `deploy/migrations/20261020_capability_installations.sql`。它只新增安装回执表和索引。已有知识表、用户、授权策略和刷新频率无需迁移。保留原有 `/var/lib/well-ambient` 持久卷，安装目录需要服务 UID 1000 可写。
