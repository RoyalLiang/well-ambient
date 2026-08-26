export type DataWindowDirection = 'previous' | 'next';

export interface BoundedPage<TItem, TMeta> {
  items: TItem[];
  meta: TMeta;
}

export function flattenBoundedPages<TItem, TMeta>(
  pages: BoundedPage<TItem, TMeta>[]
): TItem[] {
  return pages.flatMap((page) => page.items);
}

export function updateBoundedPageWindow<TItem, TMeta>(
  pages: BoundedPage<TItem, TMeta>[],
  incoming: BoundedPage<TItem, TMeta>,
  direction: DataWindowDirection,
  maxPages: number,
  itemKey: (item: TItem) => string
): BoundedPage<TItem, TMeta>[] {
  const pageLimit = Math.max(1, Math.floor(maxPages));
  const incomingKeys = new Set(incoming.items.map(itemKey));
  const retainedPages = pages
    .map((page) => ({
      ...page,
      items: page.items.filter((item) => !incomingKeys.has(itemKey(item)))
    }))
    .filter((page) => page.items.length > 0);
  const nextPages = direction === 'next'
    ? [...retainedPages, incoming]
    : [incoming, ...retainedPages];

  return direction === 'next'
    ? nextPages.slice(-pageLimit)
    : nextPages.slice(0, pageLimit);
}
