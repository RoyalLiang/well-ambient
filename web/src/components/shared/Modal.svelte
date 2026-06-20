<script context="module" lang="ts">
  let activeModalsCount = 0;

  function registerModalOpen() {
    activeModalsCount++;
    if (typeof document !== 'undefined') {
      document.body.style.overflow = 'hidden';
    }
  }

  function registerModalClose() {
    activeModalsCount--;
    if (activeModalsCount <= 0 && typeof document !== 'undefined') {
      document.body.style.overflow = '';
    }
  }
</script>

<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  const dispatch = createEventDispatcher();

  export let show = false;
  export let title = '';

  let backdropEl: HTMLDivElement;
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
      registerModalOpen();
    } else if (!show && isCurrentlyShown) {
      isCurrentlyShown = false;
      registerModalClose();
    }
  }

  onDestroy(() => {
    if (isCurrentlyShown) {
      isCurrentlyShown = false;
      registerModalClose();
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
  >
    <div class="modal-container" on:mousedown|stopPropagation on:mouseup|stopPropagation>
      <header class="modal-header">
        <h3 class="modal-title">{title}</h3>
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
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(2, 6, 23, 0.75);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
    padding: 24px;
    box-sizing: border-box;
    animation: fadeIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .modal-container {
    background: rgba(15, 23, 42, 0.85);
    border: 1px solid rgba(129, 140, 248, 0.15);
    border-radius: 16px;
    width: 100%;
    max-width: 640px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 40px rgba(99, 102, 241, 0.1);
    box-sizing: border-box;
    overflow: hidden;
    animation: scaleIn 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
    background: rgba(30, 41, 59, 0.3);
  }

  .modal-title {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 700;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .close-btn {
    background: transparent;
    border: none;
    color: #64748b;
    cursor: pointer;
    padding: 6px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
  }

  .close-btn:hover {
    background: rgba(51, 65, 85, 0.5);
    color: #f1f5f9;
  }

  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex-grow: 1;
    box-sizing: border-box;
    scrollbar-width: thin;
    scrollbar-color: rgba(51, 65, 85, 0.5) transparent;
  }

  .modal-body::-webkit-scrollbar {
    width: 6px;
  }

  .modal-body::-webkit-scrollbar-thumb {
    background-color: rgba(51, 65, 85, 0.5);
    border-radius: 3px;
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
