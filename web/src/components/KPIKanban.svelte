<script lang="ts">
  import { onMount } from 'svelte';

  type ReportType = 'daily' | 'weekly';
  type Tone = 'info' | 'success' | 'warning' | 'danger' | 'neutral';

  interface KPISummary {
    total_completed: number;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
  }

  interface UserKPI {
    username: string;
    name: string;
    avatar: string;
    department: string;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
    total_completed: number;
    overdue_completed: number;
    active_overdue: number;
    avg_cycle_days: number;
    review_count: number;
    mr_count: number;
    risk_notes: string[];
    task_count?: number;
    bug_count?: number;
    delay_ratio?: number;
    requirement_base_score?: number;
    scored_item_count?: number;
    manual_score_count?: number;
  }

  interface DepartmentKPI {
    department: string;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
    total_completed: number;
  }

  interface MemberView extends UserKPI {
    id: string;
    task_count_value: number;
    bug_count_value: number;
    delay_ratio_value: number;
    base_score_value: number;
    kpi_score: number;
    ability_score: number;
    ability_label: string;
    ability_tone: Tone;
    delivery_score: number;
    evidence_score: number;
    flow_score: number;
    risk_score: number;
  }

  interface ReportOverview {
    title: string;
    period_label: string;
    summary: string;
    total_completed: number;
    active_overdue: number;
    overdue_completed: number;
    mr_count: number;
    personal_count: number;
    department_count: number;
  }

  interface ReportRisk {
    level: string;
    owner: string;
    department: string;
    title: string;
    detail: string;
    evidence_ids: string[];
  }

  interface MeetingFocus {
    topic: string;
    owner: string;
    department: string;
    reason: string;
    evidence_ids: string[];
  }

  interface ReportEvidence {
    id: string;
    task_id: string;
    title: string;
    repo: string;
    assignee: string;
    department: string;
    status: string;
    issue_type: string;
    last_update: string;
    mr_url?: string;
  }

  interface PersonalSection {
    user: UserKPI;
    highlights: string[];
    risk_notes: string[];
    evidence_ids: string[];
  }

  interface KPIReportPreview {
    period: string;
    type: string;
    user?: string;
    generated_at: string;
    overview: ReportOverview;
    personal_sections: PersonalSection[];
    department_sections: unknown[];
    risks: ReportRisk[];
    meeting_focus: MeetingFocus[];
    evidence: ReportEvidence[];
  }

  interface MetricItem {
    label: string;
    value: string | number;
    helper: string;
    tone: Tone;
  }

  const reportModes: Array<{ key: ReportType; period: string; label: string; window: string }> = [
    { key: 'daily', period: 'day', label: '日报', window: '近 24 小时' },
    { key: 'weekly', period: 'week', label: '周报', window: '近 7 天' }
  ];

  const emptySummary: KPISummary = {
    total_completed: 0,
    tasks_completed: 0,
    bugs_completed: 0,
    demands_completed: 0
  };

  let reportType: ReportType = 'weekly';
  let activePeriod = 'week';
  let loading = true;
  let refreshing = false;
  let reportLoading = false;
  let errorMsg = '';
  let reportErrorMsg = '';
  let summary: KPISummary = { ...emptySummary };
  let userKPIList: UserKPI[] = [];
  let departmentKPIList: DepartmentKPI[] = [];
  let reportPreview: KPIReportPreview | null = null;
  let selectedMemberId = '';
  let reportUser = '';

  $: selectedMode = reportModes.find((mode) => mode.key === reportType) || reportModes[1];
  $: maxDelivery = Math.max(1, ...userKPIList.map((item) => item.total_completed || 0));
  $: maxMR = Math.max(1, ...userKPIList.map((item) => item.mr_count || 0));
  $: members = userKPIList
    .map((item) => buildMemberView(item, maxDelivery, maxMR))
    .sort((a, b) => b.ability_score - a.ability_score || b.total_completed - a.total_completed || a.name.localeCompare(b.name));
  $: selectedMember = members.find((item) => item.id === selectedMemberId) || members[0] || null;
  $: totalMR = members.reduce((sum, item) => sum + (item.mr_count || 0), 0);
  $: totalActiveOverdue = members.reduce((sum, item) => sum + (item.active_overdue || 0), 0);
  $: totalOverdueCompleted = members.reduce((sum, item) => sum + (item.overdue_completed || 0), 0);
  $: totalRisk = totalActiveOverdue + totalOverdueCompleted;
  $: totalWorkItems = members.reduce((sum, item) => sum + item.task_count_value + item.bug_count_value, 0);
  $: teamDelayRatio = totalWorkItems > 0
    ? roundOne((members.reduce((sum, item) => sum + item.delay_ratio_value * (item.task_count_value + item.bug_count_value), 0)) / totalWorkItems)
    : 0;
  $: evidenceCoverage = summary.total_completed > 0 ? Math.min(100, percentage(totalMR, summary.total_completed)) : 0;
  $: teamMetrics = buildTeamMetrics(summary, members.length, totalRisk, totalActiveOverdue, totalOverdueCompleted, evidenceCoverage, totalMR, teamDelayRatio);
  $: deliveryMix = [
    { label: '任务', value: summary.tasks_completed, percent: percentage(summary.tasks_completed, summary.total_completed), tone: 'info' as Tone },
    { label: '需求', value: summary.demands_completed, percent: percentage(summary.demands_completed, summary.total_completed), tone: 'success' as Tone },
    { label: 'Bug', value: summary.bugs_completed, percent: percentage(summary.bugs_completed, summary.total_completed), tone: 'warning' as Tone }
  ];
  $: visibleRisks = reportPreview?.risks?.slice(0, 5) || [];
  $: visibleFocus = reportPreview?.meeting_focus?.slice(0, 4) || [];
  $: visibleEvidence = reportPreview?.evidence?.slice(0, 8) || [];
  $: selectedPersonalSection = reportPreview?.personal_sections?.find((section) => section.user.name === selectedMember?.name)
    || reportPreview?.personal_sections?.[0]
    || null;

  function authHeaders() {
    const token = localStorage.getItem('jwt_token') || '';
    return { Authorization: `Bearer ${token}` };
  }

  async function fetchKPIData(initial = false) {
    if (initial) loading = true;
    else refreshing = true;
    errorMsg = '';

    try {
      const response = await fetch(`/api/kpi/performance?period=${activePeriod}`, { headers: authHeaders() });
      if (!response.ok) {
        if (response.status === 403) throw new Error('当前账号无权查看度量数据');
        throw new Error(`加载度量数据失败（${response.status}）`);
      }
      const data = await response.json();
      summary = data.summary || { ...emptySummary };
      userKPIList = Array.isArray(data.user_kpi) ? data.user_kpi : [];
      departmentKPIList = Array.isArray(data.department_kpi) ? data.department_kpi : [];
      if (!selectedMemberId && userKPIList.length > 0) {
        selectedMemberId = memberID(userKPIList[0]);
      }
      await fetchReportPreview();
    } catch (error: any) {
      errorMsg = error?.message || '加载度量数据失败，请稍后重试';
    } finally {
      loading = false;
      refreshing = false;
    }
  }

  async function fetchReportPreview() {
    reportLoading = true;
    reportErrorMsg = '';
    try {
      const params = new URLSearchParams({ period: activePeriod, type: reportType });
      if (reportUser) params.set('user', reportUser);
      const response = await fetch(`/api/kpi/report-preview?${params.toString()}`, { headers: authHeaders() });
      if (!response.ok) throw new Error(`报告生成失败（${response.status}）`);
      reportPreview = await response.json();
    } catch (error: any) {
      reportErrorMsg = error?.message || '报告生成失败，请稍后重试';
    } finally {
      reportLoading = false;
    }
  }

  function changeReportType(type: ReportType) {
    if (type === reportType || refreshing || loading) return;
    reportType = type;
    activePeriod = reportModes.find((mode) => mode.key === type)?.period || 'week';
    reportUser = '';
    fetchKPIData();
  }

  function selectMember(member: MemberView) {
    selectedMemberId = member.id;
  }

  function openPersonalReport(member: MemberView) {
    selectedMemberId = member.id;
    reportUser = member.name;
    fetchReportPreview();
  }

  function showTeamReport() {
    reportUser = '';
    fetchReportPreview();
  }

  function memberID(item: UserKPI) {
    return item.username || item.name || 'unknown-member';
  }

  function buildMemberView(item: UserKPI, deliveryMax: number, mrMax: number): MemberView {
    const taskCount = item.task_count ?? item.tasks_completed ?? 0;
    const bugCount = item.bug_count ?? item.bugs_completed ?? 0;
    const workItems = taskCount + bugCount;
    const fallbackDelayRatio = workItems > 0 ? roundOne(((item.active_overdue || 0) + (item.overdue_completed || 0)) * 100 / workItems) : 0;
    const delayRatio = Number.isFinite(item.delay_ratio) ? Number(item.delay_ratio) : fallbackDelayRatio;
    const baseScore = item.requirement_base_score || 60;
    const deliveryScore = deliveryMax > 0 ? Math.round((item.total_completed / deliveryMax) * 40) : 0;
    const evidenceScore = clamp(Math.round(((item.mr_count || 0) / mrMax) * 18 + (item.review_count || 0) * 2), 0, 25);
    const flowScore = item.total_completed > 0 ? clamp(Math.round(20 - Math.max(item.avg_cycle_days || 0, 0) * 2), 4, 20) : 0;
    const riskScore = Math.max(0, 15 - Math.min(15, (item.active_overdue || 0) * 6 + (item.overdue_completed || 0) * 4));
    const kpiScore = clamp(deliveryScore + evidenceScore + flowScore + riskScore, 0, 100);
    const abilityScore = clamp(Math.round(baseScore * 0.6 + kpiScore * 0.4), 0, 100);

    return {
      ...item,
      id: memberID(item),
      task_count_value: taskCount,
      bug_count_value: bugCount,
      delay_ratio_value: delayRatio,
      base_score_value: roundOne(baseScore),
      kpi_score: kpiScore,
      ability_score: abilityScore,
      ability_label: abilityLabel(abilityScore),
      ability_tone: scoreTone(abilityScore),
      delivery_score: deliveryScore,
      evidence_score: evidenceScore,
      flow_score: flowScore,
      risk_score: riskScore
    };
  }

  function buildTeamMetrics(
    currentSummary: KPISummary,
    memberCount: number,
    riskCount: number,
    activeOverdue: number,
    overdueCompleted: number,
    coverage: number,
    mrCount: number,
    delayRatio: number
  ): MetricItem[] {
    return [
      {
        label: '交付进度',
        value: currentSummary.total_completed,
        helper: `任务 ${currentSummary.tasks_completed} · 需求 ${currentSummary.demands_completed} · Bug ${currentSummary.bugs_completed}`,
        tone: 'info'
      },
      {
        label: '风险暴露',
        value: riskCount,
        helper: `${memberCount} 人 · 当前超期 ${activeOverdue}`,
        tone: riskCount > 0 ? 'warning' : 'success'
      },
      {
        label: '证据覆盖率',
        value: `${coverage}%`,
        helper: `MR ${mrCount} · 交付 ${currentSummary.total_completed}`,
        tone: coverage >= 80 ? 'success' : coverage >= 50 ? 'warning' : 'danger'
      },
      {
        label: 'Delay 占比',
        value: `${delayRatio}%`,
        helper: `当前超期 ${activeOverdue} · 延期完成 ${overdueCompleted}`,
        tone: delayRatio > 20 ? 'danger' : delayRatio > 0 ? 'warning' : 'success'
      }
    ];
  }

  function abilityLabel(score: number) {
    if (score >= 88) return '专家级';
    if (score >= 78) return '高级';
    if (score >= 66) return '胜任';
    return '成长中';
  }

  function scoreTone(score: number): Tone {
    if (score >= 85) return 'success';
    if (score >= 70) return 'info';
    if (score >= 55) return 'warning';
    return 'danger';
  }

  function riskTone(level: string): Tone {
    const normalized = (level || '').toLowerCase();
    if (normalized === 'high' || normalized === 'critical') return 'danger';
    if (normalized === 'medium' || normalized === 'warning') return 'warning';
    return 'info';
  }

  function clamp(value: number, min: number, max: number) {
    return Math.max(min, Math.min(max, value));
  }

  function percentage(value: number, total: number) {
    if (!total) return 0;
    return Math.round((value / total) * 100);
  }

  function roundOne(value: number) {
    return Math.round(value * 10) / 10;
  }

  function formatDateTime(value?: string) {
    if (!value) return '尚未生成';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false });
  }

  function issueTypeLabel(type: string) {
    const normalized = (type || '').toLowerCase();
    if (normalized === 'bug') return 'Bug';
    if (normalized === 'demand') return '需求';
    return '任务';
  }

  onMount(() => {
    fetchKPIData(true);
  });
</script>

<div class="metrics-workspace" aria-busy={loading || refreshing || reportLoading}>
  <section class="metrics-toolbar wa-admin-card wa-admin-toolbar" aria-label="度量周期控制">
    <div class="period-switch" aria-label="报告周期">
      {#each reportModes as mode}
        <button
          type="button"
          class:is-active={reportType === mode.key}
          aria-pressed={reportType === mode.key}
          on:click={() => changeReportType(mode.key)}
        >
          <strong>{mode.label}</strong>
          <span>{mode.window}</span>
        </button>
      {/each}
    </div>
    <div class="toolbar-context">
      <span>{reportUser ? `个人报告 · ${reportUser}` : `${selectedMode.label} · 团队视图`}</span>
      <span>生成于 {formatDateTime(reportPreview?.generated_at)}</span>
      <button type="button" class="wa-admin-action secondary" disabled={refreshing || loading} on:click={() => fetchKPIData()}>
        {refreshing ? '刷新中' : '刷新数据'}
      </button>
    </div>
  </section>

  {#if loading}
    <section class="metrics-loading" aria-label="正在加载度量数据">
      {#each Array(4) as _}
        <div class="metric-skeleton"></div>
      {/each}
      <div class="panel-skeleton wide"></div>
      <div class="panel-skeleton"></div>
    </section>
  {:else if errorMsg}
    <section class="metrics-state is-error wa-admin-card" role="alert">
      <strong>度量数据加载失败</strong>
      <span>{errorMsg}</span>
      <button type="button" class="wa-admin-action primary" on:click={() => fetchKPIData()}>重新加载</button>
    </section>
  {:else}
    <section class="metric-strip" aria-label="周期健康指标">
      {#each teamMetrics as metric}
        <article class="metric-card wa-admin-card tone-{metric.tone}">
          <span>{metric.label}</span>
          <strong>{metric.value}</strong>
          <small>{metric.helper}</small>
        </article>
      {/each}
    </section>

    <section class="overview-grid" aria-label="周期交付与风险概览">
      <article class="delivery-panel wa-admin-card">
        <header class="section-header">
          <div>
            <h2>{reportPreview?.overview.title || `${selectedMode.label}交付概览`}</h2>
            <p>{reportPreview?.overview.summary || `${selectedMode.window}内暂无可生成的交付摘要。`}</p>
          </div>
          <span class="section-meta">{selectedMode.window}</span>
        </header>

        <div class="delivery-content">
          <div class="delivery-total">
            <span>本周期完成</span>
            <strong>{summary.total_completed}</strong>
            <small>项可追溯交付</small>
          </div>
          <div class="delivery-bars">
            {#each deliveryMix as item}
              <div class="delivery-row">
                <div><span>{item.label}</span><strong>{item.value} · {item.percent}%</strong></div>
                <div class="delivery-track"><i class="tone-{item.tone}" style="width: {item.percent}%"></i></div>
              </div>
            {/each}
          </div>
        </div>

        <footer class="delivery-footer">
          <span>参与成员 <strong>{members.length}</strong></span>
          <span>平均周期 <strong>{members.length ? roundOne(members.reduce((sum, item) => sum + (item.avg_cycle_days || 0), 0) / members.length) : 0} 天</strong></span>
          <span>部门覆盖 <strong>{departmentKPIList.length}</strong></span>
        </footer>
      </article>

      <aside class="risk-panel wa-admin-card">
        <header class="section-header compact">
          <div><h2>风险与会议关注</h2><p>先处理超期，再确认协作与证据缺口。</p></div>
          <span class="risk-count">{visibleRisks.length}</span>
        </header>

        {#if reportErrorMsg}
          <div class="inline-state is-error">{reportErrorMsg}</div>
        {:else if reportLoading && !reportPreview}
          <div class="inline-state">正在生成风险摘要</div>
        {:else if visibleRisks.length === 0}
          <div class="inline-state is-success">当前周期没有超期或延期风险。</div>
        {:else}
          <div class="risk-list">
            {#each visibleRisks as risk}
              <article>
                <span class="status-pill tone-{riskTone(risk.level)}">{risk.level || '关注'}</span>
                <div><strong>{risk.title}</strong><p>{risk.detail}</p><small>{risk.owner || '未指派'} · {risk.department || '未分配'}</small></div>
              </article>
            {/each}
          </div>
        {/if}

        {#if visibleFocus.length > 0}
          <div class="focus-list">
            <h3>会议建议</h3>
            {#each visibleFocus as item}
              <div><strong>{item.topic}</strong><span>{item.owner || '未指派'} · {item.reason}</span></div>
            {/each}
          </div>
        {/if}
      </aside>
    </section>

    <section class="member-panel wa-admin-card" aria-label="个人能力与负载">
      <header class="section-header member-header">
        <div>
          <h2>人员能力与交付负载</h2>
          <p>需求基础分占 60%，周期交付与证据评分占 40%；人工修订沿用排期估算记录。</p>
        </div>
        <div class="member-actions">
          <span>{members.length} 人</span>
          {#if reportUser}<button type="button" class="wa-admin-action secondary" on:click={showTeamReport}>返回团队报告</button>{/if}
        </div>
      </header>

      {#if members.length === 0}
        <div class="metrics-state compact">
          <strong>当前周期没有个人度量记录</strong>
          <span>任务、Bug 或评审进入周期后会自动出现在这里。</span>
        </div>
      {:else}
        <div class="member-workbench">
          <div class="member-table-shell wa-admin-table-shell">
            <table class="member-table wa-admin-table">
              <thead>
                <tr><th>成员</th><th>任务 / Bug</th><th>Delay</th><th>基础分</th><th>能力评级</th><th>交付证据</th></tr>
              </thead>
              <tbody>
                {#each members as member, index}
                  <tr class:is-selected={selectedMember?.id === member.id}>
                    <td>
                      <button type="button" class="member-select" on:click={() => selectMember(member)}>
                        <span class="member-rank">{index + 1}</span>
                        {#if member.avatar}<img src={member.avatar} alt={member.name} />{:else}<span class="member-avatar">{member.name.slice(0, 1)}</span>{/if}
                        <span class="member-name"><strong>{member.name}</strong><small>{member.department || '未分配'}</small></span>
                      </button>
                    </td>
                    <td class="numeric"><strong>{member.task_count_value}</strong><span> / {member.bug_count_value}</span></td>
                    <td class="numeric"><strong class:is-danger={member.delay_ratio_value > 0}>{member.delay_ratio_value}%</strong></td>
                    <td class="numeric"><strong>{member.base_score_value}</strong><small>{member.scored_item_count || 0} 项评分</small></td>
                    <td><div class="ability-cell"><span class="status-pill tone-{member.ability_tone}">{member.ability_label}</span><strong>{member.ability_score}</strong></div></td>
                    <td><div class="evidence-cell"><strong>完成 {member.total_completed}</strong><span>MR {member.mr_count || 0} · 评审 {member.review_count || 0}</span></div></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>

          <aside class="member-inspector">
            {#if selectedMember}
              <div class="inspector-heading">
                <div><span>成员详情</span><h3>{selectedMember.name}</h3><small>{selectedMember.department || '未分配部门'}</small></div>
                <span class="status-pill tone-{selectedMember.ability_tone}">{selectedMember.ability_label}</span>
              </div>

              <div class="inspector-score">
                <div><span>综合能力</span><strong>{selectedMember.ability_score}</strong></div>
                <div><span>需求基础分</span><strong>{selectedMember.base_score_value}</strong></div>
              </div>

              <dl class="inspector-facts">
                <div><dt>任务 / Bug</dt><dd>{selectedMember.task_count_value} / {selectedMember.bug_count_value}</dd></div>
                <div><dt>Delay 占比</dt><dd class:is-danger={selectedMember.delay_ratio_value > 0}>{selectedMember.delay_ratio_value}%</dd></div>
                <div><dt>本期交付</dt><dd>{selectedMember.total_completed}</dd></div>
                <div><dt>平均周期</dt><dd>{roundOne(selectedMember.avg_cycle_days || 0)} 天</dd></div>
                <div><dt>MR / 评审</dt><dd>{selectedMember.mr_count || 0} / {selectedMember.review_count || 0}</dd></div>
                <div><dt>人工修订</dt><dd>{selectedMember.manual_score_count || 0} 项</dd></div>
              </dl>

              <div class="score-breakdown">
                <h4>周期评分拆解</h4>
                <div><span>产出</span><i style="width: {percentage(selectedMember.delivery_score, 40)}%"></i><strong>{selectedMember.delivery_score}/40</strong></div>
                <div><span>证据</span><i style="width: {percentage(selectedMember.evidence_score, 25)}%"></i><strong>{selectedMember.evidence_score}/25</strong></div>
                <div><span>效率</span><i style="width: {percentage(selectedMember.flow_score, 20)}%"></i><strong>{selectedMember.flow_score}/20</strong></div>
                <div><span>风险</span><i style="width: {percentage(selectedMember.risk_score, 15)}%"></i><strong>{selectedMember.risk_score}/15</strong></div>
              </div>

              {#if selectedMember.risk_notes?.length > 0}
                <div class="member-notes"><h4>风险说明</h4>{#each selectedMember.risk_notes.slice(0, 3) as note}<p>{note}</p>{/each}</div>
              {/if}

              <button type="button" class="wa-admin-action primary inspector-action" on:click={() => openPersonalReport(selectedMember)} disabled={reportLoading}>
                {reportLoading && reportUser === selectedMember.name ? '生成中' : '查看个人报告'}
              </button>
            {/if}
          </aside>
        </div>
      {/if}
    </section>

    <section class="evidence-panel wa-admin-card" aria-label="周期证据">
      <header class="section-header">
        <div><h2>{reportUser ? `${reportUser} 的报告证据` : '周期证据样本'}</h2><p>{selectedPersonalSection?.highlights?.[0] || '展示支撑当前周期统计的任务、需求、Bug 与交付记录。'}</p></div>
        <span class="section-meta">{reportPreview?.evidence?.length || 0} 条</span>
      </header>

      {#if reportLoading && !reportPreview}
        <div class="inline-state">正在加载报告证据</div>
      {:else if visibleEvidence.length === 0}
        <div class="inline-state">当前周期暂无可追溯证据。</div>
      {:else}
        <div class="evidence-grid">
          {#each visibleEvidence as item}
            <article>
              <div><span class="evidence-id">{item.task_id}</span><span class="status-pill tone-neutral">{issueTypeLabel(item.issue_type)}</span></div>
              <strong>{item.title}</strong>
              <small>{item.assignee || '未指派'} · {item.status || '未知状态'} · {item.repo || '未关联仓库'}</small>
              {#if item.mr_url}<a href={item.mr_url} target="_blank" rel="noopener noreferrer">查看 MR</a>{/if}
            </article>
          {/each}
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .metrics-workspace {
    width: 100%;
    min-width: 0;
    display: grid;
    gap: var(--wa-space-4, 16px);
    color: var(--wa-text-main, #263546);
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
  }

  .metrics-workspace * {
    box-sizing: border-box;
  }

  .metrics-toolbar {
    min-width: 0;
    min-height: 58px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 8px 10px;
  }

  .period-switch {
    display: inline-flex;
    gap: 8px;
  }

  .period-switch button {
    min-width: 104px;
    min-height: 40px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    border: 1px solid var(--wa-border-strong, rgba(88, 108, 128, .28));
    border-radius: 9px;
    background: var(--wa-surface-lift, #fff);
    color: var(--wa-text-main, #263546);
    cursor: pointer;
    transition: background 160ms ease, border-color 160ms ease, color 160ms ease, transform 160ms ease;
  }

  .period-switch button strong { font-size: 13px; line-height: 1; }
  .period-switch button span { color: var(--wa-text-muted, #66778a); font-size: 11px; line-height: 1; }

  .period-switch button:hover,
  .period-switch button:focus-visible {
    border-color: var(--wa-accent, #008f96);
    color: var(--wa-accent-strong, #006f76);
    background: #f0f8f8;
    outline: 2px solid rgba(0, 111, 118, .24);
    outline-offset: 2px;
  }

  .period-switch button:hover { transform: translateY(-1px); }

  .period-switch button.is-active {
    border-color: #006f76;
    color: #fff;
    background: #006f76;
  }

  .period-switch button.is-active span { color: #fff; }

  .toolbar-context {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 12px;
    color: var(--wa-text-muted, #708196);
    font-size: 12px;
    white-space: nowrap;
  }

  .metric-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .metric-card {
    position: relative;
    min-width: 0;
    min-height: 112px;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 16px 18px;
    overflow: hidden;
  }

  .metric-card::before {
    content: "";
    position: absolute;
    inset: 0 auto 0 0;
    width: 4px;
    background: var(--metric-color, var(--wa-text-muted, #708196));
  }

  .metric-card.tone-info { --metric-color: var(--wa-info, #256bd8); }
  .metric-card.tone-success { --metric-color: var(--wa-success, #14866d); }
  .metric-card.tone-warning { --metric-color: var(--wa-warning, #b7791f); }
  .metric-card.tone-danger { --metric-color: var(--wa-danger, #d34f43); }

  .metric-card > span,
  .metric-card small {
    min-width: 0;
    overflow: hidden;
    color: var(--wa-text-muted, #708196);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .metric-card strong {
    color: var(--wa-text-strong, #142231);
    font-size: 30px;
    line-height: 1;
    letter-spacing: -.025em;
    font-variant-numeric: tabular-nums;
  }

  .overview-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1.45fr) minmax(330px, .75fr);
    gap: 16px;
    align-items: stretch;
  }

  .delivery-panel,
  .risk-panel,
  .member-panel,
  .evidence-panel {
    min-width: 0;
    padding: 18px;
  }

  .delivery-panel {
    display: flex;
    flex-direction: column;
  }

  .section-header {
    min-width: 0;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 16px;
  }

  .section-header.compact { margin-bottom: 12px; }
  .section-header > div { min-width: 0; }
  .section-header h2 { margin: 0; color: var(--wa-text-strong, #142231); font-size: 17px; line-height: 1.25; font-weight: 820; }
  .section-header p { max-width: 760px; margin: 5px 0 0; color: var(--wa-text-muted, #708196); font-size: 12px; line-height: 1.55; text-wrap: pretty; }

  .section-meta,
  .risk-count,
  .member-actions > span {
    flex: 0 0 auto;
    min-height: 28px;
    display: inline-flex;
    align-items: center;
    border-radius: 8px;
    padding: 0 9px;
    color: var(--wa-text-muted, #708196);
    background: var(--wa-surface-inset, rgba(239, 244, 248, .76));
    font-size: 11px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }

  .risk-count { color: var(--wa-warning, #b7791f); background: var(--wa-warning-soft, rgba(183, 121, 31, .1)); }

  .delivery-content {
    min-width: 0;
    display: grid;
    grid-template-columns: 160px minmax(0, 1fr);
    gap: 24px;
    align-items: center;
    padding: 18px;
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-surface-inset, rgba(239, 244, 248, .66));
  }

  .delivery-total { display: grid; gap: 4px; padding-right: 22px; border-right: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .18)); }
  .delivery-total span, .delivery-total small { color: var(--wa-text-muted, #708196); font-size: 11px; }
  .delivery-total strong { color: var(--wa-text-strong, #142231); font-size: 42px; line-height: 1; letter-spacing: -.04em; font-variant-numeric: tabular-nums; }

  .delivery-bars { display: grid; gap: 13px; }
  .delivery-row { display: grid; gap: 6px; }
  .delivery-row > div:first-child { display: flex; justify-content: space-between; gap: 12px; font-size: 12px; }
  .delivery-row span { color: var(--wa-text-main, #263546); }
  .delivery-row strong { color: var(--wa-text-strong, #142231); font-variant-numeric: tabular-nums; }
  .delivery-track { height: 7px; overflow: hidden; border-radius: 999px; background: rgba(121, 139, 159, .16); }
  .delivery-track i { display: block; height: 100%; border-radius: inherit; background: var(--wa-info, #256bd8); }
  .delivery-track i.tone-success { background: var(--wa-success, #14866d); }
  .delivery-track i.tone-warning { background: var(--wa-warning, #b7791f); }

  .delivery-footer { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; margin-top: auto; padding-top: 12px; }
  .delivery-footer span { display: flex; justify-content: space-between; gap: 8px; border: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .16)); border-radius: 9px; padding: 9px 10px; color: var(--wa-text-muted, #708196); font-size: 11px; }
  .delivery-footer strong { color: var(--wa-text-strong, #142231); font-variant-numeric: tabular-nums; }

  .risk-list { display: grid; gap: 7px; }

  .risk-panel {
    height: 100%;
    max-height: none;
    overflow: visible;
  }
  .risk-list article { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 9px; padding: 9px 0; border-bottom: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .14)); }
  .risk-list article:last-child { border-bottom: 0; }
  .risk-list strong { display: block; overflow: hidden; color: var(--wa-text-strong, #142231); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
  .risk-list p { margin: 3px 0; color: var(--wa-text-main, #263546); font-size: 11px; line-height: 1.45; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; line-clamp: 2; overflow: hidden; }
  .risk-list small { color: var(--wa-text-muted, #708196); font-size: 10px; }

  .status-pill { min-height: 24px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 7px; padding: 0 7px; font-size: 10px; font-weight: 760; white-space: nowrap; }
  .status-pill.tone-info { color: var(--wa-info, #256bd8); background: var(--wa-info-soft, rgba(37, 107, 216, .09)); border-color: rgba(37, 107, 216, .14); }
  .status-pill.tone-success { color: var(--wa-success, #14866d); background: var(--wa-success-soft, rgba(20, 134, 109, .09)); border-color: rgba(20, 134, 109, .14); }
  .status-pill.tone-warning { color: var(--wa-warning, #b7791f); background: var(--wa-warning-soft, rgba(183, 121, 31, .09)); border-color: rgba(183, 121, 31, .14); }
  .status-pill.tone-danger { color: var(--wa-danger, #d34f43); background: var(--wa-danger-soft, rgba(211, 79, 67, .09)); border-color: rgba(211, 79, 67, .14); }
  .status-pill.tone-neutral { color: var(--wa-text-muted, #708196); background: var(--wa-surface-inset, rgba(239, 244, 248, .76)); border-color: var(--wa-border-soft, rgba(121, 139, 159, .14)); }

  .focus-list { margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .16)); }
  .focus-list h3 { margin: 0 0 8px; color: var(--wa-text-strong, #142231); font-size: 12px; }
  .focus-list > div { display: grid; gap: 2px; padding: 6px 0; }
  .focus-list strong { color: var(--wa-text-main, #263546); font-size: 11px; }
  .focus-list span { overflow: hidden; color: var(--wa-text-muted, #708196); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }

  .member-header { align-items: center; }
  .member-actions { display: flex; align-items: center; gap: 8px; }
  .member-workbench { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) 330px; gap: 16px; align-items: start; }
  .member-table-shell { min-width: 0; max-height: 560px; overflow: auto; border-radius: var(--wa-radius-lg, 14px); }
  .member-table { width: 100%; min-width: 900px; table-layout: fixed; }
  .member-table th { position: sticky; top: 0; z-index: 2; height: 42px; color: var(--wa-text-muted, #708196); background: var(--wa-surface-inset, #f1f5f8); font-size: 11px; text-align: left; }
  .member-table th:nth-child(1) { width: 230px; }
  .member-table th:nth-child(2) { width: 105px; text-align: right; }
  .member-table th:nth-child(3) { width: 90px; text-align: right; }
  .member-table th:nth-child(4) { width: 105px; text-align: right; }
  .member-table th:nth-child(5) { width: 145px; }
  .member-table th:nth-child(6) { width: 165px; }
  .member-table td { height: 62px; padding: 8px 12px; border-bottom: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .13)); vertical-align: middle; }
  .member-table tbody tr { background: rgba(255, 255, 255, .74); transition: background 140ms ease, box-shadow 140ms ease; }
  .member-table tbody tr:hover { background: var(--wa-row-hover, rgba(0, 143, 150, .05)); }
  .member-table tbody tr.is-selected { background: var(--wa-accent-soft, rgba(0, 143, 150, .09)); box-shadow: inset 3px 0 0 var(--wa-accent, #008f96); }
  .member-table td.numeric { text-align: right; font-variant-numeric: tabular-nums; }
  .member-table td.numeric > strong { color: var(--wa-text-strong, #142231); font-size: 13px; }
  .member-table td.numeric > span, .member-table td.numeric > small { display: block; color: var(--wa-text-muted, #708196); font-size: 10px; }
  .member-table .is-danger { color: var(--wa-danger, #d34f43) !important; }

  .member-select { width: 100%; min-width: 0; min-height: 44px; display: grid; grid-template-columns: 24px 32px minmax(0, 1fr); align-items: center; gap: 8px; border: 0; background: transparent; padding: 0; color: inherit; text-align: left; cursor: pointer; }
  .member-select:focus-visible { border-radius: 8px; outline: 2px solid var(--wa-border-focus, rgba(0, 143, 150, .34)); outline-offset: 2px; }
  .member-rank { width: 24px; height: 24px; display: grid; place-items: center; border-radius: 7px; color: var(--wa-text-muted, #708196); background: var(--wa-surface-inset, rgba(239, 244, 248, .82)); font-size: 10px; font-weight: 700; }
  .member-avatar, .member-select img { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid rgba(0, 143, 150, .14); border-radius: 10px; object-fit: cover; background: var(--wa-accent-soft, rgba(0, 143, 150, .1)); color: var(--wa-accent-strong, #006f76); font-size: 12px; font-weight: 800; }
  .member-name { min-width: 0; display: grid; gap: 2px; }
  .member-name strong { overflow: hidden; color: var(--wa-text-strong, #142231); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
  .member-name small { color: var(--wa-text-muted, #708196); font-size: 10px; }
  .ability-cell { display: flex; align-items: center; gap: 8px; }
  .ability-cell > strong { color: var(--wa-text-strong, #142231); font-size: 14px; font-variant-numeric: tabular-nums; }
  .evidence-cell { display: grid; gap: 3px; }
  .evidence-cell strong { color: var(--wa-text-strong, #142231); font-size: 11px; }
  .evidence-cell span { color: var(--wa-text-muted, #708196); font-size: 10px; }

  .member-inspector { min-width: 0; display: grid; gap: 14px; border: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .17)); border-radius: var(--wa-radius-lg, 14px); padding: 16px; background: var(--wa-surface-lift, rgba(255, 255, 255, .82)); }
  .inspector-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
  .inspector-heading > div > span { color: var(--wa-text-muted, #708196); font-size: 10px; }
  .inspector-heading h3 { margin: 3px 0; color: var(--wa-text-strong, #142231); font-size: 17px; }
  .inspector-heading small { color: var(--wa-text-muted, #708196); font-size: 10px; }
  .inspector-score { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .inspector-score > div { display: grid; gap: 4px; border-radius: 10px; padding: 11px; background: var(--wa-surface-inset, rgba(239, 244, 248, .76)); }
  .inspector-score span { color: var(--wa-text-muted, #708196); font-size: 10px; }
  .inspector-score strong { color: var(--wa-text-strong, #142231); font-size: 22px; font-variant-numeric: tabular-nums; }
  .inspector-facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin: 0; }
  .inspector-facts > div { display: flex; justify-content: space-between; gap: 8px; border-bottom: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .13)); padding: 7px 0; }
  .inspector-facts dt { color: var(--wa-text-muted, #708196); font-size: 10px; }
  .inspector-facts dd { margin: 0; color: var(--wa-text-strong, #142231); font-size: 11px; font-weight: 760; font-variant-numeric: tabular-nums; }
  .inspector-facts dd.is-danger { color: var(--wa-danger, #d34f43); }

  .score-breakdown, .member-notes { display: grid; gap: 8px; }
  .score-breakdown h4, .member-notes h4 { margin: 0; color: var(--wa-text-strong, #142231); font-size: 11px; }
  .score-breakdown > div { position: relative; min-height: 24px; display: grid; grid-template-columns: 40px minmax(0, 1fr) 42px; align-items: center; gap: 8px; color: var(--wa-text-muted, #708196); font-size: 10px; }
  .score-breakdown > div::before { content: ""; grid-column: 2; grid-row: 1; height: 5px; border-radius: 999px; background: rgba(121, 139, 159, .16); }
  .score-breakdown i { grid-column: 2; grid-row: 1; height: 5px; border-radius: 999px; background: var(--wa-accent, #008f96); }
  .score-breakdown strong { color: var(--wa-text-main, #263546); text-align: right; font-variant-numeric: tabular-nums; }
  .member-notes p { margin: 0; border-radius: 8px; padding: 7px 8px; color: var(--wa-text-main, #263546); background: var(--wa-warning-soft, rgba(183, 121, 31, .08)); font-size: 10px; line-height: 1.45; }
  .inspector-action { width: 100%; justify-content: center; }

  .evidence-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
  .evidence-grid article { min-width: 0; min-height: 112px; display: flex; flex-direction: column; gap: 7px; border: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .16)); border-radius: 11px; padding: 11px; background: var(--wa-surface-lift, rgba(255, 255, 255, .74)); }
  .evidence-grid article > div { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .evidence-id { color: var(--wa-info, #256bd8); font-size: 10px; font-weight: 800; }
  .evidence-grid article > strong { display: -webkit-box; overflow: hidden; color: var(--wa-text-strong, #142231); font-size: 11px; line-height: 1.45; -webkit-box-orient: vertical; -webkit-line-clamp: 2; line-clamp: 2; }
  .evidence-grid article > small { color: var(--wa-text-muted, #708196); font-size: 10px; line-height: 1.4; }
  .evidence-grid a { margin-top: auto; color: var(--wa-accent-strong, #006f76); font-size: 10px; font-weight: 700; text-decoration: none; }
  .evidence-grid a:hover, .evidence-grid a:focus-visible { text-decoration: underline; outline: 0; }

  .metrics-state { min-height: 180px; display: grid; place-items: center; align-content: center; gap: 8px; padding: 24px; text-align: center; }
  .metrics-state.compact { min-height: 120px; border: 1px dashed var(--wa-border-soft, rgba(121, 139, 159, .24)); border-radius: 12px; }
  .metrics-state strong { color: var(--wa-text-strong, #142231); font-size: 15px; }
  .metrics-state span { color: var(--wa-text-muted, #708196); font-size: 12px; }
  .metrics-state.is-error { border-color: rgba(211, 79, 67, .2); background: var(--wa-danger-soft, rgba(211, 79, 67, .06)); }
  .inline-state { min-height: 72px; display: grid; place-items: center; border-radius: 10px; color: var(--wa-text-muted, #708196); background: var(--wa-surface-inset, rgba(239, 244, 248, .68)); font-size: 11px; text-align: center; }
  .inline-state.is-error { color: var(--wa-danger, #d34f43); background: var(--wa-danger-soft, rgba(211, 79, 67, .07)); }
  .inline-state.is-success { color: var(--wa-success, #14866d); background: var(--wa-success-soft, rgba(20, 134, 109, .07)); }

  .metrics-loading { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
  .metric-skeleton, .panel-skeleton { border-radius: var(--wa-radius-lg, 14px); background: linear-gradient(90deg, rgba(226, 234, 243, .72), rgba(248, 250, 252, .96), rgba(226, 234, 243, .72)); background-size: 180% 100%; animation: metric-shimmer 1.2s linear infinite; }
  .metric-skeleton { height: 112px; }
  .panel-skeleton { grid-column: span 1; height: 320px; }
  .panel-skeleton.wide { grid-column: span 3; }

  @keyframes metric-shimmer { from { background-position: 100% 0; } to { background-position: -100% 0; } }

  @media (prefers-reduced-motion: reduce) {
    .metric-skeleton, .panel-skeleton { animation: none; }
  }

  @media (max-width: 1280px) {
    .metric-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .overview-grid, .member-workbench { grid-template-columns: 1fr; }
    .overview-grid { align-items: start; }
    .delivery-panel, .risk-panel { width: 100%; height: auto; }
    .member-inspector { grid-template-columns: minmax(220px, .8fr) minmax(0, 1.2fr); align-items: start; }
    .inspector-heading, .inspector-score, .inspector-facts, .inspector-action { grid-column: 1; }
    .score-breakdown, .member-notes { grid-column: 2; }
    .evidence-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }

  @media (max-width: 760px) {
    .metrics-toolbar, .toolbar-context, .section-header, .member-header { align-items: stretch; flex-direction: column; }
    .toolbar-context { white-space: normal; }
    .period-switch { width: 100%; }
    .period-switch button { flex: 1 1 0; min-width: 0; }
    .metric-strip, .metrics-loading { grid-template-columns: 1fr; }
    .delivery-content { grid-template-columns: 1fr; gap: 14px; }
    .delivery-total { padding: 0 0 14px; border-right: 0; border-bottom: 1px solid var(--wa-border-soft, rgba(121, 139, 159, .18)); }
    .delivery-footer, .inspector-score, .inspector-facts { grid-template-columns: 1fr; }
    .member-inspector { grid-template-columns: 1fr; }
    .inspector-heading, .inspector-score, .inspector-facts, .inspector-action, .score-breakdown, .member-notes { grid-column: 1; }
    .evidence-grid { grid-template-columns: 1fr; }
    .panel-skeleton, .panel-skeleton.wide { grid-column: 1; height: 220px; }
  }
</style>
