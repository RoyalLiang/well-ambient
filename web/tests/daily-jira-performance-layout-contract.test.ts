import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(
  new URL('../src/components/DailyJiraAudit.svelte', import.meta.url),
  'utf8'
);
const dataListSource = readFileSync(
  new URL('../src/components/admin-console/AdminDataList.svelte', import.meta.url),
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
  assert.match(source, /retainCurrentWindow/);
  assert.match(source, /currentPageState\?\.generation\s*===\s*nextAudit\.page\.generation/);
  assert.match(source, /flattenBoundedPages\(currentPages\)/);
  assert.match(source, /audit\s*=\s*candidateAudit/);
  assert.doesNotMatch(source, /currentItems\.slice\(auditPageLimit\)/);
});

test('Daily Jira delegates its bounded visible window to AdminDataList', () => {
  assert.match(source, /import AdminDataList from '\.\/admin-console\/AdminDataList\.svelte'/);
  assert.match(source, /<AdminDataList/);
  assert.match(source, /virtual=\{dailyJiraVirtualOptions\}/);
  assert.match(source, /rowHeight:\s*52/);
  assert.match(source, /overscan:\s*8/);
  assert.doesNotMatch(source, /visibleItems\s*=\s*filteredItems\.slice/);
  assert.doesNotMatch(source, /topSpacerHeight|bottomSpacerHeight|handleTableScroll/);
  assert.match(dataListSource, /visibleRows\s*=\s*\$derived/);
});

test('Daily Jira inherits the complete shared table style contract', () => {
  assert.doesNotMatch(source, /:global\([^)]*\.(?:admin-table|table-scroll)\b/);
  assert.match(dataListSource, /data-style-contract="admin-data-list\/v1"/);
  assert.match(dataListSource, /\.admin-table th,[\s\S]*?padding-block:\s*0/);
  assert.match(dataListSource, /\.admin-table tbody tr\.data-row\s*\{[\s\S]*?height:\s*var\(--admin-row-height\)/);
});

test('Daily Jira consumes bounded cursor pages instead of a whole audit bucket', () => {
  assert.match(source, /interface DailyJiraPageMeta/);
  assert.match(source, /URLSearchParams/);
  assert.match(source, /params\.set\('bucket',\s*bucket\)/);
  assert.match(source, /params\.set\('limit',\s*String\(auditPageLimit\)\)/);
  assert.match(source, /params\.set\('cursor',\s*cursor\)/);
  assert.match(source, /async function loadNextPage/);
  assert.match(source, /onLoadMore=\{loadNextPage\}/);
  assert.match(source, /\{loadMoreKey\}/);
  assert.match(source, /filteredItems\s*=\s*activeBucket\?\.items\s*\|\|\s*\[\]/);
  assert.doesNotMatch(source, /filterItems\(activeBucket\?\.items/);
});

test('Daily Jira rejects stale append responses before publishing them', () => {
  assert.match(source, /requestedGeneration\s*=\s*pageState\.generation/);
  assert.match(source, /requestedCursor\s*=\s*pageState\.next_cursor/);
  assert.match(source, /currentPageState\?\.generation\s*!==\s*requestedGeneration/);
  assert.match(source, /currentPageState\?\.next_cursor\s*!==\s*requestedCursor/);
  assert.match(source, /nextAudit\.page\.generation\s*!==\s*requestedGeneration/);
  assert.match(source, /loading\s*\|\|\s*refreshing\s*\|\|\s*loadingPrevious\s*\|\|\s*loadingMore/);
  assert.match(source, /previous_cursor/);
  assert.match(source, /has_previous/);
  assert.match(source, /dailyJiraWindowPageLimit\s*=\s*3/);
  assert.match(source, /updateBoundedPageWindow/);
  assert.match(source, /async function loadPreviousPage/);
  assert.match(source, /onLoadPrevious=\{loadPreviousPage\}/);
});

test('Daily Jira uses one table scrollport and a fixed non-scrolling inspector', () => {
  const finalSurface = source.slice(source.indexOf('/* L0 page gaps remain visible'));
  assert.doesNotMatch(finalSurface, /\.audit-table-panel,[\s\S]*?\.audit-inspector\s*\{[^}]*backdrop-filter:/);
  assert.doesNotMatch(finalSurface, /62dvh/);
  assert.match(finalSurface, /62svh/);
  assert.doesNotMatch(finalSurface, /:global\([^)]*\.table-scroll\b/);
  assert.match(dataListSource, /\.table-scroll\s*\{[^}]*overflow-anchor:\s*none/s);
  assert.match(
    finalSurface,
    /\.audit-inspector\s*\{[^}]*height:\s*100%;[^}]*overflow:\s*hidden;[^}]*scrollbar-gutter:\s*auto;/s
  );
  assert.doesNotMatch(
    finalSurface,
    /\.audit-inspector\s*\{[^}]*overflow-y:\s*auto/s
  );
  assert.match(
    finalSurface,
    /@media \(max-width:\s*1180px\)[\s\S]*?\.daily-jira\s*\{[^}]*height:\s*auto;[^}]*overflow:\s*visible;[\s\S]*?\.audit-inspector\s*\{[^}]*height:\s*auto;[^}]*overflow:\s*visible;/
  );
});

test('expanded reassignment fits the fixed desktop inspector without overlapping history', () => {
  const finalSurface = source.slice(source.indexOf('/* L0 page gaps remain visible'));
  const shortViewportStart = finalSurface.indexOf('@media (min-width: 1181px) and (max-height: 1000px)');
  const shortViewport = finalSurface.slice(
    shortViewportStart,
    finalSurface.indexOf('@media (max-width: 1180px)', shortViewportStart)
  );
  assert.match(
    finalSurface,
    /@media \(min-width:\s*1181px\)[\s\S]*?\.audit-inspector\s*\{[^}]*grid-template-rows:\s*max-content max-content max-content max-content minmax\(76px, max-content\);/
  );
  assert.match(
    finalSurface,
    /\.audit-inspector:has\(\.assignee-field\) \.fact-list\s*\{[^}]*grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\);/
  );
  assert.match(
    finalSurface,
    /\.audit-inspector \.note-field textarea\s*\{[^}]*height:\s*52px;[^}]*min-height:\s*52px;/
  );
  assert.match(
    finalSurface,
    /@media \(min-width:\s*1181px\) and \(max-height:\s*1000px\)[\s\S]*?\.audit-inspector\s*\{[^}]*padding:\s*14px;/
  );
  assert.match(
    shortViewport,
    /\n\s*\.fact-list\s*\{[^}]*gap:\s*0 12px;[^}]*padding:\s*6px 0 8px;/
  );
  assert.doesNotMatch(
    shortViewport,
    /\n\s*\.fact-list\s*\{[^}]*grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\);/
  );
  assert.match(
    shortViewport,
    /\.audit-inspector:has\(\.assignee-field\) \.fact-list\s*\{[^}]*grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\);/
  );
  assert.match(
    finalSurface,
    /@media \(min-width:\s*1181px\) and \(max-height:\s*1000px\)[\s\S]*?\.audit-inspector \.note-field textarea\s*\{[^}]*height:\s*44px;[^}]*min-height:\s*44px;/
  );
});
