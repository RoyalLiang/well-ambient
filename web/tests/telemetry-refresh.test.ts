import assert from 'node:assert/strict';
import test from 'node:test';
import { subscribeTelemetryUpdates } from '../src/lib/telemetry-refresh.ts';

function telemetryUpdate(taskID: string) {
  const event = new Event('well-ambient:telemetry-updated');
  Object.defineProperty(event, 'detail', { value: { task_id: taskID } });
  return event;
}

function wait(milliseconds: number) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

test('telemetry events coalesce by task and stop after cleanup', async () => {
  const target = new EventTarget();
  const batches: string[][] = [];
  const unsubscribe = subscribeTelemetryUpdates(target, ({ taskIDs }) => {
    batches.push([...taskIDs]);
  }, { delayMs: 5 });

  target.dispatchEvent(telemetryUpdate('DL-4309'));
  target.dispatchEvent(telemetryUpdate('DL-4309'));
  target.dispatchEvent(telemetryUpdate('WA-902'));
  await wait(20);
  assert.deepEqual(batches, [['DL-4309', 'WA-902']]);

  unsubscribe();
  target.dispatchEvent(telemetryUpdate('WA-903'));
  await wait(20);
  assert.equal(batches.length, 1);
});

test('an event arriving during refresh queues one latest follow-up batch', async () => {
  const target = new EventTarget();
  const batches: string[][] = [];
  let releaseFirstRefresh: (() => void) | undefined;
  const firstRefresh = new Promise<void>((resolve) => {
    releaseFirstRefresh = resolve;
  });
  const unsubscribe = subscribeTelemetryUpdates(target, async ({ taskIDs }) => {
    batches.push([...taskIDs]);
    if (batches.length === 1) await firstRefresh;
  }, { delayMs: 0 });

  target.dispatchEvent(telemetryUpdate('DL-4309'));
  await wait(5);
  target.dispatchEvent(telemetryUpdate('DL-4310'));
  target.dispatchEvent(telemetryUpdate('DL-4311'));
  await wait(5);
  assert.deepEqual(batches, [['DL-4309']]);

  releaseFirstRefresh?.();
  await wait(5);
  assert.deepEqual(batches, [['DL-4309'], ['DL-4310', 'DL-4311']]);
  unsubscribe();
});

test('task predicate ignores unrelated telemetry updates', async () => {
  const target = new EventTarget();
  const batches: string[][] = [];
  const unsubscribe = subscribeTelemetryUpdates(target, ({ taskIDs }) => {
    batches.push([...taskIDs]);
  }, {
    delayMs: 0,
    shouldRefresh: (taskID) => taskID === 'DL-4309'
  });

  target.dispatchEvent(telemetryUpdate('WA-100'));
  target.dispatchEvent(telemetryUpdate('DL-4309'));
  await wait(5);
  assert.deepEqual(batches, [['DL-4309']]);
  unsubscribe();
});

test('hidden documents defer refresh and flush once visible', async () => {
  const target = new EventTarget();
  const visibilityTarget = new EventTarget() as EventTarget & { visibilityState: string };
  visibilityTarget.visibilityState = 'hidden';
  const batches: string[][] = [];
  const unsubscribe = subscribeTelemetryUpdates(target, ({ taskIDs }) => {
    batches.push([...taskIDs]);
  }, { delayMs: 0, visibilityTarget });

  target.dispatchEvent(telemetryUpdate('DL-4309'));
  target.dispatchEvent(telemetryUpdate('DL-4310'));
  await wait(5);
  assert.deepEqual(batches, []);

  visibilityTarget.visibilityState = 'visible';
  visibilityTarget.dispatchEvent(new Event('visibilitychange'));
  await wait(5);
  assert.deepEqual(batches, [['DL-4309', 'DL-4310']]);
  unsubscribe();
});

test('a failed refresh reports the error and a later event can retry', async () => {
  const target = new EventTarget();
  let attempts = 0;
  const errors: unknown[] = [];
  const unsubscribe = subscribeTelemetryUpdates(target, async () => {
    attempts += 1;
    if (attempts === 1) throw new Error('temporary failure');
  }, { delayMs: 0, onError: (error) => errors.push(error) });

  target.dispatchEvent(telemetryUpdate('DL-4309'));
  await wait(5);
  assert.equal(attempts, 1);
  assert.equal(errors.length, 1);

  target.dispatchEvent(telemetryUpdate('DL-4310'));
  await wait(5);
  assert.equal(attempts, 2);
  unsubscribe();
});
