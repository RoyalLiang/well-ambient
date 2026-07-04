<script lang="ts">
  import { onMount } from 'svelte';

  type Tone = 'safe' | 'warn' | 'danger' | 'info' | 'muted';

  interface DeliveryHealth {
    score: number;
    label: string;
    evidence_completeness: number;
    open_exceptions: number;
    weekly_decision_count: number;
    ai_traceability: number;
    schedule_high_risk: number;
    signals?: string[];
  }

  interface DeliveryEvidence {
    total_requirements: number;
    complete_chains: number;
    incomplete_chains: number;
    average_completeness: number;
    jira_code_mismatch: number;
    missing_code_evidence: number;
    merged_but_status_open: number;
    top_missing_links?: string[];
    requirement_completeness?: RequirementEvidence[];
    recent_evidence?: EvidenceDigest[];
  }

  interface RequirementEvidence {
    demand_id: string;
    title: string;
    assignee: string;
    project: string;
    chain_status: string;
    completeness: number;
    missing_links?: string[];
    evidence_refs?: string[];
    recommended_next: string;
  }

  interface EvidenceDigest {
    task_id: string;
    task_group_id?: string;
    commit_count: number;
    mr_count: number;
    last_evidence: string;
    signals?: string[];
    evidence_refs?: string[];
    missing_links?: string[];
    chain_status?: string;
    evidence_completeness?: number;
  }

  interface DeliveryExceptions {
    total: number;
    p0: number;
    p1: number;
    p2: number;
    items?: DeliveryException[];
  }

  interface DeliveryException {
    id: string;
    type: string;
    severity: string;
    title: string;
    reason: string;
    decision_owner: string;
    deadline: string;
    recommended_action: string;
    evidence_refs?: string[];
    source: string;
  }

  interface WeeklyDecisionSummary {
    total: number;
    must_decide: number;
    this_week: number;
    decision_debt: number;
    items?: WeeklyDecisionItem[];
  }

  interface WeeklyDecisionItem {
    id: string;
    question: string;
    why_now: string;
    options?: string[];
    recommended_action: string;
    decision_owner: string;
    deadline: string;
    evidence_refs?: string[];
  }

  interface DeliverySchedule {
    total: number;
    scheduled: number;
    unscheduled: number;
    overdue: number;
    due_soon: number;
    stale: number;
    high_risk: number;
    risk_calendar_url?: string;
    upcoming?: ScheduleRiskEvent[];
  }

  interface ScheduleRiskEvent {
    demand_id: string;
    title: string;
    assignee: string;
    project_key: string;
    risk_type?: string;
    risk_level: string;
    risk_label: string;
    risk_reason: string;
    due_date: string;
    days_remaining: number;
  }

  interface AITraceSummary {
    total_outputs: number;
    traceable_outputs: number;
    traceability_percent: number;
    average_readiness: number;
    latest?: AITraceItem[];
  }

  interface AITraceItem {
    id: number;
    demand_id: string;
    task_group_id: string;
    context_pack_id: number;
    context_pack_summary: string;
    input_summary: string;
    model: string;
    prompt_template_version: string;
    rule_version: string;
    confidence: number;
    readiness_score: number;
    missing_questions?: string[];
    acceptance_criteria?: string[];
    risk_flags?: string[];
    human_feedback: string;
    created_at: string;
  }

  interface OverrideSummary {
    recent: number;
    protected: number;
    needs_review: number;
    audit_trail_url: string;
  }

  interface AuthorizationSummary {
    explain_panel_available: boolean;
    endpoint: string;
    recent_decisions: number;
    recent_denials: number;
    high_risk_grants: number;
  }

  interface EntryPoint {
    key: string;
    label: string;
    description: string;
    url: string;
    status: string;
  }

  interface DeliveryCockpitResponse {
    generated_at: string;
    north_star: string;
    health: DeliveryHealth;
    evidence: DeliveryEvidence;
    exceptions: DeliveryExceptions;
    weekly_decisions: WeeklyDecisionSummary;
    schedule: DeliverySchedule;
    ai_trace: AITraceSummary;
    override: OverrideSummary;
    authorization: AuthorizationSummary;
    entry_points: EntryPoint[];
  }

  interface MetricTile {
    label: string;
    value: string;
    sub: string;
    tone: Tone;
  }

  let cockpit: DeliveryCockpitResponse | null = null;
  let loading = true;
  let errorMsg = '';
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  $: health = cockpit?.health;
  $: evidence = cockpit?.evidence;
  $: exceptions = cockpit?.exceptions;
  $: weekly = cockpit?.weekly_decisions;
  $: schedule = cockpit?.schedule;
  $: aiTrace = cockpit?.ai_trace;
  $: override = cockpit?.override;
  $: authorization = cockpit?.authorization;
  $: topExceptions = (exceptions?.items || []).slice(0, 3);
  $: weeklyCards = (weekly?.items || []).slice(0, 3);
  $: weakestChains = (evidence?.requirement_completeness || []).slice(0, 3);
  $: latestTraces = (aiTrace?.latest || []).slice(0, 2);
  $: entryPoints = normalizeEntryPoints(cockpit?.entry_points || []);
  $: healthTone = getHealthTone(health?.label, health?.score);
  $: healthMetricTiles = buildHealthMetricTiles();

  onMount(() => {
    fetchCockpit();
    refreshTimer = setInterval(fetchCockpit, 60000);

    return () => {
      if (refreshTimer) clearInterval(refreshTimer);
    };
  });

  async function fetchCockpit() {
    if (!cockpit) loading = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/strongest-brain/delivery-cockpit');
      if (!res.ok) {
        throw new Error(`交付 cockpit 接口返回 ${res.status}`);
      }
      cockpit = await res.json();
    } catch (err: any) {
      errorMsg = err?.message || '交付 cockpit 数据加载失败';
    } finally {
      loading = false;
    }
  }

  function buildHealthMetricTiles(): MetricTile[] {
    return [
      {
        label: '证据链完整度',
        value: `${clampPercent(health?.evidence_completeness ?? evidence?.average_completeness ?? 0)}%`,
        sub: `${evidence?.complete_chains ?? 0}/${evidence?.total_requirements ?? 0} 条完整`,
        tone: percentTone(health?.evidence_completeness ?? evidence?.average_completeness ?? 0)
      },
      {
        label: '异常数',
        value: String(health?.open_exceptions ?? exceptions?.total ?? 0),
        sub: `P0 ${exceptions?.p0 ?? 0} / P1 ${exceptions?.p1 ?? 0} / P2 ${exceptions?.p2 ?? 0}`,
        tone: (exceptions?.p0 || 0) > 0 ? 'danger' : (exceptions?.p1 || 0) > 0 ? 'warn' : 'safe'
      },
      {
        label: '周会决策数',
        value: String(health?.weekly_decision_count ?? weekly?.total ?? 0),
        sub: `${weekly?.must_decide ?? 0} 个必须拍板`,
        tone: (weekly?.must_decide || 0) > 0 ? 'warn' : 'safe'
      },
      {
        label: 'AI 可追溯率',
        value: `${clampPercent(health?.ai_traceability ?? aiTrace?.traceability_percent ?? 0)}%`,
        sub: `${aiTrace?.traceable_outputs ?? 0}/${aiTrace?.total_outputs ?? 0} 次有 context pack`,
        tone: percentTone(health?.ai_traceability ?? aiTrace?.traceability_percent ?? 0)
      },
      {
        label: '排期风险',
        value: String(health?.schedule_high_risk ?? schedule?.high_risk ?? 0),
        sub: `${schedule?.overdue ?? 0} 逾期 / ${schedule?.stale ?? 0} 停滞`,
        tone: (schedule?.high_risk || 0) > 0 ? 'danger' : (schedule?.due_soon || 0) > 0 ? 'warn' : 'safe'
      }
    ];
  }

  function normalizeEntryPoints(items: EntryPoint[]): EntryPoint[] {
    if (items.length > 0) return items;
    return [
      { key: 'evidence_chain', label: '需求证据链', description: '查看 Jira、MR、commit、CI、部署和验收缺口', url: '/api/strongest-brain/evidence-chain?task_id={id}', status: 'reserved' },
      { key: 'risk_calendar', label: '风险日历', description: '查看截止日、停滞和临期风险', url: '/api/schedule/risk-calendar', status: 'reserved' },
      { key: 'ai_replay', label: 'AI 回放', description: '查看 context_pack_id、规则版本和置信度', url: '/api/strongest-brain/ai-traces', status: 'reserved' },
      { key: 'override_audit', label: 'Override', description: '查看人工调停、保护窗口和回滚线索', url: '/api/strongest-brain/override-audit', status: 'reserved' },
      { key: 'permission_explain', label: '权限解释', description: '解释用户能否执行某个操作', url: '/api/authz/explain', status: 'reserved' }
    ];
  }

  function clampPercent(value: number): number {
    if (!Number.isFinite(value)) return 0;
    return Math.max(0, Math.min(100, Math.round(value)));
  }

  function percentTone(value: number): Tone {
    const percent = clampPercent(value);
    if (percent >= 85) return 'safe';
    if (percent >= 65) return 'warn';
    return 'danger';
  }

  function getHealthTone(label = '', score = 100): Tone {
    const normalized = label.toLowerCase();
    if (normalized.includes('critical') || score < 60) return 'danger';
    if (normalized.includes('attention') || score < 80) return 'warn';
    return 'safe';
  }

  function severityTone(severity: string): Tone {
    if (severity === 'P0') return 'danger';
    if (severity === 'P1') return 'warn';
    return 'info';
  }

  function sourceLabel(source: string): string {
    switch (source) {
      case 'schedule':
        return '排期';
      case 'execution':
        return '执行证据';
      case 'context':
        return 'AI 上下文';
      default:
        return source || '系统';
    }
  }

  function statusLabel(status: string): string {
    if (status === 'available') return '已接入';
    if (status === 'reserved') return '预留';
    return status || '预留';
  }

  function formatDate(value: string): string {
    if (!value) return '待定';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
  }

  function firstText(values: string[] | undefined, fallback: string): string {
    return values && values.length > 0 ? values[0] : fallback;
  }
</script>

<section class="delivery-cockpit" aria-label="最强大脑交付 cockpit">
  <div class="cockpit-header">
    <div>
      <span class="eyebrow">STRONGEST BRAIN DELIVERY</span>
      <h2>最强大脑交付 cockpit</h2>
      <p>{cockpit?.north_star || '系统维护事实，人处理判断'}</p>
    </div>
    <div class="cockpit-refresh">
      {#if cockpit?.generated_at}
        <span class="generated-at font-mono">生成 {cockpit.generated_at}</span>
      {/if}
      <button type="button" class="refresh-action font-mono" on:click={fetchCockpit} disabled={loading}>
        {loading ? '同步中' : '刷新'}
      </button>
    </div>
  </div>

  {#if loading && !cockpit}
    <div class="cockpit-state-grid" aria-live="polite">
      {#each Array(4) as _}
        <div class="state-skeleton"></div>
      {/each}
    </div>
  {:else if errorMsg && !cockpit}
    <div class="cockpit-state error-state">
      <strong>交付 cockpit 暂不可用</strong>
      <span>{errorMsg}</span>
      <button type="button" on:click={fetchCockpit}>重试</button>
    </div>
  {:else if !cockpit}
    <div class="cockpit-state empty-state">
      <strong>暂无交付事实</strong>
      <span>等待需求、排期、证据链或 AI 解构数据进入系统。</span>
    </div>
  {:else}
    {#if errorMsg}
      <div class="cockpit-inline-warning font-mono">{errorMsg}，当前保留上一次快照。</div>
    {/if}

    <div class="health-strip">
      <div class="health-score tone-{healthTone}">
        <span class="score-label">DELIVERY HEALTH</span>
        <strong>{health?.score ?? 0}</strong>
        <span>{health?.label || 'UNKNOWN'}</span>
      </div>

      <div class="metric-strip">
        {#each healthMetricTiles as tile}
          <div class="metric-tile tone-{tile.tone}">
            <span>{tile.label}</span>
            <strong>{tile.value}</strong>
            <small>{tile.sub}</small>
          </div>
        {/each}
      </div>
    </div>

    <div class="cockpit-grid">
      <section class="cockpit-panel exception-panel">
        <div class="panel-title-row">
          <div>
            <span class="eyebrow">EXCEPTION CENTER</span>
            <h3>异常中心摘要</h3>
          </div>
          <div class="severity-counts font-mono">
            <span class="sev-p0">P0 {exceptions?.p0 ?? 0}</span>
            <span class="sev-p1">P1 {exceptions?.p1 ?? 0}</span>
            <span class="sev-p2">P2 {exceptions?.p2 ?? 0}</span>
          </div>
        </div>

        {#if topExceptions.length === 0}
          <div class="compact-empty">当前没有需要打断人的异常。</div>
        {:else}
          <div class="exception-list">
            {#each topExceptions as item}
              <article class="exception-row tone-{severityTone(item.severity)}">
                <div class="row-main">
                  <div class="row-kicker font-mono">
                    <span>{item.severity}</span>
                    <span>{sourceLabel(item.source)}</span>
                    <span>{formatDate(item.deadline)}</span>
                  </div>
                  <h4>{item.title}</h4>
                  <p>{item.reason}</p>
                </div>
                <div class="row-aside">
                  <span>{item.decision_owner || '未指定'}</span>
                  <small>{firstText(item.evidence_refs, item.recommended_action || '等待证据回流')}</small>
                </div>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <section class="cockpit-panel weekly-panel">
        <div class="panel-title-row">
          <div>
            <span class="eyebrow">WEEKLY DECISIONS</span>
            <h3>周会必须决策</h3>
          </div>
          <span class="panel-count font-mono">{weekly?.must_decide ?? 0} MUST</span>
        </div>

        {#if weeklyCards.length === 0}
          <div class="compact-empty">本周暂无必须拍板问题。</div>
        {:else}
          <div class="weekly-cards">
            {#each weeklyCards as item}
              <article class="weekly-card">
                <div class="weekly-card-head">
                  <span class="font-mono">{formatDate(item.deadline)}</span>
                  <strong>{item.decision_owner || '未指定'}</strong>
                </div>
                <h4>{item.question}</h4>
                <p>{item.why_now}</p>
                <div class="option-row">
                  {#each (item.options || []).slice(0, 3) as option}
                    <span>{option}</span>
                  {/each}
                </div>
                <small>{item.recommended_action}</small>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <section class="cockpit-panel evidence-panel">
        <div class="panel-title-row">
          <div>
            <span class="eyebrow">EVIDENCE GRAPH</span>
            <h3>证据链缺口</h3>
          </div>
          <span class="panel-count font-mono">{evidence?.incomplete_chains ?? 0} INCOMPLETE</span>
        </div>

        <div class="evidence-summary">
          <div>
            <span>Jira-Code 不一致</span>
            <strong>{evidence?.jira_code_mismatch ?? 0}</strong>
          </div>
          <div>
            <span>缺少代码证据</span>
            <strong>{evidence?.missing_code_evidence ?? 0}</strong>
          </div>
          <div>
            <span>状态未回写</span>
            <strong>{evidence?.merged_but_status_open ?? 0}</strong>
          </div>
        </div>

        {#if weakestChains.length === 0}
          <div class="compact-empty">尚无需求证据链样本。</div>
        {:else}
          <div class="chain-list">
            {#each weakestChains as item}
              <article class="chain-row">
                <div class="chain-meter" style={`--pct: ${clampPercent(item.completeness)}%`}>
                  <span></span>
                </div>
                <div>
                  <div class="chain-title">
                    <strong>{item.demand_id}</strong>
                    <span>{item.assignee || '未指派'}</span>
                  </div>
                  <p>{item.title}</p>
                  <small>{item.recommended_next || firstText(item.missing_links, '等待证据回流')}</small>
                </div>
                <span class="chain-score font-mono">{clampPercent(item.completeness)}%</span>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <section class="cockpit-panel trace-panel">
        <div class="panel-title-row">
          <div>
            <span class="eyebrow">TRACE / CONTROL</span>
            <h3>AI 回放与调停状态</h3>
          </div>
          <span class="panel-count font-mono">{aiTrace?.average_readiness ?? 0}% READY</span>
        </div>

        <div class="control-grid">
          <div>
            <span>AI 输出</span>
            <strong>{aiTrace?.total_outputs ?? 0}</strong>
            <small>{aiTrace?.traceable_outputs ?? 0} 次可回放</small>
          </div>
          <div>
            <span>Override</span>
            <strong>{override?.recent ?? 0}</strong>
            <small>{override?.protected ?? 0} 个保护中</small>
          </div>
          <div>
            <span>权限解释</span>
            <strong>{authorization?.recent_denials ?? 0}</strong>
            <small>{authorization?.explain_panel_available ? '解释 API 已接入' : '待接入'}</small>
          </div>
        </div>

        {#if latestTraces.length > 0}
          <div class="trace-list">
            {#each latestTraces as item}
              <article>
                <span class="font-mono">CTX-{item.context_pack_id || 'NA'}</span>
                <p>{item.input_summary || item.context_pack_summary || '无输入摘要'}</p>
                <small>{item.model} / {item.prompt_template_version} / {clampPercent(item.confidence * 100)}%</small>
              </article>
            {/each}
          </div>
        {:else}
          <div class="compact-empty">暂无可回放的 AI 解构记录。</div>
        {/if}
      </section>
    </div>

    <div class="entry-strip">
      {#each entryPoints as entry}
        <div class="entry-chip">
          <div>
            <span class="font-mono">{statusLabel(entry.status)}</span>
            <strong>{entry.label}</strong>
            <small>{entry.description}</small>
          </div>
          <code>{entry.url}</code>
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .delivery-cockpit {
    display: flex;
    flex-direction: column;
    gap: 16px;
    color: #e2e8f0;
    background:
      linear-gradient(180deg, rgba(15, 23, 42, 0.76), rgba(8, 13, 25, 0.7)),
      linear-gradient(90deg, rgba(14, 165, 233, 0.08), rgba(16, 185, 129, 0.05), rgba(244, 63, 94, 0.06));
    border: 1px solid rgba(51, 65, 85, 0.42);
    border-radius: 16px;
    padding: 22px;
    box-shadow: 0 14px 42px rgba(0, 0, 0, 0.36);
  }

  .cockpit-header,
  .panel-title-row,
  .health-strip,
  .weekly-card-head,
  .chain-title {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 14px;
  }

  .cockpit-header h2,
  .panel-title-row h3 {
    margin: 0;
    color: #f8fafc;
  }

  .cockpit-header h2 {
    font-size: 1.35rem;
    letter-spacing: 0;
  }

  .cockpit-header p {
    margin: 7px 0 0;
    color: #94a3b8;
    font-size: 0.84rem;
  }

  .cockpit-refresh {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .generated-at {
    color: #64748b;
    font-size: 0.7rem;
  }

  .refresh-action,
  .cockpit-state button {
    border: 1px solid rgba(56, 189, 248, 0.28);
    background: rgba(8, 47, 73, 0.34);
    color: #bae6fd;
    border-radius: 8px;
    padding: 7px 11px;
    font-size: 0.72rem;
    font-weight: 800;
    cursor: pointer;
  }

  .refresh-action:disabled {
    cursor: wait;
    opacity: 0.68;
  }

  .cockpit-state-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .state-skeleton {
    min-height: 132px;
    border-radius: 12px;
    background: linear-gradient(90deg, rgba(30, 41, 59, 0.4), rgba(51, 65, 85, 0.52), rgba(30, 41, 59, 0.4));
    background-size: 200% 100%;
    animation: skeletonShift 1.4s ease-in-out infinite;
  }

  .cockpit-state {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    border: 1px solid rgba(51, 65, 85, 0.45);
    border-radius: 12px;
    padding: 16px;
    background: rgba(2, 6, 23, 0.42);
  }

  .cockpit-state strong {
    color: #f8fafc;
  }

  .cockpit-state span,
  .compact-empty {
    color: #94a3b8;
    font-size: 0.82rem;
  }

  .error-state {
    border-color: rgba(244, 63, 94, 0.32);
  }

  .cockpit-inline-warning {
    color: #fbbf24;
    background: rgba(120, 53, 15, 0.18);
    border: 1px solid rgba(245, 158, 11, 0.25);
    border-radius: 10px;
    padding: 9px 11px;
    font-size: 0.72rem;
  }

  .health-strip {
    align-items: stretch;
  }

  .health-score {
    width: 220px;
    min-height: 132px;
    border-radius: 12px;
    padding: 15px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    border: 1px solid rgba(51, 65, 85, 0.45);
    background: rgba(2, 6, 23, 0.42);
  }

  .score-label {
    color: #64748b;
    font-size: 0.64rem;
    font-weight: 900;
    letter-spacing: 0.08em;
  }

  .health-score strong {
    color: #f8fafc;
    font-size: 2.8rem;
    line-height: 0.95;
  }

  .health-score span:last-child {
    font-size: 0.72rem;
    font-weight: 900;
  }

  .metric-strip {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 10px;
  }

  .metric-tile,
  .cockpit-panel,
  .entry-chip {
    border: 1px solid rgba(51, 65, 85, 0.38);
    background: rgba(2, 6, 23, 0.36);
    border-radius: 12px;
  }

  .metric-tile {
    min-height: 132px;
    padding: 13px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    min-width: 0;
  }

  .metric-tile span,
  .metric-tile small,
  .control-grid span,
  .control-grid small,
  .entry-chip small,
  .chain-row small,
  .trace-list small,
  .weekly-card small {
    color: #94a3b8;
    line-height: 1.35;
  }

  .metric-tile span {
    font-size: 0.72rem;
    font-weight: 800;
  }

  .metric-tile strong {
    color: #f8fafc;
    font-size: 1.62rem;
    line-height: 1;
  }

  .cockpit-grid {
    display: grid;
    grid-template-columns: minmax(0, 1.08fr) minmax(0, 0.92fr);
    gap: 14px;
  }

  .cockpit-panel {
    min-height: 286px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
  }

  .panel-title-row {
    align-items: center;
  }

  .panel-title-row h3 {
    font-size: 1rem;
  }

  .panel-count {
    color: #cbd5e1;
    background: rgba(15, 23, 42, 0.72);
    border: 1px solid rgba(71, 85, 105, 0.45);
    border-radius: 8px;
    padding: 5px 8px;
    font-size: 0.68rem;
    white-space: nowrap;
  }

  .severity-counts {
    display: flex;
    gap: 7px;
    flex-wrap: wrap;
    justify-content: flex-end;
    font-size: 0.68rem;
    font-weight: 900;
  }

  .severity-counts span {
    border-radius: 7px;
    padding: 5px 7px;
    border: 1px solid rgba(51, 65, 85, 0.4);
  }

  .sev-p0 { color: #fecdd3; background: rgba(127, 29, 29, 0.28); }
  .sev-p1 { color: #fde68a; background: rgba(120, 53, 15, 0.24); }
  .sev-p2 { color: #bae6fd; background: rgba(8, 47, 73, 0.24); }

  .exception-list,
  .weekly-cards,
  .chain-list,
  .trace-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .exception-row,
  .weekly-card,
  .chain-row,
  .trace-list article {
    background: rgba(15, 23, 42, 0.56);
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 10px;
    padding: 11px;
    min-width: 0;
  }

  .exception-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 164px;
    gap: 12px;
  }

  .row-kicker {
    display: flex;
    flex-wrap: wrap;
    gap: 7px;
    color: #64748b;
    font-size: 0.64rem;
    font-weight: 900;
  }

  .exception-row h4,
  .weekly-card h4 {
    margin: 6px 0 5px;
    color: #f8fafc;
    font-size: 0.88rem;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .exception-row p,
  .weekly-card p,
  .chain-row p,
  .trace-list p {
    margin: 0;
    color: #cbd5e1;
    font-size: 0.78rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .row-aside {
    display: flex;
    flex-direction: column;
    gap: 8px;
    align-items: flex-end;
    text-align: right;
    min-width: 0;
  }

  .row-aside span {
    color: #e2e8f0;
    font-size: 0.76rem;
    font-weight: 800;
  }

  .row-aside small {
    color: #94a3b8;
    font-size: 0.7rem;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .weekly-card-head span,
  .weekly-card-head strong {
    font-size: 0.72rem;
  }

  .weekly-card-head span {
    color: #38bdf8;
  }

  .weekly-card-head strong {
    color: #cbd5e1;
  }

  .option-row {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 9px 0;
  }

  .option-row span {
    color: #bae6fd;
    background: rgba(8, 47, 73, 0.32);
    border: 1px solid rgba(56, 189, 248, 0.18);
    border-radius: 999px;
    padding: 3px 7px;
    font-size: 0.68rem;
    font-weight: 700;
  }

  .evidence-summary,
  .control-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .evidence-summary div,
  .control-grid div {
    display: flex;
    flex-direction: column;
    gap: 5px;
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 10px;
    padding: 10px;
    background: rgba(15, 23, 42, 0.45);
  }

  .evidence-summary span,
  .control-grid span {
    font-size: 0.68rem;
    font-weight: 800;
  }

  .evidence-summary strong,
  .control-grid strong {
    color: #f8fafc;
    font-size: 1.2rem;
  }

  .chain-row {
    display: grid;
    grid-template-columns: 4px minmax(0, 1fr) auto;
    gap: 10px;
    align-items: stretch;
  }

  .chain-meter {
    width: 4px;
    min-height: 58px;
    background: rgba(51, 65, 85, 0.58);
    border-radius: 999px;
    overflow: hidden;
    align-self: stretch;
    display: flex;
    align-items: flex-end;
  }

  .chain-meter span {
    width: 100%;
    height: var(--pct);
    background: linear-gradient(180deg, #22c55e, #38bdf8);
    border-radius: inherit;
  }

  .chain-title strong {
    color: #bae6fd;
    font-size: 0.78rem;
  }

  .chain-title span {
    color: #94a3b8;
    font-size: 0.72rem;
  }

  .chain-score {
    color: #f8fafc;
    font-size: 0.76rem;
    font-weight: 900;
  }

  .trace-list article {
    display: grid;
    grid-template-columns: 86px minmax(0, 1fr);
    gap: 8px 10px;
  }

  .trace-list span {
    color: #a7f3d0;
    font-size: 0.68rem;
    font-weight: 900;
  }

  .trace-list small {
    grid-column: 2;
    font-size: 0.68rem;
  }

  .entry-strip {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 10px;
  }

  .entry-chip {
    min-height: 112px;
    padding: 11px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 10px;
    min-width: 0;
  }

  .entry-chip span {
    color: #38bdf8;
    font-size: 0.62rem;
    font-weight: 900;
  }

  .entry-chip strong {
    display: block;
    color: #f8fafc;
    margin: 5px 0;
    font-size: 0.82rem;
  }

  .entry-chip code {
    color: #64748b;
    font-size: 0.64rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tone-safe { border-color: rgba(16, 185, 129, 0.36); }
  .tone-safe strong, .tone-safe.health-score span:last-child { color: #34d399; }
  .tone-warn { border-color: rgba(245, 158, 11, 0.38); }
  .tone-warn strong, .tone-warn.health-score span:last-child { color: #fbbf24; }
  .tone-danger { border-color: rgba(244, 63, 94, 0.42); }
  .tone-danger strong, .tone-danger.health-score span:last-child { color: #fb7185; }
  .tone-info { border-color: rgba(56, 189, 248, 0.32); }
  .tone-info strong { color: #38bdf8; }

  @keyframes skeletonShift {
    from { background-position: 100% 0; }
    to { background-position: -100% 0; }
  }

  @media (max-width: 1280px) {
    .health-strip {
      flex-direction: column;
    }

    .health-score {
      width: 100%;
      min-height: 104px;
    }

    .metric-strip,
    .entry-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .cockpit-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 760px) {
    .delivery-cockpit {
      padding: 16px;
    }

    .cockpit-header,
    .panel-title-row,
    .exception-row {
      grid-template-columns: 1fr;
      flex-direction: column;
      align-items: stretch;
    }

    .cockpit-refresh,
    .row-aside {
      align-items: flex-start;
      text-align: left;
      justify-content: flex-start;
    }

    .metric-strip,
    .entry-strip,
    .evidence-summary,
    .control-grid,
    .cockpit-state-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
