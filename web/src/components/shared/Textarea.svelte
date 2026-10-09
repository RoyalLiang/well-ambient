<script lang="ts">
  export let id = '';
  export let label = '';
  export let value = '';
  export let rows = 8;
  export let placeholder = '';
  export let required = false;
  export let disabled = false;
  export let readonly = false;
  export let error = '';
  export let helperText = '';
  export let monospace = false;

  const generatedId = `textarea-${Math.random().toString(36).slice(2)}`;
  let control: HTMLTextAreaElement;
  $: controlId = id || generatedId;
  $: messageId = `${controlId}-message`;

  export function focus() {
    control?.focus();
  }
</script>

<div class="textarea-group" class:has-error={!!error} class:disabled class:readonly>
  {#if label}
    <label for={controlId}>
      {label}
      {#if required}<span class="required-mark" aria-hidden="true">*</span>{/if}
    </label>
  {/if}
  <textarea
    {...$$restProps}
    bind:this={control}
    id={controlId}
    bind:value
    {rows}
    {placeholder}
    {required}
    {disabled}
    {readonly}
    class:monospace
    spellcheck={!monospace}
    aria-invalid={error ? 'true' : undefined}
    aria-describedby={error || helperText ? messageId : undefined}
    on:input
    on:change
    on:blur
    on:focus
  ></textarea>
  {#if error}
    <span id={messageId} class="field-message error-message">{error}</span>
  {:else if helperText}
    <span id={messageId} class="field-message">{helperText}</span>
  {/if}
</div>

<style>
  /* finesse · shared textarea · product register · existing admin tokens */
  .textarea-group {
    min-width: 0;
    width: 100%;
    display: grid;
    gap: var(--wa-space-2);
    color: var(--wa-text-main);
  }

  label {
    display: flex;
    align-items: baseline;
    gap: var(--wa-space-1);
    font-size: 13px;
    font-weight: 700;
    line-height: 1.5;
  }

  .required-mark, .error-message {
    color: var(--wa-danger);
  }

  textarea {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    max-width: 100%;
    min-height: calc(var(--wa-control-h) * 3);
    padding: var(--wa-space-3);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-lift);
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
    font-size: 13px;
    line-height: 1.6;
    resize: vertical;
    transition: border-color var(--wa-duration-fast) var(--wa-ease),
      background-color var(--wa-duration-fast) var(--wa-ease);
  }

  textarea.monospace {
    font-family: var(--wa-font-mono);
  }

  textarea::placeholder {
    color: var(--wa-text-muted);
  }

  textarea:hover:not(:disabled) {
    border-color: var(--wa-border-strong);
  }

  textarea:focus {
    border-color: var(--wa-border-focus);
    background: var(--wa-surface-flat);
  }

  textarea:focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  textarea:disabled {
    cursor: not-allowed;
    background: var(--wa-surface-inset);
    color: var(--wa-text-muted);
  }

  textarea:read-only {
    background: var(--wa-surface-inset);
  }

  .has-error textarea {
    border-color: var(--wa-danger);
  }

  .field-message {
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.5;
  }

  .error-message {
    color: var(--wa-danger);
  }

  @media (max-width: 760px) {
    textarea {
      min-height: calc(var(--wa-touch-h) * 3);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    textarea {
      transition: none;
    }
  }
</style>
