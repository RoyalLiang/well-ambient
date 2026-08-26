import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const demandKanban = readFileSync(
  new URL('../src/components/DemandKanban.svelte', import.meta.url),
  'utf8'
);
const decisionDashboard = readFileSync(
  new URL('../src/components/DecisionDashboard.svelte', import.meta.url),
  'utf8'
);

test('decision dashboard gives its second visible child the remaining Jira-aligned height', () => {
  const surfaceContract = decisionDashboard.slice(
    decisionDashboard.indexOf('/* Component-owned surface hierarchy:')
  );

  assert.match(
    surfaceContract,
    /\.decision-admin\s*\{[\s\S]*?grid-template-rows:\s*116px minmax\(0, 1fr\);/
  );
  assert.doesNotMatch(surfaceContract, /grid-template-rows:\s*116px auto minmax\(0, 1fr\);/);
});

test('schedule governance delegates table rendering and virtual scrolling to AdminDataList', () => {
  assert.match(demandKanban, /import AdminDataList from '\.\/admin-console\/AdminDataList\.svelte'/);
  assert.match(
    demandKanban,
    /<AdminDataList[\s\S]*?columns=\{scheduleTableColumns\}[\s\S]*?rows=\{scheduleDataListRows\}/
  );
  assert.match(demandKanban, /cell=\{renderScheduleCell\}/);
  assert.match(demandKanban, /actions=\{renderScheduleActions\}/);
  assert.match(demandKanban, /virtual=\{scheduleVirtualOptions\}/);
  assert.match(demandKanban, /selectedRowId=\{selectedScheduleItem\?\.demand_id \|\| ''\}/);
  assert.doesNotMatch(demandKanban, /<table class="wa-admin-table schedule-admin-table">/);
  assert.doesNotMatch(demandKanban, /schedule-spacer-row/);
  assert.doesNotMatch(demandKanban, /handleScheduleScroll/);
});

test('schedule editor groups related fields without shrinking control targets', () => {
  assert.match(demandKanban, /class="schedule-editor-field is-assignee"/);
  assert.match(demandKanban, /class="schedule-editor-field is-hours"/);
  assert.match(demandKanban, /class="schedule-editor-field is-difficulty"/);
  assert.match(demandKanban, /class="schedule-editor-field is-due"/);
  assert.match(demandKanban, /class="schedule-editor-field is-task-group"/);

  const finalContract = demandKanban.slice(
    demandKanban.lastIndexOf('/* Final schedule inspector contract:')
  );
  assert.match(
    finalContract,
    /\.schedule-editor-grid\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0, 1\.25fr\) minmax\(0, 0\.7fr\) minmax\(0, 0\.85fr\);/
  );
  assert.match(
    finalContract,
    /\.schedule-editor-field\.is-task-group\s*\{[\s\S]*?grid-column:\s*2 \/ -1;/
  );
  assert.match(
    finalContract,
    /@media \(max-width: 760px\)[\s\S]*?\.schedule-editor-grid\s*\{[\s\S]*?grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/
  );
});

test('schedule editor retries after the editable demand map finishes loading', () => {
  assert.match(
    demandKanban,
    /const editorDemandReady = demandsById\.has\(selectedScheduleItem\.demand_id\);[\s\S]*?if \(editorDemandReady \|\| !loading\)[\s\S]*?loadScheduleEditor\(selectedScheduleItem\)/
  );
});
