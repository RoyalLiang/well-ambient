import assert from 'node:assert/strict';
import test from 'node:test';

import {
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
