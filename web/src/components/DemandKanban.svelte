<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { slide } from 'svelte/transition';
  import type { AdminInspectorRecord, AdminMetric, AdminTableColumn, AdminTableRow, AdminTone } from '../lib/admin-console/contract';
  import { ADMIN_TONE_CLASS, formatAdminDate, toneForRisk, toneForStatus } from '../lib/admin-console/contract';
  import { lockBodyScroll, unlockBodyScroll } from '../lib/modalScrollLock';
  import { readDeconstructStream } from '../lib/deconstruct-stream';
  import { showToast } from '../lib/toast';
  import {
    fetchDeliveryDirectory,
    type DeliveryProjectOption
  } from '../lib/delivery-directory';
  import CommitTelemetryPanel from './CommitTelemetryPanel.svelte';
  import Deconstructor from './Deconstructor.svelte';
  import DemandDeliveryControl from './DemandDeliveryControl.svelte';
  import Select from './shared/Select.svelte';

  type DemandView = 'board' | 'schedule';

  export let currentUserPermissions: string[] = [];
  export let currentUserName: string = '';
  export let currentUserEmail: string = '';
  export let currentUserDepartment: string = '';
  export let activeDemandView: DemandView = 'schedule';

  function hasPermission(perm: string): boolean {
    return currentUserPermissions.includes(perm);
  }

  interface Demand {
    task_id: string;
    title: string;
    description: string;
    repo: string;
    assignee: string;
    creator?: string;
    creator_dept?: string;
    branch: string;
    last_commit: string;
    status: string;
    issue_type: string;
    task_created_at: string;
    last_update: string;
    completed_at?: string;
    due_date?: string;
    task_group_id?: string;
    estimate_days?: number;
    estimate_hours?: number;
    difficulty?: string;
    estimate_source?: string;
    scheduled?: boolean;
  }

  type ScheduleRiskFilter = 'attention' | 'all' | 'overdue' | 'due_soon' | 'stale' | 'unscheduled' | 'safe' | 'done';
  type ScheduleSortMode = 'risk' | 'due' | 'owner';
  type EstimateSource = '' | 'ai_deconstruct' | 'manual_adjusted';

  interface ScheduleSummary {
    total: number;
    scheduled: number;
    unscheduled: number;
    in_progress: number;
    review: number;
    done: number;
    overdue: number;
    due_soon: number;
    stale: number;
  }

  interface ScheduleItem {
    demand_id: string;
    title: string;
    description: string;
    assignee: string;
    department: string;
    repo: string;
    branch: string;
    status: string;
    task_group_id: string;
    scheduled: boolean;
    due_date: string;
    created_at: string;
    last_update: string;
    completed_at?: string;
    mr_url?: string;
    estimate_days: number;
    estimate_hours: number;
    difficulty: string;
    risk_level: string;
    risk_label: string;
    risk_reason: string;
    risk_rank: number;
    days_remaining: number;
    subtask_total: number;
    subtask_done: number;
    subtask_active: number;
    subtask_review: number;
    issue_type: string;
    project_key?: string;
    project_priority?: string;
  }

  interface ScheduleResponse {
    generated_at: string;
    summary: ScheduleSummary;
    items: ScheduleItem[];
  }

  type ScheduleRiskBucketKey = 'this_week' | 'next_week' | 'later';

  interface ScheduleRiskCalendarCounts {
    total: number;
    overdue: number;
    due_soon: number;
    stale: number;
    unscheduled: number;
  }

  interface ScheduleRiskCalendarEvent {
    id?: string;
    demand_id: string;
    title: string;
    assignee: string;
    project_key: string;
    week_key?: string;
    risk_type?: string;
    risk_level: string;
    risk_label: string;
    risk_reason: string;
    due_date: string;
    days_remaining: number;
  }

  interface ScheduleRiskCalendarBucket {
    key: ScheduleRiskBucketKey;
    label: string;
    window_label: string;
    counts: ScheduleRiskCalendarCounts;
    events: ScheduleRiskCalendarEvent[];
    event_ids?: string[];
  }

  interface DeconstructEstimateResponse {
    analysis?: {
      overall_estimated_days?: number;
      overall_estimated_hours?: number;
      overall_difficulty?: string;
      estimate_basis?: string;
      confidence?: number;
    };
  }

  const scheduleRiskFilters: Array<{ value: ScheduleRiskFilter; label: string }> = [
    { value: 'attention', label: '需关注' },
    { value: 'all', label: '全部' },
    { value: 'overdue', label: '逾期' },
    { value: 'due_soon', label: '临期' },
    { value: 'stale', label: '滞后' },
    { value: 'unscheduled', label: '待排期' },
    { value: 'safe', label: '正常' },
    { value: 'done', label: '已交付' }
  ];

  const riskCalendarRiskTypes: Array<{ value: ScheduleRiskFilter; label: string }> = [
    { value: 'overdue', label: '逾期' },
    { value: 'due_soon', label: '临期' },
    { value: 'stale', label: '滞后' },
    { value: 'unscheduled', label: '待排期' }
  ];

  const riskCalendarBucketMeta: Array<{ key: ScheduleRiskBucketKey; label: string; window_label: string }> = [
    { key: 'this_week', label: '本周', window_label: '本周截止与已逾期' },
    { key: 'next_week', label: '下周', window_label: '下周到期风险' },
    { key: 'later', label: '后续', window_label: '未排期与远期风险' }
  ];

  const scheduleSortModes: Array<{ value: ScheduleSortMode; label: string }> = [
    { value: 'risk', label: '风险优先' },
    { value: 'due', label: '截止日' },
    { value: 'owner', label: '负责人' }
  ];

  const scheduleTableColumns: AdminTableColumn[] = [
    { key: 'demand', label: '需求', width: '28%' },
    { key: 'priority', label: '优先级', width: '8%', align: 'center' },
    { key: 'owner', label: '负责人', width: '11%' },
    { key: 'risk', label: '风险', width: '14%' },
    { key: 'dueDate', label: '计划日', width: '12%' },
    { key: 'status', label: '状态', width: '10%', align: 'center' },
    { key: 'evidence', label: '检查', width: '7%', align: 'center' }
  ];

  const difficultyOptions: Array<{ value: string; label: string }> = [
    { value: '', label: '未设置' },
    { value: 'Low', label: '低' },
    { value: 'Medium', label: '中' },
    { value: 'High', label: '高' }
  ];

  function createEmptyScheduleSummary(): ScheduleSummary {
    return {
      total: 0,
      scheduled: 0,
      unscheduled: 0,
      in_progress: 0,
      review: 0,
      done: 0,
      overdue: 0,
      due_soon: 0,
      stale: 0
    };
  }

  function createEmptyRiskCalendarCounts(): ScheduleRiskCalendarCounts {
    return {
      total: 0,
      overdue: 0,
      due_soon: 0,
      stale: 0,
      unscheduled: 0
    };
  }

  function createEmptyRiskCalendarBuckets(): ScheduleRiskCalendarBucket[] {
    return riskCalendarBucketMeta.map((bucket) => ({
      ...bucket,
      counts: createEmptyRiskCalendarCounts(),
      events: []
    }));
  }

  function canManageDemand(item: Demand): boolean {
    if (hasPermission('demands:write')) return true;
    if (!item.creator) return false;
    const lowerCreator = item.creator.toLowerCase();
    const lowerName = currentUserName.toLowerCase();
    const lowerEmail = currentUserEmail.toLowerCase();
    return lowerCreator === lowerName || lowerEmail.includes(lowerCreator);
  }

  let allSubTasks: any[] = [];
  let expandedDemands: { [key: string]: boolean } = {};

  function getSubTasksForDemand(taskGroupId?: string) {
    if (!taskGroupId || taskGroupId === '-' || taskGroupId === '') return [];
    return subTasksMap.get(taskGroupId) || [];
  }

  function toggleDemandSubtasks(taskId: string) {
    expandedDemands[taskId] = !expandedDemands[taskId];
    expandedDemands = expandedDemands;
  }

  // Confirm Modal States
  let showConfirmModal = false;
  let confirmTitle = '';
  let confirmMessage = '';
  let confirmType: 'delete' | 'archive' = 'delete';
  let confirmTaskId = '';
  let confirmAction: () => Promise<void> = async () => {};

  async function handleDeleteDemand(taskId: string) {
    confirmTitle = '删除需求';
    confirmMessage = '此操作会从系统中移除该需求和关联看板记录，执行后无法恢复。';
    confirmTaskId = taskId;
    confirmType = 'delete';
    confirmAction = async () => {
      showConfirmModal = false;
      const token = localStorage.getItem('jwt_token');
      try {
        const res = await fetch(`/api/demands?task_id=${taskId}`, {
          method: 'DELETE',
          headers: { 'Authorization': `Bearer ${token}` }
        });
        if (res.ok) {
          await refreshDemandWorkspace();
        } else {
          const data = await res.json();
          alert(`删除失败: ${data.message || res.statusText}`);
        }
      } catch (e) {
        console.error('Delete error:', e);
        alert('网络连接错误，删除需求失败');
      }
    };
    showConfirmModal = true;
  }

  async function handleArchiveDemand(taskId: string) {
    confirmTitle = '归档需求';
    confirmMessage = '归档后该需求会离开当前流转看板，历史数据仍会保留。';
    confirmTaskId = taskId;
    confirmType = 'archive';
    confirmAction = async () => {
      showConfirmModal = false;
      const token = localStorage.getItem('jwt_token');
      try {
        const res = await fetch('/api/demands/archive', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
          },
          body: JSON.stringify({ task_id: taskId })
        });
        if (res.ok) {
          await refreshDemandWorkspace();
        } else {
          const data = await res.json();
          alert(`归档失败: ${data.message || res.statusText}`);
        }
      } catch (e) {
        console.error('Archive error:', e);
        alert('网络连接错误，归档需求失败');
      }
    };
    showConfirmModal = true;
  }

  interface UserOption {
    id: number;
    username: string;
    email?: string;
    name: string;
    department: string;
  }

  let isMounted = false;
  let demands: Demand[] = [];
  let demandsById: Map<string, Demand> = new Map();
  let users: UserOption[] = [];
  let demandOptionAssignees: string[] = [];
  let demandOptionAssigneesLoaded = false;
  let loading = false;
  let errorMsg = '';
  let flowRequestSeq = 0;
  let scheduleItems: ScheduleItem[] = [];
  let scheduleSummary: ScheduleSummary = createEmptyScheduleSummary();
  let scheduleGeneratedAt = '';
  let scheduleLoading = false;
  let scheduleErrorMsg = '';
  let riskCalendarBuckets: ScheduleRiskCalendarBucket[] = createEmptyRiskCalendarBuckets();
  let riskCalendarGeneratedAt = '';
  let riskCalendarLoading = false;
  let riskCalendarErrorMsg = '';
  let riskCalendarUsingFallback = false;
  let riskCalendarRequestSeq = 0;
  let scheduleSearch = '';
  let scheduleSearchInput = '';
  let scheduleSearchDebounceTimer: any;

  function handleScheduleSearch(event: Event) {
    const inputVal = (event.target as HTMLInputElement).value;
    scheduleSearchInput = inputVal;
    clearTimeout(scheduleSearchDebounceTimer);
    scheduleSearchDebounceTimer = setTimeout(() => {
      scheduleSearch = inputVal;
    }, 200);
  }

  let projectSearchText = '';
  let assigneeSearchText = '';
  let scheduleAssigneeSearchText = '';
  let scheduleRiskFilter: ScheduleRiskFilter = 'attention';
  let scheduleAssigneeFilter = 'all';
  let scheduleSortMode: ScheduleSortMode = 'risk';
  let scheduleTypeFilter: 'all' | 'demand' | 'bug' = 'all';
  let scheduleProjectFilter = 'all';
  let scheduleProjectSearchText = '';
  let selectedScheduleDemandId = '';
  let scheduleInspectorMode: 'schedule' | 'telemetry' = 'schedule';
  let scheduleDraftDemandId = '';
  let scheduleSaveLoading = false;
  let scheduleSaveError = '';
  let pendingGlobalSearchDemandId = '';
  let showScheduleProjectDropdown = false;
  let showRiskDropdown = false;
  let showTypeDropdown = false;
  let showSortDropdown = false;
  let projectConfigs: any[] = [];
  let deliveryProjects: DeliveryProjectOption[] = [];
  let deliveryDirectoryReady = false;
  $: projectConfigMap = new Map<string, any>(
    projectConfigs
      .filter(c => c && c.project_key)
      .map(c => [c.project_key.toUpperCase(), c])
  );
  let telemetryScores: any[] = [];
  let activeTelemetryTaskId = '';
  let isTelemetryDrawerOpen = false;

  let coreMembers = new Set<string>();
  let coreMemberAliases = new Set<string>();

  function isCoreMember(name: string): boolean {
    if (!name || name === '未指派' || name === '-' || name === 'Unassigned') return true;
    if (!deliveryDirectoryReady) return true;
    const normalized = name.trim().toLocaleLowerCase('zh-CN');
    return coreMemberAliases.has(normalized) || coreMemberAliases.has(normalized.split(' ')[0]);
  }

  $: subTasksMap = (() => {
    const map = new Map<string, any[]>();
    for (let i = 0; i < allSubTasks.length; i++) {
      const t = allSubTasks[i];
      const gid = t.task_group_id;
      if (!gid || gid === '-' || gid === '') continue;
      if (!isCoreMember(t.assignee)) continue;
      let list = map.get(gid);
      if (!list) {
        list = [];
        map.set(gid, list);
      }
      list.push(t);
    }
    return map;
  })();

  let scheduleScrollTop = 0;
  let scheduleContainerHeight = 550;
  let scheduleContainerEl: HTMLDivElement;
  let scheduleWorkbenchEl: HTMLDivElement;
  let scheduleTablePanelEl: HTMLDivElement;
  const scheduleItemHeight = 48;
  const scheduleTableHeaderHeight = 40;
  const scheduleEmptyStateHeight = 96;
  let lastScheduleLayoutSignature = '';
  let scheduleLayoutStabilizeSeq = 0;

  let collapsedProjects: {[key: string]: boolean} = {};

  function toggleProjectCollapse(projKey: string, currentCollapsed: boolean) {
    collapsedProjects[projKey] = !currentCollapsed;
    collapsedProjects = collapsedProjects;
  }

  function getProjectKey(taskID: string): string {
    if (!taskID) return 'UNKNOWN';
    const idx = taskID.indexOf('-');
    if (idx <= 0) return 'UNKNOWN';
    return taskID.substring(0, idx).toUpperCase();
  }

  function computeCompositeWeight(item: ScheduleItem, priorityOverride?: string): number {
    const priority = priorityOverride || item.project_priority || getProjectPriority(item.demand_id);
    let baseWeight = 40; // Default P2
    if (priority === 'P0') baseWeight = 60;
    else if (priority === 'P1') baseWeight = 50;
    else if (priority === 'P2') baseWeight = 40;
    else if (priority === 'P3') baseWeight = 30;
    else if (priority === 'P4') baseWeight = 20;
    else if (priority === 'P5') baseWeight = 10;

    let riskWeight = 4;
    const risk = (item.risk_level || '').toLowerCase();
    if (risk === 'overdue' || risk === 'danger') {
      riskWeight = 20;
    } else if (risk === 'warning') {
      riskWeight = 12;
    } else if (risk === 'safe') {
      riskWeight = 4;
    } else if (risk === 'done' || item.status === 'done') {
      riskWeight = 0;
    }

    let urgencyWeight = 0;
    if (item.scheduled && item.due_date) {
      const days = item.days_remaining;
      if (days <= 0) {
        urgencyWeight = 20;
      } else {
        urgencyWeight = Math.max(0, 20 * (1 - days / 30));
      }
    }

    return baseWeight + riskWeight + urgencyWeight;
  }

  function getScheduleRiskTone(value?: string): AdminTone {
    const level = normalizeRiskLevel(value);
    if (level === 'overdue') return 'danger';
    if (level === 'due_soon' || level === 'stale' || level === 'unscheduled') return 'warning';
    if (level === 'safe' || level === 'done') return 'success';
    return toneForRisk(value);
  }

  function getScheduleStatusTone(item: ScheduleItem): AdminTone {
    const statusClass = getScheduleStatusClass(item);
    if (statusClass === 'done') return 'success';
    if (statusClass === 'progress') return 'info';
    if (statusClass === 'review' || statusClass === 'partial' || statusClass === 'unscheduled') return 'warning';
    if (statusClass === 'scheduled') return 'info';
    return toneForStatus(getScheduleStatusLabel(item));
  }

  function getPriorityTone(priority?: string): AdminTone {
    if (priority === 'P0' || priority === 'P1') return 'danger';
    if (priority === 'P2') return 'warning';
    if (priority === 'P3') return 'info';
    return 'neutral';
  }

  function getScheduleIssueTypeLabel(item: ScheduleItem): string {
    return item.issue_type === 'bug' || item.issue_type === '缺陷' || item.issue_type === '故障' || item.issue_type === 'defect'
      ? '缺陷'
      : '需求';
  }

  function getScheduleProjectLabel(item: ScheduleItem): string {
    const projectKey = item.project_key || getProjectKey(item.demand_id);
    return getProjectName(projectKey) || projectKey || '暂不指定项目';
  }

  function buildScheduleChecklist(item: ScheduleItem): string[] {
    return [
      item.due_date ? `计划完成日已锁定：${formatScheduleDate(item.due_date)}` : '缺少计划完成日期',
      hasScheduleValue(item.branch) ? `开发入口已记录：${item.branch}` : '缺少开发入口或分支',
      item.subtask_total > 0 ? `AI 解构任务进度：${item.subtask_done}/${item.subtask_total}` : '尚未导入 AI 解构任务',
      hasDeliveryEvidence(item) ? '已有仓库、分支或 MR 交付证据' : '暂无仓库、分支或 MR 证据',
      item.estimate_hours > 0 || item.estimate_days > 0 ? `工时估算：${formatScheduleEffort(item)}` : '工时仍待评估'
    ];
  }

  function buildScheduleTableRow(item: ScheduleItem): AdminTableRow {
    const priority = item.project_priority || getProjectPriority(item.demand_id);
    const riskLabel = item.risk_label || getRiskLevelLabel(item.risk_level);
    const status = getScheduleStatusLabel(item);

    return {
      id: item.demand_id,
      title: item.title,
      status,
      tone: getScheduleStatusTone(item),
      owner: item.assignee || '未指派',
      dueDate: formatAdminDate(item.due_date),
      priority,
      risk: riskLabel,
      cells: {
        issueType: getScheduleIssueTypeLabel(item),
        project: getScheduleProjectLabel(item),
        priority,
        owner: item.assignee || '未指派',
        risk: riskLabel,
        riskReason: item.risk_reason || '',
        dueDate: formatAdminDate(item.due_date),
        status,
        effort: formatScheduleEffort(item),
        subtasks: `${item.subtask_done}/${item.subtask_total || 0}`,
        updatedAt: formatAdminDate(item.last_update)
      }
    };
  }

  function buildScheduleInspectorRecord(item: ScheduleItem | null | undefined): AdminInspectorRecord | null {
    if (!item) return null;
    const priority = item.project_priority || getProjectPriority(item.demand_id);
    const progress = getScheduleProgress(item);
    return {
      id: item.demand_id,
      title: item.title,
      status: getScheduleStatusLabel(item),
      tone: getScheduleStatusTone(item),
      facts: [
        { label: '负责人', value: item.assignee || '未指派' },
        { label: '提出日期', value: formatAdminDate(item.created_at) },
        { label: '计划完成日期', value: formatAdminDate(item.due_date) },
        { label: '所属项目', value: getScheduleProjectLabel(item) },
        { label: '关联目标', value: priority || 'P2' },
        { label: '当前阶段', value: getScheduleStatusLabel(item) },
        { label: '预计工作量', value: formatScheduleEffort(item) },
        { label: '任务组', value: item.task_group_id || '未绑定' }
      ],
      sections: [
        {
          title: '需求描述',
          body: item.description || '当前需求没有补充描述。'
        },
        {
          title: '验收标准',
          items: buildScheduleChecklist(item)
        },
        {
          title: '风险说明',
          body: item.risk_reason || '当前接口未返回额外风险说明。'
        },
        {
          title: 'AI 解构',
          items: [
            `影子任务完成度：${progress}%`,
            `影子任务数量：${item.subtask_done}/${item.subtask_total || 0}`,
            `最近更新：${formatAdminDate(item.last_update)}`
          ]
        }
      ],
      actions: [
        { label: '编辑排期', kind: 'primary' },
        { label: '代码轨迹', kind: 'secondary' }
      ]
    };
  }

  $: flatRenderList = (() => {
    const sorted = [...filteredScheduleItems];
    sorted.sort((a, b) => compareScheduleItems(a, b));

    return sorted.map((item) => ({
      type: 'item',
      projectKey: item.project_key || getProjectKey(item.demand_id),
      data: item,
      row: buildScheduleTableRow(item),
      id: item.demand_id
    }));
  })();

  $: scheduleStartIndex = Math.max(0, Math.floor(scheduleScrollTop / scheduleItemHeight) - 2);
  $: scheduleViewportHeight = scheduleContainerHeight > 0 ? scheduleContainerHeight : 550;
  $: scheduleVirtualRowCount = flatRenderList.length;
  $: scheduleTableContentHeight = scheduleTableHeaderHeight + (scheduleVirtualRowCount > 0 ? scheduleVirtualRowCount * scheduleItemHeight : scheduleEmptyStateHeight);
  $: scheduleLayoutSignature = [
    activeDemandView,
    scheduleLoading ? 'schedule-loading' : 'schedule-idle',
    scheduleErrorMsg ? 'schedule-error' : 'schedule-ok',
    riskCalendarLoading ? 'risk-loading' : 'risk-idle',
    riskCalendarUsingFallback ? 'fallback' : 'live',
    riskCalendarErrorMsg ? 'risk-error' : 'risk-ok',
    scheduleItems.length,
    scheduleVirtualRowCount,
    scheduleTableContentHeight,
    riskCalendarBuckets.map((bucket) => `${bucket.key}:${bucket.counts.total}:${bucket.events.length}`).join('|')
  ].join('~');
  $: stabilizeScheduleLayout(scheduleLayoutSignature);
  $: scheduleMaxScrollTop = Math.max(0, scheduleVirtualRowCount * scheduleItemHeight - scheduleViewportHeight);
  $: if (scheduleContainerEl && scheduleScrollTop > scheduleMaxScrollTop) {
    scheduleScrollTop = scheduleMaxScrollTop;
    scheduleContainerEl.scrollTop = scheduleMaxScrollTop;
  }
  $: scheduleEndIndex = Math.min(flatRenderList.length, Math.ceil((scheduleScrollTop + scheduleViewportHeight) / scheduleItemHeight) + 2);
  $: visibleScheduleRows = flatRenderList.slice(scheduleStartIndex, scheduleEndIndex);
  $: scheduleTopPadding = scheduleStartIndex * scheduleItemHeight;
  $: scheduleBottomPadding = (flatRenderList.length - scheduleEndIndex) * scheduleItemHeight;

  function handleScheduleScroll(e: Event) {
    const target = e.target as HTMLDivElement;
    scheduleContainerHeight = target.clientHeight || scheduleContainerHeight;
    scheduleScrollTop = target.scrollTop;
  }

  async function stabilizeScheduleLayout(nextSignature: string) {
    if (!isMounted || activeDemandView !== 'schedule' || !scheduleWorkbenchEl || typeof window === 'undefined') {
      lastScheduleLayoutSignature = nextSignature;
      return;
    }
    if (lastScheduleLayoutSignature === nextSignature) return;

    const scrollBefore = window.scrollY || document.documentElement.scrollTop || 0;
    const viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
    const tableRectBefore = scheduleTablePanelEl?.getBoundingClientRect();
    const anchorEl = tableRectBefore && tableRectBefore.top < viewportHeight ? scheduleTablePanelEl : scheduleWorkbenchEl;
    const rectBefore = anchorEl.getBoundingClientRect();
    const shouldPreserveViewport = rectBefore.top < viewportHeight;

    lastScheduleLayoutSignature = nextSignature;
    if (!shouldPreserveViewport) return;

    const topBefore = rectBefore.top;
    const seq = ++scheduleLayoutStabilizeSeq;
    await tick();

    requestAnimationFrame(() => {
      if (seq !== scheduleLayoutStabilizeSeq || !anchorEl.isConnected) return;
      const rectAfter = anchorEl.getBoundingClientRect();
      const heightDelta = rectAfter.height - rectBefore.height;

      if (rectBefore.bottom <= 0 && Math.abs(heightDelta) > 1) {
        window.scrollTo({ top: Math.max(0, scrollBefore + heightDelta), left: 0, behavior: 'auto' });
        return;
      }

      const topDelta = rectAfter.top - topBefore;
      if (Math.abs(topDelta) > 1) {
        window.scrollBy({ top: topDelta, left: 0, behavior: 'auto' });
      }
    });
  }

  function getProjectPriority(taskID: string): string {
    if (!taskID) return 'P2';
    const idx = taskID.indexOf('-');
    if (idx <= 0) return 'P2';
    const key = taskID.substring(0, idx).toUpperCase();
    const config = projectConfigMap.get(key);
    return config ? config.base_priority : 'P2';
  }

  function getProjectName(projKey: string): string {
    if (!projKey) return '';
    const config = projectConfigMap.get(projKey.toUpperCase());
    if (config?.project_name) return config.project_name;
    return deliveryProjects.find((project) => project.project_key === projKey.toUpperCase())?.project_name || '';
  }

  function getProjectDisplayName(projKey: string): string {
    if (!projKey || projKey === '-') return '暂不指定项目';
    const keyUpper = projKey.toUpperCase();
    const config = projectConfigMap.get(keyUpper);
    if (config && config.project_name && config.project_name !== `${projKey}项目`) {
      return `${config.project_name} (${projKey})`;
    }
    const directoryProject = deliveryProjects.find((project) => project.project_key === keyUpper);
    if (directoryProject?.project_name && directoryProject.project_name !== `${projKey}项目`) {
      return `${directoryProject.project_name} (${projKey})`;
    }
    const score = telemetryScores.find(s => s && s.project_key && s.project_key.toUpperCase() === keyUpper);
    if (score && score.project_name && score.project_name !== `${projKey}项目`) {
      return `${score.project_name} (${projKey})`;
    }
    return projKey;
  }

  function getPriorityWeight(p?: string): number {
    if (p === 'P0') return 600;
    if (p === 'P1') return 500;
    if (p === 'P2') return 400;
    if (p === 'P3') return 300;
    if (p === 'P4') return 200;
    if (p === 'P5') return 100;
    return 400; // default P2
  }

  function compareDemandsByPriority(a: any, b: any): number {
    const priorityA = getProjectPriority(a.task_id);
    const priorityB = getProjectPriority(b.task_id);
    const weightA = getPriorityWeight(priorityA);
    const weightB = getPriorityWeight(priorityB);
    if (weightA !== weightB) {
      return weightB - weightA;
    }
    return a.task_id.localeCompare(b.task_id);
  }

  async function fetchProjectConfigs() {
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/projects/config', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const payload = await res.json();
        projectConfigs = Array.isArray(payload) ? payload : [];
      }
    } catch (err) {
      console.error('Failed to fetch project configs:', err);
    }
  }

  async function fetchTelemetryScores() {
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/projects/scores', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        telemetryScores = await res.json();
      }
    } catch (e) {
      console.error('Failed to fetch telemetry scores:', e);
    }
  }

  // Modal States
  let showCreateModal = false;
  let showScheduleModal = false;
  let showDemandDetailsModal = false;
  let showDeconstructorModal = false;
  let selectedDemand: Demand | null = null;
  let detailDemand: Demand | null = null;
  let deconstructorTitle = '';
  let deconstructorDescription = '';
  let deconstructorDemandId = '';
  let deconstructorContextLabel = '需求解构';
  let deconstructorPresentation: 'modal' | 'companion' = 'modal';
  let deconstructorHost: 'none' | 'create' | 'schedule' | 'details' = 'none';
  let companionHostHeight = 560;
  let manualModalScrollLocked = false;
  let detailDrawerEl: HTMLElement;
  let detailCloseButton: HTMLButtonElement;
  let detailReturnFocus: HTMLElement | null = null;
  let deconstructorCloseButton: HTMLButtonElement;
  let deconstructorReturnFocus: HTMLElement | null = null;

  function portalToConsole(node: HTMLElement, enabled = true) {
    if (!enabled) return {};
    const target = document.querySelector<HTMLElement>('.functional-console') || document.body;
    target.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  function portalToWorkspaceStage(node: HTMLElement, enabled = true) {
    if (!enabled) return {};
    const target = node.closest<HTMLElement>('.workspace-stage')
      || document.querySelector<HTMLElement>('.workspace-stage');
    if (!target) return {};
    target.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  // Dropdown States
  let showAssigneeDropdown = false;
  let showProjectDropdown = false;
  let showScheduleAssigneeDropdown = false;
  let activeDatePicker: 'new' | 'schedule' | null = null;
  let datePickerCursor = new Date();
  let jiraBaseUrl = '';

  // Create Form Fields
  let newTitle = '';
  let newDescription = '';
  let newAssignee = '';
  let newRepo = '-';
  let newDueDate = '';
  let newDueDateDisplay = '';

  // Schedule Form Fields
  let schedDueDate = '';
  let schedDueDateDisplay = '';
  let schedTaskGroupID = '-';
  let schedEstimateHours = 0;
  let schedEstimateDays = 0;
  let schedDifficulty = '';
  let schedEstimateSource: EstimateSource = '';
  let scheduleEstimateLoading = false;
  let scheduleEstimateError = '';
  let schedAssignee = '';
  let schedAssigneeSearchText = '';
  let showScheduleModalAssigneeDropdown = false;
  const estimateHoursPerDay = 8;

  const monthNames = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'];
  const weekdayNames = ['一', '二', '三', '四', '五', '六', '日'];

  function formatDateDisplay(value: string): string {
    if (!value) return '';
    const [year, month, day] = value.split('-');
    if (!year || !month || !day) return value;
    return `${year}.${month}.${day}`;
  }

  function normalizeDepartment(department: string): string {
    const trimmed = department.trim();
    if (!trimmed || trimmed === '未分配' || trimmed === '无部门') return '';
    return trimmed;
  }

  function createBrainGroupId(taskId: string) {
    const clean = taskId.replace(/[^A-Za-z0-9-]/g, '').toLowerCase();
    return clean ? `brain-${clean}` : `brain-${Date.now()}`;
  }

  function getEffectiveTaskGroupId(demand: Demand) {
    if (demand.task_group_id && demand.task_group_id !== '-') return demand.task_group_id;
    return createBrainGroupId(demand.task_id);
  }

  function hasTaskGroup(taskGroupId?: string) {
    return !!taskGroupId && taskGroupId !== '-' && taskGroupId !== '';
  }

  function getBrainBindingState(demand: Demand) {
    const groupId = getEffectiveTaskGroupId(demand);
    const subTasks = getSubTasksForDemand(demand.task_group_id);
    if (hasTaskGroup(demand.task_group_id) && subTasks.length > 0) {
      return `已绑定 ${subTasks.length} 个影子任务`;
    }
    if (hasTaskGroup(demand.task_group_id)) {
      return '已绑定大脑任务组，等待影子任务导入';
    }
    return `排期将创建 ${groupId}`;
  }

  function displayRepo(repo?: string) {
    if (!repo || repo === '-' || repo === 'unassigned') return 'AI 尚未映射仓库';
    return repo;
  }

  function detectedScheduleBranch(demand?: Demand | null) {
    return hasScheduleValue(demand?.branch) ? (demand?.branch || '') : '';
  }

  function hasScheduleValue(value?: string) {
    const cleaned = (value || '').trim();
    return !!cleaned && cleaned !== '-' && cleaned !== 'unassigned';
  }

  function isDemandFullyScheduled(demand: Demand) {
    return demand.scheduled ?? !!demand.due_date;
  }

  function toNumber(value: unknown) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function formatOneDecimal(value: number) {
    if (!Number.isFinite(value) || value <= 0) return '';
    return Number.isInteger(value) ? String(value) : value.toFixed(1);
  }

  function normalizeScheduleEstimateSource(value?: string): EstimateSource {
    const source = (value || '').trim().toLowerCase();
    if (source === 'ai_deconstruct') return 'ai_deconstruct';
    if (source === 'manual_adjusted' || source === 'manual') return 'manual_adjusted';
    return '';
  }

  function formatDifficultyLabel(value?: string) {
    switch ((value || '').trim().toLowerCase()) {
      case 'high':
        return '高难度';
      case 'medium':
        return '中难度';
      case 'low':
        return '低难度';
      default:
        return '难度待评估';
    }
  }

  function resetScheduleEstimateFromDemand(demand: Demand) {
    schedEstimateHours = toNumber(demand.estimate_hours);
    schedEstimateDays = estimateDaysFromHours(schedEstimateHours, toNumber(demand.estimate_days));
    schedDifficulty = demand.difficulty || '';
    schedEstimateSource = normalizeScheduleEstimateSource(demand.estimate_source) || (hasScheduleEstimate() ? 'manual_adjusted' : '');
    scheduleEstimateError = '';
    scheduleEstimateLoading = false;
  }

  function hasScheduleEstimate() {
    return schedEstimateHours > 0 || schedEstimateDays > 0 || !!schedDifficulty;
  }

  function readPositiveEstimateInput(event: Event): number {
    const value = (event.currentTarget as HTMLInputElement).value;
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
  }

  function estimateDaysFromHours(hours: number, fallbackDays = 0): number {
    if (hours > 0) {
      return Math.round((hours / estimateHoursPerDay) * 10) / 10;
    }
    return fallbackDays > 0 ? Math.round(fallbackDays * 10) / 10 : 0;
  }

  function markScheduleEstimateManual() {
    schedEstimateSource = 'manual_adjusted';
    scheduleEstimateError = '';
  }

  function updateScheduleEstimateHours(event: Event) {
    schedEstimateHours = readPositiveEstimateInput(event);
    schedEstimateDays = estimateDaysFromHours(schedEstimateHours);
    markScheduleEstimateManual();
  }

  function updateScheduleDifficulty(value: string) {
    schedDifficulty = value;
    markScheduleEstimateManual();
  }

  function clearScheduleEstimate() {
    schedEstimateHours = 0;
    schedEstimateDays = 0;
    schedDifficulty = '';
    markScheduleEstimateManual();
  }

  function buildScheduleEstimateText() {
    if (!selectedDemand) return '';
    return [
      `需求ID：${selectedDemand.task_id}`,
      `需求标题：${selectedDemand.title}`,
      `需求描述：${selectedDemand.description || '暂无补充描述'}`,
      `负责人：${selectedDemand.assignee || '未指定'}`,
      `所属项目：${displayRepo(selectedDemand.repo)}`,
      `开发分支：${detectedScheduleBranch(selectedDemand) || '由提交自动检测'}`,
      `计划截止日：${schedDueDate || '尚未设置'}`,
      `任务组：${schedTaskGroupID || getEffectiveTaskGroupId(selectedDemand)}`,
      '请仅围绕该需求给出整体工时、天数、难度、估算依据和主要排期风险。'
    ].join('\n');
  }

  function formatScheduleEstimateError(error: unknown): string {
    const rawMessage = error instanceof Error ? error.message : String(error || '');
    const normalized = rawMessage.toLowerCase();

    if (
      normalized.includes('context deadline exceeded') ||
      normalized.includes('client.timeout') ||
      normalized.includes('timed out') ||
      normalized.includes('timeout')
    ) {
      return 'AI 评估服务响应超时，请稍后重试；你也可以先手动填写工时。';
    }
    if (normalized.includes('too many requests') || normalized.includes('rate limit')) {
      return 'AI 评估请求较多，请稍后重试；你也可以先手动填写工时。';
    }
    if (normalized.includes('未返回有效工时估算')) {
      return 'AI 未返回可用的工时建议，请重试或直接手动填写。';
    }
    return 'AI 工时评估暂时不可用，请稍后重试；你也可以先手动填写工时。';
  }

  function displayProjectOption(project: string) {
    return project === '-' ? '暂不指定项目' : project;
  }

  function addFormOption(options: Set<string>, value?: string) {
    const cleaned = (value || '').trim();
    if (!cleaned || cleaned === '-' || cleaned === '未指派' || cleaned === 'unassigned') return;
    options.add(cleaned);
  }

  function sortedFormOptions(options: Set<string>) {
    return Array.from(options).sort((a, b) => a.localeCompare(b));
  }

  function buildCreateAssigneeOptions(members: Set<string>) {
    const options = new Set<string>();
    (members || new Set()).forEach((name) => {
      if (name && name !== '未指派' && name !== '-' && name !== 'Unassigned') {
        addFormOption(options, name);
      }
    });
    return sortedFormOptions(options);
  }

  function buildCreateProjectOptions(projects: DeliveryProjectOption[]) {
    const options = new Set<string>();
    (projects || []).forEach((project) => {
      if (project?.project_key) addFormOption(options, project.project_key);
    });
    return ['-', ...sortedFormOptions(options)];
  }

  async function toggleProjectDropdown() {
    showProjectDropdown = !showProjectDropdown;
    if (showProjectDropdown && createProjectOptions.length <= 1) {
      await fetchProjectConfigs();
    }
  }

  function updateNewDueDate(value: string) {
    newDueDate = value;
    newDueDateDisplay = formatDateDisplay(value);
  }

  function updateSchedDueDate(value: string) {
    schedDueDate = value;
    schedDueDateDisplay = formatDateDisplay(value);
  }

  function parseDateValue(value: string) {
    if (!value) return null;
    const [year, month, day] = value.split('-').map(Number);
    if (!year || !month || !day) return null;
    return new Date(year, month - 1, day);
  }

  function toDateValue(date: Date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  function openDatePicker(kind: 'new' | 'schedule') {
    const currentValue = kind === 'new' ? newDueDate : schedDueDate;
    activeDatePicker = activeDatePicker === kind ? null : kind;
    datePickerCursor = parseDateValue(currentValue) || new Date();
  }

  function getCalendarDays(value: string, cursor = datePickerCursor) {
    const base = cursor;
    const year = base.getFullYear();
    const month = base.getMonth();
    const first = new Date(year, month, 1);
    const last = new Date(year, month + 1, 0);
    const leading = (first.getDay() + 6) % 7;
    const days: { value: string; label: number; muted: boolean; today: boolean; selected: boolean }[] = [];
    const todayValue = toDateValue(new Date());

    for (let i = leading - 1; i >= 0; i--) {
      const date = new Date(year, month, -i);
      const dateValue = toDateValue(date);
      days.push({ value: dateValue, label: date.getDate(), muted: true, today: dateValue === todayValue, selected: dateValue === value });
    }
    for (let day = 1; day <= last.getDate(); day++) {
      const date = new Date(year, month, day);
      const dateValue = toDateValue(date);
      days.push({ value: dateValue, label: day, muted: false, today: dateValue === todayValue, selected: dateValue === value });
    }
    while (days.length % 7 !== 0) {
      const date = new Date(year, month, days.length - leading + 1);
      const dateValue = toDateValue(date);
      days.push({ value: dateValue, label: date.getDate(), muted: true, today: dateValue === todayValue, selected: dateValue === value });
    }

    return { label: `${base.getFullYear()} ${monthNames[base.getMonth()]}`, days };
  }

  function moveDateMonth(delta: number, event?: MouseEvent) {
    event?.preventDefault();
    event?.stopPropagation();
    const cursor = datePickerCursor || new Date();
    datePickerCursor = new Date(cursor.getFullYear(), cursor.getMonth() + delta, 1);
  }

  function selectDate(kind: 'new' | 'schedule', value: string) {
    if (kind === 'new') updateNewDueDate(value);
    if (kind === 'schedule') updateSchedDueDate(value);
    activeDatePicker = null;
  }

  function clearDate(kind: 'new' | 'schedule') {
    if (kind === 'new') updateNewDueDate('');
    if (kind === 'schedule') updateSchedDueDate('');
    activeDatePicker = null;
  }

  // Columns helper
  $: pendingDemands = demands.filter(d => d.status !== 'done' && d.status !== 'progress' && d.status !== 'review' && !isDemandFullyScheduled(d)).sort((a, b) => compareDemandsByPriority(a, b));
  $: scheduledDemands = demands.filter(d => d.status === 'backlog' && isDemandFullyScheduled(d)).sort((a, b) => compareDemandsByPriority(a, b));
  $: inProgressDemands = demands.filter(d => d.status === 'progress' || d.status === 'review').sort((a, b) => compareDemandsByPriority(a, b));
  $: deliveredDemands = demands.filter(d => d.status === 'done').sort((a, b) => compareDemandsByPriority(a, b));
  $: demandsById = new Map(demands.map((d) => [d.task_id, d]));
  $: createAssigneeOptions = buildCreateAssigneeOptions(coreMembers);
  $: createProjectOptions = buildCreateProjectOptions(deliveryProjects);
  $: scheduleAssigneeOptions = Array.from(coreMembers).sort((a, b) => a.localeCompare(b));
  $: detailSubtasks = detailDemand ? getSubTasksForDemand(detailDemand.task_group_id) : [];
  $: scheduleItemsWithWeights = scheduleItems.map(item => {
    const priority = item.project_priority || getProjectPriority(item.demand_id);
    const weight = computeCompositeWeight(item, priority);
    return {
      ...item,
      project_priority: priority,
      composite_weight: weight
    };
  });

  $: filteredScheduleItems = scheduleItemsWithWeights.filter(item => isCoreMember(item.assignee));
  $: scheduleCriticalItems = filteredScheduleItems
    .filter((item) => {
      const level = normalizeRiskLevel(item.risk_level);
      return level !== 'safe' && level !== 'done';
    })
    .sort((a, b) => compareScheduleItems(a, b))
    .slice(0, 3);
  $: scheduleFocusItem = scheduleCriticalItems[0] || filteredScheduleItems[0];
  $: scheduleLockRate = scheduleSummary.total > 0
    ? Math.round((scheduleSummary.scheduled / scheduleSummary.total) * 100)
    : 0;
  $: scheduleRiskLoad = scheduleSummary.overdue + scheduleSummary.due_soon + scheduleSummary.stale + scheduleSummary.unscheduled;
  $: scheduleCompletionRate = scheduleSummary.total > 0
    ? Math.round((scheduleSummary.done / scheduleSummary.total) * 100)
    : 0;
  $: scheduleAdminMetrics = [
    {
      label: '需求队列',
      value: scheduleSummary.total,
      helper: `当前筛选 ${filteredScheduleItems.length} 条`,
      tone: 'info'
    },
    {
      label: '排期完整率',
      value: `${scheduleLockRate}%`,
      helper: `已排期 ${scheduleSummary.scheduled} / 待排期 ${scheduleSummary.unscheduled}`,
      tone: scheduleLockRate >= 80 ? 'success' : 'warning'
    },
    {
      label: '高风险',
      value: scheduleRiskLoad,
      helper: `逾期 ${scheduleSummary.overdue} / 临期 ${scheduleSummary.due_soon} / 滞后 ${scheduleSummary.stale}`,
      tone: scheduleRiskLoad > 0 ? 'danger' : 'success'
    },
    {
      label: '目标完成',
      value: `${scheduleCompletionRate}%`,
      helper: `已完成 ${scheduleSummary.done} 个需求`,
      tone: 'success'
    }
  ] satisfies AdminMetric[];
  $: scheduleAdminSegments = [
    { label: '待排期', value: scheduleSummary.unscheduled, helper: '缺入口/截止日', tone: scheduleSummary.unscheduled > 0 ? 'warning' : 'neutral' },
    { label: '已排期', value: scheduleSummary.scheduled, helper: '已锁定计划', tone: 'info' },
    { label: '进行中', value: scheduleSummary.in_progress, helper: '开发推进中', tone: 'info' },
    { label: '待验收', value: scheduleSummary.review, helper: '等待确认', tone: scheduleSummary.review > 0 ? 'warning' : 'neutral' },
    { label: '已完成', value: scheduleSummary.done, helper: '完成交付', tone: 'success' }
  ] satisfies AdminMetric[];
  $: if (selectedScheduleDemandId && !filteredScheduleItems.some((item) => item.demand_id === selectedScheduleDemandId)) {
    selectedScheduleDemandId = '';
  }
  $: if (pendingGlobalSearchDemandId && filteredScheduleItems.some((item) => item.demand_id === pendingGlobalSearchDemandId)) {
    selectedScheduleDemandId = pendingGlobalSearchDemandId;
    pendingGlobalSearchDemandId = '';
  }
  $: selectedScheduleItem = filteredScheduleItems.find((item) => item.demand_id === selectedScheduleDemandId) || scheduleFocusItem || null;
  $: scheduleInspectorRecord = buildScheduleInspectorRecord(selectedScheduleItem);
  $: scheduleEditorReadOnly = !selectedScheduleItem
    || ['bug', '缺陷', '故障', 'defect'].includes((selectedScheduleItem.issue_type || '').toLowerCase())
    || !canEditScheduleItem(selectedScheduleItem);
  $: if (activeDemandView === 'schedule' && selectedScheduleItem && selectedScheduleItem.demand_id !== scheduleDraftDemandId) {
    loadScheduleEditor(selectedScheduleItem);
  }
  $: if (activeDemandView === 'schedule' && !selectedScheduleItem && scheduleDraftDemandId) {
    selectedDemand = null;
    scheduleDraftDemandId = '';
    scheduleSaveError = '';
  }
  $: if (!newAssignee && createAssigneeOptions.length > 0) {
    newAssignee = createAssigneeOptions[0];
  }
  $: if (!showProjectDropdown) projectSearchText = '';
  $: if (!showAssigneeDropdown) assigneeSearchText = '';
  $: if (!showScheduleAssigneeDropdown) scheduleAssigneeSearchText = '';
  $: if (!showScheduleProjectDropdown) scheduleProjectSearchText = '';
  $: {
    const shouldLock = showCreateModal || showScheduleModal || showConfirmModal || showDemandDetailsModal || showDeconstructorModal;
    if (shouldLock && !manualModalScrollLocked) {
      lockBodyScroll();
      manualModalScrollLocked = true;
    } else if (!shouldLock && manualModalScrollLocked) {
      unlockBodyScroll();
      manualModalScrollLocked = false;
    }
  }

  function matchesScheduleFilters(item: any, query: string): boolean {
    if (!isCoreMember(item.assignee)) {
      return false;
    }
    if (query) {
      const match = 
        (item._lowerID && item._lowerID.includes(query)) ||
        (item._lowerTitle && item._lowerTitle.includes(query)) ||
        (item._lowerAssignee && item._lowerAssignee.includes(query));
      
      if (!match) return false;
    }

    if (scheduleTypeFilter !== 'all' && item.issue_type !== scheduleTypeFilter) {
      return false;
    }

    if (scheduleAssigneeFilter !== 'all' && item.assignee !== scheduleAssigneeFilter) {
      return false;
    }

    if (scheduleProjectFilter !== 'all') {
      const projKey = item.project_key || getProjectKey(item.demand_id);
      if (projKey !== scheduleProjectFilter) {
        return false;
      }
    }

    if (scheduleRiskFilter === 'attention') {
      return item.risk_level !== 'safe' && item.risk_level !== 'done';
    }
    if (scheduleRiskFilter !== 'all') {
      return item.risk_level === scheduleRiskFilter;
    }
    return true;
  }

  function compareScheduleItems(a: any, b: any): number {
    const weightA = a.composite_weight ?? 0;
    const weightB = b.composite_weight ?? 0;
    if (weightA !== weightB) return weightB - weightA;
    return a.demand_id.localeCompare(b.demand_id);
  }

  function buildScheduleQuery(includeRiskFilter = true): string {
    const params = new URLSearchParams();
    if (scheduleProjectFilter && scheduleProjectFilter !== 'all') {
      params.append('project', scheduleProjectFilter);
    }
    if (scheduleAssigneeFilter && scheduleAssigneeFilter !== 'all') {
      params.append('assignee', scheduleAssigneeFilter);
    }
    if (scheduleSearch && scheduleSearch.trim()) {
      params.append('search', scheduleSearch.trim());
    }
    if (scheduleTypeFilter && scheduleTypeFilter !== 'all') {
      params.append('type', scheduleTypeFilter);
    }
    if (includeRiskFilter && scheduleRiskFilter && scheduleRiskFilter !== 'all') {
      params.append('risk', scheduleRiskFilter);
    }
    const queryStr = params.toString();
    return queryStr ? '?' + queryStr : '';
  }

  function normalizeRiskLevel(value?: string): string {
    const cleaned = (value || '').trim().toLowerCase().replace(/-/g, '_');
    if (cleaned === 'danger') return 'overdue';
    if (cleaned === 'warning') return 'due_soon';
    if (cleaned.includes('overdue') || cleaned.includes('逾期')) return 'overdue';
    if (cleaned.includes('due_soon') || cleaned.includes('soon') || cleaned.includes('临期')) return 'due_soon';
    if (cleaned.includes('stale') || cleaned.includes('滞后')) return 'stale';
    if (cleaned.includes('unscheduled') || cleaned.includes('待排期') || cleaned.includes('缺截止')) return 'unscheduled';
    if (cleaned.includes('done') || cleaned.includes('交付')) return 'done';
    if (cleaned.includes('safe') || cleaned.includes('正常')) return 'safe';
    return cleaned || 'unscheduled';
  }

  function getRiskLevelLabel(value?: string): string {
    const level = normalizeRiskLevel(value);
    return scheduleRiskFilters.find((filter) => filter.value === level)?.label || '风险';
  }

  function getRiskFilterFromLevel(value?: string): ScheduleRiskFilter {
    const level = normalizeRiskLevel(value);
    if (level === 'overdue' || level === 'due_soon' || level === 'stale' || level === 'unscheduled' || level === 'safe' || level === 'done') {
      return level;
    }
    return 'attention';
  }

  function isCalendarRiskLevel(value?: string): boolean {
    const level = normalizeRiskLevel(value);
    return level === 'overdue' || level === 'due_soon' || level === 'stale' || level === 'unscheduled';
  }

  function getRiskSeverityWeight(value?: string): number {
    const level = normalizeRiskLevel(value);
    if (level === 'overdue') return 500;
    if (level === 'due_soon') return 400;
    if (level === 'stale') return 300;
    if (level === 'unscheduled') return 200;
    return 0;
  }

  function sortRiskCalendarEvents(events: ScheduleRiskCalendarEvent[]): ScheduleRiskCalendarEvent[] {
    return [...events].sort((a, b) => {
      const severityDiff = getRiskSeverityWeight(b.risk_level) - getRiskSeverityWeight(a.risk_level);
      if (severityDiff !== 0) return severityDiff;
      const dayDiff = a.days_remaining - b.days_remaining;
      if (Number.isFinite(dayDiff) && dayDiff !== 0) return dayDiff;
      return a.demand_id.localeCompare(b.demand_id);
    });
  }

  function normalizeRiskCalendarEvent(raw: any): ScheduleRiskCalendarEvent {
    const demandID = raw?.demand_id || raw?.task_id || raw?.id || '';
    const riskLevel = normalizeRiskLevel(raw?.risk_type || raw?.risk_level || raw?.risk || raw?.level);
    return {
      id: raw?.id || `${riskLevel}:${demandID}`,
      demand_id: demandID,
      title: raw?.title || raw?.summary || demandID || '未命名需求',
      assignee: raw?.assignee || raw?.owner || '未指派',
      project_key: raw?.project_key || getProjectKey(demandID),
      week_key: raw?.week_key || raw?.weekKey || '',
      risk_type: raw?.risk_type || raw?.type || '',
      risk_level: riskLevel,
      risk_label: raw?.risk_label || raw?.label || getRiskLevelLabel(riskLevel),
      risk_reason: raw?.risk_reason || raw?.reason || '',
      due_date: raw?.due_date || raw?.deadline || '',
      days_remaining: Number.isFinite(Number(raw?.days_remaining)) ? Number(raw.days_remaining) : 999
    };
  }

  function normalizeRiskCalendarCounts(raw: any, events: ScheduleRiskCalendarEvent[]): ScheduleRiskCalendarCounts {
    const counts = createEmptyRiskCalendarCounts();
    const source = raw || {};
    counts.overdue = Number.isFinite(Number(source.overdue)) ? Number(source.overdue) : 0;
    counts.due_soon = Number.isFinite(Number(source.due_soon)) ? Number(source.due_soon) : 0;
    counts.stale = Number.isFinite(Number(source.stale)) ? Number(source.stale) : 0;
    counts.unscheduled = Number.isFinite(Number(source.unscheduled))
      ? Number(source.unscheduled)
      : Number.isFinite(Number(source.missing_schedule))
        ? Number(source.missing_schedule)
        : 0;

    if (counts.overdue + counts.due_soon + counts.stale + counts.unscheduled === 0 && events.length > 0) {
      for (const event of events) {
        const level = normalizeRiskLevel(event.risk_level);
        if (level === 'overdue') counts.overdue += 1;
        if (level === 'due_soon') counts.due_soon += 1;
        if (level === 'stale') counts.stale += 1;
        if (level === 'unscheduled') counts.unscheduled += 1;
      }
    }

    const derivedTotal = counts.overdue + counts.due_soon + counts.stale + counts.unscheduled;
    counts.total = Number.isFinite(Number(source.total)) && Number(source.total) > 0 ? Number(source.total) : derivedTotal;
    return counts;
  }

  function normalizeRiskBucketKey(value: unknown, index: number): ScheduleRiskBucketKey {
    const key = String(value || '').trim().toLowerCase().replace(/-/g, '_');
    if (key.includes('this') || key.includes('current') || key.includes('week_0') || key.includes('本周')) return 'this_week';
    if (key.includes('next') || key.includes('week_1') || key.includes('下周')) return 'next_week';
    if (key.includes('later') || key.includes('future') || key.includes('upcoming') || key.includes('后续')) return 'later';
    return riskCalendarBucketMeta[Math.min(index, riskCalendarBucketMeta.length - 1)].key;
  }

  function normalizeRiskCalendarBucket(raw: any, meta: { key: ScheduleRiskBucketKey; label: string; window_label: string }): ScheduleRiskCalendarBucket {
    const rawEvents = raw?.events || raw?.items || raw?.risks || [];
    const events = Array.isArray(rawEvents)
      ? sortRiskCalendarEvents(rawEvents.map(normalizeRiskCalendarEvent).filter((event) => isCalendarRiskLevel(event.risk_level)))
      : [];
    const rawCounts = raw?.counts || raw?.summary || raw;
    return {
      key: meta.key,
      label: raw?.label || meta.label,
      window_label: raw?.window_label || raw?.range_label || raw?.window || meta.window_label,
      counts: normalizeRiskCalendarCounts(rawCounts, events),
      events,
      event_ids: Array.isArray(raw?.event_ids) ? raw.event_ids : []
    };
  }

  function buildRiskCalendarBucketsFromEvents(events: ScheduleRiskCalendarEvent[]): ScheduleRiskCalendarBucket[] {
    const buckets = createEmptyRiskCalendarBuckets();
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const weekday = (today.getDay() + 6) % 7;
    const thisWeekStart = new Date(today);
    thisWeekStart.setDate(today.getDate() - weekday);
    const nextWeekStart = new Date(thisWeekStart);
    nextWeekStart.setDate(thisWeekStart.getDate() + 7);
    const laterStart = new Date(nextWeekStart);
    laterStart.setDate(nextWeekStart.getDate() + 7);

    for (const event of events) {
      if (!isCalendarRiskLevel(event.risk_level)) continue;
      let bucketKey: ScheduleRiskBucketKey = 'later';
      const riskLevel = normalizeRiskLevel(event.risk_level);
      const dueDate = parseDateValue(event.due_date ? event.due_date.slice(0, 10) : '');
      if (riskLevel === 'overdue') {
        bucketKey = 'this_week';
      } else if (dueDate && dueDate < nextWeekStart) {
        bucketKey = 'this_week';
      } else if (dueDate && dueDate < laterStart) {
        bucketKey = 'next_week';
      }

      const bucket = buckets.find((candidate) => candidate.key === bucketKey);
      if (!bucket) continue;
      const level = normalizeRiskLevel(event.risk_level);
      if (level === 'overdue') bucket.counts.overdue += 1;
      if (level === 'due_soon') bucket.counts.due_soon += 1;
      if (level === 'stale') bucket.counts.stale += 1;
      if (level === 'unscheduled') bucket.counts.unscheduled += 1;
      bucket.counts.total += 1;
      bucket.events.push(event);
    }

    return buckets.map((bucket) => ({
      ...bucket,
      events: sortRiskCalendarEvents(bucket.events)
    }));
  }

  function buildFallbackRiskCalendarBuckets(): ScheduleRiskCalendarBucket[] {
    return buildRiskCalendarBucketsFromEvents(scheduleItems.map(normalizeRiskCalendarEvent));
  }

  function normalizeRiskCalendarResponse(data: any): ScheduleRiskCalendarBucket[] {
    const allEvents = Array.isArray(data?.events)
      ? sortRiskCalendarEvents(data.events.map(normalizeRiskCalendarEvent).filter((event: ScheduleRiskCalendarEvent) => isCalendarRiskLevel(event.risk_level)))
      : [];
    const bucketArray = Array.isArray(data?.buckets)
      ? data.buckets
      : Array.isArray(data?.weeks)
        ? data.weeks
        : Array.isArray(data?.calendar)
          ? data.calendar
          : [];

    if (bucketArray.length > 0) {
      const bucketsByKey = new Map<ScheduleRiskBucketKey, any>();
      bucketArray.forEach((bucket: any, index: number) => {
        bucketsByKey.set(normalizeRiskBucketKey(bucket?.key || bucket?.bucket || bucket?.name || bucket?.label, index), bucket);
      });
      return riskCalendarBucketMeta.map((meta, index) => {
        const raw = bucketsByKey.get(meta.key) || bucketArray[index] || {};
        const bucket = normalizeRiskCalendarBucket(raw, meta);
        if (bucket.events.length === 0 && allEvents.length > 0) {
          const ids = new Set(bucket.event_ids || []);
          bucket.events = sortRiskCalendarEvents(allEvents.filter((event) => {
            if (ids.size > 0 && event.id && ids.has(event.id)) return true;
            return raw?.key && event.week_key === raw.key;
          })).slice(0, 4);
        }
        return bucket;
      });
    }

    const directBuckets = riskCalendarBucketMeta.map((meta) => data?.[meta.key]);
    if (directBuckets.some(Boolean)) {
      return riskCalendarBucketMeta.map((meta, index) => normalizeRiskCalendarBucket(directBuckets[index] || {}, meta));
    }

    const rawEvents = data?.events || data?.items || data?.risks || [];
    if (Array.isArray(rawEvents)) {
      return buildRiskCalendarBucketsFromEvents(rawEvents.map(normalizeRiskCalendarEvent));
    }

    return createEmptyRiskCalendarBuckets();
  }

  function getBucketRiskCount(bucket: ScheduleRiskCalendarBucket, risk: ScheduleRiskFilter): number {
    if (risk === 'overdue') return bucket.counts.overdue;
    if (risk === 'due_soon') return bucket.counts.due_soon;
    if (risk === 'stale') return bucket.counts.stale;
    if (risk === 'unscheduled') return bucket.counts.unscheduled;
    return bucket.counts.total;
  }

  function getRiskCalendarTotal(risk: ScheduleRiskFilter): number {
    return riskCalendarBuckets.reduce((sum, bucket) => sum + getBucketRiskCount(bucket, risk), 0);
  }

  function buildRiskCalendarFocus(): string {
    const overdue = getRiskCalendarTotal('overdue');
    const dueSoon = getRiskCalendarTotal('due_soon');
    const stale = getRiskCalendarTotal('stale');
    const unscheduled = getRiskCalendarTotal('unscheduled');
    if (overdue > 0) return `${overdue} 个逾期项优先处理`;
    if (dueSoon > 0) return `${dueSoon} 个临期项需要锁定交付口径`;
    if (stale > 0) return `${stale} 个停滞项需要补推进证据`;
    if (unscheduled > 0) return `${unscheduled} 个需求等待排期`;
    return '当前排期风险平稳';
  }

  function selectScheduleRiskFromCalendar(risk: ScheduleRiskFilter) {
    scheduleRiskFilter = risk;
    showRiskDropdown = false;
  }

  function formatRiskEventMeta(event: ScheduleRiskCalendarEvent): string {
    const dueText = event.due_date ? formatScheduleDate(event.due_date) : '未排期';
    return `${event.assignee || '未指派'} · ${dueText}`;
  }

  $: riskCalendarHasAny = riskCalendarBuckets.some((bucket) => bucket.counts.total > 0 || bucket.events.length > 0);
  $: riskCalendarTotalCount = riskCalendarBuckets.reduce((sum, bucket) => sum + bucket.counts.total, 0);
  $: riskCalendarFocus = buildRiskCalendarFocus();

  async function fetchRiskCalendar() {
    const requestSeq = ++riskCalendarRequestSeq;
    riskCalendarLoading = true;
    riskCalendarErrorMsg = '';
    riskCalendarUsingFallback = false;
    const token = localStorage.getItem('jwt_token');

    try {
      const res = await fetch('/api/schedule/risk-calendar' + buildScheduleQuery(false), {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) {
        throw new Error('风险日历接口暂不可用');
      }
      const data = await res.json();
      if (requestSeq !== riskCalendarRequestSeq) return;
      riskCalendarBuckets = normalizeRiskCalendarResponse(data);
      riskCalendarGeneratedAt = data?.generated_at || scheduleGeneratedAt || '';
    } catch (err: any) {
      if (requestSeq !== riskCalendarRequestSeq) return;
      riskCalendarErrorMsg = err.message || '风险日历接口暂不可用';
      riskCalendarUsingFallback = true;
      riskCalendarGeneratedAt = scheduleGeneratedAt;
      riskCalendarBuckets = buildFallbackRiskCalendarBuckets();
    } finally {
      if (requestSeq === riskCalendarRequestSeq) {
        riskCalendarLoading = false;
      }
    }
  }


  async function fetchSchedule() {
    scheduleLoading = true;
    scheduleErrorMsg = '';
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/schedule' + buildScheduleQuery(true), {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) {
        throw new Error('获取排期表失败');
      }
      const data: ScheduleResponse = await res.json();
      const rawItems = data.items || [];
      scheduleItems = rawItems.map((item: any) => ({
        ...item,
        issue_type: item.issue_type || 'demand',
        _lowerID: (item.demand_id || '').toLowerCase(),
        _lowerTitle: (item.title || '').toLowerCase(),
        _lowerDesc: (item.description || '').toLowerCase(),
        _lowerAssignee: (item.assignee || '').toLowerCase(),
        _lowerDept: (item.department || '').toLowerCase(),
        _lowerRepo: (item.repo || '').toLowerCase(),
        _lowerBranch: (item.branch || '').toLowerCase(),
        _lowerGroupId: (item.task_group_id || '').toLowerCase(),
        _lowerRiskLabel: (item.risk_label || '').toLowerCase(),
        _lowerIssueType: (item.issue_type || '').toLowerCase(),
      }));
      scheduleSummary = data.summary || createEmptyScheduleSummary();
      scheduleGeneratedAt = data.generated_at || '';
      fetchRiskCalendar();
    } catch (err: any) {
      scheduleErrorMsg = err.message || '获取排期表失败';
    } finally {
      scheduleLoading = false;
    }
  }

  async function refreshDemandWorkspace() {
    await fetchDemands();
    if (activeDemandView === 'schedule') {
      await fetchSchedule();
    }
  }

  async function fetchDemands() {
    const requestSeq = ++flowRequestSeq;
    loading = true;
    errorMsg = '';
    const token = localStorage.getItem('jwt_token');
    try {
      const [scheduleResponse, taskResponse] = await Promise.all([
        fetch('/api/schedule?risk=all', {
          cache: 'no-store',
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/tasks', {
          cache: 'no-store',
          headers: { 'Authorization': `Bearer ${token}` }
        })
      ]);
      if (!scheduleResponse.ok) throw new Error('获取流转事实失败');
      const scheduleData: ScheduleResponse = await scheduleResponse.json();
      const legacyTasks: Demand[] = taskResponse.ok ? await taskResponse.json() : [];
      if (requestSeq !== flowRequestSeq) return;
      const legacyByID = new Map<string, Demand>(legacyTasks.map((task) => [task.task_id, task]));
      demands = (scheduleData.items || []).map((item) => {
        const legacy = legacyByID.get(item.demand_id);
        return {
          task_id: item.demand_id,
          title: item.title,
          description: item.description,
          repo: item.repo,
          assignee: item.assignee,
          creator: legacy?.creator,
          creator_dept: legacy?.creator_dept || item.department,
          branch: item.branch,
          last_commit: legacy?.last_commit || '',
          status: item.status,
          issue_type: item.issue_type || 'demand',
          task_created_at: item.created_at,
          last_update: item.last_update,
          completed_at: item.completed_at,
          due_date: item.due_date,
          task_group_id: item.task_group_id,
          estimate_days: item.estimate_days,
          estimate_hours: item.estimate_hours,
          difficulty: item.difficulty,
          estimate_source: legacy?.estimate_source,
          scheduled: item.scheduled
        };
      });
      const workItemTypes = new Set(['demand', 'requirement', 'story', 'bug', 'defect', '缺陷', '故障']);
      allSubTasks = (legacyTasks || []).filter((task: any) =>
        task.status !== 'archived' && !workItemTypes.has((task.issue_type || '').toLowerCase().trim())
      );
    } catch (err: any) {
      if (requestSeq !== flowRequestSeq) return;
      errorMsg = err.message || '获取需求失败';
    } finally {
      if (requestSeq === flowRequestSeq) loading = false;
    }
  }

  async function fetchUsers() {
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/users', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        users = await res.json();
      }
    } catch (e) {
      console.error('Failed to fetch users:', e);
    }
  }

  async function fetchDemandOptions() {
    try {
      const directory = await fetchDeliveryDirectory();
      demandOptionAssignees = directory.assignees.map((option) => option.value);
      deliveryProjects = directory.projects;
      coreMembers = new Set(demandOptionAssignees);
      coreMemberAliases = new Set(
        directory.assignees.flatMap((option) => [option.value, ...(option.aliases || [])])
          .map((value) => value.trim().toLocaleLowerCase('zh-CN'))
          .filter(Boolean)
      );
      deliveryDirectoryReady = true;
      demandOptionAssigneesLoaded = true;
    } catch (e) {
      console.error('Failed to fetch shared delivery directory:', e);
    }
  }

  async function fetchJiraConfig() {
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

  function openCreateDemandModal() {
    showCreateModal = true;
    showAssigneeDropdown = false;
    showProjectDropdown = false;
    activeDatePicker = null;
    fetchDemandOptions();
    fetchProjectConfigs();
  }

  function closeCreateDemandModal() {
    if (deconstructorPresentation === 'companion' && deconstructorHost === 'create') {
      closeDeconstructorWorkspace();
    }
    showCreateModal = false;
    showAssigneeDropdown = false;
    showProjectDropdown = false;
    activeDatePicker = null;
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      closeCreateDemandModal();
    }
  }

  async function handleCreateDemand() {
    if (!newTitle.trim() || !newAssignee) {
      alert('需求标题与指派负责人不能为空');
      return;
    }

    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/demands', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          title: newTitle,
          description: newDescription,
          assignee: newAssignee,
          due_date: newDueDate,
          repo: newRepo === '-' ? '' : newRepo,
          creator_dept: normalizeDepartment(currentUserDepartment)
        })
      });

      if (res.ok) {
        closeCreateDemandModal();
        newTitle = '';
        newDescription = '';
        newRepo = '-';
        updateNewDueDate('');
        await refreshDemandWorkspace();
      } else {
        const err = await res.json();
        alert(`创建需求失败: ${err.message || res.statusText}`);
      }
    } catch (e) {
      console.error('Failed to create demand:', e);
      alert('网络连接错误，创建需求失败');
    }
  }

  function openScheduleModal(item: any) {
    const task_id = item.task_id || item.demand_id || '';
    const demand_id = item.demand_id || item.task_id || '';
    const converted: Demand = {
      ...item,
      task_id,
      demand_id,
    };
    selectedDemand = converted;
    schedAssignee = converted.assignee || '';
    schedAssigneeSearchText = '';
    showScheduleModalAssigneeDropdown = false;
    updateSchedDueDate(converted.due_date ? converted.due_date.slice(0, 10) : '');
    schedTaskGroupID = getEffectiveTaskGroupId(converted);
    resetScheduleEstimateFromDemand(converted);
    activeDatePicker = null;
    showScheduleModal = true;
  }

  function loadScheduleEditor(item: ScheduleItem) {
    const demand = demandsById.get(item.demand_id);
    if (!demand) {
      selectedDemand = null;
      scheduleDraftDemandId = '';
      scheduleSaveError = '当前排期记录没有匹配到可编辑的需求数据。';
      return;
    }

    selectedDemand = {
      ...demand,
      task_id: item.demand_id,
      assignee: item.assignee || demand.assignee,
      branch: item.branch || demand.branch,
      due_date: item.due_date || demand.due_date,
      task_group_id: item.task_group_id || demand.task_group_id,
      estimate_hours: item.estimate_hours || demand.estimate_hours,
      estimate_days: item.estimate_days || demand.estimate_days,
      difficulty: item.difficulty || demand.difficulty
    };
    scheduleDraftDemandId = item.demand_id;
    scheduleSaveError = '';
    schedAssignee = selectedDemand.assignee || '';
    schedAssigneeSearchText = '';
    showScheduleModalAssigneeDropdown = false;
    updateSchedDueDate(selectedDemand.due_date ? selectedDemand.due_date.slice(0, 10) : '');
    schedTaskGroupID = getEffectiveTaskGroupId(selectedDemand);
    resetScheduleEstimateFromDemand(selectedDemand);
    activeDatePicker = null;
  }

  function selectScheduleItem(item: ScheduleItem, mode: 'schedule' | 'telemetry' = 'schedule') {
    selectedScheduleDemandId = item.demand_id;
    scheduleInspectorMode = mode;
    if (scheduleDraftDemandId !== item.demand_id) loadScheduleEditor(item);
  }

  function handleScheduleInspectorTabKey(event: KeyboardEvent) {
    const tabs: Array<'schedule' | 'telemetry'> = ['schedule', 'telemetry'];
    const tablist = (event.currentTarget as HTMLElement | null)?.closest('[role="tablist"]');
    let nextIndex = tabs.indexOf(scheduleInspectorMode);
    if (event.key === 'ArrowRight') nextIndex = (nextIndex + 1) % tabs.length;
    else if (event.key === 'ArrowLeft') nextIndex = (nextIndex - 1 + tabs.length) % tabs.length;
    else if (event.key === 'Home') nextIndex = 0;
    else if (event.key === 'End') nextIndex = tabs.length - 1;
    else return;

    event.preventDefault();
    scheduleInspectorMode = tabs[nextIndex];
    requestAnimationFrame(() => {
      tablist?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[nextIndex]?.focus();
    });
  }

  function closeScheduleModal() {
    if (deconstructorPresentation === 'companion' && deconstructorHost === 'schedule') {
      closeDeconstructorWorkspace();
    }
    showScheduleModal = false;
    selectedDemand = null;
    schedAssignee = '';
    schedAssigneeSearchText = '';
    showScheduleModalAssigneeDropdown = false;
    activeDatePicker = null;
    scheduleEstimateLoading = false;
  }

  function handleScheduleModalClick(e: Event) {
    const target = e.target as HTMLElement;
    if (!target.closest('#sched-assignee-container')) {
      showScheduleModalAssigneeDropdown = false;
    }
  }

  async function openDemandDetails(demand: Demand) {
    detailReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    detailDemand = demand;
    showDemandDetailsModal = true;
    await tick();
    detailCloseButton?.focus();
  }

  function handleDemandCardKeydown(event: KeyboardEvent, demand: Demand) {
    if (event.target !== event.currentTarget) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openDemandDetails(demand);
    }
  }

  function closeDemandDetails(restoreFocus = true) {
    if (deconstructorPresentation === 'companion' && deconstructorHost === 'details') {
      closeDeconstructorWorkspace(false);
    }
    showDemandDetailsModal = false;
    detailDemand = null;
    const focusTarget = restoreFocus ? detailReturnFocus : null;
    detailReturnFocus = null;
    if (focusTarget) {
      void tick().then(() => focusTarget.focus());
    }
  }

  function handleDemandDetailsBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) closeDemandDetails();
  }

  function handleDemandDetailsKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      if (showDeconstructorModal && deconstructorPresentation === 'companion' && deconstructorHost === 'details') {
        closeDeconstructorWorkspace();
      } else {
        closeDemandDetails();
      }
      return;
    }

    if (event.key !== 'Tab' || !detailDrawerEl || (showDeconstructorModal && deconstructorHost === 'details')) return;
    const focusable = Array.from(detailDrawerEl.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [contenteditable="true"], [tabindex]:not([tabindex="-1"])'
    )).filter((element) => element.offsetParent !== null);
    if (focusable.length === 0) {
      event.preventDefault();
      detailDrawerEl.focus();
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  async function openDeconstructorWorkspace(
    item?: Partial<Demand> | ScheduleItem | null,
    contextLabel = '需求解构',
    presentation: 'modal' | 'companion' = 'modal',
    host: 'none' | 'create' | 'schedule' | 'details' = 'none'
  ) {
    deconstructorReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const title = item?.title || newTitle.trim();
    const description = item?.description || newDescription.trim();
    const demandRef = item as (Partial<Demand> & Partial<ScheduleItem>) | null | undefined;
    deconstructorTitle = title;
    deconstructorDescription = description;
    deconstructorDemandId = demandRef?.task_id || demandRef?.demand_id || '';
    deconstructorContextLabel = contextLabel;
    deconstructorPresentation = presentation;
    deconstructorHost = presentation === 'companion' ? host : 'none';
    showDeconstructorModal = true;
    await tick();
    deconstructorCloseButton?.focus();
  }

  function closeDeconstructorWorkspace(restoreFocus = true) {
    showDeconstructorModal = false;
    deconstructorPresentation = 'modal';
    deconstructorHost = 'none';
    const focusTarget = restoreFocus ? deconstructorReturnFocus : null;
    deconstructorReturnFocus = null;
    if (focusTarget) {
      void tick().then(() => focusTarget.focus());
    }
  }

  function handleDeconstructorBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) closeDeconstructorWorkspace();
  }

  function handleDeconstructorKeydown(event: KeyboardEvent) {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    event.stopPropagation();
    closeDeconstructorWorkspace();
  }

  function getJiraIssueUrl(taskId: string) {
    if (!jiraBaseUrl || !taskId || taskId.startsWith('DEMAND-')) return '';
    return `${jiraBaseUrl}/browse/${taskId}`;
  }

  async function handleEstimateScheduleEffort() {
    if (!selectedDemand) return;
    scheduleEstimateLoading = true;
    scheduleEstimateError = '';
    const token = localStorage.getItem('jwt_token');

    try {
      const res = await fetch('/api/deconstruct', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/x-ndjson',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ text: buildScheduleEstimateText() })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || 'AI 工时评估失败');
      }

      const data = await readDeconstructStream<DeconstructEstimateResponse>(res);
      const analysis = data.analysis || {};
      let hours = toNumber(analysis.overall_estimated_hours);
      const days = toNumber(analysis.overall_estimated_days);

      if (hours <= 0 && days <= 0) {
        throw new Error('AI 未返回有效工时估算');
      }
      if (hours <= 0 && days > 0) {
        hours = Math.round(days * estimateHoursPerDay * 10) / 10;
      }

      schedEstimateHours = hours;
      schedEstimateDays = estimateDaysFromHours(hours, days);
      schedDifficulty = analysis.overall_difficulty || schedDifficulty;
      schedEstimateSource = 'ai_deconstruct';
    } catch (err: any) {
      console.error('Failed to estimate schedule effort:', err);
      scheduleEstimateError = formatScheduleEstimateError(err);
    } finally {
      scheduleEstimateLoading = false;
    }
  }

  async function handleSaveSchedule(presentation: 'modal' | 'inline' = 'modal') {
    if (!selectedDemand || scheduleSaveLoading) return;

    const token = localStorage.getItem('jwt_token');
    const scheduleTaskId = selectedDemand.task_id;
    scheduleSaveLoading = true;
    scheduleSaveError = '';
    try {
      // 统一通过 /api/tasks/schedule 保存排期及负责人变更
      const res = await fetch('/api/tasks/schedule', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          task_id: selectedDemand.task_id,
          branch: detectedScheduleBranch(selectedDemand),
          due_date: schedDueDate,
          status: 'backlog', // Scheduled demands go to backlog in kanban
          task_group_id: schedTaskGroupID,
          estimate_hours: schedEstimateHours,
          estimate_days: schedEstimateDays,
          difficulty: schedDifficulty,
          estimate_source: schedEstimateSource,
          assignee: schedAssignee
        })
      });

      if (res.ok) {
        if (presentation === 'modal') closeScheduleModal();
        await refreshDemandWorkspace();
        showToast('排期设置已保存。', {
          type: 'success',
          title: scheduleTaskId
        });
      } else {
        const errText = await res.text();
        scheduleSaveError = errText || '服务未接受本次排期设置。';
        showToast(`排期保存失败：${scheduleSaveError}`, {
          type: 'error',
          title: scheduleTaskId
        });
      }
    } catch (e: any) {
      console.error('Failed to save schedule:', e);
      scheduleSaveError = e.message || '网络连接错误，保存排期失败';
      showToast(scheduleSaveError, {
        type: 'error',
        title: scheduleTaskId
      });
    } finally {
      scheduleSaveLoading = false;
    }
  }

  function isAssignee(demand: Demand): boolean {
    const name = currentUserName.toLowerCase();
    const email = currentUserEmail.toLowerCase();
    const assignee = demand.assignee.toLowerCase();
    return assignee === name || assignee === email || email.startsWith(assignee);
  }

  function getDueStatus(dateStr?: string): { text: string; className: string } {
    if (!dateStr) return { text: '未排期', className: 'text-muted' };
    const due = new Date(dateStr);
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    due.setHours(0, 0, 0, 0);

    const diffTime = due.getTime() - today.getTime();
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

    if (diffDays < 0) {
      return { text: `逾期 ${Math.abs(diffDays)} 天`, className: 'due-overdue' };
    } else if (diffDays <= 3) {
      return { text: `${diffDays} 天后截止`, className: 'due-critical' };
    } else {
      return { text: `${due.toLocaleDateString()} 截止`, className: 'due-safe' };
    }
  }

  function getScheduleProgress(item: ScheduleItem): number {
    if (item.subtask_total <= 0) return item.status === 'done' ? 100 : 0;
    return Math.round((item.subtask_done / item.subtask_total) * 100);
  }

  function formatScheduleDue(item: ScheduleItem): string {
    if (item.status === 'done') {
      return item.completed_at ? `完成于 ${formatDateDisplay(item.completed_at.slice(0, 10))}` : '已交付';
    }
    if (!item.due_date) return '未设置截止日';
    if (item.days_remaining < 0) return `逾期 ${Math.abs(item.days_remaining)} 天`;
    if (item.days_remaining === 0) return '今天截止';
    return `${item.days_remaining} 天后截止`;
  }

  function formatScheduleRiskPill(item: ScheduleItem, label: string): string {
    if (normalizeRiskLevel(item.risk_level) === 'overdue' && item.days_remaining < 0) {
      return `${label} · ${Math.abs(item.days_remaining)}天`;
    }
    return label;
  }

  function formatScheduleDate(value: string): string {
    if (!value) return '';
    return formatDateDisplay(value.slice(0, 10));
  }

  function isScheduleItemScheduled(item: ScheduleItem): boolean {
    return item.scheduled || (hasScheduleValue(item.branch) && !!item.due_date);
  }

  function getScheduleStatusLabel(item: ScheduleItem): string {
    switch (item.status) {
      case 'backlog':
        if (isScheduleItemScheduled(item)) return '已排期';
        if (hasScheduleValue(item.branch) && !item.due_date) return '缺截止日';
        if (!hasScheduleValue(item.branch) && item.due_date) return '缺开发入口';
        return '待排期';
      case 'progress':
        return '开发中';
      case 'review':
        return '评审中';
      case 'done':
        return '已交付';
      default:
        return item.status || '未知';
    }
  }

  function getScheduleStatusClass(item: ScheduleItem): string {
    if (item.status === 'progress' || item.status === 'review' || item.status === 'done') return item.status;
    if (isScheduleItemScheduled(item)) return 'scheduled';
    if (hasScheduleValue(item.branch) || item.due_date) return 'partial';
    return 'unscheduled';
  }

  function formatScheduleEffort(item: ScheduleItem): string {
    if (item.estimate_hours > 0) return `${formatOneDecimal(item.estimate_hours)} 小时`;
    if (item.estimate_days > 0) return `${formatOneDecimal(item.estimate_days)} 天`;
    return '待评估';
  }

  function hasDeliveryEvidence(item: ScheduleItem): boolean {
    return hasScheduleValue(item.branch) || hasScheduleValue(item.repo) || !!item.mr_url;
  }

  function canEditScheduleItem(item: ScheduleItem): boolean {
    const demand = demandsById.get(item.demand_id);
    return !!demand && (isAssignee(demand) || hasPermission('demands:write'));
  }

  function handleDocumentClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    
    if (!target.closest('#demand-assignee-container')) {
      showAssigneeDropdown = false;
    }
    if (!target.closest('#demand-project-container')) {
      showProjectDropdown = false;
    }
    if (!target.closest('#sched-assignee-container')) {
      showScheduleModalAssigneeDropdown = false;
    }
    if (!target.closest('.schedule-assignee-menu')) {
      showScheduleAssigneeDropdown = false;
    }
    if (!target.closest('.schedule-project-menu')) {
      showScheduleProjectDropdown = false;
    }
    if (!target.closest('.schedule-dropdown-menu')) {
      showRiskDropdown = false;
      showTypeDropdown = false;
      showSortDropdown = false;
    }
    if (!target.closest('.date-input-shell')) {
      activeDatePicker = null;
    }
  }

  function handleGlobalSearchSelection(event: Event) {
    const detail = (event as CustomEvent<{ id?: string; route?: string }>).detail;
    if (detail?.route !== 'schedule' || !detail.id) return;
    scheduleProjectFilter = 'all';
    scheduleAssigneeFilter = 'all';
    scheduleTypeFilter = 'all';
    scheduleRiskFilter = 'all';
    scheduleSearchInput = detail.id;
    scheduleSearch = detail.id;
    pendingGlobalSearchDemandId = detail.id;
  }

  onMount(() => {
    isMounted = true;
    fetchDemands();
    fetchUsers();
    fetchDemandOptions();
    fetchJiraConfig();
    fetchProjectConfigs();
    fetchTelemetryScores();
    document.addEventListener('click', handleDocumentClick);
    window.addEventListener('well-ambient:global-search-select', handleGlobalSearchSelection);

    const handleConfigUpdated = () => {
      void fetchDemandOptions();
    };
    const handleProjectPreferencesUpdated = () => {
      scheduleProjectFilter = 'all';
      void refreshDemandWorkspace();
    };
    window.addEventListener('config-updated', handleConfigUpdated);
    window.addEventListener('project-preferences-updated', handleProjectPreferencesUpdated);

    // Poll updates every 15 seconds
    const interval = setInterval(() => {
      fetchDemands();
      if (activeDemandView === 'schedule') {
        fetchSchedule();
      }
    }, 15000);
    return () => {
      clearInterval(interval);
      document.removeEventListener('click', handleDocumentClick);
      window.removeEventListener('well-ambient:global-search-select', handleGlobalSearchSelection);
      window.removeEventListener('config-updated', handleConfigUpdated);
      window.removeEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
    };
  });

  onDestroy(() => {
    if (manualModalScrollLocked) {
      unlockBodyScroll();
      manualModalScrollLocked = false;
    }
  });

  $: {
    // 显式引用依赖项，让 Svelte 编译器精确捕捉排期表过滤状态的变动
    const _view = activeDemandView;
    const _proj = scheduleProjectFilter;
    const _ass = scheduleAssigneeFilter;
    const _search = scheduleSearch;
    const _type = scheduleTypeFilter;
    const _risk = scheduleRiskFilter;
    const _mounted = isMounted;

    if (_mounted && _view === 'schedule') {
      fetchSchedule();
    }
  }
</script>

<div
  class="demand-dashboard font-sans"
  class:flow-dashboard={activeDemandView === 'board'}
  class:schedule-dashboard={activeDemandView === 'schedule'}
>
  {#if loading && demands.length === 0 && activeDemandView === 'board'}
    <div class="state-msg">加载需求大盘中...</div>
  {:else if errorMsg && activeDemandView === 'board'}
    <div class="state-msg error-msg font-mono">❌ {errorMsg}</div>
  {:else if activeDemandView === 'schedule'}
    <div class="schedule-workbench wa-grain" bind:this={scheduleWorkbenchEl}>
      <div class="schedule-signal-strip wa-admin-card" aria-label="排期治理指标与状态">
        {#each scheduleAdminMetrics as metric}
          <div class="schedule-signal-cell is-primary {ADMIN_TONE_CLASS[metric.tone || 'neutral']}">
            <span>{metric.label}</span>
            <strong class="font-mono">{metric.value}</strong>
            <em>{metric.helper}</em>
          </div>
        {/each}
        {#each scheduleAdminSegments as segment}
          <div class="schedule-signal-cell is-segment {ADMIN_TONE_CLASS[segment.tone || 'neutral']}">
            <span>{segment.label}</span>
            <strong class="font-mono">{segment.value}</strong>
            <em>{segment.helper}</em>
          </div>
        {/each}
      </div>

      <div class="schedule-main-grid">
        <section class="schedule-table-stack wa-admin-section" aria-label="需求排期表">
          <div class="schedule-control-panel wa-admin-card wa-admin-toolbar">
            <div class="schedule-search-shell">
              <span class="search-mark"></span>
              <input
                type="text"
                placeholder="搜索需求、编号、负责人"
                value={scheduleSearchInput}
                on:input={handleScheduleSearch}
              />
            </div>

            <div class="schedule-dropdown-menu custom-dropdown-container">
              <div class="combobox-trigger-wrapper">
                <button
                  type="button"
                  class="dropdown-trigger-input schedule-trigger-override dropdown-trigger-btn"
                  on:click|stopPropagation={() => showRiskDropdown = !showRiskDropdown}
                >
                  {scheduleRiskFilters.find(f => f.value === scheduleRiskFilter)?.label || '风险筛选'}
                </button>
                <span class="arrow-icon {showRiskDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showRiskDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each scheduleRiskFilters as filter}
                    <button
                      type="button"
                      class="dropdown-option-item {scheduleRiskFilter === filter.value ? 'selected' : ''}"
                      on:click={() => {
                        scheduleRiskFilter = filter.value;
                        showRiskDropdown = false;
                      }}
                    >
                      {filter.label}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="schedule-dropdown-menu custom-dropdown-container">
              <div class="combobox-trigger-wrapper">
                <button
                  type="button"
                  class="dropdown-trigger-input schedule-trigger-override dropdown-trigger-btn"
                  on:click|stopPropagation={() => showTypeDropdown = !showTypeDropdown}
                >
                  {scheduleTypeFilter === 'all' ? '全部类型' : (scheduleTypeFilter === 'demand' ? '仅需求' : '仅缺陷')}
                </button>
                <span class="arrow-icon {showTypeDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showTypeDropdown}
                <div class="dropdown-options-list glass-panel">
                  <button
                    type="button"
                    class="dropdown-option-item {scheduleTypeFilter === 'all' ? 'selected' : ''}"
                    on:click={() => {
                      scheduleTypeFilter = 'all';
                      showTypeDropdown = false;
                    }}
                  >
                    全部类型
                  </button>
                  <button
                    type="button"
                    class="dropdown-option-item {scheduleTypeFilter === 'demand' ? 'selected' : ''}"
                    on:click={() => {
                      scheduleTypeFilter = 'demand';
                      showTypeDropdown = false;
                    }}
                  >
                    仅需求
                  </button>
                  <button
                    type="button"
                    class="dropdown-option-item {scheduleTypeFilter === 'bug' ? 'selected' : ''}"
                    on:click={() => {
                      scheduleTypeFilter = 'bug';
                      showTypeDropdown = false;
                    }}
                  >
                    仅缺陷
                  </button>
                </div>
              {/if}
            </div>

            <div class="schedule-assignee-menu custom-dropdown-container">
              <div class="combobox-trigger-wrapper">
                <input
                  type="text"
                  class="dropdown-trigger-input schedule-trigger-override"
                  placeholder={scheduleAssigneeFilter === 'all' ? '全部负责人' : scheduleAssigneeFilter}
                  bind:value={scheduleAssigneeSearchText}
                  on:focus|stopPropagation={() => showScheduleAssigneeDropdown = true}
                />
                <span class="arrow-icon {showScheduleAssigneeDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showScheduleAssigneeDropdown}
                <div class="dropdown-options-list glass-panel">
                  <button
                    type="button"
                    class="dropdown-option-item {scheduleAssigneeFilter === 'all' ? 'selected' : ''}"
                    on:click={() => {
                      scheduleAssigneeFilter = 'all';
                      showScheduleAssigneeDropdown = false;
                    }}
                  >
                    全部负责人
                  </button>
                  {#each scheduleAssigneeOptions.filter(name => !scheduleAssigneeSearchText || name.toLowerCase().includes(scheduleAssigneeSearchText.toLowerCase())) as assignee}
                    <button
                      type="button"
                      class="dropdown-option-item {scheduleAssigneeFilter === assignee ? 'selected' : ''}"
                      on:click={() => {
                        scheduleAssigneeFilter = assignee;
                        showScheduleAssigneeDropdown = false;
                      }}
                    >
                      {assignee}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="schedule-project-menu custom-dropdown-container">
              <div class="combobox-trigger-wrapper">
                <input
                  type="text"
                  class="dropdown-trigger-input schedule-trigger-override"
                  placeholder={scheduleProjectFilter === 'all' ? '全部项目' : (getProjectName(scheduleProjectFilter) || scheduleProjectFilter)}
                  bind:value={scheduleProjectSearchText}
                  on:focus|stopPropagation={() => showScheduleProjectDropdown = true}
                />
                <span class="arrow-icon {showScheduleProjectDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showScheduleProjectDropdown}
                <div class="dropdown-options-list glass-panel">
                  <button
                    type="button"
                    class="dropdown-option-item {scheduleProjectFilter === 'all' ? 'selected' : ''}"
                    on:click={() => {
                      scheduleProjectFilter = 'all';
                      showScheduleProjectDropdown = false;
                    }}
                  >
                    全部项目
                  </button>
                  {#each deliveryProjects.filter(p => !scheduleProjectSearchText || p.project_name.toLowerCase().includes(scheduleProjectSearchText.toLowerCase()) || p.project_key.toLowerCase().includes(scheduleProjectSearchText.toLowerCase())) as proj}
                    <button
                      type="button"
                      class="dropdown-option-item {scheduleProjectFilter === proj.project_key ? 'selected' : ''}"
                      on:click={() => {
                        scheduleProjectFilter = proj.project_key;
                        showScheduleProjectDropdown = false;
                      }}
                    >
                      {proj.project_name}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="schedule-dropdown-menu custom-dropdown-container">
              <div class="combobox-trigger-wrapper">
                <button
                  type="button"
                  class="dropdown-trigger-input schedule-trigger-override dropdown-trigger-btn"
                  on:click|stopPropagation={() => showSortDropdown = !showSortDropdown}
                >
                  排序: {scheduleSortModes.find(m => m.value === scheduleSortMode)?.label || '默认排序'}
                </button>
                <span class="arrow-icon {showSortDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showSortDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each scheduleSortModes as mode}
                    <button
                      type="button"
                      class="dropdown-option-item {scheduleSortMode === mode.value ? 'selected' : ''}"
                      on:click={() => {
                        scheduleSortMode = mode.value;
                        showSortDropdown = false;
                      }}
                    >
                      {mode.label}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <button class="schedule-refresh-btn wa-admin-action secondary font-mono" class:is-loading={scheduleLoading} on:click={fetchSchedule}>
              刷新
            </button>
            {#if hasPermission('demands:write')}
              <button class="add-demand-btn schedule-add-demand" on:click={openCreateDemandModal}>
                <span class="add-demand-icon" aria-hidden="true">+</span>
                <span>录入新需求</span>
              </button>
            {/if}
          </div>

          {#if scheduleLoading && scheduleItems.length === 0}
            <div class="state-msg">加载排期表中...</div>
          {:else if scheduleErrorMsg}
            <div class="state-msg error-msg font-mono">{scheduleErrorMsg}</div>
          {:else}
            <div class="schedule-table-panel wa-admin-card" bind:this={scheduleTablePanelEl}>
              <div class="schedule-table-head">
                <div>
                  <span class="eyebrow">需求队列</span>
                  <h3>排期治理总表</h3>
                </div>
                <span class="schedule-count font-mono">{filteredScheduleItems.length} / {scheduleItems.length}</span>
              </div>
              <div class="schedule-table-wrapper wa-admin-table-shell" style="--schedule-content-height: {scheduleTableContentHeight}px;" on:scroll={handleScheduleScroll} bind:this={scheduleContainerEl} bind:clientHeight={scheduleContainerHeight}>
                <table class="wa-admin-table schedule-admin-table">
                  <colgroup>
                    {#each scheduleTableColumns as column}
                      <col style="width: {column.width || 'auto'}" />
                    {/each}
                    <col style="width: 7%" />
                  </colgroup>
                  <thead>
                    <tr>
                      {#each scheduleTableColumns as column}
                        <th class:align-center={column.align === 'center'} class:align-right={column.align === 'right'}>{column.label}</th>
                      {/each}
                      <th class="align-center action-col">查看</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#if scheduleTopPadding > 0}
                      <tr class="schedule-spacer-row" aria-hidden="true">
                        <td colspan={scheduleTableColumns.length + 1} style="height: {scheduleTopPadding}px;"></td>
                      </tr>
                    {/if}
                    {#each visibleScheduleRows as row (row.id)}
                      {@const item = row.data}
                      {@const adminRow = row.row}
                      {@const progress = getScheduleProgress(item)}
                      {@const priority = item.project_priority || getProjectPriority(item.demand_id)}
                      <tr
                        class="schedule-admin-row"
                        class:is-selected={selectedScheduleItem?.demand_id === item.demand_id}
                        on:click={() => selectScheduleItem(item)}
                      >
                        <td class="schedule-demand-cell">
                          <div class="schedule-demand-line" title={`${adminRow.title} · ${adminRow.cells.project}`}>
                            {#if getJiraIssueUrl(item.demand_id)}
                              <a class="schedule-id font-mono jira-id-link" href={getJiraIssueUrl(item.demand_id)} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                                {item.demand_id}
                              </a>
                            {:else}
                              <span class="schedule-id font-mono">{item.demand_id}</span>
                            {/if}
                            <strong class="schedule-demand-title">{adminRow.title}</strong>
                            <span class="schedule-project-inline">· {adminRow.cells.project}</span>
                            <span
                              class="schedule-type-dot {getScheduleIssueTypeLabel(item) === '缺陷' ? 'is-bug' : 'is-demand'}"
                              role="img"
                              aria-label={getScheduleIssueTypeLabel(item)}
                              title={getScheduleIssueTypeLabel(item)}
                            ></span>
                          </div>
                        </td>
                        <td class="align-center">
                          {#if priority}
                            <span class="priority-badge wa-admin-pill {ADMIN_TONE_CLASS[getPriorityTone(priority)]}">{priority}</span>
                          {:else}
                            <span class="schedule-row-muted">-</span>
                          {/if}
                        </td>
                        <td>
                          <div class="owner-stack">
                            <strong>{adminRow.owner}</strong>
                          </div>
                        </td>
                        <td>
                          <span class="schedule-risk-pill wa-admin-pill schedule-risk-{normalizeRiskLevel(item.risk_level)} {ADMIN_TONE_CLASS[getScheduleRiskTone(item.risk_level)]}" title={item.risk_reason || ''}>{formatScheduleRiskPill(item, adminRow.risk || getRiskLevelLabel(item.risk_level))}</span>
                        </td>
                        <td>
                          <div class="plan-stack">
                            <strong class="font-mono" title={formatScheduleDue(item)}>{adminRow.dueDate}</strong>
                          </div>
                        </td>
                        <td class="align-center">
                          <span class="status-chip wa-admin-pill status-{getScheduleStatusClass(item)} {ADMIN_TONE_CLASS[adminRow.tone || 'neutral']}">{adminRow.status}</span>
                        </td>
                        <td class="align-center">
                          <div class="subtask-stack">
                            <div class="subtask-meter" title="AI 解构任务进度: {adminRow.cells.subtasks}">
                              <span style="width: {progress}%"></span>
                              <small class="subtask-percentage-text">{adminRow.cells.subtasks}</small>
                            </div>
                          </div>
                        </td>
                        <td class="align-center action-col">
                          <div class="schedule-actions-cell">
                            <button
                              class="schedule-row-action wa-admin-action secondary is-telemetry font-mono"
                              aria-label={`在右侧查看 ${item.demand_id} 的代码轨迹`}
                              on:click|stopPropagation={() => selectScheduleItem(item, 'telemetry')}
                            >轨迹</button>
                          </div>
                        </td>
                      </tr>
                    {/each}
                    {#if scheduleBottomPadding > 0}
                      <tr class="schedule-spacer-row" aria-hidden="true">
                        <td colspan={scheduleTableColumns.length + 1} style="height: {scheduleBottomPadding}px;"></td>
                      </tr>
                    {/if}
                  </tbody>
                </table>
                {#if filteredScheduleItems.length === 0}
                  <div class="schedule-empty-state font-mono">当前筛选下暂无排期数据</div>
                {/if}
              </div>
            </div>
          {/if}
        </section>

        <aside class="schedule-inspector-panel wa-admin-card wa-admin-inspector" aria-label="需求详情与检查项">
          {#if scheduleInspectorRecord && selectedScheduleItem}
            {@const descriptionSection = scheduleInspectorRecord.sections.find((section) => section.title === '需求描述')}
            {@const checklistSection = scheduleInspectorRecord.sections.find((section) => section.title === '验收标准')}
            {@const riskSection = scheduleInspectorRecord.sections.find((section) => section.title === '风险说明')}
            <div class="schedule-inspector-head">
              <div class="inspector-title-row">
                <span class="wa-admin-pill {ADMIN_TONE_CLASS[scheduleInspectorRecord.tone || 'neutral']}">{scheduleInspectorRecord.status}</span>
                {#if getJiraIssueUrl(scheduleInspectorRecord.id)}
                  <a class="schedule-id font-mono jira-id-link" href={getJiraIssueUrl(scheduleInspectorRecord.id)} target="_blank" rel="noopener noreferrer">
                    {scheduleInspectorRecord.id}
                  </a>
                {:else}
                  <span class="schedule-id font-mono">{scheduleInspectorRecord.id}</span>
                {/if}
              </div>
              <h3>{scheduleInspectorRecord.title}</h3>
              <p>{descriptionSection?.body}</p>
            </div>

            <div class="schedule-inspector-tabs" role="tablist" aria-label="右侧需求工作区">
              <button
                id="schedule-inspector-tab-schedule"
                type="button"
                role="tab"
                aria-selected={scheduleInspectorMode === 'schedule'}
                aria-controls="schedule-inspector-panel-schedule"
                tabindex={scheduleInspectorMode === 'schedule' ? 0 : -1}
                class:active={scheduleInspectorMode === 'schedule'}
                on:click={() => scheduleInspectorMode = 'schedule'}
                on:keydown={handleScheduleInspectorTabKey}
              >
                排期设置
              </button>
              <button
                id="schedule-inspector-tab-telemetry"
                type="button"
                role="tab"
                aria-selected={scheduleInspectorMode === 'telemetry'}
                aria-controls="schedule-inspector-panel-telemetry"
                tabindex={scheduleInspectorMode === 'telemetry' ? 0 : -1}
                class:active={scheduleInspectorMode === 'telemetry'}
                on:click={() => scheduleInspectorMode = 'telemetry'}
                on:keydown={handleScheduleInspectorTabKey}
              >
                代码轨迹
              </button>
            </div>

            {#if scheduleInspectorMode === 'telemetry'}
              <div
                id="schedule-inspector-panel-telemetry"
                class="schedule-telemetry-inline"
                role="tabpanel"
                aria-labelledby="schedule-inspector-tab-telemetry"
                tabindex="0"
              >
                <CommitTelemetryPanel
                  taskID={selectedScheduleItem.demand_id}
                  isOpen={true}
                  presentation="inline"
                  onClose={() => scheduleInspectorMode = 'schedule'}
                />
              </div>
            {:else}
              <div
                id="schedule-inspector-panel-schedule"
                class="schedule-editor-body"
                role="tabpanel"
                aria-labelledby="schedule-inspector-tab-schedule"
                tabindex="0"
              >
                <div class="schedule-inspector-facts">
                  {#each scheduleInspectorRecord.facts.slice(1, 6) as fact}
                    <div>
                      <span>{fact.label}</span>
                      <strong>{fact.value}</strong>
                    </div>
                  {/each}
                </div>

                <section class="schedule-editor-section" aria-labelledby="schedule-editor-title">
                  <div class="schedule-editor-heading">
                    <div>
                      <strong id="schedule-editor-title">开发排期</strong>
                      <span>{scheduleEditorReadOnly ? '当前记录只读' : '修改后直接保存到当前需求'}</span>
                    </div>
                    <div class="schedule-editor-progress">
                      <span>AI 解构</span>
                      <strong class="font-mono">{getScheduleProgress(selectedScheduleItem)}%</strong>
                    </div>
                  </div>

                  <div class="schedule-editor-grid">
                    <div class="schedule-editor-field is-wide">
                      <span>负责人</span>
                      <Select
                        id="schedule-inline-assignee"
                        value={schedAssignee}
                        options={createAssigneeOptions.map((assignee) => ({ value: assignee, label: assignee }))}
                        placeholder="选择负责人"
                        searchPlaceholder="搜索负责人"
                        ariaLabel="负责人"
                        compact={true}
                        disabled={scheduleEditorReadOnly}
                        on:change={(event) => {
                          schedAssignee = event.detail;
                          scheduleSaveError = '';
                        }}
                      />
                    </div>

                    <label class="schedule-editor-field" for="schedule-inline-hours">
                      <span>预估工时</span>
                      <input
                        id="schedule-inline-hours"
                        type="number"
                        min="0"
                        step="0.5"
                        inputmode="decimal"
                        value={schedEstimateHours > 0 ? formatOneDecimal(schedEstimateHours) : ''}
                        placeholder="0"
                        disabled={scheduleEditorReadOnly}
                        on:input={updateScheduleEstimateHours}
                      />
                    </label>

                    <div class="schedule-editor-field">
                      <span>难度</span>
                      <Select
                        id="schedule-inline-difficulty"
                        value={schedDifficulty}
                        options={difficultyOptions}
                        placeholder="未设置"
                        ariaLabel="难度"
                        searchable={false}
                        compact={true}
                        disabled={scheduleEditorReadOnly}
                        on:change={(event) => updateScheduleDifficulty(event.detail)}
                      />
                    </div>

                    <label class="schedule-editor-field" for="schedule-inline-due">
                      <span>计划完成日</span>
                      <input
                        id="schedule-inline-due"
                        type="date"
                        value={schedDueDate}
                        disabled={scheduleEditorReadOnly}
                        on:input={(event) => updateSchedDueDate((event.currentTarget as HTMLInputElement).value)}
                      />
                    </label>

                    <div class="schedule-editor-field">
                      <span>AI 解构任务组</span>
                      <output class="schedule-editor-output font-mono" for="schedule-inline-hours">{schedTaskGroupID}</output>
                    </div>
                  </div>

                  {#if scheduleEstimateError || scheduleSaveError}
                    <p class="schedule-editor-error" role="alert">{scheduleSaveError || scheduleEstimateError}</p>
                  {/if}

                  <div class="schedule-editor-actions">
                    <div class="schedule-editor-secondary-actions" aria-label="排期辅助操作">
                      <button
                        type="button"
                        class="wa-admin-action secondary"
                        disabled={scheduleEditorReadOnly || scheduleEstimateLoading}
                        on:click={handleEstimateScheduleEffort}
                      >
                        {scheduleEstimateLoading ? '评估中' : 'AI 评估'}
                      </button>
                      <button type="button" class="wa-admin-action secondary" disabled={scheduleEditorReadOnly} on:click={clearScheduleEstimate}>清空估算</button>
                      <button
                        type="button"
                        class="wa-admin-action secondary ai-workbench-action"
                        on:click={() => openDeconstructorWorkspace(selectedScheduleItem, '排期需求解构')}
                      >
                        AI 解构
                      </button>
                    </div>
                    <button
                      type="button"
                      class="wa-admin-action primary schedule-save-action"
                      disabled={scheduleEditorReadOnly || scheduleSaveLoading}
                      on:click={() => handleSaveSchedule('inline')}
                    >
                      {scheduleSaveLoading ? '保存中' : '保存排期'}
                    </button>
                  </div>
                </section>

                <div class="schedule-editor-context">
                  <section>
                    <div class="inspector-section-head">
                      <strong>验收标准</strong>
                      <span class="font-mono">{selectedScheduleItem.subtask_done}/{selectedScheduleItem.subtask_total || 0}</span>
                    </div>
                    <div class="schedule-check-list">
                      {#each (checklistSection?.items || []).slice(0, 2) as item}
                        <div class="schedule-check-row">
                          <span class="check-dot"></span>
                          <p>{item}</p>
                        </div>
                      {/each}
                    </div>
                  </section>
                  <section>
                    <div class="inspector-section-head">
                      <strong>风险说明</strong>
                      <span class="wa-admin-pill {ADMIN_TONE_CLASS[getScheduleRiskTone(selectedScheduleItem.risk_level)]}">{selectedScheduleItem.risk_label || getRiskLevelLabel(selectedScheduleItem.risk_level)}</span>
                    </div>
                    <p>{riskSection?.body}</p>
                  </section>
                </div>

                <section class="schedule-risk-inline" aria-label="风险日历">
                  <div class="inspector-section-head">
                    <div>
                      <strong>风险日历</strong>
                      <span>{riskCalendarFocus}</span>
                    </div>
                    <button type="button" class="wa-admin-action secondary" on:click={fetchRiskCalendar} disabled={riskCalendarLoading}>
                      {riskCalendarLoading ? '刷新中' : '刷新'}
                    </button>
                  </div>
                  {#if riskCalendarErrorMsg && riskCalendarUsingFallback}
                    <div class="risk-calendar-warning font-mono">{riskCalendarErrorMsg}，已使用当前排期表生成临时视图。</div>
                  {/if}
                  <div class="inspector-risk-toolbar">
                    <button
                      type="button"
                      class:active={scheduleRiskFilter === 'attention'}
                      on:click={() => selectScheduleRiskFromCalendar('attention')}
                    >
                      <span>全部</span>
                      <strong class="font-mono">{riskCalendarTotalCount}</strong>
                    </button>
                    {#each riskCalendarRiskTypes as risk}
                      <button
                        type="button"
                        class="risk-{risk.value}"
                        class:active={scheduleRiskFilter === risk.value}
                        on:click={() => selectScheduleRiskFromCalendar(risk.value)}
                      >
                        <span>{risk.label}</span>
                        <strong class="font-mono">{getRiskCalendarTotal(risk.value)}</strong>
                      </button>
                    {/each}
                  </div>
                </section>
              </div>
            {/if}
          {:else}
            <div class="schedule-inspector-empty">
              <strong>暂无需求详情</strong>
              <p>当前筛选下没有排期记录，可以调整筛选条件或刷新排期数据。</p>
            </div>
          {/if}
        </aside>
      </div>
    </div>
  {:else}
    <!-- Kanban Lanes -->
    <div class="flow-filter-panel wa-admin-card wa-admin-toolbar">
      <span>活跃需求 <strong class="font-mono">{demands.length}</strong></span>
      <div class="flow-filter-actions">
        <button class="wa-admin-action secondary ai-workbench-action" on:click={() => openDeconstructorWorkspace(null, '需求解构')}>需求解构</button>
        {#if hasPermission('demands:write')}
          <button class="add-demand-btn" on:click={openCreateDemandModal}>
            <span class="add-demand-icon" aria-hidden="true">+</span>
            <span>录入新需求</span>
          </button>
        {/if}
      </div>
    </div>
    <div class="demand-kanban-board">

      <!-- 1. Pending Schedule -->
      <div class="kanban-lane">
        <div class="lane-header">
          <span class="lane-indicator bg-orange"></span>
          <h4>待排期 ({pendingDemands.length})</h4>
        </div>
        <div class="lane-cards">
          {#each pendingDemands as item}
            {@const priority = getProjectPriority(item.task_id)}
            <div class="demand-card border-orange-dim" role="button" tabindex="0" on:click={() => openDemandDetails(item)} on:keydown={(event) => handleDemandCardKeydown(event, item)}>
              <div class="card-top">
                {#if getJiraIssueUrl(item.task_id)}
                  <a class="demand-id demand-id-button jira-id-link font-mono" href={getJiraIssueUrl(item.task_id)} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                    #{item.task_id}
                  </a>
                {:else}
                  <button class="demand-id demand-id-button font-mono" on:click|stopPropagation={() => openDemandDetails(item)}>
                    #{item.task_id}
                  </button>
                {/if}
                <div class="card-actions">
                  <button class="icon-action-btn telemetry-action-btn" title="查看代码轨迹" on:click|stopPropagation={() => { activeTelemetryTaskId = item.task_id; isTelemetryDrawerOpen = true; }}>🚀</button>
                  {#if canManageDemand(item)}
                    <button class="icon-action-btn" title="归档需求" on:click|stopPropagation={() => handleArchiveDemand(item.task_id)}>📁</button>
                    <button class="icon-action-btn" title="物理删除" on:click|stopPropagation={() => handleDeleteDemand(item.task_id)}>🗑️</button>
                  {/if}
                  <span class="assignee-badge font-mono">👤 {item.assignee}</span>
                </div>
              </div>
              <h5>{item.title}</h5>
              <div class="creator-meta font-mono">提单人: {item.creator || '系统'} ({item.creator_dept || '无部门'})</div>
              {#if item.description}
                <p class="desc">{item.description}</p>
              {/if}
              
              {#if item.task_group_id && item.task_group_id !== '-' && item.task_group_id !== ''}
                {@const subTasks = getSubTasksForDemand(item.task_group_id)}
                {#if subTasks.length > 0}
                  {@const completedCount = subTasks.filter(t => t.status === 'done').length}
                  {@const percent = Math.round((completedCount / subTasks.length) * 100)}
                  <div class="deconstruct-subtasks-box font-mono" on:click|stopPropagation>
                    <div class="subtask-progress-row" on:click={() => toggleDemandSubtasks(item.task_id)}>
                      <span class="subtask-label">🤖 AI 拆分任务 ({completedCount}/{subTasks.length})</span>
                      <div class="subtask-bar">
                        <div class="subtask-bar-fill" style="width: {percent}%"></div>
                      </div>
                      <span class="expand-arrow">{expandedDemands[item.task_id] ? '▲' : '▼'}</span>
                    </div>
                    {#if expandedDemands[item.task_id]}
                      <div class="subtasks-list-details">
                        {#each subTasks as sub}
                          <div class="subtask-detail-item" title={sub.title}>
                            <span class="status-dot {sub.status}"></span>
                            <span class="sub-repo">[{sub.repo}]</span>
                            <span class="sub-title">{sub.title}</span>
                            <span class="sub-assignee">👤{sub.assignee}</span>
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/if}
              {/if}
              
              <div class="card-bottom">
                {#if priority}
                  <span class="priority-badge p-{priority.toLowerCase()}">{priority}</span>
                {/if}
                <span class="brain-link-badge font-mono">{getBrainBindingState(item)}</span>
                {#if isAssignee(item) || hasPermission('demands:write')}
                  <button class="action-btn schedule-btn" on:click|stopPropagation={() => openScheduleModal(item)}>
                    ⚡ 排期
                  </button>
                {/if}
              </div>
            </div>
          {/each}
          {#if pendingDemands.length === 0}
            <div class="empty-lane font-mono">暂无待排期需求</div>
          {/if}
        </div>
      </div>

      <!-- 2. Scheduled -->
      <div class="kanban-lane">
        <div class="lane-header">
          <span class="lane-indicator bg-blue"></span>
          <h4>已排期 ({scheduledDemands.length})</h4>
        </div>
        <div class="lane-cards">
          {#each scheduledDemands as item}
            {@const dueInfo = getDueStatus(item.due_date)}
            {@const priority = getProjectPriority(item.task_id)}
            <div class="demand-card border-blue-dim" role="button" tabindex="0" on:click={() => openDemandDetails(item)} on:keydown={(event) => handleDemandCardKeydown(event, item)}>
              <div class="card-top">
                {#if getJiraIssueUrl(item.task_id)}
                  <a class="demand-id demand-id-button jira-id-link font-mono" href={getJiraIssueUrl(item.task_id)} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                    #{item.task_id}
                  </a>
                {:else}
                  <button class="demand-id demand-id-button font-mono" on:click|stopPropagation={() => openDemandDetails(item)}>
                    #{item.task_id}
                  </button>
                {/if}
                <div class="card-actions">
                  <button class="icon-action-btn telemetry-action-btn" title="查看代码轨迹" on:click|stopPropagation={() => { activeTelemetryTaskId = item.task_id; isTelemetryDrawerOpen = true; }}>🚀</button>
                  {#if canManageDemand(item)}
                    <button class="icon-action-btn" title="归档需求" on:click|stopPropagation={() => handleArchiveDemand(item.task_id)}>📁</button>
                    <button class="icon-action-btn" title="物理删除" on:click|stopPropagation={() => handleDeleteDemand(item.task_id)}>🗑️</button>
                  {/if}
                  <span class="assignee-badge font-mono">👤 {item.assignee}</span>
                </div>
              </div>
              <h5>{item.title}</h5>
              <div class="creator-meta font-mono">提单人: {item.creator || '系统'} ({item.creator_dept || '无部门'})</div>
              
              <div class="branch-line font-mono">
                🌿 {item.branch && item.branch !== '-' ? item.branch : '待建分支'}
              </div>

              {#if item.task_group_id && item.task_group_id !== '-' && item.task_group_id !== ''}
                {@const subTasks = getSubTasksForDemand(item.task_group_id)}
                {#if subTasks.length > 0}
                  {@const completedCount = subTasks.filter(t => t.status === 'done').length}
                  {@const percent = Math.round((completedCount / subTasks.length) * 100)}
                  <div class="deconstruct-subtasks-box font-mono" on:click|stopPropagation>
                    <div class="subtask-progress-row" on:click={() => toggleDemandSubtasks(item.task_id)}>
                      <span class="subtask-label">🤖 AI 拆分任务 ({completedCount}/{subTasks.length})</span>
                      <div class="subtask-bar">
                        <div class="subtask-bar-fill" style="width: {percent}%"></div>
                      </div>
                      <span class="expand-arrow">{expandedDemands[item.task_id] ? '▲' : '▼'}</span>
                    </div>
                    {#if expandedDemands[item.task_id]}
                      <div class="subtasks-list-details">
                        {#each subTasks as sub}
                          <div class="subtask-detail-item" title={sub.title}>
                            <span class="status-dot {sub.status}"></span>
                            <span class="sub-repo">[{sub.repo}]</span>
                            <span class="sub-title">{sub.title}</span>
                            <span class="sub-assignee">👤{sub.assignee}</span>
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/if}
              {/if}

              {#if displayRepo(item.repo)}
                <div class="mapped-repo-line font-mono">{displayRepo(item.repo)}</div>
              {/if}
              <div class="card-bottom">
                {#if priority}
                  <span class="priority-badge p-{priority.toLowerCase()}">{priority}</span>
                {/if}
                <span class="due-badge {item.due_date ? dueInfo.className : 'text-blue'} font-mono">
                  {item.due_date ? dueInfo.text : '已分派'}
                </span>
                {#if isAssignee(item) || hasPermission('demands:write')}
                  <button class="edit-sched-btn" on:click|stopPropagation={() => openScheduleModal(item)}>
                    ⚙️
                  </button>
                {/if}
              </div>
            </div>
          {/each}
          {#if scheduledDemands.length === 0}
            <div class="empty-lane font-mono">暂无已排期需求</div>
          {/if}
        </div>
      </div>

      <!-- 3. In Progress -->
      <div class="kanban-lane">
        <div class="lane-header">
          <span class="lane-indicator bg-purple"></span>
          <h4>开发中 ({inProgressDemands.length})</h4>
        </div>
        <div class="lane-cards">
          {#each inProgressDemands as item}
            {@const dueInfo = getDueStatus(item.due_date)}
            {@const priority = getProjectPriority(item.task_id)}
            <div class="demand-card border-purple-dim" role="button" tabindex="0" on:click={() => openDemandDetails(item)} on:keydown={(event) => handleDemandCardKeydown(event, item)}>
              <div class="card-top">
                {#if getJiraIssueUrl(item.task_id)}
                  <a class="demand-id demand-id-button jira-id-link font-mono" href={getJiraIssueUrl(item.task_id)} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                    #{item.task_id}
                  </a>
                {:else}
                  <button class="demand-id demand-id-button font-mono" on:click|stopPropagation={() => openDemandDetails(item)}>
                    #{item.task_id}
                  </button>
                {/if}
                <div class="card-actions">
                  <button class="icon-action-btn telemetry-action-btn" title="查看代码轨迹" on:click|stopPropagation={() => { activeTelemetryTaskId = item.task_id; isTelemetryDrawerOpen = true; }}>🚀</button>
                  {#if canManageDemand(item)}
                    <button class="icon-action-btn" title="归档需求" on:click|stopPropagation={() => handleArchiveDemand(item.task_id)}>📁</button>
                    <button class="icon-action-btn" title="物理删除" on:click|stopPropagation={() => handleDeleteDemand(item.task_id)}>🗑️</button>
                  {/if}
                  <span class="assignee-badge font-mono">👤 {item.assignee}</span>
                </div>
              </div>
              <h5>{item.title}</h5>
              <div class="creator-meta font-mono">提单人: {item.creator || '系统'} ({item.creator_dept || '无部门'})</div>
              
              <div class="branch-line font-mono">
                🌿 {item.branch}
              </div>

              {#if item.task_group_id && item.task_group_id !== '-' && item.task_group_id !== ''}
                {@const subTasks = getSubTasksForDemand(item.task_group_id)}
                {#if subTasks.length > 0}
                  {@const completedCount = subTasks.filter(t => t.status === 'done').length}
                  {@const percent = Math.round((completedCount / subTasks.length) * 100)}
                  <div class="deconstruct-subtasks-box font-mono" on:click|stopPropagation>
                    <div class="subtask-progress-row" on:click={() => toggleDemandSubtasks(item.task_id)}>
                      <span class="subtask-label">🤖 AI 拆分任务 ({completedCount}/{subTasks.length})</span>
                      <div class="subtask-bar">
                        <div class="subtask-bar-fill" style="width: {percent}%"></div>
                      </div>
                      <span class="expand-arrow">{expandedDemands[item.task_id] ? '▲' : '▼'}</span>
                    </div>
                    {#if expandedDemands[item.task_id]}
                      <div class="subtasks-list-details">
                        {#each subTasks as sub}
                          <div class="subtask-detail-item" title={sub.title}>
                            <span class="status-dot {sub.status}"></span>
                            <span class="sub-repo">[{sub.repo}]</span>
                            <span class="sub-title">{sub.title}</span>
                            <span class="sub-assignee">👤{sub.assignee}</span>
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/if}
              {/if}

              <div class="mapped-repo-line font-mono">{displayRepo(item.repo)}</div>
              <div class="card-bottom">
                {#if priority}
                  <span class="priority-badge p-{priority.toLowerCase()}">{priority}</span>
                {/if}
                <span class="due-badge {dueInfo.className} font-mono">{dueInfo.text}</span>
                <span class="status-badge font-mono">{item.status === 'review' ? '👀评审中' : '💻进行中'}</span>
                {#if isAssignee(item) || hasPermission('demands:write')}
                  <button class="edit-sched-btn" on:click|stopPropagation={() => openScheduleModal(item)}>
                    ⚙️
                  </button>
                {/if}
              </div>
            </div>
          {/each}
          {#if inProgressDemands.length === 0}
            <div class="empty-lane font-mono">暂无开发中需求</div>
          {/if}
        </div>
      </div>

      <!-- 4. Delivered -->
      <div class="kanban-lane">
        <div class="lane-header">
          <span class="lane-indicator bg-green"></span>
          <h4>已交付 ({deliveredDemands.length})</h4>
        </div>
        <div class="lane-cards">
          {#each deliveredDemands as item}
            {@const priority = getProjectPriority(item.task_id)}
            <div class="demand-card border-green-dim card-done" role="button" tabindex="0" on:click={() => openDemandDetails(item)} on:keydown={(event) => handleDemandCardKeydown(event, item)}>
              <div class="card-top">
                {#if getJiraIssueUrl(item.task_id)}
                  <a class="demand-id demand-id-button jira-id-link font-mono" href={getJiraIssueUrl(item.task_id)} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                    #{item.task_id}
                  </a>
                {:else}
                  <button class="demand-id demand-id-button font-mono" on:click|stopPropagation={() => openDemandDetails(item)}>
                    #{item.task_id}
                  </button>
                {/if}
                <div class="card-actions">
                  <button class="icon-action-btn telemetry-action-btn" title="查看代码轨迹" on:click|stopPropagation={() => { activeTelemetryTaskId = item.task_id; isTelemetryDrawerOpen = true; }}>🚀</button>
                  {#if canManageDemand(item)}
                    <button class="icon-action-btn" title="归档需求" on:click|stopPropagation={() => handleArchiveDemand(item.task_id)}>📁</button>
                    <button class="icon-action-btn" title="物理删除" on:click|stopPropagation={() => handleDeleteDemand(item.task_id)}>🗑️</button>
                  {/if}
                  <span class="assignee-badge font-mono text-muted">👤 {item.assignee}</span>
                </div>
              </div>
              <h5 class="line-through">{item.title}</h5>
              <div class="creator-meta font-mono">提单人: {item.creator || '系统'} ({item.creator_dept || '无部门'})</div>
              
              {#if item.task_group_id && item.task_group_id !== '-' && item.task_group_id !== ''}
                {@const subTasks = getSubTasksForDemand(item.task_group_id)}
                {#if subTasks.length > 0}
                  {@const completedCount = subTasks.filter(t => t.status === 'done').length}
                  {@const percent = Math.round((completedCount / subTasks.length) * 100)}
                  <div class="deconstruct-subtasks-box font-mono" on:click|stopPropagation>
                    <div class="subtask-progress-row" on:click={() => toggleDemandSubtasks(item.task_id)}>
                      <span class="subtask-label">🤖 AI 拆分任务 ({completedCount}/{subTasks.length})</span>
                      <div class="subtask-bar">
                        <div class="subtask-bar-fill" style="width: {percent}%"></div>
                      </div>
                      <span class="expand-arrow">{expandedDemands[item.task_id] ? '▲' : '▼'}</span>
                    </div>
                    {#if expandedDemands[item.task_id]}
                      <div class="subtasks-list-details">
                        {#each subTasks as sub}
                          <div class="subtask-detail-item" title={sub.title}>
                            <span class="status-dot {sub.status}"></span>
                            <span class="sub-repo">[{sub.repo}]</span>
                            <span class="sub-title">{sub.title}</span>
                            <span class="sub-assignee">👤{sub.assignee}</span>
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/if}
              {/if}
              
              <div class="card-bottom mt-1">
                {#if priority}
                  <span class="priority-badge p-{priority.toLowerCase()}">{priority}</span>
                {/if}
                <span class="done-tag font-mono">🎉 已发布</span>
                {#if item.completed_at}
                  <span class="done-date font-mono">{new Date(item.completed_at).toLocaleDateString()}</span>
                {/if}
                {#if isAssignee(item) || hasPermission('demands:write')}
                  <button class="edit-sched-btn" on:click|stopPropagation={() => openScheduleModal(item)}>
                    ⚙️
                  </button>
                {/if}
              </div>
            </div>
          {/each}
          {#if deliveredDemands.length === 0}
            <div class="empty-lane font-mono">暂无已交付需求</div>
          {/if}
        </div>
      </div>

    </div>
  {/if}

  <!-- Create Demand Modal -->
  {#if showCreateModal}
    <div use:portalToWorkspaceStage class="modal-backdrop demand-create-backdrop" class:has-ai-companion={showDeconstructorModal && deconstructorPresentation === 'companion' && deconstructorHost === 'create'} style="--ai-host-width: 680px" on:click={handleBackdropClick}>
      <div class="modal-content demand-create-modal glass-panel" bind:clientHeight={companionHostHeight}>
        <div class="modal-header">
          <h3>📋 录入新产品需求</h3>
          <button class="close-btn" on:click={closeCreateDemandModal} aria-label="关闭新需求弹窗">&times;</button>
        </div>
        
        <div class="form-body">
          <div class="form-group">
            <label for="demand-title">需求标题 <span class="text-rose">*</span></label>
            <input type="text" id="demand-title" bind:value={newTitle} placeholder="例如：实现用户所属部门的查询及显示" />
          </div>

          <div class="form-group">
            <label for="demand-desc">需求详情 / 规格说明</label>
            <textarea id="demand-desc" rows="4" bind:value={newDescription} placeholder="请输入需求的具体内容，AI 需求解构内核在同步时会自动读取该字段..."></textarea>
          </div>

          <div class="form-group">
            <label for="demand-project">所属项目</label>
            <div class="custom-dropdown-container" id="demand-project-container">
              <div class="combobox-trigger-wrapper">
                <input
                  type="text"
                  class="dropdown-trigger-input"
                  placeholder={getProjectDisplayName(newRepo)}
                  bind:value={projectSearchText}
                  on:focus|stopPropagation={() => showProjectDropdown = true}
                />
                <span class="arrow-icon {showProjectDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showProjectDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each (projectSearchText && projectSearchText.trim() ? createProjectOptions.filter(proj => getProjectDisplayName(proj).toLowerCase().includes(projectSearchText.trim().toLowerCase())) : createProjectOptions) as project}
                    <button
                      type="button"
                      class="dropdown-option-item {newRepo === project ? 'selected' : ''}"
                      on:click|stopPropagation={() => {
                        newRepo = project;
                        projectSearchText = '';
                        showProjectDropdown = false;
                      }}
                    >
                      {getProjectDisplayName(project)}
                    </button>
                  {/each}
                  {#if createProjectOptions.length <= 1 && !projectSearchText}
                    <div class="dropdown-empty">暂无项目候选，请先在“配置中心 / 项目映射”中维护。</div>
                  {/if}
                </div>
              {/if}
            </div>
            <span class="field-hint">用于后续排期和需求归属聚合；口头需求可先暂不指定。</span>
          </div>

          <div class="form-group">
            <label for="demand-assignee">指派负责人 <span class="text-rose">*</span></label>
            <div class="custom-dropdown-container" id="demand-assignee-container">
              <div class="combobox-trigger-wrapper">
                <input
                  type="text"
                  class="dropdown-trigger-input"
                  placeholder={newAssignee || '请选择负责人'}
                  bind:value={assigneeSearchText}
                  on:focus|stopPropagation={() => showAssigneeDropdown = true}
                />
                <span class="arrow-icon {showAssigneeDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showAssigneeDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each createAssigneeOptions.filter(name => !assigneeSearchText || name.toLowerCase().includes(assigneeSearchText.toLowerCase())) as assignee}
                    <button
                      type="button"
                      class="dropdown-option-item {newAssignee === assignee ? 'selected' : ''}"
                      on:click|stopPropagation={() => {
                        newAssignee = assignee;
                        showAssigneeDropdown = false;
                      }}
                    >
                      {assignee}
                    </button>
                  {/each}
                  {#if createAssigneeOptions.length === 0}
                    <div class="dropdown-empty">暂无负责人候选</div>
                  {/if}
                </div>
              {/if}
            </div>
          </div>

          <div class="form-group">
            <label for="demand-due">期望截止交付日期</label>
            <div class="date-input-shell">
              <button type="button" id="demand-due" class="date-input-display {newDueDate ? 'has-value' : ''}" on:click|stopPropagation={() => openDatePicker('new')}>
                <span class="date-input-value">{newDueDateDisplay || '选择截止日期'}</span>
                <span class="date-input-icon"></span>
              </button>
              {#if activeDatePicker === 'new'}
                {@const calendar = getCalendarDays(newDueDate, datePickerCursor)}
                <div class="date-picker-panel">
                  <div class="date-picker-head">
                    <button type="button" on:click={(event) => moveDateMonth(-1, event)} aria-label="上个月">‹</button>
                    <strong>{calendar.label}</strong>
                    <button type="button" on:click={(event) => moveDateMonth(1, event)} aria-label="下个月">›</button>
                  </div>
                  <div class="date-week-grid font-mono">
                    {#each weekdayNames as day}<span>{day}</span>{/each}
                  </div>
                  <div class="date-grid">
                    {#each calendar.days as day}
                      <button
                        type="button"
                        class="date-cell {day.muted ? 'muted' : ''} {day.today ? 'today' : ''} {day.selected ? 'selected' : ''}"
                        on:click|stopPropagation={() => selectDate('new', day.value)}
                      >
                        {day.label}
                      </button>
                    {/each}
                  </div>
                  <div class="date-picker-foot">
                    <button type="button" on:click|stopPropagation={() => clearDate('new')}>清空日期</button>
                    <button type="button" on:click|stopPropagation={() => selectDate('new', toDateValue(new Date()))}>今天</button>
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button
            class="ai-draft-btn font-mono"
            disabled={!newTitle.trim() && !newDescription.trim()}
            on:click={() => openDeconstructorWorkspace(null, '新需求预解构', 'companion', 'create')}
          >AI 预解构</button>
          <button class="cancel-btn font-mono" on:click={closeCreateDemandModal}>取消</button>
          <button class="submit-btn font-mono" on:click={handleCreateDemand}>指派需求</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Schedule Demand Modal -->
  {#if showScheduleModal && selectedDemand}
    <div class="modal-backdrop" class:has-ai-companion={showDeconstructorModal && deconstructorPresentation === 'companion' && deconstructorHost === 'schedule'} style="--ai-host-width: 720px" on:click={closeScheduleModal}>
      <div class="modal-content schedule-modal" bind:clientHeight={companionHostHeight} on:click|stopPropagation={handleScheduleModalClick}>
        <div class="modal-header">
          <h3>{selectedDemand.issue_type === 'bug' ? '缺陷' : '需求'}开发排期与指派 · #{selectedDemand.task_id}</h3>
          <button class="close-btn" on:click={closeScheduleModal} aria-label="关闭排期弹窗">&times;</button>
        </div>

        <div class="form-body">
          <p class="demand-brief font-mono">标题: {selectedDemand.title}</p>

          <div class="form-group">
            <label for="sched-assignee">负责人指派</label>
            <div class="custom-dropdown-container" id="sched-assignee-container">
              <div class="combobox-trigger-wrapper">
                <input
                  id="sched-assignee"
                  type="text"
                  class="dropdown-trigger-input"
                  placeholder={schedAssignee || '请选择负责人'}
                  bind:value={schedAssigneeSearchText}
                  on:focus|stopPropagation={() => showScheduleModalAssigneeDropdown = true}
                />
                <span class="arrow-icon {showScheduleModalAssigneeDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showScheduleModalAssigneeDropdown}
                <div class="dropdown-options-list glass-panel" style="position: absolute; z-index: 1000; width: 100%; max-height: 200px; overflow-y: auto;">
                  {#each createAssigneeOptions.filter(name => !schedAssigneeSearchText || name.toLowerCase().includes(schedAssigneeSearchText.toLowerCase())) as assignee}
                    <button
                      type="button"
                      class="dropdown-option-item {schedAssignee === assignee ? 'selected' : ''}"
                      on:click={() => {
                        schedAssignee = assignee;
                        schedAssigneeSearchText = '';
                        showScheduleModalAssigneeDropdown = false;
                      }}
                    >
                      {assignee}
                    </button>
                  {/each}
                  {#if createAssigneeOptions.filter(name => !schedAssigneeSearchText || name.toLowerCase().includes(schedAssigneeSearchText.toLowerCase())).length === 0}
                    <div class="dropdown-empty">暂无匹配的候选人</div>
                  {/if}
                </div>
              {/if}
            </div>
          </div>

          <div class="schedule-estimate-panel">
            <div class="estimate-panel-head">
              <div>
                <strong>工时设置汇总</strong>
              </div>
              <div class="estimate-actions">
                <button
                  type="button"
                  class="estimate-ai-btn font-mono"
                  class:is-loading={scheduleEstimateLoading}
                  disabled={scheduleEstimateLoading}
                  on:click={handleEstimateScheduleEffort}
                >
                  {scheduleEstimateLoading ? '评估中' : 'AI 评估'}
                </button>
                <button type="button" class="estimate-clear-btn font-mono" on:click={clearScheduleEstimate}>清空</button>
              </div>
            </div>
            <div class="estimate-manual-grid">
              <label class="estimate-field" for="sched-estimate-hours">
                <span>预估小时</span>
                <input
                  id="sched-estimate-hours"
                  type="number"
                  min="0"
                  step="0.5"
                  inputmode="decimal"
                  value={schedEstimateHours > 0 ? formatOneDecimal(schedEstimateHours) : ''}
                  placeholder="0"
                  on:input={updateScheduleEstimateHours}
                />
              </label>
              <div class="estimate-difficulty-field difficulty-select-shell">
                <span id="sched-difficulty-label">难度</span>
                <div class="difficulty-select-control">
                  <Select
                    id="sched-difficulty"
                    value={schedDifficulty}
                    options={difficultyOptions}
                    placeholder="未设置"
                    searchable={false}
                    compact={true}
                    on:change={(event) => updateScheduleDifficulty(event.detail)}
                  />
                </div>
              </div>
            </div>
            {#if scheduleEstimateError}
              <p class="estimate-error" role="alert">
                <span class="estimate-error-icon" aria-hidden="true">!</span>
                <span>{scheduleEstimateError}</span>
              </p>
            {/if}
          </div>

          <div class="form-group">
            <label for="sched-due">截止完成日期</label>
            <div class="date-input-shell">
              <button type="button" id="sched-due" class="date-input-display {schedDueDate ? 'has-value' : ''}" on:click|stopPropagation={() => openDatePicker('schedule')}>
                <span class="date-input-value">{schedDueDateDisplay || '选择截止日期'}</span>
                <span class="date-input-icon"></span>
              </button>
              {#if activeDatePicker === 'schedule'}
                {@const calendar = getCalendarDays(schedDueDate, datePickerCursor)}
                <div class="date-picker-panel">
                  <div class="date-picker-head">
                    <button type="button" on:click={(event) => moveDateMonth(-1, event)} aria-label="上个月">‹</button>
                    <strong>{calendar.label}</strong>
                    <button type="button" on:click={(event) => moveDateMonth(1, event)} aria-label="下个月">›</button>
                  </div>
                  <div class="date-week-grid font-mono">
                    {#each weekdayNames as day}<span>{day}</span>{/each}
                  </div>
                  <div class="date-grid">
                    {#each calendar.days as day}
                      <button
                        type="button"
                        class="date-cell {day.muted ? 'muted' : ''} {day.today ? 'today' : ''} {day.selected ? 'selected' : ''}"
                        on:click|stopPropagation={() => selectDate('schedule', day.value)}
                      >
                        {day.label}
                      </button>
                    {/each}
                  </div>
                  <div class="date-picker-foot">
                    <button type="button" on:click|stopPropagation={() => clearDate('schedule')}>清空日期</button>
                    <button type="button" on:click|stopPropagation={() => selectDate('schedule', toDateValue(new Date()))}>今天</button>
                  </div>
                </div>
              {/if}
            </div>
          </div>

          <div class="form-group">
            <label for="sched-task-group">关联 AI 解构任务组</label>
            <div class="group-lock-field font-mono" id="sched-task-group">
              <span>{schedTaskGroupID}</span>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="ai-draft-btn font-mono" on:click={() => openDeconstructorWorkspace(selectedDemand, '排期需求解构', 'companion', 'schedule')}>AI 解构</button>
          <button class="cancel-btn font-mono" on:click={closeScheduleModal}>取消</button>
          <button class="submit-btn font-mono" on:click={() => handleSaveSchedule('modal')}>确认排期</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Demand Details Drawer -->
  {#if showDemandDetailsModal && detailDemand}
    <div use:portalToConsole class="modal-backdrop detail-backdrop" class:has-ai-companion={showDeconstructorModal && deconstructorPresentation === 'companion' && deconstructorHost === 'details'} style="--ai-host-width: 1080px" on:click={handleDemandDetailsBackdropClick}>
      <div
        class="modal-content detail-modal"
        bind:this={detailDrawerEl}
        bind:clientHeight={companionHostHeight}
        role="dialog"
        aria-modal="true"
        aria-labelledby="demand-detail-drawer-title"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown={handleDemandDetailsKeydown}
      >
        <div class="modal-header detail-modal-header">
          <div class="detail-modal-heading">
            <span>排期治理 / 流转看板</span>
            <h3 id="demand-detail-drawer-title">需求详情</h3>
          </div>
          <button type="button" class="close-btn" bind:this={detailCloseButton} on:click={() => closeDemandDetails()} aria-label="关闭需求详情抽屉">&times;</button>
        </div>

        <div class="detail-body">
          <section class="detail-overview" aria-labelledby="detail-demand-title">
            <div class="detail-title-block">
              {#if getJiraIssueUrl(detailDemand.task_id)}
                <a class="detail-id font-mono jira-id-link" href={getJiraIssueUrl(detailDemand.task_id)} target="_blank" rel="noopener noreferrer">
                  #{detailDemand.task_id}
                </a>
              {:else}
                <span class="detail-id font-mono">#{detailDemand.task_id}</span>
              {/if}
              <strong id="detail-demand-title">{detailDemand.title}</strong>
              <p>{detailDemand.description || '暂无需求说明'}</p>
            </div>

            <div class="detail-link-row">
              <button type="button" class="wa-admin-action primary ai-workbench-action detail-action-ai" on:click={() => openDeconstructorWorkspace(detailDemand, '需求详情解构', 'companion', 'details')}>AI 解构需求</button>
              {#if isAssignee(detailDemand) || hasPermission('demands:write') || canManageDemand(detailDemand)}
                <button type="button" class="wa-admin-action secondary detail-action-schedule" on:click={() => {
                  const demandToSchedule = detailDemand;
                  if (demandToSchedule) {
                    closeDemandDetails(false);
                    openScheduleModal(demandToSchedule);
                  }
                }}>调整排期</button>
              {/if}
            </div>
          </section>

          <dl class="detail-facts-grid">
            <div>
              <dt>负责人</dt>
              <dd>{detailDemand.assignee || '未指派'}</dd>
            </div>
            <div>
              <dt>流转状态</dt>
              <dd>{detailDemand.status || '-'}</dd>
            </div>
            <div>
              <dt>截止日期</dt>
              <dd>{detailDemand.due_date ? formatDateDisplay(detailDemand.due_date.slice(0, 10)) : '未排期'}</dd>
            </div>
            <div>
              <dt>提单人</dt>
              <dd>{detailDemand.creator || '系统'}</dd>
            </div>
            <div>
              <dt>所属部门</dt>
              <dd>{detailDemand.creator_dept || '无部门'}</dd>
            </div>
            <div>
              <dt>任务组</dt>
              <dd>{detailDemand.task_group_id || getEffectiveTaskGroupId(detailDemand)}</dd>
            </div>
          </dl>

          <div class="detail-subtasks">
            <div class="detail-section-title">
              <span>影子任务</span>
              <strong>{detailSubtasks.length}</strong>
            </div>
            {#if detailSubtasks.length > 0}
              <div class="detail-subtask-list">
                {#each detailSubtasks as sub}
                  <div class="detail-subtask-row">
                    <span class="status-dot {sub.status}"></span>
                    <strong>{sub.title}</strong>
                    <em>{sub.assignee || '未指派'} / {sub.status || '-'}</em>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="detail-empty">尚未导入 AI 影子任务。</div>
            {/if}
          </div>

          <DemandDeliveryControl
            demand={detailDemand}
            {currentUserPermissions}
            {currentUserName}
            {currentUserEmail}
            userDirectory={users}
            coreMemberNames={demandOptionAssignees}
            coreMemberDirectoryReady={demandOptionAssigneesLoaded}
          />
        </div>
      </div>
    </div>
  {/if}

  {#if showDeconstructorModal}
    <div
      use:portalToWorkspaceStage={deconstructorPresentation === 'companion' && deconstructorHost === 'create'}
      use:portalToConsole={deconstructorPresentation === 'companion' && deconstructorHost === 'details'}
      class="modal-backdrop deconstructor-backdrop"
      class:is-companion={deconstructorPresentation === 'companion'}
      class:detail-companion={deconstructorPresentation === 'companion' && deconstructorHost === 'details'}
      class:create-companion={deconstructorPresentation === 'companion' && deconstructorHost === 'create'}
      style="--ai-host-width: {deconstructorHost === 'details' ? '1080px' : deconstructorHost === 'schedule' ? '720px' : '680px'}"
      on:click={handleDeconstructorBackdropClick}
    >
      <div
        class="modal-content deconstructor-modal"
        class:is-companion={deconstructorPresentation === 'companion'}
        style="--companion-height: {companionHostHeight}px"
        role="dialog"
        aria-modal={deconstructorPresentation === 'modal'}
        aria-labelledby="deconstructor-modal-title"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown={handleDeconstructorKeydown}
      >
        <div class="modal-header">
          <div class="deconstructor-modal-title">
            <span class="deconstructor-context">AI 解构工作台</span>
            <h3 id="deconstructor-modal-title">{deconstructorContextLabel}</h3>
          </div>
          <button class="close-btn" bind:this={deconstructorCloseButton} on:click={() => closeDeconstructorWorkspace()} aria-label="关闭 AI 解构工作台">&times;</button>
        </div>
        <div class="deconstructor-modal-body">
          <Deconstructor
            {currentUserPermissions}
            initialTitle={deconstructorTitle}
            initialDescription={deconstructorDescription}
            initialDemandId={deconstructorDemandId}
            embedded={true}
            compact={true}
          />
        </div>
      </div>
    </div>
  {/if}

  <!-- Confirm Modal -->
  {#if showConfirmModal}
    <div class="modal-backdrop" on:click={() => showConfirmModal = false}>
      <div class="modal-content confirm-modal confirm-modal-{confirmType}" on:click|stopPropagation>
        <div class="confirm-header">
          <div>
            <div class="confirm-kicker font-mono">{confirmType === 'delete' ? 'IRREVERSIBLE ACTION' : 'FLOW CONTROL'}</div>
            <h3>{confirmTitle}</h3>
          </div>
          <button class="close-btn confirm-close" on:click={() => showConfirmModal = false} aria-label="关闭确认弹窗">&times;</button>
        </div>
        <div class="confirm-body">
          <div class="confirm-id-row">
            <span class="confirm-id-label font-mono">TARGET</span>
            <span class="confirm-id-value font-mono">#{confirmTaskId}</span>
          </div>
          <p class="confirm-message">{confirmMessage}</p>
        </div>
        <div class="modal-footer confirm-footer">
          <button class="cancel-btn font-mono" on:click={() => showConfirmModal = false}>取消</button>
          <button class="submit-btn font-mono confirm-btn {confirmType === 'delete' ? 'confirm-btn-delete' : 'confirm-btn-archive'}" on:click={confirmAction}>
            {confirmType === 'delete' ? '确认删除' : '确认归档'}
          </button>
        </div>
      </div>
    </div>
  {/if}
  
  <CommitTelemetryPanel taskID={activeTelemetryTaskId} isOpen={isTelemetryDrawerOpen} onClose={() => isTelemetryDrawerOpen = false} />
</div>

<style>
  .priority-badge {
    display: inline-block;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.7rem;
    font-weight: 700;
    font-family: monospace;
    margin-left: 6px;
  }

  @keyframes p0-breath {
    0% {
      box-shadow: 0 0 4px rgba(239, 68, 68, 0.4), inset 0 0 2px rgba(239, 68, 68, 0.2);
      border-color: rgba(239, 68, 68, 0.4);
    }
    50% {
      box-shadow: 0 0 14px rgba(239, 68, 68, 0.9), inset 0 0 6px rgba(239, 68, 68, 0.5);
      border-color: rgba(239, 68, 68, 0.8);
    }
    100% {
      box-shadow: 0 0 4px rgba(239, 68, 68, 0.4), inset 0 0 2px rgba(239, 68, 68, 0.2);
      border-color: rgba(239, 68, 68, 0.4);
    }
  }

  .priority-badge.p-p0 {
    background: rgba(239, 68, 68, 0.25);
    color: #ff8080;
    border: 1px solid rgba(239, 68, 68, 0.5);
    animation: p0-breath 1.2s infinite ease-in-out;
  }

  .priority-badge.p-p1 {
    background: rgba(249, 115, 22, 0.2);
    color: #ff9d5c;
    border: 1px solid rgba(249, 115, 22, 0.5);
    box-shadow: 0 0 10px rgba(249, 115, 22, 0.5), inset 0 0 3px rgba(249, 115, 22, 0.3);
  }

  .priority-badge.p-p2 {
    background: rgba(245, 158, 11, 0.2);
    color: #ffc83b;
    border: 1px solid rgba(245, 158, 11, 0.5);
    box-shadow: 0 0 8px rgba(245, 158, 11, 0.4), inset 0 0 2px rgba(245, 158, 11, 0.2);
  }

  .priority-badge.p-p3 {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border: 1px solid rgba(59, 130, 246, 0.3);
    box-shadow: none;
  }

  .priority-badge.p-p4 {
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.3);
    box-shadow: none;
  }

  .priority-badge.p-p5 {
    background: rgba(148, 163, 184, 0.15);
    color: #94a3b8;
    border: 1px solid rgba(148, 163, 184, 0.3);
    box-shadow: none;
  }

  /* Project Swimlanes Styling */
  @keyframes pg-p0-breath {
    0% {
      background: rgba(239, 68, 68, 0.05);
      box-shadow: inset 0 0 0 1px rgba(239, 68, 68, 0.28);
    }
    50% {
      background: rgba(239, 68, 68, 0.12);
      box-shadow: inset 0 0 0 1px rgba(239, 68, 68, 0.46);
    }
    100% {
      background: rgba(239, 68, 68, 0.05);
      box-shadow: inset 0 0 0 1px rgba(239, 68, 68, 0.28);
    }
  }

  .project-group-header {
    background: rgba(30, 41, 59, 0.85);
    cursor: pointer;
    user-select: none;
    transition: background 0.2s, box-shadow 0.2s;
  }
  .project-group-header:hover {
    background: rgba(51, 65, 85, 0.95);
  }

  .project-group-header.pg-p0 {
    animation: pg-p0-breath 1.2s infinite ease-in-out;
  }

  .project-group-header.pg-p1 {
    background: rgba(249, 115, 22, 0.06);
    box-shadow: inset 0 0 0 1px rgba(249, 115, 22, 0.24);
  }

  .project-group-header.pg-p2 {
    background: rgba(245, 158, 11, 0.05);
    box-shadow: inset 0 0 0 1px rgba(245, 158, 11, 0.22);
  }

  .project-group-header.pg-p3 {
    background: rgba(30, 41, 59, 0.5);
    box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.2);
  }
  .project-group-header.pg-p4 {
    background: rgba(30, 41, 59, 0.4);
    box-shadow: inset 0 0 0 1px rgba(99, 102, 241, 0.18);
  }
  .project-group-header.pg-p5 {
    background: rgba(30, 41, 59, 0.3);
    box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.16);
  }

  .project-group-cell {
    padding: 10px 16px !important;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  }

  .project-group-inner {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .collapse-chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #6366f1;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .collapse-chevron.is-collapsed {
    transform: rotate(-90deg);
  }

  .group-project-key {
    color: #f8fafc;
    font-size: 0.95rem;
    font-weight: 800;
    letter-spacing: 0.5px;
  }

  .group-count {
    font-size: 0.75rem;
    color: #94a3b8;
    margin-left: auto;
  }

  .demand-dashboard {
    display: flex;
    flex-direction: column;
    gap: 24px;
    margin-top: 10px;
    color: #e2e8f0;
  }

  .dashboard-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
  }

  .eyebrow {
    font-size: 0.65rem;
    font-weight: 800;
    color: #64748b;
    letter-spacing: 0.1em;
    display: block;
    margin-bottom: 6px;
    text-transform: uppercase;
  }

  .dashboard-header h2 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 800;
    color: #f8fafc;
  }

  .add-demand-btn {
    background: #6366f1;
    border: none;
    color: white;
    padding: 8px 18px;
    font-size: 0.8rem;
    font-weight: 700;
    border-radius: 8px;
    cursor: pointer;
    box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);
    transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .add-demand-btn:hover {
    background: #4f46e5;
    transform: translateY(-1px);
    box-shadow: 0 6px 20px rgba(99, 102, 241, 0.5);
  }

  .schedule-workbench {
    display: flex;
    flex-direction: column;
    gap: 14px;
    overflow-anchor: none;
  }

  .schedule-summary-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(130px, 1fr));
    gap: 10px;
  }

  .schedule-summary-cell {
    min-height: 92px;
    background: rgba(10, 15, 30, 0.66);
    border: 1px solid rgba(51, 65, 85, 0.32);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .schedule-summary-cell.is-blue { border-color: rgba(56, 189, 248, 0.28); }
  .schedule-summary-cell.is-red { border-color: rgba(239, 68, 68, 0.28); }
  .schedule-summary-cell.is-amber { border-color: rgba(245, 158, 11, 0.28); }
  .schedule-summary-cell.is-violet { border-color: rgba(167, 139, 250, 0.28); }

  .summary-label {
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 900;
  }

  .schedule-summary-cell strong {
    color: #f8fafc;
    font-size: 1.45rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .schedule-summary-cell em {
    color: #94a3b8;
    font-size: 0.72rem;
    font-style: normal;
  }

  .schedule-risk-calendar-panel {
    background:
      linear-gradient(180deg, rgba(15, 23, 42, 0.82), rgba(8, 13, 25, 0.72)),
      radial-gradient(circle at 12% 0%, rgba(56, 189, 248, 0.12), transparent 32%),
      radial-gradient(circle at 88% 0%, rgba(244, 63, 94, 0.1), transparent 30%);
    border: 1px solid rgba(71, 85, 105, 0.44);
    border-radius: 12px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.22);
  }

  .risk-calendar-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .risk-calendar-head h3 {
    margin: 0;
    color: #f8fafc;
    font-size: 1.04rem;
  }

  .risk-calendar-head p {
    margin: 6px 0 0;
    color: #cbd5e1;
    font-size: 0.78rem;
    line-height: 1.42;
  }

  .risk-calendar-meta {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    color: #64748b;
    font-size: 0.68rem;
    white-space: nowrap;
  }

  .risk-calendar-meta span,
  .risk-calendar-meta button {
    border: 1px solid rgba(71, 85, 105, 0.42);
    border-radius: 999px;
    background: rgba(2, 6, 23, 0.36);
    padding: 6px 9px;
  }

  .risk-calendar-meta button {
    color: #bae6fd;
    cursor: pointer;
    font-weight: 800;
    transition: background 160ms ease, border-color 160ms ease, color 160ms ease, transform 160ms ease;
  }

  .risk-calendar-meta button:hover:not(:disabled) {
    color: #f8fafc;
    background: rgba(14, 165, 233, 0.18);
    border-color: rgba(56, 189, 248, 0.38);
    transform: translateY(-1px);
  }

  .risk-calendar-meta button:disabled {
    cursor: wait;
    opacity: 0.64;
  }

  .risk-calendar-command-row {
    display: grid;
    grid-template-columns: 1.2fr repeat(4, minmax(0, 1fr));
    gap: 8px;
  }

  .risk-calendar-command-row button {
    min-height: 50px;
    border: 1px solid rgba(71, 85, 105, 0.38);
    border-radius: 10px;
    background: rgba(2, 6, 23, 0.28);
    color: #94a3b8;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 10px;
    cursor: pointer;
    transition: transform 160ms ease, background 160ms ease, border-color 160ms ease, color 160ms ease;
    min-width: 0;
  }

  .risk-calendar-command-row button:hover {
    transform: translateY(-1px);
    color: #f8fafc;
    border-color: rgba(148, 163, 184, 0.42);
    background: rgba(15, 23, 42, 0.74);
  }

  .risk-calendar-command-row button.active {
    color: #f8fafc;
    background: rgba(99, 102, 241, 0.2);
    border-color: rgba(129, 140, 248, 0.48);
    box-shadow: inset 0 0 0 1px rgba(129, 140, 248, 0.12);
  }

  .risk-calendar-command-row span {
    font-size: 0.72rem;
    font-weight: 800;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .risk-calendar-command-row strong {
    color: #f8fafc;
    font-size: 1rem;
    font-variant-numeric: tabular-nums;
  }

  .risk-calendar-command-row .risk-overdue strong { color: #fb7185; }
  .risk-calendar-command-row .risk-due_soon strong { color: #fbbf24; }
  .risk-calendar-command-row .risk-stale strong { color: #c4b5fd; }
  .risk-calendar-command-row .risk-unscheduled strong { color: #7dd3fc; }

  .risk-calendar-warning {
    color: #fbbf24;
    background: rgba(245, 158, 11, 0.08);
    border: 1px solid rgba(245, 158, 11, 0.22);
    border-radius: 8px;
    padding: 8px 10px;
    font-size: 0.68rem;
  }

  .risk-calendar-loading,
  .risk-calendar-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .risk-calendar-loading span {
    min-height: 138px;
    border-radius: 10px;
    background: linear-gradient(90deg, rgba(30, 41, 59, 0.52), rgba(51, 65, 85, 0.42), rgba(30, 41, 59, 0.52));
    background-size: 180% 100%;
    animation: calendar-shimmer 1.2s linear infinite;
  }

  @keyframes calendar-shimmer {
    from { background-position: 0 0; }
    to { background-position: -180% 0; }
  }

  .risk-calendar-bucket {
    min-width: 0;
    background: rgba(2, 6, 23, 0.3);
    border: 1px solid rgba(71, 85, 105, 0.4);
    border-radius: 12px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 11px;
    position: relative;
    overflow: hidden;
  }

  .risk-calendar-bucket::before {
    content: "";
    position: absolute;
    inset: 0 0 auto;
    height: 3px;
    background: linear-gradient(90deg, #fb7185, #fbbf24, #38bdf8);
    opacity: 0.72;
  }

  .bucket-title-row {
    display: flex;
    justify-content: space-between;
    gap: 10px;
    align-items: flex-start;
  }

  .bucket-title-row strong {
    display: flex;
    align-items: center;
    gap: 7px;
    color: #f8fafc;
    font-size: 0.9rem;
  }

  .bucket-dot {
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: #38bdf8;
    box-shadow: 0 0 12px rgba(56, 189, 248, 0.55);
  }

  .bucket-title-row span {
    display: block;
    color: #64748b;
    font-size: 0.66rem;
    margin-top: 3px;
  }

  .bucket-title-row em {
    color: #e2e8f0;
    font-style: normal;
    font-size: 1.24rem;
    font-variant-numeric: tabular-nums;
  }

  .bucket-risk-chips {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px;
  }

  .bucket-risk-chip {
    border: 1px solid rgba(71, 85, 105, 0.38);
    background: rgba(2, 6, 23, 0.28);
    border-radius: 9px;
    color: #64748b;
    padding: 7px 8px;
    cursor: pointer;
    min-width: 0;
    transition: border-color 0.16s ease, background 0.16s ease, color 0.16s ease, transform 0.16s ease;
  }

  .bucket-risk-chip:disabled {
    cursor: default;
    opacity: 0.62;
  }

  .bucket-risk-chip span {
    display: block;
    font-size: 0.62rem;
    white-space: nowrap;
  }

  .bucket-risk-chip strong {
    display: block;
    margin-top: 3px;
    font-size: 0.88rem;
    font-variant-numeric: tabular-nums;
  }

  .bucket-risk-chip.has-count.risk-overdue {
    color: #fecdd3;
    border-color: rgba(244, 63, 94, 0.36);
    background: rgba(244, 63, 94, 0.1);
  }

  .bucket-risk-chip.has-count.risk-due_soon {
    color: #fde68a;
    border-color: rgba(245, 158, 11, 0.34);
    background: rgba(245, 158, 11, 0.09);
  }

  .bucket-risk-chip.has-count.risk-stale {
    color: #c4b5fd;
    border-color: rgba(167, 139, 250, 0.34);
    background: rgba(124, 58, 237, 0.09);
  }

  .bucket-risk-chip.has-count.risk-unscheduled {
    color: #93c5fd;
    border-color: rgba(59, 130, 246, 0.32);
    background: rgba(59, 130, 246, 0.08);
  }

  .bucket-risk-chip.has-count:hover {
    border-color: rgba(226, 232, 240, 0.42);
    color: #f8fafc;
    transform: translateY(-1px);
  }

  .bucket-event-list {
    display: flex;
    flex-direction: column;
    gap: 7px;
    min-width: 0;
  }

  .bucket-event-item {
    width: 100%;
    text-align: left;
    border: 1px solid rgba(51, 65, 85, 0.34);
    background: rgba(15, 23, 42, 0.5);
    border-radius: 9px;
    padding: 8px 9px;
    color: #cbd5e1;
    cursor: pointer;
    min-width: 0;
  }

  .bucket-event-item.risk-overdue { border-color: rgba(244, 63, 94, 0.34); }
  .bucket-event-item.risk-due_soon { border-color: rgba(245, 158, 11, 0.34); }
  .bucket-event-item.risk-stale { border-color: rgba(167, 139, 250, 0.34); }
  .bucket-event-item.risk-unscheduled { border-color: rgba(96, 165, 250, 0.32); }

  .bucket-event-item:hover {
    background: rgba(30, 41, 59, 0.72);
    border-color: rgba(148, 163, 184, 0.34);
  }

  .bucket-event-item .event-task {
    display: block;
    color: #818cf8;
    font-size: 0.62rem;
    margin-bottom: 2px;
  }

  .bucket-event-item strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.73rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .bucket-event-item em {
    display: block;
    color: #64748b;
    font-style: normal;
    font-size: 0.62rem;
    margin-top: 3px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .bucket-empty {
    color: #64748b;
    border: 1px dashed rgba(71, 85, 105, 0.28);
    border-radius: 7px;
    padding: 10px;
    text-align: center;
    font-size: 0.66rem;
  }

  .schedule-control-panel {
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    gap: 10px;
    background: rgba(10, 15, 30, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 10px;
    padding: 10px;
  }

  .schedule-search-shell {
    position: relative;
    display: flex;
    align-items: center;
    flex: 2 1 280px;
    min-width: 220px;
  }

  .schedule-search-shell input {
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

  .schedule-search-shell input:focus {
    border-color: rgba(129, 140, 248, 0.78);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.15);
  }

  .search-mark {
    position: absolute;
    left: 12px;
    width: 11px;
    height: 11px;
    border: 2px solid #64748b;
    border-radius: 50%;
  }

  .search-mark::after {
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

  .schedule-assignee-menu,
  .schedule-project-menu {
    min-width: 130px;
  }

  .schedule-dropdown-menu {
    min-width: 110px;
  }

  .dropdown-trigger-btn {
    cursor: pointer !important;
  }

  .schedule-menu-trigger {
    width: 100%;
    height: 38px;
    background: rgba(15, 23, 42, 0.72);
    border: 1px solid rgba(71, 85, 105, 0.68);
    border-radius: 8px;
    color: #cbd5e1;
    padding: 0 11px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    font-size: 0.76rem;
    font-weight: 700;
  }

  .schedule-menu-trigger:hover {
    color: #f8fafc;
    border-color: rgba(129, 140, 248, 0.55);
  }

  .schedule-refresh-btn {
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

  .schedule-refresh-btn:hover {
    background: rgba(14, 165, 233, 0.2);
  }

  .schedule-refresh-btn.is-loading {
    opacity: 0.65;
    cursor: wait;
  }

  .schedule-table-panel {
    background: rgba(10, 15, 30, 0.68);
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 12px;
    overflow: hidden;
    overflow-anchor: none;
    min-width: 0;
  }

  .schedule-table-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 14px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.32);
  }

  .schedule-table-head h3 {
    margin: 0;
    color: #f8fafc;
    font-size: 1rem;
  }

  .schedule-count {
    color: #94a3b8;
    font-size: 0.7rem;
  }

  .schedule-table-wrapper {
    height: var(--schedule-content-height, 620px);
    max-height: clamp(360px, calc(100vh - 320px), 620px);
    max-height: clamp(360px, calc(100dvh - 320px), 620px);
    overflow: auto;
    overflow-anchor: none;
    overscroll-behavior: contain;
    scrollbar-gutter: stable both-edges;
    scrollbar-width: thin;
    scrollbar-color: rgba(129, 140, 248, 0.62) rgba(15, 23, 42, 0.72);
  }

  .schedule-table-wrapper::-webkit-scrollbar {
    width: 10px;
    height: 10px;
  }

  .schedule-table-wrapper::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.72);
    border-radius: 999px;
  }

  .schedule-table-wrapper::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(129, 140, 248, 0.82), rgba(56, 189, 248, 0.58));
    border: 2px solid rgba(15, 23, 42, 0.72);
    border-radius: 999px;
  }

  .schedule-table-wrapper::-webkit-scrollbar-corner {
    background: rgba(15, 23, 42, 0.72);
  }

  .grid-table {
    display: flex;
    flex-direction: column;
    width: 100%;
    min-width: 1200px;
    background: #0b0f19;
  }

  .grid-thead {
    position: sticky;
    top: 0;
    z-index: 3;
    background: #0b1220;
    border-bottom: 1px solid rgba(71, 85, 105, 0.48);
  }

  .grid-tr {
    display: grid;
    grid-template-columns: minmax(220px, 1.4fr) minmax(180px, 1.1fr) 80px 120px 80px 110px 100px 100px 130px 120px;
    align-items: center;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }

  .grid-tbody .grid-tr {
    height: 76px;
    min-height: 76px;
  }

  .grid-tr.project-group-header {
    display: block;
    cursor: pointer;
    background: rgba(30, 41, 59, 0.5);
    border-bottom: 1px solid rgba(71, 85, 105, 0.2);
    transition: background 0.2s;
  }

  .grid-tr.project-group-header:hover {
    background: rgba(30, 41, 59, 0.8);
  }

  .grid-th {
    color: #64748b;
    font-size: 0.66rem;
    font-weight: 900;
    letter-spacing: 0.04em;
    padding: 10px 8px;
    user-select: none;
    text-align: left;
  }

  .grid-td {
    padding: 10px 8px;
    font-size: 0.78rem;
    color: #cbd5e1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .grid-table .demand-badge-row {
    flex-wrap: nowrap;
  }

  .grid-td.col-action,
  .grid-th.col-action {
    position: sticky;
    right: 0;
    z-index: 4;
    background: #0b0f19;
    box-shadow: -12px 0 20px rgba(2, 6, 23, 0.2);
    border-left: 1px solid rgba(255, 255, 255, 0.04);
    justify-content: center;
    text-align: center;
  }

  .grid-td.col-action {
    background: #0b0f19;
  }

  .grid-tr:hover {
    background: rgba(30, 41, 59, 0.22);
  }

  .grid-tr:hover .grid-td.col-action {
    background: #131b2e;
  }

  .project-cell {
    white-space: normal;
    word-break: break-all;
  }

  .project-group-cell {
    padding: 8px 12px;
    width: 100%;
  }

  .demand-cell {
    width: auto;
    white-space: normal;
    word-break: break-all;
  }

  .project-cell {
    width: auto;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .demand-stack,
  .owner-stack,
  .plan-stack,
  .effort-stack,
  .evidence-stack,
  .subtask-stack,
  .risk-stack,
  .updated-stack {
    display: flex;
    flex-direction: column;
    gap: 5px;
    min-width: 0;
  }

  .demand-stack strong,
  .plan-stack strong,
  .effort-stack strong,
  .evidence-stack strong,
  .updated-stack strong,
  .owner-stack strong,
  .subtask-stack strong {
    color: #f8fafc;
    line-height: 1.32;
    word-break: break-word;
  }

  .plan-stack span,
  .effort-stack span,
  .evidence-stack span,
  .updated-stack span,
  .owner-stack span,
  .subtask-stack small {
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.4;
    word-break: break-word;
  }

  .demand-stack small,
  .risk-stack small {
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
    display: block;
  }

  .bind-badge {
    color: #10b981 !important;
    font-size: 0.65rem;
    font-weight: 700;
  }

  .unbind-badge {
    color: #64748b !important;
    font-size: 0.65rem;
    font-weight: 500;
  }

  .evidence-stack a {
    color: #7dd3fc;
    font-size: 0.68rem;
    font-weight: 800;
    text-decoration: none;
    width: fit-content;
  }

  .evidence-stack a:hover {
    color: #bae6fd;
  }

  .evidence-empty {
    width: fit-content;
    color: #64748b !important;
    background: rgba(51, 65, 85, 0.26);
    border: 1px solid rgba(71, 85, 105, 0.32);
    border-radius: 6px;
    padding: 3px 7px;
  }

  .schedule-id {
    color: #818cf8;
    font-size: 0.66rem;
    font-weight: 900;
    text-decoration: none;
    transition: color 0.15s ease;
  }

  .schedule-id.jira-id-link:hover {
    color: #a5b4fc;
    text-decoration: underline;
  }

  .status-chip {
    width: fit-content;
    border-radius: 5px;
    padding: 2px 6px;
    font-size: 0.64rem;
    font-weight: 900;
    color: #cbd5e1;
    background: rgba(100, 116, 139, 0.14);
    border: 1px solid rgba(100, 116, 139, 0.25);
  }

  .status-chip.status-scheduled { color: #bfdbfe; border-color: rgba(59, 130, 246, 0.36); background: rgba(59, 130, 246, 0.12); }
  .status-chip.status-unscheduled { color: #fed7aa; border-color: rgba(249, 115, 22, 0.36); background: rgba(249, 115, 22, 0.12); }
  .status-chip.status-partial { color: #fde68a; border-color: rgba(234, 179, 8, 0.36); background: rgba(234, 179, 8, 0.12); }
  .status-chip.status-progress { color: #c4b5fd; border-color: rgba(168, 85, 247, 0.34); background: rgba(168, 85, 247, 0.1); }
  .status-chip.status-review { color: #fde68a; border-color: rgba(234, 179, 8, 0.34); background: rgba(234, 179, 8, 0.1); }
  .status-chip.status-done { color: #86efac; border-color: rgba(16, 185, 129, 0.34); background: rgba(16, 185, 129, 0.1); }

  .subtask-meter {
    height: 14px;
    width: 100%;
    min-width: 80px;
    background: rgba(51, 65, 85, 0.72);
    border-radius: 5px;
    overflow: hidden;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .subtask-meter span {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    background: linear-gradient(90deg, #38bdf8, #818cf8);
    border-radius: inherit;
    z-index: 1;
  }

  .subtask-percentage-text {
    position: relative;
    z-index: 2;
    font-size: 0.68rem;
    font-weight: 900;
    color: #ffffff;
    font-family: monospace;
    pointer-events: none;
    line-height: 1;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
  }

  .schedule-risk-pill {
    width: fit-content;
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 0.68rem;
    font-weight: 900;
    border: 1px solid rgba(100, 116, 139, 0.28);
    color: #cbd5e1;
    background: rgba(100, 116, 139, 0.12);
  }

  .risk-overdue { color: #fecaca; background: rgba(239, 68, 68, 0.14); border-color: rgba(248, 113, 113, 0.4); }
  .risk-due_soon { color: #fed7aa; background: rgba(249, 115, 22, 0.12); border-color: rgba(251, 146, 60, 0.36); }
  .risk-stale { color: #ddd6fe; background: rgba(139, 92, 246, 0.13); border-color: rgba(167, 139, 250, 0.36); }
  .risk-unscheduled { color: #bae6fd; background: rgba(14, 165, 233, 0.11); border-color: rgba(56, 189, 248, 0.32); }
  .risk-safe { color: #bbf7d0; background: rgba(16, 185, 129, 0.1); border-color: rgba(74, 222, 128, 0.32); }
  .risk-done { color: #cbd5e1; background: rgba(100, 116, 139, 0.12); border-color: rgba(100, 116, 139, 0.28); }

  .schedule-row-action {
    width: 52px;
    background: rgba(99, 102, 241, 0.14);
    border: 1px solid rgba(129, 140, 248, 0.38);
    color: #c4b5fd;
    border-radius: 7px;
    padding: 6px 0;
    font-size: 0.72rem;
    font-weight: 800;
    cursor: pointer;
    white-space: nowrap;
    text-align: center;
    box-sizing: border-box;
  }

  .schedule-row-action:disabled {
    background: rgba(148, 163, 184, 0.08) !important;
    border: 1px solid rgba(148, 163, 184, 0.18) !important;
    color: #64748b !important;
    cursor: not-allowed !important;
    opacity: 0.65;
  }

  .schedule-row-action:hover {
    background: #4f46e5;
    color: #fff;
  }

  .schedule-actions-cell {
    display: flex;
    flex-direction: row;
    gap: 6px;
    justify-content: center;
    align-items: center;
  }

  .schedule-row-action.is-telemetry {
    background: rgba(14, 165, 233, 0.12);
    border: 1px solid rgba(56, 189, 248, 0.35);
    color: #7dd3fc;
  }

  .schedule-row-action.is-telemetry:hover {
    background: #0284c7;
    color: #fff;
  }

  .schedule-row-muted {
    color: #475569;
    font-size: 0.64rem;
    font-weight: 900;
  }

  .schedule-empty-state {
    padding: 34px;
    text-align: center;
    color: #64748b;
    border-top: 1px solid rgba(51, 65, 85, 0.3);
    font-size: 0.72rem;
  }

  @media (max-width: 1180px) {
    .schedule-summary-grid {
      grid-template-columns: repeat(3, minmax(130px, 1fr));
    }

    .risk-calendar-loading,
    .risk-calendar-grid {
      grid-template-columns: 1fr;
    }

    .risk-calendar-command-row {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .schedule-summary-grid {
      grid-template-columns: repeat(2, minmax(120px, 1fr));
    }

    .risk-calendar-head {
      flex-direction: column;
      align-items: flex-start;
    }

    .risk-calendar-meta {
      justify-content: flex-start;
      flex-wrap: wrap;
    }

    .risk-calendar-command-row {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .bucket-risk-chips {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .schedule-filter-strip,
    .schedule-sort-strip {
      overflow-x: auto;
      justify-content: flex-start;
    }

    .schedule-filter-strip button,
    .schedule-sort-strip button {
      white-space: nowrap;
    }
  }

  /* Kanban Lanes */
  .demand-kanban-board {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 20px;
    align-items: start;
  }

  .kanban-lane {
    background: rgba(10, 15, 30, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 16px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-height: 480px;
  }

  .lane-header {
    display: flex;
    align-items: center;
    gap: 8px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    padding-bottom: 10px;
  }

  .lane-header h4 {
    margin: 0;
    font-size: 0.9rem;
    font-weight: 700;
    color: #e2e8f0;
  }

  .lane-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .bg-orange { background: #f97316; box-shadow: 0 0 8px #f97316; }
  .bg-blue { background: #3b82f6; box-shadow: 0 0 8px #3b82f6; }
  .bg-purple { background: #a855f7; box-shadow: 0 0 8px #a855f7; }
  .bg-green { background: #10b981; box-shadow: 0 0 8px #10b981; }

  .lane-cards {
    display: flex;
    flex-direction: column;
    gap: 12px;
    overflow-y: auto;
    max-height: 520px;
    padding-right: 2px;
  }

  .lane-cards::-webkit-scrollbar {
    width: 4px;
  }
  .lane-cards::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.15);
    border-radius: 2px;
  }

  /* Cards */
  .demand-card {
    background: rgba(30, 41, 59, 0.25);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
    cursor: pointer;
  }

  .demand-card:hover {
    background: rgba(51, 65, 85, 0.3);
    border-color: rgba(99, 102, 241, 0.3);
    transform: translateY(-2px);
  }

  .demand-card:focus-visible {
    outline: 2px solid rgba(129, 140, 248, 0.7);
    outline-offset: 3px;
  }

  .border-orange-dim { border-top: 3px solid #f97316; }
  .border-blue-dim { border-top: 3px solid #3b82f6; }
  .border-purple-dim { border-top: 3px solid #a855f7; }
  .border-green-dim { border-top: 3px solid #10b981; }

  .card-done {
    background: rgba(30, 41, 59, 0.12);
    border-color: rgba(51, 65, 85, 0.2);
  }

  .card-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.65rem;
  }

  .demand-id {
    color: #64748b;
    font-weight: 700;
  }

  .demand-id-button {
    border: 1px solid rgba(99, 102, 241, 0.24);
    background: rgba(99, 102, 241, 0.1);
    color: #a5b4fc;
    border-radius: 5px;
    padding: 3px 7px;
    cursor: pointer;
    text-decoration: none;
    line-height: 1.1;
    font-size: 0.66rem;
  }

  .demand-id-button:hover {
    color: #ffffff;
    border-color: rgba(129, 140, 248, 0.58);
    background: rgba(99, 102, 241, 0.22);
  }

  .jira-id-link {
    color: #7dd3fc;
    border-color: rgba(56, 189, 248, 0.34);
    background: rgba(14, 165, 233, 0.1);
  }

  .assignee-badge {
    display: inline-flex;
    align-items: center;
    max-width: 132px;
    background: rgba(99, 102, 241, 0.12);
    color: #818cf8;
    padding: 2px 6px;
    border-radius: 4px;
    line-height: 1.25;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .demand-card h5 {
    margin: 0;
    font-size: 0.8rem;
    font-weight: 600;
    color: #f1f5f9;
    line-height: 1.4;
  }

  .line-through {
    text-decoration: line-through;
    color: #64748b !important;
  }

  .desc {
    margin: 0;
    font-size: 0.7rem;
    color: #94a3b8;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.4;
  }

  .branch-line {
    font-size: 0.65rem;
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.2);
    color: #94a3b8;
    padding: 3px 8px;
    border-radius: 4px;
    word-break: break-all;
  }

  .card-bottom {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.65rem;
    margin-top: 4px;
  }

  .brain-link-badge {
    color: #7dd3fc;
    background: rgba(14, 165, 233, 0.1);
    border: 1px solid rgba(14, 165, 233, 0.22);
    border-radius: 5px;
    padding: 3px 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 170px;
  }

  .mapped-repo-line {
    color: #64748b;
    background: rgba(15, 23, 42, 0.35);
    border: 1px solid rgba(51, 65, 85, 0.28);
    border-radius: 6px;
    padding: 5px 8px;
    font-size: 0.64rem;
    line-height: 1.35;
    word-break: break-word;
  }

  .action-btn {
    border: none;
    padding: 3px 8px;
    font-size: 0.65rem;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 700;
    transition: all 0.15s;
  }

  .schedule-btn {
    background: rgba(249, 115, 22, 0.15);
    color: #fdba74;
    border: 1px solid rgba(249, 115, 22, 0.3);
  }

  .schedule-btn:hover {
    background: #f97316;
    color: white;
  }

  .edit-sched-btn {
    background: transparent;
    border: none;
    color: #64748b;
    cursor: pointer;
    font-size: 0.8rem;
    padding: 2px;
  }
  
  .edit-sched-btn:hover {
    color: #f1f5f9;
  }

  .status-badge {
    color: #818cf8;
    font-weight: 700;
  }

  .done-tag {
    color: #34d399;
    font-weight: 700;
  }

  .done-date {
    color: #64748b;
  }

  .due-badge {
    padding: 1px 6px;
    border-radius: 4px;
    font-weight: 700;
  }

  .due-safe { background: rgba(16, 185, 129, 0.12); color: #34d399; }
  .due-critical { background: rgba(239, 68, 68, 0.12); color: #f87171; animation: blink 2s infinite; }
  .due-overdue { background: #ef4444; color: white; font-weight: bold; }

  .empty-lane {
    padding: 24px;
    text-align: center;
    color: #475569;
    font-size: 0.7rem;
    border: 1px dashed rgba(51, 65, 85, 0.25);
    border-radius: 8px;
  }

  /* Modal Styles */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(2, 6, 23, 0.7);
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    padding: 24px 16px;
    box-sizing: border-box;
    z-index: 1000;
  }

  .modal-content {
    width: 100%;
    max-width: 520px;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 20px;
    background: #0b1220;
    border: 1px solid rgba(71, 85, 105, 0.68);
    border-radius: 12px;
    padding: 20px;
    box-shadow: 0 24px 72px rgba(2, 6, 23, 0.72), inset 0 1px 0 rgba(255, 255, 255, 0.04);
    animation: zoomIn 0.16s ease-out;
    transform: translateZ(0);
    backface-visibility: hidden;
    max-height: calc(100dvh - 48px);
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-width: thin;
    scrollbar-color: rgba(129, 140, 248, 0.3) transparent;
  }

  .modal-content::-webkit-scrollbar {
    width: 6px;
    height: 6px;
  }

  .modal-content::-webkit-scrollbar-thumb {
    background: rgba(129, 140, 248, 0.3);
    border-radius: 3px;
  }

  .modal-content::-webkit-scrollbar-track {
    background: transparent;
  }

  .demand-create-modal {
    max-width: 560px;
    overflow: visible !important;
  }

  .schedule-modal {
    max-width: 580px;
    overflow: visible;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    padding-bottom: 12px;
  }

  .modal-header h3 {
    margin: 0;
    font-size: 1.1rem;
    color: #f8fafc;
  }

  .close-btn {
    background: transparent;
    border: none;
    color: #64748b;
    font-size: 1.5rem;
    cursor: pointer;
    line-height: 1;
  }

  .close-btn:hover {
    color: #f1f5f9;
  }

  .form-body {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 0.75rem;
    color: #94a3b8;
    font-weight: 600;
  }

  .form-group input[type="text"],
  .form-group textarea {
    width: 100%;
    box-sizing: border-box;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 8px;
    padding: 10px 12px;
    color: #e2e8f0;
    font-size: 0.8rem;
    outline: none;
    transition: border-color 0.18s ease, box-shadow 0.18s ease, background-color 0.18s ease;
    transform: translateZ(0);
  }

  .form-group textarea {
    min-width: 0;
    max-width: 100%;
    resize: vertical;
  }

  .form-group input:focus,
  .form-group textarea:focus {
    border-color: #6366f1;
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.15);
  }

  /* Creator Meta Info */
  .creator-meta {
    font-size: 0.65rem;
    color: #64748b;
    margin-top: 2px;
    margin-bottom: 6px;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .field-hint {
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.45;
  }

  /* Card Actions & Icon Buttons */
  .card-actions {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .icon-action-btn {
    background: transparent;
    border: none;
    font-size: 0.72rem;
    cursor: pointer;
    opacity: 0.5;
    transition: opacity 0.15s, transform 0.1s;
    padding: 2px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .detail-action-btn {
    color: #94a3b8;
    border: 1px solid rgba(71, 85, 105, 0.38);
    border-radius: 5px;
    padding: 2px 6px;
    font-size: 0.64rem;
    opacity: 0.72;
  }

  .detail-action-btn:hover {
    color: #e2e8f0;
    border-color: rgba(129, 140, 248, 0.45);
  }

  .icon-action-btn:hover {
    opacity: 1;
    transform: scale(1.15);
  }

  .icon-action-btn:active {
    transform: scale(0.95);
  }

  .date-input-shell {
    position: relative;
    display: flex;
    align-items: center;
    min-height: 40px;
    z-index: 5;
    width: 100%;
    box-sizing: border-box;
  }

  .date-input-display {
    position: relative;
    width: 100%;
    min-height: 40px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    box-sizing: border-box;
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.82), rgba(15, 23, 42, 0.58));
    border: 1px solid rgba(100, 116, 139, 0.45);
    border-radius: 8px;
    padding: 10px 12px;
    color: #e2e8f0;
    font-size: 0.8rem;
    line-height: 1;
    text-align: left;
    cursor: pointer;
    outline: none;
    appearance: none;
    font-family: inherit;
    transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
  }

  .date-input-value {
    color: #64748b;
    font-variant-numeric: tabular-nums;
  }

  .date-input-display.has-value .date-input-value {
    color: #e2e8f0;
  }

  .date-input-icon {
    position: relative;
    flex: 0 0 auto;
    box-sizing: border-box;
    width: 16px;
    height: 16px;
    color: #818cf8;
    border: 1px solid rgba(129, 140, 248, 0.7);
    border-radius: 4px;
    pointer-events: none;
    background: rgba(30, 41, 59, 0.5);
    overflow: hidden;
  }

  .date-input-icon::before {
    content: "";
    position: absolute;
    left: 2px;
    right: 2px;
    top: 4px;
    border-top: 1px solid currentColor;
  }

  .date-input-icon::after {
    content: "";
    position: absolute;
    left: 4px;
    top: 8px;
    width: 2px;
    height: 2px;
    background: currentColor;
    box-shadow: 5px 0 0 currentColor, 0 4px 0 currentColor, 5px 4px 0 currentColor;
    border-radius: 1px;
  }

  .date-input-shell:hover .date-input-display {
    border-color: rgba(129, 140, 248, 0.65);
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(15, 23, 42, 0.68));
  }

  .date-input-shell:focus-within .date-input-display {
    border-color: #818cf8;
    box-shadow: 0 0 0 2px rgba(129, 140, 248, 0.18);
  }

  .date-picker-panel {
    position: absolute;
    top: calc(100% + 8px);
    left: 0;
    width: 292px;
    max-width: min(292px, calc(100vw - 48px));
    box-sizing: border-box;
    background: #0b1220;
    border: 1px solid rgba(71, 85, 105, 0.76);
    border-radius: 12px;
    padding: 12px;
    box-shadow: 0 20px 48px rgba(2, 6, 23, 0.72);
    z-index: 1400;
  }

  .schedule-modal .date-picker-panel {
    position: absolute;
    width: min(292px, 100%);
    max-width: 100%;
    margin-top: 0;
    box-shadow: 0 16px 36px rgba(2, 6, 23, 0.46);
  }

  .date-picker-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 10px;
  }

  .date-picker-head strong {
    color: #e2e8f0;
    font-size: 0.86rem;
  }

  .date-picker-head button,
  .date-picker-foot button,
  .date-cell {
    border: 1px solid transparent;
    background: transparent;
    color: #94a3b8;
    cursor: pointer;
    font: inherit;
  }

  .date-picker-head button {
    width: 30px;
    height: 30px;
    border-radius: 8px;
    font-size: 1.15rem;
    line-height: 1;
    background: rgba(15, 23, 42, 0.78);
    border-color: rgba(51, 65, 85, 0.7);
  }

  .date-picker-head button:hover {
    color: #e2e8f0;
    border-color: rgba(129, 140, 248, 0.55);
  }

  .date-week-grid,
  .date-grid {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 4px;
  }

  .date-week-grid {
    margin-bottom: 6px;
  }

  .date-week-grid span {
    color: #475569;
    font-size: 0.64rem;
    text-align: center;
    font-weight: 800;
  }

  .date-cell {
    height: 32px;
    border-radius: 8px;
    font-size: 0.76rem;
    font-variant-numeric: tabular-nums;
  }

  .date-cell:hover {
    color: #f8fafc;
    background: rgba(99, 102, 241, 0.16);
    border-color: rgba(129, 140, 248, 0.42);
  }

  .date-cell.muted {
    color: #334155;
  }

  .date-cell.today {
    border-color: rgba(14, 165, 233, 0.55);
    color: #7dd3fc;
  }

  .date-cell.selected {
    background: #4f46e5;
    color: #ffffff;
    border-color: rgba(129, 140, 248, 0.9);
  }

  .date-picker-foot {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    margin-top: 10px;
    border-top: 1px solid rgba(51, 65, 85, 0.48);
    padding-top: 10px;
  }

  .date-picker-foot button {
    border-color: rgba(51, 65, 85, 0.62);
    background: rgba(15, 23, 42, 0.58);
    border-radius: 7px;
    padding: 6px 9px;
    font-size: 0.72rem;
  }

  .date-picker-foot button:hover {
    color: #f8fafc;
    border-color: rgba(129, 140, 248, 0.55);
  }

  .schedule-estimate-panel {
    position: relative;
    border-top: 1px solid rgba(51, 65, 85, 0.38);
    border-bottom: 1px solid rgba(51, 65, 85, 0.28);
    padding: 8px 0;
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 10px;
    align-items: center;
  }

  .estimate-panel-head {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    flex-direction: row;
    gap: 10px;
    min-width: 0;
  }

  .estimate-panel-head > div {
    display: flex;
    align-items: center;
    flex-direction: row;
    gap: 8px;
    min-width: 0;
  }

  .estimate-panel-head strong {
    color: #f8fafc;
    font-size: 0.8rem;
    white-space: nowrap;
  }

  .estimate-actions {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    flex-wrap: nowrap;
    gap: 8px;
  }

  .estimate-ai-btn {
    flex: 0 0 auto;
    height: 32px;
    border: 1px solid rgba(56, 189, 248, 0.38);
    background: rgba(14, 165, 233, 0.12);
    color: #7dd3fc;
    border-radius: 8px;
    padding: 0 12px;
    cursor: pointer;
    font-size: 0.68rem;
    font-weight: 900;
    transition: background 0.16s ease, border-color 0.16s ease, transform 0.1s ease;
  }

  .estimate-ai-btn:hover:not(:disabled) {
    background: rgba(14, 165, 233, 0.2);
    border-color: rgba(125, 211, 252, 0.58);
  }

  .estimate-ai-btn:active:not(:disabled) {
    transform: translateY(1px);
  }

  .estimate-ai-btn:disabled,
  .estimate-ai-btn.is-loading {
    opacity: 0.66;
    cursor: wait;
  }

  .estimate-clear-btn {
    flex: 0 0 auto;
    height: 32px;
    border: 1px solid rgba(71, 85, 105, 0.52);
    background: rgba(15, 23, 42, 0.62);
    color: #94a3b8;
    border-radius: 8px;
    padding: 0 10px;
    cursor: pointer;
    font-size: 0.68rem;
    font-weight: 900;
    transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
  }

  .estimate-clear-btn:hover {
    background: rgba(51, 65, 85, 0.48);
    border-color: rgba(100, 116, 139, 0.7);
    color: #e2e8f0;
  }

  .estimate-manual-grid {
    display: grid;
    grid-template-columns: minmax(76px, 0.62fr) minmax(104px, 0.76fr);
    gap: 8px;
    align-items: end;
  }

  .estimate-manual-grid.has-days {
    grid-template-columns: minmax(76px, 0.62fr) minmax(76px, 0.62fr) minmax(104px, 0.76fr);
  }

  .estimate-field,
  .estimate-reference-field,
  .estimate-difficulty-field {
    min-width: 0;
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 6px;
  }

  .estimate-difficulty-field {
    position: relative;
  }

  .estimate-field span,
  .estimate-reference-field span,
  .estimate-difficulty-field > span:first-child {
    display: inline-flex;
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 800;
    white-space: nowrap;
  }

  .estimate-field input,
  .estimate-reference-field strong {
    flex: 1 1 auto;
    min-width: 0;
    width: 100%;
    height: 32px;
    box-sizing: border-box;
    border: 1px solid rgba(71, 85, 105, 0.58);
    border-radius: 8px;
    outline: none;
    background: rgba(2, 6, 23, 0.18);
    color: #e2e8f0;
    font: inherit;
    font-size: 0.82rem;
    font-weight: 800;
    font-variant-numeric: tabular-nums;
    text-align: left;
    transition: border-color 0.16s ease, background 0.16s ease, box-shadow 0.16s ease;
  }

  .estimate-reference-field strong {
    display: inline-flex;
    align-items: center;
    padding: 0 8px;
    color: #cbd5e1;
    font-weight: 900;
    background: rgba(15, 23, 42, 0.38);
  }

  .estimate-field input {
    padding: 0 8px;
  }

  .estimate-field input:focus {
    border-color: rgba(129, 140, 248, 0.78);
    background-color: rgba(15, 23, 42, 0.66);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.14);
  }

  .difficulty-select-control {
    min-width: 0;
    flex: 1 1 auto;
  }

  .difficulty-select-control :global(.select-group) {
    gap: 0;
  }

  .difficulty-select-control :global(.select-trigger) {
    min-height: 32px;
    border-radius: 8px;
    font-size: 0.82rem;
    font-weight: 800;
  }

  .estimate-field input::placeholder {
    color: #475569;
  }

  .estimate-field input::-webkit-outer-spin-button,
  .estimate-field input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  .estimate-field input[type="number"] {
    appearance: textfield;
    -moz-appearance: textfield;
  }

  .estimate-error {
    grid-column: 1 / -1;
    min-width: 0;
    width: 100%;
    max-width: 100%;
    box-sizing: border-box;
    margin: 0;
    display: flex;
    align-items: flex-start;
    gap: 8px;
    font-size: 0.7rem;
    line-height: 1.5;
    color: #fecaca;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(248, 113, 113, 0.28);
    border-radius: 8px;
    padding: 8px 10px;
    overflow-wrap: anywhere;
    word-break: break-word;
    white-space: pre-wrap;
  }

  .estimate-error-icon {
    flex: 0 0 16px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    margin-top: 1px;
    border-radius: 50%;
    background: rgba(239, 68, 68, 0.14);
    color: #ef4444;
    font-size: 0.62rem;
    font-weight: 900;
    line-height: 1;
  }

  .group-lock-field {
    width: 100%;
    box-sizing: border-box;
    background: rgba(15, 23, 42, 0.56);
    border: 1px solid rgba(71, 85, 105, 0.46);
    border-radius: 8px;
    padding: 10px 12px;
    color: #cbd5e1;
    font-size: 0.78rem;
    overflow: hidden;
  }

  .group-lock-field span {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .detail-modal {
    max-width: 960px;
  }

  .detail-body {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .detail-title-block {
    border-bottom: 1px solid rgba(51, 65, 85, 0.36);
    padding-bottom: 14px;
  }

  .detail-id {
    display: inline-flex;
    width: fit-content;
    color: #7dd3fc;
    border: 1px solid rgba(56, 189, 248, 0.28);
    background: rgba(14, 165, 233, 0.08);
    border-radius: 5px;
    padding: 3px 7px;
    font-size: 0.66rem;
    font-weight: 900;
    margin-bottom: 8px;
    text-decoration: none;
    transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
  }

  .detail-id.jira-id-link:hover {
    background: rgba(14, 165, 233, 0.16);
    border-color: rgba(56, 189, 248, 0.45);
    color: #38bdf8;
  }

  .detail-title-block strong {
    display: block;
    color: #f8fafc;
    font-size: 1rem;
    line-height: 1.45;
  }

  .detail-title-block p {
    margin: 8px 0 0;
    color: #94a3b8;
    font-size: 0.82rem;
    line-height: 1.55;
  }

  .detail-facts-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .detail-facts-grid div {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.38);
    background: rgba(15, 23, 42, 0.44);
    border-radius: 8px;
    padding: 9px 10px;
  }

  .detail-facts-grid span,
  .detail-section-title span {
    display: block;
    color: #64748b;
    font-size: 0.64rem;
    font-weight: 800;
    margin-bottom: 4px;
  }

  .detail-facts-grid strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.78rem;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .detail-link-row {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .detail-link-row a,
  .detail-link-row button {
    border: 1px solid rgba(129, 140, 248, 0.35);
    background: rgba(99, 102, 241, 0.12);
    color: #c4b5fd;
    border-radius: 7px;
    padding: 7px 10px;
    font-size: 0.76rem;
    font-weight: 800;
    text-decoration: none;
    cursor: pointer;
    font-family: inherit;
  }

  .detail-link-row a:hover,
  .detail-link-row button:hover {
    background: rgba(99, 102, 241, 0.24);
    color: #ffffff;
  }

  .detail-subtasks {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .detail-section-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
  }

  .detail-section-title span {
    margin-bottom: 0;
  }

  .detail-section-title strong {
    color: #cbd5e1;
    font-size: 0.78rem;
  }

  .detail-subtask-list {
    max-height: 220px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.25) transparent;
  }

  .detail-subtask-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
    border: 1px solid rgba(51, 65, 85, 0.32);
    border-radius: 7px;
    padding: 8px 10px;
    background: rgba(2, 6, 23, 0.24);
  }

  .detail-subtask-row strong {
    color: #e2e8f0;
    font-size: 0.76rem;
    line-height: 1.35;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .detail-subtask-row em {
    color: #64748b;
    font-size: 0.68rem;
    font-style: normal;
    white-space: nowrap;
  }

  .detail-empty {
    border: 1px dashed rgba(51, 65, 85, 0.42);
    border-radius: 8px;
    color: #64748b;
    padding: 14px;
    text-align: center;
    font-size: 0.78rem;
  }

  @media (max-width: 760px) {
    .estimate-panel-head {
      align-items: flex-start;
      flex-direction: column;
      gap: 8px;
    }

    .estimate-actions {
      justify-content: flex-start;
    }

    .schedule-estimate-panel {
      grid-template-columns: 1fr;
    }

    .estimate-manual-grid {
      grid-template-columns: 1fr;
    }

    .detail-facts-grid,
    .detail-subtask-row {
      grid-template-columns: 1fr;
    }
  }

  .help-text {
    font-size: 0.65rem;
    color: #f59e0b;
    line-height: 1.4;
  }

  .demand-brief {
    background: rgba(15, 23, 42, 0.4);
    padding: 8px 12px;
    border-radius: 6px;
    border: 1px solid rgba(51, 65, 85, 0.2);
    font-size: 0.75rem;
    color: #cbd5e1;
    margin: 0;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    border-top: 1px solid rgba(51, 65, 85, 0.2);
    padding-top: 16px;
  }

  .modal-footer button {
    border: none;
    padding: 8px 20px;
    font-size: 0.8rem;
    font-weight: 700;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .cancel-btn {
    background: rgba(30, 41, 59, 0.5);
    color: #94a3b8;
    border: 1px solid rgba(51, 65, 85, 0.3) !important;
  }

  .cancel-btn:hover {
    background: rgba(51, 65, 85, 0.5);
    color: #ffffff;
  }

  .submit-btn {
    background: #6366f1;
    color: white;
    box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  }

  .submit-btn:hover {
    background: #4f46e5;
    box-shadow: 0 6px 16px rgba(99, 102, 241, 0.4);
  }

  .state-msg {
    padding: 80px 0;
    text-align: center;
    color: #64748b;
    font-size: 0.85rem;
  }

  .error-msg {
    color: #f87171;
  }

  @keyframes zoomIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  /* Custom Dropdown Styling */
  .custom-dropdown-container {
    position: relative;
    width: 100%;
  }

  .dropdown-trigger {
    width: 100%;
    box-sizing: border-box;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 8px;
    padding: 10px 12px;
    color: #e2e8f0;
    font-size: 0.8rem;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-family: inherit;
    text-align: left;
    transition: border-color 0.18s ease, background-color 0.18s ease, box-shadow 0.18s ease;
  }

  .dropdown-trigger span:first-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dropdown-trigger:hover {
    border-color: rgba(99, 102, 241, 0.4);
    background: rgba(15, 23, 42, 0.8);
  }

  .dropdown-trigger .arrow-icon {
    font-size: 0.6rem;
    color: #64748b;
    transition: transform 0.2s;
  }

  .dropdown-trigger .arrow-icon.open {
    transform: rotate(180deg);
  }

  .dropdown-options-list {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 100%;
    max-height: 200px;
    overflow-y: auto;
    background: rgba(15, 23, 42, 0.95) !important;
    border: 1px solid rgba(99, 102, 241, 0.3) !important;
    border-radius: 8px;
    z-index: 1100;
    padding: 4px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5);
  }

  .dropdown-options-list::-webkit-scrollbar {
    width: 6px;
  }

  .dropdown-options-list::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.3);
    border-radius: 3px;
  }

  .dropdown-option-item {
    width: 100%;
    border: none;
    background: transparent;
    text-align: left;
    padding: 8px 12px;
    color: #cbd5e1;
    font-size: 0.8rem;
    font-family: inherit;
    cursor: pointer;
    border-radius: 6px;
    transition: background-color 0.15s ease, color 0.15s ease;
  }

  .dropdown-option-item:hover {
    background: rgba(99, 102, 241, 0.2);
    color: #ffffff;
  }

  .dropdown-option-item.selected {
    background: rgba(99, 102, 241, 0.4);
    color: #ffffff;
    font-weight: 600;
  }

  .dropdown-empty {
    padding: 10px 12px;
    color: #64748b;
    font-size: 0.78rem;
    text-align: center;
  }

  /* Confirm Modal Specifics */
  .confirm-modal {
    max-width: 440px;
    gap: 0;
    padding: 0;
    overflow: hidden;
    background: #0b1220 !important;
    border: 1px solid rgba(71, 85, 105, 0.65) !important;
    border-radius: 12px;
    box-shadow: 0 24px 70px rgba(2, 6, 23, 0.62), inset 0 1px 0 rgba(255, 255, 255, 0.04);
  }

  .confirm-modal-delete {
    border-top: 3px solid #dc2626 !important;
  }

  .confirm-modal-archive {
    border-top: 3px solid #4f46e5 !important;
  }

  .confirm-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 18px;
    padding: 20px 22px 16px;
    border-bottom: 1px solid rgba(71, 85, 105, 0.36);
  }

  .confirm-header h3 {
    margin: 5px 0 0;
    color: #f8fafc;
    font-size: 1.05rem;
    line-height: 1.2;
  }

  .confirm-kicker {
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 800;
    letter-spacing: 0.08em;
  }

  .confirm-close {
    margin-top: -4px;
  }

  .confirm-body {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px 22px 20px;
  }

  .confirm-id-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    background: rgba(15, 23, 42, 0.72);
    border: 1px solid rgba(51, 65, 85, 0.7);
    border-radius: 8px;
    padding: 10px 12px;
  }

  .confirm-id-label {
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 800;
    letter-spacing: 0.06em;
  }

  .confirm-id-value {
    color: #cbd5e1;
    font-size: 0.76rem;
    font-weight: 800;
  }

  .confirm-message {
    font-size: 0.84rem;
    color: #cbd5e1;
    line-height: 1.55;
    margin: 0;
  }

  .confirm-footer {
    background: rgba(2, 6, 23, 0.26);
    border-top: 1px solid rgba(71, 85, 105, 0.36);
    padding: 14px 22px 18px;
  }

  .confirm-btn-delete {
    background: #dc2626 !important;
    box-shadow: none !important;
    color: white !important;
    border: 1px solid rgba(248, 113, 113, 0.35) !important;
  }

  .confirm-btn-delete:hover {
    background: #b91c1c !important;
  }

  .confirm-btn-archive {
    background: #4f46e5 !important;
    box-shadow: none !important;
    color: white !important;
    border: 1px solid rgba(129, 140, 248, 0.45) !important;
  }

  .confirm-btn-archive:hover {
    background: #4338ca !important;
  }

  .confirm-modal .cancel-btn {
    background: rgba(15, 23, 42, 0.55) !important;
    border: 1px solid rgba(71, 85, 105, 0.65) !important;
    color: #cbd5e1 !important;
    box-shadow: none !important;
    transition: all 0.2s ease;
  }

  .confirm-modal .cancel-btn:hover {
    background: rgba(30, 41, 59, 0.88) !important;
    border-color: rgba(100, 116, 139, 0.8) !important;
    color: #ffffff !important;
  }

  /* Subtasks tracker on demand cards */
  .deconstruct-subtasks-box {
    margin-top: 12px;
    padding: 8px 10px;
    background: rgba(2, 6, 23, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    box-sizing: border-box;
  }

  .subtask-progress-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.7rem;
    color: #94a3b8;
    cursor: pointer;
    user-select: none;
    gap: 8px;
  }

  .subtask-progress-row:hover {
    color: #cbd5e1;
  }

  .subtask-bar {
    flex-grow: 1;
    height: 4px;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 2px;
    overflow: hidden;
    position: relative;
  }

  .subtask-bar-fill {
    height: 100%;
    background: linear-gradient(90deg, #6366f1, #818cf8);
    border-radius: 2px;
  }

  .expand-arrow {
    font-size: 0.55rem;
    transition: transform 0.2s;
  }

  .subtasks-list-details {
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px dashed rgba(255, 255, 255, 0.06);
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 140px;
    overflow-y: auto;
  }

  .subtask-detail-item {
    display: flex;
    align-items: center;
    font-size: 0.65rem;
    color: #64748b;
    gap: 6px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .status-dot.backlog {
    background: #64748b;
    box-shadow: 0 0 4px rgba(100, 116, 139, 0.4);
  }

  .status-dot.progress {
    background: #a855f7;
    box-shadow: 0 0 6px rgba(168, 85, 247, 0.5);
  }

  .status-dot.review {
    background: #eab308;
    box-shadow: 0 0 6px rgba(234, 179, 8, 0.5);
  }

  .status-dot.done {
    background: #10b981;
    box-shadow: 0 0 6px rgba(16, 185, 129, 0.5);
  }

  .sub-repo {
    color: #818cf8;
    font-weight: 700;
    flex-shrink: 0;
  }

  .sub-title {
    color: #cbd5e1;
    flex-grow: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    text-align: left;
  }

  .sub-assignee {
    color: #64748b;
    flex-shrink: 0;
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
  .combobox-trigger-wrapper {
    position: relative;
    width: 100%;
  }
  .combobox-trigger-wrapper .dropdown-trigger-input {
    width: 100%;
    box-sizing: border-box;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 8px;
    padding: 10px 30px 10px 12px;
    color: #e2e8f0;
    font-size: 0.8rem;
    cursor: text;
    font-family: inherit;
    text-align: left;
    transition: border-color 0.18s ease, background-color 0.18s ease, box-shadow 0.18s ease;
    outline: none;
  }
  .combobox-trigger-wrapper .schedule-trigger-override {
    height: 38px;
    background: rgba(15, 23, 42, 0.72);
    border: 1px solid rgba(71, 85, 105, 0.68);
    font-size: 0.76rem;
    font-weight: 700;
    color: #cbd5e1;
  }
  .combobox-trigger-wrapper .dropdown-trigger-input::placeholder {
    color: #cbd5e1;
    opacity: 1;
  }
  .combobox-trigger-wrapper .dropdown-trigger-input:hover {
    border-color: rgba(99, 102, 241, 0.4);
    background: rgba(15, 23, 42, 0.8);
  }
  .combobox-trigger-wrapper .dropdown-trigger-input:focus {
    border-color: rgba(99, 102, 241, 0.5);
    background: rgba(15, 23, 42, 0.8);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.15);
  }
  .combobox-trigger-wrapper .arrow-icon {
    position: absolute;
    right: 12px;
    top: 50%;
    transform: translateY(-50%);
    font-size: 0.6rem;
    color: #64748b;
    transition: transform 0.2s;
    pointer-events: none;
  }
  .combobox-trigger-wrapper .arrow-icon.open {
    transform: translateY(-50%) rotate(180deg);
  }

  .demand-badge-row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
    margin-bottom: 2px;
  }
  .type-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 600;
    line-height: 1;
  }
  .type-badge.demand {
    background: rgba(59, 130, 246, 0.12);
    color: #93c5fd;
    border: 1px solid rgba(59, 130, 246, 0.25);
  }
  .type-badge.bug {
    background: rgba(239, 68, 68, 0.12);
    color: #fca5a5;
    border: 1px solid rgba(239, 68, 68, 0.25);
  }
  .assignee-disabled-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    background: rgba(30, 41, 59, 0.5);
    border: 1px dashed rgba(148, 163, 184, 0.2);
    border-radius: 6px;
    padding: 8px 12px;
    color: #94a3b8;
    font-size: 0.8rem;
  }
  .assignee-disabled-tip {
    font-size: 0.7rem;
    color: #64748b;
  }

  /* 大脑健康遥测控制台样式 */
  .brain-console-wrapper {
    margin-bottom: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .brain-toggle-btn {
    align-self: flex-start;
    background: rgba(99, 102, 241, 0.12);
    border: 1px solid rgba(129, 140, 248, 0.3);
    color: #c7d2fe;
    font-size: 0.74rem;
    font-weight: 800;
    padding: 6px 14px;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .brain-toggle-btn:hover {
    background: rgba(99, 102, 241, 0.24);
    border-color: rgba(129, 140, 248, 0.5);
    color: #ffffff;
  }
  .brain-toggle-btn.is-active {
    background: rgba(99, 102, 241, 0.38);
    border-color: rgba(129, 140, 248, 0.6);
  }
  .brain-console-card {
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.45);
    border-radius: 12px;
    padding: 18px;
    box-shadow: 0 8px 32px rgba(2, 6, 23, 0.4);
  }
  .brain-loading, .brain-empty {
    text-align: center;
    color: #64748b;
    padding: 20px 0;
    font-size: 0.8rem;
  }
  .brain-console-layout {
    display: grid;
    grid-template-columns: 180px 2.4fr 1.6fr;
    gap: 20px;
  }
  .console-section-label {
    display: block;
    font-size: 0.68rem;
    font-weight: 800;
    color: #64748b;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    margin-bottom: 10px;
  }
  
  /* 项目选择列表 */
  .brain-project-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    border-right: 1px solid rgba(51, 65, 85, 0.3);
    padding-right: 15px;
  }
  .brain-proj-btn {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: rgba(30, 41, 59, 0.3);
    border: 1px solid rgba(148, 163, 184, 0.12);
    border-radius: 6px;
    padding: 8px 12px;
    color: #e2e8f0;
    cursor: pointer;
    transition: all 0.16s ease;
    text-align: left;
  }
  .brain-proj-btn:hover {
    background: rgba(30, 41, 59, 0.6);
    border-color: rgba(99, 102, 241, 0.3);
  }
  .brain-proj-btn.active {
    background: rgba(99, 102, 241, 0.18);
    border-color: rgba(99, 102, 241, 0.5);
    box-shadow: 0 0 10px rgba(99, 102, 241, 0.15);
  }
  .proj-key {
    font-size: 0.8rem;
    font-weight: 800;
  }
  .proj-score {
    font-size: 0.76rem;
    font-weight: 800;
  }

  /* 环形分数与诊断 */
  .brain-health-core {
    display: grid;
    grid-template-columns: 100px minmax(0, 1fr);
    gap: 20px;
    align-items: center;
    border-right: 1px solid rgba(51, 65, 85, 0.3);
    padding-right: 20px;
  }
  .radial-score-box {
    flex-shrink: 0;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  .radial-ring {
    position: relative;
    width: 96px;
    height: 96px;
    border-radius: 50%;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    border: 3px solid rgba(51, 65, 85, 0.4);
    background: rgba(15, 23, 42, 0.6);
    box-shadow: 0 0 20px rgba(0, 0, 0, 0.6), inset 0 2px 8px rgba(0, 0, 0, 0.8);
    transition: all 0.3s ease;
  }
  .radial-ring.score-excellent {
    border-color: rgba(16, 185, 129, 0.45);
    background: radial-gradient(circle, rgba(16, 185, 129, 0.12) 0%, rgba(10, 15, 30, 0.6) 100%);
    box-shadow: 0 0 25px rgba(16, 185, 129, 0.18), inset 0 2px 8px rgba(16, 185, 129, 0.08);
  }
  .radial-ring.score-excellent .radial-score {
    color: #10b981;
    text-shadow: 0 0 10px rgba(16, 185, 129, 0.4);
  }
  .radial-ring.score-good {
    border-color: rgba(245, 158, 11, 0.45);
    background: radial-gradient(circle, rgba(245, 158, 11, 0.12) 0%, rgba(10, 15, 30, 0.6) 100%);
    box-shadow: 0 0 25px rgba(245, 158, 11, 0.18), inset 0 2px 8px rgba(245, 158, 11, 0.08);
  }
  .radial-ring.score-good .radial-score {
    color: #f59e0b;
    text-shadow: 0 0 10px rgba(245, 158, 11, 0.4);
  }
  .radial-ring.score-risk {
    border-color: rgba(239, 68, 68, 0.45);
    background: radial-gradient(circle, rgba(239, 68, 68, 0.12) 0%, rgba(10, 15, 30, 0.6) 100%);
    box-shadow: 0 0 25px rgba(239, 68, 68, 0.18), inset 0 2px 8px rgba(239, 68, 68, 0.08);
  }
  .radial-ring.score-risk .radial-score {
    color: #ef4444;
    text-shadow: 0 0 10px rgba(239, 68, 68, 0.4);
  }
  
  .radial-score {
    font-size: 1.8rem;
    font-weight: 900;
    line-height: 1;
  }
  .radial-label {
    font-size: 0.58rem;
    color: #64748b;
    margin-top: 4px;
    letter-spacing: 0.05em;
  }

  .diagnostic-bubble {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;
    background: rgba(99, 102, 241, 0.03);
    border: 1px solid rgba(99, 102, 241, 0.15);
    border-radius: 8px;
    padding: 14px 18px;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
  }
  .diagnostic-bubble.score-excellent {
    background: rgba(16, 185, 129, 0.03);
    border-color: rgba(16, 185, 129, 0.15);
  }
  .diagnostic-bubble.score-good {
    background: rgba(245, 158, 11, 0.03);
    border-color: rgba(245, 158, 11, 0.15);
  }
  .diagnostic-bubble.score-risk {
    background: rgba(239, 68, 68, 0.03);
    border-color: rgba(239, 68, 68, 0.15);
  }

  .bubble-title {
    font-size: 0.72rem;
    font-weight: 800;
    color: #c7d2fe;
  }
  .diagnostic-bubble p {
    margin: 0;
    font-size: 0.74rem;
    color: #94a3b8;
    line-height: 1.5;
    white-space: pre-wrap;
  }

  /* 维度打分刻度条 - 2x2双列网格 */
  .brain-dimension-board {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px 20px;
  }
  .brain-dimension-board .console-section-label {
    grid-column: span 2;
    margin-bottom: 4px;
  }
  .dimension-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .dim-label {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.74rem;
    color: #e2e8f0;
  }
  .dim-label strong {
    color: #f1f5f9;
  }
  .dim-bar-bg {
    width: 100%;
    height: 6px;
    background: rgba(15, 23, 42, 0.8);
    border-radius: 99px;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.6);
    overflow: hidden;
  }
  .dim-bar-fill {
    display: block;
    height: 100%;
    border-radius: 99px;
  }
  .dim-bar-fill.is-blue { 
    background: linear-gradient(90deg, #0ea5e9, #38bdf8); 
    box-shadow: 0 0 8px rgba(56, 189, 248, 0.4);
  }
  .dim-bar-fill.is-emerald { 
    background: linear-gradient(90deg, #059669, #34d399); 
    box-shadow: 0 0 8px rgba(52, 211, 153, 0.4);
  }
  .dim-bar-fill.is-amber { 
    background: linear-gradient(90deg, #d97706, #fbbf24); 
    box-shadow: 0 0 8px rgba(251, 191, 36, 0.4);
  }
  .dim-bar-fill.is-rose { 
    background: linear-gradient(90deg, #dc2626, #f87171); 
    box-shadow: 0 0 8px rgba(248, 113, 113, 0.4);
  }

  /* 评分样式分类色值 */
  .score-excellent {
    color: #34d399 !important;
  }
  .score-good {
    color: #fbbf24 !important;
  }
  .score-risk {
    color: #f87171 !important;
  }

  .demand-dashboard {
    --schedule-accent: var(--wa-success, #72e6b4);
    --schedule-accent-ink: #031510;
    --schedule-panel: rgba(10, 16, 22, 0.72);
    --schedule-panel-strong: rgba(13, 20, 27, 0.86);
    --schedule-panel-soft: rgba(244, 251, 255, 0.052);
    --schedule-border: var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    --schedule-border-strong: var(--wa-border-strong, rgba(203, 234, 244, 0.28));
    --schedule-text: var(--wa-text-main, #d8e5ec);
    --schedule-strong: var(--wa-text-strong, #f4fbff);
    --schedule-muted: var(--wa-text-muted, #90a8b5);
    --schedule-subtle: var(--wa-text-subtle, #5f7582);
    gap: 12px;
    margin-top: 0;
    color: var(--schedule-text);
  }

  .dashboard-header {
    min-height: 58px;
    padding: 10px 12px;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-xl, 8px);
    background: rgba(255, 255, 255, 0.82);
    box-shadow: var(--wa-shadow-sm, 0 7px 16px rgba(30, 46, 64, 0.055));
    backdrop-filter: blur(16px) saturate(124%);
    -webkit-backdrop-filter: blur(16px) saturate(124%);
  }

  .dashboard-header h2 {
    color: var(--schedule-strong);
    background: none;
    -webkit-text-fill-color: currentColor;
    font-size: 1.08rem;
    line-height: 1.12;
    letter-spacing: 0;
  }

  .eyebrow {
    color: var(--schedule-subtle);
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
    font-size: 0.68rem;
    font-weight: 820;
    letter-spacing: 0;
    text-transform: none;
  }

  .add-demand-btn {
    min-height: 36px;
    border: 1px solid color-mix(in srgb, var(--schedule-accent) 54%, transparent);
    background: var(--schedule-accent);
    color: var(--schedule-accent-ink);
    border-radius: var(--wa-radius-md, 8px);
    box-shadow: none;
    padding: 0 15px;
  }

  .add-demand-btn:hover {
    background: color-mix(in srgb, var(--schedule-accent) 88%, var(--schedule-strong));
    box-shadow: 0 12px 28px rgba(1, 9, 14, 0.28);
  }

  .schedule-control-panel,
  .schedule-table-panel,
  .schedule-risk-calendar-panel,
  .schedule-inspector-panel {
    border: 1px solid var(--schedule-border);
    background:
      linear-gradient(145deg, rgba(244, 251, 255, 0.065), rgba(244, 251, 255, 0.018)),
      var(--schedule-panel);
    box-shadow: var(--wa-shadow-panel, inset 0 1px 0 rgba(255, 255, 255, 0.82), 0 9px 24px rgba(30, 46, 64, 0.075));
    backdrop-filter: blur(16px) saturate(124%);
    -webkit-backdrop-filter: blur(16px) saturate(124%);
  }

  .schedule-filter-strip,
  .schedule-sort-strip {
    background: rgba(7, 11, 16, 0.52);
    border-color: var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
  }

  .schedule-filter-strip button,
  .schedule-sort-strip button {
    min-height: 32px;
    color: var(--schedule-muted);
    border-radius: 6px;
  }

  .schedule-workbench {
    gap: 14px;
  }

  .schedule-summary-grid {
    grid-template-columns: repeat(5, minmax(128px, 1fr));
    gap: 10px;
  }

  .schedule-summary-cell {
    position: relative;
    overflow: hidden;
    min-height: 94px;
    border: 1px solid var(--schedule-border);
    border-top: 0;
    border-radius: var(--wa-radius-lg, 12px);
    background:
      radial-gradient(circle at 100% 0%, rgba(244, 251, 255, 0.07), transparent 8rem),
      rgba(7, 11, 16, 0.54);
    padding: 13px;
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.042);
  }

  .schedule-summary-cell::before {
    content: "";
    position: absolute;
    inset: 0 auto 0 0;
    width: 3px;
    background: rgba(203, 234, 244, 0.28);
  }

  .schedule-summary-cell.is-blue::before { background: var(--wa-info, #9bd4ff); }
  .schedule-summary-cell.is-red::before { background: var(--wa-danger, #ff6177); }
  .schedule-summary-cell.is-amber::before { background: var(--wa-warning, #ffc55f); }
  .schedule-summary-cell.is-violet::before { background: #b8a7ff; }

  .summary-label,
  .schedule-count,
  .risk-calendar-meta,
  .schedule-summary-cell em {
    color: var(--schedule-muted);
  }

  .schedule-summary-cell strong {
    color: var(--schedule-strong);
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
    font-size: 1.45rem;
    font-variant-numeric: tabular-nums;
  }

  .schedule-command-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 0.34fr);
    gap: 14px;
    align-items: stretch;
  }

  .schedule-risk-calendar-panel,
  .schedule-inspector-panel {
    border-radius: var(--wa-radius-xl, 18px);
    padding: 16px;
    min-width: 0;
  }

  .schedule-risk-calendar-panel {
    display: flex;
    flex-direction: column;
    gap: 14px;
    background:
      radial-gradient(circle at 12% 0%, rgba(114, 230, 180, 0.13), transparent 18rem),
      linear-gradient(145deg, rgba(244, 251, 255, 0.07), rgba(244, 251, 255, 0.018)),
      var(--schedule-panel);
  }

  .risk-calendar-head h3,
  .schedule-table-head h3,
  .schedule-inspector-head h3 {
    color: var(--schedule-strong);
    font-size: 1rem;
    line-height: 1.24;
    letter-spacing: 0;
    text-wrap: balance;
  }

  .risk-calendar-head p,
  .schedule-inspector-head p {
    color: var(--schedule-muted);
    font-size: 0.8rem;
    line-height: 1.56;
    text-wrap: pretty;
  }

  .risk-calendar-meta span,
  .risk-calendar-meta button,
  .risk-calendar-command-row button,
  .bucket-risk-chip,
  .bucket-event-item,
  .schedule-focus-item {
    border-color: var(--schedule-border);
    background: rgba(7, 11, 16, 0.46);
  }

  .risk-calendar-meta button,
  .risk-calendar-command-row button,
  .bucket-risk-chip,
  .bucket-event-item,
  .schedule-focus-item,
  .schedule-row-action,
  .schedule-refresh-btn,
  .schedule-inspector-actions button {
    transition: transform 160ms var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)), border-color 160ms ease, background 160ms ease, color 160ms ease;
  }

  .risk-calendar-command-row button {
    min-height: 52px;
    border-radius: var(--wa-radius-md, 8px);
    color: var(--schedule-muted);
  }

  .risk-calendar-command-row button.active {
    color: var(--schedule-accent-ink);
    border-color: color-mix(in srgb, var(--schedule-accent) 64%, transparent);
    background: var(--schedule-accent);
    box-shadow: none;
  }

  .risk-calendar-command-row button.active strong,
  .risk-calendar-command-row button.active span {
    color: var(--schedule-accent-ink);
  }

  .risk-calendar-command-row strong,
  .bucket-title-row em {
    color: var(--schedule-strong);
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
    font-variant-numeric: tabular-nums;
  }

  .risk-calendar-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .risk-calendar-bucket {
    border-color: var(--schedule-border);
    background: rgba(7, 11, 16, 0.42);
    border-radius: var(--wa-radius-lg, 12px);
  }

  .risk-calendar-bucket::before {
    background: linear-gradient(90deg, var(--wa-danger, #ff6177), var(--wa-warning, #ffc55f), var(--schedule-accent));
    opacity: 0.68;
  }

  .bucket-dot {
    background: var(--schedule-accent);
    box-shadow: 0 0 12px color-mix(in srgb, var(--schedule-accent) 48%, transparent);
  }

  .bucket-title-row strong,
  .bucket-event-item strong {
    color: var(--schedule-strong);
  }

  .bucket-title-row span,
  .bucket-event-item em,
  .bucket-empty {
    color: var(--schedule-subtle);
  }

  .schedule-inspector-panel {
    display: grid;
    align-content: start;
    gap: 15px;
    background:
      radial-gradient(circle at 100% 0%, rgba(114, 230, 180, 0.11), transparent 16rem),
      linear-gradient(145deg, rgba(244, 251, 255, 0.07), rgba(244, 251, 255, 0.02)),
      var(--schedule-panel-strong);
  }

  .schedule-inspector-head h3 {
    margin: 5px 0 0;
  }

  .schedule-lock-meter {
    display: grid;
    gap: 10px;
    padding: 13px;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-lg, 12px);
    background: rgba(7, 11, 16, 0.46);
  }

  .schedule-lock-meter > div:first-child {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
  }

  .schedule-lock-meter strong {
    color: var(--schedule-strong);
    font-size: 1.7rem;
    line-height: 1;
  }

  .schedule-lock-meter span {
    color: var(--schedule-muted);
    font-size: 0.76rem;
    font-weight: 720;
  }

  .lock-track {
    height: 8px;
    overflow: hidden;
    border-radius: 999px;
    background: rgba(244, 251, 255, 0.08);
  }

  .lock-track span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, var(--schedule-accent), var(--wa-info, #9bd4ff));
  }

  .schedule-inspector-metrics {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .schedule-inspector-metrics div {
    min-height: 68px;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 10px;
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(244, 251, 255, 0.045);
  }

  .schedule-inspector-metrics span {
    color: var(--schedule-subtle);
    font-size: 0.68rem;
  }

  .schedule-inspector-metrics strong {
    min-width: 0;
    overflow: hidden;
    color: var(--schedule-strong);
    font-size: 0.84rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-focus-list {
    display: grid;
    gap: 8px;
  }

  .focus-list-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    color: var(--schedule-strong);
    font-size: 0.8rem;
  }

  .focus-list-head span {
    color: var(--schedule-subtle);
    font-size: 0.68rem;
  }

  .schedule-focus-item {
    width: 100%;
    display: grid;
    gap: 4px;
    min-height: 70px;
    padding: 10px 11px;
    border: 1px solid var(--schedule-border-strong);
    border-radius: var(--wa-radius-md, 8px);
    color: var(--schedule-text);
    cursor: pointer;
    text-align: left;
  }

  .schedule-focus-item:hover {
    transform: translateY(-1px);
    border-color: var(--schedule-border-strong);
    background: rgba(244, 251, 255, 0.075);
  }

  .schedule-focus-item span {
    color: var(--schedule-accent);
    font-size: 0.66rem;
  }

  .schedule-focus-item strong {
    overflow: hidden;
    color: var(--schedule-strong);
    font-size: 0.8rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-focus-item em {
    overflow: hidden;
    color: var(--schedule-muted);
    font-size: 0.72rem;
    font-style: normal;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-focus-item.risk-overdue { border-left-color: var(--wa-danger, #ff6177); }
  .schedule-focus-item.risk-due_soon { border-left-color: var(--wa-warning, #ffc55f); }
  .schedule-focus-item.risk-stale { border-left-color: #b8a7ff; }
  .schedule-focus-item.risk-unscheduled { border-left-color: var(--wa-info, #9bd4ff); }

  .schedule-focus-empty {
    padding: 14px;
    border: 1px dashed var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
    color: var(--schedule-subtle);
    font-size: 0.78rem;
    text-align: center;
  }

  .schedule-inspector-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .schedule-inspector-actions button {
    min-height: 38px;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(244, 251, 255, 0.055);
    color: var(--schedule-text);
    font-size: 0.76rem;
    font-weight: 800;
    cursor: pointer;
  }

  .schedule-inspector-actions button.primary {
    border-color: color-mix(in srgb, var(--schedule-accent) 54%, transparent);
    background: var(--schedule-accent);
    color: var(--schedule-accent-ink);
  }

  .schedule-inspector-actions button:hover:not(:disabled) {
    transform: translateY(-1px);
    border-color: var(--schedule-border-strong);
    background: rgba(244, 251, 255, 0.09);
  }

  .schedule-inspector-actions button.primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--schedule-accent) 88%, var(--schedule-strong));
  }

  .schedule-inspector-actions button:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  .schedule-control-panel {
    flex-wrap: wrap;
    border-radius: var(--wa-radius-lg, 12px);
    padding: 10px;
  }

  .schedule-search-shell {
    flex: 2 1 280px;
  }

  .schedule-search-shell input,
  .schedule-trigger-override,
  .schedule-menu-trigger {
    height: 38px;
    border-color: var(--schedule-border);
    background: rgba(7, 11, 16, 0.58);
    color: var(--schedule-strong);
    border-radius: var(--wa-radius-md, 8px);
  }

  .schedule-search-shell input::placeholder,
  .schedule-trigger-override::placeholder {
    color: var(--schedule-subtle);
  }

  .schedule-search-shell input:focus,
  .schedule-trigger-override:focus {
    border-color: var(--schedule-accent);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--schedule-accent) 18%, transparent);
  }

  .search-mark,
  .search-mark::after {
    border-color: var(--schedule-subtle);
    background: var(--schedule-subtle);
  }

  .schedule-refresh-btn {
    border-color: color-mix(in srgb, var(--schedule-accent) 42%, transparent);
    background: rgba(114, 230, 180, 0.1);
    color: var(--schedule-accent);
    border-radius: var(--wa-radius-md, 8px);
  }

  .schedule-refresh-btn:hover {
    transform: translateY(-1px);
    background: rgba(114, 230, 180, 0.16);
  }

  .dropdown-options-list.glass-panel {
    border-color: var(--schedule-border);
    background: rgba(9, 14, 20, 0.96);
    box-shadow: 0 18px 42px rgba(1, 9, 14, 0.38);
  }

  .dropdown-option-item {
    color: var(--schedule-text);
  }

  .dropdown-option-item:hover,
  .dropdown-option-item.selected {
    color: var(--schedule-strong);
    background: rgba(114, 230, 180, 0.12);
  }

  .schedule-table-panel {
    overflow: hidden;
    border-radius: var(--wa-radius-xl, 18px);
    background:
      linear-gradient(180deg, rgba(244, 251, 255, 0.055), rgba(244, 251, 255, 0.014)),
      var(--schedule-panel);
  }

  .schedule-table-head {
    padding: 15px 16px;
    border-bottom-color: var(--schedule-border);
  }

  .schedule-table-wrapper {
    max-height: min(620px, calc(100dvh - 320px));
    scrollbar-color: color-mix(in srgb, var(--schedule-accent) 62%, transparent) rgba(7, 11, 16, 0.72);
  }

  .schedule-table-wrapper::-webkit-scrollbar-track,
  .schedule-table-wrapper::-webkit-scrollbar-corner {
    background: rgba(7, 11, 16, 0.72);
  }

  .schedule-table-wrapper::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, var(--schedule-accent), var(--wa-info, #9bd4ff));
    border-color: rgba(7, 11, 16, 0.72);
  }

  .grid-table {
    background: rgba(7, 11, 16, 0.58);
  }

  .grid-thead {
    background: rgba(9, 14, 20, 0.98);
    border-bottom-color: var(--schedule-border);
  }

  .grid-tr {
    grid-template-columns: minmax(250px, 1.55fr) minmax(170px, 1fr) 92px 124px 82px 112px 104px 98px 130px 124px;
    border-bottom-color: rgba(203, 234, 244, 0.08);
  }

  .grid-th {
    color: var(--schedule-subtle);
    font-size: 0.66rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .grid-td {
    color: var(--schedule-text);
  }

  .grid-tr:hover {
    background: rgba(244, 251, 255, 0.052);
  }

  .grid-td.col-action,
  .grid-th.col-action,
  .grid-tr:hover .grid-td.col-action {
    background: rgba(9, 14, 20, 0.98);
    border-left-color: var(--schedule-border);
    box-shadow: -12px 0 20px rgba(1, 9, 14, 0.22);
  }

  .demand-stack strong,
  .plan-stack strong,
  .effort-stack strong,
  .evidence-stack strong,
  .updated-stack strong,
  .owner-stack strong,
  .subtask-stack strong {
    color: var(--schedule-strong);
  }

  .project-cell strong,
  .plan-stack span,
  .effort-stack span,
  .evidence-stack span,
  .updated-stack span,
  .owner-stack span,
  .subtask-stack small {
    color: var(--schedule-muted);
  }

  .schedule-id {
    color: var(--schedule-accent);
  }

  .schedule-id.jira-id-link:hover {
    color: color-mix(in srgb, var(--schedule-accent) 70%, var(--schedule-strong));
  }

  .subtask-meter {
    background: rgba(244, 251, 255, 0.075);
    border-radius: 999px;
  }

  .subtask-meter span {
    background: linear-gradient(90deg, var(--schedule-accent), var(--wa-info, #9bd4ff));
  }

  .subtask-percentage-text {
    color: var(--schedule-strong);
    text-shadow: 0 1px 2px rgba(1, 9, 14, 0.62);
  }

  .schedule-risk-pill,
  .status-chip {
    border-radius: 999px;
  }

  .schedule-row-action {
    width: 54px;
    border-color: var(--schedule-border);
    background: rgba(244, 251, 255, 0.055);
    color: var(--schedule-text);
    border-radius: var(--wa-radius-md, 8px);
  }

  .schedule-row-action:hover {
    transform: translateY(-1px);
    background: rgba(244, 251, 255, 0.09);
    color: var(--schedule-strong);
  }

  .schedule-row-action.is-telemetry {
    border-color: color-mix(in srgb, var(--schedule-accent) 40%, transparent);
    background: rgba(114, 230, 180, 0.1);
    color: var(--schedule-accent);
  }

  .schedule-row-action.is-telemetry:hover {
    background: rgba(114, 230, 180, 0.16);
    color: var(--schedule-strong);
  }

  .schedule-empty-state {
    color: var(--schedule-muted);
    border-top-color: var(--schedule-border);
  }

  @media (max-width: 1320px) {
    .schedule-command-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 980px) {
    .schedule-summary-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .risk-calendar-grid {
      grid-template-columns: 1fr;
    }

    .risk-calendar-command-row {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .schedule-inspector-metrics,
    .schedule-inspector-actions {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 680px) {
    .schedule-summary-grid,
    .risk-calendar-command-row {
      grid-template-columns: 1fr;
    }

    .risk-calendar-head,
    .schedule-table-head {
      align-items: stretch;
      flex-direction: column;
    }

    .schedule-control-panel {
      align-items: stretch;
    }

    .schedule-search-shell,
    .schedule-assignee-menu,
    .schedule-project-menu,
    .schedule-dropdown-menu,
    .schedule-refresh-btn {
      width: 100%;
      min-width: 0;
    }
  }

  /* Phase 41 schedule governance alignment */
  .demand-dashboard {
    --schedule-accent: var(--wa-accent, #008f96);
    --schedule-accent-strong: var(--wa-accent-strong, #006f76);
    --schedule-accent-ink: var(--wa-accent-ink, #ffffff);
    --schedule-panel: var(--wa-chrome-1, rgba(255, 255, 255, 0.86));
    --schedule-panel-strong: var(--wa-chrome-0, rgba(255, 255, 255, 0.94));
    --schedule-panel-soft: var(--wa-surface-inset, #f5f8fb);
    --schedule-border: var(--wa-border-soft, rgba(121, 139, 159, 0.22));
    --schedule-border-strong: var(--wa-border-strong, rgba(85, 106, 128, 0.38));
    --schedule-text: var(--wa-text-main, #293847);
    --schedule-strong: var(--wa-text-strong, #0d1722);
    --schedule-muted: var(--wa-text-muted, #667789);
    --schedule-subtle: var(--wa-text-subtle, #8a99aa);
    color: var(--schedule-text);
  }

  .dashboard-header h2 {
    color: var(--schedule-strong);
    font-size: 1.08rem;
    letter-spacing: 0;
  }

  .add-demand-btn {
    min-height: var(--wa-control-h, 36px);
    border-radius: var(--wa-radius-md, 8px);
    background: var(--schedule-accent);
    color: var(--schedule-accent-ink);
    box-shadow: var(--wa-shadow-sm, 0 7px 16px rgba(30, 46, 64, 0.055));
  }

  .add-demand-btn:hover {
    background: var(--wa-accent-strong, #006f76);
    transform: none;
    box-shadow: 0 12px 26px rgba(0, 143, 150, 0.2);
  }

  .schedule-workbench {
    gap: 12px;
    overflow-anchor: none;
  }

  .schedule-summary-grid {
    grid-template-columns: repeat(4, minmax(150px, 1fr));
    gap: var(--wa-space-3, 12px);
  }

  .schedule-summary-cell.wa-admin-metric {
    min-height: 104px;
    border-top: 0;
    border-radius: var(--wa-radius-xl, 12px);
    background: var(--schedule-panel);
  }

  .schedule-summary-cell.wa-admin-metric::before {
    display: none;
  }

  .schedule-summary-cell.wa-admin-metric span,
  .schedule-summary-cell.wa-admin-metric em {
    color: var(--schedule-muted);
  }

  .schedule-summary-cell.wa-admin-metric strong {
    color: var(--schedule-strong);
    font-size: 2rem;
  }

  .schedule-segment-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(120px, 1fr));
    gap: var(--wa-space-3, 12px);
  }

  .schedule-segment-card {
    min-width: 0;
    min-height: 72px;
    display: grid;
    align-content: center;
    gap: 4px;
    padding: 12px 14px;
    border-top: 3px solid var(--schedule-border-strong);
    box-shadow: none;
  }

  .schedule-segment-card.tone-info { border-top-color: var(--wa-info, #256bd8); }
  .schedule-segment-card.tone-success { border-top-color: var(--wa-success, #04966f); }
  .schedule-segment-card.tone-warning { border-top-color: var(--wa-warning, #d88700); }
  .schedule-segment-card.tone-danger { border-top-color: var(--wa-danger, #dd4b3e); }

  .schedule-segment-card span,
  .schedule-segment-card em {
    overflow: hidden;
    color: var(--schedule-muted);
    font-size: 0.76rem;
    font-style: normal;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-segment-card strong {
    color: var(--schedule-strong);
    font-size: 1.4rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  .schedule-main-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(340px, var(--wa-inspector-w, 420px));
    gap: var(--wa-space-4, 16px);
    align-items: start;
  }

  .schedule-table-stack {
    min-width: 0;
    gap: var(--wa-space-3, 12px);
  }

  .schedule-control-panel {
    min-height: 58px;
    flex-wrap: wrap;
    padding: 10px;
    border-radius: var(--wa-radius-xl, 12px);
    background: var(--schedule-panel);
    box-shadow: var(--wa-shadow-sm, 0 10px 24px rgba(26, 41, 58, 0.08));
  }

  .schedule-search-shell {
    flex: 1 1 230px;
    min-width: 210px;
  }

  .schedule-search-shell input,
  .schedule-control-panel .combobox-trigger-wrapper .dropdown-trigger-input,
  .schedule-trigger-override,
  .schedule-menu-trigger {
    height: var(--wa-control-h, 36px);
    border-color: var(--schedule-border);
    background: rgba(255, 255, 255, 0.84);
    color: var(--schedule-strong);
    border-radius: var(--wa-radius-md, 8px);
  }

  .schedule-search-shell input::placeholder,
  .schedule-control-panel .dropdown-trigger-input::placeholder {
    color: var(--schedule-subtle);
  }

  .schedule-search-shell input:focus,
  .schedule-control-panel .dropdown-trigger-input:focus {
    border-color: var(--schedule-accent);
    background: #ffffff;
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.14);
  }

  .search-mark {
    border-color: var(--schedule-subtle);
    background: transparent;
  }

  .search-mark::after {
    background: var(--schedule-subtle);
    border-color: transparent;
  }

  .schedule-control-panel .dropdown-options-list.glass-panel {
    border-color: var(--schedule-border) !important;
    background: #ffffff !important;
    box-shadow: 0 18px 42px rgba(26, 41, 58, 0.12);
  }

  .schedule-control-panel .dropdown-option-item {
    color: var(--schedule-text);
  }

  .schedule-control-panel .dropdown-option-item:hover,
  .schedule-control-panel .dropdown-option-item.selected {
    color: var(--schedule-strong);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
  }

  .schedule-refresh-btn.wa-admin-action {
    min-width: 64px;
    color: var(--schedule-accent);
  }

  .schedule-table-panel {
    overflow: hidden;
    border-radius: var(--wa-radius-xl, 12px);
    background: var(--schedule-panel);
    box-shadow: var(--wa-shadow-panel, 0 18px 44px rgba(26, 41, 58, 0.1));
  }

  .schedule-table-head {
    min-height: 58px;
    padding: 12px 16px;
    border-bottom-color: var(--schedule-border);
  }

  .schedule-table-head h3 {
    color: var(--schedule-strong);
    font-size: 1rem;
  }

  .schedule-count {
    color: var(--schedule-muted);
  }

  .schedule-table-wrapper.wa-admin-table-shell {
    height: var(--schedule-content-height, 620px);
    max-height: clamp(420px, calc(100dvh - 356px), 620px);
    border: 0;
    border-radius: 0;
    background: rgba(255, 255, 255, 0.72);
    scrollbar-color: rgba(0, 143, 150, 0.42) rgba(121, 139, 159, 0.12);
  }

  .schedule-table-wrapper::-webkit-scrollbar-track,
  .schedule-table-wrapper::-webkit-scrollbar-corner {
    background: rgba(121, 139, 159, 0.1);
  }

  .schedule-table-wrapper::-webkit-scrollbar-thumb {
    background: rgba(0, 143, 150, 0.42);
    border-color: rgba(255, 255, 255, 0.72);
  }

  .schedule-admin-table {
    min-width: 1080px;
    background: transparent;
  }

  .schedule-admin-table th {
    height: 42px;
    color: var(--schedule-muted);
    background: rgba(248, 251, 254, 0.96);
    letter-spacing: 0;
    text-transform: none;
  }

  .schedule-admin-table td {
    height: 76px;
    color: var(--schedule-text);
  }

  .schedule-admin-table .align-center {
    text-align: center;
  }

  .schedule-admin-table .align-right {
    text-align: right;
  }

  .schedule-admin-row {
    cursor: pointer;
  }

  .schedule-admin-row:hover {
    background: var(--wa-row-hover, #f2f8fb);
  }

  .schedule-spacer-row td {
    height: auto;
    padding: 0 !important;
    border: 0 !important;
    background: transparent;
  }

  .schedule-admin-table th.action-col,
  .schedule-admin-table td.action-col {
    position: sticky;
    right: 0;
    z-index: 2;
    background: rgba(255, 255, 255, 0.96);
    box-shadow: -12px 0 18px rgba(26, 41, 58, 0.06);
  }

  .schedule-admin-table thead th.action-col {
    z-index: 3;
    background: rgba(248, 251, 254, 0.98);
  }

  .schedule-id-stack,
  .schedule-title-stack,
  .owner-stack,
  .plan-stack,
  .subtask-stack {
    min-width: 0;
    display: grid;
    gap: 5px;
  }

  .schedule-id-stack {
    justify-items: start;
  }

  .schedule-title-stack strong,
  .owner-stack strong,
  .plan-stack strong {
    overflow: hidden;
    color: var(--schedule-strong);
    font-size: 0.82rem;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-title-stack span,
  .plan-stack span {
    overflow: hidden;
    color: var(--schedule-muted);
    font-size: 0.72rem;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-id {
    color: var(--schedule-accent);
    font-size: 0.72rem;
    font-weight: 850;
    text-decoration: none;
  }

  .schedule-id.jira-id-link:hover {
    color: var(--wa-accent-strong, #006f76);
  }

  .schedule-type-chip.wa-admin-pill,
  .priority-badge.wa-admin-pill,
  .schedule-risk-pill.wa-admin-pill,
  .status-chip.wa-admin-pill {
    margin-left: 0;
    border-radius: var(--wa-radius-sm, 6px);
    font-size: 0.72rem;
    box-shadow: none;
    animation: none;
  }

  .subtask-meter {
    height: 16px;
    min-width: 72px;
    background: rgba(121, 139, 159, 0.14);
    border-radius: 999px;
  }

  .subtask-meter span {
    background: var(--schedule-accent);
  }

  .subtask-percentage-text {
    color: var(--schedule-strong);
    text-shadow: none;
  }

  .schedule-row-action.wa-admin-action {
    width: auto;
    min-width: 44px;
    min-height: 30px;
    padding: 0 8px;
    border-radius: var(--wa-radius-sm, 6px);
    color: var(--schedule-text);
  }

  .schedule-row-action.wa-admin-action:hover {
    transform: none;
    color: var(--schedule-strong);
    background: #ffffff;
  }

  .schedule-row-action.wa-admin-action.is-telemetry {
    color: var(--schedule-accent);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
  }

  .schedule-actions-cell {
    display: flex;
    justify-content: center;
    gap: 6px;
  }

  .schedule-row-muted,
  .schedule-empty-state {
    color: var(--schedule-muted);
  }

  .schedule-inspector-panel.wa-admin-inspector {
    position: sticky;
    top: var(--wa-space-4, 16px);
    max-height: calc(100dvh - 32px);
    overflow: auto;
    border-radius: var(--wa-radius-xl, 12px);
    background: var(--schedule-panel-strong);
    color: var(--schedule-text);
    box-shadow: var(--wa-shadow-panel, 0 18px 44px rgba(26, 41, 58, 0.1));
  }

  .schedule-inspector-head {
    display: grid;
    gap: 8px;
  }

  .inspector-title-row,
  .inspector-section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .schedule-inspector-head h3 {
    margin: 0;
    color: var(--schedule-strong);
    font-size: 1rem;
    line-height: 1.35;
  }

  .schedule-inspector-head p,
  .schedule-inspector-section p {
    margin: 0;
    color: var(--schedule-text);
    font-size: 0.82rem;
    line-height: 1.6;
  }

  .schedule-inspector-facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .schedule-inspector-facts div {
    min-width: 0;
    display: grid;
    gap: 4px;
    padding: 10px;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .schedule-inspector-facts span,
  .inspector-section-head span {
    color: var(--schedule-muted);
    font-size: 0.72rem;
  }

  .schedule-inspector-facts strong {
    overflow: hidden;
    color: var(--schedule-strong);
    font-size: 0.8rem;
    line-height: 1.35;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-lock-meter {
    border-color: var(--schedule-border);
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .schedule-lock-meter strong {
    color: var(--schedule-strong);
  }

  .schedule-lock-meter span {
    color: var(--schedule-muted);
  }

  .schedule-inspector-section {
    display: grid;
    gap: 10px;
    padding-top: 14px;
    border-top: 1px solid var(--schedule-border);
  }

  .inspector-section-head strong {
    color: var(--schedule-strong);
    font-size: 0.86rem;
  }

  .schedule-check-list {
    display: grid;
    gap: 8px;
  }

  .schedule-check-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 8px;
    align-items: start;
  }

  .schedule-check-row p {
    margin: 0;
    color: var(--schedule-text);
    font-size: 0.78rem;
    line-height: 1.45;
  }

  .check-dot {
    width: 9px;
    height: 9px;
    margin-top: 5px;
    border-radius: 999px;
    background: var(--schedule-accent);
    box-shadow: 0 0 0 3px var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
  }

  .inspector-risk-toolbar {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 6px;
  }

  .inspector-risk-toolbar button {
    min-width: 0;
    min-height: 48px;
    display: grid;
    gap: 4px;
    justify-items: center;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-sm, 6px);
    background: rgba(255, 255, 255, 0.72);
    color: var(--schedule-muted);
    cursor: pointer;
  }

  .inspector-risk-toolbar button.active {
    border-color: var(--schedule-accent);
    background: var(--schedule-accent);
    color: var(--schedule-accent-ink);
  }

  .inspector-risk-toolbar span {
    overflow: hidden;
    max-width: 100%;
    font-size: 0.68rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .inspector-risk-toolbar strong {
    font-size: 0.9rem;
    font-variant-numeric: tabular-nums;
  }

  .inspector-risk-buckets {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .inspector-risk-bucket {
    min-width: 0;
    display: grid;
    gap: 4px;
    padding: 9px;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .inspector-risk-bucket span,
  .inspector-risk-bucket em {
    overflow: hidden;
    color: var(--schedule-muted);
    font-size: 0.68rem;
    font-style: normal;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .inspector-risk-bucket strong {
    color: var(--schedule-strong);
    font-size: 1rem;
  }

  .schedule-risk-section .risk-calendar-loading {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .schedule-risk-section .risk-calendar-loading span {
    min-height: 70px;
    background: linear-gradient(90deg, rgba(121, 139, 159, 0.12), rgba(255, 255, 255, 0.86), rgba(121, 139, 159, 0.12));
  }

  .risk-calendar-warning {
    color: var(--wa-warning, #d88700);
    background: var(--wa-warning-soft, rgba(216, 135, 0, 0.14));
    border-color: rgba(216, 135, 0, 0.22);
  }

  .schedule-inspector-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .schedule-inspector-actions .wa-admin-action {
    width: 100%;
  }

  .schedule-inspector-empty {
    display: grid;
    gap: 8px;
    padding: 24px 4px;
    color: var(--schedule-muted);
  }

  .schedule-inspector-empty strong {
    color: var(--schedule-strong);
  }

  @media (max-width: 1280px) {
    .schedule-main-grid {
      grid-template-columns: 1fr;
    }

    .schedule-inspector-panel.wa-admin-inspector {
      position: static;
      max-height: none;
    }
  }

  @media (max-width: 920px) {
    .schedule-summary-grid,
    .schedule-segment-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 680px) {
    .schedule-summary-grid,
    .schedule-segment-grid,
    .schedule-inspector-facts,
    .schedule-inspector-actions,
    .inspector-risk-toolbar,
    .inspector-risk-buckets {
      grid-template-columns: 1fr;
    }

    .schedule-control-panel {
      align-items: stretch;
    }

    .schedule-search-shell,
    .schedule-assignee-menu,
    .schedule-project-menu,
    .schedule-dropdown-menu,
    .schedule-refresh-btn {
      width: 100%;
      min-width: 0;
    }
  }

  /* Final Phase 41 control calibration: override legacy dark dropdown and calendar skins. */
  .schedule-control-panel .combobox-trigger-wrapper {
    height: var(--wa-control-h, 36px);
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(255, 255, 255, 0.86);
    box-shadow: none;
  }

  .schedule-control-panel .combobox-trigger-wrapper:hover,
  .schedule-control-panel .combobox-trigger-wrapper:focus-within {
    border-color: var(--schedule-accent);
    background: #ffffff;
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .schedule-control-panel .dropdown-trigger-input,
  .schedule-control-panel .dropdown-trigger-btn {
    height: 100%;
    border: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--schedule-strong) !important;
  }

  .schedule-control-panel .arrow-icon {
    color: var(--schedule-muted);
  }

  .schedule-control-panel .dropdown-options-list.glass-panel {
    padding: 5px;
    border: 1px solid var(--schedule-border) !important;
    border-radius: var(--wa-radius-lg, 8px);
    background: rgba(255, 255, 255, 0.98) !important;
    box-shadow: 0 20px 48px rgba(26, 41, 58, 0.14);
    backdrop-filter: blur(18px) saturate(124%);
    -webkit-backdrop-filter: blur(18px) saturate(124%);
  }

  .schedule-control-panel .dropdown-option-item {
    min-height: 32px;
    border-radius: var(--wa-radius-sm, 6px);
    color: var(--schedule-text);
  }

  .schedule-control-panel .dropdown-option-item:hover,
  .schedule-control-panel .dropdown-option-item.selected {
    color: var(--schedule-accent-strong);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
  }

  .date-input-shell:focus-within .date-input-display {
    border-color: var(--schedule-accent);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .date-picker-panel,
  .schedule-modal .date-picker-panel {
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-lg, 8px);
    background: rgba(255, 255, 255, 0.98);
    box-shadow: 0 22px 52px rgba(26, 41, 58, 0.16);
    color: var(--schedule-text);
  }

  .date-picker-head strong {
    color: var(--schedule-strong);
  }

  .date-picker-head button,
  .date-picker-foot button,
  .date-cell {
    color: var(--schedule-muted);
  }

  .date-picker-head button,
  .date-picker-foot button {
    border-color: var(--schedule-border);
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .date-picker-head button:hover,
  .date-picker-foot button:hover,
  .date-cell:hover {
    border-color: var(--schedule-accent);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
    color: var(--schedule-accent-strong);
  }

  .date-week-grid span,
  .date-cell.muted {
    color: var(--schedule-subtle);
  }

  .date-cell.today {
    border-color: rgba(0, 143, 150, 0.3);
    color: var(--schedule-accent-strong);
  }

  .date-cell.selected {
    border-color: var(--schedule-accent);
    background: var(--schedule-accent);
    color: var(--schedule-accent-ink);
  }

  .date-picker-foot {
    border-top-color: var(--schedule-border);
  }

  .modal-backdrop {
    background:
      linear-gradient(180deg, rgba(13, 23, 34, 0.2), rgba(13, 23, 34, 0.34)),
      rgba(238, 244, 247, 0.58);
    backdrop-filter: blur(18px) saturate(118%);
    -webkit-backdrop-filter: blur(18px) saturate(118%);
  }

  .modal-content,
  .confirm-modal {
    border: 1px solid rgba(255, 255, 255, 0.72);
    border-radius: var(--wa-radius-xl, 8px);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(247, 251, 253, 0.82)),
      rgba(255, 255, 255, 0.9);
    color: var(--schedule-text);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.92),
      0 26px 72px rgba(26, 41, 58, 0.22);
  }

  .modal-header {
    border-bottom-color: rgba(121, 139, 159, 0.14);
  }

  .modal-header h3,
  .confirm-header h3,
  .confirm-id-value,
  .confirm-message {
    color: var(--schedule-strong);
  }

  .close-btn,
  .confirm-close {
    color: var(--schedule-muted);
  }

  .close-btn:hover,
  .confirm-close:hover {
    color: var(--schedule-accent-strong);
  }

  .form-group label,
  .confirm-kicker,
  .confirm-id-label {
    color: var(--schedule-muted);
  }

  .form-group input[type="text"],
  .form-group textarea,
  .date-input-display {
    border-color: var(--schedule-border);
    background: rgba(255, 255, 255, 0.74);
    color: var(--schedule-strong);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78);
  }

  .form-group input[type="text"]:focus,
  .form-group textarea:focus,
  .date-input-shell:focus-within .date-input-display {
    border-color: var(--schedule-accent);
    background: #ffffff;
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .confirm-id-row,
  .confirm-body {
    border-color: rgba(121, 139, 159, 0.14);
    background: rgba(247, 250, 252, 0.68);
  }

  .confirm-modal .cancel-btn {
    border-color: var(--schedule-border);
    background: rgba(255, 255, 255, 0.72);
    color: var(--schedule-text);
  }

  .confirm-modal .cancel-btn:hover {
    background: #ffffff;
    color: var(--schedule-strong);
  }

  /* Decision-panel aligned schedule governance pass. */
  .dashboard-header {
    min-height: 68px;
    padding: 16px 20px;
    border-color: rgba(255, 255, 255, 0.72);
    border-radius: 20px;
    background:
      linear-gradient(135deg, rgba(255, 255, 255, 0.9), rgba(242, 248, 251, 0.72)),
      var(--wa-chrome-1, rgba(255, 255, 255, 0.86));
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.9),
      0 16px 42px rgba(30, 46, 64, 0.09);
  }

  .schedule-workbench {
    gap: var(--wa-space-4, 16px);
    overflow-anchor: none;
  }

  .schedule-summary-grid {
    grid-template-columns: repeat(4, minmax(160px, 1fr));
    gap: var(--wa-space-3, 12px);
  }

  .schedule-summary-cell.wa-admin-metric {
    min-height: 98px;
    padding: 14px 16px;
    border: 1px solid rgba(255, 255, 255, 0.66);
    border-radius: 16px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(247, 251, 253, 0.6)),
      var(--schedule-panel);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.84),
      0 12px 30px rgba(30, 46, 64, 0.065);
  }

  .schedule-summary-cell.wa-admin-metric strong {
    font-size: 1.9rem;
  }

  .schedule-segment-grid {
    grid-template-columns: repeat(5, minmax(120px, 1fr));
    gap: var(--wa-space-3, 12px);
  }

  .schedule-segment-card {
    min-height: 66px;
    padding: 11px 14px;
    border: 1px solid rgba(121, 139, 159, 0.16);
    border-top-width: 3px;
    border-radius: 14px;
    background: rgba(255, 255, 255, 0.62);
  }

  .schedule-main-grid {
    grid-template-columns: minmax(580px, 1.12fr) minmax(400px, 0.88fr);
    gap: var(--wa-space-4, 16px);
    align-items: start;
  }

  .schedule-table-stack.wa-admin-section {
    min-width: 0;
    gap: 12px;
    padding: 0;
    overflow: visible;
    border: 0;
    background: transparent;
    box-shadow: none;
  }

  .schedule-control-panel.wa-admin-toolbar {
    display: grid;
    grid-template-columns: minmax(190px, 1.2fr) repeat(5, minmax(92px, 0.7fr)) auto auto;
    align-items: center;
    gap: 8px;
    min-height: 58px;
    padding: 10px;
    overflow: visible;
    border-color: rgba(255, 255, 255, 0.64);
    border-radius: 18px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.78), rgba(247, 251, 253, 0.58)),
      var(--schedule-panel);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.82),
      0 12px 30px rgba(30, 46, 64, 0.07);
  }

  .schedule-search-shell,
  .schedule-assignee-menu,
  .schedule-project-menu,
  .schedule-dropdown-menu {
    min-width: 0;
    width: 100%;
  }

  .schedule-refresh-btn.wa-admin-action {
    min-width: 62px;
    min-height: var(--wa-control-h, 36px);
    white-space: nowrap;
  }

  .schedule-table-panel {
    overflow: hidden;
    border-color: rgba(255, 255, 255, 0.66);
    border-radius: 20px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.76), rgba(247, 251, 253, 0.58)),
      var(--schedule-panel);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.84),
      0 16px 42px rgba(30, 46, 64, 0.08);
  }

  .schedule-table-head {
    min-height: 54px;
    padding: 12px 16px;
    border-bottom-color: rgba(121, 139, 159, 0.16);
  }

  .schedule-table-wrapper.wa-admin-table-shell {
    height: var(--schedule-content-height, 620px);
    max-height: clamp(480px, calc(100dvh - 336px), 680px);
    border: 0;
    border-radius: 0;
    background: rgba(255, 255, 255, 0.64);
    overflow-anchor: none;
  }

  .schedule-admin-table {
    min-width: 980px;
    table-layout: fixed;
  }

  .schedule-admin-table th {
    height: 40px;
    padding: 9px 10px;
    font-size: 12px;
    background:
      linear-gradient(180deg, rgba(250, 253, 255, 0.98), rgba(244, 249, 252, 0.94));
  }

  .schedule-admin-table td {
    height: 68px;
    padding: 8px 10px;
    font-size: 12.5px;
  }

  .schedule-title-stack,
  .schedule-id-stack,
  .owner-stack,
  .plan-stack,
  .subtask-stack {
    gap: 4px;
  }

  .schedule-title-stack strong,
  .owner-stack strong,
  .plan-stack strong {
    font-size: 0.8rem;
    line-height: 1.22;
  }

  .schedule-title-stack span,
  .plan-stack span {
    font-size: 0.7rem;
  }

  .schedule-type-chip.wa-admin-pill,
  .priority-badge.wa-admin-pill,
  .schedule-risk-pill.wa-admin-pill,
  .status-chip.wa-admin-pill {
    min-height: 22px;
    padding-inline: 7px;
    border-radius: var(--wa-radius-sm, 7px);
    font-size: 0.7rem;
  }

  .subtask-meter {
    height: 15px;
    min-width: 66px;
  }

  .schedule-row-action.wa-admin-action {
    min-height: 28px;
    padding-inline: 8px;
  }

  .schedule-inspector-panel.wa-admin-inspector {
    top: var(--wa-space-4, 16px);
    max-height: calc(100dvh - 32px);
    padding: 16px;
    border-color: rgba(255, 255, 255, 0.68);
    border-radius: 20px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.84), rgba(247, 251, 253, 0.66)),
      var(--schedule-panel-strong);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 18px 46px rgba(30, 46, 64, 0.1);
  }

  .schedule-inspector-facts {
    gap: 8px 12px;
  }

  .schedule-inspector-facts div {
    padding: 0 0 8px;
    border: 0;
    border-bottom: 1px solid rgba(121, 139, 159, 0.15);
    border-radius: 0;
    background: transparent;
  }

  .schedule-lock-meter {
    padding: 12px 0;
    border-width: 1px 0;
    border-radius: 0;
    background: transparent;
  }

  .schedule-inspector-section {
    gap: 8px;
    padding-top: 12px;
  }

  .inspector-risk-toolbar button {
    min-height: 44px;
    border-radius: 10px;
  }

  .inspector-risk-bucket {
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.56);
  }

  @media (max-width: 1440px) {
    .schedule-control-panel.wa-admin-toolbar {
      grid-template-columns: minmax(220px, 1fr) repeat(3, minmax(112px, 0.8fr));
    }

    .schedule-refresh-btn.wa-admin-action {
      width: 100%;
    }
  }

  @media (max-width: 1280px) {
    .schedule-main-grid {
      grid-template-columns: 1fr;
    }

    .schedule-inspector-panel.wa-admin-inspector {
      position: static;
      max-height: none;
    }
  }

  @media (max-width: 760px) {
    .schedule-summary-grid,
    .schedule-segment-grid,
    .schedule-control-panel.wa-admin-toolbar {
      grid-template-columns: 1fr;
    }
  }

  .schedule-table-wrapper.wa-admin-table-shell,
  .schedule-inspector-panel.wa-admin-inspector,
  .dropdown-options-list {
    scrollbar-color: rgba(0, 143, 150, 0.42) rgba(121, 139, 159, 0.12);
  }

  .schedule-inspector-panel.wa-admin-inspector::-webkit-scrollbar-thumb,
  .dropdown-options-list::-webkit-scrollbar-thumb {
    border: 2px solid rgba(255, 255, 255, 0.76);
    border-radius: 999px;
    background: rgba(0, 143, 150, 0.38);
  }

  /* Schedule governance final containment and color harmonization. */
  .demand-dashboard {
    width: 100%;
    max-width: none;
    margin: 0;
    gap: 16px;
    color: var(--schedule-text);
  }

  .dashboard-subtitle {
    margin: 6px 0 0;
    color: var(--schedule-muted);
    font-size: 0.78rem;
    line-height: 1.45;
  }

  .schedule-workbench {
    width: 100%;
    min-width: 0;
  }

  .schedule-main-grid {
    grid-template-columns: minmax(0, 1fr) minmax(360px, 420px);
    align-items: stretch;
    gap: 16px;
  }

  .schedule-table-stack.wa-admin-section,
  .schedule-table-panel,
  .schedule-inspector-panel.wa-admin-inspector {
    min-width: 0;
  }

  .schedule-table-panel {
    display: flex;
    flex-direction: column;
    min-height: clamp(560px, calc(100dvh - 300px), 720px);
  }

  .schedule-table-wrapper.wa-admin-table-shell {
    flex: 1 1 auto;
    min-height: 460px;
    max-height: clamp(520px, calc(100dvh - 360px), 680px);
  }

  .schedule-inspector-panel.wa-admin-inspector {
    align-self: stretch;
    max-height: clamp(560px, calc(100dvh - 146px), 760px);
    overflow: auto;
    scrollbar-width: thin;
  }

  .schedule-inspector-head h3,
  .schedule-inspector-head p,
  .schedule-inspector-section p,
  .schedule-check-row p {
    overflow-wrap: anywhere;
  }

  .schedule-control-panel.wa-admin-toolbar {
    position: relative;
    z-index: 12;
  }

  .schedule-control-panel .dropdown-options-list.glass-panel,
  .schedule-modal .dropdown-options-list.glass-panel,
  .demand-create-modal .dropdown-options-list.glass-panel {
    border: 1px solid rgba(121, 139, 159, 0.18) !important;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 252, 254, 0.92)),
      rgba(255, 255, 255, 0.92) !important;
    color: var(--schedule-text);
    box-shadow: 0 18px 46px rgba(26, 41, 58, 0.14) !important;
    backdrop-filter: blur(18px) saturate(128%);
    -webkit-backdrop-filter: blur(18px) saturate(128%);
  }

  .schedule-control-panel .dropdown-option-item,
  .schedule-modal .dropdown-option-item,
  .demand-create-modal .dropdown-option-item {
    color: var(--schedule-text);
  }

  .schedule-control-panel .dropdown-option-item:hover,
  .schedule-control-panel .dropdown-option-item.selected,
  .schedule-modal .dropdown-option-item:hover,
  .schedule-modal .dropdown-option-item.selected,
  .demand-create-modal .dropdown-option-item:hover,
  .demand-create-modal .dropdown-option-item.selected {
    background: rgba(0, 143, 150, 0.1);
    color: var(--schedule-accent-strong, #006f76);
  }

  .modal-backdrop {
    padding: 24px;
    background:
      linear-gradient(180deg, rgba(13, 23, 34, 0.18), rgba(13, 23, 34, 0.28)),
      rgba(238, 244, 247, 0.64);
    backdrop-filter: blur(20px) saturate(120%);
    -webkit-backdrop-filter: blur(20px) saturate(120%);
  }

  .modal-content,
  .confirm-modal {
    gap: 0;
    padding: 0;
    border: 1px solid rgba(255, 255, 255, 0.76) !important;
    border-radius: 20px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(247, 251, 253, 0.88)),
      rgba(255, 255, 255, 0.94) !important;
    background-clip: padding-box !important;
    isolation: isolate;
    color: var(--schedule-text);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.94),
      0 28px 78px rgba(26, 41, 58, 0.2) !important;
    scrollbar-color: rgba(0, 143, 150, 0.36) rgba(121, 139, 159, 0.08);
  }

  .schedule-modal {
    width: min(720px, calc(100vw - 48px));
    max-width: 720px;
    overflow: visible;
  }

  .demand-create-modal {
    width: min(680px, calc(100vw - 48px));
    max-width: 680px;
  }

  .detail-modal {
    width: min(960px, calc(100vw - 48px));
    max-width: 960px;
  }

  .confirm-modal {
    width: min(460px, calc(100vw - 48px));
    max-width: 460px;
    border-top: 0 !important;
  }

  .modal-header,
  .confirm-header {
    padding: 18px 22px 14px;
    border-bottom: 1px solid rgba(121, 139, 159, 0.14);
    border-radius: 19px 19px 0 0;
    background: rgba(255, 255, 255, 0.46);
    background-clip: padding-box;
  }

  .modal-footer,
  .confirm-footer {
    border-radius: 0 0 19px 19px;
    background-clip: padding-box;
  }

  .detail-modal .detail-body {
    border-radius: 0 0 19px 19px;
    background-clip: padding-box;
  }

  .modal-header h3,
  .confirm-header h3 {
    color: var(--schedule-strong);
    font-size: 1rem;
    line-height: 1.3;
    letter-spacing: 0;
  }

  .close-btn,
  .confirm-close {
    width: 34px;
    height: 34px;
    border-radius: 10px;
    background: rgba(102, 119, 137, 0.08);
    color: var(--schedule-muted);
  }

  .close-btn:hover,
  .confirm-close:hover {
    background: rgba(0, 143, 150, 0.1);
    color: var(--schedule-accent-strong, #006f76);
  }

  .form-body,
  .detail-body,
  .confirm-body {
    padding: 18px 22px 20px;
  }

  .form-body {
    gap: 14px;
  }

  .demand-brief,
  .schedule-estimate-panel,
  .group-lock-field,
  .confirm-id-row,
  .confirm-body {
    border-color: rgba(121, 139, 159, 0.16);
    background: rgba(247, 250, 252, 0.76);
    color: var(--schedule-text);
  }

  .schedule-estimate-panel {
    border-radius: 16px;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  }

  .estimate-panel-head strong,
  .group-lock-field span,
  .confirm-id-value,
  .confirm-message,
  .detail-title-block strong,
  .detail-facts-grid strong,
  .detail-section-title strong,
  .detail-subtask-row strong {
    color: var(--schedule-strong);
  }

  .estimate-field span,
  .estimate-reference-field span,
  .estimate-difficulty-field > span:first-child,
  .form-group label,
  .field-hint,
  .demand-brief,
  .confirm-kicker,
  .confirm-id-label,
  .detail-title-block p,
  .detail-facts-grid span,
  .detail-section-title span,
  .detail-subtask-row em,
  .detail-empty {
    color: var(--schedule-muted);
  }

  .form-group input[type="text"],
  .form-group textarea,
  .combobox-trigger-wrapper .dropdown-trigger-input,
  .estimate-field input,
  .estimate-reference-field strong,
  .date-input-display {
    border-color: rgba(121, 139, 159, 0.2);
    background: rgba(255, 255, 255, 0.78);
    color: var(--schedule-strong);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.76);
  }

  .form-group input:focus,
  .form-group textarea:focus,
  .combobox-trigger-wrapper:focus-within .dropdown-trigger-input,
  .estimate-field input:focus,
  .date-input-shell:focus-within .date-input-display {
    border-color: var(--schedule-accent);
    background: #ffffff;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.1);
  }

  .estimate-ai-btn,
  .submit-btn,
  .confirm-btn-archive {
    border: 1px solid rgba(0, 143, 150, 0.16) !important;
    background: var(--schedule-accent) !important;
    color: var(--schedule-accent-ink) !important;
    box-shadow: 0 10px 24px rgba(0, 143, 150, 0.16) !important;
  }

  .estimate-ai-btn:hover:not(:disabled),
  .submit-btn:hover,
  .confirm-btn-archive:hover {
    background: var(--wa-accent-strong, #006f76) !important;
  }

  .estimate-clear-btn,
  .cancel-btn,
  .confirm-modal .cancel-btn {
    border: 1px solid rgba(121, 139, 159, 0.2) !important;
    background: rgba(255, 255, 255, 0.72) !important;
    color: var(--schedule-text) !important;
    box-shadow: none !important;
  }

  .estimate-clear-btn:hover,
  .cancel-btn:hover,
  .confirm-modal .cancel-btn:hover {
    background: #ffffff !important;
    color: var(--schedule-strong) !important;
  }

  .detail-title-block,
  .detail-facts-grid div,
  .detail-subtask-row,
  .detail-empty {
    border-color: rgba(121, 139, 159, 0.14);
    background: rgba(247, 250, 252, 0.66);
  }

  .modal-footer,
  .confirm-footer {
    padding: 14px 22px 18px;
    border-top: 1px solid rgba(121, 139, 159, 0.14);
    background: rgba(247, 250, 252, 0.5);
    background-clip: padding-box;
  }

  .demand-create-modal {
    overflow: hidden !important;
  }

  .demand-create-modal > .modal-header,
  .demand-create-modal > .modal-footer {
    flex: 0 0 auto;
  }

  .demand-create-modal > .form-body {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
  }

  .demand-create-modal #demand-assignee-container .dropdown-options-list,
  .demand-create-modal .date-picker-panel {
    top: auto;
    bottom: calc(100% + 8px);
  }

  @media (max-width: 1440px) {
    .demand-dashboard {
      max-width: 100%;
    }

    .schedule-main-grid {
      grid-template-columns: minmax(0, 1fr) minmax(340px, 380px);
    }
  }

  @media (max-width: 1280px) {
    .schedule-main-grid {
      grid-template-columns: 1fr;
    }

    .schedule-inspector-panel.wa-admin-inspector {
      max-height: none;
      overflow: visible;
    }
  }

  @media (max-width: 760px) {
    .modal-backdrop {
      padding: 14px;
      align-items: stretch;
    }

    .schedule-modal,
    .demand-create-modal,
    .detail-modal,
    .confirm-modal {
      width: 100%;
      max-width: none;
    }

    .demand-create-modal {
      max-height: calc(100dvh - 28px);
    }

    .demand-create-modal > .modal-footer {
      flex-wrap: wrap;
    }

    .schedule-table-wrapper.wa-admin-table-shell {
      min-height: 420px;
    }
  }

  /* Strongest-brain page contract: schedule and flow share one shell surface. */
  .demand-dashboard {
    --schedule-accent: var(--wa-accent, #008f96);
    --schedule-accent-strong: var(--wa-accent-strong, #006f76);
    --schedule-accent-ink: var(--wa-accent-ink, #ffffff);
    --schedule-panel: var(--wa-chrome-1, rgba(255, 255, 255, 0.86));
    --schedule-panel-strong: var(--wa-chrome-0, rgba(255, 255, 255, 0.94));
    --schedule-panel-soft: var(--wa-surface-inset, #f5f8fb);
    --schedule-border: var(--wa-border-soft, rgba(121, 139, 159, 0.22));
    --schedule-border-strong: var(--wa-border-strong, rgba(85, 106, 128, 0.38));
    --schedule-text: var(--wa-text-main, #293847);
    --schedule-strong: var(--wa-text-strong, #0d1722);
    --schedule-muted: var(--wa-text-muted, #667789);
    --schedule-subtle: var(--wa-text-subtle, #8a99aa);
    --flow-panel-height: clamp(600px, calc(100dvh - 188px), 760px);
    min-height: 0;
    gap: var(--wa-page-gap, 16px);
    background: transparent !important;
  }

  .dashboard-header {
    min-height: 68px;
    margin: 0;
    padding: 16px 20px;
    border-color: rgba(255, 255, 255, 0.7);
    border-radius: 20px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(247, 251, 253, 0.68)),
      var(--wa-chrome-1, rgba(255, 255, 255, 0.86));
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.9),
      0 16px 42px rgba(30, 46, 64, 0.08);
  }

  .dashboard-header h2,
  .lane-header h4,
  .demand-card h5 {
    color: var(--schedule-strong);
    background: none;
    -webkit-text-fill-color: currentColor;
  }

  .eyebrow,
  .dashboard-subtitle,
  .creator-meta,
  .desc,
  .branch-line,
  .mapped-repo-line,
  .empty-lane,
  .subtask-progress-row,
  .subtask-detail-item,
  .done-date {
    color: var(--schedule-muted);
  }

  .demand-kanban-board {
    width: 100%;
    min-width: 0;
    min-height: var(--flow-panel-height);
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-page-gap, 16px);
    align-items: stretch;
    overflow-anchor: none;
  }

  .kanban-lane {
    min-width: 0;
    min-height: var(--flow-panel-height);
    max-height: var(--flow-panel-height);
    padding: 14px;
    border: 1px solid rgba(255, 255, 255, 0.68);
    border-radius: 20px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(247, 251, 253, 0.62)),
      var(--schedule-panel);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 16px 42px rgba(30, 46, 64, 0.08);
    backdrop-filter: blur(18px) saturate(126%);
    -webkit-backdrop-filter: blur(18px) saturate(126%);
    overflow: hidden;
  }

  .lane-header {
    min-height: 40px;
    padding-bottom: 12px;
    border-bottom-color: rgba(121, 139, 159, 0.16);
  }

  .lane-indicator {
    width: 8px;
    height: 8px;
    box-shadow: none;
  }

  .lane-cards {
    flex: 1 1 auto;
    min-height: 0;
    max-height: none;
    padding: 0 2px 2px 0;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(0, 143, 150, 0.32) rgba(121, 139, 159, 0.1);
  }

  .lane-cards::-webkit-scrollbar {
    width: 8px;
  }

  .lane-cards::-webkit-scrollbar-thumb {
    border: 2px solid rgba(255, 255, 255, 0.72);
    border-radius: 999px;
    background: rgba(0, 143, 150, 0.34);
  }

  .demand-card {
    border: 1px solid rgba(121, 139, 159, 0.18);
    border-top-width: 3px;
    border-radius: 16px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(248, 252, 254, 0.7)),
      rgba(255, 255, 255, 0.76);
    color: var(--schedule-text);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 10px 24px rgba(30, 46, 64, 0.055);
    transition:
      border-color var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)),
      background var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1)),
      box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1));
  }

  .demand-card:hover {
    transform: none;
    border-color: rgba(0, 143, 150, 0.28);
    background: #ffffff;
    box-shadow: 0 14px 32px rgba(30, 46, 64, 0.09);
  }

  .demand-id-button,
  .jira-id-link,
  .assignee-badge,
  .brain-link-badge,
  .priority-badge,
  .due-badge,
  .status-badge,
  .done-tag {
    border-radius: 8px;
  }

  .demand-id-button,
  .jira-id-link,
  .brain-link-badge {
    border-color: rgba(0, 143, 150, 0.18);
    background: rgba(0, 143, 150, 0.08);
    color: var(--schedule-accent-strong);
  }

  .assignee-badge,
  .status-badge {
    background: rgba(37, 107, 216, 0.08);
    color: var(--wa-info, #256bd8);
  }

  .branch-line,
  .mapped-repo-line,
  .deconstruct-subtasks-box,
  .empty-lane {
    border-color: rgba(121, 139, 159, 0.16);
    background: rgba(247, 250, 252, 0.74);
  }

  .subtasks-list-details {
    border-top-color: rgba(121, 139, 159, 0.16);
  }

  .subtask-bar {
    background: rgba(121, 139, 159, 0.18);
  }

  .subtask-bar-fill {
    background: linear-gradient(90deg, var(--schedule-accent), rgba(0, 143, 150, 0.62));
  }

  .edit-sched-btn,
  .icon-action-btn {
    color: var(--schedule-muted);
  }

  .edit-sched-btn:hover,
  .icon-action-btn:hover {
    color: var(--schedule-accent-strong);
  }

  .schedule-main-grid {
    min-height: var(--flow-panel-height);
  }

  .schedule-table-panel,
  .schedule-inspector-panel.wa-admin-inspector {
    min-height: var(--flow-panel-height);
    max-height: var(--flow-panel-height);
  }

  .schedule-table-wrapper.wa-admin-table-shell {
    min-height: 0;
    max-height: none;
  }

  @media (max-width: 1440px) {
    .demand-kanban-board {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 1280px) {
    .schedule-table-panel,
    .schedule-inspector-panel.wa-admin-inspector,
    .kanban-lane {
      max-height: none;
    }

    .demand-kanban-board {
      min-height: auto;
    }
  }

  @media (max-width: 760px) {
    .demand-kanban-board {
      grid-template-columns: 1fr;
    }

    .kanban-lane {
      min-height: 420px;
    }
  }

  /* Schedule table and inspector share one aligned business row. */
  .schedule-dashboard {
    height: 100%;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .schedule-dashboard .schedule-workbench {
    height: auto;
    min-height: 100%;
    display: grid;
    grid-template-rows: auto auto auto;
    overflow: visible;
  }

  .schedule-dashboard .schedule-main-grid {
    display: grid;
    grid-template-columns: minmax(580px, 1.12fr) minmax(400px, 0.88fr);
    grid-template-areas:
      "controls controls"
      "table inspector";
    grid-template-rows: auto minmax(0, auto);
    align-items: stretch;
    height: auto;
    min-height: 0;
    overflow: visible;
  }

  .schedule-dashboard .schedule-table-stack.wa-admin-section {
    display: contents;
  }

  .schedule-dashboard .schedule-control-panel.wa-admin-toolbar {
    grid-area: controls;
  }

  .schedule-dashboard .schedule-table-panel {
    grid-area: table;
    display: flex;
    flex-direction: column;
    height: auto;
    min-height: 0;
    max-height: none;
  }

  .schedule-dashboard .schedule-table-stack > .state-msg {
    grid-area: table;
    min-height: 0;
  }

  .schedule-dashboard .schedule-table-wrapper.wa-admin-table-shell {
    flex: 1 1 auto;
    height: auto;
    min-height: 0;
    max-height: none;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
    grid-area: inspector;
    align-self: stretch;
    height: 100%;
    min-height: 0;
    max-height: none;
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-width: none;
  }

  .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector::-webkit-scrollbar {
    width: 0;
    height: 0;
  }

  .schedule-dashboard .schedule-inspector-facts {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 7px;
  }

  .schedule-dashboard .schedule-inspector-facts > div {
    min-width: 0;
    max-width: 100%;
    min-height: 30px;
    display: inline-flex;
    flex: 0 1 auto;
    align-items: center;
    gap: 6px;
    border: 1px solid rgba(121, 139, 159, 0.16);
    border-radius: 999px;
    padding: 4px 9px;
    background: var(--wa-surface-inset, #f2f6f9);
  }

  .schedule-dashboard .schedule-inspector-facts span {
    flex: 0 0 auto;
    color: var(--schedule-muted);
    font-size: 0.67rem;
    line-height: 1;
    white-space: nowrap;
  }

  .schedule-dashboard .schedule-inspector-facts strong {
    min-width: 0;
    overflow: hidden;
    color: var(--schedule-strong);
    font-size: 0.72rem;
    line-height: 1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-dashboard .schedule-inspector-head {
    gap: 6px;
  }

  .schedule-dashboard .schedule-lock-meter {
    padding-block: 10px;
  }

  .schedule-risk-pill.wa-admin-pill {
    position: relative;
    min-width: 76px;
    justify-content: flex-start;
    gap: 6px;
    padding: 0 9px 0 20px;
    border-radius: 999px;
    font-weight: 840;
    letter-spacing: 0;
  }

  .schedule-risk-pill.wa-admin-pill::before {
    content: "";
    position: absolute;
    left: 8px;
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: currentColor;
    opacity: 0.86;
  }

  .schedule-risk-pill.schedule-risk-overdue {
    border-color: rgba(221, 75, 62, 0.28);
    background: rgba(221, 75, 62, 0.1);
    color: #b42318;
  }

  .schedule-risk-pill.schedule-risk-due_soon {
    border-color: rgba(216, 135, 0, 0.28);
    background: rgba(216, 135, 0, 0.12);
    color: #9a5b00;
  }

  .schedule-risk-pill.schedule-risk-stale {
    border-color: rgba(124, 79, 203, 0.24);
    background: rgba(124, 79, 203, 0.1);
    color: #6841b5;
  }

  .schedule-risk-pill.schedule-risk-unscheduled {
    border-color: rgba(37, 107, 216, 0.22);
    background: rgba(37, 107, 216, 0.09);
    color: #1d56ad;
  }

  .schedule-risk-pill.schedule-risk-safe {
    border-color: rgba(4, 150, 111, 0.22);
    background: rgba(4, 150, 111, 0.1);
    color: #047857;
  }

  .schedule-risk-pill.schedule-risk-done {
    border-color: rgba(102, 119, 137, 0.2);
    background: rgba(102, 119, 137, 0.09);
    color: var(--schedule-muted);
  }

  .status-chip.wa-admin-pill {
    position: relative;
    min-width: 68px;
    justify-content: flex-start;
    gap: 6px;
    padding: 0 9px 0 20px;
    border-radius: 999px;
    font-weight: 840;
    letter-spacing: 0;
  }

  .status-chip.wa-admin-pill::before {
    content: "";
    position: absolute;
    left: 8px;
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: currentColor;
    opacity: 0.82;
  }

  .status-chip.wa-admin-pill.status-scheduled {
    border-color: rgba(0, 143, 150, 0.24);
    background: rgba(0, 143, 150, 0.1);
    color: #006f76;
  }

  .status-chip.wa-admin-pill.status-unscheduled {
    border-color: rgba(102, 119, 137, 0.2);
    background: rgba(102, 119, 137, 0.09);
    color: #667789;
  }

  .status-chip.wa-admin-pill.status-partial {
    border-color: rgba(216, 135, 0, 0.26);
    background: rgba(216, 135, 0, 0.12);
    color: #9a5b00;
  }

  .status-chip.wa-admin-pill.status-progress {
    border-color: rgba(37, 107, 216, 0.24);
    background: rgba(37, 107, 216, 0.1);
    color: #1d56ad;
  }

  .status-chip.wa-admin-pill.status-review {
    border-color: rgba(124, 79, 203, 0.24);
    background: rgba(124, 79, 203, 0.1);
    color: #6841b5;
  }

  .status-chip.wa-admin-pill.status-done {
    border-color: rgba(4, 150, 111, 0.22);
    background: rgba(4, 150, 111, 0.1);
    color: #047857;
  }

  @media (max-width: 1280px) {
    .schedule-dashboard {
      overflow-y: auto;
      overscroll-behavior: contain;
    }

    .schedule-dashboard .schedule-workbench {
      height: auto;
      overflow: visible;
    }

    .schedule-dashboard .schedule-main-grid {
      grid-template-columns: 1fr;
      grid-template-areas:
        "controls"
        "table"
        "inspector";
      grid-template-rows: auto minmax(480px, 58dvh) auto;
      height: auto;
      overflow: visible;
    }

    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      height: auto;
      min-height: 0;
      overflow: visible;
    }
  }

  /* Flow board stays inside the workspace; each lane owns its card overflow. */
  .flow-dashboard {
    position: relative;
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    box-sizing: border-box;
    gap: 12px;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    overflow: hidden;
  }

  .flow-dashboard .dashboard-header {
    min-height: 58px;
    padding: 12px 18px;
  }

  .flow-dashboard .dashboard-subtitle {
    margin-top: 3px;
  }

  .flow-dashboard .demand-kanban-board {
    flex: 1 1 auto;
    min-height: 0;
    height: 100%;
    overflow: hidden;
  }

  .flow-dashboard .kanban-lane {
    min-height: 0;
    height: 100%;
    max-height: 100%;
    padding: 12px;
    gap: 10px;
  }

  .flow-dashboard .lane-header {
    min-height: 34px;
    padding-bottom: 9px;
  }

  .flow-dashboard .lane-cards {
    min-height: 0;
    gap: 9px;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 0 0 2px;
    scrollbar-width: none;
  }

  .flow-dashboard .lane-cards::-webkit-scrollbar {
    width: 0;
    height: 0;
  }

  @media (min-width: 861px) and (max-width: 1440px) {
    .flow-dashboard .demand-kanban-board {
      grid-template-rows: repeat(2, minmax(0, 1fr));
    }
  }

  .flow-dashboard .demand-card {
    gap: 7px;
    padding: 10px 11px;
    border-radius: 14px;
  }

  .flow-dashboard .demand-card h5 {
    display: -webkit-box;
    overflow: hidden;
    font-size: 0.78rem;
    line-height: 1.32;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .flow-dashboard .desc {
    -webkit-line-clamp: 1;
  }

  .flow-dashboard .branch-line,
  .flow-dashboard .mapped-repo-line,
  .flow-dashboard .creator-meta {
    font-size: 0.62rem;
    line-height: 1.25;
  }

  .flow-dashboard .deconstruct-subtasks-box {
    margin-top: 4px;
    padding: 6px 8px;
  }

  .flow-dashboard .card-bottom {
    min-height: 24px;
    margin-top: 0;
    gap: 5px;
    flex-wrap: wrap;
  }

  @media (max-height: 780px) and (min-width: 761px) {
    .flow-dashboard {
      gap: 10px;
    }

    .flow-dashboard .dashboard-header {
      min-height: 52px;
      padding: 10px 16px;
    }

    .flow-dashboard .eyebrow,
    .flow-dashboard .dashboard-subtitle {
      display: none;
    }

    .flow-dashboard .kanban-lane {
      padding: 10px;
    }

    .flow-dashboard .demand-card {
      gap: 6px;
      padding: 9px 10px;
    }
  }

  .schedule-add-demand.add-demand-btn {
    height: var(--wa-control-h, 36px);
    min-width: 118px;
    padding-inline: 11px;
    border-radius: var(--wa-radius-pill, 999px);
    box-shadow: none;
  }

  .flow-filter-panel {
    display: flex;
    min-height: 56px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 8px 10px 8px 16px;
    border-color: var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    background: var(--wa-glass-panel-strong, rgba(252, 254, 255, 0.88));
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .flow-dashboard .kanban-lane {
    border-color: var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.76));
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .flow-filter-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }

  .ai-workbench-action,
  .ai-draft-btn {
    min-height: 38px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-color: rgba(0, 143, 150, 0.24);
    border-radius: var(--wa-radius-pill, 999px);
    background: rgba(0, 143, 150, 0.08);
    color: var(--schedule-accent-strong, #006f76);
    padding: 0 14px;
    font-family: inherit;
    font-size: 0.76rem;
    font-weight: 760;
  }

  .ai-workbench-action:hover,
  .ai-draft-btn:hover:not(:disabled) {
    border-color: rgba(0, 143, 150, 0.38);
    background: rgba(0, 143, 150, 0.13);
    color: var(--schedule-accent-strong, #006f76);
  }

  .detail-link-row .detail-action-ai {
    min-height: 38px;
    border-color: var(--wa-accent-fill, #006f76);
    border-radius: var(--wa-radius-md, 10px);
    background: var(--wa-accent-fill, #006f76);
    color: var(--wa-accent-fill-ink, #f6fbff);
    padding: 0 14px;
    box-shadow: none;
  }

  .detail-link-row .detail-action-ai:hover {
    border-color: var(--wa-accent-fill-hover, #00545a);
    background: var(--wa-accent-fill-hover, #00545a);
    color: var(--wa-accent-fill-ink, #f6fbff);
  }

  .detail-link-row .detail-action-schedule {
    min-height: 38px;
    border-color: var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-md, 10px);
    background: rgba(247, 250, 252, 0.84);
    color: var(--wa-text-main, #293847);
    padding: 0 14px;
    box-shadow: none;
  }

  .detail-link-row .detail-action-schedule:hover {
    border-color: var(--wa-border-strong, rgba(85, 106, 128, 0.32));
    background: #ffffff;
    color: var(--wa-text-strong, #0d1722);
  }

  /* Detail drawer: one stable header, one body scroll owner, and flat facts. */
  .detail-backdrop,
  .deconstructor-backdrop.detail-companion {
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100dvh;
    max-height: none;
  }

  .detail-backdrop {
    z-index: 2000;
    align-items: stretch;
    justify-content: flex-end;
    padding: 0;
    background: rgba(13, 23, 34, 0.24);
    backdrop-filter: blur(2px);
    -webkit-backdrop-filter: blur(2px);
    animation: detailBackdropIn var(--wa-duration-fast, 140ms) ease-out;
  }

  .detail-modal {
    width: min(820px, calc(100% - 32px));
    max-width: 820px;
    height: 100dvh;
    max-height: 100dvh;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    border: 0 !important;
    border-left: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18)) !important;
    border-radius: 0;
    background: var(--wa-surface-flat, #ffffff) !important;
    box-shadow: -12px 0 28px rgba(26, 41, 58, 0.18) !important;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    overflow: hidden;
    animation: detailDrawerIn var(--wa-duration, 200ms) var(--wa-ease, cubic-bezier(0.16, 1, 0.3, 1));
  }

  .modal-backdrop.detail-backdrop.has-ai-companion > .detail-modal {
    transform: none;
  }

  .detail-modal-header {
    flex: 0 0 auto;
    align-items: center;
    border-radius: 0;
  }

  .detail-modal-heading {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-1, 4px);
  }

  .detail-modal-heading > span {
    color: var(--wa-accent-strong, #006f76);
    font-size: 11px;
    font-weight: 760;
  }

  .detail-modal-heading h3 {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .detail-modal .detail-body {
    min-height: 0;
    display: grid;
    align-content: start;
    gap: var(--wa-space-5, 20px);
    padding: var(--wa-space-5, 20px) var(--wa-space-6, 24px) var(--wa-space-6, 24px);
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
    border-radius: 0;
  }

  .detail-modal .detail-body::-webkit-scrollbar {
    display: none;
    width: 0;
    height: 0;
  }

  .detail-overview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: var(--wa-space-5, 20px);
  }

  .detail-title-block {
    min-width: 0;
    padding: 0;
    border: 0;
    background: transparent;
  }

  .detail-id {
    margin-bottom: var(--wa-space-2, 8px);
    border-color: rgba(0, 47, 167, 0.18);
    background: var(--wa-info-soft, rgba(0, 47, 167, 0.1));
    color: var(--wa-info-strong, #0d3a69);
  }

  .detail-id.jira-id-link:hover {
    border-color: var(--wa-info, #002fa7);
    background: rgba(0, 47, 167, 0.14);
    color: var(--wa-info, #002fa7);
  }

  .detail-title-block strong {
    font-size: 18px;
    line-height: 1.4;
  }

  .detail-title-block p {
    max-width: 72ch;
    margin-top: var(--wa-space-2, 8px);
    font-size: 13px;
    line-height: 1.55;
  }

  .detail-link-row {
    flex-wrap: nowrap;
    justify-content: flex-end;
  }

  .detail-facts-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0 var(--wa-space-6, 24px);
    margin: 0;
    padding: 0;
    border-block: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .detail-facts-grid div {
    min-width: 0;
    padding: var(--wa-space-3, 12px) 0;
    border: 0;
    border-radius: 0;
    background: transparent;
  }

  .detail-facts-grid dt,
  .detail-facts-grid dd {
    margin: 0;
  }

  .detail-facts-grid dt {
    margin-bottom: var(--wa-space-1, 4px);
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    font-weight: 720;
  }

  .detail-facts-grid dd {
    overflow-wrap: anywhere;
    color: var(--wa-text-strong, #0d1722);
    font-size: 13px;
    line-height: 1.4;
    font-weight: 720;
  }

  .detail-subtasks {
    gap: var(--wa-space-2, 8px);
  }

  .detail-section-title {
    padding-bottom: var(--wa-space-2, 8px);
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .detail-section-title span,
  .detail-section-title strong {
    font-size: 12px;
  }

  .detail-subtask-list {
    max-height: none;
    overflow: visible;
    gap: 0;
  }

  .detail-subtask-row {
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: var(--wa-space-2, 8px);
    padding: var(--wa-space-3, 12px) 0;
    border: 0;
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: 0;
    background: transparent;
  }

  .detail-subtask-row strong {
    font-size: 12px;
  }

  .detail-subtask-row em {
    font-size: 11px;
  }

  .detail-empty {
    padding: var(--wa-space-3, 12px) 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    text-align: left;
  }

  @media (max-width: 1100px) {
    .detail-facts-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .detail-backdrop .detail-modal {
      width: 100%;
      max-width: none;
      height: 100%;
      max-height: 100%;
      border-radius: 0;
      box-shadow: none !important;
    }

    .detail-modal-header,
    .detail-modal .detail-body {
      border-radius: 0;
    }

    .detail-modal .detail-body {
      padding: var(--wa-space-4, 16px);
    }

    .detail-overview,
    .detail-facts-grid {
      grid-template-columns: 1fr;
    }

    .detail-link-row {
      justify-content: stretch;
    }

    .detail-link-row .detail-action-ai,
    .detail-link-row .detail-action-schedule {
      min-height: 44px;
      flex: 1 1 0;
    }
  }

  @media (max-width: 520px) {
    .detail-link-row,
    .detail-subtask-row {
      align-items: stretch;
      flex-direction: column;
      grid-template-columns: 1fr;
    }

    .detail-subtask-row em {
      white-space: normal;
    }
  }

  .ai-draft-btn:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  .flow-filter-panel > span {
    color: var(--wa-text-strong, #0d1722);
    font-size: 14px;
    font-weight: 760;
    letter-spacing: -0.01em;
  }

  .flow-filter-panel > span strong {
    margin-left: 5px;
    color: var(--wa-text-strong);
    font-weight: 800;
  }

  @media (max-width: 860px) {
    .flow-dashboard {
      position: static;
      inset: auto;
      height: auto;
      padding: 0;
      border: 0;
      background: transparent;
      overflow: visible;
    }

    .flow-dashboard .demand-kanban-board {
      height: auto;
      max-height: none;
    }

    .flow-dashboard .kanban-lane {
      height: min(620px, 72dvh);
      min-height: min(420px, 72dvh);
      max-height: min(620px, 72dvh);
    }

    .flow-dashboard .demand-kanban-board {
      flex: none;
      min-height: auto;
    }

    .flow-dashboard .lane-cards {
      min-height: 0;
      overflow-y: auto;
      overscroll-behavior: contain;
      scrollbar-width: none;
    }
  }

  /* Phase 43 flow-board toolbar refinement. */
  .flow-dashboard .dashboard-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 18px;
    min-height: 108px;
    padding: 20px;
    overflow: visible;
  }

  .dashboard-title-block {
    min-width: 0;
    display: grid;
    gap: 6px;
    align-content: center;
  }

  .flow-dashboard .dashboard-title-block .eyebrow {
    margin-bottom: 0;
  }

  .dashboard-actions {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: 10px;
    min-width: 0;
    align-self: center;
  }

  .flow-demand-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    height: 44px;
    padding: 0 12px;
    border: 1px solid rgba(107, 127, 146, 0.16);
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.58);
    color: var(--schedule-muted);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
    backdrop-filter: blur(14px) saturate(124%);
    -webkit-backdrop-filter: blur(14px) saturate(124%);
    white-space: nowrap;
  }

  .flow-demand-count strong {
    color: var(--schedule-strong);
    font-size: 0.95rem;
    font-weight: 840;
  }

  .flow-demand-count span {
    color: var(--schedule-muted);
    font-size: 0.72rem;
    font-weight: 760;
    letter-spacing: 0;
  }

  .add-demand-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    height: 44px;
    padding: 0 16px 0 12px;
    border: 1px solid rgba(0, 143, 150, 0.24);
    border-radius: 12px;
    background:
      linear-gradient(180deg, rgba(0, 143, 150, 0.96), rgba(0, 111, 118, 0.94)),
      var(--schedule-accent);
    color: #fff;
    box-shadow:
      0 12px 24px rgba(0, 111, 118, 0.18),
      inset 0 1px 0 rgba(255, 255, 255, 0.24);
    font-size: 0.84rem;
    font-weight: 820;
    letter-spacing: 0;
    white-space: nowrap;
    transition:
      transform 160ms var(--wa-ease, ease),
      box-shadow 160ms var(--wa-ease, ease),
      filter 160ms var(--wa-ease, ease);
  }

  .add-demand-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.18);
    color: #fff;
    font-size: 1rem;
    font-weight: 780;
    line-height: 1;
  }

  .add-demand-btn:hover {
    transform: translateY(-1px);
    filter: brightness(1.02);
    box-shadow:
      0 16px 30px rgba(0, 111, 118, 0.22),
      inset 0 1px 0 rgba(255, 255, 255, 0.28);
  }

  .add-demand-btn:active {
    transform: translateY(0) scale(0.99);
  }

  /* Compact single-line schedule table: fit the governance view without horizontal scrolling. */
  .schedule-dashboard .schedule-table-wrapper.wa-admin-table-shell {
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-gutter: stable;
  }

  .schedule-admin-table {
    width: 100%;
    min-width: 0;
    table-layout: fixed;
  }

  .schedule-admin-table th,
  .schedule-admin-table td {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-admin-table td {
    height: 48px;
    padding: 6px 8px;
    vertical-align: middle;
  }

  .schedule-demand-cell {
    position: relative;
  }

  .schedule-demand-line {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 7px;
    padding-right: 14px;
    white-space: nowrap;
  }

  .schedule-demand-line .schedule-id {
    flex: 0 0 auto;
  }

  .schedule-demand-title {
    min-width: 0;
    flex: 1 1 auto;
    overflow: hidden;
    color: var(--schedule-strong);
    font-size: 0.79rem;
    font-weight: 790;
    line-height: 1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-project-inline {
    min-width: 0;
    max-width: 76px;
    flex: 0 1 76px;
    overflow: hidden;
    color: var(--schedule-muted);
    font-size: 0.68rem;
    line-height: 1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-type-dot {
    position: absolute;
    top: 8px;
    right: 8px;
    width: 8px;
    height: 8px;
    border: 2px solid rgba(255, 255, 255, 0.92);
    border-radius: 999px;
    box-shadow: 0 0 0 1px currentColor;
  }

  .schedule-type-dot.is-demand {
    color: #1677a6;
    background: #2f94bd;
  }

  .schedule-type-dot.is-bug {
    color: #b42318;
    background: #d84b3e;
  }

  .schedule-admin-table .owner-stack,
  .schedule-admin-table .plan-stack,
  .schedule-admin-table .subtask-stack {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 0;
  }

  .schedule-admin-table .owner-stack strong,
  .schedule-admin-table .plan-stack strong {
    width: 100%;
    overflow: hidden;
    line-height: 1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .priority-badge.wa-admin-pill {
    display: inline-flex;
    height: 22px;
    min-height: 22px;
    align-items: center;
    justify-content: center;
    padding: 0 7px;
    line-height: 1;
    vertical-align: middle;
  }

  .schedule-risk-pill.wa-admin-pill,
  .status-chip.wa-admin-pill {
    max-width: 100%;
    overflow: hidden;
    align-items: center;
    line-height: 1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-actions-cell {
    width: 100%;
    gap: 3px;
  }

  .schedule-row-action.wa-admin-action {
    min-width: 0;
    flex: 1 1 0;
    padding: 0 3px;
    font-size: 0.68rem;
  }

  /* Keep all date controls on the light admin surface in hover and selected states. */
  .date-input-shell:hover .date-input-display,
  .date-input-display:hover {
    border-color: var(--schedule-accent);
    background: #ffffff;
    color: var(--schedule-strong);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.08);
  }

  .date-input-display .date-input-value {
    color: var(--schedule-muted);
  }

  .date-input-display.has-value .date-input-value {
    color: var(--schedule-strong);
  }

  .date-input-icon {
    border-color: rgba(0, 143, 150, 0.42);
    background: rgba(0, 143, 150, 0.08);
    color: var(--schedule-accent-strong);
  }

  .date-picker-head button:hover,
  .date-picker-foot button:hover,
  .date-cell:hover:not(.selected) {
    border-color: var(--schedule-accent);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
    color: var(--schedule-accent-strong);
  }

  .date-cell.selected:hover {
    border-color: var(--schedule-accent);
    background: var(--schedule-accent);
    color: var(--schedule-accent-ink);
  }

  .flow-dashboard .dashboard-subtitle {
    display: block;
    max-width: min(620px, 100%);
    margin: 0;
    color: var(--schedule-muted);
    font-size: 0.84rem;
    line-height: 1.58;
    overflow: visible;
    white-space: normal;
    text-wrap: balance;
  }

  @media (max-width: 760px) {
    .flow-dashboard .dashboard-header {
      grid-template-columns: 1fr;
      min-height: 0;
      padding: 16px;
    }

    .dashboard-actions {
      width: 100%;
      justify-content: space-between;
      flex-wrap: wrap;
    }

    .flow-demand-count,
    .add-demand-btn {
      flex: 1 1 auto;
    }
  }

  .deconstructor-backdrop {
    z-index: 1120;
    padding: 18px;
  }

  .deconstructor-modal {
    width: min(980px, calc(100vw - 36px));
    max-width: 980px;
    max-height: calc(100dvh - 36px);
    gap: 0;
    padding: 0;
    overflow: hidden;
  }

  .deconstructor-modal .modal-header {
    flex: 0 0 auto;
    align-items: center;
  }

  @media (min-width: 861px) {
    .demand-create-backdrop,
    .deconstructor-backdrop.create-companion {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
    }

    .demand-create-backdrop > .demand-create-modal {
      width: min(680px, 100%);
      max-height: 100%;
    }
  }

  @media (max-width: 860px) {
    .demand-create-backdrop,
    .deconstructor-backdrop.create-companion {
      position: fixed;
      top: var(--wa-main-content-top, 0px);
      right: 0;
      bottom: 0;
      left: 0;
      width: auto;
      height: auto;
    }
  }

  .deconstructor-modal-title {
    min-width: 0;
  }

  .deconstructor-modal-title h3 {
    margin: 2px 0 0;
  }

  .deconstructor-context {
    display: block;
    color: var(--schedule-accent-strong, #006f76);
    font-size: 0.68rem;
    line-height: 1.2;
    font-weight: 760;
    letter-spacing: 0.04em;
  }

  .deconstructor-modal-body {
    min-height: 0;
    padding: 16px 18px 20px;
    overflow: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
  }

  .deconstructor-backdrop.is-companion,
  .modal-backdrop.has-ai-companion {
    --ai-companion-width: clamp(
      420px,
      calc((100vw - var(--wa-workspace-inline-start, 0px) - var(--ai-host-width, 680px)) / 2 - 24px),
      620px
    );
  }

  .deconstructor-backdrop.is-companion {
    justify-content: center;
    padding: 24px;
    background: transparent;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    pointer-events: none;
  }

  .deconstructor-modal.is-companion {
    width: var(--ai-companion-width);
    max-width: 620px;
    height: min(var(--companion-height), 100%);
    max-height: 100%;
    border-color: rgba(121, 139, 159, 0.2);
    box-shadow: 0 28px 74px rgba(26, 41, 58, 0.24);
    pointer-events: auto;
    transform: translateX(calc((var(--ai-host-width, 680px) + 16px) / 2));
  }

  .deconstructor-backdrop.detail-companion .deconstructor-modal.is-companion {
    height: min(var(--companion-height), 100%);
    max-height: 100%;
  }

  .deconstructor-backdrop.detail-companion {
    z-index: 2020;
  }

  @media (min-width: 1501px) {
    .deconstructor-backdrop.detail-companion {
      --detail-drawer-width: min(820px, calc(100% - 32px));
      justify-content: flex-end;
      align-items: stretch;
      padding: 16px calc(var(--detail-drawer-width) + 16px) 16px 16px;
    }

    .deconstructor-backdrop.detail-companion .deconstructor-modal.is-companion {
      flex: 0 1 620px;
      width: min(620px, 100%);
      min-width: 0;
      max-width: 620px;
      height: 100%;
      max-height: 100%;
      transform: none;
    }
  }

  .modal-backdrop.has-ai-companion > .modal-content {
    transform: translateX(calc((var(--ai-companion-width, 620px) + 16px) / -2));
  }

  .deconstructor-modal.is-companion .deconstructor-modal-body {
    padding: 12px;
  }

  @media (max-width: 1500px) {
    .deconstructor-backdrop.is-companion {
      justify-content: center;
      padding: 18px;
      background:
        linear-gradient(180deg, rgba(13, 23, 34, 0.12), rgba(13, 23, 34, 0.24)),
        rgba(238, 244, 247, 0.42);
      backdrop-filter: blur(12px) saturate(112%);
      -webkit-backdrop-filter: blur(12px) saturate(112%);
      pointer-events: auto;
    }

    .deconstructor-modal.is-companion {
      width: min(680px, calc(100vw - 36px));
      max-width: 680px;
      transform: none;
    }

    .modal-backdrop.has-ai-companion > .modal-content {
      transform: none;
    }
  }

  @media (max-width: 760px) {
    .flow-filter-panel,
    .flow-filter-actions {
      align-items: stretch;
      flex-direction: column;
    }

    .deconstructor-backdrop {
      padding: 0;
    }

    .deconstructor-modal {
      width: 100vw;
      height: 100dvh;
      max-height: 100dvh;
      border-radius: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .detail-backdrop,
    .detail-modal {
      animation: none;
    }
  }

  @keyframes detailBackdropIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes detailDrawerIn {
    from { transform: translateX(32px); opacity: 0.72; }
    to { transform: translateX(0); opacity: 1; }
  }

  /* Schedule owns one bounded data row; the inspector is a concise peer, never a scroll pane. */
  .schedule-summary-cell.wa-admin-metric,
  .schedule-segment-card,
  .schedule-control-panel.wa-admin-toolbar,
  .schedule-table-panel,
  .schedule-inspector-panel.wa-admin-inspector {
    border-color: var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.76));
    background-image: none;
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .schedule-search-shell input,
  .schedule-control-panel .combobox-trigger-wrapper,
  .schedule-control-panel .dropdown-trigger-input,
  .schedule-trigger-override,
  .schedule-menu-trigger,
  .schedule-refresh-btn.wa-admin-action {
    border-radius: var(--wa-radius-pill, 999px);
  }

  .schedule-table-wrapper.wa-admin-table-shell {
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    background: transparent;
    box-shadow: none;
  }

  .schedule-dashboard .schedule-inspector-facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 12px;
  }

  .schedule-dashboard .schedule-inspector-facts > div {
    min-height: 0;
    display: grid;
    align-content: center;
    gap: 3px;
    padding: 8px 0;
    border: 0;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: 0;
    background: transparent;
  }

  .schedule-dashboard .schedule-inspector-facts span {
    font-size: 12px;
  }

  .schedule-dashboard .schedule-inspector-facts strong {
    font-size: 13px;
  }

  @media (min-width: 1281px) {
    .schedule-dashboard {
      height: 100%;
      min-height: 0;
      overflow: hidden;
    }

    .schedule-dashboard .schedule-workbench {
      height: 100%;
      min-height: 0;
      grid-template-rows: auto auto minmax(0, 1fr);
      overflow: hidden;
    }

    .schedule-dashboard .schedule-main-grid {
      height: 100%;
      min-height: 0;
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden;
    }

    .schedule-dashboard .schedule-table-panel,
    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      height: 100%;
      min-height: 0;
      max-height: none;
    }

    .schedule-dashboard .schedule-table-wrapper.wa-admin-table-shell {
      height: 100%;
      min-height: 0;
      max-height: none;
      overflow: auto;
    }

    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      position: relative;
      top: auto;
      overflow: hidden;
      overscroll-behavior: auto;
    }

    .schedule-dashboard .schedule-inspector-section p,
    .schedule-dashboard .schedule-check-row p {
      display: -webkit-box;
      overflow: hidden;
      line-clamp: 3;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 3;
    }
  }

  @media (max-width: 1280px) {
    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      align-self: start;
      height: max-content;
      min-height: 150px;
      overflow: visible;
    }
  }

  /* 排期治理统一为一条指标带，右侧检查器独占详情、编辑与代码轨迹。 */
  .schedule-signal-strip {
    min-width: 0;
    min-height: 72px;
    display: grid;
    grid-template-columns: repeat(4, minmax(104px, 1.15fr)) repeat(5, minmax(88px, 0.9fr));
    gap: 0;
    padding: 0;
    overflow: hidden;
    border: 1px solid var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.76));
    box-shadow: var(--wa-shadow-glass, inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 14px rgba(30, 52, 68, 0.085));
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .schedule-signal-cell {
    min-width: 0;
    min-height: 72px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-rows: auto auto;
    align-content: center;
    column-gap: 8px;
    row-gap: 3px;
    padding: 9px 12px;
    border-left: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    background: transparent;
  }

  .schedule-signal-cell:first-child {
    border-left: 0;
  }

  .schedule-signal-cell span,
  .schedule-signal-cell em {
    min-width: 0;
    overflow: hidden;
    color: var(--schedule-muted);
    font-size: 0.68rem;
    font-style: normal;
    line-height: 1.2;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-signal-cell span {
    align-self: end;
    font-weight: 720;
  }

  .schedule-signal-cell strong {
    grid-row: 1 / span 2;
    grid-column: 2;
    align-self: center;
    color: var(--schedule-strong);
    font-size: 1.35rem;
    font-variant-numeric: tabular-nums;
    line-height: 1;
  }

  .schedule-signal-cell.is-primary strong {
    font-size: 1.5rem;
  }

  .schedule-signal-cell.tone-danger strong {
    color: var(--wa-danger, #c93d32);
  }

  .schedule-signal-cell.tone-warning strong {
    color: var(--wa-warning-strong, #9a5b00);
  }

  .schedule-signal-cell.tone-success strong {
    color: var(--wa-success-strong, #047857);
  }

  .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
    display: flex;
    flex-direction: column;
    gap: 0;
    padding: 0;
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.82));
  }

  .schedule-dashboard .schedule-inspector-head {
    gap: 5px;
    padding: 14px 16px 11px;
  }

  .schedule-dashboard .schedule-inspector-head h3 {
    display: -webkit-box;
    overflow: hidden;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    font-size: 0.98rem;
    line-height: 1.35;
  }

  .schedule-dashboard .schedule-inspector-head p {
    display: -webkit-box;
    overflow: hidden;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    color: var(--schedule-muted);
    font-size: 0.74rem;
    line-height: 1.45;
  }

  .schedule-inspector-tabs {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 0 16px;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
  }

  .schedule-inspector-tabs button {
    min-height: 36px;
    padding: 0 10px;
    border: 0;
    border-bottom: 2px solid transparent;
    background: transparent;
    color: var(--schedule-muted);
    font-size: 0.76rem;
    font-weight: 760;
    cursor: pointer;
  }

  .schedule-inspector-tabs button:hover {
    color: var(--schedule-strong);
  }

  .schedule-inspector-tabs button.active {
    border-bottom-color: var(--schedule-accent);
    color: var(--schedule-accent-strong);
  }

  .schedule-inspector-tabs button:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.34);
    outline-offset: -3px;
  }

  .schedule-editor-body,
  .schedule-telemetry-inline {
    min-width: 0;
    padding: 12px 16px 14px;
  }

  .schedule-dashboard .schedule-inspector-facts {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 0;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
  }

  .schedule-dashboard .schedule-inspector-facts > div {
    min-width: 0;
    min-height: 50px;
    display: grid;
    align-content: center;
    gap: 3px;
    padding: 6px 8px;
    border: 0;
    border-left: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: 0;
    background: transparent;
  }

  .schedule-dashboard .schedule-inspector-facts > div:first-child {
    border-left: 0;
  }

  .schedule-dashboard .schedule-inspector-facts span,
  .schedule-dashboard .schedule-inspector-facts strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-dashboard .schedule-inspector-facts span {
    font-size: 0.62rem;
  }

  .schedule-dashboard .schedule-inspector-facts strong {
    font-size: 0.7rem;
  }

  .schedule-editor-section,
  .schedule-editor-context,
  .schedule-risk-inline {
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
  }

  .schedule-editor-section {
    padding-top: 11px;
  }

  .schedule-editor-heading,
  .schedule-editor-heading > div,
  .schedule-editor-progress,
  .schedule-editor-actions {
    display: flex;
    align-items: center;
  }

  .schedule-editor-heading {
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 10px;
  }

  .schedule-editor-heading > div:first-child {
    min-width: 0;
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .schedule-editor-heading strong {
    color: var(--schedule-strong);
    font-size: 0.82rem;
  }

  .schedule-editor-heading span {
    color: var(--schedule-muted);
    font-size: 0.64rem;
  }

  .schedule-editor-progress {
    flex: 0 0 auto;
    gap: 6px;
  }

  .schedule-editor-progress strong {
    color: var(--schedule-accent-strong);
    font-size: 0.78rem;
  }

  .schedule-editor-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px 10px;
  }

  .schedule-editor-field {
    min-width: 0;
    display: grid;
    align-content: start;
    gap: 5px;
    margin: 0;
  }

  .schedule-editor-field.is-wide {
    grid-column: 1 / -1;
  }

  .schedule-editor-field > span {
    color: var(--schedule-muted);
    font-size: 0.66rem;
    font-weight: 720;
    line-height: 1.2;
  }

  .schedule-editor-field input,
  .schedule-editor-output {
    width: 100%;
    min-width: 0;
    min-height: 34px;
    box-sizing: border-box;
    padding: 0 10px;
    border: 1px solid var(--schedule-border);
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(255, 255, 255, 0.72);
    color: var(--schedule-strong);
    font: inherit;
    font-size: 0.76rem;
  }

  .schedule-editor-output {
    display: flex;
    align-items: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-editor-field input:focus {
    outline: 2px solid rgba(0, 143, 150, 0.24);
    outline-offset: 1px;
    border-color: var(--schedule-accent);
  }

  .schedule-editor-field input:disabled,
  .schedule-editor-field :global(.select-group.disabled) {
    cursor: not-allowed;
    opacity: 0.62;
  }

  .schedule-editor-field :global(.select-group) {
    min-width: 0;
  }

  .schedule-editor-field :global(.select-trigger) {
    min-height: 34px;
    border-radius: var(--wa-radius-md, 8px);
    background: rgba(255, 255, 255, 0.72);
  }

  .schedule-editor-error {
    margin: 8px 0 0;
    color: var(--wa-danger, #c93d32);
    font-size: 0.7rem;
    line-height: 1.4;
  }

  .schedule-editor-actions {
    justify-content: flex-end;
    gap: 6px;
    margin-top: 10px;
  }

  .schedule-editor-actions .wa-admin-action {
    min-width: 0;
    min-height: 32px;
    flex: 1 1 0;
    padding-inline: 7px;
    white-space: nowrap;
  }

  .schedule-editor-context {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
    margin-top: 12px;
    padding-top: 10px;
  }

  .schedule-editor-context section {
    min-width: 0;
    display: grid;
    align-content: start;
    gap: 7px;
  }

  .schedule-editor-context .schedule-check-list {
    gap: 5px;
  }

  .schedule-editor-context .schedule-check-row {
    gap: 6px;
  }

  .schedule-editor-context .schedule-check-row p,
  .schedule-editor-context section > p {
    display: -webkit-box;
    overflow: hidden;
    margin: 0;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    color: var(--schedule-text);
    font-size: 0.68rem;
    line-height: 1.4;
  }

  .schedule-editor-context .check-dot {
    width: 7px;
    height: 7px;
    margin-top: 4px;
    box-shadow: none;
  }

  .schedule-risk-inline {
    display: grid;
    gap: 8px;
    margin-top: 10px;
    padding-top: 10px;
  }

  .schedule-risk-inline .inspector-section-head > div {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .schedule-risk-inline .inspector-section-head > div > span {
    overflow: hidden;
    font-size: 0.64rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .schedule-risk-inline .wa-admin-action {
    min-height: 30px;
    padding-inline: 9px;
  }

  .schedule-risk-inline .inspector-risk-toolbar {
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 4px;
  }

  .schedule-risk-inline .inspector-risk-toolbar button {
    min-height: 36px;
    gap: 2px;
    border-radius: var(--wa-radius-sm, 6px);
  }

  .schedule-risk-inline .inspector-risk-toolbar span {
    font-size: 0.61rem;
  }

  .schedule-risk-inline .inspector-risk-toolbar strong {
    font-size: 0.76rem;
  }

  @media (min-width: 1281px) {
    .schedule-dashboard .schedule-workbench {
      grid-template-rows: auto minmax(0, 1fr);
    }

    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      overflow: hidden;
    }
  }

  @media (max-width: 1280px) {
    .schedule-signal-strip {
      grid-template-columns: repeat(9, minmax(92px, 1fr));
      overflow-x: auto;
      overflow-y: hidden;
      scrollbar-width: none;
    }

    .schedule-signal-strip::-webkit-scrollbar {
      display: none;
    }

    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      overflow: visible;
    }
  }

  @media (max-width: 760px) {
    .schedule-signal-cell {
      padding-inline: 9px;
    }

    .schedule-signal-cell em {
      display: none;
    }

    .schedule-editor-grid,
    .schedule-editor-context {
      grid-template-columns: 1fr;
    }

    .schedule-editor-field.is-wide {
      grid-column: auto;
    }

    .schedule-editor-actions {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  /* Keep the complete editor visible inside the viewport-owned inspector. */
  @media (min-width: 1281px) {
    .schedule-dashboard .schedule-inspector-head {
      gap: 3px;
      padding: 10px 14px 8px;
    }

    .schedule-dashboard .schedule-inspector-head h3 {
      -webkit-line-clamp: 1;
      line-clamp: 1;
    }

    .schedule-dashboard .schedule-inspector-head p {
      -webkit-line-clamp: 1;
      line-clamp: 1;
    }

    .schedule-inspector-tabs {
      padding-inline: 14px;
    }

    .schedule-inspector-tabs button {
      min-height: 32px;
    }

    .schedule-editor-body,
    .schedule-telemetry-inline {
      padding: 8px 14px 10px;
    }

    .schedule-dashboard .schedule-inspector-facts > div {
      min-height: 42px;
      padding: 4px 7px;
    }

    .schedule-editor-section {
      padding-top: 8px;
    }

    .schedule-editor-heading {
      margin-bottom: 6px;
    }

    .schedule-editor-grid {
      grid-template-columns:
        minmax(112px, 1.5fr)
        minmax(54px, 0.72fr)
        minmax(62px, 0.85fr)
        minmax(88px, 1.15fr)
        minmax(82px, 1.1fr);
      gap: 6px;
    }

    .schedule-editor-field.is-wide {
      grid-column: auto;
    }

    .schedule-editor-field {
      gap: 3px;
    }

    .schedule-editor-field input,
    .schedule-editor-output,
    .schedule-editor-field :global(.select-trigger) {
      min-height: 30px;
    }

    .schedule-editor-actions {
      margin-top: 7px;
    }

    .schedule-editor-actions .wa-admin-action {
      min-height: 30px;
    }

    .schedule-editor-context {
      gap: 10px;
      margin-top: 8px;
      padding-top: 7px;
    }

    .schedule-editor-context section {
      gap: 4px;
    }

    .schedule-editor-context .schedule-check-list {
      gap: 3px;
    }

    .schedule-editor-context .schedule-check-row p,
    .schedule-editor-context section > p {
      -webkit-line-clamp: 1;
      line-clamp: 1;
    }

    .schedule-risk-inline {
      grid-template-columns: minmax(126px, 0.72fr) minmax(0, 2.28fr);
      align-items: center;
      gap: 8px;
      margin-top: 7px;
      padding-top: 7px;
    }

    .schedule-risk-inline .inspector-section-head {
      gap: 6px;
    }

    .schedule-risk-inline .inspector-section-head > div {
      gap: 0;
    }

    .schedule-risk-inline .wa-admin-action {
      min-width: 46px;
      min-height: 28px;
      padding-inline: 7px;
      white-space: nowrap;
    }

    .schedule-risk-inline .inspector-risk-toolbar button {
      min-height: 30px;
    }
  }

  /* Final schedule inspector contract: table-first master/detail, content-owned height. */
  .schedule-dashboard .schedule-main-grid {
    grid-template-columns: minmax(580px, 1fr) clamp(420px, 34vw, 560px);
    align-items: start;
  }

  .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
    align-self: start;
    width: 100%;
    height: auto;
    max-height: 100%;
    overflow: hidden;
  }

  .schedule-dashboard .schedule-inspector-head {
    flex: 0 0 auto;
    gap: var(--wa-space-1, 4px);
    padding: var(--wa-space-4, 16px);
  }

  .schedule-dashboard .schedule-inspector-head h3 {
    -webkit-line-clamp: 2;
    line-clamp: 2;
    font-size: 1rem;
    line-height: 1.4;
  }

  .schedule-dashboard .schedule-inspector-head p {
    -webkit-line-clamp: 3;
    line-clamp: 3;
    font-size: 0.76rem;
    line-height: 1.5;
  }

  .schedule-inspector-tabs {
    flex: 0 0 auto;
    padding-inline: var(--wa-space-4, 16px);
  }

  .schedule-inspector-tabs button {
    min-height: var(--wa-touch-h, 44px);
    padding-inline: var(--wa-space-3, 12px);
  }

  .schedule-editor-body,
  .schedule-telemetry-inline {
    min-height: 0;
    flex: 1 1 auto;
    overflow: auto;
    padding: var(--wa-space-4, 16px);
    overscroll-behavior: contain;
  }

  .schedule-editor-body:focus-visible,
  .schedule-telemetry-inline:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.3);
    outline-offset: -2px;
  }

  .schedule-dashboard .schedule-inspector-facts {
    grid-template-columns: repeat(6, minmax(0, 1fr));
    border-block: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-bottom-width: 1px;
  }

  .schedule-dashboard .schedule-inspector-facts > div {
    grid-column: span 2;
    min-height: 52px;
    gap: var(--wa-space-1, 4px);
    padding: var(--wa-space-2, 8px);
    border-top: 0;
  }

  .schedule-dashboard .schedule-inspector-facts > div:nth-last-child(-n + 2) {
    grid-column: span 3;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
  }

  .schedule-dashboard .schedule-inspector-facts > div:nth-child(4) {
    border-left: 0;
  }

  .schedule-dashboard .schedule-inspector-facts span {
    font-size: 0.68rem;
  }

  .schedule-dashboard .schedule-inspector-facts strong {
    font-size: 0.76rem;
  }

  .schedule-editor-section {
    padding-top: var(--wa-space-4, 16px);
    border-top: 0;
  }

  .schedule-editor-heading {
    gap: var(--wa-space-3, 12px);
    margin-bottom: var(--wa-space-3, 12px);
  }

  .schedule-editor-heading > div:first-child {
    gap: var(--wa-space-1, 4px);
  }

  .schedule-editor-heading strong {
    font-size: 0.84rem;
  }

  .schedule-editor-heading span {
    font-size: 0.68rem;
  }

  .schedule-editor-progress {
    gap: var(--wa-space-2, 8px);
  }

  .schedule-editor-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-3, 12px);
  }

  .schedule-editor-field {
    gap: var(--wa-space-1, 4px);
  }

  .schedule-editor-field.is-wide {
    grid-column: 1 / -1;
  }

  .schedule-editor-field input,
  .schedule-editor-output,
  .schedule-editor-field :global(.select-trigger) {
    min-height: var(--wa-control-h, 36px);
    padding-inline: var(--wa-space-3, 12px);
  }

  .schedule-editor-error {
    margin-top: var(--wa-space-2, 8px);
    font-size: 0.72rem;
  }

  .schedule-editor-actions {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--wa-space-3, 12px);
    margin-top: var(--wa-space-4, 16px);
  }

  .schedule-editor-secondary-actions {
    min-width: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--wa-space-2, 8px);
  }

  .schedule-editor-actions .wa-admin-action,
  .schedule-editor-secondary-actions .wa-admin-action {
    min-height: var(--wa-control-h, 36px);
    flex: 0 1 auto;
    padding-inline: var(--wa-space-3, 12px);
  }

  .schedule-editor-actions .schedule-save-action {
    min-width: 104px;
    padding-inline: var(--wa-space-4, 16px);
  }

  .schedule-editor-context {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-4, 16px);
    margin-top: var(--wa-space-4, 16px);
    padding-top: var(--wa-space-4, 16px);
  }

  .schedule-editor-context section {
    gap: var(--wa-space-2, 8px);
  }

  .schedule-editor-context .schedule-check-list,
  .schedule-editor-context .schedule-check-row {
    gap: var(--wa-space-2, 8px);
  }

  .schedule-editor-context .schedule-check-row p,
  .schedule-editor-context section > p {
    display: block;
    overflow: visible;
    -webkit-line-clamp: unset;
    line-clamp: unset;
    font-size: 0.72rem;
    line-height: 1.5;
  }

  .schedule-risk-inline {
    grid-template-columns: 1fr;
    align-items: stretch;
    gap: var(--wa-space-3, 12px);
    margin-top: var(--wa-space-4, 16px);
    padding-top: var(--wa-space-4, 16px);
  }

  .schedule-risk-inline .inspector-section-head {
    gap: var(--wa-space-3, 12px);
  }

  .schedule-risk-inline .inspector-section-head > div {
    gap: var(--wa-space-1, 4px);
  }

  .schedule-risk-inline .wa-admin-action {
    min-height: var(--wa-control-h, 36px);
    padding-inline: var(--wa-space-3, 12px);
  }

  .schedule-risk-inline .inspector-risk-toolbar {
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: var(--wa-space-2, 8px);
  }

  .schedule-risk-inline .inspector-risk-toolbar button {
    min-height: var(--wa-touch-h, 44px);
    gap: var(--wa-space-1, 4px);
  }

  @media (max-width: 1280px) {
    .schedule-dashboard .schedule-main-grid {
      grid-template-columns: minmax(0, 1fr);
      align-items: start;
    }

    .schedule-dashboard .schedule-table-panel {
      height: 100%;
      min-height: 0;
      max-height: 100%;
      overflow: hidden;
    }

    .schedule-dashboard .schedule-table-wrapper.wa-admin-table-shell {
      height: 100%;
      min-height: 0;
      max-height: 100%;
      flex: 1 1 0;
      overflow: auto;
    }

    .schedule-dashboard .schedule-inspector-panel.wa-admin-inspector {
      width: 100%;
      height: auto;
      max-height: none;
      overflow: visible;
    }

    .schedule-editor-body,
    .schedule-telemetry-inline {
      overflow: visible;
    }
  }

  @media (max-width: 760px) {
    .schedule-dashboard .schedule-inspector-head,
    .schedule-editor-body,
    .schedule-telemetry-inline {
      padding: var(--wa-space-3, 12px);
    }

    .schedule-inspector-tabs {
      padding-inline: var(--wa-space-3, 12px);
    }

    .schedule-dashboard .schedule-inspector-facts {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .schedule-dashboard .schedule-inspector-facts > div,
    .schedule-dashboard .schedule-inspector-facts > div:nth-last-child(-n + 2) {
      grid-column: auto;
      border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    }

    .schedule-dashboard .schedule-inspector-facts > div:nth-child(-n + 2) {
      border-top: 0;
    }

    .schedule-dashboard .schedule-inspector-facts > div:nth-child(odd) {
      border-left: 0;
    }

    .schedule-dashboard .schedule-inspector-facts > div:last-child {
      grid-column: 1 / -1;
      border-left: 0;
    }

    .schedule-editor-grid,
    .schedule-editor-context {
      grid-template-columns: minmax(0, 1fr);
    }

    .schedule-editor-field.is-wide {
      grid-column: auto;
    }

    .schedule-editor-field input,
    .schedule-editor-output,
    .schedule-editor-field :global(.select-trigger),
    .schedule-editor-actions .wa-admin-action,
    .schedule-editor-secondary-actions .wa-admin-action,
    .schedule-risk-inline .wa-admin-action {
      min-height: var(--wa-touch-h, 44px);
    }

    .schedule-editor-actions {
      grid-template-columns: minmax(0, 1fr);
    }

    .schedule-editor-secondary-actions {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .schedule-editor-secondary-actions .wa-admin-action,
    .schedule-editor-actions .schedule-save-action {
      width: 100%;
    }

    .schedule-risk-inline .inspector-risk-toolbar {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }
</style>
