<script lang="ts">
  import { onMount } from 'svelte';
  import Select from '../shared/Select.svelte';
  import { showToast } from '../../lib/toast';

  export let publicURL = '';
  export let onSavePublicURL: (value: string) => Promise<void> = async () => {};

  type PromptVersion = {
    purpose: PromptPurpose;
    id: number; scope_type: 'global' | 'project'; scope_id: string; version: number;
    status: 'draft' | 'active' | 'retired'; name: string; system_prompt: string;
    created_by: string; activated_by: string; created_at: string; activated_at?: string;
  };

  type PromptPurpose = 'solution_polish' | 'solution_compare_requirement' | 'solution_compare_compatibility';

  const purposeOptions = [
    { value: 'solution_polish', label: '方案润色', meta: '需求方案生成与整理' },
    { value: 'solution_compare_requirement', label: '需求等价性对比', meta: '两轮对比的第一轮' },
    { value: 'solution_compare_compatibility', label: '方案兼容性对比', meta: '两轮对比的第二轮' }
  ];
  const scopeOptions = [
    { value: 'global', label: '全局默认' },
    { value: 'project', label: '项目覆盖' }
  ];

  let prompts: PromptVersion[] = [];
  let loading = true;
  let action = '';
  let error = '';
  let notice = '';
  let purpose: PromptPurpose = 'solution_polish';
  let scopeType: 'global' | 'project' = 'global';
  let scopeID = '';
  let name = '';
  let systemPrompt = '';
  let activateOnSave = true;
  let testMarkdown = '# 示例需求方案\n\n## 目标\n\n- 验证提示词输出结构';
  let testOutput = '';
  let publicURLDraft = '';
  let publicURLSource = '';

  $: activePrompts = prompts.filter((prompt) => prompt.status === 'active');
  $: purposePrompts = prompts.filter((prompt) => prompt.purpose === purpose);
  $: if (publicURL !== publicURLSource) {
    if (!publicURLDraft || publicURLDraft === publicURLSource) publicURLDraft = publicURL || '';
    publicURLSource = publicURL || '';
  }

  onMount(() => {
    void loadPrompts();
  });

  async function api(path: string, init: RequestInit = {}) {
    const response = await fetch(path, init);
    const text = await response.text();
    let body: any = {};
    try { body = text ? JSON.parse(text) : {}; } catch { body = { message: text }; }
    if (!response.ok) throw new Error(body.message || body.error || text || `HTTP ${response.status}`);
    return body;
  }

  async function loadPrompts() {
    loading = true; error = '';
    try {
      const response = await api('/api/solution-prompts');
      prompts = response.items || [];
      if (!systemPrompt) systemPrompt = prompts.find((prompt: PromptVersion) => prompt.purpose === purpose && prompt.scope_type === 'global' && prompt.status === 'active')?.system_prompt || '';
    } catch (requestError: any) { error = requestError.message || '加载提示词失败'; }
    finally { loading = false; }
  }

  function editFrom(prompt: PromptVersion) {
    purpose = prompt.purpose;
    scopeType = prompt.scope_type;
    scopeID = prompt.scope_id || '';
    name = `${prompt.name} 新版`;
    systemPrompt = prompt.system_prompt;
    notice = `已载入 ${prompt.scope_type === 'global' ? '全局' : prompt.scope_id} v${prompt.version}，保存会新增版本。`;
  }

  function changePurpose(nextPurpose: string) {
    purpose = nextPurpose as PromptPurpose;
    const active = prompts.find((prompt) => prompt.purpose === purpose && prompt.scope_type === 'global' && prompt.status === 'active');
    systemPrompt = active?.system_prompt || '';
    scopeType = 'global';
    scopeID = '';
    name = '';
    testOutput = '';
  }

  async function savePrompt() {
    action = 'save'; error = ''; notice = '';
    try {
      const response = await api('/api/solution-prompts', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ purpose, scope_type: scopeType, scope_id: scopeID, name, system_prompt: systemPrompt, activate: activateOnSave })
      });
      showToast(`提示词 v${response.prompt.version} 已${activateOnSave ? '保存并启用' : '保存为草稿'}。历史版本未被覆盖。`, {
        title: '保存成功'
      });
      name = '';
      await loadPrompts();
    } catch (requestError: any) {
      showToast(requestError.message || '保存提示词失败', { type: 'error', title: '保存失败' });
    }
    finally { action = ''; }
  }

  async function activatePrompt(prompt: PromptVersion) {
    action = `activate-${prompt.id}`; error = ''; notice = '';
    try {
      await api(`/api/solution-prompts/${prompt.id}/activate`, { method: 'POST' });
      notice = `${prompt.scope_type === 'global' ? '全局' : prompt.scope_id} v${prompt.version} 已启用。`;
      await loadPrompts();
    } catch (requestError: any) { error = requestError.message || '启用提示词失败'; }
    finally { action = ''; }
  }

  async function testPrompt() {
    action = 'test'; error = ''; notice = ''; testOutput = '';
    try {
      const response = await api('/api/solution-prompts/test', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ system_prompt: systemPrompt, markdown: testMarkdown })
      });
      testOutput = response.markdown || '';
    } catch (requestError: any) { error = requestError.message || '测试提示词失败'; }
    finally { action = ''; }
  }

  async function savePublicURL() {
    action = 'public-url'; error = ''; notice = '';
    try {
      const url = new URL(publicURLDraft.trim());
      if (!['http:', 'https:'].includes(url.protocol)) throw new Error('只允许 HTTP 或 HTTPS 地址');
      await onSavePublicURL(url.toString().replace(/\/$/, ''));
      publicURLDraft = url.toString().replace(/\/$/, '');
      showToast('方案公开地址已保存，后续发布会把绝对链接写回 Jira。', { title: '保存成功' });
    } catch (requestError: any) {
      showToast(requestError.message || '保存公开地址失败', { type: 'error', title: '保存失败' });
    }
    finally { action = ''; }
  }

  function formatTime(value: string) {
    return value ? new Date(value).toLocaleString('zh-CN') : '-';
  }
</script>

<div class="prompt-config">
  {#if error}<div class="prompt-message error" role="alert">{error}</div>{/if}
  {#if notice}<div class="prompt-message success" aria-live="polite">{notice}</div>{/if}

  <section class="prompt-overview" aria-label="当前方案治理规则">
    <div class="section-copy"><span>当前规则</span><h2>方案治理提示词</h2><p>润色和两轮对比都绑定确切提示词版本。新版本不会追溯修改历史结果，项目级规则优先于全局规则。</p></div>
    <div class="prompt-metrics">
      <div><span>当前类型版本</span><strong>{purposePrompts.length}</strong></div>
      <div><span>生效规则</span><strong>{activePrompts.length}</strong></div>
      <div><span>项目覆盖</span><strong>{activePrompts.filter((prompt) => prompt.scope_type === 'project').length}</strong></div>
    </div>
  </section>

  <section class="prompt-section" aria-labelledby="solution-public-url-title">
    <div class="section-head"><div><span>链接出口</span><h3 id="solution-public-url-title">Jira 方案链接地址</h3><p>用于发布后写回 Jira 的稳定绝对链接；反向代理部署时应填写用户真实访问地址。</p></div></div>
    <div class="inline-form"><label for="solution-public-url">公开地址</label><input id="solution-public-url" bind:value={publicURLDraft} placeholder="https://ambient.example.com" /><button disabled={!!action || !publicURLDraft.trim()} on:click={savePublicURL}>{action === 'public-url' ? '保存中…' : '保存地址'}</button></div>
  </section>

  <section class="prompt-section" aria-labelledby="solution-prompt-editor-title">
    <div class="section-head"><div><span>新增版本</span><h3 id="solution-prompt-editor-title">编辑与验证</h3><p>评论内容按非可信输入处理。保存时创建新版本，可立即启用或留作草稿。</p></div></div>
    <div class="prompt-form-grid">
      <div class="prompt-select-field"><Select label="规则类型" value={purpose} options={purposeOptions} searchable={false} compact shadowless on:change={(event) => changePurpose(event.detail)} /></div>
      <div class="prompt-select-field"><Select label="作用域" value={scopeType} options={scopeOptions} searchable={false} compact shadowless on:change={(event) => { scopeType = event.detail as 'global' | 'project'; if (scopeType === 'global') scopeID = ''; }} /></div>
      <label>项目 Key<input bind:value={scopeID} disabled={scopeType === 'global'} placeholder="例如 WA" /></label>
      <label class="span-two">版本名称<input bind:value={name} placeholder="例如 方案润色规则 2026-08" /></label>
      <label class="span-two">系统提示词<textarea bind:value={systemPrompt} rows="16" spellcheck="true"></textarea></label>
      <label class="toggle-row span-two"><input type="checkbox" bind:checked={activateOnSave} /><span>保存后立即启用，并将同作用域旧版本标记为已退役</span></label>
    </div>
    <div class="prompt-actions"><button disabled={!!action || !systemPrompt.trim()} on:click={testPrompt}>{action === 'test' ? '测试中…' : '使用示例测试'}</button><button class="primary" disabled={!!action || !systemPrompt.trim() || (scopeType === 'project' && !scopeID.trim())} on:click={savePrompt}>{action === 'save' ? '保存中…' : '保存新版本'}</button></div>
    <div class="test-grid">
      <label>测试 Markdown<textarea bind:value={testMarkdown} rows="9"></textarea></label>
      <label>测试结果<textarea value={testOutput} rows="9" readonly placeholder="点击“使用示例测试”后显示真实模型结果"></textarea></label>
    </div>
  </section>

  <section class="prompt-section" aria-labelledby="solution-prompt-history-title">
    <div class="section-head"><div><span>版本审计</span><h3 id="solution-prompt-history-title">历史规则</h3><p>可以从任意版本建立新版，也可以显式重新启用历史版本。</p></div></div>
    {#if loading}<div class="prompt-empty">正在读取规则版本…</div>
    {:else if prompts.length === 0}<div class="prompt-empty">暂无提示词版本。</div>
    {:else}<div class="prompt-table" role="table" aria-label="方案提示词版本">
      {#each prompts as prompt}
        <div class="prompt-row" role="row">
          <div><span>{purposeOptions.find((item) => item.value === prompt.purpose)?.label || prompt.purpose} / {prompt.scope_type === 'global' ? '全局' : prompt.scope_id}</span><strong>v{prompt.version} · {prompt.name}</strong><small>{formatTime(prompt.created_at)} · {prompt.created_by}</small></div>
          <span class="status {prompt.status}">{prompt.status === 'active' ? '生效中' : prompt.status === 'draft' ? '草稿' : '已退役'}</span>
          <div class="row-actions"><button on:click={() => editFrom(prompt)}>基于此版本编辑</button>{#if prompt.status !== 'active'}<button disabled={!!action} on:click={() => activatePrompt(prompt)}>{action === `activate-${prompt.id}` ? '启用中…' : '启用'}</button>{/if}</div>
        </div>
      {/each}
    </div>{/if}
  </section>
</div>

<style>
  .prompt-config { display:grid; gap:14px; color:var(--wa-text-main,#293847); }
  .prompt-overview,.prompt-section { padding:18px; border:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); border-radius:12px; background:var(--wa-surface,#fff); }
  .prompt-overview { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:20px; align-items:start; }
  .section-copy>span,.section-head span { color:var(--wa-accent-strong,#006f76); font:700 11px/1.2 var(--wa-font-mono,monospace); letter-spacing:.08em; text-transform:uppercase; }
  h2,h3 { margin:5px 0 0; color:var(--wa-text-strong,#0d1722); }
  h2 { font-size:20px; } h3 { font-size:16px; }
  p { max-width:760px; margin:6px 0 0; color:var(--wa-text-muted,#667789); font-size:13px; line-height:1.6; }
  .prompt-metrics { display:flex; gap:8px; }
  .prompt-metrics>div { min-width:92px; display:grid; gap:4px; padding:12px; border-left:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); }
  .prompt-metrics span { color:var(--wa-text-muted,#667789); font-size:11px; } .prompt-metrics strong { color:var(--wa-text-strong,#0d1722); font-size:20px; }
  .prompt-section { display:grid; gap:14px; }
  .inline-form { display:grid; grid-template-columns:100px minmax(0,1fr) auto; gap:10px; align-items:center; }
  .inline-form label,.prompt-form-grid label,.test-grid label { display:grid; gap:6px; color:var(--wa-text-muted,#667789); font-size:12px; font-weight:700; }
  .prompt-select-field { min-width:0; }
  input,textarea { width:100%; box-sizing:border-box; border:1px solid var(--wa-border-strong,rgba(91,119,137,.28)); border-radius:8px; background:var(--wa-surface-flat,#fbfdfe); color:var(--wa-text-main,#293847); font:500 13px/1.55 var(--wa-font-sans,system-ui); }
  input { min-height:42px; padding:0 11px; } textarea { resize:vertical; padding:11px; font-family:var(--wa-font-mono,monospace); }
  input:focus,textarea:focus { outline:2px solid color-mix(in srgb,var(--wa-accent,#008f96) 25%,transparent); border-color:var(--wa-accent,#008f96); }
  button { min-height:40px; padding:0 14px; border:1px solid var(--wa-border-strong,rgba(91,119,137,.28)); border-radius:8px; background:var(--wa-surface,#fff); color:var(--wa-text-main,#293847); font-weight:700; cursor:pointer; }
  button.primary { border-color:var(--wa-accent,#008f96); background:var(--wa-accent,#008f96); color:#fff; } button:disabled { opacity:.48; cursor:not-allowed; }
  .prompt-form-grid,.test-grid { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr); gap:12px; }
  .span-two { grid-column:1/-1; }
  .toggle-row { display:flex!important; grid-template-columns:auto 1fr!important; align-items:center; } .toggle-row input { width:18px; min-height:18px; }
  .prompt-actions { display:flex; justify-content:flex-end; gap:8px; }
  .prompt-table { display:grid; border-top:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); }
  .prompt-row { display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:14px; align-items:center; padding:12px 0; border-bottom:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); }
  .prompt-row>div:first-child { display:grid; gap:3px; min-width:0; } .prompt-row>div:first-child span,.prompt-row small { color:var(--wa-text-muted,#667789); font-size:11px; } .prompt-row strong { overflow:hidden; color:var(--wa-text-strong,#0d1722); text-overflow:ellipsis; white-space:nowrap; }
  .status { padding:5px 8px; border-radius:999px; background:var(--wa-surface-soft,#f4f8f9); color:var(--wa-text-muted,#667789); font-size:11px; } .status.active { background:#e8f7f1; color:#17634f; }
  .row-actions { display:flex; gap:7px; } .row-actions button { min-height:34px; padding:0 10px; }
  .prompt-message,.prompt-empty { padding:12px; border-radius:8px; font-size:13px; } .prompt-message.error { border:1px solid rgba(194,75,88,.24); background:#fff3f4; color:#8c2d39; } .prompt-message.success { border:1px solid rgba(32,133,105,.24); background:#f0fbf7; color:#17634f; } .prompt-empty { background:var(--wa-surface-soft,#f4f8f9); color:var(--wa-text-muted,#667789); }
  @media (max-width:800px) { .prompt-overview { grid-template-columns:1fr; } .prompt-metrics { overflow-x:auto; } .inline-form,.prompt-form-grid,.test-grid,.prompt-row { grid-template-columns:1fr; } .span-two { grid-column:auto; } .row-actions { flex-wrap:wrap; } }
</style>
