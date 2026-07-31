<script lang="ts">
  import DemandKanban from './DemandKanban.svelte';
  import Alert from './shared/Alert.svelte';
  import Button from './shared/Button.svelte';
  import DatePicker from './shared/DatePicker.svelte';
  import Modal from './shared/Modal.svelte';
  import MultiSelect from './shared/MultiSelect.svelte';
  import Select from './shared/Select.svelte';
  import { showToast } from '../lib/toast';
  import { fetchDeliveryDirectory } from '../lib/delivery-directory';

  export let currentUserPermissions: string[] = [];
  export let currentUserName = '';
  export let currentUserEmail = '';
  export let currentUserDepartment = '';
  export let activeDemandView: 'board' | 'schedule' | 'releases' | 'projects' = 'schedule';

  interface ReleaseVersion {
    id: number;
    project_key: string;
    source: string;
    external_id: string;
    name: string;
    description: string;
    status: 'planned' | 'released' | 'archived';
    start_date: string | null;
    release_date: string | null;
    source_url: string;
    updated_at: string;
  }

  interface ReleasePlanItem {
    release: ReleaseVersion;
    jira_issue_count: number;
  }

  interface JiraIssue {
    work_item_id: string;
    jira_key: string;
    jira_url: string;
    title: string;
    issue_type: string;
    status: string;
    assignee: string;
    project_key: string;
    linked: boolean;
    current_release_id?: number;
    current_release_name?: string;
  }

  interface ProjectConfig {
    project_key: string;
    project_name: string;
  }

  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'planned', label: '计划中' },
    { value: 'released', label: '已发布' },
    { value: 'archived', label: '已归档' }
  ];
  const releaseStatusOptions = statusOptions.filter((option) => option.value);

  let items: ReleasePlanItem[] = [];
  let projects: ProjectConfig[] = [];
  let selectedID = 0;
  let projectFilter = '';
  let statusFilter = '';
  let search = '';
  let loading = false;
  let loadedOnce = false;
  let loadError = '';

  let draftProject = '';
  let savingProject = false;
  let jiraSearch = '';
  let linkedJiraIssues: JiraIssue[] = [];
  let jiraCandidates: JiraIssue[] = [];
  let selectedJiraIssueIDs: string[] = [];
  let jiraIssuesLoading = false;
  let jiraSearching = false;
  let jiraSearchError = '';
  let linkingJiraIssues = false;
  let removingJiraIssueID = '';
  let showCreateReleaseModal = false;
  let createName = '';
  let createDescription = '';
  let createProject = '';
  let createStatus = 'planned';
  let createStartDate = '';
  let createReleaseDate = '';
  let createError = '';
  let creatingRelease = false;

  $: canManageReleases = currentUserPermissions.includes('release:manage');
  $: selectedItem = items.find((item) => item.release.id === selectedID) || null;
  $: normalizedSearch = search.trim().toLowerCase();
  $: visibleItems = items.filter((item) => {
    const release = item.release;
    const projectMatches = !projectFilter || release.project_key === projectFilter;
    const statusMatches = !statusFilter || release.status === statusFilter;
    const searchMatches = !normalizedSearch || [
      release.name,
      release.description,
      release.project_key,
      release.external_id
    ].filter(Boolean).join(' ').toLowerCase().includes(normalizedSearch);
    return projectMatches && statusMatches && searchMatches;
  });
  $: metrics = {
    total: visibleItems.length,
    unbound: visibleItems.filter((item) => !item.release.project_key).length,
    unlinked: visibleItems.filter((item) => !item.jira_issue_count).length,
    planned: visibleItems.filter((item) => item.release.status === 'planned').length
  };
  $: projectOptions = [
    { value: '', label: '全部项目' },
    ...projects.map((project) => ({
      value: project.project_key,
      label: project.project_name || project.project_key
    }))
  ];
  $: bindingProjectOptions = projects.map((project) => ({
    value: project.project_key,
    label: project.project_name || project.project_key
  }));
  $: jiraCandidateOptions = jiraCandidates.map((issue) => ({
    value: issue.work_item_id,
    label: `${issue.jira_key} · ${issue.title || '未命名事项'}`,
    meta: issue.current_release_id
      ? `已属于版本 ${issue.current_release_name || `#${issue.current_release_id}`}`
      : `${issue.issue_type || '事项'} · ${issue.status || '未知状态'}${issue.assignee ? ` · ${issue.assignee}` : ''}`,
    disabled: !!issue.current_release_id
  }));

  function authHeaders(json = false): Record<string, string> {
    const token = localStorage.getItem('jwt_token') || '';
    return {
      ...(json ? { 'Content-Type': 'application/json' } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    };
  }

  function normalizeProject(project: ProjectConfig): ProjectConfig {
    return {
      ...project,
      project_key: (project.project_key || '').trim().toUpperCase()
    };
  }

  function statusLabel(status: string): string {
    if (status === 'released') return '已发布';
    if (status === 'archived') return '已归档';
    return '计划中';
  }

  function formatDate(value: string | null): string {
    if (!value) return '未设置';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value.slice(0, 10);
    return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' });
  }

  function formatWindow(release: ReleaseVersion): string {
    if (!release.start_date && !release.release_date) return '未设置';
    return `${formatDate(release.start_date)} — ${formatDate(release.release_date)}`;
  }

  function projectLabel(projectKey: string): string {
    if (!projectKey) return '未绑定项目';
    return projects.find((project) => project.project_key === projectKey)?.project_name || projectKey;
  }

  function syncDraft(item: ReleasePlanItem | null) {
    draftProject = item?.release.project_key || '';
    jiraSearch = '';
    linkedJiraIssues = [];
    jiraCandidates = [];
    selectedJiraIssueIDs = [];
    jiraSearchError = '';
  }

  function selectRelease(id: number) {
    selectedID = id;
    syncDraft(items.find((item) => item.release.id === id) || null);
    void loadSelectedReleaseJiraIssues();
  }

  function resetCreateRelease() {
    createName = '';
    createDescription = '';
    createProject = '';
    createStatus = 'planned';
    createStartDate = '';
    createReleaseDate = '';
    createError = '';
  }

  function openCreateRelease() {
    if (!canManageReleases) return;
    resetCreateRelease();
    showCreateReleaseModal = true;
  }

  function closeCreateRelease() {
    if (creatingRelease) return;
    showCreateReleaseModal = false;
    createError = '';
  }

  function createErrorMessage(payload: any): string {
    if (payload?.code === 'release_name_required') return '请填写版本名称。';
    if (payload?.code === 'unknown_project') return '所选项目不在项目目录中，请刷新后重试。';
    if (payload?.code === 'invalid_start_date' || payload?.code === 'invalid_release_date') {
      return '版本日期格式无效，请重新选择。';
    }
    return payload?.message || '版本创建失败';
  }

  function reportCreateError(message: string) {
    createError = message;
    showToast(message, {
      type: 'error',
      title: '无法创建版本'
    });
  }

  async function createRelease() {
    const name = createName.trim();
    if (!name) {
      reportCreateError('请填写版本名称。');
      return;
    }
    if (createStartDate && createReleaseDate && createStartDate > createReleaseDate) {
      reportCreateError('计划开始日期不能晚于发布日期。');
      return;
    }
    if (creatingRelease) return;
    creatingRelease = true;
    createError = '';
    try {
      const response = await fetch('/api/releases', {
        method: 'POST',
        headers: authHeaders(true),
        body: JSON.stringify({
          name,
          description: createDescription.trim(),
          status: createStatus,
          ...(createProject ? { project_key: createProject } : {}),
          ...(createStartDate ? { start_date: createStartDate } : {}),
          ...(createReleaseDate ? { release_date: createReleaseDate } : {})
        })
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(createErrorMessage(payload));
      selectedID = Number(payload?.id) || 0;
      showCreateReleaseModal = false;
      showToast(`版本“${name}”已创建并选中，可继续批量关联 Jira 事项。`, {
        title: '版本已创建'
      });
      await loadVersions(true);
    } catch (error: any) {
      reportCreateError(error?.message || '版本创建失败');
    } finally {
      creatingRelease = false;
    }
  }

  async function loadVersions(preserveSelection = true) {
    loading = true;
    loadError = '';
    try {
      const [releaseResponse, directory] = await Promise.all([
        fetch('/api/releases', { headers: authHeaders() }),
        fetchDeliveryDirectory()
      ]);
      const releasePayload = await releaseResponse.json().catch(() => ({}));
      if (!releaseResponse.ok) {
        throw new Error(releasePayload?.message || '版本事实加载失败');
      }
      const nextItems = Array.isArray(releasePayload.items) ? releasePayload.items : [];
      items = nextItems;
      projects = directory.projects.map((project) => normalizeProject(project));

      const nextSelectedID = preserveSelection && nextItems.some((item: ReleasePlanItem) => item.release.id === selectedID)
        ? selectedID
        : nextItems[0]?.release.id || 0;
      selectedID = nextSelectedID;
      syncDraft(nextItems.find((item: ReleasePlanItem) => item.release.id === nextSelectedID) || null);
      await loadSelectedReleaseJiraIssues();
    } catch (error: any) {
      loadError = error?.message || '版本事实加载失败';
    } finally {
      loading = false;
      loadedOnce = true;
    }
  }

  async function saveProjectBinding() {
    if (!selectedItem || !canManageReleases || savingProject) return;
    savingProject = true;
    try {
      const response = await fetch(`/api/releases/${selectedItem.release.id}`, {
        method: 'PATCH',
        headers: authHeaders(true),
        body: JSON.stringify({ project_key: draftProject })
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload?.message || '项目绑定失败');
      showToast(draftProject ? '版本已绑定到所选项目。' : '版本项目绑定已清除。', {
        title: '版本项目已更新'
      });
      await loadVersions(true);
    } catch (error: any) {
      showToast(error?.message || '项目绑定失败', {
        type: 'error',
        title: '无法更新版本项目'
      });
    } finally {
      savingProject = false;
    }
  }

  async function loadSelectedReleaseJiraIssues(searchTerm = '') {
    const release = items.find((item) => item.release.id === selectedID)?.release;
    if (!release?.id || !release.project_key) {
      linkedJiraIssues = [];
      jiraCandidates = [];
      selectedJiraIssueIDs = [];
      return;
    }
    const releaseID = release.id;
    jiraIssuesLoading = true;
    jiraSearchError = '';
    try {
      const candidateParams = new URLSearchParams({ scope: 'candidates', limit: '100' });
      if (searchTerm.trim()) candidateParams.set('q', searchTerm.trim());
      const [linkedResponse, candidateResponse] = await Promise.all([
        fetch(`/api/releases/${releaseID}/jira-issues?scope=linked&limit=200`, { headers: authHeaders() }),
        fetch(`/api/releases/${releaseID}/jira-issues?${candidateParams.toString()}`, { headers: authHeaders() })
      ]);
      const linkedPayload = await linkedResponse.json().catch(() => ({}));
      const candidatePayload = await candidateResponse.json().catch(() => ({}));
      if (!linkedResponse.ok) throw new Error(linkedPayload?.message || '已关联 Jira 事项加载失败');
      if (!candidateResponse.ok) throw new Error(candidatePayload?.message || 'Jira 事项候选加载失败');
      if (selectedID !== releaseID) return;
      linkedJiraIssues = Array.isArray(linkedPayload.items) ? linkedPayload.items : [];
      jiraCandidates = Array.isArray(candidatePayload.items) ? candidatePayload.items : [];
      selectedJiraIssueIDs = selectedJiraIssueIDs.filter((id) =>
        jiraCandidates.some((candidate) => candidate.work_item_id === id && !candidate.current_release_id)
      );
    } catch (error: any) {
      if (selectedID !== releaseID) return;
      jiraSearchError = error?.message || 'Jira 事项加载失败';
    } finally {
      if (selectedID === releaseID) jiraIssuesLoading = false;
    }
  }

  async function searchJiraIssues() {
    if (!selectedItem?.release.project_key || jiraSearching) return;
    jiraSearching = true;
    await loadSelectedReleaseJiraIssues(jiraSearch);
    jiraSearching = false;
  }

  async function linkSelectedJiraIssues() {
    if (!selectedItem || !canManageReleases || linkingJiraIssues || selectedJiraIssueIDs.length === 0) return;
    linkingJiraIssues = true;
    const count = selectedJiraIssueIDs.length;
    try {
      const response = await fetch(`/api/releases/${selectedItem.release.id}/jira-issues/bulk`, {
        method: 'POST',
        headers: authHeaders(true),
        body: JSON.stringify({
          work_item_ids: selectedJiraIssueIDs,
          reason: `批量纳入本地版本 ${selectedItem.release.name}`
        })
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload?.message || 'Jira 事项关联失败');
      selectedJiraIssueIDs = [];
      showToast(`已将 ${count} 条 Jira 事项关联到版本“${selectedItem.release.name}”。`, {
        title: 'Jira 事项已关联'
      });
      await loadVersions(true);
    } catch (error: any) {
      showToast(error?.message || 'Jira 事项关联失败', {
        type: 'error',
        title: '无法关联 Jira 事项'
      });
    } finally {
      linkingJiraIssues = false;
    }
  }

  async function removeJiraIssue(issue: JiraIssue) {
    if (!selectedItem || !canManageReleases || removingJiraIssueID) return;
    removingJiraIssueID = issue.work_item_id;
    try {
      const response = await fetch(
        `/api/releases/${selectedItem.release.id}/jira-issues/${encodeURIComponent(issue.work_item_id)}`,
        {
          method: 'DELETE',
          headers: authHeaders()
        }
      );
      const payload = response.status === 204 ? {} : await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload?.message || '移除 Jira 事项失败');
      showToast(`已从当前版本移除 ${issue.jira_key}。`, { title: 'Jira 事项已移除' });
      await loadVersions(true);
    } catch (error: any) {
      showToast(error?.message || '移除 Jira 事项失败', {
        type: 'error',
        title: '无法移除 Jira 事项'
      });
    } finally {
      removingJiraIssueID = '';
    }
  }

  function handleJiraSearchKeydown(event: KeyboardEvent) {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    void searchJiraIssues();
  }

  $: if (activeDemandView === 'releases' && !loadedOnce && !loading) {
    void loadVersions(false);
  }
</script>

{#if activeDemandView !== 'releases'}
  <DemandKanban
    {currentUserPermissions}
    {currentUserName}
    {currentUserEmail}
    {currentUserDepartment}
    activeDemandView={activeDemandView === 'board' ? 'board' : 'schedule'}
  />
{:else}
  <section class="release-plan" aria-label="版本计划">
    <header class="plan-toolbar">
      <div class="toolbar-heading">
        <span>Release catalog</span>
        <strong>现有版本与 Jira 事项</strong>
      </div>
      <div class="toolbar-filters">
        <label class="search-field">
          <span class="sr-only">搜索版本</span>
          <input bind:value={search} type="search" placeholder="搜索版本或项目" aria-label="搜索版本" />
        </label>
        <Select
          value={projectFilter}
          options={projectOptions}
          compact={true}
          clearable={true}
          ariaLabel="筛选项目"
          on:change={(event) => projectFilter = event.detail}
        />
        <Select
          value={statusFilter}
          options={statusOptions}
          compact={true}
          searchable={false}
          ariaLabel="筛选版本状态"
          on:change={(event) => statusFilter = event.detail}
        />
        {#if canManageReleases}
          <Button variant="primary" size="small" on:click={openCreateRelease}>创建版本</Button>
        {/if}
        <Button variant="secondary" size="small" on:click={() => loadVersions(true)} disabled={loading}>刷新</Button>
      </div>
    </header>

    {#if loadError}
      <div class="plan-feedback"><Alert type="error" message={loadError} /></div>
    {/if}

    <div class="plan-metrics" aria-label="版本计划摘要">
      <div><span>现有版本</span><strong>{metrics.total}</strong></div>
      <div class:attention={metrics.unbound > 0}><span>未绑定项目</span><strong>{metrics.unbound}</strong></div>
      <div class:attention={metrics.unlinked > 0}><span>未关联 Jira 事项</span><strong>{metrics.unlinked}</strong></div>
      <div><span>计划中</span><strong>{metrics.planned}</strong></div>
    </div>

    <div class="plan-workbench">
      <section class="release-list" aria-label="现有版本列表" aria-busy={loading}>
        <header>
          <div>
            <strong>版本事实</strong>
            <span>{visibleItems.length} 个版本</span>
          </div>
          <span>版本关联 Jira 事项，不创建或修改 Jira 版本</span>
        </header>
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>版本</th>
                <th>绑定项目</th>
                <th>Jira 事项</th>
                <th>版本窗口</th>
                <th>状态</th>
                <th>来源</th>
              </tr>
            </thead>
            <tbody>
              {#if loading}
                <tr><td colspan="6" class="empty-row">正在加载现有版本…</td></tr>
              {:else if visibleItems.length === 0}
                <tr>
                  <td colspan="6" class="empty-row">
                    {items.length === 0
                      ? canManageReleases
                        ? '当前还没有版本，请点击“创建版本”录入第一条版本事实。'
                        : '当前还没有可查看的版本。'
                      : '当前筛选下没有匹配版本。'}
                  </td>
                </tr>
              {:else}
                {#each visibleItems as item (item.release.id)}
                  <tr class:selected={selectedID === item.release.id} aria-selected={selectedID === item.release.id}>
                    <td>
                      <button type="button" class="version-link" on:click={() => selectRelease(item.release.id)}>
                        <strong>{item.release.name}</strong>
                        <span>#{item.release.id} · {item.release.external_id}</span>
                      </button>
                    </td>
                    <td>
                      <span class="project-pill" class:missing={!item.release.project_key}>{projectLabel(item.release.project_key)}</span>
                    </td>
                    <td>
                      {#if item.jira_issue_count}
                        <span>{item.jira_issue_count} 条</span>
                      {:else}
                        <span class="missing">未关联</span>
                      {/if}
                    </td>
                    <td>{formatWindow(item.release)}</td>
                    <td><span class="status status-{item.release.status}">{statusLabel(item.release.status)}</span></td>
                    <td>{item.release.source === 'jira' ? 'Jira 导入' : '本地版本'}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </section>

      <aside class="release-inspector" aria-label="版本关联检查器">
        {#if selectedItem}
          <header>
            <div>
              <span>RELEASE #{selectedItem.release.id}</span>
              <strong>{selectedItem.release.name}</strong>
              <small>{statusLabel(selectedItem.release.status)} · {formatWindow(selectedItem.release)}</small>
            </div>
            <span class="status status-{selectedItem.release.status}">{statusLabel(selectedItem.release.status)}</span>
          </header>

          <div class="inspector-body">
            {#if !canManageReleases}
              <Alert type="info" message="当前账号只有查看权限，项目与 Jira 事项关联不可编辑。" />
            {/if}

            <section class="inspector-section" aria-labelledby="project-binding-title">
              <div class="section-heading">
                <div>
                  <h3 id="project-binding-title">绑定项目</h3>
                  <p>版本可暂时未绑定；形成发布承诺前应明确所属项目。</p>
                </div>
                <span class="project-pill" class:missing={!selectedItem.release.project_key}>
                  {projectLabel(selectedItem.release.project_key)}
                </span>
              </div>
              <Select
                id="release-project"
                label="所属项目"
                value={draftProject}
                options={bindingProjectOptions}
                clearable={true}
                disabled={!canManageReleases}
                placeholder="暂不绑定项目"
                searchPlaceholder="搜索项目"
                on:change={(event) => draftProject = event.detail}
              />
              <div class="section-actions">
                <Button
                  variant="secondary"
                  size="small"
                  loading={savingProject}
                  disabled={!canManageReleases || draftProject === selectedItem.release.project_key}
                  on:click={saveProjectBinding}
                >保存项目绑定</Button>
              </div>
            </section>

            <section class="inspector-section" aria-labelledby="jira-binding-title">
              <div class="section-heading">
                <div>
                  <h3 id="jira-binding-title">关联 Jira 事项</h3>
                  <p>
                    {#if selectedItem.release.project_key}
                      候选范围自动继承所属项目 {selectedItem.release.project_key}，无需重复选择 Jira 项目。
                    {:else}
                      请先绑定所属项目，再从同项目的 Jira 事项中批量选择。
                    {/if}
                  </p>
                </div>
                {#if linkedJiraIssues.length}
                  <span class="linked-state">{linkedJiraIssues.length} 条</span>
                {/if}
              </div>

              {#if selectedItem.release.project_key}
                <label class="jira-search-field">
                  <span>搜索 Jira 事项</span>
                  <div>
                    <input
                      bind:value={jiraSearch}
                      type="search"
                      disabled={!canManageReleases}
                      placeholder="输入 Jira 编号或标题"
                      on:keydown={handleJiraSearchKeydown}
                    />
                    <Button
                      variant="secondary"
                      size="small"
                      loading={jiraSearching}
                      disabled={!canManageReleases}
                      on:click={searchJiraIssues}
                    >搜索</Button>
                  </div>
                </label>

                <MultiSelect
                  id="release-jira-issues"
                  label="批量选择"
                  values={selectedJiraIssueIDs}
                  options={jiraCandidateOptions}
                  disabled={!canManageReleases || jiraIssuesLoading}
                  placeholder={jiraIssuesLoading ? '正在加载 Jira 事项…' : '选择要纳入当前版本的 Jira 事项'}
                  searchPlaceholder="在当前候选中筛选"
                  emptyText={jiraSearch ? '没有匹配的 Jira 事项' : '当前项目没有可关联的 Jira 事项'}
                  helperText="已属于其他版本的事项会保留展示但不可重复选择。"
                  showClear={true}
                  summaryMode={true}
                  overlay={true}
                  on:change={(event) => selectedJiraIssueIDs = event.detail}
                />
                <div class="section-actions">
                  <Button
                    variant="primary"
                    size="small"
                    loading={linkingJiraIssues}
                    disabled={!canManageReleases || selectedJiraIssueIDs.length === 0}
                    on:click={linkSelectedJiraIssues}
                  >批量关联 {selectedJiraIssueIDs.length ? `(${selectedJiraIssueIDs.length})` : ''}</Button>
                </div>
              {/if}

              {#if jiraSearchError}
                <Alert type="error" message={jiraSearchError} />
              {/if}

              <div class="linked-issues-heading">
                <strong>已关联事项</strong>
                <span>{linkedJiraIssues.length} 条</span>
              </div>
              {#if jiraIssuesLoading && linkedJiraIssues.length === 0}
                <div class="jira-empty">正在加载已关联 Jira 事项…</div>
              {:else if linkedJiraIssues.length > 0}
                <div class="jira-results" aria-label="已关联 Jira 事项">
                  {#each linkedJiraIssues as issue (issue.work_item_id)}
                    <div class="jira-result">
                      <div>
                        <span>{issue.jira_key} · {issue.status || '未知状态'}</span>
                        <strong title={issue.title}>{issue.title || '未命名事项'}</strong>
                        <small>{issue.issue_type || '事项'}{issue.assignee ? ` · ${issue.assignee}` : ''}</small>
                      </div>
                      <div class="jira-result-actions">
                        {#if issue.jira_url}
                          <a href={issue.jira_url} target="_blank" rel="noopener noreferrer">打开 ↗</a>
                        {/if}
                        {#if canManageReleases}
                          <button
                            type="button"
                            disabled={!!removingJiraIssueID}
                            on:click={() => removeJiraIssue(issue)}
                          >{removingJiraIssueID === issue.work_item_id ? '移除中…' : '移除'}</button>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              {:else}
                <div class="jira-empty">当前版本尚未关联 Jira 事项。</div>
              {/if}
            </section>
          </div>
        {:else}
          <div class="inspector-empty">
            <strong>选择一个现有版本</strong>
            <span>在这里绑定项目并批量关联 Jira 事项。</span>
          </div>
        {/if}
      </aside>
    </div>
  </section>

  <Modal
    show={showCreateReleaseModal}
    title="创建版本"
    closeLabel="关闭创建版本弹窗"
    on:close={closeCreateRelease}
  >
    <form class="create-release-form" on:submit|preventDefault={createRelease}>
      <label class="create-release-field">
        <span>版本名称 <em>*</em></span>
        <input
          bind:value={createName}
          type="text"
          maxlength="255"
          placeholder="例如：FMS 5.4.0"
          aria-required="true"
          aria-invalid={!!createError && !createName.trim()}
        />
      </label>

      <label class="create-release-field">
        <span>版本说明</span>
        <textarea
          bind:value={createDescription}
          rows="3"
          placeholder="说明本次版本的目标或范围（选填）"
        ></textarea>
      </label>

      <div class="create-release-grid">
        <Select
          id="create-release-project"
          label="所属项目"
          value={createProject}
          options={bindingProjectOptions}
          clearable={true}
          compact={true}
          placeholder="暂不绑定项目"
          searchPlaceholder="搜索项目"
          on:change={(event) => createProject = event.detail}
        />
        <Select
          id="create-release-status"
          label="版本状态"
          value={createStatus}
          options={releaseStatusOptions}
          searchable={false}
          compact={true}
          on:change={(event) => createStatus = event.detail}
        />
      </div>

      <div class="create-release-grid">
        <DatePicker
          id="create-release-start-date"
          label="计划开始日期"
          value={createStartDate}
          max={createReleaseDate}
          compact={true}
          placeholder="选择开始日期"
          on:change={(event) => createStartDate = event.detail}
        />
        <DatePicker
          id="create-release-release-date"
          label="发布日期"
          value={createReleaseDate}
          min={createStartDate}
          compact={true}
          placeholder="选择发布日期"
          on:change={(event) => createReleaseDate = event.detail}
        />
      </div>

      <p class="create-release-note">创建后会生成独立的本地版本事实；项目可稍后绑定，Jira 事项可在右侧面板中批量关联。</p>

      {#if createError}
        <Alert type="error" message={createError} />
      {/if}
    </form>

    <div slot="footer" class="create-release-actions">
      <Button variant="secondary" on:click={closeCreateRelease} disabled={creatingRelease}>取消</Button>
      <Button
        variant="primary"
        loading={creatingRelease}
        disabled={!createName.trim()}
        on:click={createRelease}
      >创建版本</Button>
    </div>
  </Modal>
{/if}

<style>
  .release-plan {
    width: 100%;
    height: 100%;
    min-height: 0;
    display: grid;
    grid-template-rows: auto auto auto minmax(0, 1fr);
    gap: 10px;
    color: var(--wa-text-main, #293847);
  }

  .plan-toolbar,
  .toolbar-filters,
  .release-list > header,
  .release-inspector > header,
  .section-heading,
  .section-actions,
  .linked-issues-heading,
  .jira-result {
    display: flex;
    align-items: center;
  }

  .plan-toolbar {
    min-height: 52px;
    justify-content: space-between;
    gap: 16px;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    padding: 4px 2px 10px;
  }

  .toolbar-heading {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .toolbar-heading span,
  .release-inspector header span {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    font-weight: 780;
    letter-spacing: 0.09em;
    text-transform: uppercase;
  }

  .toolbar-heading strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 15px;
  }

  .toolbar-filters {
    min-width: min(100%, 760px);
    justify-content: flex-end;
    gap: 8px;
  }

  .search-field {
    min-width: 230px;
    flex: 1 1 280px;
  }

  .toolbar-filters :global(.select-group) {
    width: 168px;
  }

  .toolbar-filters input,
  .jira-search-field input {
    box-sizing: border-box;
    width: 100%;
    min-height: 34px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.2));
    border-radius: var(--wa-radius-md, 7px);
    background: rgba(255, 255, 255, 0.82);
    color: var(--wa-text-main, #293847);
    padding: 0 10px;
    font: inherit;
    font-size: 11px;
    outline: none;
  }

  .toolbar-filters input:focus,
  .jira-search-field input:focus {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .plan-feedback {
    min-height: 0;
  }

  .plan-metrics {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-surface-flat, #fbfdfe);
  }

  .plan-metrics > div {
    min-width: 0;
    min-height: 54px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 14px;
  }

  .plan-metrics > div + div {
    border-left: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.14));
  }

  .plan-metrics span {
    color: var(--wa-text-muted, #667789);
    font-size: 10px;
  }

  .plan-metrics strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 20px;
    font-variant-numeric: tabular-nums;
  }

  .plan-metrics .attention strong {
    color: var(--wa-danger, #c9473c);
  }

  .plan-workbench {
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1.6fr) minmax(340px, 0.72fr);
    gap: 12px;
  }

  .release-list,
  .release-inspector {
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-surface-flat, #fbfdfe);
  }

  .release-list {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
  }

  .release-list > header {
    min-height: 52px;
    justify-content: space-between;
    gap: 16px;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    padding: 8px 12px;
  }

  .release-list > header > div {
    display: grid;
    gap: 2px;
  }

  .release-list > header strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
  }

  .release-list > header span {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
  }

  .table-scroll {
    min-height: 0;
    overflow: auto;
    scrollbar-gutter: stable;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
  }

  th,
  td {
    overflow: hidden;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.12));
    padding: 9px 10px;
    color: var(--wa-text-main, #293847);
    font-size: 10px;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  th {
    position: sticky;
    top: 0;
    z-index: 2;
    height: 30px;
    background: var(--wa-surface-inset, #f5f8fb);
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    font-weight: 760;
  }

  th:first-child { width: 24%; }
  th:nth-child(2) { width: 16%; }
  th:nth-child(3) { width: 20%; }
  th:nth-child(4) { width: 22%; }
  th:nth-child(5) { width: 9%; }
  th:nth-child(6) { width: 9%; }

  tbody tr:hover,
  tbody tr.selected {
    background: rgba(0, 143, 150, 0.055);
  }

  tbody tr.selected {
    box-shadow: inset 3px 0 0 var(--wa-accent, #008f96);
  }

  .version-link {
    width: 100%;
    display: grid;
    gap: 2px;
    overflow: hidden;
    border: 0;
    padding: 0;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .version-link strong {
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .version-link span {
    color: var(--wa-text-muted, #667789);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 9px;
  }

  .version-link:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.22);
    outline-offset: 2px;
  }

  .missing {
    color: var(--wa-danger, #c9473c);
  }

  .project-pill {
    display: inline-flex;
    max-width: 180px;
    min-height: 22px;
    align-items: center;
    border: 1px solid rgba(0, 143, 150, 0.18);
    border-radius: 999px;
    padding: 2px 9px;
    background: rgba(0, 143, 150, 0.08);
    color: var(--wa-accent-strong, #006f76);
    font-size: 9px;
    font-weight: 760;
    line-height: 1.2;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .project-pill.missing {
    border-color: rgba(201, 71, 60, 0.18);
    background: rgba(201, 71, 60, 0.07);
  }

  .status {
    display: inline-flex;
    border-radius: 6px;
    padding: 3px 6px;
    background: rgba(102, 119, 137, 0.08);
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    font-weight: 760;
  }

  .status-planned {
    background: rgba(0, 143, 150, 0.09);
    color: var(--wa-accent-strong, #006f76);
  }

  .status-released {
    background: rgba(61, 145, 105, 0.1);
    color: #2f7854;
  }

  .empty-row {
    height: 160px;
    color: var(--wa-text-muted, #667789);
    text-align: center;
    white-space: normal;
  }

  .release-inspector {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
  }

  .release-inspector > header {
    min-height: 70px;
    justify-content: space-between;
    gap: 12px;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    padding: 11px 13px;
  }

  .release-inspector > header > div {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .release-inspector header strong {
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .release-inspector header small {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
  }

  .inspector-body {
    min-height: 0;
    display: grid;
    align-content: start;
    gap: 12px;
    overflow-y: auto;
    padding: 12px 13px;
    scrollbar-gutter: stable;
  }

  .inspector-section {
    display: grid;
    gap: 11px;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    padding-bottom: 14px;
  }

  .inspector-section:last-child {
    border-bottom: 0;
  }

  .section-heading {
    justify-content: space-between;
    gap: 10px;
  }

  .section-heading > div {
    display: grid;
    gap: 3px;
  }

  .section-heading h3 {
    margin: 0;
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
  }

  .section-heading p {
    margin: 0;
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    line-height: 1.45;
  }

  .section-actions {
    justify-content: flex-end;
  }

  .linked-state {
    border-radius: 999px;
    background: rgba(61, 145, 105, 0.1);
    color: #2f7854;
    padding: 4px 7px;
    font-size: 9px;
    font-weight: 760;
  }

  .jira-result {
    justify-content: space-between;
    gap: 12px;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    border-radius: var(--wa-radius-md, 7px);
    background: var(--wa-surface-inset, #f5f8fb);
    padding: 9px 10px;
  }

  .jira-result > div {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .jira-result span,
  .jira-result small {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
  }

  .jira-result strong {
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .jira-search-field {
    display: grid;
    gap: 6px;
  }

  .jira-search-field > span {
    color: var(--wa-text-main, #293847);
    font-size: 11px;
    font-weight: 720;
  }

  .jira-search-field > div {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 7px;
  }

  .jira-results {
    display: grid;
    gap: 6px;
  }

  .linked-issues-heading {
    justify-content: space-between;
    gap: 8px;
    padding-top: 2px;
  }

  .linked-issues-heading strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 11px;
  }

  .linked-issues-heading span {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    font-variant-numeric: tabular-nums;
  }

  .jira-result-actions {
    flex: none;
    display: flex !important;
    align-items: center;
    justify-content: flex-end;
    gap: 7px !important;
  }

  .jira-result-actions a,
  .jira-result-actions button {
    border: 0;
    padding: 0;
    background: transparent;
    font: inherit;
    font-size: 9px;
    font-weight: 740;
    text-decoration: none;
    cursor: pointer;
  }

  .jira-result-actions a {
    color: var(--wa-accent-strong, #006f76);
  }

  .jira-result-actions button {
    color: var(--wa-danger, #c9473c);
  }

  .jira-result-actions button:disabled {
    opacity: 0.5;
    cursor: wait;
  }

  .jira-empty {
    border: 1px dashed var(--wa-border-divider, rgba(123, 143, 160, 0.22));
    border-radius: var(--wa-radius-md, 7px);
    padding: 14px;
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    text-align: center;
  }

  .inspector-empty {
    grid-row: 1 / -1;
    display: grid;
    place-content: center;
    gap: 5px;
    padding: 24px;
    color: var(--wa-text-muted, #667789);
    text-align: center;
  }

  .inspector-empty strong {
    color: var(--wa-text-main, #293847);
    font-size: 12px;
  }

  .inspector-empty span {
    font-size: 10px;
  }

  .create-release-form {
    display: grid;
    gap: 16px;
  }

  .create-release-field {
    min-width: 0;
    display: grid;
    gap: 7px;
  }

  .create-release-field > span {
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    font-weight: 740;
  }

  .create-release-field em {
    color: var(--wa-danger, #dd4b3e);
    font-style: normal;
  }

  .create-release-field input,
  .create-release-field textarea {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.2));
    border-radius: var(--wa-radius-md, 7px);
    background: rgba(255, 255, 255, 0.82);
    color: var(--wa-text-main, #293847);
    padding: 9px 10px;
    font: inherit;
    font-size: 13px;
    line-height: 1.45;
    outline: none;
    resize: vertical;
  }

  .create-release-field input {
    min-height: var(--wa-control-h, 36px);
  }

  .create-release-field textarea {
    min-height: 84px;
  }

  .create-release-field input:focus,
  .create-release-field textarea:focus {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    background: var(--wa-surface-flat, #fbfdff);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .create-release-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .create-release-note {
    margin: 0;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    padding-top: 12px;
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    line-height: 1.55;
  }

  .create-release-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
  }

  @media (max-width: 1100px) {
    .release-plan {
      height: auto;
    }

    .plan-workbench {
      grid-template-columns: minmax(0, 1fr);
    }

    .release-list {
      min-height: 460px;
    }

    .release-inspector {
      min-height: 620px;
    }
  }

  @media (max-width: 760px) {
    .plan-toolbar {
      align-items: stretch;
      flex-direction: column;
    }

    .toolbar-filters {
      min-width: 0;
      display: grid;
      grid-template-columns: minmax(0, 1fr);
    }

    .search-field,
    .toolbar-filters :global(.select-group) {
      width: 100%;
      min-width: 0;
    }

    .toolbar-filters input {
      min-height: 44px;
    }

    .plan-metrics {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .plan-metrics > div:nth-child(3) {
      border-left: 0;
      border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.14));
    }

    .plan-metrics > div:nth-child(4) {
      border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.14));
    }

    .release-list > header {
      align-items: flex-start;
      flex-direction: column;
    }

    .table-scroll {
      overflow-x: auto;
    }

    table {
      min-width: 820px;
    }

    .release-inspector {
      min-height: 680px;
    }

    .jira-search-field > div {
      grid-template-columns: minmax(0, 1fr);
    }

    .jira-search-field input {
      min-height: 44px;
    }

    .create-release-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .create-release-field input {
      min-height: 44px;
    }

    .create-release-actions :global(.btn) {
      flex: 1 1 0;
      min-height: 44px;
    }
  }
</style>
