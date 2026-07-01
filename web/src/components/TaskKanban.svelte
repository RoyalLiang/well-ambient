<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
  import Modal from './shared/Modal.svelte';
  import CommitTelemetryPanel from './CommitTelemetryPanel.svelte';

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

  type TaskView = 'status' | 'personnel' | 'execution';
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
    assignee: string;
    department: string;
    repo: string;
    branch: string;
    status: string;
    task_group_id: string;
    parent_demand_id?: string;
    parent_demand?: string;
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
    items: ExecutionTaskItem[];
  }

  const executionRiskFilters: Array<{ value: ExecutionRiskFilter; label: string }> = [
    { value: 'attention', label: '需关注' },
    { value: 'all', label: '全部' },
    { value: 'high', label: '高风险' },
    { value: 'medium', label: '中风险' },
    { value: 'safe', label: '正常' },
    { value: 'done', label: '已闭环' }
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

  let isMounted = false;
  let allTasks: Task[] = [];
  let selectedProject = 'all';
  let selectedAssignee = 'all';
  let currentView: TaskView = 'status';
  let executionItems: ExecutionTaskItem[] = [];
  let executionSummary: ExecutionSummary = emptyExecutionSummary();
  let executionGeneratedAt = '';
  let executionLoading = false;
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
  let executionRiskFilter: ExecutionRiskFilter = 'attention';
  let executionAssigneeFilter = 'all';

  let collapsedAssignees: Record<string, boolean> = {};
  let userToggledAssignees: Record<string, boolean> = {};

  let intervalId: any;
  let loading = true;
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
  let showExecutionAssigneeDropdown = false;
  let projectSelectEl: HTMLElement;
  let assigneeSelectEl: HTMLElement;

  let projectSearchText = '';
  let assigneeSearchText = '';
  let execAssigneeSearchText = '';

  $: if (!showProjectDropdown) projectSearchText = '';
  $: if (!showAssigneeDropdown) assigneeSearchText = '';
  $: if (!showExecutionAssigneeDropdown) execAssigneeSearchText = '';

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
    if (!target.closest('.execution-assignee-select')) {
      showExecutionAssigneeDropdown = false;
    }
  }

  // Modal details state
  let selectedTask: Task | null = null;
  let showDetails = false;

  let allProjects: string[] = [];
  $: projectOptions = ['all', ...allProjects];
  let projectNamesMap: Record<string, string> = {};
  let coreMembers = new Set([
    "梁志远", "朱家聪", "岳颖颖", "Yue Yingying", "姜昊良", "白凌云", "陈伟华", 
    "李厚奇", "鲁俊", "刘子翔", "张路路", "qiang.deng", "MiddleQ", "zhongkou.chang", 
    "Eddie", "Antigravity"
  ]);

  function isCoreMember(name: string): boolean {
    if (!name || name === '未指派' || name === '-' || name === 'Unassigned') return true;
    return coreMembers.has(name) || coreMembers.has(name.split(' ')[0]);
  }

  function updateCoreMembers(config: any) {
    let users: string[] = [];
    if (config.jira) {
      if (config.jira.sync_users && config.jira.sync_users.length > 0) {
        users = [...config.jira.sync_users];
      } else if (config.jira.custom_jql) {
        const match = config.jira.custom_jql.match(/assignee\s+in\s*\(([^)]+)\)/i);
        if (match && match[1]) {
          users = match[1].split(',').map((name: string) => name.trim().replace(/['"]/g, ''));
        }
      }
    }
    
    if (users.length > 0) {
      users = users.filter(name => name !== '未指派' && name !== '-');
      coreMembers = new Set(users);
    }
  }

  $: hasShadowTasks = allTasks.some(t => !isCoreMember(t.assignee) && t.status.toLowerCase() !== 'done');

  $: assigneeOptions = [
    'all', 
    ...Array.from(coreMembers).sort((a, b) => a.localeCompare(b)),
    '外部协同'
  ];

  // Reactive filtered tasks
  $: filteredTasks = allTasks.filter(t => {
    if (t.issueType === 'demand') {
      return false;
    }
    const projMatch = selectedProject === 'all' || getProjectName(t.id) === selectedProject;
    
    let taskAssignee = t.assignee;
    const isCore = isCoreMember(taskAssignee);
    
    // 移除对非筛选人列表的任务数据
    if (!isCore) {
      return false;
    }
    
    const assigneeMatch = selectedAssignee === 'all' || taskAssignee === selectedAssignee;
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

  $: backlog = sortedFilteredTasks.filter(t => t.status.toLowerCase() === 'backlog');
  $: inProgress = sortedFilteredTasks.filter(t => t.status.toLowerCase() === 'progress');
  $: inReview = sortedFilteredTasks.filter(t => t.status.toLowerCase() === 'review');
  $: done = sortedFilteredTasks.filter(t => t.status.toLowerCase() === 'done');
  $: executionAssigneeOptions = [
    'all', 
    ...Array.from(coreMembers).sort((a, b) => a.localeCompare(b)),
    '外部协同'
  ];
  $: filteredExecutionItems = executionItems
    .filter(item => matchesExecutionFilters(item))
    .sort((a, b) => compareExecutionItems(a, b));

  $: viewAssignees = (selectedAssignee === 'all'
    ? [
        ...Array.from(new Set(filteredTasks.filter(t => isCoreMember(t.assignee)).map(t => t.assignee))),
        ...(filteredTasks.some(t => !isCoreMember(t.assignee)) ? ["外部协同"] : [])
      ]
    : [selectedAssignee]
  );

  $: assigneeTasksMap = (() => {
    const map = new Map<string, Task[]>();
    viewAssignees.forEach(ass => map.set(ass, []));

    sortedFilteredTasks.forEach(t => {
      const isCore = isCoreMember(t.assignee);
      const key = isCore ? t.assignee : "外部协同";
      const list = map.get(key);
      if (list) {
        list.push(t);
      } else {
        map.set(key, [t]);
      }
    });
    return map;
  })();

  // 反应式活跃度与卡点风险双驱动自适应折叠：
  $: if (viewAssignees && filteredTasks) {
    viewAssignees.forEach((ass, index) => {
      if (userToggledAssignees[ass] === undefined) {
        if (index === 0) {
          collapsedAssignees[ass] = false; // 保证首位成员必然展开讨论
        } else {
          const hasRiskTasks = filteredTasks.some(t => {
            const isMatch = ass === "外部协同" ? !isCoreMember(t.assignee) : t.assignee === ass;
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

  function setTaskView(view: TaskView) {
    currentView = view;
    if (view === 'execution') {
      fetchExecutionTasks();
    }
  }

  function matchesExecutionFilters(item: ExecutionTaskItem): boolean {
    const query = executionSearch.trim().toLowerCase();
    if (query) {
      const match = 
        (item.task_id && item.task_id.toLowerCase().includes(query)) ||
        (item.title && item.title.toLowerCase().includes(query)) ||
        (item.issue_type && item.issue_type.toLowerCase().includes(query)) ||
        (item.assignee && item.assignee.toLowerCase().includes(query)) ||
        (item.department && item.department.toLowerCase().includes(query)) ||
        (item.repo && item.repo.toLowerCase().includes(query)) ||
        (item.branch && item.branch.toLowerCase().includes(query)) ||
        (item.parent_demand_id && item.parent_demand_id.toLowerCase().includes(query)) ||
        (item.parent_demand && item.parent_demand.toLowerCase().includes(query)) ||
        (item.risk_label && item.risk_label.toLowerCase().includes(query)) ||
        (item.result_label && item.result_label.toLowerCase().includes(query));
      if (!match) return false;
    }

    if (executionAssigneeFilter === '外部协同') {
      if (isCoreMember(item.assignee)) {
        return false;
      }
    } else if (executionAssigneeFilter !== 'all') {
      if (item.assignee !== executionAssigneeFilter) {
        return false;
      }
    }

    if (executionRiskFilter === 'attention') {
      return item.risk_level !== 'safe' && item.risk_level !== 'done';
    }
    if (executionRiskFilter !== 'all') {
      return item.risk_level === executionRiskFilter;
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

  async function fetchExecutionTasks() {
    executionLoading = true;
    executionErrorMsg = '';
    try {
      const params = new URLSearchParams();
      if (selectedProject && selectedProject !== 'all') {
        params.append('project', selectedProject);
      }
      if (selectedAssignee && selectedAssignee !== 'all') {
        params.append('assignee', selectedAssignee);
      }
      if (executionRiskFilter && executionRiskFilter !== 'all') {
        params.append('risk', executionRiskFilter);
      }
      const queryStr = params.toString() ? '?' + params.toString() : '';
      const res = await fetch('/api/execution/tasks' + queryStr);
      if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
      const data: ExecutionTasksResponse = await res.json();
      executionItems = data.items || [];
      executionSummary = data.summary || emptyExecutionSummary();
      executionGeneratedAt = data.generated_at || '';
    } catch (e: any) {
      console.error('Failed to fetch execution tasks:', e);
      executionErrorMsg = e.message || '获取执行追踪失败';
    } finally {
      executionLoading = false;
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
  let activeTelemetryTaskId = '';
  let isTelemetryDrawerOpen = false;

  async function openDetails(task: Task) {
    selectedTask = task;
    showDetails = true;
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

  async function fetchConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const data = await res.json();
        if (data) {
          if (data.jira && data.jira.base_url) {
            jiraBaseUrl = data.jira.base_url.replace(/\/+$/, '');
          }
          updateCoreMembers(data);
        }
      }
    } catch (e) {
      console.error('Failed to fetch config:', e);
    }
  }

  async function fetchProjectConfigs() {
    try {
      const res = await fetch('/api/projects/config');
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data)) {
          const newMap: Record<string, string> = {};
          data.forEach((p: any) => {
            if (p.project_key && p.project_name) {
              newMap[p.project_key.toUpperCase()] = p.project_name;
            }
          });
          projectNamesMap = newMap;
        }
      }
    } catch (e) {
      console.error('Failed to fetch project configs:', e);
    }
  }

  async function fetchUsersFallback() {
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/users', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data) && data.length > 0) {
          const names = data.map((u: any) => u.name).filter(Boolean);
          if (names.length > 0) {
            coreMembers = new Set([...coreMembers, ...names]);
          }
        }
      }
    } catch (e) {
      console.error('Failed to fetch users fallback in TaskKanban:', e);
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

  function getParentDemand(taskGroupId?: string): Task | undefined {
    if (!taskGroupId || taskGroupId === '-' || taskGroupId === '') return undefined;
    return allTasks.find(t => t.issueType === 'demand' && t.taskGroupId === taskGroupId);
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

  async function fetchTasks() {
    try {
      const params = new URLSearchParams();
      if (selectedProject && selectedProject !== 'all') {
        params.append('project', selectedProject);
      }
      if (selectedAssignee && selectedAssignee !== 'all') {
        params.append('assignee', selectedAssignee);
      }
      const queryStr = params.toString() ? '?' + params.toString() : '';
      const res = await fetch('/api/tasks' + queryStr);
      if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
      const data: TaskResponse[] = await res.json();
      
      allTasks = data.map(mapTask);
      if (allProjects.length === 0 && data.length > 0) {
        allProjects = Array.from(new Set(data.map(t => getProjectName(t.task_id || t.id)))).sort((a, b) => a.localeCompare(b));
      }
      errorMsg = '';
    } catch (e: any) {
      console.error('Failed to fetch tasks:', e);
      errorMsg = e.message || '连接 API 失败';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    isMounted = true;
    fetchTasks();
    fetchConfig();
    fetchProjectConfigs();
    fetchJiraLinkConfig();
    intervalId = setInterval(() => {
      fetchTasks();
      if (currentView === 'execution') {
        fetchExecutionTasks();
      }
    }, 5000);
    document.addEventListener('click', handleDocumentClick);
  });

  onDestroy(() => {
    if (intervalId) {
      clearInterval(intervalId);
    }
    document.removeEventListener('click', handleDocumentClick);
  });

  $: {
    // 显式声明依赖项，确保 Svelte 编译器精准捕获每一次过滤条件改变及生命周期挂载
    const _view = currentView;
    const _proj = selectedProject;
    const _ass = selectedAssignee;
    const _execAss = executionAssigneeFilter;
    const _execRisk = executionRiskFilter;
    const _mounted = isMounted;

    if (_mounted) {
      if (_view === 'execution') {
        fetchExecutionTasks();
      } else {
        fetchTasks();
      }
    }
  }
</script>

<section class="kanban-section">
  <div class="section-header">
    <div class="header-left">
      <h2 class="section-title">Git 协同看板</h2>
      <span class="sync-badge">同步中</span>
    </div>
    <div class="header-right-filters">
      <!-- View Toggle -->
      <div class="view-toggle">
        <button class="toggle-btn {currentView === 'status' ? 'active' : ''}" on:click={() => setTaskView('status')}>
          状态视图
        </button>
        <button class="toggle-btn {currentView === 'personnel' ? 'active' : ''}" on:click={() => setTaskView('personnel')}>
          人员视图
        </button>
        <button class="toggle-btn {currentView === 'execution' ? 'active' : ''}" on:click={() => setTaskView('execution')}>
          执行追踪
        </button>
      </div>

      <span class="header-desc">基于 Branch/Commit 自动流转</span>
      
      <!-- Project Filter -->
      <div class="custom-select-container" bind:this={projectSelectEl}>
        <div class="custom-select-trigger combobox-trigger">
          <span class="filter-icon">📁</span>
          <input 
            type="text" 
            class="combobox-trigger-input"
            placeholder={selectedProject === 'all' ? '全部项目' : (projectNamesMap[selectedProject.toUpperCase()] || selectedProject)}
            bind:value={projectSearchText}
            on:focus|stopPropagation={() => showProjectDropdown = true}
            on:click|stopPropagation={() => showProjectDropdown = true}
          />
          <span 
            class="select-arrow"
            on:click|stopPropagation={() => showProjectDropdown = !showProjectDropdown}
            role="button"
            tabindex="0"
          >{showProjectDropdown ? '▲' : '▼'}</span>
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

      <!-- Assignee Filter -->
      <div class="custom-select-container" bind:this={assigneeSelectEl}>
        <div class="custom-select-trigger combobox-trigger">
          <span class="filter-icon">👤</span>
          <input 
            type="text" 
            class="combobox-trigger-input"
            placeholder={selectedAssignee === 'all' ? '全部经办人' : selectedAssignee}
            bind:value={assigneeSearchText}
            on:focus|stopPropagation={() => showAssigneeDropdown = true}
            on:click|stopPropagation={() => showAssigneeDropdown = true}
          />
          <span 
            class="select-arrow"
            on:click|stopPropagation={() => showAssigneeDropdown = !showAssigneeDropdown}
            role="button"
            tabindex="0"
          >{showAssigneeDropdown ? '▲' : '▼'}</span>
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

      <!-- 外部协同快捷跳转按钮 -->
      {#if hasShadowTasks}
        <button 
          class="shadow-shortcut-btn {selectedAssignee === '外部协同' ? 'active' : ''}"
          on:click={() => {
            selectedAssignee = '外部协同';
            if (currentView === 'personnel') {
              collapsedAssignees['外部协同'] = false; // 确保展开
              collapsedAssignees = { ...collapsedAssignees };
              setTimeout(() => {
                const el = document.getElementById('assignee-row-外部协同');
                if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
              }, 100);
            }
          }}
          title="一键查看外部/影子协作者的活跃任务"
        >
          👥 外部协同
        </button>
      {/if}
    </div>
  </div>

  <!-- 顶层效能统计面板 -->
  <div class="kanban-metrics">
    <div class="metric-card">
      <span class="metric-icon">📋</span>
      <div class="metric-info">
        <span class="metric-label">总任务</span>
        <span class="metric-value">{filteredTasks.length}</span>
      </div>
    </div>
    <div class="metric-card">
      <span class="metric-icon text-progress-color">⚡</span>
      <div class="metric-info">
        <span class="metric-label">进行中</span>
        <span class="metric-value">{filteredTasks.filter(t => t.status.toLowerCase() === 'progress').length}</span>
      </div>
    </div>
    <div class="metric-card">
      <span class="metric-icon text-review-color">🔍</span>
      <div class="metric-info">
        <span class="metric-label">代码评审</span>
        <span class="metric-value">{filteredTasks.filter(t => t.status.toLowerCase() === 'review').length}</span>
      </div>
    </div>
    <div class="metric-card">
      <span class="metric-icon text-done-color">✅</span>
      <div class="metric-info">
        <span class="metric-label">已完成</span>
        <span class="metric-value">{filteredTasks.filter(t => t.status.toLowerCase() === 'done').length}</span>
      </div>
    </div>
    <div class="metric-card overdue-card">
      <span class="metric-icon text-critical-color">⚠️</span>
      <div class="metric-info">
        <span class="metric-label">延期预警</span>
        <span class="metric-value">{filteredTasks.filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 3).length}</span>
      </div>
    </div>
  </div>

  {#if currentView === 'execution'}
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
              placeholder={executionAssigneeFilter === 'all' ? '全部负责人' : executionAssigneeFilter}
              bind:value={execAssigneeSearchText}
              on:focus|stopPropagation={() => showExecutionAssigneeDropdown = true}
              on:click|stopPropagation={() => showExecutionAssigneeDropdown = true}
            />
            <span 
              class="select-arrow" 
              on:click|stopPropagation={() => showExecutionAssigneeDropdown = !showExecutionAssigneeDropdown}
              role="button"
              tabindex="0"
            >{showExecutionAssigneeDropdown ? '▲' : '▼'}</span>
          </div>
          {#if showExecutionAssigneeDropdown}
            <div class="custom-select-options">
              {#each executionAssigneeOptions.filter(ass => ass === 'all' || !execAssigneeSearchText || ass.toLowerCase().includes(execAssigneeSearchText.toLowerCase())) as assignee}
                <button
                  class="custom-option {executionAssigneeFilter === assignee ? 'active' : ''}"
                  on:click={() => {
                    executionAssigneeFilter = assignee;
                    showExecutionAssigneeDropdown = false;
                  }}
                >
                  {assignee === 'all' ? '全部负责人' : assignee}
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <button class="execution-refresh-btn font-mono" class:is-loading={executionLoading} on:click={fetchExecutionTasks}>
          刷新
        </button>
      </div>

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
  {:else if currentView === 'status'}
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
            {@const parentDemand = getParentDemand(task.taskGroupId)}
            <div class="task-card {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge">{getProjectName(task.id)}</span>
                </div>
                <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
              </div>
              <h4 class="task-title">{task.title}</h4>
              {#if parentDemand}
                <div class="parent-demand-badge font-mono" title={parentDemand.title}>
                  📋 关联需求: #{parentDemand.id}
                </div>
              {/if}
              <div class="task-footer">
                <span>👤 {task.assignee}</span>
                <span class="active-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)} 天</span>
                {#if getDelayDays(task.taskCreatedAt, task.status) >= 7}
                  <span class="delay-badge text-critical">⚠️ 延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {:else if getDelayDays(task.taskCreatedAt, task.status) >= 3}
                  <span class="delay-badge text-warning">⚠️ 延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
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
            {@const parentDemand = getParentDemand(task.taskGroupId)}
            <div class="task-card card-progress {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id id-progress">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge badge-progress-sub">{getProjectName(task.id)}</span>
                </div>
                <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
              </div>
              <h4 class="task-title text-focus">{task.title}</h4>
              {#if parentDemand}
                <div class="parent-demand-badge font-mono" title={parentDemand.title}>
                  📋 关联需求: #{parentDemand.id}
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
                <span class="assignee-active">👤 {task.assignee}</span>
                <span class="active-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)} 天</span>
                {#if getDelayDays(task.taskCreatedAt, task.status) >= 7}
                  <span class="delay-badge text-critical">⚠️ 延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {:else if getDelayDays(task.taskCreatedAt, task.status) >= 3}
                  <span class="delay-badge text-warning">⚠️ 延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
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
            {@const parentDemand = getParentDemand(task.taskGroupId)}
            <div class="task-card card-review {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id id-review">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge badge-review-sub">{getProjectName(task.id)}</span>
                </div>
                <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
              </div>
              <h4 class="task-title text-focus">{task.title}</h4>
              {#if parentDemand}
                <div class="parent-demand-badge font-mono" title={parentDemand.title}>
                  📋 关联需求: #{parentDemand.id}
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
                <span>👤 {task.assignee}</span>
                <span class="active-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)} 天</span>
                {#if getDelayDays(task.taskCreatedAt, task.status) >= 7}
                  <span class="delay-badge text-critical">⚠️ 延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
                {:else if getDelayDays(task.taskCreatedAt, task.status) >= 3}
                  <span class="delay-badge text-warning">⚠️ 延期 {getDelayDays(task.taskCreatedAt, task.status)} 天</span>
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
            {@const parentDemand = getParentDemand(task.taskGroupId)}
            <div class="task-card card-done" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
              <div class="task-meta">
                <div class="meta-left">
                  <span class="task-id id-done">{task.id}</span>
                  {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                  {/if}
                  <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                  <span class="project-badge">{getProjectName(task.id)}</span>
                </div>
                <span class="task-repo">{task.repo !== '-' ? task.repo : ''}</span>
              </div>
              <h4 class="task-title title-done">{task.title}</h4>
              {#if parentDemand}
                <div class="parent-demand-badge font-mono" title={parentDemand.title}>
                  📋 关联需求: #{parentDemand.id}
                </div>
              {/if}
              <div class="task-footer">
                <span>👤 {task.assignee}</span>
                <span class="active-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)} 天</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>
  {:else}
    <!-- Personnel View (Swimlanes) -->
    <div class="personnel-view">
      {#if viewAssignees.length === 0}
        <div class="empty-view">
          <span>🎉 当前项目/经办人筛选下无关联的任务</span>
        </div>
      {:else}
        {#each viewAssignees as assignee}
          <div id="assignee-row-{assignee}" class="swimlane {collapsedAssignees[assignee] !== false ? 'collapsed' : ''}">
            <div 
              class="swimlane-header interactive-header" 
              on:click={() => toggleAssigneeCollapse(assignee)} 
              role="button" 
              tabindex="0" 
              on:keydown={(e) => e.key === 'Enter' && toggleAssigneeCollapse(assignee)}
              aria-expanded={collapsedAssignees[assignee] === false}
            >
              <div class="assignee-info">
                <span class="collapse-chevron" class:is-collapsed={collapsedAssignees[assignee] !== false}>
                  <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2.5" fill="none" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="6 9 12 15 18 9"></polyline>
                  </svg>
                </span>
                <span class="assignee-avatar">👤</span>
                <span class="assignee-name">{assignee}</span>
                <span class="assignee-count">
                  {getAssigneeTaskCount(assignee, filteredTasks)} 任务
                  {#if getAssigneeBugCount(assignee, filteredTasks) > 0}
                    · <span class="bug-count text-bug">{getAssigneeBugCount(assignee, filteredTasks)} Bug</span>
                  {/if}
                </span>
                
                <!-- 状态总览胶囊 -->
                <div class="assignee-status-overview" on:click|stopPropagation>
                  <span class="status-summary-item text-backlog">待办 {getAssigneeStatusCount(assignee, 'backlog', filteredTasks)}</span>
                  <span class="status-summary-item text-progress">进行中 {getAssigneeStatusCount(assignee, 'progress', filteredTasks)}</span>
                  <span class="status-summary-item text-review">评审中 {getAssigneeStatusCount(assignee, 'review', filteredTasks)}</span>
                  <span class="status-summary-item text-done">已完成 {getAssigneeStatusCount(assignee, 'done', filteredTasks)}</span>
                </div>

                <!-- 瓶颈与负荷预警 -->
                {#if isAssigneeHighLoad(assignee, filteredTasks)}
                  <span class="swimlane-badge badge-warning">🔥 负载高</span>
                {/if}
                {#if isAssigneeDelayed(assignee, filteredTasks)}
                  <span class="swimlane-badge badge-danger">⚠️ 有延期</span>
                {/if}
              </div>
            </div>
            
            {#if collapsedAssignees[assignee] === false}
              <div class="swimlane-grid" transition:slide={{ duration: 250 }}>
              <!-- Backlog Swimlane Column -->
              <div class="swimlane-column">
                <div class="swimlane-column-header text-backlog">待办 ({getAssigneeTasks(assignee, 'backlog').length})</div>
                <div class="swimlane-column-body">
                  {#each getAssigneeTasks(assignee, 'backlog') as task}
                    <div class="compact-task-card {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
                      <div class="compact-meta">
                        <span class="task-id">{task.id}</span>
                        {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)}d</span>
                      </div>
                      <h5 class="compact-title">{task.title}</h5>
                    </div>
                  {/each}
                  {#if getAssigneeTasks(assignee, 'backlog').length === 0}
                    <div class="empty-placeholder">暂无待办</div>
                  {/if}
                </div>
              </div>

              <!-- In Progress Swimlane Column -->
              <div class="swimlane-column">
                <div class="swimlane-column-header text-progress">进行中 ({getAssigneeTasks(assignee, 'progress').length})</div>
                <div class="swimlane-column-body">
                  {#each getAssigneeTasks(assignee, 'progress') as task}
                    <div class="compact-task-card card-progress {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
                      <div class="compact-meta">
                        <span class="task-id id-progress">{task.id}</span>
                        {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)}d</span>
                      </div>
                      <h5 class="compact-title text-focus">{task.title}</h5>
                    </div>
                  {/each}
                  {#if getAssigneeTasks(assignee, 'progress').length === 0}
                    <div class="empty-placeholder">暂无进行中</div>
                  {/if}
                </div>
              </div>

              <!-- In Review Swimlane Column -->
              <div class="swimlane-column">
                <div class="swimlane-column-header text-review">代码评审 ({getAssigneeTasks(assignee, 'review').length})</div>
                <div class="swimlane-column-body">
                  {#each getAssigneeTasks(assignee, 'review') as task}
                    <div class="compact-task-card card-review {getDelayClass(task)}" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
                      <div class="compact-meta">
                        <span class="task-id id-review">{task.id}</span>
                        {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)}d</span>
                      </div>
                      <h5 class="compact-title text-focus">{task.title}</h5>
                    </div>
                  {/each}
                  {#if getAssigneeTasks(assignee, 'review').length === 0}
                    <div class="empty-placeholder">暂无评审</div>
                  {/if}
                </div>
              </div>

              <!-- Done Swimlane Column -->
              <div class="swimlane-column">
                <div class="swimlane-column-header text-done">已完成 ({getAssigneeTasks(assignee, 'done').length})</div>
                <div class="swimlane-column-body">
                  {#each getAssigneeTasks(assignee, 'done') as task}
                    <div class="compact-task-card card-done" role="button" tabindex="0" on:click={() => openDetails(task)} on:keydown={(e) => e.key === 'Enter' && openDetails(task)}>
                      <div class="compact-meta">
                        <span class="task-id id-done">{task.id}</span>
                        {#if jiraBaseUrl && task.id && !task.id.startsWith('TASK-')}
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">🔗</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">⏱️ {getActiveDays(task.taskCreatedAt)}d</span>
                      </div>
                      <h5 class="compact-title title-done">{task.title}</h5>
                    </div>
                  {/each}
                  {#if getAssigneeTasks(assignee, 'done').length === 0}
                    <div class="empty-placeholder">暂无已完成</div>
                  {/if}
                </div>
              </div>
            </div>
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</section>

{#if showDetails && selectedTask}
  <Modal show={showDetails} title="任务详情: {selectedTask.id}" on:close={() => { showDetails = false; selectedTask = null; }}>
    <div class="details-container">
      <div class="details-row">
        <span class="label">任务 ID:</span>
        <span class="value font-mono">
          {selectedTask.id}
          {#if jiraBaseUrl && selectedTask.id && !selectedTask.id.startsWith('TASK-')}
            <a href="{jiraBaseUrl}/browse/{selectedTask.id}" target="_blank" rel="noopener noreferrer" class="jira-modal-direct-btn">
              🔗Jira 链接
            </a>
          {/if}
        </span>
      </div>
      <div class="details-row">
        <span class="label">所属项目:</span>
        <span class="value badge-project">{getProjectName(selectedTask.id)}</span>
      </div>
      {#if hasParentDemand(selectedTask.taskGroupId)}
        <div class="details-row">
          <span class="label">关联需求:</span>
          <span class="value parent-demand-detail font-mono">
            📋 #{getParentDemandId(selectedTask.taskGroupId)} <span class="title-sub">{getParentDemandTitle(selectedTask.taskGroupId)}</span>
          </span>
        </div>
      {/if}
      <div class="details-row">
        <span class="label">任务类型:</span>
        <span class="value">
          <span class="issue-type-badge type-{selectedTask.issueType.toLowerCase()}">{selectedTask.issueType === 'bug' ? 'Bug 缺陷' : 'Task 任务'}</span>
        </span>
      </div>
      <div class="details-row">
        <span class="label">任务标题:</span>
        <span class="value title-val">{selectedTask.title}</span>
      </div>
      <div class="details-row">
        <span class="label">指派人:</span>
        <span class="value">👤 {selectedTask.assignee}</span>
      </div>
      <div class="details-row">
        <span class="label">当前进度阶段:</span>
        <span class="value">
          <span class="status-dot dot-{selectedTask.status.toLowerCase()}"></span>
          <span class="status-name">{getStatusLabel(selectedTask.status, selectedTask.issueType)}</span>
        </span>
      </div>
      <div class="details-row">
        <span class="label">实际创建时间:</span>
        <span class="value">{formatTimeFull(selectedTask.taskCreatedAt)}</span>
      </div>
      
      <div class="divider"></div>
      <div class="telemetry-decoupled-section">
        <span class="decoupled-title font-mono">GIT TELEMETRY EVIDENCE</span>
        <button class="view-telemetry-drawer-btn font-mono" on:click={() => {
          activeTelemetryTaskId = selectedTask ? selectedTask.id : '';
          isTelemetryDrawerOpen = true;
        }}>
          🛰️ 展开代码提交轨迹与 MR 证据
        </button>
      </div>
      
      <div class="details-row">
        <span class="label">系统最后同步:</span>
        <span class="value">{formatTimeFull(selectedTask.rawLastUpdate)}</span>
      </div>
    </div>
  </Modal>
{/if}

<CommitTelemetryPanel taskID={activeTelemetryTaskId} isOpen={isTelemetryDrawerOpen} onClose={() => isTelemetryDrawerOpen = false} />

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
    border-top: 3px solid rgba(100, 116, 139, 0.72);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .execution-summary-cell.is-red { border-top-color: #ef4444; }
  .execution-summary-cell.is-amber { border-top-color: #f59e0b; }
  .execution-summary-cell.is-blue { border-top-color: #38bdf8; }
  .execution-summary-cell.is-violet { border-top-color: #a78bfa; }

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
    background: linear-gradient(90deg, #38bdf8, #818cf8);
    border-radius: 8px;
    transition: width 0.3s ease;
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
    color: #cbd5e1;
    font-size: 0.78rem;
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
    background: linear-gradient(135deg, #e2e8f0 0%, #cbd5e1 50%, #e2e8f0 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
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

  /* Personnel View Swimlanes Styling */
  .personnel-view {
    display: flex;
    flex-direction: column;
    gap: 24px;
    margin-bottom: 24px;
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

  .shadow-shortcut-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    background: rgba(244, 63, 94, 0.1);
    border: 1px solid rgba(244, 63, 94, 0.25);
    border-radius: 8px;
    color: #f43f5e;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 4px 12px rgba(244, 63, 94, 0.05);
    margin-left: 12px;
  }

  .shadow-shortcut-btn:hover {
    background: rgba(244, 63, 94, 0.2);
    border-color: rgba(244, 63, 94, 0.4);
    transform: translateY(-1px);
    box-shadow: 0 6px 16px rgba(244, 63, 94, 0.1);
  }

  .shadow-shortcut-btn.active {
    background: #f43f5e;
    border-color: #f43f5e;
    color: #ffffff;
    box-shadow: 0 4px 14px rgba(244, 63, 94, 0.3);
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
    border-left: 2px solid rgba(71, 85, 105, 0.4);
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
</style>
