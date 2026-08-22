import assert from 'node:assert/strict';
import test from 'node:test';
import { subscribeDailyJiraUpdates } from '../src/lib/daily-jira-refresh.ts';

function telemetryUpdate(taskID: string) {
  const event = new Event('well-ambient:telemetry-updated');
  Object.defineProperty(event, 'detail', { value: { task_id: taskID } });
  return event;
}

function wait(milliseconds: number) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

test('Jira telemetry events coalesce into one Daily Jira refresh and stop after cleanup', async () => {
  const target = new EventTarget();
  let refreshes = 0;
  const unsubscribe = subscribeDailyJiraUpdates(target, () => {
    refreshes += 1;
  }, 5);

  target.dispatchEvent(telemetryUpdate('WA-901'));
  target.dispatchEvent(telemetryUpdate('WA-902'));
  target.dispatchEvent(telemetryUpdate('WA-903'));
  await wait(20);
  assert.equal(refreshes, 1);

  unsubscribe();
  target.dispatchEvent(telemetryUpdate('WA-904'));
  await wait(20);
  assert.equal(refreshes, 1);
});

test('an update arriving during refresh queues one latest follow-up refresh', async () => {
  const target = new EventTarget();
  let refreshes = 0;
  let releaseFirstRefresh: (() => void) | undefined;
  const firstRefresh = new Promise<void>((resolve) => {
    releaseFirstRefresh = resolve;
  });
  const unsubscribe = subscribeDailyJiraUpdates(target, async () => {
    refreshes += 1;
    if (refreshes === 1) await firstRefresh;
  }, 0);

  target.dispatchEvent(telemetryUpdate('WA-905'));
  await wait(5);
  assert.equal(refreshes, 1);

  target.dispatchEvent(telemetryUpdate('WA-906'));
  await wait(5);
  assert.equal(refreshes, 1);

  releaseFirstRefresh?.();
  await wait(5);
  assert.equal(refreshes, 2);
  unsubscribe();
});
