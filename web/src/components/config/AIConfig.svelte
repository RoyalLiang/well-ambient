<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import Steps from '../shared/Steps.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';

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
    { value: 'architecture', label: '架构事实' },
    { value: 'workflow', label: '交付流程' },
    { value: 'feature_boundary', label: '能力边界' },
    { value: 'estimation_rule', label: '估算规则' },
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
  $: isConfigured = enabled || !!(baseURL || apiToken || modelName || activeContextFactCount);
  $: activeContextFactCount = contextFacts.filter(fact => fact.status === 'active').length;
  $: totalContextFactTokens = contextFacts.reduce((sum, fact) => sum + (Number(fact.token_count) || 0), 0);
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

  async function saveConfig() {
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
      data: updatedAI
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
      contextFactsError = e.message || '上下文事实加载失败';
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
      contextFactSaveSuccess = editingContextFactId ? '上下文事实已更新' : '上下文事实已创建';
      await fetchContextFacts();
      if (!editingContextFactId) beginCreateContextFact();
    } catch (e: any) {
      contextFactSaveError = e.message || '上下文事实保存失败';
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

<div class="wizard">
  {#if showEnginePanel && saveSuccess}
    <div class="success-screen">
      <div class="success-icon">
        <svg xmlns="http://www.w3.org/2000/svg" class="checkmark-svg" viewBox="0 0 52 52">
          <circle class="checkmark-circle" cx="26" cy="26" r="25" fill="none"/>
          <path class="checkmark-check" fill="none" d="M14.1 27.2l7.1 7.2 16.7-16.8"/>
        </svg>
      </div>
      <h4 class="success-title">大模型引擎配置成功！</h4>
      <p class="success-desc font-mono">配置已成功保存。现在在“AI 需求解构引擎”页面提交开发需求将对接真实的 AI 大模型解析。</p>
      <div class="success-actions">
        <Button variant="primary" on:click={finishClose}>
          完成并关闭
        </Button>
      </div>
    </div>
  {:else if showEnginePanel && !editing && isConfigured}
    <div class="config-overview">
      <div class="overview-header">
        <div>
          <span class="overview-kicker font-mono">AI Deconstructor</span>
          <h4>AI 引擎配置状态摘要</h4>
          <p>已配置后默认显示模型状态、上下文事实、健康检查和编辑入口。</p>
        </div>
        <span class="status-pill {enabled ? 'online' : 'warning'}">{enabled ? '已启用' : '已禁用'}</span>
      </div>

      <div class="overview-grid">
        <div class="overview-row">
          <span>状态摘要</span>
          <strong>{provider || 'openai'} · {modelName || '未指定模型'} · {endpointType}</strong>
        </div>
        <div class="overview-row">
          <span>健康检查</span>
          <strong>{testSuccess || testError || '尚未执行本次巡检'}</strong>
        </div>
        <div class="overview-row">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
        <div class="overview-row">
          <span>敏感项</span>
          <strong>API Key {apiToken ? '已配置' : '未配置'} · {defaultWorkHoursPerDay || 8} 小时/天</strong>
        </div>
      </div>

      <div class="context-health-grid">
        <span>上下文事实 {activeContextFactCount} active</span>
        <span>上下文 tokens {totalContextFactTokens || 0}</span>
        <span>每日折算 {defaultWorkHoursPerDay || 8} 小时</span>
        <span>Pack 预览 {contextPackPreview ? '已生成' : '待生成'}</span>
      </div>

      {#if testDetails}
        <pre class="details-pre font-mono">{testDetails}</pre>
      {/if}

      <div class="overview-actions">
        <Button variant="secondary" loading={testing} on:click={testConnection}>健康检查</Button>
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </div>
  {:else if showEnginePanel}
    <Steps {currentStep} {steps} />

    {#if currentStep === 1}
      <div class="step-content">
        <div class="info-block">
          <h4>大模型服务与凭证配置</h4>
          <p>well-ambient 的需求自解构引擎（Deconstructor）支持对接 OpenAI 兼容的大模型 API。当您配置并启用后，引擎会自动将复杂的需求文本拆解，并映射至 GitLab 的多仓项目中。</p>
        </div>

        <div class="form-section">
          <div class="form-group">
            <Switch
              id="ai-enabled"
              label="启用 AI 需求解构引擎"
              bind:checked={enabled}
            />
            <span class="helper-text-custom">开启后，自解构交互界面将不再使用 Mock 演示数据，而是通过此大模型 API 实时推理需求分配。</span>
          </div>

          {#if enabled}
            <TextInput
              id="ai-provider"
              label="大模型提供商 (Provider)"
              placeholder="openai"
              bind:value={provider}
              helperText="当前主要支持兼容 OpenAI API 协议的厂商（如 OpenAI、DeepSeek、硅基流动、阿里千问等）。"
            />

            <div class="form-group-custom">
              <span class="form-label-custom">端点类型 (Endpoint Type)</span>
              <div class="segmented-control">
                <button 
                  type="button" 
                  class="control-btn {endpointType === 'completions' ? 'active' : ''}" 
                  on:click={() => endpointType = 'completions'}
                >
                  Completions (标准)
                </button>
                <button 
                  type="button" 
                  class="control-btn {endpointType === 'responses' ? 'active' : ''}" 
                  on:click={() => endpointType = 'responses'}
                >
                  Responses (新版)
                </button>
              </div>
              <span class="helper-text-custom">
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
              <div class="credential-collapsed">
                <div>
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
                  <pre class="details-pre font-mono">{testDetails}</pre>
                {/if}
              </Alert>
            {/if}

            {#if testError}
              <Alert type="error" title="测试失败" message={testError}>
                {#if testDetails}
                  <pre class="details-pre font-mono">{testDetails}</pre>
                {/if}
              </Alert>
            {/if}

            <div class="test-row">
              <Button variant="secondary" loading={testing} on:click={testConnection}>
                测试 AI 引擎连接
              </Button>
            </div>
          {/if}

          <div class="context-config-panel compact-settings">
            <div class="context-header">
              <div>
                <span class="context-kicker">Estimation</span>
                <h4>估算折算设置</h4>
                <p class="context-intro">系统上下文由下方事实注册表维护；这里仅保留模型请求和排期折算参数。</p>
              </div>
              <span class="context-chip">小时级估算</span>
            </div>

            <div class="context-grid compact">
              <div class="form-group-custom context-field hours-field">
                <label class="form-label-custom" for="ai-work-hours">每日折算小时</label>
                <input
                  id="ai-work-hours"
                  class="context-input"
                  type="number"
                  min="1"
                  max="24"
                  step="0.5"
                  bind:value={defaultWorkHoursPerDay}
                />
              </div>
            </div>
          </div>

        </div>

        <div class="actions">
          <Button variant="primary" on:click={nextStep}>
            下一步
          </Button>
        </div>
      </div>
    {:else if currentStep === 2}
      <div class="step-content">
        <div class="info-block">
          <h4>大模型引擎配置摘要</h4>
          <p>确认无误后点击下方按钮应用并保存配置：</p>
        </div>

        <div class="summary-card">
          <div class="summary-row">
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
            <div class="summary-row">
              <span class="summary-label">服务商 (Provider):</span>
              <span class="summary-value font-mono">{provider}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">协议/端点类型:</span>
              <span class="summary-value font-mono">{endpointType === 'responses' ? 'Responses (新版)' : 'Completions (标准)'}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">API 请求地址:</span>
              <span class="summary-value font-mono">{baseURL}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">指定模型 (Model):</span>
              <span class="summary-value font-mono">{modelName || 'gpt-4o (默认)'}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">上下文事实注册表:</span>
              <span class="summary-value">{activeContextFactCount} active · {defaultWorkHoursPerDay || 8} 小时/天</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">API 凭证 Token:</span>
              <span class="summary-value font-mono">••••••••••••••••••••••••</span>
            </div>
          {/if}
        </div>

        {#if saveError}
          <Alert type="error" title="保存失败" message={saveError} />
        {/if}

        <div class="actions">
          <Button variant="ghost" on:click={prevStep} disabled={saving}>上一步</Button>
          <Button variant="primary" loading={saving} on:click={saveConfig}>
            保存并应用
          </Button>
        </div>
      </div>
    {/if}
  {/if}

  {#if showContextPanel && !saveSuccess && (!editing || currentStep === 1)}
    <div class="context-registry-panel">
      <div class="registry-header">
        <div>
          <span class="context-kicker">AI Context Registry</span>
          <h4>上下文事实注册表</h4>
          <p>将架构、流程、能力边界和估算规则沉淀为可审计事实卡，供需求解构时组装成压缩上下文包。</p>
        </div>
        <div class="registry-metrics font-mono">
          <span>{activeContextFactCount} active</span>
          <span>{totalContextFactTokens || 0} tokens</span>
        </div>
      </div>

      {#if contextFactsError}
        <div class="registry-error">
          <span>{contextFactsError}</span>
          <button type="button" on:click={fetchContextFacts}>重试</button>
        </div>
      {/if}

      <div class="registry-layout">
        <div class="registry-list-column">
          <div class="registry-toolbar">
            <span class="registry-section-title">事实卡片</span>
            <button type="button" on:click={fetchContextFacts} disabled={contextFactsLoading}>
              {contextFactsLoading ? '加载中' : '刷新'}
            </button>
          </div>

          {#if contextFactsLoading}
            <div class="registry-skeleton" aria-label="上下文事实加载中">
              <span></span>
              <span></span>
              <span></span>
            </div>
          {:else if contextFacts.length === 0}
            <div class="registry-empty">
              <strong>暂无上下文事实</strong>
              <p>先创建一条全局架构或估算规则事实，后端就能在预览接口中返回候选上下文包。</p>
            </div>
          {:else}
            <div class="registry-list" role="list" aria-label="上下文事实列表">
              {#each contextFacts as fact (contextFactKey(fact))}
                <button
                  type="button"
                  class="registry-fact-card {String(editingContextFactId) === String(fact.id ?? contextFactKey(fact)) ? 'active' : ''}"
                  on:click={() => beginEditContextFact(fact)}
                >
                  <div class="fact-card-top">
                    <span class="fact-type">{labelFor(contextFactTypes, fact.type)}</span>
                    <span class="fact-status status-{fact.status}">{labelFor(contextFactStatuses, fact.status)}</span>
                  </div>
                  <strong>{fact.summary || '未命名事实'}</strong>
                  <p>{fact.content || '暂无内容'}</p>
                  <div class="fact-meta font-mono">
                    <span>{compactScope(fact)}</span>
                    <span>{fact.source || 'manual'}</span>
                    <span>{fact.token_count || 0} tk</span>
                  </div>
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <div class="registry-editor-column">
          <div class="registry-toolbar">
            <span class="registry-section-title">{editingContextFactId ? '更新事实' : '创建事实'}</span>
            <button type="button" on:click={beginCreateContextFact}>新建</button>
          </div>

          {#if contextFactSaveError}
            <div class="inline-error">{contextFactSaveError}</div>
          {/if}
          {#if contextFactSaveSuccess}
            <div class="inline-success">{contextFactSaveSuccess}</div>
          {/if}

          <div class="registry-form-grid">
            <div class="form-group-custom">
              <span class="form-label-custom">事实类型</span>
              <div class="context-choice-grid">
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

            <div class="form-group-custom">
              <span class="form-label-custom">状态</span>
              <div class="context-choice-grid compact">
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

            <div class="form-group-custom">
              <span class="form-label-custom">Scope</span>
              <div class="context-choice-grid compact">
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

            <div class="form-group-custom">
              <label class="form-label-custom" for="context-fact-scope-id">Scope ID</label>
              <input
                id="context-fact-scope-id"
                class="context-input"
                placeholder={contextFactForm.scope === 'global' ? '全局事实可留空' : 'repo/module/demand type'}
                bind:value={contextFactForm.scope_id}
                disabled={contextFactForm.scope === 'global'}
              />
            </div>

            <div class="form-group-custom">
              <span class="form-label-custom">来源</span>
              <div class="context-choice-grid compact">
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

            <div class="form-group-custom">
              <label class="form-label-custom" for="context-fact-owner">Owner</label>
              <input id="context-fact-owner" class="context-input" placeholder="admin / team / system" bind:value={contextFactForm.owner} />
            </div>

            <div class="form-group-custom wide">
              <label class="form-label-custom" for="context-fact-summary">事实摘要</label>
              <input id="context-fact-summary" class="context-input" placeholder="一句话说明这条事实的用途" bind:value={contextFactForm.summary} />
            </div>

            <div class="form-group-custom wide">
              <label class="form-label-custom" for="context-fact-content">事实内容</label>
              <textarea
                id="context-fact-content"
                class="context-textarea compact"
                rows="5"
                placeholder="写入可压缩进 Prompt 的事实，不放临时讨论和未经确认的猜测。"
                bind:value={contextFactForm.content}
              ></textarea>
            </div>

            <div class="score-grid wide">
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

          <div class="registry-actions">
            <Button variant="ghost" on:click={beginCreateContextFact}>清空</Button>
            <Button variant="primary" loading={contextFactSaving} on:click={saveContextFact}>
              {editingContextFactId ? '更新事实' : '创建事实'}
            </Button>
          </div>
        </div>
      </div>

      <div class="pack-preview-panel">
        <div class="registry-header compact">
          <div>
            <span class="context-kicker">Pack Preview</span>
            <h4>上下文包预览</h4>
            <p>输入一段需求文本，预览后端将选择哪些事实进入 AI 解构上下文。</p>
          </div>
          <Button variant="secondary" loading={contextPreviewLoading} on:click={previewContextPack}>预览上下文包</Button>
        </div>

        <textarea
          class="context-textarea preview-demand"
          rows="3"
          placeholder="例如：为需求解构新增权限解释和上下文包归档能力，需要兼容现有粗粒度 RBAC。"
          bind:value={previewDemand}
        ></textarea>

        {#if contextPreviewError}
          <div class="inline-error">{contextPreviewError}</div>
        {/if}

        {#if contextPreviewLoading}
          <div class="registry-skeleton slim" aria-label="上下文包预览加载中">
            <span></span>
            <span></span>
          </div>
        {:else if contextPackPreview}
          <div class="pack-summary">
            <div>
              <span>Pack ID</span>
              <strong class="font-mono">{contextPackPreview.id || 'preview'}</strong>
            </div>
            <div>
              <span>Token 预算</span>
              <strong class="font-mono">{contextPackPreview.token_count || 0}/{contextPackPreview.budget_tokens || 'auto'}</strong>
            </div>
            <div>
              <span>Cache Key</span>
              <strong class="font-mono">{contextPackPreview.cache_key || '等待后端返回'}</strong>
            </div>
          </div>

          {#if contextPackPreview.summary}
            <pre class="details-pre pack-text font-mono">{contextPackPreview.summary}</pre>
          {/if}

          <div class="pack-items">
            {#each contextPackPreview.items as item, index}
              <div class="pack-item">
                <div class="pack-item-top">
                  <span class="fact-type">{index + 1}. {labelFor(contextFactTypes, item.type)}</span>
                  <span class="fact-meta font-mono">{compactScope(item)} · {formatScore(item.score)}</span>
                </div>
                <strong>{item.summary || '未命名候选事实'}</strong>
                <p>{item.reason || item.content || '后端暂未返回命中说明'}</p>
              </div>
            {:else}
              <div class="registry-empty compact">
                <strong>预览未选中事实</strong>
                <p>这通常表示后端接口仍在接入，或当前需求文本与可用事实没有匹配结果。</p>
              </div>
            {/each}
          </div>
        {:else}
          <div class="registry-empty compact">
            <strong>等待预览</strong>
            <p>预览结果会展示 pack 摘要、token 占用和被选中的事实卡。</p>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .wizard {
    display: flex;
    flex-direction: column;
    width: 100%;
    box-sizing: border-box;
  }

  .step-content {
    display: flex;
    flex-direction: column;
    animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .info-block {
    background: rgba(30, 41, 59, 0.4);
    border-left: 4px solid #38bdf8;
    padding: 12px 16px;
    border-radius: 0 8px 8px 0;
    margin-bottom: 24px;
  }

  .info-block h4 {
    margin: 0 0 6px 0;
    font-size: 0.95rem;
    font-weight: 700;
    color: #cbd5e1;
  }

  .info-block p {
    margin: 0;
    font-size: 0.8rem;
    color: #94a3b8;
    line-height: 1.5;
  }

  .form-section {
    display: flex;
    flex-direction: column;
    gap: 20px;
    margin-bottom: 24px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
  }

  .form-group-custom {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-label-custom {
    font-size: 0.85rem;
    font-weight: 600;
    color: #94a3b8;
  }

  .helper-text-custom {
    font-size: 0.725rem;
    color: #64748b;
    margin-top: 2px;
  }

  .segmented-control {
    display: flex;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.2);
    padding: 4px;
    border-radius: 8px;
    gap: 4px;
  }

  .control-btn {
    flex: 1;
    background: transparent;
    border: 1px solid transparent;
    color: #94a3b8;
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .control-btn:hover {
    color: #cbd5e1;
    background: rgba(255, 255, 255, 0.05);
  }

  .control-btn.active {
    color: #ffffff;
    background: rgba(99, 102, 241, 0.25);
    border-color: rgba(99, 102, 241, 0.4);
    box-shadow: 0 0 12px rgba(99, 102, 241, 0.2);
    text-shadow: 0 0 8px rgba(255, 255, 255, 0.5);
  }

  .test-row {
    display: flex;
    margin-top: 4px;
    margin-bottom: 12px;
  }

  .config-overview {
    display: flex;
    flex-direction: column;
    gap: 18px;
    animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .overview-header {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    align-items: flex-start;
    border-bottom: 1px solid rgba(51, 65, 85, 0.42);
    padding-bottom: 16px;
  }

  .overview-kicker {
    color: #38bdf8;
    font-size: 0.68rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .overview-header h4 {
    margin: 4px 0 6px 0;
    color: #f8fafc;
    font-size: 1.05rem;
  }

  .overview-header p {
    margin: 0;
    color: #94a3b8;
    font-size: 0.8rem;
    line-height: 1.5;
  }

  .status-pill {
    flex: none;
    border-radius: 999px;
    padding: 5px 10px;
    font-size: 0.72rem;
    font-weight: 800;
    border: 1px solid rgba(148, 163, 184, 0.24);
  }

  .status-pill.online {
    color: #34d399;
    background: rgba(16, 185, 129, 0.1);
    border-color: rgba(16, 185, 129, 0.22);
  }

  .status-pill.warning {
    color: #fbbf24;
    background: rgba(245, 158, 11, 0.1);
    border-color: rgba(245, 158, 11, 0.22);
  }

  .overview-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 10px;
  }

  .overview-row,
  .credential-collapsed,
  .context-health-grid {
    min-width: 0;
    background: rgba(15, 23, 42, 0.52);
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-radius: 8px;
    padding: 12px;
  }

  .overview-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .overview-row span,
  .credential-collapsed span {
    color: #64748b;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .overview-row strong,
  .credential-collapsed strong {
    color: #e2e8f0;
    font-size: 0.86rem;
    overflow-wrap: anywhere;
  }

  .context-health-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 8px;
    color: #94a3b8;
    font-size: 0.78rem;
    font-weight: 700;
  }

  .overview-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    flex-wrap: wrap;
  }

  .credential-collapsed {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    margin-bottom: 18px;
  }

  .credential-collapsed div {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .credential-collapsed button {
    flex: none;
    background: transparent;
    border: 1px solid rgba(99, 102, 241, 0.34);
    color: #a5b4fc;
    border-radius: 6px;
    padding: 7px 10px;
    font-size: 0.78rem;
    font-weight: 700;
    cursor: pointer;
  }

  .credential-collapsed button:hover {
    background: rgba(99, 102, 241, 0.12);
  }

  .context-config-panel {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 14px;
    background: rgba(15, 23, 42, 0.44);
    border: 1px solid rgba(51, 65, 85, 0.55);
    border-radius: 8px;
    box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.06);
  }

  .context-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .context-header h4 {
    margin: 3px 0 0 0;
    color: #e2e8f0;
    font-size: 0.95rem;
    font-weight: 700;
  }

  .context-intro {
    max-width: 720px;
    margin: 6px 0 0 0;
    color: #94a3b8;
    font-size: 0.78rem;
    line-height: 1.55;
  }

  .context-kicker {
    color: #38bdf8;
    font-size: 0.65rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .context-chip {
    flex: none;
    padding: 4px 8px;
    border: 1px solid rgba(56, 189, 248, 0.24);
    border-radius: 4px;
    background: rgba(8, 47, 73, 0.32);
    color: #7dd3fc;
    font-size: 0.7rem;
    font-weight: 700;
  }

  .context-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 14px;
  }

  .context-grid.compact {
    grid-template-columns: minmax(180px, 220px);
  }

  .context-field.wide {
    min-width: 0;
  }

  .hours-field {
    max-width: 220px;
  }

  .context-textarea,
  .context-input {
    width: 100%;
    box-sizing: border-box;
    background: rgba(2, 6, 23, 0.62);
    border: 1px solid rgba(71, 85, 105, 0.62);
    border-radius: 6px;
    color: #e2e8f0;
    font: inherit;
    font-size: 0.82rem;
    outline: none;
    transition: border-color 0.18s ease, box-shadow 0.18s ease;
  }

  .context-textarea {
    min-height: 92px;
    padding: 10px 11px;
    line-height: 1.5;
    resize: vertical;
  }

  .context-textarea,
  .details-pre {
    scrollbar-width: thin;
    scrollbar-color: rgba(56, 189, 248, 0.5) rgba(15, 23, 42, 0.72);
  }

  .context-textarea::-webkit-scrollbar,
  .details-pre::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  .context-textarea::-webkit-scrollbar-track,
  .details-pre::-webkit-scrollbar-track {
    background: rgba(2, 6, 23, 0.44);
    border-radius: 999px;
  }

  .context-textarea::-webkit-scrollbar-thumb,
  .details-pre::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(56, 189, 248, 0.62), rgba(52, 211, 153, 0.34));
    border: 2px solid rgba(2, 6, 23, 0.44);
    border-radius: 999px;
  }

  .context-input {
    height: 38px;
    padding: 0 10px;
  }

  .context-input[type='number'] {
    appearance: textfield;
    -moz-appearance: textfield;
  }

  .context-input[type='number']::-webkit-outer-spin-button,
  .context-input[type='number']::-webkit-inner-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  .context-textarea:focus,
  .context-input:focus {
    border-color: rgba(56, 189, 248, 0.72);
    box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.1);
  }

  .context-choice-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(118px, 1fr));
    gap: 7px;
  }

  .context-choice-grid.compact {
    grid-template-columns: repeat(auto-fit, minmax(86px, 1fr));
  }

  .context-choice-grid button {
    min-height: 34px;
    border: 1px solid rgba(51, 65, 85, 0.58);
    background: rgba(2, 6, 23, 0.38);
    color: #94a3b8;
    border-radius: 8px;
    font-size: 0.76rem;
    font-weight: 800;
    cursor: pointer;
    transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
  }

  .context-choice-grid button:hover {
    transform: translateY(-1px);
    color: #cbd5e1;
    border-color: rgba(56, 189, 248, 0.38);
  }

  .context-choice-grid button.active {
    color: #f8fafc;
    border-color: rgba(56, 189, 248, 0.54);
    background: rgba(8, 47, 73, 0.5);
    box-shadow: inset 0 1px 0 rgba(125, 211, 252, 0.12);
  }

  .context-registry-panel,
  .pack-preview-panel {
    margin-top: 18px;
    border: 1px solid rgba(51, 65, 85, 0.52);
    background:
      linear-gradient(135deg, rgba(56, 189, 248, 0.06), transparent 34%),
      rgba(15, 23, 42, 0.36);
    border-radius: 8px;
    padding: 16px;
  }

  .registry-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 14px;
    padding-bottom: 14px;
    margin-bottom: 14px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.42);
  }

  .registry-header.compact {
    align-items: center;
  }

  .registry-header h4 {
    margin: 4px 0 6px 0;
    color: #f8fafc;
    font-size: 1rem;
  }

  .registry-header p {
    max-width: 720px;
    margin: 0;
    color: #94a3b8;
    font-size: 0.8rem;
    line-height: 1.48;
  }

  .registry-metrics {
    flex: none;
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .registry-metrics span,
  .registry-section-title {
    color: #94a3b8;
    border: 1px solid rgba(51, 65, 85, 0.52);
    background: rgba(2, 6, 23, 0.34);
    border-radius: 6px;
    padding: 5px 8px;
    font-size: 0.72rem;
    font-weight: 800;
  }

  .registry-error,
  .inline-error,
  .inline-success {
    border-radius: 8px;
    padding: 10px 12px;
    font-size: 0.8rem;
    line-height: 1.42;
    margin-bottom: 12px;
  }

  .registry-error,
  .inline-error {
    color: #fca5a5;
    border: 1px solid rgba(239, 68, 68, 0.24);
    background: rgba(127, 29, 29, 0.16);
  }

  .inline-success {
    color: #86efac;
    border: 1px solid rgba(34, 197, 94, 0.24);
    background: rgba(6, 78, 59, 0.16);
  }

  .registry-error {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: center;
  }

  .registry-error button,
  .registry-toolbar button {
    border: 1px solid rgba(56, 189, 248, 0.28);
    background: rgba(8, 47, 73, 0.28);
    color: #7dd3fc;
    border-radius: 6px;
    padding: 6px 9px;
    font-size: 0.76rem;
    font-weight: 800;
    cursor: pointer;
  }

  .registry-error button:hover,
  .registry-toolbar button:hover:not(:disabled) {
    background: rgba(8, 47, 73, 0.42);
  }

  .registry-toolbar button:disabled {
    opacity: 0.58;
    cursor: wait;
  }

  .registry-layout {
    display: grid;
    grid-template-columns: minmax(260px, 0.8fr) minmax(320px, 1.2fr);
    gap: 14px;
    align-items: start;
  }

  .registry-list-column,
  .registry-editor-column {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.46);
    background: rgba(2, 6, 23, 0.22);
    border-radius: 8px;
    padding: 12px;
  }

  .registry-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
  }

  .registry-list {
    display: flex;
    flex-direction: column;
    gap: 9px;
    max-height: 560px;
    overflow: auto;
    padding-right: 3px;
  }

  .registry-fact-card {
    width: 100%;
    border: 1px solid rgba(51, 65, 85, 0.52);
    background: rgba(15, 23, 42, 0.42);
    color: #cbd5e1;
    border-radius: 8px;
    padding: 11px;
    text-align: left;
    cursor: pointer;
    transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease;
  }

  .registry-fact-card:hover,
  .registry-fact-card.active {
    transform: translateY(-1px);
    border-color: rgba(56, 189, 248, 0.46);
    background: rgba(8, 47, 73, 0.22);
  }

  .registry-fact-card strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.88rem;
    line-height: 1.35;
    margin-top: 8px;
  }

  .registry-fact-card p {
    margin: 6px 0 0 0;
    color: #94a3b8;
    font-size: 0.76rem;
    line-height: 1.42;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .fact-card-top,
  .pack-item-top {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    align-items: center;
  }

  .fact-type,
  .fact-status,
  .fact-meta {
    display: inline-flex;
    align-items: center;
    width: fit-content;
    border-radius: 999px;
    padding: 3px 7px;
    font-size: 0.68rem;
    font-weight: 900;
  }

  .fact-type {
    color: #7dd3fc;
    background: rgba(8, 47, 73, 0.34);
    border: 1px solid rgba(56, 189, 248, 0.24);
  }

  .fact-status {
    color: #cbd5e1;
    background: rgba(51, 65, 85, 0.38);
    border: 1px solid rgba(100, 116, 139, 0.22);
  }

  .fact-status.status-active {
    color: #86efac;
    background: rgba(6, 78, 59, 0.2);
    border-color: rgba(34, 197, 94, 0.24);
  }

  .fact-status.status-draft {
    color: #fde68a;
    background: rgba(120, 53, 15, 0.18);
    border-color: rgba(245, 158, 11, 0.22);
  }

  .fact-status.status-paused,
  .fact-status.status-retired {
    color: #94a3b8;
  }

  .fact-meta {
    gap: 7px;
    flex-wrap: wrap;
    color: #64748b;
    padding: 0;
    margin-top: 9px;
  }

  .registry-form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .registry-form-grid .wide,
  .score-grid.wide {
    grid-column: 1 / -1;
  }

  .score-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .score-grid label {
    display: flex;
    flex-direction: column;
    gap: 8px;
    border: 1px solid rgba(51, 65, 85, 0.46);
    background: rgba(2, 6, 23, 0.28);
    border-radius: 8px;
    padding: 10px;
  }

  .score-grid span {
    display: flex;
    justify-content: space-between;
    gap: 10px;
    color: #94a3b8;
    font-size: 0.76rem;
    font-weight: 800;
  }

  .score-grid b {
    color: #e2e8f0;
  }

  .score-grid input[type='range'] {
    appearance: none;
    -webkit-appearance: none;
    width: 100%;
    height: 6px;
    border-radius: 999px;
    background: linear-gradient(90deg, rgba(56, 189, 248, 0.82), rgba(52, 211, 153, 0.78));
    outline: none;
  }

  .score-grid input[type='range']::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 999px;
    background: #f8fafc;
    border: 3px solid #0891b2;
    box-shadow: 0 3px 10px rgba(2, 6, 23, 0.48);
    cursor: pointer;
  }

  .score-grid input[type='range']::-moz-range-thumb {
    width: 18px;
    height: 18px;
    border-radius: 999px;
    background: #f8fafc;
    border: 3px solid #0891b2;
    box-shadow: 0 3px 10px rgba(2, 6, 23, 0.48);
    cursor: pointer;
  }

  .registry-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid rgba(51, 65, 85, 0.38);
  }

  .registry-empty,
  .registry-skeleton {
    border: 1px dashed rgba(71, 85, 105, 0.54);
    background: rgba(2, 6, 23, 0.22);
    color: #64748b;
    border-radius: 8px;
    padding: 16px;
    font-size: 0.8rem;
    line-height: 1.45;
  }

  .registry-empty strong {
    display: block;
    color: #cbd5e1;
    margin-bottom: 5px;
  }

  .registry-empty p {
    margin: 0;
  }

  .registry-empty.compact {
    padding: 12px;
  }

  .registry-skeleton {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .registry-skeleton span {
    height: 34px;
    border-radius: 8px;
    background: linear-gradient(90deg, rgba(30, 41, 59, 0.55), rgba(51, 65, 85, 0.38), rgba(30, 41, 59, 0.55));
    background-size: 180% 100%;
    animation: registryPulse 1.4s ease-in-out infinite;
  }

  .registry-skeleton.slim span {
    height: 22px;
  }

  @keyframes registryPulse {
    0% { background-position: 100% 0; }
    100% { background-position: -100% 0; }
  }

  .pack-preview-panel {
    background:
      linear-gradient(135deg, rgba(52, 211, 153, 0.06), transparent 34%),
      rgba(15, 23, 42, 0.34);
  }

  .preview-demand {
    min-height: 86px;
    margin-bottom: 12px;
  }

  .pack-summary {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
    margin-bottom: 12px;
  }

  .pack-summary div {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.46);
    background: rgba(2, 6, 23, 0.28);
    border-radius: 8px;
    padding: 10px;
  }

  .pack-summary span {
    display: block;
    color: #64748b;
    font-size: 0.7rem;
    font-weight: 800;
    margin-bottom: 4px;
  }

  .pack-summary strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }

  .pack-text {
    max-height: 180px;
  }

  .pack-items {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 10px;
  }

  .pack-item {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.46);
    background: rgba(2, 6, 23, 0.24);
    border-radius: 8px;
    padding: 11px;
  }

  .pack-item strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.86rem;
    margin-top: 8px;
  }

  .pack-item p {
    margin: 6px 0 0 0;
    color: #94a3b8;
    font-size: 0.76rem;
    line-height: 1.45;
  }

  .details-pre {
    margin: 12px 0 0 0;
    padding: 12px;
    background: #0f172a;
    border: 1px solid rgba(51, 65, 85, 0.5);
    border-radius: 6px;
    color: #e2e8f0;
    font-size: 0.75rem;
    max-height: 120px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .summary-card {
    background: rgba(15, 23, 42, 0.3);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 24px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    font-size: 0.85rem;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 8px;
  }

  .summary-row:last-of-type {
    border-bottom: none;
    padding-bottom: 0;
  }

  .summary-label {
    flex: none;
    color: #64748b;
    font-weight: 500;
  }

  .summary-value {
    min-width: 0;
    color: #cbd5e1;
    font-weight: 600;
    text-align: right;
    overflow-wrap: anywhere;
  }

  .text-success {
    color: #34d399;
  }

  .text-muted {
    color: #64748b;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 12px;
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    padding-top: 16px;
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateX(8px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  /* Success Screen styles */
  .success-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 32px 16px;
    text-align: center;
    animation: fadeIn 0.4s ease-out;
  }

  .success-title {
    font-size: 1.25rem;
    font-weight: 700;
    margin: 0 0 8px 0;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .success-desc {
    font-size: 0.8rem;
    color: #38bdf8;
    margin: 0 0 24px 0;
    max-width: 380px;
    line-height: 1.5;
  }

  .success-icon {
    width: 64px;
    height: 64px;
    margin-bottom: 20px;
  }

  .checkmark-svg {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    display: block;
    stroke-width: 3;
    stroke: #34d399;
    stroke-miterlimit: 10;
    box-shadow: inset 0px 0px 0px #34d399;
    animation: fill .4s ease-in-out .4s forwards, scale .3s ease-in-out .9s both;
  }

  .checkmark-circle {
    stroke-dasharray: 166;
    stroke-dashoffset: 166;
    stroke-width: 3;
    stroke-miterlimit: 10;
    stroke: #34d399;
    fill: none;
    animation: stroke 0.6s cubic-bezier(0.65, 0, 0.45, 1) forwards;
  }

  .checkmark-check {
    transform-origin: 50% 50%;
    stroke-dasharray: 48;
    stroke-dashoffset: 48;
    stroke-width: 3;
    stroke: #fff;
    animation: stroke 0.3s cubic-bezier(0.65, 0, 0.45, 1) 0.8s forwards;
  }

  @keyframes stroke {
    100% {
      stroke-dashoffset: 0;
    }
  }

  @keyframes scale {
    0%, 100% {
      transform: none;
    }
    50% {
      transform: scale3d(1.1, 1.1, 1);
    }
  }

  @keyframes fill {
    100% {
      box-shadow: inset 0px 0px 0px 32px #34d399;
    }
  }

  .success-actions {
    display: flex;
    justify-content: center;
    width: 100%;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @media (max-width: 820px) {
    .overview-grid,
    .context-health-grid,
    .registry-layout,
    .registry-form-grid,
    .score-grid,
    .pack-summary {
      grid-template-columns: minmax(0, 1fr);
    }

    .registry-header,
    .overview-header,
    .credential-collapsed {
      flex-direction: column;
      align-items: stretch;
    }

    .context-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .hours-field {
      max-width: none;
    }

    .context-choice-grid,
    .context-choice-grid.compact {
      grid-template-columns: repeat(auto-fit, minmax(96px, 1fr));
    }
  }
</style>
