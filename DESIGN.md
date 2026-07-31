# well-ambient Phase 41 Admin UI Design Contract

`well-ambient` 当前前端重构以 Phase 41 原型为准。完整落地规范见 [docs/admin-ui-calibration.md](docs/admin-ui-calibration.md)。

## Active Visual Baseline

- 主基准：`output/functional-admin-console-reference.png`。
- 第一屏结构：深色左轨、浅色主工作区、白色雾面卡片、表格主导、右侧详情栏。
- 品牌信号：左轨和顶部只使用 `well-ambient`，不得继承或放大其他阶段里的旧命名。
- 信息密度：B 端管理台优先，首屏应能同时看到指标、状态分段、主表格和详情栏。
- 材质：只在卡片、顶部栏、详情栏使用克制白色雾面；禁止整页暗色玻璃、舞台剧场、霓虹发光背景和装饰性大渐变。
- 信息架构：该使用子菜单的页面必须把分组子菜单放在全局菜单栏/左轨中，不得放进主 content 区域；右侧内容区保持统一结构，上方为面包屑导航，下方为实际业务内容容器，不再混用分类索引表和独立详情栏作为同级顶层区域。
- 主内容结构：顶部栏之后默认直接进入业务内容。`FunctionalWorkspace` 不应默认渲染模块 hero；页面自身顶部只保留低矮工具条、筛选、视图切换或面包屑。
- UI 会审门禁：任何前端或 UI 改动都必须在执行前同时加载项目 `./.agents/skills/impeccable/SKILL.md`、`design-taste-frontend`（taste-skill）与 `finesse-ui`（finesse-skill），记录三方共同结论并解决分歧；任一技能缺失时不得静默降级执行。

## Superseded Guidance

旧版 Dark Tech / Hack Vibe cockpit 规范只可作为历史实验参考，不再作为生产页面或页面代理的默认设计基准。除非用户明确要求“暗色大屏/战情室”，新页面和改造页面都必须走 Phase 41 浅色管理台规范。

## Shared Contract

页面代理不得在业务组件内重新发明独立视觉系统。优先使用：

- tokens: `web/src/styles/modern-admin-tokens.css`
- shell/workspace: `web/src/components/prototype/FunctionalAdminShell.svelte`、`web/src/components/prototype/FunctionalWorkspace.svelte`
- data contract: `web/src/lib/admin-console/contract.ts`
- CSS classes: `.wa-admin-card`、`.wa-admin-metric`、`.wa-admin-section`、`.wa-admin-table`、`.wa-admin-inspector`、`.wa-admin-pill`、`.wa-admin-action`

数据接入必须保持真实 API 链路：业务页负责 fetch 和权限过滤后的事实映射，共享 UI 只消费页面 adapter 输出的 metric、table row、inspector record，不在组件内写 mock 数据或重新解释后端可见性边界。

## Configuration Center Contract

- 配置中心路由、标签、权限和回退顺序统一由 `web/src/lib/settings-sections.ts` 管理；KPI 只属于顶层度量页，不得作为配置中心子页重复出现。
- 页面层级固定为：紧凑半透明上下文头 -> 单一主工作台 -> 可选版本审计栏。宽屏审计栏位于右侧，中等及窄屏按自然高度流到主工作台下方。
- 毛玻璃只用于上下文头、主工作台、审计栏和弹窗等结构边界；表单字段、配置事实、安全分支和表格保持扁平，以间距和细分隔线组织内容，禁止玻璃卡片嵌套。
- 卡片对齐指外层顶部、列网格、标题基线和操作栏一致，不强制不同内容等高。低于 760px 时使用单列、44px 普通触控控件、全宽操作组和单一表格横向滚动边界；开关保持其紧凑尺寸。
- 统一视觉不得改变真实 API、保存、测试、回滚、用户、权限或策略行为；修改后必须逐页验证全部配置路由及概览/编辑状态，并检查桌面、1024px、760px、480px、降低动效和降低透明度兼容。
