import assert from 'node:assert/strict';
import test from 'node:test';

import {
  clampPage,
  getPageCount,
  getPaginationItems,
  getPaginationSummary
} from '../src/lib/admin-pagination.ts';

test('pagination normalizes empty and out-of-range input', () => {
  assert.equal(getPageCount(0, 10), 1);
  assert.equal(getPageCount(101, 10), 11);
  assert.equal(clampPage(0, 5), 1);
  assert.equal(clampPage(8, 5), 5);
  assert.deepEqual(getPaginationSummary(0, 9, 0), {
    page: 1,
    pageCount: 1,
    start: 0,
    end: 0,
    total: 0
  });
});

test('pagination summary reports the current visible range', () => {
  assert.deepEqual(getPaginationSummary(47, 3, 10), {
    page: 3,
    pageCount: 5,
    start: 21,
    end: 30,
    total: 47
  });
  assert.deepEqual(getPaginationSummary(47, 5, 10), {
    page: 5,
    pageCount: 5,
    start: 41,
    end: 47,
    total: 47
  });
});

test('pagination items keep first, last and a centered current window', () => {
  assert.deepEqual(getPaginationItems(1, 4), [1, 2, 3, 4]);
  assert.deepEqual(getPaginationItems(1, 12), [1, 2, 3, 4, 'ellipsis-end', 12]);
  assert.deepEqual(getPaginationItems(6, 12), [1, 'ellipsis-start', 5, 6, 7, 'ellipsis-end', 12]);
  assert.deepEqual(getPaginationItems(12, 12), [1, 'ellipsis-start', 9, 10, 11, 12]);
});
