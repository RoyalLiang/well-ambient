<script lang="ts">
  import {
    getPaginationItems,
    getPaginationSummary
  } from '../../lib/admin-pagination';

  interface Props {
    page: number;
    pageSize: number;
    total: number;
    pageSizeOptions?: number[];
    disabled?: boolean;
    loading?: boolean;
    ariaLabel?: string;
    onPageChange?: (page: number) => void;
    onPageSizeChange?: (pageSize: number) => void;
  }

  let {
    page,
    pageSize,
    total,
    pageSizeOptions = [10, 25, 50, 100],
    disabled = false,
    loading = false,
    ariaLabel = '分页',
    onPageChange = () => {},
    onPageSizeChange = () => {}
  }: Props = $props();

  let summary = $derived(getPaginationSummary(total, page, pageSize));
  let items = $derived(getPaginationItems(summary.page, summary.pageCount));
  let controlsDisabled = $derived(disabled || loading);

  function changePage(nextPage: number) {
    if (controlsDisabled || nextPage === summary.page) return;
    onPageChange(nextPage);
  }

  function changePageSize(event: Event) {
    if (controlsDisabled) return;
    const nextPageSize = Number((event.currentTarget as HTMLSelectElement).value);
    if (!Number.isFinite(nextPageSize) || nextPageSize <= 0 || nextPageSize === pageSize) return;
    onPageSizeChange(nextPageSize);
  }
</script>

<div class="admin-pagination" aria-busy={loading}>
  <p class="pagination-summary" aria-live="polite">
    共 {summary.total} 条，当前显示 {summary.start}-{summary.end}
  </p>

  <label class="page-size-control">
    <span>每页</span>
    <select
      aria-label="每页条数"
      value={pageSize}
      disabled={controlsDisabled}
      onchange={changePageSize}
    >
      {#each pageSizeOptions as option}
        <option value={option}>{option} 条</option>
      {/each}
    </select>
  </label>

  <nav aria-label={ariaLabel}>
    <button
      type="button"
      class="wa-admin-action secondary pagination-button pagination-edge"
      disabled={controlsDisabled || summary.page <= 1}
      aria-label="上一页"
      onclick={() => changePage(summary.page - 1)}
    >上一页</button>

    <div class="page-numbers" role="group" aria-label={`第 ${summary.page} 页，共 ${summary.pageCount} 页`}>
      {#each items as item}
        {#if typeof item === 'number'}
          <button
            type="button"
            class="wa-admin-action secondary pagination-button page-number"
            class:is-current={item === summary.page}
            aria-current={item === summary.page ? 'page' : undefined}
            aria-label={`第 ${item} 页`}
            disabled={controlsDisabled || item === summary.page}
            onclick={() => changePage(item)}
          >{item}</button>
        {:else}
          <span class="page-ellipsis" aria-hidden="true">…</span>
        {/if}
      {/each}
    </div>

    <span class="compact-page-summary" aria-hidden="true">
      第 {summary.page}/{summary.pageCount} 页
    </span>

    <button
      type="button"
      class="wa-admin-action secondary pagination-button pagination-edge"
      disabled={controlsDisabled || summary.page >= summary.pageCount}
      aria-label="下一页"
      onclick={() => changePage(summary.page + 1)}
    >下一页</button>
  </nav>
</div>

<style>
  /* finesse · component: pagination · register=product
   * states: default · hover · focus-visible · active · disabled · loading
   * feedback: error and success are owned by the parent data-list boundary
   * tokens: inherited (modern-admin-tokens.css) */
  .admin-pagination {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--wa-space-3);
    color: var(--wa-text-muted);
    font-size: 13px;
  }

  .pagination-summary {
    margin: 0 auto 0 0;
    color: var(--wa-text-muted);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .page-size-control,
  nav,
  .page-numbers {
    display: flex;
    align-items: center;
  }

  .page-size-control {
    gap: var(--wa-space-2);
    white-space: nowrap;
  }

  .page-size-control select {
    min-height: var(--wa-control-h);
    padding: 0 var(--wa-space-8) 0 var(--wa-space-3);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background-color: var(--wa-surface-raised);
    color: var(--wa-text-main);
    cursor: pointer;
  }

  nav,
  .page-numbers {
    gap: var(--wa-space-1);
  }

  .pagination-button.wa-admin-action {
    min-width: var(--wa-control-h);
    padding-inline: var(--wa-space-3);
    border-radius: var(--wa-radius-md);
    box-shadow: none;
    white-space: nowrap;
  }

  .pagination-button.page-number {
    padding-inline: var(--wa-space-2);
    font-variant-numeric: tabular-nums;
  }

  .pagination-button.is-current {
    border-color: var(--wa-border-focus);
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
    opacity: 1;
  }

  .page-ellipsis {
    min-width: var(--wa-control-h);
    color: var(--wa-text-subtle);
    text-align: center;
  }

  .compact-page-summary {
    display: none;
    color: var(--wa-text-main);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .admin-pagination :is(button, select):focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  @media (hover: hover) {
    .pagination-button:not(:disabled):hover,
    .page-size-control select:not(:disabled):hover {
      border-color: var(--wa-border-strong);
      background: var(--wa-surface-flat);
    }
  }

  .pagination-button:not(:disabled):active {
    transform: translateY(1px);
    background: var(--wa-surface-inset);
  }

  .admin-pagination :is(button, select):disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  @media (max-width: 760px) {
    .admin-pagination {
      align-items: stretch;
      flex-wrap: wrap;
    }

    .pagination-summary {
      flex: 1 0 100%;
    }

    .page-size-control,
    nav {
      min-height: var(--wa-touch-h);
    }

    nav {
      margin-left: auto;
    }

    .page-numbers {
      display: none;
    }

    .compact-page-summary {
      min-height: var(--wa-touch-h);
      display: inline-flex;
      align-items: center;
      padding-inline: var(--wa-space-2);
    }

    .pagination-button.wa-admin-action,
    .page-size-control select {
      min-height: var(--wa-touch-h);
    }
  }

  @media (max-width: 480px) {
    .admin-pagination {
      display: grid;
      grid-template-columns: 1fr;
    }

    .pagination-summary,
    .page-size-control,
    nav {
      width: 100%;
    }

    nav {
      justify-content: space-between;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .admin-pagination *,
    .admin-pagination *::before,
    .admin-pagination *::after {
      transition-duration: 0.001ms !important;
    }
  }
</style>
