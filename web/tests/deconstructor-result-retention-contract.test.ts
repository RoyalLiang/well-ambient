import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import {
  buildDeconstructorSessionKey,
  rememberDeconstructorSnapshot,
  restoreDeconstructorSnapshot,
  type DeconstructorCompletedSnapshot
} from '../src/lib/deconstructor-session.ts';

const sessionPath = new URL('../src/lib/deconstructor-session.ts', import.meta.url);
const deconstructor = readFileSync(
  new URL('../src/components/Deconstructor.svelte', import.meta.url),
  'utf8'
);
const demandKanban = readFileSync(
  new URL('../src/components/DemandKanban.svelte', import.meta.url),
  'utf8'
);

test('completed AI deconstruction has an explicit serializable session contract', () => {
  assert.equal(existsSync(sessionPath), true, 'a completed-result session contract must exist');
  const session = existsSync(sessionPath) ? readFileSync(sessionPath, 'utf8') : '';
  assert.match(session, /export type DeconstructorCompletedSnapshot/);
  assert.match(session, /export function buildDeconstructorSessionKey/);
  assert.match(session, /export function rememberDeconstructorSnapshot/);
  assert.match(session, /export function restoreDeconstructorSnapshot/);
});

test('deconstructor hydrates and publishes only completed, idle snapshots', () => {
  assert.match(deconstructor, /export let initialCompletedSnapshot/);
  assert.match(deconstructor, /export let onCompletedSnapshotChange/);
  assert.match(deconstructor, /hasResult && !isLoading[\s\S]*?onCompletedSnapshotChange/);
  assert.match(deconstructor, /initialCompletedSnapshot\?\.result/);

  const generationSetup = deconstructor.slice(
    deconstructor.indexOf('async function handleDeconstruct()'),
    deconstructor.indexOf('const structuredDemandText')
  );
  assert.doesNotMatch(
    generationSetup,
    /currentTaskGroupId = ''|isMockResponse = false/,
    'starting a replacement run must not corrupt the last completed snapshot'
  );
});

test('schedule host retains snapshots per context and restores without cross-demand leakage', () => {
  assert.match(demandKanban, /let deconstructorCompletedSnapshots = new Map/);
  assert.match(demandKanban, /buildDeconstructorSessionKey\(/);
  assert.match(demandKanban, /restoreDeconstructorSnapshot\(/);
  assert.match(demandKanban, /rememberDeconstructorSnapshot\(/);
  assert.match(
    demandKanban,
    /function handleDeconstructorCompletedSnapshotChange[\s\S]*?deconstructorInitialSnapshot = restoreDeconstructorSnapshot/
  );
  assert.match(demandKanban, /initialCompletedSnapshot=\{deconstructorInitialSnapshot\}/);
  assert.match(demandKanban, /onCompletedSnapshotChange=\{handleDeconstructorCompletedSnapshotChange\}/);
});

test('session cache clones completed results, isolates contexts, and evicts oldest entries', () => {
  type Result = { tasks: Array<{ id: string; title: string }> };
  const firstKey = buildDeconstructorSessionKey({ demandId: 'DG-394', title: 'ignored' });
  const sameKey = buildDeconstructorSessionKey({ demandId: ' dg-394 ' });
  const secondKey = buildDeconstructorSessionKey({ demandId: 'DG-395' });
  assert.equal(firstKey, sameKey);
  assert.notEqual(firstKey, secondKey);

  const first: DeconstructorCompletedSnapshot<Result> = {
    result: { tasks: [{ id: 'T-1', title: '第一版' }] },
    taskGroupId: 'group-dg-394',
    activeTaskId: 'T-1',
    isMockResponse: false,
    linkedDemandId: 'DG-394',
    linkedDemandTitle: '#DG-394 - 同步状态',
    demandTitle: '同步状态',
    inputText: '完成后关闭再打开'
  };

  let cache = rememberDeconstructorSnapshot(new Map(), firstKey, first, 2);
  first.result.tasks[0].title = '外部修改';
  const restored = restoreDeconstructorSnapshot(cache, firstKey);
  assert.equal(restored?.result.tasks[0].title, '第一版', 'cache must own a clone');

  if (restored) restored.result.tasks[0].title = '恢复副本修改';
  assert.equal(
    restoreDeconstructorSnapshot(cache, firstKey)?.result.tasks[0].title,
    '第一版',
    'each reopen must receive an isolated clone'
  );

  const second = { ...first, result: { tasks: [{ id: 'T-2', title: '第二项需求' }] } };
  cache = rememberDeconstructorSnapshot(cache, secondKey, second, 2);
  assert.equal(restoreDeconstructorSnapshot(cache, firstKey)?.result.tasks[0].id, 'T-1');
  assert.equal(restoreDeconstructorSnapshot(cache, secondKey)?.result.tasks[0].id, 'T-2');

  const thirdKey = buildDeconstructorSessionKey({ demandId: 'DG-396' });
  cache = rememberDeconstructorSnapshot(cache, thirdKey, second, 2);
  assert.equal(restoreDeconstructorSnapshot(cache, firstKey), null, 'oldest snapshot must be evicted');
});
