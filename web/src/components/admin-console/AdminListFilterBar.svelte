<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    label: string;
    leading?: Snippet;
    controls: Snippet;
    meta?: Snippet;
    className?: string;
  }

  let {
    label,
    leading,
    controls,
    meta,
    className = ''
  }: Props = $props();
</script>

<section
  class="admin-list-filter-bar {className}"
  data-component="AdminListFilterBar"
  data-style-contract="admin-list-filter-bar/v1"
  data-surface-contract="admin-data-list-surface/v1"
  aria-label={label}
>
  {#if leading}
    <div class="filter-leading">
      {@render leading()}
    </div>
  {/if}

  <div class="filter-controls">
    {@render controls()}
  </div>

  {#if meta}
    <div class="filter-meta">
      {@render meta()}
    </div>
  {/if}
</section>

<style>
  /* finesse · component: list-filter-bar · register=product
   * ownership: page-level filters, actions and result metadata only */
  .admin-list-filter-bar {
    position: relative;
    z-index: 3;
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, auto) minmax(220px, 1fr);
    align-items: center;
    gap: var(--wa-space-2) var(--wa-space-4);
    padding: var(--wa-space-3) var(--wa-space-4);
    border: 1px solid var(--wa-admin-list-surface-border);
    border-top-color: var(--wa-admin-list-surface-border-top);
    border-radius: var(--wa-admin-list-surface-radius);
    background: var(--wa-admin-list-surface-background);
    box-shadow: var(--wa-admin-list-surface-shadow);
    -webkit-backdrop-filter: var(--wa-admin-list-surface-filter);
    backdrop-filter: var(--wa-admin-list-surface-filter);
  }

  .filter-leading,
  .filter-controls,
  .filter-meta {
    min-width: 0;
  }

  .filter-controls {
    justify-self: stretch;
  }

  .filter-meta {
    grid-column: 1 / -1;
    padding-top: var(--wa-space-2);
    border-top: 1px solid var(--wa-border-divider);
  }

  @media (max-width: 900px) {
    .admin-list-filter-bar {
      grid-template-columns: minmax(0, 1fr);
      align-items: stretch;
      gap: var(--wa-space-2);
    }

    .filter-meta {
      grid-column: 1;
    }
  }

  @media (max-width: 640px) {
    .admin-list-filter-bar {
      padding: var(--wa-space-3);
      border-radius: var(--wa-admin-list-surface-radius);
    }
  }

  @media (prefers-reduced-transparency: reduce) {
    .admin-list-filter-bar {
      background: var(--wa-admin-list-surface-solid-background);
      -webkit-backdrop-filter: none;
      backdrop-filter: none;
    }
  }
</style>
