export const TELEMETRY_UPDATE_EVENT = 'well-ambient:telemetry-updated';

export interface TelemetryRefreshBatch {
  taskIDs: readonly string[];
}

export interface TelemetryVisibilityTarget extends EventTarget {
  readonly visibilityState: string;
}

export interface TelemetryRefreshOptions {
  delayMs?: number;
  events?: readonly string[];
  shouldRefresh?: (taskID: string) => boolean;
  visibilityTarget?: TelemetryVisibilityTarget;
  onError?: (error: unknown) => void;
}

export type TelemetryRefresh = (batch: TelemetryRefreshBatch) => void | Promise<void>;

export function subscribeTelemetryUpdates(
  target: EventTarget,
  refresh: TelemetryRefresh,
  options: TelemetryRefreshOptions = {}
): () => void {
  const delayMs = Math.max(0, options.delayMs ?? 120);
  const pendingTaskIDs = new Set<string>();
  let pendingFullRefresh = false;
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let refreshRunning = false;
  let disposed = false;

  const isVisible = () => options.visibilityTarget?.visibilityState !== 'hidden';
  const hasPendingRefresh = () => pendingFullRefresh || pendingTaskIDs.size > 0;

  const scheduleRefresh = (delay = delayMs) => {
    if (disposed || refreshRunning || refreshTimer !== undefined || !hasPendingRefresh() || !isVisible()) {
      return;
    }
    refreshTimer = setTimeout(() => {
      void runRefresh();
    }, Math.max(0, delay));
  };

  const runRefresh = async () => {
    refreshTimer = undefined;
    if (disposed || refreshRunning || !hasPendingRefresh() || !isVisible()) return;

    refreshRunning = true;
    try {
      while (!disposed && isVisible() && hasPendingRefresh()) {
        const taskIDs = [...pendingTaskIDs];
        pendingTaskIDs.clear();
        pendingFullRefresh = false;
        try {
          await refresh({ taskIDs });
        } catch (error) {
          try {
            options.onError?.(error);
          } catch {
            // Error observers must not break the refresh subscription.
          }
          break;
        }
      }
    } finally {
      refreshRunning = false;
      if (!disposed && hasPendingRefresh() && isVisible()) scheduleRefresh(0);
    }
  };

  const handleUpdate = (event: Event) => {
    const taskID = String((event as CustomEvent<{ task_id?: string }>).detail?.task_id || '').trim();
    if (disposed || !taskID || (options.shouldRefresh && !options.shouldRefresh(taskID))) return;

    pendingTaskIDs.add(taskID);
    if (refreshRunning || !isVisible()) return;
    if (refreshTimer !== undefined) {
      clearTimeout(refreshTimer);
      refreshTimer = undefined;
    }
    scheduleRefresh();
  };

  const requestFullRefresh = () => {
    if (disposed) return;
    pendingFullRefresh = true;
    scheduleRefresh();
  };
  const handleVisibilityChange = () => {
    if (!isVisible()) {
      if (refreshTimer !== undefined) {
        clearTimeout(refreshTimer);
        refreshTimer = undefined;
      }
      return;
    }
    scheduleRefresh(0);
  };

  target.addEventListener(TELEMETRY_UPDATE_EVENT, handleUpdate);
  options.visibilityTarget?.addEventListener('visibilitychange', handleVisibilityChange);
  for (const event of options.events ?? []) target.addEventListener(event, requestFullRefresh);

  return () => {
    disposed = true;
    pendingTaskIDs.clear();
    pendingFullRefresh = false;
    if (refreshTimer !== undefined) clearTimeout(refreshTimer);
    target.removeEventListener(TELEMETRY_UPDATE_EVENT, handleUpdate);
    options.visibilityTarget?.removeEventListener('visibilitychange', handleVisibilityChange);
    for (const event of options.events ?? []) target.removeEventListener(event, requestFullRefresh);
  };
}
