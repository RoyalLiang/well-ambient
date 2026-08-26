<script lang="ts">
  import { onMount } from 'svelte';
  import Alert from './shared/Alert.svelte';
  import Button from './shared/Button.svelte';
  import Select from './shared/Select.svelte';
  import AdminDataList from './admin-console/AdminDataList.svelte';
  import AdminListFilterBar from './admin-console/AdminListFilterBar.svelte';
  import type {
    AdminCellValue,
    AdminTableColumn,
    AdminTableRow
  } from '../lib/admin-console/contract';
  import { showToast } from '../lib/toast';
  import {
    flattenBoundedPages,
    updateBoundedPageWindow,
    type BoundedPage,
    type DataWindowDirection
  } from '../lib/admin-data-window';
  import { dailyJiraSnapshotFingerprint } from '../lib/daily-jira-snapshot';
  import { fetchDeliveryDirectory, type DeliveryAssigneeOption } from '../lib/delivery-directory';
  import { subscribeTelemetryUpdates } from '../lib/telemetry-refresh';

  type DailyJiraBucketKey = 'today' | 'three_day' | 'seven_day';
  type DailyJiraDecision = 'follow_up' | 'escalate' | 'reassign';
  type AssigneeMode = 'reporter' | 'specified';

  interface DecisionEvent {
    id: number;
    task_id: string;
    actor: string;
    action: string;
    old_value: string;
    new_value: string;
    reason: string;
    created_at: string;
  }

  interface DailyJiraItem {
    task_id: string;
    title: string;
    project: string;
    assignee: string;
    reporter: string;
    status: string;
    issue_type: string;
    task_created_at: string;
    last_update: string;
    due_date: string | null;
    age_days: number;
    bucket: DailyJiraBucketKey;
    overdue: boolean;
    decision_logs: string;
    decision_events: DecisionEvent[];
    latest_decision: DailyJiraDecisionState | null;
  }

  interface DailyJiraDecisionState {
    id: number;
    status: DailyJiraDecision;
    assignee: string;
    actor: string;
    note: string;
    decided_at: string;
    reminder_at: string;
    reminder_due: boolean;
  }

  interface DailyJiraBucket {
    key: DailyJiraBucketKey;
    label: string;
    description: string;
    count: number;
    items: DailyJiraItem[];
  }

  interface DailyJiraPageMeta {
    bucket: DailyJiraBucketKey;
    limit: number;
    generation: number;
    search_mode: string;
    previous_cursor?: string;
    has_previous: boolean;
    next_cursor?: string;
    has_more: boolean;
  }

  interface DailyJiraAuditResponse {
    generated_at: string;
    summary: {
      total: number;
      today: number;
      three_day: number;
      seven_day: number;
    };
    buckets: DailyJiraBucket[];
    assignees: string[];
    recent_watch_count: number;
    unclassified_count: number;
    page: DailyJiraPageMeta;
  }

  interface DailyJiraAdminRow extends AdminTableRow {
    item: DailyJiraItem;
  }

  type DailyJiraWindowPage = BoundedPage<DailyJiraItem, DailyJiraPageMeta>;

  interface SelectOption {
    value: string;
    label: string;
    meta?: string;
  }

  type JiraTimingTone = 'overdue' | 'due-soon' | 'healthy';

  interface JiraTimingState {
    tone: JiraTimingTone;
    label: '逾期' | '临期' | '健康';
    ariaLabel: string;
  }

  export let currentUserPermissions: string[] = [];

  const decisionOptions: SelectOption[] = [
    { value: 'follow_up', label: '继续跟进', meta: '保留负责人 · 24 小时后复核' },
    { value: 'escalate', label: '升级协同', meta: '跨团队介入 · 4 小时后复核' },
    { value: 'reassign', label: '快速转派', meta: '更新负责人 · 24 小时后复核' }
  ];
  const auditPageLimit = 100;
  const dailyJiraWindowPageLimit = 3;
  const dailyJiraVirtualOptions = {
    rowHeight: 52,
    overscan: 8,
    loadAheadRows: 12
  };
  const dailyJiraColumns: AdminTableColumn[] = [
    { key: 'timing', label: '时效', width: '54px', align: 'center' },
    { key: 'issue', label: 'Jira 事项', width: '31%' },
    { key: 'project', label: '项目', width: '20%', priority: 'secondary' },
    { key: 'assignee', label: '负责人', width: '112px' },
    { key: 'status', label: '状态 / 决策', width: '92px' },
    { key: 'lastUpdate', label: '最近活动', width: '110px', priority: 'secondary' }
  ];

  let audit: DailyJiraAuditResponse | null = null;
  let jiraBaseUrl = '';
  let activeBucketKey: DailyJiraBucketKey = 'seven_day';
  let selectedTaskID = '';
  let selectedItemSnapshot: DailyJiraItem | null = null;
  let searchText = '';
  let selectedDecision: DailyJiraDecision = 'follow_up';
  let assigneeMode: AssigneeMode = 'specified';
  let newAssignee = '';
  let decisionNote = '';
  let loading = true;
  let refreshing = false;
  let loadingPrevious = false;
  let loadPreviousError = '';
  let loadingMore = false;
  let loadMoreError = '';
  let refreshQueued = false;
  let submitting = false;
  let error = '';
  let hasInitializedBucket = false;
  let auditFingerprint = '';
  let lastCheckedAt = '';
  let auxiliaryResourcesLoaded = false;
  let loadedSearch = '';
  let searchTimer: number | undefined;
  let pageStates: Partial<Record<DailyJiraBucketKey, DailyJiraPageMeta>> = {};
  let pageWindows: Partial<Record<DailyJiraBucketKey, DailyJiraWindowPage[]>> = {};
  let deliveryAssignees: DeliveryAssigneeOption[] = [];

  $: canWrite = currentUserPermissions.includes('demands:write');
  $: buckets = audit?.buckets || [];
  $: activeBucket = buckets.find((bucket) => bucket.key === activeBucketKey) || buckets[0];
  $: normalizedSearch = searchText.trim().toLocaleLowerCase('zh-CN');
  $: filteredItems = activeBucket?.items || [];
  $: dataListRows = filteredItems.map(toAdminRow);
  $: activePageState = pageStates[activeBucketKey];
  $: hasPreviousRows = Boolean(activePageState?.has_previous && activePageState.previous_cursor);
  $: loadPreviousKey = activePageState?.previous_cursor
    ? `${activePageState.generation}:${activePageState.previous_cursor}`
    : `${activeBucketKey}:${normalizedSearch}:start`;
  $: hasMoreRows = Boolean(activePageState?.has_more && activePageState.next_cursor);
  $: loadMoreKey = activePageState?.next_cursor
    ? `${activePageState.generation}:${activePageState.next_cursor}`
    : `${activeBucketKey}:${normalizedSearch}:complete`;
  $: dataListResetKey = `${activeBucketKey}:${normalizedSearch}`;
  $: allItems = buckets.flatMap((bucket) => bucket.items);
  $: selectedItem = allItems.find((item) => item.task_id === selectedTaskID) || selectedItemSnapshot;
  $: assigneeOptions = [
    ...(selectedItem?.assignee && !deliveryAssignees.some((option) => option.value === selectedItem?.assignee)
      ? [{ value: selectedItem.assignee, label: selectedItem.assignee }]
      : []),
    ...deliveryAssignees.map((option) => ({
      value: option.value,
      label: option.label,
      meta: option.department || ''
    }))
  ];
  $: reporterAvailable = Boolean(selectedItem?.reporter && selectedItem.reporter !== selectedItem.assignee);
  $: assigneeModeOptions = [
    ...(reporterAvailable
      ? [{ value: 'reporter', label: '指回报告人', meta: selectedItem?.reporter || '' }]
      : []),
    { value: 'specified', label: '指定负责人', meta: '从研发成员目录选择' }
  ];
  $: decisionSubmitDisabled = !decisionNote.trim()
    || (selectedDecision === 'reassign' && (
      (assigneeMode === 'reporter' && !reporterAvailable)
      || (assigneeMode === 'specified' && (!newAssignee || newAssignee === selectedItem?.assignee))
    ));
  $: decisionSubmitLabel = selectedDecision !== 'reassign'
    ? '评论并记录结论'
    : assigneeMode === 'reporter'
      ? '评论并指回报告人'
      : '评论并更新 Jira';
  $: if (!loading && audit && activeBucket && !selectedTaskID) {
    selectItem(filteredItems[0] || null);
  }

  function selectItem(item: DailyJiraItem | null) {
    selectedTaskID = item?.task_id || '';
    selectedItemSnapshot = item;
    newAssignee = item?.assignee || '';
    selectedDecision = 'follow_up';
    assigneeMode = item?.reporter && item.reporter !== item.assignee ? 'reporter' : 'specified';
    decisionNote = '';
    error = '';
  }

  function toAdminRow(item: DailyJiraItem): DailyJiraAdminRow {
    return {
      id: item.task_id,
      title: item.title,
      status: item.status,
      owner: item.assignee,
      item,
      cells: {
        timing: item.overdue,
        issue: item.title,
        project: item.project,
        assignee: item.assignee,
        status: item.status,
        lastUpdate: item.last_update
      }
    };
  }

  function selectBucket(key: DailyJiraBucketKey) {
    activeBucketKey = key;
    loadPreviousError = '';
    loadMoreError = '';
    const bucket = buckets.find((item) => item.key === key);
    selectItem(bucket?.items[0] || null);
    if (!pageStates[key] || loadedSearch !== normalizedSearch) {
      void loadAudit(false, false, false, key);
    }
  }

  function handleSearchInput() {
    loadPreviousError = '';
    loadMoreError = '';
    if (searchTimer !== undefined) window.clearTimeout(searchTimer);
    searchTimer = window.setTimeout(() => {
      searchTimer = undefined;
      pageStates = {};
      pageWindows = {};
      loadedSearch = normalizedSearch;
      if (audit) {
        audit = {
          ...audit,
          buckets: audit.buckets.map((bucket) => ({ ...bucket, items: [] }))
        };
      }
      selectItem(null);
      void loadAudit(false, false, false, activeBucketKey);
    }, 220);
  }

  function buildAuditURL(
    bucket: DailyJiraBucketKey,
    search: string,
    cursor = '',
    direction: DataWindowDirection = 'next'
  ) {
    const params = new URLSearchParams();
    params.set('bucket', bucket);
    params.set('limit', String(auditPageLimit));
    if (search) params.set('search', search);
    if (cursor) params.set('cursor', cursor);
    if (direction === 'previous') params.set('direction', direction);
    return `/api/decision/daily-jira?${params.toString()}`;
  }

  function windowPageFromAudit(
    nextAudit: DailyJiraAuditResponse,
    bucketKey: DailyJiraBucketKey
  ): DailyJiraWindowPage {
    return {
      items: nextAudit.buckets.find((bucket) => bucket.key === bucketKey)?.items || [],
      meta: nextAudit.page
    };
  }

  function windowPageState(pages: DailyJiraWindowPage[]): DailyJiraPageMeta | undefined {
    const first = pages[0]?.meta;
    const last = pages.at(-1)?.meta;
    if (!first || !last) return undefined;
    return {
      ...last,
      previous_cursor: first.previous_cursor,
      has_previous: first.has_previous,
      next_cursor: last.next_cursor,
      has_more: last.has_more
    };
  }

  function auditWithWindowItems(
    nextAudit: DailyJiraAuditResponse,
    bucketKey: DailyJiraBucketKey,
    items: DailyJiraItem[]
  ) {
    const currentBuckets = new Map((audit?.buckets || []).map((bucket) => [bucket.key, bucket]));
    const nextBuckets = nextAudit.buckets.map((bucket) => bucket.key === bucketKey
      ? { ...bucket, items }
      : { ...bucket, items: loadedSearch === normalizedSearch
          ? currentBuckets.get(bucket.key)?.items || []
          : [] });
    return { ...nextAudit, buckets: nextBuckets };
  }

  function prepareWindowUpdate(
    nextAudit: DailyJiraAuditResponse,
    bucketKey: DailyJiraBucketKey,
    direction: DataWindowDirection,
    replace = false
  ) {
    const incomingPage = windowPageFromAudit(nextAudit, bucketKey);
    const currentPages = pageWindows[bucketKey] || [];
    const pages = replace
      ? [incomingPage]
      : updateBoundedPageWindow(
          currentPages,
          incomingPage,
          direction,
          dailyJiraWindowPageLimit,
          (item) => item.task_id
        );
    const pageState = windowPageState(pages) || nextAudit.page;
    const items = flattenBoundedPages(pages);
    return {
      pages,
      pageState,
      audit: auditWithWindowItems(nextAudit, bucketKey, items)
    };
  }

  async function loadAudit(
    initial = false,
    announce = false,
    syncSource = false,
    requestedBucket: DailyJiraBucketKey = activeBucketKey
  ) {
    if (!initial && (loading || refreshing || loadingPrevious || loadingMore)) {
      refreshQueued = true;
      return;
    }
    const requestSearch = normalizedSearch;
    if (initial) loading = true;
    else refreshing = true;
    error = '';
    loadPreviousError = '';
    loadMoreError = '';

    try {
      if (syncSource) {
        const syncResponse = await fetch('/api/decision/daily-jira/sync', { method: 'POST' });
        if (!syncResponse.ok) {
          const message = (await syncResponse.text()).trim();
          throw new Error(message || `Jira 同步失败 (${syncResponse.status})`);
        }
      }
      const shouldLoadAuxiliaryResources = initial || !auxiliaryResourcesLoaded;
      const [auditResponse, [linkResponse, directory]] = await Promise.all([
        fetch(buildAuditURL(requestedBucket, requestSearch), { cache: 'no-store' }),
        shouldLoadAuxiliaryResources
          ? Promise.all([
              fetch('/api/jira/link-config', { cache: 'no-store' }).catch((linkError) => {
                console.error('Failed to fetch Jira link config for Daily Jira:', linkError);
                return null;
              }),
              fetchDeliveryDirectory().catch((directoryError) => {
                console.error('Failed to fetch shared delivery directory for Daily Jira:', directoryError);
                return null;
              })
            ])
          : Promise.resolve([null, null] as const)
      ]);
      if (!auditResponse.ok) {
        const message = (await auditResponse.text()).trim();
        throw new Error(message || `加载失败 (${auditResponse.status})`);
      }

      const nextAudit = await auditResponse.json() as DailyJiraAuditResponse;
      if (requestSearch !== normalizedSearch) return;
      lastCheckedAt = nextAudit.generated_at;
      if (directory) deliveryAssignees = directory.assignees;

      if (linkResponse?.ok) {
        const linkConfig = await linkResponse.json();
        jiraBaseUrl = String(linkConfig?.base_url || '').replace(/\/+$/, '');
      }
      if (shouldLoadAuxiliaryResources && linkResponse && directory) auxiliaryResourcesLoaded = true;

      const currentPages = loadedSearch === requestSearch
        ? pageWindows[requestedBucket] || []
        : [];
      const currentPageState = windowPageState(currentPages);
      const retainCurrentWindow = !initial
        && currentPages.length > 0
        && currentPageState?.generation === nextAudit.page.generation;
      const windowUpdate = retainCurrentWindow
        ? {
            pages: currentPages,
            pageState: currentPageState,
            audit: auditWithWindowItems(
              nextAudit,
              requestedBucket,
              flattenBoundedPages(currentPages)
            )
          }
        : prepareWindowUpdate(nextAudit, requestedBucket, 'next', true);
      const candidateAudit = windowUpdate.audit;
      const nextAuditFingerprint = dailyJiraSnapshotFingerprint(candidateAudit);
      pageWindows = { ...pageWindows, [requestedBucket]: windowUpdate.pages };
      pageStates = { ...pageStates, [requestedBucket]: windowUpdate.pageState };
      if (!audit || !retainCurrentWindow || nextAuditFingerprint !== auditFingerprint) {
        const previousSelectedTaskID = selectedTaskID;
        loadedSearch = requestSearch;
        audit = candidateAudit;
        auditFingerprint = nextAuditFingerprint;

        if (!hasInitializedBucket) {
          activeBucketKey = (['seven_day', 'three_day', 'today'] as DailyJiraBucketKey[])
            .find((key) => nextAudit.buckets.find((bucket) => bucket.key === key)?.count) || 'today';
          hasInitializedBucket = true;
        }

        const nextBucket = audit.buckets.find((bucket) => bucket.key === activeBucketKey) || audit.buckets[0];
        const preservedItem = nextBucket?.items.find((item) => item.task_id === previousSelectedTaskID);
        if (preservedItem) {
          selectedTaskID = preservedItem.task_id;
          selectedItemSnapshot = preservedItem;
          if (selectedDecision !== 'reassign') newAssignee = preservedItem.assignee;
        } else {
          selectItem(nextBucket?.items[0] || null);
        }
      }
      if (announce) {
        showToast(`${syncSource ? 'Jira 与每日列表' : '每日 Jira'}已更新，共 ${nextAudit.summary.total} 项。`, {
          type: 'success',
          title: '刷新完成'
        });
      }
    } catch (loadError) {
      console.error('Failed to load daily Jira audit:', loadError);
      error = loadError instanceof Error ? loadError.message : '每日 Jira 审计暂时不可用';
      if (announce) {
        showToast(error, {
          type: 'error',
          title: '刷新失败'
        });
      }
    } finally {
      loading = false;
      refreshing = false;
      if (refreshQueued) {
        refreshQueued = false;
        void loadAudit(false);
      } else if (hasInitializedBucket && !pageStates[activeBucketKey]) {
        void loadAudit(false, false, false, activeBucketKey);
      }
    }
  }

  function publishWindowUpdate(
    nextAudit: DailyJiraAuditResponse,
    bucketKey: DailyJiraBucketKey,
    direction: DataWindowDirection
  ) {
    const previousSelectedTaskID = selectedTaskID;
    const windowUpdate = prepareWindowUpdate(nextAudit, bucketKey, direction);
    pageWindows = { ...pageWindows, [bucketKey]: windowUpdate.pages };
    pageStates = { ...pageStates, [bucketKey]: windowUpdate.pageState };
    audit = windowUpdate.audit;
    auditFingerprint = dailyJiraSnapshotFingerprint(audit);
    lastCheckedAt = nextAudit.generated_at;
    const preservedItem = audit.buckets
      .find((bucket) => bucket.key === bucketKey)?.items
      .find((item) => item.task_id === previousSelectedTaskID);
    if (preservedItem) {
      selectedItemSnapshot = preservedItem;
      if (selectedDecision !== 'reassign') newAssignee = preservedItem.assignee;
    }
  }

  async function loadNextPage() {
    const requestedBucket = activeBucketKey;
    const requestSearch = normalizedSearch;
    const pageState = pageStates[requestedBucket];
    if (
      loading || refreshing || loadingPrevious || loadingMore
      || !pageState?.has_more || !pageState.next_cursor
    ) return;
    const requestedGeneration = pageState.generation;
    const requestedCursor = pageState.next_cursor;

    loadingMore = true;
    loadMoreError = '';
    try {
      const response = await fetch(
        buildAuditURL(requestedBucket, requestSearch, requestedCursor),
        { cache: 'no-store' }
      );
      if (response.status === 409) {
        pageStates = { ...pageStates, [requestedBucket]: undefined };
        refreshQueued = true;
        return;
      }
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(message || `加载下一页失败 (${response.status})`);
      }
      const nextAudit = await response.json() as DailyJiraAuditResponse;
      if (requestedBucket !== activeBucketKey || requestSearch !== normalizedSearch) return;
      const currentPageState = pageStates[requestedBucket];
      if (
        currentPageState?.generation !== requestedGeneration
        || currentPageState?.next_cursor !== requestedCursor
        || nextAudit.page.generation !== requestedGeneration
      ) {
        refreshQueued = true;
        return;
      }

      publishWindowUpdate(nextAudit, requestedBucket, 'next');
    } catch (loadError) {
      console.error('Failed to load next Daily Jira page:', loadError);
      loadMoreError = loadError instanceof Error ? loadError.message : '加载更多 Jira 事项时出错';
    } finally {
      loadingMore = false;
      if (refreshQueued && !loading && !refreshing && !loadingPrevious) {
        refreshQueued = false;
        void loadAudit(false);
      }
    }
  }

  async function loadPreviousPage() {
    const requestedBucket = activeBucketKey;
    const requestSearch = normalizedSearch;
    const pageState = pageStates[requestedBucket];
    if (
      loading || refreshing || loadingPrevious || loadingMore
      || !pageState?.has_previous || !pageState.previous_cursor
    ) return;
    const requestedGeneration = pageState.generation;
    const requestedCursor = pageState.previous_cursor;

    loadingPrevious = true;
    loadPreviousError = '';
    try {
      const response = await fetch(
        buildAuditURL(requestedBucket, requestSearch, requestedCursor, 'previous'),
        { cache: 'no-store' }
      );
      if (response.status === 409) {
        pageStates = { ...pageStates, [requestedBucket]: undefined };
        refreshQueued = true;
        return;
      }
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(message || `加载前一页失败 (${response.status})`);
      }
      const previousAudit = await response.json() as DailyJiraAuditResponse;
      if (requestedBucket !== activeBucketKey || requestSearch !== normalizedSearch) return;
      const currentPageState = pageStates[requestedBucket];
      if (
        currentPageState?.generation !== requestedGeneration
        || currentPageState?.previous_cursor !== requestedCursor
        || previousAudit.page.generation !== requestedGeneration
      ) {
        refreshQueued = true;
        return;
      }

      publishWindowUpdate(previousAudit, requestedBucket, 'previous');
    } catch (loadError) {
      console.error('Failed to load previous Daily Jira page:', loadError);
      loadPreviousError = loadError instanceof Error ? loadError.message : '回补上一页 Jira 事项时出错';
    } finally {
      loadingPrevious = false;
      if (refreshQueued && !loading && !refreshing && !loadingMore) {
        refreshQueued = false;
        void loadAudit(false);
      }
    }
  }

  async function submitDecision() {
    if (!selectedItem || !canWrite || submitting) return;
    error = '';

    if (!decisionNote.trim()) {
      error = '请填写决策评论后再提交';
      return;
    }

    if (selectedDecision === 'reassign') {
      if (assigneeMode === 'reporter' && !reporterAvailable) {
        error = '当前事项没有可指回的报告人';
        return;
      }
      if (assigneeMode === 'specified' && !newAssignee) {
        error = '请选择新的负责人';
        return;
      }
      if (assigneeMode === 'specified' && newAssignee === selectedItem.assignee) {
        error = '新的负责人需要与当前负责人不同';
        return;
      }
    }

    submitting = true;
    try {
      const response = await fetch('/api/decision/daily-jira/review', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          task_id: selectedItem.task_id,
          decision: selectedDecision,
          assignee_mode: selectedDecision === 'reassign' ? assigneeMode : '',
          assignee: selectedDecision === 'reassign' && assigneeMode === 'specified' ? newAssignee : '',
          note: decisionNote.trim()
        })
      });
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(message || `提交失败 (${response.status})`);
      }

      const result = (await response.json()) as { jira_sync?: string };
      const completedDecision = selectedDecision;
      const completedAssignee = assigneeMode === 'reporter' ? selectedItem.reporter : newAssignee;
      decisionNote = '';
      const jiraWasUpdated = result.jira_sync === 'completed';
      const successMessage = jiraWasUpdated
        ? completedDecision === 'reassign'
          ? `已添加 Jira 评论并转派给 ${completedAssignee}；24 小时后提醒复核。`
          : completedDecision === 'escalate'
            ? '已同步 Jira 评论并记录升级协同；4 小时后提醒复核。'
            : '已同步 Jira 评论并记录继续跟进；24 小时后提醒复核。'
        : completedDecision === 'reassign'
          ? `已记录转派给 ${completedAssignee} 的本地决策；当前未启用 Jira 同步。`
          : '已记录本地决策；当前未启用 Jira 同步。';
      await loadAudit(false);
      selectedDecision = 'follow_up';
      showToast(successMessage, {
        type: 'success',
        title: '早会决策已记录'
      });
    } catch (submitError) {
      console.error('Failed to submit daily Jira decision:', submitError);
      error = submitError instanceof Error ? submitError.message : '提交早会决策时出错';
      showToast(error, {
        type: 'error',
        title: '操作未完成'
      });
    } finally {
      submitting = false;
    }
  }

  function getJiraUrl(taskID: string) {
    return jiraBaseUrl ? `${jiraBaseUrl}/browse/${taskID}` : '';
  }

  function formatDate(value: string | null) {
    if (!value) return '未设置';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }).format(date);
  }

  function formatDateTime(value: string) {
    if (!value) return '无记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    }).format(date);
  }

  function localDayNumber(value: string | null) {
    if (!value) return null;
    const dateOnly = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
    if (dateOnly) {
      return Date.UTC(Number(dateOnly[1]), Number(dateOnly[2]) - 1, Number(dateOnly[3])) / 86_400_000;
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return null;
    return Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()) / 86_400_000;
  }

  function getTimingState(item: DailyJiraItem): JiraTimingState {
    const generatedDay = localDayNumber(audit?.generated_at || null);
    const dueDay = localDayNumber(item.due_date);
    const daysUntilDue = generatedDay !== null && dueDay !== null ? dueDay - generatedDay : null;

    let tone: JiraTimingTone = 'healthy';
    let label: JiraTimingState['label'] = '健康';
    if (item.overdue) {
      tone = 'overdue';
      label = '逾期';
    } else if (daysUntilDue !== null && daysUntilDue >= 0 && daysUntilDue <= 3) {
      tone = 'due-soon';
      label = '临期';
    }

    const ageDescription = item.age_days === 0 ? '今天创建' : `已创建 ${item.age_days} 天`;
    const dueDescription = item.due_date ? `截止 ${formatDate(item.due_date)}` : '未设置截止日期';
    return {
      tone,
      label,
      ariaLabel: `${label}；${ageDescription}；${dueDescription}`
    };
  }

  function formatStatus(status: string) {
    switch (status.toLowerCase()) {
      case 'backlog': return '待处理';
      case 'progress': return '处理中';
      case 'review': return '评审中';
      default: return status || '未知';
    }
  }

  function formatIssueType(issueType: string) {
    return issueType.toLowerCase() === 'bug' ? '缺陷' : '需求';
  }

  function formatDecisionAction(action: string) {
    if (action === 'daily_jira_reassign') return '快速转派';
    if (action === 'daily_jira_escalate') return '升级协同';
    if (action === 'daily_jira_follow_up') return '继续跟进';
    return '审计决策';
  }

  function formatDecisionStatus(status: DailyJiraDecision) {
    if (status === 'reassign') return '已转派';
    if (status === 'escalate') return '已升级';
    return '跟进中';
  }

  function reminderLabel(decision: DailyJiraDecisionState) {
    return decision.reminder_due
      ? `复核已到期 · ${formatDateTime(decision.reminder_at)}`
      : `复核于 ${formatDateTime(decision.reminder_at)}`;
  }

  function legacyDecisionLines(item: DailyJiraItem) {
    return (item.decision_logs || '')
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !line.includes('每日 Jira 审计'))
      .reverse()
      .slice(0, 6);
  }

  onMount(() => {
    void loadAudit(true);
    const unsubscribeJiraUpdates = subscribeTelemetryUpdates(
      window,
      () => loadAudit(false),
      { visibilityTarget: document }
    );
    const handleProjectPreferencesUpdated = () => {
      void loadAudit(false);
    };
    const refreshInterval = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        void loadAudit(false);
      }
    }, 30000);
    window.addEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
    return () => {
      window.clearInterval(refreshInterval);
      if (searchTimer !== undefined) window.clearTimeout(searchTimer);
      unsubscribeJiraUpdates();
      window.removeEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
    };
  });
</script>

{#snippet renderDailyJiraCell(row: DailyJiraAdminRow, column: AdminTableColumn, value: AdminCellValue)}
  {@const item = row.item}
  {#if column.key === 'timing'}
    {@const timing = getTimingState(item)}
    <span
      class="timing-dot-only timing-{timing.tone}"
      role="img"
      aria-label={timing.ariaLabel}
    ></span>
  {:else if column.key === 'issue'}
    <button
      type="button"
      class="row-select"
      aria-pressed={selectedTaskID === item.task_id}
      aria-label={`查看 ${item.task_id}：${item.title}`}
      on:click={() => selectItem(item)}
    >
      <strong>{item.task_id}</strong>
      <span>{item.title}</span>
    </button>
  {:else if column.key === 'project'}
    <span class="cell-truncate" title={item.project}>{item.project || '未映射'}</span>
  {:else if column.key === 'assignee'}
    <span class="cell-truncate" title={item.assignee}>{item.assignee || '未指派'}</span>
  {:else if column.key === 'status'}
    <span class="status-label status-{item.status}">{formatStatus(item.status)}</span>
    {#if item.latest_decision}
      <span class="row-decision decision-{item.latest_decision.status}" class:due={item.latest_decision.reminder_due}>
        {formatDecisionStatus(item.latest_decision.status)}
      </span>
    {/if}
  {:else if column.key === 'lastUpdate'}
    <span>{formatDateTime(item.last_update)}</span>
  {:else}
    <span>{String(value ?? '-')}</span>
  {/if}
{/snippet}

{#snippet renderDailyJiraEmpty()}
  <div class="table-empty">
    <strong>{normalizedSearch ? '没有匹配的 Jira 事项' : '这个时效范围没有未解决 Jira'}</strong>
    <span>{normalizedSearch ? '清除搜索词后查看当前审计范围。' : '可切换其他时效范围，或同步 Jira 获取最新结果。'}</span>
  </div>
{/snippet}

<div
  class="daily-jira"
  class:has-feedback={Boolean(error || audit?.unclassified_count)}
  aria-busy={loading || refreshing || loadingPrevious || loadingMore}
>
  {#if error || audit?.unclassified_count}
    <div class="feedback-stack" aria-live="polite">
      {#if error}
        <Alert type="error" title="操作未完成" message={error} />
      {/if}
      {#if audit?.unclassified_count}
        <Alert type="warning" message={`${audit.unclassified_count} 项 Jira 缺少有效创建时间，暂未进入时效分组。`} />
      {/if}
    </div>
  {/if}

  <div class="audit-grid">
    <div
      id="daily-jira-table-panel"
      class="audit-table-panel"
      aria-label={activeBucket?.label || 'Jira 审计事项'}
    >
      <AdminListFilterBar label="每日 Jira 列表筛选" className="daily-jira-filter-bar">
        {#snippet leading()}
          <div class="age-tabs" role="tablist" aria-label="Jira 未解决时长范围">
            {#each buckets as bucket}
              <button
                type="button"
                role="tab"
                class="age-tab bucket-{bucket.key}"
                class:active={activeBucketKey === bucket.key}
                aria-selected={activeBucketKey === bucket.key}
                aria-controls="daily-jira-table-content"
                title={bucket.description}
                on:click={() => selectBucket(bucket.key)}
              >
                <span>{bucket.label}</span>
                <strong>{bucket.count}</strong>
              </button>
            {/each}
          </div>
        {/snippet}

        {#snippet controls()}
          <div class="table-actions">
            <label class="search-field">
              <span class="sr-only">搜索 Jira 审计事项</span>
              <input type="search" bind:value={searchText} on:input={handleSearchInput} placeholder="搜索编号、标题、项目、负责人" />
            </label>
            {#if audit}
              <small class="last-checked" title={`检查于 ${formatDateTime(lastCheckedAt || audit.generated_at)}`}>
                检查于 {formatDateTime(lastCheckedAt || audit.generated_at)}
              </small>
            {/if}
            <Button variant="secondary" size="small" loading={refreshing} disabled={loadingPrevious || loadingMore} on:click={() => loadAudit(false, true, true)}>同步 Jira</Button>
          </div>
        {/snippet}
      </AdminListFilterBar>

      <AdminDataList
        columns={dailyJiraColumns}
        rows={dataListRows}
        caption={`${activeBucket?.label || '待审事项'} Jira 列表`}
        cell={renderDailyJiraCell}
        empty={renderDailyJiraEmpty}
        loading={loading}
        skeletonRows={5}
        className="daily-jira-list"
        scrollRegionId="daily-jira-table-content"
        scrollRegionRole="tabpanel"
        tableMinWidth="740px"
        compactTableMinWidth="0px"
        virtual={dailyJiraVirtualOptions}
        totalRowCount={normalizedSearch ? filteredItems.length : activeBucket?.count || filteredItems.length}
        selectedRowId={selectedTaskID}
        resetKey={dataListResetKey}
        hasPrevious={hasPreviousRows}
        {loadingPrevious}
        {loadPreviousError}
        {loadPreviousKey}
        onLoadPrevious={loadPreviousPage}
        hasMore={hasMoreRows}
        {loadingMore}
        {loadMoreError}
        {loadMoreKey}
        onLoadMore={loadNextPage}
      />
    </div>

    <aside class="wa-admin-inspector audit-inspector" aria-label="Jira 审计详情">
      {#if selectedItem}
        {@const selectedTiming = getTimingState(selectedItem)}
        <header class="inspector-header">
          <div class="inspector-meta-line">
            <span class="issue-kind">{formatIssueType(selectedItem.issue_type)}</span>
            {#if getJiraUrl(selectedItem.task_id)}
              <a
                class="jira-key jira-link"
                href={getJiraUrl(selectedItem.task_id)}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={`在 Jira 中打开 ${selectedItem.task_id}`}
              >{selectedItem.task_id}</a>
            {:else}
              <strong class="jira-key">{selectedItem.task_id}</strong>
            {/if}
            <span class="timing-chip timing-{selectedTiming.tone}">{selectedTiming.label}</span>
          </div>
          <h3>{selectedItem.title}</h3>
        </header>

        <dl class="fact-list">
          <div><dt>负责人</dt><dd>{selectedItem.assignee || '未指派'}</dd></div>
          <div><dt>报告人</dt><dd>{selectedItem.reporter || '未同步'}</dd></div>
          <div><dt>当前状态</dt><dd>{formatStatus(selectedItem.status)}</dd></div>
          <div><dt>创建时间</dt><dd>{formatDate(selectedItem.task_created_at)}</dd></div>
          <div><dt>截止日期</dt><dd class:danger={selectedItem.overdue}>{formatDate(selectedItem.due_date)}</dd></div>
          <div><dt>最近活动</dt><dd>{formatDateTime(selectedItem.last_update)}</dd></div>
          <div><dt>所属项目</dt><dd>{selectedItem.project || '未映射'}</dd></div>
        </dl>

        <section class="decision-state" class:due={selectedItem.latest_decision?.reminder_due} aria-label="当前决策与提醒">
          {#if selectedItem.latest_decision}
            <div>
              <span>当前决策</span>
              <strong>{formatDecisionStatus(selectedItem.latest_decision.status)}</strong>
              <small>{selectedItem.latest_decision.actor || '未知操作人'} · {formatDateTime(selectedItem.latest_decision.decided_at)}</small>
            </div>
            <div>
              <span>{selectedItem.latest_decision.reminder_due ? '延期提醒' : '下次复核'}</span>
              <strong>{reminderLabel(selectedItem.latest_decision)}</strong>
              <small>{selectedItem.latest_decision.note || '未填写补充说明'}</small>
            </div>
          {:else}
            <div>
              <span>当前决策</span>
              <strong>尚未记录</strong>
              <small>提交早会结论后将自动安排延期复核。</small>
            </div>
          {/if}
        </section>

        <section class="decision-form" aria-labelledby="daily-jira-decision-title">
          <div class="section-heading">
            <h4 id="daily-jira-decision-title">早会决策</h4>
            <span>{canWrite ? '写入审计记录' : '只读权限'}</span>
          </div>

          {#if canWrite}
            <Select
              id="daily-jira-decision"
              label="决策动作"
              bind:value={selectedDecision}
              options={decisionOptions}
              searchable={false}
              compact={true}
            />
            {#if selectedDecision === 'reassign'}
              <Select
                id="daily-jira-assignee-mode"
                label="负责人去向"
                bind:value={assigneeMode}
                options={assigneeModeOptions}
                searchable={false}
                compact={true}
              />
            {/if}
            {#if selectedDecision === 'reassign' && assigneeMode === 'specified'}
              <div class="assignee-field">
                <Select
                  id="daily-jira-assignee"
                  label="新的负责人"
                  bind:value={newAssignee}
                  options={assigneeOptions}
                  placeholder="搜索负责人"
                  searchPlaceholder="输入姓名"
                  emptyText="没有可用负责人"
                  compact={true}
                />
              </div>
            {/if}
            <label class="note-field" for="daily-jira-note">
              <span>决策评论 <b aria-hidden="true">必填</b></span>
              <textarea
                id="daily-jira-note"
                bind:value={decisionNote}
                maxlength="500"
                required
                aria-required="true"
                aria-describedby="daily-jira-note-help"
                placeholder="填写判断依据、补充要求或下一步"
              ></textarea>
              <small id="daily-jira-note-help">提交后将同步为该事项的 Jira 评论。</small>
            </label>
            <Button loading={submitting} disabled={decisionSubmitDisabled} on:click={submitDecision}>
              {decisionSubmitLabel}
            </Button>
          {:else}
            <p class="read-only-note">当前账号可查看审计结果，但没有分派或记录决策的权限。</p>
          {/if}
        </section>

        <section class="decision-history" aria-labelledby="daily-jira-history-title">
          <div class="section-heading">
            <h4 id="daily-jira-history-title">决策回溯</h4>
            <span>{selectedItem.decision_events.length} 条早会记录</span>
          </div>

          {#if selectedItem.decision_events.length > 0}
            <ol class="event-list">
              {#each selectedItem.decision_events as event}
                <li>
                  <div class="event-title">
                    <strong>{formatDecisionAction(event.action)}</strong>
                    <time datetime={event.created_at}>{formatDateTime(event.created_at)}</time>
                  </div>
                  <p>{event.reason || '未填写补充说明'}</p>
                  <span>{event.actor || '未知操作人'}{event.action === 'daily_jira_reassign' ? ` · ${event.old_value || '未指派'} → ${event.new_value}` : ''}</span>
                </li>
              {/each}
            </ol>
          {:else}
            <p class="history-empty">尚未记录每日 Jira 早会决策。</p>
          {/if}

          {#if legacyDecisionLines(selectedItem).length > 0}
            <details class="legacy-history">
              <summary>查看既有决策日志</summary>
              <ul>
                {#each legacyDecisionLines(selectedItem) as line}
                  <li>{line}</li>
                {/each}
              </ul>
            </details>
          {/if}
        </section>
      {:else if !loading}
        <div class="inspector-empty">
          <strong>选择一项 Jira 开始审计</strong>
          <span>左侧列表会在这里显示事实、分派动作和历史决策。</span>
        </div>
      {/if}
    </aside>
  </div>
</div>

<style>
  .daily-jira {
    --daily-radius-panel: 20px;
    --daily-radius-card: 16px;
    --daily-radius-control: 12px;
    display: grid;
    grid-template-rows: minmax(0, 1fr);
    gap: var(--wa-space-4, 16px);
    height: 100%;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    color: var(--wa-text-main, #293847);
  }

  .daily-jira.has-feedback {
    grid-template-rows: auto minmax(0, 1fr);
  }

  .feedback-stack {
    display: grid;
    gap: var(--wa-space-2, 8px);
    min-height: 0;
    overflow: hidden;
  }

  .audit-header {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-6, 24px);
    min-width: 0;
    min-height: 76px;
    padding: 14px 16px;
    overflow: visible;
    border-color: rgba(255, 255, 255, 0.74);
    border-radius: var(--daily-radius-panel);
    background:
      linear-gradient(135deg, rgba(255, 255, 255, 0.9), rgba(242, 248, 251, 0.72)),
      var(--wa-chrome-1, rgba(255, 255, 255, 0.88));
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.92),
      0 14px 34px rgba(30, 46, 64, 0.075);
  }

  .header-copy {
    min-width: 0;
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: var(--wa-space-3, 12px);
  }

  h3,
  h4,
  p {
    margin: 0;
  }

  .total-count {
    min-height: 24px;
    display: inline-flex;
    align-items: center;
    padding: 0 9px;
    border-radius: 999px;
    background: var(--wa-info-soft, rgba(37, 107, 216, 0.1));
    color: var(--wa-info, #256bd8);
    font-family: var(--wa-font-mono, monospace);
    font-size: 0.72rem;
    font-weight: 780;
    font-variant-numeric: tabular-nums;
  }

  .search-field input {
    width: min(280px, 24vw);
    min-height: var(--wa-control-h, 36px);
    padding: 0 12px 0 34px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: var(--daily-radius-control);
    background-color: var(--wa-surface-flat, #fbfdff);
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 24 24' xmlns='http://www.w3.org/2000/svg'%3E%3Ccircle cx='11' cy='11' r='6.5' fill='none' stroke='%23667789' stroke-width='2'/%3E%3Cpath d='m16 16 4 4' fill='none' stroke='%23667789' stroke-width='2' stroke-linecap='round'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: 12px 50%;
    background-size: 15px 15px;
    color: var(--wa-text-strong, #0d1722);
    font: inherit;
    font-size: 0.78rem;
    outline: none;
  }

  .search-field input::placeholder {
    color: var(--wa-text-muted, #667789);
    opacity: 1;
  }

  .search-field input:focus-visible {
    border-color: var(--wa-accent, #008f96);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.12);
  }

  .age-tabs {
    min-width: 0;
    display: inline-grid;
    grid-template-columns: repeat(3, minmax(104px, auto));
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: var(--daily-radius-control);
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .age-tab {
    min-width: 0;
    min-height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    padding: 0 10px;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
    color: var(--wa-text-main, #293847);
    cursor: pointer;
    font: inherit;
    font-size: 0.72rem;
    font-weight: 720;
    white-space: nowrap;
    transition:
      border-color var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)),
      background var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)),
      box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1));
  }

  .age-tab:hover {
    background: rgba(255, 255, 255, 0.54);
  }

  .age-tab:focus-visible {
    position: relative;
    z-index: 1;
    outline: 2px solid var(--wa-accent, #008f96);
    outline-offset: -2px;
  }

  .age-tab.active {
    border-color: var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    background: var(--wa-surface-flat, #fbfdff);
    color: var(--wa-accent-strong, #006f76);
    box-shadow: 0 2px 8px rgba(30, 46, 64, 0.08);
  }

  .age-tab strong {
    color: currentColor;
    font-family: var(--wa-font-mono, monospace);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    font-weight: 800;
  }

  .audit-grid {
    display: grid;
    grid-template-columns: minmax(580px, 1.3fr) minmax(420px, 0.7fr);
    gap: var(--wa-space-4, 16px);
    align-items: stretch;
    min-height: 0;
    height: 100%;
    overflow: hidden;
  }

  .audit-table-panel,
  .audit-inspector {
    min-width: 0;
    min-height: 0;
    margin: 0;
    box-sizing: border-box;
  }

  .audit-inspector {
    border: 1px solid rgba(255, 255, 255, 0.68);
    border-radius: var(--daily-radius-panel);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(247, 251, 253, 0.68)),
      var(--wa-chrome-1, rgba(255, 255, 255, 0.88));
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 16px 42px rgba(30, 46, 64, 0.085);
  }

  .audit-table-panel {
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-3, 12px);
    overflow: hidden;
    height: 100%;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .table-actions {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }

  .last-checked {
    flex: 0 0 auto;
    color: var(--wa-text-muted, #667789);
    font-size: 0.6875rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  :global(.daily-jira-list) {
    flex: 1 1 auto;
    min-height: 0;
  }

  .row-select {
    width: 100%;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;
  }

  .row-select:focus-visible {
    outline: 2px solid var(--wa-accent, #008f96);
    outline-offset: 2px;
  }

  .row-select strong,
  .row-select span,
  .cell-truncate {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row-select strong {
    color: var(--wa-accent-strong, #006f76);
    font-size: 0.72rem;
    font-weight: 800;
  }

  .row-select span {
    color: var(--wa-text-strong, #0d1722);
    font-weight: 680;
  }

  .status-label,
  .issue-kind {
    display: inline-flex;
    align-items: center;
    min-height: 22px;
    padding: 0 7px;
    border-radius: 999px;
    font-size: 0.66rem;
    font-weight: 760;
    white-space: nowrap;
  }

  .timing-dot-only {
    width: 11px;
    height: 11px;
    display: inline-block;
    border-radius: 999px;
    background: currentColor;
    box-shadow:
      0 0 0 2px var(--wa-surface-flat, #fbfdff),
      0 0 0 3px color-mix(in srgb, currentColor 24%, transparent);
    vertical-align: middle;
  }

  .timing-overdue {
    color: var(--wa-danger, #c6382f);
  }

  .timing-due-soon {
    color: var(--wa-warning-strong, #8a5600);
  }

  .timing-healthy {
    color: var(--wa-success, #197448);
  }

  .status-label {
    background: var(--wa-neutral-soft, rgba(102, 119, 137, 0.1));
    color: var(--wa-text-main, #293847);
  }

  .status-label.status-progress,
  .status-label.status-review {
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.1));
    color: var(--wa-accent-strong, #006f76);
  }

  .row-decision {
    display: block;
    width: max-content;
    max-width: 100%;
    margin-top: 3px;
    overflow: hidden;
    color: var(--wa-text-muted, #667789);
    font-size: 0.61rem;
    font-weight: 720;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row-decision.decision-escalate,
  .row-decision.due {
    color: var(--wa-danger, #c6382f);
  }

  .table-empty {
    min-height: 220px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    color: var(--wa-text-muted, #667789);
    text-align: center;
  }

  .table-empty strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 0.84rem;
  }

  .skeleton-block {
    display: block;
    width: 88%;
    height: 8px;
    border-radius: 4px;
    background: var(--wa-surface-inset, #e8eef3);
  }

  .skeleton-block + .skeleton-block {
    margin-top: 6px;
  }

  .skeleton-block.short { width: 56%; }
  .skeleton-block.faint { width: 70%; opacity: 0.62; }

  .audit-inspector {
    position: relative;
    height: 100%;
    display: grid;
    grid-template-rows: 92px 124px 74px minmax(210px, 1fr) 128px;
    align-content: start;
    row-gap: var(--wa-space-3, 12px);
    overflow: hidden;
    padding: 16px;
  }

  .inspector-header {
    min-height: 0;
    height: 100%;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--wa-space-4, 16px);
    padding-bottom: 14px;
    overflow: hidden;
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .inspector-header div {
    min-width: 0;
  }

  .inspector-header h3 {
    margin-top: 8px;
    display: -webkit-box;
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 1rem;
    line-height: 1.35;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    text-wrap: pretty;
  }

  .issue-kind {
    background: var(--wa-neutral-soft, rgba(102, 119, 137, 0.1));
    color: var(--wa-text-main, #293847);
  }

  .jira-key {
    flex: 0 0 auto;
    color: var(--wa-accent-strong, #006f76);
    font-size: 0.72rem;
    font-weight: 780;
    text-decoration: none;
    white-space: nowrap;
  }

  .fact-list {
    min-height: 0;
    height: 100%;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0 var(--wa-space-4, 16px);
    margin: 0;
    padding: 6px 0 14px;
    overflow: hidden;
  }

  .fact-list div {
    min-width: 0;
    display: grid;
    gap: 3px;
    padding: 10px 0 8px;
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.14));
  }

  .fact-list dt {
    color: var(--wa-text-muted, #667789);
    font-size: 0.66rem;
  }

  .fact-list dd {
    min-width: 0;
    margin: 0;
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 0.75rem;
    font-weight: 680;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .fact-list dd.danger {
    color: var(--wa-danger, #c6382f);
  }

  .decision-state {
    min-height: 0;
    height: 100%;
    box-sizing: border-box;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-4, 16px);
    align-content: center;
    padding: 10px 0;
    border: 0;
    border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: 0;
    background: transparent;
  }

  .decision-state.due {
    border-color: rgba(198, 56, 47, 0.24);
    background: var(--wa-danger-soft, rgba(221, 75, 62, 0.08));
  }

  .decision-state div {
    min-width: 0;
    display: grid;
    align-content: center;
    gap: 3px;
  }

  .decision-state div:only-child {
    grid-column: 1 / -1;
  }

  .decision-state span,
  .decision-state small {
    overflow: hidden;
    color: var(--wa-text-muted, #667789);
    font-size: 0.64rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .decision-state strong {
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 0.74rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .decision-state.due strong {
    color: var(--wa-danger, #c6382f);
  }

  .decision-form,
  .decision-history {
    min-height: 0;
    height: 100%;
    box-sizing: border-box;
    display: grid;
    gap: var(--wa-space-2, 8px);
    padding: 10px 0;
  }

  .decision-history {
    border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    overflow: hidden;
  }

  .decision-form {
    align-content: start;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    overflow: visible;
  }

  .decision-form > .section-heading,
  .decision-form > .note-field,
  .decision-form > .read-only-note,
  .decision-form :global(.btn) {
    grid-column: 1 / -1;
  }

  .decision-form > :global(.select-group) {
    min-width: 0;
  }

  .decision-form > .assignee-field {
    grid-column: 1 / -1;
    min-width: 0;
  }

  .section-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 10px;
  }

  .section-heading h4 {
    color: var(--wa-text-strong, #0d1722);
    font-size: 0.8125rem;
    font-weight: 800;
  }

  .section-heading span {
    color: var(--wa-text-muted, #667789);
    font-size: 0.66rem;
  }

  .note-field {
    display: grid;
    gap: 6px;
    color: var(--wa-text-main, #293847);
    font-size: 0.72rem;
    font-weight: 700;
  }

  .note-field span {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }

  .note-field b {
    color: var(--wa-danger, #c6382f);
    font-size: 0.66rem;
    font-weight: 750;
  }

  .note-field small {
    color: var(--wa-text-muted, #667789);
    font-size: 0.66rem;
    font-weight: 500;
    line-height: 1.4;
  }

  .note-field textarea {
    height: 68px;
    min-height: 68px;
    resize: none;
    padding: 9px 10px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.22));
    border-radius: var(--daily-radius-control);
    background: var(--wa-surface-flat, #fbfdff);
    color: var(--wa-text-strong, #0d1722);
    font: inherit;
    font-weight: 500;
    line-height: 1.45;
    outline: none;
  }

  .note-field textarea::placeholder {
    color: var(--wa-text-muted, #667789);
    opacity: 1;
  }

  .note-field textarea:focus-visible {
    border-color: var(--wa-accent, #008f96);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.12);
  }

  .read-only-note,
  .history-empty,
  .inspector-empty {
    color: var(--wa-text-muted, #667789);
    font-size: 0.74rem;
    line-height: 1.5;
  }

  .event-list {
    display: grid;
    gap: 0;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .event-list li {
    display: grid;
    gap: 4px;
    padding: 9px 0;
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.14));
  }

  .event-list li:nth-child(n + 2) {
    display: none;
  }

  .event-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .event-title strong {
    color: var(--wa-accent-strong, #006f76);
    font-size: 0.72rem;
  }

  .event-title time,
  .event-list li > span {
    color: var(--wa-text-muted, #667789);
    font-size: 0.64rem;
  }

  .event-list p {
    color: var(--wa-text-main, #293847);
    font-size: 0.72rem;
    line-height: 1.45;
    text-wrap: pretty;
  }

  .legacy-history {
    color: var(--wa-text-main, #293847);
    font-size: 0.7rem;
  }

  .legacy-history summary {
    cursor: pointer;
    color: var(--wa-accent-strong, #006f76);
    font-weight: 720;
  }

  .legacy-history ul {
    display: grid;
    gap: 7px;
    margin: 8px 0 0;
    padding-left: 18px;
    color: var(--wa-text-muted, #667789);
    line-height: 1.45;
  }

  .inspector-empty {
    grid-row: 1 / -1;
    min-height: 260px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    text-align: center;
  }

  .inspector-empty strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 0.84rem;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (max-width: 1180px) {
    .last-checked {
      display: none;
    }

    .audit-grid {
      grid-template-columns: 1fr;
      grid-template-rows: minmax(0, 0.9fr) minmax(0, 1.1fr);
      gap: var(--wa-space-4, 16px);
      height: 100%;
    }

    .audit-table-panel {
      height: 100%;
      min-height: 0;
    }

    .audit-inspector {
      position: static;
      height: 100%;
      max-height: none;
      grid-template-columns: minmax(200px, 0.9fr) minmax(250px, 1.2fr) minmax(190px, 0.9fr);
      grid-template-rows: 92px minmax(0, 1fr);
      grid-template-areas:
        "header state state"
        "facts form history";
      column-gap: 14px;
      overflow: hidden;
    }

    .inspector-header {
      grid-area: header;
      padding-right: 12px;
    }

    .fact-list {
      grid-area: facts;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      padding-right: 12px;
    }

    .decision-state {
      grid-area: state;
      padding-top: 0;
      padding-bottom: 14px;
      border-top: 0;
    }

    .decision-form {
      grid-area: form;
      padding-inline: 2px 14px;
    }

    .decision-history {
      grid-area: history;
      padding-left: 14px;
      border-top: 0;
      border-left: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    }
  }

  @media (max-width: 800px) {
    .table-actions :global(button),
    .search-field input,
    .age-tab {
      min-height: var(--wa-touch-h, 44px);
    }
  }

  @media (max-width: 760px) {
    .daily-jira {
      --daily-radius-panel: 16px;
      --daily-radius-card: 14px;
      --daily-radius-control: 10px;
      gap: var(--wa-space-3, 12px);
    }

    .audit-header {
      align-items: stretch;
      flex-direction: column;
      gap: var(--wa-space-3, 12px);
      min-height: 0;
      padding: 14px;
    }

    .table-actions,
    .search-field,
    .search-field input {
      width: 100%;
    }

    .table-actions {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
    }

    .table-actions :global(button) {
      min-height: var(--wa-touch-h, 44px);
    }

    .search-field input {
      min-height: var(--wa-touch-h, 44px);
    }

    .age-tabs {
      width: 100%;
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .age-tab {
      min-height: var(--wa-touch-h, 44px);
      padding-inline: 8px;
    }

    .fact-list {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .audit-inspector {
      padding: 14px;
    }

    .decision-form,
    .decision-history {
      padding-block: 12px;
    }

    .decision-form :global(.select-trigger) {
      min-height: var(--wa-touch-h, 44px);
    }
  }

  @media (max-width: 520px) {
    .audit-grid {
      grid-template-rows: minmax(0, 0.5fr) minmax(0, 1.5fr);
    }

    .fact-list {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .decision-state {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--wa-space-2, 8px);
    }

    .inspector-header {
      align-items: stretch;
      flex-direction: column;
    }

    .audit-inspector {
      grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
      grid-template-rows: 104px 148px minmax(0, 1fr);
      grid-template-areas:
        "header state"
        "facts history"
        "form form";
      column-gap: 10px;
    }

    .inspector-header {
      gap: 6px;
      padding-bottom: 8px;
      padding-right: 8px;
    }

    .inspector-header h3 {
      margin-top: 4px;
    }

    .fact-list {
      padding: 4px 8px 8px 0;
    }

    .fact-list div {
      padding-block: 6px 5px;
    }

    .decision-form {
      gap: 4px;
      padding: 2px 0 0;
      border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    }

    .decision-form .note-field {
      gap: 4px;
    }

    .decision-form .note-field textarea {
      height: 44px;
      min-height: 44px;
      padding-block: 6px;
    }

    .decision-history {
      padding: 6px 0 0 10px;
    }
  }

  /* L0 page gaps remain visible; only the two peer work areas own L1 surfaces. */
  .daily-jira {
    --daily-radius-panel: var(--wa-radius-lg, 14px);
    --daily-radius-card: 0;
    --daily-radius-control: var(--wa-radius-pill, 999px);
    gap: 12px;
    background: transparent;
  }

  .audit-grid {
    grid-template-columns: minmax(0, 1fr) minmax(360px, 420px);
    gap: 12px;
  }

  .audit-inspector {
    border: 1px solid var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    border-radius: var(--daily-radius-panel);
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.76));
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
  }

  .age-tabs {
    border-radius: var(--wa-radius-pill, 999px);
    background: rgba(255, 255, 255, 0.38);
  }

  .age-tab {
    border-radius: var(--wa-radius-pill, 999px);
  }

  .age-tab.active {
    background: rgba(255, 255, 255, 0.82);
  }

  .search-field input,
  .table-actions :global(button) {
    border-radius: var(--wa-radius-pill, 999px);
  }

  .audit-table-panel {
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .total-count,
  .age-tab,
  .age-tab strong,
  .last-checked,
  .row-select strong,
  .status-label,
  .issue-kind,
  .row-decision,
  .inspector-header a,
  .jira-key,
  .fact-list dt,
  .decision-state span,
  .decision-state small,
  .section-heading span,
  .note-field,
  .event-title strong,
  .event-title time,
  .event-list li > span,
  .legacy-history {
    font-size: 12px;
  }

  .search-field input,
  .row-select span,
  .fact-list dd,
  .decision-state strong,
  .event-list p,
  .read-only-note,
  .history-empty,
  .inspector-empty {
    font-size: 13px;
  }

  .section-heading h4 {
    font-size: 14px;
  }

  @media (max-width: 360px) {
    .age-tab {
      gap: 4px;
      padding-inline: 5px;
    }
  }

  .audit-inspector {
    height: 100%;
    grid-template-rows: max-content max-content max-content minmax(210px, 1fr) minmax(96px, max-content);
    row-gap: 0;
    align-content: start;
    overflow: hidden;
    padding: 18px;
    scrollbar-gutter: auto;
  }

  /* Final Daily Jira inspector header contract:
     the linked Jira key stays within metadata; the primary title owns the full second row. */
  .inspector-header {
    min-height: 92px;
    height: auto;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas:
      "meta"
      "title";
    align-content: start;
    align-items: flex-start;
    justify-content: stretch;
    gap: 8px 16px;
    overflow: visible;
    padding-bottom: 16px;
  }

  .inspector-meta-line {
    grid-area: meta;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 7px;
    flex-wrap: nowrap;
    overflow: hidden;
  }

  .inspector-header h3 {
    grid-area: title;
    width: 100%;
    margin-top: 0;
    display: block;
    overflow: visible;
    color: var(--wa-text-strong, #0d1722);
    font-size: 17px;
    line-height: 1.38;
    letter-spacing: -0.01em;
    line-clamp: unset;
    -webkit-box-orient: initial;
    -webkit-line-clamp: unset;
    overflow-wrap: anywhere;
  }

  .issue-kind,
  .jira-key,
  .timing-chip {
    flex: 0 0 auto;
    min-height: 24px;
    display: inline-flex;
    align-items: center;
    padding: 0 8px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-pill, 999px);
    font-size: 11px;
    line-height: 1;
    white-space: nowrap;
  }

  .jira-key {
    border-color: rgba(37, 107, 216, 0.2);
    background: var(--wa-info-soft);
    color: var(--wa-info);
    font-family: var(--wa-font-mono, monospace);
  }

  .jira-link {
    position: relative;
    cursor: pointer;
    text-decoration: none;
  }

  .jira-link::after {
    content: '';
    position: absolute;
    inset-block: -10px;
    inset-inline: 0;
  }

  .jira-link:hover {
    border-color: rgba(37, 107, 216, 0.46);
    background: rgba(37, 107, 216, 0.13);
    text-decoration: none;
  }

  .jira-link:focus-visible {
    outline: 2px solid var(--wa-accent, #008f96);
    outline-offset: 2px;
  }

  .jira-link:active {
    transform: translateY(1px);
  }

  .timing-chip.timing-overdue {
    border-color: rgba(221, 75, 62, 0.2);
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .timing-chip.timing-due-soon {
    border-color: rgba(216, 135, 0, 0.22);
    background: var(--wa-warning-soft);
    color: var(--wa-warning);
  }

  .timing-chip.timing-healthy {
    border-color: rgba(4, 150, 111, 0.2);
    background: var(--wa-success-soft);
    color: var(--wa-success);
  }

  .fact-list {
    height: auto;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 18px;
    padding: 12px 0 14px;
    overflow: visible;
  }

  .fact-list div {
    padding: 8px 0;
  }

  .decision-state {
    min-height: 72px;
    height: auto;
    padding: 12px 0;
  }

  .decision-form,
  .decision-history {
    height: auto;
    padding-block: 14px;
    overflow: visible;
  }

  .decision-form {
    border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .note-field textarea {
    height: 68px;
    min-height: 68px;
    border-radius: var(--wa-radius-md, 8px);
  }

  /* The explicit reassignment flow is the inspector's tallest state. Keep the
     desktop panel fixed and non-scrolling by using its width more efficiently. */
  @media (min-width: 1181px) {
    .audit-inspector {
      grid-template-rows: max-content max-content max-content max-content minmax(76px, max-content);
    }

    .audit-inspector:has(.assignee-field) .fact-list {
      grid-template-columns: repeat(3, minmax(0, 1fr));
      padding-block: 8px 10px;
    }

    .audit-inspector:has(.assignee-field) .fact-list div {
      padding-block: 6px 5px;
    }

    .audit-inspector .decision-form {
      gap: 6px;
      padding-block: 8px;
    }

    .audit-inspector .note-field textarea {
      height: 52px;
      min-height: 52px;
    }

    .audit-inspector .decision-history {
      padding-block: 10px;
    }
  }

  @media (max-width: 1180px) {
    .daily-jira {
      height: auto;
      overflow: visible;
    }

    .audit-grid {
      height: auto;
      grid-template-columns: 1fr;
      grid-template-rows: none;
      gap: 12px;
      overflow: visible;
    }

    .audit-table-panel {
      height: clamp(420px, 62svh, 560px);
      min-height: 420px;
      padding: 0;
    }

    .audit-inspector {
      height: auto;
      grid-template-columns: 1fr;
      grid-template-rows: none;
      grid-template-areas:
        "header"
        "facts"
        "state"
        "form"
        "history";
      overflow: visible;
      overscroll-behavior: auto;
      padding: 14px;
    }

    .inspector-header,
    .fact-list,
    .decision-state,
    .decision-form,
    .decision-history {
      height: auto;
      overflow: visible;
    }
  }

  @media (max-width: 860px) {
    .event-list li:nth-child(n + 2) {
      display: grid;
    }
  }

  @media (max-width: 520px) {
    .audit-grid,
    .audit-inspector {
      grid-template-rows: none;
    }

    .audit-inspector {
      grid-template-columns: 1fr;
      grid-template-areas:
        "header"
        "facts"
        "state"
        "form"
        "history";
    }

    .decision-form {
      grid-template-columns: minmax(0, 1fr);
    }

    .inspector-header {
      grid-template-columns: minmax(0, 1fr);
      grid-template-areas:
        "meta"
        "title";
    }

    .inspector-meta-line {
      min-height: 44px;
    }

    .fact-list {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .decision-history {
      padding-left: 0;
      border-left: 0;
    }
  }

  /* Final Daily Jira page-height contract: the desktop route and both peer
     panels stay fixed; only the left table shell owns vertical scrolling. */
  .daily-jira {
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .audit-grid {
    height: 100%;
    min-height: 0;
  }

  @media (min-width: 1181px) and (max-height: 1000px) {
    .audit-inspector {
      padding: 14px;
    }

    .inspector-header {
      min-height: 0;
      gap: 6px;
      padding-bottom: 8px;
    }

    .inspector-header h3 {
      font-size: 16px;
      line-height: 1.28;
    }

    .fact-list {
      gap: 0 12px;
      padding: 6px 0 8px;
    }

    .audit-inspector:has(.assignee-field) .fact-list {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .fact-list div {
      padding: 4px 0;
    }

    .decision-state {
      min-height: 0;
      padding: 8px 0;
    }

    .audit-inspector .decision-form {
      gap: 4px;
      padding: 6px 0;
    }

    .audit-inspector .decision-form :global(.select-group),
    .audit-inspector .note-field {
      gap: 4px;
    }

    .audit-inspector .note-field textarea {
      height: 44px;
      min-height: 44px;
      padding-block: 6px;
    }

    .audit-inspector .decision-history {
      padding: 6px 0;
    }

    .audit-grid {
      height: 100%;
      min-height: 0;
    }
  }

  @media (max-width: 1180px) {
    .daily-jira {
      height: 100%;
      min-height: 0;
      overflow-x: hidden;
      overflow-y: auto;
    }

    .audit-grid {
      height: max-content;
      min-height: max-content;
      align-items: start;
    }

    .audit-inspector {
      align-self: start;
    }
  }

</style>
