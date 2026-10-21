<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Button from '../shared/Button.svelte';
  type Item = { id: number; source_ref: string; kind: string; scope: string; scope_id: string; title: string; content: string; claim: string; source_updated_at: string; source_state: string; source_truncated: boolean; chunk_index: number; chunk_count: number; content_hash: string };
  type Result = { snapshot_id: number; checked_at: string; total: number; next_cursor?: number; items: Item[]; generation: {created_at: string; content_hash: string} };
  let result: Result | null = null;
  let items: Item[] = [];
  let query = '';
  let appliedFilters = { q: '', kind: '', scope_id: '' };
  let appliedKey = '';
  $: filterKey = JSON.stringify({q:query,kind,scope_id:scope.trim()});
  $: pendingFilters = result !== null && filterKey !== appliedKey;
  let kind = '';
  let scope = '';
  let loading = false;
  let error = '';
  let detail: Item | null = null;
  let detailError = '';
  let detailBusy = '';
  let version = 0;
  let detailVersion = 0;
  const kinds: Record<string,string> = {jira:'Jira',solution:'方案',context_document:'文档',context_fact:'事实',review:'评审',capability:'能力注册'};
  const claimLabels: Record<string,string> = {recorded_requirement_not_implementation:'需求记录 · 尚未验证实现',curated_claim_requires_validation:'人工事实 · 需要核验',documented_claim_requires_validation:'文档声明 · 需要核验',published_design_not_implementation:'已发布方案 · 不代表实现',review_observation_at_bound_sha:'指定版本的评审观察',registered_capability_not_runtime_acceptance:'已注册能力 · 不代表运行验收'};
  async function load(more = false) {
    const current = ++version; detailVersion++; detail = null; detailError = ''; detailBusy = ''; error = ''; loading = true;
    const filters = more ? appliedFilters : {q:query,kind,scope_id:scope.trim()};
    const params = new URLSearchParams(filters);
    if (more && result) { params.set('snapshot_id',String(result.snapshot_id)); params.set('after',String(result.next_cursor || 0)); }
    try {
      const res = await fetch(`/api/agent-runtime/knowledge/items?${params}`);
      if (!res.ok) throw new Error(res.status === 409 ? '此快照已过期，请重新查询。' : '知识内容加载失败，请确认知识读取权限后重试。');
      const value: Result = await res.json();
      if (current !== version) return;
      result = value; items = more ? [...items,...value.items] : value.items;
      appliedFilters = filters; appliedKey = JSON.stringify(filters);
    } catch(e) { if(current === version) error=(e as Error).message; }
    finally { if(current === version) loading = false; }
  }
  async function expand(item: Item) {
    if (detail?.source_ref === item.source_ref) { detail = null; detailVersion++; return; }
    const current = ++detailVersion; detailBusy = item.source_ref; detailError = '';
    try {
      const params=new URLSearchParams({snapshot_id:String(result?.snapshot_id || 0),source_ref:item.source_ref});
      const res=await fetch(`/api/agent-runtime/knowledge/items?${params}`);
      if(!res.ok) throw new Error('来源展开失败；请刷新快照后重试。');
      const value:Result=await res.json();if(current===detailVersion) {
        detail=value.items[0] || null;
        if(detail) { const fresh = detail; items=items.map(row=>row.source_ref===fresh.source_ref?{...fresh,content:Array.from(fresh.content).slice(0,400).join('')}:row); }
      }
    }catch(e){if(current===detailVersion) detailError=(e as Error).message;}
    finally{if(current===detailVersion)detailBusy='';}
  }
  function date(value:string){return value?new Date(value).toLocaleString('zh-CN'):'未知';}
  onMount(()=>{void load();});
  onDestroy(()=>{version++;detailVersion++;});
</script>

<section class="knowledge-browser" aria-label="自动知识库内容" aria-busy={loading}>
  <header><div><h3>知识快照内容</h3><p>浏览自动整理后的证据。来源在展示前重新核验；已变更或撤回的正文不会继续展示。</p></div><Button variant="secondary" loading={loading} on:click={()=>load()}>刷新内容</Button></header>
  <form on:submit|preventDefault={()=>load()}><label><span>关键词</span><input type="search" bind:value={query} maxlength="200" placeholder="搜索需求、系统能力或场景" /></label><label><span>来源</span><select bind:value={kind}><option value="">全部来源</option>{#each Object.entries(kinds) as [key,label]}<option value={key}>{label}</option>{/each}</select></label><label><span>项目或仓库标识</span><input bind:value={scope} maxlength="160" placeholder="例如 NS2 或仓库 ID" /></label><Button type="submit" disabled={loading}>查询知识</Button></form>
  {#if result}<div class="meta" role="status"><span>快照 #{result.snapshot_id || '—'} · {result.total} 个匹配证据段</span><span>最近检查 {date(result.checked_at)}</span></div>{/if}
  {#if error}<div class="error" role="alert">{error}<Button variant="secondary" on:click={()=>load()}>重新查询</Button></div>{/if}
  {#if loading && !items.length}<p role="status">正在加载知识内容…</p>{:else if !items.length && !error}<div class="empty"><h4>暂无匹配的知识内容</h4><p>调整筛选，或等待已同步资料完成下一次整理。</p></div>{/if}
  <div class="items">{#each items as item (item.source_ref)}<article><div class="item-heading"><div><span class="source">{kinds[item.kind] || item.kind} · {item.scope_id || '全局'}</span><h4>{item.title}</h4></div><Button size="small" variant="secondary" loading={detailBusy===item.source_ref} on:click={()=>expand(item)}>{detail?.source_ref===item.source_ref?'收起来源':'展开来源'}</Button></div>
      {#if item.source_state!=='current'}<p class="warning">来源已变更或撤回，等待下一次知识整理后重新核验。</p>{:else}<p class="excerpt">{item.content}</p>{/if}
      <div class="item-meta"><code>{item.source_ref}</code><span>{claimLabels[item.claim] || item.claim} · 来源更新 {date(item.source_updated_at)}</span><span>证据段 {item.chunk_index || 1}/{item.chunk_count || 1}</span></div>
      {#if detail?.source_ref===item.source_ref}<div class="source-detail"><strong>已核验来源正文</strong>{#if detail.source_state==='current'}<pre>{detail.content}</pre>{#if detail.source_truncated}<p class="warning">来源提取或分段存在截断；此段不代表完整原文。</p>{/if}{:else}<p class="warning">来源已失效，正文不再展示。</p>{/if}<small>内容指纹 {detail.content_hash}</small></div>{/if}
    </article>{/each}</div>
  {#if detailError}<p class="error" role="alert">{detailError}</p>{/if}
  {#if pendingFilters}<p role="status">筛选条件已修改，请点击“查询知识”应用后继续浏览。</p>{/if}
  {#if result?.next_cursor}<Button variant="secondary" disabled={pendingFilters} loading={loading} on:click={()=>load(true)}>加载更多证据</Button>{/if}
  <p class="boundary">资料与方案不能证明系统已实现或部署；同步时效与来源缺口仍需结合原始证据判断。</p>
</section>

<style>
  .knowledge-browser{display:grid;gap:16px;min-width:0}header,.item-heading,.meta{display:flex;align-items:center;justify-content:space-between;gap:16px}h3{font-size:16px;margin:0 0 6px}h4{font-size:14px;margin:4px 0 8px}p{font-size:13px;line-height:1.6;margin:0;color:var(--wa-text-muted)}form{display:grid;grid-template-columns:minmax(160px,2fr) minmax(120px,1fr) minmax(150px,1fr) auto;gap:12px;align-items:end}label{display:grid;gap:6px;font-size:12px;font-weight:650}input,select{width:100%;min-height:38px;border:1px solid var(--wa-border-panel);border-radius:var(--wa-radius-md);padding:8px 10px;background:var(--wa-surface-panel);color:var(--wa-text-main);font:inherit}input:focus-visible,select:focus-visible{outline:2px solid var(--wa-border-focus);outline-offset:2px}.meta,.source,.item-meta{font-size:12px;color:var(--wa-text-muted)}.items article{padding-block:18px;border-top:1px solid var(--wa-border-divider)}.item-heading>div{min-width:0}.item-heading h4{overflow-wrap:anywhere;color:var(--wa-text-strong)}.excerpt{white-space:pre-wrap;overflow-wrap:anywhere;max-width:85ch;color:var(--wa-text-main)}.item-meta{display:flex;gap:12px;flex-wrap:wrap;margin-top:10px}code{overflow-wrap:anywhere}.source-detail{margin-top:16px;padding-block:16px;border-top:1px solid var(--wa-border-divider);font-size:13px}.source-detail pre{white-space:pre-wrap;overflow-wrap:anywhere;font:inherit;line-height:1.7}.source-detail small{overflow-wrap:anywhere;color:var(--wa-text-muted)}.warning,.error{color:var(--wa-warning)}.error{display:flex;gap:12px;align-items:center}.empty{padding:24px 0}.boundary{border-top:1px solid var(--wa-border-divider);padding-top:14px;font-size:12px}
  @media(max-width:1024px){form{grid-template-columns:1fr 1fr}}@media(max-width:760px){form{grid-template-columns:1fr}header,.item-heading,.meta{align-items:stretch;flex-direction:column}input,select{min-height:44px}.item-meta{flex-direction:column;gap:6px}}
</style>
