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
    margin-bottom: 20px;
    width: 100%;
    box-sizing: border-box;
  }

  .input-label {
    font-size: 0.825rem;
    font-weight: 600;
    color: #94a3b8;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .required-star {
    color: #f87171;
  }

  .input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
  }

  .text-input {
    width: 100%;
    background: #0b0f19;
    border: 1px solid rgba(51, 65, 85, 0.7);
    border-radius: 8px;
    color: #f1f5f9;
    padding: 10px 14px;
    font-size: 0.9rem;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-sizing: border-box;
  }

  .text-input:focus {
    outline: none;
    border-color: #6366f1;
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
    background: #0f172a;
  }

  .text-input:disabled {
    background: #1e293b;
    border-color: #334155;
    color: #64748b;
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
    color: #64748b;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    border-radius: 4px;
    transition: all 0.2s;
  }

  .toggle-password-btn:hover:not(:disabled) {
    color: #cbd5e1;
    background: rgba(51, 65, 85, 0.3);
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
  .has-error .text-input {
    border-color: rgba(239, 68, 68, 0.6);
  }

  .has-error .text-input:focus {
    border-color: #ef4444;
    box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15);
  }
</style>
