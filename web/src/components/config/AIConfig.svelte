<script lang="ts">
  import { createEventDispatcher } from 'svelte';
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
  } = {
    enabled: false,
    provider: 'openai',
    base_url: '',
    endpoint_type: 'completions',
    api_token: '',
    model: ''
  };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;

  let currentStep = 1;
  const steps = ['连接与凭证', '确认应用'];

  // Form states
  let enabled = config.enabled ?? false;
  let provider = config.provider || 'openai';
  let baseURL = config.base_url || '';
  let endpointType = config.endpoint_type || 'completions';
  let apiToken = config.api_token || '';
  let modelName = config.model || '';

  // Test states
  let testing = false;
  let testError = '';
  let testSuccess = '';
  let testDetails = '';

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
            model: modelName
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
      model: modelName
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
        <Button variant="primary" on:click={() => dispatch('close')}>
          完成并关闭
        </Button>
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
              helperText="开启后，自解构交互界面将不再使用 Mock 演示数据，而是通过此大模型 API 实时推理需求分配。"
            />
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
              <label class="form-label-custom">端点类型 (Endpoint Type)</label>
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

            <TextInput
              id="ai-api-token"
              label="API 访问凭证 (API Key)"
              placeholder="sk-..."
              type="password"
              bind:value={apiToken}
              helperText="用于鉴权访问大模型 API 的安全密钥。此值将被加密或安全存储在后端。"
            />

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
    font-size: 0.85rem;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 8px;
  }

  .summary-row:last-of-type {
    border-bottom: none;
    padding-bottom: 0;
  }

  .summary-label {
    color: #64748b;
    font-weight: 500;
  }

  .summary-value {
    color: #cbd5e1;
    font-weight: 600;
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
</style>
