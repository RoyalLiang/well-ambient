<script lang="ts">
 import { onMount, onDestroy } from 'svelte';
 import Button from '../shared/Button.svelte';
 import Switch from '../shared/Switch.svelte';
 import Select from '../shared/Select.svelte';
 import MultiSelect from '../shared/MultiSelect.svelte';
 import TextInput from '../shared/TextInput.svelte';
 import JiraIssueMappingRow from './JiraIssueMappingRow.svelte';
 import JiraIssueRunEvidence from './JiraIssueRunEvidence.svelte';
 import { initialSettings, kindLabel, statusLabel, type Workspace, type WorkspaceSettings, type Mapping, type Run } from '../../lib/jiraIssueWorkspace';
 export let canWrite=false;
 export let canAccess=false;
 let settings:WorkspaceSettings=initialSettings();
 let workspace:Workspace|null=null;
 let saved='';
 let gitlabToken='';
 let jiraToken='';
 let error='';
 let notice='';
 let busy='';
 let alive=true;
 let controller:AbortController;
 let timer:ReturnType<typeof setTimeout>;
 let analyzeRepos:string[]=[];
 let analyzeProjects:string[]=[];
 let selectedSuggestions:string[]=[];
 let openRun=0;
 const base='/api/jira-issue-sync/workspace';
 const hours=Array.from({length:24},(_,i)=>({value:String(i).padStart(2,'0'),label:String(i).padStart(2,'0')}));
 const minutes=Array.from({length:60},(_,i)=>({value:String(i).padStart(2,'0'),label:String(i).padStart(2,'0')}));
 $: dirty=Boolean(saved)&&(JSON.stringify(settings)!==saved||Boolean(gitlabToken||jiraToken));
 $: disabled=!canWrite||Boolean(busy)||Boolean(workspace?.busy)||!saved;
 $: repos=workspace?.catalog.repositories||[];
 $: projects=workspace?.catalog.projects||[];
  let discoveryAttemptedVersion = 0;
  $: catalogChecked = Boolean(workspace?.catalog.checked_at && !workspace.catalog.checked_at.startsWith('0001'));
  $: latestDiscovery = workspace?.runs.find(run => run.kind === 'discover' && run.config_version === settings.version);
  $: catalogLoading = busy === 'discover' || Boolean(workspace?.busy && latestDiscovery?.status === 'running');
  $: catalogFailure = latestDiscovery?.status === 'failed' ? latestDiscovery.message : '';
  $: catalogHint = catalogLoading ? '正在读取 Jira 项目和 GitLab 仓库…'
    : !workspace?.gitlab_configured || !workspace?.jira_configured ? '保存独立连接后读取项目与仓库。'
    : !catalogChecked ? (catalogFailure ? `目录读取失败：${catalogFailure}` : '目录尚未读取，请读取项目与仓库。')
    : !projects.length ? '未发现可访问的 Jira 项目，请检查 Jira 权限。'
    : !repos.length ? '群组中没有可同步仓库，请检查群组和仓库权限。' : '';
 $: projectOptions=projects.map(p=>({value:p.key,label:p.key,meta:p.name}));
 $: repoOptions=repos.map(r=>({value:String(r.id),label:r.path_with_namespace}));
 $: latestAnalysis=workspace?.runs.find(r=>r.kind==='analyze'&&r.status==='succeeded');
 $: running=workspace?.runs.find(r=>r.status==='running');
 $: hour=settings.time_of_day.split(':')[0]||'09';
 $: minute=settings.time_of_day.split(':')[1]||'00';
 async function request(url:string,options:RequestInit={}){const res=await fetch(url,{...options,signal:controller?.signal});const value=await res.json().catch(()=>null);if(!res.ok)throw Error(res.status===403?'需要全局超级管理员及对应配置权限。':value?.error||'请求失败，请稍后重试。');return value;}
 function poll(){clearTimeout(timer);if(alive&&workspace?.busy)timer=setTimeout(()=>load(true),1800)}
  async function load(background=false) {
    if (!canAccess) return;
    if (!background) busy='load';
    try {
      const value:Workspace=await request(base);
      if (!alive) return;
      workspace=value;
      if (!saved || (!background && !dirty)) {settings=value.settings;saved=JSON.stringify(settings)}
      if (!analyzeProjects.length) analyzeProjects=[...new Set(value.settings.mappings.map(m=>m.project))];
      if (!analyzeProjects.length && value.catalog.projects.length) analyzeProjects=[value.catalog.projects[0].key];
      if (!analyzeRepos.length) analyzeRepos=value.catalog.repositories.slice(0,5).map(r=>String(r.id));
      if (value.busy && value.runs[0] && !openRun) openRun=value.runs[0].id;
      poll();
    } catch(e) {
      if(alive) {error=(e as Error).message;if(background)timer=setTimeout(()=>load(true),5000)}
    } finally {if(alive && !background)busy=''}
    if (!background) await discoverMissingCatalog();
  }
  async function discoverMissingCatalog() {
    const version=workspace?.settings.version || 0;
    const checked=workspace?.catalog.checked_at;
    if (!alive || !canWrite || !workspace || busy || workspace.busy || dirty || !version
      || discoveryAttemptedVersion===version || !workspace.gitlab_configured || !workspace.jira_configured
      || (checked && !checked.startsWith('0001'))) return;
    discoveryAttemptedVersion=version;
    await start('discover');
  }
 async function showRun(run:Run,more=false){if(openRun===run.id&&!more){openRun=0;return}try{const detail:Run=await request(`${base}?run_id=${run.id}&offset=${more?run.next_offset:0}`);if(!alive||!workspace)return;workspace={...workspace,runs:workspace.runs.map(r=>r.id===run.id?{...detail,items:more?[...(r.items||[]),...(detail.items||[])]:detail.items}:r)};openRun=run.id}catch(e){if(alive)error=(e as Error).message}}
 function discard(){if(!workspace)return;settings=structuredClone(workspace.settings);saved=JSON.stringify(settings);gitlabToken='';jiraToken='';error='';notice='已放弃未保存修改。'}
 async function reload(){if(dirty){error='当前有未保存的修改，请先保存后刷新。';return}error='';await load()}
 async function save(){busy='save';error='';notice='';try{const value:Workspace=await request(base,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({...settings,gitlab_token:gitlabToken,jira_token:jiraToken})});if(!alive)return;workspace=value;settings=value.settings;saved=JSON.stringify(settings);gitlabToken='';jiraToken='';notice='配置已保存。';if(!value.catalog.checked_at || value.catalog.checked_at.startsWith('0001')){analyzeProjects=[];analyzeRepos=[]}}catch(e){if(alive)error=(e as Error).message}finally{if(alive)busy=''}await discoverMissingCatalog()}
 async function start(kind:string){busy=kind;error='';notice='';try{const run:Run=await request(`${base}/${kind}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({version:settings.version,repo_ids:kind==='analyze'?analyzeRepos:[],projects:analyzeProjects})});if(!alive)return;openRun=run.id;selectedSuggestions=[];await load(true)}catch(e){if(alive)error=(e as Error).message}finally{if(alive)busy=''}}
 async function stop(){if(!running)return;busy='cancel';try{await request(`${base}/cancel`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({run_id:running.id})});notice='已请求停止，已发出的请求会保留记录。';await load(true)}catch(e){error=(e as Error).message}finally{busy=''}}
 function changeMapping(index:number,mapping:Mapping){settings={...settings,mappings:settings.mappings.map((m,i)=>i===index?mapping:m)}}
 function removeMapping(index:number){settings={...settings,mappings:settings.mappings.filter((_,i)=>i!==index)}}
 function addMapping(){settings={...settings,mappings:[...settings.mappings,{project:projects[0]?.key||'',repo_ids:[],task_keys:[]}]}}
 function apply(){if(!latestAnalysis||latestAnalysis.config_version!==settings.version)return;const rows=latestAnalysis.proposals?.filter((_,i)=>selectedSuggestions.includes(String(i)))||[];const mappings=[...settings.mappings];for(const row of rows){const mapping={project:row.project,repo_ids:row.repo_ids,task_keys:row.task_keys};if(!mappings.some(m=>JSON.stringify(m)===JSON.stringify(mapping)))mappings.push(mapping)}settings={...settings,mappings};selectedSuggestions=[];notice='建议已加入映射，保存后参与同步。'}
 onMount(()=>{controller=new AbortController();load()});
 onDestroy(()=>{alive=false;clearTimeout(timer);controller?.abort()});
</script>

<section class="sync-config" aria-label="Jira Issue 同步配置" aria-busy={Boolean(busy)}>
 <header class="sync-heading">
  <div><h2>每日同步</h2><p>同步 Jira 任务，核对合并与验证证据。</p></div>
  <div class="heading-actions"><div class="activation"><Switch id="jira-sync-enabled" label="每日自动同步" checked={settings.enabled} expandedHitArea disabled={disabled} on:change={e=>settings={...settings,enabled:e.detail}} /><span>{settings.enabled?'已启用':'未启用'}</span></div><Button disabled={disabled||dirty||!settings.mappings.length} loading={busy==='sync'} on:click={()=>start('sync')}>启动同步</Button></div>
 </header>
 {#if !canAccess}<p class="empty">需要全局超级管理员查看独立连接与同步记录。</p>{:else}
 <div class="schedule-bar">
  <div class="time-control"><span>自动同步</span><div class="time-part"><Select id="sync-hour" ariaLabel="自动同步小时" value={hour} options={hours} compact searchable={false} shadowless disabled={disabled} on:change={e=>settings={...settings,time_of_day:`${e.detail}:${minute}`}} /></div><span>:</span><div class="time-part"><Select id="sync-minute" ariaLabel="自动同步分钟" value={minute} options={minutes} compact searchable={false} shadowless disabled={disabled} on:change={e=>settings={...settings,time_of_day:`${hour}:${e.detail}`}} /></div><span class="muted">北京时间</span></div>
  <div class="save-actions"><span class="config-state" role="status">{dirty?'待保存':saved?`配置 v${settings.version}`:'正在加载'}</span>{#if dirty}<Button variant="ghost" disabled={Boolean(busy)||Boolean(workspace?.busy)} on:click={discard}>放弃修改</Button>{/if}<Button variant="ghost" disabled={Boolean(busy)||Boolean(workspace?.busy)||dirty} on:click={reload}>刷新</Button><Button variant="secondary" disabled={disabled||!dirty} loading={busy==='save'} on:click={save}>保存配置</Button></div>
 </div>
 {#if error}<div class="feedback error" role="alert">{error}</div>{/if}{#if notice}<p class="feedback" role="status">{notice}</p>{/if}
 {#if workspace&&!workspace.key_ready}<p class="feedback error">独立凭据加密密钥未配置，请先配置服务端 WELL_JIRA_ISSUE_SYNC_KEY。</p>{/if}
 <section class="module" aria-labelledby="sync-connection-title">
  <div class="section-head"><div><h3 id="sync-connection-title">独立连接</h3><p>连接此同步使用的 GitLab 群组与 Jira。</p></div><Button variant="secondary" disabled={disabled||dirty||!workspace?.gitlab_configured||!workspace?.jira_configured} loading={catalogLoading} on:click={()=>start('discover')}>{catalogFailure?'重试读取目录':'读取项目与仓库'}</Button></div>
  <div class="connection-grid">
   <TextInput id="sync-group-url" label="GitLab 群组地址" bind:value={settings.group_url} placeholder="https://gitlab.example.com/group" disabled={disabled} />
   <TextInput id="sync-gitlab-token" label="GitLab Token" type="password" bind:value={gitlabToken} placeholder={workspace?.gitlab_configured?'已配置 · 留空保留':'填写独立 API Token'} disabled={disabled||!workspace?.key_ready} />
   <TextInput id="sync-jira-url" label="Jira 地址" bind:value={settings.jira_url} placeholder="https://jira.example.com" disabled={disabled} />
   <TextInput id="sync-jira-token" label="Jira PAT" type="password" bind:value={jiraToken} placeholder={workspace?.jira_configured?'已配置 · 留空保留':'填写独立个人访问令牌'} disabled={disabled||!workspace?.key_ready} />
  </div>
  <div class="connection-status"><span>GitLab {workspace?.gitlab_configured?'已配置':'未配置'}</span><span>Jira {workspace?.jira_configured?'已配置':'未配置'}</span>{#if workspace?.catalog.checked_at&&!workspace.catalog.checked_at.startsWith('0001')}<span>{repos.length} 个仓库 · {projects.length} 个项目</span><time datetime={workspace.catalog.checked_at}>读取于 {new Date(workspace.catalog.checked_at).toLocaleString('zh-CN')}</time>{/if}</div>
    {#if catalogHint}<p class="field-note" role="status">{catalogHint}</p>{/if}
 </section>
 <section class="module" aria-labelledby="sync-mapping-title">
  <div class="section-head"><div><h3 id="sync-mapping-title">项目映射</h3><p>关联 Jira 项目、任务与多个代码仓库。</p></div><Button variant="secondary" disabled={disabled||!repos.length||!projects.length} on:click={addMapping}>添加映射</Button></div>
  <div class="mappings">{#each settings.mappings as mapping,index}<JiraIssueMappingRow {mapping} {index} {projects} {repos} disabled={disabled} onChange={m=>changeMapping(index,m)} onRemove={()=>removeMapping(index)} />{:else}<p class="empty">读取独立连接后添加映射，或使用 AI 识别。</p>{/each}</div>
  <div class="analysis-control"><div class="analysis-select"><MultiSelect id="sync-analysis-projects" label="AI 分析 Jira 项目" values={analyzeProjects} options={projectOptions} appearance="settings" shadowless overlay summaryMode maxVisibleOptions={100} disabled={disabled||!projects.length} emptyText="未发现可访问的 Jira 项目" on:change={e=>analyzeProjects=e.detail} /></div><div class="analysis-select"><MultiSelect id="sync-analysis-repos" label="AI 分析仓库" values={analyzeRepos} options={repoOptions} appearance="settings" shadowless overlay summaryMode maxVisibleOptions={100} disabled={disabled||!repos.length} emptyText="未发现可同步仓库" on:change={e=>analyzeRepos=e.detail} /></div><Button variant="secondary" disabled={disabled||dirty||!workspace?.ai_ready||!analyzeRepos.length||analyzeRepos.length>20||!analyzeProjects.length||analyzeProjects.length>20} loading={busy==='analyze'} on:click={()=>start('analyze')}>分析群组</Button></div>
  <p class="field-note">每次最多 20 个仓库和最近更新的 100 个未完成任务；读取固定提交源码，生成关联建议。{#if workspace&&!workspace.ai_ready}请先启用 AI 模型连接。{/if}</p>
  {#if latestAnalysis}<div class="suggestions"><div class="section-head"><h4>AI 建议 <span>{latestAnalysis.proposals?.length||0} 条</span></h4><Button variant="secondary" disabled={disabled||!selectedSuggestions.length||latestAnalysis.config_version!==settings.version} on:click={apply}>采纳选中建议</Button></div>
   {#if latestAnalysis.config_version!==settings.version}<p class="field-note">建议来自旧配置，请重新分析。</p>{/if}
   {#each latestAnalysis.proposals||[] as proposal,index}<article class="proposal"><div class="proposal-title"><Switch id={`sync-proposal-${index}`} label={`${proposal.project} · ${proposal.task_keys.join('、')}`} checked={selectedSuggestions.includes(String(index))} disabled={disabled||latestAnalysis.config_version!==settings.version} on:change={e=>selectedSuggestions=e.detail?[...selectedSuggestions,String(index)]:selectedSuggestions.filter(v=>v!==String(index))} /><span>AI 建议</span></div><p>{proposal.reason}</p><div class="proposal-sources">{#each proposal.evidence as source}<code>{repos.find(r=>String(r.id)===source.repo_id)?.path_with_namespace||source.repo_id}/{source.path} @{source.sha.slice(0,8)}</code>{/each}</div></article>{:else}<p class="empty">本次未发现有源码依据的关联。</p>{/each}
  </div>{/if}
 </section>
 <section class="module" aria-labelledby="sync-runs-title"><div class="section-head"><div><h3 id="sync-runs-title">运行与证据</h3><p>查看同步结果和 Jira 关闭依据。</p></div>{#if running&&workspace?.busy}<Button variant="secondary" disabled={!canWrite||Boolean(busy)} loading={busy==='cancel'} on:click={stop}>停止运行</Button>{/if}</div>
  {#each workspace?.runs||[] as run (run.id)}<article class="run"><div class="run-head"><div><strong>{kindLabel(run.kind)}</strong><span class:success={run.status==='succeeded'} class:failure={run.status==='failed'}>{statusLabel(run.status)}</span><time datetime={run.created_at}>{new Date(run.created_at).toLocaleString('zh-CN')}</time></div>{#if run.items_total>0}<Button variant="ghost" size="small" ariaLabel={`${openRun===run.id?'收起':'展开'}运行 ${run.id}`} on:click={()=>showRun(run)}>{openRun===run.id?'收起':'详情'}</Button>{/if}</div><p class="run-message" role={run.status==='running'?'status':undefined}>{run.message}</p>{#if openRun===run.id}{#each run.items||[] as item (item.jira_key)}<JiraIssueRunEvidence {item} />{/each}{#if run.next_offset>0}<Button variant="ghost" on:click={()=>showRun(run,true)}>更多记录（已显示 {run.items?.length||0}/{run.items_total}）</Button>{/if}{/if}</article>{:else}<p class="empty">尚无运行记录。</p>{/each}
 </section>
 {/if}
</section>
<style>
 .sync-config{container-type:inline-size;--wa-control-h:44px;color:var(--wa-text-main,#293e3a)}h2,h3,h4,p{margin:0}h2{font-size:19px;font-weight:700;letter-spacing:-.03em}h3{font-size:15px;font-weight:700}h4{font-size:14px}.sync-heading{display:flex;justify-content:space-between;align-items:center;gap:20px}.sync-heading p,.section-head p{font-size:13px;color:var(--wa-text-muted,#62736f);line-height:1.6;margin-top:5px}.heading-actions,.activation,.save-actions,.time-control{display:flex;align-items:center;gap:12px}.activation{gap:6px;font-size:12px;color:var(--wa-text-muted,#62736f);white-space:nowrap}.schedule-bar{display:flex;justify-content:space-between;gap:16px;align-items:center;padding:20px 0 24px;flex-wrap:wrap}.time-control{font-size:13px;gap:8px}.time-part{width:86px;flex-shrink:0}.muted,.config-state{font-size:12px;color:var(--wa-text-muted,#62736f)}.save-actions{margin-left:auto}.module{border-top:1px solid var(--wa-line,#e2e9e6);padding:26px 0}.section-head{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:20px}.section-head>div{min-width:0}.connection-grid{display:grid;grid-template-columns:minmax(0,1.2fr) minmax(0,1fr);gap:18px 24px}.connection-status{display:flex;gap:8px 20px;font-size:12px;color:var(--wa-text-muted,#62736f);margin-top:18px;flex-wrap:wrap}.connection-grid :global(.input-group){margin-bottom:0}.feedback{font-size:13px;line-height:1.6;padding:10px 14px;background:var(--wa-surface-subtle,#f0f6f3);border-radius:6px;margin:0 0 16px;overflow-wrap:anywhere}.feedback.error{background:#fff5f3;color:var(--wa-danger,#a53939)}.empty{font-size:13px;color:var(--wa-text-muted,#62736f);padding:16px 0;line-height:1.7}.analysis-control{display:flex;align-items:end;gap:16px;margin-top:24px}.analysis-select{min-width:0;flex:1}.field-note{font-size:12px;color:var(--wa-text-muted,#62736f);line-height:1.7;margin-top:8px}.suggestions{margin-top:28px}.suggestions h4 span{font-size:12px;color:var(--wa-text-muted,#62736f);font-weight:400;margin-left:8px}.proposal{padding:16px 0;border-top:1px solid var(--wa-line,#e2e9e6)}.proposal-title{display:flex;gap:12px;align-items:center;justify-content:space-between}.proposal-title>span{font-size:11px;color:var(--wa-text-muted,#62736f);white-space:nowrap}.proposal>p{font-size:13px;line-height:1.7;margin:8px 0;overflow-wrap:anywhere}.proposal-sources{display:flex;flex-wrap:wrap;gap:6px 16px}.proposal-sources code{font-size:11px;color:var(--wa-text-muted,#62736f);overflow-wrap:anywhere}.run{padding:18px 0;border-top:1px solid var(--wa-line,#e2e9e6)}.run-head{display:flex;align-items:center;justify-content:space-between;gap:16px}.run-head>div{display:flex;align-items:center;flex-wrap:wrap;gap:10px 16px;font-size:13px}.run-head span,time{font-size:12px;color:var(--wa-text-muted,#62736f)}.run-head .success{color:var(--wa-success,#147d59)}.run-head .failure{color:var(--wa-danger,#b73f3f)}.run-message{font-size:13px;color:var(--wa-text-muted,#62736f);line-height:1.7;margin-top:8px;overflow-wrap:anywhere}.sync-config :global(.select-inline-input){border:0!important;background:transparent!important;box-shadow:none!important}.sync-config .activation :global(button.switch-control),.sync-config .activation :global(button.switch-control.checked){width:44px!important;height:44px!important;background:transparent!important;border:0!important;box-shadow:none!important}.sync-config .activation :global(.switch-thumb){top:13px!important}
 @container(max-width:900px){.sync-heading{align-items:flex-start}.heading-actions{flex-wrap:wrap;justify-content:flex-end}.connection-grid{gap:16px}.save-actions{gap:6px}}
 @container(max-width:600px){.sync-heading{gap:12px;flex-wrap:wrap}.heading-actions{width:100%;justify-content:space-between}.connection-grid{grid-template-columns:minmax(0,1fr)}.schedule-bar{gap:12px}.time-control{width:100%}.save-actions{width:100%;justify-content:flex-end}.section-head{align-items:flex-start;flex-wrap:wrap}.section-head>:global(button){margin-left:auto}.analysis-control{align-items:stretch;flex-direction:column}.analysis-control>:global(button){align-self:flex-end}.run-head time{flex-basis:100%}.time-control>span:last-child{margin-left:auto}}
</style>
