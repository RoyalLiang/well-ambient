<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import { focusLeftSelect, registerSelectLifecycle, type SelectCloseReason } from './selectLifecycle';

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
  export let ariaLabel = '';
  export let shadowless = false;

  let isOpen = false;
  let searchText = '';
  let controlText = '';
  let selectContainer: HTMLDivElement;
  let selectWrapper: HTMLDivElement;
  let dropdownEl: HTMLDivElement;
  let searchInput: HTMLInputElement;
  let dropdownPlacement: 'up' | 'down' = 'down';
  let dropdownMaxHeight = 268;
  let dropdownTop = -9999;
  let dropdownLeft = 12;
  let dropdownWidth = 280;
  let dropdownPositioned = false;
  let unregisterSelectLifecycle: (() => void) | undefined;
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
    dropdownPositioned = false;
    await tick();
    updateDropdownPlacement();
    if (searchable) searchInput?.focus();
  }

  function updateDropdownPlacement() {
    if (!isOpen || !selectWrapper || !dropdownEl || typeof window === 'undefined') return;
    const viewportPadding = 12;
    const triggerRect = selectWrapper.getBoundingClientRect();
    const maximumWidth = Math.max(0, window.innerWidth - viewportPadding * 2);
    dropdownWidth = Math.min(Math.max(triggerRect.width, 280), Math.min(520, maximumWidth));
    dropdownLeft = Math.min(
      Math.max(viewportPadding, triggerRect.left),
      Math.max(viewportPadding, window.innerWidth - viewportPadding - dropdownWidth)
    );
    const spaceBelow = window.innerHeight - triggerRect.bottom - viewportPadding;
    const spaceAbove = triggerRect.top - viewportPadding;
    const desiredHeight = Math.min(284, dropdownEl.scrollHeight);
    dropdownPlacement = spaceBelow < desiredHeight && spaceAbove > spaceBelow ? 'up' : 'down';
    const availableSpace = dropdownPlacement === 'up' ? spaceAbove : spaceBelow;
    dropdownMaxHeight = Math.max(96, Math.min(268, availableSpace - 18));
    const renderedHeight = Math.min(dropdownEl.scrollHeight, dropdownMaxHeight + 18);
    dropdownTop =
      dropdownPlacement === 'up'
        ? Math.max(viewportPadding, triggerRect.top - renderedHeight - 8)
        : triggerRect.bottom + 8;
    dropdownPositioned = true;
  }

  function portalDropdown(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  function closeDropdown() {
    isOpen = false;
    searchText = '';
    dropdownPositioned = false;
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

  function handleFocusOut(event: FocusEvent) {
    if (isOpen && selectContainer && focusLeftSelect(selectContainer, event, [dropdownEl])) closeDropdown();
  }

  function handleLifecycleClose(reason: SelectCloseReason) {
    closeDropdown();
    if (reason === 'escape') searchInput?.blur();
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
      unregisterSelectLifecycle = registerSelectLifecycle(selectContainer, {
        isOpen: () => isOpen,
        close: handleLifecycleClose,
        ownsTarget: (target) => !!dropdownEl?.contains(target)
      });
      window.addEventListener('resize', updateDropdownPlacement);
      window.addEventListener('scroll', updateDropdownPlacement, true);
    }
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      unregisterSelectLifecycle?.();
      window.removeEventListener('resize', updateDropdownPlacement);
      window.removeEventListener('scroll', updateDropdownPlacement, true);
    }
  });
</script>

<div
  class="select-group"
  class:has-error={!!error}
  class:disabled
  class:compact
  class:shadowless
  bind:this={selectContainer}
  on:focusout={handleFocusOut}
>
  {#if label}
    <label class="select-label" for={id}>
      {label}
      {#if required}
        <span class="required-star">*</span>
      {/if}
    </label>
  {/if}

  <div class="select-wrapper" bind:this={selectWrapper}>
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
          aria-label={ariaLabel || label || placeholder}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={isOpen}
          aria-haspopup="listbox"
          on:click|stopPropagation={openDropdown}
          on:input={handleSearchInput}
          on:keydown={handleInputKeydown}
        />
        <button
          type="button"
          class="select-chevron select-toggle"
          class:rotated={isOpen}
          disabled={disabled}
          aria-label={isOpen ? '关闭选项' : '展开选项'}
          aria-controls={listboxId}
          aria-expanded={isOpen}
          on:click|stopPropagation={toggleDropdown}
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </button>
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
        aria-label={ariaLabel || label || placeholder}
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

    {#if clearable && selectedOption && value !== '' && !disabled}
      <button type="button" class="select-clear" aria-label="清除选择" on:click={clearSelection}>×</button>
    {/if}

    {#if isOpen && !disabled}
      <div
        id={listboxId}
        class="select-dropdown"
        class:shadowless
        class:drop-up={dropdownPlacement === 'up'}
        class:is-positioned={dropdownPositioned}
        bind:this={dropdownEl}
        use:portalDropdown
        style={`--select-options-max-height: ${dropdownMaxHeight}px; top: ${dropdownTop}px; left: ${dropdownLeft}px; width: ${dropdownWidth}px;`}
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

  .select-group.shadowless .select-trigger,
  .select-group.shadowless .select-trigger:hover,
  .select-group.shadowless .select-trigger:focus-within,
  .select-group.shadowless .select-trigger:focus,
  .select-group.shadowless .select-trigger.is-active,
  .select-dropdown.shadowless {
    box-shadow: none;
  }

  .select-dropdown.shadowless {
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
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
    border-radius: var(--wa-radius-sm, 8px);
    outline: 0;
    background: transparent;
    color: var(--wa-text-strong, #0d1722);
    font: inherit;
    font-size: 13px;
    font-weight: 660;
  }

  .select-inline-input:focus,
  .select-inline-input:focus-visible {
    border-color: transparent;
    outline: none !important;
    outline-offset: 0;
  }

  .select-input-shell:has(.select-inline-input:focus-visible) {
    outline: 2px solid var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    outline-offset: 2px;
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
    right: 5px;
    top: 50%;
    transform: translateY(-50%);
  }

  .select-toggle {
    width: 26px;
    height: 26px;
    border: 0;
    border-radius: 6px;
    padding: 0;
    background: transparent;
    cursor: pointer;
  }

  .select-toggle:hover,
  .select-toggle:focus-visible {
    outline: none;
    background: rgba(102, 119, 137, .1);
    color: var(--wa-text-strong, #0d1722);
  }

  .select-chevron.rotated {
    transform: rotate(180deg);
  }

  .select-input-shell .select-chevron.rotated {
    transform: translateY(-50%) rotate(180deg);
  }

  .select-dropdown {
    position: fixed;
    z-index: 1050;
    min-width: 0;
    max-width: calc(100vw - 24px);
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
    visibility: hidden;
  }

  .select-dropdown.is-positioned {
    visibility: visible;
  }

  .select-options {
    max-height: var(--select-options-max-height, 268px);
    overflow: auto;
    display: grid;
    gap: 2px;
    padding-right: 2px;
    scrollbar-width: none;
    -ms-overflow-style: none;
  }

  .select-options::-webkit-scrollbar { display: none; width: 0; height: 0; }

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
    white-space: normal;
    overflow-wrap: anywhere;
    line-height: 1.35;
  }

  .option-copy small {
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    line-height: 1.2;
    white-space: normal;
    overflow-wrap: anywhere;
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
