export interface AgendaSummaryPayload {
  agenda_items?: Array<Record<string, unknown>>;
  auto_decisions?: Array<Record<string, unknown>>;
  [key: string]: unknown;
}

export interface AgendaSummaryState {
  data: AgendaSummaryPayload | null;
  loading: boolean;
  error: string;
  updatedAt: number;
}

export interface AgendaSummaryRefreshOptions {
  fetcher?: typeof fetch;
  force?: boolean;
  maxAgeMs?: number;
}

type AgendaSummarySubscriber = (state: AgendaSummaryState) => void;

const subscribers = new Set<AgendaSummarySubscriber>();
let state: AgendaSummaryState = {
  data: null,
  loading: false,
  error: '',
  updatedAt: 0
};
let inFlight: Promise<AgendaSummaryPayload> | null = null;

function publish(next: AgendaSummaryState) {
  state = next;
  for (const subscriber of subscribers) subscriber(state);
}

export function subscribeAgendaSummary(subscriber: AgendaSummarySubscriber): () => void {
  subscribers.add(subscriber);
  subscriber(state);
  return () => subscribers.delete(subscriber);
}

export async function refreshAgendaSummary(
  options: AgendaSummaryRefreshOptions = {}
): Promise<AgendaSummaryPayload> {
  if (inFlight) return inFlight;

  const maxAgeMs = Math.max(0, options.maxAgeMs ?? 5000);
  if (!options.force && state.data && Date.now() - state.updatedAt < maxAgeMs) {
    return state.data;
  }

  const fetcher = options.fetcher ?? fetch;
  publish({ ...state, loading: true, error: '' });
  const request = (async () => {
    try {
      const response = await fetcher('/api/agenda/summary', { cache: 'no-store' });
      if (!response.ok) throw new Error('加载决策看板数据失败');
      const payload = await response.json() as AgendaSummaryPayload;
      publish({ data: payload, loading: false, error: '', updatedAt: Date.now() });
      return payload;
    } catch (error) {
      const message = error instanceof Error ? error.message : '网络连接异常';
      publish({ ...state, loading: false, error: message });
      throw error;
    } finally {
      inFlight = null;
    }
  })();
  inFlight = request;
  return request;
}

export function resetAgendaSummaryResourceForTests() {
  inFlight = null;
  publish({ data: null, loading: false, error: '', updatedAt: 0 });
}
