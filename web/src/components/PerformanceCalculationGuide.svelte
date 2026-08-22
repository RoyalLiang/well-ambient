<script lang="ts">
  import { onMount } from 'svelte';
  import Modal from './shared/Modal.svelte';

  interface RuntimeStatus {
    scheduler_enabled: boolean;
    scheduler_started: boolean;
    calculation_running: boolean;
    interval_minutes: number;
    assessment_window_days: number;
    retention_days: number;
    formula_version: string;
    evidence_coverage_gate: number;
    minimum_samples: number;
    minimum_exposure_days: number;
    busy_retry_attempts: number;
    busy_retry_delay_ms: number;
		publication_mode: 'shadow' | 'formal';
		demand_metrics_enabled: boolean;
		bug_metrics_enabled: boolean;
		code_metrics_enabled: boolean;
		jira_history_enabled: boolean;
		git_dedupe_enabled: boolean;
    last_started_at?: string;
    last_completed_at?: string;
    last_error?: string;
  }

  interface MetricDefinition {
    code: string;
    dimension: string;
    name: string;
    weight: number;
    direction: string;
    minimum_samples: number;
    point_2_boundary: number;
    point_3_boundary: number;
    point_4_boundary: number;
    point_5_boundary: number;
    core: boolean;
    source: string;
    rule: string;
    required_evidence: string;
  }

  interface FactorValue {
    label: string;
    value: number;
  }

  interface FactorGroup {
    name: string;
    purpose: string;
    values: FactorValue[];
  }

  interface ScoreLevel {
    level: string;
    range: string;
  }

  interface FormulaExplanation {
    final_score: string;
    delivery_weight: string;
    defect_loss: string;
    defect_score: string;
    publication_gate: string;
    responsibility_rule: string;
    levels: ScoreLevel[];
  }

  interface RunExplanation {
    run_id: string;
    trigger: string;
    status: string;
    formula_version: string;
    assessment_window_start: string;
    assessment_window_end: string;
    input_watermark: string;
    snapshot_count: number;
    retention_cutoff: string;
    error_message?: string;
    completed_at?: string;
    created_at: string;
  }

  interface SnapshotExplanation {
    id: number;
    run_id: string;
    subject_key: string;
    formula_version: string;
    assessment_window_start: string;
    assessment_window_end: string;
    input_watermark: string;
    input_digest: string;
    delivery_unit_count: number;
    bug_count: number;
		demand_delay_count: number;
		bug_reopen_count: number;
		commit_count: number;
		duplicate_commit_count: number;
    effective_sample_count: number;
    exposure_days: number;
    evidence_coverage: number;
    observed_score: number | null;
    final_score: number | null;
		delivery_score?: number | null;
		quality_score?: number | null;
    trend_adjustment: number;
    risk_penalty: number;
    leverage_bonus: number;
    rating_status: string;
    level: string;
    exclusions: string[];
    adjustments: AdjustmentResult[];
    created_at: string;
  }

  interface MetricResult {
    code: string;
    name: string;
    weight: number;
    available: boolean;
    sample_qualified: boolean;
    direction: string;
    minimum_samples: number;
    raw_ratio?: number;
    point_level?: number;
    score?: number;
    weighted_points?: number;
    evidence_count: number;
    evidence_refs?: string[];
    reason?: string;
		numerator?: number;
		denominator?: number;
		raw_unit?: string;
  }

  interface ItemFactor {
    work_item_id: string;
		origin_work_item_id?: string;
    kind: string;
    project_key?: string;
    size_points?: number;
    priority_factor?: number;
		demand_level_factor?: number;
		project_factor?: number;
    complexity_factor?: number;
    stage_factor?: number;
    role_factor?: number;
    responsibility_share?: number;
    delivery_weight?: number;
    contribution_weight?: number;
    severity_factor?: number;
    escape_factor?: number;
    release_method_factor?: number;
    rollback_impact_factor?: number;
    defect_responsibility_share?: number;
    bug_loss?: number;
		closure_multiplier?: number;
    fix_contribution_share?: number;
		ownership_segment_hours?: number;
    warnings?: string[];
  }

  interface SnapshotSource {
    evidence_key: string;
    revision: number;
    event_type: string;
    evidence_ref: string;
    source_system: string;
    source_record_id: string;
    work_item_id: string;
    project_key: string;
    severity?: string;
    escape_stage?: string;
    release_method?: string;
    rollback_impact?: string;
    reason_code?: string;
    responsibility_share?: number;
    outcome: number;
    weight: number;
    occurred_at: string;
    observed_at: string;
    payload_hash: string;
  }

  interface AdjustmentResult {
    type: string;
		name?: string;
    reason_code: string;
    value: number;
		raw_ratio?: number;
		numerator?: number;
		denominator?: number;
		raw_unit?: string;
		evidence_ref?: string;
		evidence_refs?: string[];
		reason?: string;
  }

  interface SnapshotDetail {
    generated_at: string;
    read_only: boolean;
    read_only_notice: string;
    snapshot: SnapshotExplanation & { metrics: MetricResult[] };
    item_factors: ItemFactor[];
    sources: SnapshotSource[];
  }

  interface EvidenceTypeCount {
    event_type: string;
    count: number;
  }

  interface EvidenceStats {
    active_count: number;
    revision_count: number;
		source_event_count: number;
    by_type: EvidenceTypeCount[];
    last_observed_at?: string;
		last_source_observed_at?: string;
  }

  interface PerformanceExplanation {
    generated_at: string;
    read_only: boolean;
    read_only_notice: string;
    runtime: RuntimeStatus;
    formula: FormulaExplanation;
    metrics: MetricDefinition[];
    factor_groups: FactorGroup[];
    latest_run: RunExplanation | null;
    recent_snapshots: SnapshotExplanation[];
    evidence: EvidenceStats;
  }

  const evidenceTypeLabels: Record<string, string> = {
    defect_exposure: '缺陷暴露',
    defect_attribution: '缺陷归责',
    bug_reopen_outcome: 'Bug 重开结果',
    change_rollback_outcome: '变更回滚结果',
    verified_improvement: '验证后改进',
    change_safety_outcome: '变更安全结果',
    predictability_outcome: '预测准确结果',
    trend_adjustment: '趋势修正',
    trust_risk_adjustment: '信任风险',
    leverage_adjustment: '系统杠杆'
  };

  let explanation: PerformanceExplanation | null = null;
  let loading = true;
  let refreshing = false;
  let errorMessage = '';
  let requestSerial = 0;
  let selectedSnapshot: SnapshotExplanation | null = null;
  let snapshotDetail: SnapshotDetail | null = null;
  let detailLoading = false;
  let detailError = '';
  let detailRequestSerial = 0;

  onMount(() => {
    void loadExplanation();
    const refreshTimer = window.setInterval(() => {
      if (!document.hidden) void loadExplanation(true);
    }, 60_000);
    return () => window.clearInterval(refreshTimer);
  });

  async function loadExplanation(background = false) {
    const currentRequest = ++requestSerial;
    if (background && explanation) {
      refreshing = true;
    } else {
      loading = true;
    }
    errorMessage = '';
    try {
      const response = await fetch('/api/performance/explanation?snapshot_limit=30', { cache: 'no-store' });
      if (!response.ok) {
				if (response.status === 403 && currentRequest === requestSerial) {
					explanation = null;
				}
        const message = response.status === 403
          ? '当前账号不是全局超级管理员，无法查看计算说明。'
          : `读取计算说明失败，服务返回 ${response.status}`;
        throw new Error(message);
      }
      const payload = await response.json() as PerformanceExplanation;
      if (currentRequest === requestSerial) {
        explanation = payload;
      }
    } catch (error) {
      if (currentRequest === requestSerial) {
        errorMessage = error instanceof Error ? error.message : '读取计算说明失败，请稍后重试。';
      }
    } finally {
      if (currentRequest === requestSerial) {
        loading = false;
        refreshing = false;
      }
    }
  }

  async function openSnapshotDetail(snapshot: SnapshotExplanation) {
    selectedSnapshot = snapshot;
    snapshotDetail = null;
    detailError = '';
    detailLoading = true;
    const currentRequest = ++detailRequestSerial;
    try {
		if (!snapshot.id) {
			throw new Error('当前服务尚未返回快照标识，请重启后端服务后重试。');
		}
      const response = await fetch(`/api/performance/snapshots/${snapshot.id}`, { cache: 'no-store' });
      if (!response.ok) {
		throw new Error(response.status === 404 ? '该评分快照不存在或已超过保留期。' : `读取评分详情失败，服务返回 ${response.status}`);
      }
      const payload = await response.json() as SnapshotDetail;
      if (currentRequest === detailRequestSerial) snapshotDetail = payload;
    } catch (error) {
      if (currentRequest === detailRequestSerial) {
        detailError = error instanceof Error ? error.message : '读取评分详情失败，请稍后重试。';
      }
    } finally {
      if (currentRequest === detailRequestSerial) detailLoading = false;
    }
  }

  function closeSnapshotDetail() {
    detailRequestSerial += 1;
    selectedSnapshot = null;
    snapshotDetail = null;
    detailLoading = false;
    detailError = '';
  }

  function formatDateTime(value?: string) {
    if (!value) return '暂无记录';
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    return new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
    }).format(parsed);
  }

  function formatPercent(value: number) {
    return `${Math.round(value * 10000) / 100}%`;
  }

  function formatNumber(value: number) {
    return Number.isInteger(value) ? String(value) : String(Math.round(value * 100) / 100);
  }

  function runStatusLabel(status?: string) {
    if (status === 'completed') return '已完成';
    if (status === 'failed') return '失败';
    return status || '暂无运行';
  }

  function ratingStatusLabel(status: string) {
		if (status === 'formal') return '正式评分';
		if (status === 'shadow') return '影子评分';
		return '证据不足';
  }

  function formalScoreLabel(snapshot: SnapshotExplanation) {
    return snapshot.final_score === null ? 'N/A' : snapshot.final_score.toFixed(2);
  }

  function referenceScoreLabel(snapshot: SnapshotExplanation) {
    return snapshot.observed_score === null ? 'N/A' : snapshot.observed_score.toFixed(2);
  }

	function componentScoreLabel(value: number | null | undefined, maximum: number) {
		return value == null ? 'N/A' : `${value.toFixed(2)} / ${maximum}`;
	}

	function riskPenaltyLabel(value: number) {
		return value > 0 ? `-${formatNumber(value)}` : '0';
	}

  function sampleQualificationLabel(metric: MetricResult) {
    if (!metric.available) return '无可计算证据';
    return metric.sample_qualified ? '样本达标' : '仅供参考';
  }

  function metricBoundaryLabel(metric: MetricDefinition) {
    const symbol = metric.direction === 'lower_is_better' ? '≤' : '≥';
    return `2分 ${symbol}${formatPercent(metric.point_2_boundary)} / 3分 ${symbol}${formatPercent(metric.point_3_boundary)} / 4分 ${symbol}${formatPercent(metric.point_4_boundary)} / 5分 ${symbol}${formatPercent(metric.point_5_boundary)}`;
  }

  function triggerLabel(trigger?: string) {
    if (trigger === 'startup') return '服务启动';
    if (trigger === 'schedule') return '定时滚动';
    if (trigger === 'run_once') return '单次后台运行';
    return trigger || '暂无';
  }

  function factorKindLabel(kind: string) {
    if (kind === 'requirement') return '需求交付';
    if (kind === 'bug') return 'Bug 修复';
    if (kind === 'defect_attribution') return '缺陷归责';
    return kind;
  }

  function factorFormula(factor: ItemFactor) {
    if (factor.kind === 'requirement') {
			return `规模 ${formatNumber(factor.size_points || 0)} × 需求等级 ${formatNumber(factor.demand_level_factor || 0)} × 项目 ${formatNumber(factor.project_factor || 0)} × 责任 ${formatNumber(factor.responsibility_share || 0)} = ${formatNumber(factor.contribution_weight || 0)}`;
    }
    if (factor.kind === 'defect_attribution') {
			const origin = factor.origin_work_item_id ? `来源需求 ${factor.origin_work_item_id} · ` : '';
	      return `${origin}严重度 ${formatNumber(factor.severity_factor || 0)} × 逃逸 ${formatNumber(factor.escape_factor || 0)} × 责任 ${formatNumber(factor.defect_responsibility_share || 0)} × 闭环 ${formatNumber(factor.closure_multiplier || 1)} = ${formatNumber(factor.bug_loss || 0)}`;
    }
    if (factor.kind === 'change_rollback_outcome') {
      return `发布 ${formatNumber(factor.release_method_factor || 0)} × 影响 ${formatNumber(factor.rollback_impact_factor || 0)} × 责任 ${formatNumber(factor.responsibility_share || 0)}`;
    }
    if (factor.kind === 'change_safety_outcome') {
      return `发布权重 ${formatNumber(factor.release_method_factor || 0)}`;
    }
		return `修复责任区间 ${formatNumber(factor.ownership_segment_hours || 0)} 小时；修复贡献 ${formatNumber(factor.fix_contribution_share || 0)}，不自动计入缺陷责任`;
  }

	function metricRawLabel(metric: MetricResult) {
		if (!metric.available || metric.raw_ratio === undefined) return '-';
		if (metric.numerator !== undefined && metric.denominator !== undefined) {
			return `${formatNumber(metric.numerator)} / ${formatNumber(metric.denominator)} · ${formatPercent(metric.raw_ratio)}`;
		}
		return formatPercent(metric.raw_ratio);
	}

	function adjustmentRawLabel(adjustment: AdjustmentResult) {
		if (adjustment.raw_ratio === undefined) return '-';
		if (adjustment.numerator !== undefined && adjustment.denominator !== undefined) {
			return `${formatNumber(adjustment.numerator)} / ${formatNumber(adjustment.denominator)} · ${formatNumber(adjustment.raw_ratio)}`;
		}
		return formatNumber(adjustment.raw_ratio);
	}

  function metricScoreLabel(metric: MetricResult) {
    return metric.available && metric.score !== undefined ? metric.score.toFixed(2) : '不可用';
  }
</script>

<section class="performance-guide wa-admin-section" aria-labelledby="performance-guide-title" aria-busy={loading || refreshing}>
  <header class="guide-heading">
    <div>
      <span class="section-kicker">PERSONNEL PERFORMANCE</span>
      <h1 id="performance-guide-title">人员绩效计算说明</h1>
      <p>仅展示后台已持久化的计算规则、证据门槛与最近结果。评分由后台按固定周期静默滚动更新。</p>
    </div>
    <button
      type="button"
      class="wa-admin-action secondary refresh-action"
      disabled={loading || refreshing}
      on:click={() => loadExplanation(true)}
    >
      {refreshing ? '读取中' : '刷新已保存结果'}
    </button>
  </header>

  {#if loading && !explanation}
    <div class="skeleton-grid" aria-label="正在读取计算说明">
      {#each Array(4) as _}
        <div class="skeleton-block"></div>
      {/each}
    </div>
    <div class="skeleton-panel"></div>
  {:else if errorMessage && !explanation}
    <section class="wa-admin-card state-panel error-state" role="alert">
      <strong>计算说明暂时无法读取</strong>
      <p>{errorMessage}</p>
      <button type="button" class="wa-admin-action secondary" on:click={() => loadExplanation()}>重新读取</button>
    </section>
  {:else if explanation}
    <div class="read-only-notice" role="note">
      <span class="notice-mark" aria-hidden="true">i</span>
      <span>{explanation.read_only_notice}</span>
      <time datetime={explanation.generated_at}>读取于 {formatDateTime(explanation.generated_at)}</time>
    </div>

    {#if errorMessage}
      <div class="refresh-warning" role="status">{errorMessage} 当前仍展示上一次成功读取的结果。</div>
    {/if}

    <div class="status-grid" aria-label="后台计算状态">
      <article class="wa-admin-card wa-admin-metric status-card">
        <span>后台计算</span>
        <strong class="metric-value" class:running={explanation.runtime.calculation_running}>
          {explanation.runtime.calculation_running ? '计算中' : explanation.runtime.scheduler_enabled ? '静默运行' : '未启用'}
        </strong>
        <small class="metric-note">{explanation.runtime.publication_mode === 'formal' ? '正式分发布' : '影子运行'} · {explanation.runtime.scheduler_started ? '调度器已启动' : '调度器未启动'}</small>
      </article>
      <article class="wa-admin-card wa-admin-metric status-card">
        <span>滚动周期</span>
        <strong class="metric-value">{formatNumber(explanation.runtime.interval_minutes)} 分钟</strong>
        <small class="metric-note">考核窗口 {formatNumber(explanation.runtime.assessment_window_days)} 天</small>
      </article>
      <article class="wa-admin-card wa-admin-metric status-card">
        <span>最近完成</span>
        <strong class="metric-value metric-time">{formatDateTime(explanation.runtime.last_completed_at)}</strong>
        <small class="metric-note">{explanation.latest_run ? `${triggerLabel(explanation.latest_run.trigger)} · ${runStatusLabel(explanation.latest_run.status)}` : '还没有持久化运行记录'}</small>
      </article>
      <article class="wa-admin-card wa-admin-metric status-card">
        <span>审计保留</span>
        <strong class="metric-value">{formatNumber(explanation.runtime.retention_days)} 天</strong>
        <small class="metric-note">运行、快照、证据修订与审计统一滚动</small>
      </article>
    </div>

    <div class="guide-layout">
      <main class="guide-main">
        <section class="wa-admin-card content-section" aria-labelledby="formula-title">
	          <div class="section-heading">
            <div>
              <span class="section-kicker">FORMULA</span>
              <h2 id="formula-title">总分与责任边界</h2>
            </div>
	          </div>
          <dl class="formula-list">
            <div><dt>最终得分</dt><dd>{explanation.formula.final_score}</dd></div>
            <div><dt>交付权重</dt><dd>{explanation.formula.delivery_weight}</dd></div>
            <div><dt>缺陷损失</dt><dd>{explanation.formula.defect_loss}</dd></div>
            <div><dt>缺陷指标分</dt><dd>{explanation.formula.defect_score}</dd></div>
          </dl>
          <p class="boundary-note">{explanation.formula.responsibility_rule}</p>
        </section>

        <section class="content-section" aria-labelledby="metric-title">
          <div class="section-heading compact-heading">
            <div>
              <span class="section-kicker">METRICS</span>
								<h2 id="metric-title">三项正式计算口径</h2>
            </div>
            <span class="section-count">总权重 100%</span>
          </div>
          <div class="wa-admin-table-shell metric-table-shell">
            <table class="wa-admin-table metric-table">
								<caption class="sr-only">人员绩效三项正式计算的权重、来源、计算规则与所需证据</caption>
              <thead>
                <tr>
                  <th scope="col">指标</th>
                  <th scope="col" class="numeric-column">权重</th>
                  <th scope="col" class="numeric-column">最低样本</th>
                  <th scope="col">2 / 3 / 4 / 5 分边界</th>
                  <th scope="col">事实来源</th>
                  <th scope="col">业务公式</th>
                  <th scope="col">正式证据要求</th>
                </tr>
              </thead>
              <tbody>
                {#each explanation.metrics as metric}
                  <tr>
                    <td><strong>{metric.code}</strong><span>{metric.name}{metric.core ? ' · 核心' : ''}</span></td>
                    <td class="numeric-column">{formatPercent(metric.weight)}</td>
                    <td class="numeric-column">{metric.minimum_samples}</td>
                    <td>{metricBoundaryLabel(metric)}</td>
                    <td><code>{metric.source}</code></td>
                    <td>{metric.rule}</td>
                    <td>{metric.required_evidence}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </section>

        <section class="wa-admin-card content-section factor-section" aria-labelledby="factor-title">
          <div class="section-heading compact-heading">
            <div>
              <span class="section-kicker">FACTORS</span>
              <h2 id="factor-title">需求与 Bug 计算系数</h2>
            </div>
	          <span class="section-count">缺失证据会降级或不计算，不以零分替代</span>
          </div>
          <div class="factor-grid">
            {#each explanation.factor_groups as group}
              <section class="factor-group">
                <header><h3>{group.name}</h3><p>{group.purpose}</p></header>
                <dl>
                  {#each group.values as factor}
                    <div><dt>{factor.label}</dt><dd>{formatNumber(factor.value)}</dd></div>
                  {/each}
                </dl>
              </section>
            {/each}
          </div>
        </section>
      </main>

      <aside class="wa-admin-card wa-admin-inspector guide-inspector" aria-label="证据门槛与运行审计">
        <section>
          <span class="section-kicker">PUBLICATION GATE</span>
          <h2>正式分发布门槛</h2>
          <p>{explanation.formula.publication_gate}</p>
          <dl class="inspector-facts">
            <div><dt>覆盖率门槛</dt><dd>{formatPercent(explanation.runtime.evidence_coverage_gate)}</dd></div>
            <div><dt>最小有效样本</dt><dd>{explanation.runtime.minimum_samples}</dd></div>
            <div><dt>最小暴露期</dt><dd>{explanation.runtime.minimum_exposure_days} 天</dd></div>
						<div><dt>当前发布方式</dt><dd>{explanation.runtime.publication_mode === 'formal' ? '正式发布' : '影子模式'}</dd></div>
          </dl>
        </section>

        <section>
          <span class="section-kicker">EVIDENCE LEDGER</span>
          <h2>正式证据账本</h2>
          <dl class="inspector-facts">
						<div><dt>正式归责事实</dt><dd>{explanation.evidence.active_count}</dd></div>
            <div><dt>累计修订记录</dt><dd>{explanation.evidence.revision_count}</dd></div>
						<div><dt>Jira 流转事实</dt><dd>{explanation.evidence.source_event_count}</dd></div>
						<div><dt>最近源事实入账</dt><dd>{formatDateTime(explanation.evidence.last_source_observed_at)}</dd></div>
          </dl>
          <ul class="evidence-types">
            {#each explanation.evidence.by_type as item}
              <li><span>{evidenceTypeLabels[item.event_type] || item.event_type}</span><strong>{item.count}</strong></li>
            {/each}
          </ul>
        </section>

        <section>
          <span class="section-kicker">ROLLING RETENTION</span>
          <h2>滚动与故障保护</h2>
          <dl class="inspector-facts">
						<div><dt>数据库最大尝试</dt><dd>{explanation.runtime.busy_retry_attempts} 次</dd></div>
            <div><dt>重试间隔</dt><dd>{explanation.runtime.busy_retry_delay_ms} ms</dd></div>
            <div><dt>最近输入水位线</dt><dd>{formatDateTime(explanation.latest_run?.input_watermark)}</dd></div>
						<div><dt>需求 / Bug / 代码</dt><dd>{explanation.runtime.demand_metrics_enabled ? '开' : '关'} / {explanation.runtime.bug_metrics_enabled ? '开' : '关'} / {explanation.runtime.code_metrics_enabled ? '开' : '关'}</dd></div>
						<div><dt>Jira 历史 / Git 指纹</dt><dd>{explanation.runtime.jira_history_enabled ? '开' : '关'} / {explanation.runtime.git_dedupe_enabled ? '开' : '关'}</dd></div>
          </dl>
          {#if explanation.runtime.last_error || explanation.latest_run?.error_message}
            <p class="runtime-error" role="status">{explanation.runtime.last_error || explanation.latest_run?.error_message}</p>
          {:else}
            <p class="runtime-ok">当前没有持久化的运行错误。</p>
          {/if}
        </section>

        <section>
          <span class="section-kicker">LEVELS</span>
          <h2>等级区间</h2>
          <ul class="level-list">
            {#each explanation.formula.levels as level}
              <li><strong>{level.level}</strong><span>{level.range}</span></li>
            {/each}
          </ul>
        </section>
      </aside>
    </div>

    <section class="content-section" aria-labelledby="snapshot-title">
      <div class="section-heading compact-heading">
        <div>
          <span class="section-kicker">PERSISTED SNAPSHOTS</span>
          <h2 id="snapshot-title">最近人员评分快照</h2>
        </div>
	        <span class="section-count">一级只看交付、质量、风险与证据状态；最近 {explanation.recent_snapshots.length} 条</span>
      </div>
      {#if explanation.recent_snapshots.length === 0}
        <div class="wa-admin-card state-panel empty-state">
          <strong>暂无评分快照</strong>
          <p>后台首次计算完成后，这里会显示已持久化的人员评分事实。</p>
        </div>
      {:else}
        <div class="wa-admin-table-shell snapshot-table-shell">
          <table class="wa-admin-table snapshot-table">
            <caption class="sr-only">最近持久化的人员绩效评分快照</caption>
            <thead>
              <tr>
                <th scope="col">人员</th>
                <th scope="col" class="status-column">状态</th>
                <th scope="col" class="numeric-column">参考分</th>
	                <th scope="col" class="numeric-column">交付 / 55</th>
	                <th scope="col" class="numeric-column">质量 / 45</th>
	                <th scope="col" class="numeric-column">风险扣分</th>
	                <th scope="col" class="numeric-column">证据状态</th>
                <th scope="col" class="time-column">计算时间</th>
              </tr>
            </thead>
            <tbody>
              {#each explanation.recent_snapshots as snapshot}
                <tr>
                  <td>
                    <button
                      type="button"
                      class="snapshot-member-button"
                      aria-label={`查看 ${snapshot.subject_key} 的评分详情`}
                      on:click={() => openSnapshotDetail(snapshot)}
                    >
                      <strong>{snapshot.subject_key}</strong>
                      <small title={snapshot.input_digest}>水位摘要 {snapshot.input_digest.slice(0, 8)}</small>
                    </button>
                  </td>
                  <td class="status-column"><span class="wa-admin-pill" class:tone-success={snapshot.rating_status === 'formal'} class:tone-warning={snapshot.rating_status !== 'formal'}>{ratingStatusLabel(snapshot.rating_status)}</span></td>
                  <td class="numeric-column">{referenceScoreLabel(snapshot)}</td>
	                  <td class="numeric-column">{componentScoreLabel(snapshot.delivery_score, 55)}</td>
	                  <td class="numeric-column">{componentScoreLabel(snapshot.quality_score, 45)}</td>
	                  <td class="numeric-column">{riskPenaltyLabel(snapshot.risk_penalty)}</td>
	                  <td class="numeric-column"><strong>{formatPercent(snapshot.evidence_coverage)}</strong><small>{snapshot.effective_sample_count} 样本 · {snapshot.exposure_days} 天</small></td>
                  <td class="time-column">{formatDateTime(snapshot.created_at)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</section>

<Modal
  show={selectedSnapshot !== null}
  title={selectedSnapshot ? `${selectedSnapshot.subject_key} 评分详情` : '评分详情'}
  size="wide"
  shadowless
  closeLabel="关闭评分详情"
  on:close={closeSnapshotDetail}
>
  <div class="snapshot-detail" aria-busy={detailLoading}>
    {#if detailLoading}
      <div class="detail-state" aria-live="polite">
        <span class="detail-loader" aria-hidden="true"></span>
        <strong>正在读取已保存快照</strong>
        <p>不会触发后台重新计算。</p>
      </div>
    {:else if detailError}
      <div class="detail-state detail-error" role="alert">
        <strong>评分详情暂时无法读取</strong>
        <p>{detailError}</p>
        {#if selectedSnapshot}
          <button type="button" class="wa-admin-action secondary" on:click={() => selectedSnapshot && openSnapshotDetail(selectedSnapshot)}>重新读取</button>
        {/if}
      </div>
    {:else if snapshotDetail}
      <div class="detail-notice" role="note">
        <span>{snapshotDetail.read_only_notice}</span>
        <time datetime={snapshotDetail.generated_at}>读取于 {formatDateTime(snapshotDetail.generated_at)}</time>
      </div>

      <section class="detail-summary" aria-label="评分摘要">
        <div><span>参考分</span><strong>{referenceScoreLabel(snapshotDetail.snapshot)}</strong></div>
        <div><span>正式分</span><strong>{formalScoreLabel(snapshotDetail.snapshot)}</strong></div>
	        <div><span>交付得分</span><strong>{componentScoreLabel(snapshotDetail.snapshot.delivery_score, 55)}</strong></div>
	        <div><span>质量得分</span><strong>{componentScoreLabel(snapshotDetail.snapshot.quality_score, 45)}</strong></div>
	        <div><span>风险扣分</span><strong>{riskPenaltyLabel(snapshotDetail.snapshot.risk_penalty)}</strong></div>
	        <div><span>证据覆盖</span><strong>{formatPercent(snapshotDetail.snapshot.evidence_coverage)}</strong></div>
      </section>

      <section class="detail-section" aria-labelledby="snapshot-meta-title">
	        <div class="detail-section-heading">
	          <div><span class="section-kicker">SNAPSHOT</span><h3 id="snapshot-meta-title">快照与计算水位</h3></div>
	        </div>
        <dl class="detail-meta-grid">
          <div><dt>考核周期</dt><dd>{formatDateTime(snapshotDetail.snapshot.assessment_window_start)} 至 {formatDateTime(snapshotDetail.snapshot.assessment_window_end)}</dd></div>
          <div><dt>输入水位</dt><dd>{formatDateTime(snapshotDetail.snapshot.input_watermark)}</dd></div>
          <div><dt>有效样本</dt><dd>{snapshotDetail.snapshot.effective_sample_count}</dd></div>
						<div><dt>需求 / 延期</dt><dd>{snapshotDetail.snapshot.delivery_unit_count} / {snapshotDetail.snapshot.demand_delay_count}</dd></div>
						<div><dt>Bug / 重开</dt><dd>{snapshotDetail.snapshot.bug_count} / {snapshotDetail.snapshot.bug_reopen_count}</dd></div>
						<div><dt>Commit / 重复</dt><dd>{snapshotDetail.snapshot.commit_count} / {snapshotDetail.snapshot.duplicate_commit_count}</dd></div>
          <div><dt>运行记录</dt><dd class="detail-mono">{snapshotDetail.snapshot.run_id}</dd></div>
          <div><dt>输入摘要</dt><dd class="detail-mono" title={snapshotDetail.snapshot.input_digest}>{snapshotDetail.snapshot.input_digest}</dd></div>
        </dl>
      </section>

      <section class="detail-section" aria-labelledby="snapshot-metrics-title">
        <div class="detail-section-heading">
          <div><span class="section-kicker">METRICS</span><h3 id="snapshot-metrics-title">指标得分与引用</h3></div>
          <span class="detail-count">{snapshotDetail.snapshot.metrics.filter(metric => metric.available).length} 项可计算，{snapshotDetail.snapshot.metrics.filter(metric => metric.sample_qualified).length} 项样本达标</span>
        </div>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (wide evidence table needs keyboard scrolling) -->
        <div class="detail-table-shell" role="region" aria-label="指标得分详情" tabindex="0">
          <table class="detail-table metric-detail-table">
              <thead><tr><th>指标</th><th class="numeric-column">权重</th><th class="numeric-column">样本</th><th>样本资格</th><th class="numeric-column">原始比率</th><th class="numeric-column">能力档</th><th class="numeric-column">指标分</th><th class="numeric-column">加权分</th><th>计算来源</th></tr></thead>
            <tbody>
              {#each snapshotDetail.snapshot.metrics as metric}
                <tr>
                  <td><strong>{metric.code}</strong><span>{metric.name}</span></td>
                  <td class="numeric-column">{formatPercent(metric.weight)}</td>
                  <td class="numeric-column">{metric.evidence_count} / {metric.minimum_samples}</td>
                  <td>{sampleQualificationLabel(metric)}</td>
									<td class="numeric-column"><strong>{metricRawLabel(metric)}</strong>{#if metric.raw_unit}<small>{metric.raw_unit}</small>{/if}</td>
                  <td class="numeric-column">{metric.point_level === undefined ? '-' : `${metric.point_level} 分档`}</td>
                  <td class="numeric-column">{metricScoreLabel(metric)}</td>
                  <td class="numeric-column">{metric.weighted_points === undefined ? '-' : metric.weighted_points.toFixed(2)}</td>
                  <td>
	                    {#if metric.evidence_refs?.length}
	                      <span class="detail-refs">{metric.evidence_refs.join('、')}</span>
								{#if metric.reason}<small class="detail-muted">{metric.reason}</small>{/if}
                    {:else}
                      <span class="detail-muted">{metric.reason || '没有满足口径的来源'}</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>

			<section class="detail-section" aria-labelledby="snapshot-risk-title">
				<div class="detail-section-heading">
					<div><span class="section-kicker">CODE RISK</span><h3 id="snapshot-risk-title">代码风险扣分</h3></div>
					<span class="detail-count">只扣不奖 · 合计 {riskPenaltyLabel(snapshotDetail.snapshot.risk_penalty)}</span>
				</div>
				{#if snapshotDetail.snapshot.adjustments.length === 0}
					<p class="detail-empty">该快照没有代码风险记录。</p>
				{:else}
					<!-- svelte-ignore a11y_no_noninteractive_tabindex (wide risk table needs keyboard scrolling) -->
					<div class="detail-table-shell" role="region" aria-label="代码风险扣分详情" tabindex="0">
						<table class="detail-table risk-detail-table">
							<thead><tr><th>信号</th><th class="numeric-column">原始值</th><th class="numeric-column">扣分</th><th>判定状态</th></tr></thead>
							<tbody>
								{#each snapshotDetail.snapshot.adjustments as adjustment}
									<tr>
										<td><strong>{adjustment.name || adjustment.reason_code}</strong>{#if adjustment.raw_unit}<small>{adjustment.raw_unit}</small>{/if}</td>
										<td class="numeric-column">{adjustmentRawLabel(adjustment)}</td>
										<td class="numeric-column">{adjustment.value.toFixed(2)}</td>
										<td>{adjustment.reason || '已按规则判定'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</section>

      <section class="detail-section" aria-labelledby="snapshot-factors-title">
        <div class="detail-section-heading">
          <div><span class="section-kicker">ITEM FACTORS</span><h3 id="snapshot-factors-title">需求与 Bug 系数明细</h3></div>
          <span class="detail-count">{snapshotDetail.item_factors.length} 项</span>
        </div>
        {#if snapshotDetail.item_factors.length === 0}
          <p class="detail-empty">该快照没有项目系数记录。</p>
        {:else}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex (wide factor table needs keyboard scrolling) -->
          <div class="detail-table-shell" role="region" aria-label="项目系数详情" tabindex="0">
            <table class="detail-table factor-detail-table">
              <thead><tr><th>工作项</th><th>类型</th><th>项目</th><th>系数过程</th></tr></thead>
              <tbody>
                {#each snapshotDetail.item_factors as factor}
                  <tr>
                    <td class="detail-mono">{factor.work_item_id || '-'}</td>
                    <td>{factorKindLabel(factor.kind)}</td>
                    <td>{factor.project_key || '-'}</td>
                    <td><span class="detail-mono">{factorFormula(factor)}</span></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>

      <section class="detail-section" aria-labelledby="snapshot-sources-title">
        <div class="detail-section-heading">
          <div><span class="section-kicker">EVIDENCE SOURCES</span><h3 id="snapshot-sources-title">正式证据来源</h3></div>
          <span class="detail-count">{snapshotDetail.sources.length} 条</span>
        </div>
        {#if snapshotDetail.sources.length === 0}
          <p class="detail-empty">本次得分仅使用任务、交付运行等系统事实，没有引用正式证据账本记录。</p>
        {:else}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex (wide source table needs keyboard scrolling) -->
          <div class="detail-table-shell" role="region" aria-label="正式证据来源详情" tabindex="0">
            <table class="detail-table source-detail-table">
              <thead><tr><th>来源</th><th>事件</th><th>工作项</th><th class="numeric-column">权重 / 结果</th><th>发生时间</th></tr></thead>
              <tbody>
                {#each snapshotDetail.sources as source}
                  <tr>
                    <td><strong>{source.source_system}</strong><span class="detail-mono">{source.source_record_id}</span></td>
                    <td>{evidenceTypeLabels[source.event_type] || source.event_type}<small>修订 {source.revision}</small></td>
                    <td class="detail-mono">{source.work_item_id || '-'}</td>
                    <td class="numeric-column">{formatNumber(source.weight)} / {formatNumber(source.outcome)}</td>
                    <td>{formatDateTime(source.occurred_at)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>

      {#if snapshotDetail.snapshot.exclusions.length > 0}
        <section class="detail-section" aria-labelledby="snapshot-exclusions-title">
          <div class="detail-section-heading">
            <div><span class="section-kicker">EXCLUSIONS</span><h3 id="snapshot-exclusions-title">未计入项</h3></div>
          </div>
          <ul class="detail-exclusions">
            {#each snapshotDetail.snapshot.exclusions as exclusion}<li>{exclusion}</li>{/each}
          </ul>
        </section>
      {/if}
    {/if}
  </div>
</Modal>

<style>
  .performance-guide {
    width: 100%;
    min-width: 0;
    padding: 0 0 24px;
    color: var(--wa-text-main);
  }

  .guide-heading,
  .section-heading {
    min-width: 0;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 20px;
  }

  .guide-heading {
    padding: 0;
  }

  .guide-heading > div,
  .section-heading > div {
    min-width: 0;
  }

  .section-kicker {
    display: block;
    color: var(--wa-accent);
    font-family: var(--wa-font-mono);
    font-size: 10px;
    font-weight: 780;
    letter-spacing: 0.12em;
  }

  h1,
  h2,
  h3,
  p {
    margin: 0;
  }

  h1 {
    margin-top: 5px;
    color: var(--wa-text-strong);
    font-size: 24px;
    line-height: 1.2;
    letter-spacing: -0.02em;
  }

  .guide-heading p {
    max-width: 720px;
    margin-top: 8px;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.65;
  }

  .refresh-action {
    flex: none;
  }

  .read-only-notice,
  .refresh-warning {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    border: 1px solid rgba(37, 107, 216, 0.18);
    border-radius: var(--wa-radius-md);
    background: var(--wa-info-soft);
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.5;
  }

  .read-only-notice time {
    margin-left: auto;
    color: var(--wa-text-muted);
    white-space: nowrap;
  }

  .notice-mark {
    width: 20px;
    height: 20px;
    flex: none;
    display: inline-grid;
    place-items: center;
    border-radius: 50%;
    background: var(--wa-info);
    color: var(--wa-accent-fill-ink);
    font-size: 12px;
    font-weight: 800;
  }

  .refresh-warning {
    border-color: rgba(216, 135, 0, 0.22);
    background: var(--wa-warning-soft);
    color: var(--wa-warning);
  }

  .status-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .wa-admin-card,
  .wa-admin-metric {
    box-shadow: none;
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }

  .wa-admin-metric {
    min-height: 132px;
    grid-template-rows: 18px 36px minmax(35px, auto);
    align-content: start;
    gap: 8px;
    background: rgba(255, 255, 255, 0.9);
  }

  .wa-admin-metric strong {
    font-size: 24px;
    line-height: 1.18;
  }

  .wa-admin-metric .metric-value {
    min-width: 0;
    min-height: 36px;
    display: flex;
    align-items: center;
  }

  .wa-admin-metric .metric-time {
    font-size: 15px;
    line-height: 1.45;
    letter-spacing: -0.01em;
    white-space: nowrap;
  }

  .wa-admin-metric .metric-note {
    min-height: 35px;
    align-self: start;
  }

  .wa-admin-metric strong.running {
    color: var(--wa-info);
  }

  .guide-layout {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(280px, 336px);
    gap: var(--wa-space-4);
    align-items: start;
  }

  .guide-main {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-4);
  }

  .content-section {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-4);
  }

  .wa-admin-card.content-section {
    padding: 16px;
    background: rgba(255, 255, 255, 0.92);
  }

  .section-heading h2,
  .guide-inspector h2 {
    margin-top: 4px;
    color: var(--wa-text-strong);
    font-size: 16px;
    line-height: 1.35;
  }

  .compact-heading {
    align-items: end;
    padding: 0;
  }

  .section-count {
    max-width: 56%;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.5;
    text-align: right;
  }

  .formula-list,
  .inspector-facts,
  .factor-group dl {
    margin: 0;
  }

  .formula-list {
    display: grid;
    border-top: 1px solid var(--wa-border-soft);
  }

  .formula-list > div {
    display: grid;
    grid-template-columns: 108px minmax(0, 1fr);
    gap: 14px;
    padding: 12px 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  dt {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 720;
  }

  dd {
    margin: 0;
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.55;
  }

  .boundary-note {
    padding: 12px;
		border: 1px solid rgba(0, 143, 150, 0.18);
		border-radius: var(--wa-radius-sm);
		background: rgba(0, 143, 150, 0.045);
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.65;
  }

  .metric-table {
    min-width: 1320px;
  }

  .metric-table th:nth-child(1) { width: 130px; }
  .metric-table th:nth-child(2) { width: 70px; }
  .metric-table th:nth-child(3) { width: 82px; }
  .metric-table th:nth-child(4) { width: 310px; }
  .metric-table th:nth-child(5) { width: 210px; }
  .metric-table th:nth-child(6) { width: 260px; }

  .metric-table td:first-child,
  .snapshot-table td:first-child {
    display: grid;
    gap: 2px;
  }

  .metric-table td:first-child strong {
    color: var(--wa-accent-strong);
    font-family: var(--wa-font-mono);
  }

  .metric-table code {
    color: var(--wa-text-muted);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    overflow-wrap: anywhere;
  }

  .wa-admin-table .numeric-column,
  .detail-table .numeric-column {
    text-align: right;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .wa-admin-table .status-column {
    text-align: center;
    white-space: nowrap;
  }

  .wa-admin-table .time-column {
    text-align: left;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .guide-inspector {
    position: sticky;
    top: 12px;
    gap: 0;
    padding: 0 16px;
    background: rgba(255, 255, 255, 0.92);
  }

  .guide-inspector > section {
    display: grid;
    gap: 10px;
    padding: 18px 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .guide-inspector > section:last-child {
    border-bottom: 0;
  }

  .guide-inspector p {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.6;
  }

  .inspector-facts {
    display: grid;
  }

  .inspector-facts > div,
  .factor-group dl > div {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .inspector-facts > div:last-child,
  .factor-group dl > div:last-child {
    border-bottom: 0;
  }

  .inspector-facts dd {
    max-width: 58%;
    text-align: right;
  }

  .evidence-types,
  .level-list {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .evidence-types li,
  .level-list li {
    min-height: 30px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .evidence-types strong,
  .level-list strong {
    color: var(--wa-text-strong);
    font-variant-numeric: tabular-nums;
  }

  .level-list {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: 16px;
  }

  .runtime-ok,
  .runtime-error {
    padding: 9px 10px;
    border-radius: var(--wa-radius-sm);
  }

  .runtime-ok {
    background: var(--wa-success-soft);
    color: var(--wa-success) !important;
  }

  .runtime-error {
    background: var(--wa-danger-soft);
    color: var(--wa-danger) !important;
    overflow-wrap: anywhere;
  }

  .factor-section {
    padding: 16px;
  }

  .factor-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    border-top: 1px solid var(--wa-border-soft);
    border-left: 1px solid var(--wa-border-soft);
  }

  .factor-group {
    min-width: 0;
    padding: 14px;
    border-right: 1px solid var(--wa-border-soft);
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .factor-group header {
    min-height: 54px;
  }

  .factor-group h3 {
    color: var(--wa-text-strong);
    font-size: 13px;
  }

  .factor-group p {
    margin-top: 4px;
    color: var(--wa-text-muted);
    font-size: 11px;
  }

  .factor-group dd {
    font-variant-numeric: tabular-nums;
    font-weight: 760;
  }

  .snapshot-table {
	    min-width: 910px;
  }

  .snapshot-table th:first-child { width: 176px; }
  .snapshot-table th:nth-child(2) { width: 104px; }
  .snapshot-table th:nth-child(3) { width: 82px; }
	  .snapshot-table th:nth-child(4) { width: 116px; }
	  .snapshot-table th:nth-child(5) { width: 116px; }
	  .snapshot-table th:nth-child(6) { width: 92px; }
	  .snapshot-table th:nth-child(7) { width: 132px; }
	  .snapshot-table th:nth-child(8) { width: 166px; }

  .snapshot-table th:first-child,
  .snapshot-table td:first-child {
    position: sticky;
    left: 0;
    background: var(--wa-surface-panel);
  }

  .snapshot-table th:first-child {
    z-index: 3;
  }

  .snapshot-table td:first-child {
    z-index: 1;
    box-shadow: 1px 0 0 var(--wa-border-soft);
  }

  .snapshot-table tbody tr:hover td:first-child {
    background: var(--wa-surface-inset);
  }

  .snapshot-table small {
    color: var(--wa-text-subtle);
    font-family: var(--wa-font-mono);
    font-size: 10px;
  }

	.snapshot-table td.numeric-column small {
		display: block;
		margin-top: 2px;
		font-family: inherit;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

  .snapshot-member-button {
    min-width: 0;
    display: grid;
    gap: 2px;
    padding: 4px 6px;
    margin: -4px -6px;
    border: 0;
    border-radius: var(--wa-radius-sm);
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .snapshot-member-button strong {
    color: var(--wa-accent-strong);
  }

  .snapshot-member-button:hover {
    background: var(--wa-accent-soft);
  }

  .snapshot-member-button:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.34);
    outline-offset: 2px;
  }

  .snapshot-detail {
    min-width: 0;
    display: grid;
    gap: 0;
    padding: 20px 22px 24px;
    color: var(--wa-text-main);
  }

  .detail-notice {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    padding: 10px 12px;
    border: 1px solid rgba(37, 107, 216, 0.18);
    border-radius: var(--wa-radius-md);
    background: var(--wa-info-soft);
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.5;
  }

  .detail-notice time {
    flex: none;
    color: var(--wa-text-muted);
  }

  .detail-summary {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    margin-top: 18px;
    border-top: 1px solid var(--wa-border-soft);
    border-left: 1px solid var(--wa-border-soft);
  }

  .detail-summary > div {
    min-width: 0;
    display: grid;
    gap: 5px;
    padding: 14px;
    border-right: 1px solid var(--wa-border-soft);
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .detail-summary span,
  .detail-count,
  .detail-muted {
    color: var(--wa-text-muted);
    font-size: 11px;
  }

  .detail-summary strong {
    color: var(--wa-text-strong);
    font-size: 18px;
    font-variant-numeric: tabular-nums;
  }

  .detail-section {
    min-width: 0;
    display: grid;
    gap: 12px;
    padding: 18px 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .detail-section:last-child {
    padding-bottom: 0;
    border-bottom: 0;
  }

  .detail-section-heading {
    min-width: 0;
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: 16px;
  }

  .detail-section-heading h3 {
    margin-top: 3px;
    color: var(--wa-text-strong);
    font-size: 15px;
  }

  .detail-meta-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    margin: 0;
    border-top: 1px solid var(--wa-border-soft);
  }

  .detail-meta-grid > div {
    min-width: 0;
    display: grid;
    grid-template-columns: 86px minmax(0, 1fr);
    gap: 12px;
    padding: 10px 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .detail-meta-grid > div:nth-child(odd) {
    padding-right: 18px;
  }

  .detail-meta-grid > div:nth-child(even) {
    padding-left: 18px;
    border-left: 1px solid var(--wa-border-soft);
  }

  .detail-meta-grid dd {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .detail-table-shell {
    min-width: 0;
    overflow: auto;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
  }

  .detail-table-shell:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.24);
    outline-offset: 2px;
  }

  .detail-table {
    width: 100%;
    border-collapse: collapse;
    color: var(--wa-text-main);
    font-size: 12px;
  }

  .detail-table th {
    padding: 9px 10px;
    border-bottom: 1px solid var(--wa-border-soft);
    background: var(--wa-surface-inset);
    color: var(--wa-text-muted);
    font-size: 10px;
    font-weight: 760;
    text-align: left;
    white-space: nowrap;
  }

  .detail-table td {
    padding: 10px;
    border-bottom: 1px solid var(--wa-border-soft);
    vertical-align: top;
    line-height: 1.45;
  }

  .detail-table tbody tr:last-child td {
    border-bottom: 0;
  }

  .detail-table td:first-child,
  .source-detail-table td:nth-child(2) {
    display: grid;
    gap: 2px;
  }

  .detail-table td strong {
    color: var(--wa-text-strong);
  }

  .metric-detail-table { min-width: 900px; }
	.risk-detail-table { min-width: 680px; }
  .factor-detail-table { min-width: 700px; }
  .source-detail-table { min-width: 760px; }

  .detail-refs,
  .detail-mono {
    font-family: var(--wa-font-mono);
    font-size: 11px;
    overflow-wrap: anywhere;
  }

  .detail-refs {
    color: var(--wa-text-main);
  }

  .detail-table small {
    color: var(--wa-text-subtle);
    font-size: 10px;
  }

  .detail-empty,
  .detail-exclusions {
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.6;
  }

  .detail-exclusions {
    display: grid;
    gap: 7px;
    padding-left: 20px;
  }

  .detail-state {
    min-height: 320px;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 8px;
    text-align: center;
  }

  .detail-state strong {
    color: var(--wa-text-strong);
  }

  .detail-state p {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .detail-loader {
    width: 24px;
    height: 24px;
    border: 2px solid var(--wa-border-strong);
    border-top-color: var(--wa-accent);
    border-radius: 50%;
    animation: detail-spin 0.7s linear infinite;
  }

  @keyframes detail-spin {
    to { transform: rotate(360deg); }
  }

  .state-panel {
    min-height: 180px;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 8px;
    padding: 24px;
    text-align: center;
    background: rgba(255, 255, 255, 0.92);
  }

  .state-panel strong {
    color: var(--wa-text-strong);
  }

  .state-panel p {
    max-width: 520px;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.6;
  }

  .error-state {
    border-color: rgba(221, 75, 62, 0.24);
  }

  .skeleton-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .skeleton-block,
  .skeleton-panel {
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-lg);
    background: linear-gradient(90deg, var(--wa-surface-inset), rgba(255, 255, 255, 0.9), var(--wa-surface-inset));
    background-size: 220% 100%;
    animation: skeleton-shift 1.5s ease-in-out infinite;
  }

  .skeleton-block { min-height: 132px; }
  .skeleton-panel { min-height: 420px; }

  @keyframes skeleton-shift {
    from { background-position: 100% 0; }
    to { background-position: -100% 0; }
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .skeleton-block,
    .skeleton-panel,
    .detail-loader { animation: none; }
  }

  @media (max-width: 1180px) {
    .status-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .factor-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }

  @media (max-width: 980px) {
    .guide-layout { grid-template-columns: minmax(0, 1fr); }
    .guide-inspector { position: static; }
  }

  @media (max-width: 760px) {
    .performance-guide { gap: 14px; }
    .guide-heading,
    .section-heading { display: grid; gap: 12px; }
    .refresh-action { width: 100%; min-height: 44px; }
    .read-only-notice { align-items: flex-start; flex-wrap: wrap; }
    .read-only-notice time { width: 100%; margin-left: 30px; white-space: normal; }
    .status-grid,
    .skeleton-grid,
    .factor-grid { grid-template-columns: minmax(0, 1fr); }
    .guide-layout { gap: 14px; }
    .wa-admin-card.content-section,
    .factor-section { padding: 14px; }
    .formula-list > div { grid-template-columns: minmax(0, 1fr); gap: 5px; }
    .section-count { max-width: none; text-align: left; }
    .factor-group header { min-height: 0; margin-bottom: 8px; }
    .level-list { grid-template-columns: minmax(0, 1fr); }
    .snapshot-detail { padding: 14px 16px 18px; }
    .detail-notice { display: grid; gap: 4px; }
    .detail-summary,
    .detail-meta-grid { grid-template-columns: minmax(0, 1fr); }
    .detail-meta-grid > div:nth-child(odd),
    .detail-meta-grid > div:nth-child(even) { padding-inline: 0; border-left: 0; }
    .detail-section-heading { align-items: start; }
  }
</style>
