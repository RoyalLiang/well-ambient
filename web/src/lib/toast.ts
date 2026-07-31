import { writable } from 'svelte/store';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export type ToastNotice = {
  id: number;
  type: ToastType;
  title: string;
  message: string;
};

type ToastOptions = {
  type?: ToastType;
  title?: string;
  duration?: number;
};

const DEFAULT_DURATION = 6000;
const ERROR_DURATION = 8000;
const MAX_VISIBLE_TOASTS = 3;
const timers = new Map<number, ReturnType<typeof setTimeout>>();
let nextToastID = 0;

export const toastNotices = writable<ToastNotice[]>([]);

function clearToastTimer(id: number) {
  const timer = timers.get(id);
  if (timer) clearTimeout(timer);
  timers.delete(id);
}

export function dismissToast(id: number) {
  clearToastTimer(id);
  toastNotices.update((notices) => notices.filter((notice) => notice.id !== id));
}

export function showToast(message: string, options: ToastOptions = {}) {
  const id = ++nextToastID;
  const notice: ToastNotice = {
    id,
    type: options.type || 'success',
    title: options.title || '',
    message
  };

  toastNotices.update((notices) => {
    const deduplicated = notices.filter((item) => {
      const isDuplicate = item.type === notice.type && item.title === notice.title && item.message === notice.message;
      if (isDuplicate) clearToastTimer(item.id);
      return !isDuplicate;
    });
    const next = [...deduplicated, notice];
    while (next.length > MAX_VISIBLE_TOASTS) {
      const removed = next.shift();
      if (removed) clearToastTimer(removed.id);
    }
    return next;
  });

  const duration = options.duration ?? (notice.type === 'error' ? ERROR_DURATION : DEFAULT_DURATION);
  if (duration > 0) {
    timers.set(id, setTimeout(() => dismissToast(id), duration));
  }

  return id;
}
