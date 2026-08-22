import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

function component(relativePath: string) {
  return readFileSync(new URL(`../src/components/${relativePath}`, import.meta.url), 'utf8');
}

function occurrences(source: string, pattern: RegExp) {
  return source.match(pattern)?.length || 0;
}

const overlayCloseButton = component('shared/OverlayCloseButton.svelte');
const sharedModal = component('shared/Modal.svelte');

const directOverlayConsumers = new Map([
  ['CommitTelemetryPanel.svelte', 1],
  ['DecisionDashboard.svelte', 1],
  ['DecisionEventCenter.svelte', 1],
  ['DemandKanban.svelte', 5],
  ['ProjectHealthTelemetry.svelte', 1],
  ['SettingsPanel.svelte', 5]
]);

const sharedModalConsumers = new Map([
  ['TaskKanban.svelte', 2],
  ['DecisionDashboard.svelte', 1],
  ['DeliveryPlan.svelte', 1],
  ['PerformanceCalculationGuide.svelte', 1]
]);

test('every overlay close action uses one shared 44px control', () => {
  assert.match(overlayCloseButton, /type="button"/);
  assert.match(overlayCloseButton, /aria-label=\{label\}/);
  assert.match(overlayCloseButton, /width: var\(--wa-touch-h, 44px\)/);
  assert.match(overlayCloseButton, /height: var\(--wa-touch-h, 44px\)/);
  assert.match(overlayCloseButton, /flex: 0 0 var\(--wa-touch-h, 44px\)/);

  let directSurfaceCount = 0;
  for (const [relativePath, expected] of directOverlayConsumers) {
    const source = component(relativePath);
    assert.equal(
      occurrences(source, /<OverlayCloseButton\b/g),
      expected,
      `${relativePath} must keep all direct close actions on OverlayCloseButton`
    );
    directSurfaceCount += expected;
  }

  let sharedSurfaceCount = 0;
  for (const [relativePath, expected] of sharedModalConsumers) {
    const source = component(relativePath);
    assert.equal(
      occurrences(source, /<Modal\b/g),
      expected,
      `${relativePath} shared modal surface count changed unexpectedly`
    );
    sharedSurfaceCount += expected;
  }

  assert.equal(directSurfaceCount + sharedSurfaceCount, 19);
});

test('modal headers reserve a non-shrinking close column and wrap long titles', () => {
  for (const source of [
    sharedModal,
    component('DemandKanban.svelte'),
    component('ProjectHealthTelemetry.svelte'),
    component('SettingsPanel.svelte')
  ]) {
    assert.match(source, /grid-template-columns: minmax\(0, 1fr\) var\(--wa-touch-h, 44px\)/);
    assert.match(source, /overflow-wrap: anywhere/);
  }

  for (const source of [
    component('DecisionDashboard.svelte'),
    component('DecisionEventCenter.svelte')
  ]) {
    assert.match(source, /grid-template-columns: minmax\(0, 1fr\) auto/);
    assert.match(source, /\.timeline-drawer-header > div:first-child \{\s+min-width: 0/);
    assert.match(source, /\.timeline-drawer-actions \{[\s\S]*?flex: 0 0 auto/);
  }

  const commitTelemetry = component('CommitTelemetryPanel.svelte');
  assert.match(commitTelemetry, /\.drawer-title-stack \{\s+min-width: 0/);
  assert.match(commitTelemetry, /\.drawer-actions \{[\s\S]*?flex: 0 0 auto/);
  assert.match(commitTelemetry, /use:portalToWorkspaceStage/);
  assert.match(commitTelemetry, /\.drawer-root\.is-workspace-scoped \{\s+position: fixed/);
  assert.match(commitTelemetry, /--wa-drawer-workspace-top/);
  assert.match(commitTelemetry, /window\.addEventListener\('resize', syncWorkspaceBounds\)/);
});

test('legacy undersized modal close buttons cannot return to the migrated surfaces', () => {
  const legacyPatterns = [
    /class="close-btn(?:\s|\")/,
    /class="close-modal-btn(?:\s|\")/,
    /class="timeline-drawer-close(?:\s|\")/,
    /class="close-btn confirm-close"/
  ];

  for (const relativePath of directOverlayConsumers.keys()) {
    const source = component(relativePath);
    for (const pattern of legacyPatterns) {
      assert.doesNotMatch(source, pattern, `${relativePath} contains a legacy overlay close control`);
    }
  }
});
