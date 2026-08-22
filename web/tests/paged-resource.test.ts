import assert from 'node:assert/strict';
import test from 'node:test';

import { PagedResource } from '../src/lib/paged-resource.ts';

type Item = { id: number; label: string };

function response(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), { status, headers: { 'content-type': 'application/json' } });
}

test('refresh is single-flight and atomically replaces the retained snapshot', async () => {
  let requests = 0;
  let resolveRequest: ((value: Response) => void) | undefined;
  const resource = new PagedResource<Item>({
    endpoint: () => '/api/items', itemKey: item => item.id,
    fetcher: async () => {
      requests++;
      return new Promise<Response>(resolve => { resolveRequest = resolve; });
    }
  });
  const first = resource.refresh();
  const second = resource.refresh();
  assert.equal(requests, 1);
  assert.equal(resource.snapshot().loading, true);
  assert.deepEqual(resource.snapshot().items, []);
  resolveRequest?.(response({ items: [{ id: 1, label: 'current' }], page: { limit: 50, has_more: false, generation: 1 } }));
  assert.deepEqual((await first).items.map(item => item.id), [1]);
  assert.deepEqual((await second).items.map(item => item.id), [1]);
});

test('loadMore appends unique rows and keeps the current snapshot while loading', async () => {
  const calls: string[] = [];
  let resolveMore: ((value: Response) => void) | undefined;
  const resource = new PagedResource<Item>({
    endpoint: () => '/api/items?status=active', itemKey: item => item.id,
    fetcher: async input => {
      const url = String(input);
      calls.push(url);
      if (!url.includes('cursor=')) {
        return response({ items: [{ id: 1, label: 'one' }], page: { limit: 50, has_more: true, next_cursor: 'next', generation: 1 } });
      }
      return new Promise<Response>(resolve => { resolveMore = resolve; });
    }
  });
  await resource.refresh();
  const loadingMore = resource.loadMore();
  assert.equal(resource.snapshot().loadingMore, true);
  assert.deepEqual(resource.snapshot().items.map(item => item.id), [1]);
  resolveMore?.(response({ items: [{ id: 1, label: 'newer' }, { id: 2, label: 'two' }], page: { limit: 50, has_more: false, generation: 1 } }));
  assert.deepEqual((await loadingMore).items.map(item => item.id), [1, 2]);
  assert.equal(calls.length, 2);
});

test('a stale continuation cursor restarts from a new complete first page', async () => {
  let firstPages = 0;
  const resource = new PagedResource<Item>({
    endpoint: () => '/api/items', itemKey: item => item.id,
    fetcher: async input => {
      const url = String(input);
      if (url.includes('cursor=')) return response({ error: 'stale_cursor' }, 409);
      firstPages++;
      return response({
        items: [{ id: firstPages, label: `generation-${firstPages}` }],
        page: { limit: 50, has_more: firstPages === 1, next_cursor: firstPages === 1 ? 'old' : '', generation: firstPages }
      });
    }
  });
  await resource.refresh();
  const state = await resource.loadMore();
  assert.equal(firstPages, 2);
  assert.deepEqual(state.items.map(item => item.id), [2]);
  assert.equal(state.error, '');
});

test('changing scope aborts the old refresh and only publishes the new scope', async () => {
  let scope = 'first';
  const resource = new PagedResource<Item>({
    endpoint: () => `/api/items?scope=${scope}`,
    itemKey: item => item.id,
    fetcher: async (input, init) => {
      const url = String(input);
      if (url.includes('scope=second')) {
        return response({ items: [{ id: 2, label: 'second' }], page: { limit: 50, has_more: false, generation: 2 } });
      }
      return new Promise<Response>((resolve, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
        setTimeout(() => resolve(response({ items: [{ id: 1, label: 'first' }], page: { limit: 50, has_more: false, generation: 1 } })), 20);
      });
    }
  });
  const first = resource.refresh();
  scope = 'second';
  const second = resource.refresh();
  await Promise.all([first, second]);
  assert.deepEqual(resource.snapshot().items.map(item => item.id), [2]);
  assert.equal(resource.snapshot().loading, false);
});
