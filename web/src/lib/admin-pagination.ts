export type PaginationItem = number | 'ellipsis-start' | 'ellipsis-end';

export interface PaginationSummary {
  page: number;
  pageCount: number;
  start: number;
  end: number;
  total: number;
}

export function getPageCount(total: number, pageSize: number): number {
  const safeTotal = Math.max(0, Math.floor(total));
  const safePageSize = Math.max(1, Math.floor(pageSize));
  return Math.max(1, Math.ceil(safeTotal / safePageSize));
}

export function clampPage(page: number, pageCount: number): number {
  return Math.min(Math.max(1, Math.floor(page || 1)), Math.max(1, Math.floor(pageCount || 1)));
}

export function getPaginationSummary(total: number, page: number, pageSize: number): PaginationSummary {
  const safeTotal = Math.max(0, Math.floor(total));
  const safePageSize = Math.max(1, Math.floor(pageSize));
  const pageCount = getPageCount(safeTotal, safePageSize);
  const safePage = clampPage(page, pageCount);
  const start = safeTotal === 0 ? 0 : (safePage - 1) * safePageSize + 1;
  const end = safeTotal === 0 ? 0 : Math.min(safeTotal, safePage * safePageSize);

  return { page: safePage, pageCount, start, end, total: safeTotal };
}

export function getPaginationItems(page: number, pageCount: number, maxNumericItems = 5): PaginationItem[] {
  const safePageCount = Math.max(1, Math.floor(pageCount || 1));
  const safePage = clampPage(page, safePageCount);
  const numericLimit = Math.max(3, Math.floor(maxNumericItems));

  if (safePageCount <= numericLimit) {
    return Array.from({ length: safePageCount }, (_, index) => index + 1);
  }

  const middleSlots = numericLimit - 2;
  let middleStart = Math.max(2, safePage - Math.floor(middleSlots / 2));
  let middleEnd = Math.min(safePageCount - 1, middleStart + middleSlots - 1);

  if (middleEnd - middleStart + 1 < middleSlots) {
    middleStart = Math.max(2, middleEnd - middleSlots + 1);
  }

  const items: PaginationItem[] = [1];
  if (middleStart > 2) items.push('ellipsis-start');
  for (let value = middleStart; value <= middleEnd; value += 1) items.push(value);
  if (middleEnd < safePageCount - 1) items.push('ellipsis-end');
  items.push(safePageCount);
  return items;
}
