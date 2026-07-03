<script lang="ts">
  import { onMount } from 'svelte';
  export let currentUser = '';

  interface TelemetrySnippet {
    branch: string;
    last_commit: string;
    last_update: string;
  }

  interface GitCommitLog {
    id: number;
    task_id: string;
    repo: string;
    branch: string;
    commit_id: string;
    message: string;
    author: string;
    mr_iid: number;
    mr_url: string;
    action: string;
    created_at: string;
  }

  interface AgendaItem {
    task_id: string;
    title: string;
    assignee: string;
    repo?: string;
    status: string;
    issue_type: string; // bug, task
    risk_level: string;
    risk_type: string;
    desc: string;
    telemetry_snippet: TelemetrySnippet;
    due_date: string | null;
    decision_logs: string;
    git_logs?: GitCommitLog[]; // 新增：底层关联的真实 Git 历史轨迹
  }

  interface DecisionQueueItem {
    id: string;
    task_id: string;
    title: string;
    problem: string;
    evidence: string[];
    suggested_action: string;
    impact_scope: string;
    jump_label: string;
    jump_url: string;
    risk_level: string;
    risk_type: string;
    status: string;
    assignee: string;
    project: string;
    issue_type: string;
    updated_at: string;
    source: string;
  }

  interface DecisionActionPlan {
    decision_kind: string;
    primary_action: string;
    idle_cost: string;
    entry_label: string;
    entry_hint: string;
    source_label: string;
    can_intervene: boolean;
  }

  interface DecisionQueueMeaningStats {
    total: number;
    critical: number;
    open: number;
    schedule: number;
    execution: number;
    context: number;
    agenda: number;
  }

  interface SummaryTile {
    scope: string;
    value: string;
    label: string;
    detail: string;
    tone: 'safe' | 'warn' | 'danger' | 'info';
  }

  interface AutoDecision {
    time: string;
    task_id: string;
    message: string;
    assignee?: string;
    repo?: string; // 新增：仓储名
    branch?: string; // 新增：分支名
  }

  // Pre-calculated AI resolution recommendation helper
  interface AiResolvePlan {
    assignee: string;
    due_date: string;
    note: string;
    impact: string;
  }

  let agendaItems: AgendaItem[] = [];
  let autoDecisions: AutoDecision[] = [];
  let strongestBrainItems: DecisionQueueItem[] = [];
  let strongestBrainLoading = false;
  let strongestBrainError = '';
  let strongestBrainGeneratedAt = '';
  let strongestBrainApiAvailable = false;
  let selectedDecisionQueueItem: DecisionQueueItem | null = null;
  let loading = true;
  let errorMsg = '';

  let selectedItem: AgendaItem | null = null;
  let decisionLoading = false;
  let decisionSuccess = '';
  let decisionError = '';

  // Human intervention overrides
  let newAssignee = '';
  let newDueDate = '';
  let decisionNote = '';
  let operator = '';

  // 绑定当前登录用户作为默认的调停人
  $: if (currentUser && !operator) {
    operator = currentUser;
  }

  // Live interventions in current meeting
  let localDecisions: string[] = [];

  // Multi-Criteria Filters
  let currentFilter: 'all' | 'task' | 'bug' = 'all';
  let selectedAssignee = 'all';
  let selectedRepo = 'all';
  let showRiskLevel: 'all' | 'risks' = 'risks'; // 'risks' or 'all'

  // Custom select states
  let showAssigneeDropdown = false;
  let showRepoDropdown = false;
  let showOverrideAssigneeDropdown = false;
  let showOverrideDatePicker = false;
  let assigneeSelectEl: HTMLElement;
  let repoSelectEl: HTMLElement;
  let overrideAssigneeSelectEl: HTMLElement;
  let overrideDatePickerEl: HTMLElement;
  let overrideDatePickerCursor = new Date();

  let assigneeSearchText = '';
  let projectSearchText = '';

  $: if (!showAssigneeDropdown) assigneeSearchText = '';
  $: if (!showRepoDropdown) projectSearchText = '';

  const monthNames = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'];
  const weekdayNames = ['一', '二', '三', '四', '五', '六', '日'];

  // 核心成员白名单
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

  let jiraBaseUrl = '';

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
        }
      }
    } catch (e) {
      console.error('Failed to fetch config on dashboard:', e);
    }
  }

  // 响应式过滤外部协同/非核心成员
  $: filteredAgendaItems = agendaItems.filter(item => isCoreMember(item.assignee));
  $: filteredAutoDecisions = autoDecisions.filter(dec => !dec.assignee || isCoreMember(dec.assignee));

  // 响应式计算指标（仅包含核心成员）
  $: activeBugCount = filteredAgendaItems.filter(item => item.issue_type === 'bug').length;
  $: activeTaskCount = filteredAgendaItems.filter(item => item.issue_type !== 'bug').length;
  $: redZoneCount = filteredAgendaItems.filter(item => item.risk_level === 'critical').length;
  $: totalActiveTasks = activeBugCount + activeTaskCount;
  $: decisionHealthPercent = totalActiveTasks > 0
    ? Math.max(8, Math.round(((totalActiveTasks - redZoneCount) / totalActiveTasks) * 100))
    : 100;
  $: decisionHealthLabel = redZoneCount > 0 ? 'ATTENTION' : 'SAFE';

  $: hasShadowAgenda = false; // 既然完全剥离，就不存在外部协同任务的快捷方式了

  // Dynamic filter option lists
  $: assigneesList = [
    'all', 
    ...Array.from(new Set([
      ...filteredAgendaItems.map(item => item.assignee).filter(Boolean),
      ...strongestBrainItems.map(item => item.assignee).filter(Boolean)
    ]))
  ];
  $: projectList = [
    'all',
    ...Array.from(new Set([
      ...filteredAgendaItems.map(item => item.repo).filter((repo): repo is string => !!repo),
      ...strongestBrainItems.map(item => item.project).filter(Boolean)
    ]))
  ];
  $: aiPlan = getAiResolvePlan(selectedItem);
  $: overrideAssigneeOptions = Array.from(new Set([
    selectedItem?.assignee,
    aiPlan.assignee,
    ...Array.from(coreMembers),
    ...assigneesList.filter(name => name !== 'all')
  ].filter(Boolean) as string[]));
  $: newDueDateDisplay = formatDateLabel(newDueDate);

  // Combined filter with automatic risk-priority sorting (critical > warning > safe)
  $: filteredItems = filteredAgendaItems
    .filter(item => {
      // 1. Issue Type Filter (Story vs Bug)
      if (currentFilter === 'task' && item.issue_type === 'bug') return false;
      if (currentFilter === 'bug' && item.issue_type !== 'bug') return false;
      
      // 2. Assignee Filter
      if (selectedAssignee !== 'all' && item.assignee !== selectedAssignee) return false;
      
      // 3. Project Filter
      if (selectedRepo !== 'all' && item.repo !== selectedRepo) return false;
      
      // 4. Severity Filter (only show risk items vs show all active items)
      if (showRiskLevel === 'risks' && item.risk_level === 'safe') return false;
      
      return true;
    })
    .sort((a, b) => {
      const getPriority = (lvl: string) => {
        if (lvl === 'critical') return 3;
        if (lvl === 'warning') return 2;
        return 1;
      };
      return getPriority(b.risk_level) - getPriority(a.risk_level);
    });
  $: agendaDecisionQueueItems = filteredAgendaItems.map((item, index) => normalizeAgendaDecisionItem(item, index));
  $: rawDecisionQueueItems = strongestBrainItems.length > 0 ? strongestBrainItems : agendaDecisionQueueItems;
  $: visibleDecisionQueueItems = rawDecisionQueueItems
    .filter(item => matchesDecisionQueueFilters(item))
    .sort(compareDecisionQueueItems);
  $: selectedDecisionQueueItem = reconcileSelectedDecisionQueueItem(selectedDecisionQueueItem, visibleDecisionQueueItems);
  $: selectedDecisionPlan = getDecisionActionPlan(selectedDecisionQueueItem);
  $: decisionQueueMeaningStats = buildDecisionQueueMeaningStats(visibleDecisionQueueItems);
  $: decisionSummaryTiles = buildDecisionSummaryTiles(visibleDecisionQueueItems);
  $: decisionQueueSourceLabel = strongestBrainItems.length > 0
    ? `Decision Queue API${strongestBrainGeneratedAt ? ` / ${formatTimeBrief(strongestBrainGeneratedAt)}` : ''}`
    : 'Agenda fallback';

  function getAiResolvePlan(item: AgendaItem | null): AiResolvePlan {
    if (!item) return { assignee: '', due_date: '', note: '', impact: '' };
    
    // Fallback tomorrow or next week
    const nextWeek = new Date();
    nextWeek.setDate(nextWeek.getDate() + 7);
    const dateStr = nextWeek.toISOString().split('T')[0];

    if (item.issue_type === 'bug') {
      return {
        assignee: item.assignee === '朱家聪' ? '白凌云' : '朱家聪', // Recommend swapping to another expert
        due_date: dateStr,
        note: `自动调停：该缺陷修复受阻，转派专家协助并顺延截止时间。`,
        impact: '🪲 阻碍影响：此 Bug 若不解决，将直接导致 2 个关联下游 Story 无法进行合并测试。'
      };
    } else {
      return {
        assignee: '陈伟华',
        due_date: dateStr,
        note: `自动调停：开发进度遭遇卡点，转派陈伟华协助并重新对齐交付节点。`,
        impact: '🚀 交付影响：此需求挂起或延期，将导致 [FMS-Malaysia] 版本交付节点顺延约 1.5 天。'
      };
    }
  }

  function applyAiPlan() {
    if (!aiPlan) return;
    newAssignee = aiPlan.assignee;
    newDueDate = aiPlan.due_date;
    decisionNote = aiPlan.note;
    decisionSuccess = '已一键导入 AI 智能调停预案，请确认并提交干预。';
  }

  function selectAssignee(name: string) {
    selectedAssignee = name;
    showAssigneeDropdown = false;
  }

  function selectRepo(r: string) {
    selectedRepo = r;
    showRepoDropdown = false;
  }

  function asText(value: any, fallback = ''): string {
    if (value === null || value === undefined) return fallback;
    const text = String(value).trim();
    return text || fallback;
  }

  function normalizeTextList(value: any): string[] {
    if (Array.isArray(value)) {
      return value.map(item => asText(item)).filter(Boolean).slice(0, 5);
    }
    if (typeof value === 'string') {
      return value
        .split(/\n|；|;/)
        .map(item => item.trim())
        .filter(Boolean)
        .slice(0, 5);
    }
    return [];
  }

  function normalizeQueueRisk(value: any): string {
    const risk = asText(value, 'warning').toLowerCase();
    if (['critical', 'high', 'danger', 'red', 'p0'].includes(risk)) return 'critical';
    if (['safe', 'low', 'ok', 'done', 'green'].includes(risk)) return 'safe';
    return 'warning';
  }

  function normalizeQueueStatus(value: any): string {
    const status = asText(value, 'open').toLowerCase();
    if (['processing', 'triaging', 'in_progress', 'doing'].includes(status)) return 'processing';
    if (['decided', 'resolved', 'done', 'closed'].includes(status)) return 'decided';
    if (['ignored', 'dismissed'].includes(status)) return 'ignored';
    if (['meeting', 'escalated', 'escalated_meeting'].includes(status)) return 'meeting';
    return 'open';
  }

  function normalizeQueueSource(value: any): string {
    const source = asText(value, 'strongest_brain').toLowerCase();
    if (source.includes('schedule')) return 'schedule';
    if (source.includes('execution') || source.includes('evidence')) return 'execution';
    if (source.includes('context')) return 'context';
    if (source.includes('agenda')) return 'agenda_fallback';
    return source || 'strongest_brain';
  }

  function queueSourceLabel(source: string): string {
    switch (normalizeQueueSource(source)) {
      case 'schedule': return '排期';
      case 'execution': return '执行证据';
      case 'context': return '上下文';
      case 'agenda_fallback': return '旧Agenda';
      default: return '综合';
    }
  }

  function hasAgendaInterventionTarget(item: DecisionQueueItem | null): boolean {
    if (!item) return false;
    return filteredAgendaItems.some(agenda => agenda.task_id === item.task_id);
  }

  function statusText(status: string): string {
    switch (normalizeQueueStatus(status)) {
      case 'processing': return '处理中';
      case 'decided': return '已决策';
      case 'ignored': return '已忽略';
      case 'meeting': return '升级会议';
      default: return '未处理';
    }
  }

  function riskText(risk: string): string {
    switch (normalizeQueueRisk(risk)) {
      case 'critical': return '高风险';
      case 'safe': return '正常';
      default: return '中风险';
    }
  }

  function normalizeDecisionQueueItem(raw: any, index: number): DecisionQueueItem {
    const taskID = asText(raw.task_id || raw.taskId || raw.demand_id || raw.id, `decision-${index + 1}`);
    const title = asText(raw.title || raw.problem || raw.issue || raw.summary, '未命名异常决策');
    const evidence = normalizeTextList(raw.evidence || raw.evidence_list || raw.evidence_chain || raw.signals);
    const jumpTarget = raw.jump_target || raw.target || raw.link || {};
    const jumpURL = asText(raw.jump_url || raw.target_url || raw.url || jumpTarget.url);
    const jumpLabel = asText(raw.jump_label || raw.target_label || jumpTarget.label || taskID);

    return {
      id: asText(raw.id || raw.queue_id || raw.decision_id, `${taskID}-${index}`),
      task_id: taskID,
      title,
      problem: asText(raw.problem || raw.reason || raw.desc, title),
      evidence: evidence.length > 0 ? evidence : ['等待证据链读模型回传'],
      suggested_action: asText(raw.suggested_action || raw.recommendation || raw.action, '补齐事实后再决定是否转派、延期、拆分或升级会议'),
      impact_scope: asText(raw.impact_scope || raw.impact || raw.scope, '影响范围待读模型补齐'),
      jump_label: jumpLabel,
      jump_url: jumpURL,
      risk_level: normalizeQueueRisk(raw.risk_level || raw.risk || raw.severity),
      risk_type: asText(raw.risk_type || raw.type, 'exception'),
      status: normalizeQueueStatus(raw.status || raw.handling_status),
      assignee: asText(raw.assignee || raw.owner || raw.recommended_owner, '未指派'),
      project: asText(raw.project || raw.repo || raw.project_key, '未归属'),
      issue_type: asText(raw.issue_type || raw.kind, 'task').toLowerCase(),
      updated_at: asText(raw.updated_at || raw.last_update || raw.created_at),
      source: normalizeQueueSource(raw.source || raw.origin || raw.kind_source || 'strongest_brain')
    };
  }

  function normalizeAgendaDecisionItem(item: AgendaItem, index: number): DecisionQueueItem {
    const evidence = [
      item.telemetry_snippet?.branch ? `分支 ${item.telemetry_snippet.branch}` : '',
      item.telemetry_snippet?.last_commit ? `最近提交 ${item.telemetry_snippet.last_commit}` : '',
      item.desc
    ].filter(Boolean);
    const jiraURL = jiraBaseUrl && item.task_id && !item.task_id.startsWith('TASK-')
      ? `${jiraBaseUrl}/browse/${item.task_id}`
      : '';

    return {
      id: `${item.task_id || 'agenda'}-${index}`,
      task_id: item.task_id,
      title: item.title,
      problem: item.desc || getAiRecommendation(item),
      evidence: evidence.length > 0 ? evidence : ['Agenda 已命中异常，但缺少代码证据摘要'],
      suggested_action: getBrainFlowSuggestion(item),
      impact_scope: getAiResolvePlan(item).impact || '影响范围待补充',
      jump_label: item.task_id || '查看目标',
      jump_url: jiraURL,
      risk_level: normalizeQueueRisk(item.risk_level),
      risk_type: item.risk_type || 'agenda_risk',
      status: item.decision_logs ? 'processing' : 'open',
      assignee: item.assignee || '未指派',
      project: item.repo || '未归属',
      issue_type: item.issue_type || 'task',
      updated_at: item.telemetry_snippet?.last_update || item.due_date || '',
      source: 'agenda_fallback'
    };
  }

  function matchesDecisionQueueFilters(item: DecisionQueueItem): boolean {
    if (currentFilter === 'task' && item.issue_type === 'bug') return false;
    if (currentFilter === 'bug' && item.issue_type !== 'bug') return false;
    if (selectedAssignee !== 'all' && item.assignee !== selectedAssignee) return false;
    if (selectedRepo !== 'all' && item.project !== selectedRepo) return false;
    if (showRiskLevel === 'risks' && normalizeQueueRisk(item.risk_level) === 'safe') return false;
    return true;
  }

  function queueRiskRank(risk: string): number {
    const normalized = normalizeQueueRisk(risk);
    if (normalized === 'critical') return 3;
    if (normalized === 'warning') return 2;
    return 1;
  }

  function compareDecisionQueueItems(a: DecisionQueueItem, b: DecisionQueueItem): number {
    const riskDelta = queueRiskRank(b.risk_level) - queueRiskRank(a.risk_level);
    if (riskDelta !== 0) return riskDelta;
    const statusDelta = (normalizeQueueStatus(a.status) === 'open' ? -1 : 0) - (normalizeQueueStatus(b.status) === 'open' ? -1 : 0);
    if (statusDelta !== 0) return statusDelta;
    return (b.updated_at || '').localeCompare(a.updated_at || '');
  }

  function buildDecisionSummaryTiles(items: DecisionQueueItem[]): SummaryTile[] {
    const total = items.length;
    const critical = items.filter(item => normalizeQueueRisk(item.risk_level) === 'critical').length;
    const open = items.filter(item => normalizeQueueStatus(item.status) === 'open').length;
    const meeting = items.filter(item => normalizeQueueStatus(item.status) === 'meeting' || normalizeQueueRisk(item.risk_level) === 'critical').length;
    const evidenceGaps = items.filter(item => item.evidence.length === 0 || item.evidence[0].includes('等待')).length;

    return [
      {
        scope: '日内',
        value: `${critical}/${total}`,
        label: '高风险/全部',
        detail: open > 0 ? `${open} 个未处理异常需要今天拍板` : '暂无未处理异常，继续监听证据回流',
        tone: critical > 0 ? 'danger' : open > 0 ? 'warn' : 'safe'
      },
      {
        scope: '周会',
        value: `${meeting}`,
        label: '建议议题',
        detail: meeting > 0 ? '优先讨论红区、负责人冲突和需要升级的问题' : '周会可缩短为确认和复盘',
        tone: meeting > 0 ? 'warn' : 'safe'
      },
      {
        scope: '迭代',
        value: `${evidenceGaps}`,
        label: '证据缺口',
        detail: evidenceGaps > 0 ? '需要补齐上下文、排期、代码或验收证据' : '需求、证据、干预和复盘链路完整',
        tone: evidenceGaps > 0 ? 'info' : 'safe'
      }
    ];
  }

  function buildDecisionQueueMeaningStats(items: DecisionQueueItem[]): DecisionQueueMeaningStats {
    const stats: DecisionQueueMeaningStats = {
      total: items.length,
      critical: 0,
      open: 0,
      schedule: 0,
      execution: 0,
      context: 0,
      agenda: 0
    };
    for (const item of items) {
      if (normalizeQueueRisk(item.risk_level) === 'critical') stats.critical++;
      if (normalizeQueueStatus(item.status) === 'open') stats.open++;
      switch (normalizeQueueSource(item.source)) {
        case 'schedule':
          stats.schedule++;
          break;
        case 'execution':
          stats.execution++;
          break;
        case 'context':
          stats.context++;
          break;
        case 'agenda_fallback':
          stats.agenda++;
          break;
      }
    }
    return stats;
  }

  function defaultDecisionActionPlan(): DecisionActionPlan {
    return {
      decision_kind: '监听',
      primary_action: '暂无需要人工拍板的异常，保持后台事实收集。',
      idle_cost: '无即时阻塞。',
      entry_label: '等待新异常',
      entry_hint: '系统会在风险升级、证据缺口或负责人冲突时重新入队。',
      source_label: '综合',
      can_intervene: false
    };
  }

  function getDecisionKind(item: DecisionQueueItem): string {
    const riskType = asText(item.risk_type).toLowerCase();
    const source = normalizeQueueSource(item.source);
    if (source === 'context') return '补齐语料';
    if (riskType.includes('missing_schedule') || riskType.includes('unscheduled')) return '补排期';
    if (riskType.includes('overdue')) return '改承诺';
    if (riskType.includes('due_soon')) return '临期确认';
    if (riskType.includes('stale') || riskType.includes('no_commit')) return '破阻塞';
    if (riskType.includes('mismatch') || riskType.includes('conflict')) return '对齐事实';
    if (riskType.includes('orphan') || riskType.includes('unbound')) return '补归属';
    if (source === 'execution') return '补证据';
    return '人工判断';
  }

  function getIdleCost(item: DecisionQueueItem): string {
    const riskType = asText(item.risk_type).toLowerCase();
    const risk = normalizeQueueRisk(item.risk_level);
    const source = normalizeQueueSource(item.source);
    if (source === 'context') return '继续缺少系统事实会让需求解构、估算和周会摘要发生漂移。';
    if (riskType.includes('missing_schedule') || riskType.includes('unscheduled')) return '需求会停留在看板状态，无法进入可追踪负责人、分支和截止日闭环。';
    if (riskType.includes('overdue')) return '延期原因不落账会继续吞掉迭代缓冲，并让下游验收窗口失真。';
    if (riskType.includes('due_soon')) return '临期未确认会在下一轮刷新变成逾期，压缩合并和回归时间。';
    if (riskType.includes('stale') || riskType.includes('no_commit')) return '静默会掩盖真实阻塞，直到评审、合并或验收阶段集中爆雷。';
    if (riskType.includes('mismatch')) return '看板状态和代码事实继续分叉，复盘时无法判断真实完成度。';
    if (riskType.includes('orphan') || riskType.includes('unbound')) return '执行证据无法回流父需求，完成记录会变成孤岛。';
    if (risk === 'critical') return '红区问题会持续占用交付缓冲，需要今天给出明确处理口径。';
    return item.impact_scope || '影响范围需要在拍板前补齐。';
  }

  function getDecisionActionPlan(item: DecisionQueueItem | null): DecisionActionPlan {
    if (!item) return defaultDecisionActionPlan();
    const canIntervene = hasAgendaInterventionTarget(item);
    const hasJump = Boolean(item.jump_url);
    return {
      decision_kind: getDecisionKind(item),
      primary_action: item.suggested_action || '补齐事实后决定转派、延期、拆分或升级会议。',
      idle_cost: getIdleCost(item),
      entry_label: canIntervene ? '进入调停' : hasJump ? `打开${item.jump_label || '目标'}` : '定位来源',
      entry_hint: canIntervene
        ? '已匹配下方人工干预面板，可直接转派、改期或挂起。'
        : hasJump
          ? '先打开外部目标确认事实，再回到队列记录决策。'
          : `来自${queueSourceLabel(item.source)}读模型，请回到对应看板补齐事实。`,
      source_label: queueSourceLabel(item.source),
      can_intervene: canIntervene
    };
  }

  function reconcileSelectedDecisionQueueItem(current: DecisionQueueItem | null, items: DecisionQueueItem[]): DecisionQueueItem | null {
    if (items.length === 0) return null;
    if (current) {
      const match = items.find(item => item.id === current.id || item.task_id === current.task_id);
      if (match) return match;
    }
    return items[0];
  }

  function formatTimeBrief(timeStr: string): string {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    if (Number.isNaN(date.getTime())) return timeStr;
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hour = String(date.getHours()).padStart(2, '0');
    const minute = String(date.getMinutes()).padStart(2, '0');
    return `${month}-${day} ${hour}:${minute}`;
  }

  function focusDecisionQueueItem(item: DecisionQueueItem) {
    selectedDecisionQueueItem = item;
    const match = filteredAgendaItems.find(agenda => agenda.task_id === item.task_id);
    if (match) {
      selectItem(match);
    }
  }

  function selectOverrideAssignee(name: string) {
    newAssignee = name;
    showOverrideAssigneeDropdown = false;
  }

  function toggleAssigneeDropdown() {
    showAssigneeDropdown = !showAssigneeDropdown;
    showRepoDropdown = false;
  }

  function toggleRepoDropdown() {
    showRepoDropdown = !showRepoDropdown;
    showAssigneeDropdown = false;
  }

  function toggleOverrideAssigneeDropdown() {
    showOverrideAssigneeDropdown = !showOverrideAssigneeDropdown;
    showOverrideDatePicker = false;
  }

  function formatDateLabel(dateValue: string) {
    if (!dateValue) return '';
    const date = new Date(`${dateValue}T00:00:00`);
    if (Number.isNaN(date.getTime())) return dateValue;
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit'
    });
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

  function toggleOverrideDatePicker() {
    showOverrideDatePicker = !showOverrideDatePicker;
    showOverrideAssigneeDropdown = false;
    overrideDatePickerCursor = parseDateValue(newDueDate) || new Date();
  }

  function getOverrideCalendarDays(value: string) {
    const base = overrideDatePickerCursor;
    const year = base.getFullYear();
    const month = base.getMonth();
    const first = new Date(year, month, 1);
    const last = new Date(year, month + 1, 0);
    const leading = (first.getDay() + 6) % 7;
    const todayValue = toDateValue(new Date());
    const days: { value: string; label: number; muted: boolean; today: boolean; selected: boolean }[] = [];

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

  function moveOverrideDateMonth(delta: number) {
    overrideDatePickerCursor = new Date(overrideDatePickerCursor.getFullYear(), overrideDatePickerCursor.getMonth() + delta, 1);
  }

  function selectOverrideDueDate(value: string) {
    newDueDate = value;
    showOverrideDatePicker = false;
  }

  function clearOverrideDueDate() {
    newDueDate = '';
    showOverrideDatePicker = false;
  }

  function handleDocumentClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (showAssigneeDropdown && assigneeSelectEl && !assigneeSelectEl.contains(target)) {
      showAssigneeDropdown = false;
    }
    if (showRepoDropdown && repoSelectEl && !repoSelectEl.contains(target)) {
      showRepoDropdown = false;
    }
    if (showOverrideAssigneeDropdown && overrideAssigneeSelectEl && !overrideAssigneeSelectEl.contains(target)) {
      showOverrideAssigneeDropdown = false;
    }
    if (showOverrideDatePicker && overrideDatePickerEl && !overrideDatePickerEl.contains(target)) {
      showOverrideDatePicker = false;
    }
  }

  async function fetchStrongestBrainQueue() {
    strongestBrainLoading = true;
    strongestBrainError = '';
    try {
      const res = await fetch('/api/strongest-brain/decision-queue');
      if (res.status === 404) {
        strongestBrainApiAvailable = false;
        strongestBrainItems = [];
        return;
      }
      if (!res.ok) {
        throw new Error(`Decision Queue API ${res.status}`);
      }
      const data = await res.json();
      const rawItems = Array.isArray(data)
        ? data
        : (data.items || data.queue || data.decision_queue || data.decisionQueue || []);
      strongestBrainItems = rawItems.map((item: any, index: number) => normalizeDecisionQueueItem(item, index));
      strongestBrainGeneratedAt = data.generated_at || data.generatedAt || '';
      strongestBrainApiAvailable = true;
    } catch (err: any) {
      strongestBrainError = err.message || '决策队列接口暂不可用';
      strongestBrainItems = [];
      strongestBrainApiAvailable = false;
    } finally {
      strongestBrainLoading = false;
    }
  }

  async function fetchAgenda() {
    loading = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/agenda/summary');
      if (!res.ok) throw new Error('加载自决策大屏数据失败');
      const data = await res.json();
      
      const filtered = (data.agenda_items || []).filter((item: AgendaItem) => isCoreMember(item.assignee));
      agendaItems = data.agenda_items || [];
      autoDecisions = data.auto_decisions || [];
      
      // Auto-select first item if available
      if (filtered.length > 0 && !selectedItem) {
        selectItem(filtered[0]);
      } else if (selectedItem) {
        const updated = filtered.find((item: AgendaItem) => item.task_id === selectedItem?.task_id);
        if (updated) {
          selectedItem = updated;
        } else {
          selectedItem = filtered.length > 0 ? filtered[0] : null;
        }
      }
    } catch (err: any) {
      errorMsg = err.message || '网络连接异常';
    } finally {
      loading = false;
    }
  }

  function selectItem(item: AgendaItem) {
    selectedItem = item;
    newAssignee = item.assignee || '';
    newDueDate = item.due_date ? new Date(item.due_date).toISOString().split('T')[0] : '';
    decisionNote = '';
    decisionSuccess = '';
    decisionError = '';
  }

  async function submitDecision(action: string) {
    if (!selectedItem) return;
    decisionLoading = true;
    decisionSuccess = '';
    decisionError = '';

    const payload: any = { note: decisionNote };
    if (action === 'reassign') {
      if (!newAssignee.trim()) {
        decisionError = '请输入转派负责人';
        decisionLoading = false;
        return;
      }
      payload.assignee = newAssignee;
    } else if (action === 'reschedule') {
      if (!newDueDate) {
        decisionError = '请选择新的截止时间';
        decisionLoading = false;
        return;
      }
      payload.due_date = newDueDate;
    }

    try {
      const res = await fetch('/api/agenda/decision', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          task_id: selectedItem.task_id,
          action,
          payload,
          operator
        })
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || '提交人工调停指令失败');
      }

      const result = await res.json();
      decisionSuccess = '人工干预成功！已覆盖 AI 决策并同步外部系统。';
      
      const actionName = action === 'reassign' ? '覆盖指派' : (action === 'suspend' ? '强制挂起' : '调整截止期');
      localDecisions = [
        `[${new Date().toLocaleTimeString()}] 主管 ${operator} 调停 ${selectedItem.task_id}：执行了「${actionName}」干预。`,
        ...localDecisions
      ];

      await fetchAgenda();
    } catch (err: any) {
      decisionError = err.message || '提交干预请求时出错';
    } finally {
      decisionLoading = false;
    }
  }

  function getAiRecommendation(item: AgendaItem): string {
    if (item.issue_type === 'bug') {
      switch (item.risk_type) {
        case 'no_commit_48h':
          return '🤖 AI 自动诊断 (Bug)：该缺陷在“修复中”已 48h 无提交，本地可能重现失败。建议：安排技术专家加入协同排查，或一键导入下方的 AI 推荐调停预案。';
        case 'overdue':
          return '🤖 AI 自动诊断 (Bug)：该故障已超出解决时效。建议：调停并转派有经验的开发，避开核心发版窗口。';
        default:
          return '🤖 AI 自动诊断 (Bug)：故障滞留时间较长，建议在会中口头对齐是否存在设计冲突。';
      }
    } else {
      switch (item.risk_type) {
        case 'no_commit_48h':
          return '🤖 AI 自动诊断 (需求)：需求分支代码已搁置超 48 小时。建议：询问开发是否因依赖第三方接口受阻。可一键导入下方调停建议。';
        case 'overdue':
          return '🤖 AI 自动诊断 (需求)：需求工期超过预期。建议：调整计划或考虑将该需求部分范围剥离为下个迭代的影子卡片。';
        case 'potential_conflict':
          return '🤖 AI 自动诊断 (需求)：代码仓存在多分支并行开发，有潜在合并冲突。建议：指定一人作为主合入人。';
        default:
          return '🤖 AI 自动诊断 (需求)：任务开发进度偏慢，请对齐是否存在需求蔓延。';
      }
    }
  }

  function getBrainFlowSignals(item: AgendaItem): Array<{ label: string; value: string; tone: 'safe' | 'warn' | 'danger' | 'info' }> {
    const staleHours = item.telemetry_snippet.last_update
      ? Math.max(0, Math.round((Date.now() - new Date(item.telemetry_snippet.last_update).getTime()) / 36e5))
      : 0;
    const dueText = item.due_date ? formatDateLabel(item.due_date.slice(0, 10)) : '未设置';
    return [
      { label: '事实证据', value: item.telemetry_snippet.branch ? '有分支证据' : '缺少开发入口', tone: item.telemetry_snippet.branch ? 'safe' : 'warn' },
      { label: '静默时长', value: staleHours > 0 ? `${staleHours}h 未更新` : '等待首个事件', tone: staleHours > 48 ? 'danger' : staleHours > 24 ? 'warn' : 'info' },
      { label: 'Jira/看板', value: item.status.toUpperCase(), tone: item.risk_level === 'critical' ? 'danger' : 'info' },
      { label: '截止压力', value: dueText, tone: item.risk_type === 'overdue' ? 'danger' : 'info' },
      { label: '负责人负载', value: item.assignee || '未指派', tone: item.assignee ? 'safe' : 'warn' },
      { label: '验收/回滚', value: item.issue_type === 'bug' ? '优先补复现与回归' : '拆范围或补验收口径', tone: 'info' }
    ];
  }

  function getBrainFlowSuggestion(item: AgendaItem): string {
    if (item.risk_type === 'no_commit_48h') {
      return '建议先确认阻塞事实，再执行转派或结对协作；保留原负责人上下文，新增协助人承接下一次提交证据。';
    }
    if (item.risk_type === 'overdue') {
      return item.issue_type === 'bug'
        ? '建议把缺陷切成“复现、定位、修复、回归”四段，先锁定复现负责人和当天回归窗口。'
        : '建议拆分交付范围，将可独立验收部分继续推进，争议范围进入下一轮影子任务。';
    }
    if (item.risk_type === 'potential_conflict') {
      return '建议指定主合入人，先做分支同步和冲突预检，再进入 Review，避免看板状态早于代码事实。';
    }
    return '建议保持后台自动同步，只有在事实缺口、负责人变更、延期或验收风险出现时打断人工。';
  }

  onMount(() => {
    fetchAgenda();
    fetchStrongestBrainQueue();
    fetchConfig();
    const interval = setInterval(() => {
      fetchAgenda();
      fetchStrongestBrainQueue();
    }, 15000);
    document.addEventListener('click', handleDocumentClick);
    return () => {
      clearInterval(interval);
      document.removeEventListener('click', handleDocumentClick);
    };
  });
</script>

<div class="decision-war-room font-sans">
  <section class="strongest-brain-queue glass-panel">
    <div class="brain-queue-header">
      <div>
        <span class="eyebrow">STRONGEST BRAIN DECISION QUEUE</span>
        <h2>最强大脑决策队列</h2>
        <p>按日内、周会、迭代三种时间尺度收敛异常，只暴露需要人判断的问题。</p>
      </div>
      <div class="brain-source-stack font-mono">
        <span class="brain-source-chip {strongestBrainApiAvailable ? 'live' : 'fallback'}">
          {decisionQueueSourceLabel}
        </span>
        {#if strongestBrainLoading}
          <span class="brain-source-note">刷新中</span>
        {:else if strongestBrainError && strongestBrainItems.length === 0}
          <span class="brain-source-note">API 暂不可用，使用旧 agenda 兜底</span>
        {/if}
      </div>
    </div>

    <div class="brain-summary-grid">
      {#each decisionSummaryTiles as tile}
        <div class="brain-summary-tile summary-{tile.tone}">
          <span class="summary-scope font-mono">{tile.scope}</span>
          <strong class="font-mono">{tile.value}</strong>
          <span class="summary-label">{tile.label}</span>
          <p>{tile.detail}</p>
        </div>
      {/each}
    </div>

    {#if selectedDecisionQueueItem}
      <div class="brain-meaning-panel risk-{normalizeQueueRisk(selectedDecisionQueueItem.risk_level)}">
        <div class="meaning-focus">
          <span class="meaning-label font-mono">TODAY DECISION</span>
          <strong>{selectedDecisionPlan.decision_kind} · {selectedDecisionQueueItem.task_id}</strong>
          <p>{selectedDecisionPlan.idle_cost}</p>
        </div>
        <div class="meaning-action">
          <span class="meaning-label font-mono">{selectedDecisionPlan.source_label}</span>
          <strong>{selectedDecisionPlan.primary_action}</strong>
          <p>{selectedDecisionPlan.entry_hint}</p>
        </div>
        <div class="meaning-source-board font-mono">
          <span>排期 {decisionQueueMeaningStats.schedule}</span>
          <span>执行 {decisionQueueMeaningStats.execution}</span>
          <span>上下文 {decisionQueueMeaningStats.context}</span>
          <span>Agenda {decisionQueueMeaningStats.agenda}</span>
        </div>
      </div>
    {/if}

    {#if strongestBrainLoading && visibleDecisionQueueItems.length === 0}
      <div class="brain-loading-grid">
        <div class="brain-skeleton"></div>
        <div class="brain-skeleton"></div>
        <div class="brain-skeleton"></div>
      </div>
    {:else if visibleDecisionQueueItems.length === 0}
      <div class="brain-empty-state font-mono">
        当前没有需要人处理的异常决策。系统会继续监听 Jira、GitLab、排期、AI 解构和人工干预事件。
      </div>
    {:else}
      <div class="decision-queue-grid">
        {#each visibleDecisionQueueItems.slice(0, 8) as item}
          {@const actionPlan = getDecisionActionPlan(item)}
          <div
            class="decision-queue-card risk-{normalizeQueueRisk(item.risk_level)} {selectedDecisionQueueItem?.id === item.id ? 'selected' : ''}"
            on:click={() => focusDecisionQueueItem(item)}
            on:keydown={(e) => e.key === 'Enter' && focusDecisionQueueItem(item)}
            role="button"
            tabindex="0"
          >
            <div class="decision-card-top">
              <span class="queue-risk font-mono">{riskText(item.risk_level)}</span>
              <span class="queue-status font-mono status-{normalizeQueueStatus(item.status)}">{statusText(item.status)}</span>
            </div>

            <div class="decision-card-title-row">
              <div class="decision-card-id-row">
                <span class="queue-task-id font-mono">{item.task_id}</span>
                <span class="queue-source font-mono">{actionPlan.source_label}</span>
              </div>
              <h3>{item.title}</h3>
            </div>

            <div class="decision-card-section">
              <span>问题</span>
              <p>{item.problem}</p>
            </div>

            <div class="decision-card-plan">
              <div>
                <span>拍板类型</span>
                <strong>{actionPlan.decision_kind}</strong>
              </div>
              <p>{actionPlan.idle_cost}</p>
            </div>

            <div class="decision-card-section evidence">
              <span>证据</span>
              <ul>
                {#each item.evidence.slice(0, 3) as evidence}
                  <li>{evidence}</li>
                {/each}
              </ul>
            </div>

            <div class="decision-card-section">
              <span>建议动作</span>
              <p>{actionPlan.primary_action}</p>
            </div>

            <div class="decision-card-bottom font-mono">
              <span title={item.impact_scope}>影响: {item.impact_scope}</span>
              <span>负责人: {item.assignee}</span>
            </div>

            <div class="decision-card-actions">
              {#if item.jump_url}
                <a href={item.jump_url} target="_blank" rel="noopener noreferrer" on:click|stopPropagation>
                  {actionPlan.entry_label}
                </a>
              {:else if actionPlan.can_intervene}
                <span class="queue-target font-mono">点击卡片进入调停</span>
              {:else}
                <span class="queue-target font-mono">{actionPlan.entry_label}: {item.jump_label}</span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </section>
  
  <!-- Bento Grid Container -->
  <div class="bento-grid">
    
    <!-- Bento 1: Metrics Dashboard (1 Row / 1 Column) -->
    <div class="bento-card bento-metrics glass-panel">
      <div class="card-header-mini">
        <span class="eyebrow">DIAGNOSTIC STATUS</span>
        <h3>📊 遥测分类指标</h3>
      </div>
      
      <div class="metrics-grid">
        <div class="metric-item border-blue-dim">
          <span class="metric-label">🚀 进行中需求</span>
          <div class="metric-value-row">
            <span class="metric-val text-blue font-mono">{activeTaskCount}</span>
            <span class="unit">个</span>
          </div>
        </div>
        <div class="metric-item border-rose-dim">
          <span class="metric-label">🪲 进行中故障</span>
          <div class="metric-value-row">
            <span class="metric-val text-rose font-mono">{activeBugCount}</span>
            <span class="unit">个</span>
          </div>
        </div>
        <div class="metric-item border-purple-dim">
          <span class="metric-label">🚨 触发警告任务</span>
          <div class="metric-value-row">
            <span class="metric-val text-orange font-mono">{filteredAgendaItems.length}</span>
            <span class="unit">个</span>
          </div>
        </div>
        <div class="metric-item border-green-dim">
          <span class="metric-label">🤖 AI 自动流转率</span>
          <div class="metric-value-row">
            <span class="metric-val text-green font-mono">89%</span>
            <span class="unit">无感</span>
          </div>
        </div>
      </div>

      <div class="overall-progress">
        <div class="progress-labels">
          <span>迭代健康指数</span>
          <span class="font-mono text-green">91.4% SAFE</span>
        </div>
        <div class="progress-bar-bg">
          <div class="progress-bar-fill" style="width: 91.4%"></div>
        </div>
      </div>
    </div>

    <!-- Bento 2: Ambient Auto-Decisions Feed (2 Rows / 1 Column) -->
    <div class="bento-card bento-terminal glass-panel scrollable-panel">
      <div class="card-header-mini">
        <span class="eyebrow">AMBIENT TELEMETRY FLOW</span>
        <h3>⚡ AI 自动流转控制台</h3>
        <span class="pulse-indicator"></span>
      </div>
      
      <div class="terminal-container font-mono">
        <div class="terminal-header">
          <span>well-ambient-v0.1.0-agent-kernel logs</span>
        </div>
        <div class="terminal-body font-mono">
          {#each filteredAutoDecisions as dec}
            <div class="terminal-line">
              <div class="terminal-meta-row">
                <span class="time">[{dec.time}]</span>
                {#if jiraBaseUrl && dec.task_id && !dec.task_id.startsWith('TASK-')}
                  <a href="{jiraBaseUrl}/browse/{dec.task_id}" target="_blank" rel="noopener noreferrer" class="task-link">#{dec.task_id}</a>
                {:else}
                  <span class="task-link">#{dec.task_id}</span>
                {/if}
                {#if dec.repo}
                  <span class="terminal-repo">[{dec.repo}]</span>
                {/if}
                {#if dec.branch}
                  <span class="terminal-branch">({dec.branch})</span>
                {/if}
              </div>
              <p class="msg">{dec.message}</p>
            </div>
          {/each}
          <div class="terminal-line blink-line">
            <span class="time">[{new Date().toLocaleTimeString()}]</span>
            <span class="cursor">_</span>
            <p class="msg">监听 GitLab telemetry Webhook 中...</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Bento 3: Risk Diagnostics Router (1.5 Rows / 2 Columns) -->
    <div class="bento-card bento-agenda glass-panel" style={(showAssigneeDropdown || showRepoDropdown) ? 'z-index: 50; overflow: visible !important;' : ''}>
      <div class="panel-header-row">
        <div>
          <span class="eyebrow">RED-ZONE DECISIONS</span>
          <h2>⚠️ 红区卡点诊断盘</h2>
        </div>
        
        <div class="filter-bar font-mono">
          <!-- 1. Category Filter -->
          <div class="filter-group">
            <span class="filter-label-inline">分类:</span>
            <div class="filter-tabs">
              <button class="filter-btn {currentFilter === 'all' ? 'active' : ''}" on:click={() => currentFilter = 'all'}>
                ALL ({filteredAgendaItems.length})
              </button>
              <button class="filter-btn {currentFilter === 'task' ? 'active' : ''}" on:click={() => currentFilter = 'task'}>
                🚀 需求 ({filteredAgendaItems.filter(i=>i.issue_type!=='bug').length})
              </button>
              <button class="filter-btn {currentFilter === 'bug' ? 'active' : ''}" on:click={() => currentFilter = 'bug'}>
                🪲 故障 ({filteredAgendaItems.filter(i=>i.issue_type==='bug').length})
              </button>
            </div>
          </div>

          <!-- 2. Health/Risk Filter -->
          <div class="filter-group">
            <span class="filter-label-inline">健康度:</span>
            <div class="filter-tabs">
              <button class="filter-btn {showRiskLevel === 'risks' ? 'active' : ''}" on:click={() => showRiskLevel = 'risks'}>
                🚨 仅卡点 ({filteredAgendaItems.filter(i=>i.risk_level!=='safe').length})
              </button>
              <button class="filter-btn {showRiskLevel === 'all' ? 'active' : ''}" on:click={() => showRiskLevel = 'all'}>
                🌐 全活跃 ({filteredAgendaItems.length})
              </button>
            </div>
          </div>

          <!-- 3. Assignee Select -->
          <div class="filter-group select-group">
            <span class="filter-label-inline">负责人:</span>
            <div class="custom-select-container" bind:this={assigneeSelectEl} style={showAssigneeDropdown ? 'z-index: 30;' : 'z-index: 20;'}>
              <div class="custom-select-trigger combobox-trigger">
                <span class="filter-icon">👤</span>
                <input 
                  type="text" 
                  class="combobox-trigger-input"
                  placeholder={selectedAssignee === 'all' ? '全部' : selectedAssignee}
                  bind:value={assigneeSearchText}
                  on:focus|stopPropagation={() => showAssigneeDropdown = true}
                />
                <span class="select-arrow">{showAssigneeDropdown ? '▲' : '▼'}</span>
              </div>
              {#if showAssigneeDropdown}
                <div class="custom-select-options">
                  {#each assigneesList.filter(name => name === 'all' || !assigneeSearchText || name.toLowerCase().includes(assigneeSearchText.toLowerCase())) as name}
                    <button 
                      class="custom-option {selectedAssignee === name ? 'active' : ''}" 
                      on:click={() => selectAssignee(name)}
                    >
                      {name === 'all' ? '全部' : name}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <!-- 4. Project Select -->
          <div class="filter-group select-group">
            <span class="filter-label-inline">项目:</span>
            <div class="custom-select-container" bind:this={repoSelectEl} style={showRepoDropdown ? 'z-index: 30;' : 'z-index: 20;'}>
              <div class="custom-select-trigger combobox-trigger">
                <span class="filter-icon">📁</span>
                <input 
                  type="text" 
                  class="combobox-trigger-input"
                  placeholder={selectedRepo === 'all' ? '全部' : selectedRepo}
                  bind:value={projectSearchText}
                  on:focus|stopPropagation={() => showRepoDropdown = true}
                />
                <span class="select-arrow">{showRepoDropdown ? '▲' : '▼'}</span>
              </div>
              {#if showRepoDropdown}
                <div class="custom-select-options">
                  {#each projectList.filter(r => r === 'all' || !projectSearchText || r.toLowerCase().includes(projectSearchText.toLowerCase())) as r}
                    <button 
                      class="custom-option {selectedRepo === r ? 'active' : ''}" 
                      on:click={() => selectRepo(r)}
                    >
                      {r === 'all' ? '全部' : r}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>
      </div>

      <!-- Scrollable Bento Grid container for vertical flat tiling -->
      <div class="agenda-scroll-container">
        {#if loading && agendaItems.length === 0}
          <div class="state-msg">正在扫描 Git Telemetry 轨迹并诊断异常...</div>
        {:else if errorMsg}
          <div class="state-msg error-msg">❌ 加载异常: {errorMsg}</div>
        {:else if filteredItems.length === 0}
          <div class="state-msg safe-msg font-mono">
            🎉 当前过滤器下无卡点任务，AI 已自动托管流转
          </div>
        {:else}
          <div class="agenda-items-grid">
            {#each filteredItems as item}
              <div 
                class="agenda-tile-card {item.risk_level === 'critical' ? 'tile-red' : 'tile-yellow'} {selectedItem?.task_id === item.task_id ? 'selected' : ''}"
                on:click={() => selectItem(item)}
                on:keydown={(e) => e.key === 'Enter' && selectItem(item)}
                role="button"
                tabindex="0"
              >
                <div class="tile-header">
                  <div class="type-badge-col">
                    {#if item.issue_type === 'bug'}
                      <span class="type-icon type-bug">🪲 BUG</span>
                    {:else}
                      <span class="type-icon type-task">🚀 STORY</span>
                    {/if}
                  </div>
                  <span class="risk-label-mini {item.risk_level === 'critical' ? 'text-rose' : 'text-orange'} font-mono">
                    {item.risk_level === 'critical' ? '危急卡点' : '排期预警'}
                  </span>
                </div>

                {#if item.task_id}
                  {#if jiraBaseUrl && !item.task_id.startsWith('TASK-')}
                    <a href="{jiraBaseUrl}/browse/{item.task_id}" target="_blank" rel="noopener noreferrer" class="tile-project font-mono jira-id-link" on:click|stopPropagation>
                      🎫 {item.task_id}
                    </a>
                  {:else}
                    <span class="tile-project font-mono">🎫 {item.task_id}</span>
                  {/if}
                {/if}
                
                <h4 class="tile-title">{item.title}</h4>
                
                <div class="tile-footer font-mono">
                  <span class="assignee">👤 {item.assignee}</span>
                </div>
                {#if item.risk_level === 'critical'}
                  <div class="tile-pulse-glow"></div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <!-- Bento 4: Human Override Console (1.5 Rows / 2 Columns) -->
    <div class="bento-card bento-override glass-panel">
      {#if selectedItem}
        <div class="override-panel-layout">
          <!-- Left Col: Telemetry detail & AI Diagnosis -->
          <div class="override-info">
            <div class="override-title-row">
              {#if jiraBaseUrl && !selectedItem.task_id.startsWith('TASK-')}
                <a href="{jiraBaseUrl}/browse/{selectedItem.task_id}" target="_blank" rel="noopener noreferrer" class="task-id-badge font-mono jira-id-link">
                  {selectedItem.task_id}
                </a>
              {:else}
                <span class="task-id-badge font-mono">{selectedItem.task_id}</span>
              {/if}
              {#if selectedItem.repo && selectedItem.repo !== '-'}
                <span class="project-tag font-mono" title={selectedItem.repo}>📁 {selectedItem.repo}</span>
              {/if}
              <span class="type-badge {selectedItem.issue_type === 'bug' ? 'badge-bug' : 'badge-task'}">
                {selectedItem.issue_type === 'bug' ? '缺陷修复' : '功能需求'}
              </span>
            </div>
            
            <h3 class="override-title">{selectedItem.title}</h3>
            
            <div class="git-telemetry-timeline font-mono">
              <div class="timeline-meta-status">
                <span>流转状态: <code>{selectedItem.status.toLowerCase() === 'progress' && selectedItem.issue_type === 'bug' ? 'INVESTIGATION' : selectedItem.status.toUpperCase()}</code></span>
                {#if selectedItem.repo && selectedItem.repo !== '-'}
                  <span>当前追踪仓库: <code>{selectedItem.repo}</code></span>
                {/if}
              </div>

              {#if !selectedItem.git_logs || selectedItem.git_logs.length === 0}
                <div class="git-empty-alert">
                  <span class="icon">⚠️</span>
                  <div class="alert-content">
                    <strong>未检测到实际代码提交轨迹</strong>
                    <p>代码仓库中尚未检测到该任务的提交记录。AI 怀疑开发分支入口缺失，或开发工作尚未正式开始。</p>
                  </div>
                </div>
              {:else}
                <div class="git-timeline-list">
                  <span class="timeline-title-mini">📁 关联开发轨迹（最新最多10条记录）：</span>
                  {#each selectedItem.git_logs as log}
                    <div class="git-timeline-item action-{log.action}">
                      <div class="item-meta">
                        <span class="action-badge">{log.action.replace('git_', '').replace('mr_', '').toUpperCase()}</span>
                        <span class="repo-tag" title={log.repo}>{log.repo}</span>
                        <span class="branch-tag" title={log.branch}>{log.branch}</span>
                        {#if log.commit_id}
                          <span class="commit-hash"><code>{log.commit_id.slice(0, 7)}</code></span>
                        {/if}
                        <span class="author-tag">@{log.author}</span>
                        <span class="time-tag">{new Date(log.created_at).toLocaleTimeString('zh-CN', {hour: '2-digit', minute:'2-digit'})}</span>
                      </div>
                      <p class="commit-msg">{log.message}</p>
                      {#if log.mr_url}
                        <a href="{log.mr_url}" target="_blank" rel="noopener noreferrer" class="mr-action-link">
                          🔗 查看合并请求 !{log.mr_iid}
                        </a>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>

            <!-- AI Diagnosed Advice -->
            <div class="ai-diagnose-box">
              <p class="ai-msg">{getAiRecommendation(selectedItem)}</p>
              <span class="ai-reason font-mono">诊断归因: {selectedItem.desc}</span>
            </div>

            <!-- Dynamic AI Impact Analysis Simulation -->
            <div class="impact-box font-mono">
              <span class="impact-title">⚠️ AI 影响链条仿真 (Impact Simulation)</span>
              <p class="impact-desc">{aiPlan.impact}</p>
            </div>

            <div class="brain-flow-box">
              <div class="brain-flow-head">
                <span class="font-mono">最强大脑流转建议</span>
                <strong>{selectedItem.risk_type === 'none' ? '静默托管' : '需要干预'}</strong>
              </div>
              <div class="brain-flow-grid font-mono">
                {#each getBrainFlowSignals(selectedItem) as signal}
                  <div class="brain-signal signal-{signal.tone}">
                    <span>{signal.label}</span>
                    <strong>{signal.value}</strong>
                  </div>
                {/each}
              </div>
              <p>{getBrainFlowSuggestion(selectedItem)}</p>
            </div>
          </div>

          <!-- Right Col: Intervention Form -->
          <div class="override-controls">
            <div class="controls-header">
              <div class="header-main">
                <h4>⚡ 人工调停与干预 (Override)</h4>
                <button class="btn-quick-ai font-mono" on:click={applyAiPlan}>
                  💡 一键导入 AI 调停预案
                </button>
              </div>
              <p class="subtitle">对此任务进行调停决策，您的指令将覆盖 AI 的后台静默决策并自动同步</p>
            </div>

            <div class="override-decision-strip font-mono">
              <div>
                <span>当前负责人</span>
                <strong>{selectedItem.assignee || '-'}</strong>
              </div>
              <div>
                <span>当前截止</span>
                <strong>{selectedItem.due_date ? formatDateLabel(selectedItem.due_date.slice(0, 10)) : '未设置'}</strong>
              </div>
              <div>
                <span>风险等级</span>
                <strong class="risk-{selectedItem.risk_level}">{selectedItem.risk_level.toUpperCase()}</strong>
              </div>
            </div>

            <div class="override-form font-mono">
              <div class="input-row">
                <div class="input-field">
                  <label for="assignee-val">指派干预人</label>
                  <div class="override-select-container" bind:this={overrideAssigneeSelectEl}>
                    <button
                      id="assignee-val"
                      type="button"
                      class="override-select-trigger {newAssignee ? 'has-value' : ''}"
                      on:click={toggleOverrideAssigneeDropdown}
                      aria-expanded={showOverrideAssigneeDropdown}
                    >
                      <span>{newAssignee || '选择干预负责人'}</span>
                      <span class="select-arrow">{showOverrideAssigneeDropdown ? '▲' : '▼'}</span>
                    </button>
                    {#if showOverrideAssigneeDropdown}
                      <div class="override-select-options">
                        {#each overrideAssigneeOptions as name}
                          <button
                            type="button"
                            class="override-select-option {newAssignee === name ? 'active' : ''}"
                            on:click={() => selectOverrideAssignee(name)}
                          >
                            {name}
                          </button>
                        {/each}
                      </div>
                    {/if}
                  </div>
                </div>
                <div class="input-field">
                  <label for="due-val">延期截止日</label>
                  <div class="override-date-shell" bind:this={overrideDatePickerEl}>
                    <button
                      id="due-val"
                      type="button"
                      class="override-date-display {newDueDate ? 'has-value' : ''}"
                      on:click|stopPropagation={toggleOverrideDatePicker}
                      aria-expanded={showOverrideDatePicker}
                    >
                      <span>{newDueDateDisplay || '选择截止日期'}</span>
                      <span class="date-input-icon"></span>
                    </button>
                    {#if showOverrideDatePicker}
                      {@const calendar = getOverrideCalendarDays(newDueDate)}
                      <div class="override-date-picker-panel">
                        <div class="override-date-picker-head">
                          <button type="button" aria-label="上个月" on:click={() => moveOverrideDateMonth(-1)}>‹</button>
                          <strong>{calendar.label}</strong>
                          <button type="button" aria-label="下个月" on:click={() => moveOverrideDateMonth(1)}>›</button>
                        </div>
                        <div class="override-date-week-grid font-mono">
                          {#each weekdayNames as day}
                            <span>{day}</span>
                          {/each}
                        </div>
                        <div class="override-date-grid">
                          {#each calendar.days as day}
                            <button
                              type="button"
                              class="override-date-cell {day.muted ? 'muted' : ''} {day.today ? 'today' : ''} {day.selected ? 'selected' : ''}"
                              on:click={() => selectOverrideDueDate(day.value)}
                            >
                              {day.label}
                            </button>
                          {/each}
                        </div>
                        <div class="override-date-picker-foot">
                          <button type="button" on:click={() => selectOverrideDueDate(toDateValue(new Date()))}>今天</button>
                          <button type="button" on:click={clearOverrideDueDate}>清空</button>
                        </div>
                      </div>
                    {/if}
                  </div>
                </div>
              </div>
              <div class="input-field full-width">
                <label for="note-val">会议干预备注 (将写回 Jira 与飞书)</label>
                <input id="note-val" type="text" bind:value={decisionNote} placeholder="如：会中决定转派协助并顺延周期..." />
              </div>
              <div class="input-field">
                <label for="operator-val">调停决策人</label>
                <input id="operator-val" type="text" bind:value={operator} />
              </div>
            </div>

            <div class="override-actions">
              <button class="override-btn btn-primary" on:click={() => submitDecision('reassign')} disabled={decisionLoading}>
                👥 强制指派
              </button>
              <button class="override-btn btn-warning" on:click={() => submitDecision('reschedule')} disabled={decisionLoading}>
                📅 调整截止
              </button>
              <button class="override-btn btn-secondary" on:click={() => submitDecision('suspend')} disabled={decisionLoading}>
                ⏸️ 挂起任务
              </button>
            </div>

            {#if decisionSuccess}
              <div class="alert-box alert-success font-mono">{decisionSuccess}</div>
            {/if}
            {#if decisionError}
              <div class="alert-box alert-danger font-mono">{decisionError}</div>
            {/if}
          </div>
        </div>

        <!-- Timeline Log of Past Human Interventions -->
        <div class="override-history">
          <span class="eyebrow font-mono">PAST HUMAN INTERVENTIONS</span>
          <div class="logs-feed font-mono">
            {#if selectedItem.decision_logs}
              {#each selectedItem.decision_logs.split('\n') as logLine}
                {#if logLine.trim()}
                  <div class="log-item">
                    <span class="indicator-icon">👥</span>
                    <p>{logLine}</p>
                  </div>
                {/if}
              {/each}
            {:else}
              <div class="empty-logs">该卡点目前完全处于 AI 静默流转状态，尚无人工干预记录</div>
            {/if}
          </div>
        </div>
      {:else}
        <div class="override-panel-layout centered font-mono">
          <div class="empty-panel-msg">
            <span class="icon">⇅</span>
            <p>请在上方诊断盘中选择异常卡点进行人工调停 (Human Override)</p>
          </div>
        </div>
      {/if}
    </div>

  </div>

  <!-- Bottom Session decisions feed -->
  <div class="session-feed glass-panel">
    <div class="feed-header font-mono">
      <span>📢 本次会议决策广播 (LIVE FEED)</span>
    </div>
    <div class="feed-list font-mono">
      {#if localDecisions.length === 0}
        <div class="empty-feed">等待会中决策动作触发...</div>
      {:else}
        {#each localDecisions as feed}
          <div class="feed-item">
            <span class="badge">INTERVENTION</span>
            <p>{feed}</p>
          </div>
        {/each}
      {/if}
    </div>
  </div>

</div>

<style>
  /* 自动流转终端元信息与标签样式 */
  .terminal-meta-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 2px;
  }
  .terminal-repo {
    color: #38bdf8;
    background: rgba(56, 189, 248, 0.08);
    border: 1px solid rgba(56, 189, 248, 0.2);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.65rem;
  }
  .terminal-branch {
    color: #a78bfa;
    background: rgba(167, 139, 250, 0.08);
    border: 1px solid rgba(167, 139, 250, 0.2);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.65rem;
  }

  /* Git Telemetry Timeline 样式 */
  .timeline-meta-status {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
    margin-bottom: 12px;
    color: #94a3b8;
    font-size: 0.75rem;
  }
  .timeline-meta-status code {
    color: #38bdf8;
  }
  .git-empty-alert {
    background: rgba(244, 63, 94, 0.06);
    border: 1px dashed rgba(244, 63, 94, 0.35);
    border-radius: 8px;
    padding: 12px 16px;
    display: flex;
    gap: 12px;
    align-items: flex-start;
  }
  .git-empty-alert .icon {
    font-size: 1.1rem;
    line-height: 1;
  }
  .git-empty-alert .alert-content strong {
    display: block;
    color: #f43f5e;
    font-size: 0.75rem;
    margin-bottom: 4px;
  }
  .git-empty-alert .alert-content p {
    margin: 0;
    font-size: 0.7rem;
    color: #94a3b8;
    line-height: 1.4;
  }
  .git-timeline-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-height: 180px;
    overflow-y: auto;
    background: rgba(2, 6, 23, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 8px;
    padding: 12px;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.2) transparent;
  }
  .git-timeline-list::-webkit-scrollbar {
    width: 4px;
  }
  .git-timeline-list::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 2px;
  }
  .timeline-title-mini {
    font-size: 0.7rem;
    color: #64748b;
    font-weight: 700;
  }
  .git-timeline-item {
    background: rgba(15, 23, 42, 0.4);
    border-left: 3px solid #64748b;
    border-radius: 4px;
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .git-timeline-item.action-git_push { border-left-color: #3b82f6; }
  .git-timeline-item.action-mr_open { border-left-color: #a855f7; }
  .git-timeline-item.action-mr_merge { border-left-color: #10b981; }
  .git-timeline-item.action-ai_review { border-left-color: #06b6d4; background: rgba(6, 182, 212, 0.04); }
  
  .git-timeline-item .item-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    font-size: 0.65rem;
  }
  .action-badge {
    font-size: 0.55rem;
    font-weight: 800;
    padding: 1px 4px;
    border-radius: 3px;
    background: #475569;
    color: #cbd5e1;
  }
  .action-git_push .action-badge { background: rgba(59, 130, 246, 0.15); color: #60a5fa; }
  .action-mr_open .action-badge { background: rgba(168, 85, 247, 0.15); color: #c084fc; }
  .action-mr_merge .action-badge { background: rgba(16, 185, 129, 0.15); color: #34d399; }
  .action-ai_review .action-badge { background: rgba(6, 182, 212, 0.15); color: #22d3ee; }

  .repo-tag {
    color: #38bdf8;
    max-width: 100px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .branch-tag {
    color: #a78bfa;
    max-width: 100px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .commit-hash {
    color: #cbd5e1;
    background: rgba(255, 255, 255, 0.05);
    padding: 0px 4px;
    border-radius: 3px;
  }
  .author-tag {
    color: #e2e8f0;
  }
  .time-tag {
    color: #475569;
    margin-left: auto;
  }
  .commit-msg {
    margin: 0;
    font-size: 0.7rem;
    color: #cbd5e1;
    line-height: 1.4;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .mr-action-link {
    font-size: 0.65rem;
    color: #c084fc;
    text-decoration: none;
    align-self: flex-start;
    border-bottom: 1px dashed rgba(192, 132, 252, 0.4);
    padding-bottom: 1px;
    transition: all 0.2s;
  }
  .mr-action-link:hover {
    color: #d8b4fe;
    border-bottom-color: #d8b4fe;
  }

  .tile-project {
    font-size: 0.72rem;
    font-weight: 700;
    color: #34d399;
    background: rgba(16, 185, 129, 0.08);
    border: 1px solid rgba(16, 185, 129, 0.2);
    padding: 2px 6px;
    border-radius: 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    display: inline-block;
    align-self: flex-start;
  }

  .project-tag {
    font-size: 0.8rem;
    font-weight: 700;
    background: rgba(16, 185, 129, 0.15);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #34d399;
    padding: 2px 8px;
    border-radius: 4px;
    max-width: 240px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .decision-war-room {
    display: flex;
    flex-direction: column;
    gap: 24px;
    margin-top: 10px;
    color: #e2e8f0;
  }

  .strongest-brain-queue {
    display: flex;
    flex-direction: column;
    gap: 16px;
    overflow: visible;
  }

  .brain-queue-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 18px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.24);
    padding-bottom: 12px;
  }

  .brain-queue-header h2 {
    margin: 0;
    color: #f8fafc;
    font-size: 1.2rem;
    font-weight: 850;
  }

  .brain-queue-header p {
    margin: 5px 0 0 0;
    max-width: 720px;
    color: #94a3b8;
    font-size: 0.78rem;
    line-height: 1.5;
  }

  .brain-source-stack {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 6px;
    flex-shrink: 0;
  }

  .brain-source-chip {
    border: 1px solid rgba(71, 85, 105, 0.58);
    background: rgba(2, 6, 23, 0.54);
    color: #94a3b8;
    border-radius: 7px;
    padding: 6px 9px;
    font-size: 0.66rem;
    font-weight: 800;
    white-space: nowrap;
  }

  .brain-source-chip.live {
    border-color: rgba(16, 185, 129, 0.34);
    color: #34d399;
    background: rgba(16, 185, 129, 0.08);
  }

  .brain-source-chip.fallback {
    border-color: rgba(245, 158, 11, 0.28);
    color: #fbbf24;
    background: rgba(120, 53, 15, 0.12);
  }

  .brain-source-note {
    color: #64748b;
    font-size: 0.64rem;
    text-align: right;
  }

  .brain-summary-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .brain-summary-tile {
    min-width: 0;
    min-height: 112px;
    border: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(2, 6, 23, 0.32);
    border-radius: 10px;
    padding: 12px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-rows: auto auto minmax(0, 1fr);
    gap: 4px 10px;
  }

  .brain-summary-tile.summary-danger { border-color: rgba(244, 63, 94, 0.34); background: rgba(127, 29, 29, 0.12); }
  .brain-summary-tile.summary-warn { border-color: rgba(245, 158, 11, 0.3); background: rgba(120, 53, 15, 0.1); }
  .brain-summary-tile.summary-safe { border-color: rgba(16, 185, 129, 0.28); background: rgba(6, 78, 59, 0.12); }
  .brain-summary-tile.summary-info { border-color: rgba(56, 189, 248, 0.26); background: rgba(8, 47, 73, 0.12); }

  .summary-scope {
    color: #94a3b8;
    font-size: 0.64rem;
    font-weight: 900;
  }

  .brain-summary-tile strong {
    grid-column: 2;
    grid-row: 1 / span 2;
    color: #f8fafc;
    font-size: 1.45rem;
    line-height: 1;
    align-self: start;
  }

  .summary-label {
    color: #64748b;
    font-size: 0.68rem;
    font-weight: 800;
  }

  .brain-summary-tile p {
    grid-column: 1 / -1;
    margin: 4px 0 0 0;
    color: #cbd5e1;
    font-size: 0.74rem;
    line-height: 1.45;
  }

  .brain-meaning-panel {
    display: grid;
    grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr) auto;
    gap: 12px;
    align-items: stretch;
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-left-width: 4px;
    background: rgba(2, 6, 23, 0.34);
    border-radius: 8px;
    padding: 12px;
  }

  .brain-meaning-panel.risk-critical {
    border-left-color: #f43f5e;
    background: linear-gradient(90deg, rgba(127, 29, 29, 0.18), rgba(2, 6, 23, 0.34));
  }

  .brain-meaning-panel.risk-warning {
    border-left-color: #f59e0b;
    background: linear-gradient(90deg, rgba(120, 53, 15, 0.14), rgba(2, 6, 23, 0.34));
  }

  .brain-meaning-panel.risk-safe {
    border-left-color: #10b981;
  }

  .meaning-focus,
  .meaning-action {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .meaning-label {
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 900;
  }

  .meaning-focus strong,
  .meaning-action strong {
    color: #f8fafc;
    font-size: 0.86rem;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .meaning-focus p,
  .meaning-action p {
    margin: 0;
    color: #cbd5e1;
    font-size: 0.74rem;
    line-height: 1.45;
  }

  .meaning-source-board {
    min-width: 168px;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px;
    align-self: stretch;
  }

  .meaning-source-board span {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 28px;
    border: 1px solid rgba(71, 85, 105, 0.42);
    border-radius: 6px;
    background: rgba(15, 23, 42, 0.42);
    color: #94a3b8;
    font-size: 0.62rem;
    font-weight: 900;
    white-space: nowrap;
  }

  .brain-loading-grid,
  .decision-queue-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(220px, 1fr));
    gap: 12px;
  }

  .brain-skeleton {
    height: 228px;
    border-radius: 10px;
    border: 1px solid rgba(51, 65, 85, 0.36);
    background:
      linear-gradient(90deg, transparent, rgba(148, 163, 184, 0.08), transparent),
      rgba(2, 6, 23, 0.38);
    background-size: 180% 100%;
    animation: queueSkeleton 1.4s ease-in-out infinite;
  }

  @keyframes queueSkeleton {
    0% { background-position: 120% 0; }
    100% { background-position: -120% 0; }
  }

  .brain-empty-state {
    border: 1px dashed rgba(71, 85, 105, 0.58);
    background: rgba(2, 6, 23, 0.32);
    color: #64748b;
    border-radius: 10px;
    padding: 18px;
    text-align: center;
    font-size: 0.74rem;
  }

  .decision-queue-card {
    min-width: 0;
    min-height: 322px;
    border: 1px solid rgba(51, 65, 85, 0.42);
    border-top-width: 3px;
    background: rgba(2, 6, 23, 0.36);
    border-radius: 10px;
    padding: 12px;
    display: grid;
    grid-template-rows: auto auto minmax(38px, auto) minmax(60px, auto) minmax(54px, auto) minmax(42px, auto) auto auto;
    gap: 9px;
    cursor: pointer;
    outline: none;
    transition: border-color 0.18s ease, background 0.18s ease, transform 0.18s ease;
  }

  .decision-queue-card.risk-critical { border-top-color: #f43f5e; }
  .decision-queue-card.risk-warning { border-top-color: #f59e0b; }
  .decision-queue-card.risk-safe { border-top-color: #10b981; }

  .decision-queue-card:hover,
  .decision-queue-card:focus,
  .decision-queue-card.selected {
    background: rgba(15, 23, 42, 0.56);
    border-color: rgba(129, 140, 248, 0.45);
    transform: translateY(-1px);
  }

  .decision-card-top,
  .decision-card-bottom,
  .decision-card-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
  }

  .queue-risk,
  .queue-status,
  .queue-task-id,
  .queue-target {
    min-width: 0;
    border-radius: 5px;
    padding: 3px 6px;
    font-size: 0.62rem;
    font-weight: 900;
    white-space: nowrap;
  }

  .queue-risk {
    color: #f8fafc;
    background: rgba(71, 85, 105, 0.34);
  }

  .queue-status.status-open { color: #fbbf24; background: rgba(245, 158, 11, 0.12); }
  .queue-status.status-processing { color: #7dd3fc; background: rgba(14, 165, 233, 0.12); }
  .queue-status.status-decided { color: #34d399; background: rgba(16, 185, 129, 0.12); }
  .queue-status.status-ignored { color: #94a3b8; background: rgba(100, 116, 139, 0.12); }
  .queue-status.status-meeting { color: #c4b5fd; background: rgba(139, 92, 246, 0.14); }

  .decision-card-title-row {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .decision-card-id-row {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .queue-task-id {
    align-self: flex-start;
    color: #34d399;
    background: rgba(16, 185, 129, 0.1);
    border: 1px solid rgba(16, 185, 129, 0.18);
  }

  .queue-source {
    color: #93c5fd;
    background: rgba(14, 165, 233, 0.1);
    border: 1px solid rgba(14, 165, 233, 0.18);
    border-radius: 5px;
    padding: 3px 6px;
    font-size: 0.62rem;
    font-weight: 900;
    white-space: nowrap;
  }

  .decision-card-title-row h3 {
    margin: 0;
    color: #f1f5f9;
    font-size: 0.86rem;
    line-height: 1.35;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .decision-card-section {
    min-width: 0;
  }

  .decision-card-plan {
    min-width: 0;
    border: 1px solid rgba(71, 85, 105, 0.32);
    border-radius: 7px;
    background: rgba(15, 23, 42, 0.4);
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .decision-card-plan div {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .decision-card-plan span,
  .decision-card-section span {
    display: block;
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 900;
    margin-bottom: 3px;
  }

  .decision-card-plan strong {
    color: #f8fafc;
    font-size: 0.72rem;
    font-weight: 900;
    white-space: nowrap;
  }

  .decision-card-plan p,
  .decision-card-section p,
  .decision-card-section li {
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.42;
    margin: 0;
    overflow-wrap: anywhere;
  }

  .decision-card-section p {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .decision-card-plan p {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .decision-card-section ul {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin: 0;
    padding: 0 0 0 13px;
  }

  .decision-card-bottom {
    color: #64748b;
    font-size: 0.66rem;
    border-top: 1px solid rgba(51, 65, 85, 0.26);
    padding-top: 8px;
  }

  .decision-card-bottom span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .decision-card-actions a,
  .queue-target {
    color: #93c5fd;
    text-decoration: none;
    font-size: 0.68rem;
  }

  .decision-card-actions a:hover {
    color: #bfdbfe;
    text-decoration: underline;
  }

  /* Bento Grid Layout (3 Columns, 3 Rows equivalent height) */
  .bento-grid {
    display: grid;
    grid-template-columns: 350px 1fr 1fr;
    grid-template-rows: auto auto;
    gap: 24px;
  }

  .bento-card {
    display: flex;
    flex-direction: column;
  }

  /* Bento Tiles Sizing */
  .bento-metrics {
    grid-column: 1;
    grid-row: 1;
    min-height: 280px;
  }

  .bento-terminal {
    grid-column: 1;
    grid-row: 2;
    min-height: 440px;
  }

  .bento-agenda {
    grid-column: 2 / span 2;
    grid-row: 1;
    min-height: 280px;
    position: relative;
    z-index: 10;
  }

  .bento-override {
    grid-column: 2 / span 2;
    grid-row: 2;
    min-height: 440px;
    overflow: visible;
    position: relative;
    z-index: 12;
  }

  /* Premium Glassmorphism cards */
  .glass-panel {
    background: rgba(10, 15, 30, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 16px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    padding: 24px;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.5);
    transition: border-color 0.3s;
  }

  .glass-panel:hover {
    border-color: rgba(99, 102, 241, 0.25);
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

  .card-header-mini h3 {
    margin: 0 0 16px 0;
    font-size: 1rem;
    font-weight: 700;
    color: #f1f5f9;
  }

  /* Metrics Panel Styling */
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
    margin-bottom: 20px;
  }

  .metric-item {
    background: rgba(2, 6, 23, 0.4);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .border-blue-dim { border-left: 3px solid #3b82f6; }
  .border-rose-dim { border-left: 3px solid #f43f5e; }
  .border-purple-dim { border-left: 3px solid #8b5cf6; }
  .border-green-dim { border-left: 3px solid #10b981; }

  .metric-label {
    font-size: 0.7rem;
    color: #64748b;
    font-weight: 600;
  }

  .metric-value-row {
    display: flex;
    align-items: baseline;
    gap: 4px;
  }

  .metric-val {
    font-size: 1.5rem;
    font-weight: 800;
  }

  .text-blue { color: #3b82f6; }
  .text-rose { color: #f43f5e; }
  .text-orange { color: #f59e0b; }
  .text-green { color: #10b981; }

  .unit {
    font-size: 0.7rem;
    color: #475569;
  }

  .overall-progress {
    display: flex;
    flex-direction: column;
    gap: 8px;
    border-top: 1px solid rgba(51, 65, 85, 0.2);
    padding-top: 16px;
  }

  .progress-labels {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: #94a3b8;
  }

  .progress-bar-bg {
    height: 6px;
    background: rgba(30, 41, 59, 0.6);
    border-radius: 9999px;
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    background: linear-gradient(90deg, #10b981 0%, #06b6d4 100%);
    border-radius: 9999px;
  }

  /* Telemetry Terminal Panel */
  .bento-terminal {
    position: relative;
  }

  .bento-terminal .pulse-indicator {
    position: absolute;
    top: 24px;
    right: 24px;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #10b981;
    box-shadow: 0 0 8px #10b981;
    animation: pulse 2s infinite;
  }

  .terminal-container {
    background: #020617;
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 10px;
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .terminal-header {
    background: rgba(30, 41, 59, 0.4);
    padding: 6px 12px;
    font-size: 0.65rem;
    color: #64748b;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
  }

  .terminal-body {
    padding: 12px;
    font-size: 0.7rem;
    line-height: 1.5;
    color: #34d399;
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
    max-height: 360px;
    scrollbar-width: thin;
    scrollbar-color: rgba(16, 185, 129, 0.2) transparent;
  }

  .terminal-body::-webkit-scrollbar {
    width: 4px;
  }
  .terminal-body::-webkit-scrollbar-thumb {
    background: rgba(16, 185, 129, 0.2);
    border-radius: 2px;
  }

  .terminal-line {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .terminal-line .time {
    color: #475569;
  }

  .terminal-line .task-link {
    color: #818cf8;
    font-weight: 700;
  }

  .terminal-line .msg {
    margin: 0;
    color: #10b981;
    word-break: break-all;
  }

  .blink-line .cursor {
    animation: blink 1s infinite;
    color: #10b981;
  }

  /* Risk Diagnostics Panel (Bento 3) - Flattened Grid Vertical Scroll */
  .panel-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
    margin-bottom: 18px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.15);
    padding-bottom: 12px;
  }

  .panel-header-row h2 {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 800;
  }

  .filter-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    align-items: center;
  }

  .filter-group {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .filter-label-inline {
    font-size: 0.65rem;
    font-weight: 700;
    color: #475569;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .filter-tabs {
    display: flex;
    background: rgba(2, 6, 23, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 8px;
    padding: 3px;
    gap: 4px;
  }

  .filter-btn {
    background: transparent;
    border: none;
    color: #64748b;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.7rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s;
  }

  .filter-btn:hover {
    color: #cbd5e1;
  }

  .filter-btn.active {
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
  }

  .agenda-scroll-container {
    flex-grow: 1;
    overflow-y: auto;
    max-height: 200px;
    padding-right: 8px;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.2) transparent;
  }

  .agenda-scroll-container::-webkit-scrollbar {
    width: 4px;
  }
  .agenda-scroll-container::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 4px;
  }
  .agenda-scroll-container::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.45);
  }

  .agenda-items-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
    gap: 16px;
    padding: 4px 2px;
  }

  .agenda-tile-card {
    background: rgba(30, 41, 59, 0.25);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 16px;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 12px;
    text-align: left;
    position: relative;
    overflow: hidden;
    transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .tile-red { border-top: 4px solid #ef4444; }
  .tile-yellow { border-top: 4px solid #f59e0b; }

  .agenda-tile-card:hover {
    background: rgba(51, 65, 85, 0.3);
    transform: translateY(-2px);
    border-color: rgba(99, 102, 241, 0.35);
  }

  .agenda-tile-card.selected,
  .agenda-tile-card:focus {
    background: rgba(99, 102, 241, 0.1);
    border-color: #6366f1;
    box-shadow: 0 0 16px rgba(99, 102, 241, 0.25);
    outline: none;
  }

  .tile-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .type-icon {
    font-size: 0.65rem;
    font-weight: 800;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .type-bug { background: rgba(239, 68, 68, 0.15); color: #f87171; }
  .type-task { background: rgba(99, 102, 241, 0.15); color: #818cf8; }

  .risk-label-mini {
    font-size: 0.65rem;
    font-weight: 700;
  }

  .tile-title {
    font-size: 0.8rem;
    margin: 0;
    color: #e2e8f0;
    font-weight: 600;
    line-height: 1.4;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    height: 2.8em;
  }

  .tile-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.7rem;
    color: #64748b;
    border-top: 1px solid rgba(51, 65, 85, 0.2);
    padding-top: 8px;
  }

  .tile-footer .assignee {
    color: #cbd5e1;
  }

  .tile-pulse-glow {
    position: absolute;
    top: 0;
    right: 0;
    width: 60px;
    height: 60px;
    background: radial-gradient(circle, rgba(239, 68, 68, 0.1) 0%, transparent 70%);
    pointer-events: none;
  }

  /* Human Override Panel (Bento 4) */
  .override-panel-layout {
    display: grid;
    grid-template-columns: 1.1fr 0.9fr;
    gap: 24px;
  }

  .override-panel-layout.centered {
    height: 300px;
    display: flex;
    justify-content: center;
    align-items: center;
    color: #475569;
  }

  .empty-panel-msg {
    text-align: center;
  }
  .empty-panel-msg .icon {
    font-size: 3rem;
    display: block;
    margin-bottom: 12px;
  }

  .override-info {
    display: flex;
    flex-direction: column;
    gap: 12px;
    border-right: 1px solid rgba(51, 65, 85, 0.25);
    padding-right: 24px;
  }

  .override-title-row {
    display: flex;
    gap: 10px;
    align-items: center;
  }

  .task-id-badge {
    font-size: 0.8rem;
    font-weight: 800;
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.3);
    color: #818cf8;
    padding: 2px 8px;
    border-radius: 4px;
    text-decoration: none;
    display: inline-block;
  }

  a.task-id-badge:hover {
    background: rgba(99, 102, 241, 0.25);
    border-color: rgba(99, 102, 241, 0.5);
  }

  .jira-id-link {
    text-decoration: none;
    transition: all 0.2s;
  }

  .tile-project.jira-id-link:hover {
    background: rgba(16, 185, 129, 0.15);
    border-color: rgba(16, 185, 129, 0.4);
    transform: translateY(-1px);
  }

  .type-badge {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 4px;
  }
  .badge-bug { background: rgba(244, 63, 94, 0.1); color: #f43f5e; border: 1px solid rgba(244, 63, 94, 0.2); }
  .badge-task { background: rgba(6, 182, 212, 0.1); color: #06b6d4; border: 1px solid rgba(6, 182, 212, 0.2); }

  .override-title {
    margin: 0;
    font-size: 0.95rem;
    color: #f1f5f9;
    line-height: 1.4;
  }

  .git-telemetry-timeline {
    background: rgba(2, 6, 23, 0.4);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 0.7rem;
    color: #64748b;
  }

  .timeline-meta {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .timeline-meta code {
    color: #cbd5e1;
  }

  .ai-diagnose-box {
    background: rgba(99, 102, 241, 0.08);
    border: 1px dashed rgba(99, 102, 241, 0.35);
    border-radius: 10px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .ai-diagnose-box .ai-msg {
    margin: 0;
    font-size: 0.8rem;
    color: #e2e8f0;
    line-height: 1.4;
  }

  .ai-diagnose-box .ai-reason {
    font-size: 0.7rem;
    color: #818cf8;
  }

  /* Impact simulation tile */
  .impact-box {
    background: rgba(245, 158, 11, 0.05);
    border: 1px solid rgba(245, 158, 11, 0.15);
    border-radius: 10px;
    padding: 14px;
  }

  .impact-title {
    font-size: 0.7rem;
    font-weight: 700;
    color: #f59e0b;
    display: block;
    margin-bottom: 4px;
  }

  .impact-desc {
    margin: 0;
    font-size: 0.75rem;
    color: #cbd5e1;
    line-height: 1.4;
  }

  .brain-flow-box {
    border: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(2, 6, 23, 0.26);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .brain-flow-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
  }

  .brain-flow-head span {
    color: #94a3b8;
    font-size: 0.7rem;
    font-weight: 800;
  }

  .brain-flow-head strong {
    color: #7dd3fc;
    font-size: 0.72rem;
  }

  .brain-flow-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 6px;
  }

  .brain-signal {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.38);
    background: rgba(15, 23, 42, 0.42);
    border-radius: 7px;
    padding: 7px 8px;
  }

  .brain-signal span {
    display: block;
    color: #64748b;
    font-size: 0.6rem;
    margin-bottom: 4px;
  }

  .brain-signal strong {
    display: block;
    color: #cbd5e1;
    font-size: 0.68rem;
    line-height: 1.3;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .brain-signal.signal-safe strong { color: #86efac; }
  .brain-signal.signal-warn strong { color: #fbbf24; }
  .brain-signal.signal-danger strong { color: #fb7185; }
  .brain-signal.signal-info strong { color: #93c5fd; }

  .brain-flow-box p {
    margin: 0;
    color: #cbd5e1;
    font-size: 0.74rem;
    line-height: 1.45;
  }

  /* Intervention Form Controls */
  .override-controls {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .controls-header {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .controls-header .header-main {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .controls-header h4 {
    margin: 0;
    font-size: 0.85rem;
    color: #cbd5e1;
  }

  .btn-quick-ai {
    background: rgba(99, 102, 241, 0.15);
    border: 1px solid rgba(99, 102, 241, 0.35);
    color: #a5b4fc;
    font-size: 0.65rem;
    font-weight: 700;
    padding: 3px 8px;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-quick-ai:hover {
    background: rgba(99, 102, 241, 0.25);
    border-color: #6366f1;
    transform: scale(0.97);
  }

  .controls-header .subtitle {
    font-size: 0.7rem;
    color: #64748b;
    margin: 0;
  }

  .override-decision-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .override-decision-strip div {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.55);
    background: rgba(2, 6, 23, 0.36);
    border-radius: 8px;
    padding: 8px 10px;
  }

  .override-decision-strip span {
    display: block;
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 800;
    margin-bottom: 4px;
  }

  .override-decision-strip strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.72rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .override-decision-strip .risk-critical {
    color: #fb7185;
  }

  .override-decision-strip .risk-warning {
    color: #f59e0b;
  }

  .override-decision-strip .risk-safe {
    color: #34d399;
  }

  .override-form {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .input-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .input-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .input-field label {
    font-size: 0.65rem;
    color: #64748b;
    font-weight: 700;
  }

  .input-field input {
    background: rgba(2, 6, 23, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.5);
    border-radius: 6px;
    padding: 6px 10px;
    color: #f1f5f9;
    font-size: 0.75rem;
    outline: none;
    transition: border-color 0.2s;
  }

  .input-field input:focus {
    border-color: #6366f1;
  }

  .override-select-container,
  .override-date-shell {
    position: relative;
    min-height: 32px;
  }

  .override-select-trigger,
  .override-date-display {
    width: 100%;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    box-sizing: border-box;
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.88), rgba(15, 23, 42, 0.62));
    border: 1px solid rgba(71, 85, 105, 0.62);
    border-radius: 6px;
    color: #64748b;
    padding: 6px 10px;
    font-family: inherit;
    font-size: 0.75rem;
    line-height: 1;
    text-align: left;
    cursor: pointer;
    outline: none;
    appearance: none;
    transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
  }

  .override-select-trigger.has-value,
  .override-date-display.has-value {
    color: #f1f5f9;
  }

  .override-select-trigger:hover,
  .override-select-trigger:focus,
  .override-date-shell:hover .override-date-display,
  .override-date-shell:focus-within .override-date-display {
    border-color: rgba(129, 140, 248, 0.72);
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(15, 23, 42, 0.72));
    box-shadow: 0 0 0 2px rgba(129, 140, 248, 0.16);
  }

  .override-select-options {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    right: 0;
    z-index: 120;
    max-height: 180px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: #0f172a;
    border: 1px solid rgba(129, 140, 248, 0.26);
    border-radius: 8px;
    padding: 4px;
    box-shadow: 0 18px 36px rgba(2, 6, 23, 0.58);
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.25) transparent;
  }

  .override-select-option {
    width: 100%;
    border: none;
    background: transparent;
    color: #cbd5e1;
    border-radius: 5px;
    padding: 7px 9px;
    font-family: inherit;
    font-size: 0.75rem;
    line-height: 1.35;
    text-align: left;
    cursor: pointer;
    word-break: break-word;
  }

  .override-select-option:hover {
    background: rgba(99, 102, 241, 0.14);
    color: #ffffff;
  }

  .override-select-option.active {
    background: rgba(99, 102, 241, 0.9);
    color: #ffffff;
    font-weight: 700;
  }

  .date-input-icon {
    position: relative;
    flex: 0 0 auto;
    box-sizing: border-box;
    width: 15px;
    height: 15px;
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

  .override-date-picker-panel {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    width: 292px;
    background: #0b1220;
    border: 1px solid rgba(71, 85, 105, 0.76);
    border-radius: 12px;
    padding: 12px;
    box-shadow: 0 20px 48px rgba(2, 6, 23, 0.72);
    z-index: 1500;
  }

  .override-date-picker-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 10px;
  }

  .override-date-picker-head strong {
    color: #e2e8f0;
    font-size: 0.86rem;
  }

  .override-date-picker-head button,
  .override-date-picker-foot button,
  .override-date-cell {
    border: 1px solid transparent;
    background: transparent;
    color: #94a3b8;
    cursor: pointer;
    font: inherit;
  }

  .override-date-picker-head button {
    width: 30px;
    height: 30px;
    border-radius: 8px;
    font-size: 1.15rem;
    line-height: 1;
    background: rgba(15, 23, 42, 0.78);
    border-color: rgba(51, 65, 85, 0.7);
  }

  .override-date-picker-head button:hover,
  .override-date-picker-foot button:hover {
    color: #e2e8f0;
    border-color: rgba(129, 140, 248, 0.55);
  }

  .override-date-week-grid,
  .override-date-grid {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    gap: 4px;
  }

  .override-date-week-grid {
    margin-bottom: 6px;
  }

  .override-date-week-grid span {
    color: #475569;
    font-size: 0.64rem;
    text-align: center;
    font-weight: 800;
  }

  .override-date-cell {
    height: 32px;
    border-radius: 8px;
    font-size: 0.76rem;
    font-variant-numeric: tabular-nums;
  }

  .override-date-cell:hover {
    color: #f8fafc;
    background: rgba(99, 102, 241, 0.16);
    border-color: rgba(129, 140, 248, 0.42);
  }

  .override-date-cell.muted {
    color: #334155;
  }

  .override-date-cell.today {
    border-color: rgba(14, 165, 233, 0.55);
    color: #7dd3fc;
  }

  .override-date-cell.selected {
    background: #4f46e5;
    color: #ffffff;
    border-color: rgba(129, 140, 248, 0.9);
  }

  .override-date-picker-foot {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    margin-top: 10px;
    border-top: 1px solid rgba(51, 65, 85, 0.48);
    padding-top: 10px;
  }

  .override-date-picker-foot button {
    border-color: rgba(51, 65, 85, 0.62);
    background: rgba(15, 23, 42, 0.58);
    border-radius: 7px;
    padding: 6px 9px;
    font-size: 0.72rem;
  }

  .override-actions {
    display: flex;
    gap: 10px;
    margin-top: 6px;
  }

  .override-btn {
    flex: 1;
    border: none;
    padding: 8px;
    font-size: 0.75rem;
    font-weight: 700;
    border-radius: 6px;
    cursor: pointer;
    transition: transform 0.1s, opacity 0.2s;
  }

  .override-btn:active {
    transform: scale(0.97);
  }

  .btn-primary { background: #4f46e5; color: white; }
  .btn-primary:hover { background: #4338ca; }
  
  .btn-warning { background: #0d9488; color: white; }
  .btn-warning:hover { background: #0f766e; }

  .btn-secondary { background: #334155; color: #cbd5e1; }
  .btn-secondary:hover { background: #1e293b; }

  .alert-box {
    padding: 6px 10px;
    border-radius: 6px;
    font-size: 0.7rem;
  }

  .alert-success { background: rgba(16, 185, 129, 0.12); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.25); }
  .alert-danger { background: rgba(239, 68, 68, 0.12); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.25); }

  /* Intervention logs timeline */
  .override-history {
    border-top: 1px solid rgba(51, 65, 85, 0.2);
    padding-top: 14px;
    margin-top: 14px;
  }

  .logs-feed {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 100px;
    overflow-y: auto;
    background: rgba(2, 6, 23, 0.3);
    border-radius: 8px;
    padding: 8px 12px;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.1) transparent;
  }

  .logs-feed::-webkit-scrollbar {
    width: 4px;
  }
  .logs-feed::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.15);
    border-radius: 2px;
  }

  .log-item {
    display: flex;
    gap: 8px;
    align-items: flex-start;
  }

  .log-item .indicator-icon {
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .log-item p {
    margin: 0;
    font-size: 0.7rem;
    color: #cbd5e1;
    line-height: 1.4;
  }

  .empty-logs {
    font-size: 0.7rem;
    color: #475569;
    text-align: center;
  }

  /* Bottom session feed */
  .session-feed {
    margin-top: 6px;
  }

  .feed-header {
    font-size: 0.75rem;
    color: #818cf8;
    font-weight: 700;
    margin-bottom: 10px;
  }

  .feed-list {
    background: #020617;
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 8px;
    padding: 12px;
    max-height: 100px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .feed-item {
    display: flex;
    gap: 10px;
    align-items: center;
    font-size: 0.7rem;
  }

  .feed-item .badge {
    background: #ef4444;
    color: white;
    font-weight: 800;
    font-size: 0.55rem;
    padding: 1px 4px;
    border-radius: 3px;
    flex-shrink: 0;
  }

  .feed-item p {
    margin: 0;
    color: #cbd5e1;
  }

  .empty-feed {
    font-size: 0.7rem;
    color: #475569;
    text-align: center;
  }

  .state-msg {
    padding: 40px;
    text-align: center;
    color: #475569;
    font-size: 0.8rem;
  }

  .safe-msg {
    color: #10b981;
  }

  .error-msg {
    color: #f87171;
  }

  @keyframes pulse {
    0%, 100% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.1); opacity: 0.6; }
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0; }
  }

  /* Custom Select Dropdowns */
  .custom-select-container {
    position: relative;
    display: inline-block;
    z-index: 20;
    width: 145px; /* 固定容器宽度以杜绝抖动 */
  }

  .custom-select-trigger {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
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
    width: 145px;
    box-sizing: border-box;
  }

  .custom-select-trigger:hover, .custom-select-trigger:focus {
    border-color: #6366f1;
    background-color: rgba(30, 41, 59, 0.8);
    box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.2);
  }

  .trigger-label {
    flex: 1;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .select-arrow {
    font-size: 0.6rem;
    color: #818cf8;
    transition: transform 0.2s;
    line-height: 1;
  }

  .custom-select-options {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    width: max-content; /* 自适应内容宽度，防止名字被截断 */
    min-width: 145px;
    max-width: 240px;
    background: #0f172a;
    border: 1px solid rgba(129, 140, 248, 0.25);
    border-radius: 8px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5), 0 0 15px rgba(99, 102, 241, 0.1);
    z-index: 100;
    overflow-y: auto;
    max-height: 160px; /* 限制最大高度，防止卡片外层溢出截断 */
    display: flex;
    flex-direction: column;
    padding: 4px;
    box-sizing: border-box;
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
    color: #cbd5e1;
    padding: 8px 12px;
    text-align: left;
    font-size: 0.8rem;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
    width: 100%;
    white-space: normal;
    overflow: visible;
    line-height: 1.4;
    word-break: keep-all;
  }

  .custom-option:hover {
    background: rgba(99, 102, 241, 0.15);
    color: #ffffff;
  }

  .custom-option.active {
    background: #6366f1;
    color: #ffffff;
    font-weight: 600;
  }

  @keyframes dropdownFadeIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
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

  @media (max-width: 1280px) {
    .bento-grid {
      grid-template-columns: 1fr;
    }

    .bento-metrics,
    .bento-terminal,
    .bento-agenda,
    .bento-override {
      grid-column: 1;
      grid-row: auto;
    }

    .decision-queue-grid,
    .brain-loading-grid {
      grid-template-columns: repeat(2, minmax(220px, 1fr));
    }

    .brain-meaning-panel {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    }

    .meaning-source-board {
      grid-column: 1 / -1;
      min-width: 0;
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .brain-queue-header,
    .panel-header-row,
    .override-panel-layout,
    .input-row,
    .controls-header .header-main {
      grid-template-columns: 1fr;
      flex-direction: column;
      align-items: stretch;
    }

    .brain-summary-grid,
    .brain-meaning-panel,
    .decision-queue-grid,
    .brain-loading-grid,
    .metrics-grid,
    .override-decision-strip,
    .brain-flow-grid {
      grid-template-columns: 1fr;
    }

    .meaning-source-board {
      grid-column: auto;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .brain-source-stack {
      align-items: flex-start;
    }

    .override-info {
      border-right: none;
      border-bottom: 1px solid rgba(51, 65, 85, 0.25);
      padding-right: 0;
      padding-bottom: 16px;
    }

    .filter-bar,
    .override-actions,
    .decision-card-bottom {
      flex-direction: column;
      align-items: stretch;
    }

    .custom-select-container,
    .custom-select-trigger {
      width: 100%;
    }
  }
</style>
