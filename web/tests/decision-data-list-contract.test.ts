import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/components/DecisionDashboard.svelte', import.meta.url), 'utf8');

test('decision agenda uses the shared data-list owner without a second table shell', () => {
  assert.match(source, /import AdminDataList from '\.\/admin-console\/AdminDataList\.svelte'/);
  assert.match(source, /<AdminDataList[\s\S]*?columns=\{visibleAgendaColumns\}[\s\S]*?rows=\{agendaRows\}/);
  assert.match(source, /cell=\{renderAgendaCell\}/);
  assert.match(source, /empty=\{renderAgendaEmpty\}/);
  assert.doesNotMatch(source, /<AdminDataList[\s\S]*?surface="embedded"/);
  assert.match(source, /className="decision-agenda-list"/);
  assert.match(source, /virtual=\{agendaVirtualOptions\}/);
  assert.match(source, /selectedRowId=\{selectedItem\?\.task_id \|\| ''\}/);
  assert.match(source, /error=\{errorMsg\}[\s\S]*?onRetry=\{\(\) => fetchAgenda\(true\)\}/);
  assert.doesNotMatch(source, /decision-table-shell/);
  assert.doesNotMatch(source, /<tr[\s\S]*?role="button"/);
});

test('decision list preserves configurable columns, Jira links, the shared issue marker and a real detail control', () => {
  assert.match(source, /agendaDefaultColumnKeys = \['task_id', 'title', 'owner', 'risk', 'due', 'status'\]/);
  assert.match(source, /<IssueTypeMark issueType=\{row\.source\.issue_type\} \/>/);
  assert.match(source, /class="decision-title-button"[\s\S]*?aria-haspopup="dialog"[\s\S]*?openDetailDrawer\(row\.source, event\)/);
  assert.match(source, /class="table-link"[\s\S]*?target="_blank"[\s\S]*?rel="noopener noreferrer"/);
});

test('decision list inherits the shared header surface instead of muting or reskinning it', () => {
  const headerRule = source.match(
    /:global\(\.decision-agenda-list \.admin-table th\)\s*\{([\s\S]*?)\n  \}/
  )?.[1] ?? '';

  assert.doesNotMatch(headerRule, /(?:^|\n)\s*(?:background(?:-color|-image)?|color|box-shadow|letter-spacing)\s*:/);
});
