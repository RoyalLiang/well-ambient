<script lang="ts">
  type Risk = 'high' | 'medium' | 'low';
  type Tone = 'danger' | 'warning' | 'success' | 'info';

  interface NavItem {
    label: string;
    mark: string;
    route?: string;
  }

  interface Metric {
    title: string;
    value: string;
    unit: string;
    delta: string;
    tone: Tone;
    legend: Array<{ label: string; value: string; tone: Tone }>;
  }

  interface TaskRow {
    id: string;
    title: string;
    subtitle: string;
    scope: string;
    risk: Risk;
    evidence: number;
    owner: string;
    next: string;
    due: string;
  }

  interface TimelineItem {
    time: string;
    title: string;
    detail: string;
    tone: Tone;
  }

  const navItems: NavItem[] = [
    { label: '总览', mark: '⌂' },
    { label: '交付治理', mark: '◇' },
    { label: '交付风险', mark: '△' },
    { label: '证据链', mark: '□' },
    { label: '周会决策', mark: '▣' },
    { label: '任务与跟踪', mark: '▤' },
    { label: '变更管理', mark: '◉' },
    { label: '度量与洞察', mark: '▥' },
    { label: '知识库', mark: '▭' },
    { label: '配置中心', mark: '⚙', route: 'settings' }
  ];

  const metrics: Metric[] = [
    {
      title: '交付风险',
      value: '7',
      unit: '个高风险',
      delta: '较昨日 +2',
      tone: 'danger',
      legend: [
        { label: '高风险', value: '7', tone: 'danger' },
        { label: '中风险', value: '12', tone: 'warning' },
        { label: '低风险', value: '24', tone: 'success' }
      ]
    },
    {
      title: '证据链',
      value: '92',
      unit: '% 完整率',
      delta: '较昨日 +6%',
      tone: 'success',
      legend: [
        { label: '完整', value: '248', tone: 'success' },
        { label: '缺失', value: '18', tone: 'warning' },
        { label: '过期', value: '7', tone: 'danger' }
      ]
    },
    {
      title: '周会决策',
      value: '5',
      unit: '项待决策',
      delta: '本周新增 2',
      tone: 'info',
      legend: [
        { label: '已决策', value: '18', tone: 'success' },
        { label: '待决策', value: '5', tone: 'warning' },
        { label: '已延期', value: '1', tone: 'danger' }
      ]
    }
  ];

  const tasks: TaskRow[] = [
    {
      id: 'RISK-202505-0017',
      title: '智能客服升级项目',
      subtitle: 'v2.3 版本交付',
      scope: '客服平台',
      risk: 'high',
      evidence: 86,
      owner: '李思远',
      next: '完成压力测试报告',
      due: '2025-05-16'
    },
    {
      id: 'RISK-202505-0021',
      title: '数据中台治理',
      subtitle: '数据质量提升',
      scope: '数据中台',
      risk: 'medium',
      evidence: 78,
      owner: '王若曦',
      next: '补充血缘证据',
      due: '2025-05-20'
    },
    {
      id: 'RISK-202505-0024',
      title: '订单系统重构',
      subtitle: '订单核心稳定重构',
      scope: '订单中心',
      risk: 'high',
      evidence: 91,
      owner: '陈宇航',
      next: '评审接口变更影响',
      due: '2025-05-18'
    },
    {
      id: 'RISK-202505-0028',
      title: '风控策略优化',
      subtitle: '模型效果提升',
      scope: '风控平台',
      risk: 'medium',
      evidence: 95,
      owner: '刘梓涵',
      next: '补充离线评估报告',
      due: '2025-05-22'
    },
    {
      id: 'RISK-202505-0032',
      title: '基础设施升级',
      subtitle: '资源与容灾优化',
      scope: '基础架构',
      risk: 'low',
      evidence: 100,
      owner: '赵天宇',
      next: '执行变更计划',
      due: '2025-05-25'
    }
  ];

  const timeline: TimelineItem[] = [
    { time: '今天 09:42', title: '风险状态更新为高风险', detail: '由李思远更新', tone: 'danger' },
    { time: '昨天 18:21', title: '风险状态更新为中风险', detail: '由王若曦更新', tone: 'warning' },
    { time: '05-11 10:15', title: '风险识别', detail: '由系统识别', tone: 'success' }
  ];

  const riskLabel: Record<Risk, string> = {
    high: '高',
    medium: '中',
    low: '低'
  };

  const riskToneLabel: Record<Risk, string> = {
    high: '高风险',
    medium: '中风险',
    low: '低风险'
  };

  let activeNav = '总览';
  let selectedId = tasks[0].id;
  export let currentUserName = '张明远';
  export let currentUserRole = '交付负责人';
  export let alertCount = 6;
  export let onNavigate: (tab: string) => void = () => {};

  $: selectedTask = tasks.find((task) => task.id === selectedId) || tasks[0];
  $: displayName = currentUserName || '用户';
  $: roleLabel = currentUserRole === 'super_admin'
    ? '超级管理员'
    : currentUserRole === 'admin'
      ? '系统管理员'
      : currentUserRole || '交付负责人';
  $: avatarMark = displayName.slice(0, 1).toUpperCase();

  function selectTask(id: string) {
    selectedId = id;
  }

  function selectNav(item: NavItem) {
    activeNav = item.label;
    if (item.route) {
      onNavigate(item.route);
    }
  }
</script>

<svelte:head>
  <title>well-ambient</title>
</svelte:head>

<main class="prototype-screen">
  <aside class="side-rail" aria-label="主导航">
    <div class="brand-lockup">
      <div class="brand-symbol" aria-hidden="true"></div>
      <div>
        <strong>well-ambient</strong>
        <small>Well Ambient</small>
      </div>
    </div>

    <nav class="nav-list" aria-label="管理台模块">
      {#each navItems as item}
        <button
          type="button"
          class:active={activeNav === item.label}
          aria-current={activeNav === item.label ? 'page' : undefined}
          onclick={() => selectNav(item)}
        >
          <span aria-hidden="true">{item.mark}</span>
          {item.label}
        </button>
      {/each}
    </nav>

    <section class="system-status" aria-label="系统状态">
      <span aria-hidden="true"></span>
      <div>
        <small>系统状态</small>
        <strong>正常运行</strong>
      </div>
      <i aria-hidden="true"></i>
    </section>

    <button type="button" class="collapse-nav">
      <span aria-hidden="true"></span>
      收起导航
    </button>
  </aside>

  <section class="main-shell">
    <header class="topbar">
      <h1>总览</h1>

      <label class="search-control">
        <span aria-hidden="true"></span>
        <input type="search" placeholder="搜索任务、风险、证据链..." />
        <kbd>⌘K</kbd>
      </label>

      <div class="top-actions">
        <button type="button" class="new-button">
          <span aria-hidden="true">＋</span>
          新建
          <i aria-hidden="true"></i>
        </button>
        <button type="button" class="bell-button" aria-label="通知">
          <span aria-hidden="true"></span>
          {#if alertCount > 0}
            <em>{alertCount > 99 ? '99+' : alertCount}</em>
          {/if}
        </button>
        <button type="button" class="help-button" aria-label="帮助">?</button>
        <button type="button" class="profile-button">
          <span aria-hidden="true">{avatarMark}</span>
          <div>
            <strong>{displayName}</strong>
            <small>{roleLabel}</small>
          </div>
          <i aria-hidden="true"></i>
        </button>
      </div>
    </header>

    <div class="content-layout">
      <section class="left-column">
        <section class="focus-panel" aria-label="今日焦点">
          <h2>今日焦点</h2>
          <div class="metric-grid">
            {#each metrics as metric}
              <article class={`metric-card metric-${metric.tone}`}>
                <header>
                  <h3>{metric.title}</h3>
                  <button type="button" aria-label={`${metric.title}详情`}></button>
                </header>

                <div class="metric-body">
                  <div>
                    <strong>{metric.value}</strong>
                    <span>{metric.unit}</span>
                  </div>
                  <svg viewBox="0 0 150 56" role="img" aria-label={`${metric.title}趋势`}>
                    <path class="trend-fill" d="M0 51 C18 48 24 33 43 36 C58 39 64 22 81 25 C99 29 106 19 121 21 C134 22 140 9 150 7 L150 56 L0 56 Z" />
                    <path class="trend-line" d="M0 51 C18 48 24 33 43 36 C58 39 64 22 81 25 C99 29 106 19 121 21 C134 22 140 9 150 7" />
                  </svg>
                </div>

                <p>{metric.delta}</p>

                <footer>
                  {#each metric.legend as item}
                    <span class={`legend-dot legend-${item.tone}`}>
                      {item.label} {item.value}
                    </span>
                  {/each}
                </footer>
              </article>
            {/each}
          </div>
        </section>

        <section class="task-panel" aria-label="关键任务">
          <div class="panel-head">
            <h2>关键任务</h2>
            <div>
              <button type="button">全部任务</button>
              <button type="button" aria-label="筛选任务"></button>
            </div>
          </div>

          <div class="task-table" role="table" aria-label="关键任务列表">
            <div class="table-head" role="row">
              <span role="columnheader">任务</span>
              <span role="columnheader">交付范围</span>
              <span role="columnheader">交付风险</span>
              <span role="columnheader">证据链完整率</span>
              <span role="columnheader">负责人</span>
              <span role="columnheader">下一步</span>
              <span role="columnheader">目标完成</span>
            </div>

            {#each tasks as task}
              <button
                type="button"
                role="row"
                class="table-row"
                class:selected={selectedId === task.id}
                onclick={() => selectTask(task.id)}
              >
                <span class="task-name" role="cell">
                  <i aria-hidden="true"></i>
                  <span>
                    <strong>{task.title}</strong>
                    <small>{task.subtitle}</small>
                  </span>
                </span>
                <span role="cell">{task.scope}</span>
                <span role="cell">
                  <em class={`risk-pill risk-${task.risk}`}>{riskLabel[task.risk]}</em>
                </span>
                <span role="cell">
                  <strong class={`evidence evidence-${task.risk}`}>{task.evidence}%</strong>
                </span>
                <span role="cell" class="owner-cell">
                  <i aria-hidden="true">{task.owner.slice(0, 1)}</i>
                  {task.owner}
                </span>
                <span role="cell">{task.next}</span>
                <span role="cell" class:risk-date={task.risk !== 'low'}>{task.due}</span>
                <span class="row-menu" aria-hidden="true"></span>
              </button>
            {/each}
          </div>

          <footer class="table-footer">
            <span>共 28 条</span>
            <button type="button">10 条 / 页</button>
            <nav aria-label="分页">
              <button type="button" aria-label="上一页"></button>
              <button type="button" class="active">1</button>
              <button type="button">2</button>
              <button type="button">3</button>
              <button type="button" aria-label="下一页"></button>
            </nav>
          </footer>
        </section>
      </section>

      <aside class="inspector-panel" aria-label="风险详情">
        <header class="inspector-tabs">
          <button type="button" class="active">风险详情</button>
          <button type="button">证据链</button>
          <button type="button">决策记录</button>
          <button type="button" aria-label="关闭"></button>
        </header>

        <section class="inspector-body">
          <span class={`risk-chip risk-${selectedTask.risk}`}>{riskToneLabel[selectedTask.risk]}</span>
          <div class="inspector-title">
            <div>
              <h2>{selectedTask.title}</h2>
              <p>{selectedTask.subtitle}</p>
            </div>
            <small>ID {selectedTask.id}</small>
          </div>

          <dl class="details">
            <div>
              <dt>识别时间:</dt>
              <dd>2025-05-13 09:42</dd>
            </div>
            <div>
              <dt>负责人:</dt>
              <dd>{selectedTask.owner}</dd>
            </div>
            <div>
              <dt>影响范围</dt>
              <dd>{selectedTask.scope} / 用户端 / 工单系统</dd>
            </div>
            <div>
              <dt>风险原因</dt>
              <dd>压测指标未达标，峰值响应时间超阈值 30%</dd>
            </div>
            <div>
              <dt>当前状态</dt>
              <dd>分析中</dd>
            </div>
            <div>
              <dt>关联任务</dt>
              <dd><a href="/prototype.html">{selectedTask.title}</a></dd>
            </div>
            <div>
              <dt>风险等级</dt>
              <dd><span class={`detail-dot detail-${selectedTask.risk}`}></span>{riskLabel[selectedTask.risk]}</dd>
            </div>
            <div>
              <dt>应对策略</dt>
              <dd>优化服务降级策略，扩容资源并复测</dd>
            </div>
          </dl>

          <section class="risk-feed" aria-label="风险动态">
            <h3>风险动态</h3>
            {#each timeline as item}
              <article class={`feed-item feed-${item.tone}`}>
                <time>{item.time}</time>
                <div>
                  <strong>{item.title}</strong>
                  <p>{item.detail}</p>
                </div>
              </article>
            {/each}
          </section>
        </section>

        <footer class="inspector-actions">
          <button type="button">标记为已处理</button>
          <button type="button">制定应对计划</button>
        </footer>
      </aside>
    </div>
  </section>
</main>

<style>
  :global(html),
  :global(body),
  :global(#prototype) {
    min-width: 0;
    min-height: 100%;
    margin: 0;
  }

  :global(#prototype) {
    background: #eef3f6;
    color: #14211d;
    font-family:
      Inter,
      "SF Pro Display",
      "PingFang SC",
      "Microsoft YaHei",
      system-ui,
      sans-serif;
  }

  :global(*) {
    box-sizing: border-box;
  }

  button,
  input {
    font: inherit;
  }

  button {
    border: 0;
  }

  .prototype-screen {
    --rail: #07131c;
    --rail-2: #0c1b25;
    --page: #f3f7f8;
    --panel: rgba(255, 255, 255, 0.82);
    --panel-solid: #ffffff;
    --ink: #16221f;
    --muted: #6f7b80;
    --subtle: #9aa5aa;
    --line: rgba(23, 36, 42, 0.1);
    --line-strong: rgba(23, 36, 42, 0.16);
    --green: #17b978;
    --teal: #1fa8b8;
    --red: #ef3f46;
    --orange: #f59e0b;
    --radius-card: 12px;
    --radius-panel: 10px;
    --shadow-panel: 0 14px 36px rgba(36, 51, 57, 0.08);
    --shadow-card: 0 16px 38px rgba(36, 51, 57, 0.09);
    min-height: 100dvh;
    display: grid;
    grid-template-columns: 220px minmax(0, 1fr);
    overflow: hidden;
    color: var(--ink);
    font-family:
      Inter,
      "SF Pro Display",
      "PingFang SC",
      "Microsoft YaHei",
      system-ui,
      sans-serif;
    background:
      radial-gradient(circle at 54% 4%, rgba(255, 255, 255, 0.95), transparent 34%),
      linear-gradient(180deg, #edf4f7 0%, #f7fafb 42%, #edf4f6 100%);
  }

  .side-rail {
    min-width: 0;
    min-height: 100dvh;
    display: flex;
    flex-direction: column;
    gap: 22px;
    padding: 28px 10px 20px;
    background:
      linear-gradient(180deg, rgba(17, 35, 47, 0.92), rgba(5, 16, 25, 0.98)),
      var(--rail);
    color: #dcecf0;
    box-shadow: 16px 0 38px rgba(17, 31, 38, 0.18);
  }

  .brand-lockup {
    display: grid;
    grid-template-columns: 42px minmax(0, 1fr);
    align-items: center;
    gap: 10px;
    padding: 0 8px;
  }

  .brand-symbol {
    width: 38px;
    height: 38px;
    border: 2px solid rgba(36, 225, 203, 0.9);
    border-radius: 50% 50% 42% 42%;
    border-bottom-color: transparent;
    filter: drop-shadow(0 0 12px rgba(36, 225, 203, 0.28));
  }

  .brand-lockup strong {
    display: block;
    overflow: hidden;
    color: #ffffff;
    font-size: 18px;
    font-weight: 840;
    letter-spacing: 0;
    line-height: 1.08;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .brand-lockup small {
    display: block;
    margin-top: 4px;
    overflow: hidden;
    color: rgba(220, 236, 240, 0.58);
    font-size: 11px;
    font-weight: 680;
    letter-spacing: 0;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .nav-list {
    display: grid;
    gap: 10px;
  }

  .nav-list button {
    width: 100%;
    min-height: 56px;
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr);
    align-items: center;
    gap: 12px;
    padding: 0 20px;
    border-radius: 8px;
    background: transparent;
    color: rgba(220, 236, 240, 0.78);
    cursor: pointer;
    font-size: 14px;
    font-weight: 720;
    text-align: left;
  }

  .nav-list button.active {
    background:
      linear-gradient(90deg, rgba(23, 185, 120, 0.24), rgba(255, 255, 255, 0.07)),
      rgba(255, 255, 255, 0.06);
    box-shadow: inset 3px 0 0 var(--green);
    color: #ffffff;
  }

  .nav-list span {
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border: 1px solid rgba(220, 236, 240, 0.24);
    border-radius: 7px;
    color: rgba(53, 223, 196, 0.95);
    font-size: 13px;
    line-height: 1;
  }

  .system-status {
    min-height: 56px;
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr) 12px;
    align-items: center;
    gap: 10px;
    margin: auto 2px 0;
    padding: 10px;
    border: 1px solid rgba(220, 236, 240, 0.16);
    border-radius: 9px;
    background: rgba(255, 255, 255, 0.055);
  }

  .system-status > span {
    width: 28px;
    height: 28px;
    border-radius: 9px;
    background: linear-gradient(145deg, #24dc91, #11a768);
  }

  .system-status small,
  .system-status strong {
    display: block;
  }

  .system-status small {
    color: rgba(220, 236, 240, 0.66);
    font-size: 11px;
    line-height: 1.1;
  }

  .system-status strong {
    margin-top: 2px;
    color: #ffffff;
    font-size: 13px;
    font-weight: 780;
  }

  .system-status i,
  .collapse-nav span,
  .new-button i,
  .profile-button i {
    width: 8px;
    height: 8px;
    border-top: 1.5px solid currentColor;
    border-right: 1.5px solid currentColor;
    rotate: 45deg;
  }

  .collapse-nav {
    min-height: 50px;
    display: grid;
    grid-template-columns: 26px minmax(0, 1fr);
    align-items: center;
    gap: 10px;
    margin-inline: 2px;
    padding: 0 20px;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.08);
    color: rgba(220, 236, 240, 0.8);
    cursor: pointer;
    text-align: left;
  }

  .collapse-nav span {
    rotate: 225deg;
  }

  .main-shell {
    min-width: 0;
    min-height: 100dvh;
    overflow: hidden;
  }

  .topbar {
    min-width: 0;
    height: 92px;
    display: grid;
    grid-template-columns: 210px minmax(420px, 640px) minmax(0, 1fr);
    align-items: center;
    gap: 24px;
    padding: 0 18px 0 34px;
    border-bottom: 1px solid rgba(23, 36, 42, 0.08);
    background: rgba(255, 255, 255, 0.54);
    backdrop-filter: blur(22px) saturate(160%);
    -webkit-backdrop-filter: blur(22px) saturate(160%);
  }

  .topbar h1 {
    margin: 0;
    color: var(--ink);
    font-size: 26px;
    font-weight: 850;
    line-height: 1;
    letter-spacing: 0;
  }

  .search-control {
    min-width: 0;
    height: 42px;
    display: grid;
    grid-template-columns: 18px minmax(0, 1fr) auto;
    align-items: center;
    gap: 12px;
    padding: 0 14px;
    border: 1px solid rgba(23, 36, 42, 0.12);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.7);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78);
  }

  .search-control > span {
    width: 15px;
    height: 15px;
    border: 1.8px solid #8d9aa0;
    border-radius: 50%;
    position: relative;
  }

  .search-control > span::after {
    content: "";
    position: absolute;
    right: -5px;
    bottom: -3px;
    width: 7px;
    height: 2px;
    border-radius: 999px;
    background: #8d9aa0;
    rotate: 45deg;
  }

  .search-control input {
    min-width: 0;
    width: 100%;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--ink);
    font-size: 13px;
  }

  .search-control input::placeholder {
    color: #9aa5aa;
  }

  .search-control kbd {
    min-width: 32px;
    min-height: 22px;
    display: grid;
    place-items: center;
    border: 1px solid rgba(23, 36, 42, 0.12);
    border-radius: 5px;
    background: rgba(247, 249, 250, 0.86);
    color: #8a969c;
    font-size: 12px;
    font-family: inherit;
  }

  .top-actions {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 14px;
  }

  .new-button,
  .bell-button,
  .help-button,
  .profile-button,
  .metric-card header button,
  .panel-head button,
  .table-footer button,
  .inspector-tabs button,
  .inspector-actions button {
    cursor: pointer;
  }

  .new-button {
    height: 42px;
    display: grid;
    grid-template-columns: 18px auto 8px;
    align-items: center;
    gap: 8px;
    padding: 0 14px;
    border: 1px solid rgba(23, 36, 42, 0.1);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.68);
    color: #526066;
    font-size: 13px;
    font-weight: 720;
  }

  .new-button > span {
    width: 18px;
    height: 18px;
    display: grid;
    place-items: center;
    border: 1px solid rgba(82, 96, 102, 0.42);
    border-radius: 50%;
    font-size: 13px;
  }

  .new-button i,
  .profile-button i {
    color: #8b979c;
    rotate: 135deg;
  }

  .bell-button,
  .help-button {
    position: relative;
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    background: transparent;
    color: #7a878d;
  }

  .bell-button span {
    width: 15px;
    height: 17px;
    border: 1.8px solid currentColor;
    border-radius: 10px 10px 5px 5px;
  }

  .bell-button em {
    position: absolute;
    top: 0;
    right: 3px;
    width: 16px;
    height: 16px;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--red);
    color: #fff;
    font-size: 10px;
    font-style: normal;
    font-weight: 820;
  }

  .help-button {
    border: 1px solid rgba(23, 36, 42, 0.2);
    border-radius: 50%;
    font-size: 15px;
    font-weight: 760;
  }

  .profile-button {
    min-width: 166px;
    height: 50px;
    display: grid;
    grid-template-columns: 38px minmax(0, 1fr) 8px;
    align-items: center;
    gap: 10px;
    background: transparent;
    color: var(--ink);
    text-align: left;
  }

  .profile-button > span {
    width: 38px;
    height: 38px;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: #15221f;
    color: #ffffff;
    font-weight: 840;
  }

  .profile-button strong,
  .profile-button small {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .profile-button strong {
    font-size: 13px;
    font-weight: 780;
  }

  .profile-button small {
    margin-top: 2px;
    color: #7d888e;
    font-size: 12px;
  }

  .content-layout {
    height: calc(100dvh - 92px);
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(720px, 1fr) 396px;
    gap: 14px;
    padding: 16px 18px 18px;
    overflow: hidden;
  }

  .left-column {
    min-width: 0;
    display: grid;
    grid-template-rows: 316px minmax(0, 1fr);
    gap: 14px;
  }

  .focus-panel,
  .task-panel,
  .inspector-panel {
    min-width: 0;
    border: 1px solid rgba(23, 36, 42, 0.09);
    border-radius: var(--radius-card);
    background: var(--panel);
    box-shadow: var(--shadow-panel);
    backdrop-filter: blur(20px) saturate(150%);
    -webkit-backdrop-filter: blur(20px) saturate(150%);
  }

  .focus-panel {
    padding: 26px 18px 22px;
  }

  .focus-panel h2,
  .panel-head h2,
  .risk-feed h3 {
    margin: 0;
    color: var(--ink);
    font-weight: 820;
    letter-spacing: 0;
  }

  .focus-panel h2,
  .panel-head h2 {
    font-size: 22px;
    line-height: 1.1;
  }

  .metric-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 20px;
    margin-top: 22px;
  }

  .metric-card {
    min-width: 0;
    height: 218px;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto auto;
    gap: 10px;
    padding: 22px 22px 17px;
    border: 1px solid rgba(23, 36, 42, 0.07);
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.78);
    box-shadow: var(--shadow-card);
  }

  .metric-card header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .metric-card h3 {
    margin: 0;
    color: var(--ink);
    font-size: 15px;
    font-weight: 780;
  }

  .metric-card header button {
    width: 30px;
    height: 30px;
    border: 1px solid rgba(23, 36, 42, 0.12);
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.7);
    position: relative;
  }

  .metric-card header button::before {
    content: "";
    position: absolute;
    inset: 10px 12px 10px 10px;
    border-top: 1.6px solid #7d8a90;
    border-right: 1.6px solid #7d8a90;
    rotate: 45deg;
  }

  .metric-body {
    display: grid;
    grid-template-columns: minmax(96px, 0.9fr) minmax(110px, 1fr);
    align-items: center;
    gap: 14px;
    min-height: 78px;
  }

  .metric-body > div {
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  .metric-body strong {
    font-size: 48px;
    font-weight: 860;
    line-height: 0.9;
    letter-spacing: 0;
  }

  .metric-body span {
    min-width: 0;
    color: #6d7b81;
    font-size: 13px;
    font-weight: 720;
    line-height: 1.35;
  }

  .metric-danger .metric-body strong,
  .metric-danger p {
    color: var(--red);
  }

  .metric-success .metric-body strong,
  .metric-success p {
    color: var(--green);
  }

  .metric-info .metric-body strong,
  .metric-info p {
    color: var(--teal);
  }

  .metric-danger svg {
    color: var(--red);
  }

  .metric-success svg {
    color: var(--green);
  }

  .metric-info svg {
    color: var(--teal);
  }

  .metric-card svg {
    width: 100%;
    height: 56px;
    overflow: visible;
  }

  .trend-line {
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-width: 2;
  }

  .trend-fill {
    fill: currentColor;
    opacity: 0.08;
  }

  .metric-card p {
    margin: 0;
    font-size: 13px;
    font-weight: 740;
  }

  .metric-card footer {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    padding-top: 14px;
    border-top: 1px solid rgba(23, 36, 42, 0.08);
  }

  .legend-dot {
    min-width: 0;
    overflow: hidden;
    color: #6f7b80;
    font-size: 11px;
    font-weight: 680;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .legend-dot::before {
    content: "";
    width: 6px;
    height: 6px;
    display: inline-block;
    margin-right: 6px;
    border-radius: 50%;
    vertical-align: 1px;
    background: currentColor;
  }

  .legend-danger {
    color: var(--red);
  }

  .legend-warning {
    color: var(--orange);
  }

  .legend-success {
    color: var(--green);
  }

  .task-panel {
    min-height: 0;
    padding: 22px 20px 18px;
    overflow: hidden;
  }

  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    margin-bottom: 14px;
  }

  .panel-head > div {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .panel-head button:first-child {
    min-width: 118px;
    height: 42px;
    padding: 0 14px;
    border: 1px solid rgba(23, 36, 42, 0.1);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.72);
    color: #65737a;
    font-size: 13px;
    font-weight: 720;
    text-align: left;
  }

  .panel-head button:last-child {
    width: 42px;
    height: 42px;
    border: 1px solid rgba(23, 36, 42, 0.1);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.72);
    position: relative;
  }

  .panel-head button:last-child::before,
  .panel-head button:last-child::after {
    content: "";
    position: absolute;
    left: 12px;
    right: 12px;
    height: 2px;
    border-radius: 999px;
    background: #65737a;
  }

  .panel-head button:last-child::before {
    top: 14px;
    box-shadow: 6px 0 0 -2px #65737a;
  }

  .panel-head button:last-child::after {
    bottom: 14px;
    box-shadow: -6px 0 0 -2px #65737a;
  }

  .task-table {
    min-width: 0;
    overflow: hidden;
    border: 1px solid rgba(23, 36, 42, 0.09);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.52);
  }

  .table-head,
  .table-row {
    display: grid;
    grid-template-columns: minmax(190px, 1.35fr) minmax(92px, 0.62fr) minmax(90px, 0.62fr) minmax(118px, 0.72fr) minmax(104px, 0.7fr) minmax(146px, 0.95fr) minmax(108px, 0.68fr) 24px;
    align-items: center;
    gap: 14px;
    padding-inline: 16px;
  }

  .table-head {
    height: 42px;
    border-bottom: 1px solid rgba(23, 36, 42, 0.08);
    color: #6f7b80;
    font-size: 12px;
    font-weight: 760;
  }

  .table-row {
    width: 100%;
    height: 62px;
    border-bottom: 1px solid rgba(23, 36, 42, 0.075);
    background: transparent;
    color: var(--ink);
    font-size: 13px;
    text-align: left;
  }

  .table-row:last-child {
    border-bottom: 0;
  }

  .table-row.selected {
    background: rgba(23, 185, 120, 0.07);
  }

  .table-row:hover {
    background: rgba(23, 185, 120, 0.05);
  }

  .table-row > span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .task-name {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    align-items: center;
    gap: 10px;
  }

  .task-name > i {
    width: 26px;
    height: 26px;
    border: 1px solid rgba(23, 185, 120, 0.22);
    border-radius: 8px;
    background: rgba(23, 185, 120, 0.12);
  }

  .task-name strong,
  .task-name small {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .task-name strong {
    color: #26332f;
    font-size: 13px;
    font-weight: 790;
  }

  .task-name small {
    margin-top: 2px;
    color: #99a4a9;
    font-size: 11px;
    font-weight: 620;
  }

  .risk-pill,
  .risk-chip {
    display: inline-grid;
    place-items: center;
    border-radius: 5px;
    font-style: normal;
    font-weight: 760;
  }

  .risk-pill {
    min-width: 34px;
    height: 24px;
    font-size: 12px;
  }

  .risk-chip {
    width: max-content;
    height: 26px;
    padding: 0 12px;
    font-size: 12px;
  }

  .risk-high {
    background: rgba(239, 63, 70, 0.11);
    color: var(--red);
  }

  .risk-medium {
    background: rgba(245, 158, 11, 0.13);
    color: #dd8600;
  }

  .risk-low {
    background: rgba(23, 185, 120, 0.11);
    color: #0d9d61;
  }

  .evidence {
    font-weight: 790;
  }

  .evidence-high,
  .evidence-low {
    color: var(--green);
  }

  .evidence-medium {
    color: var(--orange);
  }

  .owner-cell {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .owner-cell i {
    width: 22px;
    height: 22px;
    display: inline-grid;
    flex: none;
    place-items: center;
    border-radius: 50%;
    background: #edf3f1;
    color: #586661;
    font-size: 11px;
    font-style: normal;
    font-weight: 760;
  }

  .risk-date {
    color: var(--red);
    font-weight: 720;
  }

  .row-menu {
    width: 18px;
    height: 18px;
    position: relative;
  }

  .row-menu::before {
    content: "";
    position: absolute;
    left: 8px;
    top: 3px;
    width: 3px;
    height: 3px;
    border-radius: 50%;
    background: #1f2b2f;
    box-shadow: 0 6px 0 #1f2b2f, 0 12px 0 #1f2b2f;
  }

  .table-footer {
    min-height: 48px;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 14px;
    padding-top: 16px;
    color: #7d888e;
    font-size: 13px;
  }

  .table-footer > button,
  .table-footer nav button {
    height: 34px;
    border: 1px solid rgba(23, 36, 42, 0.09);
    border-radius: 7px;
    background: rgba(255, 255, 255, 0.66);
    color: #77848a;
    font-size: 13px;
  }

  .table-footer > button {
    min-width: 102px;
    padding: 0 12px;
  }

  .table-footer nav {
    display: flex;
    gap: 8px;
  }

  .table-footer nav button {
    width: 34px;
    padding: 0;
    font-weight: 720;
  }

  .table-footer nav button.active {
    border-color: rgba(23, 185, 120, 0.34);
    background: rgba(23, 185, 120, 0.1);
    color: #0d9d61;
  }

  .table-footer nav button[aria-label] {
    position: relative;
  }

  .table-footer nav button[aria-label]::before {
    content: "";
    position: absolute;
    inset: 12px 11px 11px 13px;
    border-top: 1.5px solid currentColor;
    border-left: 1.5px solid currentColor;
    rotate: -45deg;
  }

  .table-footer nav button[aria-label="下一页"]::before {
    inset: 12px 13px 11px 11px;
    rotate: 135deg;
  }

  .inspector-panel {
    min-height: 0;
    display: grid;
    grid-template-rows: 64px minmax(0, 1fr) 84px;
    overflow: hidden;
  }

  .inspector-tabs {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr 34px;
    align-items: stretch;
    border-bottom: 1px solid rgba(23, 36, 42, 0.08);
  }

  .inspector-tabs button {
    position: relative;
    background: transparent;
    color: #717d83;
    font-size: 14px;
    font-weight: 740;
  }

  .inspector-tabs button.active {
    color: var(--ink);
  }

  .inspector-tabs button.active::after {
    content: "";
    position: absolute;
    left: 20px;
    right: 20px;
    bottom: 0;
    height: 3px;
    border-radius: 999px 999px 0 0;
    background: var(--green);
  }

  .inspector-tabs button[aria-label="关闭"]::before,
  .inspector-tabs button[aria-label="关闭"]::after {
    content: "";
    position: absolute;
    left: 12px;
    top: 31px;
    width: 13px;
    height: 1.5px;
    border-radius: 999px;
    background: #6e7a80;
  }

  .inspector-tabs button[aria-label="关闭"]::before {
    rotate: 45deg;
  }

  .inspector-tabs button[aria-label="关闭"]::after {
    rotate: -45deg;
  }

  .inspector-body {
    min-height: 0;
    overflow: auto;
    padding: 24px 22px 18px;
  }

  .inspector-title {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 14px;
    align-items: end;
    margin-top: 20px;
  }

  .inspector-title h2 {
    margin: 0;
    color: var(--ink);
    font-size: 24px;
    font-weight: 840;
    line-height: 1.18;
    letter-spacing: 0;
  }

  .inspector-title p,
  .inspector-title small {
    margin: 6px 0 0;
    color: #7c878d;
    font-size: 12px;
    font-weight: 680;
  }

  .details {
    display: grid;
    gap: 0;
    margin: 22px 0 0;
    padding: 18px 0 20px;
    border-top: 1px solid rgba(23, 36, 42, 0.08);
    border-bottom: 1px solid rgba(23, 36, 42, 0.08);
  }

  .details div {
    min-width: 0;
    display: grid;
    grid-template-columns: 78px minmax(0, 1fr);
    gap: 16px;
    padding: 9px 0;
  }

  .details dt,
  .details dd {
    min-width: 0;
    margin: 0;
    font-size: 13px;
    line-height: 1.48;
  }

  .details dt {
    color: #7d888e;
    font-weight: 760;
  }

  .details dd {
    color: #38474c;
    font-weight: 660;
    overflow-wrap: anywhere;
  }

  .details a {
    color: var(--teal);
    text-decoration: none;
  }

  .detail-dot {
    width: 7px;
    height: 7px;
    display: inline-block;
    margin-right: 8px;
    border-radius: 50%;
    vertical-align: 1px;
  }

  .detail-high {
    background: var(--red);
  }

  .detail-medium {
    background: var(--orange);
  }

  .detail-low {
    background: var(--green);
  }

  .risk-feed {
    padding-top: 22px;
  }

  .risk-feed h3 {
    font-size: 16px;
  }

  .feed-item {
    position: relative;
    display: grid;
    grid-template-columns: 74px minmax(0, 1fr);
    gap: 12px;
    margin-top: 14px;
    padding-left: 15px;
  }

  .feed-item::before {
    content: "";
    position: absolute;
    left: 1px;
    top: 8px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: currentColor;
  }

  .feed-danger {
    color: var(--red);
  }

  .feed-warning {
    color: var(--orange);
  }

  .feed-success {
    color: var(--green);
  }

  .feed-item time {
    color: #7c878d;
    font-size: 12px;
    line-height: 1.4;
  }

  .feed-item div {
    min-width: 0;
    padding: 12px 14px;
    border: 1px solid rgba(23, 36, 42, 0.08);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.56);
  }

  .feed-item strong {
    display: block;
    color: #3a484d;
    font-size: 13px;
    line-height: 1.34;
  }

  .feed-item p {
    margin: 4px 0 0;
    color: #7d888e;
    font-size: 12px;
  }

  .inspector-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 14px;
    align-items: center;
    padding: 18px 22px;
    border-top: 1px solid rgba(23, 36, 42, 0.08);
    background: rgba(255, 255, 255, 0.58);
  }

  .inspector-actions button {
    height: 42px;
    border-radius: 7px;
    font-size: 13px;
    font-weight: 760;
  }

  .inspector-actions button:first-child {
    border: 1px solid rgba(23, 36, 42, 0.12);
    background: rgba(255, 255, 255, 0.72);
    color: #58656b;
  }

  .inspector-actions button:last-child {
    background: var(--green);
    color: #ffffff;
    box-shadow: 0 16px 30px rgba(23, 185, 120, 0.22);
  }

  @media (max-width: 1380px) {
    .prototype-screen {
      grid-template-columns: 210px minmax(0, 1fr);
    }

    .topbar {
      grid-template-columns: 160px minmax(300px, 1fr) auto;
      padding-left: 26px;
    }

    .content-layout {
      grid-template-columns: 1fr;
      overflow: auto;
    }

    .left-column {
      grid-template-rows: auto auto;
    }

    .inspector-panel {
      min-height: 620px;
    }
  }

  @media (max-width: 980px) {
    .prototype-screen {
      display: block;
      overflow: auto;
    }

    .side-rail {
      min-height: auto;
      padding: 18px;
    }

    .nav-list {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .system-status,
    .collapse-nav {
      display: none;
    }

    .main-shell {
      min-height: auto;
      overflow: visible;
    }

    .topbar {
      height: auto;
      grid-template-columns: 1fr;
      padding: 18px;
    }

    .top-actions {
      justify-content: flex-start;
      flex-wrap: wrap;
    }

    .content-layout {
      height: auto;
      padding: 14px;
    }

    .metric-grid {
      grid-template-columns: 1fr;
    }

    .focus-panel {
      padding: 20px 14px;
    }

    .task-panel {
      overflow-x: auto;
    }

    .task-table {
      min-width: 900px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    *,
    *::before,
    *::after {
      scroll-behavior: auto !important;
      transition-duration: 0.001ms !important;
    }
  }
</style>
