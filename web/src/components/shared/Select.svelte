<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';

  interface SelectOption {
    value: string;
    label: string;
    meta?: string;
    disabled?: boolean;
  }

  const dispatch = createEventDispatcher<{ change: string }>();

  export let label = '';
  export let value = '';
  export let options: SelectOption[] = [];
  export let id = '';
  export let required = false;
  export let disabled = false;
  export let error = '';
  export let helperText = '';
  export let placeholder = '请选择';
  export let searchPlaceholder = '搜索选项';
  export let emptyText = '没有匹配选项';
  export let searchable = true;
  export let clearable = false;
  export let compact = false;

  let isOpen = false;
  let searchText = '';
  let controlText = '';
  let selectContainer: HTMLDivElement;
  let searchInput: HTMLInputElement;
  let generatedListboxId = `select-listbox-${Math.random().toString(36).slice(2)}`;
  $: listboxId = id ? `${id}-listbox` : generatedListboxId;

  $: selectedOption = options.find((opt) => opt.value === value);
  $: displayLabel = selectedOption ? selectedOption.label : placeholder;
  $: if (!isOpen) {
    controlText = selectedOption ? selectedOption.label : '';
  }
  $: normalizedSearch = searchText.trim().toLowerCase();
  $: filteredOptions = normalizedSearch
    ? options.filter((opt) => {
        const haystack = `${opt.label} ${opt.value} ${opt.meta || ''}`.toLowerCase();
        return haystack.includes(normalizedSearch);
      })
    : options;

  async function openDropdown() {
    if (disabled) return;
    const wasOpen = isOpen;
    isOpen = true;
    if (searchable && !wasOpen) {
      searchText = '';
      controlText = '';
    }
    await tick();
    if (searchable) searchInput?.focus();
  }

  function closeDropdown() {
    isOpen = false;
    searchText = '';
  }

  function toggleDropdown() {
    if (disabled) return;
    if (isOpen) {
      closeDropdown();
    } else {
      openDropdown();
    }
  }

  function selectOption(option: SelectOption) {
    if (option.disabled) return;
    value = option.value;
    dispatch('change', value);
    closeDropdown();
  }

  function clearSelection(event: MouseEvent) {
    event.stopPropagation();
    value = '';
    controlText = '';
    searchText = '';
    dispatch('change', value);
    closeDropdown();
  }

  function handleSearchInput(event: Event) {
    const nextValue = (event.currentTarget as HTMLInputElement).value;
    controlText = nextValue;
    searchText = nextValue;
    if (!isOpen) {
      isOpen = true;
    }
  }

  function selectFirstFilteredOption() {
    const firstEnabled = filteredOptions.find((option) => !option.disabled);
    if (firstEnabled) selectOption(firstEnabled);
  }

  function handleClickOutside(event: MouseEvent) {
    if (selectContainer && !selectContainer.contains(event.target as Node)) {
      closeDropdown();
    }
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (!isOpen) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      closeDropdown();
    }
  }

  function handleDropdownKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeDropdown();
    }
  }

  function handleInputKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeDropdown();
      searchInput?.blur();
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      if (!isOpen) openDropdown();
      return;
    }
    if (event.key === 'Enter' && isOpen) {
      event.preventDefault();
      selectFirstFilteredOption();
    }
  }

  onMount(() => {
    if (typeof window !== 'undefined') {
      window.addEventListener('click', handleClickOutside);
      window.addEventListener('keydown', handleWindowKeydown);
    }
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('click', handleClickOutside);
      window.removeEventListener('keydown', handleWindowKeydown);
    }
  });
</script>

<div
  class="select-group"
  class:has-error={!!error}
  class:disabled
  class:compact
  bind:this={selectContainer}
>
  {#if label}
    <label class="select-label" for={id}>
      {label}
      {#if required}
        <span class="required-star">*</span>
      {/if}
    </label>
  {/if}

  <div class="select-wrapper">
    {#if searchable}
      <div
        class="select-trigger select-input-shell"
        class:is-active={isOpen}
        class:has-selection={!!selectedOption && !isOpen}
        class:is-disabled={disabled}
      >
        <span class="select-search-icon" aria-hidden="true"></span>
        <input
          {id}
          bind:this={searchInput}
          class="select-inline-input"
          type="search"
          value={controlText}
          placeholder={isOpen ? searchPlaceholder : placeholder}
          autocomplete="off"
          {disabled}
          role="combobox"
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={isOpen}
          aria-haspopup="listbox"
          on:focus={openDropdown}
          on:click|stopPropagation={openDropdown}
          on:input={handleSearchInput}
          on:keydown={handleInputKeydown}
        />
        <span class="select-chevron" class:rotated={isOpen} aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </span>
      </div>
    {:else}
      <button
        {id}
        type="button"
        class="select-trigger"
        class:is-active={isOpen}
        class:has-selection={!!selectedOption}
        {disabled}
        on:click|stopPropagation={toggleDropdown}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-controls={listboxId}
      >
        <span class="select-value-text">{displayLabel}</span>
        <span class="select-chevron" class:rotated={isOpen} aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </span>
      </button>
    {/if}

    {#if clearable && selectedOption && !disabled}
      <button type="button" class="select-clear" aria-label="清除选择" on:click={clearSelection}>×</button>
    {/if}

    {#if isOpen && !disabled}
      <div
        id={listboxId}
        class="select-dropdown"
        role="listbox"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown={handleDropdownKeydown}
      >
        <div class="select-options">
          {#if filteredOptions.length === 0}
            <div class="dropdown-empty">{emptyText}</div>
          {:else}
            {#each filteredOptions as opt}
              <button
                type="button"
                class="dropdown-item"
                class:is-selected={opt.value === value}
                class:is-disabled={opt.disabled}
                on:click={() => selectOption(opt)}
                role="option"
                aria-selected={opt.value === value}
                disabled={opt.disabled}
              >
                <span class="option-copy">
                  <span class="option-label">{opt.label}</span>
                  {#if opt.meta}
                    <small>{opt.meta}</small>
                  {/if}
                </span>
                {#if opt.value === value}
                  <span class="check-icon" aria-hidden="true">
                    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <polyline points="20 6 9 17 4 12"></polyline>
                    </svg>
                  </span>
                {/if}
              </button>
            {/each}
          {/if}
        </div>
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
    position: relative;
    z-index: 1;
    width: 100%;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 7px;
    box-sizing: border-box;
  }

  .select-group:focus-within {
    z-index: 1600;
  }

  .select-group:not(.compact) {
    margin-bottom: 16px;
  }

  .select-label {
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    font-weight: 740;
  }

  .required-star {
    color: var(--wa-danger, #dd4b3e);
  }

  .select-wrapper {
    position: relative;
    width: 100%;
    min-width: 0;
    overflow: visible;
  }

  .select-trigger {
    width: 100%;
    min-width: 0;
    min-height: var(--wa-control-h, 38px);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 0 12px;
    border: 1px solid rgba(123, 143, 160, 0.2);
    border-radius: var(--wa-radius-md, 10px);
    background: rgba(255, 255, 255, 0.72);
    color: var(--wa-text-main, #293847);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78);
    font: inherit;
    font-size: 13px;
    cursor: pointer;
    text-align: left;
    transition:
      border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      background var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }

  .select-trigger:hover,
  .select-trigger:focus-within,
  .select-trigger:focus,
  .select-trigger.is-active {
    outline: none;
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    background: #ffffff !important;
    color: var(--wa-text-main, #293847) !important;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 0 0 3px rgba(0, 143, 150, 0.1);
  }

  .select-input-shell {
    position: relative;
    padding: 0 34px 0 34px;
    cursor: text;
  }

  .select-input-shell.is-disabled,
  .select-trigger:disabled {
    background: rgba(245, 248, 251, 0.72);
    border-color: rgba(123, 143, 160, 0.12);
    color: var(--wa-text-subtle, #8a99aa);
    cursor: not-allowed;
    box-shadow: none;
  }

  .select-search-icon {
    position: absolute;
    left: 12px;
    top: 50%;
    width: 14px;
    height: 14px;
    transform: translateY(-50%);
    background: var(--wa-text-muted, #667789);
    opacity: 0.84;
    pointer-events: none;
    mask: url("data:image/svg+xml,%3Csvg viewBox='0 0 24 24' xmlns='http://www.w3.org/2000/svg'%3E%3Ccircle cx='11' cy='11' r='6.5' fill='none' stroke='black' stroke-width='2'/%3E%3Cpath d='m16 16 4 4' fill='none' stroke='black' stroke-width='2' stroke-linecap='round'/%3E%3C/svg%3E") center / contain no-repeat;
    -webkit-mask: url("data:image/svg+xml,%3Csvg viewBox='0 0 24 24' xmlns='http://www.w3.org/2000/svg'%3E%3Ccircle cx='11' cy='11' r='6.5' fill='none' stroke='black' stroke-width='2'/%3E%3Cpath d='m16 16 4 4' fill='none' stroke='black' stroke-width='2' stroke-linecap='round'/%3E%3C/svg%3E") center / contain no-repeat;
  }

  .select-inline-input {
    width: 100%;
    min-width: 0;
    height: 100%;
    min-height: calc(var(--wa-control-h, 38px) - 2px);
    padding: 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--wa-text-strong, #0d1722);
    font: inherit;
    font-size: 13px;
    font-weight: 660;
  }

  .select-inline-input::placeholder {
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    font-weight: 560;
    opacity: 1;
  }

  .select-inline-input::-webkit-search-decoration,
  .select-inline-input::-webkit-search-cancel-button,
  .select-inline-input::-webkit-search-results-button,
  .select-inline-input::-webkit-search-results-decoration {
    -webkit-appearance: none;
    appearance: none;
  }

  .select-value-text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--wa-text-muted, #667789);
  }

  .select-trigger.has-selection .select-value-text {
    color: var(--wa-text-strong, #0d1722);
    font-weight: 680;
  }

  .select-clear {
    position: absolute;
    top: 50%;
    right: 30px;
    transform: translateY(-50%);
    width: 20px;
    height: 20px;
    display: grid;
    place-items: center;
    border: 0;
    border-radius: var(--wa-radius-sm, 8px);
    background: transparent;
    color: var(--wa-text-muted, #667789);
    cursor: pointer;
    font-size: 16px;
    line-height: 1;
  }

  .select-clear:hover {
    background: rgba(102, 119, 137, 0.1);
    color: var(--wa-text-strong, #0d1722);
  }

  .select-chevron {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--wa-text-muted, #667789);
    transition: transform var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }

  .select-input-shell .select-chevron {
    position: absolute;
    right: 11px;
    top: 50%;
    transform: translateY(-50%);
    pointer-events: none;
  }

  .select-chevron.rotated {
    transform: rotate(180deg);
  }

  .select-input-shell .select-chevron.rotated {
    transform: translateY(-50%) rotate(180deg);
  }

  .select-dropdown {
    position: absolute;
    top: calc(100% + 8px);
    left: 0;
    right: auto;
    z-index: 1800;
    width: max(100%, 280px);
    min-width: 100%;
    max-width: min(340px, calc(100vw - 32px));
    padding: 8px;
    border: 1px solid rgba(123, 143, 160, 0.18);
    border-radius: 8px;
    background: #ffffff !important;
    color-scheme: light;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 22px 54px rgba(26, 41, 58, 0.16);
    backdrop-filter: blur(18px) saturate(126%);
    -webkit-backdrop-filter: blur(18px) saturate(126%);
    box-sizing: border-box;
  }

  .select-options {
    max-height: var(--select-options-max-height, 268px);
    overflow: auto;
    display: grid;
    gap: 2px;
    padding-right: 2px;
    scrollbar-gutter: stable;
  }

  .dropdown-item {
    width: 100%;
    min-height: 38px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 8px 10px;
    border: 0;
    border-radius: var(--wa-radius-md, 7px);
    background: transparent;
    color: var(--wa-text-main, #293847);
    cursor: pointer;
    font: inherit;
    font-size: 13px;
    text-align: left;
    transition:
      background var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      color var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }

  .dropdown-item:hover {
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.09)) !important;
    color: var(--wa-accent-strong, #006f76) !important;
  }

  .dropdown-item.is-selected {
    background: rgba(0, 143, 150, 0.13) !important;
    color: var(--wa-accent-strong, #006f76) !important;
    font-weight: 760;
  }

  .dropdown-item.is-disabled {
    opacity: 0.48;
    cursor: not-allowed;
  }

  .option-copy {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .option-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .option-copy small {
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    line-height: 1.2;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .check-icon {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    color: var(--wa-accent-strong, #006f76);
  }

  .dropdown-empty {
    padding: 16px 10px;
    color: var(--wa-text-muted, #667789);
    font-size: 13px;
    text-align: center;
  }

  .error-text,
  .helper-text {
    font-size: 12px;
    line-height: 1.4;
  }

  .error-text {
    color: var(--wa-danger, #dd4b3e);
    font-weight: 620;
  }

  .helper-text {
    color: var(--wa-text-muted, #667789);
  }

  .has-error .select-trigger {
    border-color: rgba(221, 75, 62, 0.42);
  }

  .has-error .select-trigger:focus,
  .has-error .select-trigger.is-active {
    border-color: var(--wa-danger, #dd4b3e);
    box-shadow: 0 0 0 3px rgba(221, 75, 62, 0.12);
  }
</style>
