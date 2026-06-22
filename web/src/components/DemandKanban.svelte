<script lang="ts">
  import { onMount } from 'svelte';

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
  }

  type DemandView = 'board' | 'schedule';
  type ScheduleRiskFilter = 'attention' | 'all' | 'overdue' | 'due_soon' | 'stale' | 'unscheduled' | 'safe' | 'done';
  type ScheduleSortMode = 'risk' | 'due' | 'owner';

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
  }

  interface ScheduleResponse {
    generated_at: string;
    summary: ScheduleSummary;
    items: ScheduleItem[];
  }

  interface DemandOptionsResponse {
    assignees: string[];
    projects: string[];
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

  const scheduleSortModes: Array<{ value: ScheduleSortMode; label: string }> = [
    { value: 'risk', label: '风险优先' },
    { value: 'due', label: '截止日' },
    { value: 'owner', label: '负责人' }
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
    return allSubTasks.filter(t => t.task_group_id === taskGroupId);
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
  let scheduleSearch = '';
  let scheduleRiskFilter: ScheduleRiskFilter = 'attention';
  let scheduleAssigneeFilter = 'all';
  let scheduleSortMode: ScheduleSortMode = 'risk';

  // Modal States
  let showCreateModal = false;
  let showScheduleModal = false;
  let selectedDemand: Demand | null = null;

  // Dropdown States
  let showAssigneeDropdown = false;
  let showProjectDropdown = false;
  let showTaskGroupDropdown = false;
  let showScheduleAssigneeDropdown = false;
  let activeDatePicker: 'new' | 'schedule' | null = null;
  let datePickerCursor = new Date();
  let taskGroups: string[] = [];

  // Create Form Fields
  let newTitle = '';
  let newDescription = '';
  let newAssignee = '';
  let newRepo = '-';
  let newDueDate = '';
  let newDueDateDisplay = '';

  // Schedule Form Fields
  let schedBranch = '';
  let schedDueDate = '';
  let schedDueDateDisplay = '';
  let schedTaskGroupID = '-';

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

  function getScheduleTaskGroupOptions() {
    const options = new Set<string>();
    if (hasTaskGroup(schedTaskGroupID)) options.add(schedTaskGroupID);
    taskGroups.forEach(groupId => {
      if (hasTaskGroup(groupId)) options.add(groupId);
    });
    return Array.from(options);
  }

  function displayRepo(repo?: string) {
    if (!repo || repo === '-' || repo === 'unassigned') return 'AI 尚未映射仓库';
    return repo;
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

  function buildCreateAssigneeOptions() {
    const options = new Set<string>();
    demandOptionAssignees.forEach((name) => addFormOption(options, name));
    users.forEach((user) => addFormOption(options, user.name || user.username));
    demands.forEach((demand) => addFormOption(options, demand.assignee));
    allSubTasks.forEach((task) => addFormOption(options, task.assignee));
    scheduleItems.forEach((item) => addFormOption(options, item.assignee));
    addFormOption(options, currentUserName || currentUserEmail);
    return sortedFormOptions(options);
  }

  function buildCreateProjectOptions() {
    const options = new Set<string>();
    demandOptionProjects.forEach((project) => addFormOption(options, project));
    demands.forEach((demand) => addFormOption(options, demand.repo));
    allSubTasks.forEach((task) => addFormOption(options, task.repo));
    scheduleItems.forEach((item) => addFormOption(options, item.repo));
    return ['-', ...sortedFormOptions(options)];
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
  $: pendingDemands = demands.filter(d => d.status !== 'done' && (d.branch === '' || d.branch === '-'));
  $: scheduledDemands = demands.filter(d => d.status === 'backlog' && d.branch !== '' && d.branch !== '-');
  $: inProgressDemands = demands.filter(d => (d.status === 'progress' || d.status === 'review') && d.branch !== '' && d.branch !== '-');
  $: deliveredDemands = demands.filter(d => d.status === 'done');
  $: demandsById = new Map(demands.map((d) => [d.task_id, d]));
  $: createAssigneeOptions = buildCreateAssigneeOptions();
  $: createProjectOptions = buildCreateProjectOptions();
  $: scheduleAssigneeOptions = Array.from(new Set(scheduleItems.map((item) => item.assignee).filter(Boolean))).sort((a, b) => a.localeCompare(b));
  $: filteredScheduleItems = scheduleItems
    .filter((item) => matchesScheduleFilters(item))
    .sort((a, b) => compareScheduleItems(a, b));
  $: if (!newAssignee && createAssigneeOptions.length > 0) {
    newAssignee = createAssigneeOptions[0];
  }

  function setDemandView(view: DemandView) {
    activeDemandView = view;
    if (view === 'schedule') {
      fetchSchedule();
    }
  }

  function matchesScheduleFilters(item: ScheduleItem): boolean {
    const query = scheduleSearch.trim().toLowerCase();
    if (query) {
      const haystack = [
        item.demand_id,
        item.title,
        item.description,
        item.assignee,
        item.department,
        item.repo,
        item.branch,
        item.task_group_id,
        item.risk_label
      ].join(' ').toLowerCase();
      if (!haystack.includes(query)) return false;
    }

    if (scheduleAssigneeFilter !== 'all' && item.assignee !== scheduleAssigneeFilter) {
      return false;
    }

    if (scheduleRiskFilter === 'attention') {
      return item.risk_level !== 'safe' && item.risk_level !== 'done';
    }
    if (scheduleRiskFilter !== 'all') {
      return item.risk_level === scheduleRiskFilter;
    }
    return true;
  }

  function compareScheduleItems(a: ScheduleItem, b: ScheduleItem): number {
    if (scheduleSortMode === 'owner') {
      const ownerCompare = a.assignee.localeCompare(b.assignee);
      if (ownerCompare !== 0) return ownerCompare;
      return a.demand_id.localeCompare(b.demand_id);
    }
    if (scheduleSortMode === 'due') {
      if (a.due_date !== b.due_date) {
        if (!a.due_date) return 1;
        if (!b.due_date) return -1;
        return a.due_date.localeCompare(b.due_date);
      }
      return b.risk_rank - a.risk_rank;
    }
    if (a.risk_rank !== b.risk_rank) return b.risk_rank - a.risk_rank;
    if (a.due_date !== b.due_date) {
      if (!a.due_date) return 1;
      if (!b.due_date) return -1;
      return a.due_date.localeCompare(b.due_date);
    }
    return a.demand_id.localeCompare(b.demand_id);
  }

  async function fetchSchedule() {
    scheduleLoading = true;
    scheduleErrorMsg = '';
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/schedule', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) {
        throw new Error('获取排期表失败');
      }
      const data: ScheduleResponse = await res.json();
      scheduleItems = data.items || [];
      scheduleSummary = data.summary || createEmptyScheduleSummary();
      scheduleGeneratedAt = data.generated_at || '';
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
        
        // Extract unique, non-empty task_group_id values
        const groupsSet = new Set<string>();
        (data || []).forEach((t: any) => {
          if (t.task_group_id && t.task_group_id !== '-' && t.task_group_id !== '') {
            groupsSet.add(t.task_group_id);
          }
        });
        taskGroups = Array.from(groupsSet);
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

  function openCreateDemandModal() {
    showCreateModal = true;
    showAssigneeDropdown = false;
    showProjectDropdown = false;
    activeDatePicker = null;
    fetchDemandOptions();
  }

  function closeCreateDemandModal() {
    showCreateModal = false;
    showAssigneeDropdown = false;
    showProjectDropdown = false;
    activeDatePicker = null;
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

  function openScheduleModal(demand: Demand) {
    selectedDemand = demand;
    schedBranch = demand.branch === '-' ? '' : demand.branch;
    updateSchedDueDate(demand.due_date ? demand.due_date.slice(0, 10) : '');
    schedTaskGroupID = getEffectiveTaskGroupId(demand);
    activeDatePicker = null;
    showScheduleModal = true;
  }

  async function handleSaveSchedule() {
    if (!selectedDemand) return;
    if (!schedBranch.trim()) {
      alert('排期必须填写关联的分支名称');
      return;
    }

    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/tasks/schedule', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          task_id: selectedDemand.task_id,
          branch: schedBranch,
          due_date: schedDueDate,
          status: 'backlog', // Scheduled demands go to backlog in kanban
          task_group_id: schedTaskGroupID
        })
      });

      if (res.ok) {
        showScheduleModal = false;
        selectedDemand = null;
        await refreshDemandWorkspace();
      } else {
        const errText = await res.text();
        alert(`排期失败: ${errText}`);
      }
    } catch (e) {
      console.error('Failed to save schedule:', e);
      alert('排期请求发送失败');
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
    if (!value) return '-';
    return formatDateDisplay(value.slice(0, 10));
  }

  function getScheduleStatusLabel(status: string): string {
    switch (status) {
      case 'backlog':
        return '已排期';
      case 'progress':
        return '开发中';
      case 'review':
        return '评审中';
      case 'done':
        return '已交付';
      default:
        return status || '未知';
    }
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
    if (!target.closest('.custom-dropdown-container')) {
      showAssigneeDropdown = false;
      showProjectDropdown = false;
      showTaskGroupDropdown = false;
      showScheduleAssigneeDropdown = false;
    }
    if (!target.closest('.date-input-shell')) {
      activeDatePicker = null;
    }
  }

  onMount(() => {
    fetchDemands();
    fetchUsers();
    fetchDemandOptions();
    document.addEventListener('click', handleDocumentClick);
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
    };
  });
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

      <div class="schedule-control-panel">
        <div class="schedule-search-shell">
          <span class="search-mark"></span>
          <input bind:value={scheduleSearch} placeholder="搜索需求、负责人、仓库、分支" />
        </div>

        <div class="schedule-filter-strip">
          {#each scheduleRiskFilters as filter}
            <button
              class:active={scheduleRiskFilter === filter.value}
              on:click={() => scheduleRiskFilter = filter.value}
            >
              {filter.label}
            </button>
          {/each}
        </div>

        <div class="schedule-assignee-menu custom-dropdown-container">
          <button class="schedule-menu-trigger" on:click|stopPropagation={() => showScheduleAssigneeDropdown = !showScheduleAssigneeDropdown}>
            <span>{scheduleAssigneeFilter === 'all' ? '全部负责人' : scheduleAssigneeFilter}</span>
            <span class="arrow-icon {showScheduleAssigneeDropdown ? 'open' : ''}">▼</span>
          </button>
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
              {#each scheduleAssigneeOptions as assignee}
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

        <div class="schedule-sort-strip">
          {#each scheduleSortModes as mode}
            <button
              class:active={scheduleSortMode === mode.value}
              on:click={() => scheduleSortMode = mode.value}
            >
              {mode.label}
            </button>
          {/each}
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
          <div class="schedule-table-wrapper">
            <table class="schedule-table">
              <thead>
                <tr>
                  <th>需求</th>
                  <th>负责人</th>
                  <th>排期</th>
                  <th>交付窗口</th>
                  <th>影子任务</th>
                  <th>风险</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {#each filteredScheduleItems as item}
                  {@const progress = getScheduleProgress(item)}
                  <tr>
                    <td class="demand-cell">
                      <div class="demand-stack">
                        <span class="schedule-id font-mono">#{item.demand_id}</span>
                        <strong>{item.title}</strong>
                        <small>{item.description || '暂无需求说明'}</small>
                      </div>
                    </td>
                    <td>
                      <div class="owner-stack">
                        <strong>{item.assignee}</strong>
                        <span>{item.department}</span>
                      </div>
                    </td>
                    <td>
                      <div class="branch-stack">
                        <span class="status-chip status-{item.status}">{getScheduleStatusLabel(item.status)}</span>
                        <strong class="font-mono">{item.branch && item.branch !== '-' ? item.branch : '未绑定分支'}</strong>
                        <span>{item.repo && item.repo !== '-' ? item.repo : '未映射仓库'}</span>
                      </div>
                    </td>
                    <td>
                      <div class="due-stack">
                        <strong>{formatScheduleDue(item)}</strong>
                        <span class="font-mono">{formatScheduleDate(item.due_date)}</span>
                        <small>更新 {item.last_update || '-'}</small>
                      </div>
                    </td>
                    <td>
                      <div class="subtask-stack">
                        <div class="subtask-meter">
                          <span style="width: {progress}%"></span>
                        </div>
                        <strong>{item.subtask_done}/{item.subtask_total || 0}</strong>
                        <small>{item.task_group_id || '未绑定任务组'}</small>
                      </div>
                    </td>
                    <td>
                      <div class="risk-stack">
                        <span class="schedule-risk-pill risk-{item.risk_level}">{item.risk_label}</span>
                        <small>{item.risk_reason}</small>
                      </div>
                    </td>
                    <td>
                      {#if canEditScheduleItem(item)}
                        <button class="schedule-row-action" on:click={() => openScheduleFromItem(item)}>调整</button>
                      {:else}
                        <span class="schedule-row-muted font-mono">READ</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
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
            <div class="demand-card border-orange-dim">
              <div class="card-top">
                <span class="demand-id font-mono">#{item.task_id}</span>
                <div class="card-actions">
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
                <span class="brain-link-badge font-mono">{getBrainBindingState(item)}</span>
                {#if isAssignee(item) || hasPermission('demands:write')}
                  <button class="action-btn schedule-btn" on:click={() => openScheduleModal(item)}>
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
            <div class="demand-card border-blue-dim">
              <div class="card-top">
                <span class="demand-id font-mono">#{item.task_id}</span>
                <div class="card-actions">
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
                <span class="due-badge {dueInfo.className} font-mono">{dueInfo.text}</span>
                {#if isAssignee(item) || hasPermission('demands:write')}
                  <button class="edit-sched-btn" on:click={() => openScheduleModal(item)}>
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
            <div class="demand-card border-purple-dim">
              <div class="card-top">
                <span class="demand-id font-mono">#{item.task_id}</span>
                <div class="card-actions">
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
                <span class="due-badge {dueInfo.className} font-mono">{dueInfo.text}</span>
                <span class="status-badge font-mono">{item.status === 'review' ? '👀评审中' : '💻进行中'}</span>
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
            <div class="demand-card border-green-dim card-done">
              <div class="card-top">
                <span class="demand-id font-mono">#{item.task_id}</span>
                <div class="card-actions">
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
                <span class="done-tag font-mono">🎉 已发布</span>
                {#if item.completed_at}
                  <span class="done-date font-mono">{new Date(item.completed_at).toLocaleDateString()}</span>
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
    <div class="modal-backdrop" on:click={closeCreateDemandModal}>
      <div class="modal-content demand-create-modal glass-panel" on:click|stopPropagation>
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
            <label for="demand-project">所属项目 / 仓库</label>
            <div class="custom-dropdown-container" id="demand-project-container">
              <button
                type="button"
                class="dropdown-trigger"
                on:click|stopPropagation={() => showProjectDropdown = !showProjectDropdown}
              >
                <span>{displayProjectOption(newRepo)}</span>
                <span class="arrow-icon {showProjectDropdown ? 'open' : ''}">▼</span>
              </button>
              {#if showProjectDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each createProjectOptions as project}
                    <button
                      type="button"
                      class="dropdown-option-item {newRepo === project ? 'selected' : ''}"
                      on:click={() => {
                        newRepo = project;
                        showProjectDropdown = false;
                      }}
                    >
                      {displayProjectOption(project)}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
            <span class="field-hint">用于后续排期、代码证据和需求归属聚合；口头需求可先暂不指定。</span>
          </div>

          <div class="form-group">
            <label for="demand-assignee">指派负责人 <span class="text-rose">*</span></label>
            <div class="custom-dropdown-container" id="demand-assignee-container">
              <button
                type="button"
                class="dropdown-trigger" 
                on:click|stopPropagation={() => showAssigneeDropdown = !showAssigneeDropdown}
              >
                <span>{newAssignee || '请选择负责人'}</span>
                <span class="arrow-icon {showAssigneeDropdown ? 'open' : ''}">▼</span>
              </button>
              {#if showAssigneeDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each createAssigneeOptions as assignee}
                    <button
                      type="button"
                      class="dropdown-option-item {newAssignee === assignee ? 'selected' : ''}"
                      on:click={() => {
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
    <div class="modal-backdrop" on:click={() => showScheduleModal = false}>
      <div class="modal-content glass-panel" on:click|stopPropagation>
        <div class="modal-header">
          <h3>⚡ 需求开发排期: #{selectedDemand.task_id}</h3>
          <button class="close-btn" on:click={() => showScheduleModal = false}>&times;</button>
        </div>

        <div class="form-body">
          <p class="demand-brief font-mono">标题: {selectedDemand.title}</p>
          
          <div class="form-group">
            <label for="sched-branch">关联开发分支 <span class="text-rose">*</span></label>
            <input type="text" id="sched-branch" bind:value={schedBranch} placeholder="例如：feat/demand-department-field" />
            <span class="help-text">⚠️ 关联分支后，当您向该分支提交 commit 时，系统将自动进行卡点诊断和进度追溯。</span>
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
            <div class="brain-link-note">
              <span class="brain-link-note-label font-mono">BRAIN LINK</span>
              <span>排期会把该需求绑定到此任务组，后续 AI 需求解构同步影子任务时会复用同一组。</span>
            </div>
            <div class="custom-dropdown-container" id="sched-task-group-container">
              <div 
                class="dropdown-trigger" 
                on:click|stopPropagation={() => showTaskGroupDropdown = !showTaskGroupDropdown}
              >
                <span>{schedTaskGroupID === '-' ? '不关联任务组' : schedTaskGroupID}</span>
                <span class="arrow-icon {showTaskGroupDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showTaskGroupDropdown}
                <div class="dropdown-options-list glass-panel">
                  <div 
                    class="dropdown-option-item {schedTaskGroupID === '-' ? 'selected' : ''}"
                    on:click={() => {
                      schedTaskGroupID = '-';
                      showTaskGroupDropdown = false;
                    }}
                  >
                    不关联任务组
                  </div>
                  {#each getScheduleTaskGroupOptions() as gId}
                    <div 
                      class="dropdown-option-item {schedTaskGroupID === gId ? 'selected' : ''}"
                      on:click={() => {
                        schedTaskGroupID = gId;
                        showTaskGroupDropdown = false;
                      }}
                    >
                      {gId}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="cancel-btn font-mono" on:click={() => showScheduleModal = false}>取消</button>
          <button class="submit-btn font-mono" on:click={handleSaveSchedule}>确认排期</button>
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
</div>

<style>
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

  .schedule-control-panel {
    display: grid;
    grid-template-columns: minmax(220px, 1.25fr) minmax(300px, 2fr) minmax(150px, 0.7fr) minmax(220px, 1fr) auto;
    gap: 10px;
    align-items: center;
    background: rgba(10, 15, 30, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 10px;
    padding: 10px;
  }

  .schedule-search-shell {
    position: relative;
    display: flex;
    align-items: center;
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

  .schedule-assignee-menu {
    min-width: 150px;
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
    max-height: 620px;
    overflow: auto;
  }

  .schedule-table {
    width: 100%;
    min-width: 1080px;
    border-collapse: collapse;
    table-layout: fixed;
  }

  .schedule-table th {
    position: sticky;
    top: 0;
    z-index: 2;
    background: #0b1220;
    color: #64748b;
    font-size: 0.66rem;
    font-weight: 900;
    text-align: left;
    letter-spacing: 0.04em;
    padding: 10px 12px;
    border-bottom: 1px solid rgba(71, 85, 105, 0.48);
  }

  .schedule-table td {
    vertical-align: top;
    padding: 12px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.24);
    color: #cbd5e1;
    font-size: 0.78rem;
  }

  .schedule-table tr:hover td {
    background: rgba(30, 41, 59, 0.22);
  }

  .demand-cell {
    width: 28%;
  }

  .demand-stack,
  .owner-stack,
  .branch-stack,
  .due-stack,
  .subtask-stack,
  .risk-stack {
    display: flex;
    flex-direction: column;
    gap: 5px;
    min-width: 0;
  }

  .demand-stack strong,
  .branch-stack strong,
  .due-stack strong,
  .owner-stack strong,
  .subtask-stack strong {
    color: #f8fafc;
    line-height: 1.32;
    word-break: break-word;
  }

  .demand-stack small,
  .branch-stack span,
  .due-stack span,
  .due-stack small,
  .owner-stack span,
  .subtask-stack small,
  .risk-stack small {
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.4;
    word-break: break-word;
  }

  .schedule-id {
    color: #818cf8;
    font-size: 0.66rem;
    font-weight: 900;
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

  .status-chip.status-progress { color: #c4b5fd; border-color: rgba(168, 85, 247, 0.34); background: rgba(168, 85, 247, 0.1); }
  .status-chip.status-review { color: #fde68a; border-color: rgba(234, 179, 8, 0.34); background: rgba(234, 179, 8, 0.1); }
  .status-chip.status-done { color: #86efac; border-color: rgba(16, 185, 129, 0.34); background: rgba(16, 185, 129, 0.1); }

  .subtask-meter {
    height: 6px;
    width: 100%;
    min-width: 80px;
    background: rgba(51, 65, 85, 0.72);
    border-radius: 999px;
    overflow: hidden;
  }

  .subtask-meter span {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, #38bdf8, #818cf8);
    border-radius: inherit;
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
    background: rgba(99, 102, 241, 0.14);
    border: 1px solid rgba(129, 140, 248, 0.38);
    color: #c4b5fd;
    border-radius: 7px;
    padding: 6px 10px;
    font-size: 0.72rem;
    font-weight: 800;
    cursor: pointer;
  }

  .schedule-row-action:hover {
    background: #4f46e5;
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

    .schedule-control-panel {
      grid-template-columns: minmax(220px, 1fr) minmax(280px, 1.4fr);
    }
  }

  @media (max-width: 760px) {
    .schedule-summary-grid {
      grid-template-columns: repeat(2, minmax(120px, 1fr));
    }

    .schedule-control-panel {
      grid-template-columns: 1fr;
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
  }

  .demand-card:hover {
    background: rgba(51, 65, 85, 0.3);
    border-color: rgba(99, 102, 241, 0.3);
    transform: translateY(-2px);
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

  .assignee-badge {
    background: rgba(99, 102, 241, 0.12);
    color: #818cf8;
    padding: 2px 6px;
    border-radius: 4px;
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
    overflow-y: auto;
    padding: 24px 16px;
    box-sizing: border-box;
    z-index: 1000;
  }

  .modal-content {
    width: 100%;
    max-width: 520px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    animation: zoomIn 0.16s ease-out;
    transform: translateZ(0);
    backface-visibility: hidden;
  }

  .demand-create-modal {
    max-width: 560px;
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
    width: 16px;
    height: 16px;
    color: #818cf8;
    border: 1px solid rgba(129, 140, 248, 0.7);
    border-radius: 4px;
    pointer-events: none;
    background: rgba(30, 41, 59, 0.5);
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
    left: 3px;
    bottom: 3px;
    width: 3px;
    height: 3px;
    background: currentColor;
    box-shadow: 5px 0 0 currentColor, 10px 0 0 currentColor;
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
    background: #0b1220;
    border: 1px solid rgba(71, 85, 105, 0.76);
    border-radius: 12px;
    padding: 12px;
    box-shadow: 0 20px 48px rgba(2, 6, 23, 0.72);
    z-index: 1400;
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

  .brain-link-note {
    display: flex;
    flex-direction: column;
    gap: 5px;
    background: rgba(14, 165, 233, 0.08);
    border: 1px solid rgba(14, 165, 233, 0.22);
    border-radius: 8px;
    padding: 9px 10px;
    color: #94a3b8;
    font-size: 0.72rem;
    line-height: 1.45;
  }

  .brain-link-note-label {
    color: #38bdf8;
    font-size: 0.62rem;
    font-weight: 900;
    letter-spacing: 0.07em;
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
</style>
