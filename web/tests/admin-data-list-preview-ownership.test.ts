import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const dataList = source('src/components/admin-console/AdminDataList.svelte');
const preview = source('src/components/prototype/AdminDataListPreview.svelte');
const dailyJira = source('src/components/DailyJiraAudit.svelte');
const decisionDashboard = source('src/components/DecisionDashboard.svelte');

test('AdminDataList exposes an inspectable, versioned component style contract', () => {
  assert.match(dataList, /data-component="AdminDataList"/);
  assert.match(dataList, /data-style-contract="admin-data-list\/v1"/);
  assert.match(dataList, /data-row-height=\{rowHeight\}/);
});

test('AdminDataList owns exact row geometry instead of inheriting table padding', () => {
  assert.match(dataList, /`--admin-row-height:\s*\$\{rowHeight\}px`/);
  assert.match(dataList, /\.admin-table th,\s*\n\s*\.admin-table td\s*\{[\s\S]*?box-sizing:\s*border-box[\s\S]*?padding-block:\s*0/);
  assert.match(dataList, /\.admin-table tbody tr\.data-row\s*\{[\s\S]*?height:\s*var\(--admin-row-height\)/);
  assert.match(dataList, /\.admin-table td\s*\{[\s\S]*?height:\s*var\(--admin-row-height\)/);
});

test('preview delegates every table to AdminDataList', () => {
  assert.match(preview, /<AdminDataList\b/);
  assert.doesNotMatch(preview, /<table\b|<thead\b|<tbody\b|<tr\b|<th\b|<td\b/);
});

test('business pages cannot pierce shared table header or cell styling', () => {
  for (const page of [dailyJira, decisionDashboard]) {
    assert.doesNotMatch(page, /:global\([^)]*\.(?:admin-table|table-scroll)\b/);
  }
  assert.doesNotMatch(decisionDashboard, /\.decision-admin\s+:global\(\*\)/);
});
