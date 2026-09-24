<script lang="ts">
  import { safeEmailPreviewHTML } from '../../lib/email-preview';
  import type { EmailTemplateEntry } from '../../lib/email-config';
  import Button from '../shared/Button.svelte';

  export let preview: { subject: string; html: string; date: string; timezone: string; warnings: string[] } | null = null;
  export let reportDate = '';
  export let stale = false;
  export let previewing = false;
  export let sending = false;
  export let saving = false;
  export let dirty = false;
  export let canWrite = false;
  export let smtpEnabled = false;
  export let recipientCount = 0;
  export let onPreview: () => void = () => {};
  export let onSend: () => void = () => {};
  export let candidate: EmailTemplateEntry | null = null;
  export let candidateApplied = false;
  export let candidateBusy = false;
  export let onUseCandidate: () => void = () => {};
  export let onCloseCandidate: () => void = () => {};
  $: safeHTML = candidate ? safeEmailPreviewHTML(candidate.preview_html) : preview ? safeEmailPreviewHTML(preview.html) : '';
  $: sendReason = !canWrite ? '当前账号为只读权限。' : dirty ? '请先保存更改，再发送早报。' : !smtpEnabled ? '请先保存并启用发信服务。' : !recipientCount ? '请先保存至少一个早报收件人。' : `使用已保存配置，发送至 ${recipientCount} 个收件人。`;
</script>

<aside class="email-preview-pane" aria-label="邮件预览与发送" aria-busy={previewing || sending}>
  <div class="scw-section-head email-pane-heading">
    <div class="scw-section-copy"><h4>{candidate ? '模板样式' : '邮件预览'}</h4><p>{candidate ? candidate.name : '查看当前草稿填入真实数据后的邮件。'}</p></div>
    <span class="email-preview-state" class:stale={!candidate && stale}>{candidate ? '示例数据' : preview ? stale ? '需要更新' : '已生成' : '待预览'}</span>
  </div>
  {#if candidate}
    <div class="sample-controls"><p class="scw-help">{candidateApplied ? '此样式已用于当前草稿，保存后才用于实际发信。' : '当前展示候选原版，与草稿不同；点击使用将替换草稿。下方为示例数据。'}</p><div class="scw-section-actions">{#if canWrite}<Button variant="secondary" disabled={candidateApplied || saving || candidateBusy} on:click={onUseCandidate}>{candidateApplied ? '已用于草稿' : '使用此模板'}</Button>{/if}<Button variant="ghost" on:click={onCloseCandidate}>返回草稿预览</Button></div></div>
  {:else}
  <div class="preview-toolbar">
    <div class="scw-native-field">
      <label class="scw-native-label" for="email-report-date">报告日期（发送日）</label>
      <input id="email-report-date" class="scw-native-input" type="date" bind:value={reportDate} disabled={sending || !canWrite} aria-describedby="email-date-help" />
    </div>
    {#if canWrite}<Button variant="secondary" loading={previewing} on:click={onPreview}>预览当前草稿</Button>{/if}
  </div>
  <p id="email-date-help" class="scw-help">留空使用所选时区今天，统计昨日及前三天。预览不会发送邮件或写入 Confluence。</p>
  {/if}
  <div class="preview-output" class:is-loading={previewing}>
    {#if candidate || preview}
      <div class="preview-caption"><strong>{candidate ? candidate.name : preview?.subject}</strong><span>{candidate ? '模板样式 · 示例数据' : `${preview?.date} · ${preview?.timezone}${stale ? ' · 草稿已更改' : ''}`}</span></div>
      <iframe title={candidate ? '候选模板完整样式预览' : '早报邮件内容预览'} sandbox="allow-popups allow-popups-to-escape-sandbox" srcdoc={safeHTML}></iframe>
    {:else}
      <div class="preview-empty">
        <span class="wa-icon icon-library" aria-hidden="true"></span>
        <strong>{previewing ? '正在生成邮件预览' : '预览尚未生成'}</strong>
        <p>{previewing ? '正在读取 Jira 与成员数据。' : '保存前可先检查图表、事项明细和负责人。预览不会发送邮件或写入 Confluence。'}</p>
        <div class="preview-outline" aria-hidden="true"><span>Jira 图表分析</span><span>事项与负责人</span><span>可选 commit 分析</span></div>
      </div>
    {/if}
  </div>
  {#if !candidate}
  <div class="preview-delivery">
    <div class="delivery-copy"><strong>发送早报</strong><span>{sendReason}</span></div>
    {#if canWrite}<Button variant="secondary" loading={sending} disabled={saving || dirty || !smtpEnabled || !recipientCount} on:click={onSend}>发送已保存早报</Button>{/if}
    <p class="scw-help">Confluence 同步失败不阻止邮件发送；已发送后重试仅同步文档。邮件已发送或结果未知时不会自动重发。SMTP 接受后，请在邮箱确认实际到达。</p>
  </div>
  {/if}
</aside>

<style>
  /* finesse · register=product · shell=email-output-preview */
  .email-preview-pane { min-width: 0; display: grid; align-content: start; gap: var(--wa-space-3); }
  .scw-section-head { margin-bottom: 0; }
  .sample-controls { display: grid; gap: var(--wa-space-3); }
  .sample-controls .scw-section-actions { justify-content: flex-start; }
  .email-preview-state { flex: none; padding: 3px 8px; border-radius: var(--wa-radius-sm); background: var(--wa-neutral-soft); color: var(--scw-muted); font-size: 11px; line-height: 1.5; font-weight: 700; }
  .email-preview-state.stale { background: var(--wa-warning-soft); color: var(--wa-warning); }
  .preview-toolbar { display: flex; align-items: end; flex-wrap: wrap; gap: var(--wa-space-2); }
  .preview-toolbar .scw-native-field { flex: 1 1 160px; }
  .preview-toolbar :global(.btn) { flex: 0 0 auto; }
  .scw-help { margin-top: 0; }
  .preview-output { min-width: 0; overflow: hidden; border: 1px solid var(--scw-line); border-radius: var(--wa-radius-md); background: var(--wa-surface-inset); }
  .preview-caption { display: grid; gap: var(--wa-space-1); padding: var(--wa-space-3) var(--wa-space-4); border-bottom: 1px solid var(--scw-line); }
  .preview-caption strong { color: var(--scw-ink); font-size: 12px; overflow-wrap: anywhere; }
  .preview-caption span { color: var(--scw-muted); font-size: 11px; }
  iframe { display: block; border: 0; width: 100%; height: 540px; background: var(--wa-surface-panel); }
  .preview-empty { min-height: 240px; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: var(--wa-space-6); text-align: center; gap: var(--wa-space-3); }
  .preview-empty > .wa-icon { width: 24px; height: 24px; color: var(--scw-accent-strong); }
  .preview-empty strong { font-size: 14px; color: var(--scw-ink); }
  .preview-empty p { max-width: 30ch; margin: 0; color: var(--scw-muted); font-size: 12px; line-height: 1.7; }
  .preview-outline { display: flex; flex-wrap: wrap; justify-content: center; gap: var(--wa-space-2); margin-top: var(--wa-space-2); color: var(--scw-muted); font-size: 11px; }
  .preview-outline span + span::before { content: '·'; margin-right: var(--wa-space-2); }
  .is-loading { opacity: 0.65; }
  .preview-delivery { display: grid; gap: var(--wa-space-3); padding-top: var(--wa-space-4); border-top: 1px solid var(--scw-line); }
  .delivery-copy { display: grid; gap: var(--wa-space-1); }
  .delivery-copy strong { font-size: 13px; color: var(--scw-ink); }
  .delivery-copy span { font-size: 12px; color: var(--scw-muted); line-height: 1.6; }
  @media (max-width: 760px) { .preview-toolbar { align-items: stretch; } .preview-toolbar .scw-native-field { flex-basis: 100%; } .preview-toolbar :global(.btn) { width: 100%; min-height: 44px; } .preview-empty { min-height: 180px; } .preview-delivery :global(.btn) { min-height: 44px; } iframe { height: 480px; } }
</style>
