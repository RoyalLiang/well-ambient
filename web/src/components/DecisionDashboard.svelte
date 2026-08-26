<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import type {
    AdminCellValue,
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
  import AdminDataList from './admin-console/AdminDataList.svelte';
  import AdminListFilterBar from './admin-console/AdminListFilterBar.svelte';
  import DatePicker from './shared/DatePicker.svelte';
  import IssueTypeMark from './shared/IssueTypeMark.svelte';
  import Modal from './shared/Modal.svelte';
  import MultiSelect from './shared/MultiSelect.svelte';
  import OverlayCloseButton from './shared/OverlayCloseButton.svelte';
  import Select from './shared/Select.svelte';
  import { showToast } from '../lib/toast';
  import { refreshAgendaSummary, subscribeAgendaSummary } from '../lib/agenda-summary';
  import { subscribeTelemetryUpdates } from '../lib/telemetry-refresh';
  import {
    fetchDeliveryDirectory,
    type DeliveryAssigneeOption,
    type DeliveryProjectOption
  } from '../lib/delivery-directory';

  export let currentUser = '';
  export let timelineDrawerRequest = 0;

  const dispatch = createEventDispatcher<{
    timelineSummary: {
      kind: 'automatic' | 'manual';
      kindLabel: string;
      taskId: string;
      title: string;
      message: string;
      dateLabel: string;
      timeLabel: string;
      dateTime: string;
    } | null;
  }>();

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
    occurred_at?: string;
    task_id: string;
    message: string;
    assignee?: string;
    commit_id?: string;
    commit_url?: string;
  }

  interface LocalDecision {
    createdAt: string;
    taskId: string;
    actionLabel: string;
    operator: string;
    message: string;
  }

  interface DecisionTimelineEvent {
    id: string;
    kind: 'automatic' | 'manual';
    dateKey: string;
    dateLabel: string;
    timeLabel: string;
    dateTime: string;
    sortValue: number;
    taskId: string;
    title: string;
    message: string;
    actor: string;
    commitId?: string;
    commitUrl?: string;
  }

  interface DecisionTimelineGroup {
    dateKey: string;
    dateLabel: string;
    events: DecisionTimelineEvent[];
  }

  interface AgendaAdminRow extends AdminTableRow {
    source: AgendaItem;
  }

  interface StrongestBrainReleaseFact {
    id: number;
    project_key: string;
    name: string;
    status: string;
    release_date: string;
    work_item_count: number;
    completed_count: number;
    open_count: number;
    evidence_refs: string[];
  }

  const agendaDefaultColumnKeys = ['task_id', 'title', 'owner', 'risk', 'due', 'status'];
  const agendaColumns: AdminTableColumn[] = [
    { key: 'task_id', label: '事项', width: '132px' },
    { key: 'title', label: '标题', width: '34%' },
    { key: 'project', label: '项目', width: '140px' },
    { key: 'type', label: '类型', width: '88px' },
    { key: 'owner', label: '负责人', width: '112px' },
    { key: 'risk', label: '风险', width: '96px' },
    { key: 'risk_type', label: '风险原因', width: '128px' },
    { key: 'due', label: '计划日', width: '116px' },
    { key: 'repo', label: '仓库', width: '140px' },
    { key: 'branch', label: '分支', width: '152px' },
    { key: 'stale', label: '静默时长', width: '104px' },
    { key: 'activity', label: '最近活动', width: '116px' },
    { key: 'status', label: '状态', width: '96px' }
  ];
  const agendaColumnMinWidths: Record<string, number> = {
    task_id: 132,
    title: 260,
    project: 140,
    type: 88,
    owner: 112,
    risk: 96,
    risk_type: 128,
    due: 116,
    repo: 140,
    branch: 152,
    stale: 104,
    activity: 116,
    status: 96
  };
  const agendaVirtualOptions = { rowHeight: 48, overscan: 8, loadAheadRows: 8 };
  const configurableAgendaColumnOptions = agendaColumns
    .filter((column) => column.key !== 'task_id')
    .map((column) => ({ value: column.key, label: column.label }));

  let agendaItems: AgendaItem[] = [];
  let autoDecisions: AutoDecision[] = [];
  let loading = true;
  let configReady = false;
  let errorMsg = '';
  let releaseFacts: StrongestBrainReleaseFact[] = [];
  let releasedVersionCount = 0;
  let releaseFactsLoading = true;
  let releaseFactsError = '';

  let selectedItem: AgendaItem | null = null;
  let decisionLoading = false;

  let newAssignee = '';
  let newDueDate = '';
  let decisionNote = '';
  let operator = '';
  let localDecisions: LocalDecision[] = [];
  let adminMetrics: AdminMetric[] = [];
  let visibleAgendaColumnKeys = [...agendaDefaultColumnKeys];
  let columnPreferenceSaving = false;
  let columnPreferenceError = '';
  let columnPreferenceTimer: ReturnType<typeof setTimeout> | null = null;

  let currentFilter: 'all' | 'task' | 'bug' = 'all';
  let selectedAssignee = 'all';
  let selectedRepo = 'all';
  let showRiskLevel: 'all' | 'risks' = 'risks';
  let searchText = '';
  let detailDrawerOpen = false;
  let detailDrawerReturnEl: HTMLElement | null = null;
  let timelineDrawerOpen = false;
  let handledTimelineDrawerRequest = timelineDrawerRequest;
  let publishedTimelineSummaryKey = '';
  let timelineDrawerEl: HTMLElement;
  let timelineDrawerCloseButton: OverlayCloseButton;

  $: latestReleaseFact = releaseFacts[0] || null;
  $: remainingReleasedCount = Math.max(0, releasedVersionCount - (latestReleaseFact ? 1 : 0));

  let coreMembers = new Set<string>();
  let deliveryAssignees: DeliveryAssigneeOption[] = [];
  let deliveryProjects: DeliveryProjectOption[] = [];
  let deliveryDirectoryReady = false;
  let coreMemberAliases = new Set<string>();

  let jiraBaseUrl = '';
  let gitlabBaseUrl = '';
  let projectNamesMap: Record<string, string> = {};

  $: if (currentUser && !operator) {
    operator = currentUser;
  }

  $: filteredAgendaItems = configReady ? agendaItems.filter((item) => isVisibleAgendaItem(item)) : [];
  $: filteredAutoDecisions = autoDecisions.filter((dec) => !dec.assignee || isCoreMember(dec.assignee));

  $: activeBugCount = filteredAgendaItems.filter((item) => isBugIssueType(item.issue_type)).length;
  $: activeTaskCount = filteredAgendaItems.filter((item) => !isBugIssueType(item.issue_type)).length;
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
    ...(deliveryDirectoryReady
      ? deliveryAssignees.map((option) => option.value)
      : Array.from(new Set(filteredAgendaItems.map((item) => item.assignee).filter(Boolean))).sort((a, b) =>
          a.localeCompare(b)
        ))
  ];
  $: projectList = [
    'all',
    ...(deliveryDirectoryReady
      ? deliveryProjects.map((project) => project.project_key)
      : Array.from(
          new Set(filteredAgendaItems.map((item) => getAgendaProjectKey(item)).filter((project): project is string => !!project))
        ).sort((a, b) => a.localeCompare(b)))
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
      if (currentFilter === 'task' && isBugIssueType(item.issue_type)) return false;
      if (currentFilter === 'bug' && !isBugIssueType(item.issue_type)) return false;
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

  $: {
    const _metricDependencies = [
      filteredAgendaItems.length,
      activeTaskCount,
      activeBugCount,
      redZoneCount,
      warningCount,
      filteredAutoDecisions.length
    ];
    adminMetrics = adaptMetrics();
  }
  $: agendaRows = filteredItems.map(adaptAgendaRow);
  $: visibleAgendaColumns = agendaColumns.filter((column) => visibleAgendaColumnKeys.includes(column.key));
  $: agendaTableMinWidth = `${Math.max(760, visibleAgendaColumns.reduce(
    (width, column) => width + (agendaColumnMinWidths[column.key] || 112),
    0
  ))}px`;
  $: agendaListResetKey = [
    currentFilter,
    selectedAssignee,
    selectedRepo,
    showRiskLevel,
    searchText.trim().toLowerCase(),
    visibleAgendaColumnKeys.join(',')
  ].join('|');
  $: configurableVisibleAgendaColumnKeys = visibleAgendaColumnKeys.filter((key) => key !== 'task_id');
  $: inspectorRecord = selectedItem ? adaptInspectorRecord(selectedItem) : null;
  $: currentDueDate = selectedItem?.due_date ? formatAdminDate(selectedItem.due_date) : '';
  $: hasDueDateChange = !!newDueDate && newDueDate !== currentDueDate;
  $: decisionTimelineGroups = buildDecisionTimelineGroups(filteredAutoDecisions, localDecisions);
  $: decisionTimelineEvents = decisionTimelineGroups.flatMap((group) => group.events);
  $: latestDecisionTimelineEvent = decisionTimelineEvents[0] || null;
  $: latestTimelineSummaryKey = latestDecisionTimelineEvent
    ? `${latestDecisionTimelineEvent.id}:${latestDecisionTimelineEvent.sortValue}`
    : 'empty';
  $: if (latestTimelineSummaryKey !== publishedTimelineSummaryKey) {
    publishedTimelineSummaryKey = latestTimelineSummaryKey;
    dispatch('timelineSummary', latestDecisionTimelineEvent
      ? {
          kind: latestDecisionTimelineEvent.kind,
          kindLabel: latestDecisionTimelineEvent.kind === 'automatic' ? '自动流转' : '人工调停',
          taskId: latestDecisionTimelineEvent.taskId,
          title: latestDecisionTimelineEvent.title,
          message: latestDecisionTimelineEvent.message,
          dateLabel: latestDecisionTimelineEvent.dateLabel,
          timeLabel: latestDecisionTimelineEvent.timeLabel,
          dateTime: latestDecisionTimelineEvent.dateTime
        }
      : null);
  }
  $: if (timelineDrawerRequest > handledTimelineDrawerRequest) {
    handledTimelineDrawerRequest = timelineDrawerRequest;
    openTimelineDrawer();
  }

  function handleDueDateChange(event: CustomEvent<string>) {
    newDueDate = event.detail;
  }

  function formatPendingDueDate(value: string): string {
    const [year, month, day] = value.split('-');
    if (!year || !month || !day) return value;
    return `${month}/${day}`;
  }

  function parseTimelineDate(rawValue: string): Date {
    const raw = (rawValue || '').trim();
    const parsed = new Date(raw);
    if (raw && !Number.isNaN(parsed.getTime()) && /\d{4}[-/]\d{1,2}[-/]\d{1,2}|T/.test(raw)) {
      return parsed;
    }

    const fallback = new Date();
    const timeMatch = raw.match(/(\d{1,2}):(\d{2})(?::(\d{2}))?/);
    if (timeMatch) {
      fallback.setHours(Number(timeMatch[1]), Number(timeMatch[2]), Number(timeMatch[3] || 0), 0);
      if (fallback.getTime() > Date.now()) {
        fallback.setDate(fallback.getDate() - 1);
      }
    }
    return fallback;
  }

  function timelineDateKey(date: Date): string {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  function timelineDateLabel(date: Date): string {
    const today = new Date();
    const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
    const key = timelineDateKey(date);
    if (key === timelineDateKey(today)) return '今天';
    if (key === timelineDateKey(yesterday)) return '昨天';
    return date.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' });
  }

  function timelineTimeLabel(date: Date): string {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  }

  function buildDecisionTimelineGroups(
    automaticEvents: AutoDecision[],
    manualEvents: LocalDecision[]
  ): DecisionTimelineGroup[] {
    const events: DecisionTimelineEvent[] = automaticEvents.map((event, index) => {
      const date = parseTimelineDate(event.occurred_at || event.time);
      return {
        id: `automatic-${event.task_id}-${event.time}-${index}`,
        kind: 'automatic',
        dateKey: timelineDateKey(date),
        dateLabel: timelineDateLabel(date),
        timeLabel: timelineTimeLabel(date),
        dateTime: date.toISOString(),
        sortValue: date.getTime(),
        taskId: event.task_id,
        title: '自动流转',
        message: event.message,
        actor: event.assignee || '系统',
        commitId: event.commit_id,
        commitUrl: event.commit_url
      };
    });

    manualEvents.forEach((event, index) => {
      const date = parseTimelineDate(event.createdAt);
      events.push({
        id: `manual-${event.taskId}-${event.createdAt}-${index}`,
        kind: 'manual',
        dateKey: timelineDateKey(date),
        dateLabel: timelineDateLabel(date),
        timeLabel: timelineTimeLabel(date),
        dateTime: date.toISOString(),
        sortValue: date.getTime(),
        taskId: event.taskId,
        title: event.actionLabel,
        message: event.message,
        actor: event.operator || '人工调停'
      });
    });

    events.sort((a, b) => b.sortValue - a.sortValue);
    const grouped = new Map<string, DecisionTimelineGroup>();
    events.forEach((event) => {
      const group = grouped.get(event.dateKey) || {
        dateKey: event.dateKey,
        dateLabel: event.dateLabel,
        events: []
      };
      group.events.push(event);
      grouped.set(event.dateKey, group);
    });
    return Array.from(grouped.values());
  }

  function isCoreMember(name: string): boolean {
    if (!name) return false;
    if (!deliveryDirectoryReady) return true;
    const normalized = name.trim().toLocaleLowerCase('zh-CN');
    return coreMemberAliases.has(normalized) || coreMemberAliases.has(normalized.split(' ')[0]);
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

  async function fetchConfig() {
    try {
      const directory = await fetchDeliveryDirectory();
      deliveryAssignees = directory.assignees;
      deliveryProjects = directory.projects;
      coreMembers = new Set(directory.assignees.map((option) => option.value));
      coreMemberAliases = new Set(
        directory.assignees.flatMap((option) => [option.value, ...(option.aliases || [])])
          .map((value) => value.trim().toLocaleLowerCase('zh-CN'))
          .filter(Boolean)
      );
      projectNamesMap = Object.fromEntries(
        directory.projects.map((project) => [project.project_key, cleanProjectName(project.project_name) || project.project_key])
      );
      deliveryDirectoryReady = true;
    } catch (error) {
      console.error('Failed to fetch shared delivery directory on dashboard:', error);
    }

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
          if (!jiraBaseUrl) {
            jiraBaseUrl = data?.jira?.base_url ? data.jira.base_url.replace(/\/+$/, '') : '';
          }
          gitlabBaseUrl = data?.gitlab?.base_url ? data.gitlab.base_url.replace(/\/+$/, '') : '';
        }
      }

    } catch (e) {
      console.error('Failed to fetch config on dashboard:', e);
    } finally {
      configReady = true;
    }
  }

  function applyAgendaSummary(data: Record<string, unknown>) {
    const nextAgendaItems = Array.isArray(data.agenda_items) ? data.agenda_items as AgendaItem[] : [];
    const nextAutoDecisions = Array.isArray(data.auto_decisions) ? data.auto_decisions as AutoDecision[] : [];
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
  }

  async function fetchAgenda(force = true) {
    try {
      await refreshAgendaSummary({ force });
    } catch {
      // The shared resource publishes the error while keeping the last good snapshot visible.
    }
  }

  async function fetchReleaseFacts() {
    releaseFactsLoading = true;
    releaseFactsError = '';
    try {
      const response = await fetch('/api/strongest-brain/releases');
      if (!response.ok) throw new Error('加载版本发布事实失败');
      const summary = await response.json();
      releasedVersionCount = Number(summary.released) || 0;
      releaseFacts = Array.isArray(summary.recent) ? summary.recent : [];
    } catch (error: any) {
      releaseFactsError = error?.message || '版本发布事实暂不可用';
    } finally {
      releaseFactsLoading = false;
    }
  }

  async function fetchDecisionTableColumns() {
    try {
      const response = await fetch('/api/me/decision-table-columns');
      if (!response.ok) throw new Error('加载显示列配置失败');
      const data = await response.json();
      const configured = Array.isArray(data?.visible_columns) ? data.visible_columns : [];
      const known = new Set(agendaColumns.map((column) => column.key));
      const next = agendaColumns
        .map((column) => column.key)
        .filter((key) => configured.includes(key) && known.has(key));
      visibleAgendaColumnKeys = next.includes('task_id') ? next : [...agendaDefaultColumnKeys];
    } catch (error: any) {
      columnPreferenceError = error?.message || '显示列配置暂不可用';
    }
  }

  async function saveDecisionTableColumns(columns: string[]) {
    columnPreferenceSaving = true;
    columnPreferenceError = '';
    try {
      const response = await fetch('/api/me/decision-table-columns', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ visible_columns: columns })
      });
      if (!response.ok) throw new Error((await response.text()) || '保存显示列配置失败');
      const data = await response.json();
      if (Array.isArray(data?.visible_columns)) {
        visibleAgendaColumnKeys = data.visible_columns;
      }
    } catch (error: any) {
      columnPreferenceError = error?.message || '保存显示列配置失败';
    } finally {
      columnPreferenceSaving = false;
    }
  }

  function handleAgendaColumnsChange(event: CustomEvent<string[]>) {
    const selected = new Set(event.detail || []);
    visibleAgendaColumnKeys = agendaColumns
      .map((column) => column.key)
      .filter((key) => key === 'task_id' || selected.has(key));
    if (columnPreferenceTimer) clearTimeout(columnPreferenceTimer);
    const snapshot = [...visibleAgendaColumnKeys];
    columnPreferenceTimer = setTimeout(() => {
      columnPreferenceTimer = null;
      void saveDecisionTableColumns(snapshot);
    }, 220);
  }

  function selectItem(item: AgendaItem) {
    selectedItem = item;
    syncInterventionDraft(item);
  }

  async function openDetailDrawer(item: AgendaItem, event?: Event) {
    selectItem(item);
    const trigger = event?.currentTarget;
    detailDrawerReturnEl = trigger instanceof HTMLElement ? trigger : null;
    detailDrawerOpen = true;
  }

  async function closeDetailDrawer() {
    if (!detailDrawerOpen) return;
    detailDrawerOpen = false;
    await tick();
    if (detailDrawerReturnEl?.isConnected) {
      detailDrawerReturnEl.focus();
    }
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

    let value = '';
    if (action === 'reassign') {
      if (!newAssignee.trim()) {
        showToast('请输入转派负责人。', { type: 'error', title: '无法提交人工调停' });
        decisionLoading = false;
        return;
      }
      value = newAssignee;
    } else {
      if (!newDueDate) {
        showToast('请选择新的截止时间。', { type: 'error', title: '无法提交人工调停' });
        decisionLoading = false;
        return;
      }
      if (!hasDueDateChange) {
        showToast('新的截止日期必须与当前日期不同。', { type: 'error', title: '无法提交人工调停' });
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
          meeting_note: decisionNote.trim()
        })
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || '提交人工干预指令失败');
      }

      const result = await res.json();
      if (action === 'reschedule' && result?.due_date !== newDueDate) {
        throw new Error(`截止日期更新校验失败：期望 ${newDueDate}，实际 ${result?.due_date || '未返回'}`);
      }
      const successMessage = action === 'reschedule'
        ? `截止日期已调整为 ${newDueDate}，排期数据已同步。`
        : '人工干预已记录至事件账本并更新状态。';
      showToast(successMessage, {
        title: result?.meeting_note_synced
          ? '人工调停已保存，会议备注已提交 Jira 同步'
          : '人工调停已保存'
      });

      const actionName = action === 'reassign' ? '覆盖指派' : '调整截止期';
      const createdAt = new Date().toISOString();
      localDecisions = [{
        createdAt,
        taskId: selectedItem.task_id,
        actionLabel: actionName,
        operator: operator || '系统管理员',
        message: `${operator || '系统管理员'} 调停 ${selectedItem.task_id}：${actionName}${decisionNote ? ` · ${decisionNote}` : ''}`
      }, ...localDecisions];

      await fetchAgenda();
      window.dispatchEvent(new CustomEvent('decision-events-updated'));
      await closeDetailDrawer();
    } catch (err: any) {
      showToast(err.message || '提交干预请求时出错', {
        type: 'error',
        title: '人工调停提交失败'
      });
    } finally {
      decisionLoading = false;
    }
  }

  function adaptMetrics(): AdminMetric[] {
    return [
      {
        label: '可见事项',
        value: filteredAgendaItems.length,
        helper: `${filteredAgendaItems.length} 条活跃议程`,
        tone: 'neutral'
      },
      {
        label: '高风险',
        value: redZoneCount,
        helper: '需要立即决策',
        tone: redZoneCount > 0 ? 'danger' : 'success'
      },
      {
        label: '中风险',
        value: warningCount,
        helper: '本周持续观察',
        tone: warningCount > 0 ? 'warning' : 'success'
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
        project: getAgendaProjectLabel(item) || '-',
        type: getIssueTypeLabel(item.issue_type),
        priority: getIssueTypeLabel(item.issue_type),
        owner: item.assignee || '-',
        risk: getRiskLevelLabel(item.risk_level),
        risk_type: getRiskTypeLabel(item.risk_type),
        due: formatAdminDate(item.due_date),
        repo: item.repo || '-',
        branch: item.telemetry_snippet?.branch || '-',
        activity: formatAdminDate(item.telemetry_snippet?.last_update),
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

  function isBugIssueType(issueType: string) {
    return ['bug', 'defect', '缺陷', '故障'].includes((issueType || '').trim().toLowerCase());
  }

  function getIssueTypeLabel(issueType: string) {
    return isBugIssueType(issueType) ? '缺陷' : '任务';
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

  function portalToConsole(node: HTMLElement) {
    const target = document.querySelector<HTMLElement>('.functional-console') || document.body;
    target.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  async function openTimelineDrawer() {
    timelineDrawerOpen = true;
    await tick();
    timelineDrawerCloseButton?.focus();
  }

  async function closeTimelineDrawer() {
    if (!timelineDrawerOpen) return;
    timelineDrawerOpen = false;
    await tick();
    document.querySelector<HTMLButtonElement>('.workspace-latest-event')?.focus();
  }

  function trapDrawerFocus(event: KeyboardEvent, drawer: HTMLElement | undefined) {
    if (event.key !== 'Tab' || !drawer) return;
    const focusable = Array.from(drawer.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter((element) => !element.hasAttribute('hidden'));
    if (focusable.length === 0) return;
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

  function handleDrawerKeydown(event: KeyboardEvent) {
    if (detailDrawerOpen) return;
    if (!timelineDrawerOpen) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      closeTimelineDrawer();
      return;
    }
    trapDrawerFocus(event, timelineDrawerEl);
  }

  onMount(() => {
    let disposed = false;
    let interval: ReturnType<typeof setInterval> | null = null;
    const handleProjectPreferencesUpdated = () => {
      void fetchAgenda(true);
      void fetchReleaseFacts();
    };
    const handleReleasePublished = () => void fetchReleaseFacts();
    const unsubscribeAgenda = subscribeAgendaSummary((agendaState) => {
      loading = agendaState.loading && !agendaState.data;
      errorMsg = agendaState.error;
      if (agendaState.data) applyAgendaSummary(agendaState.data);
    });

    const bootstrapDashboard = async () => {
      await Promise.all([fetchConfig(), fetchDecisionTableColumns(), fetchReleaseFacts()]);
      if (disposed) return;
      await fetchAgenda(false);
      if (disposed) return;
      interval = setInterval(() => {
        void fetchReleaseFacts();
      }, 15000);
    };

    bootstrapDashboard();
    window.addEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
    window.addEventListener('well-ambient:release-published', handleReleasePublished);
    const unsubscribeTelemetryUpdates = subscribeTelemetryUpdates(
      window,
      () => fetchAgenda(true),
      { visibilityTarget: document }
    );

    return () => {
      disposed = true;
      if (interval) clearInterval(interval);
      if (columnPreferenceTimer) clearTimeout(columnPreferenceTimer);
      window.removeEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
      window.removeEventListener('well-ambient:release-published', handleReleasePublished);
      unsubscribeAgenda();
      unsubscribeTelemetryUpdates();
      dispatch('timelineSummary', null);
    };
  });
</script>

{#snippet renderAgendaCell(row: AgendaAdminRow, column: AdminTableColumn, value: AdminCellValue)}
  {#if column.key === 'task_id'}
    {@const jiraUrl = getJiraUrl(row.id)}
    <div class="issue-id-cell">
      <IssueTypeMark issueType={row.source.issue_type} />
      {#if jiraUrl}
        <a class="table-link" href={jiraUrl} target="_blank" rel="noopener noreferrer">
          {String(value ?? row.id)}
        </a>
      {:else}
        <span class="table-id">{String(value ?? row.id)}</span>
      {/if}
    </div>
  {:else if column.key === 'title'}
    <button
      type="button"
      class="decision-title-button"
      aria-haspopup="dialog"
      aria-label={`查看 ${row.id}：${row.title}`}
      on:click={(event) => openDetailDrawer(row.source, event)}
    >
      <strong>{row.title}</strong>
    </button>
  {:else if column.key === 'risk'}
    <span class="wa-admin-pill {toneClass(row.tone)}">{row.risk}</span>
  {:else if column.key === 'status'}
    <span class="wa-admin-pill {getStatusToneClass(row.status)}">{row.status}</span>
  {:else}
    <span class="decision-cell-value" title={String(value ?? '-')}>{String(value ?? '-')}</span>
  {/if}
{/snippet}

{#snippet renderAgendaEmpty()}
  <div class="decision-list-empty">
    <strong>当前条件下没有可见决策事项</strong>
    <span>可调整风险、类型、负责人、项目或搜索条件。</span>
  </div>
{/snippet}

<svelte:window on:keydown={handleDrawerKeydown} />

<div class="decision-admin">
  <section class="decision-summary-panel" aria-label="决策看板总览">
    <div class="decision-metrics" aria-label="决策看板指标">
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
    </div>

    <div class="decision-stage-strip" aria-label="状态分段">
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
    </div>
  </section>

  <div class="decision-main-grid">
    <section class="decision-table-section" aria-label="决策事项列表">
      <AdminListFilterBar label="决策事项筛选" className="decision-filter-bar">
        {#snippet leading()}
          <div class="toolbar-copy">
            <strong>事项选择列表</strong>
            {#if loading}
              <small>正在刷新</small>
            {:else}
              <small>{agendaRows.length} / {filteredAgendaItems.length} 条可见事项</small>
            {/if}
          </div>
        {/snippet}

        {#snippet controls()}
          <div class="toolbar-controls">
            <input class="wa-control search-control" type="search" bind:value={searchText} placeholder="搜索编号、标题、负责人" />
            <div class="toolbar-select">
              <Select
                bind:value={selectedAssignee}
                options={assigneeOptions}
                placeholder="全部负责人"
                searchPlaceholder="搜索负责人"
                compact={true}
                shadowless={true}
              />
            </div>
            <div class="toolbar-select project-select">
              <Select
                bind:value={selectedRepo}
                options={projectOptions}
                placeholder="全部项目"
                searchPlaceholder="搜索项目名"
                compact={true}
                shadowless={true}
              />
            </div>
            <div class="toolbar-select column-select">
              <MultiSelect
                values={configurableVisibleAgendaColumnKeys}
                options={configurableAgendaColumnOptions}
                placeholder="选择显示列"
                searchPlaceholder="搜索列"
                controlLabel="显示列"
                ariaLabel="配置事项列表显示列"
                summaryMode={true}
                overlay={true}
                shadowless={true}
                compact={true}
                on:change={handleAgendaColumnsChange}
              />
              <span class="column-preference-state" class:error={!!columnPreferenceError} aria-live="polite">
                {columnPreferenceSaving ? '正在保存显示列' : columnPreferenceError}
              </span>
            </div>
            <button class="wa-admin-action secondary" type="button" on:click={() => fetchAgenda(true)} disabled={loading}>
              刷新
            </button>
          </div>
        {/snippet}
      </AdminListFilterBar>

      <AdminDataList
        columns={visibleAgendaColumns}
        rows={agendaRows}
        caption="决策事项选择列表"
        cell={renderAgendaCell}
        empty={renderAgendaEmpty}
        {loading}
        error={errorMsg}
        onRetry={() => fetchAgenda(true)}
        skeletonRows={6}
        className="decision-agenda-list"
        scrollRegionId="decision-agenda-list-content"
        tableMinWidth={agendaTableMinWidth}
        compactTableMinWidth={agendaTableMinWidth}
        virtual={agendaVirtualOptions}
        totalRowCount={agendaRows.length}
        selectedRowId={selectedItem?.task_id || ''}
        resetKey={agendaListResetKey}
      />
    </section>
  </div>

  {#if inspectorRecord && selectedItem}
    <Modal
      show={detailDrawerOpen}
      title={inspectorRecord.title}
      size="wide"
      closeLabel="关闭需求详情"
      shadowless={true}
      on:close={closeDetailDrawer}
    >
      <div class="requirement-drawer-body requirement-modal-body">
        <div class="requirement-drawer-meta requirement-modal-meta">
          <span class="inspector-kicker">需求详情</span>
          {#if getJiraUrl(inspectorRecord.id)}
            <a class="table-link" href={getJiraUrl(inspectorRecord.id)} target="_blank" rel="noopener noreferrer">{inspectorRecord.id}</a>
          {:else}
            <strong>{inspectorRecord.id}</strong>
          {/if}
          <span class="wa-admin-pill {toneClass(inspectorRecord.tone)}">{inspectorRecord.status}</span>
          <span class="wa-admin-pill {toneClass(inspectorRecord.tone)}">{getRiskLevelLabel(selectedItem.risk_level)}</span>
        </div>

        <section class="requirement-drawer-section fact-section" aria-labelledby="requirement-facts-title">
          <div class="drawer-section-heading">
            <h3 id="requirement-facts-title">事项概览</h3>
            <span>Agenda 实时数据</span>
          </div>
          <dl class="fact-grid">
            {#each inspectorRecord.facts as fact}
              <div class:project-fact={fact.label === '所属项目'}>
                <dt>{fact.label}</dt>
                <dd>{fact.value}</dd>
              </div>
            {/each}
          </dl>
        </section>

        <div class="inspector-section-grid">
          {#each inspectorRecord.sections as section}
            <section
              class="inspector-section"
              class:empty-history-section={section.title === '人工干预记录' && section.items?.length === 1 && section.items[0] === '暂无人工干预记录'}
            >
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
            <div>
              <h3>人工调停</h3>
              <span>确认负责人或调整计划日后再提交</span>
            </div>
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
                shadowless={true}
              />
            </div>
            <div class="form-field">
              <DatePicker
                label="新的截止日"
                value={newDueDate}
                placeholder="选择新的截止日"
                compact={true}
                shadowless={true}
                on:change={handleDueDateChange}
              />
            </div>
            <label class="wide-field">
              <span>会议备注（选填）</span>
              <input class="wa-control" type="text" bind:value={decisionNote} placeholder="填写后将按原文同步为 Jira 评论；留空则不评论" />
            </label>
            <label>
              <span>调停决策人</span>
              <input class="wa-control" type="text" bind:value={operator} readonly />
            </label>
          </div>

        </section>
      </div>

      <div slot="footer" class="intervention-actions">
        <button class="wa-admin-action primary" type="button" on:click={() => submitDecision('reassign')} disabled={decisionLoading}>
          覆盖指派
        </button>
        <button class="wa-admin-action secondary" type="button" on:click={() => submitDecision('reschedule')} disabled={decisionLoading || !hasDueDateChange}>
          {hasDueDateChange ? `调整至 ${formatPendingDueDate(newDueDate)}` : '调整截止'}
        </button>
      </div>
    </Modal>
  {/if}

  {#if timelineDrawerOpen}
    <div use:portalToConsole class="timeline-drawer-layer">
      <button
        type="button"
        class="timeline-drawer-backdrop"
        aria-label="关闭事件记录"
        on:click={closeTimelineDrawer}
      ></button>
      <div
        class="timeline-drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="timeline-drawer-title"
        bind:this={timelineDrawerEl}
      >
        <header class="timeline-drawer-header">
          <div>
            <span class="timeline-drawer-eyebrow">决策证据</span>
            <h2 id="timeline-drawer-title">事件记录</h2>
            <p>全部可见事件按发生时间倒序展示，最新记录在前。</p>
          </div>
          <div class="timeline-drawer-actions">
            <strong class="timeline-drawer-count">{decisionTimelineEvents.length} 条</strong>
            <OverlayCloseButton
              label="关闭事件记录"
              bind:this={timelineDrawerCloseButton}
              on:click={closeTimelineDrawer}
            />
          </div>
        </header>

        <div class="timeline-drawer-body">
          {#if decisionTimelineGroups.length === 0}
            <div class="timeline-drawer-empty">
              <strong>暂无事件记录</strong>
              <span>自动流转或人工调停发生后会在这里按时间归档。</span>
            </div>
          {:else}
            {#each decisionTimelineGroups as group}
              <section class="timeline-day" aria-labelledby={`timeline-day-${group.dateKey}`}>
                <div class="timeline-day-heading">
                  <h3 id={`timeline-day-${group.dateKey}`}>{group.dateLabel}</h3>
                  <span>{group.events.length} 条</span>
                </div>
                <ol>
                  {#each group.events as event, index (event.id)}
                    <li class="timeline-drawer-event {event.kind}" aria-current={group === decisionTimelineGroups[0] && index === 0 ? 'true' : undefined}>
                      <div class="timeline-event-time">
                        <time datetime={event.dateTime}>{event.timeLabel}</time>
                        <span class="timeline-event-kind">{event.kind === 'automatic' ? '自动流转' : '人工调停'}</span>
                      </div>
                      <div class="timeline-event-content">
                        <div class="timeline-event-title">
                          <strong>{event.title}</strong>
                          {#if getJiraUrl(event.taskId)}
                            <a href={getJiraUrl(event.taskId)} target="_blank" rel="noopener noreferrer">{event.taskId}</a>
                          {:else}
                            <span>{event.taskId}</span>
                          {/if}
                        </div>
                        <p>{event.message}</p>
                        <div class="timeline-event-evidence">
                          <span>{event.actor}</span>
                          {#if event.commitId}
                            {#if normalizedCommitUrl(event.commitUrl)}
                              <a
                                href={normalizedCommitUrl(event.commitUrl)}
                                target="_blank"
                                rel="noopener noreferrer"
                                title={event.commitId}
                                on:click={(clickEvent) => openCommitUrl(clickEvent, event.commitUrl)}
                              >commit {shortCommit(event.commitId)}</a>
                            {:else}
                              <span>commit {shortCommit(event.commitId)}</span>
                            {/if}
                          {/if}
                        </div>
                      </div>
                    </li>
                  {/each}
                </ol>
              </section>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}

</div>

<style>
  .decision-admin {
    --decision-radius-panel: 20px;
    --decision-radius-card: 16px;
    --decision-radius-control: 12px;
    position: relative;
    display: grid;
    grid-template-rows: 68px 48px minmax(0, 1fr);
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
    gap: 12px;
    padding-bottom: 0;
    overflow: hidden;
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
    mask-image: linear-gradient(180deg, rgb(13, 23, 34) 0%, rgba(13, 23, 34, 0.78) 45%, transparent 100%);
    -webkit-mask-image: linear-gradient(180deg, rgb(13, 23, 34) 0%, rgba(13, 23, 34, 0.78) 45%, transparent 100%);
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
    gap: 12px;
    min-height: 0;
  }

  .decision-metric {
    position: relative;
    overflow: hidden;
    min-height: 0;
    height: 100%;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-rows: auto auto;
    align-items: center;
    column-gap: 14px;
    padding: 9px 14px 9px 17px;
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
    inset: 10px auto 10px 0;
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
    display: block;
    align-items: center;
    justify-content: space-between;
    min-width: 0;
  }

  .metric-topline span {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.25;
    font-weight: 780;
  }

  .metric-topline i {
    display: none;
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
    grid-column: 2;
    grid-row: 1 / 3;
    justify-self: end;
    align-self: center;
    margin: 0;
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 28px;
    line-height: 0.95;
    font-weight: 780;
    letter-spacing: 0;
    font-variant-numeric: tabular-nums;
  }

  .decision-metric small {
    min-width: 0;
    overflow: hidden;
    color: var(--wa-text-muted);
    font-size: 11px;
    line-height: 1.25;
    font-weight: 620;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .decision-stage-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
    min-height: 0;
  }

  .stage-chip {
    position: relative;
    min-width: 0;
    min-height: 0;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
    padding: 8px 14px;
    border: 1px solid var(--wa-border-soft);
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
    font-size: 20px;
    font-weight: 760;
    font-variant-numeric: tabular-nums;
  }

  .decision-main-grid {
    position: relative;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
    grid-template-areas: "selector";
    align-items: stretch;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .decision-table-section {
    grid-area: selector;
    position: relative;
    z-index: 20;
    min-width: 0;
    align-self: stretch;
    height: 100%;
    max-height: none;
    min-height: 0;
    box-sizing: border-box;
    overflow: visible;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    gap: 10px;
  }

  .decision-table-section:focus-within {
    z-index: 80;
  }

  .toolbar-copy {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
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
    grid-column: 2;
    justify-self: start;
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
    grid-template-columns: minmax(140px, 1.18fr) minmax(108px, 0.76fr) minmax(128px, 0.9fr) minmax(126px, 0.84fr) auto;
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

  .column-preference-state {
    position: absolute;
    top: calc(100% + 3px);
    right: 2px;
    z-index: 1;
    overflow: hidden;
    width: 1px;
    height: 1px;
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    white-space: nowrap;
    clip-path: inset(50%);
  }

  .column-preference-state.error {
    width: auto;
    height: auto;
    overflow: visible;
    color: var(--wa-danger, #c9473c);
    clip-path: none;
  }

  .toolbar-controls .wa-admin-action {
    width: auto;
    min-width: 64px;
    padding-inline: 12px;
    white-space: nowrap;
  }

  :global(.decision-agenda-list.admin-data-list) {
    position: relative;
    z-index: 1;
    grid-row: auto;
    height: 100%;
    min-height: 0;
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

  .issue-id-cell {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .issue-id-cell .table-link,
  .issue-id-cell .table-id {
    white-space: nowrap;
  }

  .decision-title-button {
    width: 100%;
    min-width: 0;
    min-height: 44px;
    display: flex;
    align-items: center;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;
  }

  .decision-title-button strong,
  .decision-cell-value {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .decision-title-button strong {
    color: var(--wa-text-strong);
    font-size: 13px;
    line-height: 1.28;
    font-weight: 780;
  }

  .decision-title-button:focus-visible {
    outline: 2px solid var(--wa-border-focus, var(--wa-accent));
    outline-offset: 2px;
  }

  .decision-list-empty {
    min-height: 220px;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: var(--wa-space-2);
    padding: var(--wa-space-8);
    text-align: center;
  }

  .decision-list-empty strong {
    color: var(--wa-text-strong);
    font-size: 16px;
  }

  .decision-list-empty span {
    color: var(--wa-text-muted);
    font-size: 13px;
  }

  .inspector-kicker {
    display: block;
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 780;
  }

  .requirement-drawer-meta,
  .section-row-head,
  .intervention-actions {
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
  }

  .requirement-drawer-meta {
    flex-wrap: wrap;
  }

  .section-row-head {
    justify-content: space-between;
  }

  .requirement-drawer-meta strong {
    color: var(--wa-text-strong);
    font-variant-numeric: tabular-nums;
  }

  .requirement-drawer-body {
    --wa-control-h: 38px;
    min-width: 0;
    min-height: 0;
    display: grid;
    grid-template-rows: auto auto auto;
    align-content: start;
    gap: 12px;
    padding: 16px 22px 18px;
    overflow: visible;
  }

  .requirement-modal-meta {
    padding-bottom: 12px;
    border-bottom: 1px solid rgba(116, 139, 156, 0.18);
  }

  .requirement-drawer-section,
  .inspector-section,
  .intervention-section {
    min-width: 0;
  }

  .requirement-drawer-section {
    display: grid;
    gap: 10px;
    padding: 0 0 14px;
    border-bottom: 1px solid rgba(116, 139, 156, 0.18);
  }

  .drawer-section-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
  }

  .drawer-section-heading h3,
  .drawer-section-heading span {
    margin: 0;
  }

  .drawer-section-heading h3 {
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 800;
  }

  .drawer-section-heading span,
  .section-row-head span {
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 660;
  }

  .fact-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px 20px;
    margin: 0;
  }

  .fact-grid div {
    min-width: 0;
    padding: 2px 0;
  }

  .fact-grid .project-fact {
    grid-column: span 2;
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
    font-size: 13px;
    line-height: 1.3;
    font-weight: 760;
    overflow-wrap: anywhere;
  }

  .inspector-section-grid {
    display: grid;
    grid-template-columns: minmax(0, 1.3fr) minmax(0, 0.7fr);
    grid-template-rows: minmax(108px, 0.85fr) minmax(140px, 1.15fr);
    gap: 0;
    align-items: stretch;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }

  .requirement-modal-body .inspector-section-grid {
    grid-template-rows: auto auto;
    height: auto;
  }

  .inspector-section {
    display: grid;
    gap: 6px;
    align-content: start;
    min-width: 0;
    min-height: min-content;
    height: 100%;
    padding: 14px 0;
  }

  .inspector-section-grid > .inspector-section:nth-child(odd) {
    padding-right: 20px;
  }

  .inspector-section-grid > .inspector-section:nth-child(even) {
    padding-left: 20px;
    border-left: 1px solid rgba(116, 139, 156, 0.18);
  }

  .inspector-section-grid > .inspector-section:nth-child(-n + 2) {
    padding-top: 2px;
    border-bottom: 1px solid rgba(116, 139, 156, 0.18);
  }

  .inspector-section-grid > .inspector-section:nth-child(n + 3) {
    padding-bottom: 2px;
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
    line-height: 1.5;
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

  .empty-history-section {
    grid-template-rows: auto minmax(0, 1fr);
    align-content: stretch;
  }

  .empty-history-section ul {
    padding-left: 0;
    list-style: none;
    place-content: center;
    text-align: center;
  }

  .intervention-form {
    position: relative;
    z-index: 2;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
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
    min-height: max-content;
    padding: 14px 0 0;
    border-top: 1px solid rgba(116, 139, 156, 0.18);
    overflow: visible;
  }

  .intervention-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
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

  .timeline-drawer-layer {
    position: fixed;
    inset: 0;
    z-index: var(--wa-layer-modal, 120);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(440px, 560px);
    pointer-events: none;
  }

  .timeline-drawer-backdrop {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: rgba(13, 23, 34, 0.28);
    backdrop-filter: blur(2px);
    -webkit-backdrop-filter: blur(2px);
    cursor: default;
    pointer-events: auto;
  }

  .timeline-drawer {
    position: relative;
    z-index: 1;
    grid-column: 2;
    min-width: 0;
    height: 100dvh;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    border-left: 1px solid var(--wa-border-soft);
    background: var(--wa-surface-flat, #fbfdff);
    box-shadow: -16px 0 40px rgba(13, 23, 34, 0.18);
    pointer-events: auto;
  }

  .timeline-drawer-header {
    min-height: 112px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: flex-start;
    gap: var(--wa-space-4);
    padding: 20px 22px;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .timeline-drawer-header > div:first-child {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-1);
  }

  .timeline-drawer-eyebrow,
  .timeline-drawer-header h2,
  .timeline-drawer-header p {
    margin: 0;
  }

  .timeline-drawer-eyebrow {
    color: var(--wa-accent-strong);
    font-size: 10px;
    font-weight: 780;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .timeline-drawer-header h2 {
    color: var(--wa-text-strong);
    font-size: 20px;
    line-height: 1.25;
  }

  .timeline-drawer-header p {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.5;
  }

  .timeline-drawer-actions {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
  }

  .timeline-drawer-count {
    color: var(--wa-text-muted);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  .timeline-drawer-body {
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
    padding: 0 22px 28px;
  }

  .timeline-day {
    display: grid;
  }

  .timeline-day-heading {
    position: sticky;
    top: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
    padding: 14px 0 10px;
    border-bottom: 1px solid var(--wa-border-soft);
    background: var(--wa-surface-flat, #fbfdff);
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 760;
  }

  .timeline-day-heading h3,
  .timeline-day-heading span {
    margin: 0;
    color: inherit;
    font: inherit;
  }

  .timeline-day ol {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .timeline-drawer-event {
    position: relative;
    min-width: 0;
    display: grid;
    grid-template-columns: 84px minmax(0, 1fr);
    gap: var(--wa-space-4);
    padding: 16px 0 16px 24px;
  }

  .timeline-drawer-event::before {
    content: "";
    position: absolute;
    top: 22px;
    left: 4px;
    z-index: 1;
    width: 9px;
    height: 9px;
    border-radius: 999px;
    background: var(--wa-info);
    box-shadow: 0 0 0 4px var(--wa-info-soft);
  }

  .timeline-drawer-event.manual::before {
    background: var(--wa-accent);
    box-shadow: 0 0 0 4px var(--wa-accent-soft);
  }

  .timeline-drawer-event:not(:last-child)::after {
    content: "";
    position: absolute;
    top: 33px;
    bottom: -6px;
    left: 8px;
    width: 1px;
    background: var(--wa-border-soft);
  }

  .timeline-event-time {
    display: grid;
    align-content: start;
    gap: var(--wa-space-1);
  }

  .timeline-event-time time {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  .timeline-event-kind {
    color: var(--wa-info);
    font-size: 10px;
    font-weight: 760;
  }

  .timeline-drawer-event.manual .timeline-event-kind {
    color: var(--wa-accent-strong);
  }

  .timeline-event-content {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-2);
  }

  .timeline-event-title,
  .timeline-event-evidence {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
  }

  .timeline-event-title strong {
    min-width: 0;
    color: var(--wa-text-strong);
    font-size: 12px;
  }

  .timeline-event-title a,
  .timeline-event-evidence a {
    color: var(--wa-info);
    text-decoration: none;
  }

  .timeline-event-title a:hover,
  .timeline-event-evidence a:hover {
    color: var(--wa-accent-strong);
  }

  .timeline-event-content p,
  .timeline-drawer-empty p {
    margin: 0;
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.55;
  }

  .timeline-event-evidence {
    color: var(--wa-text-muted);
    font-size: 10px;
  }

  .timeline-event-evidence span {
    color: inherit;
  }

  .timeline-drawer-empty {
    min-height: 240px;
    display: grid;
    place-content: center;
    gap: var(--wa-space-2);
    text-align: center;
  }

  .timeline-drawer-empty strong {
    color: var(--wa-text-strong);
  }

  @media (max-width: 900px) {
    .toolbar-controls {
      grid-template-columns: repeat(2, minmax(0, 1fr)) auto;
    }

    .search-control {
      grid-column: span 2;
    }

    .search-control,
    .toolbar-select {
      width: 100%;
    }
  }

  @media (max-width: 800px) {
    .search-control,
    .toolbar-select :global(.select-trigger),
    .toolbar-select :global(.select-inline-input),
    .toolbar-select :global(.select-toggle),
    .toolbar-select :global(.multi-select-trigger),
    .toolbar-select :global(.multi-select-chevron),
    .toolbar-controls .wa-admin-action {
      min-height: var(--wa-touch-h, 44px);
      height: var(--wa-touch-h, 44px);
    }

    .column-select :global(.multi-select-group.summary-mode .multi-select-trigger) {
      min-height: var(--wa-touch-h, 44px);
      height: var(--wa-touch-h, 44px);
    }
  }

  @media (max-width: 640px) {
    .decision-admin {
      grid-template-rows: 116px 96px minmax(0, 1fr);
      gap: 10px;
    }

    .decision-metrics,
    .decision-stage-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
    }

    .decision-metric {
      padding-block: 7px;
    }

    .search-control,
    .toolbar-select :global(.select-trigger),
    .toolbar-select :global(.select-inline-input),
    .toolbar-select :global(.select-toggle),
    .toolbar-select :global(.multi-select-trigger),
    .toolbar-select :global(.multi-select-chevron) {
      min-height: var(--wa-touch-h, 44px);
      height: var(--wa-touch-h, 44px);
    }

    .requirement-drawer-body {
      grid-template-rows: auto auto auto;
      align-content: start;
      padding: 12px 16px 16px;
    }

    .inspector-section-grid {
      grid-template-rows: none;
      grid-auto-rows: max-content;
      min-height: max-content;
      height: max-content;
    }

    .inspector-section {
      min-height: max-content;
      height: auto;
    }

    .fact-grid,
    .intervention-form {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .intervention-actions {
      flex-wrap: nowrap;
    }

    .intervention-actions .wa-admin-action {
      flex: 1 1 0;
      min-width: 0;
      min-height: 44px;
      padding-inline: 10px;
    }

    .timeline-drawer-layer {
      grid-template-columns: 1fr;
    }

    .timeline-drawer {
      grid-column: 1;
      width: 100%;
    }

    .timeline-drawer-header {
      min-height: 104px;
      padding: 16px;
    }

    .timeline-drawer-body {
      padding: 0 16px 24px;
    }

    .timeline-drawer-event {
      grid-template-columns: 1fr;
      gap: var(--wa-space-2);
    }

    .timeline-event-time {
      display: flex;
      align-items: baseline;
      gap: var(--wa-space-2);
    }

    .intervention-actions {
      align-items: stretch;
      flex-direction: row;
    }

    .toolbar-copy {
      grid-template-columns: 1fr;
    }

    .toolbar-copy small {
      justify-self: start;
    }

    .toolbar-controls {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      align-items: stretch;
    }

    .search-control,
    .toolbar-select,
    .toolbar-controls .wa-admin-action,
    .wide-field {
      width: 100%;
    }
  }

  @media (max-width: 460px) {
    .requirement-drawer-body {
      grid-template-rows: auto auto auto;
      align-content: start;
    }

    .inspector-section-grid,
    .intervention-form {
      grid-template-columns: 1fr;
    }

    .inspector-section-grid {
      grid-template-rows: none;
      grid-auto-rows: max-content;
      min-height: max-content;
      height: max-content;
    }

    .inspector-section-grid > .inspector-section,
    .inspector-section-grid > .inspector-section:nth-child(odd),
    .inspector-section-grid > .inspector-section:nth-child(even),
    .inspector-section-grid > .inspector-section:nth-child(-n + 2),
    .inspector-section-grid > .inspector-section:nth-child(n + 3) {
      padding: 14px 0;
      border-left: 0;
      border-bottom: 1px solid rgba(116, 139, 156, 0.18);
    }

    .inspector-section-grid > .inspector-section:last-child {
      border-bottom: 0;
    }
  }

  @media (max-height: 760px) {
    .requirement-drawer-body {
      grid-template-rows: auto auto auto;
      align-content: start;
    }

    .inspector-section-grid {
      grid-template-rows: none;
      grid-auto-rows: max-content;
      min-height: max-content;
      height: max-content;
    }

    .inspector-section {
      min-height: max-content;
      height: auto;
    }
  }

  /* Component-owned surface hierarchy: one summary panel and one table panel. */
  .decision-admin {
    --decision-radius-panel: var(--wa-radius-lg, 14px);
    --decision-radius-card: var(--wa-radius-sm, 8px);
    --decision-radius-control: var(--wa-radius-pill, 999px);
    --wa-shadow-sm: none;
    --wa-shadow-md: none;
    --wa-shadow-glass: none;
    --wa-shadow-panel: none;
    --wa-shadow-glow: none;
    grid-template-rows: 116px minmax(0, 1fr);
    background: transparent;
  }

  .decision-admin::before {
    display: none;
  }

  .decision-summary-panel {
    min-width: 0;
    min-height: 0;
    display: grid;
    grid-template-rows: 68px 48px;
    overflow: hidden;
    border: 1px solid var(--wa-glass-outline, rgba(72, 98, 118, 0.18));
    border-top-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    border-radius: var(--decision-radius-panel);
    background: var(--wa-glass-panel, rgba(250, 253, 255, 0.76));
    box-shadow: none;
    -webkit-backdrop-filter: blur(16px) saturate(128%);
    backdrop-filter: blur(16px) saturate(128%);
  }

  .decision-metrics,
  .decision-stage-strip {
    gap: 0;
  }

  .decision-summary-panel .decision-metric {
    min-height: 0;
    padding: 10px 16px;
    overflow: hidden;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .decision-summary-panel .decision-metric:not(:first-child) {
    border-left: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
  }

  .decision-summary-panel .decision-metric::before {
    display: none;
  }

  .decision-metric small {
    font-size: 12px;
  }

  .decision-stage-strip {
    gap: 6px;
    padding: 5px;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    background: rgba(255, 255, 255, 0.24);
  }

  .decision-summary-panel .stage-chip {
    min-height: 0;
    padding: 6px 14px;
    border: 1px solid transparent;
    border-radius: var(--wa-radius-pill, 999px);
    background: rgba(255, 255, 255, 0.4);
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    transform: none;
  }

  .decision-summary-panel .stage-chip:hover {
    border-color: var(--wa-glass-highlight, rgba(255, 255, 255, 0.82));
    background: rgba(255, 255, 255, 0.7);
    box-shadow: none;
    transform: none;
  }

  .decision-summary-panel .stage-chip.active {
    border-color: rgba(1, 139, 141, 0.24);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.1));
    color: var(--wa-accent-strong, #006f76);
    box-shadow: none;
    transform: none;
  }

  .decision-summary-panel .stage-chip.active span,
  .decision-summary-panel .stage-chip.active strong {
    color: inherit;
  }

  .decision-table-section {
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }

  .release-fact-strip {
    min-width: 0;
    min-height: 44px;
    display: flex;
    align-items: center;
    gap: 14px;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    padding: 7px 12px;
    background: transparent;
    box-shadow: none;
  }

  .release-fact-label,
  .release-fact-main {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .release-fact-label {
    flex: 0 0 180px;
  }

  .release-fact-label strong,
  .release-fact-main strong {
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .release-fact-label span,
  .release-fact-main span,
  .release-fact-state,
  .release-fact-scope,
  .release-fact-evidence,
  .release-fact-more {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
  }

  .release-fact-main {
    flex: 1 1 220px;
  }

  .release-fact-scope {
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .release-fact-scope span + span,
  .release-fact-evidence,
  .release-fact-more {
    border-left: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    padding-left: 10px;
  }

  .release-fact-evidence,
  .release-fact-more {
    flex: none;
    font-variant-numeric: tabular-nums;
  }

  .release-fact-state {
    min-width: 0;
    flex: 1 1 auto;
  }

  .release-fact-state.is-error {
    color: var(--wa-danger, #c9473c);
  }

  .decision-admin :global(.btn),
  .decision-admin :global(.select-trigger),
  .decision-admin :global(.multi-select-trigger),
  .decision-admin :global(.date-trigger),
  .decision-admin .stage-chip,
  .decision-admin .timeline-drawer {
    box-shadow: none !important;
  }

  .decision-admin :global(:focus-visible) {
    outline: 2px solid rgba(0, 143, 150, 0.24);
    outline-offset: 2px;
  }

  @media (max-width: 640px) {
    .decision-admin {
      grid-template-rows: 212px auto minmax(0, 1fr);
    }

    .decision-summary-panel {
      grid-template-rows: 116px 96px;
    }

    .decision-summary-panel .decision-metric:nth-child(odd),
    .decision-summary-panel .stage-chip:nth-child(odd) {
      border-left: 0;
    }

    .decision-summary-panel .decision-metric:nth-child(n + 3),
    .decision-summary-panel .stage-chip:nth-child(n + 3) {
      border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    }

    .release-fact-strip {
      min-height: 64px;
      align-items: flex-start;
      flex-wrap: wrap;
      gap: 5px 12px;
      padding: 9px 10px;
    }

    .release-fact-label {
      flex: 1 1 100%;
      grid-template-columns: auto minmax(0, 1fr);
      align-items: baseline;
      gap: 8px;
    }

    .release-fact-main {
      flex: 1 1 160px;
    }

    .release-fact-scope {
      flex: 1 1 100%;
      order: 3;
      white-space: normal;
    }

    .release-fact-evidence,
    .release-fact-more {
      padding-left: 8px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .stage-chip {
      transition: none;
    }
  }

</style>
