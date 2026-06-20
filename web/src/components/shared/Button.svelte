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
    font-size: 0.85rem;
    font-weight: 600;
    padding: 10px 18px;
    border-radius: 8px;
    border: 1px solid transparent;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    outline: none;
    position: relative;
    user-select: none;
    white-space: nowrap;
  }

  .btn:active {
    transform: scale(0.98);
  }

  .btn:focus-visible {
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.4);
  }

  .btn-small {
    gap: 6px;
    padding: 7px 11px;
    border-radius: 7px;
    font-size: 0.76rem;
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
    background: linear-gradient(135deg, #4f46e5 0%, #6366f1 100%);
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(79, 70, 229, 0.2);
  }

  .btn-primary:hover:not(:disabled) {
    background: linear-gradient(135deg, #4338ca 0%, #4f46e5 100%);
    box-shadow: 0 6px 16px rgba(79, 70, 229, 0.3), 0 0 8px rgba(99, 102, 241, 0.2);
  }

  .btn-secondary {
    background: rgba(30, 41, 59, 0.6);
    border-color: rgba(51, 65, 85, 0.8);
    color: #cbd5e1;
  }

  .btn-secondary:hover:not(:disabled) {
    background: rgba(51, 65, 85, 0.5);
    border-color: rgba(100, 116, 139, 0.8);
    color: #f1f5f9;
  }

  .btn-danger {
    background: rgba(239, 68, 68, 0.15);
    border-color: rgba(239, 68, 68, 0.3);
    color: #f87171;
  }

  .btn-danger:hover:not(:disabled) {
    background: #ef4444;
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(239, 68, 68, 0.2);
  }

  .btn-ghost {
    background: transparent;
    color: #94a3b8;
  }

  .btn-ghost:hover:not(:disabled) {
    background: rgba(51, 65, 85, 0.3);
    color: #cbd5e1;
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
