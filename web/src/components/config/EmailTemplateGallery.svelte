<script lang="ts">
  import { tick } from 'svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import { emailTemplateStyleName, emailTemplateSignature, type EmailTemplateEntry } from '../../lib/email-config';
  import { safeEmailPreviewHTML } from '../../lib/email-preview';

  export let templates: EmailTemplateEntry[] = [];
  export let loading = false;
  export let error = '';
  export let canWrite = false;
  export let busy = false;
  export let deletingId = '';
  export let viewedId = '';
  export let appliedSignature = '';
  export let onView: (entry: EmailTemplateEntry) => void = () => {};
  export let onUse: (entry: EmailTemplateEntry) => void = () => {};
  export let onDelete: (entry: EmailTemplateEntry) => Promise<boolean> = async () => false;
  export let onRetry: () => void = () => {};
  let track: HTMLUListElement;
  let visible = new Set<string>();
  let confirmingId = '';

  function observeCard(node: HTMLElement, id: string) {
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        visible = new Set([...visible, id]); observer.disconnect();
      }
    }, { root: track, rootMargin: '0px 300px', threshold: 0 });
    observer.observe(node);
    return { destroy() { observer.disconnect(); } };
  }
  function browse(direction: number) {
    track?.scrollBy({ left: direction * Math.max(260, track.clientWidth * 0.8), behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
  }
  async function remove(entry: EmailTemplateEntry) {
    const index = templates.findIndex(item => item.id === entry.id);
    const nextId = templates[index + 1]?.id || templates[index - 1]?.id;
    if (await onDelete(entry)) {
      confirmingId = ''; await tick();
      if (nextId) document.getElementById(`email-template-view-${nextId}`)?.focus();
    }
  }
</script>

<section class="template-gallery" aria-label="邮件模板图库" aria-busy={loading}>
  <div class="gallery-heading">
    <div><h4>模板样式 <span>{templates.length}</span></h4><p>示例数据 · 点击查看完整样式</p></div>
    <div class="gallery-navigation"><button type="button" class="scw-icon-button" aria-label="向左浏览模板" disabled={loading || !templates.length} on:click={() => browse(-1)}>上一组</button><button type="button" class="scw-icon-button" aria-label="向右浏览模板" disabled={loading || !templates.length} on:click={() => browse(1)}>下一组</button></div>
  </div>
  {#if error}<Alert type="error">{error}</Alert><Button variant="secondary" on:click={onRetry}>重新加载模板</Button>{/if}
  {#if loading && !templates.length}<div class="gallery-loading" role="status">正在加载模板样式…</div>{/if}
  {#if !loading && !error && !templates.length}
    <div class="gallery-empty" role="status"><strong>暂无可用模板</strong><p>可继续编辑当前草稿，或重新加载模板样式。</p><Button variant="secondary" on:click={onRetry}>重新加载模板</Button></div>
  {/if}
  <ul class="gallery-track" bind:this={track} aria-label="可用模板样式">
    {#each templates as entry (entry.id)}
      <li use:observeCard={entry.id}>
        <article class="template-card" class:viewed={viewedId === entry.id} data-template-id={entry.id}>
          <div class="template-shot">
            {#if visible.has(entry.id)}<iframe title={`${entry.name} 缩略预览`} aria-hidden="true" tabindex="-1" sandbox="" loading="lazy" srcdoc={safeEmailPreviewHTML(entry.preview_html)}></iframe>{/if}
            <button id={`email-template-view-${entry.id}`} class="shot-action" type="button" aria-label={`查看模板：${entry.name}`} on:click={() => onView(entry)}><span>查看完整样式</span></button>
            <span class="sample-tag">示例数据</span>
          </div>
          <div class="template-info"><strong title={`${entry.name} · ${entry.template.subject}`}>{entry.name}</strong><span>{entry.builtin ? '内置' : 'Agent 自定义整版模板'} · {emailTemplateStyleName(entry.template.style)}{!entry.builtin && entry.created_at ? ` · ${new Date(entry.created_at).toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}` : ''}</span></div>
          <div class="template-actions">
            {#if canWrite}<Button variant="secondary" size="small" disabled={busy || deletingId !== '' || appliedSignature === emailTemplateSignature(entry.template)} on:click={() => onUse(entry)}>{appliedSignature === emailTemplateSignature(entry.template) ? '当前草稿' : '使用模板'}</Button>{:else}<span class="template-readonly">只读浏览</span>{/if}
            {#if canWrite && !entry.builtin}<button class="template-remove" type="button" aria-label={`删除模板：${entry.name}`} disabled={busy || deletingId !== ''} on:click={() => confirmingId = confirmingId === entry.id ? '' : entry.id}>删除</button>{:else if entry.builtin}<span class="template-readonly">内置模板</span>{/if}
          </div>
          {#if confirmingId === entry.id}<div class="template-confirm" role="group" aria-label={`确认删除 ${entry.name}`}><p>删除「{entry.name}」？已应用草稿与发信配置会保留。</p><div><Button variant="danger" size="small" loading={deletingId === entry.id} disabled={busy} on:click={() => remove(entry)}>确认删除</Button><Button variant="ghost" size="small" disabled={deletingId !== ''} on:click={() => confirmingId = ''}>取消</Button></div></div>{/if}
        </article>
      </li>
    {/each}
  </ul>
</section>

<style>
  /* finesse · register=product · shell=horizontal-template-artifact-gallery */
  .template-gallery { min-width: 0; display: grid; gap: var(--wa-space-3); padding-bottom: var(--wa-space-5); border-bottom: 1px solid var(--scw-line); }
  .gallery-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--wa-space-3); }
  .gallery-navigation button:disabled { opacity: 0.5; cursor: not-allowed; }
  .gallery-empty { display: grid; justify-items: start; gap: var(--wa-space-3); padding-block: var(--wa-space-5); color: var(--scw-text); font-size: 13px; }
  .gallery-empty p { margin: 0; color: var(--scw-muted); line-height: 1.6; }
  h4 { margin: 0; font-size: 14px; color: var(--scw-ink); }
  h4 span { color: var(--scw-muted); font-size: 12px; font-weight: 400; margin-left: var(--wa-space-1); }
  .gallery-heading p { margin: 4px 0 0; color: var(--scw-muted); font-size: 12px; line-height: 1.6; }
  .gallery-navigation { display: flex; flex: none; gap: var(--wa-space-2); }
  .gallery-track { display: grid; grid-auto-flow: column; grid-auto-columns: 260px; align-items: start; justify-content: start; gap: var(--wa-space-3); overflow-x: auto; overscroll-behavior-inline: contain; scroll-snap-type: x proximity; padding: 3px 3px 8px; margin: 0; min-width: 0; list-style: none; }
  .gallery-track > li { min-width: 0; scroll-snap-align: start; }
  .template-card { overflow: hidden; border: 1px solid var(--scw-line-strong); border-radius: var(--wa-radius-md); background: var(--wa-surface-panel); }
  .template-card.viewed { border-color: var(--scw-accent-strong); box-shadow: 0 0 0 1px var(--scw-accent-strong); }
  .template-shot { position: relative; height: 132px; overflow: hidden; background: var(--wa-surface-inset); border-bottom: 1px solid var(--scw-line); }
  iframe { position: absolute; inset: 0 auto auto 0; border: 0; width: 680px; height: 900px; transform: scale(0.38); transform-origin: top left; pointer-events: none; }
  .shot-action { position: absolute; inset: 0; width: 100%; border: 0; background: transparent; cursor: pointer; padding: 0; }
  .shot-action span { position: absolute; left: 8px; bottom: 8px; padding: 4px 8px; border-radius: var(--wa-radius-xs); background: var(--wa-surface-panel); color: var(--scw-accent-strong); font-size: 11px; font-weight: 700; border: 1px solid var(--scw-line); }
  .shot-action:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: -3px; }
  .sample-tag { position: absolute; top: 7px; right: 7px; background: var(--wa-surface-panel); border: 1px solid var(--scw-line); padding: 2px 5px; border-radius: var(--wa-radius-xs); color: var(--scw-muted); font-size: 10px; pointer-events: none; }
  .template-info { display: grid; gap: 3px; padding: 8px 10px 0; }
  .template-info strong { color: var(--scw-ink); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .template-info > span { color: var(--scw-muted); font-size: 11px; }
  .template-actions { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 10px; }
  .template-readonly { color: var(--scw-muted); font-size: 11px; }
  .template-remove { min-height: 32px; border: 0; padding: 0 8px; color: var(--wa-danger); background: transparent; font: inherit; font-size: 12px; cursor: pointer; }
  .template-remove:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: 1px; }
  .template-remove:disabled { opacity: 0.5; cursor: not-allowed; }
  .template-confirm { padding: 10px; border-top: 1px solid var(--scw-line); background: var(--wa-surface-inset); }
  .template-confirm p { font-size: 12px; line-height: 1.6; margin: 0 0 8px; }
  .template-confirm > div { display: flex; gap: 8px; }
  .gallery-loading { min-height: 190px; display: grid; place-items: center; background: var(--wa-surface-inset); border-radius: var(--wa-radius-md); color: var(--scw-muted); font-size: 13px; }
  @media (max-width: 760px) { .gallery-heading { align-items: flex-start; } .gallery-navigation { flex-direction: row; gap: 4px; } .gallery-navigation button { min-height: 44px; } .template-remove { min-height: 44px; } .template-actions :global(.btn), .template-confirm :global(.btn) { min-height: 44px; } }
</style>
