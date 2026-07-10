<script lang="ts">
  export let label = '';
  export let value = '';
  export let placeholder = '';
  export let type = 'text';
  export let id = '';
  export let required = false;
  export let disabled = false;
  export let error = '';
  export let helperText = '';

  let showPassword = false;
  $: inputType = type === 'password' && showPassword ? 'text' : type;

  function togglePassword() {
    showPassword = !showPassword;
  }
</script>

<div class="input-group" class:has-error={!!error} class:disabled>
  {#if label}
    <label class="input-label" for={id}>
      {label}
      {#if required}
        <span class="required-star">*</span>
      {/if}
    </label>
  {/if}

  <div class="input-wrapper">
    <input
      {id}
      type={inputType}
      bind:value
      {placeholder}
      {disabled}
      {required}
      class="text-input"
      class:password-padding={type === 'password'}
    />

    {#if type === 'password'}
      <button 
        type="button" 
        class="toggle-password-btn" 
        on:click={togglePassword}
        {disabled}
        tabindex="-1"
      >
        {#if showPassword}
          <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path><line x1="1" y1="1" x2="23" y2="23"></line></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle></svg>
        {/if}
      </button>
    {/if}
  </div>

  {#if error}
    <span class="error-text">{error}</span>
  {:else if helperText}
    <span class="helper-text">{helperText}</span>
  {/if}
</div>

<style>
  .input-group {
    display: flex;
    flex-direction: column;
    gap: 7px;
    margin-bottom: 16px;
    width: 100%;
    box-sizing: border-box;
  }

  .input-label {
    font-size: 13px;
    font-weight: 700;
    color: var(--wa-text-main, #293847);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .required-star {
    color: var(--wa-danger, #dd4b3e);
  }

  .input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
  }

  .text-input {
    width: 100%;
    min-height: var(--wa-control-h, 36px);
    background: rgba(255, 255, 255, 0.84);
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-md, 7px);
    color: var(--wa-text-main, #293847);
    padding: 0 12px;
    font-size: 13px;
    transition: border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease), background var(--wa-duration-fast, 140ms) var(--wa-ease, ease), box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
    box-sizing: border-box;
  }

  .text-input:focus {
    outline: none;
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
    background: #ffffff;
  }

  .text-input::placeholder {
    color: var(--wa-text-subtle, #8a99aa);
    font-size: 12px;
    font-weight: 500;
    opacity: 1;
  }

  .text-input:disabled {
    background: var(--wa-surface-inset, #f5f8fb);
    border-color: rgba(123, 143, 160, 0.14);
    color: var(--wa-text-subtle, #8a99aa);
    cursor: not-allowed;
  }

  .password-padding {
    padding-right: 44px;
  }

  .toggle-password-btn {
    position: absolute;
    right: 12px;
    background: transparent;
    border: none;
    color: var(--wa-text-muted, #667789);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    border-radius: 4px;
    transition: all 0.2s;
  }

  .toggle-password-btn:hover:not(:disabled) {
    color: var(--wa-text-strong, #0d1722);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
  }

  .error-text {
    color: var(--wa-danger, #dd4b3e);
    font-size: 0.75rem;
    margin-top: 6px;
    font-weight: 500;
  }

  .helper-text {
    color: var(--wa-text-muted, #667789);
    font-size: 0.75rem;
    margin-top: 6px;
    line-height: 1.4;
  }

  /* Error States */
  .has-error .text-input {
    border-color: rgba(221, 75, 62, 0.5);
  }

  .has-error .text-input:focus {
    border-color: var(--wa-danger, #dd4b3e);
    box-shadow: 0 0 0 2px rgba(221, 75, 62, 0.14);
  }
</style>
