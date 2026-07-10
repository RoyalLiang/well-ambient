<script lang="ts">
  import FunctionalAdminShell from './FunctionalAdminShell.svelte';
  import FunctionalWorkspace from './FunctionalWorkspace.svelte';

  type RouteKey = 'decision' | 'schedule' | 'evidence' | 'tasks' | 'kpi' | 'settings';
  type WorkspaceTone = 'cyan' | 'green' | 'amber' | 'rose' | 'violet' | 'slate';
  type SignalTone = 'neutral' | 'good' | 'warn' | 'danger' | 'info';

  interface PreviewMetric {
    label: string;
    value: string;
    delta: string;
    tone: SignalTone;
  }

  interface PreviewRow {
    id: string;
    title: string;
    owner: string;
    state: string;
    evidence: string;
    risk: string;
    tone: SignalTone;
  }

  interface PreviewStep {
    label: string;
    value: string;
  }

  interface PreviewAction {
    label: string;
    route: RouteKey;
    kind?: 'primary' | 'secondary';
  }

  interface PreviewModule {
    kicker: string;
    title: string;
    summary: string;
    statusLabel: string;
    tone: WorkspaceTone;
    focus: string;
    command: string;
    metrics: PreviewMetric[];
    rows: PreviewRow[];
    steps: PreviewStep[];
    signals: { label: string; value: string; tone?: SignalTone }[];
    actions: PreviewAction[];
    detail: {
      title: string;
      owner: string;
      risk: number;
      verdict: string;
      evidence: string[];
    };
  }

  const routes: RouteKey[] = ['decision', 'schedule', 'evidence', 'tasks', 'kpi', 'settings'];

  const modules: Record<RouteKey, PreviewModule> = {
    decision: {
      kicker: 'DECISION BOARD',
      title: '决策看板',
      summary: '把会议、阻塞、人工介入和执行证据放在同一个决策面里，优先处理会拖慢交付的事项。',
      statusLabel: 'Agenda 已接入',
      tone: 'cyan',
      focus: '今日需要裁决 4 项',
      command: '按阻塞影响面排序',
      metrics: [
        { label: '待裁决', value: '4', delta: '2 项影响排期', tone: 'warn' },
        { label: '已关闭', value: '17', delta: '本周', tone: 'good' },
        { label: '无证据项', value: '1', delta: '需补充', tone: 'danger' }
      ],
      rows: [
        { id: 'DEC-1024', title: '支付回调幂等策略', owner: 'Eddie', state: '待裁决', evidence: 'MR + Jira', risk: '高', tone: 'danger' },
        { id: 'DEC-1021', title: '客户配置灰度窗口', owner: 'Luna', state: '需补证', evidence: 'Jira', risk: '中', tone: 'warn' },
        { id: 'DEC-1018', title: '外部协同权限边界', owner: 'Kai', state: '已同步', evidence: 'API Test', risk: '低', tone: 'good' }
      ],
      steps: [
        { label: '收敛事实', value: '证据链完整性 86%' },
        { label: '确认影响', value: '2 个项目受影响' },
        { label: '落地动作', value: '同步到排期治理' }
      ],
      signals: [
        { label: '会话状态', value: '正常', tone: 'good' },
        { label: '未读遥测', value: '2', tone: 'warn' },
        { label: '当前身份', value: '管理员', tone: 'info' }
      ],
      actions: [
        { label: '查看排期', route: 'schedule', kind: 'primary' },
        { label: '打开证据', route: 'evidence' }
      ],
      detail: {
        title: '支付回调幂等策略',
        owner: 'Eddie',
        risk: 82,
        verdict: '需要在今日站会前裁决，否则会阻塞联调窗口。',
        evidence: ['Jira 需求包含验收口径', 'MR 已关联核心修改', '测试环境缺少回放数据']
      }
    },
    schedule: {
      kicker: 'SCHEDULE CONTROL',
      title: '排期治理',
      summary: '围绕 Jira Task 主线管理需求池、负责人、截止日期和风险日历，减少视图跳转与人工比对。',
      statusLabel: 'Jira 需求同步',
      tone: 'green',
      focus: '本周 31 个需求在线',
      command: '风险日历优先',
      metrics: [
        { label: '准时率', value: '91%', delta: '+6%', tone: 'good' },
        { label: '高风险', value: '5', delta: '需跟进', tone: 'warn' },
        { label: '无人负责', value: '0', delta: '已收敛', tone: 'good' }
      ],
      rows: [
        { id: 'JIRA-891', title: '客户标签批量编辑', owner: 'Mina', state: '研发中', evidence: '3 commits', risk: '中', tone: 'warn' },
        { id: 'JIRA-876', title: '数据权限回归验证', owner: 'Kai', state: '测试中', evidence: 'CI pass', risk: '低', tone: 'good' },
        { id: 'JIRA-862', title: '报表导出队列治理', owner: 'Noah', state: '待联调', evidence: 'MR open', risk: '高', tone: 'danger' }
      ],
      steps: [
        { label: '需求入池', value: 'Jira Task 自动归集' },
        { label: '风险定位', value: '日历与负责人同步' },
        { label: '执行闭环', value: '任务面板追踪' }
      ],
      signals: [
        { label: '会话状态', value: '正常', tone: 'good' },
        { label: '排期变更', value: '6', tone: 'info' },
        { label: '高风险', value: '5', tone: 'warn' }
      ],
      actions: [
        { label: '跟踪任务', route: 'tasks', kind: 'primary' },
        { label: '查看证据', route: 'evidence' }
      ],
      detail: {
        title: '报表导出队列治理',
        owner: 'Noah',
        risk: 76,
        verdict: '队列限流策略未确认，建议先锁定验收口径再推进联调。',
        evidence: ['Jira 截止日期为本周五', 'MR 已打开但缺少压测数据', '风险日历出现交付重叠']
      }
    },
    evidence: {
      kicker: 'EVIDENCE GRAPH',
      title: '证据链',
      summary: '把项目健康、AI 解构、仓库映射和 shadow task 汇总成可追溯事实，辅助排期与绩效判断。',
      statusLabel: 'Telemetry 已聚合',
      tone: 'amber',
      focus: '证据完整度 88%',
      command: '优先补齐弱证据',
      metrics: [
        { label: '证据完整度', value: '88%', delta: '+4%', tone: 'good' },
        { label: '弱关联', value: '7', delta: '待确认', tone: 'warn' },
        { label: '孤立提交', value: '3', delta: '需映射', tone: 'danger' }
      ],
      rows: [
        { id: 'EV-442', title: '核心成员可见性边界', owner: 'Kai', state: '已验证', evidence: 'API Test', risk: '低', tone: 'good' },
        { id: 'EV-438', title: '排期视图滚动稳定性', owner: 'Mina', state: '观察中', evidence: 'Browser', risk: '中', tone: 'warn' },
        { id: 'EV-431', title: 'KPI 权重配置', owner: 'Luna', state: '待补证', evidence: 'Config', risk: '高', tone: 'danger' }
      ],
      steps: [
        { label: '采集', value: 'Jira GitLab CI' },
        { label: '归因', value: '任务与提交映射' },
        { label: '解释', value: 'AI 解构摘要' }
      ],
      signals: [
        { label: '会话状态', value: '正常', tone: 'good' },
        { label: '证据缺口', value: '7', tone: 'warn' },
        { label: '健康样本', value: '42', tone: 'info' }
      ],
      actions: [
        { label: '回到排期', route: 'schedule', kind: 'primary' },
        { label: '打开任务', route: 'tasks' }
      ],
      detail: {
        title: 'KPI 权重配置',
        owner: 'Luna',
        risk: 68,
        verdict: '配置变更有记录，但缺少对应业务目标，需要补齐决策来源。',
        evidence: ['配置中心存在变更记录', '缺少关联议题', '日报预览未覆盖本指标']
      }
    },
    tasks: {
      kicker: 'EXECUTION LOOP',
      title: '任务跟踪',
      summary: '把状态、负责人、执行风险、Jira、GitLab、MR 和 commit 证据聚合到同一个执行闭环。',
      statusLabel: '执行面板在线',
      tone: 'rose',
      focus: '9 项任务需要确认',
      command: '按交付阻塞排序',
      metrics: [
        { label: '进行中', value: '18', delta: '跨 4 组', tone: 'info' },
        { label: '阻塞', value: '3', delta: '需介入', tone: 'danger' },
        { label: '待验收', value: '11', delta: '本周', tone: 'good' }
      ],
      rows: [
        { id: 'TASK-712', title: '数据权限回归补测', owner: 'Kai', state: '测试中', evidence: 'CI pass', risk: '低', tone: 'good' },
        { id: 'TASK-709', title: '导出队列压测脚本', owner: 'Noah', state: '阻塞', evidence: 'MR open', risk: '高', tone: 'danger' },
        { id: 'TASK-701', title: '风险日历交互优化', owner: 'Mina', state: '研发中', evidence: 'commit', risk: '中', tone: 'warn' }
      ],
      steps: [
        { label: '认领', value: '负责人明确' },
        { label: '执行', value: '证据持续进入' },
        { label: '验收', value: '风险回写排期' }
      ],
      signals: [
        { label: '会话状态', value: '正常', tone: 'good' },
        { label: '阻塞任务', value: '3', tone: 'danger' },
        { label: 'MR 待审', value: '8', tone: 'warn' }
      ],
      actions: [
        { label: '查看 KPI', route: 'kpi', kind: 'primary' },
        { label: '同步排期', route: 'schedule' }
      ],
      detail: {
        title: '导出队列压测脚本',
        owner: 'Noah',
        risk: 88,
        verdict: '压测脚本阻塞验收，建议转为手动介入并补齐数据样本。',
        evidence: ['MR 已打开 2 天', 'CI 通过但未覆盖压测', '需求截止日接近']
      }
    },
    kpi: {
      kicker: 'PERFORMANCE FACTS',
      title: '度量洞察',
      summary: '把个人、部门、风险和证据沉淀为可追溯事实，避免只用主观印象解释交付质量。',
      statusLabel: '日报周报可预览',
      tone: 'violet',
      focus: '4 个指标偏离阈值',
      command: '展示事实来源',
      metrics: [
        { label: '交付质量', value: '94', delta: '+3', tone: 'good' },
        { label: '风险响应', value: '81', delta: '-5', tone: 'warn' },
        { label: '证据覆盖', value: '88%', delta: '+4%', tone: 'info' }
      ],
      rows: [
        { id: 'KPI-118', title: '交付准时率', owner: '研发一组', state: '健康', evidence: 'Schedule', risk: '低', tone: 'good' },
        { id: 'KPI-114', title: '风险响应时长', owner: '研发二组', state: '关注', evidence: 'Alerts', risk: '中', tone: 'warn' },
        { id: 'KPI-109', title: '证据缺口率', owner: '平台组', state: '偏离', evidence: 'Telemetry', risk: '高', tone: 'danger' }
      ],
      steps: [
        { label: '指标', value: '权重来自配置中心' },
        { label: '证据', value: '来自任务与遥测' },
        { label: '解释', value: '日报周报可追溯' }
      ],
      signals: [
        { label: '会话状态', value: '正常', tone: 'good' },
        { label: '偏离指标', value: '4', tone: 'warn' },
        { label: '报表状态', value: '可预览', tone: 'info' }
      ],
      actions: [
        { label: '调整配置', route: 'settings', kind: 'primary' },
        { label: '查看任务', route: 'tasks' }
      ],
      detail: {
        title: '风险响应时长',
        owner: '研发二组',
        risk: 61,
        verdict: '风险响应慢于阈值，但证据显示主要来自等待决策。',
        evidence: ['3 个阻塞项来自决策看板', '本周风险响应均值 18 小时', '配置中心阈值为 12 小时']
      }
    },
    settings: {
      kicker: 'CONTROL CENTER',
      title: '配置中心',
      summary: '把权限、项目、集成、KPI 权重和 AI 策略集中管理，确保业务面板只展示可执行配置。',
      statusLabel: '权限受控',
      tone: 'slate',
      focus: '配置变更 6 项',
      command: '按影响范围审阅',
      metrics: [
        { label: '集成状态', value: '5/5', delta: '在线', tone: 'good' },
        { label: '待审配置', value: '2', delta: '需确认', tone: 'warn' },
        { label: '权限异常', value: '0', delta: '正常', tone: 'good' }
      ],
      rows: [
        { id: 'CFG-221', title: 'KPI 权重调整', owner: 'Admin', state: '待审', evidence: 'Audit', risk: '中', tone: 'warn' },
        { id: 'CFG-216', title: 'Jira 项目映射', owner: 'Admin', state: '在线', evidence: 'Sync', risk: '低', tone: 'good' },
        { id: 'CFG-209', title: 'AI 解构策略', owner: 'Admin', state: '观察中', evidence: 'Prompt', risk: '中', tone: 'warn' }
      ],
      steps: [
        { label: '权限', value: '成员与项目边界' },
        { label: '集成', value: 'Jira GitLab WellOS' },
        { label: '策略', value: 'KPI 与 AI 规则' }
      ],
      signals: [
        { label: '会话状态', value: '正常', tone: 'good' },
        { label: '待审配置', value: '2', tone: 'warn' },
        { label: '权限异常', value: '0', tone: 'good' }
      ],
      actions: [
        { label: '看 KPI', route: 'kpi', kind: 'primary' },
        { label: '回到决策', route: 'decision' }
      ],
      detail: {
        title: 'KPI 权重调整',
        owner: 'Admin',
        risk: 58,
        verdict: '建议在周报窗口前确认，避免指标解释与历史数据不一致。',
        evidence: ['审计记录存在', '影响 3 个部门报表', '当前变更未发布']
      }
    }
  };

  const previewAlerts = [
    {
      id: 1,
      type: 'delay',
      task_id: 'JIRA-862',
      title: '报表导出队列存在延期风险',
      assignee: 'Noah',
      delay_days: 2,
      severity: 'high',
      status: 'unread',
      message: '压测脚本尚未补齐，联调窗口可能被压缩。',
      link: '',
      created_at: new Date(Date.now() - 1000 * 60 * 32).toISOString()
    },
    {
      id: 2,
      type: 'mr_event',
      task_id: 'TASK-709',
      title: 'MR 等待补充证据',
      assignee: 'Noah',
      delay_days: 0,
      severity: 'medium',
      status: 'unread',
      message: '代码已提交，缺少压测结果与验收样本。',
      link: '',
      created_at: new Date(Date.now() - 1000 * 60 * 94).toISOString()
    }
  ];

  let activeRoute: RouteKey = 'schedule';

  $: activeModule = modules[activeRoute];

  function navigate(route: string) {
    if (routes.includes(route as RouteKey)) {
      activeRoute = route as RouteKey;
    }
  }
</script>

<FunctionalAdminShell
  {activeRoute}
  availableRoutes={routes}
  currentUserName="Well Admin"
  currentUserEmail="admin@well-ambient.local"
  currentUserRole="admin"
  currentUserDepartment="Product Operations"
  currentUserPermissions={['config:read', 'schedule:read', 'tasks:read', 'kpi:read']}
  alertCount={previewAlerts.length}
  alerts={previewAlerts}
  onNavigate={navigate}
>
  <FunctionalWorkspace
    kicker={activeModule.kicker}
    title={activeModule.title}
    summary={activeModule.summary}
    statusLabel={activeModule.statusLabel}
    tone={activeModule.tone}
    signals={activeModule.signals}
    actions={activeModule.actions}
    onNavigate={navigate}
  >
    <section class="preview-board" aria-label="{activeModule.title} 原型工作区">
      <div class="command-column">
        <div class="command-surface">
          <div class="surface-head">
            <div>
              <span>PRIMARY FOCUS</span>
              <strong>{activeModule.focus}</strong>
            </div>
            <button type="button">{activeModule.command}</button>
          </div>

          <div class="metric-grid">
            {#each activeModule.metrics as metric}
              <article class="metric-card metric-{metric.tone}">
                <span>{metric.label}</span>
                <strong>{metric.value}</strong>
                <small>{metric.delta}</small>
              </article>
            {/each}
          </div>

          <div class="flow-strip" aria-label="当前工作流">
            {#each activeModule.steps as step, index}
              <div class="flow-step">
                <em>{String(index + 1).padStart(2, '0')}</em>
                <span>{step.label}</span>
                <strong>{step.value}</strong>
              </div>
            {/each}
          </div>
        </div>

        <div class="table-surface">
          <div class="table-head">
            <div>
              <span>WORK ITEMS</span>
              <strong>高价值事项队列</strong>
            </div>
            <div class="view-switch" aria-label="视图密度">
              <button type="button" class="active">Dense</button>
              <button type="button">Focus</button>
            </div>
          </div>

          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>编号</th>
                  <th>事项</th>
                  <th>负责人</th>
                  <th>状态</th>
                  <th>证据</th>
                  <th>风险</th>
                </tr>
              </thead>
              <tbody>
                {#each activeModule.rows as row}
                  <tr class:hot={row.tone === 'danger'}>
                    <td>{row.id}</td>
                    <td><strong>{row.title}</strong></td>
                    <td>{row.owner}</td>
                    <td>{row.state}</td>
                    <td>{row.evidence}</td>
                    <td><span class="risk-pill risk-{row.tone}">{row.risk}</span></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <aside class="detail-panel" aria-label="事项详情">
        <div class="detail-top">
          <span>INSPECTOR</span>
          <strong>{activeModule.detail.title}</strong>
          <p>{activeModule.detail.verdict}</p>
        </div>

        <div class="risk-meter">
          <div class="meter-head">
            <span>风险强度</span>
            <strong>{activeModule.detail.risk}%</strong>
          </div>
          <div class="meter-track">
            <span style={`width: ${activeModule.detail.risk}%`}></span>
          </div>
        </div>

        <dl class="detail-meta">
          <div>
            <dt>Owner</dt>
            <dd>{activeModule.detail.owner}</dd>
          </div>
          <div>
            <dt>Module</dt>
            <dd>{activeModule.title}</dd>
          </div>
        </dl>

        <div class="evidence-stack">
          <span>Evidence</span>
          {#each activeModule.detail.evidence as item}
            <p>{item}</p>
          {/each}
        </div>

        <div class="detail-actions">
          <button type="button" class="primary">打开工作流</button>
          <button type="button">标记已处理</button>
        </div>
      </aside>
    </section>
  </FunctionalWorkspace>
</FunctionalAdminShell>

<style>
  .preview-board {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 0.34fr);
    gap: 14px;
    min-width: 0;
  }

  .command-column {
    display: grid;
    gap: 14px;
    min-width: 0;
  }

  .command-surface,
  .table-surface,
  .detail-panel {
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background:
      linear-gradient(145deg, rgba(244, 251, 255, 0.064), rgba(244, 251, 255, 0.018)),
      rgba(9, 14, 20, 0.74);
    box-shadow: var(--wa-shadow-panel);
    backdrop-filter: blur(22px) saturate(132%);
    -webkit-backdrop-filter: blur(22px) saturate(132%);
  }

  .command-surface {
    display: grid;
    gap: 16px;
    padding: 16px;
  }

  .surface-head,
  .table-head,
  .meter-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .surface-head div,
  .table-head div,
  .detail-top {
    min-width: 0;
  }

  .surface-head span,
  .table-head span,
  .detail-top span,
  .evidence-stack > span {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 0.68rem;
    font-weight: 840;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .surface-head strong,
  .table-head strong,
  .detail-top strong {
    display: block;
    margin-top: 4px;
    color: var(--wa-text-strong);
    font-size: 1rem;
    line-height: 1.25;
  }

  button {
    min-height: 36px;
    border: 1px solid rgba(203, 234, 244, 0.16);
    border-radius: var(--wa-radius-md);
    background: rgba(244, 251, 255, 0.052);
    color: var(--wa-text-main);
    padding: 0 12px;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 760;
    cursor: pointer;
    white-space: nowrap;
    transition: transform 160ms var(--wa-ease), border-color 160ms var(--wa-ease), background 160ms var(--wa-ease);
  }

  button:hover {
    transform: translateY(-1px);
    border-color: var(--wa-border-strong);
    background: rgba(244, 251, 255, 0.085);
  }

  button:focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  button.primary {
    border-color: color-mix(in srgb, var(--wa-accent) 54%, transparent);
    background: var(--wa-accent);
    color: var(--wa-accent-ink);
  }

  .metric-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .metric-card {
    min-height: 112px;
    display: grid;
    align-content: end;
    gap: 7px;
    padding: 14px;
    border: 1px solid rgba(203, 234, 244, 0.12);
    border-radius: var(--wa-radius-lg);
    background:
      radial-gradient(circle at 88% 12%, rgba(244, 251, 255, 0.075), transparent 7rem),
      rgba(7, 11, 16, 0.5);
  }

  .metric-card span,
  .metric-card small {
    color: var(--wa-text-muted);
    font-size: 0.74rem;
  }

  .metric-card strong {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: clamp(1.55rem, 2vw, 2.25rem);
    line-height: 0.95;
    letter-spacing: 0;
  }

  .metric-good strong {
    color: var(--wa-success);
  }

  .metric-warn strong {
    color: var(--wa-warning);
  }

  .metric-danger strong {
    color: var(--wa-danger);
  }

  .metric-info strong {
    color: var(--wa-info);
  }

  .flow-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .flow-step {
    min-height: 78px;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 12px;
    border-radius: var(--wa-radius-lg);
    background: rgba(244, 251, 255, 0.045);
  }

  .flow-step em {
    color: var(--wa-accent);
    font-family: var(--wa-font-mono);
    font-size: 0.68rem;
    font-style: normal;
    font-weight: 900;
  }

  .flow-step span {
    color: var(--wa-text-strong);
    font-size: 0.83rem;
    font-weight: 780;
  }

  .flow-step strong {
    color: var(--wa-text-muted);
    font-size: 0.76rem;
    font-weight: 620;
    line-height: 1.35;
  }

  .table-surface {
    overflow: hidden;
  }

  .table-head {
    padding: 15px 16px;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .view-switch {
    display: inline-flex;
    gap: 4px;
    padding: 4px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 12px;
    background: rgba(7, 11, 16, 0.46);
  }

  .view-switch button {
    min-height: 28px;
    border-radius: 8px;
    border-color: transparent;
    background: transparent;
    padding: 0 9px;
  }

  .view-switch button.active {
    background: rgba(244, 251, 255, 0.1);
    color: var(--wa-text-strong);
  }

  .table-wrap {
    overflow-x: auto;
  }

  table {
    width: 100%;
    min-width: 760px;
    border-collapse: collapse;
  }

  th,
  td {
    padding: 13px 16px;
    border-bottom: 1px solid rgba(203, 234, 244, 0.09);
    color: var(--wa-text-main);
    font-size: 0.82rem;
    text-align: left;
    white-space: nowrap;
  }

  th {
    color: var(--wa-text-subtle);
    font-size: 0.68rem;
    font-weight: 850;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }

  td strong {
    color: var(--wa-text-strong);
    font-size: 0.84rem;
  }

  tbody tr {
    transition: background 160ms var(--wa-ease);
  }

  tbody tr:hover {
    background: var(--wa-row-hover);
  }

  tbody tr.hot {
    background: rgba(255, 97, 119, 0.06);
  }

  .risk-pill {
    min-width: 34px;
    display: inline-flex;
    justify-content: center;
    padding: 5px 8px;
    border-radius: 999px;
    border: 1px solid rgba(203, 234, 244, 0.12);
    background: rgba(244, 251, 255, 0.052);
    color: var(--wa-text-main);
    font-size: 0.72rem;
    font-weight: 780;
  }

  .risk-good {
    color: var(--wa-success);
  }

  .risk-warn {
    color: var(--wa-warning);
  }

  .risk-danger {
    color: var(--wa-danger);
  }

  .detail-panel {
    min-width: 0;
    padding: 16px;
    display: grid;
    align-content: start;
    gap: 16px;
  }

  .detail-top p {
    margin: 10px 0 0;
    color: var(--wa-text-muted);
    font-size: 0.84rem;
    line-height: 1.65;
  }

  .risk-meter {
    display: grid;
    gap: 10px;
  }

  .meter-head span {
    color: var(--wa-text-muted);
    font-size: 0.76rem;
  }

  .meter-head strong {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
  }

  .meter-track {
    height: 9px;
    overflow: hidden;
    border-radius: 999px;
    background: rgba(244, 251, 255, 0.08);
  }

  .meter-track span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, var(--wa-success), var(--wa-warning), var(--wa-danger));
  }

  .detail-meta {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
    margin: 0;
  }

  .detail-meta div {
    min-height: 70px;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 12px;
    border-radius: var(--wa-radius-lg);
    background: rgba(244, 251, 255, 0.045);
  }

  .detail-meta dt {
    color: var(--wa-text-subtle);
    font-size: 0.7rem;
  }

  .detail-meta dd {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 0.84rem;
    font-weight: 780;
  }

  .evidence-stack {
    display: grid;
    gap: 8px;
  }

  .evidence-stack p {
    margin: 0;
    padding: 10px 12px;
    border-left: 2px solid var(--wa-accent);
    border-radius: 0 var(--wa-radius-md) var(--wa-radius-md) 0;
    background: rgba(244, 251, 255, 0.045);
    color: var(--wa-text-main);
    font-size: 0.8rem;
    line-height: 1.45;
  }

  .detail-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  @media (max-width: 1220px) {
    .preview-board {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 780px) {
    .metric-grid,
    .flow-strip,
    .detail-meta,
    .detail-actions {
      grid-template-columns: 1fr;
    }

    .surface-head,
    .table-head {
      align-items: stretch;
      flex-direction: column;
    }

    .surface-head button,
    .view-switch {
      width: 100%;
    }

    .view-switch button {
      flex: 1;
    }
  }

  /* Phase 41 reference alignment: light workbench, table-first, no dark theatre surfaces. */
  .preview-board {
    grid-template-columns: minmax(0, 1fr) minmax(318px, 0.34fr);
    gap: 16px;
    color: var(--wa-text-main);
  }

  .command-column {
    gap: 12px;
  }

  .command-surface {
    padding: 0;
    border: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .surface-head {
    display: none;
  }

  .metric-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .metric-card {
    min-height: 104px;
    align-content: space-between;
    padding: 16px;
    border-color: var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background: rgba(255, 255, 255, 0.9);
    box-shadow: var(--wa-shadow-panel);
  }

  .metric-card span,
  .metric-card small {
    color: var(--wa-text-muted);
    font-size: 13px;
  }

  .metric-card strong {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-sans);
    font-size: 2.15rem;
    font-weight: 850;
    letter-spacing: 0;
  }

  .metric-good strong {
    color: var(--wa-success);
  }

  .metric-warn strong {
    color: var(--wa-warning);
  }

  .metric-danger strong {
    color: var(--wa-danger);
  }

  .metric-info strong {
    color: var(--wa-info);
  }

  .flow-strip {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .flow-step {
    min-height: 74px;
    padding: 12px 14px;
    border: 1px solid var(--wa-border-soft);
    border-top: 3px solid var(--wa-accent);
    border-radius: var(--wa-radius-xl);
    background: rgba(255, 255, 255, 0.82);
    box-shadow: none;
  }

  .flow-step em {
    color: var(--wa-accent-strong);
    font-family: var(--wa-font-sans);
    letter-spacing: 0;
  }

  .flow-step span {
    color: var(--wa-text-strong);
  }

  .flow-step strong {
    color: var(--wa-text-muted);
  }

  .table-surface,
  .detail-panel {
    border-color: var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background: rgba(255, 255, 255, 0.9);
    box-shadow: var(--wa-shadow-panel);
    backdrop-filter: blur(16px) saturate(124%);
    -webkit-backdrop-filter: blur(16px) saturate(124%);
  }

  .table-head {
    min-height: 58px;
    padding: 12px 16px;
    border-bottom-color: var(--wa-border-soft);
  }

  .surface-head span,
  .table-head span,
  .detail-top span,
  .evidence-stack > span {
    color: var(--wa-text-muted);
    font-family: var(--wa-font-sans);
    font-size: 0.78rem;
    letter-spacing: 0;
    text-transform: none;
  }

  .table-head strong,
  .detail-top strong {
    color: var(--wa-text-strong);
    font-size: 1rem;
  }

  button {
    border-color: var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.78);
    color: var(--wa-text-main);
    transition:
      border-color var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease),
      box-shadow var(--wa-duration-fast) var(--wa-ease);
  }

  button:hover {
    transform: none;
    border-color: var(--wa-border-strong);
    background: #ffffff;
    box-shadow: 0 8px 20px rgba(26, 41, 58, 0.08);
  }

  button.primary {
    border-color: var(--wa-accent);
    background: var(--wa-accent);
    color: var(--wa-accent-ink);
  }

  .view-switch {
    border-color: var(--wa-border-soft);
    border-radius: var(--wa-radius-lg);
    background: var(--wa-surface-inset);
  }

  .view-switch button.active {
    background: var(--wa-surface-flat);
    color: var(--wa-text-strong);
    box-shadow: var(--wa-shadow-sm);
  }

  .table-wrap {
    background: rgba(255, 255, 255, 0.72);
    scrollbar-color: rgba(0, 143, 150, 0.38) rgba(121, 139, 159, 0.1);
  }

  table {
    min-width: 780px;
  }

  th,
  td {
    border-bottom-color: rgba(121, 139, 159, 0.16);
    color: var(--wa-text-main);
    font-size: 13px;
  }

  th {
    color: var(--wa-text-muted);
    background: rgba(248, 251, 254, 0.96);
    letter-spacing: 0;
    text-transform: none;
  }

  td strong {
    color: var(--wa-text-strong);
  }

  tbody tr:hover {
    background: var(--wa-row-hover);
  }

  tbody tr.hot {
    background: rgba(221, 75, 62, 0.045);
    box-shadow: inset 0 0 0 1px rgba(221, 75, 62, 0.2);
  }

  .risk-pill {
    border-radius: var(--wa-radius-sm);
    background: var(--wa-neutral-soft);
    color: var(--wa-text-muted);
  }

  .risk-good {
    border-color: rgba(4, 150, 111, 0.2);
    background: var(--wa-success-soft);
    color: var(--wa-success);
  }

  .risk-warn {
    border-color: rgba(216, 135, 0, 0.22);
    background: var(--wa-warning-soft);
    color: var(--wa-warning);
  }

  .risk-danger {
    border-color: rgba(221, 75, 62, 0.22);
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .detail-panel {
    position: sticky;
    top: 16px;
    max-height: calc(100dvh - 98px);
    overflow: auto;
  }

  .detail-top p,
  .evidence-stack p {
    color: var(--wa-text-main);
  }

  .meter-head span,
  .detail-meta dt {
    color: var(--wa-text-muted);
  }

  .meter-head strong,
  .detail-meta dd {
    color: var(--wa-text-strong);
  }

  .meter-track {
    background: rgba(121, 139, 159, 0.14);
  }

  .detail-meta div,
  .evidence-stack p {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-inset);
  }

  .evidence-stack p {
    border-left-color: var(--wa-accent);
  }

  @media (max-width: 1220px) {
    .detail-panel {
      position: static;
      max-height: none;
    }
  }
</style>
