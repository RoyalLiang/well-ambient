import assert from 'node:assert/strict';
import test from 'node:test';
import { filterSkills, prefixError, publicationDisabledSkills, type OpenSkillItem } from '../src/lib/openSkillRegistry.ts';

import { skillInstallCommand } from '../src/lib/openCapabilityInstall.ts';

test('individual install commands use a single Bash argument boundary and selected client', () => {
  const command = skillInstallCommand('https://ambient.test', 'team-2-jira-analysis', 'codex');
  assert.equal(command, `curl -fsSL 'https://ambient.test/open/v1/install.sh' | bash -s -- --root "$HOME/.codex" --skill team-2-jira-analysis --skip-mcp`);
  assert.equal((command.match(/-s --/g) || []).length, 1);
  assert.match(skillInstallCommand('https://ambient.test', 'well-jira-analysis', 'claude'), /--root "\$HOME\/\.claude"/);
  assert.doesNotMatch(skillInstallCommand('https://ambient.test', 'well-jira-analysis', 'choose'), /--root/);
  assert.equal(skillInstallCommand('https://ambient.test', 'well-jira;id', 'codex'), '');
});

test('registry search combines name, purpose and tool terms without changing source records', () => {
  const rows = [{ name: 'well-jira-analysis', title: 'Jira 分析', description: '发布数据聚合', tools: ['well-jira_search_issues'] }, {name: 'well-board-decision', title: '排期', description: '决策', tools: ['well-decision_prepare']}] as OpenSkillItem[];
  assert.deepEqual(filterSkills(rows, 'jira 聚合'), [rows[0]]);
  assert.deepEqual(filterSkills(rows, 'DECISION_PREPARE'), [rows[1]]);
  assert.equal(filterSkills(rows, 'unknown').length, 0);
  assert.equal(filterSkills(rows, '').length, 2);
});

test('prefix preview accepts default and rejects command/path punctuation', () => {
  for (const prefix of ['', 'well', 'team-2']) assert.equal(prefixError(prefix), '');
  for (const prefix of ['a/b', 'a_b', 'well--team', 'Well', 'a'.repeat(33)]) assert.notEqual(prefixError(prefix), '');
});

test('publishing one child from a stopped parent preserves sibling exclusions', () => {
 const children = ['jira-analysis', 'board-decision', 'runtime-knowledge', 'code-review-reader'];
 assert.deepEqual(publicationDisabledSkills(['well-ambient'], children, 'runtime-knowledge', true).sort(), ['jira-analysis', 'board-decision', 'code-review-reader'].sort());
 assert.deepEqual(publicationDisabledSkills(['board-decision'], children, 'runtime-knowledge', true), ['board-decision']);
 assert.deepEqual(publicationDisabledSkills([], children, 'runtime-knowledge', false), ['runtime-knowledge']);
});
