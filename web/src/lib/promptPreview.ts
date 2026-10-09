export class PromptPreviewError extends Error {
  readonly kind: 'cancelled' | 'timeout' | 'request';

  constructor(kind: 'cancelled' | 'timeout' | 'request', message: string) {
    super(message);
    this.kind = kind;
    this.name = 'PromptPreviewError';
  }
}

export async function requestPromptPreview(
  payload: { system_prompt: string; markdown: string },
  signal: AbortSignal,
  options: { timeoutMs?: number; fetcher?: typeof fetch } = {}
): Promise<string> {
  const controller = new AbortController();
  let timedOut = false;
  const cancel = () => controller.abort();
  signal.addEventListener('abort', cancel, { once: true });
  if (signal.aborted) cancel();
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, options.timeoutMs ?? 185_000);
  try {
    if (controller.signal.aborted) throw new PromptPreviewError('cancelled', '已取消模型测试。');
    const response = await (options.fetcher ?? fetch)('/api/solution-prompts/test', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload), signal: controller.signal
    });
    const text = await response.text();
    let body: { markdown?: string; message?: string; error?: string } = {};
    try { body = text ? JSON.parse(text) : {}; } catch { body = { message: text }; }
    if (!response.ok) throw new PromptPreviewError(
      response.status === 504 ? 'timeout' : 'request',
      body.message || body.error || text || `HTTP ${response.status}`
    );
    return body.markdown || '';
  } catch (error) {
    if (controller.signal.aborted) throw new PromptPreviewError(
      timedOut ? 'timeout' : 'cancelled',
      timedOut ? '模型测试已超时，请稍后重试。' : '已取消模型测试。'
    );
    throw error;
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', cancel);
  }
}
