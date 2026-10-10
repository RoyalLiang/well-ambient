import assert from 'node:assert/strict';
import test from 'node:test';

import {
  invalidateAgendaSummary,
  refreshAgendaSummary,
  resetAgendaSummaryResourceForTests,
  subscribeAgendaSummary
} from '../src/lib/agenda-summary.ts';

test('agenda summary resource collapses concurrent consumers into one request', async () => {
  resetAgendaSummaryResourceForTests();
  let requests = 0;
  let releaseResponse: (() => void) | undefined;
  const responseGate = new Promise<void>((resolve) => {
    releaseResponse = resolve;
  });
  const fetcher = async () => {
    requests += 1;
    await responseGate;
    return {
      ok: true,
      json: async () => ({ agenda_items: [{ task_id: 'WA-101' }], auto_decisions: [] })
    } as Response;
  };

  const first = refreshAgendaSummary({ fetcher: fetcher as typeof fetch, force: true });
  const second = refreshAgendaSummary({ fetcher: fetcher as typeof fetch, force: true });
  assert.equal(requests, 1);
  releaseResponse?.();
  const [firstPayload, secondPayload] = await Promise.all([first, second]);

  assert.equal(requests, 1);
  assert.equal(firstPayload, secondPayload);
  assert.equal(firstPayload.agenda_items?.[0]?.task_id, 'WA-101');
});

test('Jira change during an agenda request rereads facts instead of publishing the pre-change snapshot', async () => {
  resetAgendaSummaryResourceForTests();
  let releaseFirst: () => void = () => {};
  const firstResponse = new Promise<void>(resolve => { releaseFirst = resolve; });
  let requests = 0;
  const fetcher = (async () => {
    const request = ++requests;
    if (request === 1) await firstResponse;
    return { ok: true, json: async () => ({ agenda_items: request === 1 ? [{ task_id: 'NS2-2569', assignee: '梁志远' }] : [] }) } as Response;
  }) as typeof fetch;
  const published: number[] = [];
  const stop = subscribeAgendaSummary(state => {
    if (state.data) published.push(state.data.agenda_items?.length || 0);
  });
  try {
    const initial = refreshAgendaSummary({ fetcher, force: true });
    invalidateAgendaSummary();
    const reassigned = refreshAgendaSummary({ fetcher, force: true });
    releaseFirst();
    await Promise.all([initial, reassigned]);
    assert.equal(requests, 2, 'the change event must cause a read started after the reassignment');
    assert.deepEqual(published, [0], 'do not publish the stale item after receiving its change event');
  } finally {
    releaseFirst();
    stop();
  }
});

test('agenda summary resource publishes one shared state to all consumers', async () => {
  resetAgendaSummaryResourceForTests();
  const seen: string[] = [];
  const unsubscribe = subscribeAgendaSummary((state) => {
    seen.push(`${state.loading}:${state.data?.agenda_items?.length || 0}:${state.error}`);
  });

  await refreshAgendaSummary({
    force: true,
    fetcher: (async () => ({
      ok: true,
      json: async () => ({ agenda_items: [{ task_id: 'WA-102' }], auto_decisions: [] })
    } as Response)) as typeof fetch
  });
  unsubscribe();

  assert.deepEqual(seen, ['false:0:', 'true:0:', 'false:1:']);
});

test('SSE fact invalidation bypasses a fresh cached agenda without periodic refresh', async () => {
  resetAgendaSummaryResourceForTests();
  let requests = 0;
  const fetcher = (async () => ({ ok: true, json: async () => ({ agenda_items: ++requests === 1 ? [{ task_id: 'NS2-2569' }] : [] }) })) as typeof fetch;
  await refreshAgendaSummary({ fetcher });
  await refreshAgendaSummary({ fetcher });
  assert.equal(requests, 1);
  invalidateAgendaSummary();
  const latest = await refreshAgendaSummary({ fetcher });
  assert.equal(requests, 2);
  assert.deepEqual(latest.agenda_items, []);
});
