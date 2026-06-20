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
          await fetchDemands();
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
          await fetchDemands();
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
  let users: UserOption[] = [];
  let loading = false;
  let errorMsg = '';

  // Modal States
  let showCreateModal = false;
  let showScheduleModal = false;
  let selectedDemand: Demand | null = null;

  // Dropdown States
  let showAssigneeDropdown = false;
  let showTaskGroupDropdown = false;
  let activeDatePicker: 'new' | 'schedule' | null = null;
  let datePickerCursor = new Date();
  let taskGroups: string[] = [];

  // Create Form Fields
  let newTitle = '';
  let newDescription = '';
  let newAssignee = '';
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
        if (users.length > 0 && !newAssignee) {
          newAssignee = users[0].name; // Default to first user's name
        }
      }
    } catch (e) {
      console.error('Failed to fetch users:', e);
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
          repo: '',
          creator_dept: normalizeDepartment(currentUserDepartment)
        })
      });

      if (res.ok) {
        showCreateModal = false;
        newTitle = '';
        newDescription = '';
        updateNewDueDate('');
        await fetchDemands();
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
        await fetchDemands();
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

  function handleDocumentClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.custom-dropdown-container')) {
      showAssigneeDropdown = false;
      showTaskGroupDropdown = false;
    }
    if (!target.closest('.date-input-shell')) {
      activeDatePicker = null;
    }
  }

  onMount(() => {
    fetchDemands();
    fetchUsers();
    document.addEventListener('click', handleDocumentClick);
    // Poll updates every 15 seconds
    const interval = setInterval(fetchDemands, 15000);
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
      <button class="add-demand-btn font-mono" on:click={() => showCreateModal = true}>
        ➕ 录入新需求
      </button>
    {/if}
  </div>

  {#if loading && demands.length === 0}
    <div class="state-msg">加载需求大盘中...</div>
  {:else if errorMsg}
    <div class="state-msg error-msg font-mono">❌ {errorMsg}</div>
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
    <div class="modal-backdrop" on:click={() => showCreateModal = false}>
      <div class="modal-content glass-panel" on:click|stopPropagation>
        <div class="modal-header">
          <h3>📋 录入新产品需求</h3>
          <button class="close-btn" on:click={() => showCreateModal = false}>&times;</button>
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
            <label for="demand-assignee">指派负责人 <span class="text-rose">*</span></label>
            <div class="custom-dropdown-container" id="demand-assignee-container">
              <div 
                class="dropdown-trigger" 
                on:click|stopPropagation={() => showAssigneeDropdown = !showAssigneeDropdown}
              >
                <span>{newAssignee || '请选择负责人'}</span>
                <span class="arrow-icon {showAssigneeDropdown ? 'open' : ''}">▼</span>
              </div>
              {#if showAssigneeDropdown}
                <div class="dropdown-options-list glass-panel">
                  {#each users as u}
                    <div 
                      class="dropdown-option-item {newAssignee === u.name ? 'selected' : ''}"
                      on:click={() => {
                        newAssignee = u.name;
                        showAssigneeDropdown = false;
                      }}
                    >
                      {u.name}
                    </div>
                  {/each}
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
          <button class="cancel-btn font-mono" on:click={() => showCreateModal = false}>取消</button>
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
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal-content {
    width: 100%;
    max-width: 520px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    animation: zoomIn 0.2s cubic-bezier(0.16, 1, 0.3, 1);
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
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 8px;
    padding: 10px 12px;
    color: #e2e8f0;
    font-size: 0.8rem;
    outline: none;
    transition: all 0.2s;
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
    from { transform: scale(0.95); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }

  /* Custom Dropdown Styling */
  .custom-dropdown-container {
    position: relative;
    width: 100%;
  }

  .dropdown-trigger {
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
    transition: all 0.2s;
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
    padding: 8px 12px;
    color: #cbd5e1;
    font-size: 0.8rem;
    cursor: pointer;
    border-radius: 6px;
    transition: all 0.15s;
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
