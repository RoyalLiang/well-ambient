import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { responseErrorMessage } from '../src/lib/http-error.ts';

test('config save errors preserve the actionable backend message', async () => {
  const message = await responseErrorMessage(
    new Response(JSON.stringify({ message: 'Jira 查询范围无效，配置未保存' }), { status: 400 }),
    'HTTP 400'
  );
  assert.equal(message, 'Jira 查询范围无效，配置未保存');
});

test('config save callers do not replace HTTP validation with a network error', () => {
  for (const component of ['SettingsPanel.svelte', 'IntegrationPanel.svelte']) {
    const source = readFileSync(new URL(`../src/components/${component}`, import.meta.url), 'utf8');
    assert.match(source, /responseErrorMessage\(res,/);
    assert.doesNotMatch(source, /if \(!res\.ok\) throw new Error\('Failed to save config'\)/);
  }
});
