import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function read(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const guide = read('src/components/PerformanceCalculationGuide.svelte');
const performanceConfig = read('src/components/config/PerformanceConfig.svelte');
const settingsPanel = read('src/components/SettingsPanel.svelte');
const settingsSections = read('src/lib/settings-sections.ts');

test('performance calculation is a versioned configuration switch with a core-member guard', () => {
  assert.match(settingsSections, /id: 'performance'/);
  assert.match(settingsPanel, /<PerformanceConfig/);
  assert.match(performanceConfig, /key: 'performance_brain'/);
  assert.match(performanceConfig, /on:change=\{saveEnabled\}/);
  assert.match(performanceConfig, /切换后立即生效/);
  assert.match(performanceConfig, /尚未配置 core member/);
  assert.match(performanceConfig, /当前不会生成任何成员评分/);
	assert.match(performanceConfig, /v6\.0/);
	assert.match(performanceConfig, /minimum_exposure_days/);
	for (const feature of ['publication_mode', 'demand_metrics_enabled', 'bug_metrics_enabled', 'code_metrics_enabled', 'jira_history_enabled', 'git_dedupe_enabled']) {
		assert.match(performanceConfig, new RegExp(feature));
	}
});

test('member rows open one shared read-only detail modal without triggering calculation', () => {
  assert.match(guide, /class="snapshot-member-button"/);
  assert.match(guide, /aria-label=\{`查看 \$\{snapshot\.subject_key\} 的评分详情`\}/);
  assert.match(guide, /fetch\(`\/api\/performance\/snapshots\/\$\{snapshot\.id\}`/);
  assert.match(guide, /<Modal[\s\S]*?size="wide"[\s\S]*?shadowless/);
  assert.match(guide, /不会触发后台重新计算/);
  assert.doesNotMatch(guide, /POST['"`][\s\S]*?performance\/snapshots/);
});

test('snapshot detail exposes the three v6 calculations, code risk, and auditable evidence', () => {
	for (const label of ['三项正式计算口径', '评分摘要', '参考分', '正式分', '影子评分', '快照与计算水位', '指标得分与引用', '样本资格', '代码风险扣分', '只扣不奖', '需求与 Bug 系数明细', '正式证据来源', '未计入项', '需求 / 延期', 'Bug / 重开', 'Commit / 重复']) {
    assert.match(guide, new RegExp(label));
  }
  assert.match(guide, /snapshotDetail\.snapshot\.metrics/);
  assert.match(guide, /snapshotDetail\.item_factors/);
  assert.match(guide, /snapshotDetail\.sources/);
  assert.match(guide, /snapshotDetail\.snapshot\.exclusions/);
  assert.match(guide, /snapshot\.final_score === null \? 'N\/A'/);
  assert.match(guide, /snapshot\.observed_score === null \? 'N\/A'/);
  assert.match(guide, /metric\.sample_qualified/);
	assert.match(guide, /metric\.numerator/);
	assert.match(guide, /metric\.denominator/);
	assert.match(guide, /source_event_count/);
	assert.match(guide, /snapshotDetail\.snapshot\.adjustments/);
	assert.match(guide, /adjustment\.value/);
	assert.doesNotMatch(guide, /\{explanation\.runtime\.formula_version\}/);
	assert.doesNotMatch(guide, /\{snapshotDetail\.snapshot\.formula_version\}/);
  assert.match(guide, /2 \/ 3 \/ 4 \/ 5 分边界/);
});

test('score panel keeps status baselines stable and aligns comparable table values', () => {
	assert.equal((guide.match(/class="wa-admin-card wa-admin-metric status-card"/g) || []).length, 4);
	assert.equal((guide.match(/class="metric-value/g) || []).length, 4);
	assert.equal((guide.match(/class="metric-note"/g) || []).length, 4);
	assert.match(guide, /\.wa-admin-metric\s*\{[\s\S]*?grid-template-rows:\s*18px 36px minmax\(35px, auto\)/);
	assert.match(guide, /\.skeleton-block\s*\{\s*min-height:\s*132px/);
	assert.match(guide, /\.wa-admin-card\.content-section\s*\{[\s\S]*?padding:\s*16px/);
	assert.match(guide, /\.snapshot-table\s*\{\s*min-width:\s*910px/);
	assert.match(guide, /\.wa-admin-table \.numeric-column,[\s\S]*?text-align:\s*right/);
	assert.match(guide, /\.wa-admin-table \.status-column\s*\{[\s\S]*?text-align:\s*center/);
	assert.match(guide, /class="numeric-column">参考分/);
	assert.match(guide, /class="numeric-column">交付 \/ 55/);
	assert.match(guide, /class="numeric-column">质量 \/ 45/);
	assert.match(guide, /class="numeric-column">风险扣分/);
	assert.match(guide, /class="numeric-column">证据状态/);
	assert.match(guide, /class="time-column">计算时间/);
	assert.match(guide, /\.snapshot-table th:first-child,[\s\S]*?position:\s*sticky/);
});

test('calculation factors stay in the primary content flow instead of waiting for the inspector height', () => {
	assert.match(
		guide,
		/<main class="guide-main">[\s\S]*?aria-labelledby="metric-title"[\s\S]*?aria-labelledby="factor-title"[\s\S]*?<\/main>\s*<aside class="wa-admin-card wa-admin-inspector guide-inspector"/
	);
	assert.match(guide, /\.factor-grid\s*\{[\s\S]*?grid-template-columns:\s*repeat\(auto-fit, minmax\(220px, 1fr\)\)/);
});

test('component scores tolerate snapshots created before v6 fields existed', () => {
	assert.match(guide, /delivery_score\?: number \| null/);
	assert.match(guide, /quality_score\?: number \| null/);
	assert.match(
		guide,
		/function componentScoreLabel\(value: number \| null \| undefined, maximum: number\)\s*\{[\s\S]*?value == null \? 'N\/A'/
	);
});
