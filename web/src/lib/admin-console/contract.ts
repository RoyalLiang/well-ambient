export type AdminTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger';

export type AdminCellValue = string | number | boolean | null | undefined;

export interface AdminMetric {
  label: string;
  value: string | number;
  helper?: string;
  delta?: string;
  tone?: AdminTone;
}

export interface AdminTableColumn {
  key: string;
  label: string;
  width?: string;
  align?: 'left' | 'center' | 'right';
}

export interface AdminTableRow {
  id: string;
  title: string;
  status: string;
  tone?: AdminTone;
  owner?: string;
  dueDate?: string;
  priority?: string;
  risk?: string;
  cells: Record<string, AdminCellValue>;
}

export interface AdminInspectorFact {
  label: string;
  value: string | number;
}

export interface AdminInspectorSection {
  title: string;
  body?: string;
  items?: string[];
}

export interface AdminInspectorAction {
  label: string;
  kind: 'primary' | 'secondary' | 'danger';
}

export interface AdminInspectorRecord {
  id: string;
  title: string;
  status: string;
  tone?: AdminTone;
  facts: AdminInspectorFact[];
  sections: AdminInspectorSection[];
  actions?: AdminInspectorAction[];
}

export const ADMIN_TONE_CLASS: Record<AdminTone, string> = {
  neutral: 'tone-neutral',
  info: 'tone-info',
  success: 'tone-success',
  warning: 'tone-warning',
  danger: 'tone-danger'
};

export function toneForRisk(value: string | null | undefined): AdminTone {
  const normalized = normalizeSignal(value);
  if (!normalized) return 'neutral';
  if (matchesAny(normalized, ['critical', 'danger', 'high', 'block', 'blocked', 'overdue', '高风险', '阻断', '逾期'])) {
    return 'danger';
  }
  if (matchesAny(normalized, ['warning', 'medium', 'risk', 'pending', 'delay', '中风险', '待验收', '延期'])) {
    return 'warning';
  }
  if (matchesAny(normalized, ['low', 'safe', 'done', 'completed', '低风险', '已完成', '健康'])) {
    return 'success';
  }
  return 'info';
}

export function toneForStatus(value: string | null | undefined): AdminTone {
  const normalized = normalizeSignal(value);
  if (!normalized) return 'neutral';
  if (matchesAny(normalized, ['done', 'completed', 'closed', 'resolved', '已完成', '已关闭'])) {
    return 'success';
  }
  if (matchesAny(normalized, ['blocked', 'failed', 'high', 'error', '阻塞', '失败', '高风险'])) {
    return 'danger';
  }
  if (matchesAny(normalized, ['pending', 'queued', 'review', 'wait', '待', '验收', '评审'])) {
    return 'warning';
  }
  if (matchesAny(normalized, ['active', 'progress', 'running', '进行中', '处理中'])) {
    return 'info';
  }
  return 'neutral';
}

export function formatAdminDate(value: string | number | Date | null | undefined): string {
  if (!value) return '-';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toISOString().slice(0, 10);
}

function normalizeSignal(value: string | null | undefined): string {
  return String(value || '').trim().toLowerCase();
}

function matchesAny(value: string, needles: string[]): boolean {
  return needles.some((needle) => value.includes(needle.toLowerCase()));
}
