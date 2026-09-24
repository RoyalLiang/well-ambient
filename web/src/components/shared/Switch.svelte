<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let checked = false;
  export let label = '';
  export let disabled = false;
  export let id = '';
  export let saving = false;
  export let expandedHitArea = false;

  const dispatch = createEventDispatcher();
  const generatedId = `switch-${Math.random().toString(36).slice(2)}`;
  $: controlId = id || generatedId;

  function toggle() {
    if (!disabled) {
      checked = !checked;
      dispatch('change', checked);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === ' ' || e.key === 'Enter') {
      e.preventDefault();
      toggle();
    }
  }
</script>

<div
  class="switch-container"
  class:disabled={disabled || saving}
  class:expanded-hit-area={expandedHitArea}
  aria-busy={saving}
>
  <button
    id={controlId}
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    disabled={disabled || saving}
    class="switch-control"
    class:checked
    on:click={toggle}
    on:keydown={handleKeydown}
  >
    <span class="switch-thumb" class:checked-thumb={checked}></span>
  </button>
  
  {#if label}
    <label class="switch-label" for={controlId}>{label}</label>
  {/if}
  {#if saving}<span class="switch-state" role="status">保存中…</span>{/if}
</div>

<style>
  .switch-container {
    display: inline-flex;
    align-items: center;
    gap: 12px;
    cursor: default;
    user-select: none;
    margin-bottom: 0;
  }

  .switch-control {
    width: 44px;
    height: 24px;
    border-radius: 9999px;
    background-color: #c8d1d8;
    border: none;
    position: relative;
    padding: 0;
    cursor: pointer;
    transition: background-color 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    outline: none;
  }

  .switch-control:focus-visible {
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.14);
  }

  .switch-control.checked {
    background-color: var(--wa-accent, #008f96);
  }

  .switch-thumb {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background-color: var(--wa-surface-flat, #ffffff);
    position: absolute;
    left: 3px;
    top: 3px;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  }

  .switch-thumb.checked-thumb {
    transform: translateX(20px);
  }

  .expanded-hit-area {
    min-height: 44px;
  }

  .expanded-hit-area .switch-control {
    height: 44px;
    background: transparent;
  }

  .expanded-hit-area .switch-control::before {
    content: '';
    position: absolute;
    inset: 10px 0;
    border-radius: 9999px;
    background-color: #c8d1d8;
    transition: background-color 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .expanded-hit-area .switch-control.checked::before {
    background-color: var(--wa-accent, #008f96);
  }

  .expanded-hit-area .switch-thumb {
    top: 13px;
  }

  .switch-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--wa-text-main, #293847);
    cursor: pointer;
  }

  .switch-state {
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    white-space: nowrap;
  }

  /* Disabled State */
  .disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .disabled .switch-control {
    cursor: not-allowed;
  }
</style>
