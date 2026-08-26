<script lang="ts" generics="TRow extends AdminTableRow">
  import { tick, type Snippet } from 'svelte';
  import {
    computeVirtualWindow,
    dedupeRowsByID,
    shouldRequestMore,
    shouldRequestPrevious
  } from '../../lib/admin-data-list';
  import type {
    AdminCellValue,
    AdminTableColumn,
    AdminTableRow
  } from '../../lib/admin-console/contract';

  type PreviewState = 'default' | 'hover' | 'focus' | 'active';
  type Surface = 'standalone' | 'embedded';

  interface VirtualOptions {
    rowHeight: number;
    overscan?: number;
    loadAheadRows?: number;
  }

  interface ScrollAnchor {
    rowID: string;
    viewportOffset: number;
    contentBaseOffset: number;
  }

  interface Props<TRow extends AdminTableRow> {
    columns: AdminTableColumn[];
    rows: TRow[];
    caption: string;
    cell?: Snippet<[TRow, AdminTableColumn, AdminCellValue]>;
    actions?: Snippet<[TRow, boolean]>;
    pagination?: Snippet<[boolean]>;
    empty?: Snippet;
    loading?: boolean;
    error?: string;
    success?: string;
    disabled?: boolean;
    skeletonRows?: number;
    tableMinWidth?: string;
    compactTableMinWidth?: string;
    viewportHeight?: string;
    actionsLabel?: string;
    actionsWidth?: string;
    previewState?: PreviewState;
    surface?: Surface;
    className?: string;
    scrollRegionId?: string;
    scrollRegionRole?: 'region' | 'tabpanel';
    virtual?: VirtualOptions;
    totalRowCount?: number;
    selectedRowId?: string;
    resetKey?: string;
    hasPrevious?: boolean;
    loadingPrevious?: boolean;
    loadPreviousError?: string;
    loadPreviousKey?: string;
    onLoadPrevious?: () => void | Promise<void>;
    hasMore?: boolean;
    loadingMore?: boolean;
    loadMoreError?: string;
    loadMoreKey?: string;
    onLoadMore?: () => void | Promise<void>;
    onRetry?: () => void;
  }

  let {
    columns,
    rows,
    caption,
    cell,
    actions,
    pagination,
    empty,
    loading = false,
    error = '',
    success = '',
    disabled = false,
    skeletonRows = 5,
    tableMinWidth = '760px',
    compactTableMinWidth = '',
    viewportHeight = '',
    actionsLabel = '操作',
    actionsWidth = '180px',
    previewState = 'default',
    surface = 'standalone',
    className = '',
    scrollRegionId,
    scrollRegionRole = 'region',
    virtual,
    totalRowCount,
    selectedRowId = '',
    resetKey = '',
    hasPrevious = false,
    loadingPrevious = false,
    loadPreviousError = '',
    loadPreviousKey = '',
    onLoadPrevious,
    hasMore = false,
    loadingMore = false,
    loadMoreError = '',
    loadMoreKey = '',
    onLoadMore,
    onRetry
  }: Props<TRow> = $props();

  let scrollRegion = $state<HTMLDivElement>();
  let scrollTop = $state(0);
  let viewportPixels = $state(0);
  let requestInFlight = $state<'' | 'previous' | 'next'>('');
  let consumedPreviousKey = $state('');
  let consumedNextKey = $state('');
  let previousResetKey = '';

  let uniqueRows = $derived(dedupeRowsByID(rows));
  let hasRows = $derived(uniqueRows.length > 0);
  let hasActions = $derived(Boolean(actions));
  let columnCount = $derived(columns.length + (hasActions ? 1 : 0));
  let skeletonItems = $derived(Array.from({ length: Math.max(1, skeletonRows) }));
  let controlsDisabled = $derived(disabled || (loading && !hasRows));
  let rowHeight = $derived(Math.max(1, virtual?.rowHeight ?? 48));
  let overscan = $derived(Math.max(0, virtual?.overscan ?? 6));
  let loadAheadRows = $derived(Math.max(1, virtual?.loadAheadRows ?? 8));
  let virtualWindow = $derived(virtual
    ? computeVirtualWindow({
        totalRows: uniqueRows.length,
        scrollTop,
        viewportHeight: viewportPixels,
        rowHeight,
        overscan
      })
    : {
        start: 0,
        end: uniqueRows.length,
        topSpacerHeight: 0,
        bottomSpacerHeight: 0
      });
  let visibleRows = $derived(uniqueRows.slice(virtualWindow.start, virtualWindow.end));
  let appendMode = $derived(Boolean(onLoadMore || onLoadPrevious));
  let activePreviousKey = $derived(loadPreviousKey || `${resetKey}:previous:${uniqueRows[0]?.id || ''}`);
  let activeNextKey = $derived(loadMoreKey || `${resetKey}:next:${uniqueRows.at(-1)?.id || ''}`);
  let listStyle = $derived([
    `--admin-row-height: ${rowHeight}px`,
    `--admin-table-min-width: ${tableMinWidth}`,
    compactTableMinWidth ? `--admin-compact-table-min-width: ${compactTableMinWidth}` : '',
    viewportHeight ? `--admin-viewport-height: ${viewportHeight}` : ''
  ].filter(Boolean).join('; '));

  function formatCell(value: AdminCellValue): string {
    if (value === null || value === undefined || value === '') return '-';
    if (typeof value === 'boolean') return value ? '是' : '否';
    return String(value);
  }

  function updateScrollMetrics(region = scrollRegion) {
    if (!region) return;
    scrollTop = region.scrollTop;
    viewportPixels = region.clientHeight;
  }

  function captureScrollAnchor(): ScrollAnchor | null {
    const region = scrollRegion;
    if (!region || !virtual) return null;
    const regionTop = region.getBoundingClientRect().top;
    const visibleRow = Array.from(region.querySelectorAll<HTMLElement>('tr.data-row'))
      .find((row) => row.getBoundingClientRect().bottom > regionTop);
    const rowID = visibleRow?.dataset.rowId;
    if (!visibleRow || !rowID) return null;
    const rowIndex = uniqueRows.findIndex((row) => row.id === rowID);
    if (rowIndex < 0) return null;
    const viewportOffset = visibleRow.getBoundingClientRect().top - regionTop;
    return {
      rowID,
      viewportOffset,
      contentBaseOffset: region.scrollTop + viewportOffset - rowIndex * rowHeight
    };
  }

  async function restoreScrollAnchor(anchor: ScrollAnchor | null) {
    const region = scrollRegion;
    if (!region || !anchor) return;
    const nextRowIndex = uniqueRows.findIndex((row) => row.id === anchor.rowID);
    if (nextRowIndex < 0) return;
    region.scrollTop = anchor.contentBaseOffset
      + nextRowIndex * rowHeight
      - anchor.viewportOffset;
    updateScrollMetrics(region);
    for (let pass = 0; pass < 2; pass += 1) {
      await tick();
      await waitForScrollSettlement();
      const row = Array.from(region.querySelectorAll<HTMLElement>('tr.data-row'))
        .find((candidate) => candidate.dataset.rowId === anchor.rowID);
      if (!row) continue;
      const currentOffset = row.getBoundingClientRect().top - region.getBoundingClientRect().top;
      const correction = currentOffset - anchor.viewportOffset;
      if (Math.abs(correction) < 1) return;
      region.scrollTop += correction;
      updateScrollMetrics(region);
    }
  }

  function waitForScrollSettlement() {
    return new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
  }

  function scheduleEdgeCheck() {
    queueMicrotask(checkEdges);
  }

  function checkEdges() {
    const region = scrollRegion;
    if (!region) return;
    const thresholdPx = rowHeight * loadAheadRows;
    if (onLoadPrevious && shouldRequestPrevious({
      scrollTop: region.scrollTop,
      scrollHeight: region.scrollHeight,
      viewportHeight: region.clientHeight,
      thresholdPx,
      hasMore: hasPrevious,
      loading: loadingPrevious || Boolean(requestInFlight),
      hasError: Boolean(loadPreviousError),
      disabled
    })) {
      void requestPrevious(false);
      return;
    }
    if (!onLoadMore || !shouldRequestMore({
      scrollTop: region.scrollTop,
      scrollHeight: region.scrollHeight,
      viewportHeight: region.clientHeight,
      thresholdPx,
      hasMore,
      loading: loadingMore || Boolean(requestInFlight),
      hasError: Boolean(loadMoreError),
      disabled
    })) return;
    void requestNext(false);
  }

  async function requestPrevious(force: boolean) {
    if (!onLoadPrevious || disabled || !hasPrevious || loadingPrevious || requestInFlight) return;
    if (loadPreviousError && !force) return;
    if (!force && consumedPreviousKey === activePreviousKey) return;

    consumedPreviousKey = activePreviousKey;
    requestInFlight = 'previous';
    const anchor = captureScrollAnchor();
    try {
      await onLoadPrevious();
      consumedNextKey = '';
      await tick();
      await waitForScrollSettlement();
      await tick();
      await restoreScrollAnchor(anchor);
      await waitForScrollSettlement();
    } catch (loadError) {
      console.error('AdminDataList load-previous callback failed:', loadError);
    } finally {
      requestInFlight = '';
    }
  }

  async function requestNext(force: boolean) {
    if (!onLoadMore || disabled || !hasMore || loadingMore || requestInFlight) return;
    if (loadMoreError && !force) return;
    if (!force && consumedNextKey === activeNextKey) return;

    consumedNextKey = activeNextKey;
    requestInFlight = 'next';
    const anchor = captureScrollAnchor();
    try {
      await onLoadMore();
      consumedPreviousKey = '';
      await tick();
      await waitForScrollSettlement();
      await tick();
      await restoreScrollAnchor(anchor);
      await waitForScrollSettlement();
    } catch (loadError) {
      console.error('AdminDataList load-more callback failed:', loadError);
    } finally {
      requestInFlight = '';
    }
  }

  function handleTableScroll(event: Event) {
    updateScrollMetrics(event.currentTarget as HTMLDivElement);
    checkEdges();
  }

  function handleTableScrollKey(event: KeyboardEvent) {
    const region = event.currentTarget as HTMLElement;
    if (event.target !== region || (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight')) return;

    event.preventDefault();
    const direction = event.key === 'ArrowRight' ? 1 : -1;
    const step = Math.max(48, Math.round(region.clientWidth * 0.18));
    region.scrollLeft += direction * step;
  }

  $effect(() => {
    const region = scrollRegion;
    if (!region) return;
    updateScrollMetrics(region);
    const resizeObserver = typeof ResizeObserver === 'undefined'
      ? null
      : new ResizeObserver(() => {
          updateScrollMetrics(region);
          scheduleEdgeCheck();
        });
    resizeObserver?.observe(region);
    scheduleEdgeCheck();
    return () => resizeObserver?.disconnect();
  });

  $effect(() => {
    const currentResetKey = resetKey;
    if (currentResetKey === previousResetKey) return;
    previousResetKey = currentResetKey;
    consumedPreviousKey = '';
    consumedNextKey = '';
    scrollTop = 0;
    if (scrollRegion) scrollRegion.scrollTop = 0;
  });
</script>

<section
  class="admin-data-list state-{previewState} {className}"
  class:is-disabled={disabled}
  class:is-embedded={surface === 'embedded'}
  class:is-virtual={Boolean(virtual)}
  data-component="AdminDataList"
  data-style-contract="admin-data-list/v1"
  data-surface-contract={surface === 'embedded' ? 'admin-data-list-embedded/v1' : 'admin-data-list-surface/v1'}
  data-row-height={rowHeight}
  data-surface={surface}
  aria-busy={loading || loadingPrevious || loadingMore || Boolean(requestInFlight)}
  style={listStyle}
>
  {#if success}
    <div class="list-status success-status" class:is-overlay={hasRows} role="status" aria-live="polite">
      <strong>已完成</strong>
      <span>{success}</span>
    </div>
  {:else if error && hasRows}
    <div class="list-status error-status" class:is-overlay={hasRows} role="alert">
      <span>{error}，当前结果保留显示。</span>
      {#if onRetry}
        <button type="button" class="wa-admin-action secondary" disabled={disabled} onclick={onRetry}>重新加载</button>
      {/if}
    </div>
  {:else if loading && hasRows}
    <div class="list-status loading-status" class:is-overlay={hasRows} role="status" aria-live="polite">
      正在更新，当前结果保留显示。
    </div>
  {/if}

  <!-- The scroll owner is intentionally keyboard-focusable so keyboard users can pan wide tables. -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions (wide semantic tables need a focusable keyboard scroll owner) -->
  <div
    bind:this={scrollRegion}
    id={scrollRegionId}
    class="table-scroll"
    role={scrollRegionRole}
    aria-label={`${caption}，可滚动浏览`}
    tabindex="0"
    onscroll={handleTableScroll}
    onkeydown={handleTableScrollKey}
  >
    <table
      class="admin-table wa-admin-table"
      aria-rowcount={totalRowCount === undefined ? undefined : totalRowCount + 1}
    >
      <caption>{caption}</caption>
      <colgroup>
        {#each columns as column (column.key)}
          <col
            class:column-secondary={column.priority === 'secondary'}
            style:width={column.width}
          />
        {/each}
        {#if hasActions}
          <col style:width={actionsWidth} />
        {/if}
      </colgroup>
      <thead>
        <tr>
          {#each columns as column (column.key)}
            <th
              scope="col"
              class:align-center={column.align === 'center'}
              class:align-right={column.align === 'right'}
              class:column-secondary={column.priority === 'secondary'}
            >{column.label}</th>
          {/each}
          {#if hasActions}
            <th scope="col" class="actions-heading">{actionsLabel}</th>
          {/if}
        </tr>
      </thead>
      <tbody>
        {#if loading && !hasRows}
          {#each skeletonItems as _, rowIndex}
            <tr class="skeleton-row" aria-hidden="true">
              {#each columns as column, columnIndex (column.key)}
                <td
                  class:align-center={column.align === 'center'}
                  class:align-right={column.align === 'right'}
                  class:column-secondary={column.priority === 'secondary'}
                >
                  <span class="skeleton-line" class:is-short={(rowIndex + columnIndex) % 3 === 1}></span>
                </td>
              {/each}
              {#if hasActions}
                <td><span class="skeleton-line is-short"></span></td>
              {/if}
            </tr>
          {/each}
        {:else if error && !hasRows}
          <tr class="view-state-row">
            <td colspan={columnCount}>
              <div class="view-state error-view" role="alert">
                <strong>列表暂时无法加载</strong>
                <p>{error}</p>
                {#if onRetry}
                  <button type="button" class="wa-admin-action secondary" disabled={disabled} onclick={onRetry}>重新加载</button>
                {/if}
              </div>
            </td>
          </tr>
        {:else if !hasRows}
          <tr class="view-state-row">
            <td colspan={columnCount}>
              {#if empty}
                {@render empty()}
              {:else}
                <div class="view-state empty-view">
                  <strong>暂无内容</strong>
                  <p>当前条件下没有可显示的数据，请调整筛选条件后重试。</p>
                </div>
              {/if}
            </td>
          </tr>
        {:else}
          {#if onLoadPrevious && (loadPreviousError || loadingPrevious || requestInFlight === 'previous' || hasPrevious)}
            <tr class="load-previous-row">
              <td colspan={columnCount}>
                {#if loadPreviousError}
                  <div class="load-previous-control load-previous-error" role="alert">
                    <span>{loadPreviousError}</span>
                    <button
                      type="button"
                      class="wa-admin-action secondary"
                      disabled={disabled || loadingPrevious || Boolean(requestInFlight)}
                      onclick={() => requestPrevious(true)}
                    >重新加载</button>
                  </div>
                {:else if loadingPrevious || requestInFlight === 'previous'}
                  <div class="load-previous-control" role="status" aria-live="polite">正在回补前页</div>
                {:else if hasPrevious}
                  <div class="load-previous-control">
                    <button
                      type="button"
                      class="wa-admin-action secondary"
                      disabled={disabled || Boolean(requestInFlight)}
                      onclick={() => requestPrevious(true)}
                    >加载前页</button>
                  </div>
                {/if}
              </td>
            </tr>
          {/if}
          {#if virtualWindow.topSpacerHeight > 0}
            <tr class="virtual-spacer" aria-hidden="true">
              <td colspan={columnCount} style:height={`${virtualWindow.topSpacerHeight}px`}></td>
            </tr>
          {/if}
          {#each visibleRows as row, visibleIndex (row.id)}
            <tr
              class="data-row"
              class:selected={selectedRowId === row.id}
              aria-rowindex={virtualWindow.start + visibleIndex + 2}
              data-row-id={row.id}
            >
              {#each columns as column (column.key)}
                {@const value = row.cells[column.key]}
                <td
                  class:align-center={column.align === 'center'}
                  class:align-right={column.align === 'right'}
                  class:column-secondary={column.priority === 'secondary'}
                >
                  {#if cell}
                    {@render cell(row, column, value)}
                  {:else}
                    <span class="cell-value" title={formatCell(value)}>{formatCell(value)}</span>
                  {/if}
                </td>
              {/each}
              {#if actions}
                <td class="row-actions">
                  <div class="action-group">
                    {@render actions(row, controlsDisabled)}
                  </div>
                </td>
              {/if}
            </tr>
          {/each}
          {#if virtualWindow.bottomSpacerHeight > 0}
            <tr class="virtual-spacer" aria-hidden="true">
              <td colspan={columnCount} style:height={`${virtualWindow.bottomSpacerHeight}px`}></td>
            </tr>
          {/if}
          {#if appendMode}
            <tr class="load-more-row">
              <td colspan={columnCount}>
                {#if loadMoreError}
                  <div class="load-more-control load-more-error" role="alert">
                    <span>{loadMoreError}</span>
                    <button
                      type="button"
                      class="wa-admin-action secondary"
                      disabled={disabled || loadingMore || Boolean(requestInFlight)}
                      onclick={() => requestNext(true)}
                    >重新加载</button>
                  </div>
                {:else if loadingMore || requestInFlight === 'next'}
                  <div class="load-more-control" role="status" aria-live="polite">正在加载更多</div>
                {:else if hasMore}
                  <div class="load-more-control">
                    <button
                      type="button"
                      class="wa-admin-action secondary"
                      disabled={disabled || Boolean(requestInFlight)}
                      onclick={() => requestNext(true)}
                    >加载更多</button>
                  </div>
                {:else}
                  <div class="load-more-control is-complete" role="status" aria-live="polite">
                    {hasPrevious ? '已到当前列表末尾' : `已加载全部 ${uniqueRows.length} 项`}
                  </div>
                {/if}
              </td>
            </tr>
          {/if}
        {/if}
      </tbody>
    </table>
  </div>

  {#if pagination && !appendMode}
    <footer class="list-footer">
      {@render pagination(controlsDisabled)}
    </footer>
  {/if}
</section>

<style>
  /* finesse · component: data-list · register=product
   * states: default · hover · focus-visible · active · disabled · loading · error · success
   * tokens: inherited (modern-admin-tokens.css) */
  .admin-data-list {
    position: relative;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--wa-admin-list-surface-border);
    border-top-color: var(--wa-admin-list-surface-border-top);
    border-radius: var(--wa-admin-list-surface-radius);
    background: var(--wa-admin-list-surface-background);
    box-shadow: var(--wa-admin-list-surface-shadow);
    -webkit-backdrop-filter: var(--wa-admin-list-surface-filter);
    backdrop-filter: var(--wa-admin-list-surface-filter);
  }

  .admin-data-list.is-embedded {
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
    border: 0;
    border-top: 1px solid var(--wa-border-divider);
    border-radius: 0;
    background: transparent;
    box-shadow: none;
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }

  .list-status {
    min-height: var(--wa-touch-h);
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
    padding: var(--wa-space-2) var(--wa-space-4);
    border-bottom: 1px solid var(--wa-border-divider);
    color: var(--wa-text-main);
    font-size: 13px;
  }

  .list-status strong {
    color: var(--wa-text-strong);
  }

  .list-status.is-overlay {
    position: absolute;
    z-index: 4;
    top: var(--wa-space-2);
    right: var(--wa-space-3);
    min-height: 32px;
    max-width: calc(100% - var(--wa-space-6));
    padding: var(--wa-space-1) var(--wa-space-3);
    border: 1px solid var(--wa-border-divider);
    border-radius: var(--wa-radius-md);
    box-shadow: var(--wa-shadow-sm);
  }

  .success-status {
    background: var(--wa-success-soft);
    color: var(--wa-success);
  }

  .error-status,
  .error-view {
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .error-status .wa-admin-action {
    margin-left: auto;
  }

  .loading-status {
    background: var(--wa-neutral-soft);
    color: var(--wa-text-muted);
  }

  .loading-status.is-overlay {
    pointer-events: none;
  }

  .table-scroll {
    flex: 1 1 auto;
    min-width: 0;
    min-height: 0;
    height: var(--admin-viewport-height, auto);
    overflow: auto;
    background: var(--wa-surface-lift);
    outline: 2px solid transparent;
    outline-offset: -2px;
    scrollbar-gutter: stable;
    overscroll-behavior: contain;
    overflow-anchor: none;
  }

  .is-embedded .table-scroll {
    flex: 1 1 auto;
    min-height: 0;
    height: auto;
    background: transparent;
  }

  .table-scroll:focus-visible {
    outline-color: var(--wa-border-focus);
  }

  .admin-table {
    min-width: var(--admin-table-min-width);
    border-radius: 0;
  }

  .admin-table caption {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .admin-table th,
  .admin-table td {
    box-sizing: border-box;
    height: var(--admin-row-height);
    padding-block: 0;
  }

  .admin-table tbody tr.data-row {
    height: var(--admin-row-height);
  }

  .admin-table tbody tr.data-row > td {
    height: var(--admin-row-height);
    max-height: var(--admin-row-height);
  }

  .admin-table th {
    position: sticky;
    top: 0;
    z-index: 2;
    height: var(--wa-touch-h);
    overflow: hidden;
    border-bottom-color: var(--wa-border-strong);
    background-color: var(--wa-surface-inset);
    background-image: linear-gradient(
      180deg,
      color-mix(in srgb, var(--wa-surface-flat) 94%, var(--wa-accent-soft) 6%) 0%,
      color-mix(in srgb, var(--wa-surface-inset) 96%, var(--wa-accent-soft) 4%) 100%
    );
    background-clip: padding-box;
    color: var(--wa-text-strong);
    font-weight: 760;
    letter-spacing: 0.015em;
    text-overflow: ellipsis;
    white-space: nowrap;
    box-shadow:
      inset 0 1px 0 var(--wa-glass-highlight);
  }

  .admin-table th.align-center,
  .admin-table td.align-center {
    text-align: center;
  }

  .admin-table th.align-right,
  .admin-table td.align-right {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .cell-value {
    display: block;
    overflow: hidden;
    color: var(--wa-text-main);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .actions-heading,
  .row-actions {
    text-align: right;
  }

  .action-group {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--wa-space-2);
  }

  .admin-data-list :global(.action-group .wa-admin-action) {
    min-height: var(--wa-control-h);
    padding-inline: var(--wa-space-3);
    border-radius: var(--wa-radius-md);
    box-shadow: none;
    white-space: nowrap;
  }

  .data-row.selected {
    background: var(--wa-accent-soft);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--wa-accent) 16%, transparent);
  }

  .virtual-spacer {
    background: transparent;
    pointer-events: none;
  }

  .virtual-spacer td {
    height: auto;
    padding: 0;
    border: 0;
  }

  .view-state-row td,
  .load-previous-row td,
  .load-more-row td {
    padding: 0;
  }

  .view-state {
    min-height: 220px;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: var(--wa-space-2);
    padding: var(--wa-space-8);
    text-align: center;
  }

  .view-state strong {
    color: var(--wa-text-strong);
    font-size: 16px;
  }

  .view-state p {
    max-width: 48ch;
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.55;
  }

  .skeleton-line {
    display: block;
    width: min(100%, 180px);
    height: 10px;
    border-radius: var(--wa-radius-pill);
    background: var(--wa-neutral-soft);
  }

  .skeleton-line.is-short {
    width: min(64%, 96px);
  }

  .load-previous-control,
  .load-more-control {
    min-height: var(--wa-touch-h);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--wa-space-3);
    padding: var(--wa-space-2) var(--wa-space-4);
    border-top: 1px solid var(--wa-border-divider);
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .load-previous-control {
    border-top: 0;
    border-bottom: 1px solid var(--wa-border-divider);
  }

  .load-previous-error,
  .load-more-error {
    color: var(--wa-danger);
  }

  .load-previous-control .wa-admin-action,
  .load-more-control .wa-admin-action {
    min-height: var(--wa-touch-h);
  }

  .load-previous-control.is-complete,
  .load-more-control.is-complete {
    color: var(--wa-text-subtle);
  }

  .list-footer {
    min-height: 60px;
    display: grid;
    align-items: center;
    padding: var(--wa-space-3) var(--wa-space-4);
    border-top: 1px solid var(--wa-border-divider);
    background: var(--wa-glass-panel-strong);
  }

  .admin-data-list :global(button:focus-visible),
  .admin-data-list :global(select:focus-visible),
  .state-focus :global(.wa-admin-action:first-of-type) {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  @media (hover: hover) {
    .admin-table tbody tr.data-row:hover,
    .state-hover .admin-table tbody tr.data-row:first-child {
      background: var(--wa-row-hover);
    }

    .admin-data-list :global(.action-group .wa-admin-action:not(:disabled):hover),
    .state-hover :global(.action-group .wa-admin-action:first-child) {
      border-color: var(--wa-border-strong);
      background: var(--wa-surface-flat);
    }
  }

  .admin-data-list :global(.action-group .wa-admin-action:not(:disabled):active),
  .state-active :global(.action-group .wa-admin-action:first-child) {
    transform: translateY(1px);
    background: var(--wa-surface-inset);
  }

  .admin-data-list.is-disabled :global(button),
  .admin-data-list.is-disabled :global(select) {
    opacity: 0.5;
    cursor: not-allowed;
  }

  @media (hover: none) {
    .admin-table tbody tr.data-row:hover {
      background: transparent;
    }

    .admin-table tbody tr.data-row.selected:hover {
      background: var(--wa-accent-soft);
    }
  }

  @media (max-width: 760px) {
    .admin-table {
      min-width: var(--admin-compact-table-min-width, var(--admin-table-min-width));
    }

    .column-secondary {
      display: none;
    }

    .admin-table th,
    .admin-table td {
      padding-inline: var(--wa-space-3);
    }

    .admin-data-list :global(.action-group .wa-admin-action),
    .load-more-control .wa-admin-action {
      min-height: var(--wa-touch-h);
    }

    .list-footer {
      padding: var(--wa-space-3);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .admin-data-list *,
    .admin-data-list *::before,
    .admin-data-list *::after {
      transition-duration: 0.001ms !important;
    }
  }

  @media (prefers-reduced-transparency: reduce) {
    .admin-data-list:not(.is-embedded) {
      background: var(--wa-admin-list-surface-solid-background);
      -webkit-backdrop-filter: none;
      backdrop-filter: none;
    }
  }
</style>
