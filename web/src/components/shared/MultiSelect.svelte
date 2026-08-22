<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import { focusLeftSelect, registerSelectLifecycle, type SelectCloseReason } from './selectLifecycle';

  interface SelectOption {
    value: string;
    label: string;
    meta?: string;
    disabled?: boolean;
  }

  const dispatch = createEventDispatcher<{ change: string[] }>();

  export let label = '';
  export let values: string[] = [];
  export let options: SelectOption[] = [];
  export let id = '';
  export let disabled = false;
  export let required = false;
  export let placeholder = '请选择';
  export let searchPlaceholder = '原地搜索';
  export let emptyText = '没有匹配选项';
  export let helperText = '';
  export let compact = false;
  export let summaryMode = false;
  export let overlay = false;
  export let controlLabel = '';
  export let ariaLabel = '';
  export let showClear = false;
  export let clearText = '清除筛选';
  export let shadowless = false;

  let isOpen = false;
  let searchText = '';
  let selectContainer: HTMLDivElement;
  let selectWrapper: HTMLDivElement;
  let dropdownEl: HTMLDivElement;
  let searchInput: HTMLInputElement;
  let toggleButton: HTMLButtonElement;
  let dropdownPlacement: 'up' | 'down' = 'down';
  let dropdownTop = -9999;
  let dropdownLeft = 12;
  let dropdownWidth = 280;
  let dropdownMaxHeight = 216;
  let dropdownPositioned = false;
  let toggleTimer: ReturnType<typeof setTimeout> | undefined;
  let unregisterSelectLifecycle: (() => void) | undefined;
  const generatedListboxId = `multi-select-listbox-${Math.random().toString(36).slice(2)}`;
  $: listboxId = id ? `${id}-listbox` : generatedListboxId;
  $: selectedSet = new Set(values);
  $: selectedOptions = values
    .map((value) => options.find((option) => option.value === value))
    .filter((option): option is SelectOption => Boolean(option));
  $: selectionSummary = selectedOptions.length === 0
    ? placeholder
    : selectedOptions.length === 1
      ? selectedOptions[0].label
      : `已选 ${selectedOptions.length} 项`;
  $: normalizedSearch = searchText.trim().toLowerCase();
  $: filteredOptions = normalizedSearch
    ? options.filter((option) => `${option.label} ${option.value} ${option.meta || ''}`.toLowerCase().includes(normalizedSearch))
    : options;

  async function openDropdown() {
    if (disabled) return;
    isOpen = true;
    dropdownPositioned = false;
    await tick();
    updateDropdownPlacement();
    searchInput?.focus();
  }

  function closeDropdown() {
    isOpen = false;
    searchText = '';
    dropdownPositioned = false;
  }

  function updateDropdownPlacement() {
    if (!overlay || !isOpen || !selectWrapper || !dropdownEl || typeof window === 'undefined') return;
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
    const desiredHeight = Math.min(272, dropdownEl.scrollHeight);
    dropdownPlacement = spaceBelow < desiredHeight && spaceAbove > spaceBelow ? 'up' : 'down';
    const availableSpace = dropdownPlacement === 'up' ? spaceAbove : spaceBelow;
    dropdownMaxHeight = Math.max(96, Math.min(216, availableSpace - 56));
    const renderedHeight = Math.min(dropdownEl.scrollHeight, dropdownMaxHeight + 48);
    dropdownTop = dropdownPlacement === 'up'
      ? Math.max(viewportPadding, triggerRect.top - renderedHeight - 8)
      : triggerRect.bottom + 8;
    dropdownPositioned = true;
  }

  function portalDropdown(node: HTMLElement) {
    if (overlay) document.body.appendChild(node);
    return {
      destroy() {
        if (overlay) node.remove();
      }
    };
  }

  function toggleDropdown() {
    if (disabled) return;
    if (isOpen) closeDropdown();
    else void openDropdown();
  }

  function handleTogglePointerDown(event: PointerEvent) {
    event.preventDefault();
    event.stopPropagation();
  }

  function handleToggleClick(event: MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    if (toggleTimer) clearTimeout(toggleTimer);
    toggleTimer = setTimeout(() => {
      toggleTimer = undefined;
      toggleDropdown();
    }, 0);
  }

  function handleInputClick(event: MouseEvent) {
    event.stopPropagation();
    if (isOpen) return;
    if (toggleTimer) clearTimeout(toggleTimer);
    toggleTimer = setTimeout(() => {
      toggleTimer = undefined;
      void openDropdown();
    }, 0);
  }

  async function finishSelection() {
    closeDropdown();
    await tick();
    toggleButton?.focus();
  }

  function commit(nextValues: string[]) {
    values = Array.from(new Set(nextValues));
    dispatch('change', values);
  }

  async function toggleOption(option: SelectOption) {
    if (option.disabled) return;
    if (selectedSet.has(option.value)) {
      commit(values.filter((value) => value !== option.value));
    } else {
      commit([...values, option.value]);
    }
    await tick();
    updateDropdownPlacement();
  }

  function removeValue(value: string, event?: MouseEvent) {
    event?.stopPropagation();
    if (disabled) return;
    commit(values.filter((item) => item !== value));
  }

  function clearSelection() {
    if (disabled || values.length === 0) return;
    commit([]);
  }

  function handleRemovePointerDown(event: PointerEvent) {
    event.preventDefault();
    event.stopPropagation();
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
      void openDropdown();
      return;
    }
    if (event.key === 'Enter' && isOpen) {
      const firstEnabled = filteredOptions.find((option) => !option.disabled);
      if (firstEnabled) {
        event.preventDefault();
        toggleOption(firstEnabled);
      }
      return;
    }
    if (event.key === 'Backspace' && !searchText && values.length) {
      removeValue(values[values.length - 1]);
    }
  }

  function handleFocusOut(event: FocusEvent) {
    if (isOpen && selectContainer && focusLeftSelect(selectContainer, event, [dropdownEl])) closeDropdown();
  }

  function handleLifecycleClose(reason: SelectCloseReason) {
    if (reason === 'escape') void finishSelection();
    else closeDropdown();
  }

  function handleDropdownKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      void finishSelection();
    }
  }

  onMount(() => {
    unregisterSelectLifecycle = registerSelectLifecycle(selectContainer, {
      isOpen: () => isOpen,
      close: handleLifecycleClose,
      ownsTarget: (target) => !!dropdownEl?.contains(target)
    });
    window.addEventListener('resize', updateDropdownPlacement);
    window.addEventListener('scroll', updateDropdownPlacement, true);
  });

  onDestroy(() => {
    if (toggleTimer) clearTimeout(toggleTimer);
    unregisterSelectLifecycle?.();
    window.removeEventListener('resize', updateDropdownPlacement);
    window.removeEventListener('scroll', updateDropdownPlacement, true);
  });
</script>

<div class="multi-select-group" class:disabled class:compact class:summary-mode={summaryMode} class:overlay class:shadowless bind:this={selectContainer} on:focusout={handleFocusOut}>
  {#if label}
    <label class="multi-select-label" for={id}>
      {label}{#if required}<span class="required-star">*</span>{/if}
    </label>
  {/if}

  <div class="multi-select-wrapper" bind:this={selectWrapper}>
    <div
      class="multi-select-trigger"
      class:is-active={isOpen}
      class:has-selection={selectedOptions.length > 0}
      class:is-disabled={disabled}
      role="presentation"
    >
      {#if summaryMode}
        {#if controlLabel}<span class="multi-select-prefix">{controlLabel}</span>{/if}
        {#if !isOpen}<span class="multi-select-summary" title={selectionSummary}>{selectionSummary}</span>{/if}
      {:else}
        {#each selectedOptions as option (option.value)}
          <span class="selection-chip">
            <span>{option.label}</span>
            {#if !disabled}
              <button
                type="button"
                aria-label={`移除 ${option.label}`}
                on:pointerdown={handleRemovePointerDown}
                on:click={(event) => removeValue(option.value, event)}
              >×</button>
            {/if}
          </span>
        {/each}
      {/if}
      <input
        {id}
        bind:this={searchInput}
        bind:value={searchText}
        type="search"
        placeholder={summaryMode ? (isOpen ? searchPlaceholder : '') : (selectedOptions.length ? searchPlaceholder : placeholder)}
        autocomplete="off"
        {disabled}
        aria-label={ariaLabel || controlLabel || label || placeholder}
        role="combobox"
        aria-autocomplete="list"
        aria-controls={listboxId}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        on:click={handleInputClick}
        on:input={openDropdown}
        on:keydown={handleInputKeydown}
      />
      <button
        type="button"
        class="multi-select-chevron"
        class:rotated={isOpen}
        bind:this={toggleButton}
        disabled={disabled}
        aria-label={isOpen ? '关闭选项' : '展开选项'}
        aria-controls={listboxId}
        aria-expanded={isOpen}
        on:pointerdown={handleTogglePointerDown}
        on:click={handleToggleClick}
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </button>
    </div>

    {#if isOpen && !disabled}
      <div
        class="multi-select-dropdown"
        class:shadowless
        class:is-overlay={overlay}
        class:is-positioned={dropdownPositioned}
        class:drop-up={dropdownPlacement === 'up'}
        bind:this={dropdownEl}
        use:portalDropdown
        style={overlay ? `--multi-select-options-max-height: ${dropdownMaxHeight}px; top: ${dropdownTop}px; left: ${dropdownLeft}px; width: ${dropdownWidth}px;` : ''}
      >
        <div
          id={listboxId}
          class="multi-select-options"
          role="listbox"
          tabindex="-1"
          aria-multiselectable="true"
          on:keydown={handleDropdownKeydown}
        >
          {#if filteredOptions.length === 0}
            <div class="dropdown-empty">{emptyText}</div>
          {:else}
            {#each filteredOptions as option (option.value)}
              <button
                type="button"
                class="dropdown-item"
                class:is-selected={selectedSet.has(option.value)}
                disabled={option.disabled}
                role="option"
                aria-selected={selectedSet.has(option.value)}
                on:click={() => toggleOption(option)}
              >
                <span class="option-check" aria-hidden="true">
                  {#if selectedSet.has(option.value)}
                    <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.7" stroke-linecap="round" stroke-linejoin="round">
                      <polyline points="20 6 9 17 4 12"></polyline>
                    </svg>
                  {/if}
                </span>
                <span class="option-copy"><strong>{option.label}</strong>{#if option.meta}<small>{option.meta}</small>{/if}</span>
              </button>
            {/each}
          {/if}
        </div>
        <div class="multi-select-footer">
          <span aria-live="polite">已选 {values.length} 项</span>
          <div class="multi-select-footer-actions">
            {#if showClear && values.length > 0}
              <button type="button" class="multi-select-clear" on:click={clearSelection}>{clearText}</button>
            {/if}
            <button type="button" class="multi-select-done" on:click={finishSelection}>完成选择</button>
          </div>
        </div>
      </div>
    {/if}
  </div>

  {#if helperText}<span class="helper-text">{helperText}</span>{/if}
</div>

<style>
  .multi-select-group { position: relative; z-index: 1; width: 100%; min-width: 0; display: flex; flex-direction: column; gap: 7px; box-sizing: border-box; }
  .multi-select-group:focus-within { z-index: 2; }
  .multi-select-group:not(.compact) { margin-bottom: 16px; }
  .multi-select-label { display: flex; align-items: center; gap: 4px; color: var(--wa-text-muted, #667789); font-size: 12px; font-weight: 740; }
  .required-star { color: var(--wa-danger, #dd4b3e); }
  .multi-select-wrapper { position: relative; width: 100%; min-width: 0; overflow: visible; }
  .multi-select-trigger { position: relative; width: 100%; min-width: 0; min-height: var(--wa-control-h, 38px); display: flex; align-items: center; flex-wrap: wrap; gap: 6px; box-sizing: border-box; padding: 5px 36px 5px 8px; border: 1px solid rgba(123, 143, 160, .2); border-radius: var(--wa-radius-md, 10px); background: rgba(255, 255, 255, .72); box-shadow: inset 0 1px 0 rgba(255, 255, 255, .78); cursor: text; transition: border-color 140ms ease, background 140ms ease, box-shadow 140ms ease; }
  .multi-select-trigger:hover, .multi-select-trigger:focus-within, .multi-select-trigger.is-active { border-color: var(--wa-border-focus, rgba(0, 143, 150, .86)); background: var(--wa-surface-flat, #fbfdfe); box-shadow: inset 0 1px 0 rgba(255,255,255,.88), 0 0 0 3px rgba(0,143,150,.1); }
  .multi-select-group.shadowless .multi-select-trigger, .multi-select-group.shadowless .multi-select-trigger:hover, .multi-select-group.shadowless .multi-select-trigger:focus-within, .multi-select-group.shadowless .multi-select-trigger.is-active, .multi-select-dropdown.shadowless { box-shadow: none; }
  .multi-select-trigger.is-disabled { border-color: rgba(123, 143, 160, .12); background: rgba(245, 248, 251, .72); box-shadow: none; cursor: not-allowed; }
  .multi-select-trigger input { min-width: 116px; flex: 1 1 132px; height: 26px; padding: 0; border: 0; outline: 0; background: transparent; color: var(--wa-text-strong, #0d1722); font: inherit; font-size: 12px; }
  .multi-select-trigger input::placeholder { color: var(--wa-text-muted, #667789); opacity: 1; }
  .multi-select-trigger input::-webkit-search-decoration, .multi-select-trigger input::-webkit-search-cancel-button { appearance: none; }
  .multi-select-group.summary-mode .multi-select-trigger { height: var(--wa-control-h, 38px); min-height: var(--wa-control-h, 38px); flex-wrap: nowrap; gap: 8px; padding: 0 36px 0 10px; cursor: pointer; }
  .multi-select-group.summary-mode .multi-select-trigger input { min-width: 0; flex: 1 1 auto; }
  .multi-select-group.summary-mode .multi-select-trigger:not(.is-active) input { position: absolute; inset: 0 32px 0 0; width: auto; height: 100%; opacity: 0; cursor: pointer; }
  .multi-select-prefix { flex: none; color: var(--wa-text-muted, #667789); font-size: 11px; font-weight: 760; white-space: nowrap; }
  .multi-select-summary { min-width: 0; flex: 1 1 auto; overflow: hidden; color: var(--wa-text-main, #293847); font-size: 12px; font-weight: 700; line-height: 1.2; text-overflow: ellipsis; white-space: nowrap; }
  .selection-chip { min-width: 0; max-width: 100%; height: 26px; display: inline-flex; align-items: center; gap: 3px; border: 1px solid #c5e0dc; border-radius: 7px; padding: 0 5px 0 8px; background: #edf7f5; color: #176f66; font-size: 11px; font-weight: 700; }
  .selection-chip > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .selection-chip button { width: 18px; height: 18px; display: grid; place-items: center; border: 0; border-radius: 5px; padding: 0; background: transparent; color: #4e817a; font: 700 15px/1 sans-serif; cursor: pointer; }
  .selection-chip button:hover { background: rgba(23, 111, 102, .1); color: #176f66; }
  .multi-select-chevron { position: absolute; top: 50%; right: 5px; width: 26px; height: 26px; display: grid; place-items: center; border: 0; border-radius: 6px; padding: 0; background: transparent; color: var(--wa-text-muted, #667789); transform: translateY(-50%); transition: transform 140ms ease; cursor: pointer; }
  .multi-select-chevron:hover, .multi-select-chevron:focus-visible { outline: none; background: rgba(102, 119, 137, .1); color: var(--wa-text-strong, #0d1722); }
  .multi-select-chevron.rotated { transform: translateY(-50%) rotate(180deg); }
  .multi-select-dropdown { position: relative; overflow: hidden; margin-top: 7px; border: 1px solid rgba(123, 143, 160, .24); border-radius: var(--wa-radius-md, 10px); background: rgba(255,255,255,.98); }
  .multi-select-dropdown.is-overlay { position: fixed; z-index: 1200; min-width: min(var(--multi-select-dropdown-min, 280px), calc(100vw - 24px)); margin-top: 0; visibility: hidden; box-shadow: var(--wa-shadow-md, 0 14px 34px rgba(30, 46, 64, .085)); }
  .multi-select-dropdown.is-overlay.is-positioned { visibility: visible; }
  .multi-select-options { overflow: auto; max-height: var(--multi-select-options-max-height, 216px); padding: 5px; overscroll-behavior: contain; scrollbar-width: none; -ms-overflow-style: none; }
  .multi-select-options::-webkit-scrollbar { display: none; width: 0; height: 0; }
  .multi-select-footer { min-height: 40px; display: flex; align-items: center; justify-content: space-between; gap: 12px; border-top: 1px solid rgba(123, 143, 160, .16); padding: 6px 7px 6px 12px; background: #f7fafb; }
  .multi-select-footer > span { color: var(--wa-text-muted, #667789); font-size: 10px; font-weight: 650; }
  .multi-select-footer-actions { display: flex; align-items: center; gap: 5px; }
  .multi-select-clear { min-height: 30px; border: 0; border-radius: 7px; padding: 0 9px; background: transparent; color: var(--wa-text-muted, #667789); font-family: inherit; font-size: 11px; font-weight: 700; cursor: pointer; }
  .multi-select-clear:hover { background: rgba(102, 119, 137, .09); color: var(--wa-text-main, #293847); }
  .multi-select-clear:focus-visible { outline: 3px solid rgba(38, 139, 127, .16); outline-offset: 1px; }
  .multi-select-done { min-height: 30px; border: 1px solid #b9d9d4; border-radius: 7px; padding: 0 10px; background: #edf7f5; color: #176f66; font-family: inherit; font-size: 11px; font-weight: 750; cursor: pointer; }
  .multi-select-done:hover { border-color: #8fc5bd; background: #e2f2ef; }
  .multi-select-done:focus-visible { outline: 3px solid rgba(38, 139, 127, .16); outline-offset: 1px; }
  .dropdown-item { width: 100%; min-height: 42px; display: grid; grid-template-columns: 22px minmax(0, 1fr); align-items: center; gap: 8px; border: 0; border-radius: 7px; padding: 7px 9px; background: transparent; color: var(--wa-text-main, #293847); text-align: left; cursor: pointer; }
  .dropdown-item:hover, .dropdown-item.is-selected { background: #f0f8f6; color: #176f66; }
  .dropdown-item:disabled { opacity: .45; cursor: not-allowed; }
  .option-check { width: 18px; height: 18px; display: grid; place-items: center; border: 1px solid #c6d3d9; border-radius: 5px; color: #176f66; }
  .is-selected .option-check { border-color: #7dbeb5; background: #dff1ed; }
  .option-copy { min-width: 0; display: grid; gap: 2px; }
  .option-copy strong { color: inherit; font-size: 12px; line-height: 1.3; white-space: normal; overflow-wrap: anywhere; }
  .option-copy small { color: var(--wa-text-muted, #667789); font-size: 10px; line-height: 1.3; white-space: normal; overflow-wrap: anywhere; }
  .dropdown-empty { padding: 16px 10px; color: var(--wa-text-muted, #667789); font-size: 12px; text-align: center; }
  .helper-text { color: var(--wa-text-muted, #667789); font-size: 10px; line-height: 1.4; }
  @media (max-width: 560px) {
    .multi-select-trigger input { min-width: 88px; }
    .multi-select-options { max-height: 192px; }
    .multi-select-chevron { right: 2px; width: 40px; height: 40px; }
    .multi-select-footer { min-height: 52px; }
    .dropdown-item { min-height: 44px; }
    .multi-select-done,
    .multi-select-clear { min-height: 44px; }
  }
</style>
