<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';

  export let currentUserPermissions: string[] = [];

  type Candidate = {
    id: number;
    candidate_type: string;
    scope: string;
    scope_id: string;
    title: string;
    summary: string;
    content: string;
    confidence: number;
    sensitivity: string;
    status: string;
    created_at: string;
  };

  const dispatch = createEventDispatcher();
  let candidates: Candidate[] = [];
  let loading = false;
  let error = '';
  let activeID = 0;

  $: canRead = currentUserPermissions.includes('corpus_candidate:read');
  $: canReview = currentUserPermissions.includes('corpus_candidate:review');

  onMount(() => {
    if (canRead) void loadCandidates();
  });

  async function loadCandidates() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/corpus-candidates?status=pending');
      if (!response.ok) throw new Error((await response.text()) || `HTTP ${response.status}`);
      const data = await response.json();
      candidates = data.items || [];
    } catch (err: any) {
      error = err.message || '加载语料候选失败';
    } finally {
      loading = false;
    }
  }

  async function review(candidate: Candidate, decision: 'accepted' | 'rejected') {
    activeID = candidate.id;
    error = '';
    try {
      const response = await fetch(`/api/corpus-candidates/${candidate.id}/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decision })
      });
      if (!response.ok) throw new Error((await response.text()) || `HTTP ${response.status}`);
      candidates = candidates.filter(item => item.id !== candidate.id);
      if (decision === 'accepted') dispatch('promoted');
    } catch (err: any) {
      error = err.message || '审核语料候选失败';
    } finally {
      activeID = 0;
    }
  }

  function typeLabel(value: string) {
    return ({ delivery_case: '交付案例', requirement_pattern: '验收模式', risk_rule: '风险规则' } as Record<string, string>)[value] || value;
  }

  function formatDate(value: string) {
    if (!value) return '-';
    return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
  }
</script>

{#if canRead}
  <section class="candidate-review" aria-label="交付语料候选审核">
    <header>
      <div>
        <span>DELIVERY FEEDBACK</span>
        <h4>交付语料候选</h4>
        <p>交付结果只会先进入候选队列；人工接受后才创建生效的 Context Fact。</p>
      </div>
      <button type="button" on:click={loadCandidates} disabled={loading}>{loading ? '加载中' : '刷新候选'}</button>
    </header>

    {#if error}<div class="candidate-error">{error}</div>{/if}

    {#if loading}
      <div class="candidate-empty">正在读取交付反馈…</div>
    {:else if candidates.length === 0}
      <div class="candidate-empty"><strong>没有待审核候选</strong><p>完成的自治交付会在这里生成案例、验收模式和风险复盘候选。</p></div>
    {:else}
      <div class="candidate-list">
        {#each candidates as candidate (candidate.id)}
          <article>
            <div class="candidate-meta">
              <span>{typeLabel(candidate.candidate_type)}</span>
              <span>{candidate.scope}:{candidate.scope_id || 'global'}</span>
              <span>{Math.round((candidate.confidence || 0) * 100)}%</span>
              <time>{formatDate(candidate.created_at)}</time>
            </div>
            <strong>{candidate.title}</strong>
            <p>{candidate.summary}</p>
            {#if canReview}
              <div class="candidate-actions">
                <button type="button" disabled={activeID === candidate.id} on:click={() => review(candidate, 'rejected')}>拒绝</button>
                <button class="accept" type="button" disabled={activeID === candidate.id} on:click={() => review(candidate, 'accepted')}>
                  {activeID === candidate.id ? '处理中…' : '接受并纳入语料'}
                </button>
              </div>
            {/if}
          </article>
        {/each}
      </div>
    {/if}
  </section>
{/if}

<style>
  .candidate-review { margin-top: 22px; border-top: 1px solid rgba(121,139,159,.18); padding-top: 20px; display: grid; gap: 12px; }
  header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
  header span { display: block; margin-bottom: 5px; color: #167a72; font: 700 10px/1.2 ui-monospace, monospace; letter-spacing: .1em; }
  h4 { margin: 0; color: #15212c; font-size: 16px; }
  header p { margin: 5px 0 0; color: #6d7b86; font-size: 12px; }
  button { min-height: 34px; border: 1px solid #ccd7dd; border-radius: 6px; background: #fff; color: #40505b; padding: 0 11px; font: 650 11px/32px inherit; cursor: pointer; }
  button:hover { border-color: #8ca8b2; background: #f7fafb; }
  button:focus-visible { outline: 3px solid rgba(0,143,150,.17); outline-offset: 1px; }
  button:disabled { opacity: .58; cursor: wait; }
  .candidate-list { display: grid; gap: 8px; }
  article { border: 1px solid #dce4e8; background: rgba(255,255,255,.8); padding: 12px; display: grid; gap: 7px; }
  article > strong { color: #23313b; font-size: 13px; }
  article > p { margin: 0; color: #697782; font-size: 12px; line-height: 1.45; }
  .candidate-meta { display: flex; flex-wrap: wrap; gap: 6px 10px; color: #7a8790; font: 10px/1.3 ui-monospace, monospace; }
  .candidate-meta span:first-child { color: #19786f; font-weight: 700; }
  .candidate-actions { display: flex; justify-content: flex-end; gap: 7px; }
  .candidate-actions .accept { border-color: #187c71; background: #187c71; color: #fff; }
  .candidate-actions .accept:hover { background: #116b62; }
  .candidate-empty { border: 1px dashed #cbd7dd; background: #f8fafb; padding: 18px; text-align: center; color: #73818b; font-size: 12px; }
  .candidate-empty strong { display: block; color: #394852; margin-bottom: 4px; }
  .candidate-empty p { margin: 0; }
  .candidate-error { border-left: 3px solid #c65a5a; background: #fff4f3; color: #924242; padding: 9px 10px; font-size: 12px; }
  @media (max-width: 720px) { header { display: grid; } .candidate-actions { justify-content: flex-start; } }
</style>
