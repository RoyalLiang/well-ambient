<script lang="ts">
  import { onMount } from 'svelte';

  let activePeriod = 'week'; // 'day' | 'week' | 'month' | 'year'
  let reportType = 'weekly'; // 'daily' | 'weekly'
  let selectedReportUser = '';
  let loading = false;
  let reportLoading = false;
  let errorMsg = '';
  let reportErrorMsg = '';
  let showReportUserDropdown = false;
  let reportUserSelectEl: HTMLElement;

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
  }

  interface DeptKPI {
    department: string;
    tasks_completed: number;
    bugs_completed: number;
    demands_completed: number;
    total_completed: number;
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

  interface ReportPersonalSection {
    user: UserKPI;
    highlights: string[];
    risk_notes: string[];
    evidence_ids: string[];
  }

  interface ReportDepartmentSection {
    department: string;
    total_completed: number;
    active_overdue: number;
    overdue_completed: number;
    review_count: number;
    mr_count: number;
    highlights: string[];
    evidence_ids: string[];
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
    due_date?: string;
    completed_at?: string;
    last_update: string;
    mr_url?: string;
    task_group_id?: string;
  }

  interface KPIReportPreview {
    period: string;
    type: string;
    user?: string;
    generated_at: string;
    overview: ReportOverview;
    personal_sections: ReportPersonalSection[];
    department_sections: ReportDepartmentSection[];
    risks: ReportRisk[];
    meeting_focus: MeetingFocus[];
    evidence: ReportEvidence[];
  }

  const emptySummary: KPISummary = {
    total_completed: 0,
    tasks_completed: 0,
    bugs_completed: 0,
    demands_completed: 0,
  };

  let summary: KPISummary = { ...emptySummary };
  let userKPIList: UserKPI[] = [];
  let deptKPIList: DeptKPI[] = [];
  let reportPreview: KPIReportPreview | null = null;

  $: maxUserTotal = userKPIList.length > 0 ? Math.max(...userKPIList.map(u => u.total_completed), 1) : 1;
  $: maxDeptTotal = deptKPIList.length > 0 ? Math.max(...deptKPIList.map(d => d.total_completed), 1) : 1;
  $: totalActiveOverdue = userKPIList.reduce((sum, item) => sum + (item.active_overdue || 0), 0);
  $: totalOverdueCompleted = userKPIList.reduce((sum, item) => sum + (item.overdue_completed || 0), 0);
  $: totalMRCount = userKPIList.reduce((sum, item) => sum + (item.mr_count || 0), 0);
  $: visibleReportEvidence = reportPreview?.evidence?.slice(0, 6) || [];
  $: focusedPersonalSection = reportPreview?.personal_sections?.[0] || null;
  $: reportUserLabel = selectedReportUser || '团队视图';

  function authHeaders() {
    const token = localStorage.getItem('jwt_token');
    return {
      'Authorization': `Bearer ${token || ''}`
    };
  }

  async function fetchKPIData() {
    loading = true;
    errorMsg = '';

    try {
      const res = await fetch(`/api/kpi/performance?period=${activePeriod}`, {
        headers: authHeaders()
      });

      if (!res.ok) {
        if (res.status === 403) {
          throw new Error('权限不足，只有管理员组有权查看 KPI 绩效看板');
        }
        throw new Error(`加载绩效数据失败: ${res.statusText}`);
      }

      const data = await res.json();
      summary = data.summary || { ...emptySummary };
      userKPIList = data.user_kpi || [];
      deptKPIList = data.department_kpi || [];
      await fetchReportPreview();
    } catch (err: any) {
      errorMsg = err.message || '获取绩效数据请求失败';
      console.error('KPI Fetch Error:', err);
    } finally {
      loading = false;
    }
  }

  async function fetchReportPreview() {
    reportLoading = true;
    reportErrorMsg = '';

    try {
      const params = new URLSearchParams({
        period: activePeriod,
        type: reportType
      });
      if (selectedReportUser) {
        params.set('user', selectedReportUser);
      }

      const res = await fetch(`/api/kpi/report-preview?${params.toString()}`, {
        headers: authHeaders()
      });

      if (!res.ok) {
        if (res.status === 403) {
          throw new Error('权限不足，无法生成 KPI 报告预览');
        }
        throw new Error(`加载报告预览失败: ${res.statusText}`);
      }

      reportPreview = await res.json();
    } catch (err: any) {
      reportErrorMsg = err.message || '获取报告预览请求失败';
      console.error('KPI Report Preview Fetch Error:', err);
    } finally {
      reportLoading = false;
    }
  }

  function changePeriod(p: string) {
    activePeriod = p;
    fetchKPIData();
  }

  function changeReportType(t: string) {
    reportType = t;
    fetchReportPreview();
  }

  function changeReportUser(user: string) {
    selectedReportUser = user;
    showReportUserDropdown = false;
    fetchReportPreview();
  }

  function toggleReportUserDropdown() {
    showReportUserDropdown = !showReportUserDropdown;
  }

  function handleDocumentClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (showReportUserDropdown && reportUserSelectEl && !reportUserSelectEl.contains(target)) {
      showReportUserDropdown = false;
    }
  }

  function formatMetric(value: number | undefined, digits = 0) {
    const safeValue = value || 0;
    return digits > 0 ? safeValue.toFixed(digits) : String(safeValue);
  }

  function riskTone(level: string) {
    if (level === 'high') return 'high';
    if (level === 'medium') return 'medium';
    return 'low';
  }

  onMount(() => {
    fetchKPIData();
    document.addEventListener('click', handleDocumentClick);
    return () => {
      document.removeEventListener('click', handleDocumentClick);
    };
  });
</script>

<div class="kpi-dashboard font-sans">
  <div class="kpi-header">
    <div>
      <span class="eyebrow">TEAM PERFORMANCE METRICS</span>
      <h2>📈 KPI 研发绩效大盘</h2>
    </div>

    <div class="period-selector font-mono">
      <button class="period-btn {activePeriod === 'day' ? 'active' : ''}" on:click={() => changePeriod('day')}>
        近 24 小时
      </button>
      <button class="period-btn {activePeriod === 'week' ? 'active' : ''}" on:click={() => changePeriod('week')}>
        近 7 天
      </button>
      <button class="period-btn {activePeriod === 'month' ? 'active' : ''}" on:click={() => changePeriod('month')}>
        近 30 天
      </button>
      <button class="period-btn {activePeriod === 'year' ? 'active' : ''}" on:click={() => changePeriod('year')}>
        近一年
      </button>
    </div>
  </div>

  {#if loading}
    <div class="state-msg">正在实时统计团队绩效遥测指标...</div>
  {:else if errorMsg}
    <div class="state-msg error-msg font-mono">❌ {errorMsg}</div>
  {:else}
    <div class="summary-cards">
      <div class="summary-card total-glow">
        <span class="card-icon">🏆</span>
        <div class="card-info">
          <span class="card-label">总计已交付</span>
          <span class="card-val text-indigo font-mono">{summary.total_completed}</span>
        </div>
      </div>
      <div class="summary-card task-glow">
        <span class="card-icon">🚀</span>
        <div class="card-info">
          <span class="card-label">需求已交付</span>
          <span class="card-val text-blue font-mono">{summary.tasks_completed}</span>
        </div>
      </div>
      <div class="summary-card demand-glow">
        <span class="card-icon">📋</span>
        <div class="card-info">
          <span class="card-label">大需求已交付</span>
          <span class="card-val text-purple font-mono">{summary.demands_completed}</span>
        </div>
      </div>
      <div class="summary-card bug-glow">
        <span class="card-icon">🪲</span>
        <div class="card-info">
          <span class="card-label">Bug已解决</span>
          <span class="card-val text-rose font-mono">{summary.bugs_completed}</span>
        </div>
      </div>
      <div class="summary-card risk-glow">
        <span class="card-icon">⚠️</span>
        <div class="card-info">
          <span class="card-label">当前超期</span>
          <span class="card-val text-amber font-mono">{totalActiveOverdue}</span>
        </div>
      </div>
      <div class="summary-card mr-glow">
        <span class="card-icon">🔁</span>
        <div class="card-info">
          <span class="card-label">关联 MR</span>
          <span class="card-val text-teal font-mono">{totalMRCount}</span>
        </div>
      </div>
    </div>

    <div class="report-panel glass-panel">
      <div class="report-header">
        <div>
          <h3>日报/周报预览</h3>
          <span class="panel-subtitle">基于任务、超期、评审和 MR 的事实预览</span>
        </div>

        <div class="report-controls">
          <div class="period-selector compact font-mono">
            <button class="period-btn {reportType === 'daily' ? 'active' : ''}" on:click={() => changeReportType('daily')}>
              日报
            </button>
            <button class="period-btn {reportType === 'weekly' ? 'active' : ''}" on:click={() => changeReportType('weekly')}>
              周报
            </button>
          </div>

          <div class="report-select-container font-mono" bind:this={reportUserSelectEl}>
            <button
              type="button"
              class="report-select-trigger"
              on:click={toggleReportUserDropdown}
              aria-label="报告视角筛选"
              aria-expanded={showReportUserDropdown}
            >
              <span>{reportUserLabel}</span>
              <span class="report-select-arrow">{showReportUserDropdown ? '▲' : '▼'}</span>
            </button>
            {#if showReportUserDropdown}
              <div class="report-select-options">
                <button
                  type="button"
                  class="report-select-option {selectedReportUser === '' ? 'active' : ''}"
                  on:click={() => changeReportUser('')}
                >
                  团队视图
                </button>
                {#each userKPIList as user}
                  <button
                    type="button"
                    class="report-select-option {selectedReportUser === user.name ? 'active' : ''}"
                    on:click={() => changeReportUser(user.name)}
                  >
                    {user.name}
                  </button>
                {/each}
              </div>
            {/if}
          </div>

          <button class="refresh-btn font-mono" on:click={fetchReportPreview} disabled={reportLoading}>
            {reportLoading ? '生成中' : '刷新'}
          </button>
        </div>
      </div>

      {#if reportErrorMsg}
        <div class="inline-error font-mono">{reportErrorMsg}</div>
      {:else if reportLoading && !reportPreview}
        <div class="report-loading">
          <div class="skeleton-line wide"></div>
          <div class="skeleton-grid">
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
          </div>
        </div>
      {:else if reportPreview}
        <div class="report-overview-strip font-mono">
          <div>
            <span>范围</span>
            <strong>{reportPreview.overview.period_label}</strong>
          </div>
          <div>
            <span>交付</span>
            <strong>{reportPreview.overview.total_completed}</strong>
          </div>
          <div>
            <span>当前超期</span>
            <strong class="text-amber">{reportPreview.overview.active_overdue}</strong>
          </div>
          <div>
            <span>延期完成</span>
            <strong class="text-rose">{reportPreview.overview.overdue_completed}</strong>
          </div>
          <div>
            <span>MR</span>
            <strong class="text-teal">{reportPreview.overview.mr_count}</strong>
          </div>
        </div>

        <div class="report-summary">
          <strong>{reportPreview.overview.title}</strong>
          <span>{reportPreview.overview.summary}</span>
        </div>

        <div class="report-grid">
          <section class="report-block">
            <div class="report-block-header">
              <h4>个人绩效分析</h4>
              <span class="font-mono">{reportPreview.personal_sections.length} 人</span>
            </div>
            {#if focusedPersonalSection}
              <div class="personal-focus">
                <div class="focus-title">
                  <strong>{focusedPersonalSection.user.name}</strong>
                  <span>{focusedPersonalSection.user.department || '未分配'}</span>
                </div>
                {#each focusedPersonalSection.highlights as highlight}
                  <p>{highlight}</p>
                {/each}
                {#if focusedPersonalSection.risk_notes.length > 0}
                  <div class="risk-note-list">
                    {#each focusedPersonalSection.risk_notes as note}
                      <span>{note}</span>
                    {/each}
                  </div>
                {/if}
              </div>
            {:else}
              <div class="empty-msg font-mono">暂无个人报告数据</div>
            {/if}
          </section>

          <section class="report-block">
            <div class="report-block-header">
              <h4>风险点</h4>
              <span class="font-mono">{reportPreview.risks.length} 项</span>
            </div>
            {#if reportPreview.risks.length === 0}
              <div class="empty-msg font-mono">暂无超期或延期风险</div>
            {:else}
              <div class="compact-list">
                {#each reportPreview.risks.slice(0, 4) as risk}
                  <div class="compact-row">
                    <span class="risk-pill {riskTone(risk.level)} font-mono">{risk.level}</span>
                    <div>
                      <strong>{risk.title}</strong>
                      <p>{risk.detail}</p>
                      <span class="row-meta font-mono">{risk.owner} / {risk.department} / {risk.evidence_ids.join(', ')}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </section>

          <section class="report-block">
            <div class="report-block-header">
              <h4>会议关注项</h4>
              <span class="font-mono">{reportPreview.meeting_focus.length} 项</span>
            </div>
            {#if reportPreview.meeting_focus.length === 0}
              <div class="empty-msg font-mono">暂无需要升级到会议的关注项</div>
            {:else}
              <div class="compact-list">
                {#each reportPreview.meeting_focus.slice(0, 4) as focus}
                  <div class="compact-row">
                    <span class="focus-tag font-mono">{focus.topic}</span>
                    <div>
                      <strong>{focus.owner}</strong>
                      <p>{focus.reason}</p>
                      <span class="row-meta font-mono">{focus.department} / {focus.evidence_ids.join(', ')}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </section>

          <section class="report-block evidence-block">
            <div class="report-block-header">
              <h4>证据</h4>
              <span class="font-mono">{reportPreview.evidence.length} 条</span>
            </div>
            {#if visibleReportEvidence.length === 0}
              <div class="empty-msg font-mono">暂无可追溯证据</div>
            {:else}
              <div class="evidence-list">
                {#each visibleReportEvidence as item}
                  <div class="evidence-row">
                    <span class="evidence-id font-mono">{item.task_id}</span>
                    <span class="evidence-title">{item.title}</span>
                    <span class="row-meta font-mono">{item.status} / {item.assignee}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </section>
        </div>
      {/if}
    </div>

    <div class="kpi-grid">
      <div class="kpi-panel glass-panel">
        <div class="panel-header">
          <h3>👤 研发成员个人绩效分析</h3>
          <span class="panel-subtitle">交付量、周期、评审、MR 和超期风险综合观察</span>
        </div>

        <div class="ranking-list">
          {#if userKPIList.length === 0}
            <div class="empty-msg font-mono">该周期内暂无已交付的任务或 Bug</div>
          {:else}
            {#each userKPIList as item, index}
              <div class="ranking-item">
                <div class="rank-badge font-mono" class:rank-1={index === 0} class:rank-2={index === 1} class:rank-3={index === 2}>
                  {index + 1}
                </div>

                <div class="user-avatar-wrapper">
                  {#if item.avatar}
                    <img src={item.avatar} alt={item.name} class="user-avatar" />
                  {:else}
                    <div class="avatar-placeholder font-mono">{item.name.slice(0, 1).toUpperCase()}</div>
                  {/if}
                </div>

                <div class="item-meta">
                  <div class="meta-row">
                    <span class="name">{item.name}</span>
                    <span class="dept-tag font-mono">{item.department || '未分配'}</span>
                  </div>

                  <div class="progress-container">
                    <div class="kpi-progress-bar">
                      <div class="kpi-fill" style="width: {(item.total_completed / maxUserTotal) * 100}%"></div>
                    </div>

                    <div class="breakdown font-mono">
                      <span>需求: {item.tasks_completed}</span>
                      <span>大需求: {item.demands_completed}</span>
                      <span>故障: {item.bugs_completed}</span>
                      <strong class="total-score text-indigo">总计: {item.total_completed}</strong>
                    </div>

                    <div class="process-metrics font-mono">
                      <span>周期 {formatMetric(item.avg_cycle_days, 1)} 天</span>
                      <span>MR {item.mr_count || 0}</span>
                      <span>评审 {item.review_count || 0}</span>
                      <span class:metric-alert={(item.active_overdue || 0) + (item.overdue_completed || 0) > 0}>
                        超期 {(item.active_overdue || 0) + (item.overdue_completed || 0)}
                      </span>
                    </div>

                    {#if item.risk_notes && item.risk_notes.length > 0}
                      <div class="risk-notes">
                        {#each item.risk_notes.slice(0, 2) as note}
                          <span>{note}</span>
                        {/each}
                      </div>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <div class="kpi-panel glass-panel">
        <div class="panel-header">
          <h3>🏢 部门完成指标汇总</h3>
          <span class="panel-subtitle">部门整体研发吞吐量度量</span>
        </div>

        <div class="ranking-list">
          {#if deptKPIList.length === 0}
            <div class="empty-msg font-mono">该周期内暂无已交付的部门数据</div>
          {:else}
            {#each deptKPIList as item, index}
              <div class="ranking-item">
                <div class="rank-badge font-mono" class:rank-1={index === 0} class:rank-2={index === 1} class:rank-3={index === 2}>
                  {index + 1}
                </div>

                <div class="item-meta">
                  <div class="meta-row">
                    <span class="dept-title font-semibold">{item.department}</span>
                  </div>

                  <div class="progress-container">
                    <div class="kpi-progress-bar dept-bar">
                      <div class="kpi-fill dept-fill" style="width: {(item.total_completed / maxDeptTotal) * 100}%"></div>
                    </div>

                    <div class="breakdown font-mono">
                      <span>需求: {item.tasks_completed}</span>
                      <span>大需求: {item.demands_completed}</span>
                      <span>故障: {item.bugs_completed}</span>
                      <strong class="total-score text-teal">总计: {item.total_completed}</strong>
                    </div>
                  </div>
                </div>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .kpi-dashboard {
    display: flex;
    flex-direction: column;
    gap: 24px;
    margin-top: 10px;
    color: #e2e8f0;
  }

  .kpi-header,
  .report-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
  }

  .eyebrow {
    font-size: 0.65rem;
    font-weight: 800;
    color: #64748b;
    letter-spacing: 0.1em;
    display: block;
    margin-bottom: 6px;
    text-transform: uppercase;
  }

  .kpi-header h2 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 800;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    background-clip: text;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .period-selector {
    display: inline-flex;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 8px;
    padding: 3px;
    gap: 2px;
  }

  .period-selector.compact .period-btn {
    padding: 6px 12px;
  }

  .period-btn,
  .refresh-btn,
  .report-select-trigger {
    min-height: 32px;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 600;
  }

  .period-btn,
  .refresh-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    padding: 6px 16px;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .period-btn:hover,
  .refresh-btn:hover {
    color: #ffffff;
    background: rgba(99, 102, 241, 0.1);
  }

  .period-btn.active,
  .refresh-btn {
    background: #6366f1;
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  }

  .refresh-btn:disabled {
    cursor: wait;
    opacity: 0.7;
  }

  .report-select-container {
    position: relative;
    width: 156px;
    z-index: 20;
  }

  .report-select-trigger {
    width: 100%;
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    background: rgba(15, 23, 42, 0.72);
    border: 1px solid rgba(129, 140, 248, 0.2);
    color: #cbd5e1;
    padding: 0 10px 0 12px;
    cursor: pointer;
    outline: none;
    transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
  }

  .report-select-trigger span:first-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }

  .report-select-trigger:hover,
  .report-select-trigger:focus {
    border-color: rgba(129, 140, 248, 0.65);
    background: rgba(15, 23, 42, 0.92);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.14);
  }

  .report-select-arrow {
    flex: 0 0 auto;
    color: #818cf8;
    font-size: 0.58rem;
    line-height: 1;
  }

  .report-select-options {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    width: max-content;
    min-width: 156px;
    max-width: min(280px, 76vw);
    max-height: 220px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: #0f172a;
    border: 1px solid rgba(129, 140, 248, 0.25);
    border-radius: 8px;
    box-shadow: 0 18px 36px rgba(2, 6, 23, 0.62);
    padding: 4px;
    z-index: 120;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.25) transparent;
  }

  .report-select-option {
    width: 100%;
    border: none;
    background: transparent;
    color: #cbd5e1;
    border-radius: 5px;
    padding: 8px 10px;
    font-family: inherit;
    font-size: 0.75rem;
    line-height: 1.35;
    text-align: left;
    cursor: pointer;
    white-space: normal;
    word-break: keep-all;
  }

  .report-select-option:hover {
    background: rgba(99, 102, 241, 0.14);
    color: #ffffff;
  }

  .report-select-option.active {
    background: rgba(99, 102, 241, 0.9);
    color: #ffffff;
    font-weight: 700;
  }

  .summary-cards {
    display: grid;
    grid-template-columns: repeat(6, minmax(0, 1fr));
    gap: 12px;
  }

  .summary-card {
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 8px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    padding: 12px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
    transition: all 0.3s;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.4);
  }

  .summary-card:hover {
    transform: translateY(-2px);
  }

  .total-glow:hover { border-color: rgba(99, 102, 241, 0.4); box-shadow: 0 8px 24px rgba(99, 102, 241, 0.15); }
  .task-glow:hover { border-color: rgba(59, 130, 246, 0.4); box-shadow: 0 8px 24px rgba(59, 130, 246, 0.15); }
  .demand-glow:hover { border-color: rgba(168, 85, 247, 0.4); box-shadow: 0 8px 24px rgba(168, 85, 247, 0.15); }
  .bug-glow:hover { border-color: rgba(244, 63, 94, 0.4); box-shadow: 0 8px 24px rgba(244, 63, 94, 0.15); }
  .risk-glow:hover { border-color: rgba(245, 158, 11, 0.45); box-shadow: 0 8px 24px rgba(245, 158, 11, 0.12); }
  .mr-glow:hover { border-color: rgba(45, 212, 191, 0.4); box-shadow: 0 8px 24px rgba(45, 212, 191, 0.12); }

  .card-icon {
    font-size: 1.35rem;
  }

  .card-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .card-label {
    font-size: 0.72rem;
    color: #64748b;
    font-weight: 700;
  }

  .card-val {
    font-size: 1.35rem;
    font-weight: 800;
    line-height: 1;
  }

  .text-indigo { color: #818cf8; }
  .text-blue { color: #60a5fa; }
  .text-purple { color: #c084fc; }
  .text-rose { color: #f43f5e; }
  .text-teal { color: #2dd4bf; }
  .text-amber { color: #f59e0b; }

  .glass-panel {
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 16px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    padding: 22px;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.5);
  }

  .report-panel,
  .kpi-panel {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .report-header h3,
  .panel-header h3,
  .report-block-header h4 {
    margin: 0;
    font-weight: 700;
    color: #f1f5f9;
  }

  .report-header h3,
  .panel-header h3 {
    font-size: 1.1rem;
  }

  .report-block-header h4 {
    font-size: 0.9rem;
  }

  .panel-header,
  .report-block-header {
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    padding-bottom: 12px;
  }

  .panel-subtitle {
    font-size: 0.7rem;
    color: #64748b;
  }

  .report-controls {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .inline-error {
    border: 1px solid rgba(248, 113, 113, 0.25);
    background: rgba(127, 29, 29, 0.16);
    color: #fca5a5;
    border-radius: 10px;
    padding: 10px 12px;
    font-size: 0.75rem;
  }

  .report-loading {
    display: grid;
    gap: 12px;
  }

  .skeleton-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
  }

  .skeleton-line {
    height: 42px;
    border-radius: 10px;
    background: linear-gradient(90deg, rgba(30, 41, 59, 0.35), rgba(51, 65, 85, 0.55), rgba(30, 41, 59, 0.35));
    background-size: 180% 100%;
    animation: shimmer 1.4s ease infinite;
  }

  .skeleton-line.wide {
    height: 54px;
  }

  .report-overview-strip {
    display: grid;
    grid-template-columns: repeat(5, minmax(120px, 1fr));
    gap: 10px;
  }

  .report-overview-strip div {
    border: 1px solid rgba(51, 65, 85, 0.26);
    background: rgba(15, 23, 42, 0.48);
    border-radius: 10px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .report-overview-strip span,
  .row-meta {
    color: #64748b;
    font-size: 0.65rem;
  }

  .report-overview-strip strong {
    color: #e2e8f0;
    font-size: 0.95rem;
  }

  .report-summary {
    display: flex;
    flex-direction: column;
    gap: 6px;
    border-left: 3px solid rgba(129, 140, 248, 0.65);
    padding: 8px 0 8px 12px;
  }

  .report-summary strong {
    color: #f8fafc;
    font-size: 0.92rem;
  }

  .report-summary span,
  .compact-row p,
  .personal-focus p {
    margin: 0;
    color: #94a3b8;
    font-size: 0.78rem;
    line-height: 1.55;
  }

  .report-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
  }

  .report-block {
    border: 1px solid rgba(51, 65, 85, 0.26);
    background: rgba(15, 23, 42, 0.34);
    border-radius: 12px;
    padding: 14px;
    min-width: 0;
  }

  .report-block-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 12px;
  }

  .report-block-header span {
    color: #64748b;
    font-size: 0.68rem;
  }

  .personal-focus,
  .compact-list,
  .evidence-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .focus-title {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: center;
  }

  .focus-title strong {
    color: #f8fafc;
    font-size: 0.95rem;
  }

  .focus-title span {
    color: #818cf8;
    font-size: 0.72rem;
  }

  .risk-note-list,
  .risk-notes {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .risk-note-list span,
  .risk-notes span {
    background: rgba(245, 158, 11, 0.1);
    border: 1px solid rgba(245, 158, 11, 0.22);
    color: #fbbf24;
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 0.68rem;
  }

  .compact-row {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 10px;
    align-items: start;
  }

  .compact-row strong {
    color: #e2e8f0;
    font-size: 0.82rem;
  }

  .risk-pill,
  .focus-tag,
  .evidence-id {
    border-radius: 6px;
    padding: 4px 7px;
    font-size: 0.62rem;
    white-space: nowrap;
  }

  .risk-pill.high {
    background: rgba(248, 113, 113, 0.16);
    color: #fca5a5;
    border: 1px solid rgba(248, 113, 113, 0.25);
  }

  .risk-pill.medium {
    background: rgba(245, 158, 11, 0.14);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.25);
  }

  .risk-pill.low,
  .focus-tag {
    background: rgba(99, 102, 241, 0.14);
    color: #a5b4fc;
    border: 1px solid rgba(99, 102, 241, 0.25);
  }

  .evidence-row {
    display: grid;
    grid-template-columns: minmax(96px, 0.7fr) minmax(0, 1.4fr) minmax(92px, 0.7fr);
    gap: 8px;
    align-items: center;
    border-bottom: 1px solid rgba(51, 65, 85, 0.16);
    padding-bottom: 8px;
  }

  .evidence-row:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }

  .evidence-id {
    background: rgba(45, 212, 191, 0.1);
    color: #5eead4;
    border: 1px solid rgba(45, 212, 191, 0.2);
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .evidence-title {
    color: #cbd5e1;
    font-size: 0.74rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 450px), 1fr));
    gap: 24px;
  }

  .ranking-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-height: 560px;
    overflow-y: auto;
    padding-right: 6px;
  }

  .ranking-list::-webkit-scrollbar {
    width: 4px;
  }
  .ranking-list::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 2px;
  }

  .ranking-item {
    display: flex;
    align-items: flex-start;
    gap: 16px;
    background: rgba(30, 41, 59, 0.2);
    border: 1px solid rgba(51, 65, 85, 0.2);
    border-radius: 12px;
    padding: 12px 16px;
    transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .ranking-item:hover {
    background: rgba(51, 65, 85, 0.25);
    border-color: rgba(99, 102, 241, 0.25);
    transform: translateX(4px);
  }

  .rank-badge {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: rgba(51, 65, 85, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 700;
    color: #94a3b8;
    flex-shrink: 0;
    margin-top: 7px;
  }

  .rank-1 { background: #eab308; color: #020617; box-shadow: 0 0 10px rgba(234, 179, 8, 0.4); }
  .rank-2 { background: #94a3b8; color: #020617; }
  .rank-3 { background: #b45309; color: #ffffff; }

  .user-avatar-wrapper {
    flex-shrink: 0;
    margin-top: 1px;
  }

  .user-avatar,
  .avatar-placeholder {
    width: 38px;
    height: 38px;
    border-radius: 50%;
  }

  .user-avatar {
    border: 1px solid rgba(99, 102, 241, 0.3);
    object-fit: cover;
  }

  .avatar-placeholder {
    background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
    color: white;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    font-size: 1rem;
    border: 1px solid rgba(255, 255, 255, 0.2);
  }

  .item-meta {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;
  }

  .meta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .name {
    font-size: 0.85rem;
    font-weight: 600;
    color: #f8fafc;
  }

  .dept-tag {
    font-size: 0.65rem;
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
    padding: 2px 8px;
    border-radius: 4px;
    border: 1px solid rgba(99, 102, 241, 0.2);
    white-space: nowrap;
  }

  .dept-title {
    font-size: 0.85rem;
    color: #f1f5f9;
  }

  .progress-container {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }

  .kpi-progress-bar {
    height: 6px;
    background: rgba(15, 23, 42, 0.6);
    border-radius: 999px;
    overflow: hidden;
  }

  .kpi-fill {
    height: 100%;
    background: linear-gradient(90deg, #6366f1 0%, #a855f7 100%);
    border-radius: 999px;
    transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .dept-fill {
    background: linear-gradient(90deg, #14b8a6 0%, #06b6d4 100%);
  }

  .breakdown,
  .process-metrics {
    display: flex;
    gap: 12px;
    font-size: 0.65rem;
    color: #64748b;
    flex-wrap: wrap;
    align-items: center;
  }

  .process-metrics {
    color: #94a3b8;
  }

  .metric-alert {
    color: #f59e0b;
  }

  .total-score {
    margin-left: auto;
    font-size: 0.75rem;
  }

  .state-msg {
    padding: 80px 0;
    text-align: center;
    color: #64748b;
    font-size: 0.85rem;
  }

  .empty-msg {
    padding: 24px 0;
    text-align: center;
    color: #475569;
    font-size: 0.75rem;
  }

  .error-msg {
    color: #f87171;
  }

  @keyframes shimmer {
    0% { background-position: 180% 0; }
    100% { background-position: -180% 0; }
  }

  @media (prefers-reduced-motion: reduce) {
    .summary-card,
    .ranking-item,
    .kpi-fill,
    .period-btn,
    .refresh-btn {
      transition: none;
    }

    .summary-card:hover,
    .ranking-item:hover {
      transform: none;
    }

    .skeleton-line {
      animation: none;
    }
  }

  @media (max-width: 860px) {
    .report-overview-strip,
    .report-grid,
    .skeleton-grid {
      grid-template-columns: 1fr;
    }

    .report-controls {
      width: 100%;
    }

    .report-select-container,
    .refresh-btn,
    .period-selector {
      width: 100%;
    }

    .period-selector {
      justify-content: space-between;
    }
  }

  @media (max-width: 620px) {
    .summary-cards {
      grid-template-columns: 1fr;
    }

    .glass-panel {
      padding: 16px;
    }

    .ranking-item {
      gap: 10px;
      padding: 12px;
    }

    .evidence-row {
      grid-template-columns: 1fr;
    }

    .total-score {
      margin-left: 0;
    }
  }
</style>
