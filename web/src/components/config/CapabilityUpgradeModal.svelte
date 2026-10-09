<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Modal from '../shared/Modal.svelte';
  import Button from '../shared/Button.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Textarea from '../shared/Textarea.svelte';

  export let show = false;
  export let capabilityKey = '';
  export let latestVersion = 1;
  export let mode: 'manifest' | 'remote' = 'manifest';
  export let manifestText = '';
  export let remoteURL = '';
  export let busy = false;
  export let error = '';

  const dispatch = createEventDispatcher<{ close: void; submit: void; modechange: void }>();
  const modes = ['manifest', 'remote'] as const;
  const idPrefix = `capability-upgrade-${Math.random().toString(36).slice(2)}`;

  function selectMode(next: typeof mode) {
    if (busy || next === mode) return;
    mode = next;
    dispatch('modechange');
  }

  function handleModeKeydown(event: KeyboardEvent, current: typeof mode) {
    if (busy) return;
    let nextIndex = modes.indexOf(current);
    if (event.key === 'ArrowRight') nextIndex = (nextIndex + 1) % modes.length;
    else if (event.key === 'ArrowLeft') nextIndex = (nextIndex + modes.length - 1) % modes.length;
    else if (event.key === 'Home') nextIndex = 0;
    else if (event.key === 'End') nextIndex = modes.length - 1;
    else return;
    event.preventDefault();
    selectMode(modes[nextIndex]);
    const tablist = (event.currentTarget as HTMLElement).parentElement;
    tablist?.querySelector<HTMLButtonElement>(`#${idPrefix}-tab-${modes[nextIndex]}`)?.focus();
  }

  function close() {
    if (!busy) dispatch('close');
  }

  function submit() {
    if (!busy) dispatch('submit');
  }
</script>

<Modal
  {show}
  title={`升级技能：${capabilityKey}`}
  size="wide"
  closeDisabled={busy}
  closeLabel="关闭技能升级对话框"
  on:close={close}
>
  <div class="upgrade-form" aria-busy={busy}>
    <div class="version-summary">
      <span>当前版本 <strong>v{latestVersion}</strong></span>
      <p>升级会创建新版本并切换生效版本，历史版本保留。</p>
    </div>

    <div class="mode-tabs" role="tablist" aria-label="升级方式">
      {#each modes as option}
        <button
          type="button"
          role="tab"
          id={`${idPrefix}-tab-${option}`}
          aria-selected={mode === option}
          aria-controls={`${idPrefix}-panel-${option}`}
          tabindex={mode === option ? 0 : -1}
          disabled={busy}
          class:active={mode === option}
          on:click={() => selectMode(option)}
          on:keydown={(event) => handleModeKeydown(event, option)}
        >
          {option === 'manifest' ? '提供新版 Manifest' : '远程拉取升级'}
        </button>
      {/each}
    </div>

    {#if error}
      <div class="upgrade-error" role="alert">{error}</div>
    {/if}

    <div
      class="mode-panel"
      id={`${idPrefix}-panel-manifest`}
      role="tabpanel"
      aria-labelledby={`${idPrefix}-tab-manifest`}
      hidden={mode !== 'manifest'}
    >
      <Textarea
        id={`${idPrefix}-manifest`}
        label="新版本 Manifest"
        bind:value={manifestText}
        rows={8}
        monospace
        required
        disabled={busy || mode !== 'manifest'}
        placeholder="输入新版本 Manifest（YAML 或 JSON）"
        helperText={`建议版本号为 v${latestVersion + 1}。提交前确认技能标识与升级目标一致。`}
      />
    </div>

    <div
      class="mode-panel"
      id={`${idPrefix}-panel-remote`}
      role="tabpanel"
      aria-labelledby={`${idPrefix}-tab-remote`}
      hidden={mode !== 'remote'}
    >
      <TextInput
        id={`${idPrefix}-remote-url`}
        label="远程升级 Manifest URL"
        type="url"
        bind:value={remoteURL}
        required
        disabled={busy || mode !== 'remote'}
        placeholder="https://example.com/capabilities/review-v2.yaml"
        helperText="填写新版本 Manifest 的地址，系统将在提交后读取并校验内容。"
      />
    </div>
  </div>

  <div slot="footer" class="upgrade-actions">
    <Button variant="ghost" disabled={busy} on:click={close}>取消</Button>
    <Button loading={busy} on:click={submit}>{busy ? '正在升级部署…' : '确认并部署新版本'}</Button>
  </div>
</Modal>

<style>
  /* finesse · capability upgrade · shared modal and control vocabulary */
  .upgrade-form {
    box-sizing: border-box;
    min-width: 0;
    display: grid;
    gap: var(--wa-space-4);
    padding: var(--wa-space-5) var(--wa-space-6);
    color: var(--wa-text-main);
  }

  .version-summary {
    padding-bottom: var(--wa-space-3);
    border-bottom: 1px solid var(--wa-border-divider);
    font-size: 13px;
    line-height: 1.5;
  }

  .version-summary strong {
    margin-left: var(--wa-space-1);
    color: var(--wa-text-strong);
    font-variant-numeric: tabular-nums;
  }

  .version-summary p {
    margin: var(--wa-space-2) 0 0;
    color: var(--wa-text-muted);
  }

  .mode-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
    min-width: 0;
  }

  .mode-tabs button {
    min-height: var(--wa-control-h);
    padding: var(--wa-space-2) var(--wa-space-3);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-lift);
    color: var(--wa-text-muted);
    font: 650 13px/1.5 var(--wa-font-sans);
    cursor: pointer;
    transition: background-color var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease);
  }

  .mode-tabs button:hover:not(:disabled) {
    background: var(--wa-row-hover);
    border-color: var(--wa-border-strong);
  }

  .mode-tabs button.active {
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
    border-color: var(--wa-border-focus);
  }

  .mode-tabs button:focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  .mode-tabs button:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }

  .mode-panel {
    min-width: 0;
    min-height: calc(var(--wa-control-h) * 7);
  }

  .mode-panel :global(textarea) {
    max-height: min(320px, 45dvh);
  }

  .upgrade-error {
    padding: var(--wa-space-3);
    border: 1px solid var(--wa-danger);
    border-radius: var(--wa-radius-md);
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
    font-size: 13px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .upgrade-actions {
    width: 100%;
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: var(--wa-space-3);
  }

  @media (max-width: 760px) {
    .upgrade-form {
      padding: var(--wa-space-4);
    }

    .mode-tabs button {
      min-height: var(--wa-touch-h);
      flex: 1 1 160px;
    }

    .upgrade-actions {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
    }

    .upgrade-actions :global(.btn) {
      width: 100%;
      min-height: var(--wa-touch-h);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .mode-tabs button {
      transition: none;
    }
  }
</style>
