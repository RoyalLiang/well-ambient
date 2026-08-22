<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import MarkdownWorkbench, { type MarkdownMode } from './shared/MarkdownWorkbench.svelte';
  import MultiSelect from './shared/MultiSelect.svelte';
  import Select from './shared/Select.svelte';

  export let demand: any;
  export let currentUserPermissions: string[] = [];
  export let currentUserName = '';
  export let currentUserEmail = '';
  export let userDirectory: UserDirectoryEntry[] = [];
  export let coreMemberNames: string[] = [];
  export let coreMemberDirectoryReady = false;

  type UserDirectoryEntry = {
    id: number;
    username: string;
    email?: string;
    name: string;
    department?: string;
  };

  type SelectOption = { value: string; label: string; meta?: string };

  type Spec = {
    id: number;
    demand_id: string;
    version: number;
    status: string;
    source_archive_id: number;
    context_pack_id: number;
    original_text: string;
    intent: string;
    intent_confidence: number;
    summary: string;
    user_goal: string;
    markdown_content: string;
    facts: string[];
    inferences: string[];
    missing_context: string[];
    business_rules: string[];
    main_flows: string[];
    exception_flows: string[];
    permission_rules: string[];
    data_impact: string[];
    api_impact: string[];
    ui_impact: string[];
    dependencies: string[];
    risks: string[];
    acceptance_criteria: string[];
    test_plan: string[];
    mapped_repos: string[];
    tasks: any[];
    readiness_score: number;
    authored_by: string;
    reviewed_by: string;
    frozen_by: string;
    frozen_at?: string;
  };

  type ReviewContract = {
    id: number;
    demand_spec_version_id: number;
    status: string;
    required_roles: string[];
    reviewer_candidates: string[];
    resolved_reviewers: string[];
    acceptance_owner: string;
    minimum_approvals: number;
    protected_path_rules: any[];
    segregation_rules: string[];
    review_sla_hours: number;
    escalation_owner: string;
    resolution_status: string;
    resolution_reason: string;
  };

  type FileAction = { action: 'create' | 'update'; path: string; content: string; encoding?: string };
  type GeneratedChangeSet = { summary: string; change_set: FileAction[]; test_commands: string[]; model?: string };
  type ExecutionRun = {
    id: number;
    status: string;
    repo: string;
    topic_branch: string;
    base_branch: string;
    pipeline_status: string;
    acceptance_state: string;
    resolved_reviewers_json: string;
    mr_url: string;
    mr_state: string;
    block_reason: string;
    actions?: any[];
  };

  let loadedDemandId = '';
  let loading = false;
  let actionLoading = '';
  let error = '';
  let notice = '';
  let specs: Spec[] = [];
  let spec: Spec | null = null;
  let contract: ReviewContract | null = null;
  let runs: ExecutionRun[] = [];
  let generated: GeneratedChangeSet | null = null;
  let sourcePathsText = '';
  let generationInstructions = '';
  let streamedMarkdown = '';
  let generationPhase = '';
  let generationMessage = '';
  let streamScrollOwner: HTMLElement | null = null;
  let streamFollowEnabled = true;
  let streamFollowFrame = 0;
  let markdownText = '';
  let markdownMode: MarkdownMode = 'preview';

  let summaryText = '';
  let goalText = '';
  let acceptanceText = '';
  let testPlanText = '';
  let reposText = '';
  let missingContextText = '';
  let riskText = '';
  let businessRulesText = '';
  let mainFlowsText = '';
  let exceptionFlowsText = '';
  let permissionRulesText = '';
  let dataImpactText = '';
  let apiImpactText = '';
  let uiImpactText = '';
  let dependenciesText = '';
  let confirmWithdrawDraft = false;
  let reviewerCandidates: string[] = [];
  let requiredRoles: string[] = [];
  let acceptanceOwnerText = '';
  let minimumApprovals = 1;
  let escalationOwnerText = '';
  let normalizedDirectoryKey = '';
  let contractParticipants: UserDirectoryEntry[] = [];

  const roleOptions: SelectOption[] = [
    { value: 'code_owner', label: '代码负责人', meta: 'Code owner' },
    { value: 'qa', label: '测试与质量', meta: 'QA' },
    { value: 'product_owner', label: '产品负责人', meta: 'Product owner' },
    { value: 'architecture', label: '架构负责人', meta: 'Architecture' },
    { value: 'security', label: '安全负责人', meta: 'Security' },
    { value: 'dba', label: '数据库负责人', meta: 'DBA' },
    { value: 'ops', label: '运维负责人', meta: 'Operations' }
  ];

  $: personOptions = buildPersonOptions(reviewDirectoryEntries(userDirectory, contractParticipants, coreMemberNames, coreMemberDirectoryReady));
  $: roleSelectOptions = [
    ...roleOptions,
    ...requiredRoles
      .filter((role) => !roleOptions.some((option) => option.value === role))
      .map((role) => ({ value: role, label: role, meta: '自定义角色' }))
  ];
  $: if (minimumApprovals < 1) minimumApprovals = 1;
  $: if (minimumApprovals > Math.max(1, reviewerCandidates.length)) {
    minimumApprovals = Math.max(1, reviewerCandidates.length);
  }
  $: directoryNormalizationKey = contract && personOptions.length
    ? `${contract.id}:${personOptions.map((option) => option.value.toLowerCase()).join('\u001f')}`
    : '';
  $: if (contract && directoryNormalizationKey && normalizedDirectoryKey !== directoryNormalizationKey) {
    normalizedDirectoryKey = directoryNormalizationKey;
    fillContractForm(contract);
  }
  $: if (actionLoading === 'ai-draft' && streamedMarkdown) {
    queueStreamFollow();
  }

  $: latestRun = runs[0] || null;
  $: canRead = hasPermission('demand_spec:read');
  $: canWrite = hasPermission('demand_spec:write');
  $: canFreeze = hasPermission('demand_spec:freeze');
  $: canManageReview = hasPermission('review_contract:manage');
  $: canExecute = hasPermission('execution:start');
  $: canAccept = hasPermission('execution:accept');
  $: if (demand?.task_id && demand.task_id !== loadedDemandId) {
    loadedDemandId = demand.task_id;
    void loadDeliveryWorkspace();
  }

  onMount(() => {
    if (demand?.task_id && demand.task_id !== loadedDemandId) {
      loadedDemandId = demand.task_id;
      void loadDeliveryWorkspace();
    }
  });

  onDestroy(() => {
    if (streamFollowFrame) cancelAnimationFrame(streamFollowFrame);
  });

  function findVerticalScrollOwner(node: HTMLElement) {
    let parent = node.parentElement;
    while (parent) {
      const overflowY = getComputedStyle(parent).overflowY;
      if (overflowY === 'auto' || overflowY === 'scroll' || overflowY === 'overlay') return parent;
      parent = parent.parentElement;
    }
    return document.scrollingElement instanceof HTMLElement ? document.scrollingElement : document.documentElement;
  }

  function isNearStreamBottom(owner: HTMLElement) {
    return owner.scrollHeight - owner.scrollTop - owner.clientHeight <= 96;
  }

  function queueStreamFollow() {
    if (!streamFollowEnabled || !streamScrollOwner) return;
    void tick().then(() => {
      if (!streamFollowEnabled || !streamScrollOwner) return;
      if (streamFollowFrame) cancelAnimationFrame(streamFollowFrame);
      streamFollowFrame = requestAnimationFrame(() => {
        streamFollowFrame = 0;
        if (!streamFollowEnabled || !streamScrollOwner) return;
        streamScrollOwner.scrollTop = streamScrollOwner.scrollHeight;
      });
    });
  }

  function autoFollowStream(node: HTMLElement) {
    const owner = findVerticalScrollOwner(node);
    const updateFollowState = () => {
      streamFollowEnabled = isNearStreamBottom(owner);
    };

    streamScrollOwner = owner;
    streamFollowEnabled = true;
    owner.addEventListener('scroll', updateFollowState, { passive: true });
    queueStreamFollow();

    return {
      destroy() {
        owner.removeEventListener('scroll', updateFollowState);
        if (streamScrollOwner === owner) streamScrollOwner = null;
        if (streamFollowFrame) {
          cancelAnimationFrame(streamFollowFrame);
          streamFollowFrame = 0;
        }
      }
    };
  }

  function hasPermission(permission: string) {
    return currentUserPermissions.includes(permission);
  }

  function lines(value: string) {
    return Array.from(new Set(value.split(/\r?\n|,/).map(item => item.trim()).filter(Boolean)));
  }

  function buildPersonOptions(entries: UserDirectoryEntry[]): SelectOption[] {
    const seen = new Set<string>();
    return entries.flatMap((entry) => {
      const value = entry.name?.trim();
      if (!value || seen.has(value.toLowerCase())) return [];
      seen.add(value.toLowerCase());
      const context = [entry.department?.trim(), entry.email?.trim() || entry.username?.trim()].filter(Boolean).join(' · ');
      return [{ value, label: value, meta: context }];
    });
  }

  function identityVariants(value: string) {
    const normalized = value.trim().toLowerCase();
    if (!normalized) return [];
    const variants = new Set([normalized]);
    const emailPrefix = normalized.includes('@') ? normalized.split('@')[0] : '';
    const firstToken = normalized.split(/\s+/)[0];
    if (emailPrefix) variants.add(emailPrefix);
    if (firstToken) variants.add(firstToken);
    return Array.from(variants);
  }

  function isCoreMemberEntry(entry: UserDirectoryEntry, allowedNames: string[]) {
    const allowed = new Set(allowedNames.flatMap(identityVariants));
    if (!allowed.size) return false;
    return [entry.name, entry.username, entry.email || '']
      .flatMap(identityVariants)
      .some((identity) => allowed.has(identity));
  }

  function reviewDirectoryEntries(
    pageDirectory: UserDirectoryEntry[],
    scopedDirectory: UserDirectoryEntry[],
    allowedNames: string[],
    directoryReady: boolean
  ) {
    if (!directoryReady) return scopedDirectory;
    return [...pageDirectory, ...scopedDirectory].filter((entry) => isCoreMemberEntry(entry, allowedNames));
  }

  function validPerson(value: string) {
    const normalized = value.trim().toLowerCase();
    return personOptions.find((option) => option.value.toLowerCase() === normalized)?.value || '';
  }

  function defaultReviewers(value: ReviewContract) {
    if (!personOptions.length) return value.reviewer_candidates || [];
    const saved = (value.reviewer_candidates || []).map(validPerson).filter(Boolean);
    if (saved.length) return Array.from(new Set(saved));
    const taskAssignees = (spec?.tasks || []).map((task) => validPerson(task?.assignee || '')).filter(Boolean);
    const author = (spec?.authored_by || currentUserName).trim().toLowerCase();
    return Array.from(new Set(taskAssignees.filter((name) => name && name.toLowerCase() !== author)));
  }

  function fillSpecForm(value: Spec) {
    summaryText = value.summary || '';
    goalText = value.user_goal || '';
    acceptanceText = (value.acceptance_criteria || []).join('\n');
    testPlanText = (value.test_plan || []).join('\n');
    reposText = (value.mapped_repos || []).join('\n');
    missingContextText = (value.missing_context || []).join('\n');
    riskText = (value.risks || []).join('\n');
    businessRulesText = (value.business_rules || []).join('\n');
    mainFlowsText = (value.main_flows || []).join('\n');
    exceptionFlowsText = (value.exception_flows || []).join('\n');
    permissionRulesText = (value.permission_rules || []).join('\n');
    dataImpactText = (value.data_impact || []).join('\n');
    apiImpactText = (value.api_impact || []).join('\n');
    uiImpactText = (value.ui_impact || []).join('\n');
    dependenciesText = (value.dependencies || []).join('\n');
    markdownText = value.markdown_content?.trim() || formMarkdown();
    markdownMode = 'preview';
  }

  function markdownList(title: string, values: string[]) {
    if (!values.length) return `## ${title}\n\n_暂无内容_`;
    return `## ${title}\n\n${values.map(value => `- ${value}`).join('\n')}`;
  }

  function formMarkdown() {
    return [
      `# ${summaryText.trim() || demand?.title || '需求实现规格'}`,
      goalText.trim() ? `> ${goalText.trim()}` : '',
      markdownList('业务规则', lines(businessRulesText)),
      markdownList('主流程', lines(mainFlowsText).map((value, index) => `${index + 1}. ${value}`)),
      markdownList('异常与回退', lines(exceptionFlowsText)),
      markdownList('权限与安全边界', lines(permissionRulesText)),
      markdownList('数据影响', lines(dataImpactText)),
      markdownList('API 影响', lines(apiImpactText)),
      markdownList('界面与交互影响', lines(uiImpactText)),
      markdownList('依赖项', lines(dependenciesText)),
      markdownList('风险', lines(riskText)),
      markdownList('验收标准', lines(acceptanceText)),
      markdownList('测试计划', lines(testPlanText)),
      markdownList('目标仓库', lines(reposText)),
      markdownList('待确认事项', lines(missingContextText))
    ].filter(Boolean).join('\n\n');
  }

  function fillContractForm(value: ReviewContract) {
    reviewerCandidates = defaultReviewers(value);
    requiredRoles = value.required_roles || [];
    acceptanceOwnerText = personOptions.length
      ? validPerson(value.acceptance_owner || '') || validPerson(demand?.assignee || '') || validPerson(currentUserName)
      : value.acceptance_owner || '';
    minimumApprovals = value.minimum_approvals || 1;
    escalationOwnerText = personOptions.length
      ? validPerson(value.escalation_owner || '') || validPerson(demand?.creator || '') || validPerson(currentUserName)
      : value.escalation_owner || '';
  }

  async function api(path: string, init: RequestInit = {}) {
    const response = await fetch(path, init);
    if (!response.ok) {
      const contentType = response.headers.get('content-type') || '';
      if (contentType.includes('application/json')) {
        const body = await response.json();
        throw new Error(body.blockers?.join('；') || body.error || body.message || `HTTP ${response.status}`);
      }
      throw new Error((await response.text()) || `HTTP ${response.status}`);
    }
    return response.json();
  }

  async function loadDeliveryWorkspace() {
    if (!canRead || !demand?.task_id) return;
    loading = true;
    error = '';
    try {
      const [specResponse, runResponse] = await Promise.all([
        api(`/api/demand-specs?demand_id=${encodeURIComponent(demand.task_id)}`),
        api(`/api/execution/runs?demand_id=${encodeURIComponent(demand.task_id)}`)
      ]);
      specs = specResponse.items || [];
      runs = runResponse.items || [];
      spec = specs[0] || null;
      if (spec) {
        fillSpecForm(spec);
        const contractResponse = await api(`/api/review-contracts?demand_spec_version_id=${spec.id}`);
        contractParticipants = contractResponse.participants || [];
        contract = contractResponse.review_contract || null;
        if (contract) fillContractForm(contract);
      } else {
        contract = null;
      }
    } catch (err: any) {
      error = err.message || '加载受控交付状态失败';
    } finally {
      loading = false;
    }
  }

  async function createDraft(useAI: boolean) {
    if (!canWrite) return;
    actionLoading = useAI ? 'ai-draft' : 'manual-draft';
    error = '';
    notice = '';
    streamedMarkdown = '';
    streamFollowEnabled = true;
    generationPhase = useAI ? 'preparing' : '';
    generationMessage = useAI ? '正在建立安全的流式连接…' : '';
    try {
      const demandText = [demand.title, demand.description].filter(Boolean).join('\n');
      const payload: any = {
        demand_id: demand.task_id,
        original_text: demandText,
        summary: demand.title,
        user_goal: demand.description || demand.title,
        mapped_repos: demand.repo && demand.repo !== '-' ? [demand.repo] : [],
        readiness_score: 40,
        acceptance_criteria: [],
        test_plan: []
      };
      if (useAI) {
        await createAIDraft(demandText);
        notice = 'AI 草案已生成并保存，请预览后进入编辑或审核。';
        return;
      }
      const result = await api('/api/demand-specs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
      });
      spec = result.spec;
      contract = result.review_contract;
      fillSpecForm(spec!);
      fillContractForm(contract!);
      notice = '人工草案已建立。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '创建规格草案失败';
    } finally {
      actionLoading = '';
      generationPhase = '';
      generationMessage = '';
    }
  }

  async function createAIDraft(demandText: string) {
    const response = await fetch('/api/demand-specs/ai-stream', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/x-ndjson' },
      body: JSON.stringify({ demand_id: demand.task_id, original_text: demandText })
    });
    if (!response.ok) throw new Error((await response.text()) || `HTTP ${response.status}`);
    if (!response.body) throw new Error('当前浏览器不支持流式响应');

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let completed = false;
    const consume = (line: string) => {
      if (!line.trim()) return;
      const event = JSON.parse(line);
      if (event.type === 'markdown_delta') streamedMarkdown += event.delta || '';
      if (event.phase) generationPhase = event.phase;
      if (event.message) generationMessage = event.message;
      if (event.type === 'error') throw new Error(event.message || 'AI 草案生成失败');
      if (event.type === 'complete') {
        spec = event.spec;
        contract = event.review_contract;
        specs = spec ? [spec, ...specs.filter(item => item.id !== spec?.id)] : specs;
        if (spec) fillSpecForm(spec);
        if (contract) fillContractForm(contract);
        completed = true;
      }
    };
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const records = buffer.split('\n');
      buffer = records.pop() || '';
      for (const record of records) consume(record);
      if (done) break;
    }
    if (buffer.trim()) consume(buffer);
    if (!completed) throw new Error('流式连接已结束，但草案尚未完成落库');
    await loadDeliveryWorkspace();
  }

  function specPayload() {
    if (!spec) return null;
    return {
      ...spec,
      summary: summaryText.trim(),
      user_goal: goalText.trim(),
      markdown_content: markdownText.trim() || formMarkdown(),
      acceptance_criteria: lines(acceptanceText),
      test_plan: lines(testPlanText),
      mapped_repos: lines(reposText),
      missing_context: lines(missingContextText),
      risks: lines(riskText),
      business_rules: lines(businessRulesText),
      main_flows: lines(mainFlowsText),
      exception_flows: lines(exceptionFlowsText),
      permission_rules: lines(permissionRulesText),
      data_impact: lines(dataImpactText),
      api_impact: lines(apiImpactText),
      ui_impact: lines(uiImpactText),
      dependencies: lines(dependenciesText)
    };
  }

  async function withdrawDraft() {
    if (!spec || spec.status !== 'draft' || !canWrite) return;
    actionLoading = 'withdraw-draft'; error = ''; notice = '';
    try {
      await api(`/api/demand-specs/${spec.id}`, { method: 'DELETE' });
      spec = null;
      contract = null;
      generated = null;
      confirmWithdrawDraft = false;
      notice = '规格草案已撤销，你可以重新选择 AI 生成或人工建立。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '撤销规格草案失败';
    } finally {
      actionLoading = '';
    }
  }

  async function saveSpec() {
    const payload = specPayload();
    if (!payload || spec?.status !== 'draft') return false;
    actionLoading = 'save-spec'; error = ''; notice = '';
    try {
      const result = await api('/api/demand-specs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
      });
      spec = result.spec;
      fillSpecForm(spec!);
      notice = '规格草案已保存。';
      return true;
    } catch (err: any) {
      error = err.message || '保存规格失败';
      return false;
    } finally { actionLoading = ''; }
  }

  async function temporarilySaveMarkdown() {
    if (actionLoading || markdownMode !== 'edit') return;
    const saved = await saveSpec();
    if (saved) {
      notice = 'Markdown 草案已临时保存。';
    } else {
      markdownMode = 'edit';
    }
  }

  async function saveContract() {
    if (!contract || contract.status === 'approved') return false;
    actionLoading = 'save-contract'; error = ''; notice = '';
    try {
      const result = await api('/api/review-contracts', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: contract.id,
          demand_spec_version_id: contract.demand_spec_version_id,
          required_roles: requiredRoles,
          reviewer_candidates: reviewerCandidates,
          acceptance_owner: acceptanceOwnerText.trim(),
          minimum_approvals: Number(minimumApprovals) || 1,
          protected_path_rules: contract.protected_path_rules || [],
          segregation_rules: ['author_cannot_self_approve'],
          review_sla_hours: contract.review_sla_hours || 24,
          escalation_owner: escalationOwnerText.trim()
        })
      });
      contract = result.review_contract;
      fillContractForm(contract!);
      notice = '审核契约已保存。';
      return true;
    } catch (err: any) {
      error = err.message || '保存审核契约失败';
      return false;
    } finally { actionLoading = ''; }
  }

  async function approveContract() {
    if (!contract) return;
    actionLoading = 'approve-contract'; error = ''; notice = '';
    try {
      if (!(await saveContract())) return;
      const result = await api(`/api/review-contracts/${contract.id}/approve`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      contract = result.review_contract;
      fillContractForm(contract!);
      notice = '审核契约已批准，可以冻结规格。';
    } catch (err: any) {
      error = err.message || '批准审核契约失败';
    } finally { actionLoading = ''; }
  }

  async function freezeSpec() {
    if (!spec) return;
    actionLoading = 'freeze'; error = ''; notice = '';
    try {
      if (!(await saveSpec())) return;
      const result = await api(`/api/demand-specs/${spec.id}/freeze`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      spec = result.spec;
      contract = result.review_contract;
      fillSpecForm(spec!);
      notice = '规格已冻结，后续执行将绑定此版本。';
    } catch (err: any) {
      error = err.message || '冻结规格失败';
    } finally { actionLoading = ''; }
  }

  async function generateChangeSet() {
    if (!spec || spec.status !== 'frozen') return;
    actionLoading = 'generate'; error = ''; notice = ''; generated = null;
    try {
      generated = await api('/api/execution/change-set/generate', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          demand_spec_version_id: spec.id,
          repo: lines(reposText)[0],
          source_paths: lines(sourcePathsText),
          instructions: generationInstructions.trim()
        })
      });
      notice = `AI 已生成 ${generated?.change_set.length || 0} 个受限文件动作。`;
    } catch (err: any) {
      error = err.message || '生成代码变更失败';
    } finally { actionLoading = ''; }
  }

  async function createAndStartRun() {
    if (!spec || !generated) return;
    actionLoading = 'execute'; error = ''; notice = '';
    try {
      const created = await api('/api/execution/runs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          preflight: {
            demand_spec_version_id: spec.id,
            repo: lines(reposText)[0],
            author: currentUserName || currentUserEmail,
            change_set: generated.change_set,
            test_commands: generated.test_commands
          }
        })
      });
      const run = created.run;
      const started = await api(`/api/execution/runs/${run.id}/start`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      notice = started.run?.mr_url ? 'Draft MR 已创建，等待 CI 与人工审核。' : '执行运行已启动。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '启动受控执行失败';
      await loadDeliveryWorkspace();
    } finally { actionLoading = ''; }
  }

  async function refreshRun() {
    if (!latestRun) return;
    actionLoading = 'refresh'; error = ''; notice = '';
    try {
      await api(`/api/execution/runs/${latestRun.id}/refresh`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      notice = 'CI 状态已刷新。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '刷新运行失败';
    } finally { actionLoading = ''; }
  }

  async function verifyRun(decision: 'accepted' | 'rejected') {
    if (!latestRun) return;
    actionLoading = `verify-${decision}`; error = ''; notice = '';
    try {
      await api(`/api/execution/runs/${latestRun.id}/verify`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ decision })
      });
      notice = decision === 'accepted' ? '人工验收已通过，等待 MR 合并证据。' : '本次交付已被人工拒绝。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '提交人工验收失败';
    } finally { actionLoading = ''; }
  }

  async function cancelRun() {
    if (!latestRun) return;
    actionLoading = 'cancel'; error = ''; notice = '';
    try {
      await api(`/api/execution/runs/${latestRun.id}/cancel`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
      });
      notice = '执行运行已取消，审计记录保留。';
      await loadDeliveryWorkspace();
    } catch (err: any) {
      error = err.message || '取消运行失败';
    } finally { actionLoading = ''; }
  }

  function displayStatus(value: string) {
    const labels: Record<string, string> = {
      draft: '草案', frozen: '已冻结', approved: '已批准', preflight_ready: '预检通过',
      executing: '执行中', review_pending: '待审核', acceptance_pending: '待合并', delivered: '已交付',
      tests_failed: 'CI 失败', execution_failed: '执行失败', rejected: '已拒绝', cancelled: '已取消'
    };
    return labels[value] || value || '未开始';
  }

  function runTone(value: string) {
    if (['delivered', 'success', 'frozen', 'approved'].includes(value)) return 'success';
    if (['tests_failed', 'execution_failed', 'rejected'].includes(value)) return 'danger';
    if (['executing', 'review_pending', 'acceptance_pending', 'pending'].includes(value)) return 'info';
    return 'neutral';
  }
</script>

{#if canRead}
  <section class="delivery-control" aria-label="受控自治交付">
    <header class="delivery-control-head">
      <div>
        <span class="delivery-kicker">规格与执行</span>
        <h4>AI 规格草案</h4>
        <p>先把需求变成可审核、可测试、可追溯的实现规格，再进入代码生成与交付。</p>
      </div>
      <div class="delivery-state-strip">
        <span class="state-pill {spec ? runTone(spec.status) : 'neutral'}">规格 {spec ? `v${spec.version} · ${displayStatus(spec.status)}` : '未建立'}</span>
        <span class="state-pill {contract ? runTone(contract.status) : 'neutral'}">契约 {contract ? displayStatus(contract.status) : '未建立'}</span>
        <span class="state-pill {latestRun ? runTone(latestRun.status) : 'neutral'}">执行 {latestRun ? displayStatus(latestRun.status) : '未开始'}</span>
      </div>
    </header>

    {#if loading}
      <div class="delivery-empty loading">正在读取规格、审核契约与执行证据…</div>
    {:else}
      {#if error}<div class="delivery-message error">{error}</div>{/if}
      {#if notice}<div class="delivery-message success">{notice}</div>{/if}

      {#if !spec}
        <div class="delivery-empty">
          <strong>尚未建立可执行需求规格</strong>
          <p>可以先让 AI 识别意图并解构，也可以直接建立人工草案。</p>
          {#if canWrite}
            <div class="delivery-actions">
              <button class="primary" disabled={!!actionLoading} on:click={() => createDraft(true)}>{actionLoading === 'ai-draft' ? 'AI 分析中…' : '生成AI草案'}</button>
              <button disabled={!!actionLoading} on:click={() => createDraft(false)}>建立人工草案</button>
            </div>
          {/if}
        </div>
        {#if actionLoading === 'ai-draft'}
          <div
            class="ai-draft-stream"
            aria-live="polite"
            aria-busy="true"
            data-stream-follow={streamFollowEnabled ? 'active' : 'paused'}
            use:autoFollowStream
          >
            <div class="stream-head">
              <div class="stream-orbit" aria-hidden="true"><span></span></div>
              <div><strong>{generationMessage || 'AI 正在编写实现规格'}</strong><small>{generationPhase === 'persisting' ? '正在保存草案与审核契约' : '内容将随模型生成实时出现'}</small></div>
              <span class="live-pill"><i></i> LIVE</span>
            </div>
            {#if streamedMarkdown}
              <MarkdownWorkbench
                value={streamedMarkdown}
                mode="preview"
                readonly={true}
                label="AI 实时草案"
                showToolbar={false}
                streaming={true}
                minHeight={360}
                autoHeight={true}
              />
            {:else}
              <div class="stream-skeleton" aria-hidden="true"><span></span><span></span><span></span><span></span></div>
            {/if}
          </div>
        {/if}
      {:else}
        <div class="delivery-stage specification-stage">
          <div class="stage-head"><span>规格</span><div><strong>实现规格文档</strong><small>{spec.status === 'draft' && canWrite ? '双击正文进入编辑，点击文档外空白区域临时保存并退出' : '已锁定为只读 Markdown 文档'}</small></div></div>

          <MarkdownWorkbench
            bind:value={markdownText}
            bind:mode={markdownMode}
            readonly={spec.status !== 'draft' || !canWrite}
            label={`规格 v${spec.version}`}
            showToolbar={false}
            inlineEditing={spec.status === 'draft' && canWrite}
            saving={actionLoading === 'save-spec'}
            minHeight={360}
            autoHeight={true}
            on:save={temporarilySaveMarkdown}
            on:commit={temporarilySaveMarkdown}
          />
          <div class="stage-meta">
            <span>意图：{spec.intent || '需求解构'}</span><span>就绪度：{spec.readiness_score}%</span><span>Context Pack：#{spec.context_pack_id || '-'}</span>
          </div>
          {#if spec.status === 'draft' && canWrite && !contract}
            <div class="delivery-actions draft-actions">
              <button class="danger" disabled={!!actionLoading} on:click={() => confirmWithdrawDraft = true}>撤销草案</button>
            </div>
            {#if confirmWithdrawDraft}
              <div class="withdraw-confirm" role="alert">
                <div><strong>撤销当前草案？</strong><span>仅删除未冻结的规格与审核契约，不影响原需求和影子任务。</span></div>
                <div class="delivery-actions">
                  <button disabled={!!actionLoading} on:click={() => confirmWithdrawDraft = false}>继续编辑</button>
                  <button class="danger" disabled={!!actionLoading} on:click={withdrawDraft}>{actionLoading === 'withdraw-draft' ? '撤销中…' : '确认撤销'}</button>
                </div>
              </div>
            {/if}
          {/if}
        </div>

        {#if contract}
          <div class="delivery-stage review-contract-stage">
            <div class="review-stage-header">
              <div class="stage-head"><span>审核</span><div><strong>确认审核契约</strong><small>锁定审核人与批准策略后，再按真实 Diff 解析最终 Reviewer</small></div></div>
              <div class="review-contract-meta" aria-label="审核契约摘要">
                <span class="status">{displayStatus(contract.status)}</span>
                <span>至少 {minimumApprovals} 人批准</span>
                <span>{contract.review_sla_hours || 24}h SLA</span>
              </div>
            </div>
            <div class="review-contract-form">
              <div class="review-field span-two">
                <MultiSelect
                  id="reviewer-candidates"
                  label="候选 Reviewer"
                  bind:values={reviewerCandidates}
                  options={personOptions}
                  disabled={contract.status === 'approved'}
                  required={true}
                  placeholder="选择候选审核人"
                  searchPlaceholder="搜索姓名、邮箱或部门"
                  emptyText="用户目录中没有匹配人员"
                  helperText="仅显示核心成员；可连续选择，完成后点击空白处收起"
                  compact={true}
                />
              </div>
              <div class="review-field span-two">
                <MultiSelect
                  id="required-review-roles"
                  label="必须审核角色"
                  bind:values={requiredRoles}
                  options={roleSelectOptions}
                  disabled={contract.status === 'approved'}
                  placeholder="选择职责角色"
                  searchPlaceholder="搜索审核角色"
                  compact={true}
                />
              </div>
              <div class="review-field">
                <Select
                  id="acceptance-owner"
                  label="业务验收人"
                  bind:value={acceptanceOwnerText}
                  options={personOptions}
                  disabled={contract.status === 'approved'}
                  required={true}
                  placeholder="选择业务验收人"
                  searchPlaceholder="搜索姓名、邮箱或部门"
                  emptyText="用户目录中没有匹配人员"
                  clearable={true}
                  compact={true}
                />
              </div>
              <div class="review-field">
                <Select
                  id="escalation-owner"
                  label="升级负责人"
                  bind:value={escalationOwnerText}
                  options={personOptions}
                  disabled={contract.status === 'approved'}
                  placeholder="选择升级负责人"
                  searchPlaceholder="搜索姓名、邮箱或部门"
                  emptyText="用户目录中没有匹配人员"
                  clearable={true}
                  compact={true}
                />
              </div>
              <label class="review-number-field">
                <span>最少批准数</span>
                <input type="number" min="1" max={Math.max(1, reviewerCandidates.length)} bind:value={minimumApprovals} disabled={contract.status === 'approved'} />
                <small>不超过已选择的候选审核人数</small>
              </label>
              <div class="review-policy-note">
                <span>职责分离</span>
                <strong>作者不可自审</strong>
                <small>系统会在真实 Diff 生成后解析最终 Reviewer，并自动排除规格作者。</small>
              </div>
            </div>
            {#if contract.resolved_reviewers?.length}
              <div class="resolved-line">最终 Reviewer：{contract.resolved_reviewers.join('、')} · {contract.resolution_reason}</div>
            {/if}

            {#if confirmWithdrawDraft && spec.status === 'draft' && canWrite}
              <div class="withdraw-confirm" role="alert">
                <div><strong>撤销当前草案？</strong><span>仅删除未冻结的规格与审核契约，不影响原需求和影子任务。</span></div>
                <div class="delivery-actions">
                  <button disabled={!!actionLoading} on:click={() => confirmWithdrawDraft = false}>继续编辑</button>
                  <button class="danger" disabled={!!actionLoading} on:click={withdrawDraft}>{actionLoading === 'withdraw-draft' ? '撤销中…' : '确认撤销'}</button>
                </div>
              </div>
            {/if}

            {#if (spec.status === 'draft' && canWrite) || (contract.status !== 'approved' && canManageReview) || (contract.status === 'approved' && spec.status === 'draft' && canFreeze)}
              <div class="delivery-actions review-actions">
                <div class="review-actions-main">
                  {#if spec.status === 'draft' && canWrite}
                    <button class="danger" disabled={!!actionLoading} on:click={() => confirmWithdrawDraft = true}>撤销草案</button>
                  {/if}
                  {#if contract.status !== 'approved' && canManageReview}
                    <button disabled={!!actionLoading} on:click={saveContract}>保存契约</button>
                    <button class="primary" disabled={!!actionLoading} on:click={approveContract}>{actionLoading === 'approve-contract' ? '批准中…' : '批准审核契约'}</button>
                  {:else if contract.status === 'approved' && spec.status === 'draft' && canFreeze}
                    <button class="primary" disabled={!!actionLoading} on:click={freezeSpec}>{actionLoading === 'freeze' ? '冻结中…' : '冻结规格并开放执行'}</button>
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        {/if}

        {#if spec.status === 'frozen'}
          <div class="delivery-stage">
            <div class="stage-head"><span>执行</span><div><strong>生成代码并受控交付</strong><small>只允许结构化文件动作，测试由 GitLab Pipeline 执行</small></div></div>
            {#if !latestRun || ['cancelled', 'rejected', 'execution_failed', 'tests_failed'].includes(latestRun.status)}
              <div class="delivery-grid two">
                <label><span>允许读取/更新的源文件路径</span><textarea rows="4" bind:value={sourcePathsText} placeholder="internal/server/handler.go"></textarea></label>
                <label><span>补充生成指令</span><textarea rows="4" bind:value={generationInstructions} placeholder="说明要修改的行为，不填写 shell 命令"></textarea></label>
              </div>
              <div class="delivery-actions">
                <button disabled={!!actionLoading} on:click={generateChangeSet}>{actionLoading === 'generate' ? 'AI 生成中…' : 'AI 生成代码变更'}</button>
                {#if generated && canExecute}<button class="primary" disabled={!!actionLoading} on:click={createAndStartRun}>{actionLoading === 'execute' ? '创建中…' : '创建分支、提交与 Draft MR'}</button>{/if}
              </div>
              {#if generated}
                <div class="change-set-preview">
                  <strong>{generated.summary}</strong>
                  {#each generated.change_set as change}<div><span>{change.action}</span><code>{change.path}</code></div>{/each}
                  <small>CI：{generated.test_commands.join(' · ')}</small>
                </div>
              {/if}
            {/if}

            {#if latestRun}
              <div class="run-ledger">
                <div><span>运行状态</span><strong>{displayStatus(latestRun.status)}</strong></div>
                <div><span>Pipeline</span><strong>{displayStatus(latestRun.pipeline_status)}</strong></div>
                <div><span>人工验收</span><strong>{displayStatus(latestRun.acceptance_state)}</strong></div>
                <div><span>分支</span><code>{latestRun.topic_branch || '-'}</code></div>
              </div>
              {#if latestRun.block_reason}<div class="run-blocker">{latestRun.block_reason}</div>{/if}
              <div class="delivery-actions">
                {#if latestRun.mr_url}<a class="delivery-link" href={latestRun.mr_url} target="_blank" rel="noopener noreferrer">打开 Draft MR</a>{/if}
                {#if !['delivered', 'rejected', 'cancelled'].includes(latestRun.status)}<button disabled={!!actionLoading} on:click={refreshRun}>刷新 CI</button>{/if}
                {#if canAccept && latestRun.pipeline_status === 'success' && latestRun.acceptance_state !== 'accepted'}
                  <button class="primary" disabled={!!actionLoading} on:click={() => verifyRun('accepted')}>人工验收通过</button>
                  <button class="danger" disabled={!!actionLoading} on:click={() => verifyRun('rejected')}>拒绝交付</button>
                {/if}
                {#if hasPermission('execution:cancel') && !['delivered', 'rejected', 'cancelled'].includes(latestRun.status)}<button disabled={!!actionLoading} on:click={cancelRun}>取消运行</button>{/if}
              </div>
              <div class="audit-line">已记录 {latestRun.actions?.length || 0} 个执行动作；MR 合并后由 GitLab Webhook 完成交付闭环。</div>
            {/if}
          </div>
        {/if}
      {/if}
    {/if}
  </section>
{/if}

<style>
  .delivery-control { min-width: 0; display: grid; color: var(--wa-text-main, #293847); }
  .delivery-control-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding-bottom: 16px; }
  .delivery-kicker { display: block; margin-bottom: 5px; color: var(--wa-accent-strong, #006f76); font-size: 11px; line-height: 1.2; font-weight: 750; letter-spacing: .04em; }
  h4 { margin: 0; color: var(--wa-text-strong, #17212b); font-size: 16px; letter-spacing: -.02em; }
  .delivery-control-head p { margin: 5px 0 0; color: #687784; font-size: 12px; }
  .delivery-state-strip { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 6px; }
  .state-pill { border: 1px solid #dce3e8; border-radius: 999px; padding: 5px 9px; background: #f7f9fa; color: #586773; font-size: 11px; }
  .state-pill.success { border-color: rgba(110, 204, 84, .38); background: var(--wa-success-soft, rgba(110, 204, 84, .16)); color: var(--wa-success, #2f742a); }
  .state-pill.info { border-color: rgba(0, 47, 167, .22); background: var(--wa-info-soft, rgba(0, 47, 167, .1)); color: var(--wa-info-strong, #0d3a69); }
  .state-pill.danger { border-color: rgba(200, 22, 29, .22); background: var(--wa-danger-soft, rgba(211, 73, 71, .12)); color: var(--wa-danger, #c8161d); }
  .delivery-stage { min-width: 0; display: grid; gap: 16px; padding: 16px 0; border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); }
  .specification-stage { gap: 16px; }
  .stream-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
  .stage-head { display: flex; gap: 8px; align-items: center; }
  .stage-head > span { display: grid; place-items: center; min-width: 40px; height: 28px; border-radius: 8px; padding: 0 8px; background: var(--wa-accent-soft, rgba(113, 226, 209, .22)); color: var(--wa-accent-strong, #006f76); font-size: 11px; line-height: 1; font-weight: 750; }
  .stage-head div { display: grid; gap: 2px; }
  .stage-head strong { font-size: 13px; }
  .stage-head small { color: #75828d; font-size: 11px; }
  .ai-draft-stream { display: grid; gap: 16px; padding: 16px 0; border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); }
  .stream-head { justify-content: flex-start; }
  .stream-head > div:nth-child(2) { display: grid; flex: 1; gap: 2px; }
  .stream-head strong { color: #1e3a40; font-size: 13px; }
  .stream-head small { color: #71828a; font-size: 11px; }
  .stream-orbit { position: relative; width: 30px; height: 30px; border: 1px solid rgba(42, 143, 131, .25); border-radius: 50%; background: rgba(255,255,255,.82); }
  .stream-orbit::before { content: ''; position: absolute; inset: 5px; border: 2px solid #b7ddd8; border-top-color: #21847a; border-radius: 50%; animation: stream-spin .9s linear infinite; }
  .stream-orbit span { position: absolute; inset: 12px; border-radius: 50%; background: #21847a; }
  .live-pill { display: inline-flex; align-items: center; gap: 6px; border: 1px solid #b8ddd7; border-radius: 999px; padding: 5px 8px; background: #eef8f6; color: #22766c; font: 750 10px/1 ui-monospace, monospace; letter-spacing: .04em; }
  .live-pill i { width: 6px; height: 6px; border-radius: 50%; background: #2a9487; box-shadow: 0 0 0 4px rgba(42, 148, 135, .12); animation: live-pulse 1.4s ease-in-out infinite; }
  .stream-skeleton { display: grid; gap: 11px; padding: 24px 0; }
  .stream-skeleton span { height: 9px; border-radius: 999px; background: #e4ecee; animation: skeleton-pulse 1.25s ease-in-out infinite alternate; }
  .stream-skeleton span:nth-child(1) { width: 42%; height: 18px; }
  .stream-skeleton span:nth-child(2) { width: 92%; }
  .stream-skeleton span:nth-child(3) { width: 78%; }
  .stream-skeleton span:nth-child(4) { width: 58%; }
  .delivery-grid { display: grid; gap: 12px; }
  .delivery-grid.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .span-two { grid-column: 1 / -1; }
  label { display: grid; gap: 8px; min-width: 0; }
  label > span { color: #667580; font-size: 11px; font-weight: 650; }
  textarea, input { width: 100%; box-sizing: border-box; border: 1px solid #ccd6dd; border-radius: 8px; background: #fff; color: #1f2a33; padding: 8px 12px; font-family: inherit; font-size: 13px; line-height: 1.45; outline: none; resize: vertical; transition: border-color 150ms ease, box-shadow 150ms ease; }
  textarea { min-height: 68px; max-height: 132px; }
  textarea:focus, input:focus { border-color: #55a99f; box-shadow: 0 0 0 3px rgba(38, 139, 127, .11); }
  textarea:disabled, input:disabled { background: #f4f6f7; color: #53616c; resize: none; }
  .review-contract-stage { gap: 20px; padding-block: 20px; }
  .review-stage-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
  .review-contract-meta { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px; }
  .review-contract-meta span { min-height: 28px; display: inline-flex; align-items: center; border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); border-radius: 999px; padding: 0 8px; background: #f7fafb; color: #61717d; font-size: 10px; font-weight: 650; }
  .review-contract-meta .status { border-color: rgba(113, 226, 209, .52); background: var(--wa-accent-soft, rgba(113, 226, 209, .22)); color: var(--wa-accent-strong, #006f76); }
  .review-contract-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 20px; padding-block: 4px; }
  .review-field { min-width: 0; }
  .review-number-field { align-content: start; }
  .review-number-field input { min-height: var(--wa-control-h, 38px); resize: none; }
  .review-number-field small { color: #7a8791; font-size: 10px; line-height: 1.4; }
  .review-policy-note { min-width: 0; display: grid; align-content: center; grid-template-columns: auto minmax(0, 1fr); gap: 3px 10px; border: 1px solid rgba(169, 214, 207, .64); border-radius: 12px; padding: 10px 12px; background: rgba(238, 248, 246, .56); color: #687681; }
  .review-policy-note > span { color: #73818c; font-size: 10px; font-weight: 700; letter-spacing: .03em; }
  .review-policy-note strong { color: #2b4d4a; font-size: 12px; }
  .review-policy-note small { grid-column: 1 / -1; color: #74828d; font-size: 10px; line-height: 1.45; }
  .review-actions { justify-content: flex-end; padding-top: 4px; border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, .18)); }
  .review-actions-main { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px; margin-left: auto; }
  .stage-meta, .resolved-line, .audit-line { display: flex; flex-wrap: wrap; gap: 12px; color: #687681; font-size: 11px; }
  .resolved-line { padding: 8px 10px; background: var(--wa-success-soft, rgba(110, 204, 84, .16)); color: var(--wa-success, #2f742a); }
  .delivery-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  button, .delivery-link { min-height: 38px; border: 1px solid #cbd5dc; border-radius: 10px; background: #fff; color: #34434e; padding: 0 14px; font-family: inherit; font-size: 12px; line-height: 36px; font-weight: 700; cursor: pointer; text-decoration: none; transition: background 150ms ease, border-color 150ms ease, transform 120ms ease; }
  button:hover, .delivery-link:hover { border-color: #8fa8b4; background: #f7fafb; }
  button:active, .delivery-link:active { transform: scale(.98); }
  button:focus-visible, .delivery-link:focus-visible { outline: 3px solid rgba(1, 139, 141, .18); outline-offset: 1px; }
  button:disabled { opacity: .56; cursor: wait; }
  button.primary { border-color: var(--wa-accent-fill, #006f76); background: var(--wa-accent-fill, #006f76); color: var(--wa-accent-fill-ink, #f6fbff); }
  button.primary:hover { border-color: var(--wa-accent-fill-hover, #00545a); background: var(--wa-accent-fill-hover, #00545a); color: var(--wa-accent-fill-ink, #f6fbff); }
  button.danger { border-color: rgba(200, 22, 29, .24); color: var(--wa-danger, #c8161d); background: var(--wa-danger-soft, rgba(211, 73, 71, .12)); }
  .draft-actions { justify-content: flex-end; }
  .withdraw-confirm { display: flex; align-items: center; justify-content: space-between; gap: 16px; border: 1px solid #efc9c5; border-radius: 12px; background: #fff7f5; padding: 12px; color: #7e3f3b; }
  .withdraw-confirm > div:first-child { display: grid; gap: 3px; }
  .withdraw-confirm strong { font-size: 12px; }
  .withdraw-confirm span { color: #895d58; font-size: 11px; line-height: 1.45; }
  .delivery-empty { border: 1px dashed #cbd6dc; border-radius: 12px; padding: 18px; text-align: center; color: #6d7b86; background: #f9fbfb; }
  .delivery-empty strong { display: block; color: #2b3943; margin-bottom: 4px; }
  .delivery-empty p { margin: 0 0 12px; font-size: 12px; }
  .delivery-empty .delivery-actions { justify-content: center; }
  .delivery-empty.loading { animation: pulse 1.4s ease-in-out infinite; }
  .delivery-message { margin-bottom: 12px; padding: 9px 11px; border: 1px solid; font-size: 12px; }
  .delivery-message.error { border-color: rgba(200, 22, 29, .24); background: var(--wa-danger-soft, rgba(211, 73, 71, .12)); color: var(--wa-danger, #c8161d); }
  .delivery-message.success { border-color: rgba(110, 204, 84, .38); background: var(--wa-success-soft, rgba(110, 204, 84, .16)); color: var(--wa-success, #2f742a); }
  .change-set-preview { display: grid; gap: 6px; padding: 11px; background: #f5f8f9; border: 1px solid #dde4e8; }
  .change-set-preview > div { display: grid; grid-template-columns: 58px minmax(0, 1fr); align-items: center; gap: 8px; }
  .change-set-preview span { color: #1b7f74; font: 700 10px/1 ui-monospace, monospace; text-transform: uppercase; }
  code { overflow-wrap: anywhere; color: #344852; font: 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; }
  .change-set-preview small { color: #71808a; }
  .run-ledger { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid #dde4e8; }
  .run-ledger > div { min-width: 0; padding: 10px; border-right: 1px solid #e3e8eb; display: grid; gap: 4px; }
  .run-ledger > div:last-child { border-right: 0; }
  .run-ledger span { color: #7a8791; font-size: 10px; }
  .run-ledger strong { font-size: 12px; }
  .run-blocker { padding: 9px 11px; background: #fff4f3; color: #944444; font-size: 11px; }
  @keyframes pulse { 50% { opacity: .6; } }
  @keyframes stream-spin { to { transform: rotate(360deg); } }
  @keyframes live-pulse { 50% { opacity: .45; } }
  @keyframes skeleton-pulse { to { opacity: .48; } }
  @media (max-width: 900px) {
    .delivery-control-head { display: grid; }
    .delivery-state-strip { justify-content: flex-start; }
    .delivery-grid.two, .run-ledger { grid-template-columns: 1fr; }
    .run-ledger > div { border-right: 0; border-bottom: 1px solid #e3e8eb; }
    .withdraw-confirm { align-items: stretch; flex-direction: column; }
    .review-stage-header { align-items: stretch; flex-direction: column; gap: 12px; }
    .review-contract-meta { justify-content: flex-start; }
    button, .delivery-link { min-height: 44px; line-height: 42px; }
  }
  @media (max-width: 560px) {
    .review-contract-form { grid-template-columns: 1fr; gap: 14px; }
    .review-contract-form .span-two { grid-column: auto; }
    .review-policy-note { min-height: 52px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .stream-orbit::before, .live-pill i, .stream-skeleton span { animation: none; }
  }
</style>
