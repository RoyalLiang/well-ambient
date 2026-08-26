import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const demandKanban = readFileSync(
  new URL('../src/components/DemandKanban.svelte', import.meta.url),
  'utf8'
);

const recordStart = demandKanban.indexOf('function buildScheduleInspectorRecord');
const recordEnd = demandKanban.indexOf('$: scheduleDataListRows', recordStart);
assert.notEqual(recordStart, -1, 'schedule inspector record builder must exist');
assert.notEqual(recordEnd, -1, 'schedule inspector record builder must have a stable end marker');
const recordBuilder = demandKanban.slice(recordStart, recordEnd);

const headStart = demandKanban.indexOf('<div class="schedule-inspector-head">');
const headEnd = demandKanban.indexOf('{#if scheduleInspectorMode !== \'solution\'}', headStart);
assert.notEqual(headStart, -1, 'schedule inspector header must exist');
assert.notEqual(headEnd, -1, 'schedule inspector header must have a stable end marker');
const inspectorHead = demandKanban.slice(headStart, headEnd);

const schedulePanelStart = demandKanban.indexOf('id="schedule-inspector-panel-schedule"');
const schedulePanelEnd = demandKanban.indexOf('<div class="schedule-editor-grid">', schedulePanelStart);
assert.notEqual(schedulePanelStart, -1, 'schedule settings panel must exist');
assert.notEqual(schedulePanelEnd, -1, 'schedule settings panel must have a stable form marker');
const schedulePanelLead = demandKanban.slice(schedulePanelStart, schedulePanelEnd);

test('schedule summary orders AI progress, project priority and project in the inspector header', () => {
  const progressIndex = inspectorHead.indexOf('schedule-ai-progress-pill');
  const priorityIndex = inspectorHead.indexOf('schedule-priority-pill');
  const projectIndex = inspectorHead.indexOf('schedule-project-label');

  assert.ok(progressIndex >= 0, 'AI deconstruction progress belongs in the top-left summary rail');
  assert.ok(priorityIndex > progressIndex, 'project priority must follow AI progress');
  assert.ok(projectIndex > priorityIndex, 'project must occupy the right-most former Jira-link position');
  assert.match(inspectorHead, /getScheduleProgress\(selectedScheduleItem\)/);
  assert.match(inspectorHead, /\{scheduleProjectPriority\}/);
  assert.match(inspectorHead, /\{scheduleProject\}/);
  assert.match(demandKanban, /selectedScheduleItem\.project_priority \|\| getProjectPriority/);
  assert.match(demandKanban, /scheduleProject = getScheduleProjectLabel\(selectedScheduleItem\)/);
});

test('schedule settings keeps only proposed and planned completion dates as one pill rail', () => {
  assert.match(recordBuilder, /facts:\s*\[\s*\{ label: '提出日期'/);
  assert.match(recordBuilder, /\{ label: '计划完成日期'/);
  for (const removedLabel of ['负责人', '所属项目', '关联目标', '当前阶段', '预计工作量', '任务组']) {
    assert.doesNotMatch(recordBuilder, new RegExp(`label: '${removedLabel}'`));
  }

  assert.match(schedulePanelLead, /class="schedule-date-pills"/);
  assert.match(schedulePanelLead, /\{#each scheduleInspectorRecord\.facts as fact\}/);
  assert.doesNotMatch(schedulePanelLead, /scheduleInspectorRecord\.facts\.slice/);
});

test('development schedule heading no longer repeats AI deconstruction progress', () => {
  assert.doesNotMatch(schedulePanelLead, /<div class="schedule-editor-progress">/);
});
