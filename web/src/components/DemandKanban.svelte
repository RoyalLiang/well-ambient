<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { lockBodyScroll, unlockBodyScroll } from '../lib/modalScrollLock';
  import CommitTelemetryPanel from './CommitTelemetryPanel.svelte';

  export let currentUserPermissions: string[] = [];
  export let currentUserName: string = '';
  export let currentUserEmail: string = '';
  export let currentUserDepartment: string = '';

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
  }

  type DemandView = 'board' | 'schedule';
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

  interface DemandOptionsResponse {
    assignees: string[];
    projects: string[];
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
    name: string;
    department: string;
  }

  let isMounted = false;
  let demands: Demand[] = [];
  let demandsById: Map<string, Demand> = new Map();
  let users: UserOption[] = [];
  let demandOptionAssignees: string[] = [];
  let demandOptionProjects: string[] = [];
  let loading = false;
  let errorMsg = '';
  let activeDemandView: DemandView = 'board';
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
  let showScheduleProjectDropdown = false;
  let showRiskDropdown = false;
  let showTypeDropdown = false;
  let showSortDropdown = false;
  let projectConfigs: any[] = [];
  $: projectConfigMap = new Map<string, any>(
    projectConfigs
      .filter(c => c && c.project_key)
      .map(c => [c.project_key.toUpperCase(), c])
  );
  let telemetryScores: any[] = [];
  let activeTelemetryTaskId = '';
  let isTelemetryDrawerOpen = false;

  let coreMembers = new Set([
    "梁志远", "朱家聪", "岳颖颖", "Yue Yingying", "姜昊良", "白凌云", "陈伟华", 
    "李厚奇", "鲁俊", "刘子翔", "张路路", "qiang.deng", "MiddleQ", "zhongkou.chang", 
    "Eddie", "Antigravity"
  ]);

  function isCoreMember(name: string): boolean {
    if (!name || name === '未指派' || name === '-' || name === 'Unassigned') return true;
    return coreMembers.has(name) || coreMembers.has(name.split(' ')[0]);
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

  function updateCoreMembers(config: any) {
    if (config.jira) {
      let users: string[] = [];
      if (config.jira.sync_users && config.jira.sync_users.length > 0) {
        users = [...config.jira.sync_users];
      } else if (config.jira.custom_jql) {
        const match = config.jira.custom_jql.match(/assignee\s+in\s*\(([^)]+)\)/i);
        if (match && match[1]) {
          users = match[1].split(',').map((name: string) => name.trim().replace(/['"]/g, ''));
        }
      }
      if (users.length > 0) {
        coreMembers = new Set(users);
      }
    }
  }

  async function fetchSystemConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const data = await res.json();
        if (data) {
          updateCoreMembers(data);
        }
      }
    } catch (e) {
      console.error('Failed to fetch system config in DemandKanban:', e);
    }
  }

  let scheduleScrollTop = 0;
  let scheduleContainerHeight = 550;
  let scheduleContainerEl: HTMLDivElement;
  const scheduleItemHeight = 76;
  const scheduleTableHeaderHeight = 38;
  const scheduleEmptyStateHeight = 96;

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

  $: flatRenderList = (() => {
    const sorted = [...filteredScheduleItems];
    sorted.sort((a, b) => compareScheduleItems(a, b));

    return sorted.map((item) => ({
      type: 'item',
      projectKey: item.project_key || getProjectKey(item.demand_id),
      data: item,
      id: item.demand_id
    }));
  })();

  $: scheduleStartIndex = Math.max(0, Math.floor(scheduleScrollTop / scheduleItemHeight) - 2);
  $: scheduleViewportHeight = scheduleContainerHeight > 0 ? scheduleContainerHeight : 550;
  $: scheduleVirtualRowCount = flatRenderList.length;
  $: scheduleTableContentHeight = scheduleTableHeaderHeight + (scheduleVirtualRowCount > 0 ? scheduleVirtualRowCount * scheduleItemHeight : scheduleEmptyStateHeight);
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
    return config ? config.project_name : '';
  }

  function getProjectDisplayName(projKey: string): string {
    if (!projKey || projKey === '-') return '暂不指定项目';
    const keyUpper = projKey.toUpperCase();
    const config = projectConfigMap.get(keyUpper);
    if (config && config.project_name && config.project_name !== `${projKey}项目`) {
      return `${config.project_name} (${projKey})`;
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
        projectConfigs = await res.json();
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
  let selectedDemand: Demand | null = null;
  let detailDemand: Demand | null = null;
  let manualModalScrollLocked = false;

  // Dropdown States
  let showAssigneeDropdown = false;
  let showProjectDropdown = false;
  let showScheduleAssigneeDropdown = false;
  let showScheduleDifficultyDropdown = false;
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
    return hasScheduleValue(demand.branch) && !!demand.due_date;
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
    showScheduleDifficultyDropdown = false;
    markScheduleEstimateManual();
  }

  function getScheduleDifficultyLabel() {
    return difficultyOptions.find((option) => option.value === schedDifficulty)?.label || '未设置';
  }

  function clearScheduleEstimate() {
    schedEstimateHours = 0;
    schedEstimateDays = 0;
    schedDifficulty = '';
    showScheduleDifficultyDropdown = false;
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

  function buildCreateProjectOptions(
    _optionProjects: string[],
    _demands: Demand[],
    _tasks: any[],
    _schedule: any[],
    _configs: any[],
    _scores: any[]
  ) {
    const options = new Set<string>();
    (_optionProjects || []).forEach((project) => addFormOption(options, project));
    (_configs || []).forEach((proj) => {
      if (proj) {
        if (proj.project_key) addFormOption(options, proj.project_key);
        if (proj.project_name) addFormOption(options, proj.project_name);
      }
    });
    (_scores || []).forEach((score) => {
      if (score) {
        if (score.project_key) addFormOption(options, score.project_key);
        if (score.project_name) addFormOption(options, score.project_name);
      }
    });
    return ['-', ...sortedFormOptions(options)];
  }

  async function toggleProjectDropdown() {
    showProjectDropdown = !showProjectDropdown;
    if (showProjectDropdown && createProjectOptions.length <= 1) {
      await fetchDemandOptions();
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

  function getCalendarDays(value: string) {
    const base = datePickerCursor;
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

  function moveDateMonth(delta: number) {
    datePickerCursor = new Date(datePickerCursor.getFullYear(), datePickerCursor.getMonth() + delta, 1);
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
  $: pendingDemands = demands.filter(d => isCoreMember(d.assignee) && d.status !== 'done' && (d.status === 'backlog' ? !isDemandFullyScheduled(d) : !hasScheduleValue(d.branch))).sort((a, b) => compareDemandsByPriority(a, b));
  $: scheduledDemands = demands.filter(d => isCoreMember(d.assignee) && d.status === 'backlog' && isDemandFullyScheduled(d)).sort((a, b) => compareDemandsByPriority(a, b));
  $: inProgressDemands = demands.filter(d => isCoreMember(d.assignee) && (d.status === 'progress' || d.status === 'review') && d.branch !== '' && d.branch !== '-').sort((a, b) => compareDemandsByPriority(a, b));
  $: deliveredDemands = demands.filter(d => isCoreMember(d.assignee) && d.status === 'done').sort((a, b) => compareDemandsByPriority(a, b));
  $: demandsById = new Map(demands.map((d) => [d.task_id, d]));
  $: createAssigneeOptions = buildCreateAssigneeOptions(coreMembers);
  $: createProjectOptions = buildCreateProjectOptions(
    demandOptionProjects,
    demands,
    allSubTasks,
    scheduleItems,
    projectConfigs,
    telemetryScores
  );
  $: scheduleAssigneeOptions = [...Array.from(coreMembers).sort((a, b) => a.localeCompare(b)), "外部协同"];
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

  $: filteredScheduleItems = scheduleItemsWithWeights.filter(item => {
    if (scheduleAssigneeFilter === '外部协同') return true;
    return isCoreMember(item.assignee);
  });
  $: if (!newAssignee && createAssigneeOptions.length > 0) {
    newAssignee = createAssigneeOptions[0];
  }
  $: if (!showProjectDropdown) projectSearchText = '';
  $: if (!showAssigneeDropdown) assigneeSearchText = '';
  $: if (!showScheduleAssigneeDropdown) scheduleAssigneeSearchText = '';
  $: if (!showScheduleProjectDropdown) scheduleProjectSearchText = '';
  $: {
    const shouldLock = showCreateModal || showScheduleModal || showConfirmModal || showDemandDetailsModal;
    if (shouldLock && !manualModalScrollLocked) {
      lockBodyScroll();
      manualModalScrollLocked = true;
    } else if (!shouldLock && manualModalScrollLocked) {
      unlockBodyScroll();
      manualModalScrollLocked = false;
    }
  }

  function setDemandView(view: DemandView) {
    activeDemandView = view;
    if (view === 'schedule') {
      fetchSchedule();
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

  function selectScheduleRiskFromCalendar(risk: ScheduleRiskFilter) {
    scheduleRiskFilter = risk;
    showRiskDropdown = false;
  }

  function formatRiskEventMeta(event: ScheduleRiskCalendarEvent): string {
    const dueText = event.due_date ? formatScheduleDate(event.due_date) : '未排期';
    return `${event.assignee || '未指派'} · ${dueText}`;
  }

  $: riskCalendarHasAny = riskCalendarBuckets.some((bucket) => bucket.counts.total > 0 || bucket.events.length > 0);

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
    loading = true;
    errorMsg = '';
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/tasks', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        // Filter only demands
        demands = (data || []).filter((t: any) => t.issue_type === 'demand' && t.status !== 'archived');
        // Extract all sub tasks
        allSubTasks = (data || []).filter((t: any) => t.issue_type !== 'demand' && t.status !== 'archived');
      } else {
        throw new Error('获取需求数据失败');
      }
    } catch (err: any) {
      errorMsg = err.message || '获取需求失败';
    } finally {
      loading = false;
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
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/demands/options', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data: DemandOptionsResponse = await res.json();
        demandOptionAssignees = data.assignees || [];
        demandOptionProjects = data.projects || [];
      }
    } catch (e) {
      console.error('Failed to fetch demand options:', e);
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
    fetchTelemetryScores();
  }

  function closeCreateDemandModal() {
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
    showScheduleDifficultyDropdown = false;
    showScheduleModal = true;
  }

  function closeScheduleModal() {
    showScheduleModal = false;
    selectedDemand = null;
    schedAssignee = '';
    schedAssigneeSearchText = '';
    showScheduleModalAssigneeDropdown = false;
    activeDatePicker = null;
    showScheduleDifficultyDropdown = false;
    scheduleEstimateLoading = false;
  }

  function handleScheduleModalClick(e: Event) {
    const target = e.target as HTMLElement;
    if (!target.closest('#sched-assignee-container')) {
      showScheduleModalAssigneeDropdown = false;
    }
    if (!target.closest('.difficulty-select-shell')) {
      showScheduleDifficultyDropdown = false;
    }
  }

  function openDemandDetails(demand: Demand) {
    detailDemand = demand;
    showDemandDetailsModal = true;
  }

  function handleDemandCardKeydown(event: KeyboardEvent, demand: Demand) {
    if (event.target !== event.currentTarget) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openDemandDetails(demand);
    }
  }

  function closeDemandDetails() {
    showDemandDetailsModal = false;
    detailDemand = null;
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
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ text: buildScheduleEstimateText() })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || 'AI 工时评估失败');
      }

      const data: DeconstructEstimateResponse = await res.json();
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
      scheduleEstimateError = (err.message || 'AI 工时评估失败').slice(0, 180);
    } finally {
      scheduleEstimateLoading = false;
    }
  }

  async function handleSaveSchedule() {
    if (!selectedDemand) return;

    const token = localStorage.getItem('jwt_token');
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
        closeScheduleModal();
        await refreshDemandWorkspace();
      } else {
        const errText = await res.text();
        alert(`排期失败: ${errText}`);
      }
    } catch (e: any) {
      console.error('Failed to save schedule:', e);
      alert(e.message || '网络连接错误，保存排期失败');
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

  function openScheduleFromItem(item: ScheduleItem) {
    const demand = demandsById.get(item.demand_id);
    if (demand) {
      openScheduleModal(demand);
    }
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
      showScheduleDifficultyDropdown = false;
      showRiskDropdown = false;
      showTypeDropdown = false;
      showSortDropdown = false;
    }
    if (!target.closest('.difficulty-select-shell')) {
      showScheduleDifficultyDropdown = false;
    }
    if (!target.closest('.date-input-shell')) {
      activeDatePicker = null;
    }
  }

  onMount(() => {
    isMounted = true;
    fetchDemands();
    fetchUsers();
    fetchDemandOptions();
    fetchJiraConfig();
    fetchProjectConfigs();
    fetchTelemetryScores();
    fetchSystemConfig();
    document.addEventListener('click', handleDocumentClick);

    const handleConfigUpdated = (e: Event) => {
      const customEvent = e as CustomEvent;
      const config = customEvent.detail;
      if (config) {
        updateCoreMembers(config);
      }
    };
    window.addEventListener('config-updated', handleConfigUpdated);

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
      window.removeEventListener('config-updated', handleConfigUpdated);
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

<div class="demand-dashboard font-sans">
  <div class="dashboard-header">
    <div>
      <span class="eyebrow">PRODUCT REQUIREMENTS ROADMAP</span>
      <h2>📋 需求流转与排期看板</h2>
    </div>

    {#if hasPermission('demands:write')}
      <button class="add-demand-btn font-mono" on:click={openCreateDemandModal}>
        ➕ 录入新需求
      </button>
    {/if}
  </div>

  <div class="demand-viewbar">
    <div class="view-toggle" role="tablist" aria-label="需求视图切换">
      <button class:active={activeDemandView === 'board'} on:click={() => setDemandView('board')}>流转看板</button>
      <button class:active={activeDemandView === 'schedule'} on:click={() => setDemandView('schedule')}>排期表</button>
    </div>
    <div class="viewbar-meta font-mono">
      {#if activeDemandView === 'schedule' && scheduleGeneratedAt}
        {scheduleGeneratedAt}
      {:else}
        {demands.length} active demands
      {/if}
    </div>
  </div>

  {#if loading && demands.length === 0 && activeDemandView === 'board'}
    <div class="state-msg">加载需求大盘中...</div>
  {:else if errorMsg && activeDemandView === 'board'}
    <div class="state-msg error-msg font-mono">❌ {errorMsg}</div>
  {:else if activeDemandView === 'schedule'}
    <div class="schedule-workbench">
      <div class="schedule-summary-grid">
        <div class="schedule-summary-cell">
          <span class="summary-label font-mono">TOTAL</span>
          <strong>{scheduleSummary.total}</strong>
          <em>需求总量</em>
        </div>
        <div class="schedule-summary-cell is-blue">
          <span class="summary-label font-mono">SCHEDULED</span>
          <strong>{scheduleSummary.scheduled}</strong>
          <em>已锁定排期</em>
        </div>
        <div class="schedule-summary-cell is-red">
          <span class="summary-label font-mono">OVERDUE</span>
          <strong>{scheduleSummary.overdue}</strong>
          <em>逾期风险</em>
        </div>
        <div class="schedule-summary-cell is-amber">
          <span class="summary-label font-mono">DUE SOON</span>
          <strong>{scheduleSummary.due_soon}</strong>
          <em>三日内到期</em>
        </div>
        <div class="schedule-summary-cell is-violet">
          <span class="summary-label font-mono">STALE</span>
          <strong>{scheduleSummary.stale}</strong>
          <em>推进滞后</em>
        </div>
      </div>

      <div class="schedule-risk-calendar-panel">
        <div class="risk-calendar-head">
          <div>
            <span class="eyebrow">RISK CALENDAR</span>
            <h3>排期风险日历</h3>
          </div>
          <div class="risk-calendar-meta font-mono">
            {#if riskCalendarLoading}
              <span>刷新中</span>
            {:else if riskCalendarUsingFallback}
              <span>本地排期兜底</span>
            {:else if riskCalendarGeneratedAt}
              <span>{riskCalendarGeneratedAt}</span>
            {/if}
          </div>
        </div>

        {#if riskCalendarErrorMsg && riskCalendarUsingFallback}
          <div class="risk-calendar-warning font-mono">{riskCalendarErrorMsg}，已使用当前排期表生成临时视图。</div>
        {/if}

        {#if riskCalendarLoading && !riskCalendarHasAny}
          <div class="risk-calendar-loading">
            <span></span>
            <span></span>
            <span></span>
          </div>
        {:else}
          <div class="risk-calendar-grid">
            {#each riskCalendarBuckets as bucket}
              <div class="risk-calendar-bucket">
                <div class="bucket-title-row">
                  <div>
                    <strong>{bucket.label}</strong>
                    <span class="font-mono">{bucket.window_label}</span>
                  </div>
                  <em class="font-mono">{bucket.counts.total}</em>
                </div>

                <div class="bucket-risk-chips">
                  {#each riskCalendarRiskTypes as risk}
                    {@const count = getBucketRiskCount(bucket, risk.value)}
                    <button
                      type="button"
                      class="bucket-risk-chip risk-{risk.value} {count > 0 ? 'has-count' : ''}"
                      on:click={() => selectScheduleRiskFromCalendar(risk.value)}
                      disabled={count === 0}
                    >
                      <span>{risk.label}</span>
                      <strong class="font-mono">{count}</strong>
                    </button>
                  {/each}
                </div>

                {#if bucket.events.length > 0}
                  <div class="bucket-event-list">
                    {#each bucket.events.slice(0, 3) as event}
                      <button
                        type="button"
                        class="bucket-event-item risk-{normalizeRiskLevel(event.risk_level)}"
                        on:click={() => selectScheduleRiskFromCalendar(getRiskFilterFromLevel(event.risk_level))}
                      >
                        <span class="event-task font-mono">{event.demand_id}</span>
                        <strong title={event.title}>{event.title}</strong>
                        <em class="font-mono">{formatRiskEventMeta(event)}</em>
                      </button>
                    {/each}
                  </div>
                {:else}
                  <div class="bucket-empty font-mono">暂无明细事件</div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="schedule-control-panel">
        <div class="schedule-search-shell">
          <span class="search-mark"></span>
          <input 
            type="text"
            placeholder="搜索需求、负责人、仓库、分支" 
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
              {#each projectConfigs.filter(p => !scheduleProjectSearchText || p.project_name.toLowerCase().includes(scheduleProjectSearchText.toLowerCase()) || p.project_key.toLowerCase().includes(scheduleProjectSearchText.toLowerCase())) as proj}
                <button
                  type="button"
                  class="dropdown-option-item {scheduleProjectFilter === proj.project_key ? 'selected' : ''}"
                  on:click={() => {
                    scheduleProjectFilter = proj.project_key;
                    showScheduleProjectDropdown = false;
                  }}
                >
                  {proj.project_name} ({proj.project_key})
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

        <button class="schedule-refresh-btn font-mono" class:is-loading={scheduleLoading} on:click={fetchSchedule}>
          刷新
        </button>
      </div>

      {#if scheduleLoading && scheduleItems.length === 0}
        <div class="state-msg">加载排期表中...</div>
      {:else if scheduleErrorMsg}
        <div class="state-msg error-msg font-mono">❌ {scheduleErrorMsg}</div>
      {:else}
        <div class="schedule-table-panel">
          <div class="schedule-table-head">
            <div>
              <span class="eyebrow">SCHEDULE WORKTABLE</span>
              <h3>需求排期总表</h3>
            </div>
            <span class="schedule-count font-mono">{filteredScheduleItems.length} / {scheduleItems.length}</span>
          </div>
          <div class="schedule-table-wrapper" style="--schedule-content-height: {scheduleTableContentHeight}px;" on:scroll={handleScheduleScroll} bind:this={scheduleContainerEl} bind:clientHeight={scheduleContainerHeight}>
            <div class="grid-table">
              <div class="grid-thead">
                <div class="grid-tr">
                  <div class="grid-th">需求</div>
                  <div class="grid-th">项目</div>
                  <div class="grid-th">负责人</div>
                  <div class="grid-th">排期</div>
                  <div class="grid-th">工时</div>
                  <div class="grid-th">影子任务</div>
                  <div class="grid-th">风险</div>
                  <div class="grid-th">创建时间</div>
                  <div class="grid-th">更新时间</div>
                  <div class="grid-th col-action">操作</div>
                </div>
              </div>
              <div class="grid-tbody">
                <div style="height: {scheduleTopPadding}px;"></div>
                {#each visibleScheduleRows as row (row.id)}
                  {@const item = row.data}
                  {@const progress = getScheduleProgress(item)}
                  {@const priority = item.project_priority || getProjectPriority(item.demand_id)}
                    <div class="grid-tr">
                      <div class="grid-td demand-cell">
                        <div class="demand-stack">
                          <div class="demand-badge-row">
                            {#if item.issue_type === 'bug'}
                              <span class="type-badge bug">🐛 缺陷</span>
                            {:else}
                              <span class="type-badge demand">📋 需求</span>
                            {/if}
                            {#if getJiraIssueUrl(item.demand_id)}
                              <a class="schedule-id font-mono jira-id-link" href={getJiraIssueUrl(item.demand_id)} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                                #{item.demand_id}
                              </a>
                            {:else}
                              <span class="schedule-id font-mono">#{item.demand_id}</span>
                            {/if}
                            {#if priority}
                              <span class="priority-badge p-{priority.toLowerCase()}">{priority}</span>
                            {/if}
                          </div>
                          <strong title={item.title}>{item.title}</strong>
                        </div>
                      </div>
                      <div class="grid-td project-cell">
                        <strong class="font-sans" style="font-size: 0.8rem; color: #94a3b8;">
                          {getProjectName(item.project_key || getProjectKey(item.demand_id)) || item.project_key || getProjectKey(item.demand_id)}
                        </strong>
                      </div>
                      <div class="grid-td">
                        <div class="owner-stack">
                          <strong>{item.assignee}</strong>
                        </div>
                      </div>
                      <div class="grid-td">
                        <div class="plan-stack">
                          <strong class="font-mono" style="font-size: 0.85rem; color: #f8fafc;">
                            {formatScheduleDate(item.due_date)}
                          </strong>
                          {#if !item.due_date}
                            <span class="status-chip status-{getScheduleStatusClass(item)}">{getScheduleStatusLabel(item)}</span>
                          {/if}
                        </div>
                      </div>
                      <div class="grid-td">
                        <div class="effort-stack">
                          <strong>{formatScheduleEffort(item)}</strong>
                        </div>
                      </div>
                      <div class="grid-td">
                        <div class="subtask-stack">
                          <div class="subtask-meter" title="影子任务进度: {item.subtask_done}/{item.subtask_total || 0}">
                            <span style="width: {progress}%"></span>
                            <small class="subtask-percentage-text">{item.subtask_done}/{item.subtask_total || 0}</small>
                          </div>
                        </div>
                      </div>
                      <div class="grid-td">
                        <div class="risk-stack">
                          <span class="schedule-risk-pill risk-{item.risk_level}" title={item.risk_reason || ''}>{item.risk_label}</span>
                        </div>
                      </div>
                      <div class="grid-td">
                        <span class="font-mono text-slate-400 text-xs" style="opacity: 0.85;">{item.created_at ? item.created_at.slice(0, 10) : '-'}</span>
                      </div>
                      <div class="grid-td">
                        <span class="font-mono text-slate-300 text-xs">{item.last_update ? item.last_update.slice(2, 16) : '-'}</span>
                      </div>
                      <div class="grid-td col-action">
                        <div class="schedule-actions-cell">
                          <button 
                            class="schedule-row-action" 
                            disabled={item.issue_type === 'bug' || item.issue_type === '缺陷' || item.issue_type === '故障' || item.issue_type === 'defect' || !canEditScheduleItem(item)}
                            on:click={() => openScheduleFromItem(item)}
                          >
                            调整
                          </button>
                          <button class="schedule-row-action is-telemetry font-mono" on:click|stopPropagation={() => {
                            activeTelemetryTaskId = item.demand_id;
                            isTelemetryDrawerOpen = true;
                          }}>轨迹</button>
                        </div>
                      </div>
                    </div>
                {/each}
                <div style="height: {scheduleBottomPadding}px;"></div>
              </div>
            </div>
            {#if filteredScheduleItems.length === 0}
              <div class="schedule-empty-state font-mono">当前筛选下暂无排期数据</div>
            {/if}
          </div>
        </div>
      {/if}
    </div>
  {:else}
    <!-- Kanban Lanes -->
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
    <div class="modal-backdrop" on:click={handleBackdropClick}>
      <div class="modal-content demand-create-modal glass-panel">
        <div class="modal-header">
          <h3>📋 录入新产品需求</h3>
          <button class="close-btn" on:click={closeCreateDemandModal}>&times;</button>
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
                  on:keydown={(e) => {
                    if (e.key === 'Enter' && projectSearchText.trim()) {
                      newRepo = projectSearchText.trim();
                      showProjectDropdown = false;
                    }
                  }}
                />
                <span class="arrow-icon {showProjectDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showProjectDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#if projectSearchText && !createProjectOptions.some(proj => getProjectDisplayName(proj).toLowerCase() === projectSearchText.toLowerCase())}
                    <button
                      type="button"
                      class="dropdown-option-item new-custom-option"
                      style="color: #818cf8; font-weight: 500; border-bottom: 1px solid rgba(129, 140, 248, 0.15);"
                      on:click|stopPropagation={() => {
                        newRepo = projectSearchText.trim();
                        showProjectDropdown = false;
                      }}
                    >
                      ➕ 使用新项目: "{projectSearchText}"
                    </button>
                  {/if}
                  {#each (projectSearchText && projectSearchText.trim() ? createProjectOptions.filter(proj => getProjectDisplayName(proj).toLowerCase().includes(projectSearchText.trim().toLowerCase())) : createProjectOptions) as project}
                    <button
                      type="button"
                      class="dropdown-option-item {newRepo === project ? 'selected' : ''}"
                      on:click|stopPropagation={() => {
                        newRepo = project;
                        showProjectDropdown = false;
                      }}
                    >
                      {getProjectDisplayName(project)}
                    </button>
                  {/each}
                  {#if createProjectOptions.length <= 1 && !projectSearchText}
                    <div class="dropdown-empty">暂无项目候选，请先配置项目或等待 Jira 同步。</div>
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
                {@const calendar = getCalendarDays(newDueDate)}
                <div class="date-picker-panel">
                  <div class="date-picker-head">
                    <button type="button" on:click={() => moveDateMonth(-1)} aria-label="上个月">‹</button>
                    <strong>{calendar.label}</strong>
                    <button type="button" on:click={() => moveDateMonth(1)} aria-label="下个月">›</button>
                  </div>
                  <div class="date-week-grid font-mono">
                    {#each weekdayNames as day}<span>{day}</span>{/each}
                  </div>
                  <div class="date-grid">
                    {#each calendar.days as day}
                      <button
                        type="button"
                        class="date-cell {day.muted ? 'muted' : ''} {day.today ? 'today' : ''} {day.selected ? 'selected' : ''}"
                        on:click={() => selectDate('new', day.value)}
                      >
                        {day.label}
                      </button>
                    {/each}
                  </div>
                  <div class="date-picker-foot">
                    <button type="button" on:click={() => clearDate('new')}>清空日期</button>
                    <button type="button" on:click={() => selectDate('new', toDateValue(new Date()))}>今天</button>
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="cancel-btn font-mono" on:click={closeCreateDemandModal}>取消</button>
          <button class="submit-btn font-mono" on:click={handleCreateDemand}>指派需求</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Schedule Demand Modal -->
  {#if showScheduleModal && selectedDemand}
    <div class="modal-backdrop" on:click={closeScheduleModal}>
      <div class="modal-content schedule-modal" on:click|stopPropagation={handleScheduleModalClick}>
        <div class="modal-header">
          <h3>⚡ {selectedDemand.issue_type === 'bug' ? '缺陷' : '需求'}开发排期与指派: #{selectedDemand.task_id}</h3>
          <button class="close-btn" on:click={closeScheduleModal}>&times;</button>
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
                <button
                  id="sched-difficulty"
                  type="button"
                  class="difficulty-trigger {schedDifficulty ? 'has-value' : ''}"
                  on:click|stopPropagation={() => showScheduleDifficultyDropdown = !showScheduleDifficultyDropdown}
                  aria-haspopup="listbox"
                  aria-expanded={showScheduleDifficultyDropdown}
                  aria-labelledby="sched-difficulty-label"
                >
                  <span>{getScheduleDifficultyLabel()}</span>
                  <span class="difficulty-caret">▼</span>
                </button>
                {#if showScheduleDifficultyDropdown}
                  <div class="difficulty-options" role="listbox" aria-labelledby="sched-difficulty-label">
                    {#each difficultyOptions as option}
                      <button
                        type="button"
                        role="option"
                        aria-selected={schedDifficulty === option.value}
                        class:selected={schedDifficulty === option.value}
                        on:click|stopPropagation={() => updateScheduleDifficulty(option.value)}
                      >
                        {option.label}
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>
            {#if scheduleEstimateError}
              <p class="estimate-error">{scheduleEstimateError}</p>
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
                {@const calendar = getCalendarDays(schedDueDate)}
                <div class="date-picker-panel">
                  <div class="date-picker-head">
                    <button type="button" on:click={() => moveDateMonth(-1)} aria-label="上个月">‹</button>
                    <strong>{calendar.label}</strong>
                    <button type="button" on:click={() => moveDateMonth(1)} aria-label="下个月">›</button>
                  </div>
                  <div class="date-week-grid font-mono">
                    {#each weekdayNames as day}<span>{day}</span>{/each}
                  </div>
                  <div class="date-grid">
                    {#each calendar.days as day}
                      <button
                        type="button"
                        class="date-cell {day.muted ? 'muted' : ''} {day.today ? 'today' : ''} {day.selected ? 'selected' : ''}"
                        on:click={() => selectDate('schedule', day.value)}
                      >
                        {day.label}
                      </button>
                    {/each}
                  </div>
                  <div class="date-picker-foot">
                    <button type="button" on:click={() => clearDate('schedule')}>清空日期</button>
                    <button type="button" on:click={() => selectDate('schedule', toDateValue(new Date()))}>今天</button>
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
          <button class="cancel-btn font-mono" on:click={closeScheduleModal}>取消</button>
          <button class="submit-btn font-mono" on:click={handleSaveSchedule}>确认排期</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Demand Details Modal -->
  {#if showDemandDetailsModal && detailDemand}
    <div class="modal-backdrop" on:click={closeDemandDetails}>
      <div class="modal-content detail-modal" on:click|stopPropagation>
        <div class="modal-header">
          <h3>需求详情: #{detailDemand.task_id}</h3>
          <button class="close-btn" on:click={closeDemandDetails} aria-label="关闭详情弹窗">&times;</button>
        </div>

        <div class="detail-body">
          <div class="detail-title-block">
            {#if getJiraIssueUrl(detailDemand.task_id)}
              <a class="detail-id font-mono jira-id-link" href={getJiraIssueUrl(detailDemand.task_id)} target="_blank" rel="noopener noreferrer">
                #{detailDemand.task_id}
              </a>
            {:else}
              <span class="detail-id font-mono">#{detailDemand.task_id}</span>
            {/if}
            <strong>{detailDemand.title}</strong>
            <p>{detailDemand.description || '暂无需求说明'}</p>
          </div>

          <div class="detail-facts-grid">
            <div>
              <span>负责人</span>
              <strong>{detailDemand.assignee || '未指派'}</strong>
            </div>
            <div>
              <span>流转状态</span>
              <strong>{detailDemand.status || '-'}</strong>
            </div>
            <div>
              <span>截止日期</span>
              <strong>{detailDemand.due_date ? formatDateDisplay(detailDemand.due_date.slice(0, 10)) : '未排期'}</strong>
            </div>
            <div>
              <span>提单人</span>
              <strong>{detailDemand.creator || '系统'}</strong>
            </div>
            <div>
              <span>所属部门</span>
              <strong>{detailDemand.creator_dept || '无部门'}</strong>
            </div>
            <div>
              <span>任务组</span>
              <strong>{detailDemand.task_group_id || getEffectiveTaskGroupId(detailDemand)}</strong>
            </div>
          </div>

          <div class="detail-link-row">
            {#if isAssignee(detailDemand) || hasPermission('demands:write') || canManageDemand(detailDemand)}
              <button type="button" on:click={() => { if (detailDemand) { closeDemandDetails(); openScheduleModal(detailDemand); } }}>调整排期</button>
            {/if}
          </div>

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
                    <em>{sub.assignee || '未指派'} · {sub.status || '-'}</em>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="detail-empty">尚未导入 AI 影子任务。</div>
            {/if}
          </div>
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
      border-left: 4px solid rgba(239, 68, 68, 0.6);
      box-shadow: 0 0 4px rgba(239, 68, 68, 0.1);
    }
    50% {
      background: rgba(239, 68, 68, 0.12);
      border-left: 4px solid rgba(239, 68, 68, 1.0);
      box-shadow: 0 0 12px rgba(239, 68, 68, 0.3);
    }
    100% {
      background: rgba(239, 68, 68, 0.05);
      border-left: 4px solid rgba(239, 68, 68, 0.6);
      box-shadow: 0 0 4px rgba(239, 68, 68, 0.1);
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
    border-left: 4px solid rgba(249, 115, 22, 0.8);
    box-shadow: 0 0 8px rgba(249, 115, 22, 0.2);
  }

  .project-group-header.pg-p2 {
    background: rgba(245, 158, 11, 0.05);
    border-left: 4px solid rgba(245, 158, 11, 0.7);
    box-shadow: 0 0 6px rgba(245, 158, 11, 0.15);
  }

  .project-group-header.pg-p3 {
    border-left: 4px solid #3b82f6;
    background: rgba(30, 41, 59, 0.5);
  }
  .project-group-header.pg-p4 {
    border-left: 4px solid #6366f1;
    background: rgba(30, 41, 59, 0.4);
  }
  .project-group-header.pg-p5 {
    border-left: 4px solid #94a3b8;
    background: rgba(30, 41, 59, 0.3);
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
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
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

  .demand-viewbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 14px;
    flex-wrap: wrap;
    background: rgba(15, 23, 42, 0.38);
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 10px;
    padding: 8px;
  }

  .view-toggle,
  .schedule-filter-strip,
  .schedule-sort-strip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: rgba(2, 6, 23, 0.32);
    border: 1px solid rgba(51, 65, 85, 0.38);
    border-radius: 8px;
    padding: 4px;
  }

  .view-toggle button,
  .schedule-filter-strip button,
  .schedule-sort-strip button {
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

  .view-toggle button.active,
  .schedule-filter-strip button.active,
  .schedule-sort-strip button.active {
    color: #f8fafc;
    background: rgba(99, 102, 241, 0.48);
  }

  .viewbar-meta {
    color: #64748b;
    font-size: 0.68rem;
    padding: 0 4px;
  }

  .schedule-workbench {
    display: flex;
    flex-direction: column;
    gap: 14px;
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
    border-top: 3px solid rgba(100, 116, 139, 0.72);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .schedule-summary-cell.is-blue { border-top-color: #38bdf8; }
  .schedule-summary-cell.is-red { border-top-color: #ef4444; }
  .schedule-summary-cell.is-amber { border-top-color: #f59e0b; }
  .schedule-summary-cell.is-violet { border-top-color: #a78bfa; }

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
    background: rgba(10, 15, 30, 0.62);
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 12px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
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
    font-size: 0.98rem;
  }

  .risk-calendar-meta {
    color: #64748b;
    font-size: 0.68rem;
    white-space: nowrap;
  }

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
    gap: 10px;
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
    background: rgba(15, 23, 42, 0.62);
    border: 1px solid rgba(71, 85, 105, 0.34);
    border-radius: 10px;
    padding: 11px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .bucket-title-row {
    display: flex;
    justify-content: space-between;
    gap: 10px;
    align-items: flex-start;
  }

  .bucket-title-row strong {
    display: block;
    color: #f8fafc;
    font-size: 0.9rem;
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
    font-size: 1.15rem;
    font-variant-numeric: tabular-nums;
  }

  .bucket-risk-chips {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 6px;
  }

  .bucket-risk-chip {
    border: 1px solid rgba(71, 85, 105, 0.38);
    background: rgba(2, 6, 23, 0.28);
    border-radius: 7px;
    color: #64748b;
    padding: 7px 6px;
    cursor: pointer;
    min-width: 0;
    transition: border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
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
  }

  .bucket-event-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }

  .bucket-event-item {
    width: 100%;
    text-align: left;
    border: 1px solid rgba(51, 65, 85, 0.34);
    background: rgba(2, 6, 23, 0.22);
    border-left: 3px solid rgba(100, 116, 139, 0.72);
    border-radius: 7px;
    padding: 7px 8px;
    color: #cbd5e1;
    cursor: pointer;
    min-width: 0;
  }

  .bucket-event-item.risk-overdue { border-left-color: #f43f5e; }
  .bucket-event-item.risk-due_soon { border-left-color: #f59e0b; }
  .bucket-event-item.risk-stale { border-left-color: #a78bfa; }
  .bucket-event-item.risk-unscheduled { border-left-color: #60a5fa; }

  .bucket-event-item:hover {
    background: rgba(30, 41, 59, 0.54);
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
    transition: height 0.16s ease, max-height 0.16s ease;
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
  }

  @media (max-width: 760px) {
    .schedule-summary-grid {
      grid-template-columns: repeat(2, minmax(120px, 1fr));
    }

    .risk-calendar-head {
      flex-direction: column;
      align-items: flex-start;
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
    .schedule-sort-strip button,
    .view-toggle button {
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
  .estimate-reference-field strong,
  .difficulty-trigger {
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

  .difficulty-trigger {
    cursor: pointer;
    padding: 0 9px 0 10px;
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    color: #94a3b8;
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.62), rgba(2, 6, 23, 0.24));
  }

  .difficulty-trigger.has-value {
    color: #e2e8f0;
  }

  .difficulty-caret {
    color: #64748b;
    font-size: 0.58rem;
    line-height: 1;
  }

  .estimate-field input:focus,
  .difficulty-trigger:focus {
    border-color: rgba(129, 140, 248, 0.78);
    background-color: rgba(15, 23, 42, 0.66);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.14);
  }

  .difficulty-options {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 1500;
    width: min(136px, 100%);
    box-sizing: border-box;
    padding: 4px;
    background: #0b1220;
    border: 1px solid rgba(71, 85, 105, 0.74);
    border-radius: 9px;
    box-shadow: 0 18px 42px rgba(2, 6, 23, 0.62);
  }

  .difficulty-options button {
    width: 100%;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: #94a3b8;
    padding: 7px 8px;
    text-align: left;
    font: inherit;
    font-size: 0.76rem;
    font-weight: 800;
    cursor: pointer;
  }

  .difficulty-options button:hover,
  .difficulty-options button.selected {
    color: #ffffff;
    background: rgba(99, 102, 241, 0.18);
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
    min-width: 0;
    max-width: 100%;
    box-sizing: border-box;
    margin: 0;
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
    max-width: 640px;
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
    transition: width 0.3s ease;
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
    border-left: 4px solid #6366f1;
    border-radius: 8px;
    padding: 14px 18px;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
  }
  .diagnostic-bubble.score-excellent {
    border-left-color: #10b981;
    background: rgba(16, 185, 129, 0.03);
    border-color: rgba(16, 185, 129, 0.15);
  }
  .diagnostic-bubble.score-good {
    border-left-color: #f59e0b;
    background: rgba(245, 158, 11, 0.03);
    border-color: rgba(245, 158, 11, 0.15);
  }
  .diagnostic-bubble.score-risk {
    border-left-color: #ef4444;
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
    transition: width 0.5s cubic-bezier(0.4, 0, 0.2, 1);
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
</style>
