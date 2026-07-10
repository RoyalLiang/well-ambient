<script lang="ts">
  export let variant: 'primary' | 'secondary' | 'danger' | 'ghost' = 'primary';
  export let size: 'small' | 'medium' = 'medium';
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
  export let loading = false;
</script>

<button
  {type}
  class="btn btn-{variant}"
  class:btn-small={size === 'small'}
  class:loading
  disabled={disabled || loading}
  on:click
>
  {#if loading}
    <span class="spinner" aria-hidden="true"></span>
  {/if}
  <span class="btn-content" class:loading-text={loading}>
    <slot />
  </span>
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: var(--wa-control-h, 36px);
    font-size: 13px;
    font-weight: 760;
    padding: 0 14px;
    border-radius: var(--wa-radius-md, 7px);
    border: 1px solid transparent;
    cursor: pointer;
    transition: background var(--wa-duration-fast, 140ms) var(--wa-ease, ease), border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease), box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
    outline: none;
    position: relative;
    user-select: none;
    white-space: nowrap;
  }

  .btn:active {
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .btn:focus-visible {
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.16);
  }

  .btn-small {
    gap: 6px;
    min-height: 30px;
    padding: 0 10px;
    border-radius: var(--wa-radius-sm, 6px);
    font-size: 12px;
  }

  .btn-content {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    transition: opacity 0.2s;
  }

  .loading-text {
    opacity: 0.7;
  }

  /* Variants */
  .btn-primary {
    border-color: var(--wa-accent, #008f96);
    background: var(--wa-accent, #008f96);
    color: var(--wa-accent-ink, #ffffff);
    box-shadow: 0 10px 24px rgba(0, 143, 150, 0.16);
  }

  .btn-primary:hover:not(:disabled) {
    border-color: var(--wa-accent-strong, #006f76);
    background: var(--wa-accent-strong, #006f76);
    box-shadow: 0 12px 26px rgba(0, 143, 150, 0.18);
  }

  .btn-secondary {
    background: rgba(255, 255, 255, 0.76);
    border-color: var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    color: var(--wa-text-main, #293847);
  }

  .btn-secondary:hover:not(:disabled) {
    background: #ffffff;
    border-color: var(--wa-border-strong, rgba(85, 106, 128, 0.32));
    color: var(--wa-text-strong, #0d1722);
  }

  .btn-danger {
    background: var(--wa-danger-soft, rgba(221, 75, 62, 0.12));
    border-color: rgba(221, 75, 62, 0.24);
    color: var(--wa-danger, #dd4b3e);
  }

  .btn-danger:hover:not(:disabled) {
    background: var(--wa-danger, #dd4b3e);
    color: #ffffff;
    box-shadow: 0 10px 24px rgba(221, 75, 62, 0.16);
  }

  .btn-ghost {
    background: transparent;
    color: var(--wa-text-muted, #667789);
  }

  .btn-ghost:hover:not(:disabled) {
    background: var(--wa-neutral-soft, rgba(102, 119, 137, 0.1));
    color: var(--wa-text-strong, #0d1722);
  }

  /* Disabled State */
  .btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    box-shadow: none !important;
  }

  /* Spinner */
  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid currentColor;
    border-bottom-color: transparent;
    border-radius: 50%;
    display: inline-block;
    box-sizing: border-box;
    animation: rotation 0.6s linear infinite;
  }

  @keyframes rotation {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
  }
</style>
