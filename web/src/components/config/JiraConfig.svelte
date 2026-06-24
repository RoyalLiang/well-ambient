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
    base_url: string;
    username: string;
    api_token: string;
    sync_projects?: string[];
    sync_users?: string[];
    sync_statuses?: string[];
    custom_jql?: string;
  } = {
    enabled: false,
    base_url: '',
    username: '',
    api_token: '',
    sync_projects: [],
    sync_users: [],
    sync_statuses: [],
    custom_jql: ''
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
  let baseURL = config.base_url || '';
  let username = config.username || '';
  let apiToken = config.api_token || '';
  let syncProjects = (config.sync_projects || []).join(', ');
  let syncUsers = (config.sync_users || []).join(', ');
  let syncStatuses = (config.sync_statuses || []).join(', ');
  let customJQL = config.custom_jql || '';

  // Test states
  let testing = false;
  let testError = '';
  let testSuccess = '';
  let testDetails = '';
  $: isConfigured = enabled || !!(baseURL || username || apiToken || syncProjects || syncUsers || syncStatuses || customJQL);
  $: if (!editing && !saveSuccess) {
    enabled = config.enabled ?? false;
    baseURL = config.base_url || '';
    username = config.username || '';
    apiToken = config.api_token || '';
    syncProjects = (config.sync_projects || []).join(', ');
    syncUsers = (config.sync_users || []).join(', ');
    syncStatuses = (config.sync_statuses || []).join(', ');
    customJQL = config.custom_jql || '';
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

  async function testConnection() {
    if (!baseURL || !apiToken) {
      testError = '请填写连接地址与 API 令牌/PAT';
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
          type: 'jira',
          jira: {
            enabled: enabled,
            base_url: baseURL,
            username: username,
            api_token: apiToken
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        testSuccess = 'Jira 连通性校验成功！';
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
    saving = true;
    saveError = '';

    const updatedJira = {
      enabled,
      base_url: baseURL,
      username,
      api_token: apiToken,
      sync_projects: syncProjects.split(',').map(s => s.trim()).filter(Boolean),
      sync_users: syncUsers.split(',').map(s => s.trim()).filter(Boolean),
      sync_statuses: syncStatuses.split(',').map(s => s.trim()).filter(Boolean),
      custom_jql: customJQL
    };

    dispatch('save', {
      key: 'jira',
      data: updatedJira,
      isToggle: isToggle
    });
  }

  function nextStep() {
    if (currentStep === 1) {
      if (enabled && (!baseURL || !apiToken)) {
        testError = '启用 Jira 同步时，必须填写连接地址及 API 令牌/PAT';
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
      <h4 class="success-title">Jira 配置已成功应用！</h4>
      <p class="success-desc font-mono">配置已成功写入配置文件并完成服务热重载。</p>
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
          <span class="overview-kicker font-mono">Jira Integration</span>
          <h4>Jira 配置状态摘要</h4>
          <p>默认以只读安全呈现各配置字段详情，支持右上角快速启用/禁用。</p>
        </div>
        <div style="display: flex; align-items: center; gap: 10px;">
          <Switch id="jira-overview-toggle" bind:checked={enabled} on:change={() => saveConfig(true)} />
        </div>
      </div>

      <div class="overview-grid">
        <div class="overview-row">
          <span>Jira 基础 URL 地址</span>
          <strong class="font-mono">{baseURL || '-'}</strong>
        </div>
        <div class="overview-row">
          <span>用户邮箱 (Username)</span>
          <strong class="font-mono">{username || 'Token 认证'}</strong>
        </div>
        <div class="overview-row">
          <span>敏感凭证 (API Token)</span>
          <strong>{apiToken ? '已配置 (已脱敏保护)' : '未配置'}</strong>
        </div>
        <div class="overview-row">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
        <div class="overview-row">
          <span>同步项目 (Project Keys)</span>
          <strong class="font-mono">{syncProjects || '所有项目'}</strong>
        </div>
        <div class="overview-row">
          <span>同步成员 (Assignees)</span>
          <strong class="font-mono">{syncUsers || '所有成员'}</strong>
        </div>
        <div class="overview-row">
          <span>状态范围筛选</span>
          <strong class="font-mono">{syncStatuses || '所有状态'}</strong>
        </div>
        <div class="overview-row">
          <span>健康状态</span>
          <strong class="text-success">{testSuccess || testError || '已连接'}</strong>
        </div>
      </div>

      {#if customJQL}
        <div class="jql-preview">
          <span>自定义 JQL 筛选</span>
          <code>{customJQL}</code>
        </div>
      {/if}

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
        <h4>Jira API 连接与同步设置</h4>
        <p>通过配置 Jira 认证，well-ambient 可以在检测到分支提交中的任务 ID 时，自动查询关联的 Jira Issue 标题，并将同步状态写回 Jira 问题单中。</p>
      </div>

      <Switch
        id="jira-enabled"
        label="启用 Jira 双向集成同步"
        bind:checked={enabled}
      />

      {#if enabled}
        <div class="form-sub-section">
          <TextInput
            id="jira-url"
            label="Jira 基础 URL 地址"
            placeholder="https://your-domain.atlassian.net"
            bind:value={baseURL}
            required={enabled}
            helperText="请输入 Jira 站点的根 URL 地址。"
          />

          <TextInput
            id="jira-username"
            label="Jira 用户邮箱 (Username / Email)"
            placeholder="example@yourdomain.com"
            bind:value={username}
            helperText="用于连接 API 的 Jira 账号邮箱。若使用个人访问令牌 (PAT) 进行自建 Jira 认证，请将此字段留空。"
          />

          {#if showTokenEditor}
            <TextInput
              id="jira-token"
              label="Jira API 令牌 / 个人访问令牌 (PAT)"
              placeholder="请输入 API Token 或 PAT"
              type="password"
              bind:value={apiToken}
              required={enabled}
              helperText="对于 Jira Cloud，请填写 API 令牌与对应邮箱；对于自建 Jira 服务，请填写个人访问令牌 (PAT) 并将邮箱留空。"
            />
          {:else}
            <div class="credential-collapsed">
              <div>
                <span>Jira API Token / PAT</span>
                <strong>已配置，当前默认脱敏折叠</strong>
              </div>
              <button type="button" on:click={() => showTokenEditor = true}>编辑凭证/高级配置</button>
            </div>
          {/if}

          <TextInput
            id="jira-sync-projects"
            label="同步项目键列表 (Project Keys)"
            placeholder="PROJ, TEAM"
            bind:value={syncProjects}
            helperText="需要同步的 Jira 项目键（Key），多个项目用逗号分隔。例如: PROJ, DEVS"
          />

          <TextInput
            id="jira-sync-users"
            label="分配的用户邮箱/用户名列表"
            placeholder="eddie@company.com, developer@company.com"
            bind:value={syncUsers}
            helperText="只同步指派给这些用户的任务/Bug，多个用户用逗号分隔。"
          />

          <TextInput
            id="jira-sync-statuses"
            label="同步的任务/Bug进度状态筛选"
            placeholder="To Do, In Progress, In Review, Done"
            bind:value={syncStatuses}
            helperText="需要过滤/同步的 Jira 状态，多个状态用逗号分隔。例如: To Do, In Progress, Done"
          />

          <TextInput
            id="jira-custom-jql"
            label="自定义 JQL 筛选器 (覆盖上方所有过滤条件 - 高级)"
            placeholder="project = PROJ AND status = 'In Progress'"
            bind:value={customJQL}
            helperText="自定义 Jira 检索语句 (JQL)。如果填写此项，它将直接应用于同步拉取，并覆盖上方的项目、用户和进度筛选。"
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
              测试 Jira 连接
            </Button>
          </div>
        </div>
      {/if}

      <div class="actions">
        <Button variant="primary" on:click={nextStep}>
          下一步
        </Button>
      </div>
    </div>
  {:else if currentStep === 2}
    <div class="step-content">
      <div class="info-block">
        <h4>Jira 集成配置摘要</h4>
        <p>确认无误后点击下方按钮应用并应用配置：</p>
      </div>

      <div class="summary-card">
        <div class="summary-row">
          <span class="summary-label">Jira 同步状态:</span>
          <span class="summary-value">
            {#if enabled}
              <span class="text-success">已启用</span>
            {:else}
              <span class="text-muted">已禁用</span>
            {/if}
          </span>
        </div>
        {#if enabled}
          <div class="summary-row">
            <span class="summary-label">Jira 连接地址:</span>
            <span class="summary-value font-mono">{baseURL}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">认证用户名:</span>
            <span class="summary-value font-mono">{username || '(留空/Token认证)'}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">同步项目:</span>
            <span class="summary-value font-mono">{syncProjects || '所有项目'}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">指派用户:</span>
            <span class="summary-value font-mono">{syncUsers || '所有用户'}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">进度筛选:</span>
            <span class="summary-value font-mono">{syncStatuses || '所有状态'}</span>
          </div>
          {#if customJQL}
            <div class="summary-row">
              <span class="summary-label">自定义 JQL:</span>
              <span class="summary-value font-mono text-warning">{customJQL}</span>
            </div>
          {/if}
        {/if}
      </div>

      {#if saveError}
        <Alert type="error" title="保存失败" message={saveError} />
      {/if}

      <div class="actions">
        <Button variant="ghost" on:click={prevStep} disabled={saving}>上一步</Button>
        <Button variant="primary" loading={saving} on:click={() => saveConfig(false)}>
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
    border-left: 4px solid #6366f1;
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
    line-height: 1.5;
    color: #94a3b8;
  }

  .form-sub-section {
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    padding-top: 20px;
    margin-top: 8px;
    display: flex;
    flex-direction: column;
  }

  .test-row {
    margin-bottom: 20px;
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
    display: flex;
    flex-direction: column;
    background: rgba(15, 23, 42, 0.25);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 6px;
    padding: 0 16px;
    margin-top: 10px;
  }

  .overview-row {
    min-width: 0;
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    padding: 14px 0;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    background: none;
    border-radius: 0;
    gap: 16px;
  }
  .overview-row:last-child {
    border-bottom: none;
  }

  .credential-collapsed,
  .jql-preview {
    min-width: 0;
    background: rgba(15, 23, 42, 0.52);
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-radius: 8px;
    padding: 12px;
  }

  .jql-preview {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .overview-row span {
    color: #94a3b8;
    font-size: 0.8rem;
    font-weight: 500;
  }

  .overview-row strong {
    color: #f8fafc;
    font-size: 0.85rem;
    font-weight: 600;
    text-align: right;
  }

  .credential-collapsed span,
  .jql-preview span {
    color: #64748b;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .credential-collapsed strong {
    color: #e2e8f0;
    font-size: 0.86rem;
    overflow-wrap: anywhere;
  }

  .jql-preview code {
    color: #fbbf24;
    font-size: 0.82rem;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
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

  .details-pre {
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 6px;
    padding: 10px;
    font-size: 0.725rem;
    color: #94a3b8;
    max-height: 120px;
    overflow-y: auto;
    margin: 8px 0 0 0;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .summary-card {
    background: #0b0f19;
    border: 1px solid rgba(51, 65, 85, 0.5);
    border-radius: 8px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 24px;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    font-size: 0.85rem;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 8px;
    min-width: 0;
  }

  .summary-row:last-of-type {
    border-bottom: none;
    padding-bottom: 0;
  }

  .summary-label {
    color: #64748b;
    font-weight: 500;
    flex: 0 0 auto;
  }

  .summary-value {
    color: #cbd5e1;
    font-weight: 600;
    min-width: 0;
    max-width: 100%;
    text-align: right;
    overflow-wrap: anywhere;
    word-break: break-word;
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
    animation: stroke 0.3s cubic-bezier(0.65, 0, 0.45, 1) 0.6s forwards;
  }

  .success-actions {
    display: flex;
    justify-content: center;
    width: 100%;
  }

  @keyframes stroke {
    100% {
      stroke-dashoffset: 0;
    }
  }

  @keyframes fill {
    100% {
      box-shadow: inset 0px 0px 0px 32px rgba(52, 211, 153, 0.1);
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

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }
  .indicator-online {
    background-color: #10b981;
    box-shadow: 0 0 8px #10b981;
  }
  .indicator-warning {
    background-color: #f59e0b;
    box-shadow: 0 0 8px #f59e0b;
  }
</style>
