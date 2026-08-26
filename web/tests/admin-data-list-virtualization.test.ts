import assert from 'node:assert/strict';
import test from 'node:test';

import {
  computeVirtualWindow,
  dedupeRowsByID,
  shouldRequestMore,
  shouldRequestPrevious
} from '../src/lib/admin-data-list.ts';

test('virtual window stays bounded and preserves exact spacer geometry', () => {
  assert.deepEqual(
    computeVirtualWindow({
      totalRows: 1000,
      scrollTop: 52 * 420,
      viewportHeight: 520,
      rowHeight: 52,
      overscan: 8
    }),
    {
      start: 412,
      end: 438,
      topSpacerHeight: 21424,
      bottomSpacerHeight: 29224
    }
  );

  assert.deepEqual(
    computeVirtualWindow({
      totalRows: 4,
      scrollTop: 999,
      viewportHeight: 520,
      rowHeight: 52,
      overscan: 8
    }),
    {
      start: 0,
      end: 4,
      topSpacerHeight: 0,
      bottomSpacerHeight: 0
    }
  );
});

test('near-end loading is suppressed for every non-loadable state', () => {
  const loadable = {
    scrollTop: 400,
    scrollHeight: 1000,
    viewportHeight: 500,
    thresholdPx: 104,
    hasMore: true,
    loading: false,
    hasError: false,
    disabled: false
  };

  assert.equal(shouldRequestMore(loadable), true);
  assert.equal(shouldRequestMore({ ...loadable, scrollTop: 200 }), false);
  assert.equal(shouldRequestMore({ ...loadable, hasMore: false }), false);
  assert.equal(shouldRequestMore({ ...loadable, loading: true }), false);
  assert.equal(shouldRequestMore({ ...loadable, hasError: true }), false);
  assert.equal(shouldRequestMore({ ...loadable, disabled: true }), false);
});

test('near-start loading mirrors the same loadability guards', () => {
  const loadable = {
    scrollTop: 80,
    scrollHeight: 1000,
    viewportHeight: 500,
    thresholdPx: 104,
    hasMore: true,
    loading: false,
    hasError: false,
    disabled: false
  };

  assert.equal(shouldRequestPrevious(loadable), true);
  assert.equal(shouldRequestPrevious({ ...loadable, scrollTop: 180 }), false);
  assert.equal(shouldRequestPrevious({ ...loadable, hasMore: false }), false);
  assert.equal(shouldRequestPrevious({ ...loadable, loading: true }), false);
  assert.equal(shouldRequestPrevious({ ...loadable, hasError: true }), false);
  assert.equal(shouldRequestPrevious({ ...loadable, disabled: true }), false);
});

test('duplicate row ids are removed without changing first-seen order', () => {
  const rows = [
    { id: 'WA-1', value: 'first' },
    { id: 'WA-2', value: 'second' },
    { id: 'WA-1', value: 'duplicate' }
  ];

  assert.deepEqual(dedupeRowsByID(rows), rows.slice(0, 2));
});
