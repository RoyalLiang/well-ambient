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

test('the shared browser shell remains fixed to one dynamic viewport at every breakpoint', () => {
  const contract = shell.slice(shell.indexOf('/* Final viewport container contract:'));
  assert.match(
    contract,
    /\.functional-console(?:\.rail-collapsed)?\s*\{[^}]*height:\s*100dvh(?:\s*!important)?;[^}]*min-height:\s*100dvh;[^}]*overflow:\s*hidden(?:\s*!important)?;/s
  );
  assert.match(
    contract,
    /\.console-main\s*\{[^}]*height:\s*100dvh(?:\s*!important)?;[^}]*min-height:\s*0(?:\s*!important)?;[^}]*overflow:\s*hidden(?:\s*!important)?;/s
  );
  assert.match(
    contract,
    /\.workspace-stage\s*\{[^}]*min-height:\s*0;[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    contract,
    /\.workspace-frame(?:,[^{}]+)?\s*\{[^}]*height:\s*100%;[^}]*min-height:\s*0;[^}]*overflow-x:\s*hidden;[^}]*overflow-y:\s*auto;/s
  );
});

test('viewport-fit pages share one full-height workspace chain instead of growing the document', () => {
  const contract = workspace.slice(workspace.indexOf('/* Final viewport page-height contract:'));
  assert.match(
    contract,
    /viewport-fit-frame \.functional-workspace[\s\S]*?height:\s*100%;[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*hidden;/
  );
  assert.match(
    contract,
    /viewport-fit-frame \.workspace-content[\s\S]*?height:\s*100%;[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*hidden;/
  );
});

test('Daily Jira keeps the desktop page fixed and leaves scrolling to the table scrollport', () => {
  const contract = dailyJira.slice(dailyJira.indexOf('/* Final Daily Jira page-height contract:'));
  assert.match(
    contract,
    /\.daily-jira\s*\{[^}]*height:\s*100%;[^}]*min-height:\s*0;[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    contract,
    /\.audit-grid\s*\{[^}]*height:\s*100%;[^}]*min-height:\s*0;/s
  );
  assert.match(
    contract,
    /@media \(min-width:\s*1181px\) and \(max-height:\s*1000px\)[\s\S]*?\.audit-grid\s*\{[^}]*height:\s*100%;[^}]*min-height:\s*0;/
  );
  assert.match(
    contract,
    /@media \(max-width:\s*1180px\)[\s\S]*?\.audit-grid\s*\{[^}]*height:\s*max-content;[^}]*min-height:\s*max-content;[^}]*align-items:\s*start;/
  );
});
