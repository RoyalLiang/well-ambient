import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const telemetryPanel = readFileSync(
  new URL('../src/components/CommitTelemetryPanel.svelte', import.meta.url),
  'utf8'
);

test('inline schedule telemetry renders every returned record', () => {
  assert.doesNotMatch(
    telemetryPanel,
    /presentation === 'inline' \? commits\.slice\(0,\s*\d+\) : commits/,
    'inline telemetry must not truncate the returned commit list'
  );
  assert.match(
    telemetryPanel,
    /\{#each commits as log\}/,
    'the timeline must iterate over the complete commit list'
  );
});

test('inline schedule telemetry does not hide records behind a cross-page hint', () => {
  assert.doesNotMatch(telemetryPanel, /另有 \{[^}]+\} 条轨迹/);
  assert.doesNotMatch(telemetryPanel, /任务跟踪中查看完整记录/);
  assert.doesNotMatch(telemetryPanel, /inline-overflow-note/);
});

test('inline schedule telemetry exposes complete messages instead of line clamping them', () => {
  assert.match(
    telemetryPanel,
    /\.drawer-panel\.is-inline \.timeline-body \{[\s\S]*?display: block;[\s\S]*?overflow: visible;[\s\S]*?line-clamp: unset;[\s\S]*?-webkit-line-clamp: unset;[\s\S]*?white-space: pre-wrap;/
  );
  assert.match(
    telemetryPanel,
    /\.drawer-panel\.is-inline \.mr-timeline-link \{[\s\S]*?overflow: visible;[\s\S]*?text-overflow: clip;[\s\S]*?white-space: normal;/
  );
});
