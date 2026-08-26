<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import Modal from './shared/Modal.svelte';
  import IssueTypeMark from './shared/IssueTypeMark.svelte';
  import Select from './shared/Select.svelte';
  import MultiSelect from './shared/MultiSelect.svelte';
  import CommitTelemetryPanel from './CommitTelemetryPanel.svelte';
  import { showToast } from '../lib/toast';
  import { subscribeTelemetryUpdates } from '../lib/telemetry-refresh';
  import {
    fetchDeliveryDirectory,
    type DeliveryAssigneeOption,
    type DeliveryProjectOption
  } from '../lib/delivery-directory';
  import {
    ADMIN_TONE_CLASS,
    formatAdminDate,
    toneForRisk,
    toneForStatus,
    type AdminInspectorAction,
    type AdminInspectorRecord,
    type AdminMetric,
    type AdminTableColumn,
    type AdminTableRow,
    type AdminTone
  } from '../lib/admin-console/contract';

  interface Task {
    id: string;
    title: string;
    repo: string;
    assignee: string;
    branch: string;
    lastCommit: string;
    lastUpdate: string;
    status: string;
    rawLastUpdate: string;
    issueType: string;
    taskCreatedAt: string;
    mrIid?: number;
    mrUrl?: string;
    taskGroupId?: string;
    parentWorkItemId?: string;
    parentWorkItem?: string;
    projectKey?: string;
    targetRelease?: string;
    affectedReleases?: string[];
    description?: string;
    source?: string;
    planningState?: string;
    dueDate?: string;
    syncState?: string;
    revision?: number;
  }

  interface TaskResponse {
    task_id: string;
    title: string;
    repo: string;
    assignee: string;
    branch: string;
    last_commit: string;
    status: string;
    last_update: string;
    issue_type: string;
    task_created_at: string;
    mr_iid?: number;
    mr_url?: string;
    task_group_id?: string;
  }

  type TaskView = 'status' | 'execution';
  type ExecutionRiskFilter = 'attention' | 'all' | 'high' | 'medium' | 'safe' | 'done';

  interface ExecutionSummary {
    total: number;
    active: number;
    done: number;
    bound: number;
    orphan: number;
    with_evidence: number;
    missing_evidence: number;
    stale: number;
    mismatch: number;
    high_risk: number;
  }

  interface ExecutionTaskItem {
    task_id: string;
    title: string;
    issue_type: string;
    source?: string;
    assignee: string;
    execution_assignee?: string;
    jira_assignee?: string;
    jira_status?: string;
    department: string;
    repo: string;
    branch: string;
    status: string;
    task_group_id: string;
    parent_demand_id?: string;
    parent_demand?: string;
    parent_work_item_id?: string;
    parent_work_item?: string;
    parent_issue_type?: string;
    project_key?: string;
    target_release_id?: number;
    target_release?: string;
    created_at: string;
    last_update: string;
    last_evidence_at?: string;
    last_commit: string;
    mr_url?: string;
    mr_iid?: number;
    commit_count: number;
    mr_count: number;
    merged_mr_count: number;
    evidence_score: number;
    risk_level: string;
    risk_label: string;
    risk_reason: string;
    risk_rank: number;
    result_state: string;
    result_label: string;
    active_days: number;
    evidence_age_hours: number;
    risk_tags: string[];
  }

  interface ExecutionTasksResponse {
    generated_at: string;
    summary: ExecutionSummary;
    facets?: {
      projects?: Array<{ project_key: string; project_name: string }>;
      assignees?: Array<{ value: string; label: string; department?: string }>;
    };
    items: ExecutionTaskItem[];
  }

  interface WorkItemRecord {
    task_id: string;
    project_key?: string;
    source?: string;
    external_key?: string;
    revision?: number;
    planning_state?: string;
    title?: string;
    description?: string;
    repo?: string;
    assignee?: string;
    branch?: string;
    last_commit?: string;
    status?: string;
    issue_type?: string;
    task_created_at?: string;
    last_update?: string;
    completed_at?: string;
    due_date?: string;
    mr_iid?: number;
    mr_url?: string;
    task_group_id?: string;
  }

  interface WorkItemSnapshot {
    work_item: WorkItemRecord;
    target_releases?: Array<{ id: number; project_key: string; name: string; status: string }>;
    affected_releases?: Array<{ id: number; project_key: string; name: string; status: string }>;
    sync_state?: string;
  }

  interface WorkItemsResponse {
    items: WorkItemSnapshot[];
    total: number;
    limit: number;
    offset: number;
    summary?: WorkItemSummary;
  }

  interface WorkItemSummary {
    total: number;
    active: number;
    done: number;
    backlog: number;
    progress: number;
    review: number;
    requirements: number;
    bugs: number;
    active_requirements: number;
    active_bugs: number;
    planned: number;
    unplanned: number;
  }

  interface TaskSummaryMetric extends AdminMetric {
    surface?: 'accent';
  }

  interface WorkItemCompletionResponse {
    snapshot: WorkItemSnapshot;
    evidence: {
      commit_count: number;
      merged_mr_count: number;
      repositories: string[];
      latest_at?: string;
    };
  }

  const executionRiskFilters: Array<{ value: ExecutionRiskFilter; label: string }> = [
    { value: 'attention', label: '需关注' },
    { value: 'all', label: '全部' },
    { value: 'high', label: '高风险' },
    { value: 'medium', label: '中风险' },
    { value: 'safe', label: '正常' },
    { value: 'done', label: '已闭环' }
  ];

  const taskTableColumns: AdminTableColumn[] = [
    { key: 'task', label: '事项编号 / 标题', width: '30%' },
    { key: 'type', label: '类型', width: '8%' },
    { key: 'project', label: '项目', width: '12%' },
    { key: 'owner', label: '负责人', width: '12%' },
    { key: 'status', label: '状态', width: '11%' },
    { key: 'release', label: '目标版本', width: '13%' },
    { key: 'lastUpdate', label: '最后同步', width: '10%' },
    { key: 'actions', label: '操作', width: '4%', align: 'right' }
  ];

  const executionTableColumns: AdminTableColumn[] = [
    { key: 'task', label: '执行任务 / 标题', width: '32%' },
    { key: 'owner', label: '负责人', width: '12%' },
    { key: 'result', label: '结果状态', width: '14%' },
    { key: 'evidence', label: '代码证据', width: '16%' },
    { key: 'risk', label: '风险', width: '12%' },
    { key: 'lastEvidence', label: '最新证据', width: '10%' },
    { key: 'actions', label: '操作', width: '4%', align: 'right' }
  ];

  function emptyExecutionSummary(): ExecutionSummary {
    return {
      total: 0,
      active: 0,
      done: 0,
      bound: 0,
      orphan: 0,
      with_evidence: 0,
      missing_evidence: 0,
      stale: 0,
      mismatch: 0,
      high_risk: 0
    };
  }

  function emptyWorkItemSummary(): WorkItemSummary {
    return {
      total: 0,
      active: 0,
      done: 0,
      backlog: 0,
      progress: 0,
      review: 0,
      requirements: 0,
      bugs: 0,
      active_requirements: 0,
      active_bugs: 0,
      planned: 0,
      unplanned: 0
    };
  }

  let workItemTasks: Task[] = [];
  let selectedSearchTask: Task | null = null;
  $: allTasks = currentView === 'status'
    ? (selectedSearchTask && !workItemTasks.some(task => task.id === selectedSearchTask?.id)
        ? [selectedSearchTask, ...workItemTasks]
        : workItemTasks)
    : executionItems.map(mapExecutionTask);
  let parentWorkItemsByGroup = new Map<string, { id: string; title: string }>();
  let selectedProjects: string[] = [];
  let selectedAssignees: string[] = [];
  // Retained for the disabled legacy board markup below; active Phase 41 views use the arrays above.
  let selectedProject = 'all';
  let selectedAssignee = 'all';
  export let activeTaskView: TaskView = 'status';
  export let onTaskViewChange: (view: TaskView) => void = () => {};
  export let onOpenDeliveryPlan: (workItemID: string) => void = () => {};
  let currentView: TaskView = normalizeTaskView(activeTaskView);
  $: if (normalizeTaskView(activeTaskView) !== currentView) setTaskView(activeTaskView);
  let executionItems: ExecutionTaskItem[] = [];
  let executionSummary: ExecutionSummary = emptyExecutionSummary();
  let executionGeneratedAt = '';
  let executionLoading = false;
  let executionDataLoaded = false;
  let executionErrorMsg = '';
  let executionSearch = '';
  let executionSearchInput = '';
  let executionSearchDebounceTimer: any;
  function handleExecutionSearch(event: Event) {
    const inputVal = (event.target as HTMLInputElement).value;
    executionSearchInput = inputVal;
    clearTimeout(executionSearchDebounceTimer);
    executionSearchDebounceTimer = setTimeout(() => {
      executionSearch = inputVal;
    }, 200);
  }
  let executionRiskFilter: ExecutionRiskFilter = 'all';
  let taskAdminMetrics: TaskSummaryMetric[] = [];
  let executionAdminMetrics: TaskSummaryMetric[] = [];

  let collapsedAssignees: Record<string, boolean> = {};
  let userToggledAssignees: Record<string, boolean> = {};

  let intervalId: any;
  let taskRequestSequence = 0;
  let taskRequestController: AbortController | null = null;
  let taskScopeRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  let executionRequestSequence = 0;
  let loading = true;
  let taskRefreshing = false;
  let taskDataLoaded = false;
  let workItemSummary = emptyWorkItemSummary();
  let taskLastRefreshedAt = '';
  let errorMsg = '';

  function toggleAssigneeCollapse(assignee: string) {
    const isCurrentlyCollapsed = collapsedAssignees[assignee] !== false;
    
    // 锁定用户手动干预意图
    userToggledAssignees[assignee] = true;
    
    if (isCurrentlyCollapsed) {
      // 互斥独占式展开，防止高度挤占
      viewAssignees.forEach(ass => {
        if (ass !== assignee) {
          collapsedAssignees[ass] = true;
          userToggledAssignees[ass] = true; // 对其他人锁定收起
        }
      });
      collapsedAssignees[assignee] = false;
    } else {
      collapsedAssignees[assignee] = true;
    }
    collapsedAssignees = { ...collapsedAssignees };
  }

  function getAssigneeStatusCount(assignee: string, status: string, tasks: Task[]): number {
    return (assigneeTasksMap.get(assignee) || []).filter(t => t.status.toLowerCase() === status.toLowerCase()).length;
  }

  function getAssigneeTaskCount(assignee: string, tasks: Task[]): number {
    return (assigneeTasksMap.get(assignee) || []).length;
  }

  function getAssigneeBugCount(assignee: string, tasks: Task[]): number {
    return (assigneeTasksMap.get(assignee) || []).filter(t => t.issueType === 'bug').length;
  }

  function isAssigneeHighLoad(assignee: string, tasks: Task[]): boolean {
    return (assigneeTasksMap.get(assignee) || []).filter(t => t.status.toLowerCase() !== 'done').length >= 5;
  }

  function isAssigneeDelayed(assignee: string, tasks: Task[]): boolean {
    return (assigneeTasksMap.get(assignee) || []).filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 3).length > 0;
  }

  let showProjectDropdown = false;
  let showAssigneeDropdown = false;
  let projectSelectEl: HTMLElement;
  let assigneeSelectEl: HTMLElement;

  let projectSearchText = '';
  let assigneeSearchText = '';

  $: if (!showProjectDropdown) projectSearchText = '';
  $: if (!showAssigneeDropdown) assigneeSearchText = '';

  function toggleProjectDropdown() {
    showProjectDropdown = !showProjectDropdown;
    if (showProjectDropdown) showAssigneeDropdown = false;
  }

  function toggleAssigneeDropdown() {
    showAssigneeDropdown = !showAssigneeDropdown;
    if (showAssigneeDropdown) showProjectDropdown = false;
  }

  function selectProject(proj: string) {
    selectedProject = proj;
    showProjectDropdown = false;
  }

  function selectAssignee(ass: string) {
    selectedAssignee = ass;
    showAssigneeDropdown = false;
  }

  function handleDocumentClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (showProjectDropdown && projectSelectEl && !projectSelectEl.contains(target)) {
      showProjectDropdown = false;
    }
    if (showAssigneeDropdown && assigneeSelectEl && !assigneeSelectEl.contains(target)) {
      showAssigneeDropdown = false;
    }
  }

  // Modal details state
  let selectedTask: Task | null = null;
  let showDetails = false;
  let completionConfirming = false;
  let completionSubmitting = false;
  let completionError = '';
  let selectedTaskId = '';
  let selectedExecutionTaskId = '';
  let taskTableShellEl: HTMLElement | null = null;
  let pendingRevealTaskId = '';
  let revealScheduledTaskId = '';
  let globalSearchSelectedTaskId = '';

  function handleGlobalSearchSelection(event: Event) {
    const detail = (event as CustomEvent<{ id?: string; route?: string }>).detail;
    if (detail?.route !== 'tasks' || !detail.id) return;
    const scopeChanged = selectedProjects.length > 0 || selectedAssignees.length > 0;
    selectedProjects = [];
    selectedAssignees = [];
    selectedProject = 'all';
    selectedAssignee = 'all';
    selectedTaskId = detail.id;
    globalSearchSelectedTaskId = detail.id;
    pendingRevealTaskId = detail.id;
    if (scopeChanged) void fetchTasks();
    void loadSelectedSearchTask(detail.id);
  }

  function handleGlobalSearchClear() {
    const taskId = globalSearchSelectedTaskId;
    if (!taskId) return;
    if (selectedTaskId === taskId) selectedTaskId = '';
    if (selectedSearchTask?.id === taskId) selectedSearchTask = null;
    if (pendingRevealTaskId === taskId) pendingRevealTaskId = '';
    if (revealScheduledTaskId === taskId) revealScheduledTaskId = '';
    globalSearchSelectedTaskId = '';
  }

  async function loadSelectedSearchTask(taskId: string) {
    const existing = allTasks.find(task => task.id === taskId);
    if (existing) {
      selectedSearchTask = workItemTasks.some(task => task.id === taskId) ? null : existing;
      return;
    }
    try {
      const response = await fetch(`/api/work-items/${encodeURIComponent(taskId)}`, { cache: 'no-store' });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const snapshot: WorkItemSnapshot = await response.json();
      if (selectedTaskId !== taskId) return;
      selectedSearchTask = mapWorkItem(snapshot);
    } catch (error) {
      if (selectedTaskId !== taskId) return;
      console.error('Failed to load selected work item:', error);
      showToast('已找到事项，但加载详情失败，请重试。', {
        type: 'error',
        title: '事项加载失败'
      });
      pendingRevealTaskId = '';
    }
  }

  async function revealPendingTaskRow(taskId: string) {
    revealScheduledTaskId = taskId;
    await tick();

    if (pendingRevealTaskId !== taskId || !taskTableShellEl) {
      revealScheduledTaskId = '';
      return;
    }

    const row = Array.from(taskTableShellEl.querySelectorAll<HTMLTableRowElement>('tbody tr[data-task-id]'))
      .find(candidate => candidate.dataset.taskId === taskId);

    if (!row) {
      revealScheduledTaskId = '';
      return;
    }

    const rowRect = row.getBoundingClientRect();
    const tableHead = taskTableShellEl.querySelector('thead');
    const tableOwnsVerticalScroll = taskTableShellEl.scrollHeight > taskTableShellEl.clientHeight + 1;
    const scrollOwner = tableOwnsVerticalScroll
      ? taskTableShellEl
      : taskTableShellEl.closest<HTMLElement>('.workspace-frame');

    if (!scrollOwner) {
      pendingRevealTaskId = '';
      revealScheduledTaskId = '';
      return;
    }

    const ownerRect = scrollOwner.getBoundingClientRect();
    const stickyHeadHeight = tableOwnsVerticalScroll ? (tableHead?.getBoundingClientRect().height || 0) : 0;
    const visibleTop = ownerRect.top + stickyHeadHeight;
    const visibleBottom = ownerRect.bottom;

    if (rowRect.top < visibleTop || rowRect.bottom > visibleBottom) {
      const visibleHeight = Math.max(0, scrollOwner.clientHeight - stickyHeadHeight);
      const centeredOffset = Math.max(0, (visibleHeight - rowRect.height) / 2);
      scrollOwner.scrollTop = Math.max(
        0,
        scrollOwner.scrollTop + rowRect.top - visibleTop - centeredOffset
      );
    }

    pendingRevealTaskId = '';
    revealScheduledTaskId = '';
  }

  let deliveryProjects: DeliveryProjectOption[] = [];
  let allProjects: string[] = [];
  $: allProjects = deliveryProjects.map(project => project.project_key);
  $: projectOptions = ['all', ...allProjects];
  let projectNamesMap: Record<string, string> = {};
  let activeAssigneeFacets: DeliveryAssigneeOption[] = [];
  $: assigneeOptions = ['all', ...activeAssigneeFacets.map(option => option.value)];
  $: projectMultiOptions = allProjects.map(project => ({
    value: project,
    label: projectNamesMap[project.toUpperCase()] || project,
    meta: project
  }));
  $: ownerMultiOptions = activeAssigneeFacets.map(option => ({
    value: option.value,
    label: option.label,
    meta: option.department || ''
  }));
  $: selectedAssigneeSummary = selectedAssignees.length === 0
    ? '全部负责人'
    : selectedAssignees.length === 1
      ? selectedAssignees[0]
      : `已选 ${selectedAssignees.length} 位负责人`;
  const executionRiskOptions = executionRiskFilters.map(filter => ({
    value: filter.value,
    label: filter.label
  }));

  // Reactive filtered tasks
  $: filteredTasks = allTasks.filter(t => {
    const projMatch = selectedProjects.length === 0 || selectedProjects.includes(t.projectKey || '');
    const assigneeMatch = selectedAssignees.length === 0 || selectedAssignees.includes(t.assignee);
    return projMatch && assigneeMatch;
  });

  // Sort tasks by Jira creation time (longest-running tasks first)
  $: sortedFilteredTasks = [...filteredTasks].sort((a, b) => {
    const timeA = a.taskCreatedAt ? new Date(a.taskCreatedAt).getTime() : 0;
    const timeB = b.taskCreatedAt ? new Date(b.taskCreatedAt).getTime() : 0;
    if (timeA === 0 && timeB === 0) return 0;
    if (timeA === 0) return 1;
    if (timeB === 0) return -1;
    return timeA - timeB; // Earliest created (longest days) comes first
  });

  $: backlog = sortedFilteredTasks.filter(t => getTaskStatusBucket(t) === 'backlog');
  $: inProgress = sortedFilteredTasks.filter(t => getTaskStatusBucket(t) === 'progress');
  $: inReview = sortedFilteredTasks.filter(t => getTaskStatusBucket(t) === 'review');
  $: done = sortedFilteredTasks.filter(t => getTaskStatusBucket(t) === 'done');
  $: activeTasks = sortedFilteredTasks.filter(t => getTaskStatusBucket(t) !== 'done');
  $: overdueTasks = activeTasks
    .filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 3)
    .sort((a, b) => getDelayDays(b.taskCreatedAt, b.status) - getDelayDays(a.taskCreatedAt, a.status));
  $: criticalTasks = overdueTasks.filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 7);
  $: focusTask = criticalTasks[0] || overdueTasks[0] || inReview[0] || inProgress[0] || backlog[0] || done[0] || null;
  $: focusTaskDelayDays = focusTask ? getDelayDays(focusTask.taskCreatedAt, focusTask.status) : 0;
  $: evidenceLinkedTasks = filteredTasks.filter(t => getEvidenceStatus(t).class === 'badge-has-code').length;
  $: evidenceCoverage = filteredTasks.length > 0 ? Math.round((evidenceLinkedTasks / filteredTasks.length) * 100) : 0;
  $: taskMetricActive = taskDataLoaded ? workItemSummary.active : activeTasks.length;
  $: activeRequirementCount = taskDataLoaded
    ? (workItemSummary.active_requirements ?? activeTasks.filter(t => t.issueType !== 'bug').length)
    : activeTasks.filter(t => t.issueType !== 'bug').length;
  $: activeBugCount = taskDataLoaded
    ? (workItemSummary.active_bugs ?? activeTasks.filter(t => t.issueType === 'bug').length)
    : activeTasks.filter(t => t.issueType === 'bug').length;
  $: taskFlowStages = [
    { key: 'backlog', label: '待办', value: taskDataLoaded ? workItemSummary.backlog : backlog.length, tone: 'neutral' },
    { key: 'progress', label: '进行中', value: taskDataLoaded ? workItemSummary.progress : inProgress.length, tone: 'info' },
    { key: 'review', label: '评审', value: taskDataLoaded ? workItemSummary.review : inReview.length, tone: 'warning' }
  ].map(stage => ({
    ...stage,
    percent: taskMetricActive > 0 ? Math.round((stage.value / taskMetricActive) * 100) : 0
  }));
  $: filteredExecutionItems = executionItems
    .filter(item => matchesExecutionFilters(
      item,
      selectedProjects,
      selectedAssignees,
      executionSearch,
      executionRiskFilter
    ))
    .sort((a, b) => compareExecutionItems(a, b));
  $: executionFocusItem = filteredExecutionItems[0] || executionItems[0] || null;
  $: executionEvidenceRate = executionSummary.total > 0
    ? Math.round((executionSummary.with_evidence / executionSummary.total) * 100)
    : 0;

  $: viewAssignees = (selectedAssignees.length === 0
    ? Array.from(new Set(filteredTasks.map(t => t.assignee)))
    : selectedAssignees
  );

  $: assigneeTasksMap = (() => {
    const map = new Map<string, Task[]>();
    viewAssignees.forEach(ass => map.set(ass, []));

    sortedFilteredTasks.forEach(t => {
      const key = t.assignee;
      const list = map.get(key);
      if (list) {
        list.push(t);
      } else {
        map.set(key, [t]);
      }
    });
    return map;
  })();
  $: {
    const _taskMetricDependencies = [taskMetricActive, activeRequirementCount, activeBugCount];
    taskAdminMetrics = buildTaskAdminMetrics();
  }
  $: {
    const _executionMetricDependencies = [executionSummary.total, executionSummary.active, executionSummary.done, executionSummary.high_risk, executionSummary.missing_evidence, executionEvidenceRate, filteredExecutionItems.length];
    executionAdminMetrics = buildExecutionAdminMetrics();
  }
  $: activeAdminMetrics = currentView === 'execution' ? executionAdminMetrics : taskAdminMetrics;
  $: taskTableRows = sortedFilteredTasks.map(mapTaskAdminRow);
  $: if (
    pendingRevealTaskId
    && currentView === 'status'
    && taskTableShellEl
    && taskTableRows.some(row => row.id === pendingRevealTaskId)
    && revealScheduledTaskId !== pendingRevealTaskId
  ) {
    void revealPendingTaskRow(pendingRevealTaskId);
  }
  $: selectedTaskForInspector = sortedFilteredTasks.find(t => t.id === selectedTaskId) || focusTask;
  $: taskInspector = selectedTaskForInspector ? mapTaskInspector(selectedTaskForInspector) : null;
  $: executionTableRows = filteredExecutionItems.map(mapExecutionAdminRow);
  $: selectedExecutionItem = filteredExecutionItems.find(item => item.task_id === selectedExecutionTaskId) || executionFocusItem;
  $: executionInspector = selectedExecutionItem ? mapExecutionInspector(selectedExecutionItem) : null;

  function buildTaskAdminMetrics(): TaskSummaryMetric[] {
    return [
      {
        label: '活跃事项',
        value: taskMetricActive,
        tone: 'info',
        surface: 'accent'
      },
      {
        label: '活跃需求',
        value: activeRequirementCount,
        tone: 'info'
      },
      {
        label: '活跃 Bug',
        value: activeBugCount,
        tone: 'danger'
      }
    ];
  }

  function buildExecutionAdminMetrics(): TaskSummaryMetric[] {
    return [
      {
        label: '执行任务',
        value: executionSummary.total,
        helper: `活跃 ${executionSummary.active} / 已闭环 ${executionSummary.done}`,
        tone: 'info'
      },
      {
        label: '证据覆盖率',
        value: `${executionEvidenceRate}%`,
        helper: `${executionSummary.with_evidence} 条有代码证据`,
        tone: executionEvidenceRate >= 80 ? 'success' : executionEvidenceRate >= 50 ? 'warning' : 'danger'
      },
      {
        label: '高风险',
        value: executionSummary.high_risk,
        helper: `陈旧 ${executionSummary.stale} / 缺证据 ${executionSummary.missing_evidence}`,
        tone: executionSummary.high_risk > 0 ? 'danger' : 'success'
      },
      {
        label: '状态不一致',
        value: executionSummary.mismatch,
        helper: `未绑定需求 ${executionSummary.orphan}`,
        tone: executionSummary.mismatch > 0 || executionSummary.orphan > 0 ? 'warning' : 'success'
      }
    ];
  }

  function getTaskStatusShortLabel(task: Task): string {
    const status = task.status.toLowerCase().trim();
    const bucket = getTaskStatusBucket(task);
    if (bucket === 'backlog' && ['backlog', 'todo', 'open', 'draft', 'ready'].includes(status)) return '待办';
    if (bucket === 'progress' && ['progress', 'in_progress', 'active', 'doing'].includes(status)) {
      return task.issueType === 'bug' ? '排查中' : '进行中';
    }
    if (bucket === 'review' && ['review', 'verification', 'testing', 'in_review'].includes(status)) return '评审中';
    if (bucket === 'done') return '已完成';
    return task.status || '-';
  }

  function getTaskStatusBucket(task: Task): 'backlog' | 'progress' | 'review' | 'done' {
    const normalized = `${task.planningState || ''} ${task.status || ''}`.toLowerCase();
    if (['done', 'closed', 'resolved', 'completed', 'archived', '已完成', '已关闭'].some(value => normalized.includes(value))) return 'done';
    if (['verification', 'review', 'testing', '验收', '评审', '测试'].some(value => normalized.includes(value))) return 'review';
    if (['in_progress', 'progress', 'active', 'doing', '进行中', '处理中', '排查'].some(value => normalized.includes(value))) return 'progress';
    return 'backlog';
  }

  function getIssueTypeLabel(task: Task): string {
    return task.issueType === 'bug' ? 'Bug' : '需求';
  }

  function getPlanningStateLabel(value?: string): string {
    const labels: Record<string, string> = {
      draft: '草稿',
      ready: '就绪',
      planned: '已规划',
      committed: '已承诺',
      in_progress: '进行中',
      verification: '验收中',
      done: '已完成',
      archived: '已归档'
    };
    const normalized = String(value || '').toLowerCase();
    return labels[normalized] || value || '未规划';
  }

  function getSyncStateLabel(value?: string): string {
    const labels: Record<string, string> = {
      synced: '已同步',
      pending: '待同步',
      conflict: '冲突',
      failed: '失败',
      not_required: '无需同步'
    };
    const normalized = String(value || '').toLowerCase();
    return labels[normalized] || value || '未知';
  }

  function getTaskRiskLabel(task: Task): string {
    const delayDays = getDelayDays(task.taskCreatedAt, task.status);
    const evidence = getEvidenceStatus(task);
    if (delayDays >= 7) return '高风险';
    if (delayDays >= 3) return '中风险';
    if (task.status.toLowerCase() !== 'backlog' && evidence.class === 'badge-no-code') return '缺证据';
    return '低风险';
  }

  function getTaskRiskTone(task: Task): AdminTone {
    return toneForRisk(getTaskRiskLabel(task));
  }

  function getEvidenceTone(task: Task): AdminTone {
    const evidence = getEvidenceStatus(task);
    if (evidence.class === 'badge-has-code') return 'success';
    if (evidence.class === 'badge-branch-only') return 'warning';
    return 'danger';
  }

  function getToneClass(tone: AdminTone | string | null | undefined): string {
    if (tone && tone in ADMIN_TONE_CLASS) {
      return ADMIN_TONE_CLASS[tone as AdminTone];
    }
    return ADMIN_TONE_CLASS.neutral;
  }

  function getTaskEvidencePercent(task: Task): number {
    const evidence = getEvidenceStatus(task);
    if (evidence.class === 'badge-has-code') return 100;
    if (evidence.class === 'badge-branch-only') return 45;
    return 0;
  }

  function mapTaskAdminRow(task: Task): AdminTableRow {
    return {
      id: task.id,
      title: task.title,
      status: getTaskStatusShortLabel(task),
      tone: toneForStatus(task.status),
      owner: task.assignee,
      dueDate: formatAdminDate(task.rawLastUpdate || task.taskCreatedAt),
      cells: {
        project: task.projectKey || '未归项目',
        projectLabel: task.projectKey
          ? (projectNamesMap[task.projectKey.toUpperCase()] || task.projectKey)
          : '未归项目',
        issueType: getIssueTypeLabel(task),
        owner: task.assignee || '未指派',
        targetRelease: task.targetRelease || '未归版本',
        planningState: getPlanningStateLabel(task.planningState),
        syncState: getSyncStateLabel(task.syncState),
        lastUpdate: formatTimeBrief(task.rawLastUpdate || task.taskCreatedAt),
        description: task.description || '暂无描述'
      }
    };
  }

  function mapTaskInspector(task: Task): AdminInspectorRecord {
    return {
      id: task.id,
      title: task.title,
      status: getTaskStatusShortLabel(task),
      tone: toneForStatus(task.status),
      facts: [
        { label: '事项类型', value: getIssueTypeLabel(task) },
        { label: '项目', value: task.projectKey ? (projectNamesMap[task.projectKey.toUpperCase()] || task.projectKey) : '未归项目' },
        { label: '负责人', value: task.assignee || '未指派' },
        { label: '计划状态', value: getPlanningStateLabel(task.planningState) },
        { label: '目标版本', value: task.targetRelease || '未归版本' },
        { label: '截止日期', value: formatTaskDate(task.dueDate) },
        { label: '同步状态', value: getSyncStateLabel(task.syncState) },
        { label: '最后更新', value: formatTimeBrief(task.rawLastUpdate || task.taskCreatedAt) }
      ],
      sections: [
        {
          title: '事项描述',
          body: task.description || '当前事项暂无描述。'
        },
        {
          title: '版本归属',
          items: [
            `目标版本：${task.targetRelease || '未归版本'}`,
            `影响版本：${task.affectedReleases?.length ? task.affectedReleases.join('、') : '未记录'}`
          ]
        },
        {
          title: '计划与同步',
          items: [
            `计划状态：${getPlanningStateLabel(task.planningState)}`,
            `同步状态：${getSyncStateLabel(task.syncState)}`,
            `数据来源：${task.source || '本地'}`
          ]
        }
      ],
      actions: [
        ...(isTaskDone(task) ? [] : [{ label: '确认完成', kind: 'primary' as const }]),
        { label: '打开版本计划', kind: isTaskDone(task) ? 'primary' : 'secondary' },
        { label: '查看详情', kind: 'secondary' }
      ]
    };
  }

  function getExecutionTone(item: ExecutionTaskItem): AdminTone {
    return toneForRisk(item.risk_level || item.risk_label || item.result_state);
  }

  function mapExecutionAdminRow(item: ExecutionTaskItem): AdminTableRow {
    return {
      id: item.task_id,
      title: item.title,
      status: item.result_label,
      tone: getExecutionTone(item),
      owner: item.assignee || '未指派',
      dueDate: formatAdminDate(item.last_evidence_at || item.last_update),
      risk: item.risk_label,
      cells: {
        issueType: item.parent_issue_type === 'bug' ? 'Bug' : 'Task',
        owner: item.assignee || '未指派',
        executionOwner: item.execution_assignee || item.assignee || '未指派',
        jiraOwner: item.jira_assignee || '-',
        result: item.result_label,
        evidenceScore: item.evidence_score,
        commitCount: item.commit_count,
        mrCount: item.mr_count,
        mergedMrCount: item.merged_mr_count,
        risk: item.risk_label,
        riskReason: item.risk_reason,
        lastEvidence: formatAdminDate(item.last_evidence_at || item.last_update),
        demand: item.parent_demand_id || '-',
        branch: item.branch || '-'
      }
    };
  }

  function mapExecutionInspector(item: ExecutionTaskItem): AdminInspectorRecord {
    return {
      id: item.task_id,
      title: item.title,
      status: item.result_label,
      tone: getExecutionTone(item),
      facts: [
        { label: '负责人', value: item.assignee || '未指派' },
        { label: '结果', value: item.result_label || '-' },
        { label: '证据分', value: `${item.evidence_score}%` },
        { label: '活跃天数', value: `${item.active_days} 天` },
        { label: 'Commit / MR', value: `${item.commit_count} / ${item.mr_count}` }
      ],
      sections: [
        {
          title: '关联 Demand',
          body: item.parent_demand_id ? `${item.parent_demand_id} ${item.parent_demand || ''}` : '未绑定父级 Demand'
        },
        {
          title: '风险说明',
          body: item.risk_reason || item.risk_label || '当前任务未返回风险说明'
        },
        {
          title: '代码证据',
          items: [
            `Repo: ${item.repo || '-'}`,
            `Branch: ${item.branch || '-'}`,
            `Last Commit: ${item.last_commit || '-'}`,
            `最近证据: ${formatAdminDate(item.last_evidence_at || item.last_update)}`
          ]
        }
      ],
      actions: [
        ...(item.parent_work_item_id || item.parent_demand_id
          ? [{ label: '打开版本计划', kind: 'primary' as const }]
          : []),
        { label: '打开代码轨迹', kind: 'secondary' }
      ]
    };
  }

  function handleTaskInspectorAction(action: AdminInspectorAction, task: Task) {
    if (action.label === '确认完成') {
      openDetails(task, true);
      return;
    }
    if (action.label === '打开版本计划') {
      onOpenDeliveryPlan(task.id);
      return;
    }
    if (action.label === '查看详情') {
      openDetails(task);
      return;
    }
  }

  function handleExecutionInspectorAction(action: AdminInspectorAction, item: ExecutionTaskItem) {
    if (action.label === '打开版本计划') {
      const parentID = item.parent_work_item_id || item.parent_demand_id || '';
      if (parentID) onOpenDeliveryPlan(parentID);
      return;
    }
    activeTelemetryTaskId = item.task_id;
    isTelemetryDrawerOpen = true;
  }

  // 反应式活跃度与卡点风险双驱动自适应折叠：
  $: if (viewAssignees && filteredTasks) {
    viewAssignees.forEach((ass, index) => {
      if (userToggledAssignees[ass] === undefined) {
        if (index === 0) {
          collapsedAssignees[ass] = false; // 保证首位成员必然展开讨论
        } else {
          const hasRiskTasks = filteredTasks.some(t => {
            const isMatch = t.assignee === ass;
            if (!isMatch || t.status.toLowerCase() === 'done' || !t.taskCreatedAt) return false;
            const createdTime = new Date(t.taskCreatedAt).getTime();
            const delayDays = Math.floor((new Date().getTime() - createdTime) / (1000 * 60 * 60 * 24));
            return delayDays >= 3;
          });
          collapsedAssignees[ass] = !hasRiskTasks; // 仅当有延期卡点风险时才自动展开
        }
      }
    });
  }

  function getAssigneeTasks(assignee: string, status: string): Task[] {
    const list = assigneeTasksMap.get(assignee) || [];
    return list.filter(t => t.status.toLowerCase() === status.toLowerCase());
  }

  function normalizeTaskView(view: TaskView | string): TaskView {
    return view === 'execution' ? 'execution' : 'status';
  }

  function setTaskView(view: TaskView | string) {
    const nextView = normalizeTaskView(view);
    if (currentView === nextView) return;
    selectedProjects = [];
    selectedAssignees = [];
    selectedProject = 'all';
    selectedAssignee = 'all';
    currentView = nextView;
    onTaskViewChange(nextView);
    void refreshCurrentTaskView();
  }

  function focusExecutionAttention() {
    executionRiskFilter = 'attention';
    setTaskView('execution');
  }

  function matchesExecutionFilters(
    item: ExecutionTaskItem,
    projectFilters: string[],
    assigneeFilters: string[],
    search: string,
    riskFilter: ExecutionRiskFilter
  ): boolean {
    if (projectFilters.length > 0 && !projectFilters.includes(item.project_key || '')) return false;
    if (assigneeFilters.length > 0 && !assigneeFilters.includes(item.assignee)) return false;
    const query = search.trim().toLowerCase();
    if (query) {
      const match = 
        (item.task_id && item.task_id.toLowerCase().includes(query)) ||
        (item.title && item.title.toLowerCase().includes(query)) ||
        (item.issue_type && item.issue_type.toLowerCase().includes(query)) ||
        (item.assignee && item.assignee.toLowerCase().includes(query)) ||
        (item.execution_assignee && item.execution_assignee.toLowerCase().includes(query)) ||
        (item.jira_assignee && item.jira_assignee.toLowerCase().includes(query)) ||
        (item.department && item.department.toLowerCase().includes(query)) ||
        (item.repo && item.repo.toLowerCase().includes(query)) ||
        (item.branch && item.branch.toLowerCase().includes(query)) ||
        (item.parent_demand_id && item.parent_demand_id.toLowerCase().includes(query)) ||
        (item.parent_demand && item.parent_demand.toLowerCase().includes(query)) ||
        (item.risk_label && item.risk_label.toLowerCase().includes(query)) ||
        (item.result_label && item.result_label.toLowerCase().includes(query));
      if (!match) return false;
    }

    if (riskFilter === 'attention') {
      return item.risk_level !== 'safe' && item.risk_level !== 'done';
    }
    if (riskFilter !== 'all') {
      return item.risk_level === riskFilter;
    }
    return true;
  }

  function compareExecutionItems(a: ExecutionTaskItem, b: ExecutionTaskItem): number {
    if (a.risk_rank !== b.risk_rank) return b.risk_rank - a.risk_rank;
    if (a.last_evidence_at !== b.last_evidence_at) {
      if (!a.last_evidence_at) return 1;
      if (!b.last_evidence_at) return -1;
      return b.last_evidence_at.localeCompare(a.last_evidence_at);
    }
    return a.task_id.localeCompare(b.task_id);
  }

  async function fetchExecutionTasks(announce = false) {
    if (executionLoading) return;
    const requestSequence = ++executionRequestSequence;
    executionLoading = true;
    executionErrorMsg = '';
    try {
      const res = await fetch('/api/execution/tasks', { cache: 'no-store' });
      if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
      const response: ExecutionTasksResponse = await res.json();
      if (requestSequence !== executionRequestSequence) return;
      const data = response.items || [];
      executionItems = data;
      executionDataLoaded = true;
      executionSummary = response.summary || emptyExecutionSummary();
      executionGeneratedAt = response.generated_at || '';
      const nextParents = new Map<string, { id: string; title: string }>();
      for (const item of data) {
        const parentID = item.parent_work_item_id || item.parent_demand_id || '';
        const parentTitle = item.parent_work_item || item.parent_demand || '';
        if (item.task_group_id && parentID) {
          nextParents.set(item.task_group_id, { id: parentID, title: parentTitle });
        }
      }
      parentWorkItemsByGroup = nextParents;
      executionErrorMsg = '';
      if (announce) {
        showToast(`执行任务已更新，共 ${data.length} 条。`, {
          type: 'success',
          title: '刷新完成'
        });
      }
    } catch (e: any) {
      if (requestSequence !== executionRequestSequence) return;
      console.error('Failed to fetch execution tasks:', e);
      executionErrorMsg = e.message || '连接执行任务 API 失败';
      if (announce) {
        showToast(executionErrorMsg, {
          type: 'error',
          title: '刷新失败'
        });
      }
    } finally {
      if (requestSequence === executionRequestSequence) executionLoading = false;
    }
  }

  function getDelayDays(taskCreatedAt: string, status: string): number {
    if (status.toLowerCase() === 'done' || !taskCreatedAt) return 0;
    const created = new Date(taskCreatedAt);
    if (isNaN(created.getTime())) return 0;
    const diffMs = new Date().getTime() - created.getTime();
    return Math.floor(diffMs / (1000 * 60 * 60 * 24));
  }

  function getActiveDays(createdAtStr: string): number {
    if (!createdAtStr) return 0;
    const created = new Date(createdAtStr).getTime();
    if (isNaN(created)) return 0;
    const diffMs = new Date().getTime() - created;
    return Math.max(0, Math.floor(diffMs / (1000 * 60 * 60 * 24)));
  }

  function getDelayClass(task: Task): string {
    const days = getDelayDays(task.taskCreatedAt, task.status);
    if (days >= 7) return 'delay-critical';
    if (days >= 3) return 'delay-warning';
    return '';
  }

  let selectedTaskCommits: any[] = [];
  let loadingCommits = false;
  let evidenceChain: any = null;
  let evidenceChainLoading = false;
  let evidenceChainError = '';
  let activeTelemetryTaskId = '';
  let isTelemetryDrawerOpen = false;

  function isTaskDone(task: Task | null | undefined): boolean {
    return String(task?.status || '').toLowerCase().trim() === 'done';
  }

  function resetCompletionState() {
    completionConfirming = false;
    completionSubmitting = false;
    completionError = '';
  }

  function openDetails(task: Task, startCompletion = false) {
    resetCompletionState();
    selectedTask = task;
    showDetails = true;
    completionConfirming = startCompletion && task.issueType !== 'task' && !isTaskDone(task);
  }

  function closeDetails() {
    showDetails = false;
    selectedTask = null;
    resetCompletionState();
  }

  function completionErrorMessage(payload: any): string {
    const code = String(payload?.error || payload?.code || '').trim();
    if (code === 'completion_evidence_required') {
      return '未找到可核对的 Commit 或已合并 MR。请确认提交中包含 Jira Key，等待轨迹刷新后重试。';
    }
    if (code === 'revision_conflict') {
      return '事项刚刚发生更新，请刷新任务表后重试。';
    }
    if (code === 'completion_forbidden') {
      return '仅负责人或管理员可以确认完成。';
    }
    return String(payload?.message || '完成操作失败，请稍后重试。');
  }

  async function completeSelectedTask() {
    const task = selectedTask;
    if (!task || task.issueType === 'task' || isTaskDone(task) || completionSubmitting) return;

    completionSubmitting = true;
    completionError = '';
    try {
      const response = await fetch(`/api/work-items/${encodeURIComponent(task.id)}/complete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          expected_revision: task.revision || 0,
          reason: '已在任务详情核对代码证据并确认完成'
        })
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(completionErrorMessage(payload));

      const result = payload as WorkItemCompletionResponse;
      const completedTask = mapWorkItem(result.snapshot);
      workItemTasks = workItemTasks.map(item => item.id === completedTask.id ? completedTask : item);
      selectedTask = completedTask;
      selectedTaskId = completedTask.id;
      completionConfirming = false;
      const commitCount = Number(result.evidence?.commit_count || 0);
      const mergedMRCount = Number(result.evidence?.merged_mr_count || 0);
      showToast(`已核对 ${commitCount} 条 Commit、${mergedMRCount} 条已合并 MR，无需绑定目标版本。`, {
        type: 'success',
        title: `${completedTask.id} 已完成`
      });
    } catch (error: any) {
      completionError = error?.message || '完成操作失败，请稍后重试。';
    } finally {
      completionSubmitting = false;
    }
  }

  function getEvidenceStatus(task: any) {
    if (!task.branch || task.branch === '-') {
      return { label: '零代码证据', class: 'badge-no-code' };
    }
    const hasCommit = (task.lastCommit && task.lastCommit !== '-') || (task.lastCommitID && task.lastCommitID !== '-');
    const hasMR = task.mrUrl && task.mrUrl !== '';
    if (hasCommit || hasMR) {
      return { label: '已关联代码', class: 'badge-has-code' };
    }
    return { label: '有分支无提交', class: 'badge-branch-only' };
  }

  function getProjectName(id: string): string {
    if (!id) return '-';
    const idx = id.indexOf('-');
    if (idx !== -1) {
      const prefix = id.substring(0, idx).toUpperCase();
      if (prefix === "TASK") {
        return "本地任务";
      }
      return prefix;
    }
    return "本地项目";
  }

  function getStatusLabel(status: string, issueType: string = '') {
    const s = status.toLowerCase();
    const type = (issueType || '').toLowerCase();
    if (s === 'backlog') return '待办任务 (Backlog)';
    if (s === 'progress') {
      if (type === 'bug' || type === '故障' || type === 'bug 缺陷') {
        return '排查中 (Investigation)';
      }
      return '进行中 (In Progress)';
    }
    if (s === 'review') return '代码评审 (In Review)';
    if (s === 'done') return '已完成 (Done)';
    return status;
  }

  function formatTimeFull(timeStr: string) {
    if (!timeStr) return '-';
    try {
      const date = new Date(timeStr);
      if (isNaN(date.getTime())) return timeStr;
      const yyyy = date.getFullYear();
      const mm = String(date.getMonth() + 1).padStart(2, '0');
      const dd = String(date.getDate()).padStart(2, '0');
      const hh = String(date.getHours()).padStart(2, '0');
      const min = String(date.getMinutes()).padStart(2, '0');
      const sec = String(date.getSeconds()).padStart(2, '0');
      return `${yyyy}-${mm}-${dd} ${hh}:${min}:${sec}`;
    } catch (e) {
      return timeStr;
    }
  }

  function formatTimeBrief(timeStr: string) {
    if (!timeStr) return '-';
    try {
      const date = new Date(timeStr);
      if (isNaN(date.getTime())) return timeStr;
      const mm = String(date.getMonth() + 1).padStart(2, '0');
      const dd = String(date.getDate()).padStart(2, '0');
      const hh = String(date.getHours()).padStart(2, '0');
      const min = String(date.getMinutes()).padStart(2, '0');
      return `${mm}-${dd} ${hh}:${min}`;
    } catch (e) {
      return timeStr;
    }
  }

  function formatTaskDate(timeStr?: string) {
    if (!timeStr) return '-';
    const datePrefix = timeStr.match(/^(\d{4}-\d{2}-\d{2})/);
    if (datePrefix) return datePrefix[1];
    const date = new Date(timeStr);
    if (isNaN(date.getTime())) return timeStr;
    const yyyy = date.getFullYear();
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const dd = String(date.getDate()).padStart(2, '0');
    return `${yyyy}-${mm}-${dd}`;
  }

  function formatUpdate(timeStr: string) {
    if (!timeStr) return '-';
    try {
      const date = new Date(timeStr);
      if (isNaN(date.getTime())) return timeStr;
      
      const diffMs = new Date().getTime() - date.getTime();
      const diffMins = Math.round(diffMs / 60000);
      if (diffMins < 1) return '刚刚';
      if (diffMins < 60) return `${diffMins}分钟前`;
      const diffHours = Math.round(diffMins / 60);
      if (diffHours < 24) return `${diffHours}小时前`;
      
      const yyyy = date.getFullYear();
      const mm = String(date.getMonth() + 1).padStart(2, '0');
      const dd = String(date.getDate()).padStart(2, '0');
      const hh = String(date.getHours()).padStart(2, '0');
      const min = String(date.getMinutes()).padStart(2, '0');
      return `${yyyy}-${mm}-${dd} ${hh}:${min}`;
    } catch (e) {
      return timeStr;
    }
  }

  let jiraBaseUrl = '';

  async function fetchJiraLinkConfig() {
    try {
      const res = await fetch('/api/jira/link-config');
      if (res.ok) {
        const data = await res.json();
        jiraBaseUrl = data?.base_url ? data.base_url.replace(/\/+$/, '') : '';
      }
    } catch (e) {
      console.error('Failed to fetch Jira link config:', e);
    }
  }

  function mapTask(t: TaskResponse): Task {
    const rawType = (t.issue_type || 'task').toLowerCase().trim();
    let issueType = 'task';
    if (rawType === 'bug' || rawType === '缺陷' || rawType === '故障' || rawType === 'defect') {
      issueType = 'bug';
    } else if (rawType === 'demand') {
      issueType = 'demand';
    }
    return {
      id: t.task_id,
      title: t.title || '-',
      repo: t.repo || '-',
      assignee: t.assignee || '未指派',
      branch: t.branch || '-',
      lastCommit: t.last_commit || '-',
      lastUpdate: formatUpdate(t.last_update),
      status: t.status,
      rawLastUpdate: t.last_update,
      issueType: issueType,
      taskCreatedAt: t.task_created_at || t.last_update,
      mrIid: t.mr_iid,
      mrUrl: t.mr_url,
      taskGroupId: t.task_group_id
    };
  }

  function mapWorkItem(snapshot: WorkItemSnapshot): Task {
    const item = snapshot.work_item;
    const rawType = String(item.issue_type || 'requirement').toLowerCase().trim();
    const issueType = ['bug', 'defect', '缺陷', '故障'].includes(rawType) ? 'bug' : 'requirement';
    return {
      id: item.task_id,
      title: item.title || item.task_id,
      description: item.description || '',
      repo: item.repo || '-',
      assignee: item.assignee || '未指派',
      branch: item.branch || '-',
      lastCommit: item.last_commit || '-',
      lastUpdate: formatUpdate(item.last_update || ''),
      status: item.status || item.planning_state || 'backlog',
      rawLastUpdate: item.last_update || '',
      issueType,
      taskCreatedAt: item.task_created_at || item.last_update || '',
      mrIid: item.mr_iid,
      mrUrl: item.mr_url,
      taskGroupId: item.task_group_id,
      projectKey: item.project_key || '',
      targetRelease: snapshot.target_releases?.[0]?.name || '',
      affectedReleases: (snapshot.affected_releases || []).map(release => release.name).filter(Boolean),
      source: item.source || '',
      planningState: item.planning_state || 'draft',
      dueDate: item.due_date || '',
      syncState: snapshot.sync_state || '',
      revision: item.revision || 0
    };
  }

  function mapExecutionTask(t: ExecutionTaskItem): Task {
    return {
      id: t.task_id,
      title: t.title || '-',
      repo: t.repo || '-',
      assignee: t.assignee || '未指派',
      branch: t.branch || '-',
      lastCommit: t.last_commit || '-',
      lastUpdate: formatUpdate(t.last_update),
      status: t.status,
      rawLastUpdate: t.last_update,
      issueType: 'task',
      taskCreatedAt: t.created_at || t.last_update,
      mrIid: t.mr_iid,
      mrUrl: t.mr_url,
      taskGroupId: t.task_group_id,
      parentWorkItemId: t.parent_work_item_id || t.parent_demand_id || '',
      parentWorkItem: t.parent_work_item || t.parent_demand || '',
      projectKey: t.project_key || '',
      targetRelease: t.target_release || ''
    };
  }

  function getParentDemand(taskGroupId?: string): Task | undefined {
    if (!taskGroupId || taskGroupId === '-' || taskGroupId === '') return undefined;
    const parent = parentWorkItemsByGroup.get(taskGroupId);
    if (!parent) return undefined;
    return {
      id: parent.id,
      title: parent.title,
      repo: '-',
      assignee: '',
      branch: '-',
      lastCommit: '-',
      lastUpdate: '',
      status: '',
      rawLastUpdate: '',
      issueType: 'demand',
      taskCreatedAt: '',
      taskGroupId
    };
  }

  function hasParentDemand(taskGroupId?: string): boolean {
    return Boolean(getParentDemand(taskGroupId));
  }

  function getParentDemandId(taskGroupId?: string): string {
    return getParentDemand(taskGroupId)?.id || '';
  }

  function getParentDemandTitle(taskGroupId?: string): string {
    return getParentDemand(taskGroupId)?.title || '';
  }

  async function fetchTasks(announce = false) {
    taskRequestController?.abort();
    const controller = new AbortController();
    taskRequestController = controller;
    const requestSequence = ++taskRequestSequence;
    const projectScope = [...selectedProjects];
    const assigneeScope = [...selectedAssignees];
    if (announce || taskDataLoaded) taskRefreshing = true;
    try {
      const snapshots: WorkItemSnapshot[] = [];
      let offset = 0;
      let total = 0;
      let nextSummary = emptyWorkItemSummary();
      do {
        const params = new URLSearchParams();
        params.set('active', 'true');
        params.set('limit', '500');
        params.set('offset', String(offset));
        for (const project of projectScope) params.append('project', project);
        for (const assignee of assigneeScope) params.append('assignee', assignee);
        const res = await fetch(`/api/work-items?${params.toString()}`, {
          cache: 'no-store',
          signal: controller.signal
        });
        if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
        const response: WorkItemsResponse = await res.json();
        if (requestSequence !== taskRequestSequence) return;
        const page = response.items || [];
        snapshots.push(...page);
        if (offset === 0 && response.summary) nextSummary = response.summary;
        total = response.total || snapshots.length;
        offset += page.length;
        if (page.length === 0) break;
      } while (offset < total);
      const data = snapshots.map(mapWorkItem);
      workItemTasks = data;
      workItemSummary = nextSummary;
      if (selectedSearchTask && data.some(task => task.id === selectedSearchTask?.id)) {
        selectedSearchTask = null;
      }
      taskDataLoaded = true;
      errorMsg = '';
      taskLastRefreshedAt = new Intl.DateTimeFormat('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
      }).format(new Date());
      if (announce) {
        showToast(`任务表已更新，载入 ${data.length} 条未闭环需求与 Bug。`, {
          type: 'success',
          title: '刷新完成'
        });
      }
    } catch (e: any) {
      if (requestSequence !== taskRequestSequence) return;
      if (e?.name === 'AbortError') return;
      console.error('Failed to fetch tasks:', e);
      errorMsg = e.message || '连接 API 失败';
      if (announce) {
        showToast(errorMsg, {
          type: 'error',
          title: '刷新失败'
        });
      }
    } finally {
      if (requestSequence === taskRequestSequence) {
        loading = false;
        taskRefreshing = false;
        if (taskRequestController === controller) taskRequestController = null;
      }
    }
  }

  function handleTaskScopeFiltersChanged() {
    if (currentView !== 'status') return;
    if (taskScopeRefreshTimer) clearTimeout(taskScopeRefreshTimer);
    taskScopeRefreshTimer = setTimeout(() => {
      taskScopeRefreshTimer = null;
      void fetchTasks();
    }, 120);
  }

  async function fetchSharedDeliveryDirectory() {
    try {
      const directory = await fetchDeliveryDirectory();
      deliveryProjects = directory.projects;
      activeAssigneeFacets = directory.assignees;
      projectNamesMap = {
        ...projectNamesMap,
        ...Object.fromEntries(
          directory.projects.map(project => [project.project_key, project.project_name || project.project_key])
        )
      };
      const allowedAssignees = new Set(directory.assignees.map(option => option.value));
      selectedAssignees = selectedAssignees.filter(value => allowedAssignees.has(value));
      const allowedProjects = new Set(directory.projects.map(project => project.project_key));
      selectedProjects = selectedProjects.filter(value => allowedProjects.has(value));
    } catch (e) {
      console.error('Failed to fetch shared delivery directory:', e);
    }
  }

  async function refreshStatusTasks() {
    await Promise.all([fetchTasks(true), fetchSharedDeliveryDirectory()]);
  }

  async function refreshCurrentTaskView(force = false) {
    if (currentView === 'status') {
      if (force || !taskDataLoaded) await fetchTasks();
      return;
    }
    if (force || !executionDataLoaded) await fetchExecutionTasks();
  }

  function handleProjectPreferencesUpdated() {
    selectedProjects = [];
    selectedAssignees = [];
    taskDataLoaded = false;
    executionDataLoaded = false;
    selectedProject = 'all';
    selectedAssignee = 'all';
    void Promise.all([fetchSharedDeliveryDirectory(), refreshCurrentTaskView(true)]);
  }

  onMount(() => {
    void Promise.all([fetchSharedDeliveryDirectory(), refreshCurrentTaskView()]);
    fetchJiraLinkConfig();
    const unsubscribeTelemetryUpdates = subscribeTelemetryUpdates(
      window,
      () => refreshCurrentTaskView(true),
      { visibilityTarget: document }
    );
    intervalId = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void refreshCurrentTaskView(true);
      }
    }, 60000);
    document.addEventListener('click', handleDocumentClick);
    window.addEventListener('well-ambient:global-search-select', handleGlobalSearchSelection);
    window.addEventListener('well-ambient:global-search-clear', handleGlobalSearchClear);
    window.addEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
    return unsubscribeTelemetryUpdates;
  });

  onDestroy(() => {
    if (intervalId) {
      clearInterval(intervalId);
    }
    if (taskScopeRefreshTimer) clearTimeout(taskScopeRefreshTimer);
    taskRequestController?.abort();
    document.removeEventListener('click', handleDocumentClick);
    window.removeEventListener('well-ambient:global-search-select', handleGlobalSearchSelection);
    window.removeEventListener('well-ambient:global-search-clear', handleGlobalSearchClear);
    window.removeEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
  });
</script>

{#snippet taskScopeFilters()}
  <div class="phase41-toolbar-filters" aria-label="任务筛选">
    <div class="phase41-filter-multi phase41-project-filter">
      <MultiSelect
        id="task-project-filter"
        bind:values={selectedProjects}
        on:change={handleTaskScopeFiltersChanged}
        options={projectMultiOptions}
        placeholder="全部项目"
        searchPlaceholder="搜索项目"
        emptyText="没有匹配项目"
        controlLabel="项目"
        ariaLabel="筛选任务项目，可多选"
        compact={true}
        summaryMode={true}
        overlay={true}
        showClear={true}
        clearText="全部项目"
      />
    </div>

    <div class="phase41-filter-multi phase41-owner-filter">
      <MultiSelect
        id="task-owner-filter"
        bind:values={selectedAssignees}
        on:change={handleTaskScopeFiltersChanged}
        options={ownerMultiOptions}
        placeholder="全部负责人"
        searchPlaceholder="搜索负责人"
        emptyText="没有匹配负责人"
        controlLabel="负责人"
        ariaLabel="筛选任务负责人，可多选"
        compact={true}
        summaryMode={true}
        overlay={true}
        showClear={true}
        clearText="全部负责人"
      />
    </div>
  </div>
{/snippet}

<section class="kanban-section task-console" class:view-execution={currentView === 'execution'}>
  <div
    class="phase41-summary-strip wa-admin-card"
    class:is-status={currentView !== 'execution'}
    class:is-execution={currentView === 'execution'}
    aria-label="任务指标与状态"
  >
    {#each activeAdminMetrics as metric}
      <article
        class="phase41-metric {ADMIN_TONE_CLASS[metric.tone || 'neutral']}"
        class:surface-accent={metric.surface === 'accent'}
      >
        <span>{metric.label}</span>
        <strong>{metric.value}</strong>
        {#if metric.helper}
          <small>{metric.helper}</small>
        {/if}
        {#if metric.delta}
          <em>{metric.delta}</em>
        {/if}
      </article>
    {/each}

    {#if currentView !== 'execution'}
      {#each taskFlowStages as stage}
        <button class="phase41-stage-card tone-{stage.tone}" type="button" on:click={() => setTaskView('status')}>
          <span>{stage.label}</span>
          <strong>{stage.value}</strong>
        </button>
      {/each}
    {/if}
  </div>

  {#if false}
  <div class="task-command-surface">
    <div class="task-command-main">
      <div class="task-command-eyebrow">
        <span class="font-mono">GIT DELIVERY COMMAND</span>
        <span class="task-live-dot">同步中</span>
      </div>

      <div class="task-command-title-row">
        <div class="task-command-copy">
          <h2>Git 协同看板</h2>
          <p>围绕 Jira Task、Branch、Commit、MR 和证据链组织研发执行，不再把任务状态当成孤立卡片。</p>
        </div>

        <div class="view-toggle task-view-toggle" aria-label="任务视图切换">
          <button class="toggle-btn {currentView === 'status' ? 'active' : ''}" on:click={() => setTaskView('status')}>
            状态
          </button>
          <button class="toggle-btn {currentView === 'execution' ? 'active' : ''}" on:click={() => setTaskView('execution')}>
            执行
          </button>
        </div>
      </div>

      <div class="task-filter-dock">
        <div class="custom-select-container" bind:this={projectSelectEl}>
          <div class="custom-select-trigger combobox-trigger task-filter-control">
            <span class="filter-label font-mono">PROJECT</span>
            <input
              type="text"
              class="combobox-trigger-input"
              placeholder={selectedProject === 'all' ? '全部项目' : (projectNamesMap[selectedProject.toUpperCase()] || selectedProject)}
              bind:value={projectSearchText}
              on:focus|stopPropagation={() => showProjectDropdown = true}
              on:click|stopPropagation={() => showProjectDropdown = true}
            />
            <button
              type="button"
              class="select-arrow"
              on:click|stopPropagation={() => showProjectDropdown = !showProjectDropdown}
              aria-label="切换项目筛选"
            >{showProjectDropdown ? '▲' : '▼'}</button>
          </div>
          {#if showProjectDropdown}
            <div class="custom-select-options">
              {#each projectOptions.filter(proj => {
                if (proj === 'all') return true;
                if (!projectSearchText) return true;
                const term = projectSearchText.toLowerCase();
                const keyMatch = proj.toLowerCase().includes(term);
                const name = projectNamesMap[proj.toUpperCase()] || '';
                const nameMatch = name.toLowerCase().includes(term);
                return keyMatch || nameMatch;
              }) as proj}
                <button
                  class="custom-option {selectedProject === proj ? 'active' : ''}"
                  on:click={() => selectProject(proj)}
                >
                  {proj === 'all' ? '全部项目' : (projectNamesMap[proj.toUpperCase()] || proj)}
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <div class="custom-select-container" bind:this={assigneeSelectEl}>
          <div class="custom-select-trigger combobox-trigger task-filter-control">
            <span class="filter-label font-mono">OWNER</span>
            <input
              type="text"
              class="combobox-trigger-input"
              placeholder={selectedAssignee === 'all' ? '全部经办人' : selectedAssignee}
              bind:value={assigneeSearchText}
              on:focus|stopPropagation={() => showAssigneeDropdown = true}
              on:click|stopPropagation={() => showAssigneeDropdown = true}
            />
            <button
              type="button"
              class="select-arrow"
              on:click|stopPropagation={() => showAssigneeDropdown = !showAssigneeDropdown}
              aria-label="切换经办人筛选"
            >{showAssigneeDropdown ? '▲' : '▼'}</button>
          </div>
          {#if showAssigneeDropdown}
            <div class="custom-select-options">
              {#each assigneeOptions.filter(ass => ass === 'all' || !assigneeSearchText || ass.toLowerCase().includes(assigneeSearchText.toLowerCase())) as ass}
                <button
                  class="custom-option {selectedAssignee === ass ? 'active' : ''}"
                  on:click={() => selectAssignee(ass)}
                >
                  {ass === 'all' ? '全部经办人' : ass}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <div class="task-command-metrics">
        <div class="task-metric-cell tone-total">
          <span class="metric-label font-mono">TOTAL TASKS</span>
          <strong>{filteredTasks.length}</strong>
          <em>活跃 {activeTasks.length} / 完成 {done.length}</em>
        </div>
        <div class="task-metric-cell tone-info">
          <span class="metric-label font-mono">EVIDENCE</span>
          <strong>{evidenceCoverage}%</strong>
          <em>{evidenceLinkedTasks} 条已有代码证据</em>
        </div>
        <div class="task-metric-cell tone-warning">
          <span class="metric-label font-mono">REVIEW</span>
          <strong>{inReview.length}</strong>
          <em>等待合并或评审闭环</em>
        </div>
        <div class="task-metric-cell tone-danger">
          <span class="metric-label font-mono">DELAY</span>
          <strong>{overdueTasks.length}</strong>
          <em>{criticalTasks.length} 项超过 7 天</em>
        </div>
      </div>

      <div class="task-flow-strip" aria-label="任务流转阶段">
        {#each taskFlowStages as stage}
          <button class="task-flow-card tone-{stage.tone}" type="button" on:click={() => setTaskView('status')}>
            <span class="font-mono">{stage.label}</span>
            <strong>{stage.value}</strong>
            <em>{stage.percent}%</em>
            <i style="width: {stage.percent}%"></i>
          </button>
        {/each}
      </div>
    </div>

    <aside class="task-focus-panel" aria-label="当前任务焦点">
      <div class="focus-panel-head">
        <span class="summary-kicker font-mono">FOCUS TASK</span>
        {#if focusTaskDelayDays >= 7}
          <strong class="focus-severity is-critical">CRITICAL</strong>
        {:else if focusTaskDelayDays >= 3}
          <strong class="focus-severity is-watch">WATCH</strong>
        {:else}
          <strong class="focus-severity">LIVE</strong>
        {/if}
      </div>

      {#if focusTask}
        <div class="focus-task-body">
          <span class="focus-task-id font-mono">{focusTask.id}</span>
          <h3>{focusTask.title}</h3>
          <div class="focus-meta-grid">
            <span>
              <em>负责人</em>
              <strong>{focusTask.assignee}</strong>
            </span>
            <span>
              <em>阶段</em>
              <strong>{getStatusLabel(focusTask.status, focusTask.issueType)}</strong>
            </span>
            <span>
              <em>活跃天数</em>
              <strong>{getActiveDays(focusTask.taskCreatedAt)} 天</strong>
            </span>
            <span>
              <em>证据</em>
              <strong>{getEvidenceStatus(focusTask).label}</strong>
            </span>
          </div>
        </div>

        <div class="focus-action-row">
          <button type="button" class="focus-primary-btn" on:click={() => openDetails(focusTask)}>
            查看证据链
          </button>
          <button type="button" class="focus-secondary-btn" on:click={focusExecutionAttention}>
            执行追踪
          </button>
        </div>
      {:else}
        <div class="focus-empty font-mono">当前筛选下暂无任务</div>
      {/if}

    </aside>
  </div>
  {/if}

  {#if currentView === 'execution'}
    <div class="phase41-workbench phase41-execution-workbench">
      <section class="wa-admin-section phase41-table-panel">
        <div class="wa-admin-card wa-admin-toolbar phase41-table-toolbar">
          <div>
            <span class="phase41-kicker">执行追踪</span>
            <strong>执行任务证据与结果追踪</strong>
            <small>{filteredExecutionItems.length} / {executionItems.length} 条 · {executionGeneratedAt || '未同步'}</small>
          </div>

          <div class="phase41-execution-controls">
            <div class="execution-search-shell phase41-search-control">
              <span class="execution-search-mark"></span>
              <input
                type="text"
                placeholder="搜索任务编号、标题、负责人、分支或主线"
                value={executionSearchInput}
                on:input={handleExecutionSearch}
              />
            </div>

            <div class="phase41-execution-select phase41-risk-select">
              <Select
                id="execution-risk-filter"
                bind:value={executionRiskFilter}
                on:change={(event) => executionRiskFilter = event.detail as ExecutionRiskFilter}
                options={executionRiskOptions}
                placeholder="风险状态"
                ariaLabel="筛选执行风险状态"
                searchable={false}
                compact={true}
              />
            </div>

            <div class="phase41-execution-select phase41-filter-multi phase41-project-select">
              <MultiSelect
                id="execution-project-filter"
                bind:values={selectedProjects}
                options={projectMultiOptions}
                placeholder="全部项目"
                searchPlaceholder="搜索项目"
                emptyText="没有匹配项目"
                controlLabel="项目"
                ariaLabel="筛选执行项目，可多选"
                compact={true}
                summaryMode={true}
                overlay={true}
                showClear={true}
                clearText="全部项目"
              />
            </div>

            <div class="phase41-execution-select phase41-filter-multi phase41-assignee-select">
              <MultiSelect
                id="execution-owner-filter"
                bind:values={selectedAssignees}
                options={ownerMultiOptions}
                placeholder="全部负责人"
                searchPlaceholder="搜索负责人"
                emptyText="没有匹配负责人"
                controlLabel="负责人"
                ariaLabel="筛选执行负责人，可多选"
                compact={true}
                summaryMode={true}
                overlay={true}
                showClear={true}
                clearText="全部负责人"
              />
            </div>

            <button
              class="wa-admin-action secondary"
              class:is-loading={executionLoading}
              disabled={executionLoading}
              on:click={() => fetchExecutionTasks(true)}
            >
              {executionLoading ? '刷新中' : '刷新'}
            </button>
          </div>
        </div>

        <div class="wa-admin-table-shell phase41-table-shell">
          <table class="wa-admin-table phase41-table">
            <thead>
              <tr>
                {#each executionTableColumns as column}
                  <th style="width: {column.width || 'auto'}; text-align: {column.align || 'left'}">{column.label}</th>
                {/each}
              </tr>
            </thead>
            <tbody>
              {#if executionLoading && executionTableRows.length === 0}
                {#each Array(6) as _}
                  <tr class="phase41-skeleton-row"><td colspan={executionTableColumns.length}></td></tr>
                {/each}
              {:else if executionErrorMsg}
                <tr><td colspan={executionTableColumns.length} class="phase41-empty-cell error">{executionErrorMsg}</td></tr>
              {:else if executionTableRows.length === 0}
                <tr><td colspan={executionTableColumns.length} class="phase41-empty-cell">当前筛选下暂无执行任务</td></tr>
              {:else}
                {#each executionTableRows as row (row.id)}
                  <tr
                    class:is-selected={selectedExecutionItem?.task_id === row.id}
                    tabindex="0"
                    on:click={() => selectedExecutionTaskId = row.id}
                    on:keydown={(event) => { if (event.key === 'Enter') selectedExecutionTaskId = row.id; }}
                  >
                    <td>
                      <div class="phase41-title-cell">
                        <div>
                          {#if jiraBaseUrl && row.id && !row.id.startsWith('TASK-')}
                            <a href="{jiraBaseUrl}/browse/{row.id}" target="_blank" rel="noopener noreferrer" class="phase41-id-link" on:click|stopPropagation>{row.id}</a>
                          {:else}
                            <span class="phase41-id-link as-text">{row.id}</span>
                          {/if}
                          <span class="phase41-type-label {String(row.cells.issueType).toLowerCase() === 'bug' ? 'is-bug' : 'is-task'}">
                            <span class="phase41-type-icon {String(row.cells.issueType).toLowerCase() === 'bug' ? 'is-bug' : 'is-task'}" aria-hidden="true">
                              {String(row.cells.issueType).toLowerCase() === 'bug' ? 'B' : 'T'}
                            </span>
                            {row.cells.issueType}
                          </span>
                          {#if row.cells.demand !== '-'}
                            <span class="wa-admin-pill tone-neutral">{row.cells.demand}</span>
                          {/if}
                        </div>
                        <strong>{row.title}</strong>
                      </div>
                    </td>
                    <td>
                      <div class="phase41-owner-cell">
                        <strong>{row.owner}</strong>
                        {#if row.cells.executionOwner !== row.owner}
                          <small>执行 {row.cells.executionOwner}</small>
                        {/if}
                        {#if row.cells.jiraOwner !== '-'}
                          <small>Jira {row.cells.jiraOwner}</small>
                        {/if}
                      </div>
                    </td>
                    <td><span class="wa-admin-pill {ADMIN_TONE_CLASS[row.tone || 'neutral']}">{row.status}</span></td>
                    <td>
                      <div class="phase41-evidence-cell">
                        <div class="wa-admin-progress" style="--progress: {Number(row.cells.evidenceScore) || 0}%"></div>
                        <span>{row.cells.evidenceScore}% · {row.cells.commitCount} commit / {row.cells.mrCount} MR</span>
                      </div>
                    </td>
                    <td><span class="wa-admin-pill {ADMIN_TONE_CLASS[toneForRisk(String(row.risk || ''))]}">{row.risk || '-'}</span></td>
                    <td>{row.cells.lastEvidence}</td>
                    <td class="phase41-action-cell">
                      <button
                        type="button"
                        class="wa-admin-action secondary compact"
                        on:click|stopPropagation={() => {
                          selectedExecutionTaskId = row.id;
                          activeTelemetryTaskId = row.id;
                          isTelemetryDrawerOpen = true;
                        }}
                      >
                        轨迹
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </section>

      <aside class="wa-admin-card wa-admin-inspector phase41-inspector">
        {#if executionInspector && selectedExecutionItem}
          <div class="phase41-inspector-head">
            <span class="phase41-kicker">执行详情</span>
            <h3>{executionInspector.title}</h3>
            <div>
              <span class="phase41-id-link as-text">{executionInspector.id}</span>
              <span class="wa-admin-pill {ADMIN_TONE_CLASS[executionInspector.tone || 'neutral']}">{executionInspector.status}</span>
            </div>
          </div>

          <dl class="phase41-fact-grid">
            {#each executionInspector.facts as fact}
              <div>
                <dt>{fact.label}</dt>
                <dd>{fact.value}</dd>
              </div>
            {/each}
          </dl>

          <div class="phase41-inspector-progress">
            <div class="wa-admin-progress" style="--progress: {selectedExecutionItem.evidence_score}%"></div>
            <span>证据覆盖 {selectedExecutionItem.evidence_score}%</span>
          </div>

          {#each executionInspector.sections as section}
            <section class="phase41-inspector-section">
              <h4>{section.title}</h4>
              {#if section.body}
                <p>{section.body}</p>
              {/if}
              {#if section.items}
                <ul>
                  {#each section.items as item}
                    <li>{item}</li>
                  {/each}
                </ul>
              {/if}
            </section>
          {/each}

          <div class="phase41-inspector-actions">
            {#each executionInspector.actions || [] as action}
              <button
                type="button"
                class="wa-admin-action {action.kind}"
                on:click={() => handleExecutionInspectorAction(action, selectedExecutionItem)}
              >
                {action.label}
              </button>
            {/each}
          </div>
        {:else}
          <div class="phase41-empty-inspector">暂无执行追踪数据</div>
        {/if}
      </aside>
    </div>

    {#if false}
    <div class="execution-workbench">
      <div class="execution-summary-grid">
        <div class="execution-summary-cell">
          <span class="summary-kicker font-mono">TOTAL</span>
          <strong>{executionSummary.total}</strong>
          <em>执行任务</em>
        </div>
        <div class="execution-summary-cell is-red">
          <span class="summary-kicker font-mono">HIGH RISK</span>
          <strong>{executionSummary.high_risk}</strong>
          <em>高风险</em>
        </div>
        <div class="execution-summary-cell is-amber">
          <span class="summary-kicker font-mono">ORPHAN</span>
          <strong>{executionSummary.orphan}</strong>
          <em>未绑定需求</em>
        </div>
        <div class="execution-summary-cell is-blue">
          <span class="summary-kicker font-mono">EVIDENCE</span>
          <strong>{executionSummary.with_evidence}</strong>
          <em>有代码证据</em>
        </div>
        <div class="execution-summary-cell is-violet">
          <span class="summary-kicker font-mono">MISMATCH</span>
          <strong>{executionSummary.mismatch}</strong>
          <em>状态不一致</em>
        </div>
      </div>

      <div class="execution-control-panel">
        <div class="execution-search-shell">
          <span class="execution-search-mark"></span>
          <input 
            type="text" 
            placeholder="搜索 Jira Key、任务、负责人、分支、需求" 
            value={executionSearchInput}
            on:input={handleExecutionSearch}
          />
        </div>

        <div class="execution-risk-strip">
          {#each executionRiskFilters as filter}
            <button
              class:active={executionRiskFilter === filter.value}
              on:click={() => executionRiskFilter = filter.value}
            >
              {filter.label}
            </button>
          {/each}
        </div>

        <div class="custom-select-container execution-assignee-select">
          <div class="custom-select-trigger combobox-trigger">
            <input 
              type="text" 
              class="combobox-trigger-input"
              placeholder={selectedAssignee === 'all' ? '全部负责人' : selectedAssignee}
              bind:value={assigneeSearchText}
              on:focus|stopPropagation={() => showAssigneeDropdown = true}
              on:click|stopPropagation={() => showAssigneeDropdown = true}
            />
            <button
              type="button"
              class="select-arrow" 
              on:click|stopPropagation={() => showAssigneeDropdown = !showAssigneeDropdown}
              aria-label="切换执行负责人筛选"
            >{showAssigneeDropdown ? '▲' : '▼'}</button>
          </div>
          {#if showAssigneeDropdown}
            <div class="custom-select-options">
              {#each assigneeOptions.filter(ass => ass === 'all' || !assigneeSearchText || ass.toLowerCase().includes(assigneeSearchText.toLowerCase())) as assignee}
                <button
                  class="custom-option {selectedAssignee === assignee ? 'active' : ''}"
                  on:click={() => {
                    selectedAssignee = assignee;
                    showAssigneeDropdown = false;
                  }}
                >
                  {assignee === 'all' ? '全部负责人' : assignee}
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <button class="execution-refresh-btn font-mono" class:is-loading={executionLoading} on:click={() => fetchExecutionTasks(true)}>
          刷新
        </button>
      </div>

      <div class="execution-lab-layout">
        <div class="execution-lab-main">
          {#if executionLoading && executionItems.length === 0}
            <div class="execution-state">正在加载执行追踪...</div>
          {:else if executionErrorMsg}
            <div class="execution-state error">{executionErrorMsg}</div>
          {:else}
            <div class="execution-table-panel">
          <div class="execution-table-head">
            <div>
              <span class="summary-kicker font-mono">EXECUTION OBSERVABILITY</span>
              <h3>Jira Task 开发结果追踪</h3>
            </div>
            <span class="execution-count font-mono">{filteredExecutionItems.length} / {executionItems.length} · {executionGeneratedAt || '未同步'}</span>
          </div>

          <div class="execution-table-wrapper">
            <div class="exec-grid-table">
              <div class="exec-grid-thead">
                <div class="exec-grid-tr">
                  <div class="exec-grid-th">任务名称</div>
                  <div class="exec-grid-th">负责人</div>
                  <div class="exec-grid-th">当前状态</div>
                  <div class="exec-grid-th">Git Telemetry 代码证据 & MR</div>
                  <div class="exec-grid-th col-action">操作</div>
                </div>
              </div>
              <div class="exec-grid-tbody">
                {#each filteredExecutionItems as item}
                  <div class="exec-grid-tr">
                    <div class="exec-grid-td">
                      <div class="exec-task-stack">
                        <div class="exec-id-row">
                          {#if jiraBaseUrl && item.task_id && !item.task_id.startsWith('TASK-')}
                            <a href="{jiraBaseUrl}/browse/{item.task_id}" target="_blank" rel="noopener noreferrer" class="exec-task-id font-mono exec-link-only-id">{item.task_id}</a>
                          {:else}
                            <span class="exec-task-id font-mono">{item.task_id}</span>
                          {/if}
                          <span class="exec-type-badge type-{item.issue_type}">{item.issue_type === 'bug' ? 'Bug' : 'Task'}</span>
                        </div>
                        <strong>{item.title}</strong>
                      </div>
                    </div>
                    <div class="exec-grid-td">
                      <div class="exec-assignee-cell">
                        <strong>{item.assignee || '未指派'}</strong>
                        {#if item.jira_assignee}
                          <span class="exec-assignee-source">Jira 当前</span>
                        {/if}
                        {#if item.execution_assignee && item.execution_assignee !== item.assignee}
                          <span class="exec-assignee-note">执行负责人 {item.execution_assignee}</span>
                        {/if}
                      </div>
                    </div>
                    <div class="exec-grid-td">
                      <div class="exec-result-stack">
                        <span class="result-pill result-{item.result_state}">{item.result_label}</span>
                      </div>
                    </div>
                    <div class="exec-grid-td">
                      <div class="exec-evidence-stack">
                        <div class="evidence-score-bar" title="代码完整度: {item.evidence_score}%">
                          <div class="evidence-score-fill" style="width: {item.evidence_score}%"></div>
                          <span class="evidence-score-text">{item.evidence_score}%</span>
                        </div>
                      </div>
                    </div>
                    <div class="exec-grid-td col-action">
                      <button class="exec-action-btn font-mono" on:click={() => {
                        activeTelemetryTaskId = item.task_id;
                        isTelemetryDrawerOpen = true;
                      }}>代码轨迹</button>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
            {#if filteredExecutionItems.length === 0}
              <div class="execution-empty font-mono">当前筛选下暂无执行任务</div>
            {/if}
          </div>
            </div>
          {/if}
        </div>

        <aside class="execution-inspector-panel" aria-label="执行追踪洞察">
          <div class="execution-inspector-head">
            <span class="summary-kicker font-mono">EXECUTION INSIGHT</span>
            <strong>{executionEvidenceRate}%</strong>
            <em>证据覆盖率</em>
          </div>

          {#if executionFocusItem}
            <div class="execution-focus-stack">
              <span class="exec-task-id font-mono">{executionFocusItem.task_id}</span>
              <h3>{executionFocusItem.title}</h3>
              <div class="execution-focus-meta">
                <span>
                  <em>结果</em>
                  <strong>{executionFocusItem.result_label}</strong>
                </span>
                <span>
                  <em>负责人</em>
                  <strong>{executionFocusItem.assignee || '未指派'}</strong>
                </span>
                <span>
                  <em>证据分</em>
                  <strong>{executionFocusItem.evidence_score}%</strong>
                </span>
              </div>
              <button class="exec-action-btn is-wide font-mono" type="button" on:click={() => {
                activeTelemetryTaskId = executionFocusItem.task_id;
                isTelemetryDrawerOpen = true;
              }}>打开代码轨迹</button>
            </div>
          {:else}
            <div class="execution-inspector-empty font-mono">暂无执行追踪数据</div>
          {/if}

          <div class="execution-inspector-stats">
            <span><em>缺失证据</em><strong>{executionSummary.missing_evidence}</strong></span>
            <span><em>陈旧任务</em><strong>{executionSummary.stale}</strong></span>
            <span><em>状态不一致</em><strong>{executionSummary.mismatch}</strong></span>
          </div>
        </aside>
      </div>
    </div>
    {/if}
  {:else}
    <div class="phase41-workbench phase41-status-workbench">
      <section class="wa-admin-section phase41-table-panel">
        <div class="wa-admin-card wa-admin-toolbar phase41-table-toolbar phase41-status-toolbar">
          <div>
            <span class="phase41-kicker">交付事项</span>
            <strong>需求与 Bug 事实表</strong>
            <small>{taskTableRows.length} 条当前工作集 · {selectedAssigneeSummary}{taskLastRefreshedAt ? ` · 更新于 ${taskLastRefreshedAt}` : ''}</small>
          </div>
          {@render taskScopeFilters()}
          <div class="phase41-toolbar-actions">
            {#if errorMsg}
              <span class="phase41-inline-error">{errorMsg}</span>
            {/if}
            <button
              type="button"
              class="wa-admin-action secondary"
              class:is-loading={taskRefreshing}
              disabled={taskRefreshing}
              on:click={refreshStatusTasks}
            >
              {taskRefreshing ? '刷新中' : '刷新'}
            </button>
            {#if selectedTaskForInspector}
              <button type="button" class="wa-admin-action primary" on:click={() => openDetails(selectedTaskForInspector)}>
                详情
              </button>
            {/if}
          </div>
        </div>

        <div class="wa-admin-table-shell phase41-table-shell" bind:this={taskTableShellEl}>
          <table class="wa-admin-table phase41-table">
            <thead>
              <tr>
                {#each taskTableColumns as column}
                  <th style="width: {column.width || 'auto'}; text-align: {column.align || 'left'}">{column.label}</th>
                {/each}
              </tr>
            </thead>
            <tbody>
              {#if loading && taskTableRows.length === 0}
                {#each Array(8) as _}
                  <tr class="phase41-skeleton-row"><td colspan={taskTableColumns.length}></td></tr>
                {/each}
              {:else if taskTableRows.length === 0}
                <tr><td colspan={taskTableColumns.length} class="phase41-empty-cell">当前项目和负责人筛选下暂无未闭环需求或 Bug</td></tr>
              {:else}
                {#each taskTableRows as row (row.id)}
                  <tr
                    class:is-selected={selectedTaskForInspector?.id === row.id}
                    data-task-id={row.id}
                    aria-selected={selectedTaskForInspector?.id === row.id}
                    tabindex="0"
                    on:click={() => selectedTaskId = row.id}
                    on:keydown={(event) => { if (event.key === 'Enter') selectedTaskId = row.id; }}
                  >
                    <td>
                      <div class="phase41-title-cell">
                        <div class="phase41-title-main">
                          {#if jiraBaseUrl && row.id && !row.id.startsWith('TASK-')}
                            <a href="{jiraBaseUrl}/browse/{row.id}" target="_blank" rel="noopener noreferrer" class="phase41-id-link" on:click|stopPropagation>{row.id}</a>
                          {:else}
                            <span class="phase41-id-link as-text">{row.id}</span>
                          {/if}
                          <strong title={row.title}>{row.title}</strong>
                        </div>
                      </div>
                    </td>
                    <td>
                      <span class="phase41-type-label">
                        <IssueTypeMark issueType={String(row.cells.issueType)} />
                        {row.cells.issueType}
                      </span>
                    </td>
                    <td>
                      <span class="phase41-project-cell" title={String(row.cells.projectLabel)}>
                        <strong>{row.cells.projectLabel}</strong>
                        {#if row.cells.project && row.cells.project !== row.cells.projectLabel}
                          <small>{row.cells.project}</small>
                        {/if}
                      </span>
                    </td>
                    <td><div class="phase41-owner-cell"><strong>{row.owner}</strong></div></td>
                    <td><span class="wa-admin-pill {ADMIN_TONE_CLASS[row.tone || 'neutral']}">{row.status}</span></td>
                    <td><span class="phase41-release-cell">{row.cells.targetRelease}</span></td>
                    <td>{row.cells.lastUpdate}</td>
                    <td class="phase41-action-cell">
                      <button
                        type="button"
                        class="wa-admin-action secondary compact"
                        on:click|stopPropagation={() => {
                          const task = sortedFilteredTasks.find(t => t.id === row.id);
                          if (task) openDetails(task);
                        }}
                      >
                        详情
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </section>

      <aside class="wa-admin-card wa-admin-inspector phase41-inspector">
        {#if taskInspector && selectedTaskForInspector}
          <div class="phase41-inspector-head">
            <span class="phase41-kicker">事项检查器</span>
            <h3>{taskInspector.title}</h3>
            <div>
              <span class="phase41-id-link as-text">{taskInspector.id}</span>
              <span class="wa-admin-pill {ADMIN_TONE_CLASS[taskInspector.tone || 'neutral']}">{taskInspector.status}</span>
            </div>
          </div>

          <dl class="phase41-fact-grid phase41-fact-pills">
            {#each taskInspector.facts as fact}
              <div class:is-type-fact={fact.label === '事项类型'}>
                <dt>{fact.label}</dt>
                <dd>
                  {#if fact.label === '事项类型'}
                    <span
                      class="phase41-type-icon {String(fact.value).toLowerCase().startsWith('bug') ? 'is-bug' : 'is-task'}"
                      role="img"
                      aria-label={String(fact.value).toLowerCase().startsWith('bug') ? 'Bug' : '需求'}
                      title={String(fact.value).toLowerCase().startsWith('bug') ? 'Bug' : '需求'}
                    >
                      {String(fact.value).toLowerCase().startsWith('bug') ? 'B' : 'R'}
                    </span>
                  {:else}
                    {fact.value}
                  {/if}
                </dd>
              </div>
            {/each}
          </dl>

          {#each taskInspector.sections as section}
            <section class="phase41-inspector-section">
              <h4>{section.title}</h4>
              {#if section.body}
                <p>{section.body}</p>
              {/if}
              {#if section.items}
                <ul>
                  {#each section.items as item}
                    <li>{item}</li>
                  {/each}
                </ul>
              {/if}
            </section>
          {/each}

          <div class="phase41-inspector-actions">
            {#each taskInspector.actions || [] as action}
              <button
                type="button"
                class="wa-admin-action {action.kind}"
                on:click={() => handleTaskInspectorAction(action, selectedTaskForInspector)}
              >
                {action.label}
              </button>
            {/each}
          </div>
        {:else}
          <div class="phase41-empty-inspector">当前筛选下暂无可查看事项</div>
        {/if}
      </aside>
    </div>

    {#if false}
    <div class="kanban-grid">
      <!-- Backlog -->
      <div class="column">
        <div class="column-header">
          <h3 class="column-title">
            <span class="dot dot-backlog"></span>
            待办任务 (Backlog)
          </h3>
          <span class="count-badge">{backlog.length}</span>
        </div>
        <div class="column-body">
          {#each backlog as task}
            <div class="task-card {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge">{getProjectName(task.id)}</span>
                </div>
                <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
              </div>
              <h4 class="task-title">{task.title}</h4>
              {#if hasParentDemand(task.taskGroupId)}
                <div class="parent-demand-badge font-mono" title={getParentDemandTitle(task.taskGroupId)}>
                  关联需求: #{getParentDemandId(task.taskGroupId)}
                </div>
              {/if}
              <div class="task-footer">
                <span>负责人 {task.assignee}</span>
                <span class="active-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)} 天</span>
                {#if getDelayDays(task.taskCreatedAt, task.status) >= 7}
                  <span class="delay-badge text-critical">延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {:else if getDelayDays(task.taskCreatedAt, task.status) >= 3}
                  <span class="delay-badge text-warning">延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>

      <!-- In Progress -->
      <div class="column">
        <div class="column-header">
          <h3 class="column-title">
            <span class="dot dot-progress animate-pulse"></span>
            进行中 (In Progress)
          </h3>
          <span class="count-badge badge-progress">{inProgress.length}</span>
        </div>
        <div class="column-body">
          {#each inProgress as task}
            {@const ev = getEvidenceStatus(task)}
            <div class="task-card card-progress {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id id-progress">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge badge-progress-sub">{getProjectName(task.id)}</span>
                </div>
                <div class="meta-right">
                  <span class="evidence-badge-tag {ev.class} font-mono">{ev.label}</span>
                  <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
                </div>
              </div>
              <h4 class="task-title text-focus">{task.title}</h4>
              {#if hasParentDemand(task.taskGroupId)}
                <div class="parent-demand-badge font-mono" title={getParentDemandTitle(task.taskGroupId)}>
                  关联需求: #{getParentDemandId(task.taskGroupId)}
                </div>
              {/if}
              {#if task.branch && task.branch !== '-'}
                <div class="telemetry-info">
                  <div class="telemetry-label">Branch</div>
                  <div class="telemetry-val">{task.branch}</div>
                  <div class="telemetry-label">Last Commit</div>
                  <div class="telemetry-val val-commit">{task.lastCommit}</div>
                </div>
              {/if}
              <div class="task-footer">
                <span class="assignee-active">负责人 {task.assignee}</span>
                <span class="active-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)} 天</span>
                {#if getDelayDays(task.taskCreatedAt, task.status) >= 7}
                  <span class="delay-badge text-critical">延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {:else if getDelayDays(task.taskCreatedAt, task.status) >= 3}
                  <span class="delay-badge text-warning">延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>

      <!-- In Review -->
      <div class="column">
        <div class="column-header">
          <h3 class="column-title">
            <span class="dot dot-review"></span>
            代码评审 (In Review)
          </h3>
          <span class="count-badge badge-review">{inReview.length}</span>
        </div>
        <div class="column-body">
          {#each inReview as task}
            {@const ev = getEvidenceStatus(task)}
            <div class="task-card card-review {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id id-review">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge badge-review-sub">{getProjectName(task.id)}</span>
                </div>
                <div class="meta-right">
                  <span class="evidence-badge-tag {ev.class} font-mono">{ev.label}</span>
                  <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
                </div>
              </div>
              <h4 class="task-title text-focus">{task.title}</h4>
              {#if hasParentDemand(task.taskGroupId)}
                <div class="parent-demand-badge font-mono" title={getParentDemandTitle(task.taskGroupId)}>
                  关联需求: #{getParentDemandId(task.taskGroupId)}
                </div>
              {/if}
              {#if task.branch && task.branch !== '-'}
                <div class="telemetry-info">
                  <div class="telemetry-label">Branch</div>
                  <div class="telemetry-val">{task.branch}</div>
                  {#if task.mrUrl && task.mrIid}
                    <div class="telemetry-label">Active MR</div>
                    <a href="{task.mrUrl}" target="_blank" rel="noopener noreferrer" class="mr-link" on:click|stopPropagation>!{task.mrIid}: Merge Request</a>
                  {/if}
                </div>
              {/if}
              <div class="task-footer">
                <span>负责人 {task.assignee}</span>
                <span class="active-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)} 天</span>
                {#if getDelayDays(task.taskCreatedAt, task.status) >= 7}
                  <span class="delay-badge text-critical">延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {:else if getDelayDays(task.taskCreatedAt, task.status) >= 3}
                  <span class="delay-badge text-warning">延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>

      <!-- Done -->
      <div class="column">
        <div class="column-header">
          <h3 class="column-title">
            <span class="dot dot-done"></span>
            已完成 (Done)
          </h3>
          <span class="count-badge">{done.length}</span>
        </div>
        <div class="column-body">
          {#each done as task}
            {@const ev = getEvidenceStatus(task)}
            <div class="task-card card-done" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id id-done">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge">{getProjectName(task.id)}</span>
                </div>
                <div class="meta-right">
                  <span class="evidence-badge-tag {ev.class} font-mono">{ev.label}</span>
                  <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
                </div>
              </div>
              <h4 class="task-title title-done">{task.title}</h4>
              {#if hasParentDemand(task.taskGroupId)}
                <div class="parent-demand-badge font-mono" title={getParentDemandTitle(task.taskGroupId)}>
                  关联需求: #{getParentDemandId(task.taskGroupId)}
                </div>
              {/if}
              <div class="task-footer">
                <span>负责人 {task.assignee}</span>
                <span class="active-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)} 天</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>
    {/if}
  {/if}
</section>

{#if showDetails && selectedTask}
  <Modal
    show={showDetails}
    title={selectedTask.title}
    size="wide"
    closeLabel="关闭事项详情"
    on:close={closeDetails}
  >
    <div class="task-detail-surface task-detail-modal-body">
      <div class="task-detail-modal-meta">
        <span class="phase41-kicker">{getIssueTypeLabel(selectedTask)}详情</span>
        {#if jiraBaseUrl && selectedTask.id && !selectedTask.id.startsWith('TASK-')}
          <a href="{jiraBaseUrl}/browse/{selectedTask.id}" target="_blank" rel="noopener noreferrer" class="phase41-id-link">
            {selectedTask.id}
          </a>
        {:else}
          <strong class="phase41-id-link as-text">{selectedTask.id}</strong>
        {/if}
        <span class="wa-admin-pill {ADMIN_TONE_CLASS[toneForStatus(selectedTask.status)]}">{getTaskStatusShortLabel(selectedTask)}</span>
        <span class="wa-admin-pill tone-neutral">{getPlanningStateLabel(selectedTask.planningState)}</span>
      </div>

      <section class="task-detail-overview" aria-labelledby="task-detail-facts-title">
        <div class="task-detail-section-heading">
          <h3 id="task-detail-facts-title">事项概览</h3>
        </div>
        <dl class="task-detail-fact-grid">
          <div><dt>事项类型</dt><dd>{getIssueTypeLabel(selectedTask)}</dd></div>
          <div><dt>所属项目</dt><dd>{selectedTask.projectKey ? (projectNamesMap[selectedTask.projectKey.toUpperCase()] || selectedTask.projectKey) : '未归项目'}</dd></div>
          <div><dt>负责人</dt><dd>{selectedTask.assignee || '未指派'}</dd></div>
          <div><dt>当前状态</dt><dd>{getTaskStatusShortLabel(selectedTask)}</dd></div>
          <div><dt>计划状态</dt><dd>{getPlanningStateLabel(selectedTask.planningState)}</dd></div>
          <div><dt>目标版本</dt><dd>{selectedTask.targetRelease || '未归版本'}</dd></div>
          <div><dt>截止日期</dt><dd>{formatTaskDate(selectedTask.dueDate)}</dd></div>
          <div><dt>同步状态</dt><dd>{getSyncStateLabel(selectedTask.syncState)}</dd></div>
        </dl>
      </section>

      <div class="task-detail-section-grid">
        <section class="task-detail-panel">
          <h3>事项描述</h3>
          <p>{selectedTask.description || '当前事项暂无描述。'}</p>
        </section>
        <section class="task-detail-panel">
          <h3>版本归属</h3>
          <ul>
            <li>目标版本：{selectedTask.targetRelease || '未归版本'}</li>
            <li>影响版本：{selectedTask.affectedReleases?.length ? selectedTask.affectedReleases.join('、') : '未记录'}</li>
          </ul>
        </section>
        <section class="task-detail-panel">
          <h3>来源与同步</h3>
          <ul>
            <li>数据来源：{selectedTask.source || '本地'}</li>
            <li>同步状态：{getSyncStateLabel(selectedTask.syncState)}</li>
            <li>数据修订：{selectedTask.revision ?? 0}</li>
          </ul>
        </section>
        <section class="task-detail-panel">
          <h3>{selectedTask.issueType === 'task' ? '执行上下文' : '时间记录'}</h3>
          {#if selectedTask.issueType === 'task'}
            <ul>
              <li>仓库：{selectedTask.repo || '-'}</li>
              <li>分支：{selectedTask.branch || '-'}</li>
              <li>最新提交：{selectedTask.lastCommit || '-'}</li>
            </ul>
          {:else}
            <ul>
              <li>创建时间：{formatTimeFull(selectedTask.taskCreatedAt)}</li>
              <li>最后更新：{formatTimeFull(selectedTask.rawLastUpdate)}</li>
              <li>截止日期：{formatTaskDate(selectedTask.dueDate)}</li>
            </ul>
          {/if}
        </section>
      </div>
    </div>

    <div slot="footer" class="task-detail-modal-actions">
      {#if completionConfirming && selectedTask.issueType !== 'task' && !isTaskDone(selectedTask)}
        <div class="task-completion-confirmation" role="group" aria-labelledby="task-completion-title">
          <div class="task-completion-copy">
            <strong id="task-completion-title">确认已完成 {selectedTask.id}？</strong>
            <span>系统会核对已捕获的 Commit 或已合并 MR，完成不要求绑定目标版本。</span>
            {#if completionError}
              <p class="task-completion-error" role="alert">{completionError}</p>
            {/if}
          </div>
          <div class="task-completion-actions">
            <button
              class="wa-admin-action secondary"
              type="button"
              disabled={completionSubmitting}
              on:click={() => { completionConfirming = false; completionError = ''; }}
            >
              取消
            </button>
            <button
              class="wa-admin-action primary"
              type="button"
              disabled={completionSubmitting}
              aria-busy={completionSubmitting}
              on:click={completeSelectedTask}
            >
              {completionSubmitting ? '提交中...' : '提交完成'}
            </button>
          </div>
        </div>
      {:else}
        {#if selectedTask.issueType !== 'task' && !isTaskDone(selectedTask)}
          <button class="wa-admin-action primary" type="button" on:click={() => { completionConfirming = true; completionError = ''; }}>
            确认完成
          </button>
        {/if}
        {#if selectedTask.issueType !== 'task'}
          <button class="wa-admin-action secondary" type="button" on:click={() => onOpenDeliveryPlan(selectedTask?.id || '')}>
            打开版本计划
          </button>
        {/if}
        {#if jiraBaseUrl && selectedTask.id && !selectedTask.id.startsWith('TASK-')}
          <a href="{jiraBaseUrl}/browse/{selectedTask.id}" target="_blank" rel="noopener noreferrer" class="wa-admin-action secondary">
            在 Jira 打开
          </a>
        {/if}
      {/if}
    </div>
  </Modal>
{/if}

<Modal
  show={isTelemetryDrawerOpen}
  title="代码提交轨迹"
  size="wide"
  closeLabel="关闭代码提交轨迹"
  on:close={() => isTelemetryDrawerOpen = false}
>
  <CommitTelemetryPanel
    taskID={activeTelemetryTaskId}
    isOpen={isTelemetryDrawerOpen}
    presentation="modal"
    embeddedInModal={true}
    onClose={() => isTelemetryDrawerOpen = false}
  />
</Modal>

<style>
  .active-days-badge {
    font-size: 0.75rem;
    color: #94a3b8;
    background: rgba(148, 163, 184, 0.1);
    border: 1px solid rgba(148, 163, 184, 0.25);
    padding: 1px 6px;
    border-radius: 4px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .compact-days-badge {
    font-size: 0.65rem;
    color: #64748b;
    background: rgba(100, 116, 139, 0.1);
    border: 1px solid rgba(100, 116, 139, 0.2);
    padding: 1px 4px;
    border-radius: 3px;
    margin-left: auto;
    display: inline-flex;
    align-items: center;
  }

  .kanban-section {
    margin-bottom: 32px;
  }

  .execution-workbench {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .execution-summary-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(130px, 1fr));
    gap: 10px;
  }

  .execution-summary-cell {
    min-height: 88px;
    background: rgba(10, 15, 30, 0.66);
    border: 1px solid rgba(51, 65, 85, 0.32);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .execution-summary-cell.is-red { border-color: rgba(239, 68, 68, 0.48); }
  .execution-summary-cell.is-amber { border-color: rgba(245, 158, 11, 0.48); }
  .execution-summary-cell.is-blue { border-color: rgba(56, 189, 248, 0.48); }
  .execution-summary-cell.is-violet { border-color: rgba(167, 139, 250, 0.48); }

  .summary-kicker {
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 900;
    letter-spacing: 0.06em;
  }

  .execution-summary-cell strong {
    color: #f8fafc;
    font-size: 1.45rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .execution-summary-cell em {
    color: #94a3b8;
    font-size: 0.72rem;
    font-style: normal;
  }

  .execution-control-panel {
    display: grid;
    grid-template-columns: minmax(220px, 1.1fr) minmax(300px, 1.6fr) minmax(150px, 0.7fr) auto;
    align-items: center;
    gap: 10px;
    background: rgba(10, 15, 30, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 10px;
    padding: 10px;
  }

  .execution-assignee-select {
    width: 100%;
    min-width: 0;
    justify-self: stretch;
  }

  .execution-assignee-select .custom-select-trigger {
    width: 100%;
    height: 38px;
    box-sizing: border-box;
    justify-content: space-between;
  }

  .execution-assignee-select .custom-select-options {
    right: 0;
    left: auto;
    width: 100%;
    min-width: 100%;
    box-sizing: border-box;
  }

  .execution-assignee-select .trigger-label {
    min-width: 0;
    flex: 1;
  }

  .execution-search-shell {
    position: relative;
    display: flex;
    align-items: center;
  }

  .execution-search-shell input {
    width: 100%;
    height: 38px;
    box-sizing: border-box;
    background: rgba(15, 23, 42, 0.74);
    border: 1px solid rgba(71, 85, 105, 0.68);
    border-radius: 8px;
    color: #e2e8f0;
    outline: none;
    padding: 0 12px 0 34px;
    font-size: 0.78rem;
  }

  .execution-search-shell input:focus {
    border-color: rgba(129, 140, 248, 0.78);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.15);
  }

  .execution-search-mark {
    position: absolute;
    left: 12px;
    width: 11px;
    height: 11px;
    border: 2px solid #64748b;
    border-radius: 50%;
  }

  .execution-search-mark::after {
    content: "";
    position: absolute;
    width: 6px;
    height: 2px;
    right: -5px;
    bottom: -3px;
    background: #64748b;
    transform: rotate(45deg);
    border-radius: 2px;
  }

  .execution-risk-strip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: rgba(2, 6, 23, 0.32);
    border: 1px solid rgba(51, 65, 85, 0.38);
    border-radius: 8px;
    padding: 4px;
    overflow-x: auto;
  }

  .execution-risk-strip button {
    border: none;
    background: transparent;
    color: #94a3b8;
    border-radius: 6px;
    padding: 7px 11px;
    font-size: 0.74rem;
    font-weight: 800;
    cursor: pointer;
    transition: background 0.16s ease, color 0.16s ease;
    white-space: nowrap;
  }

  .execution-risk-strip button.active {
    color: #f8fafc;
    background: rgba(99, 102, 241, 0.48);
  }

  .execution-refresh-btn {
    height: 38px;
    border: 1px solid rgba(56, 189, 248, 0.35);
    background: rgba(14, 165, 233, 0.12);
    color: #7dd3fc;
    border-radius: 8px;
    padding: 0 13px;
    cursor: pointer;
    font-size: 0.7rem;
    font-weight: 900;
  }

  .execution-refresh-btn:hover {
    background: rgba(14, 165, 233, 0.2);
  }

  .execution-refresh-btn.is-loading {
    opacity: 0.65;
    cursor: wait;
  }

  .execution-state {
    padding: 70px 0;
    text-align: center;
    color: #64748b;
    font-size: 0.85rem;
  }

  .execution-state.error {
    color: #f87171;
  }

  .execution-table-panel {
    background: rgba(10, 15, 30, 0.68);
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 12px;
    overflow: hidden;
  }

  .execution-table-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 14px;
    padding: 14px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.32);
  }

  .execution-table-head h3 {
    margin: 0;
    color: #f8fafc;
    font-size: 1rem;
  }

  .execution-count {
    color: #94a3b8;
    font-size: 0.7rem;
  }

  .execution-table-wrapper {
    max-height: 620px;
    overflow: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.48) rgba(15, 23, 42, 0.46);
  }

  .execution-table-wrapper::-webkit-scrollbar {
    width: 10px;
    height: 10px;
  }

  .execution-table-wrapper::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.52);
    border-radius: 999px;
  }

  .execution-table-wrapper::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(129, 140, 248, 0.62), rgba(56, 189, 248, 0.5));
    border: 2px solid rgba(11, 18, 32, 0.96);
    border-radius: 999px;
  }

  .execution-table-wrapper::-webkit-scrollbar-thumb:hover {
    background: linear-gradient(180deg, rgba(165, 180, 252, 0.78), rgba(125, 211, 252, 0.68));
  }

  .execution-table-wrapper::-webkit-scrollbar-corner {
    background: rgba(15, 23, 42, 0.72);
  }

  .exec-grid-table {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 1120px;
    background: #0b0f19;
  }

  .exec-grid-thead {
    position: sticky;
    top: 0;
    z-index: 3;
    background: #0b1220;
    border-bottom: 1px solid rgba(71, 85, 105, 0.48);
  }

  .exec-grid-tr {
    display: grid;
    grid-template-columns: 3fr 1.5fr 1.5fr 3fr 1fr;
    align-items: center;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }

  .exec-grid-th {
    color: #64748b;
    font-size: 0.66rem;
    font-weight: 900;
    letter-spacing: 0.04em;
    padding: 10px 12px;
    user-select: none;
    text-align: left;
  }

  .exec-grid-td {
    padding: 12px;
    font-size: 0.78rem;
    color: #cbd5e1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .exec-grid-td.col-action,
  .exec-grid-th.col-action {
    border-left: 1px solid rgba(99, 102, 241, 0.18);
    justify-content: center;
    text-align: center;
  }

  .exec-grid-tr:hover {
    background: rgba(30, 41, 59, 0.22);
  }

  .exec-task-stack,
  .exec-demand-stack,
  .exec-evidence-stack,
  .exec-result-stack,
  .exec-risk-stack,
  .exec-activity-stack {
    display: flex;
    flex-direction: column;
    gap: 5px;
    min-width: 0;
  }

  .exec-task-stack strong,
  .exec-demand-stack strong,
  .exec-evidence-stack strong,
  .exec-activity-stack strong {
    color: #f8fafc;
    line-height: 1.32;
    word-break: break-word;
  }

  .exec-task-stack small,
  .exec-demand-stack small,
  .exec-evidence-stack small,
  .exec-result-stack small,
  .exec-risk-stack small,
  .exec-activity-stack small {
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.4;
    word-break: break-word;
  }

  .exec-id-row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .exec-task-id {
    color: #818cf8;
    font-size: 0.66rem;
    font-weight: 900;
  }

  .exec-type-badge,
  .bound-pill,
  .orphan-pill,
  .result-pill,
  .exec-risk-pill {
    width: fit-content;
    border-radius: 6px;
    padding: 3px 7px;
    font-size: 0.66rem;
    font-weight: 900;
    border: 1px solid rgba(100, 116, 139, 0.28);
    color: #cbd5e1;
    background: rgba(100, 116, 139, 0.12);
  }

  .exec-type-badge.type-bug {
    color: #fecaca;
    border-color: rgba(248, 113, 113, 0.35);
    background: rgba(239, 68, 68, 0.12);
  }

  .exec-type-badge.type-task {
    color: #bfdbfe;
    border-color: rgba(96, 165, 250, 0.35);
    background: rgba(59, 130, 246, 0.12);
  }

  .exec-link,
  .exec-mr-link {
    color: #7dd3fc;
    text-decoration: none;
    font-size: 0.68rem;
    font-weight: 800;
  }

  .exec-link:hover,
  .exec-mr-link:hover {
    text-decoration: underline;
  }

  .bound-pill {
    color: #bbf7d0;
    border-color: rgba(74, 222, 128, 0.32);
    background: rgba(16, 185, 129, 0.1);
  }

  .orphan-pill {
    color: #fed7aa;
    border-color: rgba(251, 146, 60, 0.36);
    background: rgba(249, 115, 22, 0.12);
  }

  .evidence-score {
    height: 6px;
    width: 100%;
    min-width: 80px;
    background: rgba(51, 65, 85, 0.72);
    border-radius: 999px;
    overflow: hidden;
  }

  .evidence-score span {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, #38bdf8, #818cf8);
    border-radius: inherit;
  }

  .evidence-score-bar {
    position: relative;
    height: 16px;
    background: rgba(51, 65, 85, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
  }

  .evidence-score-fill {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    background: #38bdf8;
    border-radius: 8px;
  }

  .evidence-score-text {
    position: relative;
    z-index: 1;
    font-size: 0.68rem;
    font-weight: 800;
    color: #ffffff;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.7);
  }

  .exec-link-only-id {
    color: #38bdf8;
    text-decoration: none;
    cursor: pointer;
    transition: color 0.2s;
  }

  .exec-link-only-id:hover {
    color: #60a5fa;
    text-decoration: underline;
  }

  .exec-assignee-cell {
    display: grid;
    gap: 5px;
    color: #cbd5e1;
    font-size: 0.78rem;
  }

  .exec-assignee-cell strong {
    color: #e5edf8;
    line-height: 1.25;
  }

  .exec-assignee-source,
  .exec-assignee-note {
    display: inline-flex;
    width: fit-content;
    align-items: center;
    border-radius: 999px;
    font-size: 0.66rem;
    line-height: 1;
  }

  .exec-assignee-source {
    border: 1px solid rgba(56, 189, 248, 0.24);
    padding: 4px 7px;
    color: #bae6fd;
    background: rgba(14, 165, 233, 0.1);
  }

  .exec-assignee-note {
    color: #8393a8;
  }

  .result-merged_done,
  .result-jira_done {
    color: #bbf7d0;
    border-color: rgba(74, 222, 128, 0.32);
    background: rgba(16, 185, 129, 0.1);
  }

  .result-merged_waiting_jira {
    color: #fecaca;
    border-color: rgba(248, 113, 113, 0.4);
    background: rgba(239, 68, 68, 0.14);
  }

  .result-mr_active {
    color: #fde68a;
    border-color: rgba(234, 179, 8, 0.34);
    background: rgba(234, 179, 8, 0.1);
  }

  .result-coding {
    color: #bae6fd;
    border-color: rgba(56, 189, 248, 0.32);
    background: rgba(14, 165, 233, 0.11);
  }

  .risk-high {
    color: #fecaca;
    background: rgba(239, 68, 68, 0.14);
    border-color: rgba(248, 113, 113, 0.4);
  }

  .risk-medium {
    color: #fed7aa;
    background: rgba(249, 115, 22, 0.12);
    border-color: rgba(251, 146, 60, 0.36);
  }

  .risk-safe {
    color: #bbf7d0;
    background: rgba(16, 185, 129, 0.1);
    border-color: rgba(74, 222, 128, 0.32);
  }

  .risk-done {
    color: #cbd5e1;
    background: rgba(100, 116, 139, 0.12);
    border-color: rgba(100, 116, 139, 0.28);
  }

  .execution-empty {
    padding: 34px;
    text-align: center;
    color: #64748b;
    border-top: 1px solid rgba(51, 65, 85, 0.3);
    font-size: 0.72rem;
  }

  @media (max-width: 1180px) {
    .execution-summary-grid {
      grid-template-columns: repeat(3, minmax(130px, 1fr));
    }

    .execution-control-panel {
      grid-template-columns: minmax(220px, 1fr) minmax(280px, 1.4fr);
    }
  }

  @media (max-width: 760px) {
    .execution-summary-grid,
    .execution-control-panel {
      grid-template-columns: 1fr;
    }
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
    flex-wrap: wrap;
    gap: 12px;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .section-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: #e2e8f0;
    margin: 0;
  }

  .sync-badge {
    font-size: 0.75rem;
    font-weight: 600;
    background: rgba(16, 185, 129, 0.1);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.2);
    padding: 2px 10px;
    border-radius: 9999px;
  }

  .header-right {
    font-size: 0.75rem;
    color: #64748b;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .kanban-grid {
    display: grid;
    grid-template-columns: repeat(1, minmax(0, 1fr));
    gap: 24px;
  }

  @media (min-width: 1024px) {
    .kanban-grid {
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }
  }

  .column {
    background: rgba(15, 23, 42, 0.35);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 12px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    min-height: 400px;
    box-sizing: border-box;
  }

  .column-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    padding-bottom: 8px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
  }

  .column-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: #cbd5e1;
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
  }

  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .dot-backlog { background-color: #64748b; }
  .dot-progress { background-color: #6366f1; box-shadow: 0 0 8px #6366f1;}
  .dot-review { background-color: #f59e0b; }
  .dot-done { background-color: #10b981; }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.5; transform: scale(0.9); }
  }

  .animate-pulse {
    animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
  }

  .count-badge {
    font-size: 0.75rem;
    font-weight: 700;
    background: #1e293b;
    color: #94a3b8;
    padding: 2px 8px;
    border-radius: 4px;
  }

  .badge-progress {
    background: rgba(99, 102, 241, 0.1);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.2);
  }

  .badge-review {
    background: rgba(245, 158, 11, 0.1);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.2);
  }

  .column-body {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
    max-height: 520px;
    overflow-y: auto;
    padding-right: 4px;
    box-sizing: border-box;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.2) transparent;
  }

  .column-body::-webkit-scrollbar {
    width: 4px;
  }

  .column-body::-webkit-scrollbar-track {
    background: transparent;
  }

  .column-body::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 4px;
  }

  .column-body::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.5);
  }

  .task-card {
    background: rgba(17, 24, 39, 0.8);
    border: 1px solid rgba(51, 65, 85, 0.4);
    padding: 16px;
    border-radius: 8px;
    transition: all 0.2s ease-in-out;
  }

  .task-card:hover {
    border-color: rgba(148, 163, 184, 0.4);
    transform: translateY(-1px);
  }

  .card-progress {
    border-color: rgba(99, 102, 241, 0.25);
    box-shadow: 0 4px 12px -5px rgba(99, 102, 241, 0.1);
  }
  .card-progress:hover {
    border-color: rgba(99, 102, 241, 0.5);
  }

  .card-review {
    border-color: rgba(245, 158, 11, 0.2);
  }
  .card-review:hover {
    border-color: rgba(245, 158, 11, 0.45);
  }

  .card-done {
    opacity: 0.65;
  }
  .card-done:hover {
    opacity: 1.0;
  }

  .task-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .task-id {
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-weight: 700;
    color: #64748b;
    background: #0f172a;
    border: 1px solid rgba(51, 65, 85, 0.5);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .id-progress {
    color: #818cf8;
    background: rgba(99, 102, 241, 0.1);
    border-color: rgba(99, 102, 241, 0.2);
  }

  .id-review {
    color: #fbbf24;
    background: rgba(245, 158, 11, 0.1);
    border-color: rgba(245, 158, 11, 0.2);
  }

  .id-done {
    text-decoration: line-through;
  }

  .task-repo {
    font-size: 0.75rem;
    color: #64748b;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .task-title {
    font-size: 0.875rem;
    font-weight: 500;
    color: #cbd5e1;
    margin: 0 0 12px 0;
    line-height: 1.4;
  }

  .text-focus {
    color: #f1f5f9;
  }

  .title-done {
    color: #64748b;
    text-decoration: line-through;
  }

  .telemetry-info {
    background: rgba(2, 6, 23, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.3);
    padding: 8px 10px;
    border-radius: 6px;
    margin-bottom: 12px;
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    overflow-x: hidden;
  }

  .telemetry-label {
    font-size: 0.6rem;
    text-transform: uppercase;
    font-weight: 700;
    letter-spacing: 0.05em;
    margin-bottom: 2px;
  }

  .card-progress .telemetry-label {
    color: rgba(129, 140, 248, 0.8);
  }

  .card-review .telemetry-label {
    color: rgba(251, 191, 36, 0.8);
  }

  .telemetry-val {
    color: #e2e8f0;
    margin-bottom: 6px;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
  }

  .val-commit {
    margin-bottom: 0;
  }

  .mr-link {
    color: #818cf8;
    text-decoration: none;
  }

  .mr-link:hover {
    text-decoration: underline;
  }

  .task-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 8px;
    border-top: 1px solid rgba(51, 65, 85, 0.2);
    font-size: 0.7rem;
    color: #64748b;
  }

  .card-progress .task-footer {
    color: #94a3b8;
  }

  .assignee-active {
    font-weight: 500;
  }

  .time-active {
    color: #34d399;
  }

  .time-review {
    color: #fbbf24;
  }

  /* Clickable task cards */
  .task-card {
    cursor: pointer;
    outline: none;
  }
  
  .task-card:focus-visible {
    box-shadow: 0 0 0 2px #6366f1;
  }

  /* Filters styling */
  .header-right-filters {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
  }

  .header-desc {
    font-size: 0.75rem;
    color: #64748b;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    margin-right: 8px;
  }

  .filter-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .filter-icon {
    font-size: 0.95rem;
  }

  /* Custom Select Dropdowns */
  .custom-select-container {
    position: relative;
    display: inline-block;
  }

  .custom-select-trigger {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.25);
    color: #cbd5e1;
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
    transition: all 0.2s;
    outline: none;
    user-select: none;
  }

  .custom-select-trigger:hover, .custom-select-trigger:focus {
    border-color: #6366f1;
    background-color: rgba(30, 41, 59, 0.8);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.2);
  }

  .trigger-label {
    min-width: 90px;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .select-arrow {
    font-size: 0.6rem;
    color: #818cf8;
    transition: transform 0.2s;
  }

  .custom-select-options {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    min-width: 140px;
    background: rgba(15, 23, 42, 0.95);
    border: 1px solid rgba(129, 140, 248, 0.25);
    border-radius: 8px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5), 0 0 15px rgba(99, 102, 241, 0.1);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    z-index: 100;
    overflow-y: auto;
    max-height: 240px;
    display: flex;
    flex-direction: column;
    padding: 4px;
    animation: dropdownFadeIn 0.15s cubic-bezier(0.16, 1, 0.3, 1);
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.25) transparent;
  }

  .custom-select-options::-webkit-scrollbar {
    width: 4px;
  }

  .custom-select-options::-webkit-scrollbar-track {
    background: transparent;
  }

  .custom-select-options::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 4px;
  }

  .custom-select-options::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.5);
  }

  .custom-option {
    background: transparent;
    border: none;
    color: #94a3b8;
    padding: 6px 12px;
    text-align: left;
    font-size: 0.8rem;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
    outline: none;
    white-space: nowrap;
  }

  .custom-option:hover {
    background: rgba(99, 102, 241, 0.15);
    color: #ffffff;
  }

  .custom-option.active {
    background: rgba(99, 102, 241, 0.25);
    color: #a5b4fc;
    font-weight: 600;
  }

  @keyframes dropdownFadeIn {
    from { transform: translateY(-4px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }

  /* Delay Highlighting Styles */
  .delay-warning {
    border-color: rgba(245, 158, 11, 0.55) !important;
    box-shadow: 0 4px 14px -5px rgba(245, 158, 11, 0.15), 0 0 0 1px rgba(245, 158, 11, 0.2) !important;
  }
  .delay-warning:hover {
    border-color: rgba(245, 158, 11, 0.8) !important;
  }

  .delay-critical {
    border-color: rgba(239, 68, 68, 0.6) !important;
    box-shadow: 0 4px 16px -5px rgba(239, 68, 68, 0.2), 0 0 0 1px rgba(239, 68, 68, 0.25) !important;
    background: rgba(239, 68, 68, 0.02) !important;
  }
  .delay-critical:hover {
    border-color: rgba(239, 68, 68, 0.85) !important;
  }

  .delay-badge {
    font-weight: 700;
    font-size: 0.7rem;
  }
  .text-warning {
    color: #fbbf24;
  }
  .text-critical {
    color: #f87171;
  }

  /* Issue Type Badges */
  .issue-type-badge {
    font-size: 0.6rem;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 4px;
    text-transform: uppercase;
  }

  .issue-type-badge.type-bug {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.25);
  }

  .issue-type-badge.type-task {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border: 1px solid rgba(59, 130, 246, 0.25);
  }

  /* Meta layout & Project badges */
  .meta-left {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .project-badge {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 4px;
    background: rgba(148, 163, 184, 0.1);
    color: #94a3b8;
    border: 1px solid rgba(148, 163, 184, 0.15);
  }

  .badge-progress-sub {
    background: rgba(99, 102, 241, 0.1);
    color: #818cf8;
    border-color: rgba(99, 102, 241, 0.15);
  }

  .badge-review-sub {
    background: rgba(245, 158, 11, 0.1);
    color: #fbbf24;
    border-color: rgba(245, 158, 11, 0.15);
  }

  /* Details Modal Content Styling */
  .details-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .details-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    font-size: 0.875rem;
    padding-bottom: 8px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
  }

  .details-row .label {
    color: #64748b;
    font-weight: 500;
    min-width: 110px;
    text-align: left;
  }

  .details-row .value {
    color: #e2e8f0;
    text-align: right;
    max-width: 70%;
    word-break: break-all;
  }

  .value.title-val {
    font-weight: 600;
    color: #f1f5f9;
  }

  .value.branch-name, .value.commit-msg {
    color: #818cf8;
  }

  .badge-project {
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
    padding: 2px 8px;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 600;
    border: 1px solid rgba(99, 102, 241, 0.25);
  }

  .divider {
    height: 1px;
    background: rgba(51, 65, 85, 0.4);
    margin: 8px 0;
  }

  .section-title {
    font-size: 0.75rem;
    font-weight: 700;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 4px;
    text-align: left;
  }

  .status-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    margin-right: 6px;
  }

  .status-name {
    font-weight: 600;
  }

  .status-dot.dot-backlog { background-color: #64748b; }
  .status-dot.dot-progress { background-color: #6366f1; box-shadow: 0 0 6px #6366f1; }
  .status-dot.dot-review { background-color: #f59e0b; }
  .status-dot.dot-done { background-color: #10b981; }

  /* Jira Direct Link Styles */
  .jira-direct-link {
    color: #64748b;
    text-decoration: none;
    font-size: 0.75rem;
    margin-left: 2px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s;
  }
  .jira-direct-link:hover {
    color: #818cf8;
  }

  .jira-modal-direct-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.3);
    color: #a5b4fc;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 600;
    text-decoration: none;
    margin-left: 8px;
    transition: all 0.2s;
    vertical-align: middle;
  }
  .jira-modal-direct-btn:hover {
    background: rgba(99, 102, 241, 0.3);
    border-color: #6366f1;
    color: #ffffff;
  }

  /* Segmented View Toggle Styles */
  .view-toggle {
    display: inline-flex;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.15);
    border-radius: 8px;
    padding: 2px;
    gap: 2px;
  }

  .toggle-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    padding: 6px 12px;
    font-size: 0.8rem;
    font-weight: 600;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
    outline: none;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .toggle-btn:hover {
    color: #ffffff;
  }

  .toggle-btn.active {
    background: rgba(99, 102, 241, 0.25);
    color: #a5b4fc;
    box-shadow: 0 2px 8px -2px rgba(99, 102, 241, 0.3);
  }

  /* 顶层效能统计面板 styling */
  .kanban-metrics {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }

  @media (min-width: 640px) {
    .kanban-metrics {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (min-width: 1024px) {
    .kanban-metrics {
      grid-template-columns: repeat(5, minmax(0, 1fr));
    }
  }

  .metric-card {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 12px;
    padding: 16px;
    display: flex;
    align-items: center;
    gap: 14px;
    transition: all 0.2s ease-in-out;
  }

  .metric-card:hover {
    border-color: rgba(99, 102, 241, 0.35);
    background: rgba(15, 23, 42, 0.5);
    transform: translateY(-1px);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  }

  .metric-icon {
    font-size: 1.5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 42px;
    height: 42px;
    border-radius: 10px;
    background: rgba(30, 41, 59, 0.6);
  }

  .metric-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .metric-label {
    font-size: 0.75rem;
    font-weight: 500;
    color: #94a3b8;
  }

  .metric-value {
    font-size: 1.25rem;
    font-weight: 700;
    color: #e2e8f0;
  }

  /* 颜色物性 */
  .text-progress-color {
    color: #818cf8;
    text-shadow: 0 0 10px rgba(99, 102, 241, 0.2);
  }

  .text-review-color {
    color: #fbbf24;
    text-shadow: 0 0 10px rgba(245, 158, 11, 0.2);
  }

  .text-done-color {
    color: #34d399;
    text-shadow: 0 0 10px rgba(16, 185, 129, 0.2);
  }

  .text-critical-color {
    color: #f87171;
    text-shadow: 0 0 10px rgba(239, 68, 68, 0.2);
  }

  .overdue-card:hover {
    border-color: rgba(239, 68, 68, 0.35);
  }

  /* 泳道负荷/延期徽章 styling */
  .swimlane-badge {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 4px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .badge-warning {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .badge-danger {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.3);
    box-shadow: 0 0 8px rgba(239, 68, 68, 0.1);
  }

  /* 批量控制按钮已废弃以遵循智能自适应极简规范 */

  /* 个人状态总览 styling */
  .assignee-status-overview {
    display: flex;
    gap: 8px;
    margin-left: auto;
    margin-right: 16px;
  }

  .status-summary-item {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 9999px;
    background: rgba(30, 41, 59, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.2);
  }

  /* 交互式泳道头部 styling */
  .interactive-header {
    cursor: pointer;
    user-select: none;
    transition: background 0.2s ease;
  }

  .interactive-header:hover {
    background: rgba(51, 65, 85, 0.15);
  }

  .collapse-chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #6366f1;
    margin-right: 4px;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .collapse-chevron.is-collapsed {
    transform: rotate(-90deg);
  }

  .swimlane.collapsed {
    border-color: rgba(51, 65, 85, 0.2);
    background: rgba(15, 23, 42, 0.15);
  }

  .swimlane.collapsed .swimlane-header {
    border-bottom: none;
    padding-bottom: 0;
  }

  .swimlane {
    background: rgba(15, 23, 42, 0.3);
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 12px;
    padding: 16px;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .swimlane-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    padding-bottom: 8px;
  }

  .assignee-info {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  .assignee-avatar {
    font-size: 1.1rem;
  }

  .assignee-name {
    font-size: 0.95rem;
    font-weight: 700;
    color: #e2e8f0;
  }

  .assignee-count {
    font-size: 0.7rem;
    font-weight: 600;
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
    padding: 2px 8px;
    border-radius: 9999px;
  }

  .swimlane-grid {
    display: grid;
    grid-template-columns: repeat(1, minmax(0, 1fr));
    gap: 16px;
  }

  @media (min-width: 1024px) {
    .swimlane-grid {
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }
  }

  .swimlane-column {
    background: rgba(15, 23, 42, 0.2);
    border: 1px solid rgba(51, 65, 85, 0.2);
    border-radius: 8px;
    padding: 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
  }

  .swimlane-column-header {
    font-size: 0.75rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding-bottom: 6px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.15);
  }

  .text-backlog { color: #94a3b8; }
  .text-progress { color: #818cf8; }
  .text-review { color: #fbbf24; }
  .text-done { color: #34d399; }

  .swimlane-column-body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 50px;
    max-height: 240px; /* 限制垂直内容侵占 */
    overflow-y: auto;
    padding-right: 4px;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.2) transparent;
  }

  .swimlane-column-body::-webkit-scrollbar {
    width: 4px;
  }

  .swimlane-column-body::-webkit-scrollbar-track {
    background: transparent;
  }

  .swimlane-column-body::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 4px;
  }

  .swimlane-column-body::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.5);
  }

  .empty-placeholder {
    font-size: 0.7rem;
    color: #475569;
    text-align: center;
    padding: 12px 0;
    font-style: italic;
  }

  /* Compact Task Card */
  .compact-task-card {
    background: rgba(17, 24, 39, 0.85);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 6px;
    padding: 8px 10px;
    cursor: pointer;
    transition: all 0.15s ease-in-out;
    display: flex;
    flex-direction: column;
    gap: 4px;
    box-sizing: border-box;
  }

  .compact-task-card:hover {
    border-color: rgba(148, 163, 184, 0.35);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  }

  .compact-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .compact-title {
    margin: 0;
    font-size: 0.75rem;
    font-weight: 500;
    color: #cbd5e1;
    line-height: 1.4;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    text-align: left;
  }

  .empty-view {
    text-align: center;
    padding: 48px;
    color: #475569;
    font-size: 0.95rem;
  }

  .text-bug {
    color: #ef4444 !important;
  }

  .loading-commits {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 16px 0;
    color: #94a3b8;
    font-size: 0.85rem;
  }

  .empty-commits-info {
    padding: 16px;
    background: rgba(30, 41, 59, 0.4);
    border: 1px solid rgba(71, 85, 105, 0.3);
    border-radius: 8px;
    color: #94a3b8;
    font-size: 0.85rem;
    text-align: center;
  }

  .commit-timeline {
    position: relative;
    padding-left: 20px;
    margin-top: 16px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    border-left: 1px solid rgba(71, 85, 105, 0.4);
  }

  .timeline-item {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .timeline-item::before {
    content: '';
    position: absolute;
    left: -25px;
    top: 6px;
    width: 8px;
    height: 8px;
    background: #020617;
    border: 2px solid #06b6d4;
    border-radius: 50%;
    z-index: 10;
  }

  .timeline-badge-container {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .timeline-badge {
    padding: 2px 6px;
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    border-radius: 4px;
    letter-spacing: 0.5px;
  }

  .badge-git_push {
    background: rgba(6, 182, 212, 0.15);
    border: 1px solid rgba(6, 182, 212, 0.35);
    color: #06b6d4;
  }

  .badge-mr_open {
    background: rgba(244, 63, 94, 0.15);
    border: 1px solid rgba(244, 63, 94, 0.35);
    color: #f43f5e;
  }

  .badge-mr_merge {
    background: rgba(16, 185, 129, 0.15);
    border: 1px solid rgba(16, 185, 129, 0.35);
    color: #10b981;
  }

  .timeline-time {
    font-size: 0.75rem;
    color: #64748b;
  }

  .timeline-content {
    background: rgba(30, 41, 59, 0.35);
    border: 1px solid rgba(71, 85, 105, 0.25);
    border-radius: 8px;
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .timeline-meta {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
    font-size: 0.75rem;
    color: #94a3b8;
  }

  .meta-repo {
    font-weight: 500;
  }

  .meta-branch {
    color: #64748b;
  }

  .meta-hash {
    background: rgba(15, 23, 42, 0.6);
    padding: 1px 6px;
    border-radius: 4px;
    color: #38bdf8;
    border: 1px solid rgba(56, 189, 248, 0.25);
  }

  .timeline-body {
    font-size: 0.8rem;
    color: #cbd5e1;
    text-align: left;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .mr-timeline-link {
    color: #38bdf8;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  .mr-timeline-link:hover {
    color: #7dd3fc;
  }

  .timeline-footer {
    font-size: 0.7rem;
    color: #64748b;
    display: flex;
    justify-content: flex-end;
  }

  /* Parent Demand Styling for Cards and Details */
  .parent-demand-badge {
    font-size: 0.65rem;
    color: #a5b4fc;
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.3);
    border-radius: 4px;
    padding: 2px 6px;
    margin-top: 6px;
    margin-bottom: 2px;
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow: hidden;
    max-width: 100%;
    display: inline-block;
    transition: all 0.2s;
  }

  .parent-demand-badge:hover {
    background: rgba(99, 102, 241, 0.25);
    border-color: rgba(99, 102, 241, 0.4);
    color: #ffffff;
  }

  .parent-demand-detail {
    color: #a5b4fc !important;
    background: rgba(99, 102, 241, 0.12);
    border: 1px solid rgba(99, 102, 241, 0.25);
    padding: 3px 8px;
    border-radius: 6px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .parent-demand-detail .title-sub {
    color: #e2e8f0;
    font-weight: normal;
  }
  .combobox-trigger {
    cursor: text !important;
  }
  .combobox-trigger-input {
    background: transparent !important;
    border: none !important;
    outline: none !important;
    color: #cbd5e1 !important;
    font-size: 0.8rem;
    font-family: inherit;
    padding: 0 !important;
    margin: 0 !important;
    flex: 1;
    min-width: 0;
  }
  .combobox-trigger-input::placeholder {
    color: #cbd5e1 !important;
    opacity: 1;
  }

  /* Decoupled Git Telemetry styles */
  .telemetry-decoupled-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px 0;
  }
  .decoupled-title {
    font-size: 0.7rem;
    font-weight: 700;
    color: #38bdf8;
    letter-spacing: 0.05em;
  }
  .view-telemetry-drawer-btn {
    background: rgba(56, 189, 248, 0.1);
    color: #38bdf8;
    border: 1px solid rgba(56, 189, 248, 0.3);
    border-radius: 6px;
    padding: 10px 16px;
    font-size: 0.85rem;
    cursor: pointer;
    transition: background 0.2s, color 0.2s;
    width: 100%;
    text-align: center;
  }
  .view-telemetry-drawer-btn:hover {
    background: rgba(56, 189, 248, 0.2);
    color: #f1f5f9;
  }
  .exec-action-btn {
    background: rgba(30, 41, 59, 0.6);
    color: #cbd5e1;
    border: 1px solid rgba(51, 65, 85, 0.6);
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 0.75rem;
    cursor: pointer;
    transition: all 0.2s;
  }
  .exec-action-btn:hover {
    background: rgba(56, 189, 248, 0.15);
    color: #38bdf8;
    border-color: rgba(56, 189, 248, 0.4);
  }

  /* 证据小徽章与证据链样式 */
  .meta-right {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
  }
  .evidence-badge-tag {
    font-size: 0.65rem;
    padding: 2px 6px;
    border-radius: 4px;
    font-weight: 700;
    letter-spacing: 0.02em;
  }
  .evidence-badge-tag.badge-no-code {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.3);
  }
  .evidence-badge-tag.badge-branch-only {
    background: rgba(251, 191, 36, 0.15);
    color: #fbbf24;
    border: 1px solid rgba(251, 191, 36, 0.3);
  }
  .evidence-badge-tag.badge-has-code {
    background: rgba(52, 211, 153, 0.15);
    color: #34d399;
    border: 1px solid rgba(52, 211, 153, 0.3);
  }
  .evidence-chain-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-top: 14px;
    margin-bottom: 14px;
  }
  .evidence-chain-loading {
    font-size: 0.75rem;
    color: #64748b;
    padding: 8px 0;
  }
  .evidence-chain-error {
    font-size: 0.75rem;
    color: #f87171;
    padding: 8px 0;
  }
  .evidence-chain-summary-panel {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.3);
    padding: 10px;
    border-radius: 8px;
  }
  .chain-stat {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }
  .chain-stat .stat-label {
    font-size: 0.65rem;
    color: #64748b;
  }
  .chain-stat strong {
    font-size: 1rem;
    color: #f1f5f9;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }
  .evidence-timeline {
    max-height: 200px;
    overflow-y: auto;
    border-left: 1px solid rgba(99, 102, 241, 0.3);
    padding-left: 12px;
    margin-left: 8px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .timeline-node {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 4px;
    text-align: left;
  }
  .timeline-node::before {
    content: '';
    position: absolute;
    left: -17px;
    top: 5px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #818cf8;
    box-shadow: 0 0 6px #818cf8;
  }
  .node-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 0.65rem;
    color: #64748b;
  }
  .node-time {
    color: #94a3b8;
  }
  .node-repo {
    background: rgba(148, 163, 184, 0.1);
    padding: 1px 4px;
    border-radius: 3px;
  }
  .mr-link-chain {
    color: #818cf8;
    text-decoration: none;
  }
  .mr-link-chain:hover {
    text-decoration: underline;
  }
  .node-content {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 0.75rem;
  }
  .node-action {
    font-weight: 700;
    text-transform: uppercase;
    font-size: 0.65rem;
    padding: 1px 4px;
    border-radius: 3px;
  }
  .node-action.action-git_push {
    background: rgba(16, 185, 129, 0.15);
    color: #34d399;
  }
  .node-action.action-mr_open {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
  }
  .node-action.action-mr_merge {
    background: rgba(139, 92, 246, 0.15);
    color: #a78bfa;
  }
  .node-commit {
    color: #cbd5e1;
  }
  .evidence-chain-empty {
    font-size: 0.75rem;
    color: #f87171;
    text-align: center;
    padding: 12px;
    background: rgba(239, 68, 68, 0.05);
    border: 1px dashed rgba(239, 68, 68, 0.25);
    border-radius: 6px;
  }
  .evidence-chain-signals {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 6px;
  }
  .signal-chip {
    font-size: 0.65rem;
    background: rgba(30, 41, 59, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.4);
    padding: 2px 8px;
    border-radius: 9999px;
    color: #cbd5e1;
  }

  .task-console {
    --task-panel: rgba(12, 18, 24, 0.72);
    --task-panel-strong: rgba(8, 12, 17, 0.88);
    --task-border: var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    --task-border-strong: var(--wa-border-strong, rgba(203, 234, 244, 0.28));
    --task-text: var(--wa-text-main, #d8e5ec);
    --task-strong: var(--wa-text-strong, #f4fbff);
    --task-muted: var(--wa-text-muted, #90a8b5);
    --task-subtle: var(--wa-text-subtle, #5f7582);
    --task-accent: var(--wa-accent, #26ddff);
    --task-success: var(--wa-success, #72e6b4);
    --task-warning: var(--wa-warning, #ffc55f);
    --task-danger: var(--wa-danger, #ff6177);
    color: var(--task-text);
    margin-bottom: 0;
  }

  .task-console .summary-kicker,
  .task-console .exec-grid-th,
  .task-console .telemetry-label,
  .task-console .decoupled-title,
  .task-console .node-action {
    letter-spacing: 0;
  }

  .task-command-surface {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 400px);
    gap: 14px;
    margin-bottom: 18px;
  }

  .task-command-main,
  .task-focus-panel,
  .execution-inspector-panel {
    position: relative;
    overflow: hidden;
    border: 1px solid var(--task-border);
    background:
      linear-gradient(145deg, rgba(244, 251, 255, 0.075), rgba(244, 251, 255, 0.018) 42%, rgba(38, 221, 255, 0.052)),
      var(--task-panel);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.08), 0 24px 68px rgba(1, 9, 14, 0.34);
    backdrop-filter: blur(24px) saturate(138%);
    -webkit-backdrop-filter: blur(24px) saturate(138%);
  }

  .task-command-main {
    border-radius: 12px;
    padding: 18px;
    display: grid;
    gap: 16px;
  }

  .task-command-eyebrow,
  .task-command-title-row,
  .task-filter-dock,
  .focus-panel-head,
  .execution-inspector-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .task-command-eyebrow {
    color: var(--task-subtle);
    font-size: 0.68rem;
    font-weight: 900;
  }

  .task-live-dot {
    display: inline-flex;
    align-items: center;
    min-height: 22px;
    border: 1px solid rgba(114, 230, 180, 0.28);
    border-radius: 999px;
    padding: 0 9px;
    color: var(--task-success);
    background: rgba(114, 230, 180, 0.1);
    font-size: 0.72rem;
    font-weight: 800;
  }

  .task-command-copy h2 {
    margin: 0;
    color: var(--task-strong);
    font-size: clamp(1.35rem, 2vw, 2rem);
    line-height: 1.08;
    font-weight: 820;
  }

  .task-command-copy p {
    max-width: 680px;
    margin: 8px 0 0;
    color: var(--task-muted);
    font-size: 0.86rem;
    line-height: 1.6;
  }

  .task-view-toggle {
    flex: 0 0 auto;
    border-radius: 8px;
    background: rgba(244, 251, 255, 0.055);
    border: 1px solid var(--task-border);
    padding: 3px;
  }

  .task-view-toggle .toggle-btn {
    min-width: 54px;
    justify-content: center;
    border-radius: 6px;
    color: var(--task-muted);
  }

  .task-view-toggle .toggle-btn.active {
    background: rgba(38, 221, 255, 0.16);
    color: var(--task-strong);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.08);
  }

  .task-filter-dock {
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .task-filter-dock .custom-select-container {
    min-width: 220px;
  }

  .task-filter-control {
    min-height: 38px;
    width: 100%;
    box-sizing: border-box;
    border-radius: 8px;
    background: rgba(244, 251, 255, 0.048);
    border-color: var(--task-border);
  }

  .filter-label {
    color: var(--task-subtle);
    font-size: 0.62rem;
    font-weight: 900;
  }

  .task-console .select-arrow {
    border: 0;
    background: transparent;
    padding: 0;
    cursor: pointer;
    line-height: 1;
  }

  .task-command-metrics {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
  }

  .task-metric-cell {
    min-height: 92px;
    border-radius: 8px;
    border: 1px solid var(--task-border);
    background: rgba(244, 251, 255, 0.045);
    padding: 12px;
    display: grid;
    align-content: space-between;
    gap: 8px;
  }

  .task-metric-cell .metric-label {
    color: var(--task-subtle);
    font-size: 0.63rem;
    font-weight: 900;
  }

  .task-metric-cell strong {
    color: var(--task-strong);
    font-size: 1.7rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .task-metric-cell em {
    color: var(--task-muted);
    font-size: 0.72rem;
    font-style: normal;
    line-height: 1.35;
  }

  .task-metric-cell.tone-total { border-top: 2px solid var(--task-accent); }
  .task-metric-cell.tone-info { border-top: 2px solid var(--wa-info, #9bd4ff); }
  .task-metric-cell.tone-warning { border-top: 2px solid var(--task-warning); }
  .task-metric-cell.tone-danger { border-top: 2px solid var(--task-danger); }

  .task-flow-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 8px;
  }

  .task-flow-card {
    position: relative;
    overflow: hidden;
    min-height: 56px;
    border: 1px solid var(--task-border);
    border-radius: 8px;
    background: rgba(7, 11, 16, 0.52);
    color: var(--task-text);
    padding: 9px 10px;
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 4px 10px;
    text-align: left;
    cursor: pointer;
  }

  .task-flow-card span {
    color: var(--task-muted);
    font-size: 0.7rem;
    font-weight: 850;
  }

  .task-flow-card strong {
    color: var(--task-strong);
    font-size: 1rem;
    font-variant-numeric: tabular-nums;
  }

  .task-flow-card em {
    color: var(--task-subtle);
    font-size: 0.66rem;
    font-style: normal;
  }

  .task-flow-card i {
    grid-column: 1 / -1;
    height: 3px;
    min-width: 3px;
    border-radius: 999px;
    background: var(--task-accent);
  }

  .task-flow-card.tone-success i { background: var(--task-success); }
  .task-flow-card.tone-warning i { background: var(--task-warning); }
  .task-flow-card.tone-info i { background: var(--wa-info, #9bd4ff); }
  .task-flow-card:hover {
    border-color: var(--task-border-strong);
    background: rgba(244, 251, 255, 0.075);
  }

  .task-focus-panel {
    border-radius: 12px;
    padding: 16px;
    display: grid;
    align-content: start;
    gap: 16px;
  }

  .focus-severity {
    display: inline-flex;
    min-height: 22px;
    align-items: center;
    border-radius: 999px;
    border: 1px solid rgba(38, 221, 255, 0.25);
    background: rgba(38, 221, 255, 0.1);
    color: var(--task-accent);
    padding: 0 8px;
    font-size: 0.65rem;
  }

  .focus-severity.is-watch {
    border-color: rgba(255, 197, 95, 0.3);
    background: rgba(255, 197, 95, 0.12);
    color: var(--task-warning);
  }

  .focus-severity.is-critical {
    border-color: rgba(255, 97, 119, 0.34);
    background: rgba(255, 97, 119, 0.13);
    color: var(--task-danger);
  }

  .focus-task-body {
    display: grid;
    gap: 12px;
  }

  .focus-task-id {
    width: fit-content;
    border: 1px solid rgba(38, 221, 255, 0.24);
    border-radius: 6px;
    background: rgba(38, 221, 255, 0.09);
    color: var(--task-accent);
    padding: 4px 7px;
    font-size: 0.68rem;
    font-weight: 900;
  }

  .focus-task-body h3,
  .execution-focus-stack h3 {
    margin: 0;
    color: var(--task-strong);
    font-size: 1.02rem;
    line-height: 1.42;
  }

  .focus-meta-grid,
  .execution-focus-meta {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .focus-meta-grid span,
  .execution-focus-meta span,
  .execution-inspector-stats span {
    min-width: 0;
    border: 1px solid var(--task-border);
    border-radius: 8px;
    background: rgba(7, 11, 16, 0.36);
    padding: 8px;
    display: grid;
    gap: 5px;
  }

  .focus-meta-grid em,
  .execution-focus-meta em,
  .execution-inspector-stats em {
    color: var(--task-subtle);
    font-size: 0.66rem;
    font-style: normal;
  }

  .focus-meta-grid strong,
  .execution-focus-meta strong,
  .execution-inspector-stats strong {
    color: var(--task-text);
    font-size: 0.78rem;
    line-height: 1.32;
    overflow-wrap: anywhere;
  }

  .focus-action-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .focus-primary-btn,
  .focus-secondary-btn {
    min-height: 38px;
    border-radius: 8px;
    border: 1px solid var(--task-border);
    cursor: pointer;
    font-weight: 850;
  }

  .focus-primary-btn {
    background: var(--task-accent);
    border-color: rgba(184, 245, 255, 0.42);
    color: var(--wa-accent-ink, #021318);
  }

  .focus-secondary-btn {
    background: rgba(244, 251, 255, 0.055);
    color: var(--task-text);
  }

  .focus-secondary-btn:hover,
  .focus-primary-btn:hover {
    transform: translateY(-1px);
  }

  .owner-pressure-panel {
    border-top: 1px solid var(--task-border);
    padding-top: 14px;
    display: grid;
    gap: 10px;
  }

  .owner-pressure-main {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    color: var(--task-text);
  }

  .owner-pressure-main strong {
    color: var(--task-strong);
  }

  .owner-pressure-main span,
  .owner-pressure-main.is-empty {
    color: var(--task-muted);
    font-size: 0.78rem;
  }

  .owner-mini-list {
    display: grid;
    gap: 6px;
  }

  .owner-mini-list span {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    color: var(--task-muted);
    font-size: 0.74rem;
  }

  .owner-mini-list em {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-style: normal;
  }

  .owner-mini-list strong {
    color: var(--task-strong);
    font-variant-numeric: tabular-nums;
  }

  .execution-lab-layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(300px, 360px);
    gap: 12px;
    align-items: start;
  }

  .execution-lab-main {
    min-width: 0;
  }

  .execution-inspector-panel {
    border-radius: 12px;
    padding: 14px;
    display: grid;
    gap: 14px;
    position: sticky;
    top: 12px;
  }

  .execution-inspector-head {
    align-items: end;
  }

  .execution-inspector-head strong {
    color: var(--task-strong);
    font-size: 2rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .execution-inspector-head em {
    color: var(--task-muted);
    font-size: 0.72rem;
    font-style: normal;
  }

  .execution-focus-stack {
    display: grid;
    gap: 11px;
  }

  .exec-action-btn.is-wide {
    width: 100%;
    min-height: 38px;
    border-radius: 8px;
  }

  .execution-inspector-stats {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 7px;
  }

  .execution-inspector-stats span {
    text-align: center;
  }

  .execution-inspector-stats strong {
    color: var(--task-strong);
    font-size: 1rem;
  }

  .execution-inspector-empty,
  .focus-empty {
    min-height: 96px;
    border: 1px dashed var(--task-border);
    border-radius: 8px;
    display: grid;
    place-items: center;
    color: var(--task-subtle);
    font-size: 0.72rem;
  }

  .task-console .execution-workbench {
    gap: 12px;
  }

  .task-console .execution-summary-cell,
  .task-console .execution-control-panel,
  .task-console .execution-table-panel,
  .task-console .column,
  .task-console .swimlane {
    border-radius: 10px;
    border-color: var(--task-border);
    background: rgba(244, 251, 255, 0.042);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.05);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
  }

  .task-console .execution-summary-cell {
    min-height: 82px;
    border-top-width: 2px;
  }

  .task-console .execution-control-panel {
    background: rgba(8, 12, 17, 0.62);
  }

  .task-console .execution-table-panel {
    overflow: hidden;
  }

  .task-console .exec-grid-table {
    background: rgba(6, 9, 13, 0.88);
  }

  .task-console .exec-grid-thead {
    background: rgba(12, 18, 24, 0.96);
  }

  .task-console .exec-grid-tr {
    border-bottom-color: rgba(203, 234, 244, 0.08);
  }

  .task-console .exec-grid-tr:hover {
    background: rgba(231, 247, 255, 0.055);
  }

  .task-console .kanban-grid {
    gap: 12px;
  }

  .task-console .column {
    min-height: 460px;
    padding: 12px;
  }

  .task-console .column-header {
    min-height: 34px;
    margin-bottom: 10px;
    padding-bottom: 10px;
    border-bottom-color: var(--task-border);
  }

  .task-console .column-title {
    color: var(--task-text);
    font-size: 0.78rem;
  }

  .task-console .count-badge {
    border: 1px solid var(--task-border);
    background: rgba(244, 251, 255, 0.06);
    color: var(--task-strong);
  }

  .task-console .column-body {
    max-height: 620px;
    gap: 9px;
    padding-right: 6px;
  }

  .task-console .task-card,
  .task-console .compact-task-card {
    border-radius: 8px;
    border-color: rgba(203, 234, 244, 0.13);
    background: rgba(7, 11, 16, 0.56);
    box-shadow: none;
  }

  .task-console .task-card:hover,
  .task-console .compact-task-card:hover {
    transform: translateY(-1px);
    border-color: var(--task-border-strong);
    background: rgba(244, 251, 255, 0.065);
  }

  .task-console .task-id,
  .task-console .compact-days-badge,
  .task-console .active-days-badge,
  .task-console .project-badge,
  .task-console .issue-type-badge,
  .task-console .parent-demand-badge,
  .task-console .evidence-badge-tag,
  .task-console .jira-direct-link {
    border-radius: 6px;
  }

  .task-console .jira-direct-link {
    min-height: 18px;
    padding: 1px 5px;
    border: 1px solid rgba(38, 221, 255, 0.24);
    background: rgba(38, 221, 255, 0.08);
    color: var(--task-accent);
    font-size: 0.62rem;
    font-weight: 850;
  }

  .task-console .task-footer {
    gap: 8px;
    flex-wrap: wrap;
  }

  .task-console .delay-critical {
    border-color: rgba(255, 97, 119, 0.6) !important;
    background: rgba(255, 97, 119, 0.07) !important;
  }

  .task-console .delay-warning {
    border-color: rgba(255, 197, 95, 0.52) !important;
    background: rgba(255, 197, 95, 0.055) !important;
  }

  .task-console .swimlane {
    padding: 12px;
  }

  .task-console .swimlane-header {
    min-height: 42px;
  }

  .task-console .assignee-avatar {
    width: 26px;
    height: 26px;
    border: 1px solid rgba(38, 221, 255, 0.24);
    border-radius: 8px;
    display: inline-grid;
    place-items: center;
    background: rgba(38, 221, 255, 0.1);
    color: var(--task-accent);
    font-size: 0.72rem;
    font-weight: 900;
  }

  .task-console .assignee-status-overview {
    border: 0;
    background: transparent;
    padding: 0;
    flex-wrap: wrap;
  }

  .task-console .swimlane-badge {
    border-radius: 6px;
  }

  .task-console .swimlane-grid {
    gap: 10px;
  }

  .task-console .swimlane-column {
    border-color: var(--task-border);
    background: rgba(7, 11, 16, 0.42);
    border-radius: 8px;
  }

  @media (max-width: 1180px) {
    .task-command-surface,
    .execution-lab-layout {
      grid-template-columns: 1fr;
    }

    .execution-inspector-panel {
      position: relative;
      top: auto;
    }

    .task-command-metrics {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .task-command-main,
    .task-focus-panel,
    .execution-inspector-panel {
      padding: 12px;
    }

    .task-command-title-row,
    .task-command-eyebrow,
    .task-filter-dock,
    .focus-panel-head {
      align-items: stretch;
      flex-direction: column;
    }

    .task-view-toggle,
    .task-filter-dock .custom-select-container,
    .focus-action-row,
    .focus-meta-grid,
    .execution-focus-meta,
    .execution-inspector-stats {
      width: 100%;
      grid-template-columns: 1fr;
    }

    .task-command-metrics,
    .task-flow-strip {
      grid-template-columns: 1fr;
    }
  }

  .task-console {
    --task-aligned-panel-height: clamp(480px, calc(100dvh - 420px), 680px);
    color: var(--wa-text-main);
    display: grid;
    gap: var(--wa-space-4);
    margin-bottom: 0;
  }

  .task-console.view-execution {
    --task-aligned-panel-height: clamp(564px, calc(100dvh - 336px), 764px);
  }

  .task-console > .execution-workbench,
  .task-console > .kanban-grid {
    display: none;
  }

  .phase41-console-header {
    min-width: 0;
    border-radius: var(--wa-radius-md);
    padding: 10px 12px;
    display: grid;
    grid-template-columns: minmax(180px, 0.62fr) minmax(420px, 1.38fr);
    gap: var(--wa-space-3);
    align-items: center;
    background: rgba(255, 255, 255, 0.82);
    box-shadow: var(--wa-shadow-sm);
  }

  .phase41-header-copy {
    min-width: 0;
  }

  .phase41-kicker {
    display: block;
    color: var(--wa-text-muted);
    font-size: 11px;
    line-height: 1.2;
    font-weight: 760;
  }

  .phase41-header-copy h2 {
    margin: 3px 0 0;
    color: var(--wa-text-strong);
    font-size: 18px;
    line-height: 1.15;
    font-weight: 850;
    text-wrap: balance;
  }

  .phase41-header-copy p {
    display: none;
  }

  .phase41-header-actions {
    min-width: 0;
    display: grid;
    justify-items: end;
    gap: var(--wa-space-3);
  }

  .task-console .view-toggle,
  .task-console .task-view-toggle {
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: 3px;
    box-shadow: none;
  }

  .task-console .toggle-btn {
    min-height: 30px;
    color: var(--wa-text-muted);
    border-radius: var(--wa-radius-sm);
    padding: 0 12px;
    font-size: 13px;
    font-weight: 760;
  }

  .task-console .toggle-btn:hover {
    color: var(--wa-text-strong);
    background: rgba(255, 255, 255, 0.7);
  }

  .task-console .toggle-btn.active {
    color: var(--wa-accent-ink);
    background: var(--wa-accent);
    box-shadow: 0 8px 18px rgba(0, 143, 150, 0.16);
  }

  .phase41-toolbar-filters {
    position: relative;
    z-index: 2;
    min-width: 0;
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(160px, 1fr));
    gap: var(--wa-space-2);
  }

  .phase41-toolbar-filters .custom-select-container {
    z-index: 1;
    min-width: 0;
    width: 100%;
  }

  .phase41-toolbar-filters .custom-select-container:focus-within {
    z-index: 3;
  }

  .phase41-filter-multi {
    --multi-select-dropdown-min: 292px;
    position: relative;
    z-index: 1;
    min-width: 0;
  }

  .phase41-filter-multi:focus-within {
    z-index: 4;
  }

  .phase41-filter-multi :global(.multi-select-group) {
    margin: 0;
  }

  .phase41-filter-multi :global(.multi-select-trigger) {
    border-color: var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.82);
    box-shadow: none;
  }

  .phase41-filter-multi :global(.multi-select-trigger:hover),
  .phase41-filter-multi :global(.multi-select-trigger:focus-within),
  .phase41-filter-multi :global(.multi-select-trigger.is-active) {
    border-color: var(--wa-border-focus);
    background: var(--wa-surface-flat);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .phase41-filter-multi :global(.multi-select-dropdown) {
    border-color: var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.99);
  }

  .phase41-execution-controls .phase41-project-select :global(.multi-select-dropdown) {
    right: 0;
    left: auto;
  }

  .task-console .custom-select-trigger,
  .task-console .task-filter-control {
    width: 100%;
    min-height: var(--wa-control-h);
    box-sizing: border-box;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.78);
    color: var(--wa-text-main);
    box-shadow: none;
  }

  .task-console .custom-select-trigger:hover,
  .task-console .custom-select-trigger:focus-within {
    border-color: var(--wa-border-focus);
    background: #ffffff;
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .task-console .filter-label {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 760;
    white-space: nowrap;
  }

  .task-console .combobox-trigger-input {
    color: var(--wa-text-main) !important;
    font-size: 13px;
  }

  .task-console .combobox-trigger-input::placeholder {
    color: var(--wa-text-subtle) !important;
  }

  .task-console .select-arrow {
    border: 0;
    background: transparent;
    color: var(--wa-text-muted);
    cursor: pointer;
  }

  .task-console .custom-select-options {
    left: 0;
    right: auto;
    width: 100%;
    min-width: 100%;
    border: 1px solid var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.96);
    z-index: 1000;
    box-shadow: var(--wa-shadow-md);
  }

  .task-console .custom-option {
    color: var(--wa-text-main);
    border-radius: var(--wa-radius-sm);
  }

  .task-console .custom-option:hover,
  .task-console .custom-option.active {
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .phase41-summary-strip {
    min-width: 0;
    min-height: 72px;
    display: grid;
    gap: 0;
    padding: 0;
    overflow: hidden;
    border: 1px solid var(--wa-glass-outline);
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel);
    box-shadow: var(--wa-shadow-glass);
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .phase41-summary-strip.is-status {
    grid-template-columns: repeat(6, minmax(0, 1fr));
  }

  .phase41-summary-strip.is-execution {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .phase41-summary-strip > .phase41-metric,
  .phase41-summary-strip > .phase41-stage-card {
    min-height: 72px;
    border: 0;
    border-right: 1px solid var(--wa-border-divider);
    border-radius: 0;
    background: var(--task-cell-background, var(--wa-neutral-soft));
    box-shadow: none;
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }

  .phase41-summary-strip > :last-child {
    border-right: 0;
  }

  .phase41-metric {
    min-width: 0;
    padding: 9px 12px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-rows: auto auto;
    align-content: center;
    gap: 3px 8px;
  }

  .phase41-metric > span,
  .phase41-metric > small,
  .phase41-metric > em {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .phase41-metric > strong {
    color: var(--wa-text-strong);
    font-size: 20px;
    line-height: 1.1;
    font-variant-numeric: tabular-nums;
  }

  .phase41-metric > small {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .phase41-metric > em {
    justify-self: end;
  }

  .phase41-metric em {
    color: var(--wa-text-subtle);
    font-size: 12px;
    font-style: normal;
    font-weight: 720;
  }

  .phase41-stage-card {
    min-width: 0;
    padding: 9px 10px;
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 6px var(--wa-space-2);
    text-align: left;
    cursor: pointer;
    transition:
      background-color var(--wa-duration-fast) var(--wa-ease),
      color var(--wa-duration-fast) var(--wa-ease);
  }

  .phase41-stage-card span {
    min-width: 0;
    overflow: hidden;
    color: var(--wa-text-main);
    font-size: 13px;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .phase41-stage-card strong {
    color: var(--task-tone, var(--wa-text-strong));
    font-size: 20px;
    font-variant-numeric: tabular-nums;
  }

  .phase41-stage-card em {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-style: normal;
  }

  .phase41-stage-card i {
    grid-column: 1 / -1;
    height: 3px;
    min-width: 3px;
    border-radius: 999px;
    background: var(--task-tone, var(--wa-text-subtle));
  }

  @media (hover: hover) {
    .phase41-stage-card:hover {
      background: var(--wa-row-hover);
    }
  }

  .phase41-stage-card:active {
    background: var(--wa-row-active);
  }

  .phase41-workbench {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, var(--wa-inspector-w));
    gap: var(--wa-space-4);
    align-items: start;
  }

  .phase41-execution-workbench {
    grid-auto-rows: minmax(var(--task-aligned-panel-height), auto);
    align-items: stretch;
  }

  .phase41-execution-workbench .phase41-table-panel,
  .phase41-execution-workbench .phase41-inspector {
    height: auto;
    min-height: var(--task-aligned-panel-height);
    max-height: none;
    box-sizing: border-box;
  }

  .phase41-status-workbench {
    grid-auto-rows: minmax(var(--task-aligned-panel-height), auto);
    align-items: stretch;
  }

  .phase41-status-workbench .phase41-table-panel,
  .phase41-status-workbench .phase41-inspector {
    height: auto;
    min-height: var(--task-aligned-panel-height);
    max-height: none;
    box-sizing: border-box;
  }

  .phase41-status-workbench .phase41-table-panel {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    overflow: hidden;
  }

  .phase41-status-workbench .phase41-table-shell {
    min-height: 0;
    height: 100%;
    overflow: auto;
    overscroll-behavior: contain;
  }

  .phase41-status-workbench .phase41-inspector {
    position: relative;
    top: auto;
    overflow: visible;
    overscroll-behavior: auto;
  }

  .phase41-table-panel {
    min-width: 0;
  }

  .phase41-execution-workbench .phase41-table-panel {
    position: relative;
    isolation: isolate;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    overflow: hidden;
  }

  .phase41-table-toolbar {
    border-radius: var(--wa-radius-md);
    align-items: center;
  }

  .phase41-status-toolbar {
    position: relative;
    z-index: 2;
    overflow: visible;
    display: grid;
    grid-template-columns: minmax(190px, 0.58fr) minmax(360px, 1.22fr) auto;
    grid-template-areas: "copy filters actions";
  }

  .phase41-status-toolbar > div:first-child {
    grid-area: copy;
  }

  .phase41-status-toolbar .phase41-toolbar-filters {
    grid-area: filters;
  }

  .phase41-status-toolbar .phase41-toolbar-actions {
    grid-area: actions;
  }

  .phase41-execution-workbench .phase41-table-toolbar {
    position: relative;
    z-index: 2;
    overflow: visible;
  }

  .phase41-table-toolbar > div:first-child {
    min-width: 0;
    display: grid;
    gap: 4px;
  }

  .phase41-table-toolbar strong {
    color: var(--wa-text-strong);
    font-size: 16px;
    line-height: 1.25;
  }

  .phase41-table-toolbar small {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .phase41-toolbar-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--wa-space-2);
    flex-wrap: wrap;
  }

  .phase41-inline-error {
    color: var(--wa-danger);
    font-size: 12px;
  }

  .phase41-execution-controls {
    min-width: 0;
    flex: 1;
    display: grid;
    grid-template-columns:
      minmax(260px, 1.25fr)
      minmax(140px, 0.5fr)
      minmax(200px, 0.76fr)
      minmax(210px, 0.8fr)
      auto;
    gap: var(--wa-space-2);
    align-items: center;
  }

  .phase41-search-control {
    position: relative;
    min-width: 0;
  }

  .phase41-search-control input {
    width: 100%;
    height: var(--wa-control-h);
    box-sizing: border-box;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.8);
    color: var(--wa-text-main);
    padding: 0 12px 0 34px;
    font-size: 13px;
    outline: none;
  }

  .phase41-search-control input:focus {
    border-color: var(--wa-border-focus);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .phase41-search-control .execution-search-mark {
    border-color: var(--wa-text-muted);
  }

  .phase41-search-control .execution-search-mark::after {
    background: var(--wa-text-muted);
  }

  .phase41-risk-strip {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 4px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: 3px;
    overflow-x: auto;
  }

  .phase41-execution-select,
  .phase41-risk-select,
  .phase41-assignee-select {
    min-width: 0;
  }

  .phase41-table-shell {
    border-radius: var(--wa-radius-md);
  }

  .phase41-execution-workbench .phase41-table-shell {
    position: relative;
    z-index: 1;
    min-height: 0;
    height: 100%;
    overflow: auto;
    overscroll-behavior: contain;
  }

  .phase41-execution-workbench .phase41-inspector {
    position: relative;
    top: auto;
    overflow: visible;
    overscroll-behavior: auto;
  }

  .phase41-table {
    min-width: 1040px;
  }

  .phase41-table th[style*="right"],
  .phase41-table td.phase41-action-cell {
    text-align: right;
  }

  .phase41-table tbody tr {
    cursor: pointer;
  }

  .phase41-table tbody tr:focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: -2px;
  }

  .phase41-title-cell {
    min-width: 0;
    display: grid;
    gap: 6px;
  }

  .phase41-status-workbench .phase41-title-cell {
    display: block;
  }

  .phase41-status-workbench .phase41-title-main {
    min-width: 0;
    display: grid;
    grid-template-columns: auto auto minmax(0, 1fr);
    align-items: center;
    gap: 7px;
    flex-wrap: nowrap;
  }

  .phase41-status-workbench .phase41-title-main strong {
    min-width: 0;
    overflow: hidden;
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.35;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .phase41-type-icon {
    width: 24px;
    height: 24px;
    display: inline-grid;
    place-items: center;
    flex: none;
    border: 1px solid var(--wa-border-soft);
    border-radius: 999px;
    background: var(--wa-surface-inset);
    color: var(--wa-text-muted);
  }

  .phase41-type-icon svg {
    width: 14px;
    height: 14px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .phase41-type-icon.is-bug {
    border-color: rgba(221, 75, 62, 0.2);
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .phase41-type-icon.is-task {
    border-color: rgba(37, 107, 216, 0.18);
    background: var(--wa-info-soft);
    color: var(--wa-info);
  }

  .phase41-title-cell > div {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .phase41-title-cell strong {
    min-width: 0;
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.35;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .phase41-title-cell small {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .phase41-id-link {
    display: inline-flex;
    min-height: 24px;
    align-items: center;
    border: 1px solid rgba(37, 107, 216, 0.2);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-info-soft);
    color: var(--wa-info);
    padding: 0 8px;
    font-size: 12px;
    font-weight: 780;
    text-decoration: none;
    font-variant-numeric: tabular-nums;
  }

  .phase41-id-link.as-text {
    color: var(--wa-text-main);
    background: var(--wa-neutral-soft);
  }

  .phase41-owner-cell {
    display: grid;
    gap: 4px;
  }

  .phase41-owner-cell strong {
    color: var(--wa-text-main);
    font-size: 13px;
  }

  .phase41-owner-cell small,
  .phase41-evidence-cell span {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .phase41-evidence-cell {
    display: grid;
    gap: 6px;
    min-width: 0;
  }

  .phase41-evidence-cell.is-compact {
    display: block;
  }

  .phase41-status-workbench .phase41-table th,
  .phase41-status-workbench .phase41-table td {
    padding-top: 8px;
    padding-bottom: 8px;
  }

  .phase41-status-workbench .phase41-table tbody tr {
    height: 46px;
  }

  .phase41-status-workbench .phase41-table tbody tr.is-selected {
    background: var(--wa-row-active);
  }

  .phase41-action-cell .compact {
    min-height: 30px;
    padding: 0 10px;
  }

  .phase41-skeleton-row td {
    height: 48px;
    background:
      linear-gradient(90deg, rgba(102, 119, 137, 0.06), rgba(102, 119, 137, 0.13), rgba(102, 119, 137, 0.06));
    background-size: 220% 100%;
    animation: phase41Shimmer 1.2s linear infinite;
  }

  .phase41-empty-cell,
  .phase41-empty-inspector {
    min-height: 160px;
    color: var(--wa-text-muted);
    text-align: center;
    padding: var(--wa-space-8);
  }

  .phase41-empty-cell.error {
    color: var(--wa-danger);
  }

  .phase41-inspector {
    position: sticky;
    top: var(--wa-space-4);
    border-radius: var(--wa-radius-md);
  }

  .phase41-inspector-head {
    display: grid;
    gap: var(--wa-space-2);
  }

  .phase41-inspector-head h3 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 17px;
    line-height: 1.35;
    text-wrap: pretty;
  }

  .phase41-inspector-head > div {
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
    flex-wrap: wrap;
  }

  .phase41-fact-grid {
    margin: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-2);
  }

  .phase41-fact-grid div {
    min-width: 0;
    border: 0;
    border-bottom: 1px solid var(--wa-border-soft);
    border-radius: 0;
    background: transparent;
    padding: var(--wa-space-2) 0 var(--wa-space-3);
  }

  .phase41-fact-grid dt {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .phase41-fact-grid dd {
    margin: 4px 0 0;
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 760;
    overflow-wrap: anywhere;
  }

  .phase41-fact-pills {
    display: flex;
    flex-wrap: wrap;
    gap: 7px;
  }

  .phase41-fact-pills > div {
    min-width: 0;
    width: auto;
    min-height: 30px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border-radius: 999px;
    padding: 3px 9px;
    border: 1px solid var(--wa-border-soft);
    background: transparent;
  }

  .phase41-fact-pills dt {
    flex: none;
    font-size: 10px;
    white-space: nowrap;
  }

  .phase41-fact-pills dd {
    margin: 0;
    font-size: 11px;
    line-height: 1.2;
    white-space: nowrap;
  }

  .phase41-fact-pills .is-type-fact {
    padding-right: 4px;
  }

  .phase41-fact-pills .phase41-type-icon {
    width: 22px;
    height: 22px;
    margin: 0;
  }

  .phase41-inspector-progress {
    display: grid;
    gap: var(--wa-space-2);
  }

  .phase41-inspector-progress span {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .phase41-inspector-section {
    border-top: 1px solid var(--wa-border-soft);
    padding-top: var(--wa-space-3);
    display: grid;
    gap: var(--wa-space-2);
  }

  .phase41-inspector-section h4 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 14px;
    line-height: 1.3;
  }

  .phase41-inspector-section p,
  .phase41-inspector-section li {
    margin: 0;
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.55;
    overflow-wrap: anywhere;
  }

  .phase41-inspector-section ul {
    margin: 0;
    padding-left: 18px;
    display: grid;
    gap: 4px;
  }

  .phase41-inspector-actions {
    display: flex;
    gap: var(--wa-space-2);
    flex-wrap: wrap;
  }

  .task-console .wa-admin-action:focus-visible,
  .task-console .toggle-btn:focus-visible,
  .task-console .phase41-stage-card:focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  .task-console .phase41-summary-strip .phase41-stage-card:focus-visible {
    outline-offset: -3px;
  }

  .task-console .swimlane,
  .task-console .swimlane-column {
    border-color: var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.78);
    box-shadow: var(--wa-shadow-sm);
  }

  .task-console .swimlane-header {
    border-bottom-color: var(--wa-border-soft);
  }

  .task-console .assignee-name,
  .task-console .compact-title,
  .task-console .task-title {
    color: var(--wa-text-strong);
  }

  .task-console .compact-task-card {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-flat);
    box-shadow: none;
  }

  .task-console .compact-task-card:hover {
    border-color: var(--wa-border-strong);
    background: var(--wa-row-hover);
    transform: none;
  }

  .task-console .empty-placeholder,
  .task-console .empty-view {
    color: var(--wa-text-muted);
  }

  @keyframes phase41Shimmer {
    from { background-position: 100% 0; }
    to { background-position: -100% 0; }
  }

  @media (max-width: 1600px) {
    .phase41-status-toolbar {
      grid-template-columns: minmax(180px, 1fr) auto;
      grid-template-areas:
        "copy actions"
        "filters filters";
    }

    .phase41-execution-workbench .phase41-table-toolbar {
      flex-direction: column;
      align-items: stretch;
    }

    .phase41-execution-controls {
      grid-template-columns: minmax(260px, 1fr) minmax(160px, 0.56fr) minmax(220px, 0.78fr);
    }
  }

  @media (max-width: 1400px) {
    .phase41-execution-controls {
      grid-template-columns: minmax(220px, 1fr) minmax(240px, 1fr);
    }
  }

  @media (max-width: 1180px) {
    .phase41-console-header,
    .phase41-workbench {
      grid-template-columns: 1fr;
    }

    .phase41-inspector {
      position: relative;
      top: auto;
    }

    .phase41-summary-strip {
      overflow-x: auto;
      overflow-y: hidden;
      overscroll-behavior-inline: contain;
      scrollbar-width: none;
    }

    .phase41-summary-strip.is-status {
      grid-template-columns: repeat(6, minmax(112px, 1fr));
    }

    .phase41-summary-strip.is-execution {
      grid-template-columns: repeat(4, minmax(104px, 1fr));
    }

    .phase41-summary-strip::-webkit-scrollbar {
      display: none;
    }

    .phase41-execution-controls {
      grid-template-columns: minmax(220px, 1fr) minmax(260px, 1fr);
    }

    .phase41-status-workbench {
      grid-auto-rows: auto;
    }

    .phase41-status-workbench .phase41-table-panel,
    .phase41-status-workbench .phase41-inspector,
    .phase41-execution-workbench .phase41-table-panel,
    .phase41-execution-workbench .phase41-inspector {
      height: auto;
      min-height: 0;
      max-height: none;
    }
  }

  @media (max-width: 760px) {
    .phase41-console-header,
    .phase41-table-toolbar {
      align-items: stretch;
    }

    .phase41-status-toolbar {
      grid-template-columns: 1fr;
      grid-template-areas:
        "copy"
        "filters"
        "actions";
    }

    .phase41-header-actions {
      justify-items: stretch;
    }

    .phase41-toolbar-filters,
    .phase41-execution-controls,
    .phase41-fact-grid {
      grid-template-columns: 1fr;
    }

    .phase41-table-toolbar,
    .phase41-toolbar-actions {
      flex-direction: column;
      align-items: stretch;
    }

    .phase41-execution-controls {
      --wa-control-h: 44px;
    }

    .phase41-toolbar-filters {
      --wa-control-h: 44px;
    }

    .phase41-filter-multi {
      --multi-select-dropdown-min: 100%;
    }

    .phase41-filter-multi :global(input),
    .phase41-filter-multi :global(.multi-select-chevron),
    .phase41-table-toolbar .wa-admin-action,
    .phase41-toolbar-actions .wa-admin-action {
      min-height: 44px;
    }

    .phase41-filter-multi :global(.multi-select-chevron) {
      min-width: 44px;
    }

    .phase41-execution-controls .phase41-risk-strip,
    .phase41-execution-controls .wa-admin-action {
      min-height: 44px;
    }
  }

  /* Desktop task views share the remaining workspace height; only data shells scroll. */
  @media (min-width: 1181px) {
    .task-console {
      height: 100%;
      min-height: 0;
      grid-template-rows: auto auto auto;
      overflow-y: auto !important;
      overscroll-behavior: contain;
    }

    .task-console.view-execution {
      grid-template-rows: auto auto;
    }

    .phase41-workbench {
      height: auto;
      min-height: 0;
      grid-auto-rows: auto;
      align-items: stretch;
      overflow: visible;
    }

    .phase41-execution-workbench .phase41-table-panel,
    .phase41-execution-workbench .phase41-inspector,
    .phase41-status-workbench .phase41-table-panel,
    .phase41-status-workbench .phase41-inspector {
      height: auto;
      min-height: 0;
      max-height: none;
    }

    .phase41-execution-workbench .phase41-inspector,
    .phase41-status-workbench .phase41-inspector {
      min-height: 0;
      overflow-y: auto;
      overscroll-behavior: contain;
      scrollbar-width: none;
    }

    .phase41-execution-workbench .phase41-inspector::-webkit-scrollbar,
    .phase41-status-workbench .phase41-inspector::-webkit-scrollbar {
      width: 0;
      height: 0;
    }
  }

  @media (min-width: 861px) and (max-width: 1180px) {
    .task-console {
      height: 100%;
      min-height: 0;
      overflow-y: auto !important;
      overscroll-behavior: contain;
    }

    .phase41-status-workbench .phase41-table-panel,
    .phase41-execution-workbench .phase41-table-panel {
      height: clamp(420px, 58dvh, 560px);
      max-height: clamp(420px, 58dvh, 560px);
      overflow: hidden;
    }
  }

  @media (max-width: 860px) {
    .phase41-status-workbench .phase41-table-panel,
    .phase41-execution-workbench .phase41-table-panel {
      height: min(560px, 62dvh);
      min-height: 0;
      max-height: min(560px, 62dvh);
      overflow: hidden;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .task-console,
    .task-console *,
    .task-console *::before,
    .task-console *::after {
      scroll-behavior: auto !important;
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
    }
  }

  /* Task details live inside the shared light Modal, so legacy dark-board colors must not leak in. */
  .task-detail-surface {
    --detail-strong: var(--wa-text-strong, #0d1722);
    --detail-text: var(--wa-text-main, #293847);
    --detail-muted: var(--wa-text-muted, #667789);
    --detail-subtle: var(--wa-text-subtle, #8a99aa);
    color: var(--detail-text);
    gap: 0;
  }

  .task-detail-surface .jira-modal-direct-btn {
    border-color: rgba(43, 116, 214, 0.2);
    background: rgba(43, 116, 214, 0.08);
    color: var(--wa-info, #2b74d6);
  }

  .task-detail-surface .view-telemetry-drawer-btn {
    min-height: 40px;
    border-color: rgba(0, 143, 150, 0.2);
    background: rgba(0, 143, 150, 0.07);
    color: var(--wa-accent-strong, #006f76);
    font-weight: 760;
  }

  .task-detail-surface .view-telemetry-drawer-btn:hover {
    border-color: rgba(0, 143, 150, 0.34);
    background: rgba(0, 143, 150, 0.12);
    color: var(--wa-accent-strong, #006f76);
  }

  .task-detail-surface .evidence-chain-summary-panel {
    border-color: rgba(121, 139, 159, 0.16);
    background: rgba(247, 250, 252, 0.86);
  }

  .task-detail-surface .node-meta,
  .task-detail-surface .node-time {
    color: var(--detail-muted);
  }

  .task-detail-surface .node-commit {
    color: var(--detail-strong);
  }

  .task-detail-surface .evidence-timeline {
    border-left-color: rgba(0, 143, 150, 0.24);
  }

  .task-detail-surface .timeline-node::before {
    background: var(--wa-accent, #008f96);
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.1);
  }

  .task-detail-surface .evidence-chain-empty {
    border-color: rgba(221, 75, 62, 0.18);
    background: rgba(221, 75, 62, 0.055);
    color: var(--wa-danger, #c83e32);
  }

  .task-detail-surface .signal-chip {
    border-color: rgba(121, 139, 159, 0.16);
    background: rgba(121, 139, 159, 0.09);
    color: var(--detail-muted);
  }

  .task-detail-surface {
    padding: 24px 28px 28px;
  }

  .task-drawer-hero {
    display: grid;
    gap: 18px;
    padding-bottom: 22px;
  }

  .task-drawer-title-row,
  .task-section-heading,
  .task-detail-footer {
    min-width: 0;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .task-drawer-title-row > div {
    min-width: 0;
  }

  .task-drawer-kicker {
    display: block;
    margin-bottom: 8px;
    color: var(--detail-muted);
    font-size: 12px;
    font-weight: 760;
  }

  .task-detail-surface .jira-modal-direct-btn {
    flex: none;
    min-height: 34px;
    margin: 0;
    padding: 0 12px;
    border-radius: var(--wa-radius-pill, 999px);
    white-space: nowrap;
  }

  .task-detail-pill-rail {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 7px;
    flex-wrap: nowrap;
    overflow-x: auto;
    overscroll-behavior-inline: contain;
    scrollbar-width: none;
  }

  .task-detail-pill-rail::-webkit-scrollbar {
    display: none;
  }

  .task-detail-pill {
    flex: 0 0 auto;
    min-height: 32px;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 0 10px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-pill, 999px);
    background: transparent;
    color: var(--detail-text);
    font-size: 12px;
    white-space: nowrap;
  }

  .task-detail-pill em {
    color: var(--detail-muted);
    font-style: normal;
  }

  .task-detail-pill strong {
    color: var(--detail-strong);
    font-weight: 760;
  }

  .task-detail-pill.id-pill {
    border-color: rgba(37, 107, 216, 0.2);
    background: var(--wa-info-soft);
    color: var(--wa-info);
    font-weight: 800;
  }

  .task-detail-pill.status-pill {
    border-color: rgba(4, 150, 111, 0.2);
    background: var(--wa-success-soft);
    color: var(--wa-success);
    font-weight: 780;
  }

  .task-detail-pill.type-bug {
    border-color: rgba(221, 75, 62, 0.2);
    background: var(--wa-danger-soft);
  }

  .task-evidence-progress {
    display: grid;
    gap: 8px;
  }

  .task-evidence-progress > div {
    height: 5px;
    overflow: hidden;
    border-radius: var(--wa-radius-pill, 999px);
    background: var(--wa-surface-inset);
  }

  .task-evidence-progress > div span {
    width: var(--task-progress);
    height: 100%;
    display: block;
    border-radius: inherit;
    background: var(--wa-accent);
  }

  .task-evidence-progress strong {
    color: var(--detail-text);
    font-weight: 720;
  }

  .task-detail-section {
    display: grid;
    gap: 10px;
    padding: 20px 0;
    border-top: 1px solid var(--wa-border-divider);
  }

  .task-section-heading {
    align-items: center;
  }

  .task-detail-surface .view-telemetry-drawer-btn {
    min-height: 34px;
    padding: 0 12px;
    border: 1px solid rgba(0, 143, 150, 0.2);
    border-radius: var(--wa-radius-pill, 999px);
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
    cursor: pointer;
    font: inherit;
    font-size: 12px;
    font-weight: 760;
    white-space: nowrap;
  }

  .task-code-facts {
    margin: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 18px;
  }

  .task-code-facts div {
    min-width: 0;
    display: grid;
    gap: 4px;
    padding: 10px 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .section-state {
    color: var(--detail-muted);
    font-size: 12px;
  }

  .task-detail-surface .evidence-chain-summary-panel {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    border: 0;
    border-top: 1px solid var(--wa-border-soft);
    border-bottom: 1px solid var(--wa-border-soft);
    border-radius: 0;
    background: transparent;
  }

  .task-detail-surface .chain-stat {
    min-width: 0;
    padding: 13px 10px;
  }

  .task-detail-surface .evidence-timeline {
    max-height: none;
    overflow: visible;
  }

  .task-detail-footer {
    align-items: center;
    padding-top: 18px;
    border-top: 1px solid var(--wa-border-divider);
    color: var(--detail-muted);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  @media (max-width: 640px) {
    .task-detail-surface {
      padding: 20px 16px 24px;
    }

    .task-drawer-title-row {
      align-items: flex-start;
    }

    .task-code-facts {
      grid-template-columns: 1fr;
    }

    .task-detail-footer {
      align-items: flex-start;
      flex-direction: column;
      gap: 5px;
    }

    .task-detail-surface .evidence-chain-summary-panel {
      grid-template-columns: 1fr;
    }
  }

  /* Task workbenches are viewport-bounded; details remain a concise peer panel. */
  .phase41-fact-pills dt,
  .phase41-fact-pills dd {
    font-size: 12px;
  }

  /* Tone is a compact status cue, never a full-card paint layer. */
  .task-console .phase41-metric,
  .task-console .phase41-stage-card {
    --task-tone: var(--wa-border-strong);
    --task-cell-background: var(--wa-neutral-soft);
    color: var(--wa-text-main);
  }

  .task-console .phase41-table-toolbar,
  .task-console .phase41-inspector {
    border-color: var(--wa-glass-outline);
    border-top-color: var(--wa-glass-highlight);
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel);
    box-shadow: var(--wa-shadow-glass);
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .task-console .view-toggle,
  .task-console .task-view-toggle,
  .task-console .toggle-btn,
  .task-console .custom-select-trigger,
  .task-console .task-filter-control,
  .phase41-filter-multi :global(.multi-select-trigger),
  .phase41-search-control input,
  .phase41-risk-strip {
    border-radius: var(--wa-radius-pill, 999px);
  }

  .task-console .phase41-metric.tone-info,
  .task-console .phase41-stage-card.tone-info {
    --task-tone: var(--wa-info);
    --task-cell-background: var(--wa-info-soft);
  }

  .task-console .phase41-metric.tone-success,
  .task-console .phase41-stage-card.tone-success {
    --task-tone: var(--wa-success);
    --task-cell-background: var(--wa-success-soft);
  }

  .task-console .phase41-metric.tone-warning,
  .task-console .phase41-stage-card.tone-warning {
    --task-tone: var(--wa-warning);
    --task-cell-background: var(--wa-warning-soft);
  }

  .task-console .phase41-metric.tone-danger,
  .task-console .phase41-stage-card.tone-danger {
    --task-tone: var(--wa-danger);
    --task-cell-background: var(--wa-danger-soft);
  }

  .task-console .phase41-metric.tone-neutral,
  .task-console .phase41-stage-card.tone-neutral {
    --task-tone: var(--wa-text-subtle);
    --task-cell-background: var(--wa-neutral-soft);
  }

  .task-console .phase41-summary-strip .surface-accent {
    --task-tone: var(--wa-accent-strong);
    --task-cell-background: var(--wa-accent-soft);
  }

  @media (min-width: 1181px) {
    .task-console {
      height: 100%;
      min-height: 0;
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden !important;
    }

    .task-console.view-execution {
      grid-template-rows: auto minmax(0, 1fr);
    }

    .phase41-workbench {
      height: 100%;
      min-height: 0;
      grid-auto-rows: minmax(0, 1fr);
      align-items: stretch;
      overflow: hidden;
    }

    .phase41-execution-workbench .phase41-table-panel,
    .phase41-execution-workbench .phase41-inspector,
    .phase41-status-workbench .phase41-table-panel,
    .phase41-status-workbench .phase41-inspector {
      height: 100%;
      min-height: 0;
      max-height: none;
    }

    .phase41-execution-workbench .phase41-table-shell,
    .phase41-status-workbench .phase41-table-shell {
      height: 100%;
      min-height: 0;
      overflow: auto;
    }

    .phase41-execution-workbench .phase41-inspector,
    .phase41-status-workbench .phase41-inspector {
      gap: 12px;
      overflow: hidden;
      overscroll-behavior: auto;
    }

    .phase41-inspector-section p {
      display: -webkit-box;
      overflow: hidden;
      line-clamp: 3;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 3;
    }

    .phase41-inspector-section li:nth-child(n + 3) {
      display: none;
    }

    .phase41-execution-workbench .phase41-inspector-section p {
      display: block;
      overflow: visible;
      line-clamp: unset;
      -webkit-line-clamp: unset;
    }

    .phase41-execution-workbench .phase41-inspector-section li:nth-child(n + 3) {
      display: list-item;
    }
  }

  .phase41-fact-pills {
    flex-wrap: nowrap;
    overflow-x: auto;
    overscroll-behavior-inline: contain;
    scrollbar-width: none;
  }

  .phase41-fact-pills::-webkit-scrollbar {
    display: none;
  }

  .phase41-type-label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--wa-text-main);
    font-size: 12px;
    font-weight: 760;
    white-space: nowrap;
  }

  .phase41-type-label .phase41-type-icon {
    width: 24px;
    height: 24px;
    font-size: 11px;
    font-weight: 820;
  }

  .phase41-project-cell {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .phase41-project-cell strong,
  .phase41-project-cell small,
  .phase41-release-cell {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .phase41-project-cell strong,
  .phase41-release-cell {
    color: var(--wa-text-main);
    font-size: 12.5px;
  }

  .phase41-project-cell small {
    color: var(--wa-text-muted);
    font-size: 10.5px;
  }

  .task-detail-modal-body {
    min-width: 0;
    display: grid;
    align-content: start;
    gap: 12px;
    padding: 16px 22px 18px;
  }

  .task-detail-modal-meta {
    min-width: 0;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding-bottom: 12px;
    border-bottom: 1px solid rgba(116, 139, 156, 0.18);
  }

  .task-detail-overview {
    min-width: 0;
    display: grid;
    gap: 10px;
    padding: 0 0 14px;
    border-bottom: 1px solid rgba(116, 139, 156, 0.18);
  }

  .task-detail-section-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
  }

  .task-detail-section-heading h3,
  .task-detail-section-heading span {
    margin: 0;
  }

  .task-detail-section-heading h3,
  .task-detail-panel h3 {
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 800;
  }

  .task-detail-section-heading span {
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 660;
  }

  .task-detail-fact-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px 20px;
    margin: 0;
  }

  .task-detail-fact-grid div {
    min-width: 0;
    padding: 2px 0;
  }

  .task-detail-fact-grid dt {
    color: var(--wa-text-muted);
    font-size: 11.5px;
    line-height: 1.25;
    font-weight: 740;
  }

  .task-detail-fact-grid dd {
    margin: 2px 0 0;
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.35;
    font-weight: 760;
    overflow-wrap: anywhere;
  }

  .task-detail-section-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1.3fr) minmax(0, 0.7fr);
    gap: 0;
  }

  .task-detail-panel {
    min-width: 0;
    display: grid;
    align-content: start;
    gap: 6px;
    padding: 14px 0;
  }

  .task-detail-panel:nth-child(odd) {
    padding-right: 20px;
  }

  .task-detail-panel:nth-child(even) {
    padding-left: 20px;
    border-left: 1px solid rgba(116, 139, 156, 0.18);
  }

  .task-detail-panel:nth-child(-n + 2) {
    padding-top: 2px;
    border-bottom: 1px solid rgba(116, 139, 156, 0.18);
  }

  .task-detail-panel h3,
  .task-detail-panel p,
  .task-detail-panel ul {
    margin: 0;
  }

  .task-detail-panel p,
  .task-detail-panel li {
    color: var(--wa-text-main);
    font-size: 12.5px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .task-detail-panel ul {
    display: grid;
    gap: 5px;
    padding-left: 16px;
  }

  .task-detail-modal-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }

  .task-detail-modal-actions a {
    text-decoration: none;
  }

  .task-completion-confirmation {
    width: 100%;
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 18px;
  }

  .task-completion-copy {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .task-completion-copy strong {
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.35;
  }

  .task-completion-copy span,
  .task-completion-error {
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.45;
    text-wrap: pretty;
  }

  .task-completion-error {
    color: var(--wa-danger);
    font-weight: 650;
  }

  .task-completion-actions {
    flex: none;
    display: flex;
    gap: 8px;
  }

  .phase41-status-workbench .phase41-fact-pills {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
    overflow: visible;
  }

  .phase41-status-workbench .phase41-fact-pills > div {
    width: auto;
    min-height: 44px;
    display: grid;
    align-content: center;
    gap: 2px;
    padding: 7px 0;
    border: 0;
    border-bottom: 1px solid var(--wa-border-soft);
    border-radius: 0;
  }

  .phase41-status-workbench .phase41-fact-pills dt,
  .phase41-status-workbench .phase41-fact-pills dd {
    white-space: normal;
  }

  @media (min-width: 1181px) {
    .phase41-execution-workbench .phase41-inspector,
    .phase41-status-workbench .phase41-inspector {
      align-content: start;
      overflow-y: auto;
      overscroll-behavior: contain;
    }

    .phase41-status-workbench .phase41-inspector-section p {
      display: block;
      overflow: visible;
      line-clamp: unset;
      -webkit-line-clamp: unset;
    }

    .phase41-status-workbench .phase41-inspector-section li:nth-child(n + 3) {
      display: list-item;
    }
  }

  @media (max-width: 760px) {
    .task-detail-modal-body {
      padding: 14px 16px 18px;
    }

    .task-detail-fact-grid,
    .task-detail-section-grid,
    .phase41-status-workbench .phase41-fact-pills {
      grid-template-columns: 1fr;
    }

    .task-detail-panel,
    .task-detail-panel:nth-child(odd),
    .task-detail-panel:nth-child(even),
    .task-detail-panel:nth-child(-n + 2) {
      padding: 12px 0;
      border-left: 0;
      border-bottom: 1px solid rgba(116, 139, 156, 0.18);
    }

    .task-detail-section-heading {
      align-items: flex-start;
      flex-direction: column;
      gap: 4px;
    }

    .task-detail-modal-actions {
      width: 100%;
      flex-wrap: wrap;
    }

    .task-detail-modal-actions .wa-admin-action {
      flex: 1 1 0;
      justify-content: center;
      min-height: 44px;
      white-space: nowrap;
    }

    .task-completion-confirmation {
      align-items: stretch;
      flex-direction: column;
      gap: 12px;
    }

    .task-completion-actions {
      width: 100%;
    }
  }

  @media (max-width: 1180px) {
    .task-console,
    .task-console.view-execution {
      align-self: start;
      height: auto !important;
      grid-template-rows: auto auto;
      overflow: visible !important;
    }
  }

</style>
