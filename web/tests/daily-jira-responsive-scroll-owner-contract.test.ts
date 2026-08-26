import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const shell = readFileSync(
  new URL('../src/components/prototype/FunctionalAdminShell.svelte', import.meta.url),
  'utf8'
);
const workspace = readFileSync(
  new URL('../src/components/prototype/FunctionalWorkspace.svelte', import.meta.url),
  'utf8'
);
const dailyJira = readFileSync(
  new URL('../src/components/DailyJiraAudit.svelte', import.meta.url),
  'utf8'
);

test('Daily Jira identifies its fixed workspace frame without affecting sibling views', () => {
  assert.match(
    shell,
    /class:daily-jira-frame=\{activeRoute === 'decision' && activeDecisionView === 'daily_jira'\}/
  );
  assert.doesNotMatch(shell, /\.workspace-frame\.viewport-fit-frame\.daily-jira-frame\s*\{[^}]*overflow-y:\s*auto;/);
});

test('Daily Jira owns responsive scrolling inside a fixed full-height workspace chain', () => {
  assert.match(
    workspace,
    /Final viewport page-height contract:[\s\S]*?daily-jira-frame \.workspace-content > \.decision-center[\s\S]*?height:\s*100%;[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*hidden;/
  );
  assert.doesNotMatch(
    workspace.slice(workspace.indexOf('/* Final viewport page-height contract:')),
    /daily-jira-frame \.workspace-content > \.decision-center > \.daily-jira\)\s*\{[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    dailyJira.slice(dailyJira.indexOf('/* Final Daily Jira page-height contract:')),
    /@media \(max-width:\s*1180px\)[\s\S]*?\.audit-grid\s*\{[^}]*height:\s*max-content;[^}]*min-height:\s*max-content;/
  );
});

test('a short wide viewport consumes the available row instead of creating parent scrolling', () => {
  assert.match(
    dailyJira,
    /Final Daily Jira page-height contract:[\s\S]*?@media \(min-width:\s*1181px\) and \(max-height:\s*1000px\)[\s\S]*?\.audit-grid\s*\{[^}]*height:\s*100%;[^}]*min-height:\s*0;/
  );
});
