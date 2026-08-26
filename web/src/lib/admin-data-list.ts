export interface VirtualWindowInput {
  totalRows: number;
  scrollTop: number;
  viewportHeight: number;
  rowHeight: number;
  overscan: number;
}

export interface VirtualWindow {
  start: number;
  end: number;
  topSpacerHeight: number;
  bottomSpacerHeight: number;
}

export interface LoadMoreGeometry {
  scrollTop: number;
  scrollHeight: number;
  viewportHeight: number;
  thresholdPx: number;
  hasMore: boolean;
  loading: boolean;
  hasError: boolean;
  disabled: boolean;
}

export function computeVirtualWindow(input: VirtualWindowInput): VirtualWindow {
  const totalRows = Math.max(0, Math.floor(input.totalRows));
  const rowHeight = Math.max(1, input.rowHeight);
  const viewportHeight = Math.max(0, input.viewportHeight);
  const overscan = Math.max(0, Math.floor(input.overscan));

  if (totalRows === 0) {
    return { start: 0, end: 0, topSpacerHeight: 0, bottomSpacerHeight: 0 };
  }

  if (totalRows * rowHeight <= viewportHeight) {
    return { start: 0, end: totalRows, topSpacerHeight: 0, bottomSpacerHeight: 0 };
  }

  const visibleRows = Math.max(1, Math.ceil(viewportHeight / rowHeight));
  const maximumStart = Math.max(0, totalRows - visibleRows);
  const start = Math.min(
    maximumStart,
    Math.max(0, Math.floor(Math.max(0, input.scrollTop) / rowHeight) - overscan)
  );
  const end = Math.min(totalRows, start + visibleRows + overscan * 2);

  return {
    start,
    end,
    topSpacerHeight: start * rowHeight,
    bottomSpacerHeight: (totalRows - end) * rowHeight
  };
}

export function shouldRequestMore(input: LoadMoreGeometry): boolean {
  if (!input.hasMore || input.loading || input.hasError || input.disabled) return false;
  const remaining = input.scrollHeight - input.scrollTop - input.viewportHeight;
  return remaining <= Math.max(0, input.thresholdPx);
}

export function shouldRequestPrevious(input: LoadMoreGeometry): boolean {
  if (!input.hasMore || input.loading || input.hasError || input.disabled) return false;
  return input.scrollTop <= Math.max(0, input.thresholdPx);
}

export function dedupeRowsByID<TRow extends { id: string }>(rows: TRow[]): TRow[] {
  const seen = new Set<string>();
  return rows.filter((row) => {
    if (seen.has(row.id)) return false;
    seen.add(row.id);
    return true;
  });
}
