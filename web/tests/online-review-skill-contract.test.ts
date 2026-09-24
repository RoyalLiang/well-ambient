import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(relativePath: string) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

const promptConfig = source('src/components/config/SolutionPromptConfig.svelte');
const settings = source('src/lib/settings-sections.ts');
const reviewCenter = source('src/components/CodeReviewCenter.svelte');

test('settings exposes code review through the existing online skill governance surface', () => {
  assert.match(settings, /id: 'solution_prompts'[\s\S]*?label: 'AI 技能治理'/);
  assert.match(settings, /管理在线 AI 技能的规则版本、作用范围与启用记录/);
  assert.match(promptConfig, /type PromptPurpose = [^;]*'code_review'/);
  assert.match(promptConfig, /value: 'code_review', label: '代码评审'/);
  assert.match(promptConfig, /<Select label="在线技能"/);
  assert.match(promptConfig, /<h2>AI 运行技能治理<\/h2>/);
});

test('code review skill is global-only and uses draft-test-activate workflow', () => {
  assert.match(promptConfig, /isCodeReview \? 'global' : scopeType/);
  assert.match(promptConfig, /disabled=\{isCodeReview\}/);
  assert.match(promptConfig, /activate: isCodeReview \? false : activateOnSave/);
  assert.match(promptConfig, /\/api\/solution-prompts\/\$\{prompt\.id\}\/test/);
  assert.match(promptConfig, /validation_status !== 'passed'/);
  assert.match(promptConfig, /必须先通过运行验证/);
  assert.match(promptConfig, /最近验证结果/);
  assert.match(promptConfig, /{#if !isCodeReview}[\s\S]*?测试 Markdown/);
});

test('review facts expose the frozen online skill binding', () => {
  assert.match(reviewCenter, /skill_version_id: number/);
  assert.match(reviewCenter, /skill_hash: string/);
  assert.match(reviewCenter, /\*\*在线技能\*\*/);
  assert.match(reviewCenter, /selected\.prompt_version/);
});
