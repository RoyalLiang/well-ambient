import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function component(name: string) {
  return readFileSync(new URL(`../src/components/${name}`, import.meta.url), 'utf8');
}

test('all Jira-backed product views subscribe through the shared telemetry refresh scheduler', () => {
  for (const name of [
    'TaskKanban.svelte',
    'DemandKanban.svelte',
    'DecisionDashboard.svelte',
    'DailyJiraAudit.svelte'
  ]) {
    const source = component(name);
    assert.match(source, /import \{ subscribeTelemetryUpdates \} from '\.\.\/lib\/telemetry-refresh';/);
    assert.match(source, /subscribeTelemetryUpdates\(/);
    assert.match(source, /unsubscribeTelemetryUpdates\(\)|return unsubscribeTelemetryUpdates|unsubscribeJiraUpdates\(\)/);
  }
});

test('event-driven refresh keeps existing polling fallbacks for reconnect recovery', () => {
  assert.match(component('TaskKanban.svelte'), /setInterval\([\s\S]*?60000\)/);
  assert.match(component('DemandKanban.svelte'), /setInterval\([\s\S]*?15000\)/);
  assert.match(component('DecisionDashboard.svelte'), /setInterval\([\s\S]*?15000\)/);
});

test('Daily Jira recovers missed telemetry events with a visible-page polling fallback', () => {
  const source = component('DailyJiraAudit.svelte');
  assert.match(source, /setInterval\([\s\S]*?30000\)/);
  assert.match(source, /document\.visibilityState\s*===\s*'visible'/);
  assert.match(source, /clearInterval\(/);
});
