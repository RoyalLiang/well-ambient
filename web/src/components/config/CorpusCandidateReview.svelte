<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import Button from '../shared/Button.svelte';
  import MarkdownWorkbench, { type MarkdownMode } from '../shared/MarkdownWorkbench.svelte';
  import Select from '../shared/Select.svelte';
  import { showToast } from '../../lib/toast';

  export let currentUserPermissions: string[] = [];

  type SourceDocumentRef = {
    id: number;
    title: string;
    original_name: string;
    version: number;
    content_hash: string;
    ingestion_status: string;
  };

  type Candidate = {
    id: number;
    context_document_id: number;
    candidate_type: string;
    scope: string;
    scope_id: string;
    title: string;
    summary: string;
    content: string;
    source_anchor: string;
    evidence_kind: string;
    ai_model: string;
    confidence: number;
    sensitivity: string;
    review_mode: string;
    status: string;
    review_note?: string;
    source_document?: SourceDocumentRef;
    created_at: string;
  };

  type CandidateDraft = Pick<Candidate,
    'candidate_type' | 'scope' | 'scope_id' | 'title' | 'summary' | 'content' |
    'source_anchor' | 'evidence_kind' | 'confidence' | 'sensitivity'>;

  type CandidateGroup = {
    key: string;
    title: string;
    originalName: string;
    version: number;
    candidates: Candidate[];
  };

  type ContextFact = {
    id: number;
    summary: string;
    content: string;
    version: number;
  };

  type ImpactPreview = {
    candidate: Candidate;
    source_document?: { id: number; title: string; content: string; original_name: string; version: number };
    current_facts: ContextFact[];
    before_markdown: string;
    after_markdown: string;
    source_markdown: string;
    requires_impact_review: boolean;
    reason: string;
  };

  const dispatch = createEventDispatcher();
  const candidateTypes = [
    { value: 'architecture', label: '架构设计' },
    { value: 'workflow', label: '流程设计' },
    { value: 'feature_boundary', label: '功能边界' },
    { value: 'estimation_rule', label: '估算口径' },
    { value: 'glossary', label: '术语口径' },
    { value: 'risk_rule', label: '风险规则' },
    { value: 'delivery_history', label: '交付样本' }
  ];
  const scopes = [
    { value: 'global', label: '全局' },
    { value: 'repo', label: '仓库' },
    { value: 'module', label: '模块' },
    { value: 'demand_type', label: '需求类型' }
  ];
  const sensitivities = [
    { value: 'normal', label: '普通' },
    { value: 'internal', label: '内部' },
    { value: 'high', label: '高敏感' },
    { value: 'restricted', label: '受限' }
  ];

  let candidates: Candidate[] = [];
  let loading = false;
  let error = '';
  let selectedID = 0;
  let selectedCandidate: Candidate | null = null;
  let draft: CandidateDraft | null = null;
  let impact: ImpactPreview | null = null;
  let impactLoading = false;
  let activeID = 0;
  let reviewNote = '';
  let proposalMode: MarkdownMode = 'live';
  let metadataOpen = false;
  let sourceEvidenceOpen = false;

  $: canRead = currentUserPermissions.includes('corpus_candidate:read');
  $: canReview = currentUserPermissions.includes('corpus_candidate:review');
  $: canPreviewImpact = currentUserPermissions.includes('ai_context:preview');
  $: pendingCount = candidates.filter(candidate => candidate.status === 'pending').length;
  $: impactCount = candidates.filter(candidate => candidate.status === 'impact_review').length;
  $: candidateGroups = groupCandidates(candidates);
  $: selectedGroup = candidateGroups.find(group => group.candidates.some(candidate => candidate.id === selectedID)) || null;
  $: selectedGroupPosition = selectedGroup
    ? selectedGroup.candidates.findIndex(candidate => candidate.id === selectedID) + 1
    : 0;
  $: leftComparisonMarkdown = selectedCandidate?.status === 'impact_review'
    ? (impact?.before_markdown || '# 当前生效上下文\n\n正在读取影响范围…')
    : (impact?.source_markdown || '# 来源原文\n\n暂无可显示的来源正文。');

  onMount(() => {
    if (canRead) void loadCandidates();
  });

  async function parseResponse(response: Response) {
    const text = await response.text();
    if (!text) return {};
    try {
      return JSON.parse(text);
    } catch {
      return { error: text };
    }
  }

  async function loadCandidates(preferredID = selectedID) {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/corpus-candidates?status=review_queue');
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      candidates = Array.isArray(data.items) ? data.items : [];
      const next = candidates.find(item => item.id === preferredID) || candidates[0] || null;
      if (next) await selectCandidate(next);
      else clearSelection();
    } catch (reason: any) {
      error = reason.message || '加载统一审核队列失败';
    } finally {
      loading = false;
    }
  }

  async function selectCandidate(candidate: Candidate) {
    selectedID = candidate.id;
    selectedCandidate = candidate;
    draft = {
      candidate_type: normalizeCandidateType(candidate.candidate_type),
      scope: candidate.scope || 'global',
      scope_id: candidate.scope_id || '',
      title: candidate.title || '',
      summary: candidate.summary || '',
      content: candidate.content || '',
      source_anchor: candidate.source_anchor || '',
      evidence_kind: candidate.evidence_kind || 'source_fact',
      confidence: Number(candidate.confidence) || 0.8,
      sensitivity: candidate.sensitivity || 'normal'
    };
    reviewNote = candidate.review_note || '';
    proposalMode = candidate.status === 'impact_review' ? 'preview' : 'live';
    metadataOpen = false;
    sourceEvidenceOpen = false;
    impact = null;
    if (candidate.status === 'impact_review' && canPreviewImpact) await loadImpact(candidate.id);
  }

  function clearSelection() {
    selectedID = 0;
    selectedCandidate = null;
    draft = null;
    impact = null;
    reviewNote = '';
    metadataOpen = false;
    sourceEvidenceOpen = false;
  }

  async function loadImpact(candidateID = selectedID) {
    if (!candidateID || !canPreviewImpact) return;
    impactLoading = true;
    error = '';
    try {
      const response = await fetch(`/api/corpus-candidates/${candidateID}/impact`);
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      impact = data.impact || null;
    } catch (reason: any) {
      error = reason.message || '候选影响预览加载失败';
    } finally {
      impactLoading = false;
    }
  }

  function candidatePayload() {
    if (!draft) return null;
    return {
      ...draft,
      scope_id: draft.scope === 'global' ? '' : draft.scope_id.trim(),
      title: draft.title.trim(),
      summary: draft.summary.trim(),
      content: draft.content.trim(),
      source_anchor: draft.source_anchor.trim(),
      confidence: Number(draft.confidence) || 0.8
    };
  }

  function validateDraft() {
    if (!draft?.title.trim() || !draft.summary.trim() || !draft.content.trim()) {
      metadataOpen = true;
      return '请完整填写候选标题、摘要和 Markdown 内容';
    }
    if (draft.scope !== 'global' && !draft.scope_id.trim()) {
      metadataOpen = true;
      return '非全局候选需要填写范围标识';
    }
    return '';
  }

  function groupCandidates(items: Candidate[]): CandidateGroup[] {
    const groups = new Map<string, CandidateGroup>();
    for (const candidate of items) {
      const document = candidate.source_document;
      const key = document?.id ? `document:${document.id}` : `candidate:${candidate.id}`;
      const existing = groups.get(key);
      if (existing) {
        existing.candidates.push(candidate);
        continue;
      }
      groups.set(key, {
        key,
        title: document?.title || '交付归档候选',
        originalName: document?.original_name || '系统事件与交付归档',
        version: document?.version || 0,
        candidates: [candidate]
      });
    }
    return Array.from(groups.values());
  }

  async function handleSourceEvidenceToggle(event: Event) {
    const details = event.currentTarget as HTMLDetailsElement;
    sourceEvidenceOpen = details.open;
    if (details.open && !impact && selectedCandidate && canPreviewImpact) {
      await loadImpact(selectedCandidate.id);
    }
  }

  function handleScopeChange(value: string) {
    if (!draft) return;
    draft.scope = value;
    if (value === 'global') draft.scope_id = '';
  }

  async function review(decision: 'accepted' | 'rejected') {
    if (!selectedCandidate || !draft) return;
    const validationError = decision === 'accepted' ? validateDraft() : '';
    if (validationError) {
      error = validationError;
      return;
    }
    activeID = selectedCandidate.id;
    error = '';
    try {
      const response = await fetch(`/api/corpus-candidates/${selectedCandidate.id}/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decision, note: reviewNote.trim(), candidate: candidatePayload() })
      });
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      if (decision === 'rejected') {
        candidates = candidates.filter(item => item.id !== selectedCandidate?.id);
        const next = candidates[0] || null;
        if (next) await selectCandidate(next); else clearSelection();
        showToast('候选已拒绝，并保留审核记录。');
      } else if (data.requires_impact_review) {
        const updated = { ...selectedCandidate, ...data.candidate } as Candidate;
        candidates = candidates.map(item => item.id === updated.id ? updated : item);
        await selectCandidate(updated);
        showToast('初审已通过。该候选需要完成上下文影响对照后再正式发布。');
      } else {
        candidates = candidates.filter(item => item.id !== selectedCandidate?.id);
        dispatch('promoted');
        const next = candidates[0] || null;
        if (next) await selectCandidate(next); else clearSelection();
        showToast('普通语料已审核并发布为 active Context Fact。');
      }
    } catch (reason: any) {
      error = reason.message || '审核语料候选失败';
    } finally {
      activeID = 0;
    }
  }

  async function publish() {
    if (!selectedCandidate || !draft || selectedCandidate.status !== 'impact_review') return;
    const validationError = validateDraft();
    if (validationError) {
      error = validationError;
      return;
    }
    if (!canPreviewImpact || !impact) {
      error = '完成上下文影响预览后才能正式发布';
      return;
    }
    activeID = selectedCandidate.id;
    error = '';
    try {
      const response = await fetch(`/api/corpus-candidates/${selectedCandidate.id}/publish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ note: reviewNote.trim(), candidate: candidatePayload() })
      });
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      candidates = candidates.filter(item => item.id !== selectedCandidate?.id);
      dispatch('promoted');
      const next = candidates[0] || null;
      if (next) await selectCandidate(next); else clearSelection();
      showToast('影响审核已确认，候选已正式发布。');
    } catch (reason: any) {
      error = reason.message || '正式发布语料失败';
    } finally {
      activeID = 0;
    }
  }

  function normalizeCandidateType(value: string) {
    if (value === 'delivery_case') return 'delivery_history';
    if (value === 'requirement_pattern') return 'workflow';
    return candidateTypes.some(option => option.value === value) ? value : 'architecture';
  }

  function typeLabel(value: string) {
    return candidateTypes.find(option => option.value === normalizeCandidateType(value))?.label || value;
  }

  function evidenceLabel(value: string) {
    return ({ source_fact: '来源事实', source_rewrite: '原文润色', ai_suggestion: 'AI 建议补充' } as Record<string, string>)[value] || '来源事实';
  }

  function statusLabel(value: string) {
    return value === 'impact_review' ? '待影响发布' : '待人工初审';
  }

  function scopeLabel(value: string) {
    return scopes.find(option => option.value === value)?.label || value;
  }

  function sensitivityLabel(value: string) {
    return sensitivities.find(option => option.value === value)?.label || value;
  }

  function contentStartsWithTitle(candidate: CandidateDraft) {
    const firstLine = candidate.content.split('\n').find(line => line.trim())?.trim() || '';
    const heading = firstLine.match(/^#{1,6}\s+(.+?)\s*#*$/)?.[1]?.trim() || '';
    return !!heading && heading === candidate.title.trim();
  }

  function requiresImpactReview(candidate: CandidateDraft) {
    return ['high', 'restricted', 'critical'].includes(candidate.sensitivity)
      || candidate.evidence_kind === 'ai_suggestion'
      || (candidate.scope === 'global' && normalizeCandidateType(candidate.candidate_type) === 'architecture');
  }
</script>

{#if canRead}
  <section class="unified-corpus-review" aria-label="统一语料审核中心">
    <header class="review-header">
      <div>
        <h4>统一审核中心</h4>
        <p>逐条对照来源、修订候选并完成发布。普通语料一次审核，高敏感和全局架构规则需要二次影响确认。</p>
      </div>
      <Button variant="secondary" size="small" loading={loading} on:click={() => loadCandidates()}>刷新队列</Button>
    </header>

    <div class="review-counts" aria-label="审核队列统计">
      <span><b>{pendingCount}</b> 待人工初审</span>
      <span><b>{impactCount}</b> 待影响发布</span>
      <span><b>{candidateGroups.length}</b> 来源批次</span>
    </div>

    {#if error}<div class="review-message error" role="alert">{error}</div>{/if}
    {#if loading}
      <div class="review-loading" aria-live="polite"><span></span><span></span><span></span></div>
    {:else if !error && candidates.length === 0}
      <div class="review-empty">
        <strong>审核队列已清空</strong>
        <p>导入资料或完成自治交付后，新的原子语料候选会出现在这里。</p>
      </div>
    {:else if candidates.length > 0}
      <div class="review-layout">
        <aside class="candidate-queue" aria-label="候选队列">
          <div class="candidate-queue-head"><strong>按资料审核</strong><span>{candidates.length} 条候选</span></div>
          <div class="candidate-queue-list">
            {#each candidateGroups as group (group.key)}
              <section class="source-batch" aria-label={`${group.title}，${group.candidates.length} 条候选`}>
                <div class="source-batch-head">
                  <div>
                    <strong>{group.title}</strong>
                    <small>{group.originalName}{#if group.version > 0} · v{group.version}{/if}</small>
                  </div>
                  <span>{group.candidates.length}</span>
                </div>
                <div class="source-batch-candidates">
                  {#each group.candidates as candidate (candidate.id)}
                    <button type="button" class:active={selectedID === candidate.id} on:click={() => selectCandidate(candidate)}>
                      <span class="candidate-state" class:impact={candidate.status === 'impact_review'}>{statusLabel(candidate.status)}</span>
                      <strong>{candidate.title}</strong>
                      <span class="candidate-row-meta">
                        <span>{typeLabel(candidate.candidate_type)}</span>
                        <span>{sensitivityLabel(candidate.sensitivity)}</span>
                      </span>
                    </button>
                  {/each}
                </div>
              </section>
            {/each}
          </div>
        </aside>

        {#if selectedCandidate && draft}
          <div class="review-workbench">
            <div class="review-workbench-head">
              <div>
                <span>
                  {statusLabel(selectedCandidate.status)}
                  {#if selectedGroup} · 本批第 {selectedGroupPosition}/{selectedGroup.candidates.length} 条{/if}
                </span>
                {#if !contentStartsWithTitle(draft)}<strong>{draft.title}</strong>{/if}
                <p>{draft.summary}</p>
                <small>
                  {selectedGroup?.title || selectedCandidate.source_document?.title || '交付归档'}
                  {#if selectedCandidate.source_document} · {selectedCandidate.source_document.original_name} · v{selectedCandidate.source_document.version}{/if}
                  {#if selectedCandidate.source_anchor} · {selectedCandidate.source_anchor}{/if}
                </small>
              </div>
              <div class="review-badges">
                <span>{evidenceLabel(draft.evidence_kind)}</span>
                <span class:warning={requiresImpactReview(draft)}>{requiresImpactReview(draft) ? '需要影响确认' : '普通发布'}</span>
                <span>{Math.round((Number(draft.confidence) || 0) * 100)}%</span>
              </div>
            </div>

            <div class="review-workbench-body">
              <details class="candidate-metadata" bind:open={metadataOpen}>
                <summary>
                  <span>编辑结构化信息</span>
                  <small>{typeLabel(draft.candidate_type)} · {scopeLabel(draft.scope)}{#if draft.scope_id} / {draft.scope_id}{/if} · {sensitivityLabel(draft.sensitivity)}</small>
                </summary>
                <div class="candidate-fields">
                  <label class="wide"><span>候选标题</span><input bind:value={draft.title} readonly={selectedCandidate.status === 'impact_review'} /></label>
                  <label class="wide"><span>候选摘要</span><input bind:value={draft.summary} readonly={selectedCandidate.status === 'impact_review'} /></label>
                  <Select
                    id={`candidate-type-${selectedCandidate.id}`}
                    label="资料类型"
                    options={candidateTypes}
                    bind:value={draft.candidate_type}
                    searchable={false}
                    compact={true}
                    disabled={selectedCandidate.status === 'impact_review'}
                  />
                  <Select
                    id={`candidate-scope-${selectedCandidate.id}`}
                    label="适用范围"
                    options={scopes}
                    bind:value={draft.scope}
                    searchable={false}
                    compact={true}
                    disabled={selectedCandidate.status === 'impact_review'}
                    on:change={(event) => handleScopeChange(event.detail)}
                  />
                  <label><span>范围标识</span><input bind:value={draft.scope_id} disabled={selectedCandidate.status === 'impact_review' || draft.scope === 'global'} placeholder={draft.scope === 'global' ? '全局候选可留空' : 'repo / module / demand type'} /></label>
                  <Select
                    id={`candidate-sensitivity-${selectedCandidate.id}`}
                    label="敏感级别"
                    options={sensitivities}
                    bind:value={draft.sensitivity}
                    searchable={false}
                    compact={true}
                    disabled={selectedCandidate.status === 'impact_review'}
                  />
                  <label class="wide"><span>来源定位</span><input bind:value={draft.source_anchor} readonly={selectedCandidate.status === 'impact_review'} placeholder="原文标题、段落或章节定位" /></label>
                </div>
              </details>

              {#if selectedCandidate.status === 'impact_review'}
                {#if canPreviewImpact}
                  <div class="impact-notice">
                    <strong>发布前影响确认</strong>
                    <p>{impact?.reason || (impactLoading ? '正在读取当前生效上下文…' : '请刷新影响对照后再发布。')}</p>
                    <span>当前命中 {impact?.current_facts?.length || 0} 条同范围生效语料</span>
                  </div>
                  <section class="review-section" aria-labelledby="impact-comparison-title">
                    <div class="review-section-head">
                      <div>
                        <strong id="impact-comparison-title">上下文影响对照</strong>
                        <p>先核对当前生效规则，再确认本次拟发布的完整结果。</p>
                      </div>
                    </div>
                    <div class="comparison-grid">
                      <div class="comparison-pane">
                        <span class="comparison-label">当前生效上下文</span>
                        <MarkdownWorkbench
                          value={leftComparisonMarkdown}
                          mode="preview"
                          readonly={true}
                          label="当前生效上下文"
                          description="同类型、同范围的当前 active Context Fact"
                          minHeight={360}
                        />
                      </div>
                      <div class="comparison-pane">
                        <span class="comparison-label">拟发布结果</span>
                        <MarkdownWorkbench
                          value={draft.content}
                          mode="preview"
                          readonly={true}
                          label="拟发布结果"
                          description="本次发布将写入上下文的完整结果"
                          minHeight={360}
                        />
                      </div>
                    </div>
                  </section>
                {:else}
                  <div class="impact-notice blocked" role="alert">缺少 `ai_context:preview` 权限，不能完成二次影响确认和正式发布。</div>
                {/if}
              {:else}
                <section class="review-section" aria-labelledby="candidate-proposal-title">
                  <div class="review-section-head">
                    <div>
                      <strong id="candidate-proposal-title">拟发布语料</strong>
                      <p>直接审核和修订这一份候选；来源原文仅在需要核验时展开。</p>
                    </div>
                    <span>Markdown</span>
                  </div>
                  <MarkdownWorkbench
                    value={draft.content}
                    mode={proposalMode}
                    readonly={!canReview}
                    label="拟发布候选"
                    description="人工修订只更新候选，正式事实由审核动作创建"
                    minHeight={420}
                    on:change={(event) => { if (draft) draft.content = event.detail; }}
                  />
                </section>
              {/if}

              <details class="source-evidence" bind:open={sourceEvidenceOpen} on:toggle={handleSourceEvidenceToggle}>
                <summary>
                  <span>查看不可变来源证据</span>
                  <small>
                    {selectedCandidate.source_document?.original_name || '交付归档'}
                    {#if draft.source_anchor} · {draft.source_anchor}{/if}
                  </small>
                </summary>
                {#if impactLoading && !impact}
                  <div class="evidence-loading" aria-live="polite">正在加载来源证据…</div>
                {:else if impact?.source_markdown}
                  <MarkdownWorkbench value={impact.source_markdown} mode="preview" readonly={true} label="来源原文" minHeight={320} />
                {:else if !canPreviewImpact}
                  <div class="evidence-loading">当前账号没有查看完整来源正文的权限。</div>
                {:else}
                  <div class="evidence-loading">展开后读取不可变来源原文。</div>
                {/if}
              </details>

              <label class="review-note">
                <span>审核意见</span>
                <textarea rows="3" bind:value={reviewNote} placeholder="记录接受、拒绝、修订或影响确认的理由"></textarea>
              </label>
            </div>

            {#if canReview}
              <div class="review-actions">
                <Button variant="danger" disabled={activeID === selectedCandidate.id} on:click={() => review('rejected')}>拒绝候选</Button>
                <div>
                  {#if selectedCandidate.status === 'impact_review'}
                    <Button variant="secondary" loading={impactLoading} disabled={!canPreviewImpact} on:click={() => loadImpact()}>刷新影响对照</Button>
                    <Button variant="primary" loading={activeID === selectedCandidate.id} disabled={!canPreviewImpact || !impact} on:click={publish}>确认影响并发布</Button>
                  {:else}
                    <Button variant="primary" loading={activeID === selectedCandidate.id} on:click={() => review('accepted')}>
                      {requiresImpactReview(draft) ? '批准并进入影响审核' : '接受并发布'}
                    </Button>
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/if}
  </section>
{/if}

<style>
  .unified-corpus-review { min-width: 0; display: grid; gap: 16px; color: var(--scw-text, #293847); }
  .review-header, .review-workbench-head, .candidate-queue-head, .review-actions, .review-actions > div { min-width: 0; display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
  .review-header { padding-bottom: 14px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .review-header h4 { margin: 0; color: var(--scw-ink, #0d1722); font-size: 15px; text-wrap: balance; }
  .review-header p { max-width: 76ch; margin: 5px 0 0; color: var(--scw-muted, #667789); font-size: 12px; line-height: 1.5; text-wrap: pretty; }
  .review-counts { display: flex; flex-wrap: wrap; gap: 8px 22px; padding: 2px 0 14px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); color: var(--scw-muted, #667789); font-size: 11px; }
  .review-counts b { margin-right: 4px; color: var(--scw-ink, #0d1722); font: 760 13px/1 var(--wa-font-mono, monospace); }
  .review-layout { --review-panel-height: clamp(520px, calc(100dvh - 260px), 700px); min-width: 0; display: grid; grid-template-columns: minmax(280px, 340px) minmax(0, 1fr); align-items: stretch; gap: 20px; }
  .candidate-queue { position: sticky; top: 12px; min-width: 0; height: var(--review-panel-height); box-sizing: border-box; display: grid; grid-template-rows: auto minmax(0, 1fr); gap: 0; overflow: hidden; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 10px; background: rgba(251, 253, 254, .46); }
  .candidate-queue-head { align-items: center; padding: 12px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .candidate-queue-head strong { color: var(--scw-ink, #0d1722); font-size: 13px; }
  .candidate-queue-head span { color: var(--scw-muted, #667789); font: 11px/1 var(--wa-font-mono, monospace); }
  .candidate-queue-list { min-height: 0; max-height: none; overflow: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
  .source-batch { min-width: 0; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .source-batch:last-child { border-bottom: 0; }
  .source-batch-head { min-width: 0; display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; padding: 10px 12px 8px; background: rgba(233, 241, 244, .62); }
  .source-batch-head > div { min-width: 0; display: grid; gap: 3px; }
  .source-batch-head strong { overflow: hidden; color: var(--scw-ink, #0d1722); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
  .source-batch-head small { overflow: hidden; color: var(--scw-muted, #667789); font: 9px/1.4 var(--wa-font-mono, monospace); text-overflow: ellipsis; white-space: nowrap; }
  .source-batch-head > span { min-width: 24px; height: 22px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 999px; background: rgba(251, 253, 254, .76); color: var(--scw-muted, #667789); font: 10px/1 var(--wa-font-mono, monospace); }
  .source-batch-candidates { min-width: 0; }
  .source-batch-candidates > button { width: 100%; min-width: 0; display: grid; gap: 5px; padding: 10px 12px; border: 0; border-top: 1px solid var(--scw-line, rgba(92, 116, 137, .12)); background: transparent; color: var(--scw-text, #293847); text-align: left; cursor: pointer; }
  .source-batch-candidates > button:hover { background: rgba(0, 143, 150, .035); }
  .source-batch-candidates > button.active { background: rgba(0, 143, 150, .075); box-shadow: inset 0 0 0 1px rgba(0, 143, 150, .16); }
  .source-batch-candidates > button:focus-visible { position: relative; z-index: 1; outline: 2px solid rgba(0, 143, 150, .34); outline-offset: -2px; }
  .source-batch-candidates strong { overflow-wrap: anywhere; color: var(--scw-ink, #0d1722); font-size: 12px; line-height: 1.4; }
  .candidate-state { width: fit-content; color: var(--scw-accent-strong, #006f76); font-size: 10px; font-weight: 760; }
  .candidate-state.impact { color: var(--wa-warning, #b66d00); }
  .candidate-row-meta { display: flex; flex-wrap: wrap; gap: 5px 9px; color: var(--scw-subtle, #8a99aa); font: 10px/1.35 var(--wa-font-mono, monospace); }
  .review-workbench { min-width: 0; height: var(--review-panel-height); box-sizing: border-box; display: grid; grid-template-rows: auto minmax(0, 1fr) auto; gap: 0; overflow: hidden; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 10px; background: rgba(251, 253, 254, .46); }
  .review-workbench-head { align-items: center; padding: 12px 14px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .review-workbench-body { min-height: 0; display: grid; align-content: start; gap: 16px; overflow: auto; overscroll-behavior: contain; scrollbar-gutter: stable; padding: 14px; }
  .review-workbench-head > div:first-child { min-width: 0; display: grid; gap: 4px; }
  .review-workbench-head > div:first-child > span { color: var(--scw-accent-strong, #006f76); font-size: 10px; font-weight: 760; }
  .review-workbench-head strong { overflow-wrap: anywhere; color: var(--scw-ink, #0d1722); font-size: 14px; }
  .review-workbench-head p { max-width: 78ch; margin: 0; color: var(--scw-muted, #667789); font-size: 11px; line-height: 1.45; }
  .review-workbench-head small { color: var(--scw-muted, #667789); font-size: 10px; line-height: 1.4; }
  .review-badges { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 6px; }
  .review-badges span { min-height: 24px; display: inline-flex; align-items: center; border-radius: 999px; background: rgba(0, 143, 150, .09); color: var(--scw-accent-strong, #006f76); padding: 0 8px; font-size: 10px; font-weight: 700; }
  .review-badges span.warning { background: rgba(182, 109, 0, .1); color: #86530b; }
  .candidate-metadata, .source-evidence { min-width: 0; border-top: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .candidate-metadata summary, .source-evidence summary { min-width: 0; padding: 11px 2px; color: var(--scw-accent-strong, #006f76); cursor: pointer; }
  .candidate-metadata summary > span, .source-evidence summary > span { margin-right: 8px; font-size: 12px; font-weight: 760; }
  .candidate-metadata summary > small, .source-evidence summary > small { color: var(--scw-muted, #667789); font-size: 10px; font-weight: 500; line-height: 1.4; }
  .candidate-metadata[open] summary, .source-evidence[open] summary { border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .12)); }
  .candidate-fields { min-width: 0; display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px 16px; }
  .candidate-metadata .candidate-fields { padding: 14px 0 2px; }
  .candidate-fields label, .review-note { min-width: 0; display: grid; gap: 7px; color: var(--scw-text, #293847); font-size: 12px; font-weight: 700; }
  .candidate-fields :global(.select-group) { min-width: 0; }
  .candidate-fields label.wide { grid-column: span 2; }
  .candidate-fields input, .review-note textarea { width: 100%; min-height: 38px; box-sizing: border-box; border: 1px solid var(--scw-line-strong, rgba(69, 95, 118, .28)); border-radius: 8px; background: rgba(251, 253, 254, .84); color: var(--scw-ink, #0d1722); padding: 0 11px; font: 13px/1 var(--wa-font-sans, sans-serif); }
  .candidate-fields input:disabled, .candidate-fields input:read-only { background: rgba(239, 244, 246, .88); color: var(--scw-subtle, #8a99aa); }
  .candidate-fields input:focus, .review-note textarea:focus { outline: 2px solid rgba(0, 143, 150, .24); outline-offset: 1px; }
  .review-note textarea { min-height: 84px; padding: 10px 11px; line-height: 1.5; resize: vertical; }
  .review-section { min-width: 0; display: grid; gap: 10px; }
  .review-section-head { min-width: 0; display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; }
  .review-section-head > div { min-width: 0; display: grid; gap: 3px; }
  .review-section-head strong { color: var(--scw-ink, #0d1722); font-size: 12px; }
  .review-section-head p { margin: 0; color: var(--scw-muted, #667789); font-size: 10px; line-height: 1.4; }
  .review-section-head > span { color: var(--scw-subtle, #8a99aa); font: 10px/1.35 var(--wa-font-mono, monospace); }
  .impact-notice { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px 16px; padding: 11px 12px; border: 1px solid rgba(182, 109, 0, .22); border-radius: 8px; background: rgba(182, 109, 0, .07); color: #76501a; }
  .impact-notice strong { font-size: 12px; }
  .impact-notice p { margin: 0; font-size: 11px; line-height: 1.45; }
  .impact-notice span { font: 10px/1.35 var(--wa-font-mono, monospace); }
  .impact-notice.blocked { display: block; border-color: rgba(200, 66, 54, .24); background: rgba(200, 66, 54, .07); color: #96352d; font-size: 12px; }
  .comparison-grid { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr); align-items: start; gap: 16px; }
  .comparison-pane { min-width: 0; display: grid; gap: 7px; }
  .comparison-label { color: var(--scw-text, #293847); font-size: 12px; font-weight: 760; }
  .source-evidence :global(.markdown-workbench) { margin-top: 12px; }
  .evidence-loading { margin: 12px 0 2px; padding: 12px; border-radius: 8px; background: rgba(239, 244, 246, .72); color: var(--scw-muted, #667789); font-size: 11px; line-height: 1.45; }
  .review-actions { align-items: center; padding: 12px 14px; border-top: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .review-actions > div { justify-content: flex-end; gap: 8px; }
  .review-message { padding: 10px 12px; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 8px; font-size: 12px; line-height: 1.45; }
  .review-message.error { border-color: rgba(200, 66, 54, .24); background: rgba(200, 66, 54, .07); color: #96352d; }
  .review-loading { display: grid; gap: 9px; }
  .review-loading span { height: 72px; border-radius: 8px; background: rgba(226, 234, 238, .72); }
  .review-empty { display: grid; justify-items: center; gap: 7px; padding: 28px 18px; border: 1px dashed var(--scw-line-strong, rgba(69, 95, 118, .28)); border-radius: 10px; background: rgba(239, 246, 248, .48); color: var(--scw-muted, #667789); font-size: 12px; text-align: center; }
  .review-empty strong { color: var(--scw-text, #293847); }
  .review-empty p { margin: 0; }
  @media (max-width: 1180px) {
    .candidate-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
  @media (max-width: 960px) {
    .review-layout { grid-template-columns: minmax(0, 1fr); align-items: start; }
    .candidate-queue, .review-workbench { height: auto; }
    .candidate-queue { position: static; grid-template-rows: auto auto; }
    .candidate-queue-list { max-height: 320px; }
    .review-workbench { grid-template-rows: auto auto auto; overflow: visible; }
    .review-workbench-body { overflow: visible; scrollbar-gutter: auto; }
  }
  @media (max-width: 760px) {
    .review-header, .review-workbench-head, .review-actions, .review-actions > div, .impact-notice { display: grid; grid-template-columns: minmax(0, 1fr); }
    .review-section-head { align-items: flex-start; }
    .candidate-fields { grid-template-columns: minmax(0, 1fr); }
    .candidate-fields label.wide { grid-column: auto; }
    .review-badges { justify-content: flex-start; }
    .candidate-fields input, .candidate-fields :global(.select-trigger) { min-height: 44px; }
    .candidate-metadata summary, .source-evidence summary { min-height: 44px; box-sizing: border-box; }
    .review-actions :global(.btn), .review-header :global(.btn) { width: 100%; min-height: 44px; }
  }
</style>
