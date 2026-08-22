<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Switch from '../shared/Switch.svelte';
  import Alert from '../shared/Alert.svelte';

  const dispatch = createEventDispatcher();

  export let config: {
    enabled: boolean;
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
  } = {
    enabled: false,
    interval_minutes: 60,
    assessment_window_days: 90,
    retention_days: 90,
    formula_version: 'v6.0',
    evidence_coverage_gate: 0.7,
    minimum_samples: 5,
    minimum_exposure_days: 30,
    busy_retry_attempts: 3,
    busy_retry_delay_ms: 200,
    publication_mode: 'shadow',
    demand_metrics_enabled: true,
    bug_metrics_enabled: true,
    code_metrics_enabled: true,
    jira_history_enabled: true,
    git_dedupe_enabled: true
  };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let lastUpdated = '';
  export let coreMembers: string[] = [];
  export let usesJQLFallback = false;
  export let canWrite = false;

  let enabled = config.enabled ?? false;
	let formalPublication = config.publication_mode === 'formal';
	let demandMetricsEnabled = config.demand_metrics_enabled ?? true;
	let bugMetricsEnabled = config.bug_metrics_enabled ?? true;
	let codeMetricsEnabled = config.code_metrics_enabled ?? true;
	let jiraHistoryEnabled = config.jira_history_enabled ?? true;
	let gitDedupeEnabled = config.git_dedupe_enabled ?? true;
	let lastConfigEnabled = enabled;
	let lastConfigSignature = '';

	$: if ((config.enabled ?? false) !== lastConfigEnabled) {
		lastConfigEnabled = config.enabled ?? false;
		enabled = lastConfigEnabled;
	}

	$: {
		const signature = JSON.stringify([
			config.publication_mode, config.demand_metrics_enabled, config.bug_metrics_enabled,
			config.code_metrics_enabled, config.jira_history_enabled, config.git_dedupe_enabled
		]);
		if (signature !== lastConfigSignature) {
			lastConfigSignature = signature;
			formalPublication = config.publication_mode === 'formal';
			demandMetricsEnabled = config.demand_metrics_enabled ?? true;
			bugMetricsEnabled = config.bug_metrics_enabled ?? true;
			codeMetricsEnabled = config.code_metrics_enabled ?? true;
			jiraHistoryEnabled = config.jira_history_enabled ?? true;
			gitDedupeEnabled = config.git_dedupe_enabled ?? true;
		}
	}

	$: if (!saving && saveError) {
		enabled = config.enabled ?? false;
	}

  function normalizedNumber(value: number, fallback: number) {
    return Number.isFinite(value) && value > 0 ? value : fallback;
  }

  function formatUpdated(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  }

  function saveEnabled() {
    dispatch('save', {
      key: 'performance_brain',
      data: { ...config, enabled }
    });
  }

	function saveFeatures() {
		dispatch('save', {
			key: 'performance_brain',
			data: {
				...config, enabled,
				publication_mode: formalPublication ? 'formal' : 'shadow',
				demand_metrics_enabled: demandMetricsEnabled,
				bug_metrics_enabled: bugMetricsEnabled,
				code_metrics_enabled: codeMetricsEnabled,
				jira_history_enabled: jiraHistoryEnabled,
				git_dedupe_enabled: gitDedupeEnabled
			}
		});
	}

  $: coreSourceLabel = coreMembers.length > 0
    ? `Jira 同步成员 ${coreMembers.length} 人`
    : usesJQLFallback
      ? '由 Jira 自定义 JQL 的 assignee 解析'
      : '未配置，计算将保持空结果';
</script>

<div class="scw-workbench">
  <section class="scw-overview" aria-label="人员绩效后台计算配置">
    <header class="scw-header">
      <div class="scw-header-copy">
        <span class="scw-kicker">人员绩效</span>
        <h4>后台自动评分</h4>
        <p>后台按固定周期静默计算，只为 core member 生成不可变快照；页面访问不会触发计算。</p>
      </div>
      <div class="scw-toggle">
        <span>{saving ? '正在应用' : enabled ? '已启用' : '未启用'}</span>
        <Switch
          id="performance-brain-enabled"
          label="启用后台自动评分"
          bind:checked={enabled}
          disabled={saving || !canWrite}
          on:change={saveEnabled}
        />
      </div>
    </header>

    {#if enabled && coreMembers.length === 0 && !usesJQLFallback}
      <Alert
        type="warning"
        title="尚未配置 core member"
        message="请先在 Jira 服务关联中维护同步成员，或在自定义 JQL 中限定 assignee。为避免误纳入考核，当前不会生成任何成员评分。"
      />
    {/if}

    <div class="scw-status tone-{enabled ? 'success' : 'unchecked'}">
      <div class="scw-status-main">
        <span>运行策略</span>
        <strong>{enabled ? '后台静默运行' : '后台计算已关闭'}</strong>
      </div>
      <p>{canWrite ? '切换后立即生效，运行、快照、证据和审计记录继续按保留期滚动清理。' : '当前账号仅可查看配置，需要 config:write 权限才能修改。'}</p>
    </div>

    <div class="scw-read-grid">
      <div class="scw-read-item">
        <span>计算成员范围</span>
        <strong>{coreSourceLabel}</strong>
      </div>
      <div class="scw-read-item">
        <span>滚动周期</span>
        <strong>{normalizedNumber(config.interval_minutes, 60)} 分钟</strong>
      </div>
      <div class="scw-read-item">
        <span>考核窗口</span>
        <strong>{normalizedNumber(config.assessment_window_days, 90)} 天</strong>
      </div>
      <div class="scw-read-item">
        <span>审计保留</span>
        <strong>{normalizedNumber(config.retention_days, 90)} 天</strong>
      </div>
      <div class="scw-read-item">
        <span>公式版本</span>
        <strong class="font-mono">{config.formula_version || 'v6.0'}</strong>
      </div>
      <div class="scw-read-item">
        <span>正式分门槛</span>
        <strong>覆盖 {Math.round((config.evidence_coverage_gate || 0.7) * 100)}% / {normalizedNumber(config.minimum_samples, 5)} 个样本 / {normalizedNumber(config.minimum_exposure_days, 30)} 天暴露</strong>
      </div>
      <div class="scw-read-item">
        <span>最近更新时间</span>
        <strong>{formatUpdated(lastUpdated)}</strong>
      </div>
    </div>

		<div class="scw-feature-grid" aria-label="绩效评分功能开关">
			<div class="scw-feature-row">
				<div><strong>正式分发布</strong><span>{formalPublication ? '门槛通过后发布正式分' : '影子运行，只保留参考分与审计'}</span></div>
				<Switch id="performance-formal-publication" label="允许发布正式分" bind:checked={formalPublication} disabled={saving || !canWrite} on:change={saveFeatures} />
			</div>
			<div class="scw-feature-row">
				<div><strong>需求交付维度</strong><span>完成、按期和流动效率</span></div>
				<Switch id="performance-demand-metrics" label="启用需求交付维度" bind:checked={demandMetricsEnabled} disabled={saving || !canWrite} on:change={saveFeatures} />
			</div>
			<div class="scw-feature-row">
				<div><strong>Bug 质量维度</strong><span>责任密度、修复周期和重开复发</span></div>
				<Switch id="performance-bug-metrics" label="启用 Bug 质量维度" bind:checked={bugMetricsEnabled} disabled={saving || !canWrite} on:change={saveFeatures} />
			</div>
			<div class="scw-feature-row">
				<div><strong>代码过程维度</strong><span>重复变更和 Commit 密度风险</span></div>
				<Switch id="performance-code-metrics" label="启用代码过程维度" bind:checked={codeMetricsEnabled} disabled={saving || !canWrite} on:change={saveFeatures} />
			</div>
			<div class="scw-feature-row">
				<div><strong>Jira 历史流转</strong><span>采集负责人、状态、截止日和优先级变更</span></div>
				<Switch id="performance-jira-history" label="启用 Jira 历史流转采集" bind:checked={jiraHistoryEnabled} disabled={saving || !canWrite} on:change={saveFeatures} />
			</div>
			<div class="scw-feature-row">
				<div><strong>Git 重复证据</strong><span>同 SHA 去重并生成稳定变更指纹</span></div>
				<Switch id="performance-git-dedupe" label="启用 Git 去重与指纹" bind:checked={gitDedupeEnabled} disabled={saving || !canWrite} on:change={saveFeatures} />
			</div>
		</div>
  </section>
</div>
