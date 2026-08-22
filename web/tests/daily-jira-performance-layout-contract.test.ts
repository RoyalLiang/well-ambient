import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(
  new URL('../src/components/DailyJiraAudit.svelte', import.meta.url),
  'utf8'
);

test('Daily Jira decision controls share an explicit label baseline', () => {
  assert.match(source, /id="daily-jira-decision"[\s\S]*?label="决策动作"/);
  assert.match(source, /id="daily-jira-assignee-mode"[\s\S]*?label="负责人去向"/);
  assert.match(source, /\.decision-form > \.assignee-field\s*\{[\s\S]*?grid-column:\s*1\s*\/\s*-1/);
});

test('Daily Jira refresh keeps unchanged snapshots stable and loads auxiliary resources once', () => {
  assert.match(source, /dailyJiraSnapshotFingerprint/);
  assert.match(source, /nextAuditFingerprint\s*!==\s*auditFingerprint/);
  assert.match(source, /lastCheckedAt\s*=\s*nextAudit\.generated_at/);
  assert.match(source, /initial\s*\|\|\s*!auxiliaryResourcesLoaded/);
  assert.match(source, /async function loadStableAuditSegment/);
  assert.match(source, /nextPage\.page\.generation\s*!==\s*firstPage\.page\.generation/);
  assert.match(source, /audit\s*=\s*candidateAudit/);
  assert.doesNotMatch(source, /currentItems\.slice\(auditPageLimit\)/);
});

test('Daily Jira mounts only the visible table window for large audit buckets', () => {
  assert.match(source, /visibleItems\s*=\s*filteredItems\.slice\(virtualStart,\s*virtualEnd\)/);
  assert.match(source, /topSpacerHeight\s*=\s*virtualStart\s*\*\s*virtualRowHeight/);
  assert.match(source, /bottomSpacerHeight\s*=\s*\(filteredItems\.length\s*-\s*virtualEnd\)\s*\*\s*virtualRowHeight/);
  assert.match(source, /\{#each visibleItems as item, visibleIndex\}/);
});

test('Daily Jira consumes bounded cursor pages instead of a whole audit bucket', () => {
  assert.match(source, /interface DailyJiraPageMeta/);
  assert.match(source, /URLSearchParams/);
  assert.match(source, /params\.set\('bucket',\s*bucket\)/);
  assert.match(source, /params\.set\('limit',\s*String\(auditPageLimit\)\)/);
  assert.match(source, /params\.set\('cursor',\s*cursor\)/);
  assert.match(source, /async function loadNextPage/);
  assert.match(source, /remainingScroll\s*<=\s*virtualRowHeight\s*\*\s*virtualLoadAheadRows/);
  assert.match(source, /filteredItems\s*=\s*activeBucket\?\.items\s*\|\|\s*\[\]/);
  assert.doesNotMatch(source, /filterItems\(activeBucket\?\.items/);
});

test('Daily Jira scrollports use stable geometry without large-surface backdrop blur', () => {
  const finalSurface = source.slice(source.indexOf('/* L0 page gaps remain visible'));
  assert.doesNotMatch(finalSurface, /\.audit-table-panel,[\s\S]*?\.audit-inspector\s*\{[^}]*backdrop-filter:/);
  assert.doesNotMatch(finalSurface, /62dvh/);
  assert.match(finalSurface, /62svh/);
  assert.match(
    finalSurface,
    /\.audit-table-shell\s*\{[^}]*overflow-anchor:\s*none/
  );
  assert.match(
    finalSurface,
    /@media \(max-width:\s*1180px\)[\s\S]*?\.audit-inspector\s*\{[^}]*overflow-y:\s*auto/
  );
});
