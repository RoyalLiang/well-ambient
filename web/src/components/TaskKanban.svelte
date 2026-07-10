<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
  import Modal from './shared/Modal.svelte';
  import CommitTelemetryPanel from './CommitTelemetryPanel.svelte';
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

  const taskTableColumns: AdminTableColumn[] = [
    { key: 'task', label: '任务编号 / 标题', width: '34%' },
    { key: 'owner', label: '负责人', width: '12%' },
    { key: 'status', label: '状态', width: '12%' },
    { key: 'risk', label: '风险', width: '11%' },
    { key: 'evidence', label: '代码证据', width: '15%' },
    { key: 'lastUpdate', label: '最后同步', width: '10%' },
    { key: 'actions', label: '操作', width: '6%', align: 'right' }
  ];

  const executionTableColumns: AdminTableColumn[] = [
    { key: 'task', label: 'Jira Task / 标题', width: '32%' },
    { key: 'owner', label: 'Owner', width: '12%' },
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
  let selectedTaskId = '';
  let selectedExecutionTaskId = '';

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

  $: assigneeOptions = [
    'all', 
    ...Array.from(coreMembers).sort((a, b) => a.localeCompare(b))
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
  $: activeTasks = sortedFilteredTasks.filter(t => t.status.toLowerCase() !== 'done');
  $: overdueTasks = activeTasks
    .filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 3)
    .sort((a, b) => getDelayDays(b.taskCreatedAt, b.status) - getDelayDays(a.taskCreatedAt, a.status));
  $: criticalTasks = overdueTasks.filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 7);
  $: focusTask = criticalTasks[0] || overdueTasks[0] || inReview[0] || inProgress[0] || backlog[0] || done[0] || null;
  $: focusTaskDelayDays = focusTask ? getDelayDays(focusTask.taskCreatedAt, focusTask.status) : 0;
  $: evidenceLinkedTasks = filteredTasks.filter(t => getEvidenceStatus(t).class === 'badge-has-code').length;
  $: evidenceCoverage = filteredTasks.length > 0 ? Math.round((evidenceLinkedTasks / filteredTasks.length) * 100) : 0;
  $: taskFlowStages = [
    { key: 'backlog', label: '待办', value: backlog.length, tone: 'neutral' },
    { key: 'progress', label: '进行中', value: inProgress.length, tone: 'info' },
    { key: 'review', label: '评审', value: inReview.length, tone: 'warning' },
    { key: 'done', label: '完成', value: done.length, tone: 'success' }
  ].map(stage => ({
    ...stage,
    percent: filteredTasks.length > 0 ? Math.round((stage.value / filteredTasks.length) * 100) : 0
  }));
  $: executionAssigneeOptions = [
    'all', 
    ...Array.from(coreMembers).sort((a, b) => a.localeCompare(b))
  ];
  $: filteredExecutionItems = executionItems
    .filter(item => matchesExecutionFilters(item))
    .sort((a, b) => compareExecutionItems(a, b));
  $: executionFocusItem = filteredExecutionItems[0] || executionItems[0] || null;
  $: executionEvidenceRate = executionSummary.total > 0
    ? Math.round((executionSummary.with_evidence / executionSummary.total) * 100)
    : 0;

  $: viewAssignees = (selectedAssignee === 'all'
    ? Array.from(new Set(filteredTasks.filter(t => isCoreMember(t.assignee)).map(t => t.assignee)))
    : [selectedAssignee]
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
  $: ownerLoadList = viewAssignees
    .map(assignee => {
      const tasks = assigneeTasksMap.get(assignee) || [];
      const active = tasks.filter(t => t.status.toLowerCase() !== 'done').length;
      const delayed = tasks.filter(t => getDelayDays(t.taskCreatedAt, t.status) >= 3).length;
      return { assignee, total: tasks.length, active, delayed };
    })
    .filter(item => item.total > 0)
    .sort((a, b) => {
      if (b.delayed !== a.delayed) return b.delayed - a.delayed;
      if (b.active !== a.active) return b.active - a.active;
      return b.total - a.total;
    });
  $: dominantOwner = ownerLoadList[0] || null;

  $: taskCompletionRate = filteredTasks.length > 0 ? Math.round((done.length / filteredTasks.length) * 100) : 0;
  $: taskAdminMetrics = buildTaskAdminMetrics();
  $: executionAdminMetrics = buildExecutionAdminMetrics();
  $: activeAdminMetrics = currentView === 'execution' ? executionAdminMetrics : taskAdminMetrics;
  $: taskTableRows = sortedFilteredTasks.map(mapTaskAdminRow);
  $: selectedTaskForInspector = sortedFilteredTasks.find(t => t.id === selectedTaskId) || focusTask;
  $: taskInspector = selectedTaskForInspector ? mapTaskInspector(selectedTaskForInspector) : null;
  $: executionTableRows = filteredExecutionItems.map(mapExecutionAdminRow);
  $: selectedExecutionItem = filteredExecutionItems.find(item => item.task_id === selectedExecutionTaskId) || executionFocusItem;
  $: executionInspector = selectedExecutionItem ? mapExecutionInspector(selectedExecutionItem) : null;

  function buildTaskAdminMetrics(): AdminMetric[] {
    return [
      {
        label: '任务队列',
        value: filteredTasks.length,
        helper: `活跃 ${activeTasks.length} / 完成 ${done.length}`,
        delta: selectedProject === 'all' ? '全部项目' : selectedProject,
        tone: 'info'
      },
      {
        label: '证据完整率',
        value: `${evidenceCoverage}%`,
        helper: `${evidenceLinkedTasks} 条已关联代码证据`,
        tone: evidenceCoverage >= 80 ? 'success' : evidenceCoverage >= 50 ? 'warning' : 'danger'
      },
      {
        label: '高风险',
        value: criticalTasks.length,
        helper: `延期任务 ${overdueTasks.length} 条`,
        delta: criticalTasks.length > 0 ? '需跟进' : '稳定',
        tone: criticalTasks.length > 0 ? 'danger' : 'success'
      },
      {
        label: '闭环率',
        value: `${taskCompletionRate}%`,
        helper: `评审 ${inReview.length} / 进行中 ${inProgress.length}`,
        tone: taskCompletionRate >= 70 ? 'success' : 'info'
      }
    ];
  }

  function buildExecutionAdminMetrics(): AdminMetric[] {
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
    const status = task.status.toLowerCase();
    if (status === 'backlog') return '待办';
    if (status === 'progress') return task.issueType === 'bug' ? '排查中' : '进行中';
    if (status === 'review') return '评审中';
    if (status === 'done') return '已完成';
    return task.status || '-';
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
    const evidence = getEvidenceStatus(task);
    return {
      id: task.id,
      title: task.title,
      status: getTaskStatusShortLabel(task),
      tone: toneForStatus(task.status),
      owner: task.assignee,
      dueDate: formatAdminDate(task.rawLastUpdate || task.taskCreatedAt),
      risk: getTaskRiskLabel(task),
      cells: {
        project: getProjectName(task.id),
        issueType: task.issueType === 'bug' ? 'Bug' : 'Task',
        owner: task.assignee || '未指派',
        repo: task.repo || '-',
        branch: task.branch || '-',
        lastCommit: task.lastCommit || '-',
        evidence: evidence.label,
        evidenceTone: getEvidenceTone(task),
        evidencePercent: getTaskEvidencePercent(task),
        activeDays: getActiveDays(task.taskCreatedAt),
        lastUpdate: formatTimeBrief(task.rawLastUpdate || task.taskCreatedAt),
        parentDemand: getParentDemandId(task.taskGroupId) || '-'
      }
    };
  }

  function mapTaskInspector(task: Task): AdminInspectorRecord {
    const parentDemand = getParentDemand(task.taskGroupId);
    const evidence = getEvidenceStatus(task);
    const delayDays = getDelayDays(task.taskCreatedAt, task.status);
    return {
      id: task.id,
      title: task.title,
      status: getTaskStatusShortLabel(task),
      tone: getTaskRiskTone(task),
      facts: [
        { label: '负责人', value: task.assignee || '未指派' },
        { label: '项目', value: getProjectName(task.id) },
        { label: '任务类型', value: task.issueType === 'bug' ? 'Bug 缺陷' : 'Task 任务' },
        { label: '活跃天数', value: `${getActiveDays(task.taskCreatedAt)} 天` },
        { label: '最后同步', value: formatTimeBrief(task.rawLastUpdate || task.taskCreatedAt) }
      ],
      sections: [
        {
          title: 'Jira Demand 主线',
          body: parentDemand ? `${parentDemand.id} ${parentDemand.title}` : '当前任务未关联父级 Demand'
        },
        {
          title: '代码证据',
          items: [
            `证据状态: ${evidence.label}`,
            `Branch: ${task.branch || '-'}`,
            `Last Commit: ${task.lastCommit || '-'}`,
            task.mrUrl && task.mrIid ? `MR: !${task.mrIid}` : 'MR: -'
          ]
        },
        {
          title: '执行风险',
          body: delayDays >= 3
            ? `已活跃 ${delayDays} 天，建议检查交付阻塞与证据闭环。`
            : '当前筛选下未触发延期风险。'
        }
      ],
      actions: [
        { label: '查看证据链', kind: 'primary' },
        { label: '代码轨迹', kind: 'secondary' }
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
        issueType: item.issue_type === 'bug' ? 'Bug' : 'Task',
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
        { label: '打开代码轨迹', kind: 'primary' }
      ]
    };
  }

  function handleTaskInspectorAction(action: AdminInspectorAction, task: Task) {
    if (action.label === '查看证据链') {
      openDetails(task);
      return;
    }
    activeTelemetryTaskId = task.id;
    isTelemetryDrawerOpen = true;
  }

  function handleExecutionInspectorAction(_action: AdminInspectorAction, item: ExecutionTaskItem) {
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

  function setTaskView(view: TaskView) {
    currentView = view;
    if (view === 'execution') {
      fetchExecutionTasks();
    }
  }

  function focusExecutionAttention() {
    executionRiskFilter = 'attention';
    setTaskView('execution');
  }

  function matchesExecutionFilters(item: ExecutionTaskItem): boolean {
    const query = executionSearch.trim().toLowerCase();
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

    if (executionAssigneeFilter !== 'all') {
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
  let evidenceChain: any = null;
  let evidenceChainLoading = false;
  let evidenceChainError = '';
  let activeTelemetryTaskId = '';
  let isTelemetryDrawerOpen = false;

  async function openDetails(task: Task) {
    selectedTask = task;
    showDetails = true;
    
    evidenceChainLoading = true;
    evidenceChainError = '';
    evidenceChain = null;
    try {
      const res = await fetch(`/api/strongest-brain/evidence-chain?task_id=${encodeURIComponent(task.id)}`);
      if (res.ok) {
        evidenceChain = await res.json();
      } else {
        throw new Error('无法拉取该任务的交付证据链');
      }
    } catch (err: any) {
      evidenceChainError = err.message || '获取交付证据链异常';
    } finally {
      evidenceChainLoading = false;
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
        allProjects = Array.from(new Set(data.map(t => getProjectName(t.task_id)))).sort((a, b) => a.localeCompare(b));
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

<section class="kanban-section task-console">
  <div class="phase41-console-header wa-admin-card">
    <div class="phase41-header-copy">
      <span class="phase41-kicker">任务跟踪</span>
      <h2>研发执行任务台</h2>
      <p>从 Jira Task、负责人、状态、代码证据和 MR 结果构建可核查的执行事实。</p>
    </div>

    <div class="phase41-header-actions">
      <div class="view-toggle task-view-toggle" aria-label="任务视图切换">
        <button class="toggle-btn {currentView === 'status' ? 'active' : ''}" on:click={() => setTaskView('status')}>
          任务表
        </button>
        <button class="toggle-btn {currentView === 'personnel' ? 'active' : ''}" on:click={() => setTaskView('personnel')}>
          人员负载
        </button>
        <button class="toggle-btn {currentView === 'execution' ? 'active' : ''}" on:click={() => setTaskView('execution')}>
          执行追踪
        </button>
      </div>

      <div class="phase41-filter-row">
        <div class="custom-select-container" bind:this={projectSelectEl}>
          <div class="custom-select-trigger combobox-trigger task-filter-control">
            <span class="filter-label">项目</span>
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
            <span class="filter-label">负责人</span>
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
              aria-label="切换负责人筛选"
            >{showAssigneeDropdown ? '▲' : '▼'}</button>
          </div>
          {#if showAssigneeDropdown}
            <div class="custom-select-options">
              {#each assigneeOptions.filter(ass => ass === 'all' || !assigneeSearchText || ass.toLowerCase().includes(assigneeSearchText.toLowerCase())) as ass}
                <button
                  class="custom-option {selectedAssignee === ass ? 'active' : ''}"
                  on:click={() => selectAssignee(ass)}
                >
                  {ass === 'all' ? '全部负责人' : ass}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>

  <div class="phase41-metric-grid" aria-label="任务指标">
    {#each activeAdminMetrics as metric}
      <article class="wa-admin-card wa-admin-metric phase41-metric {ADMIN_TONE_CLASS[metric.tone || 'neutral']}">
        <span>{metric.label}</span>
        <strong>{metric.value}</strong>
        <small>{metric.helper}</small>
        {#if metric.delta}
          <em>{metric.delta}</em>
        {/if}
      </article>
    {/each}
  </div>

  {#if currentView !== 'execution'}
    <div class="phase41-stage-strip" aria-label="任务阶段统计">
      {#each taskFlowStages as stage}
        <button class="phase41-stage-card wa-admin-card tone-{stage.tone}" type="button" on:click={() => setTaskView('status')}>
          <span>{stage.label}</span>
          <strong>{stage.value}</strong>
          <em>{stage.percent}%</em>
          <i style="width: {stage.percent}%"></i>
        </button>
      {/each}
    </div>
  {/if}

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
          <button class="toggle-btn {currentView === 'personnel' ? 'active' : ''}" on:click={() => setTaskView('personnel')}>
            人员
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

      <div class="owner-pressure-panel">
        <span class="summary-kicker font-mono">OWNER LOAD</span>
        {#if dominantOwner}
          <div class="owner-pressure-main">
            <strong>{dominantOwner.assignee}</strong>
            <span>{dominantOwner.active} 活跃 / {dominantOwner.delayed} 延期</span>
          </div>
        {:else}
          <div class="owner-pressure-main is-empty">暂无负责人负载</div>
        {/if}

        <div class="owner-mini-list">
          {#each ownerLoadList.slice(0, 3) as owner}
            <span>
              <em>{owner.assignee}</em>
              <strong>{owner.active}</strong>
            </span>
          {/each}
        </div>
      </div>
    </aside>
  </div>
  {/if}

  {#if currentView === 'execution'}
    <div class="phase41-workbench phase41-execution-workbench">
      <section class="wa-admin-section phase41-table-panel">
        <div class="wa-admin-card wa-admin-toolbar phase41-table-toolbar">
          <div>
            <span class="phase41-kicker">执行追踪</span>
            <strong>Jira Task 开发结果追踪</strong>
            <small>{filteredExecutionItems.length} / {executionItems.length} 条 · {executionGeneratedAt || '未同步'}</small>
          </div>

          <div class="phase41-execution-controls">
            <div class="execution-search-shell phase41-search-control">
              <span class="execution-search-mark"></span>
              <input
                type="text"
                placeholder="搜索 Jira Key、任务、负责人、分支、需求"
                value={executionSearchInput}
                on:input={handleExecutionSearch}
              />
            </div>

            <div class="execution-risk-strip phase41-risk-strip">
              {#each executionRiskFilters as filter}
                <button
                  class:active={executionRiskFilter === filter.value}
                  on:click={() => executionRiskFilter = filter.value}
                >
                  {filter.label}
                </button>
              {/each}
            </div>

            <div class="custom-select-container execution-assignee-select phase41-assignee-select">
              <div class="custom-select-trigger combobox-trigger">
                <input
                  type="text"
                  class="combobox-trigger-input"
                  placeholder={executionAssigneeFilter === 'all' ? '全部负责人' : executionAssigneeFilter}
                  bind:value={execAssigneeSearchText}
                  on:focus|stopPropagation={() => showExecutionAssigneeDropdown = true}
                  on:click|stopPropagation={() => showExecutionAssigneeDropdown = true}
                />
                <button
                  type="button"
                  class="select-arrow"
                  on:click|stopPropagation={() => showExecutionAssigneeDropdown = !showExecutionAssigneeDropdown}
                  aria-label="切换执行负责人筛选"
                >{showExecutionAssigneeDropdown ? '▲' : '▼'}</button>
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

            <button class="wa-admin-action secondary" class:is-loading={executionLoading} on:click={fetchExecutionTasks}>
              刷新
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
                {#each executionTableRows as row}
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
                          <span class="wa-admin-pill tone-info">{row.cells.issueType}</span>
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
              placeholder={executionAssigneeFilter === 'all' ? '全部负责人' : executionAssigneeFilter}
              bind:value={execAssigneeSearchText}
              on:focus|stopPropagation={() => showExecutionAssigneeDropdown = true}
              on:click|stopPropagation={() => showExecutionAssigneeDropdown = true}
            />
            <button
              type="button"
              class="select-arrow" 
              on:click|stopPropagation={() => showExecutionAssigneeDropdown = !showExecutionAssigneeDropdown}
              aria-label="切换执行负责人筛选"
            >{showExecutionAssigneeDropdown ? '▲' : '▼'}</button>
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
  {:else if currentView === 'status'}
    <div class="phase41-workbench">
      <section class="wa-admin-section phase41-table-panel">
        <div class="wa-admin-card wa-admin-toolbar phase41-table-toolbar">
          <div>
            <span class="phase41-kicker">任务列表</span>
            <strong>Jira Task 执行行</strong>
            <small>{taskTableRows.length} 条核心成员可见任务 · {selectedAssignee === 'all' ? '全部负责人' : selectedAssignee}</small>
          </div>
          <div class="phase41-toolbar-actions">
            {#if errorMsg}
              <span class="phase41-inline-error">{errorMsg}</span>
            {/if}
            <button type="button" class="wa-admin-action secondary" on:click={fetchTasks}>
              刷新
            </button>
            {#if selectedTaskForInspector}
              <button type="button" class="wa-admin-action primary" on:click={() => openDetails(selectedTaskForInspector)}>
                证据链
              </button>
            {/if}
          </div>
        </div>

        <div class="wa-admin-table-shell phase41-table-shell">
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
                <tr><td colspan={taskTableColumns.length} class="phase41-empty-cell">当前项目和负责人筛选下暂无任务</td></tr>
              {:else}
                {#each taskTableRows as row}
                  <tr
                    class:is-selected={selectedTaskForInspector?.id === row.id}
                    tabindex="0"
                    on:click={() => selectedTaskId = row.id}
                    on:keydown={(event) => { if (event.key === 'Enter') selectedTaskId = row.id; }}
                  >
                    <td>
                      <div class="phase41-title-cell">
                        <div>
                          {#if jiraBaseUrl && row.id && !row.id.startsWith('TASK-')}
                            <a href="{jiraBaseUrl}/browse/{row.id}" target="_blank" rel="noopener noreferrer" class="phase41-id-link" on:click|stopPropagation>{row.id}</a>
                          {:else}
                            <span class="phase41-id-link as-text">{row.id}</span>
                          {/if}
                          <span class="wa-admin-pill tone-info">{row.cells.issueType}</span>
                          <span class="wa-admin-pill tone-neutral">{row.cells.project}</span>
                          {#if row.cells.parentDemand !== '-'}
                            <span class="wa-admin-pill tone-neutral">{row.cells.parentDemand}</span>
                          {/if}
                        </div>
                        <strong>{row.title}</strong>
                        <small>{row.cells.repo}</small>
                      </div>
                    </td>
                    <td>
                      <div class="phase41-owner-cell">
                        <strong>{row.owner}</strong>
                        <small>活跃 {row.cells.activeDays} 天</small>
                      </div>
                    </td>
                    <td><span class="wa-admin-pill {ADMIN_TONE_CLASS[row.tone || 'neutral']}">{row.status}</span></td>
                    <td><span class="wa-admin-pill {ADMIN_TONE_CLASS[toneForRisk(String(row.risk || ''))]}">{row.risk || '-'}</span></td>
                    <td>
                      <div class="phase41-evidence-cell">
                        <div class="wa-admin-progress" style="--progress: {Number(row.cells.evidencePercent) || 0}%"></div>
                        <span class="wa-admin-pill {getToneClass(String(row.cells.evidenceTone))}">{row.cells.evidence}</span>
                      </div>
                    </td>
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
            <span class="phase41-kicker">任务详情</span>
            <h3>{taskInspector.title}</h3>
            <div>
              <span class="phase41-id-link as-text">{taskInspector.id}</span>
              <span class="wa-admin-pill {ADMIN_TONE_CLASS[taskInspector.tone || 'neutral']}">{taskInspector.status}</span>
            </div>
          </div>

          <dl class="phase41-fact-grid">
            {#each taskInspector.facts as fact}
              <div>
                <dt>{fact.label}</dt>
                <dd>{fact.value}</dd>
              </div>
            {/each}
          </dl>

          <div class="phase41-inspector-progress">
            <div class="wa-admin-progress" style="--progress: {getTaskEvidencePercent(selectedTaskForInspector)}%"></div>
            <span>证据完整度 {getTaskEvidencePercent(selectedTaskForInspector)}%</span>
          </div>

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
          <div class="phase41-empty-inspector">当前筛选下暂无可查看任务</div>
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
  {:else}
    <!-- Personnel View (Swimlanes) -->
    <div class="personnel-view">
      {#if viewAssignees.length === 0}
        <div class="empty-view">
          <span>当前项目/经办人筛选下无关联任务</span>
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
                <span class="assignee-avatar font-mono">{assignee.slice(0, 1).toUpperCase()}</span>
                <span class="assignee-name">{assignee}</span>
                <span class="assignee-count">
                  {getAssigneeTaskCount(assignee, filteredTasks)} 任务
                  {#if getAssigneeBugCount(assignee, filteredTasks) > 0}
                    · <span class="bug-count text-bug">{getAssigneeBugCount(assignee, filteredTasks)} Bug</span>
                  {/if}
                </span>
                
                <!-- 状态总览胶囊 -->
                <button type="button" class="assignee-status-overview" on:click|stopPropagation aria-label="{assignee} 状态概览">
                  <span class="status-summary-item text-backlog">待办 {getAssigneeStatusCount(assignee, 'backlog', filteredTasks)}</span>
                  <span class="status-summary-item text-progress">进行中 {getAssigneeStatusCount(assignee, 'progress', filteredTasks)}</span>
                  <span class="status-summary-item text-review">评审中 {getAssigneeStatusCount(assignee, 'review', filteredTasks)}</span>
                  <span class="status-summary-item text-done">已完成 {getAssigneeStatusCount(assignee, 'done', filteredTasks)}</span>
                </button>

                <!-- 瓶颈与负荷预警 -->
                {#if isAssigneeHighLoad(assignee, filteredTasks)}
                  <span class="swimlane-badge badge-warning">负载高</span>
                {/if}
                {#if isAssigneeDelayed(assignee, filteredTasks)}
                  <span class="swimlane-badge badge-danger">有延期</span>
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
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)}d</span>
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
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)}d</span>
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
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)}d</span>
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
                          <a href="{jiraBaseUrl}/browse/{task.id}" target="_blank" rel="noopener noreferrer" class="jira-direct-link" on:click|stopPropagation title="直达 Jira">Jira</a>
                        {/if}
                        <span class="issue-type-badge type-{task.issueType.toLowerCase()}">{task.issueType === 'bug' ? 'Bug' : 'Task'}</span>
                        <span class="compact-days-badge font-mono">活跃 {getActiveDays(task.taskCreatedAt)}d</span>
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
              Jira 链接
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
            #{getParentDemandId(selectedTask.taskGroupId)} <span class="title-sub">{getParentDemandTitle(selectedTask.taskGroupId)}</span>
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
        <span class="value">{selectedTask.assignee}</span>
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
          展开代码提交轨迹与 MR 证据
        </button>
      </div>
      
      <!-- Evidence Chain 真实交付证据链 -->
      <div class="divider"></div>
      <div class="evidence-chain-section">
        <span class="decoupled-title font-mono">真实交付证据链 (Evidence Chain)</span>
        
        {#if evidenceChainLoading}
          <div class="evidence-chain-loading font-mono">正在检索关联的交付证据链...</div>
        {:else if evidenceChainError}
          <div class="evidence-chain-error font-mono">{evidenceChainError}</div>
        {:else if evidenceChain}
          {@const summary = evidenceChain.summary || {}}
          <div class="evidence-chain-summary-panel">
            <div class="chain-stat">
              <span class="stat-label">关联任务</span>
              <strong>{summary.related_tasks || 0}</strong>
            </div>
            <div class="chain-stat">
              <span class="stat-label">提交次数</span>
              <strong>{summary.commits || 0}</strong>
            </div>
            <div class="chain-stat">
              <span class="stat-label">Merge Request</span>
              <strong>{summary.merge_requests || 0} (已合并 {summary.merged_mrs || 0})</strong>
            </div>
          </div>
          
          {#if evidenceChain.evidence && evidenceChain.evidence.length > 0}
            <div class="evidence-timeline custom-scrollbar">
              {#each evidenceChain.evidence as log}
                <div class="timeline-node">
                  <div class="node-meta">
                    <span class="node-time font-mono">{log.created_at.slice(5, 16)}</span>
                    <span class="node-repo font-mono">[{log.repo}]</span>
                    {#if log.mr_url}
                      <a href={log.mr_url} target="_blank" rel="noopener noreferrer" class="mr-link-chain">MR</a>
                    {/if}
                  </div>
                  <div class="node-content">
                    <span class="node-action action-{log.action.toLowerCase()} font-mono">{log.action.replace('mr_', 'MR ')}</span>
                    {#if log.commit_id}
                      <span class="node-commit font-mono">commit: {log.commit_id.slice(0, 8)}</span>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <div class="evidence-chain-empty font-mono">暂无任何 GitLab 代码提交或 MR 合并记录事实。</div>
          {/if}
          
          {#if summary.signals && summary.signals.length > 0}
            <div class="evidence-chain-signals">
              {#each summary.signals as signal}
                <span class="signal-chip font-mono">{signal}</span>
              {/each}
            </div>
          {/if}
        {/if}
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
    border-left: 2px solid rgba(99, 102, 241, 0.3);
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

  .task-console .personnel-view {
    gap: 12px;
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
    color: var(--wa-text-main);
    display: grid;
    gap: var(--wa-space-4);
    margin-bottom: 0;
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

  .phase41-filter-row {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(180px, 1fr));
    gap: var(--wa-space-2);
  }

  .phase41-filter-row .custom-select-container {
    min-width: 0;
    width: 100%;
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

  .phase41-metric-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .phase41-metric {
    border-radius: var(--wa-radius-md);
    min-height: 112px;
  }

  .phase41-metric em {
    color: var(--wa-text-subtle);
    font-size: 12px;
    font-style: normal;
    font-weight: 720;
  }

  .phase41-stage-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .phase41-stage-card {
    min-width: 0;
    min-height: 72px;
    border-radius: var(--wa-radius-md);
    padding: var(--wa-space-3);
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 6px var(--wa-space-2);
    text-align: left;
    cursor: pointer;
  }

  .phase41-stage-card span {
    color: var(--wa-text-main);
    font-size: 13px;
    font-weight: 760;
  }

  .phase41-stage-card strong {
    color: var(--wa-text-strong);
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
    background: currentColor;
  }

  .phase41-workbench {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, var(--wa-inspector-w));
    gap: var(--wa-space-4);
    align-items: start;
  }

  .phase41-table-panel {
    min-width: 0;
  }

  .phase41-table-toolbar {
    border-radius: var(--wa-radius-md);
    align-items: center;
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
    grid-template-columns: minmax(220px, 1fr) minmax(260px, 1.1fr) minmax(170px, 0.7fr) auto;
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

  .phase41-risk-strip button {
    min-height: 28px;
    border: 0;
    border-radius: var(--wa-radius-sm);
    background: transparent;
    color: var(--wa-text-muted);
    padding: 0 9px;
    font-size: 12px;
    font-weight: 760;
    cursor: pointer;
    white-space: nowrap;
  }

  .phase41-risk-strip button.active {
    color: var(--wa-accent-strong);
    background: var(--wa-accent-soft);
  }

  .phase41-assignee-select {
    min-width: 0;
  }

  .phase41-table-shell {
    border-radius: var(--wa-radius-md);
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
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: var(--wa-space-2);
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

  .task-console .personnel-view {
    gap: var(--wa-space-3);
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

  @media (max-width: 1180px) {
    .phase41-console-header,
    .phase41-workbench {
      grid-template-columns: 1fr;
    }

    .phase41-inspector {
      position: relative;
      top: auto;
    }

    .phase41-metric-grid,
    .phase41-stage-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .phase41-execution-controls {
      grid-template-columns: minmax(220px, 1fr) minmax(260px, 1fr);
    }
  }

  @media (max-width: 760px) {
    .phase41-console-header,
    .phase41-table-toolbar {
      align-items: stretch;
    }

    .phase41-header-actions {
      justify-items: stretch;
    }

    .phase41-filter-row,
    .phase41-metric-grid,
    .phase41-stage-strip,
    .phase41-execution-controls,
    .phase41-fact-grid {
      grid-template-columns: 1fr;
    }

    .phase41-table-toolbar,
    .phase41-toolbar-actions {
      flex-direction: column;
      align-items: stretch;
    }
  }
</style>
