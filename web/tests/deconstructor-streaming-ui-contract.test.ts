import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const deconstructor = readFileSync(
  new URL('../src/components/Deconstructor.svelte', import.meta.url),
  'utf8'
);
const demandKanban = readFileSync(
  new URL('../src/components/DemandKanban.svelte', import.meta.url),
  'utf8'
);

test('AI deconstruction appends provider deltas into a visible typewriter surface', () => {
  assert.match(deconstructor, /let streamedOutput = ''/);
  assert.match(deconstructor, /event\.type === 'provider_delta'[\s\S]*?pendingStreamOutput \+= event\.delta/);
  assert.match(deconstructor, /requestAnimationFrame\([\s\S]*?streamedOutput \+= pendingStreamOutput/);
  assert.match(deconstructor, /class="streaming-output"[\s\S]*?\{streamedOutput \|\|/);
  assert.match(deconstructor, /aria-live="polite"/);
  assert.match(deconstructor, /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.streaming-cursor/);
});

test('task suggestion panel remains mounted while generation state changes', () => {
  assert.match(deconstructor, /class="compact-result-body"/);
  assert.doesNotMatch(
    deconstructor,
    /\{#if isLoading\}\s*<div class="compact-loading"[\s\S]*?\{:else\}\s*<div class="compact-task-list"/
  );
  assert.match(deconstructor, /\.compact-result-body \{[\s\S]*?min-height:/);
  assert.match(
    deconstructor,
    /\.deconstructor-workbench\.compact\.embedded \.entry-card-head \.wa-admin-action \{[\s\S]*?min-height: var\(--wa-touch-h, 44px\);[\s\S]*?white-space: nowrap;/
  );
});

test('deconstructor owns an abortable stream lifecycle and exposes safe host controls', () => {
  assert.match(deconstructor, /let deconstructController: AbortController \| null = null/);
  assert.match(deconstructor, /signal: deconstructController\.signal/);
  assert.match(deconstructor, /readDeconstructStream\([\s\S]*?signal: deconstructController\.signal/);
  assert.match(deconstructor, /export function hasActiveGeneration\(\)/);
  assert.match(deconstructor, /export function cancelActiveGeneration\(\)/);
});

test('all AI workbench close paths share confirmation then cancel the active connection', () => {
  assert.match(demandKanban, /bind:this=\{deconstructorWorkbench\}/);
  assert.match(demandKanban, /function requestCloseDeconstructorWorkspace/);
  assert.match(demandKanban, /deconstructorWorkbench\?\.hasActiveGeneration\(\)/);
  assert.match(demandKanban, /showDeconstructorCloseConfirm = true/);
  assert.match(demandKanban, /function confirmAndCloseDeconstructorWorkspace/);
  assert.match(demandKanban, /deconstructorWorkbench\?\.cancelActiveGeneration\(\)/);
  assert.match(demandKanban, /正在生成解构结果/);
  assert.match(demandKanban, /确认停止并关闭/);
  assert.match(demandKanban, /OverlayCloseButton[\s\S]*?requestCloseDeconstructorWorkspace\(\)/);
  assert.match(demandKanban, /handleDeconstructorBackdropClick[\s\S]*?requestCloseDeconstructorWorkspace\(\)/);
});
