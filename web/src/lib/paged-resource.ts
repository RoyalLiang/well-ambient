export interface ReadPageMeta {
  limit: number;
  has_more: boolean;
  next_cursor?: string;
  generation?: number;
}

export interface PagedResourceState<T> {
  items: T[];
  page: ReadPageMeta | null;
  loading: boolean;
  loadingMore: boolean;
  error: string;
  updatedAt: number;
}

export interface PagedResourceOptions<T> {
  endpoint: () => string;
  selectItems?: (payload: Record<string, unknown>) => T[];
  itemKey: (item: T) => string | number;
  pageSize?: number;
  fetcher?: typeof fetch;
  requestInit?: () => RequestInit;
  errorMessage?: string;
}

type Subscriber<T> = (state: PagedResourceState<T>) => void;

export class PagedResource<T> {
  private readonly options: PagedResourceOptions<T>;
  private readonly subscribers = new Set<Subscriber<T>>();
  private state: PagedResourceState<T> = {
    items: [], page: null, loading: false, loadingMore: false, error: '', updatedAt: 0
  };
  private controller: AbortController | null = null;
  private refreshRequest: Promise<PagedResourceState<T>> | null = null;
  private refreshEndpoint = '';
  private moreRequest: Promise<PagedResourceState<T>> | null = null;
  private revision = 0;

  constructor(options: PagedResourceOptions<T>) {
    this.options = options;
  }

  subscribe(subscriber: Subscriber<T>): () => void {
    this.subscribers.add(subscriber);
    subscriber(this.snapshot());
    return () => this.subscribers.delete(subscriber);
  }

  snapshot(): PagedResourceState<T> {
    return { ...this.state, items: [...this.state.items], page: this.state.page ? { ...this.state.page } : null };
  }

  refresh(): Promise<PagedResourceState<T>> {
    const endpoint = this.options.endpoint();
    if (this.refreshRequest && this.refreshEndpoint === endpoint) return this.refreshRequest;
    const revision = ++this.revision;
    this.controller?.abort();
    this.controller = new AbortController();
    this.refreshEndpoint = endpoint;
    this.publish({ ...this.state, loading: true, loadingMore: false, error: '' });
    const request = this.requestPage('', this.controller.signal, endpoint)
      .then(({ items, page }) => {
        if (revision === this.revision) {
          this.publish({ items, page, loading: false, loadingMore: false, error: '', updatedAt: Date.now() });
        }
        return this.snapshot();
      })
      .catch((error: unknown) => {
        if (revision === this.revision && !isAbortError(error)) {
          this.publish({ ...this.state, loading: false, loadingMore: false, error: errorMessage(error, this.options.errorMessage) });
        }
        return this.snapshot();
      })
      .finally(() => {
        if (revision === this.revision) {
          this.controller = null;
          this.refreshRequest = null;
          this.refreshEndpoint = '';
        }
      });
    this.refreshRequest = request;
    return request;
  }

  loadMore(): Promise<PagedResourceState<T>> {
    if (this.moreRequest) return this.moreRequest;
    const cursor = this.state.page?.next_cursor || '';
    if (!this.state.page?.has_more || !cursor) return Promise.resolve(this.snapshot());
    const revision = this.revision;
    this.publish({ ...this.state, loadingMore: true, error: '' });
    const request = this.requestPage(cursor)
      .then(({ items, page }) => {
        if (revision === this.revision) {
          const byKey = new Map<string | number, T>();
          for (const item of this.state.items) byKey.set(this.options.itemKey(item), item);
          for (const item of items) byKey.set(this.options.itemKey(item), item);
          this.publish({
            items: [...byKey.values()], page, loading: false, loadingMore: false, error: '', updatedAt: Date.now()
          });
        }
        return this.snapshot();
      })
      .catch(async (error: unknown) => {
        if (revision !== this.revision) return this.snapshot();
        if (error instanceof PagedRequestError && error.status === 409) {
          this.moreRequest = null;
          return this.refresh();
        }
        if (!isAbortError(error)) {
          this.publish({ ...this.state, loadingMore: false, error: errorMessage(error, this.options.errorMessage) });
        }
        return this.snapshot();
      })
      .finally(() => {
        if (revision === this.revision) this.moreRequest = null;
      });
    this.moreRequest = request;
    return request;
  }

  dispose() {
    this.revision++;
    this.controller?.abort();
    this.controller = null;
    this.refreshRequest = null;
    this.refreshEndpoint = '';
    this.moreRequest = null;
    this.subscribers.clear();
  }

  private publish(next: PagedResourceState<T>) {
    this.state = next;
    for (const subscriber of this.subscribers) subscriber(this.snapshot());
  }

  private async requestPage(cursor: string, signal?: AbortSignal, baseEndpoint = this.options.endpoint()): Promise<{ items: T[]; page: ReadPageMeta | null }> {
    const endpoint = withPageQuery(baseEndpoint, this.options.pageSize ?? 50, cursor);
    const init = this.options.requestInit?.() || {};
    const response = await (this.options.fetcher ?? fetch)(endpoint, {
      ...init,
      cache: init.cache ?? 'no-store',
      signal: signal ?? init.signal
    });
    const text = await response.text();
    let payload: Record<string, unknown> = {};
    if (text) {
      try {
        payload = JSON.parse(text) as Record<string, unknown>;
      } catch {
        payload = { message: text };
      }
    }
    if (!response.ok) {
      const message = String(payload.message || payload.error || `HTTP ${response.status}`);
      throw new PagedRequestError(response.status, message);
    }
    const items = this.options.selectItems
      ? this.options.selectItems(payload)
      : (Array.isArray(payload.items) ? payload.items as T[] : []);
    const page = isPageMeta(payload.page) ? payload.page : null;
    return { items, page };
  }
}

export class PagedRequestError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'PagedRequestError';
    this.status = status;
  }
}

function withPageQuery(endpoint: string, limit: number, cursor: string): string {
  const separator = endpoint.includes('?') ? '&' : '?';
  const query = new URLSearchParams({ limit: String(Math.min(100, Math.max(1, limit))) });
  if (cursor) query.set('cursor', cursor);
  return `${endpoint}${separator}${query.toString()}`;
}

function isPageMeta(value: unknown): value is ReadPageMeta {
  if (!value || typeof value !== 'object') return false;
  const page = value as Partial<ReadPageMeta>;
  return Number.isFinite(page.limit) && typeof page.has_more === 'boolean';
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError';
}

function errorMessage(error: unknown, fallback = '数据加载失败'): string {
  return error instanceof Error && error.message ? error.message : fallback;
}
