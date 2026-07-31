<script lang="ts">
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import Alert from './Alert.svelte';
  import { dismissToast, toastNotices } from '../../lib/toast';

  let reduceMotion = false;

  onMount(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const syncMotionPreference = () => {
      reduceMotion = media.matches;
    };

    syncMotionPreference();
    media.addEventListener('change', syncMotionPreference);
    return () => media.removeEventListener('change', syncMotionPreference);
  });
</script>

<div
  class="toast-region"
  aria-label="操作提醒"
  aria-live="polite"
  aria-relevant="additions text"
>
  {#each $toastNotices as notice (notice.id)}
    <div
      class="toast-item"
      transition:fly={{ x: 12, duration: reduceMotion ? 0 : 180 }}
    >
      <Alert
        type={notice.type}
        title={notice.title}
        message={notice.message}
        closable={true}
        closeLabel="关闭操作提醒"
        on:close={() => dismissToast(notice.id)}
      />
    </div>
  {/each}
</div>

<style>
  .toast-region {
    position: fixed;
    top: calc(var(--wa-workspace-topbar-h, 68px) + var(--wa-space-3, 12px));
    right: max(var(--wa-space-4, 16px), env(safe-area-inset-right));
    z-index: 1100;
    width: min(380px, calc(100vw - (var(--wa-space-4, 16px) * 2)));
    max-height: calc(100dvh - var(--wa-workspace-topbar-h, 68px) - (var(--wa-space-6, 24px) * 2));
    display: grid;
    align-content: start;
    gap: var(--wa-space-2, 8px);
    overflow: visible;
    pointer-events: none;
  }

  .toast-item {
    min-width: 0;
    pointer-events: auto;
  }

  .toast-item :global(.alert) {
    align-items: center;
    border-top-color: rgba(255, 255, 255, 0.86);
    background-color: rgba(251, 253, 254, 0.98);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 16px 36px rgba(36, 55, 72, 0.16);
  }

  .toast-item :global(.close-btn) {
    flex: 0 0 32px;
    width: 32px;
    min-width: 32px;
    height: 32px;
    margin-top: 0;
    align-self: center;
  }

  @media (max-width: 760px) {
    .toast-region {
      top: calc(var(--wa-workspace-topbar-h, 64px) + var(--wa-space-2, 8px));
      right: max(var(--wa-space-2, 8px), env(safe-area-inset-right));
      left: max(var(--wa-space-2, 8px), env(safe-area-inset-left));
      width: auto;
      max-height: calc(100% - (var(--wa-space-2, 8px) * 2));
    }

    .toast-item :global(.close-btn) {
      flex-basis: 44px;
      width: 44px;
      min-width: 44px;
      height: 44px;
    }
  }

  @media (prefers-reduced-transparency: reduce) {
    .toast-item :global(.alert) {
      background: #fbfdfe;
      backdrop-filter: none;
      -webkit-backdrop-filter: none;
    }
  }
</style>
