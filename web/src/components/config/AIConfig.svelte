<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Steps from '../shared/Steps.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import AIContextCenter from './AIContextCenter.svelte';

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
  $: isConfigured = enabled || !!(baseURL || apiToken || modelName || projectArchitecture || deliveryWorkflow || implementedFeatures || estimationGuidelines);
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
</script>

<div class="wizard">
  {#if saveSuccess}
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
  {:else if !editing && isConfigured}
    <div class="config-overview">
      <div class="overview-header">
        <div>
          <span class="overview-kicker font-mono">AI Deconstructor</span>
          <h4>AI 引擎配置状态摘要</h4>
          <p>已配置后默认显示模型状态、上下文覆盖、健康检查和编辑入口。</p>
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
        <span>架构 {contextStatus(projectArchitecture)}</span>
        <span>流程 {contextStatus(deliveryWorkflow)}</span>
        <span>已实现能力 {contextStatus(implementedFeatures)}</span>
        <span>估算口径 {contextStatus(estimationGuidelines)}</span>
      </div>

      {#if testDetails}
        <pre class="details-pre font-mono">{testDetails}</pre>
      {/if}

      <div class="overview-actions">
        <Button variant="secondary" loading={testing} on:click={testConnection}>健康检查</Button>
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </div>
  {:else}
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

          <div class="context-config-panel">
            <div class="context-header">
              <div>
                <span class="context-kicker">Estimation Calibration</span>
                <h4>项目上下文与估算口径</h4>
                <p class="context-intro">最强大脑需要的是可校准的判断边界：系统负责什么、事实从哪里来、哪些能力已存在、估算如何折算，而不是把功能清单整表塞进 Prompt。</p>
              </div>
              <span class="context-chip">小时级估算</span>
            </div>

            <div class="calibration-axis-grid">
              <span><b>边界</b>模块职责、上下游、明确不负责的范围</span>
              <span><b>事实源</b>GitLab / Jira / 配置库 / 决策面板的可信口径</span>
              <span><b>复用基线</b>已存在的平台级能力与可复用组件</span>
              <span><b>未知项</b>需要人工确认的接口、权限、数据与验收口径</span>
              <span><b>验证成本</b>联调、自测、回归、灰度和观测成本</span>
              <span><b>反馈闭环</b>估算与实际工时归档，用于后续校准</span>
            </div>

            <div class="context-grid">
              <div class="form-group-custom context-field wide">
                <label class="form-label-custom" for="ai-project-architecture">系统边界与责任域</label>
                <textarea
                  id="ai-project-architecture"
                  class="context-textarea"
                  rows="4"
                  placeholder="写清系统拥有的模块、上下游依赖、不能越界假设的部分。例如：Go 后端负责配置/API/任务归档；Svelte 前端负责看板与解构；GitLab/Jira 是事实源..."
                  bind:value={projectArchitecture}
                ></textarea>
                <span class="helper-text-custom">用于判断需求是配置、复用、扩展还是全新建设，避免把已有系统按从零开发估算。</span>
              </div>

              <div class="form-group-custom context-field wide">
                <label class="form-label-custom" for="ai-delivery-workflow">事实源与交付链路</label>
                <textarea
                  id="ai-delivery-workflow"
                  class="context-textarea"
                  rows="4"
                  placeholder="写清需求进入、AI 解构、影子任务、排期、分支/MR、Jira 状态同步、日报/周报与决策介入的真实链路..."
                  bind:value={deliveryWorkflow}
                ></textarea>
                <span class="helper-text-custom">用于把跨仓、评审、联调、验收、状态回写和异常处理纳入小时估算。</span>
              </div>

              <div class="form-group-custom context-field wide">
                <label class="form-label-custom" for="ai-implemented-features">平台级已实现基线</label>
                <textarea
                  id="ai-implemented-features"
                  class="context-textarea"
                  rows="4"
                  placeholder="只写平台级共识能力，不粘贴完整能力清单。例如：已具备需求解构、影子任务归档、GitLab/Jira 同步、日报/周报预览、红区诊断盘..."
                  bind:value={implementedFeatures}
                ></textarea>
                <span class="helper-text-custom">细粒度功能是否已实现放在下方模块画像中维护，这里只保留估算需要的全局基线。</span>
              </div>

              <div class="form-group-custom context-field wide">
                <label class="form-label-custom" for="ai-estimation-guidelines">估算校准规则</label>
                <textarea
                  id="ai-estimation-guidelines"
                  class="context-textarea"
                  rows="4"
                  placeholder="例如：统一按小时输出；包含开发、配置、联调、自测、回归、上线与回滚预案；已实现能力只算增量；未知项必须进入 missing_info 与风险说明。"
                  bind:value={estimationGuidelines}
                ></textarea>
                <span class="helper-text-custom">用于统一整体难度、子任务小时数、置信度、缓冲和后续估算准确性评估口径。</span>
              </div>

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
                <span class="helper-text-custom">用于归档和排期折算，默认 8 小时/天。</span>
              </div>
            </div>
          </div>

          <AIContextCenter />
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
              <span class="summary-label">项目架构上下文:</span>
              <span class="summary-value">{contextStatus(projectArchitecture)}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">研发流程上下文:</span>
              <span class="summary-value">{contextStatus(deliveryWorkflow)}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">已实现能力上下文:</span>
              <span class="summary-value">{contextStatus(implementedFeatures)}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">估算口径:</span>
              <span class="summary-value">{contextStatus(estimationGuidelines)} · {defaultWorkHoursPerDay || 8} 小时/天</span>
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

  .calibration-axis-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .calibration-axis-grid span {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 9px 10px;
    border: 1px solid rgba(51, 65, 85, 0.46);
    border-radius: 7px;
    background: rgba(2, 6, 23, 0.36);
    color: #64748b;
    font-size: 0.7rem;
    line-height: 1.45;
  }

  .calibration-axis-grid b {
    color: #cbd5e1;
    font-size: 0.76rem;
  }

  .context-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 14px;
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

  .context-textarea:focus,
  .context-input:focus {
    border-color: rgba(56, 189, 248, 0.72);
    box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.1);
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
    .calibration-axis-grid,
    .context-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .hours-field {
      max-width: none;
    }
  }
</style>
