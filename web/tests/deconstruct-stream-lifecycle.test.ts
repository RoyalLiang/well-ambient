import assert from 'node:assert/strict';
import test from 'node:test';
import { readDeconstructStream } from '../src/lib/deconstruct-stream.ts';

type Result = { tasks: Array<{ id: string }> };

test('deconstruct stream exposes provider deltas before the complete result arrives', async () => {
  const encoder = new TextEncoder();
  let finish!: () => void;
  const mayFinish = new Promise<void>((resolve) => {
    finish = resolve;
  });
  let deltaSeen!: () => void;
  const sawDelta = new Promise<void>((resolve) => {
    deltaSeen = resolve;
  });

  const response = new Response(new ReadableStream<Uint8Array>({
    async start(controller) {
      controller.enqueue(encoder.encode('{"type":"provider_delta","delta":"第一段"}\n'));
      await mayFinish;
      controller.enqueue(encoder.encode('{"type":"complete","result":{"tasks":[{"id":"T-1"}]}}\n'));
      controller.close();
    }
  }));

  const resultPromise = readDeconstructStream<Result>(response, (event) => {
    if (event.type === 'provider_delta') deltaSeen();
  });

  await sawDelta;
  let completed = false;
  void resultPromise.then(() => {
    completed = true;
  });
  await Promise.resolve();
  assert.equal(completed, false, 'delta must be observable before the complete event');

  finish();
  assert.deepEqual(await resultPromise, { tasks: [{ id: 'T-1' }] });
});

test('aborting a deconstruct stream cancels and releases the response reader', async () => {
  const encoder = new TextEncoder();
  const controller = new AbortController();
  let cancelCount = 0;
  let completionTimer: ReturnType<typeof setTimeout> | undefined;

  const response = new Response(new ReadableStream<Uint8Array>({
    start(streamController) {
      streamController.enqueue(encoder.encode('{"type":"provider_delta","delta":"仍在生成"}\n'));
      completionTimer = setTimeout(() => {
        streamController.enqueue(encoder.encode('{"type":"complete","result":{"tasks":[]}}\n'));
        streamController.close();
      }, 40);
    },
    cancel() {
      cancelCount += 1;
      if (completionTimer) clearTimeout(completionTimer);
    }
  }));

  const resultPromise = readDeconstructStream<Result>(
    response,
    (event) => {
      if (event.type === 'provider_delta') controller.abort();
    },
    { signal: controller.signal }
  );

  await assert.rejects(resultPromise, (error: unknown) => {
    return error instanceof DOMException && error.name === 'AbortError';
  });
  assert.equal(cancelCount, 1, 'abort must cancel the active response stream exactly once');
  assert.equal(response.body?.locked, false, 'reader lock must be released after abort');
});
