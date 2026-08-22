import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const projectHealthTelemetry = readFileSync(
  new URL('../src/components/ProjectHealthTelemetry.svelte', import.meta.url),
  'utf8'
);

const workbenchStart = projectHealthTelemetry.indexOf('<div class="project-health-workbench">');
const modalStart = projectHealthTelemetry.indexOf('{#if showDetailsModal && selectedProjectScore}');
assert.notEqual(workbenchStart, -1, 'project health workbench must exist');
assert.notEqual(modalStart, -1, 'project health modal boundary must exist');
const workbench = projectHealthTelemetry.slice(workbenchStart, modalStart);

test('source empty state never uses the filtered result to unmount the search controls', () => {
  assert.match(
    workbench,
    /{:else if scores\.length === 0}[\s\S]*?{:else}[\s\S]*?placeholder="搜索项目、编号"/,
    'only a genuinely empty source snapshot may replace the complete workbench'
  );
  assert.doesNotMatch(
    workbench,
    /{:else if searchMatchedScores\.length === 0}/,
    'a zero-result query must not remove its own search input'
  );
});

test('zero-result searches stay inside the table and explain how to recover', () => {
  assert.match(
    workbench,
    /searchMatchedScores\.length === 0[\s\S]*?'当前搜索无匹配项目'/,
    'the decision strip must not describe an empty search result as healthy'
  );
  assert.match(
    workbench,
    /{#if rankedFilteredScores\.length === 0}[\s\S]*?searchQuery[\s\S]*?没有匹配当前搜索的项目[\s\S]*?当前状态筛选下暂无项目/,
    'the table must distinguish search misses from health-filter misses'
  );
  assert.match(
    workbench,
    /project-health-inspector-empty[\s\S]*?searchQuery \|\| healthFilter !== 'all'[\s\S]*?调整搜索词或健康状态筛选/,
    'the inspector must describe a recoverable filter miss instead of claiming source data is missing'
  );
});

test('typing remains a local derived filter and never starts a remote refresh', () => {
  assert.match(
    workbench,
    /<input type="text" placeholder="搜索项目、编号" bind:value={searchQuery} class="telemetry-search-input" \/>/,
    'the search input must stay directly bound to local filter state'
  );
  assert.doesNotMatch(
    workbench,
    /<input[^>]+placeholder="搜索项目、编号"[^>]+on:input={[^}]*fetchScoresAndConfigs/,
    'typing must not trigger the remote refresh path'
  );
});
