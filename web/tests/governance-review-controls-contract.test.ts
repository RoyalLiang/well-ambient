import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8');
const center = read('src/components/AIGovernanceCenter.svelte');
const matrix = read('src/components/config/ReviewControlMatrix.svelte');
const section = (start: string, end: string) => center.slice(center.indexOf(start), center.indexOf(end, center.indexOf(start)));

test('global and repository matrices use the same independent MR/Commit controls', () => {
  assert.equal((center.match(/<ReviewControlMatrix\b/g) || []).length, 2);
  assert.match(center, /title="全局"[\s\S]*?values=\{commentSettings\}/);
  assert.match(center, /title="当前仓库"[\s\S]*?values=\{editingPolicy\} globalValues=\{commentSettings\}/);
  assert.match(matrix, /name: 'MR', fields: \['review_mrs', 'sync_mrs'\]/);
  assert.match(matrix, /name: 'Commit', fields: \['review_commits', 'sync_commits'\]/);
  assert.match(matrix, /label=\{controlLabel\(row\.name, field\)\}/);
  assert.match(matrix, /optimistic=\{false\}[\s\S]*?expandedHitArea/);
  assert.match(matrix, /saving=\{saving === field\}/);
  assert.match(matrix, /disabled=\{disabled \|\| loading \|\| !values \|\| Boolean\(saving\)\}/);
  assert.match(matrix, /全局已暂停/);
  assert.match(matrix, /\{#if saving === field \|\| globallyPaused\(field\)\}[\s\S]*?<span class="cell-state"/);
  assert.match(matrix, /\.control-cell \{[\s\S]*?min-height: 44px/);
});

test('global switch state changes only after acknowledged persistence and failures use toast', () => {
  const save = section('  async function saveCommentSetting(', '  async function saveRepoSetting(');
  assert.ok(save.indexOf('await api(') < save.indexOf('commentSettings = confirmed'));
  assert.ok(save.indexOf('confirmed[field] !== checked') < save.indexOf('commentSettings = confirmed'));
  assert.match(save, /showToast\([\s\S]*?type: 'success'/);
  assert.match(save, /catch[\s\S]*?showToast\([\s\S]*?type: 'error'/);
  assert.doesNotMatch(center, /commentSettings(?:Notice|Error)|policyNotice/);
  assert.match(center, /typeof value\?\.\[field\] !== 'boolean'/);
});

test('repository switch saves retain rule drafts and isolate the acknowledged repository', () => {
  const save = section('  async function saveRepoSetting(', '  async function saveRepoPolicy(');
  assert.match(save, /const projectID = editingPolicy\.project_id/);
  assert.match(save, /JSON\.stringify\(\{ project_id: projectID, \[field\]: checked \}\)/);
  assert.match(save, /if \(editingPolicy\?\.project_id === projectID\) editingPolicy = \{ \.\.\.editingPolicy, \.\.\.confirmed \}/);
  assert.doesNotMatch(save, /rules:|policyDrafts\s*=/);
  assert.ok(save.indexOf('confirmed[field] !== checked') < save.indexOf('repoPolicies ='));
  assert.match(save, /type: 'error', title: '保存失败'/);
});

test('rules preserve actionable states and opt out of shared editor help without changing its defaults', () => {
  const rules = section("{#if resolvedActiveSection === 'rules'}", '<!-- TAB 4:');
  assert.match(rules, /retrySection\('rules'\)[\s\S]*?重新加载/);
  assert.match(rules, /只读模式/);
  assert.match(rules, /policyRulesDirty[\s\S]*?未保存/);
  assert.match(rules, /规则内容超过 8000 字符/);
  assert.doesNotMatch(rules, /管理仓库工程规则与评审评论|分别控制后续评审|同时受全局评审开关控制|全局允许后/);
  assert.match(rules, /showStatusDescription=\{false\}/);
  assert.match(rules, /readonly=\{!canWriteRules \|\| savingPolicy \|\| !!savingRepoSetting \|\| loadingPolicies\}/);
  assert.match(rules, /if \(!loadingPolicies && policyRulesDirty/);
  const saveRules = section('  async function saveRepoPolicy(', '  function handleFileUpload(');
  assert.match(saveRules, /savingRepoSetting \|\| loadingPolicies\) return/);
  const markdown = read('src/components/shared/MarkdownWorkbench.svelte');
  assert.match(markdown, /export let showStatusDescription = true/);
  assert.match(markdown, /showStatusDescription \|\| saving/);
  assert.match(markdown, /else if saving\}<span>保存中…/);
});
