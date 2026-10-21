<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Button from '../shared/Button.svelte';
  export let repos: Array<{name:string;path:string;project_id:string;remote_project_id?:string;namespace_name?:string;namespace_path?:string}> = [];
  export let actionLabel = '';
  export let label = '仓库清单';
  const dispatch = createEventDispatcher();
  let query = '';
  let group = '';
  let page = 1;
  const pageSize = 25;
  const namespace = (repo:typeof repos[number]) => repo.namespace_path || repo.path.split('/').slice(0,-1).join('/');
  $: groups = [...new Set(repos.map(namespace))].sort();
  $: filtered = repos.filter(repo => (!group || namespace(repo)===group) && [repo.name,repo.path,repo.project_id,repo.namespace_name].join(' ').toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
  $: pages = Math.max(1,Math.ceil(filtered.length/pageSize));
  $: if (page>pages) page=pages;
  $: visible = filtered.slice((page-1)*pageSize,page*pageSize);
</script>

<section class="repository-directory" aria-label={label}>
  <div class="directory-filters">
    <label>查找仓库<input type="search" placeholder="完整路径、仓库名或 ID" bind:value={query} on:input={()=>page=1} /></label>
    <label>项目分组<select bind:value={group} on:change={()=>page=1}><option value="">全部分组</option>{#each groups as path}<option value={path}>{path || '未分组'}</option>{/each}</select></label>
    <span class="directory-count" role="status">{filtered.length} / {repos.length} 个仓库</span>
  </div>
  <div class="directory-table">
    <table>
      <thead><tr><th scope="col">完整仓库路径</th><th scope="col">项目分组</th><th scope="col">远端 ID / 系统标识</th>{#if actionLabel}<th scope="col">操作</th>{/if}</tr></thead>
      <tbody>
        {#each visible as repo (repo.project_id)}
          <tr><td><strong>{repo.path || repo.name}</strong><span>{repo.name}</span></td><td>{repo.namespace_name || namespace(repo) || '未分组'}{#if repo.namespace_name}<span>{namespace(repo)}</span>{/if}</td><td><code>{repo.remote_project_id || repo.project_id}</code>{#if repo.remote_project_id}<span>{repo.project_id}</span>{/if}</td>{#if actionLabel}<td><Button size="small" variant="secondary" ariaLabel={`${actionLabel} ${repo.path}`} on:click={()=>dispatch('action',repo)}>{actionLabel}</Button></td>{/if}</tr>
        {/each}
      </tbody>
    </table>
    {#if filtered.length===0}<p class="directory-empty">没有匹配的仓库。可更换项目分组或清空搜索条件。</p>{/if}
  </div>
  <footer><span>第 {page} / {pages} 页 · 每页 {pageSize} 个</span><div><Button size="small" variant="secondary" disabled={page<=1} on:click={()=>page-=1}>上一页</Button><Button size="small" variant="secondary" disabled={page>=pages} on:click={()=>page+=1}>下一页</Button></div></footer>
</section>

<style>
  .repository-directory{min-width:0;color:var(--wa-text-main);}
  .directory-filters{display:grid;grid-template-columns:minmax(0,1.3fr) minmax(0,1fr) auto;gap:16px;align-items:end;margin:16px 0;}
  label{display:grid;gap:8px;min-width:0;font-size:12px;font-weight:600;color:var(--wa-text-strong);}
  input,select{box-sizing:border-box;width:100%;min-width:0;min-height:40px;padding:9px 12px;border:1px solid var(--wa-border-soft);border-radius:9px;background:var(--wa-surface-panel);color:var(--wa-text-main);font:inherit;color-scheme:light;}
  input::placeholder{color:var(--wa-text-muted);opacity:1;}
  input:focus-visible,select:focus-visible{outline:2px solid var(--wa-accent-strong);outline-offset:2px;}
  .directory-count,footer,td span{color:var(--wa-text-muted);font-size:12px;}
  .directory-table{overflow:auto;max-height:440px;border-top:1px solid var(--wa-border-soft);}
  table{width:100%;border-collapse:collapse;text-align:left;font-size:12px;}
  th{position:sticky;top:0;z-index:1;padding:12px;color:var(--wa-text-muted);font-size:11px;font-weight:600;background:var(--wa-surface-inset);}
  td{padding:14px 12px;border-bottom:1px solid var(--wa-border-soft);vertical-align:top;overflow-wrap:anywhere;}
  td:first-child{width:42%;}td strong{font-weight:600;color:var(--wa-text-strong);}td span{display:block;margin-top:4px;}code{font-size:12px;}
  footer{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-top:14px;}footer div{display:flex;gap:8px;}
  .directory-empty{padding:24px 12px;color:var(--wa-text-muted);}
  @media(max-width:760px){.directory-filters{grid-template-columns:minmax(0,1fr);}input,select{min-height:44px;}table{min-width:620px;}footer{align-items:stretch;flex-direction:column;}footer div{justify-content:space-between;}}
</style>
