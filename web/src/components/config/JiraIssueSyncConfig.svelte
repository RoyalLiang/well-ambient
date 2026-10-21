<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Button from '../shared/Button.svelte';
  import Switch from '../shared/Switch.svelte';
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
  async function request(url:string,options:RequestInit={}) {
    const res=await fetch(url,options);if(!res.ok)throw new Error(res.status===409?'配置已被其他人修改，请重新加载。':res.status===403?'需要全局超级管理员及对应配置权限。':'请求失败，请检查项目、分组和承载仓库配置后重试。');return res.json();
  }
  async function load(){if(!canAccess)return;busy='load';error='';try{const data=await request('/api/jira-issue-sync');if(!alive)return;settings=data.settings;projects=data.projects||[];repos=data.repositories||[];preview=data.latest;tokenConfigured=data.token_configured;saved=JSON.stringify(settings);}catch(e){if(alive)error=(e as Error).message;}finally{if(alive)busy='';}}
  async function save(){if(!canWrite||busy)return;busy='save';error='';notice='';try{const value=await request('/api/jira-issue-sync',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(settings)});if(!alive)return;settings=value;saved=JSON.stringify(value);notice='配置已保存。每日任务仅生成预览，不修改远端 Issue。';}catch(e){if(alive)error=(e as Error).message;}finally{if(alive)busy='';}}
  async function generate(){if(!canWrite||busy||dirty||!ready)return;busy='preview';error='';notice='';try{const value=await request('/api/jira-issue-sync/preview',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({version:settings.version})});if(alive){preview=value;selected='';notice='预览已生成，远端未发生写入。';}}catch(e){if(alive)error=(e as Error).message;}finally{if(alive)busy='';}}
  function setGroup(value:string){settings={...settings,group_path:value,target_project_id:''};}
  function download(){if(!preview)return;const url=URL.createObjectURL(new Blob([JSON.stringify({...preview,mode:'preview_only',remote_verified:false},null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download=`jira-issue-preview-${preview.id}.json`;a.click();URL.revokeObjectURL(url);}
  onMount(()=>{void load();});onDestroy(()=>{alive=false;});
</script>

<section class="sync-config" aria-label="Jira 到 Issue 每日同步预览">
  <header><div><h2>Jira → Issue 每日同步 <span class="wa-admin-pill">预览版</span></h2><p>独立配置项目与目标分组，审阅 Issue 草稿后再决定正式同步范围。</p></div><Button variant="secondary" disabled={Boolean(busy)} on:click={load}>重新加载</Button></header>
  {#if !canAccess}<p class="warning">此配置需要全局超级管理员身份。</p>{:else}
    {#if error}<div class="warning" role="alert">{error}</div>{/if}{#if notice}<p class="notice" role="status">{notice}</p>{/if}
    <div class="connection"><div><strong>GitLab API 凭据</strong><p>{tokenConfigured?'已配置 · 可在连接配置中重新录入':'尚未配置 · 本地草稿预览仍可使用'}。Issue 接口使用 API Token；SSH Key 用于 Git 传输，不能代替 Issue API 凭据。</p></div><Button variant="secondary" on:click={onCredentials}>重新设置 GitLab Token</Button></div>
    <form on:submit|preventDefault={save}>
      <fieldset disabled={Boolean(busy)||!canWrite}><legend>来源与目标</legend><div class="fields"><label><span>Jira 项目</span><select bind:value={settings.jira_project}><option value="">选择已配置的项目</option>{#each projects as project}<option value={project}>{project}</option>{/each}</select></label><label><span>目标仓库分组</span><select value={settings.group_path} on:change={(e)=>setGroup(e.currentTarget.value)}><option value="">选择仓库命名空间</option>{#each groups as group}<option value={group}>{group}</option>{/each}</select></label><label><span>Issue 承载仓库</span><select bind:value={settings.target_project_id} disabled={!settings.group_path}><option value="">选择分组内的仓库</option>{#each targets as repo}<option value={repo.project_id}>{repo.path || repo.name} · {repo.project_id}</option>{/each}</select></label></div><p>Issue 归属于具体仓库，分组用于汇总。每条映射明确一个承载仓库，避免向分组内所有仓库重复建单。</p>{#if !groups.length}<p class="warning">现有仓库缺少带命名空间的路径。请在 GitLab 配置中重新拉取仓库并核对路径。</p>{/if}</fieldset>
      <fieldset disabled={Boolean(busy)||!canWrite}><legend>每日预览计划</legend><div class="schedule"><label><span>执行时间 · 北京时间</span><input type="time" bind:value={settings.time_of_day} required /></label><Switch id="jira-issue-preview-enabled" label="启用每日预览" checked={settings.enabled} disabled={!canWrite||Boolean(busy)} on:change={(e)=>settings={...settings,enabled:e.detail}} /></div><p>每天到期后从本系统已同步的 Jira 生成一次预览；不触发“每日 Jira”功能，不创建、更新或关闭远端 Issue。首次保存默认停用。</p></fieldset>
      <div class="form-actions"><span>{dirty?'有未保存的配置修改':`配置 v${settings.version} · ${settings.enabled?'每日预览已启用':'每日预览未启用'}`}</span><Button type="submit" loading={busy==='save'} disabled={!canWrite||Boolean(busy)||!dirty}>保存配置</Button><Button variant="secondary" loading={busy==='preview'} disabled={!canWrite||Boolean(busy)||dirty||!ready} on:click={generate}>生成预览</Button></div>
    </form>
    <section class="preview" aria-labelledby="jira-issue-preview-title"><div class="preview-heading"><div><h3 id="jira-issue-preview-title">待审阅的 Issue 草稿</h3><p>当前不连接远端核对既有 Issue，因此不将草稿标记为“待创建”或“已同步”。</p></div>{#if preview}<Button variant="secondary" on:click={download}>下载预览 JSON</Button>{/if}</div>
      {#if preview}<p class="preview-meta">预览 #{preview.id} · 配置 v{preview.config_version} · {preview.jira_project} → {preview.group_path} / 仓库 {preview.target_project_id} · {new Date(preview.created_at).toLocaleString('zh-CN')}</p>{#if preview.config_version!==settings.version}<p class="warning">此预览对应旧版配置，请重新生成后审阅。</p>{/if}<p role="status">匹配 {preview.total} 条，展示 {preview.items.length} 条{preview.truncated?' · 已达到 100 条预览上限':''}</p>
        <div class="drafts">{#each preview.items as item}<article><div><code>{item.jira_key}</code><h4>{item.title}</h4><p>来源状态 {item.source_status} · 等待远端对账</p></div><Button variant="secondary" size="small" on:click={()=>selected=selected===item.jira_key?'':item.jira_key}>{selected===item.jira_key?'收起草稿':'查看草稿'}</Button>{#if selected===item.jira_key}<div class="draft-body"><pre>{item.description}</pre><small>幂等标识 {item.dedupe_key}</small></div>{/if}</article>{:else}<p class="empty">该项目暂无已同步 Jira。上游同步完成后可重新生成预览。</p>{/each}</div>
      {:else}<div class="empty"><h4>尚未生成预览</h4><p>选择项目、分组和承载仓库，保存配置后生成第一份草稿。</p></div>{/if}
    </section>
  {/if}
</section>

<style>
  .sync-config{display:grid;gap:24px;min-width:0}header,.connection,.form-actions,.preview-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}h2{font-size:18px;margin:0 0 8px}h3{font-size:16px;margin:0 0 6px}h4{font-size:14px;margin:6px 0}p{font-size:13px;line-height:1.6;margin:0;color:var(--wa-text-muted)}.connection{padding-block:18px;border-block:1px solid var(--wa-border-divider)}.connection strong{font-size:13px}.fields{display:grid;grid-template-columns:1fr 1fr;gap:16px}.fields label:last-child{grid-column:1/-1}label{display:grid;gap:6px;font-size:12px;font-weight:650}input,select{width:100%;min-height:38px;padding:8px 12px;border:1px solid var(--wa-border-panel);border-radius:var(--wa-radius-md);background:var(--wa-surface-panel);color:var(--wa-text-main);font:inherit}input:focus-visible,select:focus-visible{outline:2px solid var(--wa-border-focus);outline-offset:2px}form{display:grid;gap:24px}fieldset{border:0;margin:0;padding:0;min-width:0;display:grid;gap:12px}legend{font-weight:750;font-size:14px;margin-bottom:14px}.schedule{display:flex;gap:28px;align-items:end}.schedule label{max-width:220px}.form-actions{border-top:1px solid var(--wa-border-divider);padding-top:18px;justify-content:flex-end}.form-actions>span{margin-right:auto;color:var(--wa-text-muted);font-size:12px}.warning{color:var(--wa-warning);font-size:13px}.notice{color:var(--wa-success)}.preview{display:grid;gap:16px;border-top:1px solid var(--wa-border-divider);padding-top:24px}.preview-meta{overflow-wrap:anywhere}.drafts article{display:flex;align-items:center;gap:16px;flex-wrap:wrap;padding-block:16px;border-top:1px solid var(--wa-border-divider)}.drafts article>div:first-child{flex:1;min-width:0}.drafts h4{overflow-wrap:anywhere}.draft-body{flex-basis:100%;padding-top:12px}.draft-body pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:13px;line-height:1.6;font-family:inherit}.draft-body small{font-size:12px;color:var(--wa-text-muted);overflow-wrap:anywhere}.empty{padding:24px 0}.wa-admin-pill{vertical-align:middle;font-size:11px}
  @media(max-width:760px){header,.connection,.form-actions,.preview-heading,.schedule{align-items:stretch;flex-direction:column}.fields{grid-template-columns:1fr}.fields label:last-child{grid-column:auto}.schedule label{max-width:none}input,select{min-height:44px}.form-actions>span{margin:0}.drafts article{align-items:stretch;flex-direction:column}.draft-body{flex-basis:auto}}
</style>
