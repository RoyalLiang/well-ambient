<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Button from '../shared/Button.svelte';
  import Switch from '../shared/Switch.svelte';
  import Select from '../shared/Select.svelte';
  export let canWrite = false;
  export let canAccess = false;
  export let onCredentials: () => void = () => {};
  type Settings = { version:number; enabled:boolean; time_of_day:string; jira_project:string; group_path:string; target_project_id:string };
  type Repo = { name:string; path:string; project_id:string };
  type Item = { jira_key:string; title:string; description:string; source_status:string; dedupe_key:string };
  type Preview = { id:number; config_version:number; jira_project:string; target_project_id:string; group_path:string; created_at:string; total:number; items:Item[]; truncated:boolean };
  let settings:Settings = {version:1,enabled:false,time_of_day:'09:00',jira_project:'',group_path:'',target_project_id:''};
  let saved = '';
  let projects:string[] = [];
  let repos:Repo[] = [];
  let preview:Preview|null = null;
  let selected = '';
  let busy = '';
  let error = '';
  let notice = '';
  let tokenConfigured = false;
  let alive = true;
  const groupOf=(repo:Repo)=>repo.path?.replace(/^\/+|\/+$/g,'').split('/').slice(0,-1).join('/') || '';
  $: groups=[...new Set(repos.map(groupOf).filter(Boolean))].sort();
  $: targets=repos.filter(repo=>groupOf(repo)===settings.group_path);
  $: dirty=JSON.stringify(settings)!==saved;
  $: ready=Boolean(settings.jira_project&&settings.group_path&&settings.target_project_id);
  $: controlsDisabled = !canWrite || Boolean(busy) || !saved;
  $: projectOptions = projects.map(value => ({ value, label: value }));
  $: groupOptions = groups.map(value => ({ value, label: value }));
  $: targetOptions = targets.map(repo => ({ value: repo.project_id, label: repo.path || repo.name, meta: `ID ${repo.project_id}` }));
  $: hour = settings.time_of_day.split(':')[0] || '09';
  $: minute = settings.time_of_day.split(':')[1] || '00';
  const hours = Array.from({ length: 24 }, (_, index) => ({ value: String(index).padStart(2, '0'), label: String(index).padStart(2, '0') }));
  const minutes = Array.from({ length: 60 }, (_, index) => ({ value: String(index).padStart(2, '0'), label: String(index).padStart(2, '0') }));
  function setTime(part: 'hour' | 'minute', value: string) {
    settings = { ...settings, time_of_day: part === 'hour' ? `${value}:${minute}` : `${hour}:${value}` };
  }
  async function request(url:string,options:RequestInit={}) {
    const res=await fetch(url,options);if(!res.ok)throw new Error(res.status===409?'配置已被其他人修改，请重新加载。':res.status===403?'需要全局超级管理员及对应配置权限。':'请求失败，请检查项目、分组和承载仓库配置后重试。');return res.json();
  }
  async function load(){if(!canAccess)return;busy='load';error='';try{const data=await request('/api/jira-issue-sync');if(!alive)return;settings=data.settings;projects=data.projects||[];repos=data.repositories||[];preview=data.latest;tokenConfigured=data.token_configured;saved=JSON.stringify(settings);}catch(e){if(alive)error=(e as Error).message;}finally{if(alive)busy='';}}
  async function save(){if(!canWrite||busy)return;busy='save';error='';notice='';try{const value=await request('/api/jira-issue-sync',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(settings)});if(!alive)return;settings=value;saved=JSON.stringify(value);notice='配置已保存。';}catch(e){if(alive)error=(e as Error).message;}finally{if(alive)busy='';}}
  async function generate(){if(!canWrite||busy||dirty||!ready)return;busy='preview';error='';notice='';try{const value=await request('/api/jira-issue-sync/preview',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({version:settings.version})});if(alive){preview=value;selected='';notice='草稿已生成。';}}catch(e){if(alive)error=(e as Error).message;}finally{if(alive)busy='';}}
  function setGroup(value:string){settings={...settings,group_path:value,target_project_id:''};}
  function download(){if(!preview)return;const url=URL.createObjectURL(new Blob([JSON.stringify({...preview,mode:'preview_only',remote_verified:false},null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download=`jira-issue-preview-${preview.id}.json`;a.click();URL.revokeObjectURL(url);}
  onMount(()=>{void load();});onDestroy(()=>{alive=false;});
</script>

<section class="sync-config" aria-label="Jira 到 Issue 每日同步预览" aria-busy={Boolean(busy)}>
  <header class="sync-heading">
    <div class="heading-copy">
      <div class="title-line"><h2>同步配置</h2><span class="mode-label">仅生成草稿</span></div>
      <p>将 Jira 记录生成目标仓库的 Issue 草稿。</p>
    </div>
    {#if canAccess}
      <div class="activation">
        <Switch id="jira-issue-preview-enabled" label="自动预览" checked={settings.enabled}
          disabled={controlsDisabled} expandedHitArea
          on:change={(e) => settings = { ...settings, enabled: e.detail }} />
        <span class="activation-state">{!saved ? '未加载' : dirty ? '待保存' : settings.enabled ? '已启用' : '未启用'}</span>
      </div>
    {/if}
  </header>

  {#if !canAccess}
    <p class="feedback" role="status">需要全局超级管理员及配置权限。</p>
  {:else}
    <div class="schedule-toolbar">
      <div class="schedule" role="group" aria-label="自动同步时间">
        <div class="schedule-copy"><h3>自动同步时间</h3><p>每日定时生成草稿</p></div>
        <div class="time-control">
          <div class="time-part"><Select id="jira-issue-hour" ariaLabel="自动同步小时" value={hour} options={hours}
            compact searchable={false} shadowless disabled={controlsDisabled} on:change={(e) => setTime('hour', e.detail)} /></div>
          <span class="time-separator" aria-hidden="true">:</span>
          <div class="time-part"><Select id="jira-issue-minute" ariaLabel="自动同步分钟" value={minute} options={minutes}
            compact searchable={false} shadowless disabled={controlsDisabled} on:change={(e) => setTime('minute', e.detail)} /></div>
          <span class="timezone">北京时间</span>
        </div>
      </div>
      <div class="toolbar-actions">
        <Button variant="ghost" disabled={Boolean(busy)} loading={busy === 'load'} on:click={load}>重新加载</Button>
        <Button variant="secondary" disabled={controlsDisabled || !dirty || !ready} loading={busy === 'save'} on:click={save}>保存配置</Button>
        <Button disabled={controlsDisabled || dirty || !ready} loading={busy === 'preview'} on:click={generate}>启动预览</Button>
      </div>
      <p class="config-state" role="status">{!saved ? (busy === 'load' ? '正在加载配置…' : '配置未加载') : dirty ? '配置已修改，保存后可启动预览。' : `配置 v${settings.version} · 仅预览，不写入远端 Issue`}</p>
    </div>

    {#if error}<div class="feedback error" role="alert">{error}</div>{/if}
    {#if notice}<p class="feedback notice" role="status">{notice}</p>{/if}

    <section class="mapping" aria-labelledby="jira-issue-mapping-title">
      <div class="section-heading"><h3 id="jira-issue-mapping-title">来源与目标</h3><p>选择 Jira 项目和 Issue 承载仓库。</p></div>
      <div class="fields">
        <Select id="jira-issue-project" label="Jira 项目" bind:value={settings.jira_project} options={projectOptions}
          placeholder="选择 Jira 项目" emptyText="暂无已配置项目" compact shadowless disabled={controlsDisabled} />
        <Select id="jira-issue-group" label="目标仓库分组" value={settings.group_path} options={groupOptions}
          placeholder="选择目标分组" emptyText="暂无仓库分组" compact shadowless disabled={controlsDisabled} on:change={(e) => setGroup(e.detail)} />
        <div class="target-field"><Select id="jira-issue-target" label="Issue 承载仓库" bind:value={settings.target_project_id} options={targetOptions}
          placeholder={settings.group_path ? '选择分组内的仓库' : '先选择目标分组'} emptyText="该分组暂无仓库" compact shadowless disabled={controlsDisabled || !settings.group_path} /></div>
      </div>
      {#if saved && !groups.length}<p class="warning">暂无仓库分组，请在 GitLab 配置中拉取仓库。</p>{/if}
      <div class="connection">
        <div class="connection-copy"><strong>GitLab API 凭据</strong><span class="credential-state" class:configured={tokenConfigured}>{tokenConfigured ? '已配置' : '未配置'}</span><p>用于访问 GitLab Issue API。</p></div>
        <Button variant="secondary" disabled={Boolean(busy) || !canWrite} on:click={onCredentials}>{tokenConfigured ? '管理凭据' : '配置凭据'}</Button>
      </div>
    </section>

    <section class="preview" aria-labelledby="jira-issue-preview-title">
      <div class="preview-heading"><div class="section-heading"><h3 id="jira-issue-preview-title">草稿预览</h3><p>审阅并导出生成的 Issue 内容。</p></div>
        {#if preview}<Button variant="secondary" on:click={download}>导出 JSON</Button>{/if}
      </div>
      {#if preview}
        <div class="preview-summary"><strong>{preview.total}<span> 条记录</span></strong><span>展示 {preview.items.length} 条{preview.truncated ? ' · 上限 100 条' : ''}</span><span>{preview.jira_project} → {preview.group_path}</span><time datetime={preview.created_at}>{new Date(preview.created_at).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })}</time></div>
        {#if preview.config_version !== settings.version}<p class="warning" role="status">草稿对应旧配置，请重新生成。</p>{/if}
        <div class="drafts">
          {#each preview.items as item}
            <article>
              <div class="draft-summary"><code>{item.jira_key}</code><h4>{item.title}</h4><span class="source-status">{item.source_status}</span></div>
              <Button variant="ghost" on:click={() => selected = selected === item.jira_key ? '' : item.jira_key}>{selected === item.jira_key ? '收起草稿' : '查看草稿'}</Button>
              {#if selected === item.jira_key}<div class="draft-body"><pre>{item.description}</pre><small>幂等标识 {item.dedupe_key}</small></div>{/if}
            </article>
          {:else}<div class="empty"><h4>暂无匹配记录</h4><p>该项目暂无已同步的 Jira 记录。</p></div>{/each}
        </div>
      {:else}<div class="empty"><h4>尚未生成草稿</h4><p>保存来源与目标后，启动预览。</p></div>{/if}
    </section>
  {/if}
</section>

<style>
  .sync-config { --wa-control-h: 44px; display: grid; gap: 24px; min-width: 0; container-type: inline-size; }
  .sync-heading, .title-line, .activation, .schedule, .time-control, .toolbar-actions, .connection, .preview-heading { display: flex; align-items: center; gap: 12px; }
  .sync-heading, .connection, .preview-heading { justify-content: space-between; }
  .heading-copy, .section-heading, .connection-copy { min-width: 0; }
  .heading-copy, .section-heading { display: grid; gap: 6px; }
  .title-line { flex-wrap: wrap; gap: 10px; }
  h2, h3, h4, p { margin: 0; }
  h2 { color: var(--wa-text-strong); font-size: 18px; font-weight: 750; line-height: 1.4; }
  h3 { color: var(--wa-text-strong); font-size: 14px; font-weight: 750; line-height: 1.5; }
  h4 { color: var(--wa-text-main); font-size: 14px; font-weight: 650; line-height: 1.5; overflow-wrap: anywhere; }
  p, .timezone, .config-state, .activation-state, .preview-summary, .source-status { color: var(--wa-text-muted); font-size: 13px; line-height: 1.6; }
  .mode-label { color: var(--wa-accent-strong); background: var(--wa-accent-soft); border-radius: var(--wa-radius-sm); padding: 3px 8px; font-size: 11px; font-weight: 650; white-space: nowrap; }
  .activation { flex-shrink: 0; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
  .activation-state { font-size: 12px; }
  .schedule-toolbar { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px 24px; padding-block: 20px; border-block: 1px solid var(--wa-border-divider); }
  .schedule { gap: 24px; flex-wrap: wrap; }
  .schedule-copy { flex-shrink: 0; }
  .schedule-copy p { font-size: 12px; }
  .time-control { gap: 8px; font-variant-numeric: tabular-nums; }
  .time-part { width: 76px; min-width: 0; }
  .sync-config .time-part :global(.select-trigger),
  .sync-config .fields :global(.select-trigger) { min-height: 44px; }
  .sync-config .fields :global(.select-inline-input),
  .sync-config .fields :global(.select-inline-input:focus) { min-height: 42px; border: 0 !important; border-radius: 0 !important; background: transparent !important; box-shadow: none !important; }
  .sync-config .activation :global(button.switch-control),
  .sync-config .activation :global(button.switch-control.checked) { width: 44px !important; height: 44px !important; min-height: 44px; background: transparent !important; box-shadow: none !important; }
  .sync-config .activation :global(.switch-thumb) { top: 13px; }
  .time-separator { color: var(--wa-text-muted); font-weight: 750; }
  .timezone { white-space: nowrap; margin-left: 4px; }
  .toolbar-actions { flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
  .config-state { grid-column: 1 / -1; font-size: 12px; }
  .feedback { padding: 12px 16px; border-radius: var(--wa-radius-md); background: var(--wa-surface-inset); }
  .error { color: var(--wa-danger); background: var(--wa-danger-soft); }
  .notice { color: var(--wa-accent-strong); background: var(--wa-accent-soft); }
  .mapping { display: grid; gap: 16px; min-width: 0; }
  .fields { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 16px; }
  .target-field { grid-column: 1 / -1; min-width: 0; }
  .connection { padding-top: 4px; gap: 16px; }
  .connection-copy { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 12px; }
  .connection strong { font-size: 13px; color: var(--wa-text-main); }
  .credential-state { display: inline-flex; align-items: center; gap: 6px; color: var(--wa-text-muted); font-size: 12px; }
  .credential-state::before { content: ''; width: 6px; height: 6px; border-radius: 50%; background: var(--wa-text-muted); }
  .credential-state.configured::before { background: var(--wa-accent-strong); }
  .preview { display: grid; gap: 16px; padding-top: 24px; border-top: 1px solid var(--wa-border-divider); min-width: 0; }
  .preview-summary { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 20px; padding-block: 12px; border-bottom: 1px solid var(--wa-border-divider); overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
  .preview-summary strong { color: var(--wa-text-strong); font-size: 18px; font-weight: 750; }
  .preview-summary strong span { color: var(--wa-text-muted); font-size: 12px; font-weight: 500; }
  .preview-summary time { margin-left: auto; font-size: 12px; }
  .warning { color: var(--wa-warning); font-size: 13px; }
  .drafts article { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; padding-block: 16px; border-bottom: 1px solid var(--wa-border-divider); }
  .draft-summary { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: baseline; gap: 12px; flex: 1; min-width: 0; }
  .draft-summary code { color: var(--wa-accent-strong); background: var(--wa-accent-soft); padding: 3px 7px; border-radius: var(--wa-radius-sm); font-size: 12px; white-space: nowrap; }
  .source-status { font-size: 12px; overflow-wrap: anywhere; }
  .draft-body { flex-basis: 100%; min-width: 0; padding: 16px; background: var(--wa-surface-inset); border-radius: var(--wa-radius-md); }
  .draft-body pre { margin: 0 0 12px; color: var(--wa-text-main); white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; font-size: 13px; line-height: 1.7; }
  .draft-body small { font-size: 12px; color: var(--wa-text-muted); overflow-wrap: anywhere; }
  .empty { display: grid; gap: 6px; padding-block: 24px; text-align: center; }
  @container (max-width: 1000px) { .schedule-toolbar { grid-template-columns: minmax(0, 1fr); } }
  @container (max-width: 600px) {
    .sync-heading { align-items: flex-start; }
    .activation { max-width: 144px; }
    .activation-state { flex-basis: 100%; text-align: right; }
    .schedule { justify-content: space-between; gap: 12px; }
    .fields { grid-template-columns: minmax(0, 1fr); }
    .target-field { grid-column: auto; }
    .connection { align-items: flex-start; }
    .connection-copy p { flex-basis: 100%; }
    .draft-summary { grid-template-columns: auto minmax(0, 1fr); }
    .source-status { grid-column: 2; }
    .preview-summary time { margin-left: 0; flex-basis: 100%; }
  }
  @container (max-width: 360px) {
    .sync-heading { gap: 8px; }
    .title-line { gap: 6px; }
    .activation { max-width: 130px; }
    .connection { flex-direction: column; align-items: stretch; }
    .connection > :global(button) { align-self: flex-end; }
    .draft-summary { flex-basis: 100%; }
    .drafts article > :global(button) { margin-left: auto; }
  }
</style>
