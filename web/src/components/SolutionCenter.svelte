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

  type CenterMode = 'catalog' | 'comparisons' | 'standards';
  type LocationIntent = { mode: 'catalog' | 'standards'; id: number };
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
  let selectedComparisonID = 0;
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
  $: filteredComparisons = filterComparisonItems(detail?.comparisons || [], projectKey, committedSearch);
  $: selectedComparison = filteredComparisons.find((item) => item.comparison.id === selectedComparisonID)
    || filteredComparisons[0]
    || null;
  $: loadingList = mode === 'catalog' ? catalogLoading : mode === 'standards' ? standardsLoading : detailLoading;
  $: listError = projectError || (mode === 'catalog' ? catalogError : mode === 'standards' ? standardsError : detailError);
  $: visibleItemsCount = mode === 'catalog' ? entries.length : mode === 'comparisons' ? filteredComparisons.length : filteredStandards.length;
  $: listLabel = mode === 'catalog' ? '已发布方案列表' : mode === 'comparisons' ? '相似方案列表' : '标准方案列表';
  $: searchPlaceholder = mode === 'catalog' ? '搜索需求、方案或项目' : mode === 'comparisons' ? '搜索相似方案' : '搜索标准方案';
  $: resultLabel = mode === 'catalog' ? '个已发布方案' : mode === 'comparisons' ? '组相似方案' : '个标准方案';

  onMount(() => {
    const handlePreferences = () => void reloadAll();
    const intent = readLocationIntent();
    if (intent?.mode === 'standards') mode = 'standards';
    window.addEventListener('project-preferences-updated', handlePreferences);
    void reloadAll(intent);
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

  async function reloadAll(intent: LocationIntent | null = null) {
    const preferredEntryID = intent?.mode === 'catalog' ? intent.id : selectedEntryID;
    const preferredStandardID = intent?.mode === 'standards' ? intent.id : selectedStandardID;
    detail = null;
    standardDetail = null;
    await Promise.all([loadProjects(), loadEntries(preferredEntryID), loadStandards(preferredStandardID)]);
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
      const nextID = preferredID || entries[0]?.id || 0;
      if (mode !== 'standards' && nextID) {
        await selectEntry(nextID);
      } else if (mode !== 'standards') {
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
      if (requestID === detailRequestID && selectedEntryID === id) {
        detail = response;
        if (!entries.some((item) => item.id === response.entry.id)) {
          entries = [response.entry, ...entries];
        }
        if (!response.comparisons.some((item) => item.comparison.id === selectedComparisonID)) {
          selectedComparisonID = response.comparisons[0]?.comparison.id || 0;
        }
      }
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
      if (!detail && selectedEntryID) void selectEntry(selectedEntryID);
      else if (!detail && entries[0]) void selectEntry(entries[0].id);
    } else if (next === 'comparisons') {
      if (!detail && selectedEntryID) void selectEntry(selectedEntryID);
      else if (!detail && entries[0]) void selectEntry(entries[0].id);
      else selectedComparisonID = selectedComparison?.comparison.id || 0;
    } else {
      const nextID = filterStandardItems(standards, projectKey, committedSearch)[0]?.standard.id || 0;
      if (nextID) void selectStandard(nextID);
    }
  }

  function handleProjectChange(event: CustomEvent<string>) {
    projectKey = event.detail;
    if (mode === 'catalog') {
      void loadEntries();
    } else if (mode === 'standards') {
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
      } else if (mode === 'standards') {
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
    else if (mode === 'standards') void Promise.all([loadProjects(), loadStandards()]);
    else if (selectedEntryID) void Promise.all([loadProjects(), selectEntry(selectedEntryID)]);
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

  function filterComparisonItems(items: ComparisonView[], selectedProject: string, queryText: string) {
    const query = queryText.trim().toLowerCase();
    return items.filter((item) => {
      if (selectedProject && item.counterpart.project_key !== selectedProject) return false;
      if (!query) return true;
      return `${item.counterpart.solution_title} ${item.counterpart.demand_title} ${item.counterpart.demand_id} ${item.counterpart.project_key} ${comparisonLabel(item)}`
        .toLowerCase()
        .includes(query);
    });
  }

  function readLocationIntent(): LocationIntent | null {
    const params = new URLSearchParams(window.location.search);
    const entryID = Number(params.get('solution_entry'));
    if (Number.isInteger(entryID) && entryID > 0) return { mode: 'catalog', id: entryID };
    const standardID = Number(params.get('solution_standard'));
    if (Number.isInteger(standardID) && standardID > 0) return { mode: 'standards', id: standardID };
    return null;
  }

  function uniqueLink(kind: 'entry' | 'standard', id: number) {
    const params = new URLSearchParams({ tab: 'solutions' });
    if (kind === 'entry') params.set('solution_entry', String(id));
    else params.set('solution_standard', String(id));
    return `${window.location.origin}${window.location.pathname}?${params.toString()}`;
  }

  async function copyUniqueLink(kind: 'entry' | 'standard', id: number) {
    const link = uniqueLink(kind, id);
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(link);
      else fallbackCopy(link);
      showToast('唯一链接已复制。', { title: '复制成功' });
    } catch (error) {
      try {
        fallbackCopy(link);
        showToast('唯一链接已复制。', { title: '复制成功' });
      } catch {
        showToast(errorMessage(error, '链接复制失败'), { type: 'error', title: '复制失败' });
      }
    }
  }

  function fallbackCopy(value: string) {
    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    const copied = document.execCommand('copy');
    textarea.remove();
    if (!copied) throw new Error('浏览器未允许复制');
  }

  function exportMarkdown(markdown: string, markdownFilename: string) {
    try {
      const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' });
      const objectURL = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectURL;
      anchor.download = markdownFilename;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 0);
      showToast('Markdown 文件已导出。', { title: '导出成功' });
    } catch (error) {
      showToast(errorMessage(error, 'Markdown 导出失败'), { type: 'error', title: '导出失败' });
    }
  }

  function markdownFilename(identity: string, title: string) {
    const safeTitle = title.trim().replace(/[\\/:*?"<>|\u0000-\u001f]+/g, '-').replace(/\s+/g, '-').slice(0, 72);
    return `${identity}-${safeTitle || '方案'}.md`;
  }

  async function openCounterpart(entryID: number) {
    mode = 'catalog';
    await selectEntry(entryID);
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
      <button type="button" class:active={mode === 'comparisons'} aria-pressed={mode === 'comparisons'} on:click={() => changeMode('comparisons')}>相似方案</button>
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
        placeholder={searchPlaceholder}
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
      <span>{resultLabel}</span>
    </div>
  </header>

  <div class="center-grid">
    <aside class="register-pane" aria-label={listLabel}>
      {#if loadingList}
        <div class="register-skeleton" aria-label="正在加载方案列表">
          {#each Array(7) as _}
            <div class="skeleton-row"><i></i><span></span></div>
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
      {:else if mode === 'comparisons' && filteredComparisons.length === 0}
        <div class="state-panel compact-state">
          <strong>暂无相似方案</strong>
          <p>{committedSearch || projectKey ? '当前筛选范围内没有匹配内容。' : detail ? '当前方案暂未发现达到召回阈值的相似方案。' : '先在“已发布方案”中选择一条方案。'}</p>
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
              <span class="row-title"><strong>{entry.solution_title || entry.demand_title || entry.demand_id}</strong><time datetime={entry.published_at}>{formatTime(entry.published_at)}</time></span>
              <span class="row-identity"><b>{entry.project_key}</b><code>{entry.demand_id}</code></span>
            </button>
          {/each}
        </div>
      {:else if mode === 'comparisons'}
        <div class="register-list">
          {#each filteredComparisons as comparison (comparison.comparison.id)}
            <button
              type="button"
              class="register-row"
              class:active={comparison.comparison.id === selectedComparison?.comparison.id}
              aria-current={comparison.comparison.id === selectedComparison?.comparison.id ? 'true' : undefined}
              on:click={() => { selectedComparisonID = comparison.comparison.id; resetDetailScroll(); resetReview(); }}
            >
              <span class="row-title"><strong>{comparison.counterpart.solution_title || comparison.counterpart.demand_title || comparison.counterpart.demand_id}</strong><i>{comparisonLabel(comparison)}</i></span>
              <span class="row-identity"><b>{comparison.counterpart.project_key}</b><code>{comparison.counterpart.demand_id}</code><em>{percentage(comparison.comparison.recall_score)}</em></span>
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
              <span class="row-title"><strong>{item.standard.title}</strong><time datetime={item.updated_at}>{formatTime(item.updated_at)}</time></span>
              <span class="row-identity"><b>STD-{item.standard.id}</b><code>{item.projects.join(' · ') || '通用'}</code></span>
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
          <button type="button" on:click={() => mode === 'standards' ? selectStandard(selectedStandardID) : selectEntry(selectedEntryID)}>重新加载</button>
        </div>
      {:else if mode === 'catalog' && detail}
        <div class="document-surface">
          <header class="document-toolbar">
            <div class="document-title"><code>{detail.entry.project_key} · {detail.entry.demand_id}</code><strong>{detail.entry.solution_title || detail.entry.demand_title || detail.entry.demand_id}</strong></div>
            <div class="document-actions">
              <a href={`/?tab=schedule&demand=${encodeURIComponent(detail.entry.demand_id)}&solution=1`}>打开需求</a>
              <button type="button" on:click={() => copyUniqueLink('entry', detail!.entry.id)}>复制链接</button>
              <button type="button" on:click={() => exportMarkdown(detail!.markdown, markdownFilename(detail!.entry.demand_id, detail!.entry.solution_title || detail!.entry.demand_title))}>导出 Markdown</button>
            </div>
          </header>
          <div class="document-preview">
            <MarkdownWorkbench
              value={detail.markdown}
              mode="preview"
              availableModes={['preview']}
              readonly
              label="方案正文"
              description=""
              showToolbar={false}
              minHeight={480}
            />
          </div>
        </div>
      {:else if mode === 'comparisons' && selectedComparison}
        <div class="comparison-detail">
          <header class="comparison-header">
            <div>
              <span>{detail?.entry.demand_id || '当前方案'} → {selectedComparison.counterpart.demand_id}</span>
              <h2>两轮对比与标准化建议</h2>
              <p>{selectedComparison.counterpart.solution_title || selectedComparison.counterpart.demand_title}</p>
            </div>
            <button type="button" class="quiet-button" on:click={() => openCounterpart(selectedComparison!.counterpart.id)}>查看方案</button>
          </header>

          <div class="score-line comparison-scores">
            <span>初筛 {percentage(selectedComparison.comparison.recall_score)}</span>
            {#if selectedComparison.round_one}<span>需求一致 {percentage(selectedComparison.round_one.score)}</span>{/if}
            {#if selectedComparison.round_two}<span>方案兼容 {percentage(selectedComparison.round_two.score)}</span>{/if}
            <b class:pending={comparisonLabel(selectedComparison) === '待审核'}>{comparisonLabel(selectedComparison)}</b>
          </div>

          {#if selectedComparison.round_one}
            <section class="analysis-block">
              <span>第一轮 · 需求一致性</span>
              <h3>{selectedComparison.round_one.equivalent ? '需求目标一致' : '需求目标存在差异'}</h3>
              {#if selectedComparison.round_one.reason}<p>{selectedComparison.round_one.reason}</p>{/if}
              {#if selectedComparison.round_one.shared_intent.length}
                <ul>{#each selectedComparison.round_one.shared_intent as item}<li>{item}</li>{/each}</ul>
              {/if}
              {#if selectedComparison.round_one.differences.length}
                <ul class="difference-list">{#each selectedComparison.round_one.differences as item}<li>{item}</li>{/each}</ul>
              {/if}
            </section>
          {/if}

          {#if selectedComparison.round_two}
            <section class="analysis-block">
              <span>第二轮 · 方案兼容性</span>
              <h3>{selectedComparison.round_two.standardizable ? '具备标准化条件' : '暂不建议标准化'}</h3>
              {#if selectedComparison.round_two.summary}<p>{selectedComparison.round_two.summary}</p>{/if}
              {#if selectedComparison.round_two.common_core.length}
                <ul>{#each selectedComparison.round_two.common_core as item}<li>{item}</li>{/each}</ul>
              {/if}
              {#if selectedComparison.round_two.conflicts.length}
                <ul class="difference-list">{#each selectedComparison.round_two.conflicts as item}<li>{item}</li>{/each}</ul>
              {/if}
            </section>
          {/if}

          {#if selectedComparison.comparison.last_error}<p class="comparison-error">{selectedComparison.comparison.last_error}</p>{/if}

          {#if selectedComparison.proposal}
            <section class="proposal-block">
              <div class="proposal-heading">
                <div><span>标准化提案</span><strong>{selectedComparison.proposal.proposal.title}</strong></div>
                <em class="status-{selectedComparison.proposal.proposal.status}">{comparisonLabel(selectedComparison)}</em>
              </div>
              {#if selectedComparison.proposal.proposal.summary}<p>{selectedComparison.proposal.proposal.summary}</p>{/if}
              <MarkdownWorkbench
                value={selectedComparison.proposal.markdown}
                mode="preview"
                availableModes={['preview']}
                readonly
                label="标准化提案"
                description=""
                showToolbar={false}
                minHeight={220}
                autoHeight
              />
              {#if selectedComparison.proposal.proposal.status === 'pending' && canReview}
                {#if reviewProposalID === selectedComparison.proposal.proposal.id}
                  <div class="review-confirmation">
                    <label for={`proposal-note-${selectedComparison.proposal.proposal.id}`}>审核说明</label>
                    <textarea
                      id={`proposal-note-${selectedComparison.proposal.proposal.id}`}
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
                    <button type="button" class="quiet-button" on:click={() => beginReview(selectedComparison!.proposal!.proposal.id, 'reject')}>驳回</button>
                    <button type="button" class="primary-button" on:click={() => beginReview(selectedComparison!.proposal!.proposal.id, 'accept')}>审核通过</button>
                  </div>
                {/if}
              {:else if selectedComparison.proposal.proposal.status !== 'pending'}
                <div class="review-audit">
                  <span>{selectedComparison.proposal.proposal.reviewed_by || '已审核'}</span>
                  {#if selectedComparison.proposal.proposal.review_note}<p>{selectedComparison.proposal.proposal.review_note}</p>{/if}
                </div>
              {/if}
            </section>
          {/if}
        </div>
      {:else if mode === 'comparisons'}
        <div class="state-panel">
          <strong>选择一组相似方案查看对比</strong>
          <p>相似方案、两轮分析和标准化提案统一在这里查看。</p>
        </div>
      {:else if mode === 'standards' && standardDetail}
        <div class="document-surface">
          <header class="document-toolbar">
            <div class="document-title"><code>STD-{standardDetail.standard.id}</code><strong>{standardDetail.standard.title}</strong></div>
            <div class="document-actions">
              <button type="button" on:click={() => copyUniqueLink('standard', standardDetail!.standard.id)}>复制链接</button>
              <button type="button" on:click={() => exportMarkdown(standardDetail!.markdown, markdownFilename(`STD-${standardDetail!.standard.id}`, standardDetail!.standard.title))}>导出 Markdown</button>
            </div>
          </header>
          <div class="document-preview">
            <MarkdownWorkbench
              value={standardDetail.markdown}
              mode="preview"
              availableModes={['preview']}
              readonly
              label="标准方案正文"
              description=""
              showToolbar={false}
              minHeight={480}
            />
          </div>
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
  /* finesse · register=product · shell=shared-sidebar + split-document-register
   * SOUL=4 SPECTACLE=1 DENSITY=8 */
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
    white-space: nowrap;
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
  .detail-pane { min-width: 0; min-height: 0; }
  .register-pane { overflow-y: auto; }
  .register-pane { border-right: 1px solid var(--wa-border-divider); background: var(--wa-surface-flat); }
  .detail-pane { overflow: hidden; background: var(--wa-surface-panel); }

  .register-list { display: flex; flex-direction: column; }
  .register-row {
    box-sizing: border-box;
    width: 100%;
    min-height: 72px;
    max-height: 72px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 6px;
    padding: 10px 14px;
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
  .row-title { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; }
  .row-title strong { overflow: hidden; color: var(--wa-text-strong); font-size: 13px; line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
  .row-title time, .row-title i { color: var(--wa-text-subtle); font-size: 10px; font-style: normal; white-space: nowrap; }
  .row-identity { min-width: 0; display: flex; align-items: center; gap: 7px; color: var(--wa-text-subtle); font-size: 10px; }
  .row-identity b { color: var(--wa-accent-strong); font-family: var(--wa-font-mono); letter-spacing: .04em; }
  .row-identity code { overflow: hidden; color: var(--wa-text-muted); font-family: var(--wa-font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .row-identity em { margin-left: auto; font-family: var(--wa-font-mono); font-style: normal; }

  .document-surface { height: 100%; min-height: 0; display: grid; grid-template-rows: auto minmax(0, 1fr); overflow: hidden; }
  .document-toolbar { min-height: 56px; display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 9px 12px 9px 18px; border-bottom: 1px solid var(--wa-border-divider); background: var(--wa-surface-flat); }
  .document-title { min-width: 0; display: flex; align-items: baseline; gap: 10px; }
  .document-title code { flex: 0 0 auto; color: var(--wa-accent-strong); font: 700 11px/1.3 var(--wa-font-mono); }
  .document-title strong { overflow: hidden; color: var(--wa-text-strong); font-size: 12px; line-height: 1.4; text-overflow: ellipsis; white-space: nowrap; }
  .document-actions { flex: 0 0 auto; display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 7px; }
  .document-actions button, .document-actions a { min-height: var(--wa-control-h); display: inline-flex; align-items: center; border: 1px solid var(--wa-border-strong); border-radius: var(--wa-radius-sm); padding: 0 11px; background: transparent; color: var(--wa-text-main); font: 720 12px/1 var(--wa-font-sans); text-decoration: none; white-space: nowrap; cursor: pointer; }
  .document-actions button:hover, .document-actions a:hover { border-color: var(--wa-border-focus); background: var(--wa-row-hover); color: var(--wa-accent-strong); }
  .document-preview { min-width: 0; min-height: 0; }
  .document-preview :global(.markdown-workbench) { height: 100%; min-height: 0; border: 0; border-radius: 0; box-shadow: none; }

  .comparison-detail { box-sizing: border-box; height: 100%; overflow-y: auto; padding: 22px 24px 40px; }
  .comparison-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; padding-bottom: 17px; border-bottom: 1px solid var(--wa-border-divider); }
  .comparison-header > div { min-width: 0; }
  .comparison-header span, .analysis-block > span, .proposal-heading span { display: block; color: var(--wa-accent-strong); font-size: 10px; font-weight: 760; letter-spacing: .06em; text-transform: uppercase; }
  .comparison-header h2 { margin: 6px 0 0; color: var(--wa-text-strong); font-size: 20px; line-height: 1.25; }
  .comparison-header p { margin: 7px 0 0; color: var(--wa-text-muted); font-size: 13px; line-height: 1.5; }
  .score-line { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 14px; color: var(--wa-text-muted); font-family: var(--wa-font-mono); font-size: 11px; }
  .comparison-scores { padding: 13px 0; border-bottom: 1px solid var(--wa-border-divider); }
  .comparison-scores b { margin-left: auto; padding: 4px 8px; border-radius: var(--wa-radius-pill); background: var(--wa-neutral-soft); color: var(--wa-text-muted); font-family: var(--wa-font-sans); font-size: 11px; }
  .comparison-scores b.pending { background: var(--wa-warning-soft); color: var(--wa-warning); }
  .analysis-block { padding: 18px 0; border-bottom: 1px solid var(--wa-border-divider); }
  .analysis-block h3 { margin: 5px 0 0; color: var(--wa-text-strong); font-size: 15px; }
  .analysis-block p, .proposal-block > p { margin: 9px 0 0; color: var(--wa-text-muted); font-size: 12px; line-height: 1.65; }
  .analysis-block ul { margin: 11px 0 0; padding-left: 20px; color: var(--wa-text-main); font-size: 12px; line-height: 1.6; }
  .analysis-block li + li { margin-top: 4px; }
  .analysis-block li::marker { color: var(--wa-accent); }
  .analysis-block .difference-list li::marker { color: var(--wa-warning); }
  .comparison-error { color: var(--wa-danger) !important; }

  .proposal-block { margin-top: 20px; padding: 16px; border: 1px solid var(--wa-border-divider); border-radius: var(--wa-radius-md); background: var(--wa-surface-inset); box-shadow: inset 0 2px var(--wa-accent-soft); }
  .proposal-heading { display: flex; justify-content: space-between; gap: 12px; }
  .proposal-heading strong { display: block; margin-top: 4px; color: var(--wa-text-strong); font-size: 13px; }
  .proposal-heading em { align-self: flex-start; color: var(--wa-text-muted); font-size: 11px; font-style: normal; }
  .proposal-heading .status-accepted { color: var(--wa-success); }
  .proposal-heading .status-rejected { color: var(--wa-danger); }
  .proposal-block :global(.markdown-workbench) { margin-top: 13px; border-color: var(--wa-border-divider); background: var(--wa-surface-flat); box-shadow: none; }
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
  .skeleton-row { box-sizing: border-box; height: 72px; display: flex; flex-direction: column; justify-content: center; gap: 9px; padding: 10px 14px; border-bottom: 1px solid var(--wa-border-divider); }
  .skeleton-row i, .skeleton-row span, .detail-skeleton > * { display: block; border-radius: 5px; background: var(--wa-neutral-soft); }
  .skeleton-row i { width: 74%; height: 12px; }
  .skeleton-row span { width: 42%; height: 9px; }
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
    .document-toolbar { align-items: flex-start; flex-direction: column; }
    .document-actions { justify-content: flex-start; }
  }

  @media (max-width: 820px) {
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
    .document-surface { height: auto; min-height: 70dvh; }
    .document-toolbar { padding: 11px 12px; }
    .document-title { width: 100%; }
    .document-actions { width: 100%; }
    .document-actions button, .document-actions a { min-height: var(--wa-touch-h); }
    .document-preview { height: 70dvh; }
    .comparison-detail { height: auto; overflow: visible; padding: 18px 15px 32px; }
    .proposal-actions button, .review-actions button { min-height: var(--wa-touch-h); }
  }

  @media (max-width: 420px) {
    .document-title { align-items: flex-start; flex-direction: column; gap: 4px; }
    .document-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .document-actions button, .document-actions a { justify-content: center; text-align: center; }
    .document-actions > :only-child, .document-actions > :nth-last-child(3):first-child { grid-column: 1 / -1; }
    .comparison-header, .proposal-heading { flex-direction: column; }
    .comparison-scores b { width: max-content; margin-left: 0; }
    .proposal-actions, .review-actions { flex-direction: column-reverse; }
    .proposal-actions button, .review-actions button { width: 100%; }
  }

  @media (prefers-reduced-motion: reduce) {
    .register-skeleton, .detail-skeleton { animation: none; }
  }
</style>
