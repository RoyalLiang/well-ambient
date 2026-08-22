import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const dailyJiraAudit = readFileSync(
  new URL('../src/components/DailyJiraAudit.svelte', import.meta.url),
  'utf8'
);

const contractMarker = '/* Final Daily Jira inspector header contract:';
const contractStart = dailyJiraAudit.lastIndexOf(contractMarker);
const finalContract = contractStart === -1 ? '' : dailyJiraAudit.slice(contractStart);

test('Jira number is the only external-link entry and keeps a static fallback', () => {
  assert.notEqual(contractStart, -1, 'final Daily Jira inspector header contract must remain explicit');
  assert.match(
    dailyJiraAudit,
    /\{#if getJiraUrl\(selectedItem\.task_id\)\}\s*<a\s+class="jira-key jira-link"\s+href=\{getJiraUrl\(selectedItem\.task_id\)\}\s+target="_blank"\s+rel="noopener noreferrer"\s+aria-label=\{`在 Jira 中打开 \$\{selectedItem\.task_id\}`\}\s*>\{selectedItem\.task_id\}<\/a>\s*\{:else\}\s*<strong class="jira-key">\{selectedItem\.task_id\}<\/strong>/,
    'the Jira number must carry the deep link while preserving a non-interactive fallback'
  );
  assert.doesNotMatch(
    dailyJiraAudit,
    />在 Jira 打开<\/a>/,
    'the duplicate standalone Jira action must not be rendered'
  );
});

test('desktop header is a single content column with a full-width title', () => {
  assert.match(
    finalContract,
    /\.inspector-header \{[\s\S]*?display: grid;[\s\S]*?grid-template-columns: minmax\(0, 1fr\);[\s\S]*?grid-template-areas:\s*"meta"\s*"title";/,
    'desktop header must model metadata and title without a vacant action column'
  );
  assert.match(finalContract, /\.inspector-meta-line \{[\s\S]*?grid-area: meta;/);
  assert.match(finalContract, /\.inspector-header h3 \{[\s\S]*?grid-area: title;[\s\S]*?width: 100%;/);
  assert.doesNotMatch(finalContract, /grid-area: action;/);
});

test('linked Jira chip exposes clear pointer, hover, focus, and active states', () => {
  assert.match(finalContract, /\.jira-link \{[\s\S]*?cursor: pointer;/);
  assert.match(finalContract, /\.jira-link::after \{[\s\S]*?inset-block: -10px;/);
  assert.match(finalContract, /\.jira-link:hover \{[\s\S]*?border-color:/);
  assert.match(finalContract, /\.jira-link:focus-visible \{[\s\S]*?outline: 2px solid/);
  assert.match(finalContract, /\.jira-link:active \{[\s\S]*?transform: translateY\(1px\);/);
});

test('phone header keeps the same metadata and title order without an action row', () => {
  assert.match(
    finalContract,
    /@media \(max-width: 520px\) \{[\s\S]*?\.inspector-header \{[\s\S]*?grid-template-columns: minmax\(0, 1fr\);[\s\S]*?grid-template-areas:\s*"meta"\s*"title"\s*;/,
    'phone header must retain the same compact metadata and title order'
  );
  assert.match(
    finalContract,
    /@media \(max-width: 520px\) \{[\s\S]*?\.inspector-meta-line \{[\s\S]*?min-height: 44px;/,
    'the compact visual chip must retain a 44px mobile interaction row'
  );
  assert.doesNotMatch(finalContract, /"action";/);
});
