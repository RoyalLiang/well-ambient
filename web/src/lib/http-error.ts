export async function responseErrorMessage(response: Response, fallback: string): Promise<string> {
  const text = (await response.text()).trim();
  if (!text) return fallback;

  try {
    const payload = JSON.parse(text) as Record<string, unknown>;
    for (const key of ['message', 'error', 'details']) {
      const value = payload[key];
      if (typeof value === 'string' && value.trim()) return value.trim();
    }
  } catch {
    // Plain-text API errors are already suitable for display.
  }
  return text;
}
