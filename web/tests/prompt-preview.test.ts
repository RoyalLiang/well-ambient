import assert from 'node:assert/strict';
import test from 'node:test';
import { PromptPreviewError, requestPromptPreview } from '../src/lib/promptPreview.ts';

const payload = { system_prompt: 'rules', markdown: 'example' };
const pendingFetch: typeof fetch = (_url, init) => new Promise((_resolve, reject) => {
  init?.signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), { once: true });
});

test('an unresponsive preview becomes a recoverable timeout', async () => {
  await assert.rejects(requestPromptPreview(payload, new AbortController().signal,
    { fetcher: pendingFetch, timeoutMs: 10 }), (error: unknown) =>
    error instanceof PromptPreviewError && error.kind === 'timeout');
});

test('cancel aborts the pending preview without being reported as a timeout', async () => {
  const controller = new AbortController();
  const preview = requestPromptPreview(payload, controller.signal, { fetcher: pendingFetch, timeoutMs: 1000 });
  controller.abort();
  await assert.rejects(preview, (error: unknown) => error instanceof PromptPreviewError && error.kind === 'cancelled');
});

test('a cancelled preview never starts a provider request', async () => {
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(requestPromptPreview(payload, controller.signal, {
    fetcher: async () => { assert.fail('cancelled preview started a request'); }
  }), (error: unknown) => error instanceof PromptPreviewError && error.kind === 'cancelled');
});

test('successful preview sends the original input and returns its result', async () => {
  const markdown = await requestPromptPreview(payload, new AbortController().signal, {
    fetcher: async (url, init) => {
      assert.equal(url, '/api/solution-prompts/test');
      assert.deepEqual(JSON.parse(init?.body as string), payload);
      return new Response(JSON.stringify({ markdown: 'result' }), { status: 200 });
    }
  });
  assert.equal(markdown, 'result');
});

test('server deadline errors keep their useful message', async () => {
  await assert.rejects(requestPromptPreview(payload, new AbortController().signal, {
    fetcher: async () => new Response(JSON.stringify({ message: '模型测试已超时，请稍后重试。' }), { status: 504 })
  }), (error: unknown) => error instanceof PromptPreviewError && error.kind === 'timeout' && error.message.includes('超时'));
});
