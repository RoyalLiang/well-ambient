<script lang="ts">
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import type { AdminMetric, AdminTableColumn, AdminTableRow, AdminTone } from '../lib/admin-console/contract';
  import { ADMIN_TONE_CLASS } from '../lib/admin-console/contract';

  let inputText = '';
  type IntentMode = 'intent' | 'summary';
  type IntentInsight = {
    intent: string;
    confidence: number;
    summary: string;
    missing_context: string[];
    next_questions: string[];
    suggested_action: string;
    endpoint: string;
    source: 'api' | 'local';
  };

  let isLoading = false;
  let hasResult = false;
  let isAIEnabled = false;
  let intentInputText = '';
  let intentMode: IntentMode = 'intent';
  let intentLoading = false;
  let intentError = '';
  let intentResult: IntentInsight | null = null;

  type DeconstructAnalysis = {
    completeness_score: number;
    overall_estimated_days: number;
    overall_estimated_hours: number;
    overall_difficulty: string;
    estimate_basis: string;
    missing_info: string[];
    risks: string[];
    dependencies: string[];
    acceptance_criteria: string[];
    schedule_notes: string[];
    meeting_questions: string[];
    confidence: number;
  };

  const emptyAnalysis = (): DeconstructAnalysis => ({
    completeness_score: 0,
    overall_estimated_days: 0,
    overall_estimated_hours: 0,
    overall_difficulty: 'Medium',
    estimate_basis: '',
    missing_info: [],
    risks: [],
    dependencies: [],
    acceptance_criteria: [],
    schedule_notes: [],
    meeting_questions: [],
    confidence: 0
  });

  type GeneratedTask = {
    id: string;
    repo: string;
    title: string;
    assignee: string;
    priority: string;
    complexity: string;
    difficulty: string;
    estimated_days: number;
    estimated_hours: number;
    estimate_basis: string;
    period_days: number;
  };

  const deconstructColumns: AdminTableColumn[] = [
    { key: 'task', label: '任务标题', width: '30%' },
    { key: 'repo', label: '代码库', width: '15%' },
    { key: 'owner', label: '负责人', width: '13%' },
    { key: 'estimate', label: '工时', width: '12%' },
    { key: 'priority', label: '优先级', width: '10%', align: 'center' },
    { key: 'difficulty', label: '难度', width: '10%', align: 'center' },
    { key: 'evidence', label: '检查项', width: '10%' }
  ];

  let result = {
    mappedRepos: [] as string[],
    tasks: [] as GeneratedTask[],
    analysis: emptyAnalysis(),
    context_pack_id: 0
  };

  let assigneesList: string[] = ['Eddie', 'Antigravity'];
  let currentTaskGroupId = '';
  let activeGeneratedTaskId = '';
  let selectedGeneratedTask: GeneratedTask | null = null;

  let isMockResponse = false;

  let isDragging = false;
  let uploadedFileName = '';
  let uploadedFileSize = 0;
  let showPromptSettings = false;
  let isImporting = false;

  let activeDemands: any[] = [];
  let selectedDemandId = '';
  let showDemandDropdown = false;
  let selectedDemandTitle = '选择要关联的产品需求 (可选)';

  $: if (hasResult && result.tasks.length > 0 && !result.tasks.some(task => task.id === activeGeneratedTaskId)) {
    activeGeneratedTaskId = result.tasks[0].id;
  }
  $: if (!hasResult && activeGeneratedTaskId) {
    activeGeneratedTaskId = '';
  }
  $: selectedGeneratedTask = result.tasks.find(task => task.id === activeGeneratedTaskId) || null;
  $: deconstructMetrics = [
    {
      label: 'AI 服务',
      value: isAIEnabled ? '启用' : '未启用',
      helper: isAIEnabled ? '可解构与识别意图' : '等待配置凭证',
      tone: isAIEnabled ? 'success' : 'danger'
    },
    {
      label: '可关联需求',
      value: activeDemands.length,
      helper: selectedDemandId ? selectedDemandTitle : '当前未绑定需求',
      tone: selectedDemandId ? 'info' : 'neutral'
    },
    {
      label: '任务证据',
      value: hasResult ? result.tasks.length : 0,
      helper: hasResult ? `${result.mappedRepos.length} 个代码库` : '尚未生成解构结果',
      tone: hasResult ? 'success' : 'neutral'
    },
    {
      label: '完整性',
      value: hasResult ? percentLabel(result.analysis.completeness_score) : '-',
      helper: hasResult ? `置信度 ${confidenceLabel(result.analysis.confidence)}` : '等待分析',
      tone: hasResult ? scoreTone(result.analysis.completeness_score) : 'neutral'
    }
  ] satisfies AdminMetric[];
  $: generatedTaskRows = result.tasks.map(buildGeneratedTaskRow) satisfies AdminTableRow[];

  function getDemandDisplayTitle(demand: any) {
    return `#${demand.task_id} - ${demand.title}`;
  }

  function createTaskGroupId(taskId: string) {
    const clean = taskId.replace(/[^A-Za-z0-9-]/g, '').toLowerCase();
    return clean ? `group-${clean}` : `group-${Date.now()}`;
  }

  function getDemandTaskGroupId(demand: any) {
    if (!demand || !demand.task_group_id || demand.task_group_id === '-') return '';
    return demand.task_group_id;
  }

  function getSelectedDemandTaskGroupId() {
    const selected = activeDemands.find((d: any) => d.task_id === selectedDemandId);
    return getDemandTaskGroupId(selected);
  }

  function selectDemandLink(demand: any | null) {
    if (!demand) {
      selectedDemandId = '';
      selectedDemandTitle = '不关联需求，仅同步任务';
      showDemandDropdown = false;
      return;
    }

    selectedDemandId = demand.task_id;
    selectedDemandTitle = getDemandDisplayTitle(demand);
    const existingGroupId = getDemandTaskGroupId(demand);
    if (existingGroupId) {
      currentTaskGroupId = existingGroupId;
    } else if (!currentTaskGroupId && hasResult) {
      currentTaskGroupId = createTaskGroupId(demand.task_id);
    }
    showDemandDropdown = false;
  }

  async function fetchActiveDemands() {
    try {
      const res = await fetch('/api/tasks');
      if (res.ok) {
        const data = await res.json();
        activeDemands = (data || []).filter((t: any) => t.issue_type === 'demand' && t.status !== 'archived');
      }
    } catch (e) {
      console.error('[Deconstructor] Failed to load active demands:', e);
    }
  }

  // 自定义下拉菜单状态控制
  let activeDropdown: { taskId: string; field: string } | null = null;

  function toggleDropdown(taskId: string, field: string) {
    if (activeDropdown && activeDropdown.taskId === taskId && activeDropdown.field === field) {
      activeDropdown = null;
    } else {
      activeDropdown = { taskId, field };
    }
  }

  function isDropdownOpen(taskId: string, field: string) {
    return activeDropdown && activeDropdown.taskId === taskId && activeDropdown.field === field;
  }

  function selectDropdownValue(taskId: string, field: string, value: any) {
    result.tasks = result.tasks.map(t => {
      if (t.id === taskId) {
        if (field === 'difficulty') {
          return { ...t, difficulty: value, complexity: value };
        }
        return { ...t, [field]: value };
      }
      return t;
    });
    activeDropdown = null;
  }

  function getHoursLabel(hours: number) {
    const normalized = Number.isFinite(hours) && hours > 0 ? Math.round(hours * 10) / 10 : 0;
    if (normalized <= 0) return '待估算';
    const dayEquivalent = Math.round((normalized / 8) * 10) / 10;
    if (normalized === 2) return '2 小时 (微调)';
    if (normalized === 4) return '4 小时 (半天)';
    if (normalized === 8) return '8 小时 (1 天)';
    if (normalized === 16) return '16 小时 (2 天)';
    if (normalized === 24) return '24 小时 (3 天)';
    if (normalized === 40) return '40 小时 (常规一周)';
    if (normalized % 8 === 0) return `${normalized} 小时 (${dayEquivalent} 天)`;
    return `${normalized} 小时`;
  }

  function normalizeAnalysis(analysis: any): DeconstructAnalysis {
    const normalized = emptyAnalysis();
    if (!analysis || typeof analysis !== 'object') return normalized;

    const listFields: Array<'missing_info' | 'risks' | 'dependencies' | 'acceptance_criteria' | 'schedule_notes' | 'meeting_questions'> = [
      'missing_info',
      'risks',
      'dependencies',
      'acceptance_criteria',
      'schedule_notes',
      'meeting_questions'
    ];

    for (const field of listFields) {
      const value = analysis[field];
      normalized[field] = Array.isArray(value)
        ? value.map((item: any) => String(item).trim()).filter(Boolean)
        : [];
    }

    const score = Number(analysis.completeness_score);
    normalized.completeness_score = Number.isFinite(score) ? Math.min(100, Math.max(0, Math.round(score))) : 0;

    const overallDays = Number(analysis.overall_estimated_days);
    const overallHours = Number(analysis.overall_estimated_hours);
    normalized.overall_estimated_days = Number.isFinite(overallDays) ? Math.max(0, Math.round(overallDays * 10) / 10) : 0;
    normalized.overall_estimated_hours = Number.isFinite(overallHours) ? Math.max(0, Math.round(overallHours * 10) / 10) : normalized.overall_estimated_days * 8;
    normalized.overall_difficulty = normalizeDifficulty(analysis.overall_difficulty);
    normalized.estimate_basis = typeof analysis.estimate_basis === 'string' ? analysis.estimate_basis.trim() : '';

    const confidence = Number(analysis.confidence);
    normalized.confidence = Number.isFinite(confidence) ? Math.min(1, Math.max(0, confidence > 1 ? confidence / 100 : confidence)) : 0;

    return normalized;
  }

  function normalizeDifficulty(value: any) {
    const normalized = String(value || '').trim().toLowerCase();
    if (['high', '高', '困难', '复杂'].includes(normalized)) return 'High';
    if (['low', '低', '简单'].includes(normalized)) return 'Low';
    return 'Medium';
  }

  function difficultyLabel(value: string) {
    if (value === 'High') return 'High (高)';
    if (value === 'Low') return 'Low (低)';
    return 'Medium (中)';
  }

  function priorityTone(value: string): AdminTone {
    const normalized = String(value || '').toLowerCase();
    if (normalized === 'high') return 'danger';
    if (normalized === 'medium') return 'warning';
    if (normalized === 'low') return 'success';
    return 'neutral';
  }

  function difficultyTone(value: string): AdminTone {
    const normalized = String(value || '').toLowerCase();
    if (normalized === 'high') return 'danger';
    if (normalized === 'medium') return 'warning';
    if (normalized === 'low') return 'success';
    return 'neutral';
  }

  function scoreTone(score: number): AdminTone {
    if (score >= 85) return 'success';
    if (score >= 65) return 'warning';
    return 'danger';
  }

  function buildGeneratedTaskRow(task: GeneratedTask): AdminTableRow {
    return {
      id: task.id,
      title: task.title,
      status: '待同步',
      tone: priorityTone(task.priority),
      owner: task.assignee,
      dueDate: '',
      priority: task.priority,
      risk: task.difficulty,
      cells: {
        task: task.title,
        repo: task.repo,
        owner: task.assignee,
        estimate: getHoursLabel(task.estimated_hours),
        priority: task.priority,
        difficulty: task.difficulty,
        evidence: task.estimate_basis ? '有估算依据' : '待补依据'
      }
    };
  }

  function selectGeneratedTask(task: GeneratedTask) {
    activeGeneratedTaskId = task.id;
  }

  function estimateSummaryLabel(days: number, hours: number) {
    if (hours > 0) return `${Math.round(hours)} 小时`;
    if (days > 0) return `${Math.round(days * 8)} 小时`;
    return '待估算';
  }

  function normalizeGeneratedTask(t: any): GeneratedTask {
    const estimatedHours = Number(t.estimated_hours);
    const estimatedDays = Number(t.estimated_days ?? t.period_days);
    const hours = Number.isFinite(estimatedHours) && estimatedHours > 0
      ? Math.round(estimatedHours * 10) / 10
      : Number.isFinite(estimatedDays) && estimatedDays > 0
        ? Math.round(estimatedDays * 80) / 10
        : 40;
    const days = Math.round((hours / 8) * 10) / 10;
    const difficulty = normalizeDifficulty(t.difficulty || t.complexity);

    return {
      ...t,
      priority: t.priority || 'Medium',
      complexity: t.complexity || difficulty,
      difficulty,
      estimated_days: days,
      estimated_hours: hours,
      estimate_basis: typeof t.estimate_basis === 'string' ? t.estimate_basis.trim() : '',
      period_days: days
    };
  }

  function selectEstimateHours(taskId: string, hours: number) {
    const days = Math.round((hours / 8) * 10) / 10;
    result.tasks = result.tasks.map(t => {
      if (t.id === taskId) {
        return {
          ...t,
          estimated_days: days,
          estimated_hours: hours,
          period_days: days
        };
      }
      return t;
    });
    activeDropdown = null;
  }

  function percentLabel(value: number) {
    return `${Math.round(value)}%`;
  }

  function confidenceLabel(value: number) {
    return `${Math.round(value * 100)}%`;
  }

  function asText(value: any, fallback = ''): string {
    if (value === null || value === undefined) return fallback;
    const text = String(value).trim();
    return text || fallback;
  }

  function normalizeTextList(value: any): string[] {
    if (Array.isArray(value)) {
      return value.map(item => asText(item)).filter(Boolean).slice(0, 6);
    }
    if (typeof value === 'string') {
      return value
        .split(/\n|；|;/)
        .map(item => item.trim())
        .filter(Boolean)
        .slice(0, 6);
    }
    return [];
  }

  function normalizeConfidenceValue(value: any): number {
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) return 0;
    return Math.min(1, Math.max(0, parsed > 1 ? parsed / 100 : parsed));
  }

  function normalizeIntentInsight(data: any, endpoint: string): IntentInsight {
    const payload = data?.result || data?.data || data || {};
    const missingContext = normalizeTextList(payload.missing_context || payload.missingContext || payload.missing_info || payload.missingInfo);
    const nextQuestions = normalizeTextList(payload.next_questions || payload.nextQuestions || payload.questions || payload.meeting_questions);
    return {
      intent: asText(payload.intent || payload.intent_type || payload.type, 'unknown'),
      confidence: normalizeConfidenceValue(payload.confidence ?? payload.score),
      summary: asText(payload.summary || payload.answer || payload.brief, '暂无摘要'),
      missing_context: missingContext,
      next_questions: nextQuestions,
      suggested_action: asText(payload.suggested_action || payload.suggestedAction || payload.action || payload.next_step, '补充缺失信息后再进入解构或调停'),
      endpoint,
      source: 'api'
    };
  }

  function inferLocalIntent(text: string, endpoint: string): IntentInsight {
    const normalized = text.trim();
    const lower = normalized.toLowerCase();
    let intent = 'requirement_clarification';
    if (/bug|缺陷|故障|报错|异常|crash/.test(lower)) {
      intent = 'bug_triage';
    } else if (/总结|周会|日报|复盘|纪要|meeting|summary/.test(lower)) {
      intent = 'conversation_summary';
    } else if (/延期|转派|挂起|升级|冲突|override/.test(lower)) {
      intent = 'override_decision';
    } else if (/排期|截止|due|schedule/.test(lower)) {
      intent = 'schedule_governance';
    }

    const missing: string[] = [];
    if (!/负责人|assignee|owner|谁/.test(lower)) missing.push('负责人或协作部门');
    if (!/截止|deadline|due|上线|日期/.test(lower)) missing.push('截止时间或期望上线窗口');
    if (!/验收|acceptance|完成标准|测试/.test(lower)) missing.push('验收标准或验证方式');
    if (!/影响|范围|impact|依赖|dependency/.test(lower)) missing.push('影响范围和外部依赖');

    const summary = normalized
      ? normalized.replace(/\s+/g, ' ').slice(0, 180)
      : '等待输入多轮对话或需求文本';

    return {
      intent,
      confidence: normalized ? 0.42 : 0,
      summary,
      missing_context: missing.slice(0, 4),
      next_questions: missing.slice(0, 3).map(item => `请补充${item}`),
      suggested_action: intent === 'conversation_summary'
        ? '先确认会议结论和待办责任人，再同步到决策队列'
        : '先补齐缺失上下文，再进入 AI 解构或人工调停',
      endpoint,
      source: 'local'
    };
  }

  function intentLabel(intent: string) {
    const normalized = intent.toLowerCase();
    if (normalized.includes('bug')) return '缺陷排查';
    if (normalized.includes('summary')) return '对话总结';
    if (normalized.includes('override')) return '人工调停';
    if (normalized.includes('schedule')) return '排期治理';
    if (normalized.includes('requirement')) return '需求澄清';
    return intent || '未知意图';
  }

  function syncIntentInputFromDemand() {
    intentInputText = inputText.trim();
    intentError = '';
  }

  async function runIntentAssistant(mode: IntentMode) {
    const text = intentInputText.trim();
    if (!text) {
      displayToast('请先输入对话或需求文本', 'error');
      return;
    }

    intentMode = mode;
    intentLoading = true;
    intentError = '';
    const endpoint = mode === 'summary' ? '/api/ai/assistant/summary' : '/api/ai/intent';

    try {
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          text,
          messages: [{ role: 'user', content: text }],
          mode
        })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP ${res.status}`);
      }

      const data = await res.json();
      intentResult = normalizeIntentInsight(data, endpoint);
    } catch (e: any) {
      intentResult = inferLocalIntent(text, endpoint);
      intentError = `${endpoint} 暂不可用，当前展示本地兜底识别结果。${e.message || ''}`.trim();
    } finally {
      intentLoading = false;
    }
  }

  async function fetchConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const config = await res.json();
        isAIEnabled = !!(config.ai && config.ai.enabled);

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
          assigneesList = users.filter(name => name !== '未指派' && name !== '-');
          console.log('[Deconstructor] Loaded team members from config:', assigneesList);
        } else {
          assigneesList = ['Eddie', 'Antigravity'];
        }
      }
    } catch (e) {
      console.error('[Deconstructor] Failed to load config:', e);
    }
  }

  function handleConfigUpdated(e: Event) {
    const customEvent = e as CustomEvent;
    const config = customEvent.detail;
    console.log('[Deconstructor] Received config-updated event:', config);
    if (config) {
      isAIEnabled = !!(config.ai && config.ai.enabled);

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
        assigneesList = users.filter(name => name !== '未指派' && name !== '-');
        console.log('[Deconstructor] Hot-updated team members from config-updated:', assigneesList);
      } else {
        assigneesList = ['Eddie', 'Antigravity'];
      }
      console.log('[Deconstructor] Updated isAIEnabled to =', isAIEnabled);
    }
  }

  onMount(() => {
    fetchConfig();
    fetchActiveDemands();

    const handleWindowFocus = () => {
      fetchConfig();
      fetchActiveDemands();
    };

    const handleOutsideClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (activeDropdown && !target.closest('.custom-dropdown-container')) {
        activeDropdown = null;
      }
      if (showDemandDropdown && !target.closest('.deconstruct-demand-link-select')) {
        showDemandDropdown = false;
      }
    };

    if (typeof window !== 'undefined') {
      window.addEventListener('config-updated', handleConfigUpdated);
      window.addEventListener('focus', handleWindowFocus);
      window.addEventListener('click', handleOutsideClick);
    }
    return () => {
      if (typeof window !== 'undefined') {
        window.removeEventListener('config-updated', handleConfigUpdated);
        window.removeEventListener('focus', handleWindowFocus);
        window.removeEventListener('click', handleOutsideClick);
      }
    };
  });

  async function handleDeconstruct() {
    if (!inputText.trim()) {
      displayToast('请先输入要解构的需求描述', 'error');
      return;
    }

    isLoading = true;
    hasResult = false;
    isMockResponse = false;
    currentTaskGroupId = '';

    try {
      const res = await fetch('/api/deconstruct', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ text: inputText })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP 错误 ${res.status}`);
      }

      const data = await res.json();
      const rawTasks = data.tasks || [];

      // 规范化负责人：模糊匹配真实 sync_users 列表中的名字，不符合的强制指派
      const tasksWithDefaults = rawTasks.map((t: any) => {
        let assigned = t.assignee || '';
        let matchedMember = '';
        for (const member of assigneesList) {
          if (assigned.toLowerCase().includes(member.toLowerCase())) {
            matchedMember = member;
            break;
          }
        }

        let finalAssignee = '';
        if (matchedMember) {
          finalAssignee = matchedMember;
        } else {
          // 如果大模型随机生成的负责人不属于实际筛选人员名单，强制匹配到第一个真实团队人员
          finalAssignee = assigneesList.length > 0 ? assigneesList[0] : 'Unassigned';
        }

        return normalizeGeneratedTask({
          ...t,
          assignee: finalAssignee
        });
      });

      result = {
        mappedRepos: data.mappedRepos || [],
        tasks: tasksWithDefaults,
        analysis: normalizeAnalysis(data.analysis),
        context_pack_id: Number(data.context_pack_id) || 0
      };

      // 成功生成解构任务时，优先沿用已选需求的任务组 ID，保证二次解构仍挂在同一父需求上。
      currentTaskGroupId = getSelectedDemandTaskGroupId() || (selectedDemandId ? createTaskGroupId(selectedDemandId) : 'group-' + Date.now());
      isMockResponse = !!data.is_mock;
      hasResult = true;
    } catch (e: any) {
      console.error('AI Deconstruct failed:', e);
      displayToast(`AI 解构失败: ${e.message}`, 'error');
    } finally {
      isLoading = false;
    }
  }

  let toastMsg = '';
  let toastType: 'success' | 'error' | 'info' = 'info';
  let showToast = false;
  let toastTimeout: any;

  function displayToast(msg: string, type: 'success' | 'error' | 'info' = 'info') {
    toastMsg = msg;
    toastType = type;
    showToast = true;
    if (toastTimeout) clearTimeout(toastTimeout);
    toastTimeout = setTimeout(() => {
      showToast = false;
    }, 3000);
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (!isAIEnabled) return;
    isDragging = true;
  }

  function handleDragLeave() {
    isDragging = false;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
    if (!isAIEnabled) return;
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      processFile(files[0]);
    }
  }

  function handleFileSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = input.files;
    if (files && files.length > 0) {
      processFile(files[0]);
    }
  }

  function processFile(file: File) {
    const allowedExtensions = ['.txt', '.md', '.json', '.csv', '.xml', '.html'];
    const fileName = file.name.toLowerCase();
    const isAllowed = allowedExtensions.some(ext => fileName.endsWith(ext));

    if (!isAllowed) {
      displayToast('仅支持文本类文件 (如 .txt, .md, .json 等)', 'error');
      return;
    }

    uploadedFileName = file.name;
    uploadedFileSize = file.size;

    const reader = new FileReader();
    reader.onload = (e) => {
      const text = e.target?.result as string;
      inputText = text;
      displayToast(`已成功读取文档 "${file.name}" 并填充到需求描述中。`, 'success');
    };
    reader.readAsText(file);
  }

  async function importTasksToKanban() {
    if (!result.tasks || result.tasks.length === 0) return;
    isImporting = true;
    try {
      const res = await fetch('/api/tasks/import', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          task_group_id: currentTaskGroupId,
          demand_id: selectedDemandId,
          input_text: inputText,
          mappedRepos: result.mappedRepos,
          analysis: result.analysis,
          context_pack_id: result.context_pack_id,
          is_mock: isMockResponse,
          tasks: result.tasks
        })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP ${res.status}`);
      }

      await fetchActiveDemands();
      displayToast(`已同步 ${result.tasks.length} 个解构任务至项目看板`, 'success');
    } catch (e: any) {
      console.error('Import tasks failed:', e);
      displayToast(`同步失败: ${e.message}`, 'error');
    } finally {
      isImporting = false;
    }
  }

  function deleteTask(taskId: string) {
    result.tasks = result.tasks.filter(t => t.id !== taskId);
    displayToast('任务已删除', 'info');
  }
</script>

<section class="deconstructor-workbench wa-grain">
  <div class="deconstructor-header">
    <div>
      <span class="eyebrow">需求证据解构</span>
      <h2>任务证据生成台</h2>
      <p>把非结构化需求转成可同步到看板的任务、仓库、负责人和检查项。</p>
    </div>
    <span class="wa-admin-pill {isAIEnabled ? 'tone-success' : 'tone-danger'}">{isAIEnabled ? 'AI 已启用' : 'AI 未启用'}</span>
  </div>

  <div class="deconstructor-metrics" aria-label="解构事实指标">
    {#each deconstructMetrics as metric}
      <div class="wa-admin-card wa-admin-metric deconstructor-metric {ADMIN_TONE_CLASS[metric.tone || 'neutral']}">
        <span>{metric.label}</span>
        <strong>{metric.value}</strong>
        <em>{metric.helper}</em>
      </div>
    {/each}
  </div>

  <div class="deconstructor-entry-card wa-admin-card {!isAIEnabled ? 'disabled-panel' : ''}">
    <div class="entry-card-head">
      <div>
        <span class="eyebrow">输入材料</span>
        <h3>需求文本与意图识别</h3>
      </div>
      <button
        on:click={handleDeconstruct}
        disabled={isLoading || !isAIEnabled}
        class="wa-admin-action primary"
      >
        {#if isLoading}
          <span class="spinner-small"></span>
          解构分析中
        {:else}
          生成解构
        {/if}
      </button>
    </div>

    {#if !isAIEnabled}
      <div class="ai-disabled-indicator">
        <span class="status-dot danger" aria-hidden="true"></span>
        <div class="warning-text">
          <strong>AI 需求解构未启用</strong>
          <p>后台大模型配置未启用或未检测到 API 凭证。启用后可导入文档、解构任务并识别意图。</p>
        </div>
      </div>
    {/if}

    <div class="entry-grid">
      <div class="entry-input-column">
        {#if isAIEnabled}
          <div
            class="file-dropzone {isDragging ? 'dragging' : ''}"
            on:dragover={handleDragOver}
            on:dragleave={handleDragLeave}
            on:drop={handleDrop}
            role="button"
            tabindex="0"
          >
            <input
              type="file"
              id="file-upload"
              accept=".txt,.md,.json,.csv,.xml,.html"
              on:change={handleFileSelect}
              class="file-input"
            />
            <label for="file-upload" class="dropzone-label">
              <span class="upload-symbol">DOC</span>
              {#if uploadedFileName}
                <span class="upload-text text-indigo">已加载：{uploadedFileName} ({uploadedFileSize} 字节)</span>
              {:else}
                <span class="upload-text">拖拽 .md / .txt / .json 文档，或 <span class="browse-link">浏览文件</span></span>
              {/if}
            </label>
          </div>
        {/if}

        <label for="raw-demand" class="input-label">非结构化需求或缺陷描述</label>
        <textarea
          id="raw-demand"
          bind:value={inputText}
          disabled={!isAIEnabled}
          class="demand-textarea"
          placeholder={isAIEnabled ? "粘贴原始需求、缺陷描述或会议整理..." : "AI 服务未启用，不可输入..."}
        ></textarea>
      </div>

      <div class="intent-console-panel">
        <div class="intent-console-header">
          <div>
            <span class="result-section-label">意图识别 / 对话总结</span>
            <p>把沟通记录拆成当前意图、缺失上下文、下一问和建议动作。</p>
          </div>
          <button
            type="button"
            class="wa-admin-action secondary font-mono"
            on:click={syncIntentInputFromDemand}
            disabled={!inputText.trim()}
          >
            载入文本
          </button>
        </div>

        <textarea
          class="intent-textarea"
          bind:value={intentInputText}
          placeholder="粘贴多轮沟通记录，或复用左侧需求文本..."
        ></textarea>

        <div class="intent-actions-row">
          <div class="intent-mode-tabs font-mono">
            <button
              type="button"
              class:intent-active={intentMode === 'intent'}
              on:click={() => intentMode = 'intent'}
            >
              意图
            </button>
            <button
              type="button"
              class:intent-active={intentMode === 'summary'}
              on:click={() => intentMode = 'summary'}
            >
              总结
            </button>
          </div>
          <button
            type="button"
            class="wa-admin-action primary font-mono"
            on:click={() => runIntentAssistant(intentMode)}
            disabled={intentLoading || !intentInputText.trim()}
          >
            {intentLoading ? '识别中' : intentMode === 'summary' ? '生成总结' : '识别意图'}
          </button>
        </div>

        {#if intentError}
          <div class="intent-inline-error font-mono">{intentError}</div>
        {/if}

        <div class="intent-result-shell">
          {#if intentResult}
            <div class="intent-result-top font-mono">
              <div>
                <span>当前意图</span>
                <strong>{intentLabel(intentResult.intent)}</strong>
              </div>
              <div>
                <span>置信度</span>
                <strong>{confidenceLabel(intentResult.confidence)}</strong>
              </div>
              <div>
                <span>来源</span>
                <strong>{intentResult.source === 'api' ? intentResult.endpoint : 'local fallback'}</strong>
              </div>
            </div>

            <div class="intent-summary-block">
              <span>摘要</span>
              <p>{intentResult.summary}</p>
            </div>

            <div class="intent-detail-grid">
              <div class="intent-detail-cell">
                <span>缺失上下文</span>
                {#if intentResult.missing_context.length > 0}
                  <ul>
                    {#each intentResult.missing_context as item}
                      <li>{item}</li>
                    {/each}
                  </ul>
                {:else}
                  <p>暂无明显缺口</p>
                {/if}
              </div>

              <div class="intent-detail-cell">
                <span>下一问</span>
                {#if intentResult.next_questions.length > 0}
                  <ul>
                    {#each intentResult.next_questions as item}
                      <li>{item}</li>
                    {/each}
                  </ul>
                {:else}
                  <p>无需追加提问，可进入下一步</p>
                {/if}
              </div>
            </div>

            <div class="intent-action-strip">
              <span class="font-mono">建议动作</span>
              <p>{intentResult.suggested_action}</p>
            </div>
          {:else}
            <div class="intent-empty-state font-mono">
              等待输入后识别，结果会展示 intent、confidence、summary、missing_context、next_questions 和 suggested_action。
            </div>
          {/if}
        </div>
      </div>
    </div>

    {#if isAIEnabled}
      <div class="prompt-constraint-panel">
        <button class="prompt-toggle-btn" on:click={() => showPromptSettings = !showPromptSettings}>
          <span>解构 Prompt 约束</span>
          <span class="arrow">{showPromptSettings ? '收起' : '展开'}</span>
        </button>
        {#if showPromptSettings}
          <div class="prompt-content font-mono" transition:slide>
            <p class="prompt-desc">解构引擎使用固定系统 Prompt，约束输出任务、仓库、负责人、工时和检查项，保证入库格式一致。</p>
            <pre class="prompt-pre"><code>{`【输出 JSON 契约规约】
{
  "mappedRepos": ["仓库名称"],
  "tasks": [
    {
      "id": "task-xxx",
      "repo": "仓库名称",
      "title": "任务标题（描述该仓库具体功能）",
      "assignee": "推荐人名字（如 Eddie）",
      "priority": "High/Medium/Low",
      "complexity": "High/Medium/Low",
      "difficulty": "High/Medium/Low",
      "estimated_days": 2.5,
      "estimated_hours": 20,
      "estimate_basis": "估算依据"
    }
  ],
  "analysis": {
    "completeness_score": 82,
    "overall_estimated_days": 6,
    "overall_estimated_hours": 48,
    "overall_difficulty": "Medium",
    "estimate_basis": "整体估算依据",
    "missing_info": ["待补充的信息"],
    "risks": ["交付风险"],
    "dependencies": ["依赖项"],
    "acceptance_criteria": ["可验证验收标准"],
    "schedule_notes": ["排期提示"],
    "meeting_questions": ["评审会问题"],
    "confidence": 0.78
  }
}`}</code></pre>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  {#if hasResult && isMockResponse}
    <div class="mock-alert-banner wa-admin-card">
      <span class="status-dot warning" aria-hidden="true"></span>
      <div class="mock-alert-text-group">
        <p class="mock-alert-title">当前结果标记为模拟响应</p>
        <p class="mock-alert-desc">后端返回了 is_mock=true。请在同步前确认 API 配置与任务内容。</p>
      </div>
    </div>
  {/if}

  <div class="deconstructor-main-grid">
    <section class="deconstructor-table-stack wa-admin-section" aria-label="任务证据列表">
      <div class="deconstructor-table-card wa-admin-card">
        <div class="deconstructor-table-head">
          <div>
            <span class="eyebrow">解构列表</span>
            <h3>任务证据表</h3>
          </div>
          <span class="deconstructor-count font-mono">{generatedTaskRows.length} tasks</span>
        </div>

        {#if isLoading}
          <div class="state-panel">
            <div class="spinner-large"></div>
            <p class="loading-text">正在匹配仓库、负责人和验收检查项</p>
          </div>
        {:else if !hasResult}
          <div class="state-panel">
            <strong>等待需求文本</strong>
            <p>输入或导入需求后生成任务证据，结果会进入下方表格并可同步到看板。</p>
          </div>
        {:else}
          <div class="result-section mapped-repos-section">
            <span class="result-section-label">匹配代码库</span>
            <div class="repo-list">
              {#each result.mappedRepos as repo}
                <span class="repo-badge">{repo}</span>
              {/each}
            </div>
          </div>

          <div class="wa-admin-table-shell deconstructor-table-shell">
            <table class="wa-admin-table deconstructor-table">
              <colgroup>
                {#each deconstructColumns as column}
                  <col style="width: {column.width || 'auto'}" />
                {/each}
                <col style="width: 86px" />
              </colgroup>
              <thead>
                <tr>
                  {#each deconstructColumns as column}
                    <th class:align-center={column.align === 'center'}>{column.label}</th>
                  {/each}
                  <th class="align-center">操作</th>
                </tr>
              </thead>
              <tbody>
                {#each result.tasks as task (task.id)}
                  {@const row = buildGeneratedTaskRow(task)}
                  <tr
                    class:is-selected={activeGeneratedTaskId === task.id}
                    on:click={() => selectGeneratedTask(task)}
                  >
                    <td>
                      <div class="task-title-cell">
                        <span class="task-gen-id font-mono">{task.id}</span>
                        <input
                          type="text"
                          class="task-title-input-premium"
                          bind:value={task.title}
                          placeholder="修改任务标题..."
                          on:click|stopPropagation
                        />
                      </div>
                    </td>
                    <td>
                      <div class="custom-dropdown-container">
                        <button
                          type="button"
                          class="custom-dropdown-trigger"
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'repo')}
                        >
                          <span class="trigger-value">{task.repo}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'repo') ? 'rotated' : ''}">▼</span>
                        </button>
                        {#if isDropdownOpen(task.id, 'repo')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each result.mappedRepos.length > 0 ? result.mappedRepos : ['frontend-dashboard', 'backend-core'] as repo}
                              <button
                                type="button"
                                class="dropdown-option-btn {task.repo === repo ? 'selected' : ''}"
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'repo', repo)}
                              >
                                {repo}
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </td>
                    <td>
                      <div class="custom-dropdown-container">
                        <button
                          type="button"
                          class="custom-dropdown-trigger"
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'assignee')}
                        >
                          <span class="trigger-value">{task.assignee}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'assignee') ? 'rotated' : ''}">▼</span>
                        </button>
                        {#if isDropdownOpen(task.id, 'assignee')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each assigneesList as member}
                              <button
                                type="button"
                                class="dropdown-option-btn {task.assignee === member ? 'selected' : ''}"
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'assignee', member)}
                              >
                                {member}
                              </button>
                            {/each}
                            <button
                              type="button"
                              class="dropdown-option-btn {task.assignee === 'Unassigned' ? 'selected' : ''}"
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'assignee', 'Unassigned')}
                            >
                              暂无分配
                            </button>
                          </div>
                        {/if}
                      </div>
                    </td>
                    <td>
                      <div class="custom-dropdown-container">
                        <button
                          type="button"
                          class="custom-dropdown-trigger"
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'estimated_hours')}
                        >
                          <span class="trigger-value">{getHoursLabel(task.estimated_hours)}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'estimated_hours') ? 'rotated' : ''}">▼</span>
                        </button>
                        {#if isDropdownOpen(task.id, 'estimated_hours')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each [2, 4, 6, 8, 12, 16, 24, 32, 40, 56, 80, 112] as hours}
                              <button
                                type="button"
                                class="dropdown-option-btn {task.estimated_hours === hours ? 'selected' : ''}"
                                on:click|stopPropagation={() => selectEstimateHours(task.id, hours)}
                              >
                                {getHoursLabel(hours)}
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </td>
                    <td class="align-center">
                      <div class="custom-dropdown-container">
                        <button
                          type="button"
                          class="wa-admin-pill task-pill-button {ADMIN_TONE_CLASS[priorityTone(task.priority)]}"
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'priority')}
                        >
                          {task.priority === 'High' ? 'High' : task.priority === 'Medium' ? 'Medium' : 'Low'}
                        </button>
                        {#if isDropdownOpen(task.id, 'priority')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each ['High', 'Medium', 'Low'] as priority}
                              <button
                                type="button"
                                class="dropdown-option-btn {task.priority === priority ? 'selected' : ''}"
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'priority', priority)}
                              >
                                {priority}
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </td>
                    <td class="align-center">
                      <div class="custom-dropdown-container">
                        <button
                          type="button"
                          class="wa-admin-pill task-pill-button {ADMIN_TONE_CLASS[difficultyTone(task.difficulty)]}"
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'difficulty')}
                        >
                          {task.difficulty}
                        </button>
                        {#if isDropdownOpen(task.id, 'difficulty')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each ['High', 'Medium', 'Low'] as difficulty}
                              <button
                                type="button"
                                class="dropdown-option-btn {task.difficulty === difficulty ? 'selected' : ''}"
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'difficulty', difficulty)}
                              >
                                {difficultyLabel(difficulty)}
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </td>
                    <td>
                      <span class="wa-admin-pill {task.estimate_basis ? 'tone-success' : 'tone-warning'}">{row.cells.evidence}</span>
                    </td>
                    <td class="align-center">
                      <button class="wa-admin-action danger compact-action" type="button" on:click|stopPropagation={() => deleteTask(task.id)}>
                        删除
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </section>

    <aside class="deconstructor-inspector wa-admin-card wa-admin-inspector" aria-label="解构详情与同步检查">
      <div class="inspector-title-row">
        <span class="wa-admin-pill {hasResult ? 'tone-info' : 'tone-neutral'}">{hasResult ? '已生成' : '待生成'}</span>
        <span class="font-mono">{currentTaskGroupId ? '任务组已创建' : '未创建任务组'}</span>
      </div>

      <h3>同步检查项</h3>
      <p>{hasResult ? '确认关联需求、估算依据和风险项后再同步到看板。' : '生成结果后，这里会显示选中任务与整体分析。'}</p>

      <div class="deconstruct-inspector-facts">
        <div>
          <span>任务数量</span>
          <strong>{result.tasks.length}</strong>
        </div>
        <div>
          <span>代码库</span>
          <strong>{result.mappedRepos.length}</strong>
        </div>
        <div>
          <span>上下文包</span>
          <strong>{result.context_pack_id || '-'}</strong>
        </div>
        <div>
          <span>可关联需求</span>
          <strong>{activeDemands.length}</strong>
        </div>
      </div>

      {#if hasResult}
        <div class="deconstruct-progress">
          <div>
            <span>需求完整性</span>
            <strong class="font-mono">{percentLabel(result.analysis.completeness_score)}</strong>
          </div>
          <div class="wa-admin-progress" style="--progress: {result.analysis.completeness_score}%" aria-hidden="true"></div>
        </div>

        <div class="inspector-sync-section">
          <h4>同步目标</h4>
          <div class="custom-dropdown-container deconstruct-demand-link-select">
            <button
              type="button"
              class="dropdown-trigger"
              on:click|stopPropagation={() => showDemandDropdown = !showDemandDropdown}
            >
              <span>{selectedDemandId === '' ? '不关联需求，仅同步任务' : selectedDemandTitle}</span>
              <span class="arrow-icon {showDemandDropdown ? 'open' : ''}">▼</span>
            </button>
            {#if showDemandDropdown}
              <div class="dropdown-options-list glass-panel">
                <button
                  type="button"
                  class="dropdown-option-item {selectedDemandId === '' ? 'selected' : ''}"
                  on:click={() => selectDemandLink(null)}
                >
                  不关联需求，仅同步任务
                </button>
                {#each activeDemands as demand}
                  <button
                    type="button"
                    class="dropdown-option-item {selectedDemandId === demand.task_id ? 'selected' : ''}"
                    on:click={() => selectDemandLink(demand)}
                  >
                    {getDemandDisplayTitle(demand)}
                  </button>
                {/each}
              </div>
            {/if}
          </div>

          <button
            class="wa-admin-action primary"
            on:click={importTasksToKanban}
            disabled={isImporting || result.tasks.length === 0}
          >
            {#if isImporting}
              <span class="spinner-small"></span>
              同步中
            {:else}
              同步到看板
            {/if}
          </button>
        </div>

        {#if selectedGeneratedTask}
          <div class="inspector-task-section">
            <h4>选中任务</h4>
            <strong>{selectedGeneratedTask.title}</strong>
            <div class="selected-task-pills">
              <span class="wa-admin-pill tone-info">{selectedGeneratedTask.repo}</span>
              <span class="wa-admin-pill tone-neutral">{selectedGeneratedTask.assignee}</span>
              <span class="wa-admin-pill {ADMIN_TONE_CLASS[priorityTone(selectedGeneratedTask.priority)]}">{selectedGeneratedTask.priority}</span>
              <span class="wa-admin-pill {ADMIN_TONE_CLASS[difficultyTone(selectedGeneratedTask.difficulty)]}">{difficultyLabel(selectedGeneratedTask.difficulty)}</span>
            </div>
            <p>{selectedGeneratedTask.estimate_basis || '该任务尚未返回估算依据，建议同步前补充。'}</p>
          </div>
        {/if}

        <div class="inspector-analysis-section">
          <h4>整体分析</h4>
          <div class="analysis-kpi-row">
            <span>置信度 <strong>{confidenceLabel(result.analysis.confidence)}</strong></span>
            <span>预估 <strong>{estimateSummaryLabel(result.analysis.overall_estimated_days, result.analysis.overall_estimated_hours)}</strong></span>
            <span>难度 <strong>{difficultyLabel(result.analysis.overall_difficulty)}</strong></span>
          </div>
          {#if result.analysis.estimate_basis}
            <p>{result.analysis.estimate_basis}</p>
          {/if}
        </div>

        <div class="inspector-checklist-grid">
          <div class="analysis-cell">
            <span class="analysis-cell-title">缺失信息</span>
            {#if result.analysis.missing_info.length > 0}
              <ul class="analysis-list">
                {#each result.analysis.missing_info as item}
                  <li>{item}</li>
                {/each}
              </ul>
            {:else}
              <p class="analysis-empty">暂无明显缺口</p>
            {/if}
          </div>

          <div class="analysis-cell risk-cell">
            <span class="analysis-cell-title">风险</span>
            {#if result.analysis.risks.length > 0}
              <ul class="analysis-list">
                {#each result.analysis.risks as item}
                  <li>{item}</li>
                {/each}
              </ul>
            {:else}
              <p class="analysis-empty">暂无高风险提示</p>
            {/if}
          </div>

          <div class="analysis-cell">
            <span class="analysis-cell-title">验收标准</span>
            {#if result.analysis.acceptance_criteria.length > 0}
              <ul class="analysis-list">
                {#each result.analysis.acceptance_criteria as item}
                  <li>{item}</li>
                {/each}
              </ul>
            {:else}
              <p class="analysis-empty">待补充可验证标准</p>
            {/if}
          </div>

          <div class="analysis-cell">
            <span class="analysis-cell-title">会审问题</span>
            {#if result.analysis.meeting_questions.length > 0}
              <ul class="analysis-list">
                {#each result.analysis.meeting_questions as item}
                  <li>{item}</li>
                {/each}
              </ul>
            {:else}
              <p class="analysis-empty">暂无待会审问题</p>
            {/if}
          </div>
        </div>
      {:else}
        <div class="inspector-empty-state">
          <strong>尚未生成任务证据</strong>
          <span>输入需求并点击“生成解构”后，检查项会在这里出现。</span>
        </div>
      {/if}
    </aside>
  </div>

  <div class="toast {toastType} {showToast ? 'show' : ''}">
    <span class="toast-status-dot" aria-hidden="true"></span>
    <span>{toastMsg}</span>
  </div>
</section>

<style>
  .deconstructor-section {
    background: rgba(17, 24, 39, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 24px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    margin-bottom: 32px;
    box-sizing: border-box;
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
    background-clip: text;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    margin: 0;
  }

  .badge {
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    background: rgba(99, 102, 241, 0.1);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.2);
    padding: 2px 8px;
    border-radius: 4px;
  }

  .badge-sub {
    font-size: 0.75rem;
    color: #64748b;
  }

  .split-layout {
    display: grid;
    grid-template-columns: repeat(1, minmax(0, 1fr));
    gap: 32px;
  }

  @media (min-width: 1024px) {
    .split-layout {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  .panel {
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }

  .disabled-panel {
    position: relative;
  }

  .demand-textarea:disabled {
    background: rgba(2, 6, 23, 0.4);
    border-color: rgba(51, 65, 85, 0.2);
    color: #475569;
    cursor: not-allowed;
  }

  .ai-disabled-indicator {
    display: flex;
    gap: 12px;
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    padding: 12px;
    border-radius: 8px;
    margin-bottom: 12px;
    align-items: flex-start;
  }

  .ai-disabled-indicator .warning-text strong {
    font-size: 0.8rem;
    font-weight: 700;
    color: #f87171;
    display: block;
    margin-bottom: 4px;
  }

  .ai-disabled-indicator .warning-text p {
    font-size: 0.75rem;
    color: #94a3b8;
    margin: 0;
    line-height: 1.4;
  }

  .input-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: #94a3b8;
    margin-bottom: 8px;
  }

  .demand-textarea {
    width: 100%;
    height: 192px;
    background: rgba(2, 6, 23, 0.8);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    padding: 16px;
    color: #cbd5e1;
    font-size: 0.875rem;
    line-height: 1.6;
    font-family: system-ui, -apple-system, sans-serif;
    resize: none;
    box-sizing: border-box;
    transition: border-color 0.2s;
  }

  .demand-textarea:focus {
    outline: none;
    border-color: #6366f1;
  }

  .intent-console-panel {
    margin-top: 14px;
    border: 1px solid rgba(56, 189, 248, 0.22);
    background: rgba(2, 6, 23, 0.34);
    border-radius: 8px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.05);
  }

  .intent-console-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .intent-console-header p {
    margin: 4px 0 0 0;
    color: #64748b;
    font-size: 0.72rem;
    line-height: 1.45;
  }

  .btn-load-intent-source,
  .btn-run-intent {
    border: 1px solid rgba(129, 140, 248, 0.28);
    background: rgba(99, 102, 241, 0.1);
    color: #c4b5fd;
    border-radius: 6px;
    padding: 6px 9px;
    font-size: 0.66rem;
    font-weight: 800;
    cursor: pointer;
    transition: border-color 0.18s ease, background 0.18s ease, transform 0.12s ease;
    white-space: nowrap;
  }

  .btn-load-intent-source:hover:not(:disabled),
  .btn-run-intent:hover:not(:disabled) {
    border-color: rgba(129, 140, 248, 0.58);
    background: rgba(99, 102, 241, 0.18);
  }

  .btn-load-intent-source:active,
  .btn-run-intent:active {
    transform: scale(0.97);
  }

  .btn-load-intent-source:disabled,
  .btn-run-intent:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .intent-textarea {
    width: 100%;
    height: 104px;
    resize: vertical;
    min-height: 88px;
    max-height: 180px;
    box-sizing: border-box;
    border: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(2, 6, 23, 0.68);
    color: #cbd5e1;
    border-radius: 7px;
    padding: 10px 11px;
    font-size: 0.78rem;
    line-height: 1.55;
    outline: none;
  }

  .intent-textarea:focus {
    border-color: rgba(56, 189, 248, 0.55);
    box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.12);
  }

  .intent-actions-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }

  .intent-mode-tabs {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 3px;
    border: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(15, 23, 42, 0.55);
    border-radius: 7px;
  }

  .intent-mode-tabs button {
    border: none;
    background: transparent;
    color: #64748b;
    border-radius: 5px;
    padding: 5px 9px;
    font: inherit;
    font-size: 0.66rem;
    font-weight: 900;
    cursor: pointer;
  }

  .intent-mode-tabs button.intent-active {
    background: rgba(56, 189, 248, 0.14);
    color: #7dd3fc;
  }

  .intent-inline-error {
    border: 1px solid rgba(245, 158, 11, 0.24);
    background: rgba(120, 53, 15, 0.1);
    color: #fbbf24;
    border-radius: 6px;
    padding: 7px 9px;
    font-size: 0.66rem;
    line-height: 1.4;
  }

  .intent-result-shell {
    border: 1px solid rgba(51, 65, 85, 0.34);
    background: rgba(15, 23, 42, 0.36);
    border-radius: 8px;
    padding: 10px;
    min-height: 126px;
  }

  .intent-result-top {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 7px;
    margin-bottom: 9px;
  }

  .intent-result-top div {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.35);
    background: rgba(2, 6, 23, 0.38);
    border-radius: 6px;
    padding: 7px 8px;
  }

  .intent-result-top span,
  .intent-summary-block span,
  .intent-detail-cell span,
  .intent-action-strip span {
    display: block;
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 900;
    margin-bottom: 4px;
  }

  .intent-result-top strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.76rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .intent-summary-block {
    border: 1px solid rgba(56, 189, 248, 0.16);
    background: rgba(8, 47, 73, 0.12);
    border-radius: 6px;
    padding: 8px 9px;
    margin-bottom: 8px;
  }

  .intent-summary-block p,
  .intent-detail-cell p,
  .intent-action-strip p {
    margin: 0;
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .intent-detail-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .intent-detail-cell {
    min-width: 0;
    min-height: 86px;
    border: 1px solid rgba(51, 65, 85, 0.32);
    border-radius: 6px;
    background: rgba(2, 6, 23, 0.26);
    padding: 8px 9px;
  }

  .intent-detail-cell ul {
    margin: 0;
    padding: 0 0 0 14px;
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.45;
  }

  .intent-detail-cell li + li {
    margin-top: 4px;
  }

  .intent-action-strip {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 8px;
    align-items: start;
    border: 1px solid rgba(16, 185, 129, 0.18);
    background: rgba(6, 78, 59, 0.1);
    border-radius: 6px;
    padding: 8px 9px;
    margin-top: 8px;
  }

  .intent-action-strip span {
    color: #34d399;
    white-space: nowrap;
  }

  .intent-empty-state {
    min-height: 104px;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    color: #64748b;
    font-size: 0.68rem;
    line-height: 1.45;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .btn-deconstruct {
    background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
    color: #ffffff;
    border: none;
    border-radius: 8px;
    padding: 10px 20px;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    box-shadow: 0 4px 12px rgba(79, 70, 229, 0.2);
  }

  .btn-deconstruct:hover {
    background: linear-gradient(135deg, #4338ca 0%, #6d28d9 100%);
    box-shadow: 0 4px 16px rgba(79, 70, 229, 0.35);
    transform: translateY(-1px);
  }

  .btn-deconstruct:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    transform: none;
  }

  .output-panel {
    background: rgba(2, 6, 23, 0.2);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 8px;
    padding: 20px;
    justify-content: center;
    min-height: 250px;
  }

  .empty-state {
    text-align: center;
    padding: 32px 0;
  }

  .empty-main {
    font-size: 0.875rem;
    color: #64748b;
    margin: 0 0 6px 0;
  }

  .empty-sub {
    font-size: 0.75rem;
    color: #475569;
    margin: 0;
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 0;
  }

  .loading-text {
    font-size: 0.875rem;
    color: #94a3b8;
    margin-top: 16px;
    font-weight: 500;
  }

  .spinner-small {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: #ffffff;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    display: inline-block;
  }

  .spinner-large {
    width: 32px;
    height: 32px;
    border: 3px solid rgba(99, 102, 241, 0.2);
    border-top-color: #6366f1;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .result-content {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .result-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .result-section-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #64748b;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .repo-list {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .repo-badge {
    padding: 4px 10px;
    background: #0f172a;
    border: 1px solid rgba(99, 102, 241, 0.2);
    border-radius: 4px;
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    color: #818cf8;
    font-weight: 500;
  }

  .analysis-panel {
    background: rgba(15, 23, 42, 0.46);
    border: 1px solid rgba(51, 65, 85, 0.38);
    border-radius: 8px;
    padding: 14px;
    box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.06);
  }

  .input-analysis-panel {
    margin-top: 18px;
    max-height: 520px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(100, 116, 139, 0.55) transparent;
  }

  .analysis-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 14px;
  }

  .analysis-subtitle {
    margin: 4px 0 0 0;
    color: #64748b;
    font-size: 0.72rem;
    line-height: 1.35;
  }

  .analysis-score-block {
    min-width: 84px;
    text-align: right;
  }

  .analysis-score-row {
    display: flex;
    align-items: flex-start;
    justify-content: flex-end;
    gap: 12px;
    flex-wrap: wrap;
    max-width: 360px;
  }

  .score-label {
    display: block;
    color: #64748b;
    font-size: 0.62rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .score-value {
    display: block;
    color: #38bdf8;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 1.38rem;
    font-weight: 800;
    line-height: 1.1;
    margin-top: 2px;
  }

  .estimate-value,
  .difficulty-value {
    color: #c4b5fd;
    font-size: 0.92rem;
    line-height: 1.2;
    margin-top: 6px;
    white-space: nowrap;
  }

  .score-track {
    height: 6px;
    background: rgba(2, 6, 23, 0.78);
    border: 1px solid rgba(51, 65, 85, 0.42);
    border-radius: 999px;
    margin: 12px 0 10px 0;
    overflow: hidden;
  }

  .score-track span {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, #0f766e 0%, #38bdf8 100%);
    border-radius: inherit;
  }

  .analysis-kpi-row {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 6px;
    margin-bottom: 10px;
  }

  .analysis-kpi-row span {
    min-width: 0;
    background: rgba(2, 6, 23, 0.38);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 6px;
    color: #64748b;
    font-size: 0.66rem;
    padding: 5px 7px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .analysis-kpi-row strong {
    color: #cbd5e1;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-weight: 700;
  }

  .estimate-basis-strip {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 8px;
    align-items: start;
    margin-bottom: 10px;
    padding: 7px 9px;
    background: rgba(30, 41, 59, 0.34);
    border: 1px solid rgba(129, 140, 248, 0.22);
    border-radius: 6px;
    color: #cbd5e1;
    font-size: 0.7rem;
    line-height: 1.45;
  }

  .estimate-basis-label {
    color: #818cf8;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.62rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    white-space: nowrap;
  }

  .analysis-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .analysis-cell {
    min-height: 78px;
    background: rgba(2, 6, 23, 0.36);
    border: 1px solid rgba(51, 65, 85, 0.32);
    border-radius: 6px;
    padding: 9px 10px;
    box-sizing: border-box;
  }

  .analysis-cell.compact {
    min-height: 62px;
  }

  .analysis-cell.risk-cell {
    border-color: rgba(245, 158, 11, 0.22);
    background: rgba(120, 53, 15, 0.08);
  }

  .analysis-cell-title {
    display: block;
    margin-bottom: 6px;
    color: #94a3b8;
    font-size: 0.64rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .analysis-list {
    display: flex;
    flex-direction: column;
    gap: 5px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .analysis-list li {
    position: relative;
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.45;
    padding-left: 10px;
    overflow-wrap: anywhere;
  }

  .analysis-list li::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0.6em;
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #38bdf8;
  }

  .risk-cell .analysis-list li::before {
    background: #f59e0b;
  }

  .analysis-empty {
    margin: 0;
    color: #475569;
    font-size: 0.72rem;
    line-height: 1.45;
  }

  @media (max-width: 640px) {
    .analysis-header {
      flex-direction: column;
    }

    .analysis-score-block {
      width: 100%;
      text-align: left;
    }

    .analysis-score-row {
      width: 100%;
      max-width: none;
      justify-content: flex-start;
    }

    .analysis-kpi-row,
    .analysis-grid,
    .intent-result-top,
    .intent-detail-grid {
      grid-template-columns: 1fr;
    }

    .intent-console-header,
    .intent-actions-row {
      flex-direction: column;
      align-items: stretch;
    }

    .intent-action-strip {
      grid-template-columns: 1fr;
    }
  }

  .task-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-height: 480px;
    overflow-y: auto;
    padding-right: 4px;
  }

  .task-list::-webkit-scrollbar {
    width: 4px;
  }

  .task-list::-webkit-scrollbar-thumb {
    background: rgba(148, 163, 184, 0.2);
    border-radius: 3px;
  }

  .generated-task-card {
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(51, 65, 85, 0.35);
    padding: 16px;
    border-radius: 10px;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.3);
    position: relative;
    overflow: visible;
  }

  .generated-task-card::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(56, 189, 248, 0.2), transparent);
  }

  .generated-task-card:hover {
    border-color: rgba(56, 189, 248, 0.35);
    box-shadow: 0 8px 30px -4px rgba(0, 0, 0, 0.4), 0 0 15px rgba(56, 189, 248, 0.06);
    transform: translateY(-2px);
  }

  .task-card-top-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .delete-task-btn-premium {
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    color: #f87171;
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 0.68rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .delete-task-btn-premium:hover {
    background: rgba(239, 68, 68, 0.22);
    border-color: rgba(239, 68, 68, 0.45);
    color: #fca5a5;
    transform: scale(1.03);
  }

  .icon-trash-premium {
    width: 13px;
    height: 13px;
    transition: transform 0.2s;
  }

  .delete-text-mini {
    font-family: inherit;
  }

  .task-title-container {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(2, 6, 23, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 6px;
    padding: 6px 12px;
    margin-bottom: 14px;
    transition: all 0.2s;
  }

  .task-title-container:focus-within {
    border-color: rgba(56, 189, 248, 0.5);
    box-shadow: 0 0 8px rgba(56, 189, 248, 0.15);
    background: rgba(2, 6, 23, 0.6);
  }

  .edit-icon-indicator {
    font-size: 0.8rem;
    opacity: 0.6;
    transition: opacity 0.2s;
  }

  .task-title-input-premium {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: #f1f5f9;
    font-size: 0.85rem;
    font-weight: 500;
    font-family: inherit;
    padding: 0;
    box-sizing: border-box;
  }

  .task-title-input-premium::placeholder {
    color: #475569;
  }

  .task-bento-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  @media (min-width: 640px) {
    .task-bento-grid {
      grid-template-columns: repeat(3, 1fr);
    }
  }

  .bento-edit-cell {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 6px;
    padding: 6px 10px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    transition: all 0.2s;
  }

  .bento-edit-cell:hover {
    border-color: rgba(51, 65, 85, 0.5);
    background: rgba(15, 23, 42, 0.55);
  }

  .cell-label {
    font-size: 0.62rem;
    color: #64748b;
    font-weight: 600;
    margin-bottom: 2px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .task-estimate-basis {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 8px;
    align-items: start;
    margin-top: 10px;
    padding: 8px 10px;
    background: rgba(2, 6, 23, 0.34);
    border: 1px solid rgba(51, 65, 85, 0.28);
    border-radius: 6px;
  }

  .task-estimate-basis span {
    color: #818cf8;
    font-size: 0.62rem;
    font-weight: 700;
    white-space: nowrap;
  }

  /* 任务组标识样式 */
  .task-meta-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .task-group-badge {
    font-size: 0.62rem;
    color: #38bdf8;
    background: rgba(56, 189, 248, 0.08);
    border: 1px solid rgba(56, 189, 248, 0.2);
    padding: 1px 5px;
    border-radius: 4px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-weight: 600;
  }

  /* 自定义非原生下拉菜单样式 */
  .custom-dropdown-container {
    position: relative;
    width: 100%;
  }

  .custom-dropdown-trigger {
    width: 100%;
    background: rgba(15, 23, 42, 0.25);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 4px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    color: #cbd5e1;
    font-size: 0.7rem;
    font-weight: 500;
    padding: 3px 6px;
    cursor: pointer;
    text-align: left;
    transition: all 0.2s;
  }

  .custom-dropdown-trigger:hover, .custom-dropdown-trigger:focus {
    color: #f1f5f9;
    border-color: rgba(56, 189, 248, 0.4);
    background: rgba(15, 23, 42, 0.45);
  }

  .trigger-value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-right: 6px;
  }

  .trigger-arrow {
    font-size: 0.52rem;
    color: #64748b;
    transition: transform 0.2s, color 0.2s;
    line-height: 1;
  }

  .trigger-arrow.rotated {
    transform: rotate(180deg);
    color: #38bdf8;
  }

  .custom-dropdown-options {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    background: #0d1527;
    border: 1px solid rgba(56, 189, 248, 0.25);
    border-radius: 6px;
    box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.6), 0 0 12px rgba(56, 189, 248, 0.1);
    z-index: 99;
    max-height: 160px;
    overflow-y: auto;
    padding: 4px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .custom-dropdown-options::-webkit-scrollbar {
    width: 5px;
  }

  .custom-dropdown-options::-webkit-scrollbar-thumb {
    background: rgba(56, 189, 248, 0.2);
    border-radius: 3px;
  }

  .dropdown-option-btn {
    width: 100%;
    background: transparent;
    border: none;
    outline: none;
    color: #94a3b8;
    font-size: 0.68rem;
    padding: 5px 8px;
    text-align: left;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .dropdown-option-btn:hover {
    background: rgba(56, 189, 248, 0.08);
    color: #f1f5f9;
  }

  .dropdown-option-btn.selected {
    background: rgba(56, 189, 248, 0.15);
    color: #38bdf8;
    font-weight: 600;
  }

  /* Toast Notification */
  .toast {
    position: fixed;
    bottom: 24px;
    right: 24px;
    z-index: 50;
    padding: 12px 20px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 500;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
    border: 1px solid;
    display: flex;
    align-items: center;
    gap: 8px;
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    transform: translateY(100px);
    opacity: 0;
  }

  .toast.show {
    transform: translateY(0);
    opacity: 1;
  }

  .toast.success {
    background: rgba(16, 185, 129, 0.15);
    border-color: rgba(16, 185, 129, 0.4);
    color: #34d399;
  }

  .toast.error {
    background: rgba(239, 68, 68, 0.15);
    border-color: rgba(239, 68, 68, 0.4);
    color: #f87171;
  }

  .toast.info {
    background: rgba(99, 102, 241, 0.15);
    border-color: rgba(99, 102, 241, 0.4);
    color: #818cf8;
  }

  .mock-alert-banner {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    background: rgba(245, 158, 11, 0.1);
    border: 1px solid rgba(245, 158, 11, 0.3);
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 20px;
    animation: fadeIn 0.3s ease-out;
  }

  .mock-alert-icon {
    font-size: 1.2rem;
    line-height: 1;
    margin-top: 2px;
  }

  .mock-alert-text-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    text-align: left;
  }

  .mock-alert-title {
    font-size: 0.85rem;
    font-weight: 700;
    color: #f59e0b;
    margin: 0;
  }

  .mock-alert-desc {
    font-size: 0.75rem;
    color: #cbd5e1;
    margin: 0;
    line-height: 1.4;
  }

  /* File Upload Dropzone */
  .file-dropzone {
    border: 1px dashed rgba(99, 102, 241, 0.35);
    background: rgba(15, 23, 42, 0.4);
    border-radius: 8px;
    padding: 14px;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s ease;
    margin-bottom: 12px;
    position: relative;
  }

  .file-dropzone:hover, .file-dropzone.dragging {
    border-color: #6366f1;
    background: rgba(30, 41, 59, 0.6);
    box-shadow: 0 0 12px rgba(99, 102, 241, 0.15);
  }

  .file-input {
    position: absolute;
    width: 100%;
    height: 100%;
    top: 0;
    left: 0;
    opacity: 0;
    cursor: pointer;
  }

  .dropzone-label {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    cursor: pointer;
    pointer-events: none;
  }

  .upload-icon {
    font-size: 1.1rem;
  }

  .upload-text {
    font-size: 0.78rem;
    color: #94a3b8;
  }

  .upload-text.text-indigo {
    color: #818cf8;
    font-weight: 600;
  }

  .browse-link {
    color: #6366f1;
    text-decoration: underline;
    font-weight: 500;
  }

  /* Prompt Constraint Panel */
  .prompt-constraint-panel {
    margin-top: 12px;
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.3);
    overflow: hidden;
  }

  .prompt-toggle-btn {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: transparent;
    border: none;
    padding: 10px 14px;
    color: #94a3b8;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s;
  }

  .prompt-toggle-btn:hover {
    background: rgba(255, 255, 255, 0.02);
    color: #e2e8f0;
  }

  .prompt-content {
    padding: 0 14px 14px 14px;
    border-top: 1px solid rgba(51, 65, 85, 0.15);
  }

  .prompt-desc {
    font-size: 0.7rem;
    color: #64748b;
    line-height: 1.4;
    margin: 8px 0;
  }

  .prompt-pre {
    margin: 6px 0 0 0;
    background: rgba(2, 6, 23, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 4px;
    padding: 8px;
    overflow-x: auto;
    font-size: 0.68rem;
    color: #818cf8;
  }

  /* Result Header Row */
  .result-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .sync-actions-group {
    display: flex;
    align-items: center;
    gap: 12px;
    position: relative;
  }

  .deconstruct-demand-link-select {
    width: 220px;
    font-size: 0.72rem;
  }

  .deconstruct-demand-link-select .dropdown-trigger {
    width: 100%;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.25);
    color: #cbd5e1;
    padding: 4px 10px;
    border-radius: 6px;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    align-items: center;
    user-select: none;
    transition: all 0.2s;
    font: inherit;
  }

  .deconstruct-demand-link-select .dropdown-trigger:hover {
    border-color: rgba(99, 102, 241, 0.5);
    background: rgba(30, 41, 59, 0.8);
  }

  .deconstruct-demand-link-select .dropdown-trigger span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 190px;
  }

  .deconstruct-demand-link-select .dropdown-options-list {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 100%;
    max-height: 180px;
    overflow-y: auto;
    background: #0f172a;
    border: 1px solid rgba(129, 140, 248, 0.25);
    border-radius: 8px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
    z-index: 120;
    padding: 4px;
    box-sizing: border-box;
  }

  .deconstruct-demand-link-select .dropdown-option-item {
    width: 100%;
    border: 0;
    background: transparent;
    padding: 6px 10px;
    color: #cbd5e1;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    transition: all 0.15s;
    text-align: left;
  }

  .deconstruct-demand-link-select .dropdown-option-item:hover {
    background: rgba(99, 102, 241, 0.15);
    color: #ffffff;
  }

  .deconstruct-demand-link-select .dropdown-option-item.selected {
    background: rgba(99, 102, 241, 0.35);
    color: #ffffff;
    font-weight: 600;
  }

  .arrow-icon {
    font-size: 0.55rem;
    color: #818cf8;
    transition: transform 0.2s;
  }

  .arrow-icon.open {
    transform: rotate(180deg);
  }

  .btn-sync-kanban {
    background: rgba(16, 185, 129, 0.12);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #34d399;
    border-radius: 6px;
    padding: 4px 12px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .btn-sync-kanban:hover:not(:disabled) {
    background: rgba(16, 185, 129, 0.22);
    border-color: rgba(16, 185, 129, 0.5);
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.15);
  }

  .btn-sync-kanban:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .deconstructor-workbench {
    display: grid;
    gap: var(--wa-space-4);
    margin-bottom: var(--wa-space-6);
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
  }

  .deconstructor-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-4);
    min-height: 58px;
    padding: 10px 12px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-xl);
    background: rgba(255, 255, 255, 0.82);
    box-shadow: var(--wa-shadow-sm);
    backdrop-filter: blur(16px) saturate(124%);
    -webkit-backdrop-filter: blur(16px) saturate(124%);
  }

  .deconstructor-header h2,
  .entry-card-head h3,
  .deconstructor-table-head h3,
  .deconstructor-inspector h3 {
    margin: 3px 0 0;
    color: var(--wa-text-strong);
    font-size: 18px;
    line-height: 1.18;
    font-weight: 840;
  }

  .entry-card-head h3,
  .deconstructor-table-head h3,
  .deconstructor-inspector h3 {
    font-size: 16px;
  }

  .deconstructor-header p,
  .deconstructor-inspector > p {
    margin: 6px 0 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.45;
  }

  .deconstructor-header p {
    display: none;
  }

  .deconstructor-workbench .eyebrow {
    display: block;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.2;
    font-weight: 780;
    letter-spacing: 0;
    text-transform: none;
  }

  .deconstructor-metrics {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-3);
  }

  .deconstructor-metric em {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.35;
    font-style: normal;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .deconstructor-entry-card,
  .deconstructor-table-card {
    min-width: 0;
    padding: var(--wa-space-4);
    display: grid;
    gap: var(--wa-space-3);
  }

  .entry-card-head,
  .deconstructor-table-head,
  .inspector-title-row,
  .deconstruct-progress > div:first-child {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--wa-space-3);
  }

  .entry-grid {
    display: grid;
    grid-template-columns: minmax(0, 0.95fr) minmax(360px, 1.05fr);
    gap: var(--wa-space-4);
    align-items: start;
  }

  .entry-input-column {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-2);
  }

  .input-label {
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 760;
  }

  .demand-textarea,
  .intent-textarea {
    width: 100%;
    box-sizing: border-box;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: rgba(255, 255, 255, 0.84);
    color: var(--wa-text-main);
    font-family: inherit;
    font-size: 13px;
    line-height: 1.55;
    outline: none;
    resize: vertical;
    transition:
      border-color var(--wa-duration-fast) var(--wa-ease),
      box-shadow var(--wa-duration-fast) var(--wa-ease),
      background var(--wa-duration-fast) var(--wa-ease);
  }

  .demand-textarea {
    height: 214px;
    padding: var(--wa-space-3);
  }

  .intent-textarea {
    min-height: 92px;
    max-height: 168px;
    padding: 10px 11px;
  }

  .demand-textarea:focus,
  .intent-textarea:focus {
    border-color: var(--wa-border-focus);
    background: #ffffff;
    box-shadow: 0 0 0 3px var(--wa-accent-soft);
  }

  .demand-textarea:disabled {
    background: var(--wa-surface-inset);
    border-color: var(--wa-border-soft);
    color: var(--wa-text-subtle);
  }

  .ai-disabled-indicator,
  .mock-alert-banner {
    margin: 0;
    display: flex;
    align-items: flex-start;
    gap: var(--wa-space-3);
    border-color: rgba(221, 75, 62, 0.22);
    background: var(--wa-danger-soft);
    color: var(--wa-text-main);
  }

  .mock-alert-banner {
    border-color: rgba(216, 135, 0, 0.24);
    background: var(--wa-warning-soft);
  }

  .warning-text strong,
  .mock-alert-title {
    color: var(--wa-text-strong);
    font-size: 13px;
  }

  .warning-text p,
  .mock-alert-desc {
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.45;
  }

  .status-dot,
  .toast-status-dot {
    width: 8px;
    height: 8px;
    margin-top: 5px;
    border-radius: 999px;
    background: var(--wa-info);
    flex: none;
  }

  .status-dot.danger {
    background: var(--wa-danger);
  }

  .status-dot.warning {
    background: var(--wa-warning);
  }

  .file-dropzone {
    margin: 0;
    border: 1px dashed var(--wa-border-strong);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: var(--wa-space-3);
    box-shadow: none;
  }

  .file-dropzone:hover,
  .file-dropzone.dragging {
    border-color: var(--wa-border-focus);
    background: #ffffff;
    box-shadow: 0 0 0 3px var(--wa-accent-soft);
  }

  .upload-symbol {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 34px;
    height: 24px;
    border-radius: var(--wa-radius-sm);
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
    font-size: 11px;
    font-weight: 820;
    font-family: var(--wa-font-mono);
  }

  .upload-text {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .upload-text.text-indigo,
  .browse-link {
    color: var(--wa-accent-strong);
  }

  .intent-console-panel,
  .intent-result-shell,
  .intent-result-top div,
  .intent-summary-block,
  .intent-detail-cell,
  .intent-action-strip,
  .prompt-constraint-panel,
  .analysis-cell {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-inset);
    box-shadow: none;
  }

  .intent-console-panel {
    margin: 0;
    padding: var(--wa-space-3);
    gap: var(--wa-space-3);
  }

  .intent-console-header p,
  .intent-empty-state,
  .result-section-label,
  .intent-result-top span,
  .intent-summary-block span,
  .intent-detail-cell span,
  .intent-action-strip span,
  .analysis-cell-title,
  .analysis-empty {
    color: var(--wa-text-muted);
  }

  .intent-result-top strong,
  .intent-summary-block p,
  .intent-detail-cell p,
  .intent-detail-cell ul,
  .intent-action-strip p,
  .analysis-list li,
  .analysis-kpi-row strong {
    color: var(--wa-text-main);
  }

  .intent-mode-tabs {
    border-color: var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.78);
  }

  .intent-mode-tabs button {
    color: var(--wa-text-muted);
  }

  .intent-mode-tabs button.intent-active {
    background: var(--wa-accent-soft);
    color: var(--wa-accent-strong);
  }

  .intent-inline-error {
    border-color: rgba(216, 135, 0, 0.24);
    background: var(--wa-warning-soft);
    color: var(--wa-warning);
  }

  .prompt-toggle-btn {
    color: var(--wa-text-main);
  }

  .prompt-toggle-btn:hover {
    background: rgba(255, 255, 255, 0.58);
    color: var(--wa-text-strong);
  }

  .prompt-content {
    border-top-color: var(--wa-border-soft);
  }

  .prompt-desc {
    color: var(--wa-text-muted);
  }

  .prompt-pre {
    background: rgba(255, 255, 255, 0.76);
    border-color: var(--wa-border-soft);
    color: var(--wa-text-main);
  }

  .deconstructor-main-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(330px, 400px);
    gap: var(--wa-space-4);
    align-items: start;
  }

  .deconstructor-table-head {
    align-items: center;
  }

  .deconstructor-count {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  .state-panel {
    min-height: 180px;
    display: grid;
    place-items: center;
    align-content: center;
    gap: var(--wa-space-2);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: var(--wa-space-5);
    text-align: center;
  }

  .state-panel strong {
    color: var(--wa-text-strong);
  }

  .state-panel p,
  .loading-text {
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.45;
  }

  .mapped-repos-section {
    padding: var(--wa-space-2) 0;
  }

  .repo-badge {
    border: 1px solid var(--wa-border-soft);
    background: var(--wa-neutral-soft);
    color: var(--wa-text-main);
    border-radius: var(--wa-radius-sm);
  }

  .deconstructor-table {
    min-width: 1120px;
  }

  .deconstructor-table .align-center,
  .deconstructor-table th.align-center {
    text-align: center;
  }

  .task-title-cell {
    min-width: 0;
    display: grid;
    gap: 5px;
  }

  .task-gen-id {
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 780;
  }

  .task-title-input-premium {
    width: 100%;
    min-width: 0;
    border: 1px solid transparent;
    border-radius: var(--wa-radius-sm);
    background: transparent;
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 650;
    padding: 4px 6px;
  }

  .task-title-input-premium:focus {
    border-color: var(--wa-border-focus);
    background: #ffffff;
    box-shadow: 0 0 0 3px var(--wa-accent-soft);
  }

  .task-title-input-premium::placeholder {
    color: var(--wa-text-subtle);
  }

  .custom-dropdown-trigger,
  .deconstruct-demand-link-select .dropdown-trigger {
    min-height: 30px;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: rgba(255, 255, 255, 0.76);
    color: var(--wa-text-main);
    font-size: 12px;
    padding: 5px 8px;
  }

  .custom-dropdown-trigger:hover,
  .custom-dropdown-trigger:focus,
  .deconstruct-demand-link-select .dropdown-trigger:hover {
    border-color: var(--wa-border-strong);
    background: #ffffff;
    color: var(--wa-text-strong);
  }

  .trigger-arrow,
  .arrow-icon {
    color: var(--wa-text-subtle);
  }

  .trigger-arrow.rotated,
  .arrow-icon.open {
    color: var(--wa-accent-strong);
  }

  .custom-dropdown-options,
  .deconstruct-demand-link-select .dropdown-options-list {
    background: #ffffff;
    border-color: var(--wa-border-soft);
    box-shadow: var(--wa-shadow-sm);
  }

  .dropdown-option-btn,
  .deconstruct-demand-link-select .dropdown-option-item {
    color: var(--wa-text-main);
  }

  .dropdown-option-btn:hover,
  .deconstruct-demand-link-select .dropdown-option-item:hover {
    background: var(--wa-row-hover);
    color: var(--wa-text-strong);
  }

  .dropdown-option-btn.selected,
  .deconstruct-demand-link-select .dropdown-option-item.selected {
    background: var(--wa-row-active);
    color: var(--wa-accent-strong);
  }

  .task-pill-button {
    width: 100%;
    border: 1px solid var(--wa-border-soft);
    cursor: pointer;
    font: inherit;
  }

  .compact-action {
    min-height: 30px;
    padding: 0 10px;
  }

  .deconstructor-inspector {
    position: sticky;
    top: var(--wa-space-4);
  }

  .deconstruct-inspector-facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-2);
  }

  .deconstruct-inspector-facts div,
  .inspector-task-section,
  .inspector-analysis-section,
  .inspector-sync-section,
  .inspector-empty-state {
    min-width: 0;
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-md);
    background: var(--wa-surface-inset);
    padding: var(--wa-space-3);
  }

  .deconstruct-inspector-facts span,
  .deconstruct-progress span,
  .inspector-task-section h4,
  .inspector-analysis-section h4,
  .inspector-sync-section h4 {
    display: block;
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.3;
    font-weight: 760;
  }

  .deconstruct-inspector-facts strong {
    display: block;
    margin-top: 4px;
    color: var(--wa-text-strong);
    font-size: 15px;
  }

  .deconstruct-progress,
  .inspector-task-section,
  .inspector-analysis-section,
  .inspector-sync-section,
  .inspector-empty-state {
    display: grid;
    gap: var(--wa-space-2);
  }

  .deconstruct-progress strong,
  .inspector-task-section > strong {
    color: var(--wa-text-strong);
  }

  .deconstruct-demand-link-select {
    width: 100%;
  }

  .deconstruct-demand-link-select .dropdown-trigger span {
    max-width: 300px;
  }

  .selected-task-pills {
    display: flex;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
  }

  .inspector-task-section p,
  .inspector-analysis-section p {
    margin: 0;
    color: var(--wa-text-main);
    font-size: 13px;
    line-height: 1.5;
  }

  .analysis-kpi-row {
    grid-template-columns: 1fr;
    margin: 0;
  }

  .analysis-kpi-row span {
    background: rgba(255, 255, 255, 0.72);
    border-color: var(--wa-border-soft);
    color: var(--wa-text-muted);
  }

  .inspector-checklist-grid {
    display: grid;
    gap: var(--wa-space-2);
  }

  .analysis-cell {
    min-height: auto;
  }

  .analysis-cell.risk-cell {
    border-color: rgba(216, 135, 0, 0.22);
    background: var(--wa-warning-soft);
  }

  .analysis-list li::before {
    background: var(--wa-accent);
  }

  .toast {
    border-color: var(--wa-border-soft);
    background: rgba(255, 255, 255, 0.94);
    box-shadow: var(--wa-shadow-md);
    color: var(--wa-text-main);
  }

  .toast.success .toast-status-dot {
    background: var(--wa-success);
  }

  .toast.error .toast-status-dot {
    background: var(--wa-danger);
  }

  .toast.info .toast-status-dot {
    background: var(--wa-info);
  }

  @media (max-width: 1180px) {
    .entry-grid,
    .deconstructor-main-grid {
      grid-template-columns: 1fr;
    }

    .deconstructor-inspector {
      position: static;
    }
  }

  @media (max-width: 820px) {
    .deconstructor-header,
    .entry-card-head {
      flex-direction: column;
      align-items: stretch;
    }

    .deconstructor-metrics,
    .deconstruct-inspector-facts {
      grid-template-columns: 1fr;
    }

    .intent-console-header,
    .intent-actions-row {
      flex-direction: column;
      align-items: stretch;
    }
  }
</style>
