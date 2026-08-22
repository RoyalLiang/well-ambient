<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { showToast } from '../lib/toast';
  import MarkdownWorkbench from './shared/MarkdownWorkbench.svelte';
  import Select from './shared/Select.svelte';

  export let currentUserPermissions: string[] = [];

  type CatalogEntry = {
    id: number;
    demand_id: string;
    project_key: string;
    demand_title: string;
    solution_title: string;
    summary: string;
    content_bytes: number;
    stored_bytes: number;
    published_at: string;
    synced_at: string;
  };

  type CatalogProject = {
    project_key: string;
    count: number;
    last_synced: string;
  };

  type RoundOne = {
    equivalent: boolean;
    score: number;
    reason: string;
    shared_intent: string[];
    differences: string[];
  };

  type RoundTwo = {
    compatible: boolean;
    standardizable: boolean;
    score: number;
    summary: string;
    common_core: string[];
    conflicts: string[];
    proposal_title: string;
  };

  type Proposal = {
    proposal: {
      id: number;
      status: string;
      title: string;
      summary: string;
      reviewed_by: string;
      review_note: string;
      reviewed_at?: string | null;
    };
    markdown: string;
  };

  type ComparisonView = {
    comparison: {
      id: number;
      status: string;
      stage: string;
      recall_score: number;
      round_one_verdict: string;
      round_one_score: number;
      round_two_verdict: string;
      round_two_score: number;
      last_error: string;
    };
    counterpart: CatalogEntry;
    round_one?: RoundOne;
    round_two?: RoundTwo;
    proposal?: Proposal;
  };

  type CatalogDetail = {
    entry: CatalogEntry;
    markdown: string;
    demand_description: string;
    comparisons: ComparisonView[];
  };

  type StandardListItem = {
    standard: {
      id: number;
      title: string;
      summary: string;
      sequence: number;
      updated_at: string;
    };
    version: number;
    projects: string[];
    updated_at: string;
  };

  type StandardDetail = {
    standard: StandardListItem['standard'];
    revision: { id: number; version: number; created_by: string; created_at: string };
    markdown: string;
    projects: string[];
  };

  type CenterMode = 'catalog' | 'standards';
  type ReviewAction = 'accept' | 'reject';

  let mode: CenterMode = 'catalog';
  let projects: CatalogProject[] = [];
  let entries: CatalogEntry[] = [];
  let standards: StandardListItem[] = [];
  let total = 0;
  let projectKey = '';
  let searchInput = '';
  let committedSearch = '';
  let catalogLoading = true;
  let standardsLoading = true;
  let projectError = '';
  let catalogError = '';
  let standardsError = '';
  let detailLoading = false;
  let detailError = '';
  let selectedEntryID = 0;
  let selectedStandardID = 0;
  let detail: CatalogDetail | null = null;
  let standardDetail: StandardDetail | null = null;
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  let catalogRequestID = 0;
  let standardsRequestID = 0;
  let detailRequestID = 0;
  let reviewProposalID = 0;
  let reviewAction: ReviewAction | '' = '';
  let reviewNote = '';
  let reviewing = false;
  let detailPaneElement: HTMLElement | null = null;

  $: canReview = currentUserPermissions.includes('solution:publish');
  $: projectOptions = [
    { value: '', label: '全部项目', meta: `${projects.reduce((sum, item) => sum + item.count, 0)} 个方案` },
    ...projects.map((item) => ({ value: item.project_key, label: item.project_key, meta: `${item.count} 个已发布方案` }))
  ];
  $: filteredStandards = standards.filter((item) => {
    if (projectKey && !item.projects.includes(projectKey)) return false;
    const query = committedSearch.trim().toLowerCase();
    if (!query) return true;
    return `${item.standard.title} ${item.standard.summary} ${item.projects.join(' ')}`.toLowerCase().includes(query);
  });
  $: loadingList = mode === 'catalog' ? catalogLoading : standardsLoading;
  $: listError = projectError || (mode === 'catalog' ? catalogError : standardsError);
  $: visibleItemsCount = mode === 'catalog' ? entries.length : filteredStandards.length;

  onMount(() => {
    const handlePreferences = () => void reloadAll();
    window.addEventListener('project-preferences-updated', handlePreferences);
    void reloadAll();
    return () => window.removeEventListener('project-preferences-updated', handlePreferences);
  });

  onDestroy(() => {
    if (searchTimer) clearTimeout(searchTimer);
  });

  async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await fetch(path, init);
    if (!response.ok) {
      const message = (await response.text()).trim() || `HTTP ${response.status}`;
      throw new Error(message);
    }
    return response.json() as Promise<T>;
  }

  async function reloadAll() {
    selectedEntryID = 0;
    selectedStandardID = 0;
    detail = null;
    standardDetail = null;
    await Promise.all([loadProjects(), loadEntries(), loadStandards()]);
  }

  async function loadProjects() {
    try {
      const response = await api<{ items: CatalogProject[] }>('/api/solution-catalog/projects');
      projects = response.items || [];
      projectError = '';
      if (projectKey && !projects.some((item) => item.project_key === projectKey)) projectKey = '';
    } catch (error) {
      projects = [];
      projectError = errorMessage(error, '项目范围加载失败');
    }
  }

  async function loadEntries(preferredID = 0) {
    const requestID = ++catalogRequestID;
    catalogLoading = true;
    catalogError = '';
    try {
      const params = new URLSearchParams({ limit: '100' });
      if (projectKey) params.set('project', projectKey);
      if (committedSearch) params.set('q', committedSearch);
      const response = await api<{ items: CatalogEntry[]; total: number }>(`/api/solution-catalog?${params.toString()}`);
      if (requestID !== catalogRequestID) return;
      entries = response.items || [];
      total = response.total || 0;
      const nextID = preferredID && entries.some((item) => item.id === preferredID)
        ? preferredID
        : entries[0]?.id || 0;
      if (nextID) {
        await selectEntry(nextID);
      } else {
        selectedEntryID = 0;
        detail = null;
        detailError = '';
      }
    } catch (error) {
      if (requestID !== catalogRequestID) return;
      entries = [];
      total = 0;
      catalogError = errorMessage(error, '方案目录加载失败');
    } finally {
      if (requestID === catalogRequestID) catalogLoading = false;
    }
  }

  async function loadStandards(preferredID = 0) {
    const requestID = ++standardsRequestID;
    standardsLoading = true;
    standardsError = '';
    try {
      const response = await api<{ items: StandardListItem[]; total: number }>('/api/solution-standards');
      if (requestID !== standardsRequestID) return;
      standards = response.items || [];
      const visible = filterStandardItems(standards, projectKey, committedSearch);
      const nextID = preferredID && visible.some((item) => item.standard.id === preferredID)
        ? preferredID
        : visible[0]?.standard.id || 0;
      if (mode === 'standards' && nextID) {
        await selectStandard(nextID);
      } else if (mode === 'standards') {
        selectedStandardID = 0;
        standardDetail = null;
      }
    } catch (error) {
      if (requestID !== standardsRequestID) return;
      standards = [];
      standardsError = errorMessage(error, '标准方案加载失败');
    } finally {
      if (requestID === standardsRequestID) standardsLoading = false;
    }
  }

  async function selectEntry(id: number) {
    resetDetailScroll();
    selectedEntryID = id;
    detail = null;
    detailError = '';
    resetReview();
    const requestID = ++detailRequestID;
    detailLoading = true;
    try {
      const response = await api<CatalogDetail>(`/api/solution-catalog/${id}`);
      if (requestID === detailRequestID && selectedEntryID === id) detail = response;
    } catch (error) {
      if (requestID === detailRequestID) detailError = errorMessage(error, '方案详情加载失败');
    } finally {
      if (requestID === detailRequestID) detailLoading = false;
    }
  }

  async function selectStandard(id: number) {
    resetDetailScroll();
    selectedStandardID = id;
    standardDetail = null;
    detailError = '';
    const requestID = ++detailRequestID;
    detailLoading = true;
    try {
      const response = await api<StandardDetail>(`/api/solution-standards/${id}`);
      if (requestID === detailRequestID && selectedStandardID === id) standardDetail = response;
    } catch (error) {
      if (requestID === detailRequestID) detailError = errorMessage(error, '标准方案详情加载失败');
    } finally {
      if (requestID === detailRequestID) detailLoading = false;
    }
  }

  function changeMode(next: CenterMode) {
    if (mode === next) return;
    resetDetailScroll();
    mode = next;
    catalogError = '';
    standardsError = '';
    detailError = '';
    if (next === 'catalog') {
      void loadEntries(selectedEntryID);
    } else {
      const nextID = filterStandardItems(standards, projectKey, committedSearch)[0]?.standard.id || 0;
      if (nextID) void selectStandard(nextID);
    }
  }

  function handleProjectChange(event: CustomEvent<string>) {
    projectKey = event.detail;
    if (mode === 'catalog') {
      void loadEntries();
    } else {
      const nextID = filterStandardItems(standards, projectKey, committedSearch)[0]?.standard.id || 0;
      if (nextID) void selectStandard(nextID);
      else {
        selectedStandardID = 0;
        standardDetail = null;
      }
    }
  }

  function handleSearch(event: Event) {
    searchInput = (event.currentTarget as HTMLInputElement).value;
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      committedSearch = searchInput.trim();
      if (mode === 'catalog') {
        void loadEntries();
      } else {
        const nextID = filterStandardItems(standards, projectKey, committedSearch)[0]?.standard.id || 0;
        if (nextID) void selectStandard(nextID);
        else {
          selectedStandardID = 0;
          standardDetail = null;
        }
      }
    }, 250);
  }

  function beginReview(proposalID: number, action: ReviewAction) {
    reviewProposalID = proposalID;
    reviewAction = action;
    reviewNote = '';
  }

  function resetReview() {
    reviewProposalID = 0;
    reviewAction = '';
    reviewNote = '';
  }

  function resetDetailScroll() {
    detailPaneElement?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
  }

  async function submitReview() {
    if (!reviewProposalID || !reviewAction || reviewing) return;
    reviewing = true;
    try {
      await api(`/api/solution-catalog/proposals/${reviewProposalID}/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: reviewAction, review_note: reviewNote.trim() })
      });
      showToast(reviewAction === 'accept' ? '提案已纳入标准方案库' : '提案已驳回并保留审计记录', {
        type: 'success', title: '审核完成'
      });
      const currentID = selectedEntryID;
      resetReview();
      await Promise.all([selectEntry(currentID), loadStandards()]);
    } catch (error) {
      showToast(errorMessage(error, '提案审核失败'), { type: 'error', title: '审核失败' });
    } finally {
      reviewing = false;
    }
  }

  function retryList() {
    if (mode === 'catalog') void Promise.all([loadProjects(), loadEntries()]);
    else void Promise.all([loadProjects(), loadStandards()]);
  }

  function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
  }

  function filterStandardItems(items: StandardListItem[], selectedProject: string, queryText: string) {
    const query = queryText.trim().toLowerCase();
    return items.filter((item) => {
      if (selectedProject && !item.projects.includes(selectedProject)) return false;
      if (!query) return true;
      return `${item.standard.title} ${item.standard.summary} ${item.projects.join(' ')}`.toLowerCase().includes(query);
    });
  }

  function formatTime(value: string) {
    if (!value) return '未记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat('zh-CN', {
      month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false
    }).format(date);
  }

  function percentage(value: number) {
    return `${Math.round(Math.max(0, Math.min(1, value || 0)) * 100)}%`;
  }

  function comparisonLabel(comparison: ComparisonView) {
    if (comparison.proposal?.proposal.status === 'pending') return '待审核';
    if (comparison.proposal?.proposal.status === 'accepted') return '已标准化';
    if (comparison.proposal?.proposal.status === 'rejected') return '已驳回';
    if (comparison.comparison.status === 'failed') return '对比失败';
    if (comparison.comparison.status === 'needs_review') return '待审核';
    if (comparison.comparison.status === 'completed') return '对比完成';
    return '对比中';
  }
</script>

<section class="solution-center" aria-label="方案治理中心">
  <header class="center-toolbar">
    <div class="mode-switch" aria-label="方案中心视图">
      <button type="button" class:active={mode === 'catalog'} aria-pressed={mode === 'catalog'} on:click={() => changeMode('catalog')}>
        已发布方案
      </button>
      <button type="button" class:active={mode === 'standards'} aria-pressed={mode === 'standards'} on:click={() => changeMode('standards')}>
        标准方案
      </button>
    </div>

    <label class="search-control">
      <span class="wa-icon icon-search" aria-hidden="true"></span>
      <span class="sr-only">搜索方案</span>
      <input
        type="search"
        value={searchInput}
        placeholder={mode === 'catalog' ? '搜索需求、方案或项目' : '搜索标准方案'}
        on:input={handleSearch}
      />
    </label>

    <div class="project-filter">
      <Select
        id="solution-center-project"
        value={projectKey}
        options={projectOptions}
        placeholder="全部项目"
        ariaLabel="按项目筛选方案"
        searchable={projects.length > 6}
        compact
        shadowless
        on:change={handleProjectChange}
      />
    </div>

    <div class="result-count" aria-live="polite">
      <strong>{mode === 'catalog' ? total : visibleItemsCount}</strong>
      <span>{mode === 'catalog' ? '个已发布方案' : '个标准方案'}</span>
    </div>
  </header>

  <div class="center-grid">
    <aside class="register-pane" aria-label={mode === 'catalog' ? '已发布方案列表' : '标准方案列表'}>
      {#if loadingList}
        <div class="register-skeleton" aria-label="正在加载方案列表">
          {#each Array(7) as _}
            <div class="skeleton-row"><i></i><span></span><b></b></div>
          {/each}
        </div>
      {:else if listError}
        <div class="state-panel compact-state" role="alert">
          <strong>目录暂时不可用</strong>
          <p>{listError}</p>
          <button type="button" on:click={retryList}>重新加载</button>
        </div>
      {:else if mode === 'catalog' && entries.length === 0}
        <div class="state-panel compact-state">
          <strong>暂无已发布方案</strong>
          <p>{committedSearch || projectKey ? '当前筛选范围内没有匹配内容。' : '需求方案发布后会自动进入这里。'}</p>
        </div>
      {:else if mode === 'standards' && filteredStandards.length === 0}
        <div class="state-panel compact-state">
          <strong>暂无标准方案</strong>
          <p>两轮对比通过并经人工审核后，标准方案会出现在这里。</p>
        </div>
      {:else if mode === 'catalog'}
        <div class="register-list">
          {#each entries as entry (entry.id)}
            <button
              type="button"
              class="register-row"
              class:active={entry.id === selectedEntryID}
              aria-current={entry.id === selectedEntryID ? 'true' : undefined}
              on:click={() => selectEntry(entry.id)}
            >
              <span class="row-topline"><b>{entry.project_key}</b><time datetime={entry.published_at}>{formatTime(entry.published_at)}</time></span>
              <strong>{entry.solution_title || entry.demand_title || entry.demand_id}</strong>
              <span class="row-summary">{entry.summary || '已发布方案，暂无摘要。'}</span>
              <span class="row-meta"><code>{entry.demand_id}</code><i>{Math.round((entry.stored_bytes / Math.max(entry.content_bytes, 1)) * 100)}% 存储</i></span>
            </button>
          {/each}
        </div>
      {:else}
        <div class="register-list">
          {#each filteredStandards as item (item.standard.id)}
            <button
              type="button"
              class="register-row"
              class:active={item.standard.id === selectedStandardID}
              aria-current={item.standard.id === selectedStandardID ? 'true' : undefined}
              on:click={() => selectStandard(item.standard.id)}
            >
              <span class="row-topline"><b>STD-{item.standard.id}</b><time datetime={item.updated_at}>{formatTime(item.updated_at)}</time></span>
              <strong>{item.standard.title}</strong>
              <span class="row-summary">{item.standard.summary || '经审核的可复用标准方案。'}</span>
              <span class="row-meta"><span>{item.projects.join(' · ')}</span><i>v{item.version}</i></span>
            </button>
          {/each}
        </div>
      {/if}
    </aside>

    <article class="detail-pane" bind:this={detailPaneElement} aria-live="polite">
      {#if detailLoading}
        <div class="detail-skeleton" aria-label="正在加载方案详情">
          <i></i><span class="skeleton-title"></span><p></p><p></p><div></div>
        </div>
      {:else if detailError}
        <div class="state-panel" role="alert">
          <strong>详情加载失败</strong>
          <p>{detailError}</p>
          <button type="button" on:click={() => mode === 'catalog' ? selectEntry(selectedEntryID) : selectStandard(selectedStandardID)}>重新加载</button>
        </div>
      {:else if mode === 'catalog' && detail}
        <div class="detail-content">
          <header class="detail-header">
            <div class="detail-identity">
              <span class="eyebrow">{detail.entry.project_key} · {detail.entry.demand_id}</span>
              <h2>{detail.entry.solution_title || detail.entry.demand_title || detail.entry.demand_id}</h2>
              {#if detail.entry.summary}<p>{detail.entry.summary}</p>{/if}
            </div>
            <a class="demand-link" href={`/?tab=schedule&demand=${encodeURIComponent(detail.entry.demand_id)}&solution=1`}>
              打开需求 <span aria-hidden="true">↗</span>
            </a>
          </header>

          <dl class="fact-strip">
            <div><dt>发布时间</dt><dd>{formatTime(detail.entry.published_at)}</dd></div>
            <div><dt>目录同步</dt><dd>{formatTime(detail.entry.synced_at)}</dd></div>
            <div><dt>原始大小</dt><dd>{Math.max(1, Math.round(detail.entry.content_bytes / 1024))} KB</dd></div>
            <div><dt>保存大小</dt><dd>{Math.max(1, Math.round(detail.entry.stored_bytes / 1024))} KB</dd></div>
          </dl>

          {#if detail.demand_description}
            <section class="detail-section requirement-context">
              <div class="section-heading"><div><span>需求背景</span><h3>对比所依据的需求事实</h3></div></div>
              <p>{detail.demand_description}</p>
            </section>
          {/if}

          <section class="detail-section markdown-section">
            <div class="section-heading"><div><span>方案正文</span><h3>当前已发布版本</h3></div></div>
            <MarkdownWorkbench
              value={detail.markdown}
              mode="live"
              availableModes={['live']}
              readonly
              label="方案正文"
              description=""
              showToolbar={false}
              minHeight={300}
              autoHeight
            />
          </section>

          <section class="detail-section comparison-section">
            <div class="section-heading">
              <div><span>相似方案</span><h3>两轮对比与标准化建议</h3></div>
              <em>{detail.comparisons.length} 组</em>
            </div>
            {#if detail.comparisons.length === 0}
              <div class="inline-empty">暂未发现达到召回阈值的相似已发布方案。</div>
            {:else}
              <div class="comparison-list">
                {#each detail.comparisons as comparison (comparison.comparison.id)}
                  <article class="comparison-row">
                    <header>
                      <div>
                        <span>{comparison.counterpart.project_key} · {comparison.counterpart.demand_id}</span>
                        <h4>{comparison.counterpart.solution_title || comparison.counterpart.demand_title}</h4>
                      </div>
                      <b class:pending={comparisonLabel(comparison) === '待审核'}>{comparisonLabel(comparison)}</b>
                    </header>
                    <div class="score-line">
                      <span>初筛 {percentage(comparison.comparison.recall_score)}</span>
                      {#if comparison.round_one}<span>需求一致 {percentage(comparison.round_one.score)}</span>{/if}
                      {#if comparison.round_two}<span>方案兼容 {percentage(comparison.round_two.score)}</span>{/if}
                    </div>
                    {#if comparison.round_one?.reason}<p>{comparison.round_one.reason}</p>{/if}
                    {#if comparison.round_two?.summary}<p>{comparison.round_two.summary}</p>{/if}
                    {#if comparison.comparison.last_error}<p class="comparison-error">{comparison.comparison.last_error}</p>{/if}

                    {#if comparison.proposal}
                      <div class="proposal-block">
                        <div class="proposal-heading">
                          <div><span>标准化提案</span><strong>{comparison.proposal.proposal.title}</strong></div>
                          <em class="status-{comparison.proposal.proposal.status}">{comparisonLabel(comparison)}</em>
                        </div>
                        {#if comparison.proposal.proposal.summary}<p>{comparison.proposal.proposal.summary}</p>{/if}
                        <MarkdownWorkbench
                          value={comparison.proposal.markdown}
                          mode="live"
                          availableModes={['live']}
                          readonly
                          label="标准化提案"
                          description=""
                          showToolbar={false}
                          minHeight={220}
                          autoHeight
                        />
                        {#if comparison.proposal.proposal.status === 'pending' && canReview}
                          {#if reviewProposalID === comparison.proposal.proposal.id}
                            <div class="review-confirmation">
                              <label for={`proposal-note-${comparison.proposal.proposal.id}`}>审核说明</label>
                              <textarea
                                id={`proposal-note-${comparison.proposal.proposal.id}`}
                                bind:value={reviewNote}
                                rows="3"
                                placeholder={reviewAction === 'accept' ? '可选：记录纳入标准库的依据' : '请记录驳回原因'}
                              ></textarea>
                              <div class="review-actions">
                                <button type="button" class="quiet-button" disabled={reviewing} on:click={resetReview}>取消</button>
                                <button type="button" class:danger={reviewAction === 'reject'} class="primary-button" disabled={reviewing || (reviewAction === 'reject' && !reviewNote.trim())} on:click={submitReview}>
                                  {reviewing ? '正在提交…' : reviewAction === 'accept' ? '确认纳入标准库' : '确认驳回'}
                                </button>
                              </div>
                            </div>
                          {:else}
                            <div class="proposal-actions">
                              <button type="button" class="quiet-button" on:click={() => beginReview(comparison.proposal!.proposal.id, 'reject')}>驳回</button>
                              <button type="button" class="primary-button" on:click={() => beginReview(comparison.proposal!.proposal.id, 'accept')}>审核通过</button>
                            </div>
                          {/if}
                        {:else if comparison.proposal.proposal.status !== 'pending'}
                          <div class="review-audit">
                            <span>{comparison.proposal.proposal.reviewed_by || '已审核'}</span>
                            {#if comparison.proposal.proposal.review_note}<p>{comparison.proposal.proposal.review_note}</p>{/if}
                          </div>
                        {/if}
                      </div>
                    {/if}
                  </article>
                {/each}
              </div>
            {/if}
          </section>
        </div>
      {:else if mode === 'standards' && standardDetail}
        <div class="detail-content">
          <header class="detail-header">
            <div class="detail-identity">
              <span class="eyebrow">STD-{standardDetail.standard.id} · v{standardDetail.revision.version}</span>
              <h2>{standardDetail.standard.title}</h2>
              {#if standardDetail.standard.summary}<p>{standardDetail.standard.summary}</p>{/if}
            </div>
          </header>
          <dl class="fact-strip">
            <div><dt>适用项目</dt><dd>{standardDetail.projects.join(' · ')}</dd></div>
            <div><dt>当前版本</dt><dd>v{standardDetail.revision.version}</dd></div>
            <div><dt>审核人</dt><dd>{standardDetail.revision.created_by}</dd></div>
            <div><dt>更新时间</dt><dd>{formatTime(standardDetail.revision.created_at)}</dd></div>
          </dl>
          <section class="detail-section markdown-section">
            <div class="section-heading"><div><span>标准正文</span><h3>审核通过的可复用方案</h3></div></div>
            <MarkdownWorkbench
              value={standardDetail.markdown}
              mode="live"
              availableModes={['live']}
              readonly
              label="标准方案正文"
              description=""
              showToolbar={false}
              minHeight={360}
              autoHeight
            />
          </section>
        </div>
      {:else}
        <div class="state-panel">
          <strong>选择一条记录查看详情</strong>
          <p>详情会在当前右侧区域更新，不会打开新标签页。</p>
        </div>
      {/if}
    </article>
  </div>
</section>

<style>
  .solution-center {
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--wa-border-panel);
    border-radius: var(--wa-radius-lg);
    background: var(--wa-surface-panel);
    box-shadow: var(--wa-shadow-peer);
  }

  .center-toolbar {
    min-height: 60px;
    display: grid;
    grid-template-columns: auto minmax(220px, 1fr) minmax(180px, 240px) auto;
    gap: 12px;
    align-items: center;
    padding: 11px 14px;
    border-bottom: 1px solid var(--wa-border-divider);
    background: var(--wa-surface-flat);
  }

  .mode-switch {
    display: inline-flex;
    padding: 3px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
  }

  .mode-switch button,
  .quiet-button,
  .primary-button,
  .state-panel button {
    min-height: var(--wa-control-h);
    border: 0;
    border-radius: var(--wa-radius-sm);
    padding: 0 13px;
    font: inherit;
    font-size: 13px;
    font-weight: 720;
    cursor: pointer;
  }

  .mode-switch button {
    background: transparent;
    color: var(--wa-text-muted);
  }

  .mode-switch button.active {
    background: var(--wa-surface-flat);
    color: var(--wa-accent-strong);
    box-shadow: var(--wa-shadow-sm);
  }

  .search-control {
    min-width: 0;
    height: var(--wa-control-h);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 11px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-chrome-1);
  }

  .search-control:focus-within {
    border-color: var(--wa-border-focus);
    box-shadow: 0 0 0 3px var(--wa-accent-soft);
  }

  .search-control .wa-icon { width: 16px; height: 16px; color: var(--wa-text-subtle); }

  .search-control input {
    min-width: 0;
    width: 100%;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--wa-text-main);
    font: inherit;
    font-size: 13px;
  }

  .project-filter { min-width: 0; }
  .result-count { display: flex; align-items: baseline; justify-content: flex-end; gap: 5px; white-space: nowrap; color: var(--wa-text-muted); font-size: 12px; }
  .result-count strong { color: var(--wa-text-strong); font-family: var(--wa-font-mono); font-size: 16px; }

  .center-grid {
    min-height: 0;
    flex: 1;
    display: grid;
    grid-template-columns: minmax(300px, 36%) minmax(0, 1fr);
  }

  .register-pane,
  .detail-pane { min-width: 0; min-height: 0; overflow-y: auto; }
  .register-pane { border-right: 1px solid var(--wa-border-divider); background: var(--wa-surface-flat); }
  .detail-pane { background: var(--wa-surface-panel); }

  .register-list { display: flex; flex-direction: column; }
  .register-row {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 7px;
    padding: 15px 16px;
    border: 0;
    border-bottom: 1px solid var(--wa-border-divider);
    background: transparent;
    color: var(--wa-text-main);
    text-align: left;
    cursor: pointer;
  }

  .register-row:hover { background: var(--wa-row-hover); }
  .register-row.active { background: var(--wa-row-active); box-shadow: inset 3px 0 var(--wa-accent); }
  .register-row:focus-visible { outline: 2px solid var(--wa-border-focus); outline-offset: -3px; }
  .register-row > strong { color: var(--wa-text-strong); font-size: 14px; line-height: 1.38; }
  .row-topline, .row-meta { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
  .row-topline { color: var(--wa-text-subtle); font-size: 11px; }
  .row-topline b { color: var(--wa-accent-strong); font-family: var(--wa-font-mono); letter-spacing: .04em; }
  .row-summary { display: -webkit-box; overflow: hidden; color: var(--wa-text-muted); font-size: 12px; line-height: 1.5; line-clamp: 2; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
  .row-meta { color: var(--wa-text-subtle); font-size: 11px; }
  .row-meta code { color: var(--wa-text-muted); font-family: var(--wa-font-mono); }
  .row-meta i { font-style: normal; }

  .detail-content { padding: 20px 22px 40px; }
  .detail-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; padding-bottom: 18px; }
  .detail-identity { min-width: 0; }
  .eyebrow, .section-heading span, .proposal-heading span { display: block; color: var(--wa-accent-strong); font-size: 11px; font-weight: 760; letter-spacing: .06em; text-transform: uppercase; }
  .detail-header h2 { margin: 7px 0 0; color: var(--wa-text-strong); font-size: clamp(20px, 2vw, 27px); line-height: 1.2; }
  .detail-header p { max-width: 760px; margin: 9px 0 0; color: var(--wa-text-muted); font-size: 13px; line-height: 1.6; }
  .demand-link { flex: 0 0 auto; min-height: var(--wa-control-h); display: inline-flex; align-items: center; gap: 6px; padding: 0 12px; border: 1px solid var(--wa-border-strong); border-radius: var(--wa-radius-sm); color: var(--wa-accent-strong); font-size: 12px; font-weight: 720; text-decoration: none; }
  .demand-link:hover { background: var(--wa-row-hover); }

  .fact-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin: 0; border-block: 1px solid var(--wa-border-divider); }
  .fact-strip div { min-width: 0; padding: 13px 14px; border-right: 1px solid var(--wa-border-divider); }
  .fact-strip div:first-child { padding-left: 0; }
  .fact-strip div:last-child { border-right: 0; }
  .fact-strip dt { color: var(--wa-text-subtle); font-size: 11px; }
  .fact-strip dd { overflow: hidden; margin: 4px 0 0; color: var(--wa-text-strong); font-size: 12px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }

  .detail-section { padding-top: 22px; }
  .detail-section + .detail-section { margin-top: 22px; border-top: 1px solid var(--wa-border-divider); }
  .section-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; margin-bottom: 13px; }
  .section-heading h3 { margin: 4px 0 0; color: var(--wa-text-strong); font-size: 15px; }
  .section-heading em { color: var(--wa-text-subtle); font-size: 12px; font-style: normal; }
  .requirement-context > p { margin: 0; padding-left: 13px; border-left: 2px solid var(--wa-accent); color: var(--wa-text-muted); font-size: 13px; line-height: 1.7; white-space: pre-wrap; }
  .markdown-section :global(.markdown-workbench), .proposal-block :global(.markdown-workbench) { border-color: var(--wa-border-divider); box-shadow: none; }

  .comparison-list { border-top: 1px solid var(--wa-border-divider); }
  .comparison-row { padding: 17px 0; border-bottom: 1px solid var(--wa-border-divider); }
  .comparison-row > header { display: flex; justify-content: space-between; gap: 14px; }
  .comparison-row > header span { color: var(--wa-text-subtle); font-size: 11px; }
  .comparison-row h4 { margin: 4px 0 0; color: var(--wa-text-strong); font-size: 14px; }
  .comparison-row > header > b { align-self: flex-start; padding: 4px 8px; border-radius: var(--wa-radius-pill); background: var(--wa-neutral-soft); color: var(--wa-text-muted); font-size: 11px; }
  .comparison-row > header > b.pending { background: var(--wa-warning-soft); color: var(--wa-warning); }
  .score-line { display: flex; flex-wrap: wrap; gap: 6px 14px; margin-top: 12px; color: var(--wa-text-muted); font-family: var(--wa-font-mono); font-size: 11px; }
  .comparison-row > p, .proposal-block > p { margin: 10px 0 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.6; }
  .comparison-error { color: var(--wa-danger) !important; }

  .proposal-block { margin-top: 15px; padding: 15px; border: 1px solid var(--wa-border-divider); border-radius: var(--wa-radius-md); background: var(--wa-surface-inset); box-shadow: inset 0 2px var(--wa-accent-soft); }
  .proposal-heading { display: flex; justify-content: space-between; gap: 12px; }
  .proposal-heading strong { display: block; margin-top: 4px; color: var(--wa-text-strong); font-size: 13px; }
  .proposal-heading em { align-self: flex-start; color: var(--wa-text-muted); font-size: 11px; font-style: normal; }
  .proposal-heading .status-accepted { color: var(--wa-success); }
  .proposal-heading .status-rejected { color: var(--wa-danger); }
  .proposal-block :global(.markdown-workbench) { margin-top: 13px; background: var(--wa-surface-flat); }
  .proposal-actions, .review-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 13px; }
  .quiet-button { border: 1px solid var(--wa-border-strong); background: transparent; color: var(--wa-text-main); }
  .primary-button, .state-panel button { background: var(--wa-accent-fill); color: var(--wa-accent-fill-ink); }
  .primary-button.danger { background: var(--wa-danger); }
  button:disabled { cursor: not-allowed; opacity: .55; }
  .review-confirmation { margin-top: 14px; padding-top: 13px; border-top: 1px solid var(--wa-border-divider); }
  .review-confirmation label { display: block; margin-bottom: 7px; color: var(--wa-text-muted); font-size: 12px; font-weight: 720; }
  .review-confirmation textarea { box-sizing: border-box; width: 100%; resize: vertical; padding: 10px; border: 1px solid var(--wa-border-soft); border-radius: var(--wa-radius-sm); background: var(--wa-surface-flat); color: var(--wa-text-main); font: inherit; font-size: 13px; }
  .review-audit { margin-top: 13px; padding-top: 11px; border-top: 1px solid var(--wa-border-divider); color: var(--wa-text-subtle); font-size: 11px; }
  .review-audit p { margin: 5px 0 0; color: var(--wa-text-muted); font-size: 12px; }

  .state-panel { min-height: 360px; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 24px; text-align: center; }
  .state-panel.compact-state { min-height: 260px; }
  .state-panel strong { color: var(--wa-text-strong); font-size: 15px; }
  .state-panel p { max-width: 420px; margin: 8px 0 16px; color: var(--wa-text-muted); font-size: 13px; line-height: 1.6; }
  .inline-empty { padding: 20px 0; color: var(--wa-text-muted); font-size: 13px; }

  .register-skeleton, .detail-skeleton { animation: pulse 1.4s ease-in-out infinite; }
  .skeleton-row { height: 112px; display: flex; flex-direction: column; gap: 10px; padding: 16px; border-bottom: 1px solid var(--wa-border-divider); }
  .skeleton-row i, .skeleton-row span, .skeleton-row b, .detail-skeleton > * { display: block; border-radius: 5px; background: var(--wa-neutral-soft); }
  .skeleton-row i { width: 28%; height: 9px; }
  .skeleton-row span { width: 76%; height: 14px; }
  .skeleton-row b { width: 94%; height: 26px; }
  .detail-skeleton { padding: 24px; }
  .detail-skeleton i { width: 120px; height: 11px; }
  .detail-skeleton .skeleton-title { width: 58%; height: 28px; margin-top: 14px; }
  .detail-skeleton p { width: 84%; height: 12px; margin-top: 13px; }
  .detail-skeleton div { width: 100%; height: 360px; margin-top: 34px; }

  .sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; }

  @keyframes pulse { 50% { opacity: .55; } }

  @media (max-width: 1100px) {
    .center-toolbar { grid-template-columns: auto minmax(180px, 1fr) minmax(170px, 210px); }
    .result-count { grid-column: 1 / -1; justify-content: flex-start; }
    .center-grid { grid-template-columns: minmax(280px, 38%) minmax(0, 1fr); }
    .fact-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .fact-strip div:nth-child(2) { border-right: 0; }
    .fact-strip div:nth-child(n + 3) { border-top: 1px solid var(--wa-border-divider); }
  }

  @media (max-width: 760px) {
    .solution-center { height: auto; min-height: 0; overflow: visible; }
    .center-toolbar { grid-template-columns: 1fr; align-items: stretch; }
    .mode-switch { width: 100%; }
    .mode-switch button { flex: 1; min-height: var(--wa-touch-h); }
    .search-control { height: var(--wa-touch-h); }
    .project-filter :global(.select-trigger) { min-height: var(--wa-touch-h); }
    .center-grid { display: block; }
    .register-pane, .detail-pane { overflow: visible; }
    .register-pane { max-height: 54dvh; overflow-y: auto; border-right: 0; border-bottom: 1px solid var(--wa-border-divider); }
    .detail-pane { min-height: 62dvh; }
    .register-row { min-height: 112px; }
    .detail-content { padding: 18px 15px 32px; }
    .detail-header { flex-direction: column; }
    .demand-link { min-height: var(--wa-touch-h); }
    .fact-strip { grid-template-columns: 1fr 1fr; }
    .fact-strip div { padding: 11px; }
    .fact-strip div:first-child { padding-left: 11px; }
    .proposal-actions button, .review-actions button { min-height: var(--wa-touch-h); }
  }

  @media (max-width: 420px) {
    .fact-strip { grid-template-columns: 1fr; }
    .fact-strip div, .fact-strip div:nth-child(2) { border-right: 0; border-top: 1px solid var(--wa-border-divider); }
    .fact-strip div:first-child { border-top: 0; }
    .comparison-row > header, .proposal-heading { flex-direction: column; }
    .proposal-actions, .review-actions { flex-direction: column-reverse; }
    .proposal-actions button, .review-actions button { width: 100%; }
  }

  @media (prefers-reduced-motion: reduce) {
    .register-skeleton, .detail-skeleton { animation: none; }
  }
</style>
