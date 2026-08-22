import assert from 'node:assert/strict';
import test from 'node:test';
import { reconcileSolutionEditor } from '../src/lib/solution-editor-sync.ts';

test('remote snapshots refresh an untouched Markdown editor', () => {
  const next = reconcileSolutionEditor(
    { markdown: '# old', baselineHash: 'old', dirty: false, remoteUpdateAvailable: false },
    { markdown: '# new', content_hash: 'new' }
  );
  assert.equal(next.markdown, '# new');
  assert.equal(next.baselineHash, 'new');
  assert.equal(next.remoteUpdateAvailable, false);
});

test('Agent or remote completion never overwrites unsaved Markdown', () => {
  const next = reconcileSolutionEditor(
    { markdown: '# local unsaved', baselineHash: 'old', dirty: true, remoteUpdateAvailable: false },
    { markdown: '# remote candidate', content_hash: 'new' }
  );
  assert.equal(next.markdown, '# local unsaved');
  assert.equal(next.baselineHash, 'old');
  assert.equal(next.dirty, true);
  assert.equal(next.remoteUpdateAvailable, true);
});
