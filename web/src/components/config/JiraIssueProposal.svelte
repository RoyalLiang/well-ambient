<script lang="ts">
 import Button from '../shared/Button.svelte';
 import Switch from '../shared/Switch.svelte';
 import {safeLink,type Proposal,type Repo,type TaskDetail} from '../../lib/jiraIssueWorkspace';
 export let proposal:Proposal; export let index=0; export let repos:Repo[]=[]; export let projects:{key:string;name:string}[]=[];
 export let taskDetails:TaskDetail[]=[]; export let groupURL=''; export let jiraURL=''; export let selected=false; export let disabled=false; export let onSelect:(value:boolean)=>void;
 let expanded=false;
 $: project=projects.find(p=>p.key===proposal.project);
 function taskURL(key:string){return safeLink(jiraURL.replace(/\/$/,'')+'/browse/'+encodeURIComponent(key))}
 function repoURL(id:string){if(!groupURL)return '';const r=repos.find(r=>String(r.id)===id);if(r?.web_url)return safeLink(r.web_url);try{return safeLink(new URL(groupURL).origin+'/'+(r?.path_with_namespace||'').split('/').map(encodeURIComponent).join('/'))}catch{return ''}}
 function sourceURL(id:string,path:string,sha:string){const base=repoURL(id);return base?base+'/-/blob/'+encodeURIComponent(sha)+'/'+path.split('/').map(encodeURIComponent).join('/'):''}
</script>
<article class="proposal" aria-label={`关联建议 ${index+1}`}>
 <header><Switch id={`sync-proposal-${index}`} label={`${proposal.project} · ${project?.name||'Jira 项目'}`} checked={selected} {disabled} on:change={e=>onSelect(e.detail)} /><span>待确认建议</span></header>
 <div class="proposal-grid">
  <div><h5>Jira 任务 <span>{proposal.task_keys.length}</span></h5>{#each proposal.task_keys as key}<a href={taskURL(key)||undefined} target="_blank" rel="noopener noreferrer">{key}</a><p class="task-title">{taskDetails.find(t=>t.key===key)?.title||'任务标题未随本次分析保存'}</p>{/each}</div>
  <div><h5>对应仓库 <span>{proposal.repo_ids.length}</span></h5>{#each proposal.repo_ids as id}<a href={repoURL(id)||undefined} target="_blank" rel="noopener noreferrer">{repos.find(r=>String(r.id)===id)?.path_with_namespace||`仓库 ${id}`}</a>{/each}</div>
 </div>
 <div class="reason"><h5>关联依据</h5><p>{proposal.reason}</p></div>
 <div class="detail-toggle"><span>{proposal.evidence.length} 条源码证据</span><Button variant="ghost" size="small" ariaExpanded={expanded} ariaControls={`sync-proposal-details-${index}`} on:click={()=>expanded=!expanded}>{expanded?'收起详情':'查看证据与任务详情'}</Button></div>
 {#if expanded}<div class="details" id={`sync-proposal-details-${index}`}><h5>固定版本源码</h5>{#each proposal.evidence as source}<a href={sourceURL(source.repo_id,source.path,source.sha)||undefined} target="_blank" rel="noopener noreferrer">{repos.find(r=>String(r.id)===source.repo_id)?.path_with_namespace||source.repo_id} / {source.path}<code>{source.sha}</code></a>{/each}<h5>Jira 详情</h5>{#each proposal.task_keys as key}<div class="task-detail"><strong>{key}</strong><p>{taskDetails.find(t=>t.key===key)?.description||'本次分析未保存任务说明。'}</p></div>{/each}<p class="limit">仅依据本次所选仓库的固定提交和最近更新的未完成任务；采纳后仍需保存映射。</p></div>{/if}
</article>
<style>
 .proposal{border-top:1px solid var(--wa-line,#e2e9e6);padding:20px 0;color:var(--wa-text-main)}header{display:flex;justify-content:space-between;align-items:center;gap:16px}header>span,.detail-toggle>span,.limit{font-size:12px;color:var(--wa-text-muted)}.proposal-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1.4fr);gap:24px;margin:18px 0}h5{font-size:12px;margin:0 0 8px;color:var(--wa-text-muted);font-weight:650}h5 span{margin-left:6px}a{display:block;color:var(--wa-accent-strong,#14786f);font-size:13px;line-height:1.6;overflow-wrap:anywhere;margin-bottom:5px}p{font-size:13px;line-height:1.7;margin:0;white-space:pre-wrap;overflow-wrap:anywhere}.task-title{margin-bottom:10px}.reason{max-width:80ch}.detail-toggle{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:12px}.details{background:var(--wa-surface-subtle,#f2f6f4);padding:16px;margin-top:12px;border-radius:var(--wa-radius-sm,8px)}.details h5:not(:first-child){margin-top:20px}code{display:block;font-size:11px;color:var(--wa-text-muted);overflow-wrap:anywhere}.task-detail{margin:10px 0}.task-detail strong{font-size:12px}.limit{margin-top:16px}@media(max-width:600px){.proposal-grid{grid-template-columns:minmax(0,1fr);gap:14px}.detail-toggle{flex-wrap:wrap}header{align-items:flex-start}}
</style>
