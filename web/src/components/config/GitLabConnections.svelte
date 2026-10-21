<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import GitLabConfig from './GitLabConfig.svelte';
  export let config: any = {enabled:false,base_url:'',api_token:'',secret_token:'',repos:[],connections:[]};
  export let saving=false;
  export let saveError='';
  export let saveSuccess=false;
  export let lastUpdated='';
  const dispatch=createEventDispatcher();
  let selected='';
  let editing=false;
  $: connections=config.connections || [];
  $: active=selected ? connections.find((item:any)=>item.id===selected) : config;
  $: selectedRepos=(config.repos || []).filter((repo:any)=>(repo.instance_id || '')===selected);
  $: view={...active,repos:selectedRepos};
  function save(event:CustomEvent<any>){
    const {repos,...values}=event.detail.data;
    const normalized=(repos || []).map((repo:any)=>{
      if(!selected)return repo;
      const remote=repo.remote_project_id || repo.project_id.replace(`${selected}:`,'');
      return {...repo,instance_id:selected,remote_project_id:remote,project_id:`${selected}:${remote}`};
    });
    const next={...config,repos:[...(config.repos || []).filter((repo:any)=>(repo.instance_id || '')!==selected),...normalized]};
    if(selected)next.connections=connections.map((item:any)=>item.id===selected ? {...item,...values} : item);
    else Object.assign(next,values);
    dispatch('save',{...event.detail,data:next});
  }
</script>

<section class="connection-selection" aria-label="GitLab 实例选择">
  <label for="gitlab-instance-selection">GitLab 实例</label>
  <select id="gitlab-instance-selection" bind:value={selected} disabled={saving || editing}>
    <option value="">默认实例 · {config.base_url || '尚未配置'}</option>
    {#each connections as connection}<option value={connection.id}>{connection.name || connection.id} · {connection.base_url}</option>{/each}
  </select>
  <p>每个实例独立保存 API Token、回调密钥与仓库清单。切换实例不会复用另一实例的令牌。{#if editing}请先保存或取消当前编辑，再切换实例。{/if}</p>
  {#if active?.root_group}<div class="connection-scope"><span>接入分组</span><strong>{active.root_group}</strong><span>包含子分组 · {selectedRepos.length} 个仓库</span></div>{/if}
</section>
{#key selected}
  <GitLabConfig config={view} instanceID={selected} rootGroup={active?.root_group || ''} {saving} {saveError} {saveSuccess} {lastUpdated} on:save={save} on:close={()=>dispatch('close')} on:editing={(event)=>editing=event.detail} />
{/key}

<style>
  .connection-selection{display:grid;grid-template-columns:minmax(0,1fr);gap:8px;padding-bottom:20px;margin-bottom:20px;border-bottom:1px solid var(--wa-border-soft);color:var(--wa-text-main);}
  label{font-size:12px;font-weight:600;color:var(--wa-text-strong);}
  select{width:100%;max-width:720px;min-width:0;min-height:40px;padding:9px 12px;background:var(--wa-surface-panel);color:var(--wa-text-main);border:1px solid var(--wa-border-soft);border-radius:9px;font:inherit;color-scheme:light;}
  select:focus-visible{outline:2px solid var(--wa-accent-strong);outline-offset:2px;}
  p{margin:0;color:var(--wa-text-muted);font-size:12px;line-height:1.6;}
  .connection-scope{display:flex;flex-wrap:wrap;gap:8px 16px;font-size:12px;color:var(--wa-text-muted);margin-top:8px;}.connection-scope strong{color:var(--wa-text-strong);font-weight:600;}
  @media(max-width:760px){select{min-height:44px;}.connection-scope{display:grid;gap:6px;}}
</style>
