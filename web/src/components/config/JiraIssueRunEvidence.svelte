<script lang="ts">
 import Button from '../shared/Button.svelte';
 import { safeLink, type SyncItem } from '../../lib/jiraIssueWorkspace';
 export let item:SyncItem;
 let expanded=false;
</script>
<article class="task-evidence">
 <div class="task-summary">
  <div><a href={safeLink(item.jira_url)} target="_blank" rel="noreferrer">{item.jira_key}</a><span>合并 {item.repositories.filter(r=>r.merged).length}/{item.repositories.length} · 验证 {item.repositories.filter(r=>r.verified).length}/{item.repositories.length}</span></div>
  <span class:complete={item.closed}>{item.closed?'Jira 已关闭':item.commented?'证据已评论':'等待证据'}</span>
  <Button variant="ghost" size="small" ariaLabel={`${expanded?'收起':'展开'} ${item.jira_key} 证据`} on:click={()=>expanded=!expanded}>{expanded?'收起':'证据'}</Button>
 </div>
 <p>{item.message}</p>
 {#if expanded}<div class="repository-evidence">
 {#each item.repositories as repo}
  <div class="repo"><strong>{repo.repository}</strong><span>{repo.message}</span>
  <div class="links">{#if safeLink(repo.issue_url)}<a href={safeLink(repo.issue_url)} target="_blank" rel="noreferrer">Issue</a>{/if}{#if safeLink(repo.mr_url)}<a href={safeLink(repo.mr_url)} target="_blank" rel="noreferrer">合并记录</a>{/if}{#if safeLink(repo.pipeline_url)}<a href={safeLink(repo.pipeline_url)} target="_blank" rel="noreferrer">验证记录</a>{/if}</div>
  {#if repo.sha}<code>{repo.sha}</code>{/if}<time datetime={repo.collected_at}>{new Date(repo.collected_at).toLocaleString('zh-CN')}</time></div>
 {/each}</div>{/if}
</article>
<style>
 .task-evidence{padding:16px 0;border-top:1px solid var(--wa-line,#e4e9e8);font-size:13px}.task-summary{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.task-summary>div{display:flex;gap:12px;flex:1;align-items:center;flex-wrap:wrap}.task-summary span,p,time{color:var(--wa-text-muted,#60716e)}.task-summary .complete{color:var(--wa-success,#147d59)}a{color:var(--wa-accent,#087e8b);font-weight:600;overflow-wrap:anywhere}p{margin:6px 0 0}.repository-evidence{padding-left:16px;border-left:2px solid var(--wa-line,#e4e9e8);margin-top:14px}.repo{display:grid;gap:6px;padding:8px 0;min-width:0}.repo>strong,.repo>span,code{overflow-wrap:anywhere}.links{display:flex;gap:16px;flex-wrap:wrap}code{font-size:12px}time{font-size:11px}
</style>
