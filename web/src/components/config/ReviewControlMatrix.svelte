<script lang="ts">
  import Switch from '../shared/Switch.svelte';

  type Setting = 'review_mrs' | 'review_commits' | 'sync_mrs' | 'sync_commits';
  type Values = Record<Setting, boolean>;
  export let title = '全局';
  export let idPrefix = 'review-controls';
  export let values: Values | null = null;
  export let globalValues: Values | null = null;
  export let disabled = false;
  export let loading = false;
  export let saving: Setting | null = null;
  export let onChange: (field: Setting, checked: boolean) => void = () => {};

  const rows: { name: string; fields: Setting[] }[] = [
    { name: 'MR', fields: ['review_mrs', 'sync_mrs'] },
    { name: 'Commit', fields: ['review_commits', 'sync_commits'] }
  ];
  function globallyPaused(field: Setting) {
    return globalValues?.[field] === false;
  }
  function controlLabel(kind: string, field: Setting) {
    return `${title} ${kind} ${field.startsWith('review') ? '评审' : '评论'}${globallyPaused(field) ? '，全局已暂停' : ''}`;
  }
</script>

<section class="review-control-matrix" aria-label={`${title}评审与评论设置`} aria-busy={loading}>
  <div class="matrix-title">{title}</div>
  <div class="column-title">评审</div>
  <div class="column-title">评论</div>
  {#each rows as row}
    <div class="row-title">{row.name}</div>
    {#each row.fields as field}
      <div class="control-cell">
        <Switch
          id={`${idPrefix}-${field}`}
          label={controlLabel(row.name, field)}
          checked={values?.[field] ?? false}
          optimistic={false}
          expandedHitArea
          saving={saving === field}
          disabled={disabled || loading || !values || Boolean(saving)}
          on:change={(event) => onChange(field, event.detail)}
        />
        {#if saving === field || globallyPaused(field)}
          <span class="cell-state" class:is-paused={globallyPaused(field)} aria-hidden="true">
            {saving === field ? '保存中…' : '全局暂停'}
          </span>
        {/if}
      </div>
    {/each}
  {/each}
</section>

<style>
  .review-control-matrix {
    width: min(100%, 288px);
    min-width: 0;
    display: grid;
    grid-template-columns: 64px repeat(2, minmax(0, 1fr));
    align-items: center;
    gap: 0 12px;
    color: var(--wa-text-main);
  }
  .matrix-title, .column-title, .row-title { font-size: 12px; line-height: 1.5; }
  .matrix-title { color: var(--wa-text-strong); font-weight: 700; }
  .column-title { color: var(--wa-text-muted); text-align: center; }
  .row-title { font-weight: 650; }
  .control-cell {
    min-width: 0;
    min-height: 44px;
    display: flex;
    flex-direction: column;
    align-items: center;
    position: relative;
  }
  .control-cell :global(.switch-container) { min-height: 44px; gap: 0; }
  .control-cell :global(.switch-control) { flex-shrink: 0; }
  .control-cell :global(.switch-label), .control-cell :global(.switch-state) {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .cell-state { min-height: 14px; font-size: 10px; line-height: 14px; color: var(--wa-text-muted); }
  .is-paused { color: var(--wa-text-muted); }
</style>
