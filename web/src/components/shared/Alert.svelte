<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let type: 'success' | 'error' | 'warning' | 'info' = 'info';
  export let title = '';
  export let message = '';
  export let closable = false;

  let show = true;

  function close() {
    show = false;
    dispatch('close');
  }
</script>

{#if show}
  <div class="alert alert-{type}">
    <div class="alert-icon">
      {#if type === 'success'}
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>
      {:else if type === 'error'}
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>
      {:else if type === 'warning'}
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>
      {:else}
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>
      {/if}
    </div>
    
    <div class="alert-content">
      {#if title}
        <h4 class="alert-title">{title}</h4>
      {/if}
      {#if message}
        <p class="alert-message">{message}</p>
      {/if}
      <slot />
    </div>

    {#if closable}
      <button class="close-btn" on:click={close} aria-label="Close alert">
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
      </button>
    {/if}
  </div>
{/if}

<style>
  .alert {
    display: flex;
    gap: 12px;
    padding: 14px 16px;
    border-radius: 8px;
    border: 1px solid transparent;
    margin-bottom: 20px;
    position: relative;
    box-sizing: border-box;
    width: 100%;
    animation: slideDown 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .alert-icon {
    flex-shrink: 0;
    display: flex;
    align-items: flex-start;
    margin-top: 2px;
  }

  .alert-content {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .alert-title {
    margin: 0;
    font-size: 0.85rem;
    font-weight: 700;
  }

  .alert-message {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.4;
  }

  .close-btn {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 2px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: currentColor;
    opacity: 0.6;
    transition: opacity 0.2s;
    height: fit-content;
    align-self: flex-start;
    margin-top: 2px;
  }

  .close-btn:hover {
    opacity: 1;
  }

  /* Types */
  .alert-success {
    background: rgba(16, 185, 129, 0.1);
    border-color: rgba(16, 185, 129, 0.2);
    color: #34d399;
  }

  .alert-error {
    background: rgba(239, 68, 68, 0.1);
    border-color: rgba(239, 68, 68, 0.2);
    color: #f87171;
  }

  .alert-warning {
    background: rgba(245, 158, 11, 0.1);
    border-color: rgba(245, 158, 11, 0.2);
    color: #fbbf24;
  }

  .alert-info {
    background: rgba(59, 130, 246, 0.1);
    border-color: rgba(59, 130, 246, 0.2);
    color: #60a5fa;
  }

  @keyframes slideDown {
    from {
      opacity: 0;
      transform: translateY(-8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
