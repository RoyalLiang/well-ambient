import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const demandKanban = readFileSync(
  new URL('../src/components/DemandKanban.svelte', import.meta.url),
  'utf8'
);
const commitTelemetry = readFileSync(
  new URL('../src/components/CommitTelemetryPanel.svelte', import.meta.url),
  'utf8'
);

const contractMarker = '/* Final schedule inspector contract:';
const contractStart = demandKanban.lastIndexOf(contractMarker);
assert.notEqual(contractStart, -1, 'final schedule inspector contract must remain explicit');

const finalContract = demandKanban.slice(contractStart);
const stackedBreakpoint = finalContract.indexOf('@media (max-width: 1280px)');
assert.notEqual(stackedBreakpoint, -1, 'stacked schedule breakpoint must remain explicit');

const desktopContract = finalContract.slice(0, stackedBreakpoint);
const stackedContract = finalContract.slice(stackedBreakpoint);

test('desktop schedule table and inspector share one stretched grid row', () => {
  assert.match(
    desktopContract,
    /\.schedule-dashboard \.schedule-main-grid \{[\s\S]*?align-items: stretch;/
  );
  assert.match(
    desktopContract,
    /\.schedule-dashboard \.schedule-inspector-panel\.wa-admin-inspector \{[\s\S]*?align-self: stretch;[\s\S]*?height: 100%;[\s\S]*?min-height: 0;[\s\S]*?overflow: hidden;/
  );
});

test('schedule tab bodies own scrolling while inline telemetry stays content-sized', () => {
  const sharedScrollContract = desktopContract.match(
    /\.schedule-editor-body,[\s\S]*?\{[\s\S]*?overflow: auto;[\s\S]*?\}/
  )?.[0] || '';
  assert.match(
    sharedScrollContract,
    /\.schedule-telemetry-inline(?:,|\s*\{)/,
    'inline telemetry must remain in the shared tab-body scroll contract'
  );
  assert.match(
    sharedScrollContract,
    /min-height: 0;[\s\S]*?flex: 1 1 auto;[\s\S]*?overflow: auto;/
  );
  assert.match(
    commitTelemetry,
    /\.drawer-panel\.is-inline \{[\s\S]*?height: auto;[\s\S]*?overflow: visible;/
  );
  assert.match(
    commitTelemetry,
    /\.drawer-panel\.is-inline \.drawer-body \{[\s\S]*?overflow: visible;/
  );
});

test('stacked schedule layout returns the inspector to natural height', () => {
  assert.match(
    stackedContract,
    /\.schedule-dashboard \.schedule-main-grid \{[\s\S]*?grid-template-columns: minmax\(0, 1fr\);[\s\S]*?align-items: start;/
  );
  assert.match(
    stackedContract,
    /\.schedule-dashboard \.schedule-inspector-panel\.wa-admin-inspector \{[\s\S]*?align-self: start;[\s\S]*?height: auto;[\s\S]*?max-height: none;[\s\S]*?overflow: visible;/
  );
});
