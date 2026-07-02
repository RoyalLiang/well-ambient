<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  export let label = '';
  export let value = '';
  export let options: { value: string; label: string }[] = [];
  export let id = '';
  export let required = false;
  export let disabled = false;
  export let error = '';
  export let helperText = '';

  let isOpen = false;
  let selectContainer: HTMLDivElement;

  $: selectedOption = options.find(opt => opt.value === value);
  $: displayLabel = selectedOption ? selectedOption.label : '请选择';

  function toggleDropdown() {
    if (disabled) return;
    isOpen = !isOpen;
  }

  function selectOption(optValue: string) {
    value = optValue;
    isOpen = false;
  }

  function handleClickOutside(event: MouseEvent) {
    if (selectContainer && !selectContainer.contains(event.target as Node)) {
      isOpen = false;
    }
  }

  onMount(() => {
    if (typeof window !== 'undefined') {
      window.addEventListener('click', handleClickOutside);
    }
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('click', handleClickOutside);
    }
  });
</script>

<div class="select-group" class:has-error={!!error} class:disabled bind:this={selectContainer}>
  {#if label}
    <label class="select-label" for={id}>
      {label}
      {#if required}
        <span class="required-star">*</span>
      {/if}
    </label>
  {/if}

  <div class="select-wrapper">
    <button
      {id}
      type="button"
      class="select-trigger"
      class:is-active={isOpen}
      {disabled}
      on:click={toggleDropdown}
      aria-haspopup="listbox"
      aria-expanded={isOpen}
    >
      <span class="select-value-text">{displayLabel}</span>
      <span class="select-chevron" class:rotated={isOpen}>
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </span>
    </button>

    {#if isOpen && !disabled}
      <div class="select-dropdown" role="listbox">
        {#each options as opt}
          <button
            type="button"
            class="dropdown-item"
            class:is-selected={opt.value === value}
            on:click={() => selectOption(opt.value)}
            role="option"
            aria-selected={opt.value === value}
          >
            <span class="option-label">{opt.label}</span>
            {#if opt.value === value}
              <span class="check-icon">
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="20 6 9 17 4 12"></polyline>
                </svg>
              </span>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  </div>

  {#if error}
    <span class="error-text">{error}</span>
  {:else if helperText}
    <span class="helper-text">{helperText}</span>
  {/if}
</div>

<style>
  .select-group {
    display: flex;
    flex-direction: column;
    margin-bottom: 20px;
    width: 100%;
    box-sizing: border-box;
  }

  .select-label {
    font-size: 0.825rem;
    font-weight: 600;
    color: #cbd5e1;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .required-star {
    color: #f87171;
  }

  .select-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
  }

  .select-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: #0b0f19;
    border: 1px solid rgba(51, 65, 85, 0.7);
    border-radius: 8px;
    color: #f1f5f9;
    padding: 10px 14px;
    font-size: 0.9rem;
    cursor: pointer;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-sizing: border-box;
    text-align: left;
  }

  .select-trigger:focus,
  .select-trigger.is-active {
    outline: none;
    border-color: #6366f1;
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
    background: #0f172a;
  }

  .select-trigger:disabled {
    background: #1e293b;
    border-color: #334155;
    color: #64748b;
    cursor: not-allowed;
  }

  .select-value-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-right: 8px;
  }

  .select-chevron {
    color: #64748b;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: transform 0.2s ease;
  }

  .select-chevron.rotated {
    transform: rotate(180deg);
  }

  .select-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    right: 0;
    background: #0b1329;
    border: 1px solid rgba(99, 102, 241, 0.3);
    border-radius: 8px;
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.6);
    z-index: 1000;
    max-height: 240px;
    overflow-y: auto;
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    box-sizing: border-box;
  }

  .dropdown-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: transparent;
    border: none;
    border-radius: 6px;
    padding: 8px 12px;
    font-size: 0.875rem;
    color: #e2e8f0;
    cursor: pointer;
    text-align: left;
    transition: all 0.15s ease;
    box-sizing: border-box;
  }

  .dropdown-item:hover {
    background: rgba(99, 102, 241, 0.12);
    color: #818cf8;
  }

  .dropdown-item.is-selected {
    background: rgba(99, 102, 241, 0.2);
    color: #a5b4fc;
    font-weight: 600;
  }

  .check-icon {
    display: flex;
    align-items: center;
    color: #a5b4fc;
  }

  .error-text {
    color: #f87171;
    font-size: 0.75rem;
    margin-top: 6px;
    font-weight: 500;
  }

  .helper-text {
    color: #64748b;
    font-size: 0.75rem;
    margin-top: 6px;
    line-height: 1.4;
  }

  /* Error States */
  .has-error .select-trigger {
    border-color: rgba(239, 68, 68, 0.6);
  }

  .has-error .select-trigger:focus {
    border-color: #ef4444;
    box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15);
  }
</style>
