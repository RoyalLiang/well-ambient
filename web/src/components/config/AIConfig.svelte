<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import { resetSettingsWorkspaceScroll } from '../../lib/settings-ui';

  const dispatch = createEventDispatcher();

  export let config: {
    enabled: boolean;
    provider: string;
    base_url: string;
    endpoint_type?: string;
    api_token: string;
    model: string;
    project_architecture?: string;
    delivery_workflow?: string;
    implemented_features?: string;
    estimation_guidelines?: string;
    default_work_hours_per_day?: number;
  } = {
    enabled: false,
    provider: 'openai',
    base_url: '',
    endpoint_type: 'completions',
    api_token: '',
    model: '',
    project_architecture: '',
    delivery_workflow: '',
    implemented_features: '',
    estimation_guidelines: '',
    default_work_hours_per_day: 8
  };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let lastUpdated = '';
  export let view: 'engine' | 'context' | 'all' = 'all';

  type ContextFactID = number | string;

  interface ContextFact {
    id?: ContextFactID;
    type: string;
    scope: string;
    scope_id: string;
    source: string;
    owner: string;
    status: string;
    version?: number | string;
    content_hash?: string;
    summary: string;
    content: string;
    token_count?: number;
    freshness?: number;
    confidence?: number;
    created_at?: string;
    updated_at?: string;
  }

  interface ContextFactForm {
    type: string;
    scope: string;
    scope_id: string;
    source: string;
    owner: string;
    status: string;
    version: number;
    summary: string;
    content: string;
    freshness: number;
    confidence: number;
  }

  interface ContextPackPreviewItem {
    id?: ContextFactID;
    type: string;
    scope: string;
    scope_id: string;
    summary: string;
    content: string;
    token_count?: number;
    score?: number;
    reason?: string;
  }

  interface ContextPackPreview {
    id?: ContextFactID;
    summary: string;
    token_count?: number;
    budget_tokens?: number;
    cache_key?: string;
    items: ContextPackPreviewItem[];
  }

  const contextFactTypes = [
    { value: 'architecture', label: '架构设计' },
    { value: 'workflow', label: '流程设计' },
    { value: 'feature_boundary', label: '功能边界' },
    { value: 'estimation_rule', label: '估算口径' },
    { value: 'glossary', label: '术语口径' },
    { value: 'risk_rule', label: '风险规则' },
    { value: 'delivery_history', label: '交付样本' }
  ];

  const contextFactScopes = [
    { value: 'global', label: '全局' },
    { value: 'repo', label: '仓库' },
    { value: 'module', label: '模块' },
    { value: 'demand_type', label: '需求类型' }
  ];

  const contextFactSources = [
    { value: 'manual', label: '手工录入' },
    { value: 'config', label: '配置迁移' },
    { value: 'gitlab', label: 'GitLab' },
    { value: 'jira', label: 'Jira' },
    { value: 'archive', label: '历史归档' },
    { value: 'doc', label: '文档' }
  ];

  const contextFactStatuses = [
    { value: 'active', label: '生效' },
    { value: 'draft', label: '草稿' },
    { value: 'paused', label: '暂停' },
    { value: 'retired', label: '归档' }
  ];

  let currentStep = 1;
  const steps = ['连接与凭证', '确认应用'];
  let editing = false;
  let showTokenEditor = !config.api_token;

  // Form states
  let enabled = config.enabled ?? false;
  let provider = config.provider || 'openai';
  let baseURL = config.base_url || '';
  let endpointType = config.endpoint_type || 'completions';
  let apiToken = config.api_token || '';
  let modelName = config.model || '';
  let projectArchitecture = config.project_architecture || '';
  let deliveryWorkflow = config.delivery_workflow || '';
  let implementedFeatures = config.implemented_features || '';
  let estimationGuidelines = config.estimation_guidelines || '';
  let defaultWorkHoursPerDay = config.default_work_hours_per_day || 8;

  // Test states
  let testing = false;
  let testError = '';
  let testSuccess = '';
  let testDetails = '';

  let contextFacts: ContextFact[] = [];
  let contextFactsLoading = false;
  let contextFactsError = '';
  let contextFactSaving = false;
  let contextFactSaveError = '';
  let contextFactSaveSuccess = '';
  let editingContextFactId: ContextFactID | null = null;
  let contextFactForm: ContextFactForm = createBlankContextFactForm();
  let previewDemand = '';
  let contextPreviewLoading = false;
  let contextPreviewError = '';
  let contextPackPreview: ContextPackPreview | null = null;

  $: showEnginePanel = view !== 'context';
  $: showContextPanel = view !== 'engine';
  $: isConfigured = enabled || !!(baseURL || apiToken || modelName);
  $: activeContextFactCount = contextFacts.filter(fact => fact.status === 'active').length;
  $: totalContextFactTokens = contextFacts.reduce((sum, fact) => sum + (Number(fact.token_count) || 0), 0);
  let configurationIssues: string[] = [];
  let healthTone = 'unchecked';
  let healthLabel = '未检测';
  let healthMessage = '尚未执行健康检查。';

  $: configurationIssues = enabled
    ? [
        ...(!baseURL ? ['API 请求地址'] : []),
        ...(!apiToken ? ['API Key'] : [])
      ]
    : [];
  $: healthTone = saveSuccess
    ? 'success'
    : !isConfigured
      ? 'incomplete'
      : !enabled
        ? 'unchecked'
        : testing
          ? 'checking'
          : testError
            ? 'error'
            : configurationIssues.length > 0
              ? 'incomplete'
              : testSuccess
                ? 'success'
                : 'unchecked';
  $: healthLabel = saveSuccess
    ? '配置已保存'
    : !isConfigured
      ? '尚未配置'
      : !enabled
        ? '引擎已停用'
        : testing
          ? '检测中'
          : testError
            ? '检测失败'
            : configurationIssues.length > 0
              ? '配置不完整'
              : testSuccess
                ? '检测通过'
                : '未检测';
  $: healthMessage = saveSuccess
    ? 'AI 引擎连接与估算参数已写入新的配置版本。'
    : !isConfigured
      ? '尚未录入模型服务信息。进入编辑后完成连接与凭证配置。'
      : !enabled
        ? '配置已保留，需求解构当前不会调用外部模型服务。'
        : testing
          ? '正在验证模型端点、认证凭证和协议响应。'
          : testError
            ? testError
            : configurationIssues.length > 0
              ? `需要补齐：${configurationIssues.join('、')}。`
              : testSuccess
                ? testSuccess
                : '尚未执行健康检查，不默认判定为已就绪。';
  $: if (!editing && !saveSuccess) {
    enabled = config.enabled ?? false;
    provider = config.provider || 'openai';
    baseURL = config.base_url || '';
    endpointType = config.endpoint_type || 'completions';
    apiToken = config.api_token || '';
    modelName = config.model || '';
    projectArchitecture = config.project_architecture || '';
    deliveryWorkflow = config.delivery_workflow || '';
    implementedFeatures = config.implemented_features || '';
    estimationGuidelines = config.estimation_guidelines || '';
    defaultWorkHoursPerDay = config.default_work_hours_per_day || 8;
    showTokenEditor = !config.api_token;
  }

  onMount(() => {
    fetchContextFacts();
  });

  function formatUpdated(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  function openEditor() {
    editing = true;
    currentStep = 1;
    testError = '';
    testSuccess = '';
    testDetails = '';
    resetSettingsWorkspaceScroll();
  }

  function finishClose() {
    editing = false;
    dispatch('close');
  }

  function normalizedWorkHours() {
    const hours = Number(defaultWorkHoursPerDay);
    return Number.isFinite(hours) && hours > 0 ? hours : 8;
  }

  async function testConnection() {
    if (!baseURL || !apiToken) {
      testError = '请填写接口请求地址与 API Key 凭证';
      return;
    }

    testing = true;
    testError = '';
    testSuccess = '';
    testDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'ai',
          ai: {
            enabled: enabled,
            provider: provider,
            base_url: baseURL,
            endpoint_type: endpointType,
            api_token: apiToken,
            model: modelName,
            project_architecture: projectArchitecture,
            delivery_workflow: deliveryWorkflow,
            implemented_features: implementedFeatures,
            estimation_guidelines: estimationGuidelines,
            default_work_hours_per_day: normalizedWorkHours()
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        testSuccess = 'AI 接口连通性测试成功！';
        testDetails = data.details || '';
      } else {
        testError = data.message;
        testDetails = data.details || '';
      }
    } catch (e: any) {
      testError = '连通性测试请求失败，请检查网络或后端状态';
      testDetails = e.message;
    } finally {
      testing = false;
    }
  }

  async function saveConfig(isToggle = false) {
    const updatedAI = {
      enabled: enabled,
      provider: provider,
      base_url: baseURL,
      endpoint_type: endpointType,
      api_token: apiToken,
      model: modelName,
      project_architecture: projectArchitecture,
      delivery_workflow: deliveryWorkflow,
      implemented_features: implementedFeatures,
      estimation_guidelines: estimationGuidelines,
      default_work_hours_per_day: normalizedWorkHours()
    };

    dispatch('save', {
      key: 'ai',
      data: updatedAI,
      isToggle: isToggle
    });
  }

  function nextStep() {
    if (currentStep === 1) {
      if (enabled && (!baseURL || !apiToken)) {
        testError = '启用 AI 解构引擎时，必须填写接口请求地址及 API Key 凭证';
        return;
      }
      currentStep = 2;
    }
  }

  function prevStep() {
    if (currentStep > 1) {
      currentStep -= 1;
    }
  }

  function contextStatus(value: string) {
    return value.trim() ? '已配置' : '未配置';
  }

  function createBlankContextFactForm(): ContextFactForm {
    return {
      type: 'architecture',
      scope: 'global',
      scope_id: '',
      source: 'manual',
      owner: '',
      status: 'active',
      version: 1,
      summary: '',
      content: '',
      freshness: 0.85,
      confidence: 0.85
    };
  }

  function normalizeContextFact(raw: any): ContextFact {
    return {
      id: raw?.id ?? raw?.fact_id ?? raw?.uuid,
      type: raw?.type || 'architecture',
      scope: raw?.scope || 'global',
      scope_id: raw?.scope_id ?? raw?.scopeId ?? '',
      source: raw?.source || 'manual',
      owner: raw?.owner || '',
      status: raw?.status || 'active',
      version: raw?.version,
      content_hash: raw?.content_hash ?? raw?.contentHash,
      summary: raw?.summary || raw?.title || '',
      content: raw?.content || raw?.text || raw?.body || raw?.summary || '',
      token_count: raw?.token_count ?? raw?.tokenCount,
      freshness: raw?.freshness,
      confidence: raw?.confidence,
      created_at: raw?.created_at ?? raw?.createdAt,
      updated_at: raw?.updated_at ?? raw?.updatedAt
    };
  }

  function normalizeContextFactList(payload: any): ContextFact[] {
    const list = Array.isArray(payload)
      ? payload
      : (payload?.facts || payload?.items || payload?.data || payload?.context_facts || []);
    return Array.isArray(list) ? list.map(normalizeContextFact) : [];
  }

  function contextFactKey(fact: ContextFact) {
    return String(fact.id ?? fact.content_hash ?? `${fact.type}:${fact.scope}:${fact.scope_id}:${fact.summary}`);
  }

  function labelFor(options: Array<{ value: string; label: string }>, value: string) {
    return options.find(option => option.value === value)?.label || value || '未定义';
  }

  function setContextFactField<K extends keyof ContextFactForm>(field: K, value: ContextFactForm[K]) {
    contextFactForm = { ...contextFactForm, [field]: value };
    if (field === 'scope' && value === 'global') {
      contextFactForm = { ...contextFactForm, scope_id: '' };
    }
  }

  function compactScope(fact: { scope: string; scope_id?: string }) {
    if (fact.scope === 'global') return 'global';
    return `${fact.scope}:${fact.scope_id || '未指定'}`;
  }

  function formatScore(value?: number) {
    const score = Number(value);
    if (!Number.isFinite(score)) return '未评分';
    return `${Math.round(score * 100)}%`;
  }

  function beginCreateContextFact() {
    editingContextFactId = null;
    contextFactForm = createBlankContextFactForm();
    contextFactSaveError = '';
    contextFactSaveSuccess = '';
  }

  function beginEditContextFact(fact: ContextFact) {
    editingContextFactId = fact.id ?? contextFactKey(fact);
    contextFactForm = {
      type: fact.type || 'architecture',
      scope: fact.scope || 'global',
      scope_id: fact.scope_id || '',
      source: fact.source || 'manual',
      owner: fact.owner || '',
      status: fact.status || 'active',
      version: Number(fact.version) || 1,
      summary: fact.summary || '',
      content: fact.content || '',
      freshness: Number(fact.freshness) || 0.85,
      confidence: Number(fact.confidence) || 0.85
    };
    contextFactSaveError = '';
    contextFactSaveSuccess = '';
  }

  function buildContextFactPayload() {
    const freshness = Number(contextFactForm.freshness);
    const confidence = Number(contextFactForm.confidence);
    return {
      id: editingContextFactId ?? undefined,
      type: contextFactForm.type,
      scope: contextFactForm.scope,
      scope_id: contextFactForm.scope === 'global' ? '' : contextFactForm.scope_id.trim(),
      source: contextFactForm.source,
      owner: contextFactForm.owner.trim(),
      status: contextFactForm.status,
      version: Number(contextFactForm.version) || 1,
      summary: contextFactForm.summary.trim(),
      content: contextFactForm.content.trim(),
      freshness: Number.isFinite(freshness) ? freshness : 0.85,
      confidence: Number.isFinite(confidence) ? confidence : 0.85
    };
  }

  async function parseJSONResponse(res: Response) {
    const text = await res.text();
    if (!text) return {};
    try {
      return JSON.parse(text);
    } catch (e) {
      return { message: text };
    }
  }

  async function fetchContextFacts() {
    contextFactsLoading = true;
    contextFactsError = '';
    try {
      const res = await fetch('/api/context/facts');
      const data = await parseJSONResponse(res);
      if (!res.ok) throw new Error(data.message || `HTTP ${res.status}`);
      contextFacts = normalizeContextFactList(data);
    } catch (e: any) {
      contextFactsError = e.message || '系统设计语料加载失败';
    } finally {
      contextFactsLoading = false;
    }
  }

  async function saveContextFact() {
    contextFactSaveError = '';
    contextFactSaveSuccess = '';
    if (!contextFactForm.summary.trim() || !contextFactForm.content.trim()) {
      contextFactSaveError = '请填写事实摘要和事实内容';
      return;
    }
    if (contextFactForm.scope !== 'global' && !contextFactForm.scope_id.trim()) {
      contextFactSaveError = '非全局事实需要填写 Scope ID';
      return;
    }

    const payload = buildContextFactPayload();
    const method = editingContextFactId ? 'PUT' : 'POST';
    contextFactSaving = true;
    try {
      let res = await fetch('/api/context/facts', {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!res.ok && editingContextFactId && [404, 405].includes(res.status)) {
        res = await fetch(`/api/context/facts/${encodeURIComponent(String(editingContextFactId))}`, {
          method,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
      }

      const data = await parseJSONResponse(res);
      if (!res.ok || data.success === false) {
        throw new Error(data.message || data.error || `HTTP ${res.status}`);
      }
      contextFactSaveSuccess = editingContextFactId ? '系统设计资料已更新' : '系统设计资料已创建';
      await fetchContextFacts();
      if (!editingContextFactId) beginCreateContextFact();
    } catch (e: any) {
      contextFactSaveError = e.message || '系统设计资料保存失败';
    } finally {
      contextFactSaving = false;
    }
  }

  function normalizePreviewItem(raw: any): ContextPackPreviewItem {
    if (typeof raw === 'string') {
      return {
        type: 'context',
        scope: 'global',
        scope_id: '',
        summary: raw,
        content: raw
      };
    }
    return {
      id: raw?.id ?? raw?.fact_id ?? raw?.item_id,
      type: raw?.type || raw?.fact_type || 'context',
      scope: raw?.scope || 'global',
      scope_id: raw?.scope_id ?? raw?.scopeId ?? '',
      summary: raw?.summary || raw?.title || raw?.content || '',
      content: raw?.content || raw?.text || raw?.summary || '',
      token_count: raw?.token_count ?? raw?.tokenCount,
      score: raw?.score ?? raw?.rank_score,
      reason: raw?.reason || raw?.match_reason || ''
    };
  }

  function normalizeContextPackPreview(payload: any): ContextPackPreview {
    const raw = payload?.pack || payload?.preview || payload?.data || payload;
    const items = raw?.items || raw?.facts || raw?.selected_facts || raw?.context_items || payload?.items || [];
    return {
      id: raw?.id ?? raw?.pack_id ?? payload?.context_pack_id,
      summary: raw?.summary || raw?.pack_summary || payload?.summary || '',
      token_count: raw?.token_count ?? raw?.tokenCount ?? payload?.token_count,
      budget_tokens: raw?.budget_tokens ?? raw?.budgetTokens ?? payload?.budget_tokens,
      cache_key: raw?.cache_key ?? raw?.cacheKey,
      items: Array.isArray(items) ? items.map(normalizePreviewItem) : []
    };
  }

  async function previewContextPack() {
    contextPreviewError = '';
    contextPackPreview = null;
    if (!previewDemand.trim()) {
      contextPreviewError = '请输入一段需求文本后再预览上下文包';
      return;
    }

    contextPreviewLoading = true;
    try {
      const res = await fetch('/api/context/pack/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          demand_text: previewDemand.trim(),
          text: previewDemand.trim(),
          model: modelName,
          provider,
          work_hours_per_day: normalizedWorkHours()
        })
      });
      const data = await parseJSONResponse(res);
      if (!res.ok || data.success === false) {
        throw new Error(data.message || data.error || `HTTP ${res.status}`);
      }
      contextPackPreview = normalizeContextPackPreview(data);
    } catch (e: any) {
      contextPreviewError = e.message || '上下文包预览失败';
    } finally {
      contextPreviewLoading = false;
    }
  }
</script>

<div class="scw-workbench">
  {#if showEnginePanel && saveSuccess}
    <Alert type="success" title="配置已保存" message="AI 引擎连接与估算参数已更新，版本审计会记录本次变更。" />
  {/if}

  {#if showEnginePanel && !editing && isConfigured}
    <section class="scw-overview" aria-label="AI 引擎配置状态">
      <header class="scw-header">
        <div>
          <span class="scw-kicker">模型配置</span>
          <h4>AI 引擎配置状态</h4>
          <p>只读摘要集中展示模型端点、凭证与估算参数，健康状态只来自真实检测或配置完整性判断。</p>
        </div>
        <div class="scw-toggle">
          <span>启用 AI 引擎</span>
          <Switch id="ai-overview-toggle" label="启用 AI 引擎" bind:checked={enabled} on:change={() => saveConfig(true)} />
        </div>
      </header>

      <div class="scw-status tone-{healthTone}">
        <div class="scw-status-main">
          <span>服务健康</span>
          <strong>{healthLabel}</strong>
        </div>
        <p>{healthMessage}</p>
      </div>

      <div class="scw-read-grid">
        <div class="scw-read-item">
          <span>AI 提供商</span>
          <strong class="font-sans">{provider || 'openai'}</strong>
        </div>
        <div class="scw-read-item">
          <span>接口基础 URL 地址</span>
          <strong class="font-mono">{baseURL || '-'}</strong>
        </div>
        <div class="scw-read-item">
          <span>大语言模型 (Model)</span>
          <strong class="font-mono">{modelName || '未指定'}</strong>
        </div>
        <div class="scw-read-item">
          <span>接口端点类型 (Endpoint Type)</span>
          <strong class="font-mono">{endpointType || 'completions'}</strong>
        </div>
        <div class="scw-read-item">
          <span>API 凭证 (Token)</span>
          <strong>{apiToken ? '已配置，已脱敏' : '未配置'}</strong>
        </div>
        <div class="scw-read-item">
          <span>每日折算工时</span>
          <strong class="font-mono">{defaultWorkHoursPerDay || 8} 小时/天</strong>
        </div>
        <div class="scw-read-item">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
      </div>

      <div class="scw-metrics three">
        <div class="scw-metric"><span>生效语料</span><strong>{activeContextFactCount} 条</strong></div>
        <div class="scw-metric"><span>语料规模</span><strong>{totalContextFactTokens || 0} tokens</strong></div>
        <div class="scw-metric"><span>Pack 预览</span><strong>{contextPackPreview ? '已生成' : '待生成'}</strong></div>
      </div>

      {#if testDetails}
        <pre class="scw-details scw-mono">{testDetails}</pre>
      {/if}

      <div class="scw-actions">
        <Button variant="secondary" loading={testing} on:click={testConnection}>健康检查</Button>
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </section>
  {:else if showEnginePanel}
    <section class="scw-editor" aria-label="编辑 AI 引擎配置">
      <div class="scw-stepper" aria-label="配置步骤">
        {#each steps as step, index}
          {@const stepNum = index + 1}
          <button
            type="button"
            class:active={currentStep === stepNum}
            class:completed={currentStep > stepNum}
            on:click={() => currentStep = stepNum}
          >
            <span class="scw-step-index">{stepNum}</span>
            {step}
          </button>
        {/each}
      </div>

    {#if currentStep === 1}
      <div class="scw-step-body">
        <div class="scw-info">
          <h4>大模型服务与凭证配置</h4>
          <p>well-ambient 的需求自解构引擎（Deconstructor）支持对接 OpenAI 兼容的大模型 API。当您配置并启用后，引擎会自动将复杂的需求文本拆解，并映射至 GitLab 的多仓项目中。</p>
        </div>

        <div class="scw-form-stack">
          <div class="scw-native-field">
            <Switch
              id="ai-enabled"
              label="启用 AI 需求解构引擎"
              bind:checked={enabled}
            />
            <span class="scw-helper">开启后，需求解构会通过该模型 API 实时推理并生成任务分配建议。</span>
          </div>

          {#if enabled}
            <TextInput
              id="ai-provider"
              label="大模型提供商 (Provider)"
              placeholder="openai"
              bind:value={provider}
              helperText="当前主要支持兼容 OpenAI API 协议的厂商（如 OpenAI、DeepSeek、硅基流动、阿里千问等）。"
            />

            <div class="scw-choice-field">
              <span class="scw-native-label">端点类型 (Endpoint Type)</span>
              <div class="scw-choice-grid two">
                <button 
                  type="button" 
                  class:active={endpointType === 'completions'}
                  on:click={() => endpointType = 'completions'}
                >
                  Completions (标准)
                </button>
                <button 
                  type="button" 
                  class:active={endpointType === 'responses'}
                  on:click={() => endpointType = 'responses'}
                >
                  Responses (新版)
                </button>
              </div>
              <span class="scw-helper">
                Completions: 标准 Chat 接口（格式为 messages 数组）；Responses: 新版 Agent 接口（格式为 input 数组）。
              </span>
            </div>

            <TextInput
              id="ai-base-url"
              label="API 请求地址 (API URL)"
              placeholder={endpointType === 'responses' ? 'https://api.pixelapi.com/v1/responses' : 'https://api.openai.com/v1/chat/completions'}
              bind:value={baseURL}
              helperText="请填写真实请求的 API 端点。若填入具体接口，我们将原样直接请求，不做强制拼接（例如：https://api.pixelapi.com/v1/chat/completions）。若只填域名，将根据端点类型自动拼接。"
            />

            {#if showTokenEditor}
              <TextInput
                id="ai-api-token"
                label="API 访问凭证 (API Key)"
                placeholder="sk-..."
                type="password"
                bind:value={apiToken}
                helperText="用于鉴权访问大模型 API 的安全密钥。此值将被加密或安全存储在后端。"
              />
            {:else}
              <div class="scw-credential">
                <div class="scw-credential-copy">
                  <span>AI API Key</span>
                  <strong>已配置，当前默认脱敏折叠</strong>
                </div>
                <button type="button" on:click={() => showTokenEditor = true}>编辑凭证/高级配置</button>
              </div>
            {/if}

            <TextInput
              id="ai-model"
              label="模型名称 (Model)"
              placeholder="gpt-4o"
              bind:value={modelName}
              helperText="指定推理模型。例如: gpt-4o、deepseek-chat、qwen-max 等。"
            />

            {#if testSuccess}
              <Alert type="success" title="测试成功" message={testSuccess}>
                {#if testDetails}
                  <pre class="scw-details scw-mono">{testDetails}</pre>
                {/if}
              </Alert>
            {/if}

            {#if testError}
              <Alert type="error" title="测试失败" message={testError}>
                {#if testDetails}
                  <pre class="scw-details scw-mono">{testDetails}</pre>
                {/if}
              </Alert>
            {/if}

            <div class="scw-section-actions">
              <Button variant="secondary" loading={testing} on:click={testConnection}>
                测试 AI 引擎连接
              </Button>
            </div>
          {/if}

          <section class="scw-section">
            <div class="scw-section-head">
              <div class="scw-section-copy">
                <span class="scw-kicker">估算口径</span>
                <h4>估算折算设置</h4>
                <p>系统设计、功能边界与流程资料由“系统设计语料库”维护；这里仅保留模型请求和排期折算参数。</p>
              </div>
              <span class="scw-badge">小时级估算</span>
            </div>

            <div class="scw-form-grid">
              <div class="scw-native-field">
                <label class="scw-native-label" for="ai-work-hours">每日折算小时</label>
                <input
                  id="ai-work-hours"
                  class="scw-native-input"
                  type="number"
                  min="1"
                  max="24"
                  step="0.5"
                  bind:value={defaultWorkHoursPerDay}
                />
              </div>
            </div>
          </section>

        </div>

        <div class="scw-actions">
          <Button variant="primary" on:click={nextStep}>
            下一步
          </Button>
        </div>
      </div>
    {:else if currentStep === 2}
      <div class="scw-step-body">
        <div class="scw-info">
          <h4>大模型引擎配置摘要</h4>
          <p>确认无误后点击下方按钮应用并保存配置：</p>
        </div>

        <div class="scw-summary">
          <div class="scw-summary-row">
            <span class="summary-label">自解构状态:</span>
            <span class="summary-value">
              {#if enabled}
                <span class="text-success">已启用 (对接真实大模型)</span>
              {:else}
                <span class="text-muted">已禁用 (回退至 Mock 演示模式)</span>
              {/if}
            </span>
          </div>
          {#if enabled}
            <div class="scw-summary-row">
              <span class="summary-label">服务商 (Provider):</span>
              <span class="summary-value font-mono">{provider}</span>
            </div>
            <div class="scw-summary-row">
              <span class="summary-label">协议/端点类型:</span>
              <span class="summary-value font-mono">{endpointType === 'responses' ? 'Responses (新版)' : 'Completions (标准)'}</span>
            </div>
            <div class="scw-summary-row">
              <span class="summary-label">API 请求地址:</span>
              <span class="summary-value font-mono">{baseURL}</span>
            </div>
            <div class="scw-summary-row">
              <span class="summary-label">指定模型 (Model):</span>
              <span class="summary-value font-mono">{modelName || 'gpt-4o (默认)'}</span>
            </div>
            <div class="scw-summary-row">
              <span class="summary-label">系统设计语料库:</span>
              <span class="summary-value">{activeContextFactCount} active · {defaultWorkHoursPerDay || 8} 小时/天</span>
            </div>
            <div class="scw-summary-row">
              <span class="summary-label">API 凭证 Token:</span>
              <span class="summary-value font-mono">••••••••••••••••••••••••</span>
            </div>
          {/if}
        </div>

        {#if saveError}
          <Alert type="error" title="保存失败" message={saveError} />
        {/if}

        <div class="scw-actions">
          <Button variant="ghost" on:click={prevStep} disabled={saving}>上一步</Button>
          <Button variant="secondary" on:click={finishClose} disabled={saving}>取消</Button>
          <Button variant="primary" loading={saving} on:click={() => saveConfig(false)}>
            保存并应用
          </Button>
        </div>
      </div>
    {/if}
    </section>
  {/if}

  {#if showContextPanel && !saveSuccess && (!editing || currentStep === 1)}
    <section class="scw-overview" aria-label="系统设计语料库">
      <header class="scw-header">
        <div class="scw-header-copy">
          <span class="scw-kicker">系统设计语料</span>
          <h4>系统设计语料库</h4>
          <p>在这里维护系统架构设计、功能设计、流程设计和估算口径；后台会将资料压缩成可缓存的上下文包，供需求解构自动引用。</p>
        </div>
        <div class="scw-section-actions">
          <span class="scw-badge success">{activeContextFactCount} 条生效</span>
          <span class="scw-badge">{totalContextFactTokens || 0} tokens</span>
        </div>
      </header>

      {#if contextFactsError}
        <div class="scw-inline-error">
          <span>{contextFactsError}</span>
          <div class="scw-section-actions"><button class="scw-icon-button" type="button" on:click={fetchContextFacts}>重试</button></div>
        </div>
      {/if}

      <div class="scw-list-layout">
        <div class="scw-list-column">
          <div class="scw-list-toolbar">
            <span class="scw-list-title">设计资料</span>
            <button type="button" on:click={fetchContextFacts} disabled={contextFactsLoading}>
              {contextFactsLoading ? '加载中' : '刷新'}
            </button>
          </div>

          {#if contextFactsLoading}
            <div class="scw-skeleton" aria-label="系统设计语料加载中">
              <span></span>
              <span></span>
              <span></span>
            </div>
          {:else if contextFacts.length === 0}
            <div class="scw-empty">
              <strong>暂无系统设计资料</strong>
              <p>先录入一条全局架构设计或流程设计，后端就能在预览接口中返回候选上下文包。</p>
            </div>
          {:else}
            <div class="scw-list" role="list" aria-label="系统设计资料列表">
              {#each contextFacts as fact (contextFactKey(fact))}
                <button
                  type="button"
                  class="scw-list-item"
                  class:active={String(editingContextFactId) === String(fact.id ?? contextFactKey(fact))}
                  on:click={() => beginEditContextFact(fact)}
                >
                  <div class="scw-list-meta">
                    <span class="scw-badge">{labelFor(contextFactTypes, fact.type)}</span>
                    <span class="scw-badge {fact.status === 'active' ? 'success' : fact.status === 'draft' ? 'warning' : ''}">{labelFor(contextFactStatuses, fact.status)}</span>
                  </div>
                  <strong>{fact.summary || '未命名事实'}</strong>
                  <p>{fact.content || '暂无内容'}</p>
                  <div class="scw-list-meta scw-mono">
                    <span>{compactScope(fact)}</span>
                    <span>{fact.source || 'manual'}</span>
                    <span>{fact.token_count || 0} tk</span>
                  </div>
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <div class="scw-editor-column">
          <div class="scw-list-toolbar">
            <span class="scw-list-title">{editingContextFactId ? '更新资料' : '创建资料'}</span>
            <button type="button" on:click={beginCreateContextFact}>新建</button>
          </div>

          {#if contextFactSaveError}
            <div class="scw-inline-error">{contextFactSaveError}</div>
          {/if}
          {#if contextFactSaveSuccess}
            <div class="scw-inline-success">{contextFactSaveSuccess}</div>
          {/if}

          <div class="scw-form-grid">
            <div class="scw-choice-field">
              <span class="scw-native-label">资料类型</span>
              <div class="scw-choice-grid">
                {#each contextFactTypes as option}
                  <button
                    type="button"
                    class:active={contextFactForm.type === option.value}
                    on:click={() => setContextFactField('type', option.value)}
                  >
                    {option.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="scw-choice-field">
              <span class="scw-native-label">状态</span>
              <div class="scw-choice-grid">
                {#each contextFactStatuses as option}
                  <button
                    type="button"
                    class:active={contextFactForm.status === option.value}
                    on:click={() => setContextFactField('status', option.value)}
                  >
                    {option.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="scw-choice-field">
              <span class="scw-native-label">适用范围</span>
              <div class="scw-choice-grid">
                {#each contextFactScopes as option}
                  <button
                    type="button"
                    class:active={contextFactForm.scope === option.value}
                    on:click={() => setContextFactField('scope', option.value)}
                  >
                    {option.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="scw-native-field">
              <label class="scw-native-label" for="context-fact-scope-id">范围标识</label>
              <input
                id="context-fact-scope-id"
                class="scw-native-input"
                placeholder={contextFactForm.scope === 'global' ? '全局事实可留空' : 'repo/module/demand type'}
                bind:value={contextFactForm.scope_id}
                disabled={contextFactForm.scope === 'global'}
              />
            </div>

            <div class="scw-choice-field">
              <span class="scw-native-label">来源</span>
              <div class="scw-choice-grid">
                {#each contextFactSources as option}
                  <button
                    type="button"
                    class:active={contextFactForm.source === option.value}
                    on:click={() => setContextFactField('source', option.value)}
                  >
                    {option.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="scw-native-field">
              <label class="scw-native-label" for="context-fact-owner">维护人</label>
              <input id="context-fact-owner" class="scw-native-input" placeholder="admin / team / system" bind:value={contextFactForm.owner} />
            </div>

            <div class="scw-native-field wide">
              <label class="scw-native-label" for="context-fact-summary">资料摘要</label>
              <input id="context-fact-summary" class="scw-native-input" placeholder="一句话说明这份设计资料覆盖的系统范围" bind:value={contextFactForm.summary} />
            </div>

            <div class="scw-native-field wide">
              <label class="scw-native-label" for="context-fact-content">设计内容</label>
              <textarea
                id="context-fact-content"
                class="scw-native-textarea"
                rows="5"
                placeholder="写入架构设计、功能边界、关键流程、依赖约束或估算规则。后台会自动计算 token、版本与上下文包命中。"
                bind:value={contextFactForm.content}
              ></textarea>
            </div>

            <div class="scw-score-grid wide">
              <label>
                <span>新鲜度 <b class="font-mono">{formatScore(contextFactForm.freshness)}</b></span>
                <input type="range" min="0" max="1" step="0.05" bind:value={contextFactForm.freshness} />
              </label>
              <label>
                <span>置信度 <b class="font-mono">{formatScore(contextFactForm.confidence)}</b></span>
                <input type="range" min="0" max="1" step="0.05" bind:value={contextFactForm.confidence} />
              </label>
            </div>
          </div>

          <div class="scw-actions">
            <Button variant="ghost" on:click={beginCreateContextFact}>清空</Button>
            <Button variant="primary" loading={contextFactSaving} on:click={saveContextFact}>
              {editingContextFactId ? '更新资料' : '创建资料'}
            </Button>
          </div>
        </div>
      </div>

      <section class="scw-section">
        <div class="scw-section-head">
          <div class="scw-section-copy">
            <span class="scw-kicker">上下文预览</span>
            <h4>上下文包预览</h4>
            <p>输入一段需求文本，预览后端会选择哪些系统设计资料进入 AI 解构上下文。</p>
          </div>
          <Button variant="secondary" loading={contextPreviewLoading} on:click={previewContextPack}>预览上下文包</Button>
        </div>

        <textarea
          class="scw-native-textarea"
          rows="3"
          placeholder="例如：为需求解构新增权限解释和上下文包归档能力，需要兼容现有粗粒度 RBAC。"
          bind:value={previewDemand}
        ></textarea>

        {#if contextPreviewError}
          <div class="scw-inline-error">{contextPreviewError}</div>
        {/if}

        {#if contextPreviewLoading}
          <div class="scw-skeleton" aria-label="上下文包预览加载中">
            <span></span>
            <span></span>
          </div>
        {:else if contextPackPreview}
          <div class="scw-metrics three">
            <div class="scw-metric">
              <span>Pack ID</span>
              <strong class="font-mono">{contextPackPreview.id || 'preview'}</strong>
            </div>
            <div class="scw-metric">
              <span>Token 预算</span>
              <strong class="font-mono">{contextPackPreview.token_count || 0}/{contextPackPreview.budget_tokens || 'auto'}</strong>
            </div>
            <div class="scw-metric">
              <span>Cache Key</span>
              <strong class="font-mono">{contextPackPreview.cache_key || '等待后端返回'}</strong>
            </div>
          </div>

          {#if contextPackPreview.summary}
            <pre class="scw-details scw-mono">{contextPackPreview.summary}</pre>
          {/if}

          <div class="scw-list">
            {#each contextPackPreview.items as item, index}
              <div class="scw-list-item static">
                <div class="scw-list-meta">
                  <span class="scw-badge">{index + 1}. {labelFor(contextFactTypes, item.type)}</span>
                  <span class="scw-mono">{compactScope(item)} / {formatScore(item.score)}</span>
                </div>
                <strong>{item.summary || '未命名候选事实'}</strong>
                <p>{item.reason || item.content || '后端暂未返回命中说明'}</p>
              </div>
            {:else}
              <div class="scw-empty">
                <strong>预览未选中资料</strong>
                <p>这通常表示后端接口仍在接入，或当前需求文本与可用设计资料没有匹配结果。</p>
              </div>
            {/each}
          </div>
        {:else}
          <div class="scw-empty">
            <strong>等待预览</strong>
            <p>预览结果会展示 pack 摘要、token 占用和被选中的设计资料。</p>
          </div>
        {/if}
      </section>
    </section>
  {/if}
</div>

<style>
  .text-success {
    color: var(--wa-success, #04966f);
  }

  .text-muted {
    color: var(--wa-text-muted, #667789);
  }

  .font-mono {
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  }

  .font-sans {
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
  }
</style>
