<script lang="ts">
  export let label = '关闭弹窗';
  export let disabled = false;

  let buttonEl: HTMLButtonElement;

  export function focus(options?: FocusOptions) {
    buttonEl?.focus(options);
  }
</script>

<button
  bind:this={buttonEl}
  class="overlay-close-button"
  type="button"
  aria-label={label}
  {disabled}
  on:click
>
  <svg aria-hidden="true" viewBox="0 0 24 24" focusable="false">
    <path d="M6 6l12 12M18 6L6 18" />
  </svg>
</button>

<style>
  .overlay-close-button {
    width: var(--wa-touch-h, 44px);
    height: var(--wa-touch-h, 44px);
    min-width: var(--wa-touch-h, 44px);
    min-height: var(--wa-touch-h, 44px);
    flex: 0 0 var(--wa-touch-h, 44px);
    align-self: start;
    display: inline-grid;
    place-items: center;
    box-sizing: border-box;
    padding: 0;
    border: 1px solid transparent;
    border-radius: var(--wa-radius-md, 7px);
    background: var(--wa-neutral-soft, rgba(102, 119, 137, 0.08));
    color: var(--wa-text-muted, #667789);
    cursor: pointer;
    transition:
      background var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      color var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }

  .overlay-close-button svg {
    width: 19px;
    height: 19px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2.25;
    stroke-linecap: round;
  }

  .overlay-close-button:hover:not(:disabled) {
    border-color: var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.1));
    color: var(--wa-accent-strong, #006f76);
  }

  .overlay-close-button:focus-visible {
    outline: none;
    border-color: var(--wa-accent, #008f96);
    box-shadow: 0 0 0 3px var(--wa-accent-soft, rgba(0, 143, 150, 0.16));
    color: var(--wa-accent-strong, #006f76);
  }

  .overlay-close-button:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  @media (prefers-reduced-motion: reduce) {
    .overlay-close-button {
      transition: none;
    }
  }
</style>
