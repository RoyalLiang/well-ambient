import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(
  new URL('../src/components/DailyJiraAudit.svelte', import.meta.url),
  'utf8'
);

test('manual Daily Jira refresh synchronizes Jira before reloading the local projection', () => {
  assert.match(source, /fetch\('\/api\/decision\/daily-jira\/sync', \{ method: 'POST' \}\)/);
  assert.match(source, /loadAudit\(false, true, true\)/);
  assert.match(source, />同步 Jira<\/Button>/);
});

test('decision writeback requires a non-blank Jira comment in UI and payload', () => {
  assert.match(source, /decisionSubmitDisabled = !decisionNote\.trim\(\)/);
  assert.match(source, /required\s+aria-required="true"/);
  assert.match(source, /请填写决策评论后再提交/);
  assert.match(source, /note: decisionNote\.trim\(\)/);
  assert.match(source, /提交后将同步为该事项的 Jira 评论/);
  assert.match(source, /result\.jira_sync === 'completed'/);
  assert.match(source, /当前未启用 Jira 同步/);
});

test('reassignment exposes reporter and specified-person targets without duplicating the form', () => {
  assert.match(source, /type AssigneeMode = 'reporter' \| 'specified'/);
  assert.match(source, /label="负责人去向"/);
  assert.match(source, /label: '指回报告人'/);
  assert.match(source, /label: '指定负责人'/);
  assert.match(source, /assignee_mode: selectedDecision === 'reassign' \? assigneeMode : ''/);
  assert.match(source, /评论并指回报告人/);
  assert.match(source, /评论并更新 Jira/);
});
