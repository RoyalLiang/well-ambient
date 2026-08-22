export type DailyJiraRefresh = () => void | Promise<void>;

export function subscribeDailyJiraUpdates(
  target: EventTarget,
  refresh: DailyJiraRefresh,
  delayMs = 120
): () => void {
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let refreshRunning = false;
  let refreshQueued = false;
  let disposed = false;

  const runRefresh = async () => {
    refreshTimer = undefined;
    if (refreshRunning) {
      refreshQueued = true;
      return;
    }

    refreshRunning = true;
    try {
      do {
        refreshQueued = false;
        await refresh();
      } while (!disposed && refreshQueued);
    } finally {
      refreshRunning = false;
    }
  };

  const handleUpdate = (event: Event) => {
    const taskID = String((event as CustomEvent<{ task_id?: string }>).detail?.task_id || '').trim();
    if (disposed || !taskID) return;
    if (refreshRunning) {
      refreshQueued = true;
      return;
    }
    if (refreshTimer !== undefined) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      void runRefresh();
    }, Math.max(0, delayMs));
  };

  target.addEventListener('well-ambient:telemetry-updated', handleUpdate);
  return () => {
    disposed = true;
    refreshQueued = false;
    if (refreshTimer !== undefined) clearTimeout(refreshTimer);
    target.removeEventListener('well-ambient:telemetry-updated', handleUpdate);
  };
}
