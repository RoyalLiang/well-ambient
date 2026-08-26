<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import DemandKanban from './DemandKanban.svelte';
  import Alert from './shared/Alert.svelte';
  import Button from './shared/Button.svelte';
  import DatePicker from './shared/DatePicker.svelte';
  import Modal from './shared/Modal.svelte';
  import MultiSelect from './shared/MultiSelect.svelte';
  import Select from './shared/Select.svelte';
  import { showToast } from '../lib/toast';
  import { fetchDeliveryDirectory } from '../lib/delivery-directory';
  import { PagedResource } from '../lib/paged-resource';

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
    status: 'planned' | 'released' | 'archived' | 'discarded';
    start_date: string | null;
    release_date: string | null;
    source_url: string;
    updated_at: string;
  }

  interface ReleasePlanItem {
    release: ReleaseVersion;
    jira_issue_count: number;
    lifecycle?: ReleaseLifecycleCapabilities;
  }

  const releaseResource = new PagedResource<ReleasePlanItem>({
    endpoint: releaseEndpoint,
    itemKey: item => item.release.id,
    pageSize: 50,
    requestInit: () => ({ headers: authHeaders() }),
    errorMessage: '版本事实加载失败'
  });

  interface ReleaseLifecycleCapabilities {
    can_publish: boolean;
    can_archive: boolean;
    can_discard: boolean;
    can_delete: boolean;
    delete_block_reason?: string;
  }

  type LifecycleAction = 'archive' | 'delete';
  type ReleaseListView = 'current' | 'archived';

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

  let jiraResourceReleaseID = 0;
  let jiraResourceSearch = '';
  const linkedJiraResource = new PagedResource<JiraIssue>({
    endpoint: () => `/api/releases/${jiraResourceReleaseID}/jira-issues?scope=linked`,
    itemKey: item => item.work_item_id,
    pageSize: 100,
    requestInit: () => ({ headers: authHeaders() }),
    errorMessage: '已关联 Jira 事项加载失败'
  });
  const candidateJiraResource = new PagedResource<JiraIssue>({
    endpoint: () => {
      const params = new URLSearchParams({ scope: 'candidates' });
      if (jiraResourceSearch) params.set('q', jiraResourceSearch);
      return `/api/releases/${jiraResourceReleaseID}/jira-issues?${params.toString()}`;
    },
    itemKey: item => item.work_item_id,
    pageSize: 100,
    requestInit: () => ({ headers: authHeaders() }),
    errorMessage: 'Jira 事项候选加载失败'
  });

  interface ProjectConfig {
    project_key: string;
    project_name: string;
  }

  let items: ReleasePlanItem[] = [];
  let projects: ProjectConfig[] = [];
  let selectedID = 0;
  let projectFilter = '';
  let releaseListView: ReleaseListView = 'current';
  let search = '';
  let loading = false;
  let loadingMore = false;
  let hasMoreReleases = false;
  let loadedOnce = false;
  let loadError = '';

  let draftProject = '';
  let savingProject = false;
  let jiraSearch = '';
  let linkedJiraIssues: JiraIssue[] = [];
  let jiraCandidates: JiraIssue[] = [];
  let selectedJiraIssueIDs: string[] = [];
  let jiraIssuesLoading = false;
  let linkedJiraLoadingMore = false;
  let candidateJiraLoadingMore = false;
  let hasMoreLinkedJiraIssues = false;
  let hasMoreJiraCandidates = false;
  let jiraSearching = false;
  let jiraSearchError = '';
  let linkingJiraIssues = false;
  let removingJiraIssueID = '';
  let showCreateReleaseModal = false;
  let createName = '';
  let createDescription = '';
  let createProject = '';
  let createStartDate = '';
  let createReleaseDate = '';
  let createError = '';
  let creatingRelease = false;
  let publishConfirming = false;
  let publishReleaseDate = '';
  let publishReason = '';
  let publishError = '';
  let publishingRelease = false;
  let lifecycleAction: LifecycleAction | '' = '';
  let lifecycleReason = '';
  let lifecycleConfirmName = '';
  let lifecycleError = '';
  let applyingLifecycleAction = false;
  let lifecycleReturnFocus: HTMLElement | null = null;
  let releaseFilterTimer: ReturnType<typeof setTimeout> | null = null;

  $: canManageReleases = currentUserPermissions.includes('release:manage');
  $: selectedItem = items.find((item) => item.release.id === selectedID) || null;
  $: selectedLifecycle = selectedItem ? lifecycleCapabilities(selectedItem) : null;
  $: selectedReleaseEditable = selectedItem?.release.status === 'planned';
  $: normalizedSearch = search.trim().toLowerCase();
  $: hasActiveFilters = Boolean(normalizedSearch || projectFilter);
  $: visibleItems = items;
  $: metrics = {
    total: visibleItems.length,
    unbound: visibleItems.filter((item) => !item.release.project_key).length,
    unlinked: visibleItems.filter((item) => !item.jira_issue_count).length,
    planned: visibleItems.filter((item) => item.release.status === 'planned').length,
    archived: visibleItems.filter((item) => item.release.status === 'archived').length
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

  function releaseEndpoint(): string {
    const params = new URLSearchParams({ view: releaseListView });
    if (projectFilter) params.set('project_key', projectFilter);
    if (normalizedSearch) params.set('q', normalizedSearch);
    const query = params.toString();
    return query ? `/api/releases?${query}` : '/api/releases';
  }

  function scheduleReleaseFilterRefresh() {
    if (releaseFilterTimer) clearTimeout(releaseFilterTimer);
    releaseFilterTimer = setTimeout(() => {
      releaseFilterTimer = null;
      void loadVersions(false);
    }, 250);
  }

  function applyProjectFilter(value: string) {
    projectFilter = value;
    void loadVersions(false);
  }

  function switchReleaseListView(view: ReleaseListView) {
    if (releaseListView === view) return;
    releaseListView = view;
    selectedID = 0;
    syncDraft(null);
    void loadVersions(false);
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
    if (status === 'discarded') return '已废弃';
    return '计划中';
  }

  function lifecycleCapabilities(item: ReleasePlanItem): ReleaseLifecycleCapabilities {
    if (item.lifecycle) return item.lifecycle;
    const local = item.release.source === 'local';
    return {
      can_publish: false,
      can_archive: local && item.release.status !== 'archived',
      can_discard: false,
      can_delete: local && item.release.status === 'archived' && item.jira_issue_count === 0,
      delete_block_reason: item.jira_issue_count > 0 ? '请先移除已关联的 Jira 事项后再删除' : ''
    };
  }

  function formatDate(value: string | null): string {
    if (!value) return '未设置';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value.slice(0, 10);
    return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' });
  }

  function formatWindow(release: ReleaseVersion): string {
    if (!release.start_date && !release.release_date) return '未设置';
    return `${formatDate(release.start_date)} 至 ${formatDate(release.release_date)}`;
  }

  function todayValue(): string {
    const now = new Date();
    const offset = now.getTimezoneOffset() * 60_000;
    return new Date(now.getTime() - offset).toISOString().slice(0, 10);
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
    hasMoreLinkedJiraIssues = false;
    hasMoreJiraCandidates = false;
    jiraSearchError = '';
    publishConfirming = false;
    publishReleaseDate = '';
    publishReason = '';
    publishError = '';
    clearLifecycleConfirmation();
  }

  function clearLifecycleConfirmation() {
    lifecycleAction = '';
    lifecycleReason = '';
    lifecycleConfirmName = '';
    lifecycleError = '';
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
    createStartDate = '';
    createReleaseDate = '';
    createError = '';
  }

  function clearFilters() {
    search = '';
    projectFilter = '';
    if (releaseFilterTimer) {
      clearTimeout(releaseFilterTimer);
      releaseFilterTimer = null;
    }
    void loadVersions(false);
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

  function publishErrorMessage(payload: any): string {
    const code = payload?.error || payload?.code;
    if (code === 'release_project_required') return '发布前请先绑定所属项目。';
    if (code === 'external_release_read_only') return 'Jira 导入版本需在来源系统中发布。';
    if (code === 'invalid_release_transition') return '只有计划中的本地版本可以发布。';
    if (code === 'release_state_conflict') return '版本状态已发生变化，请刷新后重试。';
    if (code === 'reason_required') return '请填写本次发布说明。';
    return payload?.message || '版本发布失败';
  }

  function openPublishConfirmation() {
    if (!selectedItem || !canManageReleases || selectedItem.release.source !== 'local' || selectedItem.release.status !== 'planned') return;
    if (!selectedItem.release.project_key) {
      showToast('发布前请先绑定所属项目。', { type: 'error', title: '暂不能发布' });
      return;
    }
    publishReleaseDate = selectedItem.release.release_date?.slice(0, 10) || todayValue();
    publishReason = '发布范围与日期已人工确认';
    publishError = '';
    publishConfirming = true;
  }

  function cancelPublishConfirmation() {
    if (publishingRelease) return;
    publishConfirming = false;
    publishError = '';
  }

  async function publishRelease() {
    if (!selectedItem || publishingRelease) return;
    if (!publishReleaseDate) {
      publishError = '请选择实际发布日期。';
      return;
    }
    if (!publishReason.trim()) {
      publishError = '请填写本次发布说明。';
      return;
    }
    const releaseID = selectedItem.release.id;
    const releaseName = selectedItem.release.name;
    publishingRelease = true;
    publishError = '';
    try {
      const response = await fetch(`/api/releases/${releaseID}/publish`, {
        method: 'POST',
        headers: authHeaders(true),
        body: JSON.stringify({
          release_date: publishReleaseDate,
          reason: publishReason.trim()
        })
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(publishErrorMessage(payload));
      publishConfirming = false;
      showToast(`版本“${releaseName}”已发布，发布事实已进入交付决策看板。`, {
        title: payload?.replayed ? '版本发布事实已确认' : '版本已发布'
      });
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new CustomEvent('well-ambient:release-published', { detail: { releaseID } }));
      }
      await loadVersions(true);
    } catch (error: any) {
      publishError = error?.message || '版本发布失败';
    } finally {
      publishingRelease = false;
    }
  }

  function lifecycleActionTitle(action: LifecycleAction): string {
    if (action === 'archive') return '归档版本';
    return '删除版本';
  }

  function lifecycleActionDescription(action: LifecycleAction): string {
    if (action === 'archive') return '归档后版本会移入“归档版本”列表，项目与 Jira 范围保持只读。';
    return '删除会把版本从归档列表移除；审计记录仍会保留，操作不能从页面恢复。';
  }

  function lifecycleErrorMessage(payload: any, action: LifecycleAction): string {
    const code = payload?.error || payload?.code;
    if (code === 'reason_required') return `请填写${lifecycleActionTitle(action)}原因。`;
    if (code === 'release_delete_confirmation_mismatch') return '输入的版本名称与当前版本不一致。';
    if (code === 'release_delete_blocked') return '请先移除版本关联的 Jira 事项或 Jira 版本。';
    if (code === 'release_delete_forbidden') return '只有归档版本可以删除，请先完成归档。';
    if (code === 'external_release_read_only') return 'Jira 来源版本只能在来源系统中维护。';
    if (code === 'invalid_release_transition') return '版本状态已变化，请刷新后重试。';
    if (code === 'release_state_conflict') return '版本状态已发生并发变化，请刷新后重试。';
    return payload?.message || `${lifecycleActionTitle(action)}失败`;
  }

  function openLifecycleConfirmation(action: LifecycleAction, event: MouseEvent) {
    if (!selectedItem || !selectedLifecycle || !canManageReleases || applyingLifecycleAction) return;
    if (action === 'archive' && !selectedLifecycle.can_archive) return;
    if (action === 'delete' && !selectedLifecycle.can_delete) return;
    lifecycleReturnFocus = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
    lifecycleAction = action;
    lifecycleReason = '';
    lifecycleConfirmName = '';
    lifecycleError = '';
  }

  async function cancelLifecycleConfirmation() {
    if (applyingLifecycleAction) return;
    const returnFocus = lifecycleReturnFocus;
    clearLifecycleConfirmation();
    await tick();
    returnFocus?.focus();
  }

  async function applyLifecycleAction() {
    if (!selectedItem || !lifecycleAction || applyingLifecycleAction) return;
    const action = lifecycleAction;
    const reason = lifecycleReason.trim();
    if (!reason) {
      lifecycleError = `请填写${lifecycleActionTitle(action)}原因。`;
      return;
    }
    if (action === 'delete' && lifecycleConfirmName.trim() !== selectedItem.release.name) {
      lifecycleError = '请输入完整且一致的版本名称。';
      return;
    }

    const releaseID = selectedItem.release.id;
    const releaseName = selectedItem.release.name;
    applyingLifecycleAction = true;
    lifecycleError = '';
    try {
      const endpoint = action === 'delete'
        ? `/api/releases/${releaseID}`
        : `/api/releases/${releaseID}/${action}`;
      const response = await fetch(endpoint, {
        method: action === 'delete' ? 'DELETE' : 'POST',
        headers: authHeaders(true),
        body: JSON.stringify({
          reason,
          ...(action === 'delete' ? { confirm_name: lifecycleConfirmName.trim() } : {})
        })
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(lifecycleErrorMessage(payload, action));

      clearLifecycleConfirmation();
      const title = action === 'archive' ? '版本已归档' : '版本已删除';
      const message = action === 'archive'
        ? `版本“${releaseName}”已移入归档版本列表。`
        : `版本“${releaseName}”已从归档列表移除，审计记录仍保留。`;
      showToast(message, { title });
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new CustomEvent('well-ambient:release-lifecycle-changed', {
          detail: { releaseID, action }
        }));
      }
      if (action === 'delete') selectedID = 0;
      await loadVersions(action !== 'delete');
    } catch (error: any) {
      lifecycleError = error?.message || `${lifecycleActionTitle(action)}失败`;
      showToast(lifecycleError, { type: 'error', title: `无法${lifecycleActionTitle(action)}` });
    } finally {
      applyingLifecycleAction = false;
    }
  }

  async function loadVersions(preserveSelection = true) {
    loading = true;
    loadError = '';
    try {
      let [releaseState, directory] = await Promise.all([
        releaseResource.refresh(),
        fetchDeliveryDirectory()
      ]);
      if (releaseState.error) {
        throw new Error(releaseState.error);
      }
      if (preserveSelection && selectedID && !releaseState.items.some(item => item.release.id === selectedID)) {
        while (releaseState.page?.has_more && releaseState.page?.next_cursor) {
          releaseState = await releaseResource.loadMore();
          if (releaseState.error || releaseState.items.some(item => item.release.id === selectedID)) break;
        }
      }
      const nextItems = releaseState.items;
      items = nextItems;
      hasMoreReleases = Boolean(releaseState.page?.has_more && releaseState.page?.next_cursor);
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

  async function loadMoreVersions() {
    if (loadingMore || !hasMoreReleases) return;
    loadingMore = true;
    loadError = '';
    try {
      const state = await releaseResource.loadMore();
      if (state.error) throw new Error(state.error);
      items = state.items;
      hasMoreReleases = Boolean(state.page?.has_more && state.page?.next_cursor);
    } catch (error: any) {
      loadError = error?.message || '更多版本事实加载失败';
    } finally {
      loadingMore = false;
    }
  }

  async function saveProjectBinding() {
    if (!selectedItem || !canManageReleases || !selectedReleaseEditable || savingProject) return;
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
      hasMoreLinkedJiraIssues = false;
      hasMoreJiraCandidates = false;
      return;
    }
    const releaseID = release.id;
    jiraIssuesLoading = true;
    jiraSearchError = '';
    jiraResourceReleaseID = releaseID;
    jiraResourceSearch = searchTerm.trim();
    try {
      const [linkedState, candidateState] = await Promise.all([
        linkedJiraResource.refresh(),
        candidateJiraResource.refresh()
      ]);
      if (linkedState.error) throw new Error(linkedState.error);
      if (candidateState.error) throw new Error(candidateState.error);
      if (selectedID !== releaseID) return;
      linkedJiraIssues = linkedState.items;
      jiraCandidates = candidateState.items;
      hasMoreLinkedJiraIssues = Boolean(linkedState.page?.has_more && linkedState.page?.next_cursor);
      hasMoreJiraCandidates = Boolean(candidateState.page?.has_more && candidateState.page?.next_cursor);
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

  async function loadMoreLinkedJiraIssues() {
    if (linkedJiraLoadingMore || !hasMoreLinkedJiraIssues) return;
    linkedJiraLoadingMore = true;
    try {
      const state = await linkedJiraResource.loadMore();
      if (state.error) throw new Error(state.error);
      linkedJiraIssues = state.items;
      hasMoreLinkedJiraIssues = Boolean(state.page?.has_more && state.page?.next_cursor);
    } catch (error: any) {
      jiraSearchError = error?.message || '更多已关联 Jira 事项加载失败';
    } finally {
      linkedJiraLoadingMore = false;
    }
  }

  async function loadMoreJiraCandidates() {
    if (candidateJiraLoadingMore || !hasMoreJiraCandidates) return;
    candidateJiraLoadingMore = true;
    try {
      const state = await candidateJiraResource.loadMore();
      if (state.error) throw new Error(state.error);
      jiraCandidates = state.items;
      hasMoreJiraCandidates = Boolean(state.page?.has_more && state.page?.next_cursor);
    } catch (error: any) {
      jiraSearchError = error?.message || '更多 Jira 事项候选加载失败';
    } finally {
      candidateJiraLoadingMore = false;
    }
  }

  async function searchJiraIssues() {
    if (!selectedItem?.release.project_key || !selectedReleaseEditable || jiraSearching) return;
    jiraSearching = true;
    await loadSelectedReleaseJiraIssues(jiraSearch);
    jiraSearching = false;
  }

  async function linkSelectedJiraIssues() {
    if (!selectedItem || !canManageReleases || !selectedReleaseEditable || linkingJiraIssues || selectedJiraIssueIDs.length === 0) return;
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
    if (!selectedItem || !canManageReleases || !selectedReleaseEditable || removingJiraIssueID) return;
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

  onDestroy(() => {
    if (releaseFilterTimer) clearTimeout(releaseFilterTimer);
    releaseResource.dispose();
    linkedJiraResource.dispose();
    candidateJiraResource.dispose();
  });
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
  <section class="release-plan" class:has-feedback={!!loadError} aria-label="版本计划">
    <header class="plan-toolbar wa-admin-toolbar">
      <div class="toolbar-heading">
        <strong>现有版本与 Jira 事项</strong>
        <div class="release-view-tabs" role="tablist" aria-label="版本列表视图">
          <button
            type="button"
            role="tab"
            aria-selected={releaseListView === 'current'}
            class:active={releaseListView === 'current'}
            on:click={() => switchReleaseListView('current')}
          >当前版本</button>
          <button
            type="button"
            role="tab"
            aria-selected={releaseListView === 'archived'}
            class:active={releaseListView === 'archived'}
            on:click={() => switchReleaseListView('archived')}
          >归档版本</button>
        </div>
      </div>
      <div class="toolbar-filter-group" role="group" aria-label="版本筛选">
        <label class="search-field">
          <span class="sr-only">搜索版本</span>
          <input bind:value={search} on:input={scheduleReleaseFilterRefresh} type="search" placeholder="搜索版本或项目" aria-label="搜索版本" />
        </label>
        <Select
          value={projectFilter}
          options={projectOptions}
          compact={true}
          clearable={true}
          shadowless={true}
          ariaLabel="筛选项目"
          on:change={(event) => applyProjectFilter(event.detail)}
        />
        {#if hasActiveFilters}
          <Button variant="ghost" size="small" on:click={clearFilters}>重置</Button>
        {/if}
      </div>
      <div class="toolbar-actions">
        <Button variant="secondary" size="small" on:click={() => loadVersions(true)} disabled={loading}>刷新</Button>
        {#if canManageReleases && releaseListView === 'current'}
          <Button variant="primary" size="small" on:click={openCreateRelease}>创建版本</Button>
        {/if}
      </div>
    </header>

    {#if loadError}
      <div class="plan-feedback"><Alert type="error" message={loadError} /></div>
    {/if}

    <div class="plan-metrics" aria-label="版本计划摘要">
      <div><span>{releaseListView === 'archived' ? '归档版本' : '当前版本'}</span><strong>{metrics.total}</strong></div>
      <div class:attention={metrics.unbound > 0}><span>未绑定项目</span><strong>{metrics.unbound}</strong></div>
      <div class:attention={metrics.unlinked > 0}><span>未关联 Jira 事项</span><strong>{metrics.unlinked}</strong></div>
      <div><span>{releaseListView === 'archived' ? '已归档' : '计划中'}</span><strong>{releaseListView === 'archived' ? metrics.archived : metrics.planned}</strong></div>
    </div>

    <div class="plan-workbench">
      <section class="release-list" aria-label={releaseListView === 'archived' ? '归档版本列表' : '当前版本列表'} aria-busy={loading}>
        <header>
          <div>
            <strong>{releaseListView === 'archived' ? '归档版本事实' : '当前版本事实'}</strong>
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
              {#if loading && items.length === 0}
                <tr><td colspan="6" class="empty-row">正在加载现有版本…</td></tr>
              {:else if visibleItems.length === 0}
                <tr>
                  <td colspan="6" class="empty-row">
                    {items.length === 0
                      ? releaseListView === 'archived'
                        ? '当前还没有归档版本。'
                        : canManageReleases
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
                {#if hasMoreReleases}
                  <tr class="load-more-row">
                    <td colspan="6">
                      <button type="button" on:click={loadMoreVersions} disabled={loadingMore}>
                        {loadingMore ? '正在加载更多版本' : '加载更多版本'}
                      </button>
                    </td>
                  </tr>
                {/if}
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
            <div class="inspector-header-actions">
              <span class="status status-{selectedItem.release.status}">{statusLabel(selectedItem.release.status)}</span>
              {#if canManageReleases && releaseListView === 'current' && selectedLifecycle?.can_archive}
                <Button variant="secondary" size="small" on:click={(event) => openLifecycleConfirmation('archive', event)}>归档版本</Button>
              {/if}
              {#if canManageReleases && releaseListView === 'archived'}
                <Button
                  variant="danger"
                  size="small"
                  disabled={!selectedLifecycle?.can_delete}
                  on:click={(event) => openLifecycleConfirmation('delete', event)}
                >删除版本</Button>
              {/if}
            </div>
          </header>

          <div class="inspector-body">
            {#if lifecycleAction}
              <section
                class="lifecycle-confirmation"
                class:danger={lifecycleAction === 'delete'}
                aria-labelledby="lifecycle-action-title"
                aria-describedby="lifecycle-action-description"
              >
                <div class="lifecycle-confirmation-heading">
                  <div>
                    <h3 id="lifecycle-action-title">确认{lifecycleActionTitle(lifecycleAction)}</h3>
                    <p id="lifecycle-action-description">{lifecycleActionDescription(lifecycleAction)}</p>
                  </div>
                  <span class="lifecycle-release-name">{selectedItem.release.name}</span>
                </div>
                <label class="lifecycle-field">
                  <span>操作原因 <em>*</em></span>
                  <textarea
                    bind:value={lifecycleReason}
                    rows="3"
                    maxlength="500"
                    aria-required="true"
                    placeholder={`说明为什么要${lifecycleActionTitle(lifecycleAction)}`}
                  ></textarea>
                </label>
                {#if lifecycleAction === 'delete'}
                  <label class="lifecycle-field">
                    <span>输入版本名称以确认 <em>*</em></span>
                    <input
                      bind:value={lifecycleConfirmName}
                      type="text"
                      autocomplete="off"
                      aria-required="true"
                      placeholder={selectedItem.release.name}
                    />
                  </label>
                {/if}
                {#if lifecycleError}
                  <Alert type="error" message={lifecycleError} />
                {/if}
                <div class="section-actions lifecycle-confirmation-actions">
                  <Button variant="secondary" size="small" disabled={applyingLifecycleAction} on:click={cancelLifecycleConfirmation}>取消</Button>
                  <Button
                    variant={lifecycleAction === 'delete' ? 'danger' : 'primary'}
                    size="small"
                    loading={applyingLifecycleAction}
                    on:click={applyLifecycleAction}
                  >确认{lifecycleActionTitle(lifecycleAction)}</Button>
                </div>
              </section>
            {/if}

            {#if !canManageReleases}
              <Alert type="info" message="当前账号只有查看权限，项目与 Jira 事项关联不可编辑。" />
            {:else if selectedItem.release.status !== 'planned'}
              <Alert type="info" message="已发布、已归档或已废弃版本的项目与 Jira 范围保持只读。" />
            {:else if selectedItem.release.source !== 'local'}
              <Alert type="info" message="Jira 导入版本需在来源系统中维护发布状态。" />
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
              <div class="project-binding-controls">
                <Select
                  id="release-project"
                  label="所属项目"
                  value={draftProject}
                  options={bindingProjectOptions}
                  compact={true}
                  clearable={true}
                  disabled={!canManageReleases || !selectedReleaseEditable}
                  shadowless={true}
                  placeholder="暂不绑定项目"
                  searchPlaceholder="搜索项目"
                  on:change={(event) => draftProject = event.detail}
                />
                <Button
                  variant="secondary"
                  size="medium"
                  loading={savingProject}
                  disabled={!canManageReleases || !selectedReleaseEditable || draftProject === selectedItem.release.project_key}
                  on:click={saveProjectBinding}
                >保存绑定</Button>
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
                      disabled={!canManageReleases || !selectedReleaseEditable}
                      placeholder="输入 Jira 编号或标题"
                      on:keydown={handleJiraSearchKeydown}
                    />
                    <Button
                      variant="secondary"
                      size="small"
                      loading={jiraSearching}
                      disabled={!canManageReleases || !selectedReleaseEditable}
                      on:click={searchJiraIssues}
                    >搜索</Button>
                  </div>
                </label>

                <MultiSelect
                  id="release-jira-issues"
                  label="批量选择"
                  values={selectedJiraIssueIDs}
                  options={jiraCandidateOptions}
                  disabled={!canManageReleases || !selectedReleaseEditable || jiraIssuesLoading}
                  placeholder={jiraIssuesLoading ? '正在加载 Jira 事项…' : '选择要纳入当前版本的 Jira 事项'}
                  searchPlaceholder="在当前候选中筛选"
                  emptyText={jiraSearch ? '没有匹配的 Jira 事项' : '当前项目没有可关联的 Jira 事项'}
                  helperText="已属于其他版本的事项会保留展示但不可重复选择。"
                  showClear={true}
                  summaryMode={true}
                  overlay={true}
                  shadowless={true}
                  on:change={(event) => selectedJiraIssueIDs = event.detail}
                />
                {#if hasMoreJiraCandidates}
                  <div class="section-actions">
                    <Button variant="ghost" size="small" loading={candidateJiraLoadingMore} on:click={loadMoreJiraCandidates}>加载更多候选</Button>
                  </div>
                {/if}
                <div class="section-actions">
                  <Button
                    variant="primary"
                    size="small"
                    loading={linkingJiraIssues}
                    disabled={!canManageReleases || !selectedReleaseEditable || selectedJiraIssueIDs.length === 0}
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
                        {#if canManageReleases && selectedReleaseEditable}
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
                {#if hasMoreLinkedJiraIssues}
                  <div class="section-actions">
                    <Button variant="ghost" size="small" loading={linkedJiraLoadingMore} on:click={loadMoreLinkedJiraIssues}>加载更多已关联事项</Button>
                  </div>
                {/if}
              {:else}
                <div class="jira-empty">当前版本尚未关联 Jira 事项。</div>
              {/if}
            </section>

            {#if canManageReleases && releaseListView === 'archived' && !selectedLifecycle?.can_delete && selectedLifecycle?.delete_block_reason}
              <Alert type="info" message={selectedLifecycle.delete_block_reason} />
            {/if}
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
    shadowless={true}
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

      <div class="create-release-grid single">
        <Select
          id="create-release-project"
          label="所属项目"
          value={createProject}
          options={bindingProjectOptions}
          clearable={true}
          compact={true}
          shadowless={true}
          placeholder="暂不绑定项目"
          searchPlaceholder="搜索项目"
          on:change={(event) => createProject = event.detail}
        />
      </div>

      <div class="create-release-grid">
        <DatePicker
          id="create-release-start-date"
          label="计划开始日期"
          value={createStartDate}
          max={createReleaseDate}
          compact={true}
          shadowless={true}
          placeholder="选择开始日期"
          on:change={(event) => createStartDate = event.detail}
        />
        <DatePicker
          id="create-release-release-date"
          label="发布日期"
          value={createReleaseDate}
          min={createStartDate}
          compact={true}
          shadowless={true}
          placeholder="选择发布日期"
          on:change={(event) => createReleaseDate = event.detail}
        />
      </div>

      <p class="create-release-note">新版本从计划中开始；绑定项目并核对 Jira 范围后，可在右侧检查器中发布。</p>

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
    --wa-shadow-sm: none;
    --wa-shadow-md: none;
    --wa-shadow-peer: none;
    --wa-shadow-glass: none;
    --wa-shadow-panel: none;
    --wa-shadow-glow: none;
    width: 100%;
    height: 100%;
    min-height: 0;
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr);
    gap: 10px;
    color: var(--wa-text-main, #293847);
  }

  .release-plan.has-feedback {
    grid-template-rows: auto auto auto minmax(0, 1fr);
  }

  .release-plan :global(.btn),
  .release-plan :global(.select-trigger),
  .release-plan :global(.multi-select-trigger),
  .release-plan :global(.date-trigger) {
    box-shadow: none !important;
  }

  .release-plan :global(*) {
    box-shadow: none !important;
  }

  .release-plan :global(.btn:focus-visible),
  .release-plan :global(.select-trigger:focus-within),
  .release-plan :global(.multi-select-trigger:focus-within),
  .release-plan :global(.date-trigger:focus-visible) {
    outline: 2px solid rgba(0, 143, 150, 0.22);
    outline-offset: 2px;
  }

  .plan-toolbar,
  .toolbar-filter-group,
  .toolbar-actions,
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
    min-width: 0;
    min-height: 58px;
    display: grid;
    grid-template-columns: minmax(188px, auto) minmax(0, 1fr) auto;
    gap: 14px;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-xl, 18px);
    padding: 10px 14px;
    background: var(--wa-surface-flat, #fbfdfe);
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .toolbar-heading {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

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

  .release-view-tabs {
    width: fit-content;
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 3px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.2));
    border-radius: var(--wa-radius-pill, 999px);
    background: var(--wa-surface-inset, rgba(232, 241, 244, 0.72));
  }

  .release-view-tabs button {
    min-height: 28px;
    padding: 0 11px;
    border: 0;
    border-radius: var(--wa-radius-pill, 999px);
    background: transparent;
    color: var(--wa-text-muted, #667789);
    font: inherit;
    font-size: 11px;
    font-weight: 740;
    cursor: pointer;
  }

  .release-view-tabs button.active {
    background: var(--wa-surface-flat, #fbfdfe);
    color: var(--wa-accent-strong, #006f76);
    box-shadow: 0 1px 3px rgba(28, 54, 67, 0.1);
  }

  .release-view-tabs button:focus-visible {
    outline: 2px solid var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    outline-offset: 2px;
  }

  .toolbar-heading small {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    font-variant-numeric: tabular-nums;
  }

  .toolbar-filter-group {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(180px, 1fr) minmax(132px, 168px) auto;
    gap: 8px;
  }

  .toolbar-actions {
    justify-content: flex-end;
    gap: 8px;
  }

  .search-field {
    min-width: 230px;
    flex: 1 1 280px;
  }

  .toolbar-filter-group :global(.select-group) {
    width: 100%;
  }

  .toolbar-filter-group input,
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

  .toolbar-filter-group input:focus,
  .jira-search-field input:focus {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    outline: 2px solid rgba(0, 143, 150, 0.16);
    outline-offset: 1px;
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

  .load-more-row td { padding: 0; text-align: center; }
  .load-more-row button { width: 100%; min-height: 44px; border: 0; background: var(--wa-surface-panel, #fbfdff); color: var(--wa-accent-strong, #006f76); font: 760 11px/1 var(--wa-font-sans, sans-serif); cursor: pointer; }
  .load-more-row button:hover { background: var(--wa-row-hover, #f2f8fb); }
  .load-more-row button:focus-visible { outline: 2px solid var(--wa-border-focus, rgba(1, 139, 141, .86)); outline-offset: -2px; }
  .load-more-row button:disabled { cursor: progress; opacity: .62; }

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

  .status-discarded {
    background: rgba(188, 112, 41, 0.1);
    color: #9a5d24;
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

  .inspector-header-actions {
    flex: none;
    display: flex !important;
    align-items: center;
    justify-content: flex-end;
    gap: 8px !important;
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

  .publish-confirmation {
    display: grid;
    gap: 11px;
    border: 1px solid rgba(0, 143, 150, 0.24);
    border-radius: var(--wa-radius-lg, 14px);
    padding: 12px;
    background: rgba(0, 143, 150, 0.045);
    box-shadow: none;
  }

  .lifecycle-confirmation {
    display: grid;
    gap: 12px;
    border: 1px solid rgba(188, 112, 41, 0.24);
    border-radius: var(--wa-radius-lg, 14px);
    padding: 12px;
    background: rgba(188, 112, 41, 0.045);
  }

  .lifecycle-confirmation.danger {
    border-color: rgba(221, 75, 62, 0.24);
    background: rgba(221, 75, 62, 0.045);
  }

  .lifecycle-confirmation-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .lifecycle-confirmation-heading > div {
    min-width: 0;
    display: grid;
    gap: 4px;
  }

  .lifecycle-confirmation h3,
  .lifecycle-confirmation p {
    margin: 0;
  }

  .lifecycle-confirmation h3 {
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
  }

  .lifecycle-confirmation p {
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    line-height: 1.45;
  }

  .lifecycle-release-name {
    max-width: 42%;
    overflow: hidden;
    border-radius: 999px;
    padding: 4px 8px;
    background: rgba(255, 255, 255, 0.72);
    color: var(--wa-text-main, #293847) !important;
    font-size: 10px !important;
    letter-spacing: 0 !important;
    text-overflow: ellipsis;
    text-transform: none !important;
    white-space: nowrap;
  }

  .lifecycle-field {
    display: grid;
    gap: 8px;
  }

  .lifecycle-field > span {
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    font-weight: 740;
  }

  .lifecycle-field em {
    color: var(--wa-danger, #dd4b3e);
    font-style: normal;
  }

  .lifecycle-field textarea,
  .lifecycle-field input {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.2));
    border-radius: var(--wa-radius-md, 7px);
    padding: 8px 10px;
    background: rgba(255, 255, 255, 0.82);
    color: var(--wa-text-main, #293847);
    font: inherit;
    font-size: 12px;
    line-height: 1.45;
  }

  .lifecycle-field textarea {
    min-height: 72px;
    resize: vertical;
  }

  .lifecycle-field input {
    min-height: 38px;
  }

  .lifecycle-field textarea:focus,
  .lifecycle-field input:focus {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    outline: 2px solid rgba(0, 143, 150, 0.16);
    outline-offset: 1px;
  }

  .lifecycle-confirmation-actions {
    gap: 8px;
  }

  .publish-confirmation-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .publish-confirmation-heading > div {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .publish-confirmation h3,
  .publish-confirmation p {
    margin: 0;
  }

  .publish-confirmation h3 {
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
  }

  .publish-confirmation p {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    line-height: 1.45;
  }

  .publish-confirmation-heading > button {
    width: 32px;
    height: 32px;
    flex: none;
    display: grid;
    place-items: center;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-md, 7px);
    padding: 0;
    background: transparent;
    color: var(--wa-text-muted, #667789);
    font: inherit;
    font-size: 18px;
    cursor: pointer;
  }

  .publish-confirmation-heading > button:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.22);
    outline-offset: 2px;
  }

  .publish-reason-field {
    display: grid;
    gap: 7px;
  }

  .publish-reason-field > span {
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    font-weight: 740;
  }

  .publish-reason-field em {
    color: var(--wa-danger, #dd4b3e);
    font-style: normal;
  }

  .publish-reason-field textarea {
    box-sizing: border-box;
    width: 100%;
    min-height: 72px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.2));
    border-radius: var(--wa-radius-md, 7px);
    padding: 9px 10px;
    background: rgba(255, 255, 255, 0.82);
    color: var(--wa-text-main, #293847);
    font: inherit;
    font-size: 12px;
    line-height: 1.45;
    resize: vertical;
  }

  .publish-reason-field textarea:focus {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    outline: 2px solid rgba(0, 143, 150, 0.16);
    outline-offset: 1px;
  }

  .publish-actions {
    gap: 8px;
  }

  .section-heading {
    align-items: flex-start;
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

  .project-binding-controls {
    --wa-control-h: 38px;
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) max-content;
    align-items: end;
    gap: 8px;
  }

  .project-binding-controls :global(.select-group) {
    min-width: 0;
  }

  .release-management {
    gap: 12px;
    border-bottom: 0;
    padding-top: 8px;
    padding-bottom: 0;
  }

  .release-management-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }

  .delete-block-reason {
    margin: 0;
    color: var(--wa-text-muted, #667789);
    font-size: 10px;
    line-height: 1.45;
    text-align: right;
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
    outline: 2px solid rgba(0, 143, 150, 0.16);
    outline-offset: 1px;
  }

  .create-release-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .create-release-grid.single {
    grid-template-columns: minmax(0, 1fr);
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

    .plan-toolbar {
      grid-template-columns: minmax(0, 1fr) auto;
    }

    .toolbar-filter-group {
      grid-column: 1 / -1;
      grid-row: 2;
    }

    .toolbar-actions {
      grid-column: 2;
      grid-row: 1;
    }

    .release-list {
      min-height: 460px;
    }

    .release-inspector {
      min-height: 0;
    }
  }

  @media (max-width: 760px) {
    .plan-toolbar {
      grid-template-columns: minmax(0, 1fr);
      align-items: stretch;
      gap: 10px;
      padding: 12px;
    }

    .toolbar-filter-group {
      grid-column: 1;
      grid-row: auto;
      min-width: 0;
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      grid-template-areas:
        "search"
        "project"
        "reset";
    }

    .search-field {
      grid-area: search;
    }

    .toolbar-filter-group :global(.select-group):first-of-type {
      grid-area: project;
    }

    .toolbar-filter-group :global(.btn) {
      grid-area: reset;
    }

    .toolbar-actions {
      grid-column: 1;
      grid-row: auto;
      justify-content: stretch;
    }

    .search-field,
    .toolbar-filter-group :global(.select-group) {
      width: 100%;
      min-width: 0;
    }

    .toolbar-filter-group input,
    .toolbar-filter-group :global(.select-trigger),
    .toolbar-filter-group :global(.select-inline-input),
    .toolbar-filter-group :global(.select-toggle),
    .toolbar-filter-group :global(.btn),
    .toolbar-actions :global(.btn),
    .release-view-tabs button {
      min-height: 44px;
    }

    .toolbar-filter-group :global(.select-toggle) {
      min-width: 44px;
    }

    .toolbar-actions :global(.btn) {
      flex: 1 1 0;
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

    .release-inspector > header {
      align-items: flex-start;
    }

    .inspector-header-actions {
      align-items: flex-end;
      flex-direction: column;
    }

    .publish-confirmation {
      --wa-control-h: 44px;
    }

    .publish-confirmation-heading > button {
      width: 44px;
      height: 44px;
    }

    .project-binding-controls,
    .lifecycle-confirmation {
      --wa-control-h: 44px;
    }

    .project-binding-controls :global(.select-trigger),
    .project-binding-controls :global(.btn),
    .inspector-header-actions :global(.btn),
    .lifecycle-confirmation-actions :global(.btn),
    .release-management-actions :global(.btn),
    .lifecycle-field input {
      min-height: 44px;
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

  @media (max-width: 430px) {
    .toolbar-filter-group {
      grid-template-columns: minmax(0, 1fr);
      grid-template-areas:
        "search"
        "project"
        "status"
        "reset";
    }
  }

  @media (max-width: 430px) {
    .release-inspector > header {
      align-items: stretch;
      flex-direction: column;
    }

    .inspector-header-actions {
      width: 100%;
      align-items: stretch;
    }

    .inspector-header-actions :global(.btn) {
      width: 100%;
    }

    .section-heading,
    .lifecycle-confirmation-heading {
      align-items: flex-start;
      flex-direction: column;
    }

    .lifecycle-release-name {
      max-width: 100%;
    }

    .project-binding-controls {
      grid-template-columns: minmax(0, 1fr);
    }

    .project-binding-controls :global(.btn),
    .lifecycle-confirmation-actions :global(.btn),
    .release-management-actions :global(.btn) {
      width: 100%;
    }

    .lifecycle-confirmation-actions,
    .release-management-actions {
      align-items: stretch;
      flex-direction: column;
    }

    .delete-block-reason {
      text-align: left;
    }
  }
</style>
