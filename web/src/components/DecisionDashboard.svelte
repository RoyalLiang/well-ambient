<script lang="ts">
  import { onMount } from 'svelte';
  import type {
    AdminInspectorRecord,
    AdminMetric,
    AdminTableColumn,
    AdminTableRow,
    AdminTone
  } from '../lib/admin-console/contract';
  import {
    ADMIN_TONE_CLASS,
    formatAdminDate,
    toneForRisk,
    toneForStatus
  } from '../lib/admin-console/contract';
  import Alert from './shared/Alert.svelte';
  import DatePicker from './shared/DatePicker.svelte';
  import Select from './shared/Select.svelte';

  export let currentUser = '';

  interface TelemetrySnippet {
    branch: string;
    last_update: string;
  }

  interface AgendaItem {
    task_id: string;
    title: string;
    assignee: string;
    repo?: string;
    status: string;
    issue_type: string;
    risk_level: string;
    risk_type: string;
    desc: string;
    telemetry_snippet: TelemetrySnippet;
    due_date: string | null;
    decision_logs: string;
  }

  interface AutoDecision {
    time: string;
    task_id: string;
    message: string;
    assignee?: string;
    commit_id?: string;
    commit_url?: string;
  }

  interface ResolutionDraft {
    assignee: string;
    due_date: string;
    note: string;
  }

  interface AgendaAdminRow extends AdminTableRow {
    source: AgendaItem;
  }

  interface ProjectConfigRecord {
    project_key: string;
    project_name: string;
  }

  const agendaColumns: AdminTableColumn[] = [
    { key: 'task_id', label: '事项', width: '132px' },
    { key: 'title', label: '标题', width: '34%' },
    { key: 'owner', label: '负责人', width: '112px' },
    { key: 'risk', label: '风险', width: '96px' },
    { key: 'due', label: '计划日', width: '116px' },
    { key: 'status', label: '状态', width: '96px' }
  ];

  let agendaItems: AgendaItem[] = [];
  let autoDecisions: AutoDecision[] = [];
  let loading = true;
  let configReady = false;
  let errorMsg = '';

  let selectedItem: AgendaItem | null = null;
  let decisionLoading = false;
  let decisionSuccess = '';
  let decisionError = '';

  let newAssignee = '';
  let newDueDate = '';
  let decisionNote = '';
  let operator = '';
  let localDecisions: string[] = [];

  let currentFilter: 'all' | 'task' | 'bug' = 'all';
  let selectedAssignee = 'all';
  let selectedRepo = 'all';
  let showRiskLevel: 'all' | 'risks' = 'risks';
  let searchText = '';
  let inspectorContentEl: HTMLElement;
  let decisionPanelHeight = 'clamp(640px, calc(100dvh - 112px), 820px)';

  let coreMembers = new Set([
    '梁志远',
    '朱家聪',
    '岳颖颖',
    'Yue Yingying',
    '姜昊良',
    '白凌云',
    '陈伟华',
    '李厚奇',
    '鲁俊',
    '刘子翔',
    '张路路',
    'qiang.deng',
    'MiddleQ',
    'zhongkou.chang',
    'Eddie',
    'Antigravity'
  ]);

  let jiraBaseUrl = '';
  let gitlabBaseUrl = '';
  let projectNamesMap: Record<string, string> = {};

  $: if (currentUser && !operator) {
    operator = currentUser;
  }

  $: filteredAgendaItems = configReady ? agendaItems.filter((item) => isVisibleAgendaItem(item)) : [];
  $: filteredAutoDecisions = autoDecisions.filter((dec) => !dec.assignee || isCoreMember(dec.assignee));

  $: activeBugCount = filteredAgendaItems.filter((item) => item.issue_type === 'bug').length;
  $: activeTaskCount = filteredAgendaItems.filter((item) => item.issue_type !== 'bug').length;
  $: redZoneCount = filteredAgendaItems.filter((item) => item.risk_level === 'critical').length;
  $: warningCount = filteredAgendaItems.filter((item) => item.risk_level === 'warning').length;
  $: visibleRiskCount = filteredAgendaItems.filter((item) => item.risk_level !== 'safe').length;
  $: totalActiveTasks = activeBugCount + activeTaskCount;
  $: decisionHealthPercent =
    totalActiveTasks > 0
      ? Math.max(8, Math.round(((totalActiveTasks - redZoneCount) / totalActiveTasks) * 100))
      : 100;
  $: decisionHealthLabel = redZoneCount > 0 ? `${redZoneCount} 个高风险待处理` : '暂无高风险';

  $: assigneesList = [
    'all',
    ...Array.from(new Set(filteredAgendaItems.map((item) => item.assignee).filter(Boolean))).sort((a, b) =>
      a.localeCompare(b)
    )
  ];
  $: projectList = [
    'all',
    ...Array.from(
      new Set(filteredAgendaItems.map((item) => getAgendaProjectKey(item)).filter((project): project is string => !!project))
    ).sort((a, b) => a.localeCompare(b))
  ];
  $: assigneeOptions = assigneesList.map((name) => ({
    value: name,
    label: name === 'all' ? '全部负责人' : name
  }));
  $: projectOptions = projectList.map((project) => ({
    value: project,
    label: project === 'all' ? '全部项目' : getProjectDisplayName(project)
  }));
  $: if (selectedRepo !== 'all' && !projectList.includes(selectedRepo)) {
    selectedRepo = 'all';
  }
  $: if (selectedAssignee !== 'all' && !assigneesList.includes(selectedAssignee)) {
    selectedAssignee = 'all';
  }
  $: overrideAssigneeOptions = Array.from(
    new Set([
      selectedItem?.assignee,
      ...Array.from(coreMembers),
      ...assigneesList.filter((name) => name !== 'all')
    ].filter(Boolean) as string[])
  ).sort((a, b) => a.localeCompare(b));
  $: overrideAssigneeSelectOptions = [
    { value: '', label: '选择负责人' },
    ...overrideAssigneeOptions.map((name) => ({ value: name, label: name }))
  ];

  $: filteredItems = filteredAgendaItems
    .filter((item) => {
      if (currentFilter === 'task' && item.issue_type === 'bug') return false;
      if (currentFilter === 'bug' && item.issue_type !== 'bug') return false;
      if (selectedAssignee !== 'all' && item.assignee !== selectedAssignee) return false;
      if (selectedRepo !== 'all' && getAgendaProjectKey(item) !== selectedRepo) return false;
      if (showRiskLevel === 'risks' && item.risk_level === 'safe') return false;

      const needle = searchText.trim().toLowerCase();
      if (needle) {
        const haystack = [
          item.task_id,
          item.title,
          item.assignee,
          item.status,
          getAgendaProjectKey(item),
          getAgendaProjectLabel(item)
        ]
          .join(' ')
          .toLowerCase();
        if (!haystack.includes(needle)) return false;
      }

      return true;
    })
    .sort((a, b) => riskPriority(b) - riskPriority(a));

  $: adminMetrics = adaptMetrics();
  $: agendaRows = filteredItems.map(adaptAgendaRow);
  $: inspectorRecord = selectedItem ? adaptInspectorRecord(selectedItem) : null;
  $: resolutionDraft = getResolutionDraft(selectedItem);

  function isCoreMember(name: string): boolean {
    if (!name) return false;
    return coreMembers.has(name) || coreMembers.has(name.split(' ')[0]);
  }

  function isStableJiraIssueKey(taskId: string): boolean {
    const value = (taskId || '').trim();
    const match = value.match(/^([A-Z][A-Z0-9_]*)-\d+$/);
    if (!match) return false;
    const projectKey = match[1];
    return projectKey !== 'TASK' && projectKey !== 'DEMAND';
  }

  function isVisibleAgendaItem(item: AgendaItem): boolean {
    return isStableJiraIssueKey(item.task_id) && isCoreMember(item.assignee);
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
      users = users.filter((name) => name !== '未指派' && name !== '-');
      coreMembers = new Set(users);
    }
  }

  async function fetchConfig() {
    try {
      const jiraRes = await fetch('/api/jira/link-config');
      if (jiraRes.ok) {
        const linkConfig = await jiraRes.json();
        jiraBaseUrl = linkConfig?.base_url ? linkConfig.base_url.replace(/\/+$/, '') : '';
      }

      const res = await fetch('/api/config');
      if (res.ok) {
        const data = await res.json();
        if (data) {
          updateCoreMembers(data);
          if (!jiraBaseUrl) {
            jiraBaseUrl = data?.jira?.base_url ? data.jira.base_url.replace(/\/+$/, '') : '';
          }
          gitlabBaseUrl = data?.gitlab?.base_url ? data.gitlab.base_url.replace(/\/+$/, '') : '';
        }
      }

      const projectsRes = await fetch('/api/projects/config');
      if (projectsRes.ok) {
        const projects: ProjectConfigRecord[] = await projectsRes.json();
        projectNamesMap = Array.isArray(projects)
          ? projects.reduce((acc, project) => {
              const key = normalizeProjectKey(project.project_key);
              const name = cleanProjectName(project.project_name);
              if (key && name) acc[key] = name;
              return acc;
            }, {} as Record<string, string>)
          : {};
      }
    } catch (e) {
      console.error('Failed to fetch config on dashboard:', e);
    } finally {
      configReady = true;
    }
  }

  async function fetchAgenda() {
    loading = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/agenda/summary');
      if (!res.ok) throw new Error('加载决策看板数据失败');
      const data = await res.json();

      const nextAgendaItems: AgendaItem[] = Array.isArray(data.agenda_items) ? data.agenda_items : [];
      const nextAutoDecisions: AutoDecision[] = Array.isArray(data.auto_decisions) ? data.auto_decisions : [];
      const visible = nextAgendaItems.filter((item) => isVisibleAgendaItem(item));

      agendaItems = nextAgendaItems;
      autoDecisions = nextAutoDecisions;

      if (visible.length > 0 && !selectedItem) {
        selectItem(visible[0]);
      } else if (selectedItem) {
        const updated = visible.find((item) => item.task_id === selectedItem?.task_id);
        selectedItem = updated || visible[0] || null;
        if (selectedItem) syncInterventionDraft(selectedItem);
      }
    } catch (err: any) {
      errorMsg = err.message || '网络连接异常';
    } finally {
      loading = false;
    }
  }

  function selectItem(item: AgendaItem) {
    selectedItem = item;
    syncInterventionDraft(item);
    decisionSuccess = '';
    decisionError = '';
  }

  function syncInterventionDraft(item: AgendaItem) {
    newAssignee = item.assignee || '';
    const dueDate = item.due_date ? formatAdminDate(item.due_date) : '';
    newDueDate = dueDate === '-' ? '' : dueDate;
    decisionNote = '';
  }

  async function submitDecision(action: 'reassign' | 'reschedule') {
    if (!selectedItem) return;
    decisionLoading = true;
    decisionSuccess = '';
    decisionError = '';

    let value = '';
    if (action === 'reassign') {
      if (!newAssignee.trim()) {
        decisionError = '请输入转派负责人';
        decisionLoading = false;
        return;
      }
      value = newAssignee;
    } else {
      if (!newDueDate) {
        decisionError = '请选择新的截止时间';
        decisionLoading = false;
        return;
      }
      value = newDueDate;
    }

    try {
      const res = await fetch('/api/strongest-brain/intervention', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          task_id: selectedItem.task_id,
          action,
          value,
          reason: decisionNote || '人工调停干预'
        })
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || '提交人工干预指令失败');
      }

      await res.json();
      decisionSuccess = '人工干预已记录至事件账本并更新状态。';

      const actionName = action === 'reassign' ? '覆盖指派' : '调整截止期';
      localDecisions = [
        `[${new Date().toLocaleTimeString()}] ${operator || '系统管理员'} 调停 ${selectedItem.task_id}：${actionName}`,
        ...localDecisions
      ];

      await fetchAgenda();
    } catch (err: any) {
      decisionError = err.message || '提交干预请求时出错';
    } finally {
      decisionLoading = false;
    }
  }

  function adaptMetrics(): AdminMetric[] {
    return [
      {
        label: '决策事项',
        value: filteredAgendaItems.length,
        helper: `需求 ${activeTaskCount} / 缺陷 ${activeBugCount}`,
        tone: 'info'
      },
      {
        label: '高风险',
        value: redZoneCount,
        helper: warningCount > 0 ? `另有 ${warningCount} 个中风险` : 'critical 卡点',
        tone: redZoneCount > 0 ? 'danger' : 'success'
      },
      {
        label: '流转健康度',
        value: `${decisionHealthPercent}%`,
        helper: decisionHealthLabel,
        tone: decisionHealthPercent < 80 ? 'warning' : 'success'
      },
      {
        label: '自动记录',
        value: filteredAutoDecisions.length,
        helper: '核心成员可见事件',
        tone: 'neutral'
      }
    ];
  }

  function adaptAgendaRow(item: AgendaItem): AgendaAdminRow {
    const tone = toneForAgendaItem(item);
    return {
      id: item.task_id,
      title: item.title || item.task_id,
      status: getStatusDisplay(item.status),
      tone,
      owner: item.assignee || '-',
      dueDate: formatAdminDate(item.due_date),
      priority: getIssueTypeLabel(item.issue_type),
      risk: getRiskLevelLabel(item.risk_level),
      source: item,
      cells: {
        task_id: item.task_id,
        title: item.title || '-',
        priority: getIssueTypeLabel(item.issue_type),
        owner: item.assignee || '-',
        risk: getRiskLevelLabel(item.risk_level),
        due: formatAdminDate(item.due_date),
        status: getStatusDisplay(item.status),
        stale: getStaleText(item)
      }
    };
  }

  function adaptInspectorRecord(item: AgendaItem): AdminInspectorRecord {
    const logs = splitDecisionLogs(item.decision_logs);
    return {
      id: item.task_id,
      title: item.title || item.task_id,
      status: getStatusDisplay(item.status),
      tone: toneForAgendaItem(item),
      facts: [
        { label: '负责人', value: item.assignee || '-' },
        { label: '所属项目', value: getAgendaProjectLabel(item) || '-' },
        { label: '事项类型', value: getIssueTypeLabel(item.issue_type) },
        { label: '风险类型', value: getRiskTypeLabel(item.risk_type) },
        { label: '计划完成日', value: formatAdminDate(item.due_date) },
        { label: '静默时长', value: getStaleText(item) }
      ],
      sections: [
        {
          title: '需求描述',
          body: item.desc || '后端暂无描述。'
        },
        {
          title: '处置建议',
          items: [getTriageRecommendation(item)]
        },
        {
          title: '流转摘要',
          items: [
            getFlowSuggestion(item),
            `当前负责人 ${item.assignee || '未指派'}，风险类型为 ${getRiskTypeLabel(item.risk_type)}。`,
            item.telemetry_snippet?.branch ? `关联分支：${item.telemetry_snippet.branch}` : '暂无关联分支信息。'
          ]
        },
        {
          title: '人工干预记录',
          items: logs.length > 0 ? logs : ['暂无人工干预记录']
        }
      ],
      actions: [
        { label: '覆盖指派', kind: 'primary' },
        { label: '调整截止', kind: 'secondary' }
      ]
    };
  }

  function getResolutionDraft(item: AgendaItem | null): ResolutionDraft {
    if (!item) return { assignee: '', due_date: '', note: '' };
    const existingDue = item.due_date ? formatAdminDate(item.due_date) : '';
    return {
      assignee: item.assignee || '',
      due_date: existingDue && existingDue !== '-' ? existingDue : '',
      note:
        item.risk_level === 'critical'
          ? '会中确认阻塞事实，并明确下一位负责人或新的截止时间。'
          : '会中确认推进节奏，补齐下一次状态更新时间。'
    };
  }

  function applyResolutionDraft() {
    if (!selectedItem) return;
    newAssignee = resolutionDraft.assignee;
    newDueDate = resolutionDraft.due_date;
    decisionNote = resolutionDraft.note;
    decisionSuccess = '已带入处置草稿，尚未提交。';
    decisionError = '';
  }

  function getAgendaProjectKey(item: AgendaItem) {
    const taskId = (item.task_id || '').trim();
    const delimiterIndex = taskId.indexOf('-');
    if (delimiterIndex <= 0) return '';
    const key = taskId.slice(0, delimiterIndex).trim().toUpperCase();
    if (!key || key === 'TASK' || key === 'DEMAND') return '';
    return key;
  }

  function normalizeProjectKey(value: string | undefined) {
    return (value || '').trim().toUpperCase();
  }

  function cleanProjectName(value: string | undefined) {
    const name = (value || '').trim();
    return name.replace(/\s*\([^)]*\)\s*$/, '').trim();
  }

  function getProjectDisplayName(projectKey: string) {
    const key = normalizeProjectKey(projectKey);
    if (!key || key === 'ALL') return '';
    const configuredName = cleanProjectName(projectNamesMap[key]);
    if (configuredName && configuredName !== `${key}项目`) return configuredName;
    return key;
  }

  function getAgendaProjectLabel(item: AgendaItem) {
    const key = getAgendaProjectKey(item);
    if (!key) return item.repo || '';
    return getProjectDisplayName(key) || item.repo || key;
  }

  function getIssueTypeLabel(issueType: string) {
    return issueType === 'bug' ? '缺陷' : '需求';
  }

  function getRiskLevelLabel(riskLevel: string) {
    switch (riskLevel) {
      case 'critical':
        return '高风险';
      case 'warning':
        return '中风险';
      case 'safe':
        return '低风险';
      default:
        return riskLevel || '未分级';
    }
  }

  function getRiskTypeLabel(riskType: string) {
    switch (riskType) {
      case 'no_commit_48h':
        return '连续静默';
      case 'overdue':
        return '周期超时';
      case 'potential_conflict':
        return '协作冲突';
      case 'none':
        return '常规观察';
      default:
        return riskType || '常规观察';
    }
  }

  function getStatusDisplay(status: string) {
    switch ((status || '').toLowerCase()) {
      case 'backlog':
        return '待排期';
      case 'progress':
        return '推进中';
      case 'review':
        return '评审中';
      case 'done':
        return '已完成';
      default:
        return status ? status.toUpperCase() : '未知';
    }
  }

  function getStaleHours(item: AgendaItem) {
    const lastUpdate = item.telemetry_snippet?.last_update;
    if (!lastUpdate) return 0;
    const timestamp = new Date(lastUpdate).getTime();
    if (Number.isNaN(timestamp)) return 0;
    return Math.max(0, Math.round((Date.now() - timestamp) / 36e5));
  }

  function getStaleText(item: AgendaItem) {
    const staleHours = getStaleHours(item);
    return staleHours > 0 ? `${staleHours}h` : '等待更新';
  }

  function riskPriority(item: AgendaItem) {
    if (item.risk_level === 'critical') return 3;
    if (item.risk_level === 'warning') return 2;
    return 1;
  }

  function toneForAgendaItem(item: AgendaItem): AdminTone {
    if (item.risk_level === 'critical') return 'danger';
    if (item.risk_level === 'warning') return 'warning';
    if (item.risk_level === 'safe') return 'success';
    return toneForRisk(item.risk_type || item.risk_level);
  }

  function toneClass(tone: AdminTone | undefined) {
    return ADMIN_TONE_CLASS[tone || 'neutral'];
  }

  function getStatusToneClass(status: string) {
    return toneClass(toneForStatus(status));
  }

  function getTriageRecommendation(item: AgendaItem): string {
    if (item.risk_type === 'no_commit_48h') {
      return '先确认是否存在依赖、口径或责任边界阻塞，再决定是否转派或增加协助人。';
    }
    if (item.risk_type === 'overdue') {
      return item.issue_type === 'bug'
        ? '优先拆出复现、定位、修复、回归四段，并锁定当天可验证的下一步。'
        : '先拆分可独立验收的范围，把争议范围移入下一轮确认。';
    }
    if (item.risk_type === 'potential_conflict') {
      return '指定统一协调人，对齐范围、验收顺序和责任边界后再推进。';
    }
    return '保持后台同步，只有事实缺口、负责人变更、延期或验收风险出现时再人工介入。';
  }

  function getFlowSuggestion(item: AgendaItem): string {
    const dueText = item.due_date ? formatAdminDate(item.due_date) : '未设置截止日';
    return `当前状态为 ${getStatusDisplay(item.status)}，计划完成日 ${dueText}，建议在下一次状态更新前保留这条看板记录。`;
  }

  function splitDecisionLogs(value: string) {
    return (value || '')
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);
  }

  function getJiraUrl(taskId: string) {
    if (!jiraBaseUrl || !taskId || taskId.startsWith('TASK-')) return '';
    return `${jiraBaseUrl}/browse/${taskId}`;
  }

  function shortCommit(commitID?: string) {
    return commitID ? commitID.slice(0, 8) : '';
  }

  function normalizedCommitUrl(rawUrl?: string) {
    const url = (rawUrl || '').trim();
    if (!url || /^about:blank$/i.test(url)) return '';
    const commitUrl = url.replace(/\/-\/commit\//g, '/commit/');
    if (/^https?:\/\//i.test(commitUrl)) return commitUrl;
    if (commitUrl.startsWith('//')) return `https:${commitUrl}`;
    if (commitUrl.startsWith('/') && gitlabBaseUrl) return `${gitlabBaseUrl}${commitUrl}`;
    if (commitUrl.includes('/commit/') && gitlabBaseUrl) return `${gitlabBaseUrl}/${commitUrl.replace(/^\/+/, '')}`;
    if (/^[\w.-]+(?::\d+)?\//.test(commitUrl)) return `https://${commitUrl}`;
    return '';
  }

  function openCommitUrl(event: MouseEvent, rawUrl?: string) {
    const url = normalizedCommitUrl(rawUrl);
    if (!url) {
      event.preventDefault();
      return;
    }
    event.preventDefault();
    window.open(url, '_blank', 'noopener,noreferrer');
  }

  onMount(() => {
    let disposed = false;
    let interval: ReturnType<typeof setInterval> | null = null;
    let resizeObserver: ResizeObserver | null = null;
    let measureFrame = 0;

    const syncDecisionPanelHeight = () => {
      if (measureFrame) {
        window.cancelAnimationFrame(measureFrame);
      }
      measureFrame = window.requestAnimationFrame(() => {
        if (!inspectorContentEl) return;
        const panelChromeHeight = 34;
        const nextHeight = Math.max(620, Math.ceil(inspectorContentEl.scrollHeight + panelChromeHeight));
        decisionPanelHeight = `${nextHeight}px`;
      });
    };

    const bootstrapDashboard = async () => {
      await fetchConfig();
      if (disposed) return;
      await fetchAgenda();
      if (disposed) return;
      syncDecisionPanelHeight();
      interval = setInterval(() => {
        fetchAgenda();
      }, 15000);
    };

    syncDecisionPanelHeight();
    if (typeof ResizeObserver !== 'undefined' && inspectorContentEl) {
      resizeObserver = new ResizeObserver(syncDecisionPanelHeight);
      resizeObserver.observe(inspectorContentEl);
    }
    window.addEventListener('resize', syncDecisionPanelHeight);

    bootstrapDashboard();

    return () => {
      disposed = true;
      if (interval) clearInterval(interval);
      if (resizeObserver) resizeObserver.disconnect();
      if (measureFrame) window.cancelAnimationFrame(measureFrame);
      window.removeEventListener('resize', syncDecisionPanelHeight);
    };
  });
</script>

<div class="decision-admin">
  <section class="decision-command-strip wa-glass" aria-label="决策看板概览">
    <div class="command-copy">
      <span class="command-kicker">实时 Agenda</span>
      <h2>核心决策流转</h2>
      <span class="command-subline">{decisionHealthLabel}</span>
    </div>
    <div class="command-signal-grid">
      <div class="command-signal">
        <span>可见事项</span>
        <strong>{agendaRows.length}</strong>
      </div>
      <div class="command-signal tone-danger">
        <span>高风险</span>
        <strong>{visibleRiskCount}</strong>
      </div>
      <div class="command-signal tone-info">
        <span>自动记录</span>
        <strong>{filteredAutoDecisions.length}</strong>
      </div>
    </div>
  </section>

  <section class="decision-metrics" aria-label="决策看板指标">
    {#each adminMetrics as metric}
      <article class="wa-admin-card wa-admin-metric decision-metric metric-{metric.tone || 'neutral'}">
        <div class="metric-topline">
          <span>{metric.label}</span>
          <i aria-hidden="true"></i>
        </div>
        <strong>{metric.value}</strong>
        <small>{metric.helper}</small>
      </article>
    {/each}
  </section>

  <section class="decision-stage-strip" aria-label="状态分段">
    <button
      type="button"
      class="stage-chip {currentFilter === 'all' && showRiskLevel === 'all' ? 'active' : ''}"
      on:click={() => {
        currentFilter = 'all';
        showRiskLevel = 'all';
      }}
    >
      <span>全部活跃</span>
      <strong>{filteredAgendaItems.length}</strong>
    </button>
    <button
      type="button"
      class="stage-chip {showRiskLevel === 'risks' ? 'active' : ''}"
      on:click={() => {
        currentFilter = 'all';
        showRiskLevel = 'risks';
      }}
    >
      <span>待处理风险</span>
      <strong>{visibleRiskCount}</strong>
    </button>
    <button
      type="button"
      class="stage-chip {currentFilter === 'task' ? 'active' : ''}"
      on:click={() => {
        currentFilter = 'task';
        showRiskLevel = 'all';
      }}
    >
      <span>需求</span>
      <strong>{activeTaskCount}</strong>
    </button>
    <button
      type="button"
      class="stage-chip {currentFilter === 'bug' ? 'active' : ''}"
      on:click={() => {
        currentFilter = 'bug';
        showRiskLevel = 'all';
      }}
    >
      <span>缺陷</span>
      <strong>{activeBugCount}</strong>
    </button>
  </section>

  <div class="decision-main-grid" style={`--decision-panel-height: ${decisionPanelHeight};`}>
    <section class="wa-admin-section decision-table-section" aria-label="决策事项列表">
      <div class="wa-admin-toolbar decision-toolbar">
        <div class="toolbar-copy">
          <span>Selection</span>
          <strong>事项选择列表</strong>
          {#if loading}
            <small>正在刷新</small>
          {:else}
            <small>{agendaRows.length} / {filteredAgendaItems.length} 条可见事项</small>
          {/if}
        </div>

        <div class="toolbar-controls">
          <input class="wa-control search-control" type="search" bind:value={searchText} placeholder="搜索编号、标题、负责人" />
          <div class="toolbar-select">
            <Select
              bind:value={selectedAssignee}
              options={assigneeOptions}
              placeholder="全部负责人"
              searchPlaceholder="搜索负责人"
              compact={true}
            />
          </div>
          <div class="toolbar-select project-select">
            <Select
              bind:value={selectedRepo}
              options={projectOptions}
              placeholder="全部项目"
              searchPlaceholder="搜索项目名"
              compact={true}
            />
          </div>
          <button class="wa-admin-action secondary" type="button" on:click={fetchAgenda} disabled={loading}>
            刷新
          </button>
        </div>
      </div>

      {#if errorMsg}
        <Alert type="error" message={errorMsg} />
      {/if}

      <div class="wa-admin-table-shell decision-table-shell">
        <table class="wa-admin-table">
          <thead>
            <tr>
              <th class="select-col" aria-label="选择"></th>
              {#each agendaColumns as column}
                <th
                  class:sticky-status-col={column.key === 'status'}
                  style={column.width ? `width: ${column.width}` : undefined}
                >
                  {column.label}
                </th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#if loading && agendaRows.length === 0}
              <tr>
                <td colspan={agendaColumns.length + 1}>
                  <div class="table-state">正在加载真实 Agenda 数据...</div>
                </td>
              </tr>
            {:else if agendaRows.length === 0}
              <tr>
                <td colspan={agendaColumns.length + 1}>
                  <div class="table-state">当前过滤条件下没有可见决策事项。</div>
                </td>
              </tr>
            {:else}
              {#each agendaRows as row}
                <tr
                  class:is-selected={selectedItem?.task_id === row.id}
                  on:click={() => selectItem(row.source)}
                  on:keydown={(event) => event.key === 'Enter' && selectItem(row.source)}
                  role="button"
                  tabindex="0"
                >
                  <td class="select-col">
                    <span class="row-check" class:checked={selectedItem?.task_id === row.id}></span>
                  </td>
                  {#each agendaColumns as column}
                    <td class="cell-{column.key}" class:sticky-status-col={column.key === 'status'}>
                      {#if column.key === 'task_id'}
                        {@const jiraUrl = getJiraUrl(row.id)}
                        {#if jiraUrl}
                          <a class="table-link" href={jiraUrl} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                            {row.cells.task_id}
                          </a>
                        {:else}
                          <span class="table-id">{row.cells.task_id}</span>
                        {/if}
                      {:else if column.key === 'title'}
                        <div class="title-stack">
                          <strong>{row.title}</strong>
                        </div>
                      {:else if column.key === 'risk'}
                        <span class="wa-admin-pill {toneClass(row.tone)}">{row.risk}</span>
                      {:else if column.key === 'status'}
                        <span class="wa-admin-pill {getStatusToneClass(row.status)}">{row.status}</span>
                      {:else}
                        <span>{row.cells[column.key]}</span>
                      {/if}
                    </td>
                  {/each}
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </section>

    <aside class="wa-admin-card wa-admin-inspector decision-inspector" aria-label="事项详情">
      <div class="inspector-content" bind:this={inspectorContentEl}>
        {#if inspectorRecord && selectedItem}
          <div class="inspector-head">
            <div>
              <span class="inspector-kicker">需求详情</span>
              <h2>{inspectorRecord.title}</h2>
            </div>
            <span class="wa-admin-pill {toneClass(inspectorRecord.tone)}">{inspectorRecord.status}</span>
          </div>

          <div class="inspector-id-row">
            {#if getJiraUrl(inspectorRecord.id)}
              <a class="table-link" href={getJiraUrl(inspectorRecord.id)} target="_blank" rel="noopener noreferrer">{inspectorRecord.id}</a>
            {:else}
              <strong>{inspectorRecord.id}</strong>
            {/if}
            <span class="wa-admin-pill {toneClass(inspectorRecord.tone)}">{getRiskLevelLabel(selectedItem.risk_level)}</span>
          </div>

          <dl class="fact-grid">
            {#each inspectorRecord.facts as fact}
              <div>
                <dt>{fact.label}</dt>
                <dd>{fact.value}</dd>
              </div>
            {/each}
          </dl>

          <div class="inspector-section-grid">
            {#each inspectorRecord.sections as section}
              <section class="inspector-section">
                <h3>{section.title}</h3>
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
          </div>

          <section class="inspector-section intervention-section">
            <div class="section-row-head">
              <h3>人工调停</h3>
              <button class="wa-admin-action secondary" type="button" on:click={applyResolutionDraft}>
                带入草稿
              </button>
            </div>

            <div class="intervention-form">
              <div class="form-field">
                <Select
                  label="指派负责人"
                  bind:value={newAssignee}
                  options={overrideAssigneeSelectOptions}
                  placeholder="选择负责人"
                  searchPlaceholder="搜索负责人"
                  compact={true}
                />
              </div>
              <div class="form-field">
                <DatePicker
                  label="新的截止日"
                  bind:value={newDueDate}
                  placeholder="选择新的截止日"
                  compact={true}
                />
              </div>
              <label class="wide-field">
                <span>会议备注</span>
                <input class="wa-control" type="text" bind:value={decisionNote} placeholder="填写调停依据或下一步动作" />
              </label>
              <label>
                <span>调停决策人</span>
                <input class="wa-control" type="text" bind:value={operator} />
              </label>
            </div>

            <div class="intervention-actions">
              <button class="wa-admin-action primary" type="button" on:click={() => submitDecision('reassign')} disabled={decisionLoading}>
                覆盖指派
              </button>
              <button class="wa-admin-action secondary" type="button" on:click={() => submitDecision('reschedule')} disabled={decisionLoading}>
                调整截止
              </button>
            </div>

            {#if decisionSuccess}
              <Alert type="success" message={decisionSuccess} />
            {/if}
            {#if decisionError}
              <Alert type="error" message={decisionError} />
            {/if}
          </section>
        {:else}
          <div class="inspector-empty">
            <strong>暂无可查看事项</strong>
            <p>当前数据或过滤条件下没有真实 Agenda 记录。</p>
          </div>
        {/if}
      </div>
    </aside>
  </div>

  <section class="decision-log-grid" aria-label="决策流转记录">
    <div class="wa-admin-card log-panel auto-log-panel">
      <div class="log-head">
        <div>
          <strong>自动流转记录</strong>
          <small>系统检测、提交证据与状态推进</small>
        </div>
        <span>{filteredAutoDecisions.length} 条</span>
      </div>
      <div class="log-list">
        {#if filteredAutoDecisions.length === 0}
          <div class="log-empty">当前没有可见自动流转记录。</div>
        {:else}
          {#each filteredAutoDecisions as dec}
            <article class="log-item">
              <div class="log-meta">
                <span>{dec.time || '-'}</span>
                {#if getJiraUrl(dec.task_id)}
                  <a href={getJiraUrl(dec.task_id)} target="_blank" rel="noopener noreferrer">{dec.task_id}</a>
                {:else}
                  <strong>{dec.task_id}</strong>
                {/if}
                {#if dec.commit_id}
                  {#if normalizedCommitUrl(dec.commit_url)}
                    <a
                      href={normalizedCommitUrl(dec.commit_url)}
                      target="_blank"
                      rel="noopener noreferrer"
                      title={dec.commit_id}
                      on:click={(event) => openCommitUrl(event, dec.commit_url)}
                    >
                      {shortCommit(dec.commit_id)}
                    </a>
                  {:else}
                    <strong>{shortCommit(dec.commit_id)}</strong>
                  {/if}
                {/if}
              </div>
              <p>{dec.message}</p>
            </article>
          {/each}
        {/if}
      </div>
    </div>

    <div class="wa-admin-card log-panel manual-log-panel">
      <div class="log-head">
        <div>
          <strong>本次会议操作</strong>
          <small>人工调停动作</small>
        </div>
        <span>{localDecisions.length} 条</span>
      </div>
      <div class="log-list">
        {#if localDecisions.length === 0}
          <div class="log-empty">等待人工调停动作触发。</div>
        {:else}
          {#each localDecisions as feed}
            <article class="log-item local">
              <div class="log-meta">
                <span>INTERVENTION</span>
              </div>
              <p>{feed}</p>
            </article>
          {/each}
        {/if}
      </div>
    </div>
  </section>
</div>

<style>
  .decision-admin {
    --decision-radius-panel: 20px;
    --decision-radius-card: 16px;
    --decision-radius-control: 12px;
    position: relative;
    display: grid;
    width: 100%;
    min-width: 0;
    gap: var(--wa-space-5);
    padding-bottom: var(--wa-space-4);
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
    isolation: isolate;
  }

  .decision-admin::before {
    content: "";
    position: absolute;
    inset: -18px -16px auto;
    z-index: -1;
    height: 360px;
    pointer-events: none;
    background:
      linear-gradient(135deg, rgba(0, 143, 150, 0.1), transparent 42%),
      linear-gradient(180deg, rgba(255, 255, 255, 0.58), rgba(255, 255, 255, 0));
    mask-image: linear-gradient(180deg, #000 0%, rgba(0, 0, 0, 0.78) 45%, transparent 100%);
    -webkit-mask-image: linear-gradient(180deg, #000 0%, rgba(0, 0, 0, 0.78) 45%, transparent 100%);
  }

  .decision-command-strip {
    position: relative;
    min-width: 0;
    display: flex;
    align-items: stretch;
    justify-content: space-between;
    gap: var(--wa-space-6);
    padding: 20px 22px;
    border-color: rgba(255, 255, 255, 0.74);
    border-radius: var(--decision-radius-panel);
    background:
      linear-gradient(135deg, rgba(255, 255, 255, 0.9), rgba(242, 248, 251, 0.72)),
      var(--wa-chrome-1);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.92),
      0 18px 46px rgba(30, 46, 64, 0.1);
  }

  .decision-command-strip::before {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    border-radius: inherit;
    background:
      linear-gradient(90deg, rgba(0, 143, 150, 0.08), transparent 36%),
      repeating-linear-gradient(90deg, rgba(13, 23, 34, 0.035) 0 1px, transparent 1px 52px);
    opacity: 0.82;
  }

  .command-copy,
  .command-signal-grid {
    position: relative;
    z-index: 1;
  }

  .command-copy {
    min-width: 0;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 13px 16px;
    border: 1px solid rgba(255, 255, 255, 0.72);
    border-radius: var(--decision-radius-card);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.74), rgba(246, 251, 253, 0.5)),
      rgba(255, 255, 255, 0.54);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 10px 24px rgba(30, 46, 64, 0.055);
  }

  .command-kicker,
  .command-subline,
  .command-signal span {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 740;
    letter-spacing: 0;
  }

  .command-copy h2 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 25px;
    line-height: 1.12;
    font-weight: 850;
    letter-spacing: 0;
  }

  .command-subline {
    color: var(--wa-text-main);
    font-weight: 680;
  }

  .command-signal-grid {
    width: min(520px, 48%);
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .command-signal {
    min-width: 0;
    padding: 2px 0 2px var(--wa-space-4);
    display: grid;
    gap: var(--wa-space-1);
    border-left: 1px solid rgba(123, 143, 160, 0.18);
  }

  .command-signal strong {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 26px;
    line-height: 1;
    font-weight: 780;
    font-variant-numeric: tabular-nums;
  }

  .command-signal.tone-danger {
    border-color: rgba(221, 75, 62, 0.22);
  }

  .command-signal.tone-info {
    border-color: rgba(37, 107, 216, 0.2);
  }

  .decision-metrics {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-4);
  }

  .decision-metric {
    position: relative;
    overflow: hidden;
    min-height: 124px;
    padding: 18px;
    border-color: rgba(255, 255, 255, 0.66);
    border-radius: var(--decision-radius-card);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(247, 251, 253, 0.72)),
      var(--wa-chrome-1);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.9),
      0 16px 38px rgba(30, 46, 64, 0.075);
  }

  .decision-metric::before {
    content: "";
    position: absolute;
    inset: 14px auto 14px 0;
    width: 3px;
    border-radius: 999px;
    background: var(--wa-border-soft);
  }

  .decision-metric.metric-info::before {
    background: var(--wa-info);
  }

  .decision-metric.metric-success::before {
    background: var(--wa-success);
  }

  .decision-metric.metric-warning::before {
    background: var(--wa-warning);
  }

  .decision-metric.metric-danger::before {
    background: var(--wa-danger);
  }

  .metric-topline {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
  }

  .metric-topline span {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.25;
    font-weight: 780;
  }

  .metric-topline i {
    position: relative;
    width: 36px;
    height: 36px;
    border-radius: var(--decision-radius-control);
    background: var(--wa-neutral-soft);
    border: 1px solid var(--wa-border-soft);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78);
  }

  .metric-topline i::before,
  .metric-topline i::after {
    content: "";
    position: absolute;
    border-radius: 999px;
    background: currentColor;
    opacity: 0.72;
  }

  .metric-topline i::before {
    width: 12px;
    height: 12px;
    left: 10px;
    top: 9px;
  }

  .metric-topline i::after {
    width: 16px;
    height: 4px;
    left: 9px;
    bottom: 8px;
    opacity: 0.34;
  }

  .metric-info .metric-topline i {
    background: var(--wa-info-soft);
    border-color: rgba(37, 107, 216, 0.2);
    color: var(--wa-info);
  }

  .metric-success .metric-topline i {
    background: var(--wa-success-soft);
    border-color: rgba(4, 150, 111, 0.2);
    color: var(--wa-success);
  }

  .metric-warning .metric-topline i {
    background: var(--wa-warning-soft);
    border-color: rgba(216, 135, 0, 0.2);
    color: var(--wa-warning);
  }

  .metric-danger .metric-topline i {
    background: var(--wa-danger-soft);
    border-color: rgba(221, 75, 62, 0.22);
    color: var(--wa-danger);
  }

  .decision-metric strong {
    align-self: end;
    margin-top: 8px;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: clamp(30px, 2.6vw, 38px);
    line-height: 0.95;
    font-weight: 780;
    letter-spacing: 0;
    font-variant-numeric: tabular-nums;
  }

  .decision-metric small {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.35;
    font-weight: 620;
  }

  .decision-stage-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .stage-chip {
    position: relative;
    min-width: 0;
    min-height: 70px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
    padding: var(--wa-space-3) var(--wa-space-4) var(--wa-space-3) 18px;
    border: 1px solid var(--wa-border-soft);
    border-left: 4px solid rgba(123, 143, 160, 0.18);
    border-radius: var(--decision-radius-card);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.8), rgba(246, 251, 253, 0.64)),
      rgba(255, 255, 255, 0.68);
    color: var(--wa-text-main);
    font: inherit;
    cursor: pointer;
    text-align: left;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.76);
    backdrop-filter: blur(14px) saturate(122%);
    -webkit-backdrop-filter: blur(14px) saturate(122%);
    transition:
      transform var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease),
      box-shadow var(--wa-duration-fast) var(--wa-ease);
  }

  .stage-chip:hover,
  .stage-chip.active {
    transform: translateY(-1px);
    background: rgba(255, 255, 255, 0.92);
    border-color: var(--wa-accent);
    box-shadow: var(--wa-shadow-glow);
  }

  .stage-chip span {
    color: var(--wa-text-muted);
    font-size: 13px;
    font-weight: 680;
  }

  .stage-chip strong {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 25px;
    font-weight: 760;
    font-variant-numeric: tabular-nums;
  }

  .decision-main-grid {
    --decision-panel-height: clamp(640px, calc(100dvh - 112px), 820px);
    position: relative;
    display: grid;
    grid-template-columns: minmax(540px, 1.16fr) minmax(500px, 0.84fr);
    grid-template-areas: "inspector selector";
    gap: var(--wa-space-4);
    align-items: start;
    min-height: 0;
    overflow: visible;
  }

  .decision-table-section {
    grid-area: selector;
    position: relative;
    z-index: 20;
    min-width: 0;
    height: var(--decision-panel-height);
    max-height: var(--decision-panel-height);
    min-height: var(--decision-panel-height);
    box-sizing: border-box;
    padding: 18px;
    overflow: visible;
    border: 1px solid rgba(255, 255, 255, 0.66);
    border-radius: var(--decision-radius-panel);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.74), rgba(245, 250, 252, 0.56)),
      var(--wa-chrome-2);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 16px 42px rgba(30, 46, 64, 0.08);
    backdrop-filter: blur(18px) saturate(126%);
    -webkit-backdrop-filter: blur(18px) saturate(126%);
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr);
    gap: var(--wa-space-4);
  }

  .decision-table-section:focus-within {
    z-index: 80;
  }

  .decision-toolbar {
    position: relative;
    z-index: 45;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
    gap: 10px;
    overflow: visible;
    padding: 0;
    border: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .toolbar-copy {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: baseline;
    gap: 4px 10px;
    min-width: 0;
  }

  .toolbar-copy span {
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 760;
  }

  .toolbar-copy strong {
    color: var(--wa-text-strong);
    font-size: 17px;
    line-height: 1.25;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .toolbar-copy small {
    justify-self: end;
    color: var(--wa-text-muted);
    font-size: 12px;
    white-space: nowrap;
  }

  .toolbar-controls {
    position: relative;
    z-index: 2;
    width: 100%;
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(140px, 1.25fr) minmax(112px, 0.82fr) minmax(136px, 0.96fr) auto;
    align-items: center;
    justify-content: stretch;
    gap: var(--wa-space-2);
  }

  .search-control {
    width: 100%;
    min-width: 0;
    padding: 0 12px 0 34px;
    border-radius: var(--decision-radius-control);
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 24 24' xmlns='http://www.w3.org/2000/svg'%3E%3Ccircle cx='11' cy='11' r='6.5' fill='none' stroke='%23667789' stroke-width='2'/%3E%3Cpath d='m16 16 4 4' fill='none' stroke='%23667789' stroke-width='2' stroke-linecap='round'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: 12px 50%;
    background-size: 15px 15px;
  }

  .toolbar-select {
    position: relative;
    z-index: 2;
    width: 100%;
    min-width: 0;
    overflow: visible;
  }

  .toolbar-select.project-select {
    width: 100%;
  }

  .toolbar-controls .wa-admin-action {
    width: auto;
    min-width: 64px;
    padding-inline: 12px;
    white-space: nowrap;
  }

  .decision-table-shell {
    position: relative;
    z-index: 1;
    grid-row: 3;
    height: auto;
    max-height: none;
    min-height: 0;
    overflow: auto;
    border-color: rgba(123, 143, 160, 0.18);
    border-radius: var(--decision-radius-card);
    background: rgba(255, 255, 255, 0.64);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.82),
      0 10px 26px rgba(30, 46, 64, 0.05);
    backdrop-filter: blur(14px) saturate(118%);
    -webkit-backdrop-filter: blur(14px) saturate(118%);
    scrollbar-gutter: stable;
  }

  .decision-table-shell .wa-admin-table {
    min-width: 760px;
  }

  .decision-table-shell .wa-admin-table th {
    padding: 10px 12px;
    color: var(--wa-text-muted);
    font-size: 12px;
    letter-spacing: 0;
    background:
      linear-gradient(180deg, rgba(250, 253, 255, 0.98), rgba(244, 249, 252, 0.94));
  }

  .decision-table-shell .wa-admin-table td {
    height: 48px;
    padding: 10px 12px;
    color: var(--wa-text-main);
  }

  .decision-table-shell .sticky-status-col {
    position: sticky;
    right: 0;
    z-index: 3;
    background:
      linear-gradient(90deg, rgba(255, 255, 255, 0.78), rgba(255, 255, 255, 0.98) 30%),
      rgba(255, 255, 255, 0.96);
    box-shadow: -12px 0 18px rgba(26, 41, 58, 0.08);
  }

  .decision-table-shell thead .sticky-status-col {
    z-index: 5;
    background:
      linear-gradient(90deg, rgba(248, 251, 254, 0.82), rgba(248, 251, 254, 0.98) 32%),
      rgba(248, 251, 254, 0.98);
  }

  .decision-table-shell tbody tr.is-selected .sticky-status-col {
    background:
      linear-gradient(90deg, rgba(242, 250, 250, 0.82), rgba(242, 250, 250, 0.98) 32%),
      rgba(242, 250, 250, 0.98);
  }

  .decision-table-shell .wa-admin-table tbody tr:hover {
    background: rgba(242, 248, 251, 0.9);
  }

  .decision-table-shell .wa-admin-table tbody tr.is-selected {
    background: rgba(0, 143, 150, 0.06);
    box-shadow:
      inset 3px 0 0 var(--wa-accent),
      inset 0 0 0 1px rgba(0, 143, 150, 0.14);
  }

  .select-col {
    width: 36px;
    text-align: center;
  }

  .row-check {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xs);
    background: #ffffff;
    vertical-align: middle;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
  }

  .row-check.checked {
    border-color: var(--wa-accent);
    background:
      linear-gradient(135deg, transparent 0 42%, var(--wa-accent-ink) 42% 56%, transparent 56%),
      var(--wa-accent);
  }

  .table-link,
  .log-meta a {
    color: var(--wa-info);
    font-family: var(--wa-font-mono);
    font-size: 12px;
    font-weight: 760;
    text-decoration: none;
  }

  .table-link:hover,
  .log-meta a:hover {
    text-decoration: underline;
  }

  .table-id {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 12px;
    font-weight: 760;
  }

  .title-stack {
    display: grid;
    gap: 2px;
    min-width: 0;
  }

  .title-stack strong {
    color: var(--wa-text-strong);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    line-height: 1.28;
    font-weight: 780;
  }

  .table-state {
    padding: var(--wa-space-8);
    color: var(--wa-text-muted);
    text-align: center;
  }

  .decision-inspector {
    --wa-control-h: 34px;
    grid-area: inspector;
    align-self: start;
    z-index: 30;
    position: sticky;
    top: var(--wa-space-4);
    height: var(--decision-panel-height);
    max-height: var(--decision-panel-height);
    min-height: var(--decision-panel-height);
    box-sizing: border-box;
    gap: 12px;
    padding: 16px;
    overflow: visible;
    border-color: rgba(255, 255, 255, 0.68);
    border-radius: var(--decision-radius-panel);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(247, 251, 253, 0.66)),
      var(--wa-chrome-1);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 18px 46px rgba(30, 46, 64, 0.1);
  }

  .inspector-content {
    display: grid;
    align-content: start;
    gap: 12px;
    min-width: 0;
  }

  .decision-inspector:focus-within {
    z-index: 90;
  }

  .inspector-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
  }

  .inspector-head > div {
    min-width: 0;
  }

  .inspector-head .wa-admin-pill {
    flex: none;
  }

  .inspector-kicker {
    display: block;
    margin-bottom: 2px;
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 780;
  }

  .inspector-head h2 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 17px;
    line-height: 1.28;
    font-weight: 820;
    letter-spacing: 0;
  }

  .inspector-id-row,
  .section-row-head,
  .intervention-actions,
  .log-head,
  .log-meta {
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
  }

  .inspector-id-row,
  .section-row-head,
  .log-head {
    justify-content: space-between;
  }

  .inspector-id-row strong {
    color: var(--wa-text-strong);
    font-variant-numeric: tabular-nums;
  }

  .fact-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px 12px;
    margin: 0;
  }

  .fact-grid div {
    min-width: 0;
    padding: 0 0 7px;
    border-bottom: 1px solid rgba(123, 143, 160, 0.14);
  }

  .fact-grid dt {
    color: var(--wa-text-muted);
    font-size: 11.5px;
    line-height: 1.25;
    font-weight: 740;
  }

  .fact-grid dd {
    margin: 2px 0 0;
    color: var(--wa-text-strong);
    font-size: 12.5px;
    line-height: 1.3;
    font-weight: 760;
    overflow-wrap: anywhere;
  }

  .inspector-section-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px 16px;
    align-items: stretch;
    min-width: 0;
    padding-top: 10px;
    border-top: 1px solid rgba(121, 139, 159, 0.16);
  }

  .inspector-section {
    display: grid;
    gap: 6px;
    align-content: start;
    min-width: 0;
    min-height: 94px;
    padding: 0 0 10px;
    border-bottom: 1px solid rgba(121, 139, 159, 0.12);
  }

  .inspector-section h3 {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 800;
  }

  .inspector-section p,
  .inspector-section li {
    color: var(--wa-text-main);
    font-size: 12.5px;
    line-height: 1.45;
  }

  .inspector-section p,
  .inspector-section ul {
    margin: 0;
  }

  .inspector-section p {
    overflow: visible;
  }

  .inspector-section ul {
    display: grid;
    gap: 4px;
    overflow: visible;
    padding-left: 16px;
  }

  .intervention-form {
    position: relative;
    z-index: 2;
    display: grid;
    grid-template-columns: minmax(104px, 1fr) minmax(108px, 0.92fr) minmax(132px, 1.18fr) minmax(92px, 0.78fr);
    align-items: end;
    gap: 8px;
  }

  .intervention-form label {
    position: relative;
    display: grid;
    gap: 5px;
    min-width: 0;
    overflow: visible;
  }

  .intervention-form .form-field {
    position: relative;
    min-width: 0;
    overflow: visible;
  }

  .intervention-form label span {
    color: var(--wa-text-muted);
    font-size: 11.5px;
    font-weight: 720;
  }

  .intervention-form .wa-control {
    width: 100%;
    box-sizing: border-box;
    padding: 0 10px;
    font: inherit;
    font-size: 12.5px;
  }

  .wide-field {
    grid-column: auto;
  }

  .intervention-section {
    position: relative;
    z-index: 2;
    min-height: 128px;
    padding: 12px 0 0;
    border-bottom: 0;
    background: transparent;
    overflow: visible;
  }

  .intervention-actions {
    justify-content: flex-start;
    flex-wrap: nowrap;
    gap: 8px;
  }

  .section-row-head .wa-admin-action {
    min-height: 30px;
    padding-inline: 10px;
    font-size: 12px;
  }

  .intervention-actions .wa-admin-action {
    min-height: 34px;
    padding-inline: 14px;
  }

  .intervention-section :global(.alert) {
    padding: 8px 10px;
  }

  .inspector-empty {
    min-height: 320px;
    display: grid;
    place-content: center;
    gap: var(--wa-space-2);
    text-align: center;
    color: var(--wa-text-muted);
  }

  .inspector-empty strong {
    color: var(--wa-text-strong);
  }

  .decision-log-grid {
    display: grid;
    grid-template-columns: minmax(0, 1.34fr) minmax(300px, 0.66fr);
    gap: var(--wa-space-4);
  }

  .log-panel {
    min-width: 0;
    padding: 18px;
    display: grid;
    gap: var(--wa-space-3);
    border-color: rgba(255, 255, 255, 0.66);
    border-radius: var(--decision-radius-panel);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(247, 251, 253, 0.62)),
      var(--wa-chrome-1);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 14px 34px rgba(30, 46, 64, 0.075);
  }

  .auto-log-panel {
    min-height: 320px;
  }

  .manual-log-panel {
    min-height: 320px;
  }

  .log-head > div {
    display: grid;
    gap: 3px;
    min-width: 0;
  }

  .log-head strong {
    color: var(--wa-text-strong);
    font-size: 15px;
  }

  .log-head small {
    overflow: hidden;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .log-head span {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .log-list {
    display: grid;
    gap: 8px;
    max-height: 316px;
    overflow: auto;
    padding-right: 2px;
  }

  .log-item {
    position: relative;
    display: grid;
    gap: 7px;
    padding: 10px 12px 10px 18px;
    border: 1px solid rgba(121, 139, 159, 0.15);
    border-radius: var(--decision-radius-card);
    background: rgba(255, 255, 255, 0.5);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
  }

  .log-item::before {
    content: "";
    position: absolute;
    left: 0;
    top: 10px;
    bottom: 10px;
    width: 3px;
    border-radius: 999px;
    background: var(--wa-info);
    opacity: 0.72;
  }

  .log-item.local {
    border-color: rgba(0, 143, 150, 0.18);
    background: rgba(0, 143, 150, 0.055);
  }

  .log-item.local::before {
    background: var(--wa-accent);
  }

  .log-meta {
    justify-content: flex-start;
    flex-wrap: wrap;
    gap: 6px;
    color: var(--wa-text-muted);
    font-size: 12px;
    font-variant-numeric: tabular-nums;
  }

  .log-meta span,
  .log-meta strong,
  .log-meta a {
    display: inline-flex;
    align-items: center;
    min-height: 22px;
    padding: 0 7px;
    border-radius: var(--wa-radius-sm);
    background: rgba(121, 139, 159, 0.09);
  }

  .log-meta strong {
    color: var(--wa-text-main);
  }

  .log-item p,
  .log-empty {
    margin: 0;
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.5;
  }

  .log-empty {
    padding: var(--wa-space-5);
    color: var(--wa-text-muted);
    text-align: center;
  }

  @media (max-width: 1280px) {
    .decision-main-grid {
      --decision-panel-height: auto !important;
      grid-template-areas:
        "inspector"
        "selector";
      grid-template-columns: 1fr;
      min-height: 0;
    }

    .decision-inspector {
      position: static;
      height: auto;
      max-height: none;
      overflow: visible;
    }

    .inspector-section p {
      display: block;
      max-height: none;
      overflow: visible;
      -webkit-line-clamp: unset;
    }

    .inspector-section ul {
      max-height: none;
      overflow: visible;
    }

    .decision-table-section {
      height: auto;
      max-height: none;
      min-height: 560px;
    }
  }

  @media (max-width: 900px) {
    .decision-command-strip {
      flex-direction: column;
      gap: var(--wa-space-4);
    }

    .command-signal-grid {
      width: 100%;
    }

    .decision-metrics,
    .decision-stage-strip,
    .decision-log-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .toolbar-controls {
      grid-template-columns: minmax(160px, 1.35fr) minmax(120px, 0.8fr) minmax(140px, 0.95fr) auto;
    }

    .inspector-section-grid {
      grid-template-columns: 1fr;
    }

    .intervention-form {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .wide-field {
      grid-column: 1 / -1;
    }

    .search-control,
    .toolbar-select {
      width: 100%;
    }
  }

  @media (max-width: 640px) {
    .decision-command-strip,
    .decision-table-section,
    .decision-inspector,
    .log-panel {
      padding: var(--wa-space-3);
    }

    .command-signal-grid,
    .decision-metrics,
    .decision-stage-strip,
    .decision-log-grid,
    .fact-grid,
    .intervention-form {
      grid-template-columns: 1fr;
    }

    .decision-toolbar,
    .intervention-actions {
      align-items: stretch;
      flex-direction: column;
    }

    .toolbar-copy {
      grid-template-columns: 1fr;
    }

    .toolbar-copy small {
      justify-self: start;
    }

    .toolbar-controls {
      grid-template-columns: 1fr;
      align-items: stretch;
    }

    .search-control,
    .toolbar-select,
    .toolbar-controls .wa-admin-action {
      width: 100%;
    }
  }

  /* Strongest-brain page contract: inherit shell surface and spacing. */
  .decision-admin {
    min-height: var(--wa-workspace-min-h, calc(100dvh - 112px));
    gap: var(--wa-page-gap, 16px);
    padding-bottom: 0;
    background: transparent;
  }

  .decision-admin::before {
    display: none;
  }

  .decision-command-strip,
  .decision-table-section,
  .decision-inspector,
  .log-panel,
  .stage-chip,
  .metric-card,
  .command-copy,
  .command-signal {
    border-color: rgba(255, 255, 255, 0.7);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(247, 251, 253, 0.68)),
      var(--wa-chrome-1, rgba(255, 255, 255, 0.86));
  }

  .decision-main-grid,
  .decision-log-grid,
  .decision-metrics,
  .decision-stage-strip {
    gap: var(--wa-page-gap, 16px);
  }
</style>
