import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const markdownWorkbench = readFileSync(
  new URL('../src/components/shared/MarkdownWorkbench.svelte', import.meta.url),
  'utf8',
);
const modal = readFileSync(
  new URL('../src/components/shared/Modal.svelte', import.meta.url),
  'utf8',
);

test('document label and description are visually hidden by default without losing accessible names', () => {
  assert.match(markdownWorkbench, /export let showDocumentMeta = false/);
  assert.match(
    markdownWorkbench,
    /showWorkbenchToolbar = showToolbar && \(showDocumentMeta \|\| toolbarModes\.length > 1\)/,
  );
  assert.match(markdownWorkbench, /\{:else if showWorkbenchToolbar\}/);
  assert.match(
    markdownWorkbench,
    /\{#if showDocumentMeta\}[\s\S]*?<div class="document-identity">[\s\S]*?\{label\}[\s\S]*?\{description\}[\s\S]*?\{\/if\}/,
  );
  assert.match(markdownWorkbench, /aria-label=\{label\}/);
  assert.match(markdownWorkbench, /'aria-label': `\$\{label\}编辑器`/);
  assert.match(markdownWorkbench, /aria-label=\{`\$\{label\}预览`\}/);
});

test('embedded workbenches can remove the secondary card surface without changing their content model', () => {
  assert.match(markdownWorkbench, /export let embedded = false/);
  assert.match(markdownWorkbench, /class:embedded/);
  assert.match(
    markdownWorkbench,
    /\.markdown-workbench\.embedded\s*\{[^}]*border(?:-width)?:\s*0[^}]*border-radius:\s*0[^}]*background:\s*transparent/s,
  );
});

test('modals can hide a redundant body scrollbar without disabling body scrolling', () => {
  assert.match(modal, /export let hideBodyScrollbar = false/);
  assert.match(modal, /class:hide-body-scrollbar=\{hideBodyScrollbar\}/);
  assert.match(modal, /\.modal-body\.hide-body-scrollbar\s*\{[^}]*scrollbar-width:\s*none/s);
  assert.match(modal, /\.modal-body\.hide-body-scrollbar::-webkit-scrollbar\s*\{[^}]*display:\s*none/s);
  assert.match(modal, /\.modal-body\s*\{[^}]*overflow-y:\s*auto/s);
});
