import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const filterBar = source('src/components/admin-console/AdminListFilterBar.svelte');
const dataList = source('src/components/admin-console/AdminDataList.svelte');
const adminTokens = source('src/styles/modern-admin-tokens.css');
const decision = source('src/components/DecisionDashboard.svelte');
const dailyJira = source('src/components/DailyJiraAudit.svelte');
const preview = source('src/components/prototype/AdminDataListPreview.svelte');

test('shared filter bar owns only page-level filter layout and metadata', () => {
  assert.match(filterBar, /leading\?: Snippet/);
  assert.match(filterBar, /controls: Snippet/);
  assert.match(filterBar, /meta\?: Snippet/);
  assert.match(filterBar, /data-component="AdminListFilterBar"/);
  assert.match(filterBar, /data-style-contract="admin-list-filter-bar\/v1"/);
  assert.match(filterBar, /aria-label=\{label\}/);
  assert.match(filterBar, /@media \(max-width: 900px\)/);
  assert.match(filterBar, /@media \(max-width: 640px\)/);
  const props = filterBar.match(/interface Props[\s\S]*?\n  }/)?.[0] ?? '';
  assert.doesNotMatch(filterBar, /fetch\(/);
  assert.doesNotMatch(props, /columns\??:|rows\??:/);
});

test('decision filters and list are explicit sibling components', () => {
  assert.match(decision, /import AdminListFilterBar from '\.\/admin-console\/AdminListFilterBar\.svelte'/);
  assert.match(
    decision,
    /<AdminListFilterBar[\s\S]*?<\/AdminListFilterBar>\s*<AdminDataList/
  );
  assert.match(decision, /className="decision-filter-bar"/);
  assert.doesNotMatch(decision, /<AdminDataList[\s\S]*?surface="embedded"/);
});

test('Daily Jira filters and list are explicit sibling components', () => {
  assert.match(dailyJira, /import AdminListFilterBar from '\.\/admin-console\/AdminListFilterBar\.svelte'/);
  assert.match(
    dailyJira,
    /<AdminListFilterBar[\s\S]*?<\/AdminListFilterBar>\s*<AdminDataList/
  );
  assert.match(dailyJira, /className="daily-jira-filter-bar"/);
  assert.doesNotMatch(dailyJira, /<AdminDataList[\s\S]*?surface="embedded"/);
});

test('Daily Jira keeps bucket counts inside the segmented switch without a duplicate meta row', () => {
  const filterSurface = dailyJira.match(
    /<AdminListFilterBar[\s\S]*?className="daily-jira-filter-bar"[\s\S]*?<\/AdminListFilterBar>/
  )?.[0] ?? '';

  assert.match(filterSurface, /<span>\{bucket\.label\}<\/span>\s*<strong>\{bucket\.count\}<\/strong>/);
  assert.doesNotMatch(filterSurface, /\{#snippet meta\(\)\}|class="table-meta"|filteredItems\.length/);
  assert.match(filterSurface, /class="last-checked"[\s\S]*?检查于/);
  assert.match(dailyJira, /\.age-tab\s*\{[\s\S]*?white-space:\s*nowrap/);
  assert.match(dailyJira, /\.last-checked\s*\{[\s\S]*?white-space:\s*nowrap/);
  assert.match(dailyJira, /@media \(max-width: 1180px\)[\s\S]*?\.last-checked\s*\{[\s\S]*?display:\s*none/);
  assert.match(
    dailyJira,
    /@media \(max-width: 360px\)[\s\S]*?\.age-tab\s*\{[\s\S]*?gap:\s*4px[\s\S]*?padding-inline:\s*5px/
  );
});

test('data list API stays free of page filter and toolbar ownership', () => {
  const props = dataList.match(/interface Props[\s\S]*?\n  }/)?.[0] ?? '';
  assert.doesNotMatch(props, /filters?\?:|search\?:|toolbar\?:|filterBar\?:/);
  assert.match(dataList, /data-component="AdminDataList"/);
  assert.match(dataList, /data-style-contract="admin-data-list\/v1"/);
});

test('filter bar inherits the exact standalone AdminDataList surface contract', () => {
  const surfaceTokens = [
    '--wa-admin-list-surface-background',
    '--wa-admin-list-surface-border',
    '--wa-admin-list-surface-border-top',
    '--wa-admin-list-surface-radius',
    '--wa-admin-list-surface-shadow',
    '--wa-admin-list-surface-filter',
    '--wa-admin-list-surface-solid-background'
  ];

  assert.match(filterBar, /data-surface-contract="admin-data-list-surface\/v1"/);
  assert.match(dataList, /admin-data-list-surface\/v1/);
  for (const token of surfaceTokens) {
    assert.match(adminTokens, new RegExp(`${token}:`));
    assert.match(filterBar, new RegExp(`var\\(${token}\\)`));
    assert.match(dataList, new RegExp(`var\\(${token}\\)`));
  }

  assert.doesNotMatch(filterBar, /--wa-glass-panel-strong|--wa-shadow-sm|saturate\(124%\)/);
  assert.doesNotMatch(filterBar, /\.admin-list-filter-bar:focus-within/);
});

test('signed preview renders the shared filter surface immediately before the real list component', () => {
  assert.match(preview, /data-preview="admin-list-surface"/);
  assert.match(preview, /<AdminListFilterBar[\s\S]*?<\/AdminListFilterBar>\s*<AdminDataList/);
  assert.match(preview, /@media \(max-width: 800px\)[\s\S]*?\.preview-filter-controls[\s\S]*?--wa-touch-h/);
});

test('both page filter bars keep touch targets through the 768px tablet floor', () => {
  assert.match(
    decision,
    /@media \(max-width: 800px\)[\s\S]*?\.search-control[\s\S]*?--wa-touch-h/
  );
  assert.match(
    dailyJira,
    /@media \(max-width: 800px\)[\s\S]*?\.age-tab[\s\S]*?--wa-touch-h/
  );
});
