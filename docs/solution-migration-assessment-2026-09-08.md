# solution 功能迁移可行性评估

可以将 `/Users/eddie/Workspace/Docs/solution` 的需求分级、证据分析、人工复核、案例积累和交付跟踪流程迁入当前 well-ambient。现有系统提供了较完整的承接基础，但尚未实现这套流程的全部业务规则。建议在现有交付域中新增“交付分级评审”模块，复用需求、Jira、方案版本、语料治理、权限和发布目录。

评估依据为 2026-09-08 的本地文件及当前仓库 `84e143a` 源码。本次只生成分析资料和本报告，未修改业务代码、原始工作簿或 Jira，未连接生产系统。

## 1. 源目录提供了什么

这个目录是一套由规则、人工确认案例、Jira 调研记录、Excel 和按批次编写的处理脚本组成的工作流程。隐藏目录中有读取判定 JSON、写入工作簿并检查结果的脚本；它们绑定具体文件路径和行数，不能直接作为通用后台服务部署。例如，[协同表生成脚本](/Users/eddie/Workspace/Docs/solution/.agent-runtime/work/01a037e4-ca89-7c53-af3e-ac7608188c49/build_workbook.mjs:5) 固定输入文件，并要求恰好 88 条判定。

已完成全部 16 个工作簿的结构读取和 ZIP 完整性检查，发现 15 个不同文件哈希，全部 ZIP 检查通过。多个文件是同一批事项的不同阶段产物，不能将文件行数相加作为待迁移事项总数。

| 资料 | 当前核实结果 | 迁移用途 |
| --- | --- | --- |
| `AGENTS.md` 与分级规则文档 | 定义 P1/P2/P3、证据要求、人工覆盖、评审角色、发布要求和导出格式 | 转成有版本的业务规则和评审策略 |
| `所有任务 2_等级判定0819.xlsx` 与案例 JSONL | 79 条确认案例：P1=43、P2=22、P3=14；逐条标题、等级、说明与 JSONL 一致 | 首批基准案例与回归集 |
| `需求协同表_交付_表格.xlsx` 及标定输出 | 88 条需求；原有业务优先级包含 P0/P1/P2，输出单独增加交付等级及判定说明 | 需求、角色、计划、工时和等级映射样本 |
| 最新 `待评审 (3).xlsx` | `交付版本任务!A1:N32`：31 条，17 Task、14 Bug；29 条有 Jira，2 条无 Jira；C2:C32 等级均为空 | 导入预览和试点评审批次 |
| 历次 Jira 调研 Markdown | 保存机制判断、历史修正和不确定项，反映正文、关联事项及评论参与判断 | 可追溯的历史证据，需带观察日期 |
| 历次已评审输出与脚本 | 包含表格标定、增量追加、说明改写、超链接和结构校验 | 提炼导入/导出契约，不把历史产物都导成新事项 |

0819 工作簿虽有 80 个非空数据行，但第 34 行只有场地 `HIT`，没有事项标题；有效确认案例仍为 79 条。它与输出目录中的“重新评定_直述说明_含Jira超链接”文件哈希相同，导入时应去重。

业务规则来源：[源目录规则](/Users/eddie/Workspace/Docs/solution/AGENTS.md:20)、[确认语料规则](/Users/eddie/Workspace/Docs/solution/.agent-runtime/corpus/fms-delivery-leveling.md:12)。本次提取明细保存在任务工作目录，供复核使用。

## 2. 功能与现有系统的对应关系

“可复用”表示已有源码实现及部分本地测试证据，不代表新流程已经接入或已在线验收。

| 源流程能力 | 现有承接点 | 判断与必要改造 |
| --- | --- | --- |
| 事项、负责人、Jira 身份、需求/缺陷区分 | `TaskTelemetry`、需求接口、任务和需求看板 | 复用主体；补充外部批次行、多 Jira 关联、场地映射，不能将所有事项都转成需求 |
| Jira 正文、优先级、状态和关联键读取 | `JiraClient.SearchIssues`、Jira worker | 已有；增加分级所需 components、自定义验收字段和受控关联项抓取 |
| Jira 评论分页、变更与历史来源 | `GetComments`、`JiraCommentLog`、`SolutionSourceRef` | 已有基础；增加总数/唯一 ID/字段完整性证明、快照水位和失败时的保留策略 |
| P1/P2/P3 机制判定、直接说明 | 当前无专用交付等级模型和判级处理链 | 新增结构化判定、规则版本、案例引用、证据不足状态及人工确认 |
| AI 方案与人工修订保护 | `solutions` 的不可变版本、CAS、候选采纳、提示词版本 | 可复用实现方式；目前提示词用途只有方案润色和两类方案比较，需要独立评审用途 |
| 人工确认案例积累与适用范围 | `CorpusCandidate` → 审核/影响预览 → `ContextFact` | 可复用治理流程；新增结构化判级案例映射及规则检索，明确项目/场地范围 |
| Excel 批次导入、去重、增量追加 | 资料库允许上传 XLSX；`/api/tasks/import` 导入解构任务 JSON | 尚无所需的确定性逐行迁移入口；新增批次预览、列映射、冲突处理和幂等提交 |
| 按等级匹配评审成员、业务/技术评审 | `ReviewContract` 有角色、候选人、最少通过数、SLA | 可复用角色解析思路；需新增交付评审记录、实际表决、日程与条件门禁 |
| 按项目/版本汇总、发布记录 | `ReleaseVersion`、`WorkItemReleaseLink`、发布审计事件 | 可复用；将等级和评审状态聚合进发布就绪检查 |
| 飞书发布审批 | 已有飞书配置、机器人及多维表格相关能力 | 本轮未发现对应交付分级的审批实例、回调和发布门禁；需要单独接入 |
| 场地更新状态、现场更新时间 | 当前发布目录及事项模型提供部分计划事实 | 需要明确场地部署记录；同一版本不同现场的更新时间不能压成版本的一个日期 |
| Excel 标题前缀、说明、超链接、样式保留 | 源目录脚本可作格式样本 | 需要正式导出实现和往返验收；现有系统未发现等价业务导出 |

主要承接证据：[事项模型](/Users/eddie/Workspace/well-ambient/internal/db/db.go:40)、[方案版本与来源模型](/Users/eddie/Workspace/well-ambient/internal/db/solution_assets.go:5)、[发布规划模型](/Users/eddie/Workspace/well-ambient/internal/db/delivery_planning.go:9)、[现有路由](/Users/eddie/Workspace/well-ambient/internal/server/server.go:240)。

## 3. 必须处理的语义与实现缺口

### 3.1 交付风险等级必须独立于业务优先级和缺陷严重度

源目录的 P1/P2/P3 决定评审深度和发布要求。协同表原有 F 列业务优先级与标定输出 W 列交付等级同时存在。现有 `TaskTelemetry.Priority`、`Severity` 分别承担优先级和严重度，Jira worker 会同步 Jira priority，绩效计算也使用 priority。

因此应新增 `delivery_level`，并保留 `business_priority`/现有 `Priority` 和 `Severity`。旧版文件即使把交付等级列命名为“优先级”，也必须通过模板版本和人工确认映射。直接覆盖现有 `Priority` 会混淆业务语义，并可能影响后续同步与绩效结果。[同步映射](/Users/eddie/Workspace/well-ambient/internal/server/jira_worker.go:692)、[绩效等级系数](/Users/eddie/Workspace/well-ambient/internal/performance/scoring.go:299)。

### 3.2 现有“导入资料”不是逐行导入器

资料库把文件交给 LLM 解析为文档和语料，最多保留 24 个候选。79 条基准案例或 88 行需求不能通过这个入口被保证为等量业务记录；超出候选数的部分还会被截断。`/api/tasks/import` 接收的是解构结果 JSON，并非保留 Excel 行身份和样式的导入协议。

应先用确定性解析生成批次和行记录，再决定哪些行创建事项、关联事项或导入案例。AI 负责建议分级，不负责判断是否漏行或决定批次身份。[资料抽取上限](/Users/eddie/Workspace/well-ambient/internal/server/context_document_handlers.go:481)、[任务 JSON 导入](/Users/eddie/Workspace/well-ambient/internal/server/ai_handlers.go:900)。

### 3.3 来源分页能力需要增加“完整”证明

现有 Jira 客户端已有搜索和评论分页，因此无需从头编写连接器。但搜索字段不包含 components 或可配置验收字段；关联事项键的存在也不证明已取得其正文。

`GetComments` 在评论页为空时直接成功返回，即使服务端 `total` 仍大于已读条数。worker 随后以返回 ID 集合标记缺失评论为非当前。这是新评审链需要防护的缺口；本次没有证据表明生产实际触发过。应在声明证据完整前核对 total、页进展和唯一 ID；不完整快照不得用于清退旧证据或自动确认等级。[字段列表](/Users/eddie/Workspace/well-ambient/internal/telemetry/jira_client.go:164)、[分页终止条件](/Users/eddie/Workspace/well-ambient/internal/telemetry/jira_client.go:630)、[评论集合对账](/Users/eddie/Workspace/well-ambient/internal/server/jira_worker.go:1428)。

方案模块还会筛选 `eligible/current` 来源，这适合方案生成，但交付风险评审需要读取完整业务证据，不能只读“方案”标记的评论。[方案来源选择](/Users/eddie/Workspace/well-ambient/internal/solutions/module.go:1373)。

### 3.4 审核契约、方案发布、版本发布是不同动作

`ReviewContract` 当前用于冻结需求规格及后续代码执行。批准契约检查候选人数、最少通过数和验收人后，将契约置为 approved；这不等于已收齐业务、技术等角色的实际意见。

源流程要求：P1 做业务和技术评审，P2 做业务评审，P3 免评审；三类均需走发布飞书审批，P1/P2 的评审有周二/周四安排。现有版本发布实现校验本地归属、日期、发布理由和状态，并记录事件，尚未读取这些交付分级门禁。

需要独立记录评审轮次、角色意见、审批状态和被批准的内容版本，并在 `PublishRelease` 中校验。等级、方案或发布范围改变后应重新判断原批准是否有效。规则中的角色和日程应配置化。[契约批准](/Users/eddie/Workspace/well-ambient/internal/server/autonomous_delivery_handlers.go:419)、[版本发布](/Users/eddie/Workspace/well-ambient/internal/deliveryplanning/release_publish.go:34)。

现有每日 Jira 的“升级协同”只是跟进动作，启用同步时会立即向 Jira 写评论；不能拿它充当本地风险等级确认入口。[每日 Jira 决策](/Users/eddie/Workspace/well-ambient/internal/server/daily_jira_handlers.go:449)。

### 3.5 人工确认权威性和规则范围需要显式保存

0819 的 79 条是源目录明确标识的人工确认基线。后续 AI 输出、文件修改时间或文件名中的“最新”不能自动提升为确认事实。新工作簿出现已填等级时也应保留原值并显示来源、冲突，避免静默覆盖。

建议同时保存 `suggested_level`、`confirmed_level`、`confirmed_by/at`、规则版本、证据快照、修改理由和被替代记录。无确认人资料的历史基线保留 `confirmed_by_user_workbook` 来源，不伪造系统用户或确认时间。涉及 Jstop、锁区等语义不明的事项进入“待补证据/待确认”，不只按关键词升降级。

语料也不能全部落到全局规则。当前 `normalizeContextScope` 只认可 repo/module/demand_type，直接传入 project 会归一成 global。新增项目/场地规则时，应同时完善写入校验、检索和权限，或先使用已支持且语义明确的作用域。[作用域归一](/Users/eddie/Workspace/well-ambient/internal/server/context_handlers.go:581)。

## 4. 建议的模块与数据迁移方式

在 `internal/deliveryreview` 中集中实现分级流程，对调用方提供少量操作，例如 `StageImport`、`Assess`、`Confirm`。Jira 抓取继续由现有连接模块负责，发布状态继续归 `deliveryplanning`，方案正文继续归 `solutions`。不要将这些规则分别写入多个前端组件，也不要另建一份 Jira 主表。

最小数据设计如下，最终表名在实施设计时确定：

| 记录 | 最少应保存的事实 |
| --- | --- |
| 导入批次及行映射 | 文件哈希、模板版本、sheet/row、原始字段、稳定外部身份、目标事项、冲突与处理结果 |
| 分级评估及版本 | 事项、建议/确认等级、机制说明、规则/模型版本、证据快照、关联案例、证据缺口、确认来源和修订关系 |
| 规则与案例 | 适用范围、确认状态、有效版本、出处、可替代关系；接入现有语料治理 |
| 交付评审与发布审批 | 业务/技术评审、角色意见、被批准内容版本、飞书实例和状态、重评原因 |
| 现场部署记录 | 版本、场地、计划/实际更新时间、更新状态、操作者；与全局发布状态区分 |

关键字段映射：

| Excel 字段 | 建议归属与约束 |
| --- | --- |
| 描述/需求名、背景、需求描述 | 原始文本保留；建模为事项标题及描述。标题等级前缀由展示/导出生成，避免重复叠加 |
| JIRA编号/jira链接 | 解析为关联集合；一个单元格多键不拆成重复需求；同一键不同场地/版本也不自动合并 |
| 需求编号 | 有效时作为外部稳定身份；仍需记录来源，不能假设跨表全局唯一 |
| 场地/配置现场 | 先映射项目和场地目录；空值、多场地、未知名称进入异常清单 |
| 等级/交付等级/旧模板“优先级” | 经模板映射进入独立交付等级；协同表原 F 列仍为业务优先级 |
| 责任人、开发、测试、项目经理 | 映射现有用户及角色；多人或未识别姓名不得压缩为一个随意选出的 assignee |
| 方案/研发评审状态、评审日期 | 交付评审事实；历史状态导入不直接冒充当前角色审批 |
| 发布的版本号、发布状态、版本发布时间 | 关联本地版本目录；保留原文和来源，导入记录不触发实际发布 |
| 现场更新时间/状态 | 版本与场地对应的部署记录 |
| 工时、复杂度、计划日期 | 保留源单位和角色维度；人天与小时不默认换算，复杂度不代替交付等级 |
| 判定说明、备注 | 说明与一般备注分开；判定保持“P#｜责任对象＋机制＋影响”，证据 URL 独立保存 |

导入采用“预览 → 解析冲突 → 提交”流程。文件哈希用于识别同一批次重放；行号用于溯源，不能作为跨文件稳定身份。无 Jira、标题变化、多 Jira 或同 Jira 多场地的条目，使用来源身份和人工映射处理，禁止仅靠标题模糊相似就合并。

历史基线先导入为案例，是否创建或绑定业务事项另行映射，避免把 79 个历史样本直接变成 79 条在办任务。迁移首批可使用最新 31 条待评审事项；本次不对它们重新判级。

## 5. 按可验收的阶段落地

| 阶段 | 交付结果 | 通过标准 |
| --- | --- | --- |
| A：基础与基准案例 | 独立等级模型、79 条确认案例、规则版本、批次预览及身份映射 | 79 条无遗漏且与原表一致；重复文件重放不重复；原优先级和人工确认值不变 |
| B：评审闭环 | 完整 Jira 证据、AI 建议、人工确认、修订历史；在现有工作台接入 | 最新 31 条完整进入待评审，2 条无 Jira 有明确状态；缺证据不能冒充确认；并发确认检测旧版本；新增状态通过权限与浏览器验证 |
| C：交付与导出 | 按等级的业务/技术评审、飞书审批、版本门禁、现场更新及 Excel 导出 | 未满足门禁不能发布；重复回调/超时不重复执行；P3 仍有发布审批；导出只产生一个等级前缀并保留原字段、任务顺序和链接 |

阶段 A/B 可完成分级工作的系统化；阶段 C 完成后，才具备替代当前 Excel 交付审批与现场跟踪流程的条件。当前没有实施排期、外部审批定义和生产字段清单，不能据此给出可靠工期或宣称“配置提示词即可上线”。

验收还应覆盖：同名不同机制的分级案例、人工降级后 AI 重跑、旧证据失效、关联项抓取失败、评论总数不完整、含多 Jira 的行、只含场地的空事项行、旧版模板列名歧义、受限项目数据访问、Excel 修改后往返导入和回滚。

AI 复评可以用 79 条作为回归对照并报告逐条差异；机器结果与基线不同应进入复核，不能为了获得一致率覆盖人工事实。已经用作示例的案例也不能同时用来证明模型对未见事项的泛化准确率。

## 6. 验证范围与剩余依赖

- 读取全部 16 个 XLSX 的 sheet、有效范围、表头、数据行、等级标记、超链接、公式及文件哈希；ZIP 无损坏，源文件未改写。
- 对 79 条 JSONL 逐行比对 0819 工作簿的标题、等级、判定说明：零差异。
- `internal/solutions`、`internal/solutioncatalog`、`internal/delivery`、`internal/deliveryplanning` 四个包现有测试通过。
- 4 项定向测试通过：`TestJiraGetCommentsPaginatesWithoutDroppingEdits`、`TestCorpusCandidatesRequireReviewBeforeContextPromotion`、`TestContextDocumentImportAndGovernedPublication`、`TestArchiveContextDocumentWithdrawsUnpublishedCandidates`。首次运行因沙箱禁止回环端口监听中断，允许本机测试监听后通过；使用临时数据库与模拟 HTTP 服务，不代表真实 Jira 或生产模型验证。
- 本次未运行服务、访问业务数据库、调用真实 Jira/飞书/LLM、改动界面或开展认证浏览器验收。源码未发现的业务能力表述为本轮核查缺口，不据此断言外部系统绝不存在相应实现。

实施前需补齐的外部资料是：实际 Jira 验收/组件等字段配置、项目与场地映射、审批角色和飞书流程定义、历史表中人工确认来源及冲突处理决定、现网数据库和模型配置。可行性结论不依赖先取得这些信息；完整上线验收依赖它们。
