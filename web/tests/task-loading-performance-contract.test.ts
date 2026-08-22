import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8');
}

test('task status view loads the active working set and fetches a selected historical item directly', () => {
  const component = source('../src/components/TaskKanban.svelte');
  const shell = source('../src/components/prototype/FunctionalAdminShell.svelte');
  assert.match(component, /params\.set\('active', 'true'\)/);
  assert.match(component, /fetch\(`\/api\/work-items\/\$\{encodeURIComponent\(taskId\)\}`/);
  assert.doesNotMatch(component, /\/api\/work-items\?limit=500&offset=\$\{offset\}/);
  assert.match(shell, /well-ambient:global-search-clear/);
  assert.match(component, /handleGlobalSearchClear/);
});

test('agenda consumers use one shared resource and only the event center owns the polling fallback', () => {
  const dashboard = source('../src/components/DecisionDashboard.svelte');
  const eventCenter = source('../src/components/DecisionEventCenter.svelte');
  assert.match(dashboard, /subscribeAgendaSummary/);
  assert.match(eventCenter, /subscribeAgendaSummary/);
  assert.doesNotMatch(dashboard, /fetch\('\/api\/agenda\/summary'\)/);
  assert.doesNotMatch(eventCenter, /fetch\('\/api\/agenda\/summary'\)/);
  assert.doesNotMatch(dashboard, /setInterval\([\s\S]*?fetchAgenda\(\)/);
  assert.match(eventCenter, /setInterval\([\s\S]*?15000\)/);
});

test('decision dashboard loads release facts without building the full delivery cockpit', () => {
  const dashboard = source('../src/components/DecisionDashboard.svelte');
  assert.match(dashboard, /fetch\('\/api\/strongest-brain\/releases'\)/);
  assert.doesNotMatch(dashboard, /fetch\('\/api\/strongest-brain\/delivery-cockpit'\)/);
});
