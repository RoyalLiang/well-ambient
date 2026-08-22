<script lang="ts">
  import { createEventDispatcher, onDestroy, tick } from 'svelte';
  import { lockBodyScroll, unlockBodyScroll } from '../../lib/modalScrollLock';
  import OverlayCloseButton from './OverlayCloseButton.svelte';
  const dispatch = createEventDispatcher();

  export let show = false;
  export let title = '';
  export let variant: 'dialog' | 'drawer' = 'dialog';
  export let size: 'default' | 'wide' = 'default';
  export let closeLabel = '关闭弹窗';
  export let shadowless = false;
  export let hideBodyScrollbar = false;

  let backdropEl: HTMLDivElement;
  let modalContainer: HTMLDivElement;
  let modalId = `modal-title-${Math.random().toString(36).slice(2)}`;
  let mousedownOnBackdrop = false;
  let isCurrentlyShown = false;
  let workspaceScoped = false;
  let returnFocusEl: HTMLElement | null = null;

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!show) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      close();
      return;
    }
    if (e.key !== 'Tab' || !modalContainer) return;
    const focusable = Array.from(modalContainer.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter((element) => !element.hasAttribute('hidden'));
    if (focusable.length === 0) {
      e.preventDefault();
      modalContainer.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  async function focusModal() {
    await tick();
    const firstFocusable = modalContainer?.querySelector<HTMLElement>(
      'button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    (firstFocusable || modalContainer)?.focus();
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

  function portalToOverlayLayer(node: HTMLElement) {
    const workspaceStage = node.closest<HTMLElement>('.workspace-stage')
      || document.querySelector<HTMLElement>('.workspace-stage');
    const target = workspaceStage || document.body;

    workspaceScoped = Boolean(workspaceStage);
    node.dataset.modalScope = workspaceStage ? 'workspace' : 'viewport';
    target.appendChild(node);

    return {
      destroy() {
        workspaceScoped = false;
        node.remove();
      }
    };
  }

  $: {
    if (show && !isCurrentlyShown) {
      isCurrentlyShown = true;
      returnFocusEl = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      lockBodyScroll();
      void focusModal();
    } else if (!show && isCurrentlyShown) {
      isCurrentlyShown = false;
      unlockBodyScroll();
      if (returnFocusEl?.isConnected) returnFocusEl.focus();
      returnFocusEl = null;
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
    use:portalToOverlayLayer
    class="modal-backdrop"
    class:is-workspace-scoped={workspaceScoped}
    class:is-drawer={variant === 'drawer'}
    bind:this={backdropEl} 
    on:mousedown={handleMousedown} 
    on:mouseup={handleMouseup}
    role="presentation"
  >
    <div
      class="modal-container"
      class:shadowless
      class:is-drawer={variant === 'drawer'}
      class:is-wide={size === 'wide' && variant === 'dialog'}
      bind:this={modalContainer}
      on:mousedown|stopPropagation
      on:mouseup|stopPropagation
      role="dialog"
      aria-modal="true"
      aria-labelledby={modalId}
      tabindex="-1"
    >
      <header class="modal-header">
        <h3 class="modal-title" id={modalId}>{title}</h3>
        <OverlayCloseButton label={closeLabel} on:click={close} />
      </header>
      <div class:hide-body-scrollbar={hideBodyScrollbar} class="modal-body">
        <slot />
      </div>
      {#if $$slots.footer}
        <footer class="modal-footer">
          <slot name="footer" />
        </footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    width: 100vw;
    height: 100vh;
    height: 100dvh;
    background: rgba(24, 38, 51, 0.28);
    -webkit-backdrop-filter: blur(16px) saturate(112%);
    backdrop-filter: blur(16px) saturate(112%);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
    padding: 24px;
    box-sizing: border-box;
    animation: fadeIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .modal-backdrop.is-drawer {
    justify-content: flex-end;
    align-items: stretch;
    padding: 0;
    background: rgba(24, 38, 51, 0.18);
    -webkit-backdrop-filter: blur(8px) saturate(108%);
    backdrop-filter: blur(8px) saturate(108%);
  }

  .modal-container {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(247, 251, 253, 0.82)),
      rgba(255, 255, 255, 0.9);
    border: 1px solid rgba(255, 255, 255, 0.72);
    border-radius: var(--wa-radius-xl, 8px);
    background-clip: padding-box;
    isolation: isolate;
    width: 100%;
    max-width: 640px;
    max-height: calc(100vh - 48px);
    max-height: calc(100dvh - 48px);
    min-height: 0;
    display: flex;
    flex-direction: column;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.92),
      0 26px 72px rgba(26, 41, 58, 0.22);
    box-sizing: border-box;
    overflow: hidden;
    animation: scaleIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .modal-container.is-drawer {
    width: min(620px, calc(100vw - 24px));
    max-width: none;
    height: 100dvh;
    max-height: none;
    border-width: 0 0 0 1px;
    border-radius: var(--wa-radius-xl, 18px) 0 0 var(--wa-radius-xl, 18px);
    box-shadow:
      inset 1px 0 0 rgba(255, 255, 255, 0.84),
      -24px 0 64px rgba(26, 41, 58, 0.18);
    animation: drawerIn 220ms cubic-bezier(0.16, 1, 0.3, 1);
  }

  .modal-container.shadowless,
  .modal-container.shadowless.is-drawer {
    box-shadow: none;
  }

  .modal-container.is-wide {
    max-width: 960px;
    max-height: min(900px, calc(100vh - 48px));
    max-height: min(900px, calc(100dvh - 48px));
  }

  .modal-container.is-wide .modal-body {
    padding: 0;
  }

  .modal-container.is-drawer .modal-header {
    border-radius: var(--wa-radius-xl, 18px) 0 0 0;
  }

  .modal-container.is-drawer .modal-body {
    padding: 0;
  }

  .modal-header {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: minmax(0, 1fr) var(--wa-touch-h, 44px);
    align-items: start;
    gap: var(--wa-space-4, 16px);
    padding: 20px 24px;
    border-bottom: 1px solid rgba(123, 143, 160, 0.14);
    border-radius: calc(var(--wa-radius-xl, 18px) - 1px) calc(var(--wa-radius-xl, 18px) - 1px) 0 0;
    background: rgba(255, 255, 255, 0.44);
    background-clip: padding-box;
  }

  .modal-title {
    min-width: 0;
    margin: 0;
    color: var(--wa-text-strong, #0d1722);
    font-size: 18px;
    line-height: 1.25;
    font-weight: 820;
    letter-spacing: 0;
    overflow-wrap: anywhere;
    text-wrap: pretty;
  }

  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex: 1 1 auto;
    min-height: 0;
    box-sizing: border-box;
    scrollbar-width: thin;
    scrollbar-color: rgba(0, 143, 150, 0.38) rgba(121, 139, 159, 0.1);
  }

  .modal-body.hide-body-scrollbar {
    scrollbar-width: none;
  }

  .modal-body.hide-body-scrollbar::-webkit-scrollbar {
    display: none;
  }

  .modal-footer {
    flex: 0 0 auto;
    padding: 12px 24px 16px;
    border-top: 1px solid rgba(123, 143, 160, 0.14);
    background: rgba(255, 255, 255, 0.62);
    box-sizing: border-box;
  }

  .modal-container.is-wide .modal-footer {
    padding-inline: 22px;
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

  @keyframes drawerIn {
    from { transform: translateX(28px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }

  @media (max-width: 760px) {
    .modal-backdrop:not(.is-drawer) {
      padding: 12px;
    }

    .modal-container:not(.is-drawer) {
      max-height: calc(100vh - 24px);
      max-height: calc(100dvh - 24px);
    }

    .modal-container:not(.is-drawer) .modal-header {
      gap: var(--wa-space-3, 12px);
      padding: 14px 16px;
    }

    .modal-container:not(.is-drawer) .modal-footer,
    .modal-container.is-wide .modal-footer {
      padding: 12px 16px;
    }

    .modal-container.is-drawer {
      width: 100vw;
      border-left: 0;
      border-radius: 0;
    }

    .modal-container.is-drawer .modal-header {
      min-height: 64px;
      padding: 14px 16px;
      border-radius: 0;
    }
  }

  @media (min-width: 861px) {
    .modal-backdrop.is-workspace-scoped {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
    }

    .modal-backdrop.is-workspace-scoped > .modal-container {
      max-height: calc(100% - 48px);
    }

    .modal-backdrop.is-workspace-scoped > .modal-container.is-wide {
      max-height: min(900px, calc(100% - 48px));
    }

    .modal-backdrop.is-workspace-scoped > .modal-container.is-drawer {
      width: min(620px, calc(100% - 24px));
      height: 100%;
      max-height: 100%;
    }
  }

  @media (max-width: 860px) {
    .modal-backdrop.is-workspace-scoped {
      position: fixed;
      inset: var(--wa-main-content-top, 0px) 0 0;
      width: auto;
      height: auto;
    }

    .modal-backdrop.is-workspace-scoped > .modal-container:not(.is-drawer) {
      max-height: calc(100% - 24px);
    }

    .modal-backdrop.is-workspace-scoped > .modal-container.is-drawer {
      height: 100%;
      max-height: 100%;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .modal-backdrop,
    .modal-container {
      animation: none;
    }
  }
</style>
