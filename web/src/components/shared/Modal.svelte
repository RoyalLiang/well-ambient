<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { lockBodyScroll, unlockBodyScroll } from '../../lib/modalScrollLock';
  const dispatch = createEventDispatcher();

  export let show = false;
  export let title = '';

  let backdropEl: HTMLDivElement;
  let modalId = `modal-title-${Math.random().toString(36).slice(2)}`;
  let mousedownOnBackdrop = false;
  let isCurrentlyShown = false;

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      close();
    }
  }

  function handleMousedown(e: MouseEvent) {
    mousedownOnBackdrop = e.target === backdropEl;
  }

  function handleMouseup(e: MouseEvent) {
    if (mousedownOnBackdrop && e.target === backdropEl) {
      close();
    }
    mousedownOnBackdrop = false;
  }

  $: {
    if (show && !isCurrentlyShown) {
      isCurrentlyShown = true;
      lockBodyScroll();
    } else if (!show && isCurrentlyShown) {
      isCurrentlyShown = false;
      unlockBodyScroll();
    }
  }

  onDestroy(() => {
    if (isCurrentlyShown) {
      isCurrentlyShown = false;
      unlockBodyScroll();
    }
  });
</script>

<svelte:window on:keydown={handleKeydown} />

{#if show}
  <div 
    class="modal-backdrop" 
    bind:this={backdropEl} 
    on:mousedown={handleMousedown} 
    on:mouseup={handleMouseup}
    role="presentation"
  >
    <div
      class="modal-container"
      on:mousedown|stopPropagation
      on:mouseup|stopPropagation
      role="dialog"
      aria-modal="true"
      aria-labelledby={modalId}
      tabindex="-1"
    >
      <header class="modal-header">
        <h3 class="modal-title" id={modalId}>{title}</h3>
        <button class="close-btn" on:click={close} aria-label="Close modal">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
        </button>
      </header>
      <div class="modal-body">
        <slot />
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    width: 100vw;
    height: 100vh;
    background:
      linear-gradient(180deg, rgba(13, 23, 34, 0.22), rgba(13, 23, 34, 0.36)),
      rgba(238, 244, 247, 0.58);
    backdrop-filter: blur(18px) saturate(118%);
    -webkit-backdrop-filter: blur(18px) saturate(118%);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
    padding: 24px;
    box-sizing: border-box;
    animation: fadeIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .modal-container {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(247, 251, 253, 0.82)),
      rgba(255, 255, 255, 0.9);
    border: 1px solid rgba(255, 255, 255, 0.72);
    border-radius: var(--wa-radius-xl, 8px);
    width: 100%;
    max-width: 640px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.92),
      0 26px 72px rgba(26, 41, 58, 0.22);
    box-sizing: border-box;
    overflow: hidden;
    animation: scaleIn 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid rgba(123, 143, 160, 0.14);
    background: rgba(255, 255, 255, 0.44);
  }

  .modal-title {
    margin: 0;
    color: var(--wa-text-strong, #0d1722);
    font-size: 18px;
    line-height: 1.25;
    font-weight: 820;
    letter-spacing: 0;
  }

  .close-btn {
    width: 34px;
    height: 34px;
    background: rgba(102, 119, 137, 0.08);
    border: none;
    color: var(--wa-text-muted, #667789);
    cursor: pointer;
    padding: 0;
    border-radius: var(--wa-radius-md, 7px);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
  }

  .close-btn:hover {
    background: rgba(0, 143, 150, 0.1);
    color: var(--wa-accent-strong, #006f76);
  }

  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex-grow: 1;
    box-sizing: border-box;
    scrollbar-width: thin;
    scrollbar-color: rgba(0, 143, 150, 0.38) rgba(121, 139, 159, 0.1);
  }

  .modal-body::-webkit-scrollbar {
    width: 6px;
  }

  .modal-body::-webkit-scrollbar-thumb {
    background-color: rgba(0, 143, 150, 0.36);
    border-radius: 999px;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes scaleIn {
    from { transform: scale(0.95); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }
</style>
