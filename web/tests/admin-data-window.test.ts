import assert from 'node:assert/strict';
import test from 'node:test';

import {
  flattenBoundedPages,
  updateBoundedPageWindow
} from '../src/lib/admin-data-window.ts';

interface Row {
  id: string;
}

interface Meta {
  cursor: string;
}

function page(start: number) {
  return {
    items: Array.from({ length: 100 }, (_, index): Row => ({ id: `row-${start + index}` })),
    meta: { cursor: `cursor-${start}` } satisfies Meta
  };
}

test('bounded page window evicts the opposite edge and never retains more than three pages', () => {
  let pages = [page(0), page(100), page(200)];
  pages = updateBoundedPageWindow(pages, page(300), 'next', 3, (row) => row.id);

  assert.equal(pages.length, 3);
  assert.equal(flattenBoundedPages(pages)[0]?.id, 'row-100');
  assert.equal(flattenBoundedPages(pages).at(-1)?.id, 'row-399');
  assert.equal(flattenBoundedPages(pages).length, 300);

  pages = updateBoundedPageWindow(pages, page(0), 'previous', 3, (row) => row.id);
  assert.equal(pages.length, 3);
  assert.equal(flattenBoundedPages(pages)[0]?.id, 'row-0');
  assert.equal(flattenBoundedPages(pages).at(-1)?.id, 'row-299');
  assert.equal(flattenBoundedPages(pages).length, 300);
});

test('bounded page window removes overlapping row ids at page seams', () => {
  const first = page(0);
  const overlapping = {
    items: [{ id: 'row-99' }, { id: 'row-100' }],
    meta: { cursor: 'overlap' }
  };
  const pages = updateBoundedPageWindow([first], overlapping, 'next', 3, (row) => row.id);

  assert.deepEqual(flattenBoundedPages(pages).slice(-3).map((row) => row.id), [
    'row-98',
    'row-99',
    'row-100'
  ]);
});
