import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function read(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const demandKanban = read('src/components/DemandKanban.svelte');
const deliveryPlan = read('src/components/DeliveryPlan.svelte');
const taskKanban = read('src/components/TaskKanban.svelte');
const settingsPanel = read('src/components/SettingsPanel.svelte');
const settingsSections = read('src/lib/settings-sections.ts');

test('schedule inspector removes the risk calendar surface and its background request', () => {
  assert.doesNotMatch(demandKanban, /aria-label="风险日历"/);
  assert.doesNotMatch(demandKanban, /\/api\/schedule\/risk-calendar/);
  assert.doesNotMatch(demandKanban, /fetchRiskCalendar\(\);/);
});

test('release plan separates current and archived versions with archive then delete actions', () => {
  assert.match(deliveryPlan, /type ReleaseListView = 'current' \| 'archived'/);
  assert.match(deliveryPlan, /role="tablist" aria-label="版本列表视图"/);
  assert.match(deliveryPlan, />当前版本</);
  assert.match(deliveryPlan, />归档版本</);
  assert.match(deliveryPlan, /releaseListView === 'archived'/);
  assert.match(deliveryPlan, /openLifecycleConfirmation\('archive'/);
  assert.match(deliveryPlan, /openLifecycleConfirmation\('delete'/);
  assert.doesNotMatch(deliveryPlan, /openLifecycleConfirmation\('discard'/);
  assert.doesNotMatch(deliveryPlan, />废弃版本</);
  assert.doesNotMatch(deliveryPlan, />发布版本</);
});

test('task table and execution tracking share one schedule-density summary strip', () => {
  assert.match(taskKanban, /class="phase41-summary-strip wa-admin-card"/);
  assert.match(taskKanban, /class:is-status=\{currentView !== 'execution'\}/);
  assert.match(taskKanban, /class:is-execution=\{currentView === 'execution'\}/);
  assert.doesNotMatch(taskKanban, /class="phase41-metric-grid"/);
  assert.doesNotMatch(taskKanban, /class="phase41-stage-strip"/);
  assert.doesNotMatch(taskKanban, /class="wa-admin-card wa-admin-metric phase41-metric/);
  assert.doesNotMatch(taskKanban, /class="phase41-stage-card wa-admin-card/);
  assert.match(taskKanban, /\.phase41-summary-strip\.is-status\s*\{[\s\S]*?grid-template-columns:\s*repeat\(6, minmax\(0, 1fr\)\)/);
  assert.match(taskKanban, /\.phase41-summary-strip\.is-execution\s*\{[\s\S]*?grid-template-columns:\s*repeat\(4, minmax\(0, 1fr\)\)/);
  assert.match(taskKanban, /\.phase41-summary-strip > \.phase41-metric,[\s\S]*?\.phase41-summary-strip > \.phase41-stage-card\s*\{[\s\S]*?min-height:\s*72px[\s\S]*?border-radius:\s*0[\s\S]*?background:\s*var\(--task-cell-background, var\(--wa-neutral-soft\)\)[\s\S]*?box-shadow:\s*none/);
  assert.match(taskKanban, /\.phase41-metric\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0, 1fr\) auto/);
  assert.match(taskKanban, /\.phase41-metric > strong\s*\{[\s\S]*?font-size:\s*20px/);
  const stageLabelBlock = taskKanban.match(/\.phase41-stage-card span\s*\{([^}]*)\}/)?.[1] || '';
  assert.ok(stageLabelBlock, 'stage labels need a dedicated compact layout contract');
  assert.match(stageLabelBlock, /overflow:\s*hidden/);
  assert.match(stageLabelBlock, /text-overflow:\s*ellipsis/);
  assert.match(stageLabelBlock, /white-space:\s*nowrap/);
  assert.match(taskKanban, /@media \(max-width: 1180px\)[\s\S]*?\.phase41-summary-strip\s*\{[\s\S]*?overflow-x:\s*auto/);
  assert.match(taskKanban, /@media \(max-width: 1180px\)[\s\S]*?\.phase41-summary-strip\.is-status\s*\{[\s\S]*?repeat\(6, minmax\(112px, 1fr\)\)/);
  const summaryToneBlock = taskKanban.match(
    /\.task-console \.phase41-metric,\s*\.task-console \.phase41-stage-card\s*\{([^}]*)\}/,
  )?.[1] || '';
  assert.ok(summaryToneBlock, 'summary tone block must remain available for semantic color tokens');
  assert.match(summaryToneBlock, /--task-cell-background:\s*var\(--wa-neutral-soft\)/);
  assert.doesNotMatch(summaryToneBlock, /border-radius|box-shadow|backdrop-filter/);
  assert.match(taskKanban, /\.task-console \.phase41-summary-strip \.surface-accent\s*\{[\s\S]*?--task-cell-background:\s*var\(--wa-accent-soft\)/);
  assert.match(taskKanban, /\.task-console \.phase41-(?:metric|stage-card)\.tone-info[\s\S]*?--task-cell-background:\s*var\(--wa-info-soft\)/);
  assert.match(taskKanban, /\.task-console \.phase41-(?:metric|stage-card)\.tone-danger[\s\S]*?--task-cell-background:\s*var\(--wa-danger-soft\)/);
  assert.match(taskKanban, /\.task-console \.phase41-(?:metric|stage-card)\.tone-warning[\s\S]*?--task-cell-background:\s*var\(--wa-warning-soft\)/);

  const taskMetricBuilder = taskKanban.match(/function buildTaskAdminMetrics\(\): [^{]+\{([\s\S]*?)\n  \}/)?.[1] || '';
  assert.ok(taskMetricBuilder, 'task metrics need a dedicated builder');
  assert.match(taskMetricBuilder, /label:\s*'活跃事项'[\s\S]*?value:\s*taskMetricActive/);
  assert.match(taskMetricBuilder, /label:\s*'活跃需求'[\s\S]*?value:\s*activeRequirementCount/);
  assert.match(taskMetricBuilder, /label:\s*'活跃 Bug'[\s\S]*?value:\s*activeBugCount/);
  assert.doesNotMatch(taskMetricBuilder, /交付事项|需求 \/ Bug|已归项目|闭环率|taskMetricTotal|taskMetricDone/);

  const taskStageDefinition = taskKanban.match(/\$: taskFlowStages = \[([\s\S]*?)\n  \]\.map/)?.[1] || '';
  assert.ok(taskStageDefinition, 'task stages need a dedicated active-stage definition');
  assert.doesNotMatch(taskStageDefinition, /key:\s*'done'|label:\s*'完成'/);

  const summaryMarkup = taskKanban.match(/<div\s+class="phase41-summary-strip[\s\S]*?\n  <\/div>/)?.[0] || '';
  assert.ok(summaryMarkup, 'summary strip markup must be inspectable');
  assert.match(summaryMarkup, /class:surface-accent=\{metric\.surface === 'accent'\}/);
  assert.doesNotMatch(summaryMarkup, /stage\.percent/);
});

test('configuration versions are managed on one route instead of per-form sidebars', () => {
  assert.match(settingsSections, /id: 'versions'/);
  assert.match(settingsSections, /label: '配置版本'/);
  assert.match(settingsPanel, /activeSection === 'versions'/);
  assert.match(settingsPanel, /class="config-version-workbench"/);
  assert.match(settingsPanel, /aria-label="全部配置版本"/);
  assert.match(settingsPanel, /class="version-layout"/);
  assert.doesNotMatch(settingsPanel, /<aside class="settings-audit-pane"/);
  assert.match(settingsPanel, /class:with-context=\{isIntegrationSection\}/);
  assert.match(settingsPanel, /<aside class="settings-context-pane" aria-label="配置上下文检查器">/);
  assert.match(settingsPanel, /class:compact-config=\{isIntegrationSection\}/);

  const contextPane = settingsPanel.match(/<aside class="settings-context-pane"[\s\S]*?<\/aside>/)?.[0] || '';
  assert.ok(contextPane, 'ordinary configuration pages need a contextual right inspector');
  assert.doesNotMatch(contextPane, /version-layout|configVersions|rollbackConfigVersion|配置版本审计/);
  assert.match(contextPane, /settingsInspector/);
  assert.doesNotMatch(settingsPanel, /return \['\/api\/config', '\/api\/config\/versions'\]/);
  assert.match(settingsPanel, /if \(\['gitlab', 'feishu', 'jira', 'performance', 'projects', 'ai', 'ai_context'\]\.includes\(section\)\) \{\s*return \['\/api\/config'\];/);
});
