export type DeconstructStreamEvent<T> = {
  type: 'status' | 'provider_delta' | 'complete' | 'error';
  phase?: string;
  message?: string;
  delta?: string;
  result?: T;
};

export type DeconstructStreamOptions = {
  signal?: AbortSignal;
};

function abortError(signal?: AbortSignal) {
  if (signal?.reason instanceof DOMException && signal.reason.name === 'AbortError') {
    return signal.reason;
  }
  return new DOMException('AI 流式生成已取消', 'AbortError');
}

export async function readDeconstructStream<T>(
  response: Response,
  onEvent?: (event: DeconstructStreamEvent<T>) => void,
  options: DeconstructStreamOptions = {}
): Promise<T> {
  if (!response.body) throw new Error('当前浏览器无法读取 AI 流式响应');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const { signal } = options;
  let buffer = '';
  let completed: T | undefined;
  let streamFinished = false;
  let cancelPromise: Promise<void> | null = null;

  const cancelReader = () => {
    if (!cancelPromise) {
      cancelPromise = reader.cancel(signal?.reason).catch(() => undefined);
    }
    return cancelPromise;
  };

  const handleAbort = () => {
    void cancelReader();
  };

  const consumeLine = (line: string) => {
    if (!line.trim()) return;
    const event = JSON.parse(line) as DeconstructStreamEvent<T>;
    onEvent?.(event);
    if (signal?.aborted) throw abortError(signal);
    if (event.type === 'error') throw new Error(event.message || 'AI 流式生成失败');
    if (event.type === 'complete' && event.result !== undefined) completed = event.result;
  };

  if (signal?.aborted) {
    await cancelReader();
    reader.releaseLock();
    throw abortError(signal);
  }

  signal?.addEventListener('abort', handleAbort, { once: true });
  try {
    while (true) {
      if (signal?.aborted) throw abortError(signal);
      const { value, done } = await reader.read();
      if (signal?.aborted) throw abortError(signal);
      buffer += decoder.decode(value, { stream: !done });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) consumeLine(line);
      if (done) break;
    }
    consumeLine(buffer);
    streamFinished = true;
    if (completed === undefined) throw new Error('AI 流已结束，但没有返回完整结果');
    return completed;
  } catch (error) {
    if (signal?.aborted) throw abortError(signal);
    throw error;
  } finally {
    signal?.removeEventListener('abort', handleAbort);
    if (!streamFinished) await cancelReader();
    reader.releaseLock();
  }
}
