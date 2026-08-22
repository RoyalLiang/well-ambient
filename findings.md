# Findings & Decisions

## 2026-08-21 - Daily Jira 滚轮跳底与方案发布 URL

- 两个症状必须独立判红：浏览器反馈环断言一次 wheel 的实际位移，后端回归断言缺失/可推导 public URL 时的发布行为和副作用边界。
- 既有记忆只作为定位线索：Daily Jira 是表格优先、内部滚动、自动刷新保留选中项；方案 Markdown 是人工权威，Jira 写回必须处于显式发布之后。具体实现和当前配置仍需以本地源码验证。
- 三方 UI 方向是 preserve-mode：不新增滚动动画、自定义 scrollbar、弹窗或卡片，只修复最小滚动 owner；方案 URL 错误属于后端配置/链接生成，不用前端吞错。
- 隔离已登录浏览器用 100 行、`has_more=false` 且无自动刷新夹具复现：首个 420px wheel 正常停在 420；跨过 overscan 边界的 120px 输入后，在没有后续输入时 `scrollTop` 连续跃迁到最大 4824.5px。总高度始终 5235px、首行实测 52px，证明不是分页、刷新或行高累计误差，而是虚拟 DOM 前部替换参与浏览器 scroll anchoring。
- 最小 UI 修复应由 `.audit-table-shell` 设置 `overflow-anchor: none`，不拦截 wheel、不人工写 `scrollTop`、不关闭虚拟化；需以同一真实滚轮序列证明修复后最终位移保持 540px 左右。
- 配置保存路径 `handleSaveConfig -> applyConfig -> *s.config = next` 会即时更新 `Server.PublicURL`，不存在热更新丢失。发布报错来自 handler 只看显式配置；浏览器同源 POST 已携带标准 `Origin`，可作为未配置时的请求级公开 origin 回退。显式 `server.public_url` 仍必须优先，且不从 Host、Forwarded 头自动推断。
- 最终实现保持明确优先级：有效显式公开地址优先；否则只接受单值、无用户信息/路径/query/fragment 的 HTTP(S) `Origin`；无 Origin 的自动化客户端继续得到 422。拒绝发生在事务前，草稿与 Outbox 均不变化；成功时发布和 pending Outbox 仍在同一事务内。
- 四断点同一真实滚轮序列均稳定在 540px，文档无横向溢出；说明 `overflow-anchor: none` 已切断正反馈，且没有靠 wheel 拦截或人工 scrollTop 修正掩盖问题。

---

## 2026-08-21 - 全站千万量级数据访问架构

- 页面盘点最终口径为 24 个用户页面/配置状态 + shell，不再沿用早期粗扫的 23 个口径；路由 inventory 为 77 GET API、9 个 generation dataset。
- 当前成熟度为 `verified=17` / `bounded=32` / `migration_pending=28`。已加入单调 ratchet，后续改动不得降低 verified/bounded 或新增 pending。
- 10M 通用隔离基准已实测：实体首页/深 keyset/实体时间线/本地 JSON handler warm p95 分别为 0.103/0.110/0.116/0.117ms，投影主键查找 0.006ms；数据库约 1.01GB，四条计划都无 table scan/临时排序。
- 这个 benchmark 只证明公共有界策略可达，不能替代 28 条 pending 领域 adapter 的查询计划、并发、冷缓存和浏览器证据。
- 首波跨页迁移已落地：context/corpus/release/execution 使用 generation keyset或有界详情；task activity 最多 100 项，demand spec 最多 100 版，users 最多 5,000 且 membership 从 N+1 收敛为 2 条 SQL。
- “低基数目录”也不能靠经验假设：核心成员用户加载和历史负责人 fallback 都已固定 5,000 上限，负责人 DISTINCT/ORDER BY 由 partial covering index 支撑。
- 历史 task ID 大小写变体不能用 `IN` + 普通 task_id 索引同时满足全局时间排序；精确回归捕到 `USE TEMP B-TREE FOR ORDER BY`。最终采用 `LOWER(task_id), created_at DESC, id DESC` 表达式索引与规范化等值查询。
- GORM `.Scan` 不只经过 Query callback；只监听 `gorm:query` 会漏记 Table/Scan 读取。观测底座已同时注册 Query/Row/Raw callbacks，并有两种读取同请求计数回归。
- 发布计划的 server-side project/status/q scope 变更暴露了公共资源层的跨 scope single-flight 缺口；现在只有 endpoint 相同才复用 inflight，scope 变更会 abort 旧请求并禁止旧 payload 发布。
- 浏览器慢响应探针证明公共资源层会在 180ms 刷新期间保留旧列表，续页/搜索使用真实服务端 cursor/scope；四个断点的文档尺寸稳定。停止 fixture 后出现的 SSE reconnect warning 是隔离服务终止信号，不作为业务控制台零日志宣称。
- `docs/all-page-10m-read-architecture.md` 现作为页面矩阵、路由成熟度、SLO 口径、生产 rollout 和不可宣称边界的审计文档。
- 用户明确把目标从 Daily Jira 提升到“所有页面、所有数据”；既有 Daily Jira 10M 读模型只能作为已验证的领域 adapter，不是全站完成证据。
- 统一架构必须保持领域语义：公共模块拥有有界读取、稳定快照、刷新并发和诊断；权限、过滤、排序、聚合、搜索语义和索引仍由各领域 adapter 拥有，避免万能查询抽象把复杂度泄漏给调用方。
- 当前第一目标是自动盘点并建立红灯，不先凭印象增加索引；每个风险必须能追到页面、HTTP 路由、实际查询和可执行验证。
- 历史证据已经验证三类可复用模式：TaskKanban 的 active workset + 服务端历史查询、Agenda 的字段投影/200 条历史/批量关联 + single-flight、数据资产的高水位 + filter fingerprint + keyset cursor。公共底座应抽取这些共同机制，而不是复制 Daily Jira 的领域表结构。
- `CONTEXT.md` 明确区分交付计划、执行任务、源事实、资产事件和投影快照；因此全站公共 seam 只能统一读取生命周期，不能混淆各页面的事实来源、写权限或聚合口径。
- 已接受的数据资产 ADR 要求列表使用固定高水位、不透明 keyset cursor、无实时全量 count/无 OFFSET，并保持领域当前状态由各写模型拥有；这是全站读取契约的最低公共约束。
- 当前只有 SQLite/GORM 一种真实存储。依照 deep-module seam 纪律，不为“未来可能换库”暴露假想 repository port；隔离测试与基准直接驱动同一 SQLite 实现，第二种真实存储出现后再引入 adapter。
- 当前可达信息架构至少包含 7 个一级工作区：决策、排期、方案、证据、任务、度量、配置；决策 2 个子页、排期 4 个、任务 2 个、度量 2 个，配置还有独立 section 列表。全站验收必须按实际子页/section，而不是按 7 个顶层标签计数。
- 前端请求盘点已经暴露多个架构不一致点：Agenda 已有共享 single-flight，但 ProjectHealthTelemetry 仍直接请求同一路由；`DemandKanban`/`Deconstructor` 仍读取 `/api/tasks`；ProjectBoard 周期读取完整 `/api/schedule`；Settings 多个目录接口没有显式分页；多个页面各自实现 interval/debounce/fetch/error 状态。
- 已有轻量/有界样本也应纳入公共契约而非重写：全局搜索 `/api/work-items?...limit=8`、release Jira issues `limit=200`、override audit `limit=200`、Daily Jira cursor 页、TaskKanban active workset。
- 可达子页面总数当前为 23：决策 2、排期 4、方案 1、证据 1、任务 2、度量 2、配置 12。页面性能清单将以这 23 个状态为验收单位，并单列全局 shell 的用户/通知/搜索数据。
- 数据契约不能全部做成相同分页：实体目录需要 keyset page；聚合看板需要读模型/预计算；时间线需要固定高水位；详情需要限制嵌套集合；用户/权限/项目等低基数字典需要明确 cardinality 上限与共享缓存。
- 后端路由确认同一页面常组合多个不同数据类别；例如方案 workspace 是单实体详情，但内部 revisions/sources 当前无界，jobs 仅限 20。详情接口也必须为每个嵌套集合定义上限或独立游标，不能因为 URL 是 detail 就豁免。
- 已确认的新增无界实体查询包括 corpus candidate 列表、execution run 列表及每个 run 的 actions、solution prompts/revisions/sources、performance active evidence revisions。前两者还存在“先全量 runs，再逐 run 查 actions”的明确 N+1 形态。
- `server.go` 目前直接注册大量 GET handler，没有统一的数据契约或查询预算 wrapper；性能边界分散在各 handler/module，导致 Daily Jira、data assets、Agenda 的成熟模式无法约束后来新增页面。
- `Server.Run` 直接把 `*http.ServeMux` 交给 `ListenAndServe`，测试也直接驱动 `srv.mux`；要让预算/诊断同时覆盖生产与 handler 测试，最小 seam 是统一 GET route 注册 wrapper，而不是只在 `Run` 外包一层生产 middleware。
- 全站红灯将同时做两件事：从 `server.go` 自动提取全部 GET `/api` pattern，要求每条声明数据类别/上限/一致性/预算；对用户页高风险 handler 另做 seeded 行为测试，防止“只登记 contract、实际仍全量返回”的纸面合规。
- 全站 contract 红灯已实际运行且稳定失败：生产代码没有 `allPageReadContracts`/`readContractClass`，所以不是某个遗漏 route 的偶然失败，而是公共 seam 确实不存在。命令只需约 15 秒首次编译，后续缓存后为秒级，适合作为持续反馈环。
- 当前主库文件是项目根的 `well-ambient.db`；只读 schema 显示 59 个业务表，覆盖任务、交付、方案、上下文、绩效、审计、通知与授权。全站性能不能只用 `task_telemetries` 的规模代表其他存储形态。
- 主库当前最大的热表不是任务表：`performance_audit_events=207,027`、`performance_work_item_events=205,207`，其次为 `task_telemetries=35,647`、`solution_source_refs=3,138`、`jira_comment_logs=3,082`。这验证了用户“所有数据”的修正：只优化 Daily Jira 会漏掉已更大的绩效/审计事实。
- `performance.Explain` 虽把快照输出限制到 30/100，但用 `OFFSET` 批次扫描快照找 core member，并调用 `loadActiveEvidenceFacts` 全量装载 evidence revisions；快照详情同样先装载全部 active facts 再在 Go 里按引用过滤。当前 evidence 表为空所以未显慢，但 10M 目标下会退化。
- KPI handler 每次请求调用 `loadKPIInputs` 再在内存构建个人/部门/报告投影；需要继续核对该输入函数是否投影/有界。此类聚合不能用“返回用户数少”证明基础扫描成本小。
- H2/H4 已被源码证实：`/api/schedule` 先全量读取可见需求，再按全部 group IDs 读子任务，最后用全部 work item IDs 富化 release；`/api/execution/tasks` 全量读取执行任务、全量 `DISTINCT git task_id`、多批父任务/证据后才在 Go 中搜索/负责人/风险过滤。
- `/api/tasks` 仍是 `status != archived` 的全宽 `Find`，只是前端部分调用已迁到 `/api/work-items`；Deconstructor/DemandKanban 的兼容调用会继续暴露线性 JSON/内存成本。
- `/api/tasks/commits` 对一个 task 的 Git/Jira 历史无上限；这属于 detail nested timeline，应使用独立高水位 cursor，而不是一次返回全部活动。
- KPI `loadKPIInputs` 每次加载时间窗内全部 Done、全部 active 状态任务和全部用户，再在 Go 中过滤 core members/聚合。Schedule risk calendar 又先构造完整 schedule snapshot，再筛风险和裁剪 bucket；二者都需要独立聚合/风险投影，不适合只加 LIMIT。
- 方案目录已有 40/100 上限，但仍使用 OFFSET + 实时 total count；token 搜索先 `GROUP BY` 全部匹配项到 Go，再切 offset/limit。详情的 comparisons 全量读取并逐比较查询 counterpart/proposal，是无界嵌套 N+1。
- 版本列表 `/api/releases` 全量读取 release 后以全部 IDs 做三次 grouped counts，再在 Go 里 contains 搜索；需要 cursor page + 同页 ID 聚合。其 Jira issue 子列表已有 100/200 limit，但仍需核对 cursor/深页语义。
- 修正早期粗扫结论：corpus candidate 列表和 context document 列表均已有硬 `Limit(200)`，不是无限返回；但没有 cursor，达到 200 后静默截断。context fact 列表仍无界，document detail 的 candidates 仍无界。
- 系统/授权审计与 config version 已分别限制 100、80/300、20/100；当前问题是缺稳定 cursor/高水位，而非首次响应无限。Execution runs 则确实无界且逐 run 查询 actions，属于明确 N+1。
- 公共模块的外部 interface 已收敛为 `Contract.Validate`、`Registry.Wrap` 和 `Registry.Snapshot`：调用方声明目标和预算，HTTP 观察、动态 path 匹配、有界 2048 样本窗口及 p50/p95/p99 计算均隐藏在模块内。删除该模块会把 header、计时、字节统计和分位数逻辑重新散到所有 handler，满足 deep-module 删除测试。
- SSE 是普通 response recorder 的特殊情况：wrapper 必须保留 `http.Flusher` 能力，否则统一中间层会让通知流误报“不支持 streaming”。当前 counting writer 已显式转发 Flush/Unwrap，并有参数化路由和指标序列化回归。
- 全部 77 条 GET `/api` route 已登记 read class、目标策略、成熟度、页面 surface、行/嵌套/字节预算和 20/50ms 默认目标；contract 自动发现测试与 23 页面覆盖测试已从编译红灯转绿。
- registry 已包装真实 `ListenAndServe` handler，而不是只存在于测试；响应头暴露 contract/class/target/maturity，`/api/status.read_paths` 只报告实际发生请求的有界样本 p50/p95/p99、错误与最大字节。`migration_pending` 不会被伪装成 verified。
- 公共 cursor interface 已实现为 `NewCursorCodec(contract, scope)` + `Encode(watermark, position)` + `Decode(cursor, &position)`；领域只拥有自己的排序 position/SQL，base64 envelope、版本、scope fingerprint、checksum 和篡改/跨路由拒绝由模块隐藏。
- limit 归一化统一拒绝 0/负数/非数字并硬 cap；cursor scope/contract mismatch 与格式/篡改错误分开，便于 handler 映射 409/400。定向红灯先失败于接口不存在，现已由 round-trip、跨 scope、跨 contract、tamper 和 limit tests 转绿。

## 2026-08-21 - 全局规则同步与 Daily Jira 自动同步冲突修复
## 2026-08-21 - 全局规则同步与 Daily Jira 自动同步冲突修复

- 项目 bootstrap 的 dry-run 返回 `status=existing`，它只保护性初始化缺失规则，不能替代已存在项目规则的内容同步；必须精确比较全局/规范模板与当前 `AGENTS.md`。
- 工作树包含大量既有改动；`AGENTS.md` 当前已有未提交的 pre-delivery reflection gate 增量，任何同步必须保留它及项目专属 UI 门禁/Matt 路由。
- 历史实现已经具备 `jiraInboundSyncMu`、30 秒 worker、changed broadcast、SSE 与前端 refresh scheduler；本轮用户给出的 `performance source event conflicts with retained payload` 指向 Jira 拉取后的性能历史追加边界，足以同时让自动 worker 和手动按钮失败。
- 用户明确的产品层级是“自动刷新为主，按钮仅辅助”；本轮不以增加或强化按钮替代自动同步修复。
- 主数据库只读证据：事件 456 的稳定键为 `jira:FZ-2257:2964008:0`，不可变业务事实是 priority `Medium -> Highest`、时间 `2026-07-07T01:59:14.670Z`；存量 actor 为 `武海洋 [X]`。同一 Task 当前 priority/severity 均为 `Highest`，说明冲突不是业务值矛盾。
- Jira 入站状态显示最近成功停在 2026-08-20 16:10，2026-08-21 16:33 的 227-item cycle 因该事件失败；这已证明自动 worker 与按钮共享同一阻塞点。
- 确定性红灯只改变同一历史事件的 `author.displayName` 为 `武海洋`，当前实现立即复现用户原始错误。排名最高的根因是可变展示名进入不可变哈希，而非 Jira priority 历史本身冲突。
- 受控只读 Jira probe 已确认现场当前 history 2964008 仍是 priority `Medium -> Highest`，但 author displayName 已由存量事件的 `武海洋 [X]` 变为 `武海洋`；H1 得到真实源数据证实，其他业务事实没有变化。
- 修复边界应留在 Jira adapter：通用 `performance.AppendSourceEvent` 继续严格检测所有字段；Jira changelog replay 可按已持久化的 stable semantic fields 识别同一事件，并忽略账号展示名漂移。
- 实现采用显式 `AppendSourceEventAllowingActorDrift`：先执行完整哈希校验，只有命中现有 dedupe-key 冲突时才取 retained actor 重算；若 from/to、event type、时间、source ID 或 payload 任一业务事实不同，仍返回 `ErrSourceEventConflict`。这没有放宽普通 `AppendSourceEvent`。
- 三方 UI 审查结论：现有按钮已经是 `secondary/small`，自动链已有 worker、SSE scheduler 与 30 秒可见页兜底；本轮不编辑前端，避免把后端幂等缺陷伪装成新的交互。
- 全局与规范项目模板的新增 reflection gate 已逐字存在于当前项目 `AGENTS.md`；bootstrap 对既有项目保护性不覆盖，项目专属冷启动/UI/Matt 路由保持不变。规则同步无需再复制全局项目中性段落造成双份维护。
- 全量验证通过：所有 Go 包、go vet、57 个前端契约、Svelte/TypeScript、生产 build、目标 Impeccable detector 与 diff hygiene。前端检查只报告 86 条既有 warnings，目标 Daily Jira 没有 detector finding。
- 当前主服务仍运行修复前二进制，且主数据库的 `jira-inbound` checkpoint 仍保留现场错误；本轮按授权边界未重启服务或改主库。受控重启后 worker 会立即首轮同步并在成功时清空 LastError。
- 最终只读范围 probe 把主库 205,041 条 retained 性能事件复制到内存库，并用当前 Jira 主 JQL 返回的 185 个 issue 逐项重放；全部无冲突且没有产生新内存证据，说明当前配置范围没有第二个残留阻塞。临时 probe 文件已删除。

---

## 2026-08-19 - 页面与搜索慢加载闭环

- 登录态 15 秒稳定性检查发现决策页仍有独立深层放大器：`fetchReleaseFacts` 只消费 `releases`，却调用完整 `/api/strongest-brain/delivery-cockpit`。单轮在 `strongest_brain_handlers.go:480/497` 两次物化约 35,578 条任务，并把全部 task ID 展开到 `git_commit_logs.task_id IN (...)`，最终触发 SQLite `too many SQL variables`。这条链与 Agenda 已优化查询无关，必须在消费方 API 边界修复。
- 发布事实已有独立的 `buildStrongestBrainReleaseSummary` 聚合 seam；新增只返回该投影的受权限路由，可让默认页面轮询不再构建排期、执行、证据链、异常和周决策。完整驾驶舱则应复用同一批任务/用户，并通过 `git_commit_logs JOIN task_telemetries` 应用项目范围，避免重复扫描和无界参数列表。
- 最终隔离运行跨过一次完整 15 秒刷新窗口，后端只有一次无害的用户列偏好 `record not found`；没有慢 SQL、`too many SQL variables` 或邮件扩展日志。页面在 1440/760/390px 均保持议程数据可见、发布事实无错误且 `scrollWidth == clientWidth`。
- 登录态隔离浏览器暴露了静态检查未覆盖的第四个放大器：`/api/notifications/sse` 每 15 秒调用 `computeDelayAlerts`，不仅 `SELECT *` 物化全部非 `done` 任务，还在读取循环中为每条延期事项调用 `EmailNotificationSender`；当前实现虽只写日志，但一次刷新产生约千行日志，并使未来真实邮件扩展存在重复发送风险。读取 API 必须纯化，查询应只投影提醒字段并使用与活动工作集一致的状态谓词。
- 前两轮已把 `/api/agenda/summary` 单次后端查询从无界全表/N+1 收敛为 5 条有界 SQL；本轮剩余问题在调用方和任务页数据边界，不能继续只靠加索引解决。
- TaskKanban 当前默认串行拉取约 72 个 500 条分页，累计约 35,578 行/33.1MB；服务端精确搜索约 43-46ms，说明“搜索慢”的主放大器是搜索前后仍背负的全量任务加载、DOM/派生计算和并发轮询。
- 活动事项约 1,064 行，现有接口只需 3 页/约 971KB；可把默认工作集限定为非 Done，同时让文本搜索走服务端并覆盖历史 Done，从而保留历史可发现性而不预载历史全集。
- Agenda 的两个 15 秒消费者需要共享单飞与唯一轮询所有权；单纯把两个 interval 调成不同数字仍会重复，也不能保证路由切换后的稳定刷新。
- 数据库已有 1 条 Git commit 与 5 条 notification 孤儿证据；证据链不应随任务投影删除而级联销毁，因此本轮继续采用 ORM 关联、批量预加载和索引，不启用破坏性物理外键。
- 项目记忆要求保留 TaskKanban 既有默认列和共享 Task/Bug 标记；本轮不修改列配置、标记组件或视觉系统。
- Impeccable 的性能流程要求先测量网络/载荷和真实瓶颈；现有 72 页/33.1MB 与双 15 秒请求是可量化 P0，优先级高于 CSS、bundle 或动画微优化。
- 三方 UI 门禁已达成 preserve-mode 共识：页面层级与响应式 DOM 不改，TaskKanban 只调整默认工作集和服务端搜索状态；Agenda 由共享资源接管 single-flight/freshness，`DecisionEventCenter` 成为唯一周期轮询 owner。
- design-taste-frontend 明确声明 dashboard/data table 不在其主导范围；本轮仅吸收其性能、状态完整性与 redesign 保护规则。Finesse 的通用品牌 substrate/hero 规则与 Phase 41 `DESIGN.md` 冲突时，以项目产品契约为准。
- Jira reconciliation 改为项目更新时间窗口后，返回结果仍必须与本地已知 Jira key 集合求交；否则虽然请求数下降，却会把主 JQL 原本排除的 Epic/其他事项意外导入本地，形成范围扩张。本轮在 merge 前保留该硬边界。
- 备用 pnpm 因尝试访问 registry 并在无 TTY 下重装依赖而失败；仓库现有 `web/node_modules/.bin/svelte-check` 完成同一静态检查，结果 0 errors/5 个 tsconfig 既有 warnings，未改依赖。
- 真实库副本应用新 active partial index 后，查询计划从 `idx_task_tracking_kind_status_project_owner + TEMP B-TREE ORDER BY` 变为直接扫描 `idx_task_tracking_active_last_update`，不再为活动表排序 3.5 万行。
- 真实库副本端到端 `QueryPlan`：默认 active 工作集 1,052 行、3 请求、971,732B、105.6ms；旧链路 35,566 行、72 请求、约 33.1MB、约 3.0-3.1s。精确 `HR-4202` 搜索仍为 1 行、1,734B、41.6ms，历史可发现性未牺牲。

---

## 2026-08-19 - 议程查询与聚合性能收敛

- 用户要求用关系、预加载/批量关联和数据库聚合尽可能减少慢 SQL；实现必须以减少数据规模和 SQL 次数为目标，而不是机械添加外键。
- 截图对应 `internal/agenda/handlers.go:40` 的 `/api/agenda/summary`，不是搜索接口。空项目偏好会保留全项目范围，因此生成无 WHERE 的 `SELECT * FROM task_telemetries`。
- 当前库 35,578 行：Done 34,514、Backlog 1,008、Progress 54、Review 2；97% 数据对活动议程属于历史数据。
- SQLite CLI 全量输出到空设备稳定约 70ms，活动字段投影约 10ms；GORM 267.7ms 包含 35,578 个完整结构体的扫描/分配。
- `GenerateAutonomousDecisions` 对 Done/Review 共 34,516 项逐条调用 `latestCommitReference`；每项至少查询一次 `git_commit_logs`，命中后还可能查询一至两次 `notifications`，形成主要 N+1 放大器。
- `DecisionDashboard` 与全局挂载的 `DecisionEventCenter` 都以 15 秒周期读取同一接口；本轮先修后端单次请求成本，前端去重作为独立边界，避免触发强制 UI 修改门禁。
- 上一轮 localhost curl 因 8080 进程在检查后退出而失败；本轮采用隔离 handler/数据库反馈环，不把易失运行时作为唯一验收。
- `TaskTelemetry.TaskID` 是字符串主键；`GitCommitLog.TaskID` 只有普通索引，`Notification.TaskID` 当前没有索引，三个模型未声明 GORM association。
- 真实库 `PRAGMA foreign_keys=0`，且已有 1 条 GitCommitLog 和 5 条 Notification 找不到对应 TaskTelemetry。直接启用/添加级联外键会引入迁移与审计删除风险，本轮不做破坏性约束迁移。
- 当前关系适合以 `task_id` 为稳定引用键做 ORM/仓储级批量关联，并增加 `notifications(task_id,type,created_at DESC)`；数据库实体外键需先定义孤儿保留/归档策略后独立迁移。
- `CONTEXT.md` 将 Git commit/通知归类为证据链/源事实，不能因 TaskTelemetry 当前投影删除而级联销毁，因此 `ON DELETE CASCADE` 与领域语义冲突。
- `telemetry.IsWeakSemanticCommit` 自身还会按 commit 再执行一次 `notifications COUNT`；因此旧链路不仅是每事项一次提交查询，命中非显式引用时还会继续放大。
- ADR 0002 明确列表查询不得无界读取；最近历史采用固定上限符合既有架构约束，初始上限与人工审计接口统一为 200。
- 实现采用不参与迁移的 `automaticDecisionTaskRecord` 只读关联模型，显式声明 TaskTelemetry -> GitCommitLog/Notification 的 `task_id` 关系；GORM Preload 将证据查询稳定为两个 IN 查询。
- 活动议程保留全部非 Done 事项，但只读取风险评估实际需要的 12 个字段；历史只读取 4 个任务字段和最近 200 条，避免 Description 等大字段进入内存与 JSON 链路。
- 真实库副本的查询计划已全部命中新索引：活动/历史使用各自 partial index，项目去重使用 covering index，Git/Notification 批量关联使用 `(task_id,type/action,created_at)` 复合索引。
- 真实库副本端到端 handler：`status=200`、`22.17ms`、`595,152B`、`5 SQL`、`1,064 agenda_items`、`200 auto_decisions`；旧截图单条全量 GORM 查询即为 `267.7ms / 35,578 rows`，且尚未计入旧 N+1 与 JSON。

---

## 2026-08-14 - 方案生成失败后手动重试

- 当前卡片已经区分 queued/running/retry/failed，但 `failed` 只把 `last_error` 原样作为详情展示；Cloudflare 524 的整段 JSON 因此成为默认正文，且没有恢复动作。
- 自动失败边界在 `solutions.Module.FailPolish`：claim 后 attempt 递增，attempt 小于 3 时回到 queued，达到 3 后进入终态 failed。人工重试必须建立在这个持久化终态上，而不能通过前端猜测 attempt 数。
- 现有方案 UI 已明确取消默认“重新润色”和 Agent candidate 分支；本次只能为“没有 canonical working 且最新任务终态失败”提供恢复，不得让已有人工/Agent 主方案重新进入隐式润色流程。
- 四方 UI 评审一致选择原地错误恢复：保留错误卡片层级，默认显示简短可行动摘要，把 provider 原文放入可选技术详情，按钮复用项目控件语言并覆盖 pending/disabled/focus/mobile 状态。
- 红灯已把后端与前端拆成两个可证伪反馈环：模块测试要求新一轮 job、旧失败审计保留和重复点击幂等；前端契约只失败于恢复入口、524 摘要和技术详情缺失，其余八项方案交互保持绿灯。
- 排序假设：H1 同一首次请求的幂等键只会返回原 failed job；H2 现有 polish 接口依赖 working revision，不能承担首稿恢复；H3 原始 provider payload 直出与无操作是失败状态投影缺口，而非轮询/编辑器问题。
- H1 命中：`createPolishJob` 的默认唯一键由 asset/input/prompt/source watermark 决定，冲突时原样返回已有 job；终态 failed 不会自动获得新一轮 ID 或 attempt 预算。
- H2 命中：现有 `/api/solutions/polish` 先调用 `EnsureDraft` 再 `RequestPolish/loadWorking`，会把首稿恢复错误地转成已有工作方案润色，违反“无人工草案时 Agent 直接成为 v1”的既有边界。
- H3 命中：workspace 按 job ID 倒序投影，轮询能稳定拿到最新终态；缺口精确位于 `polishStateFor(failed)` 原样返回 `last_error`，模板没有 action/details 分支。
- 主库只读事实复现了同类终态：FEL2WD-2034、HIT-900、QZ-1666 等均为 `failed/attempt_count=3`，524 payload 与用户贴出的结构一致；无需调用真实 provider 即可设计和验证恢复路径。
- 人工重试采用“新 job 引用旧 job ID 的唯一幂等键”而非重置原行：`solution:manual-retry:<failed_job_id>`。这样旧 attempt/error 保持审计不可变，双击收敛到同一新 job；若新 job再次终态失败，可基于它自己的新 ID 开启下一轮。
- 新命令只接受同 demand 最新终态失败且 asset 尚无 working revision；新 job 复制失败任务已经绑定的 input/prompt/source snapshot，避免人工点击悄然换输入。API 由 `solution:write` 包装并原子返回刷新后的 workspace。
- 卡片终态映射按 524、502/503/504、存储锁、一般 timeout 和未知错误生成短摘要；原始 `last_error` 仅放入默认折叠的技术详情。恢复操作只由 `canWrite && !working && latestJob.status==='failed'` 投影，最终资格仍由后端事务裁决。
- 点击恢复复用现有 action 锁，按钮文字变为“重新排队中…”，API 同步返回 workspace 后直接进入 queued 投影；409 会静默刷新最新 workspace 并提示状态已变化，避免过期卡片继续误操作。
- 完整验证通过：`internal/solutions` 与 `internal/server` 套件、前端方案契约 9/9、Svelte/TypeScript、生产构建、diff hygiene、Impeccable `[]` 和 Finesse P0=0；构建只保留仓库其他组件的既有 warnings 与 chunk-size 提示。
- 隔离认证浏览器在 1440px 复现 524：默认可见文本不含 provider JSON，展开“查看技术详情”后原文可读；点击后 100ms 内按钮为禁用的“重新排队中…”，900ms 假响应后原地显示“已重新加入生成队列 / 排队中”，URL 和 tab 不变、重试按钮消失。
- 390px 实测按钮 `297×44px`，文档 `scrollWidth=379 < innerWidth=390`，失败摘要与技术详情入口均完整；宽屏与手机控制台 error/warn 均为 0。隔离 fixture、Vite 和浏览器 tab 已清理，主服务、数据库、Jira 与 LLM 无写入。

---

## 2026-08-14 绩效 v6.0 数字资产算分收敛

- v6.0 的正式计算固定为 D01 交付结果 35%、D02 交付可预测性 20%、B01 工程质量 45%；指标合格贡献直接累加，Git 不再占正向权重，只在 10 个稳定指纹 Commit 且 3 个完成需求后形成最多 10 分风险扣分。
- 单需求权重已收敛为 `MIN(10, 规模点 × 需求等级 × 项目权重 × 责任份额)`；复杂度、项目阶段和默认核心研发角色不再重复放大同一事项。
- Jira 搜索字段新增 `parent`/`issuelinks`。Bug 优先使用 parent；无 parent 时只有唯一一个需求类型链接才建立来源，多个候选保持未归责，避免任意选一个人扣分。
- 来源需求的责任人取需求 Done 时的 canonical core-member owner；Bug 修复负责人仍只保留修复活动证据，不自动成为缺陷责任人。正式缺陷归责可覆盖同一 Bug 的 Jira 推断，防止双计。
- Jira-only 缺陷损失使用严重度、来源需求归责和延期/重开闭环系数；缺少逃逸阶段时采用测试阶段中性值，并把工程质量最高限制为 4 档。没有独立缺陷覆盖确认时即使零 Bug 也最高 4 档，防止缺证 100 分。
- 工程质量分母只使用完成后已暴露至少 30 天的需求权重，且总权重至少 10；D01 至少 5 个需求，D02 至少 3 个到期需求。缺截止日、估算或暴露量时保留 N/A/降级原因，不填零。
- explanation seam 新增交付/质量分量和结构化代码风险明细；最近成员表一级收敛为参考分、交付、质量、风险、证据状态，旧需求/Bug/Commit 数和事项系数保留在共享 wide Modal 详情。
- 配置和服务归一版本已升为 v6.0；旧快照仍按原公式版本和 JSON 不可变保留，最近成员投影只会在下一次后台滚动后切到 v6.0。
- 定向 Go 包测试通过；服务定向测试在允许回环监听后通过。前端契约 7/7、`pnpm check` 0 errors/84 条既有 warnings、生产构建通过。

## 2026-08-14 每日 Jira 跳转入口合并

- 当前 header 同时显示静态 `.jira-key` 和独立“在 Jira 打开”链接，两者都指向同一 Jira 事项，形成重复入口和额外视觉重量。
- `getJiraUrl(selectedItem.task_id)` 已提供条件分支，最小改法是在 meta 行内按 URL 存在与否切换链接/静态编号，不需要修改 URL 生成或后端。
- 上一轮 header grid 为 `meta action / title title`；移除 action 后应同步收敛为 `meta / title`，避免保留空 action 列或无效移动端 action 区域。
- 链接文本只显示 Jira 编号，但需通过 `aria-label="在 Jira 中打开 {key}"` 补足外链目的；保留 `target="_blank" rel="noopener noreferrer"`。
- 既有 `.jira-key` 负责尺寸、颜色和圆角，新增 `.jira-link` 只负责 cursor、hover、focus、active 和文本装饰，静态 fallback 不获得交互暗示。
- 实施后 header 从 `meta action / title title` 收敛为 `meta / title`；独立“在 Jira 打开”DOM 数量为 0，标题宽度仍为 header 的 100%。
- 登录态 `HR-4090` 的 Jira 编号链接解析为 `https://jira.westwell-lab.com/browse/HR-4090`，真实点击新增一个同 URL、正确事项标题的 Jira 页签；验收后已关闭该页签，未触发业务写入。
- 手机断点保持 24px 视觉胶囊，通过 44px meta 行与 `::after` 命中区满足触控目标；实测 header/meta 横向溢出均为 0，标题位于 meta 下方，宽度比例为 0.978。
- 手机断点的页面级横向滚动来自既有 Daily Jira 表格/外壳基线，本轮 header 局部没有溢出且未扩大该问题；不把链接合并任务扩展为整页响应式重构。
- 定向契约与刷新回归 6/6，Svelte/TypeScript 0 errors（83 条既有 warnings），生产构建、Impeccable type/layout、Finesse P0 和 diff hygiene 均通过；宽屏 console 无 warning/error。

## 2026-08-14 每日 Jira 右侧标题宽度修复

- 截图中的长标题只在右侧卡片左半区换行，Jira 操作下方留出大片无语义空白；这是局部结构约束，不是字体、标题文本或右栏总体宽度不足。
- `DailyJiraAudit.svelte` 当前把 `.inspector-meta-line` 和 `h3` 包在 `.inspector-identity`，再与 Jira 链接放进 flex header；所以按钮宽度会约束包含标题的整个左侧 flex item。
- 最小正确模型是两列两行 grid：第一行 meta/action，第二行 title/title。给旧 wrapper 增加 `width` 或 `flex:1` 只能扩大左列，仍不能让标题使用按钮下方空间。
- 独立 Impeccable 布局审计确认页面双栏、inspector 区段与密度层级均合理；只需修改 header owner，不动 `.audit-grid`、facts、决策表单、历史或滚动。
- Impeccable 机械 detector 返回 `[]`，Tailwind 任意 spacing/z-index 无匹配；该结果不能证明运行时文字宽度正确。
- in-app browser 没有登录态，但现有 Chrome 会话保留了受控认证页面；复用该 tab 后用截图中的 `FEL2WD-2037` 完成长标题验证，没有登录、填写或提交表单。
- 实施后桌面 header 为 `370.68px`，标题同宽、比例 `1.0`；`760px` 堆叠态比例 `0.985`，`390px` 单列态比例 `0.978`。三档操作按钮与标题矩形均不重叠，文档 `scrollWidth === clientWidth`。
- 390px 视觉复核确认信息顺序为标签、完整标题、Jira 操作；事实区、早会决策和历史仍按原顺序显示。宽屏标题由原先受操作列约束的半宽视觉恢复为两行全宽标题。
- 新增症状级契约先 3/3 红，再在实现后 3/3 绿；与既有实时刷新回归合计 5/5。Svelte/TypeScript 0 errors（83 条既有 warnings）、生产构建、Impeccable type/layout、Finesse 和 diff hygiene 均通过。

## 2026-08-14 右侧方案预览高度与冗余胶囊清理

- 这是上一轮方案阅读/编辑收敛后的右侧检查器 follow-up；用户明确要求预览正文填满剩余高度并删除其下两个胶囊，不应扩大为页面重构。
- 既有项目契约要求先检查 shell/workspace、双列 grid、检查器与唯一滚动 owner，再决定是否增加局部高度；不能用等高拉伸或固定空高掩盖真实内容高度问题。
- 现有 Phase 41 基线是轻量、事实优先的产品 UI；页面/检查器是唯一主要 surface，冗余 field pills 应移除而不是换一种胶囊样式。
- 待当前源码与浏览器确认：预览由 `SolutionWorkspace.svelte` 自身高度、父 `DemandKanban.svelte` 的检查器 tab，还是共享 `MarkdownWorkbench.svelte` 的 `autoHeight/minHeight` 共同限制。
- 登录态 DG-394 宽屏几何红灯已复现（2382×1036）：右侧 inspector 底边 `1014.67`，`schedule-solution-inline` 底边 `1014.11`，但 `solution-workspace` 底边只有 `971.73`，形成 `42.94px` 未使用空间；Markdown 底边到 inspector 还有 `86.93px`。
- `solution-workspace` 当前网格行是 `40px 420px 29.99px`，分别对应编辑工具条、固定 420px Markdown 与两个胶囊 footer；父 `.schedule-solution-inline` 已正确 `flex:1 1 auto` 且高度 576px，因此 shell/inspector 不是首要所有者。
- `SolutionWorkspace.svelte` 只有 `DemandKanban.svelte` 一个调用方；方案预览下两个胶囊精确来自 `.solution-facts` 的“当前来源快照”与“原文/Gzip 保存”，移除不会改变保存编码或来源事实本身。
- Impeccable 独立机械 pre-scan 对两个目标文件返回 `[]`；Tailwind 任意 spacing/z-index 扫描无匹配。静态检测无法判断运行时剩余高度与滚动所有权，浏览器几何仍是权威反馈环。
- 共享 Markdown 的 `.preview-pane` 已是正确的唯一正文滚动 owner：当前 `clientHeight=419`、`scrollHeight=7460`、`overflow-y:auto`，可滚到 `scrollTop≈7042`；修复不应把滚动提升到 `.schedule-solution-inline` 或页面。
- 视觉复核显示下方空白不是正文缺失：预览已滚到第 14 节末尾，两个胶囊之后仍留一段无信息区域。合理层级是“编辑入口 + 占满余高的 Markdown”，不再保留事实胶囊 footer。
- Impeccable 主观布局评估与机械扫描均确认 shell/inspector 不是根因；主观评估额外发现 `SolutionWorkspace` 的 14/6/28/10px 局部节奏偏离现有 4/8/12/16 token，但删除胶囊后只需把主 gap 收敛到 16px。
- 最终滚动决策与评估候选有一处分歧：不采用 `autoHeight` 让外层承载 7460px 正文，而是在宽屏让固定尺寸 Markdown 工作台弹性填满剩余区域，继续由 `.preview-pane` 内部滚动；`<=1280px` 则恢复自然 420px 高度，避免堆叠布局形成固定空高。
- 实施后宽屏（2382×1036）几何转绿：Markdown 高度由 420px 增为 488.37px，底边到 inspector 只剩 16.55px 合法内边距；`.preview-pane` 为 `clientHeight=487`、`scrollHeight=7460`、`overflow:auto`，页面无横向溢出。
- 900×900 与 390×800 的媒体查询都恢复 `grid-template-rows: 40px 420px`、`workspace flex: 0 0 auto`、inline `overflow:visible`；长正文仍只在 420px 预览区内滚动，页面 `scrollWidth=clientWidth`。
- `.solution-facts`、`currentSources`、“当前来源快照”和“Gzip 压缩保存/原文保存”已从方案预览移除；保存编码、revision/CAS、编辑 Modal 和方案状态机未改动。
- 症状级契约 14/14；Svelte/TypeScript 为 0 errors（83 条既有 warnings），生产构建通过；Impeccable 全量/layout 均为 `[]`，Finesse 无 P0（`DemandKanban` 的既有纯黑白 P2 位于本轮未触碰行）。

## 2026-08-14 方案编辑弹窗扁平化与 Markdown 表头默认隐藏

- `MarkdownWorkbench.svelte` 当前把 `label`、`description` 与 mode switch 固定渲染在同一 `.workbench-toolbar`；单一模式虽已隐藏 mode switch，但仍保留纯元信息表头。
- `label` 同时用于根 `aria-label`、CodeMirror 编辑器 `aria-label` 与预览区域 `aria-label`，因此只能默认隐藏视觉 `.document-identity`，不能删除 label 语义。
- 共享组件需要两个正交能力：`showDocumentMeta=false` 控制可视 label/description；`embedded=false` 控制外框。模式切换与状态 footer 不应跟随元信息一起消失。
- `SolutionWorkspace.svelte` 的 Modal 正文当前由 `.solution-editor-body` 增加 20/22px 内边距，工作区自身再绘制 border/radius/background，形成用户指出的二次包裹层。
- 方案弹窗只需传入 `embedded={true}` 并把正文容器改为无视觉层；冲突、错误、脏关闭提示仍是独立状态块，Modal 的标题、关闭与 footer 保持不变。
- 其他 Markdown 调用方即使不显示可视元信息，仍保留各自外部 section heading/comparison label 或可访问名称；如后续确需展示，可显式 `showDocumentMeta={true}`。
- 登录态首轮浏览器验收确认宽屏和 900px 已是单一边界；390px 下旧 `calc(100dvh - 250px)` 让 550px 工作区进入仅 512px 高的 Modal body，形成 38px 外层溢出和内外双滚动条。最终改为 `calc(100dvh - 288px)`，Modal body 为 `512/512`（client/scroll），长正文仅在 CodeMirror 的 `484/6236` 区域滚动。
- 最终登录态验证覆盖 1440×1000、900×900、390×800：三档均无视觉 label/description、无内层边框/圆角/背景、文档横向溢出为 0；关闭按钮和移动 footer 保持约 44px 目标，滚动到长正文中段时 dialog/footer 持续可见。
- 验证前后 DG-394 均为 asset revision 3、working revision 61、draft version 1；只打开和关闭编辑器，没有保存、发布或修改业务数据。

## 2026-08-14 - 方案编辑弹窗扁平化与 Markdown 表头默认隐藏

- 上一轮已把方案主页面收敛为只读预览，并让“编辑方案”打开共享 wide Modal；保存、发布、冲突与脏关闭保护都已经存在。本轮是该流程的层级 follow-up，不应重做状态机或后端契约。
- 用户明确要求编辑内容直接显示，说明 Modal 内仍存在重复视觉容器。设计边界应是“Modal 为唯一强边界，MarkdownWorkbench 为内容组件”，通过结构/组件 prop 解决，不能用负 margin 或透明边框掩盖。
- 用户询问 Markdown 表头 label/description 默认隐藏。合理的共享契约是视觉表头默认关闭、需要者显式开启，同时维持编辑区可访问名称；这比在 `SolutionWorkspace` 单点 CSS 隐藏更可维护。
- 当前结论来自前序实现与项目记忆，仍需用当前源码核对 prop、DOM、全部调用方和既有测试，避免影响需要表头的独立工作台。


## 2026-08-14 - 算分面板布局静态审计（机械检测前）

- **页面所有权：** “算分面板”由 `web/src/components/PerformanceCalculationGuide.svelte` 独立承载；共享 Phase 41 外壳和 `Modal` 已经提供正确的卡片、表格、按钮与详情容器，本轮无需改共享主题或后端接口。
- **间距与节奏：** 根 `.wa-admin-section` 使用 16px 节奏，但页面内部同时存在 `.status-grid` 12px、`.guide-layout/.guide-main/.content-section` 16px、卡片 padding 18px，标题又有 `2px` 偏移；横向起点没有落在同一 4px 基线上。应统一为 16px 主节奏、16px 卡片内边距，并去掉只为目测补偿的 2px 偏移。
- **层级：** 页首、四张状态卡、规则主区/审计侧栏、系数区、快照区的业务顺序正确；问题不在重排模块，而在状态值和章节头的基线不一致。`.metric-time` 从其他状态值的 24px 降为 15px，导致四卡主值视觉基线和权重失衡；时间应保持紧凑但用统一主值行高和固定值区承载。
- **网格：** `.guide-layout` 的 `1fr + 280–336px` 与 sticky inspector 所有权明确，980px 下单列也合理；但四卡在 1180px 直接降为 2 列，桌面窄宽会产生过高首屏。保留 4/2/1 响应式，但让每张状态卡使用同一三行网格，确保标签、值、说明横向对齐。
- **表格对齐：** 共享 `.wa-admin-table` 默认全部左对齐。指标表的权重/最低样本、快照表的参考分/等级/覆盖率/样本/天数和三组计数均属可比较数值，却没有列级数值对齐；应添加语义 class，使表头与单元格统一右对齐并使用 tabular numbers，人员、状态、时间仍按文本阅读方向左对齐。
- **密度：** 指标表 1320px、快照表 980px 的单一横向滚动边界正确，不能通过缩小字体或压扁列宽消除滚动；应优化列宽、数字对齐、表头分组和行内次级信息，保持 44px 左右可读行高。
- **弹窗：** 详情摘要 3 列和元数据 2 列结构合理，760px 单列回退存在；详情表同样缺少数字列对齐。应只补数字列语义与摘要基线，不改变 wide Modal 的关闭、滚动和只读行为。
- **响应式与可访问性：** 760px 已将标题、系数、摘要和元数据切成单列，刷新按钮达到 44px；需在真实 1280/900/390 三档验证文档横向溢出、表格唯一滚动边界、焦点环和成员详情关闭路径。

## 2026-08-14 - 算分面板布局实施与浏览器验收

- 组件本地建立固定状态卡三行：18px 标签、36px 主值、至少 35px 说明；加载骨架同步到 132px。1280px 实测四张卡的标签、主值、说明 Y 坐标完全一致，长审计说明不再把主值顶偏。
- 页首补偿、内容卡、检查器和系数区统一到 16px 基线；1280px 实测状态卡、公式标题和系数标题内容起点均为 x=303px。
- 指标、快照和详情表引入语义数值列：权重、参考分、覆盖率、样本、天数和成对计数右对齐并使用 tabular numbers；状态/等级居中，时间保持左对齐与不换行。
- 快照表从被动 980px 挤压改为 1210px 明确列宽。原本约 65px 的可比较列现在为 82–112px，计算时间为 166px；人员列在表格横向滚动时 sticky 保持可见，overflow 仍只属于 `.snapshot-table-shell`。
- 1280px 文档 `scrollWidth/clientWidth=1280/1280`；900px 为 `900/900`，卡片为 2×2、主区和检查器自然单列；390px 为 `379/379`，正文 351px、卡片单列、刷新按钮 44px，快照表 `clientWidth=349/scrollWidth=1210` 且不扩张文档。
- 390px 成员详情为 366px 宽单列 Modal，关闭按钮 44×44，正文/摘要无文档横向溢出；读取详情仍不触发后台计算。
- 定向契约 4/4、Svelte/TypeScript 0 errors、生产构建通过；Impeccable 精确检测 `[]`，Finesse P0=0 且目标文件无 findings。构建保留仓库其他组件既有 warnings 和 chunk-size warning，本次目标组件没有新增诊断。

## 2026-08-13 - 绩效 v5.0 与解决方案卡片初始盘点

- 当前 v4.2 在 `internal/performance/rules.go` 定义 10 个指标；C03/C05/C06/C08/C09/C10 多数依赖 `performance_evidence_facts`，真实库目前没有可用正式事实，造成长期 N/A 和不可执行感。
- `TaskTelemetry` 只有当前 assignee/status、completed_at、due_date、project 和难度快照；Jira 搜索没有抓取 priority、severity 或 changelog，因此不能准确计算负责人流转时间、重开、延期次数和冻结基线。
- `GitCommitLog` 只有 webhook 接收时间、SHA、作者和关联任务；真实库 390 条 push 记录对应 343 个唯一 SHA、46 个任务。相同 SHA 首先是采集去重问题，不等于重复代码；真正重复变更需要稳定 patch/diff 指纹和排除规则。
- v5.0 采用 3 维度、8 指标：加权完成率 25%、加权按期率 15%、需求流动效率 10%、责任加权缺陷密度 20%、Bug 修复周期达成率 10%、Bug 重开/复发率 10%、重复变更率 7%、Commit 密度风险 3%。
- `CONTEXT.md` 的“基础试算分按当前可用指标权重归一”已与 v4.2 不放大局部证据冲突；v5.0 应修正为合格贡献直接求和，证据不足不发布正式分。
- 前端必须保持现有 Phase 41 产品管理台，解决方案卡片问题先证明是数据/映射/渲染/CSS哪一层，不进行视觉重构。


## 2026-08-13 - 绩效 100 分异常修复

- 判定表允许最终能力分为 0—100，但单指标采用 1—5 分档；这不等于任意一个达标指标都可以独立归一为成员 100 分。单项贡献必须先按 `分档 × 20 × 权重` 落入总分权重预算。
- v4.1 的实际放大链路是：`scoreForRatio` 把 5 档直接变成 100，成员聚合得到 `100 × 0.2 = 20` 后又除以合格覆盖率 0.2，最终回到 100。尹祖雷、岳颖颖、朱家聪、李厚奇均是只有 C01 20% 覆盖却显示 100，证明问题在聚合公式，不在前端格式化或陈旧快照。
- v4.2 固定四层语义：原始比率仍可为 100%；指标分只在 1—5；加权贡献为指标分乘 20 再乘适用权重；成员参考分直接累加样本达标贡献，不除以当前覆盖率。正式分仍受既有覆盖、核心指标、样本和暴露门槛约束。
- 回归使用历史 Jira 完成需求构造 C01 3/3：修复前成员参考分为 100；修复后原始比率 100%、分档 5、指标分 5、加权贡献 20、成员参考分 20。测试同时防止 `point_level` 因量纲切换退化为 0。
- 最新 v4.2 run `perf-1786608766943626000-2c3ed79481c5` 完成 14 位 core member 快照。当前最大参考分 20、100 分记录 0、最大指标分 5、最大分档 5、最大单项加权贡献 20；正式分仍全部 N/A，未放宽发布门槛。
- 登录态页面验证当前列表最高显示 20.00。李厚奇详情的 C01 明确显示“100% / 5 分档 / 5.00 / 20.00”，证明原始百分比没有被误当成员总分；浏览器控制台无 error/warn。
- 旧 v4.1 快照作为不可变审计证据继续保留。最近人员评分通过既有最新成员投影只显示 v4.2，不能为了隐藏历史错误而删除或覆盖旧审计行。
- 用户再次指出 100 分后，修正了上一轮“100 可能是参考值”的错误判断：今后必须逐层核对原始率、指标档位、加权贡献和成员总分，不能仅凭最终值数学上可达 100 就判为合理。

## 2026-08-13 - 最近人员评分快照成员去重

- 重复并非后台为同一次运行写入了重复成员，而是 `Module.Explain` 把 90 天保留期内的所有历史快照按时间倒序直接返回；每次定时运行追加的新快照都会让同一成员在“最近快照”表再次出现。
- 当前表是成员当前态投影，不是运行历史列表。正确边界是在后端按 canonical core-member identity 去重，并利用 `created_at DESC, id DESC` 的稳定顺序保留第一条，也就是每位成员的最新持久化快照。
- `snapshot_limit` 现在限制不同成员数。扫描按 100 条分批继续，直到收集到指定数量的不同 core member 或历史耗尽，避免旧记录挤占返回配额。
- 不删除、不覆盖数据库历史。回归测试为 Alice 写入两个不同 run 的快照，列表只返回较新 ID，同时旧 ID 仍能通过 `ExplainSnapshot` 查询，数据库仍有两行。
- 当前真实库保留 287 条绩效快照、91 个历史 subject、15 次运行；最新 run `perf-1786606758730057000-d5ca28d1ffab` 为 14 行、14 位不同 core member。这一差异证明历史审计与当前唯一成员投影同时成立。
- 重启根目录服务后，登录态“计算说明”页实测最近快照为 14 行、14 个唯一成员，全部时间为 2026/08/13 15:39:18；刘子翔详情指向最新 run，参考分 48.57、正式分 N/A。
- 修复只修改后端只读投影和模块测试，未调整表格列、视觉样式、弹窗、评分公式、调度开关、core-member 范围或审计持久化规则。

## 2026-08-13 - 绩效 v4.1 全员零分回归修复

- 真实根因不是数据库没有 Jira 历史，而是 C01/C02 共用一个 `due_date` 前置门槛：考核周期内已完成、但 Jira 未填写到期日的需求在进入 C01 前即被丢弃。
- 判定表的按期率 C02 必须保持 due-only；项目验收达成率 C01 则应接受“周期内完成或周期内到期”的有效需求。v4.1 按这一边界恢复历史 `resolutiondate`，不把未到期在途事项混入按期率。
- “能计算”和“能正式发布”是两个状态。低于单指标最低样本时保留原始比率、能力档、指标分和来源引用供详情审计，但 `sample_qualified=false`，不得进入成员参考分、证据覆盖率、核心齐套或正式分。
- 成员参考分只对样本达标指标按适用权重归一；正式分继续要求 C01/C02/C04 核心指标均达标、证据覆盖不低于 70%、去重样本不少于 5、暴露期不少于 30 天。SQL `NULL` 始终显示 N/A，不转换为 0。
- 最新真实 v4.1 run `perf-1786604653763225000-0a5f95dfda3e` 完成 14 名 core member 快照：8 人有非零参考分，6 人 N/A，正式分 0 条。审计事件为 run_started 1、snapshot_created 14、run_completed 1、retention_applied 1。
- v4.1 当时把尹祖雷、岳颖颖、朱家聪、李厚奇仅有 C01 20% 覆盖的结果归一为 100；后续复核证明这不是合理参考分，而是局部证据被覆盖率二次放大的公式缺陷，已由 v4.2 修复并重算。姜昊良的低样本指标仍只供解释，不进入成员参考分。
- 刘子翔详情证明历史 Jira 已进入计算：C01 8/3、80%、参考指标分 60，来源同时包含 `resolved` 和 `due`；C02 2/3 只供参考，C07 3/3 达标，成员参考分 48.57、正式分 N/A。
- UI 继续复用共享 wide Modal，列表主列改为“参考分”，详情同时展示参考分、正式分、样本资格、Jira 引用、需求/Bug 系数过程、正式证据和未计入原因；页面读取快照不会触发重算。
- 登录态浏览器在可用的 1280×720 固定视口验证弹窗宽 960px、左右安全边界正常、内部滚动承载长证据表。该 Browser 绑定不提供 viewport override，窄屏通过页面和共享 Modal 的 `<=760px` 单列/内边距规则、生产构建与源码契约补充验证，未虚构 390px 运行结果。
- 全量 Go 测试和 `go vet` 通过；绩效前端契约 3/3、生产构建、Impeccable `[]`、Finesse P0=0。仓库级 Svelte check 唯一阻断来自本任务外的 `SolutionWorkspace.svelte` 9 个既有空值错误。

## 2026-08-13 - 任务跟踪卡片底部对齐

- 2048×924 视口覆盖下浏览器实际 CSS viewport 为 2275×1026。共享 `.workspace-frame` 高 958.67px、`padding-bottom=22px`，内容目标底边为 1004.67px；该 shell 几何与其他 viewport-fit 页面一致。
- 任务表视图已经正确：`.task-console`、状态 workbench、左表格卡、右检查器 bottom 均为 1004.67px，左右高度均 607.40px。说明共享 shell、FunctionalWorkspace 和 TaskKanban 根高度预算没有缺陷。
- 执行追踪明确复现：根节点和 workbench bottom 仍为 1004.67px，但左表格卡因 `height:clamp(...,660px)` 在 938.18px 提前结束，右检查器因 `height:auto/align-self:start` 在 989.68px 提前结束；两卡距统一底边分别多出 66.49px 和 14.99px。
- 根因是 `TaskKanban.svelte` 文件末尾桌面 cascade 先建立统一 `height:100%/stretch`，随后执行追踪专属规则又把根改为外层滚动、workbench 改为 auto/start/visible、左卡改为固定 clamp、右卡改为 sticky/auto/visible，直接撤销共享契约。
- 最小边界在 TaskKanban 执行视图，不在 shell。状态视图是正确对照；修复应让执行 workbench/左右卡继承同一剩余 grid row，并让表格 shell和检查器成为独立滚动所有者。1180px 以下的既有堆叠规则保持不变。
- 已按该边界完成修复：执行追踪不再取得外层根滚动或固定/自适应高度例外；左右卡统一占满剩余行，长表格与长检查器分别内部滚动。执行追踪正文与列表的“完整显示”覆盖仍保留，不会重新隐藏详情。
- 登录态宽屏复验中，执行追踪根/工作台/左表格/右检查器 bottom 均约 1004.67px，左右 bottom delta=0；表格 shell 的 `scrollHeight=2018`、`clientHeight=626` 且 `overflow:auto`，证明长列表仍由内部区域承载。任务表左右卡也维持同一底边。
- 1180、760、390 三档均恢复单列自然高度，任务表与执行追踪在 760/390 均无文档横向溢出；最终浏览器日志为空。宽屏与 390 截图已保存至 `outputs/ui-validation/`。


## 2026-08-12 - 全局方案治理中心

- 方案中心不是需求详情页的放大版，而是发布方案的权限化读模型与治理工作台；需求侧方案资产仍是唯一事实源。
- 标准化必须产生可审计、可拒绝、可回滚的提案，不能让模型直接改写历史修订或各项目当前方案。
- 两轮对比语义固定为：第一轮判断需求是否等价或足够近似，第二轮只在第一轮通过后判断方案核心是否兼容、差异是否属于项目变量以及是否存在冲突。
- 为避免全量两两比较，先用归一化文本和项目/标签事实召回有限候选，再创建幂等对比；后续可在不改变接口的前提下替换为向量召回。
- 权限顺序必须是“可见项目过滤 -> 检索/候选 -> 详情/统计”，不能先跨项目召回再在响应末端隐藏。
- UI 三方会审一致选择 Phase 41 product register：筛选工具条、主列表、右侧详情检查器；不使用弹窗详情、页面 hero、装饰动效或第二套 Select。
- 目录只保存发布修订的引用、归一化事实和搜索 token；正文始终从对应发布修订解压读取，避免 Markdown 双写及版本漂移。候选召回走 `(token, entry_id)` 索引并限制候选数，不做全库 N² 比较。
- 两轮比较分别绑定 `solution_compare_requirement` 与 `solution_compare_compatibility` 的已启用版本；同项目对比允许使用该项目模板，跨项目对比固定使用全局模板，prompt ID 会写入审计记录。
- 标准方案可见性取全部来源项目可见范围的交集；任何一个来源项目不可见时，标准详情、比较对手和提案均不返回，防止通过治理结果侧漏项目事实。
- 发布成功会写幂等同步任务，周期 reconciler 修复漏同步；接受提案只新增不可变标准修订，不覆盖来源方案。两轮模型调用复用无整体超时客户端，仍保留 context 取消语义。
- 浏览器验证暴露了详情容器的滚动所有权：切换方案/标准时若不归零，会继承前一正文位置。现已在选择与模式切换路径统一重置，并验证标题重新进入视口。
- 所有方案中心与提示词配置下拉框均复用共享 `Select`；1280/1024 使用列表加检查器，760/390 自然堆叠且无横向溢出。当前源码完成但运行进程尚未受控重启。

---

## 2026-08-12 - 大模型请求取消总超时

- 统一模型客户端 `internal/llm.Client` 当前仅对流式请求返回 `http.Client{}`；非流式 `Generate` 使用 `http.Client{Timeout: 90s}`。后台方案 worker 走非流式路径，因此固定 90 秒会直接形成 `Client.Timeout exceeded while awaiting headers` 并进入失败/重试。
- AI 配置探活虽然不是业务生成，但同样调用真实模型，并同时套了 8 秒 context deadline 与 8 秒 HTTP timeout；应改为继承配置请求的 `r.Context()`，只响应用户断开，不设置任意截止时间。
- 带附件的需求解构在生成前通过 Provider Files API 上传，当前客户端有 90 秒整体 timeout；上传使用来访请求 context，移除整体 timeout 后仍可由用户断开取消。
- 流式 `Stream` 默认客户端已经没有整体 timeout；`demand_spec` 以 8 秒 ticker 发送 heartbeat，这只是保活，不是截止时间。前端 `ReadableStream` 循环没有 `AbortController`/Promise timeout。
- 服务端使用 `http.ListenAndServe` 默认配置，没有 `WriteTimeout`；仓库内也未发现 `proxy_read_timeout` 等本地反向代理截止配置。
- solution job 的 10 分钟 stale lease 是任务崩溃回收，不会取消当前模型 context；本轮不把它改成生成超时。多实例下的 lease 续期属于独立一致性议题。
- 为防止未来调用方重新注入 `http.Client{Timeout: ...}`，统一 LLM 模块会复制调用方客户端、保留 Transport/CookieJar 等字段并强制将副本 `Timeout` 归零；不会修改调用方持有的原对象。

---

## 2026-08-12 - Agent 首次方案直用与人工草案边界

- 用户观察到无人工草案时产生空 v1、空 v2、Agent candidate，要求首次 Agent 成果直接成为主方案。
- 已确认目标语义：Agent 是“首稿生成器”而不是默认候选分支；人工草案存在后，不再后台自动润色，也不默认显示 Agent candidate 相关 UI。
- 历史 revision/candidate 只调整默认选择与后续生成策略，不做删除或数据清理。
- 当前生产库的成功样本一致为：v1=`human_draft/jira-sync`（94-201 bytes 的标题/描述占位），v2=`agent_candidate/agent`（约 5.8-8.8 KB 的完整正文），asset working 仍指向 v1；数据库事实排除“纯前端标签错误”。
- 当前库没有任何非 `jira-sync/agent` 作者的方案修订；因此现存资产均可按系统生成链兼容处理，但实现仍必须用明确 kind/作者判定保护未来真实人工草案。
- 根因 H1 命中：`ensureSolutionDraftForTask` 无条件调用 `EnsureDraft`，将自动占位误标为 `human_draft` 并推进 working。
- 根因 H2 命中：`CompletePolish` 无条件写 `agent_candidate/candidate` 且只推进 sequence，不推进 working。
- 根因 H3 命中：Jira 同步在 eligible 来源存在时没有检查人工草案，持续入队；旧重复任务也可能在首次 Agent 主方案完成后继续产生候选。
- 目标持久化语义：自动输入为隐藏 `system_seed` v0，不计入 history/working；首次 Agent 输出为 `agent_draft/draft` v1 并原子推进 working。之后人工编辑继续生成 `human_edit`，发布/CAS/压缩不变。
- 兼容策略不得删除历史：旧 `jira-sync` 占位重标为 seed；已有最新 Agent candidate 提升为 working，旧队列完成时若 working 已建立则幂等收敛而不追加候选。
- 新接口 `RequestInitialDraft` 把“是否已有 canonical working”收进 `solutions.Module` 的事务边界；Jira worker 不再先制造人工草案，也不再自行判断 candidate 生命周期。
- 旧数据迁移保留原 revision 版本号和所有审计行，所以旧 Agent 方案可能仍标为 v2，但空 v1 已从 workspace/history 隐藏；新生成链路从 Agent v1 开始。
- `SolutionWorkspace` 只投影 canonical working 和后台任务状态；candidate 数据与旧 API 暂留兼容，但默认 UI 不再显示候选计数、候选对照、应用候选或重新润色。
- 隔离浏览器的三态验收通过：无 working 时只有生成/失败状态且 editor=0；旧 candidate 迁移后 editor=1/candidate panel=0；人工草案 editor=1/candidate panel=0。编辑后的 dirty 正文跨 4.5 秒轮询仍保留。
- 现有 8080 未热重启；这是刻意保留的运行边界，因为启动会恢复真实 Jira/LLM/outbox worker。源码修复和迁移已验证，但真实库迁移要等下一次受控启动。

---

## 2026-08-12 - 方案润色长时间中与即时显示收敛

- 用户要求同时回答后台调度模型与修复长时间“润色中”，并将方案 UI 从显式 Markdown 类型/切换收敛为直接即时显示。
- 后端诊断必须区分 job 的 `queued/running/retry/failed/completed`、attempt/next_attempt/last_error 与 candidate 是否存在；只看前端文案无法判断串行或并行。
- 前端处于研发管理 product register；必须保留既有 Phase 41 设计与右侧检查器归属，不将本次类型精简扩展为新工作流或视觉重构。
- 项目既有共享 `MarkdownWorkbench` 已确立 `live` 为默认可见模式，且曾完成 Markdown preview/edit 行为浏览器验证。本轮应优先复用该即时模式，不在 `SolutionWorkspace` 里新造另一套 Markdown 类型切换。
- 当前队列事实：39 个 job 中 33 queued、2 running、1 failed、3 succeeded；DG-394 的 job 15 为 `queued/attempt=0`，约 17 分钟未被认领。因此“一直润色中”至少包含大量排队时间。
- 三个已成功 job 生成了 `agent_candidate` revision，证明 LLM -> 候选持久化链路可用；一个 ZK-949 job 在第 3 次尝试后因 provider 90 秒 headers 超时最终 failed。
- 同一 input revision 存在多 job 的需求包括 ZK-949（3）、DG-352/DG-354/HIT-1002/TH-2536/TPY-6843（各2）；不同 source watermark 导致幂等键不同，增大了串行队列。
- 两条 running 中 TPY-6986 约 7 分钟未更新，TPY-6944 刚在 0.5 分钟前认领；这更符合进程重启留下未达 10 分钟恢复阈值的租约，不能由“running=2”推断真正并发度为 2。
- worker 每轮最多处理 3 个任务，但实现是普通 `for` 循环逐个调用阻塞式 LLM 请求；当前真实执行并发度为 1，`batch size=3` 只是单轮串行处理上限。
- 前端把 `queued` 与 `running` 合并为同一个 `activeJob`，按钮统一显示“润色处理中…”，因此无法区分排队、实际执行和等待重试；这是 DG-394 长时间显示“润色中”的直接呈现根因。
- `GetWorkspace` 已按 job ID 倒序返回最近 20 条，数据库字段本身包含 attempt/start/next-attempt/source watermark；前端本地 `Workspace` 类型只声明了 id/status/error/created_at，丢弃了可用于解释状态的类型信息。
- `SolutionWorkspace` 目前显式开放 `live/edit/split/preview` 四种模式，虽然共享 `MarkdownWorkbench` 的默认已经是单一 `live`；因此最小前端修复只需删除该调用方的 mode state/type/availableModes，并保留 change/save 事件。
- 共享工作台即使只剩默认 `live`，toolbar 仍会渲染一个带 `aria-label="Markdown 显示模式"` 的单按钮选择器；应只在模式数量大于 1 时渲染选择器。该条件不改变任何多模式消费者，也不改变 live 编辑器事件与持久化数据。
- 空态模板意外重复渲染了两次“Jira 尚未同步到标记为方案的评论。”，属于同一方案表面的明显现有缺陷，本轮一并去重。
- `syncJiraComments` 当前在评论循环内部对每个 eligible source 立即 `EnsureDraft + RequestPolish`，而 watermark 会随每条来源增加；同一 Jira 响应内 2 条方案评论会合法生成 2 个不同幂等键。应先持久化并 reconcile 完整来源集合，再针对最终集合幂等入队一次。
- 运行库已经出现 SQLite `database is locked` 重试证据；这否定了“直接把 batch size 3 改为三路并行”作为安全最小修复。本轮保持串行事实，优先减少无意义工作并提升状态可见性。
- 22:30 左右复查运行库：queued 由 33 降至 27、succeeded 由 3 增至 6、failed 由 1 增至 4，证明 worker 在推进，不是整体停摆。DG-394 job 15 已执行第 1 次，但 provider 返回 Cloudflare 502/origin_bad_gateway，当前 queued 等待重试；这是其尚无候选的直接运行时原因。

## 2026-08-12 - 项目级方案提示词缺省日志刷屏

- 用户看到的 SQL 是 HIT project-scope 活跃提示词查找；项目未配置专属提示词时，返回 0 行本应是全局回退的正常分支。
- 回归必须同时断言两件事：实际绑定全局 prompt，且 GORM 日志不包含 `record not found`；只断言 RequestPolish 成功会漏掉本次精确症状。
- 红灯为 0.31s 的单测，精确输出 `module.go:1029 record not found` 及 HIT SQL；同一测试中 job 已成功且 `PromptTemplateVersionID` 指向 global prompt，证明这是预期 miss 的日志污染，不是业务失败。
- 假设验证：H1（可选查询误用 `First`）命中；H2（全局 logger 策略）能静音但会掩盖其他错误，不采用；H3（全局 prompt 也缺失）被 job 成功否定；H4（worker 轮询）仅放大日志频率。
- 最小修复只改 project-scope 可选查找：`Order + Limit(1) + Find`，用 `RowsAffected` 区分命中/缺省。global-scope 仍用 `First`，因为该记录缺失是应报错的真实配置故障。
- 当前库仅有 `solution_polish/global/v1/active`，没有 HIT 项目级覆盖；这是正常全局回退场景，不需要为每个项目创建提示词。

## 2026-08-12 - DG-394 Jira 方案评论静默润色失效

- DG-394 的 Jira 评论 `457134` 正文是普通方案内容，没有 `[方案]`、`# 方案` 或机器 marker；但作者是专用账号 `jira公用-解决方案`。
- 当前 `jiraSolutionCommentMarker` 只检查正文，完全忽略作者，因而该快照被持久化为 `eligible=0/marker=''`；这已支持 H1，并否定“正文 marker 在归一化中丢失”的 H2。
- 同一内容快照再次观测时，`ObserveSource` 直接将已有 current 记录标记为 `Replayed` 返回，不会更新 `Eligible/Marker/Author`；Jira worker 又仅在 `eligible && !Replayed` 时入队。所以即使修正新识别规则，DG-394 这条历史快照也不会自愈，必须同时支持重判与幂等入队。
- 两次提示词表查询因假设不存在字段失败；已改为以 SQLite `.schema` 为权威，产品根因判断不受该诊断查询错误影响。
- 提示词 H4 已在当前库中排除：`solution_polish/global` 的活跃版本 v1 存在。H3 的“首次无 working revision”也被代码排除：worker 会先调用幂等 `EnsureDraft` 再 `RequestPolish`；真正的重试缺口是 `!observed.Replayed` 门禁。

## 2026-08-12 - 代码轨迹完整可滚动浏览

- 直接根因位于 `CommitTelemetryPanel.svelte`：inline 模式用 `commits.slice(0, 4)` 生成 `visibleCommits`，模板只循环该切片，并用“另有 x 条轨迹”把用户导向任务跟踪；接口本身没有四条限制。
- 第二层内容隐藏来自 inline `.timeline-body` 的两行 `-webkit-line-clamp` 与 MR 链接/元信息的单行省略；即使记录条数完整，单条长 Jira 评论仍不可完整阅读。
- 最小修复保持接口、排序、统计、刷新和检查器外框不变：直接循环 `commits`，删除跨页面提示，inline 元信息/正文/MR 链接允许自然换行与 `overflow-wrap:anywhere`。
- 滚动所有权不下沉到 `CommitTelemetryPanel`：宽屏继续由 `DemandKanban` 的 `.schedule-telemetry-inline` 唯一承载 `overflow:auto`，`<=1280px` 恢复 `overflow:visible/max-height:none`，避免嵌套滚动。
- DG-354 是当前排期列表中可复现的多轨迹样本：数据库与页面统计均为 5 条 Jira 评论。登录态页面在宽屏渲染 5/5，轨迹容器 `805px > 627px` 并可滚到最后一条；左右卡片等高。1280/760/390 也都为 5/5、自然页面滚动、无文档横向溢出。
- 完整性源码回归从 0/3 转为 3/3；与等高契约合并后 6/6。Svelte/TypeScript、生产构建、Impeccable/Finesse 和差异卫生通过。

## 2026-08-12 - 全局弹窗关闭按钮安全区

- 用户截图原始分辨率显示：任务详情使用共享 `Modal.svelte` 的 wide dialog，标题是长且会换行的 Jira 事项标题；右上角关闭按钮处在同一 flex 标题行，但桌面按钮只有 34×34px，标题与关闭动作之间没有显式安全区契约，产生视觉与命中区侵入。
- `TaskKanban.svelte:2555-2560` 将 `selectedTask.title` 直接传给共享 `Modal.svelte`，所以截图症状不是该页面正文布局造成，而是共享标题栏的长标题约束不足。
- `Modal.svelte:136-140` 统一渲染共享标题与关闭按钮；其 header 在 `241-251` 仅使用 `space-between`，title 在 `253-260` 没有 `min-width: 0`/可收缩列规则/安全 gap，close 在 `262-275` 桌面为 34×34px、仅 760px 以下增至 44×44px。
- 页面级弹窗还分散在 `DecisionEventCenter.svelte`、`DecisionDashboard.svelte`、`CommitTelemetryPanel.svelte`、`DemandKanban.svelte`、`ProjectHealthTelemetry.svelte`、`SettingsPanel.svelte`；当前关闭按钮有 34、36、38px 和无明确尺寸多种实现，视觉/触控契约不一致。
- 日期选择器、Select/MultiSelect 清除按钮、成员删除等 `×` 不是 modal close；全局优化必须按 `role=dialog`/`aria-modal` 与真实弹窗结构建清单，避免扩大语义范围。
- 历史 modal 变更已确立：共享 `Modal.svelte` 负责 workspace-scoped 遮罩，普通弹窗遮罩 `rgba(24,38,51,.28)` + `blur(16px)`，drawer 遮罩 `.18`；本次保留这些尺寸、遮罩、焦点和关闭行为，只收敛 header/close 安全区。
- 当前工作树已有大量用户修改且本任务目标文件中也有脏文件；实现必须以小补丁叠加，不能重写或回退现有变化。
- Chrome 真实登录态红灯几何：2382×1100 视口下，共享 wide modal header 为 958.9×85.6px；标题 890.2×45px，关闭按钮被 flex 收缩为 20.7×34px；标题 `right` 与按钮 `left` 几乎相同，计算为相交且安全间距约 0px。明确失败：44×44 命中区与 ≥12px 安全间距均未满足。
- Impeccable mechanical layout detector 扫描 9 个目标文件返回 `[]`，且任意 Tailwind spacing/z-index 类补扫无命中；机械扫描无法捕获动态标题长度、min-content/flex-shrink 竞争或运行时像素交叠，因此真实浏览器红灯是本任务的权威反馈环。
- 排名假设：① header 无明确 `gap`/标题列无 `min-width:0` 挤压动作；② close 无 `flex:none` 直接导致声明宽度缩小；③桌面 34px 低于触控基线；④ Settings/Demand 等页面级 `space-between + ×` 模式复制同一结构风险。事件抽屉/代码轨迹/健康诊断已有标题列收缩与动作列保留，是低风险对照组。
- 隔离主观评估清点为 18 个真实表面、5 套实现。最小边界是共享 `OverlayCloseButton`；两套事件抽屉暂不抽取整体结构，避免把本次触控/安全区修复扩大为抽屉重构。
- 响应式共识：所有断点都维持 44px，而不是只在移动端放大；标题列使用 `min-width:0` + `overflow-wrap:anywhere`，桌面 header gap 16px、<=760px gap 12px；外层 dialog/drawer 宽高策略不变。
- 实现收敛为一个共享 `OverlayCloseButton.svelte`：14 个页面级关闭入口直接使用它，另有 4 个 `Modal.svelte` 实例继承同一实现，总计覆盖 18 个真实 modal/drawer 表面；源码契约测试同时阻止旧 34/36/38px class 回流。
- 登录态 NS2-2047 绿灯几何：桌面 2382px 下 X 为 44×44px、标题安全间距约 16px、无相交；中宽同为 16px；移动有效宽约 433px 时长标题自然增至 5 行，X 仍为 44×44px、安全间距约 12px、正文保持独立纵向滚动、文档横向溢出为 0。
- 页面级运行时抽样覆盖 Demand 新需求、Health 健康诊断、Settings 权限组、DecisionEventCenter 事件记录、CommitTelemetry 代码轨迹五类实现；桌面/移动均满足 44px、标题/动作不相交和零文档横向溢出。共享 Modal 的键盘 Tab/Shift+Tab 还验证了 X 可聚焦并有可见 focus ring。
- 代码轨迹抽屉额外发现真实层级缺陷：原 `fixed inset:0` 位于 `workspace-stage` 隔离上下文，X 中心命中的是兄弟顶栏用户菜单。最终仅 drawer 态 portal 到 workspace stage，并用 ResizeObserver/resize 同步其可视四边到 fixed inset；桌面从 y≈68、移动从 y≈64 开始，X 命中自身且可关闭，移动长页面也保持视口内高度。
- 最终验证：Svelte/TypeScript 0 errors（80 条既有 warning）、7 项前端测试、生产构建、精确 diff check、Impeccable 完整/布局检测 `[]`、Finesse `p0:0` 均通过；登录态 Chrome 最终 error/warning 日志为空。


## 2026-08-12 - 需求方案资产与 Jira/Agent 润色闭环

- 方案不能只是需求表中的长字段。实现采用稳定 `SolutionAsset`、不可变 `SolutionRevision`、来源快照、润色任务、提示词版本和 Jira outbox 六类事实，并用 asset revision 做 CAS。
- Jira 评论只在明确方案标记时进入自动润色；每次内容变化形成不可变快照，删除/恢复和 A-B-A 内容回退都保留证据。机器写回 marker 被排除，避免评论自触发循环。
- Agent 输出永远是候选，不更新 working pointer；人工对照并应用后才创建新草稿。已发布正文不可变，后续编辑必须 fork。
- Markdown 规范化原文是哈希与编辑权威。大正文仅在 gzip 确实更小时压缩；API 详情可按 `Accept-Encoding` 压缩，UI 直接显示实际节省比例。
- 认证深链验收发现并修复两处异步属性竞态：方案组件须等 `solution:read` 就绪后加载，提示词组件须在用户未修改本地值时同步迟到的 `public_url`。
- 两页并发编辑证明轮询只更新远端基线和冲突提示，不覆盖本地 dirty Markdown；用户可复制本地正文或显式加载远端。
- 空任务队列改用 `Find + RowsAffected`，正常空闲轮询不再产生 GORM 红色 `record not found` 日志。


## 2026-07-18 - Daily Jira Decision Reminders And Viewport Fit

- The existing `DecisionEvent` already provides an immutable early-meeting audit ledger, but it does not carry a reminder deadline. A dedicated `DailyJiraDecision` record now stores the operational status, assignee, actor, note, decision time, and reminder time while leaving the event ledger unchanged.
- Reminder policy is deliberately low-friction: `escalate` becomes due after 4 hours; `follow_up` and `reassign` become due after 24 hours. Notification computation selects the newest record per Jira, suppresses future and resolved items, and naturally supersedes older decisions.
- Due reviews enter the existing notification SSE as `daily_jira_reminder`, use the decision record ID for a stable per-user dismissal key, and carry the last decision note so the follow-up has context.
- The GET audit projection exposes only the latest operational decision alongside the existing event history. The table shows Jira state plus decision state, while the inspector separates current decision/reminder from the immutable retrospective list.
- The three age scopes, search, result count, last refresh, and refresh action now belong to the table header. Pointer selection works from every cell in a populated row; focusable ARIA grid rows support Enter and Space activation.
- Route height is measured from the component's rendered top to `window.innerHeight - 16px`. The root clips document growth, the table and inspector own their internal scrolling, and stacked layouts divide the same remaining height instead of expanding the browser page.
- Isolated authorized-fixture browser validation used the real `DailyJiraAudit.svelte` component and was deleted afterward. Whole-row click from the assignee cell selected `WA-719`; Enter selected `WA-720`; the 3-day header tab synchronized its row and inspector. No POST request or real Jira/decision data mutation occurred.
- Geometry at 1280x720, 1024x900, 760x1000, and 480x1000 consistently measured 16px beneath the route, zero document horizontal/vertical overflow, bounded pane scrolling, and no unintended table horizontal overflow. Narrow age/search controls and refresh all measure 44px.
- Focused Daily Jira tests and the complete `internal/server` suite pass, including persistence/reminder windows, latest-decision supersession, resolved-item suppression, and per-user reminder dismissal. Svelte check has 0 errors with the same 74 unrelated warnings; production build, `git diff --check`, Impeccable layout detection, and Finesse P0 detection pass.

## 2026-07-18 - Daily Jira Visual Unification

- Authenticated comparison at 2133x903 shows the current Daily Jira route is structurally sound and data-dense, but its floating heading, flush segmented strip, 10px flat panels, and zero-elevation inspector do not match the 16/20px matte workbench language used by the adjacent decision surface.
- At the medium breakpoint the layout stacks into a hard 560px table before the inspector. At the narrow breakpoint it retains a hard 520px table and horizontally scrollable 190px cohort cells; this delays the morning decision form and creates avoidable vertical travel.
- The table's 48px row density, semantic age/status chips, sticky header, row selection, and separate table/inspector scroll owners are correct and should be preserved.
- Three-way review agrees on a targeted preservation redesign: one compact command bar, three separate cohort controls, one aligned table/inspector workbench, consistent Phase 41 material/radii, and content-aware stacked heights.
- Finesse's grain/hero substrate and design-taste's generic Atlassian package guidance are rejected for this slice because the repository already has a committed Svelte product system. The shared Phase 41 tokens and `DecisionDashboard.svelte` are the authoritative visual reference.
- Impeccable's subjective layout assessment found hierarchy/rhythm issues while its mechanical layout detector returned no findings. No arbitrary Tailwind spacing or z-index utilities are present, confirming this is a visual-structure correction rather than a lint-style violation.
- Implementation keeps behavior in place and changes only `DailyJiraAudit.svelte`: 20px structural panels, 16px status controls, a compact command bar, a wider desktop inspector, inset table shell, and a 48px row-derived stacked height.
- Authenticated wide validation at 2133x903 measured a 1108px table panel and 596px inspector with identical 691px heights and the same 324px top edge. The table and inspector remain their own bounded scroll owners.
- At the medium state, a 3-row cohort produces a 340px table panel and places the inspector 16px below it instead of reserving 560px. At effective CSS widths of 750px and 478px, project/recent-activity columns are hidden, table width equals its scroll boundary, and document horizontal overflow is zero.
- Search validation filtered to `ICA-10903` and proved row, Jira link, and inspector title all refer to the same issue; clearing the input restored all three rows without changing business data.
- Final default-width smoke state reopened `每日 Jira`, selected `7 日及以上`, rendered 133 real rows with `ZPU-2769` selected, reported zero horizontal overflow, and had no console errors.
- Final checks pass: Svelte/TypeScript 0 errors, production build, tracked and untracked diff hygiene, Impeccable complete/layout detection, and Finesse P0 detection. The 74 warnings remain in six pre-existing files outside Daily Jira.

## 2026-07-18 - Daily Jira Morning Review

### Requirements
- Add `每日 Jira` as a submenu within the decision surface.
- Audit unresolved Jira for today, 3 days ago, and 7 days ago.
- Optimize the surface for rapid morning-meeting assignment and later review.
- Preserve the independent value of the existing agenda/manual-decision workflow.

### Initial Constraints
- Mandatory UI gate requires Impeccable, design-taste-frontend, and finesse-ui agreement before any frontend edit.
- Existing repository work is heavily dirty; changes must be narrow and preserve unrelated work.
- Prior project baseline is a light, flat, table/workbench-first admin console. Avoid a new hero/cockpit or metric-card surface.
- Exact Jira age semantics, data source, assignment mutation, decision persistence, route ownership, and permissions still require code-backed discovery.

### Research Findings
- `DESIGN.md` requires grouped submenus to live in the global left rail, not inside page content. The right content area should enter business content directly under the breadcrumb/header.
- The current production baseline is the Phase 41 light admin console: compact, table-first, restrained teal accent, flat facts, and one inspector rather than a cockpit/hero.
- `App.svelte` currently models `decision` as one top-level tab guarded by `decision:read`; `DecisionDashboard.svelte` owns Agenda summary data, Jira links, manual intervention, and the decision-event ledger.
- Existing APIs include `/api/agenda/summary`, `/api/agenda/decision`, `/api/strongest-brain/intervention`, and Jira link configuration. Jira synchronization persists local task telemetry and can push assignee overrides back to Jira.
- Navigation already supports submenus for schedule, tasks, and settings through one `NavSubItem` contract. Decision currently has no children or active decision-view state, so the smallest IA extension is a decision submenu with `决策事项` and `每日 Jira`, not a new top-level tab.
- App workspace state and breadcrumbs are view-aware for schedule/tasks; the same pattern can carry a `DecisionView` and preserve `decision:read` as the parent permission.
- `TaskTelemetry` contains stable Jira/task key, title, repo/project, assignee, status, issue type, original `TaskCreatedAt`, `LastUpdate`, due date, and `DecisionLogs`. These fields are sufficient for age audit and traceability without querying Jira live from the browser.
- Agenda currently marks unresolved items older than 7 days as critical and >48 hours without update as stale, but it is risk-derived and filters through the Agenda model. A daily Jira audit needs a dedicated complete unresolved-Jira read rather than reusing only agenda rows.
- Jira assignee overrides already have a synchronization path. The existing manual intervention API and agenda decision handler must be inspected to decide whether daily assignment can reuse one safely or needs a dedicated endpoint.
- The project has no `PRODUCT.md`; Impeccable treats this as a scoped existing-surface change, so the current code and `DESIGN.md` are the product context rather than a blocker.
- The frontend is Svelte 5 with no Atlaskit dependency. The existing semantic admin components/tokens are the committed system and must be reused; design-taste's Atlassian-system suggestion is advisory, not grounds for a framework migration.

### Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Start with route/data/action tracing before UI composition | Age buckets and assignment/history controls must reflect backend truth rather than a frontend-only read model. |
| Add `DecisionView = 'agenda' | 'daily_jira'` beneath the existing decision route | Preserves the `decision:read` parent permission and follows the shell's established submenu pattern without adding a new top-level product area. |
| Define non-overlapping natural-day cohorts: age 0, age 3-6, age 7+ | Matches the requested morning audit checkpoints, prevents duplicate rows across scopes, and avoids hour-boundary drift. Age 1-2 remains outside the requested checkpoints. |
| Add dedicated read/review endpoints under `/api/decision/daily-jira` | Backend truth should own Jira-key filtering, unresolved status, core-member visibility, age boundaries, mutation authorization, Jira sync, and event persistence. |
| Persist `follow_up`, `escalate`, and `reassign` outcomes as `DecisionEvent` plus legacy `DecisionLogs` | Supports structured retrospective history while keeping the existing decision dashboard ledger compatible. |
| Use one flat table/inspector workbench with a scope strip | Faster early-meeting scan and action than three repeated boards; keeps all counts visible without nested cards. |

### Mandatory UI Review Outcome
- Impeccable: global submenu, semantic table selection, inline inspector, complete interaction states, and desktop/1024/narrow authenticated validation.
- design-taste-frontend: preservation redesign, high density, no hero/KPI theater, existing Svelte tokens over Atlaskit migration.
- finesse-ui: product register with low spectacle/high density, feedback-only motion, skeleton/empty/error/success/read-only states.
- Shared ownership: shell navigation -> App view/breadcrumb -> Daily Jira feature state -> backend audit/write contract.

### Implementation Findings
- Added dedicated GET and POST daily Jira contracts with `decision:read` for viewing and the combined `decision:read` + `demands:write` gate for mutations.
- Reassignment updates local telemetry, keeps the Markdown kanban aligned, records both `DecisionEvent` and `DecisionLogs`, and invokes the existing Jira assignee synchronization when Jira is enabled.
- Follow-up and escalation decisions intentionally do not modify `LastUpdate`; reviewing an old Jira must not make its engineering activity look fresh.
- The frontend is isolated in `DailyJiraAudit.svelte`; existing `DecisionDashboard.svelte` remains untouched and retains the agenda/manual-intervention workflow.
- The active decision view follows the existing App/shell view contract and resets workspace scrolling through the shell's route key.
- Isolated authenticated desktop validation confirms the left rail exposes `决策事项 / 每日 Jira`, the active submenu and breadcrumb both switch to `每日 Jira`, and the page defaults to the most urgent non-empty `7 日及以上` cohort.
- The populated fixture proves six unresolved Jira across mutually exclusive today / 3-6 day / 7+ cohorts, a separate 1-2 day watch count, missing-created-time warning, selected-row inspector, write-authorized decision form, Jira deep link, and structured retrospective history without touching real Jira data.
- Browser QA found and fixed a search/detail mismatch: when filtering the active cohort, the inspector now follows the first visible Jira or clears when no match remains.
- Core-member visibility is enforced on both read and review paths. Unassigned Jira remains visible so the morning meeting can dispatch it, while external assignees and out-of-scope reassignment targets stay excluded.


## Current Task: Surface-Aware Text Contrast

- New user feedback establishes a hard surface rule: black/dark ink must not be inherited onto a dark background. Dark product surfaces use cool light primary text plus muted blue-gray secondary text; light surfaces keep the existing dark ink scale.
- The active full-height demand drawer is intentionally a light surface and should not be darkened. The risky path is the legacy `.modal-content` base, which declares `background: #0b1220` without declaring a compatible foreground before the later Phase 41 light override.
- Scope the correction to the modal contrast contract and validate rendered affected states. A repository-wide dark-theme rewrite would touch unrelated legacy panels and is not justified by this feedback.
- Authenticated computed-style inspection identified the visible collision precisely: `AI 解构需求` and `批准审核契约` both render on `rgb(1, 139, 141)` while inheriting near-black `rgb(3, 18, 25)`. The drawer itself is correctly light (`rgb(255,255,255)` with dark ink), and the dark navigation rail already uses light `rgb(212,229,242)` text.
- Use a separate filled-accent contract rather than flipping the general accent token: dark teal filled controls get a darker AA-safe fill, cool off-white ink, and a darker hover fill; outlines, links, progress, and soft accent surfaces keep the existing teal.
- Implementation adds `--wa-accent-fill`, `--wa-accent-fill-hover`, and `--wa-accent-fill-ink`, then applies them to the admin primary action, selected date, shared Button, demand-detail AI action, and delivery primary action without changing light-surface ink.
- Final authenticated computed styles for both reported controls are `rgb(0,111,118)` background and `rgb(246,251,255)` text at 5.70:1 contrast. The light drawer remains 11.99:1 and the dark rail remains 15.37:1.
- Wide and 760×800 screenshots confirm the new light text reads cleanly on the dark teal buttons, full-height drawer geometry remains intact, and horizontal overflow remains zero. The delivery approval button was visually inspected in its scrolled state; console errors are empty.
- Final static gates pass: Svelte/TypeScript check has zero errors, production build and `git diff --check` pass, and finesse detector reports P0=0 after adding the shared spinner's reduced-motion fallback.

## Current Task: Viewport-Height Demand Drawer

- The supplied regression screenshot confirms the previous drawer conversion is still scoped to the flow-board workspace: its top begins below the application header/breadcrumb and its bottom ends at the board boundary, so it reads as an enlarged board panel rather than a true drawer.
- The correction is ownership, not more styling. The detail and linked companion overlays must use viewport-fixed positioning and a global modal layer; the drawer itself must be flush right and span `100dvh` with square corners, while `.detail-body` remains the only long-content scroll owner.
- The complete application shell, including top bar and sidebar, must sit under the same restrained backdrop so visual hierarchy and hit blocking are unambiguous.
- Three-way review agrees on preserving `DemandKanban.svelte` lifecycle ownership, the existing light/teal product language, current dialog semantics, and wide companion behavior. A portal is only warranted if authenticated geometry shows a transformed or contained ancestor constrains `position: fixed`.
- Browser acceptance is geometry-based: drawer and overlay must resolve to the viewport edges at wide and narrow breakpoints, with no horizontal overflow, no shell click-through, and no regression in focus, Escape, linked close, or companion separation.
- First authenticated geometry pass proved that `position: fixed` alone is insufficient: `.workspace-stage` establishes an isolated stacking context below the `z-index: 50` top bar. The drawer rectangle reached the viewport, but hit testing and the screenshot showed the shell top bar still painting above the drawer and hiding its header.
- The planned portal fallback is therefore required. Move only the detail and detail-companion overlay roots to `.functional-console`, where their existing fixed positioning and modal z-index can outrank both top bar and rail without weakening the shell's isolation contract for every other workspace.
- Final authenticated wide geometry is exact: the overlay is `2133.33 × 1037.78`, the drawer is `820 × 1037.78` at `top=0/right=viewport`, the top-left hit belongs to the backdrop, the top-right hit belongs to the drawer header, and document horizontal overflow is `0`.
- The wide AI companion remains `620px` wide with a `16px` drawer gap, no overlap, full-viewport companion overlay, and the same `.functional-console` host.
- At the measured mobile breakpoint (`760 × 800` CSS viewport), both overlay and drawer are exactly `760 × 800` with all four edges flush, both top-corner hit tests owned by the drawer, square corners, one `727px` body scroll owner, and no horizontal overflow.
- Escape closes the portaled drawer, restores focus to the DG-319 demand card, and restores body overflow. Wide backdrop click also closes the drawer and restores body overflow. Browser console errors are empty.
- Final static gates pass: Svelte/TypeScript check has zero errors, production build and `git diff --check` pass, finesse detector reports P0=0, and full-file detector warnings remain legacy rules outside the new portal/fixed/full-height selectors.

## Current Task: Flow Detail Drawer And Unified Task Filters

- The first supplied screenshot shows a centered, near-full-height demand-detail modal floating over a large dimmed board. Its rounded modal shell, internal document workbench, and dashboard shell use competing container languages, which explains the reported lack of visual unity.
- A right-side workspace drawer is the strongest fit for this operational state: demand selection remains spatially connected to the board, the shell hierarchy stays visible, and a fixed drawer header can own close/actions while one body region owns long specification scrolling.
- The second screenshot shows a dense execution workbench with a table on the left and inspector on the right. Their top edges align, but the inspector ends earlier than the visible table and the filter controls mix search input, segmented risk chips, shared dropdowns, and a standalone refresh button in one uneven row.
- The task-filter target is one consistent control vocabulary: every categorical filter uses shared Select, search remains the only text input, refresh remains an action, and all controls share a 40px desktop/44px narrow height and a common baseline.
- The table and inspector should share one desktop workbench height token and independent body overflow only where content requires it; at narrow widths they stack in source order with the inspector following the selected table row context and no horizontal overflow.
- The working tree is heavily modified, including the likely target files. This task must use narrow patches and preserve all unrelated changes.
- Design read: internal operational product UI, calm light console, teal accent, high density, feedback-only motion, targeted preservation redesign.
- Initial responsive contract from the screenshots: right drawer at wide/desktop widths, full-width off-canvas sheet at narrow widths; task toolbar may wrap deliberately at intermediate widths, while the table/inspector stack below the project breakpoint and every touch control reaches 44px.
- `DemandKanban.svelte` owns demand-detail selection, close behavior, body scroll locking, and the linked AI companion host state. Keeping the drawer shell in this owner avoids splitting lifecycle state or risking an orphaned companion.
- The current demand detail is still rendered through the generic `.modal-backdrop/.modal-content` contract, while its content already has a distinct `detail-body`, overview, facts, subtasks, and delivery-control hierarchy. The smallest structural correction is to change only the detail host into an off-canvas dialog and leave create/schedule/confirm modals unchanged.
- `CommitTelemetryPanel.svelte` provides a repository-proven off-canvas interaction reference, but it is telemetry-specific rather than a shared primitive. Reuse its right-edge/backdrop/body-overflow behavior and token language without coupling demand detail to telemetry markup.
- `TaskKanban.svelte` already imports the shared `Select.svelte`; the execution toolbar currently mixes that primitive with a custom segmented risk strip. The user request maps directly to replacing the categorical risk strip with a shared Select while preserving the existing text search, project/owner Selects, and refresh action.
- The active Phase 41 contract defines a restrained light admin shell, one shared component vocabulary, `--wa-control-h`/`--wa-touch-h`, and table-first plus right-inspector composition. No new dependency or design system is appropriate for this targeted pass.
- Three-way review agreement: demand detail becomes a right-edge off-canvas dialog with fixed header and one scrollable body; execution risk becomes a non-searchable shared Select; all categorical controls share one geometry; execution table and inspector share a desktop height/top baseline and release it below the stacking breakpoint.
- Review ownership: `DemandKanban.svelte` owns the drawer and AI-companion lifecycle, `TaskKanban.svelte` owns filtering/selection/panel geometry, and `Select.svelte` remains the only categorical-control behavior owner.
- Review disagreement resolution: do not introduce or retrofit a global Drawer primitive in this dirty-tree pass. Follow the repository telemetry drawer behavior locally because demand detail has unique linked-companion state; revisit extraction only when a second general-purpose consumer exists.
- Validation agreement: demand drawer open/close/backdrop/Escape/focus, long-body scroll and linked AI companion; risk/project/owner selection and empty/populated task states; table-inspector top/bottom geometry; wide desktop, 1440/1180/1024/760/480; 44px narrow controls, zero horizontal overflow, open-menu hit testing, reduced motion, detector, Svelte check, build, and diff hygiene.
- The active detail host is already absolutely scoped to the workspace instead of the browser viewport. Converting its flex alignment from centered to right-edge and its panel from bounded modal height to full workspace height can produce the drawer without changing shell or route ownership.
- The linked AI companion has a desktop side-by-side transform based on a centered 1080px host. The drawer implementation must explicitly cancel the host translation and place the companion to the drawer's left on wide screens; the existing centered overlay fallback below 1500px remains the safer responsive behavior.
- `TaskKanban.svelte` already defines `--task-aligned-panel-height` and applies it to status and personnel workbenches. Execution can reuse that contract rather than introducing another height token, while its table panel becomes a two-row grid and its inspector becomes the matching overflow owner.
- Shared Select supports `searchable={false}`, adaptive up/down placement, keyboard Escape, outside dismissal, compact mode, and the existing 36px control token. It can replace the six-option risk strip without any new control code.
- Implementation converts the detail host to a right-edge 820px workspace drawer with a fixed header, one hidden-rail body scroll owner, focus-on-open, Tab containment, Escape behavior, return focus, reduced-motion fallback, full-width narrow layout, and a wide-screen companion placed to its left.
- Implementation replaces the active execution risk strip with a non-searchable shared Select and reuses the existing aligned panel-height token so the table panel and inspector share top and bottom edges on desktop while releasing the contract below 1180px.
- First static pass succeeds: `git diff --check` is clean and `pnpm -C web check` reports 0 errors. The one newly unused risk-button selector warning was removed before the final check rerun.
- Authenticated Chrome validation confirmed the wide drawer is 820px, owns one `.detail-body` scroll region (`781px` client / `3185px` content in the measured state), traps Tab focus, restores focus on Escape, and has zero document-level horizontal overflow.
- At the measured 760px viewport the detail becomes a workspace-width sheet with one scroll owner and zero horizontal overflow; the task toolbar stacks its search and three dropdown controls without page overflow.
- The first wide companion pass exposed a real flex-basis bug: percentage width was resolved against the already padded remainder and collapsed the AI panel to about 1px. The corrected companion is 620px wide, sits 16px left of the drawer, has zero overlap, and returns focus to `AI 解构需求` after Escape.
- Authenticated task geometry is exact at wide desktop: table panel and inspector are both `617.78px` high with `0px` top and bottom deltas. At 1180px they stack with a 16px gap and equal 480px heights.
- Risk dropdown behavior is verified through the shared Select contract: it exposes one expanded listbox with `需关注 / 全部 / 高风险 / 中风险 / 正常 / 已闭环`, applies the selected value, closes the list, and preserves zero horizontal overflow.
- Final checks pass: Svelte/TypeScript check has 0 errors and 74 existing warnings, production build and `git diff --check` pass, finesse detection has no P0 findings, full-file Impeccable/finesse warnings are documented legacy selectors outside the new drawer/filter/alignment rules, and the authenticated browser console error list is empty.


## Current Task: Event Timeline, Flow Detail, And Execution Filters

- User evidence shows three regressions on the current light admin-console baseline: an inner scrollbar in event mediation records, an unresolved flow-board demand detail panel, and a clipped duplicate owner filter in execution tracking.
- Prior project memory identifies `DecisionDashboard.svelte`, `DemandKanban.svelte`, and the shared `TaskKanban.svelte` task-tracking slice as the first inspection targets.
- Prior validated task-tracking order is metrics -> stage strip -> shared filters -> page-specific table/workbench. This task must keep that flow while merging project and owner filtering into the execution-tracking bar.
- Existing working tree is heavily modified across backend and frontend files. All edits and validation must be target-scoped; no cleanup, revert, or broad formatting is authorized.
- Design read: internal operational product UI, calm light console, teal accent, high density, no decorative motion, targeted evolution only.
- Screenshot 1 is a 2048x925 authenticated flow-board detail state. The demand detail surface occupies almost the full usable viewport height and visibly owns an inner vertical scrollbar at its far-right edge; the AI specification body continues below the fold. This is evidence that the previous detail-panel pass did not establish a viewport-fit/no-competing-scroll contract for this host state.
- Screenshot 2 shows two filter layers in execution tracking: a page-level row containing `项目 / 全部项目` and `负责人 / 全部负责人`, then an execution toolbar containing search, risk/status chips, and another `全部负责人` select. The inner select menu is visually under the table/header layer, so row content remains visible through/over the menu. The target hierarchy is one execution toolbar row containing project, one owner filter, search, state controls, and refresh/count affordances.
- The screenshot makes the owner duplication a structural ownership bug, not a label-only problem: the page-level shared filters and execution-local filters both own the same semantic dimension. Consolidation must leave one source of truth and one interactive control for owner filtering in execution tracking.
- `DecisionDashboard.svelte` already gives `.inspector-content` vertical overflow ownership, while `.ledger-list` adds a second `max-height: 320px; overflow: auto` boundary. The local ledger boundary is the nested scrollbar root cause; mediation records should expand naturally inside the inspector.
- `TaskKanban.svelte` sends `selectedAssignee` as the API `assignee` query and then applies `executionAssigneeFilter` to the same `item.assignee` field. Both option lists come from `coreMembers`, so this is duplicated data and behavior rather than two valid business dimensions.
- Execution tracking will hide the page-level filter strip and move `selectedProject` plus `selectedAssignee` into the active execution toolbar. Other task views retain the existing shared filter surface. The shared `Select.svelte` owns menu keyboard/dismissal behavior, while the toolbar and table receive explicit adjacent stacking levels so the menu is never painted under task rows.
- The demand-detail backdrop is fixed inside the isolated workspace stage while the shell topbar owns a higher sibling stacking layer. The detail height still uses the full dynamic viewport, so its modal can consume space hidden behind the topbar and expose a competing-looking rail. The scoped repair is to calculate desktop detail space below the shell topbar, retain exactly one body scroll owner for long specifications, and hide only that owner's visual rail.
- Responsive contract: the supplied 2048px state keeps the execution controls in one line; intermediate widths may reflow to two rows before the inspector/table become unusably narrow; narrow screens collapse to one column and preserve 44px controls. Long specifications remain wheel, touch, and keyboard scrollable even when the rail is visually hidden.
- Validation scope agreed by all three reviews: authenticated current/all/empty event states; populated long demand detail and its AI companion/close linkage; execution project/owner combinations and open-menu hit testing; wide desktop plus 1440/1280/1024/760 geometry; horizontal overflow, focus, Escape/outside dismissal, console errors, targeted Impeccable detection, Svelte checks, build, and diff hygiene.
- Implementation removes the ledger's local max-height/overflow boundary, keeps the inspector as the only visible scroll owner, and preserves natural current/all/empty expansion.
- Demand detail and its AI companion now share the board's positioned backdrop contract. At 2048x925 the host and companion align within 1px vertically with a 16px gap and zero horizontal overflow; closing the host removes both surfaces. The long body remains scrollable while `scrollbar-width: none` removes the misleading rail.
- Execution tracking now binds its project and owner Select controls directly to `selectedProject` and `selectedAssignee`; the duplicate execution-assignee state and page-level execution filter row are removed. Wide desktop renders search, risk states, project, owner, and refresh in one visual row; responsive widths reflow without horizontal overflow and narrow controls measure 44px.
- Browser hit testing proves the open owner list overlaps the table but `elementFromPoint` resolves to the dropdown option, so table rows cannot paint through or intercept the menu. Combined project plus owner filtering returned the expected single fixture task.
- Final static validation passes with zero Svelte errors (78 existing warnings), a successful production build, clean diff whitespace, and only documented legacy Impeccable findings outside the changed selectors.


## Phase 68: Corpus Approval Information Architecture Refactor

- The screenshot does not show identical candidate records: titles differ, while the same imported document, scope, timestamp, status, and long provenance content repeat around them. The first fix is presentation grouping, not destructive deduplication.
- `CorpusCandidateReview.svelte` currently renders every candidate as a full queue card with status, two-line summary, type, scope, and time. Candidates already carry `context_document_id` and `source_document`, so the UI can group them by immutable document/version without a backend change.
- Selecting a normal candidate eagerly loads the impact endpoint and places `source_markdown` beside the candidate in two equal 500px panes. That comparison is required only after a candidate enters `impact_review`; during ordinary review it duplicates source and proposal content and overwhelms the metadata/action hierarchy.
- The agreed ordinary-review composition is one editable proposal workbench with a compact source-evidence disclosure. The agreed impact-review composition remains current active context versus the locked proposal, with immutable source evidence available separately.
- Queue grouping must preserve all IDs and candidate actions. Shared source title, original filename, version, creation time, and candidate count appear once in a source-group header; row content is reduced to status, title, and the smallest discriminating metadata.
- The existing shared Select and MarkdownWorkbench components already own control behavior and Markdown rendering. This phase should change only orchestration and presentation inside `CorpusCandidateReview.svelte`.
- Implementation groups all eight authenticated fixture candidates into two document batches and renders one filename/version label per batch. Browser counts prove all eight candidate IDs remain reachable; grouping does not filter or deduplicate records.
- Ordinary review now mounts one live Markdown workbench. Structured metadata and immutable source evidence are closed disclosures by default; opening metadata exposes three shared Select controls and zero native `<select>` elements, while opening evidence lazily mounts the read-only source Markdown.
- Candidate titles are content-aware: when the first Markdown heading already equals the candidate title, the detail header suppresses its duplicate; candidates without a matching first heading still retain the header title.
- Impact review still mounts the required two-pane current-context versus proposed-content comparison. The browser-validated high-sensitivity transition changes the queue counters from `7/1` to `6/2`, locks metadata, renders the impact reason, and exposes the separate final publish action.
- The proposed impact pane now renders `draft.content`, matching the value written to `ContextFact.Content`; it no longer consumes the server's display-oriented `after_markdown` wrapper that can repeat title and summary.
- Queue height is viewport-bounded and locally scrollable. Browser geometry passed at desktop, 1024px, 760px, and 480px. At 480px the document has zero horizontal overflow, queue height is 318px, the layout is one column, and actions/disclosures measure 44px.
- Populated, empty, and error states were verified independently. The error state has exactly one alert and zero empty-state blocks; the final clean authenticated run has no console warnings or errors.
- Final validation passes `svelte-check` with 0 errors and only 78 existing warnings in unrelated files, production build, targeted Impeccable detection (`[]`), and diff hygiene.

## Phase 67: Shared Import Select And Live Markdown Composition

- The import scope field is the only control in this form still rendered as a native `<select>`, despite the repository already providing a shared Select with keyboard navigation, outside/Escape dismissal, adaptive overlay placement, errors, disabled state, and compact layout support.
- The smallest ownership fix is to pass the four scope options and current value into shared Select, then keep only the existing `global` side effect in the parent. Search is unnecessary for four fixed options.
- `MarkdownWorkbench.svelte` already owns CodeMirror, Markdown/GFM parsing, sanitized preview, value synchronization, and the edit/split/preview modes. A separate WYSIWYG package or `contenteditable` surface would duplicate ownership and risk lossy HTML-to-Markdown conversion.
- CodeMirror's official `ViewPlugin` and `Decoration` APIs can provide Typora-like WYSIWYM behavior without changing the stored value: inactive lines hide Markdown delimiters and receive semantic typography, while the active line keeps its syntax visible for editing.
- The live mode should become the corpus paste default, but existing source, split, preview/reading, readonly, save, commit, and `change` contracts must remain compatible for other consumers.
- The UI review converged on a targeted preservation pass: no surrounding layout, color, radius, or typography redesign; reuse shared Select and add live composition inside the existing editor only.
- The first Impeccable scan classified a 3px left border on formatted blockquotes as the side-tab anti-pattern. Replace it with a semantic quote widget and restrained row tint rather than suppressing the finding.
- Implementation keeps all Markdown in the existing CodeMirror state and adds only a `live` display mode. Current-line syntax remains editable; inactive-line heading, strong/emphasis, strike, code, link, quote, bullet, and task markers are represented with decorations/widgets that never rewrite the document.
- The import scope control now uses the shared non-searchable Select and retains the existing global-scope cleanup. The import form owns its 44px narrow-screen control token rather than changing every Select consumer globally.
- Browser proof passed shared Select click, Enter, Escape, outside dismissal, option selection, keyboard return to global, and global range-ID disabling. The page contains no native import `<select>`.
- Browser proof also passed immediate Markdown rendering, active-line syntax reveal, raw source preservation, split/read fallbacks, and 1440/1024/760/480 geometry. Every viewport had `scrollWidth === innerWidth`; at 760/480 the Select and all four Markdown mode buttons measured 44px.
- Final browser console and page error lists are empty. Svelte check has 0 errors and only the repository's 78 existing warnings; production build, TypeScript, final Impeccable scan (`[]`), and diff hygiene pass.

## Phase 66: Opaque File Handoff Correction

- The user explicitly corrected the Phase 65 boundary: file uploads belong to the LLM input contract, not to frontend or backend text parsing. This supersedes the earlier `.md/.txt` normalization decision.
- The current frontend violates that boundary by calling `await file.text()`, copying the result into `MarkdownWorkbench`, and posting JSON `content`.
- The current backend violates that boundary by accepting the file-derived `content`, persisting it as the source document, and concatenating `document.Content` into the LLM user prompt.
- The corrected invariant is mechanical and testable: selected `File` stays a `File`; the browser sends multipart; the server reads only opaque bytes plus metadata; the provider adapter creates a file block; uploaded bytes never occur in the text prompt; persistence receives only the LLM extraction document and candidates.
- The UI remains a restrained product workbench. File mode needs metadata/transport status rather than an editor preview; paste mode continues to reuse `MarkdownWorkbench` as the explicit human-authored text path.
- OpenAI developer-docs MCP tools are not available in this session, and the local `codex mcp list` command fails before execution because its packaged binary path is missing. Use official OpenAI-domain web documentation as the narrow fallback and log the exact request schema before implementation.
- Official OpenAI file-input documentation confirms that Responses accepts an `input_file` content part with inline Base64 `file_data` plus `filename`; the app adopts that transport while keeping a conservative 10 MiB local limit below the documented API limit.
- The native Messages adapter deliberately rejects opaque file imports with an actionable Responses-protocol error. It never falls back to local text extraction.
- Completed imports persist only LLM-returned `document_markdown`, summary, and candidates. Failed imports persist metadata/hash/status only; duplicate completed files reuse the existing extraction without another provider call.
- Browser and backend proof agree on the boundary: multipart retains exact selected bytes, the text prompt never contains the sentinel body, file mode never mounts the Markdown editor, and paste mode remains a separate explicit text input.


## Phase 65: Governed Corpus Ingestion And Unified Review Center

- The requested change extends the Phase 64 corpus tabs from presentation cleanup into a governed lifecycle spanning source documents, LLM extraction, candidate review, impact preview, and publication.
- Existing `CorpusCandidate` and `ContextFact` types already provide the promotion seam; the new work must preserve that seam while adding immutable source provenance and a separate publish gate for sensitive/global candidates.
- The repository already has a shared CodeMirror/GFM `MarkdownWorkbench.svelte`; it is the required owner for import/paste content and any Markdown comparison/editing, avoiding a parallel textarea editor.
- Design Read: enterprise configuration workbench for administrators and system-design owners, restrained light-console language, register=product, SOUL=4, SPECTACLE=2, DENSITY=8. Motion is feedback-only.
- The working tree is broadly dirty from prior phases. Phase 65 must use narrow patches and preserve every unrelated modification.
- `ContextDocument` already exists and is auto-migrated, but it currently stores only normalized title/type/scope/source/status/version/content metadata. It lacks original filename/MIME, ingestion status, uploader, source hash lineage, parse failure, and explicit raw-versus-normalized provenance.
- `ContextFact.ContextDocumentID` already provides the document-to-fact link, while `CorpusCandidate` has no source-document link. The smallest compatible model extension is to attach candidates to `ContextDocument` and add review/publication metadata rather than invent a parallel candidate table.
- The production context-pack builder reads only `ContextFact` rows whose status is `active`. This is the authoritative publication boundary and already excludes every unpromoted candidate.
- Current candidate acceptance is one atomic transaction that immediately creates an `active` fact. Sensitive/global governance therefore needs a new intermediate candidate status and a separate publish endpoint; ordinary acceptance can retain the current immediate-promotion behavior.
- Existing permissions are sufficient for the first implementation: `ai_context:read/write/preview` owns documents/import/impact preview, while `corpus_candidate:read/review` owns unified review and publication. No new RBAC code is required unless tests expose an ownership gap.
- The current corpus UI is already split into `资料库 / 候选审核 / 上下文预览`, but `资料库` is fact-first and mounts a long manual form next to the list. Phase 65 should make source documents and import the primary library task, then move direct fact creation behind an explicit advanced disclosure.
- `CorpusCandidateReview.svelte` currently renders compact read-only articles and only sends accepted/rejected. The unified center needs selected-row ownership, editable proposed Markdown/metadata, source provenance, queue state filters, and an impact-review state with a separate publish action.
- `MarkdownWorkbench.svelte` already exposes edit/split/preview, read-only, change/save/commit, custom height, and responsive stacking. It can be reused for import content, manual fact content, candidate editing, and left/right read-only comparison without changing the editor engine.
- The current AI context page already follows the Phase 64 progressive-disclosure tab contract. Preserve that outer task hierarchy rather than adding another nested page title or card grid.
- Three-way UI review converged on a source-first library and selected-candidate workbench. Glass remains limited to the existing outer Settings structure; source rows, metadata, Markdown editors, comparisons, and action bars stay flat with hairlines.
- The initial supported file boundary is text-native `.md`, `.markdown`, and `.txt`, normalized to Markdown while preserving original filename, MIME, hash, uploader, version, and source content. Unsupported binary documents must fail explicitly instead of silently producing unreliable text.
- The impact comparison should compare the currently active matching fact set against the proposed candidate, while showing the immutable source document as provenance. Only `impact_review` candidates expose the final publish action.


## Demand Detail Action Hierarchy And Draft Footer

- The visible purple mismatch came from selector specificity: the legacy `.detail-link-row button` rule outranked the newer single-class AI button rule. Scoped two-class action rules now let the shared admin tokens own the actual rendered colors.
- The top actions now express product hierarchy instead of category decoration: `AI 解构需求` is the teal primary accelerator, while `调整排期` is a neutral secondary utility with the same height, radius, focus language, and restrained hover treatment.
- `撤销草案` is no longer stranded beneath the specification document. It shares the terminal review action row with contract controls, remains visually isolated on the left as a destructive action, and preserves the existing explicit confirmation alert.
- Authenticated DG-319 validation opened and cancelled withdrawal without mutating business data. Desktop and narrow states showed no horizontal overflow, focus/hover states matched tokens, and the console remained error-free.
- Svelte check, production build, diff hygiene, and Phase 61-targeted Impeccable detection pass. Whole-file Impeccable findings remain pre-existing outside the changed ranges.

## Reviewer Multi-select Consecutive Selection Correction

- Phase 59's close-after-toggle behavior was the direct cause of the reported regression. A multi-select needs one continuous selection session, so selecting or deselecting an option now updates the bound values without ending that session.
- The shared capture-phase outside-pointer and Escape lifecycle remains correct. Clicking elsewhere in the modal closes the list even though the modal stops bubbling clicks, while `完成选择` and the chevron remain explicit dismissal paths.
- Removing a selected chip exposed a separate focus boundary: pointerdown focused the remove button and the following value update deleted that focused node, causing `focusout` to close the list. Preventing focus transfer on remove-button pointerdown preserves the open session without suppressing its click action.
- Authenticated DG-319 validation proves consecutive selection, option deselection, chip removal, outside dismissal, Escape, explicit completion, and chevron close. Temporary values were restored and no save/approval action was triggered.
- Desktop and narrow screenshots show the bounded in-flow options remain inside the modal with no horizontal overflow; Svelte check, build, targeted Impeccable detection, and diff hygiene pass.

## Deterministic Multi-select Completion

- The screenshot's apparent outside-click failure was partly a hit-testing problem: the absolute full-width option list covered the following form rows, so clicks intended for required roles or owners were still inside the multi-select.
- Keeping the list open for consecutive selection needs a visible completion affordance. A footer outside the ARIA listbox now owns selected count and `完成选择`.
- Escape previously called `closeDropdown()` and then focused the search input; its `focus` handler immediately reopened the list. Closed-state focus now returns to the chevron.
- Moving the list into normal flow initially exposed a containing-block bug: the absolute chevron used the expanding wrapper for `top: 50%`, moved over `张路路`, and caused apparent click-through. `position: relative` on the fixed trigger keeps the arrow stable.
- Browser geometry proves the open list ends 37px before the next form group, has no overlap or horizontal overflow, and keeps the toggle inside the trigger at both default and 480px widths.
- The authenticated app route was unavailable because the automation session had no login state. The actual shared Svelte component was therefore mounted in a temporary local harness for interaction testing; the harness was deleted after validation.

## Review Select Dismissal And Core-member Boundary

- The close failure was event-boundary specific: the shared controls listened for bubbling `window.click`, while the demand-detail modal intentionally stopped click propagation. Capture-phase `pointerdown` is the stable outside-dismiss contract for both controls.
- Searchable triggers previously exposed only an open path. Making the chevron a real toggle button provides an explicit close path without changing single- versus multi-selection behavior.
- Owner leakage came from merging the full `/api/users` page directory into contract choices. The existing `coreMemberVisibility` service and `/api/demands/options` assignee directory are the authoritative backend/frontend boundary.
- Backend filtering alone is insufficient during a rolling restart because the frontend may still talk to an old server. The UI therefore intersects the combined directory with the core-member list when available and trusts only contract-scoped participants before it arrives.
- A contract-ID-only normalization guard can retain a previously valid but later-filtered owner when directory data arrives asynchronously. Including the normalized option signature in the guard ensures stale non-core values are removed before save.
- Focused lifecycle testing proves a seeded non-core `External Operator` is absent while configured `Reviewer` remains available. Svelte check, build, Impeccable complete/layout detection, and diff hygiene pass.
- Live authenticated browser interaction remains unverified in this run because the browser was locked to a generated connection-error page after the local listener had been down; browser policy prohibited further navigation or DOM actions from that state.

## Health Diagnosis Intervention Modal — Audit Start

- User reports that the modal's layout and palette have excessive contrast and diverge from the system aesthetic.
- Target direction: calm light admin-console surfaces, cool neutral hierarchy, one restrained risk accent, fewer nested card boundaries, and clearer reading order.
- Screenshot shows the modal is owned by `ProjectHealthTelemetry.svelte`. The current surface combines a pale pink command block, saturated red diagnosis copy/score, blue/green/orange/pink metric bars, lavender status chips, cyan scrollbar, and cool-gray cards; the number of competing hues is the main aesthetic mismatch.
- The header is visually under-structured, while the top intervention block dominates due to its tinted area and oversized white `49.64`. Below it, multiple bordered cards inside section cards create a nested-card effect and make the two columns feel fragmented.
- The modal width is reasonable, but body content is dense and vertically scrollable. The left score/formula card consumes substantial height while the right column continues into intervention content, so section rhythm and cross-column alignment need calibration.
- Markup contains many inline layout styles inside the modal (`phdi-score-row`, formula width, metric row headers), which makes the presentation harder to calibrate responsively and should be replaced with scoped classes.
- The file contains both legacy dark modal/table styles and later light admin-console overrides. The final modal fix must land after the current light overrides or remove conflicting rules, not add another partially competing palette earlier in the file.
- Existing system tokens already provide the correct palette: white/inset surfaces, cool gray text/borders, teal accent, and muted success/warning/danger soft colors.

### Open Questions

- Which component owns the visible modal and whether later CSS overrides are fighting its base styles. **Owner found:** `ProjectHealthTelemetry.svelte`; style conflict still to inspect.
- Whether the two-column body is structurally sound at the screenshot viewport or needs source-order/layout changes. **Resolved:** keep the two-column source order at desktop, tighten the gap, and collapse to one column below 860px.
- Which content blocks must remain visually emphasized for intervention safety. **Resolved:** urgency, overall score state, diagnosis evidence, dimension status, and recommended intervention route; all other surfaces remain neutral.

### Chosen Visual Contract

- Scope all corrections under `.health-diagnosis-modal` so table/workbench styles remain untouched.
- Use a structural header with small context label, a compact intervention summary, and a body-owned scrollbar; remove modal-wide hover contrast.
- Replace the top tinted block's nested white cards with three divided summary cells on one quiet surface.
- Make diagnosis copy neutral with a restrained tone border, remove score glow/text-shadow, and use soft semantic fills only for state chips and result nodes.
- Normalize metric rows to one quiet surface and semantic low-saturation bars; recommendations use neutral rows with a narrow tone rail instead of colored cards.

### Audit Errors

- A CSS-token search omitted the `--` argument separator, so `rg` parsed the pattern as a flag. The token file was still read directly; subsequent searches will use the correct separator.
- The browser's constrained evaluation DOM does not implement `document.createElement`; isolated visual verification must replace the local preview body's markup directly or use existing locators.
- Direct body markup replacement is also blocked because the evaluation DOM exposes `innerHTML` as getter-only. A temporary static page served by the local preview is the appropriate non-auth visual harness.

## Streamed LLM Attachments And Demand File Archive — Initial Correction

- The browser extraction approach is the wrong boundary for this product: it increases the web bundle, cannot reliably preserve document fidelity, and prevents durable original-file provenance.
- Required target flow: `multipart/form-data` upload to `/api/deconstruct` → bounded backend spool/stream → gzip-compressed original storage + database metadata/association → provider-specific file delivery to the LLM → deconstruction result/archive linkage.
- Existing JSON-only deconstruction calls must remain valid; multipart is an additive request form.
- PDF/DOC/DOCX browser dependencies added in the previous pass are candidates for removal once the backend attachment path is in place.
- `/api/deconstruct` currently decodes JSON directly, builds a text-only `messages: [{role, content}]` payload, and calls an OpenAI-compatible endpoint itself. The configured endpoint mode is either `completions` or `responses`.
- `DeconstructArchive` is not created by preview deconstruction; it is created later during task import/archive. Attachment metadata therefore needs a nullable staged association first (demand/context/session key), then archive linkage when import persists the archive.
- The database uses GORM `AutoMigrate`, so a dedicated attachment model can be added without a handwritten migration.
- Official Responses file input supports `input_file` with Base64 `file_data`, Files API `file_id`, or external URL. PDF supplies extracted text plus page images; DOC/DOCX and other rich documents supply extracted text. The accepted rich-document list includes both `.doc` and `.docx`.
- For this local archive flow, inline Base64 `file_data` avoids creating unmanaged remote Files API objects. It is not literal zero-copy streaming at the provider hop because the Responses JSON contract requires Base64, but the browser-to-server hop remains multipart streaming and the original bytes remain preserved locally.
- Native `input_file` is a Responses API contract. Text-only requests preserve the configured endpoint, while attachment requests automatically derive the same provider's `/v1/files` and `/v1/responses` paths. Providers missing those APIs return their actual compatibility error; the server never falls back to local parsing.
- Runtime persistence is currently rooted at the process working directory (`well-ambient.db` in `cmd/server/main.go`), with no storage-dir config. A sibling default such as `data/demand-attachments/` is consistent, but a configurable server attachment directory is safer for deployment and tests.
- Frontend import already receives `archive_id` only after `/api/tasks/import`. The deconstruction response exposes `context_pack_id`, so attachment rows can be created at upload time with `DemandID` + `ContextPackID`, then updated with `DeconstructArchiveID` inside the import transaction.
- The frontend currently sends only `{text}` to `/api/deconstruct` and later sends `demand_id`, `task_group_id`, and `context_pack_id` to `/api/tasks/import`. Multipart should include `text`, `demand_id`, `task_group_id`, and repeated `files` so association data is available immediately.

### Open Architecture Questions

- Does the current OpenAI-compatible provider contract support native file parts, Files API references, or only text messages? **Resolved:** only the configured `responses` path should advertise native attachments; completions remains text-only.
- Which persisted ID is available before and after deconstruction to associate independent uploads safely? **Resolved:** stage by attachment ID + uploader + `context_pack_id`; finalize with `deconstruct_archive_id` during task import.
- What application data directory/config convention already exists for durable local artifacts? **Resolved:** add `server.attachment_dir`, defaulting to `data/demand-attachments`, because current runtime has no artifact directory contract.

### Chosen Backend Contract

- Attachment row: original name, MIME, extension, gzip-relative path, SHA-256, original/compressed sizes, uploader, demand ID, task-group ID, context-pack ID, archive ID, status, timestamps.
- Request: JSON remains supported when no file is present; multipart fields are `text`, `demand_id`, `task_group_id`, and repeated `files`.
- Provider: stream each gzip-decoded original to `/v1/files` with `purpose=user_data`, automatically use `/v1/responses` with `input_file` IDs even when text calls are configured for completions, then best-effort delete remote files.
- Limits: allow the existing document/text formats, max 20 MB per file and max 3 attachments per request.

### Implementation Result

- `DemandAttachment` is auto-migrated and records gzip location, SHA-256, original/compressed size, uploader, demand/task-group/context-pack/archive associations, and lifecycle status.
- Provider upload opens the gzip archive, streams decompressed bytes through `io.Pipe` into a multipart Files API request, references returned IDs in a Responses request, and deletes remote files after completion.
- Frontend retains `File` objects only until submit, sends multipart without setting its own boundary header, and passes returned attachment IDs into task import for transactional archive linking.
- Browser PDF/DOC parsing code and `mammoth`/`pdfjs-dist` dependencies are fully removed; repository search finds no remaining references.
- Focused end-to-end test proves provider receives the original bytes, Responses payload includes the remote file ID, remote cleanup occurs, local metadata is persisted, and import links the attachment to both demand and archive.
- Existing `chat/completions` configuration remains valid for text requests; the focused test now starts from completions configuration and proves attachment requests automatically target `/v1/files` plus `/v1/responses`.
- Final backend suite passes across `internal/server`, `internal/db`, and `internal/config`; frontend production build passes and no browser parser dependency/reference remains.

### Self-Reflection

- The first correction stopped too early at “support these file extensions” and chose a browser parsing implementation without tracing the product's provenance and provider contracts. For requirement evidence, preserve originals and design the persistence/provider boundary before selecting client libraries.

### Discovery Errors

- A repository search command used the shell pattern `.env*`; zsh rejected it before `rg` ran because no file matched. Future searches must quote the pattern or avoid shell globs.
- A search command listed nonexistent `main.go` explicitly, causing an `rg` path error even though the useful results were returned. Future searches should use discovered paths such as `cmd/server/main.go` only.
- Dependency removal initially failed because pnpm selected `.pnpm-store/v10` while current modules are linked from `~/Library/pnpm/store/v10`. Use the existing store explicitly to keep the lockfile operation bounded.
- Initial Go tests were blocked before compilation because the sandbox denied writes to `~/Library/Caches/go-build`; use a workspace-safe `/tmp` cache. Frontend build passes. `pnpm check` still reports the pre-existing `DemandKanban.svelte:1798` union-property error outside this attachment change.
- With `/tmp` cache, the backend compiled and tests advanced until an unrelated GitLab webhook test attempted to bind `httptest` on IPv6 loopback, which the sandbox blocks. Focused attachment tests will use approved loopback access.

## Demand Difficulty And Task Detail Surface — Initial Audit

- Demand scheduling uses a page-local custom difficulty dropdown even though the repository now has a shared Select with outside-click, keyboard, and viewport-aware placement behavior. The selected difficulty is intended to flow through `schedDifficulty` into `/api/tasks/schedule`, where the backend normalizes Low/Medium/High correctly.
- Replace only this custom control with shared Select and keep the existing estimate state/save payload. This removes duplicated open-state and outside-click logic from the affected field.
- The shared Modal is already a light frosted surface, but TaskKanban's slotted detail content still declares dark-theme values such as `#e2e8f0`, `#f1f5f9`, and a dark evidence summary. On the light modal these become unreadable or visually inconsistent.
- Add a final `.task-detail-surface` contract scoped to the modal content, covering labels, values, project/type/status badges, dividers, telemetry action, evidence summary/timeline/empty states, and signal chips without changing task data behavior.

### Implementation And Validation Result

- The custom difficulty trigger/listbox and its duplicated open-state handlers were removed. Shared Select now emits the selected Low/Medium/High value directly into `schedDifficulty`, while the existing schedule payload and manual-estimate source behavior remain unchanged.
- The light task-detail contract is appended after legacy TaskKanban styles and raises contrast for every screenshot-visible region: metadata values, long title, project/bug badges, status, telemetry button, evidence summary, empty-state warning, timeline, and signal chips.
- `pnpm --dir web check` passes with 0 errors and 74 existing warnings; the replacement added no new unused-selector or component diagnostics.
- `pnpm --dir web build`, `git diff --check`, and focused `TestDemandAndKPILogic` pass. The backend test confirms High difficulty persists with `EstimateSource=manual_adjusted`.
- Authenticated browser interaction was not available from the no-login preview, so the two exact logged-in surfaces were not click-tested in-browser this turn.

## Adaptive Overlays And Decision Timeline — Initial Audit

### Corrective Follow-up: Timeline Date Semantics

- User verification exposed that records such as 21:08 appeared under Today at 10:30. The timeline UI was structurally correct but the event contract carried only `HH:mm:ss`, and the first frontend implementation incorrectly supplied today's date.
- Database evidence confirms these are historical records, not timezone-shifted current events: TPY-6597 is `2026-06-19 21:08:05+08:00`, HIT-999 is `2026-07-09 17:54:33+08:00`, HR-4260 is `2026-07-08 17:53:34+08:00`, and ZK-944 is `2026-07-10 17:18:53+08:00`.
- `AutoDecision` now exposes `occurred_at` as the full source timestamp. The old `time` field remains for compatibility, while the frontend uses `occurred_at` for correct date grouping and local-time rendering.
- For a rolling deployment where the frontend briefly talks to an old backend, a time-only value later than the current clock is treated as the previous day instead of a future event; once the backend restarts, the exact source date wins.

### Corrective Follow-up: HR-4202 Reschedule Consistency

- The schedule page and intervention handler already read/write the same `task_telemetries.due_date`; this was not a stale schedule cache or a split-table problem.
- Read-only evidence shows HR-4202 remains `2026-06-25`, and the 10:10 decision event recorded `old_value=2026-06-25` and `new_value=2026-06-25`. The old UI therefore submitted the unchanged date and the backend incorrectly reported success.
- The decision form now consumes DatePicker's explicit change event, shows the exact pending target in the button label, disables submission until the target differs, and verifies the persisted `due_date` returned by the server.
- The server rejects no-op reschedules with HTTP 409, returns the persisted due date on success, and the regression test proves a decision change from June 25 to July 13 is immediately present in the schedule projection.

- The visible management shell is `FunctionalAdminShell.svelte`; its notification popover toggles locally but has no document/window click-outside listener. The legacy `App.svelte` notification branch already has such a listener, so changing only that branch would not fix the reported UI.
- `Select.svelte` and `DatePicker.svelte` are shared across the decision and settings surfaces, but both hard-code `top: calc(100% + 8px)`. Their existing click-outside and Escape behavior can be retained while adding one live placement calculation after render and on viewport scroll/resize.
- The decision page currently renders automatic and manual records as two columns with separate `min-height: 320px` cards. This makes a single manual record occupy a large empty panel and separates events that belong to the same operational chronology.
- Use one content-height panel with a unified event read model, date headers, a continuous timeline rail, distinct automatic/manual markers, preserved Jira/commit links, and a bounded internal list only when event volume requires it.

### Implementation And Validation Result

- The active shell now binds the notification wrapper and closes only when a window click lands outside it; clicks on notification actions remain inside the boundary.
- Shared Select and DatePicker primitives calculate available space after render and on captured scroll/resize, choose the roomier direction when the preferred panel does not fit, and cap internal panel height to the chosen viewport space.
- Automatic records and current-session interventions now map into one reverse-chronological event model, group under Today/Yesterday/full-date labels, retain Jira/commit links, and distinguish automatic/manual events with compact markers.
- Browser QA on the unauthenticated Settings preview confirmed Select opens down with room and adds `drop-up` at the bottom of a 1280x520 viewport; the panel stayed within `187px..363px` and emitted no console errors.
- Notification and decision timeline browser interaction remain login-gated in the preview environment. Static checks and production build cover those paths; no browser-login workaround was attempted.

## KPI Workspace Clean Rebuild — Correction Audit

- The screenshot confirms severe layout failure: the page content occupies only the left half of a wide workspace, the report inspector floats in an unrelated center column, the member table begins hundreds of pixels below the metric row, and the rest of the viewport is empty.
- This was caused by retaining old `.kpi-dashboard` definitions with named areas such as `header`, `focus`, `segments`, `diagnostic`, and `breadth` after those DOM sections were removed. Later CSS attempted to redefine the grid, but earlier `display: contents`, area ownership, sticky report sizing, and responsive rules continued to conflict.
- The prior implementation therefore violated the redesign skill's highest-leverage rule: it changed structure without removing the obsolete layout system. A clean component replacement is lower risk than trying to reason about more than 3,000 lines of accumulated KPI CSS.
- The corrected information hierarchy will be source-order aligned and compact: controls, four health metrics, delivery/risk overview, member capability table with adjacent detail, and evidence. No empty named tracks, sticky center inspector, or page-level masonry.

### Clean Rebuild Result

- `KPIKanban.svelte` was replaced instead of patched. The new file has one script/read-model layer, one source-ordered markup tree, and one style block; no `.kpi-dashboard`, `.report-panel`, `.member-matrix-panel`, legacy named area, or `display: contents` contract remains.
- Wide authenticated measurement: the KPI workspace uses the full 1720px content width; toolbar, metric strip, overview, member panel, and evidence panel share the same x-coordinate and width. Document overflow is zero.
- The first runtime pass found that the risk/meeting list made the overview row 689px tall. The final risk panel is capped at 430px with internal scrolling and independent card heights, moving the member workspace from y=1057 to y=798 on the same viewport.
- Medium-width verification collapses overview and member layouts independently. Narrow verification has zero document overflow; only the 900px data table scrolls within its bounded table shell.
- Real interaction checks passed for weekly to daily switching, team metric refresh, member row selection, personal report generation, evidence scoping, and return to team report. Browser console returned zero errors.
- Self-review lesson: for a heavily iterated component, markup replacement must be paired with deletion of the old layout/CSS system. End-of-file overrides are not a valid structural refactor.

## Main Workspace IA And Metrics Rebuild — Initial Audit

- The shared shell already provides breadcrumb and route identity, so component-local title/description cards repeat context and delay access to the first actionable row.
- The requested Decision, Schedule, Evidence, Task, and KPI removals are explicit authorization to restructure those surfaces; protected behavior is the underlying data and operational actions, not the duplicate presentation wrapper.
- Product UI constraints for this pass: first-screen metrics must be scannable in one row, data numerals use tabular figures, cards exist only where they group a decision or comparison, and child pages must own loading/empty/error states.
- Task Table and Execution Tracking should remain separate information views: the first answers ownership/status/priority, while the second answers delivery evidence and execution state. Their filters and selected-task context should be aligned, but merging both datasets into one dense table would weaken both jobs.
- People Load is the primary task-tracking redesign target and should prioritize capacity, active work, bugs, delay exposure, and concentration risk per person rather than decorative summary tiles.
- KPI reconstruction must first locate the existing daily/weekly report contract and the AI/manual demand score source. If the backend already exposes sufficient task/demand fields, prefer a frontend read model; add or alter API contracts only when a required metric cannot be computed correctly client-side.

### Audit Questions

- Which shell state should own task submenu selection and how is it synchronized with the component's current `taskView` state?
- Which existing records expose demand type, delay/overdue status, owner, progress, risk, and base score without introducing synthetic data?
- Which page-local header wrappers can be removed without also removing useful actions, filters, or loading feedback?

### Implemented Decisions

- Decision now starts with one six-column priority row: weekly demands, weekly bugs, visible items, high risk, medium risk, and automatic records. The duplicate `核心决策流转` command card is gone.
- Schedule and flow views no longer repeat their route title. The new-demand action lives in the schedule filter toolbar and a compact flow control row; authenticated desktop verification confirms the eight schedule controls remain on one row with zero document overflow.
- Evidence search/count/refresh moved into the evidence table header, allowing the `项目介入优先级` intro card to be removed without losing control access.
- Task navigation now belongs to the global rail. Task Table and Execution Tracking remain separate because ownership/status and code/MR evidence are distinct user questions; both share the same project/owner filter surface.
- People Load is now a comparison table ordered by delay and active load, with task/Bug counts, a six-active-item capacity reference, explicit scheduling status, and a selected-person risk inspector.
- KPI uses the existing period/report endpoints for daily and weekly views. The first row now exposes delivery progress, risk exposure, evidence coverage, and delay ratio; the personal matrix exposes task count, Bug count, delay ratio, requirement base score, capability rating, and delivery evidence.
- Requirement base score is derived from AI deconstruction difficulty (High 90, Medium 75, Low 60) or estimate duration fallback; existing `manual_adjusted` scheduling estimates are counted as human revisions. Capability rating combines 60% requirement base score and 40% evidence-backed KPI score.
- Browser testing found two hidden Svelte dependency-order bugs in derived Decision and Task metrics. Both were corrected by declaring the metric arrays and explicitly referencing their reactive inputs; the first loaded render now shows real counts rather than zero or a blank page.
- The first workload-profile assertion also exposed a denominator mismatch: delay ratio excluded demands from task/Bug volume but included overdue demands in the numerator. The final model now computes both numerator and denominator from the same task/Bug population.

## Rounded Surface And Modal Corner Audit — Initial Findings

- The supplied screenshot shows a correctly rounded white modal body but four pale rectangular corner protrusions outside that radius. This points to an unclipped inner/decorative layer or a square background/shadow layer, not to the outer modal radius being absent.
- The visible artifact is strongest at the top-right and bottom corners where a square layer extends beyond the white rounded surface; the fix must clip all modal layers at one authoritative outer container rather than merely increasing `border-radius`.
- The requested visual direction is a flat, calm, rounded management console: outer modals/panels should use the larger container radius, while inputs and inner controls keep a tighter related radius.
- Prior schedule work in `DemandKanban.svelte` is currently uncommitted and belongs to the same user workflow; the corner audit must preserve it while keeping background `task_status.md` and `well-ambient.db` changes untouched.

### Audit Questions

- Which shared tokens/classes own modal, card, panel, table-shell, inspector, and workbench radii?
- Which pseudo-elements, gradients, inset layers, sticky sections, or `overflow: visible` declarations can visually escape a rounded parent?
- Which page-level legacy overrides reintroduce square corners after the shared light-console calibration?

### Confirmed Root Cause

- `DemandKanban.svelte` gives `.modal-content` a `20px` outer radius, but both `.demand-create-modal` and `.schedule-modal` retain `overflow: visible` so dropdowns and date pickers can escape the modal box.
- The modal header and footer use their own translucent backgrounds without matching top/bottom corner radii. Because the parent does not clip descendants, those square child backgrounds remain visible in all four rounded-corner cutouts exactly as shown in the screenshot.
- Simply changing the modal to `overflow: hidden` would remove the artifact but regress open dropdown/date-picker overlays. The safer repair is to give first/last modal surface sections matching inner radii and reserve clipping for surface primitives that do not host escaping overlays.
- Shared `.wa-admin-card` and `.wa-admin-table-shell` already declare the large radius token, but only table shells clip their content through `overflow: auto`; shared cards lack a consistent `background-clip`/isolation contract and must be audited for square child layers.
- Structural `.wa-admin-section` intentionally has no background or border and should not receive decorative radius merely because of its name; only visible surfaces should be rounded.

### Cross-Page Audit Classification

- The reusable `shared/Modal.svelte` is already safe: its container owns the radius and `overflow: hidden`, so header/body backgrounds cannot escape.
- `ProjectHealthTelemetry.svelte` also places scrolling on the rounded modal container itself; its current overflow behavior clips inner content and does not reproduce the screenshot artifact.
- `CommitTelemetryPanel.svelte` is an intentional edge-to-edge right drawer, not a floating card. Its square viewport-edge corners are part of the sheet pattern and should not be globally rounded like a centered modal.
- Most `2px`–`5px` radii found in production components belong to scrollbar thumbs, progress bars, status markers, small badges, or legacy micro-controls rather than visible outer cards. Raising all of them would create oversized pills and blur hierarchy.
- The many `border-radius: 0 !important` rules in `SettingsPanel.svelte` reset transparent nested legacy wrappers inside already-rounded `section-card` surfaces. They are structural flattening rules, not exposed square cards, so they should remain zero.
- The global audit therefore needs a semantic surface contract rather than a numeric search-and-replace: centered modals and visible cards/panels round and clip their decorative layers; transparent wrappers and viewport-edge drawers remain structural.

### Primary Page Surface Inventory

- The active global scale is already appropriately rounded: `6 / 8 / 10 / 14 / 18px`, with `18px` assigned to outer admin cards, toolbars, and table shells.
- Decision, task, demand, KPI, deconstructor, and Settings production pages consistently opt visible outer panels into `wa-admin-card`, `glass-panel`, `section-card`, or `config-audit-panel` classes. Their final light-console overrides resolve to `14px`–`22px` outer radii.
- KPI's earlier dark `.glass-panel { border-radius: 8px }` is superseded later in the same component by the global `18px` token; it is not an active sharp-corner gap.
- Settings' earlier `12px` surfaces are superseded by Phase 41/46 page rules using `14px`–`22px`, while zero-radius rules remain confined to transparent nested wrappers and flat inner sections.
- The shared design system should be hardened with `background-clip: padding-box` and isolation on visible surface primitives so translucent backgrounds, borders, and pseudo layers respect the declared radius without forcing overflow clipping on interactive containers.

### Implemented Surface Contract

- Shared `wa-panel`, `wa-admin-card`, `wa-admin-toolbar`, and `wa-admin-table-shell` surfaces now use `background-clip: padding-box` and their own isolation context, so translucent backgrounds and borders render inside the declared rounded geometry.
- The reusable shared modal now applies the same surface contract and explicitly rounds its top header layer in addition to retaining parent overflow clipping.
- Demand, schedule, detail, and confirmation modals now share a `20px` outer radius with `19px` inner edge radii on header/footer surfaces. This removes square corner protrusions while leaving dropdown and date-picker overflow available.
- No page-specific mass radius rewrites were needed: the audit found that visible Decision, Task, KPI, Deconstructor, Settings, and telemetry surfaces already use the rounded outer contracts; remaining small/zero radii are intentional inner geometry.

### Authenticated Browser Evidence

- Reused the existing signed-in local management-console page at `localhost:5173`; the latest source changes were already hot-reloaded.
- The new-demand modal computes to `20px` outer radius, `19px 19px 0 0` header radius, padding-box surface clipping, isolated painting, and intentionally visible X/Y overflow.
- Pixel-corner hit testing at all four `1px` corner insets resolves to the modal backdrop, while `10px` insets resolve to the rounded header/footer surfaces. This proves the former square child backgrounds no longer occupy the rounded cutout area.
- A current browser screenshot confirms the pale rectangular protrusions shown in the user screenshot are gone and the shadow now follows a visually continuous rounded silhouette.
- Schedule-page visible outer surfaces compute to `14px`–`20px`; the only `0px` hit is the deliberately flush inner table viewport nested inside a clipped `20px` table panel, so it has no exposed square outer corner.
- Decision-page metrics, table shell, inspector, and log panels compute to `16px`–`20px`, with no visible square outer surface.
- Evidence-page loading surface computes to the global `18px` outer radius; no square outer container is exposed while its data view refreshes.
- Task-page header, metrics, stage selectors, toolbar, table shell, and inspector consistently compute to `10px`. This is the intentionally tighter dense-workbench tier, still rounded on all four corners with no `0px` exposed surface.
- KPI route was observed in its loading state; no exposed card shell existed during that instant, while the component's final CSS audit confirms its populated `glass-panel` surfaces resolve to the global `18px` radius.
- Settings' visible audit surface computes to `8px`, a smaller nested-panel tier with all four corners rounded. Static inspection confirms its primary section cards retain the larger `14px`–`22px` outer tier.

## Controlled Autonomous Delivery Rollout — Initial Findings

- The repository already exposes deterministic intent recognition through `/api/ai/intent`, but the current handler does not yet call the configured LLM enhancement path.
- AI deconstruction already selects and persists a `context_pack_id`, and task import archives the normalized output with a trace.
- Imported AI work is represented as shadow execution tasks linked to a demand through `task_group_id`; this is the right compatibility boundary for autonomous execution.
- GitLab webhooks already record branch/MR evidence and trigger an AI MR review report, but the application does not create branches, commits, MRs, or assign reviewers.
- The new design should extend the existing strongest-brain and policy infrastructure instead of adding a separate cockpit or a second task system.
- Reviewer selection needs a two-stage contract: required roles/candidates/approval rules are frozen with the demand spec, while concrete people are resolved again from the actual changed paths before the MR becomes ready.
- Raw AI output, human edits, test evidence, review outcomes, and acceptance outcomes belong in an immutable execution/case record. Only curated, scoped, versioned candidates may enter the trusted context registry.
- The current worktree contained substantial completed UI and visibility work. It was checkpointed as commit `8f50f00` before this rollout.
- Database schema changes use GORM `AutoMigrate` in `internal/db/db.go`; the new delivery models can be added without a separate migration framework.
- Permission seeding is incremental. Super admins automatically receive every new permission, while admin/member grants must be selected deliberately.
- The existing authorization wrapper already maps permission prefixes to resource types and supports repository-scoped query parameters, so execution routes should reuse `withPermission` rather than invent another authorization layer.
- GitLab API support currently covers project discovery and webhook management through a small authenticated HTTP client. Controlled delivery should extract a reusable API client pattern while keeping tests on `httptest.Server`.
- `RepoMapping` already provides name, path, and project ID. That is sufficient to resolve a configured repository without adding another repository table in the first cut.
- The existing Go 1.22 `ServeMux` path variables are already used for version rollback and can safely support spec freeze and review-contract approval routes.
- Demand-spec lifecycle works cleanly as an extension of `TaskTelemetry`: the demand remains the root, each draft gets a monotonically increasing version, and frozen rows are immutable.
- Creating a new draft can invalidate only unstarted older runs (`pending` and preflight states) without touching executing or completed delivery evidence.
- Reviewer resolution is safest as a pure deterministic function. Protected-path candidates are selected first, then the general candidate pool fills the minimum approval count; the author and unavailable identities are excluded case-insensitively.
- The execution engine can validate explicit structured file actions without interpreting shell text. Relative path normalization, `.git`/parent traversal bans, duplicate detection, and encoding validation form the v1 change-set boundary.
- Preflight should re-resolve reviewers from the actual change paths and persist that resolution on the approved review contract; this preserves the early contract while allowing diff-aware final assignment.
- GitLab's repository commit API is a strong safety boundary for v1: the system submits a validated list of file actions in one commit instead of executing model-authored shell commands.
- Reviewer names must resolve to concrete active GitLab user IDs before branch creation; otherwise the run fails before any write-side effect.
- Run-key idempotency prevents repeated create/start requests from duplicating branches, commits, or MRs. Repeated status refresh remains append-only by using unique action-event keys.
- Automatic tests are represented by the GitLab Pipeline triggered by the pushed topic branch/Draft MR. The system records required test commands and refreshes the authoritative pipeline result instead of claiming local shell execution.
- AI code generation is bounded to supplied source snapshots for updates; it may create safe new paths, cannot delete files, and its JSON output must pass the same deterministic preflight validator before execution.
- Human acceptance and MR merge are separate facts. A run enters `acceptance_pending` after the named acceptance owner approves it, and only the GitLab merge webhook can close it as `delivered` when the pipeline is also successful.
- The existing demand-detail modal is the lowest-coupling UI integration point: it keeps specification, contract, run, Pipeline, MR and acceptance context attached to one demand without adding another cockpit or task surface.
- Moving the existing Deconstructor from the evidence page to the demand board preserves its richer editing workflow; importing reviewed shadow tasks now also creates a versioned spec draft from the archived context pack.
- Delivery feedback needs an explicit quarantine state. `CorpusCandidate.status=pending` is never selected by context-pack assembly; accepting a candidate creates a separate versioned, active Context Fact with archive provenance and a content hash.
- Candidate generation is idempotent by run/type/scope, so both a human-verification path and an MR webhook can safely request feedback generation without duplicates.
- Final regression confirmed the demand-options implementation intentionally exposes only configured core/JQL identities; the stale test was updated to assert that local non-core users and historical task owners remain excluded.
- A completed execution task without commit/MR evidence must produce `evidence_missing` even when its parent demand is independently due soon; parent schedule risk cannot suppress the stricter completion-evidence rule.

## Phase 50 Settings Column Height Synchronization

### Root Cause And Fix
- Phase 49 gave the audit pane its own sticky viewport height, so its bottom edge could not follow the left configuration surface.
- Simply changing the grid to `align-items: stretch` was insufficient: the long GitLab diff then contributed its intrinsic height and expanded both columns to `1984px`.
- The desktop audit panel is now absolutely contained by a stretched, zero-intrinsic-size grid item. The left configuration surface determines the row height, while version/diff overflow remains internal to the audit panel.
- At `max-width: 1280px`, the audit returns to normal flow with a bounded `640px` height so stacked layouts remain readable.
- Version and diff scroll regions now use the same `10px` WebKit geometry, light track, bordered teal thumb, hover color, Firefox `thin` width, and `scrollbar-color` as the global admin tokens.

### Browser Verification
- At `1600x900`, GitLab, Feishu, Jira, Project priority, and AI engine all report a `0px` primary/audit height delta.
- Measured paired heights are GitLab `1206px`, Feishu `552px`, Jira `707px`, Project `535px`, and AI `642px`.
- GitLab long diff content is `1558px` inside a `780px` scroll viewport; Jira's version history is `440px` inside a `209px` scroll viewport.
- At `1200x760`, both stacked columns resolve to `1153px` width, the audit is bounded at `640px`, the panel returns to relative positioning, and document horizontal overflow remains `0px`.

## Phase 49 Settings Adaptive Layout And Control Consistency

### Screenshot Findings
- The audit panel inherits the height of the much taller main configuration content, creating a large blank right column while the actual version list and diff preview are compressed into a narrow upper region.
- Version history already has its own scroll region, but the inspector grid fixes the list/detail rows too aggressively and constrains diff values to short nested scrollers; long arrays become unreadable raw JSON strips.
- The first Settings breadcrumb is a real button inside a globally styled breadcrumb surface, which visually creates a nested pill/card. Settings also owns this breadcrumb locally, so routes outside Settings never receive the same page context.
- GitLab overview explicitly renders `repoList.slice(0, 5)`, even though its count reflects the full configured repository list.
- Project editing mixes a bespoke searchable dropdown with the shared `Select`, so the three fields do not share one trigger, option, hover, and focus language.
- Select and combobox hover rules are duplicated across shared tokens and page-local CSS. Dark legacy hover declarations can still win in some contexts.
- Placeholder typography is independently overridden in several demand/task selection controls, producing sizes that are larger than surrounding control text.

### Implementation Direction
- Give the audit inspector a viewport-bounded sticky height, a bounded scrollable version list, and a flexible diff detail region with wrapped/preformatted values.
- Render one shell-owned breadcrumb/context bar for all primary routes; Settings supplies its subgroup and current child label through the shell contract instead of rendering another local breadcrumb.
- Render every configured GitLab repository and bound the table container with an internal vertical scrollbar.
- Use shared `Select` for Project phase/priority/weight and align the searchable Project Key picker to the same visual tokens.
- Lock light hover/open backgrounds and 12px placeholder typography in shared input/select tokens, then remove stronger page-local exceptions where needed.

### Browser Verification
- The QA entry now mounts the real `SettingsPanel` with deterministic config/version fixtures instead of a hand-authored audit placeholder.
- At 1600 x 900, the Settings grid resolves to `1119px 420px`; the audit inspector is sticky, 720px high, and the diff region scrolls internally (`1558px` content inside a `294px` viewport).
- At 1280px the audit inspector moves below the main content at full width, avoiding a compressed side column.
- At 390px the grid resolves to one 359px column with no document-level horizontal overflow; the audit workspace stays bounded at 620px.
- A Jira fixture with nine visible versions produces a 189px/440px internally scrollable version list on narrow screens.
- A GitLab fixture with twelve configured repositories renders all twelve rows; the repository table is 358px high with 690px scroll content.
- Project phase and priority use the shared non-search Select, while Project Key uses the shared searchable Select. The open trigger and menu compute to white/light surfaces, and the Project Key placeholder computes to 12px.
- The async Settings summary was corrected so breadcrumb meta and header facts now both report `12 个仓库` after config load instead of retaining the initial zero state.

## Phase 48 Unified Settings Main-Content Rebuild

### Correction
- The latest implementation completed a page-level rebuild for `GitLabConfig.svelte` only.
- `SettingsPanel.svelte` grid/audit tuning and `Switch.svelte` token changes affect multiple routes, but are shared infrastructure rather than page-level rebuilds.
- Jira and Project contain earlier substantial local changes; Feishu and AI contain smaller earlier changes. None should be treated as aligned with the new GitLab workbench until audited against the same structure and states.

### Locked Direction
- Product UI, calm precision, light management console, `SPECTACLE=2`, `DENSITY=8`.
- Preserve API contracts, field names/order, save behavior, left-rail IA, and the existing teal accent.
- Prefer flat sections and sparse dividers over nested cards; use labels above controls, visible status language, fixed action placement, and explicit loading/empty/error/disabled states.

### Route Inventory
- Integration/AI child components: GitLab, Feishu, Jira, Project priority, AI engine, and AI context/system-design corpus.
- SettingsPanel-owned routes: KPI, users, permission matrix, authorization policies, and audit.
- The user-visible left rail exposes all 11 routes; therefore visual verification must cover all 11 even when only six use child config components.
- `SettingsPanel.svelte` already owns the common breadcrumb, route summary, main/audit split, and several non-integration page implementations.

### Child Page Audit
- Feishu and Jira still use the legacy `Steps` wizard and full success-screen pattern; both overview pages can report `已就绪` before a health check has actually run.
- Feishu and Jira duplicate overview rows, credentials, summary cards, and action CSS instead of sharing the GitLab workbench language.
- AI engine and system-design corpus share one very large component with two conditional surfaces; engine editing still uses the legacy wizard while the corpus has its own nested registry/editor visual system.
- The existing data/test/save functions are usable and should be preserved; the main work is state semantics, markup hierarchy, and shared presentation.
- Project priority is a list/editor page with useful loading, empty, error, and success behavior, but its markup still uses the old card-header/table/form vocabulary and relies on a long stack of local CSS resets.
- A shared, prefixed workbench stylesheet is lower risk than another round of broad `:global(... !important)` overrides: new markup can opt into the contract while legacy selectors become inactive.

### Shared Workbench Contract
- Root: one full-width `scw-workbench`, no nested page card.
- Overview: compact header with labeled switch, explicit status banner, two-column read grid, optional fact/table section, and one bottom action bar.
- Edit: compact segmented stepper, flat bordered sections, two-column field grid, inline test feedback, and a stable bottom action bar.
- Data pages: title/action toolbar, four-column summary strip, normal-density table, composed skeleton/empty/error states, and inline editor surface.
- Status semantics: `success`, `error`, `incomplete`, `checking`, and `unchecked`; never infer `ready` from the absence of an error.
- Shared `Button`, `TextInput`, `Select`, and `Switch` already expose the required labels, focus treatment, disabled states, and semantic variants; the rebuild should compose these controls rather than duplicate their CSS.
- The workbench stylesheet only needs to own page hierarchy, grids, tables, status surfaces, native textarea/range fields, and action placement.

### AI Surface Findings
- AI engine `isConfigured` currently includes active context-fact count, so an engine with no endpoint can incorrectly enter the configured overview just because the corpus has data. Engine configuration state should depend only on engine fields.
- The engine overview also defaults to `已就绪` without a connection test; it needs the same explicit unchecked/incomplete/error/success model as GitLab, Feishu, and Jira.
- The system-design corpus already has real loading, empty, retry, edit, save, preview, and result states. Its behavior can stay intact while the registry/list/editor shell is flattened into the shared workbench.

### Visual QA Path
- Production preview and the existing development tab both require authentication; no credential submission or auth bypass is appropriate for this task.
- Vite already supports a separate multi-page prototype entry, so a second local-only `settings-preview.html` can mount the real config components with deterministic mock API responses.
- The QA page remains outside production navigation and validates component pixels, overflow, responsive collapse, and edit-state transitions without changing authentication behavior.

### Desktop Visual QA
- The standalone QA entry mounts the real GitLab, Feishu, Jira, Project, AI engine, and system-design components with deterministic fixture responses; it does not duplicate their markup.
- GitLab, Feishu, Jira, Project, and AI engine overview/edit states have no document-level horizontal overflow at 1280 x 720.
- Project keeps the three-row table within its own content width and the edit form resolves to two balanced columns.
- Feishu and Jira keep the four-step editor inside the workbench; their status banners remain explicit instead of reporting a successful connection by default.
- AI engine summary metrics resolve to three equal columns. The two endpoint choices now use an explicit two-column variant so their track count stays deterministic.

### Narrow-Viewport Finding
- At 390px, Feishu, Jira, Project, AI engine, and system-design overview states initially stayed within the document width; Project intentionally scrolls only its table container.
- GitLab exposed a real component defect: an inherited grid min-content track expanded a 329px workbench to 471px. The production component now clamps the root and first-level grids with `minmax(0, 1fr)`, `min-width: 0`, and `max-width: 100%`.
- After the correction, all six integration/AI routes remain within a 390px document width. Feishu's four-step control and Project/GitLab tables keep overflow inside their own bounded containers.

### SettingsPanel-Owned Routes
- KPI, members, permission tree, authorization policies, and security audit are not newly rewritten in Phase 48. They already contain page-specific Phase 46 structures (KPI surface, member table, permission tree, policy builder/explainer, and audit tables) rather than receiving only shell decoration.
- Current static review confirms those five routes continue to use the shared flat section/table/control rules. Phase 48's new page-level implementation scope is the six integration/AI routes.

## Phase 45 Settings Surface System Rebuild

### Requirements
- Rebuild every page under the Settings menu as one coherent product surface.
- Align spacing, typography, controls, states, and responsive layout across all configuration modules.
- Use restrained frosted glass at page-level surfaces only.
- Do not nest capsule-like containers around switches, inputs, or other controls.
- Preserve existing APIs, permissions, configuration behavior, and version rollback flows.

### Initial Visual Findings
- The supplied Jira screenshot has too many independent rounded outlines: page frame, summary panel, metric tiles, field tiles, JQL box, version cards, and inner diff boxes all compete at the same visual level.
- The toggle is visually isolated inside its own rounded halo instead of belonging to a clear section header/action row.
- Configuration facts are rendered as equal cards even when they are simple label-value rows, producing a card-in-card rhythm and wasting vertical space.
- The audit column is narrow but contains nested cards and a second scroll region, making the hierarchy cramped and inconsistent with the main panel.
- Frosted glass should separate page regions, not be applied recursively to every field.
- The previous Phase 45 convergence block explicitly applied glass, borders, and radius to field groups, credentials, registry items, project items, version items, and diff rows; this directly caused the nested-container appearance.
- Integration modules were additionally wrapped by `section-card`, creating a redundant surface around child `wizard`/`project-config-container` roots.

### Technical Direction
| Decision | Rationale |
|----------|-----------|
| Use a flat section + field-row model | Removes nested capsules and makes scanning/editing faster |
| Keep one page-level glass surface per column | Preserves depth without gray overlays or card nesting |
| Use a single teal accent and cool neutral scale | Matches the existing shell while avoiding purple/gray drift |
| Keep version history as a compact inspector | Audit remains available without competing with configuration work |
| Keep legacy root class during migration and add a later Phase 46 contract | Prevents Svelte unused-selector noise while the new contract wins by cascade order; visual behavior is owned by Phase 46 selectors |

### Browser Verification Findings
- GitLab, Feishu, Jira, AI engine, and project summaries render as flat label-value lists inside one primary glass surface.
- Jira and AI edit modes render field wrappers with transparent backgrounds, zero radius, and zero outer borders; only actual controls retain control styling.
- Version history is a flat selectable list with the selected version detail below it; it no longer compresses detail into a narrow nested column.
- The context library uses a flat list/editor split with one divider; option sets are borderless grids without an outer capsule.
- Member, permission, policy, audit, and KPI routes contain no dark surface drift; the permission branch card and decorative side stripe were removed.
- At 1280px the project table receives full width and the audit inspector moves below, eliminating horizontal table scroll.
- At 760px the global navigation is an off-canvas drawer and the configuration page remains visible behind it.

### Pre-flight Evidence
- One teal accent is locked across configuration routes; status colors are semantic exceptions only.
- Glass is limited to breadcrumb, page header, primary work surface, and version inspector.
- Gradient text was removed from GitLab, Feishu, AI, and project success/header states.
- Browser metrics found no content overflow at 1024px or 760px and no decorated field wrappers.


## Requirements
- Phase 41 correction: user paused the rollout because the current implementation drifted away from the original prototype style and data-chain intent.
- Phase 41 execution model: recalibrate first, then use subagents; one common agent owns shared components/data contracts, and one page agent owns each page slice.
- Phase 41 visual baseline: `output/functional-admin-console-reference.png` is the primary target because it uses `well-ambient`; `functional-admin-console-reference-alt.png` and `modern-admin-reference-phase40.png` can inform layout but must not reintroduce `最强大脑`.
- Phase 41 protection rule: page agents must not invent new page-local visual systems; they must consume the shared prototype-aligned contract from the common agent.
- Current Phase 40 feedback: prior prototype passes are still not beautiful or comfortable enough; use `finesse-skill`, `taste-skill`, and `ui-skill` together, generate a visual prototype first, then implement the chosen direction.
- Phase 40 output requirement: the prototype must be directly visible without login so visual quality can be judged before migrating existing authenticated business pages.
- Current Phase 37 requirement: execute the recommended UI redesign implementation order by starting with audit, coded prototype, componentized design substrate, safe app-shell exposure, and frontend validation.
- Phase 37 UI protection rules: do not revive the rejected visible decision queue, do not revive the visible delivery cockpit, do not remount project health telemetry, and do not rewrite existing business pages until the prototype proves a concrete operator workflow.
- Phase 37 design read: `研发协同管理台 · restrained glass + evidence-first · register=product · SPECTACLE=2 · DENSITY=9`.
- Current rollout requirement: commit all uncommitted code, then use subagents to start implementing every phase from `docs/strongest-brain-delivery-transformation-plan.md`.
- New baseline commit before rollout: `adcc981 chore: capture strongest brain baseline`.
- Current worker split: Worker A owns backend evidence/exception/weekly decision read-model helpers, Worker B owns AI trace/readiness/context-pack backend helpers, Worker C owns frontend delivery cockpit integration.
- Phase 26 rollout is now implemented as a cross-phase delivery cockpit plus supporting read models, not as a revived visible decision queue.
- Check and repair the repository according to `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`.
- Planned areas: confirmation modal styling, WellOS department extraction, select dropdown truncation, AI deconstruction to demand association, demand card shadow task progress, targeted tests/builds.
- Follow-up requirements: require `taste-skill` for frontend UI work, explain/deepen demand scheduling and AI deconstruction binding, fix user department display, and fix the native-looking new-demand due-date style.
- Current extension requirement: generate a concrete landing plan from the architecture recommendations and use self-agents to land GitLab/Jira integration, KPI report preview, and AI deconstruction expansion in bounded implementation tracks.
- Current task addendum: compact the work-summary effort/AI-evaluation controls, optimize the config version audit/rollback surface, lock background scroll under modals, repair assignee/Jira/detail interactions across decision and demand boards, and make demand/task/bug status transitions feel seamless with strongest-brain recommendation guidance.
- Current backend worker scope: implement the strongest-brain v1 backend read-model loop for EvidenceChain, DecisionQueueItem, schedule governance reuse, and AI intent recognition/summary, limited to backend server/db/telemetry files and focused tests.
- Current Phase 22 requirement: based on `docs/strongest-brain-phased-master-plan.md`, add AI intent recognition and summarization to the plan and implementation, checkpoint all existing code first, then use subagents to land the complete strongest-brain plan as a v1 closed loop.
- Current Phase 23 continuation: proceed into master-plan Phase 4 by adding a schedule-governance risk calendar read model and compact schedule-view surface, while preserving the existing `/api/schedule` contract.
- Current Phase 24 feedback: the visible "决策队列" does not yet communicate its display meaning; improve the cockpit so it shows why an item matters, what decision is needed, what happens if it is ignored, and where the operator should act.
- Current Phase 25 feedback: the user still judges the visible "决策队列" as not useful; remove that visible surface instead of continuing to iterate on it.
- Current Phase 27 feedback: the visible strongest-brain delivery cockpit still does not prove its existence value; temporarily remove it and continue other phases through smaller, independent APIs.
- Current Phase 28 feedback: refine the visible schedule/evidence UI by fixing red-zone card height and Jira-line density, upgrading the schedule risk calendar's interaction quality, and moving demand deconstruction from evidence observation to schedule governance.
- Current Phase 29 bug: refreshing the decision dashboard briefly renders Git branch/commit-derived cards in the red-zone grid before config/member filtering removes them.
- Current Phase 30 direction: optimize project health telemetry into an exception triage surface and add explicit manual intervention directions for red/yellow projects.
- Current Phase 31 feedback: the visible project telemetry panel still does not show enough practical value; hide it from the evidence observatory for now.
- Current Phase 34 feedback: KPI page clicks cause visible jumping, and the dashboard needs more depth and breadth beyond the first score/matrix refactor.
- Current Phase 35 bug: "Jira Task 开发结果追踪" showed stale child execution assignee/status when the bound Jira Task had already been completed or reassigned.

## Research Findings
- Phase 41 primary visual baseline is a management console with dark left rail, light mist workspace, white frosted cards, compact metric cards, table-first main surface, right inspector, low-saturation shadows, teal/green accent, and brand text `well-ambient`.
- The later dark/glass per-page edits drifted because production pages were allowed to define local visual systems rather than consuming one shared prototype contract.
- Current `DESIGN.md` still describes the older dark-tech cockpit direction; it conflicts with the Phase 41 prototype-aligned target and needs an explicit override or update.
- Phase 41 common contract now supersedes the old dark cockpit contract: production pages should consume `web/src/lib/admin-console/contract.ts`, `docs/admin-ui-calibration.md`, and the `.wa-admin-*` token classes rather than inventing local visual systems.
- Decision, Schedule, Task, Evidence, KPI, and Settings now have production-page Phase 41 shells while preserving their existing API/data chains.
- The Settings worker stalled without output; the main thread completed `SettingsPanel.svelte` with a container-level adapter and shell to avoid blocking the page rollout.
- `SettingsPanel.svelte` still embeds legacy config child components; those are intentionally preserved for functionality and can be handled as independent child-component polish later.
- Settings IA correction: multi-area settings pages should use grouped submenus in the left nav, and the right work area should be one unified frame with breadcrumb navigation above the actual content.
- Settings rail correction: grouped Settings submenus must belong to the global `FunctionalAdminShell` rail/menu, not to `SettingsPanel.svelte`'s main content frame.
- Phase 44 correction: UI-related work now defaults to `ui-skill`, then falls back to `finesse-skill` and `taste-skill`; this replaces the older finesse-first/taste-only assumptions in local task memory.
- Phase 44 main-content rule: production pages should not render a module hero inside `FunctionalWorkspace` by default. The topbar is the page identity layer; content should begin with real metrics, compact controls, tables, and inspectors.
- Phase 44 visual correction: migrated page-level headers should behave like low-height toolbars, not landing-page or cockpit hero blocks; explanatory copy is hidden where it competes with the operational content.
- The previous Settings right side mixed hero, metrics, category table, module content, and inspector at one level; that made the information hierarchy unclear and conflicted with the requested management-console pattern.
- Phase 41 validation passes for TypeScript app build and Vite production build; remaining frontend warnings are existing repository-wide warnings, mostly in `DemandKanban.svelte`, shared modal/switch, and config child components.
- Phase 40 skill review result: `taste-skill` is not the primary dashboard implementation skill, but its image-first and anti-generic rules are the right correction for a visual-quality failure; `ui-skill` should govern component hierarchy, and `finesse-skill` should remain the product UI audit guard.
- The generated reference image validates a stronger visual direction than Phase 39: light workspace, dark translucent rail, flat information surfaces, restrained frosted glass, comfortable whitespace, task table, and right inspector.
- Keeping the new prototype at `/prototype.html` removes the previous visual-review blocker where the app landed on the login page before the `UI 原型` tab could be inspected.
- Vite needs explicit multi-page build inputs for `prototype.html`; otherwise the dev server can serve the file but production build only emits `index.html`.
- Phase 37 should be treated as product UI, not a brand/landing surface: density, tables, command rows, evidence panels, and clear interaction states matter more than hero spectacle.
- The current frontend has several large Svelte components with local style systems (`DemandKanban`, `SettingsPanel`, `TaskKanban`, `Deconstructor`, `DecisionDashboard`, `KPIKanban`), so a safe redesign starts by adding shared tokens/prototype surfaces rather than editing all pages at once.
- `App.svelte` currently uses top tabs for decision, schedule, evidence, and settings; the lowest-risk prototype insertion point is an additional permission-gated preview tab rather than replacing existing pages.
- The existing visual direction overuses emoji labels, gradients, and local glass panels; Phase 37 should replace those patterns in the prototype with restrained semantic status, compact command bars, fixed-density rows, and purposeful acrylic only on shell/overlays.
- Git status shows the repository contents are currently untracked; avoid treating that as disposable state.
- Phase 22 baseline checkpoint commit created before new implementation: `2a7890c chore: checkpoint current workspace changes`.
- Phase 22 uses three subagents with disjoint scopes: backend strongest-brain/AI intent API, frontend cockpit/AI conversation UI, and QA integration review.
- The active frontend design read is an internal研发协同 dark cockpit, not a marketing or landing surface; density should remain operational and UI copy should be plain, auditable, and concise.
- Phase 22 landed a v1 strongest-brain loop: master plan addendum, decision queue API, evidence chain API, deterministic AI intent/summary API, cockpit decision queue UI, and transparent deconstructor conversation panel.
- Full server package validation is still sandbox-limited because existing GitLab webhook tests bind a local `httptest` listener; targeted Phase 22 handler tests pass without listener binding.
- Phase 23 should reuse `buildScheduleResponse` and `resolveScheduleRisk` instead of creating a second risk taxonomy; the new calendar is a read-model projection over existing schedule facts.
- Current dirty `task_status.md` and `well-ambient.db` are not part of the Phase 23 scope and should remain unstaged unless explicitly requested.
- Phase 23 subagents timed out and were closed; frontend partial work appeared in the shared worktree and was integrated by the main agent, while backend/API/test work was completed locally.
- The Phase 23 risk calendar now returns additive week/month buckets plus event summaries, while the schedule table still consumes the existing `/api/schedule` contract.
- The decision queue API already returns source-level items from schedule, execution, and context read models, but the frontend normalized every API item as `strongest_brain`, hiding the item's origin and making the queue feel like a generic list.
- The existing queue cards had problem/evidence/action text, but no explicit current focus, no idle-cost language, and no clear handling entry for items that do not map to the old Agenda intervention panel.
- After adding meaning language, the user still found the strongest-brain decision queue surface useless. The better product move is subtraction: remove the surface and leave the actionable Agenda metrics/filter/intervention flow as the decision dashboard's main workflow.
- Removing the frontend queue fetch also stops the decision dashboard from polling `/api/strongest-brain/decision-queue` every 15 seconds when the visible queue is gone.
- Follow-up `find` showed `internal/server` and `cmd/server` do exist, but were not returned by the initial `rg --files` listing, likely due ignore rules.
- `DemandKanban.svelte` already contains demand/subtask filtering, card progress rendering, and confirm modal classes.
- `Deconstructor.svelte` already fetches active demands and posts `demand_id` to `/api/tasks/import`.
- `DecisionDashboard.svelte` already uses `width: max-content; min-width: 145px; max-width: 240px;` on `.custom-select-options`; `.custom-option` still has `overflow: hidden`, so long labels may still clip inside the widened menu.
- `handleLogin` already read WellOS response bytes before unmarshalling, but the direct raw log would expose auth tokens. It now logs a recursively redacted payload.
- `handleImportTasks` already accepted `demand_id`; it now rejects missing demand IDs instead of silently succeeding, preserves existing task metadata, and syncs imported tasks to markdown after commit.
- `ProcessWebhookEvent` previously rebuilt `TaskTelemetry` without `task_group_id`, `due_date`, creator, and decision-log fields; it now preserves those fields so AI task-group linkage survives Git telemetry.
- `DemandKanban.svelte` now filters archived demands/subtasks out of the active demand board.
- `TaskKanban.svelte` had a Svelte 5 compile error from a duplicate nested `{@const parentDemand...}` in the Done lane; the duplicate was removed.
- Earlier follow-up work recorded a fallback path for sessions where `taste-skill` is unavailable. In the latest request, the provided `design-taste-frontend` skill was read and applied to the touched demand UI.
- Department display was fragile because the frontend relied on localStorage/login response only. Department now travels through WellOS extraction, DB persistence, JWT claim, `/api/me`, and frontend refresh.
- Demand scheduling and AI deconstruction are now linked by the durable `task_group_id`: when a demand already has a task group, future AI imports reuse it.
- The new-demand due-date field now uses a styled date input shell with a custom calendar affordance and an invisible native picker hit target.
- The latest UI follow-up used the requested `design-taste-frontend` direction as an internal-dashboard repair, keeping the existing dark operational system instead of introducing landing-page styling.
- New demand creator departments could still remain hidden when the user row already contained placeholder values such as `未分配`. Those placeholders are now treated as missing, allowing JWT login department data or the frontend `creator_dept` fallback to overwrite them.
- The delete/archive dialog no longer inherits the old glass panel styling; it uses a flatter system modal with explicit target ID, compact metadata, and action-specific destructive/archive buttons.
- The latest follow-up replaces native date inputs entirely in touched demand forms with a custom calendar component, so browser-native date styling no longer leaks into the new-demand or schedule dialogs.
- Demand scheduling and AI deconstruction now share a stable generated task group when a demand has no explicit group: `brain-{demand_id}`. This lets schedule decisions and imported AI shadow tasks extend the same demand graph.
- New product demands should not auto-inherit the first GitLab repository. The repo field now stays empty on create, and demand cards show AI mapping state until shadow tasks or downstream development work establish repository context.
- A readonly personal profile surface is appropriate because OS profile data is authoritative and not editable in this app. It should live in the avatar dropdown rather than consume a main navigation tab, while `/api/me` repairs stale placeholder department/avatar fields from authenticated claims.
- WellOS department parsing must not accept generic top-level `displayName` as a department, because that can be the user's display name. Generic labels are now accepted only inside a known department context.
- When WellOS is in planned maintenance, the safe fallback is not password bypass for everyone. Only locally known users with existing group/permission snapshots and a matching local cached password hash from a prior successful WellOS login can receive a degraded session, and that session must be marked and audited.
- The app should visibly disclose degraded authentication, because OS-owned profile data may be stale until WellOS comes back.
- Local development has a separate need from production degraded auth. The development bypass must be opt-in via `WELL_AMBIENT_DEV_AUTH=1`, restricted to loopback requests, and visibly marked as `local_dev_auth`.
- GitLab webhook management belongs with the existing config handlers because `GET /api/gitlab/projects` already lives there and uses the `config:write` permission boundary.
- `internal/server/*` is ignored by `.gitignore` via the `server` pattern, so edits there do not appear in normal `git status`/`git diff`; verify with direct file reads and tests.
- GitLab API error responses should not be proxied back to callers because self-hosted GitLab can echo request details. Returning status-only messages avoids leaking `api_token` or `secret_token`.
- Existing Jira sync is already wired through `ProcessWebhookEvent -> SyncStatusToJira`; this MVP leaves Jira client behavior unchanged and focuses on automatically installing the GitLab webhook that feeds that path.
- GitLab project browsing already exists in the configuration panel and backend; the practical next step is webhook ensure/status automation over selected mapped repositories.
- KPI should move from raw completion ranking to factual performance previews: delivery count, cycle/risk indicators, report-preview narrative, and evidence links.
- AI deconstruction should move from task generation to a demand operation map: completeness, missing information, risks, dependencies, acceptance criteria, schedule notes, meeting questions, and confidence.
- The Override card had native-feeling assignee/deadline controls because it still used a plain text input and raw `input[type=date]`; replacing them with the existing dark-system custom control pattern fixes the mismatch while preserving the date input interaction.
- The red-zone filter can be presented as project filtering while continuing to use the existing `repo` JSON field from agenda telemetry; this avoids a backend/API rename for a label-level UX request.
- Native `<select>` styling in the KPI report preview is better replaced with a button-backed custom dropdown because browser option popovers cannot be consistently themed across platforms.
- Jira custom JQL summary overflow is a CSS containment issue: summary values need `min-width: 0`, right-aligned wrapping, and `overflow-wrap: anywhere` for long query strings.
- AI deconstruction already had editable `period_days` in the frontend, but the backend prompt did not require the model to output estimate fields. This made the visible schedule default rather than an auditable model estimate.
- Estimate accuracy evaluation needs both the original deconstruction run and task-level normalized estimates. A `DeconstructArchive` run record plus `TaskTelemetry` estimate fields keeps run-level and task-level evidence joinable by `task_group_id` and archive ID.
- Webhook/MR telemetry updates must preserve estimate fields, otherwise completion events would erase the baseline needed to compare estimated versus actual cycle time.
- The "strongest brain" product direction is to make daily/weekly coordination exception-driven: the system should assemble facts from demand decomposition, GitLab/Jira telemetry, KPI reports, and decision logs, then ask humans only for blocked decisions, missing context, and risk tradeoffs.
- AI deconstruction estimate quality depends on configured system context. The model now receives architecture, workflow, implemented capabilities, estimation guidelines, and work-hours-per-day from AI settings instead of guessing the current product map.
- Deconstructor task effort should be edited in hours. Day values remain stored as derived scheduling compatibility data, but the operator-facing control and prompt now treat hours as primary.
- Keeping AI analysis below the deconstruction button leaves the right result panel dedicated to project mapping and shadow-task cards, which better matches the user's review workflow.
- The strongest-brain landing model should turn status meetings into exception review: fact collectors ingest Jira/GitLab/AI archives/KPI data, the diff layer compares goal-plan-actual-risk, and humans only see direction decisions, resource tradeoffs, and override interventions.
- `全局领航能力汇总.xlsx` 第一个 sheet `功能列表` contains only the path-planning module capability catalog: 39 features across standard functions, extensions, advanced exploration, and customization.
- The path-planning sheet shows 20 durable capabilities, 7 initially validated, 3 development-complete, 3 in progress, 5 pending development, and 1 unmarked; 25 are configurable, 12 non-configurable, and 2 unclear.
- A long module capability catalog should be archived in a dedicated AI context document, with only a compact summary injected into `ai.project_architecture`, `ai.delivery_workflow`, `ai.implemented_features`, and `ai.estimation_guidelines`.
- Module prompt context must explicitly state scope boundaries so AI does not confuse path-planning business capabilities with well-ambient's collaboration-system capabilities.
- The path-planning/module-image direction was later identified as mis-scoped for “AI 上下文配置中心”: the center should configure system design, product capabilities, and delivery/process context for demand deconstruction, not behave as a targeted module capability feature.
- AI context should now be maintained as structured facts rather than four large visible text areas. The old AI config fields remain a backend fallback only, while the UI path is fact cards plus pack preview.
- A context pack must be persisted and linked to every deconstruction archive so future estimate review can reconstruct which system facts the model saw.
- The first Context Registry retrieval path is deterministic scope/keyword scoring with token budgets. This is intentionally simpler than embeddings so cache keys, tests, and audit behavior are stable before adding vector recall.
- Policy RBAC is now a layer over the existing permission matrix: explicit deny policies override legacy group grants, allow policies can grant exceptions, and global super_admin remains break-glass access.
- Authorization failures and high-risk grants should be logged as decisions with subject, action, resource, scope, matched policy, missing permission, reason, and risk level.
- Settings now treats AI engine configuration and AI context facts as separate admin tabs. This prevents the context registry from being perceived as part of provider/model setup.
- The visual permission matrix should not be maintained as a fixed table. The new Settings permission view fetches the live permission catalog from `/api/permissions`, derives branches by permission namespace, and renders vertical permission nodes so future seeded permissions appear without adding columns by hand.
- Policy authorization is easier to operate when it follows the sentence model "who can do what, where, and why". Templates, segmented subject/scope controls, permission action chips, and a priority stepper reduce the raw native form surface.
- The latest demand creation issue is a source aggregation problem: the modal needs candidates from users, Jira config/JQL, historical tasks, current user, and configured projects, not only `/api/users`.
- In this deployment Jira `Task` is a demand, not an execution-only task. The code should map Jira Task/Story/Feature/Epic-like issue types to `issue_type = demand`, while Bug/Defect-like issue types remain `bug`.
- Execution tracking remains useful after that change, but its scope becomes AI shadow tasks, manually split execution tasks, and Bug development evidence, not Jira Task demand intake.
- WellOS login no longer provides the authoritative avatar and department fields directly. After login succeeds, the backend must call `GET /api/user/info?token={token}` and use `data.avatar`, `data.realname`, and `data.department_name`.
- Demand schedule rows must distinguish workflow status from schedule completeness. `backlog` can mean a demand is waiting or planned, but it is not proof of schedule unless the row has both a development branch and a due date.
- The schedule table should keep schedule status, effort estimate, delivery evidence, risk, and update time in separate columns. Mixing branch/repo placeholders into the schedule column makes unscheduled work look scheduled.
- AI effort estimation can reuse the existing `/api/deconstruct` contract for the scheduling MVP. The schedule modal should treat it as an optional estimate baseline and persist the selected overall hours/days/difficulty only when the user confirms scheduling.
- Current code already contains partial fixes for this task: `modalScrollLock.ts`, scroll locking in shared Modal, DemandKanban and SettingsPanel manual modal locks, compact schedule effort controls, demand assignee editing UI, and a page-filtered config-version audit panel.
- Demand and Task boards currently fetch Jira base URL through `/api/config`, which requires `config:read`; normal demand/task users can therefore lose the Jira direct-link affordance even when Jira is configured.
- Demand assignee editing can fail for creators/assignees who are allowed to call `/api/demands/reassign` but cannot call `/api/demands/options`, because that route currently requires `demands:write` and the dropdown then only contains the current assignee.
- Agenda decision reassignment writes `TaskTelemetry`, decision logs, and kanban markdown, but Jira sync later unconditionally restores the Jira assignee. This explains the user-visible "success but no effect" behavior after a background sync.
- The config version audit panel now filters by section, but selection can still point at a version from another section and the two-column diff layout consumes too much height on integration pages.
- Final Phase 21 fix: expose only Jira `base_url` through authenticated `/api/jira/link-config`, allow authenticated demand users to load assignee/project options, preserve recent local assignee override logs from Jira sync for 24 hours, compact the schedule estimate strip, make config-version audit section-specific and lower height, and use card-level detail fallback on demand cards.
- Current Phase 22 backend constraint: this worker should not modify frontend files and must not revert changes made by parallel agents.
- Worker B Phase 26: `db.DeconstructArchive` already stores `context_pack_id`, `input_text`, deconstruction output JSON, confidence, completeness, and archive timestamps; `db.ContextPack` stores model, prompt template version, token budget/count, summary, hash, and item count.
- Worker B Phase 26: `/api/deconstruct` currently assembles and persists a context pack and returns `context_pack_id`, but it does not create a deconstruction archive because import/demand linkage happens later.
- Worker B Phase 26: before this slice, `/api/tasks/import` archived deconstruction output with `context_pack_id` and auto-built an `import_archive` context pack only when the request omitted one but included `input_text`; the MVP now also derives an input snapshot from imported tasks and returns archive/trace metadata.
- Phase 26 backend now exposes a consolidated strongest-brain delivery cockpit that aggregates evidence health, exception-only progress signals, weekly decision questions, schedule risks, AI trace metadata, override audit events, authorization explanation summaries, and concrete entry points.
- Phase 26 evidence read models now distinguish completed-without-evidence, merged-but-not-completed, scheduled-but-stale, due-soon, overdue, missing links, chain status, and evidence completeness.
- Phase 26 AI trace work keeps every generated recommendation replayable by `archive_id`, `context_pack_id`, prompt/source version metadata, confidence, completeness, selected context facts, and deterministic missing-question buckets.
- Phase 26 frontend initially mounted a delivery cockpit above the existing decision dashboard, but Phase 27 removes that visible surface because the user still does not see its value.
- Backend aggregate read models remain available for reuse, but user-facing strongest-brain work should proceed as focused phase-specific pages or APIs rather than a single all-in-one cockpit.
- Phase 2 can now consume `/api/strongest-brain/exceptions`, which returns an exception-only response with summary counts, severity, owner, deadline, evidence refs, missing links, close requirements, chain status, and evidence completeness.
- Phase 3 can now consume `/api/strongest-brain/weekly-decisions`, which returns decision-meeting mode data with weekly decision cards, options, evidence refs, owners, deadlines, decision debt, and must-decide counts.
- The strongest-brain decision queue remains a backend compatibility/read-model source, but the user-facing direction should not be to repackage it as a generic queue or cockpit.
- The first modern-admin prototype still read like a static concept board; the next iteration needs component boundaries, real operator controls, and a dense table-first workflow to satisfy the management-console requirement.
- For this admin UI, glass should be limited to shell and inspector separation. The main work surface should stay flat, scan-friendly, and table-oriented.
- The prototype should prove the exception-to-mediation loop before migrating existing production pages: command table, preflight drawer, evidence chain, workflow path, and weekly decision queue are the minimum useful slice.
- The componentized prototype became more usable but still lacked a memorable visual center; a bolder product UI needs a first-screen theatre that makes the selected issue, risk, evidence gap, and action intent visually unavoidable.
- Product UI can be bold without becoming a landing page when spectacle is bound to real state: ambient grids, acrylic edge depth, status signal glow, and a selected-issue theatre should point to operational facts, not decoration.
- The red-zone diagnostic cards need fixed row sizing and a compact identity header because variable title/project rows make the upper/lower card heights feel uneven.
- The schedule risk calendar should behave like a governance command surface: show the current focus, give direct risk filter actions with counts, keep refresh visible, and make each time bucket scan-friendly.
- Evidence observatory should focus on facts, links, health telemetry, and collaboration evidence. Demand deconstruction produces readiness, estimates, subtasks, and schedule inputs, so it belongs in the schedule governance page.
- The red-zone refresh flicker is caused by agenda data loading before config-driven core-member filtering settles; lower-case branch slices such as `revert-4` and `with-5` can pass the default in-memory member list for one paint.
- The red-zone decision grid should only render stable Jira issue keys. Git branch names and internal shadow-task prefixes can still exist in telemetry, but they should not appear as story cards.
- Project health telemetry should answer "which project needs human intervention now" before showing raw score math. The primary surface now ranks red/yellow projects ahead of green projects and exposes the top exception as the command focus.
- The four health dimensions map naturally to four manual intervention directions: schedule truth confirmation, evidence-chain repair, ownership/load mediation, and quality/regression triage.
- The first health-telemetry optimization can stay frontend-only because `/api/projects/scores` already provides enough SH/EQ/CE/SI inputs to derive health level, weakest dimension, evidence gap, and decision question.
- The detail modal is more useful as an intervention dossier than a metric explanation page: it now opens with urgency, weakest dimension, evidence gap, and a human action before showing formula details.
- Project avatars and priority cards should carry the same red/yellow/green state language as PHDI. This avoids another decorative identity treatment and keeps the page visually tied to operational severity.
- Project health telemetry remains conceptually useful only if it has a concrete operator workflow. Until that workflow is proven, the evidence observatory should not spend first-screen attention on the panel.
- Hiding the project telemetry mount is safer than deleting the component because the scoring and diagnosis work can either be reworked into a sharper workflow or removed in a later cleanup with less urgency.
- KPI 研发绩效大盘 should treat daily and weekly reports as first-class review grains, not as one option in a generic time-range selector.
- The existing KPI API already exposes enough fields for a first explainable score: completed tasks, demands, bugs, MR count, review count, cycle days, active overdue, and overdue completed.
- A member matrix is more useful than ranking cards for this workflow because operators need to compare task/Bug volume, code evidence, cycle time, and risk side by side.
- The score should be shown with its component breakdown so it remains an auditable evaluation aid instead of an opaque performance verdict.
- KPI aggregation previously ignored the coreMember boundary, so non-core assignees could still affect summary totals and report evidence even if other dashboards filtered them.
- `jira.sync_users` is the clearest coreMember source; custom JQL `assignee in (...)` remains a useful fallback for deployments that encode the team in JQL only.
- Core member filtering must happen before KPI aggregation, otherwise user rows may be hidden while totals, departments, risks, or evidence still leak non-core work.
- Core member matching needs aliases because Jira usernames, WellOS/local display names, and email prefixes can refer to the same person.
- KPI click jumping came from two UI behaviors: report-mode refreshes could replace the whole mounted dashboard with the full loading state, and active button feedback used `transform: scale(...)`.
- The remaining daily/weekly jump came from a second layer of layout shift: the sync indicator was still conditionally inserted into the header, and the report/member sections could resize substantially when daily and weekly data had different row counts.
- A useful KPI second layer should diagnose team shape, not just add more totals: delivery concentration, evidence coverage, risk load, review pressure, Bug share, and score spread explain whether the score is healthy.
- KPI breadth should show distribution across work type, score bands, departments, and cycle time so managers can compare team balance without manually reading every row.
- Execution tracking rows already knew their parent demand through `task_group_id`, but owner/status/result calculations ignored the parent Jira Task demand facts.
- In this deployment Jira Task is the demand主线, so its current Jira assignee/status must drive execution tracking display for bound rows; child execution task assignee remains useful only as secondary execution evidence.
- Filtering execution rows by assignee before DTO construction can lose rows after Jira reassignment, because SQL only sees the stale child execution assignee.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Add a coded prototype before full UI migration | The current pages are business-critical and large; a prototype can validate IA, density, visual language, and component vocabulary without breaking existing workflows. |
| Gate the prototype as a preview surface instead of replacing the current dashboard | The user asked to start implementation, but previous broad cockpit surfaces were rejected; a preview lets us evaluate the new management-console direction safely. |
| Use restrained glass as a material layer, not decorative glassmorphism everywhere | Product UI needs scannability and contrast; acrylic should separate shell, panels, and overlays rather than turn every card into a glossy object. |
| Keep the prototype focused on exception, schedule, evidence, and override workflows | These map to the strongest-brain closed loop and avoid reviving generic queues or low-value health dashboards. |
| Refactor the prototype into local components before touching production pages | The first prototype's weakness was structure as much as visuals; Shell, Metric, Badge, Table, and Inspector components make the desired admin vocabulary explicit and reusable. |
| Make the command table the primary prototype surface | Operators need to compare risk, owner, stage, due date, evidence, and next action in one scan; card-first layouts are too slow for this workflow. |
| Keep the right-side drawer as mediation preflight rather than generic detail | The ultimate value is shortening human intervention, so the selected item should expose reason, close condition, evidence state, route, audit trail, and direct actions. |
| Add a first-screen Theatre component for the bolder prototype pass | The user asked for beauty and boldness; the Theatre gives the product UI a memorable visual center while still carrying selected issue facts and actions. |
| Raise spectacle through substrate and data-state visualization, not through a marketing hero | The page is still an admin console, so boldness should come from acrylic depth, ambient grid, status signals, and operational hierarchy rather than decorative copy or unrelated imagery. |
| Locate actual symbols with `rg` before editing | Current repo structure differs from the plan. |
| Use `find` for ignored server files | `rg --files` missed backend files that are relevant to this task. |
| Redact login debug output | Keeps observability without violating token/secret handling. |
| Preserve task metadata during import/webhook updates | Prevents AI demand-task relationships and due dates from being erased by later state transitions. |
| Use a current-user profile endpoint | Fixes stale or missing frontend department display after refresh without granting admin user-list access. |
| Reuse selected demand task groups | Deepens and extends the AI decomposition graph around the demand rather than creating separate groups per run. |
| Generate `brain-{demand_id}` when a selected demand has no task group | Scheduling and AI deconstruction need a durable relationship before repo-level tasks exist. |
| Add readonly profile in the avatar dropdown instead of editable personal settings | User name, avatar, and department are OS-owned values, so this app should display and refresh them near the user trigger, not mutate them or consume a main tab. |
| Stop showing Git repo as default product-demand metadata | A repository belongs to mapped implementation tasks, not to the initial product-demand card before AI/repo mapping. |
| Use local access snapshots plus cached password verification during WellOS maintenance fallback | This preserves availability for known users without allowing unknown accounts or wrong passwords to bypass OS authentication. |
| Mark degraded JWTs and audit degraded logins | Operators need a visible trail that the session came from maintenance fallback rather than normal OS auth. |
| Keep local development auth separate from production maintenance fallback | Developers can continue debugging during OS downtime without weakening deployed authentication. |
| Keep GitLab webhook ensure/status under `config:write` | It mutates external GitLab project configuration and uses integration credentials, matching the existing GitLab project-selection API boundary. |
| Match existing hooks by normalized webhook URL | A hook for the same callback should be updated in place so token/events drift is repaired without duplicate callbacks. |
| Do not include GitLab response bodies in per-project errors | Prevents accidental token/secret exposure while still reporting enough status for operators to diagnose. |
| Use self-agents for disjoint implementation tracks | The requested work naturally splits across integration, KPI/reporting, and AI deconstruction modules with minimal file overlap. |
| Prefer structured previews before LLM-generated prose for reports | The system should make work facts auditable before adding summarization polish. |
| Preserve existing dark cockpit styling for control polish | The requested fixes are usability refinements inside an internal dashboard, not a redesign or marketing-page treatment. |
| Use custom button/dropdown shells for report and Override controls | This keeps the visual system consistent and avoids platform-native select/date surfaces leaking into the UI. |
| Persist every imported deconstruction run as an archive | Estimate accuracy is a longitudinal problem; retaining only current task fields would lose the original model rationale and overall estimate. |
| Normalize missing subtask estimates from overall estimate and difficulty weights | LLM outputs can omit optional fields; deterministic fallback keeps downstream due dates, UI display, and archives usable. |
| Configure AI project context instead of relying on implicit model knowledge | Architecture, workflow, implemented capabilities, and estimation rules must be first-class prompt inputs to reduce estimate drift. |
| Make hours the primary deconstruction estimate unit | Operators tune task effort in hours; day values remain derived compatibility data for due-date and archive logic. |
| Store long domain capability catalogs as archived context documents and inject compact summaries into config | Long feature inventories are better versioned/audited in docs, while runtime prompt fields should stay concise enough for stable AI behavior. |
| Remove the module-image AI context center rather than renaming it | Its upload/version/toggle model made the context center behave like a targeted feature; the remaining generic AI context fields better match the intended system-design and workflow calibration purpose. |
| Build AI context as a structured fact registry | Demand deconstruction needs typed facts, scope, freshness, confidence, token counts, hashes, and persisted pack assembly to keep prompts compact and auditable. |
| Keep legacy AI context fields as fallback only | Existing deployments should not lose prompt context, but future context should be curated through facts and packs rather than a broad text configuration center. |
| Layer policy authorization over existing RBAC | This preserves current group-permission compatibility while enabling subject/action/resource/scope decisions and explicit deny overrides. |
| Log denied and high-risk authorization decisions | Permission problems should be explainable without guessing which group, policy, scope, or missing permission caused the result. |
| Source permission tree data from the backend permission catalog | Permission growth should be driven by seeded `permissions` data rather than a hard-coded frontend matrix. |
| Keep AI context facts in their own Settings tab | The context registry is an input layer for demand deconstruction, while AI engine config is provider/model connectivity. |
| Replace policy raw selects with guided controls | Admins need fewer fragile fields and more visible policy intent before saving allow/deny rules. |
| Map Jira Task-like issue types to demands | The team's Jira Task items are real requirements that need scheduling, iteration planning, and delivery tracking. |
| Keep user-info refresh best-effort after successful auth | A temporary user-info failure should not reject a valid WellOS login, but successful responses must refresh DB, JWT, and frontend payload profile fields. |
| Use row-level `scheduled` for schedule table truth | A boolean contract avoids frontends and future pages reinterpreting `backlog` as “已排期” without branch and due-date evidence. |
| Keep AI schedule estimation optional | AI configuration failures should not block a normal schedule edit; the estimate is useful context and persisted telemetry, not a mandatory gate. |
| Preserve local manual assignee overrides from Jira sync for 24 hours | A decision-panel reassignment should visibly take effect immediately, while still allowing Jira to become authoritative again after the handoff window if no matching Jira update arrives. |
| End local assignee override protection when Jira reports `done` | Completion is a stronger Jira fact than an active handoff window; the execution tracking board must not keep a stale local owner after Jira has finished and reassigned the issue. |
| Keep recent completed Jira issues in correction sync | The production custom JQL excludes `done`, so completed issues need a bounded key-based keep-alive window to capture post-completion owner changes and reopen facts. |
| Expose Jira link metadata through a non-secret authenticated endpoint | Demand/task users need to open Jira issues, but should not require full `config:read` access or receive integration tokens. |
| Build the strongest-brain queue as an aggregation layer over schedule and execution read models | This lands a useful v1 loop without duplicating risk logic already covered by `/api/schedule` and `/api/execution/tasks`. |
| Use deterministic AI intent/summary fallback first | The conversation surface must remain explainable and usable when AI providers are disabled, misconfigured, or slow. |
| Fetch Jira link config before full config in the decision cockpit | Decision users need issue links, but should not depend on `config:read` to render the queue. |
| Implement risk calendar as additive endpoint | `/api/schedule` is already consumed by the table and tests; a new `/api/schedule/risk-calendar` avoids breaking existing filters and virtualized row rendering. |
| Keep risk calendar as a read-model projection | Phase 4 needs visible schedule governance, but risk rules should still come from the existing schedule resolver until a unified event ledger lands. |
| Let the frontend fall back to current schedule rows | Operators should still see a useful risk calendar if the new endpoint is temporarily unavailable or deployed behind the frontend. |
| Treat the decision queue as a cockpit projection rather than a new backend contract | The fastest useful fix is to reveal existing queue source, decision kind, idle cost, and entry state in the frontend without changing `/api/strongest-brain/decision-queue`. |
| Keep non-Agenda queue items visible but honest about their entry | Schedule/execution/context items may not be actionable in the old intervention form, so the UI now labels them as source-located work instead of pretending every card can be handled by the same control. |
| Remove the strongest-brain decision queue only from the frontend dashboard for now | The user rejected the visible surface; leaving backend APIs intact avoids unnecessary churn to Phase 22 contracts/tests while removing the useless UI and polling. |
| Worker B Phase 26 uses `DeconstructArchive + ContextPack` as the AI trace MVP source of truth | The existing schema already stores archive output, input snapshot, confidence, context pack metadata, prompt template version, and selected pack items, so a read model avoids new migrations during the parallel rollout. |
| Worker B Phase 26 leaves trace/clarification route registration to the main agent | `server.go` is being edited by another worker; helper handlers are implemented but not registered to avoid route ownership conflicts. |
| Worker B Phase 26 derives missing-question tiers deterministically from deconstruction analysis | `missing_info` becomes must-answer, dependencies become can-defer, and meeting questions are classified by readiness/keywords so tests stay stable without external AI calls. |
| Implement the strongest-brain "all phases" request as one evidence cockpit plus focused read APIs | The pain points are connected by the same proof chain, so the first landing slice should consolidate status, decisions, readiness, schedule risk, override audit, traceability, and permission explanation rather than scatter new pages. |
| Keep the old decision queue hidden | The user explicitly rejected that visible surface; the new cockpit surfaces only exception and decision prompts with evidence and entry points. |
| Keep AI trace/readiness deterministic for MVP tests | Deterministic read models make replay, audit, and missing-question behavior stable without depending on external model calls. |
| Remove the visible delivery cockpit for now | The user still does not see its existence value; continuing to defend the surface would add UI noise, so only the reusable backend foundations stay. |
| Split Phase 2 and Phase 3 into independent APIs | Exception handling and weekly decisions have clearer product contracts than a broad cockpit, and future UI can be purpose-built around each phase. |
| Reuse one strongest-brain decision snapshot for queue, exceptions, and weekly decisions | Shared evidence/risk logic prevents three endpoints from drifting on severity, owners, missing links, and evidence completeness. |
| Move demand deconstruction into schedule governance | The deconstruction output feeds scope clarity, estimate confidence, missing questions, and schedule readiness rather than evidence observation. |
| Make the risk calendar filter row count-first | Operators decide by severity/count first, so all risk chips expose totals and switch the calendar view directly. |
| Gate red-zone agenda rendering on config readiness | Prevents the first paint from using the broad default member list before `/api/config` has had a chance to narrow the visible scope. |
| Require stable Jira issue keys for red-zone cards | Lower-case Git branch slices and internal generated task IDs are telemetry evidence, not decision-board story cards. |
| Treat project health telemetry as an exception triage surface | The panel should reduce daily progress chasing by ranking projects that need human action, not by making operators compare raw score columns. |
| Derive manual intervention from the weakest health dimension | This gives every red/yellow project one concrete action direction while preserving the existing scoring API and avoiding premature backend expansion. |
| Keep the health telemetry refactor frontend-only for the first slice | Existing score fields already support the intervention model; backend changes should wait until interventions need persistence, ownership assignment, audit, or rollback. |
| Remove decorative identity gradients from project health rows | Severity color carries operational meaning here, so project identity should not compete with red/yellow/green health status. |
| Hide the project telemetry panel until its workflow value is proven | A visible dashboard module should either drive an operator action or stay out of the way; the evidence observatory keeps the task/code evidence surface while telemetry remains dormant. |
| Preserve the telemetry component instead of deleting it immediately | The user asked to hide it temporarily, so the safer move is to unmount the surface and leave the component available for redesign or later cleanup. |
| Make KPI daily/weekly mode the primary control | The stated product goal is clear report granularity, so the UI should start from review cadence instead of raw period ranges. |
| Derive the KPI score client-side for this slice | Existing backend fields are sufficient for a transparent first model; backend persistence can wait until the score formula needs audit history, calibration, or permissions. |
| Use output, evidence, flow, and risk as the score dimensions | These dimensions map cleanly to the available data and avoid evaluating people from a single task-count metric. |
| Replace member ranking cards with a matrix table | Performance review needs scan-friendly comparison across multiple dimensions, not variable-height decorative cards. |
| Keep department mix separate from personal score | Department totals explain delivery distribution, but should not silently add or subtract from an individual's score. |
| Apply coreMember filtering inside KPI backend endpoints | Filtering only in the UI would leave summary, department, risk, and evidence data polluted by non-core assignees. |
| Disable KPI coreMember filtering when no member source is configured | Empty local/test deployments should preserve existing all-data behavior instead of returning an apparently broken empty dashboard. |
| Resolve core members by display name, username, email prefix, and first token | Jira and local profile identity fields vary, so strict string equality would incorrectly remove legitimate core members. |
| Reuse custom JQL assignees only as fallback | Explicit `jira.sync_users` is a clearer team boundary and matches existing frontend precedence. |
| Enforce coreMember visibility at API boundaries | Non-coreMember data should not be returned and then hidden in the UI; task, schedule, execution, and options APIs should apply the same configured member boundary before encoding. |
| Keep KPI as the only place that may use wider context internally | KPI can later use broader data for calculation context, but response evidence and visible user rows must remain constrained to included coreMember users. |
| Keep KPI mounted during post-load refresh | Background data refresh should not resize or remount the dashboard; inline sync feedback preserves spatial stability. |
| Replace click scale animation with color/border feedback | Internal dashboard controls should acknowledge clicks without changing element dimensions or causing perceived page jump. |
| Reserve fixed KPI refresh and matrix geometry | Daily/weekly switching changes data volume, so sync feedback, dynamic copy, report preview, and member matrix need stable slots or internal scrolling. |
| Add KPI depth and breadth as diagnostic panels | The next useful layer is explanatory structure around concentration, evidence, risk, review, quality, and distribution, not another decorative ranking card. |
| Let parent Jira Task facts drive bound execution rows | The execution tracking table is labeled as Jira Task result tracking, so the displayed owner/status should follow the bound Jira demand主线 rather than stale child execution fields. |
| Preserve child execution assignee separately | The old execution assignee may explain historical code activity, but it should not be mistaken for the current Jira owner. |
| Apply execution assignee/search filters after DTO construction | Effective owner/search fields include parent Jira facts, so pre-DT0 SQL filtering against child assignee can hide correctly reassigned rows. |
| Let Jira completion override local assignee protection | The sync worker's 24-hour manual override window can be useful during active triage, but it must stop once Jira returns a completed status. |
| Treat FZ-2220 as a completed-fact refresh gap, not a completion-flow gap | The local row is already `done`; the stale field is assignee, caused by completed issues exiting keep-alive while the main Jira JQL excludes `done`. |
| Redact secondary execution assignees that are non-core | A core-owned Jira row can still have an old child execution assignee; that helper field must not leak a non-core name in the tracking table. |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Browser DOM snapshot check failed while verifying the local app | Switched to a lightweight page-text read; confirmed the Vite app is reachable but the in-app browser is currently on the login screen, so visual prototype verification requires logging in. |
| Browser verification of the componentized prototype was blocked by login | The app loaded at `http://127.0.0.1:5174/`, but the in-app browser stayed on the login screen and browser security policy rejected `javascript:` navigation for setting local preview auth. |
| Phase 38 `pnpm check` still fails outside the prototype files | Remaining errors are existing non-prototype issues in `TaskKanban.svelte` and `ProjectConfig.svelte`; production build and diff hygiene pass. |
| Phase 39 browser visual verification still needs a real login | The preview is behind the existing authenticated app shell; build-level validation passes, but visual review of `UI 原型` must happen after login. |
| Sandboxed Vite dev server could not bind `127.0.0.1:5173` | Re-ran the same dev server command with approved local bind permissions; Vite started on `http://127.0.0.1:5174/` because 5173 was already in use. |
| Initial discovery suggested missing backend server | Confirmed the files exist via `find`; proceed with direct reads. |
| Frontend build failed on `TaskKanban.svelte` const placement | Removed the duplicate const from inside the card markup. |
| Svelte build reports existing a11y warnings | Build succeeds; warnings remain outside this plan's blocking scope. |
| Earlier session lacked `taste-skill` availability | Wrote the required rule and recorded `frontend-design` as the fallback for future unavailable sessions; latest demand UI fixes used the requested `design-taste-frontend` skill. |
| `apply_patch` failed due formatted context drift | Re-read snippets, patched with smaller context, and logged the tooling lesson. |
| `svelte-check` still fails after this UI polish | Remaining type errors are pre-existing and outside the touched files: `TaskKanban` nullable values, `Button size` props in config/settings panels, `Switch helperText`, and `App.svelte` header indexing. Production build passes. |
| Full server test suite cannot run inside the current sandbox | Existing tests using `httptest.NewServer` fail with `listen tcp6 [::1]:0: bind: operation not permitted`; targeted non-listener permission catalog test passes. |
| `svelte-check` still fails after Settings/RBAC polish | Remaining errors are pre-existing in `TaskKanban.svelte` and `App.svelte`; touched Settings/AI/Button files no longer contribute errors, and production build passes. |
| Phase 22 full server package test cannot be re-run outside the sandbox in this session | The required escalation was rejected due account usage limit; targeted Phase 22 Go tests, frontend build, and `git diff --check` passed. |
| Phase 23 subagents did not return final reports before timeout | Closed all three agents, integrated the shared-worktree frontend partial, and completed backend/API/tests in the main thread. |
| Decision queue display meaning was unclear to the user | Added a current focus panel plus per-card source, decision type, no-action cost, and handling-entry labels. |
| The added decision queue meaning still did not satisfy the user's usefulness threshold | Removed the queue surface and its frontend polling/state/style code. |
| Worker-owned helper names conflicted during integration | Kept the richer shared strongest-brain helpers in `strongest_brain_handlers.go`, renamed delivery-only helpers, and removed the duplicate temporary helper file. |
| Full internal Go validation failed inside the sandbox because `httptest` could not bind loopback | Reran the same `go test ./internal/... -count=1` with approved non-sandbox execution and it passed. |
| `task_status.md` and `well-ambient.db` changed during validation/runtime side effects | Kept those generated/runtime changes out of the Phase 26 implementation commit instead of mixing operational data with source changes. |
| The delivery cockpit UI did not communicate its value even after adding purpose text and action strips | Removed the frontend component and shifted implementation effort to smaller phase APIs that can be evaluated independently. |
| Phase 32 `pnpm check` failed outside the KPI component | Treated the 3 existing type errors in `TaskKanban.svelte` and `ProjectConfig.svelte` as validation blockers outside this KPI refactor; production build and diff hygiene passed. |
| KPI report preview leaked non-core evidence when no user filter was selected | Moved coreMember filtering ahead of performance/report builders so evidence, risks, meeting focus, and overview all consume the same filtered task set. |
| KPI page jumped on report-mode clicks | Preserved the mounted dashboard after first load, added inline sync state, blocked duplicate refresh clicks, and removed active-state scale transforms. |
| KPI still jumped after the first fix | Kept the sync indicator mounted with hidden visibility, fixed dynamic text heights, gave report/member panels stable minimum heights, and capped the member matrix with internal scroll plus sticky header. |
| Jira execution tracking showed stale负责人/status | Added effective Jira assignee/status projection, secondary execution-assignee display, post-DTO filtering, and regression coverage for reassigned completed parent Jira Tasks. |
| Jira sync could preserve a stale owner after completion | Passed Jira mapped status into the local override guard and added a completed-status regression so `done` issues refresh from Jira immediately. |
| FZ-2220 stayed on an old assignee after completion | Changed keep-alive from active-only to active plus recently completed Jira keys, with tests for the 14-day completed correction window. |
| Non-coreMember rows could still be returned outside KPI | Added a shared visibility filter, removed external collaboration dropdown entries, and added tests proving task, schedule, and execution APIs suppress non-core data. |

## Resources
- Source plan: `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`

## 2026-07-11 Flow Board and AI Deconstruction Findings

- `.flow-dashboard .lane-cards` already declares `overflow-y: auto`, but the board and lanes override their shared height token to `auto`; without a bounded ancestor height, the lane never becomes a scroll container and is clipped by the shell.
- The gray strip between the filter card and lanes is the light page background showing through a transparent, gapped flow root. A single subtle white flow surface can connect the regions without introducing another heavy card.
- The demand-detail modal closes from an unconditional backdrop click handler. A target/currentTarget boundary is safer for nested async controls and prevents AI actions from dismissing detail state.
- `DemandDeliveryControl.svelte` uses invalid `font: ... inherit` shorthand for fields and buttons, so browsers discard the declaration and inherit inconsistent typography.
- `Deconstructor.svelte` is mounted by `App.svelte` only in the flow-board view. The product model should be one demand-intelligence workbench launched with seeded text from capture, schedule, or flow context.

## 2026-07-11 Phase 48 Correction Findings

- A zero-height flex child only grows reliably when the complete ancestor chain has a definite height. The authenticated flow shell did not satisfy that condition, so the safe contract is a viewport-clamped board height plus internal lane scrolling.
- “可关联需求” is not a passive metric: selecting one reuses that demand's existing task group, while the unbound option creates independent AI tasks. The selector therefore belongs before generation, not below the result fold.
- The full deconstructor includes file import, intent conversation, prompt disclosure, editable evidence table, and deep analysis. A contextual modal needs only association, source text, generation state, compact task suggestions, and the final synchronization checks.
- The settings project list is served by `/api/projects/config`; mixing `/api/demands/options`, telemetry scores, and historical task data created extra values. New-demand project choices now consume only the same filtered project-config response.
- A side-by-side AI companion is viable above 1500px; narrower viewports need an overlay fallback to preserve usable field widths.

## 2026-07-11 Phase 49 Browser Findings

- At 2048x925 the new-demand dialog measured `680x599.27` and the AI companion measured `620x597`; their vertical edges differ by about 2px and the horizontal gap is 16px.
- The combined dialog group center is exactly `1024px`, matching the viewport center, and neither the document nor the AI panel has horizontal overflow.
- Both surfaces resolve to the same light-console surface color `rgba(255,255,255,0.94)`, border `rgba(255,255,255,0.76)`, and primary text color `rgb(41,56,71)`.
- The compact AI panel visibly separates “需求标题” from “需求内容 / 规格说明”; the deconstruction request serializes them as `【需求标题】` and `【需求内容】` sections.
- At the default 1292x903 viewport the responsive fallback presents the AI panel as a centered overlay with no horizontal overflow, preserving readable field widths.

## 2026-07-11 Phase 51 Demand Association Search Findings

- The association selector already had a bounded option menu, so the lowest-risk interaction is a sticky search row inside that menu rather than a second modal or global search surface.
- Filtering now matches demand ID, title, owner, repository, and project name without mutating the source demand list.
- Authenticated browser validation filtered 101 demands to the single HR-4202 result, then selected it and confirmed the dropdown closed with the association value retained.
- The open-menu state had no horizontal overflow and the browser console reported no errors.
- Visual evidence is saved at `output/ui-validation-demand-association-search.png`.
# Health Diagnosis Intervention Modal Calibration

- `ProjectHealthTelemetry.svelte` owned both the modal structure and a large legacy high-contrast style block; a final `.health-diagnosis-modal` scope was required to avoid changing the surrounding project-health workbench.
- The visual mismatch came from a full pink intervention surface, saturated score/diagnostic text, four competing metric colors, lavender badges, and repeated inset-card borders.
- The revised contract uses a white workbench surface, restrained neutral sections, muted semantic rails/chips, one consistent type hierarchy, and a body-owned scrollbar.
- Browser harness verification at 1280×720 measured a 920×680 modal with 20px vertical safety space, no document horizontal overflow, two columns of approximately 455px/396px, and `overflow-y: auto` confined to the modal body (`610px` client / `668px` scroll height).
- The authenticated project-health route remains login-gated in preview. The current in-app browser facade also cannot resize the viewport, so the 860px/620px single-column rules were verified from compiled/source CSS rather than reported as live narrow-screen testing.
# Task Panels Height And Internal Scroll Alignment

- Both reported surfaces are in `web/src/components/TaskKanban.svelte`.
- The personnel grid explicitly uses `align-items: start`, so the table card and inspector size independently; neither has a shared height contract.
- The Jira Task status workbench also uses `align-items: start`. Its left section has a toolbar plus table shell, but the shell has no bounded height or overflow ownership, so row count expands the page while the inspector remains content-height/sticky.
- The correct desktop contract is equal-height grid tracks, column cards/panels as `min-height: 0` flex/grid containers, and overflow on the table shell only. At the existing 1180px collapse breakpoint, fixed heights should be released.
- The shared admin table already provides sticky headers and `overflow: auto`, so the fix only needs to bound the shell through its parent grid rather than duplicate table behavior.
- Browser verification at 1280×720 measured both status panels at 480px with a 0px height delta. The Jira Task shell measured 367px client height versus 2259px scroll height and retained `overflow-y: auto`.
- The personnel table and inspector also both measured 480px with a 0px height delta. Its table shell measured 401px client height versus 921px scroll height; no document-level horizontal overflow was introduced.
# Compact Task Rows, Inspector Pills, And Filter Layering

- The status table repeats project, parent Demand, repository, active-day, and evidence-progress metadata below the primary row content; these secondary elements are the source of the excessive row height.
- Task type is currently plain `Bug`/`Task` text in the table and `Bug 缺陷`/`Task 任务` in the generic inspector fact grid.
- The inspector facts are rendered as five independent two-line cards. They can be converted safely to a wrapping capsule list because the underlying `AdminInspectorRecord` contract does not need to change.
- The project/owner dropdowns live inside `.phase41-filter-surface`; its stacking context is not raised above the metric grid, while the absolute menu itself only has a local z-index.
- `.wa-admin-card` uses `isolation: isolate`, so raising only `.custom-select-options` cannot place it above later sibling cards; the filter surface itself must be positioned and assigned the higher z-index.
- Browser verification measured compact task rows at about 55px and confirmed there are no title, owner, or evidence secondary-line nodes in the status rows.
- All five inspector facts render in a wrapping flex capsule group; the sampled capsule height is 30px. The task-type icon is 24px, has no visible text, and retains `aria-label="缺陷"` or `aria-label="任务"`.
- The open project menu overlaps the following metric cards while remaining the top hit-tested element (`custom-option`), proving the filter surface stacking fix works. No document-level horizontal overflow was introduced.

# 2026-07-12 Phase 52 Screenshot Findings

- The schedule inspector repeats eight label/value facts in a two-column ruled grid, consuming most of the panel height before progress and acceptance criteria.
- Status, owner, dates, project, goal, stage, estimate, and task-group identifiers are secondary facts and can become wrapping capsules; title, progress, and acceptance criteria should remain the primary reading path.
- The shell global search is visually present but the user reports no effective search behavior, so the implementation must trace keyboard submission, result state, and route navigation rather than only restyle the input.
- The login email prefix and fixed domain are visually split by a hard inner boundary; they should read as one continuous field group with a shared focus ring and softer separator.
- `FunctionalAdminShell.svelte` owns the global search, but its input is currently unbound and has no submit handler, result state, API request, or navigation behavior.
- `App.svelte` owns the login prefix field and already wraps prefix and suffix together; the remaining issue is the suffix treatment and autofill/background seam, not form semantics.
- `DemandKanban.svelte` builds the schedule inspector as eight generic facts and renders them in a two-column ruled grid; the data contract can stay unchanged while the render becomes a wrapping capsule list.
- Authenticated search for `ZPU-2769` returned one demand result with title, owner, status, and route; selecting it opened the schedule workbench and produced no console errors.
- At `2133x902`, the schedule inspector renders all eight facts as wrapping 30px pills in a 67px block with no document horizontal overflow.
- At `1280x720`, the unauthenticated login preview renders the combined email control at 49px high with a transparent prefix input and a 32px rounded suffix capsule; the screenshot shows one continuous field surface without the former hard seam.
- A non-first-row regression with `HR-4202` confirmed exact focus handoff: the schedule filter, selected table row, and inspector all moved to `HR-4202`, with no browser console errors.
# 2026-07-12 - Demand detail AI workbench and executable spec draft

- `DemandDeliveryControl.svelte` can create/update/freeze a demand spec but the server exposes no draft-withdrawal route; a real undo requires a backend transaction, not UI-only state clearing.
- Draft creation also creates a `ReviewContract`, so withdrawal must delete the draft contract and spec together and reject non-draft specs.
- `DemandKanban.svelte` opens the new-demand AI panel with `presentation='companion'`, but demand-detail and schedule call the same function with its default standalone modal presentation.
- Host close handlers currently clear only their own state, leaving `showDeconstructorModal` untouched; this is the orphaned right-panel bug.
- The current companion geometry is hard-coded for the 680px create-demand host. Schedule is 720px and detail is 760px, so a shared host-width variable is needed for a centered pair.
- `DemandSpecVersion` already persists business rules, main/exception flows, permission rules, data/API/UI impact, dependencies, risks, acceptance criteria, test plan, mapped repositories, and tasks.
- `DeconstructAnalysis` currently generates only completeness, estimates, missing info, risks, dependencies, acceptance criteria, schedule notes, and meeting questions. The reserved implementation fields are never populated, which weakens downstream code generation.
- The safe execution boundary is already appropriate: a frozen spec feeds code generation, updates are restricted to supplied source snapshots, delete actions are rejected, and CI plus human review remain required.
- Authenticated browser validation exposed an adjacent detail-to-schedule bug: the click handler called `closeDemandDetails()` before reading `detailDemand`, so it always cleared the reference and failed to open schedule. Capture the demand first, then switch panels.

# 2026-07-12 - Streamed AI specification draft and Markdown preview

- The reported flow waits synchronously for a slow LLM result, so an intermediary can time out even when the provider eventually succeeds. The replacement must emit bytes early and periodically, then send a distinct persisted final event.
- The supplied screenshot shows one long modal composed of many uniform textarea fields. This flattens headings, lists, code, emphasis, and tables into body-text boxes, creates multiple nested scrollbars, and weakens the reading path.
- The result surface should have one document reading column with Markdown typography, a compact status/header rail, and explicit generating/complete/error states. Editing can remain a separate intentional mode rather than making every field look permanently editable.
- Visual baseline: calm white workbench, teal accent used sparingly, neutral dividers, restrained status chips, one body-owned scrollbar, and responsive single-column behavior.
- The existing development-auth path required both an explicit environment flag and a loopback request, but still contacted WellOS first. Short-circuiting only under those two guards prevents external authentication traffic during local browser validation without changing production behavior.
- Authenticated DG-319 validation at 1280×720 rendered 14 semantic Markdown headings and 6 lists with zero result textareas, no document horizontal overflow, and one modal-owned vertical scrollbar (`3210px` scroll height / `670px` client height).
- The explicit Preview/Edit switch preserves the readable Markdown result as the default while keeping all governed fields available for manual correction and save.
- At 760×900 the detail modal measured 732px wide with 14px side safety space; the Markdown article measured 611px and its scroll/client widths matched, while the document remained exactly 760px wide.
- Browser screenshot review confirmed the calm white/cyan workbench palette, clear heading rhythm, restrained status chips, responsive stacked toolbar, and no competing nested result scrollbars. Browser console errors: 0.
- Validation screenshots: `output/ui-validation-ai-spec-markdown-desktop.png`, `output/ui-validation-ai-spec-markdown-body.png`, `output/ui-validation-ai-spec-edit-mode.png`, and `output/ui-validation-ai-spec-markdown-760.png`.

# 2026-07-12 - Impeccable review gate and shared Markdown workbench

- The remaining card nesting was structural, not a Markdown renderer problem: `delivery-control > delivery-stage > markdown-preview/spec-editor` retained three rounded, bordered surfaces after the previous content refactor.
- The project already has `web/src/components/shared`, so the Markdown editor belongs there rather than in a page-local design-system fork.
- Impeccable, taste-skill, and finesse-ui agreed on one legitimate tool boundary, a proven editor engine, flat stage hierarchy, progressive disclosure for governed fields, and authenticated responsive verification.
- `MarkdownWorkbench.svelte` now owns CodeMirror editing, GFM rendering, DOMPurify sanitization, edit/split/preview modes, read-only behavior, line/character status, save shortcut, and streaming presentation.
- The stream preview and persisted specification use the same shared component; the legacy hand-written `web/src/lib/markdown.ts` renderer is deleted.
- Authenticated DG-319 validation measured a 460px desktop editor with 83 lines / 1136 characters, equal 351px split panes, zero nested workbenches, and transparent 0px-radius delivery stages separated only by 1px hairlines.
- At 760x900, split mode expands to 700px with two 282px panes and no clipping; preview mode remains 460px and the workbench, toolbar, and modal all have matching scroll/client widths.
- The Impeccable detector returned `[]`, browser console errors were zero, and no business data was saved or mutated during interaction testing.

# 2026-07-12 - Inline Markdown editing and review contract layout

- Persistent edit/split/preview controls competed with the document even though preview is the dominant state; the demand-spec surface now renders only the Markdown document until direct interaction requests editing.
- `MarkdownWorkbench.svelte` now supports an inline-edit contract: double-click enters CodeMirror, an outside pointer commits changed content, unchanged content exits locally, save failure remains in edit mode, and a 1px keyboard entry becomes visible only on focus.
- Authenticated DG-319 validation changed the source from 1136 to 1137 characters with a trailing space, clicked outside, received `Markdown 草案已临时保存。`, and returned to preview; payload trimming preserved the actual Markdown body.
- The old review contract used three equal columns even though the third column contained three stacked policy fields. The replacement uses reviewer and policy fieldsets in a 1.08/.92 grid, a 24px divider rhythm, compact status/SLA metadata, and right-aligned approval actions.
- Desktop detail width is now one coherent 960px contract instead of competing 640px and 760px declarations; the AI companion host variable uses the same 960px geometry.
- At 1280x720, the detail modal measured 960px, the Markdown workbench 903px, and review columns about 475/404px. At 760x900, the modal remained 732px, the review layout collapsed to one 675px column, and modal/workbench scroll widths matched client widths.
- Targeted Impeccable and layout scans returned `[]`; the broad `DemandKanban.svelte` warnings belong to unrelated legacy regions and were recorded as explicit existing debt.

# 2026-07-12 - Flat review contract and directory-backed reviewers

- The incorrect `Eddie Antigravity` reviewer came from `Jira.SyncUsers` being copied wholesale into every new review contract; that configuration is a sync scope, not an approval decision.
- Reviewer candidates should represent an explicit contract selection. The only defensible automatic candidates are actual implementation-task assignees, with the specification author removed by default.
- Responsible-person controls require a contextual directory even when the reviewer manager lacks global `users:read`; returning the minimal participant projection with the readable review contract keeps the permission boundary aligned with the workflow.
- The page already loads the user directory for common admin roles. Merging it with the contract-scoped directory makes HMR/old-backend development sessions resilient while the shared selector still consumes one de-duplicated adapter shape.
- Authenticated DG-319 review showed 20+ searchable real users with name, department, and email context. Department filtering reduced the list to exact matches, and two candidates remained selected simultaneously without saving.
- The final review surface has one form hierarchy and no nested fieldsets/cards. The Markdown document flows directly into the review section, and the removed raw structured matrix no longer appears in the DOM.
## Responses-first LLM Transport And Browser Streaming

- The pre-existing demand-spec editor already streamed provider SSE into browser NDJSON, but `/api/deconstruct` still buffered a full provider response and both deconstruction consumers used `res.json()`. Streaming was therefore partial, not system-wide.
- Active Chat Completions construction was duplicated across the server client, demand deconstruction, telemetry review, and connection test. One `internal/llm` client now owns protocol normalization, payloads, authentication, synchronous parsing, and SSE parsing.
- OpenAI, Sub2API, and compatible gateways use Responses by default. The persisted `completions` value is a migration input only; explicit native Anthropic uses Claude Messages. Attachment file IDs remain a Responses-only feature and fail explicitly for native Messages.
- The browser protocol remains provider-neutral NDJSON. Demand deconstruction now emits status, provider delta, parse, complete, and error events; UI consumers display progress without rendering incomplete JSON.
- `撤销草案` now shares one right-aligned progression group with `保存契约` and `批准审核契约`, preserving semantic danger styling and confirmation while removing the isolated far-left action zone.
- Authenticated UI validation proved the protocol-choice states. A copied historical database was not safe for repeated full-app validation because background delay-alert processing activated even with external integrations disabled; fixture-only databases are required for future isolated browser runs.

# 2026-07-13 - Configuration center full visual refactor audit

- The requested change is a product-register redesign of the authenticated configuration center, not a new marketing surface. Preserve the Phase 41 light console, real API behavior, permissions, route labels outside the removed Settings KPI entry, and the existing teal accent.
- The current Settings family has one shared `scw-*` workbench for Feishu, Jira, Project, and AI, but GitLab still owns a separate legacy vocabulary and `SettingsPanel.svelte` carries roughly 4,900 lines of accumulated Phase 41/46/49 style overrides. That layered ownership is the primary source of cross-page drift.
- Authenticated 2133x902 inspection covered Feishu, GitLab, and Members. Feishu and GitLab use the same outer header and audit-column concept, but internal headings, status strips, tables, and action alignment differ; Members drops the audit column and exposes the older RBAC table styling.
- The outer page currently stacks breadcrumb, a second title/facts card, then the workbench. This spends vertical space on duplicate identity and makes the configuration content begin too low. The redesign should merge page identity, status, and facts into one compact translucent command header.
- Glass should communicate page chrome and persistent workbench boundaries only. Field rows, tables, and permission branches should remain flat or use hairlines so the interface does not become nested glass cards.
- KPI remains a valid top-level `度量洞察` route, but the `KPI 绩效大盘` Settings child is redundant. Remove it from Settings types, permissions/fallback routing, shell subnavigation, Settings metadata, and Settings rendering without deleting KPI permissions or the top-level KPI page.
- Browser inspection was read-only: no configuration save, health check, rollback, membership mutation, or authorization action was triggered.
- The implemented route registry has 10 Settings sections and no KPI section; App fallback, shell submenu, page metadata, access checks, and Settings rendering now consume that registry instead of maintaining divergent local arrays.
- The unified visual contract is a translucent context header plus one primary workbench and optional audit rail. Inner configuration facts, forms, security branches, and tables use transparent surfaces or hairline boundaries so the explicit glass request does not reintroduce nested-card depth.
- Authenticated validation opened GitLab, Feishu, Jira, Project priority, AI engine, system corpus, members, permission matrix, policy authorization, and audit log. All exposed the expected page region/heading, none exposed a KPI Settings item, and none showed a load failure.
- Screenshot review at 2133x902, 1024x900, 760x900, and 480x900 confirmed aligned outer panels, responsive inspector flow, readable tables, consistent teal status color, and no visible horizontal overflow. The first 480px pass exposed a generic 44px touch rule inflating switches; excluding `role=switch` restored the intended 44x24 control.
- The GitLab edit wizard was entered without modifying fields and reviewed at 760x900; navigating away restored the read-only overview. Browser console errors remained zero and no business mutation was issued.

# 2026-07-13 - Configuration panel alignment follow-up audit

- The authenticated policy page reproduces the user's complaint at 2133x902: template cards sit above a dense two-column editor/explainer, both columns expand full permission-chip inventories, and each column introduces its own inner scrollbar. Density comes from simultaneous exposure rather than task-focused grouping.
- `SettingsPanel.svelte` still mounts `phase41-settings phase46-settings phase49-settings settings-unified`. The legacy selectors carry higher specificity and `!important`, so earlier `stretch`, absolute-position, fixed-height, and overflow rules can beat the new natural-height/sticky contract.
- The three-way review agrees that Settings layout ownership must converge on one final contract: shared grid controls only columns and top baseline; primary content uses natural page height; the version inspector is the only sticky side rail; only version list, diff/code, and explicit wide-table containers own local scrolling.
- System corpus drift is page-internal. `AIConfig.svelte` renders the source library/editor, candidate review, and context preview in one continuous page. The source list has a bounded height while the editor grows naturally, and the unbounded candidate article list can extend the page indefinitely.
- Policy drift is page-internal. Template choices, policy editor, authorization explainer, policy table, and authorization audit are all visible at once. The complete permission vocabulary is expanded twice, and the lower 7-column table is constrained to half width beside another internally scrolling region.
- Impeccable and finesse prefer explicit tabs/progressive disclosure for the corpus and policy task families; taste recommends the lower-risk first slice: fix corpus list/editor stretch, top-align policy editor/explainer, make policy table plus audit full-width stacked, restore semantic allow/deny colors, and remove duplicate inner page identities.
- Shared direction for this follow-up: stop legacy root phase classes from owning geometry; use natural-height outer layout; normalize the two named pages into clear task groups; preserve APIs, permissions, field values/order, mutations, and route labels. The first implementation removed the classes outright, but that exposed hundreds of dormant CSS warnings, so the stable solution keeps compatibility class names while a single `#settings-unified-root` owns live geometry above them.
- The implemented policy surface now defaults to one compact strategy-list task with `策略列表 / 新建策略 / 授权解释 / 授权审计` tabs. At 2133x902 the primary workbench is 337px high, the page needs no vertical scroll, and document horizontal overflow is exactly zero; the previous editor, explainer, table, and 40 audit rows no longer expand simultaneously.
- Policy table and audit history are explicit named scroll regions, action vocabularies are bounded, semantic allow/deny colors are restored, and all choice/switch states expose pressed or checked semantics without changing their mutations.
- Authenticated policy state sweep confirms exactly one `tabpanel` for each of the four tabs. The create form exposes 27 existing actions inside a bounded chooser and 6 pressed-state controls; the explain form exposes 2 pressed states; the 40-row audit is capped at 523px with local scrolling; all four states keep document horizontal overflow at zero.
- Authenticated corpus library validation shows one active task panel and a deliberate 630px / 1029px source-editor split. Both columns start at the same 399px baseline and resolve to the same 743px grid track; the page has zero horizontal overflow and no duplicate internal page title.
- The corpus candidate tab resolves to a compact 171px empty state when no work is pending; populated queues are capped at ten articles per page by component-owned pagination. The preview tab resolves to one 293px task panel with a single textarea, one preview action, and one calm waiting state; neither task adds horizontal overflow or performs a request during validation.
- GitLab desktop validation proves the repaired two-panel contract: both panes start at 260px with a 0px top delta; the 1201px primary pane stays natural height while the sticky audit pane is independently capped at 790px. The first medium-width pass exposed a remaining 1978px audit expansion caused by an unconstrained diff row, so the responsive audit owner is now capped to roughly one viewport and its version/diff regions retain local scrolling.
- Responsive GitLab checks requested 1024, 760, and 480 widths; Chrome reported actual content viewports of 1138, 844, and 533px. At 844px the formerly 1978px audit panel is now exactly 700px, with its 1559px diff content scrolling inside a 281px region. At 533px the primary and audit panels are one column, the audit stays 700px, version rows are bounded, and document horizontal overflow remains zero.
- At the 533px policy pass, every task remains the only visible panel: the action vocabularies scroll inside 219px/179px regions, the 40-row audit scrolls inside 560px, and priority control height is 44px. The pass exposed 34px policy tabs from a legacy important rule; the unified mobile selector now restores all four tabs to 44px.
- The full desktop route sweep found no load failures, no horizontal overflow, and a 0px top delta on GitLab, Feishu, Jira, Project, and AI engine audit rails. It also exposed content-driven 4536px member, 4532px permission-tree, and 5982px security-audit surfaces; these directories are now bounded to a 720px data workbench with sticky table headers and named scrolling regions instead of stretching the entire outer glass panel.
- Post-fix desktop measurements reduce member and audit primary panels to 739px and permission matrix to 934px; their data regions are 614px high and retain the full 4410/4339/5856px datasets via local scrolling. At exact 760 and 480 viewports, all three pages keep zero document overflow; the 480 member/audit tables own their necessary horizontal scroll while the permission tree stays within its 405px region.
- Exact responsive validation covered corpus task panels at 1024/760 and policy task panels at 1024/760/480. Every state mounts exactly one `tabpanel`, mobile controls are 44px, tab strips do not overflow, and every page keeps document horizontal overflow at zero. A fresh authenticated tab reports zero console errors after the null-memberships normalization.

# 2026-07-14 - Governed corpus ingestion and unified review center

- Raw source documents need their own lifecycle, provenance, version, hash, uploader, and ingestion state; a `ContextFact` alone cannot preserve immutable evidence or represent failed extraction.
- LLM extraction is safest when it returns bounded atomic candidates with explicit `source_fact`, `source_rewrite`, or `ai_suggestion` evidence kinds. AI-only supplements and high/global architecture rules must remain outside production context until a separate impact gate completes.
- The final publish endpoint must use the exact persisted candidate that was previewed. Candidate metadata/content is frozen in the impact stage, re-review invalidates prior preview metadata, and the backend rejects publish requests without a recorded impact preview.
- Archiving a raw source must also withdraw its unpublished candidates atomically; otherwise an invalidated source could still publish later. Accepted candidates and their active facts remain untouched.
- The source-first layout resolves the previous mixed corpus page: `资料库` owns raw-document import/version viewing, `统一审核` owns a queue plus one review workbench, manual `ContextFact` entry is an advanced disclosure, and `上下文预览` remains the production verification task.
- Authenticated isolated-browser validation exercised real Markdown import through a mock Responses provider, ordinary acceptance, sensitive/global first approval, impact comparison, final publication, and source publication counts without touching the workspace database.
- Exact 1440, 1024, 760, and 480px checks found zero document or component horizontal overflow. Desktop Markdown comparisons had 0-2px top/height deltas, narrow comparisons stacked in source/current-first order, browser console errors were zero, and both published candidates retained their source-document link.

# 2026-07-14 - Corpus decision feedback correction

- `CorpusCandidateReview.svelte` treated four completed decisions as persistent panel content through one local `success` state. Those messages shifted the review layout even though they required no follow-up action.
- The existing shared `Alert.svelte` already owns the project's success presentation, but the repository had no reusable application-level toast queue or overlay host. `Deconstructor.svelte` contains a page-local bottom-right toast that is not an appropriate shared owner for Settings feedback.
- The correct feedback taxonomy is recovery-based: validation and request failures remain in the review container with `role="alert"`; reject, ordinary publish, impact-stage approval, and final impact publication use a non-blocking `aria-live="polite"` upper-right toast.
- Production and the isolated settings preview require separate host mounts because they are different Svelte entry trees. One shared store keeps the queue and duration contract identical without coupling the candidate component to either shell.
- Browser validation confirmed the exact final message `影响审核已确认，候选已正式发布。` in the upper-right overlay at 16px top with a bounded 420px desktop width, while `.review-message.success` remained absent.
- The 4.5-second timer removed the toast after the bounded 4.8-second check, manual close removed it immediately, and a fixture 503 remained inside the review container as `role="alert"` with no toast and no false empty state.
- At 1440/1024/760/480 widths the notification remained fixed, within the viewport, and caused no document horizontal overflow. The 760/480 layouts use safe side insets and a 44x44 close target; browser console errors and warnings were zero.

# 2026-07-15 - Impact review flow and panel alignment correction

- The apparent “still two-column” problem is nested: the outer queue/detail grid is required for navigation and is explicitly referenced by the height complaint, while the inner `comparison-grid` is the redundant second two-column system. The impact comparison should therefore become a vertical current-context-first sequence without removing the outer queue.
- The height mismatch comes from unrelated sizing rules: only `.candidate-queue-list` had `max-height: min(70vh, 680px)`, while `.review-workbench` remained content-sized. `align-items` cannot align their bottoms when the right side contains two fixed-height Markdown workbenches plus metadata and actions.
- The stable desktop contract is one shared viewport-relative panel height on both outer columns. Queue header and workbench header/action row remain stable; the queue list and workbench body are the named scroll owners. Below 960px the fixed height is removed and the existing stacked flow returns.
- `MarkdownWorkbench` uses the `minHeight` prop as an actual fixed component height and already owns document scrolling. The vertical impact panes are reduced from 420px to 360px to keep each comparison leg legible without recreating an abnormally tall review body.
- The visual change stays inside `CorpusCandidateReview.svelte`; no lifecycle, API, permission, grouping, Markdown value, or toast behavior changes.

# 2026-07-15 - Workspace-bounded toast positioning correction

- The toast overlap is structural: `App.svelte` mounts `ToastHost` above every authentication/shell branch and `ToastHost.svelte` fixes it to viewport coordinates. Changing `top` would only encode one Header height and fail when the maintenance banner or responsive Header changes.
- The Shell already has the correct ownership boundary. `.console-main` orders Header, optional maintenance banner, and `.workspace-frame`; introducing one relative `workspace-stage` after the first two lets the toast inherit the actual remaining content rectangle without measuring Header pixels.
- `.workspace-frame` must remain the sole page scroll owner. The new stage wraps it, establishes `position: relative`, `isolation: isolate`, and `overflow: hidden`; the toast is absolute inside the stage, while the workspace continues to scroll independently.
- Toast pixels need both a queue bound and a geometric bound. The existing maximum of three notices remains, while `max-block-size: calc(100% - 2 * gutter)` prevents long messages from extending below the workspace.
- The global `--wa-layer-toast` token is not defined, so the current fallback 1400 exceeds Header and popover layers. Stage-local containment allows a low local layer and makes escape into Header impossible even if notice content grows.
- The isolated settings preview has a separate Svelte tree and needs the same ownership pattern around `preview-settings-host`; `settings-preview-entry.ts` remains a bootstrap-only file.
- The two isolated Impeccable assessments agreed on component ownership and containment. The subjective audit caught the Header/workspace hierarchy and long-message height risk; the detector returned clean `[]` and separately confirmed that this bug is outside its mechanical rule coverage.
- Browser interaction exposed a preview-only follow-up: placing the host in a natural-height stage bounds it geometrically but lets the whole stage scroll offscreen. The production Shell does not have this problem because its workspace stage occupies the remaining viewport. The preview now mirrors that contract with a fixed-height page, fixed topbar/tabs, and one internally scrolling settings host.
- Final correction rule: feedback scope and visual containment are separate decisions. A store may be application-global while its overlay is workspace-local; mounting both globally is not required and can violate shell hierarchy.

# 2026-07-15 - Phase 72 initial visual and design-system audit

- The attached flow-board detail has three competing vertical owners: the centered overlay document, the modal-level rail, and a second Markdown rail. The primary defect is information architecture and scroll ownership, not merely colors or padding.
- The visible metadata is simple fact content but is presented as six equal nested cards, which flattens hierarchy. A compact definition grid or summary strip can preserve every value with less visual noise.
- The empty shadow-task section takes the same width and visual weight as populated content. It should become a compact status row or a collapsed secondary section until it contains actionable records.
- The Markdown specification currently behaves like a second page embedded in the modal. The detail workbench should own the only vertical scroll, while the Markdown region expands naturally inside it and defaults to the single `即时排版` presentation.
- Product-register color must remain restrained. The supplied ten-color library is a source palette, not a requirement to show ten accents. A stable semantic mapping with teal primary, green success, orange warning, red danger, and navy information protects scanability and state meaning.
- The strongest-brain decision timeline should be treated as a chronological evidence ledger: decision/outcome first, actor and time aligned consistently, supporting evidence available progressively, latest meaningful event visually current, and no decorative zigzag timeline.
- Live ownership is now located: `MarkdownWorkbench.svelte` renders the four mode controls; `Select.svelte` and `MultiSelect.svelte` own scrollable option lists; `DecisionDashboard.svelte` owns the named decision-event timeline; `DemandKanban.svelte` owns the click-through detail modal and composes `DemandDeliveryControl.svelte` for the AI specification.
- The flow-board detail is not a duplicate page: `DemandKanban.svelte` owns task context and actions, while `DemandDeliveryControl.svelte` owns the AI-spec lifecycle. The redesign must preserve that ownership and remove nested scroll through composition rather than copying spec markup into the modal.
- `MarkdownWorkbench.svelte` currently defaults to `mode='preview'` and always renders four mode buttons when its toolbar is enabled. Live callers already use `live`, while readonly evidence explicitly uses `preview`; the safe component contract is a default single live mode plus an opt-in list of additional modes, with an explicit current mode retained for readonly consumers.
- Both shared dropdowns already have the correct scroll behavior. `Select.svelte .select-options` and `MultiSelect.svelte .multi-select-options` use bounded `overflow:auto`; only the visible rails need hiding with cross-browser scrollbar rules. Scrolling, keyboard navigation, overscroll containment, and max-height must stay intact.
- `modern-admin-tokens.css` is the semantic source of truth. The global token values currently use teal `#008f96`, green `#04966f`, amber `#d88700`, red `#dd4b3e`, and blue `#256bd8`. Updating those semantic primitives is safer and more consistent than scattering the supplied hex values through individual components.
- Many component fallbacks still embed the old values. The token update will govern normal application rendering; only touched shared and redesigned components should receive updated fallbacks to avoid expanding this phase into an unrelated whole-repo mechanical rewrite.
- `DecisionDashboard.svelte` places the global decision timeline after the entire dynamically-height-matched table/inspector workbench, so it is visually buried and detached from the selected decision context. Its current nested date column, connector line, marker, time column, kind pill, title, task pill, commit pill, and body create too many equal-weight signals.
- The main decision page already has the correct primary order: metrics, state filters, selection table, selected-item inspector. The timeline should become an evidence ledger associated with the selected record, while an unselected state can show the latest filtered events. This preserves global discoverability without keeping a second full-width dashboard below the main task.
- The demand detail composes the right owners but exposes them in one long flex column. `DemandKanban.svelte` provides header, title, six facts, actions, shadow tasks, then a complete `DemandDeliveryControl.svelte`; the modal and Markdown workbench each scroll. The new structure should keep a sticky summary/action header and one detail-body scroll owner, with the delivery component using an embedded/natural-height presentation.
- `DemandDeliveryControl.svelte` initializes specification Markdown as `preview`, hides its toolbar, and enters edit by double-click. It should keep that lifecycle. The shared Markdown default change must not force this explicit inline-edit consumer into live mode; instead the component should allow the parent’s explicit mode while the toolbar's default visible controls remain restrained.
- The details companion contract is width-coupled through `--ai-host-width` and `companionHostHeight`. Any wider detail workbench must update both host declarations and preserve the existing 16px host/companion gap; below 1500px the companion already becomes a centered overlay, so the base detail width must still fit independently.
- Contrast measurements constrain the palette mapping: `#6ecc54` and `#71e2d1` require dark ink; `#eb5c20` also requires dark ink; `#002fa7`, `#470125`, `#492d22`, and `#0d3a69` support white; `#c8161d` supports white; `#d34947` and `#018b8d` narrowly fail 4.5:1 with white at normal text size. Therefore teal/red library colors should serve accents, borders, icons, or soft surfaces unless paired with a separately verified dark ink or darker semantic strong tone.
- The isolated Impeccable mechanical layout detector returned `[]`, and arbitrary Tailwind spacing/z-index patterns were absent. Its strict CSS number pass found 324 legacy non-scale spacing declarations and 36 explicit z-index values across the seven large target files. This phase will not mechanically normalize entire legacy files; new and directly changed layout values must use existing tokens, and untouched declarations are a deliberate preservation exception.
- Every current `MarkdownWorkbench` consumer already supplies a mode explicitly. Editable corpus surfaces use `live`, manual advanced entry uses `edit`, and evidence/read-only surfaces use `preview`; the delivery spec binds an explicit preview/edit lifecycle. Changing the component default to `live` is forward-safe, while a default single-mode toolbar will render the consumer's explicit current mode when it is not live.
- The working tree is heavily modified and the shared Markdown/MultiSelect files are currently untracked from the prior corpus implementation. Phase 72 must patch only the named surface contracts and never normalize or restore unrelated user work.
- Existing browser captures confirm the detail defect is not caused by the supplied screenshot scaling: at the prior 1280-wide validation the modal header and summary are readable, but the six repeated fact cards, full empty shadow-task band, delivery header, and a second bordered Markdown viewport still consume nearly the entire visible height before the document body is reached.
- The previous companion screenshot also confirms the project already uses a centered host/companion ownership pattern. The detail redesign should preserve that interaction and improve the host surface itself rather than replacing the companion with a nested tab or a second modal.
- The isolated subjective assessment agreed on single detail-body scrolling, flat fact presentation, one default Markdown mode, restrained palette roles, and selection-first responsive causality. It argued for retaining the global timeline below the main workbench; the final review instead makes the ledger contextual and adds an explicit all-records scope, satisfying the user's location correction without deleting audit coverage.

# 2026-07-20 - Jira version sources and decision bottom substrate

- Jira release-page URLs are stable query references rather than issue URLs. The safe application boundary is same-origin parsing of `/projects/{projectKey}/versions/{numericVersionID}` followed by Jira search JQL `(project = "KEY" AND fixVersion = ID)`; page HTML scraping is unnecessary.
- Version-source JQL must remain additive to the ordinary or custom branch. Otherwise a release outside the ordinary project list—or a custom JQL result set—would silently disappear despite being explicitly configured.
- Version projects are part of the effective project catalog, not just the Jira worker. Their configured number/name now feeds demand options, personal project preferences, project configuration/scores, and telemetry score scope.
- The reported decision-page strip was shell-owned: `.workspace-frame` painted the gray page substrate and applied four-sided padding while the decision roots remained transparent. Setting only the decision route's bottom inset to zero lets the page/card root reach the viewport without changing card backgrounds or unrelated routes.
- Authenticated fixture geometry proved the ownership change: at 2048x924 the decision frame and both Decision Agenda/Daily Jira roots end at y=924 with `padding-bottom: 0`; at 760x900 they end at y=900 with no horizontal document overflow.
- Browser configuration validation recognized the supplied link as `PRJ25024 / 13622`, offered the parsed project key before the project name was entered, and displayed the completed valid state without saving or connecting to Jira.

# 2026-07-15 - Phase 72 implementation findings

- The strongest-brain timeline works better as evidence attached to the selected decision than as a second dashboard below the selection/detail workbench. Keeping `当前 / 全部` scope preserves audit completeness without weakening causality.
- Hiding a dropdown rail is safe only when the list remains the same scroll owner. The final shared rule changes scrollbar presentation only; measured overflow, option count, focus ownership, and list height remain intact.
- `#018b8d`, `#6ecc54`, `#eb5c20`, and `#d34947` are not safe default white-text fills for normal text. The semantic token layer therefore separates source color from readable foreground instead of applying one universal white label treatment.
- The detail modal's excess height was caused by multiple scroll owners and repeated card chrome, not by one incorrect `max-height`. Bounded modal geometry plus a single scrolling body and natural-height specification preview resolves both the visual and interaction defect.
- Full-file aesthetic detectors remain noisy on the heavily evolved dashboard sources. The relevant Phase 72 selectors are clean; remaining findings point to older risk-calendar, board, and delivery-policy styles and should be handled as a separate legacy cleanup rather than folded into this scoped refactor.

# 2026-07-18 - Daily Jira timing badge clarification

- Creation age and deadline health are separate facts. The audit buckets and `age_days` remain unchanged, while the first-column status is derived from `due_date` plus the backend `generated_at` audit timestamp.
- The compact contract is `圆点 + 逾期/临期/健康 + · + 今天/N天`. Red means already overdue, amber means due today through three natural days, and green means later or without a due date; every color also has visible text and a full accessible description.
- Date-only Jira deadlines are normalized as calendar dates before comparison so `YYYY-MM-DD` values do not shift across browser time zones.
- The first column is 126px on wide screens, 112px at 760px, and 104px at 520px. Browser measurement at 1280/760/480 found 24px-high badges, `white-space: nowrap`, equal client/scroll widths, zero document overflow, and the existing 16px bottom gap.
- Browser checks also confirmed `健康 · 今天`, exact three-day `临期`, red overdue, green no-deadline, whole-row Enter selection, read-only safety with no POST, and an empty console error list.
# 2026-07-18 - Backend LLM deconstruction availability

- Latest database configuration enables AI, has a non-empty credential, targets `https://sub2.congmingai.com`, and selects `gpt-5.5`.
- Persisted `endpoint_type` is `completions`; `AIConfig.Protocol()` deliberately normalizes every non-Messages value to `responses`, so the real endpoint is `/v1/responses`.
- The authenticated health check passed with the message that `gpt-5.5` is reachable through Responses.
- A harmless synthetic request exercised `/api/deconstruct`, provider streaming, NDJSON browser transport, structured JSON parsing, and UI normalization; it produced three task suggestions with 76% completeness.
- No demand, shadow task, or deconstruct archive was created because neither task synchronization nor demand assignment was executed. The run persisted context pack `24` (329 tokens, two items) as a diagnostic context artifact.
- `go test ./internal/config ./internal/llm` and the focused `internal/server` deconstruction/streaming tests passed.

# 2026-07-20 - Daily Jira bottom ownership and Jira version-link audit

- The bottom mismatch was a duplicated ownership defect. `FunctionalAdminShell` already fixes the viewport and owns the responsive gutter, while `DailyJiraAudit` independently subtracted its DOM top and a fixed 16px from `window.innerHeight`.
- `FunctionalWorkspace` explicitly connected only `.decision-admin` to the viewport-fit content track. Adding `.daily-jira` to the same direct-child contract and using `height: 100%` removes the second calculation without changing panel styling, row density, or responsive information order.
- At 2048x925, the frame ends at 925, its bottom padding is 22px, and both Daily Jira panels end at 903. Decision Agenda produces the same 903px root bottom. The exact equality demonstrates component ownership rather than a viewport-specific visual compensation.
- The authenticated Jira release page is not an issue URL; it is a release/version view. Its authoritative “在问题导航器中查看” link uses `project = 11900 AND fixVersion = 13622`, proving the version's issue set is queryable through Jira search.
- The current client exposes paginated `/rest/api/2/search`, and the worker gives `custom_jql` precedence over project/user/status-generated JQL. Therefore the 21 matching issues are ingestible today by configuring that JQL, but pasting `/projects/PRJ25024/versions/13622` itself is not understood because no release-URL parser or version resolver exists.
# 2026-07-31 - Local release to Jira work-item association

- “全部项目”后的叉号来自共享 Select 把空字符串选项也当成可清除选择；清除动作现在只在真实非空值存在时渲染。
- 创建弹窗和检查器下拉显示不全的根因是绝对定位菜单仍受滚动容器裁切；菜单现已 portal 到 body，并按触发器和视口计算上下方向、宽度与边界。
- 本地版本的项目已经唯一决定 Jira 候选范围，因此 Jira 项目下拉是重复且可能产生冲突的输入，已从右侧检查器移除。
- 版本与 Jira 事项继续以 `work_item_release_links` 表达 N:M 关系；本地关联不会创建 Jira 版本，也不会写回或清空 Jira `fixVersion`。
- 批量关联在一个事务内校验 Jira 来源、同项目和主目标版本冲突，并通过 CAS 更新关系；任何一条冲突都会整体回滚。
- 版本列表的 Jira 数量、候选检索和项目看板版本标识均使用批量/聚合查询，避免逐行 N+1。
- 旧 `release_jira_links` 表和兼容接口暂不删除；新 UI 不再把它当作事实来源，后续删除仍需经过兼容观测门禁。
- 隔离认证浏览器验证了创建成功/失败 Toast、完整长项目名、批量关联两条 Jira、关联列表、项目看板版本标识和 744px 无横向溢出；未触碰生产数据或真实 Jira。

# 2026-07-31 - Task Table Work Item convergence

- 全局搜索原先只更新结果列表，Enter 没有稳定执行“选择 + 导航 + 定位”；搜索入口现以 Work Item 为事实源，优先编号 / 标题精确匹配，否则选择排序首项。
- 任务表错误显示 Git 提交的根因是状态、执行、人员三个视图共用了 `/api/execution/tasks`。状态视图现使用 `/api/work-items`，执行与人员仍保留 Execution Task 证据边界。
- 状态视图的项目和负责人选项必须来自可见 Work Item 及权威项目目录，不能复用 Git 执行任务的提交者和仓库目录；跨视图筛选也必须在切换时清理。
- 右侧文字重叠来自桌面覆盖规则同时使用 `overflow: hidden`、正文行截断和隐藏后续列表项。检查器现独立纵向滚动，正文和列表完整可达。
- 任务详情与决策看板统一的是弹窗容器、事实网格、分区和响应式节奏，不复制决策业务字段，也不修改共享 `Modal`。
- 隔离登录态浏览器在 1280×720 和 390×844 验证了需求 / Bug 回车定位、数据边界、筛选目录、长文本检查器、详情弹窗和横向滚动归属；未进行外部请求或业务写入。

# 2026-07-31 - Task Table core-member candidate boundary

- Work Item 行的当前负责人是事实数据，不等于允许用户筛选或重新指派的候选目录；外部协同负责人可以留在表格中，但不能因此进入负责人下拉。
- 状态视图此前直接对可见 Work Item 的 `assignee` 去重，导致所有历史/外部负责人都成为候选；现改为复用 `/api/demands/options` 的 core member 目录。
- 后端候选接口此前在 `jira.sync_users` 已配置时仍无条件合并 custom JQL assignees，与中央可见性规则不一致。统一规则是 `sync_users` 优先，只有其为空时才从 JQL 回退。
- 执行追踪和人员负载仍使用 Execution Task facets；本次只收窄任务表状态视图的候选授权边界，不隐藏 Work Item，也不篡改行内负责人。
- 隔离登录态浏览器证明：外部负责人行仍可见，负责人下拉只显示核心成员。

# 2026-08-01 - Governed data asset architecture audit

- Baseline `ecd1aaf` contains all pre-existing delivery-planning, evidence, UI, local database, project-rule, and validation artifacts; the new architecture starts from a clean worktree.
- Current persistence is fragmented by source and purpose: `WebhookLog`, `GitCommitLog`, `JiraCommentLog`, `DecisionEvent`, `WorkItemEvent`, `TaskTelemetry`, KPI report projections, and context/corpus records each own different time and provenance fields.
- `WorkItemEvent` is already the strongest immutable domain-specific stream: it records actor, reason, before/after, revision, source, sync state, and created time inside the planning transaction. It lacks occurred/observed/recorded time separation, generic source identity, retention/classification, payload isolation, and a cross-domain cursor query.
- Current KPI preview is computed from `TaskTelemetry` snapshots and returns evidence IDs, but the generated report itself is not an immutable versioned asset tied to an input high-water mark. Recomputing later can therefore change a historical narrative.
- SQLite is the only active database adapter. The first implementation must make current SQLite reads fast while keeping time and scope columns suitable for future time partitioning; introducing a PostgreSQL-only partition implementation now would be an untested seam.
- Hot metadata and cold payload must be separate tables. This prevents timeline/report candidate queries from decoding or moving raw comments, webhook bodies, before/after snapshots, and model output.
- Keyset pagination on descending `(occurred_at, id)` is the required list contract. Page size is bounded; payload retrieval is an explicit detail operation.
- Stable pagination needs an ID high-water mark captured on the first page and carried in an opaque cursor. This excludes concurrently appended and late-arriving events until the caller starts a new timeline, preventing duplicate or shifting pages.
- The cursor must include a normalized-filter fingerprint; otherwise a caller could reuse a cursor with a different project/subject/source filter and silently skip unrelated rows.
- Source replay idempotency cannot compare payload hash alone. The module needs a full fingerprint covering normalized metadata, time semantics, retention/classification, relationships, and payload hash; the same dedupe key with a different fingerprint is a conflict.
- Immutable report/analysis history requires a snapshot metadata table, cold payload table, and an explicit snapshot-to-event evidence relation. Snapshot metadata carries `as_of`, input high-water mark, producer/version, evidence hash, classification, and retention.
- Planning mutations already create `WorkItemEvent` in the same transaction. Appending the normalized asset event through the same GORM transaction is the safest first integration: any planning, domain-audit, outbox, or asset failure rolls the complete command back.
- Historical backfill must not silently create schema during a dry-run. The migration CLI therefore opens SQLite read-only for assessment, while apply mode requires an explicit backup, performs the additive migration, and then advances by `WorkItemEvent.id` in bounded batches.
- Deep pagination performance stays tied to the composite metadata index rather than table depth: a 100-row page near row 20,000 of a 25,000-row fixture completed in about 1.90 ms on the local Apple M4 Pro without reading payload rows.
- The 100-row GORM scan still allocates about 245 KB and 6,568 objects per benchmark operation. Latency is acceptable for the initial bounded API, but allocation reduction through a narrower raw-scan read adapter is a measurable future optimization if production profiling shows GC pressure.
- The full repository test command contains two unrelated suites that bind random loopback ports. Restricted execution cannot run them, and automatic permission review timed out; affected data-asset routes were separately exercised without a network listener.

# 2026-08-01 - Governed data asset hardening findings

- Database triggers must reject duplicate inserts as well as updates/deletes; otherwise SQLite `INSERT OR REPLACE` can delete and recreate an immutable row while bypassing update guards.
- A safe SQLite backup under concurrent/WAL use must come from the database engine. `VACUUM INTO` followed by `quick_check` and fsync captures committed state more reliably than copying the main file alone.
- Dry-run correctness requires the same normalized fingerprint comparison as apply mode. Counting only dedupe keys hides conflicts and gives unsafe migration estimates.
- Historical malformed JSON is still evidence. The backfill preserves it as a JSON string and records validity flags instead of silently replacing it with `null`.
- Snapshot immutability is not enough: creation must prove the declared input high-water mark is real and every evidence event occurred no later than the snapshot `as_of` boundary.
- Release metadata changes are decision-relevant source facts and now enter the same governed ledger; actual weekly/monthly sealed report producers and broader Jira/GitLab source ingestion remain explicit next-phase work.
- A 100-row deep keyset read remains near 1.98 ms locally, but its roughly 245 KB and 6,568 allocations are a concrete profiling target if production GC pressure appears.

# 2026-08-02 - Daily Jira sync refresh findings

- `/api/daily-jira` already uses a live database query and the frontend request sets `cache: no-store`; stale browser data was not an HTTP-cache defect.
- `syncJiraTasks` created and updated `TaskTelemetry` records without invoking `BroadcastTelemetryUpdated`, leaving the established SSE pipeline silent after scheduled or manual Jira synchronization.
- `App.svelte` already converts SSE `telemetry-updated` messages into `well-ambient:telemetry-updated`, but `DailyJiraAudit.svelte` previously refreshed only on mount, manual action, or project-preference updates.
- Broadcasting after the whole Jira batch commits prevents subscribers from observing a partially synchronized set. Tracking only successful writes prevents noisy refreshes on unchanged sync cycles.
- A small event subscriber is the correct frontend boundary: it merges burst notifications, queues one follow-up if a notification arrives mid-refresh, and removes both timer and listener on teardown.
- Browser evidence showed an unselected issue changing title, owner, status, and update time immediately after a simulated sync event while the operator's selected issue and inspector remained stable.

# 2026-08-02 - Daily Jira recent-activity timestamp findings

- The local database confirms Jira rows are clustered around synchronization batches, matching the reported symptom before any formatting layer is involved.
- `syncJiraTasks` ignored the already-decoded Jira `fields.updated` and assigned `time.Now()` to `TaskTelemetry.LastUpdate` for new rows and any local correction.
- `LastUpdate` is also used for local status overwrite protection, assignee override preservation, completion keep-alive, scheduling and Git evidence. Replacing it directly with Jira time would silently weaken those unrelated contracts.
- Jira comment synchronization writes only `JiraCommentLog`; it does not mutate the task timestamp. Daily Jira formatting also preserves distinct timestamps, so neither was causal.
- A dedicated `SourceUpdatedAt` preserves both clocks: Jira source activity for the audit list and local mutation time for concurrency/control logic.
- First synchronization after schema migration intentionally backfills source time and emits the existing telemetry update; unchanged subsequent syncs do not rewrite or rebroadcast it.
# 2026-08-12 版本发布闭环与筛选胶囊工具条

- 版本页已有 `planned/released/archived` 展示和通用 `PATCH /api/releases/{id}`，但没有任何“发布版本”交互；创建弹窗反而允许直接创建 `released`，会绕开一次真实发布动作。
- 通用 PATCH 只更新 `release_versions`，不写 `release_version_published` 资产事件，因此现有最强大脑 `GET /api/strongest-brain/delivery-cockpit` 无法引用发布证据。
- 最强大脑交付驾驶舱当前只从排期事项读取 `target_release_id/name`，响应没有版本状态、发布日期、发布范围数量或资产证据引用。
- 红灯回归 `TestPublishReleasePersistsEvidenceAndMakesItVisibleToStrongestBrain` 首次在隔离 Go 缓存下稳定返回 `404 page not found`；这证明缺少专用发布入口，而不是前端按钮绑定错误。
- 新发布契约采用本地计划中版本单向发布：必须已绑定项目、提交发布日期/操作者/原因；状态更新和不可变资产事件在同一事务内完成，幂等重试复用同一证据引用。
- 通用 PATCH 的 planned -> released 与本地创建非 planned 状态必须关闭，否则专用发布证据链仍可被绕过；归档继续保留通用状态变更。
- 机械 UI 扫描对 `DeliveryPlan.svelte` 和 `DecisionDashboard.svelte` 均返回 `[]`，但人工之外的几何数据确认版本筛选栏在 760-776px 之间会因 `min-width:760px` 溢出，移动端两个 Select 与两个 small Button 低于 44px。
- 与其他页面一致的 toolbar 容器是 `wa-admin-card wa-admin-toolbar` 的大圆角 surface；真正 999px 半径适合单行控件，不适合会换行的整行工具条。版本页根 shadow tokens 已全部为 `none`，可复用几何而不恢复 elevation。
- `planning-with-files` 首次按错误安装路径读取失败，改用技能清单声明的 `/Users/eddie/.agents/skills/planning-with-files/SKILL.md` 后继续；首次 Go 红灯又被默认缓存目录权限挡住，改用 `/tmp/well-ambient-go-build` 后获得功能性 404 红灯。

## 2026-08-12 版本发布闭环最终结论

- 发布不是把状态字段改成 `released`，而是一个受治理的领域动作；只有专用事务同时形成实际发布日期、操作者、原因和不可变资产事件，最强大脑才有可信证据可读。
- 发布事实必须防绕过：本地版本只能从 `planned` 创建，通用 PATCH 不允许 planned -> released，发布/归档后版本元数据和 Jira 范围均只读。
- 最强大脑不再依赖排期事项间接推断版本发布；`delivery-cockpit.releases` 直接投影项目可见范围内的最近发布版本、范围完成度和证据引用。
- 整行筛选工具条应使用可换行的 `18px` 圆角容器，而非单控件使用的 `999px` 胶囊；这样在桌面仍像胶囊，在平板与移动端换行后不会形成畸形外观。
- 页面根部覆盖 shadow token 不足以影响 portal 内容；共享浮层组件需要显式、向后兼容的 `shadowless` 接口，才能在不改变其他页面的情况下保证该流程的下拉、日期和弹窗也为零阴影。
- 登录态隔离浏览器验证确认发布后同一版本不可再次发布或编辑，最强大脑立即显示版本、日期和 `证据 1`，三个断点均无文档级横向溢出，两个受影响页面的 shadow 元素计数均为零。
# 2026-08-12 方案资产闭环现状证据

- `DemandSpecVersion` 已提供需求级版本、Markdown、模型/规则版本和冻结状态，但草案更新直接 `Save`，没有 CAS；它同时保存 Markdown 与结构化 JSON，当前编辑器只更新 Markdown，存在语义分叉风险。
- `MarkdownWorkbench` 已支持 CodeMirror、只读/编辑/预览、外部点击提交和 `Mod-s`；父组件响应式赋值会全量替换编辑器文档，因此后台任务完成只能广播“候选可用”，不能把候选正文写入当前绑定值。
- Jira 评论同步当前只以 `comment_id` 新增 `JiraCommentLog`，不保存 `updated`、内容哈希或删除状态，无法识别已编辑评论；`AddComment`/`GetComments` 可作为 adapter，但写回当前为直接调用而非 outbox。
- `internal/dataassets` 已证明 4KB 阈值、gzip、原文哈希、最大解压限制的编码方式可行；方案模块应保持相同安全属性，但不能依赖其未导出的 codec。
- `AIConfig` 的项目上下文字段不是方案提示词模板；`config:write` 同时授予 admin。方案全局提示词必须使用独立模型和 `solution_prompt:manage`，handler 还要校验全局 `super_admin`，不能只依赖通用权限。
- 当前需求详情已经挂载 `DemandDeliveryControl`，最小 UI 接线点明确；不新增菜单或全局最强大脑队列。

## 2026-08-12 人员绩效后台引擎初始判断

- 现有 `KPIKanban.svelte` 的相对能力分是前端展示性计算；现有项目健康度 PHDI 是项目评分，两者都不能直接作为人员绩效考核结果。
- 本次应形成独立 Performance 模块和持久化边界。模块接口保持小而稳定，后台调度、事实适配、公式版本、快照追加、审计追加和到期清理由内部拥有。
- 当前任务遥测能提供人员、事项类型、状态、工期、难度和部分完成时间，但缺少正式验收、生产逸出、回滚、CI 一次通过、SLA 与改进验证等完整事实。因此首版必须允许指标不可用，并以证据覆盖门槛阻止低覆盖结果成为正式评级。
- 滚动刷新不应覆盖上一版分数：每次运行固定输入水位和公式版本，生成新快照；保留策略只删除到期数据，并额外写入本次清理的截止时间与数量。
- 现有不可变数据资产表通过钩子禁止删除，不适合直接承载允许按保留期清理的绩效历史；绩效快照与审计需要独立表，并把写入和清理权限收口在模块内。

## 2026-08-12 人员绩效后台引擎完成结论

- 新增的 Performance 深模块只暴露 `Start`、`Stop`、`RunOnce`，服务启动时在后台立即试算，之后按周期串行运行；没有新增页面按钮、写接口或访问触发器。
- 每轮固定考核窗口、输入水位和公式版本，形成新的运行记录与人员快照；同一运行和人员有唯一约束，重复周期只追加新快照，不更新旧分数。
- 需求贡献权重为 `min(10, 规模点 × 项目优先级 × 复杂度 × 项目阶段) × 责任份额`；当前主责人份额为 1。Bug 损失函数为 `严重度 × 逸出阶段 × 缺陷责任份额`，任何缺项均不默认补值；Bug 当前指派人只记录为修复贡献人。
- 十项量化指标保持 C01-C10 及既定权重。现有事实可支持验收、按期、一次通过、Bug SLA、变更安全和可预测性的一部分证据，最高覆盖权重为 62%；默认 80% 门槛下只产生 `insufficient_evidence` 试算快照，不会产生正式等级。
- 运行开始、逐人员快照、完成、失败和保留清理都有追加式审计；快照、运行和审计按 `created_at < cutoff` 同事务清理，默认 90 天，恰好位于截止点的数据保留。
- `config.example.yaml` 默认启用，60 分钟滚动、90 天考核窗口、90 天保留；已有生产配置若没有 `performance_brain` 段保持关闭，需要显式启用，避免无意启动人员考核。
- 验证只使用独立内存数据库，没有打开或迁移现有 `well-ambient.db`；新增包连续 10 轮、race、全仓 Go 测试、vet、gofmt 和 diff 卫生检查均通过。

## 2026-08-12 人员绩效下一阶段启动约束

- 本阶段同时触发 planning-with-files、codebase-design 及项目强制 UI 三方门禁；后端继续保持深 Module，小 Interface，事实来源通过内部 Adapter 接入，页面只读取解释和结果，不成为计算触发器。
- UI 属于现有管理产品而非营销页：应继承现有信息架构、共享控件、色彩和密度，优先可解释性、证据状态、权限安全与多断点稳定，不引入视觉噱头或新的设计系统。
- 计算说明页必须由后端再次执行超管授权，不能只依赖前端隐藏导航；非超管访问路由和接口都应被拒绝。
- 三方门禁的职责需要按适用性拆分：design-taste-frontend 明确不负责仪表盘/数据表，应只提供反模板、可访问性、状态完整和现有品牌保留检查；Finesse 与 Impeccable 的 product register 负责信息密度、组件体系和实际管理页实现方向。
- 当前页面不需要英雄区、图片资产或营销动效。拟采用既有产品主题、低动效、高密度的解释型工作台，并把运行状态、公式口径、证据缺口和最近快照作为真实数据而非装饰性指标。
- Impeccable 上下文检测确认项目没有 `PRODUCT.md`，但已有 `DESIGN.md`；本任务是现有产品内的限定功能，因此不启动 init 流程，以真实代码与 `DESIGN.md` 为准。
- 历史审计再次确认旧成员分数是前端相对分且存在语义失真；新页面必须明确展示新后端公式版本和证据覆盖状态，不能继续沿用旧 KPI 能力分口径。
- 三方初始 Design Read 为：研发绩效治理后台，克制、证据优先，product register，SPECTACLE=1，DENSITY=8；只允许 150-250ms 的状态反馈，不做页面入场编排和装饰性动效。
- Impeccable 与 Finesse 一致要求继承现有组件词汇、固定字号、受限强调色、完整 loading/empty/error 状态和结构化响应式；不安装新的 Fluent/Carbon 套件，以免与现有 Svelte 设计系统混用。
- Finesse 的通用 substrate 建议加入 grain、强展示字体和层叠深度，但这与现有产品的扁平、无装饰纹理、稳定数据工作台契约冲突；本页保留现有浅色主题、系统字体、hairline 和无阴影结构，仅采用其数字对齐、单一强调色和状态完整性要求。
- `DESIGN.md` 将 Phase 41 规定为权威视觉基线：深色左轨、浅色主工作区、白色雾面结构边界、表格主导、无模块 hero；任何子菜单必须进入全局左轨，业务组件不得自造视觉系统或 mock 数据。
- 当前 App 有六个顶层区域，KPI 已是 `kpi` 顶层并加载 `InsightsWorkspace activeLens="kpi"`。计算说明最合适成为 KPI 下的超管子菜单，而不是新增第七个同义顶层入口或塞进内容区标签。
- 后端通用 `withPermission` 会让全局 super_admin break-glass 通过，但“仅超管可看”仍需独立全局 super_admin 校验，不能用一个可分配给普通组的 permission 代替。
- `FunctionalAdminShell` 已有按权限过滤的可复用子菜单模型，扩展 KPI 子菜单只需新增 `KPIView`、`activeKPIView` 和导航回调；无需在说明页内重复标签导航。
- `withGlobalSuperAdmin` 已在 server 包内实现，可由绩效只读路由复用；正确包装顺序是 `withAuth(withGlobalSuperAdmin(handler))`，否则全局组校验拿不到认证头。
- 当前 Performance Module 仍把 C04/C05/C06/C10 硬编码为不可用，也没有读取最新运行/快照的方法。这是下一阶段的真实缺口：需要追加式绩效证据账本、内部证据 Adapter、四项指标计算及只读说明投影，外部调度 Interface 不应因此膨胀。

## 2026-08-12 人员绩效正式证据账本设计

- C04/C05/C06/C10 不能继续从任务标题、修复人或状态变化推断；使用带来源、发生时间、观察时间和操作人的显式证据事实。
- 同一 `evidence_key` 的更正与撤销必须追加 revision，不能原位覆盖；当前计算只读取水位线前的最后一版，`void` 版不参与评分。
- 正式证据类型限定为缺陷暴露、缺陷归责、Bug 重开结果、变更回滚结果、验证后改进，修复贡献与缺陷责任继续严格分离。
- 90 天默认保留策略同时覆盖运行、快照、正式证据和审计；每次滚动计算记录各类删除数量。
- SQLite 后台滚动计算需要有限次数的 `BUSY/locked` 重试，避免与正常管理操作短暂竞争时产生不必要的失败快照。
- 证据入账接口只追加 ledger，不主动触发一次计算；因此录入操作本身不会改变交互响应时间，下一次 startup/schedule 水位线统一消费。
- 计算说明读取模型由绩效模块直接输出公式、十项指标、系数表、运行态、最近持久化快照和证据统计；页面读取不调用 `RunOnce`，因此刷新说明页与滚动计算解耦。
- 首次 Impeccable 实现后检测命中一处 `side-tab`：责任边界说明使用 3px 单侧强调线。已按 Phase 41 扁平 hairline 契约改为完整细边框与轻量底色，不保留 AI 风格单侧粗线。
- 浏览器角色降权验证确认导航能隐藏超管子项；进一步加固运行中降权：App 立即将计算说明退回度量概览，说明接口一旦返回 403 立即清空缓存的人员快照，避免仅靠下一次登录或刷新收口。
# 2026-08-12 排期治理轨迹卡片高度对齐 - 初始发现

- 用户截图显示宽屏排期看板的左右主从面板顶部基本对齐，但右侧“代码轨迹”卡片在最后一条轨迹后结束，底边明显高于左侧需求表格面板，右栏下方形成大块无业务含义的空白。
- 这不是“轨迹条目太少”的内容问题。验收对象应是双栏工作台外框的同一行几何；轨迹内容少时外框仍对齐，内容多时只允许右侧主体内部滚动。
- 项目设计契约要求 Phase 41 浅色、表格主导、右侧详情栏和稳定布局；当前任务为 redesign-preserve，不调整视觉系统或业务信息。
- 相关历史约束提示先检查共享 shell/workspace 高度、双栏 grid、gutter 与 scroll owner；同时明确禁止用无所有权的强制等高掩盖内容差异。因此目标是“外框共享行高、内层独立滚动、堆叠态自然高度”。
- 需要以真实 DOM 的 left/right rect、clientHeight/scrollHeight、computed overflowY、document overflow 和断点切换为权威证据，截图只用于定位症状。
- 浏览器控制面没有可认领的既有标签页；已在同一浏览器会话打开 `http://localhost:5173/`，应用标题为 `web`。下一步先确认登录态/当前路由，再做几何红灯，不假设截图状态仍然保留。
- 内置浏览器确认为登录页，因此已按规范切换到 Chrome；Chrome 中存在可精确认领的 `well-ambient` 本地标签页（`http://localhost:5173/`），可复用现有登录态，不需要读取或输入任何凭据。
- 已认领 Chrome 的现有登录页，当前正是截图对应的排期看板状态：HR-4202 被选中，右侧“代码轨迹”页签已激活并显示 1 Push、0 MR、3 Comment。可直接建立不修改业务数据的运行时几何红灯。
- 2382×1100 运行时初测：`.schedule-main-grid` 为 `display:grid`，`top=235.09`、`bottom=1078.00`、`height=842.92`、`overflowY=hidden`；右侧 `.schedule-inspector-panel` 为 `top=310.19`、`bottom=892.32`、`height=582.13`，计算样式 `align-self:start`。按主内容行底边计，右侧少约 `185.69px`，与截图空洞一致。
- 右侧 `.schedule-telemetry-inline` 高约 `431.86px` 且 `overflowY=auto`；内部 `.drawer-panel.is-inline` 高约 `399.86px`，高度跟随内容。说明内容自身已具备滚动边界候选，首要问题是检查器外框没有参与主网格行的共享高度。
- 首次左侧 ARIA region 命中了 `display:contents` 的 `.schedule-table-stack`，其矩形为 0；下一步需要枚举 `.schedule-main-grid` 的直接/关键子项，锁定真实表格卡片矩形，避免拿容器底边代替左卡底边。
- 已锁定真实左卡 `.schedule-table-panel`。在 2382px 宽时其 `top=310.19`，内容稳定后 `bottom=1015.78`、`height=705.59`，内部 `.schedule-table-wrapper` 为 `overflowY:auto` 且 `scrollHeight=4312 > clientHeight=642`，左侧滚动归属清楚。
- 第一次固化红灯时页面在两次采样之间发生了响应式/热更新状态变化：Chrome 实际 viewport 高度从 1100 变为 1038，代码轨迹 DOM 暂时消失，右检查器变为与左卡等高。因此“必须存在 `.schedule-telemetry-inline`”让断言因状态缺失而 RED，但没有继续捕获用户原始 bottom mismatch。需要先确认当前选中页签并用明确 viewport override 固定 2382×1100，再运行最终红灯；不能把这次状态漂移当成修复成功。
- 后续快照证明不是产品自动切页，而是被认领的用户标签页已切到“任务跟踪 / 任务表”（全局搜索 DL-4309）。为避免与用户当前操作争用，后续将新建一个同一 Chrome 会话下的验证标签页，复用登录态但由本任务独占；不再继续控制用户原标签页。
- 独占验证页复用了认证并进入排期看板；默认选中 HR-4202 且“排期设置”页签激活。已只读切换到“代码轨迹”，确认 `.schedule-telemetry-inline` 与 `.drawer-panel.is-inline` 存在，数据仍为 Push 1 / MR 0 / Comment 3。
- 稳定反馈环已建立：在独占登录页、2382×1038、HR-4202“代码轨迹”状态下连续两次运行 `await checkScheduleAlignment()` 均返回 `RED`；左右 top delta=0，但 bottom/height delta 均为 `123.4636px`，结果确定且 0.2 秒内完成。
- 红灯同时证明当前正确滚动边界没有丢失：左 `.schedule-table-wrapper` 为 `overflowY:auto` 且 `4312 > 642px`；右 `.schedule-telemetry-inline` 为 `overflowY:auto`；嵌入 `.drawer-panel.is-inline` 与 `.drawer-body` 均为 `overflowY:visible`。修复必须只改变外框等高，不改变这些内部所有者。
- 源码最终 cascade 位于 `DemandKanban.svelte` 后段：较早的 `@media (min-width:1281px)` 已声明双栏面板 `height:100%`，但更晚的“Final schedule inspector contract”把 `.schedule-main-grid` 设为 `align-items:start`，并把 inspector 设为 `align-self:start; height:auto`。这是与 computed style 完全一致的直接根因。
- `<=1280px` 的后置媒体规则已经显式保持单列、inspector `height:auto; max-height:none; overflow:visible`，因此桌面修复可安全限定为 `min-width:1281px` 或把通用规则恢复 stretch、由现有窄屏规则覆盖。
- 两份隔离审查汇总：Impeccable 视觉评估认为外框需表达同一业务行、内容不做均匀拉伸；机械 detector 返回 `[]` 但 cascade 扫描命中同一末尾覆盖。design-taste 只采用 redesign-preserve；finesse product register 支持共享外框、单一滚动所有者，反对修改 `CommitTelemetryPanel`。
- 回归测试先红后绿：`node --test --experimental-strip-types web/tests/schedule-inspector-height-contract.test.ts` 初次 3 项中 1 项失败，准确指出最终宽屏块仍为 `start/auto`；最小 CSS 修复后 3/3 通过。
- 修复后同一独占登录页连续两次 `await checkScheduleAlignment()` 返回 `GREEN`：左右 top/bottom/height delta 全部为 `0`，两侧均 `705.59px` 高；inspector 计算为 `align-self:stretch`。轨迹 tab body 从 `431.86px` 填充到 `555.32px` 并仍为唯一 `overflowY:auto`，inline drawer/body 仍为 `visible`。
- 假设 1 得到直接确认：只改最终 cascade 后 123.46px 差值归零；没有出现根高度链漂移、第二层滚动或延迟回缩，因此当前证据不支持假设 2-4，无需扩大到共享 shell 或 `CommitTelemetryPanel`。
- 静态门禁通过：目标回归与既有 modal 回归共 6/6；`pnpm -C web check`、app TypeScript、生产构建、目标 diff check 均 exit 0；Impeccable layout detector 返回 `[]`；Finesse P0=0，唯一 P2 是 `DemandKanban` 既有纯白色值，与本次高度选择器无关。
- 宽屏修复后截图已人工复核：左表与右侧轨迹卡片从同一标题基线开始并落在同一底边，原截图右栏下方大块空洞消失；轨迹内容仍顶部聚集，没有被按条目均匀拉开。证据文件：`outputs/ui-validation/schedule-inspector-height-wide.png`。
- 宽屏“排期设置”页签同样通过：左右 top/bottom/height delta=0；`.schedule-editor-body` 为 `overflowY:auto`，`scrollHeight=665 > clientHeight=555`，右侧表单可独立滚动且没有裁剪。
- 1280×900 精确断点下，grid 已变为单列 `"controls" "table" "inspector"`；inspector 计算为 `align-self:start; height:auto; overflowY:visible`，editor 也为 visible 且 `clientHeight===scrollHeight=665`。右栏自然流到左表下方，宽度一致，document 水平溢出为 0。内容延伸超过视口但 document 固定，因此需补测 schedule workspace 的纵向滚动所有者。
- 1280×900 的纵向所有者已确认：`.demand-dashboard` 为 `overflowY:auto`，`scrollHeight=1520 > clientHeight=744`；其外层 workspace/frame 均保持 hidden，正是既有 861-1280 工作区滚动契约，没有内容丢失或 document 纵向溢出。
- 760×900 已进入自然页面流：`.demand-dashboard` 和 grid 均为 `overflowY:visible`，document `scrollHeight=2480 > clientHeight=900`；左右面板宽度一致为约 719.79px，inspector `align-self:start/overflow visible`，editor `clientHeight===scrollHeight=1119`，document `scrollWidth===clientWidth=748`，无水平溢出。
- 760×900 的“代码轨迹”页签同样保持自然内容高度：inspector 与 inline telemetry 均为 `overflowY:visible`，由 document 承担纵向滚动，没有桌面等高或第二层滚动泄漏。
- 390×844 的两个页签均完成验证：document `scrollWidth===clientWidth=378`；左右面板宽度一致约 349.79px。排期设置 editor `clientHeight===scrollHeight=1119`，代码轨迹 inline 高约 423.85px，inspector/inline 都为 `overflowY:visible`，内容完整进入自然文档流。
- 390px 全页截图已人工复核：筛选、表格、检查器保持单列顺序，排期设置表单与代码轨迹条目均无裁剪或巨型空卡。证据：`outputs/ui-validation/schedule-inspector-height-390.png`、`outputs/ui-validation/schedule-inspector-height-390-trace.png`。
- 独占验证页控制台最终 `error/warning` 为空；已恢复浏览器视口并释放所有本轮认领/创建的标签页。
# 2026-08-12 代码轨迹完整可滚动浏览

- 用户明确拒绝轨迹条数截断及“另有 x 条轨迹，可在任务跟踪中查看完整记录”提示；完整记录必须在排期看板当前代码轨迹页签内可达。
- 三方评审共识是修复数据呈现边界，不扩大外框或增加嵌套滚动：宽屏由 `.schedule-telemetry-inline` 滚动，窄屏由页面自然滚动。
- 保护现有左右面板等高、轨迹排序/统计/刷新与其他页面数据链路；只移除当前组件的隐藏边界和误导性跨页提示。
- 红灯命令 `node --test --experimental-strip-types web/tests/schedule-telemetry-completeness-contract.test.ts` 稳定 0/2；第一项直接命中 `presentation === 'inline' ? commits.slice(0, 4) : commits`，第二项命中“另有 x 条轨迹”及 `inline-overflow-note`。
- inline 组件不仅隐藏第 5 条以后记录，单条 `.timeline-body` 还使用两行 `line-clamp`；“当前页签内浏览所有内容”需要同时解除记录级切片和正文级裁切，由既有外层滚动区承载增长。
- `handleGetTaskCommits` 使用不带 `Limit` 的 Git/Jira 查询，合并全部记录后仅按 `CreatedAt` 倒序排序并完整编码；后端注释与现有测试也明确是 “returns all”，假设 3 已排除。
- `DemandKanban.svelte` 已让 `.schedule-telemetry-inline` 在宽屏使用 `min-height:0; flex:1 1 auto; overflow:auto`，并在 `<=1280px` 恢复 `overflow:visible`；inline drawer/body 本身保持 visible。滚动所有权契约正确，假设 4 不需要结构修改。
- 需要修改的唯一产品组件是 `CommitTelemetryPanel.svelte`：直接遍历 `commits`，删除 overflow note，并在 inline 模式解除 timeline message/MR link 的两行或单行裁切，允许长内容换行。
- 最小修复后同一测试命令 3/3：组件直接 `{#each commits as log}`，源码不再包含切片、隐藏提示或 `inline-overflow-note`；inline message/MR link 改为完整换行显示。
# 2026-08-12 DG-394 静默方案润色诊断

- 用户可见症状：DG-394 在 Jira 存在方案评论，但排期治理右侧方案卡片没有显示静默 Agent 润色结果。
- 既定业务契约：Jira 明确标记评论形成来源快照；后台自动入队；Agent 输出保存为候选修订，不能覆盖人工 Markdown 或已发布版本。
- 当前首要证据链：`solution_assets` -> `solution_source_refs(current && eligible)` -> `solution_polish_jobs` -> `solution_revisions(kind=candidate)` -> `/api/solutions/workspace`。
- 尚未确认：DG-394 是否已有资产/来源、评论标记是否 eligible、任务是否入队或失败、候选是否已保存。
- 数据库事实：`solution_assets.id=105`；`solution_source_refs.id=274/external_id=457134/author=jira公用-解决方案/current=1/eligible=0/marker=''`；没有 polish job 或 revision。
- Phase 1 红灯：`active_sources=0 completed_jobs=0 candidates=0` 且退出码 1，直接捕获用户所述“有评论但无静默候选”的链路缺口。
- 排序假设：H1 正文标记格式未被识别；H2 Jira 标准化丢失标记；H3 首次自动入队错误依赖 working revision；H4 prompt/LLM 不可用（因无 job 证据，优先级最低）。
## 2026-08-13 人员绩效 Core Member 与详情扩展

- `loadSubjectEvidence` 当前直接按 `TaskTelemetry.assignee` 和正式证据 `subject_key` 聚合，未接入任何 core-member 过滤，因此所有具名负责人都可能形成绩效快照。
- 仓库已有 core-member 权威来源：优先 `jira.sync_users`，为空时解析 `jira.custom_jql` 的 assignee；`kpiCoreMemberFilter` 再通过用户目录把姓名和 username 别名归一。绩效模块应复用这一份配置结果，而不是另建人员名单。
- 普通看板在没有 core-member 配置时沿用“过滤关闭”以保持本地可用，但人员绩效属于考核发布边界，用户明确要求只包含 core member，因此该模块应 fail closed：名单为空时不生成任何人员快照。
- 当前快照已经持久化 `metrics_json`、`item_factors_json`、`exclusions_json`、输入摘要、样本数、证据覆盖率和评分状态，成员详情无需重新计算；应由 Performance Module 解码并投影现有快照，查询保持只读。
- UI 方向沿用既有 Phase 41 product register：成员行改成可聚焦入口，详情使用共享 overlay 语义和单一正文滚动，不增加新导航或新的视觉系统。
- 设置页现有保存链会提交完整配置并生成配置版本，但 `performance_brain` 尚未进入前端配置模型和独立配置面板；新增开关应沿用这条受权限保护、可审计的保存链。
- 共享 `Modal` 已统一实现工作区内居中、焦点圈定、Escape/遮罩关闭和窄屏适配；绩效详情应直接复用，不另造定位逻辑。
- 当前服务启动时将 `performance_brain` 配置复制进计算模块，运行中保存配置不会自动重建定时器；若不引入可重配置生命周期，设置页必须明确“重启服务后生效”。本次优先评估并实现运行时可切换，避免开关名存实亡。
- 计算模块当前把 `Settings` 作为无锁值字段，`Start` 的 ticker 也只读取一次 interval；要支持在线开关，不能直接并发改字段，需由模块提供串行化 `Reconfigure`，停止旧循环后原子替换设置并按新开关重启。
- 正式证据的 as-of 查询按 `created_at <= input_watermark` 选每个 evidence key 的最后修订，适合详情接口还原该快照当时可见的来源；详情查询可保持只读并避免使用当前时点数据冒充历史。
- `Server.Start` 已有一个覆盖所有后台模块的 `workerContext`，但未保存在 `Server` 上；要在线启停绩效循环，应保存该 context，并让配置应用成功后调用模块重配置。服务退出仍由同一 context 取消，保持现有生命周期边界。
- `/api/config` 已受 `config:write` 保护并在保存后归档版本；开关不需要另造旁路接口。`applyConfig` 是保存与回滚的共同入口，运行态同步应放在这里，确保手工保存和版本回滚行为一致。
- 用户目录当前支持姓名、username、邮箱前缀及去点号/下划线/连字符别名解析；绩效过滤必须保留这些等价身份，否则 `sync_users` 写 username、Jira 任务写中文姓名时会误排除 core member。
- 绩效模块测试库尚未迁移 `users` 表；新增 core-member 别名回归时需把 `userdb.User` 纳入测试迁移，现有评分测试也要显式配置 core member 才能符合 fail-closed 新契约。
- 后台定向验证结果：`internal/performance` 全部通过；`internal/server` 在允许本机 `httptest` 临时端口后全包通过。新增在线重配置、core-only、空名单 fail-closed、别名归并、详情只读和接口鉴权回归均已进入测试集。
- 绩效页现有快照表只有纯文本成员单元格；最小交互改动是把姓名/摘要包成表内按钮，仍保持列宽与行布局不变。详情使用共享 `Modal`，不改变页面主层级。
- 设置页配置组件普遍采用 `config + save` 事件契约和项目 `Switch/Button/Alert`；绩效开关应复用同一契约，而不是在 `SettingsPanel` 内直接写临时表单。
- 设置导航由 `settings-sections.ts` 统一定义分组、标签与权限；新增 `performance` 应作为“集成设置”中的受 `config:read` 管理项，以免扩展整个管理台分组模型。保存仍由 `config:write` 后端校验。
- 绩效页现有响应式表格最小宽度约 980px；详情弹窗应以 `wide` 共享 Modal 承载独立的摘要、指标、系数和来源分区，窄屏由 Modal 既有 viewport 规则和内部单一滚动区处理。
- 共享 Modal 的 `wide` 模式固定最大 960px、工作区内居中、正文唯一 `overflow-y:auto`，且自动恢复触发按钮焦点；完全满足“位置参考其他页面”和键盘访问要求。
- 配置组件的总样式已提供 `scw-overview/header/toggle/read-grid/actions` 等稳定类；新绩效开关面板可无新增视觉系统地复用这些类，并显示当前滚动周期、窗口、保留期和 core-member 来源摘要。
- 第一轮前端验证：生产构建通过；绩效配置/共享弹窗静态契约 6/6。全量 `svelte-check` 的 9 个错误全部来自本轮未修改的 `SolutionWorkspace.svelte` 空值收窄，绩效新增文件无类型错误，新增的 3 条滚动区域 a11y 提示已用项目既有说明式豁免收敛。
- 详情 Modal 现在覆盖评分摘要、快照/水位、十项指标引用、需求/Bug 系数过程、正式证据来源和未计入项；所有数据来自 `GET /api/performance/snapshots/{id}`，UI 不存在计算写接口。
- 登录态浏览器已验证计算说明页真实渲染：后台状态、公式、十项指标、系数表和 30 条持久化快照均可读，成员入口具备明确按钮语义，共享详情 Modal 能打开并由统一关闭按钮获得初始焦点。
- 当前 30 条是旧计算器遗留的全员快照，且运行中的 8080 后端尚未加载新增详情路由/快照 ID，点击详情返回 404。新代码必须在 `Explain` 阶段也按当前 core-member 名单过滤历史展示，随后重启本地开发后端再完成真实详情验证。
- `Explain` 已改为按当前 core-member 身份集合分页筛选持久化快照；配置为空时返回空列表，超过首个 100 条批次的核心成员历史也能被找到。旧的非核心快照和审计仍保留在数据库，只从当前考核视图排除。
- 新增回归用 105 条更新的非核心快照遮挡一条 core-member 快照，仍只返回规范化后的 `Alice Smith`；`internal/performance` 与绩效前端契约 6/6 均通过。
- 本地 Go 服务按根目录命令重启后，新启动轮次在 12:49:26 完成；计算说明页从旧的 30 条全员结果切换为最近 28 条 core-member 快照，当前可见姓名均来自 `jira.sync_users` 名单，快照按钮也已带有效 ID。
- 最新韩程程快照的真实详情接口返回完整只读投影：公式版本、90 天窗口、输入水位、10 项指标及不可用原因、1 条需求和 5 条 Bug 的逐事项系数、正式证据来源空态、10 条排除原因；未再出现旧后端 404。
- 2382×1038 宽屏实测 Modal 为 960px 宽，精确居中于工作区，正文 `overflow-y:auto`，document 水平溢出为 0，统一关闭按钮自动获得焦点；Escape 关闭后焦点回到原成员按钮。
- 当前两轮共 28 个快照按钮，对应 14 个唯一成员：韩程程、陈伟华、纵涵、白凌云、王财和、梁志远、李厚奇、朱家聪、张路路、岳颖颖、尹祖雷、孙海峰、姜昊良、刘子翔；全部在当前 Jira core-member 配置内。
- 登录态配置中心已出现“集成设置 / 绩效计算”入口；真实页面显示开关已启用、后台静默运行、60 分钟周期、90 天窗口与审计保留、`personnel-v1`、80% 覆盖和 5 个样本门槛。当前名单来源显示为从 Jira 自定义 JQL 的 assignee 解析，与服务启动日志的 assignee 限定一致。
- 开关沿用配置版本审计区域和 `config:write` 权限；本轮浏览器只读验证未切换开关，在线启停行为由后端 `Reconfigure` 回归覆盖，避免为验收额外写入用户配置。
- 390×844 视口下，绩效配置页仍保留开关、运行状态和全部配置事实，未因移动断点丢失入口；下一步从同一断点进入计算说明并测量详情弹窗边界。
- Chrome 的 390×844 override 在当前浏览器缩放下报告 433×938 CSS viewport；弹窗实际 `left=12/right=421/top=24/bottom=914`，完全落在可用视口内。正文是唯一纵向滚动区（816/2306px），document 水平溢出为 0，关闭控件约 44×44px 且初始聚焦。
- 最终代码二次重启后，计算说明显示调度器“静默运行 / 已启动”，最近完成更新为 12:53:47、触发源为服务启动且状态已完成，证明最后一道非核心详情 404 边界已随最终二进制加载。

## 2026-08-13 人员绩效 Jira 历史证据为零诊断

- 原始反馈命令：对最新 completed run 统计“事项数大于 0、可用指标数等于 0”的成员快照，结果为 `14` 且退出码 `1`；精确复现用户所述全员 0/历史需求未参与。
- 最新轮次实际已为 14 名 core member 归集 Jira 事项，例如梁志远 14 个需求、27 个 Bug；core-member 筛选和事项类型转换不是主断点。
- 14 名有事项成员的 `effective_sample_count=0`、`evidence_coverage=0`、`observed_score/final_score=NULL`。页面语义应为 N/A/证据不足，不是事实上的 0 分。
- 当前 90 天数据中 14 名成员都有已完成需求或 Bug，但所有人的 `completed_at` 数量都为 0；梁志远有 6 个 done 需求、13 个 done Bug，却没有一条明确完成时间。
- `JiraIssue` 结构只接收 created/updated/status 等基础字段，未接收 `resolutiondate`、`duedate`、原始估算；`syncJiraTasks` 也从未写入 CompletedAt/DueDate/EstimateDays。
- 当前业务 `custom_jql` 明确排除 done/testing 等状态，因此它只适合活跃看板，不足以发现 90 天考核周期内的历史完成事项。
- 数据库 `execution_runs=0`，所以 C01/C03/C08 没有执行验收、首次通过或流水线证据；C04/C05/C06/C10 也没有正式证据账本记录。不能为消除 0 分而虚构这些维度。
- `NormalizeIssueType` 已支持 requirement/demand/bug，Jira Task 也在入库前映射成 requirement；事项类型假设被排除。
- 最小安全修复应包含：独立于活跃 JQL 的周期历史完成查询；持久化 Jira resolution/due/original estimate；无 execution run 时仅允许 Jira Done 作为 C01 的可追溯降级证据。其余证据维度仍保持不可用。
- 三个修复前回归已红：单条已完成 Jira 历史需求仍产生 `EvidenceCoverage=0/ObservedScore=nil`；服务端缺少历史 JQL 构造器；历史同步入口不存在。测试准确覆盖原断点，而非只断言服务未报错。
- 三个回归已转绿：单条 Jira 已解决需求得到 C01=100、覆盖率 20%、有效样本 1 和 `jira:WA-HISTORY:resolved` 引用，但因 80%/5 样本门槛仍不发布正式等级。
- 历史 JQL 明确包含项目、issue type、core member、`statusCategory = Done` 和 `resolutiondate >= -90d`，不会继承活跃业务 JQL 的 done 排除条件。
- Jira 搜索现在显式请求并解析 `resolutiondate`、`duedate`、`timeoriginalestimate/timetracking`；历史适配会写入 CompletedAt/DueDate/EstimateDays，手工估算已有来源时不覆盖。
- 历史适配不拉评论、不触发方案链；按绩效滚动周期节流。历史事实发生变化后追加一轮 schedule 快照，因此启用或重启后无需再等待整整一个周期。
- `internal/telemetry`、`internal/performance`、`internal/server` 全包测试与 vet 已通过。
- 完成类指标增加统一状态边界：只有当前仍为 `done` 且存在完成时间的事项才能进入 C01/C02/C07/C09；重新打开的 Jira 事项不会沿用旧 resolution 产生虚假按期或可预测性得分。对应回归已先红后绿。
- 历史同步增加幂等回归；同一批未变化的 Jira 完成事实不会重复触发评分快照。
- 真实服务于 2026-08-13 13:13 启动并加载数据库配置版本 21；历史 JQL 成功返回 234 条已解决事项，13:14:06 立即追加一轮 `schedule/completed` 快照。
- 原始 SQL 反馈从 14 个“有事项但零可用指标”的成员降为 0；最新 14 名 core member 全部有 C01，合计保存 92 条 `jira:<issue>:resolved` 证据引用。
- 登录态页面确认最新 14 人均显示非零试算分，范围 50.00-100.00；当前证据覆盖为 20%-40%，因此仍标记“证据不足/待发布”，不把试算分冒充正式等级。
- 梁志远详情显示 14 个有效样本、20 个需求/44 个 Bug、C01=100，并直接列出 `jira:DL-4106:resolved`、`jira:FZ-2247:resolved` 等 12 条历史 Jira 验收引用；需求系数表同时列出对应项目、规模与权重过程。
- 《研发考核评分判定表》v4.0 与当前 `personnel-v1` 存在实质差异：判定表要求逐指标 2/3/4/5 分边界、逐指标最小样本、70% 覆盖率、30 天最小暴露期及 85/75/60/45 等级；当前实现使用连续比例百分制、全局 5 样本、80% 覆盖率和 90/80/70/60 等级。
- 当前历史 Jira 降级查询只拉 `statusCategory = Done`，导致 C01 的可见样本天然只有分子、没有周期内已到期但未完成的分母；这会把“只看到完成项”的幸存者样本推成 100 分，不能用于人员绩效判定。
- 三方 UI 门禁的共同约束已明确到方向层：保留现有 Phase 41 产品界面与共享 Modal，不新增视觉系统；主列表只展示正式分，试算与计算来源在详情渐进披露。仍需完成组件归属、响应式和验证范围的书面一致意见后才可编辑前端。
- 表格技能要求将工作簿单元格作为评分口径的证据源并在交付中引用精确 sheet/range；本次不改写工作簿，只读提取并通过代码测试逐项镜像其配置。
- design-taste-frontend 明确数据密集型 dashboard 不属于其营销页面构建范围，因此三方门禁中只采用 redesign-preserve、状态完整性、对比度和不改变 IA 的约束；Finesse 与 Impeccable 的 product register 共同要求高密度、低动效、使用既有组件。
- 项目没有 PRODUCT.md，但 Impeccable 将本次识别为现有组件上的 scoped fix，因此不阻塞实施；设计上下文以现有 `DESIGN.md`、Phase 41 token 和目标组件为准。
- 历史记忆再次确认人员评分必须把代码事实、公式、测量窗口、证据缺口和建议分开，且模型输出不能直接冒充正式人事结论；这与本次“正式分仅在发布门槛满足后显示”的方向一致。
- 当前轮次用 artifact-tool 2.8.43 复核源工作簿：只有“使用说明”和“系数配置”两张表，后者以常量表定义 2/3/4/5 分边界而非可执行公式；“全指标3分对应60”给出统一换算，因此实现采用离散 2/3/4/5 档对应 40/60/80/100，未达到2分边界为1分即20。
- 判定表明确执行顺序是先冻结周期、适用指标、暴露天数和公式版本，再录入项目/Bug/版本/改进事实，最后由五席会议只作通过、补证、重归因或不评级；会议不得直接修改原始分。自动后台因此只能形成可审计投影，不能绕过资格门槛发布人事结论。
- 判定表并不禁止单项 100 分：达到某指标 5 分边界会得到 100；真正错误是当前单个 Done 样本绕过 C01 最低 3 样本，且只抓完成项导致分母缺失。修复应保留合法 100 的可能性，同时阻止不合格样本和不完整覆盖成为正式分。
- 代码差异已定位：`metricAvailable` 直接把比例乘 100；`calculateSubject` 用可用权重重新归一；`levelFor` 使用 A/B/C/D/E 90/80/70/60；默认公式/覆盖门槛是 `personnel-v1`/80%。这些都必须被 v4.0 规则模型替换。
- 当前 C01/C02/C07 只在事项已完成后创建样本，导致未完成但已到期事项不会进入分母；C09 使用估算天数与实际耗时误差，也不符合判定表“责任延期/计划周期”公式。
- 现有 Jira 绩效历史 JQL 固定 `statusCategory = Done AND resolutiondate >= -Nd`，日志也将结果描述为 resolved issues；必须扩为“周期内已解决或周期内已到期”的评估事实查询，并在评分水位上再判定 due eligibility。
- 当前系数偏离判定表：优先级 P0=1.30/P1=1.15、复杂度高1.25低.85、阶段 POC/.90 交付1.15 售后1.10、严重度10/6/3/1、生产逃逸2.50。v4.0 要求分别是高/中/低1.2/1/.85，高/中/低1.3/1/.8，生产/开发/运维/设计需求1.15/1/.9/.85，S1/S2/S3/S4=8/5/3/1，生产/UAT测试开发=1.5/1.2/1/.6。
- 现有测试明确断言历史单个 Jira Done 的 `ObservedScore=100`，这正是需要先翻红再修改的错误契约；另一个“全指标 formal”夹具每项只有 1 条，但依靠全局样本数通过，和逐指标最低样本规则冲突。
- 说明接口和配置 UI 仍公开旧公式、旧等级和旧系数，所以本次不能只改 `scoring.go`；必须同步默认配置、只读说明、快照详情字段与前端标签，避免后台 v4.0、页面 personnel-v1 的双口径。
- `TaskTelemetry` 没有成员在单个项目中的负责人/核心研发/支持角色字段；由于评分边界只允许 core member，v4.0 自动计算可安全采用“核心研发=1”的中性角色系数并在事项因子中留警告，不能凭 assignee 推断负责人1.2。
- 正式证据账本当前只有通用 `Weight/Outcome/ResponsibilityShare`，无法审计判定表中的发布方式、回退影响、责任归因和趋势/信任风险/杠杆原因码。要完整实现 v4.0，需要扩展证据字段与快照调整量，并继续以追加修订保存。
- C09 在 Jira 基础字段中无法区分责任延期与外部阻塞；不能继续用估算误差冒充预测准确率。安全实现应只消费明确的责任延期/外部阻塞证据，缺少时保持 N/A，而不是把所有逾期归责给当前 assignee。
- 配置文件 `config.yaml` 与 `config.example.yaml` 当前仍显式保存 `personnel-v1`、80% 覆盖率；即便代码默认值改为 v4.0，真实重算也会继续吃旧显式值。实现完成后必须同步本地配置到 `v4.0`、70% 和 30 天暴露期，再重启重算。
- 评分相关文件本身来自当前未提交的工作树，且仓库还有大量用户/前序任务改动；本次只在现有绩效、Jira历史、配置和目标 UI 文件上做增量补丁，不清理、不还原其他改动。
# 2026-08-14 算分面板样式与对齐优化

- 本轮仅优化人员绩效算分面板的视觉层级、布局和对齐；评分算法、数据语义、后台滚动计算、core-member 和审计边界保持不变。
- 项目强制使用 Impeccable、design-taste-frontend、finesse-ui 三方 UI 门禁；全局 `ui-design-system` 也被任务语义触发。当前尚未定位最终组件，不能先行编辑前端。

# 2026-08-13 scorecard v4 closeout

- `SettingsPanel.svelte` still declared and seeded the legacy `personnel-v1` / 80% shape, which caused the new `PerformanceConfig` contract to fail type checking. The shared settings model must carry `minimum_exposure_days` and v4 defaults.
- A persisted legacy configuration can otherwise keep reporting the old formula version or gates after deployment. Formula version `v4.0`, 70% coverage, 5 global samples, and 30 exposure days are therefore normalized as the immutable workbook contract; scheduling, enablement, window, and retention remain configurable.
- The focused TypeScript contract test cannot use `tsx` because it is not installed. Node 22's `--experimental-strip-types` path executes the `.ts` contract directly; all three governance contracts pass.
- Workbook governance forbids using the same work item in the base score and a leverage/risk adjustment. The scorer now excludes and audits such duplicate adjustment evidence.
- The first live v4 run produced 14 core-member snapshots, zero formal scores, and trial values from 20 to 45. C01 contains 14 weighted references across the roster, including 8 due-denominator references and 6 resolved references; no live formal or trial 100 remains.
- All 14 live rows are N/A for the formal score because the v4 publication contract is not met. Exposure is sufficient (83-90 days), but current governed evidence coverage is at most 32%; this is a real evidence gap, not a zero-score substitution.
- The v4 run created one run-start audit, fourteen snapshot-created audits, one run-completed audit, and one retention audit. The preceding `personnel-v1` runs remain immutable history.

# 2026-08-13 scorecard v5 closeout

- 5.0 将人员绩效收敛为 3 个可执行维度、8 项可审计指标；数量只作为证据和风险提示，不再直接奖励需求数或 commit 次数。
- 最终试算分采用各指标离散档位分乘固定权重后直接求和，不按“可用指标权重”重新归一；未达到样本、覆盖和暴露门槛时正式分保持 `N/A`。
- 需求系数由规模、需求等级、项目权重、复杂度、阶段、角色与责任共同形成并封顶 10；Bug 修复负责人流转区间与缺陷责任归因分离，避免把经手人自动认定为责任人。
- Jira 历史同步已持久化 4774 条状态、负责人、优先级、截止日期和快照来源事件，覆盖 860 个事项；Git 证据按 SHA 去重并使用稳定指纹，所有来源事件与评分快照保留追加式审计。
- 最新 v5.0 启动轮次只包含 14 名 core member，14 条快照无重复，全部处于 shadow；试算分范围 4-44，精确 100 分为 0，正式分保持 `N/A`。
- 解决方案工作区缺少已加载正文的 `{:else}` 分支，导致数据存在但页面空白；补充分支后，登录态浏览器确认 DG-352 方案正文 244 行、4060 字符可见。
- 宽屏和真实 390px 视口已验证：计算说明展示 8 项 v5 指标、成员详情可追溯分子/分母/事项系数/来源引用，Modal 仅有一个关闭按钮且关闭后销毁。
- Go 全量测试、vet、24 项前端契约、Svelte 检查、生产构建、Impeccable 检测、差异卫生和工作簿公式/视觉检查全部通过；测试进程均已停止，标准服务需由使用者按根目录命令启动。

# 2026-08-13 方案默认预览与弹窗编辑

- 当前 `SolutionWorkspace.svelte` 在主页面直接挂载 `MarkdownWorkbench mode="live"`，保存、发布和“编辑新版本”与固定链接同层，阅读与修改任务没有分离。
- 默认界面可见的版本信息来自 `方案 v{version}`、`历史版本 {count}`、加载文案和远端冲突文案；后台 `asset.revision`、`working.id` 与 `expected_revision` 是 CAS 所需事实，必须保留但无需展示。
- 共享 `Modal.svelte` 已提供 wide 960px、焦点圈定/归还、Escape/背景关闭和 760px 移动端几何；本次不需要新建 overlay 或修改共享组件。
- Markdown 正文仍是唯一权威；主预览应绑定 `workspace.working.markdown`（已保存事实），弹窗编辑器绑定 `editorState.markdown`（本地草稿），避免未保存修改在主预览中伪装为已保存内容。
- 已发布正文不可直接保存；“编辑方案”是明确写意图，点击后可调用既有 fork API 建立可审计草稿，再进入即时编辑。保存和发布继续携带 `expected_revision` 与 `base_revision_id`。
- 新增的两个契约测试已按预期先红：当前组件没有共享 Modal、默认 preview、单一编辑入口或脏草稿关闭保护。
- 实现后主视图绑定已保存的 `workspace.working.markdown` 并使用 `mode="preview"`；弹窗绑定本地 `editorState.markdown` 并使用 `mode="live"`，未保存内容不会出现在主预览。
- 保存和发布已收敛到共享 Modal footer；发布前会先保存脏草稿，已发布正文点击编辑时仍通过既有 fork API 创建受治理草稿，CAS 的 `expected_revision` 与 `base_revision_id` 未改变。
- 使用真实数据库的隔离副本和禁用 Jira/AI/绩效的临时配置完成登录态验收；原数据库、方案历史和审计记录未被验证过程修改，临时进程与数据库已清理。
- DG-352 默认 DOM 只有“编辑方案”和“方案预览”，不存在“保存方案”“发布方案”或可见版本号；点击后共享 Modal 显示“即时修改方案”、实时编辑器、保存和发布操作。
- 桌面弹窗保持 wide 居中和固定 footer；390×844 下弹窗落在视口内，正文单一纵向滚动，保存/发布形成 44px 操作区且无横向溢出。
- 输入未保存测试文本后保存按钮启用；点击关闭出现“继续编辑/放弃修改”，放弃后 Modal 销毁、预览恢复且测试文本未持久化。最终浏览器 console warning/error 为 0。
- 最终验证为 26/26 前端契约、`pnpm check` 零错误、生产构建成功、Impeccable `[]`、Finesse P0/P2 均 0、`git diff --check` 通过；仅保留其他现有页面的 Svelte warnings 与既有大 chunk 提示。

# 2026-08-14 版本右侧面板与生命周期

- 项目选择和保存按钮分离的直接原因是二者作为 `.inspector-section` 的两个网格子项各占一行，同时非 compact Select 自带 16px 下边距；不是项目数据或 Select portal 问题。
- 首次响应式实现用 inspector 容器 420px 作为堆叠阈值，真实桌面 inspector 宽约 397px，导致桌面误触发单列。登录态几何验证捕获后改为仅在页面视口不超过 430px 时堆叠。
- 原版本模型只有 planned/released/archived；通用 PATCH 虽能部分修改状态，但没有废弃语义、专用归档接口、删除路由或追加式生命周期证据，前端也没有入口。
- 新状态机明确为 planned → released → archived，或 planned → discarded；released/archived 作为发布事实不可删除，planned/discarded 仅在无活跃事项关联和 Jira 版本关联时允许软删除。
- 归档、废弃和删除都要求 actor 与 reason，使用稳定 dedupe key 写入不可变 DataAssetEvent；归档/废弃重复请求回放同一事实而不重复写事件。
- ReleaseVersion 增加 indexed `deleted_at`，正常列表和仓储查询自动排除软删除项，Unscoped 与不可变事件仍保留审计事实。
- 列表 API 同批聚合活跃事项与 Jira 版本关联计数，返回 `can_publish/can_archive/can_discard/can_delete` 和删除阻塞原因，避免前端只靠状态猜测权限。
- 登录态页面验证：1440px 和真实 760px 下项目选择/保存底边一致、间距约 8px；760px 控件均约 44px；390px 有意单列且按钮全宽约 310px，页面横向溢出均为 0，inspector `min-height` 为 0。
- 废弃与删除确认均显示影响、版本名、必填原因；删除额外要求精确版本名称。取消确认后焦点回到原“废弃版本”按钮，浏览器 console error 为 0。
# 2026-08-14 绩效 v6.0 数字资产算分收敛

- 用户确认开始按“完成结果35 + 可预测性20 + 工程质量45 - Git风险最多10”的方案改造；后台滚动、core member、不可变审计和可配置保留期继续保留。
- 当前 `internal/performance/rules.go` 将 D01-D03、B01-B03、G01-G02 作为 8 个正向加权指标；Git 两项仍贡献 10% 正分，与新方案“风险只扣不奖”不一致。
- 当前质量主指标 B01 只读取 `performance_evidence_facts.defect_attribution`；Jira Bug 的修复负责人只参与 B02/B03，纯 Jira/Commit 环境通常无法满足正式核心质量指标。
- 当前需求权重还乘复杂度、项目阶段和固定核心研发角色，责任份额默认 1；v6.0 应只保留规模、需求优先级、项目优先级和可证实责任份额。
- UI 三方共同采用 Phase 41 preserve-mode：一级只显示交付、质量、风险、证据状态，旧指标/事项因子降为成员详情证据；共享 shell 和 Modal 不改。
- Impeccable/product 与 finesse/product 均要求固定字号、右对齐 tabular 数值列、完整 loading/empty/error 状态、44px 移动端触控和一致组件词汇；本次不改字体、配色、导航或共享组件。
- finesse redesign 审计确认本轮最高价值不是重画页面，而是减少一级认知负担并保持成员表和详情 Modal 的既有交互所有权。
- codebase-design 将 `calculateSubject` 识别为现有深模块的核心 interface；v6.0 应把权重、质量损失、风险扣分和资格门槛隐藏在绩效模块内部，通过既有 run/snapshot/explanation seam 暴露结果，不能把新算法分散到 handler 或前端。
- Jira/Git/database 都是本地可替代依赖，继续通过现有 SQLite 测试适配器验证持久化输出即可，不需要新增外部 port 或浅层 wrapper。
- 当前 Jira 搜索字段与 `JiraIssue` 结构不包含 `parent` 或 `issuelinks`，`TaskTelemetry.ParentWorkItemID` 只在执行任务迁移/继承中使用；纯 Jira Bug 到原始需求的自动归责尚无数据通路。
- v6.0 要形成 Jira-only 个人质量分，必须在 Jira adapter 增加 parent/issuelinks 采集并以明确规则选择原始 requirement；否则 Bug 只能保留为项目级或未归责证据，不能落到修复人。
- 现有模块测试已覆盖完整 8 指标、历史无 due 完成、到期分母、重开需求、core-member 去重和不可变快照；v6.0 应在同一 Module interface 下把这些契约改写为 3 正向项 + 风险扣分，旧历史行无需迁移。
- Jira 同步的创建、keep-alive 和历史补采都复用 `applyJiraPerformanceFields`，因此 parent/issue-link 归一应集中在该函数和 `JiraIssue` adapter，避免三个同步路径各写一套规则。
- 算分说明页的类型、公式摘要、8 指标表、快照列、详情指标/系数表均直接消费 explanation contract；v6.0 可以保持页面与 Modal owner，只调整 contract 字段投影和主表列，不需要新路由或共享 UI 组件。
- 需求权重公式的可见文案仍展示复杂度、阶段、角色；v6.0 后端删除这些乘数后，详情必须同步改为“规模 × 需求优先级 × 项目优先级 × 责任份额”。
- v6.0 最终规则仅保留 D01/D02/B01 三项正向计算；Git 只产生重复变更率和 Commit 密度两项风险扣分，不再以 Commit 数量形成正向绩效。
- 质量分母只接纳完成后已暴露至少 30 天的需求；Jira 缺少逃逸阶段或缺陷覆盖确认时最高按 4 档，避免无证据的质量满档。
- Jira Bug 只有 direct parent 或唯一需求链接时才进入个人质量归因；多候选保持未归责，修复负责人数据只作为闭环贡献事实。
- 最新隔离数据库实算产生 14 条且 14 名唯一 core member 快照；参考分非空范围 11–71，没有精确 0 或 100，样本不足保持 N/A，不再用 0 代替缺证据。
- 页面刷新只重新读取持久化 explanation/snapshot；刷新前后 run 数均为 37，未产生后台重算。隔离启动的下一轮最终快照确认风险审计 JSON 中负零记录为 0。
# 2026-08-14 - 度量洞察卡片流与空白修复

- 大空白的直接原因是 `.guide-layout` 的单行双列 Grid 由更高的右侧 `.guide-inspector` 决定行高，而后续全宽 `.factor-section` 只能等整行结束；sticky 不会把 inspector 从文档流中移除。
- 正确结构是让“总分与责任边界 -> 三项正式计算口径 -> 需求与 Bug 计算系数”同属 `.guide-main`，由各自真实内容高度连续排布；不需要负 margin、固定高度或绝对定位。
- 宽屏系数矩阵使用 `repeat(auto-fit, minmax(220px, 1fr))`，中屏保留 2 列、窄屏 1 列；指标表仍在自己的 shell 内局部横向滚动，不扩大整个文档。
- 浏览器以登录态验证到指标区与系数区间距为 16px，系数确实是 `.guide-main` 的直接子项，页面级横向溢出为 0；980px 下 inspector 位于主内容之后。
- 当前运行服务仍可能返回 v6 之前的快照 JSON，其中 `delivery_score/quality_score` 字段不存在。前端 TypeScript 先前把两者写成必填 `number | null`，掩盖了真实运行态的 `undefined`；显示辅助函数直接 `toFixed` 因而报错。字段改为可选并使用 nullish 回退后，旧快照显示 `N/A`，新页签控制台无 warning/error。

# 2026-08-15 Jira 完成状态与评论入站同步

- 用户给出真实样本 DL-4309：刚在 Jira 中完成事项并新增评论，但系统没有同步刷新；要求按整条同步链路做通用修复，不能做单记录补丁。
- 仓库当前有大量既有未提交修改，且 Jira worker、daily Jira、strongest-brain、telemetry、performance 与 UI 均已有并行工作；本次必须逐文件确认差异归属，只做可叠加的小改动。
- 诊断门槛：先得到一个已执行、秒级、确定性、能对“状态和评论均未进入系统”判红的命令；在此之前不基于代码外观猜根因。
- 真实系统检查默认只读；真实 Jira 不用于制造测试评论/状态，主数据库不作为可破坏的测试夹具。
- `CONTEXT.md` 明确 Jira 事项是需求/缺陷等源事实的外部来源，系统内的交付项、资产事件和投影快照有不同时间语义；修复不能把“页面当前状态”当成唯一事实，也不能让刷新动作临时生成不可审计结论。
- 既有领域约束区分 `version_sources` 的 Jira 查询范围和系统内工作项的真实业务归属；同步修复必须保留项目/版本范围，不用 DL-4309 特例绕过范围治理。
- 历史实现已存在本地保存后的 Jira 到期日/评论写回，这是出站同步；本次症状是 Jira 完成状态与评论进入系统的入站同步，两者必须分别追踪，不能误用写回测试证明入站已恢复。
- 先前经验要求真实 Jira 校验当前配置与会话，但隔离浏览器验证不能声称真实 Jira 写入；本次只读获取 DL-4309 的状态/评论时间可作为反馈环输入，回归应使用可控 Jira HTTP 夹具。
- 当前入站主链为 `syncJiraTasks` 每 30 秒运行：主 JQL 搜索后更新 TaskTelemetry、同步评论，再对本地 Jira 事项做 keep-alive 搜索，最后只为 `changedTaskIDs` 广播 `telemetry-updated`。
- 已发现一个可直接解释“只新增 Jira 评论时页面不刷新”的结构性缺口：`syncJiraComments` 无返回值，新增/编辑/删除评论均不会进入 `changedTaskIDs`，因此该周期不会广播；评论虽可能已经落库，打开中的活动面板仍不知道要重新取数。
- “完成状态未同步”仍需真实样本和红灯确认。主 JQL 可能按配置排除 Done，依赖本地 keep-alive 抓回；keep-alive 又受本地事项识别、项目范围与 14 天窗口约束。任何一个边界漏掉都会让完成变更永久脱离主查询。
- 当前工作树已有未提交改动：SearchIssues 已分页、评论已分页、SourceUpdatedAt 已持久化、批次结束后已按任务广播。这些改动尚未覆盖“评论变化本身触发广播”或“主 JQL 退出事项必须被可靠抓回”的症状级契约，不能视为本次完成。
- 状态更新仍以本地 `LastUpdate` 的 15 秒保护窗判断；若该字段混合 Jira 活动与本地动作，可能短暂延迟但下一轮应恢复。需要用连续两轮夹具区分“短暂保护”与“永久漏项”。
- 2026-08-15 真实 Jira 页面只读确认：DL-4309 状态 `Done`、解决结果“完成”，评论 `533697` 内容“仿真环境已部署”，评论/更新/解决约为北京时间 22:41。
- 主数据库随后已存在相同事实：TaskTelemetry 为 `done`，`source_updated_at=2026-08-15 14:41:25Z`、`last_update=14:45:10Z`；JiraCommentLog 已有 `533697`。因此当前核心症状不是最终落库丢失，而是约 4 分钟延迟和/或落库后前端没有正确刷新。
- 本地服务 PID 31231 正监听 8080。需要继续核对其启动时间/日志，以及受影响页面是否订阅 `telemetry-updated`；数据库事实已证明不能把问题简化成 Jira API 鉴权失败。
- 该本地服务启动于 20:19:25，并在完成第一次检查后、第二次 HTTP 探测前退出；未发现日志文件句柄，因而不能从现有进程还原 22:41–22:45 的失败/重试细节。后续不得把这段缺失日志伪装成已证明原因。
- 前端刷新契约不一致：`DailyJiraAudit`、打开的 `CommitTelemetryPanel`、`SolutionWorkspace` 订阅 `well-ambient:telemetry-updated`；`TaskKanban` 只每 60 秒轮询，`DemandKanban` 每 15 秒轮询，`DecisionDashboard` 每 15 秒轮询。即使后端广播成功，任务看板也不会立即响应。
- 后端广播只携带 `task_id`，适合统一驱动受影响投影重新读取；当前页面各自轮询导致 15–60 秒额外陈旧窗口，也无法在 Jira 评论变更但 TaskTelemetry 字段未变时获知变化。
- 已建立正确调用缝的红灯：DL-4309 字段已完全同步且仅出现评论 533697 时，评论成功落库但 telemetry channel 为空；失败文本为 `Jira comment changed local activity without broadcasting a telemetry update`，2–6 秒可重复。
- 代码结构还显示一个更高阶延迟来源：主查询中的每个事项都串行执行评论 HTTP 请求；相同事项进入 keep-alive 后同周期再次同步评论；`changedTaskIDs` 又只在所有主项和 keep-alive 完成后统一广播。需要用请求计数/慢响应测试量化，而不是凭代码外观下结论。
- 当前真实 `config.yaml` 的 custom JQL 覆盖约 40 个项目、Bug/Task、15 名负责人，并明确 `status not in (done, testing, awaiting fix, merge request)`；DL 在项目范围内，但 DL-4309 一完成就退出主查询，只能等主查询整批处理完后进入 keep-alive。这与“评论/完成后约 4 分钟才落库”的真实时间线相符。
- 当前没有 Jira 入站同步状态/水位模型；DB 只有交付 outbox、方案 source watermark 和绩效 input watermark 等其他领域状态。若要可靠增量抓取 Done 后的评论/重开，需要新增受限同步状态，不能挪用业务 `TaskTelemetry.LastUpdate`。
- `InitDB` 使用集中 AutoMigrate，允许新增纯加法同步状态表并由现有内存测试自动创建；任何水位只应在事项字段与评论均成功提交后推进。
- 主库现有 1,012 条 Jira 事项：497 条非 done、431 条 14 天内 done、84 条更早 done；2,488 条评论覆盖 842 个事项。按当前实现，一个稳定周期最多先对约 497 条主结果逐条拉评论，再对约 928 条 keep-alive 结果重复拉评论，约 1,425 次串行评论请求，30 秒 ticker 无法提供 30 秒新鲜度保证。
- `codebase-design` 指向的深模块 seam 应是“一次 Jira 入站 reconcile 周期 -> 返回每事项结果与周期状态”；Jira HTTP 是 true-external adapter，SQLite 是 local-substitutable adapter。调用方不应知道 JQL 补偿、水位、评论分页、幂等或广播顺序。
- 当前 Jira worker/client/db 文件已有约 1,400 行未提交并行修改，本次不能整文件重写。实施应通过少量私有 helper、加法模型和调用点收敛完成，保留性能/方案/版本同步既有逻辑。
- 三条症状回归已同时变红：评论-only 落库无广播；稳定事项两轮被重复拉评论 4 次（主查询 + keep-alive 各一次/轮）；旧 Done 事项既不在主查询也不在 14 天 keep-alive，重开和新评论完全遗漏。
- 实现需要先收集并按 key 合并主/补偿结果、按 Jira updated 降序处理，再以每事项成功水位跳过稳定评论；这样既消除重复请求，又让刚更新的 Done 事项优先完成并立即广播。
- 后端初版实现已使三条红灯转绿，并保持全部 `internal/server -run Jira` 与 `internal/telemetry` 回归通过：主/补偿查询合并去重，成功周期保存 checkpoint，评论成功保存 per-issue watermark，变更事项逐个广播。
- JiraCommentLog 现在是明确的当前投影：`current=true` 才进入活动 API；远端删除会把本地投影标为非 current，同时 solution source set 继续按全量 ID 对账。历史行未物理删除。
- 前端属于既有高密度产品界面的可靠性修复，而非视觉改版：Impeccable 要求保留 Phase 41 的层级、token 和事实优先语言；design-taste-frontend 明确其营销页规则不适用于 dense product UI，仅采用 preserve-mode、稳定状态和无布局抖动约束；finesse-ui 将本面归类为 product register，目标是低 spectacle、高 density。
- 三方评审的共同方向是把 SSE 事件合并、并发去重、运行中补刷和销毁清理收进一个共享刷新模块；Task/Demand/Decision 组件只声明自己需要重新读取的投影。此次不新增卡片、文案、控件、动画或 CSS，因此既有响应式几何与可访问交互保持不变。
- design-taste-frontend 自身把 dashboard/dense product UI 列为 out-of-scope；本次只执行其 redesign-preserve 与稳定性守则，不引入营销页 hero、图像、动效、字体或页面结构。现有信息架构、路由、文案、主题和 analytics 事件均锁定。
- 受影响四个 Svelte 文件均已有大批用户未提交改动，不能改写组件结构；事件接入应只增加 import、订阅和 cleanup。既有未跟踪 `daily-jira-refresh.ts` 已具备 debounce 与运行中补刷，适合作为兼容适配器，其通用职责应下沉到新 `telemetry-refresh.ts`。
- `TaskKanban.fetchTasks` 自带单飞但会直接丢弃并发调用；共享调度器必须在刷新运行中记住新事件并补跑。`DemandKanban`/`DecisionDashboard` 的数据替换不会先清空现有数组，事件刷新可保持当前内容，但应避免新增 loading skeleton 或视觉反馈。
- 旧 `/api/status` 只返回硬编码在线状态，无法判断 Jira worker 尚未首轮、正在同步、失败或长期停滞。新增公开状态必须只暴露布尔错误与时间/计数，不能回传 `LastError` 原文，以免 URL、令牌或内部请求细节泄漏。
- 后端 SSE 每个客户端只有 32 个 task ID 缓冲，`BroadcastTelemetryUpdated` 在满载时直接 default 丢弃；初次/恢复同步若同时变化超过 32 项，会让打开的 task-specific 活动/方案面板永久错过自己的事件。轮询只能兜底列表，不能证明所有按 task 过滤的消费者即时一致。
- Jira 已启用但没有任何可构造的 JQL 时，旧 worker 在保存 `LastStartedAt` 后直接返回，状态会永远像“同步中”并最终 stale；现在该路径必须落明确失败且不推进成功水位。
- 根因不是单点鉴权失败，而是四个可叠加的新鲜度缺口：Done 后立即退出主 JQL、每事项评论串行且同周期可能重复、评论-only 变化不广播、核心列表页不统一消费 SSE；真实 DL-4309 的约 4 分钟落库差正好符合这组链路缺陷。
- 通用修复通过周期与事项双水位、5 分钟 inclusive overlap 的已知 Jira key 补偿查询、主/补偿去重、按 Jira updated 新到旧逐事项 reconcile、评论当前投影和成功后才推进水位来封住边界遗漏；单事项失败不会阻塞其后重试资格。
- SSE 端使用每客户端去重待发集承接 32 槽缓冲之外的 task ID；前端共享调度器再做短时合并、单飞和运行中补刷，形成“后端不丢事件、前端不重复轰炸接口、轮询继续兜底”的分层契约。
- `/api/status.jira_sync` 只公开 enabled/state/时间/计数/has_error，足以区分 pending、syncing、healthy、error、stale、unavailable，同时不暴露具体 Jira 错误文本或内部请求细节。
- 登录态实际构建验证中，DL-4309 的状态事件在约 635ms 内把任务页从“进行中”原子替换为“已完成”，只产生一次 work-items 读取，列表几何完全不变且 console 为空；这证明修复不是仅有代码静态正确性。
- 临时 Jira 后端验证方案被主动放弃：复制数据库中的版本化配置会覆盖临时 YAML 并可能重新启用外部 worker。最终浏览器证据改为同源无外连 Service Worker 夹具，验证后已注销并清理，避免对真实 Jira、主库或 outbox 造成副作用。

# 2026-08-15 证据链筛选输入空白回归

- 已确认本地现有后端 `:8080` 与前端开发服务 `:5173` 正在监听，可优先复用真实登录态建立症状级浏览器红灯，不需要先启动或改写服务。
- UI 修复按既有 Phase 41 产品界面 preserve-mode 执行；finesse 的营销型视觉基质与本次高密度产品筛选可靠性无关，不引入纹理、字体、配色、页面结构或动效变化。
- 两次同步追加 findings/progress 的组合补丁均因跨文件锚点不精确被整体拒绝，未产生文件修改；现已拆分计划错误记录与进度追加，避免一个错误锚点阻断全部更新。
- in-app Browser 已打开 `http://127.0.0.1:5173/`，当前落在登录页而非已登录路由；因此尚未取得筛选输入空白的红灯，下一步只定位本地测试登录方式，不检查 bug 源码。
- README、docs、版本化配置中没有公开的本地默认账号/密码；登录走后端 `/api/login` 与 WellOS 鉴权，需从隔离测试登录契约或现有会话建立复现，不能打印或猜测真实凭据。
- `/api/login` 支持 loopback dev auth，但会在所连接数据库中创建/更新用户、授予本地超管组并写审计日志；因此不对当前 8080 主服务尝试假账号，改用复制数据库与禁用外联 worker 的隔离后端。
- 只读 `ps -p 31231` 被环境权限拒绝，未获得也未输出进程环境；不会升级去读取可能含敏感配置的进程详情。仓库可见主库为 `well-ambient.db`，配置模板为 `config.example.yaml`。
- `cmd/server` 将数据库固定为进程工作目录下的 `well-ambient.db`，因此隔离验证必须在 `/tmp` 工作目录运行编译产物并放置数据库副本，不能从仓库根直接启动。
- `config.example.yaml` 实际含非占位敏感字段；后续不再整文件输出或复用其中值，临时配置只写本地端口与外联禁用开关，最终答复不引用任何敏感值。
- 主库中的版本配置表名为 `config_versions`；隔离副本需清空该表，防止 `BootstrapVersionedConfig` 用归档配置重新启用 Jira/GitLab/AI。
- 后端配置结构允许显式关闭 GitLab、Feishu、Jira、AI 与 performance worker；当前未发现后端直接托管 `web/dist` 的入口，因此隔离前端将用单独 Vite 实例代理到隔离后端。
- 隔离副本已通过 SQLite backup 创建并清空 `config_versions`、Jira 发布 outbox 与待处理 polish job；临时配置被重新归档为副本内 version 1，外联配置保持禁用。
- 沙箱内首次绑定 18080 与 5174 均按预期被 EPERM 拒绝；经受控端口授权后，隔离后端与独立 Vite 已分别在 `127.0.0.1:18080/5174` 运行，主 8080/5173 未停止或改动。
- 隔离 dev 登录成功，页面读取数据库副本中的真实投影（含 DL-4309），并显示“本地临时会话”；登录写入只发生在 `/tmp` 副本。
- 证据链页在副本上初始显示 36 个项目、37 行（含表头）。通过 DOM `fill('N')` 后立即与 900ms 后都保持完整工作台，结果为 2 个项目/3 行，未出现整页空白；说明问题可能依赖逐键输入、零结果、选中项切换或请求时序，当前反馈环尚未判红。
- 红灯已建立：输入 `N` 后短暂正确显示 2/36 项，但随后“搜索项目、编号”输入框本身消失，自动化填充报 `no_matches`；页面最终只保留共享 shell 与证据链标题，工作台内容变为“没有匹配的项目证据数据”。
- 可自动判定症状为：初始 `textbox=1/workbench=1/rows=37`，输入后短暂 `textbox=1/rows=3`，延迟刷新后 `textbox=0/workbench=1/rows=0` 且出现顶层空数据文案。反馈环无需依赖截图主观判断。
- 源码确认筛选本身是纯本地 `scores.filter`，输入事件不会直接发 API；但顶层渲染错误地以 `searchMatchedScores.length === 0` 决定是否卸载整个工作台，导致任一零匹配中间字符都会连筛选框一起移除。表格内部其实已有正确的“当前状态筛选下暂无项目”空行，却被顶层分支提前截断。
- `scores` 只有 `fetchScoresAndConfigs()` 会覆盖，组件内调用点仅 onMount 与显式“刷新”按钮；还需用精确定时复现与数据计数排除隔离 worker/重挂载造成 `scores=[]` 的第二条路径。
- `web/tests` 目前没有 ProjectHealthTelemetry 的筛选/空状态回归；仓库惯例既有源码契约测试与纯 TypeScript 行为测试，可在不引入新测试框架的前提下先锁定顶层源数据 empty 与表内筛选 empty 的所有权。
- 保证无匹配的 `__NO_SUCH_PROJECT__` 已把红灯最小化为同步 80ms：输入前 `input=1/rows=37`，输入后 `input=0/rows=0/topEmpty=1/workbench=1`。这排除了远端请求、选中项回写和异步竞态是必要条件；根因就是过滤后数组错误拥有整个工作台的 empty 分支。
- 三方 UI 评审共同要求保留现有 Phase 41 产品层级、筛选即时性和本地数据快照；修复不加 debounce、不发输入请求、不改 CSS，而是把“源数据为空”与“当前筛选无结果”分到正确的渲染层级。
- 定向契约先真实红 1/3：源 empty 所有权和可恢复空状态两项失败，本地筛选不发请求项已通过；实现后 3/3 转绿。
- 生产改动仅调整 ProjectHealthTelemetry 的三个渲染状态：顶层空态改看 `scores.length`，表内区分搜索无匹配/健康筛选无匹配，inspector 对筛选空集提供恢复提示；未改请求、筛选算法、CSS 或路由。
- HMR 后浏览器绿灯：保证零匹配词等待 1.1 秒仍保持 `input=1/table=1/rows=2`，显示表内与 inspector 的可恢复空态，顶层 source-empty 为 0；输入框保持 active，源计数仍为 36。
- 连续英文 `N/NS/NS2`、中文 `南/南沙`、退格与清空全过程均保持 `input=1`、`sourceEmpty=0`，未再卸载页面；状态筛选零命中也保留表格。当前会话的健康筛选状态影响清空后的行数，需先读取实际激活筛选/计数再验证“清空恢复全量”，不能仅按 row 数下结论。
- 浏览器 wrapper 的 `fill('')` 没有清掉当前 NS2 值；改用真实键盘 `Meta+A` + `Backspace` 后，输入仍 active、计数恢复 `36 / 36`、表格恢复 37 行。该差异属于测试驱动方式，不是产品状态异常。
- 浏览器页签 wrapper 不通过可枚举属性暴露 viewport/console 方法；不猜内部实现，优先使用已知截图/语义 API与现有窗口尺寸，窄屏若无法安全调整则用已有响应式 CSS 未改动事实加 `svelte-check/build` 兜底。
- 在 `NS2` 筛选状态点击显式刷新，立即与 1.2 秒后都保持 `input=1/rows=2`、`sourceEmpty=0`，最终仍为 `1 / 36`；输入不会触发远端刷新，显式刷新也不卸载当前筛选快照。
- 已知 Browser Playwright 表面没有 `setViewportSize` 方法；一次调用返回明确 TypeError，未影响页签。停止猜测该私有能力，不重复调用；窄屏将以浏览器现有可用面板尺寸或静态响应式契约说明边界。
- 全部前端契约 48/48 通过，`pnpm check` 为 0 errors/86 条既有 warnings；目标组件只命中 6 条既有 unused CSS warning，本次改动区无新诊断。
- 生产构建通过，仅保留既有大 chunk advisory。Impeccable 检测结果 `[]`；Finesse 仅报目标文件旧 CSS 的 3 个纯白 P2（2429/2489/3293），均不在本次状态改动区，按 dirty-tree 保护不顺手改色。
- 浏览器零结果态暴露一处文案语义：0 个搜索命中时决策条仍写“当前项目证据健康稳定”。虽然不再空白，但“无匹配”不等于“稳定”；应在同一状态所有权修复中补齐并用定向契约锁定。
- 决策条准确性回归先 2/3 红，增加 `searchMatchedScores.length === 0` 分支后 3/3 绿；浏览器现在同时显示“当前搜索无匹配项目”、表内无匹配和 inspector 恢复提示。
- 最终浏览器复核：零结果 `input/summary/tableEmpty/recover` 均为 1；真实键盘清空后仍 `input=1`，恢复 `36 / 36` 和 37 行。
- 最终全量前端验证再次通过：48/48 契约、`pnpm check` 0 errors/86 个既有 warnings、生产构建成功、Impeccable `[]`、Finesse 仍仅 3 个未触碰的旧纯白 P2、`git diff --check` 无输出。
- 目标组件原本已有并行未提交的共享 Modal close-button 改造；本次只叠加 437–625 附近的筛选状态所有权/文案修改，不覆盖或回滚该既有改动。
- 停止隔离后端时输出的“Email extension hook”经源码核验是明确 stub：只 `log.Printf` 后返回 nil，不执行 SMTP 或网络发送；所有通知/登录写入仅发生在 `/tmp` 数据库副本。
- 18080/5174 已无监听；临时目录内仅有隔离 DB、编译产物与禁外联配置，已永久删除且不可恢复。Browser 不提供已尝试的 close API，两个验证页签已导航到 `about:blank`，不再连接本地服务。

# 2026-08-16 Jira 同步协程与间隔复核

- 既有 DL-4309 修复记录表明同步链路已改为主/补偿查询合并、事项双水位、逐事项广播、SSE 不丢事件和前端共享刷新；本次只以当前代码重新确认 worker 生命周期与所有时间间隔，不把旧记录当作当前运行事实。
- 需要明确区分服务端 Jira 拉取周期、失败后的下轮重试、任务 keep-alive 频率、同步成功后的 SSE 推送，以及前端 debounce/轮询兜底。
- 当前源码在 `Server.Start()` 中以 `go s.startJiraSyncWorker()` 启动一个后台 goroutine；worker 创建固定 `30s` ticker，并在 ticker 前立即执行首轮 `runCycle()`。
- Jira 拉取并未对每个事项再开 goroutine：主查询、补偿查询、按 Jira updated 排序后的逐事项 reconcile 均在同一个 worker goroutine 中串行完成；事项有变化时在该事项完成后立即广播。
- `30s` 当前是 `jira_worker.go` 内硬编码值，不在 `JiraConfig` 中；另有 `15s` 状态覆盖保护和 `5m` 增量查询 overlap，它们不是同步轮询间隔。
- Jira HTTP client 单次请求超时为 `10s`；失败没有独立快速重试循环，而是记录本轮失败并等待下一次 worker tick（通常 30s，若本轮已跨过 tick 则可能立即进入积压的一次 tick）。
- 同步成功后 SSE task event 没有固定等待周期；后端写出后，前端共享调度器默认用 `120ms` 合并同批事件。SSE 自身另有 `15s` heartbeat，它只维持连接和推送通知兜底，不负责向 Jira 发起同步。
- `performance_brain.interval_minutes` 只控制独立的绩效 Jira 历史同步/计算，不是交付项状态与评论的常规 Jira 入站同步间隔；不要把示例中的 60 分钟误认为 DL-4309 链路周期。
- 页面刷新兜底并不统一：任务页 `60s`；需求页与决策页 `15s`；Daily Jira 当前没有定时轮询，只在首次加载、项目偏好变化和 Jira SSE event 时刷新。正常 SSE 路径均先经过共享 `120ms` 合并器。
- 浏览器 `EventSource` 使用浏览器原生自动重连；后端同步事件到达时直接转成 `well-ambient:telemetry-updated`。因此 DL-4309 正常可见延迟上界主要是“等下一轮 Jira worker（0–30s）+ 当轮 Jira HTTP/串行 reconcile 用时 + 前端约 120ms”，不是页面 15/60 秒轮询。
- 健康接口在最近成功超过 `10m`，或一轮开始后超过 `10m` 仍未成功时标为 `stale`；该阈值用于告警，不是同步间隔。
- 定向 Go 回归 7 个症状级用例通过，覆盖评论-only 广播、稳定事项不重复抓评论、旧 Done 补采、失败不推进水位、新事项不被慢事项延迟、SSE burst 不丢和状态脱敏；前端共享调度/消费者 7/7 通过。
- `127.0.0.1:8080/api/status` 当前不可连接，本机没有可供核对的运行实例；因此 30 秒结论是当前工作树实现和测试事实，部署/重启是否完成仍需在目标环境验证。

# 2026-08-16 Daily Jira 属性变更滞留与水位日志修复

- 用户实测 Jira 属性变化后 Daily Jira 仍保留该事项，同时控制台持续输出 `jira_worker.go:1068 record not found`；必须分别验证投影资格重算和水位持久化，不能假定二者同根。
- 1068 行是按 task_id 读取 `JiraIssueSyncState` 的 `First`；代码把 `ErrRecordNotFound` 当作预期首轮状态继续处理，但 GORM 默认 logger 仍把这次探测打印为 `record not found`。因此日志本身不是 fatal error，但若同一事项持续出现，说明该事项后续未能保存成功水位。
- 主库只读证据：DL-4309 当前本地状态已是 `done`，对应 issue sync state 存在且 `last_error` 为空；它按当前 Daily Jira resolved 规则应被排除，用户本次看到的事项可能不是 DL-4309，或浏览器仍持有旧投影。
- 更严重的运行证据是最近 Jira 全周期失败：`successful_through` 与 `last_succeeded_at` 仍为零，`last_error` 已截断到 4000 字符；单轮处理 1004 项、判定 681 项变化并耗时约 33 秒。另有 2 条 Jira task 尚无 issue state。
- 多个 source_updated_at 停留在 6 月的 Done 事项，其 `LastUpdate` 却被本轮统一推进到 21:56；这表明当前 reconcile 存在非来源变化的本地写放大，可能持续刷新本地保护时间并阻止 Jira 状态/负责人覆盖。
- 全周期失败已精确归类：多个 `append Jira performance events ... conflicts with retained payload`。相同 Jira revision 的绩效快照在当前代码生成了与既有不可变 payload 不同的内容，性能证据冲突被并入核心 Jira 周期错误，导致 `SuccessfulThrough` 永不推进并反复全量补偿。
- 1004 条 issue sync state 本身没有持久错误；缺 state 的仅 HKAA-778/HKAA-779。`record not found` 是这两条首建查询的可见噪音，不能解释大量 Daily Jira 滞留，但应改为静默的可选状态读取并验证后续能创建。
- Daily Jira API 每次直接查询 DB 并在响应构建时排除 `done/archived/resolved/closed`；只要本地 status 真正变为 done，新请求不会继续返回该事项。由此前端旧显示必须来自“状态未落库”或“SSE/请求未触发”，不是服务端列表缓存。
- 症状级红灯已确定：已有 Jira 事项的 `LastUpdate` 刚被任意本地投影触碰时，Jira 返回 Done，当前 `time.Since(LastUpdate) > 15s` 守卫拒绝状态覆盖；数据库继续为 `progress`，Daily Jira 响应继续包含它。
- 同一红灯输出同时复现 363/1013/1068 的 `record not found`。这些都是代码随后按“缺行则初始化/创建”处理的可选查询，却使用会触发 GORM 错误日志的 `First`；1068 不是同步失败本身。
- 假设 1 已证实；假设 2 也由主库的 performance retained-payload 冲突证实；假设 3 已证实为独立日志噪音。假设 4（纯前端缓存）不是必要条件，后端状态已可在隔离回归中保持错误。
- 日志红灯 `TestJiraCommentSyncInitialWatermarkDoesNotLogRecordNotFound` 已稳定失败并精确捕获 1068；它同时验证首次同步结束后水位必须真实持久化，修复不能只是调低日志级别。
- 冲突样本的已保留 payload 只有 assignee/due_date/issue_type/priority/resolution_date/status；当前 producer 新增了 `parent_work_item_id`，即使为空也改变 hash。绩效评分实际从 `TaskTelemetry.ParentWorkItemID` 读取关联，不读取 snapshot payload 中该字段，因此把它塞进既有不可变事件既无消费价值又破坏幂等。
- 主库 1117 条 Jira snapshot 中已有 198 条包含 `parent_work_item_id`，其余为旧 schema；不能简单删除该字段，否则会把冲突方向反转。正确兼容策略是：先按当前 schema 幂等追加，只有命中 typed retained-payload conflict 时才用旧 schema payload 重放；两者都不匹配仍必须报错。
- 主库在 22:08 将 NS2-2110/NS2-2111/NS2-2195 的 Jira source/assignee 事实更新成功，changed_count=3；这些负责人不在 core-member 可见范围，新的 Daily Jira API 响应应排除它们。若打开页面仍显示，说明浏览器错过 SSE 后持有旧投影。
- Daily Jira 当前只在 mount、项目偏好变化和 telemetry SSE event 时加载，没有定时轮询；SSE 不提供 missed-event replay，因此断线窗口或页面订阅前发生的更新会永久停留到手工刷新/重新进入。
- 当前主服务监听 IPv6 `*:8080`，此前 IPv4 curl 失败不能证明进程不在；后续运行验证使用 `[::1]`。
- Impeccable/product 初审：这是既有高密度管理台的可靠性修复，UI 不增加任何控件、卡片、文案、颜色或动画；Daily Jira 组件继续拥有数据读取，轮询只在文档可见时运行，并在 destroy 时清理，保持当前快照直到新响应完成以避免抖动。
- design-taste-frontend 明确声明 dashboards/data tables 不属于其主要落地范围；本次只采用 redesign-preserve、完整加载/错误状态、响应式和稳定性约束，不引入其 landing-page 架构、字体、图像、motion 或视觉系统。
# 2026-08-16 Daily Jira 同步滞留修复结论

- Jira 入站 worker 确认由 `Server.Start()` 通过 `go s.startJiraSyncWorker()` 启动；主 Jira 入站周期为固定 30 秒，并在启动时立即执行一次。绩效 Jira 历史补采复用同一 worker，但按 `performance_brain.interval_minutes` 节流，当前配置为 60 分钟。
- “Jira 已完成但 Daily Jira 仍显示”的后端直接原因是 Jira 源任务也受通用 `LastUpdate > 15s` 状态保护；现在 Jira 源状态始终接受 Jira 映射状态，Git 来源仍保留 15 秒保护。
- 负责人同步此前只看本地转派 24 小时保护，不比较 Jira `updated`；现在较新的 Jira 更新时间会结束本地保护，避免人工在 Jira 修改负责人后被旧本地决策吞掉。
- 控制台 `jira_worker.go:1068 record not found` 来自首次评论同步用 GORM `First` 探测不存在水位；缺失水位本来是合法初始状态。改为 `Limit(1).Find` + `RowsAffected` 后，仍会创建水位，但不再打印误导性错误。
- 周期持续失败的真正错误不是水位缺失，而是绩效不可变 snapshot 在新增 `parent_work_item_id` 后与历史 payload hash 冲突。现在仅对 typed retained-payload conflict 回放精确 legacy payload；其他冲突仍失败，不会被吞掉。
- Daily Jira 页面原先只有 SSE，没有 reconnect/missed-event 轮询；SSE 无 replay 时页面可永久陈旧。现在保留 SSE 即时刷新，并新增 30 秒可见页轮询、单飞队列与卸载清理。
- 登录态浏览器验证时 DL-4309、NS2-2110、NS2-2111、NS2-2195 均不在表格，console 无错误，桌面/移动均无横向溢出；Impeccable detector 0 findings。
- 修复代码全量验证通过，但原 `go run cmd/server/main.go` 父子进程停止后，两次新进程启动权限审查均超时。当前 8080 无监听；必须由用户在原终端执行同一启动命令或显式批准后续启动，才能完成运行态绿水位观察。
# 2026-08-19 页面与搜索数据加载变慢深层诊断

- 当前工作树包含大量既有未提交后端、前端、数据库和规划文件改动；本轮只追加诊断证据，不回滚或覆盖用户改动。
- 诊断采用症状级计时环：页面首载与搜索必须分别记录请求数、最慢接口、TTFB/总耗时、返回体规模和渲染完成时间。
- 本机已有后端 `*:8080` 与 Vite `*:5173`；受控只读测量显示静态入口约 9.8ms、`/api/status` 约 0.85ms，服务存活与静态资源不是当前秒级等待的主因。
- Chrome 登录态实测决策看板：reload 调用约 193ms 返回，但真实 207 条 Agenda 数据到“正在刷新”消失共约 2.20s。
- 全局搜索 `HR-4202` 在约 650ms 内导航到任务表，但此时指标仍为 0、表格仅 skeleton；最终页面加载的是未过滤的 35,566 条可见事项，搜索键没有在服务端缩小初始数据集。
- 页面最终 DOM 直接包含大量任务行；全局搜索等待目标 60s 超时后，页面已显示 35,566 条，说明当前搜索流程至少包含“整表请求 + 整表客户端渲染”，需要下一步分别测接口与 DOM 放大成本。
- 任务表未把路由保存在 URL，直接 reload 会回到默认决策看板；因此真实搜索反馈环必须从全局搜索提交开始，而不能把任务页 reload 当成同一路径。
- 全局搜索进入任务表的 skeleton 后，浏览器对完成标记的普通 locator 查询在 3 秒 CDP 期限内也无法返回，当前表仍只有 8 条 skeleton 行；这不是网络断开，而是大数据装载/渲染期间页面主线程或 DOM 查询被明显阻塞的直接证据。
- 源码已确认搜索建议本身使用服务端 `/api/work-items?search=<query>&limit=8`，但选中后只切换到任务表并广播事项 ID；`TaskKanban.handleGlobalSearchSelection` 仅记录待定位 ID，不把搜索词或 ID传入加载查询。
- `TaskKanban.fetchTasks` 固定按 `limit=500`、不断递增 offset，串行拉完 `response.total` 才一次性赋给 `workItemTasks`。35,566 条数据意味着约 72 次顺序 HTTP+JSON 循环，且在最后一页之前 UI 一直只有 skeleton。
- 全量数组赋值后任务表会直接展开所有结果，再为待定位 ID 扫描 `tbody tr[data-task-id]`；这把后端分页抵消成“前端全量获取 + 全量 DOM”，是页面加载和搜索后加载共同变慢的首要结构性原因。
- 后端 `QueryPlan` 每一页都先执行完整 `COUNT`，再执行带 `ORDER BY last_update DESC, task_id ASC` 的分页查询，并额外批量查询发布关联、发布版本和最新同步状态；单页 SQL 数受测试约束在 6 次以内，但前端 72 页把它放大为约 288–432 次 SQL。
- 当前分页使用 offset，而不是游标；若排序列缺少匹配复合索引，后续高 offset 页会重复扫描/排序。下一步用主库只读 `EXPLAIN QUERY PLAN` 和实测验证。
- 主库约 334MB，`task_telemetries` 35,578 行，其中需求/Bug 35,566 行；发布关联仅 3 行、同步操作 0 行，因此关联表规模不是当前慢加载主因。
- 查询计划能用 `LOWER(TRIM(issue_type))` 表达式索引筛选类型，但分页和搜索都显示 `USE TEMP B-TREE FOR ORDER BY`；现有 `last_update` 单列索引未同时满足类型过滤与 `task_id` 次序。
- 单页 SQL 只读测量约 20ms（offset 0）、30ms（offset 10,000）、60ms（offset 35,000）；搜索 8 条约 20ms。单次并不严重，但 72 页重复 COUNT/排序后，纯 SQL 外部循环已约 3.03s。
- 任务行全部文本字段合计约 3.94MB，description 为空、decision_logs 仅 371 字节；传输量会放大解析，但 334MB 数据库大小主要来自其他表，不能把文件总大小误判为本页 payload 根因。
- 当前任务没有附着 app terminal，因此无法从本任务直接读取已运行后端 stdout；继续以登录态浏览器、主库只读查询和服务代码为证据。
- 新建只读临时探针复用真实 `deliveryplanning.QueryPlan` 和与前端相同的 500 行 offset 循环，同时对每页 JSON 编码。首跑得到 35,566 行、72 页、累计 33,055,075 字节 JSON、3,035ms；首/中位/P95/末页分别约 21/41/63/59ms。
- 同一探针的服务端精确搜索 `HR-4202&limit=8` 仅 1 条、1,604 字节、43ms。它证伪“搜索查询本身很慢”，并支持假设 1：慢发生在命中后的全量任务页加载，而不是搜索建议 API。
- 只读探针连续三次全量结果为约 3,035/3,095/2,998ms，搜索约 43/44/43ms；全量路径稳定可复现，不是一次性冷缓存抖动。
- 35,566 条赋值会触发多轮前端全量计算：filter、创建时间排序、backlog/progress/review/done/active/overdue/critical/evidence/type/project 多次过滤、负责人集合与分组、35,566 条 `mapTaskAdminRow`，然后 Svelte `{#each taskTableRows}` 直接生成全部表格行。
- `viewAssignees` 后还有按负责人逐个执行 `filteredTasks.some(...)` 的风险折叠计算，复杂度最坏为负责人数量乘事项数量；它会和全量 DOM 一起放大主线程阻塞。
- TaskKanban 在可见页面每 60 秒调用 `refreshCurrentTaskView(true)`，每个 telemetry 更新也触发同一全量加载；因此 72 请求/33MB/全量重算并非只发生在首次进入页面，会周期性重复并造成搜索或其他交互“越来越慢/反复卡顿”。
- 目录请求 `/api/delivery/directory` 与主任务加载并行，不会阻止 `fetchTasks` 本身完成；目前没有证据表明它是首要瓶颈，后续仅需做单独成本排除。
- 当前需求/Bug 涉及 490 个非空负责人、42 个项目。负责人折叠的 `viewAssignees.forEach(... filteredTasks.some(...))` 最坏可达约 1,743 万次条件检查，随后还要建立 35,566 行表格 DOM。
- 当前未提交差异没有新引入 500 行全量分页、全量 taskTableRows 或 60 秒轮询；这些在 HEAD 已存在。未提交差异新增的是 telemetry 事件触发 `refreshCurrentTaskView(true)`，会让既有昂贵全量路径从“首载/每分钟”扩大到“每次事项变更”。
- 文件历史显示当前全量 TaskKanban 主线至少自 `ecd1aaf`（2026-08-01）存在；因此“最近变慢”更可能是数据规模增长叠加新事件刷新，而不是某次把单页查询从快改慢。
- HEAD 中跟踪的数据库仅约 7.8MB，历史快照只有 700 条任务、687 条需求/Bug；当前主库约 334MB、35,566 条需求/Bug，页面数据规模增长约 51.8 倍。
- 当前 35,566 条中 34,514 条（约 97.0%）已 Done；34,406 条的本地 `last_update` 集中在 2026-08-17。页面默认把大量历史完成事项与 1,052 条非 Done 一起全量加载，正是近期性能拐点的直接数据原因。
- `task_created_at` 覆盖 2021-12 至 2026-08，35,562 条来源为 Jira；这不是 8 月 17 日一天产生的新业务，而是那天批量写入/刷新了多年历史事项后，旧的全量前端策略被数据规模击穿。
- 用两版数据库都支持的同构只读 SQL 做差分：HEAD 的 687 条/2 页约 0.01s，当前 35,566 条/72 页约 3.03s；同一加载算法在数据增长 51.8 倍后，SQL 编排耗时放大约 300 倍，符合 offset 重复排序/COUNT 的非线性退化。
- 当前服务层同一探针加入“只取非 Done”对照：1,052 条/3 页/971KB 仅约 35ms；全量为 35,566 条/72 页/33.1MB 约 2,977ms。仅历史 Done 默认纳入就把服务层查询+编码放大约 84 倍，证明默认数据边界是关键根因。
- 更深的领域边界原因已定位：`syncPerformanceJiraHistory` 为人员绩效取“已完成/到期”的历史 Jira 样本，却把每条历史事项直接 create/save 到共享 `task_telemetries`。TaskKanban 同样把该表当运营任务目录读取，导致绩效分析样本进入默认任务页面。
- 绩效历史 JQL 会跨已配置项目和 issue types，包含核心成员近期 Done/到期事项，以及一定窗口内创建、更新或解决的全部 Bug；旧事项只要近期更新/解决也会被导入，所以 `task_created_at` 可追溯到 2021 并不矛盾。
- 绩效历史同步对新导入事项设置 `LastUpdate=now`，对字段变化的历史事项也把 `LastUpdate` 更新为当前同步时刻；这解释了 34,406 条集中在 8 月 17 日，并使运营任务表的默认 `ORDER BY last_update DESC` 首先处理/展示大批历史完成项。
- 因此根因不只是“缺分页”：同一持久化表混合了运营工作项目录与绩效历史样本，两者默认读取边界不同。即使只优化索引，TaskKanban 仍会错误地消费 34,514 条 Done 历史样本并全量渲染。
- 影响已扩展到全站后端：Jira worker 每 30 秒先无界读取全部本地 Jira 任务，再把主 JQL 未命中的本地 key 每 50 个组成一个 reconciliation JQL 串行搜索。35,562 个 Jira key、主轮约 187 个 key 的上界约 708 批/周期。
- 源码存在 `shouldIncludeInJiraKeepAlive`（非 Done 或 Done 且 14 天内）但生产构造 reconciliation JQL 的路径没有调用它；该函数目前只被单元测试直接测试，形成“测试通过但真实调用遗漏”的缝隙。
- 即便现在接上 keep-alive 判断，绩效历史同步把历史 Done 的 `LastUpdate` 更新为同步时刻，当前 34,514 条 Done 全都落在 14 天窗口内，短期仍会进入 keep-alive；需要把运营活跃时间与同步/绩效样本时间分开。
- 已观测 `/api/status` 的一次 Jira 周期从 12:42:47 到 12:43:07，约 20.0 秒；在 30 秒 ticker 下 worker 约三分之二时间处于同步周期。这与数百个 reconciliation 批次的结构性放大一致，可解释不只任务页、连其他页面 API/搜索也间歇变慢。
- 测试缺口已确认：helper 单测能证明 30 天前 Done 应退出 keep-alive，但 `buildJiraReconciliationJQLs` 的集成测试只验证项目、去重、overlap 和 50 条批次；真实 sync 测试只覆盖 active Jira 与排除 GitLab，没有把 old Done 放进生产调用链。因此未使用 helper 的缺陷没有被任何症状级测试捕获。
- 21:02:47 再次只读查看状态时，worker 已开始新一轮 syncing，而上一轮 21:02:38 才完成；30 秒 ticker 按固定节拍运行，约 20 秒周期结束后只空闲约 9 秒即进入下一轮，后台接近连续占用。
- 当前 `GET /api/work-items` 使用普通 `writeJSON`，只设置 JSON Content-Type；只有 Solution 系列路由包了 gzip。Vite 本地代理也没有 API 压缩配置，所以任务页累计约 33.1MB JSON 在本地开发链路中没有应用级压缩。
- 压缩只能减少网络字节，不能消除 72 次串行请求、288–291 次 SQL、前端多轮 O(N)/O(N log N) 计算和 35,566 行 DOM；它是次级优化，不是根因修复。
- syncing 状态下 `/api/status` 自身 TTFB 仍约 1ms，说明 worker 没有把所有 HTTP 全局阻塞；“全站慢”应理解为共享数据/查询与高频后台负载的放大风险，而任务/搜索链路的实测根因仍是明确的全量工作项路径。

# 2026-08-19 Daily Jira 源同步与决策写回

- Daily Jira 原“刷新”只 GET 本地投影；即使 Jira 已改负责人或评论，只要后台 worker 停止、失败或尚未完成，重复刷新仍会读取同一份旧数据。
- 新 `POST /api/decision/daily-jira/sync` 直接复用入站同步并与 30 秒 worker 互斥，完成后再返回水位结果；失败返回 502，不再用成功提示掩盖旧投影。
- NS2-2262 回归证明：主 JQL 不再命中后，补偿查询仍能拉取最新负责人、报告人和评论；负责人离开审计范围后，下一次 Daily Jira GET 的 total 为 0。
- Jira 报告人同时保存显示名和可写回用户名；“指回报告人”不受当前核心成员目录限制，但缺失 Jira 用户名时明确拒绝。
- 决策评论在 UI 和 API 均禁止空白；Jira 启用时，评论和负责人通过单个 issue PUT 写回。Jira 失败返回 502，TaskTelemetry、DecisionEvent、DailyJiraDecision 均不提交。
- 当前配置的 Jira 认证在只读 NS2-2262 探针中返回 401，因此本轮没有真实 Jira 写入证据；本地接口、Jira HTTP fixture、构建态浏览器 mock 和全量测试均通过。
# 2026-08-21 Daily Jira 样式、滚动稳定性与刷新性能修复

- 用户截图显示右侧“早会决策”区域中，左侧“快速转派”选择器和右侧“负责人去向”选择器未共享同一顶部基线；“新的负责人”落在下一行左侧，形成不清晰的两列网格语义。
- 截图中的列表和右侧检查器都出现独立滚动条；页面抖动需先确认真实 scroll owner、滚动条 gutter、选中项保持与刷新替换方式，不能仅凭静态截图归因。
- 刷新卡顿需要分别量化网络/服务端 SQL、JSON 体积、Svelte 更新和 DOM 布局成本；索引与聚合/分组优化必须以当前查询和 EXPLAIN 证据为准。
- 主库只读统计：`task_telemetries=35,636`，`source=jira=35,620`，未解决 Jira 为 `1,048`；当前 `handleGetDailyJiraAudit` 在项目范围后直接 `Find(&tasks)`，实际是全宽全量读取，再在 Go 中做 source/status/visibility/bucket 过滤。
- 现有活动事项部分索引 `idx_task_tracking_active_last_update` 可服务未解决状态谓词；对拟议的 source + unresolved 投影查询，主库 `EXPLAIN QUERY PLAN` 已选择该索引。先修查询边界，无证据支持再新增重复索引。
- 当前库 `decision_events=9`、`daily_jira_decisions=0`，事件/最新决策分组不是当前主要成本；不能用复杂窗口/聚合替代已证明的全量任务读取主因。
- 前端 `loadAudit` 每次 30 秒轮询/SSE 都并行请求 audit、Jira link、delivery directory，且无条件 `audit = nextAudit`。后两者是辅助目录/config 数据，不应随每个数据快照重复加载；相同业务快照不应触发列表 DOM 重算。
- 布局错位的确定性原因：决策动作 `Select label=""`，同排的“负责人去向”有标签；共享 Select 仅在 label 非空时渲染标签，所以控件顶边相差一个标签行。
- 滚动稳定风险：桌面大面板有 `backdrop-filter: blur(16px)`；861–1180px 的后置规则把固定高度网格中的 inspector 改成 `overflow: visible` 且未清理早期三列 areas；<=860px 表格高度使用 `62dvh`，移动浏览器 chrome 会使其滚动中改高。
- 后端根因已确认并修复：旧 handler 对 `task_telemetries` 做全宽 `Find` 后才在 Go 里过滤 Jira/完成态；新查询先用 SQL 过滤 source 与 unresolved status，只投影 Daily Jira 需要的字段。回归捕获到旧实现执行两次 task query、物化 203 行；修复后只执行一次并物化 3 行。
- 主库只读对照从 35,636 行、约 7.75 MiB 全字段文本降至 1,061 行、约 0.20 MiB 投影文本，降幅约 97.4%。未解决 Jira 的业务候选为 1,048，额外 13 条 legacy/source 空值在既有候选逻辑中继续排除。
- `EXPLAIN QUERY PLAN` 证明现有 `idx_task_tracking_active_last_update` 部分索引已覆盖 unresolved 谓词；`decision_events=9`、`daily_jira_decisions=0`，其临时排序/分组规模太小，不是刷新卡顿来源。没有为了“看起来优化”新增重复索引或复杂窗口查询。
- 前端卡顿的独立放大器是默认 7 日桶有 982 行并全部挂载，实测 13,052 个 DOM 节点、内部滚动高 51,099px。固定 52px 行高虚拟化后仅挂载 28 个数据行、634 个 DOM 节点，滚动高仍精确保持 51,099px。
- 刷新稳定性通过业务快照指纹修复：忽略仅代表响应时刻的 `generated_at`，相同业务数据不替换 `audit`；同时仍更新“检查于”时间。Jira 链接配置与交付目录只在初载/重试加载，不再被 30 秒轮询和 SSE 重复请求。
- 自动刷新真实浏览器证据：等待一个 30 秒周期后“检查于”从 17:32 更新到 17:33，`scrollTop=320`、首个可见 key、28 个挂载行、scrollHeight/clientHeight、主面板与 inspector 矩形均未变化。
- 滚动抖动修复边界：移除两个大滚动面板的 `backdrop-filter`；<=1180px 明确单列 grid areas 和 inspector scroll owner；<=860px 用 `clamp(420px, 62svh, 560px)` 取代会随移动浏览器 chrome 波动的 `dvh`。
- 快速转派错位修复边界：为动作选择器补“决策动作”标签，与“负责人去向”共享基线；“新的负责人”独占整行，<=520px 转为单列。1440/1180/860/760/480px 实测对齐且无文档级横向溢出。
- 隔离浏览器三次 Daily Jira mount 为 423/420/410ms；深滚到 38,304px 后仍仅挂载 30 行且面板几何、滚动范围稳定。
- 全量验证通过：`go test ./... -count=1`、`go vet ./...`、前端 63/63 tests、`svelte-check` 0 errors（86 个既有 warnings）、production build、Impeccable 全量/布局 detectors、Finesse detector 和 `git diff --check`。
- 隔离服务使用临时数据库副本并关闭 Jira/GitLab/AI，验证后 server、Vite 和临时目录均已清理；未重启主服务、未写主数据库、未写真实 Jira。

## 2026-08-21 extension: 千万量级有界读取架构

- 当前优化仍不是有界接口：`GET /api/decision/daily-jira` 会查询所有未解决 Jira，再在 Go 中做权限过滤、候选识别、事件/决策 `IN (...)`、桶分类和全量排序，最后把三个桶全部返回。数据继续增长时，后端物化、`IN` 参数、JSON 和前端数组都会重新线性退化。
- 现有接口把查询、权限、桶计算、排序、事件关联、计数和 JSON shape 全塞在 handler 中，是浅实现集群；千万量级需要把“稳定读取一页”做成 deep module seam，让 handler 和 Svelte 不知道 SQL、游标与索引细节。
- 当前前端虚拟化只降低 DOM，不降低网络与 JSON：`filteredItems` 仍来自 `activeBucket.items` 全量数组，搜索也只在浏览器本地 contains 过滤。千万量级必须改为服务端搜索和 cursor-backed page window。
- 当前视觉排序是 `overdue DESC, age DESC, recent activity ASC, task_id ASC`。`age DESC` 等价于 `task_created_at ASC`，但 overdue 和自然日 bucket 是随日期变化的派生键；若不物化/增量维护这些排序键，单靠普通复合索引无法保证深页 keyset 与现有顺序同时稳定。
- `TaskTelemetry.Revision` 是每行 revision，不是跨行稳定快照水位；跨多个 HTTP page 的一致性不能靠 `MAX(last_update)` 临时扫描。需要由写路径或数据库触发器维护全局 generation，cursor 携带 generation，变化时明确失效而不是悄悄重复/漏行。
- Summary 不能在 10M 活跃行上每次 `COUNT/GROUP BY` 后仍承诺毫秒级。应维护按 project/assignee/bucket 的小型计数投影，页面读只做有限维度求和；自然日跨桶和 overdue 变化通过按 `next_transition_day` 索引的增量 rollover 处理。
- 现有事件索引支持 task lookup，但一次页面只应为最多 page-size 个 task IDs 查询最近事件/决策，不能再对整个桶构造 `IN`。
- SQLite 版本为 3.43.2，启用了 `ENABLE_FTS5`，可把三字及以上的编号/标题/项目/负责人 contains 搜索放到 trigram FTS；短查询应走 task/project/assignee 的前缀索引，避免在 10M 行上回退到 `%LIKE%`。
- 现有 GORM `TaskTelemetry` 写入入口分散在 Jira worker、决策、交付与其他模块；仅在 Jira worker 手动维护投影会产生漏同步。数据库级 task insert/update/delete trigger 是更可靠的内部实现，调用方无需知道投影存在。
- `TaskTelemetry` 时间字段由 SQLite datetime 保存；读取索引与 cursor 不应直接依赖带时区字符串的字典序，投影应保存 Unix 秒/自然日整数作为稳定排序键。
- 项目权限已有 `project_key` 加 task ID 前缀兼容路径；读模型可在写时归一化 project key，避免每页执行 `UPPER/TRIM/LIKE` 表达式并让复合索引失效。
- 10M 目标将限定为“10M 投影行中的有界页读取”，页大小上限 100，warm p95 query <20ms、handler <50ms；首次迁移回填、午夜 rollover、网络与并发另列指标，不混进 page-read 承诺。
- 能力探针纠正：系统 `sqlite3` CLI 有 FTS5，但 Go 应用驱动默认没有，只有 `sqlite_fts5` build tag 才启用；仓库没有统一 build wrapper/GOFLAGS。核心架构不能强依赖 FTS。最终采用 built-in B-tree 的编号/标题开头/项目/负责人/状态前缀索引，若部署显式启用 FTS5 再自动升级为 trigram contains；响应必须暴露实际 search mode，避免语义不透明。
- 最终读模型以物化 `sort_overdue` + `(sort_overdue, created_unix, activity_unix, task_id)` 行值 seek 消除深页前缀扫描；100k 尾页对照从约 4.2ms p95 降到约 0.13ms。
- 首次迁移先批量物化 entries/counts，再创建二级索引；v2→v3 会重建依赖 `sort_overdue` 的 page/search indexes 与 task projection triggers，避免旧触发器继续写错排序键。
- 页面富化不再对全桶或全历史构造查询；最多 100 个 task IDs，事件按任务最多 8 条、提醒按任务最多 1 条，并由 `(task_id, action, created_at DESC, id DESC)` 与 `(task_id, created_at DESC, id DESC)` 覆盖。
- 自动刷新在已加载多页时会从同一 generation 重新走游标到当前加载前沿，完整后一次性替换；不会把新首页与旧尾页拼成混合快照。
- 10,000,000 行、100 行页、warm、500 samples 实测：Reader 首页面 p95 0.824ms、cursor 页面 0.845ms、精确 Jira key 0.367ms、标题前缀 0.525ms；原始 midpoint/tail seek p95 0.115/0.112ms。
- 6.25GB 合成库的行装载约 26.8s、索引构建约 69.8s，证明首次物化不是在线请求；生产必须先备份、对等数据副本演练并安排迁移窗口。
- 登录态隔离浏览器在 260 行深滚动处经历真实 generation 变化与 30 秒自动刷新：`scrollTop=13144.5`、`scrollHeight=13555`、左右面板高度 `519.5/519.5` 均不变，精确搜索随后读到新标题。
- 毫秒结论只覆盖本机 warm SQLite read module；生产 HTTP p95、并发锁等待、冷缓存、网络和低选择性广泛搜索必须通过部署后监控确认。
