export const TELEMETRY_UPDATE_EVENT = 'well-ambient:telemetry-updated';

export interface TelemetryRefreshBatch {
  taskIDs: readonly string[];
}

export interface TelemetryVisibilityTarget extends EventTarget {
  readonly visibilityState: string;
}

export interface TelemetryRefreshOptions {
  delayMs?: number;
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
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let refreshRunning = false;
  let disposed = false;

  const isVisible = () => options.visibilityTarget?.visibilityState !== 'hidden';

  const scheduleRefresh = (delay = delayMs) => {
    if (disposed || refreshRunning || refreshTimer !== undefined || pendingTaskIDs.size === 0 || !isVisible()) {
      return;
    }
    refreshTimer = setTimeout(() => {
      void runRefresh();
    }, Math.max(0, delay));
  };

  const runRefresh = async () => {
    refreshTimer = undefined;
    if (disposed || refreshRunning || pendingTaskIDs.size === 0 || !isVisible()) return;

    refreshRunning = true;
    try {
      while (!disposed && isVisible() && pendingTaskIDs.size > 0) {
        const taskIDs = [...pendingTaskIDs];
        pendingTaskIDs.clear();
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
      if (!disposed && pendingTaskIDs.size > 0 && isVisible()) scheduleRefresh(0);
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

  return () => {
    disposed = true;
    pendingTaskIDs.clear();
    if (refreshTimer !== undefined) clearTimeout(refreshTimer);
    target.removeEventListener(TELEMETRY_UPDATE_EVENT, handleUpdate);
    options.visibilityTarget?.removeEventListener('visibilitychange', handleVisibilityChange);
  };
}
