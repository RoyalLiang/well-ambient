# Session: 2026-08-27 - 引导页减负、维护库渐进展示与 SQLite 迁移进度

## 2026-08-27 品牌图标替换进度

- 已完成项目冷启动、coding.complex/tool-routing/delivery 规则加载与 planning-with-files 建档。
- 已读取项目 Impeccable、design-taste-frontend、finesse-ui、imagegen 约束，并完成三方评审。
- 已确认目标引用：`web/index.html` 的 Vite 默认 favicon 与 `FunctionalAdminShell.svelte` 左轨 `wa` 占位标记；尚未编辑前端文件。
- 已生成并收敛 `web/public/brand-mark.svg`，`web/index.html` 与共享 `FunctionalAdminShell` 复用同一资产；旧 Vite `favicon.svg` 已移除。
- 品牌合同 2/2、`pnpm check` 0 error、`pnpm build`、`git diff --check` 通过；Impeccable `[]`，Finesse P0=0，P1 为目标文件既有双 sticky 检测项。
- 浏览器已检查 1280 展开、94px 折叠、390/320 移动抽屉，以及 16/32/42px 在浅/深背景的可读性；临时响应式夹具已删除，5177 验证服务待停止。

- **Phase 1 status:** in_progress
- 收到真实迁移失败截图后进入追加诊断：确认 `notifications` 仅是最后记录表，失败阶段被错误覆盖、服务端根错误未记录；未自动重跑数据库写操作。
- 已对 SQLite 源表做只读结构/数据质量检查，暂无 NULL、超长、空时间或 NUL 证据；下一步先补失败阶段保留、安全 SQLSTATE 日志、本地 setup 默认和 SSL 原生 select 样式合同。
- 三条追加红灯已转绿：失败 stage/table/count 保留且 SQLSTATE 双 `%w` 贯通；本地示例默认 setup；SSL 原生 select 的统一 SVG chevron 通过合同与双设计检测。
- 浏览器无写入夹具验证失败态 1280/390/320px 均 active“迁移 SQLite”、零横溢；SSL 触发器 44px、箭头不遮挡。夹具已停止，真实 setup 后端已用最终代码恢复在 127.0.0.1:18197，5175 页面回到空凭据初始态。
- 最终回归：目标前端 15/15、全量前端 166/166，db/config/server Go 包通过，Svelte/TypeScript 0 error，production build 成功，Impeccable `[]`、Finesse findings 为空、diff check 通过。
- 现场根因边界：旧进程没有保留真实 PostgreSQL SQLSTATE，当前只能确认 `notifications` 批量复制边界且源表静态质量正常；下一次由用户填写凭据重试时，页面与服务日志会返回真实五位 SQLSTATE，系统不会自动重跑。
- 已启用 planning-with-files、diagnosing-bugs，以及项目强制的 Impeccable、design-taste-frontend、finesse-ui 三方 UI 审查；完整读取 onboarding/product、product workflow、palette、移动底线、反廉价与交付检查参考。
- 已读取既有 Finesse 记录：上一版数据库安装页为居中工作流、低动效、中高密度；本轮保持相同产品语法，只进一步减文案、渐进展示高级字段并补齐真实迁移状态，不做另一个风格化页面。
- 下一步读取当前组件、setup server 与测试，先建立三个能准确失败的症状合同；在三方会审记录完整前不编辑 frontend 业务文件。
- 已确认 UI/后端已有迁移选择与轮询骨架，定位测试环境丢失及全流程进度映射缺口。
- 已冻结三方评审结论、响应式范围与验证边界；尚未编辑任何 frontend 业务文件。
- 用户确认 Design Read；进入 Phase 3，先以三个源码合同建立红灯，再修改唯一 UI owner。
- 三条目标合同先准确失败，实施后 `database-setup-contract` 13/13 通过；维护库渐进展示、SQLite 预告和端到端进度均已锁定。
- `pnpm check` 为 0 errors（148 条既有 warning 来自其他 11 个文件）；`pnpm build` 成功，现有大 chunk/历史 Svelte warnings 未扩大处理范围。
- `internal/db` 与 `internal/server` 的 SQLite/安装定向 Go 测试通过；Impeccable 复扫为 `[]`，Finesse 为 0 P0。
- 真实浏览器完成待测试首屏、连接成功、SQLite 迁移/跳过选择、43% 复制进度、默认桌面/390/320px 与 console 验证；临时状态桩已停止。
- 已恢复真实隔离 setup 后端 `127.0.0.1:18197`，Vite 页面保留在 `127.0.0.1:5175`，识别一致性 SQLite 快照且未填写任何密码/令牌。

---

# Session: 2026-08-27 - 正式居中引导、配置数据库化与 Docker 瘦身

- **Phase 5 status:** completed（反思门禁后按用户要求开启隔离本地引导环境）
- 已读取项目冷启动、复杂编码/数据库安全/规划规则与现有 `DESIGN.md`；启用 planning-with-files 和 diagnosing-bugs。
- 已完整加载项目 Impeccable 主技能与 onboard/product 参考、design-taste-frontend、finesse-ui 主技能及 product/workflow/redesign 参考；完成初始三方方向记录，尚未编辑任何 UI/frontend 业务文件。
- 已运行 Impeccable 项目上下文探测：`web` 目标存在，项目无 `PRODUCT.md`、有 `DESIGN.md`，因此按既有页面定点整改继续。
- 已记录初始脏工作树：`Dockerfile`、`Makefile`、`task_status.md` 已修改；下一步先核对 diff、引导页入口、配置 API/数据库模型与 Docker 分层反馈环。
- 已记录一次技能/上下文聚合输出截断，并改用分段读取；未重复同一失败方式。
- 已定位首次安装页真实 owner `DatabaseSetup.svelte`，并以源码确认桌面双栏、左侧大标题叙事与右侧完整工作区是用户反馈的直接根因。
- 已完成并记录强制三方 UI 会审：不新增欢迎 step，不隐藏必需字段；改为所有断点单列居中，保留现有两步业务状态机、安全边界与交互。
- 已逐项核对 SettingsPanel 及其独立配置 section：中心配置当前仍写 YAML，数据库中的 ConfigVersion 只是脱敏历史；决定新增 current/runtime config 单例，数据库连接留作文件 bootstrap，版本历史继续脱敏。
- 已核对 Dockerfile 用户 diff 与 `.dockerignore`：server runtime 误用完整 Go 工具链镜像是主因，大体积 `.git`/数据库/依赖目录均已排除；本机无 Docker 兼容运行时，实际字节验收需标明环境缺口。
- 下一步先增加 UI、配置持久化和 Docker 运行时三条症状级红灯，再实施最小修复。
- 三条红灯分别准确失败于双栏 CSS、`golang` server runtime 与 `db.RuntimeConfig` 缺失；新增实现后目标 UI/Docker 合同 10/10、配置/Schema 定向 Go 回归通过。
- 配置保存已改为同一事务创建脱敏 `ConfigVersion` 并 upsert 单例 `RuntimeConfig`，事务成功后才更新内存；新增回归证明启动 YAML 不被改写、runtime 表失败时版本插入也会回滚。
- 一次只读存储检索因不存在 glob 被 zsh 拒绝；已按 Self-Improving 改为 `rg -g`，并确认设置面板中心配置统一走 `/api/config`，方案提示词、项目映射、用户/权限/策略等独立 section 原本已使用各自数据库模型/API。
- 引导页已改成正式居中的单列工作流，保留原两步字段、连接测试、失败/成功、迁移/跳过和重启恢复状态机；三套 UI 检测的目标 findings 均为 0。
- 新增 `runtime_configs` 单例 current row：启动优先从数据库恢复设置，数据库连接继续使用 YAML bootstrap；手动保存/回滚同一事务写 current row 与脱敏版本历史，普通设置保存不再改写 YAML。
- `--migrate-only` 已包含配置同步。在主库精确副本上用新构建执行真实迁移，legacy v28 成功变成 current v28，payload 与最新历史同步且不含 database bootstrap；主库因旧 8080 实例仍运行而未在线改动。
- Docker server runtime 已从误带完整 Go 工具链恢复到 `debian:bookworm-slim`，web runtime 继续使用 nginx alpine；`.git`、约 400M 数据库、node_modules、dist 与备份均已被 `.dockerignore` 排除，未做破坏性本地清理。
- 浏览器在 1440/1024/760/390/320 实测单列居中、零 document 横溢；连接失败/成功、第二步迁移选择、可见键盘焦点与 console 0 error/warning 通过。隔离 fixture、Vite、临时数据库副本、配置和二进制均已停止并删除。
- 最终自动验证通过：安装/Docker/迁移合同 11/11，Svelte/TS 0 errors、production build、Go 全量、Go vet、diff hygiene、Impeccable `[]`、Finesse P0=0。沙箱内 Go 的 `httptest` 端口限制由受控同命令沙箱外重跑解决。
- 本机无 Docker/Podman/nerdctl/Finch/Colima/Lima/OrbStack，无法实测最终镜像字节数；代码级根因和 runtime 边界已闭环，但不得声称精确 post-fix MB 或已低于某一阈值。
- 用户在反思门禁后要求本地测试环境开启引导页；已用 `/tmp/well-ambient-setup-local.yaml` 启动 setup-only 后端 `127.0.0.1:18197` 与 Vite 前端 `127.0.0.1:5175`，不改主配置/主库/8080 服务。浏览器已打开并保留该页，DOM 标题为“完成运行数据库配置”、9 个表单控件、无横向溢出，setup status 为 `setup_required=true`，console error/warning 为空。

---

# Session: 2026-08-26 - 编辑方案保存时 Toast 统一与闪烁修复

- 已读取项目冷启动规则、复杂编码/内存约束、planning-with-files、diagnosing-bugs、项目 Impeccable、design-taste-frontend 与 finesse-ui 主技能；大型技能首次聚合读取被截断，随后分块完整读取并记录错误。
- 已运行 Impeccable 上下文探测；项目无 PRODUCT.md，但有 DESIGN.md，按 scoped fix 路径继续，不进入 from-scratch init。
- 已查看并文字化用户截图：弹窗正文顶部绿色内联成功提示与全局 Toast 基线不一致；编辑器还存在“内容已保存”状态，需建立两个症状各自可证伪的回归。
- 已快速复核相关项目记忆，确认历史共享反馈 owner 为 ToastHost/showToast；当前尚未编辑任何 frontend 文件。
- 已完成 Impeccable、design-taste-frontend、finesse-ui 三方会审并写入任务计划：共享 Toast 统一反馈，MarkdownWorkbench 保存期间持续挂载，只有可恢复的冲突/放弃状态保留内联。此节点后才允许编辑 frontend/test 文件。
- 已在 `solution-entry-contract.test.ts` 建立保存症状级红灯（9/10），随后修改 `SolutionWorkspace.svelte`：保存成功/普通失败改走全局 Toast，移除编辑 modal 内成功条幅，冲突保留可恢复内联块。
- 首轮绿灯发现 `saveDraft` 仍清空旧 `notice`，回归有意保持红色；删除无渲染用途的耦合后目标合同 10/10 通过。下一步运行相关/全量前端、类型、构建与设计检测。

# Session: 2026-08-26 - 任务跟踪第二行长条卡片样式统一

# Session: 2026-08-26 - 任务跟踪活跃事项摘要与语义底色

- 已读取项目 mandatory UI gate、Impeccable 产品约束、design-taste-frontend preserve 约束、Finesse 组件范围/产品 UI/产品配色/重设计/移动底线，以及 planning-with-files 和 Self-Improving。
- 已查看用户原始 3840×1736 截图，确认历史总量与完成量占据最高视觉权重、活跃数被截断、需求/Bug 口径混排、子项缺少语义底色。
- 已完成相关项目记忆快速复核：共享改动应落在 `TaskKanban.svelte`；活跃数必须复用 QueryPlan 的 ActiveOnly 聚合，不能恢复全历史加载。
- 当前只更新计划/发现/进展记录，尚未编辑任何 UI/frontend 文件；下一步核对当前源码的数据字段、排期治理同类底色和项目 token，再完成三方会审。
- token 搜索首轮因模式以 `--wa-` 开头被 `rg` 当作选项；已记录并切换到 `rg --`，不重复失败形式。
- 已完成共享组件、后端 QueryPlan 聚合、现有排期治理带与 `--wa-*` token 审计；确认需求/Bug 当前是全历史聚合，阶段数是活跃聚合但百分比错误地以历史总量为分母。
- 三方 UI 审查已写入任务计划并一致通过：状态页收敛为 6 项纯活跃摘要，复用既有 soft token 底色；执行追踪保留 4 项原语义。本节点以后才允许编辑 UI/frontend 文件。
- 已先修改定向服务/UI 合同建立红灯；首次执行分别被 Go 默认缓存写权限和缺失 tsx runner 阻断，尚未进入产品断言。已记录并改走 `/tmp` Go cache 与项目现有测试脚本。
- 第二轮环境核对显示 Go 还需要可写/可读模块缓存，Node 22 需要显式 TypeScript strip flag；继续先解析仓库已验证命令，不把环境失败误报为测试红灯。
- 已取得真实红灯：Go 编译失败于两个 active 类型字段不存在；UI 目标合同失败于仍为 8 列透明摘要。实施后两者均转绿，Go 定向 1/1、管理台目标合同 4/4。
- 实施边界为 5 个目标文件：deliveryplanning summary model/query/test、TaskKanban 共享条带、管理台源码合同。历史 `requirements/bugs` 字段保留，新 UI 使用新增 active 聚合；执行追踪指标语义不变但复用新的 soft 背景规则。
- 定向 Go 包、目标/全量前端合同、Svelte/TypeScript、production build、diff hygiene 和两套设计检测均已通过：1/1、4/4、146/146、0 errors、Impeccable `[]`、Finesse P0=0。
- Chrome 的两个本地页面均为未登录状态，输入框没有安全预填；已停止在认证边界前，等待用户在现有标签页完成登录后继续真实任务表/人员负载/执行追踪与 1440/1024/760/390/320 验收。

---

# Session: 2026-08-26 - 任务跟踪第二行长条卡片样式统一（上一任务）

- 已完成项目冷启动规则、复杂编码/规划/交付约束读取，并使用 planning-with-files 维护当前任务证据。
- 已完成 Impeccable、design-taste-frontend、finesse-ui 三方审查；共同方向是共享 wrapper 单一玻璃表面、透明分隔单元、单行高密度与条带内横向滚动。
- 已在 `TaskKanban.svelte` 共享 owner 落地任务表/执行追踪统一条带，并在既有管理台契约测试中锁定结构、8/4 列、透明子项、72px、阶段标签单行和窄屏滚动。
- 端口探测第一次误用 zsh 只读变量 `status`；已按 Self-Improving 做有界复盘并切换变量名，产品与服务未受影响。
- 目标契约 4/4、全量前端 141/141、`pnpm check` 0 errors、production build、diff hygiene、Impeccable `[]` 均通过；Finesse `p0=0`，剩余均为目标区外既有 P2。
- 已登录 Chrome 对照排期治理并验证任务表、执行追踪：1440/1024/760/390/最小移动宽度均为单层 72px 条带，子项透明且无圆角/阴影，横向溢出只留在条带内，document 无横溢，console error=0。
- 移动端首轮发现阶段标签换行导致 status 84px；补充精确红灯并实施 `nowrap/ellipsis` 后复验为 72px。临时后端、数据库快照与配置已停止并从 `/tmp` 删除，主库及外部系统未修改。
- 当前代码和运行态证据已完成，进入一次性交付反思门禁等待用户反馈。
- 用户已确认只要求样式同步、无需单元数量同步；现有 8/4 项结构符合边界，无需追加代码，反思门禁已完成。

---

# Session: 2026-08-23 - Daily Jira 负责人变更自动退出列表

- 已加载项目冷启动、复杂编码/内存规则、CONTEXT、相关 Jira ADR，以及 diagnosing-bugs、planning-with-files；Serena 已激活并读取初始说明。
- 已复核最近两轮 Daily Jira 记忆：自动 worker/事件广播是主路径，手动按钮只作恢复；上一轮代码本地通过但运行服务未重启、真实 Jira 未验证。
- 已记录任务前脏工作树，尚未编辑业务代码或触碰主服务/主数据库/真实 Jira；正在建立 NS2-1986 等价的确定性红灯。
- 已捕获运行态红灯：NS2-1986 手动轮成功更新，但 checkpoint 的相邻入站轮间隔约 7 分钟，且随后再次停滞；源码证明全量绩效 Jira 历史回放内联阻塞 30 秒入站 worker。
- 已新增先红后绿回归 `TestJiraInboundWorkerContinuesWhilePerformanceHistorySyncIsBlocked`，将入站与绩效历史拆成独立 context worker；负责人离开范围、广播、手动同步和 FZ-2257 幂等定向用例全部通过。
- `go test -race` 新并发回归通过；新增 stale 状态回归，漏掉 4 个 30 秒周期后 `/api/status` 标记 stale。`go vet ./...` 已通过，正在执行全量回归与清理。
- 全量 `go test ./... -count=1` 通过；新 worker/stale 回归连续 30 次通过，`git diff --check` 通过。Serena 生成缓存已精确清理，不再出现在工作树。
- 当前代码验证完成；仍需在用户反馈后决定是否受控重启本机 8080 服务，并以真实运行态 checkpoint/认证页面完成最后闭环。
- 用户回复“继续交付”后已执行受控重启；新服务连续完成多个 30 秒 Jira 周期，`/api/status` 保持 healthy，192 项、无错误。
- 运行态发现并修复绩效评分的 35,660 ID 单次 `IN` 参数溢出；新增 500 条分批读取、覆盖索引和大范围回归。
- 首次重启验证捕获 Jira/绩效同秒启动的 SQLite 写锁竞争；绩效 startup 错峰 5 秒后再次启动，Jira 首轮先成功，绩效 startup 随后 completed 并生成 14 个快照。
- 最终全仓 `go test ./... -count=1`、`go vet ./...`、关键 Jira 30 次重复、两个 race 回归、数据库/绩效包测试和 query-plan 合同均通过；当前 8080 运行最新源码。
- 登录态浏览器仍显示登录页，未绕过认证做页面点击；NS2-1986 的负责人投影、核心范围排除和通用页面可见性回归已形成样例级闭环。

---

# Session: 2026-08-19 - 页面与搜索慢加载闭环

# Session: 2026-08-21 - Daily Jira 滚轮跳底与方案发布 URL

- 已加载项目复杂编码/内存/浏览器规则，以及 diagnosing-bugs、planning-with-files 和强制三方 UI 技能。
- 已冻结 preserve-mode 设计方向和两个独立验收契约；当前处于精确复现阶段，尚未编辑业务代码。
- 旧浏览器 tab 已失效并按规则换成新 tab；隔离 fixture/Vite 默认绑定被沙箱拒绝，等待受控本机环回启动。
- 隔离 fixture 与 Vite 已在独立 18192/4177 端口启动；经 fixture 登录进入 Daily Jira，100 行数据、无续页和无刷新干扰的红灯环境已就绪。
- 真实 CUA 滚轮红灯已捕获：420px 初始滚动正常；再输入 120px 跨过虚拟 overscan 边界后，滚动在 600ms 内自行级联至 4824.5px 底部。首行 52px、总高度恒定，H1 滚动锚定成立，H2/H3 被排除。
- 发布配置链已核对：保存后运行时配置原位生效；报错不是热更新，而是缺省 URL 只生成相对链接。实施将保留显式配置优先，并使用浏览器 `Origin` 作有限回退。
- 两次旧版输入 API 与一次 `tab.ax` 能力探测失败均未改变页面；已切换到当前标签支持的 `cua.scroll` 并记录真实输入证据。
- 后端实现已返回 200 和正确链接；HTTP body 与 Outbox 的原始字符串断言先后被 JSON 的 `\u0026` 标准转义击中，均已改为 JSON 解码后的字段级断言，未改变产品代码。
- UI 最小修复已落地：只在 `.audit-table-shell` 增加 `overflow-anchor: none`；1024/760/390/1440 四个已登录断点均以 420px+120px 真实滚轮稳定停在 540px，计算样式均为 none，文档 `scrollWidth == clientWidth`。
- 方案发布已改为“有效显式 `server.public_url` > 合法单值 HTTP(S) Origin > 相对链接并拒绝”；不读取 Host/Forwarded。成功用例返回 200，拒绝用例仍 422，下一步补强草稿/Outbox 副作用断言并跑全量验证。
- 发布副作用回归已补强并通过：422 时草稿仍为 draft、Outbox=0；200 时生成唯一 pending Outbox，解码后的 link 与响应一致。
- 最终验证通过：完整 Go（受控 loopback）、Go vet、70/70 前端契约、Svelte/TS 0 errors/86 既有 warnings、生产 build、Impeccable `[]`、Finesse P0=0/目标 findings 为空。
- 登录态 1440/1024/760/390 四断点的真实滚轮、容器宽度和总高度验证通过；浏览器视口/页签、18192/4177 服务与临时 fixture 已清理，未触碰主服务、主库或外部系统。

---

# Session: 2026-08-21 - 全站千万量级数据访问架构

- 全站矩阵已收敛为 77 GET API、24 个页面/配置状态 + shell、9 个 generation dataset；早期 23 页口径已更正。
- 后端已接入 contract inventory、opaque scope/generation cursor、limit policy、request-scoped GORM Query/Row/Raw 观测与 query/handler/response budget breach；status 默认只返回汇总，`?read_contracts=full` 才返回 77 条声明。
- 已迁移 context facts/documents、corpus candidates、execution runs/actions、releases/project releases/release Jira issues；详情候选集最多 100 且给出 continuation，execution action 每 run 最多 20。
- 用户目录 membership 从 N+1 收敛为固定 2 条 SQL；task activity 与 demand spec 历史均硬限 100，task ID 大小写历史通过 `LOWER(task_id)` 表达式索引消除临时排序。
- 人员可见性和无配置边界时的历史负责人兜底均硬限 5,000；后者新增 partial covering index，EXPLAIN 无 table scan/临时排序。
- 前端 `PagedResource` 已覆盖 release/Jira/context/corpus 首波消费者，实现同 scope single-flight、跨 scope abort/stale suppression、稳定快照、续页去重和 409 generation 恢复；新增跨页 UI 契约 2/2、资源层 4/4 通过。
- 10M/500-sample 隔离基准通过：有界查询和本地 handler warm p95 为 0.103–0.117ms，projection lookup 0.006ms，四条 EXPLAIN 全部命中索引；临时 1.01GB 数据库已自动删除。
- 架构/矩阵/benchmark/rollout 文档已落盘 `docs/all-page-10m-read-architecture.md`；当前成熟度 17 verified / 32 bounded / 28 pending，新增单调 ratchet 防止回退。
- 隔离认证浏览器已验证发布列表与两类 Jira 列表真实 cursor 续页、服务端搜索、180ms 慢刷新期间行数和列表矩形完全稳定，以及 1440/1024/760/390 无文档横向溢出；临时浏览器页、4176/18191 服务和 fixture 已清理。
- 最终 Go 全量回归再次通过；前端契约、check/build、Impeccable `[]`、Finesse P0=0 和 `git diff --check` 通过。
- 错误恢复：小基准首次因 status/scope 校验数据奇偶性耦合无 tail anchor，修正 seed 分布后 10K/10M 均通过；首次从 `web/` 内运行测试误写 `web/tests/...`，改为 `tests/...` 后通过；用户目录查询数回归首次未 seed user，因此合法地只有 1 条 SQL，增加 3 个用户后稳定证明 2 条 SQL。
- 用户将范围明确扩展为所有页面、所有数据；已停止把 Daily Jira 局部 benchmark 当作最终架构完成声明。
- 已加载 `diagnosing-bugs`、`codebase-design`/deepening 与 `planning-with-files`；计划先建立全站页面/接口/查询/索引风险矩阵和可执行红灯，再实现公共深模块与分领域 adapter。
- 已在计划中冻结目标口径：10M 有界数据库读取 warm p95 <20ms、本地 handler p95 <50ms；生产端到端、冷缓存、并发和迁移回填必须单独验收。
- 已完成相关历史与领域词汇复核：TaskKanban、Agenda、数据资产和 Daily Jira 已各有局部有界模式；全站方案将抽取稳定页/高水位/请求合并等共同机制，并保留领域 adapter 的权限、过滤和聚合语义。
- 已复核现有数据资产 ADR 与 Daily Jira 10M 文档；修正规划中的假想存储 adapter，当前测试/基准将直接覆盖同一 SQLite 实现。
- 已完成第一轮前端可达页与请求扫描：确认至少 7 个一级工作区及多个子页/配置 section；发现共享 Agenda 仍有旁路、`/api/tasks` 与 `/api/schedule` 全量读取、无显式分页目录和多套独立轮询/请求状态实现。下一步把这些调用映射到后端路由与具体 ORM/SQL。
- 已核对导航定义，固定为 23 个页面/子页面验收单位；开始按实体列表、聚合看板、时间线、详情、低基数字典五类映射后端查询，不用单一分页方案覆盖所有语义。
- 已映射 server GET 路由和第一批 ORM 终结查询；确认 corpus/execution/solution/performance 等模块仍有无界列表、无界嵌套集合或 N+1，且路由层没有统一数据契约/预算。下一步建立能精确捕获这些形态的全站红灯清单。
- 已确定公共观察 seam 必须位于 GET route 注册处，才能同时覆盖生产 mux 和现有 handler 测试；红灯会自动检查所有 GET API contract，并对高风险页面补真实数据行为断言。
- 已新增并运行全站读取 contract 红灯；它稳定失败于公共契约实现不存在，精确覆盖全部 GET API、23 个页面状态与 20 条已知高基数路由。已向用户展示 5 个可证伪假设并继续验证。
- 已定位主库并用 SQLite 只读模式列出 59 个业务表；一次搜索误包含不存在的 `configs` 目录，已记录并改用精确现存路径，未重复同一错误。
- 主库只读计数显示绩效 audit/source 事件均已超过 20.5 万行，高于 3.56 万任务；已定位 performance explanation 的 OFFSET 与全量 active-evidence 路径，证明全站优化不能围绕单一任务表设计。
- 已确认 H2/H4：tasks、schedule、execution、KPI、risk calendar 和 task activity 均存在全量写模型读取、Go 后过滤/聚合、无界嵌套或大 `IN`；这些路径将成为第一迁移波次。
- 已完成方案、版本、上下文、审计、执行 run 的源码复核：区分了“已有 cap 但无 cursor”和“真实无界/N+1”，纠正 corpus/context documents 的初步判断；风险矩阵开始具备逐路由事实而非正则猜测。
- 已实现 `internal/readmodel` 公共深模块：contract 验证、路由观察 wrapper、响应字节与 p50/p95/p99 指标、动态 path 匹配、SSE Flush 透传；模块测试通过。下一步登记全部 77 个 GET API 并接入真实 server handler。
- 77 条 GET API contract、23 页面状态覆盖和高基数分类门禁全部转绿；真实 server handler 已接入 registry，`/api/status` 能显示已观测路由指标。Phase 2 完成，Phase 3 开始迁移 pending 领域 adapter。
- 已新增共享 CursorCodec 与 limit policy，完成先红后绿的 contract/scope/checksum/tamper/limit 回归；下一步以 context facts/documents、corpus candidates、execution runs、release/solution lists 为首批实体 adapter。

# Session: 2026-08-21 - 全局规则同步与 Daily Jira 自动同步冲突修复

- 已读取项目冷启动规则、复杂编码/内存流程、相关历史记忆与最近 Daily Jira 同步实现摘要。
- 已启用 diagnosing-bugs、planning-with-files，以及项目强制 Impeccable/design-taste-frontend/finesse-ui 三方门禁；尚未编辑前端业务代码。
- 已确认脏工作树和 bootstrap 保护行为；下一步建立 FZ-2257 retained payload 冲突的症状级回归，并核对自动 worker 到页面刷新链路。
- 主数据库只读查询确认 FZ-2257 存量事件、当前任务事实和失败的 inbound checkpoint；未修改数据库。
- 新增精确回归并看见红灯：同一 history ID/index 与相同 priority 变化，仅 author displayName 漂移即触发用户原始 retained payload conflict。
- 受控只读 Jira 查询确认 FZ-2257 当前 author displayName 已变化且 priority 业务事实未变；临时 probe 测试文件已清理。
- 已实现 retained actor replay：只豁免可变作者展示名，其他性能事件业务 payload 仍严格冲突；模块级安全回归和 FZ-2257 adapter 回归已转绿。
- 自动同步集成回归使用 mock Jira 连跑两次 30 秒 worker 同源链路，第二次作者改名后 inbound state 仍成功且账本仅一条事件；loopback 受控运行通过。
- UI 三方评审确认无需前端改动：现有按钮为辅助样式，页面已订阅 SSE 并保留可见页轮询兜底。
- 完整 `go test ./...`、`go vet ./...`、57/57 前端契约、Svelte/TypeScript 0 errors、生产 build、Impeccable `[]` 与 diff hygiene 已通过。
- canonical bootstrap 已正式执行并保护性确认项目规则存在；全局新增 reflection gate 已同步，项目专属规则未被覆盖。
- 本轮未重启主服务、未修改主数据库、未向 Jira 写入；交付状态为代码完成、运行态待受控重启。
- 真实范围只读重放通过：185 个当前 Jira issue 对 205,041 条主库 retained 事件为 0 冲突、0 新证据；临时 probe 已清理。

---

# Session: 2026-08-19 - 页面与搜索慢加载闭环

- **Status:** complete locally；主运行服务需受控重启后生效。
- 浏览器 15 秒稳定性日志进一步发现 `DecisionDashboard` 为 4 条发布事实轮询完整 `delivery-cockpit`：每轮两次读取约 35,578 条任务，随后 Git log 查询因约 3.5 万个 `IN` 参数报 `too many SQL variables`。
- 新增轻量 `/api/strongest-brain/releases` 路由并切换 Dashboard；完整驾驶舱/决策快照改为复用一次任务与用户读取，Git log 使用任务表 JOIN 和项目范围条件，不再展开 task ID 列表。专用路由、JOIN SQL 形状、既有驾驶舱/决策中心和前端消费契约定向回归已通过。
- 已继承前两轮确定性基线：任务页 35,578 条/72 页/33.1MB 默认加载、服务端精确搜索约 43-46ms、Agenda 两个 15 秒调用方，以及 Jira 历史批次风险。
- 已启用 diagnosing-bugs、planning-with-files 和项目强制 Impeccable/design-taste-frontend/finesse-ui；正在完成三方 product/preserve-mode 评审，尚未修改前端业务代码。
- 当前保护边界是不删除证据孤儿、不自动启用级联外键、不触发 Jira/LLM/生产写入，并保持 TaskKanban 默认列、Task/Bug 标记和响应式视觉结构不变。
- 三方 UI 评审已完成并写入计划：共同采用 Phase 41 product preserve-mode，TaskKanban 保持现有层级只改变数据边界，Agenda 使用共享资源与唯一轮询 owner；前端编辑门禁已开放，下一步先建立红灯。
- 三条红灯已建立并稳定失败：active-scope/聚合字段缺失；35,500 条 Jira 历史生成 710 个 reconciliation JQL；Agenda 共享资源缺失、任务页仍全量分页。失败输出直接对应目标实现缺口。
- 首轮实现回归发现 aggregate `Select` 污染同一 GORM statement，明细只返回 1 个聚合行；Agenda/任务前端 4/4 与 Jira 定向回归已绿。已按 Self-Improving 复盘把聚合和明细显式拆为独立 GORM session，避免链式 projection 泄漏。
- 前端相关契约 12/12 通过，仓库本地 svelte-check 为 0 errors/5 个既有 tsconfig warnings；备用 pnpm 的联网/无 TTY 安装失败已绕开且未触碰依赖目录。Jira 项目窗口结果在 merge 前新增本地已知 key 求交，避免性能优化扩大同步范围。
- Jira 范围保护回归首次编译缺少测试文件的 `errors/gorm` import，产品代码未执行；已补齐最小 import 后按原断言重跑。
- deliveryplanning 与 server 完整包测试已通过（server 在获批回环环境）；真实库副本新查询为 1,052 行/3 请求/971,732B/105.6ms，精确历史搜索 1 行/1,734B/41.6ms。临时性能探针已从源码移除。
- 通知读路径已改为活动状态必要字段投影，并移除读取时的 Email 扩展调用；发布事实改走轻量路由，完整驾驶舱复用一次任务/用户快照并用 JOIN 读取 Git 证据。
- 最终 `go test ./...`、`go vet ./...`、54/54 前端契约、Svelte/TypeScript 0 error、生产构建和两套设计检测通过；Finesse 只报告目标组件原有纯白 P2。
- 隔离登录态跨过 15 秒轮询后不再出现 3.5 万行扫描、巨型 `IN`、SQLite 变量超限或邮件钩子；1440/760/390px 均无横向溢出。371MB 临时数据库/二进制、临时 pnpm store、服务和浏览器页签已清理，主数据库与外部系统未修改。

---

# Session: 2026-08-14 - 每日 Jira 跳转入口合并

- **Status:** complete locally; not deployed.
- 完成 Impeccable、design-taste-frontend、finesse-ui 与 UI design system preserve-mode 审查；共同结论是让最具体的 Jira 编号承担唯一深链动作，不改共享 shell、URL 生成、数据或决策流程。
- 症状级契约先 0/4 红后 4/4 绿；与实时刷新回归合计 6/6。编号按 URL 有无条件渲染为 `a.jira-key.jira-link` 或静态 `strong.jira-key`，独立“在 Jira 打开”操作和 action grid 已删除。
- 链接保留 `_blank`、`noopener noreferrer` 与明确 `aria-label`，具有 hover/focus/active 状态；手机端维持紧凑视觉胶囊，并通过 44px meta 行扩展命中区域。
- 登录态宽屏和手机断点均只有 1 个 Jira 链接、0 个独立操作，header/meta 无局部横向溢出，标题宽度比例为 1.0/0.978，console 无 warning/error。
- 真实点击 `HR-4090` 成功新开 `https://jira.westwell-lab.com/browse/HR-4090` 且 Jira 页标题匹配；验收后关闭页签，没有填写或提交早会决策，也没有 Jira 写入。
- `pnpm check` 0 errors/83 既有 warnings、生产 build、Impeccable type/layout `[]`、Finesse P0=0 与 diff hygiene 全部通过；截图保存为 `outputs/ui-validation/daily-jira-jira-key-link-{wide,390}.png`。

# Session: 2026-08-14 - 右侧方案预览高度与冗余胶囊清理

- 已分类为既有排期治理产品 UI 的 preserve-mode 布局回归，加载项目 continuation/complex coding 规则、planning-with-files、diagnosing-bugs、Impeccable layout/product、design-taste-frontend、finesse-ui product/redesign 与 UI design system。
- Design Read 固定为 Phase 41 轻量研发排期检查器，`SOUL=4`、`SPECTACLE=1`、`DENSITY=8`；保护 shell、数据、方案状态机、编辑弹窗和其他检查器 tab。
- 当前阶段只读定位右侧预览、两个胶囊与高度/滚动所有权；前端编辑门禁尚未开放。
- 计划文件首次插入因上下文锚点遗漏空行而验证失败，未产生修改；已改为按实际文件头精确插入。
- 已用登录态 DG-394 建立红灯：inspector 比 `solution-workspace` 多出 42.94px，Markdown 底部距 inspector 86.93px；父 inline 已拉满，子工作区仍由 40/420/30px 固定内容行决定高度。
- Impeccable 机械布局 detector=`[]`，Tailwind 任意 spacing/z-index 无命中；该空结果不覆盖运行时高度问题。
- Chrome 首次误用已移除的 `tabs.claim/open` 方法，未操作页面；读取当前能力说明后改用 `chrome.user.claimTab(openTabs 返回对象)` 成功连接登录态页面。
- 已确认 Markdown 内部预览滚动有效（419/7460），并在滚到正文末尾的真实截图中复核胶囊与剩余空白；后续修复只改变高度分配，不改变正文滚动机制。
- Impeccable 双评估完成：主观评估定位子工作区固定 420px 与冗余 footer，机械 layout detector=`[]`。三方门禁现已就 hierarchy、owner、desktop/narrow 行为、可访问性和验证范围达成共识，允许进入前端实现。
- 新增症状级契约后首次运行稳定红灯：9 项中 8 PASS、目标用例因仍存在 `solution-facts/currentSources` 精确失败；反馈环可同时约束胶囊删除、桌面弹性预览、父容器 flex 与 `<=1280px` 自然高度回退。
- 首次实现后目标行为已满足，但契约把 `.solution-preview-content` 的 CSS 声明顺序写死而产生伪红灯；已改为提取同一 selector block 后分别断言 flex、min-height 与 grid rows，不改变产品代码。
- 已完成最小实现：删除 `.solution-facts/currentSources`，增加 `.solution-preview-content` 弹性行；父 `.schedule-solution-inline` 仅补 flex column，`<=1280px` 显式恢复自然 420px 高度。
- 登录态 DG-394 宽屏最终为 inspector→Markdown 底边 `16.55px`、Markdown `488.37px`、正文 `487/7460` 单一滚动；900px 与 390px 均为 420px 正文滚动、胶囊 0、文档横向溢出 0。
- 定向契约 14/14、`pnpm check` 0 errors/83 warnings、生产 build、Impeccable 全量/layout `[]`、Finesse P0=0 与精确 diff hygiene 通过；未点击保存或发布，未修改业务数据。
- 已保存宽屏、900px、390px 验收截图到 `outputs/ui-validation/solution-preview-fill-{wide,900,390}.png`。

# Session: 2026-08-14 - 算分面板样式与对齐优化

- **Status:** complete locally.
- 完成项目冷启动、planning-with-files、Impeccable、design-taste-frontend、finesse-ui、UI design system 与浏览器门禁；四方共同采用 Phase 41 `redesign-preserve`，不修改评分公式、数据、配置、审计或共享 Modal。
- 静态和登录态基线确认三项问题：四卡主值基线受字号/说明换行影响，页面同时使用 16/18/2px 内容偏移，980px 快照表把多数列压到约 65px 且所有数据左对齐。
- 组件本地完成固定三行状态卡、132px 等高骨架、16px 内容基线、语义数值/状态/时间对齐、1210px 明确快照列宽与 sticky 人员列；详情表沿用相同数值对齐。
- 1280px 实测四卡标签/值/说明 Y 基线一致，状态/公式/系数内容起点同为 303px；900px 为 2×2 卡片和单列规则/审计；390px 正文 351px、刷新 44px、文档无横向溢出，快照表独立横滚。
- 390px 成员详情为 366px 单列 Modal，关闭按钮 44×44；打开详情只读既有快照，不触发重算。
- 定向契约 4/4、Svelte/TypeScript 0 errors、生产构建、Impeccable `[]`、Finesse P0=0 且零目标 findings 全部通过；现有其他组件 warnings 与 chunk-size warning 未扩大处理。
- 浏览器验证使用临时数据库副本、禁用集成和清空任务/通知队列的隔离服务；未修改主数据库、评分结果或外部系统。

# Session: 2026-08-13 - 绩效 v5.0 全量落地与解决方案卡片正文修复

- **Status:** in progress.
- 已加载项目冷启动/连续任务状态，并启用 planning-with-files、domain-modeling、codebase-design、diagnosing-bugs、spreadsheets 及项目强制 Impeccable/design-taste/finesse/browser 流程。
- 已建立两条独立验收链和计划：v5.0 不缩减为单纯改权重；解决方案卡片不以静态代码猜测替代真实正文红灯。
- 已确认当前数据缺口：Jira 缺流转历史/priority/severity，Git 缺唯一采集约束和 patch 指纹，正式绩效事实为空；这些将作为 v5.0 源事实层的直接改造对象。
- UI 三方共同选择 Phase 41 `redesign-preserve`，评分页和解决方案页不改变信息架构、导航、视觉系统或共享交互所有权。
- 已新增方案正文分支回归并先红后绿：`SolutionWorkspace.svelte` 的有效正文此前错误嵌套在空态分支内，现已增加独立 `{:else}`；目标契约 5/5、`svelte-check` 0 errors。
- 真实浏览器复验仍得到空的 `需求方案` 区域；DOM 仅一个空注释，而当前源码数据库确认 `NS2-986` 的 `working_revision_id=71`、正文 12197 bytes。说明运行中的旧后端/响应仍没有提供当前源码可映射的 working revision，需在新版服务重启后复验原始路径，暂不关闭该症状。
- 错误恢复：裸 `node --test` 不支持 `.ts`，已改用 `node --experimental-strip-types --test`；早期 SQLite 查询误用了不存在的方案/项目列，检查实际 schema 后已纠正，未产生写入。

# Session: 2026-08-13 - 绩效 100 分异常修复

- **Status:** complete locally; root service running.
- 按 diagnosing-bugs 重新核对判定表、真实库与页面，确认 v4.1 的 100 不是正常高分，而是只有 C01 20% 证据时被 `qualifiedWeight` 二次归一放大。
- 先将历史 Jira C01 满档场景改成红灯：成员参考分预期 20，修复前得到 100；随后把指标分恢复为 1—5、加权贡献固定为 `score × 20 × weight`，成员聚合改为直接累加合格贡献。
- 量纲修复首次使详情 `point_level` 变成 0，登录态页面即时暴露该回归；测试增加 5 分档断言后再次红灯并修正为 `int(score)`。这是本轮反馈环发现的第二个真实问题。
- 公式版本升为 v4.2，本地运行配置与示例配置同步。后台真实重算完成 14 位 core member 快照；当前参考分最大 20、100 分记录 0、指标分最大 5、加权贡献最大 20。
- 旧 v4.1 100 分行按不可变审计要求保留，最近成员投影使用最新 v4.2。登录态列表无当前 100 分，详情实测 C01 为“100% / 5 分档 / 5.00 / 20.00”，浏览器控制台无错误。
- 定向红绿灯、`internal/performance` 完整测试、配置/接口版本测试通过；前端未改动，既有构建、治理契约、Impeccable 检测和登录态交互继续通过。

# Session: 2026-08-13 - 最近人员评分快照成员去重

- **Status:** complete locally; root service running.
- 已用同一 core member 跨两个 run 的持久化快照建立红灯，修复前接口稳定返回两行，排除前端重复渲染和不稳定排序。
- `Module.Explain` 现按倒序扫描，对 canonical core-member identity 只接收第一条；`snapshot_limit` 按不同成员计数，历史扫描继续分批进行。
- 历史审计没有清理或覆盖：模块回归证明旧快照 ID 仍可解释且数据库行数不变；真实库当前保留 287 条快照和 15 次运行，最新 run 为 14 行/14 位成员。
- 根目录服务已重启并保持监听 8080；登录态页面验证最近快照 14 行、14 个唯一成员，全部来自 15:39:18 最新运行，成员详情打开后 run ID 与该次运行一致。
- 定向绩效包回归、完整 `go test ./...`/`go vet ./...`、绩效前端契约 3/3、`web/` 生产构建、Impeccable/Finesse 检测、gofmt 和精确 diff check 均通过。全量测试首次仅因沙箱禁止 `httptest` 绑定回环端口而失败，按权限规范在沙箱外原样重跑后全部通过。
- 首次从仓库根执行 `pnpm build` 因无 manifest 失败；按 Self-Improving 复核后改在 `web/` 执行并通过。构建只报告仓库既有 Svelte 与 chunk-size 告警，本次未修改前端文件。

# Session: 2026-08-13 - 绩效 v4.1 全员零分修复

- **Status:** complete locally; root service running.
- 按 diagnosing-bugs 建立真实库反馈环，确认 14 名 core member 不是没有历史 Jira，而是 C01 被错误绑定到 `due_date`；完成或到期口径与按期率口径已拆分。
- 先补红灯再修复：无 due 的周期完成需求进入 C01、到期任务仍进入 C02、低样本指标可解释但不计入成员聚合、样本达标指标才进入覆盖率与正式发布门槛。
- 公式版本升为 v4.1；运行时配置响应归一到当前支持版本，本地 `config.yaml` 和 `config.example.yaml` 已对齐 v4.1。调度开关、90 天窗口/审计保留和 core-member guard 保持不变。
- 列表和详情已明确拆成“参考分 / 正式分 / 证据不足”；详情新增样本资格、可计算/达标数量和逐指标来源。低样本单项分不会再上卷为成员参考分。
- 根目录服务完成真实重算并保持监听 8080。最新运行生成 14 条快照、8 条非零参考分、6 条 N/A、0 条正式分；17 条生命周期审计与 retention 记录完整。
- 隔离登录态页面只读验证了 v4.1 规则、最新快照、刘子翔合格/低样本混合详情、姜昊良低样本 N/A、Jira resolved/due 引用和 47/9 项系数过程；没有触发页面重算或业务写入。隔离服务、数据库副本和浏览器 tab 已清理。
- 最终后端 `go test ./...` 与 `go vet ./...` 通过；前端绩效契约 3/3、`pnpm build`、Impeccable `[]`、Finesse P0=0、gofmt 和精确 diff check 通过。
- 仓库级 `pnpm check` 仍被本任务外 `SolutionWorkspace.svelte` 的 9 个既有错误阻断；没有修改该脏文件。Browser 控制固定为 1280×720，未声称完成不可执行的 viewport emulation；窄屏规则由共享 Modal 和页面断点源码契约补充验证。

# Session: 2026-08-12 - 代码轨迹完整可滚动浏览

- **Status:** complete locally.
- 完成强制 Impeccable、design-taste-frontend、finesse-ui 三方评审；共同选择保留现有排期检查器和响应式几何，只修复轨迹数据呈现边界与正文裁切。
- 建立 3 项红灯源码契约，明确捕获 inline 固定四条切片、跨页面隐藏提示和长正文/MR 链接裁切；修复后 3/3，通过与排期等高测试组合后为 6/6。
- `CommitTelemetryPanel` 现直接渲染接口返回的全部 `commits`，移除“另有 x 条轨迹”提示，并让 inline 元信息、Jira 正文与 MR 链接完整换行。
- 静态门禁通过：Svelte 0 errors/80 条既有 warnings，TypeScript、生产构建、Impeccable `[]`、Finesse P0=0、`git diff --check` 均通过。
- 登录态 Chrome 使用真实 DG-354 验证数据库 5 条、DOM 5 条、COMMENT 5。宽屏轨迹区可滚到底且最后一条可见，左右面板 bottom delta=0；1280/760/390 都完整显示 5 条并使用自然页面滚动，文档横向溢出为 0。
- 验证截图已保存到 `outputs/ui-validation/schedule-telemetry-complete-wide.png` 与 `outputs/ui-validation/schedule-telemetry-complete-390.png`；未修改业务数据、Jira 或远程环境。

# Session: 2026-08-13 - 任务跟踪卡片底部对齐

- **Status:** complete locally.
- 加载并应用 diagnosing-bugs、planning-with-files 与项目强制 Impeccable/design-taste-frontend/finesse-ui 门禁；Design Read 为 Phase 41 高密度产品管理台，保留现有视觉和业务流。
- 复用已登录 Chrome 的独占验证页，在相同宽屏下测量任务表与执行追踪的 shell、根、workbench、表格卡和检查器矩形。
- 任务表左右卡与 22px shell 底部 inset 完全对齐；执行追踪左卡提前 66.49px、右卡提前 14.99px，红灯稳定复现。
- 根因定位到 TaskKanban 文件末尾的执行视图专属 cascade；共享 shell 和 FunctionalWorkspace 无需修改。
- 新增症状级契约，修复前 1/3 通过、2/3 精确失败；随后删除执行视图的根滚动、auto/start、固定 clamp 和 sticky/auto 覆盖，保留完整详情内容并统一检查器内部滚动，修复后 3/3 通过。
- 症状与相邻排期等高契约合计 6/6；Svelte check 0 errors/81 条既有 warnings，独立 TypeScript 与生产 build 通过，Impeccable layout 检测为空，Finesse 无 P0，精确 diff check 无输出。
- 登录态宽屏最终测量：执行追踪 workbench/左卡/右卡相对任务根底边差均小于 0.001px，左右差为 0；任务表保持相同结果，表格滚动区仍可容纳 2018px 长内容。
- 1180/760/390 响应式验收通过：两视图按既有规则自然堆叠，760/390 文档横向溢出均为 0；console 无 error/warn。已保存宽屏与 390 验收截图并结束浏览器会话，未提交业务动作或修改运行数据。

# Progress Log

## 2026-08-19 - 页面与搜索慢加载闭环

- 前端 53/53、生产构建、Svelte 0 errors、TypeScript、`go vet ./...` 和受控回环下 `go test ./... -count=1` 均通过；Impeccable 0 findings，Finesse 无 P0，仅报告本次未触及的 6 处既有纯白色值。
- 隔离登录态浏览器确认任务页只加载/展示 1,052 条当前工作集，同时保留 35,566 总量聚合；已完成的 `CR-487` 未预载在活动集内，但能由全局搜索直接加载并打开详情。
- 首次隔离启动因数据库副本的 versioned config 覆盖临时 YAML 而立刻尝试绑定旧 8080，端口占用后退出；清空仅临时副本的 `config_versions` 后，以 Jira/GitLab/飞书/AI/绩效关闭的 127.0.0.1 服务完成验证，未修改主数据库。
- 浏览器运行进一步发现通知 SSE 的延期提醒读路径每 15 秒逐事项执行邮件扩展钩子并输出海量日志；已停止临时服务，准备加入症状级回归并把该读路径改成必要字段投影、统一活动状态谓词且无发送副作用。

## 2026-08-19 - 议程查询与聚合性能收敛

- 已加载 `diagnosing-bugs` 与 `planning-with-files`，确认本轮仅修改后端数据访问链路，不触发 UI 门禁。
- 已保留现有大量脏修改并在三个既有计划文件顶部追加独立任务段，未覆盖历史内容。
- 已建立当前基线：35,578 行全表扫描、34,514 条 Done、至少 34,516 次逐事项证据查询；下一步检查模型/索引/迁移安全并建立红灯。
- 已验证现库外键关闭且存在 1 条提交、5 条通知孤儿记录；决定不在本轮追加破坏性数据库外键，而使用稳定 `task_id` 关系、批量关联与复合索引。
- 首次计划回写补丁因误含空路径 hunk 被整体拒绝；确认无部分落盘后改用精确非空补丁。
- 新增 handler 症状级回归后，默认 Go 环境在标准库/缓存阶段失败，尚未形成有效产品红灯；准备切换工作区 Go 与 `/tmp` 缓存。
- 使用 `/tmp/well-ambient-gocache` 后红灯进入真实 handler：旧实现返回 250 条自动历史，超过 200 上限；正在合并 SQL 数量断言以同时捕获 N+1。
- 最终红灯稳定报告 `250 auto decisions / 252 SQL`，同时捕获无界历史和 N+1；Phase 1-2 完成，进入实现。
- 已新增只读议程仓储：活动事项字段投影、最近 200 条历史、Git/Notification 两个显式 `task_id` 关联预加载、去重项目查询均在同一事务中完成。
- `GenerateAutonomousDecisions` 已改为批量证据关联，弱语义判断新增纯内存入口；数据库初始化新增活动/历史部分索引及 Git/Notification/Repo 复合索引。
- 症状级回归已由 `250 events / 252 SQL` 红转绿，当前断言上限为 `200 events / 6 SQL`。
- 真实库副本的全部查询计划命中新索引，三次活动/历史/项目组合裸 SQL 均小于 10ms；临时 handler 探针测得 22.17ms、595KB、5 SQL、1,064 活动项和 200 历史项。
- 临时真实性能探针源码已按 diagnosing-bugs 清理，仅保留正式症状级回归。
- 项目范围回归首次因忽略系统兜底事件而伪红；已改为检查 HIT 事件存在且 NS2 事件不泄漏，保留原业务语义。
- 全量 Go 测试除 `internal/llm`、`internal/server` 的 sandbox IPv6 loopback 监听限制外均通过；`go vet ./...` 已通过，准备受控重跑原全量测试。
- 受控环境重跑 `go test ./... -count=1` 全部通过；再次运行 `go vet ./...`、目标文件 `git diff --check` 和临时探针/DEBUG 清理检查均通过。
- Phase 1-4 完成本地交付；运行中的旧服务需重启后才会执行新查询与 `CREATE INDEX IF NOT EXISTS`，本轮未主动重启或修改主数据库。

---

## 2026-08-14 - 方案生成失败后手动重试

- 已分类为 `coding.complex` + targeted product redesign，加载项目 cold-start/continuation/coding/memory 规则、planning-with-files、diagnosing-bugs，以及强制 Impeccable、design-taste-frontend、finesse-ui 和 UI design system。
- Design Read 固定为 Phase 41 研发交付方案卡片，`SOUL=4`、`SPECTACLE=1`、`DENSITY=8`；共同方向是失败状态原地恢复，不新增 modal/tab/卡片，不改变 Markdown、版本、发布或 Jira 来源链路。
- 初步源码定位确认 `failed` 状态直接显示 `last_error`，当前 UI 没有 retry handler；自动重试在第三次失败后终止。下一步先建立后端状态机与前端入口的症状级红灯，再决定最小持久化语义。
- 初次计划插入因顶部已有更新导致锚点失效，`apply_patch` 验证失败且无部分写入；已按 Self-Improving 规则读取真实文件头并用稳定锚点重新插入。
- 已新增症状级测试并运行红灯：Go 测试精确失败于专用 retry command/method 不存在；前端契约 8/9，通过项未回归，目标用例精确失败于“重新生成”、pending、API、技术详情和 524 摘要缺失。
- 三项假设已用源码边界和主库只读样本逐项确认：默认幂等键重放旧失败；旧 polish 路由会先建立 working draft；workspace 轮询正确，问题属于终态投影与专用命令缺失。准备进入最小实现。
- 后端模块已实现 `RetryFailedPolish`：保留原失败行，新建 attempt=0/queued 替代任务，以失败 job ID 形成幂等链，并拒绝已有 working 或被更新任务越过的旧失败。
- 新增 `POST /api/solutions/jobs/{id}/retry`，由 `solution:write` 权限保护，首次返回 202、重复返回 200/replayed，并原子返回最新 workspace；模块与 API 定向测试均 PASS。
- 卡片失败态已加入友好错误摘要、折叠技术详情和统一 primary 重试按钮；提交期间禁用并显示“重新排队中…”，成功后消费响应 workspace 原地更新。移动端按钮为 44px，不增加 modal/tab/新卡片。
- 前端方案契约已由目标 8/9 红转为 9/9 绿；新增 stale failure 与 existing working 后端保护测试也已 PASS。
- 首轮完整验证中 solutions、前端 check/build 与 diff hygiene 通过；server 套件被 sandbox 的本地 loopback 限制中断，准备按受控权限原命令重跑。目标组件同时暴露两个无消费者的旧链接 CSS warning，已作为目标卫生清理。
- 受控权限原样重跑 `internal/server` 完整套件通过；solutions 套件、前端方案契约 9/9、`pnpm check`（0 errors）、生产 build、Impeccable `[]`、Finesse P0=0 和 `git diff --check` 均通过。
- 使用 127.0.0.1:18189 内存假后端和 4182 独立 Vite 完成认证浏览器验收；首次沙箱监听 EPERM 后按规则受控启动，验收完毕已关闭，不触碰现有 8080/5173/18081 进程。
- 1440px 默认错误卡只显示友好 524 摘要，技术 JSON 可折叠查看；点击后即时禁用为“重新排队中…”，随后原地进入 queued 并移除重试入口，没有 modal/tab/URL 变化。
- 390px 实测重试按钮 44px 高、文档无横向溢出，浏览器 console 无 error/warn；临时浏览器 tab 已关闭并重置 viewport。当前任务本地完成，未调用真实 LLM/Jira、未写主数据库、未部署或重启后台。

---

## 2026-08-14 - 绩效 v6.0 数字资产算分收敛

- 已加载项目冷启动规则、历史绩效证据、planning-with-files，以及强制 Impeccable、design-taste-frontend、finesse-ui 三方门禁。
- Design Read 固定为 Phase 41 高密度产品管理台，`SOUL=4`、`SPECTACLE=2`、`DENSITY=8`；三方共同选择保留页面/Modal 所有权，只收敛一级信息层级。
- 已确认当前工作树包含大量用户和前序任务修改；本轮只增量修改绩效模块、Jira 必要证据采集、配置解释、目标页面和针对性测试。
- 下一步完整读取产品 UI 深层规范，核对 Jira parent/issue-link 结构并建立 v6.0 公式、归责和风险扣分红灯。
- Impeccable product、finesse product/redesign 深层规范已完整读取；UI 保护边界和桌面/移动验证范围已冻结，可以进入源码与测试盘点。
- codebase-design/deepening 已完成；后端 seam 固定为现有绩效 Module run/explanation interface，新增复杂度只进入模块内部纯计算与本地数据库测试。
- 源码盘点确认 Jira adapter 尚未采集 parent/issuelinks，当前 B01 只认追加式正式归责；下一步先建立 Jira link、v6 规则数和 Git 只扣不奖的红灯。
- 已确认 Jira 三条同步路径共享字段应用函数，前端只消费 explanation seam；改造可以保持较小外部 interface。下一步进入测试设计和精确红灯。
- v6.0 后端已实现：D01/D02/B01 权重为 35/20/45，需求系数删除复杂度/阶段/角色，质量使用 30 天充分暴露分母，Git 重复率和 Commit 密度只形成最多 10 分扣分。
- Jira client 已采集 parent/issuelinks，历史 JQL 对 core-member 需求与项目内 Bug 使用不同证据范围；唯一来源需求链接归到需求 Done 负责人，多链接保持未归责，修复负责人不自动背负质量损失。
- explanation API 已派生交付/质量分量并持久化结构化代码风险；配置、示例配置和设置页默认版本同步为 v6.0，历史快照不改写。
- 前端一级成员表已压缩为参考分、交付、质量、风险和证据状态；详情新增代码风险表，系数公式只展示四个可执行需求系数，公式版本不再作为页面视觉信息显示。
- 定向 Go 测试通过；服务包首次全量在沙箱内因禁止 `httptest` 绑定回环端口失败，按规范在允许回环监听环境执行目标服务测试后通过。测试夹具曾因复用反序列化对象残留 parent 失败，重置夹具后“多链接不归责”保护转绿。
- 前端生产构建通过，`pnpm check` 为 0 errors/84 条既有 warnings；契约测试首次误用未安装的 tsx、随后普通 Node 不识别 `.ts`，最终使用 Node 22 `--experimental-strip-types` 运行并 7/7 通过。
- 服务端旧八指标 explanation 测试已改为精确断言 D01/D02/B01；全量 `go test ./...` 与 `go vet ./...` 通过。
- Impeccable 对三个绩效前端目标文件检测结果为 `[]`；前端 7/7 契约、Svelte check 0 errors（84 条其他文件既有 warnings）和生产构建通过。
- 隔离数据库启动完成最终 v6 startup run：14 条快照对应 14 名唯一 core member，可计算参考分 11–71，0 分/100 分/负零审计记录均为 0；6 名样本不足成员以 N/A 展示。
- 登录态浏览器验证 1280/1024/900/390：一级八列稳定、三项指标和两项代码风险信号可追溯、共享 Modal 内部滚动与关闭正常、页面无文档横向溢出；新会话 console error 为 0。
- “刷新已保存结果”前后隔离库 run 记录均为 37，证明读取不会触发计算；所有临时页面、前端和后端进程已关闭，现有服务和源数据库未重启、未写入。

## 2026-08-14 - 每日 Jira 跳转入口合并

- 已加载 planning-with-files、Impeccable distill/product、design-taste-frontend、finesse-ui product/redesign 和 UI design system。
- 三方一致采用 preserve-mode 合并：编号胶囊承担 Jira 深链，独立操作胶囊删除；无 URL 静态 fallback、键盘焦点和新窗口安全属性保留。
- 已确认最小 owner 为 `DailyJiraAudit.svelte` inspector header；下一步先修改现有症状级契约形成红灯，再编辑生产代码。

## 2026-08-14 - 每日 Jira 右侧标题宽度修复

- 已加载项目冷启动规则、planning-with-files、diagnosing-bugs 及强制 Impeccable/design-taste-frontend/finesse-ui/UI design system 门禁。
- Design Read 固定为 Phase 41 高密度 Daily Jira 产品检查器，preserve 模式；标题是主事实，Jira 链接是首行次级动作。
- 两路 Impeccable 只读审计完成：主观审计定位 flex 一维布局与两行信息关系不匹配；机械 detector=`[]`，无 Tailwind 任意 spacing/z-index 命中。
- 三方已就 hierarchy、组件 owner、桌面/移动行为与验证范围达成一致；只把 header 扁平为 meta/title/action 三个同级元素，并以两列两行 grid 分配空间。
- 新增标题宽度契约先 3/3 红，实施后 3/3 绿；与既有 Daily Jira 实时刷新测试合计 5/5。
- 已复用现有 Chrome 登录态页面验证 `FEL2WD-2037`：宽屏标题占 header 100%，760px 堆叠态 98.5%，390px 单列态 97.8%；操作无重叠，文档无横向溢出，console 无 warning/error。
- `pnpm check` 0 errors/83 既有 warnings、生产构建、Impeccable type/layout `[]`、Finesse P0=0 和精确 diff check 全部通过。
- 验收截图保存为 `outputs/ui-validation/daily-jira-inspector-title-wide.png` 与 `daily-jira-inspector-title-390.png`；只切换时间分组和选中行，未填写或提交早会决策。

## 2026-08-14 - 方案编辑弹窗扁平化与 Markdown 表头默认隐藏

- 已完成共享 `MarkdownWorkbench`、`SolutionWorkspace`、`Modal`、全部调用方与现有方案契约的源码核对。
- 已确认最小归属：共享组件提供默认隐藏视觉元信息和可选嵌入态；方案弹窗只消费嵌入态，不修改共享 Modal、保存发布、冲突、权限或 revision/CAS。
- 已保护现有脏修改：`MarkdownWorkbench.svelte` 的单模式 mode switch 条件和 `Modal.svelte` 的统一关闭按钮改动均属于既有工作，本轮不回退。
- 两项红灯已建立并转绿，定向方案/同步契约最终 13/13 PASS；`pnpm check` 为 0 errors，生产构建 PASS，Impeccable detect=`[]`，Finesse 高风险样式扫描无新增命中，`git diff --check` 通过。
- 登录态 DG-394 已验证主预览视觉表头隐藏；编辑弹窗在 1440/900/390 三档直接显示 533 字符正文、无内框/圆角/背景、无文档横向溢出。
- 390px 双滚动已修复：Modal body `512/512` 不再产生外层滚动，CodeMirror `484/6236` 可独立浏览全部正文；关闭后焦点归还“编辑方案”，console 无 error/warn。
- 验证前后 DG-394 的 revision/working revision/draft 状态完全一致，未保存、发布或修改业务数据；验收截图已保存至 `outputs/ui-validation/solution-editor-wide.png`、`solution-editor-900.png`、`solution-editor-390.png`。

## 2026-08-14 - 方案编辑弹窗扁平化与 Markdown 表头默认隐藏

- 已分类为既有产品 UI 的 preserve-mode follow-up，加载项目冷启动规则、planning-with-files、Impeccable/product、design-taste-frontend、finesse-ui/product/redesign/anti-cheap/preflight。
- Design Read 固定为研发排期方案编辑，`register=product`、`SOUL=4`、`SPECTACLE=1`、`DENSITY=8`；三方共同方向为 Modal 单一强容器、Markdown 正文直接显示、视觉表头共享 opt-in。
- 已记录保护边界：不修改后端、保存/发布/冲突/脏关闭状态机、共享 Modal 关闭几何、Phase 41 tokens 或其他业务调用；下一步核对当前源码和调用方后再编辑前端。

## 2026-08-12 - 全局方案治理中心

- 已加载项目冷启动规则、planning-with-files、domain-modeling、codebase-design，以及强制 Impeccable、design-taste-frontend、finesse-ui 全部相关规则。
- 已运行 Impeccable 项目上下文识别，确认使用现有 `DESIGN.md`，无独立 `PRODUCT.md`；本次是既有产品页扩展，不触发从零初始化。
- 三方 UI 评审已完成并写入计划：Phase 41 浅色管理台、列表加右侧检查器、移动端自然堆叠、完整加载/空/错误状态、全部选择器复用共享 `Select`。
- 已确认当前工作树包含大量用户未提交修改，本任务将只增量修改直接相关文件，不回退或格式化无关内容。
- 已新增 `solutioncatalog` 深模块与目录/搜索 token/同步任务/两轮比较/标准化提案/标准修订模型；发布事务写入幂等任务，周期 reconciler 负责补偿，目录不复制 Markdown 正文。
- 已接入权限化目录、详情、项目、标准、校准与提案审核 API；所有查询先应用用户项目偏好，两轮 worker 绑定版本化提示词，模型请求未设置整体 timeout。
- 已新增全局“方案中心”导航和页面：已发布方案/标准方案双视图、搜索、共享项目 Select、左列表右详情、Markdown 即时只读、相似比较和内联审核；移动端自然堆叠，不新增 modal 或 tab 跳转。
- 后端完整定向回归 PASS：`internal/config`、`internal/db`、`internal/solutions`、`internal/solutioncatalog`、`internal/server`；EXPLAIN 回归确认候选查询使用 `idx_solution_catalog_token_entry`。
- 前端 `pnpm check && pnpm build` PASS（0 errors）；目标 Impeccable 检测无发现，`git diff --check` PASS。
- 隔离认证浏览器已验证列表选择不改 URL/tab、共享 Select、搜索空态、稳定加载、错误重试、接受提案生成标准，以及切换详情滚动归零；1280/1024/760/390 无横向溢出，390 主控件均为 44px，最终 console 无 error/warn。
- 所有隔离 preview/fixture 服务与浏览器测试 tab 已关闭；未访问真实 Jira、未调用真实 LLM、未重启后端或修改业务数据。新 schema、路由与 reconciler 等待下一次受控重启/部署加载。

---

## 2026-08-12 - 大模型请求取消总超时

- 已加载 diagnosing-bugs 与 planning-with-files；完成统一 LLM 客户端、所有 Generate/Stream 调用方、前端 reader、HTTP Server、context deadline、Provider Files 和 solution lease 扫描。
- 已向用户展示四项可证伪假设。H1 命中：非流式模型生成有固定 90 秒整体 timeout；H2 本地流式外层未命中；H3 命中 AI 配置探活 8 秒 deadline；H4 的 worker lease 不会终止当前请求。
- 下一步先修改现有客户端测试形成精确红灯，再移除生成、AI 探活和 Provider Files 上传的任意整体 timeout；保留调用方 context 取消。
- 红灯已运行并精确失败：`non-stream timeout = 1m30s, want no overall timeout`；没有依赖真实 provider 或等待 90 秒。
- 统一客户端已收敛为一个 `httpClient()`：默认 `Timeout=0`，`Generate`/`Stream` 共用；调用方 context 仍挂在 `http.NewRequestWithContext` 上。
- AI 配置探活已由固定 8 秒 context/client 改为继承 HTTP 请求 context；Provider Files 上传客户端也改为 `Timeout=0`，并增加独立契约测试。
- 定向客户端无超时、流 context 取消和 Files 客户端无超时测试全部 PASS；复扫只剩生成后文件清理 30 秒保护与测试注入的 2 秒客户端，不属于真实模型生成调用。
- 完整相关回归通过：`internal/llm`、`internal/server`、`internal/solutions`、`internal/telemetry` 全部 PASS；gofmt 与精确 `git diff --check` 无输出。
- 最终调用方复扫确认所有真实 `providerllm.Client` 均未注入带 timeout 的 HTTP 客户端，也没有模型调用 `WithTimeout/WithDeadline`；前端 reader 无 AbortController/Promise timeout，Go server 无 WriteTimeout。
- 未调用真实大模型、未修改业务数据，也未重启会触发 Jira/LLM/outbox 的现有后端。修复等待下一次受控重启/部署加载。
- 最后将模块契约进一步收紧：即使未来注入带 `Timeout` 的自定义 HTTP client，也会复制并归零 timeout，同时保留 Transport 且不修改调用方对象。首版测试误比较函数型接口导致 panic，按 self-improvement 记录后改用可比较 transport 指针；最终完整回归再次 PASS。

---

## 2026-08-12 - Agent 首次方案直用与人工草案边界

- 已启用 diagnosing-bugs、planning-with-files 与强制 UI 门禁；新任务计划已写入，尚未修改生产代码或业务数据。
- memory quick pass 仅命中共享 `MarkdownWorkbench` 的 live 默认契约，没有命中 Agent/人工草案生命周期历史决定。
- design-taste 首段读取因输出预算截断，已停止使用该输出并切换为 180 行分段；按规则不把一次性工具偏差写入长期记忆。
- Self-Improving 完整规则已读取；design-taste 已按 180 行分段无截断读到 EOF，finesse-ui 及 redesign/product 参考也已完整读取。
- Impeccable product register 与项目 `DESIGN.md` 已复核：本轮必须保留 Phase 41 右侧单一检查器、现有 Markdown 主编辑器与响应式结构，候选分支应通过状态语义收敛而不是新增弹窗、tab 或视觉层。
- 已查询完整真实 revision/job 链：成功样本均为系统占位 v1 + Agent candidate v2，working 指向占位；没有发现真实人工作者修订。DG-352 在诊断期间也由运行中旧 worker 新增 candidate，证明兼容旧队列是必要范围。
- 已添加四个模块接口红灯：无人工草案时 seed 不泄漏、Agent 成为唯一可编辑 v1；人工草案存在时不入队；Jira 同步对应验证。默认 Go cache 被 sandbox 拒绝后改到 `/tmp`，红灯精确停在新接口/kind 尚未实现。
- codebase-design 与 deepening 规则已加载；选择把 seed、人工边界、首次完成推进和旧任务幂等收敛封装进 `solutions.Module`，Jira worker 只调用一个高杠杆接口，不在调用方复制判定。
- 三方 UI 评审已完成并写入计划：一个 canonical working、就地状态、无 candidate 对照/计数/应用/重新润色；不改右检查器、Markdown 同步与响应式布局。
- 已实现 `RequestInitialDraft` 深模块：自动来源只创建隐藏 v0 `system_seed`；首次 Agent 完成直接写 `agent_draft/draft` v1 并推进 working。已有 working 时不入队，重复旧任务收敛到当前 working。
- 已加入无损兼容迁移：旧 `jira-sync` 空占位转为隐藏 seed，最新 Agent candidate 提升为 canonical working；不删除 revision/job/source，保留版本号、压缩、发布与审计链。
- Jira 同步改为调用单一首次草案接口；人工草案存在时静默跳过。方案 UI 已移除 candidate 对照、计数、应用和“重新润色”，只显示生成状态或 canonical Markdown 主文档。
- 四条 solutions 生命周期测试、Jira 人工边界测试、方案入口契约 4/4、完整 `internal/solutions`/`internal/server`、Svelte/TypeScript 与生产 build 全部通过。
- Impeccable targeted detect=`[]`；Finesse P0=0，仅保留两处既有纯白 fallback P2。精确 diff check 与临时端口清理通过。
- 使用数据库副本和禁用外部集成的隔离服务完成认证浏览器三态验收；验证旧 candidate 直接显示为主方案、人工草案不出现候选/润色、生成中不出现空 editor，以及 dirty Markdown 不被轮询覆盖。隔离副本、二进制、配置和服务已清理。
- 用户原 Chrome tab 已恢复到 DG-394 深链。当前 8080 仍为旧进程；没有在未授权情况下触发真实 Jira、LLM 或 outbox，兼容迁移等待下一次受控启动。

---

## 2026-08-12 - 方案润色长时间中与即时显示收敛

- 已加载 planning-with-files 和项目本地 Impeccable 完整规则；本任务将使用现有 `task_plan.md/findings.md/progress.md` 持久化诊断事实与三方 UI 共识。
- 首次合并读取两个大型 UI 技能文件时输出被截断；已记录并改为分块读取，不使用截断版规则作决策。
- design-taste-frontend 全文共 1206 行，finesse-ui 共 293 行；已改为每段最多 240 行持续读取，直到两个文件 EOF 后才形成三方结论。
- design-taste-frontend 已分块读到 1206 行 EOF。该技能明确将 dashboard/admin 列为非主场，本轮只采用 redesign-preserve、完整 loading/empty/error 状态、文案自审与响应式保护，不引入营销页视觉语法。
- finesse-ui 已读到 293 行 EOF，Impeccable product register 和项目 `DESIGN.md` 已读取。Design Read 确认为 Phase 41 研发管理 product surface，`SPECTACLE=1/DENSITY=8`；保留既有单一右检查器、熟悉控件与紧凑信息密度。
- Finesse redesign-mode 与 product-ui 参考已完整读取；本轮定位为最高杠杆的状态可见性与组件精简，保护路由、信息架构、设计 token、方案 Markdown 数据契约、人工应用 candidate 和编辑同步行为。
- 快速历史核对命中一条可复用契约：共享 `MarkdownWorkbench` 的 `live` 模式已是默认显示，现有 rollout 也已验证 preview/edit 行为。下一步对照当前方案工作台实现，判断是否只需移除多余 mode prop/文案。
- 已运行数据库诊断命令，同时输出 job 状态聚合、最近 30 条队列、非 queued 详情、重复 input revision、candidate revision 与有效来源数。DG-394 为纯排队，该反馈环已捕获用户的精确症状。
- 已确认 worker 的有效并发度为 1：每轮 `for` 最多取 3 条，但每条都会同步等待 LLM 完成后才继续；不是 3 路并行。前端则把 queued/running 折叠为同一文案。
- 前端定位时误假设 `web/src/services` 存在且一次输出过大；已完成 Self-Improving 复盘，不写入未经用户请求的长期记忆，后续改用真实文件清单与分段读取。
- 已完成实现前三方评审并写入计划：状态在现有方案检查器内展开，方案主编辑器固定共享组件的 `live` 模式，候选对照保持只读；不修改共享编辑器、页面几何或同步协议。
- 已定位精确前端根因：`activeJob` 合并 queued/running，按钮统一显示“润色处理中…”，同时调用方把共享编辑器的四种模式重新开放。状态细节在 API model 中已有，不需要新增接口或迁移。
- 已定位队列放大器：同一 Jira 评论页在循环内按逐条增长的来源 watermark 入队。计划先用“两条 eligible 评论只产生一个含完整 source_refs 的 job”建立红灯，再把入队移到完整对账之后。
- 红灯已成立：两条方案评论产生 job 1=`source_refs:[1]`、job 2=`[1,2]`；前端契约也精确失败于四模式仍开放。测试环境的首次 loopback 拒绝已按既有受控路径重跑，不混入产品判断。
- 检查共享编辑器模板后确认：单一 live 模式仍显示一个无意义的模式按钮。本轮将加 `toolbarModes.length > 1` 渲染条件；多模式调用方完全不受影响。
- 首次组合实现补丁因一个前端 notice 片段顺序不匹配被整体拒绝；已确认没有任何部分落盘，正在按小补丁继续。
- 后端“对账后单次入队”和前端 job 状态模型已成功落盘；模板补丁因重复空态行已不在当前文件而拒绝，尚未修改 template/shared markup，继续按现状小段处理。
- 模板与共享编辑器小补丁现已落盘：状态行和按钮使用真实 job 文案；保存/复制不再暴露 Markdown 类型术语；方案固定 live；仅一个模式时不渲染 mode switch。编辑 change/save/CAS/dirty 代码未改。
- 定向绿灯：Jira 专用作者自愈与同批多评论单次入队 2/2 PASS；方案前端契约 4/4 PASS；gofmt/diff hygiene PASS。
- 完整 server/solutions 回归、前端 Svelte/TypeScript 检查与生产构建均通过；已移除本轮造成的 `.solution-facts .danger` unused selector，未处理无关既有 warnings。
- Impeccable 目标文件检测无发现；Finesse P0=0。已加载 Browser 控制规则，准备复用登录态本地页面完成真实 DG-394 与断点验收。
- 浏览器连接已建立，但当前绑定没有打开的 tab；这是正常清理状态。下一步在同一浏览器中新建本地 5173 页面，不切换浏览器或读取会话存储。
- 本地 5173 页面已正常打开；in-app browser 显示登录页且无可复用认证。按 Browser 规则，用户未指定浏览器时先尝试已有 Chrome 登录态，再决定是否需要用户介入。
- Chrome 中存在已登录的 `http://localhost:5173/` 页面，以及 DG-394 Jira 页面；将只认领本地应用 tab 做只读 UI 验证，不操作 Jira tab、不提交任何业务动作。
- 已认领本地应用并通过深链打开 DG-394。真实 panel 为“等待重试”，候选 0/来源 1；mode selector 与单个即时模式按钮均不存在，保存方案按钮存在。全程只读，未点击润色/保存/发布或访问 Jira tab。
- 已确认 Chrome 支持临时 viewport override；将按 1440、760、390 三档检查方案检查器可见性、文档横向溢出、状态与即时编辑器，完成后重置默认视口。
- 三档响应式只读验证均无横向溢出，panel/live 编辑器稳定，模式选择器为 0。补齐窄屏 44px 动作命中区，并修正已过 retry 时间仍显示过期“预计时间”的文案；需重新跑定向前端与浏览器窄屏绿灯。
- 移动端 HMR 绿灯：实际 CSS viewport 433px 下状态为“等待重试，已到重试时间，正在等待后台队列”，3 个动作均 44px，live=true、mode switch=0、横向溢出=0。首张截图位于列表上方，下一步滚动到右侧方案卡片本体做视觉复核。
- 已滚动到方案卡片和编辑器本体完成视觉复核：状态不与按钮冲突，标题自然换行，live 语法装饰和 6 行/74 字符状态完整；未进入编辑或触发保存。发现一处上层介绍仍暴露“Markdown 内容”，将改为“方案正文”。
- 方案检查器上层介绍已改为“查看 Jira 方案评论、方案正文与润色候选”，不再把存储格式作为用户显示类型。
- 最新只读 DB 复查确认 worker 持续推进；DG-394 的一次执行因 provider 502 失败并进入重试，不再将该状态笼统描述为正在润色。
- 最终前端回归、检查、构建、Impeccable/Finesse 与 diff hygiene 已通过；准备核对 dirty/untracked 归属、浏览器 console，并释放本轮认领的 tab。
- 工作树包含大量本轮之前/同会话既有修改；`SolutionWorkspace.svelte` 与 `web/tests/` 仍为未跟踪的新方案功能文件。本轮只做小补丁叠加，没有回退、覆盖或暂存任何用户/既有改动；数据库只读查询之外未手工改写业务数据。
- 浏览器最终状态为 DG-394 正在润色（22:30 开始）、live=true、mode switch=0、上层“方案正文”文案生效。console 的唯一 error 是分步 HMR 历史记录；最终源码没有 `editorMode` 引用，check/build 已通过。用户原 Chrome tab 保留，临时 in-app tab 已清理。

## 2026-08-12 - 项目级方案提示词缺省日志刷屏

- 已加载 diagnosing-bugs 技能并读取领域 `CONTEXT.md`。当前只构建一条针对 HIT project-scope 缺省的日志红灯，未修改生产查询。
- 红灯命令已运行：`GOCACHE=/tmp/well-ambient-gocache go test ./internal/solutions -run '^TestProjectPromptMissFallsBackWithoutRecordNotFoundLog$' -count=1`，稳定 FAIL 并捕获与用户完全一致的 HIT SQL；其余环境和 worker 均不是复现必需条件。
- 生产查询已仅在可选 project-scope 分支改为 `Find + RowsAffected`；全局 prompt 仍使用 `First` 并在真实缺失时返回 `no active solution prompt`。定向红灯与项目优先对照测试已同时 PASS。
- 完整 `internal/solutions` 与 `internal/server` 套件 PASS；gofmt 无差异，相关 diff check PASS。当前 8080 监听 PID 36200 仍需重启才能加载修复；本轮未触发 Jira 读取、AI 调用或 Jira 写回。

## 2026-08-12 - DG-394 Jira 方案评论静默润色失效

- 已完成第一轮定向探针：H1（专用作者信号未识别）得到直接证据，H2（正文 marker 被丢失）被否定。
- 额外确认了历史数据自愈缺口：相同快照重放不会更新资格元数据，worker 也会因 `Replayed` 跳过入队。下一步先写两类红灯：专用作者识别，以及相同快照从不合格重判为合格后的幂等入队。
- 诊断 SQL 先后误用 `enabled` 和 `scene/updated_at`；已完成 Self-Improving 复盘，后续查表前先以实际 schema 校验字段，不把查询失败混入产品根因。
- 已建立两条可执行红灯：模块测试证明重放后仍是 `eligible=false/marker=''`；server 集成测试在允许临时 loopback 后证明作者改为 `jira公用-解决方案` 仍不会重判、也不会入队。
- 最小修复已落盘：评论识别支持专用解决方案账号；`ObserveSource` 在不改正文/哈希/快照 ID 的前提下刷新 `marker/eligible`；worker 对每次合格观测幂等请求润色，避免同步瞬时中断后永久丢任务。
- 定向结果：`TestObserveSourceReplayRefreshesDerivedClassification` 和 `TestJiraSyncReclassifiesDedicatedSolutionAuthorAndQueuesPolish` 均已从 RED 转为 PASS；第三次相同 Jira 同步后 job 总数仍为 1。
- 完整 `internal/solutions` 与 `internal/server` 回归全部通过。当前本地库中 DG-394 仍是修复前运行态：asset 105 的 working revision 为 0，source 274 仍 `eligible=0`，且无 polish job；这是接下来运行态刷新的精确前置断言。
- 8080 监听进程 PID 28593 的 cwd 已确认为当前仓库；Jira worker 启动时立即同步，之后每 30 秒重试，solution worker 也会随 server 启动。
- 运行进程链已精确确认：父进程 PID 28568 为 `go run cmd/server/main.go`，子进程 PID 28593 监听 8080，stdin/stdout/stderr 均连接 `/dev/ttys007`。重载时将只终止这两个已确认进程，并按原命令从当前仓库启动。
- solution worker 启动时立即处理，之后每 5 秒扫描；任务成功后会持久化 candidate revision 并广播需求更新。
- 已安全终止仅属于当前仓库的旧后端 PID 28568/28593。原命令重启因“Jira 私有内容发送至外部 AI + 可能 Jira outbox 写回”需要明确授权而被审核拒绝；当前 8080 新进程尚未启动，未尝试任何规避方案。

## 2026-08-12 - 全局弹窗关闭按钮安全区优化完成

- 已加载项目冷启动路由、复杂编码/内存/工具/交付规则、planning-with-files、diagnosing-bugs，以及强制 Impeccable、design-taste-frontend、finesse-ui 三方门禁。
- 已运行 Impeccable context，确认 `PRODUCT.md` 缺失但 `DESIGN.md` 存在；本次是现有产品 UI 的 scoped refinement，不触发从零 init。
- 已检查用户截图原图、Phase 41 设计契约、共享 Modal 历史约束、工作树状态和第一轮 modal close 源码清单。
- 已建立红灯目标：捕获长标题/多行标题与关闭按钮 44×44 命中区相交或安全间距不足；尚未编辑任何前端文件。
- Impeccable 要求的隔离主观布局评估和机械 detector 预扫描已并行启动；等待合并结果后记录三方共识再实施。
- 已连接应用内浏览器并检查现有页面；当前没有可复用的打开页面或登录态 tab。下一步先探测本地已运行端口，若无现成服务则使用项目允许的安全本地验证路径，不绕过认证。
- 首次端口探测因误用 zsh 只读变量 `status` 提前失败，已按 planning/self-improvement 规则记录；改用非保留变量并增加 IPv6 探测。
- Chrome 中存在已登录的 `http://localhost:5173/` 页面并已认领；首次全量 DOM 快照因页面 evaluate 超时，准备改用更轻量的目标读取。
- 轻量可见 DOM 成功确认截图中的 NS2-2047 弹窗仍处于打开状态；locator 几何已把侵入量化为按钮约 20.7×34px、标题/按钮相交、安全间距约 0px，形成可重复红灯。
- 隔离机械 detector 预扫描完成：9 个 modal 目标 layout findings=0，证明源码检测器是下限，不能替代运行时几何；等待隔离主观布局评估完成后合并三方共识。
- 隔离主观布局评估完成；已将 Impeccable、design-taste-frontend、finesse-ui 的共享方向、分歧解决、组件所有权、响应式规则和几何验收写入计划。前端编辑门禁现已满足，进入实现。
- 已将 18 个弹窗/抽屉入口迁移到共享 44px 关闭按钮与标题安全列；`pnpm -C web check` 为 0 error，80 条均为既有告警。
- 新增源码契约回归；首次用默认 `node --test` 运行被 Node 22 的 `.ts` 加载限制阻断，已确认该运行时提供 `--experimental-strip-types`，下一次使用该受支持参数执行。
- 已重新打开真实 NS2-2047 详情并完成桌面几何采样；焦点验证第一次选用了 Chrome locator 不支持的 `.focus()`，几何结果已保留，后续改走受支持的键盘/DOM CUA 交互。
- NS2-2047 已在桌面、中宽和移动宽度通过 44px、16/12px gap、零相交、零文档横向溢出验证，并保存三张截图；关闭动作生效。进入其它页面级实现抽样。
- Demand 页面级“新需求”弹窗已在桌面和移动宽度通过 44px、16/12px gap、零相交和零横向溢出；仅打开后关闭，未填写或提交任何数据。
- Health、Settings、DecisionEventCenter、CommitTelemetry 五类页面级实现完成桌面/移动几何抽样，均为 44px、零相交、零文档横向溢出。代码轨迹 X 额外暴露顶栏层级覆盖；最终由 workspace portal action 同步真实可视四边到 fixed inset，并已通过桌面与移动长页面复验。
- 代码轨迹最终复验通过：桌面/移动 X 均命中自身并成功关闭，抽屉从真实工作区顶边开始、保持视口内高度；最终 Chrome error/warning 日志均为空。
- 最终 `pnpm -C web check`、7 项前端测试、生产构建、精确 diff check、Impeccable 完整/布局检测和 Finesse P0 门禁全部通过；三张 NS2 长标题截图和一张 Settings 移动截图保存在 `outputs/ui-validation/`。


## 2026-08-12 - 需求方案资产、自动润色与压缩保存完成

- 新增方案资产深模块、数据库迁移、CAS/不可变版本、来源快照、gzip/哈希、润色任务、提示词版本、Jira 幂等 outbox 和回收 worker lease。
- Jira 同步增加评论分页与方案标记识别；Agent 通过版本绑定提示词与来源生成候选，平台安全边界始终追加且不可由超管提示词移除。
- 新增方案读写/发布权限和仅全局超管可用的提示词管理权限；提示词保存、启用进入审计日志。
- 需求详情新增稳定方案链接、Markdown 工作台、候选对照/应用、发布/fork 和冲突恢复；配置中心新增提示词版本治理与公开链接配置。
- 隔离认证浏览器完成桌面/平板/手机验收，验证 97% 压缩、并发 dirty 保护、深链、候选、发布、提示词 v2 和公开地址同步；未访问真实 Jira/AI 或项目数据库。
- Go 回归、TypeScript/Svelte 检查、同步单测、生产构建、Impeccable 检测和 diff hygiene 通过。现有 Svelte warning 为 80 条、0 error。


## Session: 2026-07-20 - Task Project/Owner Multi-Select And Surface Rhythm

- **Status:** complete.
- Completed and recorded the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits. The shared direction keeps one bounded toolbar row, server-correct multi-selection, the existing table/inspector ownership, and scoped transparent secondary surfaces.
- Extended `/api/tasks` and `/api/execution/tasks` to accept repeated or comma-separated Project/Owner values. Values are ORed within each dimension and intersected across dimensions after saved personal project scope and existing core-member visibility.
- Extended the shared `MultiSelect` with an optional compact summary/overlay mode, clear-to-all action, visible Project/Owner ownership labels, keyboard behavior, and touch-safe narrow-screen controls without changing the project-preference default presentation.
- Replaced status, personnel, and execution Project/Owner filters with local multi-select arrays, repeated query serialization, count summaries, and responsive wrapping. Fixed the 1440px project menu clipping by aligning the overlay inside the execution table panel.
- Added the execution-only viewport height budget so its two workbench panes land on the shared shell bottom gutter despite omitting the stage strip. Flattened gray nested fills in task facts, personnel facts, and personnel priority rows while retaining semantic status washes.
- Focused filter tests and the full `internal/db`, `internal/server`, and `internal/agenda` suites pass. Frontend check passes with 0 errors and the existing 72 warnings; production build, diff hygiene, exact-file Impeccable detection, and Finesse P0 detection pass.
- Authenticated isolated browser validation passed at 2048, 1440, 1024, 760, and 480 widths. HIT + NS2 returned 43 execution rows; adding 梁志远 + 朱家聪 returned the correct 3-row intersection; clearing restored 390 rows. Dropdowns are unclipped, narrow controls are 44px, document overflow is zero, and a fresh logged-in execution tab has no console errors.
- Used a copied database with external integrations disabled and left the existing 8080/5173 services untouched. No Jira/demand record was created, updated, assigned, synchronized, or reviewed; both isolated validation services were stopped.

## Session: 2026-07-18 - Daily Jira Decision Reminders And Viewport Fit

- **Status:** complete.
- Completed and recorded the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits; agreement covers table ownership, whole-row selection, persistent status/reminder ownership, responsive pane behavior, and validation scope.
- Added the `DailyJiraDecision` migration and transactional creation beside `DecisionEvent`. Escalation schedules a 4-hour review; follow-up/reassignment schedule a 24-hour review.
- Extended audit responses with the newest decision state and added dynamic notification-center reminders that ignore superseded/future/resolved decisions and honor `daily_jira_reminder_<decision id>` dismissal keys.
- Moved cohort/search/refresh controls into the table header; made ARIA grid rows clickable from every cell and activatable with Enter/Space; added current decision/due-reminder status to the table and inspector.
- Refit the route to the exact remaining viewport with a 16px bottom gap. Desktop keeps aligned side-by-side panes; medium/narrow uses two bounded rows. Table and inspector remain internal scroll owners and the document does not scroll.
- Restarted the confirmed repository backend so AutoMigrate created `daily_jira_decisions`; the app is running with database configuration version 19 and the existing Jira worker schedule.
- Safe authorized-fixture browser validation of the real component passed pointer/keyboard row selection, header filtering, latest/future/absent decision states, wide/1024/760/480 geometry, 44px narrow controls, zero document overflow, zero unintended table horizontal overflow, and empty console errors. The temporary QA entry was deleted and no decision was submitted.
- Focused and full `internal/server` tests pass. `pnpm -C web check` passes with 0 errors and 74 unrelated existing warnings; production build, diff hygiene, Impeccable layout detection, and Finesse P0 detection pass.
- Reused the existing self-improvement entries for the recurring default Go cache and loopback `httptest` sandbox restrictions, updated their recurrence metadata, and completed validation with `/tmp` GOCACHE plus managed loopback permission.

## Session: 2026-07-18 - Daily Jira Visual Unification

- **Status:** complete.
- Loaded project-local Impeccable plus design-taste-frontend and finesse-ui, including the product, layout, redesign, anti-cheap, and preflight references required by the UI gate.
- Reused the live authenticated Chrome session and inspected Daily Jira with real data at the default wide viewport plus medium/narrow viewport overrides without submitting any decision.
- Compared the surface with the adjacent `决策事项` workbench and confirmed the mismatch is material/radius/hierarchy/responsive spacing, not data ownership or workflow semantics.
- Completed the three-way review and recorded hierarchy, ownership, responsive behavior, validation scope, protection rules, and disagreement resolution before frontend edits.
- Ran the subjective layout assessment before the Impeccable mechanical pre-scan. The detector returned `[]`; no arbitrary Tailwind spacing/z-index patterns exist in the target.
- Implemented the scoped `DailyJiraAudit.svelte` unification: compact matte command bar, independent cohort controls, aligned 20px table/inspector workbench, wider desktop inspector, inset table surface, content-aware stacked height, and secondary-column collapse at narrow widths.
- Svelte/TypeScript checks pass with 0 errors; the 74 existing warnings remain in six unrelated legacy files. Production build, tracked/untracked diff hygiene, Impeccable complete/layout scans, and Finesse P0 scan pass.
- Authenticated screenshot/geometry review passed at 2133x903, medium 3-row stack, CSS 750px, and CSS 478px. Wide panels align exactly; medium table contracts to 340px; narrow tables have no overflow and expose the inspector immediately below.
- Search-to-detail synchronization passed on real `ICA-10903`, then the temporary search was cleared. No decision was submitted and no business data was mutated.
- Final default-width state is left on `每日 Jira / 7 日及以上` with 133 real rows, `ZPU-2769` selected, zero horizontal overflow, and an empty console error list.

## Session: 2026-07-18 - Daily Jira 404 Runtime Recovery

- **Status:** complete.
- Confirmed the live `/api/decision/daily-jira` returned 404 even though the route exists in `internal/server/server.go`.
- Identified port 8080 as a stale `go run cmd/server/main.go` process started on 2026-07-15 from this repository.
- Restarted only that confirmed repository backend using the same command; the database-backed configuration version loaded normally and the Jira worker resumed its existing schedule.
- Verified the direct 8080 route and the Vite-proxied 5173 route both now return 401 without a token, proving the protected Daily Jira route is loaded instead of falling through to 404.
- No source-code change was required for the 404; the fix was runtime reload. The user only needs to refresh the Daily Jira page.

## Session: 2026-07-18 - Daily Jira Morning Review

### Phase 1: Discovery and mandatory review preparation
- **Status:** in_progress
- Started from the user request for a decision-area `每日 Jira` submenu with today / 3-day / 7-day unresolved audits, morning assignment, and retrospective traceability.
- Loaded the repository cold-start route, selected the complex coding preset, and loaded project planning/memory rules.
- Loaded the mandatory Impeccable, design-taste-frontend, finesse-ui, and planning-with-files instructions.
- Recorded the active task, constraints, review gate, phases, and validation scope before implementation.
- Ran Impeccable context discovery for `web`; the project has `DESIGN.md` but no `PRODUCT.md`, which is acceptable for this scoped change.
- Read the product-register guidance and existing Phase 41 design contract, then traced the first decision/Jira route and API touchpoints.
- Confirmed the shell's reusable submenu contract and the App-level view/breadcrumb patterns.
- Confirmed local Jira tasks already retain creation age, last activity, due date, current assignee, unresolved status, and decision logs.
- Logged a discovery-path error for nonexistent `internal/server/task_handlers.go` and switched future inspection to resolved source paths.
- Completed and recorded the mandatory three-way UI review. The gate now permits frontend implementation.
- Fixed the age contract at natural-day cohorts 0, 3-6, and 7+; selected dedicated read/review APIs with structured and legacy audit persistence.
- Implemented `internal/server/daily_jira_handlers.go` and focused tests for cohort boundaries, unresolved/Jira filtering, reassignment persistence, kanban sync, structured history, legacy history, and preservation of stale activity on review-only decisions.
- Added the decision submenu/view wiring in `App.svelte` and `FunctionalAdminShell.svelte`, plus the new responsive `DailyJiraAudit.svelte` table/inspector workbench.
- Focused Go tests passed: `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(BuildDailyJiraAuditResponse|PostDailyJiraReview)' -count=1`.
- `pnpm -C web check`, `pnpm -C web build`, app TypeScript, and `git diff --check` passed. Svelte reports 74 existing warnings in six legacy files and no remaining warning in `DailyJiraAudit.svelte`.
- The first design-detector sweep correctly exposed pre-existing App-level side stripes/gradient motion debt plus four new pure-white fallbacks; the new fallbacks were corrected, while unrelated legacy debt remains outside this feature slice.
- Both target-only design detectors now pass with no finding in `DailyJiraAudit.svelte`.
- The already-running app had no authenticated in-app-browser session, and its backend did not enable loopback dev auth. Switched to an isolated static build plus fixture-only API so authenticated validation cannot mutate real Jira or historical local data.
- Authenticated desktop DOM validation passed submenu state, breadcrumb ownership, default urgent cohort, all three bucket counts, watch/unclassified indicators, populated table selection, detail facts, decision controls, Jira link, and structured decision history.
- Authenticated 1024 and 760 validation passed responsive stacking, mobile sidebar entry, table scroll ownership, detail form/history placement, and the disabled-to-enabled reassign guard. Browser QA also caught and verified the search/detail selection repair; no decision submission was performed.
- Added server-side core-scope enforcement while preserving unassigned Jira for dispatch, plus backend note/assignee length validation.
- Final focused daily-Jira tests and the complete `internal/server` package suite pass; the full suite required managed loopback permission for existing `httptest` webhook cases. TypeScript, production build, `git diff --check`, and both target design detectors pass; browser console has no error entries.
- Files modified for this task: `internal/server/daily_jira_handlers.go`, `internal/server/daily_jira_handlers_test.go`, `internal/server/server.go`, `web/src/App.svelte`, `web/src/components/prototype/FunctionalAdminShell.svelte`, `web/src/components/DailyJiraAudit.svelte`, plus active plan/findings/progress and error-log records.


## Current Task: Surface-Aware Text Contrast

- 2026-07-16: Added the user-specified dark-surface contrast rule and completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits.
- 2026-07-16: Review resolves in favor of a scoped semantic modal foreground contract plus rendered contrast scanning; the light full-height drawer remains visually unchanged.
- 2026-07-16: Authenticated computed styles confirmed the issue on the two visible primary controls (`rgb(1,139,141)` background with `rgb(3,18,25)` text); light drawer ink and dark rail ink are already correctly paired.
- 2026-07-16: Added dedicated filled-accent background/hover/ink tokens and applied them to the target demand/delivery actions plus shared primary-action and selected-date contracts.
- 2026-07-16: Added the detector-required reduced-motion fallback for the shared Button loading spinner.
- 2026-07-16: Authenticated wide and 760×800 visual checks confirm both target controls use 5.70:1 cool-white-on-dark-teal contrast, drawer/rail surface pairings remain correct, horizontal overflow is zero, and console errors are empty.
- 2026-07-16: Final Svelte/TypeScript check, production build, `git diff --check`, and finesse P0 detector gate pass.
- 2026-07-16: **Status:** complete.

## Current Task: Viewport-Height Demand Drawer

- 2026-07-16: Inspected the new screenshot at original resolution and confirmed the active drawer is bounded by the flow-board workspace instead of the browser viewport.
- 2026-07-16: Reapplied the mandatory Impeccable, design-taste-frontend, and finesse-ui review. The shared direction is a viewport-fixed, square-corner, full-height right drawer with a global backdrop and unchanged single-body scrolling/focus lifecycle.
- 2026-07-16: Recorded component ownership, responsive behavior, disagreement resolution, and exact geometry/browser validation criteria before editing frontend code.
- 2026-07-16: Implemented viewport-fixed `100dvh` geometry and square drawer edges; Svelte check, production build, and diff hygiene pass, with detector output limited to documented legacy findings outside this change.
- 2026-07-16: Authenticated wide/mobile geometry found the drawer rectangle correct but the shell top bar still above it because `.workspace-stage` is an isolated stacking context. Activated the pre-agreed portal fallback instead of weakening the global shell isolation rule.
- 2026-07-16: Portaled only the demand-detail and detail-companion overlay roots to `.functional-console`; the global shell isolation contract remains unchanged.
- 2026-07-16: Final authenticated visual and geometry validation passed at 2133×1038 and 760×800. Drawer/header own the top edge, the global backdrop owns the shell, AI companion keeps a 16px gap, Escape/backdrop/focus/body-scroll lifecycles pass, horizontal overflow is zero, and console errors are empty.
- 2026-07-16: Final Svelte/TypeScript check has zero errors; production build, `git diff --check`, and detector P0 gate pass. Detector findings are documented legacy selectors outside this scoped change.
- 2026-07-16: **Status:** complete.

## Current Task: Flow Detail Drawer And Unified Task Filters

- 2026-07-16: Classified as complex frontend coding with shared-component, responsive, and authenticated browser-visible risk.
- 2026-07-16: Loaded cold-start routing, complex coding workflow, tool/memory routing, planning-with-files, project-local Impeccable, design-taste-frontend, and finesse-ui instructions.
- 2026-07-16: Ran Impeccable context for `web`; `PRODUCT.md` is absent, so this scoped refinement proceeds against the existing code and `DESIGN.md` without initiating a new product brief.
- 2026-07-16: Inspected both supplied screenshots at original resolution and recorded the drawer, unified-filter, panel-alignment, responsive, and scroll-ownership acceptance criteria.
- 2026-07-16: Traced demand-detail lifecycle to `DemandKanban.svelte`, confirmed the current detail still uses the generic centered modal host, and found the existing telemetry off-canvas panel as the closest repository interaction reference.
- 2026-07-16: Traced the task toolbar to `TaskKanban.svelte`; shared Select already owns project/owner dropdown behavior, while the categorical risk filter remains a custom segmented strip.
- 2026-07-16: Completed and recorded the mandatory three-way review. The agreed implementation uses a local state-safe demand drawer, shared Select for all categorical task filters, and one desktop table/inspector height contract with deliberate stacked fallbacks.
- 2026-07-16: Phase 3 implementation started only after hierarchy, component ownership, responsive behavior, accessibility, validation scope, and disagreements were recorded.
- 2026-07-16: Confirmed implementation mechanics: the workspace-scoped detail host can become a right-edge drawer in place, the companion needs a wide-screen left-of-drawer override, and execution can reuse the existing aligned panel-height token.
- 2026-07-16: Implemented the demand drawer, focus/Escape/reduced-motion behavior, wide companion placement, non-searchable shared risk Select, and aligned execution table/inspector height contract.
- 2026-07-16: First `git diff --check` and Svelte check pass with 0 errors; removed the only newly surfaced unused risk-button selector before continuing validation.
- 2026-07-16: Reused the signed-in Chrome localhost session and visually reviewed the real flow-board drawer at 2133x1038 and 760x800, including long-body scroll ownership, Tab containment, Escape/focus return, and zero horizontal overflow.
- 2026-07-16: Browser QA found the first wide AI companion collapsed to about 1px. Replaced the percentage-after-padding width calculation with a bounded flex basis, then revalidated a 620px companion, 16px drawer gap, zero overlap, Escape close, and focus return.
- 2026-07-16: Visually reviewed execution tracking at wide desktop and 760px. Risk is a shared dropdown, table/inspector top and bottom deltas are both 0px at desktop, and the 1180px stacked state keeps a 16px gap with equal panel heights.
- 2026-07-16: Final `pnpm -C web check` passes with 0 errors and 74 existing warnings; `pnpm -C web build` and `git diff --check` pass. Finesse detection reports P0=0; full-file detector warnings are legacy rules outside the changed surfaces.
- 2026-07-16: Final authenticated browser smoke check confirms the semantic demand and AI dialogs, 620px companion geometry, 16px gap, no overlap/overflow, clean overlay close state, and an empty console error list.
- 2026-07-16: **Status:** complete.


## Current Task: Event Timeline, Flow Detail, And Execution Filters

- 2026-07-15: Classified as complex frontend coding with shared-component and browser-visible regression risk.
- 2026-07-15: Loaded cold-start routing, complex coding workflow, memory/tool routing, local Impeccable, design-taste-frontend, finesse-ui, ui-design-system, and planning-with-files instructions.
- 2026-07-15: Ran Impeccable project context for `web`; existing `DESIGN.md` found, `PRODUCT.md` absent, so the scoped fix proceeds against existing code.
- 2026-07-15: Recorded the dirty-tree constraint and created the current task addendum before code inspection or edits.
- 2026-07-15: Inspected both source screenshots at original resolution and translated the visible scrollbar, modal-height, duplicate-filter, and dropdown-stacking symptoms into explicit acceptance criteria.
- 2026-07-15: Logged and resolved one ripgrep leading-hyphen pattern error; subsequent CSS token searches use the option terminator.
- 2026-07-15: Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui reviews. All three agree on one scroll owner, canonical project/owner filters in the execution toolbar, shared Select ownership, deliberate responsive reflow, and authenticated browser coverage.
- 2026-07-15: Resolved the demand-detail review disagreement in favor of a scoped geometry and scrollbar-rail repair that preserves long-content scrolling; a broader progressive-disclosure/shared-modal migration is explicitly deferred.
- 2026-07-15: Phase 3 implementation started only after the shared direction and disagreement resolution were recorded in the active plan.
- 2026-07-15: Removed the mediation ledger's nested overflow boundary; consolidated execution project/owner filtering into the active toolbar using shared Select; and repaired demand-detail/companion workspace geometry plus hidden-rail body scrolling.
- 2026-07-15: `pnpm -C web check` passed with 0 errors and 78 existing warnings; production build and `git diff --check` passed. Targeted Impeccable detection found only legacy rules outside the changed surfaces.
- 2026-07-15: Isolated authenticated browser validation passed decision current/all expansion, demand detail at 2048/1280/760, companion alignment and linked close, execution filter combinations, open-menu hit testing, responsive reflow, 44px narrow controls, and zero horizontal overflow.
- 2026-07-15: **Status:** complete.


## 2026-07-14 - Phase 68 corpus approval information architecture started

- Inspected the supplied authenticated screenshot at original resolution and traced the repeated queue/detail content to `CorpusCandidateReview.svelte`.
- Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review. All three agree on source-batch grouping, one primary proposal for ordinary review, on-demand source evidence, and full comparison only for impact review.
- Confirmed the current candidate shape already includes document ID and source-document metadata, so the page can remove repeated presentation without dropping records or changing the backend lifecycle.
- Recorded hierarchy, component ownership, responsive behavior, validation scope, and the comparison disagreement resolution in the active Phase 68 plan before frontend implementation.

## 2026-07-14 - Phase 68 corpus approval information architecture complete

- Rebuilt the queue around source-document batches, keeping every candidate row while rendering shared filename/version metadata once per batch.
- Replaced the ordinary two-document comparison with one Typora-like live candidate workbench; moved metadata and immutable source evidence into explicit disclosures and reused shared Select for all classification controls.
- Kept the current-context versus proposed-content comparison only for `impact_review`, preserved the two-stage publication gate, and removed the duplicated impact heading wrapper.
- Added content-aware title suppression, bounded local queue scrolling, single-column responsive collapse, and mutually exclusive error versus successful-empty presentation.
- Extended the authenticated Settings preview with two document batches, ordinary/high-sensitivity candidates, impact responses, lifecycle responses, and explicit populated/empty/error variants.
- Browser validation passed source grouping, ordinary editor, shared Select open/Escape, source evidence disclosure, high-sensitivity transition, impact comparison, empty/error states, and 1440/1024/760/480 geometry. The final clean run has no console warnings or errors.
- `pnpm -C web check` passes with 0 errors and 78 existing unrelated warnings; `pnpm -C web build`, targeted Impeccable detection, and `git diff --check` pass.

## 2026-07-14 - Phase 67 shared Select and live Markdown composition started

- Audited the native scope dropdown, the shared Select API/interaction contract, and the existing CodeMirror Markdown workbench.
- Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review before editing frontend files.
- Chose a bounded WYSIWYM implementation: canonical Markdown remains unchanged, inactive lines render semantic formatting, and the active line reveals syntax for precise editing.
- Recorded component ownership, responsive behavior, protection rules, browser validation scope, and the rejected lossy `contenteditable` alternative in `task_plan.md`.
- **Status:** in progress

## 2026-07-14 - Phase 67 shared Select and live Markdown composition complete

- Replaced the import form's native scope dropdown with shared Select, including the existing global-scope cleanup, disabled state, adaptive menu, and 44px narrow-screen control sizing.
- Added CodeMirror-native `live` mode to MarkdownWorkbench. Inactive lines immediately render semantic Markdown while the active line exposes syntax; canonical Markdown and existing change/save/commit contracts remain unchanged.
- Made corpus Markdown paste start in `即时排版`; retained explicit `源码 / 分屏 / 阅读` modes for precision editing and compatibility.
- Replaced the detector-flagged blockquote side border with a semantic quote widget and restrained row tint. Final targeted Impeccable detection returns `[]`.
- Svelte check passes with 0 errors and 78 existing warnings; TypeScript, production build, and `git diff --check` pass.
- Isolated authenticated Chrome validation passed Select mouse/keyboard lifecycle, raw Markdown preservation, all live formatting families, fallback modes, zero console/page errors, and zero horizontal overflow at 1440/1024/760/480. Narrow Select/mode controls measure 44px.
- Reviewed screenshots: `output/ui-validation-corpus-live-markdown-1440.png` and `output/ui-validation-corpus-live-markdown-480.png`.
- **Status:** complete

## 2026-07-15 - Phase 72 Markdown, decision ledger, palette, and flow detail redesign complete

- Reduced the shared Markdown toolbar to one default `即时排版` mode while preserving explicit advanced edit and read-only preview lifecycles.
- Hid visible option-list rails across shared Select/MultiSelect and compatible listboxes without removing overflow scrolling or keyboard behavior.
- Replaced the detached full-width decision timeline with a contextual current/all evidence ledger inside the strongest-brain inspector, including empty, automatic, manual, actor, task, time, and commit states.
- Introduced the supplied color library through semantic product tokens with contrast-safe foregrounds and restrained state usage.
- Rebuilt the flow-board demand detail into a wider bounded workbench with flat facts, adjacent actions, one body scroll owner, natural-height Markdown preview, responsive one-column narrow layout, and 44px narrow actions.
- Authenticated safe-fixture browser validation covered decision current/all/empty states, long hidden-rail dropdown overflow, selection/detail ordering, populated detail, Markdown preview/edit, and 1440/1024/760/480 geometry. A later reconnect was blocked by the browser URL policy and was not bypassed.
- Final checks pass: `pnpm -C web check` reports 0 errors and 78 existing warnings in 6 files; production build and `git diff --check` pass. Full-file detectors report only documented legacy findings outside the Phase 72 selectors.
- **Status:** complete

## 2026-07-15 - Phase 72 interface system refinement started

- Reloaded the project-local Impeccable, design-taste-frontend, finesse-ui, project UI-system, and planning-with-files guidance before frontend edits.
- Inspected the supplied flow-board detail screenshot and recorded the nested-scroll, narrow-column, repeated-card, and first-screen hierarchy defects.
- Established the initial product-register direction and semantic mapping for the requested source palette; implementation remains blocked on completing the mandatory isolated layout assessments and target-file audit.
- Completed the mandatory isolated subjective layout assessment and mechanical pre-scan, then recorded the four-skill agreement, disagreement resolution, component ownership, responsive contract, protection rules, and validation scope in `task_plan.md` before frontend edits.
- **Status:** in_progress

## Session: 2026-07-13 - Responses-first LLM Transport And Unified Draft Actions

- **Status:** complete with a safety-scoped browser-streaming validation exception
- Added a unified provider client for OpenAI/Sub2API Responses and native Claude Messages, including sync and SSE parsing, auth headers, file IDs, and legacy Chat Completions URL migration.
- Migrated demand specs, demand deconstruction, autonomous execution, telemetry review, and AI connection tests away from active Chat Completions payloads and `choices[].message` parsing.
- Converted `/api/deconstruct` to provider SSE plus browser NDJSON and updated both Svelte consumers to read the streamed completion through a shared parser.
- Replaced the Completions configuration choice with Responses API and Claude Messages, and moved `撤销草案` directly before `保存契约` in one right-aligned footer group.
- Go package tests, Svelte check, production build, structural footer assertion, and diff hygiene pass. Authenticated browser validation covered both protocol-choice states without saving or calling a provider.
- Full authenticated draft replay was stopped because the isolated copy of historical data activated background delay-alert handling; the safety reviewer rejected a restart to prevent possible external notifications.

## Session: 2026-07-13 - Demand Detail Action Hierarchy And Draft Footer

- **Status:** complete
- Moved `撤销草案` into the bottom contract action row, left-separated from save/approve progression actions, while preserving the no-contract fallback and existing confirmation flow.
- Replaced the detail action pair's inherited purple treatment with shared teal-primary and neutral-secondary semantics.
- Authenticated DG-319 testing passed desktop and narrow layout, hover, keyboard focus, withdrawal open/cancel, action adjacency, and horizontal-overflow checks without saving or mutating business data.
- `pnpm --dir web check`, production build, `git diff --check`, and Phase 61-targeted Impeccable detection pass; browser console errors are empty.

## Session: 2026-07-13 - Deterministic Multi-select Completion

- **Status:** complete
- Reproduced the user's screenshot failure and confirmed the open candidate list covered required roles and owner controls, making their clicks hit reviewer options instead of outside-dismiss logic.
- Reworked the shared multi-select so its bounded options participate in form flow, added selected-count plus `完成选择`, and preserved consecutive multi-selection.
- Fixed Escape reopening, removed the parent container's unconditional open handler, and anchored the chevron to the trigger so it cannot drift into options when the list expands.
- Real-component browser testing passed select, complete, chevron close, outside click, Escape, desktop geometry, and 480px responsive behavior without saving business data.
- Svelte check reports 0 errors; production build, targeted Impeccable complete/layout scans, and diff hygiene pass.

## Session: 2026-07-12 - Review Select Dismissal And Core-member Boundary

- **Status:** complete with an authenticated-browser validation exception
- Repaired `Select` and `MultiSelect` at the shared primitive: capture-phase outside pointer, focus exit, Escape, and an explicit chevron toggle now close either list even inside modal content that stops bubbling clicks.
- Preserved deliberate selection semantics: a single owner selection closes immediately; reviewer multi-select remains open only so multiple people can be selected consecutively.
- Filtered review participants through backend `coreMemberVisibility` and filtered the old-backend frontend compatibility directory through `/api/demands/options`.
- Added directory-signature normalization so a late core-member response clears stale non-core reviewer/owner values before save.
- Focused Go tests, `pnpm --dir web check` (0 errors), production build, targeted Impeccable complete/layout scans, and `git diff --check` pass.
- Authenticated browser interaction was attempted after restarting Vite, but the in-app browser stayed locked to the earlier localhost connection-error page by browser security policy; no workaround or alternate browser surface was used.

## Session: 2026-07-12 - Demand Detail AI Workbench And Executable Spec Draft

- **Status:** complete

- Implemented draft-only withdrawal and atomic cleanup of the matching review contract; frozen specifications remain immutable.
- Expanded AI deconstruction and draft persistence with business rules, main/exception flows, permission rules, data impact, API impact, UI impact, dependencies, risks, acceptance criteria, test plan, repositories, and task breakdown.
- Unified create, detail, and schedule companion ownership; host close now closes the companion and desktop geometry uses the real host width.
- Fixed the adjacent detail-to-schedule transition by capturing the demand before clearing detail state.
- Focused demand-spec tests, `pnpm -C web check`, `pnpm -C web build`, TypeScript compilation, and `git diff --check` passed.
- Authenticated browser validation at 2133x902 measured exact group centering, a 16px panel gap, about 1px host/companion height delta, and zero horizontal overflow for detail and schedule pairs.
- Existing draft `DG-319` displayed the implementation blueprint and guarded withdrawal confirmation; final deletion was intentionally not executed.

## Session: 2026-07-11 - Health Diagnosis Intervention Modal Calibration

### Phase 1: Visual and code audit
- **Status:** complete
- Loaded the existing-project redesign, UI audit, and file-planning workflows.
- Recorded the constraint to reduce contrast without weakening genuine risk/intervention signals.
- Inspected the supplied screenshot and located the modal markup in `ProjectHealthTelemetry.svelte`.
- Captured the visual root cause: six competing accent families plus nested bordered cards, not simply “red is too bright.”
- Traced the modal markup and confirmed it mixes inline layout styles with legacy dark rules and later light overrides.
- Logged a harmless CSS-token search error caused by a missing `rg --` separator.
- Next action: inspect the final active modal override block, then define and implement one scoped modal contract.

### Phase 2: Visual contract
- **Status:** complete
- Kept desktop two-column information architecture and defined a single-column breakpoint below 860px.
- Chose system cool neutrals and teal as the base, with muted semantic accents only where risk meaning requires them.
- Chose body-owned scrolling, reduced nested-card borders, and scoped styles to avoid changing project tables.

### Phase 3: Implementation
- **Status:** in_progress
- Next action: add the modal scope/context header, remove inline layout declarations, add metric tone classes, and implement the final scoped style block.
- Implemented the scoped modal contract and passed production build plus target-component static checks.
- First isolated browser harness attempt failed because `document.createElement` is unavailable in the constrained evaluation surface; logged and switched to direct body markup replacement.
- Direct body replacement was also rejected by the read-only evaluation DOM. Stopped retrying DOM mutation and moved the harness to a temporary preview-served HTML file.

## Session: 2026-07-11 - Streamed LLM Attachments And Demand File Archive

### Phase 1: Architecture correction
- **Status:** complete
- Applied file-planning, self-correction, and existing admin UI workflows.
- Recorded the corrected boundary: original files belong in backend multipart handling, compressed local storage, durable demand/deconstruction association, and provider-aware LLM delivery.
- Confirmed the current deconstruction handler is JSON-only and sends text-only OpenAI-compatible payloads; GORM AutoMigrate is the schema path.
- Verified the official Responses `input_file` contract and accepted `.pdf`, `.doc`, and `.docx` formats. Chose inline Base64 file data for provider delivery and multipart streaming for browser-to-server upload.
- Traced the post-preview import transaction and selected `context_pack_id` as the stable staging link, with archive linkage finalized during `/api/tasks/import`.
- Logged a second discovery-path error caused by naming a nonexistent root `main.go`; no implementation action depended on that failed path.
- Logged and corrected a failed discovery command caused by an unmatched zsh glob.
- Next action: trace provider request types, deconstruction archive creation timing, database migrations, and existing data-directory conventions before changing contracts.

### Phase 2: Contract definition
- **Status:** complete
- Selected streamed Files API upload plus Responses `input_file`, with remote cleanup and local gzip retention.
- Defined staged context-pack association followed by import-time archive association.
- Defined additive multipart fields, bounded file counts/sizes, storage configuration, and attachment metadata.

### Phase 3: Backend implementation
- **Status:** complete
- Next action: add model/migration, storage/provider helpers, handler integration, and import linkage.
- Added the attachment model/migration, gzip storage and multipart reader, streamed Files API uploader, Responses file references, remote cleanup, and import-time archive linking.
- Dependency cleanup hit a pnpm store-location mismatch; logged it and will retry with the already-linked store path.
- Frontend production build and diff hygiene pass; full Svelte check remains blocked by the existing `DemandKanban.svelte:1798` type error.
- Initial Go tests did not reach compilation because the default build cache is outside the sandbox; next run will use `/tmp/well-ambient-gocache`.
- The cache-safe backend run compiled successfully but the broad server package was stopped by an unrelated loopback-binding test; focused new tests remain the validation target.

### Phase 4: Frontend streaming conversion
- **Status:** complete
- Replaced local parsing with up to three retained `File` objects and multipart submission.
- Added attachment IDs to the deconstruction/import state bridge and updated upload copy to describe compressed archival plus LLM parsing.
- Removed browser parsing libraries from package metadata and lockfile.

### Phase 5: Focused validation
- **Status:** complete
- Added and passed a provider-loopback end-to-end test covering upload bytes, file ID use, remote deletion, gzip metadata, demand association, and archive association.
- Updated the test to begin with completions configuration and confirmed attachment calls auto-route to Files + Responses.
- Complete backend suite passes; frontend production build and repository-wide diff hygiene pass.
- `pnpm check` remains independently blocked by the pre-existing `DemandKanban.svelte:1798` union-property error; the production build validates the changed frontend path.

## Session: 2026-07-11 - Demand Difficulty And Task Detail Surface

### Audit
- **Status:** complete
- Applied the UI design-system and file-planning workflows to the two screenshot-scoped issues.
- Traced difficulty from the custom dropdown to the schedule request and confirmed backend normalization accepts the existing option values.
- Traced the task detail mismatch to legacy dark colors inside TaskKanban content rendered in the shared light Modal.

### Implementation And Validation
- **Status:** complete
- Replaced the bespoke difficulty dropdown with shared Select and removed its redundant local open/close state and dark menu CSS.
- Added the `task-detail-surface` light palette contract after all legacy TaskKanban rules, including narrow-screen layout behavior.
- Frontend check passes with 0 errors and 74 existing warnings; production build and diff hygiene pass.
- Focused demand scheduling test passes and confirms selected difficulty persistence.
- Browser interaction remains unverified because both target surfaces require a real authenticated session unavailable to the standalone preview.

## Session: 2026-07-11 - Adaptive Overlays And Decision Timeline

### Corrective Follow-up: Historical Event Dates
- **Status:** complete
- User screenshot showed future-looking times under Today; traced the issue to the backend stripping dates with `Format("15:04:05")` and the frontend defaulting time-only records to today.
- Added `occurred_at` to automatic decision records, populated it from task `LastUpdate` and fallback event timestamps, and retained `time` for compatibility.
- Updated the timeline to prefer the full timestamp and added a safe old-backend rollover rule.
- Verified the reported task IDs against the read-only SQLite database; their dates span June 19 through July 10 rather than today.
- Focused Agenda tests, frontend check/build, and diff hygiene pass.

### Corrective Follow-up: HR-4202 Reschedule Consistency
- **Status:** complete
- Traced the intervention and schedule endpoints to the same TaskTelemetry row and inspected HR-4202 plus its latest decision event read-only.
- Prevented unchanged due dates from being presented or recorded as successful adjustments.
- Added explicit frontend target capture, target-date button copy, server response verification, and a persisted due-date response.
- Added and passed an intervention-to-schedule projection regression for June 25 to July 13 plus a no-op rejection assertion.
- First test-fixture patch matched an earlier `now := time.Now()` in the same file; moved the fixture using the exact test-function boundary and reran successfully.
- Focused server test, frontend check/build, JSON/diff hygiene pass.

### Audit
- **Status:** complete
- Read the repository cold-start rules, complex coding preset, UI design-system skill, file-planning skill, and failure-handling skill.
- Inspected the supplied screenshots and traced each reported behavior to the active shell, shared Select/DatePicker primitives, and `DecisionDashboard.svelte` split log region.
- Confirmed the worktree is already dirty; changes will remain limited to the targeted UI components plus these existing planning files.
- First combined planning-file patch failed because `progress.md` starts with `# Progress Log`, not `# Progress`; re-read the live file and applied this smaller exact-context patch.

### Implementation And Validation
- **Status:** complete
- Added outside-click notification dismissal to the active functional shell.
- Added shared viewport-aware up/down placement and bounded overflow to Select and DatePicker.
- Replaced split automatic/manual log cards with one content-height, date-grouped decision event timeline.
- `pnpm --dir web check` passes with 0 errors and 74 existing warnings.
- `pnpm --dir web build` and `git diff --check` pass.
- Browser-tested shared Select down/up behavior at 1280x520 with zero console errors; authenticated notification/timeline browser testing was unavailable from the no-login preview.
- Initial preview command failed at the repository root because only `web/package.json` exists; retrying with `pnpm --dir web preview` was correct. The sandbox then blocked local binding, and the approved local preview command succeeded.

## Session: 2026-07-10 — KPI Workspace Clean Rebuild

### Phase 1: Failure audit
- **Status:** complete
- Loaded the UI audit, product-dashboard, redesign, and self-improvement workflows because the user rejected the prior KPI result.
- Inspected the supplied screenshot and confirmed the failure is structural, not a spacing issue.
- Identified conflicting legacy grid-area, display-contents, sticky panel, and responsive rules as the root cause.
- Next action: replace `KPIKanban.svelte` entirely with one source-order-aligned component and one scoped style system.

### Phase 2: Clean data and read-model layer
- **Status:** complete
- Replaced the entire legacy KPI component while preserving the performance and report-preview APIs.
- Kept real daily/weekly summaries, member workload, Bug count, delay ratio, requirement base score, capability rating, risks, meetings, and evidence.
- Added backward-safe frontend fallbacks for a currently running backend that has not yet restarted with the newest KPI DTO fields.

### Phase 3: Single-flow workspace
- **Status:** complete
- Implemented one vertical flow: period toolbar, four metrics, delivery/risk overview, people capability workbench, and evidence grid.
- Used local overview and member grids only; each collapses independently without named page-level areas or sticky page inspectors.
- Capped the risk reader at 430px after live measurement exposed unnecessary row stretching.

### Phase 4: Validation
- **Status:** complete
- `pnpm --dir web check` passes with 0 errors and 74 non-KPI warnings, down from 94 because the obsolete KPI CSS was deleted.
- `pnpm --dir web build` and `git diff --check` pass.
- Authenticated browser verification passed at the default 2133px-class viewport, 900px override, and 390px override with zero document-level horizontal overflow.
- Runtime interactions passed: 日报/周报, member selection, personal report, evidence scoping, and team-report restoration. Browser console has zero errors.
- Final skill pre-flight: no fake data, no marketing hero, no decorative motion, no legacy grid tracks, explicit focus states, 44px member controls, reduced-motion skeleton fallback, and bounded table/risk scrolling.

## Session: 2026-07-10 — Main Workspace IA And Metrics Rebuild

### Phase 1: Audit and contracts
- **Status:** complete
- Loaded the UI design-system, product-dashboard refinement, existing-project redesign, and file-planning workflows.
- Locked the product design read to calm precision, low spectacle, and high information density.
- Declared shell-owned page identity, real-data-only metrics, and preservation of existing behavior as protection rules.
- Next action: inspect App shell subnavigation, the five target components, and relevant KPI/scoring API contracts before applying structural edits.

### Phase 2: Decision, Schedule, and Evidence hierarchy
- **Status:** complete
- Removed the three duplicate intro/title cards and preserved their actions by moving controls into the first real data surface.
- Rebuilt Decision's top row around six priority statistics and kept it responsive at three, two, and one columns.
- Authenticated browser verification confirms the Schedule toolbar stays single-line at desktop and all three routes have zero document-level horizontal overflow.

### Phase 3: Task information architecture and People Load
- **Status:** complete
- Added shell-owned Task Table, People Load, and Execution Tracking submenus with child-aware breadcrumbs and scroll reset keys.
- Removed the in-page task title/view-switch card and replaced it with one shared filter/context toolbar.
- Rebuilt People Load as a sortable capacity table plus selected-person risk inspector; authenticated data showed 13 members and no page overflow.
- Kept Task Table and Execution Tracking separate while aligning their project/owner filter language.

### Phase 4: Daily/weekly metrics and capability model
- **Status:** complete
- Removed the KPI hero card and the redundant score/distribution dashboard stack.
- Reorganized the page into period controls, multidimensional health metrics, report evidence, personal capability matrix, and department facts.
- Extended the KPI response with total task/Bug counts, delay ratio, derived requirement base score, scored-item count, and manual-revision count.
- Added a focused unit test covering AI difficulty and manual-estimate base-score paths.

### Phase 5: Validation and pre-flight
- **Status:** complete
- `pnpm --dir web check` passes with 0 errors; the 94 warnings are legacy accessibility/unused-selector noise, including selectors made obsolete by intentionally removed cards.
- Focused KPI/server tests pass; production build and `git diff --check` pass.
- The first sandboxed full Go run reached the existing local-listener test and was blocked from binding `httptest` on `[::1]`; the approved local-listener rerun completed with `go test ./...` passing across every package.
- Added and passed an aggregation assertion for personal task/Bug totals, same-population delay ratio, base-score fallback, AI difficulty scoring, and manual estimate scoring.
- Authenticated Chrome validation covered Decision, Schedule, Evidence, all three Task submenus, KPI weekly/daily switching, and 390px-class narrow layout. All target routes have zero document-level horizontal overflow; the KPI data table scrolls only inside its bounded table shell on narrow screens.
- Anti-cheap/pre-flight outcome: the redesign removes decorative intro cards, uses one existing accent/token system, shows only real API metrics, adds no motion or fake data, and keeps product density high with explicit mobile collapse rules.

## Session: 2026-07-10 — Rounded Surface And Modal Corner Audit

### Phase 1: Audit
- **Status:** complete
- Loaded the UI design-system, existing-project redesign, and file-planning workflows.
- Inspected the supplied screenshot and recorded the likely clipped-layer/root-radius mismatch.
- Next action: inventory shared tokens and every modal/card/panel/container selector before choosing the smallest shared fix.
- Confirmed the screenshot artifact comes from square translucent header/footer backgrounds escaping a `20px` modal whose overflow is intentionally visible for dropdown/date-picker overlays.
- Rejected a blanket `overflow: hidden` modal fix because it would clip interactive overlays; the repair will radius the modal's edge sections and strengthen shared visible-surface primitives separately.
- Audited the shared modal, project-health modal, telemetry drawer, Settings structural resets, and low-radius declarations. Shared modal primitives are already clipped; the screenshot bug is local to the demand/schedule modal family, while global work should target visible surface classes only.
- Completed the production-page inventory across decision, demand, task, KPI, deconstructor, telemetry, shared modal, and Settings surfaces. Outer page containers already converge on the `18px` token or explicit `14px`–`22px` radii.

### Phase 2: Shared and modal repair
- **Status:** complete
- Add a non-clipping rounded-surface contract to shared visible primitives, then give demand/schedule modal edge sections matching inner radii so overlay menus remain usable.
- Hardened global visible surfaces with padding-box background clipping and isolated paint contexts.
- Rounded the shared modal header layer and the demand modal family's header/footer/detail edge layers without clipping escaping dropdown/date-picker overlays.

### Phase 3: Page-specific gap closure
- **Status:** complete
- Confirmed active outer surfaces across Decision, Demand, Task, KPI, Deconstructor, Settings, shared modal, project health, and telemetry already resolve to the rounded design-system contract.
- Preserved intentional small radii for micro-controls and zero radii for transparent structural wrappers and the viewport-edge telemetry drawer.

### Phase 4: Validation
- **Status:** complete
- `pnpm --dir web check` passes with 0 errors and the existing 65 warnings.
- `pnpm --dir web build` and `git diff --check` pass.
- Authenticated browser inspection confirms the new-demand modal's four 1px corner points expose the backdrop, not square header/footer layers; the rendered screenshot has a continuous rounded silhouette.
- Authenticated browser audits pass for Schedule and Decision: exposed surface radii are `14px`–`20px`; the Schedule table's `0px` inner viewport remains safely nested inside a clipped `20px` parent.
- Authenticated browser audits pass for Evidence and Task: the evidence surface is `18px`; the dense task workbench consistently uses a tighter but fully rounded `10px` tier.
- Authenticated main-route audit also covered KPI's loading state and Settings' visible audit panel; no exposed square card/container was found. Static final-rule inspection covers KPI populated panels and Settings child routes.
- Restored the claimed browser tab to its original Schedule page with the new-demand modal open, then released browser control.
- Final rerun: `pnpm --dir web check` passes with 0 errors and 65 existing warnings; `pnpm --dir web build` and `git diff --check` pass.

## Session: 2026-07-10 — Controlled Autonomous Delivery Rollout

### Phase 0: Baseline and planning
- **Status:** complete
- Loaded the repository cold-start rules, design routing, memory guidance, and `planning-with-files` skill.
- Audited the existing strongest-brain plan, intent API, context-pack deconstruction, archive/import path, GitLab telemetry, and AI MR review.
- Checkpointed all pre-existing uncommitted work as `8f50f00 chore: checkpoint admin console and strongest brain baseline`.
- Began the versioned demand-spec, review-contract, controlled-execution, reviewer-resolution, and corpus-feedback rollout plan.
- Added `docs/controlled-autonomous-delivery-plan.md` with lifecycle, contracts, safety boundaries, phase acceptance, and delivery definition.

### Phase 1: Persistence and state machine
- **Status:** complete
- Next action: inspect DB migration, permission seeding, policy conventions, and GitLab configuration before adding models and transition helpers.
- Confirmed GORM AutoMigrate, incremental permission seeds, existing policy wrapper, `RepoMapping` configuration, and an `httptest`-friendly GitLab HTTP pattern.
- Added the four persistence roots, append-only action log, incremental permissions, and a pure execution-run state machine with tests.
- First targeted Go test attempt was blocked before compilation by the sandboxed default Go build cache; no code diagnostic was produced.
- Policy-compliant rerun passed: `go test ./internal/delivery ./internal/db/...`.

### Phase 2: Human-reviewed executable specification
- **Status:** complete
- Implement draft/create/update/read/freeze behavior and review-contract manage/approve behavior with frozen-version immutability.
- Added authenticated, permission-gated spec and contract routes.
- Added archive hydration so a deconstruction archive can initialize the human-reviewable specification while retaining context-pack provenance.
- Added default review-contract generation, approval checks, freeze readiness gates, immutable frozen versions, and stale-run invalidation.
- Focused lifecycle and readiness-blocker tests pass.

### Phase 3: Reviewer resolution and preflight
- **Status:** complete
- Implement deterministic reviewer resolution, protected-path rules, segregation of duties, validated change-set artifacts, and structured preflight checks.
- Added protected-path glob handling, minimum approvals, author/unavailable exclusion, and structured blocker reasons.
- Added permission-gated execution preflight that verifies frozen spec, approved contract, mapped/configured repository, safe file actions, test policy, and reviewer resolution.
- Pure resolver/change-set tests and the endpoint integration test pass.

### Phase 4: Controlled execution and GitLab adapter
- **Status:** complete
- Add idempotent execution runs/actions and an `httptest`-covered GitLab branch/commit/Draft-MR client.
- Implemented run create/list/start/cancel, append-only actions, safe topic branch naming, GitLab project/reviewer/branch/commit/Draft-MR operations, and evidence projection back to the demand.
- First HTTP contract test attempt compiled, then the sandbox blocked the local `httptest.Server` listener before requests executed.
- Added frozen-spec AI change-set generation with source-file budgets, update-path allowlisting, deletion bans, and completions/responses protocol support.
- Added GitLab Pipeline refresh and test-failure state handling.
- Escalated local-only HTTP contract tests pass, including request shape, reviewer IDs, branch/commit/MR sequence, pipeline result, evidence projection, and idempotency.

### Phase 5: Demand workspace and human verification
- **Status:** complete
- Use the UI design-system skill in the existing light admin-console register.
- Move deconstruction/intent work into the demand workspace and add a compact demand-detail delivery control instead of a new cockpit.
- Added `DemandDeliveryControl.svelte` with spec review, contract approval, freeze, AI generation, execute, MR/CI evidence, cancel and named-owner acceptance actions.
- Moved Deconstructor from evidence to the demand-board workspace and made task import create a reviewable spec draft from its archive.
- Added `execution:accept`, named acceptance-owner enforcement, and MR webhook delivery reconciliation.
- `pnpm --dir web build` passes. `pnpm --dir web check` reports only the eight existing `TaskKanban.svelte` `parentDemand` nullability errors; the new component has no errors.
- Human acceptance and MR reconciliation focused Go tests pass.

### Phase 6: Delivery feedback and governed candidates
- **Status:** complete
- Generate idempotent delivery candidates and require explicit review before creating active context facts.
- Added delivery-case, requirement-pattern and risk-rule candidates with provenance, confidence and scope.
- Added permission-gated list/review APIs; acceptance creates a versioned active Context Fact, rejection creates none, and repeated decisions are idempotent.
- Added a compact candidate-review surface to the existing System Design corpus page and refreshes the fact list after promotion.
- Focused candidate quarantine/promotion tests and frontend production build pass.

### Phase 7: Integration and delivery
- **Status:** complete
- Run full backend regression, final frontend build/check, diff hygiene, update plan/progress/findings, and commit.
- Frontend `pnpm --dir web check` now passes with 0 errors and 65 existing warnings after replacing four unsafe `parentDemand` narrowings with existing safe helpers.
- First full Go regression found two baseline failures: local user omission in demand options and a missing evidence-gap decision item. Both were resolved with narrow compatibility fixes that preserve the configured visibility boundary and raw execution evidence.
- Final `GOCACHE=/tmp/well-ambient-gocache go test ./...` passes across all Go packages.
- Final `pnpm --dir web build` passes; `pnpm --dir web check` passes with 0 errors and 65 pre-existing warnings.
- Final `git diff --check` passes. Background-generated `task_status.md` and `well-ambient.db` changes remain intentionally outside the delivery commit.

## Session: 2026-07-10 - Phase 50 Settings Column Height Synchronization

### Implementation And Validation
- **Status:** complete
- Replaced the independent sticky audit height with a desktop grid-stretch contract driven by the left configuration surface.
- Removed audit diff content from intrinsic grid sizing through an absolutely contained panel, preventing long payloads from inflating both columns.
- Restored normal-flow positioning at the stacked breakpoint and retained the responsive audit height cap.
- Replaced legacy local indigo/dark scrollbar styling with the global light-console scrollbar contract for version and diff regions.
- Browser-checked all five integration pages with audit columns at `1600x900`; every left/right height delta is `0px`.
- Browser-checked the `1200x760` stacked layout; no horizontal overflow and internal diff scrolling remain intact.
- `pnpm --dir web build` and targeted `git diff --check` pass. Existing repository-wide Svelte accessibility warnings remain outside this CSS correction.

### Tool Note
- The browser wrapper does not expose `playwright.setViewportSize`; responsive verification uses the documented browser viewport capability instead.

## Session: 2026-07-10 - Phase 49 Settings Adaptive Layout And Control Consistency

### Audit And Root-Cause Pass
- **Status:** complete
- Confirmed the audit panel stretch/compression conflict, duplicated local breadcrumb ownership, GitLab `slice(0, 5)` truncation, mixed Project select implementations, and competing hover/placeholder rules.
- Rebuilt the real Settings QA entry around `SettingsPanel`; the first reload exposed and then fixed a missing explicit `settingsInspector` declaration in the reactive async-summary update.
- Browser verification also exposed stale breadcrumb counts caused by hidden dependencies inside `integrationDetail()`. The breadcrumb now reads the already-reactive inspector fact instead of invoking the untracked helper directly.

### Tool Notes
- A repository-root `package.json` read failed because this workspace keeps the frontend package at `web/package.json`; subsequent package commands use `pnpm --dir web`.
- The in-app locator wrapper does not expose `hover()`. Hover and open states share the same final CSS lock, so browser validation uses the real clicked/open state and computed backgrounds instead of simulating a hover through page evaluation.

### Implementation And Validation
- **Status:** complete
- Added shared non-Settings breadcrumb rendering through `FunctionalWorkspace`; Settings keeps its child-aware breadcrumb but no longer renders a nested first-item button.
- Moved Settings category context exclusively into the breadcrumb and reduced the large summary header from four facts to three operational facts.
- Rebuilt the version inspector with a 420px desktop width, viewport-bounded sticky height, separately scrollable version/diff regions, formatted JSON, and responsive full-width/narrow modes.
- Raised version retrieval from 12 to 40, removed the page-level eight-version/eight-diff truncation, and kept long collections bounded by internal scrolling.
- Removed GitLab's five-row overview slice and added a 360px repository table scroll boundary with sticky headings.
- Replaced Project's bespoke Project Key dropdown and searchable phase/priority fields with shared Select modes.
- Locked shared/native select open and hover surfaces to the light palette and set shared/global input placeholders to 12px.
- `pnpm --dir web build` passes and target `git diff --check` passes.
- Full `pnpm --dir web check` returns only the existing eight `TaskKanban.svelte` `parentDemand` nullability errors and 65 existing warnings; Phase 49 files add no diagnostics.
- Browser verification covers 1600x900, 1280x720, and 390x844 layouts with no document-level horizontal overflow.

## Session: 2026-07-10 - Phase 48 Unified Settings Main-Content Rebuild

### Scope Correction And Audit
- **Status:** complete
- Confirmed that the latest page-level implementation covered GitLab only.
- Separated shared-shell changes from actual child-page rebuilds.
- Loaded Self-Improving, UI design-system, finesse product-UI/redesign guidance, taste anti-slop constraints, and file-based planning guidance.
- Next action: inventory every Settings route, define the shared workbench contract, then rebuild the remaining child pages.

### Error Log
- `ProjectConfig.svelte` large patch failed on attempt 1 because a malformed deletion marker made the expected context invalid. No file changes were applied; retry strategy changed to small exact-context patches.
- A read attempted `web/src/prototype.ts`, but the repository uses `web/src/prototype-entry.ts`; the correct entry was read on the next inspection.
- In-app preview screenshot capture closed the active tab target after all DOM and layout measurements had completed. The tab binding was reacquired for the final static check; no application error or console failure accompanied the tool-side closure.
- The final attempt to keep Vite Preview running on `127.0.0.1:4175` was blocked by sandbox `EPERM`; the required escalation was then rejected because the current Codex usage limit prevented approval. No workaround was attempted. The built `dist/settings-preview.html` artifact and completed browser QA remain valid, but no persistent review URL is available at handoff.

### Shared Contract And Child Pages
- **Status:** complete
- Added `web/src/styles/settings-config-workbench.css` and loaded it from `SettingsPanel.svelte`.
- Migrated Feishu, Jira, Project priority, AI engine, and system-design corpus markup to the shared workbench hierarchy.
- Replaced false ready/connected defaults with explicit unchecked, incomplete, checking, error, and success states.
- Corrected AI engine configuration detection so context facts alone do not mark the model engine configured.
- Removed obsolete page-local style blocks and retained only component-specific rules; subsequent production builds pass without target-page unused-selector warnings.
- Added `settings-preview.html`, a local-only Vite entry that mounts the real six config components behind deterministic API fixtures for visual QA without weakening production authentication.
- Desktop browser review now covers GitLab, Feishu, Jira, Project priority, and AI engine overview/edit states with no document-level horizontal overflow.

### Responsive QA And Final Validation
- **Status:** complete
- System-design list/editor and context-pack result states pass desktop overflow checks.
- First 390px pass found and corrected GitLab grid min-content overflow; the other five child pages stayed within the document width, with Project table overflow isolated to its own scroll container.
- GitLab, Feishu, Jira, Project, and AI narrow edit states now pass document-width checks; Feishu intentionally keeps its four-step control internally scrollable.
- `pnpm --dir web build` passes. Full `pnpm --dir web check` remains blocked only by eight existing `TaskKanban.svelte` nullability errors; no target Settings file reports an error.
- Target-file `git diff --check` passes. Route coverage and the built local QA entry are recorded; a persistent preview process could not be retained because sandbox escalation was unavailable.


## Session: 2026-07-10 - Phase 45 Settings Surface System Rebuild

### Phase 1: Audit and shared system
- **Status:** complete
- **Started:** 2026-07-10
- Actions taken:
  - Loaded the UI design system, existing-project redesign, finesse product UI, and file-planning guidance.
  - Reconfirmed the light admin-console baseline and the user's preference for restrained glass.
  - Recorded the explicit ban on nested container/capsule/control composition.
  - Removed redundant `section-card` wrappers from GitLab, Feishu, Jira, project, AI engine, and AI context routes.
  - Added the Phase 46 flat workbench contract: one glass surface per page column, definition-list summaries, flat form grids, flat audit rows, and restrained semantic states.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/components/SettingsPanel.svelte`

### Phase 2: Browser visual verification and correction
- **Status:** complete
- Actions taken:
  - Production build passed after the structural and visual-system change.
  - Diff hygiene passed for the touched files.
  - Ran in-app Playwright inspection against an isolated local API fixture across all 11 Settings routes.
  - Fixed version-detail compression, rounded version/summary rows, context option wrapping, project table overflow, and permission branch card nesting.
  - Added shared scroll reset so configuration edit modes and menu route changes always open at the workbench top.
  - Rebuilt the <=860px shell as an off-canvas navigation drawer and verified open/close state at 760px.
  - Removed the isolated API fixture and restored the normal Vite proxy after visual verification.
- Files created/modified:
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/config/GitLabConfig.svelte`
  - `web/src/components/config/FeishuConfig.svelte`
  - `web/src/components/config/JiraConfig.svelte`
  - `web/src/components/config/AIConfig.svelte`
  - `web/src/components/config/ProjectConfig.svelte`
  - `web/src/lib/settings-ui.ts`


## Session: 2026-07-07

### Phase 40: High-Fidelity Admin Prototype Reset
- **Status:** complete
- **Started:** 2026-07-07
- Actions taken:
  - Re-ran the UI review with `finesse-skill`, `taste-skill`, `ui-skill`, and the image-first workflow because prior coded passes still did not meet the visual bar.
  - Generated a high-fidelity reference image for a light, flat, frosted-glass management console and copied it to `output/modern-admin-reference-phase40.png`.
  - Added `web/prototype.html` as an unauthenticated standalone Vite entry so the prototype can be reviewed without logging into the main app.
  - Added `web/src/prototype-entry.ts` and `web/src/components/prototype/StandaloneAdminPrototype.svelte`.
  - Reframed the prototype around a dark left rail, clean top command bar, large focus card, metric stack, risk/evidence task table, and right-side inspector.
  - Updated `web/vite.config.ts` so both `index.html` and `prototype.html` are emitted in production builds.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/vite.config.ts`
  - `web/prototype.html`
  - `web/src/prototype-entry.ts`
  - `web/src/components/prototype/StandaloneAdminPrototype.svelte`
  - `output/modern-admin-reference-phase40.png`
- Validation:
  - `pnpm build` in `web` passed and emitted `dist/prototype.html`.
  - `git diff --check` passed.
  - `pnpm check` still reports the three known non-prototype TypeScript errors in `TaskKanban.svelte` and `ProjectConfig.svelte`; no prototype-specific errors were reported.
  - Browser screenshot verification at `http://127.0.0.1:5174/prototype.html` confirmed the standalone prototype renders the left rail, focus card, task table, and inspector.

## Session: 2026-07-06

### Phase 37: Modern Admin UI Prototype
- **Status:** complete
- **Started:** 2026-07-06
- Actions taken:
  - Loaded planning-with-files, `finesse-skill` product UI guidance, and `ui-skill`.
  - Set the design read to product register, restrained glass, evidence-first, SPECTACLE 2, DENSITY 9.
  - Recorded the prototype protection rules: do not revive the rejected decision queue, delivery cockpit, or project health telemetry surfaces.
  - Started with a coded prototype and design-token substrate rather than rewriting existing business pages.
  - Added `modern-admin-tokens.css` with restrained acrylic, semantic status colors, density spacing, focus rings, and reduced-motion behavior.
  - Added a static coded `ModernAdminPrototype` surface covering exception command, weekly decisions, schedule governance, evidence replay, and override preflight.
  - Exposed the prototype as a `config:read`-gated `UI 原型` tab in `App.svelte`.
  - Started Vite dev server for local review; sandboxed bind failed, approved local bind succeeded and Vite selected port 5174 because 5173 was occupied.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/main.ts`
  - `web/src/App.svelte`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/ModernAdminPrototype.svelte`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Local preview is running at `http://127.0.0.1:5174/`.
  - Browser page-text check confirmed the app is reachable but the in-app browser is on the login screen, so visual prototype review requires logging in and opening the `UI 原型` tab.

## Session: 2026-07-05

### Phase 31: Hide Project Health Telemetry
- **Status:** complete
- **Started:** 2026-07-05
- Actions taken:
  - Accepted the product judgment that the visible project telemetry panel still does not prove its practical operator value.
  - Removed `ProjectHealthTelemetry` from the evidence observatory render tree.
  - Removed the now-unused `ProjectHealthTelemetry` import from `App.svelte`.
  - Kept `ProjectHealthTelemetry.svelte` in the codebase so the scoring foundation can be reworked or deleted later with a clearer product contract.
- Files modified:
  - `web/src/App.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed with existing Svelte warnings in other components.

### Phase 30: Project Health Telemetry Triage And Manual Intervention
- **Status:** complete
- **Started:** 2026-07-05
- Actions taken:
  - Applied `finesse-skill` guidance as a restrained product-dashboard refactor.
  - Rebuilt the expanded project health panel around an exception command board: top risk focus, red/yellow/green counts, average health, and priority intervention cards.
  - Derived each project's health level, weakest dimension, evidence gap, decision question, and manual intervention direction from existing SH/EQ/CE/SI/PHDI fields.
  - Replaced raw table columns with compact evidence micro-cells, weakest-dimension diagnosis, and a dedicated manual intervention column.
  - Reworked the details modal into an intervention dossier with urgency, decision question, weakest evidence, human action, source route, and diagnostic evidence.
  - Removed new `ProjectHealthTelemetry.svelte` build warnings by changing the modal close path and deleting unused CSS.
- Files modified:
  - `web/src/components/ProjectHealthTelemetry.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed; remaining Svelte warnings are existing warnings in other components, not in `ProjectHealthTelemetry.svelte`.

## Session: 2026-07-04

### Phase 29: Red-Zone First Paint Filtering
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Loaded the global `ui-skill` before changing the red-zone UI behavior.
  - Confirmed the screenshot showed lower-case Git branch/commit-derived IDs such as `revert-4`, `with-5`, and `middleq-...` being rendered as story cards during the first paint.
  - Changed dashboard bootstrap so config/member filtering finishes before the first agenda fetch populates visible red-zone cards.
  - Added a stable Jira issue-key guard for red-zone agenda items, excluding internal `TASK-*`/`DEMAND-*` and lower-case Git branch slices from the decision cards.
  - Left the existing auto-decision telemetry feed untouched because the reported issue was the red-zone card grid.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/components/DecisionDashboard.svelte`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.
  - Running Vite preview received HMR for `DecisionDashboard.svelte`.

### Phase 28: Schedule And Evidence UI Refinement
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Loaded the global `ui-skill` before changing the touched UI surfaces.
  - Reworked the red-zone diagnostic card header so story/bug type and Jira number sit on one line.
  - Locked red-zone card rows to a stable equal-height grid so upper and lower cards render consistently.
  - Rebuilt the schedule risk calendar as an operational control panel with a primary focus line, refresh action, risk filter chips, clearer bucket hierarchy, and denser event rows.
  - Moved the demand deconstruction engine from the evidence observatory into the schedule governance page, keeping the evidence observatory focused on health telemetry and task/evidence collaboration.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/App.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/DemandKanban.svelte`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.

### Phase 27: Remove Delivery Cockpit And Continue Phase APIs
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Accepted the product judgment that the visible strongest-brain delivery cockpit still does not prove its value.
  - Loaded the global `ui-skill` before touching UI code.
  - Removed `StrongestBrainDeliveryCockpit` from the decision dashboard and deleted the frontend component.
  - Preserved backend strongest-brain read models so later phase-specific pages can reuse evidence, exception, weekly decision, AI trace, override, and authz data.
  - Added `GET /api/strongest-brain/exceptions` as the Phase 2 exception-center API with mode, summary, evidence, close requirements, owners, deadlines, severity, and evidence completeness.
  - Added `GET /api/strongest-brain/weekly-decisions` as the Phase 3 decision-meeting API with mode, summary, decision cards, options, owners, evidence refs, and decision debt.
  - Refactored strongest-brain decision snapshot construction so decision queue, exception center, and weekly decision center share one read model.
- Files created/modified:
  - `.agents/domains/coding.yaml`
  - `.agents/memory/project.jsonl`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/server.go`
  - `internal/server/strongest_brain_handlers.go`
  - `internal/server/strongest_brain_handlers_test.go`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/StrongestBrainDeliveryCockpit.svelte` deleted
- Validation:
  - `git diff --check` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestStrongestBrain(ExceptionAndWeeklyDecisionCenters|DecisionQueue|EvidenceChain|DeliveryCockpit)' -count=1` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.

### Phase 26: Strongest Brain Delivery Transformation Rollout
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Committed the existing workspace first as baseline `adcc981 chore: capture strongest brain baseline`.
  - Loaded complex-task planning/execution guidance and scoped frontend taste guidance to a dense internal delivery cockpit.
  - Spawned three subagents for backend evidence/exception/weekly-decision work, AI trace/readiness/context-pack work, and frontend cockpit integration.
  - Added a delivery cockpit read model over schedule, execution, decision, AI trace, override, and authorization evidence.
  - Extended strongest-brain evidence chain and decision queue read models with missing links, chain status, evidence completeness, exception summary, and weekly decision cards.
  - Added AI output trace, requirement clarification, and context-pack replay read models over existing deconstruction archive and context pack data.
  - Mounted a compact `StrongestBrainDeliveryCockpit` above the decision dashboard without restoring the rejected old decision-queue surface.
  - Registered protected APIs for delivery cockpit, demand readiness, override audit, AI traces, output trace, clarification, and context-pack replay.
  - Kept generated runtime changes in `task_status.md` and `well-ambient.db` out of the implementation commit.
- Files created/modified:
  - `.agents/state.json`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_trace_helpers.go`
  - `internal/server/ai_trace_helpers_test.go`
  - `internal/server/server.go`
  - `internal/server/strongest_brain_delivery_handlers.go`
  - `internal/server/strongest_brain_handlers.go`
  - `internal/server/strongest_brain_handlers_test.go`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/StrongestBrainDeliveryCockpit.svelte`
- Validation:
  - `git diff --check` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(StrongestBrainDeliveryCockpit|StrongestBrainDecisionQueue|StrongestBrainEvidenceChain|AIIntentSummary|AIOutputTraceReadModelReplaysArchiveContextPack|RequirementClarificationReadModelClassifiesQuestions|ImportTasksAutoArchivesContextPackAndTrace|ImportTasksArchivesContextPackID|ParseDeconstructResponseContent)' -count=1` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/... -count=1` passed after approved non-sandbox rerun because sandboxed `httptest` loopback binding is blocked.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.

## Session: 2026-07-03

### Phase 25: Remove Decision Queue Surface
- **Status:** complete
- **Started:** 2026-07-03
- Actions taken:
  - Treated the latest feedback as a removal request rather than another UI explanation pass.
  - Removed the visible strongest-brain decision queue section from the decision dashboard.
  - Removed frontend `/api/strongest-brain/decision-queue` fetching and the 15-second queue polling path.
  - Removed queue-only interfaces, state, derived helpers, handlers, CSS, and responsive selector remnants.
  - Preserved the existing actionable Agenda metrics, filters, task selection, AI plan import, and manual intervention workflow.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
  - `web/src/components/DecisionDashboard.svelte`
- Validation:
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.

### Phase 24: Decision Queue Meaning
- **Status:** complete
- **Started:** 2026-07-03
- Actions taken:
  - Treated the user feedback as a cockpit usefulness gap in the Phase 22 strongest-brain decision queue.
  - Loaded AGENTS cold-start context, planning/execution guidance, and the required frontend taste guidance.
  - Inspected `DecisionDashboard.svelte` and the strongest-brain decision queue API projection.
  - Preserved the backend API contract and carried backend item `source` through frontend normalization.
  - Added a selected decision focus panel showing the current decision kind, idle cost, suggested action, handling entry, and source counts.
  - Added per-card queue source, decision type, no-action cost, and clearer handling entry labels.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
  - `web/src/components/DecisionDashboard.svelte`
- Validation:
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.

### Phase 23: Schedule Governance Risk Calendar
- **Status:** complete
- **Started:** 2026-07-03
- Actions taken:
  - Resumed after Phase 22 implementation commit `2e9ea85`.
  - Loaded AGENTS cold-start context, planning/execution skills, and continuation memory.
  - Selected master-plan Phase 4 as the next executable slice: schedule governance risk calendar.
  - Spawned backend worker Fermat for `/api/schedule/risk-calendar`.
  - Spawned frontend worker James for the demand schedule-view risk calendar surface.
  - Spawned QA explorer Archimedes for schedule/decision/notification contract risk review.
  - Confirmed current dirty `task_status.md` and `well-ambient.db` are outside the intended Phase 23 scope.
  - Closed the subagents after timeout/shutdown and integrated the partial frontend work that appeared in the shared worktree.
  - Added `/api/schedule/risk-calendar` as an additive read model over existing schedule risk rules.
  - Added week/month risk buckets, calendar events, and summary counts for overdue, due soon, stale, and missing schedule.
  - Added a compact risk calendar panel to the demand schedule view with fallback from current schedule rows if the API is unavailable.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/server.go`
  - `internal/server/schedule_risk_calendar_handlers.go`
  - `internal/server/schedule_risk_calendar_handlers_test.go`
  - `web/src/components/DemandKanban.svelte`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestGetScheduleRiskCalendar|TestGetScheduleBuildsDemandTimeline' -count=1` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.
  - `git diff --check` passed.

## Session: 2026-07-02

### Phase 22: Strongest Brain Master Plan Landing, Intent Recognition, And Summary
- **Status:** complete
- **Started:** 2026-07-02
- Actions taken:
  - Committed all existing workspace changes before new implementation: `2a7890c chore: checkpoint current workspace changes`.
  - Loaded required planning and execution skills.
  - Loaded `design-taste-frontend` and scoped it to a dense internal dark cockpit repair, not a marketing redesign.
  - Spawned backend worker Sagan for strongest-brain read models, decision queue, evidence chain, and AI intent/summary API.
  - Spawned frontend worker Hilbert for visible strongest-brain cockpit and AI intent/summary interaction.
  - Spawned QA explorer Averroes for integration risk and validation recommendations.
  - Backend worker loaded AGENTS cold-start files, selected the `coding.complex` preset, and recorded backend-only scope.
  - Backend worker confirmed no frontend edits should be made and parallel-agent changes must not be reverted.
  - Added Phase 3.5 to the master plan for AI intent recognition and summarization.
  - Added `/api/strongest-brain/decision-queue`, `/api/strongest-brain/evidence-chain`, `/api/ai/intent`, `/api/ai/intent-summary`, and `/api/ai/assistant/summary`.
  - Reused schedule and execution read models to build a v1 strongest-brain decision queue with evidence digests.
  - Added a deterministic AI intent/summary fallback so the conversation UI works when AI providers are unavailable.
  - Added the visible strongest-brain decision queue to the decision cockpit and a transparent AI intent/summary panel to the deconstructor.
  - Closed the QA subagent after integrating its findings.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `docs/strongest-brain-phased-master-plan.md`
  - `internal/server/server.go`
  - `internal/server/strongest_brain_handlers.go`
  - `internal/server/strongest_brain_handlers_test.go`
  - `web/src/App.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/Deconstructor.svelte`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestStrongestBrain|TestAIIntentSummary' -count=1` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.
  - `git diff --check` passed.
  - Full `go test ./internal/server -count=1` remains environment-limited in the sandbox because existing `httptest.NewServer` tests cannot bind `[::1]:0`; escalation was unavailable due account usage limit.

## Session: 2026-06-23

### Phase 21: Board Cohesion, Audit Panel, And Modal Lock Polish
- **Status:** complete
- **Started:** 2026-06-23
- Actions taken:
  - Resumed after context compaction and reread the active planning files.
  - Read the required `design-taste-frontend` guidance and applied it as an internal-dashboard repair constraint.
  - Added the current six-part task addendum to `task_plan.md` and `findings.md`.
  - Added a non-secret Jira link-config endpoint so demand/task users can open Jira without `config:read`.
  - Allowed authenticated demand users to load reassignment candidates and protected recent manual assignee overrides from Jira sync rollback.
  - Compacted the schedule effort summary, made difficulty a dropdown, and reduced the config-version audit panel height with section-specific filtering.
  - Confirmed modal scroll locking covers shared Modal, Demand modals, and Settings modals.
  - Made demand cards keyboard/click accessible for details while preserving Jira/direct action click targets.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/config_handlers.go`
  - `internal/server/server.go`
  - `internal/server/jira_worker.go`
  - `internal/server/jira_worker_test.go`
  - `internal/server/server_test.go`
  - `internal/agenda/handlers.go`
  - `internal/agenda/handlers_test.go`
  - `internal/server/demand_handlers.go`
  - `web/src/lib/modalScrollLock.ts`
  - `web/src/components/shared/Modal.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/DecisionDashboard.svelte`

## Latest Validation After Phase 21
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Targeted reassignment/Jira metadata tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server ./internal/agenda -run 'TestShouldPreserveLocalAssignee|TestGetDemandOptions|TestGetJiraLinkConfig|TestPostAgendaDecisionReassignSyncsKanban' -count=1` | targeted backend behavior passes | passed | pass |
| Frontend production build | `pnpm build` in `web` | Vite build succeeds | passed with existing Svelte a11y warnings | pass |

## Latest Error Log
| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-06-23 | Svelte build failed because `{@const detailSubtasks...}` was not an immediate child of an allowed block | 1 | Replaced the inline const with a reactive `detailSubtasks` value before rerunning the build successfully |

## Session: 2026-06-20

### GitLab/Jira Integration Worker A: Webhook Ensure/Status MVP
- **Status:** complete
- Actions taken:
  - Loaded required AGENTS cold-start files and selected the complex coding preset.
  - Inspected `internal/config/config.go`, `internal/server/config_handlers.go`, `internal/server/server.go`, and `internal/telemetry/jira_client.go`.
  - Added protected GitLab webhook installation and inspection handlers.
  - Registered `POST /api/gitlab/webhooks/ensure` and `GET /api/gitlab/webhooks/status` behind `config:write`.
  - Implemented GitLab API list/create/update project hook calls with push and merge request events enabled.
  - Kept GitLab API token and webhook secret out of response bodies and returned status-only GitLab API errors.
  - Added httptest-backed regression coverage for update, create, status, URL defaulting, event flags, and response secret redaction.
- Files modified:
  - `internal/server/config_handlers.go`
  - `internal/server/server.go`
  - `internal/server/config_handlers_gitlab_webhook_test.go`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestEnsureGitLabWebhooks|TestGetGitLabWebhookStatus|TestResolveGitLabWebhookURL' -count=1 -v` passed after rerun with loopback bind permission.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -count=1` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/telemetry -count=1` passed.
- Notes:
  - A sandboxed first attempt at the httptest suite failed because the sandbox blocked local listener binding; the same tests passed with approved loopback access.
  - `internal/server/*` is ignored by `.gitignore`, so normal `git status` does not show those edited files.

### Phase 1: Requirements & Discovery
- **Status:** complete
- **Started:** 2026-06-20
- Actions taken:
  - Loaded required AGENTS cold-start files.
  - Loaded `executing-plans`, `planning-with-files`, and `frontend-design` skill guidance.
  - Read the external implementation plan.
  - Initial file search missed planned backend paths.
  - Located ignored backend server files with `find`.
  - Mapped existing frontend implementation against the plan.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/notification_handlers.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server_test.go`
  - `internal/telemetry/handler.go`
  - `internal/telemetry/handler_test.go`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/dist/*` (generated by `pnpm build`)

### Phase 2: Planning & Structure
- **Status:** complete
- Actions taken:
  - Mapped the plan's backend items to `internal/server/notification_handlers.go` and `internal/server/ai_handlers.go`.
  - Mapped frontend items to `DemandKanban.svelte`, `DecisionDashboard.svelte`, `Deconstructor.svelte`, and related `TaskKanban.svelte` parent-demand display.

### Phase 3: Implementation
- **Status:** complete
- Actions taken:
  - Added top-level department candidate extraction and recursive redaction for WellOS login response logs.
  - Tightened department recursion so non-department `name` fields are not treated as departments.
  - Made `/api/tasks/import` preserve existing metadata, reject unknown `demand_id`, link demands, and sync imported shadow tasks to markdown.
  - Preserved `task_group_id`, due date, creator, and decision logs during GitLab webhook telemetry updates.
  - Filtered archived demand data from `DemandKanban`.
  - Let `DecisionDashboard` custom-select options wrap instead of clipping.
  - Removed invalid duplicate nested `{@const}` in `TaskKanban`.

### Phase 4: Testing & Verification
- **Status:** complete
- Actions taken:
  - Ran targeted Go tests.
  - Ran full Go tests.
  - Ran frontend production build.
  - Noted existing Svelte a11y warnings that do not block build.

### Phase 6: Follow-up Fixes
- **Status:** complete
- Actions taken:
  - Added a project rule requiring `taste-skill` for frontend UI changes, with `frontend-design` as fallback when unavailable.
  - Added department to JWT claims and auth headers.
  - Added `GET /api/me` so the frontend can refresh the current user's department/profile without admin permissions.
  - Expanded WellOS department extraction to handle organization/unit/Chinese keys and department IDs.
  - Updated the app shell to recover department, permissions, and role from JWT and `/api/me`.
  - Updated AI deconstruction import so a selected demand with an existing task group keeps that group for new shadow tasks.
  - Updated the Deconstructor demand dropdown to use semantic button elements for the touched UI.
  - Replaced demand schedule date inputs with a styled shell and custom calendar icon while keeping native date picking behavior.
  - Added focused tests for JWT department claim, `/api/me`, department extraction, and task-group reuse.
- Files created/modified:
  - `AGENTS.md`
  - `.agents/domains/coding.yaml`
  - `.learnings/ERRORS.md`
  - `internal/server/jwt.go`
  - `internal/server/server.go`
  - `internal/server/user_handlers.go`
  - `internal/server/notification_handlers.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/dist/*` (generated by `pnpm build`)

## Test Results
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Targeted Go tests | `go test ./internal/server ./internal/telemetry -v -count=1` | server and telemetry pass | passed | pass |
| Full Go tests | `go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend build | `pnpm build` in `web` | production build succeeds | passed with existing a11y warnings | pass |
| Full Go tests after follow-up | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend build after follow-up | `pnpm build` in `web` | production build succeeds | passed with remaining existing a11y warnings outside the Deconstructor dropdown touched area | pass |
| Phase 45 frontend build | `pnpm -C web build` | production build succeeds | passed; existing DemandKanban and Switch warnings remain | pass |
| Phase 45 TypeScript | `pnpm -C web exec tsc -p tsconfig.app.json --pretty false` | no type errors | passed | pass |
| Phase 45 browser route sweep | all 11 Settings routes | no dark drift, content overflow, or nested field surfaces | passed with isolated local fixture | pass |
| Phase 45 responsive workbench | 1024x768 and 760x900 | content remains visible and navigation usable | passed; drawer verified open and closed | pass |

### Phase 7: Modal, Department, and Date Follow-up
- **Status:** complete
- Actions taken:
  - Applied the requested `design-taste-frontend` skill direction as a constrained internal-dashboard UI repair.
  - Reworked the delete/archive confirmation dialog into a flatter dark system modal with explicit target metadata and action-specific color.
  - Removed the native visual surface from new-demand and schedule date fields by adding a custom display layer over the native date picker hit target.
  - Passed the current user's department from `App.svelte` into `DemandKanban.svelte` and sent it as `creator_dept` when creating a demand.
  - Strengthened backend creator department resolution to fall back from DB to JWT-auth header to request body.
  - Treated `未分配` and `无部门` as placeholders rather than valid departments, so a refreshed login department can overwrite stale placeholder data.
  - Added focused server tests for JWT department fallback, request-body fallback, user table backfill, and placeholder overwrite.
- Files modified:
  - `internal/server/demand_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`
  - `web/src/components/DemandKanban.svelte`

### Phase 8: Brain Binding, Profile, Repo, and Calendar Follow-up
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend` as the active UI constraint for this frontend change set.
  - Moved the readonly personal profile out of the main tab navigation and into the avatar dropdown so users can inspect OS-returned name, avatar, role, permissions, and department at the trigger point.
  - Updated `/api/me` to backfill stale or missing user name, avatar, and department from authenticated JWT headers, including placeholder department values.
  - Passed authenticated avatar through middleware headers so the profile endpoint can repair missing avatar data.
  - Tightened WellOS department extraction so generic display-name fields are only accepted inside department context.
  - Linked demand scheduling and AI decomposition through a stable `brain-{demand_id}` task group when no explicit group exists.
  - Updated AI task imports so selected demands without an existing group reuse the same stable brain group across scheduling and deconstruction.
  - Removed the new-demand repo auto-default from the first GitLab repo; new product demands now submit an empty repo and cards show AI mapping state instead.
  - Replaced native date inputs in new-demand and schedule forms with a custom in-system calendar component.
  - Added focused regression tests for `/api/me` backfill, stable brain-group imports, and department parsing edge cases.
- Files created/modified:
  - `internal/server/server.go`
  - `internal/server/user_handlers.go`
  - `internal/server/notification_handlers.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`
  - `web/src/components/ProfilePanel.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/Deconstructor.svelte`

## Latest Validation Update
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Full Go tests after brain/profile/calendar follow-up | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend production build after brain/profile/calendar follow-up | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |

### Phase 9: WellOS Maintenance Login Degradation
- **Status:** complete
- Actions taken:
  - Added a guarded local fallback for WellOS login outages.
  - Allowed degraded login only for users that already exist locally, have a local user-group/permission snapshot, and pass local cached-password verification from a prior successful WellOS login.
  - Denied degraded login for unknown users or users without local access permissions.
  - Issued degraded JWT sessions with a `wellos-maintenance-fallback` marker instead of a real WellOS token.
  - Recorded degraded logins as `user_login_degraded` audit events.
  - Kept the hardened WellOS request path using HTTP/1.1, no keep-alive reuse, browser-like headers, and one retry for transient EOF/reset errors.
  - Added a frontend maintenance banner when the backend returns `degraded: true`.
  - Added regression tests for allowed degraded login, unknown-user denial, and wrong-password denial.
  - Added an explicit local-development auth switch for OS outages: `WELL_AMBIENT_DEV_AUTH=1`.
  - Restricted dev auth to loopback requests only, creates/updates a local developer user, binds `super_admin`, and marks the session with `local_dev_auth`.
- Files modified:
  - `internal/db/user/user.go`
  - `internal/server/notification_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`

## Latest Validation After Maintenance Fallback
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Degraded login server tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestLogin(FallsBack|DoesNotFallback)' -count=1 -v` | existing local user succeeds, unknown user fails | passed | pass |
| Full Go tests after degraded login | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend production build after degraded login banner | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Dev auth server tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestLogin(FallsBack|DoesNotFallback|UsesDevAuth)' -count=1 -v` | dev auth works only on loopback when enabled | passed | pass |

### Phase 10: System Extension Landing
- **Status:** complete
- Actions taken:
  - Converted the extension recommendations into three bounded implementation tracks.
  - Spawned worker A for GitLab/Jira integration and GitLab webhook ensure/status backend APIs.
  - Spawned worker B for KPI personal performance analysis and daily/weekly report preview.
  - Spawned worker C for AI demand deconstruction analysis fields and frontend display.
  - Applied `design-taste-frontend` as the active UI constraint for touched dashboard UI: internal dark cockpit, low motion, high density, no marketing-style sections.
  - Added GitLab config-panel actions for webhook status inspection and install/update after a successful config save.
  - Integrated worker changes and validated Go plus frontend build.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/config_handlers.go`
  - `internal/server/config_handlers_gitlab_webhook_test.go`
  - `internal/server/server.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_handlers_test.go`
  - `internal/server/kpi_handlers.go`
  - `internal/server/kpi_handlers_test.go`
  - `web/src/components/config/GitLabConfig.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/KPIKanban.svelte`

## Latest Validation After Phase 10
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Targeted extension tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestEnsureGitLabWebhooks|TestGetGitLabWebhookStatus|TestResolveGitLabWebhookURL|TestParseDeconstructResponseContent|TestKPIPerformanceIncludesProcessRiskFields|TestKPIReportPreview' -count=1 -v` | GitLab webhook, AI analysis, and KPI report tests pass | passed with approved loopback bind for httptest | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed with approved loopback bind for httptest | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |

## Latest Validation
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Full Go tests after modal/department/date follow-up | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend production build after modal/department/date follow-up | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on pre-existing TS errors outside this change set | known issue |

### Phase 11: Dashboard UI Control Polish
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend` as a constrained internal dashboard UI repair.
  - Replaced the Override assignee text input with a dark-system custom dropdown populated from the selected item, AI suggestion, core members, and agenda assignees.
  - Replaced the Override deadline native visual surface with a custom dark display shell backed by a transparent clickable native date input.
  - Renamed the red-zone diagnostic filter from code repository language to project filtering while preserving the existing `repo` API field.
  - Replaced the KPI report preview team-view native select with a custom dark dropdown and outside-click dismissal.
  - Added wrapping constraints for Jira custom JQL summary values so long JQL strings remain inside the summary card.
- Files modified:
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/KPIKanban.svelte`
  - `web/src/components/config/JiraConfig.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`

## Latest Validation After Dashboard UI Control Polish
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on pre-existing TS errors outside this change set | known issue |

### Phase 12: AI Deconstruction Estimation & Meeting Replacement
- **Status:** complete
- Actions taken:
  - Extended the AI deconstruction prompt and JSON contract with overall estimated days/hours, overall difficulty, estimate basis, and subtask estimated days/hours/difficulty.
  - Added deterministic normalization so missing task estimates are distributed from the overall estimate using difficulty weights.
  - Added `DeconstructArchive` to persist each imported deconstruction run with input text, mapped repos, task snapshot, analysis snapshot, overall estimate, difficulty, confidence, and task group.
  - Added task-level estimate telemetry fields: estimate days, estimate hours, difficulty, source, and archive ID.
  - Preserved estimate telemetry through GitLab/MR webhook state updates.
  - Updated the Deconstructor UI to show overall estimate/difficulty and editable subtask estimate/difficulty while keeping the existing dark cockpit style.
  - Sent input text, analysis, mapped repos, mock state, and task estimates during one-click import so archives are complete.
  - Added regression tests for estimate normalization and import archive persistence.
- Files modified:
  - `internal/db/db.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_handlers_test.go`
  - `internal/server/server_test.go`
  - `internal/telemetry/handler.go`
  - `web/src/components/Deconstructor.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.learnings/ERRORS.md`

## Latest Validation After Phase 12
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Estimate/import focused tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestParseDeconstructResponseContent|TestImportTasks(LinksDemand|ReusesLinkedDemandTaskGroup|CreatesStableBrainGroupForDemand|ArchivesEstimates|RejectsUnknownDemand)' -count=1 -v` | estimate parsing and archive import pass | passed | pass |
| Telemetry preservation tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/telemetry -run TestProcessWebhook -count=1 -v` | webhook tests pass | passed | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed with approved loopback access for httptest | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on pre-existing TS errors outside this change set | known issue |

### Phase 13: AI Project Context & Hour-Based Deconstruction
- **Status:** complete
- Actions taken:
  - Added AI settings fields for project architecture, delivery workflow, implemented capabilities, estimation guidelines, and default work-hours-per-day.
  - Injected those context fields into the deconstruction prompt, with a fallback well-ambient system summary when fields are empty.
  - Made `estimated_hours` and `overall_estimated_hours` the primary prompt/UI estimate fields while preserving `estimated_days` as derived scheduling/archive compatibility data.
  - Added configured work-hours normalization so non-8-hour day settings remain consistent in parsed AI output.
  - Moved the AI analysis panel to the left input column directly under the deconstruction button, keeping the right panel focused on project mapping and shadow task cards.
  - Updated Deconstructor subtask effort controls from day options to hour options.
  - Removed the local `Switch helperText` type error from `AIConfig.svelte`.
- Files modified:
  - `internal/config/config.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_handlers_test.go`
  - `web/src/components/config/AIConfig.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/IntegrationPanel.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`

## Latest Validation After Phase 13
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Focused AI deconstruction tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestParseDeconstructResponseContent|TestImportTasksArchivesEstimates' -count=1 -v` | parse/import estimate tests pass | passed | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed with approved loopback access for httptest | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on 8 pre-existing TS errors outside this change set; no `AIConfig` type error remains | known issue |

### Phase 14: Path Planning Module Prompt Context
- **Status:** complete
- Actions taken:
  - Read the uploaded workbook `全局领航能力汇总.xlsx`, first sheet `功能列表`.
  - Extracted 39 path-planning module capabilities with category, status, configurability, site/project coverage, and validation notes.
  - Created `docs/ai-context/path-planning-module.md` as the archived source-of-truth summary for AI use.
  - Populated `config.example.yaml` AI context fields with a compact prompt-ready summary that references the archive document.
  - Preserved module scope boundaries: this catalog is path-planning business context, not the full well-ambient collaboration architecture.
  - Validated config parsing and focused AI deconstruction tests.
- Files modified:
  - `config.example.yaml`
  - `docs/ai-context/path-planning-module.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
  - `.learnings/ERRORS.md`

## Latest Validation After Phase 14
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Config package tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/config -run Test -count=1 -v` | config tests pass and YAML remains load-compatible | passed | pass |
| Focused AI deconstruction tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestParseDeconstructResponseContent|TestImportTasksArchivesEstimates' -count=1 -v` | parse/import estimate tests pass | passed | pass |

### Phase 15: Remove Mis-scoped AI Context Center
- **Status:** complete
- Actions taken:
  - Removed the `AIContextCenter` import/render from AI settings while keeping the generic project-context and estimation fields.
  - Deleted the AI context profile upload/list/toggle backend, tests, and database model.
  - Removed `/api/ai-context/*` route registration and stopped appending enabled module profiles to the deconstruction prompt.
  - Cleared path-planning-specific context values from `config.example.yaml`.
  - Deleted the path-planning AI context archive document because it represented the mis-scoped targeted module direction.
  - Updated task memory to mark the module-image direction as superseded by a general system-context configuration model.
- Files modified:
  - `config.example.yaml`
  - `internal/db/db.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server.go`
  - `web/src/components/config/AIConfig.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Files deleted:
  - `docs/ai-context/path-planning-module.md`
  - `internal/server/ai_context_handlers.go`
  - `internal/server/ai_context_handlers_test.go`
  - `web/src/components/config/AIContextCenter.svelte`

## Latest Validation After Phase 15
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | sandbox loopback bind failed, then passed with approved loopback access | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |

### Phase 16: Strongest Brain Context Registry And Policy RBAC
- **Status:** complete
- Actions taken:
  - Wrote the implementation plan to `docs/strongest-brain-implementation-plan.md`.
  - Added AI Context Registry data models for documents, facts, chunks, packs, and pack items.
  - Added context fact list/create/update APIs plus deterministic context-pack preview and persistence.
  - Rewired AI deconstruction to use a persisted context pack before prompt assembly and return `context_pack_id`.
  - Linked imported deconstruction archives to `context_pack_id`, with import-time pack creation fallback.
  - Added policy authorization models, fine-grained permission seeds, deny/allow evaluator, decision audit logging, and `withPermission` compatibility.
  - Added policy list/save, authorization explain, and authorization audit APIs.
  - Reworked AI settings UI so structured facts replace the old visible free-text context center; legacy fields remain backend fallback only.
  - Added strategy/RBAC UI for policy creation, decision explanation, policy list, and audit logs.
- Files modified:
  - `docs/strongest-brain-implementation-plan.md`
  - `internal/db/db.go`
  - `internal/db/user/init.go`
  - `internal/db/user/user.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/authz/policy.go`
  - `internal/server/authz_handlers.go`
  - `internal/server/context_handlers.go`
  - `internal/server/context_handlers_test.go`
  - `internal/server/policy_authorization_test.go`
  - `internal/server/server.go`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/config/AIConfig.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`

## Latest Validation After Phase 16
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Focused context/auth tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(Context|Authorization|WithPermission|Authorize|ImportTasksArchives|ParseDeconstruct)'` | new context, archive, and auth paths pass | passed | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./...` | all Go packages pass | sandbox loopback bind failed, then passed with approved loopback access | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings in untouched DemandKanban/TaskKanban/shared controls | pass |

### Phase 17: AI Engine Tab, Permission Tree, and Policy Workbench Polish
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend`/taste-skill guidance as a restrained internal admin-console refactor.
  - Split the AI settings surface into an independent `AI 引擎配置` tab and a separate `上下文事实` tab backed by the same `AIConfig` component view modes.
  - Replaced native-looking AI context fact selects with button-backed choice grids and styled the number/range/text controls to match the dark product system.
  - Added `GET /api/permissions` so the frontend can use the live seeded permission catalog rather than a stale hard-coded matrix.
  - Rebuilt the visual permission matrix as a namespace-derived permission tree with per-group coverage toggles and responsive vertical layout.
  - Reworked policy authorization from raw selects/checkbox/number inputs into quick-start templates, segmented subject/scope controls, permission action chips, a state toggle, and a priority stepper.
  - Added `Button.size` support because existing config/settings call sites already used `size="small"`.
  - Added focused backend coverage for the permission catalog handler.
- Files modified:
  - `internal/server/server.go`
  - `internal/server/user_handlers.go`
  - `internal/server/policy_authorization_test.go`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/config/AIConfig.svelte`
  - `web/src/components/shared/Button.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
- Validation:
  - `pnpm build` in `web` passed with existing Svelte accessibility warnings in unrelated/touched-old areas.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run TestHandleListPermissionsReturnsSeededCatalog` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server/authz ./internal/agenda ./internal/config ./internal/kanban ./internal/telemetry` passed.
  - Full `go test ./...` and `go test ./internal/server` remain blocked by sandbox loopback restrictions from existing `httptest.NewServer` tests.
  - `pnpm check` still fails on pre-existing `TaskKanban.svelte` and `App.svelte` TypeScript errors; touched Settings/AI/Button files no longer add errors.

## Latest Validation After Phase 17
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Frontend production build | `pnpm build` in `web` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Permission catalog handler test | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run TestHandleListPermissionsReturnsSeededCatalog` | `/api/permissions` handler returns seeded permissions | passed | pass |
| Non-listener package tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server/authz ./internal/agenda ./internal/config ./internal/kanban ./internal/telemetry` | packages pass without local listener use | passed | pass |
| Full server tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server` | server tests pass | blocked by sandbox loopback bind in existing `httptest.NewServer` tests | blocked |
| Frontend type check | `pnpm check` in `web` | type check passes | failed on pre-existing `TaskKanban.svelte` and `App.svelte` errors outside touched Settings/AI/Button files | blocked |

### Phase 18: Demand Creation Sources and Jira Task Demand Model
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend` guidance as an internal dense product UI repair.
  - Added the `/api/demands/options` form metadata endpoint so demand creation can load assignees and projects from users, Jira config/JQL, historical tasks, and configured repositories.
  - Added a project selector to the new-demand modal and kept “暂不指定项目” as the safe default for oral requirements.
  - Replaced div-backed dropdown options with button-backed controls and removed blur/scale animation from the modal backdrop to reduce typing-time visual jitter.
  - Changed Jira issue type mapping so Jira Task/Story/Feature/Epic-like items enter `issue_type = demand`; Bug/Defect-like items remain `bug`.
  - Added `docs/demand-bug-lifecycle-model.md` and updated execution/schedule docs to match the new demand/bug/execution split.
- Files modified:
  - `internal/server/demand_handlers.go`
  - `internal/server/jira_worker.go`
  - `internal/server/jira_worker_test.go`
  - `internal/server/server.go`
  - `internal/server/server_test.go`
  - `web/src/components/DemandKanban.svelte`
  - `docs/demand-bug-lifecycle-model.md`
  - `docs/execution-task-observability-mvp.md`
  - `docs/schedule-table-mvp.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Latest Validation After Phase 18
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Demand/Jira focused Go tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(MapJiraStatus|MapJiraIssueType|BuildJQL|ParseJiraTime|GetDemandOptionsBuildsFormCandidates|CreateDemandUsesAuthenticatedDepartmentFallback|CreateDemandUsesRequestDepartmentFallback)' -count=1 -v` | Jira mapping, demand options, and create-demand fallbacks pass | passed | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte accessibility warnings | pass |
| Frontend type check | `pnpm --dir web check` | type check passes | passed with existing warnings, 0 errors | pass |

### Phase 19: WellOS User Info Profile Refresh
- **Status:** complete
- Actions taken:
  - Added `GET https://wellos.westwell-lab.com/api/user/info?token={token}` support after normal WellOS login succeeds.
  - Used returned `realname`, `avatar`, `email`, and `department_name` to refresh local user records.
  - Generated JWT claims and login response payloads from the refreshed avatar and department values.
  - Left maintenance fallback and local development auth on local cached profile data only.
  - Added a focused regression test for login profile refresh through the new WellOS user-info response.
- Files modified:
  - `internal/server/notification_handlers.go`
  - `internal/server/server_test.go`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Latest Validation After Phase 19
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| WellOS profile refresh tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestLoginRefreshesAvatarAndDepartmentFromWellOSUserInfo|TestGetCurrentUser(ReturnsDepartment|BackfillsDepartmentFromJWT|RefreshesDepartmentFromJWT)|TestGenerateJWTIncludesDepartment' -count=1 -v` | login response, DB, and JWT use user-info avatar and department | passed | pass |

### Phase 20: Demand Schedule Table and Effort Estimate Polish
- **Status:** complete
- Actions taken:
  - Added row-level `scheduled` to the schedule DTO so UI truth is not inferred from workflow status.
  - Extended schedule saving to persist optional AI estimate hours, days, and difficulty.
  - Split the schedule table into schedule, effort, delivery evidence, risk, and update time columns.
  - Removed branch/repo placeholder text from the schedule column and added styled internal scrollbars.
  - Reworked the scheduling modal into an opaque dark-system surface with a layout-stable custom calendar and optional AI effort panel.
  - Updated `docs/schedule-table-mvp.md` with the schedule truth and AI estimate contract.
- Files modified:
  - `internal/server/demand_handlers.go`
  - `internal/server/schedule_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/components/DemandKanban.svelte`
  - `docs/schedule-table-mvp.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Latest Validation After Phase 20
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Schedule focused Go tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestGetScheduleBuildsDemandTimeline|TestDemandAndKPILogic' -count=1 -v` | schedule DTO and schedule save estimate fields pass | passed | pass |
| Frontend type check | `pnpm --dir web check` | Svelte/TS has 0 errors | passed with existing warnings | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing warnings | pass |
| Diff hygiene | `git diff --check` | no whitespace errors | passed | pass |

## Error Log
| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-07-04 | Worker B targeted `go test ./internal/server` could not compile because concurrent strongest-brain symbols were undefined: `enrichStrongestBrainDecisionItems`, `dedupeStrongestBrainDecisionItems`, `buildStrongestBrainWeeklyDecisions`, `buildStrongestBrainDecisionSummary`, `buildStrongestBrainExceptionSummary` | 1 | Left unrelated strongest-brain files untouched; recorded validation blocker and kept Worker B changes scoped to AI/context handlers and tests. |
| 2026-06-20 | `rg --files` did not surface `internal/server` | 1 | Switched to `find` and targeted file reads. |
| 2026-06-20 | Go tests could not write default Go cache in sandbox | 1 | Re-ran with approved elevated `go test`. |
| 2026-06-20 | `pnpm build` failed on invalid nested `{@const}` in `TaskKanban.svelte` | 1 | Removed duplicate nested const from Done lane. |
| 2026-06-20 | `apply_patch` context did not match after `gofmt` | 1 | Re-read formatted snippets, patched with smaller context, and logged to `.learnings/ERRORS.md`. |
| 2026-06-20 | `apply_patch` context did not match while updating `task_plan.md` | 1 | Re-read the exact current section and applied a smaller-context patch. |

### Phase 26: Worker B AI Trace And Requirement Clarification Backend Slice
- **Status:** complete
- Actions taken:
  - Added AI output trace, Context Pack replay, and requirement clarification read-model helpers/handlers without registering new routes in `server.go`.
  - Extended `/api/deconstruct` response with a `trace` object when context pack replay succeeds.
  - Made `/api/tasks/import` auto-build an `import_archive` context pack from imported task snapshots when no explicit `context_pack_id`/`input_text` is supplied.
  - Made deconstruction archive creation require a non-zero `context_pack_id` and return `archive_id`, `context_pack_id`, `imported_count`, and `trace` from import.
  - Added focused tests for trace replay, missing-question tiering/readiness, and import auto-archive trace output.
- Validation:
  - `git diff --check` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(AIOutputTraceReadModelReplaysArchiveContextPack|RequirementClarificationReadModelClassifiesQuestions|ImportTasksAutoArchivesContextPackAndTrace|ImportTasksArchivesContextPackID|ParseDeconstructResponseContent)' -count=1 -v` passed.

## 5-Question Reboot Check
| Question | Answer |
|----------|--------|
| Where am I? | Complete |
| Where am I going? | Deliver concise final summary |
| What's the goal? | Temporarily hide the project health telemetry panel because its practical operator value is still unclear. |
| What have I learned? | A dashboard module should stay visible only when it drives a concrete operator action; telemetry without a proven workflow should not consume the evidence observatory. |
| What have I done? | Removed the `ProjectHealthTelemetry` mount from `App.svelte`, recorded Phase 31 in task memory, and confirmed production build plus diff hygiene. |

### Phase 32: KPI R&D Performance Dashboard Refactor
- **Status:** complete
- Actions taken:
  - Applied `finesse-skill` product UI direction to rebuild the KPI dashboard as a dense evaluation cockpit rather than a loose ranking board.
  - Promoted daily/weekly report granularity to the top-level control and synchronized it with the backend `day`/`week` period values.
  - Added a report brief panel that shows current report window, generation time, delivery totals, task/demand/Bug split, active overdue count, and MR count.
  - Added a team average score panel and an explainable score model: output contribution, code evidence, flow efficiency, and risk health.
  - Replaced the old member ranking cards with a member matrix showing KPI score, task/demand/Bug counts, MR/review/cycle evidence, active/overdue risk, and score breakdown.
  - Reworked the department view into compact department mix cards with proportional delivery bars and task/demand/Bug distribution.
  - Removed stale KPI leaderboard CSS and added responsive styles for the new dashboard structure.
- Files modified:
  - `web/src/components/KPIKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - `.agents/state.json` parsed successfully.
  - `pnpm check` remains blocked by 3 non-KPI errors: `TaskKanban.svelte` references missing `TaskResponse.id`, and `ProjectConfig.svelte` assigns numbers where strings are expected.
  - Existing repository-wide Svelte a11y and unused CSS warnings remain outside the KPI component.

### Phase 33: KPI Core Member Data Boundary
- **Status:** complete
- Actions taken:
  - Added a server-side core member filter to the KPI endpoints so non-core member data is removed before KPI aggregation and report preview generation.
  - Derived core members from `jira.sync_users`; when absent, fell back to assignees parsed from custom JQL `assignee in (...)`.
  - Kept the filter disabled when no core member source is configured to avoid clearing KPI data in unconfigured test/local deployments.
  - Matched core members by display name, username, email prefix, and first token so Jira usernames and local profile names stay aligned.
  - Ensured KPI summary, user rows, department rows, report overview, personal sections, risks, meeting focus, and evidence are all based on the filtered task/user set.
  - Added a regression test proving a non-core member task is excluded from KPI performance and report preview evidence.
- Files modified:
  - `internal/server/kpi_handlers.go`
  - `internal/server/kpi_handlers_test.go`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestKPI(PerformanceIncludesProcessRiskFields|ReportPreviewReturnsEvidenceBackedSections|FiltersNonCoreMemberData|ReportPreviewRejectsUnknownType)' -count=1 -v` passed.

### Phase 34: KPI Interaction Stability And Analysis Depth
- **Status:** complete
- Actions taken:
  - Applied `finesse-skill` product UI guidance for the KPI follow-up as a dense internal performance dashboard, not a decorative redesign.
  - Fixed click/refresh jumping by preserving the already-loaded KPI dashboard while the daily/weekly mode refreshes data.
  - Added an inline sync indicator for background refreshes so the page does not swap to a full loading state after first load.
  - Prevented repeated daily/weekly clicks while a refresh is in flight and removed active-state scale transforms from KPI controls.
  - Followed up on the remaining daily/weekly layout jump by making the sync indicator a fixed reserved slot, locking dynamic copy height, and giving the report/member matrix panels stable minimum heights.
  - Capped the member matrix as an internal scroll area with a sticky header so weekly rows do not stretch the whole page and daily rows do not collapse it.
  - Added a performance depth diagnosis panel for delivery concentration, evidence coverage, risk load, review pressure, Bug share, and score spread.
  - Added a breadth distribution panel for task/demand/Bug mix, score bands, department coverage, and average cycle time.
- Files modified:
  - `web/src/components/KPIKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Existing repository-wide Svelte warnings remain outside the KPI interaction/depth pass.

### Phase 35: Jira Execution Tracking Fact Alignment
- **Status:** complete
- Actions taken:
  - Traced the "Jira Task 开发结果追踪" view from `TaskKanban.svelte` through `/api/execution/tasks` to `execution_handlers.go`.
  - Confirmed bound execution rows were displaying the child execution task's assignee/status, even when the parent Jira Task demand had already been completed or reassigned in Jira.
  - Added explicit `execution_assignee`, `jira_assignee`, and `jira_status` fields to the execution tracking DTO.
  - Made the bound Jira Task demand drive the effective `assignee`, `status`, result state, risk state, last update, and assignee/search filters.
  - Kept the child execution assignee visible as secondary table evidence so historical execution ownership is not lost.
  - Updated Jira sync so a completed Jira status terminates the 24-hour local manual assignee protection window.
  - Updated Jira keep-alive correction sync to continue probing recently completed Jira Task/Bug keys for 14 days, allowing post-completion owner changes and reopen facts to refresh even when the main custom JQL excludes `done`.
  - Added a regression case where the parent Jira Task is `done` and reassigned to Bob while the child execution task still says Alice/progress.
- Files modified:
  - `internal/server/execution_handlers.go`
  - `internal/server/jira_worker.go`
  - `internal/server/server_test.go`
  - `internal/server/jira_worker_test.go`
  - `web/src/components/TaskKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestShouldPreserveLocalAssignee|TestGetExecutionTasksBuildsEvidenceObservability' -count=1 -v` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestShouldPreserveLocalAssignee|TestShouldIncludeInJiraKeepAlive' -count=1 -v` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Existing repository-wide Svelte warnings remain outside the execution tracking fact-alignment fix.

### Phase 36: Core Member Visibility Boundary
- **Status:** complete
- Actions taken:
  - Added a shared coreMember visibility helper that reuses the configured KPI member source: `jira.sync_users` first, custom JQL assignees as fallback, disabled when no member source is configured.
  - Applied the visibility boundary to `/api/tasks`, `/api/schedule`, `/api/execution/tasks`, and `/api/demands/options`.
  - Made `/api/schedule` filter both demand rows and subtask stats so non-coreMember-owned subtasks do not affect the visible schedule projection.
  - Made `/api/execution/tasks` filter by effective current owner and redact non-core `execution_assignee` / `jira_assignee` secondary fields before encoding.
  - Added a KPI report evidence guard so report evidence remains tied to included coreMember users.
  - Removed the visible `外部协同` assignee entries and shortcut branch from `TaskKanban.svelte` and `DemandKanban.svelte`.
- Files modified:
  - `internal/server/core_member_visibility.go`
  - `internal/server/server.go`
  - `internal/server/schedule_handlers.go`
  - `internal/server/execution_handlers.go`
  - `internal/server/demand_handlers.go`
  - `internal/server/kpi_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestGetTasksFiltersNonCoreMemberData|TestGetScheduleFiltersNonCoreMemberData|TestGetExecutionTasksFiltersNonCoreMemberData|TestKPIFiltersNonCoreMemberData|TestGetExecutionTasksBuildsEvidenceObservability' -count=1 -v` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Existing repository-wide Svelte warnings remain outside this data-boundary fix.

### Phase 38: Finesse Componentized Admin Prototype Refactor
- **Status:** complete
- Actions taken:
  - Re-read and applied `finesse-skill` product UI guidance after the first prototype was rejected as too static and not aligned with the requested management-console feel.
  - Reworked `modern-admin-tokens.css` from a more decorative deep-blue glass layer into a flatter charcoal/acrylic token substrate with semantic status colors, stable control sizing, and quieter surfaces.
  - Split the prototype into local reusable pieces: `PrototypeShell`, `PrototypeMetric`, `PrototypeBadge`, `PrototypeTable`, and `PrototypeInspector`.
  - Replaced the single-file concept-board prototype with a dense operational UI: left lane navigation, command header, search/filter/density toolbar, metric strip, exception table, weekly decision queue, selected workflow path, and right-side mediation preflight drawer.
  - Kept the preview behind the existing `config:read` gated `UI 原型` tab instead of replacing current production pages.
  - Attempted browser verification at `http://127.0.0.1:5174/`; the app is reachable, but the in-app browser remained on the login page and browser policy blocked local script navigation for setting a preview auth state.
- Files modified:
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/ModernAdminPrototype.svelte`
  - `web/src/components/prototype/PrototypeShell.svelte`
  - `web/src/components/prototype/PrototypeMetric.svelte`
  - `web/src/components/prototype/PrototypeBadge.svelte`
  - `web/src/components/prototype/PrototypeTable.svelte`
  - `web/src/components/prototype/PrototypeInspector.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - `pnpm check` still fails on 3 pre-existing non-prototype TypeScript errors: `TaskKanban.svelte` references missing `TaskResponse.id`, and `ProjectConfig.svelte` binds numeric values where string values are expected.
  - Existing repository-wide Svelte a11y and unused CSS warnings remain outside the prototype refactor.

### Phase 39: Bold Beauty Admin Prototype Pass
- **Status:** complete
- Actions taken:
  - Treated the feedback "美即正义" as a `finesse-skill` bolder iteration while keeping the scope inside the preview prototype.
  - Pushed the visual direction to an editorial control-room aesthetic: deeper substrate, stronger cyan focus, richer grain, acrylic edges, ambient grid, and more decisive contrast.
  - Added `PrototypeTheatre.svelte` as the new first-screen visual center, combining selected issue focus, risk badge, owner/due/wait/stage facts, signal counts, and direct mediation actions.
  - Reworked `PrototypeShell.svelte` so the rail and app stage feel intentional rather than merely functional.
  - Refined `PrototypeMetric`, `PrototypeBadge`, `PrototypeTable`, and `PrototypeInspector` with stronger hierarchy, richer borders, more elegant status glow, sharper selected-row feedback, and a more instrument-like preflight drawer.
  - Left production pages, auth behavior, backend routes, and real data flows untouched.
- Files modified:
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/ModernAdminPrototype.svelte`
  - `web/src/components/prototype/PrototypeTheatre.svelte`
  - `web/src/components/prototype/PrototypeShell.svelte`
  - `web/src/components/prototype/PrototypeMetric.svelte`
  - `web/src/components/prototype/PrototypeBadge.svelte`
  - `web/src/components/prototype/PrototypeTable.svelte`
  - `web/src/components/prototype/PrototypeInspector.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Filtered `pnpm check` still reports only the 3 known non-prototype TypeScript errors in `TaskKanban.svelte` and `ProjectConfig.svelte`.
  - Browser visual verification still requires logging in to the existing app and opening `UI 原型`.

### Phase 41: Prototype-Aligned Multi-Agent Refactor Calibration
- **Status:** complete
- Actions taken:
  - Paused further page implementation after user feedback that the current style drifted away from the original prototype and data-chain intent.
  - Re-read AGENTS cold-start files, active state, memory hits, and frontend design skills.
  - Reconfirmed the primary visual baseline as `output/functional-admin-console-reference.png`: `well-ambient` brand, dark left rail, light main workspace, table-first middle surface, right inspector, restrained frosted cards.
  - Marked `output/functional-admin-console-reference-alt.png` and `output/modern-admin-reference-phase40.png` as layout references only because they still show `最强大脑`.
  - Spawned a common worker agent to own shared Phase 41 design/data contract work before page agents edit production surfaces.
  - Integrated the common worker output: `DESIGN.md`, `docs/admin-ui-calibration.md`, `modern-admin-tokens.css`, `FunctionalAdminShell.svelte`, `FunctionalWorkspace.svelte`, and `web/src/lib/admin-console/contract.ts`.
  - Integrated Decision, Schedule, Task, Evidence, and KPI page workers with disjoint file ownership and Phase 41 metric/table/inspector vocabulary.
  - Preserved the existing production data chains: decision agenda/intervention APIs, task/schedule/execution APIs, evidence/deconstruction APIs, KPI performance/report preview APIs, and settings/config/RBAC/audit APIs.
  - Closed the stalled Settings worker and completed `SettingsPanel.svelte` in the main thread with a Phase 41 container shell, real adapter metrics, category table, and right inspector while leaving config child components functional.
  - Confirmed no `最强大脑`, `Strongest Brain`, or `StrongestBrain` strings in the migrated page targets.
  - Removed remaining old emoji markers from `SettingsPanel.svelte` page/control copy.
- Files modified in this slice:
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/lib/admin-console/contract.ts`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/prototype/FunctionalWorkspace.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/ProjectHealthTelemetry.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/KPIKanban.svelte`
  - `web/src/components/SettingsPanel.svelte`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check -- web/src/components/SettingsPanel.svelte` passed.
  - Targeted migrated-page brand scan passed.
  - Existing repository-wide Svelte warnings remain, mainly in `DemandKanban.svelte`, shared modal/switch, and config child components; Settings no longer contributes new warnings.

### Phase 42: Settings IA Correction
- **Status:** complete
- Actions taken:
  - Converted the Settings left navigation from flat grouped labels into real collapsible submenus with `aria-expanded`.
  - Replaced the mixed right-side stack of hero, metrics, category table, module content, and inspector with a unified right content frame.
  - Added a breadcrumb bar above the Settings content area: `管理台配置 / group / current section`.
  - Kept the actual business content below the breadcrumb in one content shell so GitLab, Feishu, Jira, Projects, AI, KPI, RBAC, policy, and audit flows render consistently.
  - Preserved existing API/data flows and child config components; this was a container/IA correction only.
- Files modified:
  - `web/src/components/SettingsPanel.svelte`
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check -- web/src/components/SettingsPanel.svelte` passed.
  - Build output has no new `SettingsPanel.svelte` warnings; remaining warnings are existing repository-wide warnings outside this IA correction.

### Phase 43: Settings Rail Navigation Correction
- **Status:** complete
- Actions taken:
  - Moved Settings grouped submenus out of the Settings content frame and into the global `FunctionalAdminShell` left rail.
  - Added Shell-level Settings submenu permission filtering, active section highlight, and callback-driven section navigation.
  - Removed the internal `SettingsPanel.svelte` sidebar markup, sidebar state, resize observer, and sidebar-specific CSS.
  - Kept `SettingsPanel.svelte` as a single breadcrumb-plus-content frame so the right content area remains unified.
  - Preserved existing config, KPI, RBAC, authorization, and audit data flows; this was a navigation ownership correction.
- Files modified:
  - `web/src/App.svelte`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Targeted `SettingsPanel.svelte` scan found no internal sidebar/menu selectors or grouped-navigation helpers.
  - Build output has no new `SettingsPanel.svelte` warnings; remaining warnings are existing repository-wide warnings outside this correction.

### Phase 44: Main Content Prototype Alignment
- **Status:** complete
- Actions taken:
  - Updated the local frontend UI rule so UI work defaults to `ui-skill`, then falls back to `finesse-skill` and `taste-skill`.
  - Flattened `FunctionalWorkspace.svelte` so production pages do not render the old module hero by default.
  - Tightened `FunctionalAdminShell.svelte` main content spacing, topbar sizing, search width, light background, and card shadow tokens toward `output/functional-admin-console-reference.png`.
  - Converted the migrated Demand, Task, KPI, Project Health, Deconstructor, and Settings page-level headers into compact workbench toolbars instead of explanatory hero blocks.
  - Preserved existing page data flows and child panels; this pass changes visual hierarchy only.
- Files modified:
  - `.agents/domains/coding.yaml`
  - `.agents/memory/project.jsonl`
  - `.agents/state.json`
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/prototype/FunctionalWorkspace.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/KPIKanban.svelte`
  - `web/src/components/ProjectHealthTelemetry.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/SettingsPanel.svelte`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Targeted `showContextBar` scan confirmed no production page enables the optional workspace context hero.
  - Build still reports existing repository-wide Svelte warnings in `DemandKanban.svelte`, shared modal/switch, and config child components.

## 2026-07-11 Phase 47 Start

- Read the repository routing instructions and selected the complex coding workflow.
- Applied the UI design-system and file-based planning skills for this cross-surface frontend fix.
- Confirmed the flow scroll failure is a broken bounded-height chain rather than a missing overflow declaration.
- Confirmed AI deconstruction is currently flow-only because `App.svelte` conditionally mounts it only for the board view.
- Confirmed the demand AI-spec typography drift comes partly from invalid CSS font shorthand, and the detail backdrop close condition needs hardening.

## 2026-07-11 Phase 47 Complete

- Made the flow root a bounded flex surface, set the board to consume the remaining height, and made every lane a real independent scroll container.
- Replaced the gray gap with one restrained translucent white surface behind the filter and lane region.
- Hardened demand-detail dismissal so only a direct backdrop click closes it; nested AI actions preserve the detail modal.
- Corrected controlled-delivery field/button typography and tightened its cards, spacing, radii, and textarea heights to the light admin-console baseline.
- Moved AI deconstruction ownership into `DemandKanban` and exposed seeded workbench entry points from new-demand capture, schedule inspection/edit, the flow toolbar, and demand detail.
- Removed the board-only standalone deconstructor mount from `App.svelte` so there is one reusable capability instead of a view-specific page appendage.
- Validation passed: `pnpm build`, `pnpm exec tsc -p tsconfig.app.json --pretty false`, and `git diff --check`.
- Browser preview reached the application login page, but no authenticated preview session was available, so protected-route visual interaction remains unverified in this run.

## 2026-07-11 Phase 48 Complete

- Replaced the disappearing `height: 0` flow board with a `560px–850px` viewport-clamped board and preserved independent lane scrolling.
- Added a compact AI mode that removes the metric strip, document upload, intent console, prompt contract, empty result table, and wide editable evidence table from contextual dialogs.
- Promoted demand association above the input and explained that it controls whether generated tasks reuse a demand task group or remain independent.
- Added compact generated-task rows and kept synchronization analysis below them only after a result exists.
- Made `AI 预解构` open as a right-side companion to the new-demand modal on wide screens, with a centered adaptive fallback on narrower screens.
- Changed new-demand project options to use only `/api/projects/config`, matching the settings project source and canonical project keys/names.
- Disabled horizontal resizing for requirement description textareas.
- Validation passed: `pnpm build`, `pnpm exec tsc -p tsconfig.app.json --pretty false`, and `git diff --check`; existing repository-wide Svelte accessibility/unused-selector warnings remain non-blocking.

## 2026-07-11 Phase 49 Complete

- Updated `.agents/output/delivery.yaml` so every UI change requires authenticated browser simulation and screenshot-based visual review; authentication blockers now require waiting for user input rather than claiming completion.
- Bound the AI companion height to the rendered new-demand modal height and centered the two-dialog desktop group with a 16px gap.
- Split title and description into distinct compact fields and changed `/api/deconstruct` input to labeled `【需求标题】` / `【需求内容】` sections.
- Authenticated browser validation passed at the default responsive viewport and 2048x925.
- Wide-screen measurements: create `680x599.27`, AI `620x597`, group center `1024/1024`, gap `16px`, no horizontal overflow.
- Screenshot review confirmed consistent surface/border/text colors, aligned top/bottom edges, readable information hierarchy, and a centered narrow-screen fallback.
- Saved validation screenshots to `output/ui-validation-demand-ai-responsive.png` and `output/ui-validation-demand-ai-wide.png`.
- `pnpm build`, TypeScript compilation, and `git diff --check` passed; existing repository-wide Svelte warnings remain non-blocking.

## 2026-07-11 Phase 51 Complete

- Added an inline search field to both compact demand-association dropdown render paths in `Deconstructor.svelte`.
- Added real-time matching across demand ID, title, owner, repository, and project, plus a live filtered/total count and explicit no-results state.
- Reset the search query after selection, unbound selection, outside dismissal, and reopening so stale filters do not surprise the next interaction.
- Authenticated browser validation filtered `101` demands to `HR-4202`, selected the result, confirmed the menu closed and the value remained selected, and found no console errors or horizontal overflow.
- Saved and reviewed `output/ui-validation-demand-association-search.png`.
- `pnpm build`, TypeScript compilation, and `git diff --check` passed; existing repository-wide Svelte warnings remain non-blocking.
# Health Diagnosis Intervention Modal Calibration

- Added a dedicated modal scope and clearer title hierarchy without changing diagnosis data or close behavior.
- Reworked the intervention brief, diagnostic evidence, PHDI formula, project facts, metric rows, and recommendations into the light admin-console palette.
- Preserved red/amber/green only as low-saturation semantic states; removed glow, loud multi-color scores, and nested high-contrast surfaces.
- Added responsive one-column layout and compact mobile treatment while keeping scrolling inside the modal body.
- Validation: `pnpm build` passed; targeted `pnpm check` only surfaced the pre-existing `DemandKanban.svelte` union error and an existing unused selector warning; `git diff --check` passed.
- Desktop browser harness passed at 1280×720 with no page overflow and correct modal-owned scrolling. Dynamic narrow-viewport verification was unavailable because the browser facade has no viewport resize method.
# Task Panels Height And Internal Scroll Alignment

- Located the active personnel and Jira Task status layouts and their final light-admin CSS overrides.
- Confirmed this is a layout/overflow ownership defect, not a data rendering issue.
- Added scoped status/personnel modifiers, a shared responsive desktop height, equal-height grid tracks, fixed toolbar rows, and internal table/inspector scrolling.
- `pnpm build` passed with only existing Svelte warnings; `git diff --check` passed.
- Browser harness confirmed exact 480px/480px alignment for both workbenches and verified internal table scrolling with no page overflow.
# Compact Task Rows, Inspector Pills, And Filter Layering

- Located the exact status-row, inspector fact, and top filter dropdown markup in `TaskKanban.svelte`.
- Replaced visible task-type text with accessible SVG icons, compacted status rows to one primary line, converted inspector facts to wrapping capsules, and raised the owning filter surface above subsequent cards.
- `pnpm build` and `git diff --check` pass; build output contains only existing warnings outside this change.
- Browser harness verified row density, title width, icon accessibility, five capsule facts, and an unobstructed open dropdown over the metric grid.

## 2026-07-12 Phase 52 Start

- Applied the UI design-system and file-based planning workflows for the three-part frontend refinement.
- Reviewed the supplied schedule screenshot and recorded the inspector density, inactive global search, and segmented login-field issues.
- Preserved the existing dirty worktree and limited planned edits to the owning frontend components.
- Confirmed ownership: `DemandKanban.svelte` renders the inspector, `FunctionalAdminShell.svelte` owns the inert search input, and `App.svelte` owns the login field group.
- Implemented a real shell search flow with delayed input search, explicit submit, bounded results, empty/error states, and route-aware result actions.
- Replaced the inspector's ruled fact grid with wrapping pills and restyled the login email suffix as an intentional capsule inside one continuous focus surface.
- TypeScript, production build, and diff hygiene passed; the first build exposed two new search ARIA warnings, which were corrected with an explicit combobox contract and option selection state before browser review.
- Authenticated browser validation confirmed live search results and route navigation, eight compact inspector pills, no horizontal overflow, and no console errors at `2133x902`.
- Isolated unauthenticated validation confirmed the login prefix/suffix control at `1280x720` without changing or logging out the user's active Chrome session.
- Added result-ID handoff from the shell to schedule/task components so selecting a global result can clear conflicting filters and focus the exact record after route navigation.

## 2026-07-12 Phase 52 Complete

- Delivered the compact schedule-inspector pill layout, functional global search with exact result focus, and continuous login email field group.
- Final validation passed: TypeScript, production build, `git diff --check`, authenticated search/route/focus interaction at `2133x902`, and isolated login screenshot review at `1280x720`.
- Final browser regression selected non-first-row `HR-4202` in both the table and inspector with no console errors or document horizontal overflow.
# 2026-07-12 - Demand detail AI workbench and executable spec draft

- Loaded the repository cold-start modules and classified the work as `coding.complex`, medium risk.
- Applied UI skill order `ui-design-system -> finesse-ui -> design-taste-frontend`; selected product register, SOUL 4, SPECTACLE 2, DENSITY 7.
- Inspected the dirty tree and preserved existing uncommitted demand-modal, responsive AI, and broader project changes.
- Traced the demand spec lifecycle, deconstruction prompt/schema, delivery controls, and companion modal geometry.
- Confirmed three root causes: no draft-withdrawal API, standalone presentation defaults for detail/schedule, and incomplete AI implementation-spec output.
- Implemented draft-only withdrawal, expanded the AI implementation-spec schema, added the editable implementation blueprint, and unified create/schedule/detail companion ownership.
- TypeScript compile and `git diff --check` passed; focused Go tests require the repository-standard `/tmp` cache because the sandbox blocks the default macOS Go cache.

# 2026-07-12 - Streamed AI specification draft and Markdown preview

- Applied the repository `coding.complex` preset, file-based planning workflow, and UI design-system audit workflow.
- Reviewed the supplied screenshot and recorded the current textarea-matrix, nested-scroll, weak-hierarchy, and Markdown-flattening problems.
- Preserved the existing dirty worktree; planned changes are limited to the AI specification generation/transport/rendering path plus task memory.
- **Status:** discovery in progress
- Implemented the first backend/frontend slice: NDJSON events, provider token streaming, heartbeat frames, transaction-gated completion, persisted Markdown, semantic preview, explicit edit mode, and safe local Markdown rendering.
- The first combined validation command used the wrong working directory for Go paths; frontend production build still completed successfully with only existing repository warnings. Logged the error and split the next validation by package root.
- Added a provider-level SSE regression with deliberately split Markdown/JSON delimiters; its first run was blocked before execution by sandbox loopback restrictions, so focused approved-loopback validation is required.
- Corrected UTF-8 chunk boundaries after the SSE regression exposed multibyte Chinese characters being split by delimiter look-behind; the focused stream/spec lifecycle suite now passes.
- TypeScript, production build, focused Go lifecycle/stream/login tests, and diff hygiene pass.
- Browser validation reached the actual local login route at `127.0.0.1:5175`, but the long-running API does not allow the disposable local account. The login page is handed off for the user to authenticate; Markdown preview/edit and screenshot review remain pending.
- **Status:** implementation and automated validation complete; authenticated visual acceptance pending
- Authenticated browser validation resumed on the real DG-319 flow-board detail modal at 1280×720. The preview has 14 semantic headings, 6 lists, zero textareas, no horizontal document overflow, and modal-owned vertical scrolling.
- Screenshot review found underscore-style Markdown emphasis leaking as literal syntax; added safe `_text_` rendering before continuing visual acceptance.
- Rechecked the live preview after the emphasis fix: `_暂无内容_` no longer appears literally and eight `<em>` nodes render safely.
- Verified the Preview/Edit switch against DG-319 without saving or mutating business data; edit mode exposes every governed field and preview mode restores the semantic document.
- Authenticated screenshots passed at 1280×720 desktop and 760×900 narrow viewport with no document horizontal overflow or browser console errors.
- Final focused Go stream/spec/login tests, TypeScript compilation, production build, and diff hygiene pass. Existing unrelated Svelte warnings remain unchanged.
- **Status:** complete

# 2026-07-12 - Impeccable review gate and shared Markdown workbench

- Installed Impeccable 3.9.1 into `~/.agents/skills/impeccable` and `./.agents/skills/impeccable`; scoped installer output contained no repository metadata, sidecars, or cache files to clean.
- Replaced the old frontend skill fallback with a mandatory Impeccable + taste-skill + finesse-ui review gate in `AGENTS.md`, `.agents/domains/coding.yaml`, `DESIGN.md`, and project routing memory.
- Added the shared CodeMirror/GFM `MarkdownWorkbench.svelte`, migrated live AI output and persisted specification editing, and removed the page-local Markdown renderer.
- Flattened the delivery surface to spacing and hairlines while keeping one bordered editor workbench and collapsing structured execution fields behind progressive disclosure.
- Corrected the first narrow split-height defect discovered during authenticated browser QA.
- Final validation passed: `pnpm check` (0 errors), `pnpm build`, `git diff --check`, Impeccable detector (`[]`), desktop edit/split/preview interaction, 760x900 responsive checks, and zero browser console errors.
- **Status:** complete

## 2026-07-14 - Phase 65 governed corpus ingestion and review center

- Loaded the mandatory project-local Impeccable skill, design-taste-frontend, finesse-ui, and file-planning guidance.
- Classified the surface as a dense product-register Settings workbench and locked a restrained, low-motion, high-density design read.
- Started a narrow audit of the existing corpus promotion seam, LLM path, source persistence, permissions, and shared Markdown editor before any frontend edit.
- Confirmed that the repository already migrates `ContextDocument`, links facts to documents, filters production context to active facts, and promotes accepted candidates transactionally. The implementation can extend these seams instead of replacing them.
- Audited the current AIConfig, candidate queue, and shared Markdown workbench. The target composition is source-first library, one selected-candidate review workbench, and advanced manual fact editing within the existing three-task corpus shell.
- Completed and recorded the mandatory Impeccable, design-taste-frontend, and finesse-ui review before any frontend edit. Shared direction and disagreements are now locked in the active Phase 65 plan.
- Added the source-document model extensions, import/list/archive routes, LLM extraction, review-mode classifier, impact preview, and final publish endpoint. Legacy focused tests pass; the new mock-provider test needs local-loopback permission because sandboxed `httptest` cannot bind `[::1]`.
- **Status:** in progress

# 2026-07-13 - Reviewer multi-select consecutive selection correction

- Restored the expected multi-select session: reviewer selection, option deselection, and expanded-state chip removal now keep the option list open.
- Preserved capture-phase outside-pointer, focus-exit, Escape, explicit completion, and chevron dismissal; updated reviewer guidance to describe consecutive selection and outside-click completion.
- Added a pointerdown focus guard for chip removal after authenticated testing exposed that deleting the focused remove button otherwise triggered focus-exit closure.
- Authenticated DG-319 testing covered desktop and narrow layouts, consecutive selection, outside dismissal, option deselection, chip removal, Escape, `完成选择`, and chevron close with no browser console errors or horizontal overflow.
- Restored all temporary reviewer values before releasing the browser tab; no save or approval action was triggered.
- Final Svelte check passes with 0 errors and 78 existing warnings; production build, targeted Impeccable detection, and diff hygiene pass.
- **Status:** complete

# 2026-07-12 - Inline Markdown editing and review contract layout

- Completed the mandatory Impeccable + taste-skill + finesse-ui review, including isolated layout assessment and mechanical scan.
- Removed visible Markdown mode/save controls from the demand specification while preserving the shared workbench's reusable API.
- Added double-click editing, outside-click temporary persistence, failure-safe edit retention, and a focus-only keyboard entry.
- Rebuilt the review contract into two flat semantic groups with responsive one-column fallback and restrained summary metadata.
- Unified demand-detail and AI companion width ownership at 960px desktop maximum.
- Final validation passed: Svelte check with 0 errors, production build, targeted and layout-scope Impeccable scans (`[]`), `git diff --check`, authenticated DG-319 double-click/save/exit flow, 1280x720 and 760x900 layout metrics, screenshots, and zero browser console errors.
- **Status:** complete

# 2026-07-12 - Flat review contract and directory-backed reviewers

- Removed the duplicate “结构化执行字段” presentation while retaining the governed payload compatibility behind the Markdown document.
- Added the shared `MultiSelect.svelte` primitive with inline search, removable selections, keyboard handling, adaptive placement, and accessible multi-select semantics.
- Rebuilt the review contract as one flat responsive form: multi-select reviewers/roles, shared searchable single-select owners, compact approval count, and a restrained segregation note.
- Stopped preloading Jira `sync_users` as reviewers; default candidates now come only from implementation-task assignees and exclude the specification author.
- Added a permission-scoped participant directory to the review-contract response and retained the already-loaded page directory as a development/backward-compatible source.
- Final validation passed: focused Go tests, Svelte check with 0 errors, production build, diff hygiene, targeted/full layout Impeccable scans (`[]`), and authenticated DG-319 search/multi-select/default-owner/screenshot review.
- **Status:** complete
## 2026-07-13 — Phase 59 select lifecycle correction

- Confirmed the live Vite module was current, so the remaining report was an interaction-contract bug rather than stale frontend output.
- Kept `Select.svelte` and `MultiSelect.svelte` as typed shared primitives and extracted their duplicated outside-pointer/Escape handling into `web/src/components/shared/selectLifecycle.ts`.
- Changed multi reviewer selection and deselection to close immediately and restore focus; removed focus-triggered opening that could move the in-flow list underneath the originating click.
- Updated reviewer helper copy to explain the close/reopen workflow.
- Actual-component browser checks covered selection, deselection, chevron, outside pointer, and Escape for both select modes. Svelte check, production build, complete/layout Impeccable scans, and diff hygiene pass.

## 2026-07-13 — Phase 63 configuration center unified glass workbench

- Removed the Settings-only KPI route and submenu while preserving the top-level KPI page and KPI permission/policy resources.
- Added one shared Settings route registry and routed App access/fallback, shell navigation, and SettingsPanel metadata through it.
- Rebuilt the page context, primary workbench, audit inspector, configuration forms/tables, security surfaces, and modals around one restrained glass-and-hairline system with reduced-motion and reduced-transparency fallbacks.
- Normalized GitLab and the existing shared configuration workbenches without changing their fetch, save, test, rollback, permission, or mutation contracts.
- Authenticated browser QA covered all 10 Settings pages, GitLab overview/edit state, and 2133x902, 1024x900, 760x900, and 480x900 responsive layouts. Fixed the mobile switch inflation found during the first 480px review.
- Final validation passes: TypeScript, Svelte check with 0 errors, production build, targeted Impeccable detection, diff hygiene, and zero browser console errors.
- **Status:** complete

## 2026-07-13 — Phase 64 configuration alignment and dense-page normalization

- Completed the mandatory Impeccable + taste-skill + finesse-ui review and recorded the progressive-disclosure decision before frontend edits.
- Reassigned Settings geometry to one stable unified root so legacy Phase height, stretch, absolute-position, and overflow rules cannot make the audit rail or primary panel abnormally tall.
- Rebuilt policy authorization as four task tabs and system corpus as three task tabs while preserving every existing field, API, permission, result, and mutation path.
- Bounded policy tables, policy action vocabularies, audit history, and corpus candidate presentation; candidate review now uses the shared visual tokens and ten-item pagination.
- TypeScript, Svelte check (0 errors, 78 existing warnings), production build, Impeccable detection (`[]`), and diff hygiene pass. Authenticated browser validation is in progress; the first 2133x902 policy pass has zero horizontal overflow and no document vertical overflow.
- Authenticated browser validation now covers all 10 routes plus every corpus/policy task at desktop and exact 1024/760/480 viewports. GitLab's responsive audit rail is capped to 680/700px, long members/permission/audit datasets are bounded, and every measured route has zero document horizontal overflow.
- A fresh authenticated tab verified the runtime-null memberships guard across permission and member pages with zero console errors. No mutation was submitted.
- Final validation passes: TypeScript, Svelte check with 0 errors and 78 existing warnings, production build, targeted Impeccable detection (`[]`), and `git diff --check`.
- **Status:** complete

## 2026-07-14 - Phase 65 governed corpus ingestion and unified review center complete

- Added versioned raw-document persistence, list/detail/import/archive APIs, Markdown/text normalization, configured-LLM extraction, provenance labels, failure retention, and candidate/document publication counts.
- Extended corpus governance with editable pending candidates, ordinary immediate publication, high-sensitive/global/AI-suggestion impact staging, recorded preview actor/time, preview invalidation on re-review, and immutable final publication of the previewed candidate.
- Rebuilt the corpus route around raw-document management, shared Markdown import/preview, advanced manual fact maintenance, a unified queue/workbench review center, and side-by-side current-versus-proposed impact review.
- Added regression coverage for import extraction, pending exclusion, ordinary publication, pending rejection, impact bypass rejection, recorded preview, final publication, source linkage, document counts, and archive-time withdrawal of unpublished candidates.
- Final checks pass: `go test ./... -count=1`, `pnpm -C web check` (0 errors, 78 existing warnings), `pnpm -C web build`, Impeccable detector `[]`, Finesse detector with zero findings, and target diff hygiene.
- Authenticated fixture-browser validation passed the complete workflow at 1440/1024/760/480 widths with zero horizontal overflow and zero console errors; validation evidence is saved under `output/ui-validation-corpus-*.png`.
- **Status:** complete
## 2026-07-14 - Phase 66 opaque file handoff correction started

- Re-loaded the required project Impeccable, design-taste-frontend, finesse-ui, OpenAI docs, planning, and self-correction guidance.
- Confirmed the current violation on both sides of the boundary: browser `File.text()` plus backend persisted/prompt-concatenated file-derived content.
- Recorded the three-way UI direction, provider/file ownership, persistence boundary, protection rules, validation scope, and the superseded Phase 65 normalization decision in `task_plan.md` before frontend edits.
- The current phase will keep direct Markdown paste as a separate advanced text path while making uploaded files opaque end-to-end.
- The first focused Go test attempt was blocked by the default user cache permissions; TypeScript passed. Validation now uses isolated `/tmp` Go caches instead of repeating the failing command.

## 2026-07-14 - Phase 66 opaque file handoff correction complete

- Added provider-owned opaque file inputs in `internal/llm`: Responses emits `input_file` with filename and inline Base64 data; native Messages returns an explicit unsupported-protocol error instead of parsing locally.
- Replaced file-derived JSON content with multipart transport. The server now limits, validates, hashes, and forwards raw bytes but never decodes or concatenates them into the prompt.
- Changed managed document persistence to use only LLM-returned `document_markdown`, summary, and candidates. Failed uploads retain metadata/hash/failure status with an empty document body, and duplicate completed uploads reuse the existing extraction.
- Reworked the source workbench into mutually exclusive file and Markdown modes. File mode keeps only the selected `File` and transport metadata; Markdown mode reuses `MarkdownWorkbench` for intentional human text.
- Added regression coverage for exact opaque-byte transport, prompt isolation, unsupported protocols, multipart success/missing/oversize/failure/deduplication, LLM-only persistence, and manual Markdown compatibility.
- Final automated checks pass: `go test ./... -count=1`, TypeScript, `pnpm -C web check` with 0 errors, production build, Impeccable detection `[]`, and `git diff --check`.
- Isolated authenticated Chrome validation passed empty, file-selected, submitting, success, failure, Markdown-edit, and Markdown-preview states at 1440/1024/760/480. All widths had zero horizontal overflow; desktop columns shared the same top edge; narrow controls measured 44px; the intentional 502 fixture was the only expected network error and there were zero unexpected console/page errors.
- Reviewed screenshots saved as `output/ui-validation-corpus-opaque-file-desktop.png`, `output/ui-validation-corpus-opaque-file-1024.png`, `output/ui-validation-corpus-opaque-file-760.png`, and `output/ui-validation-corpus-opaque-markdown-480.png`.
- Finesse pre-flight self-grade: the surface remains product-register, preserves one teal accent and the existing type/radius system, introduces no banned decorative patterns, uses explicit action copy, keeps labels above inputs and 44px narrow controls, and honestly claims no spectacle engine.
- **Status:** complete

## 2026-07-14 - Phase 69 transient corpus decision toasts complete

- Completed the mandatory project-local Impeccable, design-taste-frontend, and finesse-ui review before frontend edits. The shared decision is global transient success plus contextual recoverable errors, with product-register styling and feedback-only motion.
- Added `web/src/lib/toast.ts` for a bounded three-notice queue, 4.5-second default dismissal, timer cleanup, and manual dismissal.
- Added `web/src/components/shared/ToastHost.svelte`, reusing `Alert.svelte` for the existing visual vocabulary and adding upper-right safe-area placement, reduced-motion handling, reduced-transparency fallback, responsive width, and 44px narrow close targets.
- Mounted one host in the production `App.svelte` tree and one in the isolated settings preview tree; updated the shared alert close-label contract for accessible Chinese copy.
- Removed the candidate review's local success state, in-container success markup, and success CSS. Reject, ordinary publish, impact-stage approval, and final impact publish now publish to the shared toast queue; contextual errors remain unchanged.
- Final automated validation passes: `pnpm -C web check` with zero errors and existing unrelated warnings, production build, targeted Impeccable detector `[]`, and target `git diff --check`.
- Isolated authenticated browser validation covered ordinary publish, exact final impact-publish copy, automatic dismissal, manual dismissal, contextual 503 error, and 1440/1024/760/480 layouts. Every viewport had zero horizontal overflow and the browser console had zero warnings/errors.
- **Status:** complete

## 2026-07-15 - Phase 70 impact review alignment complete

- Loaded the mandatory project-local Impeccable guidance and completed the design-taste-frontend plus finesse-ui product/redesign review before frontend edits.
- Recorded the shared direction, the superseded Phase 68 side-by-side decision, protection rules, responsive contract, validation scope, and correction log in `task_plan.md`.
- Converted the impact comparison to one vertical reading flow and introduced a shared desktop panel height with explicit queue-list/workbench-body scrolling. Narrow layouts keep natural height.
- `pnpm -C web check` passes with 0 errors and 78 existing unrelated warnings; the production Vite build passes.
- Targeted Impeccable detection returns `[]`, finesse detection returns zero findings, and target diff hygiene passes.
- At the 1440px impact-review pass, the queue and workbench both measure 670px high with identical y=179 and bottom=849; their top and bottom deltas are exactly 0px.
- The impact comparison computes to one 950px column. “当前生效上下文” and “拟发布结果” share x=414 and appear at y=430 and y=830 respectively, confirming the ordered vertical flow.
- The queue list is the left scroll owner (`625/701px`) and the workbench body is the right scroll owner (`530/1150px`); the header and 61px action row remain outside the body scroll. Document horizontal overflow is zero.
- The browser's 1024 override produced a fresh 1269x720 content viewport. At that medium desktop size, impact queue/workbench both measure 520px high with 0px top/bottom delta, comparison remains one 790px column, and document overflow stays zero.
- Visual screenshot review confirms two aligned outer bordered panels, a stable right action footer, and only the current-context comparison leg visible at the top of the right scroll body; the proposed result follows below rather than competing beside it.
- Switching back to an ordinary candidate keeps the same 520px outer heights and 0px top/bottom delta, renders exactly one proposal region and zero comparison panes, and preserves right-body scrolling without changing the workflow.
- Chrome's exact 760 override reports a 749x900 content viewport. The queue and workbench stack at the same x=35, the queue returns to a natural 366px block with a 320px local list, and the workbench returns to natural 1500px height with `overflow-y: visible`.
- The two impact panes remain one 649px column at y=970 and y=1370, document overflow is zero, and all three terminal action buttons measure 44px. Screenshot review confirms a clean queue-first/workbench-second vertical composition.
- At exact 480x900, the queue and workbench stack at x=27 with equal 415px widths; both impact panes share x=42 and 385px width, the three action buttons measure 44x385px, source evidence expands in place, and document horizontal overflow remains zero.
- At exact 1024x900, the desktop contract is active: queue and workbench both start at y=564 and end at y=1204 with identical 640px heights. Queue-list and workbench-body own their respective scrolling, both impact panes share x=414 and 534px width, and document horizontal overflow is zero.
- Authenticated browser coverage now includes impact and ordinary candidates, desktop and stacked layouts, source disclosure, scroll ownership, action sizing, and zero unexpected console warnings/errors. The correction is validated against actual rectangles rather than CSS declarations alone.
- Self-review lesson: distinguish the valid outer queue/detail relationship from a redundant inner comparison grid, then verify both information order and rendered top/bottom geometry at the affected breakpoints.
- **Status:** complete

## 2026-07-15 - Phase 71 workspace-bounded toast positioning complete

- Reloaded the mandatory project-local Impeccable, design-taste-frontend, finesse-ui, planning-with-files, and self-correction guidance before frontend edits.
- Completed isolated layout judgment and mechanical pre-scan. Both confirm that the toast queue/store is valid while the viewport-root host and global fixed layer are not.
- Recorded the shared component ownership, stage geometry, z-index, responsive, protection, and browser validation contracts in `task_plan.md` before implementation.
- Removed the unconditional root host from `App.svelte`, mounted the production host inside a new `FunctionalAdminShell.workspace-stage`, and converted `ToastHost` from viewport-fixed/global-1400 to stage-absolute/local-10 with stage-relative width and max height.
- Updated the isolated settings preview to use the same fixed-header, bounded-stage, internally scrolling content contract so test toasts remain visible when the review workbench is scrolled.
- `pnpm -C web check` passes with 0 errors and the same 78 existing warnings; the production Vite build passes. Impeccable layout detection returns `[]`; targeted ToastHost/preview Impeccable and Finesse scans are clean. Broader scans only report known legacy App/Shell findings outside this positioning change.
- Browser shell geometry at 1280x720: Header is y=0..68, workspace stage is y=68..720, and the empty host anchor is y=80, exactly 12px below the stage start. Header/toast and profile/toast intersection areas are both zero; document overflow is zero.
- A real ordinary-candidate publish in the isolated preview rendered the 58px toast at y=142, exactly 12px inside the y=130..688 content stage. Topbar/toast intersection and page overflow are zero, with 488px remaining below the notice.
- At exact 760x900, the production shell Header is y=0..64 and the stage is y=64..900. The host anchor begins 8px inside the stage, spans only the available workspace width, and has zero Header/profile intersection and zero document overflow.
- A real 760px publish renders a 70px toast at y=138 inside the y=130..868 stage, leaving 660px below it. The notice uses the available 708px stage width and the close target is 44x44px.
- At exact 480x900, a real publish renders the notice at y=153 inside the y=145..890 stage with 667px remaining below, zero topbar intersection, zero page/document overflow, and a 44x44px close target.
- The 480px toast auto-dismisses after the existing timer and manual close removes a fresh notice immediately. Collapsing the desktop rail from 270px to 94px leaves the Header/stage boundary and 12px toast inset intact with zero Header/profile intersection.
- Browser geometry and screenshots show the notice belongs to the review workspace rather than the Header. The selected browser surface did not expose a console-log API, so no new console-log claim is made; every exercised interaction completed without a page-control exception after the documented harness corrections.
- Final `pnpm -C web check` passes with 0 errors and 78 existing warnings; production build, targeted Impeccable, Impeccable layout scan, targeted Finesse, and target `git diff --check` all pass.
- Self-reflection: the prior implementation correctly separated transient success from panel content but chose the browser viewport as the overlay owner. Future shell-level overlays must first identify the intended visual boundary and establish their containing block there before selecting `fixed`, `absolute`, or z-index values.
- **Status:** complete
# 2026-07-18 - Backend LLM deconstruction availability audit complete

- Confirmed effective configuration: AI enabled, token present and redacted, Pixel provider, `gpt-5.5`, Responses API.
- Confirmed real gateway health through the authenticated configuration center.
- Completed one non-synced synthetic pre-deconstruction and received three structured tasks, 76% completeness, 72% confidence, and a 16-hour overall estimate.
- Closed the AI companion and cancelled the host new-demand form; no actual demand or task sync was submitted.
- Focused Go configuration, provider-client, deconstruction, attachment-stream, and demand-spec stream tests pass.
- Residual configuration debt: the database snapshot still stores legacy `endpoint_type: completions`; runtime normalization makes it safe today, but a future explicit save should persist `responses`.

# 2026-07-18 - Daily Jira timing badge clarification complete

- Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review before editing the Daily Jira surface.
- Renamed the first table column and related empty/unclassified copy from age wording to timing wording without changing backend cohorts or reminder policy.
- Replaced the wrapped age/overdue stack with one accessible, non-wrapping dot badge: red overdue, amber due within three natural days, and green healthy, followed by factual creation age.
- `pnpm check` passes with 0 errors and 79 existing warnings; production build, Impeccable layout detection, Finesse detection, and `git diff --check` pass.
- Real-component browser validation passed at 1280/760/480 with all semantic states, today/multi-day copy, keyboard row selection, zero browser-level overflow, a fixed 16px bottom gap, and no console errors. The isolated read-only fixture and local validation server were removed after use.
- **Status:** complete

# 2026-07-20 - Per-user project preferences complete

- Added per-user project preference persistence plus authenticated `GET`/`PUT /api/me/project-preferences`; no saved rows means all projects, while selected mode stores normalized project keys.
- Applied the preference scope to demand, Jira/task, schedule, risk, execution, agenda, strongest-brain, notification, and deconstruction/archive read paths, including direct-record access guards.
- Added an inline preference editor to the profile popover with shared multi-select behavior, explicit all/selected modes, empty-selection protection, and page refresh events for the affected demand/Jira views.
- Full Go suites pass for `internal/db`, `internal/server`, and `internal/agenda`; `pnpm --dir web check` passes with 0 errors and 72 pre-existing warnings; the production web build passes; exact affected-file Impeccable detection returns `[]`; `git diff --check` passes.
- Authenticated isolated browser validation covered default-all, multi-project save, filtered agenda and Daily Jira results, restore-all, and 480px responsive behavior. Agenda results changed from 150 to 6 for HIT + NS2, and every visible Daily Jira row used HIT or NS2.
- Fixed the mobile profile popover discovered during validation; at 480px it now stays within the viewport at x=12..468. The test user was restored to all projects, isolated services were stopped, and no demand/Jira record was created or synchronized.
- **Status:** complete

# 2026-07-20 - Daily Jira bottom ownership and Jira version-link audit complete

- Removed Daily Jira's duplicate viewport-height calculation and connected the component to the same shell/workspace height chain used by Decision Agenda.
- Authenticated 2048x925 comparison measured both routes at the same 22px bottom gutter; 1024x900 and 760x900 preserved 14px and 10px responsive gutters with zero document overflow.
- The Jira release page for `PRJ25024 / 13622` exposes 21 issues through `project = 11900 AND fixVersion = 13622`. Existing `custom_jql` plus `SearchIssues` can ingest that issue set, but the application does not currently parse release-page URLs automatically.
- No Jira sync or business-record mutation was performed. Frontend check/build, diff hygiene, exact-file Impeccable/Finesse scans, and authenticated console validation pass.
- **Status:** complete

# 2026-07-20 - Jira version sources and decision bottom substrate complete

- Added typed Jira version sources with project number, project name, and version-page URL; server-side normalization enforces the configured Jira origin/base path, numeric version ID, project-key agreement, uniqueness, and canonical storage.
- Jira synchronization now unions ordinary/custom JQL with version clauses such as `(project = "PRJ25024" AND fixVersion = 13622)` and includes valid version projects in keep-alive scope and project catalogs.
- Added the responsive Settings editor with add/remove rows, immediate parsing, parsed-key adoption, invalid/duplicate feedback, overview, and confirmation summary. Existing Jira connection and custom-JQL behavior remains intact, with release sources explicitly additive.
- Removed only the decision route's shell bottom inset. Decision Agenda and Daily Jira cards now meet the browser bottom consistently without painting over business surfaces or changing unrelated page gutters.
- Go config/telemetry and focused server tests pass; frontend check/build, Impeccable complete/layout scans, Finesse P0 gate, and `git diff --check` pass.
- Authenticated isolated browser validation passed the supplied version URL and both decision pages at 2048x924 and 760x900 with zero horizontal overflow and no console errors. No save, Jira sync, or business-data mutation was performed; all temporary services were stopped.
- **Status:** complete

# 2026-07-29 - Project board, Jira reverse sync, and decision table complete

- Added the Schedule Governance project board with saved-project scope, priority fallback, a horizontally scrollable selector, and Todo/In Progress/Done lanes with expandable details.
- Added Jira due-date reverse synchronization after successful schedule or decision rescheduling and Jira comment creation from non-empty decision conclusions.
- Added per-user persisted decision-table columns, consistent Bug/Task markers, and uniform status-cell row backgrounds.
- Go package and focused regression tests pass. Frontend check has 0 errors and 73 existing warnings; production build, exact-file Impeccable/Finesse detection, and diff hygiene pass.
- Authenticated isolated browser validation passed at 1440×900 and 390×844, including project switching, lane recomputation, detail expansion, preference persistence across reload, responsive scroll ownership, 44px narrow controls, matching status backgrounds, and an empty console error list.
- Jira, GitLab, and AI writes were disabled during validation. No production data was changed; temporary services and database files were removed.
- **Status:** complete

# 2026-07-30 - Delivery domain convergence implementation complete

- Froze the delivery domain vocabulary and SQLite baseline before adding project, release, release-link, event, sync-operation, outbox, parent-work-item, and revision facts.
- Added a single delivery-planning service and routed new work-item planning APIs plus the legacy schedule mutation adapter through its project/release gates, CAS checks, audit reason, atomic event, and outbox rules.
- Added Jira release reconciliation, ambiguity handling, manual-primary preservation, migration dry-run tooling, compatibility counters, and `/api/delivery/quality` deletion-gate metrics.
- Added the version-planning workbench, kept Project Board read-only, separated its card detail action from a persistent Jira anchor, exposed owners in collapsed cards, and restricted Task Tracking to execution tasks with optional parent context.
- Full Go tests pass; frontend check has zero errors; production build, diff hygiene, and exact-file Impeccable detection pass. Finesse has zero P0 findings and no findings in the two new workbench surfaces.
- Authenticated isolated browser validation passed Project Board pointer/keyboard/Jira/owner behavior at desktop and narrow widths, Version Plan selection stability and internal scrolling, and execution-only Task Tracking. No real Jira navigation or business mutation was performed.
- Phase 8 physical deletion remains gated because the compatibility counters have not yet observed one stable release cycle with zero calls. The compatibility routes are marked deprecated and measurable, but not unsafely removed.
- **Status:** implementation complete; compatibility deletion pending observation gate
# 2026-07-31 - Version Plan multi-Jira association complete

- Fixed the empty “all projects” Select clear affordance and portaled shared dropdowns to the body so modal/inspector overflow no longer clips long project options.
- Routed version creation success and failure through the upper-right Toast while retaining field-local validation.
- Replaced Jira release-version selection with project-scoped Jira work-item search, atomic multi-select association, linked-item listing, and individual removal.
- Added release Jira-item counts and Project Board primary-release projection without per-row queries; linked Jira cards now show the local version name.
- Kept `work_item_release_links` as the single relationship source, added candidate/count/projection indexes, blocked conflicting project changes, and prevented local associations from producing Jira fixVersion outbox writes.
- Added the local release ownership ADR and a database schema/index/query design document. Legacy `release_jira_links` remains compatibility-only.
- Full `go test ./...` passed. `pnpm check` passed with 0 errors and 82 pre-existing warnings; production build passed; exact affected-file Impeccable detection returned `[]`.
- Authenticated isolated browser validation passed the desktop workflow and 744px responsive state, including both creation Toast paths, long dropdown options, two-item batch association, linked-item removal surface, and Project Board version badges. No production or real Jira data was changed.
- **Status:** complete

# 2026-07-31 - Version Plan batch-selection height stabilization complete

- Reproduced the exact cumulative-growth defect in the authenticated Version Plan inspector: 12 selected Jira items grew the selector from 38px to 422px and the inspector scroll content from 556px to 940px; one long item already grew the trigger to 70px.
- Enabled the shared MultiSelect summary presentation only for the batch Jira call site. The selector now retains a fixed control height and displays a count such as “已选 30 项”, while the overlay continues to own complete candidate display, search, clear, and continuous selection.
- Authenticated browser validation passed at 1440×900 and 390×900. Selecting 30 of 48 candidates kept the trigger at 36px, preserved inspector height, kept the overlay inside the viewport, produced no horizontal overflow, and left browser error logs empty.
- `pnpm check` passes with 0 errors and 79 existing warnings; production build, exact DeliveryPlan Impeccable/Finesse detection, and targeted whitespace checks pass.
- Validation used the isolated local database with Jira, GitLab, Feishu, and AI disabled. No batch-association request or production write was submitted.
- **Status:** complete

# 2026-07-31 - Schedule inspector and task-directory convergence complete

- Rebalanced the Schedule Board into a fluid table plus a 420–560px content-owned inspector, with independent body scrolling, clearer fact/edit/action/risk hierarchy, complete tab keyboard semantics, and responsive 3+2 / 2+2+1 layouts.
- Made execution assignee facets merge the authorized user directory, configured members, and visible local-task owners. Jira aliases now resolve through the shared identity directory, while unmapped local owners remain available.
- Kept project facets on the authoritative user-scoped project catalog. Task Table, Execution Tracking, and Personnel Load now expose identical project and owner choices in the authenticated browser fixture.
- Full Go tests passed. Frontend check passed with 0 errors and 79 existing warnings; production build, exact whitespace checks, Impeccable detection, and the Finesse P0 gate passed.
- Authenticated browser validation passed at 1440×900, 1024×900, 760×900, and 390×900 with internal table/inspector scrolling, no overlap or horizontal overflow, stable multi-select geometry, keyboard tab switching, and an empty clean-page console log.
- Validation used integrations-disabled isolated SQLite data. No schedule save or production Jira/database write was performed.
- **Status:** complete

# 2026-07-31 - Task Table Work Item convergence complete

- Rewired global demand/Bug search to Work Items and made Enter select the exact ID/title match or the first ranked result before navigating and scrolling to the highlighted row.
- Restored the status Task Table to the Work Item read model with correct project/owner facets while retaining Execution Tasks for execution tracking and personnel load.
- Removed inspector clamping/hidden-list behavior, added independent scrolling, and rebuilt task detail with the same shared `wide` Modal hierarchy used by Decision Dashboard.
- Authenticated fixture browser validation passed at 1280×720 and 390×844 with correct Work Item/Git boundaries, filters, long-content reachability, responsive modal geometry, zero document overflow, and no console errors.
- Frontend check, production build, targeted Impeccable/Finesse detection, static contracts, and diff hygiene pass. Validation was local and read-only.
- **Status:** complete

# 2026-07-31 - Task Table core-member candidate boundary complete

- Replaced Work Item-derived owner options with the shared core-member directory while preserving real owner facts in table rows.
- Aligned `/api/demands/options` with the central member rule: configured `sync_users` wins and custom-JQL assignees are fallback-only.
- Added backend regressions for primary/fallback behavior and a frontend source contract preventing future Work Item-derived candidate leakage.
- Isolated authenticated browser validation kept an external assignee visible in its row while exposing only the configured core member in the owner dropdown.
- Internal server tests, frontend check/build, Impeccable/Finesse gates, and diff hygiene pass; no external integration or business write was used.
- **Status:** complete

# 2026-07-31 - Shared delivery owner and project directory complete

- Corrected the three-person Task Table regression by separating RBAC authorization from delivery ownership. One authenticated `/api/delivery/directory` now returns configured Jira core members and the authoritative Jira/version-source project catalog.
- Migrated Decision Dashboard, Task Table, Execution Tracking, Demand/Schedule forms, Daily Jira reassignment, Version Plan, Deconstructor, and Project Config mapping to the shared directory. Project Preferences was confirmed to already use the same backend catalog.
- Added regressions proving RBAC-only users, visible local execution owners, task IDs, repositories, and commit evidence cannot pollute reusable owner/project candidates.
- Complete server/database tests, frontend check/build, Impeccable detection, Finesse P0 gate, and diff hygiene pass.
- Authenticated isolated browser validation confirmed matching 10-person and 4-project sets across the main delivery pages. Desktop and 390px Task Table checks showed an unclipped 10-option menu, zero document overflow, a 44px narrow trigger, and an error-free fresh task route.
- Fixed an additional empty-project-config `null` payload crash found during the browser audit; external integrations remained disabled and the isolated services/database were removed.
- **Status:** complete

# 2026-07-31 - FZ-2247 dual-repository realtime trajectory repair complete

- Canonicalized Jira issue identity across GitLab evidence so lowercase parser output and historical lowercase logs resolve to the existing `FZ-2247` Work Item instead of creating or hiding a Git-derived duplicate.
- Scanned every commit in a push batch, preserved the canonical Jira owner when Git evidence arrives, and kept both `task_executor` and `crane_manager` evidence under one trajectory even when only a non-final commit carries the Jira key.
- Added task-scoped SSE telemetry updates and open-panel background refresh with stale-request protection and no list flicker.
- Full Go tests, frontend check/build, detector gates, whitespace checks, and authenticated browser validation passed. The live panel advanced from PUSH 3 to PUSH 4 without manual refresh and rendered both repositories at desktop, tablet, and mobile widths without overflow.
- Production/runtime evidence still shows no `crane_manager` webhook delivery. No external GitLab mutation or deployment was performed; webhook installation/status remains the required environment follow-up.
- **Status:** local repair and validation complete

# 2026-08-01 - Governed data asset architecture

- Committed the complete pre-existing worktree as `ecd1aaf feat: consolidate delivery planning and admin workflows` before starting new architecture work.
- Loaded project continuity, coding, memory, delivery, domain-modeling, codebase-design, and file-planning rules.
- Started the backend-only architecture phase; no frontend or external-system write is in scope.
- Initial audit confirms the repository already has a strong Work Item audit stream but lacks a unified governed ledger, immutable report/analysis artifacts, hot/cold data separation, and bounded cross-source cursor queries.
- Fixed the module direction at six operations: append, bounded timeline, event detail, seal snapshot, snapshot detail, and latest scoped snapshot. SQLite remains the real local adapter and tests cross the same module interface.
- Added the domain vocabulary, accepted ADR, and architecture document covering fact layers, time semantics, hot/cold storage, cursor watermarks, idempotency, evidence snapshots, migration, and future partition readiness.
- Added additive database models for event metadata/payloads and snapshot metadata/payloads/evidence, explicit composite indexes, and SQLite update/delete triggers.
- Added the initial `internal/dataassets.Module` implementation with normalized append, full-fingerprint idempotency, gzip thresholding, hash-checked detail reads, stable cursor timelines, and immutable evidence snapshots.
- `gofmt` and `go test ./internal/dataassets ./internal/db -count=1` pass; module behavior tests are the next step.
- Added interface-level tests for replay/conflict, compression/integrity, stable late-event pagination, snapshot evidence/versioning, immutable raw-SQL guards, index plans, and a 12,000-row bounded metadata set.
- Added protected read-only event/snapshot endpoints under the new `data_asset:read` permission. Timeline responses expose only metadata; detail requests explicitly retrieve governed payloads.
- Integrated Work Item planning/reconcile audit events into the ledger in the same transaction and proved an injected asset failure rolls back the task mutation and both audit streams.
- Targeted module/database/delivery-planning tests and the new server handler tests pass.
- Initial 25,000-row benchmark measured roughly 1.07 ms/op for a 100-row page on the local Apple M4 Pro before changing the benchmark to a genuinely deep keyset cursor; the deep-cursor result remains to be recorded.
- Added cursor-batched historical WorkItemEvent backfill plus `cmd/data-assets-migrate`. Dry-run is read-only; apply mode requires a pre-migration backup and is idempotent on replay.
- Added 1,201-event backfill coverage, including dry-run, apply, replay, and resume-from-ID behavior.
- Final deep-keyset benchmark at approximately row 20,000 of 25,000 rows measured `1,897,392 ns/op` for a 100-row page on the local Apple M4 Pro; the query remains metadata-only and OFFSET-free.
- `go test ./cmd/data-assets-migrate ./internal/dataassets ./internal/db ./internal/deliveryplanning -count=1`, focused data-asset server tests, `go vet`, and `git diff --check` pass.
- Full `go test ./...` advanced through all other packages but the existing LLM and GitLab webhook suites require local `httptest` listeners; the sandbox denied those binds and both automatic escalation reviews timed out. This is recorded as an environment validation gap, not a passing result.
- No frontend, production database, external integration, destructive retention, or migration apply action was performed.
- **Status:** local governed data-asset foundation complete; deployment dry-run/apply remains explicit follow-up

# 2026-08-01 - Governed data asset review, dry-run and isolated commit

- Completed independent standards and specification review; all identified P1 correctness/safety findings were resolved before commit.
- Hardened immutable inserts, deterministic replay inspection, snapshot temporal/watermark validation, WAL-consistent backup, conflict-aware dry-run, malformed JSON evidence preservation, release metadata capture, explicit actor classification, and covering indexes.
- Targeted tests, focused server tests, vet, whitespace validation, query-plan checks, and an independently exported staged-index test run all passed.
- The deep keyset benchmark returned `1,979,583 ns/op`, `245316 B/op`, and `6568 allocs/op` for a 100-row page near row 20,000 of 25,000 rows.
- Read-only assessment of `well-ambient.db` reported 7 scanned, 7 would append, 0 conflicts; SHA-256 remained `fb4918f33eabc3435d847bed2faa5d69adb0e9bb1aae39e7be23cf69b0848849`.
- Created isolated architecture commit `bd2df59 feat: add governed data asset ledger`. Concurrent completion-flow, frontend, runtime-status, and database changes remain untouched and uncommitted.
- **Status:** reviewed, read-only assessed, and committed locally; production apply not run

# 2026-08-02 - Daily Jira automatic refresh after Jira sync

- Reproduced the stale-page path and ruled out response caching: Jira sync persisted fresh `TaskTelemetry`, but the worker emitted no update event and Daily Jira did not subscribe to the existing SSE-derived telemetry event.
- Added change-aware, post-batch telemetry broadcasts to Jira synchronization. An unchanged follow-up sync emits no duplicate event.
- Added a debounced Daily Jira event subscriber with in-flight coalescing and teardown cleanup, preserving current filters and selection.
- Added deterministic backend and frontend regressions for changed/unchanged syncs, burst events, in-flight events, and unsubscribe behavior.
- Targeted and related Go tests, `go vet`, frontend tests, `pnpm check`, `pnpm build`, Impeccable/Finesse gates, and diff hygiene passed.
- Isolated real-component browser validation confirmed automatic fact updates while retaining the selected issue at desktop, 760px, and 390px with zero horizontal overflow and no console errors.
- Existing localhost service, real Jira, and the project database were not modified during browser validation.
- **Status:** complete locally; not deployed

# 2026-08-02 - Daily Jira source activity time correction

- Reproduced the user-visible clustering with two existing Jira rows whose source `updated` timestamps differed but whose local sync timestamps were identical.
- Added `TaskTelemetry.SourceUpdatedAt` as a separate source-fact timestamp; local `LastUpdate` keeps its existing mutation/concurrency semantics.
- Jira ordinary and keep-alive synchronization now populate and advance the source timestamp only from valid Jira `fields.updated` values.
- Daily Jira projects `SourceUpdatedAt` as recent activity and falls back to legacy `LastUpdate` until the next synchronization backfills old rows.
- The exact red regression turned green; the related Jira/Daily Jira set, complete server package, database package, full Go suite, vet, and diff hygiene passed.
- No frontend files, real Jira, or the project database were modified for this correction.
- **Status:** complete locally; not deployed
# 2026-08-12 版本发布闭环与筛选胶囊工具条

- 已加载项目 UI 门禁、Impeccable layout、design-taste-frontend、finesse product/redesign/preflight 与 diagnosing-bugs；任务计划已记录共同方向、分歧、组件归属、响应式和验证范围。
- 已启动两份隔离布局评审；机械扫描完成且 detector 为零未解决项，视觉评审仍在收敛。
- 已建立发布闭环红灯：专用 `POST /api/releases/{id}/publish` 缺失时返回 404。
- 已实现领域发布服务、专用权限路由、状态与资产事件原子写、幂等重试，以及最强大脑 `delivery-cockpit.releases` 发布事实投影。
- 已关闭通用 PATCH planned -> released 的绕过路径，并要求本地版本创建从 planned 开始。
- 定向 Go 回归已由红转绿：`TestPublishReleasePersistsEvidenceAndMakesItVisibleToStrongestBrain`、发布前置条件、版本状态不重开和版本计划列表均通过。
- 下一步：完成视觉评审合并，实施版本页发布交互、筛选胶囊与最强大脑可见表面，再跑全量静态/浏览器验证。

## 2026-08-12 版本发布闭环最终进展

- 完成领域发布服务、专用路由、原子资产证据、幂等重试、状态/范围锁定和最强大脑发布事实投影。
- 完成版本页内联发布确认、发布后只读、即时刷新，以及最强大脑最近发布事实条。
- 完成“现有版本与 Jira 事项”整行胶囊工具条和桌面/平板/移动端响应式布局；受影响的容器、卡片、控件、portal 浮层和 modal 均无阴影，焦点仍使用清晰 outline。
- `go test ./... -count=1`、`go vet ./...`、`pnpm check`、`pnpm build`、Impeccable、Finesse 与 `git diff --check` 通过。
- 登录态隔离浏览器完成发布前确认、实际发布、发布后只读、最强大脑即时可见、`1440px`/`900px`/`390px` 响应式、零阴影、无横向溢出和空控制台验证。
- 未连接真实 Jira、未修改项目数据库、未部署或重启生产服务。
- **Status:** complete locally; not deployed
# 2026-08-12 - Solution asset implementation started

- Loaded project cold-start rules, codebase-design, planning-with-files, and the mandatory Impeccable/design-taste-frontend/finesse-ui gate.
- Recorded a three-way product-UI direction: preserve the Phase 41 demand-detail drawer, use one authoritative Markdown document, show Agent output as a candidate, and require human Diff/apply before changing a draft.
- Audited the existing versioned demand specs, Markdown workbench, Jira comment adapters, AI streaming path, permissions, and gzip data-asset codec.
- Protected the dirty worktree by starting an independent `internal/solutions` module and limiting future shared-file edits to additive migration/routes/permission wiring and the existing demand-detail mount point.
- **Current:** implementing additive solution schema, compression, CAS, immutable revisions, source references, polish jobs, prompt versions, and Jira outbox.

## 2026-08-12 - Personnel performance brain implementation started

- Loaded the planning-with-files, codebase-design, and domain-modeling instructions for this backend-only change.
- Fixed the module boundary: silent background runner plus one run-once seam; no frontend trigger and no reuse of project-health scoring.
- Chosen persistence semantics: append-only run/snapshot/audit records, fixed input watermark and formula version, with configurable transactional retention defaulting to 90 days.
- Protected the dirty worktree by avoiding the existing frontend and planning additive backend files plus the smallest necessary config, migration, and server-lifecycle wiring.
- Implemented the domain vocabulary, additive schema, scorer evidence contract, runner lifecycle, and temporary-database regressions.
- First full Go run found an SQLite polling lock in the new runner test and sandbox-denied local listeners in existing LLM/server tests; logged both in `.learnings/ERRORS.md` and began targeted correction.
- Repeated-package verification exposed shared in-memory SQLite state leaking across `go test -count`; assigned every test invocation its own database identity in addition to single-connection serialization.
- Stabilized the runner regression and passed it ten consecutive times; race detection also passed (with a non-failing macOS linker warning).
- Passed the complete Go suite with local-listener permission, plus `go vet ./...`, gofmt and `git diff --check`.
- Did not open, migrate, or write the existing `well-ambient.db`; no frontend, deployment, or service restart was performed.
- **Status:** complete locally; existing deployment configs must opt in with `performance_brain.enabled: true`.

## 2026-08-12 - Personnel performance next stage started

- Activated planning-with-files and codebase-design for the deeper evidence and runner work.
- Activated the mandatory Impeccable, design-taste-frontend, and finesse-ui review gate before any frontend edit.
- Added a requirement-complete plan covering formal evidence, runner hardening, super-admin read authorization, calculation explanation, responsive browser validation, and final completion audit.
- **Current:** reading the full UI skill instructions, then auditing authoritative backend and frontend state before selecting the implementation seam.

## 2026-08-12 - Personnel performance evidence and runner core

- Added an append-only, revisioned formal evidence ledger with explicit observe/void actions, source identity, payload hash and actor audit.
- Wired C04/C05/C06/C10 to formal evidence only while preserving the fix-contribution versus defect-responsibility boundary.
- Extended retention to evidence revisions and added bounded SQLite busy retries plus in-memory runner status for read-only diagnostics.
- Added a module-owned calculation explanation read model; opening or refreshing it cannot trigger a calculation.
- Fixed the isolated performance test migration after the first focused run exposed the newly added table omission.
- **Current:** verify the backend core, then add super-admin-only handlers and regressions.

## 2026-08-12 - Personnel performance backend next phase verified

- Added super-admin-only read and evidence-ingest endpoints; both authorization paths are server-enforced and covered by member-versus-superadmin tests.
- Focused regressions pass for append-only evidence revisions and voids, complete ten-metric formal scoring, evidence retention, bounded SQLite contention retry, read-only explanation behavior, and evidence API audit without a calculation trigger.
- The existing project database was not opened or migrated; all verification used package memory databases and the shared isolated server-test database.
- A first plan checkpoint patch used wording from an earlier draft and missed the current task-plan context; no product file changed, and this checkpoint uses the actual task section.
- **Current:** implement the reviewed Phase 41 calculation explanation subview and role-filtered navigation.

## 2026-08-12 - Personnel performance calculation guide implemented

- Added the superadmin-only KPI child navigation and guarded the client handler in addition to the backend authorization boundary.
- Implemented a read-only Phase 41 page with stable background refresh, complete loading/error/empty states, runtime facts, formulas, ten-metric table, evidence and retention inspector, factor matrix, and recent persisted snapshots.
- `pnpm check` passes with zero errors and the same 80 pre-existing warnings in unrelated files.
- Impeccable detection found one side-accent anti-pattern in the first pass; replaced it with a full hairline boundary and queued a clean rerun.
- **Current:** complete design detection and backend/frontend build validation before authenticated browser checks.

## 2026-08-12 - Personnel performance authenticated browser validation

- Isolated superadmin session confirmed the KPI child menu, calculation explanation hierarchy, all ten metric rows, all six factor groups, read-only notice, background status, evidence ledger, retention policy, and empty snapshot state.
- After isolated evidence and task fixtures, startup and scheduled runs produced append-only formal Alice snapshots at 97.70 / A, 100% evidence coverage, 7 effective samples, and a 1 / 1 demand-to-Bug count.
- Clicking “刷新已保存结果” left the run count at 6 before and after, proving the page reads persisted state without triggering calculation.
- During a simulated API outage, the page retained two previously loaded snapshots and showed a recovery warning; after restart, the warning cleared and the next persisted snapshot appeared atomically.
- At 1440x900, 900x900, and 390x844, document overflow was zero; the main/inspector layout changed from two columns to one, metric tables scrolled internally, and the mobile refresh control measured 44px.
- Measured text contrast was 18.06:1 for the main heading and 11.59:1 for the responsibility note; a fresh successful page session had no console warnings or errors.
- Non-superadmin role hid the calculation submenu. Runtime membership removal caused a 403 refresh to remove the snapshot table immediately, and the App role guard returned the user to a non-sensitive view after role refresh.
- Impeccable detection is clean (`[]`); focused backend tests, `pnpm check`, and production build pass.
- Full `go test ./... -count=1`, `go vet ./...`, performance race detection, `pnpm check`, `pnpm build`, Impeccable detection, and diff hygiene all passed. The frontend checks retain 80 pre-existing unrelated warnings and the existing bundle-size advisory, with zero errors.
- Final requirement audit confirmed that fix contribution is not used as defect responsibility, reads never call `RunOnce`, evidence revisions and score/audit history follow the configured retention window, and both APIs remain server-authorized for global superadmins.
- Added the formal-performance-evidence term to the domain context and completed the implementation plan. The real Jira/CI producers and deployment config remain an environment integration step; no production service, real Jira, or existing project database was touched.
- **Status:** complete locally; not deployed
# 2026-08-12 排期治理轨迹卡片高度对齐

- 已按 `coding.complex` 启动新任务，保护当前大量未提交用户改动，尚未编辑任何前端文件。
- 已加载项目路由、Impeccable、diagnosing-bugs、planning-with-files、design-taste-frontend、finesse-ui、Chrome 与 well-ambient 管理台验证约束。
- Impeccable 识别为 product register 的 layout 修复；已按其要求隔离发起布局判断与机械预扫描，两项均为只读。
- 已把截图症状转成可证伪验收：宽屏左右外框 top/bottom 对齐，右侧内容主体独立滚动，窄屏堆叠恢复自然高度且无文档级横向溢出。
- 已连接本地应用的现有浏览器控制面；接下来优先认领现有登录态标签页，不启动重复服务或重新登录。
- Chrome 登录态已复用并命中 HR-4202 代码轨迹状态；浏览器复现不需要任何点击或写入，后续只做视口覆盖与只读矩形/滚动测量。
- 已在用户截图同尺寸 2382×1100 获得红灯：右检查器计算为 `align-self:start`，底边比工作台主网格少约 185.69px；诊断继续锁定真实左卡矩形和最终 CSS cascade。
- 已停止控制被用户切换的原 Chrome 标签页，创建同一认证会话下的独占验证页并成功进入排期治理；后续几何不会再与用户当前操作争用。
- 两份隔离审查均已返回：视觉评估与机械 cascade 扫描独立指向 `DemandKanban.svelte` 文件末尾的 `align-items:start`、`align-self:start; height:auto` 覆盖，不建议修改 `CommitTelemetryPanel`。
- 已在独占登录页连续两次运行几何断言，稳定获得 123.4636px 底边/高度差红灯，同时确认左右 top 对齐和现有内层滚动归属。
- 已读取最终 CSS cascade 与 DOM 结构；三方会审的共享方向、分歧、组件归属、宽屏/堆叠响应式契约已在计划中记录，满足实现前门禁。
- 已新增 `web/tests/schedule-inspector-height-contract.test.ts`，先观察宽屏共享行断言失败，再把 `DemandKanban.svelte` 最终契约恢复为桌面 stretch/100%，并为 <=1280px 明确保留 start/auto。
- 目标测试 3/3 通过；宽屏 HR-4202 代码轨迹运行时连续两次几何断言全绿，原 123.46px 底边差变为 0，滚动所有者未改变。
- 并行静态门禁全部通过：6 项前端回归、Svelte/TypeScript、构建、Impeccable、Finesse P0 与 diff hygiene。
- 已保存并视觉复核宽屏结果 `outputs/ui-validation/schedule-inspector-height-wide.png`；卡片底边对齐，轨迹条目仍按内容自然排列。
- 宽屏排期设置页签也完成 0px 几何对齐，表单 body 保持独立滚动。
- 1280×900 已确认单列堆叠和 inspector 自然高度生效，无水平溢出；接下来补测 workspace 纵向滚动所有者并覆盖 760/390。
- 1280 的滚动归属在 `.demand-dashboard`，760 则切换为 document 自然滚动；两者均无水平溢出，桌面等高没有泄漏到堆叠态。
- 760 的代码轨迹与 390×844 的排期设置/代码轨迹均已完成登录态验证：窄屏由 document 自然滚动，左右卡宽一致，两个页签内容完整且无水平溢出。
- 已保存并人工复核 `schedule-inspector-height-390.png` 与 `schedule-inspector-height-390-trace.png`；最终浏览器控制台无 warning/error，视口已恢复，验证标签页已释放。
- **Status:** complete locally; not deployed
# 2026-08-12 代码轨迹完整可滚动浏览

- 已加载 diagnosing-bugs、planning-with-files 及项目强制 Impeccable/design-taste-frontend/finesse-ui 门禁。
- 已完成三方实现前评审：完整轨迹留在当前页签，宽屏单一正文滚动，窄屏自然流；不改外框等高、数据接口或轨迹业务语义。
- **Current:** 建立能直接捕获固定条数切片和隐藏提示的红灯回归。
- 已新增 `web/tests/schedule-telemetry-completeness-contract.test.ts`；修改产品代码前运行结果为 0/2，准确复现用户指出的隐藏提示和固定四条切片。
- 已向用户列出四项可证伪假设；当前 Phase 2 核对接口是否还有第二层限制，并检查正文 line-clamp 与滚动归属。
- 已确认后端 `/api/tasks/commits` 无条数限制并返回全部倒序记录；宽屏 `.schedule-telemetry-inline` 已是唯一滚动区，窄屏已有自然流契约。
- Phase 2 完成；最小产品改动限定为 `CommitTelemetryPanel.svelte` 的完整列表渲染、提示删除与 inline 正文解除裁切。
- 已完成最小产品修复；`schedule-telemetry-completeness-contract.test.ts` 由 0/3 转为 3/3。
- **Current:** 运行 Svelte/TypeScript、生产构建、设计检测与 diff hygiene，然后进入登录态浏览器完整条数/滚动验证。
- 组合回归首次运行 8/9；失败来自既有高度测试对 selector 顺序的脆弱假设，当前源码新增合法的 `.schedule-solution-inline` 同级页签。已调整为行为归属断言，未修改产品滚动规则。
# 2026-08-12 DG-394 静默方案润色失效

- 已加载 diagnosing-bugs 与 planning-with-files，建立逐跳证据链和阶段计划。
- 已确认当前任务优先诊断后台链路；在证据表明前端返回错误前不修改 UI。
- 下一步：执行 DG-394 数据库/API 红灯，收集任务状态与最近后台日志。
- Phase 1 完成：DG-394 资产与 current 来源存在，但来源不 eligible，且任务/候选均不存在；红灯命令退出 1。
- 已向用户给出 4 个可证伪假设并按现有证据排序；进入真实正文、判定器和自动入队边界检查。

## 2026-08-13 人员绩效 Core Member、配置开关与成员详情

- 完成 core-member fail-closed 聚合边界、用户目录别名归并、在线 `Reconfigure` 启停、受治理配置面板、只读快照详情接口和共享 Modal 多维呈现。
- 登录态首轮验证发现运行中的旧后端仍返回不含 ID 的历史全员快照；已把说明列表也收紧为当前 core member，旧数据继续保留作审计但不展示。
- 新增跨 100 条历史分页边界回归并通过；绩效 Go 包与配置/Modal 前端契约均已通过。
- 最终本地服务启动轮次完成；配置入口、14 名 core member、详情多维数据、桌面/移动几何、44px 关闭控件、Escape 与焦点恢复均已在登录态浏览器通过。
- 最终 Go 绩效/服务端回归、生产构建、前端 6 项契约、Impeccable 检测和目标差异卫生均通过；构建输出仅保留其他现有页面的警告。
- **Status:** complete locally; local development service running, not deployed

## 2026-08-13 人员绩效 Jira 历史证据为零

- 已启用 diagnosing-bugs 与 planning-with-files，建立当前真实数据库的只读红灯。
- 红灯稳定输出 `snapshots_with_jira_items_but_zero_available_metrics=14`，退出码 1。
- 已完成数据链最小化：历史 Jira 事项已归集，但完成/计划/估算与执行验收证据未入库；活跃 JQL 又排除了 done 历史。
- 已向用户给出 5 个排序假设并完成代码/数据对照；主因确认，进入先测试后修复阶段。
- 三个目标回归均已先红：评分断言得到 nil，历史 JQL/同步函数尚不存在。
- 已实现历史完成 JQL、Jira 完成/计划/估算字段适配和 C01 Jira resolution 降级证据；三个红灯全部转绿。
- 完整相关 Go 包测试与 vet 通过。
- 已补充重新打开事项不得复用旧完成证据及历史同步幂等回归，均先红后绿；相关三个 Go 包全量测试与 vet 通过。
- 本地新版服务已启动，真实 Jira 历史同步返回 234 条已解决事项并在 13:14:06 追加 schedule 快照。
- 原始数据库红灯由 14 降为 0；最新 14 名 core member 均有非零试算分，C01 共含 92 条 Jira resolution 引用。
- 登录态浏览器已确认列表非零、梁志远详情可追溯到 12 条历史 Jira 完成引用和逐事项计算系数；正式分仍受 80% 覆盖门槛保护。
- **Status:** complete locally; updated development service running on port 8080

## 2026-08-13 按判定表 v4.0 重实现并重算

- 已加载 spreadsheet、diagnosing-bugs、planning-with-files、自我改进及项目强制三方 UI 技能；修正了 design-taste-frontend 的实际路径后重新完整读取。
- 已锁定判定表中的权重、阈值、逐指标样本、覆盖率、暴露期、等级和系数；当前正在把这些规则固化为测试与单一后端规则模型。
- 已确认首个必须消除的红灯：只查询 Jira Done 会让 C01 缺少到期未完成事项分母，单个完成项因此得到 100 试算分。
- 已完成项目冷启动升级、领域词汇、Phase 41 设计契约和三方 product-register 参考加载；UI 门禁结论已写入计划，前端编辑门禁现已打开。
- 已定位并加载工作区内置 `@oai/artifact-tool` 2.8.43，准备对源工作簿做当前轮次的精确 range/formula 复核。
- v4.0 规则、系数和 C01 到期分母红灯已建立并转绿；单个 Jira Done 现在保留来源但不能越过 C01 最低 3 样本形成 100 试算分。
- 第一轮全包回归准确暴露 3 个旧契约：旧测试期待单样本可用、全局样本替代逐指标样本、旧 Bug 系数。已把正式评分夹具扩为满足每个 v4.0 指标最低样本与 30 天暴露期的真实结构。
- 已加入发布方式、回退影响、责任系数和趋势/信任风险/杠杆原因码的追加式证据字段；正式评分夹具已覆盖 13 条事项/发布系数明细。目标测试仅剩夹具计数同步，服务端 `httptest` 仍需按既有学习使用本机回环权限运行。
- **Current:** 完成项目/表格上下文加载，建立 v4.0 红灯测试和历史到期事项查询契约。
# 2026-08-14 scoring panel alignment polish

- Restored project cold-start rules and classified the task as complex preserve-mode product UI refinement.
- Activated planning-with-files and the mandatory UI review gate; target component discovery and browser redline are next.

# 2026-08-13 scorecard v4 closeout

- Fixed the SettingsPanel performance configuration shape and replaced its legacy v1 fallback values.
- Locked the runtime scoring contract to workbook v4 defaults so an older database config cannot silently change newly published score semantics.
- Cleaned the new evidence-label mapping before the next Svelte validation pass.
- Verified the focused frontend governance contract (3/3) with Node's TypeScript stripping mode.
- Added audited base/adjustment de-duplication with a regression fixture.
- Full Go validation found one legacy rollback API fixture without the newly mandatory v4 release method; the fixture now carries the explicit audited factor and strict validation is retained.
- Restarted the root service with `go run cmd/server/main.go`; it loaded database config version 21, synchronized 246 resolved-or-due Jira issues, and persisted a completed v4 startup run for exactly 14 core members.
- Queried the persisted v4 snapshots and audits: all formal scores are N/A, trial scores are 20-45 where calculable, C01 includes both due and resolved references, and the full run/snapshot/retention audit set exists.
- Authenticated desktop, tablet, and mobile validation passed: the list shows formal `N/A`, the member dialog separates trial score from formal score, Jira references include both `resolved` and `due`, focus returns after Escape, and document overflow remains zero.
- Source inspection confirmed the apparent duplicate “等级” entry was overlapping `sed` output rather than duplicated UI markup; no source change was needed.
- Impeccable exact-target detection returned `[]`; Finesse reported no P0 finding. The single target P2 is the intentional white glyph on the blue information marker, retained for contrast.

# 2026-08-13 scorecard v5 closeout

- Implemented the complete v5.0 scoring contract: three dimensions, eight executable metrics, fixed-weight aggregation, shadow/formal publication modes, configurable dimension/source switches, and core-member-only calculation.
- Added governed Jira changelog ingestion and immutable source-event storage; synchronized 4,774 historical events for 860 work items and recalculated a 14-member v5.0 startup run without duplicate members or 100-point snapshots.
- Added demand/project coefficient evidence, responsibility-separated bug attribution and fix-cycle segments, recurring-defect evidence, Git SHA de-duplication, stable fingerprints, and raw numerator/denominator/unit projections.
- Updated the calculation guide, member detail dialog, settings configuration, example YAML, runtime explanation, audit output, and the v5.0 executable workbook.
- Fixed the solution workspace loaded-content branch and added a regression contract; authenticated browser validation confirmed the solution body on wide and 390px layouts.
- Validation passed: `go test ./... -count=1`, `go vet ./...`, 24/24 frontend contracts, `pnpm check`, `pnpm build`, Impeccable target detection, browser console/interaction checks, workbook formula scan, and `git diff --check`.
- Cleaned temporary validation sources and stopped validation servers. The repository remains deliberately dirty with the user's broader in-progress work preserved.

# 2026-08-13 solution preview and edit dialog

- Loaded the project UI gate and completed the Impeccable, design-taste-frontend, and finesse-ui preserve-mode review.
- Recorded the agreed hierarchy, ownership, responsive contract, dirty-state safety, validation scope, and skill disagreements in `task_plan.md` before frontend edits.
- Audited `SolutionWorkspace.svelte`, `MarkdownWorkbench.svelte`, the shared `Modal.svelte`, Phase 41 tokens, solution CAS APIs, existing editor reconciliation, and prior solution-governance memory.
- Added focused static contracts for default preview, a single edit-dialog entry, dialog-contained save/publish, removed visible version copy, dirty-close protection, and retained CAS fields.
- Focused contract run is intentionally red (5 pass, 2 fail) against the old implementation; implementation is the next phase.
- Reworked `SolutionWorkspace.svelte` so the persisted Markdown renders in preview mode while a single “编辑方案” action opens the shared wide Modal with a live editor.
- Moved save and publish into the Modal, added save-before-publish orchestration, automatic governed draft creation for published content, and inline dirty-close confirmation without a native confirm dialog.
- Removed visible revision numbering/history and changed remote-update copy while retaining the backend revision, working revision ID, CAS request fields, immutable history, and reconciliation behavior.
- First green attempt exposed two contract-only mismatches: an obsolete assertion forbade `availableModes`, and the new dialog assertion expected static button text despite loading-state expressions. Removed the unnecessary mode props and tightened the contract around the wired handlers and stable labels.
- Focused solution entry contracts now pass 7/7.
- `pnpm check` initially failed on Svelte named-slot placement; the footer is now a direct Modal child and the condition lives inside the slot. The error is resolved and recorded in `.learnings/ERRORS.md`.
- Full frontend contracts pass 26/26. `pnpm check` passes with zero errors and 81 warnings in four unrelated existing files; the edited `SolutionWorkspace.svelte` contributes no warning.
- Production build passes; only the existing large-chunk advisory remains.
- Impeccable target detection returned `[]`. Finesse found two P2 pure-white fallbacks in the target button styles; replaced them with the existing tinted Phase 41 surface/ink fallbacks before the final detector pass.
- Built an isolated validation server from a copied database with Jira, AI, and performance calculation disabled; reused the existing Go module cache after the first empty-cache build failed under restricted networking.
- Authenticated browser validation passed for DG-352 on desktop and 390×844: default preview, one edit entry, shared edit Modal, dialog-contained save/publish, mobile footer geometry, and no visible version copy.
- Dirty-close validation passed: editing enabled Save, the close action presented continue/discard choices, discard destroyed the Modal, and the test text did not persist. Browser console contained no warnings or errors.
- Final validation passed: 26/26 frontend contracts, `pnpm check` with zero errors, production build, final Impeccable/Finesse detectors, and diff hygiene. Temporary browser tab, backend process, build artifacts, and copied database were cleaned up.
- **Status:** complete locally; not deployed

# 2026-08-14 release lifecycle and inspector layout

- Completed the mandatory Impeccable, design-taste-frontend, finesse-ui, and UI design-system review before frontend edits; recorded hierarchy, ownership, responsive behavior, destructive-action semantics, and validation scope in `task_plan.md`.
- Added red API regressions that initially reproduced missing discard/archive routes as 404 and missing release DELETE as 405.
- Implemented audited archive, discard, and protected soft-delete lifecycle actions, dedicated permission routes, discarded status filtering, idempotent transition replay, and server-projected lifecycle capabilities.
- Replaced the split project Select/action rows with one aligned control grid; added lifecycle management and shared inline confirmation states with reason, exact-name delete confirmation, loading protection, Toast feedback, and focus restoration.
- Corrected the first 420px container-query attempt after authenticated desktop geometry proved it still stacked at a 397px inspector; the final breakpoint is viewport-based at 430px.
- Validation passed: focused lifecycle tests, full `internal/server` tests, `internal/deliveryplanning` tests, `pnpm --dir web check` with zero errors, production build, both UI detectors, `git diff --check`, and authenticated 1440/760/390 browser geometry/interaction checks with zero console errors.
- Existing repository-wide frontend warnings and Vite large-chunk advisory remain outside the target; no remote deployment or production mutation was performed.
- **Status:** complete locally; local running backend was not restarted by this task.
# Session: 2026-08-14 - 度量洞察卡片流与空白修复

- **Status:** complete locally; not deployed.
- 完成 Impeccable、design-taste-frontend 与 finesse-ui 三方 preserve-mode 评审；根因是同一 Grid 行被右侧长 inspector 撑高，而系数卡片位于 Grid 之后，并非 margin 或卡片固定高度。
- 已将“需求与 Bug 计算系数”移入左侧 `.guide-main`，保持指标区到系数区 16px 连续内容节奏；系数网格按容器自动分列，980px 以下 inspector 自然下置。
- 浏览器复核发现旧服务快照缺少 v6 `delivery_score/quality_score` 时会调用 `undefined.toFixed`；建立回归后将字段声明为可选并在显示层回退为 `N/A`，没有改动后台评分或数据。
- 定向契约 6/6、`pnpm check` 与生产构建通过；Impeccable layout=`[]`，Finesse 目标文件无 findings。登录态宽/中/窄断点无页面级横向溢出，新 Chrome 页签 warning/error 为 0。

# Session: 2026-08-15 - Jira 完成状态与评论入站同步

- **Status:** Phase 1 in progress.
- 已加载项目 cold-start 规则、复杂编码预设、tool routing、`diagnosing-bugs` 与 `planning-with-files`；确认必须先建症状级反馈环。
- 已检查工作树并记录保护边界：保留所有既有修改；真实 Jira 与主数据库只读，测试使用隔离依赖。
- 下一步：读取 CONTEXT/相关 ADR、Jira 同步代码、定向测试、运行配置与 DL-4309 当前落库状态，构造红灯命令。
- 已通过现有登录态只读核验 DL-4309：Jira 为 Done，评论 533697 在约 22:41 创建；未点击编辑、工作流、评论或删除入口。
- 已只读核对主数据库：状态与评论最终在约 22:45 落库，说明需要修的是同步延迟/运行调度与变更后刷新通知，而非单条数据回填。
- 错误：PATH 中无 `curl`，首次本地 HTTP 探测未执行；下一步使用 `/usr/bin/curl` 并检查服务启动时间/相关页面订阅。
- 只读进程检查确认服务从 20:19 运行至本轮检查中途退出；没有可读日志句柄，未执行重启或进程控制。
- 代码检索确认刷新消费者不统一：Daily Jira/活动面板可事件刷新，TaskKanban 仍依赖 60 秒轮询，Demand/Decision 依赖 15 秒轮询；下一步在后端建立评论变更广播红灯，并验证 Done 退出主 JQL 后的 keep-alive 契约。
- 新增 `TestJiraSyncBroadcastsWhenOnlyJiraCommentChanges`，使用 DL-4309/533697 形状的 httptest + 隔离 DB；确认评论落库后没有 telemetry 广播，定向命令稳定红。
- 首次夹具因 `CompletedAt` 时间种错而使任务字段变化、错误通过；已修正为真实 resolution 时间并记录，避免把旁路字段广播误当评论广播。
- 安全读取真实配置确认：custom JQL 明确排除 Done，且范围横跨约 40 项目/15 人；未打印凭据。当前无 Jira 入站同步水位表，为旧 Done 更新的可靠补采和可观察性留下缺口。
- 数据量量化：1,012 个 Jira 事项、497 活跃、431 个 14 天内 Done、84 个旧 Done；当前最坏周期约 1,425 次串行评论请求。采用深模块 seam，把去重、增量补采、评论水位和每事项通知隐藏在单次 reconcile 内。
- 新增并运行三条定向红灯：评论广播缺失、稳定事项两轮评论请求 4 次、旧 Done 重开/评论完全漏采；Phase 2 完成，进入深模块实现。
- 实现加法 `JiraInboundSyncState`/`JiraIssueSyncState`、评论 current 投影、主/补偿查询合并去重、updated 水位重叠、按最新更新时间处理和逐事项广播；三条定向回归已转绿。
- 全部 Jira server 回归与 `internal/telemetry` 套件通过。实现中一次花括号遗漏被 gofmt 精确捕获并已修复。
- 后端补充回归现已覆盖失败不推进水位、远端评论删除、最新事项不被无关慢事项阻塞；六条本次症状级测试均通过。
- 已进入强制前端三方评审：完成 Impeccable 项目设计上下文和 finesse-ui product register 主规则读取，正在完成 design-taste 与产品/改版/交付检查参考后，再记录共同实现边界并编辑 Svelte。
- 自检：一次计划文件补丁因跨文件复用错误锚点被拒绝且未产生修改；后续先读取每个目标文件的独立尾部，再构造追加补丁。
- design-taste-frontend 全文已完成读取；已确认它对本任务只提供 preserve-mode 与 no-CLS/no-jitter 检查，不是产品看板的视觉系统。下一步完成 finesse 产品/改版/反廉价/交付参考，再落计划门禁结论。
- finesse-ui 的 product、redesign、anti-cheap、preflight 参考已完整读取；三方门禁结论已写入 `task_plan.md`，前端编辑门禁开放。
- 路径自检：首次检索误把实际位于 `web/tests/` 的 `daily-jira-refresh.test.ts` 写成 `web/src/lib/`，`rg` 报缺失但其余结果仍可读；已改为先用 `rg --files` 解析真实路径。
- 已逐段检查 Task/Demand/Decision/Daily Jira 的加载、轮询和 cleanup；确认使用最小加法接入，避免覆盖这些文件中现有的大批用户修改。
- 新增通用 `telemetry-refresh` 红测并确认正确失败于模块不存在；实现后 5 条通用契约转绿，既有 Daily Jira 2 条契约继续通过。
- 静态检查首次发现 Daily 兼容层使用 `.ts` 扩展不符合当前 tsconfig；已保留用户原有兼容模块不变，让四个生产页面直接接入通用模块，避免修改全局 TypeScript 解析策略。
- 四个页面现已直接接入共享 `telemetry-refresh`，轮询兜底保留；7/7 刷新契约通过，`pnpm --dir web check` 为 0 error，86 条 warning 均来自既有文件区域。
- `/api/status` 的 Jira 入站状态红测已从缺字段转绿：现在区分 disabled/pending/syncing/healthy/error/stale/unavailable，暴露安全的时间和计数，但不泄漏具体错误文本。
- 补偿 JQL 新增边界回归并通过：项目范围、主查询去重、50-key 有界分批、5 分钟 inclusive overlap 和未来时钟水位钳制均已锁定。
- 继续审计通知链发现 SSE 32 条缓冲满时会静默丢 task ID；下一步建立 burst 红灯并用每客户端去重待发集消除丢事件窗口。
- SSE burst 红灯已从缺少保留机制转绿：每个客户端现在把缓冲外 task ID 放入去重待发集，写出一个缓冲事件后按 key 稳定顺序排空，断开时一并清理；不再用扩大固定 buffer 掩盖丢失。
- 空 Jira 范围红灯已转绿：启用但无同步范围时保存可见失败，不再留下永久 syncing 幻象，也不推进成功水位。
- 完整 `internal/server` 与 `internal/telemetry` 回归通过；全部 45 条前端契约通过，其中本次新增 9 条刷新/消费契约。
- 仓库全量 `go test ./...` 与 `go vet ./...` 通过；生产前端构建通过，仅保留已有 Svelte warning 与大 chunk advisory。
- UI 检测：Impeccable 对四个目标组件返回 `[]`；Finesse 仅命中 Task/Demand 旧 CSS 的纯白 P2，本次没有修改对应样式或可见文案。
- 隔离浏览器服务首次启动发现复制库中的版本化配置会覆盖临时 YAML，进程在端口权限失败前短暂进入 worker 启动；未获得端口审批且未继续绕过。仅临时副本已清除版本配置与待执行 outbox，主库/真实 Jira 未修改；浏览器验证改用无外部连接的前端夹具或已有登录态。
- 无外连 Service Worker 夹具完成了真实构建产物的登录态端到端验证：DL-4309 从“进行中”切换为“已完成”后约 635ms 更新列表、指标与详情，`/api/work-items` 只重取 1 次，浏览器 console 为空。
- 完成态与进行中态下任务表矩形均为 `x=293, y=500.28125, width=1168, height=104.96875`，证明事件刷新没有引发布局抖动；1920×813 截图已人工检查层级、对齐、溢出和状态一致性。
- Demand、Decision 与 Daily Jira 的隔离浏览器事件注入分别触发既有投影接口；通用 5 条调度器契约覆盖事件合并、in-flight 补刷、无关任务过滤、隐藏页恢复和失败后重试，4 个消费者契约确认保留轮询兜底。
- 最终回归：`GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1`、`GOCACHE=/tmp/well-ambient-gocache go vet ./...`、45/45 前端契约、`pnpm --dir web check`、生产构建、`git diff --check` 全部通过；check 仍有 86 条既有 warning，构建仍有既有大 chunk 提示。
- 临时预览、Service Worker、认证 localStorage、复制数据库和调试文件均已清理；正式 `web/dist` 已重新构建。真实 Jira 与主数据库全程只读，现有服务未重启，修复需按正常流程部署/重启后才生效。

# Session: 2026-08-15 - 证据链筛选输入空白回归

- **Status:** Phase 1 in progress.
- 已加载 diagnosing-bugs、planning-with-files 与强制 UI 三方门禁资料；当前只建立症状反馈环，尚未编辑前端源码。
- 已确认现有后端 8080、前端 5173 均在运行，下一步复用登录态打开证据链页面并实测单字符输入造成的空白。
- Browser 当前没有遗留页签，打开 5173 后显示登录页；下一步解析本地测试账号来源并完成只读登录复现。
- 已修正登录检索命令且确认无公开默认凭据；正在核对 `/api/login` 测试契约，目标仍是获得可控登录态而不是触碰真实凭据。
- 已确认开发登录存在数据库写入副作用，主动放弃在主服务上登录；下一步构造隔离数据库后端并在其上复现。
- 进程命令只读检查因权限被拒绝；该信息不是复现必需条件，已转向配置模板与隔离数据库路径，不申请扩大权限。
- 已确定隔离启动方法：编译后从 `/tmp` 运行、复制主库、清除副本版本配置、只使用全外联禁用 YAML，并复用构建产物验证真实页面。
- 已确认需清空副本 `config_versions`，且后端不是可直接依赖的静态站点；下一步核对 Vite 代理变量后启动独立前后端端口。
- 隔离后端 18080 与前端 5174 已启动；两次沙箱端口 EPERM 均按规则改为受控授权启动，下一步执行 dev 登录并进入证据链页面红灯复现。
- Browser 已在 5174 完成本地临时登录并加载管理台；下一步点击证据链，定位筛选输入与空白的可自动判定信号。
- 已进入证据链并完成第一次真实输入实验：`N` 只做正常本地筛选，没有空白。下一步逐键采样零结果/连续输入和控制台，最小化触发条件。
- 红灯已复现：`N` 先本地筛出 2 项，随后延迟刷新把整个证据工作台替换为空数据分支，连筛选输入框一起卸载。Phase 1 完成，进入根因假设与调用缝核对。
- 首轮源码证据确认顶层 empty 分支所有权错误：它基于过滤后数组而非源数据；下一步建立目标组件回归，先锁住“零结果仍保留筛选/表格”，并精确复测是否还有异步覆盖。
- 已确认无现成证据链筛选测试；准备按仓库 `node:test` 约定新增定向红灯。生产前端编辑前仍需先把本任务的三方 UI 评审结论写入计划。
- 保证零匹配词在 80ms 内稳定复现输入框卸载，已排除需要异步请求才能触发的假设；三方 UI 评审完成并记录，前端编辑门禁开放，下一步先加红测。
- 新增 `project-health-filter-contract.test.ts`；首次裸 `node --test` 被 Node 22 的 `.ts` 扩展加载限制阻断，断言尚未运行，已查到仓库标准参数 `--experimental-strip-types`。
- 定向回归按正确运行器参数先 1/3 红、修复后 3/3 绿；真实浏览器保证零匹配已从“筛选框卸载、0 行”变为“筛选框与表格保持、上下文空行可恢复”。
- 连续中英文、退格、清空与健康筛选零命中都未再空白；清空后的全量计数受当前健康状态筛选影响，正在读取实际激活状态后补做独立恢复断言。
- 真实键盘清空已验证恢复全量 36/36、37 行且焦点保留；下一步跑宽/窄断点、console、检查/构建和 UI 检测器。
- 已确认浏览器封装不枚举 viewport/console 方法，避免盲猜私有接口；继续用已知语义交互与截图能力验证目标状态，并以未改 CSS 约束响应式范围。
- 显式刷新下稳定快照验证通过；尝试切换 390px 时确认当前 Browser API 不提供 `setViewportSize`，未改变页面，已记录且不重复。
- 48/48 契约、0-error check、生产构建和 Impeccable 检测通过；Finesse 仅有目标文件旧 CSS 的 3 个既有 P2。最后补齐零结果决策条的准确文案后重跑定向与差异卫生。
- 零结果决策条语义已通过先红后绿回归补齐，浏览器零结果与清空恢复均复核通过；正在执行最终全量验证和清理。
- 最终 48/48、0-error check、生产构建、两套检测器和 diff hygiene 均完成；进入隔离后端/Vite、临时数据库与浏览器页签清理。
- **Status:** complete locally; not deployed.
- 隔离服务已停止、端口已释放、Email hook 已确认仅为日志 stub、临时 DB/二进制/配置已永久清除；验证页签已释放到 about:blank。

# Session: 2026-08-16 - Jira 同步协程与间隔复核

- **Status:** Phase 1 in progress.
- 已加载项目规则、diagnosing-bugs 与 planning-with-files，并恢复既有 DL-4309 修复证据；下一步从当前代码和定向测试确认 goroutine 所有权及各层间隔。
- 已确认 `Server.Start -> go startJiraSyncWorker -> immediate runCycle + 30s ticker`；正在补齐配置化、失败重试、SSE keep-alive 与前端兜底间隔证据。
- 已补齐 Jira HTTP `10s` 超时、SSE `15s` heartbeat 与前端事件合并 `120ms`；下一步读取各页面轮询兜底和健康状态阈值，并运行定向回归。
- 已确认任务/需求/决策页兜底轮询分别为 `60s/15s/15s`，Daily Jira 为事件驱动且无周期轮询；准备核对状态 stale 阈值后运行症状级 Jira 回归。
- 状态接口以 `10m` 未成功或单轮未结束判定 stale；首次 Go 回归仅因沙箱禁止 httptest 监听而中止，已按 self-improving/3-strike 规则记录并切换为受控权限重跑，不重复原方式。
- 受控权限下 Go 症状级回归全部通过，前端调度器与消费者回归 7/7 通过。
- 本机 8080 状态探测连接失败，未擅自启动服务或连接真实 Jira；**Status:** current-source verification complete, runtime activation unverified.

# Session: 2026-08-16 - Daily Jira 属性变更滞留与水位日志修复

- **Status:** Phase 1 in progress.
- 已加载 diagnosing-bugs 与 planning-with-files；下一步先从 1068 行和 Daily Jira 候选调用缝建立两个红灯。
- 已定位 1068 为预期缺水位探测日志，并从主库确认更深层异常：最近全周期失败、1004 项全量处理、681 项写放大、成功水位为零。下一步验证是哪一字段反复变化及失败类别。
- 已确认周期错误来自绩效源事件不可变 payload 冲突；Daily Jira 服务端无缓存。准备建立“recent LastUpdate + Jira Done”后仍被候选返回的最小回归，同时审计性能事件失败是否应阻断核心同步水位。
- 新增 `TestJiraSyncCompletionOverridesRecentLocalProjectionAndRemovesDailyJiraCandidate` 并运行；它稳定红于 `status = progress, want done`，同时捕获用户看到的 1068 日志。Phase 1/2 完成，进入先红后绿修复。
- 新增水位日志红灯并确认准确失败于 1068 `record not found`；一次绩效样本查询因表名假设错误失败，已记录并改为从 schema 解析实际表名。
- 已从真实保留事件确认 payload schema drift：当前多出的 `parent_work_item_id` 造成 hash 冲突，而评分关联使用 TaskTelemetry。下一步加入 schema 稳定性红灯后实施三处最小修复。
- 已量化 schema 混存为 198/1117 条带新字段；确定不能原地改历史或全量删除字段，将以 typed conflict + legacy replay 兼容旧不可变事件。
- 三个后端红灯经 status authority、静默水位查询和 legacy snapshot replay 修复后已转绿。
- 主库实证出现“后端已更新、页面可能仍旧”的独立路径；确认 Daily Jira 没有轮询兜底。进入强制三方 UI 评审后再补前端恢复能力。
- Impeccable context/product register 与现有 token 已读；结论是不触碰视觉层级，只补可见页轮询、单飞/稳定替换和销毁清理。正在读取其余两方门禁。
- design-taste-frontend 主规则正在完整分段读取；已确定该产品表格只适用 preserve-mode，不执行营销页默认。
# 2026-08-16 Daily Jira 同步滞留修复

- 建立并跑通 4 个后端症状回归：完成态移出 Daily Jira、首次评论水位无 `record not found`、旧绩效快照兼容、较新的 Jira 负责人修改胜出。
- 修复 Jira 源状态采纳、负责人 freshness、GORM 水位探测噪声、不可变绩效快照 schema 演进；新增 typed conflict sentinel，非兼容冲突仍严格报错。
- 完成强制三方 UI 审查后，为 Daily Jira 增加 30 秒可见页兜底，保留 SSE 即时刷新与现有单飞队列；契约回归 3/3 通过，Impeccable detector 0 findings。
- `go test ./...` 全仓通过；`pnpm --dir web check` 0 error（既有 warnings）；`pnpm --dir web build` 成功。
- 登录态 Chrome 验证目标事项已从列表消失、console 0 error、2133px/421px 无横向溢出。
- 旧本地后端父子进程已按 PID 受控停止；新进程启动审批连续两次超时，未绕过权限。当前 8080 无监听，下一动作：在项目根目录运行 `go run cmd/server/main.go`，随后等待一个 30 秒周期确认 `/api/status` 的 `jira_sync.state=healthy`。
# 2026-08-19 页面与搜索数据加载变慢深层诊断

- 已加载项目冷启动规则、review/planning 路由、`diagnosing-bugs`、`planning-with-files` 与浏览器控制技能。
- 当前阶段：Phase 1，定位可复用的本地运行实例并建立只读性能基线。
- 已复用 Chrome 登录态完成真实决策看板首载与全局搜索；发现搜索会先进入任务表并加载 35,566 条全量数据，下一步用稳定完成标记重测时间并定位对应 API。
- 已定位前端放大链：全局搜索建议仅取 8 条，但跳转后任务表串行取尽全部分页并一次性渲染；现进入 Phase 2，检查后端每页查询、总数统计和数据库索引是否进一步放大。
- 已运行只读真实服务层探针：全量路径约 3.04s/72 页/33.1MB JSON，精确搜索约 43ms/1.6KB；下一步复跑确定稳定性，并检查渲染行数和刷新触发频率。
- 三次探针均约 3.0–3.1s，且发现 60 秒轮询与每个 telemetry 事件都会重跑全量路径；继续检查负责人维度、目录成本和最近变更，区分主因与放大因素。
- 已确认 490 个负责人带来额外多轮全表计算；SSE 全量刷新是当前未提交的新放大因素，基础全量加载在 HEAD 已存在。下一步验证数据规模/状态分布是否是触发性能拐点的变化。
- 已完成 HEAD/current 差分与 active-only 对照：历史 687 条时算法快，当前 35,566 条且 97% Done 后退化；非 Done 对照仅约 35ms。首要因果链已成立，继续量化 SQL 次数和前端渲染边界。
- 已追到数据增长来源：绩效历史 Jira 同步把历史样本写入 TaskKanban 共用的 `task_telemetries`，并用同步时间刷新 LastUpdate。当前阶段进入根因确认与修复边界整理。
- 已发现第二条全站放大链：30 秒 Jira worker 无界读取全部本地 Jira key，keep-alive 过滤函数未接入 reconciliation 构造，理论上每周期可形成约 708 个 50-key 查询；一次已观测周期约 20 秒。
- 已确认测试缝隙与运行态连续占用：old Done helper 未进入生产 JQL 构造测试；新同步轮次在上一轮完成约 9 秒后又开始。
- 已排除全局 HTTP 停顿：syncing 时状态接口仍约 1ms；任务链路无 gzip，但压缩不足以修复全量编排和 DOM。根因证据已完整，进入交付整理。
- 诊断完成：确认“绩效历史样本污染运营目录 → 35,566 条全量任务加载/渲染 → 高频轮询/SSE/Jira reconciliation 重复放大”的完整因果链。
- 已清理临时 Go 探针与 HEAD 数据库副本；仅保留本次规划/发现/进度和 warm-memory checkpoint，没有修改业务代码、主数据库或运行服务。

# 2026-08-19 Daily Jira 源同步与决策写回

- 已建立 NS2-2262 离开审计范围、空评论、指定人写回、指回报告人和 Jira 失败不落本地的后端红绿回归，5/5 通过。
- 已实现手动源同步、报告人投影、评论必填和原子 Jira 决策写回；页面提交后按 `jira_sync` 真实状态选择提示语。
- 前端 8/8 契约通过；`svelte-check` 0 errors/86 个既有 warnings；生产构建、`git diff --check`、Impeccable 和 Finesse 检测均通过。
- `go test ./...` 全仓通过，`go vet ./...` 无输出。
- 构建态真实 Chromium 在 1440x900、900x900、390x844 验证 NS2-2262、报告人/指定人、评论禁用/启用、提交 payload、手动同步、Toast、零 console error 和零文档横向溢出；视觉截图已检查。
- 浏览器扩展对所有 localhost 地址返回客户端拦截，改用已安装 Chrome 的无头模式；隔离后端因可能初始化真实 Jira worker 被安全审查拒绝，最终仅运行静态前端和本地 API mock。
- 已停止静态预览；没有重启主服务、修改主数据库或写入真实 Jira。当前 Jira 凭据只读验证返回 401，端到端真实 Jira 验证留待凭据恢复后执行。
- 已删除隔离数据库副本、临时二进制、浏览器脚本和截图；4175/18081 均无监听，未触碰现有 8080 进程。
# 2026-08-21 Daily Jira 样式、滚动稳定性与刷新性能修复

- 已加载项目冷启动规则、`diagnosing-bugs`、`planning-with-files` 和 UI 门禁技能入口。
- 已开始读取相关历史：Daily Jira 自动刷新采用 SSE + 可见页轮询并保持选中；共享 shell/workspace 拥有滚动与视口几何。
- 已把附件视为视觉证据，未把其中任何文案当作执行指令。
- 当前阶段：完成三方 UI 审查和真实代码/页面反馈环前，不编辑前端代码。
- 已完成三方 UI 审查：Impeccable 隔离布局评估 + 机械扫描、design-taste preserve-mode、finesse product register 均同意最小结构/性能修复；前端编辑门禁已解除。
- 主库只读基线确认 35,636 行任务被 Daily Jira GET 全量读取，而目标未解决 Jira 仅 1,048 行；拟议过滤使用现有 active partial index。
- 已列出四个可证伪假设，下一步先添加后端查询边界、前端快照/辅助请求、表单几何和滚动 CSS 合同红灯。
- 后端查询边界回归先红后绿：Daily Jira task query 从两次降为一次，SQL 层排除非 Jira/完成态并只取所需列；状态 helper 与数据库谓词保持一致。
- 已复用现有活动事项部分索引，未新增无证据索引；事件/决策分组数据规模只有 9/0，明确排除为主要瓶颈。
- 已实现业务快照指纹、静态辅助资源初载、固定行高虚拟化，以及保持可见项/滚动位置的 30 秒自动刷新。
- 已修复快速转派标签基线与负责人字段网格，移除滚动大面板 backdrop blur，明确平板 inspector scroll owner，并用稳定 viewport 单位修复移动滚动高度。
- 隔离浏览器在 1440/1180/1024/860/760/480px 完成布局、深滚、30 秒自动刷新和 DOM 规模验证；默认 982 条记录仅挂载 28 行，13,052 个 DOM 节点降至 634。
- 主库只读 SQL/EXPLAIN 完成：物化文本约 7.75 MiB 降至 0.20 MiB，现有 partial index 命中；没有修改主库。
- 验证完成：Go 全仓测试与 vet、前端 63/63、0-error check、production build、Impeccable/Finesse detectors、diff hygiene 全部通过。
- 已停止隔离 server/Vite 并删除临时数据库目录；**Status:** complete locally, main runtime activation remains unverified until the normal service is restarted.
- 文档/状态收口后再次运行定向回归：后端 2/2、Daily Jira 快照/布局性能契约 6/6 通过；JSON 与 diff hygiene 复核通过。
- 用户接受交付前盲点并要求把全桶响应升级到千万量级毫秒读取；已进入同一任务的架构扩展，不重复反思门禁。
- 已加载 `codebase-design`、`planning-with-files` 和强制三方 UI 门禁，记录 deep-module seam、10M 可测目标及 preserve-mode 保护规则；当前 Phase 1 审计接口与索引归属。
- Phase 1 发现：当前 GET 仍是全桶、全事件 `IN` 和浏览器全量数组；SQLite 已具备 FTS5，任务写入口分散，决定采用数据库触发器维护归一化读投影，并用 generation cursor 明确拒绝跨写入的失效深页。
- 第一组 4 个 read-module 回归按预期红，但同时发现 Go 驱动无 FTS5；已停止依赖 CLI 能力，改为 capability-detected optional FTS + 默认 B-tree prefix adapter，并把运行时 search mode 纳入可观察接口。
- 已完成 `internal/dailyjira` deep module、v3 migration、计数投影、后台自然日 rollover、100 行上限与 generation-bound row-value keyset cursor；无关 TaskTelemetry 字段更新不再使 cursor 失效。
- 已把 handler 改为只读取一页并对最多 100 个 task IDs 做 indexed Top-N event/reminder enrichment；前端改为服务端搜索、游标增量加载、虚拟窗口和同代多页原子刷新。
- 最终 10M disposable benchmark：首页面/cursor 页面 p95 为 0.824/0.845ms，选择性 key/title 搜索 p95 为 0.367/0.525ms，mid/tail raw seek p95 为 0.115/0.112ms；临时 6.25GB 数据库已删除。
- 全量 Go 测试与 vet、optional FTS build-tag suite、64/64 前端测试、0-error Svelte check、production build、Impeccable/Finesse detectors 与 targeted diff hygiene 全部通过。
- 登录态隔离浏览器完成 100→200→260 分页、真实 generation 变化、30 秒深滚自动刷新和搜索确认；滚动位置/高度/面板几何无变化且无错误。下一步只剩按正常流程迁移/重启主服务并采集生产 HTTP 与锁等待指标。

# 2026-08-23 Serena MCP 全局接入

- 已读取 `skill-installer`、OpenAI Docs、Self-Improving 和 planning-with-files 技能说明。
- 已核对 OpenAI 官方 MCP 配置与 Serena 官方 Quick Start/Codex 客户端文档，确认 Serena 应作为全局 STDIO MCP 安装，不是 `SKILL.md`。
- 已建立五阶段计划；下一步检查本机 `uv`/`serena`/Codex 配置与现有 hooks，再执行最小增量安装。
- 已安装并初始化 `uv 0.12.5`、`serena-agent 1.7.0`，生成 `~/.serena/serena_config.yml`，LSP backend 成功。
- 已在 `~/.codex/config.toml` 增量加入全局 Serena STDIO MCP；解析证明原配置除 Serena block 外完全不变，文件权限保持 0600。
- 已完成临时 Go 项目 MCP 协议验证：initialize、24 工具发现和 `get_symbols_overview(main.go)` 均成功，未在当前项目创建 `.serena` 目录。
- 已同步全局 AGENTS、规范仓库根/项目模板/coding domain 与当前项目 AGENTS/coding domain；规范版本升为 1.1.0。
- 当前项目严格 YAML、规范校验、8/8 单测、diff hygiene 和新 Git 项目 bootstrap 均通过；新项目来源版本为 1.1.0 并自动含 Serena 路由。
- 最终复核再次通过当前项目严格 YAML、规范仓库校验、8/8 单测与 `git diff --check`；确认当前项目未生成 `.serena`，规范仓库没有 remote，临时安装/冒烟目录均已清理。
- 交付前检查曾用过窄的两空格 YAML 匹配而返回 1；按实际四空格结构重查后 Serena 规则完整存在，这是检查命令问题，不是配置缺失。
- 已完成交付前 Agent 自检；用户安装 Codex CLI 后要求补充验证，本任务不重复门禁。
- `codex-cli 0.149.0` 的 `codex mcp list/get serena` 已确认 Serena 为 enabled STDIO server，命令、Codex context、15/120 秒超时和 writes 审批配置均正确。
- 临时只读 `codex exec` 会话实际通过 Serena 激活当前项目、读取初始指令，并成功读取 Go `main` 符号；随后补齐项目 Serena `go + svelte` 双语言配置，第二次真实调用成功解析 `DemandKanban.svelte`。
- 仅保留可版本化的 `.serena/project.yml` 与 `.serena/.gitignore`；已删除此次验证生成的符号缓存和本机覆盖文件。

## 2026-08-23 通用列表组件 / finesse-skill

- 已完成项目冷启动规则、复杂编码/设计/工具路由、skill-installer、planning-with-files、Impeccable、旧版 finesse 入口和浏览器技能加载。
- 已通过官方 GitHub 搜索确认上游为 `mouse-lin/finesse-skill`；首次 installer 临时安装未产出文件，已记录失败并切换验证路径。
- 已确认官方 `main` 最新提交为 `5050b6c71e27b829d1b3087d2be889d29c60db00`；稀疏 checkout 仍需在授权网络环境完成对象拉取。
- 已将全局 finesse 从 `0.2.0` 升级到 `0.20.0`；官方 56 文件逐项一致，旧目录已从临时隔离区删除（不可恢复，但官方旧版仍可按历史提交重新安装），项目工作树未做清理。
- 已完整加载升级后的 finesse 入口、`component-scope.md`、Impeccable 入口/product register 与 design-taste-frontend；接下来定位项目现有 tokens、列表/分页模式和预览所有权。
- Serena 已在 `/Users/eddie/Workspace/well-ambient` 激活；当前语言服务仅报告 Go，Svelte 语义检查将以现有 Svelte 工具链和精确搜索补足。
- 已实现 `AdminDataList.svelte`、`AdminPagination.svelte`、分页纯函数、独立 Vite preview 入口与 7 个契约/算法回归；真实调用方可传列、单元格、行操作和分页组件。
- 浏览器验证中补齐方向键横向滚动与 10px 外壳圆角，并生成桌面、状态、760/480/320 及窄屏操作列 8 张真实预览图。
- 实现后三方会审最终均 PASS；7/7 测试、0-error Svelte check、production build、Impeccable `[]`、Finesse `p0=0` 和 diff hygiene 全部通过。
- 已停止本地 Vite、删除错误/原始截图、临时 build、finesse 稀疏升级仓库和旧版隔离备份；只保留升级后的全局 skill、生产组件、preview 源码、测试与交付预览图。
- 用户反馈直接打开预览为空白；已用 diagnosing-bugs 建立自包含 HTML 红灯，确认根因是源码入口依赖 Vite 转译。
- 已新增 `web/scripts/build-admin-data-list-preview.mjs` 与 package script，把同一 Svelte 入口输出为 88 KB 单文件预览；两次生成哈希一致。
- 8/8 定向测试、0-error Svelte check、production build、浏览器分页/操作/零控制台错误全部通过；已停止临时 4174 预览服务并保留新的修复截图。

## 2026-08-24 通用列表懒加载与 Daily Jira 接入

- 已完成冷启动分类：coding.complex、continuation、frontend shared-module integration；启用 planning-with-files 与 codebase-design。
- 已读取 Daily Jira 相关记忆，锁定唯一左表 scroll owner、稳定 snapshot/selection 与服务端 cursor/generation 约束。
- 已建立五阶段计划；当前处于 Phase 1，前端编辑门禁未解除。
- Serena 针对 Svelte 的定向恢复仍失败，已按项目规则回退到精确 `rg`、分段源码读取、Svelte check 与浏览器证据；未修改用户现有 `.serena` 状态。
- 已确认 Daily Jira 当前由页面自行维护虚拟窗口、近底触发与 ResizeObserver；服务端 cursor/generation、同代刷新与合并逻辑可原样保留。
- 已向现有 Impeccable、design-taste-frontend、finesse-ui 三个会审角色发送只读评审，等待共同方向与分歧收敛；尚未编辑前端代码。
- 三方会审已收敛并写入计划：single scroll owner、embedded surface、fixed-row virtualization、edge-trigger load-more、native table + real row button、responsive column priority，以及 Daily Jira request generation/epoch 提交校验。
- 按 finesse Design Read 门禁暂停在实现前，等待用户确认 preserve-mode 方向；前端编辑仍未开始。
- 用户已确认开始实现；前端门禁解除，Phase 2 进入症状级回归编写。
- 新增虚拟窗口、近底触发、去重、共享组件 API、Daily Jira 委托与 stale generation 提交校验回归；修正测试运行器后得到真实业务红灯，目标能力尚未实现。
- 已实现共享列表虚拟化/懒加载与 Daily Jira 接入；定向回归 17/17、Svelte check 0 errors。当前进入全量契约、构建、UI 检测与运行态浏览器阶段。
- 通用预览运行态完成 100→200、失败保持、显式重试、连续追加至 1000/1000；同一 load key 无重复请求，DOM 始终约 25 条数据行。
- Chrome 在 1440/1024/760/480/390/320 六断点验证无 document 横向溢出，窄屏列优先级正确、操作控件最小高度 44px；预览截图已保存。
- 当前应用会话只到登录页，未使用或猜测凭据；真实 Daily Jira 登录态滚动仍是环境缺口，不能用静态结果冒充。
- 最终定向回归 20/20、全量前端契约 93/93、Svelte/TypeScript 0 errors、生产构建、单文件预览构建、Impeccable `[]`、Finesse `p0=0` 与 diff hygiene 均通过；本地 Vite 已停止。
- 用户确认继续解决 JS 行数组累计问题；已完成新一轮 Impeccable、design-taste-frontend、finesse-ui 会审并锁定服务端双向 keyset cursor + 前端 3 页有界窗口，不采用同样会无界增长的 cursor history 栈。
- 服务端已实现 `previous_cursor/has_previous` 与 `direction=previous`，读模型及 HTTP handler 的 next/previous 往返回归通过；Daily Jira 页面改为最多 3×100 行的双向页窗口。
- 浏览器首次仍加载旧的内联预览包，重建单文件后确认新逻辑；进一步发现虚拟器声明 48px、浏览器实际 59px 的几何偏差，已在 virtual 模式把数据行严格锁到声明高度。
- 真实分段滚动完成 `1–300 → 101–400 → 1–300 → 101–400`；向下淘汰锚点 `WA-0290` 的 y 偏移保持 `-31px`，窗口固定 300 行、DOM 固定 25 行，方向切换不再被旧消费键阻塞。
- 上一页失败、固定边界错误、可访问重试和恢复均通过；1440/1024/760/480/390/320 无 document 横溢，44px 控件不换行。新截图保存为 `artifacts/admin-data-list-bounded-window-preview.png`。
- 最终 Go 两包全量测试、前端 96/96、0-error check、production/single-file build、Impeccable、Finesse `p0=0` 与 diff hygiene 全通过；本地服务和临时浏览器标签已关闭。实际应用路由仍被登录页阻断，未使用或猜测凭据。

## 2026-08-24 决策面板 / Daily Jira 筛选区拆分

- 已完成项目规则、相关历史记忆、planning-with-files、Impeccable、design-taste-frontend 与 finesse-ui 主入口加载；继续遵守不调用 Serena 的既定要求。
- 已运行 Impeccable 上下文解析并确认范围化已有产品改造路径；当前进入三方会审与现状 DOM/令牌核对，尚未编辑前端文件。
- 一次计划记录补丁因锚点取自 `task_plan.md` 而非目标文件末尾而被拒绝，未产生文件改动；已改为每次先读取目标文件真实尾部再构造精确 patch。
- 已完整加载 Finesse redesign/product/palette/mobile 参考与最接近的轻色侧栏 dashboard 示例，并核对 `DESIGN.md` 与 `modern-admin-tokens.css`；颜色、导航和信息架构均列为保护项。
- 已定位两个页面的现有筛选、列表和断点样式：两者都需要抽取共享筛选表面合同，但保留页面自身状态、处理器、结果文案和业务控件。
- 已新增失败合同并实现 `AdminListFilterBar.svelte`；Decision 与 Daily Jira 均迁移为共享筛选组件紧邻 standalone `AdminDataList`，原父容器改为透明布局 owner。
- 19/19 定向合同通过；一次正则把 CSS `grid-template-columns` 误判为 Props 的 `columns`，收窄到接口块后通过，未掩盖实现错误。
- `pnpm check` 0 errors / 87 条既有 warnings、production build、110/110 全量前端合同和目标文件 diff hygiene 均通过。
- 已在登录态 Decision 与 Daily Jira 页面完成桌面、平板和移动断点验证；两个筛选区均是共享组件，列表内不含业务筛选，空搜索与真实键盘清空恢复通过。
- Daily Jira 真实滚动由 100 项补齐到 183 项，DOM 挂载行维持约 32–33，回到顶部后首批数据正常回补；没有触发“同步 Jira”写操作。
- 320/375/414/768 与既定 1440/1024/760/390 均无 document 横向溢出；预览页 11 个共享列表实例正常渲染，console error 为 0。
- Impeccable detector 返回 `[]`；Finesse strict 返回 `p0=0`，仅报告页面级既有 build-stamp/色值 P2。
- Finesse 768px floor 首轮量化显示 document 不横溢但部分筛选控件只有 30–36px；已把 Decision/Daily Jira 的筛选控件触控下限扩至 800px，并新增合同。认证会话随后过期，修复后的 768px 由源码合同、Svelte check 和 production build覆盖，未冒充已有第二张登录态截图。
- 浏览器验证中两次定位器能力假设不成立（Daily 搜索框误按 placeholder 作为可访问名、locator 无 `inputValue`）；均改为唯一业务选择器与只读 DOM value，没有产品代码改动。直接从预览导航回应用使临时验证页签的内存登录态失效，未读取或猜测凭据；此前完整登录态证据已保存于本轮输出。

## 2026-08-24 用户反馈：筛选条必须按列表组件样式生成

- 已复核反馈并确认不是主观差异：两个共享组件实际引用了不同背景、阴影、饱和度与根级焦点状态。
- 已完成 Impeccable、design-taste-frontend preserve、finesse-ui product-component 三方会审；共同裁决为保留筛选/列表职责分离，但共享一个 `AdminDataList` 表面合同，不再把筛选条做成第二种玻璃工具条。
- 已排序三个可证伪假设；下一步先补失败合同，再集中抽取表面令牌并做真实组件预览的计算样式对照。
- 新增共享 `--wa-admin-list-surface-*` 令牌和 `admin-data-list-surface/v1` DOM 合同；`AdminDataList` 与 `AdminListFilterBar` 均只消费该合同，筛选条根级 `focus-within` 已移除。
- 签入预览改为直接渲染真实筛选组件和真实列表组件，并提供搜索、项目筛选、重置与结果摘要；768px 实测发现并修复 36px 触控高度，最终 768/390/320 均为 44px。
- 真实预览 1440/768/390/320 的六项计算表面属性完全一致，document 横溢为 0；真实搜索 `WA-263` 只显示目标行，聚焦不再改变筛选外壳。
- 登录态决策事项和每日 Jira 当前源码均验证为 sibling、相同表面合同和相同计算样式；Daily Jira 仍为 100/183 数据窗口、约 30 个挂载行，没有执行同步 Jira。
- 最终 112/112 前端合同、0-error check、单文件预览/production build、Impeccable `[]`、Finesse `p0=0` 与 diff hygiene 通过；临时预览服务和验证标签已关闭，原 2133×902 业务标签保留。

## 2026-08-24 决策事项筛选选择态椭圆边框

- 已将任务分类为中风险前端回归，加载 diagnosing-bugs、Impeccable、design-taste-frontend、finesse-ui、planning-with-files 与 self-improvement。
- 已在修改前完成三方会审并记录共同方向、分歧裁决、组件所有权、响应式与验证范围；下一步先在登录态真实页面建立选择态几何红灯。
- 记录文件首次联合补丁因 `findings.md` 上下文不匹配而原子失败；改为读取真实 EOF 后分别追加，未触及产品源码。
- 已在真实登录态页面逐项点击三个下拉并建立红灯：负责人、项目、显示列的内部搜索输入全部在聚焦时绘制 999px 椭圆 outline，而外层 trigger 保持 10px；开始进入源码根因定位。
- 首轮定向合同 2/2 后复验发现单选仍有嵌套小焦点框；已收紧回归合同，要求内部 input 的 outline 显式压过页面全局 focus 规则，仅由外层 trigger 显示可见焦点。
- 第二轮定向合同 2/2，桌面三个下拉的内部 outline 均消失；非默认负责人/项目选择态也保持 10px 外框。760px 实测发现显示列高度未跟随其他两个下拉，已把该断点一致性加入失败合同。
- 新断点合同经一次 false-positive 修正后如期变红；增加 max-800 summary-mode 特异规则并删除冗余 max-640 重复后恢复 2/2。760/390 三下拉均约 44px、无内部 outline/椭圆且无 document 横溢，原浏览器视口与默认筛选已恢复。
- 交付级验证完成：114/114 前端合同、0-error check、production build、目标 diff hygiene、Impeccable `[]`、Finesse strict `p0=0`；登录态控制台无 error/warn，仅有 Vite debug/HMR 日志。

## 2026-08-24 Daily Jira 数量收拢到切换项

- 已重新加载 Impeccable、design-taste-frontend、finesse-ui product/component、planning-with-files、self-improvement 与浏览器验证规范；design-taste-frontend 明确只采用 preserve-mode。
- 三方会审已写入计划：切换项内部保留数量并强制单行，删除重复 meta 行，检查时间移入桌面首行且窄屏让位；共享列表和数据窗口不改。
- 已从截图、源码和登录态页面确认红灯结构：页面同时存在含数量的三个 tab 与独立 `.filter-meta` 第二行，下一步补失败合同后实施最小调整。
- 新合同在旧源码上 7/8，失败内容精确指向 Daily Jira 的 meta snippet；实现后 8/8。当前进入全量检查、设计检测与登录态断点复验。
- 登录态 2133×902 已完成首轮绿灯与视觉截图：筛选高度减少约 33px，无第二行，数量无换行，检查时间同行，列表仍挂载约 29 个虚拟行且 `aria-rowcount=184`。
- 1024×900 已通过：筛选条仍为单层表面，switch 数量无换行/无溢出，检查时间按响应式优先级隐藏，document overflow=0。
- 760×900、390×780 已通过：响应式仅保留两组主控件行，无重复数量 meta；三项 switch 高约 44px且数量同行，document overflow=0。
- 390px 三个 bucket 已逐个切换验证，列表分别显示对应总量并恢复到“7 日及以上 183”；切换前后数量始终同一行，未触发同步 Jira。
- 懒加载复验第一次直接赋值 `scrollTop` 被浏览器页面代理拒绝；已记录为工具限制，下一步改用 `scrollTo()` 完成相同验证，产品页面未发生错误或业务写操作。
- 改用 `scrollTo()` 后 390px 懒加载通过：虚拟范围从 5305px 扩至 9612px、DOM 数据行固定 23、总量 184（含表头）、无 meta/横溢；工具错误已标记 resolved。
- 浏览器状态已恢复到 2133×902、列表顶部和“7 日及以上 183”；最终日志零 error/warn，仅有 Vite debug/HMR。
- 320px 最窄屏补充回归先捕获最长 tab 的 2px 内部挤压，加入 max-360 精调后定向 8/8，真实运行三个 tab 均零溢出且保持 44px/12px。
- 最终交付级验证完成：115/115 前端合同、0-error check（87 个既有 warnings）、production build、diff hygiene、Impeccable `[]` 与 Finesse strict `p0=0`；登录态 2133/1024/760/390/320、三 bucket、近底懒加载和最终 console 均通过，页面已恢复默认视图。

## 2026-08-24 面板高度统一与排期治理列表/设置优化

- 已按 coding.complex + continuation + UI layout 回归分类，加载项目冷启动、planning、diagnosing-bugs、Impeccable layout/product、design-taste-frontend 与 finesse product/workflow/palette；当前未编辑产品 UI。
- 已记录实现前三方会审：统一 shell 几何合同，DemandKanban 列表迁移共享 `AdminDataList`，inspector 只重组强关联短字段；颜色、导航、业务状态与保存链路均受保护。
- Impeccable 要求的隔离机械预扫描完成：目标组件 detector `[]`、无 arbitrary Tailwind spacing/z-index。人工布局评估仍在独立进行。
- 下一步连接现有登录态页面，量出三页工作区、列表与 inspector 真实几何，建立红灯后再补合同和实现。
- 登录态红灯确认 Decision 多一个空 grid row，造成列表底线比 Daily Jira 高 12px；修正后两页根底线完全一致、列表底线差约 0.00003px，均无 document overflow。
- 排期治理已删除手写 table/虚拟 spacer/页面 scroll handler，直接使用共享 `AdminDataList`；104 条只挂载 28 行，列表和右 inspector 桌面高度均为 570.03px。
- 右侧排期设置收敛为两行：负责人+工时+难度、完成日+任务组；editor scrollHeight 从 665 降至 601，控件尺寸未缩小。
- 浏览器验收发现路由切换时 editable-demand 映射晚到会留下空表单，补上响应式回补条件后负责人、日期、任务组均恢复；列表行选择、代码轨迹联动与控制台均通过。
- 最终 121/121 前端合同、0-error check（87 个既有 warning）、production build、diff check、Impeccable `[]`、Finesse `p0=0` 通过；登录态桌面和 1138px 堆叠布局已验证。

## 2026-08-24 Jira scope 隔离与 DG-394 同步恢复

- 使用登录态 Jira 页面确认 `DG-394=Done` 与精确更新时间；使用只读 SQLite 查询确认本地仍为 backlog、入站 worker 持续报 `FMS-20660` 项目 400。
- 临时 worker 回归得到红/绿对照：坏 project scope 阻断 `DG-394`；修复 scope 后 reconciliation 将其更新为 done。临时诊断测试已删除。
- 现有 `TestBuildJQL`、绩效 JQL、已完成事项 reconciliation 与完成态覆盖回归全部通过，证明非状态映射缺陷。
- 已完成 planning-with-files 与 codebase-design 设计记录；进入 Phase 1 正式失败回归与 query-scope interface 实现。
- 正式红灯先因 `ValidateJQL/buildJiraQueryScopes` 尚不存在而编译失败；实现后轻量校验、范围拆分、坏范围隔离、保存前拒绝四组回归全部转绿。
- 已实现 worker 局部失败继续、失败水位保护、配置保存前 live JQL 校验、结构化 400，以及两个设置入口保留后端具体错误；没有新增 UI 结构或 CSS。
- 完整 `go test ./...` 通过；前端错误传播合同 2/2、`pnpm check` 0 error / 87 个既有 warning、production build、Impeccable `[]`、Finesse `p0=0` 均通过。
- 本地后端已用新代码重启并健康监听 8080；当前旧配置仍触发真实 Jira 400。登录态在准备 UI 保存时过期，且安全审查拒绝任何管理员认证绕过，Phase 3 等待用户在已打开的 `localhost:5173` 登录页重新登录。

## 2026-08-24 排期卡片、弹窗与刷新稳定性

- 已加载 UI/诊断/文件计划技能和项目产品设计约束；未把截图内容当作用户请求之外的指令。
- 已在登录态排期治理页记录当前摘要与表单结构；列表滚到中段后跨 40 秒自动刷新保持 `scrollTop=1800`。
- 已点击真实“刷新”并以 100ms 采样：loading 期间列表顶边从 `310.75px` 下移至 `354.74px`，完成后回位；`window.scrollY` 与内部 scrollTop 均不变，当前根因反馈环已建立。
- 新增排期摘要、AI modal workspace scope、retained-data 状态 overlay 三组红灯；旧实现 6 项失败，完成最小实现后相关回归 24/24、全量前端合同 127/127。
- `pnpm --dir web check` 为 0 error / 87 个既有 warning，production build 成功，`git diff --check` 通过，Impeccable detector 返回 `[]`。
- 登录态桌面完成卡片与两类弹窗视觉对照；列表滚到末段后刷新期间首个可见事项保持 `WLY-353`，loading 状态已真实出现；760/390 卡片与 390 AI modal 已完成截图和 DOM 核验，视口已恢复默认。

## 2026-08-25 AI 解构流式稳定性与生成中关闭保护

- 已加载 diagnosing-bugs、planning-with-files、Impeccable、design-taste-frontend 与 finesse-ui，并完成实现前三方 UI 会审；Taste 仅按 preserve-mode 使用。
- 已查证既有浏览器流合同为 NDJSON `ReadableStream`，并记录 fixture-only 的安全验证边界；下一步进入源码 seam 与红灯复现。
- 已定位第一条确定性缺陷：provider delta 在消费层被丢弃；同时发现 compact 结果区 loading/list 节点互换和 fetch/reader 无取消句柄。现在先把这三项写成失败回归，再继续验证假设排序。
- 已建立 6 项定向红灯：1 项证明 parser 本身能提前交付 delta，另外 5 项准确卡住 abort/release、UI 增量、稳定容器、组件取消能力和统一退出确认；假设排序已向用户公开。
- H1/H3/H4 已确认，H5 已由逐事件 Flush 证据排除；H2 源码分支确认，待浏览器量化后进入实现。
- 登录态基线确认 result panel 空闲时完全不存在，外层 modal/body 几何已记录；五项假设闭环完成，Phase 1 可转入实现。
- 已实现增量文本缓冲+rAF 批量刷新、常驻结果 owner、fetch/reader 双层取消、reader lock 释放，以及统一 request/confirm/force-close 状态机；定向 6/6 转绿。
- 相关弹窗/排期回归合计 15/15、全量前端合同 133/133，`pnpm --dir web check` 0 error / 87 个既有 warning，production build 与 diff hygiene 通过。
- Impeccable detector 无 finding；Finesse `p0=0`。报告的纯白、transition-all、直写色值和缺少整页 stamp 都来自目标大组件既有区域，未由本轮新增代码引入。
- 登录态桌面验证中，生成前后 modal 几何固定为 980x682.2；流式内容持续增长，result body 常驻。760/390 断点无 document 横向溢出，关闭按钮约 44px。
- 使用验证后删除的本地慢流夹具完成关闭状态机实测：继续等待保持流增长；确认停止并关闭调用底层 `ReadableStream.cancel()`，然后卸载两层 dialog。未发送真实 Jira 内容。
- 浏览器测试页已恢复默认视口；临时夹具源码已删除，最终构建基于正式代码。当前进入交付前双问题自审。
- Finesse 精确断点补测捕获并修复 compact“生成解构”按钮 36px 触控高度；320/375/414/768 复验均约 44px、nowrap、零横溢、单栏。最终再跑 133/133、check/build、Impeccable/Finesse 与 diff check 全通过。

## 2026-08-25 AI 解构完成结果关闭后恢复

- 已加载 diagnosing-bugs、planning-with-files、项目 Impeccable、design-taste-frontend 与 finesse-ui；三方会审裁决为保持 UI 不变，把最后一次完整结果从瞬时弹窗提升到排期页会话层。
- 当前尚未编辑产品源码。下一步先审计现有任务组/需求 identity 与 mount/close 路径，建立同需求恢复、跨需求隔离、未完成不覆盖三类红灯。
- Finesse product、AI console 与 redesign 参考已完整加载；共同约束再次确认：完成产物属于工作流过去时，但本轮仅修复状态所有权，不扩展为历史中心、不改视觉结构。
- 已确认关闭函数没有主动清空字段，根因是子组件卸载；同时发现服务端有最新解构归档读取接口，但其权限高于生成接口。本轮先以父层会话快照保证所有生成用户可恢复，不把修复绑定到额外权限。
- 已审计结果修改入口：负责人/优先级/难度/工时/删除都会更新 `result.tasks`；快照应在“有完整结果且当前未生成”时统一发布，才能保留用户对完整结果的本地修订，并避免逐个遗漏 mutation。
- 定向红灯已建立并得到 0/3：会话模块、子组件完成态合同、父组件隔离缓存三层均按预期失败；现在进入最小实现。
- 已实现有界会话缓存、稳定 demand/draft key、克隆式恢复、子组件初始 hydrate 与完整空闲态发布、父组件按上下文注入；新一轮生成不再清空上次完整结果的组 ID/mock 标志。正在运行定向回归。
- 定向相关回归 10/10；`pnpm --dir web check` 0 error、87 个既有 warning。下一步进入浏览器 fixture 的完成→关闭→重开与跨需求隔离验证。
- 已连接现有登录态排期页（梁志远）并确认列表/inspector 正常；现只在 `import.meta.env.DEV + deconstruct_fixture=retention` 下加入可删除的本地完成流，确保真实点击链不发送 Jira 内容或调用模型。
- 首次验证沿用了不支持的 `goTo` 大小写，未发生导航；已查到当前客户端合同为 `tab.goto()` 并更新既有工具学习记录，产品代码未受影响。
- 登录态 fixture 核心链通过：HR-4202 完成→关闭→重开直接恢复；ZPU-2769 打开为空，不串结果；再回 HR-4202 仍恢复。下一步补窄屏几何与生成中取消兼容检查，然后删除夹具。
- 390px 恢复结果通过：DOM 横溢 0、工作台横溢 0、结果单列且滚动 owner 正常；截图已确认任务建议、88% 完整性与分析均可见。补测取消时裸 `history` 不可用，未点击业务控件，已改为 `window.history`。
- 页面沙箱最终不支持任何 history owner，改用根节点 dataset；随后父组件 HMR 正常关闭了 modal，旧定位器无匹配且未执行操作。已记录工具学习，重新从当前排期页建立验证状态。
- 完整结果已重新建立。直接 dataset 赋值又被页面代理拒绝，仍未启动新生成；验证方式收敛到已证明可用的 DOM 方法 `setAttribute`。
- 根元素 `setAttribute` 也不在页面代理能力内，仍未启动新生成。最终改为用真实可见输入 `[fixture:slow]` 选择慢流，避免任何浏览器内部状态写入；HMR 后将按正常 UI 重建最后一轮状态。
- `fill()` 未传播 Svelte 绑定，慢流未启动、空闲关闭直接生效；脚本虽已计算确认不存在却仍继续点击，产生无匹配错误。现已拆成条件化阶段，改用真实键盘序列并先读回输入值。
- 键盘值虽可读回，仍未确定性切换提交分支；夹具最终改为同页第一次完成、第二次慢流的固定序列。下一轮从新加载页面开始，避免继承任何旧调用计数。
- 确定序列绿灯：第一次完整、第二次慢流、关闭确认、确认取消、重开恢复第一次结果全部通过；临时 fixture 文件、import 与 fetch 分支已删除，正式代码不含验证入口。
- 最终全量前端合同 137/137，`pnpm --dir web check` 为 0 errors / 87 个既有 warnings，production build 成功；Impeccable `[]`、Finesse strict `p0=0`、目标 diff hygiene 均通过。
- 浏览器已导航回 `http://localhost:5173/` 并恢复登录态排期看板，验证参数已清理；最终 reload 后 console error/warn 为 0。

## 2026-08-25 排期/版本/任务/配置四页面收敛

- 已启用 planning-with-files，并加载项目 Impeccable、design-taste-frontend preserve、finesse-ui product/workflow/redesign 门禁；当前尚未编辑产品源码。
- Design Read 固定为现有浅色 teal 研发管理台，低动效、高密度、零图片；任务/排期以列表为中心，配置版本改为共享列表管理，不引入新卡片系统。
- 下一步先审计风险日历、版本动作、TaskKanban 共享布局和 SettingsPanel/各配置版本所有权，再以四条症状级合同建立红灯。
- 已完成三方会审与登录态基线：风险日历占右栏底部 110px；版本前后端生命周期与目标相反；任务表顶部卡片合计约 235px、执行追踪 116px；逐配置版本侧栏宽约 391px 且随表单拉到近 2000px。
- 共同实现方向已写入任务计划：排期停止挂载/请求风险日历；版本增加 current/archive tab 且生命周期收敛为 archive→delete；任务两页统一紧凑 summary strip；配置版本独立路由，普通配置页单列并收紧表单节奏。
- 四条失败合同已转绿；版本服务端补齐 planned/released/discarded→archived、仅 archived 删除、current/archive 列表隔离和能力矩阵回归，旧生命周期测试同步更新。
- 全量 Go 测试在允许本地 `httptest` 监听后全部通过；前端 141/141 合同、`svelte-check` 0 error、production build 与目标 `git diff --check` 通过。
- Impeccable detector 为 `[]`；Finesse 无 P0，仅报告目标大组件既有直写色值、transition/build-stamp P2，未扩大为主题重写。
- 登录态桌面验证：排期风险日历节点 0；任务表/执行追踪 summary 均 73px，工作区 y=235；普通配置页无版本 aside，统一版本页显示 27 个版本；版本页有 current/archive Tab 且当前页无发布/删除入口。
- 751/350px 真实断点发现并修复任务摘要塌陷、版本筛选旧网格位和 28–40px 触控目标；最终四页面均无 document 横溢，350px 操作控件为 44px，浏览器 console error/warn 为 0，视口已恢复默认。
- 用户指出配置页失去右侧面板后，新增失败合同并恢复普通集成配置的上下文检查器；独立配置版本页继续全宽，检查器不包含版本列表、diff 或回滚。
- 只读浏览器预览复验桌面 1368px + 340px 双栏、751/350px 顺序堆叠和零横溢；GitLab/Jira/AI 均通过。最终 141/141 前端合同、0-error check、production build、Impeccable 与 Finesse P0 门禁通过。
## 2026-08-26 Linux/PostgreSQL 一键部署与风险收口

- 已完成：读取项目冷启动/设计/规划规则，启用 `planning-with-files` 与 `codebase-design`。
- 已完成：只读盘点数据库初始化、配置版本、健康检查、前端同源接口、产物平台、Git/CI/部署文件和持久化路径。
- 已确认：用户授权数据库配置化、生产 PostgreSQL、Linux 目标、示例凭据清理及剩余部署风险修复。
- 当前阶段：Phase 0，准备建立配置/数据库/迁移安全合同；尚未修改生产业务代码或当前数据库。
- 已触及：仅追加 `task_plan.md`、`findings.md`、`progress.md` 当前任务记录；未覆盖既有任务内容。
- 已完成 PostgreSQL dialect 初查：定位 26 个 SQLite 相关生产文件；下一步先锁定在线请求/worker 必需的 `dailyjira`、`readmodel` 与不可变数据资产兼容面，离线 SQLite 工具保持显式单用途。
- 环境检查：Docker CLI 不可用、PostgreSQL adapter 未缓存；已记录验证边界，不将静态 Dockerfile 检查冒充真实镜像运行。
- 已完成：加入 `gorm.io/driver/postgres v1.6.0` 与 pgx 依赖；首次沙箱 DNS 失败已按审批流程成功恢复。
- 已实现第一批配置安全改造：数据库 bootstrap 配置解析/校验/连接池默认、API/版本档案排除、保存与回滚保留；示例 token/secret 值已机械替换为不可用占位符并加入本地 SQLite 数据库示例。
- 定向验证：`internal/config` 与 `internal/server` 的数据库配置/bootstrap 版本测试通过；隔离依赖缓存首次缺包问题已记录并恢复。
- 已实现数据库 adapter 初始化骨架、连接池/ping/close、独立 migration interface；`cmd/server` 已接入数据库配置、migration-only/skip-migrate、构建信息和优雅服务生命周期。编译与方言迁移验证待下一步完成。
- 数据库/服务骨架定向编译测试已通过；已选定 PostgreSQL Daily Jira adapter 方案，开始实现 PostgreSQL read generation trigger 与直接分页查询。
- 一次 Daily Jira 广补丁因事务上下文假设错误而原子失败、未产生部分修改；已按失败复盘拆分，完成 PostgreSQL migration dispatch 与 repeatable-read 代际读取。
- 已完成 PostgreSQL Daily Jira 直接读取 adapter：保留 scope、search、bucket、双向 keyset cursor、summary 与 generation stale 检测；PostgreSQL 时间桶在 repeatable-read 内按 CURRENT_DATE 推导，不运行 SQLite rollover。
- 测试诊断：非 server 目标包通过；server 全量在 bulk planning 卡住。90 秒超时堆栈确认是本轮 SQLite 单连接池默认导致的嵌套读取等待，已进入 adapter 级修复，不是业务测试随机慢。
- SQLite adapter 已改为共享内存 DSN、4/2 小型连接池、5 秒 busy timeout、foreign keys，文件库启用 WAL；原卡住的 `TestBulkPlanningIsAtomicAndDecodesPlanningFields` 现于 0.42s 通过。
- PostgreSQL read generation 使用 statement-level trigger 并把 generation/时间戳列设为 BIGINT；data-asset 五类追加式表已补 PostgreSQL update/delete/replace 数据库 guard。相关 readmodel/dailyjira/db 测试通过。
- 已完成配置前端调用链审计；下一步仅改后端响应/保存/测试解析，不触碰 UI 文件，避免触发不必要的视觉改造面。
- 配置秘密保护已实现并定向通过：`GET /api/config` 不再返回数据库 DSN 或六类集成秘密，只返回 `__configured__`；保存、连接测试和 GitLab 项目读取会在服务端恢复真实值，显式空值仍可清除秘密。
- 已增加 liveness/readiness/build identity 回归；已拆开 read generation 的写入迁移与只读运行验收，避免 `--skip-migrate` 仍隐式修改 schema。
- Linux 基础镜像标签已按官方当前标签核对；准备生成多阶段 server/web 镜像与 PostgreSQL 17 Compose 交付面。
- 已完成 PostgreSQL 在线兼容面：配置化 adapter、连接池/ping/close、显式迁移、Daily Jira 直接分页/summary、PostgreSQL generation trigger、数据资产 guard、跨方言二进制列与 solution catalog epoch 查询。
- 已完成配置秘密收口：公开配置 marker、保存/测试继承、数据库 bootstrap 隔离、配置版本明文清洗；示例六类敏感字段的 placeholder-only 静态门禁通过。
- 已完成 Linux 交付文件：多阶段 Dockerfile、PostgreSQL/server/migrate/web Compose、Nginx 同源代理、生产示例、Makefile、部署/回滚脚本、Linux 手册和时序数据库 ADR。
- 已从 Git 索引移除 `well-ambient.db`，本地 375MiB 数据库文件保留未改；未重写历史、未连接生产、未搬迁当前 SQLite 数据。
- 交付验证：`go test ./...` 全通过；`go vet ./...` 通过；Go module verify 通过；Linux AMD64/ARM64 交叉构建生成静态 ELF；`pnpm -C web check` 0 error/148 个既有 warning，production build 通过。
- 静态部署门禁：两份 bash 脚本 `bash -n`、可执行位、Compose/YAML 结构合同、敏感示例门禁、DB 不再 tracked、目标 `git diff --check` 全通过。
- 隔离生命周期烟测：`/live=OK`、数据库 `/ready=OK`、version/commit 注入正确、SIGTERM 优雅退出成功。
- 环境缺口：本机无 Docker/PostgreSQL，未冒充完成真实镜像、Compose 或 PostgreSQL 集成验证；这些步骤及 SQLite 历史数据迁移已写入 staging 门禁。

## 2026-08-26 初次上线数据库引导与迁移指南

- 已启用 planning-with-files、codebase-design、Impeccable onboarding/product、design-taste-frontend、finesse-ui、docs-write；尚未编辑 frontend 或业务启动代码。
- 已确定核心安全方向：显式一次性 setup 状态 + token 授权 + 空库初始化/既有库验收分流 + 0600 bootstrap YAML 持久化；数据库断连不进入 setup。
- 当前进入 Phase 0，继续加载完整 UI 技能并审计 App/auth、配置写入、server main 与 Compose 首次启动路径。
- 已实现显式 `database.driver: setup` 启动分支；setup server 不初始化业务数据库、不挂载普通 API，成功后把 URL-safe PostgreSQL DSN 原子写入 `0600` runtime YAML 并退出，由 Compose 重启正常服务。
- 已实现 token 保护的数据库 test/apply、PostgreSQL 14+ 检查、空 schema/完整当前 schema/未知 schema 三态识别，以及空库初始化和已迁移库接入两条互斥路径。
- 已把首次安装接入 Linux Compose 与部署脚本；首次部署在 setup ready 后退出供管理员配置，后续内部 PG 自动 custom dump，外部 PG 没有明确备份引用则 fail-closed。
- 已实现 `App.svelte` 启动前 setup probe 与 `DatabaseSetup.svelte` 一次性表单；密码/token 不进浏览器缓存，信息变化强制重测，成功后清理秘密并等待服务重启。
- 已新增 SQLite → PostgreSQL 上线迁移指南，并更新 Linux 部署手册：覆盖停写快照、完整性/外键检查、附件哈希、显式字段映射、sequence、读模型重建、验收、切流与回滚；明确当前没有生产可用的自动搬迁器。
- 定向 setup/config/schema 单元测试、全量 `go test ./...`（获批本地 httptest 监听）、`go vet ./...`、`go mod verify`、146 项前端合同、`pnpm check`（0 error/148 既有 warning）、production build、bash/YAML/diff 静态门禁均通过。
- Impeccable 目标扫描返回 `[]`；Finesse 无 P0。浏览器完成 setup、normal、recovery 三态，以及 1280/768/414/320px、键盘焦点、44px 目标、秘密不回显和零横溢验证。
- 当前进入交付前反思门禁。真实 PostgreSQL/Compose/SSL/权限与生产 SQLite 数据迁移尚未运行，必须在 Linux staging 独立验收。

## 2026-08-26 首次安装本地 SQLite 一次性迁移决策

- 已重新加载 planning-with-files、codebase-design、项目 Impeccable、design-taste-frontend 与 finesse-ui，并完成新范围的三方 UI 会审；尚未编辑产品或迁移源码。
- 已确认现有安装页已具备 token 保护的真实 PostgreSQL 连接测试，但仍是单页；新范围将改为连接测试 → 本地数据决策/最终执行两步。
- 已确认根目录存在 `well-ambient.db`，而生产容器当前不挂载该文件；计划通过显式受控 runtime legacy 路径检测，不扩大为目录扫描或任意路径探测。
- 已确认当前仓库无可直接生产使用的 SQLite → PostgreSQL mover；`RequiredSchemaModels()` 可提供 owned-table 白名单和顺序，但 seed、guard、sequence、read generation 与大库时长仍需新合同和 staging 证据。
- 下一步按 finesse 门禁先提交 Design Read 等待用户确认；确认前不编辑 frontend。
- 用户已确认开始实施，并明确目标数据库在全新 PG 中应不存在；三方 UI 方向门禁解除。
- 后端已实现维护库探测、缺库/建库权限事实、显式 `CREATE DATABASE ... TEMPLATE template0`、一次性 SQLite 选择与异步安装状态；setup 专项测试通过。
- 数据层已实现只读 SQLite 快照校验、owned-table 分批复制、事务回滚、逐表行数验证、PG sequence 重置、read model/seed 收口和 `ANALYZE`；小型 SQLite→SQLite 方言模拟通过，用于验证复制算法，不冒充真实 PG。
- PG 专用索引清单和方言跳过合同已落地；`internal/db`、`internal/config` 定向包测试通过。
- 前端两步页已完成：第一步连接测试明确“目标数据库不存在”，第二步按服务端快照事实只提示一次迁移选择，并轮询表/行进度。`pnpm check` 0 error、production build 成功，输出仅含仓库既有 warnings/chunk 提示。
- Compose 现只预建维护库 `postgres`；部署 runtime 新增 legacy 目录，文档已改为当前自动迁移合同与 staging 门禁；`config.example.yaml` 已清除真实组织/项目/JQL/人员上下文。
- 当前进入完整本地 SQLite 快照模拟、全量测试、静态/设计检测、浏览器断点验证和精确 Git 提交阶段。
- 真实根目录 SQLite 快照最终模拟通过：按 owned-table 白名单从受控只读文件复制 466,555 行、63 张表；该结果只证明迁移算法和事务路径，不冒充真实 PostgreSQL 方言验收。
- 最终验证通过：全量 Go 测试与 vet、148 项前端合同、Svelte 0-error 检查、production build、bash/YAML/敏感示例/diff 门禁；浏览器覆盖缺库、一次性迁移选择、失败重试、正常入口与 1280/768/414/320px 零横溢。
- 已精确暂存本任务数据库/部署/安装页文件，保留 Jira、交付计划、管理台等无关脏改动；从 Git index 导出的独立临时树完成全 Go 编译、相关 Go 测试、setup 7 项合同、Svelte 检查和生产构建。
- 已创建提交 `3a447f7 feat: add guided postgres setup and legacy migration`；提交后新增行凭据扫描为 clean，无关工作树修改仍未暂存、未提交。

## 2026-08-26 外置 PostgreSQL Compose 改造

- 已启用 planning-with-files 与 docs-write，读取当前 Compose、生产示例、部署脚本和 Linux 手册。
- 已确认目标为纯应用 Compose：用户现有 PostgreSQL 留在 Compose 外部，不执行任何数据库远程操作。
- 已确认仅移动当前 Compose 不足以启动，因为镜像仍依赖本地源码构建；下一步改为可配置的预构建镜像引用，并同步部署脚本和搬运清单。
- `docs-write` 声明的共享风格指南文件在安装目录中缺失，已按 self-improving 归因为技能资源问题并记录；本轮继续遵循主技能中的行动优先、占位符、可执行示例和格式规则。
- 已完成强制前端三方门禁：三方共同裁决为 UI、层级、组件所有权和响应式全部不变，只更新部署合同测试；浏览器视觉回归不在本次必要范围。
- `compose.yaml` 已删除 postgres service、postgres_data、内部 DSN、build context 和依赖，保留 migrate/server/web；server/migrate 增加 Linux `host.docker.internal:host-gateway` 映射。
- `.env.production.example` 已改为预构建 server/web 镜像、不可变版本和运行 UID/GID；Makefile 支持本地构建与离线 image bundle。
- `deploy.sh` 已移除内部 PG 启动和 `pg_dump` 分支，首次安装直接启动应用，后续迁移必须提供外部备份 ID；`rollback.sh` 不再假设本地固定镜像名。
- 新增 `make compose-bundle`，生成只含 Compose、生产环境模板、运行配置模板和部署/回滚脚本的可上传压缩包；镜像可独立走仓库或 `make image-bundle` 离线导入。
- 修复部署脚本覆盖 `.env.production` 中 `APP_UID/APP_GID` 的风险；以 root 执行脚本时不再隐式把应用容器改为 root 用户。
- Linux 手册已给出打包、`scp`、服务器解压、镜像推送/离线导入、环境初始化、Compose 校验与首次启动命令，并明确同机 PG 使用 `host.docker.internal` 而非容器内 `127.0.0.1`。
- 验证完成：shell 语法、YAML 三服务/无 PG/无本地 build 合同、部署包内容、示例占位符、可执行位和精确 diff 通过；数据库安装定向合同 8/8、前端全量合同 149/149、Svelte 0 error（148 个既有 warning）、production build 通过。
- 当前开发机仍没有 Docker/PostgreSQL，因此没有伪装完成目标服务器上的真实拉取、Compose 启动或容器到现有 PG 的网络/SSL/权限验收；服务器操作清单已写入手册。
- 已生成可直接上传的 `deploy/bundles/well-ambient-compose-2026.08.26-1.tar.gz`（4.6K，SHA-256 `479d8fe3e990e5e3c215162324449bf9ba74076346ddf3c28a561e2ad7d814e2`）；该目录已忽略，不进入 Git 提交。
## 2026-08-27 首次安装令牌自动生成

- 用户已确认：自动生成时可在终端打印完整令牌和临时文件路径，并接受 Docker 日志保留副本的安全取舍。
- 已加载 `planning-with-files`、`docs-write`、`codebase-design` 和失败后的 `self-improvement` 约束。
- 已完成只读盘点；下一步先补令牌 provision 单元测试，再实现启动入口与部署合同。
- 红灯合同已建立：`go test ./internal/server -run TestProvisionSetupToken` 因 provision 接口尚不存在而按预期失败。
- 第一次启动入口整块补丁因现有文案上下文漂移而原子失败，未产生半成品；已改为按当前文件小块提交。
- 已实现深模块令牌 provision，并接入 `cmd/server`：只有自动生成分支会打印完整令牌与 `0600` 文件路径；setup server 退出后先清理文件，再处理启动错误。
- provision 定向测试已从红转绿，显式 token、自动生成、权限、文件内容、随机源失败和幂等清理均通过；现同步部署入口和说明。
- Compose 已允许空令牌；部署脚本只校验非空显式令牌，并在自动生成时从 server 日志回显本次令牌和临时文件路径。
- 生产环境示例、本地 setup 示例、Linux 部署手册和前端部署合同已同步。两次多文件文案上下文漂移均为原子失败，拆分后完成。
- Prettier 未安装，文档技能的格式化命令无法执行且未改文件；改用仓库已有检查，不下载新依赖。
- 本地无 Docker CLI，Compose runtime 校验未执行；改用 YAML 解析与静态部署合同，并将真实 Docker 启动列为环境缺口。
- 定向 server 测试、16 项安装合同和 `bash -n deploy/deploy.sh` 已通过；Ruby 2.6 首次 YAML 调用仅因 API 版本不兼容失败，正改用兼容形式重试。
- Ruby 2.6 兼容形式已成功解析 `compose.yaml`；进入全量 Go、前端检查/构建和隔离进程烟测。
- `go vet ./...`、前端类型/Svelte 检查和生产构建已通过；全量 Go 仅被沙箱禁止既有 `httptest` loopback 阻断，按流程在受控权限下重跑。
- 全量 Go 测试在获批 loopback 权限下通过。隔离 18198 进程已生成 64 位十六进制令牌，临时文件为 `0600`、65 字节；待同权限 `/ready` 与退出清理验证。
- 隔离 18198 `/ready` 返回 `SETUP`；SIGINT 后进程退出码 0，自动令牌文件确认消失。现有 18197 引导环境未重启。
- 本任务实现与验证阶段均完成，进入精确差异审查和临时产物清理。
- 精确差异审查与 `git diff --check` 通过；烟测配置、二进制和令牌文件均已清理。未触碰现有 18197 引导进程，未修改或提交无关工作树内容。
## 2026-08-27 SQLite 迁移 PostgreSQL 22021

- 502 修复红灯已转绿：launcher 合同要求 backend host/port 覆盖；server 在 runtime config restore 后重新应用该覆盖，空覆盖保持数据库运行地址不变。
- 隔离真实 PG 配置验证：数据库恢复 runtime config version 28 后，新版仍监听 18198，`/ready=200`、setup API=404。
- 当前现场已替换为新版 18197 正式后端并停止旧 8080 supervisor；5175 HTML=200、同源 setup API=404（非 502）、18197 `/ready=200`。
- 最终 `go test ./...`、`go vet ./...`、launcher `bash -n`、目标 `git diff --check` 全通过。
- 用户真实 PostgreSQL 重试已越过 22021 并完成迁移；随后复现新红灯：5175 `/api/setup/status`=502、18197 未监听、8080 `/ready`=200。
- 已确认 runtime config restore 将 server.port 从临时 YAML 的 18197 恢复为历史 8080；进入本地进程级地址覆盖回归测试与修复。
- 两条最小红灯已先失败后转绿：`firstCommitLine` 不再返回非法 UTF-8；迁移批次会修复非法 UTF-8 和 NUL，并将修复值计入 `TextValuesRepaired`。
- 真实 408.6 MB 快照全量只读模拟通过：复制 470,518 行、64 张应用表，目标事务和引用数据初始化完成。
- setup backend 现在会在服务端日志记录修复的历史文本值数量，不输出原始业务内容。
- 只读 SQLite 检查通过：408,551,424 字节、69 表、UTF-8、quick_check=ok；默认沙箱 CLI 无法打开后改用显式 `mode=ro&immutable=1` 受控读取。
- 临时诊断测试遍历 `RequiredSchemaModels` 对应表的全部 TEXT 列，红灯精确落在 `notifications.message` 两条记录；未输出消息正文。
- 进一步确认 `notifications.id=277/284` 均为 semantic link review，坏字节位于 `firstCommitLine` 生成的 commit 摘要尾部。
- 已启用 `diagnosing-bugs` 与 `planning-with-files`；先建立 SQLSTATE 22021 的红灯反馈循环，再修复。
- 旧任务记忆只确认此前真实 PostgreSQL 迁移曾是未验证项；本轮不沿用旧结论，以当前日志、SQLite 字节和迁移代码为准。

## 2026-08-27 本地开发环境未进入引导页

- 已加载 `diagnosing-bugs`、浏览器控制、`planning-with-files` 和 `self-improvement`。
- 已建立四阶段诊断计划；尚未修改产品代码或运行配置。
- 浏览器连接成功，但 ambient 提示的 5175 标签不可接管；未触碰用户标签，改用同浏览器临时标签复现。
- 原始浏览器反馈循环已红：5175 连接被拒绝。下一步分别检查 5175 Vite 与 18197 setup backend 监听状态和本地启动入口。
- 端口/配置最小化完成：Vite 和 18197 均未运行，8080 指向本地 `config.yaml` 的非 setup 模式；仓库缺少一键启动 setup backend + 正确 Vite proxy 的本地入口。
- 红灯命令已固定：先请求 5175 HTML，再断言同源 `/api/setup/status` 包含 `setup_required: true`；当前稳定在第一步连接失败。默认 Vite 首次启动被沙箱监听限制阻断，正按受控权限重跑。
- Vite 已在获批的 5175 loopback 上启动。浏览器策略阻止自动刷新本地 URL，停止该自动化路径，改用同源只读 HTTP 探针。
- 默认 Vite 场景最小化为 `/api/setup/status` 404；切换到 18197 setup proxy 后原反馈循环已转绿，返回 `setup_required: true` 和真实 SQLite 快照事实。
- 当前临时 5175/18197 环境已恢复。进入持久化修复：先为 `make dev-setup` 与 launcher wiring 建立失败合同，再实现脚本。
- 持久化修复完成：新增 `scripts/dev-setup.sh` 与 `make dev-setup`，使用 owner-only 临时配置启动 setup backend，自动识别仓库根目录 SQLite，并把 Vite 同源 `/api` 固定代理到隔离的 18197。
- 原始反馈循环已在新入口转绿：5175 HTML 可达，`/api/setup/status` 返回 `setup_required: true`、SQLite 408,223,744 字节/69 表；5175 与 18197 保持运行供用户刷新验收。
- 验证通过：launcher 合同测试、`go test ./...`、`go vet ./...`、`pnpm -C web check`（0 error，既有 warnings）、`pnpm -C web build`、`bash -n scripts/dev-setup.sh`、目标 `git diff --check`。
- 浏览器控制受本地 URL 策略阻止，未改用其他浏览器或 CDP 绕过；本轮以同源 HTTP 事实验证启动链路，视觉状态留给用户在已打开的 5175 标签刷新确认。
# 2026-08-27 自动发布元数据与一键部署进度

- 已读取 planning-with-files 工作流并建立本任务计划。
- 已回看上一版 Linux/PostgreSQL 部署约定：预构建镜像、Compose bundle、离线 image bundle，以及外部备份引用门禁。
- 已完成自动发布元数据生成器及 Make/部署/回滚/文档接入；默认无需人工输入版本、构建日期或批次内容。
- 已通过聚焦 Go 契约、Web 部署契约（16/16）、Bash 语法、Make 干跑、真实 Compose bundle、确定性/特殊字符测试与 `make verify` 全量验证。
- `make verify` 在受限沙箱首次因现有 `httptest.NewServer` 无法绑定回环地址失败；以同一命令开放本机回环后通过 Go test/vet、Svelte check/build。
- 已执行 Impeccable 检测，更新后的部署契约测试无 UI 违规；本任务没有渲染组件或交互变化，因此没有新增浏览器状态需要验证。
- 本机没有 Docker CLI；尚不能执行真实镜像构建、体积对比、`docker load` 与 Compose 启动验证。
# 2026-08-28 三项修复进度

- 浏览器反馈：桌面登录失败卡片位移为 0；390px 初测仍有 14.92px 增高/7.46px 上移，已定位为错误文案换行突破 `min-height`，正在改为固定双行槽后复验。
- 工具限制：浏览器页面只读执行层不暴露 Resource Timing，无法从页面直接统计 `/api/login` 条目；请求 single-flight 由源码合同验证，真实浏览器验证聚焦 URL、错误态和几何。
- 命令纠正：手工启动隔离 Vite 时多传一个 `--`，被 Vite 当作位置参数并回退 5174；已停止该进程并按仓库脚本的正确参数形式在 5186 启动。
- 验证发现：`database-setup-contract` 仍硬编码改镜像前的官方 Debian/nginx 地址，与已提交私有基础镜像不一致；产品代码未失败，已把合同收敛为运行阶段与构建工具链隔离语义。
- 工具错误：第二次跨文件补丁因 Makefile 的 tar 列表是多行结构而原子失败，确认部署脚本未被部分修改；随后按 deploy、Makefile、登录逻辑分别应用小补丁。
- 工具错误：首次实现补丁因测试文件断言顺序与补丁上下文不一致而原子失败，确认未产生部分代码改动；已切换为逐文件精确锚点补丁。
- 已加载项目冷启动/设计合同、planning-with-files、diagnosing-bugs、项目 Impeccable、design-taste-frontend 与 finesse-ui 规则。
- 已完成登录失败状态的三方 UI 评审并写入计划；前端编辑门禁已开放，但尚未修改业务代码。
- 已确认 SQLite 宿主路径与 Compose 容器路径映射正确，旧 runtime 配置不随模板演进是首个可执行根因；下一步建立红灯合同。
- 已确认登录提交有 `preventDefault` 且 `/api/login` 不触发全局 401 登出，当前首要复现点转为条件错误块造成的布局回流。
- 已确认一键本地服务应复用 `scripts/dev-setup.sh`，避免引入第二套后端/Vite 生命周期。
# 2026-08-28 验证完成

- 三个独立红灯均已转绿并完成真实运行验证；实现阶段结束，进入一次性交付反思门禁。
- 定向 Go 首次在受限沙箱因既有 `httptest.NewServer` 无法监听回环失败；按权限流程原命令重跑通过，随后全仓 `make verify` 退出码 0。
- Impeccable 检测 `[]`；21/21 定向合同通过；脚本 `bash -n`/ShellCheck、`git diff --check` 通过。
- 真实浏览器桌面/390/320 登录失败卡片几何稳定，320 成功登录进入认证管理台；所有临时服务、浏览器页签、viewport 覆盖和 `/tmp` 数据已清理。

## 2026-08-28 部署登录误报维护修复

- 新增两条红灯合同：TLS 校验失败不得被报告为 maintenance，并必须返回 `wellos_unreachable`；server 运行镜像必须显式复制系统 CA bundle。
- 实施后，5 个登录降级/本地开发认证定向 Go 测试通过，Docker/数据库 setup 合同 16/16 通过；`internal/server` 已无用户可见的 “under maintenance/维护中” 文案。
- 本机没有 Docker CLI，无法直接构建并进入最终 server 镜像检查证书文件；已用静态 Dockerfile 合同和公开端点 TLS 握手分别锁定镜像输入与上游证书有效性，待发布主机完成真实镜像 smoke test。
- 最终回归：5 个登录降级/开发认证定向测试通过，数据库部署合同 16/16 通过，`git diff --check` 通过，全仓 `make verify` 退出码为 0；前端只有仓库既有 Svelte/chunk-size warnings。

## 2026-08-28 SQLite 引导部署验证

- 新红灯准确捕获两个遗漏：非 SQLite 文件仍被 helper 接受；部署脚本没有通过 `/api/setup/status` 证明容器已识别快照。
- 实施后部署/setup 定向合同 21/21 通过，Bash 语法、ShellCheck 和 `git diff --check` 通过。
- 服务器侧预期路径已固定为 `<项目根>/deploy/runtime/data/legacy/well-ambient.db`，对应容器路径 `/var/lib/well-ambient/legacy/well-ambient.db`；等待全仓回归和服务器使用新诊断输出复验。
- 全仓 `make verify` 退出码 0；Go test/vet、Svelte check/build 均通过，只有仓库既有 warning。实现阶段完成，进入一次性交付反思与服务器侧复验。
- 服务器容器证据锁定最终根因：bind-mounted runtime YAML 已更新，但旧 setup 进程未重载。新增 `--force-recreate` 合同准确红灯，修复后部署/setup 合同恢复 21/21；当前服务器可用 `docker restart <server-container>` 立即复验。
