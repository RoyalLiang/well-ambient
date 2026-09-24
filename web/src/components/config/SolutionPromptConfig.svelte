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
    content_hash: string; validation_status: 'untested' | 'passed' | 'failed';
    validation_summary?: string; validated_by?: string; validated_at?: string;
    created_by: string; activated_by: string; created_at: string; activated_at?: string;
  };

  type PromptPurpose = 'solution_polish' | 'solution_compare_requirement' | 'solution_compare_compatibility' | 'code_review';
  type PromptDraft = {
    scopeType: 'global' | 'project'; scopeID: string; name: string;
    systemPrompt: string; activateOnSave: boolean; testMarkdown: string;
  };

  const purposeOptions = [
    { value: 'solution_polish', label: '方案润色', meta: '需求方案生成与整理' },
    { value: 'solution_compare_requirement', label: '需求等价性对比', meta: '两轮对比的第一轮' },
    { value: 'solution_compare_compatibility', label: '方案兼容性对比', meta: '两轮对比的第二轮' },
    { value: 'code_review', label: '代码评审', meta: '证据驱动的两轮代码评审' }
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
  let drafts: Partial<Record<PromptPurpose, PromptDraft>> = {};
  let publicURLDraft = '';
  let publicURLSource = '';

  $: purposePrompts = prompts.filter((prompt) => prompt.purpose === purpose);
  $: activePurposePrompts = purposePrompts.filter((prompt) => prompt.status === 'active');
  $: isCodeReview = purpose === 'code_review';
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
    activateOnSave = false;
    notice = `已载入 ${prompt.scope_type === 'global' ? '全局' : prompt.scope_id} v${prompt.version}，保存会新增版本。`;
  }

  function changePurpose(nextPurpose: string) {
    drafts = {
      ...drafts,
      [purpose]: { scopeType, scopeID, name, systemPrompt, activateOnSave, testMarkdown }
    };
    purpose = nextPurpose as PromptPurpose;
    const nextIsCodeReview = purpose === 'code_review';
    const saved = drafts[purpose];
    const active = prompts.find((prompt) => prompt.purpose === purpose && prompt.scope_type === 'global' && prompt.status === 'active');
    scopeType = saved?.scopeType || 'global';
    scopeID = saved?.scopeID || '';
    name = saved?.name || '';
    systemPrompt = saved?.systemPrompt || active?.system_prompt || '';
    activateOnSave = nextIsCodeReview ? false : (saved?.activateOnSave ?? true);
    testMarkdown = saved?.testMarkdown || '# 示例需求方案\n\n## 目标\n\n- 验证提示词输出结构';
    if (nextIsCodeReview) {
      scopeType = 'global';
      scopeID = '';
      activateOnSave = false;
    }
    testOutput = '';
  }

  async function savePrompt() {
    action = 'save'; error = ''; notice = '';
    try {
      const response = await api('/api/solution-prompts', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          purpose,
          scope_type: isCodeReview ? 'global' : scopeType,
          scope_id: isCodeReview ? '' : scopeID,
          name,
          system_prompt: systemPrompt,
          activate: isCodeReview ? false : activateOnSave
        })
      });
      showToast(`技能版本 v${response.prompt.version} 已${isCodeReview || !activateOnSave ? '保存为草稿' : '保存并启用'}。历史版本未被覆盖。`, {
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
    if (prompt.purpose === 'code_review' && prompt.validation_status !== 'passed') {
      error = '代码评审技能必须先通过运行验证。';
      return;
    }
    if (!window.confirm(`确认将 ${prompt.name} v${prompt.version} 设为生效版本？仅影响后续新任务，历史运行保持原版本。`)) return;
    action = `activate-${prompt.id}`; error = ''; notice = '';
    try {
      await api(`/api/solution-prompts/${prompt.id}/activate`, { method: 'POST' });
      notice = `${prompt.scope_type === 'global' ? '全局' : prompt.scope_id} v${prompt.version} 已启用。`;
      await loadPrompts();
    } catch (requestError: any) { error = requestError.message || '启用提示词失败'; }
    finally { action = ''; }
  }

  async function testStoredPrompt(prompt: PromptVersion) {
    action = `test-${prompt.id}`; error = ''; notice = '';
    try {
      const response = await api(`/api/solution-prompts/${prompt.id}/test`, { method: 'POST' });
      notice = `${prompt.name} v${prompt.version} 已通过真实代码评审运行验证。`;
      testOutput = JSON.stringify(response.result || {}, null, 2);
      await loadPrompts();
    } catch (requestError: any) {
      error = requestError.message || '代码评审技能验证失败';
      await loadPrompts();
    } finally { action = ''; }
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

  <section class="prompt-overview" aria-label="在线 AI 技能治理">
    <div class="section-copy">
      <h2>AI 运行技能治理</h2>
      <p>每次运行绑定确切技能版本；启用只影响新任务，历史结果保持不变。</p>
      <div class="skill-selector"><Select label="在线技能" value={purpose} options={purposeOptions} searchable={false} compact shadowless on:change={(event) => changePurpose(event.detail)} /></div>
    </div>
    <div class="prompt-metrics">
      <div><span>当前技能版本</span><strong>{purposePrompts.length}</strong></div>
      <div><span>当前生效版本</span><strong>{activePurposePrompts.length}</strong></div>
      <div><span>作用域覆盖</span><strong>{purposePrompts.filter((prompt) => prompt.scope_type === 'project').length}</strong></div>
    </div>
  </section>

  {#if !isCodeReview}
    <section class="prompt-section" aria-labelledby="solution-public-url-title">
      <div class="section-head"><div><span>方案发布</span><h3 id="solution-public-url-title">Jira 方案链接地址</h3><p>用于发布后写回 Jira 的稳定绝对链接；反向代理部署时应填写用户真实访问地址。</p></div></div>
      <div class="inline-form"><label for="solution-public-url">公开地址</label><input id="solution-public-url" bind:value={publicURLDraft} placeholder="https://ambient.example.com" /><button disabled={!!action || !publicURLDraft.trim()} on:click={savePublicURL}>{action === 'public-url' ? '保存中…' : '保存地址'}</button></div>
    </section>
  {/if}

  <section class="prompt-section" aria-labelledby="solution-prompt-editor-title">
    <div class="section-head"><div><span>版本编辑</span><h3 id="solution-prompt-editor-title">编辑技能指令</h3><p>运行输入按不可信资料处理；保存会创建新版本，不覆盖历史。</p></div></div>
    <div class="prompt-form-grid">
      <div class="prompt-select-field"><Select label="作用域" value={isCodeReview ? 'global' : scopeType} options={scopeOptions} searchable={false} compact shadowless disabled={isCodeReview} on:change={(event) => { scopeType = event.detail as 'global' | 'project'; if (scopeType === 'global') scopeID = ''; }} /></div>
      {#if !isCodeReview}<label>项目 Key<input bind:value={scopeID} disabled={scopeType === 'global'} placeholder="例如 WA" /></label>{/if}
      <label class="span-two">版本名称<input bind:value={name} placeholder={isCodeReview ? '例如 代码评审技能 2026-09' : '例如 方案润色规则 2026-09'} /></label>
      <label class="span-two">系统指令<textarea bind:value={systemPrompt} rows="16" spellcheck="true"></textarea></label>
      {#if isCodeReview}
        <p class="activation-note span-two">代码评审技能先保存为草稿；对该版本运行真实两轮评审验证通过后，才能设为生效版本。</p>
      {:else}
        <label class="toggle-row span-two"><input type="checkbox" bind:checked={activateOnSave} /><span>保存后立即启用，并将同作用域旧版本标记为已退役</span></label>
      {/if}
    </div>
    <div class="prompt-actions">{#if !isCodeReview}<button disabled={!!action || !systemPrompt.trim()} on:click={testPrompt}>{action === 'test' ? '测试中…' : '使用示例测试'}</button>{/if}<button class="primary" disabled={!!action || !systemPrompt.trim() || (!isCodeReview && scopeType === 'project' && !scopeID.trim())} on:click={savePrompt}>{action === 'save' ? '保存中…' : '保存新版本'}</button></div>
    {#if !isCodeReview}
      <div class="test-grid">
        <label>测试 Markdown<textarea bind:value={testMarkdown} rows="9"></textarea></label>
        <label>测试结果<textarea value={testOutput} rows="9" readonly placeholder="点击“使用示例测试”后显示真实模型结果"></textarea></label>
      </div>
    {:else if testOutput}
      <label>最近验证结果<textarea value={testOutput} rows="9" readonly></textarea></label>
    {/if}
  </section>

  <section class="prompt-section" aria-labelledby="solution-prompt-history-title">
    <div class="section-head"><div><span>版本审计</span><h3 id="solution-prompt-history-title">版本与最近启用记录</h3><p>可以从任意版本建立新版；设为生效版本只影响后续新任务。</p></div></div>
    {#if loading}<div class="prompt-empty">正在读取规则版本…</div>
    {:else if purposePrompts.length === 0}<div class="prompt-empty">暂无技能版本。</div>
    {:else}<div class="prompt-table" role="table" aria-label="在线 AI 技能版本">
      {#each purposePrompts as prompt}
        <div class="prompt-row" role="row">
          <div role="cell"><span>{prompt.scope_type === 'global' ? '全局默认' : prompt.scope_id}</span><strong>v{prompt.version} · {prompt.name}</strong><small>创建：{formatTime(prompt.created_at)} · {prompt.created_by}</small><small>最近启用：{prompt.activated_at ? `${formatTime(prompt.activated_at)} · ${prompt.activated_by || '-'}` : '未启用'}</small>{#if prompt.purpose === 'code_review'}<small>验证：{prompt.validation_status === 'passed' ? '已通过' : prompt.validation_status === 'failed' ? '未通过' : '未验证'}</small>{/if}</div>
          <span role="cell" class="status {prompt.status}">{prompt.status === 'active' ? '生效中' : prompt.status === 'draft' ? '草稿' : '已退役'}</span>
          <div role="cell" class="row-actions"><button on:click={() => editFrom(prompt)}>基于此版本编辑</button>{#if prompt.purpose === 'code_review' && prompt.validation_status !== 'passed'}<button disabled={!!action} on:click={() => testStoredPrompt(prompt)}>{action === `test-${prompt.id}` ? '验证中…' : '运行验证'}</button>{/if}{#if prompt.status !== 'active'}<button disabled={!!action || (prompt.purpose === 'code_review' && prompt.validation_status !== 'passed')} on:click={() => activatePrompt(prompt)}>{action === `activate-${prompt.id}` ? '启用中…' : '设为生效版本'}</button>{/if}</div>
        </div>
      {/each}
    </div>{/if}
  </section>
</div>

<style>
  .prompt-config { display:grid; gap:14px; color:var(--wa-text-main,#293847); }
  .prompt-overview,.prompt-section { padding:18px; border:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); border-radius:12px; background:var(--wa-surface,#fff); }
  .prompt-overview { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:20px; align-items:start; }
  .section-head span { color:var(--wa-accent-strong,#006f76); font:700 11px/1.2 var(--wa-font-mono,monospace); letter-spacing:.08em; text-transform:uppercase; }
  h2,h3 { margin:5px 0 0; color:var(--wa-text-strong,#0d1722); }
  h2 { font-size:20px; } h3 { font-size:16px; }
  p { max-width:760px; margin:6px 0 0; color:var(--wa-text-muted,#667789); font-size:13px; line-height:1.6; }
  .prompt-metrics { display:flex; gap:8px; }
  .skill-selector { width:min(360px,100%); margin-top:14px; }
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
  .activation-note { margin:0; padding:10px 12px; border:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); border-radius:8px; background:var(--wa-surface-soft,#f4f8f9); }
  .prompt-actions { display:flex; justify-content:flex-end; gap:8px; }
  .prompt-table { display:grid; border-top:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); }
  .prompt-row { display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:14px; align-items:center; padding:12px 0; border-bottom:1px solid var(--wa-border-soft,rgba(123,143,160,.18)); }
  .prompt-row>div:first-child { display:grid; gap:3px; min-width:0; } .prompt-row>div:first-child span,.prompt-row small { color:var(--wa-text-muted,#667789); font-size:11px; } .prompt-row strong { overflow:hidden; color:var(--wa-text-strong,#0d1722); text-overflow:ellipsis; white-space:nowrap; }
  .status { padding:5px 8px; border-radius:999px; background:var(--wa-surface-soft,#f4f8f9); color:var(--wa-text-muted,#667789); font-size:11px; } .status.active { background:#e8f7f1; color:#17634f; }
  .row-actions { display:flex; gap:7px; } .row-actions button { min-height:34px; padding:0 10px; }
  .prompt-message,.prompt-empty { padding:12px; border-radius:8px; font-size:13px; } .prompt-message.error { border:1px solid rgba(194,75,88,.24); background:#fff3f4; color:#8c2d39; } .prompt-message.success { border:1px solid rgba(32,133,105,.24); background:#f0fbf7; color:#17634f; } .prompt-empty { background:var(--wa-surface-soft,#f4f8f9); color:var(--wa-text-muted,#667789); }
  @media (max-width:800px) { .prompt-overview { grid-template-columns:1fr; } .prompt-metrics { overflow-x:auto; } .inline-form,.prompt-form-grid,.test-grid,.prompt-row { grid-template-columns:1fr; } .span-two { grid-column:auto; } .row-actions { flex-wrap:wrap; } }
  @media (max-width:760px) { input,button { min-height:44px; } .row-actions { display:grid; grid-template-columns:1fr; } .row-actions button,.prompt-actions button { width:100%; min-height:44px; } .prompt-actions { display:grid; grid-template-columns:1fr; } .toggle-row { min-height:44px; } }
</style>
