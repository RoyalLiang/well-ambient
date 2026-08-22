import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const taskKanban = readFileSync(
  new URL('../src/components/TaskKanban.svelte', import.meta.url),
  'utf8'
);

const finalContractStart = taskKanban.indexOf(
  '/* Task workbenches are viewport-bounded; details remain a concise peer panel. */'
);
assert.notEqual(finalContractStart, -1, 'final task workbench contract marker must exist');
const finalContract = taskKanban.slice(finalContractStart);

test('desktop execution workbench keeps the same bounded final row as task status', () => {
  assert.match(
    finalContract,
    /\.task-console\.view-execution \{[\s\S]*?grid-template-rows: auto minmax\(0, 1fr\);[\s\S]*?\}/,
    'execution view must reserve its final grid row for the workbench'
  );
  assert.doesNotMatch(
    finalContract,
    /\.task-console\.view-execution \{[\s\S]*?overflow-y: auto !important;[\s\S]*?\}/,
    'execution view must not replace the bounded workspace with root scrolling'
  );
  assert.doesNotMatch(
    finalContract,
    /\.phase41-execution-workbench \{[\s\S]*?height: auto;[\s\S]*?align-items: start;[\s\S]*?\}/,
    'execution workbench must not opt out of the shared full-height stretch contract'
  );
});

test('desktop execution table and inspector stretch to the shared bottom edge', () => {
  assert.match(
    finalContract,
    /\.phase41-execution-workbench \.phase41-table-panel,[\s\S]*?\.phase41-execution-workbench \.phase41-inspector,[\s\S]*?\{[\s\S]*?height: 100%;[\s\S]*?min-height: 0;[\s\S]*?max-height: none;[\s\S]*?\}/,
    'both execution cards must inherit the shared full-height peer contract'
  );
  assert.doesNotMatch(
    finalContract,
    /\.phase41-execution-workbench \.phase41-table-panel \{[\s\S]*?height: clamp\(/,
    'the execution table must not end early at an unrelated viewport clamp'
  );
  assert.match(
    finalContract,
    /\.phase41-execution-workbench \.phase41-inspector,[\s\S]*?\.phase41-status-workbench \.phase41-inspector \{[\s\S]*?overflow-y: auto;[\s\S]*?overscroll-behavior: contain;[\s\S]*?\}/,
    'both inspectors must preserve complete content through internal scrolling'
  );
});

test('stacked task layouts keep their existing natural-height contract', () => {
  const stackedContract = taskKanban.match(
    /@media \(max-width: 1180px\) \{[\s\S]*?\.phase41-status-workbench \.phase41-table-panel,[\s\S]*?\.phase41-execution-workbench \.phase41-inspector \{[\s\S]*?height: auto;[\s\S]*?min-height: 0;[\s\S]*?max-height: none;[\s\S]*?\}/
  )?.[0] || '';

  assert.ok(stackedContract, 'stacked task cards must return to natural height');
});
