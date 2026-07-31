export type DeconstructStreamEvent<T> = {
  type: 'status' | 'provider_delta' | 'complete' | 'error';
  phase?: string;
  message?: string;
  delta?: string;
  result?: T;
};

export async function readDeconstructStream<T>(
  response: Response,
  onEvent?: (event: DeconstructStreamEvent<T>) => void
): Promise<T> {
  if (!response.body) throw new Error('当前浏览器无法读取 AI 流式响应');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let completed: T | undefined;

  const consumeLine = (line: string) => {
    if (!line.trim()) return;
    const event = JSON.parse(line) as DeconstructStreamEvent<T>;
    onEvent?.(event);
    if (event.type === 'error') throw new Error(event.message || 'AI 流式生成失败');
    if (event.type === 'complete' && event.result !== undefined) completed = event.result;
  };

  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';
    for (const line of lines) consumeLine(line);
    if (done) break;
  }
  consumeLine(buffer);
  if (completed === undefined) throw new Error('AI 流已结束，但没有返回完整结果');
  return completed;
}
