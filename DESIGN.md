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
- UI skill 顺序：前端 UI 改造默认先用 `ui-skill`，不可用时依次回退 `finesse-skill`、`taste-skill`。

## Superseded Guidance

旧版 Dark Tech / Hack Vibe cockpit 规范只可作为历史实验参考，不再作为生产页面或页面代理的默认设计基准。除非用户明确要求“暗色大屏/战情室”，新页面和改造页面都必须走 Phase 41 浅色管理台规范。

## Shared Contract

页面代理不得在业务组件内重新发明独立视觉系统。优先使用：

- tokens: `web/src/styles/modern-admin-tokens.css`
- shell/workspace: `web/src/components/prototype/FunctionalAdminShell.svelte`、`web/src/components/prototype/FunctionalWorkspace.svelte`
- data contract: `web/src/lib/admin-console/contract.ts`
- CSS classes: `.wa-admin-card`、`.wa-admin-metric`、`.wa-admin-section`、`.wa-admin-table`、`.wa-admin-inspector`、`.wa-admin-pill`、`.wa-admin-action`

数据接入必须保持真实 API 链路：业务页负责 fetch 和权限过滤后的事实映射，共享 UI 只消费页面 adapter 输出的 metric、table row、inspector record，不在组件内写 mock 数据或重新解释后端可见性边界。
