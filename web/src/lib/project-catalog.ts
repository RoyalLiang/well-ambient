import type { DeliveryProjectOption } from './delivery-directory';
import { subscribeTelemetryUpdates } from './telemetry-refresh';

export type ProjectCatalogOption = DeliveryProjectOption;

export async function fetchProjectCatalog(signal?: AbortSignal): Promise<ProjectCatalogOption[]> {
  const response = await fetch('/api/projects/catalog', { cache: 'no-store', signal });
  if (!response.ok) throw new Error(`项目目录加载失败 (${response.status})`);
  const payload = await response.json();
  if (!Array.isArray(payload?.projects)) throw new Error('项目目录返回格式无效');
  const projects = new Map<string, ProjectCatalogOption>();
  for (const item of payload.projects) {
    const key = String(item?.project_key ?? '').trim().toUpperCase();
    if (key) projects.set(key, { project_key: key, project_name: String(item?.project_name || key).trim() });
  }
  return [...projects.values()].sort((a, b) => a.project_key.localeCompare(b.project_key));
}

// Sync notifications can arrive once per issue. Coalesce them without losing a
// change that arrives while the current directory request is still in flight.
export function watchProjectCatalog(
  onUpdate: (projects: ProjectCatalogOption[]) => void,
  onError: (message: string) => void
) {
  let disposed = false;
  let controller: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pending = false;
  async function refresh() {
    if (disposed) return;
    if (controller) { pending = true; return; }
    controller = new AbortController();
    try {
      const projects = await fetchProjectCatalog(controller.signal);
      if (!disposed) onUpdate(projects);
    } catch (error) {
      if (!disposed) onError(error instanceof Error ? error.message : '项目目录加载失败');
    } finally {
      controller = undefined;
      if (pending && !disposed) { pending = false; schedule(); }
    }
  }
  function schedule() {
    if (disposed || timer || document.visibilityState === 'hidden') return;
    timer = setTimeout(() => { timer = undefined; void refresh(); }, 300);
  }
  const unsubscribeTelemetry = subscribeTelemetryUpdates(window, refresh, { delayMs: 300, visibilityTarget: document });
  const events = ['jira-sync-complete', 'jira-sync-updated', 'config-updated', 'project-config-updated', 'focus'];
  for (const event of events) window.addEventListener(event, schedule);
  document.addEventListener('visibilitychange', schedule);
  void refresh();
  return {
    refresh,
    destroy() {
      disposed = true;
      clearTimeout(timer);
      controller?.abort();
      unsubscribeTelemetry();
      for (const event of events) window.removeEventListener(event, schedule);
      document.removeEventListener('visibilitychange', schedule);
    }
  };
}
