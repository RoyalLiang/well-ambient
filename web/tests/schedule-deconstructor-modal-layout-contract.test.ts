import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const demandKanban = readFileSync(
  new URL('../src/components/DemandKanban.svelte', import.meta.url),
  'utf8'
);

const modalStart = demandKanban.indexOf('{#if showDeconstructorModal}');
const modalEnd = demandKanban.indexOf('<!-- Confirm Modal -->', modalStart);
assert.notEqual(modalStart, -1, 'AI deconstruction modal must exist');
assert.notEqual(modalEnd, -1, 'AI deconstruction modal must have a stable end marker');
const deconstructorMarkup = demandKanban.slice(modalStart, modalEnd);

test('standalone AI deconstruction modal uses the shared workspace overlay owner', () => {
  assert.match(
    deconstructorMarkup,
    /use:portalToWorkspaceStage=\{deconstructorPresentation === 'modal' \|\| \(deconstructorPresentation === 'companion' && deconstructorHost === 'create'\)\}/
  );
  assert.match(
    deconstructorMarkup,
    /class:is-workspace-scoped=\{deconstructorPresentation === 'modal'\}/
  );
});

test('workspace-scoped AI modal follows desktop and narrow workspace geometry', () => {
  assert.match(
    demandKanban,
    /@media \(min-width: 861px\) \{[\s\S]*?\.deconstructor-backdrop\.is-workspace-scoped[\s\S]*?position: absolute;[\s\S]*?inset: 0;[\s\S]*?width: 100%;[\s\S]*?height: 100%;/
  );
  assert.match(
    demandKanban,
    /@media \(max-width: 860px\) \{[\s\S]*?\.deconstructor-backdrop\.is-workspace-scoped[\s\S]*?position: fixed;[\s\S]*?top: var\(--wa-main-content-top, 0px\);/
  );
  assert.match(
    demandKanban,
    /\.deconstructor-modal-body \{[\s\S]*?padding: 18px 22px 20px;/
  );
});
