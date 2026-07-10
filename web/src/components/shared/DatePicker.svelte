<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';

  const dispatch = createEventDispatcher<{ change: string }>();

  export let label = '';
  export let value = '';
  export let id = '';
  export let required = false;
  export let disabled = false;
  export let error = '';
  export let helperText = '';
  export let placeholder = '选择日期';
  export let clearable = true;
  export let compact = false;
  export let min = '';
  export let max = '';

  const weekdayNames = ['一', '二', '三', '四', '五', '六', '日'];

  interface CalendarDay {
    value: string;
    label: number;
    muted: boolean;
    today: boolean;
    selected: boolean;
    disabled: boolean;
  }

  let isOpen = false;
  let containerEl: HTMLDivElement;
  let cursorDate = parseDate(value) || new Date();

  $: selectedDate = parseDate(value);
  $: displayLabel = selectedDate ? formatDateLabel(selectedDate) : placeholder;
  $: calendar = buildCalendar(cursorDate, value);

  function parseDate(dateValue: string): Date | null {
    if (!dateValue) return null;
    const [year, month, day] = dateValue.split('-').map(Number);
    if (!year || !month || !day) return null;
    const date = new Date(year, month - 1, day);
    return Number.isNaN(date.getTime()) ? null : date;
  }

  function toDateValue(date: Date): string {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  function formatDateLabel(date: Date): string {
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit'
    });
  }

  function isDateDisabled(dateValue: string): boolean {
    if (min && dateValue < min) return true;
    if (max && dateValue > max) return true;
    return false;
  }

  function buildCalendar(cursor: Date, selectedValue: string) {
    const year = cursor.getFullYear();
    const month = cursor.getMonth();
    const first = new Date(year, month, 1);
    const last = new Date(year, month + 1, 0);
    const leading = (first.getDay() + 6) % 7;
    const todayValue = toDateValue(new Date());
    const days: CalendarDay[] = [];

    for (let i = leading - 1; i >= 0; i -= 1) {
      const date = new Date(year, month, -i);
      const dateValue = toDateValue(date);
      days.push({
        value: dateValue,
        label: date.getDate(),
        muted: true,
        today: dateValue === todayValue,
        selected: dateValue === selectedValue,
        disabled: isDateDisabled(dateValue)
      });
    }

    for (let day = 1; day <= last.getDate(); day += 1) {
      const date = new Date(year, month, day);
      const dateValue = toDateValue(date);
      days.push({
        value: dateValue,
        label: day,
        muted: false,
        today: dateValue === todayValue,
        selected: dateValue === selectedValue,
        disabled: isDateDisabled(dateValue)
      });
    }

    while (days.length % 7 !== 0) {
      const date = new Date(year, month, days.length - leading + 1);
      const dateValue = toDateValue(date);
      days.push({
        value: dateValue,
        label: date.getDate(),
        muted: true,
        today: dateValue === todayValue,
        selected: dateValue === selectedValue,
        disabled: isDateDisabled(dateValue)
      });
    }

    return {
      title: `${year}年 ${month + 1}月`,
      days
    };
  }

  function openPicker() {
    if (disabled) return;
    cursorDate = selectedDate || new Date();
    isOpen = true;
  }

  function closePicker() {
    isOpen = false;
  }

  function togglePicker() {
    if (isOpen) {
      closePicker();
    } else {
      openPicker();
    }
  }

  function moveMonth(delta: number) {
    cursorDate = new Date(cursorDate.getFullYear(), cursorDate.getMonth() + delta, 1);
  }

  function selectDay(day: CalendarDay) {
    if (day.disabled) return;
    value = day.value;
    dispatch('change', value);
    closePicker();
  }

  function clearValue(event: MouseEvent) {
    event.stopPropagation();
    value = '';
    dispatch('change', value);
    closePicker();
  }

  function chooseToday() {
    const todayValue = toDateValue(new Date());
    if (isDateDisabled(todayValue)) return;
    value = todayValue;
    cursorDate = new Date();
    dispatch('change', value);
    closePicker();
  }

  function handleClickOutside(event: MouseEvent) {
    if (containerEl && !containerEl.contains(event.target as Node)) {
      closePicker();
    }
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (!isOpen) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      closePicker();
    }
  }

  function handlePanelKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closePicker();
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

<div class="date-group" class:has-error={!!error} class:disabled class:compact bind:this={containerEl}>
  {#if label}
    <label class="date-label" for={id}>
      {label}
      {#if required}
        <span class="required-star">*</span>
      {/if}
    </label>
  {/if}

  <div class="date-wrapper">
    <button
      {id}
      type="button"
      class="date-trigger"
      class:is-active={isOpen}
      class:has-value={!!value}
      {disabled}
      on:click|stopPropagation={togglePicker}
      aria-haspopup="dialog"
      aria-expanded={isOpen}
    >
      <span class="date-icon" aria-hidden="true"></span>
      <span class="date-value">{displayLabel}</span>
    </button>

    {#if clearable && value && !disabled}
      <button type="button" class="date-clear" aria-label="清除日期" on:click={clearValue}>×</button>
    {/if}

    {#if isOpen && !disabled}
      <div
        class="date-picker-panel"
        role="dialog"
        aria-label="选择日期"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown={handlePanelKeydown}
      >
        <div class="date-picker-head">
          <button type="button" on:click={() => moveMonth(-1)} aria-label="上个月">‹</button>
          <strong>{calendar.title}</strong>
          <button type="button" on:click={() => moveMonth(1)} aria-label="下个月">›</button>
        </div>

        <div class="date-week-grid" aria-hidden="true">
          {#each weekdayNames as weekday}
            <span>{weekday}</span>
          {/each}
        </div>

        <div class="date-grid">
          {#each calendar.days as day}
            <button
              type="button"
              class="date-cell"
              class:muted={day.muted}
              class:today={day.today}
              class:selected={day.selected}
              disabled={day.disabled}
              on:click={() => selectDay(day)}
              aria-label={day.value}
            >
              {day.label}
            </button>
          {/each}
        </div>

        <div class="date-picker-foot">
          {#if clearable}
            <button type="button" on:click={clearValue}>清除</button>
          {/if}
          <button type="button" on:click={chooseToday}>今天</button>
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
  .date-group {
    position: relative;
    z-index: 1;
    width: 100%;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 7px;
    box-sizing: border-box;
  }

  .date-group:focus-within {
    z-index: 1700;
  }

  .date-group:not(.compact) {
    margin-bottom: 16px;
  }

  .date-label {
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

  .date-wrapper {
    position: relative;
    width: 100%;
    min-width: 0;
    overflow: visible;
  }

  .date-trigger {
    width: 100%;
    min-height: var(--wa-control-h, 36px);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 10px;
    border: 1px solid rgba(123, 143, 160, 0.2);
    border-radius: var(--wa-radius-md, 7px);
    background: rgba(255, 255, 255, 0.72);
    color: var(--wa-text-muted, #667789);
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

  .date-trigger:hover,
  .date-trigger:focus,
  .date-trigger.is-active {
    outline: none;
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    background: rgba(255, 255, 255, 0.94);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 0 0 3px rgba(0, 143, 150, 0.1);
  }

  .date-trigger.has-value {
    color: var(--wa-text-strong, #0d1722);
    font-weight: 680;
  }

  .date-trigger:disabled {
    opacity: 0.58;
    cursor: not-allowed;
  }

  .date-icon {
    width: 15px;
    height: 15px;
    flex: 0 0 15px;
    background: currentColor;
    mask: url("data:image/svg+xml,%3Csvg viewBox='0 0 24 24' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M7 4v3M17 4v3M5 8h14M6 6h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2Z' fill='none' stroke='black' stroke-width='1.8' stroke-linecap='round'/%3E%3C/svg%3E") center / contain no-repeat;
    -webkit-mask: url("data:image/svg+xml,%3Csvg viewBox='0 0 24 24' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M7 4v3M17 4v3M5 8h14M6 6h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2Z' fill='none' stroke='black' stroke-width='1.8' stroke-linecap='round'/%3E%3C/svg%3E") center / contain no-repeat;
  }

  .date-value {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .date-clear {
    position: absolute;
    top: 50%;
    right: 8px;
    transform: translateY(-50%);
    width: 20px;
    height: 20px;
    display: grid;
    place-items: center;
    border: 0;
    border-radius: var(--wa-radius-xs, 4px);
    background: transparent;
    color: var(--wa-text-muted, #667789);
    cursor: pointer;
    font-size: 16px;
    line-height: 1;
  }

  .date-clear:hover {
    background: rgba(102, 119, 137, 0.1);
    color: var(--wa-text-strong, #0d1722);
  }

  .date-picker-panel {
    position: absolute;
    top: calc(100% + 8px);
    left: 0;
    right: auto;
    z-index: 1900;
    width: min(292px, calc(100vw - 32px));
    max-height: var(--date-panel-max-height, 380px);
    overflow: auto;
    padding: 10px;
    border: 1px solid rgba(123, 143, 160, 0.18);
    border-radius: var(--wa-radius-lg, 8px);
    background: rgba(255, 255, 255, 0.97);
    color: var(--wa-text-main, #293847);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 24px 58px rgba(26, 41, 58, 0.17);
    backdrop-filter: blur(18px) saturate(126%);
    -webkit-backdrop-filter: blur(18px) saturate(126%);
  }

  .date-picker-head {
    min-height: 34px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 8px;
  }

  .date-picker-head strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 14px;
    font-weight: 800;
  }

  .date-picker-head button,
  .date-picker-foot button,
  .date-cell {
    border: 0;
    background: transparent;
    color: var(--wa-text-main, #293847);
    font: inherit;
    cursor: pointer;
  }

  .date-picker-head button {
    width: 32px;
    height: 32px;
    border-radius: var(--wa-radius-md, 7px);
    color: var(--wa-text-muted, #667789);
    font-size: 22px;
    line-height: 1;
  }

  .date-picker-head button:hover,
  .date-picker-foot button:hover {
    background: rgba(0, 143, 150, 0.09);
    color: var(--wa-accent-strong, #006f76);
  }

  .date-week-grid,
  .date-grid {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    gap: 4px;
  }

  .date-week-grid {
    margin-bottom: 6px;
  }

  .date-week-grid span {
    color: var(--wa-text-subtle, #8a99aa);
    font-size: 11px;
    font-weight: 760;
    text-align: center;
  }

  .date-cell {
    height: 34px;
    border-radius: var(--wa-radius-md, 7px);
    font-size: 12px;
    font-variant-numeric: tabular-nums;
  }

  .date-cell:hover:not(:disabled) {
    background: rgba(0, 143, 150, 0.09);
    color: var(--wa-accent-strong, #006f76);
  }

  .date-cell.muted {
    color: var(--wa-text-subtle, #8a99aa);
  }

  .date-cell.today {
    box-shadow: inset 0 0 0 1px rgba(0, 143, 150, 0.38);
    color: var(--wa-accent-strong, #006f76);
    font-weight: 760;
  }

  .date-cell.selected {
    background: var(--wa-accent, #008f96);
    color: var(--wa-accent-ink, #ffffff);
    font-weight: 800;
    box-shadow: 0 8px 18px rgba(0, 143, 150, 0.18);
  }

  .date-cell:disabled {
    opacity: 0.32;
    cursor: not-allowed;
  }

  .date-picker-foot {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
    margin-top: 9px;
    padding-top: 9px;
    border-top: 1px solid rgba(123, 143, 160, 0.14);
  }

  .date-picker-foot button {
    min-height: 30px;
    padding: 0 10px;
    border-radius: var(--wa-radius-md, 7px);
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    font-weight: 740;
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

  .has-error .date-trigger {
    border-color: rgba(221, 75, 62, 0.42);
  }
</style>
