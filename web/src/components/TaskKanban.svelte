<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
  import Modal from './shared/Modal.svelte';

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

  let allTasks: Task[] = [];
  let selectedProject = 'all';
  let selectedAssignee = 'all';
  let currentView = 'status';

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
    return tasks.filter(t => {
      const statusMatch = t.status.toLowerCase() === status.toLowerCase();
      if (assignee === "外部协同") {
        return !isCoreMember(t.assignee) && statusMatch;
      }
      return t.assignee === assignee && statusMatch;
    }).length;
  }

  function getAssigneeTaskCount(assignee: string, tasks: Task[]): number {
    return tasks.filter(t => {
      if (assignee === "外部协同") {
        return !isCoreMember(t.assignee);
      }
      return t.assignee === assignee;
    }).length;
  }

  function getAssigneeBugCount(assignee: string, tasks: Task[]): number {
    return tasks.filter(t => {
      const isBug = t.issueType === 'bug';
      if (assignee === "外部协同") {
        return !isCoreMember(t.assignee) && isBug;
      }
      return t.assignee === assignee && isBug;
    }).length;
  }

  function isAssigneeHighLoad(assignee: string, tasks: Task[]): boolean {
    return tasks.filter(t => {
      const activeMatch = t.status.toLowerCase() !== 'done';
      if (assignee === "外部协同") {
        return !isCoreMember(t.assignee) && activeMatch;
      }
      return t.assignee === assignee && activeMatch;
    }).length >= 5;
  }

  function isAssigneeDelayed(assignee: string, tasks: Task[]): boolean {
    return tasks.filter(t => {
      const delayMatch = getDelayDays(t.taskCreatedAt, t.status) >= 3;
      if (assignee === "外部协同") {
        return !isCoreMember(t.assignee) && delayMatch;
      }
      return t.assignee === assignee && delayMatch;
    }).length > 0;
  }

  let showProjectDropdown = false;
  let showAssigneeDropdown = false;
  let projectSelectEl: HTMLElement;
  let assigneeSelectEl: HTMLElement;

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

  // Reactive dropdown options populated from allTasks
  $: projectOptions = ['all', ...Array.from(new Set(allTasks.map(t => getProjectName(t.id))))];
  let coreMembers = new Set([
    "梁志远", "朱家聪", "岳颖颖", "Yue Yingying", "姜昊良", "白凌云", "陈伟华", 
    "李厚奇", "鲁俊", "刘子翔", "张路路", "qiang.deng", "MiddleQ", "zhongkou.chang", 
    "Eddie", "Antigravity"
  ]);

  function isCoreMember(name: string): boolean {
    if (!name) return false;
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
    ...Array.from(new Set(allTasks.filter(t => {
      if (isCoreMember(t.assignee)) return true;
      return t.status.toLowerCase() !== 'done';
    }).map(t => {
      if (!t.assignee) return null;
      if (isCoreMember(t.assignee)) return t.assignee;
      return "外部协同";
    }).filter(Boolean)))
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

  $: viewAssignees = (selectedAssignee === 'all'
    ? [
        ...Array.from(new Set(filteredTasks.filter(t => isCoreMember(t.assignee)).map(t => t.assignee))),
        ...(filteredTasks.some(t => !isCoreMember(t.assignee)) ? ["外部协同"] : [])
      ]
    : [selectedAssignee]
  );

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
    return filteredTasks
      .filter(t => {
        const statusMatch = t.status.toLowerCase() === status.toLowerCase();
        if (assignee === "外部协同") {
          return !isCoreMember(t.assignee) && statusMatch;
        }
        return t.assignee === assignee && statusMatch;
      })
      .sort((a, b) => {
        const timeA = a.taskCreatedAt ? new Date(a.taskCreatedAt).getTime() : 0;
        const timeB = b.taskCreatedAt ? new Date(b.taskCreatedAt).getTime() : 0;
        if (timeA === 0 && timeB === 0) return 0;
        if (timeA === 0) return 1;
        if (timeB === 0) return -1;
        return timeA - timeB; // Longest-running first
      });
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

  async function openDetails(task: Task) {
    selectedTask = task;
    showDetails = true;
    loadingCommits = true;
    selectedTaskCommits = [];
    try {
      const res = await fetch(`/api/tasks/commits?task_id=${task.id}`);
      if (res.ok) {
        selectedTaskCommits = await res.json();
      }
    } catch (e) {
      console.error('Failed to fetch commits:', e);
    } finally {
      loadingCommits = false;
    }
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

  async function fetchTasks() {
    try {
      const res = await fetch('/api/tasks');
      if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
      const data: TaskResponse[] = await res.json();
      
      allTasks = data.map(mapTask);
      errorMsg = '';
    } catch (e: any) {
      console.error('Failed to fetch tasks:', e);
      errorMsg = e.message || '连接 API 失败';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    fetchTasks();
    fetchConfig();
    intervalId = setInterval(fetchTasks, 5000);
    document.addEventListener('click', handleDocumentClick);
  });

  onDestroy(() => {
    if (intervalId) {
      clearInterval(intervalId);
    }
    document.removeEventListener('click', handleDocumentClick);
  });
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
        <button class="toggle-btn {currentView === 'status' ? 'active' : ''}" on:click={() => currentView = 'status'}>
          📊 状态视图
        </button>
        <button class="toggle-btn {currentView === 'personnel' ? 'active' : ''}" on:click={() => currentView = 'personnel'}>
          👤 人员视图
        </button>
      </div>

      <span class="header-desc">基于 Branch/Commit 自动流转</span>
      
      <!-- Project Filter -->
      <div class="custom-select-container" bind:this={projectSelectEl}>
        <button class="custom-select-trigger" on:click={toggleProjectDropdown} aria-label="项目筛选">
          <span class="filter-icon">📁</span>
          <span class="trigger-label">{selectedProject === 'all' ? '全部项目' : selectedProject}</span>
          <span class="select-arrow">{showProjectDropdown ? '▲' : '▼'}</span>
        </button>
        {#if showProjectDropdown}
          <div class="custom-select-options">
            {#each projectOptions as proj}
              <button 
                class="custom-option {selectedProject === proj ? 'active' : ''}" 
                on:click={() => selectProject(proj)}
              >
                {proj === 'all' ? '全部项目' : proj}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Assignee Filter -->
      <div class="custom-select-container" bind:this={assigneeSelectEl}>
        <button class="custom-select-trigger" on:click={toggleAssigneeDropdown} aria-label="经办人筛选">
          <span class="filter-icon">👤</span>
          <span class="trigger-label">{selectedAssignee === 'all' ? '全部经办人' : selectedAssignee}</span>
          <span class="select-arrow">{showAssigneeDropdown ? '▲' : '▼'}</span>
        </button>
        {#if showAssigneeDropdown}
          <div class="custom-select-options">
            {#each assigneeOptions as ass}
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
            if (currentView === 'assignee') {
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

  {#if currentView === 'status'}
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
                <span class="collapse-arrow">{collapsedAssignees[assignee] === false ? '▼' : '▶'}</span>
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
      {#if getParentDemand(selectedTask.taskGroupId)}
        {@const parentDemand = getParentDemand(selectedTask.taskGroupId)}
        <div class="details-row">
          <span class="label">关联需求:</span>
          <span class="value parent-demand-detail font-mono">
            📋 #{parentDemand.id} <span class="title-sub">{parentDemand.title}</span>
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
      <div class="section-title">Git Telemetry 提交时序与多模块轨迹</div>
      
      {#if loadingCommits}
        <div class="loading-commits">
          <span class="spinner"></span> 正在拉取最新的 Git 遥测明细...
        </div>
      {:else if selectedTaskCommits.length > 0}
        <div class="commit-timeline">
          {#each selectedTaskCommits as log}
            <div class="timeline-item">
              <div class="timeline-badge-container">
                <span class="timeline-badge badge-{log.action}">{log.action === 'git_push' ? 'Push' : 'MR'}</span>
                <span class="timeline-time font-mono">{formatTimeBrief(log.created_at)}</span>
              </div>
              <div class="timeline-content">
                <div class="timeline-meta">
                  <span class="meta-repo">📁 {log.repo}</span>
                  <span class="meta-branch">🌿 {log.branch}</span>
                  {#if log.commit_id}
                    <span class="meta-hash font-mono" title="Commit Hash">{log.commit_id.substring(0, 8)}</span>
                  {/if}
                </div>
                <div class="timeline-body font-mono">
                  {#if log.mr_url}
                    <a href={log.mr_url} target="_blank" rel="noopener noreferrer" class="mr-timeline-link">
                      !{log.mr_iid}: {log.message}
                    </a>
                  {:else}
                    {log.message}
                  {/if}
                </div>
                <div class="timeline-footer">
                  <span>👤 提交人: {log.author}</span>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <div class="empty-commits-info">
          ℹ️ 该任务目前处于 Jira 状态，暂无关联的代码提交 (Git Telemetry) 记录。
        </div>
      {/if}
      
      <div class="details-row">
        <span class="label">系统最后同步:</span>
        <span class="value">{formatTimeFull(selectedTask.rawLastUpdate)}</span>
      </div>
    </div>
  </Modal>
{/if}

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

  .collapse-arrow {
    font-size: 0.65rem;
    color: #64748b;
    margin-right: 4px;
    transition: transform 0.2s ease;
    display: inline-block;
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
</style>
